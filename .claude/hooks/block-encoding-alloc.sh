#!/bin/bash
# PreToolUse hook: Block per-call allocations in wire-facing code
# BLOCKING: All wire encoding must use pooled buffers, not make()/append()/Bytes()/Pack().
# References:
#   ai/rules/buffer-first.md          -- mechanical reference
#   ai/rules/design-principles.md     -- "No make where pools exist", "Pool strategy by goroutine shape"
#   plan/learned/603-make-pool-audit.md    -- 2026-04-16 audit that expanded scope to TACACS+, plugin-rpc,
#                                              BGP forward_build, BMP sender, BFD Verify, L2TP reliable

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

# Scope: BGP wire-encoding hot paths only. Plugin encode.go and CLI/API
# routes are NOT hot path (called at human speed; pools are wrong there).
#
# The 2026-04-16 audit (learned/603) considered widening this hook to
# every wire-facing subsystem (TACACS+, plugin-rpc, BMP, BFD, L2TP).
# Empirical testing showed that line-based regex cannot distinguish:
#   - legitimate one-shot `make([]byte, N)` in pool New funcs and
#     per-session scratch init from
#   - new variable-size allocations on the hot path
# nor can it distinguish:
#   - intentional `append(buf, ...)` builders (plugin-rpc AppendXxx) from
#   - byte-slice append regressions
# Widening produced too many false positives in the migrated files.
#
# Scope is narrow on purpose. Files included below have been audited:
# every existing `make([]byte, ...)` site in them is either allowlisted
# by the MAKE_ALLOW regex or carries a `// pool-fallback` opt-in comment.
# Coverage of the other migrated subsystems (bmp/sender, filter_community,
# bgp/nlri) is enforced by:
#   - the design rules in `ai/rules/design-principles.md`
#   - the per-subsystem pool helpers (calling `make` instead is obvious in review)
#   - `/ze-find-alloc` for periodic auditing
IS_ENCODE=0
case "$FILE_PATH" in
    */message/update_build*)   IS_ENCODE=1 ;;
    */message/update_split*)   IS_ENCODE=1 ;;
    */message/pack*)           IS_ENCODE=1 ;;
    */reactor_wire*)           IS_ENCODE=1 ;;
    */reactor/forward_build*)  IS_ENCODE=1 ;;
    */bgp/nlri/base*)          IS_ENCODE=1 ;;
    */bgp/nlri/inet*)          IS_ENCODE=1 ;;
    */bgp/nlri/rd*)            IS_ENCODE=1 ;;
esac

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
APPEND_MATCHES=$(echo "$CONTENT" | grep -nE 'append[[:space:]]*\(' | grep -viE '(args|strings|labels|families|errors|ERRORS|names|fields|parts)' | grep -viE '//.*append' | head -3 || true)
if [[ -n "$APPEND_MATCHES" ]]; then
    ERRORS+=("append() in encoding code — use pre-computed size + offset writes:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$APPEND_MATCHES"
fi

# 2. Check for make([]byte in non-pool context.
# Allowlist: pool New funcs (return make), Pool/New named contexts,
# specific named patterns, and explicit `// pool-fallback` markers.
#
# `result := make` is NOT blanket-exempted (audit 2026-04-16 found it
# masked update_build.go:496). Code that legitimately allocates a result
# slice for a sync.Pool fallback must add `// pool-fallback` on the same
# line as the make call to opt in.
MAKE_ALLOW='return make|Pool|New.*func|nlriBytes[[:space:]]*:=[[:space:]]*make|owned[[:space:]]*:=[[:space:]]*make|//[[:space:]]*pool-fallback'
MAKE_MATCHES=$(echo "$CONTENT" | grep -nE 'make[[:space:]]*\([[:space:]]*\[[[:space:]]*\][[:space:]]*byte' | grep -viE "($MAKE_ALLOW)" | head -3 || true)
if [[ -n "$MAKE_MATCHES" ]]; then
    ERRORS+=("make([]byte) in encoding hot path — use pool buffer instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$MAKE_MATCHES"
fi

# 3. Check for .Bytes() calls (should use .WriteTo)
# Only exclude: rd.Bytes() which returns a fixed [8]byte (no alloc), and json/spec access patterns
BYTES_MATCHES=$(echo "$CONTENT" | grep -nE '\.[[:space:]]*Bytes[[:space:]]*\([[:space:]]*\)' | grep -viE '(rd\.Bytes|spec\.|json\.|\.String\(\)\.Bytes)' | head -3 || true)
if [[ -n "$BYTES_MATCHES" ]]; then
    ERRORS+=(".Bytes() in encoding code — use .WriteTo(buf, off) instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$BYTES_MATCHES"
fi

# 4. Check for .Pack() calls (legacy pattern)
PACK_MATCHES=$(echo "$CONTENT" | grep -nE '\.[[:space:]]*Pack[[:space:]]*\([[:space:]]*\)' | head -3 || true)
if [[ -n "$PACK_MATCHES" ]]; then
    ERRORS+=(".Pack() is legacy — use .WriteTo(buf, off) instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$PACK_MATCHES"
fi

# 5. Check for buildFoo() returning []byte (should be writeFoo writing into buffer)
BUILD_RETURN_MATCHES=$(echo "$CONTENT" | grep -nE 'func[[:space:]]+build[[:alnum:]_]+\(.*\)[[:space:]]*\([[:space:]]*\[[[:space:]]*\][[:space:]]*byte' | head -3 || true)
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
    echo -e "${RED}${BOLD}❌ BLOCKED: per-call allocation in encoding hot path${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Encoding hot path MUST use pooled buffers + WriteTo(buf, off).${RESET}" >&2
    echo -e "  ${YELLOW}Pool buffer = RFC max length = bounded encoding space.${RESET}" >&2
    echo -e "  ${YELLOW}If a per-call make IS a legit sync.Pool fallback or one-shot${RESET}" >&2
    echo -e "  ${YELLOW}init, add '// pool-fallback' on the same line as the make call.${RESET}" >&2
    echo -e "  ${YELLOW}Rules: ai/rules/{buffer-first,design-principles}.md${RESET}" >&2
    exit 2
fi

exit 0
