# Spec: redistribution-filter

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/wire-format.md` - IPC framing
4. `docs/architecture/api/process-protocol.md` - 5-stage startup
5. `internal/component/plugin/registry/registry.go` - Registration struct, filter types
6. `internal/component/bgp/reactor/reactor_notify.go` - ingress filter chain
7. `internal/component/bgp/reactor/reactor_api_forward.go` - egress filter chain
8. `internal/core/ipc/schema/ze-plugin-engine.yang` - stage 1 declaration RPC

## Task

Add policy filter support to ze's plugin protocol so external plugins can act as route filters. A normal plugin declares filter capability and requested attributes during the wire protocol handshake (stage 1). The user references plugin names in a `redistribution { import [...] export [...] }` config block at bgp/group/peer levels. On UPDATE, the reactor sends a tailored text representation (only declared attributes) to each filter in chain order. Filters respond accept/reject/modify. Modified attributes use dirty tracking and pointer-swap for efficient re-encoding.

This spec covers external plugin filters only. Internal/in-engine optimized filters are a separate future spec.

## Design Decisions (agreed with user)

| Decision | Rationale |
|----------|-----------|
| Plugins are normal plugins | No new plugin type. `plugin { external <name> { ... } }` as usual. Filter capability declared over wire protocol at stage 1 |
| Wire protocol declares filter | New fields in `declare-registration`: filter direction(s) and requested attribute list |
| No plugin-side config in ze | Plugin handles its own filtering logic. Ze only knows: "this plugin is a filter, it wants these attributes" |
| `redistribution {}` config block | Under `peer-fields` grouping (group/peer level) AND bgp container (global level). Contains `import [...]` and `export [...]` leaf-lists |
| Config hierarchy: bgp > group > peer | Cumulative: bgp-level filters run first, then group-level, then peer-level |
| Chain semantics: piped transforms | Each filter sees output of previous filter. Short-circuit on reject. Default accept at end of chain |
| Per-UPDATE granularity | Filters operate on full UPDATE, not per-NLRI. To filter specific NLRIs, return a modified NLRI list |
| Withdrawals go through filters | Enables prefix-length filtering and default-route blocking on withdrawals |
| Plugin declares requested attributes | At stage 1 via `declare-registration`. Reactor parses only the union of all declared attributes across the chain |
| Subset input, delta-only output | Filter receives only declared attributes as text. Responds with only changed fields on modify |
| Dirty field tracking | Modified fields marked dirty. Only dirty fields trigger re-encoding of the cached UPDATE |
| Pointer-swap for parsed attributes | When a filter modifies an attribute, replace the parsed object pointer in the cache |
| Raw mode available | Plugin can request raw wire bytes instead of/in addition to parsed attributes. Forces full re-parse on modify (inefficient but flexible) |
| Plugin declares failure mode | `on-error` field in stage 1: reject (fail-closed) or accept (fail-open). Plugin author knows the security semantics of their filter |
| Role plugin stays as-is | In-process Go closures via IngressFilter/EgressFilter. Runs first in chain (RFC-mandated). Shares chain position but not the new protocol |
| Same plugin usable multiple times in chain | User may list a plugin name more than once in import/export (e.g., prepend twice) |
| External plugins first | IPC over text protocol. Internal fast-path filters are a separate future spec |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/wire-format.md` - IPC framing format
  -> Constraint: all messages use `#<id> <verb> [<json>]` framing over MuxConn
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage startup protocol
  -> Constraint: stage 1 is `declare-registration`, plugin -> engine over socket A
  -> Constraint: barriers between stages, tier-ordered startup
- [ ] `docs/architecture/core-design.md` - reactor, wire layer, event dispatch
  -> Constraint: WireUpdate is lazy-parsed, zero-copy from pool buffer
  -> Constraint: per-attribute-type pools with refcounted handles

### RFC Summaries (MUST for protocol work)
- [ ] No specific RFC governs policy filters. This is ze-internal protocol design.

