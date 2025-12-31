#!/bin/bash
# SessionStart hook - checks git status and test state

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Check git status
MODIFIED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

if [ "$MODIFIED" -gt 0 ]; then
    echo "⚠️  $MODIFIED uncommitted changes:"
    git status -s
else
    echo "✅ Repo clean"
fi
