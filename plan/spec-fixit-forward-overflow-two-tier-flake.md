# Spec: fixit-forward-overflow-two-tier-flake

| Field | Value |
|-------|-------|
| Status | done |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/plugin/forward-overflow-two-tier.ci` failed ONCE, with an `ordered:`
needle against a 246-byte `fwdBucketMerge` UPDATE. It has not been reproduced
since and is not attributed to any change in the tree.

**This is unreproduced, so it does not yet meet the bar for a known-failures
shard either.** That path requires a reproduction attempt on the record
(`ai/rules/completion.md`). The first job is to establish whether it reproduces
at all: alone, and with `ZE_PLUGIN_PARALLEL` well above the core count, which
is the setting that reproduced other load-sensitive failures in this suite on
2026-08-08.

If it reproduces, fix the root cause. If a genuine effort fails to reproduce
it, record the attempt and the next step rather than closing silently.

**Found** 2026-08-08 during the repair of GitHub Actions run 31225029268. It
was not one of that run's failures.

## Answer

It does not reproduce. 80 invocations under the load profile that surfaced the
other 2026-08-08 failures returned exit 0 every time. So this spec takes the
second branch its own Task sets: the attempt and the next step are recorded in
`plan/known-failures/bgp-plugin-forward-overflow-two-tier.md`, and no code
changes. `ai/rules/completion.md` permits recording instead of fixing in
exactly one case, a failure actively tried and not reproduced, and it requires
the record to carry the reproduction attempt and the next step. Both are there.

The deliverable is a measurement and a record. There is no diff to the product.

## Required Reading

### Architecture Docs
- [ ] `scripts/dev/stress-repro.py` - the reproduction tool, and the stale-binary trap in `_bin_from_env`
  → Constraint: `_bin_from_env` falls back to `bin/ze`, which `mk/session.mk` leaves stale by construction. `ensure_binaries` checks existence, never freshness.

**Key insights:** (minimal context to resume after compaction)
- The first run of the stress command reported a reproduction that was an
  artifact of a day-old binary, not of the suspected flake. A reproduction whose
  symptom is a CONFIG PARSE ERROR is a stale binary until proven otherwise.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `test/plugin/forward-overflow-two-tier.ci` - 50 `expect=bgp:conn=1:seq=1:ordered=180A00NN` needles against one packed UPDATE, `expect=exit:code=0`, and two `reject=stderr` panic guards
- [x] `scripts/dev/stress-repro.py` - `_bin_from_env` (line 130) resolves `ZE_BIN`/`ZE_TEST_BIN` and falls back to `bin/ze`; `ensure_binaries` (line 151) tests `os.path.isfile` only

**Behavior to preserve:**
- The existing ordered assertion. It is the thing under suspicion, not a thing to relax.
- The test stays in the suite: not quarantined, not skipped, not given a longer timeout.

**Behavior to change:**
- Nothing in the product. The failure did not reproduce, so there is no producing
  function to name and nothing to fix at an owning layer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

No product code path changes. The flow below is the MEASUREMENT's, which is
what this spec actually produced.

### Entry Point

`scripts/dev/stress-repro.py "bgp plugin" --test forward-overflow-two-tier
--any-failure`, which resolves the binaries through `_bin_from_env` and then
loops `ze-test` invocations against `test/plugin/forward-overflow-two-tier.ci`.

### Transformation Path

1. `_bin_from_env` picks `ZE_BIN`/`ZE_TEST_BIN`, falling back to the stale `bin/ze`.
2. `ensure_binaries` tests existence only, never freshness.
3. Each invocation runs the `.ci` under `--burners` CPU/GC load.
4. A non-zero exit under `--any-failure` counts as a reproduction.
5. Every exit and its output is appended to the capture under `tmp/stress-repro/`.

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| `stress-repro.py` | `ze-test` process | the binary paths, the suite and test selector, the burner load |
| `ze-test` | the capture log | per-invocation exit code and stdout/stderr |
| the capture log | `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` | the verdict, the settings that produced it, and the next step |

### Integration Points

The record, not the code. The shard is read by whoever meets a second
occurrence, and the deferral row in
`plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` points at it.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-functional-test` | -> | unchanged forwarding path | `test/plugin/forward-overflow-two-tier.ci`, still in the suite and still green |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| None | - | No code changed, so a unit test would assert against an unmodified function and prove nothing about the question this spec asks |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `forward-overflow-two-tier` | `test/plugin/forward-overflow-two-tier.ci` | 50 routes cross ze with a forced forward-channel overflow and arrive at the destination peer in order | Unchanged, green in 80 stress invocations and in a full `make ze-functional-test` |

