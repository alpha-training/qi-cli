#!/bin/bash

APP_NAME="qi"
DIST_DIR="dist"
cp ../qi/qi.q .
mkdir -p $DIST_DIR

# Function to build and set permissions
build_bin() {
    local os=$1
    local arch=$2
    local suffix=$3
    local cgo=$4

    echo "📦 Building for $os-$arch (CGO_ENABLED=$cgo)..."
    CGO_ENABLED=$cgo GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o $DIST_DIR/$APP_NAME$suffix .
    chmod +x $DIST_DIR/$APP_NAME$suffix
}

# Convert input to lowercase for easier matching
TARGET=$(echo "$1" | tr '[:upper:]' '[:lower:]')

case "$TARGET" in
    linux|l)
        echo "🐧 Targeting Linux..."
        build_bin linux amd64 "-linux-x64" 0
        ;;
    windows|w)
        echo "🪟 Targeting Windows..."
        build_bin windows amd64 ".exe" 0
        ;;
    mac|m)
        echo "🍎 Targeting Mac (Dual Build)..."
        build_bin darwin arm64 "-mac-arm64" 1
        build_bin darwin amd64 "-mac-x64" 1
        ;;
    dev)
        echo "🛠️  Running DEV build for local OS only..."
        build_bin $(go env GOOS) $(go env GOARCH) "" 1
        ;;
    *)
        echo "🚀 Running FULL release build..."
        build_bin darwin arm64 "-mac-arm64" 1
        build_bin darwin amd64 "-mac-x64" 1
        build_bin windows amd64 ".exe" 0
        build_bin linux amd64 "-linux-x64" 0
        ;;
esac

rm qi.q
echo "✅ Done!"