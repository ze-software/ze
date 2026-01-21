#!/bin/bash
# UserPromptSubmit hook: Detect context compaction and remind to re-read specs
# NON-BLOCKING: Just outputs a reminder

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Read the user message from stdin
MESSAGE=$(cat)

# Check for compaction indicators (both patterns for reliability)
if echo "$MESSAGE" | grep -q "continued from a previous conversation" && \
   echo "$MESSAGE" | grep -q "ran out of context\|context compaction"; then

    # Find active specs
    ACTIVE_SPECS=$(find docs/plan -maxdepth 1 -name "spec-*.md" 2>/dev/null)

    echo "🔄 CONTEXT COMPACTION DETECTED" >&2
    echo "" >&2
    echo "⚠️  Spec details were lost. You MUST re-read before continuing:" >&2
    echo "" >&2

    if [ -n "$ACTIVE_SPECS" ]; then
        for spec in $ACTIVE_SPECS; do
            echo "   📋 READ: $spec" >&2
        done
    else
        echo "   (No active specs found)" >&2
    fi

    echo "" >&2
    echo "   📚 READ: .claude/rules/planning.md" >&2
    echo "" >&2
fi

exit 0
