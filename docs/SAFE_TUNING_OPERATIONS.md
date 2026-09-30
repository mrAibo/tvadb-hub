# Safe Tuning operation boundaries

Safe Tuning analysis, apply and restore use an explicit ADB serial and Android
user 0. Concurrent package queries use the same serial and tool paths. Changing
the Current+ selection after an operation starts cannot redirect its commands.
Apply requires the serial shown in the confirmation; a changed selection at
request admission is rejected. The frontend discards stale analysis responses
and never enables Apply for an analysis belonging to another device.

Existing read-only facade methods remain available. New device-scoped analysis,
snapshot listing and restore methods serve the frontend. Apply requests without
`expectedSerial` deliberately fail closed and require fresh confirmation.

Profile eligibility is separate from recommendation scoring. Manufacturer and
brand are alternative family identifiers. Model and codename are alternative
exact identifiers. A model-specific profile must match its family and at least
one exact identifier; manufacturer alone cannot select it. Matching ignores case
and surrounding whitespace, but does not use substring matches. Generic profiles
remain available when no device-specific profile is eligible.

Apply and restore are serialized in the backend and pin tool paths for their
duration. Apply confirms the hardware serial too when Android exposes one.
Restore rechecks the recorded hardware identity, preventing a reused wireless
address from restoring a different device. Snapshot lookup still uses the ADB
serial: changing wireless endpoints requires recovery through the old endpoint
or an explicitly reviewed snapshot migration; no identity guess is made.

Version 2 journals record Android user 0, installation status and the exact
enabled setting (default, enabled, disabled, disabled-user, disabled-until-used).
`dumpsys package` must provide an unambiguous state; unsupported firmware fails
closed before mutation. All original states are captured before the plan is
saved. Each item follows `planned → pending → applied → restore-pending → restored`.
An uncertain command/verification outcome is recorded as `unknown` and stops
further mutations. Journal-write failure also stops the batch immediately.

The pending intent is written and synced before each device command. If the app
stops after the command but before recording its result, Restore still sees that
item and reconciles observed state against the original and planned change.
Restore verifies each result, saves progress per item and can be repeated after
interruption. It refuses to overwrite a conflicting state changed outside the
operation. Atomic writes use unique temporary files to avoid writer collisions.

Legacy snapshots remain readable. Their enabled boolean cannot reconstruct an
original default/disabled subtype exactly. Recovery retains this limitation and
refuses unexplained states. Legacy entries with `applied:false` are reconciled
too, since the old format could omit a successfully executed change. The UI's
recovery count therefore includes uncertain entries, not just confirmed changes.

These tests use simulated package-manager responses. Physical-device validation
of dumpsys formats, firmware behavior and restart recovery is required before
claiming verified support for a particular TV family.
