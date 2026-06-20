# Spec: ospf-7-lsdb-flooding

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-6-neighbor-nsm.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - Shared Contracts (LSA inventory, LSA header + body layout, Metrics table row "ospf-7"), Design Principles (lazy/buffer-first LSDB, per-area LSDB), Architecture (`lsdb/`)
4. `docs/research/ospf-implementation-guide.md` sec 3 (LSDB layout, MaxAge/MaxAgeDiff/LSRefreshTime, purge), sec 5e (flooding §13, retransmission, acknowledgement), and traps 1/2/3/13/14
5. `plan/spec-isis-6-lsdb.md` + `plan/spec-isis-7-flooding.md` - the IS-IS LSDB + flooding siblings; this spec combines their patterns (lazy raw-bytes store, origination by regeneration, freshness-driven flag pump, purge retention) for OSPF
6. `plan/spec-ospf-2-wire.md` - LSA header + body codec, Fletcher-16 LSA checksum, LS Update / LS Ack packet codec
7. `plan/spec-ospf-6-neighbor-nsm.md` - the neighbour FSM that gates flooding (Full neighbours get the retransmit list; LS Request drain produces the first LS Updates)

## Task

Implement the per-area Link-State Database (LSDB), self-LSA origination, and the
RFC 2328 §13 flooding procedure for OSPFv2 in the `internal/component/ospf/lsdb/`
package. This is phase 7 of the OSPF spec set (`plan/spec-ospf-0-umbrella.md`).
The LSDB is the in-memory record of every Link-State Advertisement known to the
node. Following the umbrella Design Principle "Lazy / buffer-first LSDB" and the
IS-IS sibling (`plan/spec-isis-6-lsdb.md`), each LSA is stored as its on-wire
byte slice plus a thin parsed metadata header (LSAKey = (LS Type, Link State ID,
Advertising Router), LS Sequence Number, LS Age, LS Checksum, Length); the LSA
body is parsed only when SPF (ospf-8) or the CLI needs it. Per the umbrella
"Per-area LSDB" principle, each area keeps its own LSDB; Type 5 AS-External LSAs
live in an AS-wide store shared across all areas.

This spec owns three responsibilities:

- **Self-origination.** Build this router's own Router-LSA (Type 1) from live
  interface and neighbour state (link records: point-to-point, transit via the
  DR, stub network; the V/E/B flag byte), and originate a Network-LSA (Type 2)
  for each segment where this router is the DR (network mask + the Router IDs of
  all fully-adjacent attached routers including self). Re-originate on
  interface / neighbour / cost change and on the LSRefreshTime timer.
  Stub-router / max-metric origination (RFC 6987): set the metric of all
  non-stub Router-LSA links to LSInfinity (0xFFFF) so the node is reachable but
  not transited.
- **The §13 flooding procedure.** On receiving an LSA in an LS Update, run the
  §13.1 freshness comparison against the local copy, install/replace it in the
  LSDB, flood it out the OTHER interfaces, and send it to neighbours via
  per-neighbour retransmit lists (resent every RxmtInterval until acknowledged).
  Acknowledge received LSAs implicitly (the re-flood out the receiving interface
  is the ack) or explicitly (direct unicast ack, or delayed ack coalesced to the
  interface multicast address per §13.5 Table 19). Rate-limit acceptance with
  MinLSArrival and re-origination with MinLSInterval.
