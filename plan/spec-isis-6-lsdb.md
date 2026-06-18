# Spec: isis-6-lsdb

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-5-adjacency.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - umbrella scope, package layout, design principles (row "isis-6")
4. `docs/research/isis-implementation-guide.md` sec 5 "LSP Database and SPF" (LSDB model, aging, origination, fragmentation) and sec 12 items 1-9 (checksum, wraparound, zero-age purge, TLV ordering, fragmentation, up/down bit)
5. `ai/rules/buffer-first.md` - lazy raw-bytes storage, `WriteTo(buf, off) int` encode
6. `plan/spec-isis-5-adjacency.md` - adjacency table that feeds origination (TLV 22 neighbours)
7. `plan/spec-isis-2-wire.md` - PDU/TLV codec, Fletcher checksum, opaque-TLV passthrough
8. `plan/spec-isis-1-types.md` - SystemID, LSPID, sequence number, lifetime types

## Task

Implement the IS-IS Link-State Database (LSDB) store and own-LSP origination for
the `internal/component/isis/lsdb/` package. This is phase 6 of the IS-IS spec
set (`spec-isis-0-umbrella.md`). The LSDB is the in-memory record of every
Link-State PDU known to the node, held separately for Level 1 and Level 2. It
must store each LSP per the Ze buffer-first philosophy: the raw PDU bytes plus a
small parsed metadata header (LSPID, sequence number, remaining lifetime,
checksum), parsing TLVs only on demand. Storing raw bytes lets the node
re-flood LSPs (including ones carrying TLVs it does not understand) verbatim,
matching ISO/IEC 10589 sec 7.3.14 and Ze's zero-copy `WireUpdate` model.

This spec owns the database itself and the origination of the node's *own* LSPs.
It does NOT own the flooding algorithm, CSNP/PSNP synchronisation, or the wire
transmission of LSPs: that is the sibling spec `spec-isis-7-flooding`. To keep
the two specs cleanly separable, this spec defines the data model that flooding
consumes: per-LSP, per-circuit SRM (Send Routing Message) and SSN (Send Sequence
Number) flag bitmaps, with the operations to set, clear, and query them. The
flooding spec drives those flags; this spec only stores and exposes them.

Own-LSP origination builds the node's L1 and/or L2 LSPs from live state:
adjacencies (Extended IS Reachability, TLV 22), connected and redistributed
IPv4 prefixes (Extended IP Reachability, TLV 135; IPv6 TLV 236 is wired in
`spec-isis-12-ipv6`), Area Addresses (TLV 1), Protocols Supported (TLV 129),
the router's IPv4 interface addresses (IP Interface Address, TLV 132, RFC 1195;
IPv6 TLV 232 is added in `spec-isis-12-ipv6`), Dynamic Hostname (TLV 137,
RFC 5301), and the overload bit (RFC 3787). LSPs are
regenerated on topology change, assigned and incremented sequence numbers, aged
down once per second, refreshed before expiry, purged at zero lifetime, and
fragmented across LSP numbers 0..255 when they exceed the maximum PDU size.

Ze has no IS-IS today; this package is entirely new. It depends on
`spec-isis-5-adjacency` (the adjacency table feeds TLV 22 origination), the
`spec-isis-2-wire` codec (PDU/TLV encode/decode and the Fletcher checksum), and
the `spec-isis-1-types` domain types (SystemID, LSPID, sequence, lifetime). It
exposes a `show isis database` snapshot API consumed and rendered in
`spec-isis-13-cli-diag-interop`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]: checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations; these survive compaction. -->
- [ ] `docs/research/isis-implementation-guide.md` sec 5 "LSP Database and SPF" - LSDB data model, aging routine, origination triggers, fragmentation
  -> Decision: store raw LSP bytes + parsed metadata (LSPID, sequence, lifetime, checksum); parse TLVs lazily, never eagerly into structs (bio-rd does eager; Ze does lazy)
  -> Constraint: lazy storage - the LSDB entry holds the verbatim PDU so unknown TLVs re-flood unchanged (ISO/IEC 10589 sec 7.3.14)
  -> Constraint: aging runs once per second over all entries; decrement remaining lifetime by 1; at 0, do NOT delete immediately
- [ ] `docs/research/isis-implementation-guide.md` sec 12 items 1-9 - known hard problems
  -> Constraint: sequence wraparound (ISO/IEC 10589 sec 7.3.3) - at 0xFFFFFFFF stop originating, purge, wait MaxAge + ZeroAgeLifetime, then re-originate from sequence 1
  -> Constraint: zero-age purge (item 3 and 8) - a remaining-lifetime-0 LSP must be flooded as a purge and kept for a grace period (ZeroAgeLifetime), NOT deleted at once; distinguish a received purge (re-flood) from local expiry (garbage collect)
  -> Constraint: fragmentation (item 7) - split own state across LSP numbers 0..255 (256 fragments; fragment 0 carries the non-fragmentable fields and is valid, RFC 3786) when it exceeds maxPDUSize; each fragment carries its own sequence number and checksum
  -> Constraint: overload bit (RFC 3787, item 9 context) - set in the LSP header attribute byte; SPF must avoid transiting an overloaded node (honouring noted here, enforced in `spec-isis-9-spf-rib`)
