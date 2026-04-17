# 613 -- vpp-2 FIB VPP Plugin

## Context

The VPP integration set needed a FIB plugin to program VPP's forwarding table directly
from ze's system RIB via GoVPP binary API, bypassing the kernel netlink path entirely.
This is the core value proposition of ze's VPP strategy: sub-second convergence for a
full BGP table (~250K API calls/sec) compared to ~6s via netlink batching. The plugin
mirrors fib-kernel and fib-p4 structurally but targets VPP's IPRouteAddDel API.

## Decisions

- **Copied fib-p4/fib-kernel pattern** over inventing a new plugin shape. Same event
  subscription (system-rib, best-change), installed map tracking, replay-request on
  startup, `show` command. Keeps all three FIB backends interchangeable.
- **Per-route dispatch** over time-based batch accumulation. VPP has no multi-route batch
  API; sysRIB already delivers per-family batches. The YANG `batch-size` and
  `batch-interval-ms` leaves exist and are accepted by the parser but not consumed.
  Adding a timer goroutine for cross-emission accumulation would add complexity for zero
  measurable benefit given the API constraint.
- **Mock GoVPP channel** for backend tests over integration against a real VPP or the
  Python stub. The testChannel captures IPRouteAddDel requests and returns configurable
  retvals, enabling byte-level verification of prefix conversion, AF selection, table-id
  propagation, and error handling without any VPP dependency.
- **Noop fallback** when VPP connector is nil (connector not available at startup). The
  plugin runs with a mockBackend, allowing `fib.vpp` config to be present without
  requiring a running VPP instance. The vpp-7 test harness relies on this.

## Consequences

- Every sysRIB best-change event now has two potential consumers (fib-kernel for local
  services, fib-vpp for transit). Both subscribe independently; no coordination needed.
- VPP restart recovery is simple: the plugin subscribes to `(vpp, reconnected)`, emits
  `(system-rib, replay-request)`, and sysRIB replays the full table. No sweep/reconcile
  needed because VPP FIB is ephemeral.
- MPLS label support (vpp-3) will extend the backend's `toFibPath` to populate
  `NLabels`/`LabelStack` fields. The current zero values are correct for unlabeled routes.
- The 003-fib-withdraw, 004-vpp-restart, and 007-coexist functional tests are deferred
  to vpp-7 Phase 3 (tracked in plan/deferrals.md).

## Gotchas

- **Spec status was stale at 1/3.** All three implementation phases (backend, event
  processing, plugin wiring) were already complete. The gap was backend_test.go (no unit
  tests for the GoVPP backend layer) and the unfilled audit tables.
- **`go build ./...` fails** with undefined `format.FormatMessage` in
  `bgp/server/events.go`. Pre-existing, unrelated to fibvpp. Package-level
  `go test ./internal/plugins/fibvpp/` compiles and passes cleanly.
- **GoVPP `api.Channel` interface** includes `CheckCompatiblity` (note the typo in the
  vendored code). Mock must match this spelling exactly.

## Files

- `internal/plugins/fibvpp/backend_test.go` -- new: 15 tests via mock GoVPP channel
- `internal/plugins/fibvpp/fibvpp.go` -- comment clarifying batch config intent
- `internal/plugins/fibvpp/backend.go` -- unchanged (read for audit)
- `internal/plugins/fibvpp/register.go` -- unchanged (read for audit)
- `internal/plugins/fibvpp/stats.go` -- unchanged (read for audit)
- `plan/spec-vpp-2-fib.md` -- filled audit, verification, implementation summary
- `plan/deferrals.md` -- 6 vpp-7 deferral rows added
