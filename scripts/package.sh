#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_ROOT="$(cd "$HERE/.." && pwd)"

cd "$DESKTOP_ROOT"

# Step 1: ensure cross-platform binaries exist
if [ ! -f "resources/agentfm-$(node -p 'process.platform')-$(node -p 'process.arch')$(node -p 'process.platform === \"win32\" ? \".exe\" : \"\"')" ]; then
  echo "Building cross-platform binaries..."
  ./scripts/build-binaries.sh
fi

# Step 2: build renderer + main
npm run build

# Step 3: electron-builder
npx electron-builder
