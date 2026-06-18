# Spec: isis-3-l2-transport

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-1-types.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - umbrella scope (this is row isis-3)
4. `internal/component/pppoe/kernel_linux.go` - proven AF_PACKET/SOCK_RAW socket pattern this transport is modelled on
5. `internal/component/pppoe/discovery.go` - proven Ethernet frame build/parse + constants pattern
6. `internal/plugins/iface/netlink/monitor_linux.go`, `internal/component/iface/events/events.go` - interface up/down subscription
7. `docs/research/isis-implementation-guide.md` sec 6 (circuit types, multicast MACs) and sec 12 item 6 (MTU mismatch; note padding is added by the engine, isis-5, not by this transport)

## Task

Implement a raw Layer-2 transport for IS-IS in `internal/component/isis/transport/`,
modelled on the proven PPPoE `AF_PACKET`/`SOCK_RAW` socket pattern. This is the
single genuinely-new low-level capability in the IS-IS spec set (umbrella sec
"Existing Foundation"). It provides the byte pipe that spec-isis-5 (adjacency)
and spec-isis-7 (flooding) consume to send and receive IS-IS PDUs over Ethernet.

IS-IS does NOT run over IP or over an Ethernet II ethertype. It runs directly
over IEEE 802.3 frames with an LLC header. Each transmitted frame is: destination
MAC (6) + source MAC (6) + 2-byte 802.3 length field (the length of LLC+PDU, NOT
an ethertype) + LLC header (DSAP 0xFE, SSAP 0xFE, control 0x03) + IS-IS PDU. The
802.3 length distinguishes these frames from Ethernet II (where the same 2-byte
field is an ethertype >= 0x0600); assuming an ethertype is the most common
implementation error and this spec makes the framing explicit.

The transport sends to ISO multicast destination MACs on BOTH broadcast and
point-to-point (P2P) circuits (AllL1ISs 01:80:c2:00:00:14, AllL2ISs
01:80:c2:00:00:15, AllISs 09:00:2b:00:00:05), selecting the level-appropriate
group. P2P circuits ALSO use the level multicast group; no neighbour unicast MAC
is learned or required before the first Hello, so there is no first-Hello
MAC-discovery problem. The receive path must accept frames addressed to these
multicast groups.

The backend sits behind a Go interface (open, close, send, receive per
interface) so a future BSD or VPP backend can drop in (FRR isolates three
raw-socket backends). v1 ships only the Linux `AF_PACKET` backend. Per-interface
RX and TX goroutines drive each circuit; received frames are dispatched by source
ifindex from the link-layer address (same shared-socket + `SockaddrLinklayer`
model as PPPoE). The transport subscribes to the iface EventBus and opens a
circuit on link up, closes it and signals teardown on link down.

This transport does NOT pad PDUs. Per umbrella Shared Contracts "Final PDU
bytes: padding then authentication (owner: engine, NOT transport)", the Padding
TLV (8) is part of the PDU and is added by the engine (isis-5) during PDU
construction, BEFORE authentication signing, so the signed digest covers the
padding (RFC 5304 signs padded Hellos). The transport receives the final,
already-padded, already-signed PDU bytes and adds only the 802.3 + LLC framing;
it MUST NOT pad or otherwise alter the PDU bytes. To let the engine size the
padding, the transport EXPOSES the interface MTU (resolved by ioctl). MTU-mismatch
detection stays a transport observation: the received padded-Hello frame size /
padding lets the transport surface an inferred neighbour MTU to the engine for
comparison (research guide sec 12 item 6, ISO/IEC 10589 sec 8.2.3). The act of
padding belongs to the engine.

The raw socket requires `CAP_NET_RAW`. A registered doctor check (per
`ai/rules/doctor-checks.md`) verifies the capability / raw-socket open before
the daemon starts.

## Required Reading

### Architecture Docs
- [ ] `internal/component/pppoe/kernel_linux.go` - `openDiscoverySocket`, `readDiscoveryFrame`, `sendDiscoveryFrame`, `resolveInterface` (lines 26-126); the AF_PACKET model
  -> Constraint: shared `AF_PACKET`/`SOCK_RAW` socket, dispatch by `SockaddrLinklayer.Ifindex` from `Recvfrom`; send via `Sendto` with `SockaddrLinklayer{Ifindex, Halen, Addr}`
  -> Constraint: `htons` byte-order helper and `SO_RCVTIMEO` for periodic stop-signal checks both reusable in shape
