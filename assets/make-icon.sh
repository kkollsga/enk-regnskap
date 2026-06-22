#!/usr/bin/env bash
# Regenererer app-ikonet fra assets/icon.html via headless Chrome, og bygger
# assets/icon.icns + web/static/favicon.png. Krever Google Chrome og macOS
# (sips + iconutil).
set -euo pipefail
cd "$(dirname "$0")"

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$CHROME" ] || CHROME="$(command -v google-chrome || command -v chromium || true)"
if [ -z "$CHROME" ]; then echo "Fant ikke Chrome – kan ikke rendre ikon"; exit 1; fi

"$CHROME" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=1 \
  --screenshot="icon-1024.png" --window-size=1024,1024 \
  --default-background-color=00000000 "file://$PWD/icon.html" 2>/dev/null

rm -rf EnkRegnskap.iconset && mkdir EnkRegnskap.iconset
for sz in 16 32 128 256 512; do
  sips -z $sz $sz icon-1024.png --out "EnkRegnskap.iconset/icon_${sz}x${sz}.png" >/dev/null
  d=$((sz*2))
  sips -z $d $d icon-1024.png --out "EnkRegnskap.iconset/icon_${sz}x${sz}@2x.png" >/dev/null
done
iconutil -c icns EnkRegnskap.iconset -o icon.icns
sips -z 256 256 icon-1024.png --out ../web/static/favicon.png >/dev/null
rm -rf EnkRegnskap.iconset
echo "Ferdig: assets/icon.icns + web/static/favicon.png"
