#!/bin/bash
# Sync canonical skills and AGENTS.md from their sources into
# tool-specific directories.
#
# Skills:
#   Canonical source: ai/skills/<name>.md
#   Targets:
#     .claude/skills/<name>/SKILL.md  -- verbatim copy
#     .codex/skills/<name>/SKILL.md   -- YAML frontmatter added
#     .agents/skills/<name>/SKILL.md  -- .claude/ paths replaced with .agents/
#
# AGENTS.md:
#   Generated from CLAUDE.md with title adjusted for Codex.
#
# Usage: make ze-ai-sync
#        scripts/dev/skill_sync.sh [--dry-run]

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

CANON_DIR="ai/skills"
CLAUDE_DIR=".claude/skills"
CODEX_DIR=".codex/skills"
AGENTS_DIR=".agents/skills"

dry_run=false
if [ "${1:-}" = "--dry-run" ]; then
    dry_run=true
fi

synced=0

for src in "$CANON_DIR"/*.md; do
    [ -f "$src" ] || continue
    name=$(basename "$src" .md)

    # Extract first heading as description (strip # prefix)
    first_line=$(head -1 "$src")
    description="${first_line#\# }"

    if $dry_run; then
        echo "would sync: $name"
        continue
    fi

    # Claude: verbatim copy
    mkdir -p "$CLAUDE_DIR/$name"
    cp "$src" "$CLAUDE_DIR/$name/SKILL.md"

    # Codex: add YAML frontmatter
    mkdir -p "$CODEX_DIR/$name"
    {
        echo "---"
        echo "name: $name"
        echo "description: $description"
        echo "---"
        echo ""
        cat "$src"
    } > "$CODEX_DIR/$name/SKILL.md"

    # Agents (Codex CLI): replace .claude/ references with .agents/
    mkdir -p "$AGENTS_DIR/$name"
    sed 's/\.claude\//\.agents\//g' "$src" > "$AGENTS_DIR/$name/SKILL.md"

    synced=$((synced + 1))
done

# Generate tool-specific instruction files from ai/INSTRUCTIONS.md
INSTRUCTIONS="ai/INSTRUCTIONS.md"
if [ -f "$INSTRUCTIONS" ]; then
    sed 's/{{TOOL}}/Claude/' "$INSTRUCTIONS" > CLAUDE.md
    sed 's/{{TOOL}}/Codex/'  "$INSTRUCTIONS" > AGENTS.md
fi

echo "synced $synced skill(s) + CLAUDE.md + AGENTS.md"