- **Ageing and purge.** A once-per-second MaxAge (3600 s) walker ages every LSA;
  an LSA reaching MaxAge is a purge, flooded like any other LSA and **retained**
  in the LSDB until acknowledged by all neighbours, then removed (trap #3).
  Self-originated LSAs are refreshed at LSRefreshTime (1800 s). LS Sequence
  Number wraparound at MaxSequenceNumber (0x7FFFFFFF) flushes the LSA as a MaxAge
  purge, then re-originates at InitialSequenceNumber (0x80000001) (trap #2). Each
  refresh recomputes the Fletcher checksum in the origination path (trap #13).

Ze has no OSPF today; this package is entirely new. It depends on
`spec-ospf-6-neighbor-nsm` (the neighbour FSM gates flooding: only Full
neighbours carry retransmit lists, and the LS Request drain at Loading produces
the first LS Updates), the `spec-ospf-2-wire` codec (LSA header / body and LS
Update / LS Ack encode/decode plus the Fletcher checksum), the
`spec-ospf-1-types` domain types (LSAKey, LSSequenceNumber, LSAge, Metric), the
`spec-ospf-5-interface-ism` interface + DR/BDR state that feeds Router-LSA link
records and Network-LSA origination, and the `spec-ospf-4-component-config`
packet receive dispatcher (LS Update type 4, LS Ack type 5). It exposes a
`show ip ospf database` snapshot consumed and rendered in
`spec-ospf-13-cli-diag-interop`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` sec 3 (LSDB Layout; MaxAge, Purge, Refresh) - LSDB data model, the three aging timers, purge retention
  → Decision: store each LSA as its on-wire byte slice plus a thin header struct (LSAKey, sequence, checksum, length, age countdown); parse the body only when SPF or the CLI needs it (lazy over eager) — flooding a Type 5 to two neighbours and recomputing a checksum on purge need no body parse
  → Constraint: per-type tables per area keyed on (LS Type, Link State ID, Advertising Router); Type 5 lives in an AS-wide table on the instance; references come from the LSDB itself, per-neighbour retransmit lists, per-neighbour LS Request lists, and delayed-ack lists
  → Constraint: MaxAge 3600 s (a purge), MaxAgeDiff 900 s (freshness equivalence window), LSRefreshTime 1800 s (re-flood own LSAs); a purged LSA is kept until every neighbour has acknowledged it, then deleted (§14.1)
- [ ] `docs/research/ospf-implementation-guide.md` sec 5e (Flooding, Retransmission, Acknowledgement) - the §13 procedure, per-neighbour retransmit lists, ack rules (§13.5 Table 19), max-age purge flooding
  → Decision: drive transmission from per-neighbour retransmit lists and per-interface delayed-ack lists; the §13.1 comparison decides install/replace/ack/send-back
  → Constraint: flood out every interface EXCEPT the receiving one; queue on each Full neighbour's retransmit list; resend every RxmtInterval; an ack removes the LSA from the list; on broadcast where we are DR/BDR send to AllSPFRouters, where DROther send to AllDRouters; P2P uses AllSPFRouters
  → Constraint: §13.5 acks — DR-sourced LSA we are BDR for: delay; duplicate already on our retransmit list: direct ack; newer LSA we re-flooded back out the receiving interface: suppress (implicit ack); otherwise delay. Delayed-ack flush is shorter than RxmtInterval
  → Constraint: a purge (age = MaxAge) is flooded like any LSA but receivers keep it (still MaxAge) until every retransmit list is cleared, then delete (delete-too-early lets a late neighbour re-inject the stale copy)
- [ ] `docs/research/ospf-implementation-guide.md` traps 1/2/3/13/14 - the hard correctness edges
  → Constraint: trap #1 Fletcher-16 over the LSA from the Options field (excludes LS Age, includes the checksum position), RFC 905 test vectors, encode AND decode (self-interop passes while cross-interop fails when only one side is right)
  → Constraint: trap #2 sequence compare — signed 32-bit; MaxSequenceNumber (0x7FFFFFFF) is newest; wraparound is handled by purge-then-re-originate at InitialSequenceNumber (0x80000001), never a naive "higher is newer"
  → Constraint: trap #3 max-age purge retention — retain + re-flood a MaxAge LSA until every neighbour has acked or is no longer Full; delete only then
  → Constraint: trap #13 checksum refresh on re-origination — recompute the Fletcher checksum in the origination path when the sequence number increments, not opportunistically
  → Constraint: trap #14 clock vs monotonic — use a monotonic clock for RxmtInterval and aging; compute LS Age as a delta from a monotonic origination timestamp, not wall-clock `time()`
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, lazy parse, no-alloc encode
  → Constraint: LSA encode is `WriteTo(buf, off) int` into a pooled buffer; never `buildLSA() []byte`; the Fletcher checksum is backfilled at its fixed header offset after the body is written; flood re-transmits the stored raw LSA bytes (with LS Age incremented per §13.3 InfTransDelay) rather than re-encoding
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md` - self-contained component, registration not switch
  → Constraint: all LSDB / origination / flooding / aging code stays under `internal/component/ospf/lsdb/`; LS Update / LS Ack handlers register with the ospf-4 packet receive dispatcher rather than holding a packet-type switch
- [ ] `plan/spec-ospf-0-umbrella.md` - Shared Contracts (LSA inventory, LSA header + body layout, Metrics canonical), Design Principles, Architecture (`lsdb/`)
  → Constraint: package is `internal/component/ospf/lsdb/`; per-area LSDBs plus one AS-wide Type 5 store; the LSA header / body layout in the umbrella is authoritative; this spec owns exactly the eight `ze_ospf_lsdb_*` / `ze_ospf_lsa_*` / `ze_ospf_lsupdates_*` / `ze_ospf_lsacks_sent_total` / `ze_ospf_retransmissions_total` metric rows and no others

### RFC Summaries (MUST for protocol work; created via `/ze-rfc` at implementation time)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2 base: §12 LSA header, §13 flooding (incl. §13.1 freshness, §13.3 flood-out, §13.5 ack Table 19, §13.7 self-originated), §14 aging (MaxAge/MaxAgeDiff/LSRefreshTime), §12.1.6 LSInfinity, MinLSArrival, MinLSInterval, InitialSequenceNumber, MaxSequenceNumber
  → Constraint: §13.1 a received LSA is more recent if (a) higher LS sequence number; else (b) higher LS checksum; else (c) if exactly one has age MaxAge that one is more recent; else (d) if ages differ by more than MaxAgeDiff the smaller LS age is more recent; else the two are functionally equivalent
  → Constraint: §13.4 a self-originated LSA received with a higher sequence than ours forces us to re-originate at one greater (or, if we no longer wish to originate it, flush via MaxAge); §13.7 self-originated MaxAge LSAs are flushed
  → Constraint: §14 InitialSequenceNumber 0x80000001, MaxSequenceNumber 0x7FFFFFFF; at MaxSequenceNumber prematurely age to MaxAge, flush, and re-originate at InitialSequenceNumber once flushed; MinLSArrival 1 s (reject a received instance arriving sooner); MinLSInterval 5 s (rate-limit own re-origination)
- [ ] `rfc/short/rfc905.md` - Fletcher-16 checksum (Annex), RFC 905 test vectors (created by ospf-1)
  → Constraint: LSA re-origination and refresh recompute the Fletcher-16 checksum over the LSA from the Options field onward (LS Age excluded); reuse the ospf-1 algorithm, do not re-implement
- [ ] `rfc/short/rfc6987.md` - OSPF Stub Router Advertisement (max-metric router-lsa)
  → Constraint: when stub-router / max-metric is configured (or during a configurable startup window), originate the Router-LSA with all non-stub link metrics set to LSInfinity (0xFFFF); stub-network link metrics keep their real cost so locally-attached prefixes stay reachable

**Key insights:** (minimal context to resume after compaction)
- LSDB = per-area tables (Router/Network/Summary) + one AS-wide Type 5 table, each entry keyed by LSAKey = (LS Type, Link State ID, Advertising Router); entry = raw LSA bytes + metadata (sequence, age origin timestamp, checksum, length)
- §13.1 freshness order: higher sequence → higher checksum → the MaxAge rule → within MaxAgeDiff the lower age → else equivalent
- Flooding: install, flood out OTHER interfaces, queue on Full neighbours' retransmit lists (resend every RxmtInterval until acked); ack implicitly (re-flood out the receiving interface) or explicitly (direct/delayed per §13.5 Table 19); MinLSArrival gates acceptance, MinLSInterval gates own re-origination
- Origination: Router-LSA from interface/neighbour state; Network-LSA when DR; re-originate on change and at LSRefreshTime; max-metric (RFC 6987) sets non-stub metrics to LSInfinity
- Purge: a MaxAge LSA is retained until every neighbour acks, then deleted (trap #3); sequence wraparound flushes then re-originates at InitialSequenceNumber (trap #2); refresh recomputes the Fletcher checksum (trap #13); use a monotonic clock for age and RxmtInterval (trap #14)

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; ospf-1..ospf-6 are sibling specs feeding this one)
- [ ] Ze has no OSPF protocol; there is no LSDB, no LSA origination, and no flooding today. The closest in-tree precedent is the IS-IS LSDB (`internal/component/isis/lsdb/`) — a lazy raw-bytes store with origination by regeneration, a 1 s aging tick, zero-age purge retention, and a flag-driven flood pump
  → Constraint: this package is created wholesale; it copies the IS-IS LSDB/flooding above-the-wire patterns but shares no code (OSPF has network vertices / Network-LSAs, per-neighbour retransmit lists, and a different freshness rule)
- [ ] `internal/component/bgp/wireu/wire_update.go` - BGP `WireUpdate` lazy/zero-copy model: raw bytes held, fields parsed on demand
  → Constraint: mirror it for the LSDB entry — store raw LSA bytes, parse the body lazily; never eager-parse into structs
- [ ] ospf-2 (sibling) owns the LSA header / body codec and the Fletcher checksum; ospf-5 (sibling) owns interface state + DR/BDR identity; ospf-6 (sibling) owns the neighbour FSM, the LS Request list, and the dispatcher delivery of LS Update / LS Ack
  → Constraint: this spec reads the ospf-2 parsed LSA header for freshness metadata, reads ospf-5 interface/DR state for origination, and is driven by ospf-6 neighbour transitions (retransmit list lives per Full neighbour); it does not redefine any of those

**Behavior to preserve:**
- BGP, LDP, RSVP-TE, IS-IS, static, connected route sources remain independent and functional (the OSPF LSDB does not touch them)
- The ospf-5 interface/DR state and ospf-6 neighbour FSM are consumed read-only by origination and flooding; no change to their semantics
- The ospf-2 LSA / packet codec and ospf-1 Fletcher checksum are consumed, not reimplemented

**Behavior to change:**
- None pre-existing. New `internal/component/ospf/lsdb/` package: per-area + AS-wide LSDB store, Router/Network-LSA origination (incl. RFC 6987 max-metric), the §13 flooding procedure (retransmit lists, delayed/direct acks, MinLSArrival/MinLSInterval), the MaxAge aging walker + purge retention, LSRefresh, sequence wraparound, and the `show ip ospf database` snapshot

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- LS Update packet received on an interface (from the ospf-4 packet receive dispatcher, type 4) carrying one or more LSAs
- LS Ack packet received on an interface (type 5) carrying LSA headers acknowledging LSAs on a neighbour's retransmit list
- Interface / neighbour / cost change events (from ospf-5 ISM and ospf-6 NSM) triggering self-LSA (re-)origination
- DR/BDR election result (ospf-5) gaining or losing DR status on a segment, triggering Network-LSA origination or flush
- Timer ticks: the 1 s MaxAge aging walker, the per-neighbour RxmtInterval retransmit timer, the per-interface delayed-ack flush timer, and the LSRefreshTime / MinLSInterval origination timers
- A `show ip ospf database` request (from ospf-13 CLI)

### Transformation Path
1. **LS Update receive:** the ospf-4 dispatcher hands `(ifindex, src, payload)` to the LS Update handler → for each LSA: validate the Fletcher checksum and known LS Type (ospf-2), drop a Type 5 received into a stub/NSSA area → look up the LSAKey in the per-area (or AS-wide for Type 5) LSDB → §13.1 freshness compare against the local copy. Received strictly newer (and not arrived sooner than MinLSArrival): install/replace the raw bytes + metadata, set it on the retransmit list of every Full neighbour on every interface EXCEPT the receiving one, flood out those interfaces, and arrange an ack to the sender (implicit re-flood, direct, or delayed per §13.5 Table 19). Received identical: treat as an implied/explicit ack (remove from the sender's retransmit list if present) and possibly send a delayed ack. Received older: send our newer copy directly back to the sender. Self-originated received with a higher sequence than ours (§13.4): re-originate at one greater, or flush via MaxAge if we no longer wish to originate it.
2. **LS Ack receive:** for each acknowledged LSA header, remove the matching LSA from that neighbour's retransmit list; when a neighbour's retransmit list drains, stop its RxmtInterval timer; an acknowledged MaxAge purge that is now acked by all neighbours is deleted from the LSDB.
3. **Self-origination:** interface/neighbour/cost change or LSRefreshTime → build the Router-LSA from ospf-5 interface state (link records: P2P, transit-via-DR, stub network; V/E/B flags) and, where this router is DR, the Network-LSA (mask + attached fully-adjacent Router IDs incl. self); assign the next LS Sequence Number; recompute the Fletcher checksum; install in the LSDB; set it on every eligible Full neighbour's retransmit list and flood it. MinLSInterval rate-limits re-origination of the same LSA. RFC 6987 max-metric sets non-stub link metrics to LSInfinity.
4. **Flood out interfaces:** for each interface except the receiving one, walk its neighbours: Exchange/Loading neighbours are checked against their database-summary / LS Request lists to avoid redundant flooding; Full neighbours get the LSA queued on the retransmit list; transmit one LS Update to AllSPFRouters (DR/BDR on broadcast, or P2P) or AllDRouters (DROther on broadcast). LS Age is incremented by InfTransDelay before transmission (§13.3).
5. **Retransmit:** a per-neighbour RxmtInterval timer resends every LSA still on the neighbour's retransmit list (the stored raw bytes, age re-incremented); acks remove entries; a neighbour leaving Full empties its list.
6. **Aging:** the 1 s walker increments every LSA's LS Age (computed as a delta from its monotonic origination timestamp); at MaxAge the LSA is a purge — flooded as a purge and **retained** until acked by all neighbours, then deleted; a self-originated LSA reaching LSRefreshTime is re-originated (new sequence, fresh checksum, age reset).
7. **Wraparound:** an own LSA whose sequence reaches MaxSequenceNumber is prematurely aged to MaxAge, flushed, and re-originated at InitialSequenceNumber once the flush is acknowledged.
8. **Snapshot:** `show ip ospf database` → a read-locked copy of per-area and AS-wide metadata (and lazily parsed body summaries) grouped by LS Type.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Dispatcher (ospf-4) ↔ flooding | LS Update / LS Ack handlers register with the ospf-4 packet receive dispatcher; receive `(ifindex, src, payload)` | [ ] |
| Wire codec (ospf-2) ↔ LSDB | parsed LSA header (key, sequence, age, checksum, length) + verbatim raw LSA bytes; Fletcher checksum recompute on origination | [ ] |
| Interface/DR (ospf-5) ↔ origination | read-only interface state + DR/BDR identity → Router-LSA link records and Network-LSA origination | [ ] |
| Neighbour FSM (ospf-6) ↔ flooding | per-Full-neighbour retransmit list; LS Request list drained to first LS Updates; neighbour leaving Full empties its retransmit list | [ ] |
| Transport (ospf-3) ↔ flooding | TX of LS Update / LS Ack to the multicast / unicast destination on a named interface (no RX coupling; receive arrives via the ospf-4 dispatcher) | [ ] |
| LSDB ↔ SPF (ospf-8) | lazy body parse of stored LSAs to build the SPF graph | [ ] |
| LSDB ↔ CLI (ospf-13) | `show ip ospf database` snapshot | [ ] |

### Integration Points
- `internal/component/ospf/lsdb/lsdb.go` - the per-area + AS-wide store, freshness compare, snapshot (new)
- `internal/component/ospf/lsdb/origination.go` - Router-LSA / Network-LSA build, sequence assignment, max-metric (new)
- `internal/component/ospf/lsdb/flooding.go` - §13 receive + flood-out + retransmit + ack (new)
- `internal/component/ospf/lsdb/aging.go` - MaxAge walker, purge retention, LSRefresh, wraparound (new)
- ospf-4 packet receive dispatcher - LS Update / LS Ack handlers register here
- ospf-5 interface + DR state - read for origination
- ospf-6 neighbour FSM + retransmit-list scope + LS Request list - drives flooding
- ospf-8 (SPF) reads the LSDB (lazy body parse); ospf-9/10/11 originate Summary/External/NSSA LSAs INTO this store reusing the same model

### Architectural Verification
- [ ] No bypassed layers (frames → ospf-3 RX → ospf-4 dispatcher → flooding handler → LSDB / retransmit list; TX → ospf-3; origination → ospf-2 encode → LSDB)
- [ ] No unintended coupling (LSDB independent of BGP/BGP-LS; flooding does not import SPF/redistribute; transport/wire unaware of flooding; no packet-type switch outside ospf-4)
- [ ] No duplicated functionality (uses the ospf-2 codec + ospf-1 Fletcher checksum; does not reimplement LSA encode/parse or the checksum)
- [ ] Zero-copy preserved (entries hold raw bytes; body parse on demand; flood re-transmits stored raw bytes with only the LS Age incremented; encode is buffer-first `WriteTo`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The ospf-2 codec exposes a metadata-only LSA header parse (key, sequence, age, checksum, length) plus a full body parse, so the LSDB reads freshness fields without parsing the body | umbrella "LSA header + body layout"; buffer-first lazy model | LSDB parses the full body on every receive (defeats lazy storage) | ospf-2 codec API review; unit test parsing the header only | unvalidated |
| A-2 | ospf-5 exposes interface state + DR/BDR identity + per-interface fully-adjacent neighbour set as a read-only snapshot at origination time | umbrella dependency (ospf-7 depends on ospf-6 which depends on ospf-5) | Router/Network-LSA origination needs a different access pattern | origination unit test against a fake interface/neighbour table | unvalidated |
| A-3 | ospf-6 exposes the per-neighbour state (Full/Exchange/Loading), the per-neighbour LS Request list, and a place to attach a per-neighbour retransmit list | umbrella ospf-6 scope (NSM, LS Request list) | flooding must store its own neighbour bookkeeping, blurring the ospf-6/ospf-7 split | ospf-6 API review; flooding wiring test | unvalidated |
| A-4 | MaxAge, MaxAgeDiff, LSRefreshTime, RxmtInterval, MinLSArrival, MinLSInterval, InitialSequenceNumber, MaxSequenceNumber, LSInfinity are RFC-fixed constants (RxmtInterval is a per-interface config leaf resolved by ospf-4) | RFC 2328 §C / umbrella interface config (`retransmit-interval`) | timers hardcoded where they should be tunable, or vice versa | config resolve check; aging/retransmit tests with injected short timers | unvalidated |
| A-5 | Re-transmitting the stored raw LSA bytes verbatim (only LS Age adjusted) is RFC-correct and preserves the Fletcher checksum (which excludes LS Age) | research sec 5e + trap #1 (Fletcher excludes LS Age) | must re-encode from parsed form, risking checksum/byte drift | round-trip + flood test asserting byte-identical re-flood except LS Age | unvalidated |
| A-6 | A Type 5 received into a stub/NSSA area can be identified and dropped using the receiving interface's area type from ospf-5 | umbrella LSA inventory ("Type 5 not flooded into stub/NSSA") | stub/NSSA flooding leaks Type 5 LSAs | flooding test injecting a Type 5 on a stub-area interface | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | §13.1 freshness compare implemented wrong (skips the checksum or MaxAgeDiff tier) causes flap or a stuck mismatch | LSDBs never converge in the multi-node test | freshness matrix unit test covering all five §13.1 outcomes before any timer wiring |
| R-2 | Max-age purge deleted too early; a late neighbour re-injects the stale copy (trap #3) | a purged LSA reappears after a neighbour reconnects | retain the purge until every neighbour acks or is no longer Full; functional test: stop a neighbour's ack, verify the purge persists, resume, verify it is acked then removed |
| R-3 | Sequence wraparound at MaxSequenceNumber mis-ordered (treats wrapped as older) (trap #2) | purge / re-origination loop after a long-running session | boundary test: 0x80000001 vs 0x7FFFFFFF, 0x7FFFFFFF vs 0x7FFFFFFF; flush-then-re-originate at InitialSequenceNumber |
| R-4 | Checksum recomputed at the wrong time (not on a sequence bump) (trap #13) | cross-interop Fletcher failures while self-interop passes | recompute the Fletcher checksum in the origination path on every sequence increment; assert encode AND decode against RFC 905 vectors |
| R-5 | Retransmit list never drains (missed ack path) → endless retransmit storm | `ze_ospf_retransmissions_total` grows without bound in soak | explicit ack paths (LS Ack receive, implicit re-flood, duplicate-on-list direct ack); counter assertion in the functional test; neighbour-leaves-Full empties the list |
| R-6 | Delayed vs direct vs suppressed ack chosen wrong (§13.5 Table 19) → duplicate floods or a stuck neighbour | duplicate LS Updates / unacked LSAs on the wire | §13.5 ack-decision unit test covering DR-sourced/BDR, duplicate-on-retransmit-list, re-flooded-implicit, and otherwise-delayed cases |
| R-7 | Aging uses wall-clock time; an NTP/VM clock jump skews LS Age (trap #14) | LSA ages jump or purge prematurely after a clock step | monotonic clock for aging and RxmtInterval; LS Age computed as a delta from a monotonic origination timestamp |
| R-8 | Aging walker races with origination / flooding readers | data race under `-race`; intermittent failures | single writer for the LSDB (RWMutex or owning goroutine); snapshot reads under read lock |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| interface reaches DR with a Full neighbour (from ospf-5/ospf-6) | → | Router-LSA + Network-LSA originated, installed in the area LSDB, queued on the Full neighbour's retransmit list, flooded | `TestOSPFOriginateOnAdjacencyFull` |
| LS Update received carrying an LSA newer than the local copy | → | §13.1 compare accepts, installs, floods out other interfaces, sets retransmit lists, acks the sender | `TestOSPFLSDBSync` (three-node line; LSA originated at A floods A→B→C; B and C end with A's LSA and retransmit lists drained after ack) |
| per-neighbour RxmtInterval timer fires with an unacked LSA | → | stored raw LSA re-transmitted on that neighbour's interface; entry removed on the subsequent LS Ack | `TestOSPFRetransmitTimer` |
| LS Ack received for an LSA on a neighbour's retransmit list | → | LSA removed from that neighbour's retransmit list; acked MaxAge purge deleted from the LSDB | `TestOSPFLSAckClearsRetransmit` |
| 1 s aging walker reaches MaxAge on an LSA | → | LSA marked as a purge, flooded, retained until acked by all neighbours, then deleted | `TestOSPFLSDBAgeToPurge` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An interface reaches DR/Full with at least one fully-adjacent neighbour and a connected stub network | A Router-LSA (Type 1) is originated with the correct V/E/B flag byte and link records (transit-via-DR for the segment, stub-network for the prefix), valid InitialSequenceNumber, and a valid Fletcher checksum; a Network-LSA (Type 2) is originated with the network mask and the Router IDs of all fully-adjacent attached routers including self |
| AC-2 | An interface cost, neighbour adjacency, or DR status changes | The affected self-LSA is re-originated with the sequence number incremented by one and the Fletcher checksum recomputed; re-origination of the same LSA is rate-limited to once per MinLSInterval (5 s) |
| AC-3 | Stub-router / max-metric (RFC 6987) is in effect | The Router-LSA is originated with all non-stub link metrics set to LSInfinity (0xFFFF); stub-network link metrics keep their real cost |
| AC-4 | An LS Update carries an LSA strictly more recent than our copy (higher sequence, or equal sequence and higher checksum, or the MaxAge / MaxAgeDiff rule), arriving no sooner than MinLSArrival | The LSA is installed/replaced; it is set on the retransmit list of every Full neighbour on every interface except the receiving one and flooded out those interfaces; the sender is acknowledged (implicit re-flood, direct, or delayed per §13.5 Table 19) |
| AC-5 | An LS Update carries an LSA identical to our copy | Treated as an implied or explicit acknowledgement; if it is on the sender's retransmit list it is removed; a delayed ack may be sent; no re-flood |
| AC-6 | An LS Update carries an LSA less recent than our copy | Our newer copy is sent directly back to the sender; the stale copy is not installed |
| AC-7 | An LS Update carries a Type 5 AS-External LSA received on a stub or NSSA area interface | The LSA is dropped (Type 5 is never flooded into a stub/NSSA area); it is not installed and not re-flooded |
| AC-8 | An LSA arrives a second time within MinLSArrival (1 s) of the last accepted instance | The new instance is not accepted (MinLSArrival rate limit), though it may still be acknowledged |
| AC-9 | A per-neighbour retransmit list holds an unacked LSA when the RxmtInterval timer fires | The stored raw LSA bytes are re-transmitted to that neighbour (LS Age re-incremented by InfTransDelay); the entry remains until an LS Ack (or an implicit ack) removes it; `ze_ospf_retransmissions_total` increments |
| AC-10 | A neighbour leaves the Full state | That neighbour's retransmit list is emptied and its RxmtInterval timer stopped |
| AC-11 | A received LSA from the DR for which we are the BDR / a duplicate already on our retransmit list / a newer LSA we re-flooded back out the receiving interface / any other case | The §13.5 Table 19 rule is applied: delay (BDR-of-DR), direct ack (duplicate on retransmit list), suppress (implicit ack via the re-flood), otherwise delay; delayed acks are coalesced and flushed to the interface multicast address before RxmtInterval elapses |
| AC-12 | A self-originated LSA reaches LSRefreshTime (1800 s) | It is re-originated with the next sequence number, a freshly recomputed Fletcher checksum, and LS Age reset; it is re-flooded |
| AC-13 | A self-originated LSA's sequence number reaches MaxSequenceNumber (0x7FFFFFFF) | The LSA is prematurely aged to MaxAge and flushed; once the flush is acknowledged it is re-originated at InitialSequenceNumber (0x80000001) |
| AC-14 | An LSA's LS Age reaches MaxAge (3600 s) | The LSA is flooded as a purge and retained in the LSDB (still at MaxAge) until every neighbour has acknowledged it or is no longer Full; only then is it deleted — it is NOT deleted at the instant it reaches MaxAge |
| AC-15 | A self-originated LSA is received from the network with a higher sequence number than ours (§13.4) | We re-originate the LSA at one greater than the received sequence (or flush it via MaxAge if we no longer wish to originate it) |
| AC-16 | `show ip ospf database` (and per-type variants) | A snapshot lists, per area and for the AS-wide store, each LSA's LS Type, Link State ID, Advertising Router, LS Sequence Number, LS Age, LS Checksum, and Length, grouped by type |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up an OSPF adjacency and expects the node to advertise itself | adjacency Full (ospf-6) → Router/Network-LSA origination → LSDB install → retransmit list set on Full neighbours → flood (ospf-3 TX) → neighbour installs and floods onward | `TestOSPFOriginateOnAdjacencyFull`, `TestOSPFLSDBSync`, `test/ospf/ospf-flooding.ci` |
| 2 | Connects three nodes in a line and originates a network on the far end | origin LSA → §13 flood receipt + retransmit/ack (ospf-7) → next hop installs and floods onward → all three LSDBs converge | `TestOSPFLSDBSync`, `test/ospf/ospf-flooding.ci` |
| 3 | Shuts down a node, expecting its LSAs to clear network-wide | originator flush (MaxAge purge) → §13 re-flood → peers receive the purge, retain until acked, then delete | `TestOSPFLSDBAgeToPurge`, `test/ospf/ospf-flooding.ci` (purge phase) |
| 4 | Enables max-metric (RFC 6987) during maintenance | config → Router-LSA re-originated with non-stub metrics at LSInfinity → SPF avoids transit (honoured in ospf-8) | `TestOSPFOriginateMaxMetric` + SPF honour in `spec-ospf-8-spf-rib` |
| 5 | Runs `show ip ospf database` to inspect the LSDB | CLI → RPC → LSDB snapshot → render (ospf-13) | `test/ospf/ospf-show.ci` (ospf-13), `TestOSPFLSDBSnapshot` |

<!-- Functional/interop coverage for the full flood + SPF path is shared with ospf-8 (route install)
     and ospf-13 (FRR interop). This spec's standalone functional surface is ospf-flooding.ci
     (flood + purge over a line) and the show snapshot, exercised in ospf-13. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFLSDBStoreRetrieve` | `internal/component/ospf/lsdb/lsdb_test.go` | insert an LSA, retrieve raw bytes + metadata by LSAKey; per-area isolation; Type 5 in the AS-wide store | |
| `TestOSPFFreshnessCompareMatrix` | `internal/component/ospf/lsdb/lsdb_test.go` | all five §13.1 outcomes: higher sequence, equal sequence + higher checksum, the MaxAge rule, within MaxAgeDiff the lower age, else functionally equivalent | |
| `TestOSPFLSDBStoreVerbatim` | `internal/component/ospf/lsdb/lsdb_test.go` | stored bytes for a received LSA round-trip byte-for-byte (re-flood verbatim except LS Age); single owned copy (buffer alias-safe) | |
| `TestOSPFOriginateRouterLSA` | `internal/component/ospf/lsdb/origination_test.go` | Router-LSA built from a fake interface/neighbour table with the correct V/E/B flags, P2P / transit-via-DR / stub-network link records, valid InitialSequenceNumber + Fletcher checksum | |
| `TestOSPFOriginateNetworkLSA` | `internal/component/ospf/lsdb/origination_test.go` | Network-LSA originated when this router is DR, carrying the network mask and the Router IDs of all fully-adjacent attached routers including self; flushed when DR status is lost | |
| `TestOSPFOriginateOnAdjacencyFull` | `internal/component/ospf/lsdb/origination_test.go` | adjacency Full triggers Router/Network-LSA origination, install, and retransmit-list queueing | |
| `TestOSPFOriginateReorigOnChange` | `internal/component/ospf/lsdb/origination_test.go` | cost/neighbour/DR change increments the sequence and recomputes the checksum; MinLSInterval rate-limits re-origination | |
| `TestOSPFOriginateMaxMetric` | `internal/component/ospf/lsdb/origination_test.go` | RFC 6987 max-metric sets non-stub link metrics to LSInfinity (0xFFFF); stub-network metrics keep their cost | |
| `TestOSPFOriginateSelfReceivedHigherSeq` | `internal/component/ospf/lsdb/origination_test.go` | §13.4: a self-originated LSA received at a higher sequence forces re-origination at one greater (or flush) | |
| `TestOSPFSequenceWraparound` | `internal/component/ospf/lsdb/aging_test.go` | at MaxSequenceNumber: flush via MaxAge, then re-originate at InitialSequenceNumber once acked | |
| `TestOSPFFloodOutOtherInterfaces` | `internal/component/ospf/lsdb/flooding_test.go` | a newer LSA is queued on every Full neighbour's retransmit list except the receiving interface; DR/BDR → AllSPFRouters, DROther → AllDRouters; P2P → AllSPFRouters | |
| `TestOSPFAckDecisionTable` | `internal/component/ospf/lsdb/flooding_test.go` | §13.5 Table 19: DR-sourced/BDR → delay; duplicate on retransmit list → direct; re-flooded-back → suppress (implicit); else → delay | |
| `TestOSPFRetransmitTimer` | `internal/component/ospf/lsdb/flooding_test.go` | RxmtInterval resends unacked LSAs (raw bytes, LS Age re-incremented); LS Ack removes them; neighbour leaving Full empties the list | |
| `TestOSPFMinLSArrivalReject` | `internal/component/ospf/lsdb/flooding_test.go` | a second instance arriving within MinLSArrival (1 s) is not accepted | |
| `TestOSPFStubAreaDropsType5` | `internal/component/ospf/lsdb/flooding_test.go` | a Type 5 received on a stub/NSSA area interface is dropped, not installed, not re-flooded | |
| `TestOSPFLSDBAgeDecrement` | `internal/component/ospf/lsdb/aging_test.go` | the 1 s walker increments every LSA's LS Age (monotonic-delta based) | |
| `TestOSPFLSDBAgeToPurge` | `internal/component/ospf/lsdb/aging_test.go` | LS Age MaxAge → purge flooded + retained; deleted only after all neighbours ack or leave Full (trap #3) | |
| `TestOSPFLSRefresh` | `internal/component/ospf/lsdb/aging_test.go` | a self-LSA at LSRefreshTime is re-originated (next sequence, fresh checksum, age reset, re-flood) | |
| `TestOSPFLSDBSnapshot` | `internal/component/ospf/lsdb/lsdb_test.go` | snapshot returns per-area + AS-wide LSAs with type/LSID/advrtr/sequence/age/checksum/length grouped by type | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LS Sequence Number (signed) | 0x80000001..0x7FFFFFFF | 0x7FFFFFFF (MaxSequenceNumber, newest) | 0x80000000 (reserved; never originated; origination starts at InitialSequenceNumber 0x80000001) | wraps: at 0x7FFFFFFF flush via MaxAge then re-originate at 0x80000001 |
| LS Age (seconds) | 0..3600 | 3600 (MaxAge, a purge) | N/A (0 is valid, freshly originated) | 3601 (clamped to MaxAge) |
| MaxAgeDiff freshness window | 0..900 s | 900 | N/A | within MaxAgeDiff → lower age wins; beyond → already decided by the MaxAge rule |
| Link metric (Router-LSA link, 16-bit) | 0..0xFFFF | 0xFFFF (LSInfinity; max-metric / unreachable transit) | N/A | 0x10000 |
| LSA Length (16-bit, includes the 20-byte header) | 20..0xFFFF | 0xFFFF | 19 (shorter than the header — reject) | N/A (16-bit field) |
| MinLSArrival | 1 s | 1 | N/A (a sooner arrival is rejected) | N/A |
| MinLSInterval | 5 s | 5 | N/A (a sooner re-origination is deferred) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-flooding` | `test/ospf/ospf-flooding.ci` | three-node line floods an LSA end to end; retransmit/ack drains; a purge re-floods then ages out after all acks | |
| (show snapshot) | `test/ospf/ospf-show.ci` (ospf-13) | `show ip ospf database` renders per-area and AS-wide entries grouped by type | |

<!-- Raw-IP / multicast flooding over real interfaces is Linux-only and runs as a QEMU
     integration test (ai/rules/qemu-testing.md), not a plain .ci. The .ci above drives the
     flood/ack/purge logic over an in-process / pipe transport; the QEMU + FRR interop path
     is owned by ospf-13. -->

### Interop Tests (MANDATORY for protocol features)
<!-- Flooding correctness against a real peer (FRR ospfd) is proven by the umbrella's mandatory
     FRR scenarios owned by ospf-13; this spec relies on them rather than duplicating an
     interop scenario for flooding alone. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-p2p-frr` (umbrella, owned by ospf-13) | `test/interop/scenarios/` | FRR ospfd | FRR accepts our originated Router/Network-LSAs (valid sequence/checksum/flags) and we install + flood FRR's; LSDBs converge | |
| `ospf-broadcast-dr-frr` (umbrella, owned by ospf-13) | `test/interop/scenarios/` | FRR ospfd | Network-LSA origination as DR and §13.5 delayed-ack to the multicast address against a real stack | |

### Future (if deferring any tests)
- Summary (Type 3/4), AS-External (Type 5), and NSSA (Type 7) origination tests land with ospf-9 / ospf-10 / ospf-11; this spec covers Router (Type 1) and Network (Type 2) origination plus the generic flood/age/purge that those types reuse.
- A flooding-only FRR interop scenario is not added separately; the umbrella P2P and broadcast scenarios (ospf-13) exercise flooding end to end.

## Files to Modify
<!-- This spec is almost entirely new-file. Modifications are limited to wiring created by ospf-4/5/6. -->
- `internal/component/ospf/instance.go` (created by ospf-4) - construct the per-area LSDB + AS-wide Type 5 store, start the 1 s aging / refresh timer goroutine, register the LS Update / LS Ack handlers with the packet receive dispatcher, and pass the LSDB handle to areas/interfaces
- `internal/component/ospf/area.go` (created by ospf-4) - hold the area's LSDB handle and the self-LSA re-origination trigger
- `internal/component/ospf/events.go` (created by ospf-4) - subscribe origination to interface/neighbour/cost-change and DR-election events

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | `retransmit-interval` / `transmit-delay` and max-metric config leaves belong to `ze-ospf-conf.yang` (ospf-4 / ospf-13); this spec consumes them, it defines no schema |
| YANG validation constraints | No | range on the timer leaves owned by ospf-4 |
| YANG custom validators | No | none for this spec |
| CLI commands/flags | No | `show ip ospf database` is registered and rendered in `spec-ospf-13-cli-diag-interop`; this spec exposes the snapshot API only |
| CLI grammar (action before identifier) | No | `ai/rules/cli-grammar.md` (applied in ospf-13) |
| Editor autocomplete | No | N/A (no new config leaves here) |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-flooding.ci` (flood/ack/purge); snapshot via `test/ospf/ospf-show.ci` (ospf-13) |
| Pipe completeness | No | `show ip ospf database` output through `ApplyPipes`/`ProcessPipes` (ospf-13) |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | raw socket / `CAP_NET_RAW` doctor check owned by `spec-ospf-3-ip-transport`; flooding adds none |
| Prometheus counters/metrics | Yes | this spec OWNS and registers its rows from the umbrella `## Shared Contracts` "Metrics (canonical)" table: `ze_ospf_lsdb_lsas{area,type}`, `ze_ospf_lsa_originations_total{type}`, `ze_ospf_lsa_refreshes_total{type}`, `ze_ospf_lsa_purges_total{type}`, `ze_ospf_lsupdates_sent_total{interface}`, `ze_ospf_lsupdates_received_total{interface}`, `ze_ospf_lsacks_sent_total{interface}`, `ze_ospf_retransmissions_total{area}`. Per-owner registration HERE, NOT in ospf-13 (ospf-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | LSDB/flooding is internal; user-facing surface (`show ip ospf database`) documented in ospf-13 |
| 2 | Config syntax changed? | No | `retransmit-interval` / `transmit-delay` / max-metric leaves documented with ospf-4 / ospf-13 |
| 3 | CLI command added/changed? | No | `show ip ospf database` documented in ospf-13 |
| 4 | API/RPC added/changed? | No | snapshot RPC documented in ospf-13 |
| 5 | Plugin added/changed? | No | component-internal, no new plugin |
| 6 | Has a user guide page? | No | covered by `docs/guide/ospf.md` (owned by ospf-13) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` - LSA origination contents (Router-LSA link records + V/E/B flags, Network-LSA), LS Update / LS Ack flooding, aging/purge |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§12 header, §13 flooding, §14 aging), `rfc/short/rfc905.md` (Fletcher refresh), `rfc/short/rfc6987.md` (max-metric) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - new `test/ospf/ospf-flooding.ci` |
| 11 | Affects daemon comparison? | No | OSPF comparison row owned by ospf-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` or an OSPF subsystem doc - LSDB store + origination + flooding design (lazy raw-bytes model, per-area + AS-wide split) |
| 13 | Route metadata keys added/changed? | No | flooding touches no route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` - the eight `ze_ospf_lsdb_*` / `ze_ospf_lsa_*` / `ze_ospf_lsupdates_*` / `ze_ospf_lsacks_sent_total` / `ze_ospf_retransmissions_total` series owned and registered HERE; ospf-13 only scrapes/surfaces |
| 15 | Registered plugin/event/command/capability changed? | No | LS Update / LS Ack handlers register with the ospf-4 dispatcher (existing registration surface), no new plugin/event type |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/ospf/lsdb/lsdb.go` - the LSDB store: per-area type tables + one AS-wide Type 5 table keyed by LSAKey, insert/lookup/delete, the §13.1 freshness compare, MinLSArrival gate, snapshot API
- `internal/component/ospf/lsdb/entry.go` - the per-LSA entry: raw bytes + parsed metadata (LSAKey, sequence, monotonic origination timestamp / derived LS Age, checksum, length), purged-state marker, lazy body accessors
- `internal/component/ospf/lsdb/origination.go` - Router-LSA build (link records + V/E/B flags) and Network-LSA build (as DR), sequence assignment/increment, §13.4 self-received handling, RFC 6987 max-metric, MinLSInterval rate limit
- `internal/component/ospf/lsdb/flooding.go` - the §13 receive procedure (freshness-driven install/replace/send-back), flood-out other interfaces, per-neighbour retransmit lists + RxmtInterval, §13.5 Table 19 ack decision (implicit/direct/delayed), LS Ack receive
- `internal/component/ospf/lsdb/aging.go` - the 1 s MaxAge walker, purge retention until all-acked, LSRefresh at LSRefreshTime, sequence wraparound flush-and-re-originate
- `internal/component/ospf/lsdb/lsdb_test.go` - store/retrieve, freshness matrix, verbatim storage, snapshot
- `internal/component/ospf/lsdb/origination_test.go` - Router/Network-LSA origination, re-orig on change, max-metric, self-received-higher-seq
- `internal/component/ospf/lsdb/flooding_test.go` - flood-out, ack decision, retransmit, MinLSArrival, stub-area Type 5 drop
- `internal/component/ospf/lsdb/aging_test.go` - age decrement, age-to-purge retention, LSRefresh, wraparound
- `test/ospf/ospf-flooding.ci` - functional flood + retransmit/ack + purge over a three-node line

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-ospf-6-neighbor-nsm, spec-ospf-5-interface-ism, spec-ospf-2-wire |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan - check what ospf-4/5/6/2 already provide |
| 3. Wiring phase | Wiring Test table - register LS Update / LS Ack handlers + origination trigger, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - LSDB skeleton + LS Update / LS Ack handler registration with the ospf-4 dispatcher + an origination trigger reachable from adjacency Full
   - Tests: `TestOSPFOriginateOnAdjacencyFull` (fails: origination is a stub), `TestOSPFLSDBStoreRetrieve`
   - Files: `internal/component/ospf/lsdb/lsdb.go` (struct + insert/lookup stubs), `entry.go`, handler registration in `internal/component/ospf/instance.go`, origination hook in `events.go`
   - Verify: an adjacency-Full event reaches the origination entry point; LS Update / LS Ack handlers are reachable from the dispatcher; the wiring test fails only because origination/compare return stubs
2. **Phase: Store + freshness + snapshot** - the database core
   - Tests: `TestOSPFLSDBStoreRetrieve`, `TestOSPFFreshnessCompareMatrix`, `TestOSPFLSDBStoreVerbatim`, `TestOSPFLSDBSnapshot`
   - Files: `lsdb.go`, `entry.go`
   - Verify: insert/lookup/delete, per-area + AS-wide isolation, all five §13.1 outcomes, verbatim raw-byte storage (single owned copy), snapshot
3. **Phase: Origination** - build own LSAs from live state
   - Tests: `TestOSPFOriginateRouterLSA`, `TestOSPFOriginateNetworkLSA`, `TestOSPFOriginateReorigOnChange`, `TestOSPFOriginateMaxMetric`, `TestOSPFOriginateSelfReceivedHigherSeq`
   - Files: `origination.go`
   - Verify: Router-LSA link records + V/E/B flags; Network-LSA as DR with attached Router IDs incl. self; sequence increment + Fletcher recompute (via ospf-2); MinLSInterval rate limit; RFC 6987 max-metric; §13.4 self-received handling
4. **Phase: Flooding** - the §13 receive + flood-out + retransmit + ack procedure
   - Tests: `TestOSPFFloodOutOtherInterfaces`, `TestOSPFAckDecisionTable`, `TestOSPFRetransmitTimer`, `TestOSPFMinLSArrivalReject`, `TestOSPFStubAreaDropsType5`, `TestOSPFLSDBSync`, `TestOSPFLSAckClearsRetransmit`
   - Files: `flooding.go`
   - Verify: flood out other interfaces with correct multicast destination; §13.5 ack decision; retransmit lifecycle (no storm); MinLSArrival gate; stub-area Type 5 drop; three-node line converges and retransmit lists drain after ack
5. **Phase: Aging + purge + refresh + wraparound** - lifecycle timers
   - Tests: `TestOSPFLSDBAgeDecrement`, `TestOSPFLSDBAgeToPurge`, `TestOSPFLSRefresh`, `TestOSPFSequenceWraparound`
   - Files: `aging.go`
   - Verify: monotonic-delta age increment; MaxAge purge retained until all-acked then deleted (trap #3); LSRefresh re-originates; MaxSequenceNumber flush-then-re-originate at InitialSequenceNumber (trap #2)
6. **Functional test** - `test/ospf/ospf-flooding.ci`: three-node flood, retransmit/ack drain, purge re-flood + age-out
7. **Metrics** - register HERE (per-owner) the eight `ze_ospf_lsdb_*` / `ze_ospf_lsa_*` / `ze_ospf_lsupdates_*` / `ze_ospf_lsacks_sent_total` / `ze_ospf_retransmissions_total` series from the umbrella canonical table; ospf-13 only scrapes/asserts
8. **RFC refs** - `// RFC 2328 Section 13.x` / `// RFC 6987 Section x` comments above the freshness table, ack decision, purge retention, origination, and max-metric
9. **Full verification** - `make ze-verify`
10. **Complete spec** - fill audit tables, write learned summary to `plan/learned/NNN-ospf-7-lsdb-flooding.md`, two commits per planning.md

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-16 has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path; origination contents match the umbrella LSA layout (Router-LSA V/E/B + link records, Network-LSA mask + attached routers); flooding matches RFC 2328 §13 |
| Correctness | §13.1 freshness order exact (sequence → checksum → MaxAge rule → MaxAgeDiff → equivalent); purge retained until all-acked (trap #3); checksum recomputed on every sequence change (trap #13); wraparound flush-then-re-originate (trap #2); monotonic age (trap #14); §13.5 ack decision (Table 19) |
| Naming | Package `lsdb`; exported types/methods follow Ze conventions; the eight metric names listed verbatim |
| Data flow | LSAs stored as raw bytes + metadata; body parse lazy; LS Update / LS Ack arrive via the ospf-4 dispatcher (no packet-type switch here); flood re-transmits stored raw bytes; retransmit lists driven by ospf-6 Full neighbours; no bypass of the ospf-2 codec |
| CLI grammar | n/a (no new command here; `show ip ospf database` in ospf-13) |
| Doctor checks | n/a (raw socket / `CAP_NET_RAW` is ospf-3) |
| YANG validation | n/a (no new leaf; timer/max-metric leaves owned by ospf-4 / ospf-13) |
| Prometheus counters | the eight owned series registered HERE (per-owner) with the exact umbrella names; ospf-13 only scrapes/asserts |
| Rule: buffer-first | LSA encode is `WriteTo(buf, off) int`; Fletcher backfilled at the fixed offset; flood re-transmits stored raw bytes (only LS Age adjusted); no `buildLSA() []byte`, no per-LSA alloc on the retransmit timer |
| Rule: plugin-self-containment | all LSDB / origination / flooding / aging code under `internal/component/ospf/lsdb/`; no OSPF spelling in generic packages, no flooding spelling in ospf-2/ospf-3 |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| LSDB store + entry | `ls internal/component/ospf/lsdb/lsdb.go internal/component/ospf/lsdb/entry.go` |
| Origination + flooding + aging | `ls internal/component/ospf/lsdb/origination.go internal/component/ospf/lsdb/flooding.go internal/component/ospf/lsdb/aging.go` |
| Unit tests pass | `go test ./internal/component/ospf/lsdb/...` |
| Freshness + ack + purge + wraparound covered | `go test -run 'Freshness|AckDecision|AgeToPurge|Wraparound' ./internal/component/ospf/lsdb/` |
| Functional flood test | `ls test/ospf/ospf-flooding.ci` and run via the .ci runner |
| Snapshot API consumed by ospf-13 | `grep -r 'Snapshot' internal/component/ospf/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Received LSA metadata parsed only after the ospf-2 codec validated lengths and the Fletcher checksum; no slicing past LSA bounds in the LSDB; unknown LS Types dropped, not stored as a runnable body |
| Resource exhaustion | LSDB size bounded per area and AS-wide; per-neighbour retransmit list and per-interface delayed-ack list bounded; MinLSArrival rate-limits accepted instances; reject oversize / malformed LSAs before storing; retransmit timer does not unbounded-loop |
| Stale / replay | Lower-sequence / stale-checksum LSAs never replace newer; MaxAge purge handled per §14 with retention, no premature deletion; a self-received higher-sequence LSA is handled per §13.4, not blindly installed |
| Spoofing | Act only on LSAs for the area the receiving interface belongs to (Type 5 dropped on stub/NSSA interfaces); never re-flood out the receiving interface; authentication (ospf-12) is verified upstream before an LSA reaches the LSDB |
| Memory safety | Stored raw bytes are a single owned copy (not aliasing a reused receive buffer) so re-flood is not corrupted when the receive buffer is reused; aging walker and flooding readers serialised under the LSDB lock |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2328 §13/§14 summary / the Current Behavior contract from ospf-2/ospf-5/ospf-6 |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| Interop mismatch | Capture with tcpdump, compare LS Update / LS Ack and the Fletcher checksum to FRR, fix compare/origination logic |
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
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
<!-- Delete this section if nothing qualifies. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Store LSAs as raw bytes + parsed metadata, parse the body lazily | Eager parse into structs | Ze buffer-first model; cheap byte-for-byte re-flood (LS Age excluded from the Fletcher checksum, so re-transmit needs no re-encode); parse only what SPF/CLI needs (research sec 3) |
| Per-area type tables + one AS-wide Type 5 table | Single LSDB with a domain filter (BIRD) | FRR model; simpler for a first pass; matches the umbrella "Per-area LSDB" principle; Type 5 is AS-scoped so it lives once on the instance |
| Per-neighbour retransmit lists + per-interface delayed-ack lists drive transmission | A single global send map | RFC 2328 §13 model; an ack removes exactly the entry it acknowledges; a neighbour leaving Full empties its list; reliable per-neighbour delivery |
| MaxAge purge retained until all neighbours ack, distinct from deletion | Delete at MaxAge | RFC 2328 §14 / trap #3; deleting too early lets a late neighbour re-inject the stale copy |
| Recompute the Fletcher checksum in the origination path on every sequence bump | Recompute opportunistically / on read | Trap #13; the body did not change but the sequence did, so the checksum must be refreshed exactly when the sequence increments |
| Monotonic clock for LS Age and RxmtInterval | Wall-clock `time()` | Trap #14; an NTP / VM-migration clock step would otherwise skew ages and timers |
| Re-originate Router/Network-LSAs by full regeneration on change | Incremental link-record edits | Simpler and correct; origination is not a hot path; mirrors the IS-IS sibling's full-LSP regeneration |

## Known Limitations
<!-- Source for learned summary Consequences section. -->
- This spec originates only Router-LSA (Type 1) and Network-LSA (Type 2). Summary (Type 3/4), AS-External (Type 5), and NSSA (Type 7) origination are owned by ospf-9 / ospf-10 / ospf-11, which reuse this store, the generic flood procedure, and the aging/purge model.
- Broadcast (DR/BDR) and point-to-point network types only; NBMA and point-to-multipoint flooding nuances (RFC 2328 §13.3 Table 19 NBMA rows) are out of scope per the umbrella.
- Opaque LSAs (Type 9/10/11) are not stored or flooded in v1 (the umbrella defers the opaque framework); lazy passthrough would be added with that framework.
- Flood reduction / demand circuits (DoNotAge, RFC 1793) are out of scope; the DoNotAge bit is decoded by ospf-2 but not originated here.
- RxmtInterval and transmit-delay come from per-interface config (ospf-4); the other §13/§14 constants (MaxAge, MaxAgeDiff, LSRefreshTime, MinLSArrival, MinLSInterval, InitialSequenceNumber, MaxSequenceNumber, LSInfinity) are RFC-fixed, not tunable.

## RFC Documentation

Add `// RFC 2328 Section X.Y: "<quoted requirement>"` (and RFC 905 / RFC 6987 as applicable) above enforcing code.
MUST document: the §13.1 freshness comparison decision (sequence → checksum → MaxAge rule → MaxAgeDiff → equivalent), the §13.3 flood-out destination rules, the §13.5 Table 19 ack decision (implicit / direct / delayed), the §13.7 self-originated MaxAge flush and §13.4 self-received-higher-sequence re-origination, the §14.1 MaxAge purge retention, LSRefreshTime refresh, MinLSArrival / MinLSInterval rate limits, the InitialSequenceNumber / MaxSequenceNumber wraparound (RFC 2328 §14), the Fletcher checksum recompute on re-origination (RFC 905), and the LSInfinity max-metric origination (RFC 6987).

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Per-area LSDB stores LSAs as lazy raw bytes + metadata; Type 5 AS-wide | functional test | `TestOSPFLSDBStoreRetrieve`, `TestOSPFLSDBStoreVerbatim` |
| Router-LSA / Network-LSA self-origination (incl. RFC 6987 max-metric) | functional test | `TestOSPFOriginateRouterLSA`, `TestOSPFOriginateNetworkLSA`, `TestOSPFOriginateMaxMetric` |
| The §13 flooding procedure (freshness, retransmit, ack, rate limits) | functional test + interop | `TestOSPFLSDBSync`, `test/ospf/ospf-flooding.ci`, FRR scenario (ospf-13) |
| MaxAge walker + purge retention, LSRefresh, sequence wraparound | functional test | `TestOSPFLSDBAgeToPurge`, `TestOSPFLSRefresh`, `TestOSPFSequenceWraparound` |
| `show ip ospf database` snapshot | functional test | `TestOSPFLSDBSnapshot`, `test/ospf/ospf-show.ci` (ospf-13) |
| The eight owned Prometheus series registered with exact names | metric assertion | metric-series unit test; ospf-13 scrape |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/ospf/lsdb/`, instance/area wiring)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
