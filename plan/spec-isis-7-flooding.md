# Spec: isis-7-flooding

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-6-lsdb.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - row "isis-7", dependency graph, package layout
4. `plan/spec-isis-6-lsdb.md` - LSDB store, SRM/SSN flags, origination, aging, purge (the data this spec drives)
5. `plan/spec-isis-3-l2-transport.md` - per-circuit RX/TX path that flooding transmits over
6. `plan/spec-isis-4-component-config.md` (and `plan/spec-isis-0-umbrella.md` Shared Contracts "PDU receive dispatcher") - the isis-4 dispatcher that delivers LSP/CSNP/PSNP to this spec
7. `plan/spec-isis-2-wire.md` - LSP/CSNP/PSNP codecs and LSP Entries TLV 9
8. `docs/research/isis-implementation-guide.md` section 4c "LSP Flooding" (lines ~301-364) and section 5 (LSPDB model)

## Task

Implement LSP flooding and SNP (sequence number PDU) synchronisation for IS-IS,
the mechanism that disseminates link-state PDUs across the network so that every
router in a level converges on the same LSDB. This is the dynamic counterpart to
spec-isis-6: isis-6 owns the LSDB store, the per-LSP SRM/SSN flags, origination,
aging, refresh, and purge; isis-7 owns the algorithms that *consume and produce*
those flags over the wire.

Concretely, this spec adds:
- Reliable flooding on LSP receipt (ISO/IEC 10589 sec 7.3.14-17): freshness comparison
  against the local LSDB, accept/replace newer LSPs, drive SRM (send) on all
  circuits except the one it arrived on, drive SSN (acknowledge) on the incoming
  circuit, and handle zero-lifetime purges.
- A periodic flood timer (default 5 s) that transmits LSPs whose SRM flag is set on
  a circuit and clears SRM once sent/acknowledged.
- CSNP (Complete Sequence Number PDU) generation and receipt: periodic full-database
  advertisement built from LSP Entries (TLV 9) across the LSPID range, with cadence
  differing between LAN and P2P circuits, and an initial CSNP on P2P at adjacency Up
  for fast sync.
- PSNP (Partial Sequence Number PDU) generation and receipt: acknowledge LSPs we
  hold (driven by SSN flags on held LSDB entries) and request LSPs we do not yet
  hold (driven by a per-circuit pending-request set, see below), and on receiving a
  PSNP clear SSN and supply requested LSPs.
- A per-circuit pending-request set so the receiver can request LSPs it does not yet
  hold. SSN flags live on EXISTING LSDB entries, so they can only acknowledge LSPs we
  already have. When a received CSNP lists an LSP ID we do not hold (or hold an older
  copy of), there is no LSDB entry to mark, so SSN cannot represent that request. The
  receiver records the wanted (LSPID, level, sequence) in a per-circuit pending-request
  set, independent of LSDB entries, and a PSNP drains that set to request the missing
  LSPs. SSN keeps its acknowledge semantics for LSPs we already hold; the pending-request
  set covers requests for LSPs we do not yet hold. A pending entry is cleared when the
  requested LSP arrives and is stored in the LSDB.
- Independent operation for both routing levels (L1 and L2).

Flooding lives in `internal/component/isis/lsdb/` alongside the store from isis-6:
`flooding.go` (receipt algorithm, periodic SRM-driven TX) and `snp.go` (CSNP/PSNP
build, send, receive, plus the per-circuit pending-request set for LSPs not yet
held). LSP, CSNP, and PSNP PDUs are delivered to this spec by the isis-4 PDU receive
dispatcher (umbrella Shared Contracts "PDU receive dispatcher", owner isis-4
`server.go`); this spec registers handlers with that dispatcher rather than holding
its own PDU-type switch. It depends on isis-6 (LSDB + SRM/SSN flag storage), isis-4
(PDU dispatcher), isis-3 (per-circuit transport TX), and isis-2 (LSP/CSNP/PSNP wire
codecs incl. TLV 9).

The LAN-specific CSNP cadence and the DIS-only obligation to source periodic CSNPs
on broadcast circuits belong to spec-isis-8 (DIS election), but the CSNP/PSNP
build, send, and receive *mechanism* lives here so isis-8 only supplies the
"who and how often" policy.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] - checkboxes are template markers, not progress trackers. -->
- [ ] `docs/research/isis-implementation-guide.md` section 4c (LSP Flooding) + section 5 (LSPDB model) - flooding algorithm, SRM/SSN semantics, periodic SRM timer, CSNP/PSNP, purge
  -> Decision: drive transmission entirely from SRM/SSN flags stored per-circuit in the LSDB entry (isis-6); flooding.go is a flag-to-wire pump, never a second source of truth
  -> Constraint: freshness compare follows ISO/IEC 10589 sec 7.3.15 / 7.3.16.1 exactly, in the order (1) sequence number, (2) remaining lifetime, (3) checksum: higher seq -> accept+replace+SRM-others+SSN-in. At EQUAL seq, compare remaining lifetime BEFORE checksum: a received LSP with remaining lifetime 0 (a purge) is treated as MORE recent than a held copy with non-zero lifetime, so accept and re-flood it; only at equal seq AND equal lifetime-class (both non-zero) does a differing checksum trigger a PSNP request (the held copy may be corrupt) and an equal checksum count as a duplicate. Lower seq -> send our newer copy back. The remaining-lifetime tier must not be skipped (a missing tier loses purges that arrive at the same sequence number)
  -> Constraint: SRM set when a newer LSP is received; SRM cleared when the LSP is sent on that circuit and acknowledged (PSNP or observed in CSNP); SSN set on the receiving circuit; SSN cleared when a PSNP is sent
  -> Constraint: zero-lifetime purge with seq >= ours is accepted and re-flooded, marked purged locally but not deleted until MaxAge elapses (deletion is isis-6's job; re-flood is this spec's)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, lazy parse, no-alloc TX path
  -> Constraint: flood LSPs by re-transmitting the stored raw LSP bytes (isis-6 lazy storage); CSNP/PSNP TLV 9 entries encode buffer-first via `WriteTo(buf, off) int`; no per-LSP allocation on the periodic timer
