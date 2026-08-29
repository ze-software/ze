# Subsystem flow digests

<!-- Living docs, maintained by hand (NOT generated). -->

Each file here is a "current flow" digest for one subsystem: what it is, how data
flows through it (entry to exit, with `file:line`), the load-bearing files, and the
invariants and gotchas. Read the digest to orient before diving into a subsystem, then
open the files it names.

These are **living** documents. When you change a subsystem's flow, update its digest
in the same work. They are hand-maintained, not generated, so treat any detail as a
strong hint and verify `file:line` before relying on it.

Each digest declares the subtree(s) it anchors into with a machine-readable header:

    <!-- digest-base: internal/component/bgp/reactor internal/component/bgp/fsm -->

`./le digest` (run inside `./le doc check verify` and by the digest stage in
`internal/le/docwiring` when a digest or a `.go` under one of those bases
changes) validates that every `file:line` anchor resolves to a real file and an in-range line.
Anchors are written subsystem-relative (`peer.go`, not the full path); a bare name must be unique across
the declared bases, so qualify it with enough path (`storage/familyrib.go`, or the full
repo-relative path) when the same basename exists under more than one base. The check
fails closed on such ambiguity rather than guessing, so it will not silently validate an
anchor against the wrong same-named file. This catches the anchors rotting, but cannot
check that the prose still matches the code, so this file stays authoritative: verify
before relying on a detail. The recurrence record lives in
`plan/journal/`, the canonical design in `docs/architecture/`; a digest is the
fast-orientation layer between them, and `ai/PACKAGE-MAP.md` is the per-package index
below it.

| Area | Digest | Subsystem |
|------|--------|-----------|
| BGP core | `bgp-reactor.md` | BGP reactor / session / FSM (the peer event loop) |
| BGP core | `wire-and-pools.md` | Wire encoding + buffer/pools (buffer-first, zero-copy) |
| BGP core | `rib.md` | RIB: route storage + best-path selection |
| BGP core | `config-pipeline.md` | YANG config: File to Tree to ResolveBGPTree to live peers |
| BGP core | `plugin-transport.md` | Engine to plugin: registry, DirectBridge, EventBus |
| BGP core | `cli-editor.md` | CLI / config editor (SSH, YANG editor, completion) |
| Routing | `ospf.md` | OSPFv2/v3: Hello/adjacency, LSDB, SPF, route install |
| Routing | `isis.md` | IS-IS: IIH adjacency, LSP/LSDB (L1/L2), SPF, route install |
| Routing | `mpls-signaling.md` | MPLS label signaling (LDP + RSVP-TE) and the label FIB |
| Forwarding | `fib-programming.md` | RIB to FIB to kernel: redistribute, sysrib, netlink program |
| Forwarding | `vpp-dataplane.md` | VPP dataplane: process lifecycle, route/interface programming |
| Interfaces/services | `iface.md` | Interface management: YANG to netlink, WireGuard, PPPoE, address ownership |
| Interfaces/services | `firewall.md` | Firewall/nftables: ruleset apply, NAT, FlowSpec, policy routing |
| Interfaces/services | `ipsec-ike.md` | IPsec/IKEv2: SA negotiation, child SA, XFRM install |
| Interfaces/services | `subscriber.md` | Subscriber sessions: PPPoE/L2TP access concentrator, PPP, RADIUS |
| Interfaces/services | `dns-services.md` | DNS: shared harness, as112 anycast sink, geodns |
| Control/API/UI | `api-ipc.md` | External API command protocol + IPC/muxconn transport |
| Control/API/UI | `web.md` | Web interface + looking glass (HTMX/SSE, query path) |
| Control/API/UI | `mcp.md` | MCP server: tool registration, JSON-RPC dispatch to commands |
| Control/API/UI | `hub-engine.md` | Hub/engine/coordinator: bootstrap, subsystem wiring, fleet-managed |
| Telemetry/security | `observation-telemetry.md` | Observation feed fan-out + metrics/Netdata/Prometheus/gNMI |
| Telemetry/security | `flow-ddos.md` | Flow export, DDoS detect/mitigate, behavioral anomaly |
| Telemetry/security | `aaa-auth.md` | AAA: SSH login, authn (local/TACACS+), RBAC authz, accounting |

To add a subsystem: trace it from real code, write `<name>.md` in the same shape, set its
`<!-- digest-base: -->` header, add a row here, and confirm `./le digest` passes.
