#!/bin/sh
set -e

# ==============================================================================
# AgentFM Auto-Installer
# ==============================================================================

REPO="Agent-FM/agentfm-core"
APP_NAME="agentfm"
INSTALL_DIR="/usr/local/bin"

# 🎨 Color formatting for a premium feel
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' 

echo "${CYAN}🚀 Starting AgentFM Installation...${NC}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    *) echo "${RED}❌ Unsupported OS: $OS${NC}"; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "${RED}❌ Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

echo "🔍 Detected System: ${YELLOW}${OS}_${ARCH}${NC}"

echo "🌐 Fetching latest release version from GitHub..."

API_RESPONSE=$(curl -s "https://api.github.com/repos/$REPO/releases/latest")

LATEST_VERSION=$(echo "$API_RESPONSE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "${RED}❌ Failed to fetch the latest version.${NC}"
    echo "${YELLOW}GitHub API returned this response:${NC}"
    echo "$API_RESPONSE"
    exit 1
fi

echo "📦 Installing AgentFM ${LATEST_VERSION}..."

APP_FILENAME="${APP_NAME}_${OS}_${ARCH}"

APP_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${APP_FILENAME}"

TMP_DIR=$(mktemp -d)
TMP_APP="${TMP_DIR}/${APP_NAME}"

echo "⬇️  Downloading AgentFM from: $APP_URL"
if ! curl -fSL "$APP_URL" -o "$TMP_APP"; then
    echo "${RED}❌ Failed to download AgentFM!${NC}"
    rm -rf "$TMP_DIR"
    exit 1
fi

# Verify the download against the release's SHA256SUMS before it goes
# anywhere near sudo. Releases published before checksums existed have no
# SHA256SUMS: we warn and continue, unless AGENTFM_REQUIRE_CHECKSUM=1.
SUMS_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/SHA256SUMS"
if curl -fsSL "$SUMS_URL" -o "${TMP_DIR}/SHA256SUMS" 2>/dev/null; then
    EXPECTED=$(awk -v f="$APP_FILENAME" '$2 == f || $2 == "*"f { print $1 }' "${TMP_DIR}/SHA256SUMS")
    if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL=$(sha256sum "$TMP_APP" | awk '{ print $1 }')
    else
        ACTUAL=$(shasum -a 256 "$TMP_APP" | awk '{ print $1 }')
    fi
    if [ -z "$EXPECTED" ] || [ "$EXPECTED" != "$ACTUAL" ]; then
        echo "${RED}❌ Checksum verification failed for ${APP_FILENAME}. Not installing.${NC}"
        echo "   expected: ${EXPECTED:-<not listed in SHA256SUMS>}"
        echo "   actual:   ${ACTUAL}"
        rm -rf "$TMP_DIR"
        exit 1
    fi
    echo "🔐 Checksum verified (sha256 ${ACTUAL})"
elif [ "${AGENTFM_REQUIRE_CHECKSUM:-0}" = "1" ]; then
    echo "${RED}❌ Release ${LATEST_VERSION} has no SHA256SUMS and AGENTFM_REQUIRE_CHECKSUM=1. Not installing.${NC}"
    rm -rf "$TMP_DIR"
    exit 1
else
    echo "⚠️  ${YELLOW}Release ${LATEST_VERSION} publishes no SHA256SUMS; the download could not be verified.${NC}"
fi

chmod +x "$TMP_APP"

echo "🔑 Moving binaries to ${INSTALL_DIR} (may require sudo password)..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_APP" "${INSTALL_DIR}/${APP_NAME}"
else
    sudo mv "$TMP_APP" "${INSTALL_DIR}/${APP_NAME}"
fi

rm -rf "$TMP_DIR"

echo ""
if command -v podman >/dev/null 2>&1; then
    echo "✅ Podman detected. You are ready to host AI Worker nodes!"
else
    echo "⚠️  ${YELLOW}Note: Podman is not installed.${NC}"
    echo "   You can run Boss and relay nodes immediately (relay: 'agentfm -mode relay'), but to host Worker nodes, please install Podman: https://podman.io"
fi

echo "Ok"
