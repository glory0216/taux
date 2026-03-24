#!/usr/bin/env bash
# taux.tmux — TPM plugin entry point.
# Install: set -g @plugin 'glory0216/taux'
#
# Options (set in ~/.tmux.conf before TPM init):
#   @taux-key-dashboard   Dashboard popup key       (default: H)
#   @taux-key-sessions    Active sessions popup key  (default: A)
#   @taux-key-stats       Stats popup key            (default: S)
#   @taux-status          Show status in status-right (default: on)
#   @taux-status-interval Status refresh interval     (default: 10)
#   @taux-notify          Notify on session completion (default: on)

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Helper: read tmux option with default ─────────────────────
get_opt() {
    local option="$1"
    local default="$2"
    local value
    value=$(tmux show-option -gqv "$option")
    echo "${value:-$default}"
}

# ── Resolve binary ────────────────────────────────────────────
resolve_binary() {
    # 1. Already in PATH
    if command -v taux &>/dev/null; then
        echo "taux"
        return
    fi
    # 2. Plugin bin/ directory
    if [ -x "$CURRENT_DIR/bin/taux" ]; then
        echo "$CURRENT_DIR/bin/taux"
        return
    fi
    # 3. ~/.local/bin (curl installer default)
    if [ -x "$HOME/.local/bin/taux" ]; then
        echo "$HOME/.local/bin/taux"
        return
    fi
    echo ""
}

# ── Auto-install if missing ───────────────────────────────────
TAUX_BIN=$(resolve_binary)
if [ -z "$TAUX_BIN" ]; then
    bash "$CURRENT_DIR/scripts/install.sh"
    TAUX_BIN=$(resolve_binary)
    if [ -z "$TAUX_BIN" ]; then
        tmux display-message "taux: installation failed. Run install.sh manually."
        exit 1
    fi
fi

# ── Read user options ─────────────────────────────────────────
KEY_DASHBOARD=$(get_opt "@taux-key-dashboard" "H")
KEY_SESSIONS=$(get_opt "@taux-key-sessions" "A")
KEY_STATS=$(get_opt "@taux-key-stats" "S")
KEY_PEEK=$(get_opt "@taux-key-peek" "P")
STATUS_ENABLED=$(get_opt "@taux-status" "on")
STATUS_INTERVAL=$(get_opt "@taux-status-interval" "10")

# ── Keybindings ───────────────────────────────────────────────
tmux bind-key "$KEY_DASHBOARD" display-popup -E -w 80% -h 80% -T ' taux ' "$TAUX_BIN dashboard --split-target #{pane_id}"
tmux bind-key "$KEY_SESSIONS" display-popup -E -w 60% -h 50% -T ' Active Sessions ' "bash -c '$TAUX_BIN get sessions -s active; read -rsn1'"
tmux bind-key "$KEY_STATS" display-popup -E -w 50% -h 40% -T ' Stats ' "bash -c '$TAUX_BIN get stats; read -rsn1'"
tmux bind-key "$KEY_PEEK" display-popup -E -w 60% -h 50% -T ' Peek ' "bash -c '$TAUX_BIN peek; read -rsn1'"

# ── Window status highlight ──────────────────────────────────
tmux setw -g window-status-style 'fg=colour245'
tmux setw -g window-status-current-style 'fg=colour16,bg=colour39,bold'

# ── Status bar ────────────────────────────────────────────────
if [ "$STATUS_ENABLED" = "on" ]; then
    tmux set -g status-interval "$STATUS_INTERVAL"

    # Append taux status to existing status-right (don't overwrite)
    current_status_right=$(tmux show-option -gqv status-right)
    taux_status_fragment="#($TAUX_BIN status 2>/dev/null)"

    if ! echo "$current_status_right" | grep -q "taux status"; then
        tmux set -g status-right "${taux_status_fragment}  ${current_status_right}"
    fi
fi
