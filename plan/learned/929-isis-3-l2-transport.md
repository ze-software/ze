# 929 -- isis-3-l2-transport

## Context
Spec `isis-3-l2-transport` adds the single genuinely-new low-level capability in the
IS-IS spec set: a raw Layer-2 transport (`internal/component/isis/transport/`) that is
the byte pipe between a raw socket and the IS-IS engine. It is modelled on the proven
PPPoE `AF_PACKET`/`SOCK_RAW` pattern but generalises the framing from a single Ethernet
II ethertype to IEEE 802.3 length + LLC, which is how IS-IS actually runs on the wire
(ISO/IEC 10589). The transport sends final engine PDUs to ISO multicast groups, delivers
received PDUs up as `(ifindex, pdu)`, drives circuit lifecycle from the iface EventBus,
and surfaces a `CAP_NET_RAW` doctor check. The implementation is DONE: the whole tree
builds (darwin + linux `go vet`), all transport unit tests pass under `-race`
(`ok ... 1.426s`), and golangci-lint is clean across the isis tree. What remains is
interop validation pending Linux execution (the QEMU veth tests and FRR interop need a
Linux/QEMU host; the darwin dev host cannot run them).

## Decisions
- The transport is a PURE byte pipe: `frame.go` adds ONLY 802.3 length + LLC
  (0xFE/0xFE/0x03) and NEVER pads, inspects, or alters the PDU. Padding (TLV 8) is
  owned by the engine (isis-5) and added before authentication signing, so the signed,
  padded Hello bytes reach the transport untouched (RFC 5304). The transport's only MTU
  role is to EXPOSE the ioctl MTU (so the engine can size padding) and to INFER the
  neighbour MTU from a received frame size (so the engine can compare; ISO/IEC 10589 sec
  8.2.3). Keeping the act of padding out of the transport is the core insight.
- One `AF_PACKET`/`SOCK_RAW` socket PER circuit, bound to the ifindex, rather than the
  single shared discovery socket PPPoE uses. IS-IS 802.3 frames have no ethertype to
  bind/filter on, so the socket binds `ETH_P_ALL` and the ISO multicast groups are
  joined explicitly with `PACKET_ADD_MEMBERSHIP` (AllL1ISs / AllL2ISs / AllISs); a
  receive-path `IsISMulticastMAC` filter accepts only those destinations. Promiscuous
  mode is avoided.
- The backend sits behind a `Backend` / `CircuitHandle` Go interface so a future BSD or
  VPP backend can drop in; v1 ships the Linux backend plus a non-Linux stub
  (`NewBackend()` whose `OpenCircuit` fails cleanly) so the component still loads for
  config parsing and the platform-neutral unit tests on any OS.
- Circuit lifecycle is driven from the iface EventBus (`interface/up`/`interface/down`).
  The EventBus handler must not block on I/O, so it only enqueues the interface name on
  a BOUNDED worker queue; a worker goroutine performs the socket open/close. A periodic
  `RescanInterfaces` backstop re-opens any enabled-but-closed circuit, so a dropped
  `interface/up` (queue overflow under a flap burst) self-heals without operator action.
- The doctor check is split: `doctor.go` holds the platform-neutral check body (testable
  on any OS), `doctor_linux.go` / `doctor_other.go` hold the platform raw-socket probe,
  and `register.go` registers it from `init()` via `diagnostic.RegisterDoctorCheck`
  (component-style path; the single IS-IS plugin Registration is owned by isis-4). The
  whole check is self-contained under `transport/` (plugin-self-containment).

## Consequences
- IS-IS frames are structurally protected from the classic framing bug: BuildFrame
  writes a LENGTH and rejects an LLC+PDU that would reach `0x0600`; ParseFrame rejects an
  ethertype-valued field before any slice into the PDU. A frame FRR cannot parse cannot
  be emitted by construction.
- Owned metrics (this spec is the registration owner, not isis-13):
  `ze_isis_frames_sent_total{interface}`, `ze_isis_frames_received_total{interface}`,
  `ze_isis_frames_dropped_total{interface,reason}`, `ze_isis_sockets_open`.
- Owned diagnostic code: `doctor-isis-raw-socket` (in `internal/core/diagnostic/codes.go`),
  surfaced via `ze explain` / `ze doctor --json`.

## Gotchas
- **B3 transport race (found in adversarial review):** the engine fans Hello, flood, and
  DIS/SNP sends CONCURRENTLY onto the SAME circuit. The transport orchestrator releases
  its own `t.mu` BEFORE calling `CircuitHandle.Send`, so relying on the orchestrator lock
  is not enough. `linuxCircuit.Send` writes a reused `sendBuf` then `Sendto`s it; two
  goroutines could interleave BuildFrame+Sendto and transmit a torn frame. Fix: hold
  `linuxCircuit.sendMu` ACROSS BuildFrame+Sendto. Guard it on every platform: the darwin
  fake `sharedBufCircuit` reproduces the hazard so `go test -race` flags it without a raw
  socket (`TestISISTransportConcurrentSendSerialised`), and the Linux veth integration
  test `TestISISTransportConcurrentSendNoTear` proves it on the wire. Pattern reusable
  for any future raw backend with a reused send buffer.
