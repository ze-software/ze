#!/bin/bash
# SessionStart hook - automates session initialization
set -e

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Check git status
MODIFIED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
if [ "$MODIFIED" -gt 0 ]; then
    echo "modified_files: $MODIFIED"
    git status -s | head -10
    exit 2  # Ask user how to proceed
fi

# Report test status from continuation file
if [ -f "plan/CLAUDE_CONTINUATION.md" ]; then
    STATUS=$(grep -A3 "^## CURRENT STATUS" plan/CLAUDE_CONTINUATION.md 2>/dev/null | tail -3 || true)
    if [ -n "$STATUS" ]; then
        echo "test_status:"
        echo "$STATUS"
    fi
fi

echo "session: ready"
