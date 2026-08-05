# IS-IS Wire Format

Ze implements the IS-IS PDU and TLV wire codec per
[ISO/IEC 10589](../../../iso/short/iso10589.md) as extended by
[RFC 1195](../../../rfc/short/rfc1195.md),
[RFC 5305](../../../rfc/short/rfc5305.md) (wide metrics, TLV 22 / 135),
[RFC 5308](../../../rfc/short/rfc5308.md) (IPv6, TLV 232 / 236),
[RFC 5301](../../../rfc/short/rfc5301.md) (dynamic hostname, TLV 137),
[RFC 5303](../../../rfc/short/rfc5303.md) (P2P three-way, TLV 240), and
[RFC 5304](../../../rfc/short/rfc5304.md) (authentication TLV 10 structure).

The `spec-isis-2-wire` slice (see `plan/learned/NNN-isis-2-wire.md`) delivers the
serialization layer only: the common header, all nine PDU types, the core TLVs,
the ISO 8473 Fletcher checksum, and opaque unknown-TLV passthrough. It contains
no runtime, sockets, timers, LSDB, or FSM; those live in later children
(isis-3 transport, isis-5 adjacency, isis-6 LSDB, isis-7 flooding). All codec
code lives in `internal/plugins/isis/packet/`, depending only on the domain
types in `internal/plugins/isis/types` (isis-1).

<!-- source: internal/plugins/isis/packet/doc.go — package overview -->

## Layering

```
types (leaf)  <-  packet (this codec)  <-  runtime (transport, circuit, lsdb, spf)
```

`packet` imports only `internal/plugins/isis/types` (plus the Go standard
library and `internal/core/textbuf` for display). It MUST NOT import the
runtime, nor BGP-LS (which carries link-state topology inside BGP NLRI and is a
separate codepath).

- **Decode is lazy and zero-copy** (ISO/IEC 10589 clause 7.3.14): a PDU view
  holds the caller's byte slice plus offsets; `TLVIterator` yields
  `(type, value-slice)` without copying. Unknown TLVs are retained as opaque
  spans so the LSDB can re-flood them verbatim. A decoded view is valid only
  while the caller's backing slice is stable (isis-6 copies LSP bytes it
  retains; see `TLV.CopyValue`).
- **Encode is buffer-first** (`ai/rules/performance.md`): every PDU and TLV
  writes into a caller-owned buffer via `WriteTo(buf []byte, off int) int`. The
  PDU Length field and the LSP Fletcher checksum are written by
  skip-and-backfill, never a `Len()`-then-`WriteTo()` double traversal.

## Package surface

| Area | File | Key symbols |
|------|------|-------------|
| Common header | `header.go` | `PDUType` + 9 constants, `Header`, `DecodeHeader`, `CommonHeaderLen`, `ProtocolDiscriminator` |
| Dispatch | `pdu.go` | `PDU` (decoded union), `DecodePDU` |
| Hello | `hello.go` | `LANHello`, `P2PHello`, `CircuitType`, `DecodeLANHello`, `DecodeP2PHello` |
| LSP | `lsp.go` | `LSP`, `DecodeLSP`, `(*LSP).WriteTo`, `(*LSP).VerifyChecksum`, `(*LSP).IsOverloaded` |
| CSNP / PSNP | `csnp.go`, `psnp.go` | `CSNP`, `PSNP`, `DecodeCSNP`, `DecodePSNP` |
| Checksum | `checksum.go` | `Checksum`, `VerifyChecksum` (ISO 8473 Fletcher, two-step) |
| TLV framing | `tlv.go`, `tlv_opaque.go` | `TLVIterator`, `TLV`, `SubTLV`, `DecodeTLVs`, `AuthTLVIndex`, TLV type constants |
| Core TLVs | `tlv_core.go` | TLV 1, 8 (`WritePaddingTLV`), 9 (`LSPEntry`), 22 (`ExtISReachEntry`), 129, 137, 240 (`P2PThreeWayTLV`) |
| Neighbor TLVs | `tlv_neighbours.go` | TLV 6 (`ISNeighborsTLV`), TLV 2 (`NarrowISReachTLV`, decode-only) |
| IPv4 TLVs | `tlv_ipv4.go` | TLV 132, 135 (`ExtIPReachEntry`, `ExtendedIPReachTLV`) |
| IPv6 TLVs | `tlv_ipv6.go` | TLV 232, 236 (`IPv6ReachEntry`, `IPv6ReachabilityTLV`) |
| Auth TLV | `tlv_auth.go` | `AuthTLV`, auth-type constants (structure only; sign/verify is isis-10) |
| JSON view | `json.go` | `PDU.ToJSON`, `JSONView` (offline decode rendering) |

