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
engine can compare (ISO/IEC 10589 section 8.2.3). `inferNeighborMTU(frameSize)`
subtracts the two MAC addresses and the length field. A capture shorter than the
header returns a non-positive value, treated as unknown, so it cannot underflow.

<!-- source: internal/plugins/isis/transport/frame.go -- BuildFrame, ParseFrame, the 802.3 and LLC layout -->
<!-- source: internal/plugins/isis/transport/transport.go -- InterfaceMTU, inferNeighborMTU, observeNeighborFrame -->

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

## Decision: a panic in a circuit's receive path ends that circuit, never the process

Each circuit has two receive tasks: the backend's socket reader
(`linuxCircuit.readLoop`, task `interface-read`) and the transport's fan-in
(`Transport.rxLoop`, task `interface-fan-in`). Each defers the shared
`taskguard` boundary at its top, so the panic is logged at ERROR, counted in
`ze_task_panics_total{component="isis",task}` and written as a crash record
that `ze show crashes` lists. The circuit then leaves IS-IS through its ordinary
link-down path (the engine downs its adjacencies), and the 30-second rescan
reopens it; the other circuits keep receiving. A reader that ends for any other
reason takes the same path (`receiveEnded`), and `linkDown` closes only the
circuit state it was handed, never a replacement a link flap already opened.

The reader's guard is deferred before `close(recvCh)`, so it runs last and also
contains a panic in that close: the whole reader, its exit included, sits
inside the boundary. The fan-in may therefore log the link-down before the guard
logs the panic.
<!-- source: internal/plugins/isis/transport/panic_guard.go -- receivePanics, receiveEnded -->
<!-- source: internal/plugins/isis/transport/backend_linux.go -- readLoop -->
<!-- source: internal/plugins/isis/transport/transport.go -- rxLoop, linkDown -->

## Decision: the doctor check is split by platform and self-contained

`doctor.go` holds the platform-neutral check body, so it is testable on any OS.
`doctor_linux.go` and `doctor_other.go` hold the raw-socket probe,
`checkRawSocket`, which returns the reason the socket is unavailable, and the
check prints that reason as the warning. On Linux the reason wraps the socket
error: a permission refusal (`EPERM`, `EACCES`) adds the `CAP_NET_RAW` hint, and
any other error is reported as itself. On any other host the probe always
refuses, naming the host's OS, because the transport is Linux-only.
`register.go` registers the check from `init()`. The whole check lives under `transport/`, and
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

`./le test qemu all` uses `integrationPackages` in
`internal/le/test/qemu/alltests.go`. Add every new integration-tagged package there.
