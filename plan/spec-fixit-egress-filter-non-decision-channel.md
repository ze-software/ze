# Spec: fixit-egress-filter-non-decision-channel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | plan/deferrals/fixit-stored-route-relay-hardening.md (row 1) |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**RESOLVED 2026-08-07. Implementation is complete; the Review Gate and closure
are the phases still owed.** Both questions the spec was opened for now have
answers, and both are recorded below in "Outcome".

## Outcome (2026-08-07)

**The ordering question: the window IS reachable, and ordering does not close
it.** `filterapi.Register` puts `LLGREgressFilter` into the egress pipeline from
`init()` in `internal/component/bgp/plugins/gr/register.go`, and
`internal/component/plugin/all/all_ze_bgp.go` imports the package for every
`ze_bgp` build, so the filter is live from process start. The only non-test store
of `egressState` is `setEgressState`, called from `RunGRPlugin`'s `OnConfigure`
callback in `internal/component/bgp/plugins/gr/gr.go`. Nothing sequences the
registration against the store. The gap is not merely a startup race either: when
the GR plugin engine does not run in this process, `egressState` is nil for the
whole process lifetime while the filter stays registered and stays called. So the
spec does not close "with a test proving the window is unreachable"; it closes
with the answer implemented.

**The RFC question: fail CLOSED.** RFC 9494 Section 4.3 keys the decision on
whether the capability "has been received" from the neighbor. With no state
loaded it has not been received from anyone, so `hasLLGR=false` is the literal
reading rather than a defensive choice. The destination then takes the treatment
already written for a peer known not to have advertised it: withdraw for EBGP,
Section 4.6 NO_EXPORT + LOCAL_PREF=0 for IBGP. No new branch was invented; the
nil state simply stops short-circuiting past the existing ones.

This needed no owner ruling. Full conformance plus a tagged test was reachable,
which `ai/rules/rfc-compliance.md` makes the answer rather than a question. The
two directions are not symmetric: failing closed costs a transient withdraw or
depreference toward a peer that turns out to be LLGR-capable, repaired the moment
`OnConfigure` stores the state, while failing open puts a long-lived stale route
into a neighbor that never agreed to hold one, at normal preference, which is the
risk RFC 9494 Section 5.2 describes.

**Shape of the fix** (`internal/component/bgp/plugins/gr/gr_egress.go`):
the `staleLevel == 0` fast path moved ABOVE the state load, since it needs no
state and now answers the common case more cheaply than before. A latched WARN
(`egressStateMissingWarned`) says the state is unloaded once per process,
matching `recordDrop` in the sibling `role` plugin rather than inventing a shape.

**The WARN was unhearable at first, and that is fixed (2026-08-07).** A reviewer
found that every caller of `SetLogger` is on the ENGINE path
(`ConfigureEngineLogger` and the CLI `ConfigLogger`, both in `register.go`), so in
the case the warning exists for -- the engine never runs and the state is nil for
the whole process -- `logger()` was still `init()`'s discard logger. The latch was
spent on a dropped line, and no later occurrence could speak. Failing closed held;
the "or say something" half of `ai/rules/evidence.md` did not. `egressWarnLogger`
now routes through `slogutil.Logger(grSubsystem)` when `SetLogger` never ran, and
through the engine's own logger when it did, so an operator who silenced the
subsystem still gets silence. `loggerConfigured` (gr.go) is what tells the two
apart.

**A test pinned the violation and was replaced.** `TestLLGREgressFilter_NilState`
asserted a stale route to an EBGP destination passing through with
`mods.Len() == 0`, which is the Section 4.3 violation with a green bar on top
(`ai/rules/rfc-compliance.md`). Its stated purpose, preventing a nil-pointer
panic, still holds and is still covered.

**Proof.** `RFC9494-4.3-3` was unproven in `ai/RFC-REQUIREMENTS.md` (`--` in both
polarity columns) and now carries both:

