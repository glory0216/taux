#!/usr/bin/env bash
# taux — extend tmux for AI sessions.
# Manage, observe, and clean up your AI agent sessions — without leaving your terminal.
# Install: set -g @plugin 'glory0216/taux'

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Check if taux binary exists
if ! command -v taux &> /dev/null; then
    # Try to find in current dir
    if [ -x "$CURRENT_DIR/bin/taux" ]; then
        TAUX_BIN="$CURRENT_DIR/bin/taux"
    else
        tmux display-message "taux: binary not found. Run 'make build' in $CURRENT_DIR"
        exit 1
    fi
else
    TAUX_BIN="taux"
fi

# Set status-right to include taux status
tmux set -g status-interval 10
tmux set -g status-right "#($TAUX_BIN status 2>/dev/null)  %H:%M %Y-%m-%d"

# Keybindings
tmux bind H display-popup -E -w 80% -h 80% -T ' taux ' "$TAUX_BIN dashboard"
tmux bind A display-popup -E -w 60% -h 50% -T ' Active Sessions ' "$TAUX_BIN get sessions -s active"
tmux bind S display-popup -E -w 50% -h 40% -T ' Stats ' "$TAUX_BIN get stats"
