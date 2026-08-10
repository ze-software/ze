# Spec: the draft-incubator carve-out clears the guard for live tests

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 2/2 |
| Deferral shard | - |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The draft-incubator carve-out committed in `9f6cada32` turns an already-raised
block into a pass, and one shape of command reaches that state with a LIVE test
in its argument list.

**A Go test is invisible to the all-must-be-drafts rule.** `_draft_only`
(`.claude/hooks/pretool-bash.py`) collects targets with
`re.findall(r"[^\s'\"]*test/[^\s'\"]*", cmd)`, which requires the literal
`test/`. A Go test path carries `_test.go` and usually no `test/` segment, so it
is never collected, `all()` sees only the draft, and the command clears. The
function's own docstring states the opposite: "Requires EVERY named path to be a
draft."

    rm test/draft/p/wip.ci internal/component/bgp/reactor/peer_test.go

`check_test_deletion` returns `None`, which is ALLOW. Before `9f6cada32` that
command blocked.

Two consequences of the same miss ride with it, and one line closes all three.
A draft named outside the deleting verb's arguments (a trailing comment, a
second segment of an `&&` chain) also cleared the block, because the live path
beside it was the invisible Go test. And a traversal path
(`rm test/draft/../plugin/live.ci`) matched `startswith("test/draft/")` on raw
text, so the incubator prefix bought the removal of the live test it reaches.

The goal: the carve-out passes a draft and nothing else, and a fixture fails if
it is ever widened again.

**How this was found.** An independent review run on 2026-08-08 to satisfy the
review gate for the guard-must-know-what-is-not-yet-its-subject record,
which documents the carve-out. The verdict was not clean, so that summary cannot
land until this spec does. It is the one file of the 2026-08-08 commit sweep
held back for a reason other than its own content.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - the draft workflow the carve-out exists to serve
  → Constraint: a draft ends in exactly two moves, promote it or delete it, so
    deleting a draft must need no approval. That direction stays.
- [ ] `ai/rules/evidence.md` - the guard rules this defect breaks
  → Constraint: a guard fails closed or says something. A permissive value on
    the miss path means the guard does not exist.

**Key insights:** (minimal context to resume after compaction)
- The carve-out is fail-open by construction: it converts a raised block into a
  pass on text in the same command line.
- The whole fix is what the matcher COLLECTS. Once a Go test path is a target
  and every target is normalized, the all-must-be-drafts rule does the rest.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `.claude/hooks/pretool-bash.py` - `DRAFT_DIR`, `_draft_only`,
      `check_test_deletion` (lines 386-427): raises the errors, then drops them
      at `if not errors or _draft_only(cmd)`
- [ ] `.claude/hooks/pretool-writeedit.py` - `DRAFT_SEGMENT`, `_is_draft`,
      `c_test_weakening` (lines 2393-2437): the skip precedes both the RFC-tag
      branch and the weakening heuristic
- [ ] `scripts/dev/hook-fixture-check.py` - `run_draft_incubator`: the six
      fixtures that pin the carve-out today

**Behavior to preserve:**
- Deleting or editing a real draft under `test/draft/` needs no approval. That
  is the point of the carve-out, and `9f6cada32` landed it correctly for the
  shapes its fixtures cover.
- `test/drafts/`, `test/plugin/draft-x.ci` and `TEST/DRAFT/` are not the
  incubator and keep blocking.
- The incubator root stays deletable, with or without its trailing slash.

**Behavior to change:**
- A command that names any non-draft test path blocks, whatever else it names.
- A test path is any token carrying a `test/` segment or a `_test.go` name, so a
  Go test is one whether or not a `test/` directory sits above it.
- Every collected token is normalized before it is matched, so
  `test/draft/../plugin/live.ci` is judged as the live test it reaches.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `Bash` tool call carrying `rm`, `git rm` or `git checkout`, dispatched to
  `check_test_deletion` (`.claude/hooks/pretool-bash.py`).

### Transformation Path
1. The dispatcher builds `ctx` and passes the raw command string through.
2. `check_test_deletion` raises its block over the whole command line.
3. `_draft_only` collects every test-shaped token of that same command line,
   normalizes each one, and drops the block only when every one of them is the
   incubator or sits under it. An empty target list leaves the block standing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | JSON tool input on stdin, exit code out | No |

### Integration Points
- `scripts/dev/hook-fixture-check.py` - the fixture suite is the only surface
  that drives these checks from their entry points.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the exemption reads the same command line the block is raised over (`check_test_deletion`, `.claude/hooks/pretool-bash.py`) |
