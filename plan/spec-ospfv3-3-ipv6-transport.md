# Spec: ospfv3-3-ipv6-transport

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospfv3-1-types.md, spec-ospfv3-2-wire.md |
| Phase | 1/9 |
| Updated | 2026-06-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now).
2. `.claude/rules/planning.md` - workflow rules.
3. `plan/spec-ospfv3-0-umbrella.md` - umbrella scope (this is row ospfv3-3); "Transport" in-scope row, the "Packet Receive Path"/"Packet Transmit Path" data flow, and AC-1/AC-15.
4. `plan/spec-ospf-3-ip-transport.md` - the OSPFv2 sibling this transport mirrors in shape (Backend/InterfaceHandle/Transport orchestrator, doctor split, four metric series, QEMU veth integration tests, iface EventBus lifecycle). The OSPFv3 transport copies the orchestration and diverges only where IPv6 forces it.
5. `internal/plugins/ospf/transport/` - the implemented OSPFv2 raw-IPv4 transport (`transport.go`, `backend_linux.go`, `backend_other.go`, `multicast.go`, `doctor*.go`, `metrics.go`); copy the orchestrator shape, replace the IPv4 raw-socket internals.
6. `internal/component/ppp/ra_linux.go` - the in-tree `golang.org/x/net/ipv6.PacketConn` multicast pattern (`net.ListenPacket("ip6:...","::")`, `ipv6.NewPacketConn`, `JoinGroup`, `WriteTo` with a `ControlMessage`).
7. `internal/component/bfd/transport/udp_linux.go` - the in-tree IPv6 hop-limit / control-message pattern (`IPV6_RECVHOPLIMIT`, `ParseSocketControlMessage`).
8. `internal/plugins/ospfv3/packet/checksum.go` - `PacketChecksum(src, dst, pkt) uint16` / `VerifyPacketChecksum(src, dst, pkt) bool` and the `offChecksum = 12` field offset (`header.go`); the transport finalizes the checksum on TX and carries `src`/`dst` up for verification.

## Task

Implement the raw IPv6 transport for OSPFv3 in `internal/plugins/ospfv3/transport/`,
mirroring the orchestration of the proven OSPFv2 transport
(`internal/plugins/ospf/transport/`) but replacing every IPv4 raw-socket internal
with the IPv6 equivalent. OSPFv3 runs directly over IPv6 with IP protocol number
89, link-local source addresses, and the multicast groups `ff02::5`
(AllSPFRouters) and `ff02::6` (AllDRouters) (RFC 5340 §2.9).

The transport opens a per-interface raw IPv6 socket
(`net.ListenPacket("ip6:89", "::")` wrapped in `golang.org/x/net/ipv6.PacketConn`,
the in-tree PPP-RA IPv6 multicast pattern). Unlike the OSPFv2 transport, an IPv6
raw socket delivers the upper-layer payload directly: there is **no IP header to
strip**. The receiving source address comes from `ReadFrom`, and the destination
address, receiving interface index, and hop limit come from the IPv6 ancillary
control message (`SetControlMessage(ipv6.FlagDst | ipv6.FlagInterface |
ipv6.FlagHopLimit, true)`). The transport delivers
`(ifindex, src, dst, hopLimit, payload)` to the engine.

The transport performs exactly ONE header-level check: the OSPFv3 Instance ID
demux. RFC 5340 §4.2.1 discards a packet whose Instance ID does not match the
Instance ID configured for the receiving interface. The Instance ID is a
per-interface link-local selector (common-header byte 14), so it is a
receive-interface filter the transport owns: the umbrella's "Packet Receive Path"
assigns it to the transport and its TDD plan names
`TestOSPFv3TransportRejectsWrongInstance` in this package. ospfv3-4 supplies each
interface's configured Instance ID through `EnableInterface(name, instanceID)`;
the transport reads byte 14 (via `packet.PeekInstanceID`, after the short-datagram
guard guarantees a 16-byte header is present) and drops a mismatch
(`ze_ospf_packets_dropped_total{reason="instance-mismatch"}`) before delivery.
Beyond that single demux byte the transport MUST NOT parse or alter the OSPFv3
packet body: the common-header `Type` dispatch and all OSPF-semantic validation
(version / area / checksum / RFC 7166 trailer) are owned by ospfv3-4
(`instance.go`), not by this transport.

