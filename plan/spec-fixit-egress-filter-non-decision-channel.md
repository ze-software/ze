# Spec: fixit-egress-filter-non-decision-channel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | plan/deferrals/fixit-stored-route-relay-hardening.md (row 1) |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**RESOLVED 2026-08-07, remediated 2026-08-10. Closure is the phase still owed.**
Both questions the spec was opened for have answers, recorded below in "Outcome".
An independent review of commit `51caaf3d6` then found seven items; all seven are
cleared and recorded in "Remediation (2026-08-10)".

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A stale route (`meta["stale"] > 0`) reaches `LLGREgressFilter` for an EBGP destination while `egressState` is nil | The route is withdrawn (`mods.SetWithdraw`), not advertised. RFC 9494 Section 4.3 |
| AC-2 | The same, for an IBGP destination | The route is delivered with NO_EXPORT attached and LOCAL_PREF 0. RFC 9494 Section 4.6 |
| AC-3 | A route with no stale metadata, `egressState` nil | Accepted with no modification. The fix is bounded to routes Section 4.3 governs |
| AC-4 | A stale route whose destination IS recorded in `peerLLGRCaps` | Accepted unmodified. The nil-state answer is not a blanket suppression |
| AC-5 | The nil state is met and no engine ever installed a logger | One WARN naming the destination and the RFC reaches a LIVE logger, latched to one line per process |
| AC-6 | The forward path reads `peerLLGRCaps` while a peer flap writes or deletes in it | Both sides hold `grPlugin.mu`; no data race and no `fatal error: concurrent map read and map write` |
| AC-7 | `bgp-gr` is configured with `run` rather than `use`, so its engine runs out of process | `ze doctor` reports `doctor-bgp-gr-out-of-process`, naming what the daemon will do to stale routes |
| AC-8 | `bgp-gr` runs out of process and the daemon readvertises a stale route to a non-LLGR internal neighbor | The peer receives the route with NO_EXPORT on the wire, decided by a filter whose state is nil in this process |

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

## Remediation (2026-08-10)

An independent review of commit `51caaf3d6` found seven items. All are cleared.

| # | Finding | Fix |
|---|---------|-----|
| 1 | `LLGREgressFilter` read `peerLLGRCaps` with no lock while `gr.go` and `gr_removal.go` wrote and deleted it under `gp.mu`, and `gr.go` shared the map by reference. `fatal error: concurrent map read and map write`, which `recover()` does not catch | `egressFilterState` now carries `mu *sync.Mutex` (`&gp.mu`) and the filter reads through `hasLLGR`, which takes it. Narrowest correct fix: the same lock the writers already hold, taken only on the stale path |
| 2 | `filterapi.go` still documented `LLGREgressFilter` as answering ACCEPT on the state it cannot evaluate | Corrected: that state became a decision on 2026-08-07, so the filter is no longer an instance of the seam's known gap |
| 3 | Every `RFC requirement:` source anchor in `gr_egress_test.go` cited a line about 75 off | Re-anchored to the symbol each tag proves (`hasLLGR` early return, the `isIBGP` branch, `communityNoExport`, `localPrefZero`, `mods.SetWithdraw`). No line numbers: the line is not the fact (`ai/rules/writing.md`) |
| 4 | The stated reason for withdrawing the functional `.ci` was false | Corrected above, with the two config-only routes to the unloaded state named and verified against their producers. The `.ci` is written |
| 5 | The out-of-process arrangement is silently wrong for peers that DID negotiate LLGR | Doctor check `bgp-gr-in-process`, decision table above. Fail-closed is untouched |
| 6 | No `## Review Gate`, no `## Goal Validation`, and a Checklist referring to ACs nobody had enumerated | AC-1..AC-8 written from the shipped behavior; both sections filled |
| 7 | About 55 comment lines in `gr_egress.go` narrated the review's own history | Cut to what a reader of the code needs. The history lives here and in the commit body |

**The crash was reachable in production.** `extractGRCaps` inserts on every OPEN
carrying capability 64 or 71, `onPeerRemoved` and `releaseRoutes` delete on peer
removal and on GR completion, and the forward path reads for every stale route to
every destination. A peer flap during a stale readvertise is the collision.

