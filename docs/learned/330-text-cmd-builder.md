# 330 — Text Command Builder Consolidation

## Objective

Eliminate duplicate `cmdBuilder` in `bgp-watchdog` by consolidating both text command builders into `shared.FormatAnnounceCommand`/`FormatWithdrawCommand`, extending `shared.Route` with `RD` and `Labels` fields for VPN support.

## Decisions

- Use long-form keywords (`local-preference`, `community`) not short-form (`pref`, `s-com`): engine's `ResolveAlias` accepts both, long-form is canonical and self-documenting
- Keep `FormatRouteCommand` as a deprecated alias rather than breaking rename — package-level alias in `bgp-rib/event.go` handles indirection
- Keep `watchdogRouteKey` standalone: pool key format (`prefix#pathID`) differs from shared format (`family:prefix[:pathID]`)

## Patterns

- Text parser `ResolveAlias` normalises short→long keywords, so keyword choice has zero wire-level impact
- `shared` package must remain a leaf — no imports from plugin packages; safe for cross-plugin use because of this constraint

## Gotchas

None.

## Files

- `internal/plugin/bgp/shared/format.go`, `format_test.go`, `route.go`
- `internal/plugins/bgp-rib/event.go`
- `internal/plugins/bgp-watchdog/config.go`, `config_test.go`, `server.go`
- `internal/plugins/bgp-watchdog/command.go` — deleted
- `internal/plugins/bgp-watchdog/command_test.go` — deleted
