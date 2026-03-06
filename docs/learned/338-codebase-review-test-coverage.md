# 338 — Codebase Review: Test and Fuzz Coverage

## Objective

Close test coverage gaps found during full codebase review. Added 15 fuzz targets (12 → 27 total), reactor unit tests for 10 previously untested files, tests for 4 previously untestested packages, and functional tests for the route server plugin.

## Decisions

- Fuzz targets prioritized by attack surface: BGP message parsers (header, OPEN, UPDATE, NOTIFICATION, ROUTE-REFRESH), wire prefix parsing, route distinguisher, text protocol scanner, and config parsing
- Reactor tests focused on session handlers (OPEN/UPDATE/NOTIFICATION/KEEPALIVE processing), capability negotiation, UPDATE validation, peer connection init, peer send, wire operations, registry management, session flow, peer settings, and static routes
- Previously untested packages got dedicated test files: config/migration, plugin/cli, cmd/ze/plugin, cmd/ze/exabgp
- Route server got 3 functional tests: IPv4 withdrawal, IPv6 processing, backpressure — the largest plugin (6265 lines) previously had zero dedicated functional tests
- Added CLI help/error functional tests for plugin and exabgp commands

## Patterns

- Fuzz tests for wire parsers should accept `[]byte` and verify no panics — the goal is crash resistance, not semantic correctness
- Reactor unit tests require mock types for peers, sessions, and connections — the `mock_reactor_test.go` pattern is reusable across reactor test files
- Functional `.ci` tests for plugins follow the pattern: config → start peers → inject routes → verify output contains expected routes/withdrawals
- Packages in `cmd/` can be tested by calling their `Run(args)` function with crafted arguments and checking the exit code

## Gotchas

- The reactor package is heavily interconnected — testing `session_handlers.go` requires mocking the full session context (capabilities, peer state, wire buffers), not just the handler function signatures
- Config migration tests need sample ExaBGP config fixtures — the migration code handles multiple ExaBGP format versions, each needs a test case
- Route server functional tests must account for async route delivery — use `expect=stdout:contains=` with sufficient timeout rather than exact ordering

## Files

- `internal/component/bgp/message/fuzz_test.go` — 5 fuzz targets (header, OPEN, UPDATE, NOTIFICATION, ROUTE-REFRESH)
- `internal/component/bgp/wireu/prefix_fuzz_test.go` — prefix parsing fuzz
- `internal/component/bgp/nlri/rd_fuzz_test.go` — route distinguisher fuzz
- `internal/component/bgp/textparse/scanner_fuzz_test.go` — text protocol fuzz
- `internal/component/config/fuzz_test.go` — config parsing fuzz
- `internal/component/bgp/reactor/session_handlers_test.go` — session handler tests
- `internal/component/bgp/reactor/session_negotiate_test.go` — negotiation tests
- `internal/component/bgp/reactor/session_validate_test.go` — validation tests
- `internal/component/bgp/reactor/peer_connection_test.go` — peer connection tests
- `internal/component/bgp/reactor/peer_send_test.go` — peer send tests
- `internal/component/bgp/reactor/reactor_wire_test.go` — wire operation tests
- `internal/component/bgp/reactor/session_flow_test.go` — session flow tests
- `internal/component/bgp/reactor/peersettings_test.go` — settings tests
- `internal/component/bgp/reactor/peer_static_routes_test.go` — static route tests
- `internal/component/config/migration/migration_test.go` — migration tests
- `internal/component/plugin/cli/cli_test.go` — plugin CLI tests
- `cmd/ze/plugin/main_test.go` — plugin command tests
- `cmd/ze/exabgp/main_test.go` — exabgp command tests
- `test/plugin/rs-ipv4-withdrawal.ci` — route server functional test
- `test/plugin/rs-ipv6-processing.ci` — route server functional test
- `test/plugin/cli-plugin-help.ci` — CLI help functional test
- `test/plugin/cli-exabgp-help.ci` — CLI help functional test
- `test/plugin/cli-plugin-unknown.ci` — CLI error functional test
