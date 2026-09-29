#!/usr/bin/env bash
set -euo pipefail

APP_NAME="DroidSphere"
VERSION="${VERSION:-$(awk '/^VERSION :=/ {print $3; exit}' Makefile)}"
BUILD_DIR="bin"
APPIMAGE_TOOL="build/tools/appimagetool-x86_64.AppImage"
APPIMAGE_OUT="build/AppImage-out"
DIST_DIR="bin"

if [ -z "$VERSION" ]; then
    echo "Error: Could not resolve VERSION from Makefile."
    exit 1
fi

if [ ! -f "$BUILD_DIR/$APP_NAME" ]; then
    echo "Error: Binary not found at $BUILD_DIR/$APP_NAME"
    echo "Run 'wails3 build GOOS=linux' first."
    exit 1
fi

if [ ! -f "build/linux/$APP_NAME.desktop" ]; then
    echo "Error: Desktop file not found at build/linux/$APP_NAME.desktop"
    echo "The Linux build task should generate it automatically."
    exit 1
fi

if [ ! -f "$APPIMAGE_TOOL" ]; then
    echo "Downloading appimagetool..."
    mkdir -p "$(dirname "$APPIMAGE_TOOL")"
    curl -fSL -o "$APPIMAGE_TOOL" \
        "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"
    chmod +x "$APPIMAGE_TOOL"
fi

rm -rf "$APPIMAGE_OUT"
APPDIR="$APPIMAGE_OUT/$APP_NAME.AppDir"
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

cp "$BUILD_DIR/$APP_NAME" "$APPDIR/usr/bin/"
cp "build/linux/$APP_NAME.desktop" "$APPDIR/"
cp "build/appicon.png" "$APPDIR/$APP_NAME.png"
cp "build/appicon.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/$APP_NAME.png"

cat > "$APPDIR/AppRun" << 'APPRUN'
#!/usr/bin/env bash
SELF=$(readlink -f "$0")
HERE=${SELF%/*}
export PATH="${HERE}/usr/bin:${PATH}"
export LD_LIBRARY_PATH="${HERE}/usr/lib:${LD_LIBRARY_PATH:-}"
exec "${HERE}/usr/bin/DroidSphere" "$@"
APPRUN
chmod +x "$APPDIR/AppRun"

mkdir -p "$DIST_DIR"
OUTPUT="$DIST_DIR/$APP_NAME-$VERSION-linux-x86_64-system.AppImage"
ARCH=x86_64 "$APPIMAGE_TOOL" "$APPDIR" "$OUTPUT"
echo "AppImage: $OUTPUT"