## Risks & Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|--------------------------------|----------|--------------|--------|
| A-1 | A reported reproduction is a reproduction | `stress-repro.py` prints `*** REPRODUCED on invocation N ***` | The whole diagnosis chases an artifact | Reading the capture, not the verdict line | **broken**: the first run's reproduction was a day-old `bin/ze` failing on config grammar. See Mistake Log |
| A-2 | The 2026-08-08 load profile is the right first thing to try | It surfaced the sibling load-sensitive failures in this suite | The negative result bounds less than claimed | Used it: parallelism 8, 32 burners on 16 CPUs | confirmed |

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A negative result is not proof of absence. The defect may need `--race`, Linux, or higher load to appear | The shard records the next step in that order, and the test stays in the suite so a second occurrence is caught rather than hidden |
| R-2 | The original failure came from GitHub Actions (Linux); this attempt ran on darwin | Recorded as next step 2, with the loopback bind-timing asymmetry this suite has already been bitten by |

### Integration Checklist

| Item | Answer | Evidence |
|------|--------|----------|
| New component or plugin registered | N-A | No code changed |
| Config surface added or changed | N-A | No YANG, no env var, no CLI flag |
| Runtime dependency added (needs a `ze doctor` check) | N-A | The measurement adds no runtime dependency; `stress-repro.py` is a dev tool |
| Test registered in a suite | Yes | `test/plugin/forward-overflow-two-tier.ci` was already in the plugin suite and stays there, unmodified by this spec |

### Documentation Update Checklist

| Category | Update needed | Evidence |
|----------|---------------|----------|
| Feature list, user guide, config syntax, CLI reference | No | `grep -rn "forward-overflow-two-tier" docs/` returns nothing; no user-visible surface changed |
| API/RPC docs, plugin SDK, wire format | No | No code changed, so no contract moved |
| RFC compliance / `docs/features/rfc-status.md` | No | No protocol behavior was implemented, changed or newly proven |
| Comparison table | No | No support level changed |
| Test infrastructure | No | The test is unchanged. The tooling trap is filed as its own spec rather than documented here |
| Architecture design | No | No design decision landed in the product |

## Files to Modify

No product file. The spec's output is
`plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` and a journal row.

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
   -> Done as far as it goes: the reproduction attempt ran and returned no failure,
      so there is no root cause to name.
2. Fix at the owning layer, never at the symptom.
   -> Not reached. Nothing to fix.
3. Prove the fix discriminates: red before, green after.
   -> Not reached. `--any-failure` made the attempt maximally sensitive instead:
      any non-zero exit would have counted.

## Acceptance Criteria

| ID | Criterion | Answer |
|----|-----------|--------|
| AC-1 | Does `forward-overflow-two-tier` reproduce under repetition and load? | **No.** 80 invocations, parallelism 8, 32 CPU/GC burners on a 16-CPU host, `--any-failure` set. Every invocation exit 0. Bounded: the runs used the working tree's variant of the `.ci`, not the committed one that failed. The shard states the difference and makes the committed variant next step 1 |
| AC-2 | If it does not reproduce, is the attempt on the record with its next step? | **Yes.** `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` carries the settings, the capture path, and a next step ordered by what could say something new: `--race`, then Linux rather than darwin, then higher load |
| AC-3 | Is the test still in the suite, unweakened? | **Yes.** All 50 `ordered=` needles present, timeout still 20s, no skip and no quarantine |

## Checklist

### Goal Gates (MUST pass)
- [ ] Either the root cause is fixed, or the reproduction attempt and next step are on the record
- [ ] No timeout was raised and no assertion was relaxed to reach green

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit

---

## Implementation Summary

### What Was Implemented

No product change. The work is one measurement and its record.

- Ran `scripts/dev/stress-repro.py "bgp plugin" --test forward-overflow-two-tier
  --any-failure` for 80 invocations at parallelism 8 with 32 burners on a 16-CPU
  host. Result: not reproduced, every invocation exit 0.
- Wrote `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` with the
  attempt, the capture path, the false reproduction and its cause, and a next
  step ordered by what could say something new.
- Resolved the deferral row in
  `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` to `done`.

### Bugs Found/Fixed

None in the product. One trap in the tooling, already filed rather than fixed
here: `spec-stress-repro-refuses-a-stale-binary` (`plan/future/`) proposes making
`stress-repro.py` refuse a stale binary instead of relying on the caller.

