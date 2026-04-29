# Spec: bgp-functional-test-evidence

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | bgp-4 |
| Updated | 2026-04-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/functional-tests.md` - runner capabilities and assertion primitives
4. `internal/component/bgp/config/loader_test.go` - skipped example-config coverage
5. `internal/component/plugin/registry/registry_bgp_filter.go` - egress filters run per destination during forward
6. `internal/component/plugin/types_bgp.go` - `ForwardUpdatesDirect()` uses the same forward pipeline
7. `internal/component/bgp/reactor/reactor_notify.go` - safe egress filter path
8. `test/plugin/community-strip.ci` - current framework-wired / TODO sentinel
9. `test/plugin/role-otc-egress-filter.ci` - current AC-claim mismatch
10. `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` - deferred deep-byte assertion
11. `test/plugin/llgr-readvertise.ci` - explicit partial coverage
12. `test/plugin/nexthop-self-ipv6-forward.ci` - explicit single-ze-peer blocker

## Task

Make the BGP/plugin test evidence honest and executable. Remove blanket skips where current parser coverage should exist, and ensure targeted functional tests either prove the ACs they claim or are explicitly downgraded to partial/blocked status with the exact limitation named. The goal is trustworthy review evidence, not optimistic comments.

## Required Reading

### Architecture / Test-System Docs
- [ ] `docs/functional-tests.md` - runner capabilities, `ze-peer` expectations, observer/runtime-fail patterns
  -> Decision: end-to-end proof should use runner-supported primitives (`expect=bgp`, `expect=stderr/stdout`, `runtime_fail`) rather than count-only observer heuristics when the claim is about wire behavior.
  -> Constraint: if the runner cannot prove a claim, the test must be marked partial/blocked, not full AC coverage.
- [ ] `.claude/memory/feedback_periodic_test_sweep.md` - recurring test-gap pattern
  -> Decision: missing runner support is a real gap to either close or name explicitly.
  -> Constraint: do not leave silent coverage holes just because integration tests exist nearby.

### Source / Behavior Docs
- [ ] `internal/component/plugin/registry/registry_bgp_filter.go` - egress filter contract
  -> Constraint: egress filters are called only during per-destination forward paths; tests that never trigger forwarding cannot claim egress-filter ACs.
- [ ] `internal/component/plugin/types_bgp.go` - fast-path forward contract
  -> Decision: `ForwardUpdatesDirect()` reuses the same forward pipeline as `ForwardUpdate()`.
  -> Constraint: fast-path tests can prove forwarding-path wiring without the legacy text RPC, but only the assertions they actually make.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - safe ingress/egress filter calls
  -> Constraint: observer-only tests that never hit the reactor forward path do not exercise `safeEgressFilter()`.

**Key insights:**
- `VALIDATES:` comments are documentation only; if they overstate coverage, reviewers get false confidence.
- `community-strip.ci` and `role-otc-egress-filter.ci` currently admit in-file that they do not exercise the AC they claim.
- `TestParseAllConfigFiles` is a blanket skip even though `etc/ze/bgp/` still ships many example configs; that hides drift between shipped examples and the real parser.
- Partial coverage is fine in this repo if it is named precisely. "Framework wired" or "TODO" cannot stand in for passing evidence.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/loader_test.go` - `TestParseAllConfigFiles` is fully skipped with `TODO: Convert etc/ze/bgp/*.conf files from ExaBGP to native Ze syntax`.
- [ ] `etc/ze/bgp/conf-ebgp.conf` - shipped example still uses legacy ExaBGP-style peer syntax (`peer 127.0.0.1 { local-as ... peer-as ... }`), not current native Ze config layout.
- [ ] `etc/ze/bgp/conf-template.conf` - another shipped example in legacy syntax, confirming the skip is covering a real compatibility gap rather than a missing directory.
- [ ] `etc/ze/bgp/api-simple.conf` - plugin example fixtures also live in the same legacy-style tree.
- [ ] `docs/functional-tests.md` - documents `ze-peer`, observer plugins, and runner assertions; there is no magic AC auditing beyond the assertions written in the `.ci` file.
- [ ] `internal/component/plugin/registry/registry_bgp_filter.go` - egress filters run per destination peer during forward, with `ModAccumulator` modifications applied after filter approval.
- [ ] `internal/component/plugin/types_bgp.go` - `ForwardUpdatesDirect()` explicitly reuses the same per-destination forward pipeline as `ForwardUpdate()`.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `safeEgressFilter()` is only relevant when the reactor actually forwards to a destination peer.
- [ ] `test/plugin/community-strip.ci` - header claims `VALIDATES: AC-7`, but the file itself says AC verification is TODO, "framework wired", and architecturally blocked because no forwarding plugin is loaded.
- [ ] `test/plugin/role-otc-egress-filter.ci` - header claims OTC behavior, but the footer says AC verification is TODO due to the same architectural blocker as `community-strip.ci`.
- [ ] `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` - explicitly proves fast-path wiring and rejects legacy RPC fallback, but says deep A==B byte equality is deferred to follow-up.
- [ ] `test/plugin/llgr-readvertise.ci` - explicitly marks itself `AC-9 (partial)` and says multi-peer suppression coverage is blocked by missing test infrastructure.
- [ ] `test/plugin/nexthop-self-ipv6-forward.ci` - explicitly says wire-level forwarding verification is blocked by a single-`ze-peer` multi-IP timing issue and only checks config/peer-detail state.

