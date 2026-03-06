#!/bin/bash
# PreToolUse hook: Block per-call allocations in encoding code
# BLOCKING: All encoding must use pooled buffers, not make()/append()/Bytes()/Pack() (buffer-first.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit for Go files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Only check hot-path encoding — UPDATE building and message packing.
# Plugin encode.go and helpers.go are NOT hot path
# (CLI/registry/API routes called at human speed — pools are wrong there).
IS_ENCODE=0
if [[ "$FILE_PATH" =~ update_build ]]; then
    IS_ENCODE=1
elif [[ "$FILE_PATH" =~ /message/pack ]]; then
    IS_ENCODE=1
elif [[ "$FILE_PATH" =~ reactor_wire ]]; then
    IS_ENCODE=1
fi

if [[ "$IS_ENCODE" -eq 0 ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# 1. Check for append() in encoding code
# Allow: append to args/strings slices (CLI parsing), but not byte slices
APPEND_MATCHES=$(echo "$CONTENT" | grep -nE 'append\s*\(' | grep -viE '(args|strings|labels|families|errors|ERRORS|names|fields|parts)' | grep -viE '//.*append' | head -3 || true)
if [[ -n "$APPEND_MATCHES" ]]; then
    ERRORS+=("append() in encoding code — use pre-computed size + offset writes:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$APPEND_MATCHES"
fi

# 2. Check for make([]byte in non-pool context
# Allow: pool New func (return make), result copies that are followed by copy()
MAKE_MATCHES=$(echo "$CONTENT" | grep -nE 'make\s*\(\s*\[\s*\]\s*byte' | grep -viE '(return make|Pool|New.*func|nlriBytes\s*:=\s*make|owned\s*:=\s*make|result\s*:=\s*make)' | head -3 || true)
if [[ -n "$MAKE_MATCHES" ]]; then
    ERRORS+=("make([]byte) in encoding code — use pool buffer instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$MAKE_MATCHES"
fi

# 3. Check for .Bytes() calls (should use .WriteTo)
# Only exclude: rd.Bytes() which returns a fixed [8]byte (no alloc), and json/spec access patterns
BYTES_MATCHES=$(echo "$CONTENT" | grep -nE '\.\s*Bytes\s*\(\s*\)' | grep -viE '(rd\.Bytes|spec\.|json\.|\.String\(\)\.Bytes)' | head -3 || true)
if [[ -n "$BYTES_MATCHES" ]]; then
    ERRORS+=(".Bytes() in encoding code — use .WriteTo(buf, off) instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$BYTES_MATCHES"
fi

# 4. Check for .Pack() calls (legacy pattern)
PACK_MATCHES=$(echo "$CONTENT" | grep -nE '\.\s*Pack\s*\(\s*\)' | head -3 || true)
if [[ -n "$PACK_MATCHES" ]]; then
    ERRORS+=(".Pack() is legacy — use .WriteTo(buf, off) instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$PACK_MATCHES"
fi

# 5. Check for buildFoo() returning []byte (should be writeFoo writing into buffer)
BUILD_RETURN_MATCHES=$(echo "$CONTENT" | grep -nE 'func\s+build\w+\(.*\)\s*\(\s*\[\s*\]\s*byte' | head -3 || true)
if [[ -n "$BUILD_RETURN_MATCHES" ]]; then
    ERRORS+=("build*() returning []byte — use write*() writing into caller buffer:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$BUILD_RETURN_MATCHES"
fi

# 6. Check for Len()-then-WriteTo() double traversal (anti-pattern in hot paths)
# Detect: x.Len() followed by WriteAttrTo(x, ...) — should use WriteAttrToWithLen.
# Allow: CheckedWriteTo (capacity guard), WriteAttrToWithLen (already fixed).
LEN_WRITE_MATCHES=$(echo "$CONTENT" | grep -nE '\.Len\(\)' | grep -viE '(CheckedWriteTo|WriteAttrToWithLen|// .*Len)' | head -3 || true)
if [[ -n "$LEN_WRITE_MATCHES" ]]; then
    # Only flag if there's also a WriteAttrTo (not WithLen) call nearby
    HAS_WRITE_ATTR_TO=$(echo "$CONTENT" | grep -cE 'WriteAttrTo\(' | grep -v 'WriteAttrToWithLen\|WriteAttrToWithContext' || true)
    if [[ "$HAS_WRITE_ATTR_TO" -gt 0 ]]; then
        ERRORS+=(".Len() with WriteAttrTo() — use WriteAttrToWithLen() or skip-and-backfill:")
        while IFS= read -r line; do
            [[ -n "$line" ]] && ERRORS+=("  $line")
        done <<< "$LEN_WRITE_MATCHES"
    fi
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Per-call allocation in encoding code${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}All encoding MUST use pooled buffers + WriteTo(buf, off)${RESET}" >&2
    echo -e "  ${YELLOW}Pool buffer = RFC max length = bounded encoding space${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/buffer-first.md${RESET}" >&2
    exit 2
fi

exit 0