- [ ] `internal/component/pppoe/discovery.go` - frame constants + zero-copy parse (lines 13-21, 95-143)
  -> Constraint: parse returns views into the receive buffer (zero-copy); validate minimum length and reject before slicing; cap loop iterations against crafted floods
  -> Decision: PPPoE binds a single `Protocol` ethertype; IS-IS 802.3 frames are NOT an ethertype, so the bind/socket protocol selection differs (see Current Behavior)
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` (lines 72-87) + `internal/component/iface/events/events.go`
  -> Constraint: subscribe to iface events; `events.Namespace = "interface"`, `EventUp`/`EventDown` drive circuit open/close and adjacency teardown
- [ ] `docs/research/isis-implementation-guide.md` sec 6 "Circuit Types and Network Model" (lines 456-499)
  -> Constraint: broadcast uses multicast MACs and LAN IIH; P2P ALSO sends to the level multicast MAC (no neighbour unicast MAC learned or required before the first Hello) and uses P2P IIH; circuits bind to interfaces and react to up/down
- [ ] `docs/research/isis-implementation-guide.md` sec 12 item 6 "Padded Hellos and MTU Mismatch" (lines 878-885)
  -> Constraint: padding is added by the engine (isis-5) during PDU build, before auth; this transport does NOT pad. The transport exposes the interface MTU so the engine can size the padding, and infers/surfaces the neighbour MTU from received frame size for comparison
- [ ] `ai/rules/doctor-checks.md`, `ai/rules/qemu-testing.md`
  -> Constraint: owning-package doctor check + `doctor-*` code in `internal/core/diagnostic/codes.go` + unit + functional test; linux-only code MUST ship a QEMU integration test (hardware-only is not a skip reason)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`
  -> Constraint: frame build writes into a caller-supplied buffer (buffer-first); parse is zero-copy views; no per-frame allocation on the hot path

### RFC Summaries (MUST for protocol work)
- [ ] `iso/short/iso10589.md` - IS-IS base (created in isis-2 / isis-5)
  -> Constraint: 802.3 + LLC SAP 0xFE framing; ISO multicast MAC assignments; sec 8.2.3 padded Hello MTU detection

