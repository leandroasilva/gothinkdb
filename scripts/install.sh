#!/bin/bash
# GoThinkDB Universal Installer (macOS & Linux)
# Usage: curl -fsSL https://raw.githubusercontent.com/leandroasilva/gothinkdb/main/scripts/install.sh | bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}==========================================${NC}"
    echo -e "${BLUE}GoThinkDB Installer${NC}"
    echo -e "${BLUE}==========================================${NC}"
    echo ""
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_info() {
    echo -e "${YELLOW}→${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS_NAME="linux";;
    Darwin*)    OS_NAME="darwin";;
    *)          print_error "Unsupported OS: ${OS}"; exit 1;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)     ARCH_NAME="amd64";;
    aarch64)    ARCH_NAME="arm64";;
    arm64)      ARCH_NAME="arm64";;
    *)          print_error "Unsupported architecture: ${ARCH}"; exit 1;;
esac

print_header
print_info "Detected: ${OS_NAME} ${ARCH_NAME}"
echo ""

# Get latest release version
print_info "Fetching latest release..."
LATEST_VERSION=$(curl -s https://api.github.com/repos/leandroasilva/gothinkdb/releases/latest | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    print_error "Could not fetch latest version"
    exit 1
fi

print_success "Latest version: v${LATEST_VERSION}"
echo ""

# Determine install directory
if [ "$OS_NAME" = "darwin" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="/usr/local/bin"
fi

# Check if we need sudo
NEED_SUDO=false
if [ ! -w "$INSTALL_DIR" ]; then
    NEED_SUDO=true
fi

# Download binary
BINARY_NAME="gothinkdb-${OS_NAME}-${ARCH_NAME}"
DOWNLOAD_URL="https://github.com/leandroasilva/gothinkdb/releases/download/v${LATEST_VERSION}/${BINARY_NAME}"

print_info "Downloading ${BINARY_NAME}..."
TEMP_FILE="/tmp/${BINARY_NAME}"

if ! curl -L -o "$TEMP_FILE" "$DOWNLOAD_URL" 2>/dev/null; then
    print_error "Failed to download binary"
    exit 1
fi

# Make executable
chmod +x "$TEMP_FILE"

# Install
if [ "$NEED_SUDO" = true ]; then
    print_info "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TEMP_FILE" "${INSTALL_DIR}/gothinkdb"
else
    print_info "Installing to ${INSTALL_DIR}..."
    mv "$TEMP_FILE" "${INSTALL_DIR}/gothinkdb"
fi

print_success "Installed to ${INSTALL_DIR}/gothinkdb"
echo ""

# Create data directory
DATA_DIR="$HOME/.gothinkdb"
mkdir -p "$DATA_DIR"
print_success "Created data directory: ${DATA_DIR}"
echo ""

# Verify installation
if command -v gothinkdb &> /dev/null; then
    print_success "GoThinkDB is ready!"
    echo ""
    echo -e "${BLUE}==========================================${NC}"
    echo -e "${BLUE}Installation Complete!${NC}"
    echo -e "${BLUE}==========================================${NC}"
    echo ""
    echo "Start GoThinkDB:"
    echo "  gothinkdb -data ${DATA_DIR}"
    echo ""
    echo "Or run in background:"
    echo "  gothinkdb -data ${DATA_DIR} &"
    echo ""
    echo "Access dashboard:"
    echo "  http://localhost:8080"
    echo ""
    echo "Default credentials:"
    echo "  Username: admin"
    echo "  Password: admin"
    echo ""
    echo "Stop with: Ctrl+C or kill the process"
    echo ""
else
    print_error "Installation failed - gothinkdb not found in PATH"
    echo "Please add ${INSTALL_DIR} to your PATH"
    exit 1
fi
