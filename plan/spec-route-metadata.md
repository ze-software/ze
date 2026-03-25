# Spec: route-metadata

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/registry/registry.go` - EgressFilterFunc, IngressFilterFunc, PeerFilterInfo
4. `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate, egress filter call site
5. `internal/component/bgp/reactor/reactor_notify.go` - safeEgressFilter, ingress filter chain
6. `internal/component/bgp/reactor/received_update.go` - ReceivedUpdate struct

## Task

Add a generic route metadata channel (`map[string]any`) that travels with routes from ingress through the forward path to egress filters. This allows policy decisions (LLGR stale suppression, OTC filtering, route weight) without parsing wire bytes in every filter.

Additionally, add a modifications accumulator (`map[string]any`) that egress filters can write to. Modifications are applied once after all filters pass, before sending. This enables per-peer attribute modification (e.g., LLGR partial deployment: add NO_EXPORT + set LOCAL_PREF=0 for IBGP non-LLGR peers).

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Route metadata type | `map[string]any` on ReceivedUpdate, set at ingress, readable by egress filters |
| Modifications accumulator | `map[string]any` per egress filter call, writable by filters, applied by forward path |
| EgressFilterFunc signature | Extend to receive metadata and mods parameters |
| IngressFilterFunc signature | Extend to return metadata alongside modified payload |
| safeEgressFilter / safeIngressFilter | Update wrappers for new signatures |
| ForwardUpdate | Pass metadata to filters, apply mods after filter chain |
| ReceivedUpdate | Add metadata field |
| UpdateRouteInput RPC | Add `Meta` field for plugin-originated metadata |
| SDK UpdateRoute | Add variant with metadata parameter |
| CommandContext | Add Meta field, plumbed from RPC to reactor |
| handleUpdateRouteRPC | Pass RPC meta to CommandContext |
| Registry filter collection | Updated types |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Role plugin migration to metadata | Separate spec; role currently works via wire parsing |
| LLGR egress filter | Separate spec (spec-llgr-4); will consume this infrastructure |
| Multi-protocol weight | Future; this spec provides the mechanism |
| Modification application logic | Only the framework (mods map + apply hook); specific mod types (add-community, set-local-pref) added by consuming specs |

### Scope Boundary: Modification Application

This spec creates the framework: filters write to `mods`, and a hook applies them after filtering. The initial set of supported modification keys is minimal:

Mod keys follow the convention `<action>:<target>:<name>` where names match the text command format (kebab-case).

| Action | Meaning |
|--------|---------|
| `set` | Replace value |
| `del` | Remove |
| `add` | Append (lists) |
| `withdraw` | Convert announce to withdrawal |

| Target | Meaning |
|--------|---------|
| `attr` | Path attribute |
| `nlri` | Route/prefix (`*` for all) |

| Mod Key | Value Type | Effect | Needed By |
|---------|-----------|--------|-----------|
| `"add:attr:community"` | `[]string` | Append communities to UPDATE | LLGR-4 (NO_EXPORT) |
| `"set:attr:local-preference"` | `uint32` | Override LOCAL_PREF in UPDATE | LLGR-4 (LOCAL_PREF=0) |
| `"del:attr:local-preference"` | `bool` | Remove LOCAL_PREF from UPDATE | example |
| `"del:nlri:10.0.0.0/24"` | `bool` | Remove prefix from NLRI list (still an announce) | per-prefix filtering |
| `"del:nlri:*"` | `bool` | Remove all NLRIs from announce (empty UPDATE) | per-prefix filtering |
| `"withdraw:nlri:*"` | `bool` | Convert all NLRIs to withdrawals | LLGR-4 (non-LLGR peer) |
| `"withdraw:nlri:10.0.0.0/24"` | `bool` | Convert one prefix to withdrawal, keep rest as announce | per-prefix filtering |

Additional mod keys are added by consuming specs as needed. The apply hook dispatches on key name.

### Scope Boundary: Metadata Injection via RPC

Metadata can be set at two points:
1. **Ingress filters** (for received UPDATEs) -- filters write to meta map during ingress chain
2. **UpdateRoute RPC** (for plugin-originated UPDATEs) -- plugin includes `Meta` in RPC input

This spec adds a `Meta` field to `UpdateRouteInput` and plumbs it through to `ReceivedUpdate.Meta`. This is how the RIB passes stale level when resending routes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward path, UPDATE building, ReceivedUpdate lifecycle
  -> Constraint: ReceivedUpdate is immutable after creation; metadata must be set at creation time
  -> Constraint: egress filters called per-destination-peer; mods must be per-call, not shared

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR partial deployment: NO_EXPORT + LOCAL_PREF=0 for IBGP non-LLGR peers
  -> Constraint: modifications to UPDATE must happen before wire encoding to peer

