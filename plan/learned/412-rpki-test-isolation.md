# 412 -- RPKI Test Isolation

## Objective

Fix intermittent rpki-decorator functional test failures (tests 132-135) when run as part of the full plugin suite.

## Decisions

- Fix `hasConfiguredPlugin()` to also check the `Run` command field, not just config name
- Increase test 135 timeout from 20s to 30s (startup overhead under concurrent load)

## Patterns

- Plugin config names (user-facing, e.g., `adj-rib-in`) can differ from registry names (internal, e.g., `bgp-adj-rib-in`). Any code that compares the two must account for this.
- `hasConfiguredPlugin` is the gatekeeper for auto-loading. If it doesn't recognize a plugin as configured, the auto-loader launches a duplicate internal instance.

## Gotchas

- **Root cause was name mismatch, not timing**: config uses short names (`adj-rib-in`), registry uses full names (`bgp-adj-rib-in`). `hasConfiguredPlugin()` did exact match only. Auto-loader thought the plugin wasn't configured and launched a second instance. Both tried to register the same commands.
- **Registration conflicts appeared intermittent because**: which instance registered first was a race. Sometimes the explicit one won, sometimes the auto-loaded one. The loser got the conflict error.
- **Secondary issue**: test 135 timeout was tight (20s for a test that sleeps 8s + reads 10s + startup). Increased to 30s.
- **Empty string guard**: `strings.Contains(run, "")` is always true. Must guard `hasConfiguredPlugin("")` early return.

## Files

- `internal/component/plugin/server/server.go` -- `hasConfiguredPlugin()` now checks Run field
- `test/plugin/rpki-decorator-timeout.ci` -- timeout 20s to 30s
