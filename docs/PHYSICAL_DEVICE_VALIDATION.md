# Physical Android device validation

This checklist is the final manual gate before publishing a DroidSphere Windows release.
Automated CI verifies builds and packaging, but a real Android device is still required
to validate authorization, device interaction and the release candidate end to end.

## Preconditions

- A real Android phone, tablet, TV or streaming device.
- Developer options enabled.
- USB debugging or Wireless debugging enabled.
- DroidSphere uses a current managed or selected ADB binary.

## Core validation

1. Connect the device by USB or Wireless ADB.
2. Confirm it appears in the device selector and becomes the active target.
3. Open **Settings → Physical Android device validation**.
4. Run **Validate connected device**.
5. Expected blocking checks:
   - ADB connection: pass
   - Device classification: pass
   - ADB diagnostics: pass or no blocking failure
   - Device metadata: pass or warning only
6. Wireless-only checks such as network transport and secure mDNS may be warnings
   during a valid USB-only test.

Copy the JSON report and retain it with the release test notes.

## Device workflows

Verify on the selected device, where supported:

- app listing and launch/force-stop;
- APK install and uninstall with a test package;
- screenshot;
- scrcpy session;
- shell command;
- Logcat start/filter/export;
- dual-pane file push and pull;
- settings backup export/import.

For Android TV targets, additionally test D-pad, Home, Back, media and volume controls.

## Windows package smoke test

1. Run the portable DroidSphere executable.
2. Complete first-run tool setup.
3. Run the NSIS installer.
4. Start the installed application from its shortcut.
5. Reconnect the same Android device.
6. Uninstall and confirm the application and shortcuts are removed.

## Release gate

Publish only after at least one real Android device has completed the core validation
and the intended release workflows without unresolved blocking failures.

## Recording a successful release gate

After all blocking checks and the intended Windows package smoke test pass for a specific release candidate:

1. Retain the validation JSON and test notes with the release evidence. Avoid committing device-identifying data to the public repository.
2. Set the repository Actions variable `PHYSICAL_DEVICE_VALIDATED_VERSION` to the exact version that was tested, for example `0.1.0`.
3. Publish only that version. A later version requires a new physical validation and a new variable value.

The release workflow enforces this value for both manual publication and tag-triggered publication. Building an unpublished release candidate remains possible without it.