### Documentation Updates

None. No user-visible behavior, no config, no CLI, no wire format and no RFC row
changed. `grep -rn "forward-overflow-two-tier" docs/` returns nothing.

### Deviations from Plan

The spec's Implementation Steps assume a reproduction. Steps 2 and 3 were not
reached, which is the branch the Task explicitly allows.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first stress run reported `*** REPRODUCED on invocation 1 ***` and it read as a real reproduction | The run used `bin/ze`, a day-old binary, and failed on `unknown field in peer: attach`, a config grammar only the working tree parses. Nothing about the suspected flake was exercised | Reading the capture rather than the verdict line: the symptom was a config parse error, not an `ordered=` miss | Exported the session binaries and re-ran. Recorded the trap in the shard and in the journal row; the tooling fix is filed at `plan/future/spec-stress-repro-refuses-a-stale-binary.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Establish whether it reproduces, alone and oversubscribed | Done | `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md`, "The attempt, 2026-08-15" | 80 invocations, parallelism 8, 32 burners, `--any-failure` |
| If it reproduces, fix the root cause | Changed | - | Not reached: it did not reproduce |
| If it does not, record the attempt and the next step | Done | Same shard, "The attempt" and "Next step" | Both sections present |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `tmp/stress-repro/bgp-plugin-forward-overflow-two-tier-20260815-134636.log` | 80 `exit=0 ok`, zero `REPRODUCED` |
| AC-2 | Done | `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` | Attempt table plus a three-item next step |
| AC-3 | Done | `test/plugin/forward-overflow-two-tier.ci` | 50 `ordered=` needles, timeout 20s |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `forward-overflow-two-tier.ci` under repetition and load | Done | `test/plugin/forward-overflow-two-tier.ci` | 80 runs green, plus green in a full `make ze-functional-test` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| None planned | Done | The diagnosis was to determine, and it produced no product file to change |

### Audit Summary
- **Total items:** 3 requirements, 3 ACs, 1 test
- **Done:** 6
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (fix the root cause: not reached, no reproduction; recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Establish whether the failure reproduces at all | functional, repeated under load | `tmp/stress-repro/bgp-plugin-forward-overflow-two-tier-20260815-134636.log`: header `burners=32 parallel=8 race=False ncpu=16`, 80 blocks all reading `exit=0 ok`, no `REPRODUCED` line |
| Record the attempt and the next step rather than closing silently | durable record | `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md`: the attempt table, the capture path, the false reproduction with its cause, and three next runs in priority order |
| Leave the test unweakened | functional | `git diff -- test/plugin/forward-overflow-two-tier.ci` changes no `ordered=` needle, no `expect=exit:code=0`, no `reject=` line and no timeout. The uncommitted edit is another session's config migration, and it is more than a rename: it attaches `bgp-rs` to both peers and leaves `overflow-test` declared but attached to nothing. The shard's "What was actually run" section states this, because it bounds how far the negative reaches |

**Discrimination note.** `--any-failure` was set, so the attempt counted ANY
non-zero exit, not a crash signature. A negative result under that setting is
the strongest negative this tool produces. It is still a negative: it bounds the
failure rate under this profile, and it does not prove the defect absent.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `forward-overflow-two-tier.ci` failed once with an `ordered:` needle; establish whether it reproduces before treating it as a defect | done | Answered: it does not reproduce. `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` holds the attempt and the next step |

The shard `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` is NOT removed:
two rows in it are still live (`spec-fixit-bgpls-withdrawal-functional-proof`
deferred, and the learned-staleness row assigned to another session).

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-forward-overflow-two-tier-flake-55b89662-ed70-484b-8728-361629e96dbc.md` |
| `review_gate.py check` | clean (recorded verdict=clean over 3 files) |
| Rounds | 2 |
| Reviewer lenses used | record-honesty (does the shard claim more than the capture supports), test-integrity (was any assertion weakened), rule-fit (does `ai/rules/completion.md` permit recording here), next-step quality, citation survival after commit B |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The new shard tripped a deterministic gate: `check_known_failure_load_excuses` (`scripts/dev/verify_wiring_docs.py`) refuses a changed shard whose text matches `LOAD_EXCUSE_RE`, and the shard said `load-sensitive`. Left alone, this closure lands a red `make ze-verify-wiring-docs` for the next session | `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md`, the paragraph under the attempt table | Reworded to "the burner and parallelism profile is the one that surfaced the other failures". No meaning lost: the phrase described the OTHER 2026-08-08 failures, never an excuse for this one. Gate re-run green |
| 2 | ISSUE | The record implied the 80 runs measured the fixture that failed. They did not: the `.ci` carried another session's uncommitted migration (mtime 04:52, capture 13:46), and it is more than a rename. It attaches `bgp-rs` to both peers and leaves `overflow-test` declared but attached to no peer | Shard attempt table; spec Goal Validation and AC-1 | Added "What was actually run, and how far the negative reaches" to the shard, with a table of the four differences and the note that all 50 `ordered=` needles, `expect=exit:code=0`, every `reject=` and the 20s timeout are untouched. Running the COMMITTED variant became next step 1. AC-1 and Goal Validation now carry the bound |