**Key insights:** (minimal context to resume after compaction)
- IS-IS frame is 802.3 length + LLC (DSAP 0xFE / SSAP 0xFE / control 0x03) + PDU, NOT an ethertype. Assuming an ethertype is the classic error.
- Multicast dest MACs: AllL1ISs 01:80:c2:00:00:14, AllL2ISs 01:80:c2:00:00:15, AllISs 09:00:2b:00:00:05; select by level on BOTH broadcast and P2P circuits. P2P also sends to the level multicast group (NOT a neighbour unicast MAC) and learns no neighbour MAC before the first Hello (matches the umbrella "Frame addressing" contract and the send/AC rows below).
- Pattern is proven in PPPoE: shared AF_PACKET/SOCK_RAW socket, `SockaddrLinklayer` ifindex dispatch, `Sendto` per interface.
- Raw multicast receive may need `PACKET_ADD_MEMBERSHIP` and/or promiscuous mode; validate on veth (mirrors umbrella assumption A-4).
- `CAP_NET_RAW` required; doctor check guards startup.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/pppoe/kernel_linux.go` - the only raw `AF_PACKET`/`SOCK_RAW` user in Ze; binds a single Ethernet II ethertype (`ETH_P_PPP_DISC` 0x8863), dispatches by `SockaddrLinklayer.Ifindex`
  -> Constraint: reuse the socket / ifindex-dispatch shape; do NOT reuse the ethertype bind verbatim (IS-IS is 802.3, not ethertype)
- [ ] `internal/component/pppoe/discovery.go` - Ethernet II frame parse with ethertype validation, zero-copy tag views
  -> Constraint: IS-IS replaces ethertype validation with 802.3-length + LLC-SAP validation
- [ ] `internal/component/iface/events/events.go` - interface event namespace and `EventUp`/`EventDown` constants
  -> Constraint: drive circuit lifecycle from these events, not from a transport-private poll loop
- [ ] Ze has no ISO/CLNS path and no 802.3/LLC framing anywhere; IS-IS builds its own
  -> Constraint: this transport is the first 802.3/LLC user; nothing existing to extend

**Behavior to preserve:**
- PPPoE raw-socket code unchanged: IS-IS adds a sibling transport, it does not refactor PPPoE in place.
- iface EventBus semantics unchanged: IS-IS is a new subscriber only.
- Existing doctor checks and `internal/core/diagnostic/codes.go` entries unchanged; a new `doctor-isis-raw-socket` code is appended.

**Behavior to change:**
- New package `internal/component/isis/transport/` with a backend interface and a Linux `AF_PACKET` backend.
- New `doctor-isis-raw-socket` diagnostic code and a registered doctor check.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound: raw 802.3 frames arrive on enabled interfaces via a raw `AF_PACKET`/`SOCK_RAW` socket; bytes plus source ifindex come from `Recvfrom` / `SockaddrLinklayer`.
- Outbound: the IS-IS engine (isis-5/isis-7) hands a FINAL, already-padded, already-signed PDU plus a circuit reference and a level selector (the destination selector is the level multicast group, NOT a neighbour MAC); the transport frames and sends it without altering the PDU bytes.
- Lifecycle: iface EventBus `interface/up` and `interface/down` events open and close circuits.

### Transformation Path
1. **Open:** on `interface/up` for a configured circuit, resolve ifindex/hwaddr/MTU (ioctl as in PPPoE `resolveInterface`), open / join the raw socket for that interface, start RX and TX goroutines.
2. **Receive:** `Recvfrom` -> frame bytes + source ifindex -> validate 802.3 length and LLC header (DSAP/SSAP 0xFE, control 0x03) -> strip header -> deliver `(ifindex, pdu []byte)` to the engine (PDU bytes are a zero-copy view; parse happens in isis-2). The transport hands raw PDUs up; it does NOT switch on PDU type. The PDU-type dispatcher (keyed by the 5-bit PDU type) is owned by isis-4 (`server.go`) per umbrella Shared Contracts "PDU receive dispatcher", not by this transport.
3. **Send:** final engine PDU (already padded and signed) + circuit + destination selector -> build frame (dest MAC + src MAC + 802.3 length + LLC + PDU) -> `Sendto` with `SockaddrLinklayer{Ifindex, Halen, Addr}`. The transport does NOT pad and does NOT alter the PDU bytes.
4. **Multicast select:** both broadcast and P2P circuits choose the level multicast group AllL1ISs / AllL2ISs (AllISs accepted on receive) by level; P2P does NOT use a neighbour unicast MAC and learns none before the first Hello.
5. **MTU:** the transport exposes the interface MTU (ioctl) so the engine can size the Padding TLV; it does NOT pad. On receive it records the observed frame size / padding and surfaces an inferred neighbour MTU so the engine can compare.
6. **Close:** on `interface/down`, stop RX/TX goroutines, close / leave the socket, signal the engine to tear down adjacencies on that circuit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> transport | raw `AF_PACKET` 802.3 frames, ifindex from `SockaddrLinklayer` | [ ] |
| transport <-> IS-IS engine | backend interface: open/close per circuit, send PDU, receive `(ifindex, pdu)` channel/callback | [ ] |
| iface EventBus <-> transport | subscribe `interface/up` and `interface/down`; open/close circuits | [ ] |
| transport <-> doctor | `doctor-isis-raw-socket` check probes raw-socket open / `CAP_NET_RAW` | [ ] |

### Integration Points
- New package `internal/component/isis/transport/` (interface, Linux backend, frame codec, multicast constants).
- Consumes `internal/component/isis/types` (spec-isis-1) for SystemID / level types where the destination selector needs them.
- Subscribes to `internal/component/iface/events` for link up/down.
- Registers a doctor check (`internal/core/diagnostic/codes.go` + owning-package registration).
- Provides the send/receive primitive that spec-isis-5 (adjacency) and spec-isis-7 (flooding) build on.

### Architectural Verification
- [ ] No bypassed layers (frames flow socket -> transport codec -> engine; engine never touches the socket directly)
- [ ] No unintended coupling (backend behind an interface; Linux specifics in a `_linux.go` file; PPPoE untouched)
- [ ] No duplicated functionality (one transport; engine reuses it for IIH/LSP/CSNP/PSNP rather than opening its own socket)
- [ ] Zero-copy preserved (received PDU bytes are views into the receive buffer; frame build is buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PPPoE's `AF_PACKET`/`SOCK_RAW` shared-socket + ifindex-dispatch pattern generalises to 802.3+LLC IS-IS frames | `internal/component/pppoe/kernel_linux.go:26-90` | transport needs a different socket mechanism | functional/QEMU send+recv of an IIH-shaped frame on a veth pair | unvalidated |
| A-2 | Raw multicast receive of ISO MACs works without extra socket options on a Linux veth/bridge | research guide sec 6; PPPoE uses broadcast not multicast so this is unverified | need `PACKET_ADD_MEMBERSHIP` and/or promiscuous mode | QEMU two-veth functional test receiving on the peer | unvalidated (mirrors umbrella A-4) |
| A-3 | The 802.3 length field (length of LLC+PDU) is the correct framing and the kernel does not overwrite it for `SOCK_RAW` | ISO/IEC 10589; research guide sec 6 | framing wrong, FRR cannot parse Ze frames | byte-level unit test on built frame + interop in isis-13 | unvalidated |
| A-4 | Interface MTU from ioctl (as PPPoE `resolveInterface`) is the value the transport exposes to the engine for sizing the Padding TLV (engine pads, not transport) | research guide sec 12 item 6 | wrong MTU exposed, engine mis-pads, false mismatch | boundary unit test + QEMU MTU-mismatch test | unvalidated |
| A-5 | `CAP_NET_RAW` is the only privilege needed to open the socket on the gokrazy appliance | PPPoE raw socket precedent | socket open EPERM at startup | doctor check + QEMU open probe | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | 802.3 framing implemented as an ethertype by mistake | FRR / tcpdump shows frame parsed as Ethernet II or rejected | explicit byte-level frame unit test with the 802.3 length + LLC bytes asserted; interop in isis-13 |
| R-2 | Multicast receive silently drops frames (membership/promisc not set) | RX goroutine never delivers a frame the peer sent | A-2 validation on veth; add `PACKET_ADD_MEMBERSHIP` if needed; record outcome in the spec |
| R-3 | Per-interface goroutine leak on rapid link flap | goroutine count climbs under flap | bounded lifecycle tied to `interface/down`; `SO_RCVTIMEO` so RX wakes to observe stop |
| R-4 | `CAP_NET_RAW` absent at runtime | socket open EPERM at startup | doctor check `doctor-isis-raw-socket` with a clear message before the daemon runs |
| R-5 | Wrong MTU exposed to the engine (or oversized final PDU) causes a false MTU-mismatch warning or send EMSGSIZE | spurious mismatch logs or send EMSGSIZE | expose the exact ioctl MTU; reject sending a PDU larger than MTU; boundary tests for min/max MTU |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `interface/up` event for a configured circuit | -> | transport opens raw socket, starts RX/TX | `TestISISTransportOpenOnLinkUp` |
| engine sends a final IIH-shaped PDU on a circuit | -> | frame built (802.3+LLC, no pad) and sent via `Sendto` | `TestISISTransportSendFrame` |
| frame arrives on the peer interface | -> | RX delivers `(ifindex, pdu)` to the engine | `TestISISTransportVethRoundTrip` (`transport_integration_linux_test.go`, QEMU) |
| `interface/down` event | -> | transport closes socket, signals circuit teardown | `TestISISTransportCloseOnLinkDown` |
| `ze doctor` with IS-IS configured | -> | `doctor-isis-raw-socket` check runs | `TestISISDoctorRawSocket` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two interfaces of a veth pair, transport opened on both | A frame sent to the level multicast MAC on one is received on the other with source ifindex matching the sender |
| AC-2 | Build a frame for a PDU | Bytes are dest MAC + src MAC + 2-byte 802.3 length (= LLC+PDU length, < 0x0600) + LLC (0xFE 0xFE 0x03) + PDU; no ethertype present |
| AC-3 | Query a circuit with MTU 1500 | Transport exposes MTU 1500 to the engine (so the engine can size the Padding TLV); the transport itself does NOT pad. A final PDU handed to the transport is sent byte-for-byte unaltered |
| AC-4 | Peer sends a padded Hello smaller than local MTU | Transport infers the neighbour MTU from the received frame size and surfaces it; mismatch detected at the engine (warning/event), per ISO/IEC 10589 sec 8.2.3 |
| AC-5 | Broadcast or P2P circuit at L1 vs L2 vs both | Destination MAC is AllL1ISs / AllL2ISs / (both groups) selected by level on BOTH broadcast and P2P circuits; a P2P circuit also sends to the level multicast group and uses no neighbour unicast MAC (none learned before the first Hello) |
| AC-6 | `interface/down` event on an open circuit | RX/TX goroutines stop, socket closed, circuit-teardown signal emitted (no goroutine leak) |
| AC-7 | Raw socket open without `CAP_NET_RAW` | Open fails with a clear error; `doctor-isis-raw-socket` reports the missing capability |
| AC-8 | Received frame fails 802.3-length or LLC-SAP validation | Frame is rejected before slicing into the PDU; no panic; counter incremented |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up an IS-IS-enabled interface | `interface/up` -> transport open -> RX/TX goroutines running | `TestISISTransportOpenOnLinkUp`, `TestISISTransportVethRoundTrip` (QEMU integration) |
| 2 | Has IS-IS send a Hello to the LAN | final engine PDU -> frame build (802.3+LLC, multicast MAC, no pad) -> `Sendto` -> peer RX | `TestISISTransportSendFrame`, `TestISISTransportVethRoundTrip` (QEMU integration) |
| 3 | Connects to a neighbour with a smaller MTU | padded Hello received (engine padded it) -> transport infers neighbour MTU -> mismatch surfaced | QEMU MTU-mismatch integration test (`transport_integration_linux_test.go`) |
| 4 | Runs `ze doctor` before starting IS-IS | doctor -> `doctor-isis-raw-socket` check -> raw-socket open / capability result | `TestISISDoctorRawSocket`, `ze doctor --json` functional test |
| 5 | Takes the interface down | `interface/down` -> circuit close -> engine tears down adjacencies | `TestISISTransportCloseOnLinkDown` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildFrame` | `internal/component/isis/transport/frame_test.go` | dest/src MAC + 802.3 length + LLC (0xFE/0xFE/0x03) + PDU layout, byte-exact | |
| `TestParseFrame` | `internal/component/isis/transport/frame_test.go` | strips 802.3+LLC, returns zero-copy PDU view, rejects bad length / bad SAP / short frame | |
| `TestParseFrameRejectEthertype` | `internal/component/isis/transport/frame_test.go` | a frame whose 802.3 field is >= 0x0600 (an ethertype) is rejected, not parsed | |
| `TestMulticastMACForLevel` | `internal/component/isis/transport/multicast_test.go` | level L1 -> AllL1ISs, L2 -> AllL2ISs, AllISs constant; bytes match | |
| `TestSendDoesNotAlterPDU` | `internal/component/isis/transport/frame_test.go` | the transport adds only 802.3+LLC and sends the final PDU byte-for-byte; it does NOT pad; a PDU larger than MTU is rejected | |
| `TestExposeInterfaceMTU` | `internal/component/isis/transport/mtu_test.go` | the transport exposes the ioctl interface MTU so the engine can size the Padding TLV | |
| `TestInferNeighbourMTU` | `internal/component/isis/transport/mtu_test.go` | observed received frame size maps to an inferred neighbour MTU; mismatch surfaced | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| 802.3 length field | 3..1500 (LLC 3 + PDU 0..1497) | 1500 | < 3 (no room for LLC) | >= 0x0600 (would be an ethertype) |
| Interface MTU exposed to engine | 512..9000 | 9000 | < 512 (too small for IS-IS) | > 9000 (jumbo cap) |
| Frame total length on RX | 17..MTU+18 | MTU+18 | < 17 (MAC*2 + len + LLC) | > MTU+18 |
| LLC DSAP/SSAP | 0xFE only | 0xFE | n/a | any other value rejected |
| LLC control | 0x03 only | 0x03 | n/a | any other value rejected |

