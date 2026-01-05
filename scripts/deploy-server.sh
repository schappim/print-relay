#!/bin/bash
#
# PrintRelay Server Deploy Script
#
# Deploys the latest PrintRelay server to printers.littlebird.app (149.28.48.112)
#
# Usage:
#   ./scripts/deploy-server.sh              # Deploy latest from GitHub
#   ./scripts/deploy-server.sh --local      # Deploy from local dist/ folder
#   ./scripts/deploy-server.sh --version 1.2.0  # Deploy specific version
#
set -e

# Configuration
SERVER_IP="149.28.48.112"
SERVER_USER="root"
SSH_KEY="$HOME/.ssh/id_rsa"
REPO="schappim/print-relay"
SERVICE_NAME="printrelay-server"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info() {
    echo -e "${BLUE}==>${NC} ${BOLD}$1${NC}"
}

success() {
    echo -e "${GREEN}==>${NC} ${BOLD}$1${NC}"
}

warn() {
    echo -e "${YELLOW}==>${NC} ${BOLD}$1${NC}"
}

error() {
    echo -e "${RED}==>${NC} ${BOLD}$1${NC}"
    exit 1
}

# Parse arguments
USE_LOCAL=false
VERSION=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --local)
            USE_LOCAL=true
            shift
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --help|-h)
            echo "PrintRelay Server Deploy Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --local          Deploy from local dist/ folder"
            echo "  --version VER    Deploy specific version (default: latest)"
            echo "  --help, -h       Show this help message"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║${NC}          ${BOLD}PrintRelay Server Deploy${NC}                         ${BLUE}║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check SSH connection
info "Testing SSH connection to ${SERVER_IP}..."
if ! ssh -i "$SSH_KEY" -o BatchMode=yes -o ConnectTimeout=5 "${SERVER_USER}@${SERVER_IP}" "echo connected" &>/dev/null; then
    error "Cannot connect to server. Check SSH key and server availability."
fi
success "SSH connection OK"

# Determine source of binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

if [ "$USE_LOCAL" = true ]; then
    info "Using local binary from dist/"
    BINARY_PATH="${PROJECT_ROOT}/dist/printrelay-server-linux-amd64"
    if [ ! -f "$BINARY_PATH" ]; then
        error "Local binary not found: $BINARY_PATH"
    fi
else
    # Get version if not specified
    if [ -z "$VERSION" ]; then
        info "Fetching latest version..."
        VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            error "Could not determine latest version"
        fi
    fi

    info "Downloading version ${VERSION}..."
    TMPDIR=$(mktemp -d)
    trap "rm -rf $TMPDIR" EXIT

    TARBALL="printrelay-server-linux-amd64.tar.gz"
    URL="https://github.com/${REPO}/releases/download/v${VERSION}/${TARBALL}"

    if ! curl -sL --fail "$URL" -o "$TMPDIR/$TARBALL"; then
        error "Failed to download: $URL"
    fi

    tar -xzf "$TMPDIR/$TARBALL" -C "$TMPDIR"
    BINARY_PATH="$TMPDIR/printrelay-server-linux-amd64"
fi

# Get current version on server
info "Checking current version on server..."
CURRENT_VERSION=$(ssh -i "$SSH_KEY" "${SERVER_USER}@${SERVER_IP}" "printrelay-server --version 2>/dev/null || echo 'unknown'")
echo "  Current: $CURRENT_VERSION"
if [ -n "$VERSION" ]; then
    echo "  New:     v$VERSION"
fi

# Upload binary
info "Uploading binary to server..."
scp -i "$SSH_KEY" "$BINARY_PATH" "${SERVER_USER}@${SERVER_IP}:/tmp/printrelay-server-new"

# Deploy on server
info "Deploying on server..."
ssh -i "$SSH_KEY" "${SERVER_USER}@${SERVER_IP}" bash << 'DEPLOY_EOF'
set -e

# Stop service
echo "  Stopping service..."
systemctl stop printrelay-server || true

# Backup old binary
if [ -f /usr/local/bin/printrelay-server ]; then
    cp /usr/local/bin/printrelay-server /usr/local/bin/printrelay-server.bak
fi

# Install new binary
mv /tmp/printrelay-server-new /usr/local/bin/printrelay-server
chmod +x /usr/local/bin/printrelay-server

# Start service
echo "  Starting service..."
systemctl start printrelay-server

# Check status
sleep 2
if systemctl is-active --quiet printrelay-server; then
    echo "  Service is running"
else
    echo "  WARNING: Service may not be running correctly"
    systemctl status printrelay-server --no-pager || true
fi
DEPLOY_EOF

# Verify deployment
info "Verifying deployment..."
sleep 2
PING_RESULT=$(curl -s https://printers.littlebird.app/ping 2>/dev/null || echo "failed")

if [ "$PING_RESULT" = '"pong"' ]; then
    success "Deployment successful!"
    echo ""
    echo "Server is running at https://printers.littlebird.app"
    echo ""
else
    warn "Server may not be responding correctly. Check logs:"
    echo "  ssh -i $SSH_KEY ${SERVER_USER}@${SERVER_IP} journalctl -u printrelay-server -n 50"
fi
