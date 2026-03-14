#!/bin/bash
# Builds the Song Converter for Linux, Windows, and macOS
# Requires: Go, Docker, and fyne-cross
# Install fyne-cross: go install github.com/fyne-io/fyne-cross@latest

set -e

echo "==> Installing/updating fyne-cross..."
go install github.com/fyne-io/fyne-cross@latest

echo ""
echo "==> Building for Linux (amd64)..."
fyne-cross linux -arch=amd64 -name=SongConverter

echo ""
echo "==> Building for Windows (amd64)..."
fyne-cross windows -arch=amd64 -name=SongConverter

echo ""
echo "==> Building for macOS (amd64 + arm64)..."
fyne-cross darwin -arch=amd64 -name=SongConverter
fyne-cross darwin -arch=arm64  -name=SongConverter

echo ""
echo "==> Done! Binaries are in the fyne-cross/dist/ folder:"
ls fyne-cross/dist/
