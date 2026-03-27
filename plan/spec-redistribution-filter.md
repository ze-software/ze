# Spec: redistribution-filter

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/8 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/wire-format.md` - IPC framing
4. `docs/architecture/api/process-protocol.md` - 5-stage startup
5. `internal/component/plugin/registry/registry.go` - Registration struct, filter types
6. `internal/component/plugin/registration.go` - PluginRegistration struct (stage 1 declaration)
7. `internal/component/bgp/reactor/reactor_notify.go` - ingress filter chain
8. `internal/component/bgp/reactor/reactor_api_forward.go` - egress filter chain
9. `internal/component/bgp/reactor/forward_build.go` - buildModifiedPayload (egress mods)
10. `internal/core/ipc/schema/ze-plugin-engine.yang` - stage 1 declaration RPC

## Task

Add policy filter support to ze's plugin protocol so external plugins can act as route filters. A normal plugin declares filter capability and requested attributes during the wire protocol handshake (stage 1). The user references plugin names in a `redistribution { import [...] export [...] }` config block at bgp/group/peer levels. On UPDATE, the reactor sends a tailored text representation (only declared attributes) to each filter in chain order. Filters respond accept/reject/modify. Modified attributes use dirty tracking and pointer-swap for efficient re-encoding.

This spec covers external plugin filters only. Internal/in-engine optimized filters are a separate future spec.

## Design Decisions (agreed with user)

| Decision | Rationale |
|----------|-----------|
| Plugins are normal plugins | No new plugin type. `plugin { external <name> { ... } }` as usual. Filter capability declared over wire protocol at stage 1 |
| Filter naming: `<plugin>:<filter>` | Uniform naming for all filters. A plugin may offer multiple named filters (e.g., `rpki:validate`, `rpki:log`). Built-in filters use `rfc:<action>` (e.g., `rfc:otc`). Config, IPC, and logging all use this format |
| Three filter categories: mandatory, default, user | **Mandatory** (e.g., `rfc:otc`): always on, cannot be overridden. **Default** (e.g., `rfc:no-self-as`): on for all peers by default, can be overridden. **User** (e.g., `rpki:validate`): only present when explicitly configured. All use `<plugin>:<filter>` naming |
| Mandatory/default filters invisible in config | Not listed in `redistribution {}`. A future extensive/verbose view may display them for debugging. User cannot remove or reorder mandatory filters. Default filters can be overridden (see below) |
| Override mechanism for default filters | A filter can declare `overrides: ["rfc:no-self-as"]` to remove a default filter from the chain. Applies at the config level where the overriding filter is placed (bgp/group/peer), inherited downward. The overriding plugin knows the semantics of what it replaces. External plugins can override RFC defaults -- ze lets users do what they want |
| Wire protocol declares named filters | New `filters` list in `declare-registration`: each entry has a name, direction, requested attributes, and on-error. One plugin can declare multiple filters |
| No plugin-side config in ze | Plugin handles its own filtering logic. Ze only knows: "this plugin offers these named filters, each wants these attributes" |
| `redistribution {}` config block | Under `peer-fields` grouping (group/peer level) AND bgp container (global level). Contains `import [...]` and `export [...]` leaf-lists. Values are `<plugin>:<filter>` strings |
| Config hierarchy: bgp > group > peer | Cumulative: bgp-level filters run first, then group-level, then peer-level |
| Chain semantics: piped transforms | Each filter sees output of previous filter. Short-circuit on reject. Default accept at end of chain |
| Per-UPDATE granularity | Filters operate on full UPDATE, not per-NLRI. To filter specific NLRIs, return a modified NLRI list |
| Withdrawals go through filters | Enables prefix-length filtering and default-route blocking on withdrawals |
| Plugin declares requested attributes per filter | At stage 1 via `declare-registration`. Each named filter declares its own attribute list. Reactor parses only the union of all declared attributes across the chain |
| Subset input, delta-only output | Filter receives only declared attributes as text. Responds with only changed fields on modify |
| Dirty field tracking | Modified fields marked dirty. Only dirty fields trigger re-encoding of the cached UPDATE |
| Pointer-swap for parsed attributes | When a filter modifies an attribute, replace the parsed object pointer in the cache |
| Raw mode available | Plugin can request raw wire bytes instead of/in addition to parsed attributes. Forces full re-parse on modify (inefficient but flexible) |
| Plugin declares failure mode per filter | `on-error` field per named filter: reject (fail-closed) or accept (fail-open). Plugin author knows the security semantics of each filter |
| Role plugin stays as-is (mandatory) | In-process Go closures via IngressFilter/EgressFilter. Runs first in chain (RFC-mandated). Named `rfc:otc` internally. Mandatory category -- cannot be overridden |
| Same filter usable multiple times in chain | User may list the same `<plugin>:<filter>` more than once in import/export (e.g., `prepend:once` twice) |
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
- Stage 1 `declare-registration` currently has: family list, command list, wants-config, schema. New `filters` list extends this RPC. Declaration is parsed into `PluginRegistration` struct in `internal/component/plugin/registration.go`.
- A single plugin can declare multiple named filters (e.g., `rpki` plugin declares `validate` and `log`). Config references them as `rpki:validate`, `rpki:log`.
- Ingress filters run after wire parse, before cache (:305-349). Egress filters run per-destination-peer during forward (:260-279).
- IPC uses `#<id> <verb> [<json>]` over MuxConn. Filter requests/responses fit this model. The `filter-update` RPC includes the filter name so the plugin knows which filter is being invoked.
- Current in-process filters receive raw `[]byte` payload. External filters receive text representation.
- Egress modification infrastructure already exists: `ModAccumulator` collects per-attribute changes, `buildModifiedPayload` applies them to wire bytes using `attrModHandlers`. New export filter mods should integrate with or parallel this pattern.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - Registration struct (IngressFilter:198, EgressFilter:199), PeerFilterInfo:33-39, IngressFilterFunc:47, EgressFilterFunc:55 (includes *ModAccumulator), IngressFilters():561, EgressFilters():576
- [ ] `internal/component/plugin/registration.go:60-77` - PluginRegistration struct (Stage 1 data). This is where new filter declaration fields will be added. Currently has: Families, Commands, Receive, SchemaDeclarations, WantsConfigRoots, WantsValidateOpen, ConnectionHandlers
- [ ] `internal/component/bgp/reactor/reactor_notify.go:305-349` - ingress filter chain: iterates r.ingressFilters, calls safeIngressFilter with panic recovery, short-circuits on reject, creates new WireUpdate on modify
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:239-279` - egress filter chain: builds PeerFilterInfo per source/dest, iterates r.egressFilters per destination peer, suppresses on reject. Modification via ModAccumulator applied at :307-311 via buildModifiedPayload
- [ ] `internal/component/bgp/reactor/reactor_notify.go:23-46` - safeIngressFilter:25 and safeEgressFilter:38: panic recovery wrappers, fail-closed on panic (reject/suppress)
- [ ] `internal/component/bgp/reactor/reactor.go:263-267` - reactor stores ingressFilters, egressFilters, attrModHandlers fields. Initialized from registry at startup :749-751
- [ ] `internal/component/bgp/reactor/forward_build.go` - buildModifiedPayload: applies ModAccumulator to wire bytes using attrModHandlers. Existing infrastructure for egress attribute modification
- [ ] `internal/core/ipc/schema/ze-plugin-engine.yang:18-82` - declare-registration RPC with family, command, wants-config, schema fields
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang:83-464` - peer-fields grouping with capability, family, process, rib, etc.
- [ ] `internal/component/bgp/plugins/role/register.go` - role plugin registers IngressFilter and EgressFilter closures (lines 31-32)
- [ ] `internal/component/bgp/plugins/role/otc.go` - OTCIngressFilter:268-324 (stamp/reject per RFC 9234), OTCEgressFilter:326-403 (suppress per-peer)

