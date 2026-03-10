#!/bin/bash

APP_NAME="qi"
DIST_DIR="dist"
SOURCE_Q_FILE="../qi/qi.q" 
LOCAL_Q_FILE="./qi.q"

# --- Cleanup & Setup ---

#cleanup() {
   # if [ -f "$LOCAL_Q_FILE" ]; then
  #      echo "🧹 Cleaning up temporary file..."
 #       rm -f "$LOCAL_Q_FILE"
#    fi
#}
# Trap ensures cleanup happens even if the script fails
#trap cleanup EXIT

echo "🚀 Starting build process..."
mkdir -p "$DIST_DIR"

# Copy source for go:embed (since it doesn't follow symlinks)
if [ -f "$SOURCE_Q_FILE" ]; then
    cp "$SOURCE_Q_FILE" "$LOCAL_Q_FILE"
else
    echo "❌ Error: Source script $SOURCE_Q_FILE not found!"
    exit 1
fi

# --- Build Function ---

build_bin() {
    local goos=$1
    local goarch=$2
    local cgo=$3
    
    # 1. Map Go OS names to your folder names
    case "$goos" in
        darwin)  os_dir="mac" ;;
        linux)   os_dir="lin" ;;
        windows) os_dir="win" ;;
        *)       os_dir="$goos" ;;
    esac

    # 2. Handle Architecture Subfolders
    # Only 'mac' uses the nested architecture structure in your manual tree
    if [[ "$os_dir" == "mac" ]]; then
        # Map amd64 to x86_64 to match your tree exactly
        local arch_dir=$(echo "$goarch" | sed 's/amd64/x86_64/')
        target_dir="$DIST_DIR/$os_dir/$arch_dir"
    else
        # 'lin' and 'win' stay flat according to your example
        target_dir="$DIST_DIR/$os_dir"
    fi

    mkdir -p "$target_dir"
    
    # 3. Handle Extensions
    local extension=""
    [[ "$goos" == "windows" ]] && extension=".exe"
    
    local output_path="$target_dir/${APP_NAME}${extension}"

    echo "📦 Building $goos ($goarch) -> $output_path"
    
    # 4. Execute Build
    CGO_ENABLED=$cgo GOOS=$goos GOARCH=$goarch \
    go build -ldflags="-s -w" -o "$output_path" .
    
    if [ $? -eq 0 ]; then
        [[ "$goos" != "windows" ]] && chmod +x "$output_path"
    else
        echo "❌ Build failed for $goos-$goarch"
        exit 1
    fi
}

# --- Target Selection ---

# Convert input to lowercase; default to 'all' if empty
TARGET=$(echo "${1:-all}" | tr '[:upper:]' '[:lower:]')

case "$TARGET" in
    linux|l)   build_bin linux amd64 0 ;;
    windows|w) build_bin windows amd64 0 ;;
    mac|m)
        build_bin darwin arm64 1
        build_bin darwin amd64 1
        ;;
    dev)       
        build_bin $(go env GOOS) $(go env GOARCH) 1 
        ;;
    *)
        # Default 'all' behavior
        build_bin darwin arm64 1
        build_bin darwin amd64 1
        build_bin windows amd64 0
        build_bin linux amd64 0
        ;;
esac

echo "✅ Done! Check the $DIST_DIR folder."