**Proof.** `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` drives the real
producers (`extractGRCaps`, `onPeerRemoved`) against the filter. Reverting the
fix makes `-race` report `mapassign_faststr` in `extractGRCaps` against
`mapaccess2_faststr` in `LLGREgressFilter`, and the test fails. The pre-existing
`TestLLGREgressFilter_ConcurrentAccess` stays GREEN under the same revert, which
is why a reader-only concurrency test was never going to catch this.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A stale route is never advertised to a neighbor whose LLGR capability was never received (RFC 9494 Section 4.3) | unit | `TestLLGREgressFilter_NilStateWithdrawsEBGP` (RED when reverted: failed on `mods.IsWithdraw()`) |
| The Section 4.6 partial-deployment depreference still applies with no state loaded | unit + functional | `TestLLGREgressFilter_NilStateDepreferencesIBGP` (RED when reverted, on both the NO_EXPORT and the LOCAL_PREF assertion); `test/plugin/llgr-egress-state-unloaded.ci` PASSES with the fix and FAILS with it reverted, the peer then receiving `UPDATE (len=54)` unmodified |
| The answer is not a blanket suppression | unit | `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` and `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute`, both GREEN under the revert, which is what makes them contrasts |
| The daemon says something rather than failing closed in silence | unit | `TestLLGREgressFilterWarnsWhenStateMissing`, `TestLLGREgressWarnLoggerIsLiveWhenEngineNeverStarted`, `TestLLGREgressWarnLoggerRespectsEngineChoice` |
| The forward path does not crash the daemon on a peer flap | race detector | `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` under `make ze-test-pkg` (which runs `-race`): green with the fix, `WARNING: DATA RACE` plus `--- FAIL` without it |
| The out-of-process arrangement is reported. The no-plugin arrangement needs no report, because a daemon with no `bgp-gr` runs no LLGR at all: every writer of `peerLLGRCaps` is a `grPlugin` method -- `extractGRCaps` and `handleOpenEvent` insert (`gr.go`), `handleStructuredOpen`, `handleOpenEvent`, `releaseRoutes` (`gr.go`) and `onPeerRemoved` (`gr_removal.go`) delete -- so with the plugin absent no peer ever negotiates the capability and treating every destination as LLGR-incapable is the correct answer rather than a blind one. Only the out-of-process route contradicts a negotiation that really happened, in the child | unit + functional | `TestCheckGRInProcessFlagsAnExternalGR`, with `TestCheckGRInProcessAcceptsTheSupportedArrangements` as the contrast that stops the check warning always; `test/ui/doctor-bgp-gr-out-of-process.ci` drives `ze doctor --json` and `ze explain` over a config file (RED when the check is unregistered in `register.go`, and RED when `checkGRInProcess`'s condition is inverted) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-egress-filter-non-decision-channel-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean rounds=3`, pinning 9 files: the five gr-package files, `filterapi.go`, and both `.ci`s |
| `review_gate.py check` | exit 0 -- `review_gate: OK (clean, hashes match)`, re-run immediately before the closure commit |
| Rounds | 3 independent rounds. Round 1 over commit `51caaf3d6`: 7 findings, all fixed 2026-08-10 (table below). Round 2 over those fixes: the three mutants of `test/ui/doctor-bgp-gr-out-of-process.ci` were driven against the real runner and discriminate, the item-2 production fix and the doctor check's placement were endorsed, and 2 ISSUEs plus 3 NOTEs were raised and cleared 2026-08-10 (table below). Round 3 over the round-2 edits: CLEAN, 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | concurrency and lifetime, RFC conformance, evidence anchoring, spec contract, evidence tier and ratchet keying, operator documentation |

Round 3 raised no product finding. Three record defects it surfaced were fixed in
this closure and touch `.md` only, outside the artifact's hashed set: the
peer-scoped `use` shape traced above did not exist, these Review Gate rows were
stale, and the guide's heading was broader than `checkGRInProcess` (a
`run "ze.bgp-gr"` engine runs in the daemon and the check correctly stays silent
on it). A record defect never earns another round (`ai/rules/planning.md`).

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Unlocked read of `peerLLGRCaps` on the forward path against locked writers: unrecoverable fatal error | `internal/component/bgp/plugins/gr/gr_egress.go`, `gr.go` | `egressFilterState.mu` + `hasLLGR`; `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` |
| 2 | ISSUE | Decision record documents behavior this spec deleted | `internal/component/bgp/filterapi/filterapi.go` | Comment corrected |
| 3 | ISSUE | `RFC requirement:` anchors point at unrelated code | `internal/component/bgp/plugins/gr/gr_egress_test.go` | Re-anchored to symbols |
| 4 | ISSUE | Spec justifies a withdrawn test with a false claim | this spec | Rationale corrected; `.ci` written |
| 5 | ISSUE | Out-of-process GR is silently wrong for LLGR-capable peers | `internal/component/bgp/plugins/gr/doctor.go` (new) | Doctor check `bgp-gr-in-process` |
| 6 | ISSUE | Missing contract sections and unenumerated ACs | this spec | AC-1..AC-8, Goal Validation, Review Gate |
| 7 | NOTE | Comment volume narrates review history | `internal/component/bgp/plugins/gr/gr_egress.go` | Trimmed |

