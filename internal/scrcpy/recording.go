package scrcpy

import (
	"ADBKit/internal/core"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// defaultRecordingStopGrace bounds each wait for the recording owner: first for a
// graceful interrupt, then for the bounded process-tree termination.
const defaultRecordingStopGrace = 5 * time.Second

// recordingProcess is the single-owner record of one scrcpy recording run. The
// goroutine started by StartRecording owns cmd.Wait and the exit status; every
// other method observes the record only through Service.recordingMu and done.
type recordingProcess struct {
	cmd    *exec.Cmd
	path   string
	serial string

	// manual marks a requested stop. It is written under Service.recordingMu
	// before the process is signaled and read by the owner after Wait returned.
	manual bool
	// stopping marks a requested stop whose completion the owner has not confirmed
	// yet. While it is set the record stays owned: no second stop signals it again
	// and no new recording may start on top of a process that is still running.
	stopping bool
	// done is closed by the owner once Wait returned and the slot was released.
	done chan struct{}
	// waitErr is written by the owner before done is closed, so a reader that
	// observed done can read it without further synchronization.
	waitErr error
}

func (s *Service) StartRecording(serial, outputPath string, opts Options) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	trimmedSerial := strings.TrimSpace(serial)
	if trimmedSerial == "" {
		return core.NewOperationError(
			"start_scrcpy_recording",
			"Device serial is required",
			"serial must not be empty",
			false,
		)
	}
	trimmedPath := strings.TrimSpace(outputPath)
	if trimmedPath == "" {
		return core.NewOperationError(
			"start_scrcpy_recording",
			"Output file path is required",
			"output path must not be empty",
			false,
		)
	}

	s.recordingMu.Lock()
	active := s.recording
	stopping := active != nil && active.stopping
	s.recordingMu.Unlock()
	if active != nil {
		detail := "stop current recording before starting a new one"
		if stopping {
			// The process of the previous stop is still owned: starting a new
			// recording here would mix two scrcpy processes and two output files.
			detail = "the previous stop is not confirmed complete; wait for it before starting a new recording"
		}
		return core.NewOperationError(
			"start_scrcpy_recording",
			"Recording is already in progress",
			detail,
			true,
		)
	}

	scrcpyPath, err := s.resolveBinaryPath()
	if err != nil {
		return err
	}

	adbPath, _ := s.resolveADBPath()
	if adbPath != "" && !opts.NoAudio && (opts.AudioOnly || opts.normalizedAudioSource() != core.ScrcpyAudioSourceOutput) {
		if sdk, sdkErr := detectAndroidSDK(s.ctx, adbPath, trimmedSerial); sdkErr == nil {
			if err := validateAudioCompatibility(opts, sdk); err != nil {
				return err
			}
		}
	}

	args := buildRecordingArgs(trimmedSerial, trimmedPath, opts)

	s.mu.Lock()
	activeSession := s.process
	s.mu.Unlock()
	if activeSession != nil && activeSession.session.Serial == trimmedSerial {
		args = append(args, "--port", "27199:27209")
	}

	cmd := core.NewCommandContext(s.ctx, scrcpyPath, args...)
	if adbPath != "" {
		cmd.Env = append(os.Environ(), "ADB="+adbPath)
	}
	stderrPipe, pipeErr := cmd.StderrPipe()
	if pipeErr != nil {
		return core.NewOperationError(
			"start_scrcpy_recording",
			"Failed to prepare recording process",
			pipeErr.Error(),
			true,
		)
	}

	if startErr := cmd.Start(); startErr != nil {
		return core.NewOperationError(
			"start_scrcpy_recording",
			"Failed to start scrcpy recording",
			startErr.Error(),
			true,
		)
	}

	rec := &recordingProcess{cmd: cmd, path: trimmedPath, serial: trimmedSerial, done: make(chan struct{})}

	s.recordingMu.Lock()
	if s.recording != nil {
		// A concurrent start won the slot after the early check above. Release
		// this process instead of letting two records exist for one output file.
		s.recordingMu.Unlock()
		_ = core.TerminateProcessTree(cmd)
		go func() {
			// This goroutine is the only owner of this cmd's Wait, so the losing
			// start never leaves an unreaped process or an unread pipe behind.
			_ = stderrPipe.Close()
			_ = cmd.Wait()
		}()
		return core.NewOperationError(
			"start_scrcpy_recording",
			"Recording is already in progress",
			"stop current recording before starting a new one",
			true,
		)
	}
	s.recording = rec
	s.recordingMu.Unlock()

	s.logAudit("start_scrcpy_recording", trimmedSerial, true, fmt.Sprintf("path=%s", trimmedPath))
	go s.monitorRecordingProcess(rec, stderrPipe)
	return nil
}

