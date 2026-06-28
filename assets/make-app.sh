#!/usr/bin/env bash
# Bygger den frittstående macOS-appen "EnkRegnskap.app" (native WKWebView).
# Krever macOS med Xcode command line tools (clang + WebKit).
set -euo pipefail

cd "$(dirname "$0")/.."

APP="dist/EnkRegnskap.app"
BIN="EnkRegnskap"
# Versjon hentes fra VERSION-miljøvariabel (f.eks. release-tag "v1.2.3"),
# uten ledende "v". Faller tilbake til 1.0 ved lokal bygging.
VERSION="${VERSION:-1.0}"
VERSION="${VERSION#v}"

echo "Bygger desktop-binær (CGo + WebKit) ..."
CGO_ENABLED=1 go build \
  -ldflags "-X github.com/kkollsga/enk-regnskap/internal/core.Version=$VERSION" \
  -o "/tmp/$BIN" ./cmd/desktop

echo "Pakker $APP ..."
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "/tmp/$BIN" "$APP/Contents/MacOS/$BIN"
chmod +x "$APP/Contents/MacOS/$BIN"
cp assets/icon.icns "$APP/Contents/Resources/icon.icns"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>            <string>ENK Regnskap</string>
  <key>CFBundleDisplayName</key>     <string>ENK Regnskap</string>
  <key>CFBundleIdentifier</key>      <string>no.kkollsga.enk-regnskap</string>
  <key>CFBundleVersion</key>         <string>$VERSION</string>
  <key>CFBundleShortVersionString</key> <string>$VERSION</string>
  <key>CFBundleExecutable</key>      <string>$BIN</string>
  <key>CFBundleIconFile</key>        <string>icon</string>
  <key>CFBundlePackageType</key>     <string>APPL</string>
  <key>LSMinimumSystemVersion</key>  <string>11.0</string>
  <key>NSHighResolutionCapable</key> <true/>
  <key>LSApplicationCategoryType</key> <string>public.app-category.finance</string>
</dict>
</plist>
PLIST

rm -f "/tmp/$BIN"
echo "Ferdig: $APP"
echo "Åpne med:  open \"$APP\"   (eller flytt den til /Applications)"
