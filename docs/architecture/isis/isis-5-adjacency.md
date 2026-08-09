# IS-IS Circuits and Adjacency

A circuit is the per-interface runtime created for each interface IS-IS is
enabled on. It consumes the Layer-2 byte pipe and the hello codec, runs a
periodic hello sender, and drives one adjacency state machine per neighbor
(ISO/IEC 10589 clause 8.2).

| Concern | File |
|---------|------|
| Adjacency record and state | `adjacency/adjacency.go` |
| The state machine | `adjacency/fsm.go` |
| Per-circuit neighbor table | `adjacency/table.go` |
| Circuit struct and interfaces | `circuit/circuit.go` |
| Hello origination and padding | `circuit/hello.go` |
| Receive dispatch, send, sweep | `circuit/runtime.go` |
| Engine glue | `circuits.go` |

## Decision: the state machine is pure

`ReceiveHello` takes a parsed hello input, the immutable local identity, and the
current time; it mutates an adjacency in place and returns a transition. It does
no I/O and never parses raw bytes. The circuit decodes with the codec, fills the
input, and is the single writer of the table.

Every transition is therefore unit-testable with a fake clock and no socket,
which is what let this layer be completed and proven on a host with no
`AF_PACKET`.

<!-- source: internal/plugins/isis/adjacency/fsm.go -- ReceiveHello, Expire, Down, classify -->

## Decision: bidirectionality is one medium-specific switch

The condition for reaching Up differs by medium and lives in one place:

| Medium | Up condition |
|--------|--------------|
| LAN | the neighbor's TLV 6 SNPA list contains our own SNPA |
| Point-to-point, TLV 240 ever seen | the neighbor reports Up or Initializing **and** echoes our system ID (RFC 5303) |
| Point-to-point, no TLV 240 ever seen | implicit two-way fallback (RFC 5303 section 3.2) |

Whether TLV 240 was ever seen is **sticky** per adjacency, so a three-way-capable
peer is never silently downgraded to the legacy path.

## Decision: padding is owned by the engine

`circuit/hello.go` builds the full hello (origination TLV 1, 129, 132, plus TLV 6
on a LAN or TLV 240 on a point-to-point link), then `padHello` appends TLV 8 to
the interface MTU **before** authentication, so the digest covers the padding
(RFC 5304). The transport adds only 802.3 and LLC framing.

<!-- source: internal/plugins/isis/circuit/hello.go -- the hello build, padHello, HoldTime -->

The advertised hold time is the hello interval times the hold multiplier, clamped
to the 16-bit range, so a zero multiplier can never advertise an
instantly-expiring adjacency.

## Decision: keying and the reap grace period

The table keys per `(system ID, level)`: one adjacency per point-to-point
circuit, one per `(circuit, system ID)` on a LAN. A dropped adjacency is held for
a grace period before it is reaped, to absorb transient flaps.

<!-- source: internal/plugins/isis/adjacency/table.go -- Update, Snapshot, UpCount, the reap grace -->

## Trap: an RWMutex is not reentrant

Rendering the event snapshot through the table's read lock while inside a
write-locked iteration deadlocks. The fix adds a lock-free per-adjacency snapshot
captured under the existing write lock, and fires events **after** the lock is
released.

This bug class recurred across the circuit and table work. Prefer a value
snapshot captured under the writer. Never re-enter a held RWMutex.

<!-- source: internal/plugins/isis/circuit/runtime.go -- fireEvents, called outside the table lock -->

## Trap: TLV 8 padding and the PDU length field

The hello encoder backfills the PDU length to the **unpadded** length. Appending
TLV 8 after that makes a receiver's decoder skip the padding. `padHello` rewrites
the PDU length field to the full padded length.

## Partial: the level-1 area mismatch reject is not logged

A level-1 area mismatch is **rejected**, so no adjacency forms. That is the
load-bearing security property and it is complete. The reason is returned as a
structured reject token, but the dispatch call site discards the returned
transition without emitting a log line. Emitting it is a small follow-up.

## Transport additions this layer required

The LAN three-way echo needs the frame source MAC threaded through to the state
machine, so the raw frame gained a source MAC field and the transport gained
circuit-info and ifindex-to-name accessors. Both additions are additive.

## Owned metrics

`ze_isis_adjacencies_up{level,interface}` and `ze_isis_adjacencies_total{level}`.