The OSPFv3 packet checksum is the **IPv6 upper-layer checksum** bound to the IPv6
source and destination addresses (RFC 5340 §A.3.1). The codec's `Packet.WriteTo`
deliberately leaves the checksum field zero because the codec cannot know the
addresses. The transport is the only layer that authoritatively knows the on-wire
source (the egress interface's link-local address) and the destination, so the
transport **finalizes the checksum on TX**: it computes
`packet.FinalizePacketChecksum(src, dst, bytes)` (a helper this spec adds to the
ospfv3-2 codec -- the `header.go` comment already promises it) which writes the
value at `offChecksum` (byte 12) before sending. On RX the transport carries
`src` and `dst` up so the ospfv3-4 dispatcher can call
`packet.VerifyPacketChecksum(src, dst, payload[:Header.Length])` -- the checksum
covers the **OSPF packet length**, not the received datagram length, so a
datagram with trailing bytes (e.g. an LLS block) must be verified over
`payload[:Header.Length]`, not the whole `RawPacket.Payload` (a contract this
spec sets on `RawPacket`; F-1).

**Source binding makes the pseudo-header source provably the on-wire source
(eliminating the A-6 risk).** The transport sets the same selected link-local
address it fed to `FinalizePacketChecksum` as the explicit send source -- the
`ControlMessage.Src` field of `WriteTo` (IPV6_PKTINFO) -- so the kernel emits the
packet from exactly the address the checksum covers. This is the design, not a
fallback: relying on the kernel's default source selection over a multicast socket
is the A-6 source-mismatch risk. The transport does **not** use
`ipv6.PacketConn.SetChecksum(true, 12)` kernel checksum offload (the alternative
Go OSPFv3 idiom that would have the kernel compute the checksum from the on-wire
source) because (a) the RFC 7166 auth path needs the checksum left zero for signed
packets -- a per-packet behaviour the always-on socket option cannot express
cleanly -- and (b) the ospfv3-2 codec's `PacketChecksum`/`VerifyPacketChecksum`
are already golden-tested, so user-space finalization reuses proven code; binding
`cm.Src` removes the source-mismatch risk the offload path would otherwise avoid.
The kernel does not clobber a user-space checksum because IPV6_CHECKSUM is off by
default (RFC 3542 §3.1); FRR `ospf6d` finalizes in user space the same way.

The transport joins `ff02::5` on every enabled interface and joins `ff02::6`
only when this router is DR or BDR on that interface; it leaves a group on
interface teardown or DR/BDR role change. Membership is per-interface via
`ipv6.PacketConn.JoinGroup(ifi, group)` / `LeaveGroup`. Outbound multicast uses
`SetMulticastHopLimit(1)` (link-local scope), `SetMulticastInterface(ifi)` so a
packet leaves the intended interface, and `SetMulticastLoopback(false)` so the
local router does not receive its own multicast; outbound unicast uses
`SetHopLimit(1)`. A send is directed by the engine to either a multicast group
(`ff02::5` / `ff02::6`) or a unicast neighbour link-local address; the transport
does not choose the destination, it only finalizes the checksum, frames, and
sends.

OSPFv3 uses Hop Limit 1 for link-local scoping (mirroring OSPFv2's TTL 1); the
link-local source address provides the primary on-link confinement (a packet
sourced from `fe80::/10` is never forwarded off the link). Base OSPFv3 does NOT
mandate the GTSM hop-limit-255 check (that is BFD's RFC 5881 behaviour, not
OSPF's); GTSM hardening is a future option. The transport delivers the received
hop limit up so a later spec can add a policy check without a transport change.

The backend sits behind a Go interface (open, close, send, receive, join/leave
group per interface) so a future BSD or VPP backend can drop in: a real Linux
`backend_linux.go` and a non-Linux stub `backend_other.go` behind the interface.
v1 ships only the Linux backend. The transport subscribes to the iface EventBus
and opens an interface on link up, closes it and signals teardown on link down,
mirroring the OSPFv2 sibling. Because an IPv6 link-local address is assigned
asynchronously (Duplicate Address Detection completes ~1s after link up), an open
that finds no usable link-local source is marked pending and retried by the
periodic interface rescan and on `interface/addr-added` — an OSPFv3-specific
wrinkle the IPv4 sibling does not have.

The raw socket requires `CAP_NET_RAW`. A registered doctor check
`doctor-ospfv3-raw-socket` (per `ai/rules/doctor-checks.md`, modelled on
`internal/plugins/ospf/transport/doctor*.go`) opens and immediately closes a raw
IPv6 proto-89 socket before the daemon starts; its `doctor-*` code is registered
in `internal/core/diagnostic/codes.go` alongside `doctor-ospf-raw-socket`. This
transport reuses the shared OSPFv2 `ze_ospf_packets_*` / `ze_ospf_sockets_open`
Prometheus series (the metrics registry is get-or-create by name, so v2 and v3
share one series; OSPFv3 is "our OSPF" and does not fork a `ze_ospfv3_*`
namespace); ospfv3-13 only scrapes/asserts them.

Because the raw socket and IPv6 multicast membership are Linux-only kernel
capabilities, the send/receive path is proven by a Linux-only QEMU integration
test on a veth pair (link-local multicast send/recv), per `ai/rules/qemu-testing.md`;
a plain `.ci` covers only the user-visible doctor output.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `internal/plugins/ospf/transport/transport.go` - `RawPacket`, `InterfaceHandle`, `Backend`, `Transport` orchestrator, `StripIPv4Header`
  → Constraint: copy the `Transport` orchestrator method set verbatim (New, SetMetrics, EnableInterface/DisableInterface, SetSigner, OnInterfaceUp/Down, Receive, SubscribeIfaceEvents, RescanInterfaces, HandleLinkUp/Down, RecordDrop, SendPacket, JoinAllDRouters/LeaveAllDRouters, InterfaceOpen, OpenInterfaceCount, InterfaceNameByIfIndex, Close) so ospfv3-4/5/6 wiring mirrors ospf
  → Decision: extend `RawPacket` with `Dst netip.Addr` and `HopLimit uint8` (OSPFv3 checksum needs the dst; GTSM/policy needs the hop limit); drop `StripIPv4Header` entirely (IPv6 raw sockets have no IP header on receive)
- [ ] `internal/plugins/ospf/transport/backend_linux.go` - the per-interface socket open, multicast options, join/leave, RX `Recvfrom`, TX `Sendto`
  → Constraint: replace `unix.Socket(AF_INET, SOCK_RAW, 89)` + `IP_*` options + `IPMreq` with a per-interface `ipv6.PacketConn`; keep the per-interface RX goroutine, the receive-buffer copy-out, and `SO_BINDTODEVICE`
- [ ] `internal/component/ppp/ra_linux.go` - `net.ListenPacket("ip6:ipv6-icmp","::")` → `ipv6.NewPacketConn` → `JoinGroup(iface, group)` → `WriteTo(buf, &ipv6.ControlMessage{IfIndex:...}, dst)`
  → Constraint: this is the canonical in-tree IPv6 multicast pattern; use `"ip6:89"` for the OSPF protocol number; the `ControlMessage.IfIndex` selects the egress interface on TX
- [ ] `internal/component/bfd/transport/udp_linux.go` - `IPV6_RECVHOPLIMIT`, `IPV6_UNICAST_HOPS`, `parseReceivedTTL` (`unix.ParseSocketControlMessage`)
  → Constraint: the received hop limit comes from the ancillary control message; with `ipv6.PacketConn` it is `ControlMessage.HopLimit` once `SetControlMessage(ipv6.FlagHopLimit, true)` is set (no manual cmsg parse needed); OSPFv3 uses hop limit 1, not 255
- [ ] `internal/plugins/ospfv3/packet/checksum.go` + `header.go` - `PacketChecksum`/`VerifyPacketChecksum`, `offChecksum = 12`, the stale `FinalizePacketChecksum` comment
  → Constraint: the transport finalizes the checksum on TX via a new `packet.FinalizePacketChecksum(src, dst, pkt)` (added by this spec) and carries `src`/`dst` up for ospfv3-4 to verify; the transport never reads or writes any OSPF field except the checksum the codec left zero
- [ ] `internal/component/iface/events/events.go` - `EventUp`/`EventDown`/`EventAddrAdded`
  → Constraint: `EventUp`/`EventDown` drive per-interface open/close; `EventAddrAdded` (and the periodic rescan) re-attempt an open that was pending because the link-local source was not yet ready (IPv6 DAD)
- [ ] `ai/rules/doctor-checks.md`, `ai/rules/qemu-testing.md`
  → Constraint: owning-package doctor check + `doctor-ospfv3-raw-socket` code in `internal/core/diagnostic/codes.go` + unit + functional test; linux-only code MUST ship a QEMU integration test (hardware-only is not a skip reason)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`
  → Constraint: the received payload is a view into the receive buffer, copied out once before queueing (as the OSPFv2 sibling does); no per-datagram allocation on the hot path beyond the one queue copy and the checksum write
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md`
  → Constraint: all transport code + the doctor check live under `internal/plugins/ospfv3/transport/`; the transport holds no protocol `Type` switch (dispatch is ospfv3-4)

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc5340.md` - OSPFv3: §2.9 (transport: IPv6 protocol 89, link-local source, `ff02::5`/`ff02::6`), §A.3.1 (16-byte header, Instance ID, IPv6 pseudo-header checksum with Next Header 89)
  → Constraint: send/receive IPv6 proto 89 from a link-local source to `ff02::5` (all routers) and `ff02::6` (DR/BDR); the checksum is the IPv6 upper-layer checksum over the OSPF packet plus the pseudo-header; the transport supplies the addresses
- [ ] `rfc/short/rfc7166.md` - OSPFv3 Authentication Trailer (forward-looking; auth itself is ospfv3-12)
  → Constraint: when an Authentication Trailer is appended the checksum is left zero (RFC 7166 §2.2); the transport's `SetSigner` hook is the seam where ospfv3-12 will append the trailer and skip checksum finalization — design the TX path so "signer present ⇒ do not finalize the checksum" is the only change ospfv3-12 needs

**Key insights:** (minimal context to resume after compaction)
- OSPFv3 transport = the OSPFv2 transport orchestrator with the IPv4 raw socket swapped for a `golang.org/x/net/ipv6.PacketConn` (PPP-RA pattern). No IP header strip; `src` from `ReadFrom`, `dst`/`ifindex`/`hopLimit` from the control message.
- The checksum is address-bound (IPv6 upper-layer checksum, RFC 5340 §A.3.1). Only the transport knows the egress link-local source, so the transport finalizes the checksum on TX (`packet.FinalizePacketChecksum`) and carries `src`/`dst` up for ospfv3-4 to verify on RX.
- Join `ff02::5` on every enabled interface; join `ff02::6` ONLY when DR/BDR; leave on teardown or role change. Per-interface `JoinGroup`/`LeaveGroup`.
- `SetMulticastHopLimit(1)` + `SetMulticastInterface(ifi)` + `SetMulticastLoopback(false)`; unicast `SetHopLimit(1)`. Send to multicast or unicast as directed by the engine. The transport NEVER parses/alters the OSPFv3 body (only the checksum field).
- `CAP_NET_RAW` required; `doctor-ospfv3-raw-socket` guards startup. The link-local source can lag link-up (IPv6 DAD): a pending open is retried by the rescan and `interface/addr-added`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/transport/` - the implemented OSPFv2 raw-IPv4 transport; `RawPacket{IfIndex,Src,Payload}`, the `Transport` orchestrator and its full method set, the per-interface Linux backend with `IP_*` socket options and `IPMreq` membership, `StripIPv4Header`
  → Constraint: reuse the orchestrator and lifecycle verbatim; the IPv4 backend internals and `StripIPv4Header` do NOT carry over
- [ ] `internal/component/ppp/ra_linux.go` - the only in-tree `ipv6.PacketConn` multicast sender/receiver
  → Constraint: model the IPv6 socket open / join / control-message handling on this; it proves `JoinGroup` + `WriteTo` with `ControlMessage.IfIndex` works in-tree
- [ ] `internal/component/bfd/transport/udp_linux.go` - the in-tree IPv6 hop-limit control-message reader
  → Constraint: hop limit arrives as ancillary data; with `ipv6.PacketConn` it is `ControlMessage.HopLimit`; OSPFv3 sets and expects hop limit 1, not BFD's 255
- [ ] `internal/plugins/ospfv3/packet/checksum.go`, `header.go` - `PacketChecksum`/`VerifyPacketChecksum`, `offChecksum`, the absent `FinalizePacketChecksum`
  → Constraint: the codec leaves the checksum zero; the transport finalizes it; add `FinalizePacketChecksum` so the `header.go` comment becomes true
- [ ] Ze has an OSPFv2 IP-multicast transport and a PPP IPv6 ICMP multicast sender, but no IPv6 raw OSPF transport
  → Constraint: this is the first raw IPv6 proto-number multicast transport; nothing existing to extend, but two patterns to combine (OSPFv2 orchestrator + PPP-RA IPv6 socket)

**Behavior to preserve:**
- OSPFv2 raw-socket transport unchanged: OSPFv3 adds a sibling transport in a separate package; it does not refactor the OSPFv2 transport in place.
- PPP RA and BFD IPv6 socket code unchanged: OSPFv3 is a new, independent IPv6 socket user.
- iface EventBus semantics unchanged: OSPFv3 is a new subscriber only.
- Existing doctor checks and `internal/core/diagnostic/codes.go` entries unchanged; a new `doctor-ospfv3-raw-socket` code is appended alongside `doctor-ospf-raw-socket`.
- The ospfv3-2 codec's public behaviour is unchanged; `FinalizePacketChecksum` is an additive helper that writes the value `PacketChecksum` already computes.

**Behavior to change:**
- New package `internal/plugins/ospfv3/transport/` with a backend interface, a Linux `ipv6.PacketConn` proto-89 backend, and a non-Linux stub.
- New `packet.FinalizePacketChecksum(src, dst, pkt)` helper in `internal/plugins/ospfv3/packet/checksum.go`.
- New `doctor-ospfv3-raw-socket` diagnostic code and a registered doctor check.
- The shared OSPFv2 `ze_ospf_*` transport series are reused (not forked) here.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound: raw IPv6 datagrams (Next Header 89) arrive on enabled interfaces via a per-interface `ipv6.PacketConn` joined to `ff02::5` (and `ff02::6` when DR/BDR); the payload, source address, destination address, receiving ifindex, and hop limit come from `ReadFrom` plus the ancillary `ControlMessage`.
- Outbound: the OSPFv3 engine (ospfv3-5/6/7) hands a final OSPFv3 packet (checksum field zero) plus an interface reference and a destination selector (a multicast group `ff02::5`/`ff02::6` or a unicast neighbour link-local address); the transport finalizes the checksum from the egress source and destination, then sends the bytes as the IPv6 upper-layer payload (the kernel builds the IPv6 header).
- Lifecycle: iface EventBus `interface/up` and `interface/down` events open and close per-interface RX/TX; `interface/addr-added` and the periodic rescan retry an open that was pending on the link-local source; a DR/BDR role-change signal from ospfv3-5 triggers `ff02::6` join/leave.

### Transformation Path
1. **Open:** on `interface/up` for a configured/enabled interface, resolve the logical interface through the iface resolver (`os-name` / `mac/match`), require a usable link-local (`fe80::/10`) source address (else mark pending and return), open the per-interface `ipv6.PacketConn` (`net.ListenPacket("ip6:89","::")` + `ipv6.NewPacketConn`), `SO_BINDTODEVICE` to the resolved kernel device, set `SetMulticastHopLimit(1)`, `SetMulticastInterface(ifi)`, `SetMulticastLoopback(false)`, `SetHopLimit(1)`, `SetControlMessage(ipv6.FlagDst | ipv6.FlagInterface | ipv6.FlagHopLimit, true)`, `JoinGroup(ifi, ff02::5)`, start RX and TX goroutines.
2. **Receive:** `ReadFrom` → `(payload, controlMessage, src)` → deliver `RawPacket{IfIndex, Src, Dst, HopLimit, Payload}` (payload copied out of the shared receive buffer before queueing; `Dst`/`IfIndex`/`HopLimit` from the control message). No IP-header strip. The transport hands raw OSPFv3 packets up; it does NOT switch on the common-header `Type` (that dispatcher is ospfv3-4) and does NOT verify the checksum (that is ospfv3-4, using the `Src`/`Dst` carried in `RawPacket`). A datagram shorter than the 16-byte OSPFv3 common header is dropped (`ze_ospf_packets_dropped_total{reason="short"}`).
3. **Send:** final engine OSPFv3 packet + interface + destination selector → finalize the checksum (`packet.FinalizePacketChecksum(srcLinkLocal, dst, payload)`, unless a signer is set, in which case ospfv3-12 owns the trailer and the checksum stays zero) → `WriteTo(payload, &ipv6.ControlMessage{IfIndex, HopLimit:1}, dst)` on the per-interface socket. The kernel builds the IPv6 header. The transport does NOT alter any OSPFv3 body byte.
4. **DR/BDR membership:** when ospfv3-5 signals this router became DR or BDR on an interface, `JoinGroup(ifi, ff02::6)`; on losing the role or on teardown, `LeaveGroup`.
5. **Close:** on `interface/down`, stop RX/TX goroutines, leave joined groups, close the per-interface socket, signal the engine to tear down adjacencies on that interface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ transport | raw IPv6 proto-89 datagrams via `ipv6.PacketConn`; no IP header on RX; `src` from `ReadFrom`, `dst`/`ifindex`/`hopLimit` from the `ControlMessage` | [ ] |
| transport ↔ packet codec | TX finalizes the checksum via `packet.FinalizePacketChecksum(src, dst, pkt)`; RX carries `src`/`dst` up for `packet.VerifyPacketChecksum` (called by ospfv3-4) | [ ] |
| transport ↔ OSPFv3 engine | backend interface: open/close per interface, send `(packet, dst selector)`, receive `RawPacket{ifindex,src,dst,hopLimit,payload}` channel, join/leave `ff02::6` on DR/BDR change | [ ] |
| iface EventBus ↔ transport | subscribe `interface/up`, `interface/down`, `interface/addr-added`; open/close/retry per-interface RX/TX | [ ] |
| ospfv3-5 (DR/BDR election) ↔ transport | DR/BDR role-change signal → `ff02::6` join/leave | [ ] |
| transport ↔ doctor | `doctor-ospfv3-raw-socket` check probes raw IPv6 socket open / `CAP_NET_RAW` | [ ] |

### Integration Points
- New package `internal/plugins/ospfv3/transport/` (backend interface, Linux backend, non-Linux stub, multicast constants, doctor check, metrics).
- Consumes `internal/plugins/ospfv3/types` (spec-ospfv3-1) where a destination selector or typed Instance/Interface value is needed.
- Consumes `internal/plugins/ospfv3/packet` (spec-ospfv3-2) for `FinalizePacketChecksum` / `PacketChecksum` and `offChecksum`.
- Subscribes to `internal/component/iface/events` for link up/down/addr-added.
- Registers a doctor check (`internal/core/diagnostic/codes.go` + owning-package registration).
- Provides the send/receive primitive that the packet receive dispatcher (ospfv3-4 `instance.go`) and the runtime specs (ospfv3-5 Hello, ospfv3-6 DD/LS Request, ospfv3-7 LS Update/LS Ack flooding) build on.

### Architectural Verification
- [ ] No bypassed layers (datagrams flow socket → `RawPacket` → engine; the engine never touches the socket directly; the `Type` dispatcher and checksum verification are ospfv3-4)
- [ ] No unintended coupling (backend behind an interface; Linux specifics in `backend_linux.go`; OSPFv2 transport untouched; no PPP/BFD coupling beyond the shared `x/net/ipv6` library)
- [ ] No duplicated functionality (one OSPFv3 transport; the engine reuses it for Hello/DD/LSReq/LSUpdate/LSAck rather than opening its own socket)
- [ ] Zero-copy preserved (received payload is a view copied out once before queueing; the only TX mutation is the 2-byte checksum write into the caller's buffer)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `golang.org/x/net/ipv6.PacketConn` over `net.ListenPacket("ip6:89","::")` can send and receive raw OSPF (proto 89) datagrams with per-packet `ControlMessage` dst/ifindex/hoplimit | `internal/component/ppp/ra_linux.go` (ICMPv6 PacketConn), `go.mod` `golang.org/x/net v0.52.0` | fall back to raw `unix.Socket(AF_INET6, SOCK_RAW, 89)` + manual `recvmsg`/`sendmsg` cmsg handling (BFD pattern) | QEMU veth link-local multicast round-trip (`TestOSPFv3TransportVethMulticastRoundTrip`) | unvalidated |
| A-2 | IPv6 multicast receive on a raw proto-89 `ipv6.PacketConn` works with per-interface `JoinGroup(ifi, ff02::5)` and no promiscuous mode | RFC 5340 §2.9; PPP-RA `JoinGroup` precedent (ICMPv6, not raw proto) | need raw-socket-specific membership tuning or `unix` `IPV6_JOIN_GROUP` directly | QEMU two-netns veth multicast receive | unvalidated |
| A-3 | The receiving ifindex, destination group, and hop limit are recoverable from the `ipv6.ControlMessage` after `SetControlMessage(FlagDst\|FlagInterface\|FlagHopLimit, true)` | `golang.org/x/net/ipv6` API; BFD cmsg precedent | RX cannot attribute a datagram to an interface or know the dst for the checksum; fall back to socket-per-interface bind + `recvmsg` | QEMU veth round-trip asserting ifindex, dst, hop limit | unvalidated |
| A-4 | `SetMulticastHopLimit(1)` + `SetMulticastInterface(ifi)` make multicast leave the intended interface with link-local scope, and `SetMulticastLoopback(false)` suppresses local self-receipt | RFC 5340 §2.9 link-local scope; OSPFv2 sibling `IP_MULTICAST_TTL`=1 / loop-off precedent | packets leave the wrong interface, are routed off-link, or the router receives its own Hellos | QEMU veth test: peer receives, sender does not loop back | unvalidated |
| A-5 | `ff02::6` (AllDRouters) join/leave can be toggled per-interface at runtime on a DR/BDR role change without re-opening the socket | `ipv6.PacketConn.JoinGroup`/`LeaveGroup` semantics; OSPFv2 sibling A-5 confirmed for IPv4 | need to re-open the socket on every election change (churn) | unit test toggling membership + QEMU `ff02::6` receive test | unvalidated |
| A-6 | The egress interface link-local (`fe80::/10`) source the transport selects for the checksum matches the source the kernel actually puts on the wire | RFC 5340 §2.9 link-local source; kernel source-address selection for a link-local destination over a bound interface | checksum verification fails at the peer because the pseudo-header source differs from the on-wire source | QEMU veth round-trip: peer `VerifyPacketChecksum` passes | unvalidated |
| A-7 | The link-local source may be unavailable at `interface/up` (DAD) but becomes available shortly after, recoverable via the periodic rescan and `interface/addr-added` | IPv6 DAD (~1s); `internal/component/iface/events` `EventAddrAdded`; OSPFv2 sibling rescan backstop | the interface never opens after a flap because the open is attempted only once while the address is tentative | unit test: pending open succeeds on rescan after a link-local appears | unvalidated |
| A-8 | `CAP_NET_RAW` is the only privilege needed to open the raw IPv6 socket and join the multicast groups on the gokrazy appliance | OSPFv2 raw-socket precedent; `ai/rules/doctor-checks.md` | socket open or `JoinGroup` EPERM at startup | `doctor-ospfv3-raw-socket` check + QEMU open probe | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `ipv6.PacketConn` cannot bind a raw proto-89 socket the way it binds ICMPv6, or `ReadFrom` does not surface the control message for a raw socket | RX never delivers a Hello / `cm` is nil | A-1/A-3 validation on veth; fall back to raw `unix` `AF_INET6 SOCK_RAW` proto 89 + `recvmsg`/`sendmsg` (BFD cmsg pattern) behind the same `Backend` interface |
| R-2 | Checksum source mismatch: the link-local source used in the pseudo-header differs from the on-wire source (multiple `fe80::` addresses, or kernel picks another) | the peer drops every packet on checksum failure; `ze_ospf_packets_received_total` at the peer stays zero | A-6: pick the interface's primary link-local deterministically; assert peer `VerifyPacketChecksum` in the veth round-trip; if the kernel overrides the source, bind the socket to the link-local source explicitly |
| R-3 | The router receives its own multicast (`SetMulticastLoopback` left on) → phantom self-neighbour | the engine sees a Hello sourced from its own Router ID / interface | `SetMulticastLoopback(false)` in setup; unit test asserts the option via a recording backend; QEMU test asserts the sender does not loop back |
| R-4 | Per-interface goroutine leak on rapid link flap | goroutine count climbs under flap | bounded lifecycle tied to `interface/down`; a read deadline (`SetReadDeadline`) so RX wakes to observe stop; bounded EventBus worker queue + periodic rescan backstop |
| R-5 | Hop limit not 1 → OSPFv3 packets routed off the link, or a GTSM-strict peer drops them | a remote (non-adjacent) router receives OSPFv3 packets, or FRR drops on hop-limit | `SetMulticastHopLimit(1)`/`SetHopLimit(1)` set in setup and asserted in a unit test; the link-local source already confines scope |
| R-6 | `ff02::6` membership not toggled on DR/BDR change → DROther never reaches the DR, or a former DR keeps receiving DR traffic | adjacency stuck because Updates to `ff02::6` are not received after becoming DR | A-5: per-interface join on the DR/BDR signal, leave on role loss/teardown; unit test toggles membership; QEMU `ff02::6` receive test |
| R-7 | The link-local source is not ready at link-up and the interface never opens | adjacency never forms on a freshly-upped interface | A-7: pending-open retried by the rescan and `interface/addr-added`; unit test for the pending→open transition |
| R-8 | `CAP_NET_RAW` absent at runtime | socket open or join EPERM at startup | doctor check `doctor-ospfv3-raw-socket` with a clear message before the daemon runs |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface/up` event for an enabled interface with a ready link-local source | → | transport opens the `ipv6.PacketConn`, sets multicast options, joins `ff02::5`, starts RX/TX | `TestOSPFv3TransportOpenOnLinkUp` |
| `interface/up` with no link-local yet, then `interface/addr-added` | → | open is deferred then completes when the link-local appears | `TestOSPFv3TransportOpenPendingLinkLocal` |
| engine sends a final Hello-shaped packet to `ff02::5` on an interface | → | transport finalizes the checksum and sends via `WriteTo` with the egress ifindex and hop limit 1 | `TestOSPFv3TransportSendMulticast` |
| datagram arrives on the peer interface | → | RX delivers `RawPacket{ifindex,src,dst,hopLimit,payload}`; peer `VerifyPacketChecksum` passes | `TestOSPFv3TransportVethMulticastRoundTrip` (`transport_integration_linux_test.go`, QEMU) |
| ospfv3-5 signals this router became DR on an interface | → | transport joins `ff02::6` on that interface; leaves on role loss | `TestOSPFv3TransportJoinAllDRouters` |
| `interface/down` event | → | transport leaves groups, closes the socket, signals engine teardown | `TestOSPFv3TransportCloseOnLinkDown` |
| OSPFv3 doctor check registration | → | `doctor-ospfv3-raw-socket` check is registered; body fires when an OSPFv3 config tree is present | `TestOSPFv3DoctorRawSocketCheckRegistered`, `TestOSPFv3CheckRawSocketUnavailable`; `.ci` covers `ze explain` until OSPFv3 config lands in spec-ospfv3-4 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two interfaces of a veth pair, transport opened on both, both joined `ff02::5` | A packet sent to `ff02::5` on one is received on the other with the receiving ifindex matching the receiver, the source address matching the sender's link-local, and the destination `ff02::5`; the payload is the OSPFv3 packet with no IP header |
| AC-2 | Send a final OSPFv3 packet (checksum field zero) to a multicast destination | The transport finalizes the checksum from the egress link-local source and the destination (`packet.FinalizePacketChecksum`), then sends the bytes as the IPv6 upper-layer payload (the kernel builds the IPv6 header); no OSPFv3 body byte other than the checksum is changed; multicast leaves with hop limit 1 |
| AC-3 | An OSPFv3-enabled interface comes up with a ready link-local address | The per-interface socket sets multicast hop limit 1, multicast interface = that interface, multicast loopback off, control-message flags for dst/ifindex/hoplimit, and joins `ff02::5`; the sender does not receive its own multicast |
| AC-4 | This router becomes DR (or BDR) on an interface, then loses the role | The transport joins `ff02::6` on that interface on becoming DR/BDR and leaves it on losing the role or on interface teardown |
| AC-5 | Engine directs a send to a unicast neighbour link-local address (e.g. a retransmission or DD) | The packet is unicast to that address on the interface socket with hop limit 1; no multicast group is used; the checksum is finalized for that destination; the body is otherwise unaltered |
| AC-6 | `interface/down` event on an open interface | RX/TX goroutines stop, joined groups are left, the socket is closed, and an engine teardown signal is emitted (no goroutine leak) |
| AC-7 | Raw IPv6 socket open without `CAP_NET_RAW` | Open fails with a clear error naming `CAP_NET_RAW`; `doctor-ospfv3-raw-socket` reports the missing capability |
| AC-8 | Received datagram shorter than the 16-byte OSPFv3 common header | The datagram is dropped before delivery; no panic; `ze_ospf_packets_dropped_total{reason="short"}` is incremented |
| AC-9 | `interface/up` while the interface link-local address is still tentative (DAD), then the address becomes ready | The open is marked pending and completes on the next rescan or `interface/addr-added`; no error is surfaced for the transient unavailability |
| AC-10 | A packet received on a veth pair is checksum-verified at the receiver | `packet.VerifyPacketChecksum(src, dst, payload)` returns true for a packet the transport finalized, proving the pseudo-header source matches the on-wire source (A-6) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up an OSPFv3-enabled interface | `interface/up` → link-local ready → transport open → multicast options set → `ff02::5` joined → RX/TX goroutines running | `TestOSPFv3TransportOpenOnLinkUp`, `TestOSPFv3TransportVethMulticastRoundTrip` (QEMU integration) |
| 2 | Has OSPFv3 send a Hello to the segment | final engine packet → checksum finalized → `WriteTo` `ff02::5` (kernel IPv6 header, hop limit 1) → peer RX delivers `(ifindex, src, dst, payload)`, `VerifyPacketChecksum` passes | `TestOSPFv3TransportSendMulticast`, `TestOSPFv3TransportVethMulticastRoundTrip` (QEMU integration) |
| 3 | This router is elected DR on a LAN | ospfv3-5 DR signal → transport joins `ff02::6` → DROther packets to `ff02::6` are received | `TestOSPFv3TransportJoinAllDRouters`, QEMU `ff02::6` receive case |
| 4 | Brings up an interface before its link-local is ready | `interface/up` (tentative) → pending → `interface/addr-added` / rescan → open completes | `TestOSPFv3TransportOpenPendingLinkLocal` |
| 5 | Looks up the OSPFv3 raw-socket readiness diagnostic before OSPFv3 config syntax exists | `ze explain doctor-ospfv3-raw-socket` → diagnostic metadata with `CAP_NET_RAW` guidance; registered-check execution is unit-tested with a synthetic OSPFv3 config tree until spec-ospfv3-4 lands config syntax | `ospfv3-doctor-raw-socket.ci`, `TestOSPFv3DoctorRawSocketCheckRegistered`, `TestOSPFv3CheckRawSocketUnavailable` |
| 6 | Takes the interface down | `interface/down` → leave groups → circuit close → engine tears down adjacencies | `TestOSPFv3TransportCloseOnLinkDown` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3MulticastGroupConstants` | `internal/plugins/ospfv3/transport/multicast_test.go` | `AllSPFRouters` = `ff02::5`, `AllDRouters` = `ff02::6`; bytes exact; `Protocol` = 89 | |
| `TestOSPFv3FinalizePacketChecksum` | `internal/plugins/ospfv3/packet/checksum_test.go` | `FinalizePacketChecksum(src,dst,pkt)` writes the value `PacketChecksum` returns at `offChecksum`; `VerifyPacketChecksum` then passes; idempotent | |
| `TestOSPFv3TransportFinalizesChecksumOnSend` | `internal/plugins/ospfv3/transport/transport_test.go` | a send with a zero checksum field is finalized for the egress src/dst before reaching the backend (recording backend captures finalized bytes; `VerifyPacketChecksum` passes) | |
| `TestOSPFv3TransportSendDoesNotAlterBody` | `internal/plugins/ospfv3/transport/transport_test.go` | the transport changes only the 2 checksum bytes and adds no IP header/padding; every other body byte is unchanged | |
| `TestOSPFv3SocketOptionsMulticast` | `internal/plugins/ospfv3/transport/transport_test.go` | on open the backend requests multicast hop limit 1, multicast interface = the interface, multicast loopback off, control-message flags dst/ifindex/hoplimit (recording backend) | |
| `TestOSPFv3JoinLeaveAllDRouters` | `internal/plugins/ospfv3/transport/transport_test.go` | a DR/BDR signal joins `ff02::6`; role-loss/teardown leaves it; idempotent join/leave | |
| `TestOSPFv3TransportOpenOnLinkUp` | `internal/plugins/ospfv3/transport/transport_test.go` | `interface/up` (link-local ready) opens, sets options, joins `ff02::5`, starts RX/TX (wiring) | |
| `TestOSPFv3TransportOpenPendingLinkLocal` | `internal/plugins/ospfv3/transport/transport_test.go` | `interface/up` with no link-local marks pending; `interface/addr-added`/rescan completes the open (wiring; A-7) | |
| `TestOSPFv3TransportSendMulticast` | `internal/plugins/ospfv3/transport/transport_test.go` | engine packet to `ff02::5` → backend `WriteTo` with the right destination, ifindex, hop limit (wiring) | |
| `TestOSPFv3TransportCloseOnLinkDown` | `internal/plugins/ospfv3/transport/transport_test.go` | `interface/down` leaves groups, closes the socket, signals teardown, no leak (wiring) | |
| `TestOSPFv3TransportReceiveCarriesDstHopLimit` | `internal/plugins/ospfv3/transport/transport_test.go` | a received datagram is delivered with the receiving ifindex, dst, and hop limit from the control message (recording backend injects them) | |
| `TestOSPFv3TransportDropsShortDatagram` | `internal/plugins/ospfv3/transport/transport_test.go` | a datagram shorter than the 16-byte common header is dropped with `reason="short"`; no panic | |
| `TestOSPFv3DoctorRawSocketUnavailable` | `internal/plugins/ospfv3/transport/doctor_test.go` | the check fires when OSPFv3 is configured and the raw socket cannot be opened; emits `doctor-ospfv3-raw-socket` | |
| `TestOSPFv3DoctorCodeRegistered` | `internal/plugins/ospfv3/transport/doctor_test.go` | `doctor-ospfv3-raw-socket` is registered in `internal/core/diagnostic/codes.go` and explainable | |
| `TestOSPFv3TransportMetricsSeries` | `internal/plugins/ospfv3/transport/metrics_test.go` | exactly four series register: `ze_ospf_packets_sent_total`, `ze_ospf_packets_received_total`, `ze_ospf_packets_dropped_total`, `ze_ospf_sockets_open` | |
| `TestOSPFv3ResolveInterfaceUsesIfaceResolver` | `internal/plugins/ospfv3/transport/backend_linux_test.go` | logical OSPFv3 names resolve through the shared iface resolver before the backend binds the socket (matches OSPFv2/IS-IS `os-name` behaviour) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Received datagram length | 16..65535 (min OSPFv3 common header 16) | 65535 | < 16 (dropped `reason="short"`) | > 65535 (datagram cap) |
| Multicast/unicast hop limit | 1 only (OSPFv3 link-local) | 1 | 0 (would not leave host) | > 1 (would route off-link) |
| Multicast group last byte | `ff02::5` (AllSPFRouters) / `ff02::6` (AllDRouters) | `::6` | n/a | any other rejected as a programming error |
| Checksum field offset | byte 12 (`offChecksum`) | 12 | n/a | n/a (fixed by the codec) |
| Instance ID (carried, not validated here) | 0..255 | 255 | n/a | n/a (validation is ospfv3-4) |

### Functional Tests
`.ci` files cover ONLY user-visible config/doctor output. The raw IPv6 / multicast
send-receive path is a kernel-capability path proven by a Linux-only QEMU
integration test (see QEMU Integration Tests below), NOT by a `.ci` file
(`ai/rules/qemu-testing.md`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-doctor-raw-socket` | `test/ospfv3/ospfv3-doctor-raw-socket.ci` | `ze doctor --json` / `ze explain` exposes the `doctor-ospfv3-raw-socket` result | |

### QEMU Integration Tests (MANDATORY for linux-only code — `ai/rules/qemu-testing.md`)
| Test | File | Build tags | End-User Scenario | Status |
|------|------|------------|-------------------|--------|
| `TestOSPFv3TransportVethMulticastRoundTrip` | `internal/plugins/ospfv3/transport/transport_integration_linux_test.go` | `integration && linux` | open the transport on a veth pair in a netns, join `ff02::5` on both, send a finalized packet to `ff02::5` on one, receive it on the peer with the correct ifindex, source, and dst; the peer `VerifyPacketChecksum` passes; the sender does not loop back (validates A-1..A-4, A-6) | |
| `TestOSPFv3TransportAllDRoutersReceive` | `internal/plugins/ospfv3/transport/transport_integration_linux_test.go` | `integration && linux` | after a DR/BDR join of `ff02::6`, a packet sent to `ff02::6` is received; after leave, it is not (validates A-5) | |
| `TestOSPFv3TransportRawSocketCap` | `internal/plugins/ospfv3/transport/transport_integration_linux_test.go` | `integration && linux` | raw IPv6 proto-89 open succeeds under `CAP_NET_RAW`; `t.Skip` when the capability is absent (validates A-8) | |
| `TestOSPFv3TransportPendingLinkLocalOnVeth` | `internal/plugins/ospfv3/transport/transport_integration_linux_test.go` | `integration && linux` | a freshly-created veth whose link-local is still in DAD opens once the address leaves the tentative state via the rescan path (validates A-7) | |

The `ze-qemu-integration-test` Makefile target auto-derives this package from its
`//go:build integration && linux` tag (`ZE_QEMU_INTEGRATION_PKGS` in
`mk/test-integration.mk`), so no Makefile edit is needed. The QEMU all-tests
evidence script `scripts/evidence/qemu-all-tests.sh` HARDCODES its package list
and does NOT derive from tags, so `./internal/plugins/ospfv3/transport/...` MUST
be added there explicitly (Files to Modify) or these tests are silently skipped
in the all-tests evidence run. The tests use a veth pair in a network namespace
(`t.Skip` when `CAP_NET_ADMIN`/`CAP_NET_RAW` are missing), exactly as the OSPFv2
and IS-IS siblings do.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred to ospfv3-13) | `test/interop/scenarios/` | FRR ospf6d | FRR receives Ze's `ff02::5` Hellos and vice versa; full adjacency needs the ISM (ospfv3-5) and NSM (ospfv3-6) | |

Wire-level transport correctness (link-local multicast send/receive, no IP-header
strip, hop limit 1, membership, checksum source match) is proven here by unit
tests and the QEMU veth integration tests. End-to-end FRR `ospf6d` interop
requires the ISM/NSM (ospfv3-5/ospfv3-6) and is owned by ospfv3-13; this is not a
deferral of this spec's own acceptance criteria.

### Future (if deferring any tests)
- FRR `ospf6d` interop of full OSPFv3 packets over this transport: owned by ospfv3-13 (needs ospfv3-5/ospfv3-6). Not a deferral of ospfv3-3 ACs.
- BSD / VPP backends behind the same interface: out of scope for v1 per the umbrella.
- GTSM (RFC 5082) inbound hop-limit-255 hardening: out of scope; base OSPFv3 uses hop limit 1 and the transport carries the received hop limit up for a future policy check without a transport change.

## Files to Modify
- `internal/plugins/ospfv3/packet/checksum.go` - add `FinalizePacketChecksum(src, dst netip.Addr, pkt []byte)` that computes `PacketChecksum` and writes it at `offChecksum` (making the `header.go` comment true); ospfv3-3 is its first consumer
- `internal/core/diagnostic/codes.go` - add the `doctor-ospfv3-raw-socket` code (title, description, examples), alongside `doctor-ospf-raw-socket`. ospfv3-3 OWNS this code; ospfv3-13 surfaces but must not re-register it
- `internal/component/iface/events/events.go` - no change expected; consume `EventUp`/`EventDown`/`EventAddrAdded` (read-only)
- `scripts/evidence/qemu-all-tests.sh` - add `./internal/plugins/ospfv3/transport/...` to the hardcoded integration-tests package list; this script does not derive packages from build tags (unlike `ze-qemu-integration-test`), so without this edit the transport integration tests are skipped in the all-tests evidence

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | per-interface OSPFv3 enrolment config is owned by ospfv3-4 |
| YANG validation constraints | No | n/a in this spec |
| YANG custom validators | No | n/a in this spec |
| CLI commands/flags | No | transport stats surfaced via ospfv3-13 `show ipv6 ospf interface` |
| CLI grammar (action before identifier) | No | n/a in this spec |
| Editor autocomplete | No | n/a in this spec |
| Functional test for new RPC/API | Yes | `test/ospfv3/ospfv3-doctor-raw-socket.ci` (user-visible doctor output); raw multicast send/receive proven by QEMU integration test |
| Pipe completeness | No | n/a (no command output in this spec) |
| Env var registration | No | n/a in this spec |
| Doctor check for runtime dependencies | Yes | `doctor-ospfv3-raw-socket` (raw IPv6 socket open / `CAP_NET_RAW`), owning-package registration + `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | this spec REUSES the shared OSPFv2 transport series `ze_ospf_packets_sent_total{interface,type}`, `ze_ospf_packets_received_total{interface,type}`, `ze_ospf_packets_dropped_total{interface,reason}`, `ze_ospf_sockets_open` (gauge). OSPFv3 is "our OSPF" so it does NOT fork a `ze_ospfv3_*` namespace; the registry is get-or-create by name so v2 and v3 share one series, and the `interface` label distinguishes them. NOTE: the no-label `ze_ospf_sockets_open` gauge is set per transport, so dual-stack accuracy needs Inc/Dec or a family label -- deferred to ospfv3-4 (which wires the production registry; v3 SetMetrics is unused until then) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | transport is internal; the user-facing OSPFv3 feature row is owned by ospfv3-13 |
| 2 | Config syntax changed? | No | config owned by ospfv3-4 |
| 3 | CLI command added/changed? | No | owned by ospfv3-13 |
| 4 | API/RPC added/changed? | No | none in this spec |
| 5 | Plugin added/changed? | Yes | edge plugin under `internal/plugins/ospfv3/`; user-facing docs owned by ospfv3-13 |
| 6 | Has a user guide page? | No | `docs/guide/ospfv3.md` owned by ospfv3-13 |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospfv3.md` (IPv6 proto 89 transport, `ff02::5`/`ff02::6` multicast, hop limit 1, no IP-header strip, address-bound checksum finalized on TX) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc5340.md` (§2.9 addressing/multicast, §A.3.1 checksum) |
| 10 | Test infrastructure changed? | Yes | the raw IPv6 / multicast send/receive tests are QEMU integration tests (`transport_integration_linux_test.go`, `integration && linux`), documented under QEMU/integration testing; only the user-visible doctor test `test/ospfv3/ospfv3-doctor-raw-socket.ci` is a `docs/functional-tests.md` entry |
| 11 | Affects daemon comparison? | No | owned by ospfv3-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new OSPFv3 transport layer; first raw IPv6 proto-number multicast transport) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (the four `ze_ospf_packets_*` / `ze_ospf_sockets_open` rows) |
| 15 | Registered plugin/event/command/capability changed? | Yes | doctor code `doctor-ospfv3-raw-socket` registered; note in `docs/guide/status.md` via ospfv3-13 |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/plugins/ospfv3/transport/transport.go` - the `Backend` interface (open/close per interface, send `(packet, destination selector)`, receive `RawPacket`, join/leave `ff02::6` on DR/BDR change), the `Transport` orchestrator (per-interface registry, iface EventBus subscription with a bounded worker queue + periodic rescan backstop, send fan-out with checksum finalization, receive fan-in), and the `RawPacket{IfIndex,Src,Dst,HopLimit,Payload}` delivery type. The transport finalizes only the checksum and never parses the OSPFv3 `Type`
- `internal/plugins/ospfv3/transport/multicast.go` - `Protocol` (89), `AllSPFRouters` (`ff02::5`) / `AllDRouters` (`ff02::6`) constants and the destination-selector type (multicast group or unicast link-local address)
- `internal/plugins/ospfv3/transport/backend_linux.go` - `//go:build linux` raw IPv6 proto-89 backend: shared iface resolver lookup before bind, link-local source selection, `net.ListenPacket("ip6:89","::")` + `ipv6.NewPacketConn`, `SO_BINDTODEVICE`, `SetMulticastHopLimit(1)`, `SetMulticastInterface`, `SetMulticastLoopback(false)`, `SetHopLimit(1)`, `SetControlMessage(FlagDst|FlagInterface|FlagHopLimit)`, `JoinGroup`/`LeaveGroup`, `ReadFrom` RX with control-message attribution, `WriteTo` TX, a read deadline for stop-signal wakeups
- `internal/plugins/ospfv3/transport/backend_other.go` - `//go:build !linux` stub returning a not-supported error so non-Linux builds compile and config/unit tests run
- `internal/plugins/ospfv3/transport/doctor.go` - platform-neutral doctor check body (fires when OSPFv3 is configured and the raw socket is unavailable), modelled on the OSPFv2 / IS-IS split
- `internal/plugins/ospfv3/transport/doctor_linux.go` - `//go:build linux` raw IPv6 proto-89 socket probe (open+close; EPERM → unavailable), modelled on `internal/plugins/ospf/transport/doctor_linux.go`
- `internal/plugins/ospfv3/transport/doctor_other.go` - `//go:build !linux` probe stub
- `internal/plugins/ospfv3/transport/register.go` - doctor-check registration from `init()` via `diagnostic.RegisterDoctorCheck`
- `internal/plugins/ospfv3/transport/metrics.go` - reuses the shared OSPFv2 `ze_ospf_*` transport series (no forked `ze_ospfv3_*` namespace)
- `internal/plugins/ospfv3/transport/transport_test.go` - finalize-on-send, send-does-not-alter-body, socket-options, join/leave, wiring (open/pending/send/close), receive-carries-dst-hoplimit, short-datagram drop, boundary tests
- `internal/plugins/ospfv3/transport/multicast_test.go` - multicast group + protocol constants byte-exact
- `internal/plugins/ospfv3/transport/doctor_test.go` - doctor check + registration unit tests
- `internal/plugins/ospfv3/transport/metrics_test.go` - exact-series registration test
- `internal/plugins/ospfv3/transport/transport_integration_linux_test.go` - `//go:build integration && linux` QEMU veth link-local multicast round-trip + AllDRouters receive + `CAP_NET_RAW` probe + pending-link-local (the raw IPv6 / multicast path lives here, NOT in a `.ci` file)
- `test/ospfv3/ospfv3-doctor-raw-socket.ci` - functional doctor-check test (user-visible `ze doctor --json` / `ze explain` output only)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-qemu-integration-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — define the `Backend` interface, `RawPacket{IfIndex,Src,Dst,HopLimit,Payload}`, destination-selector type, and per-interface registry; subscribe to the iface EventBus; write failing wiring tests
   - Tests: `TestOSPFv3TransportOpenOnLinkUp`, `TestOSPFv3TransportCloseOnLinkDown`, `TestOSPFv3TransportSendMulticast`, `TestOSPFv3TransportOpenPendingLinkLocal`
   - Files: `transport.go`, `multicast.go`, `backend_other.go` stub
   - Verify: `interface/up`/`interface/down`/`interface/addr-added` reach the transport; backend methods are stubs so wiring tests fail on missing I/O
2. **Phase: Checksum finalize + send-does-not-alter** — add `packet.FinalizePacketChecksum`; the send path finalizes the checksum and adds no IP header/padding
   - Tests: `TestOSPFv3FinalizePacketChecksum`, `TestOSPFv3TransportFinalizesChecksumOnSend`, `TestOSPFv3TransportSendDoesNotAlterBody`, `TestOSPFv3MulticastGroupConstants`, boundary tests
   - Files: `internal/plugins/ospfv3/packet/checksum.go`, `transport.go`, `multicast.go`
   - Verify: the finalized bytes verify with `VerifyPacketChecksum`; only the 2 checksum bytes change; the short-datagram drop path works
3. **Phase: Linux `ipv6.PacketConn` backend** — proto-89 socket open, link-local source selection, multicast options (hop limit 1, interface, loopback off, control-message flags), `JoinGroup` `ff02::5`, `ReadFrom` RX with dst/ifindex/hoplimit attribution, `WriteTo` TX, read deadline, pending-open on missing link-local
   - Tests: `TestOSPFv3SocketOptionsMulticast`, `TestOSPFv3TransportReceiveCarriesDstHopLimit`, `TestOSPFv3ResolveInterfaceUsesIfaceResolver`, `transport_integration_linux_test.go` (QEMU veth round-trip + pending-link-local)
   - Files: `backend_linux.go`
   - Verify: packet sent on one veth received on the peer with the correct ifindex/src/dst; peer `VerifyPacketChecksum` passes; sender does not loop back; resolve A-1/A-2/A-3/A-6/A-7 and record the outcome
4. **Phase: AllDRouters membership on DR/BDR change** — per-interface `ff02::6` join on a DR/BDR signal, leave on role loss / teardown
   - Tests: `TestOSPFv3JoinLeaveAllDRouters`, `TestOSPFv3TransportJoinAllDRouters`, QEMU `TestOSPFv3TransportAllDRoutersReceive`
   - Files: `transport.go`, `backend_linux.go`
   - Verify: joining `ff02::6` makes DROther-addressed packets receivable; leaving stops them; join/leave idempotent
5. **Phase: Doctor check + metrics** — `doctor-ospfv3-raw-socket` code + check, the four transport Prometheus series
   - Tests: `TestOSPFv3DoctorRawSocketUnavailable`, `TestOSPFv3DoctorCodeRegistered`, `TestOSPFv3TransportMetricsSeries`
   - Files: `doctor.go`, `doctor_linux.go`, `doctor_other.go`, `register.go`, `metrics.go`, `internal/core/diagnostic/codes.go`
   - Verify: `ze doctor --json` exposes the code; the doctor-coverage test passes; exactly four series register
6. **Functional + QEMU integration tests** → `test/ospfv3/ospfv3-doctor-raw-socket.ci` and `transport_integration_linux_test.go`. Add the package to the hardcoded list in `scripts/evidence/qemu-all-tests.sh`
7. **RFC refs** → Add `// RFC 5340 Section 2.9 ...` / `// RFC 5340 Section A.3.1 ...` comments on the addressing / multicast / hop-limit / checksum code
8. **Full verification** → `make ze-verify` + `make ze-qemu-integration-test`
9. **Complete spec** → fill audit tables, write learned summary to `plan/learned/NNN-ospfv3-3-ipv6-transport.md`; two-commit closure (held until the user commits, per the OSPF workflow)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Feature completeness | Open/close + send (multicast + unicast, checksum finalized) + receive (dst/ifindex/hoplimit) + `ff02::5` join + `ff02::6` DR/BDR join/leave + hop limit 1 + loop-off + pending-link-local + link-event lifecycle + doctor all present; the backend interface lets a second backend drop in |
| Correctness | raw IPv6 proto 89 via `ipv6.PacketConn`; no IP-header strip; `src` from `ReadFrom`, `dst`/`ifindex`/`hoplimit` from the control message; multicast groups `ff02::5`/`ff02::6` byte-exact; hop limit 1; loopback off; the checksum is finalized for the egress src/dst and otherwise the body is byte-for-byte |
| Naming | Package `transport`; doctor code `doctor-ospfv3-raw-socket`; backend files `backend_linux.go`/`backend_other.go`; metric series reuse the shared OSPFv2 `ze_ospf_*` names (no `ze_ospfv3_*` fork) |
| Data flow | The engine never touches the socket; datagrams flow socket → `RawPacket` → engine; the `Type` dispatcher and checksum verification are ospfv3-4; iface events drive lifecycle; DR/BDR signal drives `ff02::6` membership |
| CLI grammar | n/a (no CLI in this spec) |
| Doctor checks | `doctor-ospfv3-raw-socket` registered per `ai/rules/doctor-checks.md`; unit + functional test present |
| YANG validation | n/a (config owned by ospfv3-4) |
| Prometheus counters | the shared OSPFv2 `ze_ospf_*` transport series reused here (no forked `ze_ospfv3_*` namespace); `interface` label distinguishes v2/v3 |
| Rule: qemu-testing | the Linux backend has an `integration && linux` QEMU test; no hardware-only skip |
| Rule: plugin-self-containment | all transport code + the doctor check under `internal/plugins/ospfv3/transport/` |
| Rule: no-cross-version | shares no code with the OSPFv2 transport; only the orchestrator shape is mirrored |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Transport package | `ls internal/plugins/ospfv3/transport/` |
| Linux backend + non-linux stub | `ls internal/plugins/ospfv3/transport/backend_linux.go internal/plugins/ospfv3/transport/backend_other.go` |
| Checksum finalize helper | `grep -n 'func FinalizePacketChecksum' internal/plugins/ospfv3/packet/checksum.go` |
| Doctor code registered | `grep doctor-ospfv3-raw-socket internal/core/diagnostic/codes.go` |
| Shared `ze_ospf_*` series reused (no `ze_ospfv3_*` fork) | `grep -E 'ze_ospf_packets_(sent\|received\|dropped)_total\|ze_ospf_sockets_open' internal/plugins/ospfv3/transport/metrics.go` returns the 4 shared names; `grep ze_ospfv3_ internal/plugins/ospfv3/transport/metrics.go` returns nothing |
| Functional test (user-visible) | `ls test/ospfv3/ospfv3-doctor-raw-socket.ci` |
| QEMU integration test (raw IPv6 / multicast) | `ls internal/plugins/ospfv3/transport/transport_integration_linux_test.go`; auto-derived via its `integration && linux` tag; `grep ospfv3/transport scripts/evidence/qemu-all-tests.sh` confirms the explicit add |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every received datagram validated (min 16-byte common header) before delivery; reject not panic; the dropped counter increments with a reason |
| Resource exhaustion | Bounded receive buffer; per-interface goroutine count tied to link state; no unbounded receive queue; bounded EventBus worker queue |
| Privilege | `CAP_NET_RAW` only; the doctor check surfaces the requirement |
| Spoofing | Source/destination addresses and receiving ifindex recorded for the engine; the checksum binds to src/dst (a spoofed source fails `VerifyPacketChecksum` at ospfv3-4); version/area/auth enforcement is ospfv3-4/ospfv3-12 |
| Off-link reach | Hop limit 1 on multicast and unicast so OSPFv3 packets cannot be routed off the link; link-local source confines scope; assert the hop-limit option |
| Crafted datagrams | A truncated datagram must not over-read or panic; the short-datagram drop path is the only structural guard, validation is ospfv3-4 |
| Checksum source integrity | The finalized checksum must use the actual on-wire link-local source (A-6); a wrong source silently fails verification at peers — proven by the veth round-trip |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; re-check addressing against RFC 5340 §2.9/§A.3.1 and the OSPFv2/PPP-RA models |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional/QEMU test fails | Check AC; if A-1 (`ipv6.PacketConn` raw bind), A-2 (membership), A-3 (control message), or A-6 (checksum source) is the cause, fall back per the Risks table and record |
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
The OSPFv3 transport is the OSPFv2 transport orchestrator with the IPv4 raw
socket swapped for a `golang.org/x/net/ipv6.PacketConn`, and it carries ONE
responsibility the IPv4 sibling does not: because the OSPFv3 packet checksum is an
IPv6 upper-layer checksum bound to the source and destination addresses (RFC 5340
§A.3.1), and the transport is the only layer that knows the on-wire link-local
source, the transport finalizes the checksum on TX and carries `src`/`dst` up for
verification on RX. Everything else is a near-mechanical IPv4→IPv6 substitution:
no IP-header strip (IPv6 raw sockets deliver the upper-layer payload), `dst`/
`ifindex`/`hoplimit` from the ancillary control message instead of socket-per-
interface attribution, `ff02::5`/`ff02::6` instead of `224.0.0.5`/`224.0.0.6`,
hop limit 1 via `SetMulticastHopLimit` instead of `IP_MULTICAST_TTL`, and a
link-local source that can lag link-up (IPv6 DAD), forcing a pending-open retry
the IPv4 sibling never needed.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Mirror the OSPFv2 `Backend`/`InterfaceHandle`/`Transport` orchestrator shape (per-interface socket) | A single shared raw IPv6 socket dispatching by `ControlMessage.IfIndex` | keeps ospfv3-4/5/6 engine wiring identical to ospf; a shared socket would diverge the higher-layer contract for marginal FD savings; per-interface membership and source selection stay simple |
| `golang.org/x/net/ipv6.PacketConn` over `net.ListenPacket("ip6:89","::")` | raw `unix.Socket(AF_INET6, SOCK_RAW, 89)` + manual `recvmsg`/`sendmsg` cmsg handling | the in-tree IPv6 multicast pattern (PPP RA) already uses `ipv6.PacketConn` with `JoinGroup` + `ControlMessage`; it gives dst/ifindex/hoplimit without hand-rolled cmsg parsing; the raw-`unix` path is the documented fallback if a raw proto-89 `PacketConn` cannot surface the control message |
| The transport finalizes the checksum on TX and carries `src`/`dst` up for ospfv3-4 to verify | the codec self-computes (impossible — no addresses); the engine finalizes (must first ask the transport for the egress source — more coupling) | only the transport knows the on-wire link-local source; the codec deliberately leaves the field zero; this also makes the RFC 7166 auth seam clean — "signer present ⇒ skip finalize" is the only change ospfv3-12 needs |
| Add `packet.FinalizePacketChecksum` to the ospfv3-2 codec | the transport writes `offChecksum` bytes by hand | the `header.go` comment already promises the helper; one codec call (compute + write) is cleaner and testable, and removes a stale-comment gap from ospfv3-2 |
| No IP-header strip; `dst`/`ifindex`/`hoplimit` from the control message | port the OSPFv2 `StripIPv4Header` | IPv6 raw sockets deliver the upper-layer payload with no IP header; attribution data is ancillary, not in the datagram |
| Hop limit 1 (link-local), no GTSM-255 | GTSM hop-limit-255 like BFD | base OSPFv3 uses hop limit 1 and link-local source confinement; GTSM is a BFD (RFC 5881) requirement, not OSPF's; the transport carries the received hop limit up so a future policy check needs no transport change |
| Pending-open retried by the rescan and `interface/addr-added` | open once on `interface/up` and fail if the link-local is tentative | IPv6 DAD assigns the link-local ~1s after link-up; a one-shot open would never form an adjacency on a freshly-upped interface |
| Transport never parses the OSPFv3 `Type`; dispatch and validation are ospfv3-4 | transport switches on the common-header `Type` and verifies the checksum | umbrella "Packet Receive Path" contract: the transport hands raw packets up; the `Type` dispatcher and version/Instance ID/area/checksum validation live in ospfv3-4 |
| Split doctor into `doctor.go` (neutral) + `doctor_linux.go`/`doctor_other.go` (probe) | a single `doctor_linux.go` | platform-neutral check body testable on any OS; the probe is the only platform-specific part (the OSPFv2/IS-IS siblings did the same) |

## Known Limitations
- v1 ships only the Linux `ipv6.PacketConn` backend; BSD/VPP backends are out of scope (umbrella).
- Only the Broadcast and Point-to-Point network types are in umbrella scope: the transport supports the `ff02::5`/`ff02::6` multicast and unicast link-local destination cases those need; NBMA/P2MP/virtual-link are future.
- Full FRR `ospf6d` interop over this transport is proven in ospfv3-13 (needs the ospfv3-5 ISM and ospfv3-6 NSM); ospfv3-3 proves transport correctness by unit tests and the QEMU veth multicast send/receive integration test.
- The transport does NOT validate version/Instance ID/area, does NOT verify the checksum, and does NOT dispatch by packet `Type`; those are owned by ospfv3-4 (`instance.go`) and ospfv3-12 (auth). GTSM hop-limit hardening is deferred.

## RFC Documentation

Add `// RFC 5340 Section X.Y: "<quoted requirement>"` above enforcing code:
- §2.9 — IPv6 protocol 89, link-local source, `ff02::5`/`ff02::6` multicast (addressing/multicast/hop-limit code).
- §A.3.1 — IPv6 upper-layer checksum with Next Header 89 over the OSPF packet length (checksum finalize/verify code).
- RFC 7166 §2.2 — checksum omitted when the Authentication Trailer is present (the `SetSigner` seam; full enforcement in ospfv3-12).

## Implementation Summary

### What Was Implemented
- [filled at completion]

### Bugs Found/Fixed
- [filled at completion]

### Documentation Updates
- [filled at completion]

### Deviations from Plan
- [filled at completion]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Raw IPv6 proto-89 send/receive with link-local source and `ff02::5`/`ff02::6` | QEMU integration test | `TestOSPFv3TransportVethMulticastRoundTrip` |
| Address-bound checksum finalized on TX, verifiable at the peer | QEMU integration test | round-trip asserts peer `VerifyPacketChecksum` passes (A-6) |
| `ff02::6` membership toggles on DR/BDR | QEMU integration test | `TestOSPFv3TransportAllDRoutersReceive` |
| Raw-socket dependency surfaced before runtime | functional + unit test | `ospfv3-doctor-raw-socket.ci`, `TestOSPFv3DoctorRawSocketUnavailable` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / acknowledged |

### Fixes applied
- [filled during review]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | Test | Verified |
|-------------|------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the backend interface has the Linux backend + a future BSD/VPP backend as the 2nd+ use case)
- [ ] No speculative features (needed NOW for ospfv3-5/6/7)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] QEMU integration tests for the linux-only raw socket path
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-3-ipv6-transport.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump (held until the user commits, per the OSPF workflow)
- [ ] **Commit B:** `git rm plan/<spec>` only