## Common header (ISO/IEC 10589 clause 9.5)

All nine PDU types share an 8-octet common header; the PDU type octet selects
the body layout.

| Offset | Bytes | Field | Notes |
|--------|-------|-------|-------|
| 0 | 1 | Intradomain Routeing Protocol Discriminator | `0x83` (NLPID for IS-IS) |
| 1 | 1 | Length Indicator | length of this PDU's fixed header |
| 2 | 1 | Version / Protocol ID Extension | `0x01` |
| 3 | 1 | ID Length | 0 (shorthand) or 6; Ze fixes System IDs at 6 octets |
| 4 | 1 | PDU Type | low 5 bits; top 3 reserved |
| 5 | 1 | Version | `0x01` |
| 6 | 1 | Reserved | `0x00` |
| 7 | 1 | Maximum Area Addresses | 0 = the default 3 |

`DecodeHeader` validates the discriminator, both version octets, and the ID
length, rejects an unknown PDU type, and returns the body offset
(`CommonHeaderLen`). It is bound-checked before every read and never panics.

### PDU type codes (authoritative, ISO/IEC 10589 clause 9)

| PDU | Code | PDU | Code |
|-----|------|-----|------|
| L1 LAN IIH | `0x0f` | L2 LSP | `0x14` |
| L2 LAN IIH | `0x10` | L1 CSNP | `0x18` |
| P2P IIH | `0x11` | L2 CSNP | `0x19` |
| L1 LSP | `0x12` | L1 PSNP | `0x1a` |
| | | L2 PSNP | `0x1b` |

> The internal research guide (`docs/research/isis-implementation-guide.md`
> sec 2) transcribes the L1 codes incorrectly (L1 LSP 0x18, L1 CSNP 0x24,
> L1 PSNP 0x26). The values above are the authoritative ISO/IEC 10589 values and
> are pinned by `TestISISPDUConstants`.

## PDU bodies

