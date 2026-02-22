# Ze Project Memory

Rationale: `.claude/rationale/memory.md`

## Maintenance (BLOCKING at session end)

Before committing:
1. **Dedup**: remove entries already in `.claude/rules/*.md`
2. **Stale**: remove entries referencing deleted files/functions
3. **Merge**: combine related bullets, heading + 1-3 lines max
4. **Overflow**: entries >5 lines → `.claude/rationale/memory.md`
5. **Cap**: 200 lines hard limit (system truncates after)

## When to Consult Rationale

Read `.claude/rationale/<name>.md` when a rule needs context, examples, or the compressed rule doesn't fully cover the situation.

## Project Knowledge (not in other rules)

### Family Registration
Families registered dynamically by plugins via `PluginRegistry.Register()` — not a static list.
Validate format (contains "/", non-empty parts) — never enumerate all families.

### Config Pipeline
File → Tree → `ResolveBGPTree()` → `map[string]any` → `reactor.PeersFromTree()`.
Key files: `config/resolve.go`, `config/peers.go`, `reactor/config.go`.

### Bash Timeout
Default 15000ms. Longer only for `make ze-verify`, `make ze-unit-test`.

### Linter Hook
`auto_linter.sh` runs goimports on Edit/Write. Add import + usage in same edit to avoid cascading removals.
