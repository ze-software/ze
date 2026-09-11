# BFD — Bidirectional Forwarding Detection

**Status:** implemented and wired. The internal plugin includes UDP transport,
GTSM, RFC 5880 FSM, detection timers, keyed MD5 and SHA1 authentication,
echo mode, and multi-hop support. Static routes and BGP peers can use the BFD
service for next-hop and session tracking.
<!-- source: internal/component/bfd/bfd.go -- RunBFDPlugin -->
<!-- source: internal/component/bfd/transport/udp_linux.go -- applySocketOptions, applySocketOptionsV6 -->
<!-- source: internal/component/bfd/session/auth.go -- SetAuth, Sign, Verify -->
<!-- source: internal/component/bfd/engine/echo.go -- echoTickLocked, handleEchoInbound -->

## Source

| Area | Path |
|------|------|
| Plugin entry | `internal/component/bfd/bfd.go`, `internal/component/bfd/register.go` |
| Public API types | `internal/component/bfd/api/` |
| Wire codec | `internal/component/bfd/packet/` |
| Session FSM + timers | `internal/component/bfd/session/` |
| UDP and loopback transports | `internal/component/bfd/transport/` |
| Express-loop runtime | `internal/component/bfd/engine/` |
| YANG schema | `internal/component/bfd/yang/ze-bfd-conf.yang` |
| Reference RFCs | `rfc/short/rfc5880.md`, `rfc5881.md`, `rfc5882.md`, `rfc5883.md` |
| Client integration pattern | `docs/architecture/ospf/bfd-client.md` |
| Deep-dive research | `docs/research/bfd-implementation-guide.md` |

<!-- source: internal/component/bfd/ -- plugin layout -->

## Design

### Encapsulation onion

BFD follows the same buffer-first / lazy-over-eager principles as the BGP
subsystem. Wire bytes flow into a pre-allocated pool slot, are parsed by
value into a `packet.Control` struct without allocation, drive the
`session.Machine` state mutations, and the response is written back into a
pool buffer via `Control.WriteTo(buf, off) int`. No `make([]byte, ...)` runs
on the per-packet hot path.

<!-- source: internal/component/bfd/packet/control.go -- ParseControl, WriteTo -->
<!-- source: internal/component/bfd/packet/pool.go -- Acquire, Release -->

### Express-loop runtime

The engine borrows BIRD 3.x's "express loop" pattern: every BFD-related
mutation runs on one dedicated goroutine per `Loop` instance (typically one
per VRF). The loop owns the session map exclusively, so individual sessions
need no locks. The trade-off is that the loop has to do its own timer
scheduling rather than rely on Go's runtime; a 5 ms `PollInterval` ticker
gives sub-50 ms detection-time resolution at modest CPU cost.

The reason for choosing this model over a goroutine-per-session is GC
sensitivity: at 50 ms BFD intervals with `DetectMult=3`, a 150 ms STW pause
looks indistinguishable from a real failure. Keeping the hot path
allocation-free and pinned to one goroutine minimises GC pressure on the
session-driving thread.

<!-- source: internal/component/bfd/engine/loop.go -- run, tick, handleInbound -->
<!-- source: internal/component/bfd/engine/engine.go -- Loop type -->

### Session lookup

Two indexes:

| Index | Key | Used when |
|-------|-----|-----------|
| `byDiscr` | local discriminator (uint32) | `Your Discriminator != 0` — fast path |
| `byKey` | `(peer, vrf, mode, interface)` | First-packet (`Your Discriminator == 0`) |

The first-packet index is essential because RFC 5880 §6.8.6 leaves the
demultiplexing tuple to the application. Falling back to a linear scan over
the session map (a tempting shortcut) is non-deterministic across Go's
randomised map iteration order — two sessions sharing the same `(peer, mode)`
but differing by interface/VRF would race for the first incoming packet.

<!-- source: internal/component/bfd/engine/engine.go -- firstPacketKey, firstPacketIndex -->

