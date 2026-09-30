# Safe Tuning signed metadata feed

DroidSphere can optionally overlay its built-in Safe Tuning profiles with a
separately hosted metadata feed. The feed is opt-in: the application ships
with no remote catalog URL or trust key, performs no automatic metadata
download, and never applies package changes after a metadata update.

This keeps the MIT application separate from external datasets whose licenses
may differ.

## Trust model

A feed is accepted only when all of the following are true:

- the configured feed URL uses HTTPS and contains no embedded credentials;
- the user has pinned a 32-byte Ed25519 public key in DroidSphere;
- the envelope key id matches that public key;
- the Ed25519 signature verifies over the exact payload bytes;
- the payload uses the supported schema and passes strict JSON validation;
- feed-level and profile-level source/license metadata are present;
- profile ids, package names, matching criteria and risk values are valid;
- caution/dangerous/blocked rules are never selected by default;
- DroidSphere's hard-protected Android package floor cannot be made actionable.

Verified payloads are cached under a namespace derived from the feed URL and
public key. The active and previous signed revisions are retained so the user
can explicitly roll back. A remote refresh cannot silently downgrade an active
revision or reuse the same revision number with different signed content.

If configuration, download, signature, schema or cache validation fails,
DroidSphere continues with the built-in catalog.

## No automatic device changes

Metadata refresh and package tuning are separate operations. Refreshing a feed
only changes the catalog used for the next analysis. The user must still:

1. select/analyze the active device;
2. review the profile source, license and package classifications;
3. select packages;
4. acknowledge caution items when applicable;
5. confirm the operation.

The existing backend risk checks and hard-protected package floor run again at
apply time, and DroidSphere writes a restore snapshot before any package
change.

## Feed payload schema

The signing tool accepts the unsigned payload JSON. Example:

```json
{
  "schemaVersion": 1,
  "revision": 1,
  "version": "2026.09.30",
  "generatedAt": "2026-09-30T09:00:00Z",
  "sourceName": "Example DroidSphere metadata",
  "sourceUrl": "https://example.com/droidsphere-metadata",
  "sourceLicense": "MIT",
  "profiles": [
    {
      "id": "example-oem",
      "name": "Example OEM",
      "deviceFamily": "Example Android",
      "description": "Example conservative profile.",
      "sourceName": "Example dataset",
      "sourceUrl": "https://example.com/dataset",
      "sourceLicense": "MIT",
      "criteria": {
        "manufacturers": ["example"]
      },
      "keep": ["com.example.core"],
      "rules": [
        {
          "packageName": "com.example.optional",
          "label": "Optional app",
          "category": "Optional",
          "risk": "safe",
          "reason": "Optional example component.",
          "defaultSelected": true
        }
      ]
    }
  ]
}
```

`revision` is the monotonic anti-rollback counter. `version` is the
human-readable release identifier.

## Generate the signing key

Build or run the repository tool locally:

```text
go run ./cmd/droidsphere-feed keygen --private ./private.key --public ./public.key
```

The private key file is created with owner-only permissions where the platform
supports them and the tool refuses to overwrite an existing key. Keep it
offline or in an appropriate secret-management system. **Never commit the
private key.**

The public key is not secret. Its base64 contents are what users pin in the
Safe Tuning screen.

## Sign a release

Create the unsigned payload, increment `revision`, then run:

```text
go run ./cmd/droidsphere-feed sign --private ./private.key --payload ./payload.json --out ./safe-tuning-feed.json
```

The tool validates the payload before signing. Invalid package/risk/schema
content is rejected rather than signed. Publish only the resulting envelope
file over HTTPS, ideally as a versioned release asset in a metadata-only
repository.

## Configure DroidSphere

In **Safe Tuning → Signed metadata feed**:

1. enter the HTTPS URL of the signed envelope;
2. paste the base64 Ed25519 public key;
3. choose **Save trust**;
4. choose **Check update**.

DroidSphere shows the verified version, revision, key id, SHA-256 payload
digest, source/license and profile count. **Roll back** swaps to the previous
verified cached revision. **Built-ins only** removes the configured remote
trust without deleting the application-bundled profiles.

## Licensing boundary

Every feed and every profile must declare its source and license. This
mechanism intentionally does not bundle UAD-NG or another copyleft dataset into
the MIT repository. A separately maintained feed may consume external data
only when its maintainer has established an appropriate licensing basis and
preserves the required attribution/license terms.
