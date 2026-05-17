# Spec: ASPA Path Verification

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-05-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rpki-0-umbrella.md` - RPKI umbrella spec (D5 defers ASPA here)
4. `internal/component/bgp/plugins/rpki/rpki.go` - existing RPKI plugin structure
5. `internal/component/bgp/plugins/rpki/validate.go` - existing ROA validation algorithm
6. `internal/component/bgp/plugins/rpki/rtr_pdu.go` - existing RTR PDU types

## Task

Add ASPA (Autonomous System Provider Authorization) path verification to Ze. ASPA allows an AS to declare its authorized upstream providers via RPKI. When a route is received, the AS_PATH is verified against ASPA records: each hop in the path is checked to confirm the customer-provider relationship is authorized. This is the "upstream verification" algorithm from draft-ietf-sidrops-aspa-verification-24 (Internet-Draft, no RFC number assigned as of March 2026).

ASPA complements ROA origin validation (RFC 6811) by verifying not just the origin but the entire AS_PATH. This spec extends the existing `bgp-rpki` plugin.

### Relationship to RPKI Umbrella

This spec fulfills the deferral "D5: ASPA deferred" from `spec-rpki-0-umbrella.md`, which states: "ASPA validation (draft-ietf-sidrops-aspa-verification) is a separate concern with its own RTR PDU type and validation algorithm."

### Scope

| In scope | Out of scope |
|----------|-------------|
| ASPA record storage (customer-AS to authorized-provider set) | ROA validation changes (already in rpki-0) |
| RTR v2 version negotiation + ASPA PDU (receive from cache server) | RTR v2 features beyond negotiation and ASPA PDU |
| Upstream path verification algorithm | Downstream path verification |
| ASPA validation states (Valid, Invalid, Unknown) | Policy actions based on ASPA state (separate concern) |
| Per-route ASPA validation integrated into RPKI plugin | ASPA-aware best-path selection in RIB |
| Config for enabling ASPA validation | ASPA record creation/signing (CA-side) |
| JSON event output with ASPA validation state | |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RPKI plugin integration pattern
  -> Decision: RPKI plugin is an "API mode" plugin (not RIB mode). Receives WireUpdate via DirectBridge, extracts attrs via lazy parse. Does not own RIB storage.
  -> Constraint: Plugin reads from WireUpdate (zero-copy, buf owned by WireUpdate). Must not hold references past event callback return.
- [ ] `docs/architecture/wire/attributes.md` - AS_PATH attribute format (needed for verification)
  -> Decision: AS_PATH is type code 2, flags 0x40 (well-known mandatory). Parsed via `attribute.ParseASPath(data, fourByte)` returning `*ASPath` with `Segments []ASPathSegment`.
  -> Constraint: Segment types 1-4 (ASSet, ASSequence, ASConfedSequence, ASConfedSet). ASN4 encoding when both peers support 4-byte capability. MaxASPathTotalLength = 1000.
- [ ] `docs/architecture/api/architecture.md` - plugin event format
  -> Decision: Engine sends events via DirectBridge (StructuredEvent) or JSON fallback. RPKI uses both paths. Event JSON envelope: `{"type":"bgp","bgp":{"peer":{...},"message":{...},"rpki":{...}}}`.
  -> Constraint: Plugin emits events via `p.EmitEvent(ctx, root, eventType, direction, peer, jsonStr)`. Event JSON must be kebab-case keys per json-format.md.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6811.md` - ROA validation (existing, for pattern reference)
  -> Constraint: Origin AS derived from rightmost AS in final AS_SEQUENCE; AS_SET yields NONE (can never match). Re-validation MUST fire on VRP cache change (Section 4).
- [ ] `rfc/short/rfc8210.md` - RTR v1 protocol (PDU types, session lifecycle)
  -> Constraint: Version negotiation Section 7: client sends highest version, server responds or sends error code 4. PDU types 0-10 in v1. End of Data is 24 bytes with timing params.
- [ ] `rfc/short/rfc9582.md` - RTR v2 protocol (ASPA PDU type 11) - CREATED
  -> Constraint: ASPA PDU type 11, v2 only. Fixed 16 bytes + 4*N providers. AFI flags byte (0=both, 1=IPv4, 2=IPv6). Announce replaces entire provider set (not delta). Providers MUST be sorted ascending. Min 1 provider per PDU.
