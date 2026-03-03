# 180 — Native Update Syntax

## Objective

Add `update { attribute { } nlri { } }` config syntax as Ze's native replacement for ExaBGP's `announce { }` / `static { }` blocks, mirroring the API command vocabulary.

## Decisions

- No `add` keyword in config — config always announces; unlike the API which has explicit add/del.
- `attribute { }` block separates attrs from NLRI cleanly; multiple `update { }` blocks per peer allow different attr sets for different routes.
- Functional tests deferred — config parsing produces `StaticRouteConfig` which feeds existing pipeline already tested by `simple-v4.ci`, `simple-v6.ci`.
- `applyAttributesFromTree()` extracted as shared helper to eliminate duplication between new `extractRoutesFromUpdateBlock()` and existing `parseRouteConfig()`.
- VPN inline NLRI syntax (`ipv4/mpls-vpn rd 1:1 label 100 10.0.0.0/24`) not yet supported — workaround is `rd` and `label` in attribute block.

## Patterns

- Config syntax mirrors API but simplified (no action keywords) — reuse the same vocabulary so operators learn one set of terms.
- All attributes from the old `announce { }` / `static { }` syntax supported in `update { }` including path-information, labels, bgp-prefix-sid.

## Gotchas

- None.

## Files

- `internal/plugin/bgp/schema/ze-bgp.yang` — added `list update` with `attribute` and `nlri` containers
- `internal/component/config/bgp.go` — `extractRoutesFromUpdateBlock()`, `applyAttributesFromTree()` shared helper
- `internal/component/config/bgp_test.go` — 11 unit tests
