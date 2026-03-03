#!/bin/bash
# docs/demo-teardown.sh — Remove temporary demo data.
# Usage: bash docs/demo-teardown.sh

set -euo pipefail

DEMO_HOME="/tmp/taux-demo"

if [ -d "$DEMO_HOME" ]; then
  rm -rf "$DEMO_HOME"
  echo "Demo data removed: $DEMO_HOME"
else
  echo "Nothing to clean: $DEMO_HOME does not exist."
fi