- [ ] `rfc/short/draft-ietf-sidrops-aspa-verification.md` - Upstream path verification algorithm - CREATED
  -> Constraint: Normalize path (strip consecutive-dup prepends, remove confed segments). AS_SET -> Unknown. Walk adjacent pairs from neighbor toward origin: check_pair(provider_candidate, customer_as). "Not Provider+" on any hop -> Invalid. All "Provider+" -> Valid. Any "No Attestation" -> Unknown. Re-validation MUST on ASPA data change.

**Key insights:** (summary of all checkpoint lines -- minimal context to resume after compaction)
- RTR v2 negotiation required: ASPA PDU only sent in v2 sessions; v1 fallback means ROA-only
- ASPA PDU is replace-semantics (announce = full provider set replacement, not incremental)
- Verification is per-UPDATE not per-prefix (AS_PATH shared across all NLRIs in one UPDATE)
- Algorithm input: normalized unique-hop list + ASPA database; output: Valid/Invalid/Unknown
- Re-validation on cache change is MUST (both RFC 6811 Section 4 and ASPA draft Section 7). Handled inside RPKI plugin via route tracker (no cross-plugin dependency).
- AFI flags in ASPA PDU: cache key should be (customer-AS, AFI) or simplified to customer-AS if MAY (ignore AFI) is chosen
- `*attribute.ASPath` with full Segments available from AttrsWire; need segment types for AS_SET detection
- RPKI plugin owns its own route state for re-validation: tracker stores normalized paths + reverse index by customer-AS

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` (601L) - Plugin entry point. RPKIPlugin struct holds `*ROACache`, `validateCh chan`, sessions. OnStructuredEvent extracts `*ASPath` via `rpkiOriginASFromWire(msg.AttrsWire)`. Runs validation worker goroutine. OnAllPluginsReady enables validation gate. Registration: commands rpki {status,cache,roa,summary}, WantsConfig: ["bgp"].
  -> Constraint: ASPA verification must not block OnStructuredEvent callback. Must use same async pattern (channel + worker) or run inline if cheap enough.
  -> Constraint: `rpkiOriginASFromWire` already gets `*attribute.ASPath` via `attrs.Get(attribute.AttrASPath)`. ASPA can reuse same access path for full segments.
- [ ] `internal/component/bgp/plugins/rpki/validate.go` (149L) - RFC 6811 origin validation. `ROACache.Validate(prefix, originAS) uint8`. States: NotValidated=0, Valid=1, NotFound=2, Invalid=3. OriginNone=0xFFFFFFFF sentinel for AS_SET.
  -> Constraint: ASPA uses different state semantics (Valid/Invalid/Unknown vs Valid/Invalid/NotFound). Must not reuse same constants. Needs own state type.
- [ ] `internal/component/bgp/plugins/rpki/roa_cache.go` (252L) - Thread-safe ROA cache. `map[string][]vrpEntry` keyed by prefix. Add/Remove/ApplyDelta/Clear/FindCovering. maxVRPs=1_000_000. Own `sync.RWMutex`.
  -> Constraint: Pattern to follow: separate struct, own mutex, Add/Remove/ApplyDelta/Clear methods, size limit. ASPA cache is simpler (no covering-prefix search, just direct uint32 key lookup).
- [ ] `internal/component/bgp/plugins/rpki/rtr_pdu.go` (161L) - RTR v1 PDU types 0-10. `rtrVersion uint8 = 1` package const. VRP struct, RTRHeader struct. Parse functions: `parseHeader`, `parseIPv4Prefix`, `parseIPv6Prefix`, `parseEndOfData`. Write: `writeResetQuery`, `writeSerialQuery`.
  -> Constraint: `rtrVersion` is package const used by write functions. Must become per-session or parameter. pduASPA=11 needs adding. Parse function pattern: `parseASPAPDU(buf []byte) (ASPARecord, bool, error)` matching prefix parse signature.
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` (300L) - RTRSession struct with Run() goroutine. connectAndSync -> readLoop -> handlePDU switch. Accumulates pendingVRPs/pendingDels between CacheResp and EndOfData, applies atomically via `cache.ApplyDelta`. Retry with `retryInterval` on failure.
  -> Constraint: Session needs `aspaCache *ASPACache` field + `pendingASPA`/`pendingASPADels` slices. Version negotiation: if error code 4 received, downgrade `s.version` from 2 to 1 and retry in same Run() loop iteration.
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` (87L) - Parses `rpki` subtree from BGP config JSON. `rpkiConfig` struct: CacheServers, ValidationTimeout. Uses `configjson.ParseBGPSubtree`.
  -> Constraint: Add `ASPAEnabled bool` field (default true when ASPA cache has data). Config key: `"aspa-validation"` (kebab-case, boolean).
- [ ] `internal/component/bgp/plugins/rpki/emit.go` (110L) - Builds rpki event JSON via `json.Marshal` of typed structs. `rpkiEventJSON` envelope. `buildRPKIEvent` for per-prefix states, `buildRPKIEventUnavailable` for empty cache.
  -> Constraint: Add `"aspa-state"` field to rpki event. Must be sibling of existing rpki section. Value: "valid"/"invalid"/"unknown". Use same struct-based json.Marshal pattern (not string concatenation).

**Behavior to preserve:** (unless user explicitly said to change)
- Existing ROA validation algorithm and states
- RTR session lifecycle and reconnection logic
- RPKI plugin event flow (hold route, validate, accept/reject)
- Existing PDU parsing for ROA prefixes
- Config structure for cache server

**Behavior to change:** (only if user explicitly requested)
- Add ASPA record cache alongside ROA cache (separate `ASPACache` struct)
- RTR version negotiation: send v2 query, fall back to v1 if error code 4 (ASPA unavailable at v1)
- Add ASPA RTR PDU type 11 parsing (v2 sessions only)
- Add upstream path verification algorithm (normalize + walk hop pairs)
- Add ASPA validation state to rpki event JSON output (informational, no accept/reject change)
- Extend config to enable/disable ASPA verification

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- RTR cache server sends ASPA PDUs (type 11, v2 only) during sync between Cache Response and End of Data
- BGP UPDATE received with AS_PATH attribute triggers ASPA verification

### Transformation Path
1. RTR session receives ASPA PDU (16 bytes fixed + 4*N provider ASNs) -> parse flags, AFI, customer-AS, provider-AS-set
2. ASPA cache stores (customer-AS, AFI) -> set of authorized providers. Announce = full replacement of provider set.
3. At End of Data: apply accumulated ASPA changes atomically (same as ROA ApplyDelta pattern)
4. Route received -> extract `*attribute.ASPath` from AttrsWire (already parsed, lazy)
5. Normalize: remove consecutive-dup prepends, strip AS_CONFED_SEQUENCE, flag AS_SET -> Unknown
6. Upstream verification: walk normalized path from neighbor toward origin, check_pair(provider_candidate, customer_as) per hop
7. Result: Valid (all "Provider+"), Invalid (any "Not Provider+"), Unknown (any "No Attestation" and none "Not Provider+")
8. ASPA state included in rpki event JSON (AC-6). Does NOT affect accept/reject dispatch (policy actions are out of scope). ROA-only flow unchanged.
9. Route withdrawn -> tracker removes entry for (peer, family, prefix, pathID) from primary map and reverse index. No event emitted for withdrawals.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RTR cache -> ASPA cache | ASPA PDU type 11 parsing -> ASPACache storage (v2 sessions only) | [ ] |
| Engine -> RPKI plugin | StructuredEvent with RawMessage containing AttrsWire (AS_PATH) + PeerAS | [ ] |
| RPKI plugin -> event consumers | EmitEvent with `"aspa-state"` field in rpki event JSON (informational, no dispatch change) | [ ] |
| ASPA cache update -> route tracker | End of Data triggers re-validation of tracked routes whose path includes changed customer-AS | [ ] |

### Integration Points
- `rtr_pdu.go` - add `pduASPA uint8 = 11`, `pduASPAFixedLen = 16`, parse function: extract flags/AFI/customer-AS/provider-list from wire bytes
- `rtr_session.go` - handle ASPA PDU in `handlePDU` switch (accumulate in pendingASPA slices alongside pendingVRPs); requires RTR v2 negotiation (per-session version field replacing package const `rtrVersion`)
- `rpki.go` - call ASPA verification once per UPDATE (not per-prefix) after extracting `*attribute.ASPath`; pass result to emit (informational only, no dispatch change)
- New `aspa_verify.go` - upstream path verification algorithm per draft-ietf-sidrops-aspa-verification Section 6
- New `aspa_cache.go` - ASPA record storage: `map[uint32]map[uint32]struct{}` (customer-AS -> provider-AS set); separate struct with own mutex

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RTR v2 session receives ASPA PDU type 11 | -> | ASPACache.Lookup returns provider set | `TestParseASPAPDU` + `TestASPACacheAdd` |
| UPDATE with AS_PATH + ASPA cache populated | -> | ASPA verification runs, event emitted with aspa-state | `rpki-aspa-valid.ci` / `rpki-aspa-invalid.ci` |
| ASPA cache updated at End of Data | -> | Tracker re-validates affected routes, emits updated events | `TestASPATrackerRevalidate` |
| Config with `aspa-validation: false` | -> | ASPA verification skipped, no aspa-state in event | `rpki-aspa-disabled.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RTR ASPA PDU received | ASPA record stored in cache (customer-AS -> provider set) |
| AC-2 | Route with AS_PATH where all hops have authorized providers | ASPA state = Valid |
| AC-3 | Route with AS_PATH containing unauthorized provider hop | ASPA state = Invalid |
| AC-4 | Route with AS_PATH where some hops have no ASPA records | ASPA state = Unknown |
| AC-5 | ASPA cache update (new/withdrawn records) | RPKI plugin re-validates tracked routes internally, emits updated events for routes whose ASPA state changed |
| AC-6 | JSON event output includes ASPA validation state | `"aspa-state"` field in event JSON |
| AC-7 | ASPA disabled in config | No ASPA verification performed, ROA-only |
| AC-8 | Malformed ASPA PDU from RTR | Error logged, PDU skipped, session continues |
| AC-9 | AS_PATH with AS_SET segments | ASPA verification result = Unknown (cannot verify sets) |
| AC-10 | RTR cache only supports v1 (no ASPA PDU) | Session operates at v1, ASPA cache empty, no aspa-state in events (ROA-only) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestASPACacheAdd` | `rpki/aspa_cache_test.go` | Add ASPA record to cache | |
| `TestASPACacheRemove` | `rpki/aspa_cache_test.go` | Withdraw ASPA record | |
| `TestASPACacheLookup` | `rpki/aspa_cache_test.go` | Lookup providers for customer AS | |
| `TestASPACacheReplace` | `rpki/aspa_cache_test.go` | Announce replaces entire provider set (not delta) | |
| `TestRTRVersionNegotiationV2` | `rpki/rtr_session_test.go` | Session negotiates v2 when server supports it | |
| `TestRTRVersionFallbackV1` | `rpki/rtr_session_test.go` | Session falls back to v1 on error code 4 | |
| `TestRTRVersionMismatchError` | `rpki/rtr_session_test.go` | Error code 8 after negotiation drops session | |
| `TestParseASPAPDU` | `rpki/rtr_pdu_test.go` | Parse ASPA PDU (type 11) from wire bytes: flags, AFI, customer-AS, provider list | |
| `TestParseASPAPDUMalformed` | `rpki/rtr_pdu_test.go` | Malformed ASPA PDU: too short, zero providers, length mismatch | |
| `TestParseASPAPDUUnknownAFI` | `rpki/rtr_pdu_test.go` | Unknown AFI value (>=3) ignored per RFC 9582 | |
| `TestParseASPAPDUSelfRef` | `rpki/rtr_pdu_test.go` | Customer AS in own provider set -> malformed, skip | |
| `TestParseASPAPDUUnsorted` | `rpki/rtr_pdu_test.go` | Provider ASNs not ascending -> malformed, skip | |
| `TestASPAVerifyValid` | `rpki/aspa_verify_test.go` | All hops authorized -> Valid | |
| `TestASPAVerifyInvalid` | `rpki/aspa_verify_test.go` | Unauthorized hop -> Invalid | |
| `TestASPAVerifyUnknown` | `rpki/aspa_verify_test.go` | Missing ASPA records -> Unknown | |
| `TestASPAVerifyASSet` | `rpki/aspa_verify_test.go` | AS_SET in path -> Unknown | |
| `TestASPAVerifySingleHop` | `rpki/aspa_verify_test.go` | Single-hop AS_PATH (trivially Valid) | |
| `TestASPAVerifyEmptyPath` | `rpki/aspa_verify_test.go` | Empty AS_PATH -> Valid | |
| `TestASPANormalizePrepends` | `rpki/aspa_verify_test.go` | [A,A,B,B,B] -> [A,B]; [A,B,A] unchanged | |
| `TestASPANormalizeConfed` | `rpki/aspa_verify_test.go` | AS_CONFED_SEQUENCE stripped from path | |
| `TestASPATrackerAdd` | `rpki/aspa_tracker_test.go` | Track route with normalized path, reverse index populated | |
| `TestASPATrackerRemove` | `rpki/aspa_tracker_test.go` | Withdraw removes route from tracker and reverse index | |
| `TestASPATrackerRevalidate` | `rpki/aspa_tracker_test.go` | Cache change triggers re-verification of affected routes only | |
| `TestASPATrackerReverseIndex` | `rpki/aspa_tracker_test.go` | Lookup by customer-AS returns correct route refs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Customer AS | 1 - 2^32-2 | 0xFFFFFFFE | 0 (reserved, RFC 9582) | 0xFFFFFFFF (reserved) |
| Provider AS | 1 - 2^32-2 | 0xFFFFFFFE | 0 (reserved, RFC 9582) | 0xFFFFFFFF (reserved) |
| Provider count per ASPA | 1+ | 1 (minimum, RFC 9582 Section 5.12) | 0 (malformed: PDU length=16 with no providers) | implementation limit |
| ASPA PDU length | 20 (min: 16+4) | 65536 (max PDU) | 16 (no providers = malformed) | >65536 (rejected by readLoop) |
| AFI flags | 0-2 | 2 (IPv6-only) | N/A | 3+ (unknown, MUST ignore per RFC 9582) |
| AS_PATH length for verification | 0 - 1000 | 1000 (MaxASPathTotalLength) | N/A | 1001 (already rejected by ParseASPath) |
| Normalized path length | 0 (empty = Valid) | 1000 | N/A | N/A (bounded by AS_PATH max) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-aspa-valid` | `test/plugin/*.ci` | Route with fully authorized path -> event contains `"aspa-state":"valid"` | |
| `rpki-aspa-invalid` | `test/plugin/*.ci` | Route with unauthorized hop -> event contains `"aspa-state":"invalid"` | |
| `rpki-aspa-unknown` | `test/plugin/*.ci` | Route with no ASPA coverage -> event contains `"aspa-state":"unknown"` | |
| `rpki-aspa-disabled` | `test/plugin/*.ci` | ASPA disabled in config -> no `aspa-state` field in event | |

