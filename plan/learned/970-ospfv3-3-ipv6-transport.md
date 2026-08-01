# 970 - OSPFv3 raw IPv6 transport (spec-ospfv3-3)

## Context

Third implementation child of the OSPFv3 (RFC 5340) umbrella. Created
`internal/plugins/ospfv3/transport/` -- the raw IPv6 proto-89 transport that opens
per-interface sockets, joins `ff02::5`/`ff02::6`, and carries OSPFv3 datagrams
to/from the engine. It mirrors the orchestration of the OSPFv2 transport
(`internal/plugins/ospf/transport/`) but replaces every IPv4 raw-socket internal
with the IPv6 equivalent, and adds the one responsibility OSPFv2 does not have:
finalizing the address-bound IPv6 checksum on transmit.

## Decisions

- **Mirror the OSPFv2 `Backend`/`InterfaceHandle`/`Transport` orchestrator shape,
  swap only the socket internals.** `transport.go` is a near-verbatim copy of the
  OSPFv2 orchestrator (per-interface registry, iface EventBus subscription,
  rescan backstop, link lifecycle) so the later ospfv3-4/5/6 wiring matches ospf.
  The IPv6 divergences are confined to the backend and three orchestrator points:
  `RawPacket` gains `Dst`/`HopLimit`; `Send` takes the explicit source; the RX
  loop does the Instance ID demux.
- **`golang.org/x/net/ipv6.PacketConn` over `net.ListenPacket("ip6:89","::")`,
  not raw `unix` syscalls.** The in-tree IPv6 multicast pattern (PPP RA) already
  uses `ipv6.PacketConn` with `JoinGroup` + a per-packet `ControlMessage`. IPv6
  raw sockets deliver the upper-layer payload with **no IP header to strip**
  (unlike the OSPFv2 IPv4 raw socket); `src` comes from `ReadFrom` and
  `dst`/`ifindex`/`hopLimit` from the control message after
  `SetControlMessage(FlagDst|FlagInterface|FlagHopLimit, true)`.
- **The transport finalizes the IPv6 upper-layer checksum on TX and binds the
  send source to the pseudo-header source.** RFC 5340 §A.3.1 binds the checksum
  to the IPv6 src/dst, the codec leaves the field zero, and only the transport
  knows the on-wire source. `SendPacket` calls `packet.FinalizePacketChecksum`
  (a helper this spec added to the ospfv3-2 codec) and sets the same link-local in
  `WriteTo`'s `ControlMessage.Src` (IPV6_PKTINFO) so the kernel emits from exactly
  the address the checksum covers. On RX the transport carries `src`/`dst` up so
  ospfv3-4 verifies over `payload[:Length]`.
- **The transport does the Instance ID demux (RFC 5340 §4.2.1), not ospfv3-4.**
  Instance ID is a per-interface receive filter the transport owns: the umbrella
  assigns it to the transport and names `TestOSPFv3TransportRejectsWrongInstance`
  there. ospfv3-4 supplies each interface's Instance ID via
  `EnableInterface(name, instanceID)`; the RX loop reads byte 14 via
  `packet.PeekInstanceID` (the only header field the transport reads) and drops a
  mismatch. Version/area/checksum stay in ospfv3-4.
- **OSPFv3 is "our OSPF" for the operator: REUSE the `ze_ospf_*` metric series, do
  NOT fork a `ze_ospfv3_*` namespace** (user directive). The metrics registry is
  get-or-create by name, so the v2 and v3 transports share one series; the
  `interface` label distinguishes v2 from v3 traffic. The 3 counters are additive
  and clean; the no-label `ze_ospf_sockets_open` gauge is `Set` per transport, so
  dual-stack accuracy needs Inc/Dec or a family label (deferred to ospfv3-4, which
  wires the production registry -- v3 `SetMetrics` is unused until then). The v2
  `ze_ospf_sockets_open` help was neutralised (dropped "IPv4") so the shared
  series is consistent regardless of which plugin registers first. This generalises:
  later OSPFv3 surfaces (CLI, the engine-level metrics the umbrella listed as
  `ze_ospfv3_*`) should likewise be reconsidered for the unified `ze_ospf_*` /
  "OSPF" naming rather than a parallel v3 namespace.

