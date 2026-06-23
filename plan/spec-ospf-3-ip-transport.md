# Spec: ospf-3-ip-transport

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-1-types.md |
| Phase | 10/10 (implementation/review complete; overall commit deferred until all OSPF specs complete) |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - umbrella scope (this is row ospf-3); Shared Contracts "Frame addressing + transport" and "Packet receive dispatcher", the canonical Metrics table, and the "Existing Foundation" RSVP-TE raw-IP row
4. `internal/plugins/rsvpte/transport_linux.go` - the proven `AF_INET SOCK_RAW` raw-IP transport (proto 46) this transport is modelled on; kernel builds the outgoing IP header, the receive path strips the IPv4 header via the IHL field
5. `internal/plugins/rsvpte/doctor_linux.go` - the raw-socket doctor probe pattern (open+close, `CAP_NET_RAW`)
6. `internal/plugins/iface/netlink/monitor_linux.go`, `internal/component/iface/events/events.go` - interface up/down subscription that drives circuit open/close
7. `docs/research/ospf-implementation-guide.md` sec 2 (Protocol and Addressing, lines 83-97), sec 7 (Network Types and Interface Model, lines 470-518), sec 9 (Concurrency and I/O Model, lines 548-591)

## Task

Implement the raw IP transport for OSPFv2 in `internal/plugins/ospf/transport/`,
modelled on the proven RSVP-TE `AF_INET SOCK_RAW` socket pattern
(`internal/plugins/rsvpte/transport_linux.go`, proto 46). OSPF runs directly
over IPv4 with IP protocol number 89 (`docs/research/ospf-implementation-guide.md`
sec 2). This is NOT a new low-level capability: per `plan/spec-ospf-0-umbrella.md`
"Existing Foundation", the raw-IP socket already exists in-tree (RSVP-TE) and the
only new I/O wrinkle is IP multicast group membership on the raw socket.

The transport opens an `AF_INET SOCK_RAW` socket on IP protocol 89. The kernel
builds the outgoing IP header (`IP_HDRINCL` off), so a send supplies only the
OSPF payload; on receive a raw IPv4 socket delivers the full datagram including
the IP header, which the transport strips using the IHL field (low nibble of byte
0, times 4) before delivering `(ifindex, src netip.Addr, payload []byte)` to the
engine. The transport MUST NOT parse or alter the OSPF payload bytes: per
`plan/spec-ospf-0-umbrella.md` "Packet receive dispatcher", the common-header
`Type` dispatch and all validation (version/area/checksum/auth) are owned by
ospf-4 (`instance.go`), not by this transport.

Unlike RSVP-TE (which only ever unicasts), OSPF requires IP multicast group
membership. The transport joins `224.0.0.5` (AllSPFRouters) on every enabled
interface and joins `224.0.0.6` (AllDRouters) only when this router is DR or BDR
on that interface; it leaves a group on interface teardown or DR/BDR role change.
Membership is per-interface using an `IPMreq` bound to the interface IPv4 address
(`IP_ADD_MEMBERSHIP` / `IP_DROP_MEMBERSHIP`), the QEMU-validated Linux raw
multicast receive path. Outbound multicast uses `IP_MULTICAST_TTL` = 1
(link-local, `docs/research/ospf-implementation-guide.md` sec 2 "TTL 1"),
`IP_MULTICAST_IF` / per-interface source selection so a packet leaves the
intended interface, and `IP_MULTICAST_LOOP` off so the local router does not
receive its own multicast.
A send is directed by the engine to either a multicast group (`224.0.0.5` /
`224.0.0.6`) or a unicast neighbour address; the transport does not decide the
destination, it only frames and sends.

The backend sits behind a Go interface (open, close, send, receive, join/leave
group per interface) so a future BSD or VPP backend can drop in: a real Linux
`backend_linux.go` and a non-Linux stub `backend_other.go` behind the interface.
v1 ships only the Linux backend. Per-interface RX and TX goroutines drive each
enabled interface; received datagrams are dispatched by the receiving ifindex.
The transport subscribes to the iface EventBus and opens an interface on link up,
closes it and signals teardown on link down (mirroring the IS-IS sibling
`plan/learned/929-isis-3-l2-transport.md`).

The raw socket requires `CAP_NET_RAW`. A registered doctor check
`doctor-ospf-raw-socket` (per `ai/rules/doctor-checks.md`, modelled on
`internal/plugins/rsvpte/doctor_linux.go`) opens and immediately closes an
`AF_INET SOCK_RAW` proto-89 socket before the daemon starts; its `doctor-*` code
is registered in `internal/core/diagnostic/codes.go`. This transport OWNS the
four `ze_ospf_packets_*` / `ze_ospf_sockets_open` Prometheus series from the
umbrella canonical Metrics table; ospf-13 only scrapes/asserts them.

Because the raw socket and IP multicast membership are Linux-only kernel
capabilities, the send/receive path is proven by a Linux-only QEMU integration
test on a veth pair (multicast send/recv), per `ai/rules/qemu-testing.md`; a
plain `.ci` covers only the user-visible doctor output.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/rsvpte/transport_linux.go` - `openRawTransport`, `Send`, `readLoop`, `stripIPv4Header` (lines 35-127); the `AF_INET SOCK_RAW` proto-number model
  -> Constraint: `unix.Socket(AF_INET, SOCK_RAW, 89)`; kernel builds the outgoing IP header (`IP_HDRINCL` off); a send supplies only the OSPF payload; on receive strip the IPv4 header via IHL (`stripIPv4Header`) and deliver `(src, payload)`; `CAP_NET_RAW` required
  -> Decision: RSVP-TE binds the socket to the local router-id address for deterministic source selection; OSPF instead selects the source per-interface (`IP_MULTICAST_IF` / per-interface socket) so multicast leaves the right link, and learns the receiving ifindex on RX
- [ ] `internal/plugins/rsvpte/doctor_linux.go` - `rsvpRawSocketAvailable` (lines 13-22); open+close raw-socket probe
  -> Constraint: model `doctor-ospf-raw-socket` on this: open `AF_INET SOCK_RAW` proto 89, close immediately, EPERM (no `CAP_NET_RAW`) -> false
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` (lines 72-87) + `internal/component/iface/events/events.go`
  -> Constraint: subscribe to iface events; `EventUp`/`EventDown` drive per-interface open/close, multicast (re)join/leave, and engine teardown; the EventBus handler must not block on I/O (bounded queue + periodic rescan backstop, as in `plan/learned/929-isis-3-l2-transport.md`)
- [ ] `docs/research/ospf-implementation-guide.md` sec 2 "Protocol and Addressing" (lines 83-97)
  -> Constraint: IP protocol 89; destination is `224.0.0.5` (all routers), `224.0.0.6` (DR/BDR senders to DROther), or unicast (some retransmits / DD on P2P); IP TTL 1 (link-local), so packets cannot be routed off the link
- [ ] `docs/research/ospf-implementation-guide.md` sec 7 "Network Types and Interface Model" (lines 470-518)
  -> Constraint: broadcast joins both groups via the DR/BDR; point-to-point uses `224.0.0.5` only and elects no DR; only Broadcast and Point-to-Point are in v1 scope (NBMA/P2MP/virtual-link/loopback future) -- the transport must support both the multicast and the unicast destination cases
