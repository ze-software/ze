# IS-IS Layer-2 Transport

`internal/plugins/isis/transport` is the byte pipe between a raw socket and the
IS-IS engine. It sends final engine PDUs to the ISO multicast groups, delivers
received PDUs up as `(ifindex, PDU)`, drives circuit lifecycle from the interface
event bus, and owns the `CAP_NET_RAW` readiness check.

It generalizes the PPPoE `AF_PACKET` / `SOCK_RAW` pattern from a single Ethernet
II ethertype to IEEE 802.3 length plus LLC, which is how IS-IS runs on the wire.

## Decision: the transport is a pure byte pipe

`frame.go` adds **only** the 802.3 length and the LLC header (`0xFE`, `0xFE`,
`0x03`). It never pads, inspects, or alters the PDU.

Padding (TLV 8) is owned by the engine and is added **before** authentication
signing, so the signed, padded hello bytes reach the transport untouched
(RFC 5304). Keeping the act of padding out of the transport is the core insight
of this layer.

The transport's only MTU role is to **expose** the ioctl MTU, so the engine can
size padding, and to **infer** a neighbor MTU from a received frame size, so the
engine can compare (ISO/IEC 10589 section 8.2.3). `InferNeighborMTU(frameSize)`
subtracts the two MAC addresses and the length field. A capture shorter than the
header returns a non-positive value, treated as unknown, so it cannot underflow.

<!-- source: internal/plugins/isis/transport/frame.go -- BuildFrame, ParseFrame, the 802.3 and LLC layout -->
<!-- source: internal/plugins/isis/transport/transport.go -- InterfaceMTU, InferNeighborMTU, ObserveNeighborFrame -->

## Decision: one socket per circuit, with explicit multicast membership

Each circuit binds its own `AF_PACKET` / `SOCK_RAW` socket to its ifindex, rather
than sharing one discovery socket as PPPoE does. IS-IS 802.3 frames carry no
ethertype to bind or filter on, so the socket binds `ETH_P_ALL`, the ISO
multicast groups are joined explicitly with `PACKET_ADD_MEMBERSHIP`, and a
receive-path filter accepts only those destination MACs. Promiscuous mode is
avoided.

Binding the socket alone does **not** deliver multicast. The membership call is
required.

<!-- source: internal/plugins/isis/transport/backend_linux.go -- the AF_PACKET backend, PACKET_ADD_MEMBERSHIP -->
<!-- source: internal/plugins/isis/transport/multicast.go -- MulticastMACForLevel, IsISMulticastMAC -->

## Decision: a backend interface with a non-Linux stub

`Backend` and `CircuitHandle` are Go interfaces so a BSD or VPP backend can drop
in. The non-Linux build ships a stub whose circuit open fails cleanly, so the
component still loads for config parsing and the platform-neutral unit tests run
on any OS.

## Decision: the event-bus handler enqueues, a worker opens the socket

The interface up and down handler must not block on I/O, so it only enqueues the
interface name on a bounded queue; a worker goroutine opens and closes the
socket. A periodic rescan re-opens any enabled-but-closed circuit, so an
interface-up event dropped by a full queue during a flap burst self-heals with no
operator action.

## Decision: the doctor check is split by platform and self-contained

`doctor.go` holds the platform-neutral check body, so it is testable on any OS.
`doctor_linux.go` and `doctor_other.go` hold the raw-socket probe. `register.go`
registers the check from `init()`. The whole check lives under `transport/`, and
it owns the `doctor-isis-raw-socket` diagnostic code. One code, one owner: the
IS-IS component does not re-register it.

<!-- source: internal/plugins/isis/transport/doctor.go -- the raw-socket readiness check -->
<!-- source: internal/plugins/isis/transport/register.go -- doctor-check registration -->

## Trap: the field after the source MAC is a length, not an ethertype

Any value at or above `0x0600` is an Ethernet II ethertype and is rejected. This
is the classic IS-IS framing bug. `BuildFrame` writes a length and refuses an LLC
plus PDU that would reach `0x0600`; `ParseFrame` rejects an ethertype-valued
field before it slices into the PDU. A frame a peer cannot parse cannot be
emitted by construction.

## Trap: a reused send buffer needs a send lock, not the orchestrator lock

The engine fans hello, flood, and DIS or SNP sends **concurrently** onto the same
circuit. The transport orchestrator releases its own lock before calling into the
circuit handle, so the orchestrator lock does not serialize sends. The Linux
circuit writes a reused send buffer and then sends it, so two goroutines could
interleave the frame build and the send and transmit a torn frame.

The fix holds the circuit's send lock **across** the frame build and the send.
Guard this on every platform: the darwin fake circuit reproduces the hazard so
the race detector flags it with no raw socket, and a Linux veth test proves it on
the wire. The pattern applies to any future raw backend with a reused send
buffer.

## Owned metrics

`ze_isis_frames_sent_total{interface}`,
`ze_isis_frames_received_total{interface}`,
`ze_isis_frames_dropped_total{interface,reason}`, and `ze_isis_sockets_open` are
registered here, not by the CLI layer.

<!-- source: internal/plugins/isis/transport/metrics.go -- the four transport series -->

## Trap: QEMU evidence uses an explicit package list

`./le qemu all-tests` uses `integrationPackages` in
`internal/le/qemu/alltests.go`. Add every new integration-tagged package there.
