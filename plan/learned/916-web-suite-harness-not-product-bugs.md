# web-suite-harness-not-product-bugs

## Context

`known-failures.md` listed the web suite as "81/81 FAIL -- systemic: web
server startup". Because the suite was uniformly failing, nothing in it had
ever run green, so no one knew which of the 81 tests were individually sound.

## What was actually true

One harness root cause hid everything else. The runner launched
`ze start --web <port> --insecure-web` against an empty temp config store; the
full `--web` daemon needs a loaded config, so it printed `config "ze.conf" has
unknown type` and exited before binding the port. Every test then timed out at
the readiness probe (30.1s). Fixing that one launch revealed the suite was
~76/81, with the residual being a mix of render-race flakiness and four
genuine test bugs the all-failing harness had masked.

## Fixes

Harness (`internal/test/cli/cmd_web.go`,
`internal/component/web/testing/runner.go`):

1. **`--web-only`.** Web UI functional tests do not need a daemon: RunWebOnly
   serves the whole UI surface (config editor, nav, fragments, SSE log stream)
   with no config. It is the mode the daemon's own error hint recommends.
2. **HTTP readiness, not TCP.** The probe did a bare TCP connect, which
   succeeds the instant the listener binds -- before routes mount -- so a
   browser could hit an empty page. Changed to an HTTPS GET (any status proves
   the mux serves).
3. **Auto-waiting assertions.** `checkExpectation` took one point-in-time
   snapshot; under load an HTMX/JS render lands a few hundred ms after
   WaitLoad reports idle, caught as "(empty page)". Wrapped in a bounded retry
   (5s, 250ms poll) -- the Playwright/Cypress auto-wait pattern.
4. **Per-test session close.** Sessions are keyed per test (unique nick) but
   were only swept at suite end, so 80+ live browser pages accumulated in the
   shared agent-browser daemon and starved late tests. Added `defer
   browser.Close()` per test.
5. **Seed a zefs admin** into the temp store so `/show/users/` lists the
   always-on "(system)" power user (`usersFromZefsDB`); the empty store
   otherwise renders "No users".

Four genuine test bugs (authorized `.wb` edits, `ai/rules/testing.md`):
`scenario-interface-setup` and `interface-configured-display` filled a
`field-mac-address` that the **key-only** add overlay never renders
(mac-address is edited on the entry detail page); `logs-live-stream` asserted
the transient "Connecting" that `log-live.js` replaces with "Connected" on SSE
open; `system-users-power` needed the seeded admin.

## Lesson

- A uniformly-failing suite is almost always ONE harness root cause, not N
  product/test bugs. Same trap as [[911-exabgp-flaky-eor-race-not-encoding-bugs]],
  now confirmed in a second domain. Fix the harness first; only then is the
  per-test triage meaningful.
- A test readiness probe must verify what the client needs (a real HTTP
  response), not a cheaper proxy (TCP accept). The proxy passes too early.
- Browser-test assertions are point-in-time reads; they need bounded
  auto-waiting or they flake on render timing, especially under parallel load.
- A shared browser daemon needs per-test cleanup, or late tests degrade as
  pages leak.
- These suites are performance-sensitive: under heavy host load (this box sat
  at load avg ~7 from unrelated apps) a rotating handful flake per full run
  even though each passes in isolation. Verify a suspect test individually
  before attributing a failure to the change; do not chase a clean full-suite
  number on a loaded machine.

## Left open (separate, pre-existing)

- Residual full-suite flakiness is host-load-sensitive render races, not
  product or harness defects; expected reliable on a quiet CI host.
- `make ze-lint-changed` currently fails on a non-compiling
  `internal/component/bgp/config` (`undefined: familyIPv4SRPolicy`, SR-policy
  work in flight in another session) -- unrelated to the web suite.

## Files

None recorded.
