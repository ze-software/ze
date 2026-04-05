#!/bin/bash
# PreCompact hook: Auto-save session state before context compaction
# Advisory: Saves a snapshot so Claude can recover after compaction

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

STATE_FILE=$(_state_file)
TIMESTAMP=$(date -Iseconds)

# Read current spec from session marker
SID=$(_session_id)
MARKER=".claude/.session-${SID}"
SELECTED_SPEC=""
if [ -f "$MARKER" ]; then
    SELECTED_SPEC=$(head -1 "$MARKER" 2>/dev/null)
    [ "$SELECTED_SPEC" = "unassigned" ] && SELECTED_SPEC=""
fi

# Read current git state (compact)
MODIFIED_FILES=$(git diff --name-only 2>/dev/null | head -20)
STAGED_FILES=$(git diff --cached --name-only 2>/dev/null | head -20)

# If state file already exists and has content, preserve it.
# Just prepend the compaction timestamp so Claude knows when it happened.
if [ -f "$STATE_FILE" ] && [ -s "$STATE_FILE" ]; then
    # Check if it already has a compaction marker at the top
    if grep -q "^## Last Compaction" "$STATE_FILE"; then
        # Update the existing marker
        sed -i '' "s/^## Last Compaction.*/## Last Compaction: $TIMESTAMP/" "$STATE_FILE"
    else
        # Insert compaction marker after the first heading
        {
            head -1 "$STATE_FILE"
            echo ""
            echo "## Last Compaction: $TIMESTAMP"
            tail -n +2 "$STATE_FILE"
        } > "${STATE_FILE}.tmp" && mv "${STATE_FILE}.tmp" "$STATE_FILE"
    fi
else
    # Create minimal session state from available information
    {
        echo "# Session State"
        echo ""
        echo "## Last Compaction: $TIMESTAMP"
        echo ""
        if [ -n "$SELECTED_SPEC" ]; then
            echo "## Active Spec"
            echo "$SELECTED_SPEC"
            echo ""
        fi
        if [ -n "$MODIFIED_FILES" ]; then
            echo "## Modified Files"
            echo "$MODIFIED_FILES" | while read -r f; do echo "- \`$f\`"; done
            echo ""
        fi
        if [ -n "$STAGED_FILES" ]; then
            echo "## Staged Files"
            echo "$STAGED_FILES" | while read -r f; do echo "- \`$f\`"; done
            echo ""
        fi
    } > "$STATE_FILE"
fi

# Mark compaction detected (per-session to avoid cross-session contamination)
echo "$TIMESTAMP" > ".claude/.compaction-detected-${SID}"

# Output to stderr (not tokens) -- Claude sees this in hook output
echo "Session state saved before compaction ($STATE_FILE)" >&2

exit 0
