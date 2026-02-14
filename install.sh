#!/bin/bash
set -e

REPO="net2share/dnstm"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="dnstm"
FORCE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --force|-f)
            FORCE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--force]"
            echo ""
            echo "Install or update dnstm."
            echo ""
            echo "Options:"
            echo "  --force, -f    Skip confirmation prompts"
            echo "  --help, -h     Show this help message"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

success() {
    echo -e "${GREEN}$1${NC}"
}

warn() {
    echo -e "${YELLOW}$1${NC}"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    error "Please run as root (sudo)"
fi

# Check if already installed
if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    echo "dnstm is already installed."
    if [ "$FORCE" = true ]; then
        echo "Running update with --force..."
        exec "${INSTALL_DIR}/${BINARY_NAME}" update --force
    else
        read -p "Would you like to check for updates? [y/N] " -n 1 -r < /dev/tty
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            exec "${INSTALL_DIR}/${BINARY_NAME}" update
        fi
    fi
    exit 0
fi

# Not installed - proceed with fresh install

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
    error "Unsupported OS: $OS. Only Linux is supported."
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l|armv7)
        ARCH="armv7"
        ;;
    i386|i686)
        ARCH="386"
        ;;
    *)
        error "Unsupported architecture: $ARCH"
        ;;
esac

echo "Detected: ${OS}/${ARCH}"

# Confirmation (unless --force)
if [ "$FORCE" = false ]; then
    read -p "Install dnstm? [y/N] " -n 1 -r < /dev/tty
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled."
        exit 0
    fi
fi

# Get latest release
echo "Fetching latest release..."
LATEST_RELEASE=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
    error "Failed to fetch latest release"
fi

echo "Latest version: ${LATEST_RELEASE}"

# Download binary
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}/${BINARY_NAME}-${OS}-${ARCH}"
echo "Downloading from: ${DOWNLOAD_URL}"

TMP_FILE=$(mktemp)
if ! curl -sL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
    rm -f "$TMP_FILE"
    error "Failed to download binary"
fi

# Verify download
if [ ! -s "$TMP_FILE" ]; then
    rm -f "$TMP_FILE"
    error "Downloaded file is empty"
fi

# Install
mkdir -p "$INSTALL_DIR"
mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

success "Successfully installed ${BINARY_NAME} ${LATEST_RELEASE} to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""

# Install transport binaries
echo "Installing transport binaries..."
"${INSTALL_DIR}/${BINARY_NAME}" install

echo ""
echo "Getting started:"
echo "  ${BINARY_NAME}              # Interactive menu"
echo "  ${BINARY_NAME} tunnel add   # Add a tunnel"
echo ""
echo "Documentation: https://github.com/${REPO}#readme"
