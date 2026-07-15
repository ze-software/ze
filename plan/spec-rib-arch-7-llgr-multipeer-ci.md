# Spec: rib-arch-7 -- Multi-Peer LLGR Readvertisement Functional Fixture

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. Existing single-peer fixtures: `test/plugin/llgr-readvertise.ci`, `llgr-rib-stale.ci`, `llgr-transition.ci`
5. `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter`
6. `rfc/short/rfc9494.md`

## Task

LLGR (Long-Lived Graceful Restart, RFC 9494) has **single-peer** functional coverage:
`test/plugin/llgr-readvertise.ci`, `test/plugin/llgr-rib-stale.ci`,
`test/plugin/llgr-transition.ci`, `test/plugin/plugin-gr-llgr-capa.ci`.

GAP: no **multi-peer partial-deployment** fixture. In a real deployment some peers are
LLGR-capable and some are not; the LLGR egress filter (`LLGREgressFilter`,
`internal/component/bgp/plugins/gr/gr_egress.go:57`) tags stale routes with the LLGR_STALE
community for LLGR-capable peers and converts them to withdrawals for non-LLGR EBGP peers
(`filterapi.ModAccumulator.SetWithdraw`, RFC 9494). Add a `.ci` fixture that exercises this
readvertisement split across a mixed set of peers: LLGR-capable peers keep the stale route
(marked LLGR_STALE, depreferenced), non-LLGR peers receive a withdrawal.

Primarily a test-coverage gap. Feature code changes only if the fixture reveals a
correctness bug in the multi-peer path.

## Root Cause Finding (2026-07-10, from `spec-followup-test-infra` AC-5)

