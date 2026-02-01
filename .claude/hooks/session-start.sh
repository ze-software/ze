#!/bin/bash
# SessionStart hook - compact status summary with rule reminders

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Git status summary (compact)
STATUS=$(git status --porcelain 2>/dev/null)
if [ -n "$STATUS" ]; then
    TOTAL=$(echo "$STATUS" | wc -l | tr -d ' ')
    MODIFIED=$(echo "$STATUS" | grep -c '^ M' 2>/dev/null || true)
    ADDED=$(echo "$STATUS" | grep -c '^??' 2>/dev/null || true)
    : "${MODIFIED:=0}" "${ADDED:=0}"
    echo "⚠️ ${TOTAL} uncommitted: ${MODIFIED}M ${ADDED}A"
else
    echo "✅ Clean"
fi

# Selected spec - with READ reminder
SELECTED_SPEC=$(grep -v '^#' .claude/selected-spec 2>/dev/null | grep -v '^$' | head -1)
SPEC_COUNT=$(find docs/plan -maxdepth 1 -name "spec-*.md" 2>/dev/null | wc -l | tr -d ' ')

if [ -n "$SELECTED_SPEC" ] && [ -f "docs/plan/$SELECTED_SPEC" ]; then
    echo "🎯 SPEC: $SELECTED_SPEC (+$((SPEC_COUNT-1)) others)"
    echo "   → READ docs/plan/$SELECTED_SPEC BEFORE any work"
elif [ "$SPEC_COUNT" -gt 0 ]; then
    echo "📋 ${SPEC_COUNT} specs, none selected"
fi

# Session state reminder
if [ -f ".claude/session-state.md" ]; then
    LAST_UPDATE=$(head -5 .claude/session-state.md | grep -o '20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]' | head -1)
    [ -n "$LAST_UPDATE" ] && echo "📝 Session state exists (updated: $LAST_UPDATE)"
fi

# Top rule reminder (one line)
echo "⚠️ RULE: Read spec + source files BEFORE writing any code"
