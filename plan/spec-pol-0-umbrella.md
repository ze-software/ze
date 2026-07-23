# Spec: Structured Policy Language Umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-05-24 |

Reconciliation needed (2026-07-22 plan review): all five child spec files
(pol-1..pol-5) are gone from `plan/`; pol-2 (actions), pol-3 (validation) and
pol-4 (explain) closed with learned 782/809/814, but pol-1 (named-sets) and
pol-5 (rs-defaults) have NO learned summary and no obvious landing (no
`bgp/sets` package under `internal/`). Before treating this umbrella as an
open `ready` roadmap: mark pol-2/3/4 done and resolve whether pol-1/pol-5
landed elsewhere or were dropped -- if dropped, that is an unrecorded scope
reduction to surface to Thomas.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/541-policy-framework.md` - original policy framework decisions
4. `plan/learned/572-cmd-8-policy-show.md` - policy introspection commands
5. `plan/learned/593-cmd-2-session-policy.md` - session policy knobs
6. `internal/component/bgp/reactor/filter_chain.go` - chain execution
7. `internal/component/bgp/reactor/filter_delta.go` - wire-level delta tracking

## Task

Mature Ze's BGP route policy from individual filter plugins into a structured,
operator-friendly policy language. The current system has the right foundations
(fail-closed IPC, declared attribute checks, piped chain execution, delta
application) but lacks the vocabulary operators need for daily work.

**Design philosophy: macros, not a programming language.**

Junos policy-options is powerful but complex. Most engineers do not need a
general-purpose language inside their NOS config. They need simple keywords that
express common operator intent: remove private ASNs, add a community, increment
MED by 50, reject bogons. Ze provides these as single-purpose filter plugins
composed via the existing import/export chain. The complexity lives in Ze's
implementation, not in the operator's config.

**Operations match attribute semantics:**

| Attribute type | Examples | Operations |
|---------------|----------|------------|
| Integer | local-preference, med, aigp | set, increment, decrement |
| List | community, large-community, extended-community | add, remove, set (replace all) |
| Path | as-path | prepend (exists), remove-private |
| Prefix | nlri | match via prefix-set (accept/reject) |

**Composition over conditionals:** match filters followed by action filters in the
chain. "If community 65000:100 then set local-pref 200" is two chain entries:
`community-match:HAS-CUSTOMER` then `modify:LP-200`. No from/then blocks needed.

The work splits into five child specs:

| Spec | Scope | Depends |
|------|-------|---------|
| `spec-pol-1-named-sets.md` | Prefix-set, community-set, AS-path-set definitions in `bgp/sets` | - |
| `spec-pol-2-actions.md` | Action macros: inc/dec, community add/remove, remove-private-as | - |
| `spec-pol-3-validation.md` | Compile-time validation of all policy references at commit | pol-1, pol-2 |
| `spec-pol-4-explain.md` | Policy trace: dry-run test, per-filter explain output | pol-2 |
| `spec-pol-5-rs-defaults.md` | Route-server default chains and built-in sets | pol-1, pol-2 |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component registration, policy filter chain
  -> Constraint: filters piped via PolicyFilterChain, text format, delta modify
- [ ] `ai/patterns/registration.md` - plugin init/registry/blank-import pattern
  -> Constraint: new filter types register via registry.Registration with FilterTypes field
- [ ] `ai/rules/plugin-design.md` - plugin isolation, cross-boundary value types
  -> Constraint: no cross-boundary pointers, ModAccumulator for wire-level ops
- [ ] `ai/rules/config-design.md` - YANG augment vs grouping, listener pattern
  -> Constraint: filter plugins augment bgp/policy; new sets container needs augment points

### Learned Summaries
- [ ] `plan/learned/541-policy-framework.md` - original policy framework decisions
  -> Decision: specialized filter plugins over generic policy language
  -> Decision: three config concerns separated: policy (definitions), filter (chains), redistribute (sources)
  -> Decision: each filter type augments bgp/policy with ze:filter-marked list
  -> Decision: inactive: prefix for deactivation
- [ ] `plan/learned/572-cmd-8-policy-show.md` - policy introspection
  -> Decision: show policy list and show policy chain implemented; show policy detail and show policy test deferred
- [ ] `plan/learned/593-cmd-2-session-policy.md` - session policy knobs
  -> Decision: AttrModSuppress action for attribute removal
  -> Decision: send-community as leaf-list enum for granular control
- [ ] `plan/learned/551-filter-non-cidr-families.md` - non-CIDR NLRI handling
  -> Constraint: non-CIDR families get marker-only blocks; raw=true for per-NLRI decisions

**Key insights:**
- The original policy framework explicitly chose "specialized filter plugins over a generic policy language" (541). This spec extends that decision, not reverses it.
- Six filter plugins already exist: prefix-list, as-path-list, community-match, modify, community tag/strip, loop-detection.
- The modify filter is unconditional by design. Conditional modification = match filter + modify filter composed in chain.
- show policy test (dry-run) was explicitly deferred in 572 as future work.
- MP_REACH rewrite is explicitly outside scope (filter_delta.go:33-42). Filters needing per-NLRI decisions on non-CIDR families declare raw=true.
- ModAccumulator supports five ops: Set, Add, Remove, Prepend, Suppress. Actions spec (pol-2) builds on these existing constants.
- AttrModAdd and AttrModRemove constants exist (registry_bgp_filter.go:93-94) and are referenced by filter_community handler code, but only via Set paths today. The infrastructure for list-level add/remove is partially wired.

## Current Behavior (MANDATORY)

**Source files read:**

### Filter Chain Engine
- [ ] `internal/component/bgp/reactor/filter_chain.go` (304 lines) - PolicyFilterChain pipes named filters, applyFilterDelta merges text deltas, policyFilterFunc wraps IPC with timeout/fail-closed, validateModifyDelta enforces AC-13
  -> Constraint: chain is linear pipe, reject short-circuits, default=accept at end
  -> Constraint: 5-second per-filter IPC timeout (policyFilterTimeout)
- [ ] `internal/component/bgp/reactor/filter_format.go` (261 lines) - AppendUpdateForFilter renders UPDATE to text, attrNameToCode maps 13 attribute names
  -> Constraint: zero-alloc append-based formatting
  -> Constraint: non-CIDR families emit marker-only blocks (no prefix enumeration)
- [ ] `internal/component/bgp/reactor/filter_delta.go` (523 lines) - textDeltaToModOps converts text deltas to wire AttrModSet ops, encodeAttrValue handles 13 attribute types, ExtractASPathPrependOps separate because needs localAS
  -> Constraint: AS_PATH and NLRI explicitly skipped in textDeltaToModOps (lines 198-201)
  -> Constraint: AS-path prepend handled separately via ExtractASPathPrependOps
- [ ] `internal/component/bgp/reactor/forward_build.go` (546 lines) - buildModifiedPayload applies ModAccumulator ops to wire bytes, single-pass output, NLRI override for prefix-list modify
  -> Constraint: copy-on-modify via per-peer pool buffer

### Filter Invocation
- [ ] `internal/component/bgp/reactor/reactor_notify.go` (536 lines) - ingress path: in-process mandatory filters first (OTC, loop-detection), then external import policy chain via PolicyFilterChain
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - egress path: in-process mandatory filters first, then external export policy chain, ModAccumulator accumulates wire ops per destination peer

### Existing Filter Plugins
- [ ] `internal/component/bgp/plugins/filter_prefix/` (1193 lines total) - prefix-list filter with ge/le ranges, per-prefix partition (modify for partial accept)
  -> Constraint: inline entry definitions, no external set reference
- [ ] `internal/component/bgp/plugins/filter_aspath/` - AS-path regex filter, first match wins, accept/reject only (no modify)
  -> Constraint: regex via Go RE2 (linear time, ReDoS-safe)
  -> Constraint: inline regex entries, no external set reference
- [ ] `internal/component/bgp/plugins/filter_community_match/` - community presence match filter, standard/large/extended, first match wins
  -> Constraint: inline community entries, no external set reference
- [ ] `internal/component/bgp/plugins/filter_modify/` - unconditional attribute setter: local-preference, med, origin, next-hop, as-path-prepend
  -> Constraint: no inc/dec, no community manipulation, no conditional logic
- [ ] `internal/component/bgp/plugins/filter_community/` (1721 lines total) - community tag/strip at ingress/egress, cumulative config across inheritance levels
  -> Constraint: separate from policy chain; uses in-process IngressFilter/EgressFilter closures, not text-format RPC
  -> Constraint: tag/strip operates on entire attribute (bulk), not individual values in chain context
- [ ] `internal/component/bgp/reactor/filter/loop.go` - loop detection, in-process, protocol stage, reads PeerFilterInfo for allow-own-as and cluster-id

### Plugin Registry
- [ ] `internal/component/plugin/registry/registry_bgp_filter.go` (164 lines) - PeerFilterInfo, IngressFilterFunc, EgressFilterFunc, ModAccumulator with Op/Ops/SetWithdraw/IsWithdraw/Reset/HasMods methods, AttrOp struct (code + action + wire bytes), five mod action constants (Set=0, Add=1, Remove=2, Prepend=3, Suppress=4), three filter stage constants (Protocol=0, Policy=100, Annotation=200)
  -> Constraint: Add and Remove constants defined but only used by filter_community handler code (communityAttrModHandler, largeCommunityAttrModHandler, extCommunityAttrModHandler) via Set action today. The Add/Remove action paths in handlers need implementation for community add/remove in policy chain.

### Config and YANG
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - bgp/policy container (augmented by filter plugins), bgp/filter container (import/export leaf-lists), per-peer filter containers
  -> Constraint: policy container description: "Named filter definitions for the route policy framework. Each filter type is a list added by its plugin via augment."
- [ ] `internal/component/bgp/config/filter_registry.go` - BuildFilterRegistry from all ze:filter-marked lists in policy container, ValidateFilterNames for plain names at parse time
  -> Constraint: colon names (plugin:filter) validated at runtime since plugins register after config parse
- [ ] `internal/component/bgp/config/redistribution.go` - extractFilterChain, concatFilters (bgp+group+peer), canonicalizeFilterRefs resolves user forms to plugin:filter canonical form
  -> Constraint: three referencing forms: plain name (CUSTOMERS), type form (prefix-list:CUSTOMERS), plugin form (bgp-filter-prefix:CUSTOMERS)

### Filter Plugin YANG Pattern

All filter plugins follow the same YANG augment pattern:

| Plugin | YANG module | Augments | List name | ze:filter | Key | Entry structure |
|--------|------------|----------|-----------|-----------|-----|----------------|
| filter_prefix | ze-filter-prefix | /bgp:bgp/bgp:policy | prefix-list | yes | name | entry list: prefix + ge + le + action |
| filter_aspath | ze-filter-aspath | /bgp:bgp/bgp:policy | as-path-list | yes | name | entry list: regex + action |
| filter_community_match | ze-filter-community-match | /bgp:bgp/bgp:policy | community-match | yes | name | entry list: community + type + action |
| filter_modify | ze-filter-modify | /bgp:bgp/bgp:policy | modify | yes | name | set container: local-pref, med, origin, next-hop, as-path-prepend |
| loop-detection | ze-loop-detection | /bgp:bgp/bgp:policy | loop-detection | yes | name | allow-own-as + cluster-id |

### Filter Registration Pattern

All filter plugins register identically via init() in register.go:

| Field | Purpose | Example (filter_prefix) |
|-------|---------|------------------------|
| Name | Plugin registry name | bgp-filter-prefix |
| FilterTypes | YANG list name(s) this plugin owns | prefix-list |
| YANG | Embedded YANG schema | fpschema.ZeFilterPrefixYANG |
| ConfigRoots | Config sections to receive | bgp |
| Dependencies | Plugins that must start first | bgp |
| RunEngine | In-process handler function | RunFilterPrefix |

### Policy Introspection (existing)
- `show policy list` - lists registered filter types with plugin names
- `show policy chain peer X [import/export]` - shows effective filter chain after inheritance
- `show policy detail` - deferred (per-filter config query)
- `show policy test` - deferred (dry-run execution)

**Behavior to preserve:**
- Filter chain piped execution model (linear, short-circuit reject, default accept)
- Text-format wire protocol between engine and filter plugins
- Delta-only modify responses (filter returns only changed attributes)
- Three-layer config inheritance (bgp > group > peer) with cumulative filter chains
- inactive: prefix for deactivation
- In-process protocol-stage filters (loop-detection, OTC) separate from policy-stage
- Zero-alloc append-based text formatting
- Copy-on-modify via per-peer pool buffers
- AC-13 declared-attribute validation for modify deltas
- Existing filter plugin YANG schemas and config syntax unchanged
- Existing filter chain referencing forms (plain, type, plugin) unchanged

**Behavior to change:**
- Add new `bgp/sets` YANG container parallel to `bgp/policy` for named set definitions
- Extend existing filter plugins to optionally reference named sets instead of inline entries
- Add new action filter plugins (remove-private-as, as-path-length)
- Extend modify filter with inc/dec operations and community add/remove
- Add compile-time validation of policy references at config commit
- Add policy explain/trace for debugging
- Add route-server default chain presets

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG containers `bgp/sets` (new) and `bgp/policy` (existing) parsed at startup and reload
- Runtime: UPDATE bytes arrive on ingress or are forwarded on egress, passing through the filter chain

### Transformation Path

**Config path (startup/reload):**
1. YANG schema parsed: `bgp/sets` container holds named set definitions
2. YANG schema parsed: `bgp/policy` container holds filter definitions (may reference sets)
3. Config tree resolution: filter chains extracted per peer (bgp > group > peer inheritance)
4. Filter registry built: validates all filter names, canonicalizes references
5. **New (pol-3):** set references in filters validated against sets container
6. Peer settings frozen: ImportFilters/ExportFilters stored on PeerSettings

**Runtime path (per UPDATE, unchanged architecture):**
1. In-process protocol-stage filters run first (loop-detection, OTC)
2. PolicyFilterChain iterates import/export filter chain
3. Each filter receives text-format update, returns accept/reject/modify
4. Modify deltas merged via applyFilterDelta (text level)
5. textDeltaToModOps converts to wire-level AttrOp entries on ModAccumulator
6. buildModifiedPayload applies accumulated ops to produce per-peer wire bytes

**New: explain path (pol-4):**
1. Operator invokes `show policy test peer X import` with a route description
2. Engine constructs synthetic text-format update from route description
3. Chain executed in dry-run mode, collecting per-filter decisions
4. Results returned as structured JSON with per-filter accept/reject/modify and reason

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Filter plugins | YANG tree sections delivered via OnConfigure callback | [ ] |
| Config -> Set definitions | YANG tree sections in bgp/sets parsed into shared registry | [ ] |
| Filter plugin -> Named set | Plugin reads set from registry by name (value copy, no pointer) | [ ] |
| Engine -> Filter plugin (text) | PolicyFilterFunc RPC with text-format update string | [ ] |
| Engine -> Filter plugin (in-process) | IngressFilterFunc/EgressFilterFunc with wire bytes + meta | [ ] |
| Filter result -> Wire | ModAccumulator ops applied by buildModifiedPayload | [ ] |

### Integration Points
- `filter_chain.go:PolicyFilterChain` - unchanged, drives all filter execution
- `registry.Registration` - FilterTypes field for new filter plugins
- `filter_registry.go:BuildFilterRegistry` - extended to validate set references (pol-3)
- `redistribution.go:canonicalizeFilterRefs` - extended for new filter type names
- `registry_bgp_filter.go:ModAccumulator` - existing Add/Remove ops used by new action plugins
- `show_policy.go` - extended for explain output (pol-4)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design Decisions

### D-1: Macros over language

Ze's policy is a vocabulary of single-purpose operators, not a programming language.
Each operator does one thing. Composition happens in the filter chain. This is the
opposite of Junos policy-options, which provides from/then/term blocks that mix
matching and action in one policy-statement.

**Why:** Most network engineers do not need (or want) to learn a policy language.
They need `remove-private-as`, not a regex rewrite pipeline. The cognitive load
should be in choosing which operators to compose, not in learning syntax.

**Comparison to vendor approaches:**

| Aspect | Junos policy-options | FRR route-map | BIRD filter | Ze policy |
|--------|---------------------|---------------|-------------|-----------|
| Model | General-purpose language with from/then terms | Ordered match/set entries with sequence numbers | Turing-complete filter language | Single-purpose operators composed in chain |
| Conditionals | from { } then { } | match + set in same entry | if/else/case | Separate match filter + action filter in chain |
| Named sets | prefix-list, community, as-path-group | prefix-list, community-list, as-path-list | define filter/function | Named sets in bgp/sets |
| Reuse | policy-statement referenced from policy chain | route-map referenced by name | function call | Filter instance referenced by name in chain |
| Complexity | High (subroutines, boolean logic, regex) | Medium (sequence numbers, continue) | Very high (variables, loops, print) | Low (compose simple operators) |

### D-2: Named sets in separate bgp/sets container

Named set definitions (prefix-set, community-set, as-path-set) live in a new
`bgp/sets` container parallel to `bgp/policy`, not inside it.

**Why:** Sets are data definitions (what to match). Policy filters are behaviors
(what to do when matched). Separating them makes the config tree self-documenting:
`bgp/sets` answers "what are my prefix lists?", `bgp/policy` answers "what filters
are defined?", `bgp/filter` answers "what chain does this peer use?"

### D-3: Compose via chain, not single-filter conditionals

"If community X then set local-pref Y" is expressed as two chain entries, not a
single filter with from/then blocks.

**Why:** Single-purpose filters are simpler to understand, test, and debug
individually. The chain is the composition mechanism. Adding from/then blocks
would recreate Junos complexity inside each filter.

**Trade-off:** More chain entries for conditional actions. Acceptable because chain
entries are cheap (leaf-list values) and the config is still shorter than Junos.

### D-4: Operations match attribute semantics

Rather than a generic "set attribute to value" operation, Ze provides operations
that match what operators actually do with each attribute type:

| Attribute type | Why these operations |
|---------------|---------------------|
| Integer (local-pref, med) | Operators set absolute values, or adjust relative to received value (customer += 50) |
| List (communities) | Operators add/remove individual values, not replace the whole list |
| Path (AS-path) | Operators prepend their own AS (existing) or strip private ASNs |

### D-5: Built-in sets for common operator needs

Route-server operators (and others) need bogon prefix lists, private AS lists, and
similar well-known sets. Ze ships these as built-in named sets that are always
available without configuration. Distinguished by `ze:` prefix.

**Why:** Every RS operator builds the same bogon list. Shipping it eliminates a
common source of misconfiguration and keeps it maintained as RIR allocations change.

### D-6: Plugin SDK is the escape hatch for complex logic

The built-in macro filters cover daily operator needs. For complex conditional
logic that exceeds what chain composition can express (e.g., "if AS-path matches X
AND community contains Y, then set local-pref Z and add community W"), operators
write a plugin. The SDK already supports this:

1. Plugin declares filters via `Filters []FilterDecl` in Stage 1 registration
2. Engine calls `OnFilterUpdate` at runtime with text-format update + optional raw bytes
3. Plugin returns accept/reject/modify with delta
4. Users reference as `<plugin>:<filter>` in the chain

This is explicitly not a gap to fill with a built-in conditional language. The
plugin SDK is the right layer for arbitrary logic. Ze's job is to make the common
case trivial and the complex case possible.

### D-7: Explain output for debugging

Policy debugging in production is hard. "Why was this route rejected?" requires
mentally simulating the chain. Ze provides `show policy test` to dry-run a route
and `show policy explain` to trace decisions.

**Why:** BIRD has `show route ... filter ...` for this. FRR has `debug route-map`.
Junos has `test policy`. Operators expect it.

## Child Spec Breakdown

### spec-pol-1-named-sets

**Goal:** Define reusable prefix-set, community-set, and AS-path-set definitions
in a new `bgp/sets` container. Update existing filter plugins to optionally
reference named sets instead of (or in addition to) inline entries.

**YANG structure (new bgp/sets container):**

| Set type | List key | Entry fields | Referenced by |
|----------|----------|-------------|---------------|
| prefix-set | name | entry list: prefix (IPv4/IPv6 CIDR), ge, le | prefix-list filter |
| community-set | name | entry list: value (standard/large/extended format), type enum | community-match filter, community add/remove in modify |
| as-path-set | name | entry list: regex pattern | as-path-list filter |

**Plugin updates:**
- prefix-list: add optional `set` leaf referencing a prefix-set name; when present, entries come from the set
- as-path-list: add optional `set` leaf referencing an as-path-set name
- community-match: add optional `set` leaf referencing a community-set name

**Design constraint:** inline entries and set references are mutually exclusive per
filter instance. A prefix-list either has inline entries or references a set, not both.
This prevents confusing merge semantics.

### spec-pol-2-actions

**Goal:** Extend the action vocabulary with operators that match daily tasks.

**Extend existing modify filter (ze-filter-modify):**

| New leaf | Container | Attribute | Semantics |
|----------|-----------|-----------|-----------|
| increment | set | local-preference, med, aigp | Add N to current value (saturate at uint32 max) |
| decrement | set | local-preference, med, aigp | Subtract N from current value (floor at 0) |
| community-add | set | community | Append standard community values to route |
| community-remove | set | community | Remove specific standard community values from route |
| large-community-add | set | large-community | Append large community values to route |
| large-community-remove | set | large-community | Remove specific large community values from route |
| extended-community-add | set | extended-community | Append extended community values to route |
| extended-community-remove | set | extended-community | Remove specific extended community values from route |

**Inc/dec wire path:** plugin reads current value from filter text, computes arithmetic,
returns absolute value in modify delta. Engine sees only AttrModSet. No new wire op needed.

**Community add/remove wire path:** plugin returns modify delta. Engine emits AttrModAdd
or AttrModRemove on ModAccumulator. Existing community handler functions
(communityAttrModHandler, largeCommunityAttrModHandler, extCommunityAttrModHandler in
filter_community/handler.go) need Add/Remove branches alongside existing Set path.

**New filter plugin: remove-private-as**

| Field | Value |
|-------|-------|
| Plugin name | bgp-filter-remove-private-as |
| YANG list name | remove-private-as |
| FilterTypes registration | remove-private-as |
| Behavior | Strip ASNs in 64512-65534 and 4200000000-4294967294 from AS_PATH |
| Config leaves | name; replace-with (optional enum: peer-as replaces private ASNs with peer's ASN) |
| Chain position | Typically in export chain |
| Action returned | modify (rewritten AS_PATH) or accept (no private ASNs found) |

**New filter plugin: as-path-length**

| Field | Value |
|-------|-------|
| Plugin name | bgp-filter-aspath-length |
| YANG list name | as-path-length |
| FilterTypes registration | as-path-length |
| Behavior | Accept or reject based on AS_PATH hop count |
| Config leaves | name; max (uint16, reject paths longer than N); min (uint16, reject paths shorter than N) |
| Chain position | Typically in import chain |
| Action returned | accept or reject |

**Wire-level gap for remove-private-as:** textDeltaToModOps currently skips AS_PATH
(filter_delta.go:198) to avoid clobbering EBGP prepend. remove-private-as needs to
modify AS_PATH without conflicting with the EBGP prepend path. Options:
1. New AttrModReplace action that replaces AS_PATH contents while preserving EBGP prepend ordering
2. Plugin operates at wire level (raw=true) and rewrites AS_PATH directly
3. Engine applies AS_PATH modification via a dedicated code path (not textDeltaToModOps)

This is the key design question for pol-2 and should be resolved during its design phase.

### spec-pol-3-validation

**Goal:** Validate all policy references at config commit time, catching errors
before they reach runtime.

**Validations:**

| Check | Severity | When | Error example |
|-------|----------|------|---------------|
| Set existence | Error (reject commit) | Filter references a named set | "prefix-list CUSTOMERS references undefined prefix-set CUST-PREFIXES" |
| Filter existence | Error (reject commit) | Chain entry references unknown filter | Already implemented for plain names via ValidateFilterNames |
| Type compatibility | Error (reject commit) | Filter references wrong set type | "as-path-list MY-FILTER references prefix-set (expected as-path-set)" |
| Empty set | Warning (log) | Named set defined with no entries | "prefix-set CUSTOMERS is empty" |
| Unused set | Warning (log) | Named set not referenced by any filter | "community-set OLD-TAGS is not referenced" |

**Implementation:** extends existing filter_registry.go validation. Set existence
and type checks run during BuildFilterRegistry. Warnings emitted during config verify.

### spec-pol-4-explain

**Goal:** Let operators debug policy decisions without reading config or mentally
simulating the chain.

**Two commands:**

| Command | Input | Output |
|---------|-------|--------|
| show policy test peer X import prefix P [attributes] | Peer + prefix + optional attrs | Per-filter verdict with final outcome |
| show policy explain peer X import | None (uses trace buffer) | Last N routes with per-filter trace |

**show policy test:** construct synthetic text-format update from user input, execute
PolicyFilterChain in dry-run mode (no wire ops), collect per-filter decisions, return
structured JSON.

**show policy explain:** per-peer circular trace buffer (configurable depth, default 0 = off).
When enabled, PolicyFilterChain invocations record per-filter decisions. CLI queries
the buffer.

### spec-pol-5-rs-defaults

**Goal:** Ship sensible defaults for route-server deployments.

**Built-in named sets (ze: prefix, always available):**

| Set name | Contents | Source |
|----------|----------|--------|
| ze:bogon-prefixes-v4 | RFC 5737, RFC 1918, RFC 6598, RFC 6890 prefixes | Ze releases |
| ze:bogon-prefixes-v6 | RFC 3849, RFC 4291 mapped, documentation prefixes | Ze releases |
| ze:bogon-asns | 0, 23456, 64496-64511, 65535, 4294967295 | Ze releases |
| ze:private-asns | 64512-65534, 4200000000-4294967294 | RFC 6996 |

**Built-in filters (ze: prefix, always available):**

| Filter name | Type | Behavior |
|-------------|------|----------|
| ze:reject-bogon-prefixes | prefix-list referencing ze:bogon-prefixes-v4 + v6 | Reject bogon prefixes |
| ze:reject-bogon-asns | as-path-list matching ze:bogon-asns | Reject routes with bogon ASNs |
| ze:reject-long-aspath | as-path-length with max 50 | Reject excessively long paths |

**RS profile:** YANG container `bgp/rs-profile` that auto-populates global import
chain with safety filters when enabled.

## Execution Order

| Phase | Specs | Rationale |
|-------|-------|-----------|
| 1 | pol-1 (named sets), pol-2 (actions) | Independent, no cross-dependencies |
| 2 | pol-3 (validation) | Needs sets and new actions to exist |
| 3 | pol-4 (explain), pol-5 (rs-defaults) | Polish and operator experience |

Within Phase 1, pol-1 and pol-2 are independent and can be implemented in parallel.

## Open Questions

| # | Question | Options | Proposed |
|---|----------|---------|----------|
| 1 | Should built-in sets use a `ze:` prefix to distinguish from user sets? | Yes (clear separation) / No (simpler) | Yes |
| 2 | Should inc/dec saturate or error on overflow? | Saturate at 0/max / reject | Saturate |
| 3 | Should remove-private-as handle AS_SET segments? | Both / AS_SEQUENCE only | Both |
| 4 | Should explain trace buffer be per-peer or global? | Per-peer (precise, more memory) / global ring | Per-peer, default off |
| 5 | Should community add be idempotent? | Yes (no-op if present) / No (append) | Yes |
| 6 | How should remove-private-as modify AS_PATH given textDeltaToModOps skips it? | New AttrModReplace / raw=true / dedicated path | Resolve in pol-2 design |

## Related Deferrals

| Deferral | Source | Addressed by |
|----------|--------|-------------|
| bgp-filter-prefix plugin spec | spec-filter-community | pol-1 extends with set references |
| bgp-filter-irr plugin | spec-filter-community | Out of scope (separate spec, uses pol-1 sets) |
| AS-path prepend via modify: needs dedicated mechanism | spec-cmd-7-route-modify | pol-2 resolves |
| show policy detail | cmd-8 (learned/572) | pol-4 subsumes |
| show policy test (dry-run) | cmd-8 (learned/572) | pol-4 implements |

## Out of Scope

| Item | Why |
|------|-----|
| MP_REACH rewrite | Explicitly outside v1 (filter_delta.go:33-42) |
| General-purpose policy language (from/then terms) | Design decision D-1 |
| IRR-based filtering | Separate spec, consumes pol-1 sets |
| Flowspec-based filtering | Different mechanism (wire-encoded rules) |
| Inter-VRF route leaking policy | VRF spec scope |
| BGP well-known community actions (no-export, no-advertise) | Already RFC-mandated in-process |

## Competitive Gap Analysis

| Capability | BIRD | FRR | Junos | Ze today | Ze after pol |
|-----------|------|-----|-------|----------|-------------|
| Named prefix sets | inline define | ip prefix-list | prefix-list | inline in filter | bgp/sets/prefix-set |
| Named community sets | inline match | community-list | community | inline in filter | bgp/sets/community-set |
| Named AS-path sets | inline match | as-path-list | as-path-group | inline in filter | bgp/sets/as-path-set |
| Set local-pref | assign | set local-preference | then local-preference | modify filter | modify filter (same) |
| Inc/dec local-pref | arithmetic | set local-preference +N | then local-preference add | not possible | modify inc/dec |
| Add community | .add() | set community additive | then community add | tag (bulk) | modify community-add |
| Remove community | .delete() | set comm-list delete | then community delete | strip (bulk) | modify community-remove |
| Remove private AS | manual filter | remove-private-AS | remove-private | not possible | remove-private-as filter |
| Reject long AS-path | path.len > N | regex workaround | as-path-length | regex workaround | as-path-length filter |
| Policy dry-run | show route filter | debug route-map | test policy | not possible | show policy test |
| Bogon rejection | manual filter | manual config | manual config | manual prefix-list | ze:reject-bogon-prefixes |
| RS defaults | none | none | none | none | rs-profile |

## Wiring Test (MANDATORY -- NOT deferrable)

Umbrella coordinates child specs. Wiring tests live in each child spec.

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `bgp/sets` config | -> | set registry OnConfigure | pol-1: TestSetRegistryWiring |
| modify filter with inc/dec config | -> | modify handleFilterUpdate | pol-2: TestModifyIncDecWiring |
| remove-private-as in export chain | -> | remove-private-as handleFilterUpdate | pol-2: TestRemovePrivateASWiring |
| config commit with bad set ref | -> | BuildFilterRegistry validation | pol-3: TestSetRefValidation |
| show policy test CLI | -> | policy test handler | pol-4: TestPolicyTestWiring |
| rs-profile enable in config | -> | auto-populate import chain | pol-5: TestRSProfileWiring |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Named prefix-set defined in bgp/sets, referenced by prefix-list filter | Prefix-list uses set entries for matching |
| AC-2 | Named community-set defined, referenced by community-match filter | Community-match uses set entries |
| AC-3 | Named as-path-set defined, referenced by as-path-list filter | AS-path-list uses set regex patterns |
| AC-4 | modify filter with increment local-preference 50 on route with LP 100 | Route exits with local-preference 150 |
| AC-5 | modify filter with decrement med 30 on route with MED 20 | Route exits with MED 0 (floor, not underflow) |
| AC-6 | modify filter with community-add 65000:200 | Route gains community 65000:200, existing communities preserved |
| AC-7 | modify filter with community-remove 65000:100 | Community 65000:100 removed, others preserved |
| AC-8 | modify filter with large-community-add 65000:100:200 | Large community added to route |
| AC-9 | remove-private-as filter in export chain, AS_PATH contains 64512 | 64512 stripped from AS_PATH |
| AC-10 | remove-private-as with replace-with peer-as, peer AS is 65001 | Private ASN replaced with 65001 |
| AC-11 | as-path-length filter with max 30, route has 35-hop path | Route rejected |
| AC-12 | Config commit references undefined prefix-set | Commit rejected with clear error message |
| AC-13 | Config commit references prefix-set from as-path-list (type mismatch) | Commit rejected with type error |
| AC-14 | show policy test peer X import prefix 10.0.0.0/24 | JSON output with per-filter verdict |
| AC-15 | rs-profile enabled, no explicit import chain | Global import chain auto-populated with safety filters |
| AC-16 | ze:bogon-prefixes-v4 set referenced, route for 192.168.1.0/24 | Route rejected (RFC 1918 bogon) |

## 🧪 TDD Test Plan

### Unit Tests

Tests are defined in each child spec. Summary:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| pol-1: set registry, prefix-set parsing, set references | child spec | Named sets parsed and resolvable | |
| pol-2: inc/dec arithmetic, community add/remove, remove-private-as | child spec | Action operations correct | |
| pol-3: reference validation, type checking | child spec | Bad references caught at commit | |
| pol-4: dry-run execution, trace buffer | child spec | Explain output matches chain behavior | |
| pol-5: built-in sets, RS profile chain population | child spec | Defaults auto-populated correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| increment value | 1-4294967295 | 4294967295 | 0 | N/A (uint32) |
| decrement value | 1-4294967295 | 4294967295 | 0 | N/A (uint32) |
| as-path-length max | 1-65535 | 65535 | 0 | 65536 |
| as-path-length min | 0-65535 | 65535 | N/A | 65536 |
| as-path-prepend count | 1-32 | 32 | 0 | 33 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Defined in each child spec | child specs | Per-child-spec scenarios | |
| policy child suites | `test/plugin/policy-*.ci` (one per child spec; see Files to Create) | Named sets, action macros, commit validation, explain, and rs-defaults exercised from the user entry point | |

### Interop Tests (MANDATORY for protocol features)
AS_PATH modification (remove-private-as) and community actions affect wire-format
UPDATEs. Interop validation needed for pol-2.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| remove-private-as with FRR peer | `test/interop/scenarios/` | FRR | Private ASN stripped, FRR accepts modified path | |
| community-add with BIRD peer | `test/interop/scenarios/` | BIRD | Added communities visible to BIRD | |

### Future (if deferring any tests)
- IRR-based filtering tests (separate spec)
- VPP-specific policy tests (if VPP backend gains filter hooks)

## Files to Modify

Umbrella scope. Detailed file lists in each child spec.

- `internal/component/bgp/yang/ze-bgp-conf.yang` - add bgp/sets container
- `internal/component/bgp/config/filter_registry.go` - extend for set validation
- `internal/component/bgp/config/redistribution.go` - extend canonicalize for new filter types
- `internal/component/bgp/plugins/filter_modify/` - extend with inc/dec and community ops
- `internal/component/bgp/plugins/filter_prefix/` - add set reference leaf
- `internal/component/bgp/plugins/filter_aspath/` - add set reference leaf
- `internal/component/bgp/plugins/filter_community_match/` - add set reference leaf
- `internal/component/bgp/plugins/filter_community/handler.go` - Add/Remove handler branches
- `internal/component/bgp/reactor/filter_delta.go` - AS_PATH modify path for remove-private-as
- `internal/component/cmd/show/show_policy.go` - explain output

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | bgp/sets container, new filter type lists, rs-profile, show policy test |
| CLI commands/flags | [x] | show policy test, show policy explain |
| CLI grammar (action before identifier) | [x] | show policy test peer X import |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | child spec .ci tests |
| Doctor check for runtime dependencies | [ ] | N/A - no new runtime dependencies |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - structured policy language |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - bgp/sets, modify extensions |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - show policy test/explain |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - policy test RPC |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` - remove-private-as, as-path-length |
| 6 | Has a user guide page? | [x] | `docs/guide/policy.md` - new guide page |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A (no new RFC) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - policy capabilities |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - sets container, explain |

