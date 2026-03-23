# Spec: bgp-filter-community

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/registry/registry.go` - filter types, Registration struct
4. `internal/component/bgp/reactor/reactor_notify.go` - ingress/egress filter pipeline
5. `internal/component/bgp/attribute/community.go` - community types
6. `internal/component/bgp/plugins/role/register.go` - reference filter plugin registration

## Task

Add a community filter plugin that allows operators to:

1. Define named communities (standard, large, extended) in a `community` block under `bgp`
2. Tag routes with named communities on ingress and/or egress (cumulative across global, group, peer levels)
3. Strip named communities from routes on ingress and/or egress (cumulative across levels)
4. Reference communities by name only in filter rules (never by raw value)

Infrastructure changes required:

| Change | Why |
|--------|-----|
| EgressFilterFunc signature: return `(pass bool, changed bool, payload []byte)` | Egress filters currently cannot modify payload; stripping requires mutation |
| FilterPriority field in Registration | Filter execution order is currently non-deterministic (map iteration); multiple filter plugins need deterministic ordering |
| PeerFilterInfo: add Name and GroupName fields | Filter plugins need peer identity for config lookup; avoids each plugin building its own name-to-IP map |
| `ze:cumulative` YANG extension for leaf-lists | Tag/strip lists must accumulate across bgp/group/peer levels; deepMergeMaps currently replaces leaf-lists |
| `filter` YANG container with `ingress`/`egress` sub-containers | Common structure for all filter plugins (community, prefix, IRR). Verify during Phase 3: if goyang merges duplicate containers from multiple augments, each plugin defines its own. If not, add empty containers to ze-bgp-conf.yang (same pattern as `capability`) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor filter pipeline, plugin lifecycle
  → Constraint: filters are read-only closures, captured at reactor startup
  → Constraint: panic recovery wraps all filter calls (fail-open)
- [ ] `docs/architecture/wire/attributes.md` - attribute wire format, community encoding
  → Constraint: buffer-first encoding, WriteTo pattern
- [ ] `docs/architecture/config/syntax.md` - config syntax and YANG patterns
  → Constraint: YANG-driven parsing, augment pattern for plugins

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc1997.md` - standard communities (16:16)
  → Constraint: 4 bytes per community, attribute code 8, well-known values 0xFFFFFF01-04
- [ ] `rfc/short/rfc8092.md` - large communities (32:32:32) -- create if missing
  → Constraint: 12 bytes per community, attribute code 32, dedup per RFC 8092 Section 5

**Key insights:**
- Ingress filters already support payload mutation (return modified []byte)
- Egress filters return bool only -- signature change needed
- Filter ordering is non-deterministic (map iteration over plugins registry)
- Community wire types and parsing already exist in attribute/community.go
- Config parsing for community values exists in config/routeattr_community.go
- Cumulative config only exists for routes (explicit append in peers.go); all other leaf-lists are replaced by deep-merge

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - Registration struct, IngressFilterFunc, EgressFilterFunc, IngressFilters(), EgressFilters() collection
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - safeIngressFilter (panic recovery, payload mutation), safeEgressFilter (panic recovery, bool only), ingress pipeline at lines 270-297
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - egress pipeline at lines 255-272, per-destination evaluation
- [ ] `internal/component/bgp/reactor/reactor.go` - filter slice storage (lines 251-252), collection at startup (lines 661-662)
- [ ] `internal/component/bgp/attribute/community.go` - Community (uint32), LargeCommunity (3x uint32), ExtendedCommunity ([8]byte), parsing, WriteTo, well-known constants
- [ ] `internal/component/bgp/config/routeattr_community.go` - ParseCommunity, ParseLargeCommunity, ParseExtendedCommunity from config text
- [ ] `internal/component/bgp/plugins/role/register.go` - reference pattern for filter plugin registration
- [ ] `internal/component/bgp/plugins/role/otc.go` - OTCIngressFilter (payload mutation via insertOTCInPayload), OTCEgressFilter (bool suppression)
- [ ] `internal/component/bgp/plugins/role/config.go` - OnConfigure callback pattern for per-peer config
- [ ] `internal/component/bgp/config/resolve.go` - deepMergeMaps (replaces leaf-lists, merges maps), ResolveBGPTree
- [ ] `internal/component/bgp/config/peers.go` - patchRoutes pattern (cumulative via explicit append)

