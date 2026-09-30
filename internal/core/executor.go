package core

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

// ExecRequest defines the input for a process execution.
type ExecRequest struct {
	Command string
	Args    []string
	Timeout time.Duration
}

// ExecResult holds the output of a process execution.
type ExecResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exitCode"`
	Duration time.Duration `json:"duration"`
}

// RunCommand executes a process with optional timeout and captures stdout/stderr.
func RunCommand(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	cmd := NewCommandContext(ctx, req.Command, req.Args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, NewOperationError("exec", "failed to start process", err.Error(), true)
		}
	}

	result := &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}
	return result, err
}

// RunCommandWithStdin executes a process that receives input via stdin.
func RunCommandWithStdin(ctx context.Context, req ExecRequest, stdin string) (*ExecResult, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	cmd := NewCommandContext(ctx, req.Command, req.Args...)
	cmd.Stdin = bytes.NewBufferString(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, NewOperationError("exec", "failed to start process", err.Error(), true)
		}
	}

	result := &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}
	return result, err
}

// StreamingExecRequest extends ExecRequest with a line-level progress callback.
type StreamingExecRequest struct {
	Command      string
	Args         []string
	Timeout      time.Duration
	OnStderrLine func(line string)
}

// scanProgressLines splits on both \n and \r so that adb's carriage-return
// driven progress updates are emitted as individual tokens.
func scanProgressLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// errPTYUnsupported signals that PTY-backed execution is unavailable and the
// caller should fall back to pipe streaming.
var errPTYUnsupported = errors.New("pty unsupported on this platform")

// RunCommandStreaming starts a process and streams output line-by-line via
// callback. It prefers a PTY so adb emits interactive progress; on platforms
// where PTY is unavailable it falls back to dual-pipe streaming.
func RunCommandStreaming(ctx context.Context, req StreamingExecRequest) (*ExecResult, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	if result, err := runWithPTY(ctx, req.Command, req.Args, req.OnStderrLine); err == nil {
		return result, nil
	} else if !errors.Is(err, errPTYUnsupported) {
		return result, err
	}
	return runWithPipes(ctx, req)
}

// Assigning writers lets os/exec own and drain its copy goroutines before Wait
// returns. Unlike StdoutPipe/StderrPipe, Wait cannot close unread output early.
func runWithPipes(ctx context.Context, req StreamingExecRequest) (*ExecResult, error) {
	cmd := NewCommandContext(ctx, req.Command, req.Args...)
	var callbackMu sync.Mutex
	onLine := func(line string) {
		if req.OnStderrLine != nil {
			callbackMu.Lock()
			defer callbackMu.Unlock()
			req.OnStderrLine(line)
		}
	}
	onReadError := func() { _ = TerminateProcessTree(cmd) }
	stdout := NewProcessLineWriter(onLine, onReadError)
	stderr := NewProcessLineWriter(onLine, onReadError)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, NewOperationError("exec", "failed to start process", err.Error(), true)
	}
	waitErr := cmd.Wait() // also joins both output copy goroutines
	stdout.Flush()
	stderr.Flush()
	result := &ExecResult{Stdout: stdout.output.String(), Stderr: stderr.output.String(), Duration: time.Since(start)}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	result.ExitCode = exitCode
	if readErr := errors.Join(stdout.err, stderr.err); readErr != nil {
		return result, NewOperationError("exec", "failed to read process output", readErr.Error(), true)
	}
	if waitErr != nil && exitCode == 0 {
		return result, NewOperationError("exec", "process output drain failed", waitErr.Error(), true)
	}
	return result, waitErr
}

const maxCapturedOutput = 1024 * 1024

// Retain only a diagnostic tail; compact amortized rather than copying a MiB
// on every short progress line. Internal storage is bounded to two MiB.
type boundedOutput struct{ buffer bytes.Buffer }

func (b *boundedOutput) append(text string) {
	if len(text) >= maxCapturedOutput {
		b.buffer.Reset()
		b.buffer.WriteString(text[len(text)-maxCapturedOutput:])
		return
	}
	if b.buffer.Len()+len(text) > 2*maxCapturedOutput {
		tail := append([]byte(nil), b.buffer.Bytes()[b.buffer.Len()-maxCapturedOutput:]...)
		b.buffer.Reset()
		b.buffer.Write(tail)
	}
	b.buffer.WriteString(text)
}

func (b *boundedOutput) String() string {
	data := b.buffer.Bytes()
	if len(data) > maxCapturedOutput {
		data = data[len(data)-maxCapturedOutput:]
	}
	return string(data)
}

// ProcessLineWriter is a bounded CR/LF line writer for an os/exec output copier.
// Each writer belongs to one output stream. Flush/ReadError are called only
// after cmd.Wait has joined the copy goroutine; callbacks must return promptly.
type ProcessLineWriter struct {
	pending []byte
	output  boundedOutput
	onLine  func(string)
	onError func()
	err     error
}

func NewProcessLineWriter(onLine func(string), onError func()) *ProcessLineWriter {
	return &ProcessLineWriter{onLine: onLine, onError: onError}
}

func (w *ProcessLineWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	total := len(data)
	for len(data) > 0 {
		n := bytes.IndexAny(data, "\r\n")
		end := n
		if n < 0 {
			end = len(data)
		}
		if len(w.pending)+end >= maxCapturedOutput {
			w.err = errors.New("process output line exceeds 1 MiB")
			w.pending = nil
			if w.onError != nil {
				w.onError()
			}
			return total - len(data), w.err
		}
		w.pending = append(w.pending, data[:end]...)
		if n < 0 {
			break
		}
		w.emit()
		data = data[n+1:]
	}
	return total, nil
}

func (w *ProcessLineWriter) emit() {
	line := string(w.pending)
	w.output.append(line + "\n")
	w.pending = w.pending[:0]
	if w.onLine != nil {
		w.onLine(line)
	}
}

func (w *ProcessLineWriter) Flush() {
	if w.err == nil && len(w.pending) > 0 {
		w.emit()
	}
}

func (w *ProcessLineWriter) ReadError() error { return w.err }
