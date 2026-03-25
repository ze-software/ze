#!/bin/bash
# UserPromptSubmit hook: Detect context compaction (token-optimized)

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."
MESSAGE=$(cat)

if echo "$MESSAGE" | grep -q "continued from a previous conversation" && \
   echo "$MESSAGE" | grep -q "ran out of context\|context compaction"; then
    echo "$(date -Iseconds)" > .claude/.compaction-detected
    SELECTED_SPEC=$(grep -v '^#' .claude/selected-spec 2>/dev/null | grep -v '^$' | tail -1 | tr -d '[:space:]')

    # Compact output to stderr (no tokens)
    echo "🔄 COMPACTION: Read .claude/rules/post-compaction.md" >&2
    [ -n "$SELECTED_SPEC" ] && echo "   spec: $SELECTED_SPEC" >&2
fi

exit 0