### One session per neighbor, whatever asks for it

RFC 5882 §4.4: "If multiple control protocols wish to establish BFD sessions
with the same remote system for the same data protocol, all MUST share a single
BFD session."

The engine shares a session per `api.Key`, so the requirement binds the key as
much as the registry. The clients do not build a key the same way. OSPF always
names the interface it runs on and the address it runs from. BGP names an
interface only when the operator wrote the optional `bfd interface` leaf, and a
local address only when the peer has one. A pinned `single-hop-session` entry
can leave both out.

`api.SessionRequest.Canonical` reduces those shapes to one key before the
engine sees them. Every client passes through it, `pluginService.EnsureSession`
for the protocol clients and `applyPinned` for the configured ones, so BGP and
OSPF to one neighbor land on one session and one packet stream. It defaults the
VRF, and then completes what the client left out, differently for the two hop
modes:

| Mode | Interface | Local address |
|------|-----------|---------------|
| Single-hop | the link holding the request's local address, or the one link whose connected prefix contains the peer | that link's address in the prefix that contains the peer |
| Multi-hop | cleared: a routed session is on no link, and `SessionRequest.Interface` is documented single-hop only. The transport holds the other half of that invariant, stamping no ingress interface on a multi-hop packet: the first-packet index is an exact match, so one stamped there would make every RFC 5880 §6.8.6 lookup miss | the address on the interface the route to the peer leaves by, which is the source the stack would have chosen |

A link belongs to a routing instance, and a request is only ever given a link
from the VRF it names. `connectedLinks` resolves each link's VRF by walking
`MasterIndex` up to a device of type `vrf`, so a member under a bridge under a
VRF reads as that VRF. The same prefix in two VRFs reaches two different
systems, so crossing them would merge two sessions that are not one.

The derivation refuses to guess, and every refusal leaves the request exactly as
the client wrote it. Single-hop refuses when no link matches or more than one
does; an IPv6 link-local peer is the standing example, because every link
carries `fe80::/64`. Multi-hop refuses when there is no route, when the egress
interface carries no address of the peer's family, and when it carries more than
one. A multi-hop request in a VRF is refused at the source: `ifcomp.RouteLookup`
reads the default routing table, so `topologyFor` does not call it there. With
no interface backend loaded the link table is empty and every key stays as its
client wrote it. Both OSPF families name their own interface and address, so
they never reach the derivation at all.

A refusal is safe but not free: the under-specified request gets its own
session, so those are exactly the configurations where §4.4's single session is
not achieved. The seven of them are listed in `rfc/short/rfc5882.md`, under
"Multiple Control Protocols (Section 4.4)", where the conformance ledger reads
them.

<!-- source: internal/component/bfd/api/session_identity.go -- Canonical, canonicalMultiHop -->
<!-- source: internal/component/bfd/transport/udp.go -- ingressInterface -->
<!-- source: internal/component/bfd/session_identity.go -- connectedLinks, topologyFor, vrfMembership -->

### Discriminator allocation

Discriminators are 32-bit unsigned, must be unique within the local
implementation, and zero is reserved by RFC 5880 §6.3 as "unknown."
`allocateDiscriminatorLocked` walks the counter, skipping zero on wrap and
checking `byDiscr` for collisions before assigning. After 2³² attempts it
returns `ErrDiscriminatorSpaceExhausted`. The walk is O(N) in live session
count, which only matters at session creation time and is bounded by config.

<!-- source: internal/component/bfd/engine/engine.go -- allocateDiscriminatorLocked -->

### Lock order

The `Loop` has two mutexes: `mu` for the session registry and `subsMu` for
the subscriber registry. Lock order is `mu → subsMu`; the reverse is
forbidden. The express loop holds `mu` while calling into the session FSM,
which calls `notify` which briefly takes `subsMu` to read the subscriber
list. Subscriber delivery happens outside `subsMu` via a non-blocking
capacity check (`trySendStateChange`) so a slow consumer cannot stall the
loop.

