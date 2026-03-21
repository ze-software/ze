# 288 — Clock and Network Injection (ze-sim)

## Objective

Replace all direct `time.*` and `net.*` calls in the reactor and FSM with injectable interfaces (`Clock`, `Dialer`, `ListenerFactory`) so that chaos testing and deterministic simulation can inject virtual time and mock networks without modifying production behavior.

## Decisions

- Setter injection (`SetClock` / `SetDialer` / `SetListenerFactory`) over constructor changes — `NewSession(settings)` is called 34+ times in tests; changing constructors would require modifying every call site.
- `ListenerFactory.Listen` takes `context.Context` (unlike `net.Listen`) — better cancellation API, improvement over the stdlib signature.
- `FakeClock`, `FakeDialer`, `FakeListenerFactory` added to `internal/sim/fake.go` — closes integration completeness gap; integration smoke tests in `recent_cache_test.go` prove the injection path works end-to-end.
- AC-5 through AC-8 (FSM timer mock tests, Session/Listener integration) deferred to chaos Phase 9 — these require more complex setup (VirtualClock advancement triggering timer callbacks); the infrastructure is in place.
- Architecture docs update deferred — will add when chaos Phase 9 uses the injection in practice.

## Patterns

- `var _ sim.Clock = sim.RealClock{}` compile-time interface conformance check — prevents goimports from removing the import in files where cross-file typecheck errors confuse the linter tool.
- Grep-based audit test (`TestNoDirectTimeCalls`, `TestNoDirectNetCalls`) in `sim/audit_test.go` — enforces no regression to direct calls.
- Lazy `sync.Once` start (`startReader`) pattern: don't activate the goroutine in the constructor; activate on first use. Used here for `SetClock` wiring propagation from reactor down to children.

## Gotchas

- goimports silently removes new imports before usage is added in a separate edit — must combine import declaration + struct field + constructor + setter in a single edit.
- `sim.Timer` is an interface; interfaces cannot have fields. After replacing `time.NewTimer` with `clock.NewTimer`, channel access changes from `timer.C` (field) to `timer.C()` (method).
- `_ = conn.Close()` pattern is blocked by `block-ignored-errors.sh` hook — use a `closeOrLog(t, c)` helper instead.

## Files

- `internal/sim/clock.go` — Clock, Timer, RealClock interfaces + implementations
- `internal/sim/network.go` — Dialer, ListenerFactory, RealDialer, RealListenerFactory
- `internal/sim/fake.go` — FakeClock, FakeDialer, FakeListenerFactory for testing
- `internal/sim/audit_test.go` — grep-based enforcement
- Modified: `fsm/timer.go`, `reactor/session.go`, `reactor/listener.go`, `reactor/peer.go`, `reactor/api_sync.go`, `reactor/recent_cache.go`, `reactor/reactor.go`
