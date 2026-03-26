# Spec: apply-mods

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/10 |
| Updated | 2026-03-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9234.md` - OTC egress stamping rules (Section 5)
4. `internal/component/plugin/registry/registry.go` - ModAccumulator, filter types, Registration
5. `internal/component/bgp/reactor/reactor_api_forward.go` - egress filter chain, TODO at line 275
6. `internal/component/bgp/plugins/role/otc.go` - OTC helpers, egress filter
7. `plan/learned/419-route-metadata.md` - route metadata architecture decisions

## Task

Implement the `applyMods` framework in the reactor forward path and use it for RFC 9234 OTC egress stamping. Currently egress filters can only accept or suppress routes. The `ModAccumulator` infrastructure exists but nothing applies accumulated mods. This spec adds:

1. A mod handler registry where plugins register handlers for specific mod keys at startup
2. The `applyMods` call in the reactor forward path (replacing the TODO at `reactor_api_forward.go:275`)
3. OTC egress stamping via the mod system: when a route without OTC is sent to Customer/Peer/RS-Client, stamp OTC = local ASN (RFC 9234 Section 5)
4. Unicast-only scope enforcement for OTC filters (RFC 9234 Section 5: AFI 1/2, SAFI 1 only)

Parent context: `plan/learned/401-role-otc.md` (Phase 2 complete), `plan/learned/419-route-metadata.md` (metadata + ModAccumulator infrastructure).

### Why now

OTC egress stamping is a MUST requirement in RFC 9234 Section 5. Without it, routes sent to Customer/Peer/RS-Client lack OTC, allowing downstream leak detection to fail. The mod framework also unblocks `spec-llgr-4-readvertisement.md` which needs `mods["add:attr:community"]`, `mods["set:attr:local-preference"]`, and `mods["withdraw:nlri:*"]`.

### Scope

**In scope:**

| Area | Description |
|------|-------------|
| Attr mod handler registry | Register per-attribute-code handler in `registry.go`. A handler receives the source attribute bytes (nil if absent), all ops for that code, output buffer + offset, and writes the result. It knows the attribute's semantics (scalar, list, sequence). |
| `applyMods` progressive build | After egress filter chain and wire selection, if `mods.Len() > 0`: single-pass progressive build into a pooled buffer. Walk source attributes, collect ops per attr code, call registered handler. Unchanged attrs copied verbatim. After walk, call handlers for unconsumed codes (additions). Skip-and-backfill for attr_len. Zero-copy fast path preserved when `mods.Len() == 0`. |
| AttrOp structure | Each mod entry is flat: attr code (uint8), action (uint8: set/add/remove/prepend), buf (pre-built value bytes). Multiple entries with same code allowed -- handler receives all ops for its code at once. |
| OTC egress stamping | Role plugin writes `mods.Op(35, actionSet, otcValueBytes)` when route has no OTC and destination is Customer/Peer/RS-Client |
| Unicast-only scope | OTC ingress and egress filters skip non-unicast families (IPv4/IPv6 unicast only per RFC) |
| LocalASN capture | Role plugin captures reactor's LocalAS during OnConfigure for use in egress stamping |

**Out of scope:**

| Area | Reason |
|------|--------|
| LLGR mod handlers | Separate spec (`spec-llgr-4-readvertisement.md`) |
| `resolveExport` optimization | Performance, not correctness; separate concern |
| Private AS removal filter | Future filter, not RFC 9234 |
| AS Confederation OTC rules | No confederation support yet |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward path, egress filter chain, WireUpdate
  -> Constraint: forward path is per-peer in reactor_api_forward.go, wire selection happens after filter chain
  -> Constraint: WireUpdate created from payload via `wireu.NewWireUpdate(payload, ctxID)`
- [ ] `docs/architecture/meta/README.md` - route metadata key registry, meta conventions
  -> Constraint: mod keys follow `<action>:<target>:<name>` convention

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9234.md` - OTC egress stamping rules
  -> Constraint: "If a route is to be advertised to a Customer, a Peer, or an RS-Client [...] and the OTC Attribute is not present, then [...] an OTC Attribute MUST be added with a value equal to the AS number of the local AS." (Section 5)
  -> Constraint: "Once the OTC Attribute has been set, it MUST be preserved unchanged." (Section 5)
  -> Constraint: OTC procedures "MUST NOT be applied to other address families by default" (AFI 1/2, SAFI 1 only)