### Functional Tests
`.ci` files cover ONLY user-visible config/doctor output. The raw-L2 / AF_PACKET
/ veth send-receive path is kernel-capability code and is proven by a Linux-only
QEMU integration test (see QEMU Integration Tests below), NOT by a `.ci` file
(`ai/rules/qemu-testing.md`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-doctor-raw-socket` | `test/isis/isis-doctor-raw-socket.ci` | `ze doctor --json` exposes `doctor-isis-raw-socket` result | |

### QEMU Integration Tests (MANDATORY for linux-only code -- `ai/rules/qemu-testing.md`)
| Test | File | Build tags | End-User Scenario | Status |
|------|------|------------|-------------------|--------|
| `TestISISTransportVethRoundTrip` | `internal/component/isis/transport/transport_integration_linux_test.go` | `integration && linux` | open transport on a veth pair in a netns, send a frame, receive it on the peer with correct ifindex (validates A-1, A-2 multicast membership/promisc) | |
| `TestISISTransportMTUExpose` | `internal/component/isis/transport/transport_integration_linux_test.go` | `integration && linux` | resolve a veth's ioctl MTU and confirm the transport exposes it; a smaller peer padded-Hello surfaces an inferred neighbour MTU (validates A-4) | |
| `TestISISTransportRawSocketCap` | `internal/component/isis/transport/transport_integration_linux_test.go` | `integration && linux` | raw-socket open succeeds under `CAP_NET_RAW`; `t.Skip` when the capability is absent (validates A-5) | |

Because `transport_integration_linux_test.go` carries the `//go:build integration && linux`
tag, the `ze-qemu-integration-test` Makefile target picks it up automatically:
`ZE_QEMU_INTEGRATION_PKGS` is DERIVED from that exact tag by a `grep` over
`internal/`/`cmd/` (`mk/test-integration.mk:224-235`), so no Makefile edit is
needed and a new tagged package cannot be silently omitted. The QEMU all-tests
evidence script `scripts/evidence/qemu-all-tests.sh`, however, HARDCODES its
integration-test package list (around lines 144-156) and does NOT derive from
tags, so the `internal/component/isis/transport/...` package MUST be added there
explicitly (Files to Modify) or these tests will be silently skipped in the
all-tests evidence run. The tests use a `veth` pair in a network namespace
(`t.Skip` when `CAP_NET_ADMIN`/`CAP_NET_RAW` are missing).

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred to isis-13) | `test/interop/scenarios/` | FRR isisd | FRR parses Ze 802.3+LLC frames and vice versa; full adjacency needs isis-5 | |

