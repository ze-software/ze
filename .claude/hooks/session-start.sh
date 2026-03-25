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

# Selected specs - multi-session aware (one per line)
SELECTED_SPECS=$(grep -v '^#' .claude/selected-spec 2>/dev/null | grep -v '^$')
SELECTED_COUNT=$(echo "$SELECTED_SPECS" | grep -c . 2>/dev/null || true)
SPEC_COUNT=$(find plan -maxdepth 1 -name "spec-*.md" 2>/dev/null | wc -l | tr -d ' ')

if [ "$SELECTED_COUNT" -gt 1 ]; then
    echo "⚠️ ${SELECTED_COUNT} Claude sessions active on specs:"
    echo "$SELECTED_SPECS" | while read -r spec; do
        [ -f "plan/$spec" ] && echo "   - $spec" || echo "   - $spec (missing)"
    done
    echo "   → READ your spec BEFORE any work"
elif [ "$SELECTED_COUNT" -eq 1 ] && [ -f "plan/$SELECTED_SPECS" ]; then
    echo "🎯 SPEC: $SELECTED_SPECS (+$((SPEC_COUNT-1)) others)"
    echo "   → READ plan/$SELECTED_SPECS BEFORE any work"
elif [ "$SPEC_COUNT" -gt 0 ]; then
    echo "📋 ${SPEC_COUNT} specs, none selected"
fi

# Spec status summary (compact counts by status)
if [ "$SPEC_COUNT" -gt 0 ]; then
    COUNTS=""
    for status in in-progress ready design skeleton blocked deferred; do
        N=$(grep -l "| Status | *${status}" plan/spec-*.md 2>/dev/null | wc -l | tr -d ' ')
        [ "$N" -gt 0 ] && COUNTS="${COUNTS:+$COUNTS, }${N} ${status}"
    done
    [ -n "$COUNTS" ] && echo "   ($COUNTS)"
fi

# Session state reminder with phase display
if [ -f ".claude/session-state.md" ]; then
    LAST_UPDATE=$(head -5 .claude/session-state.md | grep -o '20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]' | head -1)
    PHASE=$(grep '^Phase:' .claude/session-state.md 2>/dev/null | head -1 | sed 's/^Phase:\s*//')
    if [ -n "$PHASE" ]; then
        echo "📝 Session state (phase: $PHASE)"
    elif [ -n "$LAST_UPDATE" ]; then
        echo "📝 Session state exists (updated: $LAST_UPDATE)"
    fi
fi

# Blocking reminders
echo "⚠️ BLOCKING: ToolSearch select:LSP — load BEFORE any work"
echo "⚠️ RULE: Read spec + source files BEFORE writing any code"
