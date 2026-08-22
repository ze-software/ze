# Spec: fixit-functional-suite-pins-the-unshipped-backend

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Every functional test runs against a storage backend Ze does not ship, so no
`.ci` or `.wb` can see a defect in the one operators get.**

Two producers were read on 2026-08-22 and both hold today.

`ze_core_dispatch.go` registers `ze.storage.blob` with `Default: "true"`
(`cmd/ze/ze_core_dispatch.go`, the `env.MustRegister` call). Blob storage is what
a shipped daemon uses.

`runner_exec.go` appends `ze.storage.blob=false` to the daemon environment in two
places (`internal/test/runner/runner_exec.go`, the suite launch path and the
process launch path). Every daemon the functional runner starts therefore uses
filesystem storage.

So the suite's green says nothing about the shipped configuration. A defect that
exists only under blob storage passes every `.ci` and every `.wb`, and the suite
reports full coverage while exercising a path no operator runs.

**This is not hypothetical, and the instance is what found it.**
`spec-fixit-zefs-diff-structural-ops` closed a defect that appeared only under
blob storage: the pending-change review UI rendered a count over an empty body.
It was reachable from the web editor, which is an operator surface, and the
functional suite could not see it. That spec fixed the same class one layer down
for `.et` tests by giving the runner a `storage` option. The `.ci` and `.wb`
runners still have no such seam, which is why the surface the defect's own Task
names, `/config/diff`, has no functional test.

**The pin is presumably deliberate and its reason is not recorded.** Filesystem
storage is easier to inspect from a test, and a suite written before blob storage
became the default would have had no reason to choose. Establish the reason
before removing the pin: a test that reads a config file directly from disk
breaks under blob storage, and that is a real cost to weigh rather than a
surprise to hit.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - what blob storage is and how a key resolves
  → Constraint: a blob store resolves a directory key differently from a file path, which is the seam the closed spec's defect lived in.
- [ ] `docs/architecture/testing/runner-architecture.md` - how the functional runner launches a daemon
  → Constraint: the runner owns the daemon environment. A per-test option is the established way to vary it, and `.et` already has one.
- [ ] `docs/functional-tests.md` - what the suites claim to cover
  → Constraint: the page states what a green run means. It must stop implying coverage of a backend no test runs.

**Key insights:**
- A suite that pins a non-default configuration reports coverage of something nobody ships.
- The `.et` runner already solved this with a `storage` option, so the shape is established and does not need designing.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/ze_core_dispatch.go` - registers `ze.storage.blob` with default `true`
- [ ] `internal/test/runner/runner_exec.go` - appends `ze.storage.blob=false` at two launch sites
- [ ] `internal/component/cli/testing/runner.go` - `runTestCase` and its `case "storage"` branch, the seam `.et` already has
- [ ] `internal/component/config/storage/blob.go` - `(*blobStorage).List` and `resolveDirKey`, the producer whose defect the suite could not see

**Behavior to preserve:**
- A test that genuinely needs filesystem storage keeps getting it, by asking rather than by inheriting a suite-wide default.
- The runner keeps one daemon environment producer. This adds an option to it; it does not add a second path.
- Existing `.ci` and `.wb` assertions keep passing where they are backend-independent.

**Behavior to change:**
- The functional runner defaults to the SHIPPED backend, so a green means what a reader assumes it means.
- A test that reads config state from the filesystem declares that it needs filesystem storage, and the declaration is visible in the test.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` or `.wb` file is run by the functional runner, which launches a `ze` daemon.
- Format at entry: the test file's directives, plus the environment the runner composes.

### Transformation Path
1. The runner reads the test file and composes the daemon environment (`internal/test/runner/runner_exec.go`) -- the defect is here.
2. `ze.storage.blob=false` is appended unconditionally at two launch sites.
3. The daemon starts, and `ze_core_dispatch.go` reads the key it registered with default `true`, finding the override.
4. Every config read and write in that daemon goes through filesystem storage.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ daemon | the composed process environment | No |
| Daemon ↔ storage | `config.Storage`, blob or filesystem | No |

