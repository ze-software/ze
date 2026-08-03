#!/bin/bash
# PostToolUse hook on Read: refresh a freshness marker whenever implementation
# source is read. "Implementation source" is the file types a spec can
# legitimately be ABOUT: .go under internal/, pkg/, cmd/, PLUS .py under
# scripts/, .sh under .claude/hooks/, the Makefile, and mk/ files. A spec about
# a Python dev tool or a shell hook is grounded by reading THAT tool, not by
# reading an unrelated .go file purely to satisfy this gate (T-4).
#
# Companion to mark-lsp-invoked.sh. The c_design_without_lsp check in
# pretool-writeedit.py accepts EITHER marker before allowing a spec/design file
# write: an LSP invocation OR a source Read. Rationale: reading the function that
# PRODUCES a behavior is the verification we actually want before authoring a
# spec that claims something about that behavior (ai/rules/evidence.md,
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

# Implementation source a spec can be ABOUT counts as "investigated the
# producer": Go under internal/pkg/cmd, Python dev tooling under scripts/, shell
# hooks under .claude/hooks/, and the make wiring (Makefile, mk/). Docs, specs,
# config, and unrelated markdown do not satisfy the gate.
case "$FILE_PATH" in
    */internal/*.go|*/pkg/*.go|*/cmd/*.go \
    |*/scripts/*.py \
    |*/.claude/hooks/*.sh \
    |*/Makefile|Makefile \
    |*/mk/*.mk|mk/*.mk)
        mkdir -p tmp/session
        date -Iseconds > "tmp/session/.source-read-${SID}"
        ;;
esac

exit 0