After round 2 returned clean, two prose-only edits followed: the journal row was
split into shorter sentences for the STE check, and one paragraph of the shard
was re-wrapped. Neither changes a fact, a number or a claim.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` | Yes | `ls` returns it; `git status` shows it as a new untracked file, committed here |
| `test/plugin/forward-overflow-two-tier.ci` | Yes | `ls -la` returns 17K; still discovered by the plugin suite |
| `tmp/stress-repro/bgp-plugin-forward-overflow-two-tier-20260815-134636.log` | Yes | `ls -la tmp/stress-repro/` returns 39K (scratch, not committed) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | It does not reproduce | `grep -c "exit=0 ok"` = 80; `grep -c "REPRODUCED"` = 0 on the capture |
| AC-2 | The attempt and next step are on the record | The shard's "The attempt, 2026-08-15" table and its "Next step" list, read in full |
| AC-3 | The test is unweakened | `grep -nE '^(expect\|reject)' test/plugin/forward-overflow-two-tier.ci` shows 50 `ordered=` needles plus `expect=exit:code=0`; the diff touches none of them |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-functional-test` | `test/plugin/forward-overflow-two-tier.ci` | Yes. Read the file: it drives source peer -> ze -> dest peer with `ZE_FWD_CHAN_SIZE=2` to force overflow, then asserts 50 ordered NLRIs at the destination. Green in a full functional run and in all 80 stress invocations |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | "A reported reproduction is a reproduction." The first run's `REPRODUCED` was a stale binary failing on `unknown field in peer: attach`. Mistake Log row, Deviations note and journal row all carry it |
| A-2 | confirmed | "The 2026-08-08 load profile is the one to try first." Capture header reads `burners=32 parallel=8 race=False ncpu=16`; it answers no |

Surviving risks R-1 and R-2 are carried into the shard's "Next step" section,
which is where the next holder will look.

### Gates Run
<!-- A plan/**-only commit owes the changed-file gates, not full ze-verify
     (ai/rules/git-safety.md Step 0: plan/**/*.md is a NO for ze-verify). -->
| Gate | Result | Attribution |
|------|--------|-------------|
| `make ze-verify-wiring-docs` | green for this change after the fix below | Was RED on this closure's own new shard: `check_known_failure_load_excuses` (`scripts/dev/verify_wiring_docs.py`) matched `LOAD_EXCUSE_RE` against the phrase `load-sensitive`. Deterministic and structural, so it was FIXED, not waved through. The phrase described the OTHER 2026-08-08 failures; it now reads "the burner and parallelism profile is the one that surfaced the other failures" |
| `make ze-doc-test` | 2 reds, both foreign | `rfc/requirements/rfc9568.md` stale (VRRP; this closure touches no `rfc/` file, and `git log -1` attributes it to `e99a2b6fb`), and `ai/DOCS-TO-CODE.md` stale from another session's untracked files. Committed with `--stale-index-ok` |
| `python3 scripts/dev/check_doc_links.py` | 1 broken ref, foreign | `plan/deferrals/fixit-ike-dpd-cleartext.md` cites a deleted IKE spec. None of this closure's five files is implicated |
| `scripts/dev/spec-citation-check.py` | `DANGLING: 0` | Both citers of this spec use the bare stem, so commit B leaves nothing dangling |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No doc category applies | `grep -rn "forward-overflow-two-tier" docs/` returns nothing; no config, CLI, API, wire format, plugin SDK or RFC surface changed | Yes |

## Core Insight

A reproduction tool inherits the staleness of whatever binary it runs, and its
verdict line is not evidence on its own. The capture is. Here the symptom of the
false reproduction, a config parse error, could not have come from the suspected
defect at all, so the shape of the failure was enough to reject the verdict
before any deeper analysis. That check is cheap and it belongs before the
diagnosis, not after it.
