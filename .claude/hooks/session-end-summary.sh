#!/bin/bash
# Stop hook: Append a timestamped summary to session-state.md
# Advisory: Captures git state at end of interaction for next session

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

STATE_FILE=".claude/session-state.md"
TIMESTAMP=$(date -Iseconds)

# Gather current state
SELECTED_SPEC=$(grep -v '^#' .claude/selected-spec 2>/dev/null | grep -v '^$' | head -1)
MODIFIED=$(git diff --name-only 2>/dev/null | head -20)
STAGED=$(git diff --cached --name-only 2>/dev/null | head -20)
RECENT_COMMIT=$(git log -1 --oneline 2>/dev/null)
BRANCH=$(git branch --show-current 2>/dev/null)

# Only append if there are uncommitted changes or the file already exists
# (avoid creating noise on trivial interactions)
HAS_CHANGES=$(git status --porcelain 2>/dev/null | head -1)

if [ -z "$HAS_CHANGES" ] && [ ! -f "$STATE_FILE" ]; then
    exit 0
fi

# Append session-end marker
{
    echo ""
    echo "---"
    echo "## Session End: $TIMESTAMP"
    echo ""
    echo "Branch: \`$BRANCH\`"
    [ -n "$RECENT_COMMIT" ] && echo "Last commit: $RECENT_COMMIT"
    [ -n "$SELECTED_SPEC" ] && echo "Spec: \`$SELECTED_SPEC\`"
    if [ -n "$MODIFIED" ]; then
        echo ""
        echo "Uncommitted changes:"
        echo "$MODIFIED" | while read -r f; do echo "- \`$f\`"; done
    fi
    if [ -n "$STAGED" ]; then
        echo ""
        echo "Staged:"
        echo "$STAGED" | while read -r f; do echo "- \`$f\`"; done
    fi
} >> "$STATE_FILE"

exit 0
