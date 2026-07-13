#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "usage: $0 <darwin-binary>..." >&2
  echo "stamps the AgentFM Finder icon onto each binary and packs it into a" >&2
  echo "metadata-preserving .zip suitable for GitHub release assets" >&2
  exit 1
fi

script_dir="$(cd "$(dirname "$0")" && pwd)"
icns="$script_dir/../agentfm-desktop/build/icon.icns"
[ -f "$icns" ] || { echo "icon not found: $icns" >&2; exit 1; }

for bin in "$@"; do
  [ -f "$bin" ] || { echo "no such file: $bin" >&2; exit 1; }
  chmod +x "$bin"
  swift "$script_dir/seticon.swift" "$icns" "$bin"
  ditto -c -k --sequesterRsrc "$bin" "${bin}.zip"
  echo "packed ${bin}.zip"
done
