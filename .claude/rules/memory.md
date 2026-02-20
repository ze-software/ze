# Ze Project Memory

Rationale: `.claude/rationale/memory.md`

## Memory Maintenance (BLOCKING at session end)

Before committing at session end:
1. **Dedup**: remove entries already in `.claude/rules/*.md` — no duplication
2. **Stale**: remove entries referencing deleted files/functions
3. **Merge**: combine related bullets, keep heading + 1-3 lines max
4. **Overflow**: entries >5 lines → move to `.claude/rationale/memory.md`
5. **MEMORY.md cap**: 200 lines hard limit (system truncates after)

## When to Consult Rationale

Read `.claude/rationale/<name>.md` when:
- A rule says something but you need to know WHY
- You need examples/code patterns referenced by a rule
- You encounter a situation the compressed rule doesn't fully cover

## Project Knowledge (not covered by other rules)

### Family Registration
Families registered DYNAMICALLY by plugins via `PluginRegistry.Register()` — not a static list.
Validate format (contains "/", non-empty parts) — never enumerate all families.

### Config Pipeline
BGPConfig eliminated. Flow: file → Tree → ResolveBGPTree() → map[string]any → reactor.PeersFromTree().
Key files: `config/resolve.go`, `config/peers.go`, `reactor/config.go`.

### Bash Timeout
Default 15000ms. Longer only for `make ze-verify`, `make ze-unit-test`.

### Linter Hook
`auto_linter.sh` runs goimports on Edit/Write. Add import + usage in same edit to avoid cascading removals.