**Behavior to preserve:**
- Existing IngressFilterFunc signature and all current ingress filter callers
- OTC ingress/egress filter functionality (bgp-role plugin)
- safeIngressFilter panic recovery and fail-open behavior
- Community wire format parsing and encoding (attribute/community.go)
- Config parsing for community values in route attributes (routeattr_community.go)
- Deep-merge replacement semantics for all existing config fields
- Route cumulative pattern in peers.go

**Behavior to change:**
- EgressFilterFunc signature: from `func(src, dst PeerFilterInfo, payload []byte) bool` to `func(src, dst PeerFilterInfo, payload []byte) (pass bool, changed bool, modifiedPayload []byte)`
- safeEgressFilter: update to handle new return values, copy-on-first-edit semantics
- Egress filter pipeline in reactor_api_forward.go: apply modified payload when changed=true; fresh payload per destination peer (never leak modifications across peers)
- PeerFilterInfo: add Name (string) and GroupName (string) fields; reactor populates from PeerSettings
- Registration struct: add FilterPriority field (int, lower = first)
- IngressFilters() and EgressFilters(): sort by FilterPriority, then by plugin name for equal priority
- OTC egress filter: update signature to match new EgressFilterFunc (returns pass, false, nil -- no mutation)
- deepMergeMaps in resolve.go: support `ze:cumulative` extension on leaf-lists (append instead of replace)

**Pre-implementation:** Create `rfc/short/rfc8092.md` (large communities) if missing.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point

Two entry points, both at config time:

**Community definitions:** Config file `community` block under `bgp` with named definitions. YANG parser extracts into tree nodes (map[string]any). Plugin receives via OnConfigure callback, parses into name-to-values map (name to type + wire bytes).

**Filter config:** Tag/strip lists at global, group, and peer levels. Accumulated (not replaced) during config resolution, same pattern as routes in peers.go. Plugin receives via OnConfigure, parses into per-peer tag/strip lists keyed by peer IP.

### Transformation Path

**Ingress (strip first, then tag):**
1. UPDATE received by reactor (wire bytes)
2. Reactor calls ingress filter chain (reactor_notify.go:270-297)
3. Community filter receives payload + source PeerFilterInfo (with Name, GroupName)
4. **Strip phase:** look up source peer's ingress strip list, remove matching community values from payload (copy-on-first-edit)
5. **Tag phase:** look up source peer's ingress tag list, add configured communities to payload (extend existing attribute or append new one)
6. Return (accept=true, modifiedPayload); passed to next filter, then cached and dispatched

**Egress (strip first, then tag):**
1. ForwardUpdate per destination peer (reactor_api_forward.go:255-272)
2. Reactor retrieves fresh payload from WireUpdate.Payload() for EACH destination peer (never reuse a modified payload across peers)
3. Reactor calls egress filter chain; within a single peer's chain, modified payload is passed to the next filter (same as ingress chaining)
4. Community filter receives payload + source + dest PeerFilterInfo (with Name, GroupName)
5. **Strip phase:** look up destination peer's egress strip list, remove matching community values from payload (copy-on-first-edit)
6. **Tag phase:** look up destination peer's egress tag list, add configured communities to payload
7. Return (pass=true, changed=true, modifiedPayload); used for this destination peer's wire encoding only

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config → Plugin | JSON via OnConfigure callback (SDK) | [ ] |
| Plugin state → Filter closure | Package-level map read by closure (same as bgp-role) | [ ] |
| Filter → Reactor | Return values (bool + optional modified payload) | [ ] |
| Wire bytes → Modified wire bytes | In-place attribute manipulation in payload copy | [ ] |

