# 215 — YANG IPC Cleanup

## Objective

Use YANG-driven validation for API text commands, replacing ad-hoc string checks with schema-enforced enumeration and type validation.

## Decisions

- `origin` field changed from `type string` to a YANG enumeration (`igp`, `egp`, `incomplete`) — string validation was previously manual and error-prone.
- `ValueValidator` interface placed at the consumer site (like `SetLogger`) so callers can inject validation without importing the yang package directly.
- Family validation is format-only by design (checks for "/" separator and non-empty parts) — families are registered dynamically by plugins, so a static enumeration would be wrong.
- Engine wiring added in `LoadReactorFileWithPlugins` to connect the YANG validator to the API command parser.

## Patterns

- Consumer-site interface for injectable validators matches the SetLogger pattern used throughout Ze.
- Format-only validation for extensible registries (families, capabilities) is correct — do not validate against a static list when the list is dynamic.

## Gotchas

- `validateDecodeHex` was fully implemented but never called in any production path — discovered during audit and removed as dead code. Dead code that looks like production code is a maintenance hazard.

## Files

- `internal/component/config/yang/validator.go` — YANG validator with ValueValidator interface
- `internal/plugins/bgp/reactor/` — engine wiring in LoadReactorFileWithPlugins
