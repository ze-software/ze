# 333 — Move shared BGP Types to component/bgp

## Objective

Relocate `internal/component/bgp/shared/` to `internal/component/bgp/` — BGP is a subsystem, not plugin infrastructure, and the package name "shared" describes relationship rather than content.

## Decisions

- Renamed package from `shared` to `bgp` — package names should describe what they provide, not their relationship to consumers.
- Deleted deprecated `FormatRouteCommand()` wrapper at the same time — ze is unreleased, no compat needed.
- `format.go` still imports `internal/component/bgp/attribute` for `FormatASPath()` — this residual coupling resolves when the full BGP subsystem migrates to `internal/component/bgp/` in arch-0.

## Patterns

- `internal/component/` is the correct layer for subsystem domain types consumed by plugins — not `internal/component/plugin/`.
- Subsystem ≠ Plugin (from CLAUDE.md): BGP domain types belong at the component layer even though plugins consume them.
- Mechanical package move: rename package declaration, update import paths in 4 consumers, delete old directory.

## Gotchas

None. Clean migration with no surprises.

## Files

- `internal/component/bgp/` — route.go, event.go, format.go, nlri.go (new location, package `bgp`)
- `internal/component/bgp/shared/` — deleted entirely
- `internal/component/bgp/plugins/rib/event.go`, `bgp-adj-rib-in/rib.go`, `bgp-watchdog/config.go` — import path updated
