# 617 -- iface-vpp-ready-gate

## Context

`spec-vpp-4-iface` (learned/615) shipped the vpp `iface.Backend`
implementation. The backend loads cleanly, but its first call --
`applyConfig`'s `ListInterfaces()` at Phase 3 -- fires while the `vpp`
component is still inside its initial GoVPP handshake. `ensureChannel`
returned a generic `"govpp: not connected"` error which `applyConfig`
recorded via `record("list interfaces for reconciliation", ...)`.
Result: every startup under the vpp backend logged
`ERROR deliverConfigRPC failed ... list interfaces for reconciliation`
and reconciliation never re-ran once VPP actually came up, so stale
addresses on running VPP interfaces would never be cleaned.

This spec closed that gap: defer reconciliation silently when the
backend reports "not ready", and re-run it when
`vppevents.EventConnected` / `EventReconnected` fires. Netlink path
unchanged.

## Decisions

- **Typed sentinel instead of string matching.** `iface.ErrBackendNotReady`
  is a package-level `errors.New` value that `ifacevpp.ensureChannel`
  wraps via `%w`. `applyConfig` branches on `errors.Is(err,
  ErrBackendNotReady)`. Chose this over matching the `"VPP connector not
  available"` string because the error text is an implementation detail of
  `ensureChannel` that should be free to change. The sentinel is the
  explicit, testable control-flow signal.
- **Gate at `IsConnected()`, not `NewChannel()`.** `vpp.Connector`
  exposes both. The original `ensureChannel` called `NewChannel()` and
  relied on it returning `"govpp: not connected"`. But
  `setActiveConnector(m.connector)` in `vpp.go:147` runs *before*
  `runOnce` calls `Connect()` -- so `GetActiveConnector() != nil` is
  true before the handshake completes, and `NewChannel()` is the only
  thing that detects the pre-handshake state. Wrapping that error text
  is brittle; checking `connector.IsConnected()` up front and
  synthesizing the sentinel is robust. It also means no channel is
  opened and discarded when the backend is not ready, which avoids
  leaking pre-handshake state.
- **Don't cache the sentinel in `chErr`.** The original `ensureChannel`
  set `b.chErr` on failure and returned it on every subsequent call.
  That is wrong for the "not ready" case: the next call (triggered by
  `EventConnected`) MUST retry, not re-return a stale "not ready"
  verdict. Added an explicit non-caching branch for the sentinel path;
  `ensureChannel` stays non-caching for
  `ErrBackendNotReady` and caches only terminal errors (bad connector,
  `NewChannel` failure after handshake, `populateNameMap` failure).
- **`reconcileOnReady` returns `(errs, deferred bool)`.** Spec asked
  for `[]error`. The bool makes the deferred state explicit at the call
  site, which matches "Explicit > implicit" in design-principles and
  avoids forcing `applyConfig` to re-scan `errs` for the sentinel.
- **`activeCfg` promoted to `atomic.Pointer[ifaceConfig]`.** The
  EventBus goroutine reads it while the SDK goroutine writes it from
  `OnConfigure` / `OnConfigApply`. A plain pointer plus mutex would
  work; `atomic.Pointer` is cleaner and matches the "short critical
  section" shape of this state.
- **`vppReadyOnce` around the subscription.** `OnConfigure` fires on
  every config reload, not just the first one. Without the `sync.Once`
  the plugin would accumulate duplicate subscriptions, and every future
  `EventConnected` would run reconciliation N times. Guarded via
  `vppReadyOnce.Do` inside the first `OnConfigure`; subsequent reloads
  leave the subscription intact.
- **Subscribe to BOTH `EventConnected` and `EventReconnected`.**
  `EventConnected` fires exactly once per VPPManager lifecycle (first
  successful handshake); `EventReconnected` fires on every subsequent
  successful handshake after a crash-recovery. fibvpp subscribes only
  to `EventReconnected` because it uses a mock backend pre-connect.
  iface needs both: the initial-connect path is the whole point of the
  deferred-reconcile flow.
- **Keep the additive-only fallback.** When reconcile is deferred at
  startup, `addDesiredAddresses` still applies every configured
  address immediately so the daemon is usable before VPP comes up.
  This preserves the existing partial-success contract; the only
  difference is that the deferred branch no longer appends an error
  to `errs`.

## Consequences