- [ ] `ai/rules/plugin-self-containment.md` - self-contained component
  -> Constraint: all flooding/SNP logic stays under `internal/component/isis/lsdb/`; no flooding spelling leaks into transport (isis-3) or wire (isis-2)

### RFC Summaries (MUST for protocol work)
- [ ] `iso/short/iso10589.md` - IS-IS base (created by isis-2/isis-6; this spec adds flooding-specific quotes)
  -> Constraint: sec 7.3.14 receive-side flooding rules and unknown-TLV verbatim re-flood
  -> Constraint: sec 7.3.15 the LSP sequence/lifetime/checksum comparison decision table
  -> Constraint: sec 7.3.16-17 SNP (CSNP/PSNP) processing: which entries set SRM vs SSN, ack vs request
  -> Constraint: purge re-flood semantics: lifetime 0 LSP is flooded then aged out, originator must not reuse the LSPID until purge expiry

**Key insights:** (minimal context to resume after compaction)
- Flooding is reliable: SRM marks "I owe you this LSP on this circuit", SSN marks "I owe you an ack on this circuit". The 5 s timer drains SRM; PSNP/CSNP receipt clears SRM (ack) and CSNP gaps set SRM/SSN.
- CSNP = "here is everything I have" (full digest, TLV 9 entries over the LSPID range). PSNP = "here is a partial list" used both to acknowledge specific LSPs (SSN on held entries) and to request missing/newer ones (per-circuit pending-request set, since an LSP we do not yet hold has no LSDB entry to carry an SSN flag).
- LAN: DIS sources periodic CSNPs; routers PSNP-ack. P2P: send one CSNP at adjacency Up to sync, then periodically (slower); both ends ack/request via PSNP. (DIS cadence policy is isis-8; mechanism here.)
- L1 and L2 flood independently over the same circuit; SRM/SSN flags and CSNP/PSNP are per-level.

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey - isis-6/isis-3/isis-2 are sibling specs, not yet implemented)
- [ ] Ze has no IS-IS protocol at all; there is no flooding, no SNP, no LSDB sync today
  -> Constraint: this spec is entirely new; "current behavior" is the contract exposed by isis-6 (LSDB + SRM/SSN flag API) and isis-3 (circuit RX/TX) once those land
- [ ] isis-6 (sibling) owns the LSDB entry, including per-circuit SRM/SSN flag storage, origination, lifetime decrement, refresh, and purge marking
  -> Constraint: this spec calls into that flag API; it does not store LSPs or own the aging timer
- [ ] isis-3 (sibling) owns per-circuit raw L2 RX/TX goroutines and delivers `(ifindex, pdu []byte)` after stripping 802.3+LLC; it holds NO protocol PDU-type switch
  -> Constraint: flooding transmits via the isis-3 circuit TX path; it does NOT register with isis-3 for receive
- [ ] isis-4 (sibling) `server.go` owns the PDU receive dispatcher keyed by the 5-bit PDU type (umbrella Shared Contracts "PDU receive dispatcher")
  -> Constraint: flooding registers LSP (0x12/0x14) and CSNP/PSNP (0x18/0x19/0x1a/0x1b) handlers with the isis-4 dispatcher at startup; it never re-derives the PDU type or maintains its own switch
- [ ] isis-2 (sibling) owns LSP/CSNP/PSNP codecs and LSP Entries TLV 9
  -> Constraint: CSNP/PSNP build reuses the TLV 9 encoder; LSP freshness fields (LSPID, sequence, lifetime, checksum) come from the isis-2 parsed header

**Behavior to preserve:**
- isis-6 LSDB store, flag storage, aging/refresh/purge semantics unchanged (this spec drives the flags, does not redefine them)
- isis-3 transport RX/TX and isis-2 codecs unchanged (this spec is a consumer)
- L1 and L2 remain independent databases and flooding domains

**Behavior to change:**
- None pre-existing. New: receive handlers for LSP/CSNP/PSNP, a periodic flood timer per circuit/level, a CSNP cadence timer, and SRM/SSN-driven transmission.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- LSP PDU received on a circuit (from the isis-4 PDU receive dispatcher) for level L1 or L2
- CSNP PDU received on a circuit (full digest from a neighbour / DIS; via the isis-4 dispatcher)
- PSNP PDU received on a circuit (ack or request from a neighbour; via the isis-4 dispatcher)
- Periodic flood timer tick (default 5 s) per circuit/level
- CSNP cadence timer tick (LAN periodic from DIS; P2P initial-at-Up plus slow periodic)
- Adjacency Up event on a P2P circuit (triggers the initial CSNP)

