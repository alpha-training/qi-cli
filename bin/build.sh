#!/bin/bash

APP_NAME="qi"
DIST_DIR="dist"
cp ../qi/qi.q .

mkdir -p $DIST_DIR

echo "🚀 Building regular binaries..."

# Build and immediately fix permissions
build_and_chmod() {
    export GOOS=$1
    export GOARCH=$2
    OUT_FILE=$3
    echo "📦 Building $OUT_FILE..."
    go build -ldflags="-s -w" -o $OUT_FILE .
    # Fix the executable bit on the host machine
    chmod +x $OUT_FILE
}

build_and_chmod darwin arm64 $DIST_DIR/$APP_NAME-mac-arm64
build_and_chmod darwin amd64 $DIST_DIR/$APP_NAME-mac-x64
build_and_chmod windows amd64 $DIST_DIR/$APP_NAME.exe
build_and_chmod linux amd64 $DIST_DIR/$APP_NAME-linux-x64

rm qi.q
echo "✅ Done!"