# Read-only image and text previews

The dual-pane File Manager includes a preview action for supported **regular files** on both the host PC and the selected Android device. Preview is non-destructive and does not enable elevated ADB access, change HOME, change package state, or transfer files into permanent host storage.

## Supported formats

- UTF-8 text with no NUL bytes: `.txt`, `.md`, `.log`, `.csv`, `.json`, `.xml`, `.yaml`, `.yml`, `.ini`, `.conf`, `.cfg`, `.properties`, `.go`, `.ts`, `.tsx`, `.js`, `.jsx`, `.py`, `.sh`, `.css`, `.html` and `.htm`. Text is rendered as inert React text, **not HTML**.
- Images: PNG, JPEG, GIF and WebP. The backend validates the file's actual content type; SVG is deliberately excluded because it may contain active content.

The backend limits both host and remote reads to **2 MiB**, including files growing during an open. The frontend renders at most 200,000 text characters for responsiveness. Unsupported or binary formats are rejected without uploading to an external service.

## Security and device binding

Host reading uses `os.Lstat` to reject symlinks, directories and special files. Android reading uses the official `adb -s <serial> exec-out cat <quoted-path>` transport with a timeout, bounded capture and process-tree cancellation if the preview grows beyond the limit. No host shell is involved. The client sends the selected device serial explicitly and discards replies belonging to a device no longer selected, closing open previews on device change.

## Testing and limitations

Go unit tests exercise text/image content checks, oversized reads, binary rejection, target binding and remote read limit. Frontend unit tests exercise inert text rendering, errors and preview-action availability. Cross-platform CI is required before merge. Actual Android TV, Fire TV and firmware access tests remain a separate hardware gate; the firmware may deny reading certain paths, in which case the preview reports an error instead of escalating privileges.
