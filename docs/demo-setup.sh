#!/bin/bash
# docs/demo-setup.sh — Create temporary demo data for VHS recording.
# Usage: bash docs/demo-setup.sh
# This creates isolated demo sessions in /tmp/taux-demo so real sessions are hidden.

set -euo pipefail

DEMO_HOME="/tmp/taux-demo"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

CLAUDE_DIR="$DEMO_HOME/.claude"
CODEX_DIR="$DEMO_HOME/.codex"
GEMINI_DIR="$DEMO_HOME/.gemini"
CONFIG_DIR="$DEMO_HOME/.config/taux"

echo "=== taux demo setup ==="

# -------------------------------------------------------------------
# 1. Build taux binary
# -------------------------------------------------------------------
echo "[1/7] Building taux..."
(cd "$PROJECT_DIR" && go build -o bin/taux ./cmd/taux/)

# -------------------------------------------------------------------
# 2. Clean & create directory tree
# -------------------------------------------------------------------
echo "[2/7] Creating directories..."
rm -rf "$DEMO_HOME"
mkdir -p "$DEMO_HOME/.local/bin"
mkdir -p "$CONFIG_DIR"
mkdir -p "$CLAUDE_DIR/projects"
mkdir -p "$CODEX_DIR/sessions"
mkdir -p "$GEMINI_DIR/tmp"
touch "$DEMO_HOME/.zshrc"
cp "$PROJECT_DIR/bin/taux" "$DEMO_HOME/.local/bin/taux"

# -------------------------------------------------------------------
# 3. Generate timestamps via Python (cross-platform)
# -------------------------------------------------------------------
gen_ts() {
  python3 -c "
from datetime import datetime, timezone, timedelta
print((datetime.now(timezone.utc) - timedelta($1)).strftime('%Y-%m-%dT%H:%M:%SZ'))
"
}

gen_unix() {
  python3 -c "
from datetime import datetime, timezone, timedelta
print(f'{(datetime.now(timezone.utc) - timedelta($1)).timestamp():.3f}')
"
}

today_str() {
  python3 -c "from datetime import datetime, timezone; print(datetime.now(timezone.utc).strftime('%Y-%m-%d'))"
}

# Generate padding text (~2600 chars, JSON-safe)
PAD_TEXT=$(python3 -c "print('This implementation follows standard patterns with proper error handling, input validation, and comprehensive test coverage for all edge cases. ' * 18)")

# -------------------------------------------------------------------
# 4. Helper: create Claude JSONL session
# -------------------------------------------------------------------
create_claude() {
  local proj_enc="$1" uuid="$2" model="$3"
  local start_offset="$4" end_offset="$5"
  local desc="$6" branch="$7" cwd="$8" target_kb="$9"

  local start_ts end_ts
  start_ts=$(gen_ts "$start_offset")
  end_ts=$(gen_ts "$end_offset")

  local dir="$CLAUDE_DIR/projects/$proj_enc"
  mkdir -p "$dir"
  local file="$dir/$uuid.jsonl"

  # First line: user message (provides description + timestamp)
  echo "{\"uuid\":\"u001\",\"sessionId\":\"$uuid\",\"type\":\"user\",\"timestamp\":\"$start_ts\",\"cwd\":\"$cwd\",\"version\":\"1.0.13\",\"gitBranch\":\"$branch\",\"message\":{\"role\":\"user\",\"content\":\"$desc\"}}" > "$file"

  # Second line: assistant with model + usage
  echo "{\"uuid\":\"u002\",\"sessionId\":\"$uuid\",\"type\":\"assistant\",\"timestamp\":\"$start_ts\",\"cwd\":\"$cwd\",\"version\":\"1.0.13\",\"gitBranch\":\"$branch\",\"message\":{\"model\":\"$model\",\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"I will help you with that. Let me analyze the codebase and implement the changes.\"}],\"usage\":{\"input_tokens\":1500,\"output_tokens\":800,\"cache_read_input_tokens\":45000,\"cache_creation_input_tokens\":2000}}}" >> "$file"

  # Pad to target size
  local target_bytes=$((target_kb * 1024))
  local current
  current=$(wc -c < "$file")
  while [ "$current" -lt "$target_bytes" ]; do
    echo "{\"uuid\":\"p\",\"type\":\"assistant\",\"timestamp\":\"$start_ts\",\"message\":{\"model\":\"$model\",\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"$PAD_TEXT\"}],\"usage\":{\"input_tokens\":1800,\"output_tokens\":950,\"cache_read_input_tokens\":42000,\"cache_creation_input_tokens\":1800}}}" >> "$file"
    current=$(wc -c < "$file")
  done

  # Last line: final timestamp
  echo "{\"uuid\":\"final\",\"type\":\"assistant\",\"timestamp\":\"$end_ts\",\"cwd\":\"$cwd\",\"message\":{\"model\":\"$model\",\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"All changes applied successfully.\"}]}}" >> "$file"
}

