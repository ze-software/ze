# 402 -- RPKI Validation Gate

## Objective

Extend bgp-adj-rib-in with a "pending" route state and accept/reject commands, so a validation plugin can gate route installation without coupling the RIB to any specific validator.

## Decisions

- **Generic coordination primitive:** The validation gate is not RPKI-specific. Any future validator plugin could use enable-validation/accept/reject commands.
- **Zero overhead when disabled:** A single boolean check (`validationEnabled`) gates the pending path. When no validator loads, routes stored immediately as before.
- **Pending routes in separate tracking:** Pending routes tracked separately from installed routes. On accept, moved to installed. On reject, discarded.
- **Timeout sweep goroutine:** Background goroutine scans pending routes every 5s. Routes pending longer than timeout (default 30s) promoted to Installed with state=NotValidated (fail-open).
- **Five validation states:** NotValidated(0), Valid(1), NotFound(2), Invalid(3), Pending(4). Pending is internal to adj-rib-in only.

## Patterns

- **Command-based inter-plugin coordination:** Same pattern as GR retain/release. Validator sends text commands, adj-rib-in handles them. No shared types or imports.
- **State transitions are idempotent:** accept-routes on already-installed route returns error, no state change. reject-routes on non-pending route returns error.

## Gotchas

- The validation gate must handle the case where bgp-rpki crashes mid-validation. The timeout sweep is the safety net -- without it, routes would be stuck as pending forever.
- Multiple pending routes for same prefix (with different path-IDs via ADD-PATH) are resolved independently.

## Files

- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` -- pending route map, timeout scanner, validation states
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` -- enable-validation, accept-routes, reject-routes command handlers
- `test/plugin/rpki-passthrough.ci` -- routes flow through unchanged without rpki loaded
- `test/plugin/rpki-timeout.ci` -- fail-open timeout behavior