### Integration Points

- `registry.Registration` - FilterPriority field, updated EgressFilterFunc type
- `registry.IngressFilters()` / `EgressFilters()` - sorted by priority
- `reactor_notify.go` safeEgressFilter - updated for new signature
- `reactor_api_forward.go` egress pipeline - apply modified payload
- `config/resolve.go` - cumulative accumulation for filter tag/strip (like routes)
- `config/peers.go` - extract filter config from tree, accumulate across levels
- `attribute/community.go` - existing types used for wire manipulation
- `bgp-role` plugin - OTCEgressFilter signature update (no behavior change)

### Architectural Verification

- [ ] No bypassed layers (filters run in reactor pipeline, config via standard delivery)
- [ ] No unintended coupling (plugin reads own config, filter closures capture plugin state)
- [ ] No duplicated functionality (uses existing community types and parsing)
- [ ] Zero-copy preserved where applicable (filter creates new payload only when modifying; unmodified routes pass through unchanged)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with community definitions + tag list | → | Community filter ingress tagging | `test/plugin/community-tag.ci` |
| Config with community definitions + strip list | → | Community filter egress stripping | `test/plugin/community-strip.ci` |
| Config with cumulative tag (global + group + peer) | → | Accumulation in config resolution | `test/plugin/community-cumulative.ci` |
| Config with filter priority (role + community) | → | Priority ordering in filter chain | `test/plugin/community-priority.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `community { standard NAME { VALUE; } }` | Named community parsed and stored; config accepted |
| AC-2 | Config with `community { large NAME { VALUE; } }` | Named large community parsed and stored; config accepted |
| AC-3 | Config with `community { extended NAME { VALUE; } }` | Named extended community parsed and stored; config accepted |
| AC-4 | Config referencing undefined community name in tag/strip | Config rejected during verify (OnConfigure) with error naming the undefined community |
| AC-5 | Community name with multiple values defined | All values stored under the name; all added/stripped together |
| AC-6 | Peer with ingress tag list, receives UPDATE | UPDATE payload modified: configured communities added to path attributes |
| AC-7 | Peer with egress strip list, forwarding UPDATE | UPDATE payload modified: configured communities removed from path attributes |
| AC-8 | Peer in group with global tag, group tag, and peer tag | All three levels of communities added (cumulative) |
| AC-9 | Peer with ingress strip list, receives UPDATE with tagged communities | Configured communities removed from received UPDATE |
| AC-10 | Peer with egress tag list, forwarding UPDATE | Communities added to forwarded UPDATE |
| AC-11 | EgressFilterFunc with new signature, existing OTC filter | OTC filter works unchanged (returns pass, changed=false, nil) |
| AC-12 | Two filter plugins with different FilterPriority | Filters execute in priority order (lower number first) |
| AC-13 | UPDATE with no matching communities for strip | Payload unchanged, route passes through |
| AC-14 | Ingress tag adds standard community to UPDATE that has no COMMUNITY attribute | New COMMUNITY attribute created in payload |
| AC-15 | Ingress tag adds standard community to UPDATE that already has COMMUNITY attribute | Values appended to existing COMMUNITY attribute |
| AC-16 | Strip removes all values from a COMMUNITY attribute | Entire attribute removed from payload (not left empty) |
| AC-17 | Tag adds community that already exists in UPDATE | Community NOT deduplicated (added again); use strip-first to prevent duplicates |
| AC-18 | Peer with both ingress strip and ingress tag | Strip executes before tag (free space first, less data to iterate) |
| AC-19 | Egress filter modifies payload for peer A, then evaluates peer B | Peer B receives fresh original payload, not peer A's modified version |
| AC-20 | PeerFilterInfo passed to filter includes Name and GroupName | Filter receives peer identity without building its own lookup map |
| AC-21 | `ze:cumulative` leaf-list at bgp + group + peer levels | Values from all three levels accumulated (not replaced) in resolved config |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseCommunityDefinitions` | `plugins/filter-community/config_test.go` | AC-1, AC-2, AC-3: named community parsing |  |
| `TestParseCommunityMultipleValues` | `plugins/filter-community/config_test.go` | AC-5: multiple values per name |  |
| `TestRejectUndefinedCommunityRef` | `plugins/filter-community/config_test.go` | AC-4: undefined name rejected |  |
| `TestIngressTagStandard` | `plugins/filter-community/filter_test.go` | AC-6, AC-14, AC-15: standard community insertion |  |
| `TestIngressTagLarge` | `plugins/filter-community/filter_test.go` | AC-6: large community insertion |  |
| `TestIngressTagExtended` | `plugins/filter-community/filter_test.go` | AC-6: extended community insertion |  |
| `TestEgressStripStandard` | `plugins/filter-community/filter_test.go` | AC-7, AC-16: standard community removal |  |
| `TestEgressStripLarge` | `plugins/filter-community/filter_test.go` | AC-7: large community removal |  |
| `TestEgressStripExtended` | `plugins/filter-community/filter_test.go` | AC-7: extended community removal |  |
| `TestStripNoMatch` | `plugins/filter-community/filter_test.go` | AC-13: no-op when no match |  |
| `TestCumulativeTagConfig` | `plugins/filter-community/config_test.go` | AC-8: global + group + peer accumulation |  |
| `TestIngressStrip` | `plugins/filter-community/filter_test.go` | AC-9: strip on ingress direction |  |
| `TestEgressTag` | `plugins/filter-community/filter_test.go` | AC-10: tag on egress direction |  |
| `TestEgressFilterSignature` | `internal/component/plugin/registry/registry_test.go` | AC-11: new EgressFilterFunc signature |  |
| `TestFilterPriorityOrdering` | `internal/component/plugin/registry/registry_test.go` | AC-12: priority-sorted filter collection |  |
| `TestOTCEgressFilterUpdated` | `internal/component/bgp/plugins/role/otc_test.go` | AC-11: OTC filter with new signature |  |
| `TestTagExistingCommunityNoDeDup` | `plugins/filter-community/filter_test.go` | AC-17: no dedup on tag |  |
| `TestStripBeforeTag` | `plugins/filter-community/filter_test.go` | AC-18: strip runs before tag within filter |  |
| `TestEgressPayloadIsolation` | `internal/component/bgp/reactor/reactor_forward_test.go` | AC-19: fresh payload per destination peer |  |
| `TestPeerFilterInfoFields` | `internal/component/plugin/registry/registry_test.go` | AC-20: Name and GroupName populated |  |
| `TestCumulativeLeafList` | `internal/component/bgp/config/resolve_test.go` | AC-21: ze:cumulative accumulation |  |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Standard community value | 0 - 0xFFFFFFFF | 0xFFFFFFFF | N/A (uint32) | N/A (uint32) |
| Large community each field | 0 - 0xFFFFFFFF | 0xFFFFFFFF | N/A (uint32) | N/A (uint32) |
| FilterPriority | 0 - MaxInt | 0 (highest priority) | N/A | N/A |
| Community name length | 1+ chars | 1 char | 0 chars (empty) | N/A |
| Values per community | 1+ | 1 | 0 (empty block) | N/A |
| Payload size after tag | up to 4096 (standard) or 65535 (extended msg) | Max message size | N/A | Overflow handled |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `community-tag` | `test/plugin/community-tag.ci` | Config with named community + ingress tag; verify community appears in JSON output | |
| `community-strip` | `test/plugin/community-strip.ci` | Config with named community + egress strip; verify community absent in forwarded output | |
| `community-cumulative` | `test/plugin/community-cumulative.ci` | Config with global + group + peer tags; verify all communities present | |
| `community-priority` | `test/plugin/community-priority.ci` | Config with role + community filters; verify execution order | |
| `community-parse-error` | `test/parse/community-filter.ci` | Config referencing undefined community name; verify parse error | |