**Key insights:**
- Stage 1 `declare-registration` currently has: family list, command list, wants-config, schema. New filter fields extend this RPC.
- Ingress filters run after wire parse, before cache. Egress filters run per-destination-peer during forward.
- IPC uses `#<id> <verb> [<json>]` over MuxConn. Filter requests/responses fit this model.
- Current in-process filters receive raw `[]byte` payload. External filters receive text representation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - Registration struct with IngressFilter/EgressFilter fields, PeerFilterInfo struct, IngressFilterFunc/EgressFilterFunc types, IngressFilters()/EgressFilters() query functions
- [ ] `internal/component/bgp/reactor/reactor_notify.go:270-297` - ingress filter chain: iterates r.ingressFilters, calls safeIngressFilter with panic recovery, short-circuits on reject, creates new WireUpdate on modify
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:234-268` - egress filter chain: builds PeerFilterInfo per source/dest, iterates r.egressFilters per destination peer, suppresses on reject
- [ ] `internal/component/bgp/reactor/reactor_notify.go:22-45` - safeIngressFilter and safeEgressFilter: panic recovery wrappers, fail-closed on panic (reject/suppress)
- [ ] `internal/core/ipc/schema/ze-plugin-engine.yang:18-82` - declare-registration RPC with family, command, wants-config, schema fields
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang:83-435` - peer-fields grouping with capability, family, process, etc.
- [ ] `internal/component/bgp/plugins/role/register.go` - role plugin registers IngressFilter and EgressFilter closures
- [ ] `internal/component/bgp/plugins/role/otc.go` - OTC ingress filter (stamp/reject), OTC egress filter (suppress per-peer)

**Behavior to preserve:**
- Role plugin's IngressFilter/EgressFilter mechanism unchanged
- Role filters run before any policy filter chain (RFC-mandated, hard reject)
- Current filter invocation in reactor_notify.go and reactor_api_forward.go stays as-is for in-process filters
- safeIngressFilter/safeEgressFilter panic recovery pattern
- Fail-open on filter panic
- 5-stage startup protocol for all plugins
- WireUpdate lazy parsing and zero-copy
- `#<id> <verb> [<json>]` IPC framing

**Behavior to change:**
- Extend `declare-registration` RPC with filter capability fields
- Add `redistribution {}` to ze-bgp-conf.yang (peer-fields grouping + bgp container)
- Add config parsing for redistribution block
- Add per-peer filter chain resolution (bgp > group > peer cumulative)
- Add filter IPC interaction at runtime (engine sends update text, plugin responds)
- Add attribute accumulation across filter chain (union of declared attributes)
- Add dirty tracking on modify for efficient re-encoding

## Data Flow (MANDATORY)

### Entry Point: Import (Ingress) Filter Chain

| Step | Location | Format |
|------|----------|--------|
| Wire bytes received | reactor_notify.go | Raw BGP UPDATE body |
| In-process ingress filters (role OTC) | reactor_notify.go:270-297 | `[]byte` payload |
| **NEW: resolve peer's import filter chain** | reactor (new code) | Config: bgp + group + peer merged |
| **NEW: parse declared attributes** | reactor (new code) | Lazy parse only union of requested attributes |
| **NEW: for each filter in import chain** | reactor (new code) | Send text via IPC, receive response |
| **NEW: on modify, update cached attributes** | reactor (new code) | Pointer-swap dirty attributes, re-encode if needed |
| Cache in recentUpdates | reactor_notify.go:299-313 | WireUpdate (possibly modified) |

### Entry Point: Export (Egress) Filter Chain

| Step | Location | Format |
|------|----------|--------|
| Forward request | reactor_api_forward.go | UpdateID -> cached ReceivedUpdate |
| In-process egress filters (role OTC) | reactor_api_forward.go:252-268 | `[]byte` payload, per-peer |
| **NEW: resolve destination peer's export filter chain** | reactor (new code) | Config: bgp + group + peer merged |
| **NEW: parse declared attributes** | reactor (new code) | Lazy parse only union of requested attributes |
| **NEW: for each filter in export chain** | reactor (new code) | Send text via IPC, receive response |
| **NEW: on reject, skip peer** | reactor (new code) | Same as current egress filter short-circuit |
| **NEW: on modify, use modified version for this peer** | reactor (new code) | Dirty tracking per forward, not cached globally |
| Select wire version (IBGP/EBGP) | reactor_api_forward.go:270+ | Per-peer wire |

