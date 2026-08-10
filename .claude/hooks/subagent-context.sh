#!/bin/bash
# SubagentStart hook: inject compact project context into every spawned agent.
# Output is automatically prepended to the agent's context.
#
# This exists to make delegation CHEAP. ai/rules/planning.md requires the
# main thread to hand every subagent its spec, its phase, and the rules that
# govern it; when that is manual per-spawn work, delegating costs more than
# working inline and the rule loses. So the harness supplies it instead.
#
# The claimed spec resolves through the parent session's marker: subagents
# inherit $CLAUDE_CODE_SESSION_ID from the parent deliberately
# (.claude/hooks/lib/session_id.py, "Precedence" 1), so tmp/session/.session-<sid>
# is the PARENT's claim, which is exactly the one the agent is working under.

# $CLAUDE_PROJECT_DIR first, $0-relative only as a fallback -- the convention every
# other hook here follows. Deriving the root from $0 alone resolves to the checkout
# the script lives in, which is the wrong tree for a worktree and untestable from a
# fixture project.
cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.." || exit 0

# Current branch
BRANCH=$(git branch --show-current 2>/dev/null || echo "unknown")

# Parent session's claimed spec, if any.
SPEC=""
SPEC_STATUS=""
SPEC_STATE=""
source .claude/hooks/lib/state-file.sh 2>/dev/null || true
if type _session_id &>/dev/null; then
    SID=$(_session_id 2>/dev/null || echo "")
    if [ -n "$SID" ] && [ -f "tmp/session/.session-${SID}" ]; then
        CLAIM=$(head -1 "tmp/session/.session-${SID}" 2>/dev/null || true)
        if [ -n "$CLAIM" ] && [ "$CLAIM" != "unassigned" ] && [ -f "plan/$CLAIM" ]; then
            SPEC="plan/$CLAIM"
            SPEC_STATUS=$(grep -m1 -E "^\| Status \|" "$SPEC" 2>/dev/null | sed 's/|//g; s/Status//; s/^ *//; s/ *$//')
            # The per-spec digest an earlier phase wrote. One resolver owns this
            # file family (lib/state-file.sh); never spell a second path here.
            # Absent file means absent line: an empty path teaches nothing.
            if type _find_latest_state_for_spec &>/dev/null; then
                STEM=$(echo "$CLAIM" | sed 's/^spec-//; s/\.md$//')
                SPEC_STATE=$(_find_latest_state_for_spec "$STEM" 2>/dev/null || true)
                [ -n "$SPEC_STATE" ] && [ -f "$SPEC_STATE" ] || SPEC_STATE=""
            fi
        fi
    fi
fi

cat <<EOF
Ze is a Network OS in Go (BGP, CLI, web, plugins). Key constraints:
- Zero-copy, buffer-first encoding: WriteTo(buf, off) int -- no make/append in encoding
- Registration pattern: init() in register.go, never direct imports between components
- YANG required for all RPCs -- no "command module" category
- Lazy over eager: pass raw bytes, offset iterators, no intermediate structs
- JSON keys: kebab-case (exception: lg/handler_api.go for birdwatcher compat)
- Config pipeline: File -> Tree -> ResolveBGPTree() -> map[string]any -> PeersFromTree()
- Goroutines: long-lived workers on channels, never per-event
- Rules: ai/rules/ (performance.md, architecture.md, plugins.md)
- Branch: $BRANCH
EOF

if [ -n "$SPEC" ]; then
    cat <<EOF

Spec claimed by the session that spawned you: $SPEC${SPEC_STATUS:+ (Status: $SPEC_STATUS)}
Read it before acting. Its acceptance criteria are what your work is judged against.
EOF
    if [ -n "$SPEC_STATE" ]; then
        cat <<EOF
Digest of that spec's earlier phases: $SPEC_STATE
Read it before re-deriving what an earlier phase already established.
EOF
    fi
fi

cat <<'EOF'

You are a subagent under ai/rules/planning.md. Your contract:
- Report FACTS, each cited as file:line, and read the function that PRODUCES a
  behavior rather than one that consumes it (ai/rules/evidence.md). Your
  report is a claim the main thread will verify, not evidence on its own.
- To resolve a symbol, in this order: try the LSP tool (ToolSearch
  query="select:LSP") and use it if your registry carries it; if that comes back
  empty, run gopls from Bash -- same server, same answers. gopls symbols <file>
  maps a file, then gopls definition|references <file>:<line>:<col>. Prefer the
  file, symbol and line range your prompt already carries. Never read a whole
  file to hunt for a symbol, and never report that you could not look. You
  CANNOT ask the user.
- Rules are ROUTED, not preloaded. ai/rules/TRIGGERS.md names every rule in one
  line each, with its severity and the situation that makes it apply. When a
  trigger matches your task, READ ai/rules/<name>.md before acting on its topic.
  That read is the only way you get the rule: the trigger line is all you hold.
  ai/rules/CORE.md is already loaded in full and needs no read.
- Never claim done with work remaining (ai/rules/completion.md), and
  never park a blocker or weaken a test to reach green (ai/rules/completion.md).
- Write every log, capture and scratch file under
  dir=$(scripts/dev/session-scratch.sh), which is the scratch/ subdirectory of
  tmp/session/<YYYY-MM-DD>-<session-id>/. tmp/ is keyed per CHECKOUT, so a fixed name at its root
  (tmp/out.log, tmp/agentA.log) is the same file for every session working this
  tree, and nothing cleans it. Full rule: ai/rules/commands.md.
- Batch independent tool calls into ONE message: 85% of measured API calls
  carried exactly one tool call.
- Read the range you were given; where none was, Grep first and Read the range
  it names, never the whole file: 62% of measured Reads re-read a path this
  session had already read.
  Full rule: ai/rules/context-economy.md.
EOF
