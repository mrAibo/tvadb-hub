package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ADBKit/internal/core"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	platformToolsVersion = "37.0.1"

	// Google SDK repository metadata pins the exact Platform Tools archives by
	// SHA-1 and byte size. Verification is fail-closed before extraction.
	platformToolsLinuxSHA1   = "477254aa5f903c15cf51001717bdf347fb6b53e0"
	platformToolsDarwinSHA1  = "6ae73f4de6452dc57e62ec02b68eed92a4c21661"
	platformToolsWindowsSHA1 = "e03e78b1d80b396f1c3358e31251cb31740e1110"
	platformToolsLinuxSize   = int64(9054187)
	platformToolsDarwinSize  = int64(16110554)
	platformToolsWindowsSize = int64(8044989)

	scrcpyVersion              = "4.1"
	scrcpyLinuxAMD64SHA256     = "ad56ae8bfeedf41e824945c11dbf55fcb092b3e615b9b486f48a50e30d389635"
	scrcpyDarwinAMD64SHA256    = "ee2a7223bc8dbdc4f482db1134bcf441178dafb833492b71ca4c22090c58ce72"
	scrcpyDarwinARM64SHA256    = "20fd47c9014dd5e0fa77091f3cb7adbda8445a360c4584aeaa0150b5b3988ff3"
	scrcpyWindowsAMD64SHA256   = "5b12172b3264b2889f4583ee64752ce832e29bc8b1089dca81093459697165db"
	eventName                  = "binary_download_progress"
)
type ProgressEvent struct {
	Name          string  `json:"name"`
	Percent       float64 `json:"percent"`
	BytesReceived int64   `json:"bytesReceived"`
	BytesTotal    int64   `json:"bytesTotal"`
	Status        string  `json:"status"`
}

type Service struct {
	ctx     context.Context
	dataDir string
}

func NewService(ctx context.Context, dataDir string) *Service {
	return &Service{ctx: ctx, dataDir: dataDir}
}

