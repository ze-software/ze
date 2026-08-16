#!/bin/bash
# Sync canonical skills, subagents and AGENTS.md from their sources into
# tool-specific directories.
#
# Skills:
#   Canonical source: ai/skills/<name>.md (has YAML frontmatter)
#   Targets:
#     .claude/skills/<name>/SKILL.md  -- verbatim copy
#     .codex/skills/<name>/SKILL.md   -- verbatim copy
#     .agents/skills/<name>/SKILL.md  -- .claude/ paths replaced with .agents/
#
# Subagents:
#   Canonical source: ai/agents/<name>.md (frontmatter: name, description, tools)
#   Target:
#     .claude/agents/<name>.md        -- verbatim copy, flat, no per-agent dir
#   Claude Code is the only tool here that reads agent definitions, so there is
#   one target rather than three. The layout is the harness's, not ours: it
#   loads .claude/agents/<name>.md at SESSION START, so a new or edited agent
#   takes effect in the NEXT session, never the one that wrote it.
#
# CLAUDE.md / AGENTS.md:
#   Generated from ai/INSTRUCTIONS.md ({{TOOL}} substituted).
#
# All targets are gitignored, so `git diff` can NEVER show drift for them.
# Use --check (content comparison against a fresh generation in a temp dir)
# to detect drift; the session-start hook runs it and warns.
#
# Usage: make ze-ai-skills-sync          (sync)
#        make ze-ai-sync-check         (check only, no writes)
#        scripts/dev/skill_sync.sh [--dry-run|--check]

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

CANON_DIR="ai/skills"
CLAUDE_DIR=".claude/skills"
CODEX_DIR=".codex/skills"
AGENTS_DIR=".agents/skills"
AGENT_CANON_DIR="ai/agents"
AGENT_CLAUDE_DIR=".claude/agents"
INSTRUCTIONS="ai/INSTRUCTIONS.md"

mode="sync"
case "${1:-}" in
    --dry-run) mode="dry" ;;
    --check)   mode="check" ;;
esac

# generate_into <root>: write every generated output under <root>,
# preserving repo-relative paths.
generate_into() {
    local root="$1"
    local src name
    for src in "$CANON_DIR"/*.md; do
        [ -f "$src" ] || continue
        name=$(basename "$src" .md)

        # Claude: verbatim copy
        mkdir -p "$root/$CLAUDE_DIR/$name"
        cp "$src" "$root/$CLAUDE_DIR/$name/SKILL.md"

        # Codex: verbatim copy
        mkdir -p "$root/$CODEX_DIR/$name"
        cp "$src" "$root/$CODEX_DIR/$name/SKILL.md"

        # Agents (Codex CLI): replace .claude/ references with .agents/
        mkdir -p "$root/$AGENTS_DIR/$name"
        sed 's/\.claude\//\.agents\//g' "$src" > "$root/$AGENTS_DIR/$name/SKILL.md"
    done

    # Subagent definitions: one flat file per agent, Claude Code only.
    mkdir -p "$root/$AGENT_CLAUDE_DIR"
    for src in "$AGENT_CANON_DIR"/*.md; do
        [ -f "$src" ] || continue
        cp "$src" "$root/$AGENT_CLAUDE_DIR/$(basename "$src")"
    done

    # Generate tool-specific instruction files from ai/INSTRUCTIONS.md
    if [ -f "$INSTRUCTIONS" ]; then
        sed 's/{{TOOL}}/Claude/' "$INSTRUCTIONS" > "$root/CLAUDE.md"
        sed 's/{{TOOL}}/Codex/'  "$INSTRUCTIONS" > "$root/AGENTS.md"
    fi
}

if [ "$mode" = "dry" ]; then
    for src in "$CANON_DIR"/*.md; do
        [ -f "$src" ] || continue
        echo "would sync: $(basename "$src" .md)"
    done
    exit 0
fi

if [ "$mode" = "check" ]; then
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    generate_into "$tmpdir"

    stale=0
    # diff -rq also reports orphans ("Only in <dir>"): a mirror entry whose
    # canonical source was removed, or a missing mirror for a new skill.
    for dir in "$CLAUDE_DIR" "$CODEX_DIR" "$AGENTS_DIR" "$AGENT_CLAUDE_DIR"; do
        if ! diff -rq "$tmpdir/$dir" "$dir"; then
            stale=1
        fi
    done
    for f in CLAUDE.md AGENTS.md; do
        if ! diff -q "$tmpdir/$f" "$f" >/dev/null 2>&1; then
            echo "stale: $f"
            stale=1
        fi
    done

    if [ "$stale" -ne 0 ]; then
        echo "generated agent files are stale -- run: make ze-generated-files-update" >&2
        exit 1
    fi
    echo "generated agent files in sync"
    exit 0
fi

synced=0
for src in "$CANON_DIR"/*.md; do
    [ -f "$src" ] || continue
    synced=$((synced + 1))
done
agents=0
for src in "$AGENT_CANON_DIR"/*.md; do
    [ -f "$src" ] || continue
    agents=$((agents + 1))
done
generate_into "."
echo "synced $synced skill(s) + $agents agent(s) + CLAUDE.md + AGENTS.md"
