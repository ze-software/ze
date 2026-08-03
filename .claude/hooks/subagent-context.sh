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
source .claude/hooks/lib/state-file.sh 2>/dev/null || true
if type _session_id &>/dev/null; then
    SID=$(_session_id 2>/dev/null || echo "")
    if [ -n "$SID" ] && [ -f "tmp/session/.session-${SID}" ]; then
        CLAIM=$(head -1 "tmp/session/.session-${SID}" 2>/dev/null || true)
        if [ -n "$CLAIM" ] && [ "$CLAIM" != "unassigned" ] && [ -f "plan/$CLAIM" ]; then
            SPEC="plan/$CLAIM"
            SPEC_STATUS=$(grep -m1 -E "^\| Status \|" "$SPEC" 2>/dev/null | sed 's/|//g; s/Status//; s/^ *//; s/ *$//')
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
fi

cat <<'EOF'

You are a subagent under ai/rules/planning.md. Your contract:
- Report FACTS, each cited as file:line, and read the function that PRODUCES a
  behavior rather than one that consumes it (ai/rules/evidence.md). Your
  report is a claim the main thread will verify, not evidence on its own.
- You have NO LSP tool and you CANNOT ask the user. If the task genuinely needs
  either, say so in your report and stop rather than guessing.
- Rules are ROUTED, not preloaded. ai/rules/TRIGGERS.md names every rule in one
  line each, with its severity and the situation that makes it apply. When a
  trigger matches your task, READ ai/rules/<name>.md before acting on its topic.
  That read is the only way you get the rule: the trigger line is all you hold.
  ai/rules/CORE.md is already loaded in full and needs no read.
- Never claim done with work remaining (ai/rules/completion.md), and
  never park a blocker or weaken a test to reach green (ai/rules/completion.md).
EOF
