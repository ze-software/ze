# 355 — Test Coverage — Reactor, CLI, Config, Functional

## Objective

Add ~4,300 lines of test code across 21 files covering reactor pure functions, config migration, session handlers, CLI packages, peer send/connection, registry, and functional tests.

## Decisions

- Phases 1-6 (unit tests) completed first — highest value/effort ratio
- Reactor wire tests use hex comparison for wire byte verification
- Session handler tests use `net.Pipe()` with raw BGP message byte slices
- CLI tests call `Run(args)` and check exit codes with mock registry for plugin dispatch
- RS multi-peer propagation tests (Phase 7) and LLNH/CLI functional tests (Phase 8) remain as gaps

## Patterns

- Table-driven tests with `VALIDATES:`/`PREVENTS:` comments throughout
- Session mock pattern: `NewSession(settings)` + `net.Pipe()` + craft raw messages
- Functional `.ci` pattern: `stdin=peer` for ze-peer expectations, `stdin=ze-bgp` for config, `cmd=background` for peers, `cmd=foreground` for ze
- CLI test pattern: `Run([]string{...})` → check exit code, mock registry for dispatch

## Gotchas

- `reactor_api.go` (858L), `reactor_api_batch.go` (610L) intentionally deferred — require full reactor+RIB+plugin infrastructure, already exercised by functional tests
- `cmd/ze-test/` not tested — it IS the test infrastructure; low value testing the test runner
- `peer_initial_sync.go`, `peer_rib_routes.go` deeply integrated with RIB plugin, tested via `rib-reconnect.ci`

## Files

- `internal/component/bgp/reactor/reactor_wire_test.go` — 16+ attribute writer tests
- `internal/component/bgp/reactor/peersettings_test.go` — connection mode, peer key, IBGP/EBGP
- `internal/component/bgp/reactor/session_flow_test.go` — pause/resume idempotency
- `internal/component/bgp/reactor/session_handlers_test.go` — handleOpen, handleKeepalive, etc.
- `internal/component/bgp/reactor/session_negotiate_test.go` — family intersection, ASN4, hold time
- `internal/component/bgp/reactor/session_validate_test.go` — RFC 7606 enforcement
- `internal/component/bgp/reactor/peer_send_test.go` — send with no session, wire bytes
- `internal/component/bgp/reactor/peer_connection_test.go` — collision resolution, pending connections
- `internal/component/bgp/reactor/peer_static_routes_test.go` — static route building
- `internal/component/config/migration/migration_test.go` — config transformation pipeline
- `internal/component/plugin/cli/cli_test.go` — RunPlugin flags, hex input, env FD
- `cmd/ze/plugin/main_test.go` — plugin dispatch, help, unknown plugin
- `cmd/ze/exabgp/main_test.go` — subcommands, migrate, invalid flags
- `test/plugin/cli-plugin-help.ci`, `cli-plugin-unknown.ci`, `cli-exabgp-help.ci` — CLI functional
- `test/plugin/rs-backpressure.ci`, `rs-ipv4-withdrawal.ci`, `rs-ipv6-processing.ci` — RS functional
