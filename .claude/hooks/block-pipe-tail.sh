#!/bin/bash
# BLOCKING HOOK: Prevents piping test output instead of capturing to file
# The testing.md rule says: Never | tail
# The git-safety.md rule says: capture to tmp/ze-test.log
# Use: make ze-verify > tmp/ze-test.log 2>&1
# Then: grep failures from the log file
# Exit code 2 = BLOCK the command

COMMAND="$CLAUDE_TOOL_INPUT_command"

if [[ "$COMMAND" == *"| tail"* ]]; then
    echo "❌ Blocked: '| tail' — capture to file instead, or use Read tool" >&2
    exit 2
fi

# Block piping make ze-* commands through grep/head/tail instead of capturing to file
if [[ "$COMMAND" =~ make\ ze-.*\| ]]; then
    echo "❌ Blocked: piping make ze-* output" >&2
    echo "  → Use: make ze-verify > tmp/ze-test.log 2>&1" >&2
    echo "  → Then: grep -E 'FAIL|TEST FAILURE' tmp/ze-test.log" >&2
    exit 2
fi

exit 0