func buildRecordingArgs(serial string, outputPath string, opts Options) []string {
	args := []string{"--no-window", "--record", outputPath, "--serial", serial}
	if !opts.AudioOnly {
		if opts.BitRate > 0 {
			args = append(args, "--video-bit-rate", fmt.Sprintf("%d", opts.BitRate))
		}
		if opts.MaxFPS > 0 {
			args = append(args, "--max-fps", fmt.Sprintf("%d", opts.MaxFPS))
		}
		if opts.MaxSize > 0 {
			args = append(args, "--max-size", fmt.Sprintf("%d", opts.MaxSize))
		}
		if opts.VideoCodec != "" && opts.VideoCodec != "h264" {
			args = append(args, "--video-codec", opts.VideoCodec)
		}
	}
	if opts.AudioBitRate > 0 {
		args = append(args, "--audio-bit-rate", fmt.Sprintf("%d", opts.AudioBitRate))
	}
	if opts.AudioCodec != "" && opts.AudioCodec != "opus" {
		args = append(args, "--audio-codec", opts.AudioCodec)
	}
	if source := opts.normalizedAudioSource(); !opts.NoAudio && source != core.ScrcpyAudioSourceOutput {
		args = append(args, "--audio-source", source)
	}
	if opts.NoAudio {
		args = append(args, "--no-audio")
	}
	if opts.AudioOnly {
		args = append(args, "--no-video", "--no-control")
	}
	return args
}