## Gotchas

- **FRR `ospf6d` REQUIRES received multicast OSPFv3 packets to have IPv6 Hop
  Limit exactly 1, citing RFC 5340 Appendix A.1.** The independent design review
  found this in the FRR source (`ospf6_message.c`: `if (IN6_IS_ADDR_MULTICAST(&dst)
  && hoplim != 1) drop`). Sending GTSM hop-limit-255 would make FRR silently drop
  every Hello and adjacency would never form. Hop limit 1 is not just acceptable,
  it is mandatory for interop; base OSPFv3 does NOT use GTSM (that is BFD's RFC
  5881 behaviour). The link-local source provides on-link confinement.
- **The address-bound checksum reproduces the ospfv3-2 "self-consistent but wrong
  at a real peer" failure class unless the on-wire source is bound.** If the
  pseudo-header source differs from the source the kernel actually puts on the
  wire (multiple `fe80::`, kernel source selection), every packet fails
  verification at the peer while a same-host veth round-trip might still pass.
  Fix: set `ControlMessage.Src` to the selected link-local (the design, not a
  fallback). The QEMU round-trip asserts the *peer* `VerifyPacketChecksum` to
  catch it.
- **`ipv6.PacketConn.SetChecksum(true, 12)` kernel offload was rejected** even
  though it is the canonical Go OSPFv3 idiom and would eliminate the source-match
  risk outright: the always-on socket option cannot leave the checksum zero for
  RFC 7166 signed packets (the auth seam), and user-space finalization reuses the
  golden-tested ospfv3-2 codec. `cm.Src` binding recovers the source guarantee.
- **The IPv6 link-local source can lag link-up (Duplicate Address Detection,
  ~1s).** An open that finds no `fe80::` source returns `ErrNoLinkLocal` and is
  retried by the periodic rescan and `interface/addr-added` -- a wrinkle the
  IPv4 sibling never had. The QEMU test opens with a retry loop for the same
  reason.
- **An independent spec design review (no BLOCKER, 3 ISSUEs) was worth it before
  coding.** It confirmed hop-limit-1 against the FRR source, hardened the checksum
  source binding from a fallback to the design, flagged that ospfv3-4 must verify
  over `payload[:Length]` (trailing LLS bytes), and reconciled the Instance ID
  placement with the umbrella. The ospfv3-2 lesson (round-trips hide wire errors)
  generalises: review the design against the RFC and a reference implementation
  before building the tests that would otherwise lock in the wrong assumption.

## Verification anchors

- `go test -race ./internal/plugins/ospfv3/...` clean (packet + types + transport)
  on darwin (backend_other stub); `GOOS=linux go vet ./internal/plugins/ospfv3/...`
  and `-tags integration` both clean (backend_linux + the QEMU test compile).
  `make ze-doc-test` passes (wire/ospfv3.md, core-design.md, metrics anchors).
- Unit tests: orchestrator wiring (open/close/send/pending), `FinalizePacketChecksum`
  on send + `cm.Src` binding, Instance ID demux drop, short-datagram drop,
  receive carries dst/hop-limit, ff02::6 join/leave, metrics series, doctor
  (`doctor-ospfv3-raw-socket` registered + fires). QEMU integration
  (`transport_integration_linux_test.go`, `integration && linux`): veth
  link-local multicast round-trip asserting peer `VerifyPacketChecksum` (A-6),
  ff02::6 receive after join / silence after leave (A-5), CAP_NET_RAW probe,
  pending-link-local open. **The QEMU tests are written and compile but run only
  on a Linux host with CAP_NET_RAW/CAP_NET_ADMIN (CI/QEMU), not the dev host.**
- Codec additions to ospfv3-2: `packet.FinalizePacketChecksum` (makes the stale
  `header.go` comment true) and `packet.PeekInstanceID` (the transport's demux
  accessor), each with a unit test.
- Next OSPFv3 target (umbrella): `spec-ospfv3-4-plugin-config.md` (plugin
  registration, YANG `ospfv3` config tree, instance lifecycle wiring the
  transport's `NewBackend`/`EnableInterface`/`Receive`/`SendPacket`).

## Files

None recorded.
