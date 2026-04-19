#!/bin/bash
# BLOCKING HOOK: refuses Edit / Write to plan/design-*.md and
# plan/spec-*.md unless the LSP tool has been ACTUALLY INVOKED in the
# current session within the freshness window.
#
# Rationale: `block-until-lsp.sh` enforces that the LSP tool schema is
# loaded at session start. That is not the same as USING it. A session
# can load LSP once and then write pages of architectural claims
# without ever calling goToDefinition / findReferences / documentSymbol
# to verify a single symbol. This hook closes that loophole for the
# claim-making files (design docs and specs).
#
# Flow:
#   * PostToolUse on LSP writes tmp/session/.lsp-invoked-<SID>.
#   * This hook runs PreToolUse on Edit|Write targeting plan/design-*.md
#     or plan/spec-*.md. It rejects unless the marker exists AND is
#     fresher than LSP_FRESHNESS_SECONDS (default 1800 = 30 min).
#
# Freshness is needed because an LSP call from three hours ago does not
# validate a design claim written now. The session-wide "any call is
# enough" variant is too weak.
#
# Bypass (explicit): `touch tmp/session/.lsp-invoked-<SID>` to refresh
# the marker when you genuinely do not need LSP for a tiny edit.

cd "${CLAUDE_PROJECT_DIR:-}" 2>/dev/null || cd "$(dirname "$0")/../.."
PROJECT_DIR=$(pwd -P)

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

case "$TOOL_NAME" in
    Edit|Write|MultiEdit|NotebookEdit) ;;
    *) exit 0 ;;
esac

# Normalise file path to repo-relative for matching.
REL="${FILE_PATH#"$PROJECT_DIR"/}"

# Scope: plan/design-*.md and plan/spec-*.md only. Learned summaries
# (plan/learned/) and deferrals (plan/deferrals.md) are out of scope --
# they are post-work recordings, not claim-making design files.
case "$REL" in
    plan/design-*.md|plan/spec-*.md) ;;
    *) exit 0 ;;
esac

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)
MARKER="tmp/session/.lsp-invoked-${SID}"

: "${LSP_FRESHNESS_SECONDS:=1800}"

if [ ! -f "$MARKER" ]; then
    cat >&2 <<EOF
❌ Blocked: LSP has not been invoked in this session.

   You are about to write $REL, a design / spec file. The project rule
   (rules/design-context.md) requires grepping and verifying against
   the code before producing design claims. LSP is the precise tool
   for that: goToDefinition, findReferences, documentSymbol.

   Run at least one LSP operation that exercises a symbol relevant to
   this design, then retry the edit.

   Bypass (only when LSP is genuinely irrelevant):
       touch $MARKER
EOF
    exit 2
fi

# Freshness check: POSIX mtime in seconds since epoch.
NOW=$(date +%s)
if command -v stat >/dev/null 2>&1; then
    # macOS stat -f, GNU stat -c
    MTIME=$(stat -f %m "$MARKER" 2>/dev/null || stat -c %Y "$MARKER" 2>/dev/null)
else
    MTIME=""
fi

if [ -n "$MTIME" ]; then
    AGE=$(( NOW - MTIME ))
    if [ "$AGE" -gt "$LSP_FRESHNESS_SECONDS" ]; then
        cat >&2 <<EOF
❌ Blocked: LSP invocation is stale (${AGE}s old, limit ${LSP_FRESHNESS_SECONDS}s).

   You are about to write $REL, a design / spec file. The last LSP
   call was more than $(( LSP_FRESHNESS_SECONDS / 60 )) minutes ago, which does not
   count as fresh evidence for design claims made now.

   Run a fresh LSP operation that exercises a symbol relevant to this
   edit, then retry.

   Bypass (only when LSP is genuinely irrelevant):
       touch $MARKER
EOF
        exit 2
    fi
fi

exit 0
