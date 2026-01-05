#!/bin/bash
#
# PrintRelay Release Script
#
# Automates the entire release process:
# - Builds binaries for all platforms
# - Creates GitHub release with tarballs
# - Updates Homebrew formulas
# - Updates install.sh version
# - Creates .deb packages for APT repository
# - Signs and publishes APT repository
#
# Usage:
#   ./scripts/release.sh <version>
#   ./scripts/release.sh 1.1.0
#   ./scripts/release.sh 1.1.0 --dry-run
#
set -e

# Configuration
REPO_OWNER="schappim"
REPO_NAME="print-relay"
HOMEBREW_REPO="homebrew-printrelay"
APT_REPO="printrelay-apt"
GPG_KEY="printrelay@littlebird.com.au"

# Paths (adjust these if your directory structure differs)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HOMEBREW_PATH="${PROJECT_ROOT}/../${HOMEBREW_REPO}"
APT_PATH="${PROJECT_ROOT}/../${APT_REPO}"
DIST_PATH="${PROJECT_ROOT}/dist"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Flags
DRY_RUN=false
SKIP_PUSH=false

#------------------------------------------------------------------------------
# Helper Functions
#------------------------------------------------------------------------------

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

run() {
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[DRY-RUN]${NC} $*"
    else
        "$@"
    fi
}

