# File transfer operation boundaries

DroidSphere continues to use official ADB CLI/Sync for transfers. Exactly one
push/pull operation is admitted at a time; overlaps fail before sending transfer
commands. A batch owns one cancellation context and operation ID. Device serial,
ADB path, compression capabilities/preference and verification preference are
captured once and kept for every file, retry and hash. Changing UI selection or
settings cannot retarget an in-flight batch. New device-scoped APIs require the
confirmed serial; legacy facade signatures remain compatible.

Cancellation/progress includes an operation ID and target. Stale ID cancellation
does not affect a newer operation. A release is idempotent and cannot clear a
new operation's cancellation function. The UI rejects overlapping starts and
does not clear a new device's selection when an old batch completes.

## Results

Detailed batch APIs return a bounded (maximum 1000 selected items) result with
target, operation ID and one item per input: success, failed, cancelled or
skipped. Completed/failed/cancelled/not-attempted counts are explicit. Ordinary
file failures do not erase earlier successes. Cancellation stops further work;
a SHA-256 mismatch stops the remaining batch. Neither outcome removes partial
data automatically. Per-item diagnostic messages are bounded to 4096 bytes.

The UI reports actual results rather than the number originally selected,
retains failed pull selections for retry, and shows the last batch with per-file
details. Historical string APIs now return an error for incomplete batches,
instead of a successful return containing failure text. Admission errors still
reject the request; detailed per-item outcomes are returned as data.

Host command arguments use native absolute paths. Remote names in batch pulls
must map to a local basename, not a host traversal or reserved path. Compression
and structured subprocess execution are unchanged.

## Retry and verification

Retries use actual bounded stdout/stderr diagnostics plus the process error,
not only generic exit status. Nil/nonzero results are not successful transfers.
The original three attempts and cancellable two-second retry delay remain.

Hashing distinguishes unsupported host types (directory/symlink/non-regular:
unavailable, never followed) from missing/unreadable/changed host files (failure).
Host identity, size and modification time are rechecked after hashing. Remote
file/permission errors are not mistaken for missing hash commands. Exit 127 or
specific sha256sum/toybox command absence is unavailable; malformed output and
other execution errors fail. Remote hashing has a ten-minute execution limit
and shares cancellation with the transfer. Verification failure is explicit in
progress; no unavailable/failure state is represented as a verified match.

Fake-runner tests cover target/tool/settings capture, operation ownership,
partial/cancelled results, stderr-driven retry and hash classifications. Frontend
tests cover truthful counts, overlapping starts, cancellation and old-target
completion. Physical USB/Wireless transfers and firmware behavior still require
real-device validation; cross-compilation is not runtime validation.