func (s *Service) DownloadPlatformTools(ctx context.Context) error {
	goos := runtime.GOOS
	var url, expectedSHA1 string
	var expectedSize int64

	switch goos {
	case "linux":
		url = fmt.Sprintf("https://dl.google.com/android/repository/platform-tools_r%s-linux.zip", platformToolsVersion)
		expectedSHA1 = platformToolsLinuxSHA1
		expectedSize = platformToolsLinuxSize
	case "darwin":
		url = fmt.Sprintf("https://dl.google.com/android/repository/platform-tools_r%s-darwin.zip", platformToolsVersion)
		expectedSHA1 = platformToolsDarwinSHA1
		expectedSize = platformToolsDarwinSize
	case "windows":
		url = fmt.Sprintf("https://dl.google.com/android/repository/platform-tools_r%s-win.zip", platformToolsVersion)
		expectedSHA1 = platformToolsWindowsSHA1
		expectedSize = platformToolsWindowsSize
	default:
		return core.NewOperationError("download_platform_tools", "unsupported OS", goos, false)
	}

	s.emitProgress("platform-tools", 0, 0, 0, "downloading")

	archivePath := filepath.Join(s.dataDir, "bin", "platform-tools.zip")
	defer os.Remove(archivePath)

	progressFn := func(received, total int64) {
		pct := 0.0
		if total > 0 {
			pct = float64(received) / float64(total) * 100
		}
		s.emitProgress("platform-tools", pct, received, total, "downloading")
	}

	dl := NewDownloader(progressFn)
	if err := dl.Fetch(ctx, url, archivePath); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}
	if err := VerifyFileSize(archivePath, expectedSize); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}
	if err := VerifySHA1(archivePath, expectedSHA1); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}

	s.emitProgress("platform-tools", 50, 0, 0, "extracting")

	tmpDir := filepath.Join(s.dataDir, "bin", ".extract-platform-tools")
	defer os.RemoveAll(tmpDir)

	if err := ExtractZip(archivePath, tmpDir); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}

	extractedDir, err := FindExtractedDir(tmpDir, "platform-tools")
	if err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}
	if err := ValidatePackageContents(extractedDir, []string{"adb", "fastboot"}); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}

	destDir := filepath.Join(s.dataDir, "bin", "platform-tools")
	if err := MoveExtractedDir(extractedDir, destDir); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}

	if err := ValidatePackageContents(destDir, []string{"adb", "fastboot"}); err != nil {
		s.emitProgress("platform-tools", 0, 0, 0, "error")
		return err
	}

	s.cleanupOldStandalone("adb", "fastboot")
	s.emitProgress("platform-tools", 100, 0, 0, "done")
	return nil
}
func (s *Service) DownloadScrcpy(ctx context.Context) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var archiveName, expectedSHA256 string
	switch goos {
	case "linux":
		if goarch != "amd64" {
			return core.NewOperationError(
				"download_scrcpy",
				"managed scrcpy is unavailable for this Linux architecture",
				"scrcpy v"+scrcpyVersion+" does not publish a Linux "+goarch+" desktop archive",
				false,
			)
		}
		archiveName = fmt.Sprintf("scrcpy-linux-x86_64-v%s.tar.gz", scrcpyVersion)
		expectedSHA256 = scrcpyLinuxAMD64SHA256
	case "darwin":
		switch goarch {
		case "amd64":
			archiveName = fmt.Sprintf("scrcpy-macos-x86_64-v%s.tar.gz", scrcpyVersion)
			expectedSHA256 = scrcpyDarwinAMD64SHA256
		case "arm64":
			archiveName = fmt.Sprintf("scrcpy-macos-aarch64-v%s.tar.gz", scrcpyVersion)
			expectedSHA256 = scrcpyDarwinARM64SHA256
		default:
			return core.NewOperationError("download_scrcpy", "unsupported macOS architecture", goarch, false)
		}
	case "windows":
		if goarch != "amd64" {
			return core.NewOperationError(
				"download_scrcpy",
				"managed scrcpy is unavailable for this Windows architecture",
				"the managed scrcpy package is the upstream win64 build",
				false,
			)
		}
		archiveName = fmt.Sprintf("scrcpy-win64-v%s.zip", scrcpyVersion)
		expectedSHA256 = scrcpyWindowsAMD64SHA256
	default:
		return core.NewOperationError("download_scrcpy", "unsupported OS", goos, false)
	}

	url := fmt.Sprintf("https://github.com/Genymobile/scrcpy/releases/download/v%s/%s", scrcpyVersion, archiveName)

	s.emitProgress("scrcpy", 0, 0, 0, "downloading")

	archivePath := filepath.Join(s.dataDir, "bin", archiveName)
	defer os.Remove(archivePath)

	progressFn := func(received, total int64) {
		pct := 0.0
		if total > 0 {
			pct = float64(received) / float64(total) * 100
		}
		s.emitProgress("scrcpy", pct, received, total, "downloading")
	}

	dl := NewDownloader(progressFn)
	if err := dl.Fetch(ctx, url, archivePath); err != nil {
		s.emitProgress("scrcpy", 0, 0, 0, "error")
		return err
	}
	if err := VerifySHA256(archivePath, expectedSHA256); err != nil {
		s.emitProgress("scrcpy", 0, 0, 0, "error")
		return err
	}

	s.emitProgress("scrcpy", 50, 0, 0, "extracting")

	tmpDir := filepath.Join(s.dataDir, "bin", ".extract-scrcpy")
	defer os.RemoveAll(tmpDir)

	if goos == "windows" {
		if err := ExtractZip(archivePath, tmpDir); err != nil {
			s.emitProgress("scrcpy", 0, 0, 0, "error")
			return err
		}
	} else {
		if err := ExtractTarGz(archivePath, tmpDir); err != nil {
			s.emitProgress("scrcpy", 0, 0, 0, "error")
			return err
		}
	}

	extractedDir, err := FindExtractedDir(tmpDir, "scrcpy")
	if err != nil {
		s.emitProgress("scrcpy", 0, 0, 0, "error")
		return err
	}
	if err := ValidateScrcpyPackage(extractedDir); err != nil {
		s.emitProgress("scrcpy", 0, 0, 0, "error")
		return err
	}

	destDir := filepath.Join(s.dataDir, "bin", "scrcpy")
	if err := MoveExtractedDir(extractedDir, destDir); err != nil {
		s.emitProgress("scrcpy", 0, 0, 0, "error")
		return err
	}

	if err := ValidateScrcpyPackage(destDir); err != nil {
		s.emitProgress("scrcpy", 0, 0, 0, "error")
		return err
	}

	s.cleanupOldStandalone("scrcpy")
	s.emitProgress("scrcpy", 100, 0, 0, "done")
	return nil
}
func (s *Service) emitProgress(name string, pct float64, received, total int64, status string) {
	if s.ctx == nil {
		return
	}
	application.Get().Event.Emit(eventName, ProgressEvent{
		Name:          name,
		Percent:       pct,
		BytesReceived: received,
		BytesTotal:    total,
		Status:        strings.ToLower(status),
	})

	_ = time.Now()
}

func (s *Service) cleanupOldStandalone(names ...string) {
	for _, name := range names {
		path := filepath.Join(s.dataDir, "bin", BinaryExecutableName(name))
		os.Remove(path)
	}
}
