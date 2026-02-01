#!/bin/bash
set -e

# Findex systemd installation script
# Run as root or with sudo

INSTALL_DIR="/opt/findex"
CONFIG_DIR="/etc/findex"
DATA_DIR="/var/lib/findex"
USER="findex"
GROUP="findex"

echo "=== Findex Systemd Installation ==="

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root or with sudo"
    exit 1
fi

# Create user if doesn't exist
if ! id "$USER" &>/dev/null; then
    echo "Creating user $USER..."
    useradd -r -s /sbin/nologin -d "$DATA_DIR" "$USER"
fi

# Create directories
echo "Creating directories..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"
chown "$USER:$GROUP" "$DATA_DIR"

# Check if binaries exist in current directory
if [ ! -f "findex" ] || [ ! -f "findex-webserver" ]; then
    echo "Building binaries..."
    if command -v go &>/dev/null; then
        go build -o findex ./cmd/findex
        go build -o findex-webserver ./cmd/webserver
    else
        echo "Error: Go is not installed and binaries not found"
        exit 1
    fi
fi

# Install binaries
echo "Installing binaries to $INSTALL_DIR..."
cp findex findex-webserver "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/findex" "$INSTALL_DIR/findex-webserver"

# Install config if doesn't exist
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    if [ -f "index_config.yaml" ]; then
        echo "Installing default config..."
        cp index_config.yaml "$CONFIG_DIR/config.yaml"
        chown "$USER:$GROUP" "$CONFIG_DIR/config.yaml"
        echo "IMPORTANT: Edit $CONFIG_DIR/config.yaml and set db_path to $DATA_DIR/"
    else
        echo "Warning: No config file found. Create $CONFIG_DIR/config.yaml manually."
    fi
fi

# Install systemd units
echo "Installing systemd units..."
cp systemd/findex-web.service /etc/systemd/system/
cp systemd/findex-scanner.service /etc/systemd/system/
cp systemd/findex-scanner.timer /etc/systemd/system/

# Reload systemd
systemctl daemon-reload

echo ""
echo "=== Installation complete ==="
echo ""
echo "Next steps:"
echo "1. Edit config: sudo nano $CONFIG_DIR/config.yaml"
echo "2. Enable web server: sudo systemctl enable --now findex-web.service"
echo "3. Enable scanner timer: sudo systemctl enable --now findex-scanner.timer"
echo "4. Run initial scan: sudo systemctl start findex-scanner.service"
echo ""
echo "Check status:"
echo "  sudo systemctl status findex-web.service"
echo "  sudo journalctl -u findex-web.service -f"
