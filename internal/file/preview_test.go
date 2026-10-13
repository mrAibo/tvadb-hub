package file

import (
	"ADBKit/internal/core"
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewLocalTextAndBoundaries(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(good, []byte("hello π"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{}
	got, err := svc.PreviewLocalFile(good)
	if err != nil || got.Kind != "text" || got.Text != "hello π" || got.Source != "local" {
		t.Fatalf("preview=%+v, err=%v", got, err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"binary.txt", []byte{0, 1, 2}},
		{"badutf8.txt", []byte{0xff}},
		{"oversize.txt", bytes.Repeat([]byte("A"), int(MaxPreviewBytes)+1)},
		{"unsupported.exe", []byte("hello")},
		{"fake.png", []byte("<script>alert(1)</script>")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(dir, tc.name)
			if err := os.WriteFile(target, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.PreviewLocalFile(target); err == nil {
				t.Fatal("should reject unsafe preview")
			}
		})
	}
	if _, err := svc.PreviewLocalFile(dir); err == nil {
		t.Fatal("directory preview accepted")
	}
}

func TestPreviewImageAllowlist(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	got, err := decodePreview("device", "abc", "/sdcard/a.png", "image", data.Bytes())
	if err != nil || got.MimeType != "image/png" || !strings.HasPrefix(got.ImageDataURL, "data:image/png;base64,") {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if _, err := previewKind("a.svg"); err == nil {
		t.Fatal("SVG with scripts should be rejected")
	}
}

func TestPreviewRemoteConfirmedTargetAndLimit(t *testing.T) {
	calls := 0
	s := &Service{
		getBinPath: func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb"} },
		readPreviewRemote: func(ctx context.Context, binary, serial, remotePath string) ([]byte, error) {
			calls++
			if binary != "adb" || serial != "TV-A" || remotePath != "/sdcard/notes.txt" {
				t.Fatalf("unexpected remote target: %q %q %q", binary, serial, remotePath)
			}
			return []byte("read-only"), nil
		},
	}
	if _, err := s.PreviewRemoteFileForDevice(context.Background(), "", "/sdcard/notes.txt"); err == nil {
		t.Fatal("blank serial was accepted")
	}
	if _, err := s.PreviewRemoteFileForDevice(context.Background(), "TV-A", "/sdcard/notes.exe"); err == nil {
		t.Fatal("unsupported extension was accepted")
	}
	if calls != 0 {
		t.Fatal("ADB was called before validating inputs")
	}
	got, err := s.PreviewRemoteFileForDevice(context.Background(), "TV-A", "/sdcard/../sdcard/notes.txt")
	if err != nil || got.Serial != "TV-A" || got.Source != "device" || got.Text != "read-only" || calls != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", got, err, calls)
	}
	s.readPreviewRemote = func(context.Context, string, string, string) ([]byte, error) {
		return bytes.Repeat([]byte("x"), int(MaxPreviewBytes)+1), nil
	}
	if _, err := s.PreviewRemoteFileForDevice(context.Background(), "TV-A", "/sdcard/notes.txt"); err == nil {
		t.Fatal("remote oversize accepted")
	}
}
