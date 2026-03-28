# 451 — rib-show-filters

## Objective

Add community and AS-path regex filters to `rib show in` and `rib show best` commands, enabling VyOS-style route filtering: `rib show in community 65000:100` and `rib show in regexp "64501 64502"`.

## Decisions

- **Keyword-value parsing:** `parseShowFilters` recognizes `community <value>` and `regexp <pattern>` as keyword-value pairs. Positional args (family, prefix) use heuristics (contains "/", starts with letter vs digit).
- **Error on unknown args:** Ze's fail-on-unknown rule applied — `parseShowFilters` returns error for unrecognized arguments, not silent ignore.
- **Filter during iteration:** Community/regexp checks applied inside the existing `PeerRIB.Iterate()` callback — no extra pass, no copies.
- **Best-path filters AFTER selection:** `bestPathShowJSON` runs `SelectBest()` first, then applies community/regexp filters on the best route's attributes. This filters output, not candidate set.
- **Extracted to separate file:** Filter parsing and matching moved to `rib_show_filter.go` for modularity.

## Patterns

- **Pool access for filtering:** `entry.HasX() → pool.X.Get(handle) → format(data) → match` — same pool access pattern used by attribute enrichment.
- **dispatch-command for `.ci` wiring tests:** Python test plugins can call `ze-plugin-engine:dispatch-command` to exercise any command path end-to-end. Plugin stderr is consumed by the engine's relay goroutine (not visible to test runner), so use dispatch-command + daemon shutdown for pass/fail signaling.
- **Tokenizer handles quoted args:** The command dispatcher's `tokenize()` supports double-quoted strings — important for regexp patterns with spaces.

## Gotchas

- **Plugin stderr not testable:** Plugin subprocess stderr is consumed by `relayStderrFrom()` inside the daemon — never reaches the test runner's `expect=stderr:contains=`. Cannot use stderr for test assertions.
- **Hook blocked silent ignore:** Initial `switch/default: i++` pattern silently swallowed unknown args. Hook `block-silent-ignore.sh` caught this — restructured to if/else chain with explicit error.
- **`slices.Contains` required:** Linter flagged manual loop over formatted communities. Use `slices.Contains(formatCommunities(data), community)`.
- **`bestPathShowJSON` filter bypass:** When `peerRIB` was nil for the best peer, the `if peerRIB != nil` block was skipped entirely, falling through to `append` without applying community/regexp filters. Fix: flatten control flow — nil peerRIB with active filters → skip; nil peerRIB without filters → include bare; non-nil → apply filters. Nested `if` blocks that skip filter checks on the nil path are a trap.
- **API schemas not in `ze schema show`:** API (`-api`) YANG modules are only indexed for `ze schema methods/events`, not `ze schema show/list` (which only has `-conf` modules). `.ci` tests for API schemas must use `methods`, not `show`.
- **Dangling keyword misleading error:** Combined condition `arg == "community" && i+1 < len(args)` falls through to "unrecognized" when value is missing. Fix: split into keyword recognition then value check, giving "community requires a value" instead.

## Files

- `internal/component/bgp/plugins/rib/rib_show_filter.go` — filter parsing and matching (extracted)
- `internal/component/bgp/plugins/rib/rib_commands.go` — inboundShowJSON, bestPathShowJSON apply filters
- `internal/component/bgp/plugins/rib/rib_commands_test.go` — 9 new filter tests
- `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` — community/regexp input leaves
- `test/plugin/rib-show-filter.ci` — dispatch-command wiring test