| PDU | Fixed fields after the common header |
|-----|--------------------------------------|
| LAN IIH | circuit type (1), System ID (6), holding time (2), PDU length (2), priority (1, high bit reserved, 0..127), LAN ID = SourceID (7), then TLVs (ISO/IEC 10589 clause 9.5: the priority octet's high bit IS the reserved bit -- there is no separate reserved octet, so the fixed header is 27, not 28) |
| P2P IIH | circuit type (1), System ID (6), holding time (2), PDU length (2), local circuit ID (1), then TLVs |
| LSP | PDU length (2), remaining lifetime (2), LSP ID (8), sequence number (4), checksum (2), type block (1), then TLVs |
| CSNP | PDU length (2), Source ID (7), start LSP ID (8), end LSP ID (8), then TLVs (TLV 9) |
| PSNP | PDU length (2), Source ID (7), then TLVs (TLV 9) |

The PDU Length field is written via skip-and-backfill on encode and bounds the
TLV region on decode (a zero or oversized length falls back to the available
body so decode never over-reads).

## Fletcher checksum (ISO/IEC 10589 clause 7.3.11)

LSPs carry an ISO 8473 Fletcher checksum computed from the octet **after** the
Remaining Lifetime field to the end of the PDU. The checksum field itself sits
inside the checksummed region (offset 12 within it: LSP ID 8 + sequence 4), so
it participates in its own computation. This requires a two-step adjustment.

`Checksum(data, checkOff)` runs the two running sums (mod 255, the value 0 being
reserved) with the checksum octets treated as zero, then solves for the two
octets X, Y such that re-summing the whole region yields zero:

```
X = (m-1)*C0 - C1   (mod 255)   -- high octet
Y = C1 - m*C0       (mod 255)   -- low octet
where m = len(region) - checkOff
```

`VerifyChecksum(data)` re-sums the region with the checksum in place and reports
whether both sums are zero. Encode and verify are separate, separately tested
functions; `TestISISChecksumVectors` proves `VerifyChecksum(encode(x)) == 0`
across offsets and lengths, and `TestISISChecksumDetectsCorruption` proves any
single-byte flip is detected. `(*LSP).WriteTo` backfills the checksum last;
`(*LSP).VerifyChecksum` validates a decoded LSP over its raw bytes.

## TLVs

TLVs use `Type (1) | Length (1) | Value (Length)`; the same framing nests as
sub-TLVs (RFC 5305 sec 2). `TLVIterator` walks a region lazily and stops cleanly
(`ErrTruncated`) on a TLV whose declared length overruns the region. Every
decoded TLV (known or unknown) is retained as an opaque `TLV`; re-encoding the
slice reproduces the region byte-for-byte, which is how the LSDB re-floods LSPs
carrying TLVs Ze does not understand.

| TLV | Name | Codec notes |
|-----|------|-------------|
| 1 | Area Addresses | length-prefixed list of 1..13-octet area addresses |
| 2 | IS Reachability (narrow) | **decode-only** (AC-14): parsed for interop; Ze originates TLV 22 instead |
| 6 | IS Neighbors | list of 6-octet SNPAs (MACs); basis for LAN three-way adjacency |
| 8 | Padding | zero octets to the MTU; written by `WritePaddingTLV` (originated in isis-5) |
| 9 | LSP Entries | 16-octet records (lifetime, LSP ID, sequence, checksum) for CSNP/PSNP |
| 10 | Authentication | auth-type octet + opaque value; structure only (sign/verify is isis-10) |
| 22 | Extended IS Reachability | 7-octet neighbor + **3-octet (24-bit)** metric + sub-TLV length + sub-TLVs (4/6/8) |
| 129 | Protocols Supported | NLPID list (`0xCC` IPv4, `0x8E` IPv6) |
| 132 | IP Interface Address | list of 4-octet IPv4 addresses |
| 135 | Extended IP Reachability | see layout below |
| 137 | Dynamic Hostname | 1..255-byte ASCII name |
| 232 | IPv6 Interface Address | list of 16-octet IPv6 addresses |
| 236 | IPv6 Reachability | see layout below |
| 240 | P2P Three-Way Adjacency | value length 1, 5, or 15 (state, +local circuit ID, +neighbor) |

### TLV 135 / 236 entry layout (canonical)

The metric width and the position of the up/down bit are the two recurring
traps; the layout below is the single cross-spec contract (umbrella
"Shared Contracts"), read by the codec (isis-2), origination (isis-6),
SPF (isis-9), redistribution (isis-11), and IPv6 (isis-12).

**TLV 135 (IPv4, RFC 5305 sec 4):**

| Field | Size | Notes |
|-------|------|-------|
| Metric | 4 octets | 32-bit (distinct from the 24-bit TLV 22 metric); never capped at 24-bit |
| Control | 1 octet | up/down bit `0x80` + sub-TLV-present (S) bit `0x40` + 6-bit prefix length (0..32) |
| Prefix | `ceil(len/8)` octets | packed; trailing bits zero |
| Sub-TLV length | 0 or 1 octet | present **only** when S is set |
| Sub-TLVs | variable | present only when S is set |

The up/down bit (RFC 5305 sec 4.1, RFC 2966) lives in the **control octet**, not
the high bit of the metric.

**Redistribution use (isis-11).** Routes redistributed *into* IS-IS (connected,
static, BGP) are originated as ordinary TLV 135 entries with a fixed default
metric. Because TLV 135 has **no external bit** (unlike TLV 236's `X`), a
redistributed IPv4 route is not wire-distinguishable from internal reachability;
the only flag is the up/down bit, which is `0` on injection and set to `1` only
when the prefix is leaked to a lower level (RFC 2966). Ze does not fabricate an
IPv4 external marking the protocol lacks. Routes redistributed *out* of IS-IS to
BGP travel the redistribute orchestrator, not the wire encoding.

**TLV 236 (IPv6, RFC 5308 sec 2):**

| Field | Size | Notes |
|-------|------|-------|
| Metric | 4 octets | 32-bit |
| Flags | 1 octet | U up/down `0x80`, X external `0x40`, S sub-TLV-present `0x20`, 5 reserved bits |
| Prefix length | 1 octet | 0..128 |
| Prefix | `ceil(len/8)` octets | packed |
| Sub-TLV length | 0 or 1 octet | present **only** when S is set |
| Sub-TLVs | variable | present only when S is set |

> RFC 5308 sec 2 draws the flags octet as `U|X|S|Reserve(5)` MSB-first, so the
> high bits in order are U `0x80`, X `0x40`, S `0x20`. The codec follows the RFC
> bit order (pinned by `TestISISTLVIPv6FlagBits`) so Ze and an interop peer (FRR)
> agree on the external / sub-TLV-present bits.

## LSP origination, aging, and fragmentation (isis-6)

The LSDB (`internal/plugins/isis/lsdb/`) stores every LSP per Ze's
buffer-first model: the verbatim PDU bytes (a single owned copy, never an alias
of the receive buffer) plus parsed freshness metadata (LSP ID, sequence,
remaining lifetime, checksum, overload). TLVs are parsed lazily, so an LSP
carrying an unknown TLV re-floods byte-for-byte (ISO/IEC 10589 clause 7.3.14).

**Own-LSP contents.** The node originates its own L1 and/or L2 LSP set by full
regeneration on any topology change (clause 7.3.12). Fragment 0 carries the
non-fragmentable TLVs and the overload bit:

| TLV | Source |
|-----|--------|
| 1 (Area Addresses) | configured NETs |
| 129 (Protocols Supported) | NLPID `0xCC` (IPv4), `0x8E` (IPv6) when the family is enabled |
| 132 (IP Interface Address, RFC 1195) | the node's own IPv4 interface addresses (SPF next-hop source) |
| 232 (IPv6 Interface Address, RFC 5308) | the node's own **non-link-local** IPv6 interface addresses (LSP scope); IPv6 enabled only |
| 137 (Dynamic Hostname, RFC 5301) | configured hostname |
| 22 (Extended IS Reachability, RFC 5305) | each Up adjacency, 24-bit wide metric |
| 135 (Extended IP Reachability, RFC 5305) | connected/redistributed IPv4 prefixes, 32-bit metric |
| 236 (IPv6 Reachability, RFC 5308) | connected/redistributed IPv6 prefixes, 32-bit metric; IPv6 enabled only |

Wide metrics only (RFC 5305 / RFC 5308): TLV 22 is 24-bit, TLV 135 / 236 are
32-bit. The overload (OL) bit (RFC 3787 sec 4) is set **only in the
non-pseudonode LSP fragment 0**, never in higher fragments.

**IPv6 address scope (RFC 5308 origination, isis-12).** The IPv6 interface-address
TLV (232) is scoped by PDU: the **Hello (IIH)** carries **only** the
**link-local** (`fe80::/10`) address (so a neighbour learns the IPv6 next-hop),
while the **LSP** carries **only** the **non-link-local** addresses (sec 3). The
IPv6 Reachability TLV (236) **never** advertises a link-local prefix (sec 2).
Redistributed IPv6 prefixes set the TLV 236 **external (X)** bit (sec 2); the
up/down bit is set only on a down-level leak (RFC 2966), exactly as IPv4 TLV 135.

**Sequence numbers and wraparound.** Sequence numbers start at 1 (0 is reserved,
never a purge) and increment monotonically on every (re)origination and refresh
(clause 7.3.16.1). At `0xFFFFFFFF` the LSP ID is purged and its re-origination is
suspended for MaxAge + ZeroAgeLifetime, after which it re-originates from 1.

**A claim on an own LSP ID.** An LSP bearing this node's own System ID, arriving
at a sequence the node did not issue, is **never stored** (clause 7.3.16.4 c-1:
"shall not overwrite with the received LSP"). The node instead raises its own
sequence **above** the claimed one and re-originates, arming SRM on every eligible
circuit (clause 7.3.16.4 c-2..c-4, deferring to clause 7.3.16.1). This covers a
neighbour's **purge** of the node's own LSP, a higher sequence returned by a
system that outlived the node's last incarnation, and the equal-sequence
differing-checksum case (clause 7.3.16.2, LSP confusion). A claim BELOW the
node's own sequence is not answered: the ordinary flood of the copy the node
already holds corrects the sender (clause 7.3.16.4 b-3). Neither is a claim on an
LSP ID the node does not originate, nor a REPEAT of a claim already answered (one
answer per claim, so a retransmission cannot drive a sequence-bump storm). A
claim AT the maximum sequence falls into the wraparound path above, so raising a
sequence can never defeat the suspension, and a claim raised during a suspension
neither shortens it nor survives it.

When the node holds NO copy of the claimed LSP ID, clause 7.3.16.4 a) governs
instead: the arrival is acknowledged and not retained. The SSN flag cannot carry
that acknowledgement (it lives on an LSDB entry, and there is none), so it is
queued and the next PSNP carries it at the ARRIVED sequence. An acknowledgement
echoes what was received; a REQUEST goes out at sequence 0 so the holder reads it
as older and supplies the LSP.

**An own LSP's stored checksum comes from its own bytes.** `LSP.WriteTo` fills the
struct with the pre-signature checksum and `SignPDU` recomputes it inside the
bytes, so anything that stores or advertises an own LSP's checksum reads it back
from the encoded PDU (`packet.LSPChecksumOf`). A divergence there makes the CSNP
this node sources advertise a value no receiver can reproduce, which clause
7.3.16.2 turns into a purge of this node's own LSP.
<!-- source: internal/plugins/isis/lsdb/origination.go -- RaiseSequenceFloor, encodeAndSign -->
<!-- source: internal/plugins/isis/lsdb/lsdb.go -- ownConflictResult -->
<!-- source: internal/plugins/isis/lsdb/snp.go -- recordAckOnly, drainAckOnly -->
<!-- source: internal/plugins/isis/own_lsp_conflict.go -- reoriginateAboveClaim -->

**Aging and purge.** Remaining Lifetime decrements once per second (clause
7.3.16.4). At 0 the LSP becomes a **purge**: it is re-flooded (isis-7) and
retained in the LSDB for the ZeroAgeLifetime grace period, then garbage-collected
- it is **not** deleted at the instant lifetime hits 0 (clause 7.3.16/17). A
purge that arrives on the wire and a local expiry are retained identically but
handled by distinct paths (a received purge is re-flooded; a local expiry is
collected). Defaults: MaxAge 1200 s, refresh 900 s, ZeroAgeLifetime 60 s.

**Fragmentation.** When the own state exceeds the max LSP size (the smallest
circuit MTU, default 1492), it is split across LSP numbers 0..255 (the 256
-fragment model; RFC 3786 extended fragments are out of scope for v1). Each
fragment is a distinct LSP with its own sequence number and Fletcher checksum; a
single TLV entry (a TLV 22 neighbour or a TLV 135 prefix) is never split across
fragments. Fragment 0 always exists and carries the non-fragmentable fields.

**Flooding flags.** The LSDB holds per-LSP, per-circuit SRM (Send Routeing
Message) and SSN (Send Sequence Number) flags (clause 7.3.4/7.3.5); origination
arms SRM on every eligible circuit. The LSDB only stores and exposes the flags;
the flooding child (isis-7) drives them and performs the transmission.

### Reliable flooding and SNP synchronisation (isis-7)

The flooding child (`internal/plugins/isis/lsdb/flooding.go`, `snp.go`) is the
flag-to-wire pump. It owns no LSDB storage: it consumes the freshness compare and
the SRM/SSN flag API of isis-6 and drives them over the isis-3 transport. LSP,
CSNP, and PSNP PDUs arrive via the isis-4 PDU receive dispatcher (keyed by the
5-bit PDU type); flooding registers handlers there rather than holding its own
switch.

**On LSP receipt (ISO/IEC 10589 clause 7.3.14-16).** The freshness compare lives
in isis-6 (`LSDB.Receive`); isis-7 maps the outcome to flags. Higher sequence:
accept, replace the stored bytes, set SRM on every circuit *except* the one it
arrived on (never re-flood onto the incoming circuit), set SSN on the incoming
circuit. Equal sequence and equal checksum: a duplicate (a LAN circuit sets SSN
to acknowledge). Equal sequence, differing checksum: keep our copy and set SSN on
the incoming circuit to request the authoritative copy via PSNP. Lower sequence:
set SRM on the incoming circuit to send our newer copy back. The remaining-
lifetime tier is applied *before* the checksum: a received purge (remaining
lifetime 0) at the same sequence as a held non-zero copy is more recent, so it is
accepted, marked purged, and re-flooded (clause 7.3.16.1).

**Periodic flood (SRM timer, ~5 s).** For each LSP with SRM set on a non-passive
circuit, the stored raw LSP bytes are re-transmitted verbatim (unknown TLVs
preserved, clause 7.3.14). On a point-to-point circuit SRM is cleared on send (the
CSNP-at-Up plus periodic PSNP reconcile any loss); on a broadcast circuit SRM is
left set until an acknowledgement is observed, so an un-acked LSP resends on the
next tick.

**CSNP / PSNP.** A CSNP advertises the sender's whole LSDB as TLV 9 (LSP Entries)
over a Start..End LSP-ID range (one PDU for the common case, split into ordered,
contiguous ranges for a large database). On receipt, each listed entry is
compared: an entry the neighbour holds newer than ours (or that we do not hold at
all) is requested; an entry we hold newer is sent (SRM); an equal entry clears
SRM (an implicit ack). Because an SSN flag can only sit on an LSP we already hold,
a request for an LSP we do *not* hold is recorded in a per-circuit pending-request
set, drained into PSNP requests, and cleared when the LSP arrives. A PSNP also
acknowledges held LSPs (from the SSN flags, cleared as the ack is built). On a P2P
circuit an initial CSNP is sent the moment the adjacency reaches Up to synchronise
the two databases fast; the LAN periodic-CSNP cadence is DIS policy (isis-8).

### DIS election and pseudo-node LSPs (isis-8)

On a **broadcast** (multi-access LAN) circuit one IS per level is elected the
**Designated IS** (DIS) (ISO/IEC 10589 clause 8.4.5). Election compares
`(priority, MAC)`: the highest DIS priority (0..127, from the LAN IIH fixed
header) wins, and an equal priority is broken by the higher source MAC (SNPA).
Priority 0 is valid (lowest preference); it does not forbid winning. Election runs
**independently per level**, so a node may be DIS for L1, L2, both, or neither on
the same circuit. A point-to-point circuit has no DIS and no pseudo-node.

The elected DIS originates a **pseudo-node LSP** — a virtual node representing the
LAN. Its LSP ID carries a **non-zero pseudonode octet** (the LAN ID
`<dis-system-id>.<pseudonode-id>`; the pseudonode octet is 0 only for a real
router's LSP). The pseudo-node LSP lists every router on the segment (the DIS
included) as a single Extended IS Reachability (TLV 22) entry at **metric 0** — a
virtual node with zero-cost edges to its members. It carries no TLV 1/129/132/137
(a pseudo-node has no area, protocols, address, or hostname of its own). It is an
ordinary LSP: the same origination, freshness, flooding, aging, and fragmentation
rules apply (a many-member LAN splits the pseudo-node across LSP numbers 0..255).

Every router on the segment (the DIS and each non-DIS router) then advertises the
LAN as a **single** TLV 22 entry pointing at the pseudo-node (metric = its circuit
metric) in its **own** LSP, *instead of* one entry per peer. This collapses the
`N*(N-1)` mesh into a star with the pseudo-node at the centre.

On a role change the DIS state is kept consistent: a router that **loses** the DIS
role while still present **purges** (zero Remaining Lifetime, bumped sequence) the
pseudo-node LSP it originated before yielding, so no phantom node lingers in
another router's SPF. A new DIS originates its own pseudo-node. Election is
**damped** (a candidate change must persist a short window before the role
transfers), so a flapping LAN does not churn the pseudo-node LSP. While DIS, the
node also sources the periodic **LAN CSNP** (clause 7.3.15.2) from the pseudo-node
Source ID to keep the segment synchronised.

## Authentication (isis-10)

The Authentication TLV (type 10) carries a 1-octet authentication type followed by
an algorithm-dependent value. Ze emits and accepts three types:

| Auth type | Code | Standard | TLV 10 value |
|-----------|------|----------|--------------|
| Cleartext | 1 | ISO/IEC 10589 | the password (sanity only) |
| HMAC-MD5 | 54 | RFC 5304 | 16-octet digest |
| Generic crypto | 3 | RFC 5310 | 2-octet Key ID + HMAC-SHA digest (20/28/32/48/64 octets) |

(The clean-room research guide lists the MD5 and generic-crypto codes swapped; the
RFCs are authoritative — HMAC-MD5 is 54, generic crypto is 3.)

**TLV 10 first.** RFC 5304 sec 1 requires the Authentication TLV to be the **first**
TLV in the PDU. Ze emits it first on sign and rejects on decode a PDU whose TLV 10
is present but not first.

**Field zeroing before the digest** (RFC 5304 sec 2) depends on the PDU class:

- **All PDUs:** the Authentication Value octets inside TLV 10 are set to zero.
- **LSPs additionally:** the LSP **Checksum** and **Remaining Lifetime** fields are
  set to zero. (Zeroing Remaining Lifetime is what lets a transit router age an LSP
  without re-signing it.)
- IIH / CSNP / PSNP have no checksum or lifetime field, so only the Authentication
  Value is zeroed.

**Send order:** build the PDU **with** the Padding TLV (8, IIH only) → insert and
sign TLV 10 → compute the Fletcher checksum (LSPs only) **last**, so the digest is
already in place when the checksum runs. The transport then adds only 802.3+LLC
framing and never alters the PDU bytes. **Receive order:** accept the checksum,
save the Authentication Value + Checksum + Remaining Lifetime, zero them, recompute
the digest, restore the saved fields, and constant-time compare (`hmac.Equal`)
against every currently-valid key in the relevant chain.

**Authenticated purges.** A purge (zero Remaining Lifetime) is signed like any
LSP, but with the body stripped to **only** TLV 10 (RFC 5304 sec 2: an originator
that purges "MUST remove the body of the LSP and add the authentication TLV"). On
receive, an unauthenticated purge, or a purge that carries any TLV other than
TLV 10, is **rejected** — preventing a router without the key from spoofing a purge
by zeroing a captured LSP's lifetime and re-flooding it.

The structural TLV 10 codec is in `tlv_auth.go` (isis-2); the sign/verify backend,
field zeroing, TLV-first enforcement, and constant-time compare are in
`packet/auth_verify.go`, with the key store and per-PDU-class chain selection in
the `isis` component (`auth_keystore.go`, `auth_wiring.go`).

## Offline decode tool

`ze isis decode` reads one IS-IS PDU from stdin and prints a JSON view on
stdout. It accepts both ASCII hex (a pasted/captured `831b...` string) and raw
PDU bytes (an IS-IS PDU starts with `0x83`, which is not an ASCII hex digit, so
the two are unambiguous). This is a thin caller over `packet.DecodePDU` +
`PDU.ToJSON` that proves the codec wires end-to-end; the running-daemon CLI
surface (`show isis ...`) is owned by isis-13.

```
$ printf '831b01060f...' | ze isis decode --pretty
{ "type": "l1-lan-hello", "lan-hello": { "system-id": "0000.0000.0001", ... } }
```

Functional coverage: `test/isis-wire/isis-pdu-1.ci` (a captured LAN L1 Hello)
and `test/isis-wire/isis-truncated.ci` (malformed input is rejected). Run with
`make ze-isis-wire-test`.

## Tests

| Concern | Test |
|---------|------|
| PDU type constants | `TestISISPDUConstants` |
| Common header round-trip + rejection | `TestISISHeaderRoundTrip`, `TestISISHeaderRejects` |
| Fletcher checksum vectors + corruption | `TestISISChecksumVectors`, `TestISISChecksumDetectsCorruption` |
| PDU round-trips (all 9) | `TestISISLANIIHRoundTrip`, `TestISISP2PIIHRoundTrip`, `TestISISLSPRoundTrip`, `TestISISCSNPRoundTrip`, `TestISISPSNPRoundTrip` |
| TLV round-trips | `TestISISTLV*` (core, neighbors, IPv4, IPv6, auth) |
| Unknown-TLV passthrough | `TestISISUnknownTLVPassthrough` |
| Fuzz (no panic) | `FuzzISISDecodePDU`, `FuzzISISTLVIterator`, `FuzzISISRoundTrip` |
