#!/bin/bash
# SubagentStart hook: inject compact project context into every spawned agent.
# Output is automatically prepended to the agent's context.

DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_DIR="$(cd "$DIR/.." && pwd)"

# Selected spec (if any)
SPEC=""
if [ -f "$PROJECT_DIR/tmp/session/selected-spec" ]; then
  SPEC=$(cat "$PROJECT_DIR/tmp/session/selected-spec" 2>/dev/null | tr -d '[:space:]')
fi

# Current branch
BRANCH=$(git -C "$PROJECT_DIR" branch --show-current 2>/dev/null || echo "unknown")

cat <<EOF
Ze is a Network OS in Go (BGP, CLI, web, plugins). Key constraints:
- Zero-copy, buffer-first encoding: WriteTo(buf, off) int -- no make/append in encoding
- Registration pattern: init() in register.go, never direct imports between components
- YANG required for all RPCs -- no "command module" category
- Lazy over eager: pass raw bytes, offset iterators, no intermediate structs
- JSON keys: kebab-case (exception: lg/handler_api.go for birdwatcher compat)
- Config pipeline: File -> Tree -> ResolveBGPTree() -> map[string]any -> PeersFromTree()
- Goroutines: long-lived workers on channels, never per-event
- Rules: ai/rules/ (buffer-first.md, design-principles.md, plugin-design.md)
- Branch: $BRANCH
EOF

if [ -n "$SPEC" ] && [ -f "$PROJECT_DIR/plan/$SPEC" ]; then
  echo "- Spec: $SPEC"
fi