### Transformation Path
1. **LSP receive:** the isis-4 dispatcher hands the LSP PDU (and ifindex/circuit) to this handler -> parse LSPID/sequence/lifetime/checksum from the PDU header (isis-2) -> look up the LSDB entry (isis-6) -> freshness compare (ISO/IEC 10589 sec 7.3.15 / 7.3.16.1), comparing in order (sequence number, then remaining lifetime, then checksum) -> if higher seq, accept + replace stored raw bytes + set SRM on every circuit except the incoming one + set SSN on the incoming circuit; if equal seq, compare remaining lifetime FIRST -- a received LSP with remaining lifetime 0 (purge) is more recent than a held non-zero copy, so accept + re-flood it (SRM on others); only at equal seq AND equal lifetime-class does a differing checksum set SSN on incoming to request via PSNP, and an equal checksum count as a duplicate (clear nothing, optionally set SSN to ack on LAN); if lower seq, set SRM on the incoming circuit to send our newer copy back. Zero-lifetime purge with seq >= ours: accept, mark purged (isis-6), re-flood (SRM on others). On accepting and storing any newer LSP, clear any matching per-circuit pending-request entry (the request is now satisfied).
2. **Periodic flood (SRM):** timer tick -> for each LSDB entry with SRM set on this circuit and the circuit not passive -> transmit the stored raw LSP bytes via isis-3 -> clear SRM (P2P) or leave SRM until ack (LAN, cleared on PSNP/CSNP observation). Unacknowledged SRM resends on the next tick.
3. **CSNP build/send:** cadence tick (or P2P adjacency Up) -> enumerate the LSDB for the level -> build CSNP(s) with LSP Entries (TLV 9) covering the LSPID range (start/end LSPID fields) -> transmit via isis-3.
4. **CSNP receive:** for each TLV 9 entry, compare to our LSDB. Missing (we do not hold the LSPID at all) or neighbour-newer-but-we-only-hold-an-older-copy -> we have no current/correct LSDB entry to mark with SSN, so record (LSPID, level, sequence) in the per-circuit pending-request set so a PSNP can request it; a missing LSP is NOT represented by an SSN flag (there is no entry to set it on). Neighbour-newer where we still hold a (stale) entry MAY also set SSN on that held entry, but the authoritative request is the pending-request set so the PSNP covers LSPs we do not yet hold. We-newer -> set SRM on this circuit (send ours). Entries we hold that are absent from a complete CSNP range -> set SRM to send them. Equal -> clear SRM on this circuit (implicit ack of LSPs the neighbour confirms it has) and clear any matching pending-request entry.
5. **PSNP build/send:** build the request/ack list from two sources: (a) for each LSDB entry with SSN set on this circuit -> add an LSP Entry (TLV 9) -> clear SSN (acknowledge a held LSP); (b) for each pending-request entry on this circuit -> add an LSP Entry (TLV 9) -> the entry stays pending until the requested LSP arrives and is stored. Transmit the PSNP. On LAN, (a) acknowledges held LSPs; (b) requests LSPs we do not yet hold. A pending-request entry is cleared when the requested LSP is received and stored in the LSDB.
6. **PSNP receive:** for each TLV 9 entry the neighbour acknowledges at our sequence -> clear SRM on this circuit (ack received); for each entry the neighbour lists as missing/older -> set SRM on this circuit to supply the requested LSP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Dispatcher (isis-4) <-> flooding | flooding registers LSP/CSNP/PSNP handlers with the isis-4 PDU receive dispatcher; receives `(ifindex, pdu)` per type | [ ] |
| Transport (isis-3) <-> flooding | TX of raw LSP / built SNP bytes on a named circuit (no RX coupling; receive arrives via the isis-4 dispatcher) | [ ] |
| LSDB (isis-6) <-> flooding | LSDB lookup/replace + per-circuit SRM/SSN flag get/set API (held LSPs); per-circuit pending-request set owned by this spec for LSPs not yet held | [ ] |
| Wire codec (isis-2) <-> flooding | LSP header field parse; CSNP/PSNP + TLV 9 encode/decode | [ ] |
| Adjacency (isis-5) <-> flooding | P2P adjacency Up event triggers initial CSNP; circuit set defines SRM-others scope | [ ] |

### Integration Points
- `internal/component/isis/lsdb/flooding.go` - receive-side algorithm + periodic SRM timer (new)
- `internal/component/isis/lsdb/snp.go` - CSNP/PSNP build, send, receive + per-circuit pending-request set (new)
- isis-6 LSDB entry SRM/SSN flag API - consumed here (SSN only on held LSPs)
- isis-4 PDU receive dispatcher - flooding registers LSP/CSNP/PSNP handlers here (umbrella Shared Contracts)
- isis-3 circuit TX path - consumed here for transmit (no RX registration)
- isis-2 LSP/CSNP/PSNP + TLV 9 codecs - consumed here
- isis-8 (DIS) supplies LAN CSNP cadence policy and DIS-only periodic CSNP sourcing (mechanism here)

