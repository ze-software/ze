# 108 — Config Capability Validation

## Objective

Add fail-fast config-time validation: if a peer enables `route-refresh` or `graceful-restart`, there must be at least one process binding with `send { update; }` configured. Catches misconfigurations before startup rather than silently failing at runtime.

## Decisions

- Validation runs AFTER template/match/inherit merging is complete — must see the fully resolved peer config, not the raw parse tree.
- `send { all; }` satisfies the requirement (it sets `Update = true`).
- Enhanced route-refresh implied by route-refresh — no separate check needed.

## Patterns

- Validation placed in `validateProcessCapabilities()` called from `LoadBGPConfig` after peer parsing completes.

## Gotchas

- `route-refresh true;` stored in `multiValues` (via `AppendValue`), but `Get()` only checks `values` map — silent ignore. Flag syntax `route-refresh;` works correctly. Tests updated to use flag syntax.
- `graceful-restart 120;` (value mode) not handled — only block mode `graceful-restart { restart-time 120; }` works. Tests updated to use block syntax.
- Three existing tests and one functional test config (`test/data/encode/vpn.conf`) used route-refresh without a process binding — needed updating.

## Files

- `internal/component/config/bgp.go` — `validateProcessCapabilities()` function at line 833-874