check_dependencies() {
    local missing=()

    for cmd in go git gh dpkg-deb gpg shasum curl tar; do
        if ! command -v "$cmd" &> /dev/null; then
            missing+=("$cmd")
        fi
    done

    if [ ${#missing[@]} -ne 0 ]; then
        error "Missing dependencies: ${missing[*]}"
    fi
}

check_gpg_key() {
    if ! gpg --list-keys "$GPG_KEY" &> /dev/null; then
        error "GPG key not found: $GPG_KEY"
    fi
}

check_paths() {
    if [ ! -d "$PROJECT_ROOT" ]; then
        error "Project root not found: $PROJECT_ROOT"
    fi

    if [ ! -d "$HOMEBREW_PATH" ]; then
        error "Homebrew repo not found: $HOMEBREW_PATH"
    fi

    if [ ! -d "$APT_PATH" ]; then
        error "APT repo not found: $APT_PATH"
    fi
}

#------------------------------------------------------------------------------
# Build Functions
#------------------------------------------------------------------------------

build_binaries() {
    info "Building binaries for version $VERSION..."

    mkdir -p "$DIST_PATH"
    cd "$PROJECT_ROOT"

    # Build matrix: OS x ARCH x COMPONENT
    local platforms=(
        "darwin:amd64"
        "darwin:arm64"
        "linux:amd64"
        "linux:arm64"
        "linux:arm"
    )

    local components=("server" "client")

    for platform in "${platforms[@]}"; do
        IFS=':' read -r os arch <<< "$platform"
        for component in "${components[@]}"; do
            local output="printrelay-${component}-${os}-${arch}"
            info "  Building ${output}..."

            if [ "$DRY_RUN" = false ]; then
                GOOS="$os" GOARCH="$arch" go build \
                    -ldflags "-s -w -X main.Version=${VERSION}" \
                    -o "${DIST_PATH}/${output}" \
                    "./cmd/${component}"
            fi
        done
    done

    success "All binaries built"
}

create_tarballs() {
    info "Creating tarballs..."

    cd "$DIST_PATH"

    for binary in printrelay-*; do
        if [[ -f "$binary" && ! "$binary" =~ \.tar\.gz$ ]]; then
            local tarball="${binary}.tar.gz"
            info "  Creating ${tarball}..."
            run tar -czvf "$tarball" "$binary" > /dev/null
        fi
    done

    success "All tarballs created"
}

calculate_checksums() {
    info "Calculating SHA256 checksums..."

    cd "$DIST_PATH"

    # Write checksums to a file for later reference
    CHECKSUMS_FILE="${DIST_PATH}/checksums.txt"
    rm -f "$CHECKSUMS_FILE"

    for tarball in *.tar.gz; do
        if [ -f "$tarball" ]; then
            local sha256
            sha256=$(shasum -a 256 "$tarball" | cut -d' ' -f1)
            echo "${tarball}:${sha256}" >> "$CHECKSUMS_FILE"
            echo "  $tarball: $sha256"
        fi
    done

    success "Checksums calculated"
}

get_checksum() {
    local filename=$1
    grep "^${filename}:" "${DIST_PATH}/checksums.txt" | cut -d':' -f2
}

#------------------------------------------------------------------------------
# GitHub Release Functions
#------------------------------------------------------------------------------

create_github_release() {
    info "Creating GitHub release v${VERSION}..."

    cd "$PROJECT_ROOT"

    # Check if release already exists
    if gh release view "v${VERSION}" --repo "${REPO_OWNER}/${REPO_NAME}" &> /dev/null; then
        warn "Release v${VERSION} already exists, updating..."
        run gh release delete "v${VERSION}" --repo "${REPO_OWNER}/${REPO_NAME}" --yes || true
    fi

    # Create release
    run gh release create "v${VERSION}" \
        --repo "${REPO_OWNER}/${REPO_NAME}" \
        --title "v${VERSION}" \
        --notes "## PrintRelay v${VERSION}

### Installation

**macOS (Homebrew):**
\`\`\`bash
brew tap ${REPO_OWNER}/printrelay
brew install printrelay-server printrelay-client
\`\`\`

**macOS & Linux (Script):**
\`\`\`bash
curl -sSL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/install.sh | bash
\`\`\`

**Debian/Ubuntu (APT):**
\`\`\`bash
curl -fsSL https://${REPO_OWNER}.github.io/${APT_REPO}/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/printrelay.gpg
echo \"deb [signed-by=/usr/share/keyrings/printrelay.gpg] https://${REPO_OWNER}.github.io/${APT_REPO} stable main\" | sudo tee /etc/apt/sources.list.d/printrelay.list
sudo apt update && sudo apt install printrelay-server printrelay-client
\`\`\`
" \
        "${DIST_PATH}"/*.tar.gz

    success "GitHub release created"
}

#------------------------------------------------------------------------------
# Homebrew Functions
#------------------------------------------------------------------------------

update_homebrew_formula() {
    local component=$1
    local formula_path="${HOMEBREW_PATH}/Formula/printrelay-${component}.rb"

    info "Updating Homebrew formula: printrelay-${component}..."

    if [ ! -f "$formula_path" ]; then
        error "Formula not found: $formula_path"
    fi

    if [ "$DRY_RUN" = true ]; then
        echo "[DRY-RUN] Would update $formula_path with version $VERSION"
        return
    fi

    # Update version
    sed -i '' "s/version \"[^\"]*\"/version \"${VERSION}\"/" "$formula_path"

    # Update URLs and checksums for each architecture
    local darwin_amd64_sha
    local darwin_arm64_sha
    darwin_amd64_sha=$(get_checksum "printrelay-${component}-darwin-amd64.tar.gz")
    darwin_arm64_sha=$(get_checksum "printrelay-${component}-darwin-arm64.tar.gz")

    # Update amd64
    sed -i '' "s|releases/download/v[^/]*/printrelay-${component}-darwin-amd64|releases/download/v${VERSION}/printrelay-${component}-darwin-amd64|" "$formula_path"
    sed -i '' "/darwin-amd64/,/sha256/{s/sha256 \"[^\"]*\"/sha256 \"${darwin_amd64_sha}\"/;}" "$formula_path"

    # Update arm64
    sed -i '' "s|releases/download/v[^/]*/printrelay-${component}-darwin-arm64|releases/download/v${VERSION}/printrelay-${component}-darwin-arm64|" "$formula_path"
    sed -i '' "/darwin-arm64/,/sha256/{s/sha256 \"[^\"]*\"/sha256 \"${darwin_arm64_sha}\"/;}" "$formula_path"

    success "Updated printrelay-${component} formula"
}

update_homebrew() {
    info "Updating Homebrew formulas..."

    update_homebrew_formula "server"
    update_homebrew_formula "client"

    if [ "$DRY_RUN" = false ]; then
        cd "$HOMEBREW_PATH"
        git add -A
        git commit -m "Update to v${VERSION}" || true

        if [ "$SKIP_PUSH" = false ]; then
            run git push
        fi
    fi

    success "Homebrew formulas updated"
}

#------------------------------------------------------------------------------
# Install Script Functions
#------------------------------------------------------------------------------

update_install_script() {
    info "Updating install.sh version..."

    local install_script="${PROJECT_ROOT}/install.sh"

    if [ "$DRY_RUN" = true ]; then
        echo "[DRY-RUN] Would update VERSION in $install_script to $VERSION"
        return
    fi

    sed -i '' "s/^VERSION=\"[^\"]*\"/VERSION=\"${VERSION}\"/" "$install_script"

    success "install.sh updated"
}

#------------------------------------------------------------------------------
# APT Repository Functions
#------------------------------------------------------------------------------

build_deb_package() {
    local component=$1
    local arch=$2
    local deb_arch=$3

    local binary_name="printrelay-${component}-linux-${arch}"
    local deb_name="printrelay-${component}_${VERSION}_${deb_arch}.deb"
    local pool_dir="${APT_PATH}/pool/main/p/printrelay"

    info "  Building ${deb_name}..."

    if [ "$DRY_RUN" = true ]; then
        return
    fi

    local pkgdir
    pkgdir=$(mktemp -d)

    mkdir -p "${pkgdir}/DEBIAN"
    mkdir -p "${pkgdir}/usr/bin"

    cp "${DIST_PATH}/${binary_name}" "${pkgdir}/usr/bin/printrelay-${component}"
    chmod 755 "${pkgdir}/usr/bin/printrelay-${component}"

    local description
    if [ "$component" = "server" ]; then
        description="Cloud print relay server - enables remote printing via REST API"
    else
        description="Cloud print relay client - connects local printers to PrintRelay server"
    fi

    cat > "${pkgdir}/DEBIAN/control" << EOF
Package: printrelay-${component}
Version: ${VERSION}
Section: net
Priority: optional
Architecture: ${deb_arch}
Maintainer: PrintRelay <${GPG_KEY}>
Description: ${description}
Homepage: https://github.com/${REPO_OWNER}/${REPO_NAME}
EOF

    mkdir -p "$pool_dir"
    dpkg-deb --build --root-owner-group "$pkgdir" "${pool_dir}/${deb_name}"
    rm -rf "$pkgdir"
}

build_deb_packages() {
    info "Building .deb packages..."

    # Remove old .deb files
    if [ "$DRY_RUN" = false ]; then
        rm -f "${APT_PATH}/pool/main/p/printrelay/"*.deb 2>/dev/null || true
    fi

    build_deb_package "server" "amd64" "amd64"
    build_deb_package "server" "arm64" "arm64"
    build_deb_package "server" "arm" "armhf"
    build_deb_package "client" "amd64" "amd64"
    build_deb_package "client" "arm64" "arm64"
    build_deb_package "client" "arm" "armhf"

    success "All .deb packages built"
}

update_apt_metadata() {
    info "Updating APT repository metadata..."

    if [ "$DRY_RUN" = true ]; then
        echo "[DRY-RUN] Would regenerate Packages and Release files"
        return
    fi

    cd "$APT_PATH"

    # Ensure all architecture directories exist
    mkdir -p dists/stable/main/binary-amd64
    mkdir -p dists/stable/main/binary-arm64
    mkdir -p dists/stable/main/binary-armhf

    # Generate Packages files
    dpkg-scanpackages --arch amd64 pool/ > dists/stable/main/binary-amd64/Packages
    gzip -9c dists/stable/main/binary-amd64/Packages > dists/stable/main/binary-amd64/Packages.gz

    dpkg-scanpackages --arch arm64 pool/ > dists/stable/main/binary-arm64/Packages
    gzip -9c dists/stable/main/binary-arm64/Packages > dists/stable/main/binary-arm64/Packages.gz

    dpkg-scanpackages --arch armhf pool/ > dists/stable/main/binary-armhf/Packages
    gzip -9c dists/stable/main/binary-armhf/Packages > dists/stable/main/binary-armhf/Packages.gz

    # Generate Release file
    cd dists/stable

    cat > Release << EOF
Origin: PrintRelay
Label: PrintRelay
Suite: stable
Codename: stable
Version: ${VERSION}
Architectures: amd64 arm64 armhf
Components: main
Description: PrintRelay APT Repository
Date: $(date -Ru)
EOF

    # Add checksums
    echo "MD5Sum:" >> Release
    for f in main/binary-*/Packages*; do
        echo " $(md5 -q "$f") $(wc -c < "$f" | tr -d ' ') $f" >> Release
    done

    echo "SHA256:" >> Release
    for f in main/binary-*/Packages*; do
        echo " $(shasum -a 256 "$f" | cut -d' ' -f1) $(wc -c < "$f" | tr -d ' ') $f" >> Release
    done

    # Sign Release file
    rm -f Release.gpg InRelease
    gpg --default-key "$GPG_KEY" -abs -o Release.gpg Release
    gpg --default-key "$GPG_KEY" --clearsign -o InRelease Release

    success "APT metadata updated and signed"
}

update_apt_repo() {
    info "Updating APT repository..."

    build_deb_packages
    update_apt_metadata

    if [ "$DRY_RUN" = false ]; then
        cd "$APT_PATH"
        git add -A
        git commit -m "Update to v${VERSION}" || true

        if [ "$SKIP_PUSH" = false ]; then
            run git push
        fi
    fi

    success "APT repository updated"
}

#------------------------------------------------------------------------------
# Main Release Functions
#------------------------------------------------------------------------------

commit_and_push_main() {
    info "Committing changes to main repository..."

    if [ "$DRY_RUN" = true ]; then
        echo "[DRY-RUN] Would commit and push main repo"
        return
    fi

    cd "$PROJECT_ROOT"
    git add -A
    git commit -m "Release v${VERSION}" || true

    if [ "$SKIP_PUSH" = false ]; then
        run git push
    fi

    success "Main repository updated"
}

print_summary() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}          ${BOLD}Release v${VERSION} Complete!${NC}                       ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Repositories updated:"
    echo "  - https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/tag/v${VERSION}"
    echo "  - https://github.com/${REPO_OWNER}/${HOMEBREW_REPO}"
    echo "  - https://github.com/${REPO_OWNER}/${APT_REPO}"
    echo ""
    echo "Installation commands:"
    echo ""
    echo "  Homebrew (macOS):"
    echo "    brew update && brew upgrade printrelay-server printrelay-client"
    echo ""
    echo "  Script (macOS & Linux):"
    echo "    curl -sSL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/install.sh | bash"
    echo ""
    echo "  APT (Debian/Ubuntu):"
    echo "    sudo apt update && sudo apt upgrade printrelay-server printrelay-client"
    echo ""
}

show_usage() {
    echo "PrintRelay Release Script"
    echo ""
    echo "Usage: $0 <version> [options]"
    echo ""
    echo "Arguments:"
    echo "  version     Version number (e.g., 1.1.0)"
    echo ""
    echo "Options:"
    echo "  --dry-run   Show what would be done without making changes"
    echo "  --skip-push Skip pushing to remote repositories"
    echo "  --help      Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 1.1.0              # Release version 1.1.0"
    echo "  $0 1.1.0 --dry-run    # Preview release without changes"
    echo ""
}

main() {
    # Parse arguments
    if [ $# -lt 1 ]; then
        show_usage
        exit 1
    fi

    VERSION=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --skip-push)
                SKIP_PUSH=true
                shift
                ;;
            --help|-h)
                show_usage
                exit 0
                ;;
            *)
                if [ -z "$VERSION" ]; then
                    VERSION="$1"
                else
                    error "Unknown option: $1"
                fi
                shift
                ;;
        esac
    done

    if [ -z "$VERSION" ]; then
        error "Version number is required"
    fi

    # Validate version format
    if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        error "Invalid version format. Use semantic versioning (e.g., 1.1.0)"
    fi

    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}          ${BOLD}PrintRelay Release Script${NC}                        ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}          Version: ${GREEN}${VERSION}${NC}                                  ${BLUE}║${NC}"
    if [ "$DRY_RUN" = true ]; then
        echo -e "${BLUE}║${NC}          Mode: ${YELLOW}DRY RUN${NC}                                   ${BLUE}║${NC}"
    fi
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Pre-flight checks
    info "Running pre-flight checks..."
    check_dependencies
    check_gpg_key
    check_paths
    success "All checks passed"
    echo ""

    # Build
    build_binaries
    create_tarballs
    calculate_checksums
    echo ""

    # Update install.sh first (before GitHub release)
    update_install_script
    echo ""

    # Commit main repo changes before creating release
    commit_and_push_main
    echo ""

    # Create GitHub release
    create_github_release
    echo ""

    # Update Homebrew
    update_homebrew
    echo ""

    # Update APT
    update_apt_repo
    echo ""

    # Summary
    if [ "$DRY_RUN" = false ]; then
        print_summary
    else
        echo ""
        warn "DRY RUN complete. No changes were made."
        echo ""
    fi
}

main "$@"
