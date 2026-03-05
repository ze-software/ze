# 361 — Config BGP Separation

## Objective

Move BGP-specific config code from generic `internal/component/config/` to `internal/component/bgp/config/`, making the config package content-agnostic. Also remove dead environment fields deferred from YANG reorganisation (learned 334).

## Decisions

- 12 source files + 9 test files moved via git mv with package declaration and import path updates
- Generic files (loader.go, environment.go, provider.go, tree.go, etc.) stay in config/
- Subdirectories (editor/, yang/, migration/, env/) stay in config/ — they couple through config, not BGP directly
- Dead YANG leaves (tcp.delay, tcp.acl, reactor.speed, reactor.cache-ttl) and containers (bgp, cache, api) removed
- Corresponding environment.go struct fields and envOptions entries removed

## Patterns

- Pure file relocation — no logic changes, only import paths updated
- loader.go in generic config imports bgp/config for BGP extraction calls, maintaining unidirectional dependency
- Zero BGP subsystem imports remain in generic config package (verified via grep)

## Gotchas

- serialize.go in generic config needed updating to reference BGP types from the new package location
- Editor subdirectory references to reactor/capability are indirect (through config types), not direct BGP imports — these stay in generic config

## Files

- `internal/component/bgp/config/` — 25+ files (bgp.go, peers.go, bgp_routes.go, routeattr*.go, plugins.go, validators*.go, loader_routes.go, loader_prefix.go, and tests)
- `internal/component/config/loader.go` — updated imports to reference bgp/config
- `internal/component/bgp/schema/ze-bgp-conf.yang` — dead fields removed
- `internal/component/config/environment.go` — dead struct fields removed
