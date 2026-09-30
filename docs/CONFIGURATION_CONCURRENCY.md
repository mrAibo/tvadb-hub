# Configuration concurrency

The application owns mutable settings under its mutex. Services receive detached
configuration snapshots, including copied maps and slices. Binary-path closures
call a synchronized snapshot resolver rather than retaining a live config pointer.
Snapshot callers may modify their copy without affecting application settings.

Configuration mutation and persistence remain within the same lock boundary.
Nickname access returns a copy. Audit-enabled and capability reads are synchronized.
Device discovery runs outside the application lock, because resolving its tool
paths also acquires that lock. This prevents recursive-lock deadlock during device
selection. Settings backup formats, legacy paths and internal module names remain
compatible.

CI includes the application facade in its race-detector packages. Tests exercise
concurrent settings/path reads and writes, detached maps/slices and device
selection using the synchronized resolver without a physical device.
