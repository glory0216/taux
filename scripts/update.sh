#!/usr/bin/env bash
# scripts/update.sh — Force re-download of taux binary.
# Called by TPM on `prefix + U` (update plugins).

set -euo pipefail

PLUGIN_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Remove existing binary to force re-download
if [ -d "$HOME/.local/bin" ] && [ -x "$HOME/.local/bin/taux" ]; then
    rm -f "$HOME/.local/bin/taux"
elif [ -x "$PLUGIN_DIR/bin/taux" ]; then
    rm -f "$PLUGIN_DIR/bin/taux"
fi

# Re-run install
exec "$PLUGIN_DIR/scripts/install.sh"
