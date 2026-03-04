#!/usr/bin/env sh
set -eu

# ── Configuration ──────────────────────────────────────────────
GITHUB_REPO="glory0216/taux"
INSTALL_DIR="${TAUX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${TAUX_VERSION:-latest}"
PATH_NOTICE_SHOWN=0

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
        *":${INSTALL_DIR}:"*) ok "${INSTALL_DIR} is in PATH"; return 0 ;;
    esac

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
        ok "Added PATH entry to $profile"
    fi

    printf "\n"
    printf "${YELLOW}  ┌─────────────────────────────────────────────────────────┐${NC}\n"
    printf "${YELLOW}  │  ${NC}${INSTALL_DIR} is not in your current PATH.${YELLOW}          │${NC}\n"
    printf "${YELLOW}  │                                                         │${NC}\n"
    printf "${YELLOW}  │  ${NC}Run one of the following to start using taux:${YELLOW}          │${NC}\n"
    printf "${YELLOW}  │                                                         │${NC}\n"
    printf "${YELLOW}  │  ${GREEN}source ${profile}${YELLOW}$(printf '%*s' $((33 - ${#profile})) '')│${NC}\n"
    printf "${YELLOW}  │  ${NC}or open a new terminal session${YELLOW}                        │${NC}\n"
    printf "${YELLOW}  └─────────────────────────────────────────────────────────┘${NC}\n"
    PATH_NOTICE_SHOWN=1
}

# ── Install tmux if missing ──────────────────────────────────
install_tmux() {
    os="$1"
    info "Installing tmux..."

    if [ "$os" = "darwin" ]; then
        if command -v brew >/dev/null 2>&1; then
            brew install tmux
        else
            fail "Homebrew is required to install tmux on macOS. Install from https://brew.sh"
        fi
    else
        if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update -qq && sudo apt-get install -y -qq tmux
        elif command -v dnf >/dev/null 2>&1; then
            sudo dnf install -y tmux
        elif command -v yum >/dev/null 2>&1; then
            sudo yum install -y tmux
        elif command -v pacman >/dev/null 2>&1; then
            sudo pacman -S --noconfirm tmux
        elif command -v apk >/dev/null 2>&1; then
            sudo apk add tmux
        else
            fail "Could not detect package manager. Install tmux manually and retry."
        fi
    fi

    if command -v tmux >/dev/null 2>&1; then
        ok "tmux $(tmux -V | sed 's/tmux //') installed"
    else
        fail "tmux installation failed"
    fi
}

# ── Build from source ────────────────────────────────────────
build_from_source() {
    info "No release found. Building from source..."

    if ! command -v go >/dev/null 2>&1; then
        fail "Go is required to build from source. Install from https://go.dev/dl/"
    fi
    ok "Go $(go version | sed -E 's/.*go([0-9]+\.[0-9]+\.[0-9]+).*/\1/') found"

    if ! command -v git >/dev/null 2>&1; then
        fail "git is required to clone the repository. Install git and retry."
    fi

    info "Cloning repository..."
    build_dir=$(mktemp -d)
    trap 'rm -rf "$build_dir"' EXIT
    git clone --depth 1 "https://github.com/${GITHUB_REPO}.git" "${build_dir}/taux"
    cd "${build_dir}/taux"

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

    # tmux setup
    if ! command -v tmux >/dev/null 2>&1; then
        warn "tmux is not installed. taux requires tmux for full functionality."
        printf "  Install tmux now? [Y/n] "
        if [ -t 0 ]; then
            read -r answer </dev/tty || answer="n"
        else
            answer="y"
        fi
        case "$answer" in
            [nN]*) warn "Skipped tmux installation. Install later, then run: taux setup" ;;
            *)     install_tmux "$OS" ;;
        esac
    fi

    if command -v tmux >/dev/null 2>&1; then
        info "Setting up tmux integration..."
        "${INSTALL_DIR}/taux" setup </dev/null 2>/dev/null || true
        ok "tmux configured (prefix+H for dashboard)"
    fi

    printf "\n"
    printf "  ${GREEN}╔═══════════════════════════════════════════════════╗${NC}\n"
    printf "  ${GREEN}║           Installation complete!                  ║${NC}\n"
    printf "  ${GREEN}╚═══════════════════════════════════════════════════╝${NC}\n"
    printf "\n"
    printf "  Quick start:\n"
    printf "    ${GREEN}taux${NC}                 Launch dashboard\n"
    printf "    ${GREEN}taux get sessions${NC}    List all sessions\n"
    printf "    ${GREEN}taux get stats${NC}       View statistics\n"
    printf "\n"
    printf "  tmux shortcuts:\n"
    printf "    ${GREEN}prefix + H${NC}           Dashboard popup\n"
    printf "    ${GREEN}prefix + A${NC}           Active sessions popup\n"
    printf "    ${GREEN}prefix + S${NC}           Stats popup\n"
    printf "\n"
}

# ── Main ──────────────────────────────────────────────────────
main() {
    printf "\n"
    printf "${BLUE}taux installer${NC}\n"
    printf "Manage, observe, and clean up your AI agent sessions.\n\n"

    OS=$(detect_os)
    ARCH=$(detect_arch)

    # Try GitHub release first, fall back to source build
    if resolve_version; then
        info "Installing taux v${VERSION} for ${OS}/${ARCH}"

        ARCHIVE_NAME="taux_${VERSION}_${OS}_${ARCH}.tar.gz"
        BASE_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"

        dl_dir=$(mktemp -d)
        trap 'rm -rf "$dl_dir"' EXIT

        # Download archive and checksums
        info "Downloading ${ARCHIVE_NAME}..."
        if download "${BASE_URL}/${ARCHIVE_NAME}" "${dl_dir}/${ARCHIVE_NAME}" 2>/dev/null; then
            ok "Downloaded"

            info "Verifying checksum..."
            download "${BASE_URL}/checksums.txt" "${dl_dir}/checksums.txt" 2>/dev/null || true
            if [ -f "${dl_dir}/checksums.txt" ]; then
                verify_checksum "${dl_dir}/${ARCHIVE_NAME}" "${dl_dir}/checksums.txt" "$ARCHIVE_NAME"
            fi

            # Extract
            info "Extracting..."
            tar -xzf "${dl_dir}/${ARCHIVE_NAME}" -C "${dl_dir}"
            ok "Extracted"

            # Find the binary (handles both flat and wrapped archives)
            taux_bin=$(find "${dl_dir}" -name taux -type f -perm +111 2>/dev/null | head -1) || true
            if [ -z "$taux_bin" ]; then
                # fallback: look for any file named taux
                taux_bin=$(find "${dl_dir}" -name taux -type f | head -1) || true
            fi
            if [ -z "$taux_bin" ]; then
                fail "Could not find taux binary in the downloaded archive."
            fi

            # Install binary
            mkdir -p "${INSTALL_DIR}"
            cp "$taux_bin" "${INSTALL_DIR}/taux"
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
