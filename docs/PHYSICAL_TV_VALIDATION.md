# Physical Google TV validation

This checklist is the final manual gate before a DroidSphere Windows release.
Automated CI can verify the application and Windows packaging, but it cannot
prove behavior against a real Android/Google TV on a home network.

## Preconditions

- Windows PC and TV are on the same LAN.
- Developer options are enabled on the TV.
- Wireless debugging is enabled.
- DroidSphere uses a current managed or selected ADB binary.
- For a first-time connection, keep the TV pairing-code screen open.

## 1. Discovery and first pairing

1. Open **Discover / Pair / Connect**.
2. Confirm the TV appears from mDNS discovery.
3. Confirm the UI distinguishes the pairing endpoint from the normal connect
   endpoint.
4. Enter only the six-digit pairing code.
5. Confirm pairing succeeds.
6. Confirm DroidSphere then connects to the separate
   `_adb-tls-connect._tcp` endpoint.
7. Confirm the connected device enters the ready state.

Record a failure if the user has to manually discover or copy the dynamic
connect port.

## 2. Reconnect

1. Close DroidSphere.
2. Restart Wireless debugging on the TV so the dynamic port can change.
3. Start DroidSphere again.
4. Confirm the remembered TV is rediscovered and reconnects with the new port.
5. Use the reconnect UI to confirm the remembered-device state and last-seen
   information are sensible.

## 3. Read-only validation report

Open:

**Settings → Physical Google TV validation**

Run **Validate connected TV**.

Expected result:

- ADB connection: pass
- Android / Google TV classification: pass
- Wireless ADB transport: pass
- Secure mDNS endpoint: pass
- Wireless diagnostics: pass
- TV metadata: pass or, at minimum, no blocking failure

Copy the JSON report and retain it with the release test notes.

## 4. TV remote

Verify:

- D-pad up/down/left/right
- Select
- Home
- Back
- Play/Pause
- Volume up/down
- Mute

Power/sleep actions should be tested deliberately because they change the TV
power state.

## 5. APK workflow

With a test APK:

1. Install a new APK.
2. Replace/update it.
3. Verify the installed package appears in Apps.
4. Launch and force-stop it.
5. Test enable/disable.
6. Test uninstall.

With a known split APK package, verify base + split installation through the
multi-file flow.

## 6. Screen and debugging tools

Verify:

- screenshot saves a valid PNG;
- Scrcpy opens with the TV Balanced preset;
- Logcat starts, filters and exports;
- safe TV shell shortcuts return output;
- File Explorer can push a small file to `/sdcard/`;
- File Explorer can pull the same file back.

## 7. Windows package smoke test

On a clean or disposable Windows user profile:

1. Run the portable executable.
2. Complete first-run tool setup.
3. Run the NSIS installer.
4. Start the installed application from the shortcut.
5. Confirm the same TV reconnects.
6. Uninstall and confirm the application and created shortcuts are removed.

## Release gate

The roadmap item **Validate against a physical Google TV** should only be marked
complete after the checks above have been performed against a real device and
the blocking failures have been resolved.
