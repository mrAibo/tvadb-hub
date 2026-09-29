package app

import (
	"path/filepath"
	"testing"
)

func TestHostOpenCommand(t *testing.T) {
	file := filepath.Join("tmp", "platform-tools", "adb")
	dir := filepath.Dir(file)

	tests := []struct {
		goos   string
		isDir  bool
		want   string
		arg0   string
	}{
		{goos: "windows", isDir: false, want: "explorer.exe", arg0: "/select,"},
		{goos: "darwin", isDir: false, want: "open", arg0: "-R"},
		{goos: "linux", isDir: false, want: "xdg-open", arg0: dir},
		{goos: "linux", isDir: true, want: "xdg-open", arg0: file},
	}

	for _, tt := range tests {
		name, args := hostOpenCommand(tt.goos, file, tt.isDir)
		if name != tt.want {
			t.Fatalf("%s: command=%q want=%q", tt.goos, name, tt.want)
		}
		if len(args) == 0 || args[0] != tt.arg0 {
			t.Fatalf("%s: args=%v want first=%q", tt.goos, args, tt.arg0)
		}
	}

	name, args := hostOpenCommand("plan9", file, false)
	if name != "" || args != nil {
		t.Fatalf("unsupported platform should return empty command, got %q %v", name, args)
	}
}