**Key insights:**
- ReceivedUpdate is created once at ingress and cached; metadata set once, read many times
- Egress filters are called per-destination-peer; mods accumulator must be fresh per peer
- IngressFilterFunc already returns modified payload; extending to also return metadata is natural
- The forward path already has per-peer branching (IBGP vs EBGP wire versions); applying mods fits here
- Mods require re-encoding the UPDATE; this is already done for EBGP peers (AS-PATH prepend)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go:31-47` - PeerFilterInfo (Address, PeerAS), IngressFilterFunc, EgressFilterFunc
- [ ] `internal/component/bgp/reactor/received_update.go:34-67` - ReceivedUpdate struct (WireUpdate, SourcePeerIP, ReceivedAt; no metadata)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:234-272` - ForwardUpdate egress filter loop: builds PeerFilterInfo, calls safeEgressFilter, suppresses on false
- [ ] `internal/component/bgp/reactor/reactor_notify.go:22-45` - safeIngressFilter (returns accept+modified), safeEgressFilter (returns accept only)
- [ ] `internal/component/bgp/reactor/reactor_notify.go:294-321` - Ingress filter chain: iterates filters, handles modified payload
- [ ] `internal/component/bgp/plugins/role/otc.go:262-300` - OTCEgressFilter: parses wire bytes via extractAttrsFromPayload to find OTC attribute
- [ ] `internal/component/bgp/plugins/role/register.go:28` - EgressFilter registration in Registration struct

**Behavior to preserve:**
- Ingress filter chain: sequential, modified payload replaces original, fail-closed on panic (reject route)
- Egress filter chain: sequential, first rejection suppresses, fail-closed on panic (suppress route)
- Role OTC egress filter continues to work (wire parsing still valid until migrated)
- ReceivedUpdate immutability after cache insertion
- Zero-copy forwarding for same-context peers (metadata is sideband, not in wire bytes)

**Behavior to change:**
- EgressFilterFunc gains metadata (read) and mods (write) parameters
- IngressFilterFunc gains metadata return value
- ReceivedUpdate carries metadata set at ingress
- ForwardUpdate creates fresh mods map per peer, applies after filter chain passes

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Ingress: received UPDATE -> ingress filter chain -> filter sets metadata -> stored on ReceivedUpdate
- Egress: ForwardUpdate per-peer loop -> egress filter reads metadata + writes mods -> mods applied

### Transformation Path

**Path A: Received UPDATEs (from peers)**
1. Peer sends UPDATE -> reactor receives wire bytes
2. Ingress filter chain runs: each filter can set metadata on a builder (e.g., `meta["otc"] = asn`)
3. ReceivedUpdate created with metadata map -> cached
4. Plugin requests ForwardUpdate -> retrieves cached ReceivedUpdate with metadata
5. Per destination peer: egress filter receives (src, dest, payload, meta, mods)
6. Filter reads meta, optionally writes to mods, returns accept/reject
7. After all filters pass: if mods is non-empty, apply modifications to UPDATE before sending
8. Modified UPDATE (or original if no mods) dispatched to forward pool

**Path B: Plugin-originated UPDATEs (from RIB resend, API commands)**
1. Plugin calls SDK UpdateRouteWithMeta(ctx, selector, command, meta)
2. RPC sends UpdateRouteInput with Meta field
3. handleUpdateRouteRPC stores meta on CommandContext
4. Reactor processes text command, builds wire UPDATE, creates ReceivedUpdate
5. ReceivedUpdate.Meta set from CommandContext.Meta
6. ForwardUpdate retrieves cached entry -> same egress filter path as Path A

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ingress filter -> ReceivedUpdate | Metadata map set during filter chain, stored on ReceivedUpdate | [ ] |
| ReceivedUpdate -> ForwardUpdate | Metadata read from cached entry, passed to egress filters | [ ] |
| Egress filter -> forward path | Mods map written by filter, applied by ForwardUpdate after chain | [ ] |
| Forward path -> wire | Mods applied: re-encode UPDATE with modified attributes | [ ] |

### Integration Points
- `registry.EgressFilterFunc` - signature change (all registered filters must update)
- `registry.IngressFilterFunc` - signature change (all registered filters must update)
- `role/otc.go` OTCEgressFilter + OTCIngressFilter - add new parameters (initially unused)
- `reactor_api_forward.go` ForwardUpdate - pass metadata, collect mods, apply
- `reactor_notify.go` ingress chain - collect metadata from filters

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (metadata is sideband, wire bytes unchanged unless mods applied)

