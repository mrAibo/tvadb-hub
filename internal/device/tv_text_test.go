package device

import (
	"ADBKit/internal/core"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSendTVTextPrefersClipboardAndPreservesUnicode(t *testing.T) {
	var calls []core.ExecRequest
	result, err := sendTVText(
		context.Background(),
		"adb",
		"SERIAL",
		"Grüße aus Hannover",
		func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			calls = append(calls, req)
			command := req.Args[len(req.Args)-1]
			if command == "cmd clipboard help" {
				return &core.ExecResult{Stdout: "  set TEXT\n  get\n"}, nil
			}
			if command == "cmd clipboard get" {
				return &core.ExecResult{Stdout: "Grüße aus Hannover\n"}, nil
			}
			return &core.ExecResult{ExitCode: 0}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != TVTextMethodAndroidClipboard {
		t.Fatalf("method=%q want=%q", result.Method, TVTextMethodAndroidClipboard)
	}
	if len(calls) != 4 {
		t.Fatalf("calls=%d want=4", len(calls))
	}
	if got := calls[1].Args[len(calls[1].Args)-1]; !strings.Contains(got, core.QuoteShellArg("Grüße aus Hannover")) {
		t.Fatalf("clipboard command did not shell-quote text: %q", got)
	}
	if got := calls[3].Args[len(calls[3].Args)-1]; got != "input keyevent KEYCODE_PASTE" {
		t.Fatalf("paste command=%q", got)
	}
}

func TestSendTVTextFallsBackToPrintableASCII(t *testing.T) {
	var calls []core.ExecRequest
	result, err := sendTVText(
		context.Background(),
		"adb",
		"SERIAL",
		"hello world; safe",
		func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			calls = append(calls, req)
			if len(calls) == 1 {
				return &core.ExecResult{ExitCode: 1, Stderr: "clipboard unavailable"}, errors.New("exit status 1")
			}
			return &core.ExecResult{ExitCode: 0}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != TVTextMethodInputText {
		t.Fatalf("method=%q want=%q", result.Method, TVTextMethodInputText)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%d want=2", len(calls))
	}
	got := calls[1].Args[len(calls[1].Args)-1]
	want := "input text " + core.QuoteShellArg("hello%sworld;%ssafe")
	if got != want {
		t.Fatalf("fallback command=%q want=%q", got, want)
	}
}

func TestSendTVTextDoesNotFallbackUnicodeThroughInputText(t *testing.T) {
	calls := 0
	_, err := sendTVText(
		context.Background(),
		"adb",
		"SERIAL",
		"Привет",
		func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			calls++
			return &core.ExecResult{ExitCode: 1, Stderr: "clipboard unavailable"}, errors.New("exit status 1")
		},
	)
	if err == nil {
		t.Fatal("expected Unicode fallback to be rejected")
	}
	if calls != 1 {
		t.Fatalf("calls=%d want=1", calls)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "printable ascii") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendTVTextShellQuotesInjectionCharacters(t *testing.T) {
	payload := "hello'; touch /sdcard/pwned; echo 'x"
	var first core.ExecRequest
	_, _ = sendTVText(
		context.Background(),
		"adb",
		"SERIAL",
		payload,
		func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			if strings.HasPrefix(req.Args[len(req.Args)-1], "cmd clipboard set ") {
				first = req
			}
			if req.Args[len(req.Args)-1] == "cmd clipboard help" {
				return &core.ExecResult{Stdout: "  set TEXT\n  get\n"}, nil
			}
			return &core.ExecResult{ExitCode: 0}, nil
		},
	)
	command := first.Args[len(first.Args)-1]
	if command != "cmd clipboard set "+core.QuoteShellArg(payload) {
		t.Fatalf("unsafe clipboard command: %q", command)
	}
}

func TestSendTVTextRejectsPercentFallbackAmbiguity(t *testing.T) {
	_, err := sendTVText(
		context.Background(),
		"adb",
		"SERIAL",
		"100%safe",
		func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			return &core.ExecResult{ExitCode: 1}, errors.New("clipboard unavailable")
		},
	)
	if err == nil {
		t.Fatal("expected percent-containing fallback text to be rejected")
	}
}

func TestZeroExitUnsupportedClipboardNeverTriggersPaste(t *testing.T) {
	for _, text := range []string{"hello", "Привет"} {
		var commands []string
		result, err := sendTVText(context.Background(), "adb", "A", text, func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			command := req.Args[len(req.Args)-1]
			commands = append(commands, command)
			if strings.HasPrefix(command, "cmd clipboard") {
				return &core.ExecResult{ExitCode: 0, Stderr: "No shell command implementation."}, nil
			}
			return &core.ExecResult{}, nil
		})
		for _, command := range commands {
			if strings.Contains(command, "KEYCODE_PASTE") {
				t.Fatal("pasted after unsupported clipboard")
			}
		}
		if text == "hello" && (err != nil || result.Method != TVTextMethodInputText) {
			t.Fatalf("ASCII fallback: %+v %v", result, err)
		}
		if text != "hello" && (err == nil || len(commands) != 1) {
			t.Fatalf("Unicode unsupported path: %+v %v", commands, err)
		}
	}
}
