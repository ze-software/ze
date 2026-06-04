#!/bin/bash
# BLOCKING HOOK: Prevents piping test output instead of capturing to file
# The testing.md rule says: Never | tail
# Use: make ze-verify (auto-captures to tmp/ze-verify.log)
# Then: grep failures from the log file
# Exit code 2 = BLOCK the command

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

if [[ "$COMMAND" == *"| tail"* ]]; then
    echo "❌ Blocked: '| tail' -- capture to file instead, or use Read tool" >&2
    exit 2
fi

# Block piping make ze-* commands through lossy filters (grep/head/tail/awk/sed).
# Allow: | tee <file> (non-lossy, writes to file and passes all output through).
if [[ "$COMMAND" =~ make\ ze-.*\| ]]; then
    AFTER_PIPE="${COMMAND##*|}"
    if [[ "$AFTER_PIPE" =~ ^[[:space:]]*tee[[:space:]] ]]; then
        exit 0
    fi
    echo "❌ Blocked: piping make ze-* output through a lossy filter" >&2
    echo "  -- Use: make ze-verify ZE_VERIFY_LOG=tmp/ze-verify-\$\$.log" >&2
    echo "  -- Or:  make ze-verify 2>&1 | tee tmp/ze-verify-\$\$.log" >&2
    echo "  -- Then: Read the log with offset/limit" >&2
    exit 2
fi

exit 0
