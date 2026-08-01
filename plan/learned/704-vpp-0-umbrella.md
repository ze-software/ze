# 704 -- vpp-0-umbrella

Spec: spec-vpp-0-umbrella.md
Date: 2026-05-15

## What this was

Umbrella spec for ze's VPP integration: Strategy 3 (RIB to GoVPP to VPP FIB,
bypassing the kernel netlink path). No code of its own. It defined the
architecture, child spec breakdown, dependency ordering, YANG config shape,
and the wiring test table that every child had to satisfy.

Seven child specs plus two cross-referenced firewall/traffic specs, spanning
roughly 6000 LOC of new Go code and one Python stub.

## Child spec outcomes

| Child | Scope | Learned | Outcome |
|-------|-------|---------|---------|
| vpp-1 (lifecycle) | Component: startup.conf, DPDK, GoVPP, health | 611 | Shipped. Manager with Run(ctx) loop, `vpp.external` leaf for non-supervised mode |
| vpp-2 (fib) | FIB plugin: IPv4/IPv6 route programming | 613 | Shipped. Copied fib-p4/fib-kernel pattern. Per-route dispatch (no batch API in VPP) |
| vpp-3 (mpls) | MPLS label push/swap/pop from BGP labels | 702 | Shipped. Labels as FamilyRIB side-data, three operations across two VPP APIs |
| vpp-4 (iface) | iface.Backend implementation | 615 | Shipped. Lazy channel acquisition, monitor event translation to netlink-compatible shape |
| vpp-5 (features) | L2XC, VXLAN, policers, ACLs, SRv6, sFlow | -- | Retired. Design dead-end: VPP features belong under ze abstractions (iface/firewall/traffic), not a parallel config surface |
| vpp-6 (telemetry) | Stats segment polling, Prometheus metrics | 612 | Shipped. Poller in VPP component, fibvpp owns its own route-count metrics |
| vpp-7 (test harness) | Python GoVPP stub + ze-test vpp runner | 610 | Shipped. Stdlib-only stub, binapi scraping keeps it in sync with pinned VPP release |
| fw-6 (firewall-vpp) | VPP ACL backend | 671 | Shipped. Cross-ref: depends on vpp-1 |
| fw-7 (traffic-vpp) | VPP policer backend | 627 | Shipped. Cross-ref: depends on vpp-1 |
| iface-vpp-ready-gate | Deferred reconciliation until VPP connected | 617 | Shipped. Follow-up from vpp-4 cold-boot race |

## Decisions that held

1. **GoVPP binary API, no CGo.** GoVPP's socket client is pure Go. No build
   complexity, no CGo cross-compilation issues. Held across all seven phases.

2. **Component + plugin split.** VPP lifecycle in `internal/component/vpp/`,
   FIB in `internal/plugins/fibvpp/`, interfaces in `internal/plugins/ifacevpp/`.
   Clean separation survived every phase without coupling creep.

3. **EventBus for lifecycle, direct import for API.** `("vpp",
   "connected/disconnected/reconnected")` for notifications, `vpp.Channel()`
   for GoVPP calls. Dependency ordering via `Dependencies: ["rib", "vpp"]`.

4. **fib-kernel and fib-vpp coexist.** Both subscribe to `(system-rib,
   best-change)` independently. No coordination needed.

5. **VPP FIB is ephemeral.** Recovery via replay-request from sysRIB. Simpler
   than fib-kernel's crash recovery, proved correct in vpp-2 and vpp-3.

6. **Pin to VPP 25.02 LTS.** GoVPP bindings match the release. Stub uses
   binapi scraping to stay in sync automatically.

## Decisions that changed

1. **vpp-5 retired.** The umbrella planned a "VPP features" phase for L2XC,
   VXLAN, bridge domains, policers, ACLs, SRv6, sFlow under a dedicated VPP
   config surface. Killed as a design dead-end: VPP is a backend for ze's
   existing abstractions, not a separate config namespace. Features re-homed:
   bridge to iface backend, VXLAN to iface tunnel, policer/ACL to
   fw-6/fw-7.

2. **vpp-3 was deferred then unblocked.** Originally blocked on sysRIB event
   payload lacking a labels field. Resolved by adding labels as FamilyRIB
   side-data (parallel BART store) rather than extending RouteEntry.

3. **Batch dispatch dropped.** vpp-2 planned batch optimization with
   configurable batch-size and batch-interval. VPP has no multi-route batch
   API, sysRIB already delivers per-family batches. YANG leaves exist but
   are not consumed. Per-route dispatch is correct.

4. **`vpp.external` leaf added (not in umbrella).** Emerged from vpp-7 test
   harness needs, but has independent value for systemd-managed and
   container-sidecar deployments.

## Patterns established

- **FIB backend pattern is now three-deep** (kernel, p4, vpp). Any future
  dataplane (XDP, ASIC SDK) copies the same registration + event subscription
  shape.

- **iface.Backend pattern is two-deep** (netlink, vpp). Backend-agnostic
  event payloads: ifacevpp translates VPP events to the same JSON shape
  as ifacenetlink, so downstream consumers stay backend-agnostic.

- **Python GoVPP stub** (`test/scripts/vpp_stub.py`): stdlib-only, 340
  lines, handler-extensible. Adding a new VPP message type is 1-2 handlers.
  Binapi scraping keeps the stub's message table in sync with vendored code.

- **Lazy channel acquisition** for backends that load before their
  dependency is ready (`ensureChannel` + `sync.Once`). Applied in ifacevpp,
  applicable to any future backend with a cold-boot race.

## Cross-cutting lessons

- **Side-data parallel stores need parallel cleanup.** vpp-3 found that
  adding a parallel BART store for labels without updating all removal paths
  (Remove, PurgeStale, Reset) creates invisible leaks.

- **Same-best optimization must check new fields.** When extending an event
  struct with a new field (labels), the same-best short-circuit must compare
  the new field or changes are silently dropped.

- **`sync.Once` recursion deadlocks silently.** vpp-4's `dumpAllRaw` split
  was needed because helper methods called back into the gated initializer.

- **VPP string fields are NUL-padded.** All 64-byte string fields from VPP
  need `trimCString` before use as map keys or return values.

- **GoVPP `sockclnt_create` type inversion.** The vendored binapi declares
  `sockclnt_create` as `ReplyMessage` and `sockclnt_create_reply` as
  `RequestMessage`, opposite of intuition. The stub must honor declared types.

## What remains outside this umbrella

- Tunnel types (VXLAN, GRE, IPIP) under iface backend: need GoVPP binapi
  vendoring approval
- LCP pair management in ifacevpp: needs LCP binapi vendoring
- Stats segment emulation in vpp_stub.py for `012-telemetry.ci` functional
  test
- Multi-label stack (stacked LSPs) in fibvpp MPLS dispatch
- L3VPN (per-VRF FIB tables) extending the MPLS label chain
- `ze config validate` for YANG enum/range constraints (pre-existing gap)

## Files

None recorded.
