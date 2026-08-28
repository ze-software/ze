# Spec: filter-wire-0-umbrella -- Wire-native filter-chain IPC (remove the text round-trip)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` (Ingress Filter Pipeline; Route Metadata and Modification Accumulator, ~lines 628-699)
4. `internal/component/bgp/reactor/filter_chain.go`, `filter_delta.go`, `filter_ordered.go`, `filter_format.go`, `forward_build.go`
5. `internal/component/bgp/filterapi/filterapi.go` (ModAccumulator), `internal/component/bgp/reactor/forward_context.go` (encoding-context stability)
6. `pkg/plugin/rpc/types.go` (FilterUpdateInput/Output), `pkg/plugin/sdk/sdk_callbacks.go` (FilterUpdateHandler)

## Task

The external-plugin BGP policy filter chain converts every filtered UPDATE through a
**text round-trip**: the reactor renders the wire attributes to a text string
(`AppendUpdateForFilter`, `filter_format.go`), ships that text to each filter plugin, a
modifying plugin returns a **text delta**, the reactor merges deltas as text
(`applyFilterDelta`, `filter_chain.go`), then converts the merged text back into wire
attribute-operations (`textDeltaToModOps`, `filter_delta.go`) and applies them
(`buildModifiedPayload`, `forward_build.go`). The in-process filters (community, role,
redistribute) already skip all of this -- they receive wire bytes and write wire mod-ops
directly (`core-design.md`).

**Goal:** remove the wire->text->text-delta->wire-ops round-trip for the external chain and
unify it onto the wire-native model the in-process filters and the apply engine already use:
filters read the raw UPDATE (`Raw` carrier, already delivered per rib-arch-2 / learned 1127)
and emit **wire mod-ops** (`{code, action, value-bytes}`), which feed the `ModAccumulator`
directly. This deletes `parseFilterAttrs` / `applyFilterDelta` / `formatFilterAttrs` /
`textDeltaToModOps` / `extractLegacyNLRIOverride` and the `Update string` IPC fields.

**Open design axis (benchmark-gated -- see Risks & Assumptions A-1):** whether to keep a text
representation for API-originated routes (provenance-aware dual representation) OR convert
API-origin routes to raw once at reactor entry so a single raw representation flows everywhere
(unify-on-raw). The user's expectation is that raw is more compact and easier to manipulate,
and that a one-time entry conversion will not regress vs text manipulation -- but this MUST be
measured before the final call (Phase 1 benchmark below). Unify-on-raw is the preferred
direction; dual-representation is the documented fallback.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `docs/architecture/core-design.md` (Ingress Filter Pipeline; Route Metadata / ModAccumulator, ~628-699) - the two filter mechanisms and the mod-op apply model
  -> Decision: in-process filters use `func(source, payload []byte, meta) (accept, modifiedPayload []byte)` and write `mods.Op(code, action, valueBytes)`; the external chain is the only text-based holdout. The redesign unifies the external chain onto this existing model, it does NOT invent a new algebra.
  -> Constraint: the cached `WireUpdate` after filtering is the canonical post-filter representation every downstream consumer reads (`core-design.md`); the redesign must keep that invariant -- filters still produce a modified wire payload, only the intermediate representation changes.
  -> Constraint: "copy on modify" -- every modify produces a new buffer; original wire buffer released on cache eviction (`core-design.md`).

### RFC Summaries
- [ ] `rfc/short/rfc6793.md` - AS4_PATH / four-octet ASN. The encoding-context (ASN4) stability is what makes binary forwarding safe only when source and dest share the capability.
  -> Constraint: forwarding a route between peers with different ASN4 capability requires AS_PATH/AS4_PATH re-encoding; `fwdContextIDWithASN4` (`forward_context.go`) already gates this.
- [ ] `rfc/short/rfc6996.md` - private ASN ranges (remove-private-as rewriting).

