# 522 -- FIB Pipeline (Umbrella)

## Context

Ze had no route installation pipeline. The BGP RIB computed best paths only on demand (CLI query), never tracked changes in real-time, and never published them. There was no system-wide route selection across protocols, and no mechanism to program the OS kernel routing table. Routes learned via BGP existed only in memory -- they never reached the forwarding plane.

## Decisions

- Chose Bus-only communication between all FIB pipeline components (over direct function calls or shared state), because plugins must not import each other and the Bus is the established inter-component channel.
- Chose batch event format (one Bus event carries an array of prefix changes) over per-prefix events, because a full-table peer-down would produce 900K individual publishes otherwise. Changes collected under RIB lock, published after release.
- Chose admin distance (lower wins) for system RIB selection over a more complex policy engine, because it matches standard networking convention and covers the immediate need. Configurable via YANG.
- Chose control-topic replay (`rib/replay-request`) over Bus interface changes for full-table sync on late subscriber join, because it avoids modifying the Bus interface and uses existing pub/sub patterns.
- Chose build-tag OS selection for fib-kernel backends (`backend_linux.go` with netlink, `backend_other.go` with noop) over runtime detection, matching the iface plugin pattern.
- Placed sysrib and fib-kernel in `internal/plugins/` (not under `internal/component/bgp/plugins/`) because they are protocol-independent -- they react to Bus events from any protocol, not just BGP.

## Consequences

- BGP RIB now tracks best-path changes in real-time and publishes `rib/best-change/bgp` events. This is the foundation for any downstream consumer (system RIB, monitoring, analytics).
- System RIB plugin enables future multi-protocol route selection (static, OSPF) without changing the BGP RIB or fib-kernel -- new protocols just publish to `rib/best-change/<protocol>`.
- fib-kernel with netlink backend can program Linux routing tables. Custom `rtm_protocol` ID (RTPROT_ZE=250) enables crash recovery via stale-mark-then-sweep.
- Kernel route monitoring (netlink multicast groups) detects external route modifications and re-asserts ze routes -- standard routing daemon behavior.
- `internal/plugins/` directory is now established for non-BGP plugins. `scripts/gen-plugin-imports.go` updated to scan it.
- fib-p4 plugin is structurally complete (Bus subscription, event processing, backend interface, YANG, tests) but uses a noop backend. Real P4Runtime transport requires adding `google.golang.org/grpc` to go.mod.

## Gotchas

- The `unused` linter traces the full call graph. Adding a function that's only reachable through a chain of uncalled functions causes lint errors on the entire chain. Must wire into production code before the linter accepts it.
- The `check-existing-patterns.sh` hook blocks exported functions (`SetLogger`, `SetBus`) that exist in other packages. Per-plugin pattern requires unexported names (`setLogger`, `setBus`) when functions are only called within the same package.
- `handleReceivedStructured` originally used `defer r.mu.Unlock()`. Changing to explicit unlock (to publish after lock release) required careful restructuring to avoid forgetting the unlock on early returns.
- The Bus `Subscribe` prefix matching means `rib/best-change/` catches `rib/best-change/bgp`, `rib/best-change/static`, etc. -- but also `rib/best-change/replay-request` if someone carelessly names a topic. Topic naming discipline matters.

## Files

- `internal/component/plugin/registry/registry.go` -- ConfigureBus field + SetBus/GetBus
- `internal/component/plugin/inprocess.go` -- wire ConfigureBus into plugin runner
- `internal/component/bgp/plugins/rib/rib.go` -- Bus storage, bestPrev tracking state
- `internal/component/bgp/plugins/rib/rib_structured.go` -- best-path change detection after insert/remove
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- best-path tracking, replay, Bus publishing
- `internal/plugins/sysrib/` -- System RIB plugin (sysrib.go, register.go, schema/)
- `internal/plugins/fibkernel/` -- FIB kernel plugin (fibkernel.go, register.go, backend*.go, monitor*.go, schema/)
- `internal/plugins/fibp4/` -- FIB P4 plugin (fibp4.go, register.go, backend.go, schema/). Noop backend until gRPC/P4Runtime added to go.mod
- `scripts/gen-plugin-imports.go` -- scan internal/plugins/ directory
- `docs/architecture/core-design.md`, `docs/features.md`, `docs/guide/plugins.md` -- documentation
- `test/plugin/fib-rib-event.ci`, `test/plugin/fib-sysrib.ci` -- functional tests