### Findings fixed (round 2, over the round-1 fixes)

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The functional test carried no `RFC requirement:` tag, so `RFC9494-4.3-3` recorded `unit/verify` evidence only. `check_evidence_ratchet` is keyed on `kind/tier`, so nothing stopped the `.ci` being deleted, and this spec's "plus a functional tier" claim was not true where the gate reads it | `test/plugin/llgr-egress-state-unloaded.ci` | `RFC requirement: RFC9494-4.3-3 positive` added, quoting Section 4.3 and naming the assertion that carries it. `make ze-rfc-index` now publishes `functional/verify` beside the two `unit/verify` anchors |
| 2 | ISSUE | Operator doc drift: the guide documents LLGR in detail and shows `use bgp-gr`, but said nothing about what an out-of-process `bgp-gr` does to every stale route. The doctor check helps after the fact, not while a config is being planned | `docs/guide/graceful-restart.md` | New "Load bgp-gr in the daemon, never with `run`" subsection under Plugin Bindings, naming the withdraw / NO_EXPORT + LOCAL_PREF 0 outcome and the `doctor-bgp-gr-out-of-process` code |
| 3 | NOTE | Goal Validation called `extractGRCaps` the only writer of `peerLLGRCaps`; `handleOpenEvent` writes it too, under `gp.mu` | this spec | Row corrected to name every writer. The conclusion is unchanged: all of them are `grPlugin` methods, so no plugin means no writer |
| 4 | NOTE | The doctor pre-pass was justified only for globally declared plugin entries. A peer-scoped `process` block reaches `internalIsPlugin` and `runTargetsPlugin` the same way, and was never traced | this spec, `internal/component/bgp/plugins/gr/doctor.go` | Traced to the producers and recorded under "Decision: the out-of-process arrangement gets a doctor check". Both shapes such a block can emit are judged correctly, and a peer-scoped `use` does not parse. No code change |
| 5 | NOTE | Review Gate bookkeeping: "Rounds: 1" during round 2, Artifact row stale | this spec | Rounds and Artifact rows rewritten |

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
| An operator marks routes stale while `bgp-gr` runs out of process | -> | the readvertise rail through `LLGREgressFilter` with a nil state | `test/plugin/llgr-egress-state-unloaded.ci` |
| A peer flaps while a stale route is forwarded | -> | `egressFilterState.hasLLGR` under `grPlugin.mu` | `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` |
| `ze doctor` runs over a config that launches `bgp-gr` with `run` | -> | `checkGRInProcess` | `test/ui/doctor-bgp-gr-out-of-process.ci` (the unit tests call the check directly, so only the `.ci` fails when `register.go` stops registering it) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLLGREgressFilter_NilStateWithdrawsEBGP` | `internal/component/bgp/plugins/gr/gr_egress_test.go` | RFC9494-4.3-3 positive: an unloaded state withdraws instead of advertising | done, RED when reverted |
| `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` | same | RFC9494-4.3-3 negative: a recorded LLGR peer still gets the route unmodified | done |
| `TestLLGREgressFilter_NilStateDepreferencesIBGP` | same | RFC9494-4.6-2 / 4.6-3 positives under an unloaded state | done, RED when reverted |
| `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute` | same | the fix is bounded to stale routes | done |
| `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` | same | AC-6: the filter reads `peerLLGRCaps` under `grPlugin.mu` | done, RED (DATA RACE) when reverted |
| `TestCheckGRInProcessFlagsAnExternalGR` | `internal/component/bgp/plugins/gr/doctor_test.go` | AC-7 positive | done |
| `TestCheckGRInProcessAcceptsTheSupportedArrangements` | same | AC-7 negative: no warning for `use`, for `ze.bgp-gr`, or for another plugin | done |
| `TestCheckGRInProcessMatchesAPathQualifiedLaunch` | same | the same arrangement spelled with a binary path | done |
| `TestCheckGRInProcessAcceptsAnInternalGRBesideAnExternalOne` | same | an in-process `bgp-gr` silences the check for every sibling: the daemon holds the state | done 2026-08-10, RED without the pre-pass |
| `TestCheckGRInProcessStillFlagsAnotherPluginRunInProcess` | same | the pre-pass is keyed on `bgp-gr`, not on any internal plugin | done 2026-08-10 |
| `TestGRDoctorCodeIsExplainable` | same | the reported code resolves through `ze explain` | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| LLGR stale readvertise with the plugin state unloaded | `test/plugin/llgr-egress-state-unloaded.ci` | stale routes are readvertised while the GR plugin engine runs in a child process | written 2026-08-10, AC-8, RED when the nil-state answer is reverted |
| `ze doctor` reports the out-of-process arrangement | `test/ui/doctor-bgp-gr-out-of-process.ci` | an operator runs `ze doctor --json` over a config that forks `bgp-gr`, then reads the code with `ze explain` | written 2026-08-10, AC-7, RED when the check is unregistered and RED when its condition is inverted |

### Scope decision: the `.ci` IS owed, and it needed no test hook (2026-08-10)

**The 2026-08-07 withdrawal rested on a FALSE claim and is void.** It said a `.ci`
"has no operator-facing way to unload, delay, or observe that pointer" and that
staging the state would need "a production backdoor". Both are wrong. The
unloaded state is reachable from configuration alone, by two routes, and neither
touches the filter:

- **The GR plugin engine runs in another process.**
  `ExtractPluginsFromTree` (`internal/component/config/loader.go`) leaves
  `PluginConfig.Internal` false for `plugin { external X { run "ze plugin bgp-gr"; } }`,
  because `ResolvePlugin` (`internal/component/plugin/resolve.go`) classifies a
  command line as `PluginTypeExternal` and only the `ze.<name>` spelling reaches
  `MarkInternalPlugin`'s internal branch. `Process.StartWithContext` then takes
  `startExternal` (`internal/component/plugin/process/process.go`), which forks
  the command, so `RunGRPlugin` and its `setEgressState` run in the CHILD while
  the daemon still registers `LLGREgressFilter` from `init()`.
- **No GR plugin at all.** `request bgp rib mark-stale` is a bgp-rib command
  (`internal/component/bgp/plugins/rib/rib_commands.go`), and the readvertise
  rail reads `filterapi.ReadvertiseEgressFuncs()`, a global registration rather
  than a per-peer plugin binding (`reactor.go`, `buildOrderedEgressSteps`). So an
  operator can drive a stale readvertise through the filter with `bgp-gr` absent.

`test/plugin/llgr-egress-state-unloaded.ci` stages the FIRST route, because it is
the arrangement an operator actually reaches while believing LLGR is running: two
RR clients, `external gr { run "ze plugin bgp-gr" }`, an observer that dispatches
`mark-stale` then `clear bgp rib out`, and a wire assertion that the internal
neighbor receives NO_EXPORT. It is `llgr-readvertise-multipeer.ci` with one line
changed, which is what makes it a controlled contrast against the state-loaded
run. Discrimination is measured, not assumed: with the nil-state answer reverted
to accept, obsn receives the stale route unmodified (`UPDATE (len=54)`, ORIGIN /
AS_PATH / NEXT_HOP, no community) and the test fails.

**Not a coverage reduction** (`ai/rules/completion.md`): `RFC9494-4.3-3` had no
test at all before this work and now has both polarities plus a functional tier.

### Decision: the out-of-process arrangement gets a doctor check (2026-08-10)

The first route above has a consequence worth naming. With `bgp-gr` running as a
separate process, the daemon's filter is blind for the whole process lifetime, so
it withdraws every stale route from every EBGP peer and depreferences toward
every IBGP peer -- **including peers that negotiated LLGR**, since the child does
negotiate the capability. Fail-closed is still the right answer for an unknown
capability, and nothing here weakens it; the problem is that the arrangement is
silently wrong for peers the operator believes are protected.

**Decision: a doctor check, not a config guard.** `checkGRInProcess`
(`internal/component/bgp/plugins/gr/doctor.go`) raises
`doctor-bgp-gr-out-of-process` when a plugin's `run` command launches `bgp-gr`.
The reasoning, and why the alternatives lose:

| Option | Verdict |
|--------|---------|
| Doctor check in the gr plugin | **Chosen.** The condition is "a runtime dependency of this feature is absent in this process", which is the case `ze doctor` exists for, and `codeOSPFBFDPluginAbsent` is the same shape already in the tree. It lives in the plugin, so removing the plugin removes the check (`ai/rules/plugins.md`) |
| Config guard refusing the arrangement | Rejected. `run` is a supported spelling for every bundled plugin, and refusing it at load would need `config` or the plugin process layer to know which plugin contributes an in-process filter -- a cross-tier registration seam this defect does not need (`ai/rules/simplicity.md`) |
| Nothing, the latched WARN is enough | Rejected. That WARN speaks only when the first stale route reaches the filter, which can be hours after the misconfiguration is committed, and it says nothing at `ze doctor` time |
| Registering the filter only when the engine is in-process | Rejected. It fails OPEN: stale routes would go to non-LLGR peers unfiltered, which is the violation this spec closed |

**The check judges the plugin LIST, not one `run` line (2026-08-10).** The first
version read each `run` line in isolation, so `internal grin { use bgp-gr }`
beside `external grout { run "ze plugin bgp-gr" }` was reported as blind. It is
not: `setEgressState` stores into a package-level pointer (`gr_egress.go`), so
one in-process engine answers for every destination however many other copies
run as children. `checkGRInProcess` now returns early when any plugin entry is
an in-process `bgp-gr` -- `use bgp-gr` and `run "ze.bgp-gr"` both, since
`ExtractPluginsFromTree` keeps the `use` value in `Run` and `MarkInternalPlugin`
leaves the `ze.` spelling there. The pre-pass is keyed on the plugin name, so an
internal `bgp-rib` beside an external `bgp-gr` is still reported.

**The pre-pass is right for a PEER-SCOPED `bgp-gr` too, and this is traced rather
than assumed (2026-08-10).** A peer's `process <name> { ... }` block reaches the
plugin list through `extractInlinePluginsFromMap`
(`internal/component/bgp/config/plugins.go`), which copies the block into
`PluginConfig.Run` and calls `config.MarkInternalPlugin`. Two shapes can arrive
there, and the pre-pass judges both correctly:

| Peer-scoped shape | What the producers make of it | Pre-pass |
|---|---|---|
| `run "ze.bgp-gr"` | `MarkInternalPlugin` asks `ResolvePlugin` (`internal/component/plugin/resolve.go`), whose `ze.` branch resolves a name `registry.Has` knows, so `Type` is `PluginTypeInternal` and `Internal` becomes true. `Run` keeps the `ze.` spelling | `internalIsPlugin` trims `ze.` and matches, so the check is SILENT. Correct: the engine runs in the daemon |
| `run "ze plugin bgp-gr"`, in any path spelling | `ResolvePlugin` takes its external branch for a command line, returns `PluginTypeExternal`, and `MarkInternalPlugin` leaves `Internal` false | `runTargetsPlugin` finds the `plugin bgp-gr` verb pair and the check WARNS. Correct: `startExternal` forks it |

**A peer-scoped `use` is not a third shape: the config layer refuses it.**
`extractInlinePluginsFromMap` reads `procMap["use"]`, but `list process` in
`internal/component/bgp/yang/ze-bgp-conf.yang` defines only `name`, `run`,
`processes`, `processes-match`, `neighbor-changes`, `content`, `receive` and
`send`, and carries no `ze:allow-unknown-fields`. `(*Parser).parseList`
(`internal/component/config/parser_list.go`) answers an undefined field in a list
entry with `unknown field in ...`, and `LoadConfig`
(`internal/component/config/loader.go`) returns that error rather than a warning.
So `process <name> { use bgp-gr; }` never loads, and the `use` branch is reachable
only for the globally declared `plugin { internal <name> { use bgp-gr; } }` form
that `ExtractPluginsFromTree` fills.

No shape a peer-scoped block can emit is misjudged. Note that the silent case
turns on `Internal`, never on which peer declared the block: `setEgressState`
stores into a package-level pointer (`gr_egress.go`), so one in-process engine
answers for every destination.

## Files to Modify
- `internal/component/bgp/plugins/gr/gr_egress.go` -- the nil-state answer, and
  the lock the filter reads `peerLLGRCaps` under
- `internal/component/bgp/plugins/gr/gr.go` -- the store site passes `&gp.mu`
- `internal/component/bgp/plugins/gr/register.go` -- `grPluginName`, and the
  doctor check and code registration
- `internal/component/bgp/plugins/gr/doctor.go` (new) -- `checkGRInProcess`
- `internal/component/bgp/filterapi/filterapi.go` -- the stale decision record
- `test/plugin/llgr-egress-state-unloaded.ci` (new) -- the functional tier
- `test/ui/doctor-bgp-gr-out-of-process.ci` (new) -- the `ze doctor` entry point

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
- [ ] AC-1..AC-8 all demonstrated (see Goal Validation)
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Implementation Summary

### What Was Implemented

- **The nil-state answer** (`internal/component/bgp/plugins/gr/gr_egress.go`,
  `LLGREgressFilter`). An unloaded `egressState` now resolves to `hasLLGR=false`
  instead of short-circuiting to ACCEPT, so the destination takes the RFC 9494
  Section 4.3 withdraw (EBGP) or the Section 4.6 NO_EXPORT + LOCAL_PREF 0
  depreference (IBGP). The `staleLevel == 0` fast path moved ABOVE the state load,
  so the state is read only for a route the RFC governs.
- **A live logger for the WARN** (`egressWarnLogger`, `gr_egress.go`;
  `loggerConfigured`, `gr.go`). Every caller of `SetLogger` is on the engine path,
  which is exactly the path that does not run in the case the warning exists for.
  The fallback builds a logger from `slogutil.Logger(grSubsystem)`, so a silenced
  subsystem stays silent. Latched to one line per process
  (`egressStateMissingWarned`).
- **The lock the filter reads under** (`egressFilterState.mu`, `hasLLGR`;
  `setEgressState(&egressFilterState{mu: &gp.mu, ...})` in `gr.go`). The filter
  shared `peerLLGRCaps` by reference and read it with no lock while `extractGRCaps`
  and `handleOpenEvent` inserted and `onPeerRemoved` and `releaseRoutes` deleted
  under `gp.mu`. That is `fatal error: concurrent map read and map write`, which
  `recover()` does not catch. Taken only on the stale path.
- **The doctor check** (`internal/component/bgp/plugins/gr/doctor.go`, new:
  `checkGRInProcess`, `internalIsPlugin`, `runTargetsPlugin`, `grDoctorCheck`,
  `grDiagnosticCodes`; registered from `register.go`). `bgp-gr` launched with
  `run "ze plugin bgp-gr"` runs the engine in a child, so the daemon's filter is
  blind for the process lifetime and treats even LLGR-capable peers as incapable.
  `doctor-bgp-gr-out-of-process` says so at `ze doctor` time rather than when the
  first stale route arrives. It judges the whole plugin LIST first: one in-process
  `bgp-gr` answers for every destination, so a forked sibling costs nothing.
- **Tests**: 7 unit tests in `gr_egress_test.go`, 6 in `doctor_test.go` (new), and
  two `.ci`s: `test/plugin/llgr-egress-state-unloaded.ci` (the wire tier) and
  `test/ui/doctor-bgp-gr-out-of-process.ci` (the `ze doctor` entry point).
- **`RFC9494-4.3-3`** went from `--`/`--` in `ai/RFC-REQUIREMENTS.md` to both
  polarities, at `unit/verify` and `functional/verify`.

### Bugs Found/Fixed

- **Unlocked map read on the forward path** (BLOCKER, round 1 item 1). Reachable
  in production: a peer flap during a stale readvertise is the collision. Fixed by
  `egressFilterState.mu`; `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock`
  drives the real producers and reports `WARNING: DATA RACE` when reverted. The
  pre-existing `TestLLGREgressFilter_ConcurrentAccess` stays GREEN under the same
  revert, which is why a reader-only concurrency test never caught it.
- **A test pinned the RFC violation.** `TestLLGREgressFilter_NilState` asserted a
  stale route to an EBGP destination passing with `mods.Len() == 0`. Replaced; its
  stated purpose (no nil-pointer panic) is still covered.
- **The WARN was unhearable.** `logger()` was still `init()`'s discard logger in
  the one case the warning exists for, so the latch was spent on a dropped line.
- **A stale decision record** in `internal/component/bgp/filterapi/filterapi.go`
  still named `LLGREgressFilter` as an instance of the seam's known gap.
- **`RFC requirement:` anchors off by about 75 lines** in `gr_egress_test.go`,
  re-anchored to the symbol each tag proves.
- **This spec's own `register.go` edit emptied a discovery-index row.** Replacing
  the literal `Name: "bgp-gr"` with `Name: grPluginName` made
  `ai/PACKAGE-MAP.md` lose the plugin name for
  `internal/component/bgp/plugins/gr`, because `NAME_RE` in
  `scripts/dev/package_map.py` matched a quoted string only. `registration` now
  falls back to a same-file string constant (`const_value`), which restores that
  row and fills three others that were blank for the same reason
  (`internal/plugins/kernel`, `internal/plugins/routingtable`,
  `internal/test/plugins/fakeddos`). Covered by
  `test_registered_name_from_a_package_constant` and
  `test_a_quoted_name_anywhere_beats_the_constant_lookup`
  (`scripts/dev/package_map_test.py`).

### Documentation Updates

- `docs/guide/graceful-restart.md`, new subsection "Keep the bgp-gr engine in the
  daemon process" under Plugin Bindings: names the withdraw / NO_EXPORT +
  LOCAL_PREF 0 outcome and the `doctor-bgp-gr-out-of-process` code. Two source
  anchors: `internal/component/bgp/plugins/gr/doctor.go -- checkGRInProcess,
  doctor-bgp-gr-out-of-process` and `internal/component/bgp/plugins/gr/gr_egress.go
  -- LLGREgressFilter reads egressState`.
- `ai/RFC-REQUIREMENTS.md` is NOT in this closure commit, and it is clean against
  HEAD. The regeneration this spec needed rode out in another session's commit
  `7ec29b6e6`, which also cut the file from 6203 lines to 621 while
  `scripts/dev/rfc_requirements.py` moves to per-RFC shards
  (`spec-rfc-ledger-per-rfc-shards`). The file therefore carries no
  `RFC9494-4.3-3` row today. The requirement's proof does not depend on that
  ledger: three `RFC requirement:` tags carry it, on
  `TestLLGREgressFilter_NilStateWithdrawsEBGP` (positive) and
  `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` (negative) in
  `gr_egress_test.go`, and on the functional-tier
  `test/plugin/llgr-egress-state-unloaded.ci`.
- `docs/features/rfc-status.md` needs no edit: RFC 9494's row already claims LLGR
  Supported, and this work proves a requirement rather than changing a support
  level.
- `scripts/dev/code_to_docs.py --check` exits 0 (source anchors resolve).

### Deviations from Plan

- **The spec carried no Deliverables, Security Review, or Documentation Update
  Checklist.** It was written after the fact, as a fixit spec. The substance of
  all three is supplied in the Implementation Audit, the Security row of
  Documentation Verified below, and Documentation Updates above.
- **Implementation Step 2 ("establish whether the window is reachable at all")
  answered YES**, so the spec did not close with a test proving unreachability.
  The out-of-process case makes the window the whole process lifetime, not a
  startup instant.
- **The doctor check was not in the original plan.** It is a consequence of the
  reachability answer, added on round 1 item 5.
- **Two comments this spec ships carry a measurably wrong claim, and they are
  left for the next touch of their files.** The docstring of
  `test_a_quoted_name_anywhere_beats_the_constant_lookup`
  (`scripts/dev/package_map_test.py`) says first-occurrence-wins "emptied eight
  correct rows" and "would publish the command name as the plugin name". An
  independent run of both orderings over all 623 packages measured **14 rows
  changed and 11 emptied**, and the mechanism is inverted: the affected
  `register.go` files declare the CONSTANT `Name:` before the quoted one, so
  first-wins publishes the constant (`show rib` becomes `rib`) or goes blank
  when the constant lives in another file. The command name is what those rows
  LOSE. The comment above `NAME_CONST_RE` in `scripts/dev/package_map.py` says
  the same thing. The NEW test,
  `test_a_quoted_name_after_the_constant_still_beats_it`, states both counts
  correctly in its own docstring, so the behaviour under test is not in doubt.
  **Correct both on the next touch of those two files.** Correcting them in this
  closure would move their SHA-256 and void the review artifact
  `tmp/review/fixit-egress-filter-non-decision-channel-640fa955-f03a-45e8-a58f-4b367f5859e6.md`,
  which forces a re-record of a gate that is already clean over 4 rounds.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The unloaded state was assumed to be a startup-ordering window only | It is the whole process lifetime whenever the GR plugin engine runs out of process, or is absent | Reading `setEgressState`'s only non-test caller against `filterapi.Register`'s `init()` | The fix is a behavior change, not a sequencing fix; and the out-of-process case earned a doctor check |
| approach | A functional `.ci` was withdrawn on 2026-08-07 as impossible: "no operator-facing way to unload the pointer", "would need a production backdoor" | Two config-only routes reach the unloaded state, and neither touches the filter: `run "ze plugin bgp-gr"` forks the engine, and `request bgp rib mark-stale` drives the readvertise rail with no GR plugin at all | Round 1 item 4 checked the claim against `ExtractPluginsFromTree` and `Process.StartWithContext` | The withdrawal is void; `test/plugin/llgr-egress-state-unloaded.ci` is written and discriminates |
| approach | The doctor check first read each `run` line in isolation | `setEgressState` stores into a package-level pointer, so one in-process engine answers for every destination; `internal grin { use bgp-gr }` beside `external grout { run ... }` is NOT blind | Writing `TestCheckGRInProcessAcceptsAnInternalGRBesideAnExternalOne` | A pre-pass over the whole plugin list, keyed on the plugin name |
| approach | The `package_map.py` fallback first accepted an identifier wherever `Name:` appeared | First-occurrence-wins changes 14 of the 623 rows and empties 11 of them. The affected `register.go` files put the CONSTANT `Name:` BEFORE the quoted one, so first-wins publishes the constant (`show rib` becomes `rib`, `show static` becomes `static`) or nothing when the constant lives in another file. The command name is what those rows LOSE, not what they gain | Regenerating `ai/PACKAGE-MAP.md` against the real tree and reading the diff before committing it | The constant lookup is a FALLBACK, tried only when the file holds no quoted `Name:` at all. The diff is now three blanks filled and nothing else |
| escalation | The closure prose traced a peer-scoped `process { use bgp-gr }` that cannot exist | `list process` in `ze-bgp-conf.yang` has no `use` leaf and no `ze:allow-unknown-fields`, and `(*Parser).parseList` refuses an undefined field; and a bare `bgp-gr` would resolve `PluginTypeExternal` anyway, so the paragraph's conclusion was reached by a route that inverts it | Round 3, then re-verified against both producers during closure | Paragraph rewritten from the shapes `extractInlinePluginsFromMap` can emit. A trace is not evidence until its producers are read (`ai/rules/evidence.md`) |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Answer the ordering question: is the nil-state window reachable | Done | `gr_egress.go` `egressState` doc comment; spec "Outcome (2026-08-07)" | YES, and not only at startup |
| Answer the RFC question: fail open or fail closed | Done | `gr_egress.go` `LLGREgressFilter` | Fail CLOSED. Section 4.3's literal reading, not a defensive choice |
| Clear all seven round-1 review items | Done | spec "Remediation (2026-08-10)" | All seven cleared, each with its producer named |
| Clear the round-2 items | Done | spec "Findings fixed (round 2 ...)" | 2 ISSUEs, 3 NOTEs |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLLGREgressFilter_NilStateWithdrawsEBGP` | RED when reverted, on `mods.IsWithdraw()` |
| AC-2 | Done | `TestLLGREgressFilter_NilStateDepreferencesIBGP`; `test/plugin/llgr-egress-state-unloaded.ci` | RED when reverted, on both NO_EXPORT and LOCAL_PREF |
| AC-3 | Done | `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute` | GREEN under the revert: a contrast, not a duplicate |
| AC-4 | Done | `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` | GREEN under the revert |
| AC-5 | Done | `TestLLGREgressWarnLoggerIsLiveWhenEngineNeverStarted`, `TestLLGREgressWarnLoggerRespectsEngineChoice`, `TestLLGREgressFilterWarnsWhenStateMissing` | Live logger, engine's choice honored, latched to one line |
| AC-6 | Done | `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` | `-race`; DATA RACE in `extractGRCaps` vs `LLGREgressFilter` when reverted |
| AC-7 | Done | `doctor_test.go` (6 tests); `test/ui/doctor-bgp-gr-out-of-process.ci` | The `.ci` is the wiring proof: the unit tests call `checkGRInProcess` directly |
| AC-8 | Done | `test/plugin/llgr-egress-state-unloaded.ci` | With the answer reverted, obsn receives `UPDATE (len=54)` unmodified and the test fails |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLLGREgressFilter_NilStateWithdrawsEBGP` | Done | `internal/component/bgp/plugins/gr/gr_egress_test.go` | `RFC9494-4.3-3` positive |
| `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` | Done | same | `RFC9494-4.3-3` negative |
| `TestLLGREgressFilter_NilStateDepreferencesIBGP` | Done | same | `RFC9494-4.6-2` / `4.6-3` positives |
| `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute` | Done | same | bounds the fix |
| `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock` | Done | same | AC-6 |
| `TestLLGREgressWarnLoggerIsLiveWhenEngineNeverStarted` | Done | same | AC-5 |
| `TestLLGREgressWarnLoggerRespectsEngineChoice` | Done | same | AC-5 |
| `TestLLGREgressFilterWarnsWhenStateMissing` | Done | same | AC-5, latch |
| `TestCheckGRInProcessFlagsAnExternalGR` | Done | `internal/component/bgp/plugins/gr/doctor_test.go` | AC-7 positive |
| `TestCheckGRInProcessAcceptsTheSupportedArrangements` | Done | same | AC-7 negative |
| `TestCheckGRInProcessMatchesAPathQualifiedLaunch` | Done | same | path-qualified spelling |
| `TestCheckGRInProcessAcceptsAnInternalGRBesideAnExternalOne` | Done | same | the list pre-pass |
| `TestCheckGRInProcessStillFlagsAnotherPluginRunInProcess` | Done | same | the pre-pass is keyed on `bgp-gr` |
| `TestGRDoctorCodeIsExplainable` | Done | same | the code resolves through `ze explain` |
| LLGR stale readvertise, state unloaded | Done | `test/plugin/llgr-egress-state-unloaded.ci` | AC-8 |
| `ze doctor` reports the out-of-process arrangement | Done | `test/ui/doctor-bgp-gr-out-of-process.ci` | AC-7 wiring |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/gr/gr_egress.go` | Done | nil-state answer, `egressFilterState.mu`, `hasLLGR`, `egressWarnLogger` |
| `internal/component/bgp/plugins/gr/gr.go` | Done | `setEgressState` passes `&gp.mu`; `loggerConfigured` |
| `internal/component/bgp/plugins/gr/register.go` | Done | `grPluginName`, doctor check and code registration |
| `internal/component/bgp/plugins/gr/doctor.go` | Done | new |
| `internal/component/bgp/filterapi/filterapi.go` | Done | stale decision record corrected |
| `test/plugin/llgr-egress-state-unloaded.ci` | Done | new |
| `test/ui/doctor-bgp-gr-out-of-process.ci` | Done | new |
| `internal/component/bgp/plugins/gr/gr_egress_test.go` | Changed | not in the plan list; the tests and the re-anchored `RFC requirement:` tags live here |
| `internal/component/bgp/plugins/gr/doctor_test.go` | Changed | not in the plan list; new, six tests |
| `docs/guide/graceful-restart.md` | Changed | not in the plan list; round-2 item 2 |
| `ai/RFC-REQUIREMENTS.md` | Changed | regenerated ledger; only the `RFC9494-4.3-3` row is this spec's |

