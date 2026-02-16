#!/bin/bash

APP_NAME="qi"
DIST_DIR="dist"
cp ../qi/qi.q .

# Create the output folder
mkdir -p $DIST_DIR

echo "🚀 Building regular binaries..."

# 1. Mac Apple Silicon (M1/M2/M3)
echo "🍎 Building Mac (ARM64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $DIST_DIR/$APP_NAME-mac-arm64 .

# 2. Mac Intel
echo "💻 Building Mac (Intel)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $DIST_DIR/$APP_NAME-mac-x64 .

# 3. Windows 64-bit
echo "🪟 Building Windows (x64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $DIST_DIR/$APP_NAME.exe .

# 4. Linux 64-bit
echo "🐧 Building Linux (x64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $DIST_DIR/$APP_NAME-linux-x64 .

rm qi.q

echo "✅ Done! Binaries are in the /$DIST_DIR folder."
