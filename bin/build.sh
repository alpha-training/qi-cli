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

if [ "$1" == "dev" ]; then
    echo "🛠️  Running DEV build for local OS only..."
    # Detect local OS and Arch
    LOCAL_OS=$(go env GOOS)
    LOCAL_ARCH=$(go env GOARCH)
    
    # Use CGO=1 for local dev so the 'Doctor' can talk to local libs
    build_bin $LOCAL_OS $LOCAL_ARCH "" 1
else
    echo "🚀 Running FULL release build..."
    # Mac (ARM & Intel) - CGO enabled for SSL Doctor
    build_bin darwin arm64 "-mac-arm64" 1
    build_bin darwin amd64 "-mac-x64" 1
    
    # Windows
    build_bin windows amd64 ".exe" 0
    
    # Linux - CGO disabled for maximum portability/static linking
    build_bin linux amd64 "-linux-x64" 0
fi

rm qi.q
echo "✅ Done!"