#!/usr/bin/env bash
# ENK Regnskap – installer for macOS.
# Bruk:  curl -fsSL https://kkollsga.github.io/enk-regnskap/install.sh | bash
#
# Laster ned siste release-DMG som passer maskinens arkitektur, kopierer
# EnkRegnskap.app til Programmer og fjerner eventuelt karanteneflagg.
set -euo pipefail

REPO="kkollsga/enk-regnskap"
APP="EnkRegnskap.app"

say() { printf '\033[1;34m▸\033[0m %s\n' "$1"; }
die() { printf '\033[1;31m✗ %s\033[0m\n' "$1" >&2; exit 1; }

[[ "$(uname -s)" == "Darwin" ]] || die "Dette skriptet er kun for macOS."

case "$(uname -m)" in
  arm64) ARCH="arm64" ;;
  x86_64) ARCH="intel" ;;
  *) die "Ukjent arkitektur: $(uname -m)" ;;
esac

say "Finner siste versjon av ENK Regnskap ($ARCH) ..."
RELEASE="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")"
TAG="$(printf '%s' "$RELEASE" | grep -o '"tag_name"[^,]*' | head -1 | sed -E 's/.*: *"([^"]+)".*/\1/')"
URL="$(printf '%s' "$RELEASE" \
  | grep -o "https://github.com/$REPO/releases/download/[^\"]*-$ARCH\.dmg" \
  | head -1)"

if [[ -z "${URL:-}" ]]; then
  die "Fant ingen $ARCH-DMG i siste release. Se https://github.com/$REPO/releases – eller bygg fra kilde med 'make dmg'."
fi

TMP="$(mktemp -d)"
trap 'hdiutil detach "$MOUNT" >/dev/null 2>&1 || true; rm -rf "$TMP"' EXIT
DMG="$TMP/EnkRegnskap.dmg"

say "Laster ned ${TAG:-siste versjon} ..."
curl -fsSL "$URL" -o "$DMG"

say "Monterer diskbildet ..."
# Tab-separert utdata; mount-punktet er felt 3 og kan inneholde mellomrom.
MOUNT="$(hdiutil attach "$DMG" -nobrowse -readonly | tail -1 | cut -f3-)"
[[ -d "$MOUNT/$APP" ]] || die "Fant ikke $APP i diskbildet."

DEST="/Applications"
if [[ ! -w "$DEST" ]]; then
  DEST="$HOME/Applications"
  mkdir -p "$DEST"
fi

say "Installerer til $DEST ..."
rm -rf "$DEST/$APP"
cp -R "$MOUNT/$APP" "$DEST/"

# curl setter ikke karanteneflagg, men vi rydder for sikkerhets skyld.
xattr -dr com.apple.quarantine "$DEST/$APP" 2>/dev/null || true

say "Ferdig! ENK Regnskap ${TAG:-} er installert i $DEST."
open "$DEST/$APP" 2>/dev/null || true
