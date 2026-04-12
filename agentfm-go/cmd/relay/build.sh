#!/bin/bash

mkdir -p build

TARGETS=(
    "darwin/amd64"   # Mac Intel
    "darwin/arm64"   # Mac Apple Silicon
    "linux/amd64"    # Linux 64-bit (Standard Cloud VPS)
    "linux/arm64"    # Linux ARM 64-bit (AWS Graviton)
    "linux/386"      # Linux 32-bit
    "linux/arm"      # Linux ARM 32-bit (Raspberry Pi)
    "linux/riscv64"  # Linux RISC-V 64-bit
    "windows/amd64"  # Windows 64-bit
)

echo "📡 Starting cross-compilation for Relay Server..."

for TARGET in "${TARGETS[@]}"; do
    GOOS=$(echo $TARGET | cut -d '/' -f 1)
    GOARCH=$(echo $TARGET | cut -d '/' -f 2)

    OUTPUT_NAME="build/relay_${GOOS}_${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi

    echo "⏳ Building Relay for $GOOS ($GOARCH)..."
    
    env GOOS=$GOOS GOARCH=$GOARCH go build -o $OUTPUT_NAME .

    if [ $? -ne 0 ]; then
        echo "❌ Failed to build Relay for $GOOS/$GOARCH"
    else
        echo "✅ Built: $OUTPUT_NAME"
    fi
done

echo "🎉 All Relay builds complete! Check the /build folder."