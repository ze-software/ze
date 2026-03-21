# 225 — Config Reload 5: End-to-End Tests

## Objective

Write functional tests proving the complete reload path works from SIGHUP through coordinator to plugin delivery.

## Decisions

- SIGHUP reload in the functional test daemon uses the direct `LoadReactorFromFile()` path, NOT the coordinator path — the coordinator is not wired into the functional test daemon. Multi-plugin and config-delivery tests were skipped because of this gap.
- `Session.Run()` had a TCP connection leak on context cancel: `closeConn()` was missing from the cancellation path. Fixed as a prerequisite to reliable functional tests.

## Patterns

- Functional test daemon wiring must match production wiring — gaps between the two produce tests that pass in CI but mask production bugs. Document every known difference.

## Gotchas

- The functional test daemon and the production daemon have different coordinator wiring. Tests exercising SIGHUP only test the direct-reload path, not the two-phase coordinator. This was a known limitation captured explicitly.
- TCP connection leak in `Session.Run()` was only discovered when functional tests started exercising the cancellation path — unit tests with mocks cannot catch resource leaks.

## Files

- `test/` — functional test cases for config reload
- `internal/component/bgp/reactor/server.go` — Session.Run() connection leak fix
