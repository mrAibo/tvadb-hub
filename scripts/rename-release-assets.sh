#!/usr/bin/env bash
set -euo pipefail

# Rename raw build outputs in bin/ to canonical versioned release names.
# APP_NAME and VERSION are read from the Makefile.
#
# Usage: rename-release-assets.sh [windows|linux|all]  (default: all)

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT_DIR/bin"
TARGET="${1:-all}"

APP_NAME="$(grep -E '^APP_NAME :=' "$ROOT_DIR/Makefile" | awk '{print $3}')"
VERSION="$(grep -E '^VERSION :=' "$ROOT_DIR/Makefile" | awk '{print $3}')"

if [ -z "$APP_NAME" ] || [ -z "$VERSION" ]; then
  echo "Error: could not read APP_NAME/VERSION from Makefile" >&2
  exit 1
fi

rename_asset() {
  local src="$1"
  local dst="$2"

  if [ "$src" = "$dst" ]; then
    return 0
  fi
  if [ ! -f "$BIN_DIR/$src" ]; then
    echo "Skipped bin/$src (not found)"
    return 0
  fi

  rm -f "$BIN_DIR/$dst"
  mv "$BIN_DIR/$src" "$BIN_DIR/$dst"
  echo "Renamed bin/$src -> bin/$dst"
}

rename_windows() {
  local portable="${APP_NAME}.exe"
  local portable_release="${APP_NAME}-${VERSION}-windows-amd64.exe"
  local installer="${APP_NAME}-amd64-installer.exe"
  local installer_release="${APP_NAME}-${VERSION}-windows-amd64-installer.exe"

  rename_asset "$portable" "$portable_release"

  if [ -f "$BIN_DIR/$installer" ]; then
    rename_asset "$installer" "$installer_release"
    return
  fi

  local discovered
  discovered="$(find "$BIN_DIR" -maxdepth 1 -type f -name "${APP_NAME}-*-installer.exe" ! -name "$installer_release" -printf '%f\n' | head -n 1)"
  if [ -n "$discovered" ]; then
    rename_asset "$discovered" "$installer_release"
  elif [ ! -f "$BIN_DIR/$installer_release" ]; then
    echo "Skipped Windows installer (not found in bin/)"
  fi
}

rename_linux() {
  local bundled
  bundled="$(find "$BIN_DIR" -maxdepth 1 -name '*.AppImage' ! -name '*-system.AppImage' ! -name "${APP_NAME}-${VERSION}-linux-amd64.AppImage" -printf '%f\n' | head -n 1)"
  if [ -n "$bundled" ]; then
    rename_asset "$bundled" "${APP_NAME}-${VERSION}-linux-amd64.AppImage"
  fi

  rename_asset "${APP_NAME}.deb" "${APP_NAME}-${VERSION}-linux-amd64.deb"
  rename_asset "${APP_NAME}.rpm" "${APP_NAME}-${VERSION}-1.x86_64.rpm"
  rename_asset "${APP_NAME}.pkg.tar.zst" "${APP_NAME}-${VERSION}-1-x86_64.pkg.tar.zst"
}

case "$TARGET" in
  windows)
    rename_windows
    ;;
  linux)
    rename_linux
    ;;
  all)
    rename_windows
    rename_linux
    ;;
  *)
    echo "Error: unknown target '$TARGET' (want windows|linux|all)" >&2
    exit 1
    ;;
esac