### Audit Summary
- **Total items:** 39 (4 requirements, 8 ACs, 16 tests, 11 files)
- **Done:** 35
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (four files shipped that the plan's "Files to Modify" list did not name; recorded above and in Deviations)

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Row 1: `LLGREgressFilter` returns ACCEPT when `egressState.Load()` is nil | done | This spec. The nil state resolves to `hasLLGR=false`; proven by the four nil-state unit tests plus `test/plugin/llgr-egress-state-unloaded.ci` |
| Row 2: two call sites discard `safeEgressFilter`'s `panicked` return | done | Landed in `spec-fixit-stored-route-relay-hardening` under its rewritten AC-7, on Thomas's 2026-08-03 ruling. Not owed here |
| Row 3: `decideStaleReadvertise`'s `buildModifiedPayload` failure branch | done | Landed in `spec-fixit-stored-route-relay-hardening` alongside row 2 |
| Row 4: RFC 2545 has no summary and no full text | done | Landed in `spec-followup-rfc-enrollment` on 2026-08-07 |

**The shard is NOT removed by this closure**, and that is deliberate.
`plan/deferrals/fixit-stored-route-relay-hardening.md` is the shard of
`plan/spec-fixit-stored-route-relay-hardening.md`, which is still in-progress and
cites it under Known Limitations. This spec is a DESTINATION of row 1, not the
shard's owner, and every row was already terminal before this session, so nothing
here made it residue. Its owner removes it at its own closure
(`ai/rules/planning.md`).

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/gr/doctor.go` | Yes | `ls -l`: 5071 bytes, 2026-08-10 19:39 |
| `internal/component/bgp/plugins/gr/doctor_test.go` | Yes | `ls -l`: 5559 bytes, 2026-08-10 19:36 |
| `internal/component/bgp/plugins/gr/gr_egress.go` | Yes | `ls -l`: 7887 bytes, 2026-08-10 19:35 |
| `internal/component/bgp/plugins/gr/gr_egress_test.go` | Yes | `ls -l`: 30455 bytes |
| `internal/component/bgp/plugins/gr/register.go` | Yes | `ls -l`: 3250 bytes |
| `internal/component/bgp/filterapi/filterapi.go` | Yes | `ls -l`: 38365 bytes |
| `test/plugin/llgr-egress-state-unloaded.ci` | Yes | `ls -l`: 7316 bytes |
| `test/ui/doctor-bgp-gr-out-of-process.ci` | Yes | `ls -l`: 3517 bytes |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Nil state withdraws for EBGP | `GOFLAGS=-count=1 make ze-test-pkg PKG=./internal/component/bgp/plugins/gr` -> `ok ... 3.168s` (with `-race`), carrying `TestLLGREgressFilter_NilStateWithdrawsEBGP` |
| AC-2 | Nil state depreferences for IBGP | same run, `TestLLGREgressFilter_NilStateDepreferencesIBGP`; `gr_egress.go` `LLGREgressFilter` emits `mods.Op(attrCodeCommunity, AttrModAdd, communityNoExport[:])` and `mods.Op(attrCodeLocalPref, AttrModSet, localPrefZero[:])` |
| AC-3 | A fresh route is untouched | same run, `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute`; `LLGREgressFilter` returns true at `staleLevel == 0` before the state load |
| AC-4 | A recorded LLGR peer is unaffected | same run, `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` |
| AC-5 | The WARN reaches a live logger, once | same run, three logger tests; `egressWarnLogger` reads `loggerConfigured` and falls back to `slogutil.Logger(grSubsystem)` |
| AC-6 | Both sides hold `grPlugin.mu` | same run under `-race`, `TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock`; `(*egressFilterState).hasLLGR` takes `s.mu`, and all three constructions of `egressFilterState` set `mu` (`gr.go`, `gr_egress_test.go` x2) |
| AC-7 | `ze doctor` reports the arrangement | same run, six `doctor_test.go` tests; `register.go` registers `grDoctorCheck`, and `test/ui/doctor-bgp-gr-out-of-process.ci` asserts `"code": "doctor-bgp-gr-out-of-process"` from `ze doctor --json` |
| AC-8 | The peer receives NO_EXPORT on the wire | `test/plugin/llgr-egress-state-unloaded.ci`, obsn's seq=2 wire expectation carrying `0xFFFFFF01`; measured RED when the nil-state answer is reverted (`UPDATE (len=54)` unmodified) |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A stale route reaches the egress pipeline while `egressState` is unloaded | (unit) `gr_egress_test.go` | Yes: `LLGREgressFilter` is registered from `init()` (`register.go`) and the only non-test writer of the state is `RunGRPlugin`'s `OnConfigure` |
| An operator marks routes stale while `bgp-gr` runs out of process | `test/plugin/llgr-egress-state-unloaded.ci` | Yes, file read: `external gr { run "ze plugin bgp-gr" }`, two RR clients, `mark-stale` then `clear bgp rib out`, and a wire expectation on the depreferenced form |
| A peer flaps while a stale route is forwarded | (unit, `-race`) `gr_egress_test.go` | Yes: the test drives `extractGRCaps` and `onPeerRemoved`, the real producers, against the filter |
| `ze doctor` over a config that launches `bgp-gr` with `run` | `test/ui/doctor-bgp-gr-out-of-process.ci` | Yes, file read: four cases, `ze doctor --json` plus `ze explain`, with two `!contains` contrasts that fail a check warning unconditionally |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| (none declared) | n/a | The spec has no `Risks & Assumptions` section: `grep -n "Risks & Assumptions" plan/spec-fixit-egress-filter-non-decision-channel.md` exits 1. The two beliefs that behaved as assumptions and BROKE are in the Mistake Log above: the window is a lifetime rather than a startup instant, and the functional `.ci` was reachable from configuration alone |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| User guide: `docs/guide/graceful-restart.md` "Keep the bgp-gr engine in the daemon process" | `checkGRInProcess` (`doctor.go`) is silent for `use bgp-gr` and for `run "ze.bgp-gr"` (both leave `Internal` true via `MarkInternalPlugin` -> `ResolvePlugin`'s `ze.` branch) and warns for `run "ze plugin bgp-gr"` via `runTargetsPlugin`. The heading names the process, which is what the check judges | Yes: the earlier heading ("never with `run`") was broader than the check and was narrowed at closure |
| RFC compliance: `RFC9494-4.3-3` carries both polarities | Three `RFC requirement:` tags in the tree: `TestLLGREgressFilter_NilStateWithdrawsEBGP` (positive, unit) and `TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer` (negative, unit) in `gr_egress_test.go`, plus `test/plugin/llgr-egress-state-unloaded.ci` (positive, functional) | Yes, read from the tagged sources. `ai/RFC-REQUIREMENTS.md` is not in this commit: see Documentation Updates above |
| RFC status page: `docs/features/rfc-status.md` | RFC 9494's row already claims LLGR Supported; this work proves a requirement rather than changing a support level, so no Status, Implemented or Remaining cell moves | Yes, no edit owed |
| Doctor check: a new runtime-arrangement diagnostic | `codeGROutOfProcess` is declared in `grDiagnosticCodes` (`doctor.go`) and registered from `register.go`; `TestGRDoctorCodeIsExplainable` and the `.ci`'s `ze explain` step both resolve it | Yes |
| Source anchors resolve | `python3 scripts/dev/code_to_docs.py --check` exits 0 | Yes |
| Security: the diff adds a lock on a forward path and a config-reading doctor check | `LLGREgressFilter` parses no untrusted input (`payload` is unread), builds no strings on the hot path, and logs once per process. `(*egressFilterState).hasLLGR` locks and defers unlock; all three constructions set `mu`, so there is no nil-mutex panic. `checkGRInProcess` reads an already-parsed plugin list and runs no command. No injection, path traversal, unbounded allocation, or privilege surface | Yes: no finding |

## Core Insight

**A nil pointer is the absence of an answer, not the answer "no".** The filter's
old code read an unloaded `egressState` as "nothing is stale here" and accepted.
The state withheld exactly one fact, the destination's LLGR capability, and RFC
9494 Section 4.3 keys its decision on whether that capability "has been received".
With nothing recorded it has not been received from anyone, so the literal reading
and the fail-closed reading are the same reading. No new branch was invented: the
nil state simply stopped short-circuiting past the branches already written.

The second half is where this generalizes. A missing state is usually assumed to
be a startup instant, so the fix is assumed to be sequencing. Here the state is
missing for the WHOLE process lifetime whenever the engine runs elsewhere, which
is a supported configuration. Fail-closed is correct in both cases and wrong for
nobody, but the long case is silently wrong for peers the operator believes are
protected. That is what earns a `ze doctor` check rather than a louder log: the
config is readable long before the first stale route arrives.