### Integration Points
- `runner_exec.go` - the single composer of the daemon environment
- `runTestCase`'s `case "storage"` branch (`internal/component/cli/testing/runner.go`) - the precedent this follows

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The pin has a reason, and the reason is that some tests read config state from the filesystem | filesystem storage is inspectable from a shell; blob storage is not | removing the pin breaks a population nobody sized, and the phase-1 count is the answer | run the whole suite once with the pin removed and count what fails | unvalidated |
| A-2 | The failures a flipped default produces are test-harness failures, not product defects | the product ships blob storage, so a product defect under blob would already reach operators | each failure is a real defect the suite was hiding, which makes this spec much larger and is the outcome worth knowing early | read the first several failures at their producers before fixing any | unvalidated |
| A-3 | A per-test `storage` option is enough, and no test needs BOTH backends in one run | the `.et` runner's option works that way, and the closed spec's pair of files is two tests rather than one | the seam needs to parameterize a test rather than select for it | the count from A-1, split by cause | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Flipping the default reddens a large population at once and stalls the release | the phase-1 count is large | the count is taken FIRST, before any change lands. If it is large, the default flips per suite rather than at once, and the order is recorded here |
| R-2 | A failure is read as a harness problem and papered over with the option, hiding a real product defect | a test gains `storage: filesystem` with no reason stated | every use of the option states why that test needs the backend it asks for, and A-2 is settled by reading producers rather than by assuming |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing ships differently. The cost of being wrong is a red suite and a stalled release, not a defect reaching an operator |
| How is it reverted? | Single commit revert. The pin returns and the suite is exactly as it is today |
| Who else touches this path? | Every functional suite. `spec-fixit-zefs-diff-structural-ops` closed the `.et` half of this on 2026-08-22 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `.ci` that asserts which backend the daemon is using | → | the runner's environment composer | `test/plugin/storage-backend-is-the-shipped-default.ci` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
| A `.ci` that declares it needs filesystem storage | → | the new option | `TestRunnerStorageOptionSelectsTheBackend` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `.ci` with no storage directive | The daemon runs the shipped default, blob storage |
| AC-2 | A `.ci` declaring it needs filesystem storage | The daemon runs filesystem storage, and the declaration is in the test file |
| AC-3 | The whole functional suite | It passes, with every filesystem declaration carrying a stated reason |
| AC-4 | A defect reachable only under blob storage | At least one `.ci` can fail for it, proven by reverting the closed spec's `List` fix and observing a red |
| AC-5 | The runner is grepped for a pinned non-default env value | No environment key is pinned away from its registered default without a test-visible declaration |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Edits configuration through the web editor on a shipped daemon and reviews the pending changes | web editor → `EditorManager.Diff` → blob storage | `test/web/config-diff-structural-op.wb` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
| 2 | Runs the functional suite to gain confidence before a release | runner → daemon on the shipped backend | the suite itself |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunnerStorageOptionSelectsTheBackend` | `internal/test/runner/runner_exec_test.go` | the option reaches the daemon environment, both values | |
| `TestRunnerDefaultsToTheShippedBackend` | `internal/test/runner/runner_exec_test.go` | validates AC-1: no directive means blob | |
| `TestNoEnvKeyIsPinnedAwayFromItsDefault` | `internal/test/runner/runner_exec_test.go` | validates AC-5 structurally, so the next pin cannot be silent | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the option is an enumeration of two backends, not a number | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `storage-backend-is-the-shipped-default` | `test/plugin/storage-backend-is-the-shipped-default.ci` | the daemon a test drives is the daemon an operator runs | | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
| `config-diff-structural-op` | `test/web/config-diff-structural-op.wb` | the surface the closed spec's Task named, finally covered | | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Storage backend selection is local and reaches no wire. No peer can observe it | N-A |

## Files to Modify
- `internal/test/runner/runner_exec.go` - the default becomes the shipped backend, and a per-test option selects the other
- `docs/functional-tests.md` - state which backend a suite runs and how a test asks for the other
- `docs/architecture/testing/runner-architecture.md` - the environment composer's contract
- `docs/architecture/testing/ci-format.md` - the design doc `runner_exec.go` declares: the new storage directive joins the `.ci` vocabulary
- Every `.ci` or `.wb` the phase-1 count identifies, each gaining a declaration with a stated reason

## Files to Create
- `test/plugin/storage-backend-is-the-shipped-default.ci` - the AC-1 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
- `test/web/config-diff-structural-op.wb` - the AC-4 proof, and the closed spec's missing coverage <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no operator-visible setting changes; `ze.storage.blob` already exists |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | No | `ze.storage.blob` is already registered; this stops overriding it |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | a test harness surface |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a coverage hole is closed |
| 2 | Config syntax changed? | No | no leaf changes |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | the runner is not an operator surface |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC obligation is touched |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, and the runner architecture page. Also `docs/architecture/testing/ci-format.md`, the design doc `runner_exec.go` declares in its `// Design:` header: it defines the `.ci` directive vocabulary, and this spec adds a storage directive to it |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/runner-architecture.md`, the environment composer's contract |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `runner_exec.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify any documented runner directive list |

