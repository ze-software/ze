# Spec: verify-scope-6-wiring-docs-attribution

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-verify-scope-1-shared-checkout-freshness.md` |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make `./le doc wiring` report WHICH FILES each of its failures is about, so
the commit helper can tell a red that belongs to this session from a red that
belongs to another one.

Sub-spec 1 taught `commit_helper.py` to attribute a structural red: a gate is
dropped only when every failure group named files and every one of those files
lies outside the commit. Measured on the live ledger, that mechanism reaches
almost nothing, and one gate is why:

| Population | Count |
|------------|-------|
| Open `structural gates (red)` debt rows | 102 |
| Of those, rows naming `./le doc wiring` | **71** |
| Rows attribution can act on today | about 3 |

`classifyWiringDocs` (`internal/le/verify/engine/run.go`) builds its groups from
prose, with `runningRE` matching `^Running ([A-Za-z0-9_-]+)\.{3}` and `failedRE`
matching `([A-Za-z0-9_-]+) failed`, so `Related` carries a CHECK NAME. Sub-spec
1's `related_repo_path` then finds no file, the group counts as unattributable,
and the gate is charged to whoever happened to be committing. That is the
fail-closed half working as designed, and it is why 71 rows exist.

**The gate is a router, and that is the whole difficulty.** `main`
(`internal/le/doc/wiring/wiring.go`) runs five checks unconditionally
(`check_ci_sleep_ratchet`, `check_ci_sleep_justification`,
`check_known_failure_load_excuses`, `check_ci_log_subsystem_keys`,
`check_design_refs`), then `check_wiring`, then delegates the remaining selected
targets through `run_make_target`. Some of those name a file. The ci-sleep
ratchet reports a COUNT against a baseline. A delegated target relays another
program's stdout. So no single regex over the log can attribute them all, and a
partial capture is worse than none: `classifyStage` replaces its groups with
`genericGroup` only when the slice is EMPTY, so capturing four sub-checks and
missing the ratchet would return a non-empty slice and silently DROP the
ratchet's failure.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `ai/rules/precommit-verify.md` - how a red is judged in a shared checkout, and what attribution now does with each structural red
  → Constraint: a gate drops only when every group named files and every file is foreign
- [ ] `ai/rules/evidence.md` - a guard must fail closed
  → Constraint: a zero value must never be a valid-looking answer

**Key insights:**
- The structured protocol already exists and is proven. `run_suite` (`internal/le/functional/suites.go`) emits `VERIFY FAILURE GROUP: {json}` for a budget expiry, and `classifyFunctional` (`internal/le/verify/engine/run.go`) parses it into a `failureGroup`. The producer declares its own group instead of the consumer guessing from prose.
- That parser is PRIVATE to `classifyFunctional` today. `ai/rules/no-layering.md`: extract it, do not write a second one.
- `check_wiring` failing returns 1 immediately, so the delegated targets never run and `gate_rc` from the five earlier checks is discarded. Any group design has to survive that early return.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/doc/wiring/wiring.go` - `main` and its dispatch; `check_wiring`, `check_design_refs`, `check_ci_sleep_ratchet`, `check_ci_sleep_justification`, `check_known_failure_load_excuses`, `check_ci_log_subsystem_keys`, `run_make_target`, `selected_targets`, `changed_files`
- [ ] `internal/le/verify/engine/run.go` - `classifyStage` and its empty-slice fallback to `genericGroup`; `classifyWiringDocs` and its two regexes; `classifyFunctional` and the `VERIFY FAILURE GROUP:` parser
- [ ] `internal/le/commit/prepare.go` - `structural_gate_reds`, `GateReds`, `group_related_paths`, `related_repo_path`, `PATH_BEARING_GROUP_KINDS`
- [ ] `plan/verification-debt/*.md` - the rows and the gate names they carry

**Behavior to preserve:**
- `./le doc wiring` stays a STRUCTURAL gate: `--unverified` cannot wave it through, and a red inside the session's own paths still refuses the commit.
- The check's exit code and what it refuses are unchanged. This spec changes what it REPORTS, never what it judges.
- `changed_files` keeps reading the whole shared tree. Narrowing it is a different question and is not in scope here.

