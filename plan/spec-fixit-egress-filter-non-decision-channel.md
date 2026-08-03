# Spec: fixit-egress-filter-non-decision-channel

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

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
| GR restart before the plugin stores its egress state | -> | `LLGREgressFilter` nil-state answer | `.ci` in `test/plugin/` asserting the stale route's treatment toward a non-LLGR peer |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| LLGR nil-state answer | `internal/component/bgp/plugins/gr/gr_egress_test.go` | the chosen fail-open or fail-closed behavior for an unloaded plugin state | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| LLGR stale readvertise before plugin state load | `test/plugin/*.ci` | a peer restarts and stale routes are readvertised while the GR plugin is still starting | |

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
