# 074 — Fix Test C

## Objective

Replace a flaky functional test (test C) that had a race between script teardown and test peer teardown with two focused tests: C1 (reconnection after NOTIFICATION) and C2 (teardown command).

## Decisions

- Chose to add a `send:raw:...` action to the test peer rather than using a separate signal mechanism: lets the peer act as a "job done" signal by sending a route to ZeBGP after reconnect, removing the timing dependency.
- Split into two tests with single concerns rather than fixing the original: two focused tests are more maintainable and provide clearer failure attribution.

## Patterns

- `groupMessages()` parse + `NextSendAction()` consume pattern for test peer: peer checks for pending send actions after each message match.

## Gotchas

- Root cause of test C flakiness was a bug: reactor queued teardown even when `p.session == nil`, causing a stale teardown on reconnect. Fix: guard teardown dispatch with nil check.
- Dual teardown mechanisms (script AND test peer disconnect) always race; design new tests to use only one mechanism.

## Files

- `internal/test/peer/peer.go`, `checker.go` — send:raw action support
- `test/data/api/reconnect.*`, `teardown-cmd.*` — new test files