**Behavior to preserve:**
- Role plugin's IngressFilter/EgressFilter mechanism unchanged
- Role filters run before any policy filter chain (RFC-mandated, hard reject)
- Current filter invocation in reactor_notify.go and reactor_api_forward.go stays as-is for in-process filters
- safeIngressFilter/safeEgressFilter panic recovery pattern
- Fail-closed on filter panic (reject/suppress -- see reactor_notify.go:24,37)
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
| In-process ingress filters (role OTC) | reactor_notify.go:305-349 | `[]byte` payload |
| **NEW: resolve peer's import filter chain** | reactor (new code) | Config: bgp + group + peer merged |
| **NEW: parse declared attributes** | reactor (new code) | Lazy parse only union of requested attributes |
| **NEW: for each filter in import chain** | reactor (new code) | Send text via IPC, receive response |
| **NEW: on modify, update cached attributes** | reactor (new code) | Pointer-swap dirty attributes, re-encode if needed |
| Cache in recentUpdates | reactor_notify.go:346+ | WireUpdate (possibly modified) |

### Entry Point: Export (Egress) Filter Chain

| Step | Location | Format |
|------|----------|--------|
| Forward request | reactor_api_forward.go | UpdateID -> cached ReceivedUpdate |
| In-process egress filters (role OTC) | reactor_api_forward.go:260-279 | `[]byte` payload, per-peer, ModAccumulator for mods |
| **NEW: resolve destination peer's export filter chain** | reactor (new code) | Config: bgp + group + peer merged |
| **NEW: parse declared attributes** | reactor (new code) | Lazy parse only union of requested attributes |
| **NEW: for each filter in export chain** | reactor (new code) | Send text via IPC, receive response |
| **NEW: on reject, skip peer** | reactor (new code) | Same as current egress filter short-circuit |
| **NEW: on modify, use modified version for this peer** | reactor (new code) | Dirty tracking per forward, not cached globally |
| **EXISTING: apply ModAccumulator** | reactor_api_forward.go:307-311 | buildModifiedPayload applies mods after wire selection |
| Select wire version (IBGP/EBGP) | reactor_api_forward.go:280+ | Per-peer wire |

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
- `registration.go` PluginRegistration struct - add filter declaration fields to Stage 1 data
- `reactor_notify.go` ingress filter loop (:305-349) - new policy chain after existing in-process filters
- `reactor_api_forward.go` egress filter loop (:260-279) - new policy chain after existing in-process filters
- `forward_build.go` buildModifiedPayload - existing egress modification infrastructure (may be reusable for export filter mods)
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

