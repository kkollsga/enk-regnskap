#!/usr/bin/env bash
# Pakker EnkRegnskap.app i et drag-til-Applications DMG-bilde.
# Bruk: assets/make-dmg.sh [app-sti] [dmg-utfil]
set -euo pipefail

cd "$(dirname "$0")/.."

APP="${1:-dist/EnkRegnskap.app}"
OUT="${2:-dist/EnkRegnskap.dmg}"

if [[ ! -d "$APP" ]]; then
  echo "Finner ikke $APP – kjør 'make mac-app' først." >&2
  exit 1
fi

STAGING="$(mktemp -d)"
trap 'rm -rf "$STAGING"' EXIT

cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"

rm -f "$OUT"
hdiutil create \
  -volname "ENK Regnskap" \
  -srcfolder "$STAGING" \
  -fs HFS+ \
  -format UDZO -ov \
  "$OUT" >/dev/null

echo "Ferdig: $OUT"