**Behavior to preserve:**
- A test may stay partial or blocked if the runner truly cannot prove the full AC yet.
- Existing runner primitives (`ze-peer`, observer plugins, `runtime_fail`, `expect=bgp`) stay the preferred evidence path.
- `bgp-rs-fastpath-ebgp-shared.ci` keeps its current fast-path wiring proof even if deeper byte-equality remains deferred.
- Shipped legacy example configs in `etc/ze/bgp/` may remain as legacy artifacts if the repo chooses to keep them, but parser coverage must then say exactly what is and is not covered.

**Behavior to change:**
- Blanket skipped coverage (`TestParseAllConfigFiles`) must become honest coverage or an explicit curated exclusion.
- Targeted `.ci` tests must not claim AC coverage for paths they do not execute.
- Related partial/blocked tests must state exact scope and blocker in a consistent way.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point -- Example Config Coverage
1. `loader_test.go` is the unit-level parser coverage entry point for shipped BGP example configs.
2. It should iterate over the example fixture set and call `LoadReactor(...)` or a curated equivalent.
3. Today the path is cut off at `t.Skip(...)`, so no parser evidence exists for the shipped tree.

### Entry Point -- Functional `.ci` Test
1. `.ci` file is read by the functional test runner.
2. Runner launches `ze-peer`, helper/observer scripts, and `ze`.
3. Assertions come only from explicit runner directives (`expect=...`, `reject=...`) plus helper scripts calling `runtime_fail(...)`.
4. Comments such as `VALIDATES:` and `PREVENTS:` are human-facing documentation and must match the assertions actually present.

### Entry Point -- Egress Filter / Forwarding Proof
1. Route enters via peer or cache consumer.
2. Reactor forwards to a destination peer through `ForwardUpdate()` or `ForwardUpdatesDirect()`.
3. Egress filters run per destination via `safeEgressFilter(...)`.
4. If no forwarding-capable plugin or destination-forward path is exercised, the test does not prove egress-filter behavior.

### Transformation Path
1. Reviewer reads `VALIDATES:` / `STATUS:` comments and infers expected coverage.
2. Runner executes actual `.ci` directives and helper scripts.
3. Only explicit assertions become evidence.
4. This spec closes the gap between step 1 (claim) and step 3 (proof).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Example config files -> parser coverage | `loader_test.go` fixture sweep | [ ] |
| `.ci` file -> runner | `ze-test` loads directives and spawns processes | [ ] |
| Observer helper -> daemon | `ze-plugin-engine:dispatch-command` / `runtime_fail(...)` | [ ] |
| Reactor forward path -> egress filter | `ForwardUpdate()` / `ForwardUpdatesDirect()` -> `safeEgressFilter()` | [ ] |
| Wire behavior -> test proof | `expect=bgp:...` or deterministic stderr/stdout assertion | [ ] |

### Integration Points
- `internal/component/bgp/config/loader_test.go` - parser coverage signal for shipped examples.
- `etc/ze/bgp/*.conf` - legacy example corpus that currently sits outside real parser coverage.
- `test/plugin/*.ci` - human-facing AC claims plus actual runner assertions.
- `docs/functional-tests.md` - authoritative statement of what the runner can and cannot prove today.

