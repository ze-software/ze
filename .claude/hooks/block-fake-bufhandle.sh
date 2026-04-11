#!/bin/bash
# PreToolUse hook: block fake BufHandle construction outside testPoolBuf.
#
# BLOCKING (exit 2): `BufHandle{Buf: make(...)}` outside the approved helper
# is a latent memory-corruption bug. The handle's zero-value ID=0/idx=0
# collides with the first real slot of bufMuxStd.block[0], so when the cache
# later evicts the entry and calls ReturnReadBuffer, the real bufmux either
# logs "double return detected" or silently marks a real-owned slot as free.
# Tests must use testPoolBuf(t) which tags the handle with the noPoolBufID
# sentinel so ReturnReadBuffer skips it.
#
# See .claude/known-failures.md "Observer-exit antipattern" section and
# commit 3b21dadb for the full history.
#
# Exceptions: testPoolBuf's own definition (which carries noPoolBufID), and
# comments/docs referencing the old pattern.

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only Go files
if [[ "$FILE_PATH" != *.go ]]; then
    exit 0
fi

case "$TOOL_NAME" in
    Write)
        CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')
        ;;
    Edit|MultiEdit)
        CONTENT=$(echo "$INPUT" | jq -r '.tool_input.new_string // empty')
        ;;
    *)
        exit 0
        ;;
esac

# Look for the bad pattern. grep -F for literal match, then strip comment
# lines and the noPoolBufID-tagged form that testPoolBuf uses.
BAD_LINES=$(echo "$CONTENT" | grep -nE 'BufHandle\{[^}]*Buf:[[:space:]]*make' || true)
if [[ -z "$BAD_LINES" ]]; then
    exit 0
fi

# Filter out lines that:
#   1. Start with // (comment lines)
#   2. Also contain noPoolBufID (the approved form inside testPoolBuf)
BAD_FILTERED=$(echo "$BAD_LINES" | grep -v '^[[:space:]]*[0-9]*:[[:space:]]*//' | grep -v 'noPoolBufID' || true)
if [[ -z "$BAD_FILTERED" ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${RED}${BOLD}BLOCKED: fake BufHandle construction in $FILE_PATH${RESET}" >&2
echo "" >&2
echo -e "  ${RED}Found:${RESET} BufHandle{Buf: make(...)} without noPoolBufID sentinel" >&2
echo "$BAD_FILTERED" | while IFS= read -r line; do
    echo -e "    ${RED}>${RESET} $line" >&2
done
echo "" >&2
echo -e "  A fake handle's zero-value ID=0/idx=0 collides with the first real" >&2
echo -e "  slot of bufMuxStd.block[0]. When the cache later evicts the entry" >&2
echo -e "  and calls ReturnReadBuffer, the real bufmux either logs \"double" >&2
echo -e "  return detected\" or silently marks a real-owned slot as free" >&2
echo -e "  (latent memory corruption)." >&2
echo "" >&2
echo -e "  ${YELLOW}Fix:${RESET} use the testPoolBuf(t) helper in reactor_test.go, which" >&2
echo -e "  returns BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)}." >&2
echo -e "  ReturnReadBuffer short-circuits handles carrying the sentinel." >&2
echo "" >&2
echo -e "  ${YELLOW}Reference:${RESET} commit 3b21dadb, .claude/known-failures.md" >&2

exit 2
