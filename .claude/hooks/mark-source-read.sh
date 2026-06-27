#!/bin/bash
# PostToolUse hook on Read: refresh a freshness marker whenever implementation
# source (.go under internal/, pkg/, cmd/) is read.
#
# Companion to mark-lsp-invoked.sh. The c_design_without_lsp check in
# pretool-writeedit.py accepts EITHER marker before allowing a spec/design file
# write: an LSP invocation OR a source Read. Rationale: reading the function that
# PRODUCES a behavior is the verification we actually want before authoring a
# spec that claims something about that behavior (ai/rules/no-fabrication.md,
# "Behavioral claims and recommendations"). Requiring the LSP tool specifically
# false-negatives legitimate investigation done via the Read tool.
#
# Marker path: tmp/session/.source-read-<SID>. Content: ISO-8601 timestamp.
# Non-blocking: this hook only records; it never rejects a Read.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only implementation Go source counts as "investigated the producer".
# Docs, specs, config, and markdown do not satisfy the gate.
case "$FILE_PATH" in
    */internal/*.go|*/pkg/*.go|*/cmd/*.go)
        mkdir -p tmp/session
        date -Iseconds > "tmp/session/.source-read-${SID}"
        ;;
esac

exit 0
