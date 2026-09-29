#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

make_version="$(awk '/^VERSION :=/ {print $3; exit}' "$ROOT_DIR/Makefile")"
config_version="$(grep -E '^[[:space:]]+version:' "$ROOT_DIR/build/config.yml" | head -n 1 | sed -E 's/^[[:space:]]+version:[[:space:]]*"([^"]+)".*/\1/')"
core_version="$(sed -n 's/^const Version = "\([^"]*\)"[[:space:]]*$/\1/p' "$ROOT_DIR/internal/core/version.go" | head -n 1)"

if [[ -z "$make_version" || -z "$config_version" || -z "$core_version" ]]; then
  echo "Version consistency check failed: unable to read one or more version sources." >&2
  echo "Makefile: ${make_version:-<missing>}" >&2
  echo "build/config.yml info.version: ${config_version:-<missing>}" >&2
  echo "internal/core/version.go: ${core_version:-<missing>}" >&2
  exit 1
fi

if [[ "$make_version" != "$config_version" || "$make_version" != "$core_version" ]]; then
  echo "Version mismatch:" >&2
  echo "  Makefile VERSION:                 $make_version" >&2
  echo "  build/config.yml info.version:    $config_version" >&2
  echo "  internal/core/version.go Version: $core_version" >&2
  exit 1
fi

if [[ ! "$make_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version '$make_version' is not a stable three-part version." >&2
  exit 1
fi

echo "Version consistency OK: $make_version"
