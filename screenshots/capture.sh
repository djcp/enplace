#!/usr/bin/env bash
# capture.sh — generate dark and light SVG screenshots of enplace
#
# Pipeline:
#   1. Start enplace in a detached tmux session
#   2. Navigate to the target page via tmux send-keys
#   3. Attach and record with asciinema (captures the stable view)
#   4. Render with termtosvg using the solarized_dark / solarized_light template
#   5. Pick the second-to-last still frame (last = blank screen after session ends)
#
# Requires: tmux, asciinema, termtosvg
#
# Usage:  ./screenshots/capture.sh
# Output: screenshots/{dark,light}-{list,detail}.svg
#         screenshots/{dark,light}-{list,detail}.cast  (kept for re-rendering)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_DIR/enplace"

declare -A HINTS=(
    [tmux]="apt install tmux  /  brew install tmux"
    [asciinema]="apt install asciinema  /  pip install asciinema  /  brew install asciinema"
    [termtosvg]="pip install termtosvg  /  pipx install termtosvg"
)
missing=0
for cmd in tmux asciinema termtosvg; do
    command -v "$cmd" >/dev/null 2>&1 && continue
    echo "Missing: $cmd — ${HINTS[$cmd]}"
    missing=1
done
(( missing == 0 )) || exit 1
[[ -x "$BINARY" ]] || { echo "Run ./screenshots/regenerate.sh to build first, or: go build -o enplace ."; exit 1; }

capture() {
    local theme=$1    # dark | light
    local page=$2     # list | detail
    local template=$3
    local colorfgbg=$4
    local cast="$SCRIPT_DIR/$theme-$page.cast"
    local outfile="$SCRIPT_DIR/$theme-$page.svg"
    local session="enplace-$theme-$page"

    echo "→ recording $theme $page…"

    # Clean up any leftover session from a previous run
    tmux kill-session -t "$session" 2>/dev/null || true

    # Start enplace in a detached tmux session.
    # COLORFGBG controls dark/light detection in lipgloss AdaptiveColor.
    # Status bar is disabled immediately (before enplace renders) so it never
    # appears in the recording and the pane is a clean 160x45.
    tmux new-session -d -s "$session" -x 160 -y 45 \
        -e "COLORFGBG=$colorfgbg" \
        -e "PATH=$PROJECT_DIR:$PATH" \
        "$BINARY" \; set-option -t "$session" status off

    # Give enplace time to fully render the list view
    sleep 4

    if [[ "$page" == "detail" ]]; then
        # Navigate into the first recipe and wait for the detail view to render
        tmux send-keys -t "$session" Enter
        sleep 4
    fi

    # Record the stable view: attach, hold for 3s, then kill the session.
    # Killing the session (not sending q) ensures the last frame the recording
    # captures is the live TUI, not enplace's terminal-restore blank screen.
    (sleep 3; tmux kill-session -t "$session" 2>/dev/null) &
    local killer_pid=$!

    asciinema rec --overwrite --cols 160 --rows 45 \
        -c "tmux attach-session -t $session" \
        "$cast" 2>/dev/null

    wait "$killer_pid" 2>/dev/null || true
    tmux kill-session -t "$session" 2>/dev/null || true

    # Render the cast to still-frame SVGs using the chosen template.
    # The last frame is blank (terminal state after the session ends).
    # The second-to-last is the stable enplace page we want.
    local tmpdir
    tmpdir=$(mktemp -d)

    termtosvg render "$cast" -s -t "$template" "$tmpdir" 2>/dev/null

    local -a frames
    mapfile -t frames < <(ls -1v "$tmpdir"/*.svg 2>/dev/null)

    if [[ ${#frames[@]} -eq 0 ]]; then
        echo "  ERROR: no frames produced for $theme $page" >&2
        rm -rf "$tmpdir"
        return 1
    fi

    local target
    if [[ ${#frames[@]} -ge 2 ]]; then
        target="${frames[-2]}"   # second-to-last: stable UI before session ends
    else
        target="${frames[-1]}"   # only one frame — take it
    fi

    cp "$target" "$outfile"
    rm -rf "$tmpdir"

    local size
    size=$(wc -c < "$outfile")
    echo "  ✓ $outfile (${size} bytes, from ${#frames[@]} frames)"
}

capture dark  list   solarized_dark  "15;0"
capture dark  detail solarized_dark  "15;0"
capture light list   solarized_light "0;15"
capture light detail solarized_light "0;15"

echo ""
echo "Done. Screenshots in $SCRIPT_DIR/:"
ls -lh "$SCRIPT_DIR"/*.svg