### Entry Point: Stage 1 Declaration

| Step | Location | Format |
|------|----------|--------|
| Plugin sends `declare-registration` | IPC wire (socket A) | `#<id> ze-plugin-engine:declare-registration {... "filter": {...} ...}` |
| Engine records filter capability | Plugin coordinator / process manager | New fields on plugin runtime state |
| Config validation | Config parsing | `redistribution { import [...] }` names must match filter-capable plugins |

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> Plugin (declaration) | JSON in `declare-registration` RPC at stage 1 | [ ] |
| Engine <-> Plugin (runtime filter) | New callback RPC `filter-update` over socket B | [ ] |
| Config -> Reactor | Resolved per-peer filter chains from config tree | [ ] |
| Wire bytes <-> Text representation | Selective attribute parsing + text formatting | [ ] |

### Integration Points
- `registry.go` Registration struct - no changes needed (filter capability is runtime/IPC, not compile-time)
- `reactor_notify.go` ingress filter loop - new policy chain after existing in-process filters
- `reactor_api_forward.go` egress filter loop - new policy chain after existing in-process filters
- `ze-plugin-engine.yang` - extend declare-registration input
- `ze-bgp-conf.yang` - add redistribution container to peer-fields and bgp
- `ze-plugin-callback.yang` - new filter-update RPC (engine -> plugin)
- Config resolution (`component/config/`) - parse and merge redistribution blocks

