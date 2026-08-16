# Spec: improve-4 -- File-Driven Protocol Conformance Fixtures

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-improve-3-event-replay |
| Phase | - |
| Updated | 2026-07-10 |

Update (2026-07-22 plan review): still hard-blocked -- the Depends
(`spec-improve-3-event-replay`) is `ready`, not started (no capture writer or
replay harness in the reactor), and this spec consumes its JSONL schema and
stub-`net.Conn` harness. Anchor drift fixed in-body: `stagesForMode`
`verify_run.go`, `LoadExpectFile` `expect.go`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `plan/spec-improve-3-event-replay.md` -- capture format this consumes
5. `docs/functional-tests.md` -- existing .ci harness

## Task

Ze has many test layers (.ci functional tests, interop scenarios against FRR/BIRD/
GoBGP, exabgp-compat, stress, QEMU), but no shared file-driven fixture format where one
directory tells a protocol scenario end to end: input config, input event stream,
expected operational state, expected outbound wire events, expected diagnostics. With
such a format, a maintainer can read a scenario without reading harness code, the
fixture doubles as a protocol narrative, and a captured production bug can be dropped
in as a new regression test.

Define `test/protocol/<name>/<scenario>/` fixtures whose event-stream input is the
spec-improve-3 capture format (JSONL), and a runner that loads config, replays the
stream, and diffs actual vs expected state/output/diagnostics. Build exactly ONE BGP
fixture first to prove the format; broadening to more scenarios and protocols is
explicitly follow-up work, not this spec.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - .ci harness capabilities and conventions
  → Decision: dedicated fixture runner hosted as a ze-test subcommand (`registerRoot` pattern, `internal/test/cli/register.go`) + make target; the .ci dialect stays untouched -- fixtures are data directories, not a second script dialect (satisfies the no-layering row below). Directives/parser surveyed 2026-07-10 (parser `internal/test/runner/record_parse.go` parseAndAdd; executor `runner_exec.go,:557`; directives `docs/functional-tests.md`)
- [ ] `plan/spec-improve-3-event-replay.md` - capture/replay machinery this reuses
  → Constraint: fixture event streams use the versioned capture schema, no second format -- schema now ENUMERATED (improve-3 "Capture Format (v1)", 2026-07-10): header line + seq/ts/type events, message bytes base64, config ops with tx-id
- [ ] `ai/rules/testing.md` - where fixture tests sit relative to .ci gate
  → Constraint: read 2026-07-10: the rule's directory table must gain a `test/protocol/` row when this lands (discovery-updates); and a test that EXISTS is not one that GATES -- the runner itself must be mutation-verified, which AC-3 (mutated expected file fails with a diff) provides
- [ ] `plan/deterministic-simulation-analysis.md` - determinism requirements for stable expected-output diffs
  → Constraint: replay asserts OUTCOMES, not interleavings (improve-3 scope); expected-state diffs must go through canonicalization -- precedent exists: `compareJSON`/`normalizeNeighborSection` strip volatile fields (`internal/test/runner/decoding.go`)

### RFC Summaries (MUST for protocol work)
- First fixture exercises existing RFC 4271 behavior; no new protocol claims. Cite
  `rfc/short/rfc4271.md` sections in the fixture's README during design.

