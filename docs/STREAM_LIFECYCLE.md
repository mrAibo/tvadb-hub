# Process output and Logcat lifecycle

Streaming subprocesses and Logcat assign bounded line writers to `os/exec`,
which owns the stdout/stderr copy goroutines. `Wait` joins those readers before
the caller flushes final partial lines and publishes the final result/batch.
They do not call `Wait` on a command with unread `StdoutPipe`/`StderrPipe` data.

Each channel retains a diagnostic tail of at most 1 MiB (not full transfer
output). Lines are split on CR and LF, including chunk boundaries. A line of
1 MiB or more is an explicit reader failure and cancels the child, rather than
silently reporting a successful operation. Streaming callbacks are serialized
between stdout/stderr and must return promptly. A central `WaitDelay` bounds
draining inherited pipe descriptors; drain failures are errors, not success.

Unix PTY streaming retains the same bounded tail and reports scanner errors.
Linux PTY EIO at ordinary EOF is not a failure. Cancellation closes the PTY and
uses existing process-tree termination. PTY output remains merged in stdout;
pipe execution retains separate stderr diagnostics.

Logcat preserves 75 ms / 128 entry batches, channel ordering and final partial
flush before the terminal status. Start/restart/stop/shutdown are serialized;
Stop returns after the child is reaped, output copiers finish and batches flush.
Context cancellation and explicit stop do not generate false scanner errors.
Reader failures do generate an error entry and error status. A stream is removed
from the registry only after final status, so a restart cannot be overwritten
by a late terminal event from the previous stream.

Tests use harmless helper subprocesses for dual-channel bursts, final partial
lines, oversized output, cancellation and Logcat lifecycle ordering. Unit tests
are not evidence of physical-device throughput or firmware compatibility.
