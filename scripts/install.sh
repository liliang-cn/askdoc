#!/bin/bash
set -e

BINARY="askdoc"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/askdoc"
DATA_DIR="/var/lib/askdoc"
SERVICE_USER="askdoc"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

if [ "$EUID" -ne 0 ]; then
    log_error "Please run as root (sudo)"
    exit 1
fi

if [ ! -f "$BINARY" ]; then
    log_error "Binary '$BINARY' not found"
    log_info "Download from: https://github.com/liliang-cn/askdoc/releases"
    exit 1
fi

log_info "Installing AskDoc..."

id -u $SERVICE_USER >/dev/null 2>&1 || useradd -r -s /bin/false $SERVICE_USER

log_info "Creating directories..."
mkdir -p $CONFIG_DIR $DATA_DIR/data $DATA_DIR/documents

log_info "Installing binary..."
cp $BINARY $INSTALL_DIR/$BINARY
chmod +x $INSTALL_DIR/$BINARY

if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    log_info "Installing config..."
    cat > $CONFIG_DIR/config.yaml << 'EOF'
server:
  port: 43510
  host: 0.0.0.0
  base_url: "http://localhost:43510"

admin:
  api_key: "change-me-in-production"

database:
  path: "/var/lib/askdoc/data/askdoc.db"

storage:
  documents: "/var/lib/askdoc/documents"

llm:
  base_url: "http://localhost:11434/v1"
  api_key: "ollama"
  llm_model: "qwen3:8b"
  embedding_model: "qwen3-embedding:8b"

rag:
  db_path: "/var/lib/askdoc/data/rag.db"
  index_type: "hnsw"
  chunk_size: 512
  chunk_overlap: 50

rate_limit:
  enabled: true
  requests_per_hour: 100
EOF
    log_warn "Edit $CONFIG_DIR/config.yaml!"
else
    log_info "Config exists, skipping..."
fi

log_info "Installing systemd service..."
cat > /etc/systemd/system/askdoc.service << EOF
[Unit]
Description=AskDoc - AI Document Q&A Service
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$DATA_DIR
ExecStart=$INSTALL_DIR/$BINARY --config $CONFIG_DIR/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR
ReadOnlyPaths=$CONFIG_DIR

[Install]
WantedBy=multi-user.target
EOF

chown -R $SERVICE_USER:$SERVICE_USER $DATA_dir $CONFIG_DIR
chmod 600 $CONFIG_DIR/config.yaml
systemctl daemon-reload

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "1. Edit config: sudo nano $CONFIG_DIR/config.yaml"
echo "2. Start service: sudo systemctl enable --now askdoc"
echo "3. Check status: sudo systemctl status askdoc"
echo "4. View logs: sudo journalctl -u askdoc -f"
echo ""
echo "Web UI: http://localhost:43510/admin"