### Future (if deferring any tests)
- Downstream path verification (separate algorithm, separate spec)
- ASPA-aware best-path selection (RIB plugin concern)
- Policy actions based on ASPA state (accept/reject/local-pref adjustment)

## Files to Modify
- `internal/component/bgp/plugins/rpki/rpki.go` - integrate ASPA verification into validation flow
- `internal/component/bgp/plugins/rpki/rtr_pdu.go` - add ASPA PDU type and parser
- `internal/component/bgp/plugins/rpki/rtr_session.go` - handle ASPA PDU in receive loop
- `internal/component/bgp/plugins/rpki/rpki_config.go` - ASPA enable/disable config
- `internal/component/bgp/plugins/rpki/emit.go` - ASPA state in event JSON
- `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` - ASPA config schema

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `rpki/schema/ze-rpki.yang` (ASPA config) |
| RPC count in architecture docs | [ ] | N/A (no new RPCs, extends existing) |
| CLI commands/flags | [ ] | possibly `rpki show aspa-cache` |
| CLI usage/help text | [ ] | if CLI command added |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/plugin/*.ci` |

## Files to Create
- `internal/component/bgp/plugins/rpki/aspa_cache.go` - ASPA record storage (customer-AS -> provider set)
- `internal/component/bgp/plugins/rpki/aspa_cache_test.go` - cache unit tests
- `internal/component/bgp/plugins/rpki/aspa_verify.go` - upstream path verification algorithm
- `internal/component/bgp/plugins/rpki/aspa_verify_test.go` - verification unit tests
- `internal/component/bgp/plugins/rpki/aspa_tracker.go` - route tracker: stores normalized AS_PATH + ASPA state per active route, reverse index (customer-AS -> routes), handles re-validation on cache change
- `internal/component/bgp/plugins/rpki/aspa_tracker_test.go` - tracker unit tests
- `test/plugin/rpki-aspa-*.ci` - functional tests

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: RFC summary** -- Create RFC summaries for ASPA verification and RTR v2
   - Verify: `rfc/short/rfc9582.md` and `rfc/short/draft-ietf-sidrops-aspa-verification.md` exist -- DONE
2. **Phase: ASPA cache** -- Customer-AS to provider-set storage (`map[uint32]map[uint32]struct{}`)
   - Tests: `TestASPACacheAdd`, `TestASPACacheRemove`, `TestASPACacheLookup`, `TestASPACacheReplace` (announce = full replace)
   - Files: `aspa_cache.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: RTR v2 negotiation** -- Per-session version field, v2 query with v1 fallback
   - Tests: `TestRTRVersionNegotiationV2`, `TestRTRVersionFallbackV1`, `TestRTRVersionMismatchError`
   - Files: `rtr_pdu.go` (version const -> session field), `rtr_session.go` (negotiation logic in connectAndSync)
   - Verify: tests fail -> implement -> tests pass; existing RTR v1 tests still pass
4. **Phase: RTR ASPA PDU** -- Parse ASPA PDU type 11 from RTR cache server
   - Tests: `TestParseASPAPDU`, `TestParseASPAPDUMalformed`, `TestParseASPAPDUUnknownAFI`
   - Files: `rtr_pdu.go` (parser), `rtr_session.go` (handlePDU case + pending ASPA accumulation)
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Upstream verification** -- Normalize AS_PATH, walk hop pairs, check provider authorization
   - Tests: `TestASPAVerifyValid`, `TestASPAVerifyInvalid`, `TestASPAVerifyUnknown`, `TestASPAVerifyASSet`, `TestASPAVerifySingleHop`, `TestASPAVerifyEmptyPath`, `TestASPANormalizePrepends`, `TestASPANormalizeConfed`
   - Files: `aspa_verify.go`
   - Verify: tests fail -> implement -> tests pass
6. **Phase: Route tracker** -- Track active routes with normalized AS_PATH for re-validation (AC-5)
   - Tests: `TestASPATrackerAdd`, `TestASPATrackerRemove`, `TestASPATrackerRevalidate`, `TestASPATrackerReverseIndex`
   - Files: `aspa_tracker.go` (stores per-route: peer+family+prefix+pathID -> normalized path + ASPA state; reverse index: customer-AS -> route refs; re-validation method; must handle withdrawals to remove stale entries from both primary map and reverse index)
   - Verify: tests fail -> implement -> tests pass
7. **Phase: Plugin integration** -- Wire ASPA verification into RPKI plugin event flow (once per UPDATE, state in event JSON, track for re-validation)
   - Tests: integration-level tests (verify aspa-state appears in emitted event, verify re-validation emits on cache change, verify no dispatch change)
   - Files: `rpki.go` (call verify after extracting ASPath, track route, pass result to emit), `emit.go` (add `"aspa-state"` field to rpki event JSON)
   - Verify: tests fail -> implement -> tests pass; existing ROA accept/reject behavior unchanged
8. **Phase: Config and YANG** -- ASPA enable/disable config
   - Tests: config parsing tests
   - Files: `rpki_config.go`, `schema/ze-rpki.yang`
   - Verify: tests fail -> implement -> tests pass
9. **Functional tests** -- Create .ci tests for ASPA validation scenarios
10. **RFC refs** -- Add `// RFC 9582 Section X.Y` and `// draft-ietf-sidrops-aspa-verification Section X` comments
11. **Full verification** -- `make ze-verify`
12. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Upstream verification algorithm matches RFC exactly (hop-by-hop check) |
| Naming | JSON key is `"aspa-state"` (kebab-case), cache types follow rpki naming |
| Data flow | ASPA verification runs per-UPDATE alongside ROA per-prefix; ASPA state reported in event JSON, does not alter accept/reject |
| Rule: no-layering | ASPA extends existing RPKI plugin, does not create parallel validation |
| Rule: single-responsibility | aspa_cache.go and aspa_verify.go are separate concerns from ROA |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `aspa_cache.go` exists | `ls internal/component/bgp/plugins/rpki/aspa_cache.go` |
| `aspa_verify.go` exists | `ls internal/component/bgp/plugins/rpki/aspa_verify.go` |
| ASPA PDU type in rtr_pdu.go | `grep -n ASPA internal/component/bgp/plugins/rpki/rtr_pdu.go` |
| ASPA verification called from rpki.go | `grep -n aspa internal/component/bgp/plugins/rpki/rpki.go` |
| Unit tests pass | `go test -race ./internal/component/bgp/plugins/rpki/... -run ASPA -v` |
| Functional tests exist | `ls test/plugin/rpki-aspa-*.ci` |
| RFC summaries exist | `ls rfc/short/rfc9582.md rfc/short/draft-ietf-sidrops-aspa-verification.md` |
| YANG schema updated | `grep -n aspa internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | ASPA PDU length validation, provider-AS count bounds |
| Resource exhaustion (cache) | ASPA cache size limits (large number of ASPA records from cache server) |
| Resource exhaustion (tracker) | Route tracker size limit (malicious peer flooding routes to exhaust tracker memory). maxTrackedRoutes enforced. |
| Path verification DoS | Verification time bounded by AS_PATH length (already capped at 1000) |
| Cache poisoning | RTR session auth ensures trusted cache server only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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
<!-- LIVE -- write IMMEDIATELY when you learn something -->

- RTR version must become per-session (not package const): `rtrVersion` at rtr_pdu.go:13 is currently `uint8 = 1`. Needs to be a field on `RTRSession` with negotiation logic.
- ASPA PDU announce = full provider set replacement (not incremental delta like ROA). Cache storage is simpler: store/overwrite entire set on announce, delete on withdraw.
- Verification runs once per UPDATE, not per-prefix. AS_PATH is shared across all NLRIs in one message. ROA is per-prefix, ASPA is per-message.
- ASPA state is INFORMATIONAL ONLY in this spec. Does not drive accept/reject. Policy actions are out of scope (spec scope table). The event JSON carries `"aspa-state"` for observability. Future policy spec will use it for decisions.
- AFI flags field in ASPA PDU: RFC 9582 allows router to MAY ignore AFI and apply to all families. Simplest initial implementation: treat AFI=0 (both) as default, store per-customer-AS without AFI dimension. Can add AFI-awareness later if needed.
- AC-5 re-validation stays INSIDE the RPKI plugin. Plugin maintains its own route tracker: per-route (peer, family, prefix, pathID) -> normalized AS_PATH + current ASPA state. On ASPA cache change at End of Data, plugin walks affected routes (those whose path includes changed customer-AS), re-verifies, emits updated events. No adj-rib-in involvement.
- Reverse index for efficient re-validation: RPKI plugin keeps map[uint32][]routeRef (customer-AS -> routes that traverse it). Built incrementally as UPDATEs arrive. Withdrawn routes removed from index. Tracker size bounded by maxTrackedRoutes (same order as maxVRPs: 1M). Drop oldest on overflow with warning log.
- Pre-normalized AS_PATH stored per tracked route as owned `[]uint32` (copied from parsed ASPath). MUST NOT hold references into WireUpdate buffer past callback return. Tracker owns its data.
- Existing `rpkiOriginASFromWire` calls `attrs.Get(attribute.AttrASPath)` and gets `*attribute.ASPath`. Same call provides full Segments for ASPA verification. No additional parsing needed.
- Path normalization (prepend removal, confed stripping) must be consecutive-duplicate only. [A, B, A] is NOT reduced. This is a correctness-critical detail.
- `NewRTRSession` signature needs change: must accept `*ASPACache` alongside `*ROACache`. Current: `NewRTRSession(address, port, pref, cache, stopCh)`.
- RFC 9582: "Customer AS MUST NOT appear in its own provider set" - need validation on PDU parse.
- RFC 9582: "Provider ASNs MUST be sorted ascending" - malformed if unsorted. Log and skip PDU.
- ASPA withdraw semantics differ from ROA: withdraw needs only (customer-AS, AFI) match, not full provider set match. Simpler than ROA removal.

## RFC Documentation

Add `// RFC 9582 Section X.Y: "<quoted requirement>"` for PDU format and cache semantics.
Add `// draft-ietf-sidrops-aspa-verification Section X: "<quoted requirement>"` for verification algorithm.
MUST document: validation states, upstream verification steps, ASPA PDU format, AS_SET handling, normalization rules, version negotiation.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