- [ ] `ai/rules/buffer-first.md` - zero-copy, lazy parse, no-alloc encode
  -> Constraint: LSP encode is `WriteTo(buf, off) int` into a pooled buffer; never `buildLSP() []byte`; the checksum is backfilled at its fixed header offset after the body is written
  -> Constraint: stored raw bytes come from the receive buffer or the build buffer copied once into the entry; TLV parse for SPF/CLI is on demand
- [ ] `plan/spec-isis-0-umbrella.md` - package layout, design principles, dependency graph (row "isis-6")
  -> Constraint: package is `internal/component/isis/lsdb/`; it imports `packet` (codec) and `types`, never the runtime/circuit layer above it
  -> Constraint: wide metrics only (RFC 5305); the TLV 22 IS-reachability metric is 24-bit, the TLV 135 prefix metric is 32-bit (4-octet field); narrow metrics are not originated

### RFC Summaries (MUST for protocol work)
- [ ] `iso/short/iso10589.md` - IS-IS base (created in earlier child)
  -> Constraint: sec 7.3.3 sequence wraparound (sequence 0 is reserved, never originated; origination starts at 1); sec 7.3.4/7.3.5 SRM/SSN flag semantics; sec 7.3.11 checksum; sec 7.3.12 origination triggers; sec 7.3.14 re-flood unknown TLVs verbatim; sec 7.3.16/7.3.17 LSP aging and purge signalled by remaining-lifetime 0 (NOT sequence 0) with ZeroAgeLifetime
- [ ] `rfc/short/rfc5305.md` - wide metrics, TLV 22 (Extended IS Reachability, 24-bit IS metric), TLV 135 (Extended IP Reachability, 32-bit prefix metric)
  -> Constraint: origination uses wide-metric TLV 22 (24-bit IS-reachability metric) and TLV 135 (32-bit prefix metric in a 4-octet field); up/down bit lives in the TLV 135 control octet, not the metric (set on L1<->L2 leak, applied in `spec-isis-9-spf-rib`)
- [ ] `rfc/short/rfc1195.md` - IS-IS for IP: Protocols Supported TLV 129 (NLPID), IP Interface Address TLV 132
  -> Constraint: an IP-capable router includes TLV 132 in its own LSPs carrying its IPv4 interface addresses; peers use these for next-hop resolution during SPF
- [ ] `rfc/short/rfc5301.md` - Dynamic Hostname TLV 137
  -> Constraint: own LSP advertises the node hostname in TLV 137 (advertise here; display in `spec-isis-13-cli-diag-interop`)
- [ ] `rfc/short/rfc3787.md` - overload bit
  -> Constraint: overload bit set in the LSP header; node still reachable but not transited