Wire-level framing correctness is proven here by byte-exact unit tests and the
QEMU veth integration test. End-to-end FRR interop requires the adjacency FSM
(isis-5) and is owned by isis-13; this is not a deferral of this spec's own
acceptance criteria.

### Future (if deferring any tests)
- FRR interop of full PDUs over this transport: owned by isis-13 (needs isis-5 adjacency). Not a deferral of isis-3 ACs.
- BSD / VPP backends behind the same interface: out of scope for v1 per umbrella.

## Files to Modify
- `internal/core/diagnostic/codes.go` - add `doctor-isis-raw-socket` code (title, description, examples), alongside existing `doctor-pppoe-module`. isis-3 OWNS this code; isis-13 surfaces but must not re-register it
- `internal/component/iface/events/events.go` - no change expected; consume `EventUp`/`EventDown` (read-only)
- `scripts/evidence/qemu-all-tests.sh` - add `./internal/component/isis/transport/...` to the hardcoded integration-tests package list (around lines 144-156); this script does not derive packages from build tags (unlike `ze-qemu-integration-test`), so without this edit the transport integration tests are skipped in the all-tests evidence

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | none here; per-interface IS-IS config is owned by isis-4 |
| YANG validation constraints | No | n/a in this spec |
| YANG custom validators | No | n/a in this spec |
| CLI commands/flags | No | transport stats surfaced via isis-13 `show isis interface` |
| CLI grammar (action before identifier) | No | n/a in this spec |
| Editor autocomplete | No | n/a in this spec |
| Functional test for new RPC/API | Yes | `test/isis/isis-doctor-raw-socket.ci` (user-visible doctor output); raw send/receive proven by QEMU integration test `transport_integration_linux_test.go` |
| Pipe completeness | No | n/a (no command output in this spec) |
| Env var registration | No | n/a in this spec |
| Doctor check for runtime dependencies | Yes | `doctor-isis-raw-socket` (raw socket open / `CAP_NET_RAW`), owning-package registration + `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | this spec OWNS and registers the transport series from the umbrella `## Shared Contracts` "Metrics (canonical)" table: `ze_isis_frames_sent_total{interface}`, `ze_isis_frames_received_total{interface}`, `ze_isis_frames_dropped_total{interface,reason}`, `ze_isis_sockets_open{}`. Registration is here (per-owner), NOT central in isis-13; isis-13 only scrapes/asserts them |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | transport is internal; user-facing IS-IS feature row owned by isis-13 |
| 2 | Config syntax changed? | No | config owned by isis-4 |
| 3 | CLI command added/changed? | No | owned by isis-13 |
| 4 | API/RPC added/changed? | No | none in this spec |
| 5 | Plugin added/changed? | No | component, not plugin |
| 6 | Has a user guide page? | No | `docs/guide/isis.md` owned by isis-13 |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/isis.md` (802.3+LLC framing, multicast MACs) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `iso/short/iso10589.md` (framing, multicast, padded Hello) |
| 10 | Test infrastructure changed? | Yes | the raw-L2 send/receive tests are QEMU integration tests (`transport_integration_linux_test.go`, `integration && linux`), documented under QEMU/integration testing (`ai/rules/qemu-testing.md`), NOT plain `.ci`; only the user-visible doctor test `test/isis/isis-doctor-raw-socket.ci` is a `docs/functional-tests.md` entry |
| 11 | Affects daemon comparison? | No | owned by isis-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new transport layer; first 802.3/LLC user) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (transport counters) |
| 15 | Registered plugin/event/command/capability changed? | Yes | doctor code `doctor-isis-raw-socket` registered; note in `docs/guide/status.md` via isis-13 |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/isis/transport/transport.go` - the `Backend` interface (open/close per interface, send the final PDU with destination selector, receive `(ifindex, pdu)`, expose interface MTU), circuit registry, iface EventBus subscription, neighbour-MTU-inference logic (platform-neutral); the transport does NOT pad
- `internal/component/isis/transport/frame.go` - 802.3 + LLC frame build (buffer-first) and zero-copy parse; LLC SAP 0xFE constants; no padding (the engine pads the PDU before it reaches the transport)
- `internal/component/isis/transport/multicast.go` - ISO multicast MAC constants (AllL1ISs / AllL2ISs / AllISs) and level-to-MAC selection
- `internal/component/isis/transport/backend_linux.go` - `//go:build linux` AF_PACKET/SOCK_RAW backend: socket open (multicast membership as needed), `Recvfrom` RX, `Sendto` TX, ifindex/hwaddr/MTU resolve via ioctl, `SO_RCVTIMEO`
- `internal/component/isis/transport/backend_other.go` - `//go:build !linux` stub returning a not-supported error so non-Linux builds compile
- `internal/component/isis/transport/doctor_linux.go` - `//go:build linux` doctor check probing raw-socket open / `CAP_NET_RAW`, emitting `doctor-isis-raw-socket`; registration from the owning package
- `internal/component/isis/transport/frame_test.go` - frame build/parse, ethertype-rejection, send-does-not-alter-PDU, boundary tests
- `internal/component/isis/transport/multicast_test.go` - level-to-MAC selection unit tests
- `internal/component/isis/transport/mtu_test.go` - neighbour-MTU inference and mismatch tests
- `internal/component/isis/transport/doctor_test.go` - doctor check unit test (fires when IS-IS configured, emits the code)
- `internal/component/isis/transport/transport_integration_linux_test.go` - `//go:build integration && linux` QEMU two-veth send/receive + MTU expose/mismatch + `CAP_NET_RAW` capability probe (the raw-L2 path lives here, NOT in a `.ci` file)
- `test/isis/isis-doctor-raw-socket.ci` - functional doctor-check test (user-visible `ze doctor --json` output only)

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