## Architecture Decisions

### AD-1: Metadata on ReceivedUpdate, not PeerFilterInfo

Metadata describes the route, not the peer. It is set once at ingress and read many times by egress filters across all destination peers. Storing it on ReceivedUpdate (which is cached) avoids copying metadata per-peer.

`PeerFilterInfo` stays peer-only (Address, PeerAS).

### AD-2: Mods map is per-peer, fresh each iteration

Each destination peer gets a fresh `map[string]any` for modifications. Filters accumulate mods for that specific peer. After all filters pass, mods are applied. This prevents cross-peer contamination.

If no filter writes to mods (common case), the map stays nil -- zero allocation.

### AD-3: Lazy mods allocation

The mods map is only allocated when a filter first writes to it. The egress filter signature passes a pointer or setter function rather than pre-allocating a map for every peer:

```
type EgressFilterFunc func(source, dest PeerFilterInfo, payload []byte, meta map[string]any, mods ModAccumulator) bool
```

Where `ModAccumulator` is a thin type with `Set(key string, val any)` that lazily allocates the underlying map. This keeps the zero-mod path allocation-free.

### AD-4: IngressFilterFunc returns metadata alongside modified payload

Extended signature:

```
type IngressFilterFunc func(source PeerFilterInfo, payload []byte, meta map[string]any) (accept bool, modifiedPayload []byte)
```

The `meta` map is created once before the ingress chain and passed through all filters. Each filter can read and write to it. After the chain completes, the map is stored on ReceivedUpdate.

### AD-5: Modification application point

Mods are applied in ForwardUpdate after the egress filter chain passes, before dispatching to the forward pool. If mods are present, the UPDATE must be re-encoded. This reuses the existing re-encode path (already used for EBGP AS-PATH prepending via `message.Update` parsing).

An `applyMods(wireUpdate, mods) (modifiedWireUpdate, error)` function in the reactor handles each mod key:

| Mod Key | Wire Operation |
|---------|---------------|
| `"add:attr:community"` | Parse UPDATE attrs via `wireUpdate.Attrs()`, find COMMUNITIES attr (type 8), append community values, update attr length, re-encode into pool buffer |
| `"set:attr:local-preference"` | Find LOCAL_PREF attr (type 5), overwrite 4-byte value at known offset; if absent, add attr (flags+type+len+4 bytes value) |
| `"del:attr:local-preference"` | Find LOCAL_PREF attr (type 5), remove it from attrs, re-encode |
| `"del:nlri:<prefix>"` | Remove specific prefix from NLRI/MP_REACH sections (announce unchanged, fewer prefixes) |
| `"del:nlri:*"` | Remove all NLRIs from announce sections (empty UPDATE) |
| `"withdraw:nlri:*"` | Move all NLRIs to withdrawn-routes field, strip path attributes. Uses existing `message.Update` building path |
| `"withdraw:nlri:<prefix>"` | Move one prefix from announce to withdrawn-routes, keep rest as announce |

The `<action>:<target>:<name>` convention means new modifications can be added without changing the framework -- just register a handler for the new key in the applyMods dispatcher.

Unknown mod keys are logged and skipped (fail-open). The function returns the original wireUpdate unchanged if all keys are unknown.

The re-encoded UPDATE uses a pool buffer (same pool as EBGP variants). The modified buffer is attached to the fwdItem and released after the forward pool worker sends it.

### AD-7: UpdateRouteInput metadata field

`rpc.UpdateRouteInput` gains `Meta map[string]any` with JSON tag `"meta,omitempty"`. Backward compatible: existing plugins that do not set Meta send no extra bytes. The SDK gains `UpdateRouteWithMeta(ctx, selector, command, meta)` alongside the existing `UpdateRoute` (which passes nil meta).

`handleUpdateRouteRPC` copies `input.Meta` to `cmdCtx.Meta` (new field on CommandContext). The reactor, when creating ReceivedUpdate from a command, copies `cmdCtx.Meta` to `ReceivedUpdate.Meta`.

### AD-6: ModAccumulator type

```
type ModAccumulator struct {
    m map[string]any
}

func (a *ModAccumulator) Set(key string, val any) {
    if a.m == nil {
        a.m = make(map[string]any, 2)
    }
    a.m[key] = val
}

func (a *ModAccumulator) Len() int { return len(a.m) }
```

