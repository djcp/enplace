#!/usr/bin/env bash
# Rebuild the enplace binary and regenerate all screenshots.
#
# Run from anywhere:
#   ./screenshots/regenerate.sh
#
# Output: screenshots/{dark,light}-{list,detail}.svg

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── dependency checks ─────────────────────────────────────────────────────────
missing=0
check() {
    local cmd=$1 hint=$2
    command -v "$cmd" >/dev/null 2>&1 && return
    echo "Missing: $cmd — $hint"
    missing=1
}

check go        "install from https://go.dev/dl/"
check tmux      "apt install tmux  /  brew install tmux"
check asciinema "apt install asciinema  /  pip install asciinema  /  brew install asciinema"
check termtosvg "pip install termtosvg  /  pipx install termtosvg"

(( missing == 0 )) || exit 1

cd "$PROJECT_DIR"

echo "=== building enplace ==="
go build -o enplace .

echo ""
echo "=== generating screenshots ==="
bash "$PROJECT_DIR/screenshots/capture.sh"
