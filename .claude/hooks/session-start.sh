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

# Check for selected spec
SELECTED_SPEC=""
if [ -f ".claude/selected-spec" ]; then
    # Read non-comment, non-empty line
    SELECTED_SPEC=$(grep -v '^#' .claude/selected-spec | grep -v '^$' | head -1)
fi

# Check for active specs in docs/plan/
ACTIVE_SPECS=$(find docs/plan -maxdepth 1 -name "spec-*.md" 2>/dev/null | sort)
SPEC_COUNT=$(echo "$ACTIVE_SPECS" | grep -c .)

if [ -n "$SELECTED_SPEC" ] && [ -f "docs/plan/$SELECTED_SPEC" ]; then
    echo ""
    echo "🎯 SELECTED SPEC - RE-READ before continuing:"
    echo "   → docs/plan/$SELECTED_SPEC"
    echo ""
    echo "⚠️  After compaction: RE-READ the spec and its Required Reading docs!"
    echo ""
    if [ "$SPEC_COUNT" -gt 1 ]; then
        echo "📋 Other specs ($((SPEC_COUNT - 1))):"
        for spec in $ACTIVE_SPECS; do
            BASENAME=$(basename "$spec")
            if [ "$BASENAME" != "$SELECTED_SPEC" ]; then
                echo "   → $spec"
            fi
        done
    fi
elif [ -n "$ACTIVE_SPECS" ]; then
    echo ""
    echo "📋 ACTIVE SPECS ($SPEC_COUNT) - No spec selected:"
    for spec in $ACTIVE_SPECS; do
        echo "   → $spec"
    done
    echo ""
    echo "💡 To select: write spec filename to .claude/selected-spec"
    echo "   Example: echo 'spec-rfc9234-role.md' >> .claude/selected-spec"
fi
