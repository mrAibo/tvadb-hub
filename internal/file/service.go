package file

import (
	"ADBKit/internal/core"
	"context"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultPath = "/sdcard/"
	dirType     = "directory"
	regularType = "file"
	symlinkType = "symlink"
	otherType   = "other"
	sizeUnknown = "-"
	sizeDirNone = "--"

	transferRetries = 3
	transferDelay   = 2 * time.Second

	TransferProgressEvent = "file_transfer_progress"
)

var adbProgressPattern = regexp.MustCompile(`\[\s*(\d+)%\]\s*(.*)`)

type Service struct {
	wailsCtx            context.Context
	resolveActiveSerial func(context.Context) (string, error)
	getBinPath          func() core.BinaryPaths

	mu                      sync.Mutex
	cancelFunc              context.CancelFunc
	activeOperationID       string
	nextOperationID         uint64
	runStreaming            func(context.Context, core.StreamingExecRequest) (*core.ExecResult, error)
	runCommand              func(context.Context, core.ExecRequest) (*core.ExecResult, error)
	getTransferCompression  func() string
	getTransferVerification func() bool
	compressionCache        map[string]adbCompressionCapabilities
}

func NewService(
	wailsCtx context.Context,
	resolveActiveSerial func(context.Context) (string, error),
	getBinPath func() core.BinaryPaths,
) *Service {
	return &Service{
		wailsCtx:            wailsCtx,
		resolveActiveSerial: resolveActiveSerial,
		getBinPath:          getBinPath,
		compressionCache:    make(map[string]adbCompressionCapabilities),
	}
}

func (s *Service) SetTransferCompressionResolver(resolve func() string) {
	s.mu.Lock()
	s.getTransferCompression = resolve
	s.mu.Unlock()
}

func (s *Service) SetTransferVerificationResolver(resolve func() bool) {
	s.mu.Lock()
	s.getTransferVerification = resolve
	s.mu.Unlock()
}

func (s *Service) requireActiveSerial(ctx context.Context) (string, error) {
	if s.resolveActiveSerial == nil {
		return "", core.NewOperationError("resolve_active_serial", "No active device is available", "active serial resolver is not configured", true)
	}
	return s.resolveActiveSerial(ctx)
}

// targetService is the smallest non-transfer view of a file service pinned to one
// confirmed device. It owns a fresh Service value built from explicit immutable
// callbacks, so it copies no mutex, no cancellation/operation state and no
// transfer cache. Transfers and cancellation always stay on the original service:
// this view deliberately exposes no transfer method.
type targetService struct {
	svc *Service
}

// ForTarget pins the resolver and the tool paths used by listing, deleting, mkdir,
// rename and storage helpers for one confirmed device. The pinned service never
// consults the mutable global device selection, and a blank serial is refused
// before any command runs. The pinned callbacks are read without holding s.mu,
// because that lock also guards the transfer state this view must not touch.
func (s *Service) ForTarget(serial string) *targetService {
	paths := core.BinaryPaths{}
	if s.getBinPath != nil {
		paths = s.getBinPath()
	}
	trimmed := strings.TrimSpace(serial)
	return &targetService{svc: &Service{
		wailsCtx: s.wailsCtx,
		resolveActiveSerial: func(context.Context) (string, error) {
			if trimmed == "" {
				return "", core.NewOperationError("device_target", "Confirmed device is required", "select and confirm an ADB device", false)
			}
			return trimmed, nil
		},
		getBinPath: func() core.BinaryPaths { return paths },
	}}
}

func (t *targetService) ListFiles(ctx context.Context, remotePath string, showHidden bool) ([]Entry, error) {
	return t.svc.ListFiles(ctx, remotePath, showHidden)
}

func (t *targetService) GetDirectorySize(ctx context.Context, remotePath string) (string, error) {
	return t.svc.GetDirectorySize(ctx, remotePath)
}

