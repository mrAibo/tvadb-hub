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

Apply and restore are serialized in the backend. The current snapshot format
still has limitations around interruption between device changes and snapshot
updates; the following journal package will address interrupted-operation recovery.
