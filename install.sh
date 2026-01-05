#!/bin/bash
#
# PrintRelay Installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/schappim/print-relay/main/install.sh | bash
#   curl -sSL https://raw.githubusercontent.com/schappim/print-relay/main/install.sh | bash -s -- --client-only
#   curl -sSL https://raw.githubusercontent.com/schappim/print-relay/main/install.sh | bash -s -- --server-only
#
set -e

REPO="schappim/print-relay"
VERSION="1.1.0"
INSTALL_DIR="/usr/local/bin"
SUDO=""

# Colors (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    NC=''
fi

print_banner() {
    echo -e "${BLUE}"
    echo '╔═══════════════════════════════════════════════════════════╗'
    echo '║                                                           ║'
    echo '║   ██████╗ ██████╗ ██╗███╗   ██╗████████╗                  ║'
    echo '║   ██╔══██╗██╔══██╗██║████╗  ██║╚══██╔══╝                  ║'
    echo '║   ██████╔╝██████╔╝██║██╔██╗ ██║   ██║                     ║'
    echo '║   ██╔═══╝ ██╔══██╗██║██║╚██╗██║   ██║                     ║'
    echo '║   ██║     ██║  ██║██║██║ ╚████║   ██║                     ║'
    echo '║   ╚═╝     ╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝   ╚═╝                     ║'
    echo '║                                                           ║'
    echo '║   ██████╗ ███████╗██╗      █████╗ ██╗   ██╗               ║'
    echo '║   ██╔══██╗██╔════╝██║     ██╔══██╗╚██╗ ██╔╝               ║'
    echo '║   ██████╔╝█████╗  ██║     ███████║ ╚████╔╝                ║'
    echo '║   ██╔══██╗██╔══╝  ██║     ██╔══██║  ╚██╔╝                 ║'
    echo '║   ██║  ██║███████╗███████╗██║  ██║   ██║                  ║'
    echo '║   ╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═╝   ╚═╝                  ║'
    echo '║                                                           ║'
    echo '║   Cloud Print Relay System                                ║'
    echo '║                                                           ║'
    echo '╚═══════════════════════════════════════════════════════════╝'
    echo -e "${NC}"
}

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

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux";;
        Darwin*) echo "darwin";;
        *)       echo "unsupported";;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)      echo "amd64";;
        arm64|aarch64)     echo "arm64";;
        armv7l|armv6l|arm) echo "arm";;
        *)                 echo "unsupported";;
    esac
}

check_dependencies() {
    local missing=()

    if ! command -v curl &> /dev/null; then
        missing+=("curl")
    fi

    if ! command -v tar &> /dev/null; then
        missing+=("tar")
    fi

    if [ ${#missing[@]} -ne 0 ]; then
        error "Missing required dependencies: ${missing[*]}"
    fi
}

setup_install_dir() {
    if [ -w "$INSTALL_DIR" ]; then
        return
    fi

    if command -v sudo &> /dev/null; then
        SUDO="sudo"
        warn "Will use sudo to install to $INSTALL_DIR"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
        warn "Installing to $INSTALL_DIR"
        warn "Make sure $INSTALL_DIR is in your PATH"
    fi
}

download_and_install() {
    local component=$1
    local os=$2
    local arch=$3

    local filename="printrelay-${component}-${os}-${arch}"
    local tarball="${filename}.tar.gz"
    local url="https://github.com/${REPO}/releases/download/v${VERSION}/${tarball}"

    info "Downloading printrelay-${component}..."

    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf $tmpdir" RETURN

    if ! curl -sL --fail "$url" -o "$tmpdir/$tarball" 2>/dev/null; then
        error "Failed to download $url"
    fi

    if ! tar -xzf "$tmpdir/$tarball" -C "$tmpdir" 2>/dev/null; then
        error "Failed to extract $tarball"
    fi

    $SUDO mv "$tmpdir/$filename" "$INSTALL_DIR/printrelay-${component}"
    $SUDO chmod +x "$INSTALL_DIR/printrelay-${component}"

    success "Installed printrelay-${component} to $INSTALL_DIR"
}

print_usage() {
    echo ""
    echo -e "${BOLD}Usage:${NC}"
    echo ""
    if [ "$install_server" = true ]; then
        echo "  Start the server:"
        echo -e "    ${GREEN}printrelay-server -port 8080 -api-key \"\$(openssl rand -hex 32)\" -client-key \"\$(openssl rand -hex 32)\"${NC}"
        echo ""
    fi
    if [ "$install_client" = true ]; then
        echo "  Start the client:"
        echo -e "    ${GREEN}printrelay-client -server \"wss://your-server.com/ws\" -key \"YOUR_CLIENT_KEY\"${NC}"
        echo ""
    fi
    echo -e "  Documentation: ${BLUE}https://github.com/${REPO}${NC}"
    echo ""
}

main() {
    local install_server=true
    local install_client=true

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --server-only)
                install_client=false
                shift
                ;;
            --client-only)
                install_server=false
                shift
                ;;
            --version)
                VERSION="$2"
                shift 2
                ;;
            --help|-h)
                echo "PrintRelay Installer"
                echo ""
                echo "Usage: install.sh [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --server-only    Install only the server"
                echo "  --client-only    Install only the client"
                echo "  --version VER    Install specific version (default: $VERSION)"
                echo "  --help, -h       Show this help message"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done

    print_banner

    # Detect platform
    local os
    local arch
    os=$(detect_os)
    arch=$(detect_arch)

    if [ "$os" = "unsupported" ]; then
        error "Unsupported operating system: $(uname -s)"
    fi

    if [ "$arch" = "unsupported" ]; then
        error "Unsupported architecture: $(uname -m)"
    fi

    info "Detected platform: ${os}/${arch}"
    info "Version: v${VERSION}"
    echo ""

    # Check dependencies
    check_dependencies

    # Setup installation directory
    setup_install_dir
    echo ""

    # Download and install components
    if [ "$install_server" = true ]; then
        download_and_install "server" "$os" "$arch"
    fi

    if [ "$install_client" = true ]; then
        download_and_install "client" "$os" "$arch"
    fi

    echo ""
    success "Installation complete!"
    print_usage
}

main "$@"
