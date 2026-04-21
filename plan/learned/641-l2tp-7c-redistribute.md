# 641 -- l2tp-7c L2TP route-change events for redistribute

## Context

L2TP subscriber routes (/32 IPv4, /128 IPv6) needed to flow from the
L2TP subsystem to BGP peers via the existing bgp-redistribute pipeline.
The route observer already tracked session IPs and logged inject/withdraw,
but never emitted events. The bgp-redistribute consumer already handled
generic route-change batches from any non-BGP producer. The missing piece
was the producer side: a typed EventBus handle, emission logic in the
observer, and end-to-end functional tests.

## Decisions

- **Typed event handle in `l2tp/events/` subpackage** (over inline
  registration in route_observer.go): follows the `sysrib/events/`
  precedent; keeps the handle importable by both producer (observer) and
  consumer (bgp-redistribute) without pulling the full L2TP subsystem.
- **Pool-based batch lifecycle** (`AcquireBatch`/`ReleaseBatch`): matches
  the redistevents contract. Observer acquires, fills, emits, then
  releases. Subscribers must not retain the batch past Emit.
- **Per-family emission** (over a single multi-entry batch): a dual-stack
  subscriber emits two separate batches (one ipv4/unicast, one
  ipv6/unicast). On teardown, only families that were actually up get a
  remove batch. This prevents spurious withdrawals.
- **fakel2tp test plugin** (over trying to drive real L2TP sessions in
  .ci tests): emits on the L2TP event handle via dispatch-command,
  identical to how fakeredist tests the generic pipeline. Avoids kernel
  module dependency in functional tests.
- **Nil-bus tolerance preserved**: observer works without a bus (tests,
  partial subsystem init) -- state tracking and counters still function.

## Consequences

- Operators can now configure `redistribute { import l2tp { family
  ipv4/unicast ipv6/unicast; } }` and BGP peers receive subscriber
  prefixes as they come online.
- The fakel2tp plugin is shipped in production all.go (zero runtime cost
  when not invoked) and enables any future L2TP .ci test that needs
  synthetic route events.
- Pre-existing enum-over-string build errors in session_write.go and
  reactor_test.go (string literals where `rpc.MessageDirection` was
  expected) were fixed as a side effect of building.
- Pre-existing exhaustive-switch lint failures in fib-kernel, fib-p4,
  fib-vpp were fixed (missing `RouteActionDel` + `RouteActionUnspecified`
  cases).

## Gotchas

- `ReleaseBatch` zeroes the batch after Emit returns. Test stubs that
  capture the payload pointer see all-zero fields unless they deep-copy
  in `Emit`. The `recordingBus` test fake handles this.
- `RegisterSource` is idempotent for same name+protocol but returns
  `ErrSourceConflict` if the protocol differs. fakel2tp and the real
  subsystem both register "l2tp"/"l2tp" so both succeed regardless of
  init order.
- L2TP subsystem only starts when config has `l2tp { enabled true }` or
  listeners. The fakel2tp plugin independently registers the "l2tp"
  source so .ci tests don't need L2TP config.

## Files

- `internal/component/l2tp/events/events.go` -- typed EventBus handle (created)
- `internal/component/l2tp/events/events_test.go` -- handle registration tests (created)
- `internal/component/l2tp/route_observer.go` -- emit add/remove batches (modified)
- `internal/component/l2tp/route_observer_test.go` -- 6 new emission tests (modified)
- `internal/component/l2tp/subsystem.go` -- pass bus to observer (modified)
- `internal/test/plugins/fakel2tp/` -- test-only L2TP event producer (created)
- `test/plugin/redistribute-l2tp-announce.ci` -- announce .ci test (created)
- `test/plugin/redistribute-l2tp-not-configured.ci` -- no-config .ci test (created)
- `test/plugin/redistribute-l2tp-withdraw.ci` -- withdraw .ci test (created)
