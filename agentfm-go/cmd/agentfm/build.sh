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
    env GOOS=$GOOS GOARCH=$GOARCH go build -o $OUTPUT_NAME main.go
    if [ $? -ne 0 ]; then
        echo "❌ Failed to build for $GOOS/$GOARCH"
    else
        echo "✅ Built: $OUTPUT_NAME"
    fi
done

echo "🎉 All builds complete! Check the /build folder."