**Key insights:** (minimal context to resume after compaction)
- LSDB = two maps (L1, L2), keyed by LSPID, each entry = raw bytes + metadata (LSPID, sequence, remaining lifetime, checksum) + per-circuit SRM/SSN bitmaps
- Aging = 1s tick decrementing remaining lifetime; refresh at lsp-refresh-interval (default 900s); MaxAge = lsp-lifetime (default 1200s)
- Wraparound = at 0xFFFFFFFF purge and wait MaxAge + ZeroAgeLifetime before re-originating from 1
- Purge != expiry: a zero-lifetime LSP is kept for a grace period and re-flooded, not deleted at once
- Fragmentation = own state split across LSP numbers 0..255, each fragment a separate LSP with its own sequence and checksum
- Origination is full regeneration on topology change, not incremental

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/wireu/wire_update.go` - BGP `WireUpdate` lazy/zero-copy model: raw bytes held, fields parsed on demand
  -> Constraint: mirror it for the LSDB - store raw LSP bytes, parse TLVs lazily; never eager-parse into structs
- [ ] `internal/component/bgp/plugins/nlri/ls/types.go` - BGP-LS carries LS topology inside BGP NLRI, not the IGP LSDB
  -> Constraint: the IS-IS LSDB is independent of BGP-LS; do not couple
- [ ] `internal/component/isis/lsdb/lsdb.go` - does not exist yet; Ze has no IS-IS protocol and no LSDB
  -> Constraint: this package is created wholesale; nothing to preserve in-tree

**Behavior to preserve:**
- BGP, LDP, RSVP-TE, static, connected route sources remain independent and functional (the LSDB does not touch them)
- The adjacency table and FSM from `spec-isis-5-adjacency` are consumed read-only by origination; no change to adjacency semantics

**Behavior to change:**
- None existing. New `internal/component/isis/lsdb/` package: LSDB store, origination, aging, SRM/SSN flag model, `show isis database` snapshot

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Received LSP PDUs arrive from the flooding/receive path (`spec-isis-7-flooding`) as raw bytes already validated by the `spec-isis-2-wire` codec
- Topology-change signals arrive from `spec-isis-5-adjacency` (adjacency Up/Down) and from connected/redistributed prefix sources, triggering origination
- Timer ticks (1s aging, refresh, wraparound-wait) arrive from the LSDB's own timer goroutine

### Transformation Path
1. **LSP receive:** raw LSP bytes -> parse header for freshness metadata (LSPID, sequence number, remaining lifetime, checksum) -> compare against any existing entry -> store raw + metadata (newer replaces older; equal updates lifetime; older is ignored and triggers SSN/SRM per flooding rules)
2. **Topology change:** adjacency or prefix change -> regenerate own LSP set per level -> assign/increment sequence number -> compute checksum -> store own entry -> set SRM on all eligible circuits (signal to `spec-isis-7-flooding` to flood)
3. **Aging:** 1s tick -> decrement remaining lifetime of every entry -> at 0, mark purged (not delete), flood as purge, start ZeroAgeLifetime grace timer -> at grace expiry, delete
4. **Refresh:** own LSP nearing MaxAge (at lsp-refresh-interval) -> increment sequence -> recompute checksum -> reset remaining lifetime to MaxAge -> set SRM
5. **Wraparound:** own LSP sequence reaches 0xFFFFFFFF -> purge -> suspend origination of that LSPID for MaxAge + ZeroAgeLifetime -> re-originate from sequence 1
6. **Snapshot:** `show isis database` request -> read-locked copy of metadata (and lazily parsed TLV summaries) per level

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire codec <-> LSDB | parsed metadata header + verbatim raw LSP bytes (`spec-isis-2-wire`) | [ ] |
| Adjacency <-> origination | read-only adjacency table snapshot -> TLV 22 neighbours (`spec-isis-5-adjacency`) | [ ] |
| LSDB <-> flooding | SRM/SSN per-circuit bitmaps set here, consumed by `spec-isis-7-flooding` | [ ] |
| LSDB <-> SPF | lazy TLV parse of stored LSPs (`spec-isis-9-spf-rib`) | [ ] |
| LSDB <-> CLI | `show isis database` snapshot (`spec-isis-13-cli-diag-interop`) | [ ] |

### Integration Points
- `spec-isis-7-flooding` reads/clears SRM and SSN flags and transmits LSPs/CSNP/PSNP
- `spec-isis-8-dis-broadcast` originates pseudo-node LSPs into this LSDB (same store, same SRM/SSN model) and lists the pseudo-node as a neighbour in the node's own LSP
- `spec-isis-9-spf-rib` reads the LSDB (lazy TLV parse) to build the SPF graph and honours the overload bit
- `spec-isis-12-ipv6` adds TLV 236/232 to origination (IPv6 reachability and interface address)
- `spec-isis-5-adjacency` supplies the neighbours that become TLV 22 entries

### Architectural Verification
- [ ] No bypassed layers (raw bytes -> codec metadata -> LSDB entry; origination -> codec encode -> entry; never hand-rolled outside `packet`)
- [ ] No unintended coupling (LSDB independent of BGP-LS; imports only `packet` and `types`)
- [ ] No duplicated functionality (uses the `spec-isis-2-wire` codec and checksum; does not reimplement encode/parse)
- [ ] Zero-copy preserved (entries hold raw bytes; TLV parse is on demand; encode is buffer-first `WriteTo`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `spec-isis-2-wire` codec exposes both a metadata-only header parse and a full TLV parse so the LSDB can read freshness fields without parsing the whole PDU | umbrella package layout; buffer-first lazy model | LSDB must parse full PDU on every receive (defeats lazy storage) | isis-2 codec API review; unit test parsing header only | unvalidated |
| A-2 | The adjacency table from `spec-isis-5-adjacency` is queryable as a read-only snapshot at origination time | umbrella dependency (isis-6 depends on isis-5) | Origination needs a different adjacency access pattern | origination unit test against a fake adjacency table | unvalidated |
| A-3 | Per-circuit SRM/SSN bitmaps can be indexed by a stable small circuit index (one bit per circuit) | research guide sec 4/5 (bio-rd indexes flags by interface) | Need a map keyed by circuit ID instead of a bitmap | flag-ops unit test with multiple circuits | unvalidated |
| A-4 | MaxAge (lsp-lifetime), refresh (lsp-refresh-interval) and ZeroAgeLifetime are configurable leaves resolved by `spec-isis-4-component-config` with the RFC defaults (1200s / 900s / grace) | umbrella config decisions | Timers hardcoded; no operator override | config resolve check; aging test with injected short timers | unvalidated |
| A-5 | maxPDUSize for fragmentation is derivable from circuit MTU (default 1492 for Ethernet) provided by `spec-isis-3-l2-transport` | research guide sec 5/12 item 7 | Fragmentation threshold wrong; oversized LSPs | fragmentation unit test with a forced small max | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Sequence wraparound mishandled (re-originate before purge propagates) causes flap or loop | soak/chaos LSP flap after a long-running session | Explicit wraparound test: at 0xFFFFFFFF, assert purge + suspension window + re-origin from 1 |
| R-2 | Purge confused with local expiry; stale LSP lingers or valid purge dropped | LSDB shows a dead LSP that never clears, or a peer keeps an old LSP | Distinct purged state + ZeroAgeLifetime grace; separate code paths for received-purge vs local-expiry |
| R-3 | Fragmentation boundary off-by-one; a fragment exceeds maxPDUSize or splits a TLV mid-value | encode overruns or peer rejects fragment | Fragmentation test forcing multiple fragments; never split within a single TLV value |
| R-4 | Checksum recomputed at the wrong time (before all TLVs assembled, or not on refresh) | interop checksum failures | Compute checksum as the final encode step; recompute on every sequence increment |
| R-5 | Aging tick races with origination/flooding readers | data race under `-race`; intermittent test failures | Single writer for the LSDB (RWMutex or owning goroutine); snapshot reads under read lock |

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| adjacency reaches Up (from `spec-isis-5-adjacency`) | → | origination builds own LSP, stores it, sets SRM on eligible circuits | `TestISISOriginateOnAdjacencyUp` |
| own LSP stored | → | LSDB lookup returns the stored raw bytes + metadata | `TestISISLSDBStoreRetrieve` |
| received LSP (newer) | → | freshness compare replaces entry, sets SSN/SRM per flooding contract | `TestISISLSDBReceiveNewer` |
| 1s aging tick reaches lifetime 0 | → | entry marked purged (not deleted), purge flood signalled | `TestISISLSDBAgeToPurge` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Node has one Up adjacency and one connected prefix | Own LSP originated for the configured level(s) with a valid (non-zero) sequence number and a valid Fletcher checksum, containing TLV 1 (area), TLV 129 (protocols supported), TLV 22 (the neighbour), TLV 132 (the router's IPv4 interface addresses, RFC 1195), TLV 135 (the prefix), and TLV 137 (hostname) |
| AC-2 | Own LSP exists; 1s aging runs to remaining lifetime 0 | The LSP is marked purged and signalled for purge flooding, kept in the LSDB for the ZeroAgeLifetime grace period, then deleted; it is NOT deleted at the instant lifetime hits 0 |
| AC-3 | Own LSP reaches lsp-refresh-interval before MaxAge | Sequence number is incremented, checksum recomputed, remaining lifetime reset to MaxAge, SRM set for re-flood |
| AC-4 | Own LSP sequence number reaches 0xFFFFFFFF | The LSP is purged and origination of that LSPID is suspended for MaxAge + ZeroAgeLifetime, after which it re-originates from sequence number 1 |
| AC-5 | Own state exceeds maxPDUSize for one LSP | State is split across LSP numbers 0..255; each fragment is a distinct LSP with its own sequence number and checksum; no single TLV is split across fragments |
| AC-6 | Overload condition configured/triggered | The overload bit is set in the originated LSP header; SPF must not transit this node (enforcement noted, implemented in `spec-isis-9-spf-rib`) |
| AC-7 | A received LSP carries a TLV the node does not understand | The LSP is stored verbatim and re-floodable byte-for-byte (ISO/IEC 10589 sec 7.3.14); the unknown TLV is preserved |
| AC-8 | A received LSP is newer / equal / older than the stored entry | Newer replaces and is marked for flood (SRM); equal updates the lifetime and is acknowledged (SSN); older is not stored and the receiving circuit is marked to send the newer copy back |
| AC-9 | A received purge (remaining lifetime 0, originator sequence) vs a locally expired LSP | The purge is re-flooded and retained for the grace period; the locally expired LSP is garbage-collected; the two are handled by distinct paths |
| AC-10 | `show isis database` for L1 and L2 | A snapshot lists each LSPID with sequence number, remaining lifetime, checksum, and overload flag for the requested level |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up an IS-IS adjacency and expects the node to advertise itself | adjacency Up -> origination -> LSDB store -> SRM set -> (flood in `spec-isis-7-flooding`) | `TestISISOriginateOnAdjacencyUp`, `test/isis/isis-lsdb-sync.ci` (isis-7) |
| 2 | Leaves a node idle long enough for an LSP to age out | aging tick -> lifetime 0 -> purge -> grace -> delete | `TestISISLSDBAgeToPurge` |
| 3 | Runs `show isis database` to inspect the LSDB | CLI -> RPC -> LSDB snapshot -> render (isis-13) | `test/isis/isis-show.ci` (isis-13), `TestISISLSDBSnapshot` |
| 4 | Configures the overload bit during maintenance | config -> origination sets overload -> SPF avoids transit | `TestISISOriginateOverloadBit` + SPF honour in `spec-isis-9-spf-rib` |
| 5 | Advertises enough prefixes to require fragmentation | origination -> exceeds maxPDUSize -> split across LSP numbers | `TestISISOriginateFragmentation` |

<!-- Functional/interop coverage for the full sync path is owned by spec-isis-7-flooding
     (isis-lsdb-sync.ci) and spec-isis-9-spf-rib (isis-route-install.ci); this spec's
     functional surface is the show snapshot, exercised in isis-13. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISLSDBStoreRetrieve` | `internal/component/isis/lsdb/lsdb_test.go` | insert an entry, retrieve raw bytes + metadata by LSPID, per-level isolation (L1 vs L2) | |