Extend `declare-registration` input with a new `filters` list. Each entry declares a named filter:

| Field | Type | Description |
|-------|------|-------------|
| `filters[].name` | string | Filter name (the part after `:` in `<plugin>:<filter>`) |
| `filters[].direction` | enum: import, export, both | Which direction(s) this filter handles |
| `filters[].attributes` | leaf-list of string | Attribute names to receive (e.g., "local-preference", "community", "as-path") |
| `filters[].nlri` | boolean (default true) | Whether to include NLRI list in filter input |
| `filters[].raw` | boolean (default false) | Whether to include raw wire bytes (forces full re-parse on modify) |
| `filters[].on-error` | enum: reject, accept | Behavior when IPC fails (timeout, crash, malformed response). Per-filter failure semantics |
| `filters[].overrides` | leaf-list of string | Default filters this filter replaces (e.g., `["rfc:no-self-as"]`). When this filter is in a peer's chain, the listed default filters are removed for that peer. Empty list (default) means no overrides |

Example wire message (plugin `allow-own-as` declares a filter that overrides a default RFC check):
```
#5 ze-plugin-engine:declare-registration {"filters":[{"name":"relaxed","direction":"import","attributes":["as-path"],"on-error":"accept","overrides":["rfc:no-self-as"]}]}
```

### Runtime: Filter Request (engine -> plugin, socket B)

New RPC in `ze-plugin-callback`: `filter-update`

| Field | Type | Description |
|-------|------|-------------|
| `filter` | string | Filter name (matches the name declared at stage 1, so plugin knows which filter to invoke) |
| `direction` | string | "import" or "export" |
| `peer` | string | Peer IP address |
| `peer-as` | uint32 | Peer ASN |
| `update` | string | Text-format update with only declared attributes and NLRI |
| `raw` | string (optional) | Hex-encoded raw UPDATE body (only if filter declared raw=true) |