`Loop.subscribe` is the second writer to a subscriber channel, and the only
other one. It takes both locks in the same `mu → subsMu` order and, inside
them, enqueues the session's current state and appends the channel to the
registry. Both halves are needed: the snapshot is what lets a client that joins
an already-Up session learn the state at all, since `EnsureSession` on an
existing key only bumps a refcount, and doing them together is what stops a
transition landing in the gap and being written to a subscriber list the new
channel is not yet in.

That leaves the `len/cap` precheck race-free for a different reason than the
old single-writer invariant gave. The express loop is still the only writer to
a PUBLISHED channel: `subscribe` writes only while it holds both locks, before
the append, so `makeNotify` cannot hold that channel yet and cannot run at all
meanwhile.

<!-- source: internal/component/bfd/engine/engine.go -- Loop, subscribe, makeNotify, trySendStateChange -->

### Timer arithmetic

Per RFC 5880 §6.8.4, detection time is computed using the **remote**
multiplier and the negotiated RX rate:

```
detect_time = remote_DetectMult × max(local_RequiredMinRx, remote_DesiredMinTx)
```

A common bug is using the local multiplier; `DetectionInterval` in
`session/timers.go` carefully uses `RemoteDetectMult` and falls back to the
local value only before the first packet has been received. All time math
runs in microseconds because RFC 5880 expresses every interval in
microseconds; converting to milliseconds anywhere on the hot path produces
sessions that run 1000× off-rate.

<!-- source: internal/component/bfd/session/timers.go -- DetectionInterval, TransmitInterval -->

### Slow start and Poll/Final

Sessions Init at the slow-start floor (1 second TX) regardless of the
configured operating interval, per RFC 5880 §6.8.3. Once the FSM reaches
`Up`, `onStateChange` initiates a Poll Sequence to switch to the configured
fast intervals. The Poll bit stays set on outgoing packets until the peer
replies with `F=1`, at which point `Receive` clears `PollOutstanding`.

The detection deadline is cleared after a detection-time fire so subsequent
ticks do not see a stale past time. RFC 5880 §6.8.1's requirement to clear
`bfd.RemoteDiscr` is honored only on detection-driven Down transitions, not
on peer-signaled Down (a peer-signaled Down still leaves the peer reachable
and clearing the discriminator would force an unnecessary handshake reset).

<!-- source: internal/component/bfd/session/fsm.go -- onStateChange -->
<!-- source: internal/component/bfd/session/timers.go -- CheckDetection -->

### Memory lifecycle

| Stage | Pool | Notes |
|-------|------|-------|
| RX (UDP transport) | Per-`UDP` instance: 16-slot ring of 128-byte buffers, allocated once at goroutine start. Slot release closures are pre-built and indexed by slot — zero per-packet alloc. |
| RX (loopback transport) | `packet.Acquire()` per send; the receiver's `Inbound.Release` calls `packet.Release`. |
| TX | `packet.Acquire()` per outgoing packet; `defer packet.Release(pb)` returns it. |
| Parsing | `ParseControl` returns a `Control` value; no escape, no alloc. |
| Session FSM | All state lives on `Machine` which is heap-allocated once at session creation. Mutations are in place. |

The `packet.Buf` type wraps `*[]byte` so the same pointer round-trips through
`sync.Pool`'s `Get`/`Put` without escaping a fresh slice header per release.
The benchmark `BenchmarkRoundTrip` measures **0 B/op, 0 allocs/op** at ~60
ns/op for the full Acquire → WriteTo → ParseControl → Release cycle.

<!-- source: internal/component/bfd/packet/pool.go -- Buf, Acquire, Release -->
<!-- source: internal/component/bfd/transport/udp.go -- readLoop -->
<!-- source: internal/component/bfd/packet/bench_test.go -- BenchmarkRoundTrip -->

