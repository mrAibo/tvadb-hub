# Dynamic HOME / IME recovery protection

Safe Tuning's static and device-profile safety floors remain non-overridable.
They now also include read-only device discovery for the pinned Android user 0:

- All HOME activities returned by `cmd package query-activities --components`.
- Current HOME resolution from `cmd package resolve-activity --components`.
- Selected and enabled input-method packages from user-0 secure settings.

This protects unknown OEM launchers/keyboards and the currently enabled backup
recovery paths, regardless of their profile/feed IDs or claimed risk labels.
Analysis marks installed matches protected/non-actionable, and apply performs
fresh discovery again before each disabling/uninstall-user action. The Go
boundary, not frontend selection, decides eligibility. No result is cached.

Discovery requires explicit component-only HOME output and valid IME settings.
Explicit `null` means no configured IME; empty, malformed, ambiguous,
unsupported, permission-denied or failed output blocks disabling operations.
There is no unscoped Android-user fallback. Conservative refusal on unsupported
firmware is intentional; inspect the error instead of guessing package names.

An old/imported snapshot cannot restore a disabled state onto today's HOME/IME.
Ordinary exact-state recovery is retained. Re-enabling/reinstalling a package
does not require working HOME discovery, so a broken launcher does not prevent
recovery. Restoring disabled states requires the live safety check.

Each read has a five-second deadline. Operations pin serial/tool paths. These
checks cannot atomically lock external Android settings: another device-side
actor can change HOME/IME between a final read and a package command. Do not
perform concurrent launcher/keyboard changes during tuning. No physical
firmware support is claimed from fake-runner tests.

Protocol references (primary sources):

- [AOSP PackageManagerShellCommand, Android 8.1](https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-8.1.0_r68/services/core/java/com/android/server/pm/PackageManagerShellCommand.java): component-only query/resolve output and explicit user selection.
- [AOSP InputMethodManagerService](https://android.googlesource.com/platform/frameworks/base/+/master/services/core/java/com/android/server/inputmethod/InputMethodManagerService.java): selected/enabled IME secure settings and user scoping.

This is not authorization for a universal launcher replacement command. The
separate guarded wizard still needs capability/firmware checks, durable rollback
state, explicit confirmation and post-change verification.