- A ze process starting with `interface { backend vpp; }` no longer
  logs `ERROR deliverConfigRPC failed ... list interfaces for
  reconciliation` during the startup race window. The 006-iface-create
  functional test asserts this via `reject=stderr:pattern=` on both
  patterns.
- Once VPP completes its handshake, iface's EventBus handler runs
  `reconcileOnVPPReady(activeCfg)` which calls `reconcileOnReady`,
  which runs Phase 3 (address add/remove) and Phase 4 (prune
  non-config Ze-managed interfaces). Addresses configured via ze now
  converge after the handshake even though they were not reachable
  during the initial apply.
- VPP crash-and-reconnect rerun the same reconciliation via
  `EventReconnected`, matching fibvpp's RIB-replay pattern.
- Netlink backend behavior is unchanged: its `ListInterfaces` never
  returns `ErrBackendNotReady`, so the `deferred=false` branch of
  `reconcileOnReady` runs synchronously on every apply.

## Tests

Unit (`internal/component/iface/config_test.go` +
`internal/plugins/ifacevpp/ifacevpp_test.go`):

- `TestEnsureChannel_NoConnectorReturnsSentinel` (AC-1)
- `TestEnsureChannel_NotConnectedReturnsSentinel` (AC-1)
- `TestEnsureChannel_NotReadyDoesNotCache` (regression guard for the
  non-caching contract)
- `TestReconcileOnReady_DefersOnSentinel` (AC-2)
- `TestReconcileOnReady_RecordsNonSentinelError` (AC-8)
- `TestApplyConfig_SkipsReconcileOnSentinel` (AC-2/3)
- `TestReconcileOnReady_AddsMissing` (AC-6)
- `TestReconcileOnReady_PrunesNonConfigInterface` (AC-6)
- `TestReconcileOnVPPReady_NoOpWhenActiveCfgNil` (defensive startup order)
- `TestReconcileOnVPPReady_RunsReconcile` (AC-4 at the handler layer)
- `TestReconcileOnVPPReady_InvokedOnEventConnected` (AC-4
  end-to-end via recordingEventBus + same `events.AsString` wiring
  register.go uses)
- `TestReconcileOnVPPReady_InvokedOnEventReconnected` (AC-5)
- `TestUnsubscribeOnShutdown` (AC-7 -- verifies the cleanup path
  returned by Subscribe actually removes handlers)

Functional: `test/vpp/006-iface-create.ci` extended with two reject
patterns. `bin/ze-test vpp 006-iface-create` -> 1/1 PASS in 5.0s.

## Deviations

- Spec named the event-bus unit tests `TestReconcileOnReady_InvokedOn*`
  but the actual helper is `reconcileOnVPPReady` (the package-level
  EventBus entry). Test names follow the symbol.
- Spec's `TestReconcileOnReady_AddsMissingRemovesStale` split into
  `TestReconcileOnReady_AddsMissing` + `_PrunesNonConfigInterface` for
  focused assertions.

## Out-of-Scope (follow-ups)

- An end-to-end reconcile-on-connect test that pre-populates a VPP
  interface with an address NOT in config and verifies it gets removed
  after `EventConnected` is deferred to `spec-vpp-stub-iface-api`
  because `vpp_stub.py` would need `sw_interface_dump`,
  `sw_interface_add_del_address`, and `sw_interface_details`
  implementations.
- `/ze-review` formal run is deferred to the user (interactive slash
  command, not agent-invokable).

## Files Touched

- `internal/component/iface/backend.go` -- `ErrBackendNotReady` sentinel.
- `internal/component/iface/config.go` -- `reconcileOnReady`,
  `reconcileOnVPPReady`, `addDesiredAddresses`; `applyConfig` branches
  on `deferred`.
- `internal/component/iface/config_test.go` -- `recordingEventBus` stub
  + 5 new event-driven / cleanup tests.
- `internal/component/iface/register.go` -- `activeCfg` atomic
  promotion; `vppReadyOnce`; two subscriptions appended to
  `unsubscribers`.
- `internal/plugins/ifacevpp/ifacevpp.go` -- `ensureChannel` checks
  `connector.IsConnected()`; non-caching sentinel branch.
- `internal/plugins/ifacevpp/ifacevpp_test.go` -- 3 tests.
- `internal/plugins/ifacevpp/query.go` -- minor edit (error-wrap
  alignment with the new sentinel).
- `test/vpp/006-iface-create.ci` -- two reject patterns.