## Wire format

A BFD Control packet is 24 bytes mandatory plus an optional authentication
section. The fields are documented in `rfc/short/rfc5880.md` Section 4.1.
The codec emits and accepts the literal RFC layout in network byte order.

The reception procedure (Section 6.8.6) is implemented as a straight-line
ladder of structural checks (`packet.ParseControl`), followed by the FSM
transition table (`session.applyTransitionLocked`). Every reject path returns
a typed error so a fuzz harness can assert that no malformed input slips
through; `FuzzParseControl` and `FuzzParseAuth` cover this.

<!-- source: internal/component/bfd/packet/control.go -- ParseControl reception checks -->
<!-- source: internal/component/bfd/session/fsm.go -- applyTransitionLocked -->
<!-- source: internal/component/bfd/packet/fuzz_test.go -- fuzz harnesses -->

## Remaining gaps

The remaining implementation and verification gaps are:

| Gap | Required for | Notes |
|-----|--------------|-------|
| Demand mode | RFC 5880 §6.6 (rarely deployed) | Skipped intentionally |

## Stage 2 complete (production transport hardening)

Stage 0 (skeleton, commit `e5a4add9`), Stage 1 (lifecycle wiring), and
Stage 2 (transport hardening) are all merged. The production path is now:

| Layer | File | Responsibility |
|-------|------|----------------|
| Transport open | `internal/component/bfd/transport/udp.go` | `net.ListenConfig.ListenPacket` with a Control callback that invokes `applySocketOptions`. |
| Socket options | `internal/component/bfd/transport/udp_linux.go` | `IP_RECVTTL`, `IP_TTL=255`, `SO_BINDTODEVICE` (Linux only). |
| Packet recv | `internal/component/bfd/transport/udp.go` -- `readLoop` | `ReadMsgUDPAddrPort` + `parseReceivedTTL` populate `Inbound.TTL`. |
| TTL gate | `internal/component/bfd/engine/loop.go` -- `passesTTLGate` | RFC 5881 §5 single-hop TTL=255 and RFC 5883 §5 multi-hop min-TTL. |
| Jitter | `internal/component/bfd/engine/engine.go` -- `applyJitter` | RFC 5880 §6.8.7 [0, 25%) reduction, clamped [10%, 25%) when `detect-multiplier=1`. |
| Device choice | `internal/component/bfd/bfd.go` -- `resolveLoopDevices` | Per-loop `SO_BINDTODEVICE` target derived from pinned sessions. |

## Stage 4 complete (operator UX)

Stage 4 adds the operator-facing surface on top of the engine Snapshot:
three `show bfd` commands and five `ze_bfd_*` Prometheus metrics.
<!-- source: internal/component/bfd/engine/snapshot.go — Loop.Snapshot, Loop.SessionDetail -->
<!-- source: internal/component/bfd/cmd/bfd.go — handleShowSessions, handleShowSession, handleShowProfile -->
<!-- source: internal/component/bfd/metrics.go — bfdMetrics, metricsHook, refreshSessionsGauge -->

| Layer | File | Responsibility |
|-------|------|----------------|
| Snapshot types | `internal/component/bfd/api/snapshot.go` | `SessionState`, `TransitionRecord`, `ProfileState` |
| Engine Snapshot | `internal/component/bfd/engine/snapshot.go` | `Loop.Snapshot`, `Loop.SessionDetail`, `sessionEntry.snapshot` |
| Transition ring | `internal/component/bfd/engine/engine.go` | `sessionEntry.recordTransition` (`api.TransitionHistoryDepth=8`) |
| Metrics hook | `internal/component/bfd/engine/engine.go` | `Loop.SetMetricsHook`, `MetricsHook` interface |
| Prometheus wiring | `internal/component/bfd/metrics.go` | `bindMetricsRegistry`, `metricsHook`, `refreshSessionsGauge` |
| Service surface | `internal/component/bfd/api/service.go` | `Snapshot`, `SessionDetail`, `Profiles` |
| CLI handlers | `internal/component/bfd/cmd/bfd.go` | `handleShowSessions`, `handleShowSession`, `handleShowProfile` |
| YANG API | `internal/component/bfd/yang/ze-bfd-api.yang` | `show-sessions`, `show-session`, `show-profile` RPCs |
| YANG cmd tree | `internal/component/cmd/bfd/yang/ze-bfd-cmd.yang` | augments `clishowcmd:show` with `bfd { sessions, session, profile }` |


