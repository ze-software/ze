# sleep-test-determinism

Converted the fixed-`time.Sleep` test backlog (the second category in
`feedback_periodic_test_sweep`) to deterministic synchronization across ~13
packages, and removed the test-only production surface the first attempts added.

## Conversion taxonomy (pick by what the test waits for)

- **Async goroutine effect** (hooks, manager start, BMP session): sync on the
  real observable. Prefer a poll-until-condition on the side effect (marker
  file, metric, connection close) or the existing signal (SDK `OnStarted`,
  `Stop()`'s `<-done`). `require.Eventually` is the established idiom (38 files).
- **Timer / TTL behavior** (DNS cache, DHCP lease, session-health stuck/EOR,
  telemetry tick): inject `clock.Clock` (field defaulting to `clock.RealClock{}`,
  production path byte-identical) and advance a fake clock. Never `time.Now()`
  directly in a package that takes a clock (trap in `RECURRING-PATTERNS.md`).
- **Event ordering** (exabgp bridge): expose a non-destructive introspector and
  `require.Eventually` on it; not a fixed pre-signal delay.

## No test-only production surface (the "production ignores it" smell)

A field/return/branch that only tests consume is a smell, even when it makes
tests deterministic. Removed: `storage.Manager.ready` (Stop already
synchronizes), `collector.tickDone` (observe via the test's own collector),
`healthcheck.runHooks`'s returned `*sync.WaitGroup` (tests effect-poll the
marker). Keep such a member ONLY when there is genuinely no observable
alternative, and then document it as test-only introspection
(`bridge.pendingResponses.isWaiting`). Dependency injection (a clock used on the
real path) is NOT this smell.

## sim.FakeClock now fires AfterFunc

`internal/test/sim.FakeClock.AfterFunc` was inert; it now schedules callbacks
that fire in deadline order on `Add()`/`Set()` (lock released before each
callback). `session_health`/`dhcp` tests migrated off `chaos.VirtualClock` onto
it, dropping the `internal/chaos` import from unit tests. `After`/`NewTimer`/
tickers stay manual to avoid changing the 5 existing reactor `FakeClock` tests.
Follow-up: collapse `chaos.VirtualClock` onto `sim.FakeClock` (it has ~10
non-test callers and would need firing `NewTimer`/tickers too).

## Why this matters

Replacing sleeps with real sync exposes latent bugs: `session_health`'s test
`fakeClock` embedded `clock.RealClock` and overrode only `Now()`, so its
`AfterFunc` timers fired in real wall-clock time. The conversion surfaced and
fixed it. See `feedback_sleep_hides_races`.

Scope note: `exabgp/bridge` sleep conversions were left uncommitted because
`bridge_test.go` was simultaneously edited by concurrent SR-Policy work and
could not be cleanly separated with `git add` (whole-file staging).

## Files

None recorded.
