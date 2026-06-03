#!/usr/bin/env bash
set -euo pipefail

# Convenience: build the local agentfm binary (if missing) and run electron-vite dev.
HERE="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_ROOT="$(cd "$HERE/.." && pwd)"
CORE_BIN="$DESKTOP_ROOT/../agentfm-core/agentfm-go/agentfm"

if [ ! -f "$CORE_BIN" ]; then
  echo "Building dev agentfm binary..."
  (cd "$(dirname "$CORE_BIN")" && go build -o agentfm ./cmd/agentfm)
fi

cd "$DESKTOP_ROOT"
exec npm run dev
