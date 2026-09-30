package scrcpy

import (
	"ADBKit/internal/core"
	"strings"
	"testing"
)

func TestPushClipboard_RejectsEmptySerial(t *testing.T) {
	svc := &Service{}
	err := svc.PushClipboard("", "hello")
	if err == nil {
		t.Fatal("expected error for empty serial")
	}
	if !strings.Contains(err.Error(), "Device serial is required") {
		t.Errorf("expected friendly error, got %v", err)
	}
}

func TestPushClipboard_RejectsEmptyText(t *testing.T) {
	svc := &Service{}
	err := svc.PushClipboard("ABC123", "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if !strings.Contains(err.Error(), "Clipboard text is required") {
		t.Errorf("expected friendly error, got %v", err)
	}
}

func TestGetClipboard_RejectsEmptySerial(t *testing.T) {
	svc := &Service{}
	if _, err := svc.GetClipboard(""); err == nil {
		t.Fatal("expected error for empty serial")
	}
}

func TestBuildClipboardSetCommandShellQuotesText(t *testing.T) {
	text := "hello'; touch /sdcard/pwned; echo 'x"
	got := buildClipboardSetCommand(text)
	want := "cmd clipboard set " + core.QuoteShellArg(text)
	if got != want {
		t.Fatalf("buildClipboardSetCommand() = %q, want %q", got, want)
	}
}