## Implementation Steps

1. **Phase: Count it, and validate A-1 and A-2 (MANDATORY FIRST)** -- remove the pin locally, run the whole functional suite, and count what fails
   - Files: this spec's Assumptions table, and a recorded list of the failing tests grouped by cause
   - Verify: A-1 and A-2 flip to `confirmed` or `broken`. **Read the first several failures at their producers before fixing any.** A failure that is a real product defect under the shipped backend is the outcome that matters most, and it must not be papered over with the option
   - If the count is large, record the per-suite order here before anything lands
2. **Phase: Wiring** -- the option and its tests, with the default still pinned
   - Tests: `TestRunnerStorageOptionSelectsTheBackend`, `storage-backend-is-the-shipped-default`
   - Files: `internal/test/runner/runner_exec.go`
   - Verify: a test can ask for either backend, and the `.ci` reports which one it got
3. **Phase: Flip the default** -- the shipped backend becomes what a test gets by default
   - Tests: `TestRunnerDefaultsToTheShippedBackend`, plus the suite
   - Verify: the suite is green, and every filesystem declaration states its reason
4. **Phase: Prove it can fail** -- close AC-4
   - Tests: `test/web/config-diff-structural-op.wb` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
   - Verify: reverting `(*blobStorage).List` to `resolveKey` reddens it. Without this the spec has moved the pin and proven nothing
5. **Phase: Close the class** -- no env key is pinned away from its default silently
   - Tests: `TestNoEnvKeyIsPinnedAwayFromItsDefault`
   - Verify: the guard fails when a pin is added without a declaration

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | Every filesystem declaration states a reason, and none was added to silence a failure nobody read |
| Data flow | One environment composer, one option, no second launch path |
| Rule: `ai/rules/completion.md` | A red found in phase 1 that is a product defect gets fixed or gets its own spec. It is never resolved by declaring the test filesystem-only |
| Rule: `ai/rules/interop-and-goal-validation.md` | AC-4 is the discrimination proof for this whole spec. Without it the change is a preference |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The suite runs the shipped backend by default | `TestRunnerDefaultsToTheShippedBackend` |
| A blob-only defect can redden a functional test | revert `(*blobStorage).List` and run `config-diff-structural-op.wb` |
| No silent pin remains | `TestNoEnvKeyIsPinnedAwayFromItsDefault` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None: the option is a test-file directive read by the runner, not operator input |
| Resource exhaustion | Blob storage has a different write pattern from filesystem. If the suite slows materially, record the measurement rather than reverting on impression |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A suite that pins a non-default value reports coverage of a configuration nobody ships, and the report is indistinguishable from real coverage.
- The cost of the pin is invisible until a defect escapes through it, which is why the instance that found this one came from a closed spec rather than from the suite.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Default to the shipped backend and let a test ask for the other | **B. Run every test twice, once per backend.** REJECTED for the first release: it doubles the suite's cost to cover a difference most tests cannot express. Worth revisiting once the phase-1 count says how many tests are backend-sensitive. **C. Leave the pin and add blob-only tests beside it.** REJECTED: it leaves the default suite testing an unshipped configuration, which is the defect | The `.et` runner already took this shape on 2026-08-22, so this makes the `.ci` and `.wb` runners consistent with a decision already made rather than inventing one |

## Known Limitations

- Phase 1 bounds this spec, and its count is not known when the spec is written. A large count changes the landing order and is recorded here before anything lands.

## RFC Documentation (Scope: protocol)

N-A. Storage backend selection reaches no wire and no RFC obligation.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
