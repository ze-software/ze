# Spec: fixit-private-asn-leak-deferred-ebgp-auto-filters

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - (but SEQUENCES against three in-progress specs -- see "Convergence") |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/config/peers.go:643-673` - `prependDefaultFilters`, the precedent this design copies
4. `internal/component/bgp/reactor/filter_ordered.go:221-260` - `runEgressPolicyChainASN4`, the one shared chain body
5. `internal/component/bgp/reactor/reactor_api_forward.go:480-499` - the ONLY consumer of `orderedEgressSteps`, and why it is the wrong hook
6. `internal/component/bgp/wireu/aspath_rewrite.go:63` - `rewriteASPathPrepend`, the seam three other specs are already editing

## Task

Give every EBGP peer two **auto-added** entries on its export filter chain:

1. **prepend our ASN when sending** (RFC 4271 §9.1.2)
2. **remove LOCAL_PREF** (RFC 4271 §5.1.5 MUST NOT)

instead of special-casing both inside the wire path.

**Provenance:** Thomas's design ruling, 2026-07-16, verbatim: *"we should have an
'auto-added' filter added to the chain of ebgp peer to add our ASN to the peer when sending
and another one to remove local pref"*. Given in answer to the EBGP-prepend question that
`plan/spec-fixit-private-asn-leak.md:311-317` flagged and deliberately did not answer
("Flagged for Thomas"; also listed as an approved open item at `:281` and `:409`).

### This is not only a design preference. One half of it is an unenforced RFC MUST NOT

The trace (2026-07-16) found that **RFC 4271 §5.1.5 is not enforced on the forwarded egress
path at all**:

| Path | LOCAL_PREF to an EBGP peer | Evidence |
|------|---------------------------|----------|
| Ingress (received from EBGP) | discarded | `message/rfc7606.go:442-450` `validateLocalPrefAttr`: `if !isIBGP` -> `RFC7606ActionAttributeDiscard` / `DiscardReasonEBGPInvalid` |
| Origination (`update text` / plugin) | IBGP only | `message/update_build_plugin.go:97-105` `if ub.IsIBGP { ... attribute.LocalPref(lp) }`; `:79-81` drops a plugin-supplied raw LOCAL_PREF |
| Origination (rib commit) | IBGP only | `rib/commit.go:341-344` `includeLocalPref := c.isIBGP()` |
| **Forwarded (IBGP source -> EBGP destination)** | **KEPT ON THE WIRE** | `grep -i localpref internal/component/bgp/wireu/*.go` -> **zero non-test hits**. The EBGP wire path prepends AS_PATH and never touches LOCAL_PREF. No egress strip anywhere in `reactor/*.go` |

The only reason this usually does not bite is that LOCAL_PREF from an EBGP *source* is
discarded at ingress -- which does not cover **IBGP source -> EBGP destination**. That case
leaks LOCAL_PREF onto the EBGP wire today. Thomas's second auto-filter closes it.
`attribute/builder.go:454-457` even carries the comment *"LOCAL_PREF (filtered at send time
for eBGP)"* above a function that appends `LocalPref` unconditionally: the send-time filter
it defers to does not exist.

### The precedent already exists -- on the import side only

`config/peers.go:643-673` `prependDefaultFilters` prepends loop-detection entries to
`ps.ImportFilters` unless already present (`filterChainContains`, `:667`). Line `:670` only
ever touches `ImportFilters`. **The export chain is 100% user config today; nothing is
auto-added to it.** This design is `prependDefaultFilters`' mirror image, and should look
like it.

`PeerSettings.IsEBGP()` (`reactor/peersettings.go:541-543`, `LocalAS != PeerAS`) is
available at exactly the config-time point where `prependDefaultFilters` runs
(`config/peers.go:647`, which receives `map[string]*reactor.PeerSettings`), so an
EBGP-gated auto-append is directly expressible.

### Both filters are expressible in today's vocabulary -- no new ops needed

| Auto-filter | Existing op | Evidence |
|-------------|-------------|----------|
| prepend our ASN | `mods.Op(byte(attribute.AttrASPath), filterapi.AttrModPrepend, buf)` | exactly what `ExtractASPathPrependOps` already emits; it builds an AS_SEQUENCE of N x localAS at `filter_delta.go:547-557` |
| remove LOCAL_PREF | `mods.Op(byte(attribute.AttrLocalPref), filterapi.AttrModSuppress, nil)` | `AttrModSuppress` = "Remove entire attribute from UPDATE" (`filterapi/filterapi.go:191-197`); `{attribute.AttrLocalPref, 0x40}` already has a registered generic handler (`filter_delta_handlers.go:91`) |

The filter *text* vocabulary already names both concepts: `faASPathPrepend`
("as-path-prepend", `filter_chain.go:50`) and `faLocalPreference` ("local-preference",
`:35`/`:58`).

### The hook choice is the crux, and one option is a trap

| Candidate hook | Runs on forwarded path? | Runs on originated/injected path? | Verdict |
|----------------|------------------------|-----------------------------------|---------|
| `filterapi` in-process egress step (`buildOrderedEgressSteps`, `filter_ordered.go:102-125`) | YES (`reactor_api_forward.go:480-499` is its only consumer) | **NO** | **Trap.** `exportFilterForBody` does not iterate `orderedEgressSteps`; it calls `runEgressPolicyChainASN4` directly (`egress_inject_filter.go:56`). An auto-filter registered here silently misses every originated route -- the exact class of bug `spec-fixit-private-asn-leak` just fixed |
| A `FilterRef` auto-appended to `ps.ExportFilters` at config time, à la `prependDefaultFilters` | YES | YES | **Preferred.** Both paths reach the one shared body `runEgressPolicyChainASN4` (`filter_ordered.go:221-260`), so a chain entry is honored by both automatically |

This is not a new invariant: it is the Goal Gate `spec-fixit-private-asn-leak.md:423`
already banked -- *"one shared chain body; a future outcome added to the chain is honored by
both paths automatically."* This design is the first thing to cash that in.

**Open design question (do not answer in code):** `FilterRef` is a name reference with no
parameters and no origin marker -- `FilterRef{Name string, Inactive bool}`
(`filterapi/filterapi.go:52-55`). There is no "auto" bit. An auto entry must therefore
either reserve a name (and `config/peers.go:184-191` `ValidateFilterNames` +
`:200-203` `canonicalizeFilterRefs` must accept it), or `FilterRef` grows a marker. Decide
which, and how `show`/config output renders an entry the operator never typed.

### Convergence: three in-progress specs are standing on the seam this design retires

`wireu.rewriteASPathPrepend` (`aspath_rewrite.go:63`) is the de-facto per-destination EBGP
egress hook today. Moving prepend into the chain changes what it is for. **This is a
sequencing decision, and it belongs to Thomas, not to whoever picks this up:**

| Spec | Status | Its claim on the seam | Conflict |
|------|--------|----------------------|----------|
| `spec-fixit-as4path-missing-on-rewrite` | in-progress | owns the *inside* of `rewriteASPathPrepend` / `aspath_transcode.go` / `aspath_as4.go`: `RewriteASPath`/`RewriteASPathDual` substitute AS_TRANS but never emit the AS4_PATH RFC 6793 §4.2.2 requires | actively editing the function this design would bypass or replace for EBGP prepend |
| `spec-perf-next-1-ebgp-wire-lockfree` | in-progress (1/5) | owns the `ReceivedUpdate.EBGPWire` cache that **memoizes prepend output** (`received_update.go:138`) | moving prepend into the chain changes what that cache caches, or makes it dead |
| `spec-fixit-tombstone-ebgp-transitive` | in-progress | adds the Section 5.3 Transitive-clear **inside `rewriteASPathPrepend`**, because `attr_discard.go:59` says the egress rule "is enforced per destination on the EBGP wire path, in `wireu.rewriteASPathPrepend`, not here" | same seam; and its `.ci` ruling (`:65-72`) is explicitly the interim answer *"until we re-engineer how we deal with attributes"* -- **this spec looks like that re-engineering** |

Prepend also exists in **two independent implementations** already, which is context for why
consolidating into the chain is attractive:

- forwarded: `wireu.rewriteASPathPrepend`, reached only via `RewriteASPath` (`:35-40`) and `RewriteASPathDual` (`:52-58`), all callers gated `facts.isEBGP && !facts.rsClient` (`reactor_api_forward.go:527`, `forward_rs.go:352`)
- originated: `rib/commit.go:435-465` `buildASPathFromExplicit` -- IBGP preserves as-is (`:436-442`), else prepends `c.ctx.LocalASN()` to the first AS_SEQUENCE (`:444-464`)

**Correction worth carrying:** `spec-fixit-private-asn-leak.md:311-317` says "the originated
path does not prepend the local AS for EBGP peers". That is true of the
`update text` / `BuildPlugin` path (`update_build_plugin.go:58-69`), **not** of the
`rib/commit` path, which does prepend for EBGP (`commit.go:444-464`). So "the originated
path" is at least two paths that disagree with each other. Establish which is which before
designing; that disagreement may be the actual bug.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - registration over hardcoding
  → Constraint: an auto-added chain entry must go through the existing filter registry, not a new per-feature switch in a core struct
- [ ] `ai/rules/memory-architecture.md` - buffer ownership on the forward path
  → Constraint: the received wire is shared by every destination and MUST NOT be mutated per destination. `rewriteASPathPrepend` is the only point already paying for a per-destination buffer -- a chain entry that forces a second copy is a real cost, and `spec-fixit-tombstone-ebgp-transitive` notes the EBGP RS-client case (`facts.isEBGP && facts.rsClient`) has no per-destination buffer at all
- [ ] `ai/rules/config-surface.md` + `ai/patterns/config-option.md` - if the auto entries become visible/disable-able

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - §5.1.5 (`:381`, `:702`: "MUST NOT include LOCAL_PREF in UPDATE messages sent to external peers"), §9.1.2 (prepend)
  → Constraint: §5.1.5 is a MUST NOT and is currently unenforced on the forwarded egress path
- [ ] `rfc/short/rfc6793.md` - AS4_PATH, owned by `spec-fixit-as4path-missing-on-rewrite`
  → Constraint: whatever performs prepend inherits that spec's AS4_PATH obligation

**Key insights:**
- `IsEBGP()` is `LocalAS != PeerAS`, so the `local-as` override modes (`localASNoPrepend`, `localASReplaceAS`, `asOverride` -- `peer_forward_facts.go:48-50`) interact with it, and the existing prepend already branches on them (`reactor_api_forward.go:519`). An auto-filter must reproduce that branching or it is a regression.
- `runEgressPolicyChainASN4`'s internal order is already: `PolicyFilterChain` -> Reject -> raw override -> text-delta -> `textDeltaToModOps` (`:249`) -> `ExtractRemovePrivateASOps` (`:250`) -> `ExtractASPathPrependOps` (`:251`) -> `buildModifiedPayload` (`:254`). Where the auto entries sit relative to `remove-private-as` is load-bearing: `remove-private-as-export.ci:1-4` asserts strip happens **before** prepend and that "EBGP local-AS prepend uses the stripped AS_PATH as its base".
- `orderedEgressSteps` is assembled once (`reactor.go:1214`) and sorted by `filterapi.LessOrder(name, stage, priority)` across stages `FilterStageProtocol=0` / `Policy=100` / `Annotation=200` / `PeerChain=300`. An RFC-mandated auto-filter is conceptually `FilterStageProtocol` ("RFC-mandated checks") -- but see the hook trap: that ladder is not reached by the originated path.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/peers.go` - `:143-168` `ExportFilters = concatFilters(bgpExport, groupExport, peerExport)` (cumulative bgp+group+peer); `:184-191` `ValidateFilterNames`; `:200-203` `canonicalizeFilterRefs` (`<plugin>:<filter>`); `:643-673` `prependDefaultFilters` (import-only precedent)
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `:52-58` `orderedEgressStep`; `:66-69` `egressStepResult{accept, wireOverride}`; `:102-125` `buildOrderedEgressSteps`; `:195-203` / `:221-260` the chain bodies
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - `:56` calls `runEgressPolicyChainASN4` directly, bypassing `orderedEgressSteps`
- [ ] `internal/component/bgp/reactor/peersettings.go` - `:392-394` `ExportFilters []filterapi.FilterRef` (frozen per-peer chain); `:536-543` `IsIBGP`/`IsEBGP`

**Behavior to preserve:**
- `remove-private-as-export.ci`'s assertion that strip precedes prepend, and its AS_PATH `[65000 64496 64497]` byte-exact.
- `attributes.ci:54-72` -- peer1 is **IBGP** (`local 1`/`remote 1`, `:66-67`); its `400504000000C8` (LOCAL_PREF=200) and verbatim AS_PATH `[1 2 3 4]` (`:10`, `:12`) must not move. It is the IBGP keep-LOCAL_PREF / no-prepend baseline and does **not** cover EBGP. (`spec-fixit-private-asn-leak.md:313` cites it as evidence of "the same verbatim-as-path behavior" -- true only for IBGP.)
- Every `local-as` override mode's current prepend behavior.

**Behavior to change:**
- LOCAL_PREF must stop reaching EBGP peers on the forwarded path (RFC 4271 §5.1.5).
- Prepend for EBGP becomes a chain entry rather than a wire-path special case.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | An auto `FilterRef` on `ExportFilters` reaches both the forwarded and originated paths | both reach `runEgressPolicyChainASN4`; Goal Gate `spec-fixit-private-asn-leak.md:423` | the design silently misses originated routes -- the leak class all over again | a functional test on each path before building anything else | unvalidated |
| A-2 | Moving prepend off `rewriteASPathPrepend` does not cost a second per-destination copy | that function exists because it is the one place already paying for one | the chain approach costs an extra copy per EBGP destination per UPDATE | benchmark against `spec-perf-next-1-ebgp-wire-lockfree`'s baseline | unvalidated |
| A-3 | IBGP-source -> EBGP-destination LOCAL_PREF leak is real | `grep -i localpref internal/component/bgp/wireu/*.go` = zero non-test hits; no egress strip in `reactor/*.go` | the MUST NOT half of this spec evaporates | **write the failing functional test FIRST** -- it does not exist today | unvalidated (grep-established, not test-established) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Three in-progress specs are editing the seam this retires; landing out of order wastes one of them | any of the three closes while this is in design | Thomas sequences this explicitly. Cheapest order is likely: let the three land, then re-home their EBGP-egress logic into the chain |
| R-2 | The EBGP RS-client case has no per-destination buffer (`forward_rs.go:351-360`, `reactor_api_forward.go:526-535`) so a chain entry cannot mutate its wire | RS-client tests break, or the auto entries silently skip RS-clients | Same open question `spec-fixit-tombstone-ebgp-transitive` Known Limitations already flags as "Thomas's call": conformance vs RS zero-copy |
| R-3 | `FilterRef` has no "auto" marker, so an auto entry is indistinguishable from a typed one in config/show output | operator sees a filter they never wrote, or cannot see the one protecting them | Design the marker + rendering before implementing |
| R-4 | Prepend's two implementations disagree (`update text` does not prepend, `rib/commit` does) and consolidation silently picks one | the two originated paths produce different AS_PATHs for the same intent | Establish the intended behavior for each first; that disagreement may itself be the bug |

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config parse: a peer's `ExportFilters` are assembled from bgp + group + peer (`config/peers.go:143-168`), validated (`:184-191`), canonicalized to `<plugin>:<filter>` (`:200-203`). **This is where the auto entries are appended**, mirroring `prependDefaultFilters` (`:643-673`), gated on `ps.IsEBGP()` (`peersettings.go:541-543`)
- Runtime: a route reaches the peer's egress, forwarded or originated

### Transformation Path
1. Chain frozen onto the peer: `PeerSettings.ExportFilters []filterapi.FilterRef` (`peersettings.go:392-394`)
2. Both egress paths converge on the one shared body `runEgressPolicyChainASN4` (`filter_ordered.go:221-260`) -- the forwarded path via `reactor_api_forward.go:480-499`, the originated path via `egress_inject_filter.go:56`
3. Inside that body, in order: `PolicyFilterChain` -> Reject -> raw override -> text-delta -> `textDeltaToModOps` (`:249`) -> `ExtractRemovePrivateASOps` (`:250`) -> `ExtractASPathPrependOps` (`:251`) -> `buildModifiedPayload` (`:254`)
4. The auto entries emit ops in the existing vocabulary: `AttrModPrepend` on AS_PATH, `AttrModSuppress` on LOCAL_PREF (`filterapi/filterapi.go:191-197`, `:220-224`)
5. `buildModifiedPayload` writes the per-destination wire

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ per-peer runtime | auto `FilterRef` appended at parse time, frozen onto `PeerSettings` | [ ] |
| Wire ↔ per-destination buffer | `buildModifiedPayload` vs `wireu.rewriteASPathPrepend`'s existing per-destination copy | [ ] |
| Shared received wire ↔ RS-client fast path | `forward_rs.go:351-360` hands out the SHARED wire with no per-destination buffer (R-2) | [ ] |

### Integration Points
- `internal/component/bgp/config/peers.go:643-673` (`prependDefaultFilters`) - the precedent to mirror onto the export side; it is import-only today (`:670`)
- `internal/component/bgp/reactor/filter_delta.go:536-558` (`ExtractASPathPrependOps`) - already builds an AS_SEQUENCE of N x localAS; the prepend auto-filter should reuse it, not re-implement it
- `internal/component/bgp/reactor/filter_delta_handlers.go:91` - `{attribute.AttrLocalPref, 0x40}` handler already registered; `AttrModSuppress` needs no new plumbing
- `internal/component/bgp/wireu/aspath_rewrite.go:63` (`rewriteASPathPrepend`) - what this design retires or bypasses for EBGP prepend; **three in-progress specs are editing it** (see Convergence)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IBGP peer sends a route with LOCAL_PREF; ze forwards to an EBGP peer | → | [auto LOCAL_PREF suppress entry] | [fill during design -- MUST be RED before the fix] |
| API `update text` announces to an EBGP peer | → | [auto prepend entry] | [fill during design] |
| Route forwarded from an EBGP peer to an EBGP peer | → | [auto prepend entry] | `remove-private-as-export.ci` (must stay GREEN) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IBGP-sourced route with LOCAL_PREF forwarded to an EBGP peer | LOCAL_PREF is absent from the wire (RFC 4271 §5.1.5) |
| AC-2 | Any route to an IBGP peer | LOCAL_PREF unchanged; `attributes.ci` stays byte-exact |
| AC-3 | Route to an EBGP peer via the forwarded path | Local ASN prepended exactly as today, including every `local-as` override mode |
| AC-4 | Route to an EBGP peer via the originated/injected path | Same prepend outcome as AC-3 (the two paths agree) |
| AC-5 | `remove-private-as:STRIP` configured on an EBGP peer | Strip still precedes prepend; `remove-private-as-export.ci` AS_PATH byte-exact |
| AC-6 | Operator inspects the peer's export chain | The auto entries are discoverable and their origin is unambiguous |
| AC-7 | EBGP RS-client peer | Defined behavior, not an accident (see R-2) |
| AC-8 | Benchmark, forwarded EBGP path | No added allocation per destination vs the `rewriteASPathPrepend` baseline (A-2) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill during design] | `internal/component/bgp/config/peers_test.go` | auto entries appended for EBGP peers only, and not duplicated when user-configured | |
| [fill during design] | `internal/component/bgp/reactor/filter_delta_test.go` | `AttrModSuppress` on `AttrLocalPref` removes the attribute | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| **[the A-3 proof]** | `test/plugin/*.ci` | IBGP source -> EBGP destination: LOCAL_PREF must not appear on the wire. **RED before the fix** | |
| `remove-private-as-export` | `test/plugin/remove-private-as-export.ci` | strip-then-prepend unchanged | |
| `attributes` | `test/plugin/attributes.ci` | IBGP baseline unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [fill during design] | `test/interop/scenarios/` | FRR or BIRD | a real EBGP peer sees no LOCAL_PREF and a correct AS_PATH | |

## Files to Modify
- `internal/component/bgp/config/peers.go` - the auto-append, mirroring `prependDefaultFilters` (`:643-673`) onto the export side; `ValidateFilterNames` (`:184-191`) and `canonicalizeFilterRefs` (`:200-203`) must accept the reserved names
- `internal/component/bgp/reactor/filter_ordered.go` - ordering of the auto entries within the chain
- `internal/component/bgp/wireu/aspath_rewrite.go` - only if prepend is relocated (**coordinate: three in-progress specs, see Convergence**)
- `internal/core/bgp/attribute/builder.go` - `:454-457`'s comment promises a send-time eBGP filter that does not exist; make it true or fix the comment

## Implementation Steps

### Implementation Phases

Blocked on Thomas sequencing this against the three in-progress specs (R-1), so phases are
sketched, not committed. The ordering below is what the trace already justifies:

0. **Phase 0: sequencing + the hook proof.** Confirm the sequencing call. Then settle A-1
   with a test before building anything: does an auto `FilterRef` on `ExportFilters`
   actually reach BOTH paths? If it does not, the design is a silent no-op on originated
   routes -- the exact bug class `spec-fixit-private-asn-leak` just fixed. Also resolve R-4:
   establish what `update text` vs `rib/commit` should each do, since they disagree today.
1. **Phase 1: Wiring (MANDATORY FIRST)** — the RED functional test for AC-1: IBGP source
   carrying LOCAL_PREF, forwarded to an EBGP destination, must not put LOCAL_PREF on the
   wire. **This test does not exist and the violation is only grep-established** (A-3), so
   this is the phase that turns a claim into a fact.
   - Verify: RED before any fix, for the right reason (LOCAL_PREF visible on the wire)
2. **Phase 2: the LOCAL_PREF auto entry** — `AttrModSuppress` on `AttrLocalPref`, EBGP-gated,
   auto-appended at config time. Smaller and independent of the prepend relocation, so it
   can land alone and close a live RFC MUST NOT.
   - Verify: Phase 1 goes GREEN; `attributes.ci` (IBGP baseline) stays byte-exact
3. **Phase 3: the prepend auto entry** — the contentious half: it touches the seam three
   other specs own. Must reproduce every `local-as` override mode (`localASNoPrepend`,
   `localASReplaceAS`, `asOverride`) and keep strip-before-prepend ordering.
   - Verify: `remove-private-as-export.ci` AS_PATH byte-exact; benchmark vs A-2
4. **Phase 4: retire the old path** — per `ai/rules/no-layering.md`, if prepend moves, the
   old implementation is deleted, not left beside the new one.
5. **Functional + interop tests** → a real FRR/BIRD peer sees no LOCAL_PREF and a correct AS_PATH
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit tables, learned summary, two-commit closure

### Failure Routing
| Failure | Route To |
|---------|----------|
| The auto entry does not fire on the originated path | STOP. A-1 is broken and the design is wrong; do not paper over it with a second hook |
| Benchmark shows an added per-destination copy | A-2 broken → back to design; the chain may not be the right home for prepend |
| An RS-client test breaks | R-2: this is the conformance-vs-zero-copy call, and it is Thomas's, not the implementer's |
| `remove-private-as-export.ci` AS_PATH changes | You reordered strip and prepend. The test is right; the chain order is wrong |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (RFC 4271 S5.1.5 / S9.1.2 against a real peer)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations
- The RS-client zero-copy tradeoff (R-2) is inherited, not solved here.
- Sequencing against the three in-progress specs is Thomas's call and gates this leaving `skeleton`.
- The RFC 4271 S5.1.5 violation is grep-established, not test-established (A-3). Until Phase 1's RED test exists, it is a strong claim, not a proven one.