- **Raw multicast receive needs `PACKET_ADD_MEMBERSHIP`** -- binding the socket alone
  does NOT deliver multicast. This resolves spec assumption A-2 / risk R-2 by design;
  the on-the-wire confirmation is the veth QEMU test (interop validation pending Linux
  execution).
- **The 2-byte field after the source MAC is an 802.3 LENGTH, not an ethertype.** Any
  value `>= 0x0600` is an Ethernet II ethertype and is rejected. This is THE IS-IS
  framing trap; the codec makes it explicit on both build and parse.
- **MTU inference math:** `InferNeighborMTU(frameSize)` = `frameSize - (2*MACLen +
  LengthFieldLen)` = the LLC+PDU L2 payload the neighbour padded to fill its link. A
  capture shorter than the header returns a non-positive value (treated as "unknown"),
  avoiding underflow.
- **QEMU evidence script does NOT derive packages from build tags** (unlike the
  `ze-qemu-integration-test` target, which derives `ZE_QEMU_INTEGRATION_PKGS` from a
  `grep` over `//go:build integration && linux`). `scripts/evidence/qemu-all-tests.sh`
  hardcodes its package list, so `./internal/component/isis/transport/...` had to be
  added there explicitly (lines 157-158) or the integration tests would be silently
  skipped in the all-tests evidence run.
- **FRR interop over this transport is owned by isis-13** (needs the isis-5 adjacency
  FSM); the `test/interop/scenarios/isis-*-frr` scenarios exist but are isis-13's, not
  this spec's ACs. isis-3 proves framing by byte-exact unit tests and the QEMU veth
  send/receive integration test.

## Interop validation pending Linux execution
The three QEMU integration tests in `transport_integration_linux_test.go`
(`TestISISTransportVethRoundTrip`, `TestISISTransportMTUExpose`,
`TestISISTransportRawSocketCap`, plus `TestISISTransportConcurrentSendNoTear`) are
WRITTEN and tagged `integration && linux`; they were NOT executed in this session
because the host is darwin (no AF_PACKET, no veth, no CAP_NET_RAW). They validate
assumptions A-1 (PPPoE pattern generalises to 802.3+LLC), A-2 (multicast receive via
`PACKET_ADD_MEMBERSHIP`, no promiscuous mode), A-4 (ioctl MTU exposed; smaller peer
frame infers a neighbour MTU), and A-5 (raw open under CAP_NET_RAW). The unit/build
evidence is real and passing on darwin; the on-the-wire confirmation is interop
validation pending Linux execution (`make ze-qemu-integration-test` /
`scripts/evidence/qemu-all-tests.sh` on a Linux/QEMU host).

## Files
- `internal/plugins/isis/transport/transport.go` (+`transport_test.go`): `Backend` /
  `CircuitHandle` interfaces, `Transport` orchestrator (circuit registry, EventBus
  subscription + bounded queue + rescan, `SendPDU`/`SendPDUBothLevels`, `Receive`,
  `InterfaceMTU`/`CircuitInfo`/`CircuitNameByIfIndex`, `ObserveNeighborFrame`/
  `InferNeighborMTU`/`OnMTUMismatch`, `Close`).
- `internal/plugins/isis/transport/frame.go` (+test): buffer-first 802.3+LLC build,
  zero-copy parse, ethertype/SAP/length/short rejection.
- `internal/plugins/isis/transport/multicast.go` (+test): `Level`, ISO MAC constants,
  `MulticastMACForLevel`, `IsISMulticastMAC`.
- `internal/plugins/isis/transport/backend_linux.go` (+test): AF_PACKET per-circuit
  backend, `PACKET_ADD_MEMBERSHIP`, `SO_RCVTIMEO`, ioctl resolve, serialized `Send`.
- `internal/plugins/isis/transport/backend_other.go` (+test): non-Linux stub.
- `internal/plugins/isis/transport/doctor.go` / `doctor_linux.go` / `doctor_other.go`
  / `register.go` (+`doctor_test.go`): `doctor-isis-raw-socket` check + registration.
- `internal/plugins/isis/transport/metrics.go` (+test): the four ze_isis_* series.
- `internal/plugins/isis/transport/transport_integration_linux_test.go`: the QEMU
  veth tests (scenario written; execution pending Linux/QEMU).
- `internal/core/diagnostic/codes.go`: `doctor-isis-raw-socket` code (title/description).
- `scripts/evidence/qemu-all-tests.sh`: explicit transport-package add (lines 157-158).
- `test/isis/isis-doctor-raw-socket.ci`: user-visible `ze explain` surface.
- Doc note: `docs/architecture/wire/isis.md` references the transport but lacks a
  dedicated 802.3+LLC framing section (frame layout / 0xFE LLC SAP / multicast MAC
  enumeration); the wire-format facts live in the `frame.go`/`multicast.go` source
  headers. A canonical wire-doc framing section and a `core-design.md` transport-layer
  mention are follow-ups (recorded in the spec's Documentation Verified table).