1. **Phase: Wiring (MANDATORY FIRST)** -- define the `Backend` interface and circuit registry, subscribe to iface EventBus, write failing wiring tests
   - Tests: `TestISISTransportOpenOnLinkUp`, `TestISISTransportCloseOnLinkDown`
   - Files: `internal/component/isis/transport/transport.go`, `backend_other.go` stub
   - Verify: `interface/up`/`interface/down` reach the transport; backend methods are stubs so wiring tests fail on missing I/O
2. **Phase: Frame codec** -- 802.3 + LLC build/parse, multicast constants (no padding: the engine pads the PDU before it reaches the transport)
   - Tests: `TestBuildFrame`, `TestParseFrame`, `TestParseFrameRejectEthertype`, `TestMulticastMACForLevel`, `TestSendDoesNotAlterPDU`, boundary tests
   - Files: `frame.go`, `multicast.go`
   - Verify: byte-exact frame layout; ethertype-shaped frames rejected; the final PDU is sent byte-for-byte (no pad); a PDU larger than MTU is rejected
3. **Phase: Linux AF_PACKET backend** -- socket open (multicast membership), `Recvfrom` RX goroutine with ifindex dispatch, `Sendto` TX, ioctl resolve, `SO_RCVTIMEO`
   - Tests: `transport_integration_linux_test.go` (QEMU two-veth send/receive)
   - Files: `backend_linux.go`
   - Verify: frame sent on one veth received on the peer with correct ifindex; resolve A-2 (membership/promisc) and record the outcome
