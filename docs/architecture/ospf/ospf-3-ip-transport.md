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
- **Multicast membership and the multicast egress use `IPMreqn`, naming the
  device by ifindex and the source by the interface IPv4 address.** An address
  alone selects the first device holding it. On an unnumbered link the donor
  holds the same address as the borrowed /32, so an address-bound membership
  lands on the donor and `IP_MULTICAST_IF` by address is refused with EINVAL
  on a socket bound to the unnumbered device. The ifindex comes from the iface
  resolver, whose cache a link event clears when a device is re-created.
  <!-- source: internal/plugins/ospf/transport/backend_linux.go -- multicastDevice, joinGroup, setMulticastOptions -->
  <!-- source: internal/plugins/ospf/transport/multicast.go -- AllSPFRouters, AllDRouters -->
- **On a point-to-point interface, a packet meant for one neighbor goes to
  AllSPFRouters.** RFC 2328 Section 8.1 sets that destination for every packet
  on a physical point-to-point network: Database Description, Link State
  Request, a retransmitted or requested Link State Update and a direct Link
  State Acknowledgment, as well as Hellos and floods. Every other network type
  sends those to the neighbor's address. On an unnumbered link the neighbor's
  address is its donor, which no route on the link names, so a unicast never
  arrives: FRR sat in ExStart until this rule was applied. The neighbor table
  and the database both ask `PacketDestination`; OSPFv3 addresses pass through.
  <!-- source: internal/plugins/ospf/transport/multicast.go -- PacketDestination -->
  <!-- source: internal/plugins/ospf/neighbor/dd.go -- sendDBDescLocked, resendLastDDLocked -->
  <!-- source: internal/plugins/ospf/lsdb/flooding.go -- linkDestination -->
- **Transport metrics label packet TYPE only, and do not validate.**
  Common-header dispatch and validation belong to the runtime packages. The
  transport owns receive-side malformed IPv4 drops before dispatch, counted as
  `ze_ospf_packets_dropped_total{reason="malformed-ipv4"}`.
- **Receive validates the IPv4 envelope before queueing a payload.** It checks
  version, header and total lengths, checksum, protocol 89, and destination.
  The destination must be the interface address or an OSPF multicast group.
  Source, destination and TTL remain available to the engine. The engine uses the
  source for the RFC 2328 section 8.2 case (1) check that a same-area packet comes
  from the receiving interface's network (`ospf-4-component-config.md`).
  <!-- source: internal/plugins/ospf/transport/backend_linux.go -- receiveIPv4, deliverDatagram -->
- **An active IPv4 interface needs an assigned unicast source address.**
  An absent, unspecified or multicast address refuses socket startup.
  <!-- source: internal/plugins/ospf/transport/backend_linux.go -- interfaceIPv4 -->
- **A signer can refuse transmission.** A nil result from the configured signer
  drops the packet before either the link-local or routed socket send. The caller
  receives a signing error and `ze_ospf_packets_dropped_total{reason="send-error"}`
  increments. With no authentication configured, the engine signer returns the
  original payload instead.
  <!-- source: internal/plugins/ospf/transport/transport.go -- SetSigner, SendPacket, SendPacketRouted -->
- **A panic in an interface's receive path ends that interface, never the
  process.** Each interface has two receive tasks: the backend's socket reader
  (`linuxInterface.readLoop`, task `interface-read`) and the transport's fan-in
  (`Transport.rxLoop`, task `interface-fan-in`). Each defers the shared
  `taskguard` boundary at its top, so a Ze defect a received datagram reaches is
  logged at ERROR, counted in `ze_task_panics_total{component="ospf",task}` and
  written as a crash record that `ze show crashes` lists. The interface then
  leaves OSPF through its ordinary link-down path: its neighbors go Down
  (RFC 2328 Section 9.3, InterfaceDown), and the 30-second rescan reopens it.
  The other interfaces keep receiving. A reader that ends for any other reason,
  such as a socket error, takes the same path (`receiveEnded`), so an open
  interface never sits with nobody reading its socket.
  <!-- source: internal/plugins/ospf/transport/panic_guard.go -- receivePanics, receiveEnded -->
  <!-- source: internal/plugins/ospf/transport/backend_linux.go -- readLoop -->
  <!-- source: internal/plugins/ospf/transport/transport.go -- rxLoop, linkDown -->

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
- **Multicast evidence crosses a peer namespace.** The veth round-trip test
  reads the production transmit socket's `IP_MULTICAST_IF` and observes the
  datagram at the peer. The AllDRouters test first receives on that link, then
  leaves the group and proves that subsequent group traffic is not delivered.
  The single-process fixture reloads the real interface backend on each
  namespace transition: its persistent netlink counter socket does not move
  when the calling thread changes namespace.