### Architectural Verification
- [ ] No false full-coverage claims remain in targeted tests.
- [ ] Any remaining blockers are exact, named, and tied to a real runner/infrastructure limitation.
- [ ] Egress-filter ACs are claimed only when a forward path is actually exercised.
- [ ] Parser coverage either executes against current fixtures or explicitly curates legacy exclusions.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Shipped BGP example fixtures | -> | `TestParseAllConfigFiles` or replacement coverage in `loader_test.go` | `TestParseAllConfigFiles` |
| Community strip functional proof | -> | forward path -> egress filter -> dest assertion | `community-strip.ci` |
| OTC egress filter functional proof | -> | forward path -> OTC filter -> dest assertion | `role-otc-egress-filter.ci` |
| Fast-path forwarding proof | -> | `ForwardUpdatesDirect()` -> forward pipeline | `bgp-rs-fastpath-ebgp-shared.ci` |
| LLGR partial coverage | -> | reconnect -> readvertise -> dest assertion | `llgr-readvertise.ci` |
| IPv6 next-hop / RR partial coverage | -> | config parse -> peer detail / forward path as applicable | `nexthop-self-ipv6-forward.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Example config coverage in `loader_test.go` | No blanket skip remains; the test either parses a curated native subset or explicitly documents why specific legacy fixtures are excluded |
| AC-2 | `community-strip.ci` after this work | It either proves egress strip behavior with real assertions, or its `VALIDATES` / status text is downgraded so it no longer claims AC-7 coverage |
| AC-3 | `role-otc-egress-filter.ci` after this work | It either proves the claimed OTC egress behavior, or its coverage claim is downgraded with an explicit blocker |
| AC-4 | `bgp-rs-fastpath-ebgp-shared.ci` after this work | The file clearly distinguishes wiring proof from any still-deferred byte-equality proof |
| AC-5 | `llgr-readvertise.ci` and `nexthop-self-ipv6-forward.ci` after this work | Partial coverage claims remain precise and do not imply more than the assertions prove |
| AC-6 | Remaining blockers in targeted tests | Each blocker names the exact missing runner/infrastructure capability, not just "TODO" or "framework wired" |
| AC-7 | Reviewers reading targeted files | The `VALIDATES` / `STATUS` comments and the actual assertions now agree |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAllConfigFiles` | `internal/component/bgp/config/loader_test.go` | Shipped examples are either truly covered or intentionally curated out | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | No new numeric input is introduced by this spec | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `community-strip` | `test/plugin/community-strip.ci` | Reviewer can trust whether egress community stripping is actually proven or still blocked | |
| `role-otc-egress-filter` | `test/plugin/role-otc-egress-filter.ci` | Reviewer can trust whether OTC egress behavior is actually proven or still blocked | |
| `bgp-rs-fastpath-ebgp-shared` | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Fast-path coverage claim matches the proof in the file | |
| `llgr-readvertise` | `test/plugin/llgr-readvertise.ci` | Partial LLGR coverage is scoped honestly | |
| `nexthop-self-ipv6-forward` | `test/plugin/nexthop-self-ipv6-forward.ci` | IPv6 forwarding/config coverage is scoped honestly | |

### Future (if deferring any tests)
- If multi-peer / multi-IP infrastructure is still missing after audit, record the exact missing primitive in `docs/functional-tests.md` and keep the affected tests explicitly partial/blocked.

## Files to Modify

- `internal/component/bgp/config/loader_test.go` - replace the blanket skip with real coverage or curated exclusions.
- `etc/ze/bgp/*` - convert selected fixtures or curate/document which legacy fixtures are intentionally out of native parser coverage.
- `test/plugin/community-strip.ci` - make AC claim and assertions agree.
- `test/plugin/role-otc-egress-filter.ci` - make AC claim and assertions agree.
- `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` - clarify exact proof vs deferred follow-up.
- `test/plugin/llgr-readvertise.ci` - normalize partial-coverage wording if needed.
- `test/plugin/nexthop-self-ipv6-forward.ci` - normalize partial/blocker wording if needed.
- `docs/functional-tests.md` - document any runner limitation that remains a real blocker after the audit.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | targeted `.ci` files above - evidence must match claim |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if any new runner limitation/primitive is documented |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- None required by design. Create a fixture manifest or helper only if that is the smallest honest way to separate native parser coverage from intentionally legacy example files.
- `plan/learned/NNN-bgp-functional-test-evidence.md` - implementation summary and final blocker classification

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section - fix every BLOCKER/ISSUE before full verification |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Example-config coverage audit** - decide whether to convert a native subset or curate intentional exclusions, then remove the blanket skip.
   - Tests: `TestParseAllConfigFiles`
   - Files: `internal/component/bgp/config/loader_test.go`, selected `etc/ze/bgp/*` fixtures if conversion is chosen
   - Verify: tests fail -> implement -> tests pass
