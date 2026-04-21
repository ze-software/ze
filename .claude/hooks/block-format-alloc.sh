#!/bin/bash
# PreToolUse hook: Block fmt.Sprintf / fmt.Fprintf / strings.Builder / strings.Join
# in BGP text/JSON format-generation files migrated by fmt-0 / fmt-2-json-append.
# BLOCKING: All text+JSON formatting in these files must use Append idiom
# (strconv.AppendUint, netip.Addr.AppendTo, hex.AppendEncode, etc.).
# References:
#   ai/rules/buffer-first.md                -- Append shape + banned-pattern discipline
#   plan/learned/614-fmt-0-append.md             -- idiom fmt-2 extends (hand-maintained guard over 3 files)
#   plan/learned/NNN-fmt-2-json-append.md        -- graduates the guard to a real hook over 9 files

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit for Go files.
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip test files -- tests may use fmt.Sprintf for assertion messages.
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip empty edits (e.g., pure deletions).
if [[ -z "$CONTENT" ]]; then
    exit 0
fi

# Allowlist: files migrated by fmt-0 (already clean) + file migrated by
# fmt-2-json-append. The hook makes the hand-maintained discipline mechanical.
# `json.go` is intentionally excluded: its map[string]any + json.Marshal idiom
# is out of scope for fmt-2 (deferred until a concrete perf need emerges).
IS_FORMAT_FILE=0
case "$FILE_PATH" in
    */bgp/reactor/filter_format.go)       IS_FORMAT_FILE=1 ;;
    */bgp/attribute/text.go)              IS_FORMAT_FILE=1 ;;
    */bgp/format/text.go)                 IS_FORMAT_FILE=1 ;;
    */bgp/format/text_json.go)            IS_FORMAT_FILE=1 ;;
    */bgp/format/text_update.go)          IS_FORMAT_FILE=1 ;;
    */bgp/format/text_human.go)           IS_FORMAT_FILE=1 ;;
    */bgp/format/summary.go)              IS_FORMAT_FILE=1 ;;
    */bgp/format/codec.go)                IS_FORMAT_FILE=1 ;;
    */bgp/format/decode.go)               IS_FORMAT_FILE=1 ;;
esac

if [[ "$IS_FORMAT_FILE" -eq 0 ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Banned primitives -- exact reference list matches fmt-0 guard + fmt-2-json-append spec.
# Each pattern is an ERE that must not appear in the submitted content.
declare -A BANNED=(
    ['fmt\.Sprintf\(']="fmt.Sprintf -- use strconv.AppendUint / netip.Addr.AppendTo / hex.AppendEncode"
    ['fmt\.Fprintf\(']="fmt.Fprintf -- use Append shape writing into a caller buffer"
    ['strings\.Join\(']="strings.Join -- build with append(buf, sep...) instead"
    ['strings\.Builder']="strings.Builder -- use []byte scratch + append"
    ['strings\.NewReplacer']="strings.NewReplacer -- use a bounded byte-by-byte rewrite"
    ['strings\.ReplaceAll\(']="strings.ReplaceAll -- use a bounded byte-by-byte rewrite"
    ['strconv\.FormatUint\(']="strconv.FormatUint -- use strconv.AppendUint into scratch buffer"
    ['strconv\.FormatInt\(']="strconv.FormatInt -- use strconv.AppendInt into scratch buffer"
)

for pattern in "${!BANNED[@]}"; do
    # grep -E is POSIX ERE; `//.*pattern` comments should not trigger either way
    MATCHES=$(echo "$CONTENT" | grep -nE "$pattern" | head -3 || true)
    if [[ -n "$MATCHES" ]]; then
        ERRORS+=("${BANNED[$pattern]}")
        while IFS= read -r line; do
            [[ -n "$line" ]] && ERRORS+=("  $line")
        done <<< "$MATCHES"
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}BLOCKED: banned format primitive in ${FILE_PATH}${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}x${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}This file is gated by the fmt-0 / fmt-2-json-append Append-idiom hook.${RESET}" >&2
    echo -e "  ${YELLOW}Allowed helpers: strconv.AppendUint, netip.Addr.AppendTo,${RESET}" >&2
    echo -e "  ${YELLOW}hex.AppendEncode, or a local [N]byte scratch + append.${RESET}" >&2
    echo -e "  ${YELLOW}Rules: ai/rules/buffer-first.md${RESET}" >&2
    exit 2
fi

exit 0