| Test (`internal/component/bgp/plugins/gr/gr_egress_test.go`) | Proves |
|---|---|
| `TestLLGREgressFilter_NilStateWithdrawsEBGP` | RFC9494-4.3-3 positive: unloaded state withdraws rather than advertises |
| `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` | RFC9494-4.3-3 negative: a recorded LLGR peer still receives the route unmodified |
| `TestLLGREgressFilter_NilStateDepreferencesIBGP` | RFC9494-4.6-2 / 4.6-3 positives under an unloaded state |
| `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute` | bounds the fix: a fresh route is never suppressed |
| `TestLLGREgressWarnLoggerIsLiveWhenEngineNeverStarted` | the WARN reaches a live logger when the engine never ran |
| `TestLLGREgressWarnLoggerRespectsEngineChoice` | the fallback does not override the engine's own logger |
| `TestLLGREgressFilterWarnsWhenStateMissing` | the warning is actually emitted, and latched to one line |

Both nil-state tests were verified RED with the fix reverted:
`NilStateWithdrawsEBGP` failed on `mods.IsWithdraw()`, and
`NilStateDepreferencesIBGP` failed on both the NO_EXPORT and the LOCAL_PREF
assertion. The two contrast tests stayed green under the revert, which is what
makes them contrasts rather than duplicates.

Gates: `make ze-test-pkg PKG=./internal/component/bgp/plugins/gr` green,
`golangci-lint run ./internal/component/bgp/plugins/gr` reports 0 issues,
`make ze-rfc-check` green after `make ze-rfc-index`.

**Provenance:** deferred from `plan/spec-fixit-stored-route-relay-hardening.md`
(R6-1) on 2026-08-03. Rows live in
`plan/deferrals/fixit-stored-route-relay-hardening.md`.

**Scope after 2026-08-03: item 1 only, and item 1 is the whole spec.** Items 2, 3
and 4 below are CLOSED and are kept only as the record of why this spec is now
narrow. Items 2 and 4 landed in `plan/spec-fixit-stored-route-relay-hardening.md`
under its rewritten AC-7; item 3 was the `EgressFilterFunc` signature change,
which that rewrite replaced, so it is owed by nobody.

What remains is one question: **should `LLGREgressFilter` accept when its plugin
state is not loaded?** That is an RFC 9494 fail-open/fail-closed decision plus
the ordering question of whether the window is reachable at all. If the window
turns out to be unreachable, this spec closes with a test proving it and no
behavior change.