### Architectural Verification
- [ ] No bypassed layers (filters integrate into existing reactor filter points)
- [ ] No unintended coupling (plugins don't know about each other, only engine orchestrates)
- [ ] No duplicated functionality (extends filter chain, doesn't recreate it)
- [ ] Zero-copy preserved where applicable (parsed attribute pointers swapped, not copied)

## Wire Protocol Design

### Stage 1: Filter Declaration

Extend `declare-registration` input with a new `filter` container:

| Field | Type | Description |
|-------|------|-------------|
| `filter.direction` | enum: import, export, both | Which direction(s) this plugin filters |
| `filter.attributes` | leaf-list of string | Attribute names to receive (e.g., "local-preference", "community", "as-path") |
| `filter.nlri` | boolean (default true) | Whether to include NLRI list in filter input |
| `filter.raw` | boolean (default false) | Whether to include raw wire bytes (forces full re-parse on modify) |
| `filter.on-error` | enum: reject, accept | Behavior when IPC fails (timeout, crash, malformed response). Plugin declares its own failure semantics |

Example wire message:
```
#5 ze-plugin-engine:declare-registration {"filter":{"direction":"both","attributes":["local-preference","community"],"nlri":true,"on-error":"reject"}}
```

### Runtime: Filter Request (engine -> plugin, socket B)

New RPC in `ze-plugin-callback`: `filter-update`

| Field | Type | Description |
|-------|------|-------------|
| `direction` | string | "import" or "export" |
| `peer` | string | Peer IP address |
| `peer-as` | uint32 | Peer ASN |
| `update` | string | Text-format update with only declared attributes and NLRI |
| `raw` | string (optional) | Hex-encoded raw UPDATE body (only if plugin declared raw=true) |

Example:
```
#42 ze-plugin-callback:filter-update {"direction":"import","peer":"10.0.0.1","peer-as":65001,"update":"local-preference 100 community 65000:1 65000:2 nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24"}
```

### Runtime: Filter Response (plugin -> engine)

| Response | Format | Meaning |
|----------|--------|---------|
| Accept | `#42 ok {"action":"accept"}` | Pass update through unchanged |
| Reject | `#42 ok {"action":"reject"}` | Drop update (short-circuit, no further filters) |
| Modify | `#42 ok {"action":"modify","update":"local-preference 200"}` | Delta-only: changed fields. Plugin can only modify attributes it declared |
| Modify with NLRI | `#42 ok {"action":"modify","update":"nlri ipv4/unicast add 10.0.0.0/24"}` | Filtered NLRI list (removed prefixes are dropped) |
| Modify with raw | `#42 ok {"action":"modify","raw":"400101..."}` | Full raw UPDATE body replacement (forces re-parse) |

### Attribute Name Vocabulary

Attribute names used in filter declaration and text format:

| Attribute Name | Wire Code | Text Format |
|----------------|-----------|-------------|
| `origin` | 1 | `origin igp` / `origin egp` / `origin incomplete` |
| `as-path` | 2 | `as-path 65001 65002 65003` |
| `next-hop` | 3 | `next-hop 10.0.0.1` |
| `med` | 4 | `med 100` |
| `local-preference` | 5 | `local-preference 200` |
| `atomic-aggregate` | 6 | `atomic-aggregate` |
| `aggregator` | 7 | `aggregator 65001:10.0.0.1` |
| `community` | 8 | `community 65000:1 65000:2` |
| `originator-id` | 9 | `originator-id 1.1.1.1` |
| `cluster-list` | 10 | `cluster-list 1.1.1.1 2.2.2.2` |
| `extended-community` | 16 | `extended-community target:65000:100` |
| `large-community` | 32 | `large-community 65000:1:2` |

NLRI is always formatted as: `nlri <family> add <prefix>...` or `nlri <family> del <prefix>...`

## Config Design

### YANG Schema Addition

Add `redistribution` container to `peer-fields` grouping (covers group and peer levels) and separately to `bgp` container (covers global level):

| Location | Container | Children |
|----------|-----------|----------|
| `grouping peer-fields` | `redistribution` | `leaf-list import`, `leaf-list export` |
| `container bgp` | `redistribution` | `leaf-list import`, `leaf-list export` |

Leaf-list values are plugin names (strings). Validated at config load: each name must correspond to a plugin that declared filter capability for the matching direction.

### Config Examples

Global level (applies to all peers):
```
bgp {
    redistribution {
        import [ rpki-filter ]
    }
}
```

Group level (cumulative with global):
```
bgp {
    redistribution {
        import [ rpki-filter ]
    }
    group customers {
        redistribution {
            import [ community-scrub ]
        }
        peer customer-a {
            remote { ip 10.0.0.1; as 65001; }
            redistribution {
                export [ prepend-filter ]
            }
        }
    }
}
```

Effective chains for customer-a:
- Import: `rpki-filter` (bgp) -> `community-scrub` (group)
- Export: `prepend-filter` (peer)

### Config Resolution

| Level | Merge Rule |
|-------|-----------|
| bgp | Base chain |
| group | Append to bgp chain |
| peer | Append to group chain |

Result: ordered list of plugin names per peer per direction. Resolved once at config load, stored on peer settings.

### Config Validation

| Check | Error |
|-------|-------|
| Plugin name not found | `redistribution: unknown plugin "<name>"` |
| Plugin not filter-capable | `redistribution: plugin "<name>" did not declare filter capability` |
| Plugin declared import-only, used in export | `redistribution: plugin "<name>" only supports import filtering` |
| Plugin declared export-only, used in import | `redistribution: plugin "<name>" only supports import filtering` |

Note: config validation for filter capability happens after stage 1 (plugins must declare before config can validate). This means redistribution validation is deferred until after plugin startup.

## Attribute Accumulation

When resolving filter chains, the reactor computes the union of all declared attributes across all filters in the chain. Parsing happens once per UPDATE for the accumulated set.

| Filter | Declares | Union |
|--------|----------|-------|
| rpki-filter | as-path, origin | as-path, origin |
| community-scrub | community | as-path, origin, community |

The reactor parses as-path, origin, and community once. rpki-filter receives only as-path and origin. community-scrub receives only community.

When community-scrub returns `modify community 65000:99`, the reactor:
1. Marks community as dirty
2. Replaces the parsed community attribute pointer
3. After chain completes, re-encodes only dirty attributes into the UPDATE payload

## Dirty Tracking and Pointer-Swap

| Step | What Happens |
|------|-------------|
| Chain starts | Parse union of declared attributes into typed objects. Record original pointers. |
| Filter returns modify | Parse modified text value. Replace pointer for that attribute. Mark dirty. |
| Chain ends, no dirty fields | Use original wire bytes unchanged (zero overhead). |
| Chain ends, dirty fields | Re-encode UPDATE: unchanged attributes from original wire, dirty attributes from new objects. |

For export filters, modifications are per-peer (don't affect the cached version). For import filters, modifications affect the cached version (before storage).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `redistribution { import [...] }` config | -> | Config parsing + filter chain resolution | `test/plugin/redistribution-import-accept.ci` |
| Received UPDATE + import filter | -> | Reactor ingress policy chain + IPC | `test/plugin/redistribution-import-reject.ci` |
| Received UPDATE + import filter modify | -> | Reactor ingress policy chain + dirty tracking | `test/plugin/redistribution-import-modify.ci` |
| Forward UPDATE + export filter | -> | Reactor egress policy chain + IPC | `test/plugin/redistribution-export-reject.ci` |
| Stage 1 filter declaration | -> | Plugin startup + filter registration | `test/plugin/redistribution-declare.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin sends `declare-registration` with `filter` field | Engine records filter capability and requested attributes |
| AC-2 | Config has `redistribution { import [ plugin-a ] }` and plugin-a declared import filter | Config validates successfully, filter chain resolved |
| AC-3 | Config has `redistribution { import [ unknown ] }` | Config error: unknown plugin |
| AC-4 | Config has `redistribution { export [ import-only-plugin ] }` | Config error: plugin only supports import |
| AC-5 | Received UPDATE, import filter returns accept | UPDATE cached and dispatched normally |
| AC-6 | Received UPDATE, import filter returns reject | UPDATE dropped, not cached, not dispatched |
| AC-7 | Received UPDATE, import filter returns modify with changed local-pref | UPDATE cached with modified local-pref, dirty re-encoding |
| AC-8 | Received UPDATE, import filter returns modify with filtered NLRI list | UPDATE cached with reduced NLRI set |
| AC-9 | Forward UPDATE, export filter returns reject | UPDATE not sent to that peer |
| AC-10 | Forward UPDATE, export filter returns modify | Modified version sent to that peer only, cache unchanged |
| AC-11 | Three filters in chain: first modifies, second sees modification, third accepts | Piped transform semantics work end-to-end |
| AC-12 | Config hierarchy: bgp import [ a ], group import [ b ], peer import [ c ] | Effective chain: a -> b -> c |
| AC-13 | Filter declared `attributes: [local-preference]` but modify response changes community | Error: plugin can only modify declared attributes |
| AC-14 | Role OTC rejects on ingress | Policy filter chain never runs (role is first) |
| AC-15 | Plugin declares raw=true | Filter receives raw hex in addition to text attributes |
| AC-16 | Withdrawal UPDATE goes through import filter | Filter can reject withdrawal (e.g., block default route withdraw) |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilterDeclarationParse` | `internal/core/ipc/declaration_test.go` | Parse filter fields from declare-registration JSON | |
| `TestRedistributionConfigParse` | `internal/component/config/redistribution_test.go` | Parse redistribution YANG config block | |
| `TestRedistributionConfigValidation` | `internal/component/config/redistribution_test.go` | Reject unknown/incompatible plugin names | |
| `TestFilterChainResolution` | `internal/component/config/redistribution_test.go` | Merge bgp > group > peer chains correctly | |
| `TestAttributeAccumulation` | `internal/component/bgp/reactor/filter_chain_test.go` | Union of declared attributes computed correctly | |
| `TestFilterResponseParse` | `internal/component/bgp/reactor/filter_chain_test.go` | Parse accept/reject/modify responses | |
| `TestDirtyTracking` | `internal/component/bgp/reactor/filter_chain_test.go` | Only dirty attributes trigger re-encoding | |
| `TestFilterModifyOnlyDeclared` | `internal/component/bgp/reactor/filter_chain_test.go` | Reject modify of undeclared attribute | |
| `TestFilterChainPipedTransform` | `internal/component/bgp/reactor/filter_chain_test.go` | Second filter sees first filter's modifications | |
| `TestFilterChainShortCircuit` | `internal/component/bgp/reactor/filter_chain_test.go` | Reject stops chain, no further filters called | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Attribute count in declaration | 0-N | N (all known) | N/A | Unknown attribute name -> error |
| Filter chain length | 0-N | N (many filters) | N/A | N/A (no max, but performance) |
| NLRI count in modify response | 0-N | 0 (empty = drop all) | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribution-import-accept` | `test/plugin/redistribution-import-accept.ci` | External filter plugin accepts all routes, routes pass through | |
| `redistribution-import-reject` | `test/plugin/redistribution-import-reject.ci` | External filter plugin rejects routes, routes dropped | |
| `redistribution-import-modify` | `test/plugin/redistribution-import-modify.ci` | External filter plugin modifies local-pref, cached value reflects change | |
| `redistribution-export-reject` | `test/plugin/redistribution-export-reject.ci` | Export filter rejects for specific peer, route not forwarded to that peer | |
| `redistribution-declare` | `test/plugin/redistribution-declare.ci` | Plugin declares filter capability at stage 1, config validates | |
| `redistribution-chain-order` | `test/plugin/redistribution-chain-order.ci` | Multiple filters in chain, execution order matches config | |

### Future (if deferring any tests)
- Property testing: random UPDATE payloads through filter chains preserve invariants
- Performance benchmarks: filter chain overhead per UPDATE
- Chaos tests: filter plugin crash/timeout during filtering

## Files to Modify

- `internal/core/ipc/schema/ze-plugin-engine.yang` - add filter container to declare-registration input
- `internal/core/ipc/schema/ze-plugin-callback.yang` - add filter-update RPC
- `internal/core/ipc/declaration.go` (or equivalent) - parse filter fields from JSON
- `internal/component/bgp/schema/ze-bgp-conf.yang` - add redistribution container to peer-fields and bgp
- `internal/component/config/` - parse redistribution config, resolve filter chains
- `internal/component/bgp/reactor/reactor.go` - store resolved filter chains, wire into startup
- `internal/component/bgp/reactor/reactor_notify.go` - add policy filter chain after in-process ingress filters
- `internal/component/bgp/reactor/reactor_api_forward.go` - add policy filter chain after in-process egress filters
- `pkg/plugin/sdk/` - add filter callback support for plugin-side SDK

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `ze-plugin-engine.yang` (extend), `ze-plugin-callback.yang` (new RPC) |
| YANG schema (config) | [x] | `ze-bgp-conf.yang` (redistribution container) |
| CLI commands/flags | [ ] | N/A -- no CLI command needed (YAGNI) |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/redistribution-*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add redistribution filter support |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - redistribution block syntax, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - filter-update RPC |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` - filter plugin capability |
| 6 | Has a user guide page? | [x] | `docs/guide/redistribution.md` - new guide page |
| 7 | Wire format changed? | [ ] | N/A (uses existing framing) |
| 8 | Plugin SDK/protocol changed? | [x] | `.claude/rules/plugin-design.md` - filter declaration, `docs/architecture/api/process-protocol.md` - stage 1 filter fields |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - policy filter support |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - filter chain, attribute accumulation |

## Files to Create

- `internal/component/bgp/reactor/filter_chain.go` - policy filter chain execution, attribute accumulation, dirty tracking, IPC interaction
- `internal/component/bgp/reactor/filter_chain_test.go` - unit tests for filter chain
- `internal/component/config/redistribution.go` - config parsing and chain resolution
- `internal/component/config/redistribution_test.go` - config tests
- `docs/guide/redistribution.md` - user guide for redistribution filters
- `test/plugin/redistribution-import-accept.ci` - functional test
- `test/plugin/redistribution-import-reject.ci` - functional test
- `test/plugin/redistribution-import-modify.ci` - functional test
- `test/plugin/redistribution-export-reject.ci` - functional test
- `test/plugin/redistribution-declare.ci` - functional test
- `test/plugin/redistribution-chain-order.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
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

1. **Phase: YANG schemas** -- Extend declare-registration with filter container. Add redistribution to ze-bgp-conf.yang. Add filter-update callback RPC.
   - Tests: `TestFilterDeclarationParse`, `TestRedistributionConfigParse`
   - Files: `ze-plugin-engine.yang`, `ze-bgp-conf.yang`, `ze-plugin-callback.yang`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Config parsing** -- Parse redistribution blocks. Resolve filter chains with bgp > group > peer merging. Validate plugin names against filter-capable plugins.
   - Tests: `TestRedistributionConfigValidation`, `TestFilterChainResolution`
   - Files: `redistribution.go`, `redistribution_test.go`, config pipeline files
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Filter chain runtime** -- Implement attribute accumulation, IPC interaction (send text, parse response), chain execution with piped transforms, short-circuit on reject.
   - Tests: `TestAttributeAccumulation`, `TestFilterResponseParse`, `TestFilterChainPipedTransform`, `TestFilterChainShortCircuit`
   - Files: `filter_chain.go`, `filter_chain_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Dirty tracking** -- Implement dirty field marking, pointer-swap for modified attributes, selective re-encoding of UPDATE payload.
   - Tests: `TestDirtyTracking`, `TestFilterModifyOnlyDeclared`
   - Files: `filter_chain.go`, `filter_chain_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Reactor wiring** -- Wire policy filter chain into reactor ingress (after role) and egress (after role, per-peer). Integrate with config-resolved per-peer chains.
   - Tests: functional tests
   - Files: `reactor_notify.go`, `reactor_api_forward.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: SDK support** -- Add filter callback support to plugin SDK so external plugins can register filter handlers.
   - Tests: SDK tests
   - Files: `pkg/plugin/sdk/` files
   - Verify: tests fail -> implement -> tests pass

7. **Functional tests** -- Create .ci tests with external test filter plugin.
   - Files: `test/plugin/redistribution-*.ci`

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation with file:line |
| Correctness | Filter chain order matches config hierarchy (bgp > group > peer) |
| Correctness | Modify response only accepted for declared attributes |
| Correctness | Dirty tracking correctly identifies changed vs unchanged attributes |
| Naming | JSON keys use kebab-case in filter declaration and response |
| Naming | YANG uses kebab-case for all new elements |
| Data flow | Ingress policy chain runs after role, before cache |
| Data flow | Egress policy chain runs after role, per destination peer |
| Data flow | Export modifications per-peer only, don't affect cache |
| Rule: buffer-first | Any re-encoding uses pooled buffers |
| Rule: goroutine-lifecycle | No per-UPDATE goroutines for IPC (reuse plugin's MuxConn) |
| Rule: plugin-design | Filter plugins registered through normal plugin lifecycle |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| declare-registration extended with filter fields | grep "filter" ze-plugin-engine.yang |
| redistribution container in ze-bgp-conf.yang | grep "redistribution" ze-bgp-conf.yang |
| filter-update callback RPC | grep "filter-update" ze-plugin-callback.yang |
| Config parsing for redistribution | grep "redistribution" internal/component/config/*.go |
| Filter chain execution in reactor | grep "policyFilter\|filterChain" internal/component/bgp/reactor/*.go |
| Dirty tracking | grep "dirty\|Dirty" internal/component/bgp/reactor/filter_chain.go |
| SDK filter callback | grep "filter\|Filter" pkg/plugin/sdk/*.go |
| Functional tests exist | ls test/plugin/redistribution-*.ci |
| Documentation updated | ls docs/guide/redistribution.md |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Filter response from external plugin: validate action is accept/reject/modify, validate modified attributes are in declared set |
| Input validation | Modify response text parsing: malformed text must not crash reactor |
| Resource exhaustion | Filter plugin timeout: IPC call must have timeout, don't block reactor forever |
| Resource exhaustion | Large modify response: bound the size of modify text accepted |
| Error leakage | Filter errors should not expose internal state to plugin |
| Failure mode per-plugin | On filter IPC error/timeout: use the plugin's declared `on-error` (reject or accept). Log error with peer and filter name |

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

## Open Questions

All resolved.

| # | Question | Resolution |
|---|----------|-----------|
| 1 | On filter IPC error/timeout: fail-open or fail-closed? | **Plugin declares its own failure mode** via `on-error` field in stage 1 (reject or accept). |
| 2 | IPC timeout value for filter-update RPC? | **5 seconds per filter.** |
| 3 | SDK convenience callback (`OnFilterUpdate`)? | **Yes.** |
| 4 | Export modify needs re-encoding? | **Yes,** must re-encode for wire transmission. |

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

No RFC governs this feature. This is ze-internal protocol design.

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
- [ ] AC-1..AC-16 all demonstrated
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
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit**
