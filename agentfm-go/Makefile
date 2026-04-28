# ==============================================================================
# AgentFM Build & Deployment Makefile
# ==============================================================================

APP_NAME := agentfm
RELAY_NAME := relay
RELAY_INSTALL_NAME := agentfm-relay
VERSION := 1.2.0
RELEASE_TAG := AgentFM
MAIN_FILE := ./cmd/agentfm
RELAY_MAIN_FILE := ./cmd/relay
BUILD_DIR := build
INSTALL_DIR := /usr/local/bin
GITHUB_REPO := Agent-FM/agentfm-core

HOST_OS := $(shell go env GOOS 2>/dev/null || uname -s | tr A-Z a-z)
HOST_ARCH := $(shell go env GOARCH 2>/dev/null || uname -m)

# Normalize architecture names
ifeq ($(HOST_ARCH),x86_64)
    HOST_ARCH := amd64
endif
ifeq ($(HOST_ARCH),aarch64)
    HOST_ARCH := arm64
endif

# Map OS names for UI
ifeq ($(HOST_OS),darwin)
    OS_NAME := macOS
else ifeq ($(HOST_OS),linux)
    OS_NAME := Linux
else
    OS_NAME := $(HOST_OS)
endif

# Determine exact binary filenames for downloads
ifeq ($(HOST_OS),windows)
    BINARY_NAME := $(APP_NAME)_$(HOST_OS)_$(HOST_ARCH).exe
    RELAY_BINARY_NAME := $(RELAY_NAME)_$(HOST_OS)_$(HOST_ARCH).exe
else
    BINARY_NAME := $(APP_NAME)_$(HOST_OS)_$(HOST_ARCH)
    RELAY_BINARY_NAME := $(RELAY_NAME)_$(HOST_OS)_$(HOST_ARCH)
endif

DOWNLOAD_URL := https://github.com/$(GITHUB_REPO)/releases/download/$(RELEASE_TAG)/$(BINARY_NAME)
RELAY_DOWNLOAD_URL := https://github.com/$(GITHUB_REPO)/releases/download/$(RELEASE_TAG)/$(RELAY_BINARY_NAME)

.PHONY: all check-deps build build-agentfm build-relay build-all install uninstall clean help setup-deps download-install \
        test test-unit test-integration test-race test-coverage

# Default target when someone just types `make`
all: check-deps build

# ==========================================
# 🛠️ PREREQUISITE CHECKS
# ==========================================
check-deps:
	@echo "🔍 Checking system prerequisites for $(OS_NAME) ($(HOST_ARCH))..."
	@command -v go >/dev/null 2>&1 || { \
		echo "❌ Go is not installed. AgentFM requires Go 1.21+."; \
		echo "👉 Run 'make setup-deps' to see installation instructions."; \
		exit 1; \
	}
	@command -v podman >/dev/null 2>&1 || { \
		echo "⚠️  WARNING: Podman is not installed!"; \
		echo "   You can still run a Boss node, but Worker nodes require Podman to sandbox AI agents."; \
		echo "   👉 Run 'make setup-deps' for instructions."; \
	}
	@echo "✅ Prerequisites satisfied!"

# Setup helper for missing dependencies
setup-deps:
ifeq ($(HOST_OS),darwin)
	@echo "🍎 macOS Detected. Use Homebrew to install prerequisites:"
	@echo "   brew install go podman"
	@echo "   podman machine init && podman machine start"
else ifeq ($(HOST_OS),linux)
	@echo "🐧 Linux Detected. Install using your package manager:"
	@echo "   Debian/Ubuntu: sudo apt update && sudo apt install golang podman"
	@echo "   Fedora/RHEL:   sudo dnf install golang podman"
else
	@echo "💻 Please install Go (https://go.dev) and Podman (https://podman.io) manually."
endif