An in-process BGP egress filter answers with a bare `bool`. Three rails consume
that answer, and each one loses a different part of it. This spec owns what is
left after Thomas ruled Q-1 on 2026-08-03 ("an unrecorded destination role keeps
matching `export { unknown }`"), which closed R6-1's original example and left
these three:

**1. `LLGREgressFilter` accepts when it cannot evaluate.** `egressState.Load()`
returns nil before `RunGRPlugin` stores the state, and the filter returns true
(`internal/component/bgp/plugins/gr/gr_egress.go`). The destination's LLGR
capability is exactly what the nil state withholds, so a stale route goes out
unmodified toward a peer that may not have advertised LLGR. RFC 9494 Section 4.3
says such a route SHOULD NOT be advertised, and Section 4.6's iBGP depreference
is skipped too. Decide fail-open versus fail-closed for that window, and prove
the window is either closed by ordering (state stored before the filter can be
called) or handled by the answer.

**2. Two rails discard the failure signal that already exists. — DONE
elsewhere, 2026-08-03.** `safeEgressFilter`
(`internal/component/bgp/reactor/reactor_notify.go`) returns `(accept, panicked)`
and `forward_rs.go` and `decideStaleReadvertise`
(`internal/component/bgp/reactor/reactor_api_batch.go`) both discarded
`panicked`. Thomas replaced AC-7 of
`plan/spec-fixit-stored-route-relay-hardening.md` with this work, so it landed
there: the RS rail now hands a panicked destination to the plugin rail through
`FastPathSkipped`, and the readvertise rail returns `staleFilterFailed` so the
batch reports `errStaleReadvertiseFilterPanic` rather than
`ErrNoPeersAcceptedFamily`.
Proof: `internal/component/bgp/reactor/egress_filter_failure_test.go`.

**3. Whether `filterapi.EgressFilterFunc` needs a second return at all. — CLOSED
as not owed, 2026-08-03.** This was AC-7 of the source spec. Its verdict there,
with all three registered filters read, was NOT owed: none has a state the second
return would newly express. Thomas did not keep the signature change; he replaced
AC-7 with item 2. So the seam keeps its bare `bool`, and the `otc_test.go` /
`config_test.go` edits it would have dragged in are not needed.

**What is left here is item 1**, which is an RFC 9494 fail-open decision plus a
startup-ordering question, independent of both closed items.

**4. `decideStaleReadvertise`'s OTHER failure branch. — DONE elsewhere,
2026-08-03.** When `buildModifiedPayload` refused the LLGR depreference mods the
function returned `staleSuppress`, reporting a failure to REALIZE the filter's
decision as the filter deciding to drop the route. Fixed alongside item 2 in
`plan/spec-fixit-stored-route-relay-hardening.md` rather than deferred here: it
returns `staleBuildFailed`, and the sentinel became `errStaleReadvertiseWithheld`
wrapped by `errStaleReadvertiseFilterPanic` and `errStaleReadvertiseBuildFailed`.
`recordModifyFailureAddr` is untouched.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `plan/spec-fixit-stored-route-relay-hardening.md` -- R6-1, Q-1's ruling, and the AC-7 reassessment that produced this spec

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` -- LLGR: Section 4.3 (do not advertise LLGR_STALE to
      a peer that did not advertise the capability) and Section 4.6 (the optional
      iBGP partial-deployment depreference)
- [ ] `rfc/short/rfc9234.md` -- BGP Roles and OTC, for the egress filter that
      shares the seam

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` -- `LLGREgressFilter`,
      `egressState`, `setEgressState`, `staleFromMeta`
- [ ] `internal/component/bgp/reactor/reactor_notify.go` -- `safeEgressFilter`
      and its `panicked` return
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- `forwardUpdateCore`,
      `egressStepResult`, `suppressedCount`, `errAllDestinationsSuppressed`
- [ ] `internal/component/bgp/reactor/forward_rs.go` -- the RS fanout call site
      that discards `panicked`
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` -- `decideStaleReadvertise`,
      `staleOutcome`, the second call site that discards it
- [ ] `internal/component/bgp/filterapi/filterapi.go` -- `EgressFilterFunc` and
      its KNOWN GAP note

**Behavior to preserve:**
- `forwardUpdateCore`'s existing classification: only a genuine policy decision
  increments `suppressedCount`, so `errAllDestinationsSuppressed` keeps meaning
  "handled" for `RelayStoredRoute`.
- Q-1's ruling: an unrecorded destination role keeps matching an explicit
  `export { unknown }`. Nothing here reopens it.
- The RFC 9234 assertions in `otc_test.go` and `config_test.go` stay where they
  are unless Thomas approves moving them.

**Behavior to change:**
- A stale route must not go out unevaluated because the GR plugin state is not
  loaded yet.

**Behavior to preserve (landed 2026-08-03, item 2):** a filter panic on the RS
fanout and on the stale re-advertise rail is reported as a failure, not as
policy. Do not re-collapse the two while changing the LLGR answer.

## Data Flow

### Entry Point
An UPDATE reaches the reactor forward rail, or the readvertise rail runs after a
GR restart. Both fan out per destination peer and call the registered in-process
egress filters.

### Transformation Path
1. `forwardUpdateCore` walks `orderedEgressSteps` per destination, calling
   `safeEgressFilter` for each in-process step and folding the answer into
   `egressStepResult{accept, failed}`.
2. `forward_rs.go` runs the same filters on the route-server fanout, discarding
   `panicked`.
3. `decideStaleReadvertise` runs `readvertiseEgressFilters` over a packed
   announce body and maps the answer to a `staleOutcome`, discarding `panicked`.
4. `RelayStoredRoute` reads `errAllDestinationsSuppressed` as a handled route,
   which is why the accept/suppress/fail distinction has to survive step 1.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gr plugin -> reactor | `filterapi` registration, `LLGREgressFilter` | [ ] |
| reactor forward rail -> relay completeness count | `errAllDestinationsSuppressed` | [ ] |
| reactor -> session write | pre-filtered forward, no re-gate | [ ] |

### Integration Points
- `filterapi.EgressFilterFunc` (`internal/component/bgp/filterapi/filterapi.go`)
- `safeEgressFilter` (`internal/component/bgp/reactor/reactor_notify.go`)
- `LLGREgressFilter` (`internal/component/bgp/plugins/gr/gr_egress.go`)

## Wiring Test
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A stale route reaches the egress pipeline while `egressState` is unloaded | -> | `LLGREgressFilter` nil-state answer | `TestLLGREgressFilter_NilStateWithdrawsEBGP`, `TestLLGREgressFilter_NilStateDepreferencesIBGP` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLLGREgressFilter_NilStateWithdrawsEBGP` | `internal/component/bgp/plugins/gr/gr_egress_test.go` | RFC9494-4.3-3 positive: an unloaded state withdraws instead of advertising | done, RED when reverted |
| `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` | same | RFC9494-4.3-3 negative: a recorded LLGR peer still gets the route unmodified | done |
| `TestLLGREgressFilter_NilStateDepreferencesIBGP` | same | RFC9494-4.6-2 / 4.6-3 positives under an unloaded state | done, RED when reverted |
| `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute` | same | the fix is bounded to stale routes | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| LLGR stale readvertise before plugin state load | `test/plugin/*.ci` | a peer restarts and stale routes are readvertised while the GR plugin is still starting | **WITHDRAWN 2026-08-07: not implementable, and nothing is owed later** -- see the decision below |

### Scope decision: no functional `.ci` for the nil-state window (2026-08-07)

**Decision: this row is withdrawn. Nothing is owed later and nothing is
deferred.** It was a skeleton-era placeholder written before anyone had read the
producer. It carries no `plan/deferrals/` shard on purpose: a shard records work
that is still owed, and recording this one would create a backlog row that can
never be closed.

**Evidence.** The state the test would have to stage is `egressState` being nil.
That pointer is package-private to `internal/component/bgp/plugins/gr`, and its
only non-test writer is `setEgressState`, called from `RunGRPlugin`'s
`OnConfigure` callback. A `.ci` drives the built `ze` binary from outside the
process and has no operator-facing way to unload, delay, or observe that pointer.
Staging the window end to end would therefore require a new hook whose only
purpose is to suppress or delay the egress state, which is a production
backdoor into filter state: a control that could disable an RFC 9494 gate at
runtime. Building it would create a worse defect than the one this spec fixed.

**What covers the behavior instead.** The unit tests drive `LLGREgressFilter`
itself, which is the function that PRODUCES the behavior rather than a caller of
it (`ai/rules/evidence.md`), and each fails when the fix is reverted. The
observed reds are recorded in "Outcome" above. `ai/rules/testing.md`'s functional
requirement is about proving a user-visible workflow; the workflow here is the
ordinary LLGR stale readvertise, which the existing `RFC9494-4.6-*` coverage
already exercises with the state loaded. What is new is one branch inside that
function, and the unit tier reaches it exactly.

**Not a coverage reduction** (`ai/rules/completion.md`): `RFC9494-4.3-3` had no
test at all before this work and now has both polarities. Nothing lost a tier.
`check_evidence_ratchet` agrees -- `make ze-rfc-check` is green.

**If the reviewer disagrees**, the fix is a purpose-built startup-ordering seam,
designed as such and reviewed on its own merits, not a test-only backdoor bolted
onto the filter. That would be a separate spec.

## Files to Modify
- `internal/component/bgp/plugins/gr/gr_egress.go` -- the nil-state answer

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. RFC
9494 Sections 4.3 and 4.6 govern the LLGR half.

## Implementation Steps

1. Read the producer; confirm the nil-state answer against the tree before
   writing anything.
2. Establish whether the window is reachable at all (is the egress state stored
   before any peer can be readvertised to?).
3. Decide and implement the LLGR nil-state answer, with the RFC 9494 quote above
   the code, with a failing test first.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
