#!/bin/bash

APP_NAME="qi"
DIST_DIR="dist"
# The real source of your q script
SOURCE_Q_FILE="../qi/qi.q" 
# The local filename Go expects to embed
LOCAL_Q_FILE="./qi.q"

echo "🧹 Cleaning up old artifacts..."
rm -f "$LOCAL_Q_FILE"
mkdir -p "$DIST_DIR"

# 1. Physical Copy (Since go:embed doesn't follow symlinks)
if [ -f "$SOURCE_Q_FILE" ]; then
    echo "📄 Copying $SOURCE_Q_FILE to local directory for embedding..."
    cp "$SOURCE_Q_FILE" "$LOCAL_Q_FILE"
else
    echo "❌ Error: Source script $SOURCE_Q_FILE not found!"
    exit 1
fi

# Function to build
build_bin() {
    local os=$1
    local arch=$2
    local suffix=$3
    local cgo=$4

    echo "📦 Building for $os-$arch (CGO_ENABLED=$cgo)..."
    CGO_ENABLED=$cgo GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o $DIST_DIR/$APP_NAME$suffix .
    
    if [[ "$os" != "windows" ]]; then
        chmod +x $DIST_DIR/$APP_NAME$suffix
    fi
}

# Target selection
TARGET=$(echo "$1" | tr '[:upper:]' '[:lower:]')

case "$TARGET" in
    linux|l)   build_bin linux amd64 "-linux-x64" 0 ;;
    windows|w) build_bin windows amd64 ".exe" 0 ;;
    mac|m)
        build_bin darwin arm64 "-mac-arm64" 1
        build_bin darwin amd64 "-mac-x64" 1
        ;;
    dev)       build_bin $(go env GOOS) $(go env GOARCH) "" 1 ;;
    *)
        build_bin darwin arm64 "-mac-arm64" 1
        build_bin darwin amd64 "-mac-x64" 1
        build_bin windows amd64 ".exe" 0
        build_bin linux amd64 "-linux-x64" 0
        ;;
esac

# 2. Cleanup: Remove the temporary copy so it doesn't clutter your repo
# echo "🧹 Removing temporary local copy of $LOCAL_Q_FILE..."
# rm -f "$LOCAL_Q_FILE"

echo "✅ Done!"