Passed by pointer to egress filters. Forward path checks `Len() > 0` after filter chain to decide whether to apply modifications.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Received UPDATE with ingress filter that sets metadata | -> | Metadata stored on ReceivedUpdate | `TestIngressFilterSetsMetadata` |
| ForwardUpdate with egress filter that reads metadata | -> | Filter receives metadata from cached update | `TestEgressFilterReadsMetadata` |
| Egress filter writes to mods accumulator | -> | Mods applied to UPDATE before sending | `TestEgressFilterModsApplied` |
| Egress filter with no mods | -> | Original UPDATE sent unchanged | `TestEgressFilterNoModsPassthrough` |
| Role OTC filters updated | -> | OTC still works with new signature | `TestOTCEgressFilterNewSignature` |
| Functional: metadata set at ingress, read at egress | -> | End-to-end metadata flow | `test/plugin/route-metadata.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ingress filter sets `meta["test-key"] = 42` | ReceivedUpdate.Meta contains `"test-key": 42` after caching |
| AC-2 | Egress filter reads `meta["test-key"]` | Value matches what ingress filter set |
| AC-3 | Egress filter calls `mods.Set("set:attr:local-preference", uint32(0))` | Forward path receives non-empty mods, applies LOCAL_PREF=0 |
| AC-4 | Egress filter calls `mods.Set("add:attr:community", []string{"no-export"})` | Forward path appends community to UPDATE |
| AC-5 | No filter writes to mods | Original UPDATE sent unchanged (zero allocation) |
| AC-6 | Multiple egress filters, second writes to mods | Mods from all filters accumulated, all applied |
| AC-7 | OTC ingress filter with new signature | OTC still accepts/rejects and stamps correctly |
| AC-8 | OTC egress filter with new signature | OTC still suppresses correctly |
| AC-9 | Mods applied to one peer, not another | Each peer gets independent mods (no cross-contamination) |
| AC-10 | Plugin calls UpdateRouteWithMeta with `meta["stale"] = 2` | ReceivedUpdate.Meta contains `"stale": 2` |
| AC-11 | Egress filter sets `mods.Set("withdraw:nlri:*", true)` | Forward path sends withdrawal instead of announce |
| AC-12 | applyMods with `"add:attr:community"` | Community appended in wire bytes, original communities preserved |
| AC-13 | applyMods with `"set:attr:local-preference"` | LOCAL_PREF value overwritten in wire bytes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModAccumulator_LazyAlloc` | `registry/registry_test.go` | Set allocates on first write, Len=0 when empty | |
| `TestModAccumulator_MultipleKeys` | `registry/registry_test.go` | Multiple Set calls accumulated | |
| `TestIngressFilterSetsMetadata` | `reactor_notify_test.go` | Ingress filter meta map stored on ReceivedUpdate | |
| `TestEgressFilterReadsMetadata` | `reactor_api_forward_test.go` | Egress filter receives meta from ReceivedUpdate | |
| `TestEgressFilterModsApplied` | `reactor_api_forward_test.go` | Mods applied to UPDATE (LOCAL_PREF override) | |
| `TestEgressFilterNoModsPassthrough` | `reactor_api_forward_test.go` | No mods -> original wire bytes sent | |
| `TestEgressFilterModsPerPeer` | `reactor_api_forward_test.go` | AC-9: mods independent per peer | |
| `TestOTCEgressFilterNewSignature` | `role/otc_test.go` | OTC egress filter works with meta+mods params | |
| `TestOTCIngressFilterNewSignature` | `role/otc_test.go` | OTC ingress filter works with meta param | |
| `TestApplyMods_AddCommunity` | `reactor_api_forward_test.go` | AC-4: community appended to UPDATE | |
| `TestApplyMods_SetLocalPref` | `reactor_api_forward_test.go` | AC-13: LOCAL_PREF overridden in UPDATE | |
| `TestApplyMods_Withdraw` | `reactor_api_forward_test.go` | AC-11: announce converted to withdrawal | |
| `TestUpdateRouteWithMeta` | `sdk/sdk_engine_test.go` | AC-10: meta carried through RPC to ReceivedUpdate | |
| `TestCommandContextMeta` | `server/dispatch_test.go` | Meta copied from RPC input to CommandContext | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no new numeric inputs in this spec | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `route-metadata` | `test/plugin/route-metadata.ci` | Metadata set at ingress, visible in egress filter behavior | |

### Future (if deferring any tests)
- Role OTC migration to use metadata instead of wire parsing (separate spec)
- Multi-protocol weight tests (future spec)