- [ ] `docs/research/ospf-implementation-guide.md` sec 9 "Concurrency and I/O Model" (lines 548-591)
  -> Constraint: per-interface RX/TX goroutines (the guide's Model B per-interface split); a slow engine/SPF must not back up the socket (accept into a channel and process asynchronously); the interface TX queue is an ordered FIFO per interface
- [ ] `ai/rules/doctor-checks.md`, `ai/rules/qemu-testing.md`
  -> Constraint: owning-package doctor check + `doctor-ospf-raw-socket` code in `internal/core/diagnostic/codes.go` + unit + functional test; linux-only code MUST ship a QEMU integration test (hardware-only is not a skip reason)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`
  -> Constraint: the received payload is a view into the receive buffer (copied out before queueing, as RSVP-TE does); send writes into a caller-supplied / reused buffer; no per-datagram allocation on the hot path beyond the one queue copy
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md`
  -> Constraint: all transport code + the doctor check live under `internal/plugins/ospf/transport/`; the transport holds no protocol `Type` switch (dispatch is ospf-4)

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2 base: §D.3 (addressing), the AllSPFRouters/AllDRouters multicast groups, IP protocol 89, TTL 1
  -> Constraint: send to `224.0.0.5` (AllSPFRouters) from all routers and `224.0.0.6` (AllDRouters) only as DR/BDR; TTL 1; the transport carries the OSPF packet as the raw IP payload (kernel builds the IP header)

**Key insights:** (minimal context to resume after compaction)
- OSPF is `AF_INET SOCK_RAW` on IP protocol 89: the in-tree RSVP-TE pattern (proto 46) PLUS IP multicast membership. No new low-level capability beyond multicast join/leave.
- Join `224.0.0.5` on every enabled interface; join `224.0.0.6` ONLY when DR/BDR on that interface; leave on teardown or DR/BDR role change. Per-interface via `ip_mreqn`/ifindex.
- `IP_MULTICAST_TTL`/TTL = 1 (link-local), `IP_MULTICAST_IF` per-interface source selection, `IP_MULTICAST_LOOP` off. Send to multicast or unicast as directed by the engine. The transport NEVER parses/alters OSPF payload bytes.
- Kernel builds the outgoing IP header (`IP_HDRINCL` off); on receive strip the IPv4 header via IHL and deliver `(ifindex, src, payload)`. Dispatch by common-header `Type` is ospf-4, not here.
- `CAP_NET_RAW` required; `doctor-ospf-raw-socket` guards startup. Multicast receive on a raw socket may need explicit `IP_ADD_MEMBERSHIP` per interface (umbrella assumption A-4); validate on veth.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/plugins/rsvpte/transport_linux.go` - the only `AF_INET SOCK_RAW` proto-number user in Ze; opens proto 46, kernel builds the IP header, `stripIPv4Header` strips on receive, unicast only, binds to the local router-id address
  -> Constraint: reuse the socket open / IP-header-strip shape verbatim (proto 89 instead of 46); do NOT reuse the single-address bind (OSPF is per-interface multicast, not a single unicast source) and ADD multicast membership + multicast socket options
- [ ] `internal/plugins/rsvpte/doctor_linux.go` - `rsvpRawSocketAvailable` open+close probe returning false on EPERM
  -> Constraint: model `doctor-ospf-raw-socket` on this exactly, proto 89
- [ ] `internal/component/iface/events/events.go` - interface event namespace and `EventUp`/`EventDown` constants
  -> Constraint: drive per-interface lifecycle from these events, not from a transport-private poll loop; bounded worker queue + periodic rescan backstop (the IS-IS sibling pattern)
- [ ] Ze has no OSPF and no IP-multicast-membership user; RSVP-TE's raw-IP transport is unicast-only, so multicast join/leave and the multicast socket options are net-new in this transport
  -> Constraint: this transport is the first IP multicast group member; nothing existing to extend for membership

**Behavior to preserve:**
- RSVP-TE raw-socket code unchanged: OSPF adds a sibling transport, it does not refactor RSVP-TE in place (unless a shared raw-IP helper is extracted cleanly, which is not required by this spec).
- iface EventBus semantics unchanged: OSPF is a new subscriber only.
- Existing doctor checks and `internal/core/diagnostic/codes.go` entries unchanged; a new `doctor-ospf-raw-socket` code is appended alongside `doctor-rsvpte-rawsock-unavailable` and `doctor-isis-raw-socket`.

**Behavior to change:**
- New package `internal/plugins/ospf/transport/` with a backend interface, a Linux `AF_INET SOCK_RAW` proto-89 backend, and a non-Linux stub.
- New `doctor-ospf-raw-socket` diagnostic code and a registered doctor check.
- The four `ze_ospf_packets_*` / `ze_ospf_sockets_open` Prometheus series registered here.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound: raw IPv4 datagrams (IP proto 89) arrive on enabled interfaces via an `AF_INET SOCK_RAW` socket joined to `224.0.0.5` (and `224.0.0.6` when DR/BDR); bytes plus the receiving ifindex and source address come from `Recvfrom`.
- Outbound: the OSPF engine (ospf-5/6/7) hands a final OSPF packet plus an interface reference and a destination selector (a multicast group `224.0.0.5`/`224.0.0.6` or a unicast neighbour address); the transport sends it as the raw IP payload without altering the bytes (the kernel builds the IP header).
- Lifecycle: iface EventBus `interface/up` and `interface/down` events open and close per-interface RX/TX; a DR/BDR role-change signal from ospf-5 triggers `224.0.0.6` join/leave.

### Transformation Path
1. **Open:** on `interface/up` for a configured/enabled interface, resolve the logical interface through the iface resolver (`os-name` / `mac/match`), open or attach the raw socket on the resolved kernel device, set `IP_MULTICAST_TTL` = 1, `IP_MULTICAST_IF` to this interface, `IP_MULTICAST_LOOP` off, join `224.0.0.5` via `IP_ADD_MEMBERSHIP`, start RX and TX goroutines.
2. **Receive:** `Recvfrom` -> full datagram + source address + receiving ifindex -> `stripIPv4Header` (IHL nibble) -> deliver `(ifindex, src netip.Addr, payload []byte)` to the engine (payload copied out of the shared receive buffer before queueing). The transport hands raw OSPF packets up; it does NOT switch on the common-header `Type` (that dispatcher is ospf-4).
3. **Send:** final engine OSPF packet + interface + destination selector -> `Sendto` to `224.0.0.5`/`224.0.0.6`/unicast on the per-interface socket; the kernel builds the IP header with TTL 1 for multicast. The transport does NOT pad and does NOT alter the OSPF payload bytes.
4. **DR/BDR membership:** when ospf-5 signals this router became DR or BDR on an interface, join `224.0.0.6` on that interface; on losing the role or on teardown, leave it (`IP_DROP_MEMBERSHIP`).
5. **Close:** on `interface/down`, stop RX/TX goroutines, leave joined groups, close the per-interface socket, signal the engine to tear down adjacencies on that interface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> transport | raw `AF_INET SOCK_RAW` proto-89 IPv4 datagrams; IP header stripped on RX; source + ifindex from `Recvfrom` | [ ] |
| transport <-> OSPF engine | backend interface: open/close per interface, send (packet, dst selector), receive `(ifindex, src, payload)` channel/callback, join/leave `224.0.0.6` on DR/BDR change | [ ] |
| iface EventBus <-> transport | subscribe `interface/up` and `interface/down`; open/close per-interface RX/TX | [ ] |
| ospf-5 (DR/BDR election) <-> transport | DR/BDR role-change signal -> `224.0.0.6` join/leave | [ ] |
| transport <-> doctor | `doctor-ospf-raw-socket` check probes raw-socket open / `CAP_NET_RAW` | [ ] |

### Integration Points
- New package `internal/plugins/ospf/transport/` (backend interface, Linux backend, non-Linux stub, multicast constants, doctor check, metrics).
- Consumes `internal/plugins/ospf/types` (spec-ospf-1) where a destination selector or address needs typed values.
- Subscribes to `internal/component/iface/events` for link up/down.
- Registers a doctor check (`internal/core/diagnostic/codes.go` + owning-package registration).
- Provides the send/receive primitive that the packet receive dispatcher (ospf-4 `instance.go`) and the runtime specs (ospf-5 Hello, ospf-6 DD/LS Request, ospf-7 LS Update/LS Ack flooding) build on.

### Architectural Verification
- [ ] No bypassed layers (datagrams flow socket -> `stripIPv4Header` -> engine; the engine never touches the socket directly; the `Type` dispatcher is ospf-4)
- [ ] No unintended coupling (backend behind an interface; Linux specifics in `backend_linux.go`; RSVP-TE untouched; no IS-IS coupling)
- [ ] No duplicated functionality (one OSPF transport; the engine reuses it for Hello/DD/LSReq/LSUpdate/LSAck rather than opening its own socket; IP-header strip reuses the RSVP-TE pattern)
- [ ] Zero-copy preserved (received payload is a view copied out once before queueing; send is buffer-first; no per-datagram alloc beyond the queue copy)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | RSVP-TE's `AF_INET SOCK_RAW` pattern (kernel builds the IP header, IHL strip on receive) generalises to OSPF proto 89 with IP multicast membership added | `internal/plugins/rsvpte/transport_linux.go:35-127` | transport needs a different socket mechanism (e.g. `IP_HDRINCL`, `IP_PKTINFO` for ifindex, per-interface fan-out) | QEMU veth multicast send/recv (`TestOSPFTransportVethMulticastRoundTrip`) | confirmed: QEMU `go test -tags integration -run TestOSPFTransport` passed |
| A-2 | IP multicast receive on a raw `AF_INET SOCK_RAW` socket works on Linux with per-interface `IP_ADD_MEMBERSHIP` and no promiscuous mode | `docs/research/ospf-implementation-guide.md` sec 9; umbrella assumption A-4; RSVP-TE precedent is unicast-only so this was unverified | need `IP_MULTICAST_ALL` tuning, `PACKET`-level membership, or promiscuous mode | QEMU two-netns veth multicast receive (`TestOSPFTransportVethMulticastRoundTrip`) | confirmed with receive socket membership on `224.0.0.5` |
| A-3 | The receiving ifindex is recoverable on a raw IPv4 socket using a per-interface receive socket | `docs/research/ospf-implementation-guide.md` sec 9 per-interface model; RSVP-TE uses a single socket and does not need ifindex | RX cannot attribute a datagram to an interface; need `IP_PKTINFO`/`recvmsg` | QEMU veth round-trip asserting the receiving ifindex | confirmed: socket-per-interface model returns the receiver handle ifindex |
| A-4 | `IP_MULTICAST_TTL` = 1 + `IP_MULTICAST_IF` make multicast leave the intended interface with link-local scope, and transmit-side `IP_MULTICAST_LOOP` off suppresses local self-receipt | RFC 2328 §D.3 TTL 1; `docs/research/ospf-implementation-guide.md` sec 2 | packets routed off-link, leave the wrong interface, or the router receives its own Hellos | QEMU veth test: peer receives, sender does not loop back | confirmed: QEMU peer receives and sender receive channel stays empty |
| A-5 | `224.0.0.6` (AllDRouters) join/leave can be toggled per-interface at runtime on a DR/BDR role change without re-opening the socket | Linux `IP_ADD_MEMBERSHIP`/`IP_DROP_MEMBERSHIP` semantics | need to re-open the socket on every election change (churn) | unit test toggling membership + QEMU DR-group receive test | confirmed: `TestOSPFTransportJoinLeaveAllDRouters` and `TestOSPFTransportAllDRoutersReceive` |
| A-6 | `CAP_NET_RAW` is the only privilege needed to open the socket and join the multicast groups on the gokrazy appliance | RSVP-TE raw-socket precedent; `ai/rules/doctor-checks.md` | socket open or `IP_ADD_MEMBERSHIP` EPERM at startup | `doctor-ospf-raw-socket` check + QEMU open probe | confirmed for raw socket open in QEMU; CAP_NET_ADMIN is test-only for veth setup |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Multicast receive silently drops frames (membership not set per-interface, or needs promiscuous mode) | RX goroutine never delivers a Hello the peer sent | A-2 validation on veth; join with `IP_ADD_MEMBERSHIP` bound to the interface IPv4 address; record the outcome in the spec; avoid promiscuous mode |
| R-2 | The router receives its own multicast (`IP_MULTICAST_LOOP` left on) -> phantom self-neighbour | the engine sees a Hello sourced from its own router-id / interface | `IP_MULTICAST_LOOP` off in the socket setup; unit test asserts the option; QEMU test asserts the sender does not loop back |
| R-3 | RX cannot attribute a datagram to the receiving interface -> Hellos mis-bound to the wrong area | adjacency forms on the wrong interface, or area check (ospf-4) misfires | A-3: use `IP_PKTINFO`/`recvmsg` or a socket-per-interface; assert the ifindex in the veth round-trip test |
| R-4 | Per-interface goroutine leak on rapid link flap | goroutine count climbs under flap | bounded lifecycle tied to `interface/down`; `SO_RCVTIMEO` so RX wakes to observe stop; bounded EventBus queue + periodic rescan backstop |
| R-5 | TTL not 1 -> OSPF packets routed off the link | a remote (non-adjacent) router receives OSPF packets | `IP_MULTICAST_TTL` = 1 set in socket setup and asserted in a unit test; the kernel builds the IP header so the option, not a manual header, controls TTL |
| R-6 | `224.0.0.6` membership not toggled on DR/BDR change -> DROther never reaches the DR, or a former DR keeps receiving DR traffic | adjacency stuck because Updates to `224.0.0.6` are not received after becoming DR | A-5: per-interface join on DR/BDR signal, leave on role loss/teardown; unit test toggles membership; QEMU DR-group receive test |
| R-7 | `CAP_NET_RAW` absent at runtime | socket open or join EPERM at startup | doctor check `doctor-ospf-raw-socket` with a clear message before the daemon runs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `interface/up` event for an enabled interface | -> | transport opens raw socket, sets multicast options, joins `224.0.0.5`, starts RX/TX | `TestOSPFTransportOpenOnLinkUp` |
| engine sends a final Hello-shaped packet to `224.0.0.5` on an interface | -> | packet sent via `Sendto` as the raw IP payload (kernel builds the IP header, TTL 1) | `TestOSPFTransportSendMulticast` |
| datagram arrives on the peer interface | -> | RX strips the IP header and delivers `(ifindex, src, payload)` to the engine | `TestOSPFTransportVethMulticastRoundTrip` (`transport_integration_linux_test.go`, QEMU) |
| ospf-5 signals this router became DR on an interface | -> | transport joins `224.0.0.6` on that interface; leaves on role loss | `TestOSPFTransportJoinAllDRouters` |
| `interface/down` event | -> | transport leaves groups, closes socket, signals engine teardown | `TestOSPFTransportCloseOnLinkDown` |
| OSPF doctor check registration | -> | `doctor-ospf-raw-socket` check is registered; check body fires when an OSPF config tree is present | `TestOSPFDoctorRawSocketCheckRegistered`, `TestCheckOSPFRawSocketUnavailable`; `.ci` covers `ze explain` until OSPF config lands in spec-ospf-4 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two interfaces of a veth pair, transport opened on both, both joined `224.0.0.5` | A packet sent to `224.0.0.5` on one is received on the other with the receiving ifindex matching the receiver and the source address matching the sender; `stripIPv4Header` returns the OSPF payload only |
| AC-2 | Send a final OSPF packet to a multicast destination | The packet is delivered as the raw IP payload (the transport supplies no IP header; the kernel builds it); the OSPF bytes are sent byte-for-byte unaltered; multicast leaves with TTL 1 |
| AC-3 | An OSPF-enabled interface comes up | The per-interface socket sets `IP_MULTICAST_TTL` = 1, `IP_MULTICAST_IF` to that interface, `IP_MULTICAST_LOOP` off, and joins `224.0.0.5`; the sender does not receive its own multicast |
| AC-4 | This router becomes DR (or BDR) on an interface, then loses the role | The transport joins `224.0.0.6` on that interface on becoming DR/BDR and leaves it (`IP_DROP_MEMBERSHIP`) on losing the role or on interface teardown |
| AC-5 | Engine directs a send to a unicast neighbour address (e.g. a retransmission or DD on P2P) | The packet is unicast to that address on the interface socket; no multicast group is used; the bytes are unaltered |
| AC-6 | `interface/down` event on an open interface | RX/TX goroutines stop, joined groups are left, the socket is closed, and an engine teardown signal is emitted (no goroutine leak) |
| AC-7 | Raw socket open without `CAP_NET_RAW` | Open fails with a clear error naming `CAP_NET_RAW`; `doctor-ospf-raw-socket` reports the missing capability |
| AC-8 | Received datagram shorter than a minimal IPv4 header, or an IHL that overruns the buffer | The datagram is rejected before slicing into the OSPF payload; no panic; the `ze_ospf_packets_dropped_total` counter is incremented with a `reason` label |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up an OSPF-enabled interface | `interface/up` -> transport open -> multicast options set -> `224.0.0.5` joined -> RX/TX goroutines running | `TestOSPFTransportOpenOnLinkUp`, `TestOSPFTransportVethMulticastRoundTrip` (QEMU integration) |
| 2 | Has OSPF send a Hello to the segment | final engine packet -> `Sendto` `224.0.0.5` (kernel IP header, TTL 1) -> peer RX strips IP header -> `(ifindex, src, payload)` | `TestOSPFTransportSendMulticast`, `TestOSPFTransportVethMulticastRoundTrip` (QEMU integration) |
| 3 | This router is elected DR on a LAN | ospf-5 DR signal -> transport joins `224.0.0.6` -> DROther packets to `224.0.0.6` are received | `TestOSPFTransportJoinAllDRouters`, QEMU DR-group receive case |
| 4 | Looks up the OSPF raw-socket readiness diagnostic before OSPF config syntax exists | `ze explain doctor-ospf-raw-socket` -> diagnostic metadata with `CAP_NET_RAW` guidance; registered check execution is unit-tested with a synthetic OSPF config tree until spec-ospf-4 lands config syntax | `ospf-doctor-raw-socket.ci`, `TestOSPFDoctorRawSocketCheckRegistered`, `TestCheckOSPFRawSocketUnavailable` |
| 5 | Takes the interface down | `interface/down` -> leave groups -> circuit close -> engine tears down adjacencies | `TestOSPFTransportCloseOnLinkDown` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStripIPv4Header` | `internal/plugins/ospf/transport/transport_test.go` | strips the IPv4 header via the IHL nibble, returns the OSPF payload + source; rejects a short datagram and an IHL overrun without panic | |
| `TestMulticastGroupConstants` | `internal/plugins/ospf/transport/multicast_test.go` | `AllSPFRouters` = `224.0.0.5`, `AllDRouters` = `224.0.0.6`; bytes exact | |
| `TestSendDoesNotAlterPayload` | `internal/plugins/ospf/transport/transport_test.go` | the transport adds no IP header and no padding; the final OSPF packet is sent byte-for-byte (fake backend captures the payload) | |
| `TestSocketOptionsMulticast` | `internal/plugins/ospf/transport/transport_test.go` | on open, the backend requests `IP_MULTICAST_TTL` = 1, `IP_MULTICAST_IF` = the interface, `IP_MULTICAST_LOOP` off (asserted via a fake/recording backend) | |
| `TestJoinLeaveAllDRouters` | `internal/plugins/ospf/transport/transport_test.go` | a DR/BDR signal joins `224.0.0.6` on the interface; a role-loss/teardown signal leaves it; idempotent join/leave | |
| `TestOSPFTransportOpenOnLinkUp` | `internal/plugins/ospf/transport/transport_test.go` | `interface/up` opens the interface, sets options, joins `224.0.0.5`, starts RX/TX (wiring) | |
| `TestOSPFTransportSendMulticast` | `internal/plugins/ospf/transport/transport_test.go` | engine packet to `224.0.0.5` -> backend `Sendto` with the right destination (wiring) | |
| `TestOSPFTransportCloseOnLinkDown` | `internal/plugins/ospf/transport/transport_test.go` | `interface/down` leaves groups, closes the socket, signals teardown, no leak (wiring) | |
| `TestOSPFTransportReceiveDispatchByIfindex` | `internal/plugins/ospf/transport/transport_test.go` | a received datagram is delivered with the receiving ifindex (fake backend injects ifindex) | |
| `TestOSPFDoctorRawSocketUnavailable` | `internal/plugins/ospf/transport/doctor_test.go` | the check fires when OSPF is configured and the raw socket cannot be opened; emits `doctor-ospf-raw-socket` | |
| `TestOSPFDoctorCodeRegistered` | `internal/plugins/ospf/transport/doctor_test.go` | `doctor-ospf-raw-socket` is registered in `internal/core/diagnostic/codes.go` and explainable | |
| `TestOSPFTransportMetricsSeries` | `internal/plugins/ospf/transport/metrics_test.go` | exactly the four umbrella-owned series register: `ze_ospf_packets_sent_total`, `ze_ospf_packets_received_total`, `ze_ospf_packets_dropped_total`, `ze_ospf_sockets_open` | |
| `TestResolveOSPFInterfaceUsesIfaceResolverOSName` | `internal/plugins/ospf/transport/backend_linux_test.go` | logical OSPF names resolve through the shared iface resolver before the backend binds sockets, so `os-name` selectors match IS-IS behavior | |
| `TestLinuxInterfaceCountsMalformedReceiveDrop` | `internal/plugins/ospf/transport/backend_linux_test.go` | malformed IPv4 datagrams increment `ze_ospf_packets_dropped_total{reason="malformed-ipv4"}` before discard | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Received datagram total length | 20..65535 (min IPv4 header 20) | 65535 | < 20 (no IPv4 header) | > 65535 (datagram cap) |
| IHL header length (byte 0 low nibble x4) | 20..60 | 60 | < 20 (illegal IHL) | > received length (overrun -> reject) |
| OSPF payload after strip | 0..(len - IHL) | len - IHL | n/a | n/a (slice bounded by received length) |
| `IP_MULTICAST_TTL` | 1 only (OSPF) | 1 | 0 (would not leave host) | > 1 (would route off-link) |
| Multicast group last octet | 5 (AllSPFRouters) / 6 (AllDRouters) | 6 | n/a | any other rejected as a programming error |

### Functional Tests
`.ci` files cover ONLY user-visible config/doctor output. The raw IP / multicast
send-receive path is a kernel-capability path and is proven by a Linux-only QEMU
integration test (see QEMU Integration Tests below), NOT by a `.ci` file
(`ai/rules/qemu-testing.md`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-doctor-raw-socket` | `test/ospf/ospf-doctor-raw-socket.ci` | `ze doctor --json` / `ze explain` exposes the `doctor-ospf-raw-socket` result | |

### QEMU Integration Tests (MANDATORY for linux-only code -- `ai/rules/qemu-testing.md`)
| Test | File | Build tags | End-User Scenario | Status |
|------|------|------------|-------------------|--------|
| `TestOSPFTransportVethMulticastRoundTrip` | `internal/plugins/ospf/transport/transport_integration_linux_test.go` | `integration && linux` | open the transport on a veth pair in a netns, join `224.0.0.5` on both, send a packet to `224.0.0.5` on one, receive it on the peer with the correct ifindex and source; the sender does not loop back (validates A-1, A-2, A-3, A-4) | |
| `TestOSPFTransportAllDRoutersReceive` | `internal/plugins/ospf/transport/transport_integration_linux_test.go` | `integration && linux` | after a DR/BDR join of `224.0.0.6`, a packet sent to `224.0.0.6` is received; after leave, it is not (validates A-5) | |
| `TestOSPFTransportRawSocketCap` | `internal/plugins/ospf/transport/transport_integration_linux_test.go` | `integration && linux` | raw `AF_INET SOCK_RAW` proto-89 open succeeds under `CAP_NET_RAW`; `t.Skip` when the capability is absent (validates A-6) | |

Because `transport_integration_linux_test.go` carries the
`//go:build integration && linux` tag, the `ze-qemu-integration-test` Makefile
target picks it up automatically: `ZE_QEMU_INTEGRATION_PKGS` is DERIVED from that
exact tag by a `grep` over `internal/`/`cmd/` (`mk/test-integration.mk`), so no
Makefile edit is needed and a new tagged package cannot be silently omitted. The
QEMU all-tests evidence script `scripts/evidence/qemu-all-tests.sh`, however,
HARDCODES its integration-test package list and does NOT derive from tags, so the
`internal/plugins/ospf/transport/...` package MUST be added there explicitly
(Files to Modify) or these tests will be silently skipped in the all-tests
evidence run. The tests use a veth pair in a network namespace (`t.Skip` when
`CAP_NET_ADMIN`/`CAP_NET_RAW` are missing), exactly as the IS-IS sibling
(`plan/learned/929-isis-3-l2-transport.md`) does.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred to ospf-13) | `test/interop/scenarios/` | FRR ospfd | FRR receives Ze's `224.0.0.5` Hellos and vice versa; full adjacency needs the ISM (ospf-5) and NSM (ospf-6) | |

Wire-level transport correctness (multicast send/receive, IP-header strip, TTL 1,
membership) is proven here by unit tests and the QEMU veth integration tests.
End-to-end FRR interop requires the ISM/NSM (ospf-5/ospf-6) and is owned by
ospf-13; this is not a deferral of this spec's own acceptance criteria.

### Future (if deferring any tests)
- FRR `ospfd` interop of full OSPF packets over this transport: owned by ospf-13 (needs ospf-5/ospf-6). Not a deferral of ospf-3 ACs.
- BSD / VPP backends behind the same interface: out of scope for v1 per the umbrella.

## Files to Modify
- `internal/core/diagnostic/codes.go` - add the `doctor-ospf-raw-socket` code (title, description, examples), alongside the existing `doctor-rsvpte-rawsock-unavailable` and `doctor-isis-raw-socket`. ospf-3 OWNS this code; ospf-13 surfaces but must not re-register it
- `internal/component/iface/events/events.go` - no change expected; consume `EventUp`/`EventDown` (read-only)
- `scripts/evidence/qemu-all-tests.sh` - add `./internal/plugins/ospf/transport/...` to the hardcoded integration-tests package list; this script does not derive packages from build tags (unlike `ze-qemu-integration-test`), so without this edit the transport integration tests are skipped in the all-tests evidence

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | per-interface OSPF enrolment config is owned by ospf-4 |
| YANG validation constraints | No | n/a in this spec |
| YANG custom validators | No | n/a in this spec |
| CLI commands/flags | No | transport stats surfaced via ospf-13 `show ip ospf interface` |
| CLI grammar (action before identifier) | No | n/a in this spec |
| Editor autocomplete | No | n/a in this spec |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-doctor-raw-socket.ci` (user-visible doctor output); raw multicast send/receive proven by QEMU integration test `transport_integration_linux_test.go` |
| Pipe completeness | No | n/a (no command output in this spec) |
| Env var registration | No | n/a in this spec |
| Doctor check for runtime dependencies | Yes | `doctor-ospf-raw-socket` (raw socket open / `CAP_NET_RAW`), owning-package registration + `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | this spec OWNS and registers exactly the transport series from the umbrella `## Shared Contracts` "Metrics (canonical)" table: `ze_ospf_packets_sent_total{interface,type}`, `ze_ospf_packets_received_total{interface,type}`, `ze_ospf_packets_dropped_total{interface,reason}`, `ze_ospf_sockets_open` (gauge, no labels). Registration is here (per-owner), NOT central in ospf-13; ospf-13 only scrapes/asserts them |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | transport is internal; the user-facing OSPF feature row is owned by ospf-13 |
| 2 | Config syntax changed? | No | config owned by ospf-4 |
| 3 | CLI command added/changed? | No | owned by ospf-13 |
| 4 | API/RPC added/changed? | No | none in this spec |
| 5 | Plugin added/changed? | Yes | edge plugin under `internal/plugins/ospf/`; user-facing docs owned by ospf-13 |
| 6 | Has a user guide page? | No | `docs/guide/ospf.md` owned by ospf-13 |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` (IP proto 89 transport, `224.0.0.5`/`224.0.0.6` multicast, TTL 1, IP-header strip) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§D.3 addressing, multicast groups, TTL 1) |
| 10 | Test infrastructure changed? | Yes | the raw IP / multicast send/receive tests are QEMU integration tests (`transport_integration_linux_test.go`, `integration && linux`), documented under QEMU/integration testing (`ai/rules/qemu-testing.md`), NOT plain `.ci`; only the user-visible doctor test `test/ospf/ospf-doctor-raw-socket.ci` is a `docs/functional-tests.md` entry |
| 11 | Affects daemon comparison? | No | owned by ospf-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new OSPF transport layer; first IP multicast group member) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (the four `ze_ospf_packets_*` / `ze_ospf_sockets_open` rows) |
| 15 | Registered plugin/event/command/capability changed? | Yes | doctor code `doctor-ospf-raw-socket` registered; note in `docs/guide/status.md` via ospf-13 |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/plugins/ospf/transport/transport.go` - the `Backend` interface (open/close per interface, send `(packet, destination selector)`, receive `(ifindex, src, payload)`, join/leave `224.0.0.6` on DR/BDR change), the `Transport` orchestrator (per-interface registry, iface EventBus subscription with a bounded worker queue + periodic rescan backstop, send fan-out, receive fan-in), and the platform-neutral `stripIPv4Header` (IHL strip). The transport does NOT parse the OSPF `Type` and does NOT alter the payload
- `internal/plugins/ospf/transport/multicast.go` - `AllSPFRouters` (`224.0.0.5`) / `AllDRouters` (`224.0.0.6`) constants and the destination-selector type (multicast group or unicast address)
- `internal/plugins/ospf/transport/backend_linux.go` - `//go:build linux` `AF_INET SOCK_RAW` proto-89 backend: shared iface resolver lookup (`os-name` / `mac/match`) before socket bind, socket open, `IP_MULTICAST_TTL` = 1, `IP_MULTICAST_IF`, `IP_MULTICAST_LOOP` off, `IP_ADD_MEMBERSHIP`/`IP_DROP_MEMBERSHIP` using the interface IPv4 address, `Recvfrom` RX with ifindex attribution by per-interface socket, `Sendto` TX, `SO_RCVTIMEO` for stop-signal wakeups
- `internal/plugins/ospf/transport/backend_other.go` - `//go:build !linux` stub returning a not-supported error so non-Linux builds compile and config/unit tests run
- `internal/plugins/ospf/transport/doctor.go` - platform-neutral doctor check body (fires when OSPF is configured and the raw socket is unavailable), modelled on the RSVP-TE / IS-IS split
- `internal/plugins/ospf/transport/doctor_linux.go` - `//go:build linux` raw-socket probe (`AF_INET SOCK_RAW` proto 89 open+close; EPERM -> unavailable), modelled on `internal/plugins/rsvpte/doctor_linux.go`
- `internal/plugins/ospf/transport/doctor_other.go` - `//go:build !linux` probe stub
- `internal/plugins/ospf/transport/register.go` - doctor-check registration from `init()` via `diagnostic.RegisterDoctorCheck`
- `internal/plugins/ospf/transport/metrics.go` - the four transport-owned series (`ze_ospf_packets_sent_total`, `ze_ospf_packets_received_total`, `ze_ospf_packets_dropped_total`, `ze_ospf_sockets_open`)
- `internal/plugins/ospf/transport/transport_test.go` - strip, send-does-not-alter, socket-options, join/leave, wiring (open/send/close), receive-by-ifindex, boundary tests
- `internal/plugins/ospf/transport/multicast_test.go` - multicast group constants byte-exact
- `internal/plugins/ospf/transport/doctor_test.go` - doctor check + registration unit tests
- `internal/plugins/ospf/transport/metrics_test.go` - exact-series registration test
- `internal/plugins/ospf/transport/transport_integration_linux_test.go` - `//go:build integration && linux` QEMU veth multicast round-trip + AllDRouters receive + `CAP_NET_RAW` capability probe (the raw IP / multicast path lives here, NOT in a `.ci` file)
- `test/ospf/ospf-doctor-raw-socket.ci` - functional doctor-check test (user-visible `ze doctor --json` / `ze explain` output only)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-qemu-integration-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- define the `Backend` interface, destination-selector type, and per-interface registry; subscribe to the iface EventBus; write failing wiring tests
   - Tests: `TestOSPFTransportOpenOnLinkUp`, `TestOSPFTransportCloseOnLinkDown`, `TestOSPFTransportSendMulticast`
   - Files: `internal/plugins/ospf/transport/transport.go`, `multicast.go`, `backend_other.go` stub
   - Verify: `interface/up`/`interface/down` reach the transport; backend methods are stubs so wiring tests fail on missing I/O
2. **Phase: IP-header strip + send-does-not-alter** -- platform-neutral `stripIPv4Header` (IHL) and the send path that adds no IP header and no padding
   - Tests: `TestStripIPv4Header`, `TestSendDoesNotAlterPayload`, `TestMulticastGroupConstants`, boundary tests
   - Files: `transport.go`, `multicast.go`
   - Verify: IHL strip returns the OSPF payload + source; a short datagram / IHL overrun is rejected without panic; the final packet is sent byte-for-byte
3. **Phase: Linux `AF_INET SOCK_RAW` backend** -- proto-89 socket open, multicast socket options (`IP_MULTICAST_TTL` = 1, `IP_MULTICAST_IF`, `IP_MULTICAST_LOOP` off), `IP_ADD_MEMBERSHIP` for `224.0.0.5`, `Recvfrom`/`recvmsg` RX with ifindex recovery, `Sendto` TX, `SO_RCVTIMEO`
   - Tests: `TestSocketOptionsMulticast`, `TestOSPFTransportReceiveDispatchByIfindex`, `transport_integration_linux_test.go` (QEMU veth round-trip)
   - Files: `backend_linux.go`
   - Verify: packet sent on one veth received on the peer with the correct ifindex and source; sender does not loop back; resolve A-2 (membership) / A-3 (ifindex) and record the outcome
4. **Phase: AllDRouters membership on DR/BDR change** -- per-interface `224.0.0.6` join on a DR/BDR signal, leave on role loss / teardown
   - Tests: `TestJoinLeaveAllDRouters`, `TestOSPFTransportJoinAllDRouters`, QEMU `TestOSPFTransportAllDRoutersReceive`
   - Files: `transport.go`, `backend_linux.go`
   - Verify: joining `224.0.0.6` makes DROther-addressed packets receivable; leaving stops them; join/leave idempotent
5. **Phase: Doctor check + metrics** -- `doctor-ospf-raw-socket` code + check, the four transport Prometheus series
   - Tests: `TestOSPFDoctorRawSocketUnavailable`, `TestOSPFDoctorCodeRegistered`, `TestOSPFTransportMetricsSeries`
   - Files: `doctor.go`, `doctor_linux.go`, `doctor_other.go`, `register.go`, `metrics.go`, `internal/core/diagnostic/codes.go`
   - Verify: `ze doctor --json` exposes the code; `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered'` passes; exactly four series register
6. **Functional + QEMU integration tests** -> `test/ospf/ospf-doctor-raw-socket.ci` (user-visible doctor output) and `transport_integration_linux_test.go` (raw multicast send/receive on veth). The `ze-qemu-integration-test` target auto-derives this package from its `//go:build integration && linux` tag (`ZE_QEMU_INTEGRATION_PKGS`, no Makefile edit); add the package explicitly only to the hardcoded list in `scripts/evidence/qemu-all-tests.sh`
7. **RFC refs** -> Add `// RFC 2328 Section D.3 ...` comments on the addressing / multicast / TTL code
8. **Full verification** -> `make ze-verify` + `make ze-qemu-integration-test`
9. **Complete spec** -> fill audit tables, write learned summary to `plan/learned/NNN-ospf-3-ip-transport.md`; two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has implementation with file:line |
| Feature completeness | Open/close + send (multicast + unicast) + receive (ifindex dispatch) + `224.0.0.5` join + `224.0.0.6` DR/BDR join/leave + TTL 1 + loop-off + link-event lifecycle + doctor all present; the backend interface lets a second backend drop in |
| Correctness | `AF_INET SOCK_RAW` proto 89; kernel builds the IP header (no manual header); IHL strip on receive; multicast groups `224.0.0.5`/`224.0.0.6` byte-exact; TTL 1; `IP_MULTICAST_LOOP` off; the OSPF payload is sent byte-for-byte |
| Naming | Package `transport`; doctor code `doctor-ospf-raw-socket`; backend files `backend_linux.go` / `backend_other.go`; metric series exactly the four umbrella names |
| Data flow | The engine never touches the socket; datagrams flow socket -> `stripIPv4Header` -> engine; the `Type` dispatcher is ospf-4; iface events drive lifecycle; DR/BDR signal drives `224.0.0.6` membership |
| CLI grammar | n/a (no CLI in this spec) |
| Doctor checks | `doctor-ospf-raw-socket` registered per `ai/rules/doctor-checks.md`; unit + functional test present |
| YANG validation | n/a (config owned by ospf-4) |
| Prometheus counters | exactly the four umbrella series defined and registered here; names match the canonical table |
| Rule: qemu-testing | the Linux backend has an `integration && linux` QEMU test; no hardware-only skip |
| Rule: plugin-self-containment | all transport code + the doctor check under `internal/plugins/ospf/transport/` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Transport package | `ls internal/plugins/ospf/transport/` |
| Linux backend + non-linux stub | `ls internal/plugins/ospf/transport/backend_linux.go internal/plugins/ospf/transport/backend_other.go` |
| Doctor code registered | `grep doctor-ospf-raw-socket internal/core/diagnostic/codes.go` |
| Four metric series owned here | `grep -E 'ze_ospf_packets_(sent|received|dropped)_total|ze_ospf_sockets_open' internal/plugins/ospf/transport/metrics.go` |
| Functional test (user-visible) | `ls test/ospf/ospf-doctor-raw-socket.ci` |
| QEMU integration test (raw IP / multicast) | `ls internal/plugins/ospf/transport/transport_integration_linux_test.go`; auto-derived into `ze-qemu-integration-test` via its `integration && linux` build tag (`ZE_QEMU_INTEGRATION_PKGS` in `mk/test-integration.mk`); `grep ospf/transport scripts/evidence/qemu-all-tests.sh` confirms the explicit add to the hardcoded all-tests list |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every received datagram validated (min IPv4 header length, IHL bounds) before slicing into the OSPF payload; reject not panic; the dropped counter increments with a reason |
| Resource exhaustion | Bounded receive buffer; per-interface goroutine count tied to link state; no unbounded receive queue; bounded EventBus worker queue |
| Privilege | `CAP_NET_RAW` only; the doctor check surfaces the requirement; consider dropping the capability after socket open if feasible |
| Spoofing | Source address and receiving ifindex recorded for the engine to apply area / neighbour checks (version/area/checksum/auth enforcement is ospf-4 / ospf-12, not the transport) |
| Off-link reach | TTL 1 on multicast (`IP_MULTICAST_TTL` = 1) so OSPF packets cannot be routed off the link; assert the option |
| Crafted datagrams | A malformed IHL or a truncated datagram must not over-read the receive buffer; fuzz the strip path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; re-check addressing against RFC 2328 §D.3 and the RSVP-TE model |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional/QEMU test fails | Check AC; if A-2 (multicast membership) or A-3 (ifindex) is the cause, fix the receive membership or per-interface socket attribution and record |
| Audit finds missing AC | Back to the relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
The OSPF transport is a pure byte pipe over an `AF_INET SOCK_RAW` proto-89 socket:
it adds ONLY IP multicast membership and per-interface source/TTL selection on top
of the proven RSVP-TE raw-IP pattern, and it never inspects or alters the OSPF
payload. The single net-new capability beyond RSVP-TE is IP multicast group
membership (`224.0.0.5` always, `224.0.0.6` only as DR/BDR), which makes the
membership lifecycle (tied to interface up/down and DR/BDR election) the one place
where correctness risk concentrates -- everything else is the RSVP-TE model with
the destination chosen by the engine.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `AF_INET SOCK_RAW` proto 89, kernel builds the IP header (`IP_HDRINCL` off) | `IP_HDRINCL` with a manually-built IP header | reuses the RSVP-TE model verbatim; the kernel handles the IP header and TTL via `IP_MULTICAST_TTL`; no manual checksum/header errors |
| Per-interface multicast source via `IP_MULTICAST_IF` (and ifindex on RX) | RSVP-TE's single bind to the router-id address | OSPF multicast must leave the intended interface and RX must attribute a datagram to an interface; a single unicast bind cannot do either |
| Logical interface names resolve through the shared iface resolver before socket bind | Direct `net.InterfaceByName(name)` lookup in the OSPF backend | matches IS-IS: OSPF config names remain logical Ze names while `os-name` and `mac/match` select the kernel device |
| Backend behind a Go interface; Linux backend + non-Linux stub | A single `_linux.go` with no abstraction | lets a future BSD/VPP backend drop in (umbrella design principle); non-Linux builds compile and run config/unit tests |
| `224.0.0.6` membership toggled per-interface on a DR/BDR signal | Always join both groups | RFC 2328 §D.3: only the DR and BDR join AllDRouters; joining unconditionally would receive DR-only traffic when DROther |
| Transport never parses the OSPF `Type`; dispatch is ospf-4 | Transport switches on the common-header `Type` | umbrella "Packet receive dispatcher" contract: the transport hands raw packets up; the `Type` dispatcher + validation live in ospf-4 `instance.go` |
| Split doctor into `doctor.go` (neutral) + `doctor_linux.go`/`doctor_other.go` (probe) | A single `doctor_linux.go` | platform-neutral check body testable on any OS; the probe is the only platform-specific part (the IS-IS sibling did the same per `ai/rules/doctor-checks.md`) |

## Known Limitations
- v1 ships only the Linux `AF_INET SOCK_RAW` backend; BSD/VPP backends are out of scope (umbrella).
- Only the Broadcast and Point-to-Point network types are supported (umbrella scope): the transport supports the `224.0.0.5`/`224.0.0.6` multicast and unicast destination cases those need; NBMA/P2MP/virtual-link/loopback are future and would add their own destination handling.
- Full FRR `ospfd` interop over this transport is proven in ospf-13 (needs the ospf-5 ISM and ospf-6 NSM); ospf-3 proves transport correctness by unit tests and the QEMU veth multicast send/receive integration test.
- The transport does NOT validate version/area/checksum/auth and does NOT dispatch by packet `Type`; those are owned by ospf-4 (`instance.go`) and ospf-12 (auth).

## RFC Documentation

Add `// RFC 2328 Section D.3: "<quoted requirement>"` above enforcing code.
MUST document: IP protocol 89 transport, the AllSPFRouters (`224.0.0.5`) /
AllDRouters (`224.0.0.6`) multicast groups and which routers join each, IP TTL 1
(link-local scope), the receive-path IPv4-header strip (IHL), and the
`IP_MULTICAST_LOOP`-off / per-interface source-selection requirements.

## Implementation Summary

### What Was Implemented
- `internal/plugins/ospf/transport/` with platform-neutral `Transport`, `Backend`, `InterfaceHandle`, `RawPacket`, interface lifecycle, receive fan-in, multicast/unicast send, AllDRouters join/leave, packet type metric labels, non-Linux stub, Linux raw IPv4 backend, metrics, and doctor registration.
- Linux backend opens one receive and one transmit `AF_INET/SOCK_RAW` protocol 89 socket per resolved kernel interface. RX owns multicast membership; TX owns TTL 1, `IP_MULTICAST_IF`, and loop suppression.
- `doctor-ospf-raw-socket` registered in `internal/core/diagnostic/codes.go` and imported by `cmd/ze/ze_core_dispatch.go`.
- QEMU integration tests create a two-netns veth topology, prove AllSPFRouters receive, AllDRouters join/leave receive, raw socket capability, ifindex attribution, source address, and sender non-loop.
- Functional fixture `test/ospf/ospf-doctor-raw-socket.ci`, `ze-test ospf` registration, `make ze-ospf-test`, and QEMU all-tests package inclusion.

### Bugs Found/Fixed
- Same-netns veth was an invalid test topology for raw IPv4 multicast. Fix: peer interface moved into a second netns and the test restores the original namespace immediately after `netns.NewNamed`.
- A single raw socket with `IP_MULTICAST_LOOP=0` conflated TX and RX behavior. Fix: split per-interface RX and TX sockets so TX suppresses self-loop while RX can receive peer traffic generated by the QEMU host kernel.
- `IP_ADD_MEMBERSHIP` using `IPMreqn` was replaced with `IPMreq` bound to the interface IPv4 address after QEMU validation matched the Linux raw multicast receive behavior.
- The Linux backend initially resolved OSPF interface names directly with `net.InterfaceByName`. Fix: resolve through the shared iface resolver, mirroring IS-IS, so logical names and `os-name` selectors bind to the intended kernel device.
- Malformed receive datagrams initially bypassed drop metrics. Fix: pass a backend drop recorder and count `malformed-ipv4` before discard.
- `ze_ospf_sockets_open` now reports two raw sockets per open interface, matching the split RX/TX backend.

### Documentation Updates
- `docs/architecture/wire/ospf.md` documents protocol 89 transport, multicast groups, TTL 1, IHL strip, and layer ownership with source anchors.
- `docs/architecture/core-design.md` records the OSPF raw IPv4 transport layer with source anchors.
- `docs/plugin-development/metrics.md` lists the four OSPF transport metrics.
- `docs/functional-tests.md`, `mk/test-functional.mk`, and `Makefile` list the OSPF functional fixture and make target.
- `make ze-doc-test` passed.

### Deviations from Plan
- The Linux backend uses separate RX and TX raw sockets per interface. This preserves the approved raw IPv4 socket design while avoiding transmit-side multicast loop suppression from hiding QEMU peer traffic on the receive socket.
- The QEMU test uses a two-netns veth topology, not two interfaces in one namespace. That matches two routers on a link and avoids Linux local multicast loopback semantics in a single namespace.
- The backend resolves logical names through the iface resolver instead of direct `net.InterfaceByName`. This is consistent with IS-IS and required for `os-name` / `mac/match` interface selectors.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Raw IPv4 proto 89 transport package | done | `internal/plugins/ospf/transport/` | Linux backend plus non-Linux stub |
| Kernel-built IP header, OSPF bytes unchanged | done | `Transport.SendPacket`, `linuxInterface.Send` | Unit tests assert byte-for-byte payload |
| IHL strip on receive | done | `StripIPv4Header`, `linuxInterface.readLoop` | Unit and QEMU coverage |
| AllSPFRouters join and AllDRouters toggle | done | `OpenInterface`, `JoinAllDRouters`, `LeaveAllDRouters` | Unit and QEMU coverage |
| Interface up/down lifecycle | done | `EnableInterface`, `HandleLinkUp`, `HandleLinkDown` | Fake backend unit coverage |
| Doctor check and code | done | `doctor.go`, `doctor_linux.go`, `register.go`, `internal/core/diagnostic/codes.go` | Unit and `.ci` coverage |
| Metrics | done | `metrics.go`, `transport.go` | Exact series and labels tested |
| QEMU integration | done | `transport_integration_linux_test.go`, `scripts/evidence/qemu-all-tests.sh` | `go test -tags integration -run TestOSPFTransport` passed in QEMU |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestOSPFTransportVethMulticastRoundTrip` in QEMU | Peer receives from `224.0.0.5` with source `192.0.2.1` and receiver ifindex |
| AC-2 | done | `TestOSPFTransportSendMulticastDoesNotAlterPayload`; QEMU tcpdump/manual probe during diagnosis | Transport sends OSPF bytes as raw IP payload |
| AC-3 | done | `TestOSPFTransportVethMulticastRoundTrip`; RFC comments in `backend_linux.go` | TTL 1 and loop-off are on TX socket, sender does not receive its own multicast |
| AC-4 | done | `TestOSPFTransportJoinAllDRouters`; `TestOSPFTransportAllDRoutersReceive` | Join enables receive, leave stops receive |
| AC-5 | done | `TestOSPFTransportSendUnicastDoesNotAlterPayload` | Unicast destination preserved, payload unchanged |
| AC-6 | done | `TestOSPFTransportCloseOnLinkDown` | Closes handle and removes interface state |
| AC-7 | done | `TestOSPFDoctorRawSocketUnavailable`, `TestOSPFDoctorCodeRegistered`, `make ze-ospf-test`, QEMU raw socket cap probe | Error names `CAP_NET_RAW`; explain surface exists |
| AC-8 | done | `TestStripIPv4Header`, `TestLinuxInterfaceCountsMalformedReceiveDrop` | Short datagrams and invalid IHL rejected without slicing past buffer; malformed receive drops increment reason `malformed-ipv4` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestStripIPv4Header` | pass | `transport_test.go` | IHL strip and reject paths |
| `TestMulticastGroupConstants` | pass | `transport_test.go` | Protocol 89 and multicast constants |
| `TestOSPFTransportSendMulticastDoesNotAlterPayload` | pass | `transport_test.go` | Byte-for-byte multicast send |
| `TestOSPFTransportSendUnicastDoesNotAlterPayload` | pass | `transport_test.go` | Byte-for-byte unicast send |
| `TestOSPFTransportJoinLeaveAllDRouters` | pass | `transport_test.go` | DR group toggle |
| `TestOSPFTransportOpenOnLinkUp` | pass | `transport_test.go` | Wiring from link up |
| `TestOSPFTransportCloseOnLinkDown` | pass | `transport_test.go` | Wiring from link down |
| `TestOSPFTransportReceiveDispatchByIfindex` | pass | `transport_test.go` | Receive fan-in preserves ifindex |
| `TestOSPFDoctorRawSocketUnavailable` / `TestOSPFDoctorCodeRegistered` | pass | `doctor_test.go` | Doctor code and registration |
| `TestOSPFTransportMetricsSeries` | pass | `metrics_test.go` | Exact metric names and labels |
| `TestOSPFTransportVethMulticastRoundTrip` | pass | `transport_integration_linux_test.go` | QEMU raw multicast receive |
| `TestOSPFTransportAllDRoutersReceive` | pass | `transport_integration_linux_test.go` | QEMU DR group join/leave |
| `TestOSPFTransportRawSocketCap` | pass | `transport_integration_linux_test.go` | QEMU CAP_NET_RAW probe |
| `ospf-doctor-raw-socket.ci` | pass | `test/ospf/ospf-doctor-raw-socket.ci` | `make ze-ospf-test` |
| `TestResolveOSPFInterfaceUsesIfaceResolverOSName` | pass | `backend_linux_test.go` | Logical name -> resolved kernel device through iface resolver |
| `TestLinuxInterfaceCountsMalformedReceiveDrop` | pass | `backend_linux_test.go` | Malformed raw receive datagrams increment drop metric reason |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ospf/transport/*.go` | done | Transport, Linux backend, non-Linux stub, doctor, metrics, tests |
| `internal/core/diagnostic/codes.go` | done | Added `doctor-ospf-raw-socket` |
| `cmd/ze/ze_core_dispatch.go` | done | Blank import for OSPF transport doctor registration |
| `scripts/evidence/qemu-all-tests.sh` | done | Adds OSPF transport integration package |
| `test/ospf/ospf-doctor-raw-socket.ci` | done | Explain fixture |
| `internal/test/cli/register.go`, `mk/test-functional.mk`, `Makefile` | done | OSPF functional runner and target |
| `docs/architecture/wire/ospf.md`, `docs/architecture/core-design.md`, `docs/plugin-development/metrics.md`, `docs/functional-tests.md` | done | Transport, metrics, and test docs |

### Audit Summary
- **Total items:** 34
- **Done:** 34
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5, documented in Deviations and Bugs Found/Fixed

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Raw IP send/receive for OSPF (proto 89) | unit + QEMU integration | `go test ./internal/plugins/ospf/transport`; QEMU `go test -tags integration -count=1 -timeout 120s -run TestOSPFTransport ./internal/plugins/ospf/transport` passed |
| IP multicast membership (`224.0.0.5` always, `224.0.0.6` as DR/BDR), TTL 1, loop off | unit + QEMU integration | `TestOSPFTransportJoinLeaveAllDRouters`, `TestOSPFTransportVethMulticastRoundTrip`, `TestOSPFTransportAllDRoutersReceive` |
| OSPF payload sent unaltered; IP header built by the kernel | unit + QEMU evidence | `TestOSPFTransportSendMulticastDoesNotAlterPayload`, `TestOSPFTransportSendUnicastDoesNotAlterPayload`, QEMU raw socket send/receive path |
| `CAP_NET_RAW` doctor check | unit + explain fixture + QEMU | `TestCheckOSPFRawSocketUnavailable` covers check execution with a synthetic OSPF config tree; `TestOSPFDoctorRawSocketCheckRegistered` covers registration; `make ze-ospf-test` covers `ze explain` metadata only; `TestOSPFTransportRawSocketCap` covers raw socket open in QEMU |
| Four transport metric series owned here | unit test | `TestOSPFTransportMetricsSeries` asserts names and labels |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Backend resolved logical interface names with `net.InterfaceByName`, bypassing iface `os-name` / `mac/match` | `backend_linux.go` | fixed with `iface.Resolve` / `iface.Addresses` and `TestResolveOSPFInterfaceUsesIfaceResolverOSName` |
| 2 | ISSUE | Malformed IPv4 receive datagrams bypassed drop metrics | `backend_linux.go` | fixed with backend drop recorder and `TestLinuxInterfaceCountsMalformedReceiveDrop` |

### Fixes applied
- Resolved logical interface names through the shared iface resolver before socket bind, matching IS-IS.
- Counted malformed receive drops with reason `malformed-ipv4`.
- Set both `IP_TTL=1` and `IP_MULTICAST_TTL=1` on the TX socket so unicast OSPF stays link-local.
- Added doctor-check registration coverage with `diagnostic.DoctorCheckNames()`.
- Moved `ze-ospf-test` help/docs below non-gated release-evidence suites and labelled it diagnostic explain.
- Documented both OSPF drop metric reasons: `malformed-ipv4` and `send-error`.
- Corrected spec evidence so `make ze-ospf-test` is explain metadata only until spec-ospf-4 adds OSPF config syntax.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 3 | ISSUE | TX socket did not set `IP_TTL=1` for unicast OSPF sends | `backend_linux.go` | fixed and covered by QEMU `expectTransmitTTL` |
| 4 | ISSUE | Doctor registration not covered by tests | `doctor_test.go` | fixed with `TestOSPFDoctorRawSocketCheckRegistered` |
| 5 | NOTE | `ze-ospf-test` help placement implied default gate coverage | `Makefile` | moved below separator and docs updated |
| 6 | ISSUE | `.ci` fixture/spec overclaimed `ze doctor` execution before OSPF config exists | `test/ospf/ospf-doctor-raw-socket.ci`, this spec | relabelled fixture as `ze explain`; spec now cites unit check execution |
| 7 | NOTE | Drop metric docs omitted `send-error` | `docs/plugin-development/metrics.md` | documented receive and send-failure reasons |
| 8 | clean | OSPFTransportReview5 found the doctor evidence wording correct | this spec | no further action |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist / Compile
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/ospf/transport/` | yes | `go test ./internal/plugins/ospf/transport` passed |
| `test/ospf/ospf-doctor-raw-socket.ci` | yes | `make ze-ospf-test` passed |
| `docs/architecture/wire/ospf.md`, `docs/architecture/core-design.md`, `docs/plugin-development/metrics.md`, `docs/functional-tests.md` | yes | `make ze-doc-test` passed |
| `plan/learned/957-ospf-3-ip-transport.md` | yes | learned summary written and indexed in `ai/LEARNED-INDEX.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | veth peer receives AllSPFRouters with source and receiver ifindex | QEMU `TestOSPFTransportVethMulticastRoundTrip` passed |
| AC-2 | multicast send supplies only OSPF bytes | `TestOSPFTransportSendMulticastDoesNotAlterPayload`; QEMU receive payload match |
| AC-3 | TTL 1, loop off, AllSPFRouters join on open | QEMU `expectTransmitTTL` asserts `IP_TTL=1` and `IP_MULTICAST_TTL=1`; sender receive channel remains empty |
| AC-4 | AllDRouters join/leave toggles runtime receive | `TestOSPFTransportJoinLeaveAllDRouters`; QEMU `TestOSPFTransportAllDRoutersReceive` |
| AC-5 | unicast destination sends bytes unchanged | `TestOSPFTransportSendUnicastDoesNotAlterPayload`; TX socket `IP_TTL=1` covered in QEMU |
| AC-6 | interface down closes handle and emits teardown | `TestOSPFTransportCloseOnLinkDown` |
| AC-7 | raw-socket capability surfaced by doctor metadata and registered check | `TestCheckOSPFRawSocketUnavailable`, `TestOSPFDoctorRawSocketCheckRegistered`, `make ze-ospf-test` explain fixture |
| AC-8 | malformed IPv4 receive rejected and counted | `TestStripIPv4Header`, `TestLinuxInterfaceCountsMalformedReceiveDrop` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File / Test | Verified |
|-------------|-----------------|----------|
| `interface/up` for enabled interface | `TestOSPFTransportOpenOnLinkUp` | opens backend and joins AllSPFRouters |
| engine send to multicast/unicast | send-does-not-alter unit tests + QEMU veth receive | packet bytes arrive unchanged |
| raw datagram receive | `TestOSPFTransportVethMulticastRoundTrip` | receive path strips IPv4 header and preserves ifindex/src |
| DR/BDR role signal | `TestOSPFTransportJoinLeaveAllDRouters`, `TestOSPFTransportAllDRoutersReceive` | joins/leaves `224.0.0.6` |
| OSPF doctor diagnostic | `TestOSPFDoctorRawSocketCheckRegistered`, `TestCheckOSPFRawSocketUnavailable`, `test/ospf/ospf-doctor-raw-socket.ci` | unit tests cover registered check execution; `.ci` covers `ze explain` metadata until OSPF config lands in spec-ospf-4 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | QEMU raw IPv4 proto-89 veth send/receive passed |
| A-2 | confirmed | QEMU multicast membership receive passed with address-bound `IPMreq` |
| A-3 | confirmed | per-interface socket ifindex asserted in receive test |
| A-4 | confirmed | QEMU peer receives while sender does not loop back; TTL assertions pass |
| A-5 | confirmed | AllDRouters join/leave unit and QEMU tests pass |
| A-6 | confirmed | raw socket doctor unit and QEMU capability probe pass |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Wire transport layer | `docs/architecture/wire/ospf.md` with source anchors to OSPF transport | `make ze-doc-test` passed |
| Core architecture OSPF transport note | `docs/architecture/core-design.md` source anchors | `make ze-doc-test` passed |
| Metrics inventory and drop reasons | `docs/plugin-development/metrics.md` source anchors to `deliverDatagram` and `SendPacket` | `make ze-doc-test` passed |
| Functional test inventory | `docs/functional-tests.md`, `Makefile`, `mk/test-functional.mk` | `make ze-doc-test`, `make ze-ospf-test`, `make ze-ospf-wire-test` passed |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-8 all demonstrated
- [x] In-scope End-to-End User Stories: every story has a working path and a passing test
- [x] Wiring Test table complete -- every row has a concrete test name, none deferred
- [x] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [x] Child-focused lint, unit, functional, doc, wire, and QEMU transport evidence captured
- [ ] Overall OSPF final `make ze-verify-fast` passes (owned by final OSPF chain, not this child)
- [x] Feature code integrated (`internal/plugins/ospf/transport/`)
- [x] Integration completeness proven end-to-end for this transport child
- [x] Documentation Update Checklist answered Yes/No with source evidence
- [x] Critical Review passes
- [x] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); A-2 (multicast membership) and A-3 (ifindex recovery) explicitly resolved

### Quality Gates (SHOULD pass)
- [x] RFC constraint comments added
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (backend interface justified by future BSD/VPP)
- [x] No speculative features (only the Linux backend in v1)
- [x] Single responsibility per file (transport / multicast / backend / doctor / metrics)
- [x] Explicit > implicit behavior
- [x] Minimal coupling (the engine never touches the socket)

### TDD
- [x] Tests written
- [x] Tests FAIL or initially found defects during review/QEMU diagnosis (same-netns multicast, RX/TX loop suppression, iface resolver, malformed drops, unicast TTL, doctor registration)
- [x] Tests PASS (see Pre-Commit Verification)
- [x] Boundary tests for all numeric inputs
- [x] Functional tests for end-to-end behavior
- [x] Interop tests for protocol features (transport correctness proven here; full FRR interop owned by ospf-13)
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval or are assigned to later OSPF child specs by umbrella scope (full FRR interop in ospf-13, final `make ze-verify-fast` at overall completion)
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/957-ospf-3-ip-transport.md`
- [ ] Commit script deferred until all OSPFv2 and OSPFv3 work is complete, per user request
