# 109 — Plugin Stage Timeout Config

## Objective

Make the plugin startup stage timeout configurable per-plugin via a `timeout` keyword in the plugin config block (default: 5s, the previous hardcoded value).

## Decisions

- Timeout stored as `time.Duration`; zero means "use default 5s" — allows omitting the field to get default behaviour.
- Negative durations rejected at parse time — `time.ParseDuration("-5s")` succeeds but would cause immediate context expiration.
- Per-plugin timeout, no global override needed.

## Patterns

- Three `PluginConfig` structs exist: in `internal/component/config/bgp.go` (parsing), `internal/reactor/reactor.go` (reactor), and `internal/component/plugin/types.go` (plugin). All three must be updated when adding fields — they are the same concept split across package boundaries.

## Gotchas

- Negative duration edge case: `time.ParseDuration` accepts negative values. Without explicit validation, `-5s` would be set, creating a context that expires immediately on first stage transition. Added `TestPluginConfigTimeoutNegative` and validation guard.
- Multi-plugin timeout semantics: with different timeouts per plugin, fast plugins may hit their timeout while waiting for slow ones to sync. Documented in `docs/architecture/config/syntax.md`.

## Files

- `internal/component/config/bgp.go`, `internal/reactor/reactor.go`, `internal/component/plugin/types.go` — `StageTimeout time.Duration` added to `PluginConfig`
- `internal/component/plugin/server.go` — `stageTransition()` and "ready" handling use `proc.config.StageTimeout`