# ==========================================
# 🔨 NATIVE BUILD (For current machine)
# ==========================================
build-agentfm:
	@echo "🔨 Building native $(APP_NAME) binary for $(OS_NAME) ($(HOST_ARCH))..."
	@go mod tidy
	@go build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(APP_NAME) $(MAIN_FILE)
	@echo "✅ $(APP_NAME) build complete! Run ./$(APP_NAME) --help"

build-relay:
	@echo "🔨 Building native $(RELAY_NAME) binary for $(OS_NAME) ($(HOST_ARCH))..."
	@go mod tidy
	@go build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(RELAY_NAME) $(RELAY_MAIN_FILE)
	@echo "✅ $(RELAY_NAME) build complete! Run ./$(RELAY_NAME) --help"

build: build-agentfm build-relay
	@echo "✅ Both binaries built successfully!"

# ==========================================
# 🌍 CROSS-COMPILATION (For Releases)
# ==========================================
build-all: clean
	@echo "🌍 Cross-compiling for all major platforms..."
	@mkdir -p $(BUILD_DIR)
	
	@echo "  -> Compiling Linux (amd64)..."
	@GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP_NAME)_linux_amd64 $(MAIN_FILE)
	@GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(RELAY_NAME)_linux_amd64 $(RELAY_MAIN_FILE)
	
	@echo "  -> Compiling Linux (arm64)..."
	@GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP_NAME)_linux_arm64 $(MAIN_FILE)
	@GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(RELAY_NAME)_linux_arm64 $(RELAY_MAIN_FILE)
	
	@echo "  -> Compiling macOS (amd64/Intel)..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP_NAME)_darwin_amd64 $(MAIN_FILE)
	@GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(RELAY_NAME)_darwin_amd64 $(RELAY_MAIN_FILE)
	
	@echo "  -> Compiling macOS (arm64/Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP_NAME)_darwin_arm64 $(MAIN_FILE)
	@GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(RELAY_NAME)_darwin_arm64 $(RELAY_MAIN_FILE)
	
	@echo "  -> Compiling Windows (amd64)..."
	@GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP_NAME)_windows_amd64.exe $(MAIN_FILE)
	@GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(RELAY_NAME)_windows_amd64.exe $(RELAY_MAIN_FILE)
	
	@echo "✅ All binaries generated in ./$(BUILD_DIR)/"