| `TestISISLSDBReceiveNewer` | `internal/component/isis/lsdb/lsdb_test.go` | freshness compare: newer replaces, equal updates lifetime, older rejected; flag effects | |
| `TestISISLSDBStoreVerbatim` | `internal/component/isis/lsdb/lsdb_test.go` | stored bytes for an LSP with an unknown TLV round-trip byte-for-byte (re-flood verbatim) | |
| `TestISISLSDBAgeDecrement` | `internal/component/isis/lsdb/aging_test.go` | 1s tick decrements every entry's remaining lifetime by 1 | |
| `TestISISLSDBAgeToPurge` | `internal/component/isis/lsdb/aging_test.go` | lifetime 0 -> marked purged, not deleted; deleted only after ZeroAgeLifetime grace | |
| `TestISISLSDBPurgeVsExpiry` | `internal/component/isis/lsdb/aging_test.go` | received purge re-flooded + retained; local expiry garbage-collected; distinct paths | |
| `TestISISOriginateOnAdjacencyUp` | `internal/component/isis/lsdb/origination_test.go` | own LSP built from a fake adjacency table with valid sequence + checksum + TLVs 1/129/22/132/135/137 | |
| `TestISISOriginateRegenOnChange` | `internal/component/isis/lsdb/origination_test.go` | adjacency/prefix change regenerates the full own LSP set and increments the sequence number | |
| `TestISISRefreshIncrementsSeq` | `internal/component/isis/lsdb/origination_test.go` | refresh timer increments sequence, recomputes checksum, resets lifetime, sets SRM | |
| `TestISISSequenceWraparound` | `internal/component/isis/lsdb/origination_test.go` | at 0xFFFFFFFF: purge, suspend for MaxAge + ZeroAgeLifetime, re-originate from 1 | |
| `TestISISOriginateFragmentation` | `internal/component/isis/lsdb/origination_test.go` | own state over maxPDUSize splits across LSP numbers 0..255; no TLV split mid-value | |
| `TestISISOriginateOverloadBit` | `internal/component/isis/lsdb/origination_test.go` | overload condition sets the overload bit in the LSP header | |
| `TestISISSRMSSNFlagOps` | `internal/component/isis/lsdb/lsdb_test.go` | per-circuit SRM/SSN set/clear/query; independent per circuit; cleared on entry removal | |
| `TestISISLSDBSnapshot` | `internal/component/isis/lsdb/lsdb_test.go` | snapshot returns per-level LSPID + sequence + lifetime + checksum + overload | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LSP sequence number | 1..0xFFFFFFFF | 0xFFFFFFFF | 0 (reserved/invalid; origination starts at 1; sequence 0 is NOT a purge) | wraps -> triggers purge (remaining-lifetime 0) + re-origin from 1 |
| Remaining lifetime | 0..65535 s | 65535 | N/A (0 is the purge signal, valid) | 65536 |
| LSP number (fragment) | 0..255 | 255 | N/A | 256 |
| IS-reachability metric (TLV 22, 24-bit) | 0..16777215 | 16777215 | N/A | 16777216 |
| IPv4 prefix metric (TLV 135, 32-bit, via isis-12 also TLV 236) | 0..4294967295 | 4294967295 | N/A | N/A (full 4-octet field) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (covered via isis-7) | `test/isis/isis-lsdb-sync.ci` | two nodes exchange LSPs; both LSDBs converge | |
| (covered via isis-9) | `test/isis/isis-route-install.ci` | LSDB feeds SPF; remote prefix installed | |
| (show snapshot) | `test/isis/isis-show.ci` (isis-13) | `show isis database` renders L1/L2 entries | |

