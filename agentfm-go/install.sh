#!/bin/sh
set -e

# ==============================================================================
# AgentFM & Relay Auto-Installer
# ==============================================================================

REPO="Agent-FM/agentfm-core"
APP_NAME="agentfm"
RELAY_NAME="relay"
RELAY_INSTALL_NAME="agentfm-relay" 
INSTALL_DIR="/usr/local/bin"

# 🎨 Color formatting for a premium feel
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' 

echo "${CYAN}🚀 Starting AgentFM & Relay Installation...${NC}"

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

echo "📦 Installing AgentFM Suite ${LATEST_VERSION}..."

APP_FILENAME="${APP_NAME}_${OS}_${ARCH}"
RELAY_FILENAME="${RELAY_NAME}_${OS}_${ARCH}"

APP_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${APP_FILENAME}"
RELAY_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${RELAY_FILENAME}"

TMP_DIR=$(mktemp -d)
TMP_APP="${TMP_DIR}/${APP_NAME}"
TMP_RELAY="${TMP_DIR}/${RELAY_NAME}"

echo "⬇️  Downloading AgentFM from: $APP_URL"
if ! curl -fSL "$APP_URL" -o "$TMP_APP"; then
    echo "${RED}❌ Failed to download AgentFM!${NC}"
    rm -rf "$TMP_DIR"
    exit 1
fi

echo "⬇️  Downloading Relay from: $RELAY_URL"
if ! curl -fSL "$RELAY_URL" -o "$TMP_RELAY"; then
    echo "${RED}❌ Failed to download Relay!${NC}"
    rm -rf "$TMP_DIR"
    exit 1
fi

chmod +x "$TMP_APP" "$TMP_RELAY"

echo "🔑 Moving binaries to ${INSTALL_DIR} (may require sudo password)..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_APP" "${INSTALL_DIR}/${APP_NAME}"
    mv "$TMP_RELAY" "${INSTALL_DIR}/${RELAY_INSTALL_NAME}"
else
    sudo mv "$TMP_APP" "${INSTALL_DIR}/${APP_NAME}"
    sudo mv "$TMP_RELAY" "${INSTALL_DIR}/${RELAY_INSTALL_NAME}"
fi

rm -rf "$TMP_DIR"

echo ""
if command -v podman >/dev/null 2>&1; then
    echo "✅ Podman detected. You are ready to host AI Worker nodes!"
else
    echo "⚠️  ${YELLOW}Note: Podman is not installed.${NC}"
    echo "   You can run Boss nodes and Relays immediately, but to host Worker nodes, please install Podman: https://podman.io"
fi

echo "Ok"
