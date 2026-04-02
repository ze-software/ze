#!/bin/bash
# BLOCKING HOOK: Prevents destructive git commands
# This hook is called by Claude Code before any Bash command
# Exit code 2 = BLOCK the command

COMMAND="$CLAUDE_TOOL_INPUT_command"

# List of destructive git command patterns
DESTRUCTIVE_PATTERNS=(
    "git commit"
    "git push"
    "git reset"
    "git checkout --"
    "git checkout -f"
    "git checkout HEAD"
    "git restore"
    "git revert"
    "git stash drop"
    "git stash clear"
    "git clean"
    "git push --force"
    "git push -f"
)

# Allow git restore --staged (unstaging is safe)
if [[ "$COMMAND" == *"git restore --staged"* ]]; then
    exit 0
fi

for pattern in "${DESTRUCTIVE_PATTERNS[@]}"; do
    if [[ "$COMMAND" == *"$pattern"* ]]; then
        echo "❌ Blocked: $pattern (run manually)" >&2
        exit 2
    fi
done

exit 0