func (t *targetService) GetStorageInfo(ctx context.Context) (StorageInfo, error) {
	return t.svc.GetStorageInfo(ctx)
}

func (t *targetService) ListSdCards(ctx context.Context) ([]SdCard, error) {
	return t.svc.ListSdCards(ctx)
}

func (t *targetService) UnblockPath(ctx context.Context, remotePath string) (UnblockResult, error) {
	return t.svc.UnblockPath(ctx, remotePath)
}

func (t *targetService) DeleteFile(ctx context.Context, remotePath string) (string, error) {
	return t.svc.DeleteFile(ctx, remotePath)
}

func (t *targetService) DeleteMultipleFiles(ctx context.Context, remotePaths []string) (string, error) {
	return t.svc.DeleteMultipleFiles(ctx, remotePaths)
}

func (t *targetService) CreateDirectory(ctx context.Context, remotePath string) (string, error) {
	return t.svc.CreateDirectory(ctx, remotePath)
}

func (t *targetService) RenameFile(ctx context.Context, oldRemotePath string, newRemotePath string) (string, error) {
	return t.svc.RenameFile(ctx, oldRemotePath, newRemotePath)
}

// CancelTransfer cancels the active file transfer if one is in progress.
func (s *Service) CancelTransfer() {
	s.CancelTransferFor("")
}

// An old UI event must not cancel a newer transfer. Empty ID preserves the
// historical single-active-transfer cancellation API.
func (s *Service) CancelTransferFor(operationID string) {
	s.mu.Lock()
	fn := s.cancelFunc
	if operationID != "" && operationID != s.activeOperationID {
		fn = nil
	}
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ListSdCards runs `adb shell sm list-volumes` and returns the parsed list of
// currently-mounted storage volumes on the active device.
func (s *Service) ListSdCards(ctx context.Context) ([]SdCard, error) {
	serial, err := s.requireActiveSerial(ctx)
	if err != nil {
		return nil, err
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"-s", serial, "shell", "sm", "list-volumes"},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return nil, core.NewOperationError("list_sd_cards", "Failed to list storage volumes", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return nil, core.NewOperationError("list_sd_cards", "Failed to list storage volumes", strings.TrimSpace(result.Stderr), true)
	}

	return ParseSdCardList(result.Stdout), nil
}

// UnblockPath returns honest guidance for recovering access to a blocked remote
// path. It classifies the path, checks whether it is an SD card mount point that
// is currently missing from the volume list, and surfaces the appropriate
// UnblockResult. ADBKit never fabricates a bypass — scoped-storage and
// system-path restrictions are enforced by Android itself.
func (s *Service) UnblockPath(ctx context.Context, remotePath string) (UnblockResult, error) {
	class := ClassifyPath(remotePath)

	if class == PathSystem {
		return UnblockResult{
			Type:   UnblockNotNeeded,
			Path:   remotePath,
			Reason: "System paths cannot be accessed through File Explorer.",
		}, nil
	}

	if class == PathProtected {
		return UnblockResult{
			Type:   UnblockOpenSettings,
			Path:   remotePath,
			Reason: "This path is protected by Android scoped storage.",
		}, nil
	}

	if !IsSdCardMountPoint(remotePath) {
		return UnblockResult{Type: UnblockNotNeeded, Path: remotePath}, nil
	}

	cards, err := s.ListSdCards(ctx)
	if err != nil {
		return UnblockResult{}, err
	}

	mountPoint, _ := normalizeRemotePath(remotePath)
	for _, card := range cards {
		if card.MountPoint == mountPoint || strings.HasPrefix(mountPoint, card.MountPoint+"/") {
			return UnblockResult{
				Type:   UnblockNotNeeded,
				Path:   remotePath,
				Reason: "The storage volume is accessible.",
			}, nil
		}
	}

	return UnblockResult{
		Type:   UnblockVolumeMissing,
		Path:   remotePath,
		Reason: "The storage volume holding this path is not currently mounted.",
	}, nil
}