# -------------------------------------------------------------------
# 5. Helper: create Codex JSONL session
# -------------------------------------------------------------------
create_codex() {
  local date_dir="$1" session_id="$2" model="$3"
  local start_offset="$4" end_offset="$5"
  local desc="$6" target_kb="$7"

  local start_unix end_unix
  start_unix=$(gen_unix "$start_offset")
  end_unix=$(gen_unix "$end_offset")

  local dir="$CODEX_DIR/sessions/$date_dir"
  mkdir -p "$dir"
  local file="$dir/rollout-$session_id.jsonl"

  echo "{\"type\":\"thread.started\",\"timestamp\":$start_unix,\"payload\":{\"model\":\"$model\"}}" > "$file"
  echo "{\"type\":\"item.created\",\"timestamp\":$start_unix,\"payload\":{\"item\":{\"role\":\"user\",\"type\":\"message\",\"content\":[{\"type\":\"input_text\",\"text\":\"$desc\"}]}}}" >> "$file"
  echo "{\"type\":\"item.created\",\"timestamp\":$start_unix,\"payload\":{\"item\":{\"role\":\"assistant\",\"type\":\"message\",\"content\":[{\"type\":\"text\",\"text\":\"I will implement the changes now.\"}]}}}" >> "$file"

  # Pad
  local target_bytes=$((target_kb * 1024))
  local current
  current=$(wc -c < "$file")
  while [ "$current" -lt "$target_bytes" ]; do
    echo "{\"type\":\"item.created\",\"timestamp\":$start_unix,\"payload\":{\"item\":{\"role\":\"assistant\",\"type\":\"message\",\"content\":[{\"type\":\"text\",\"text\":\"$PAD_TEXT\"}]}}}" >> "$file"
    current=$(wc -c < "$file")
  done

  echo "{\"type\":\"token_count\",\"timestamp\":$end_unix,\"payload\":{\"input\":2400,\"cached_input\":800,\"output\":1200,\"reasoning\":150,\"total\":4550}}" >> "$file"
  echo "{\"type\":\"turn.completed\",\"timestamp\":$end_unix,\"payload\":{\"model\":\"$model\"}}" >> "$file"
}

