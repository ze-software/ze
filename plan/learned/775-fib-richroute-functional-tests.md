# 775: FIB Rich Route Functional Tests

Spec: spec-fib-depth-3-richroute

## Context

fib-kernel handles rich route attributes (RouteType, Metric, TableID, Labels) via the
richRouteBackend interface. Unit tests already covered the dispatch logic directly.
This spec added functional tests proving the event delivery pipeline works end-to-end.

## Decisions

**fakefib test plugin created.** RouteType and TableID do not flow through the
bgp-rib -> sysrib pipeline because sysrib's protocolRoute struct has no fields for
them. Rather than extending sysrib (which would be a feature change, not a test),
a test-only internal plugin (fakefib) emits sysrib BestChangeBatch events directly
on the EventBus. This bypasses sysrib to deliver rich fields to fib-kernel.

**Metric tested via full pipeline.** MED from bgp-rib maps to BestChangeEntry.Metric
and flows through sysrib unchanged. AC-2 uses `bgp rib inject ... med 200` to
exercise the real production path.

## Consequences

- fakefib lives in `internal/test/plugins/fakefib/`, loaded only under the `zetest`
  build tag. Zero cost in production builds.
- On macOS, fib-kernel uses unsupportedBackend (no richRouteBackend). Functional
  tests verify event delivery and processing, not kernel route state. Kernel-level
  verification is in unit tests (fibkernel_test.go) and QEMU integration tests.

## Gotchas

- sysrib does not populate RouteType or TableID on outgoing BestChangeEntry. Any
  future spec that needs these fields end-to-end must add them to protocolRoute
  and the recomputeBest emitter.
- `bgp rib inject` only supports simple prefix families. MPLS labeled unicast
  injection is not possible via this command, so Labels must be tested via fakefib.

## Files

None recorded.
