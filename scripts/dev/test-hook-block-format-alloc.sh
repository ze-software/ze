#!/bin/bash
# Smoke test for .claude/hooks/block-format-alloc.sh.
#
# Verifies:
#   AC-8  -- Write event for decode.go containing fmt.Sprintf     -> exit 2
#   AC-9  -- Write event for json.go  containing json.Marshal     -> exit 0 (out of scope)
#   Negative control: clean content in allowlisted file           -> exit 0
#   Negative control: banned content in a non-allowlisted file    -> exit 0
#   Test-file short-circuit                                       -> exit 0
#
# Usage: scripts/dev/test-hook-block-format-alloc.sh
# Exits 0 on success, 1 on first failed expectation.

set -eu

HOOK="$(cd "$(dirname "$0")/../.." && pwd)/.claude/hooks/block-format-alloc.sh"

if [[ ! -x "$HOOK" ]]; then
    echo "FAIL: hook not executable at $HOOK" >&2
    exit 1
fi

pass=0
fail=0

run_case() {
    local desc="$1"
    local expected="$2"
    local payload="$3"

    set +e
    echo "$payload" | "$HOOK" >/dev/null 2>/dev/null
    local actual=$?
    set -e

    if [[ "$actual" == "$expected" ]]; then
        printf '  PASS: %-72s [exit=%s]\n' "$desc" "$actual"
        pass=$((pass + 1))
    else
        printf '  FAIL: %-72s expected=%s got=%s\n' "$desc" "$expected" "$actual" >&2
        fail=$((fail + 1))
    fi
}

# ---- AC-8: banned fmt.Sprintf in decode.go must be blocked -----------------
run_case "AC-8 fmt.Sprintf in decode.go blocks" 2 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/internal/component/bgp/format/decode.go",
    "content": "package format\n\nfunc bad(x uint8) string { return fmt.Sprintf(\"%d\", x) }\n"
  }
}'

# ---- AC-9: json.Marshal in json.go allowed (out of scope) -------------------
run_case "AC-9 json.Marshal in json.go allowed" 0 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/internal/component/bgp/format/json.go",
    "content": "package format\n\nfunc encode(m map[string]any) ([]byte, error) { return json.Marshal(m) }\n"
  }
}'

# ---- Negative control: clean Append-style content in allowlisted file -------
run_case "Clean Append content in text_json.go allowed" 0 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/internal/component/bgp/format/text_json.go",
    "content": "package format\n\nfunc good(buf []byte, x uint32) []byte { return strconv.AppendUint(buf, uint64(x), 10) }\n"
  }
}'

# ---- Negative control: banned content in non-allowlisted file allowed -------
run_case "fmt.Sprintf in non-allowlisted file allowed" 0 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/cmd/ze/other/main.go",
    "content": "package main\n\nfunc demo(x uint8) string { return fmt.Sprintf(\"%d\", x) }\n"
  }
}'

# ---- Test file short-circuit ------------------------------------------------
run_case "fmt.Sprintf in decode_test.go allowed (tests exempt)" 0 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/internal/component/bgp/format/decode_test.go",
    "content": "package format\n\nfunc TestFoo(t *testing.T) { fmt.Sprintf(\"%d\", 1) }\n"
  }
}'

# ---- strings.Builder banned in reactor/filter_format.go ---------------------
run_case "strings.Builder in filter_format.go blocks" 2 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/internal/component/bgp/reactor/filter_format.go",
    "content": "package reactor\n\nfunc bad() { var b strings.Builder; _ = b }\n"
  }
}'

# ---- Empty content short-circuit --------------------------------------------
run_case "Empty content short-circuits" 0 '{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/repo/internal/component/bgp/format/decode.go",
    "content": ""
  }
}'

# ---- Non-Write tool short-circuit -------------------------------------------
run_case "Non-Write tool short-circuits" 0 '{
  "tool_name": "Bash",
  "tool_input": {
    "command": "fmt.Sprintf(\"%d\", 1)"
  }
}'

echo ""
echo "PASS=$pass FAIL=$fail"
if [[ "$fail" -gt 0 ]]; then
    exit 1
fi
