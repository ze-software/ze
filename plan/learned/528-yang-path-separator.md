# 528 -- YANG Path Separator

## Context

Two separator conventions coexisted in the codebase: dot (`.`) for handler/schema/validator paths
(`bgp.peer.timer`), and slash (`/`) for YANG extensions (`ze:required`) and web/CLI context paths.
This inconsistency meant consumers had to know which convention applied where. The goal was to
unify on `/` everywhere, aligning with the YANG extension convention already established.

## Decisions

- Chose centralized helpers (`JoinPath`, `AppendPath`, `SplitPath` in `config/path.go`) over
  inline replacement, so the separator is defined once and consumers express intent not mechanism.
- `yang/validator.go` uses inline `/` instead of the helpers because `config` imports `yang`,
  creating an import cycle if `yang` imports `config` back. Accepted this as the only exception.
- Environment variable paths (`ze.bgp.X.Y`) and slogutil subsystem paths (`bgp.wire`,
  `bgp.reactor`) left as dots -- these use dot as env var naming convention, not YANG paths.

## Consequences

- Any new config path construction should use `config.JoinPath`/`AppendPath`/`SplitPath`.
- The `yang` sub-package is the one place that must use `/` directly due to the import cycle.
- Handler registration strings (e.g., in plugin `SetSchema` calls) now use `/`: `"bgp/peer"`.
- Hub event wildcard patterns changed from `bgp.*` to `bgp/*`.
- The `parentRemoved` function in `startup_autoload.go` checks for `/` not `.` when walking paths.

## Gotchas

- The spec initially listed ~10 files. Actual scope was 50+ production files, 16 test files,
  22 `.et` files. Handler path literals (`"bgp.peer"`) were scattered across plugin registrations,
  schema lookups, validator calls, hub routing, and SDK examples -- far beyond the config layer.
- Mechanical sed replacements only fixed the first dot after a prefix (`bgp/peer.timer` instead
  of `bgp/peer/timer`). Had to do multiple passes to catch partially-fixed paths.
- `parentRemoved()` and `yangLeafName()` had hardcoded `'.'` character comparisons that grep for
  `"."` string patterns wouldn't find. Searching for the character literal caught them.
- `hasConfiguredPlugin` had a latent nil dereference (`s.config` nil in test setup) that only
  surfaced after the path change altered which code paths were reached during reload tests.
- Cumulative leaf-list paths in `resolve.go` (`"filter.ingress.community.tag"`) are config paths
  too, not just handler paths -- easy to miss because they look like data structure navigation.

## Files

- New: `internal/component/config/path.go`, `path_test.go`
- Config layer: `reader.go`, `schema.go`, `diff.go`, `tree.go`, `parser_freeform.go`,
  `environment.go`, `schema_defaults.go`, `yang_schema.go`, `yang/validator.go`
- BGP config: `resolve.go`, `peers.go`, `plugins.go`
- Hub: `router.go`, `schema.go`
- Plugin server: `hub.go`, `schema.go`, `reload.go`, `server.go`, `startup_autoload.go`, `node_with.go`
- Web/CLI: `web/cli.go`, `cli/validator.go`, `cli/testing/expect.go`, `web/fragment.go`
- SDK: `pkg/plugin/plugin.go`
- Commands: `cmd/ze/schema/main.go`, `cmd/ze/config/cmd_migrate.go`
- BGP plugins: `cmd/peer/peer.go`, `cmd/update/update_text.go`
- Test runner: `internal/test/runner/json.go`
- 22 `.et` editor test files, 16 `_test.go` files