| No unintended coupling (components stay isolated) | Yes | `_draft_only` gained one call to `os.path.normpath`; the hook takes on no new import and no new file |
| No duplicated functionality (extends existing, does not recreate) | Yes | the existing matcher was corrected in place; no second matcher, no new module |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire or buffer code |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | no registry surface; the hooks are dispatched by `.claude/settings.json` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Correcting what the matcher COLLECTS closes the defect, with no shell parsing and no second module | the block is raised over the whole command line, so a rule over every test-shaped token of that line is the same scope | the carve-out would have to read each verb's arguments per shell segment | the 14-case probe from `check_test_deletion` (all 14 correct, 3 red at HEAD) | confirmed |
| A-2 | Normalizing each token is enough to defeat a traversal spelling | `os.path.normpath` is lexical and needs no filesystem | a resolved path would be needed, and a link inside the incubator would still name its target | `draft-traversal-rm-still-needs-approval` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stricter carve-out re-blocks legitimate draft deletion, which is the friction `9f6cada32` removed | an agent cannot empty `test/draft/` | the six existing fixtures plus `draft-root-rm-recursive-needs-no-approval` and `draft-pair-rm-needs-no-approval` pin the permitted direction. The root fixture is RED against a normalize-only matcher, because `test/draft/` normalizes to `test/draft` and is not under `test/draft/` |
| R-2 | A `.ci` test outside every `test/` directory is not a test-shaped token, so a draft named beside it would clear the block | such a file appears in the tree | none needed today: every `.ci` in the repository sits under a `test/` directory (`find . -name '*.ci' -not -path './test/*'` returns only worktree copies, which carry `test/` in their own path, and one `tmp/` scratch file) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. The guard governs agent behavior in this repository: too loose and a live test is deleted without approval, too tight and an agent cannot empty the draft incubator |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any session editing `.claude/hooks/`; `scripts/dev/hook-fixture-check.py` is shared |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `Bash` tool call naming a draft and a Go test | → | `check_test_deletion` (`.claude/hooks/pretool-bash.py`) | `mixed-go-test-rm-still-needs-approval` |
| a `Bash` tool call naming a draft in a second shell segment | → | `check_test_deletion` | `draft-then-live-go-test-rm-still-needs-approval` |
| a `Bash` tool call on a traversal path | → | `check_test_deletion` | `draft-traversal-rm-still-needs-approval` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rm test/draft/a.ci internal/x/y_test.go` | Blocked. A Go test path is a test path whether or not it carries a `test/` segment |
| AC-2 | `rm test/draft/a.ci && rm internal/x/y_test.go`, and the same chain written with a newline | Blocked. The exemption is read over the same command line the block is raised over, so a second segment buys nothing |
| AC-3 | `rm -r test/draft/a.ci test/plugin/` and `git rm test/draft/a.ci test/plugin/live` | Blocked, both. `git rm` and the recursive form reach the same rule |
| AC-4 | `rm test/draft/../plugin/live.ci` | Blocked. Each token is normalized before it is matched, so the path is judged where it lands |
| AC-5 | `rm test/draft/wip.ci`, `rm -r test/draft/plugin/`, `rm test/draft/a.ci test/draft/b.ci`, `rm -r test/draft/` and an edit of an `RFC requirement:` tagged draft | Allowed. The six fixtures of `9f6cada32` stay green, and the incubator ROOT stays deletable with or without its trailing slash |
| AC-6 | `rm internal/x/y_test.go` alone | Blocked. An empty target list is not a draft list, and a carve-out that drops an already-raised block returns False on every miss path |
| AC-7 | AC-1, AC-2 (the `&&` form) and AC-4 re-run against the code as `9f6cada32` left it | RED, all three. AC-3, AC-5 and AC-6 are controls: they pass in both worlds, and the root case of AC-5 is RED against a normalize-only matcher that drops the incubator-root clause |

## Design Decisions

| Decision | Reason |
|----------|--------|
| The two guards keep their own draft matchers. No shared `is_draft` module | The `pretool-writeedit.py` matcher is not part of this defect: `c_test_weakening` takes ONE `file_path`, so no live test can ride into it beside a draft. Sharing a predicate would be a second module and a second load path bought for a symmetry nobody reads, and the fix here is one line in the matcher that IS wrong |
| The path is normalized (`os.path.normpath`), not resolved (`os.path.realpath`) | A link planted under `test/draft/` is not the failure this hook exists to catch. The hook guards an agent against deleting a live test by accident, and an agent that first creates a link to a live test inside the incubator has already decided to delete it. Resolution also costs a filesystem call on a path that need not exist, which normalization does not |
| The carve-out stays a rule over the whole command line, with no shell parsing | The block is raised over the whole command line, so the exemption is computed over the same text. Extracting each verb's arguments per segment would add a parser, a quoting failure mode, and its own miss paths, to reach the same verdict on every shape tested here |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| n/a | n/a | the hooks carry no unit-test surface; the fixture suite is the driver | n/a |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| n/a | no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
All nine live in `run_draft_incubator` (`scripts/dev/hook-fixture-check.py`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mixed-go-test-rm-still-needs-approval` | `run_draft_incubator` | an agent deletes a live Go test beside a draft (AC-1). RED at HEAD | done |
| `draft-then-live-go-test-rm-still-needs-approval` | `run_draft_incubator` | the same two deletes chained with `&&` (AC-2). RED at HEAD | done |
| `draft-traversal-rm-still-needs-approval` | `run_draft_incubator` | an agent deletes through `test/draft/..` (AC-4). RED at HEAD | done |
| `draft-then-live-newline-rm-still-needs-approval` | `run_draft_incubator` | the same chain written with a newline (AC-2) | done |
| `mixed-recursive-rm-still-needs-approval` | `run_draft_incubator` | `rm -r` over a draft and a live directory (AC-3) | done |
| `mixed-git-rm-still-needs-approval` | `run_draft_incubator` | the same argument list under `git rm` (AC-3) | done |
| `live-go-test-rm-still-needs-approval` | `run_draft_incubator` | a Go test deleted alone, with no draft named (AC-6) | done |
| `draft-root-rm-recursive-needs-no-approval` | `run_draft_incubator` | an agent empties the incubator root (AC-5). RED against a normalize-only matcher | done |
| `draft-pair-rm-needs-no-approval` | `run_draft_incubator` | an agent deletes two drafts in one command (AC-5) | done |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| n/a | n/a | n/a | Scope is tooling; no protocol peer is involved | n/a |