4. **Phase: MTU expose + neighbour inference** -- expose the ioctl interface MTU to the engine, record observed received frame size, infer/compare neighbour MTU, surface mismatch (the transport never pads)
   - Tests: `TestExposeInterfaceMTU`, `TestInferNeighbourMTU`, QEMU MTU-mismatch case
   - Files: `transport.go`, `mtu_test.go`
   - Verify: exposed MTU equals the ioctl value; mismatch detected and reported per ISO/IEC 10589 sec 8.2.3
5. **Phase: Doctor check + metrics** -- `doctor-isis-raw-socket` code + check, transport Prometheus counters
   - Tests: `TestISISDoctorRawSocket`, `doctor_test.go`
   - Files: `doctor_linux.go`, `internal/core/diagnostic/codes.go`
   - Verify: `ze doctor --json` exposes the code; `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered'` passes
6. **Functional + QEMU integration tests** -> `test/isis/isis-doctor-raw-socket.ci` (user-visible doctor output) and `transport_integration_linux_test.go` (raw send/receive on veth). The `ze-qemu-integration-test` target auto-derives this package from its `//go:build integration && linux` tag (`ZE_QEMU_INTEGRATION_PKGS`, no Makefile edit); add the package explicitly only to the hardcoded list in `scripts/evidence/qemu-all-tests.sh`
7. **RFC refs** -> Add `// ISO/IEC 10589 Section 8.2.3 ...` comments on the framing / MTU-detection code
8. **Full verification** -> `make ze-verify` + `make ze-qemu-integration-test`
9. **Complete spec** -> fill audit tables, write learned summary to `plan/learned/NNN-isis-3-l2-transport.md`; two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has implementation with file:line |
| Feature completeness | Send + receive + multicast select + MTU expose/infer + link-event lifecycle + doctor all present; transport does NOT pad (engine owns padding); backend interface lets a second backend drop in |
| Correctness | Frame is 802.3 length + LLC (0xFE/0xFE/0x03) + PDU, NOT an ethertype; multicast MAC bytes exact; the final PDU is sent byte-for-byte (no padding by the transport) |
| Naming | Package `transport`; doctor code `doctor-isis-raw-socket`; backend file `backend_linux.go` / `backend_other.go` |
| Data flow | Engine never touches the socket; frames flow socket -> codec -> engine; iface events drive lifecycle |
| CLI grammar | n/a (no CLI in this spec) |
| Doctor checks | `doctor-isis-raw-socket` registered per `ai/rules/doctor-checks.md`; unit + functional test present |
| YANG validation | n/a (config owned by isis-4) |
| Prometheus counters | transport counters defined and registered; names listed |
| Rule: qemu-testing | linux backend has `integration && linux` QEMU test; no hardware-only skip |
| Rule: plugin-self-containment | all transport code + doctor check under `internal/component/isis/transport/` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Transport package | `ls internal/component/isis/transport/` |
| Linux backend + non-linux stub | `ls internal/component/isis/transport/backend_linux.go internal/component/isis/transport/backend_other.go` |
| Doctor code registered | `grep doctor-isis-raw-socket internal/core/diagnostic/codes.go` |
| Functional test (user-visible) | `ls test/isis/isis-doctor-raw-socket.ci` |
| QEMU integration test (raw L2) | `ls internal/component/isis/transport/transport_integration_linux_test.go`; auto-derived into `ze-qemu-integration-test` via its `integration && linux` build tag (`ZE_QEMU_INTEGRATION_PKGS` in `mk/test-integration.mk`); `grep isis/transport scripts/evidence/qemu-all-tests.sh` confirms the explicit add to the hardcoded all-tests list |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every received frame validated (min length, 802.3 length bounds, LLC SAP/control) before slicing into the PDU; reject not panic |
| Resource exhaustion | Bounded receive buffer; per-circuit goroutine count tied to link state; no unbounded frame queue |
| Privilege | `CAP_NET_RAW` only; consider dropping after socket open if feasible; doctor check surfaces the requirement |
| Spoofing | Source MAC / ifindex recorded for the engine to apply adjacency checks (level/area enforcement is isis-5) |
| Crafted frames | Malformed 802.3 length or LLC must not over-read the buffer; fuzz the parser |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; re-check framing against ISO/IEC 10589 |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional/QEMU test fails | Check AC; if A-2 (multicast membership) is the cause, add `PACKET_ADD_MEMBERSHIP` and record |
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

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- v1 ships only the Linux `AF_PACKET` backend; BSD/VPP backends are out of scope (umbrella).
- Full FRR interop over this transport is proven in isis-13 (needs the isis-5 adjacency FSM); isis-3 proves framing by byte-exact unit tests and the QEMU veth send/receive integration test.
- The transport does NOT pad PDUs; padding is owned by the engine (isis-5) and added before authentication (umbrella Shared Contracts). The transport only exposes the interface MTU and infers the neighbour MTU for mismatch detection.