**Key insights:**
- The external policy chain is the single text-based filter mechanism; everything else is already wire + mod-ops.
- Text is *derived from wire at filter time* today (`AppendUpdateForFilter`), for BOTH API-origin and peer-origin routes -- by filter time every route is already wire. So the round-trip is pure overhead at the filter stage; the only place text is genuinely "native" is the API origination command (`update text ...`), which already converts to wire at entry via `AnnounceNLRIBatch`.
- Two modify operations (`as-path-prepend`, `remove-private`) structurally cannot be computed plugin-side: they need `localAS` and the `asn4` flag which live only reactor-side (`filter_delta.go`, `:570`). They must keep a thin typed-intent channel.

## Current Behavior (MANDATORY)

**Source files read (with the three research agents' findings folded in):**
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain` (text pipe, `:175`), `applyFilterDelta` (text merge, `:223`), `parseFilterAttrs`/`formatFilterAttrs`, AC-13 validation, `policyFilterFunc` builds `FilterUpdateInput`.
  -> Constraint: a raw override is TERMINAL -- cannot compose with downstream filters. The text-delta layer exists specifically to compose multiple filters. The wire-op design composes via a single shared `ModAccumulator` instead.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `textDeltaToModOps` (`:202`, skips faNLRI/faASPath/faASPathPrepend/faRemovePrivate at `:206`), `ExtractASPathPrependOps` (`:536`, needs localAS), `ExtractRemovePrivateASOps` (`:570`, needs raw attrs + asn4 + peerAS), `extractLegacyNLRIOverride` (`:51`, IPv4-unicast only), the `encode*Value` wire encoders.
- [ ] `internal/component/bgp/reactor/filter_format.go` - `AppendUpdateForFilter` (`:42`, wire->text at filter time), `AppendAttrsForFilter` (`:165`, empty-declared => all attrs), `attrNameToCode`.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - the import and export call sites: `AppendUpdateForFilter` -> `PolicyFilterChain` -> `parseFilterAttrs` x2 -> `textDeltaToModOps` + `ExtractRemovePrivateASOps` + `ExtractASPathPrependOps` -> `extractLegacyNLRIOverride` -> `buildModifiedPayload`.
- [ ] `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` consumes `ModAccumulator` ops via per-code `AttrModHandler`s; `groupOpsByCode`; NLRI override (Step 8, `:270`).
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator`, `Op(code, action, buf)`, actions `AttrModSet/Add/Remove/Prepend/Suppress`, `AttrOp{Code,Action,Buf}`, out-of-band channels `SetWithdraw`/`SetNLRIRewrite`/`SetWithdrawnRewrite`, `RegisterAttrModHandler`.
- [ ] `internal/component/bgp/reactor/forward_context.go` - `fwdContextIDWithASN4`: same ASN4 => forward unchanged; different => register new encoding context.
- [ ] `pkg/plugin/rpc/types.go` - `FilterUpdateInput` (`:177`: Filter, Direction, Peer, PeerAS, `Update string`, `Raw []byte`), `FilterUpdateOutput` (`:192`: Action, `Update string`, `Raw []byte`, Teardown...), `FilterAction` enum (`enums.go`).
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `FilterUpdateHandler func(*FilterUpdateInput)(*FilterUpdateOutput,error)`, `OnFilterUpdate`, decode/call/encode shim. NOTE: no typed DirectBridge fast-path for filter-update -- always JSON (`Raw` base64) both directions.
- [ ] the 7 plugins (per-plugin detail lives in the child specs): filter_aspath, filter_aspath_length, filter_community_match (accept/reject only), filter_prefix, filter_irr (NLRI modify), filter_modify, filter_remove_private_as (attribute modify).

**The 7 external plugins (research summary -- exhaustive per-plugin detail in children):**

| Plugin | Callback | Reads today | Decision | Migration |
|--------|----------|-------------|----------|-----------|
| filter_aspath | filter_aspath.go | AS_PATH (text) | accept/reject | easy: wire-decode read-only |
| filter_aspath_length | filter_aspath_length.go | AS_PATH hop count | accept/reject | easy: wire-decode read-only |
| filter_community_match | filter_community_match.go | COMMUNITY/LARGE/EXT | accept/reject | easy: wire-decode read-only |
| filter_prefix | filter_prefix.go | NLRI prefixes | accept/reject/**modify** | hard: emit wire NLRI mod-op (filtered prefix set) |
| filter_irr | filter_irr.go | NLRI prefixes | accept/reject/**modify** | hard: emit wire NLRI mod-op |
| filter_modify | filter_modify.go | local-pref/med/origin/next-hop/aigp + communities + as-path-prepend | **modify** | attribute mod-ops + prepend intent |
| filter_remove_private_as | filter_remove_private_as.go | AS_PATH (text) + AS4_PATH (raw gate) | **modify** | remove-private intent (reactor computes wire) |

Cross-cutting: none of the 7 declare a `FilterDecl` except filter_remove_private_as (`raw=true`, `Attributes:["as-path","remove-private"]`); the others get declaredAttrs=empty => receive ALL attributes as text and AC-13 modify-validation is skipped (which is why filter_prefix/irr may emit an `nlri` delta with no declared attrs).

**Behavior to preserve:**
- The post-filter cached `WireUpdate` remains the canonical representation for all downstream consumers.
- Every existing filter's accept/reject/modify semantics and every existing filter `.ci`/Go test expectation.
- AC-13 (a filter may only modify attributes it declared) -- re-expressed at the mod-op level.
- The IPv4-unicast-only NLRI modify limitation is documented today (`filter_delta.go`); the redesign is an opportunity to fix the IPv6/MP_REACH NLRI case (child spec decision, not forced).
- Teardown (import-only) semantics.

**Behavior to change:**
- The filter IPC representation (text -> raw + mod-ops). External-visible protocol change (versioned; SDK-facing).

## Data Flow (MANDATORY)

### Entry Point
- Peer-received UPDATE: wire bytes -> reactor ingress (`reactor_notify.go`).
- API-originated route: `update text ...` command -> `ParseUpdateText` -> structured -> `AnnounceNLRIBatch` builds wire (`reactor_api_batch.go`).

### Transformation Path (today, export example)
1. `WireUpdate` (wire) held in rib-out.
2. `AppendUpdateForFilter` renders wire -> **text** (`filter_ordered.go`).
3. `PolicyFilterChain` pipes text through each external filter; modify returns **text delta**.
4. `applyFilterDelta` merges deltas (text).
5. `parseFilterAttrs` x2 + `textDeltaToModOps` + Extract{ASPathPrepend,RemovePrivateAS}Ops -> `ModAccumulator` (wire ops).
6. `extractLegacyNLRIOverride` -> nlriOverride bytes.
7. `buildModifiedPayload` -> modified wire `WireUpdate`.

### Transformation Path (proposed)
1. `WireUpdate` (wire) held in rib-out.
2. Reactor ships `Raw` (wire body) to each external filter (no text render).
3. Filter wire-decodes what it needs (SDK accessor), returns **wire mod-ops** (+ optional NLRI rewrite, withdraw, or as-path-prepend/remove-private intent).
4. Reactor accumulates mod-ops across the chain into one `ModAccumulator`; intents (prepend/remove-private) resolved reactor-side with localAS/asn4.
5. `buildModifiedPayload` -> modified wire `WireUpdate` (unchanged apply engine).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> external filter plugin | JSON: `FilterUpdateInput.Raw` (base64) down, mod-op list up. No typed bridge exists for this RPC (`sdk_dispatch.go`); a typed DirectBridge slot is an optional perf follow-up. | [ ] |
| Reactor <-> ModAccumulator | `mods.Op(code, action, valueBytes)` + out-of-band NLRI/withdraw channels | [ ] |

### Integration Points
- `buildModifiedPayload` + `AttrModHandler`s: unchanged; fed from filter mod-ops instead of textDeltaToModOps.
- `fwdContextIDWithASN4`: unchanged; binary splice safe only for same-context forwards.

### Architectural Verification
- [ ] No bypassed layers (mod-ops still flow through buildModifiedPayload)
- [ ] No unintended coupling (plugins stay wire-only; reactor keeps AS-context ops)
- [ ] No duplicated functionality (deletes the text machinery, does not add a parallel one)
- [ ] Zero-copy preserved (Raw passed without copy where the plugin gets a marshalled copy anyway)
- [ ] Registration over hardcoding (mod-op handlers already registry-based via RegisterAttrModHandler)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Unify-on-raw: a one-time text->raw conversion at API entry + raw-based manipulation does NOT significantly regress vs today's text manipulation. | User's expectation (raw is more compact); `forward_build.go` already applies wire ops with no text. | If it regresses, fall back to provenance-aware dual representation (keep text for API-origin). | **Phase 1 benchmark** vs `BenchmarkFilterModifyEgress` (filter_delta_test.go) + a new API-origin benchmark. | unvalidated |
| A-2 | Mod-ops can express every existing modify a filter emits, EXCEPT as-path-prepend and remove-private (which keep a typed intent). | Agent audit: filter_modify/remove_private are the only attribute modifiers; filter_prefix/irr only rewrite NLRI. | If a modify can't be expressed, that plugin keeps a narrow text/intent escape hatch. | Per-plugin child-spec design + tests | unvalidated |
| A-3 | The 3 read-only filters need only single-attribute wire decode; no output-side change. | Agent audit (accept/reject only, never emit delta). | If one modifies, it moves to the "hard" child. | grep each callback for FilterModify | unvalidated |
| A-4 | AC-13 (declared-attribute restriction) is expressible over mod-op codes. | Today it parses the text delta; mod-ops already carry the attr code. | Re-derive from FilterDecl.Attributes -> code set. | unit test on the validator | unvalidated |
| A-5 | NLRI rewrite (filter_prefix/irr) can be carried as `ModAccumulator.SetNLRIRewrite` / a wire prefix list, replacing the text `nlri ...` delta. | `filterapi.go`, `forward_build.go` already support NLRI override. | If MP_REACH families are in scope, needs MP rewrite (not supported today). | filter_prefix child spec + .ci | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | High blast radius: protocol + reactor + 7 plugins + SDK + all filter tests. | Compile/lint fanout when `pkg/plugin/rpc` changes (rib-arch-4/2 pattern). | Umbrella + child specs; migrate one plugin at a time behind a versioned protocol; budget a repair commit. |
| R-2 | Cross-process external plugins (non-bridge) get a protocol break. | Old external plugins send text deltas. | Version the FilterUpdateOutput; support both during migration, or gate on a declared protocol version. |
| R-3 | Losing the human-readable text hurts `policy trace`/dry-run explainability. | `policy_dryrun.go` renders text today. | Keep a wire->text renderer for the trace/explain surface ONLY (display), decoupled from the filter transport. |
| R-4 | as-path-prepend/remove-private intent channel re-introduces a mini text protocol. | Two special-case fields. | Typed intent (enum + count), not text tokens. |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Peer UPDATE + configured `filter { export [modify] }` | -> | reactor ships Raw, filter emits mod-op, buildModifiedPayload applies | child-spec `.ci` asserting the modified wire on the receiving peer |
| `policy trace` / dry-run | -> | wire->text display renderer (decoupled) | dry-run `.ci` still shows readable attrs |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Phase 1 benchmark run | text->raw entry conversion + raw manipulation measured vs text path; A-1 resolved (unify-on-raw or dual) with numbers recorded |
| AC-2 | A read-only filter (aspath/aspath-length/community-match) on a wire UPDATE | same accept/reject decision as today, reading `Raw` only, `Update string` gone |
| AC-3 | filter_modify sets local-preference | emits an AttrModSet mod-op; receiving peer sees LOCAL_PREF changed; no text delta crosses IPC |
| AC-4 | filter_modify as-path-prepend N | emits a typed prepend intent; reactor builds AttrModPrepend with localAS; wire AS_PATH prepended N times |
| AC-5 | filter_remove_private_as on a path with private ASNs | emits remove-private intent; reactor rewrites AS_PATH/AS4_PATH segment-preserving; wire path stripped |
| AC-6 | filter_prefix on a mixed accept/reject NLRI set | emits a wire NLRI rewrite (accepted subset); receiving peer sees only accepted prefixes |
| AC-7 | filter declares attribute set, tries to modify undeclared attr | rejected (AC-13 at mod-op level) |
| AC-8 | text machinery removed | `parseFilterAttrs`, `applyFilterDelta`, `formatFilterAttrs`, `textDeltaToModOps`, `extractLegacyNLRIOverride`, `Update string` fields all deleted; grep clean |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `filter { export [modify-lp] }`; peer route forwarded | wire -> reactor ships Raw -> filter_modify mod-op -> buildModifiedPayload -> wire to peer | child `.ci` asserts LOCAL_PREF on receiving peer |
| 2 | Runs `policy trace` | wire -> display renderer -> readable attrs | dry-run `.ci` |
| 3 | External (non-bridge) plugin author writes a filter | SDK accessor reads Raw, returns mod-ops | SDK unit + functional test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BenchmarkFilterModifyEgress` (reuse) | `filter_delta_test.go` | perf guard: raw path <= text path | |
| `BenchmarkFilterModifyEgress_Raw` (new) | reactor | raw-manipulation cost for A-1 | |
| `TestModOpValidateDeclared` | reactor | AC-13 over mod-op codes | |
| per-plugin decode/emit tests | each plugin | wire-decode decision + mod-op output | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-modify-wire` | `test/plugin/*.ci` | modify filter changes LOCAL_PREF on wire | |
| `filter-prefix-wire` | `test/plugin/*.ci` | prefix filter drops rejected NLRI on wire | |
| `filter-remove-private-wire` | `test/plugin/*.ci` | private ASN stripped on wire | |
| existing filter `.ci` suite | `test/plugin/` | all pass unchanged (semantics preserved) | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing egress-filter interop (if any) | `test/interop/scenarios/` | FRR/BIRD | modified attrs accepted by a real peer | |

## Files to Modify
- `pkg/plugin/rpc/types.go` - FilterUpdateInput/Output: drop `Update string`, add mod-op list + intent fields; version.
- `pkg/plugin/sdk/sdk_callbacks.go`, `sdk_types.go` - SDK accessor over Raw; handler signature.
- `internal/component/bgp/reactor/filter_chain.go` - delete text pipe/merge/parse/format; compose mod-ops.
- `internal/component/bgp/reactor/filter_delta.go` - delete textDeltaToModOps/extractLegacyNLRIOverride/encode*Value text encoders; keep Extract{ASPathPrepend,RemovePrivateAS}Ops driven by typed intent.
- `internal/component/bgp/reactor/filter_ordered.go` - rewire import/export call sites to mod-op composition.
- `internal/component/bgp/reactor/filter_format.go` - repurpose to a display-only renderer (trace/explain).
- `internal/component/bgp/reactor/policy_dryrun.go` - use the display renderer; mod-op trace.
- the 7 plugins (per child spec).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Plugin SDK/protocol changed | [x] | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| Functional test for new RPC/API | [x] | `test/plugin/*.ci` |
| Doctor check | [ ] | n/a (no new runtime dependency) |
| Prometheus counters | [ ] | reuse existing filter metrics |

## Files to Create
- `plan/spec-filter-wire-1-protocol.md` - IPC protocol + SDK accessor + versioning (skeleton).
- `plan/spec-filter-wire-2-reactor.md` - reactor mod-op composition + AS-context intent + display renderer (skeleton).
- `plan/spec-filter-wire-3-plugins-readonly.md` - filter_aspath, filter_aspath_length, filter_community_match (skeleton).
- `plan/spec-filter-wire-4-plugins-nlri.md` - filter_prefix, filter_irr (NLRI rewrite; MP_REACH decision) (skeleton).
- `plan/spec-filter-wire-5-plugins-modify.md` - filter_modify, filter_remove_private_as (attribute mod-ops + intent) (skeleton).
- `test/plugin/filter-*-wire.ci` - per-plugin wire assertions.

## Implementation Phases

1. **Phase 1: Benchmark spike + decision gate (MANDATORY FIRST -- resolves A-1).**
   - Build a raw-manipulation micro-benchmark and an API-origin text->raw entry benchmark; compare against `BenchmarkFilterModifyEgress` and `AppendUpdateForFilter`/`AppendAttrsForFilter` benchmarks.
   - Decide unify-on-raw vs dual-representation; record numbers in A-1; update the design.
   - No production code deleted yet.
2. **Phase 2 (child 1): protocol + SDK.** Versioned FilterUpdateInput/Output with mod-op list + intents; SDK accessor over Raw; both representations tolerated during migration.
3. **Phase 3 (child 2): reactor composition.** Mod-op accumulation across the chain; AS-context intent resolution; display-only renderer; AC-13 at mod-op level.
4. **Phase 4 (child 3): read-only plugins.** Migrate aspath/aspath-length/community-match to wire-decode.
5. **Phase 5 (child 5): modify plugins.** filter_modify + filter_remove_private_as to mod-ops/intents.
6. **Phase 6 (child 4): NLRI plugins.** filter_prefix + filter_irr to wire NLRI rewrite (+ MP_REACH decision).
7. **Phase 7: delete the text machinery.** Remove parseFilterAttrs/applyFilterDelta/formatFilterAttrs/textDeltaToModOps/extractLegacyNLRIOverride/`Update string`; close perf-next-2 (its goal met by deletion on the peer-forward path); repair-commit the lint fanout.

Each phase ends with a Self-Critical Review.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Unify external chain onto wire mod-ops | invent a new composition algebra | in-process filters + apply engine already speak mod-ops; reuse, don't reinvent |
| Benchmark-gate unify-on-raw vs dual | commit to one upfront | user requires measurement; raw is expected more compact but must be proven not to regress API-origin |
| as-path-prepend / remove-private keep typed intent | make plugins self-compute | plugin structurally lacks localAS + asn4 flag (reactor-only); a typed intent (not text) is minimal |
| Keep a wire->text renderer for trace/explain only | delete all text | human-readable policy trace is a real UX need; decouple display from transport |
| Umbrella + per-plugin children | one big spec | reviewable, incremental, one-plugin-at-a-time migration behind a versioned protocol |

## Known Limitations
- Cross-process external filter plugins need a protocol version bump; a compatibility window is required.
- MP_REACH NLRI rewrite for filter_prefix/irr is not supported today (the text path never handled it, `filter_delta.go`); the NLRI child spec decides whether to add it.

## Relationship to perf-next-2-filter-delta-alloc
`spec-perf-next-2-filter-delta-alloc.md` (in-progress, Phase 1/5) reduces allocations in
`textDeltaToModOps` (~24 allocs/modified UPDATE). This redesign DELETES that path on the
peer-forward filter stage, so its goal is met by deletion there; its scope narrows to any
residual text-conversion the API-origin path keeps (if dual-representation wins A-1) or is
fully absorbed (if unify-on-raw wins). Its `BenchmarkFilterModifyEgress` becomes this
redesign's perf-regression guard. Reconciliation recorded here per user direction 2026-07-16.

## Implementation Steps

<!-- Umbrella coordination spec: the concrete per-file work lives in the child specs.
     The phase list under "Implementation Phases" above is the ordered plan; this section
     maps it to the child specs that own each step. -->

1. **Phase 1 (this umbrella): benchmark spike + decision gate.** Resolve A-1 (unify-on-raw vs dual) with measured numbers before any deletion.
2. **Child `spec-filter-wire-1-protocol.md`:** versioned FilterUpdateInput/Output (mod-op list + typed intents), SDK accessor over `Raw`.
3. **Child `spec-filter-wire-2-reactor.md`:** reactor mod-op composition, AS-context intent resolution, display-only renderer, AC-13 at mod-op level.
4. **Child `spec-filter-wire-3-plugins-readonly.md`:** filter_aspath, filter_aspath_length, filter_community_match.
5. **Child `spec-filter-wire-5-plugins-modify.md`:** filter_modify, filter_remove_private_as.
6. **Child `spec-filter-wire-4-plugins-nlri.md`:** filter_prefix, filter_irr.
7. **Delete the text machinery** and close perf-next-2 (final step, after all plugins migrated).

Select each child individually when implementing (per `ai/rules/planning.md` "Spec Sets"). The
umbrella stays open until every child closes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated (across children)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify current mode full` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
