#!/bin/bash
# Sync canonical skills from ai/skills/ into tool-specific directories.
#
# Canonical source: ai/skills/<name>.md
# Targets:
#   .claude/skills/<name>/SKILL.md  -- verbatim copy
#   .codex/skills/<name>/SKILL.md   -- YAML frontmatter added
#   .agents/skills/<name>/SKILL.md  -- .claude/ paths replaced with .agents/
#
# Usage: make ze-skill-sync
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

echo "synced $synced skill(s) to $CLAUDE_DIR, $CODEX_DIR, $AGENTS_DIR"
