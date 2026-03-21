# 065 — Remove Version Numbers from Code

## Objective

Remove all version number references (v6, v7, v2, v3, "legacy", "current") from the codebase — API version constants, config fields, and migration comments — leaving exactly one output format.

## Decisions

- Mechanical refactor, no design decisions.
- `ErrV2Config` renamed to `ErrOldConfig` (only rename that required changing public symbol name)
- Legitimate remaining references: OSPFv2/v3 (protocol names), `conf-srv6-mup-v3` (MUP protocol version), variable names `v2`/`v3` in attribute tests — all preserved

## Patterns

- None.

## Gotchas

- None.

## Files

- `internal/component/plugin/types.go` — deleted `APIVersionLegacy`, `APIVersionNLRI`, `ContentConfig.Version`
- `internal/component/plugin/text.go` — removed version comparison branches, kept one format
- `internal/reactor/peersettings.go` — deleted `Version int`
- `cmd/ze/bgp/config_fmt.go` — `ErrV2Config` → `ErrOldConfig`
- Multiple test files and docs — version terminology removed from comments
