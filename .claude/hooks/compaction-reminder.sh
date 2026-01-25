#!/bin/bash
# UserPromptSubmit hook: Detect context compaction and remind to re-read specs
# NON-BLOCKING: Just outputs a reminder, but creates marker file

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Read the user message from stdin
MESSAGE=$(cat)

# Check for compaction indicators (both patterns for reliability)
if echo "$MESSAGE" | grep -q "continued from a previous conversation" && \
   echo "$MESSAGE" | grep -q "ran out of context\|context compaction"; then

    # Create compaction marker - signals that recovery is needed
    echo "$(date -Iseconds)" > .claude/.compaction-detected

    # Find active specs
    SELECTED_SPEC=$(cat .claude/selected-spec 2>/dev/null | tr -d '[:space:]')

    echo "🔄 CONTEXT COMPACTION DETECTED" >&2
    echo "" >&2
    echo "⛔ BLOCKING: Complete recovery before ANY action:" >&2
    echo "" >&2
    echo "   1. Read selected spec:" >&2
    if [ -n "$SELECTED_SPEC" ]; then
        echo "      → docs/plan/$SELECTED_SPEC" >&2
    else
        echo "      → (none selected)" >&2
    fi
    echo "" >&2
    echo "   2. Read Post-Compaction Recovery section in spec" >&2
    echo "" >&2
    echo "   3. Read these rules:" >&2
    echo "      → .claude/rules/post-compaction.md" >&2
    echo "      → .claude/rules/planning.md" >&2
    echo "      → .claude/rules/spec-no-code.md" >&2
    echo "" >&2
    echo "   4. Check session state:" >&2
    echo "      → .claude/session-state.md (if exists)" >&2
    echo "" >&2
    echo "   5. Check git status for current state" >&2
    echo "" >&2
    echo "   6. Update session-state.md after reading" >&2
    echo "" >&2
    echo "⚠️  DO NOT write code until recovery complete!" >&2
    echo "" >&2
fi

exit 0
