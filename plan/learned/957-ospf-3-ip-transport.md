# 957 -- OSPF 3 IP Transport

## Context
OSPFv2 needed a raw IPv4 transport under `internal/plugins/ospf/transport/` before ISM, NSM, flooding, and SPF could run. The transport had to mirror Ze's IS-IS lifecycle shape while staying OSPF-specific: protocol 89, AllSPFRouters and AllDRouters multicast, link-local TTL, iface resolver integration, and a doctor check for `CAP_NET_RAW`. The prior codebase had RSVP-TE raw IPv4 unicast and IS-IS L2 transport, but no IP multicast raw-socket user.

## Decisions
- Chose Linux `AF_INET/SOCK_RAW` protocol 89 over `IP_HDRINCL` because RSVP-TE already proves the kernel-built IP header model and avoids manual IP checksum/header code.
- Chose per-interface RX and TX raw sockets over one socket because `IP_MULTICAST_LOOP=0` belongs on transmit while receive must still accept peer multicast on the same link.
- Chose iface resolver lookup over direct `net.InterfaceByName` because OSPF config names must remain Ze logical names, matching IS-IS and preserving `os-name` / `mac/match` selectors.
- Chose `IPMreq` bound to the interface IPv4 address over `IPMreqn` because QEMU raw multicast receive matched the address-bound membership path.
- Chose two-netns veth QEMU tests over same-netns veth because same-host local multicast loopback does not model two routers on one link.
- Chose transport-only packet type labels for metrics over packet validation because common-header dispatch and validation are owned by later runtime specs.

## Consequences
- Future OSPF runtime specs must call transport APIs for sends, receives, DR/BDR AllDRouters membership, and interface teardown; they must not open sockets directly.
- Unicast OSPF sends use the same TX socket as multicast, so the socket must set both `IP_TTL=1` and `IP_MULTICAST_TTL=1`.
- `ze_ospf_sockets_open` counts two sockets per open interface, not one, because RX and TX are deliberately split.
- Full FRR interop remains owned by ospf-13; ospf-3 proves only raw transport behavior with unit tests and QEMU veth multicast.
- OSPF transport metrics own receive-side malformed IPv4 drops before packet dispatch via `ze_ospf_packets_dropped_total{reason="malformed-ipv4"}`.

## Gotchas
- Same-netns veth can show packets in tcpdump while raw socket receive stays silent; use a peer netns for multicast transport tests.
- Disabling multicast loopback on a single socket can suppress the receive evidence you need; split TX and RX sockets before debugging protocol logic.
- Direct kernel name lookup silently breaks logical interface configs such as `uplink os-name eth0`; use `iface.Resolve` and `iface.Addresses` at the transport boundary.
- A doctor diagnostic code test is not enough; assert `diagnostic.DoctorCheckNames()` so removing `register.go` or the blank import fails.
- `IP_MULTICAST_TTL` does not affect unicast retransmissions or DD packets; set `IP_TTL` too.

## Files
- `internal/plugins/ospf/transport/`
- `internal/core/diagnostic/codes.go`
- `cmd/ze/ze_core_dispatch.go`
- `internal/test/cli/register.go`
- `mk/test-functional.mk`
- `Makefile`
- `scripts/evidence/qemu-all-tests.sh`
- `test/ospf/ospf-doctor-raw-socket.ci`
- `docs/architecture/wire/ospf.md`
- `docs/architecture/core-design.md`
- `docs/plugin-development/metrics.md`
- `docs/functional-tests.md`
- `plan/learned/957-ospf-3-ip-transport.md`
