#!/usr/bin/env sh
set -eu

# ── Configuration ──────────────────────────────────────────────
GITHUB_REPO="glory0216/taux"
INSTALL_DIR="${TAUX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${TAUX_VERSION:-latest}"

# ── Color helpers (safe for pipes) ─────────────────────────────
if [ -t 1 ]; then
    BLUE='\033[0;34m'; GREEN='\033[0;32m'
    YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
else
    BLUE=''; GREEN=''; YELLOW=''; RED=''; NC=''
fi

info()  { printf "${BLUE}==> %s${NC}\n" "$1"; }
ok()    { printf "${GREEN}  + %s${NC}\n" "$1"; }
warn()  { printf "${YELLOW}  ! %s${NC}\n" "$1"; }
fail()  { printf "${RED}  x %s${NC}\n" "$1"; exit 1; }

# ── Detect OS ──────────────────────────────────────────────────
detect_os() {
    case "$(uname -s)" in
        Darwin)  echo "darwin" ;;
        Linux)   echo "linux" ;;
        *)       fail "Unsupported OS: $(uname -s). taux supports macOS and Linux." ;;
    esac
}

# ── Detect Architecture ───────────────────────────────────────
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)     echo "amd64" ;;
        aarch64|arm64)    echo "arm64" ;;
        *)                fail "Unsupported architecture: $(uname -m)" ;;
    esac
}

# ── Resolve version ───────────────────────────────────────────
resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(download_silent "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null \
            | grep '"tag_name"' | head -1 | sed -E 's/.*"v([^"]+)".*/\1/') || true
        if [ -z "$VERSION" ]; then
            return 1
        fi
    fi
    VERSION="${VERSION#v}"
    return 0
}

# ── Download helpers ──────────────────────────────────────────
download() {
    url="$1"; dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$dest" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$dest" "$url"
    else
        fail "Neither curl nor wget found. Install one and retry."
    fi
}

download_silent() {
    url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O- "$url"
    else
        fail "Neither curl nor wget found."
    fi
}

# ── Verify checksum ───────────────────────────────────────────
verify_checksum() {
    archive_path="$1"; checksums_path="$2"; archive_name="$3"
    expected=$(grep "${archive_name}" "$checksums_path" | awk '{print $1}')
    if [ -z "$expected" ]; then
        warn "Checksum entry not found; skipping verification."
        return 0
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive_path" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive_path" | awk '{print $1}')
    else
        warn "No sha256sum or shasum found; skipping verification."
        return 0
    fi

    if [ "$expected" != "$actual" ]; then
        fail "Checksum mismatch! Expected: ${expected}, Got: ${actual}"
    fi
    ok "Checksum verified"
}

# ── Ensure PATH includes install dir ──────────────────────────
ensure_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return 0 ;;
    esac

    warn "${INSTALL_DIR} is not in your PATH"
    shell_name="$(basename "${SHELL:-sh}")"
    case "$shell_name" in
        zsh)  profile="$HOME/.zshrc" ;;
        bash) profile="$HOME/.bashrc" ;;
        *)    profile="$HOME/.profile" ;;
    esac

    export_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
    if [ -f "$profile" ] && grep -qF "$INSTALL_DIR" "$profile" 2>/dev/null; then
        ok "${INSTALL_DIR} already referenced in $profile"
    else
        printf '\n# taux\n%s\n' "$export_line" >> "$profile"
        ok "Added PATH entry to $profile -- run 'source $profile' or restart your shell"
    fi
}

