# Ze Project Memory (Claude-Specific)

## Maintenance (BLOCKING at session end)

1. **Dedup** against `ai/rules/*.md` and `.claude/rules/*.md`
2. **Stale**: drop entries referencing deleted files/functions
3. **Merge** related bullets; heading + 1-3 lines max
4. **Overflow** >5 lines -> `ai/rationale/memory.md`
5. **Cap**: 200 lines hard

Consult `ai/rationale/<name>.md` when the compressed rule leaves gaps.

Project knowledge and mistake log: `ai/rules/project-knowledge.md`
