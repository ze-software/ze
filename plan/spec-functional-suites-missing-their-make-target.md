# Spec: functional-suites-missing-their-make-target

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/functional-suites-missing-their-make-target.md` |
| Handoff | verify |
| Updated | 2026-08-11 |

<!-- Handoff `verify`: the implementation session commits its work, sets Status to
     `verification`, and stops. A later Opus 5 session reviews that commit and closes
     the spec. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The original defect was found on 2026-08-11. The retired functional-suite
failure block printed `make ze-<suite>-test`, and three names had no target:
`ze-ldp-test`, `ze-rsvpte-test`, and `ze-install-test`. The historical
reproduction returned `No rule to make target` for each name.

The clean cutover removed that target namespace. `internal/le/functional/suites.go`
now owns the suite names, and `Suite.Rerun` prints `./le functional <suite>`.
The current actions are `./le functional ldp`, `./le functional rsvpte`, and
`./le functional install`.

The remaining task is to verify that every suite in `functional.Suites` is
reachable by its area-local name, every failure hint uses that native form, and
no native action or report reconstructs a retired `ze-*-test` identity.

## Historical migration record (do not execute)

The sections below preserve the original target-addition design, measurements,
and decisions. Every Make command in that record is past evidence. Its current
replacement is `./le functional <suite>`, and current source is
`internal/le/functional/actions.go` plus `internal/le/functional/suites.go`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [ ] `ai/rules/commands.md` - the rule that a suite runs through `make`, not
  through the runner binary
  → Constraint: the "Running this session's `ze-test` binary yourself" paragraph
  states that a direct `$(ZEBIN_TEST)` call is not equivalent to
  `make ze-<suite>-test`, because it rebuilds a `ze` without the test build tags.
  A suite with no target forces the operator into the path this rule refuses.
- [ ] `ai/rules/testing.md` - how a `.ci` earns its verify tier
  → Constraint: the carrier table derives a `.ci` tag's tier from the
  `all_suites` line. That line is load-bearing beyond this file, so this spec
  adds no name to it and removes none.
- [ ] `ai/patterns/functional-test.md` - the directory to make-target table
  → Decision: that table is the documented map from a test directory to its make
  target, so the three new targets belong in it.

**Key insights:** (minimal context to resume after compaction)
- The `all_suites` line in `internal/le/functional/suites.go` is the single source of truth
  for the gated suite set. This spec reads it and adds no second list.
- Every standalone suite target is two lines with the same shape. The work is
  three copies of that shape plus a `.PHONY` edit.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `internal/le/functional/suites.go` - the `all_suites` line sets the gated suite set and
  the progress denominator. The `run_suite` shell function records each failed
  name, one `run_suite` call runs each suite, and the failure block prints
  `  make ze-<suite>-test` for every failed name. Below that block sit 33
  standalone suite targets. `ze-functional-isis-test` is the reference shape: prerequisite
  `$(ZE_TEST_DEPS)`, then one recipe line
  `@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) isis --all`.
  A `.PHONY` block near the head of the file declares the targets.
- [ ] `internal/le/` native action tables - includes `internal/le/functional/suites.go` and defines no suite target
  itself, so `internal/le/functional/suites.go` is the only file to edit for the targets.

Confirmed by reading the tree on 2026-08-11:

| Fact | Evidence |
|------|----------|
| `test/ldp/` exists with 3 `.ci` files | `ldp-convergence.ci`, `ldp-reload.ci`, `ldp-session.ci` |
| `test/rsvpte/` exists with 5 `.ci` files | `rsvpte-bandwidth.ci`, `rsvpte-frr.ci`, `rsvpte-lsp-setup.ci`, `rsvpte-lsp-teardown.ci`, `rsvpte-reroute.ci` |
| `test/install/` exists with 40 `.ci` files | includes `install-help.ci`, `qemu-full.ci`, `qemu-iso.ci` |
| The three aggregate lines take no extra arguments | the `run_suite ldp`, `run_suite rsvpte` and `run_suite install` lines each read `$(SUITE_RUN) $(ZE_TEST_RUN) <suite> --all` |
| None of the three suites needed the retired `ze-functional-warm` prerequisite; the native runner now builds lazily | `grep -rn "exec=go test" test/ldp test/rsvpte test/install` returned nothing |
| None of the three suites needs `$(ZE_TEST_DEPS_STRIPPED)` | `grep -rln "ze-stripped" test/ldp test/rsvpte test/install` returns nothing |
| `ze-functional-ipsec-test` is defined but absent from `.PHONY` | the target exists beside `ze-functional-policy-test`; the `.PHONY` block does not name it |

Behavior to preserve: the four properties below stay true after the change.
- The `all_suites` line stays the one place the gated suite set is written.
- Each suite target keeps the `SUITE_RUN` wall-clock cap and the `ZE_ALT_BUILD`
  isolated-binary path that the 33 existing targets use.
- A standalone target and the aggregate `run_suite` line for the same suite run
  the same command with the same arguments.
- Suites that have a runner and no `all_suites` entry (static, traffic,
  flow-export, vpp, vrrp, chaos-web) stay out of the gating run.

Behavior to change:
- Three suite names gain a standalone make target. Nothing else changes: no
  suite enters or leaves `all_suites`, no `.ci` file is edited, and no product
  code is touched.

## Data Flow (MANDATORY)

### Entry Point
`./le functional` (the aggregate gate), the retry hint it prints on
failure, and `make ze-<suite>-test` typed by an operator for one suite.

### Transformation Path
1. `./le functional` reads the `all_suites` line and sets the progress
   denominator.
2. `run_suite` runs each suite and appends every failed name to `failed_names`.
3. The failure block prints one `  make ze-<suite>-test` line per failed name.
4. The operator runs that command. Make either finds the target and runs the
   suite, or exits 2 with `No rule to make target`.
5. A found target arms the cleanup trap, builds the isolated binary set, and
   runs the suite under the wall-clock cap.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| `all_suites` line to the run loop | shell word split over the suite set | Yes, read in `internal/le/functional/suites.go` |
| Failure block to the operator | a printed make command name | Yes, the `printf "  make ze-%s-test\n"` call |
| Suite target to the test runner | `$(ZE_TEST_RUN)`, which pins `ZE_TEST_NO_BUILD=1`, `ZE_BIN` and `ZE_TEST_BIN` at the isolated set | Yes, the `ZE_TEST_RUN` definition |

### Integration Points
- `$(ZE_ALT_BUILD)` and `$(ZE_ALT_TRAP)` build and remove the isolated binary
  set. Each new target uses both.
- `$(SUITE_RUN)` applies the 600s process-group cap from `ZE_SUITE_TIMEOUT`.
- `$(ZE_TEST_DEPS)` is empty in the default isolated mode and `$(ZEBIN_TEST)`
  under `ZE_TEST_CANONICAL=1`.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | each new target calls `$(ZE_TEST_RUN)`, the same runner entry every sibling target uses |
| No unintended coupling (components stay isolated) | Yes | the change is inside one make file; no product package is touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | the suite set stays in `all_suites`; the new targets add no second list of names |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no data path is touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | a make target is not a runtime feature. The runner already discovers these suites; the change adds no dispatch entry to any Go package |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three suites pass when run on their own | they run inside `./le functional`, which is the merge gate | a red is a suite defect, not a target defect. It is attributed and fixed or reported under `ai/rules/completion.md`; the spec does not close over it | the three recorded runs in Phase 3 | unvalidated |
| A-2 | `$(ZE_TEST_DEPS)` is the correct prerequisite for all three | no `ze-stripped` reference under `test/ldp`, `test/rsvpte` or `test/install`, and the canonical branch sets `ZE_TEST_DEPS` to `$(ZEBIN_TEST)` | a suite cannot find a binary, which the Phase 3 run shows at once | the Phase 3 runs, in the default isolated mode | unvalidated |
| A-3 | The install suite finishes inside the 600s `SUITE_RUN` cap on its own | it finishes inside the same cap as part of the aggregate run | see R-1 | the recorded duration of the retired `ze-install-test` (current: `./le functional install`) | unvalidated |
| A-4 | No other `internal/le/` already defines these three names | `make -n` on each name exits 2 with `No rule to make target` | a duplicate definition warns at parse time | `make -n` on each name after Phase 1 | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the retired `ze-install-test` (current: `./le functional install`) trips the 600s cap the aggregate shares | exit code 124 from `timeout` | the cap is shared with the aggregate on purpose. Do not raise `ZE_SUITE_TIMEOUT` for the new target alone: that makes the standalone and the aggregate disagree, which is the defect this spec closes. Record the measured duration and report it |
| R-2 | A suite is red for a reason this change did not cause | the same `.ci` fails inside `./le functional` | attribute the red by test name (`ai/rules/commands.md`), then fix it or report it. Do not weaken the suite, and do not close the spec over it |
| R-3 | Two suites run at once and poison each other through shared ports or the throwaway root | flaky failures that do not reproduce | run the three targets one at a time, and never beside a verify run (`ai/rules/commands.md`) |
| R-4 | A doc edit goes stale against a source anchor | `./le doc check verify` fails on an anchor | the four anchors naming `internal/le/functional/suites.go` cover the suite list, the non-gated targets, the isolated-binary block, and `ze-functional-isis-test`. None of the four claims changes. Run `./le doc check verify` after the doc edits |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A wrong target either fails to parse, which stops every `make` call in the repository, or runs the wrong suite. Both are visible on the first run |
| How is it reverted? | Single commit revert. The change adds lines and removes none |
| Who else touches this path? | Any session that adds a functional suite edits the same file. `internal/le/rfc/rfc.go` reads the `all_suites` line through `_ALL_SUITES_RE` to derive `.ci` verify tiers; this change does not edit that line, so the derivation is unaffected |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the retired `ze-ldp-test` (current: `./le functional ldp`) | → | the `ze-ldp-test` recipe in `internal/le/functional/suites.go` | the recorded run passes `test/ldp/ldp-session.ci`, `test/ldp/ldp-convergence.ci` and `test/ldp/ldp-reload.ci`, 3 of 3 |
| the retired `ze-rsvpte-test` (current: `./le functional rsvpte`) | → | the `ze-rsvpte-test` recipe in `internal/le/functional/suites.go` | the recorded run passes `test/rsvpte/rsvpte-lsp-setup.ci` and the other four files, 5 of 5 |
| the retired `ze-install-test` (current: `./le functional install`) | → | the `ze-install-test` recipe in `internal/le/functional/suites.go` | the recorded run passes `test/install/install-help.ci` and the rest of the suite, 0 failures over 40 files |
| the failure hint of `./le functional` | → | the `printf "  make ze-%s-test\n"` block in `./le functional` | `make -n ze-ldp-test`, `make -n ze-rsvpte-test` and `make -n ze-install-test` each print a recipe and exit 0, where each printed `No rule to make target` and exited 2 before the change |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the retired `ze-ldp-test` (current: `./le functional ldp`) on a clean tree | the ldp suite runs and reports 3 of 3 tests passed, with no test skipped for a missing binary |
| AC-2 | the retired `ze-rsvpte-test` (current: `./le functional rsvpte`) on a clean tree | the rsvpte suite runs and reports 5 of 5 tests passed |
| AC-3 | `./le functional install` on a clean tree | the install suite runs over `test/install/*.ci` and reports 0 failures. Its pass, skip and fail counts match the install slice of the aggregate run |
| AC-4 | The recipe text of each new target, compared against that suite's `run_suite` line | the command and its arguments are identical, `$(SUITE_RUN) $(ZE_TEST_RUN) <suite> --all`, with the same `$(ZE_ALT_TRAP)` trap and `$(ZE_ALT_BUILD)` prefix as `ze-functional-isis-test` |
| AC-5 | `./le functional` after the change | it still runs 24 suites. The progress counter reaches `[24/24]` and the final line reads `PASS  all 24 suites` when no suite is skipped |
| AC-6 | `grep '^.PHONY' internal/le/functional/suites.go` | the declarations name `ze-ldp-test`, `ze-rsvpte-test`, `ze-install-test`, and `ze-functional-ipsec-test`, which is the one existing target the same block omits |
| AC-7 | `make -n ze-ldp-test`, `make -n ze-rsvpte-test`, `make -n ze-install-test` | each prints a recipe and exits 0. Before the change each printed `No rule to make target` and exited 2 |

## 🧪 TDD Test Plan

The hard part of this spec is proof. A make target has no unit test. "The target
exists" is not evidence that it works. A target that resolves and runs zero tests
satisfies existence and fails the goal. Every AC above therefore binds to an
observed run with a test count, never to the presence of a line.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| N-A. A make target has no unit-test surface, and this spec adds no Go, Python or shell code to drive. The proof is the recorded suite runs below plus the `make -n` probe of AC-7 | - | - | - |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A. The change adds no numeric input. `ZE_SUITE_TIMEOUT` keeps its existing default | - | - | - | - |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the retired `ze-ldp-test` (current: `./le functional ldp`) | `test/ldp/*.ci` | an operator whose ldp suite failed inside `./le functional` runs the printed command and sees the 3 ldp tests run | |
| `./le functional rsvpte` | `test/rsvpte/*.ci` | the same operator reruns the rsvpte suite alone and sees its 5 tests run | |
| `./le functional install` | `test/install/*.ci` | the same operator reruns the install suite alone, with no hand-built binary set | |
| `./le functional` | all 24 gated suites | the gate still runs every suite and still prints a runnable command for each failure | |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. The change adds no wire-visible behavior and no protocol code, so `ai/rules/interop-and-goal-validation.md` requires no scenario. The ldp and rsvpte suites this makes reachable keep the protocol coverage they already had | - |

## Files to Modify

- `internal/le/functional/suites.go` - add the `ze-ldp-test`, `ze-rsvpte-test` and
  `ze-install-test` targets beside their siblings; add those three names and
  `ze-functional-ipsec-test` to the `.PHONY` block; add three lines to the quick-reference
  comment at the head of the file
- `ai/patterns/functional-test.md` - add `test/ldp/`, `test/rsvpte/` and
  `test/install/` rows to the directory to make-target table
- `docs/functional-tests.md` - the sentence
  "Run the install suite with `bin/ze-test install --all`" in the install
  section. Name `./le functional install` there, because that is now the documented
  way to run the suite

## Files to Create

None.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | the change adds no config leaf |
| YANG validation constraints | N-A | no leaf added |
| YANG custom validators | N-A | no leaf added |
| CLI commands/flags | N-A | a make target is not a `ze` command. The `ze-test` CLI already accepts all three suite names |
| CLI grammar (keyword before value) | N-A | no CLI surface changed |
| Editor autocomplete | N-A | no config leaf added |
| Functional test for new RPC/API | N-A | no RPC or API added. The functional evidence is the three suite runs above |
| Pipe completeness | N-A | no command output produced |
| Env var registration | N-A | no new variable. `ZE_SUITE_TIMEOUT`, `ZE_SUFFIX` and `ZE_TEST_CANONICAL` are unchanged make variables |
| Doctor check for runtime dependencies | N-A | the change adds no file path, socket, service, port or binary at runtime |
| Prometheus counters/metrics | N-A | no observable daemon state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface touched |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a developer-facing make target, not a product feature |
| 2 | Config syntax changed? | No | no config touched |
| 3 | CLI command added/changed? | No | the `ze-test` CLI is unchanged |
| 4 | API/RPC added/changed? | No | none added |
| 5 | Plugin added/changed? | No | none touched |
| 6 | Has a user guide page? | No | the audience is a developer, and `docs/functional-tests.md` is that page |
| 7 | Wire format changed? | No | no protocol code touched |
| 8 | Plugin SDK/protocol changed? | No | none touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | the ldp and rsvpte `.ci` tags keep the verify tier the `all_suites` line already gives them, and that line is unchanged |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (the install-suite run instruction) and the table in `ai/patterns/functional-test.md` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` describes daemon features |
| 12 | Internal architecture changed? | No | no architectural boundary moved |
| 13 | Route metadata keys added/changed? | No | none touched |
| 14 | Prometheus counters added/changed? | No | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | the suite inventory (`all_suites`) is unchanged. Only the per-suite entry points grow |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, checked | `docs/functional-tests.md` carries four anchors naming `internal/le/functional/suites.go`: the `./le functional` suite list (still 24), the non-gated functional targets (unchanged), the isolated-binary block (unchanged), and `ze-functional-isis-test` (unchanged). Every claim still holds. Re-check with `./le doc check verify` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md` shows `bin/ze-test install --all` as the way to run the install suite. That example is now incomplete, and row 10 fixes it |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point is missing, then create it
   - Tests: run `make -n ze-ldp-test`, `make -n ze-rsvpte-test` and
     `make -n ze-install-test` FIRST, and record the three
     `No rule to make target` lines with their exit code 2. This is the RED half
     of AC-7 and it cannot be recovered after the edit
   - Files: `internal/le/functional/suites.go`. Add the three targets, each an exact copy of
     the `ze-functional-isis-test` shape with the suite name swapped: prerequisite
     `$(ZE_TEST_DEPS)`, one recipe line carrying the `@trap '$(ZE_ALT_TRAP)' EXIT;`
     prefix, `$(ZE_ALT_BUILD)`, `$(SUITE_RUN)`, `$(ZE_TEST_RUN)`, the suite name,
     and `--all`. Take the argument list from that suite's own `run_suite` line,
     which is a bare `--all` for all three. Put `ze-ldp-test` and
     `ze-rsvpte-test` beside the other gated targets, and `ze-install-test`
     beside `ze-functional-appliance-test`. The historical plan added no
     `ze-functional-warm` prerequisite because none of the three suites shelled out to `go test`
   - Verify: `make -n` on each of the three names prints the recipe and exits 0.
     AC-7 is met
2. **Phase: Declarations and the reference comment**
   - Tests: `grep '^.PHONY' internal/le/functional/suites.go` names all four new entries
   - Files: `internal/le/functional/suites.go`, the `.PHONY` block. Add `ze-ldp-test`,
     `ze-rsvpte-test`, `ze-install-test`, and `ze-functional-ipsec-test`. The last is a
     defect of the same class in the same lines. That target has existed since
     the ipsec suite landed and was never declared. A file of that name in the
     repository root would silently disable it. Add the three matching lines to
     the quick-reference comment at the head of the file
   - Verify: AC-6, and `make -n ze-ldp-test` still resolves
3. **Phase: Proof**
   - Tests: run `./le functional ldp`, then `./le functional rsvpte`, then
     `./le functional install`. Run them one at a time, each in the foreground, and
     never beside another suite or a verify run (`ai/rules/commands.md`). Record
     the per-suite pass count and the wall-clock duration of each
   - Files: none
   - Verify: AC-1, AC-2, AC-3. A run that reports zero tests fails its AC, even
     though the target resolved
4. **Phase: Aggregate and docs**
   - Tests: `./le functional` for AC-5, or read it from this session's
     verify log if that run already happened. Then `./le doc check verify` for the doc
     edits
   - Files: `ai/patterns/functional-test.md`, `docs/functional-tests.md`
   - Verify: AC-5, and the two documentation rows answered Yes above
5. **Phase: Hand off**
   - Commit through `internal/le/commit/prepare.go`, set Status to `verification`,
     close the deferral shard row, and stop. Handoff is `verify`: a later Opus 5
     session reviews that commit and closes the spec

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each of AC-1 to AC-7 has a recorded command and its output, never a claim |
| Feature completeness | Every name in `all_suites` resolves to a target. Check all 24, not only the three |
| Correctness | Each new recipe is identical to its `run_suite` line in command and arguments. A silent extra flag makes the standalone and the aggregate disagree |
| Naming | The target names are what the failure block prints, `ze-<suite>-test`, with the suite spelled as `all_suites` spells it |
| Data flow | No second list of suite names is introduced anywhere |
| Rule: `ai/rules/commands.md` | The evidence comes from `make`, never from a direct `bin/ze-test` call |
| Rule: `ai/rules/simplicity.md` | The change adds three targets and four `.PHONY` names. It adds no gate, no script and no variable |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Three targets exist and resolve | `make -n ze-ldp-test`, `make -n ze-rsvpte-test`, `make -n ze-install-test` each exit 0 |
| Every gated suite name resolves | `make -n ze-<suite>-test` for each of the 24 names in the `all_suites` line, all exit 0 |
| Each target runs its suite | the three recorded runs with their pass counts: 3, 5, and the install suite with 0 failures |
| The aggregate is unchanged at 24 suites | the `[24/24]` progress line and the `PASS  all 24 suites` line from `./le functional` |
| `.PHONY` names every target in the file | `grep '^.PHONY' internal/le/functional/suites.go`, compared against the target definitions |
| Docs match the file | `./le doc check verify` |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Input validation | N-A. The change adds no input path. The recipes take no operator-supplied value beyond the existing `ZE_SUITE_TIMEOUT` and `ZE_SUFFIX` make variables |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Make refuses to parse the file | a tab-versus-space error in a new recipe line. Fix in Phase 1 |
| A new target runs zero tests | the suite name is misspelled against the runner's own name. Compare it with the `run_suite` line |
| A suite is red | attribute it by test name. Red inside the aggregate too means it predates this change: report it under `ai/rules/completion.md`. Red only standalone means the target is wrong: fix the target |
| `./le functional install` exits 124 | R-1. Record the duration and report it. Do not raise the cap |
| `./le doc check verify` fails | a stale source anchor. Fix the doc claim, never the anchor |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The failure hint and the target list are two spellings of one fact, and nothing
  binds them. That is what let three names drift apart from 33.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add three explicit targets that copy the `ze-functional-isis-test` shape | a pattern rule `ze-%-test` covering every present and future suite | a pattern rule matches any name, so a typo such as `ze-lpd-test` would resolve and hand a bad suite name to the runner instead of failing at make. It would also create targets for suites that must not run that way. The explicit copy keeps make's own "no rule" error as the guard |
| Ship the three targets and nothing else | also add an inventory check that refuses an `all_suites` name with no matching target | the deferral row commissions the missing targets. A new gate is a separate deliverable with its own home, population and test, and `internal/le/rfc/rfc.go` already owns the nearest check over that same line: `_ALL_SUITES_RE` reads it, and the `undispatched` refusal rejects a name with no `run_suite` line. Extending that reader to target existence is separable work. It is recorded under Known Limitations and needs its own spec |
| The historical design used `$(ZE_TEST_DEPS)` with no `ze-functional-warm` prerequisite | copy `ze-functional-ospf-test`, which carried that prerequisite | the warm target existed for suites whose `.ci` files ran `exec=go test`; the native runner now builds lazily |
| Add `ze-functional-ipsec-test` to `.PHONY` in the same change | leave it, and spec it separately | it is one word on a line this change already edits, and it is the same defect class. Fixing it here costs nothing and leaves the file consistent (`ai/rules/completion.md`: related code is in scope) |

## Known Limitations

- Nothing stops the next suite from repeating this. A check that reads the
  `all_suites` line and refuses a name with no `ze-<name>-test` target is the
  durable half, and it is deliberately outside this spec: a gate carries its own
  home, population and test. It needs its own spec, and this one does not create
  it.
- The quick-reference comment at the head of `internal/le/functional/suites.go` is not
  exhaustive today: it omits `ze-functional-isis-test`, `ze-functional-ipsec-test`, `ze-functional-ospfv3-test`
  and `ze-functional-runner-test`. This change adds the three lines it creates and leaves
  those four gaps alone.
- The evidence is a run on one host. A suite that depends on host tooling, such
  as the QEMU tests under `test/install/`, can behave differently elsewhere. The
  standalone target and the aggregate share that property exactly, which is the
  guarantee this spec makes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-7 each proven by a recorded command and its output
- [ ] Wiring Test table complete: every row a concrete command, none deferred
- [ ] `./le verify worktree` passes, or a scoped attribution is recorded per `ai/rules/git-safety.md`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste the three `No rule to make target` lines from Phase 1)
- [ ] Tests PASS (paste the three suite runs and their pass counts)
- [ ] Boundary tests for all numeric inputs, or N-A with a reason
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Quality Gates
- [ ] `./le functional` reaches `[24/24]`
- [ ] `./le doc check verify`
- [ ] `./le verify current mode full`

### Closure
- [ ] Status set to `verification`, the work committed, and the session stopped. Handoff is `verify`: a later Opus 5 session reviews that commit and closes the spec
- [ ] Deferral shard row closed
