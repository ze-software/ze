# 136 — RIB Plugin Functional Test

## Objective

Create functional test coverage for the RIB plugin. The original plan (standalone Go test program) was rejected in favor of existing test patterns.

## Decisions

- Rejected standalone Go test program (`test/cmd/rib-functional/`) because: wrong doc path, wrong command name (`inbound clear` vs `inbound empty`), wrong JSON event formats, and architecture conflict with `test/plugin/` patterns. Existing unit and functional tests already covered the scenarios.
- Added `TestHandleRequest_RIBAdjacentOutboundResend_DownPeer` to unit tests and `rib-withdrawal.ci` to functional tests instead.

## Patterns

None beyond: always verify command names and JSON formats against the actual implementation before writing test harnesses.

## Gotchas

- The spec referenced wrong doc filename case, wrong command name, wrong JSON formats. Reading the actual source and running the binary would have caught all of these before writing the harness.

## Files

- `internal/plugin/rib/rib_test.go` — added `TestHandleRequest_RIBAdjacentOutboundResend_DownPeer`
- `test/plugin/rib-withdrawal.ci` — verifies withdrawn routes not replayed on reconnect
