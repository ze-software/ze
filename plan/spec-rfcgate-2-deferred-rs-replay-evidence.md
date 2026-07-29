# Spec: rfcgate-2-deferred-rs-replay-evidence

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (nothing deferred out of this spec yet) |
| Updated | 2026-07-29 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/deferral-tracking.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Deferred out of `plan/spec-rfcgate-2-evidence.md` (see
`plan/deferrals/rfcgate-2-evidence.md`).

### The premise this spec was created on is REFUTED (2026-07-29)

~~**The defect.** Ze configured as a route server prepends its own AS to routes
it re-advertises through the adj-rib-in replay-on-peer-up path. RFC 7947 Section
2.2.2.1 says a route server should not insert itself into the AS_PATH, and Ze
honors that on the bgp-rs FORWARD rail; the REPLAY rail is the generic per-peer
re-advertisement path and does not know the peer belongs to a route server.~~

~~**The symptom, reproduced.** `python3 test/interop/run.py
47-rfc7606-relay-shape-frr` fails at Path 1 with `FAIL: AS 65001 found in
AS_PATH for 10.0.0.0/24`.~~

**There is no such defect.** Verified against the producing code, not the
symptom:

| Claim | Producing code | What it shows |
|-------|----------------|---------------|
| One prepend gate serves BOTH rails | `internal/component/bgp/reactor/reactor_api_forward.go:711` -- `if facts.isEBGP && !facts.rsClient` | The replay rail is not a second, RS-unaware path |
| Both rails reach that gate | `RelayStoredRoute` -> `forwardUpdateCore` (`reactor/reactor_api_relay.go:253`); `ForwardUpdate` -> `forwardUpdateCore` (`reactor/reactor_api_forward.go:358`) | The replay rail IS the forward rail below the entry point |
| `rsClient` has exactly one source | `reactor/peer_forward_facts.go:111` (`rsClient: s.RSClient`) <- `reactor/config.go:266` (the `session/rs-client` leaf) | Nothing else can set it |
| Its default is `false` | `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang:40-46` | An unconfigured peer is not an RS-client |
| No interop scenario sets it | `grep -rn 'rs-client' test/interop/` returned nothing **at the moment of the refutation** | Scenario 47's peer was, as configured, a plain eBGP peer |

→ Correction (2026-07-29, same day): that grep now returns ten hits. `rs-client true;`
was added to both peers of scenarios `14-route-server-frr` and `47-rfc7606-relay-shape-frr`
later the same day, each with an inline comment recording that the leaf defaults to false.
The row above is kept in the past tense because it is the EVIDENCE for the refutation, not
a live claim: re-running that grep today proves the fix landed, not that the diagnosis was
wrong. A future reader who runs it and finds hits should read this note rather than
concluding the refutation was mistaken.

So Ze prepended **correctly**. Scenario 47's earlier green depended on an RFC
4271 Section 5.1.2 bug that commit `8bb55e509` ("fix(bgp): prepend the local AS
on eBGP announces, RFC 4271 5.1.2", 2026-07-25) fixed; when the bug went away
the scenario's expectation stopped being satisfiable, and the resulting red was
misread as a new route-server defect. The general lesson is recorded in the
parent spec's Known Limitations: **a test that goes red after an unrelated fix
may have been green *because* of the bug.**

### The real gap, which is what this spec now owns

RFC 7947 Section 2.2.2.1's AS_PATH transparency obligation is proven **only by
unit tests**, and no test pins it through the REPLAY entry point at all:

| Fact | Evidence |
|------|----------|
| `RFC7947-x-1` has unit-only evidence | `ai/RFC-REQUIREMENTS.md:4040` -- positive `forward_rs_test.go:431` (unit/verify), negative `forward_rs_test.go:323` (unit/verify). No functional, editor or interop carrier |
| The one relay test asserts nothing about the AS_PATH | `internal/component/bgp/reactor/reactor_api_relay_test.go:92` (`TestRelayStoredRouteForwardsThroughForwardRail`) asserts the destination peer and `assert.NotEmpty(t, item.rawBodies)`, i.e. that the relay reached the forward pool and that buffers are released. It says nothing about the transform |
| No rs-client relay test exists | `grep -n 'RSClient\|rsClient' internal/component/bgp/reactor/reactor_api_relay_test.go` returns nothing: the relay fixture's destination is never an RS-client |
| The requirement is a SHOULD NOT, not a MUST | `rfc/short/rfc7947.md:49` -- `[RFC7947-x-1] [SHOULD NOT]`. The earlier framing as a "violation" of a MUST overstated it |

The gap is therefore **evidence, not behaviour**: nothing proves that a route
relayed to an rs-client peer through `RelayStoredRoute` is byte-identical in
AS_PATH to what arrived, and the parent spec's whole thesis is that a wire
obligation proven only in-process is proven at the wrong altitude.

### The work

1. Add an rs-client **relay** test pinning AS_PATH byte-identity through
   `RelayStoredRoute`, alongside the existing forward-rail coverage. Both rails
   share the gate at `reactor_api_forward.go:711`, so this is a test for a
   behaviour believed correct -- write it RED first by flipping the gate.
2. Decide the carrier for `RFC7947-x-1`'s non-unit evidence. A `.ci` with two
   rs-client peers is the parent spec's AC-18 default (a `.ci` runs on every
   push); an interop scenario with `rs-client` actually set is the stronger
   proof and the nightly-tier option. Do not pick interop by reflex.
3. Only then re-examine scenario 47. Its Path 1 assertion
   (`check_route_no_as("10.0.0.0/24", "65001")`) is testing route-server
   transparency **on a peer that is not configured as an RS-client**. Either the
   scenario config gains `rs-client`, or the assertion is wrong and must change.
   That decision belongs here, with the evidence, not in the tooling spec.
4. `RFC7606-5.1-3 positive` may then be tagged onto scenario 47 and
   mutation-verified RED per `ai/rules/interop-and-goal-validation.md`.

Note for whoever picks this up: `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py`
still carries a `# NOT YET TAGGED` header repeating the refuted premise. Correct
it in the same change; it was written before the refutation.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [output format, function signature, or `.ci` expectation callers depend on]

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: e.g. "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what we believe] | [where the belief comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what could bite] | [how we would notice] | [what we would do] |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [e.g. "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config-surface.md` (YANG vs env var) and `ai/rules/config-naming.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli-grammar.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves; dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/pipe-completeness.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: entry point exists and is reachable; the wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific: e.g. "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific: e.g. "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific: e.g. "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
