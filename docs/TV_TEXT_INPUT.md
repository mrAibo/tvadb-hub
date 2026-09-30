# TV text transport

The TV panel uses Android clipboard shell access only when `cmd clipboard help`
advertises `set` and `get`. It sets the quoted text, checks exact Unicode readback,
then sends PASTE. A zero exit code is insufficient: stock Android's unimplemented
Binder shell command can exit successfully while reporting no implementation.
Unsupported commands and stale readback never trigger PASTE.

When clipboard access is unavailable, the fallback accepts printable ASCII
without percent signs. It never sends Unicode through `adb shell input text`.
Failure is reported to the user. Focused fields may still ignore PASTE or key
events; command/readback checks cannot prove what an arbitrary app displays.

DroidSphere starts the official external scrcpy program, which owns its own
control channel. The app's existing clipboard facade uses Android shell access;
it does not implement scrcpy's `SET_CLIPBOARD` protocol. The TV panel therefore
does not label that path as scrcpy transport or infer Unicode support from an
active scrcpy session. Users can use scrcpy's native paste shortcut in its window
when supported by their device/session. A future native control-channel adapter
must be integrated with the existing service and separately tested before the
panel can automatically use it.

The historical clipboard facade names and result constants remain compatible.
Unsupported shell access now fails explicitly rather than claiming success.
