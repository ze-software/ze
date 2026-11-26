# Spec: bgp-listen-netns -- Full-stack network namespace isolation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release full-stack network namespace isolation (`netns <name> { ... }`), split out of `plan/release/fixes/spec-bgp-netns.md` per Thomas's 2026-11-24 owner ruling rejecting a per-protocol BGP `listen { netns }` leaf. |
| RFC gaps | - |
| Depends | plan/post-release/features/spec-vrf-0-umbrella.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Per-protocol `bgp { listen { netns } }` leaf vs. full-stack isolation | Reject the per-protocol `bgp { listen { netns } }` leaf. Network namespaces are a full-stack wrapper (`netns <name> { bgp { ... } interface { ... } }`, nestable) designed post-release alongside Linux VRF support (`plan/post-release/features/spec-vrf-0-umbrella.md`), not a BGP-only listener leaf. |
| Release-1 VPP LCP compatibility | Handled separately in `plan/release/fixes/spec-bgp-netns.md` by changing `vpp.lcp.netns` default from `"dataplane"` to `""` (host namespace) for v1.0 so VPP LCP TAP interfaces and routing protocols work together out of the box without per-protocol netns plumbing. |

## Task

Provide full-stack Linux network namespace isolation in Ze as a nestable configuration wrapper (`netns <name> { bgp { ... } interface { ... } }`) designed alongside the post-release VRF architecture (`plan/post-release/features/spec-vrf-0-umbrella.md`).

This enables operators who deliberately isolate a forwarding plane (such as VPP LCP TAPs placed in an explicit `vpp.lcp.netns` namespace like `"dataplane"`, or multi-tenant routing domains) to run BGP, OSPF, interfaces, RIB, FIB, and management services inside named Linux network namespaces while keeping Ze's management plane in the host namespace.

This spec does **not** add a BGP-only `bgp { listen { netns } }` leaf; Thomas explicitly rejected that per-protocol shape on 2026-11-24.

## Required Reading

- [ ] `plan/post-release/features/spec-vrf-0-umbrella.md` - full-stack per-VRF isolation model (multi-instance hub, per-VRF component spawning, YANG wrapping)
  → Constraint: design `netns <name> { ... }` alongside VRF so nesting (`netns` containing `vrf` or nested `netns`) shares one stack-instantiation model rather than inventing a second orchestrator.
- [ ] `plan/release/fixes/spec-bgp-netns.md` - v1.0 default fix (`vpp.lcp.netns` default `""`) and origin of this split
  → Decision: v1.0 defaults LCP TAPs to the host namespace (`""`); non-empty `vpp.lcp.netns` continues to warn via `doctor-vpp-lcp-netns` until full-stack `netns` lands.
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/forwarding.md`, `ai/doors/core.md` - process boundary, module tiers, and compile-out boundaries.

## Carried-Forward Research Notes (from 2026-07-16 investigation)

These verified facts from the earlier `spec-bgp-netns` research remain useful input when designing full-stack `netns` post-release:

- **Per-process vs. per-socket netns entry:** `spec-vrf-0-umbrella.md` spawns one plugin process per component per VRF. If a `netns <name>` stack spawns its component processes inside the target network namespace (or enters the namespace per-process), listeners, dialers, netlink sockets, and other services (`web`, `lg`, `dns`, `ospf`, `bgp`) inherit the namespace uniformly without per-socket thread pinning.
- **If per-socket netns binding is ever needed inside a shared process:** `Reactor.newListenerFactory` (`internal/component/bgp/reactor/reactor.go`) selects either a fresh `network.RealListenerFactory{MD5Peers, ListenTTL}` or `r.listenerFactory`, never stacking them. Any per-socket netns property would therefore have to be a field on `RealListenerFactory` (`internal/core/network/network.go`) and `RealDialer`, composing with `MD5Peers` and `ListenTTL` rather than wrapping `ListenerFactory`.
- **In-tree netns precedents:** `github.com/vishvananda/netns` is already a direct dependency (`go.mod`), used in `internal/core/routewatch/routewatch_linux.go` and `internal/test/runner/netns_linux.go`.
- **Unvalidated kernel hypothesis (if per-socket pinning is used):** whether `setns` + `runtime.LockOSThread` around socket create+bind allows `Accept` after thread unpin must be proven under QEMU (`ai/rules/platform-linux.md`) before relying on it.
- **Doctor check reconciliation:** once Ze can run routing stacks inside a named namespace, `checkVPPLCPNetns` (`internal/plugins/iface/vpp/doctor.go`, code `doctor-vpp-lcp-netns`) should narrow from warning on any non-empty `vpp.lcp.netns` to warning when `vpp.lcp.netns` disagrees with the namespace in which the routing stack runs.

## Current Behavior

- Ze runs its entire control plane in a single network namespace (the namespace of the `ze` process).
- `vpp.lcp.netns` defaults to `""` (per `plan/release/fixes/spec-bgp-netns.md`), placing VPP LCP TAPs in VPP's own (host) namespace.
- Setting `vpp.lcp.netns` to a non-empty name emits `doctor-vpp-lcp-netns` because Ze's BGP listener (`internal/core/network/network.go`, `RealListenerFactory.Listen`) and other protocols cannot cross into another namespace unless `ze` itself is launched inside that namespace.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Config declares `netns <name> { bgp { ... } interface { ... } }` (including nested `netns` or `vrf` blocks) | Ze runs the enclosed protocol and interface stack inside the named Linux network namespace while keeping top-level management in the host namespace | Placeholder skeleton AC; refine with concrete test assertions and permitted survivors when promoted to `design` |
| AC-2 | `vpp.lcp.netns` names `<name>` and a matching `netns <name> { bgp { ... } }` stack is configured | BGP binds and peers over LCP TAP interfaces inside `<name>`, and `checkVPPLCPNetns` (`doctor-vpp-lcp-netns`) narrows to warn only on namespace mismatch | Placeholder skeleton AC; refine when promoted to `design` |

## Open Design Questions (for future design phase)

1. Process-per-netns (entering netns at plugin subprocess launch, sharing `spec-vrf-0-umbrella.md`'s orchestrator) vs. in-process socket-level netns switching.
2. Exact YANG wrapping and nesting rules between `netns <name> { ... }` and `vrf <name> { ... }`.
3. Lifecycle of the Linux network namespace itself (whether Ze creates/deletes `/var/run/netns/<name>` or binds only to pre-existing namespaces).
4. QEMU integration rail for multi-namespace routing and VPP LCP end-to-end verification (`ai/rules/platform-linux.md`).
