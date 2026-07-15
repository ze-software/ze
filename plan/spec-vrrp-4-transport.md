# Spec: vrrp-4 -- VRRP Transport (raw sockets, GARP/NA senders, doctor, metrics)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vrrp-1 |
| Phase | 9/10 |
| Updated | 2026-07-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-vrrp-0-umbrella.md` -- decisions, A-3 (socket split), R-5 (GARP/NA senders)
4. `rfc/short/rfc9568.md` (Constants, MUST Tx/Rx, source-address + TTL rules), `rfc/short/rfc3768.md`
5. `internal/plugins/ospf/transport/backend_linux.go` -- raw-IP socket model
6. `internal/plugins/isis/transport/backend_linux.go` -- AF_PACKET model; `internal/plugins/isis/transport/register.go` + `doctor*.go` + `metrics.go` -- doctor/metrics model

## Task

Build `internal/plugins/vrrp/transport`: the per-instance raw-socket layer for VRRP
adverts (IP protocol 112, IPv4 and IPv6), plus the two net-new LAN announcers that
make failover actually move traffic -- a gratuitous-ARP sender and an unsolicited
Neighbor Advertisement sender -- with a `doctor-vrrp-raw-socket` readiness check,
transport-owned Prometheus metrics, a non-Linux stub so darwin builds and config
validation keep working, and QEMU integration tests for every Linux-only file.

Socket model (umbrella decision, holo split, umbrella A-3): per instance,
**receive** on a raw proto-112 socket bound to the PARENT interface with the
multicast group joined (224.0.0.18 / ff02::12), **transmit** on a raw proto-112
socket bound to the instance's MACVLAN so frames egress with the virtual MAC
00:00:5e:00:0{1|2}:{vrid}. GARP goes out an AF_PACKET socket on the macvlan;
unsolicited NA goes out a raw ICMPv6 socket on the macvlan.

Out of scope here: packet encode/decode/validation (spec-vrrp-1), FSM/timers
(spec-vrrp-2), macvlan device lifecycle (spec-vrrp-3), plugin registration /
YANG / engine wiring / show commands (spec-vrrp-5), keepalived interop
(spec-vrrp-6). This spec consumes spec-vrrp-1's encoder (`WriteTo` into a caller
buffer), its IHL-aware IPv4 header strip helper, and its receive-error taxonomy
(metric `reason` labels).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/module-tiers.md` - package placement
  → Decision: transport is a sub-package of the vrrp edge plugin (`internal/plugins/vrrp/transport`), same tier and shape as `internal/plugins/ospf/transport` and `internal/plugins/isis/transport`
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Constraint: everything (doctor check, probe, metrics, frame builders) lives under `internal/plugins/vrrp/transport/`; the only central touch is the CodeMeta row in `internal/core/diagnostic/codes.go` (established isis/ospf precedent, `codes.go:361,367`)
- [ ] `ai/rules/goroutine-lifecycle.md` - worker rules
  → Constraint: one long-lived readLoop goroutine per rx socket (SO_RCVTIMEO wakeups) + one long-lived per-instance announcer worker fed by a channel; NO per-packet and NO per-burst goroutines
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`, `ai/rules/no-sprintf-alloc.md` - hot-path memory
  → Constraint: adverts encode via spec-vrrp-1 `WriteTo(buf, off)` into a per-instance reusable buffer; GARP/NA frames build into per-instance reusable buffers; rx payload is copied once out of the shared recv buffer (ospf `deliverDatagram` model) and never re-copied
- [ ] `ai/rules/qemu-testing.md` - linux-only testing
  → Constraint: every `//go:build linux` file here ships `//go:build integration && linux` QEMU tests (veth + netns); `ZE_QEMU_INTEGRATION_PKGS` in `mk/test-integration.mk:319` derives the package list from that build tag, so the new package is picked up with NO Makefile edit (verified below)
- [ ] `ai/rules/doctor-checks.md` - readiness checks
  → Constraint: transport owns the raw-socket dependency, so it owns registration (`diagnostic.RegisterDoctorCheck` from its own `register.go` init, component-style path -- the transport is not its own plugin Registration), the probe, the CodeMeta, the unit test, and a functional test through `ze doctor`
- [ ] `ai/rules/spec-no-code.md`, `ai/rules/planning.md` - spec style
  → Constraint: tables and prose only; plain fences for byte-layout examples only
- [ ] `plan/spec-vrrp-0-umbrella.md` - parent decisions
  → Decision: macvlan virtual MAC from day one; rx-parent/tx-macvlan split is umbrella A-3, validated HERE by QEMU test; GARP/NA correctness is umbrella R-5, mitigated HERE by golden-byte tests
- [ ] `ai/rules/testing.md`, `ai/rules/tdd.md`, `ai/rules/interop-and-goal-validation.md` - test discipline
  → Constraint: payload-predicate waits, never sleeps, in QEMU tests; interop proof itself is spec-vrrp-6's mandate, this spec supplies the frames it exercises

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - VRRPv3
  → Constraint: Tx TTL/Hop Limit MUST be 255 (sec 5.1.1.3 / 5.1.2.3); dst 224.0.0.18 / ff02::12 (Constants); source MAC MUST be the virtual router MAC (sec 7.2); source IP MUST be the interface's primary IPv4 address or its link-local IPv6 address (sec 7.2); IPv6 checksum uses the pseudo-header with Next Header 112 (sec 5.2.8); gratuitous ARP carries the virtual IPv4 address with the virtual MAC as target link-layer (errata 7947/7949); unsolicited NA has R set, S clear, O set with TLL = virtual router MAC (sec 6.4.1/6.4.2); delay GARP/ND at boot until address AND virtual MAC are configured (sec 8.1.2/8.2.2)
- [ ] `rfc/short/rfc3768.md` - VRRPv2
  → Constraint: identical IPv4 transport surface (proto 112, dst 224.0.0.18, TTL 255, virtual MAC source, primary-IPv4 source, sec 5.2.1-5.2.4, 7.2); the transport is version-agnostic -- only the codec payload differs, so v2 needs no transport branch