### Architectural Verification
- [ ] No bypassed layers (frames -> isis-3 RX -> isis-4 PDU dispatcher -> flooding handler -> isis-6 flags / pending-request set; TX -> isis-3)
- [ ] No unintended coupling (flooding does not import SPF/redistribute; transport/wire unaware of flooding; flooding does not re-implement the PDU-type switch owned by isis-4)
- [ ] No duplicated functionality (LSDB store/flags owned by isis-6; codecs by isis-2; this spec only orchestrates)
- [ ] Zero-copy preserved (flood re-transmits stored raw LSP bytes; SNP encode is buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | isis-6 exposes per-circuit SRM/SSN flag get/set keyed by (LSPID, level, circuit) | umbrella package layout + research sec 5 LSPDB model | flooding must store flags itself, blurring the isis-6/isis-7 split | isis-6 API review before isis-7 implement | unvalidated |
| A-2 | isis-4 `server.go` exposes a PDU receive dispatcher where a consumer registers a handler per PDU type, and isis-3 lets a consumer transmit raw bytes on a named circuit | umbrella Shared Contracts "PDU receive dispatcher" + isis-3 scope | need a different RX-dispatch/TX hook shape | isis-7 wiring test over a veth/pipe pair | unvalidated |
| A-3 | Re-transmitting stored raw LSP bytes verbatim is RFC-correct (unknown TLVs preserved) | research sec 4c + ISO/IEC 10589 sec 7.3.14 | must re-encode from parsed form, losing unknown TLVs | round-trip + flood test asserting byte-identical re-flood | unvalidated |
| A-4 | A single CSNP can hold the whole LSDB digest in the common case; range split only needed for large DBs | research sec 4c | must always paginate CSNP by LSPID range | boundary test with a DB exceeding one CSNP MTU | unvalidated |
| A-5 | isis-2 LSP header parse exposes sequence + checksum cheaply without full TLV parse | buffer-first lazy LSDB philosophy | freshness compare forces full parse on every receipt | unit test reading header fields from raw bytes | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Freshness comparison table implemented wrong (equal-seq-diff-checksum vs duplicate) causes flap or stuck mismatch | LSDBs never converge in the 3-node test | freshness comparison matrix unit test covering all 5 cases before any timer wiring |
| R-2 | SRM never cleared (missed ack path) causes endless re-flood storm | flood counter grows without bound in soak | explicit ack paths: PSNP receive clears SRM, CSNP equal-entry clears SRM; counter assertion in functional test |
| R-3 | Sequence wraparound at 0xFFFFFFFF mis-ordered (treats wrapped as older) | purge/re-origination loop | boundary test on sequence compare incl. wrap and reserved-0 handling (purge is remaining-lifetime 0), aligned with isis-6 |
| R-4 | Purge re-flood deletes the LSP locally before peers receive it | downstream nodes keep a stale entry | mark purged but defer deletion to isis-6 MaxAge timer; functional test asserts purge propagates then ages out |
| R-5 | P2P initial CSNP not sent (or sent before adjacency Up) leaves the two LSDBs unsynced | P2P pair diverges while LAN converges | trigger initial CSNP strictly on adjacency Up event; P2P sync assertion in functional test |
| R-6 | L1 and L2 flooding cross-contaminate (shared flag/state) | wrong-level LSP appears in a database | per-level flag namespaces; test runs L1 and L2 on the same circuit and asserts isolation |
| R-7 | Missing-LSP request modelled as an SSN flag on a non-existent entry, so the request is silently dropped and the receiver never asks for the LSP | CSNP lists an unknown LSP but no PSNP request follows; LSDB never converges | per-circuit pending-request set independent of LSDB entries; cleared only when the LSP arrives and is stored; `TestCSNPGapRequestPending` asserts a PSNP request for an LSP we do not hold |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| LSP received on circuit, newer than LSDB | -> | freshness compare accepts, sets SRM on other circuits, sets SSN on incoming | `TestISISLSDBSync` (three-node line; LSP originated at A floods A->B->C; B and C end with A's LSP and SRM cleared after ack) |
| Periodic flood timer tick with SRM set | -> | stored raw LSP transmitted on that circuit; SRM cleared on ack | `TestISISFloodSRMTimer` |
| CSNP received listing an LSP we do not hold | -> | per-circuit pending-request entry recorded (no SSN, no LSDB entry exists); PSNP request emitted | `TestISISCSNPGapRequest` |
| PSNP received acknowledging our LSP | -> | SRM cleared on that circuit | `TestISISPSNPAck` |
| three-node line, functional | -> | full flood + CSNP/PSNP sync over real circuits | `test/isis/isis-flooding.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LSP with a higher sequence number than the local copy arrives on circuit I | LSP accepted, local copy replaced, SRM set on all circuits except I, SSN set on I |
| AC-2 | LSP with equal sequence, equal lifetime-class (both non-zero), but differing checksum arrives (we hold an entry) | LSP discarded; SSN set on the held entry on the incoming circuit so a PSNP requests the correct copy (checksum is compared only after the remaining-lifetime tier) |
| AC-3 | LSP with equal sequence, equal lifetime-class, and equal checksum arrives | Treated as duplicate; no replace, no SRM set (LAN may set SSN to ack) |
| AC-4 | LSP with a lower sequence number arrives on circuit I | SRM set on I to send our newer copy back to the sender |
| AC-5 | Periodic flood timer fires with SRM set on a circuit | Stored raw LSP bytes transmitted on that circuit; SRM cleared after send (P2P) / after ack (LAN); unacknowledged SRM resent next tick |
| AC-6 | Three-node line A-B-C; A originates an LSP | LSP floods A->B->C; B and C LSDBs contain A's LSP; all SRM cleared once acknowledged |
| AC-7 | CSNP arrives listing an LSP we hold an older copy of (neighbour-newer) | a per-circuit pending-request entry is recorded for the newer (LSPID, seq); a PSNP requesting the LSP is sent (SSN MAY also be set on the held stale entry, but the pending-request set is the authoritative request) |
| AC-8 | CSNP arrives listing an LSP older than ours | SRM set on that circuit to send our newer copy |
| AC-9 | PSNP arrives acknowledging our LSP at our sequence | SRM cleared on that circuit |
| AC-10 | PSNP arrives requesting an LSP we hold | SRM set on that circuit to supply it |
| AC-11 | P2P adjacency reaches Up | An initial CSNP is sent on that circuit to synchronise the two LSDBs |
| AC-12 | Self-originated LSP purged (lifetime 0, seq bumped) by isis-6 | Purge LSP re-flooded on all circuits, propagates, then ages out at MaxAge without local premature deletion |
| AC-13 | Two LSDBs initially out of sync, CSNP/PSNP exchanged | Both LSDBs converge to identical contents (missing requested, newer supplied) |
| AC-14 | L1 and L2 both active on one circuit | Flooding and SNP operate per-level; an L1 LSP never enters the L2 database and vice versa |
| AC-15 | CSNP arrives listing an LSP ID the receiver does not hold at all | Because no LSDB entry exists, SSN cannot be set; the receiver records a per-circuit pending-request entry and a PSNP requesting that LSP is sent. When the requested LSP later arrives and is stored, the pending-request entry is cleared |
| AC-16 | An LSP arrives with the SAME sequence number as our held copy but remaining lifetime 0 (a purge), our held copy having a non-zero lifetime | The purge is treated as MORE recent (remaining-lifetime tier, before checksum): it is accepted, the held copy marked purged, and the purge re-flooded (SRM on other circuits). A same-sequence purge is NOT discarded as a duplicate |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects three nodes in a line and originates a prefix on the far end | origin LSP (isis-6) -> flood receipt + SRM/SSN (isis-7) -> circuit TX (isis-3) -> next hop receives, floods onward | `TestISISLSDBSync`, `test/isis/isis-flooding.ci` |
| 2 | Brings up a node whose LSDB is behind its neighbour's | CSNP digest received -> gap detection records pending-request entries for LSPs not yet held (SSN only for held entries) -> PSNP request -> neighbour supplies via SRM -> arriving LSPs clear the pending-request entries -> LSDB converges | `TestISISCSNPGapRequest`, `TestCSNPGapRequestPending`, `test/isis/isis-flooding.ci` |
| 3 | Shuts down a node, expecting its routes to clear network-wide | isis-6 purge (lifetime 0) -> isis-7 re-flood -> peers receive purge, age out at MaxAge | `test/isis/isis-flooding.ci` (purge phase) |
| 4 | Connects two nodes over a P2P link and expects instant LSDB sync | adjacency Up -> initial CSNP -> reciprocal PSNP requests -> both LSDBs identical | `test/isis/isis-flooding.ci` (P2P initial-CSNP phase) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFreshnessCompareMatrix` | `internal/component/isis/lsdb/flooding_test.go` | All five ISO/IEC 10589 sec 7.3.15 outcomes: higher seq (accept+SRM+SSN), equal seq diff checksum (request), equal seq same checksum (duplicate), lower seq (send back), purge seq>=ours (accept+re-flood) | |
| `TestSRMDrivenSend` | `internal/component/isis/lsdb/flooding_test.go` | Periodic timer transmits exactly the SRM-set LSPs on each circuit and clears SRM on ack; passive circuit skipped | |
| `TestSRMResendOnNoAck` | `internal/component/isis/lsdb/flooding_test.go` | SRM left set (no ack) causes resend on the next tick | |
| `TestZeroLifetimePurgeReflood` | `internal/component/isis/lsdb/flooding_test.go` | Lifetime-0 LSP with seq >= ours accepted, marked purged, SRM set on other circuits, not deleted locally | |
| `TestCSNPBuildRange` | `internal/component/isis/lsdb/snp_test.go` | CSNP carries TLV 9 entries for the level over the start/end LSPID range; multi-PDU split when the DB exceeds one CSNP | |
| `TestCSNPGapDetection` | `internal/component/isis/lsdb/snp_test.go` | Neighbour-newer-but-held entry sets SSN; older entry sets SRM; equal entry clears SRM and clears any matching pending-request | |
| `TestCSNPGapRequestPending` | `internal/component/isis/lsdb/snp_test.go` | A CSNP listing an LSP ID we do NOT hold records a per-circuit pending-request entry (no SSN, since no LSDB entry exists); a subsequent PSNP includes a TLV 9 request for it; when the LSP later arrives and is stored the pending-request entry is cleared | |
| `TestPSNPRequestAndAck` | `internal/component/isis/lsdb/snp_test.go` | SSN-set held entries produce a PSNP acknowledge; pending-request entries produce a PSNP request; received PSNP at our seq clears SRM; received PSNP request sets SRM | |
| `TestLevelIsolation` | `internal/component/isis/lsdb/flooding_test.go` | L1 and L2 SRM/SSN flags, pending-request sets, and SNP processing do not cross levels | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LSP sequence number (freshness compare) | 1..0xFFFFFFFF | 0xFFFFFFFF | 0 reserved (invalid, never a valid sequence; NOT a purge marker -- purge is remaining-lifetime 0, see the lifetime row) | at 0xFFFFFFFF the originator purges (remaining-lifetime 0) then re-originates from 1; the sequence never wraps to 0 |
| Sequence compare across wrap | n/a | 0xFFFFFFFF newer than 0xFFFFFFFE | n/a | 0xFFFFFFFF then re-origination at 1 after purge, not treated as older |
| Remaining lifetime (purge trigger) | 0..65535 s | 65535 | n/a | 0 == purge, accept+re-flood |
| CSNP LSPID range (start..end LSPID) | 0x00..0x00 to 0xFF..0xFF | full range in one CSNP | n/a | exceeds one PDU -> split into ordered ranges |
| Flood timer interval | 5..30 s (ISO/IEC 10589) | 30 | <5 clamp | >30 clamp |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-flooding` | `test/isis/isis-flooding.ci` | Three-node line floods an LSP end to end; CSNP/PSNP sync two divergent LSDBs; purge re-floods then ages out; P2P initial CSNP syncs a pair | |

### Interop Tests (MANDATORY for protocol features)
<!-- Flooding/SNP correctness against a real peer is proven by the umbrella's FRR scenarios; this spec relies on them rather than duplicating an interop scenario for flooding alone. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-p2p-frr` (umbrella) | `test/interop/scenarios/` | FRR isisd | LSDB sync via flooding + P2P CSNP/PSNP against a real stack | |
| `isis-lan-dis-frr` (umbrella) | `test/interop/scenarios/` | FRR isisd | LAN CSNP cadence + PSNP ack against a real DIS (cadence policy from isis-8) | |

### Future (if deferring any tests)
- A flooding-specific FRR interop scenario is not added separately; the umbrella P2P and LAN scenarios exercise flooding end to end. Add one only if a flooding-only edge case fails interop.

## Files to Modify
- `internal/component/isis/lsdb/lsdb.go` (created by isis-6) - call sites that wire the flood timer and SNP cadence into the LSDB lifecycle, if isis-6 left hooks
- `internal/component/isis/server.go` (created by isis-4/5) - register the per-circuit flood timer and CSNP cadence timer; subscribe to P2P adjacency Up for the initial CSNP

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | flood/CSNP intervals are runtime constants in v1; revisit only if users need tuning |
| YANG validation constraints | No | n/a unless intervals become config leaves |
| YANG custom validators | No | n/a |
| CLI commands/flags | No | flooding counters surface under `show isis database` / metrics (isis-6/isis-13), not a new command here |
| CLI grammar (action before identifier) | No | n/a (no new command) |
| Editor autocomplete | No | n/a |
| Functional test for new RPC/API | Yes | `test/isis/isis-flooding.ci` |
| Pipe completeness | No | n/a (no new output command) |
| Env var registration | No | n/a |
| Doctor check for runtime dependencies | No | transport/socket doctor check is isis-3; flooding adds none |
| Prometheus counters/metrics | Yes | this spec OWNS and registers its rows from the umbrella "Metrics (canonical)" table: `ze_isis_lsps_received_total{level}`, `ze_isis_lsps_transmitted_total{level}`, `ze_isis_csnp_sent_total{level}`, `ze_isis_csnp_received_total{level}`, `ze_isis_psnp_sent_total{level}`, `ze_isis_psnp_received_total{level}`, `ze_isis_srm_resends_total{level}`, `ze_isis_lsps_dropped_total{level,reason}`. Per-owner registration here, not in isis-13 |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | flooding is internal protocol mechanism; user-facing IS-IS row is the umbrella's `docs/features.md` |
| 2 | Config syntax changed? | No | no new config leaves |
| 3 | CLI command added/changed? | No | no new command |
| 4 | API/RPC added/changed? | No | no new RPC |
| 5 | Plugin added/changed? | No | component-internal, no new plugin |
| 6 | Has a user guide page? | No | covered by umbrella `docs/guide/isis.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/isis.md` - CSNP/PSNP/LSP-Entries flooding section |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented? | Yes | `iso/short/iso10589.md` - flooding sec 7.3.14-17 |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - new `test/isis/isis-flooding.ci` |
| 11 | Affects daemon comparison? | No | umbrella owns the IS-IS comparison row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` / IS-IS subsystem note - flooding/SNP responsibility split |
| 13 | Route metadata keys added/changed? | No | flooding touches no route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` - flooding/SNP counters |
| 15 | Registered plugin/event/command/capability changed? | No | no new registration surface |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/isis/lsdb/flooding.go` - LSP receive flooding algorithm (freshness compare, SRM/SSN drive, zero-lifetime purge re-flood) + periodic SRM-driven transmit timer
- `internal/component/isis/lsdb/flooding_test.go` - freshness matrix, SRM-driven send, resend-on-no-ack, purge re-flood, level isolation
- `internal/component/isis/lsdb/snp.go` - CSNP/PSNP build, send, and receive; per-circuit pending-request set for LSPs not yet held; CSNP gap detection -> SRM (we-newer) / SSN (held-stale) / pending-request (not held); PSNP ack (SSN) + request (pending-request set); P2P initial CSNP
- `internal/component/isis/lsdb/snp_test.go` - CSNP build/range, gap detection, pending-request request/clear, PSNP request/ack
- `test/isis/isis-flooding.ci` - functional flood + CSNP/PSNP sync + purge over a three-node line and a P2P pair

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-isis-6-lsdb, spec-isis-3-l2-transport, spec-isis-2-wire |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan - check what isis-6/isis-3/isis-2 already provide |
| 3. Wiring phase | Wiring Test table - register RX handlers + timers, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register LSP/CSNP/PSNP receive handlers with the isis-4 PDU receive dispatcher (umbrella Shared Contracts) and create stubbed flood + CSNP timers; write the failing wiring tests
   - Tests: `TestISISLSDBSync` (fails - no flooding logic yet)
   - Files: `internal/component/isis/lsdb/flooding.go` (handler skeletons), `internal/component/isis/lsdb/snp.go` (timer + pending-request skeletons), server.go timer + dispatcher-handler registration
   - Verify: handlers are reachable from the isis-4 dispatcher; timers fire; tests fail because compare/transmit are stubs
2. **Phase: Freshness comparison** - implement the ISO/IEC 10589 sec 7.3.15 decision table against the isis-6 LSDB and SRM/SSN flag API
   - Tests: `TestFreshnessCompareMatrix`, boundary sequence/wrap/purge tests
   - Files: `flooding.go`
   - Verify: all five outcomes correct, including purge and wraparound
3. **Phase: SRM-driven transmit** - periodic flood timer drains SRM, transmits stored raw LSP bytes, clears SRM on ack, resends when unacknowledged, skips passive circuits
   - Tests: `TestSRMDrivenSend`, `TestSRMResendOnNoAck`, `TestZeroLifetimePurgeReflood`
   - Files: `flooding.go`
   - Verify: SRM lifecycle correct; no re-flood storm; purge re-floods without local deletion
4. **Phase: CSNP build/send/receive** - enumerate LSDB into TLV 9 entries over the LSPID range; gap detection sets SRM (we-newer), SSN (held-stale), or a per-circuit pending-request entry (LSP not held at all); equal entry clears SRM and any matching pending-request; P2P initial CSNP on adjacency Up
   - Tests: `TestCSNPBuildRange`, `TestCSNPGapDetection`, `TestCSNPGapRequestPending`
   - Files: `snp.go`
   - Verify: gaps for not-held LSPs become pending-request entries, older-ours supplied, equal acknowledged; multi-PDU range split correct
5. **Phase: PSNP build/send/receive** - PSNP request/ack built from both SSN (held LSPs) and the per-circuit pending-request set (LSPs not yet held); received PSNP clears SRM (ack) and sets SRM (request); arriving LSPs clear matching pending-request entries
   - Tests: `TestPSNPRequestAndAck`, `TestISISCSNPGapRequest`, `TestISISPSNPAck`
   - Files: `snp.go`
   - Verify: ack clears SRM; request supplies LSP; SSN cleared on PSNP send for held LSPs; pending-request entries drained into PSNP requests and cleared on LSP arrival
6. **Phase: Level isolation + wiring closure** - confirm L1/L2 independence; make `TestISISLSDBSync` pass end to end
   - Tests: `TestLevelIsolation`, `TestISISLSDBSync`
   - Files: `flooding.go`, `snp.go`
   - Verify: three-node line converges; L1/L2 do not cross
7. **Functional test** - `test/isis/isis-flooding.ci`: three-node flood, CSNP/PSNP divergent sync, purge re-flood + age-out, P2P initial CSNP
8. **Metrics** - register HERE (per-owner) the flooding/SNP series from the umbrella "Metrics (canonical)" table (`ze_isis_lsps_received_total`, `ze_isis_lsps_transmitted_total`, `ze_isis_csnp_sent_total`/`_received_total`, `ze_isis_psnp_sent_total`/`_received_total`, `ze_isis_srm_resends_total`, `ze_isis_lsps_dropped_total`, all `{level}`); isis-13 only scrapes/asserts them
9. **RFC refs** - `// ISO/IEC 10589 Section 7.3.14-17` comments above the comparison table, SRM/SSN transitions, purge re-flood, SNP processing
10. **Full verification** - `make ze-verify`
11. **Complete spec** - fill audit tables, write learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-16 has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path; flooding matches ISO/IEC 10589 reliable-flooding behavior |
| Correctness | Freshness compare matches ISO/IEC 10589 sec 7.3.15 exactly; SRM/SSN transitions per sec 7.3.16-17; purge re-flood per sec 7.3.14 |
| Naming | Internal package names only; no new YANG/CLI surface to name here |
| Data flow | LSP/CSNP/PSNP arrive via the isis-4 PDU dispatcher (no own PDU-type switch); Flood TX re-uses stored raw LSP bytes; SNP encode buffer-first; no SPF/redistribute import; flooding owns no LSDB store (only the per-circuit pending-request set for not-yet-held LSPs) |
| CLI grammar | n/a (no new command) |
| Doctor checks | n/a (transport doctor check is isis-3) |
| YANG validation | n/a (no new leaf) |
| Prometheus counters | Flooding/SNP series registered HERE (per-owner) with the exact names from the umbrella "Metrics (canonical)" table; isis-13 only scrapes/asserts, it does not register them |
| Rule: plugin-self-containment | All flooding/SNP code under `internal/component/isis/lsdb/`; no flooding spelling in isis-2/isis-3 |
| Rule: buffer-first | No per-LSP allocation on the periodic flood timer; CSNP/PSNP encode via `WriteTo(buf, off) int` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| flooding.go + snp.go | `ls internal/component/isis/lsdb/flooding.go internal/component/isis/lsdb/snp.go` |
| Unit tests | `go test ./internal/component/isis/lsdb/ -run 'Freshness|SRM|CSNP|PSNP|Level'` |
| Wiring test | `go test ./internal/component/isis/... -run TestISISLSDBSync` |
| Functional test | `ls test/isis/isis-flooding.ci` and run via the .ci runner |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every received LSP/CSNP/PSNP TLV length bound-checked before slicing (delegated to isis-2 parse; flooding asserts parse success before acting) |
| Resource exhaustion | Flood rate limited by the periodic timer; SRM resend does not unbounded-loop; CSNP range split caps PDU size; reject implausibly large LSP counts; per-circuit pending-request set bounded (deduplicated by LSPID, aged/capped) so a malicious CSNP cannot grow it without bound |
| Spoofing | Act only on LSPs for the level/area the adjacency authorises (level checks delegated to isis-5/isis-6); never re-flood onto the incoming circuit |
| Stale/replay | Lower-sequence or stale-checksum LSPs never replace newer; purge handled per RFC, no premature deletion |
| Authentication | TLV 10 verify is isis-10; flooding must not bypass it (verify before accept once isis-10 lands) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read ISO/IEC 10589 sec 7.3.14-17 summary / Current Behavior contract from isis-6 |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Interop mismatch | Capture with tcpdump, compare CSNP/PSNP/LSP to FRR, fix compare/SNP logic |
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
<!-- LIVE - write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Flooding drives the SRM/SSN flags owned by isis-6 rather than storing its own state | Flooding keeps a parallel per-circuit send map | One source of truth; isis-6 owns the LSDB entry incl. flags; isis-7 is a flag-to-wire pump |
| A per-circuit pending-request set, owned by this spec, represents CSNP-driven requests for LSPs not yet held; SSN keeps its acknowledge semantics on held LSPs only | Set SSN on the LSP we lack (impossible: no LSDB entry exists to carry the flag); create a placeholder stub LSDB entry marked "requested" (blurs the isis-6/isis-7 split, pollutes the LSDB with non-LSP entries) | An LSP we do not hold has no LSDB entry, so SSN cannot represent the request; a small per-circuit request set keeps the LSDB clean and lets a PSNP request LSPs we do not yet hold |
| Receive LSP/CSNP/PSNP via the isis-4 PDU dispatcher | flooding registers its own PDU-type switch with isis-3 | Single dispatcher (umbrella Shared Contracts); transport stays protocol-agnostic; no duplicated PDU-type switch |
| Re-flood by re-transmitting stored raw LSP bytes | Re-encode from parsed TLVs | Buffer-first; preserves unknown TLVs verbatim (ISO/IEC 10589 sec 7.3.14) |
| CSNP/PSNP mechanism here, LAN cadence policy in isis-8 | Put all SNP code in isis-8 | DIS is broadcast-specific; the SNP build/send/receive is needed on P2P too, so it belongs with flooding |

## Known Limitations
<!-- Source for learned summary Consequences section. -->
- Flood/CSNP intervals are runtime constants in v1 (RFC-recommended 5 s flood, level-appropriate CSNP); not yet tunable via YANG.
- Mesh groups (RFC 2973 flooding scope reduction) are out of scope (umbrella out-of-scope table).
- LAN CSNP cadence and DIS-only periodic CSNP sourcing are policy supplied by isis-8; this spec provides only the mechanism.

## RFC Documentation

Add `// ISO/IEC 10589 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: the sec 7.3.15 comparison decision table, SRM/SSN set/clear transitions (sec 7.3.16-17), zero-lifetime purge re-flood (sec 7.3.14), and the "do not re-flood on the incoming circuit" rule.

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
| Reliable flooding propagates LSPs network-wide | functional test | `test/isis/isis-flooding.ci` (three-node line) |
| CSNP/PSNP synchronise divergent LSDBs | functional test | `test/isis/isis-flooding.ci` (sync phase) + `TestISISCSNPGapRequest` |
| Purge re-floods then ages out | functional test | `test/isis/isis-flooding.ci` (purge phase) |

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/isis/lsdb/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (out-of-scope honoured)
- [ ] Single responsibility (flooding/SNP only; LSDB store stays in isis-6)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (via umbrella scenarios)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-7-flooding.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-isis-7-flooding.md` only