# -------------------------------------------------------------------
# 6. Helper: create Gemini JSON session
# -------------------------------------------------------------------
create_gemini() {
  local hash="$1" session_id="$2" model="$3"
  local desc="$4" msg_count="$5" project_name="$6" age_offset="$7"

  local chat_dir="$GEMINI_DIR/tmp/$hash/chats"
  mkdir -p "$chat_dir"

  # Create project metadata
  echo "{\"name\": \"$project_name\"}" > "$GEMINI_DIR/tmp/$hash/metadata.json"

  # Build messages array
  local file="$chat_dir/$session_id.json"
  local messages="["

  # First user + model pair
  messages="$messages{\"role\":\"user\",\"parts\":[{\"text\":\"$desc\"}]},"
  messages="$messages{\"role\":\"model\",\"parts\":[{\"text\":\"I will help you implement this step by step.\"}],\"usageMetadata\":{\"model\":\"$model\",\"promptTokenCount\":200,\"candidatesTokenCount\":450,\"totalTokenCount\":650}}"

  # Add more message pairs
  local i=2
  while [ "$i" -lt "$msg_count" ]; do
    messages="$messages,{\"role\":\"user\",\"parts\":[{\"text\":\"Continue with the next step of the implementation.\"}]}"
    messages="$messages,{\"role\":\"model\",\"parts\":[{\"text\":\"Here is the next implementation step with proper error handling and validation.\"}],\"usageMetadata\":{\"model\":\"$model\",\"promptTokenCount\":$((150 + i * 10)),\"candidatesTokenCount\":$((300 + i * 15)),\"totalTokenCount\":$((450 + i * 25))}}"
    i=$((i + 2))
  done

  messages="$messages]"
  echo "{\"messages\":$messages}" > "$file"

  # Set file modtime for proper AGE display (Gemini uses file modtime)
  local touch_ts
  touch_ts=$(python3 -c "
from datetime import datetime, timezone, timedelta
t = datetime.now(timezone.utc) - timedelta($age_offset)
print(t.strftime('%Y%m%d%H%M.%S'))
")
  touch -t "$touch_ts" "$file"
}

# -------------------------------------------------------------------
# Claude Sessions (14)
# -------------------------------------------------------------------
echo "[3/7] Creating Claude sessions (14)..."

WD="-Users-dev-projects-web-dashboard"
ML="-Users-dev-projects-ml-pipeline"
MA="-Users-dev-projects-mobile-app"
IT="-Users-dev-projects-infra-terraform"
AG="-Users-dev-projects-api-gateway"
DS="-Users-dev-projects-data-service"
CT="-Users-dev-projects-cli-tools"
DO="-Users-dev-projects-docs-site"

# Session 1: web-dashboard / opus-4-6 / 45m ago / alias: auth-refactor
create_claude "$WD" "a1b2c3d4-e5f6-7890-abcd-ef1234567890" "claude-opus-4-6" \
  "minutes=45" "minutes=15" \
  "Implement OAuth2 login flow with Google provider" "feature/auth" \
  "/Users/dev/projects/web-dashboard" 900

# Session 2: web-dashboard / sonnet-4-6 / 2h ago / alias: fix-timeout
create_claude "$WD" "b2c3d4e5-f6a7-8901-bcde-f12345678901" "claude-sonnet-4-6" \
  "hours=2" "hours=1, minutes=30" \
  "Fix session timeout bug in middleware" "fix/session-timeout" \
  "/Users/dev/projects/web-dashboard" 300

# Session 3: ml-pipeline / opus-4-5 / 3h ago / alias: redis-cache
create_claude "$ML" "c3d4e5f6-a7b8-9012-cdef-123456789012" "claude-opus-4-5-20251101" \
  "hours=3" "hours=2, minutes=45" \
  "Add Redis caching layer for model predictions" "feature/redis-cache" \
  "/Users/dev/projects/ml-pipeline" 1200

# Session 4: mobile-app / haiku-4-5 / 1d ago
create_claude "$MA" "d4e5f6a7-b8c9-0123-defa-234567890123" "claude-haiku-4-5" \
  "days=1" "hours=23" \
  "Create notification settings screen" "feature/notifications" \
  "/Users/dev/projects/mobile-app" 200

# Session 5: infra-terraform / opus-4-6 / 5h ago / alias: k8s-deploy
create_claude "$IT" "e5f6a7b8-c9d0-1234-efab-567890123456" "claude-opus-4-6" \
  "hours=5" "hours=4, minutes=30" \
  "Setup Kubernetes deployment manifests" "feature/k8s-deploy" \
  "/Users/dev/projects/infra-terraform" 750

# Session 6: api-gateway / sonnet-4-6 / 8h ago
create_claude "$AG" "f6a7b8c9-d0e1-2345-fabc-678901234567" "claude-sonnet-4-6" \
  "hours=8" "hours=7" \
  "Add rate limiting middleware" "feature/rate-limit" \
  "/Users/dev/projects/api-gateway" 450

# Session 7: web-dashboard / opus-4-6 / 1d ago / alias: perf-optimize
create_claude "$WD" "a7b8c9d0-e1f2-3456-abcd-789012345678" "claude-opus-4-6" \
  "days=1" "hours=22" \
  "Optimize database queries for dashboard" "perf/db-queries" \
  "/Users/dev/projects/web-dashboard" 1500

# Session 8: data-service / sonnet-4-6 / 30m ago / alias: db-migration
create_claude "$DS" "b8c9d0e1-f2a3-4567-bcde-890123456789" "claude-sonnet-4-6" \
  "minutes=30" "minutes=10" \
  "Migrate user table to new schema" "feature/db-migration" \
  "/Users/dev/projects/data-service" 350

# Session 9: cli-tools / opus-4-6 / 2d ago
create_claude "$CT" "c9d0e1f2-a3b4-5678-cdef-901234567890" "claude-opus-4-6" \
  "days=2" "days=2" \
  "Add JSON output format for all commands" "feature/json-output" \
  "/Users/dev/projects/cli-tools" 270

# Session 10: docs-site / haiku-4-5 / 3d ago / alias: readme-update
create_claude "$DO" "d0e1f2a3-b4c5-6789-defa-012345678901" "claude-haiku-4-5" \
  "days=3" "days=3" \
  "Update API reference documentation" "docs/api-reference" \
  "/Users/dev/projects/docs-site" 100

# Session 11: api-gateway / opus-4-5 / 12h ago
create_claude "$AG" "e1f2a3b4-c5d6-7890-efab-123456789012" "claude-opus-4-5-20251101" \
  "hours=12" "hours=11" \
  "Implement GraphQL subscriptions" "feature/graphql-subs" \
  "/Users/dev/projects/api-gateway" 600

# Session 12: ml-pipeline / sonnet-4-6 / 6h ago
create_claude "$ML" "f2a3b4c5-d6e7-8901-fabc-234567890123" "claude-sonnet-4-6" \
  "hours=6" "hours=5, minutes=30" \
  "Write unit tests for data preprocessing" "test/preprocessing" \
  "/Users/dev/projects/ml-pipeline" 400

# Session 13: web-dashboard / sonnet-4-6 / 4d ago
create_claude "$WD" "a3b4c5d6-e7f8-9012-abcd-345678901234" "claude-sonnet-4-6" \
  "days=4" "days=4" \
  "Add dark mode theme support" "feature/dark-mode" \
  "/Users/dev/projects/web-dashboard" 550

# Session 14: infra-terraform / opus-4-6 / 2d ago
create_claude "$IT" "b4c5d6e7-f8a9-0123-bcde-456789012345" "claude-opus-4-6" \
  "days=2" "days=2" \
  "Setup monitoring alerts for production" "ops/monitoring-alerts" \
  "/Users/dev/projects/infra-terraform" 680

# -------------------------------------------------------------------
# Codex Sessions (4)
# -------------------------------------------------------------------
echo "[4/7] Creating Codex sessions (4)..."

CODEX_DATE1=$(python3 -c "from datetime import datetime,timezone,timedelta; d=datetime.now(timezone.utc)-timedelta(days=1); print(d.strftime('%Y/%m/%d'))")
CODEX_DATE2=$(python3 -c "from datetime import datetime,timezone,timedelta; d=datetime.now(timezone.utc)-timedelta(days=2); print(d.strftime('%Y/%m/%d'))")
CODEX_DATE3=$(python3 -c "from datetime import datetime,timezone,timedelta; d=datetime.now(timezone.utc)-timedelta(days=3); print(d.strftime('%Y/%m/%d'))")
CODEX_DATE4=$(python3 -c "from datetime import datetime,timezone,timedelta; d=datetime.now(timezone.utc)-timedelta(days=5); print(d.strftime('%Y/%m/%d'))")

create_codex "$CODEX_DATE1" "a8f29c3b-e1d4-4a7f-9c82-1b3d5e7f9a2c" "o3" \
  "days=1" "days=1" \
  "Refactor error handling in API routes" 200

create_codex "$CODEX_DATE2" "b7e38d4c-f2e5-5b8a-ad93-2c4e6f8a0b3d" "o4-mini" \
  "days=2" "days=2" \
  "Add tab completion for CLI commands" 150

create_codex "$CODEX_DATE3" "c6d47e5d-a3f6-6c9b-bea4-3d5f7a9b1c4e" "o3" \
  "days=3" "days=3" \
  "Generate Terraform modules for VPC setup" 180

create_codex "$CODEX_DATE4" "d5c56f6e-b4a7-7dac-cfb5-4e6a8b0c2d5f" "o4-mini" \
  "days=5" "days=5" \
  "Create database migration scripts" 120

# -------------------------------------------------------------------
# Gemini Sessions (4)
# -------------------------------------------------------------------
echo "[5/7] Creating Gemini sessions (4)..."

create_gemini "proj-abc12345" "g7e8f9-etl-pipeline" \
  "gemini-2.5-pro" "Build ETL pipeline for analytics data" 24 "data-service" "hours=12"

create_gemini "proj-def67890" "g6d7e8-feature-extract" \
  "gemini-2.5-flash" "Implement feature extraction module" 16 "ml-pipeline" "days=1"

create_gemini "proj-abc12345" "g5c6d7-responsive-ui" \
  "gemini-2.5-pro" "Design responsive layout components" 20 "data-service" "days=2"

create_gemini "proj-ghi24680" "g4b5c6-push-notif" \
  "gemini-2.5-flash" "Setup push notification service" 12 "mobile-app" "days=4"

# -------------------------------------------------------------------
# Stats cache (Claude)
# -------------------------------------------------------------------
echo "[6/7] Creating stats cache and config..."

python3 << 'PYEOF' > "$DEMO_HOME/.claude/stats-cache.json"
import json, sys
from datetime import datetime, timedelta

# Use local time to match Go's time.Now().Format("2006-01-02")
now = datetime.now()
today = now.strftime("%Y-%m-%d")

def date_ago(days):
    return (now - timedelta(days=days)).strftime("%Y-%m-%d")

data = {
    "version": 1,
    "lastComputedDate": today,
    "dailyActivity": [
        {"date": today,        "messageCount": 847, "sessionCount": 6, "toolCallCount": 234},
        {"date": date_ago(1),  "messageCount": 623, "sessionCount": 5, "toolCallCount": 189},
        {"date": date_ago(2),  "messageCount": 512, "sessionCount": 4, "toolCallCount": 156},
        {"date": date_ago(3),  "messageCount": 389, "sessionCount": 3, "toolCallCount": 112},
        {"date": date_ago(4),  "messageCount": 445, "sessionCount": 4, "toolCallCount": 134},
        {"date": date_ago(5),  "messageCount": 298, "sessionCount": 2, "toolCallCount": 87},
        {"date": date_ago(6),  "messageCount": 367, "sessionCount": 3, "toolCallCount": 98},
        {"date": date_ago(8),  "messageCount": 234, "sessionCount": 2, "toolCallCount": 67},
        {"date": date_ago(10), "messageCount": 456, "sessionCount": 4, "toolCallCount": 145},
        {"date": date_ago(12), "messageCount": 312, "sessionCount": 3, "toolCallCount": 89},
        {"date": date_ago(15), "messageCount": 278, "sessionCount": 2, "toolCallCount": 76},
        {"date": date_ago(18), "messageCount": 534, "sessionCount": 5, "toolCallCount": 167},
        {"date": date_ago(20), "messageCount": 189, "sessionCount": 2, "toolCallCount": 54},
        {"date": date_ago(22), "messageCount": 423, "sessionCount": 3, "toolCallCount": 123},
        {"date": date_ago(25), "messageCount": 356, "sessionCount": 3, "toolCallCount": 98},
        {"date": date_ago(28), "messageCount": 267, "sessionCount": 2, "toolCallCount": 78},
        {"date": date_ago(35), "messageCount": 445, "sessionCount": 4, "toolCallCount": 134},
        {"date": date_ago(42), "messageCount": 312, "sessionCount": 3, "toolCallCount": 89},
        {"date": date_ago(50), "messageCount": 234, "sessionCount": 2, "toolCallCount": 67},
        {"date": date_ago(60), "messageCount": 189, "sessionCount": 2, "toolCallCount": 54}
    ],
    "dailyModelTokens": [
        {"date": today,        "tokensByModel": {"claude-opus-4-6": 89000, "claude-sonnet-4-6": 45000, "claude-haiku-4-5": 12000}},
        {"date": date_ago(1),  "tokensByModel": {"claude-opus-4-6": 72000, "claude-sonnet-4-6": 38000, "claude-opus-4-5-20251101": 25000}},
        {"date": date_ago(2),  "tokensByModel": {"claude-opus-4-6": 58000, "claude-sonnet-4-6": 32000, "claude-opus-4-5-20251101": 18000}},
        {"date": date_ago(3),  "tokensByModel": {"claude-haiku-4-5": 8000, "claude-sonnet-4-6": 28000, "claude-opus-4-5-20251101": 15000}},
        {"date": date_ago(4),  "tokensByModel": {"claude-opus-4-6": 65000, "claude-sonnet-4-6": 34000}},
        {"date": date_ago(5),  "tokensByModel": {"claude-opus-4-6": 42000, "claude-haiku-4-5": 6000}},
        {"date": date_ago(6),  "tokensByModel": {"claude-opus-4-6": 51000, "claude-sonnet-4-6": 24000}},
        {"date": date_ago(8),  "tokensByModel": {"claude-opus-4-6": 38000, "claude-opus-4-5-20251101": 12000}},
        {"date": date_ago(10), "tokensByModel": {"claude-opus-4-6": 67000, "claude-sonnet-4-6": 35000}},
        {"date": date_ago(12), "tokensByModel": {"claude-sonnet-4-6": 42000, "claude-haiku-4-5": 9000}},
        {"date": date_ago(15), "tokensByModel": {"claude-opus-4-6": 45000, "claude-sonnet-4-6": 22000}},
        {"date": date_ago(18), "tokensByModel": {"claude-opus-4-6": 78000, "claude-sonnet-4-6": 41000, "claude-opus-4-5-20251101": 20000}},
        {"date": date_ago(20), "tokensByModel": {"claude-opus-4-6": 32000}},
        {"date": date_ago(22), "tokensByModel": {"claude-opus-4-6": 56000, "claude-sonnet-4-6": 29000}},
        {"date": date_ago(25), "tokensByModel": {"claude-opus-4-6": 48000, "claude-haiku-4-5": 7000}},
        {"date": date_ago(28), "tokensByModel": {"claude-sonnet-4-6": 35000, "claude-opus-4-5-20251101": 14000}},
        {"date": date_ago(35), "tokensByModel": {"claude-opus-4-6": 62000, "claude-sonnet-4-6": 33000}},
        {"date": date_ago(42), "tokensByModel": {"claude-opus-4-6": 44000, "claude-opus-4-5-20251101": 16000}},
        {"date": date_ago(50), "tokensByModel": {"claude-opus-4-6": 36000, "claude-sonnet-4-6": 18000}},
        {"date": date_ago(60), "tokensByModel": {"claude-opus-4-6": 28000, "claude-haiku-4-5": 5000}}
    ],
    "modelUsage": {
        "claude-opus-4-6": {
            "inputTokens": 1850000,
            "outputTokens": 920000,
            "cacheReadInputTokens": 145000000,
            "cacheCreationInputTokens": 7800000,
            "webSearchRequests": 0,
            "costUSD": 0
        },
        "claude-sonnet-4-6": {
            "inputTokens": 680000,
            "outputTokens": 340000,
            "cacheReadInputTokens": 52000000,
            "cacheCreationInputTokens": 2800000,
            "webSearchRequests": 0,
            "costUSD": 0
        },
        "claude-opus-4-5-20251101": {
            "inputTokens": 450000,
            "outputTokens": 230000,
            "cacheReadInputTokens": 38000000,
            "cacheCreationInputTokens": 1900000,
            "webSearchRequests": 0,
            "costUSD": 0
        },
        "claude-haiku-4-5": {
            "inputTokens": 120000,
            "outputTokens": 58000,
            "cacheReadInputTokens": 8500000,
            "cacheCreationInputTokens": 420000,
            "webSearchRequests": 0,
            "costUSD": 0
        }
    },
    "totalSessions": 64,
    "totalMessages": 8390,
    "longestSession": {
        "sessionId": "a7b8c9d0-e1f2-3456-abcd-789012345678",
        "duration": 7200,
        "messageCount": 523,
        "timestamp": date_ago(1)
    },
    "firstSessionDate": date_ago(60),
    "hourCounts": {str(h): (80 + h * 15 if 9 <= h <= 22 else 10 + h * 2) for h in range(24)}
}

json.dump(data, sys.stdout, indent=2)
PYEOF

# -------------------------------------------------------------------
# Config + Aliases
# -------------------------------------------------------------------
cat > "$CONFIG_DIR/config.toml" << 'EOF'
[general]
default_limit = 30
cache_ttl = 10

[providers]
enabled = ["claude", "codex", "gemini"]

[providers.claude]
data_dir = "~/.claude"

[providers.codex]
data_dir = "~/.codex"

[providers.gemini]
data_dir = "~/.gemini"

[tmux]
status_interval = 10
status_position = "right"
EOF

cat > "$CONFIG_DIR/aliases.json" << 'EOF'
{
  "a1b2c3d4-e5f6-7890-abcd-ef1234567890": "auth-refactor",
  "b2c3d4e5-f6a7-8901-bcde-f12345678901": "fix-timeout",
  "c3d4e5f6-a7b8-9012-cdef-123456789012": "redis-cache",
  "e5f6a7b8-c9d0-1234-efab-567890123456": "k8s-deploy",
  "a7b8c9d0-e1f2-3456-abcd-789012345678": "perf-optimize",
  "b8c9d0e1-f2a3-4567-bcde-890123456789": "db-migration",
  "d0e1f2a3-b4c5-6789-defa-012345678901": "readme-update",
  "a8f29c3b-e1d4-4a7f-9c82-1b3d5e7f9a2c": "codex-api"
}
EOF

# -------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------
echo "[7/7] Verifying..."
TOTAL_CLAUDE=$(find "$CLAUDE_DIR/projects" -name "*.jsonl" | wc -l | tr -d ' ')
TOTAL_CODEX=$(find "$CODEX_DIR/sessions" -name "*.jsonl" | wc -l | tr -d ' ')
TOTAL_GEMINI=$(find "$GEMINI_DIR/tmp" -name "*.json" -not -name "metadata.json" | wc -l | tr -d ' ')
TOTAL=$((TOTAL_CLAUDE + TOTAL_CODEX + TOTAL_GEMINI))

echo ""
echo "=== Demo setup complete ==="
echo "  Claude:  $TOTAL_CLAUDE sessions"
echo "  Codex:   $TOTAL_CODEX sessions"
echo "  Gemini:  $TOTAL_GEMINI sessions"
echo "  Total:   $TOTAL sessions"
echo "  Aliases: 8"
echo "  Home:    $DEMO_HOME"
echo ""
echo "To test:  HOME=$DEMO_HOME $DEMO_HOME/.local/bin/taux get sessions"
echo "To record: vhs docs/demo.tape"
echo "To clean:  bash docs/demo-teardown.sh"