# ── Build from source ────────────────────────────────────────
build_from_source() {
    info "No release found. Building from source..."

    if ! command -v go >/dev/null 2>&1; then
        fail "Go is required to build from source. Install from https://go.dev/dl/"
    fi
    ok "Go $(go version | sed -E 's/.*go([0-9]+\.[0-9]+\.[0-9]+).*/\1/') found"

    # Detect script directory (for local clone installs)
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

    if [ -f "${SCRIPT_DIR}/cmd/taux/main.go" ]; then
        info "Building from local source at ${SCRIPT_DIR}..."
        cd "$SCRIPT_DIR"
    else
        info "Cloning repository..."
        BUILD_TMPDIR=$(mktemp -d)
        trap 'rm -rf "$BUILD_TMPDIR"' EXIT
        git clone "https://github.com/${GITHUB_REPO}.git" "${BUILD_TMPDIR}/taux"
        cd "${BUILD_TMPDIR}/taux"
    fi

    mkdir -p "${INSTALL_DIR}"
    go build -ldflags "-s -w -X github.com/glory0216/taux/internal/cli.Version=source" \
        -o "${INSTALL_DIR}/taux" ./cmd/taux/
    chmod +x "${INSTALL_DIR}/taux"
    ok "Built and installed to ${INSTALL_DIR}/taux"
}

# ── Setup config + tmux ──────────────────────────────────────
post_install() {
    # PATH
    ensure_path

    # Verify
    if "${INSTALL_DIR}/taux" --version >/dev/null 2>&1; then
        ok "$("${INSTALL_DIR}/taux" --version)"
    else
        fail "Binary verification failed"
    fi

    # tmux setup (optional)
    if command -v tmux >/dev/null 2>&1; then
        info "Setting up tmux integration..."
        "${INSTALL_DIR}/taux" setup </dev/null 2>/dev/null || true
        ok "tmux configured (prefix+H for dashboard)"
    else
        warn "tmux not found -- install tmux later, then run: taux setup"
    fi

    printf "\n${GREEN}Installation complete!${NC}\n\n"
    printf "  Quick start:\n"
    printf "    taux                 -- Launch dashboard\n"
    printf "    taux get sessions    -- List all sessions\n"
    printf "    taux get stats       -- View statistics\n\n"
    printf "  In tmux:\n"
    printf "    prefix + H           -- Dashboard popup\n"
    printf "    prefix + A           -- Active sessions popup\n"
    printf "    prefix + S           -- Stats popup\n\n"
}

# ── Main ──────────────────────────────────────────────────────
main() {
    printf "\n"
    printf "${BLUE}taux installer${NC}\n"
    printf "extend tmux for AI sessions.\n\n"

    OS=$(detect_os)
    ARCH=$(detect_arch)

    # Try GitHub release first, fall back to source build
    if resolve_version; then
        info "Installing taux v${VERSION} for ${OS}/${ARCH}"

        ARCHIVE_NAME="taux_${VERSION}_${OS}_${ARCH}.tar.gz"
        BASE_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"

        TMPDIR=$(mktemp -d)
        trap 'rm -rf "$TMPDIR"' EXIT

        # Download archive and checksums
        info "Downloading ${ARCHIVE_NAME}..."
        if download "${BASE_URL}/${ARCHIVE_NAME}" "${TMPDIR}/${ARCHIVE_NAME}" 2>/dev/null; then
            ok "Downloaded"

            info "Verifying checksum..."
            download "${BASE_URL}/checksums.txt" "${TMPDIR}/checksums.txt" 2>/dev/null || true
            if [ -f "${TMPDIR}/checksums.txt" ]; then
                verify_checksum "${TMPDIR}/${ARCHIVE_NAME}" "${TMPDIR}/checksums.txt" "$ARCHIVE_NAME"
            fi

            # Extract
            info "Extracting..."
            tar -xzf "${TMPDIR}/${ARCHIVE_NAME}" -C "${TMPDIR}"
            ok "Extracted"

            # Install binary
            mkdir -p "${INSTALL_DIR}"
            cp "${TMPDIR}/taux" "${INSTALL_DIR}/taux"
            chmod +x "${INSTALL_DIR}/taux"
            ok "Installed to ${INSTALL_DIR}/taux"
        else
            warn "Download failed. Falling back to source build..."
            build_from_source
        fi
    else
        warn "No GitHub release found. Building from source..."
        build_from_source
    fi

    post_install
}

main "$@"
