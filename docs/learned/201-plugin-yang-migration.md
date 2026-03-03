# 201 — Plugin YANG Migration

## Objective

Move the `graceful-restart` YANG schema from the core `ze-bgp.yang` into the GR plugin's own YANG, using `declare wants config bgp` to receive the relevant config subtree.

## Decisions

- Internal plugin YANG is always loaded (not opt-in) — eliminates the chicken-and-egg problem where the engine cannot read plugin YANG without loading the plugin, which requires reading its YANG first.
- GR plugin YANG requires 4 augment paths to cover all template combinations — fewer paths leave some template contexts unreachable.

## Patterns

- Internal plugins always have their YANG schemas merged at engine startup, regardless of whether the plugin is active in the current config.

## Gotchas

- If internal plugin YANG were opt-in, the engine could not parse configs that use the plugin's schema before the plugin is loaded. Always-load avoids this circular dependency.
- Missing augment path: each template variant (peer, group, global) needs its own augment or config under that scope is unreachable.

## Files

- `internal/plugins/bgp-gr/` — GR plugin with YANG schema
- `internal/plugins/bgp/subsystem.go` — always-load internal plugin YANG
- `internal/component/config/yang/` — YANG merge at startup
