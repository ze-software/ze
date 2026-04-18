#!/bin/bash
# BLOCKING HOOK: refuses every tool call other than ToolSearch until the
# session has loaded the LSP tool via `ToolSearch query="select:LSP"`.
#
# Rationale: rules/session-start.md step 1 is BLOCKING with a no-exceptions
# clause, but the rule was repeatedly rationalized away ("the task is
# shell-only", "I won't navigate Go"). This hook makes the rule mechanical:
# the very first tool call of any new session must be the LSP load,
# otherwise every subsequent tool call is rejected with exit code 2.
#
# Marker file: tmp/session/.lsp-loaded-<SID>. Written when a ToolSearch with
# "LSP" in its query is observed; every other tool call is allowed once the
# marker exists. The marker is per-session (stable session id from the JWT
# access token, PPID-walk fallback) and survives context compaction because
# the file-system outlasts the in-memory tool table.
#
# Bypass for existing sessions: `touch tmp/session/.lsp-loaded-<SID>`.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)
mkdir -p tmp/session
MARKER="tmp/session/.lsp-loaded-${SID}"

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')

# ToolSearch itself is always allowed -- it is how LSP gets loaded.
if [ "$TOOL_NAME" = "ToolSearch" ]; then
    QUERY=$(echo "$INPUT" | jq -r '.tool_input.query // empty')
    # Treat any query that names LSP (select:LSP, or a keyword search that
    # matches LSP's description) as sufficient to lift the gate. The cost of
    # false-positive unblock is small; the cost of false-negative block is a
    # stuck session.
    if echo "$QUERY" | grep -qi "LSP"; then
        date -Iseconds > "$MARKER"
    fi
    exit 0
fi

# LSP already loaded this session -> allow.
if [ -f "$MARKER" ]; then
    exit 0
fi

# Block everything else.
cat >&2 <<'EOF'
❌ Blocked: LSP tool must be loaded before any other tool call.

   First tool call of every session MUST be:
       ToolSearch query="select:LSP"

   See rules/session-start.md, "LSP Load (step 1) -- no-exceptions clause".
   No task-type exception (shell-only, docs-only, trivial, etc.) applies.

   Bypass for existing sessions (rare, e.g. hook added mid-session):
       touch tmp/session/.lsp-loaded-$(whichever _session_id resolves to)
EOF
exit 2