**Key insights:**
- `ModAccumulator` already exists with `Set/Get/Range/Len/Reset`, lazily allocated, per-peer fresh instance
- `EgressFilterFunc` already receives `*ModAccumulator`; OTC egress filter now writes mods (implemented)
- `applyMods` is implemented at `reactor_api_forward.go:289-312` (runs after wire selection)
- `ModHandlerFunc` is a post-accept transformation (current: callback per mod key). Progressive build will replace callbacks with pre-built attribute bytes for mechanical copy/replace.
- `insertOTCInPayload` exists in `otc.go:229-266` and works correctly
- `isPayloadUnicast` (not `extractFamilyFromPayload`) gates OTC on unicast families in both filters
- Role plugin captures localASN via `setFilterState` during OnConfigure (implemented)
- `spec-llgr-4` depends on this framework for community addition, local-pref modification, and withdrawal conversion
- Progressive build aligns with buffer-first architecture: pooled buffer + `WriteTo(buf, off)` pattern, skip-and-backfill for length fields (same as `reactor_wire.go`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - ModAccumulator (lines 54-94), EgressFilterFunc (line 52), Registration struct (lines 98-152), IngressFilters/EgressFilters (lines 506-534). No mod handler registry exists yet.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate method. Lines 256-274: egress filter chain with fresh `ModAccumulator` per peer. Line 275: `TODO(spec-llgr-4): applyMods(mods)`. Lines 277-288: wire selection for EBGP/IBGP. Line 291: fwdItem built with meta.
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter` (lines 278-312): ignores `mods` parameter (underscore). `checkOTCEgress` (lines 152-157): checks if route has OTC and dest is Provider/Peer/RS (suppression only). `insertOTCInPayload` (lines 180-217): creates new payload with OTC appended. `isUnicastFamily` (lines 92-96): defined but unused. `extractAttrsFromPayload` (lines 161-176): parses UPDATE payload for attributes.
- [ ] `internal/component/bgp/plugins/role/role.go` - Package-level filter state: `filterPeerConfigs`, `filterRemoteRoles`, `filterNameToIP` maps. `setFilterState` called during OnConfigure. No `localASN` captured currently.
- [ ] `internal/component/bgp/plugins/role/register.go` - Registers `IngressFilter: OTCIngressFilter`, `EgressFilter: OTCEgressFilter`. Registers OTC attribute name.
- [ ] `internal/component/bgp/reactor/reactor.go` - `Config.LocalAS` (line 106): reactor's local ASN, available at configure time.

**Behavior to preserve:**
- OTC ingress stamping and leak rejection (already working)
- OTC egress suppression: routes with OTC not sent to Provider/Peer/RS (already working)
- Export role filtering (already working)
- `meta["src-role"]` set at ingress, read at egress (already working)
- Lazy ModAccumulator allocation (zero cost when no mods written)
- Fail-closed panic recovery wrappers (safeIngressFilter, safeEgressFilter)
- Forward path zero-copy when no mods and ContextID matches

**Behavior to change:**
- `OTCEgressFilter` stops ignoring `mods` parameter; writes `"set:attr:otc"` mod when stamping needed
- New `applyMods` call in forward path applies accumulated mods, producing modified WireUpdate
- OTC ingress and egress filters check family; skip non-unicast
- Role plugin captures `localASN` during OnConfigure for egress stamping value

## Data Flow (MANDATORY)

### Entry Point -- Egress Stamping
- ForwardUpdate dispatches a route to matching destination peers
- Per-peer egress filter chain runs with fresh ModAccumulator

### Transformation Path

Per-peer forward loop in `reactor_api_forward.go`:

1. **Filter phase**: egress filter chain runs with fresh `ModAccumulator` per peer. Each filter can suppress (return false) or accept (return true) and optionally write mods. If any filter suppresses, skip this peer entirely. OTC egress filter checks: route has no OTC AND dest is Customer/Peer/RS-Client -> writes mod (attr code 35, action "add", pre-built OTC bytes).
2. **All filters accepted** -- route is going to this peer.
3. **Wire selection**: select EBGP/IBGP wire version based on peer type. EBGP peers get the pre-built AS-PATH-prepended wire, IBGP peers get original. This runs BEFORE mods because EBGP wires are pre-built once per ForwardUpdate call (loop-invariant); mods must apply to the peer-specific wire, not the original.
4. **Apply mods** (if `mods.Len() > 0`): progressive single-pass build into a pooled buffer. See Progressive Build Algorithm below. When `mods.Len() == 0` (common case): zero-copy, no buffer, no iteration.
5. **Build fwdItem**: forward item built with final wire + meta.

### Progressive Build Algorithm

When `mods.Len() > 0`, a single-pass progressive build rewrites the selected wire's payload into a pooled buffer. This avoids per-handler full-payload allocation and handles all mod types (add, replace, remove) uniformly in one pass.

**Buffer source:** pooled buffer (4096 or 64K depending on extended message negotiation). Returned to pool after send.

**Pass structure (skip-and-backfill):**

| Step | Action | Notes |
|------|--------|-------|
| 1 | Group ops by attr code | Scan `mods.Ops()`, build per-code op lists. Track which codes have ops. |
| 2 | Copy withdrawn section verbatim | 2-byte length + withdrawn bytes. Unchanged by egress mods. Can write immediately. |
| 3 | Skip attr_len field, save offset | Will be backfilled after all attributes are written. |
| 4 | Walk source attributes one by one | For each attribute: read flags + type code (1 byte at `attrs[off+1]`) + length. |
| 5 | Per attribute: check if code has ops | No ops: copy attribute bytes verbatim. Has ops: call registered handler with source bytes + ops list, handler writes result into buf. Mark code as consumed. |
| 6 | After all source attributes: unconsumed codes | For each code with ops not yet consumed (attribute absent in source): call handler with nil source + ops list. Handler creates attribute from scratch. |
| 7 | Backfill attr_len | Write actual attribute section length at saved offset from step 3. |
| 8 | Copy NLRI section verbatim | Trailing bytes after attribute section. For structural mods (LLGR withdraw conversion), this step handles NLRI transformation. |
| 9 | Wrap result | `wireu.NewWireUpdate(buf[:totalLen], sourceCtxID)`. BGP message header (19 bytes, total length at bytes 16-17) is written by the send layer, not applyMods. |

**AttrOp structure (flat, one entry per operation):**

| Field | Type | Purpose |
|-------|------|---------|
| Code | uint8 | Attribute type code (e.g., 35 for OTC, 8 for COMMUNITY, 5 for LOCAL_PREF) |
| Action | uint8 | Operation: set, add, remove, prepend |
| Buf | []byte | Pre-built wire bytes of the VALUE (not the full attribute -- handler writes the header) |

Multiple entries with the same Code are allowed. The progressive build groups ops by Code and passes all ops for a given code to the handler at once.

**Action semantics (attribute-type-dependent):**

| Action | Meaning | Example |
|--------|---------|---------|
| set | Replace entire attribute value | LOCAL_PREF: set 0. OTC: set ASN. MED: set 100. |
| add | Append value to attribute's list | COMMUNITY: add 65000:300 |
| remove | Remove value from attribute's list | COMMUNITY: remove NO_EXPORT |
| prepend | Prepend value to attribute's sequence | AS_PATH: prepend ASN |

**Handler per attribute code:**

Each registered handler knows its attribute's semantics. It receives: source attribute bytes (nil if absent in source), list of ops, output buffer + offset. It writes the complete attribute (header + value) into the output buffer. The progressive build engine is generic -- attribute knowledge lives in handlers.

| Source attr | Handler behavior |
|-------------|-----------------|
| Present | Read source header + value, apply ops (set replaces value, add appends, remove filters, prepend inserts), write result |
| Absent | Create attribute from scratch using ops, write header + value |

**Example: COMMUNITY with 3 ops on same UPDATE:**

| # | Code | Action | Buf |
|---|------|--------|-----|
| 0 | 8 | add | 4 bytes (LLGR_STALE) |
| 1 | 8 | remove | 4 bytes (NO_EXPORT) |
| 2 | 8 | add | 4 bytes (NO_EXPORT_SUBCONFED) |

Handler receives source community bytes + all 3 ops. Produces: source communities minus NO_EXPORT, plus LLGR_STALE and NO_EXPORT_SUBCONFED. One pass, one write.

**Attribute-level vs structural mods:**

| Mod type | Scope | Handled at |
|----------|-------|------------|
| Attribute ops (set/add/remove/prepend) | Single attribute by type code | Step 4 (attribute walk) or step 5 (unconsumed = new attr) |
| NLRI structural (e.g., LLGR withdraw conversion) | NLRI section rewrite | Step 7 (NLRI copy) -- future, separate ModAccumulator field |

**Performance characteristics:**
- Zero-copy fast path when `mods.Len() == 0` (no buffer, no iteration, no allocation)
- Single pass through source attributes when mods present (no second pass to patch)
- Attr code comparison is one uint8 compare per attribute per mod -- negligible
- Pooled buffer eliminates allocation cost
- Sequential writes into same cache line -- prefetcher-friendly
- Most attributes are unchanged (bulk copy), only modified attrs differ

### Entry Point -- Family Scope
- OTC ingress filter receives payload with path attributes
- OTC egress filter receives payload with path attributes
- Both need to determine if family is IPv4/IPv6 unicast before applying OTC rules

### Transformation Path -- Family Check
1. Parse UPDATE payload for MP_REACH_NLRI (type 14) or MP_UNREACH_NLRI (type 15) attributes
2. If neither present: family is IPv4 unicast (RFC 4271 implicit)
3. If MP_REACH present: read AFI (2 bytes) + SAFI (1 byte) from attribute value
4. Check AFI in {1, 2} and SAFI == 1; if not, skip OTC processing

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registry -> Reactor | Attr mod handlers registered at init by attr code, retrieved at reactor startup | [ ] |
| Egress filter -> ModAccumulator | Filter writes AttrOp entries (code + action + value bytes); reactor reads after chain | [ ] |
| ModAccumulator -> Progressive build | Reactor groups ops by code, calls registered handler per code during single-pass walk | [ ] |
| Handler -> output buffer | Handler writes complete attribute (header + value) into pooled buffer at given offset | [ ] |
| Progressive build -> WireUpdate | Build produces payload in pooled buffer; reactor wraps in WireUpdate | [ ] |

### Integration Points
- `registry.go` - `AttrOp` struct, `AttrModHandler` type, registration by attr code, `ModAccumulator` with `Op()` method
- `reactor_api_forward.go` - Replace current `applyMods` with progressive build using registered handlers
- `role/otc.go:OTCEgressFilter` - Write `mods.Op(35, actionSet, otcValueBytes)` instead of string-keyed mod
- `role/register.go` - Register OTC attr mod handler for code 35

### Architectural Verification
- [ ] No bypassed layers (mods flow through registry-based handlers, reactor never imports plugins)
- [ ] No unintended coupling (reactor calls handlers by attr code, doesn't know about OTC/role)
- [ ] Handler per attr code encapsulates attribute semantics (scalar set, list add/remove, sequence prepend)
- [ ] Zero-copy preserved when no mods (common case: `mods.Len() == 0` skips applyMods entirely)
- [ ] Pooled buffer eliminates per-peer allocation when mods are present

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `import provider` + `export default` + route without OTC forwarded to Customer peer | -> | OTC egress stamping via applyMods | `test/plugin/role-otc-egress-stamp.ci` |
| Config with `import provider` + multicast family route | -> | OTC not applied to non-unicast family | `test/plugin/role-otc-unicast-scope.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Phase |
|-------|-------------------|-------------------|-------|
| AC-1 | Route without OTC forwarded to Customer peer (local role = provider) | OTC attribute added with value = local ASN | v1 done |
| AC-2 | Route without OTC forwarded to Peer (local role = provider) | OTC attribute added with value = local ASN | v1 done |
| AC-3 | Route without OTC forwarded to RS-Client (local role = RS) | OTC attribute added with value = local ASN | v1 done |
| AC-4 | Route without OTC forwarded to Provider (local role = customer) | No OTC stamped | v1 done |
| AC-5 | Route with OTC already present forwarded to Customer peer | OTC preserved unchanged | v1 done |
| AC-6 | Non-unicast family route from Provider peer | OTC ingress rules not applied | v1 done |
| AC-7 | Non-unicast family route forwarded to Customer peer | OTC egress rules not applied | v1 done |
| AC-8 | No role configured on either peer | No OTC processing, no mods written | v1 done |
| AC-9 | `mods.Len() == 0` after filter chain | `applyMods` is a no-op, zero allocation, no buffer from pool | v1 done, v2 preserve |
| AC-10 | OTC attr overflow during mod handler | Route forwarded with original payload unchanged | v1 done, v2 preserve |
| AC-11 | `AttrOp` struct with code, action, buf fields | Egress filter writes `mods.Op(35, actionSet, otcValueBytes)` | v2 |
| AC-12 | `AttrModHandler` registered by attr code (uint8) | Handler callable from reactor without importing role plugin | v2 |
| AC-13 | Progressive build with single OTC set op | Output payload contains OTC attribute, attr_len correct, NLRI preserved | v2 |
| AC-14 | Progressive build with no matching source attr | Handler called with nil source, creates attribute from scratch | v2 |
| AC-15 | Progressive build with matching source attr (future: LOCAL_PREF set) | Handler called with source bytes, replaces value | v2 |
| AC-16 | Multiple ops on same attr code (future: COMMUNITY add+remove) | Handler receives all ops, produces single result in one write | v2 |
| AC-17 | Progressive build uses pooled buffer | Buffer obtained from pool, returned after send. No `make([]byte)` in hot path. | v2 |
| AC-18 | Unknown attr code in ops (no handler registered) | Ops skipped with warning log, source attr copied unchanged | v2 |

## TDD Test Plan

### v1 Unit Tests (existing -- update for v2 API)
| Test | File | Validates | v2 change |
|------|------|-----------|-----------|
| `TestModHandlerRegistration` | `registry/registry_test.go` | Register and retrieve handler | Rewrite: register by attr code, retrieve by code |
| `TestModHandlerNotFound` | `registry/registry_test.go` | Unknown code returns nil | Rewrite: lookup by uint8 code |
| `TestOTCEgressStampMod` | `role/otc_test.go` | Egress filter writes mod when no OTC + dest is Customer | Rewrite: assert `mods.Op(35, actionSet, ...)` |
| `TestOTCEgressNoStampProvider` | `role/otc_test.go` | No mod when dest is Provider | Assert `mods.Len() == 0` (API unchanged) |
| `TestOTCEgressPreserveExisting` | `role/otc_test.go` | No mod when OTC already present | Assert `mods.Len() == 0` (API unchanged) |
| `TestOTCEgressStampLocalASN` | `role/otc_test.go` | Mod value is local ASN bytes | Rewrite: check AttrOp.Buf contains local ASN |
| `TestOTCIngressUnicastOnly` | `role/otc_test.go` | Ingress skips non-unicast | No change (ingress unaffected) |
| `TestOTCEgressUnicastOnly` | `role/otc_test.go` | Egress skips non-unicast | No change |
| `TestIsPayloadUnicast` | `role/otc_test.go` | Family detection from payload | No change |
| `TestOTCModHandler` | `role/otc_test.go` | OTC handler produces correct bytes | Rewrite: new handler signature (src, ops, buf, off) |
| `TestOTCStampOverflow` | `role/otc_test.go` | Overflow returns original payload | Rewrite: handler writes 0 bytes on overflow |

### v2 Unit Tests (new)
| Test | File | Validates | AC |
|------|------|-----------|-----|
| `TestAttrOpStructure` | `registry/registry_test.go` | AttrOp holds code, action, buf; ModAccumulator.Op() stores entry | AC-11 |
| `TestAttrModHandlerRegistration` | `registry/registry_test.go` | Register handler by uint8 code, retrieve by code | AC-12 |
| `TestProgressiveBuildNoMods` | `reactor/forward_build_test.go` | `mods.Len() == 0` returns original payload, no buffer from pool | AC-9 |
| `TestProgressiveBuildOTCAdd` | `reactor/forward_build_test.go` | Single set op for code 35, source has no OTC: OTC appended, attr_len correct, NLRI intact | AC-13, AC-14 |
| `TestProgressiveBuildAttrReplace` | `reactor/forward_build_test.go` | Set op for code 5 (LOCAL_PREF), source has LOCAL_PREF: value replaced, rest unchanged | AC-15 |
| `TestProgressiveBuildMultiOps` | `reactor/forward_build_test.go` | Three ops on code 8 (COMMUNITY): add+remove+add, handler receives all, single result | AC-16 |
| `TestProgressiveBuildPooledBuffer` | `reactor/forward_build_test.go` | Buffer from pool, not make([]byte) | AC-17 |
| `TestProgressiveBuildUnknownCode` | `reactor/forward_build_test.go` | Op for unregistered code: warning logged, source attr copied unchanged | AC-18 |
| `TestProgressiveBuildWithdrawnPreserved` | `reactor/forward_build_test.go` | Withdrawn section copied verbatim, unaffected by attr mods | |
| `TestProgressiveBuildNLRIPreserved` | `reactor/forward_build_test.go` | NLRI section copied verbatim after modified attrs | |
| `TestProgressiveBuildAttrLenBackfill` | `reactor/forward_build_test.go` | attr_len field matches actual attribute section length after mods | |
| `TestOTCHandlerNewSignature` | `role/otc_test.go` | OTC handler: nil source + set op -> writes 7-byte OTC attr into buf | AC-14 |
| `TestOTCHandlerExistingPreserved` | `role/otc_test.go` | OTC handler: source has OTC + set op -> copies source unchanged | AC-5 |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Local ASN for OTC stamp | 1-4294967295 | 4294967295 | 0 (should not stamp with ASN 0) | N/A (uint32) |
| MP_REACH AFI | 1-2 for unicast scope | 2 (IPv6) | 0 (reserved) | 3+ (not unicast scope) |
| MP_REACH SAFI | 1 for unicast scope | 1 | 0 (reserved) | 2+ (not unicast) |
| AttrOp.Code | 0-255 | 255 | N/A (uint8) | N/A (uint8) |
| Attr_len after mods | 0-65535 | 65535 | N/A | Overflow: skip mod, use original payload |

### Functional Tests (existing -- behavioral assertions unchanged)
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `role-otc-egress-stamp` | `test/plugin/role-otc-egress-stamp.ci` | Provider sends route without OTC; Customer peer receives route WITH OTC = local ASN | exists |
| `role-otc-unicast-scope` | `test/plugin/role-otc-unicast-scope.ci` | Multicast route forwarded without OTC processing regardless of role config | exists |

### Future (if deferring any tests)
- Property-based testing for progressive build round-trip (build -> parse -> compare)
- Benchmark: progressive build vs v1 callback for 1, 2, 3 concurrent mods
- Benchmark: applyMods overhead per peer when `mods.Len() == 0` (must be zero allocation)

## Files to Modify
- `internal/component/plugin/registry/registry.go` - Replace `ModHandlerFunc`/string-keyed API with `AttrOp` struct, `AttrModHandler` type, registration by uint8 code, `ModAccumulator` with `Op()` method
- `internal/component/plugin/registry/registry_test.go` - Update tests for new API (attr code registration, AttrOp structure)
- `internal/component/bgp/reactor/reactor_api_forward.go` - Replace current callback-based `applyMods` with progressive build engine
- `internal/component/bgp/reactor/reactor.go` - Load attr mod handlers by code at startup (replaces string-keyed map)
- `internal/component/bgp/reactor/reactor_notify.go` - Update `safeModHandler` for new handler signature (or replace with per-attr-code panic recovery in build engine)
- `internal/component/bgp/plugins/role/otc.go` - OTC egress filter: `mods.Op(35, actionSet, otcValueBytes)`. OTC handler: new signature writing into caller's buffer.
- `internal/component/bgp/plugins/role/register.go` - Register OTC handler by attr code 35
- `internal/component/bgp/reactor/forward_update_test.go` - Update mod-related tests for new API

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional tests | No new ones | Existing `.ci` tests validate behavior, API is internal |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | OTC stamping is automatic per RFC, not user-configured |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Same role plugin, internal API change |
| 6 | Has a user guide page? | No | Behavior unchanged from user perspective |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | Already implemented in v1 |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - document progressive build in forward path, AttrOp structure, per-attr-code handlers |

## Files to Create
- `internal/component/bgp/reactor/forward_build.go` - Progressive build engine (separate file from forward path dispatch)
- `internal/component/bgp/reactor/forward_build_test.go` - Progressive build unit tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases 1-5 below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

v1 phases (1-5) are already implemented. v2 phases (6-10) are new work.

1. ~~**Phase: Mod handler registry**~~ -- v1 done
2. ~~**Phase: applyMods callback in forward path**~~ -- v1 done
3. ~~**Phase: Family extraction + unicast scope**~~ -- v1 done
4. ~~**Phase: OTC egress stamping via string-keyed mods**~~ -- v1 done
5. ~~**Phase: Functional tests**~~ -- v1 done (both .ci tests exist)

6. **Phase: AttrOp + ModAccumulator refactor** -- Replace string-keyed `map[string]any` with `AttrOp` struct and `Op()` method in `registry.go`. Replace `ModHandlerFunc` with `AttrModHandler` registered by uint8 code.
   - Tests: `TestAttrOpStructure`, `TestAttrModHandlerRegistration`
   - Files: `registry/registry.go`, `registry/registry_test.go`
   - Verify: tests fail -> implement -> tests pass. Compilation breaks expected (callers use old API).

7. **Phase: OTC migration to AttrOp** -- Migrate OTC egress filter to `mods.Op(35, actionSet, otcValueBytes)`. Migrate OTC handler to new signature. Update registration in `register.go`.
   - Tests: `TestOTCEgressStampMod` (rewritten), `TestOTCHandlerNewSignature`, `TestOTCHandlerExistingPreserved`
   - Files: `role/otc.go`, `role/register.go`, `role/otc_test.go`
   - Verify: OTC filter writes AttrOp, handler writes into buf. Compilation fixed.

8. **Phase: Progressive build engine** -- Implement single-pass build in `forward_build.go`. Group ops by code, walk source attrs, call handlers, backfill attr_len, copy NLRI.
   - Tests: `TestProgressiveBuildNoMods`, `TestProgressiveBuildOTCAdd`, `TestProgressiveBuildAttrReplace`, `TestProgressiveBuildMultiOps`, `TestProgressiveBuildUnknownCode`, `TestProgressiveBuildWithdrawnPreserved`, `TestProgressiveBuildNLRIPreserved`, `TestProgressiveBuildAttrLenBackfill`
   - Files: `reactor/forward_build.go`, `reactor/forward_build_test.go`
   - Verify: all progressive build tests pass in isolation

9. **Phase: Wire into forward path** -- Replace callback-based `applyMods` in `reactor_api_forward.go` with progressive build call. Add pooled buffer get/put. Update `reactor.go` handler loading.
   - Tests: `TestProgressiveBuildPooledBuffer`, existing `forward_update_test.go` tests updated
   - Files: `reactor/reactor_api_forward.go`, `reactor/reactor.go`, `reactor/reactor_notify.go`, `reactor/forward_update_test.go`
   - Verify: `make ze-verify` passes. Existing `.ci` tests pass unchanged (behavioral equivalence).

10. **Phase: Cleanup + verification**
    - Remove dead v1 code (`ModHandlerFunc`, `safeModHandler` if replaced, string-keyed registration)
    - `make ze-verify`
    - Audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | OTC stamped with local ASN per RFC 9234 Section 5 |
| RFC compliance | Existing OTC preserved unchanged; stamping only when OTC absent |
| Scope | OTC processing gated on unicast family (AFI 1/2, SAFI 1) |
| Backward compat | No role config = no OTC processing, no mods written |
| Performance | `mods.Len() == 0` = zero-copy, zero allocation. Mods present = pooled buffer, single pass. |
| Coupling | Reactor calls handlers by uint8 attr code, never imports role plugin |
| Data flow | Egress filter writes AttrOp -> progressive build groups by code -> handler writes into buf -> WireUpdate wraps result |
| No v1 residue | String-keyed `ModHandlerFunc`, `safeModHandler` callback path removed. No dead code. |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `AttrOp` struct in registry | grep "AttrOp" in registry.go |
| `AttrModHandler` type | grep "AttrModHandler" in registry.go |
| Registration by attr code | grep "RegisterAttrModHandler" in registry.go |
| `ModAccumulator.Op()` method | grep "func.*ModAccumulator.*Op(" in registry.go |
| Progressive build engine | ls reactor/forward_build.go |
| Progressive build wired into forward path | grep "progressiveBuild\|buildModifiedPayload" in reactor_api_forward.go |
| OTC filter uses AttrOp | grep "mods.Op(.*35" in otc.go |
| OTC handler registered by code | grep "RegisterAttrModHandler.*35\|otcAttrCode" in register.go |
| Pooled buffer in build | grep "buildBufPool\|Get()\|Put(" in forward_build.go |
| v1 dead code removed | grep "ModHandlerFunc\|RegisterModHandler\b" returns zero hits outside tests |
| Functional tests still pass | `make ze-functional-test` (no .ci changes needed) |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Handler validates AttrOp.Buf length before using; malformed op skipped |
| Overflow | attr_len > 65535 after mods: skip modification, use original payload |
| Panic safety | Handler called with panic recovery in progressive build loop |
| No injection | AttrOp.Code is uint8 from filter code, not user input. AttrOp.Buf is pre-built wire bytes. |
| Buffer bounds | Progressive build checks `off + writeLen <= len(buf)` before every write |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH |
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

### Option A vs Option B for applyMods (decided: Option B)

Two approaches were considered for applying mods to UPDATE payloads:

| Approach | Description | Strengths | Weaknesses |
|----------|-------------|-----------|------------|
| A: Full copy then in-place patch | memcpy entire payload, then overwrite at specific offsets | Fast memcpy (SIMD/vectorized), simple for same-size replacements | Breaks when mod changes payload size (OTC add = +7 bytes, community append = +variable). Requires memmove to shift trailing bytes. Initial copy needs slack space calculated upfront. |
| B: Progressive single-pass build | Walk source attributes, copy unchanged, replace modified, append new -- all in one pass into pooled buffer | Handles all mod types uniformly (add/replace/remove). One pass, no shifting. Aligns with buffer-first architecture (skip-and-backfill). | Many small copies instead of one memcpy. |

**Decision: Option B.** Rationale:
- Iteration is needed anyway to find target attributes; Option A does find-then-patch (two passes), Option B does find-and-write (one pass)
- OTC and LLGR mods change payload size, which is the majority of mod types. Option A's memcpy advantage only holds for same-size patches.
- Aligns with the project's buffer-first `WriteTo(buf, off)` pattern and pooled buffers
- Small sequential copies into the same cache line are prefetcher-friendly; the performance difference vs one memcpy is negligible for UPDATE-sized payloads (typically < 4096 bytes)

### Why mods run after wire selection

EBGP wires (with AS-PATH prepended) are pre-built once per ForwardUpdate call, before the per-peer loop. If applyMods ran before wire selection, mods would modify the original payload, but wire selection would pick the pre-built EBGP wire (generated from the unmodified original). Mods would be silently lost for all EBGP peers. Mods must run on the selected wire's payload.

### Flat AttrOp structure with per-attribute handlers

The mod system uses flat AttrOp entries (code uint8, action uint8, buf []byte) instead of string-keyed mods with callbacks on full payloads. This design emerged from three observations:

1. **Attribute operations are type-specific.** COMMUNITY supports add/remove (list), LOCAL_PREF supports set (scalar), AS_PATH supports prepend (sequence). A single generic handler cannot know these semantics. One handler per attribute code is the right granularity.
2. **Multiple ops on the same attribute.** A single peer may need 3 COMMUNITY ops (add X, remove Y, add Z). Flat entries with the same Code accumulate naturally. The handler receives all ops at once and produces one result in one write.
3. **Filters don't know source content.** An egress filter writes a general rule ("ensure OTC = local ASN") without knowing if the source payload has OTC or not. The progressive build discovers this during the walk and calls the handler with the source bytes (or nil if absent). The handler handles both cases.

## RFC Documentation

Add `// RFC 9234 Section 5: "<quoted requirement>"` above enforcing code.
MUST document: OTC egress stamp rule (add OTC = local ASN to Customer/Peer/RS-Client), OTC preservation (existing OTC unchanged), unicast-only scope.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-apply-mods.md`
- [ ] Summary included in commit
