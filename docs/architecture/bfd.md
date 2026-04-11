# BFD — Bidirectional Forwarding Detection

**Status:** skeleton merged, NOT wired into the engine startup path. The plugin
compiles, the codec and FSM are tested under `-race`, and a loopback engine
test exercises the full three-way handshake. The `RunBFDPlugin` entry point
is a no-op stub; opening real UDP sockets and exposing the Service over RPC
lands in a follow-up commit.

## Source

| Area | Path |
|------|------|
| Plugin entry | `internal/plugins/bfd/bfd.go`, `internal/plugins/bfd/register.go` |
| Public API types | `internal/plugins/bfd/api/` |
| Wire codec | `internal/plugins/bfd/packet/` |
| Session FSM + timers | `internal/plugins/bfd/session/` |
| UDP and loopback transports | `internal/plugins/bfd/transport/` |
| Express-loop runtime | `internal/plugins/bfd/engine/` |
| YANG schema | `internal/plugins/bfd/schema/ze-bfd-conf.yang` |
| Reference RFCs | `rfc/short/rfc5880.md`, `rfc5881.md`, `rfc5882.md`, `rfc5883.md` |
| Deep-dive research | `docs/research/bfd-implementation-guide.md` |

<!-- source: internal/plugins/bfd/ -- plugin layout -->

## Design

### Encapsulation onion

BFD follows the same buffer-first / lazy-over-eager principles as the BGP
subsystem. Wire bytes flow into a pre-allocated pool slot, are parsed by
value into a `packet.Control` struct without allocation, drive the
`session.Machine` state mutations, and the response is written back into a
pool buffer via `Control.WriteTo(buf, off) int`. No `make([]byte, ...)` runs
on the per-packet hot path.

<!-- source: internal/plugins/bfd/packet/control.go -- ParseControl, WriteTo -->
<!-- source: internal/plugins/bfd/packet/pool.go -- Acquire, Release -->

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

<!-- source: internal/plugins/bfd/engine/loop.go -- run, tick, handleInbound -->
<!-- source: internal/plugins/bfd/engine/engine.go -- Loop type -->

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

<!-- source: internal/plugins/bfd/engine/engine.go -- firstPacketKey, firstPacketIndex -->

### Discriminator allocation

Discriminators are 32-bit unsigned, must be unique within the local
implementation, and zero is reserved by RFC 5880 §6.3 as "unknown."
`allocateDiscriminatorLocked` walks the counter, skipping zero on wrap and
checking `byDiscr` for collisions before assigning. After 2³² attempts it
returns `ErrDiscriminatorSpaceExhausted`. The walk is O(N) in live session
count, which only matters at session creation time and is bounded by config.

<!-- source: internal/plugins/bfd/engine/engine.go -- allocateDiscriminatorLocked -->

### Lock order

The `Loop` has two mutexes: `mu` for the session registry and `subsMu` for
the subscriber registry. Lock order is `mu → subsMu`; the reverse is
forbidden. The express loop holds `mu` while calling into the session FSM,
which calls `notify` which briefly takes `subsMu` to read the subscriber
list. Subscriber delivery happens outside `subsMu` via a non-blocking
capacity check (`trySendStateChange`) so a slow consumer cannot stall the
loop. The single-writer invariant (only the express loop writes to subscriber
channels) keeps the `len/cap` precheck race-free.

<!-- source: internal/plugins/bfd/engine/engine.go -- Loop, makeNotify, trySendStateChange -->

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

<!-- source: internal/plugins/bfd/session/timers.go -- DetectionInterval, TransmitInterval -->

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

<!-- source: internal/plugins/bfd/session/fsm.go -- onStateChange -->
<!-- source: internal/plugins/bfd/session/timers.go -- CheckDetection -->

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

<!-- source: internal/plugins/bfd/packet/pool.go -- Buf, Acquire, Release -->
<!-- source: internal/plugins/bfd/transport/udp.go -- readLoop -->
<!-- source: internal/plugins/bfd/packet/bench_test.go -- BenchmarkRoundTrip -->

## Wire format

A BFD Control packet is 24 bytes mandatory plus an optional authentication
section. The fields are documented in `rfc/short/rfc5880.md` Section 4.1.
The codec emits and accepts the literal RFC layout in network byte order.

The reception procedure (Section 6.8.6) is implemented as a straight-line
ladder of structural checks (`packet.ParseControl`), followed by the FSM
transition table (`session.applyTransitionLocked`). Every reject path returns
a typed error so a fuzz harness can assert that no malformed input slips
through; `FuzzParseControl` and `FuzzParseAuth` cover this.

<!-- source: internal/plugins/bfd/packet/control.go -- ParseControl reception checks -->
<!-- source: internal/plugins/bfd/session/fsm.go -- applyTransitionLocked -->
<!-- source: internal/plugins/bfd/packet/fuzz_test.go -- fuzz harnesses -->

## What is not done

These items are intentionally absent from the skeleton commit and will land
as follow-ups when wiring begins:

| Gap | Required for | Notes |
|-----|--------------|-------|
| `RunBFDPlugin` real implementation | making the plugin reachable from a running ze | Pattern: `internal/plugins/sysrib/sysrib.go` |
| `make generate` to add to `internal/component/plugin/all/all.go` | plugin auto-load on engine startup | The `register.go` warning comment exists to prevent accidental wiring |
| GTSM TTL extraction (`IP_RECVTTL`/`IPV6_RECVHOPLIMIT`) | RFC 5881 §5 single-hop spoofing defence | Transport sets `Inbound.TTL = 0`; engine must enforce TTL=255 once available |
| Outbound TTL=255 | Peer's GTSM check passing | `IP_TTL` setsockopt |
| `SO_BINDTODEVICE` for single-hop | Preventing kernel from picking the wrong interface | Required when multiple interfaces share an IP range |
| Per-VRF socket setup | Multi-VRF deployments | The `VRF` field is currently a label, not a kernel primitive |
| Jitter on TX | RFC 5880 §6.8.7 (0–25% per packet, anti-self-synchronisation) | One-line addition in `engine/loop.go` `tick` |
| Authentication | RFC 5880 §6.7 (deferrable) | Parser exists; verifier and key management are TODO |
| Echo mode | RFC 5880 §6.4 + RFC 5881 §5 (single-hop optional) | UDP 3785 socket, RTT tracking |
| Demand mode | RFC 5880 §6.6 (rarely deployed) | Skipped intentionally |
| BGP plugin opt-in (`bgp peer { bfd { ... } }`) | Real BFD use from BGP | Shape proposed in `docs/guide/bfd.md` |
| `show bfd sessions` CLI | Operator visibility | YANG-driven CLI command |
| `test/plugin/bfd/*.ci` functional tests | Integration completeness per `rules/integration-completeness.md` | Required before claiming "wired" |
| Interop with FRR `bfdd` | Wire-compat verification | Highest-value test |

## Testing

```
go test -race ./internal/plugins/bfd/...
go test -fuzz=FuzzParseControl -fuzztime=10s ./internal/plugins/bfd/packet/
go test -fuzz=FuzzParseAuth    -fuzztime=10s ./internal/plugins/bfd/packet/
go test -run=^$ -bench=BenchmarkRoundTrip -benchmem ./internal/plugins/bfd/packet/
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
