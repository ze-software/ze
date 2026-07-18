#!/bin/bash
# Stop hook: Write a compact session snapshot to per-spec session state file.
# Keeps the three most recent summaries. Cleans up session marker.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

STATE_FILE=$(_state_file)
TIMESTAMP=$(date -Iseconds)

# Gather current state
SID=$(_session_id)
MARKER=".claude/.session-${SID}"
SELECTED_SPEC=""
if [ -f "$MARKER" ]; then
    SELECTED_SPEC=$(head -1 "$MARKER" 2>/dev/null)
    [ "$SELECTED_SPEC" = "unassigned" ] && SELECTED_SPEC=""
fi

MODIFIED=$(git diff --name-only 2>/dev/null | head -20)
STAGED=$(git diff --cached --name-only 2>/dev/null | head -20)
RECENT_COMMIT=$(git log -1 --oneline 2>/dev/null)
BRANCH=$(git branch --show-current 2>/dev/null)

# Skip the snapshot on a clean tree with no spec -- but do NOT remove the state
# file. A Stop hook fires between EVERY turn, and a clean-tree pause mid-session
# (e.g. right after a commit) is not session end: deleting the state file here
# deadlocked the next edit against the pretool session-state gate, which requires
# the file after a compaction. The 24h orphan sweep (lib/state-file.sh
# _cleanup_stale_markers) reclaims it once its marker is gone; true end-of-session
# cleanup belongs in a SessionEnd hook, not here.
HAS_CHANGES=$(git status --porcelain 2>/dev/null | head -1)
if [ -z "$HAS_CHANGES" ] && [ -z "$SELECTED_SPEC" ]; then
    _release_session
    exit 0
fi

# Build new snapshot
NEW_SNAPSHOT=$(cat <<SNAP
## Session: $TIMESTAMP

Branch: \`$BRANCH\`
$([ -n "$RECENT_COMMIT" ] && echo "Last commit: $RECENT_COMMIT")
$([ -n "$SELECTED_SPEC" ] && echo "Spec: \`$SELECTED_SPEC\`")
$(if [ -n "$MODIFIED" ]; then
    echo ""
    echo "Uncommitted:"
    echo "$MODIFIED" | while read -r f; do echo "- \`$f\`"; done
fi)
$(if [ -n "$STAGED" ]; then
    echo ""
    echo "Staged:"
    echo "$STAGED" | while read -r f; do echo "- \`$f\`"; done
fi)
SNAP
)

# Extract the two most recent snapshots from existing file
PREVIOUS=""
if [ -f "$STATE_FILE" ]; then
    PREVIOUS=$(awk '
        /^## Session:/ { block++; if (block > 2) exit }
        block >= 1 { print }
    ' "$STATE_FILE")
fi

# Write: header + new snapshot + up to 2 previous snapshots
{
    echo "# Session State"
    echo ""
    echo "$NEW_SNAPSHOT"
    if [ -n "$PREVIOUS" ]; then
        echo ""
        echo "---"
        echo "$PREVIOUS"
    fi
} > "$STATE_FILE"

# Clean up session marker and per-session compaction marker
rm -f ".claude/.compaction-detected-${SID}"
_release_session

exit 0