<!-- This spec adds no standalone .ci file: its end-to-end behavior is only observable
     once flooding (isis-7) carries LSPs and SPF (isis-9) consumes them. The functional
     surface owned here (the show snapshot) is exercised by isis-13's isis-show.ci. -->

### Interop Tests (MANDATORY for protocol features)
<!-- Wire/interop verification for LSP exchange is owned by the flooding and SPF children,
     which is where LSPs actually traverse the wire. This spec is the in-memory store +
     origination; it has no independent on-the-wire surface. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (covered via isis-7) | `test/interop/scenarios/isis-p2p-frr` | FRR isisd | FRR accepts our originated LSPs (valid seq/checksum/TLVs) and we store FRR's | |
| (covered via isis-9) | `test/interop/scenarios/isis-p2p-frr` | FRR isisd | SPF over the synced LSDB converges to FRR-equivalent routes | |

### Future (if deferring any tests)
- IPv6 reachability (TLV 236) and interface-address (TLV 232) origination tests land with `spec-isis-12-ipv6`
- Pseudo-node LSP origination tests land with `spec-isis-8-dis-broadcast`

## Files to Modify
<!-- This spec is almost entirely new-file. Modifications are limited to shared wiring already created by isis-4. -->
- `internal/component/isis/server.go` - start the LSDB aging/refresh timer goroutine and pass the LSDB handle to circuits (skeleton from `spec-isis-4-component-config`)
- `internal/component/isis/events.go` - subscribe origination to adjacency Up/Down and prefix-change events

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | lsp-lifetime / lsp-refresh-interval / overload-bit leaves belong to `ze-isis-conf.yang` (added in `spec-isis-4-component-config`); this spec consumes them, does not define schema |
| YANG validation constraints | No | range on lifetime/refresh leaves (owned by isis-4) |
| YANG custom validators | No | none for this spec |
| CLI commands/flags | No | `show isis database` registered/rendered in `spec-isis-13-cli-diag-interop`; this spec exposes the snapshot API only |
| CLI grammar (action before identifier) | No | `ai/rules/cli-grammar.md` (applied in isis-13) |
| Editor autocomplete | No | N/A (no new config leaves here) |
| Functional test for new RPC/API | No | snapshot exercised via `test/isis/isis-show.ci` (isis-13) |
| Pipe completeness | No | `show isis database` output through `ApplyPipes`/`ProcessPipes` (isis-13) |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | none new (raw socket / CAP_NET_RAW owned by `spec-isis-3-l2-transport`) |
| Prometheus counters/metrics | Yes | this spec OWNS and registers its rows from the umbrella `## Shared Contracts` "Metrics (canonical)" table: `ze_isis_lsps{level}` (LSDB size), `ze_isis_lsp_fragments{level}`, `ze_isis_lsp_originations_total{level}`, `ze_isis_sequence_wraps_total{level}`, `ze_isis_purges_total{level}`. Per-owner registration here, NOT in isis-13 (isis-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | LSDB is internal; user-facing surface (`show isis database`) documented in isis-13 |
| 2 | Config syntax changed? | No | lsp-lifetime/refresh/overload leaves documented with isis-4 |
| 3 | CLI command added/changed? | No | `show isis database` documented in isis-13 |
| 4 | API/RPC added/changed? | No | snapshot RPC documented in isis-13 |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | `docs/guide/isis.md` (owned by isis-13) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/isis.md` - LSP origination contents (TLVs 1/22/129/132/135/137, overload bit), aging/purge, fragmentation |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `iso/short/iso10589.md` (7.3.3 wraparound, 7.3.11 checksum, 7.3.16/17 aging/purge), `rfc/short/rfc5305.md` (TLV 22 24-bit metric / TLV 135 32-bit prefix metric), `rfc/short/rfc1195.md` (TLV 129/132), `rfc/short/rfc5301.md` (TLV 137), `rfc/short/rfc3787.md` (overload) |
| 10 | Test infrastructure changed? | No | new tests live under existing `internal/component/isis/lsdb/` |
| 11 | Affects daemon comparison? | No | IS-IS comparison row owned by isis-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` or an IS-IS subsystem doc - LSDB store + origination design (lazy raw-bytes model) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` - LSDB series (`ze_isis_lsps`, `ze_isis_lsp_fragments`, `ze_isis_lsp_originations_total`, `ze_isis_sequence_wraps_total`, `ze_isis_purges_total`) owned and registered HERE per the umbrella canonical table; isis-13 only scrapes/surfaces |
| 15 | Registered plugin/event/command/capability changed? | No | |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/isis/lsdb/lsdb.go` - the LSDB store: two per-level databases keyed by LSPID, insert/lookup/delete, freshness compare, per-circuit SRM/SSN flag bitmaps, snapshot API
- `internal/component/isis/lsdb/entry.go` - the per-LSP entry: raw bytes + parsed metadata (LSPID, sequence number, remaining lifetime, checksum, overload), purged-state marker, lazy TLV accessors
- `internal/component/isis/lsdb/origination.go` - own-LSP build from adjacencies (TLV 22) + connected/redistributed prefixes (TLV 135) + the router's IPv4 interface addresses (TLV 132) + TLV 1/129/137 + overload bit; sequence assignment/increment; fragmentation across LSP numbers; regeneration on topology change
- `internal/component/isis/lsdb/aging.go` - 1s lifetime decrement, refresh timer, MaxAge handling, zero-age purge with grace period, sequence-number wraparound suspension
- `internal/component/isis/lsdb/lsdb_test.go` - store/retrieve, freshness, verbatim storage, SRM/SSN flag ops, snapshot
- `internal/component/isis/lsdb/origination_test.go` - originate on adjacency Up, regen on change, refresh increments seq, wraparound, fragmentation, overload bit
- `internal/component/isis/lsdb/aging_test.go` - decrement, age-to-purge, purge-vs-expiry

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan - check what exists |
| 3. Wiring phase | Wiring Test table - register origination trigger, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - LSDB skeleton + origination trigger reachable from adjacency Up
   - Tests: `TestISISOriginateOnAdjacencyUp` (fails: origination is a stub), `TestISISLSDBStoreRetrieve`
   - Files: `internal/component/isis/lsdb/lsdb.go` (struct + insert/lookup stubs), `internal/component/isis/lsdb/entry.go`, hook into `internal/component/isis/events.go`
   - Verify: an adjacency-Up event reaches the origination entry point; the wiring test fails only because origination returns a stub LSP
2. **Phase: Store + freshness + flags** - the database core
   - Tests: `TestISISLSDBStoreRetrieve`, `TestISISLSDBReceiveNewer`, `TestISISLSDBStoreVerbatim`, `TestISISSRMSSNFlagOps`, `TestISISLSDBSnapshot`
   - Files: `internal/component/isis/lsdb/lsdb.go`, `entry.go`
   - Verify: insert/lookup/delete, freshness compare, verbatim raw-byte storage, per-circuit SRM/SSN ops, snapshot all pass
3. **Phase: Origination** - build own LSPs from live state
   - Tests: `TestISISOriginateOnAdjacencyUp`, `TestISISOriginateRegenOnChange`, `TestISISOriginateOverloadBit`
   - Files: `internal/component/isis/lsdb/origination.go`
   - Verify: own LSP built with TLV 1/129/22/132/135/137, valid sequence + Fletcher checksum (via `spec-isis-2-wire`), overload bit set on demand; regen on change
4. **Phase: Aging + refresh + purge** - lifecycle timers
   - Tests: `TestISISLSDBAgeDecrement`, `TestISISLSDBAgeToPurge`, `TestISISLSDBPurgeVsExpiry`, `TestISISRefreshIncrementsSeq`
   - Files: `internal/component/isis/lsdb/aging.go`
   - Verify: 1s decrement; lifetime-0 purge with grace; purge-vs-expiry distinction; refresh increments sequence and resets lifetime
5. **Phase: Wraparound + fragmentation** - the hard edges
   - Tests: `TestISISSequenceWraparound`, `TestISISOriginateFragmentation`
   - Files: `internal/component/isis/lsdb/origination.go`, `aging.go`
   - Verify: 0xFFFFFFFF triggers purge + suspension window + re-origin from 1; oversized state splits across LSP numbers 0..255 with no TLV split mid-value
6. **Phase: Metrics** - Prometheus counters from the umbrella canonical table (LSDB size, fragments, purges, wraparounds, origination events); registration here, scrape assertion in isis-13
7. **Functional/interop** - none standalone; LSDB sync proven via `spec-isis-7-flooding`, SPF via `spec-isis-9-spf-rib`, snapshot via isis-13
8. **Full verification** - `make ze-verify`
9. **Complete spec** - fill audit tables, write learned summary to `plan/learned/NNN-isis-6-lsdb.md`, two commits per planning.md

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path; origination contents match RFC 1195/5305/5301/3787 (TLV 1/22/129/132/135/137 + overload) |
| Correctness | Sequence wraparound (ISO/IEC 10589 7.3.3) waits MaxAge + ZeroAgeLifetime; purge != expiry; checksum recomputed on every sequence change; no TLV split across fragments |
| Naming | Package `lsdb`; exported types/methods follow Ze conventions; counter metric names listed |
| Data flow | LSPs stored as raw bytes + metadata; TLV parse lazy; SRM/SSN set here and consumed by `spec-isis-7-flooding`; no bypass of the `spec-isis-2-wire` codec |
| Rule: buffer-first | LSP encode is `WriteTo(buf, off) int`; checksum backfilled at fixed offset; no `buildLSP() []byte`, no `append`-based encode |
| Rule: plugin-self-containment | All LSDB code under `internal/component/isis/lsdb/`; no IS-IS spelling in generic packages |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| LSDB store + entry | `ls internal/component/isis/lsdb/lsdb.go internal/component/isis/lsdb/entry.go` |
| Origination + aging | `ls internal/component/isis/lsdb/origination.go internal/component/isis/lsdb/aging.go` |
| Unit tests pass | `go test ./internal/component/isis/lsdb/...` |
| Wraparound + fragmentation covered | `go test -run 'Wraparound|Fragmentation' ./internal/component/isis/lsdb/` |
| Snapshot API consumed by isis-13 | `grep -r 'Snapshot' internal/component/isis/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Received LSP metadata parsed only after the `spec-isis-2-wire` codec validated lengths; no slicing past PDU bounds in the LSDB |
| Resource exhaustion | LSDB size cap per level; max LSP fragments (255); reject oversize / malformed before storing; bounded purge-retention list |
| Sequence/lifetime sanity | Reject implausible sequence/lifetime jumps; wraparound handled, not crashed |
| Spoofing | Authentication (TLV 10) is verified upstream in `spec-isis-10-auth` before an LSP reaches the LSDB; the LSDB trusts only validated PDUs |
| Memory safety | Stored raw bytes are a single owned copy (not aliasing a reused receive buffer) to avoid corruption when the receive buffer is reused |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior / RFC summary |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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
<!-- LIVE: write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
<!-- Delete this section if nothing qualifies. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Store LSPs as raw bytes + parsed metadata, parse TLVs lazily | Eager parse into structs (bio-rd) | Ze buffer-first model; cheap byte-for-byte re-flood of unknown TLVs (ISO/IEC 10589 7.3.14); parse only what SPF/CLI needs |
| SRM/SSN as per-LSP per-circuit bitmaps owned by the LSDB | Flags owned by the flooding layer | Keeps the flooding spec (isis-7) free of storage concerns; the data model lives with the data |
| Zero-age purge retained for a grace period, distinct from expiry | Delete at lifetime 0 (bio-rd) | ISO/IEC 10589 7.3.16/17; prevents a node that missed the purge from keeping a stale LSP |
| Full LSP-set regeneration on topology change | Incremental TLV edits | Simpler and correct; matches bio-rd; origination is not a hot path |

## Known Limitations
- Wide metrics only (RFC 5305): TLV 22 carries a 24-bit IS-reachability metric, TLV 135 a 32-bit prefix metric; narrow metrics are decoded for interop elsewhere but never originated
- IPv6 reachability (TLV 236, 32-bit prefix metric) and interface-address (TLV 232) origination is out of scope here; added in `spec-isis-12-ipv6`
- Pseudo-node LSP origination is out of scope here; added in `spec-isis-8-dis-broadcast` (it reuses this store and SRM/SSN model)
- Overload-bit honouring in path computation is noted here but implemented in `spec-isis-9-spf-rib`
- Single-topology only (no RFC 5120 MT)

## RFC Documentation

Add `// ISO/IEC 10589 Section X.Y: "<quoted requirement>"` (and 1195 / 5305 / 5301 / 3787 as applicable) above enforcing code.
MUST document: sequence wraparound (7.3.3, sequence 0 reserved), checksum recompute (7.3.11), origination triggers (7.3.12), re-flood verbatim (7.3.14), aging and zero-age purge signalled by remaining-lifetime 0 (7.3.16/7.3.17), TLV 22 24-bit IS metric and TLV 135 32-bit prefix metric origination (RFC 5305), TLV 132 IP interface address (RFC 1195), TLV 137 hostname (RFC 5301), overload bit (RFC 3787).

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
| Own LSP originated with valid seq + checksum | unit test | `TestISISOriginateOnAdjacencyUp` |
| Lifetime decrements to purge (not delete) | unit test | `TestISISLSDBAgeToPurge` |
| Refresh increments sequence | unit test | `TestISISRefreshIncrementsSeq` |
| Wraparound purges then waits | unit test | `TestISISSequenceWraparound` |
| Fragmentation across LSP numbers | unit test | `TestISISOriginateFragmentation` |
| LSDB feeds flooding + SPF | functional test (siblings) | `test/isis/isis-lsdb-sync.ci` (isis-7), `test/isis/isis-route-install.ci` (isis-9) |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete: every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/isis/lsdb/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`, no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass, defer with user approval)
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
- [ ] Functional tests for end-to-end behavior (via siblings isis-7/isis-9/isis-13)
- [ ] Interop tests for protocol features (via siblings isis-7/isis-9)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes: all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-isis-6-lsdb.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-isis-6-lsdb.md` only (preserves edited spec in git history from commit A)