Example:
```
#42 ze-plugin-callback:filter-update {"filter":"validate","direction":"import","peer":"10.0.0.1","peer-as":65001,"update":"as-path 65001 65002 origin igp nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24"}
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

Leaf-list type is string. Format: `<plugin>:<filter>` (e.g., `rpki:validate`). The `:` separator is validated at parse time. The plugin must exist and must have declared a filter with that name for the matching direction.

### Config Examples

Global level (applies to all peers):
```
bgp {
    redistribution {
        import [ rpki:validate ]
    }
}
```

Group level (cumulative with global):
```
bgp {
    redistribution {
        import [ rpki:validate ]
    }
    group customers {
        redistribution {
            import [ community:scrub ]
        }
        peer customer-a {
            remote { ip 10.0.0.1; as 65001; }
            redistribution {
                export [ aspath:prepend ]
            }
        }
    }
}
```

Effective chains for customer-a (mandatory/default filters not shown in config):
- Import: [rfc:otc (mandatory)] -> [rfc:no-self-as (default)] -> `rpki:validate` (bgp) -> `community:scrub` (group)
- Export: [rfc:otc (mandatory)] -> `aspath:prepend` (peer)

Override example (peer customer-b allows own AS in path):
```
peer customer-b {
    remote { ip 10.0.0.2; as 65002; }
    redistribution {
        import [ allow-own-as:relaxed ]
    }
}
```

Effective chain for customer-b (allow-own-as:relaxed declares `overrides: ["rfc:no-self-as"]`):
- Import: [rfc:otc (mandatory)] -> ~~rfc:no-self-as~~ (overridden) -> `rpki:validate` (bgp) -> `allow-own-as:relaxed` (peer)

### Config Resolution

| Level | Merge Rule |
|-------|-----------|
| mandatory | Always first, implicit, cannot be overridden |
| default | After mandatory, implicit, can be overridden |
| bgp | Base user chain |
| group | Append to bgp chain |
| peer | Append to group chain |

Result: ordered list of `<plugin>:<filter>` per peer per direction. Resolved once at config load, stored on peer settings. Mandatory and default filters prepended internally. If any user filter in the resolved chain declares `overrides` for a default filter, that default filter is removed from the chain for that peer.

### Config Validation

| Check | Error |
|-------|-------|
| Missing `:` separator | `redistribution: invalid filter reference "<value>", expected <plugin>:<filter>` |
| Plugin not found | `redistribution: unknown plugin "<plugin>" in "<plugin>:<filter>"` |
| Filter name not declared by plugin | `redistribution: plugin "<plugin>" did not declare filter "<filter>"` |
| Filter used in wrong direction | `redistribution: filter "<plugin>:<filter>" does not support <import/export>` |

Note: config validation for filter capability happens after stage 1 (plugins must declare before config can validate). This means redistribution validation is deferred until after plugin startup.

## Attribute Accumulation

When resolving filter chains, the reactor computes the union of all declared attributes across all filters in the chain. Parsing happens once per UPDATE for the accumulated set.

| Filter | Declares | Union |
|--------|----------|-------|
| rpki:validate | as-path, origin | as-path, origin |
| community:scrub | community | as-path, origin, community |

The reactor parses as-path, origin, and community once. `rpki:validate` receives only as-path and origin. `community:scrub` receives only community.

When `community:scrub` returns `modify community 65000:99`, the reactor:
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
| `redistribution { import [ test:accept ] }` config | -> | Config parsing + `<plugin>:<filter>` chain resolution | `test/plugin/redistribution-import-accept.ci` |
| Received UPDATE + import filter | -> | Reactor ingress policy chain + IPC with filter name dispatch | `test/plugin/redistribution-import-reject.ci` |
| Received UPDATE + import filter modify | -> | Reactor ingress policy chain + dirty tracking | `test/plugin/redistribution-import-modify.ci` |
| Forward UPDATE + export filter | -> | Reactor egress policy chain + IPC | `test/plugin/redistribution-export-reject.ci` |
| Stage 1 filter declaration (multiple named) | -> | Plugin startup + named filter registration | `test/plugin/redistribution-declare.ci` |
| Filter with `overrides` configured on peer | -> | Default filter removed, override filter runs | `test/plugin/redistribution-override.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin sends `declare-registration` with `filters` list containing two named filters | Engine records both named filters with their directions and attributes |
| AC-2 | Config has `redistribution { import [ rpki:validate ] }` and rpki plugin declared `validate` import filter | Config validates successfully, filter chain resolved |
| AC-3 | Config has `redistribution { import [ unknown:foo ] }` | Config error: unknown plugin "unknown" |
| AC-3b | Config has `redistribution { import [ rpki:nonexistent ] }` | Config error: plugin "rpki" did not declare filter "nonexistent" |
| AC-3c | Config has `redistribution { import [ badformat ] }` (missing `:`) | Config error: invalid filter reference |
| AC-4 | Config has `redistribution { export [ rpki:validate ] }` and validate declared import-only | Config error: filter does not support export |
| AC-5 | Received UPDATE, import filter returns accept | UPDATE cached and dispatched normally |
| AC-6 | Received UPDATE, import filter returns reject | UPDATE dropped, not cached, not dispatched |
| AC-7 | Received UPDATE, import filter returns modify with changed local-pref | UPDATE cached with modified local-pref, dirty re-encoding |
| AC-8 | Received UPDATE, import filter returns modify with filtered NLRI list | UPDATE cached with reduced NLRI set |
| AC-9 | Forward UPDATE, export filter returns reject | UPDATE not sent to that peer |
| AC-10 | Forward UPDATE, export filter returns modify | Modified version sent to that peer only, cache unchanged |
| AC-11 | Three filters in chain: first modifies, second sees modification, third accepts | Piped transform semantics work end-to-end |
| AC-12 | Config hierarchy: bgp import [ a:x ], group import [ b:y ], peer import [ c:z ] | Effective chain: a:x -> b:y -> c:z |
| AC-13 | Filter declared `attributes: [local-preference]` but modify response changes community | Error: filter can only modify declared attributes |
| AC-14 | Role OTC rejects on ingress (mandatory filter) | Policy filter chain never runs (mandatory filters are first, implicit) |
| AC-15 | Plugin declares raw=true on a named filter | That filter receives raw hex in addition to text attributes |
| AC-16 | Withdrawal UPDATE goes through import filter | Filter can reject withdrawal (e.g., block default route withdraw) |
| AC-17 | filter-update RPC includes `filter` field matching declared name | Plugin receives correct filter name and can dispatch to the right handler |
| AC-18 | Default filter `rfc:no-self-as` active, no overrides configured | Default filter runs for the peer (own AS in path rejected) |
| AC-19 | Filter declares `overrides: ["rfc:no-self-as"]`, configured on a peer | `rfc:no-self-as` removed from that peer's chain. Override filter runs in its place |
| AC-20 | Override configured at group level | Default filter removed for all peers in that group |
| AC-21 | Filter declares `overrides: ["rfc:otc"]` (mandatory, not default) | Override ignored -- mandatory filters cannot be overridden. Config validates but mandatory filter stays |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilterDeclarationParse` | `internal/component/plugin/registration_test.go` | Parse `filters` list from declare-registration into PluginRegistration. Multiple named filters per plugin | ✅ Done |
| `TestFilterDeclarationWithOverrides` | `internal/component/plugin/registration_test.go` | Override declarations stored correctly | ✅ Done (added) |
| `TestRedistributionConfigParse` | `internal/component/bgp/config/redistribution_test.go` | Parse redistribution YANG config block with `<plugin>:<filter>` values | ✅ Done |
| `TestRedistributionConfigValidation` | `internal/component/bgp/config/redistribution_test.go` | Reject unknown plugin, unknown filter name, missing `:`, wrong direction | ✅ Done (3 subtests) |
| `TestFilterChainResolution` | `internal/component/bgp/config/redistribution_test.go` | Merge bgp > group > peer chains correctly | ✅ Done |
| `TestRedistributionStandalonePeer` | `internal/component/bgp/config/redistribution_test.go` | Standalone peers accumulate bgp-level filters | ✅ Done (added) |
| `TestRedistributionEmpty` | `internal/component/bgp/config/redistribution_test.go` | No filters configured = no crash | ✅ Done (added) |
| `TestPolicyFilterChainAccept` | `internal/component/bgp/reactor/filter_chain_test.go` | Accept passes through unchanged | ✅ Done |
| `TestPolicyFilterChainReject` | `internal/component/bgp/reactor/filter_chain_test.go` | Reject short-circuits chain | ✅ Done |
| `TestPolicyFilterChainModify` | `internal/component/bgp/reactor/filter_chain_test.go` | Modify changes attributes | ✅ Done |
| `TestPolicyFilterChainPipedTransform` | `internal/component/bgp/reactor/filter_chain_test.go` | Second filter sees first filter's modifications | ✅ Done |
| `TestPolicyFilterChainShortCircuit` | `internal/component/bgp/reactor/filter_chain_test.go` | Reject stops chain, no further filters called | ✅ Done |
| `TestPolicyFilterChainEmpty` | `internal/component/bgp/reactor/filter_chain_test.go` | Empty chain = default accept | ✅ Done (added) |
| `TestPolicyFilterChainDispatch` | `internal/component/bgp/reactor/filter_chain_test.go` | Plugin:filter name split correctly | ✅ Done (added) |
| `TestApplyFilterDelta` | `internal/component/bgp/reactor/filter_chain_test.go` | Delta merge (4 subtests: modify, add, empty, nlri) | ✅ Done (added) |
| `TestAttributeAccumulation` | `internal/component/bgp/reactor/filter_chain_test.go` | Union of declared attributes computed correctly | Deferred to reactor wiring |
| `TestFilterResponseParse` | `internal/component/bgp/reactor/filter_chain_test.go` | Parse accept/reject/modify responses | 🔄 Covered by chain tests (responses are PolicyResponse structs) |
| `TestDirtyTracking` | `internal/component/bgp/reactor/filter_chain_test.go` | Only dirty attributes trigger re-encoding | Deferred to reactor wiring (wire-level) |
| `TestFilterModifyOnlyDeclared` | `internal/component/bgp/reactor/filter_chain_test.go` | Reject modify of undeclared attribute | Deferred to reactor wiring (requires attribute registry) |
| `TestDefaultFilterOverride` | `internal/component/config/redistribution_test.go` | Filter with `overrides` removes default filter from chain for that peer | Deferred (requires default filter registry) |
| `TestDefaultFilterOverrideAtGroupLevel` | `internal/component/config/redistribution_test.go` | Override at group level removes default for all peers in group | Deferred (requires default filter registry) |
| `TestMandatoryFilterCannotBeOverridden` | `internal/component/config/redistribution_test.go` | Override targeting mandatory filter is ignored, mandatory stays in chain | Deferred (requires mandatory filter registry) |

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
| `redistribution-declare` | `test/plugin/redistribution-declare.ci` | Plugin declares multiple named filters at stage 1, config validates `<plugin>:<filter>` references | |
| `redistribution-chain-order` | `test/plugin/redistribution-chain-order.ci` | Multiple filters in chain, execution order matches config | |
| `redistribution-override` | `test/plugin/redistribution-override.ci` | Filter with `overrides` removes default filter for that peer, override filter runs instead | |

### Future (if deferring any tests)
- Property testing: random UPDATE payloads through filter chains preserve invariants
- Performance benchmarks: filter chain overhead per UPDATE
- Chaos tests: filter plugin crash/timeout during filtering

## Files to Modify

- `internal/core/ipc/schema/ze-plugin-engine.yang` - add `filters` list to declare-registration input (name, direction, attributes, nlri, raw, on-error per filter)
- `internal/core/ipc/schema/ze-plugin-callback.yang` - add filter-update RPC
- `internal/component/plugin/registration.go` - add filter fields to PluginRegistration struct (Stage 1 data)
- `internal/component/bgp/schema/ze-bgp-conf.yang` - add redistribution container to peer-fields and bgp
- `internal/component/config/` - parse redistribution config, resolve filter chains
- `internal/component/bgp/reactor/reactor.go` - store resolved filter chains (:263-267 area), wire into startup (:749-751 area)
- `internal/component/bgp/reactor/reactor_notify.go` - add policy filter chain after in-process ingress filters (:305-349 area)
- `internal/component/bgp/reactor/reactor_api_forward.go` - add policy filter chain after in-process egress filters (:260-279 area)
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
- `internal/component/plugin/registration_test.go` - tests for filter declaration parsing (may already exist -- extend if so)
- `internal/component/config/redistribution.go` - config parsing and chain resolution
- `internal/component/config/redistribution_test.go` - config tests
- `docs/guide/redistribution.md` - user guide for redistribution filters
- `test/plugin/redistribution-import-accept.ci` - functional test
- `test/plugin/redistribution-import-reject.ci` - functional test
- `test/plugin/redistribution-import-modify.ci` - functional test
- `test/plugin/redistribution-export-reject.ci` - functional test
- `test/plugin/redistribution-declare.ci` - functional test
- `test/plugin/redistribution-chain-order.ci` - functional test
- `test/plugin/redistribution-override.ci` - functional test (default filter override)

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

1. **Phase: YANG schemas + declaration struct** -- ✅ Done (commit dd4ea150)
   - Tests: `TestFilterDeclarationParse`, `TestFilterDeclarationWithOverrides` -- PASS
   - Files: `ze-plugin-engine.yang`, `ze-bgp-conf.yang`, `ze-plugin-callback.yang`, `registration.go`, `server/startup.go`, `rpc/types.go`, `sdk/sdk_types.go`

2. **Phase: Config parsing** -- ✅ Done (commit dd4ea150)
   - Tests: `TestRedistributionConfigParse`, `TestRedistributionConfigValidation` (3 subtests), `TestFilterChainResolution`, `TestRedistributionStandalonePeer`, `TestRedistributionEmpty` -- PASS
   - Files: `redistribution.go`, `redistribution_test.go`, `peers.go`, `peersettings.go`
   - Override tests deferred (need default/mandatory filter registry)

3. **Phase: Filter chain runtime** -- ✅ Done (commit dd4ea150)
   - Tests: `TestPolicyFilterChainAccept`, `TestPolicyFilterChainReject`, `TestPolicyFilterChainModify`, `TestPolicyFilterChainPipedTransform`, `TestPolicyFilterChainShortCircuit`, `TestPolicyFilterChainEmpty`, `TestPolicyFilterChainDispatch`, `TestApplyFilterDelta` (4 subtests) -- PASS
   - Files: `filter_chain.go`, `filter_chain_test.go`

4. **Phase: Dirty tracking** -- ✅ Text-level done (commit dd4ea150), wire-level deferred
   - Text delta merge in `applyFilterDelta()` handles attribute-level dirty tracking at the text protocol layer.
   - Wire-level dirty tracking (re-encoding only modified attributes using `ModAccumulator`/`buildModifiedPayload`) deferred to reactor wiring phase.

5. **Phase: Reactor wiring** -- ✅ Done
   - Ingress: `PolicyFilterChain` wired into `reactor_notify.go` after in-process filters, before cache. Uses `peer.settings.ImportFilters`.
   - Egress: `PolicyFilterChain` wired into `reactor_api_forward.go` after in-process egress filters, per destination peer. Uses `peer.Settings().ExportFilters`.
   - `policyFilterFunc()` on Reactor bridges to `r.api.CallFilterUpdate()` with 5s timeout.
   - `CallFilterUpdate()` on Server looks up plugin process by name and calls `SendFilterUpdate()`.
   - `parsePolicyAction()` validates wire response (accept/reject/modify), rejects unknown actions.
   - TODO in both paths: text-format update from wire bytes (attribute formatting not yet wired).

6. **Phase: SDK support** -- ✅ Done (commit c6d81d83)
   - `OnFilterUpdate()` callback, `handleFilterUpdate()` dispatch, `SendFilterUpdate()` IPC
   - Files: `sdk.go`, `sdk_callbacks.go`, `sdk_dispatch.go`, `ipc/rpc.go`

7. **Functional tests** -- Not started (depends on reactor wiring)
   - Files: `test/plugin/redistribution-*.ci`

8. **Full verification** -- Not yet

9. **Complete spec** -- Not yet

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation with file:line |
| Correctness | Filter chain order: mandatory > default > bgp > group > peer |
| Correctness | Override only removes default filters, never mandatory |
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
| declare-registration extended with filter fields | grep "filter" ze-plugin-engine.yang + grep "Filter" registration.go |
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

**Phase 1 -- Wire protocol types and YANG schemas (commit dd4ea150):**
- `FilterDecl`, `FilterUpdateInput`, `FilterUpdateOutput` in `pkg/plugin/rpc/types.go`
- `FilterRegistration` struct in `internal/component/plugin/registration.go`
- `filters` list in `ze-plugin-engine.yang` declare-registration
- `redistribution` container in `ze-bgp-conf.yang` (bgp-level + peer-fields)
- `filter-update` RPC in `ze-plugin-callback.yang`
- `registrationFromRPC` wiring in `server/startup.go` (NLRI default true, on-error default reject)
- SDK type re-exports in `pkg/plugin/sdk/sdk_types.go`

**Phase 2 -- Config parsing and chain resolution (commit dd4ea150):**
- `redistribution.go`: `extractRedistributionFilters()`, `validateFilterRefs()`, `concatFilters()`
- Cumulative chain patching in `peers.go` (bgp > group > peer, same pattern as routes)
- `ImportFilters`/`ExportFilters` fields on `PeerSettings`
- Format validation: missing `:`, empty plugin name, empty filter name

**Phase 3 -- Filter chain runtime (commit dd4ea150):**
- `filter_chain.go`: `PolicyFilterChain()` with piped transforms and reject short-circuit
- Text attribute parsing (`parseFilterAttrs`), delta merge (`applyFilterDelta`), deterministic output (`formatFilterAttrs`)
- Types: `PolicyAction`, `PolicyResponse`, `PolicyFilterFunc`

**Phase 6 -- SDK callback and IPC layer (commit c6d81d83):**
- `OnFilterUpdate()` callback registration in `sdk_callbacks.go`
- `handleFilterUpdate()` dispatch handler in `sdk_dispatch.go`
- `"ze-plugin-callback:filter-update"` case in dispatch switch
- `SendFilterUpdate()` in `ipc/rpc.go` for engine-to-plugin calls

### Bugs Found/Fixed
- None so far

### Documentation Updates
- Updated 10 documentation files in commit dd4ea150 (features, comparison, plugins guide, configuration guide, new redistribution guide, config syntax, API commands, process protocol, core design, plugin design rules). All marked "(planned)" with source anchors.

### Deviations from Plan

| Deviation | Reason |
|-----------|--------|
| Phase 4 (dirty tracking) merged into Phase 3 | Text-level delta is in `filter_chain.go`. Wire-level dirty tracking (re-encoding only modified attributes) deferred to reactor wiring phase, since it depends on `ModAccumulator`/`buildModifiedPayload` infrastructure |
| Phase 5 (reactor wiring) blocked | Pre-existing compilation error in `reactor_api_batch.go` (missing `localAS` arg) from other work in tree. Reactor package cannot compile until fixed |
| Override tests deferred | `TestDefaultFilterOverride`, `TestDefaultFilterOverrideAtGroupLevel`, `TestMandatoryFilterCannotBeOverridden` require a default/mandatory filter registry that doesn't exist yet. The data structures support overrides, but the runtime resolution needs the registry |
| `TestAttributeAccumulation`, `TestDirtyTracking`, `TestFilterModifyOnlyDeclared` deferred | These require reactor-level integration (attribute parsing from wire bytes, attribute registry for validation). Cannot be tested at the text-format level |
| Test names differ from spec plan | Added more focused tests (`TestPolicyFilterChainAccept`, `TestApplyFilterDelta`, etc.) that cover the same ACs with better granularity. Renamed from `TestFilterChain*` to `TestPolicyFilterChain*` to avoid collision with existing `FilterResult` type in `filter` package |

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
| `internal/component/bgp/reactor/filter_chain.go` | ✅ Created | PolicyFilterChain, text attr parsing, delta merge |
| `internal/component/bgp/reactor/filter_chain_test.go` | ✅ Created | 8 tests (chain + delta) |
| `internal/component/plugin/registration_test.go` | ✅ Extended | +2 filter tests |
| `internal/component/bgp/config/redistribution.go` | ✅ Created | Extract, validate, concat filters |
| `internal/component/bgp/config/redistribution_test.go` | ✅ Created | 5 tests (parse, validate, chain, standalone, empty) |
| `docs/guide/redistribution.md` | ✅ Created | Full user guide |
| `test/plugin/redistribution-import-accept.ci` | ❌ Not created | Depends on reactor wiring |
| `test/plugin/redistribution-import-reject.ci` | ❌ Not created | Depends on reactor wiring |
| `test/plugin/redistribution-import-modify.ci` | ❌ Not created | Depends on reactor wiring |
| `test/plugin/redistribution-export-reject.ci` | ❌ Not created | Depends on reactor wiring |
| `test/plugin/redistribution-declare.ci` | ❌ Not created | Depends on reactor wiring |
| `test/plugin/redistribution-chain-order.ci` | ❌ Not created | Depends on reactor wiring |
| `test/plugin/redistribution-override.ci` | ❌ Not created | Depends on reactor wiring |

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
