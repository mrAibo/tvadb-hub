package core

import (
	"context"
	"strings"
	"testing"
)

func TestClipboardRejectsZeroExitUnimplementedBinderShell(t *testing.T) {
	calls := 0
	err := SetAndroidClipboard(context.Background(), "adb", "A", "Привет", func(context.Context, ExecRequest) (*ExecResult, error) {
		calls++
		return &ExecResult{Stderr: "No shell command implementation."}, nil
	})
	if err == nil || calls != 1 {
		t.Fatalf("unimplemented shell accepted: calls=%d err=%v", calls, err)
	}
}

func TestClipboardRequiresExactReadbackIncludingUnicodeAndNewlines(t *testing.T) {
	for _, text := range []string{"Grüße", "Привет\nмир\n", "text with 'quotes'", "unknown command: not found"} {
		t.Run(text, func(t *testing.T) {
			var commands []string
			err := SetAndroidClipboard(context.Background(), "adb", "A", text, func(_ context.Context, req ExecRequest) (*ExecResult, error) {
				command := req.Args[len(req.Args)-1]
				commands = append(commands, command)
				switch command {
				case "cmd clipboard help":
					return &ExecResult{Stdout: "Clipboard commands:\n  set TEXT\n  get\n"}, nil
				case "cmd clipboard get":
					return &ExecResult{Stdout: text + "\n"}, nil
				}
				return &ExecResult{}, nil
			})
			if err != nil || len(commands) != 3 || commands[1] != "cmd clipboard set "+QuoteShellArg(text) {
				t.Fatalf("commands=%+v err=%v", commands, err)
			}
		})
	}
}

func TestClipboardRejectsStaleReadback(t *testing.T) {
	err := SetAndroidClipboard(context.Background(), "adb", "A", "requested text", func(_ context.Context, req ExecRequest) (*ExecResult, error) {
		command := req.Args[len(req.Args)-1]
		if strings.HasSuffix(command, "help") {
			return &ExecResult{Stdout: "  set TEXT\n  get\n"}, nil
		}
		return &ExecResult{Stdout: "old clipboard"}, nil
	})
	if err == nil {
		t.Fatal("stale clipboard content accepted")
	}
}