2. **Phase: Egress-claim honesty** - fix or downgrade the tests that currently admit they do not prove their claimed AC.
   - Tests: `community-strip.ci`, `role-otc-egress-filter.ci`
   - Files: targeted `.ci` files above, plus any helper/fixture they truly need
   - Verify: targeted functional tests pass and comments match assertions
3. **Phase: Partial-coverage normalization** - make deferred follow-ups and partial coverage explicit without overstating proof.
   - Tests: `bgp-rs-fastpath-ebgp-shared.ci`, `llgr-readvertise.ci`, `nexthop-self-ipv6-forward.ci`
   - Files: targeted `.ci` files above
   - Verify: no targeted file overclaims coverage
4. **Phase: Docs + blocker accounting** - if any blocker remains real, document it in `docs/functional-tests.md` and the learned summary.
   - Tests: relevant targeted test commands still pass
   - Files: `docs/functional-tests.md`, `plan/learned/NNN-bgp-functional-test-evidence.md`

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 is backed by a real assertion, a precise downgrade, or a documented blocker |
| Correctness | No targeted file claims egress-filter or wire-byte coverage without actually exercising the forward/wire path |
| Naming | `VALIDATES`, `STATUS`, and blocker comments use precise terms like `partial` / `blocked`, not vague TODO language |
| Data flow | Tests claiming egress behavior really drive `ForwardUpdate()` / `ForwardUpdatesDirect()` into destination assertions |
| Rule: exact-or-reject | When exact proof is impossible, the file says so explicitly instead of approximating the AC |
| Rule: test evidence | Count-only observer checks are not used to claim wire-path behavior unless that is exactly what the AC is about |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| No blanket skip remains in `TestParseAllConfigFiles` | `rg 't\\.Skip\\(' internal/component/bgp/config/loader_test.go` |
| `community-strip.ci` no longer self-contradicts its AC claim | `rg -n 'VALIDATES|STATUS|TODO|blocked|framework wired' test/plugin/community-strip.ci` |
| `role-otc-egress-filter.ci` no longer self-contradicts its AC claim | `rg -n 'VALIDATES|TODO|blocked' test/plugin/role-otc-egress-filter.ci` |
| Fast-path test distinguishes proven vs deferred scope | `rg -n 'deferred|VALIDATES' test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| Remaining blockers are documented in functional test docs if they still matter | `rg -n 'single-ze-peer|multi-IP|blocked' docs/functional-tests.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| False positives | No helper script can succeed without tripping `runtime_fail(...)` on the intended failure path |
| Hidden skips | No new unconditional skip/downgrade hides a real regression |
| Process cleanup | Updated `.ci` scripts still terminate background processes cleanly and do not turn timeouts into false passes |
| Assertion integrity | Log-only assertions are used only when wire assertions are impossible and that limitation is stated explicitly |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Legacy fixture conversion explodes scope | Curate a minimal native subset and document legacy exclusions instead of converting the whole tree |
| Egress test still cannot prove the claim with current runner primitives | Downgrade the claim and document the missing capability in `docs/functional-tests.md` |
| Reviewer disagrees whether a test is partial vs full | Trace the exact runner assertions and align the label to those, not to intent |
| Updated `.ci` becomes flaky | Prefer deterministic `expect=bgp` / explicit helper markers over sleep-based observer checks |

## Review Gate

### /ze-review Findings
| Severity | File | Finding | Status |
| NOTE | scoped changes | No BLOCKER/ISSUE findings after review of the parser coverage conversion, fixture classification, and targeted `.ci`/docs wording. Targeted verification passed for `TestParseAllConfigFiles` and the edited plugin tests. | clean |
|----------|------|---------|--------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] No targeted test self-reports "TODO" while still claiming full AC coverage
- [ ] Parser coverage for shipped examples is no longer hidden behind a blanket skip

### Design
- [ ] Exact proof where possible, explicit blocker where not
- [ ] No test comment overstates what the assertions prove
- [ ] Runner limitations documented once, not rediscovered ad hoc per test

### TDD
- [ ] Parser coverage fixed with tests first
- [ ] Functional test claims updated only alongside real assertions or precise downgrades
- [ ] Targeted functional tests rerun after each edit

### Completion
- [ ] Critical Review passes
- [ ] Review Gate filled
- [ ] Learned summary written to `plan/learned/NNN-bgp-functional-test-evidence.md`
- [ ] Spec entry removed from `tmp/session/selected-spec`
