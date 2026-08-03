# 1286 -- Plugin ConfigureMetrics never reached the registry; role-otc-egress-filter proved nothing

## Context

`test/plugin/role-otc-egress-filter.ci` (CI test 401) timed out at exactly 15.0s,
three runs out of three when run alone. It was reported as intermittent; it was
not. The observer polled `show bgp adj-rib-in status` for `total-routes >= 1`,
a predicate that could never become true (the source ze-peer had already closed,
and `status()` counts only `ribIn`, which `handleStructuredState` empties on
peer-down), burning its full 5.02s budget; `quiesce()` then burned the full 10.00s
`quiesceTimeout` because the closed peer's `opQueue` never drains and
`PendingSync` does not gate on peer state. 5.02 + 10.00 = 15.02s against a 15s
budget.

Fixing the timeout exposed two further problems. The test's only real assertion
was an ABSENCE (the dest peer receives its End-of-RIB and nothing else), which a
daemon satisfies just as well by never deciding anything -- and in one run in
three it did exactly that. Reaching for the state that would prove the egress
filter had actually run, `ze_role_route_suppressions_total`, found it did not
exist at runtime: no internal plugin's metrics were registered at all.

## Decisions

- **`ConfigureMetrics` is deferred, not conditional.** `GetInternalPluginRunner`
  read `registry.GetMetricsRegistry()` at spawn time and dropped the hook when it
  was nil. It is ALWAYS nil there: `runPluginPhase` spawns every process of a
  phase (step (a)) before the tier-ordered handshake (step (c)), and the registry
  is not created until the bgp plugin's stage-2 `OnConfigure` builds the reactor.
  Chosen over moving the exporter earlier in hub startup, which would have to
  reconcile with `startStandaloneTelemetry` and would still bind correctness to
  one ordering. `registry.InjectPluginMetrics` defers the hook and
  `SetMetricsRegistry` drains the pending set, so whichever happens second does
  the injection, once per plugin.
- **The test asserts both halves.** The source's `updates-received >= 1` (ze had
  the route) plus the dest's `updates-sent - eor-sent == 0` (nothing but the EOR
  left), behind two barriers: `wait_peer_eor_sent(2)` and `quiesce()`. Either
  half alone is vacuous. `expect=stderr:contains=role dropped a route` pins that
  the ROLE PLUGIN is what withheld it.
- **Both ze-peers linger.** Chosen over asserting the disjunction of the two
  daemon behaviors, which would have re-admitted the vacuity.
- **`flushInPlace` bound-checks before slicing.** `flush()` already restores the
  pre-write snapshot and falls back to `flushFull` on error, so an error is a
  strictly better outcome than the panic.

## Consequences

- Every internal plugin's counters now exist: `ze_role_*`, `ze_rib_*`,
  `ze_attr_pool_*` and the rest were all absent from `show metrics values` before
  this. Any alert or dashboard built against a plugin metric was silently empty.
- `explainNoTarget` now prints `down` and `not-yet-up` separately. The two need
  opposite fixes and the log line exists to tell them apart; one label for both
  cost a full diagnosis cycle chasing a replay-ordering bug that did not exist.
- A plugin author must not read the metrics struct during `init()` or at the top
  of `RunEngine`, and cannot order the injection with a `Dependency` --
  dependencies order the handshake, not the spawn.

## Gotchas

- **`why="not-up"` did not mean "not yet up".** `PeerState.StateSeen` exists to
  distinguish "the peer-up event has not arrived" from "the peer went down", and
  `explainNoTarget` ignored it. The dest was a check-mode ze-peer that had closed
  the instant its rule matched -- there was no destination left to suppress
  toward, which is not the same failure at all.
- **A green functional suite hid this for both defects.** The old 401 passed
  whenever the daemon never decided, and nothing anywhere asserted that a plugin
  metric was reachable.
- **`go test` without the feature tags fakes reds.** Running the plugin packages
  bare reported `TestGetInternalPluginRunner` and `TestGetPluginForFamily`
  failures that vanish under `-tags "$ZE_FEATURES"` (`ai/rules/commands.md`).
- **A verify run concurrent with other suites fakes reds too.** A backgrounded
  `ze-verify-changed` overlapping functional runs reported failures in
  `rib-inject-rfc5549`, `ospf-interface-runtime` and `ospf-ldp-sync-show`; all
  three pass in a clean serial run. It did surface one real bug -- the zefs
  panic -- so read such a log for CRASHES, never for verdicts.
- **Both mutation checks were needed.** Disabling `OTCEgressFilter` flips 401 red
  ("provider route leaked", `updates-sent: 2, eor-sent: 1`); restoring the old
  `if mr != nil` flips `plugin-metrics-registered` red ("absent from the metrics
  registry", `count: 0`). Neither test would have caught its own subject without
  that check.

## The exabgp reds were an unfinished migration, not a flake

`make ze-exabgp-test` was red on 13 `conf-*` tests, deterministically, before any
of this session's changes. Same cause every time: ze emitted MP_REACH (14) before
EXT_COMMUNITY (16) where the committed expectation had them the other way round.

That is `7a0aa4b25 fix(bgp): one attribute order for every UPDATE builder`
(`plan/learned/1285`), which chose ascending type code and rewrote 52
expectations across `test/encode` and `test/plugin` -- but counted only those two
trees. `test/exabgp-compat/encoding` holds more expectations in the same shape,
and they were missed, so the suite has been red since. 1285's own advice
("apply the change, run the suites, count") is exactly what would have caught it;
the count was taken over an incomplete set.

The rewrite used the same discipline 1285 prescribes -- permute the COMMITTED
hex, never paste the daemon's output -- but the tool it describes was never
committed, so it is now `scripts/dev/reorder_attr_expectations.py`. It asserts
per expectation that the attribute set, every attribute's bytes and the block
length are unchanged, and 0 were left alone. `1d96f883b` rewrote 53 expectation
lines across 13 files.

These goldens were captured from ExaBGP, so permuting them narrows what the
suite proves from "byte-identical to ExaBGP" to "byte-identical up to attribute
order". RFC 4271 Section 5 leaves the order free, and the convention was an owner
decision already taken in 1285 -- but it is a change in what the compat suite
asserts, and it should be a conscious one rather than a side effect.

## Files

- `internal/component/plugin/registry/registry.go` -- `InjectPluginMetrics`, deferred set, `SetMetricsRegistry` drain, `Reset`
- `internal/component/plugin/inprocess.go` -- spawn path defers instead of dropping
- `internal/component/plugin/registry/metrics_injection_test.go` -- ordering, idempotence, drain, Reset isolation
- `test/plugin/plugin-metrics-registered.ci` -- functional proof a plugin's metric families are queryable
- `test/plugin/role-otc-egress-filter.ci` -- both barriers, both halves, both peers linger
- `internal/component/bgp/plugins/rs/server_forward.go` -- `down` vs `not-yet-up`
- `pkg/zefs/store.go` -- `flushInPlace` bound check
- `internal/test/cli/cmd_bgp.go` -- `cmdReload` constant (lint)
- `scripts/dev/reorder_attr_expectations.py` -- the permute-don't-paste transform 1285 described but never committed
- `test/exabgp-compat/encoding/*.ci` -- 53 expectations across 13 files permuted into 1285's attribute order