**Behavior to change:**
- Each failing sub-check declares a failure group naming the files it is about.
- A sub-check with no file to name declares a group that says so, and is charged.
- The declared-group protocol becomes available to EVERY stage, and `classifyWiringDocs` prefers declared groups over its prose regexes.

**→ Decision (2026-08-19, main thread, from phase 1's measurement): the PROTOCOL
goes to the stage level; the ADOPTION stays scoped to this gate.**

Phase 1 asked whether the payoff was there and found it smaller than the header
table claims. Of 103 open `structural gates (red)` rows, 72 name this gate, and
only **33** name gates that would ALL be attributable after this spec. Thirty-nine
also name a gate that stays unattributable, led by `./le repository generated-check`
(15), `./le doc check links` (13) and `./le test-weakened check` (7). Nineteen name
no gate at all.

The reason is one mechanism rather than nine separate gaps, and it is verified:
`classifyStage` (`internal/le/verify/engine/run.go`) dispatches to a classifier for
SIX stage names and gives every other stage `genericGroup`, whose `Kind` is
`stage`. `PATH_BEARING_GROUP_KINDS` (`internal/le/commit/prepare.go`) does not
contain `stage`, and `group_related_paths` returns an empty list for every kind
outside it before any path lookup. So 24 of the 30 stages cannot be attributed
at all, whatever they print.

**→ Correction (2026-08-19).** This spec was written when that set was the
inverse, `NON_PATH_GROUP_KINDS`, listing the kinds that do NOT bear paths. The
final verification pass over sub-spec 1 found the polarity wrong: `kind` arrives
as producer JSON, so the namespace is OPEN, and a kind absent from a denylist --
`unparsed`, which THIS spec added -- fell through to a filesystem-existence
check and could be mistaken for attribution evidence. The set is now an
allowlist, so an unknown kind names no path by default. The conclusion above is
unchanged; only the direction of the test is.

So phase 2 extracts the parser and calls it from `classifyStage`, for every
stage, rather than from `classifyWiringDocs` alone. That is nearly the same edit
and it turns each remaining gate into a small, separable adoption instead of a
second mechanism. This spec still ADOPTS it in one gate only: widening the
mechanism is cheap, widening the adoption is nine more producers to get right,
and each deserves its own evidence.

**→ Decision: phase 3 MUST capture `check_design_refs`'s child output.** Phase 1
found it holds only `proc.returncode`: it runs its child without
`capture_output`, so the paths it is complaining about live one process down.
It is unconditional and tree-wide, which makes it among the likeliest reds on a
shared checkout, so leaving it unattributable would charge the gate on most runs
and collapse even the 33.

**→ Constraint: a group line is emitted AT THE POINT OF FAILURE.** The script
never flushes and its recipe passes no `-u`, so a child's output can precede the
parent's prints in the stage log. A design that depends on a group line's
position relative to prose, or that dumps groups at the end of the run, would be
wrong on exactly the runs that matter.

**→ AC-8 is resolved and needs no code.** Phase 1 verified that `check_wiring`'s
early return substitutes 1 for a `gate_rc` that is already 0 or 1, so the exit
code is identical, and each of the five earlier checks has already printed its
own `FAILED:` block. Skipping the delegated targets is deliberate fail-fast,
matching `run_make_target`'s own `GateFailure`.

## Data Flow (MANDATORY)

### Entry Point
- `./le doc wiring`, as a stage of either verify mode.

### Transformation Path
1. A sub-check fails and prints a `VERIFY FAILURE GROUP:` line naming its files.
2. `classifyStage` routes the stage to `classifyWiringDocs`.
3. `classifyWiringDocs` reads the declared groups; it falls back to its regexes only when the check declared none at all.
4. `writeVerifyStatus` records them in the failure index.
5. `structural_gate_reds` attributes each group's paths against the commit's file list, and charges the gate unless every group named files and every file is foreign.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Sub-check ↔ verify runner | one `VERIFY FAILURE GROUP:` line per failure | No |
| Failure index ↔ commit helper | `tmp/ze-verify-failures.json` | No |

### Integration Points
- The `VERIFY FAILURE GROUP:` parser, extracted from `classifyFunctional` and shared.
- `related_repo_path` and `PATH_BEARING_GROUP_KINDS` (`internal/le/commit/prepare.go`), which already decide what counts as a path.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Phase 1 Findings -- Enumerate before changing (2026-08-19)

### A-1: every failure path in `main`, and what it can name

`main` (`internal/le/doc/wiring/wiring.go`) has eight paths to a non-zero exit.
"Names files" means the path holds a repository path in a variable at the moment
it decides to fail, not that a path appears somewhere in the log.

| # | Failure path | Prints | Names files at decision time | Population it judges |
|---|--------------|--------|------------------------------|----------------------|
| 1 | `check_ci_sleep_ratchet` | `ci-sleep ratchet FAILED:` and a count against the delta baseline | **No.** `count` is a sum over `real_ci_files`; no per-file list is kept | whole tree, triggered by any changed `.ci` |
| 2 | `check_ci_sleep_justification` | `ci-sleep justification FAILED:` then `rel:line: text` | **Yes.** `violations` carries `rel` per row | changed `.ci` only |
| 3 | `check_known_failure_load_excuses` | `known-failure load excuse FAILED:` then `rel:line: text` | **Yes.** `violations` carries `rel` per row | changed shards only |
| 4 | `check_ci_log_subsystem_keys` | `ci log-subsystem key FAILED:` then `rel:line: ze.log.<key>` | **Yes.** `violations` is `(rel, lineno, subsystem, text)` | whole tree, triggered by any changed `.ci` |
| 5 | `check_design_refs` | nothing of its own | **No, as written.** It runs `check_doc_links.py --design-only` with no `capture_output`, so it holds only `proc.returncode`. The CHILD's own `check_design_refs` (`internal/le/doc/check/links.go`) prints `<go>:<line>: broken Design reference: <target>` straight to the inherited stdout | whole tree, unconditional |
| 6 | `check_wiring`, then `main`'s early `return 1` | `Wiring check FAILED:` then `sym.path:sym.line: exported ...` | **Yes.** `issues` carries `sym.path` | files in `changed`, which is the whole shared tree |
| 7 | `run_make_target` | the child's stdout and stderr, then `GateFailure("<target> failed")` on stderr | **No.** It holds the TARGET name. Any path in the relayed text was produced by another program | one delegated target |
| 8 | `check_plugin_imports` | `GateFailure("plugin import check failed")` | **No.** Unreachable from the gate: `main` returns 0 on its `--check-plugin-imports` branch, and the `./le doc wiring` recipe in `internal/le/doc/check/actions.go` never passes that flag | n-a |

**Verdict: A-1 CONFIRMED, with one path that must change to keep the payoff.**
Every path either names its files or structurally cannot, so no path names files
"only in prose". Paths 1, 7 and 8 name none and are the charged case AC-4 and
AC-5 already describe. Path 5 is the one that decides the payoff: it is
UNCONDITIONAL and TREE-WIDE, so in a shared checkout it is among the likeliest
reds, and its files exist one process down, in a child this wrapper does not
capture. Declaring it unattributable would charge the gate on most shared-tree
runs and waste the mechanism. Phase 3 must switch it to `capture_output=True`,
re-print what it captures, and parse the `<path>:<line>:` prefix.

**Buffering hazard for phases 3 and 4.** `verify_wiring_docs.py` never flushes
and its recipe does not pass `-u`, so when the stage's stdout is a pipe the
parent's `print` calls are block-buffered while the `check_design_refs` child
writes to the same descriptor directly. Child output can therefore appear BEFORE
parent output that was produced earlier. A declared-group line must not depend on
its position relative to a sub-check's prose, and capturing path 5 removes the
interleave as a side effect.

### A-2: the payoff, re-counted from the ledger

Counted over `plan/verification-debt/*.md`, rows whose `Gate owed` contains
`structural gates (red)` and whose Status is `open`. A gate is attributable
TODAY when its stage has a classifier that records a repo path: `classifyLint`,
`classifyGoTest` and `classifyVet` (`internal/le/verify/engine/run.go`). Every other
stage falls through `classifyStage` to `genericGroup` with `Kind: "stage"`, and
`PATH_BEARING_GROUP_KINDS` (`internal/le/commit/prepare.go`) does not admit that
kind, so it is refused before any path lookup.

| Population | Count |
|------------|-------|
| Open `structural gates (red)` rows | 103 |
| Rows naming the wiring gate | 72 |
| **Rows naming ONLY gates attributable after this spec** | **33** |
| Rows also naming a gate that stays unattributable | 39 |
| Rows naming no gate at all in their Reason prose | 19 |

The 39 are blocked by: `./le repository generated-check` (15), `./le doc check links`
(13), `./le test-weakened check` (7), `./le staticcheck-feature-matrix check` (4),
`ze-rules-*` (3), `./le doc check verify` (2), `./le spec citation anchors` (1),
`ze-unit-hook-test` (1), `./le docs-to-code index-check` (1).

**Verdict: A-2 CONFIRMED but SMALLER than the spec assumed. The payoff is 33
rows, not 71.** 33 is also an upper bound: it counts rows whose other named
gates COULD be attributed, not rows whose files are proven foreign.

**The blocker is not another gate, it is the same mechanism one level up.** Nine
distinct stages are unattributable for one reason: no classifier, so
`genericGroup` with `Kind: "stage"`. The declared-group protocol this spec builds
is not wiring-specific, and applied at the STAGE level (any make target may emit
`VERIFY FAILURE GROUP:`, and `classifyStage` prefers declared groups over
`genericGroup`) it reaches all 103 rows rather than 33.

### AC-8: the early return is not a defect

`main` computes `gate_rc` from the five unconditional checks, and `check_wiring`
failing does `return 1` without folding it in. The value discarded is 0 or 1 and
the substitute is 1, so the exit code is identical. The five checks each PRINT
their own `... FAILED:` block before returning, and `check_design_refs`'s child
writes to the inherited stdout, so every verdict is already in the stage log when
the early return happens. Nothing is masked.

The real information loss on that path is that the delegated targets never run,
and that is deliberate fail-fast: `run_make_target` raising `GateFailure` skips
the remaining targets and `gate_rc` in exactly the same way, and `main`'s
no-targets branch returns `gate_rc` faithfully. **AC-8 is satisfied by this
record: the five verdicts CAN be reported and already are.** Phase 3 must keep
that property when it adds group lines, which means the group line for each of
the five is emitted at the point of failure, never accumulated for an end-of-run
dump the early return would skip.

### Recommendation

Run phases 2 to 5. The mechanism is right, and 33 of 103 open rows is real. Two
amendments the main thread rules on before phase 3 starts:

| # | Amendment | Why |
|---|-----------|-----|
| 1 | `check_design_refs` captures its child's stdout and parses `<path>:<line>:` | Otherwise the likeliest tree-wide red declares no file, the gate is charged, and most of the 33 do not clear either |
| 2 | Declare groups at the STAGE level, not only inside this gate | The same `genericGroup` / `Kind: "stage"` refusal blocks 39 rows across nine other stages. Deciding this now avoids building the protocol twice |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every sub-check that can fail knows which files it is complaining about, or knows that it does not | `check_wiring` and `check_design_refs` already print `path:line`; the ci-sleep ratchet prints a count | A sub-check exists that names files only in prose, and the group would have to re-parse them | Enumerate every failure path in `main` and say, per sub-check, what it can name | confirmed -- 8 paths: 4 name files, 3 structurally name none, 1 (`check_design_refs`) needs `capture_output` to reach the child's paths |
| A-2 | Attributing this gate materially reduces the debt ledger | 71 of 102 structural rows name it | The rows name it alongside another unattributable gate, so dropping this one changes nothing | Re-read the 71 rows: count those naming ONLY gates that would become attributable | confirmed, reduced -- 33 of 103 open rows, not 71 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A partial capture silently drops a failure | a sub-check fails and no group mentions it | All-or-nothing: the check declares a group for EVERY failure it reports, or `classifyWiringDocs` ignores declared groups entirely and keeps its prose path |
| R-2 | A malformed group line vanishes | a failure disappears from the index | The parser currently does `continue` on a JSON error. It must instead produce a group saying the line could not be parsed |
| R-3 | Attribution starts dropping reds it should charge | a foreign-looking red that was really this session's | A group naming no file keeps `Kind` outside the path-bearing set, so sub-spec 1 charges it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A structural red is charged to the wrong session, or worse, dropped. No runtime behavior changes |
| How is it reverted? | Single commit revert; `classifyWiringDocs` keeps its prose path as the fallback |
| Who else touches this path? | Every session's commit goes through `structural_gate_reds` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a failing sub-check | → | its `VERIFY FAILURE GROUP:` line | `TestEveryWiringSubcheckDeclaresItsFiles` |
| a stage log carrying declared groups | → | `classifyWiringDocs` preferring them | `TestClassifyWiringDocsPrefersDeclaredGroups` |
| a stage log carrying none | → | the prose fallback | `TestClassifyWiringDocsFallsBackToProse` |
| a malformed group line | → | the unparseable-group branch | `TestMalformedGroupLineIsReported` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `check_wiring` fails on a symbol in `internal/component/ssh` | The group's `related` names that file, and `structural_gate_reds` attributes it |
| AC-2 | A commit carries only files outside every declared group | The gate is not charged a debt row |
| AC-3 | A commit carries a file inside a declared group | The gate refuses, as today |
| AC-4 | The ci-sleep ratchet fails, reporting a count | A group is declared, it names no file, and the gate is CHARGED rather than dropped |
| AC-5 | A delegated target fails through `run_make_target` | A group is declared naming that target, with a kind outside the path-bearing set, and the gate is charged |
| AC-6 | Any sub-check fails and declares no group | `classifyWiringDocs` ignores every declared group and uses its prose path, so no failure is lost |
| AC-7 | A `VERIFY FAILURE GROUP:` line is malformed | A group is produced saying so; the failure does not vanish |
| AC-8 | `check_wiring` fails, so `main` returns early | The five earlier checks' verdicts are still reported, or the spec records why they cannot be |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryWiringSubcheckDeclaresItsFiles` | `internal/le/` | AC-1, AC-4, AC-5: every failure path declares a group | green |
| `TestClassifyWiringDocsPrefersDeclaredGroups` | `internal/le/verify/engine/verifyengine_test.go` | AC-1 | green |
| `TestClassifyWiringDocsFallsBackToProse` | `internal/le/verify/engine/verifyengine_test.go` | AC-6: partial capture cannot drop a failure | green |
| `TestMalformedGroupLineIsReported` | `internal/le/verify/engine/verifyengine_test.go` | AC-7, R-2 | green |
| `TestAStageWithNoClassifierCanDeclareItsOwnGroups` | `internal/le/verify/engine/verifyengine_test.go` | the protocol reaches every stage, not only this gate | green |
| `TestTheWiringGateSpeaksTheProtocolThisRunnerReads` | `internal/le/verify/engine/verifyengine_test.go` | the Python producer and the Go reader agree on prefix and keys | green |
| `TestAPathologicalPathCannotForgeAGroup` | `internal/le/verify/engine/verifyengine_test.go` | Security Review: a path cannot forge a second group | green |
| `DeclaredGroupProtocolTest` | `internal/le/` | escaping, the 50-path bound, the count, and a failing run end to end | green |
| `test_wiring_red_outside_the_commit_is_not_charged` | `internal/le/` | AC-2 | green |
| `test_wiring_red_inside_the_commit_still_refuses` | `internal/le/` | AC-3 | green |
| `test_unattributable_wiring_red_is_charged` | `internal/le/` | AC-4 | green |
| `TestSplitLinesReportsATruncatedRead` | `internal/le/verify/engine/verifyengine_test.go` | R-1 from the reader's side: a truncated stage log makes the declared count disagree, so the stage falls back rather than trusting a partial set | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| declared groups per run | 0-N | N | N/A | N/A |
| files named by one group | 0-N | N | N/A | N/A |

<!-- Zero declared groups is valid and meaningful: it selects the prose
     fallback (AC-6). Zero files in a group is also valid and is the
     unattributable case (AC-4), which must be charged rather than dropped. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-wiring-attribution` | `test/runner/verify-scope-wiring-attribution.ci` | A developer commits while another session's file reddens the wiring gate, and is not charged a debt row for it | green |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes | |

## Files to Modify
- `internal/le/doc/wiring/wiring.go` - each failure path declares its group
- `internal/le/verify/engine/run.go` - extract the shared `VERIFY FAILURE GROUP:` parser; `classifyWiringDocs` prefers declared groups; a malformed line becomes a group
- `internal/le/verify/engine/verifyengine_test.go`, `internal/le/`, `internal/le/`
- `ai/rules/points/precommit-verify/**` and the regenerated `ai/rules/precommit-verify.md`, `TRIGGERS.md`, `CORE.md`, `INDEX.md` - the reach of attribution changes, so its text does too
- `docs/architecture/testing/verify-freshness-scope.md` - the declared-group protocol
- `docs/functional-tests.md` - the new `test/runner/` member

## Files to Create
- `test/runner/verify-scope-wiring-attribution.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Commit and verify tooling only |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | No `ze` subcommand changes |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | No `ze.*` leaf added |
| Doctor check for runtime dependencies | N-A | No new runtime path, socket, port, module, or binary |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC surface |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the new `test/runner/` member |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`: the declared-group protocol |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `verify_wiring_docs.py` and `verify_run.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/precommit-verify.md` describes what attribution reaches |

## Implementation Steps

1. **Phase: Enumerate before changing (MANDATORY FIRST)** -- A-1 decides the shape
   - List every failure path in `main` (`internal/le/doc/wiring/wiring.go`) and say, per sub-check, whether it can name files, and which
   - Answer A-2 by re-reading the 71 rows: how many name ONLY gates this makes attributable
   - **If most of the 71 also name an unattributable gate, say so.** The mechanism would still be right and the payoff would not be there
2. **Phase: Share the parser** -- extract the `VERIFY FAILURE GROUP:` reader from `classifyFunctional` so one implementation serves both stages, and make a malformed line a group rather than a silent skip
   - Tests: `TestMalformedGroupLineIsReported`
3. **Phase: Declare the groups** -- each failure path emits one, naming files where it has them
   - Tests: `TestEveryWiringSubcheckDeclaresItsFiles`
4. **Phase: Prefer them, all or nothing** -- `classifyWiringDocs` uses declared groups only when the run declared one for every failure it reported
   - Tests: `TestClassifyWiringDocsPrefersDeclaredGroups`, `TestClassifyWiringDocsFallsBackToProse`
5. **Phase: Attribution end to end** -- the commit helper drops a foreign wiring red and charges an unattributable one
   - Tests: the three `commit_helper_test.py` cases, `verify-scope-wiring-attribution.ci`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every failure path in `main` declares a group, including the early return |
| Correctness | A partial capture CANNOT drop a failure. Drive a run where one sub-check declares and another does not |
| Naming | One parser for the declared-group protocol, used by both stages |
| Data flow | The producer declares its own files; the consumer never re-parses prose to find them |
| Rule: `ai/rules/evidence.md` | An unattributable group is charged, never dropped. The empty answer is not the valid-looking one |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One shared parser | `grep -c 'VERIFY FAILURE GROUP:' internal/le/verify/engine/run.go` names one reader |
| Every failure declares a group | `TestEveryWiringSubcheckDeclaresItsFiles` |
| The ledger's largest class becomes attributable | Re-count the 71 rows against the new behavior |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The group line is JSON built from file paths that can hold any byte a filename can. A path with a quote or a newline must not forge a second group, and a malformed line must be reported rather than skipped |

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

- The producer knows its own files; the consumer was guessing from prose. That is the whole defect, and the repository already solved it once for the functional stage. This spec generalises that solution rather than inventing a second one.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The sub-check declares its own group | Widen `classifyWiringDocs`'s regexes to capture file paths | A regex over prose cannot see a delegated target's stdout, and a partial capture DROPS the failures it misses, because `classifyStage` falls back to `genericGroup` only on an empty slice |
| Extract the existing parser rather than write a second | Copy the parser into `classifyWiringDocs` | `ai/rules/no-layering.md`. Two parsers of one protocol drift, and one of them is always the wrong one |
| All-or-nothing adoption | Let declared and prose groups mix | Mixing is exactly how a failure disappears: one declared group makes the slice non-empty and suppresses the fallback for everything else |
| An unattributable group is CHARGED | Drop it, or omit the group | Sub-spec 1 settled this: guessing attribution lets a real red go uncharged, which rung 3 of `ai/rules/rule-precedence.md` forbids |

## Known Limitations
- `changed_files` still reads the whole shared tree, so the gate still JUDGES another session's files. This spec makes the resulting red attributable; it does not stop it happening.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