**Key insights:**
- The fixture format's value is that production captures (spec-improve-3) become
  regression tests with an expected-output block added.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/session.go` - session loop a fixture run drives via replay (:706)
- [ ] `internal/component/plugin/server/event_ring.go` - diagnostics trail a fixture can assert on (:47)
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - state dump commands usable as expected-state probes (:250-274)
- [ ] `test/` directory layout - existing suites (functional .ci, interop, exabgp-compat, stress) to confirm no equivalent fixture format exists (survey during design)

**Design-phase research completed (2026-07-10; producers read by research agent; digest in tmp/session/session-state-improve-4-conformance-fixtures-56997.md):**
- Survey result: NO fixture format exists. Only ONE golden-file regenerator in the whole tree: `-update` flag in `internal/component/plugin/all/all_test.go` (`snapshot()` :54, write :57-66) with make target `ze-plugin-snapshot-update` (`Makefile:111-113`). This is the regen-UX precedent to mirror.
- State probes are ad-hoc `map[string]any`, no schema: peer state producer `internal/component/bgp/plugins/cmd/peer/peer.go`; adj-rib-in `rib_commands.go` show() / `:220` status(); reached via `opDispatchCommand` (`internal/component/plugin/server/dispatch_registry.go`). Canonicalization is therefore MANDATORY for stable diffs (A-1).
- Outbound-wire assertion machinery already exists in ze-test peer check mode: `LoadExpectFile` (`internal/test/peer/expect.go`), matcher `Checker.ExpectedOrKeepalive` (`checker.go`; marker strip :402-404; `matchRule` :612 prefix:/contains:/exact) -- A-2's expected-wire surface (research agent).
- Topology constraints: everything is TCP over loopback (no netns); single shared bgp port (`ze.test.bgp.port`, `cmd_peer.go`); single-peer-multi-IP scenarios are known-flaky (`docs/functional-tests.md`) -- v1 single-session scope avoids this.
- Test-harness env conventions: typed `env.MustRegister` registry; existing vars ZE_TEST_NO_BUILD/ZE_BIN (`runner.go,:265`), ZE_VERIFY_MODE (`parallel.go`), ZE_SKIP_SUITES (`cmd_web.go`).

**Behavior to preserve:** (unless user explicitly said to change)
- Existing .ci, interop, exabgp-compat, and stress suites unchanged; fixtures are a new
  suite, not a migration.
- `make ze-precommit-verify` stage list changes only by adding the fixture stage (design decision
  whether it joins ze-functional-test or gets its own target).

**Behavior to change:** (only if user explicitly requested)
- None; additive test infrastructure.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-conformance-test` (name per design) discovers `test/protocol/*/*/` fixture
  directories and runs each scenario.

### Transformation Path
1. Runner loads the scenario's input config and starts the component under test (BGP session harness from spec-improve-3).
2. Runner replays the scenario's JSONL event stream through the real processing path.
3. Runner collects actual operational state (via existing dump/show commands), outbound wire events, and diagnostics.
4. Runner diffs actual vs the scenario's expected files; mismatch fails with a readable diff naming the file and field.
5. A new scenario is authored by capturing (spec-improve-3) or hand-writing a stream, then recording expected outputs.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Fixture files ↔ runner | directory convention + versioned schemas | [ ] |
| Runner ↔ session | spec-improve-3 replay harness (same path as production) | [ ] |
| Runner ↔ state probes | existing dump/show command outputs | [ ] |

### Integration Points
- spec-improve-3 replay harness - drives the input stream.
- Existing state dump commands (adj-rib-in dump and peers/summary equivalents) - expected-state probes.
- `mk/` test targets + `scripts/status/verify_run.go` stage list - runner invocation.

