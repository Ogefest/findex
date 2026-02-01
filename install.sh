#!/bin/bash
set -e

# Findex installer - downloads from GitHub releases and installs
# Usage: curl -sSL https://raw.githubusercontent.com/ogefest/findex/main/install.sh | sudo bash

REPO="ogefest/findex"
INSTALL_DIR="/opt/findex"
CONFIG_DIR="/etc/findex"
DATA_DIR="/var/lib/findex"
USER="findex"
GROUP="findex"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    error "Please run as root or with sudo"
fi

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            error "Unsupported OS: $OS"
            ;;
    esac

    echo "${OS}-${ARCH}"
}

# Get latest release version from GitHub
get_latest_version() {
    curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and extract release
download_release() {
    local version=$1
    local platform=$2
    local url="https://github.com/${REPO}/releases/download/${version}/findex-${version}-${platform}.tar.gz"
    local tmp_dir=$(mktemp -d)

    info "Downloading findex ${version} for ${platform}..."
    curl -sSL "$url" -o "${tmp_dir}/findex.tar.gz" || error "Failed to download release"

    info "Extracting..."
    tar -xzf "${tmp_dir}/findex.tar.gz" -C "${tmp_dir}"

    # Find extracted directory
    EXTRACTED_DIR=$(find "${tmp_dir}" -maxdepth 1 -type d -name "findex-*" | head -1)
    if [ -z "$EXTRACTED_DIR" ]; then
        error "Failed to find extracted directory"
    fi

    echo "$EXTRACTED_DIR"
}

# Main installation
main() {
    info "=== Findex Installer ==="

    # Parse arguments
    VERSION="${1:-}"
    if [ -z "$VERSION" ]; then
        info "Fetching latest version..."
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            error "Could not determine latest version"
        fi
    fi

    PLATFORM=$(detect_platform)
    info "Installing findex ${VERSION} for ${PLATFORM}"

    # Download release
    EXTRACTED_DIR=$(download_release "$VERSION" "$PLATFORM")

    # Create user if doesn't exist
    if ! id "$USER" &>/dev/null; then
        info "Creating user ${USER}..."
        useradd -r -s /sbin/nologin -d "$DATA_DIR" "$USER"
    fi

    # Create directories
    info "Creating directories..."
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    chown "$USER:$GROUP" "$DATA_DIR"

    # Install binaries
    info "Installing binaries to ${INSTALL_DIR}..."
    cp "${EXTRACTED_DIR}/findex" "$INSTALL_DIR/"
    cp "${EXTRACTED_DIR}/findex-webserver" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/findex" "$INSTALL_DIR/findex-webserver"

    # Install config if doesn't exist
    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        if [ -f "${EXTRACTED_DIR}/config.example.yaml" ]; then
            info "Installing default config..."
            cp "${EXTRACTED_DIR}/config.example.yaml" "$CONFIG_DIR/config.yaml"
            chown "$USER:$GROUP" "$CONFIG_DIR/config.yaml"

            # Update db_path in config
            sed -i "s|db_path:.*|db_path: ${DATA_DIR}/index.db|g" "$CONFIG_DIR/config.yaml" 2>/dev/null || true
        fi
    else
        warn "Config already exists at ${CONFIG_DIR}/config.yaml, skipping"
    fi

    # Install systemd units (Linux only)
    if [ -d "${EXTRACTED_DIR}/systemd" ]; then
        info "Installing systemd units..."
        cp "${EXTRACTED_DIR}/systemd/findex-web.service" /etc/systemd/system/
        cp "${EXTRACTED_DIR}/systemd/findex-scanner.service" /etc/systemd/system/
        cp "${EXTRACTED_DIR}/systemd/findex-scanner.timer" /etc/systemd/system/
        systemctl daemon-reload
    fi

    # Cleanup
    rm -rf "$(dirname "$EXTRACTED_DIR")"

    info ""
    info "=== Installation complete ==="
    info ""
    info "Installed version: ${VERSION}"
    info "Binaries: ${INSTALL_DIR}/"
    info "Config: ${CONFIG_DIR}/config.yaml"
    info "Data: ${DATA_DIR}/"
    info ""
    info "Next steps:"
    info "1. Edit config: sudo nano ${CONFIG_DIR}/config.yaml"
    info "2. Enable web server: sudo systemctl enable --now findex-web.service"
    info "3. Enable scanner timer: sudo systemctl enable --now findex-scanner.timer"
    info "4. Run initial scan: sudo systemctl start findex-scanner.service"
    info ""
    info "Check status:"
    info "  sudo systemctl status findex-web.service"
    info "  sudo journalctl -u findex-web.service -f"
}

main "$@"