## RFC Documentation

Add `// ISO/IEC 10589 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: 802.3 + LLC SAP 0xFE framing, ISO multicast MAC selection, MTU detection from received padded Hellos (sec 8.2.3; padding itself is added by the engine, not this transport), receive-path length/SAP validation.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Raw L2 send/receive for IS-IS | QEMU integration test | `transport_integration_linux_test.go` (`TestISISTransportVethRoundTrip`) |
| 802.3 + LLC framing (not ethertype), final PDU sent unaltered | unit test | `TestBuildFrame`, `TestParseFrameRejectEthertype`, `TestSendDoesNotAlterPDU` |
| Expose MTU + neighbour-MTU mismatch detection (transport does NOT pad) | unit + QEMU test | `TestExposeInterfaceMTU`, `TestInferNeighbourMTU`, QEMU MTU-mismatch case |
| `CAP_NET_RAW` doctor check | functional test | `test/isis/isis-doctor-raw-socket.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

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
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-qemu-integration-test` passes (linux backend)
- [ ] Feature code integrated (`internal/component/isis/transport/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); A-2 (multicast membership) explicitly resolved

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (backend interface justified by future BSD/VPP)
- [ ] No speculative features (only Linux backend in v1)
- [ ] Single responsibility per file (frame / multicast / backend / doctor)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (engine never touches the socket)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (framing proven here; full FRR interop owned by isis-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-3-l2-transport.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-isis-3-l2-transport.md` only
