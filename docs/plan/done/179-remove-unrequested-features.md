# Spec: delete-shit-claude-created

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/compatibility.md` - NO ExaBGP in engine

## Task

Remove shit Claude added without explicit user request. This includes:
1. YANG features that were never requested
2. ExaBGP syntax in engine (violates compatibility rule)

### Rule Violation

From `.claude/rules/compatibility.md`:
- **Engine code:** No ExaBGP format awareness, no compatibility shims
- **Config migration:** One-time conversion, not runtime compatibility

Claude violated this by keeping ExaBGP syntax in the engine instead of external tools.

## Phase 1: DONE - Removed YANG bloat

**Deleted:**
- `yang/ze-bgp.yang` (leftover file)
- `yang/ze-gr.yang` (leftover file)
- `peer-group`, `route-map`, `prefix-list` from schema handlers

## Phase 2: TODO - Remove ExaBGP syntax from engine

ExaBGP syntax that should NOT be in engine (should be in `ze bgp config migrate`):

| Syntax | Location | Issue |
|--------|----------|-------|
| `announce { }` | YANG + bgp.go | ExaBGP route format |
| `static { }` | migration code | Should convert to ZeBGP syntax, not announce |
| `operational { }` | YANG + bgp.go | ExaBGP-specific, "parsed but not processed" |
| `neighbor-changes` | YANG + bgp.go | ExaBGP API feature |
| `neighbor` keyword | migration | Old ExaBGP (migrated to peer) |

### What ZeBGP Native Syntax Should Be

Need to define ZeBGP's own route syntax (not ExaBGP's `announce`/`static`).

Options:
1. Define new ZeBGP syntax in separate spec
2. Keep migration tool but have it output ZeBGP syntax (not ExaBGP `announce`)

## Required Reading

- [x] `.claude/rules/compatibility.md` - NO ExaBGP in engine

## Phase 1 Summary: DONE

### Deleted
- Entire `yang/` folder (all files were leftovers):
  - `yang/ze-bgp.yang`
  - `yang/ze-gr.yang`
  - `yang/ze-plugin.yang`
  - `yang/ze-types.yang`

### Updated
- `cmd/ze/bgp/schema.go` - removed `bgp.peer-group`, `bgp.route-map`, `bgp.prefix-list`
- `docs/architecture/hub-architecture.md` - removed peer-group/route-map examples
- Tests updated to not reference removed handlers

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

## Phase 2: TODO - Separate spec needed

Removing ExaBGP syntax from engine requires:
1. Define ZeBGP native route syntax
2. Update migration to output ZeBGP syntax (not ExaBGP `announce`)
3. Remove ExaBGP parsing from engine

This is a larger task - needs its own spec.
