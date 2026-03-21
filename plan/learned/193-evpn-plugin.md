# 193 — EVPN Plugin

## Objective

Create an EVPN family plugin and resolve the import cycle caused by `nlri/evpn.go` re-exporting EVPN types.

## Decisions

- Delete `nlri/evpn.go` re-export layer entirely rather than creating an import workaround — re-exports exist only for convenience, not because `nlri` owns EVPN types.
- EVPN types move to the plugin package; consumers updated to import evpn directly.
- `RouteDistinguisher` stays in `nlri` — it is shared across EVPN, VPN, BGP-LS, and other families.

## Patterns

- Family plugins own their types. `nlri` owns shared types used by multiple families (Family, RouteDistinguisher). Never create re-export shims — update importers instead.
- CLI uses `--json`/`--text` flags (format is explicit in the flag name) for standalone tools. Engine-side decode uses `--decode` (bool) because format is determined by the protocol message, not the caller.

## Gotchas

- Import cycle `nlri → evpn → nlri` was introduced by placing EVPN types in `nlri` and then importing `nlri` from the plugin. The cycle was latent until the plugin tried to import `nlri`. Remove re-exports at the point of cycle discovery, not retroactively.

## Files

- `internal/component/bgp/plugins/nlri/evpn/` — EVPN plugin with types
- `internal/bgp/nlri/` — RouteDistinguisher stays; nlri/evpn.go deleted