### Architectural Verification
- [ ] No bypassed layers (fixtures exercise the real read/process path via replay)
- [ ] No unintended coupling (runner depends on capture format package + CLI probes only)
- [ ] No duplicated functionality (does not reimplement .ci or interop suites)
- [ ] Registration over hardcoding -- runner discovers fixtures from the directory tree; per-protocol probes register, no scenario switch in the runner (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | State/output dumps are deterministic enough to diff byte-stably (ordering, timestamps) | JSON format rules exist (`ai/rules/cli.md`) | Expected files flake; need canonicalization pass | Run the first fixture 100x during implementation | unvalidated |
| A-2 | The spec-improve-3 replay harness can expose outbound wire events for assertion | RESOLVED at design (2026-07-10): the harness drives the session over a STUB net.Conn (improve-3 Files to Create) -- everything the session sends is written to that stub; the runner captures writes, re-frames by BGP header length, and matches with the existing rule semantics (`matchRule`, `internal/test/peer/checker.go`: exact/prefix:/contains: on hex). No new reactor surface needed. See "Expected-Wire Observation" below | - | TestFixtureWireCapture unit test in phase 2 | confirmed (mechanism specified) |
| A-3 | One directory format fits future protocols (OSPF/IS-IS) without redesign | the review reports another daemon running 10+ protocols on one such format (unverified) | Format revision needed when a second protocol lands | Sketch an OSPF scenario on paper during design (no implementation) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Expected-output files rot as features evolve | frequent fixture churn in PRs | keep expected files small and semantic (state slices, not full dumps); regeneration command with review diff |
| R-2 | Fixture suite duplicates what a .ci already covers, doubling maintenance | design-phase overlap review | first fixture targets a scenario .ci cannot express (byte-level captured stream in, state out) |
| R-3 | Runner grows protocol-specific logic | code review | probes register per protocol; runner stays generic |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| make ze-conformance-test | → | fixture discovery + runner | TestFixtureRunnerDiscovers |
| first BGP scenario directory | → | replay -> state diff -> pass/fail | test/protocol/bgp/basic-session scenario (runner-executed) |
| deliberately broken expected file | → | readable diff failure | TestFixtureRunnerFailsWithDiff |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-conformance-test` | Discovers and runs all `test/protocol/*/*/` scenarios |
| AC-2 | First BGP scenario (session establish + UPDATEs) | Passes: actual state/output/diagnostics match expected files |
| AC-3 | Mutated expected file | Fails with a diff naming file and mismatching field |
| AC-4 | Scenario missing a required file | Clear error naming the missing file, not a panic |
| AC-5 | Same scenario run repeatedly | Identical result (determinism, A-1) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Developer converts a spec-improve-3 production capture into a regression test | capture file + config + recorded expected state -> new scenario dir | test/protocol/bgp/basic-session scenario |
| 2 | Maintainer reads a scenario to understand protocol behavior | fixture directory is self-describing | scenario README convention (design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFixtureRunnerDiscovers | runner package test | directory discovery, schema validation | |
| TestFixtureRunnerFailsWithDiff | runner package test | mismatch reporting quality | |
| TestFixtureCanonicalization | runner package test | deterministic diffable output (A-1) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| basic-session | `test/protocol/bgp/basic-session/` (runner suite; harness-level equivalent of a .ci) | replayed session produces expected RIB state + outbound events | |
| runner-gate | `test/protocol/runner.ci` (if .ci integration chosen at design) | conformance stage wired into functional gate | |

### Interop Tests (MANDATORY for protocol features)
- N/A: no new wire behavior; interop suites remain the cross-daemon check.

## Files to Modify
- `mk/` test makefiles - `ze-conformance-test` target
- `scripts/status/verify_run.go` - add stage (design decision: default vs opt-in) (`stagesForMode` :214-280 stage lists)
- `docs/functional-tests.md` - document the fixture format

## Files to Create
- fixture runner (location per module tiers during design)
- `test/protocol/bgp/basic-session/` - config, events.jsonl, expected-state, expected-output, expected-diagnostics, README
- `test/protocol/README.md` - format specification

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | test infrastructure, no config surface |
| YANG validation constraints | N/A | none |
| YANG custom validators | N/A | none |
| CLI commands/flags | Yes | ze-test subcommand for the runner (`internal/test/cli/register.go` registerRoot) |
| CLI grammar (action before identifier) | Yes | verify subcommand name against `ai/rules/cli.md` at implementation |
| Editor autocomplete | N/A | none |
| Functional test for new RPC/API | Yes | the basic-session scenario itself + TestFixtureRunnerFailsWithDiff (mutation-verify per functional-test-gate) |
| Pipe completeness | N/A | runner is a host test tool, not a NOS CLI command |
| Env var registration | N/A | regen is a flag, not an env var (Key Design Decisions) |
| Doctor check for runtime dependencies | N/A | host-side tool; no appliance runtime dependency |
| Prometheus counters/metrics | N/A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer test infrastructure |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | ze-test subcommand documented in `docs/functional-tests.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | fixture READMEs cite existing rfc/short sections, no status change |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (format spec) + `ai/rules/testing.md` directory table row + `ai/rules/testing.md` per discovery-updates |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (conformance-fixture capability) |
| 12 | Internal architecture changed? | No | additive |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | command inventory + `ai/INDEX.md` keyword row |
| 16 | Any changed source file is referenced by existing doc source anchors? | Check at implementation | grep `docs/` for anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none exist yet |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - runner skeleton + make target + failing discovery test
2. **Phase: format + first scenario** - schema, canonicalization, basic-session fixture
3. **Phase: diff quality + failure modes** (AC-3, AC-4)
4. **Phase: verify integration** - stage wiring, determinism soak (AC-5)
5. `make ze-precommit-verify`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 with file:line |
| Correctness | replay path is the production path; expected files semantic, not dump-everything |
| Registration over hardcoding | probes register per protocol; runner generic (`ai/rules/plugins.md`) |
| Rule: no-layering | no parallel mini-.ci dialect; one fixture format |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | runner parses fixture files defensively (they may come from captures of hostile peers) |
| Resource exhaustion | scenario timeouts; bounded replay |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Expected-Wire Observation (added 2026-07-10 at design gate, per user request)

The replay harness's stub `net.Conn` is the wire tap: every byte the session sends
goes through the stub's Write. The runner:
1. Buffers stub writes and re-frames them into BGP messages by header length
   (16-byte marker + 2-byte length, same framing production reads use).
2. Hex-encodes each outbound frame (marker-stripped variant supported, mirroring
   `Checker.ExpectedOrKeepalive` `checker.go`).
3. Matches ordered frames against the scenario's `expected-wire` file, one rule per
   line, reusing the `matchRule` rule set (`checker.go`): exact hex (default,
   case-insensitive), `prefix:<hex>`, `contains:<hex>`; plus `keepalive` as a
   convenience token (KEEPALIVEs are otherwise noise).
4. In `-update` mode the observed frames are written back as exact-hex rules;
   authors then loosen to prefix:/contains: where fields are volatile.

## Canonicalization Contract (added 2026-07-10 at design gate, per user request)

Expected-state files are canonical JSON slices; the runner canonicalizes ACTUAL
state before diffing:
1. Each expected-state file declares its probe (the dispatch command producing the
   state, e.g. `show bgp adj-rib-in`) and the JSON paths included -- a slice, never
   the whole dump (R-1).
2. Canonical form: object keys sorted lexicographically; kebab-case keys as produced
   (per `ai/rules/cli.md`); volatile fields removed per a strip-list that is
   VERSIONED WITH THE RUNNER (seeded from the fields `normalizeNeighborSection`
   already strips, `internal/test/runner/decoding.go`); numbers formatted uniformly.
3. Arrays compare ordered by default; a probe may declare set-semantics for a path
   (order-insensitive compare) -- declared in the expected file, not runner code.
4. The strip-list and set-semantics declarations live in the fixture format spec
   (`test/protocol/README.md`) so a failing diff is explainable from fixture files
   alone.

## Design Insights
- The five-surface layout mirrors the reviewed daemon's four captured surfaces
  (verified `holo-protocol/src/test/stub/mod.rs:320-429`); diagnostics is Ze's
  addition because its event ring and doctor codes are queryable.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One BGP fixture first | fixture sets for every protocol | prove the format before spending on breadth (the review's own advice) |
| Reuse spec-improve-3 capture schema for event input | separate fixture event dialect | production captures become fixtures with zero translation |
| Regen via `-update` flag on the runner + `make ze-conformance-update` | HOLO_UPDATE-style env var | mirrors the tree's ONLY existing golden regenerator (`all_test.go` + `Makefile:111-113`); env conventions stay for harness plumbing, flags for regen |
| Expected-state files are semantic slices diffed after canonicalization | full-dump byte compare | state producers are ad-hoc maps with volatile fields; `compareJSON`/`normalizeNeighborSection` precedent (`internal/test/runner/decoding.go`) makes slice-diffs stable (A-1, R-1) |
| Runner hosted as ze-test subcommand + make target | extending the .ci dialect with fixture directives | fixtures are data, .ci is a script dialect; mixing them creates the parallel-dialect layering R-3/no-layering forbids |
| Verify wiring goes in BOTH `stagesForMode` branches ONLY | Makefile `_ze-verify-impl`/`_ze-verify-changed-impl` | those Makefile targets are documented dead with zero callers (post-wave corrections below; `Makefile:280-287`) |
| Fixture surfaces: config-in, events-in, expected-state, expected-wire, expected-diagnostics | state-only fixtures | mirrors the four-surface transducer model verified at primary source in the reviewed daemon (`holo-protocol/src/test/stub/mod.rs:320-429`, collector `stub/collector.rs:83-161`); Ze adds diagnostics as a fifth surface because its event ring + doctor codes are queryable |

## Known Limitations
- v1 covers single-session BGP scenarios; topology/convergence fixtures are follow-up.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-standard-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Post-wave corrections (2026-07-10)

Re-verified against the followup implementation wave (unpushed origin/main..HEAD commits):

- The live producer of the `make ze-precommit-verify` stage list is `stagesForMode` (`scripts/status/verify_run.go`, consumed at `:137` and `:192`). The wave inserted `ze-port-defaults-check` and `ze-platform-vet` into BOTH branches (`ze-precommit-verify-changed` branch at `:233`/`:235`, default branch at `:255`/`:257`); the function now spans `:214-280` (Files to Modify updated to match).
- The planned conformance stage must be added to BOTH branches of `stagesForMode`, and NOT to the Makefile `_ze-verify-impl` / `_ze-verify-changed-impl` targets: those have zero callers and are documented as dead (`Makefile:280-287` comment); a stage added only there never runs under `make ze-precommit-verify` or CI.
- `docs/functional-tests.md` (Required Reading item 1) has grown since this spec was written, e.g. the MCP GET-SSE section (`docs/functional-tests.md`), and new `.ci` suites exist under `test/plugin/` (`as112-dot.ci`, `as112-doh.ci`, `exabgp-bridge-internal.ci`, `exabgp-bridge-sdk.ci`). The Current Behavior test-layout survey must be redone against the current tree at design time.
