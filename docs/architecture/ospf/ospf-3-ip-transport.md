# OSPFv2 raw IPv4 transport

`internal/plugins/ospf/transport` opens the raw IPv4 protocol-89 sockets, joins
the OSPF multicast groups and carries datagrams to and from the engine.

## Decisions

- **`AF_INET/SOCK_RAW` protocol 89, not `IP_HDRINCL`.** RSVP-TE already proves
  the kernel-built IP header model in this tree. `IP_HDRINCL` would add manual
  IP header and checksum code.
  <!-- source: internal/plugins/ospf/transport/backend_linux.go -- OpenInterface -->
- **One receive socket and one transmit socket per interface.**
  `IP_MULTICAST_LOOP=0` belongs on transmit, while receive must still accept
  peer multicast on the same link. A single socket cannot hold both.
  Consequence: `ze_ospf_sockets_open` counts two sockets per open interface.
  <!-- source: internal/plugins/ospf/transport/metrics.go -- socketsOpen -->
- **Interface names resolve through the iface resolver, never
  `net.InterfaceByName`.** OSPF config names are Ze logical names, so the
  `os-name` and `mac/match` selectors must keep working.
- **Multicast membership uses `IPMreq` bound to the interface IPv4 address, not
  `IPMreqn`.** QEMU raw multicast receive matched the address-bound membership
  path.
  <!-- source: internal/plugins/ospf/transport/multicast.go -- AllSPFRouters, AllDRouters -->
- **Transport metrics label packet TYPE only, and do not validate.**
  Common-header dispatch and validation belong to the runtime packages. The
  transport owns receive-side malformed IPv4 drops before dispatch, counted as
  `ze_ospf_packets_dropped_total{reason="malformed-ipv4"}`.

## Constraints on callers

- Runtime code calls the transport for sends, receives, AllDRouters membership
  and interface teardown. It never opens a socket itself.
- A unicast OSPF send uses the same transmit socket as multicast, so that socket
  sets `IP_TTL=1` AND `IP_MULTICAST_TTL=1`. `IP_MULTICAST_TTL` alone does not
  cover unicast retransmissions or Database Description packets.

## Traps

- Same-netns veth shows packets in tcpdump while the raw socket receive stays
  silent. Use a peer network namespace for multicast transport tests.
- Disabling multicast loopback on a single socket suppresses the receive
  evidence needed for debugging. Split transmit and receive first.
- A doctor diagnostic-code test is not enough. Assert
  `diagnostic.DoctorCheckNames()` so that removing `register.go` or its blank
  import fails the test.
  <!-- source: internal/plugins/ospf/transport/register.go -- init -->
  <!-- source: internal/plugins/ospf/transport/doctor.go -- checkOSPFRawSocket -->