# ==========================================
# 🚀 INSTALLATION (Global system path)
# ==========================================
install: build
	@echo "🚀 Installing locally compiled binaries to $(INSTALL_DIR)..."
	@echo "🔑 You may be prompted for your password to write to $(INSTALL_DIR)."
	@sudo mv $(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@sudo chmod +x $(INSTALL_DIR)/$(APP_NAME)
	@sudo mv $(RELAY_NAME) $(INSTALL_DIR)/$(RELAY_INSTALL_NAME)
	@sudo chmod +x $(INSTALL_DIR)/$(RELAY_INSTALL_NAME)
	@echo "✅ Installation successful! You can now run '$(APP_NAME)' and '$(RELAY_INSTALL_NAME)' from anywhere."

uninstall:
	@echo "🗑️  Removing binaries from $(INSTALL_DIR)..."
	@sudo rm -f $(INSTALL_DIR)/$(APP_NAME)
	@sudo rm -f $(INSTALL_DIR)/$(RELAY_INSTALL_NAME)
	@echo "✅ Uninstalled."

# ==========================================
# 🌐 DOWNLOAD PRE-BUILT RELEASE
# ==========================================
download-install:
	@echo "⬇️  Downloading $(APP_NAME) v$(VERSION) for $(HOST_OS) ($(HOST_ARCH))..."
	@echo "🔗 Fetching from: $(DOWNLOAD_URL)"
	@curl -fSL $(DOWNLOAD_URL) -o $(APP_NAME)_tmp || { \
		echo "❌ Failed to download binary. Make sure release $(RELEASE_TAG) exists at $(DOWNLOAD_URL)."; \
		exit 1; \
	}
	@echo "⬇️  Downloading $(RELAY_NAME) v$(VERSION) for $(HOST_OS) ($(HOST_ARCH))..."
	@echo "🔗 Fetching from: $(RELAY_DOWNLOAD_URL)"
	@curl -fSL $(RELAY_DOWNLOAD_URL) -o $(RELAY_NAME)_tmp || { \
		echo "❌ Failed to download relay. Make sure release $(RELEASE_TAG) exists at $(RELAY_DOWNLOAD_URL)."; \
		exit 1; \
	}
	@echo "🚀 Installing pre-built binaries to $(INSTALL_DIR)..."
	@chmod +x $(APP_NAME)_tmp $(RELAY_NAME)_tmp
	@sudo mv $(APP_NAME)_tmp $(INSTALL_DIR)/$(APP_NAME)
	@sudo mv $(RELAY_NAME)_tmp $(INSTALL_DIR)/$(RELAY_INSTALL_NAME)
	@echo "✅ Successfully installed both! Run '$(APP_NAME) --help' and '$(RELAY_INSTALL_NAME) --help' to verify."

# ==========================================
# 🧹 CLEANUP
# ==========================================
clean:
	@echo "🧹 Cleaning up old binaries..."
	@rm -f $(APP_NAME) $(RELAY_NAME)
	@rm -rf $(BUILD_DIR)
	@echo "✅ Cleaned."

# ==========================================
# 🧪 TESTS
# ==========================================
# The suite is split into two tiers:
#   - unit:        fast, package-scoped tests under ./internal/...
#   - integration: slower, cross-package scenarios under ./test/integration/...
# Both run under -race by default; see test/README.md for the layout contract.

test: test-unit

test-unit:
	@echo "🧪 Running unit tests (./internal/...) under -race..."
	@go test -race -count=1 -timeout 120s ./internal/...

test-integration:
	@echo "🧪 Running integration tests (./test/integration/...) under -race..."
	@go test -race -count=1 -timeout 180s ./test/integration/...

test-race:
	@echo "🧪 Running full suite under -race..."
	@go test -race -count=1 -timeout 300s ./...

test-coverage:
	@echo "📊 Computing unit-test coverage..."
	@go test -race -count=1 -timeout 180s -cover -coverprofile=coverage.out ./internal/...
	@go tool cover -func=coverage.out | tail -1
	@echo "    -> open a full function report with: go tool cover -func=coverage.out"
	@echo "    -> open an HTML report with:         go tool cover -html=coverage.out"

# ==========================================
# 📖 HELP
# ==========================================
help:
	@echo "🚀 AgentFM Makefile Commands:"
	@echo "  make              - Checks deps and builds both native binaries"
	@echo "  make build        - Compiles BOTH agentfm and relay for your current OS"
	@echo "  make build-agentfm- Compiles ONLY the agentfm node binary"
	@echo "  make build-relay  - Compiles ONLY the relay server binary"
	@echo "  make install      - Compiles and installs both tools globally to $(INSTALL_DIR)"
	@echo "  make download-install - ⬇️ Downloads pre-built releases and installs them"
	@echo "  make uninstall    - Removes the tools from your system"
	@echo "  make build-all    - Cross-compiles for Mac, Linux, and Windows (x86 & ARM)"
	@echo "  make check-deps   - Validates Go and Podman are installed"
	@echo "  make setup-deps   - Prints OS-specific instructions to install missing tools"
	@echo "  make clean        - Deletes compiled binaries"
	@echo ""
	@echo "🧪 Test Targets:"
	@echo "  make test              - Runs unit tests (./internal/...) under -race"
	@echo "  make test-unit         - Same as 'make test'"
	@echo "  make test-integration  - Runs integration tests (./test/integration/...)"
	@echo "  make test-race         - Runs the full suite under -race"
	@echo "  make test-coverage     - Produces coverage.out + prints the summary"