// monitorRecordingProcess is the single owner of one recording process: it drains
// stderr, calls cmd.Wait exactly once, publishes the truthful outcome and then
// closes done. StopRecording never calls Wait; it only waits for this owner.
func (s *Service) monitorRecordingProcess(rec *recordingProcess, stderrPipe io.ReadCloser) {
	var stderrBuf strings.Builder
	buf := make([]byte, 1024)
	for {
		n, readErr := stderrPipe.Read(buf)
		if n > 0 {
			stderrBuf.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}
	_ = stderrPipe.Close()

	rec.waitErr = rec.cmd.Wait()

	s.recordingMu.Lock()
	manualStop := rec.manual
	stillCurrent := s.recording == rec
	if stillCurrent {
		s.recording = nil
	}
	s.recordingMu.Unlock()

	close(rec.done)

	if manualStop {
		// A requested stop is reported by StopRecording itself; an interrupt we
		// asked for is not an unexpected recording exit.
		return
	}
	if rec.waitErr == nil {
		return
	}

	detail := strings.TrimSpace(stderrBuf.String())
	if detail == "" {
		detail = rec.waitErr.Error()
	}
	if stillCurrent {
		application.Get().Event.Emit(EventError, SessionEvent{
			Status:  StatusError,
			Message: "Recording stopped unexpectedly: " + detail,
		})
		return
	}
	s.logAudit("recording_exit", rec.serial, false, fmt.Sprintf("exit_err=%v stderr=%s", rec.waitErr, stderrBuf.String()))
}

func (s *Service) StopRecording() (string, error) {
	s.recordingMu.Lock()
	rec := s.recording
	if rec != nil {
		if rec.stopping {
			// A previous stop is still unconfirmed: never signal a second time and
			// never report the still-owned process as "no active recording".
			s.recordingMu.Unlock()
			return "", core.NewOperationError(
				"stop_scrcpy_recording",
				"Recording stop is already in progress",
				"the previous stop is not confirmed complete; the recording process is still owned",
				true,
			)
		}
		// Mark the manual reason and the stop attempt under the existing mutex, but
		// keep the record owned: only the single Wait owner releases the slot, so no
		// new recording can be mixed with a process that is still running.
		rec.manual = true
		rec.stopping = true
	}
	s.recordingMu.Unlock()

	if rec == nil {
		return "", core.NewOperationError(
			"stop_scrcpy_recording",
			"No active recording found",
			"start a recording before stopping",
			true,
		)
	}

	// Request a graceful stop. Process.Signal(os.Interrupt) is unsupported on
	// Windows, so the bounded process-tree termination below is the only
	// guarantee there. The signal error is reported instead of being presented
	// as a graceful stop or a completed file finalization.
	signalErr := error(nil)
	if rec.cmd.Process != nil {
		signalErr = rec.cmd.Process.Signal(os.Interrupt)
	}

	grace := s.recordingStopGrace
	if grace <= 0 {
		grace = defaultRecordingStopGrace
	}

	completed := false
	var terminateErr error
	select {
	case <-rec.done:
		completed = true
	case <-time.After(grace):
		terminateErr = core.TerminateProcessTree(rec.cmd)
		select {
		case <-rec.done:
			completed = true
		case <-time.After(grace):
		}
	}

	if !completed {
		// The owner never confirmed that the process ended, so the record stays
		// owned and its output must not be presented as a finished file. Only facts
		// that can be read without racing the owner are reported.
		detail := fmt.Sprintf("path=%s; process completion unconfirmed after %s", rec.path, grace)
		if terminateErr != nil {
			detail += fmt.Sprintf("; process-tree termination failed: %v", terminateErr)
		}
		if signalErr != nil {
			detail += fmt.Sprintf("; graceful interrupt unavailable: %v", signalErr)
		}
		if info, statErr := os.Stat(rec.path); statErr != nil {
			detail += fmt.Sprintf("; output: %v", statErr)
		} else {
			detail += fmt.Sprintf("; output size=%d", info.Size())
		}
		s.logAudit("stop_scrcpy_recording", rec.serial, false, detail)
		return "", core.NewOperationError(
			"stop_scrcpy_recording",
			"Recording stop could not be confirmed",
			detail,
			true,
		)
	}

	// Give a tiny bit of time for the filesystem to catch up if needed
	time.Sleep(100 * time.Millisecond)

	info, statErr := os.Stat(rec.path)
	if statErr != nil {
		detail := fmt.Sprintf("path=%s err=%v", rec.path, statErr)
		if signalErr != nil {
			detail += fmt.Sprintf("; graceful interrupt unavailable: %v", signalErr)
		}
		if rec.waitErr != nil {
			detail += fmt.Sprintf("; process exit: %v", rec.waitErr)
		}
		s.logAudit("stop_scrcpy_recording", rec.serial, false, detail)
		return "", core.NewOperationError(
			"stop_scrcpy_recording",
			"Recording file not found",
			detail,
			true,
		)
	}
	if info.Size() == 0 {
		detail := "scrcpy failed to capture any frames. Ensure the device screen is on and no other scrcpy instance is using the same encoder."
		if rec.waitErr != nil {
			detail += fmt.Sprintf("; process exit: %v", rec.waitErr)
		}
		s.logAudit("stop_scrcpy_recording", rec.serial, false, detail)
		return "", core.NewOperationError(
			"stop_scrcpy_recording",
			"Recording file is empty",
			detail,
			true,
		)
	}

	s.logAudit("stop_scrcpy_recording", rec.serial, true, fmt.Sprintf("path=%s size=%d", rec.path, info.Size()))
	return rec.path, nil
}

func (s *Service) TakeScreenshot(sessionID, outputPath string) (string, error) {
	process, err := s.getSession(sessionID)
	if err != nil {
		return "", err
	}

	trimmedPath := strings.TrimSpace(outputPath)
	if trimmedPath == "" {
		return "", core.NewOperationError(
			"take_scrcpy_screenshot",
			"Output file path is required",
			"output path must not be empty",
			false,
		)
	}

	adbPath, err := s.resolveADBPath()
	if err != nil {
		return "", err
	}

	cmd := core.NewCommandContext(s.ctx, adbPath, "-s", process.session.Serial, "exec-out", "screencap", "-p")
	outFile, createErr := os.Create(trimmedPath)
	if createErr != nil {
		return "", core.NewOperationError(
			"take_scrcpy_screenshot",
			"Failed to create screenshot file",
			createErr.Error(),
			true,
		)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	if runErr := cmd.Run(); runErr != nil {
		return "", core.NewOperationError(
			"take_scrcpy_screenshot",
			"Failed to capture screenshot",
			runErr.Error(),
			true,
		)
	}

	s.logAudit("take_scrcpy_screenshot", process.session.Serial, true, fmt.Sprintf("path=%s", trimmedPath))
	return trimmedPath, nil
}
