#!/bin/bash
# SessionStart hook - checks git status, test state, and active specs

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Check git status
MODIFIED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

if [ "$MODIFIED" -gt 0 ]; then
    echo "⚠️  $MODIFIED uncommitted changes:"
    git status -s
else
    echo "✅ Repo clean"
fi

# Check for active specs in docs/plan/
# Active specs are spec-*.md files (not in done/ subdirectory)
ACTIVE_SPECS=$(find docs/plan -maxdepth 1 -name "spec-*.md" 2>/dev/null)

if [ -n "$ACTIVE_SPECS" ]; then
    echo ""
    echo "📋 ACTIVE SPECS - Re-read before continuing work:"
    for spec in $ACTIVE_SPECS; do
        echo "   → $spec"
    done
    echo ""
    echo "⚠️  After compaction: spec details are lost. RE-READ the spec file!"
    echo "   Also re-read: .claude/rules/planning.md"
fi