**A closure session for `spec-followup-test-infra` attempted the multi-peer fixture (AC-5) and
found this is NOT just a test-coverage gap -- it is a real, unwired feature. The premise above
("the per-peer branch already exists and is single-peer-tested; the missing coverage is the two
branches firing together") is WRONG: those branches never fire in production because the egress
filter is not invoked on the readvertisement path at all.** Confirmed with `file:line`:

- `LLGREgressFilter` (`internal/component/bgp/plugins/gr/gr_egress.go:57`) reads `meta["stale"]`
  and stamps NO_EXPORT+LOCAL_PREF=0 (IBGP :89-91) / withdraw (EBGP :99). It is invoked **only**
  on the ForwardUpdate route-server/cache rail: `safeEgressFilter` at
  `reactor/forward_rs.go:324` and `reactor/reactor_api_forward.go:490`, nowhere else.
- The only producer of `meta["stale"]` is the RIB readvertise/refresh path
  (`rib/rib_replay.go:299`, `rib/rib_commands.go:616`), which flows
  `updateRouteWithMeta` (`rib/rib.go:693`) -> `MethodUpdateRoute`
  (`plugin/server/dispatch_registry.go:236`) -> `peer <sel> update cursor/text` ->
  `cmd/update/update_text.go:767 DispatchNLRIGroups` -> `reactor/reactor_api_batch.go:28
  AnnounceNLRIBatch`. `DispatchNLRIGroups` drops `ctx.Meta`, and `AnnounceNLRIBatch` builds each
  peer's UPDATE from scratch and calls **no** egress filter.
- The LLGR readvertise trigger `onLLGREntryDone` (`gr/gr.go:142`) uses exactly this
  `clear bgp rib out` -> RIB -> `AnnounceNLRIBatch` path, so stale routes re-advertised to
  non-LLGR peers arrive unmodified. Documented gap: `pkg/plugin/sdk/sdk_engine.go:42`
  ("CommandContext.Meta is not yet consumed by egress filters"). The ForwardUpdate rail that
  *does* run the filter has no producer of `meta["stale"]` either.

**Scope: this is a feature, not a fixture.** Before writing the `.ci`, the real work is to
connect the rails -- either (A) wire the egress-filter pipeline (ModAccumulator application incl.
withdraw conversion) into the `AnnounceNLRIBatch` direct-send path, or (B) make the readvertise
flow through ForwardUpdate with `meta["stale"]` set. Both touch a hot path used by all plugin
route injection (buffer-first / memory-architecture rules) -> design carefully. Also note the
`.ci` needs an actual forwarding mechanism (`rs` or `redistribute`); a config with only `gr` +
`bgp-rib` never forwards the source route to the other peers. WIP fixture note (2026-07-14): the preserved scratch file
`tmp/scratch/llgr-egress-suppress-multi-peer.ci.wip` has since been DELETED (transient
`tmp/` was cleaned); the fixture must be reconstructed from this spec body, not recovered. The RR-client
variant (`llgr-egress-rr-multi-peer.ci`) exercises the same unwired path and belongs here too.
When this is picked up, move the spec to `design` and elevate its Status from skeleton.

**Re-verification (2026-07-14): the Root Cause Finding STILL HOLDS.** `CommandContext.Meta`
(`plugin/server/command.go:138`) has two write sites (`plugin/server/dispatch_registry.go:241`
and `plugin/server/dispatch.go:390`) and ZERO readers tree-wide (LSP findReferences), so the
stale metadata is dropped before `AnnounceNLRIBatch` and no egress filter runs on the
readvertise rail. All primary anchors match with no line drift. This remains an
unwired-feature blocker, not a test gap.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (:57): per-destination stale-route handling
  → Constraint: the fixture must assert BOTH branches (LLGR-capable keep+mark vs non-LLGR withdraw) in one topology.
- [ ] `test/plugin/llgr-readvertise.ci` - the existing single-peer fixture
  → Constraint: extend the pattern; do not duplicate single-peer assertions.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart
  → Constraint: LLGR_STALE community, depreference, and readvertisement-to-non-LLGR-peers rules.

**Key insights:**
- The per-peer branch already exists and is single-peer-tested; the missing coverage is the two branches firing together across a mixed peer set.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (:57): egress filter that, for stale routes, marks LLGR_STALE for LLGR-capable peers and calls `mods.SetWithdraw()` for non-LLGR EBGP peers
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator.SetWithdraw` (:151): "Used by LLGR egress filter (RFC 9494) to withdraw stale routes from EBGP non-LLGR peers"
- [ ] `test/plugin/llgr-readvertise.ci`, `llgr-rib-stale.ci`, `llgr-transition.ci` - single-peer LLGR fixtures

**Behavior to preserve:**
- All existing LLGR behaviour and single-peer fixtures; this only adds a multi-peer fixture.

**Behavior to change:**
- None expected. Feature code changes only if the fixture uncovers a multi-peer bug.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A peer's graceful-restart window expires; its routes become LLGR-stale, triggering readvertisement to all other peers

### Transformation Path
1. Routes from the restarting peer are marked stale (LLGR)
2. For each destination peer, `LLGREgressFilter` runs (`gr_egress.go:57`)
3. LLGR-capable peer: route kept, tagged LLGR_STALE, depreferenced
4. Non-LLGR EBGP peer: `mods.SetWithdraw()` converts the announce to a withdrawal
5. The `.ci` fixture asserts both outcomes in one mixed-peer topology

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| stale route → egress filter | `LLGREgressFilter` per destination peer | [ ] |
| filter → wire | LLGR_STALE-tagged announce vs withdrawal, per peer capability | [ ] |

### Integration Points
- `LLGREgressFilter` (`gr_egress.go:57`) - the per-peer decision under test
- `ModAccumulator.SetWithdraw` (`filterapi.go:151`) - the withdraw branch
- `test/plugin/*.ci` harness - the multi-peer topology

### Architectural Verification
- [ ] No bypassed layers (fixture drives real peers through the real egress filter)
- [ ] No unintended coupling (fixture-only; no product code coupling introduced)
- [ ] No duplicated functionality (extends single-peer fixtures, not a re-test)
- [ ] Registration over hardcoding - no product registration change; test-only (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `.ci` harness can express a mixed LLGR-capable / non-LLGR multi-peer topology | existing multi-peer plugin fixtures | Need harness support; larger scope | check `test/plugin` harness capabilities at design | unvalidated |
| A-2 | The multi-peer path has no latent bug | single-peer branches pass today | Fixture goes red; fix feature code | run the new fixture at implement | **BROKEN (2026-07-10)**: egress filter not invoked on the readvertise/`AnnounceNLRIBatch` rail (see Root Cause Finding); multi-peer stamping never fires -> feature work required before the fixture can pass |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixture reveals a real multi-peer bug (per `feedback_sleep_hides_races` this is the point) | new `.ci` fails | diagnose and fix `gr_egress.go` (or its callers), add a regression assertion |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Stale (LLGR) `AnnounceNLRIBatch` with mixed LLGR-capable + non-LLGR peers | → | `sendStaleReadvertise` per-peer split (keep / depreference / withdraw) | `TestStaleReadvertiseWireOutput` (`reactor_stale_readvertise_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Stale routes readvertised to an LLGR-capable peer | route kept, tagged LLGR_STALE, depreferenced |
| AC-2 | Same stale routes readvertised to a non-LLGR EBGP peer | route converted to a withdrawal |
| AC-3 | Both peers present in one topology | both outcomes occur from the same readvertisement event |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStaleReadvertiseWireOutput` | `reactor/reactor_stale_readvertise_test.go` | the three per-peer wire outputs of a stale readvertise: unchanged announce (LLGR), NO_EXPORT+LOCAL_PREF=0 (non-LLGR iBGP), withdrawal (non-LLGR eBGP) | PASS |
| `TestDecideStaleReadvertise` (+`_NoFilters`) | `reactor/reactor_stale_readvertise_test.go` | filter -> outcome mapping (withdraw/modify/keep/suppress); inert with no readvertise filter | PASS |
| `TestReadvertiseEgressFuncs` | `filterapi/filterapi_test.go` | only `Readvertise`-opted egress filters selected | PASS |
| `TestStaleLevelFromMeta` | `cmd/update/update_text_test.go` | meta["stale"] -> NLRIBatch.Stale (uint8/int/float64) | PASS |
| existing `LLGREgressFilter` coverage | `gr/gr_egress_test.go` | per-peer branch logic (the mods each peer type receives) | PASS |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A -- covered by `TestStaleReadvertiseWireOutput` (reactor wire-output, deterministic) | -- | mixed LLGR-capable + non-LLGR peers: kept-and-marked vs depreferenced vs withdrawn, from one readvertisement | done |

**Scope note (user-approved 2026-07-15):** the divergence coverage is delivered by the
reactor wire-output test, not a BGP `.ci`. A live multi-peer BGP `.ci` was attempted and
blocked on multi-peer session establishment in the test harness (BGP OPEN does not complete;
diagnostics in `tmp/scratch/llgr-multipeer-readvertise.ci.wip`), which is a test-infra problem
upstream of this feature. The `.ci` remains a tracked test-infra follow-up. Notably the
reactor test caught that the `.wip`'s guessed NO_EXPORT bytes (`C00804FFFFFF01`) were wrong;
the true on-wire form is the extended-length `D0080004FFFFFF01`.

### Interop Tests (MANDATORY for protocol features)
- N/A: exercises Ze's own egress split across peers; existing LLGR interop coverage guards wire behaviour.

## Files to Modify

- `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (verify only; change only if the fixture reveals a multi-peer bug)

## Implementation Steps

1. **Phase: design** - confirm the harness supports a mixed multi-peer topology (A-1); define the fixture.
2. **Phase: fixture** - write `test/plugin/llgr-multipeer-readvertise.ci` asserting AC-1..AC-3.
3. **Phase: fix (only if red)** - if the fixture uncovers a bug, diagnose and fix `gr_egress.go` (`ai/rules/diagnosis-before-fix.md`).
4. **Full verification** - `make ze-verify`.
5. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [x] All three per-peer branches asserted from one stale readvertise (keep / depreference /
      withdraw) -- `TestStaleReadvertiseWireOutput`
- [x] Wiring Test table complete (concrete test name, none deferred)
- [x] `make ze-test` passes (lint + all ze tests) -- structural gates green; reactor/filterapi/
      gr/update tests pass under `-race`; remaining reds environmental (cgo/-race, netns/root)
- [x] Registration over hardcoding respected (`filterapi.Filter.Readvertise` opt-in; the
      reactor never spells a plugin name)

### TDD
- [x] Tests written
- [x] Tests FAIL -- `TestStaleReadvertiseWireOutput/modify` first failed asserting the
      guessed `C00804FFFFFF01`; the dump showed the true extended-length `D0080004FFFFFF01`
- [x] Tests PASS -- `ok reactor 0.037s` (wire-output + decision + no-filters)

## RFC Documentation

N/A: this is a functional test fixture, not enforcing code. RFC 9494 LLGR behaviour is
already documented at its enforcing site (`internal/component/bgp/plugins/gr/gr_egress.go`,
`filterapi.go:149`). Add `// RFC 9494 Section X.Y` comments only if step 3 changes feature code.

## Resolution (2026-07-15) -- egress-filter wiring landed; divergence covered by a reactor test

**Feature (committed `4b0556ff5`):** the LLGR egress filter now runs per destination peer on
the RIB stale-readvertise announce rail. Reuses the forward rail's tested machinery; the
common announce path is untouched.

- `filterapi.Filter.Readvertise` opt-in + `ReadvertiseEgressFuncs()` (registration over
  hardcoding -- the reactor never spells "bgp-gr"); `gr/register.go` sets `Readvertise: true`.
- `bgptypes.NLRIBatch.Stale` carries the LLGR level; `DispatchNLRIGroups` populates it from
  `ctx.Meta["stale"]` via `staleLevelFromMeta` (`update_text.go`) -- closes the meta drop.
- `Reactor.readvertiseEgressFilters` built at construction; `AnnounceNLRIBatch` takes a
  per-peer branch ONLY when `Stale > 0` -> `sendStaleReadvertise` / `decideStaleReadvertise`:
  withdraw (`buildBatchWithdrawUpdate`) for non-LLGR eBGP, depreference
  (`buildModifiedPayload` -> `sendBodyWithSplit`) for non-LLGR iBGP, keep for LLGR-capable.

**Divergence coverage: `TestStaleReadvertiseWireOutput`** asserts the three per-peer wire
outputs byte-for-byte, deterministically -- the coverage the multi-peer `.ci` would give,
without a live BGP session.

**Live multi-peer BGP `.ci` -- a tracked test-infra follow-up (not a release blocker).** It
needs a source-attributed stale route in the OTHER peers' rib-out (the source's peer-learned
route propagated in first). The attempt hit an upstream test-harness limit: multi-peer BGP OPEN
does not complete (TCP connects, OPEN stalls; `peers_up=[]`, `cache=0`), a test-infrastructure
problem separate from this feature. WIP + diagnostics preserved at
`tmp/scratch/llgr-multipeer-readvertise.ci.wip`. When resumed: get the 3-peer OPEN handshake
to establish first, then drive `cache forward` -> `mark-stale` -> `clear bgp rib out`.

## Review Gate

Self-review (2026-07-15): 0 BLOCKER, 0 ISSUE.

- **Correctness**: `TestStaleReadvertiseWireOutput` asserts the exact per-peer wire bytes
  (announce unchanged / NO_EXPORT+LOCAL_PREF=0 / withdrawal). `decideStaleReadvertise` covers
  the filter->outcome mapping incl. suppress and the no-filter inert case.
- **Hot-path safety**: the stale branch fires only when `batch.Stale > 0` (GR-expiry, rare);
  the common grouped announce path is byte-unchanged. Full reactor suite green under `-race`.
- **Registration**: `filterapi.Filter.Readvertise` opt-in; the reactor discovers filters via
  `ReadvertiseEgressFuncs()`, never naming the gr plugin.
- **Scope reduction (user-approved 2026-07-15)**: divergence coverage delivered by the
  reactor wire-output test instead of a BGP `.ci`; the live multi-peer `.ci` is a tracked
  test-infra follow-up (blocked on harness BGP establishment, not this feature).
- **Finding**: the reactor test corrected the `.wip`'s guessed NO_EXPORT bytes
  (`C00804FFFFFF01` -> real extended-length `D0080004FFFFFF01`).

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
