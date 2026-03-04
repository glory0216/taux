#!/usr/bin/env bash
# scripts/install.sh — Download taux binary from GitHub Releases.
# Called automatically by taux.tmux when the binary is missing.

set -euo pipefail

PLUGIN_DIR="$(cd "$(dirname "$0")/.." && pwd)"
GITHUB_REPO="glory0216/taux"

# ── Platform detection ────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) tmux display-message "taux: unsupported architecture $(uname -m)"; exit 1 ;;
esac

# ── Resolve install location ─────────────────────────────────
# Prefer ~/.local/bin (shared with curl installer), fallback to plugin bin/
if [ -d "$HOME/.local/bin" ] && echo "$PATH" | tr ':' '\n' | grep -q "$HOME/.local/bin"; then
    INSTALL_DIR="$HOME/.local/bin"
else
    INSTALL_DIR="$PLUGIN_DIR/bin"
fi

BINARY="$INSTALL_DIR/taux"

# Skip if already installed
if [ -x "$BINARY" ]; then
    exit 0
fi

# ── Resolve latest version ───────────────────────────────────
VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"v([^"]+)".*/\1/') || true

if [ -z "$VERSION" ]; then
    tmux display-message "taux: could not resolve latest version"
    exit 1
fi

# ── Download and install ─────────────────────────────────────
ARCHIVE="taux_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${ARCHIVE}"

dl_dir=$(mktemp -d)
trap 'rm -rf "$dl_dir"' EXIT

if ! curl -fsSL -o "${dl_dir}/${ARCHIVE}" "$URL" 2>/dev/null; then
    tmux display-message "taux: download failed for ${ARCHIVE}"
    exit 1
fi

tar -xzf "${dl_dir}/${ARCHIVE}" -C "${dl_dir}"

# Find binary (handles both flat and wrapped archives)
taux_bin=$(find "${dl_dir}" -name taux -type f -perm +111 2>/dev/null | head -1) || true
if [ -z "$taux_bin" ]; then
    taux_bin=$(find "${dl_dir}" -name taux -type f | head -1) || true
fi
if [ -z "$taux_bin" ]; then
    tmux display-message "taux: binary not found in archive"
    exit 1
fi

mkdir -p "$INSTALL_DIR"
cp "$taux_bin" "$BINARY"
chmod +x "$BINARY"

tmux display-message "taux v${VERSION} installed to ${BINARY}"
