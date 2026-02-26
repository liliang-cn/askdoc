#!/bin/bash
set -e

# AskDoc Uninstallation Script
# Usage: sudo ./uninstall.sh

BINARY="askdoc"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/askdoc"
DATA_DIR="/var/lib/askdoc"
SERVICE_USER="askdoc"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo)"
    exit 1
fi

log_info "Uninstalling AskDoc..."

# Stop service
if systemctl is-active --quiet askdoc; then
    log_info "Stopping service..."
    systemctl stop askdoc
fi

# Disable service
if systemctl is-enabled --quiet askdoc 2>/dev/null; then
    log_info "Disabling service..."
    systemctl disable askdoc
fi

# Remove service file
if [ -f "/etc/systemd/system/askdoc.service" ]; then
    log_info "Removing systemd service..."
    rm -f /etc/systemd/system/askdoc.service
    systemctl daemon-reload
fi

# Remove binary
if [ -f "$INSTALL_DIR/$BINARY" ]; then
    log_info "Removing binary..."
    rm -f $INSTALL_DIR/$BINARY
fi

# Ask about data removal
echo ""
read -p "Remove data directory ($DATA_DIR)? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Removing data directory..."
    rm -rf $DATA_DIR
fi

# Ask about config removal
read -p "Remove config directory ($CONFIG_DIR)? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Removing config directory..."
    rm -rf $CONFIG_DIR
fi

# Ask about user removal
read -p "Remove user '$SERVICE_USER'? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Removing user..."
    userdel $SERVICE_USER 2>/dev/null || true
fi

log_info ""
log_info "Uninstallation complete!"