## Files to Modify
- `.claude/hooks/pretool-bash.py` - `_draft_only`: the collecting regex sees a
  `_test.go` name, every token is normalized, and the incubator root stays a
  draft. `check_test_deletion` is untouched
- `scripts/dev/hook-fixture-check.py` - `run_draft_incubator`: the nine fixtures
  above
- `ai/rules/points/testing/draft-a-functional-test-before-it-is-live-blocking/a-draft-is-promoted-or-deleted-never-left.md`
  (rendered into `ai/rules/testing.md`) - names what a test path is

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface; this is an agent guard |
| YANG validation constraints | N-A | no config leaf |
| YANG custom validators | N-A | no config leaf |
| CLI commands/flags | N-A | no CLI surface |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no config leaf |
| Functional test for new RPC/API | N-A | no RPC; the fixture suite is the functional surface |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | N-A | no observable daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | an agent guard, not a product feature |
| 2 | Config syntax changed? | No | no config surface |
| 3 | CLI command added/changed? | No | no CLI surface |
| 4 | API/RPC added/changed? | No | no RPC |
| 5 | Plugin added/changed? | No | no plugin |
| 6 | Has a user guide page? | No | no operator-facing page |
| 7 | Wire format changed? | No | no wire code |
| 8 | Plugin SDK/protocol changed? | No | no SDK code |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no protocol code |
| 10 | Test infrastructure changed? | Yes | done: the `ai/rules/testing.md` draft-workflow point now names what a test path is |
| 11 | Affects daemon comparison? | No | no daemon behavior |
| 12 | Internal architecture changed? | No | no component boundary moves |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | no metric |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | done: `ai/rules/testing.md` is the one anchor naming `check_test_deletion`, and it now states which paths that guard counts |
| 17 | Existing docs show config/CLI/API examples for this area? | No | no examples cover the hooks |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the nine fixtures, three RED
   - Tests: the nine names in the Functional Tests table
   - Files: `scripts/dev/hook-fixture-check.py`
   - Verify: run the section against the hooks as `9f6cada32` left them. The
     three AC-1/AC-2/AC-4 fixtures fail there; the six controls pass in both
     worlds and say so (AC-7)
2. **Phase: correct what `_draft_only` collects**
   - Tests: all nine
   - Files: `.claude/hooks/pretool-bash.py`
   - Verify: the section is green, and every control that was green stays green
     (AC-1 to AC-6)

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a fixture, and the three defect fixtures are red before the fix |
| Feature completeness | Each of the three defect shapes is checked on its own, not only in combination |
| Correctness | The carve-out passes a draft and nothing else. Name the miss path and the value it returns |
| Simplicity | The diff is the smallest one that makes every fixture pass. Name what was NOT built and why (Design Decisions) |
| Data flow | Normalization happens before matching, on every collected token |
| Rule: `ai/rules/evidence.md` | The guard fails closed: an empty target list leaves the block standing |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The new fixtures | `python3 scripts/dev/hook-fixture-check.py` names them and reports all green (section `draft-incubator`, 15 fixtures) |
| The three defect fixtures discriminate | re-run the section against the `9f6cada32` version of `pretool-bash.py` and observe 3 RED of 15 |
| The six existing fixtures still pass | the same run reports `run_draft_incubator` fully green |
| That record can land | `/ze-review` re-run returns clean and `review_gate.py record` succeeds |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