## Files to Modify

- `internal/component/plugin/registry/registry.go` - EgressFilterFunc, IngressFilterFunc signatures, ModAccumulator type, PeerFilterInfo unchanged
- `internal/component/bgp/reactor/received_update.go` - Add Meta field to ReceivedUpdate
- `internal/component/bgp/reactor/reactor_notify.go` - safeEgressFilter, safeIngressFilter updated signatures; ingress chain collects metadata
- `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate passes meta to egress filters, creates mods per peer, applies after chain; applyMods function
- `internal/component/bgp/plugins/role/otc.go` - OTCEgressFilter, OTCIngressFilter updated signatures (params added, initially unused)
- `internal/component/bgp/plugins/role/otc_test.go` - Update test calls for new signatures
- `pkg/plugin/rpc/types.go` - Add Meta field to UpdateRouteInput
- `pkg/plugin/sdk/sdk_engine.go` - Add UpdateRouteWithMeta function
- `internal/component/plugin/server/command.go` - Add Meta field to CommandContext
- `internal/component/plugin/server/dispatch.go` - Pass meta from RPC input to CommandContext

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/route-metadata.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- document route metadata and modification accumulator in forward path section |

## Files to Create

- `test/plugin/route-metadata.ci` - functional test for metadata flow

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

1. **Phase: ModAccumulator type** -- lazy-alloc accumulator in registry package
   - Tests: `TestModAccumulator_LazyAlloc`, `TestModAccumulator_MultipleKeys`
   - Files: `registry/registry.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Filter signature changes** -- update EgressFilterFunc, IngressFilterFunc
   - Tests: `TestOTCEgressFilterNewSignature`, `TestOTCIngressFilterNewSignature`
   - Files: `registry/registry.go`, `role/otc.go`, `role/otc_test.go`
   - Verify: tests fail -> implement -> tests pass. All existing OTC tests still pass.

3. **Phase: Safe filter wrappers** -- update safeEgressFilter, safeIngressFilter
   - Tests: existing panic recovery tests still pass
   - Files: `reactor_notify.go`
   - Verify: compilation + existing tests pass

4. **Phase: ReceivedUpdate metadata** -- add Meta field, set during ingress
   - Tests: `TestIngressFilterSetsMetadata`
   - Files: `received_update.go`, `reactor_notify.go` (ingress chain)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: ForwardUpdate metadata + mods** -- pass meta to egress, create mods per peer
   - Tests: `TestEgressFilterReadsMetadata`, `TestEgressFilterNoModsPassthrough`, `TestEgressFilterModsPerPeer`
   - Files: `reactor_api_forward.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Mod application** -- apply mods to UPDATE after filter chain
   - Tests: `TestEgressFilterModsApplied`, `TestApplyMods_AddCommunity`, `TestApplyMods_SetLocalPref`
   - Files: `reactor_api_forward.go`
   - Verify: tests fail -> implement -> tests pass

7. **Functional test** -- create `test/plugin/route-metadata.ci`

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Metadata flows ingress -> cache -> egress; mods are per-peer; lazy alloc works |
| Naming | `Meta`, `ModAccumulator`, `Set()` consistent with Go conventions |
| Data flow | Metadata set once at ingress, read-only at egress; mods fresh per peer |
| Rule: no-layering | No old filter path kept alongside new; signature change is clean |
| Rule: buffer-first | Mod application reuses existing re-encode path, no new allocations in hot path without mods |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| ModAccumulator type | grep `ModAccumulator` in registry.go |
| EgressFilterFunc new signature | grep `EgressFilterFunc` in registry.go, check meta+mods params |
| IngressFilterFunc new signature | grep `IngressFilterFunc` in registry.go, check meta param |
| ReceivedUpdate.Meta field | grep `Meta` in received_update.go |
| ForwardUpdate passes meta | grep `meta` in reactor_api_forward.go egress filter section |
| Role OTC updated | grep `meta` or `mods` in role/otc.go |
| Functional test | ls `test/plugin/route-metadata.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Metadata keys from ingress filters are plugin-defined; no external input |
| Resource exhaustion | Metadata map bounded by number of ingress filters (small, fixed); mods map bounded by filter count per peer |
| Type safety | `map[string]any` values must be type-asserted; panic recovery in safeEgressFilter catches assertion failures |

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

## RFC Documentation

N/A -- this is infrastructure, not protocol implementation. Consuming specs (LLGR-4) add RFC comments.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Write learned summary to `plan/learned/NNN-route-metadata.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
