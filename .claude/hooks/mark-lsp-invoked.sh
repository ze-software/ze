#!/bin/bash
# PostToolUse hook on LSP: write a freshness marker whenever the LSP tool
# is actually invoked (not just loaded via ToolSearch).
#
# The marker distinguishes "LSP is available this session" (covered by
# block-until-lsp.sh) from "LSP has been used to investigate code in the
# last N minutes" (what block-design-without-lsp.sh checks).
#
# Marker path: tmp/session/.lsp-invoked-<SID>
# File content: ISO-8601 timestamp of the most recent invocation.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)
mkdir -p tmp/session
MARKER="tmp/session/.lsp-invoked-${SID}"

date -Iseconds > "$MARKER"
exit 0