### Things that are intentionally done the way they are

A future session may be tempted to "clean up" any of these. They are
deliberate:

| Looks like | Actual reason |
|-----------|---------------|
| `type Machine struct` instead of `type Session struct` in `session/` | `Machine` names the BFD state machine directly and avoids a second `Session` identity beside `internal/component/bgp/reactor.Session`. |
| `type Loop struct` instead of `type Engine struct` in `engine/` | `Loop` names the scheduled express loop and distinguishes it from `internal/component/engine.Engine`. |
| `trySendStateChange` uses a `len/cap` precheck instead of `select { case ch <- ...: default: }` | The precheck is race-free because the express loop is the only writer, and the invariant is documented on `Loop`. If a future refactor adds a second writer, use an explicit non-blocking send and document how a full channel is handled. |
| `packet.Buf` wraps `*[]byte` in a struct instead of using raw `[]byte` | `sync.Pool.Put(&buf)` escapes a fresh 24-byte slice header per call if you pass `[]byte`. Wrapping in a struct carried as a value lets the same `*[]byte` round-trip through the pool. The benchmark `BenchmarkRoundTrip` measures 0 B/op; any refactor that returns to raw `[]byte` will regress it. |
| `firstPacketKey` includes `Local` and `Interface` | Per RFC 5880 §6.8.6 the session is selected on "some combination of other fields", and the fields have to be ones the receiver can observe. It cannot observe the peer's chosen SOURCE address, but the destination address and the ingress interface are its own, and the kernel reports both in `IP_PKTINFO`. Two sessions to one peer address on two links, which is what an IPv6 link-local peer looks like, are distinguishable only by those two fields. The socket binds the wildcard, so `readLoop` takes them from the control message and never from `Bind`. |
| `allocateDiscriminatorLocked` walks the counter instead of using a random value | Deterministic for tests and trivially debuggable. Swap to CSPRNG seeding only if a deployment asks for it. |

## Testing

```
CGO_ENABLED=0 go test ./internal/component/bfd/...
CGO_ENABLED=0 go test -fuzz=FuzzParseControl -fuzztime=10s ./internal/component/bfd/packet/
CGO_ENABLED=0 go test -fuzz=FuzzParseAuth    -fuzztime=10s ./internal/component/bfd/packet/
CGO_ENABLED=0 go test -run=^$ -bench=BenchmarkRoundTrip -benchmem ./internal/component/bfd/packet/
```

The engine test (`TestLoopbackHandshake`) creates two paired `Loop`
instances over an in-memory `transport.Loopback` pair and asserts both reach
`Up` through the full three-way handshake. `TestLoopbackPollFinalTerminates`
asserts the Poll Sequence terminates and the configured fast TX interval
takes over from the slow-start floor. `TestUDPLoopback` exercises the real
kernel UDP path on `127.0.0.1`.

## Reference

- RFC 5880 — base protocol (`rfc/short/rfc5880.md`)
- RFC 5881 — single-hop, UDP 3784, GTSM (`rfc/short/rfc5881.md`)
- RFC 5882 — generic application: client contract (`rfc/short/rfc5882.md`)
- RFC 5883 — multi-hop, UDP 4784 (`rfc/short/rfc5883.md`)
- Implementation guide: `docs/research/bfd-implementation-guide.md`
- Operator guide (planned UX): `docs/guide/bfd.md`
