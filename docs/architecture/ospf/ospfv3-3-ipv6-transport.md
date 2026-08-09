# OSPFv3 raw IPv6 transport

`internal/plugins/ospf/v3/transport` opens the per-interface raw IPv6 protocol-89
sockets, joins `ff02::5` and `ff02::6`, and carries OSPFv3 datagrams to and from
the engine. It adds the one responsibility the OSPFv2 transport does not have:
it finalizes the address-bound checksum on transmit.

## Decisions

- **Mirror the OSPFv2 orchestrator shape and swap only the socket internals.**
  The per-interface registry, the iface event subscription, the rescan backstop
  and the link lifecycle are the same, so the engine wiring matches OSPFv2. The
  IPv6 divergences are confined to the backend and three orchestrator points:
  the raw packet gains a destination and a hop limit, send takes an explicit
  source, and the receive loop demultiplexes the Instance ID.
  <!-- source: internal/plugins/ospf/v3/transport/transport.go -- Transport, SendPacket -->
- **`golang.org/x/net/ipv6.PacketConn`, not raw syscalls.** The in-tree IPv6
  multicast pattern already uses it with `JoinGroup` and a per-packet control
  message. An IPv6 raw socket delivers the upper-layer payload with NO IP header
  to strip, unlike the OSPFv2 IPv4 raw socket. The source comes from `ReadFrom`,
  and destination, ifindex and hop limit come from the control message.
  <!-- source: internal/plugins/ospf/v3/transport/backend_linux.go -- open, readLoop -->
- **The transport finalizes the checksum and binds the send source.** RFC 5340
  Appendix A.3.1 binds the checksum to the IPv6 source and destination, the
  codec leaves the field zero, and only the transport knows the on-wire source.
  Send computes the checksum and sets the same link-local address in the control
  message, so the kernel emits from exactly the address the checksum covers.
- **Kernel checksum offload was rejected.** `SetChecksum(true, 12)` is the
  canonical Go idiom and would remove the source-match risk, and the always-on
  socket option cannot leave the checksum zero for RFC 7166 signed packets.
  User-space finalization also reuses the golden-tested codec.
- **The transport owns the Instance ID demultiplex (RFC 5340 Section 4.2.1).**
  Instance ID is a per-interface receive filter. The engine supplies it when it
  enables an interface, and the receive loop reads header byte 14 and drops a
  mismatch. It is the only header field the transport reads. Version, area and
  checksum stay in the engine.
  <!-- source: internal/plugins/ospf/v3/packet/header.go -- PeekInstanceID -->
- **OSPFv3 is "our OSPF" for the operator: it reuses the `ze_ospf_*` metric
  series and does not fork a `ze_ospfv3_*` namespace.** The registry is
  get-or-create by name, so the two transports share one series and the
  `interface` label distinguishes the traffic.
  <!-- source: internal/plugins/ospf/v3/transport/metrics.go -- metrics -->

## Traps

- **FRR `ospf6d` drops a received multicast OSPFv3 packet whose IPv6 hop limit
  is not exactly 1**, citing RFC 5340 Appendix A.1. Sending a GTSM hop limit of
  255 makes FRR discard every Hello and no adjacency forms. Hop limit 1 is
  mandatory for interop. Base OSPFv3 does not use GTSM, which is BFD behaviour.
  The link-local source provides the on-link confinement.
- **The address-bound checksum reproduces the "self-consistent but wrong at a
  real peer" failure class unless the on-wire source is bound.** If the
  pseudo-header source differs from the source the kernel selects, every packet
  fails verification at the peer while a same-host veth round-trip can still
  pass. The test asserts the PEER verification.
- **The IPv6 link-local source can lag link-up by about one second, because of
  Duplicate Address Detection.** An open that finds no `fe80::` source returns a
  no-link-local error and is retried by the periodic rescan and by the
  address-added event. The IPv4 sibling never had this wrinkle.
