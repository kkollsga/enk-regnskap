#!/usr/bin/env bash
# Krysskompiler ENK Regnskap for alle stottede plattformer.
# Ingen CGo, ingen system-avhengigheter.
set -euo pipefail

BINARY="enk-regnskap"
PKG="./cmd/server"
DIST="dist"

rm -rf "$DIST"
mkdir -p "$DIST"

echo "Bygger windows/amd64 ..."
GOOS=windows GOARCH=amd64 go build -o "$DIST/$BINARY-windows.exe" "$PKG"

echo "Bygger darwin/arm64 ..."
GOOS=darwin GOARCH=arm64 go build -o "$DIST/$BINARY-mac" "$PKG"

echo "Bygger linux/amd64 ..."
GOOS=linux GOARCH=amd64 go build -o "$DIST/$BINARY-linux" "$PKG"

echo "Ferdig. Binærer i $DIST/:"
ls -lh "$DIST"
