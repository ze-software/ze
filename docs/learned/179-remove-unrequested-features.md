# 179 — Remove Unrequested Features

## Objective

Remove YANG schema bloat and ExaBGP syntax that Claude added without explicit user request, restoring compliance with the no-ExaBGP-in-engine compatibility rule.

## Decisions

- Phase 1 (YANG bloat): deleted entire leftover `yang/` folder and removed `peer-group`, `route-map`, `prefix-list` from schema handlers — none were requested.
- Phase 2 (ExaBGP engine syntax): deferred to a separate spec requiring its own design — removing `announce { }`, `static { }`, `operational { }` from the engine needs a defined Ze-native replacement first.

## Patterns

- When Claude adds unrequested features, a dedicated cleanup spec is the right approach — ensures systematic removal with proper testing.
- Compatibility rule enforcement: ExaBGP awareness belongs only in `ze bgp config migrate`, never in engine code.

## Gotchas

- Leftover YANG files from old structure (`yang/ze-bgp.yang`, `yang/ze-gr.yang`, `yang/ze-plugin.yang`, `yang/ze-types.yang`) existed alongside the correct location — always check for stale directories.

## Files

- `yang/` directory — deleted (all 4 files were leftovers)
- `cmd/ze/bgp/schema.go` — removed peer-group, route-map, prefix-list handlers
- `docs/architecture/hub-architecture.md` — removed peer-group/route-map examples