## Files to Create

Detailed in child specs. Summary:

- `internal/component/bgp/sets/` - new set registry package (pol-1)
- `internal/component/bgp/sets/yang/ze-bgp-sets.yang` - YANG for bgp/sets (pol-1)
- `internal/component/bgp/plugins/filter_remove_private_as/` - new plugin (pol-2)
- `internal/component/bgp/plugins/filter_aspath_length/` - new plugin (pol-2)
- `test/plugin/policy-*.ci` - functional tests per child spec

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + relevant child spec |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan in child spec |
| 3. Wiring phase | Wiring Test table in child spec |
| 4. Implement (TDD) | Implementation phases in child spec |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each child spec defines its own phases. Umbrella execution order:

1. **pol-1: Named sets** - bgp/sets YANG, set registry, plugin reference leaves
2. **pol-2: Actions** - modify extensions, remove-private-as, as-path-length
3. **pol-3: Validation** - compile-time checks in filter_registry
4. **pol-4: Explain** - show policy test, trace buffer
5. **pol-5: RS defaults** - built-in sets, built-in filters, rs-profile

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Inc/dec saturates correctly; community add is idempotent; remove-private-as handles AS_SET |
| Naming | Set names, filter type names follow existing kebab-case pattern |
| Data flow | Set data passed as value copy to filter plugins, no cross-boundary pointers |
| CLI grammar | show policy test peer X import (action before identifier) |
| Doctor checks | N/A (no new runtime dependencies) |
| Rule: no-sprintf | No fmt.Sprintf in filter text format or delta paths |
| Rule: buffer-first | Wire encoding uses existing WriteTo/append patterns |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| bgp/sets YANG container exists | grep for "bgp/sets" in YANG |
| prefix-set, community-set, as-path-set parseable | config parse test |
| Set references resolve in filter plugins | functional test with set reference |
| modify inc/dec works | unit test + functional test |
| community add/remove works | unit test + functional test |
| remove-private-as strips private ASNs | unit test + functional test |
| as-path-length accepts/rejects correctly | unit test + functional test |
| Bad set references rejected at commit | validation test |
| show policy test returns per-filter trace | functional test |
| rs-profile auto-populates chain | config test |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Set names validated (no injection via YANG list keys); regex patterns bounded (existing 512-char limit) |
| Integer overflow | Inc/dec saturates at uint32 boundaries, no wraparound |
| Resource exhaustion | Named sets have no size limit; consider max-entries if needed |
| Regex DoS | AS-path-set reuses Go RE2 (linear time, inherently safe) |
| Policy bypass | Built-in ze: sets cannot be overridden by user config |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
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

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [Pending]

### Bugs Found/Fixed
- [Pending]

### Documentation Updates
- [Pending]

### Deviations from Plan
- [Pending]

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Named sets reusable across filters | functional test | pol-1 test |
| Operator-friendly action macros | functional test | pol-2 tests |
| Compile-time validation catches errors | functional test | pol-3 test |
| Policy debugging via explain | functional test | pol-4 test |
| RS defaults reduce boilerplate | config test | pol-5 test |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [Pending]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