**Key insights:**
- The transport never inspects VRRP payload bytes (isis transport discipline); spec-vrrp-1 encodes/validates, spec-vrrp-2 times, this package moves bytes and builds the two L2/ND announcement frames.
- Both reference implementations get the announcers wrong (uvrrpd: NA MAC typo + missing TLL; holo: NA wrong pseudo-header/dst/TLL/hop-limit -- holo bug 7; stale pre-encoded adverts -- holo bug 8; missing IPV6_MULTICAST_HOPS 255 -- holo bug 13; dead counters -- holo bug 9). Each is an explicit negative test here.
- The tx macvlan carries no stable IPv4 address, which forces the IP_HDRINCL decision (Key Design Decisions #1).

## Current Behavior (MANDATORY)

**Source files read:** (producers read directly this session)
- [ ] `internal/plugins/ospf/transport/backend_linux.go` - the raw-IP model: `openInterfaceSocket` :235 (AF_INET/SOCK_RAW proto + SO_BINDTODEVICE), `setMulticastOptions` :263 (IP_TTL/IP_MULTICAST_TTL 1 -- vrrp needs 255 -- MULTICAST_LOOP 0, MULTICAST_IF), `joinGroup` :287 (IP_ADD_MEMBERSHIP via IPMreq with local addr), `Send` :155 (unix.Sendto under sendMu), `readLoop` :194 (SO_RCVTIMEO 500ms wakeups, EAGAIN/EINTR continue, EBADF/EINVAL return), `deliverDatagram` :217 (strip header, copy payload, blocking send to recvCh), iface seam vars :30-31 (`resolveIfaceBinding`/`resolveIfaceAddresses` overridable in tests), `interfaceIPv4` :246 (first ipv4 from `iface.Addresses` = primary)
- [ ] `internal/plugins/ospf/transport/transport.go` - `StripIPv4Header` :525 (IHL-aware, rejects short/IHL overrun; returns payload + src but DISCARDS TTL -- vrrp uses spec-vrrp-1's helper which also extracts TTL and dst)
- [ ] `internal/plugins/ospf/transport/backend_other.go` - stub pattern: `ErrUnsupportedPlatform` typed error :9, `NewBackend` returns stub :13, OpenInterface returns the error :15
- [ ] `internal/plugins/isis/transport/backend_linux.go` - AF_PACKET model for GARP: `OpenCircuit` :53 (socket AF_PACKET/SOCK_RAW + bind by ifindex :66-73), `Send` :135 (BuildFrame into reusable sendBuf + Sendto SockaddrLinklayer under sendMu), `joinMulticast` :238 (PACKET_ADD_MEMBERSHIP), `htons` :45, rcvTimeout 500ms :37
- [ ] `internal/plugins/isis/transport/register.go` - doctor registration model :20-35 (diagnostic.DoctorCheck{Name, Phase PostConfig, Order, Component, Codes, Check} from init; exit 2 on failure)
- [ ] `internal/plugins/isis/transport/doctor.go` - check fn :26-42 (gate on config tree container, probe seam var :21, SeverityWarning diagnostic)
- [ ] `internal/plugins/isis/transport/doctor_linux.go` - probe :14 (open + close the exact socket the transport needs)
- [ ] `internal/plugins/isis/transport/doctor_other.go` - non-linux probe stub returns true :12 (no dependency off linux, doctor stays quiet)
- [ ] `internal/plugins/isis/transport/metrics.go` - transport-owned counters :23 (`newTransportMetrics` registers CounterVec/Gauge on the injected registry), NopRegistry fallback :49 (`nopTransportMetrics` before SetMetrics / in unit tests)
- [ ] `internal/plugins/isis/transport/transport.go` - orchestrator wiring: `New` seam `subscribe: iface.Subscribe` :141, `SubscribeIfaceEvents` :258, rescan backstop :284-299 + `RescanInterfaces` :333, `SetMetrics` :150, deliver channel cap 256 :142, per-circuit rxLoop fan-in :411, `Close` teardown-before-wait :587
- [ ] `internal/core/diagnostic/codes.go` - builtin CodeMeta rows for transport-owned codes: `doctor-isis-raw-socket` :361, `doctor-ospf-raw-socket` :367 (title + description + `ze explain` examples)
- [ ] `internal/component/iface/integration_helpers_linux_test.go` - `withNetNS` :36 (netns create, t.Skip without CAP_NET_ADMIN, cleanup restore) -- the QEMU test pattern to reuse

Also read (not Go): `mk/test-integration.mk` :314-325 -- `ZE_QEMU_INTEGRATION_PKGS` is derived by grepping for `^//go:build integration && linux`, so `./internal/plugins/vrrp/transport` joins `ze-qemu-integration-test` automatically once its integration test file exists. No Makefile change is needed for this spec.

**Behavior to preserve:**
- ospf and isis transports untouched (vrrp gets its own package; no shared code moves)
- The builtin-codes list in `internal/core/diagnostic/codes.go` only GAINS a row; existing codes, doctor phases, and orders unchanged
- `ze-qemu-integration-test` package derivation stays tag-driven; no hardcoded package added
- darwin `make ze-verify` stays green: all socket code behind `//go:build linux`, stub returns a typed error, frame builders are platform-independent

**Behavior to change:**
- None removed. New: `internal/plugins/vrrp/transport` package, one CodeMeta row in `codes.go`, new `ze_vrrp_*` transport metric series.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Engine calls (spec-vrrp-5 wires them): OpenInstance (per configured group), UpdateAdvert (FSM action parameters changed), SendAdvert (Adver_Timer fire and prio-0 shutdown), AnnounceMaster (Master transition, VIP list), CloseInstance / Close
- Wire rx: proto-112 datagrams delivered by the kernel to the per-instance rx socket bound to the parent interface (multicast 224.0.0.18 / ff02::12, plus any unicast proto-112 the kernel passes)
- iface address events on the parent unit (source-address re-resolution)
- Doctor runner (post-config phase) and metrics registry injection (ConfigureMetrics -> SetMetrics)

### Transformation Path
1. OpenInstance(family, vrid, parent, macvlanDevice, virtualMAC): resolve parent binding/addresses via the iface seam (`resolveIfaceBinding`/`resolveIfaceAddresses` vars, ospf `backend_linux.go:30-31` pattern); open the rx socket set on the parent (join group, SO_RCVTIMEO); open the tx advert socket + announce socket on the macvlan; start the readLoop and announcer worker; sockets_open gauge up.
2. UpdateAdvert(params): spec-vrrp-1 `WriteTo` encodes the advert into the per-instance tx buffer; IPv4 additionally builds the 20-byte IPv4 header in front (IP_HDRINCL); the buffer is versioned so a send never uses stale parameters (holo bug 8). IPv4 source re-resolves from the parent's addresses here and on parent address events.
3. SendAdvert(): sendto of the prepared buffer (v4: header+payload to 224.0.0.18; v6: payload with IPV6_PKTINFO cmsg pinning source = macvlan link-local + macvlan ifindex, dst ff02::12); adverts_sent counter on success, packet_errors{reason=send-error} on failure; v6 with no macvlan link-local yet: skip + count {reason=no-link-local}, retry naturally on next timer tick.
4. Datagram arrives -> readLoop (one long-lived goroutine per rx socket): Recvfrom into the per-socket reusable buffer; IPv4: spec-vrrp-1 IHL-aware strip extracts payload, src, dst, TTL; IPv6: Recvmsg cmsgs IPV6_HOPLIMIT + IPV6_PKTINFO give hop-limit and dst, Recvfrom src; build RxMeta, copy payload once, non-blocking send to the bounded engine channel; on full channel drop + count {reason=rx-overflow}; adverts_received counter on delivery.
5. AnnounceMaster(vips) -> enqueue a burst job to the announcer worker: per IPv4 VIP build the GARP frame, per IPv6 VIP build the unsolicited NA, send each frame `announceRepeatCount` times with `announceRepeatInterval` spacing through the macvlan announce socket; announcements_sent{kind} per frame.
6. Counters/gauges -> transport metrics registry (NopRegistry until SetMetrics).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| transport <-> kernel | AF_INET/AF_INET6 SOCK_RAW proto 112 (rx parent, tx macvlan), AF_PACKET SOCK_RAW ETH_P_ARP (GARP), AF_INET6 SOCK_RAW IPPROTO_ICMPV6 (NA); build-tagged linux, stub elsewhere | [ ] |
| transport <-> engine (spec-vrrp-5) | same-process Go calls in; bounded (RxMeta, payload) channel out; RecordRxError hook for codec-validation counters | [ ] |
| transport <-> codec (spec-vrrp-1) | `WriteTo(buf, off)` encode into caller buffer; IHL-aware IPv4 strip helper; error-taxonomy reason strings | [ ] |
| transport <-> iface component | `iface.Resolve` / `iface.Addresses` / `iface.Subscribe` through overridable seam vars (ospf/isis pattern) | [ ] |
| transport <-> doctor | `diagnostic.RegisterDoctorCheck` from transport init; CodeMeta row in `internal/core/diagnostic/codes.go` | [ ] |
| transport <-> telemetry | `metrics.Registry` injected via SetMetrics; NopRegistry fallback | [ ] |

### Integration Points
- `internal/plugins/ospf/transport/backend_linux.go:235,263,287,194,217` - socket open / options / join / readLoop / deliver shapes copied and adapted (TTL 255 instead of 1; drop-on-overflow instead of blocking)
- `internal/plugins/isis/transport/backend_linux.go:53,135,238` - AF_PACKET open/bind/send/join shape for the GARP socket
- `internal/plugins/isis/transport/register.go:20`, `doctor.go:21,26`, `doctor_linux.go:14`, `doctor_other.go:12` - doctor check anatomy (probe seam, config gate, warning diagnostic)
- `internal/plugins/isis/transport/metrics.go:23,49` - metrics ownership + NopRegistry fallback
- `internal/plugins/isis/transport/transport.go:141,258,284` - iface.Subscribe seam, event wiring, rescan backstop (vrrp uses the same seam for parent ADDRESS re-resolution; link up/down instance lifecycle is the engine's call via spec-vrrp-5)
- `internal/core/diagnostic/codes.go:361` - CodeMeta placement precedent for `doctor-vrrp-raw-socket`
- `mk/test-integration.mk:319` - tag-derived QEMU package list (no edit required)
- spec-vrrp-1 (sibling): encoder `WriteTo`, IPv4 strip helper with TTL/dst extraction, error taxonomy for `reason` labels
- spec-vrrp-3 (sibling): produces the macvlan device + virtual MAC that OpenInstance receives BY NAME; this spec never creates devices

### Architectural Verification
- [ ] No bypassed layers (transport touches the kernel only through its own sockets; interface state queries go through iface seams; devices come from spec-vrrp-3 via the engine, never created here)
- [ ] No unintended coupling (no imports of vrrp fsm/engine packages; engine depends on transport, not vice versa; no sibling-plugin imports)
- [ ] No duplicated functionality (IPv4 strip and checksum live in spec-vrrp-1 and are consumed, not re-implemented; socket patterns adapted from ospf/isis, not shared prematurely -- three transports with different frame disciplines)
- [ ] Zero-copy preserved where applicable (rx: one copy out of the shared recv buffer; tx: encode-in-place into reusable per-instance buffers; no per-packet allocations beyond the single rx payload copy)
- [ ] Registration over hardcoding -- doctor check registered via `diagnostic.RegisterDoctorCheck` from the owning package; metrics registered on the injected registry; no vrrp spelling added to any central runner, switch, or factory (the CodeMeta table row in `codes.go` is the documented builtin-listing mechanism every transport uses, `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | macvlan bridge mode delivers proto-112 multicast to the PARENT-bound rx socket while tx from the macvlan egresses with the virtual MAC (umbrella A-3) | holo-vrrp runs exactly this split in production (research-holo-digest) | rx moves to a macvlan-bound socket, or tx falls back to AF_PACKET full frames (uvrrpd model); interface change contained in backend_linux.go | `TestIntegrationAdvertOnPeerVeth` + `TestIntegrationRxDeliversFromPeer` (QEMU, veth + netns) | unvalidated |
| A-2 | An IP_HDRINCL raw send on a SO_BINDTODEVICE-bound addressless macvlan egresses with the macvlan MAC as L2 source | holo tx socket model (digest); Linux raw(7) documents IP_HDRINCL send behavior | build full L2 frames on AF_PACKET for adverts too (uvrrpd model); Send path swaps, API unchanged | `TestIntegrationAdvertOnPeerVeth` asserts L2 src MAC == macvlan MAC | unvalidated |
| A-3 | Linux fills IPv4 header checksum, total length, and id when zero under IP_HDRINCL, so the tx path zeroes them | raw(7) man page (documented kernel contract) | compute IPv4 header checksum in software in the header builder | `TestIntegrationAdvertOnPeerVeth` verifies the captured IPv4 header checksum | unvalidated |
| A-4 | Raw IPPROTO_ICMPV6 sockets always kernel-compute the ICMPv6 checksum (RFC 3542 sec 3.1), so the NA sender needs no IPV6_CHECKSUM option | RFC 3542 sec 3.1; holo uses a raw ICMPv6 socket for NA | set IPV6_CHECKSUM explicitly or compute via the spec-vrrp-1 pseudo-header helper | `TestIntegrationNAOnWire` verifies the captured NA checksum against a software computation | unvalidated |
| A-5 | IPV6_CHECKSUM at offset 6 on the SOCK_RAW proto-112 tx socket makes the kernel fill the VRRP checksum (pseudo-header included) | holo tx socket sets exactly this (digest) | codec computes the v6 checksum in software before send (it already does for v4 and unit tests) | `TestIntegrationAdvertV6OnWire` verifies checksum + peer-side software validation | unvalidated |
| A-6 | spec-vrrp-1 ships `WriteTo(buf, off)`, an IHL-aware IPv4 strip helper exposing TTL + src + dst, and a stable error-reason taxonomy | umbrella child-1 scope row + this child's task statement | transport defines a local RxMeta/strip until child 1 lands; coordinate before coding | implementation-start audit against the merged spec-vrrp-1 package | unvalidated |
| A-7 | Parent unit primary IPv4 = first configured IPv4 address, resolvable via `iface.Addresses` | ospf does exactly this (`backend_linux.go:246-261` interfaceIPv4) | engine passes an explicit source address parameter into OpenInstance/UpdateAdvert | `TestSendAdvertUsesParentPrimaryV4Source` with seam-injected addresses | unvalidated |
| A-8 | The QEMU integration Makefile picks up the new package automatically from build tags | `mk/test-integration.mk:314-319` derives ZE_QEMU_INTEGRATION_PKGS from `^//go:build integration && linux` | add the package explicitly to the target | run the derivation grep after creating the integration test file; `make -n ze-qemu-integration-test` lists the package | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Wrong GARP/NA bytes are silently ineffective -- traffic blackholes after failover while all unit tests pass (umbrella R-5) | Peer ARP/ND cache does not repoint in the QEMU lab / interop scenarios | Golden-byte frame tests (exact 42-byte GARP, exact 32-byte NA) + on-wire byte comparison in QEMU + spec-vrrp-6 keepalived failover evidence |
| R-2 | Kernel drops or rewrites NA/adverts sourced from an address not yet assigned (macvlan link-local under DAD) | Integration test sees no frame on the peer veth right after macvlan creation | Skip-and-count until link-local present ({reason=no-link-local}); adverts retry on the next FSM timer tick; announcer bursts re-run on the next Master transition; document the DAD window in Known Limitations |
| R-3 | Multiple instances on one parent each receive EVERY proto-112 datagram (kernel fan-out to all matching raw sockets), multiplying rx work | rx counters grow at N x advert rate with N groups on one interface | Accept for v1 (VRRP rates are trivially low; codec discards foreign VRIDs cheaply); a shared per-parent rx socket with vrid dispatch is the recorded fallback |
| R-4 | Drop-on-overflow rx channel drops a prio-0 advert under flood, delaying a failover by one skew window | rx-overflow counter increments during chaos/flood tests | Channel cap 256 (isis precedent) is >> any legitimate VRRP rate; overflow only under attack, where GTSM TTL validation discards the flood downstream anyway; counter makes it observable |
| R-5 | SO_RCVTIMEO polling (2 wakeups/s per socket) multiplied by many groups wastes cycles | CPU profile in a 100-group QEMU run | Accepted: identical to ospf/isis precedent; sockets are per-instance by design decision |
| R-6 | QEMU capture tests flake on timing (frame not yet on wire when asserted) | Intermittent CI failures in integration target | Payload-predicate waits reading the AF_PACKET capture socket with deadline, never fixed sleeps (ai/rules/testing.md sleep ratchet) |
| R-7 | IPv6 multicast hop limit silently defaults to 1 if IPV6_MULTICAST_HOPS is forgotten on any v6-sending socket (holo bug 13) | Peer receives adverts/NA with hop limit 1 and (correctly) discards | Explicit option on BOTH the v6 advert socket and the NA socket; negative integration assertions capture and check hop limit == 255 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine opens a configured group (spec-vrrp-5 OnStarted) | → | transport OpenInstance: sockets + joins + readLoop + announcer | `TestTransportOpenInstanceWiring` (fake backend) + `test/vrrp/vrrp-instance-up.ci` (spec-vrrp-5, needs-linux) |
| Adver_Timer fire (spec-vrrp-2 action) | → | UpdateAdvert/SendAdvert -> advert datagram on the wire | `TestIntegrationAdvertOnPeerVeth` + `test/vrrp/vrrp-instance-up.ci` (spec-vrrp-5) |
| proto-112 datagram arrives on parent | → | readLoop -> (RxMeta, payload) on the engine channel | `TestReadLoopDeliversRxMeta` (fake backend) + `test/vrrp/vrrp-backup-hold.ci` (spec-vrrp-6, QEMU) |
| Master transition (spec-vrrp-2 action) | → | AnnounceMaster -> GARP/NA burst on macvlan | `TestAnnounceMasterBurstWiring` + ~~`test/vrrp/vrrp-backup-hold.ci`~~ `test/vrrp/vrrp-failover.ci` (spec-vrrp-6 owns promotion+GARP there; Finding 13) + failover step of `scripts/evidence/effective-vrrp-keepalived.py` |
| `ze doctor` on a vrrp config | → | checkVRRPRawSocket via doctor registry | `TestVRRPRawSocketDoctorWarn` + `test/vrrp/vrrp-doctor.ci` (spec-vrrp-5) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | OpenInstance(ipv4, vrid, parent, macvlan) on linux | rx socket AF_INET/SOCK_RAW proto 112 bound (SO_BINDTODEVICE) to the parent OS device with 224.0.0.18 joined by parent ifindex; tx socket bound to the macvlan with IP_HDRINCL=1 and IP_MULTICAST_LOOP=0; AF_PACKET ETH_P_ARP socket bound to macvlan ifindex; getsockopt read-back confirms each option |
| AC-2 | OpenInstance(ipv6, ...) on linux | rx socket AF_INET6/SOCK_RAW proto 112 bound to parent, ff02::12 joined by ifindex, IPV6_RECVPKTINFO + IPV6_RECVHOPLIMIT set; tx socket bound to macvlan with IPV6_MULTICAST_HOPS=255, IPV6_TCLASS=0xc0, IPV6_MULTICAST_LOOP=0, IPV6_CHECKSUM=6; NA socket IPPROTO_ICMPV6 on macvlan with IPV6_MULTICAST_HOPS=255 |
| AC-3 | SendAdvert (v4) after UpdateAdvert | Peer capture shows: L2 src = macvlan (virtual) MAC, IPv4 TTL 255, TOS 0xc0, proto 112, dst 224.0.0.18, src = parent unit primary IPv4, header checksum valid, payload byte-identical to the spec-vrrp-1 encoding |
| AC-4 | SendAdvert (v6) after UpdateAdvert | Peer capture shows: hop limit 255, traffic class 0xc0, dst ff02::12, src = macvlan link-local, next header 112, VRRP checksum valid under pseudo-header recomputation |
| AC-5 | UpdateAdvert with changed priority/interval/VIPs, then SendAdvert | Next datagram carries the NEW parameters (no stale pre-encoded advert -- holo bug 8 negative) |
| AC-6 | Datagram received on the parent (any TTL value) | Engine channel receives (RxMeta{Src, Dst, TTL/HopLimit, Family, IfIndex}, payload) with the IPv4 header stripped IHL-aware; TTL delivered unmodified (255 AND non-255 both delivered -- GTSM discard is spec-vrrp-1 validation, fed by this meta); spec-vrrp-1 Decode derives address size from RxMeta.Family (orchestrator D-A) |
| AC-7 | Engine channel full (256 queued) when a datagram arrives | Datagram dropped without blocking the readLoop, packet_errors{reason=rx-overflow} incremented, no goroutine or allocation per packet |
| AC-8 | AnnounceMaster with N IPv4 VIPs | Per VIP, exactly announceRepeatCount (3) GARP frames spaced announceRepeatInterval (100ms) apart; each frame golden-byte exact (broadcast dst, virtual-MAC eth src + sha, op=1, sip=tip=VIP, ~~tha zero~~ tha = virtual MAC per rfc/short/rfc9568.md errata 7947/7949, orchestrator D-E / Finding 5); announcements_sent{kind=garp} += 3N |
| AC-9 | AnnounceMaster with N IPv6 VIPs | Per VIP, 3 unsolicited NAs: type 136, R=1 S=0 O=1, target=VIP, TLL option = virtual MAC, dst ff02::1, hop limit 255, src = macvlan link-local (holo bug 7 quadruple negative: pseudo-header, dst, TLL, hop limit all asserted) |
| AC-10 | SendAdvert (v6) while the macvlan has no link-local yet | Send skipped, packet_errors{reason=no-link-local} incremented, no error returned upward, no crash; next SendAdvert retries |
| AC-11 | `ze doctor` with vrrp groups configured and CAP_NET_RAW absent | Warning diagnostic `doctor-vrrp-raw-socket` with actionable message; with no vrrp configured: silent; `ze explain doctor-vrrp-raw-socket` resolves (CodeMeta registered) |
| AC-12 | Metrics registry injected via SetMetrics | All five series registered with declared names/labels; every counter incremented by the path it observes (no holo-bug-9 dead counters); NopRegistry used before injection |
| AC-13 | Build/config-validate on darwin | Package compiles; OpenInstance returns the typed unsupported-platform error (ospf `backend_other.go:9` model); frame builders unit-testable on darwin |
| AC-14 | CloseInstance / Close | All fds closed, readLoop and announcer goroutines exit (no leak under `-race` with goroutine count check), sockets_open gauge returns to 0 |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a v3 IPv4 group, commits | config -> engine (spec-vrrp-5) -> OpenInstance -> UpdateAdvert/SendAdvert -> advert multicast visible to LAN peers | `test/vrrp/vrrp-instance-up.ci` (spec-vrrp-5) + `TestIntegrationAdvertOnPeerVeth` |
| 2 | Watches failover: backup promotes and hosts keep reaching the VIP | master loss -> FSM (spec-vrrp-2) -> AnnounceMaster -> GARP/NA repoint bridges and neighbor caches | ~~`test/vrrp/vrrp-backup-hold.ci`~~ `test/vrrp/vrrp-failover.ci` (Finding 13) + `scripts/evidence/effective-vrrp-keepalived.py` failover step (spec-vrrp-6) |
| 3 | Runs `ze doctor` before deployment without CAP_NET_RAW | doctor registry -> checkVRRPRawSocket -> actionable warning | `test/vrrp/vrrp-doctor.ci` (spec-vrrp-5) + `TestVRRPRawSocketDoctorWarn` |
| 4 | Scrapes Prometheus during operation | transport counters -> metrics registry -> scrape | `test/vrrp/vrrp-metrics.ci` (spec-vrrp-5) + `TestTransportMetricsRegistered` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildGARPFrameGolden` | `internal/plugins/vrrp/transport/garp_test.go` | Exact 42-byte frame for vrid 10 / VIP 192.0.2.1 (layout below); runs on darwin | |
| `TestBuildGARPFramePerVIP` | `internal/plugins/vrrp/transport/garp_test.go` | One frame per IPv4 VIP; zero VIPs -> zero frames | |
| `TestBuildNAMessageGolden` | `internal/plugins/vrrp/transport/na_test.go` | Exact 32-byte ICMPv6 NA (type 136, flags 0xa0000000, target, TLL option) for vrid 10 / VIP 2001:db8::1 | |
| `TestNAHoloBugNegatives` | `internal/plugins/vrrp/transport/na_test.go` | Golden asserts dst ff02::1 (not target), TLL present, hop limit 255 requested, checksum-relevant bytes match a software pseudo-header computation (holo bug 7) | |
| `TestAnnounceBurstRepeatsAndSpacing` | `internal/plugins/vrrp/transport/announce_test.go` | 3 repeats per VIP with 100ms spacing via injected timing seam; worker drains queued bursts; stop terminates worker | |
| `TestAnnounceMasterBurstWiring` | `internal/plugins/vrrp/transport/announce_test.go` | AnnounceMaster on an open instance reaches the announce sender (fake sender records frames) | |
| `TestTransportOpenInstanceWiring` | `internal/plugins/vrrp/transport/transport_test.go` | OpenInstance drives Backend.OpenInstance (fake), registers the instance, bumps sockets_open | |
| `TestUpdateAdvertReencodes` | `internal/plugins/vrrp/transport/transport_test.go` | Priority change between sends -> second send carries new bytes (holo bug 8) | |
| `TestSendAdvertUsesParentPrimaryV4Source` | `internal/plugins/vrrp/transport/transport_test.go` | Seam-injected addresses: first IPv4 of the parent unit chosen; re-resolved after an address-change event and on UpdateAdvert | |
| `TestSendAdvertNoLinkLocalSkipsAndCounts` | `internal/plugins/vrrp/transport/transport_test.go` | AC-10: skip + {reason=no-link-local}; retry succeeds once link-local appears | |
| `TestReadLoopDeliversRxMeta` | `internal/plugins/vrrp/transport/transport_test.go` | Fake handle rx -> engine channel carries Src/Dst/TTL/Family/IfIndex + payload; TTL 254 passes through unmodified | |
| `TestRxOverflowDropsAndCounts` | `internal/plugins/vrrp/transport/transport_test.go` | AC-7: full channel -> drop + counter; readLoop never blocks | |
| `TestCloseStopsGoroutines` | `internal/plugins/vrrp/transport/transport_test.go` | AC-14: Close joins readLoop + announcer; no goroutine leak | |
| `TestTransportMetricsRegistered` | `internal/plugins/vrrp/transport/metrics_test.go` | Five series with exact names/labels on a recording registry; NopRegistry default | |
| `TestCounterSnapshotAndReset` | `internal/plugins/vrrp/transport/metrics_test.go` | CounterSnapshot read-back matches increments per instance; ResetCounters zeroes only that instance (Finding 7) | |
| `TestVRRPRawSocketDoctorWarn` | `internal/plugins/vrrp/transport/doctor_test.go` | vrrp configured + probe false -> one SeverityWarning with code `doctor-vrrp-raw-socket` | |
| `TestVRRPRawSocketDoctorSilent` | `internal/plugins/vrrp/transport/doctor_test.go` | No vrrp group in the tree -> no diagnostics regardless of probe | |
| `TestVRRPRawSocketCodeRegistered` | `internal/plugins/vrrp/transport/doctor_test.go` | `diagnostic.Lookup("doctor-vrrp-raw-socket")` returns CodeMeta with title/description (isis `doctor_test.go:73` model) | |
| `TestOpenInstanceUnsupportedPlatform` | `internal/plugins/vrrp/transport/backend_other_test.go` (`//go:build !linux`) | AC-13: typed error, no panic | |

QEMU integration tests (`//go:build integration && linux`, file `internal/plugins/vrrp/transport/transport_integration_linux_test.go`, veth pair + netns via the `withNetNS` pattern of `internal/component/iface/integration_helpers_linux_test.go:36`; the test creates its parent veth and a macvlan on it directly, independent of spec-vrrp-3):

| Test | Validates | Status |
|------|-----------|--------|
| `TestIntegrationOpenInstanceSocketOptions` | AC-1/AC-2 via getsockopt read-back: SO_BINDTODEVICE names, IP_HDRINCL=1, IPV6_MULTICAST_HOPS=255, IPV6_TCLASS=0xc0, IPV6_CHECKSUM=6, SO_RCVTIMEO=500ms | |
| `TestIntegrationMulticastJoinVisible` | 224.0.0.18 in `/proc/net/igmp` and ff02::12 in `/proc/net/igmp6` for the parent while open; gone after Close | |
| `TestIntegrationAdvertOnPeerVeth` | AC-3 + A-1/A-2/A-3: AF_PACKET capture on the peer veth end asserts L2 src MAC, TTL 255, TOS, src/dst IP, header checksum, exact payload bytes | |
| `TestIntegrationAdvertV6OnWire` | AC-4 + A-5 + R-7: hop limit 255, tclass, link-local src, kernel-filled VRRP checksum validates under pseudo-header | |
| `TestIntegrationGARPOnWire` | AC-8: captured frames byte-equal the golden frame; count and spacing observed | |
| `TestIntegrationNAOnWire` | AC-9 + A-4: captured NA byte-equal (checksum verified by software recomputation); hop limit 255 | |
| `TestIntegrationRxDeliversFromPeer` | AC-6 + A-1: peer netns sends proto-112 datagrams (TTL 255 and TTL 254); both delivered with correct RxMeta | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VIPs per announce burst | 1-16 | 16 (48 frames) | 0 VIPs -> 0 frames, no error | 17 never reaches transport (YANG max-elements 16, spec-vrrp-5); defensively announces all given |
| v4 advert datagram (header+payload) | 28-92 bytes | 92 (16 VIPs) | short encode rejected by spec-vrrp-1 | oversize rejected by spec-vrrp-1 |
| v6 advert payload | 24-264 bytes | 264 (16 VIPs incl. link-local first) | short encode rejected by spec-vrrp-1 | oversize rejected by spec-vrrp-1 |
| per-instance tx buffer | 512 bytes, sized FROM spec-vrrp-1's exported MaxLen constants (>= MaxLenV3v6 = 264, plus 60-byte IPv4 header headroom; Finding 17 -- not an independent choice) | holds 92 (v4) and 264 (v6) max | n/a (derived constant) | encode into larger-than-buffer fails closed with error, tested |
| rx buffer | 2048 bytes | 60B IPv4 header + 264 payload fits | n/a (constant) | jumbo datagram truncates at Recvfrom; codec length check discards + counts |
| engine rx channel | 0-256 queued | 256 | n/a | 257th dropped + {reason=rx-overflow} |
| announceRepeatCount | 3 (constant) | burst emits exactly 3 per VIP | n/a | n/a |
| announceRepeatInterval | 100ms (constant) | spacing asserted via timing seam | n/a | n/a |
| SO_RCVTIMEO | 500ms (constant) | Close observed within 2x timeout in `TestCloseStopsGoroutines` | n/a | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| vrrp-instance-up | `test/vrrp/vrrp-instance-up.ci` (spec-vrrp-5, needs-linux) | Commit a v4 group: sockets open, adverts visible, exercising OpenInstance/SendAdvert end-to-end | |
| vrrp-backup-hold | `test/vrrp/vrrp-backup-hold.ci` (spec-vrrp-6, needs-linux/QEMU) | Backup stays held by received adverts flowing through this rx path (hold-only; ~~failover emits GARP/NA~~ moved to vrrp-failover.ci, Finding 13) | |
| vrrp-failover | `test/vrrp/vrrp-failover.ci` (spec-vrrp-6, needs-linux/QEMU) | Backup promotes on master loss; this transport's GARP/NA burst repoints the LAN | |
| vrrp-doctor | `test/vrrp/vrrp-doctor.ci` (spec-vrrp-5) | `ze doctor` surfaces `doctor-vrrp-raw-socket` through the user entry point | |
| vrrp-metrics | `test/vrrp/vrrp-metrics.ci` (spec-vrrp-5) | Transport counters visible via the metrics scrape | |

The `.ci` files are owned and created by spec-vrrp-5/spec-vrrp-6 (the transport has no user-facing entry point of its own; the engine is wired there). This spec's own executable coverage is the Go unit + QEMU integration suite above; the rows here bind this transport into the umbrella's functional surface.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| v3 election + failover + preempt | `test/interop/scenarios/vrrp-v3-keepalived/` (spec-vrrp-6) | keepalived | keepalived accepts this transport's adverts (MAC/TTL/checksum/source) and hosts follow its GARP/NA | |
| v2 election | `test/interop/scenarios/vrrp-v2-keepalived/` (spec-vrrp-6) | keepalived (default v2) | Same IPv4 transport carries v2 payloads interoperably | |

Scenario implementation is spec-vrrp-6's deliverable; listed here because they are the final proof for this spec's wire artifacts (umbrella R-5).

### Future (if deferring any tests)
- None deferred. (Configurable burst counts, if added by spec-vrrp-5, bring their own YANG boundary tests there.)

## Files to Modify
- `internal/core/diagnostic/codes.go` - add CodeMeta row `doctor-vrrp-raw-socket` (title, description naming CAP_NET_RAW and proto 112, `ze explain` examples) next to the isis/ospf raw-socket rows (:361, :367)

No change: `mk/test-integration.mk` -- `ZE_QEMU_INTEGRATION_PKGS` (:314-319) derives the QEMU package list from `^//go:build integration && linux` build tags, so `./internal/plugins/vrrp/transport` is included automatically once `transport_integration_linux_test.go` exists. Verified against the derivation grep during implementation (A-8).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | Transport has no config surface; all knobs internal constants for now (spec-vrrp-5 owns YANG) |
| YANG validation constraints | No | n/a here (spec-vrrp-5) |
| YANG custom validators | No | n/a here (spec-vrrp-5) |
| CLI commands/flags | No | No CLI surface in the transport |
| CLI grammar (action before identifier) | No | n/a |
| Editor autocomplete | No | n/a |
| Functional test for new RPC/API | Yes | Bound via `test/vrrp/*.ci` rows above (owned by spec-vrrp-5/6); Go integration tests here |
| Pipe completeness | No | No command output produced here |
| Env var registration | No | No environment/ leaves; all tunables are constants or future YANG (config-surface rule) |
| Doctor check for runtime dependencies | Yes | `internal/plugins/vrrp/transport/register.go` + `doctor.go` + `doctor_linux.go` + `doctor_other.go`; CodeMeta in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | `internal/plugins/vrrp/transport/metrics.go`; series listed in the Metrics table below |

Metrics (transport-owned; exact names/labels; isis `metrics.go:23` ownership model):

| Series | Type | Labels | Incremented by |
|--------|------|--------|----------------|
| `ze_vrrp_adverts_sent_total` | CounterVec | interface, vrid, family | SendAdvert success |
| `ze_vrrp_adverts_received_total` | CounterVec | interface, vrid, family | readLoop delivery to the engine channel |
| `ze_vrrp_packet_errors_total` | CounterVec | interface, vrid, family, reason | transport reasons: rx-overflow, malformed-ipv4, send-error, garp-send-error, na-send-error, no-link-local; PLUS spec-vrrp-1 validation taxonomy reasons incremented by the engine through the transport's RecordRxError hook (single owner for the series, no holo-bug-9 dead counters) |
| `ze_vrrp_announcements_sent_total` | CounterVec | interface, vrid, family, kind (garp or na) | announcer per frame sent |
| `ze_vrrp_sockets_open` | Gauge | (none) | OpenInstance / CloseInstance |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | User-visible VRRP feature docs land with spec-vrrp-5 (umbrella rows) |
| 2 | Config syntax changed? | No | No config surface here |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Plugin registration is spec-vrrp-5 |
| 6 | Has a user guide page? | No | `docs/guide/vrrp.md` is spec-vrrp-5's deliverable |
| 7 | Wire format changed? | No | VRRP is RFC-defined; summaries exist |
| 8 | Plugin SDK/protocol changed? | No | No SDK changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No (deferred to spec-vrrp-5) | The `docs/features/rfc-status.md` rows for 9568/3768 land once the full feature is user-reachable (umbrella checklist row 9); transport-level RFC comments are still added in code here |
| 10 | Test infrastructure changed? | No | No new runner/target; QEMU package list is tag-derived |
| 11 | Affects daemon comparison? | No | spec-vrrp-5 owns `docs/comparison.md` |
| 12 | Internal architecture changed? | No | New leaf package following existing transport pattern; grep `docs/` for transport anchors during implementation to confirm |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (or the monitoring guide the umbrella row 14 selects): the five `ze_vrrp_*` transport series + labels |
| 15 | Registered plugin/event/command inventory changed? | No | No plugin/event/command registered here (doctor check is not inventory-listed; verify by grep during implementation) |
| 16 | Changed source files referenced by doc source anchors? | Yes (check) | Grep `docs/` for `internal/core/diagnostic/codes.go` anchors; update if any claim enumerates builtin codes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | None exist for vrrp yet |

## Files to Create
- `internal/plugins/vrrp/transport/transport.go` - orchestrator: Backend/InstanceHandle interfaces, RxMeta, instance registry, bounded delivery channel, SetLogger/SetMetrics, iface seam vars, RecordRxError hook
- `internal/plugins/vrrp/transport/backend_linux.go` - `//go:build linux`: socket set open (rx parent / tx macvlan), options, joins, IPv4 header build (IP_HDRINCL), readLoop, sends
- `internal/plugins/vrrp/transport/backend_other.go` - `//go:build !linux`: typed unsupported-platform error (ospf `backend_other.go:9` model)
- `internal/plugins/vrrp/transport/garp.go` - pure GARP frame builder (testable on darwin)
- `internal/plugins/vrrp/transport/garp_linux.go` - AF_PACKET ETH_P_ARP sender on the macvlan (isis `Send` :135 model)
- `internal/plugins/vrrp/transport/na.go` - pure unsolicited-NA message builder (testable on darwin)
- `internal/plugins/vrrp/transport/na_linux.go` - raw ICMPv6 sender on the macvlan (IPV6_PKTINFO cmsg, IPV6_MULTICAST_HOPS 255)
- `internal/plugins/vrrp/transport/announce.go` - per-instance announcer worker (burst queue, repeat count/interval constants, injectable timing seam)
- `internal/plugins/vrrp/transport/register.go` - init(): `diagnostic.RegisterDoctorCheck` for `doctor-vrrp-raw-socket` (isis `register.go:20` model)
- `internal/plugins/vrrp/transport/doctor.go` - check function gated on vrrp groups present in the config tree; probe seam var
- `internal/plugins/vrrp/transport/doctor_linux.go` - probe: open + close AF_INET/SOCK_RAW proto 112 (the exact rx capability)
- `internal/plugins/vrrp/transport/doctor_other.go` - probe stub returning true (isis `doctor_other.go:12` model)
- `internal/plugins/vrrp/transport/metrics.go` - the five `ze_vrrp_*` series + NopRegistry fallback (isis `metrics.go:23,49` model)
- `internal/plugins/vrrp/transport/garp_test.go`, `na_test.go`, `announce_test.go`, `transport_test.go`, `doctor_test.go`, `metrics_test.go`, `backend_other_test.go` - unit suite (TDD plan above)
- `internal/plugins/vrrp/transport/transport_integration_linux_test.go` - `//go:build integration && linux` QEMU suite (TDD plan above)

API shape (prose, for the implementer; final signatures at implementation):

| Element | Shape |
|---------|-------|
| InstanceSpec | family (v4/v6), vrid, parent logical name, macvlan OS device name, virtual MAC |
| Backend | OpenInstance(spec) returning an InstanceHandle or error; linux vs stub |
| InstanceHandle | SendAdvert(prepared bytes / v6 cmsg params), SendAnnounce(frame bytes), Recv() channel of raw rx items, Close() |
| Transport (orchestrator) | OpenInstance, CloseInstance, UpdateAdvert(params), SendAdvert, AnnounceMaster(vips), Receive() bounded (RxMeta, payload) channel, RecordRxError(instance, reason), CounterSnapshot(instance), ResetCounters(instance), SetMetrics, SetLogger, Close |
| RxMeta | Src, Dst (netip.Addr); TTL uint8 (hop-limit for v6); Family uint8 (spec-vrrp-1 Decode derives address size from it, orchestrator D-A); IfIndex int |
| CounterSnapshot | Per-instance read-back for spec-vrrp-5 `show vrrp statistics` / `clear vrrp statistics` (Finding 7): adverts sent/received, packet errors by reason (including checksum-rfc5798-compat from the spec-vrrp-1 taxonomy), announcements sent by kind; ResetCounters zeroes one instance's counters. Prio-0 sent/received counters are ENGINE-owned per D-F (engine counts post-Decode for rx and on SendAdvertZeroPriority execution for tx; the transport never parses payloads) |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-vrrp-0-umbrella.md` + spec-vrrp-1 (codec contract) |
| 2. Audit | Files to Modify/Create + TDD Test Plan -- check what exists; validate A-6 (codec surface) and A-8 (QEMU derivation) first |
| 3. Wiring phase | Wiring Test table -- package skeleton, interfaces, doctor registration, failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-qemu-integration-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section -- loop until 0 BLOCKER / 0 ISSUE |
| 14. Present summary + close | Executive Summary; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- package skeleton with Backend/InstanceHandle/Transport/RxMeta, `register.go` doctor registration + CodeMeta row in `codes.go`, `metrics.go` with NopRegistry, `backend_other.go` stub
   - Tests: `TestTransportOpenInstanceWiring`, `TestVRRPRawSocketCodeRegistered`, `TestOpenInstanceUnsupportedPlatform`, `TestTransportMetricsRegistered` (all failing or trivially passing against stubs)
   - Files: transport.go, register.go, doctor.go, doctor_other.go, metrics.go, backend_other.go, codes.go
   - Verify: entry points exist; wiring test fails because the linux backend is a stub
2. **Phase: Frame builders (pure)** -- GARP frame and NA message builders into caller buffers
   - Tests: `TestBuildGARPFrameGolden`, `TestBuildGARPFramePerVIP`, `TestBuildNAMessageGolden`, `TestNAHoloBugNegatives`
   - Files: garp.go, na.go
   - Verify: golden bytes match the layouts below on darwin
3. **Phase: Linux backend rx/tx** -- socket set open with full option matrix, joins, IPv4 header builder, readLoop with RxMeta extraction, drop-on-overflow delivery
   - Tests: `TestReadLoopDeliversRxMeta`, `TestRxOverflowDropsAndCounts`, `TestCloseStopsGoroutines`; integration: `TestIntegrationOpenInstanceSocketOptions`, `TestIntegrationMulticastJoinVisible`, `TestIntegrationRxDeliversFromPeer`
   - Files: backend_linux.go
   - Verify: unit tests via fake handles pass; QEMU suite exercises the real kernel
4. **Phase: Advert send path** -- per-instance buffer, UpdateAdvert re-encode via spec-vrrp-1 WriteTo, source resolution (v4 parent primary, v6 macvlan link-local), skip-and-count without link-local
   - Tests: `TestUpdateAdvertReencodes`, `TestSendAdvertUsesParentPrimaryV4Source`, `TestSendAdvertNoLinkLocalSkipsAndCounts`; integration: `TestIntegrationAdvertOnPeerVeth`, `TestIntegrationAdvertV6OnWire`
   - Files: transport.go, backend_linux.go
   - Verify: AC-3/4/5/10 pass; A-1/A-2/A-3/A-5 flip to confirmed or broken
5. **Phase: Announcers** -- announcer worker, GARP/NA senders, burst semantics
   - Tests: `TestAnnounceBurstRepeatsAndSpacing`, `TestAnnounceMasterBurstWiring`; integration: `TestIntegrationGARPOnWire`, `TestIntegrationNAOnWire`
   - Files: announce.go, garp_linux.go, na_linux.go
   - Verify: AC-8/9 pass; A-4 resolved
6. **Phase: Doctor probe (linux)** -- probe + check gating
   - Tests: `TestVRRPRawSocketDoctorWarn`, `TestVRRPRawSocketDoctorSilent`
   - Files: doctor_linux.go, doctor.go
   - Verify: AC-11; mechanical check `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered'`
7. **Functional tests** -- confirm the spec-vrrp-5/6 `.ci` rows exercise this transport once those specs land; QEMU package auto-inclusion verified (A-8)
8. **RFC refs** -- add RFC comments per the RFC Documentation section
9. **Full verification** -- `make ze-verify` + `make ze-qemu-integration-test`
10. **Complete spec** -- audit tables, learned summary `plan/learned/NNN-vrrp-4-transport.md`, two commits (A: code+tests+spec+summary; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-14 has implementation with file:line |
| Feature completeness | Every End-to-End User Story path is closed; GARP AND NA both implemented (uvrrpd/holo each botch one) |
| Correctness | Socket options match AC-1/AC-2 exactly; TTL/hop-limit 255 and TOS/tclass 0xc0 on every tx socket; RxMeta TTL delivered unmodified; no VRRP payload inspection in the transport |
| Naming | Metrics exactly `ze_vrrp_*` as tabled; diagnostic code `doctor-vrrp-raw-socket`; log subsystem `vrrp/transport` prefix on messages |
| Data flow | Frames only via this package's sockets; interface lookups only via iface seams; devices consumed by name, never created |
| CLI grammar | n/a -- no CLI commands added |
| Registration over hardcoding | Doctor check registered via `diagnostic.RegisterDoctorCheck` from the owning package and discovered by the runner; metrics registered on the injected registry; no vrrp switch/case/factory added to any core or shared package |
| Doctor checks | `doctor-vrrp-raw-socket` registered, gated on vrrp config presence, explainable via `ze explain`, unit + functional tested |
| YANG validation | n/a -- no YANG leaves here |
| Prometheus counters | All five series registered AND incremented by real paths (grep every metric field for a non-test increment site -- holo bug 9 guard) |
| Rule: qemu-testing | Every `//go:build linux` file covered by the integration suite; package present in the derived QEMU list |
| Rule: goroutine-lifecycle | Exactly two long-lived goroutines per instance (readLoop, announcer); no `go func()` in rx or send paths |
| Rule: buffer-first | No per-packet allocations beyond the single rx payload copy; encode/build into reusable buffers |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Transport package with linux + stub backends | `go build ./internal/plugins/vrrp/transport` on darwin AND `GOOS=linux go build` |
| Golden GARP/NA builders | `go test ./internal/plugins/vrrp/transport -run 'TestBuildGARPFrameGolden|TestBuildNAMessageGolden'` |
| QEMU integration suite green | `make ze-qemu-integration-test` output includes the vrrp transport package |
| Doctor check live | `go test ./internal/component/doctor -run TestDoctorCoverageCodesRegistered` + `ze explain doctor-vrrp-raw-socket` |
| Metrics series | `grep -n 'ze_vrrp_' internal/plugins/vrrp/transport/metrics.go` matches the table; increment sites grep-verified |
| No stale-advert path | `go test ./internal/plugins/vrrp/transport -run TestUpdateAdvertReencodes` |
| QEMU package auto-derivation (A-8) | grep of `mk/test-integration.mk` derivation against the new file; `make -n ze-qemu-integration-test` shows the package |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Rx path: IHL-aware strip (spec-vrrp-1) bounds-checked before slicing; truncated datagrams dropped + counted; RxMeta never derived from unvalidated offsets |
| Spoofing resistance | TTL/hop-limit captured faithfully so downstream GTSM (discard != 255) works; transport itself never trusts payload content |
| Resource exhaustion | Advert floods: O(1) per datagram, bounded channel with drop+count, no per-packet goroutines/allocations; burst queue bounded so repeated Master flaps cannot pile unbounded announce jobs |
| Privilege | CAP_NET_RAW required; failure paths return actionable errors naming the capability; doctor check surfaces it pre-start; no privilege state cached across Close |
| State manipulation | Announcers only fire on explicit engine calls (never on rx input), so a forged advert cannot make this node emit GARP/NA |
| Error leakage | Log lines carry interface/vrid/reason, never raw packet hex at info level |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read the cited producer (Current Behavior) or `rfc/short/` section -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| QEMU test flaky | Payload-predicate waits with deadlines (R-6); never add sleeps |
| Kernel behavior contradicts A-1..A-5 | Mark assumption broken, Mistake Log row, apply the recorded fallback, present if design-invalidating |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A `netip.Addr{}` (unresolved v4 src) could be passed to `As4()` | `As4()` panics on a non-IPv4 zero Addr | reasoning about the DAD/unresolved path before writing `encodeLocked` | guarded with `Is4()`; FillChecksum tolerates a zero src (message-only path) |
| Spec Integration Points implied the transport runs its own `iface.Subscribe` reader for address events | goroutine-lifecycle mandates exactly two long-lived goroutines per instance | reconciling the two rules | address re-resolution moved to `UpdateAdvert` + `RefreshParentAddresses(key)` called by the engine; seam retained (deviation, documented) |
| Any listed macvlan link-local is a usable IPV6_PKTINFO source | a TENTATIVE (IFA_F_TENTATIVE, DAD-running) link-local makes rawv6 sendmsg fail EINVAL (`ip6_datagram_send_ctl` requires a non-tentative assigned source) | QEMU run: `TestIntegrationAdvertV6OnWire` "sendmsg: invalid argument" | `macvlanLinkLocal` now resolves via the iface seam and skips `Tentative` addresses (the `AddrInfo.Tentative` field exists for exactly this, OSPFv3 precedent); residual EINVAL invalidates the cache and maps to no-link-local (skip+count+retry, AC-10) |
| Link-local resolution can run on any goroutine | netlink queries land in the calling THREAD's netns; the announcer worker's thread is not the netns-entered test thread, so lazy resolution saw the wrong namespace and the NA never sent | QEMU run: `TestIntegrationNAOnWire` "no matching frame within 3s" (no EINVAL: device lookup failed first) | `AnnounceMaster` warms the source cache on the ENGINE's goroutine via `WarmV6Source` (v6SourceWarmer); the worker only transmits. Harmless in single-netns production, correct in netns tests |
| `/proc/net/igmp` shows the current thread's netns | `/proc/net` -> `/proc/self/net` = the thread-group LEADER's netns; setns(2) is per-thread, so the test read the ORIGINAL namespace (the join itself worked -- `TestIntegrationRxDeliversFromPeer` received 224.0.0.18) | QEMU run: `TestIntegrationMulticastJoinVisible` "not in /proc/net/igmp" | test reads `/proc/thread-self/net/igmp`; also added the spec's gone-after-Close assertion |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Move `TestCounterSnapshotAndReset` from metrics_test.go into a new counters_test.go | test-weakening hook blocks deleting a Test func | left the test in metrics_test.go; counters.go carries a non-blocking test-first notice only |
| Per-instance fan-in rxLoop forwarding handle.Recv() into the engine channel (isis model) | would be a third goroutine per instance | the backend readLoop delivers directly through `rxSink` (drop-on-overflow), keeping the two-goroutine budget; the pure `rxSink.deliver` helper is unit-tested |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| n/a | - | - | no recurring pattern; hook friction (empty `default:`, stale `// Related:`, TDD test-first, >600-line files) was expected and handled inline |

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- `ZE_QEMU_INTEGRATION_PKGS` is derived from build tags (`mk/test-integration.mk:314-319`), so new integration-tagged packages self-register with the QEMU target; no Makefile bookkeeping.
- ospf's `StripIPv4Header` (`transport.go:525`) discards TTL, which is fine for OSPF but disqualifies it for VRRP GTSM; spec-vrrp-1's helper must surface TTL, which is why the helper lives in the codec (validation input), not here.
- The two-goroutine budget (readLoop + announcer) forces the drop-on-overflow rx delivery into the backend readLoop itself (no orchestrator fan-in goroutine). Extracting the non-blocking send + counting into a pure `rxSink.deliver` keeps AC-6/AC-7 unit-testable on darwin while the real socket read is only integration-tested.
- v4 and v6 tx checksums diverge by design: v4 VRRP checksum is software-filled (`packet.FillChecksum`, message-only) because IP_HDRINCL only offloads the IP-header checksum; v6 is kernel-offloaded via `IPV6_CHECKSUM=6`. Both keep the tx hot path allocation-free (encode into a reusable per-instance buffer).
- IPv6 raw sockets deliver no IP header, so v6 RxMeta is built from ancillary data (`IPV6_RECVHOPLIMIT`+`IPV6_RECVPKTINFO`) plus the Recvmsg sender address, whereas v4 raw sockets include the header and use the codec's IHL-aware strip. Two readLoops (`readLoopV4`/`readLoopV6`), one delivery path.
- The IPV6_PKTINFO send cmsg has no `x/sys/unix` builder in the repo; the arch-correct idiom is an `unsafe` overlay of `unix.Cmsghdr` + `unix.Inet6Pktinfo` with `SetLen(CmsgLen(...))` (mirrors the accepted `//nolint:gosec` pattern in `internal/core/smart/smart_linux.go`).
- The doctor gate cannot use a top-level `GetContainer("vrrp")` (isis/ospf model) because VRRP nests under `interface ... unit ... ipv4/ipv6 vrrp`; walking `CollectContainerPaths` for a final `vrrp` segment is robust to whatever nesting spec-vrrp-5's YANG augment lands.

## Core Insight

VRRP transmit identity is split across two devices: the L2 identity (virtual MAC)
comes from binding the tx socket to the macvlan, while the L3 identity (primary
IPv4 of the "sending interface", RFC 9568 sec 7.2) belongs to the parent. No
kernel default supplies both at once, which is exactly why IPv4 tx needs
IP_HDRINCL with an explicit source and IPv6 tx needs an IPV6_PKTINFO cmsg: each
send must pin the half of the identity the socket binding does not provide.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| IPv4 tx uses IP_HDRINCL with an explicitly built IPv4 header (TTL 255, TOS 0xc0, proto 112, dst 224.0.0.18, src = parent unit primary IPv4; checksum/len/id zeroed for the kernel, A-3) | Plain raw socket + IP_TTL/IP_MULTICAST_TTL/IP_TOS/IP_MULTICAST_IF options, kernel-selected source | The tx socket is bound to the macvlan, which has NO stable IPv4 address: kernel source selection would fail when Backup (no address at all) or pick a VIP when Master, violating RFC 9568 sec 7.2 (source MUST be the interface primary IPv4) except in the owner case. holo ships exactly this IP_HDRINCL model in production. The option-based alternative stays viable only if the macvlan ever carries a dedicated IPv4 address, which the umbrella design forbids |
| IPv4 source = parent unit's FIRST configured IPv4 address ("primary", ospf `interfaceIPv4` precedent), resolved at OpenInstance, re-resolved on every UpdateAdvert and on parent address-change events | Engine passes a fixed source once; resolve per send | Matches RFC "primary IPv4 address of the sending interface" with a deterministic, documented rule; re-resolution points bound staleness without per-send lookup cost |
| IPv6 advert source = macvlan link-local, pinned per send via IPV6_PKTINFO cmsg (src + macvlan ifindex); skip-and-count when absent | Parent link-local as source | RFC 9568 sec 7.2 source is the sending interface's link-local and the sending interface is the macvlan (frames must carry the virtual MAC); holo model; parent link-local would advertise a MAC/IP identity mismatch |
| v6 VRRP checksum: kernel via IPV6_CHECKSUM offset 6 on the TX socket only; RX socket does NOT set IPV6_CHECKSUM -- spec-vrrp-1 validates in software and the error is counted | Set IPV6_CHECKSUM on rx too (kernel silently discards bad checksums) | Kernel rx discard would zero the checksum-error counter forever (holo bug 9 dead-counter class) and hide attacks/misconfig from `SHOULD log` (RFC 9568 sec 7.1); tx offload keeps the hot path allocation-free while the codec remains the single validation authority |
| NA checksum by the kernel (raw IPPROTO_ICMPV6 sockets always compute it, RFC 3542 sec 3.1, A-4); golden tests verify bytes via software pseudo-header recomputation | Software checksum + IPV6_CHECKSUM management | Kernel behavior is mandatory for ICMPv6 raw sockets (the option is rejected on them); fighting it adds code for no control |
| Unsolicited NA source = macvlan link-local | VIP as source (uvrrpd) | Uniform with advert identity and valid per RFC 4861 sec 7.2.6 (an address of the sending interface -- VIPs are installed on the macvlan in Master, link-local is present in both directions of the transition); avoids ordering coupling with VIP install |
| GARP/NA burst: 3 repeats per VIP, 100ms spacing, internal constants | Single frame (uvrrpd); configurable knob now | keepalived's garp repeat precedent shows single frames get lost exactly when failover storms the LAN; 3x100ms is cheap insurance; ~~a YANG knob is deferred to spec-vrrp-5 only if trivial there~~ no knob in this umbrella (Finding 18; recorded in umbrella Known Limitations) |
| Per-instance socket set (rx parent + tx macvlan + announce macvlan) | Shared per-parent rx socket with vrid demux | Umbrella A-3/holo split; per-instance keeps lifecycle trivial (Close = close 3 fds) and isolates failures; the kernel fan-out cost is negligible at VRRP rates (R-3 records the shared-socket fallback) |
| Rx delivery: bounded channel (256), drop + count on overflow | Blocking send (ospf `deliverDatagram:227`) | A blocked readLoop stops draining the kernel buffer for ALL traffic on that socket; VRRP correctness is timer-driven and a counted drop under flood is strictly safer than a stalled socket (R-4) |
| Frame builders (GARP/NA) are pure, platform-independent files | Build frames inline in linux senders | Golden-byte tests must run on darwin (`make ze-verify` native); mirrors the codec/transport split |
| Metrics owned by the transport, engine increments validation reasons via a RecordRxError hook | Split the series between engine and transport registries | One owner per series (isis model); reason taxonomy stays consistent between transport-level and codec-level errors |

## Known Limitations
- GARP/NA burst count and spacing are internal constants (3 x 100ms); ~~a config knob is deferred to spec-vrrp-5 (trivial YANG addition if wanted)~~ decided (cross-review Finding 18): NO config knob anywhere in this umbrella; the constants stay internal and the umbrella Known Limitations records this.
- N instances on one parent each receive every proto-112 datagram on that parent (kernel raw-socket fan-out); acceptable at VRRP rates, shared-rx fallback recorded in R-3.
- IPv6 sends are skipped (and counted) during the macvlan link-local DAD window right after device creation; the FSM's next timer tick retries, so worst case is one advert interval of silence at instance birth.
- No VRRP unicast peers (umbrella decision); the transport only implements the multicast paths.
- The transport does not filter rx by TTL/checksum/vrid -- by design, validation and its per-reason counting live in spec-vrrp-1/engine so the SHOULD-log/count rules are honored in one place.

## RFC Documentation

Add `// RFC 9568 Section X.Y: "<quoted requirement>"` (or RFC 3768) above enforcing code:

| Code site | RFC comment |
|-----------|-------------|
| IPv4 header builder TTL field | RFC 9568 sec 5.1.1.3 / RFC 3768 sec 5.2.3: TTL MUST be 255 |
| v6 tx socket IPV6_MULTICAST_HOPS | RFC 9568 sec 5.1.2.3: Hop Limit MUST be 255 |
| tx socket binding to macvlan | RFC 9568 sec 7.2: source MAC MUST be the Virtual Router MAC |
| IPv4 source resolution | RFC 9568 sec 7.2 / RFC 3768 sec 7.2: source IP = interface primary IPv4 |
| v6 PKTINFO source pinning | RFC 9568 sec 7.2: source = link-local of the sending interface |
| destination constants | RFC 9568 Constants / RFC 3768 sec 5.2.2: 224.0.0.18 and ff02::12, proto 112 |
| GARP builder | RFC 9568 errata 7947/7949: gratuitous ARP contains the virtual IPv4 address; target link-layer = Virtual Router MAC |
| NA builder flags | RFC 9568 sec 6.4.1/6.4.2: unsolicited NA with R set, S clear, O set, TLL = Virtual Router MAC |
| NA hop limit | RFC 4861 sec 7.1.2: ND messages require Hop Limit 255 |
| announce gating (device + MAC present before frames) | RFC 9568 sec 8.1.2 / 8.2.2: delay GARP/ND until address and Virtual Router MAC are configured |

Byte-layout references for the golden tests (vrid 10, virtual MACs 00:00:5e:00:01:0a / 00:00:5e:00:02:0a):

GARP frame, VIP 192.0.2.1 (42 bytes):

```
ff ff ff ff ff ff   eth dst = broadcast
00 00 5e 00 01 0a   eth src = virtual MAC
08 06               ethertype ARP
00 01 08 00 06 04   htype 1, ptype 0x0800, hlen 6, plen 4
00 01               oper 1 (request)
00 00 5e 00 01 0a   sha = virtual MAC
c0 00 02 01         spa = 192.0.2.1 (VIP)
00 00 5e 00 01 0a   tha = virtual MAC (D-E: errata 7947/7949; supersedes earlier zero tha)
c0 00 02 01         tpa = 192.0.2.1 (VIP)
```

Unsolicited NA ICMPv6 message, VIP 2001:db8::1 (32 bytes; IPv6 header: src = macvlan
link-local, dst ff02::1, next header 58, hop limit 255):

```
88 00               type 136 (NA), code 0
XX XX               checksum (kernel-computed; golden test recomputes)
a0 00 00 00         flags: R=1 S=0 O=1
20 01 0d b8 00 00 00 00
00 00 00 00 00 00 00 01   target = 2001:db8::1 (VIP)
02 01               option: Target Link-Layer Address, len 1
00 00 5e 00 02 0a   virtual MAC
```

## Implementation Summary

### What Was Implemented
- Package `internal/plugins/vrrp/transport` (22 files, ~1345 non-test + ~1867 test lines): orchestrator (`transport.go`), per-instance counter snapshot (`counters.go`), pure GARP/NA frame builders (`garp.go`/`na.go`), announcer worker (`announce.go`), transport-owned metrics (`metrics.go`), doctor check (`doctor.go` + `doctor_linux.go`/`doctor_other.go` + `register.go`), non-Linux stub (`backend_other.go`), Linux backend (`backend_linux.go` + `garp_linux.go`/`na_linux.go`).
- Orchestrator API for the engine (spec-vrrp-5): `New`, `SetMetrics`, `SetLogger`, `OpenInstance(InstanceSpec) (InstanceKey, error)`, `UpdateAdvert(key, AdvertParams)`, `SendAdvert(key)`, `AnnounceMaster(key, vips)`, `Receive() <-chan RxItem`, `RecordRxError(key, reason)`, `CounterSnapshot(key)`, `ResetCounters(key)`, `RefreshParentAddresses(key)`, `CloseInstance(key)`, `Close`.
- Per-instance socket set (Linux): rx raw proto-112 on parent (SO_BINDTODEVICE + ifindex multicast join + SO_RCVTIMEO; v6 adds IPV6_RECVPKTINFO/RECVHOPLIMIT); tx raw proto-112 on macvlan (v4 IP_HDRINCL + IP_MULTICAST_LOOP 0; v6 IPV6_MULTICAST_HOPS 255 + IPV6_TCLASS 0xc0 + IPV6_MULTICAST_LOOP 0 + IPV6_CHECKSUM 6); announce socket on macvlan (v4 AF_PACKET ETH_P_ARP; v6 raw ICMPv6 + IPV6_MULTICAST_HOPS 255).
- GARP frame (tha = virtual MAC per D-E), unsolicited NA (R=1 S=0 O=1 + TLL option), 3x100ms announce bursts via internal constants, bounded(256) rx channel drop+count, `CounterSnapshot`/`ResetCounters` per-instance read-back, `doctor-vrrp-raw-socket` check + CodeMeta row, five `ze_vrrp_*` metric series, `backend_other.go` darwin stub.
- Full unit suite (darwin-runnable: golden bytes, counters, announce, wiring, rx/overflow/close, doctor, stub) + `//go:build integration && linux` QEMU suite (socket options, multicast join, v4/v6 advert on-wire, GARP/NA on-wire, rx-from-peer).

### Bugs Found/Fixed
- No bugs in the consumed sibling (`vrrp/packet`); the contract from its implementer held (FillChecksum family inference from src.Is6, StripIPv4Header leaving IfIndex=0 for the transport to fill, WriteTo assuming a validated Advertisement). The transport calls `adv.Validate()` inside `encodeLocked` before `WriteTo` per that contract.
- Guarded `inst.v4Src.As4()` behind `Is4()` because a zero `netip.Addr` panics on `As4()`; FillChecksum with a zero/absent v4 src still selects the message-only path (no panic) since it only touches src when `src.Is6()`.
- **QEMU-found kernel bug 1 (tentative pktinfo source)**: the v6 advert send used the macvlan link-local while still IFA_F_TENTATIVE (DAD window), which rawv6 sendmsg rejects with EINVAL. Fixed: `macvlanLinkLocal` resolves through the iface seam and skips `Tentative` (and non-`LinkLocal`) addresses; a residual sendmsg EINVAL (source removed/DAD-cycled between resolve and send) invalidates the cache and is counted {reason=no-link-local} with natural retry (`backend_linux.go` SendAdvert, `na_linux.go` sendNALocked).
- **QEMU-found kernel bug 2 (thread-netns resolution)**: the announcer worker resolved the link-local lazily on its own goroutine; netlink queries bind to the calling thread's netns, so in the netns-scoped test the device was invisible and NAs never reached the wire. Fixed: `AnnounceMaster` warms the source cache on the engine's goroutine (`WarmV6Source` / `v6SourceWarmer`); the worker only does socket I/O.
- **QEMU-found test bug (proc netns view)**: `/proc/net/igmp` is `/proc/self/net/...` = the thread-group leader's netns, not the setns'd test thread's; the IGMP join was real (rx test received 224.0.0.18) but observed in the wrong namespace. Fixed: read `/proc/thread-self/net/igmp`; also added the gone-after-Close assertion from the TDD table. Scoped QEMU rerun: `ok internal/plugins/vrrp/transport 9.456s`, `QEMU VM: PASS` (tmp/vrrp4-qemu-fix.log).

### Documentation Updates
- `internal/core/diagnostic/codes.go`: added the `doctor-vrrp-raw-socket` CodeMeta row (title + description naming CAP_NET_RAW and proto 112 + `ze explain` examples), next to the isis/ospf/ospfv3 raw-socket rows. No other central file touched.
- The five `ze_vrrp_*` series + labels are documented inline in `metrics.go`; the `docs/plugin-development/metrics.md` addition (checklist row 14) is owned by spec-vrrp-5 when the feature becomes user-reachable (was being edited by the parallel sibling session; left untouched to avoid a cross-session collision).

### Deviations from Plan
- **IPv4 header builder location**: `buildIPv4Header` lives in `transport.go` (orchestrator), not `backend_linux.go`. Rationale: the API is "SendAdvert(prepared bytes)", so the orchestrator prepares the full v4 datagram; keeping the builder platform-independent lets it be exercised on darwin. No behavior change.
- **counters.go split**: `instanceCounters` moved out of `transport.go` into `counters.go` to keep `transport.go` under the 600-line soft cap. Its unit coverage stays in `metrics_test.go` (`TestCounterSnapshotAndReset`); the test-relocation was blocked by the test-weakening hook, so the test was left in place rather than moved.
- **Parent address re-resolution without a third goroutine**: the goroutine-lifecycle rule mandates exactly two long-lived goroutines per instance (readLoop + announcer). A live `iface.Subscribe` reader would be a third, so address re-resolution is done synchronously in `UpdateAdvert` and via an explicit `RefreshParentAddresses(key)` method the engine (spec-vrrp-5, which already subscribes to iface events) calls on a parent address-change event. The `resolveIfaceAddresses` seam is retained and overridable (ospf model). See Mistake Log.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-instance socket set (rx parent / tx macvlan / announce macvlan) | Done | `backend_linux.go:openV4/openV6` | full option matrix |
| GARP sender (tha = virtual MAC, D-E) | Done | `garp.go`, `garp_linux.go` | golden bytes |
| Unsolicited NA (R=1 S=0 O=1 + TLL) | Done | `na.go`, `na_linux.go` | golden bytes |
| 3x100ms announce bursts, internal constants | Done | `announce.go` | injectable timing seam |
| Bounded(256) rx channel drop+count | Done | `transport.go:rxSink.deliver` | AC-7 |
| CounterSnapshot/ResetCounters API | Done | `counters.go`, `transport.go` | Finding 7 |
| doctor-vrrp-raw-socket check + CodeMeta | Done | `doctor*.go`, `register.go`, `codes.go` | |
| Five ze_vrrp_* metric series | Done | `metrics.go` | all incremented |
| backend_other.go darwin stub | Done | `backend_other.go` | typed error |
| QEMU integration tests for every linux file | Done | `transport_integration_linux_test.go` | A-8 auto-derived |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 v4 socket options | Done | `TestIntegrationOpenInstanceSocketOptions/v4` | getsockopt read-back |
| AC-2 v6 socket options | Done | `TestIntegrationOpenInstanceSocketOptions/v6` | HOPS/TCLASS/CHECKSUM |
| AC-3 v4 advert on wire | Done | `TestIntegrationAdvertOnPeerVeth` | MAC/TTL/TOS/src/dst |
| AC-4 v6 advert on wire | Done | `TestIntegrationAdvertV6OnWire` | hop limit/dst/src |
| AC-5 no stale advert | Done | `TestUpdateAdvertReencodes` | holo bug 8 negative |
| AC-6 rx meta (TTL preserved) | Done | `TestReadLoopDeliversRxMeta`, `TestIntegrationRxDeliversFromPeer` | TTL 254 passes through |
| AC-7 rx overflow drop+count | Done | `TestRxOverflowDropsAndCounts` | non-blocking |
| AC-8 GARP burst | Done | `TestBuildGARPFrameGolden`, `TestAnnounceBurstRepeatsAndSpacing`, `TestIntegrationGARPOnWire` | 3N frames |
| AC-9 NA burst | Done | `TestBuildNAMessageGolden`, `TestNAHoloBugNegatives`, `TestIntegrationNAOnWire` | quadruple negative |
| AC-10 v6 no-link-local skip | Done | `TestSendAdvertNoLinkLocalSkipsAndCounts` | retry succeeds |
| AC-11 doctor warn/silent/explain | Done | `TestVRRPRawSocketDoctorWarn/Silent`, `TestVRRPRawSocketCodeRegistered` | |
| AC-12 metrics registered + incremented | Done | `TestTransportMetricsRegistered`, `TestCounterSnapshotAndReset` | no dead counters |
| AC-13 darwin build + stub | Done | `TestOpenInstanceUnsupportedPlatform` (`//go:build !linux`) | typed error |
| AC-14 close, no goroutine leak | Done | `TestCloseStopsGoroutines` | -race |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 18 unit tests | Done | `*_test.go` | green on darwin, `-race` |
| All 7 QEMU integration tests | Written | `transport_integration_linux_test.go` | cross-vet green; executed by `ze-qemu-integration-test` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| transport.go, backend_linux.go, backend_other.go | Done | + counters.go (split for size) |
| garp.go, garp_linux.go, na.go, na_linux.go, announce.go | Done | |
| register.go, doctor.go, doctor_linux.go, doctor_other.go, metrics.go | Done | |
| all unit + integration test files | Done | |
| codes.go CodeMeta row | Done | only central touch |
| mk/test-integration.mk | No change | A-8: tag-derived, verified |

### Audit Summary
- **Total items:** 14 AC + 10 task requirements + 25 tests + file set
- **Done:** all (unit tests green on darwin `-race`; integration tests cross-vet green, run in QEMU)
- **Partial:** none
- **Skipped:** none
- **Changed:** 3 deviations (IPv4 header location, counters.go split, address re-resolution without a 3rd goroutine) — documented in Deviations + Mistake Log

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Adverts leave with virtual-MAC L2 source, TTL/hop-limit 255, correct IPvX source | QEMU integration test | `TestIntegrationAdvertOnPeerVeth` (L2 src == vmac, TTL 255, TOS 0xc0, src 192.0.2.251, dst 224.0.0.18) + `TestIntegrationAdvertV6OnWire` (hop limit 255, dst ff02::12, src macvlan link-local); tx socket options asserted by `TestIntegrationOpenInstanceSocketOptions` |
| Received adverts reach the engine with TTL/dst meta for GTSM validation | QEMU integration + unit test | `TestIntegrationRxDeliversFromPeer` (peer sends TTL 254; delivered unmodified with Family/Dst) + `TestReadLoopDeliversRxMeta` (rxSink delivers RxMeta{TTL 254, Src, Dst, Family, IfIndex} + payload) |
| GARP and NA frames are byte-correct and burst-repeated | golden unit tests + QEMU capture | `TestBuildGARPFrameGolden`/`TestBuildNAMessageGolden` (exact bytes), `TestNAHoloBugNegatives` (dst/TLL/hop-limit/pseudo-header), `TestAnnounceBurstRepeatsAndSpacing` (3x100ms), `TestIntegrationGARPOnWire`/`TestIntegrationNAOnWire` (on-wire byte-equal + kernel checksum verifies) |
| Readiness observable before start | doctor unit test | `TestVRRPRawSocketDoctorWarn` (warn when vrrp configured + probe false), `TestVRRPRawSocketDoctorSilent` (silent otherwise), `TestVRRPRawSocketCodeRegistered` (explainable); functional `vrrp-doctor.ci` owned by spec-vrrp-5 |
| Transport observable in production | metrics tests | `TestTransportMetricsRegistered` (5 series, exact names/labels) + `TestCounterSnapshotAndReset` (per-instance read-back + Prometheus increments, monotonic across reset); functional `vrrp-metrics.ci` owned by spec-vrrp-5 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during /ze-review)

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end (QEMU suite green, package in derived list)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (bound to spec-vrrp-6 scenarios above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-vrrp-4-transport.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-vrrp-4-transport.md` only (preserves edited spec in git history from commit A)