### Future (if deferring any tests)

- None planned -- all filter types (standard, large, extended) tested in this spec

## Files to Modify

- `internal/component/plugin/registry/registry.go` - EgressFilterFunc signature, FilterPriority field, PeerFilterInfo extension, sorted filter collection
- `internal/component/bgp/reactor/reactor_notify.go` - safeEgressFilter updated for new return values
- `internal/component/bgp/reactor/reactor_api_forward.go` - egress pipeline: copy-on-first-edit, apply modified payload, fresh payload per destination peer, populate PeerFilterInfo Name/GroupName
- `internal/component/bgp/reactor/reactor_notify.go` - ingress pipeline: populate PeerFilterInfo Name/GroupName
- `internal/component/bgp/plugins/role/otc.go` - OTCEgressFilter updated to new signature (returns pass, false, nil -- no behavior change)
- `internal/component/bgp/plugins/role/otc_test.go` - tests updated for new signature
- `internal/component/bgp/config/resolve.go` - `ze:cumulative` support in deepMergeMaps (append instead of replace for marked leaf-lists)
- `internal/component/config/yang/` - register `ze:cumulative` extension

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config blocks) | [x] | `internal/component/bgp/plugins/filter-community/schema/ze-filter-community.yang` |
| CLI commands/flags | [ ] | N/A (config-driven, no new CLI commands) |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/community-tag.ci`, `community-strip.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add community filtering |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - community block and filter syntax; `docs/architecture/config/syntax.md` - community definitions |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` - new bgp-filter-community plugin |
| 6 | Has a user guide page? | [x] | `docs/guide/community-filtering.md` - new page for community filter usage |
| 7 | Wire format changed? | [ ] | N/A (community wire format unchanged) |
| 8 | Plugin SDK/protocol changed? | [x] | `.claude/rules/plugin-design.md` - FilterPriority field; `docs/architecture/api/process-protocol.md` - updated EgressFilterFunc |
| 9 | RFC behavior implemented? | [ ] | N/A (no automatic RFC enforcement) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - community filtering capability |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - egress filter mutation, filter priority |

## Files to Create

- `internal/component/bgp/plugins/filter-community/filter_community.go` - plugin entry point, filter state, OnConfigure callback
- `internal/component/bgp/plugins/filter-community/config.go` - config parsing for community definitions and filter rules
- `internal/component/bgp/plugins/filter-community/filter.go` - ingress/egress filter functions (payload mutation)
- `internal/component/bgp/plugins/filter-community/register.go` - plugin registration with filters and FilterPriority
- `internal/component/bgp/plugins/filter-community/schema/ze-filter-community.yang` - YANG schema
- `internal/component/bgp/plugins/filter-community/schema/register.go` - YANG embed and registration
- `internal/component/bgp/plugins/filter-community/schema/embed.go` - embedded YANG string
- `internal/component/bgp/plugins/filter-community/config_test.go` - config parsing tests
- `internal/component/bgp/plugins/filter-community/filter_test.go` - filter logic tests
- `test/plugin/community-tag.ci` - functional test: ingress tagging
- `test/plugin/community-strip.ci` - functional test: egress stripping
- `test/plugin/community-cumulative.ci` - functional test: cumulative tagging
- `test/plugin/community-priority.ci` - functional test: filter priority ordering
- `test/parse/community-filter.ci` - functional test: config validation

## Design

### Community Definition Config Structure

The `community` block lives under `bgp` and defines named communities by type.

| YANG Path | Type | Key | Children | Description |
|-----------|------|-----|----------|-------------|
| `/bgp/community` | container | - | `standard`, `large`, `extended` | Community definitions |
| `/bgp/community/standard` | list | `name` | `value` (leaf-list of string) | Standard communities (ASN:value format) |
| `/bgp/community/large` | list | `name` | `value` (leaf-list of string) | Large communities (GA:LD1:LD2 format) |
| `/bgp/community/extended` | list | `name` | `value` (leaf-list of string) | Extended communities (all supported formats) |

### Filter Config Structure

The `filter` block augments `bgp`, `group`, and `peer` levels with `ingress`/`egress` sub-containers. Each filter plugin augments these with its specific config.

| YANG Path (relative to peer/group/bgp) | Type | Description |
|-----------------------------------------|------|-------------|
| `filter` | container | Filter rules |
| `filter/ingress` | container | Ingress direction filters |
| `filter/ingress/community` | container | Community filter config for ingress |
| `filter/ingress/community/tag` | leaf-list of string | Named communities to add |
| `filter/ingress/community/strip` | leaf-list of string | Named communities to remove |
| `filter/egress` | container | Egress direction filters |
| `filter/egress/community` | container | Community filter config for egress |
| `filter/egress/community/tag` | leaf-list of string | Named communities to add |
| `filter/egress/community/strip` | leaf-list of string | Named communities to remove |

### Cumulative Accumulation via `ze:cumulative`

New YANG extension `ze:cumulative` marks specific leaf-lists as additive during config resolution. When `deepMergeMaps` encounters a `ze:cumulative` leaf-list, it appends values from the more-specific level instead of replacing.

| Leaf-list type | Merge behavior |
|----------------|---------------|
| Normal (default) | Most-specific level replaces (current behavior, unchanged) |
| `ze:cumulative` | Values from all levels accumulated |

| Config Level | Effect on `ze:cumulative` fields |
|-------------|--------|
| `bgp` level | Base tag/strip applied to all peers |
| `group` level | Appended to bgp-level tag/strip for all peers in group |
| `peer` level | Appended to bgp + group tag/strip for this peer |

Result for a peer: union of all applicable levels. Order within a level does not matter (communities are set-like). No dedup performed during accumulation (strip-first ordering handles duplicates at filter time).

Fields marked `ze:cumulative` in this spec: `filter/ingress/community/tag`, `filter/ingress/community/strip`, `filter/egress/community/tag`, `filter/egress/community/strip`.

### PeerFilterInfo Extension

| Field | Type | New? | Purpose |
|-------|------|------|---------|
| `Address` | `netip.Addr` | existing | Peer IP address |
| `PeerAS` | `uint32` | existing | Remote AS number |
| `Name` | `string` | **new** | Peer name from config |
| `GroupName` | `string` | **new** | Group name (empty if standalone peer) |

Avoids each filter plugin building its own name-to-IP mapping. The reactor already has this information in PeerSettings.

### EgressFilterFunc Signature Change

| Field | Old | New |
|-------|-----|-----|
| Return type | `bool` | `(pass bool, changed bool, modifiedPayload []byte)` |
| Semantics | true = forward, false = suppress | pass: forward/suppress; changed: payload was modified; modifiedPayload: replacement bytes (used only when changed=true) |

All existing egress filter callers (bgp-role OTC) updated to return `(pass, false, nil)` -- no behavior change.

### Egress Payload Isolation

The reactor MUST retrieve a fresh payload from `WireUpdate.Payload()` for each destination peer. Within a single peer's filter chain, modified payload is passed to subsequent filters (chaining). Across destination peers, each starts from the original. Copy-on-first-edit: the filter copies the payload only when it actually needs to modify it.

### FilterPriority

| Field | Type | Default | Semantics |
|-------|------|---------|-----------|
| `FilterPriority` | `int` | 0 | Lower number executes first; equal priority: sorted by plugin name |

| Plugin | Priority | Rationale |
|--------|----------|-----------|
| bgp-role (OTC) | 0 | Protocol-level, must run first |
| bgp-filter-community | 10 | Community rules before prefix rules |
| bgp-filter-prefix (future) | 20 | After community, before IRR |
| bgp-filter-irr (future) | 30 | After all other filters |

### Wire Manipulation

Tagging and stripping operate on raw UPDATE payload bytes. The approach follows the same pattern as OTC insertion (insertOTCInPayload in otc.go). Within the community filter, **strip runs first, then tag** (free space first, iterate on less data).

**Stripping (runs first):**

1. Scan path attributes in payload to find target attribute (code 8, 32, or 16)
2. If found: scan values within attribute, build new attribute without matching values
3. If all values removed: build new payload without the attribute entirely
4. If not found or no match: return nil (no modification)
5. Update attribute length fields and overall path attribute length
6. Return modified payload

**Tagging (runs second):**

1. Scan path attributes in payload to find target attribute (code 8, 32, or 16)
2. If found: create new payload with attribute length extended and new values appended (no dedup; use strip to remove duplicates if needed)
3. If not found: create new payload with new attribute appended after existing attributes. Attribute flags: Optional Transitive (0xC0) for COMMUNITY, LARGE_COMMUNITY, and EXTENDED_COMMUNITY
4. Update attribute length fields and overall path attribute length
5. Return modified payload

Both operations use copy-on-first-edit: allocate a new byte slice only when a modification is actually needed. The original payload is never mutated (shared across destination peers in egress).

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
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

1. **Phase: Infrastructure -- EgressFilterFunc signature + PeerFilterInfo** -- Change EgressFilterFunc return type, add Name/GroupName to PeerFilterInfo, update safeEgressFilter, update egress pipeline (copy-on-first-edit, fresh payload per peer), update OTC filter, populate PeerFilterInfo in reactor
   - Tests: `TestEgressFilterSignature`, `TestOTCEgressFilterUpdated`, `TestEgressPayloadIsolation`, `TestPeerFilterInfoFields`
   - Files: `registry/registry.go`, `reactor/reactor_notify.go`, `reactor/reactor_api_forward.go`, `plugins/role/otc.go`, `plugins/role/otc_test.go`
   - Verify: existing tests pass with new signature (no behavior change)

2. **Phase: Infrastructure -- FilterPriority** -- Add FilterPriority to Registration, sort filters by priority in IngressFilters()/EgressFilters()
   - Tests: `TestFilterPriorityOrdering`
   - Files: `registry/registry.go`, `registry/registry_test.go`
   - Verify: bgp-role keeps priority 0, filter ordering is deterministic

3. **Phase: Infrastructure -- ze:cumulative** -- Register YANG extension, modify deepMergeMaps to append marked leaf-lists
   - Tests: `TestCumulativeLeafList`
   - Files: `config/yang/` (extension registration), `config/resolve.go` (merge behavior)
   - Verify: cumulative leaf-lists accumulate across levels, normal leaf-lists still replaced

4. **Phase: YANG + Config -- Community definitions** -- YANG schema for community block, config parsing for named communities
   - Tests: `TestParseCommunityDefinitions`, `TestParseCommunityMultipleValues`
   - Files: new plugin schema files, config parsing
   - Verify: named communities parsed from config

5. **Phase: YANG + Config -- Filter block and accumulation** -- YANG for filter/ingress/egress/community with ze:cumulative on tag/strip, verification that goyang handles filter container (see B3 in Design section). Validation at OnConfigure (reject undefined community names)
   - Tests: `TestRejectUndefinedCommunityRef`, `TestCumulativeTagConfig`
   - Files: YANG schema, plugin config.go
   - Verify: tag/strip lists accumulated across levels, undefined names rejected at verify time

6. **Phase: Filter -- Stripping** -- Wire manipulation to remove community values from payload (runs first in filter)
   - Tests: `TestEgressStripStandard`, `TestEgressStripLarge`, `TestEgressStripExtended`, `TestStripNoMatch`
   - Files: filter.go, filter_test.go
   - Verify: communities correctly removed, no-op when no match, empty attribute removed entirely

7. **Phase: Filter -- Tagging** -- Wire manipulation to insert/extend community attributes in payload (runs second in filter). No dedup (AC-17)
   - Tests: `TestIngressTagStandard`, `TestIngressTagLarge`, `TestIngressTagExtended`, `TestTagExistingCommunityNoDeDup`
   - Files: filter.go, filter_test.go
   - Verify: communities correctly added, correct attribute flags (0xC0), no dedup

8. **Phase: Bidirectional + ordering** -- All four combinations (ingress tag, ingress strip, egress tag, egress strip), strip-before-tag ordering within filter
   - Tests: `TestIngressStrip`, `TestEgressTag`, `TestStripBeforeTag`
   - Files: filter.go, filter_test.go
   - Verify: all four combinations work, strip always runs before tag

9. **Phase: Plugin registration and wiring** -- register.go, make generate, functional tests
   - Tests: all functional tests (`community-tag.ci`, `community-strip.ci`, `community-cumulative.ci`, `community-priority.ci`, `community-filter.ci`)
   - Files: register.go, all.go (generated), functional test files
   - Verify: feature reachable from config end-to-end

10. **Full verification** -- `make ze-verify`

11. **Complete spec** -- Fill audit tables, write learned summary to `plan/learned/NNN-filter-community.md`, delete spec from `plan/`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Wire manipulation preserves valid BGP UPDATE structure (lengths, flags 0xC0, attribute ordering) |
| Naming | JSON keys kebab-case, YANG uses kebab-case, community names validated |
| Data flow | Config → OnConfigure → filter closure → reactor pipeline; no shortcutting |
| Rule: no-layering | Old EgressFilterFunc signature fully replaced, no compatibility shim |
| Rule: buffer-first | Wire manipulation uses offset writes, not append; new payload from pool if possible |
| Rule: integration-completeness | Every filter path reachable from config via .ci test |
| Egress isolation | Fresh payload per destination peer; modified payload NOT leaked across peers |
| Strip-before-tag | Strip phase runs before tag phase in every code path |
| No dedup on tag | Tag appends without checking for existing values |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| EgressFilterFunc returns (bool, bool, []byte) | grep for "EgressFilterFunc" in registry.go |
| FilterPriority field in Registration | grep for "FilterPriority" in registry.go |
| Plugin registered | grep for "filter-community" in registry after make generate |
| YANG schema exists | ls plugins/filter-community/schema/ |
| Ingress tagging works | test/plugin/community-tag.ci passes |
| Egress stripping works | test/plugin/community-strip.ci passes |
| Cumulative accumulation works | test/plugin/community-cumulative.ci passes |
| Config validation works | test/parse/community-filter.ci passes |
| OTC filter unchanged behavior | existing role tests pass |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Community names validated (non-empty, no special chars); values validated via existing Parse* functions |
| Payload overflow | Tagging must not exceed max UPDATE size (4096 or 65535 with extended messages); reject or truncate |
| Malformed payload | Filter must handle malformed input gracefully (short reads, invalid attribute lengths); fail-open |
| Memory allocation | Wire manipulation creates bounded copies; no unbounded allocations from attacker-controlled input |
| Config injection | Community names from config are strings; no command injection vector |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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

- EgressFilterFunc signature change is infrastructure that benefits all future filter plugins
- Filter priority mechanism enables predictable pipeline ordering without naming tricks
- Cumulative accumulation for tag/strip follows the proven routes pattern in peers.go
- Wire manipulation for community insert/remove parallels OTC insertion in otc.go
- Named communities with validation at config time prevents typo-based misconfigurations

## RFC Documentation

No RFC enforcement in this plugin (operator controls all community behavior via filters). RFC references for wire format only:
- RFC 1997: standard community attribute (code 8, 4 bytes per value)
- RFC 8092: large community attribute (code 32, 12 bytes per value, dedup required)
- RFC 4360: extended community attribute (code 16, 8 bytes per value)

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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
- [ ] AC-1..AC-21 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-filter-community.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
