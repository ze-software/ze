# IS-IS Link-State Database

`internal/plugins/isis/lsdb` is the per-level store of every LSP plus own-LSP
origination: the entry model, the freshness compare, the per-circuit flooding
flags, aging with zero-age purge, refresh, sequence wraparound, and fragmentation
across LSP numbers 0 to 255.

The **wire** contents of an originated LSP, the aging and purge timings, and the
fragmentation rules are documented in
[`../wire/isis.md`](../wire/isis.md). This page carries the storage and
engine-side design.

| Concern | File |
|---------|------|
| The two-level store and flag API | `lsdb/lsdb.go` |
| Per-LSP entry, lazy decode, freshness compare | `lsdb/entry.go` |
| Own-LSP origination and the fragment packer | `lsdb/origination.go` |
| Aging, purge, garbage collection | `lsdb/aging.go` |
| IPv6 scope filters | `lsdb/origination_ipv6.go` |
| Engine glue | `lsdb_wiring.go` |
| Own-LSP sequence and conflict state | `own_lsp_conflict.go` |

## Decision: store raw bytes, parse TLVs lazily

An entry holds one **owned** copy of the verbatim PDU plus a small typed header
(LSP ID, sequence, remaining lifetime, checksum, type block) and parses TLVs only
on demand. This mirrors the BGP `WireUpdate` model: an LSP carrying an unknown
TLV re-floods byte for byte (ISO/IEC 10589 clause 7.3.14), and SPF and the CLI
parse only what they need.

Receive and insert always copy into an entry-owned slice, so a reused transport
receive buffer cannot corrupt a stored entry. A test scribbles the caller's
buffer after the store and asserts the stored bytes are intact.

<!-- source: internal/plugins/isis/lsdb/entry.go -- Entry, Raw, Decode, compareFreshness -->
<!-- source: internal/plugins/isis/lsdb/lsdb.go -- the two-level store, Receive, Insert, the flag API -->

## Decision: the LSDB stores the flooding flags, the flooding layer drives them

The send-routeing-message (SRM) and send-sequence-number (SSN) flags are per-LSP,
per-circuit maps keyed by a small engine-assigned circuit ID rather than the
sparse kernel ifindex. The LSDB stores, queries and clears them; the flooding
layer drives them. A third map distinguishes a first flood from a true
retransmission, so the resend counter is accurate. Closing a circuit drops it
from every entry.

## Decision: freshness has three tiers, in order

Sequence, then a purge tiebreak, then checksum.

- A higher sequence is newer.
- At an equal sequence a remaining-lifetime-0 purge beats a live LSP (clause
  7.3.16.1): a withdrawn LSP cannot be resurrected.
- At an equal sequence and equal purge state, a differing checksum keeps **ours**
  so a bit-flipped duplicate cannot displace a good LSP. The originator bumps its
  sequence to resolve genuine ambiguity.

Skipping the lifetime tier loses every purge that arrives at the same sequence.

## Decision: purge is not expiry, and both are retained

A lifetime-0 LSP is **marked** purged, not deleted, and kept for the zero-age
lifetime before it is collected. A purge that arrived on the wire is re-flooded
once inside that window, guarded so there is no per-second storm, and it stays
observably distinct from a local expiry through a received-purge flag.

<!-- source: internal/plugins/isis/lsdb/aging.go -- Tick, markPurgedLocked, the zero-age grace and collection -->

## Decision: origination is a pure function, so a redundant regeneration is free

Origination is full regeneration from `(node info, level state)`. Because it is
pure, the engine compares the freshly built input against the last-originated
input and skips a regeneration that would produce the same bytes. An adjacency
flap that fires origination from several goroutines for the **same** resulting
state collapses to one sequence bump and one re-flood, while a real topology
change always falls through. This is the flooding-amplification fix.

<!-- source: internal/plugins/isis/lsdb/origination.go -- the Originator, full regeneration, the fragment packer -->

## Decision: sequence wraparound is purge, suspend, then restart at 1

At the 32-bit maximum the originator purges the LSP ID at that sequence, records
a suspension deadline of max age plus the zero-age lifetime, and refuses to
re-originate that ID until the window elapses. It then restarts at 1.

The wrap pass itself does **not** produce the sequence-1 LSP: a wrapped fragment
is purged, suspended and skipped on that same pass, and a **later** origination
call after the deadline produces it.

## Decision: engine glue lives in its own root file

`lsdb_wiring.go` owns the engine's LSDB and originator, the adjacency-up
origination trigger, the one-second aging loop, SRM arming on eligible circuits,
and the snapshot proxy. It is a sibling of `flooding_wiring.go` and
`dis_wiring.go` rather than being threaded through the circuit or LSDB packages.

<!-- source: internal/plugins/isis/lsdb_wiring.go -- the engine LSDB, the aging loop, SRM arming -->

## Trap: the aging tick is the only refresh driver in a quiescent network

The aging pass folds in own-LSP and pseudo-node refresh. Without that, nothing
else calls origination in a settled network, the node's own LSPs age to max age
and purge, and the node vanishes from its peers' databases. Refresh is cheap when
it is not due: a timestamp compare, with no level-state rebuild.

## Trap: stale-fragment purge stops at the first gap

The stale-fragment walk stops at the first absent fragment number. Fragments are
always contiguous from 0, so a gap means "no more own fragments", not "skip and
continue".

## Trap: an equal-sequence update does not re-store bytes

Only a newer LSP, or a first sighting, copies raw bytes and bumps the size gauge.
An equal-sequence update touches the lifetime alone; an older one stores nothing.
The sorted-ID cache is rebuilt only when the **key set** changes, never on a
freshness overwrite, so the hot flood and CSNP path copies an already-sorted
slice under the read lock.

## Trap: a purge must strip the body and be re-signed

RFC 5304 section 2 requires an originator that purges to remove the LSP body and
add the authentication TLV. Every purge routes through the body strip and then
the signer, so a purge can never carry a stray body and always carries a valid
TLV 10 when signing is on.

## Trap: the metric handle read is the race, not the increment

`SetMetrics` rebinds the interface value under the write lock while the
originator increments counters holding its **own** mutex. The counter increment
is concurrency-safe; the **handle read** is not, so it is taken under the lock.
The LSDB never acquires the originator's mutex, so there is no lock-ordering
cycle.

## Bounds

A 256-fragment cap and a per-level maximum LSP count bound memory against a
hostile flood of distinct LSP IDs. An LSP ID already present is always updatable,
so a refresh or a purge is never dropped by the cap.
