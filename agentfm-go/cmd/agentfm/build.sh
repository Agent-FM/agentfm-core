#!/bin/bash

mkdir -p build

rm -rf build/*

TARGETS=(
    "darwin/amd64"   # Mac Intel
    "darwin/arm64"   # Mac Apple Silicon (M1/M2/M3)
    "linux/amd64"    # Linux 64-bit (Standard Servers/PCs)
    "linux/arm64"    # Linux ARM 64-bit 
    "linux/386"      # Linux 32-bit (Legacy PCs)
    "linux/arm"      # Linux ARM 32-bit (Older Raspberry Pi)
    "linux/riscv64"  # Linux RISC-V 64-bit 
    "windows/amd64"  # Windows 64-bit 
)

echo "🚀 Starting cross-compilation matrix..."

for TARGET in "${TARGETS[@]}"; do
    GOOS=$(echo $TARGET | cut -d '/' -f 1)
    GOARCH=$(echo $TARGET | cut -d '/' -f 2)
    OUTPUT_NAME="build/agentfm_${GOOS}_${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi
    echo "⏳ Building for $GOOS ($GOARCH)..."
    # Build the package, not main.go alone: cmd/agentfm is split across
    # several files (help.go, relay.go, ...) that main.go depends on.
    env GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUTPUT_NAME" .
    if [ $? -ne 0 ]; then
        echo "❌ Failed to build for $GOOS/$GOARCH"
    else
        echo "✅ Built: $OUTPUT_NAME"
    fi
done

# Ship the installer with the release so install links can point at a
# published release asset instead of the main branch.
cp ../../install.sh build/install.sh

# SHA256SUMS lets install.sh (and anyone downloading by hand) verify a
# binary before running it. Upload it with the other assets.
echo "🔐 Writing build/SHA256SUMS..."
(
    cd build
    if ! ls agentfm_* >/dev/null 2>&1; then
        echo "❌ No binaries in build/; not writing SHA256SUMS."
        exit 1
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum agentfm_* install.sh > SHA256SUMS
    else
        shasum -a 256 agentfm_* install.sh > SHA256SUMS
    fi
)

echo "🎉 All builds complete! Check the /build folder."
echo "   Upload every file in build/, including SHA256SUMS and install.sh, to the GitHub release."