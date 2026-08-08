# Spec: the draft-incubator carve-out clears the guard for live tests

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The draft-incubator carve-out committed in `9f6cada32` turns an already-raised
block into a pass on a substring match over the same command line, so a LIVE
test can be deleted or weakened by naming a draft path beside it. Three
independent routes reach that state. Each was found by an independent review and
confirmed against the producing function.

**Route 1: a Go test is invisible to the all-must-be-drafts rule.**
`_draft_only` (`.claude/hooks/pretool-bash.py`) collects targets with
`re.findall(r"[^\s'\"]*test/[^\s'\"]*", cmd)`, which requires the literal
`test/`. A Go test path carries `_test.go` and usually no `test/` segment, so it
is never collected, `all()` sees only the draft, and the command clears. The
function's own docstring states the opposite: "Requires EVERY named path to be a
draft."

    rm test/draft/p/wip.ci internal/component/bgp/reactor/peer_test.go

`check_test_deletion` returns `None`, which is ALLOW. Before `9f6cada32` that
command blocked.

**Route 2: the matcher reads the command line, not the deleting verb's
arguments.** `check_test_deletion` calls `_draft_only(cmd)` on the whole string,
so a draft named in a trailing comment or in a second command clears the block.
Each of these is ALLOW today:

    rm internal/x/y_test.go  # cleanup of test/draft/
    git rm internal/x/y_test.go && ls test/draft/
    git checkout -- internal/x/y_test.go ; ls test/draft/

The third also defeats the `git checkout` branch, which the carve-out was never
meant to reach.

**Route 3: neither matcher normalizes the path.** `_draft_only` tests
`t.startswith(DRAFT_DIR)` and `_is_draft` (`.claude/hooks/pretool-writeedit.py`)
tests `norm.startswith(DRAFT_SEGMENT) or "/" + DRAFT_SEGMENT in norm`, both over
raw text. So `rm test/draft/../plugin/live.ci` is ALLOW, and
`_is_draft("/repo/test/draft/../plugin/live.ci")` is `True`. `c_test_weakening`
returns on it as its FIRST statement, before `_carries_rfc_tag` and before the
weakening heuristic, so a tagged live test is editable under a draft-looking
path. `ctx["fp"]` is the raw tool input: no `normpath`, no `realpath`.

Two narrower findings ride with them. The two matchers disagree on what a draft
is: `_is_draft` matches `*/test/draft/` at any depth while `_draft_only` matches
the repo root only, and `.gitignore` covers the repo root, so the writeedit
carve-out is wider than the fact both docstrings rest on. And neither resolves
symlinks, so a link created inside the gitignored incubator points the carve-out
at a live test by name.

The goal: the carve-out passes a draft and nothing else, and a fixture fails if
it is ever widened again.

**How this was found.** An independent review run on 2026-08-08 to satisfy the
review gate for `plan/learned/1365-guard-must-know-what-is-not-yet-its-subject.md`,
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
  pass on attacker-controlled text in the same command line.
- The three routes are independent. Closing one leaves the other two open.

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
- `test/drafts/`, `test/plugin/draft-x.ci`, `TEST/DRAFT/` and `.//test/draft/`
  already behave correctly and must keep doing so.

**Behavior to change:**
- A command that names any non-draft test path blocks, whatever else it names.
- The carve-out reads the deleting verb's arguments, not the command line.
- Both matchers normalize before they match, and agree on what a draft is.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `Bash` tool call carrying `rm`, `git rm` or `git checkout`, dispatched to
  `check_test_deletion` (`.claude/hooks/pretool-bash.py`).
- A `Write`, `Edit` or `MultiEdit` tool call carrying a `file_path`, dispatched
  to `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`).

### Transformation Path
1. The dispatcher builds `ctx` and passes the raw command string or raw
   `file_path` through with no normalization.
2. The check raises its block.
3. The carve-out tests the raw text and can drop that block.

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
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The two matchers can share one definition of a draft | both encode the same `.gitignore` fact | two definitions stay and each needs its own fixtures | reading both call sites | unvalidated |
| A-2 | The deleting verb's arguments can be extracted from the command string well enough to guard on | `check_test_deletion` already parses the verb with a regex | argument extraction is unreliable, and the carve-out must instead require EVERY test-shaped token to be a draft | one fixture per shell form | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stricter carve-out re-blocks legitimate draft deletion, which is the friction `9f6cada32` removed | an agent cannot empty `test/draft/` | keep the six existing fixtures green; they pin the permitted direction |
| R-2 | Closing the matcher without scoping the arguments leaves route 2 open | the comment-form fixture still passes | one fixture per route, each red before its own fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. The guard governs agent behavior in this repository: too loose and a live test is deleted without approval, too tight and an agent cannot empty the draft incubator |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any session editing `.claude/hooks/`; `scripts/dev/hook-fixture-check.py` is shared |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `Bash` tool call naming a draft and a Go test | → | `check_test_deletion` (`.claude/hooks/pretool-bash.py`) | `mixed-rm-go-test-still-needs-approval` |
| a `Bash` tool call naming a draft outside the rm arguments | → | `check_test_deletion` | `rm-live-with-draft-in-comment-still-blocks` |
| a `Write` tool call on a traversal path | → | `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`) | `draft-traversal-edit-still-blocks` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rm test/draft/p/wip.ci internal/component/bgp/reactor/peer_test.go` | Blocked. A Go test path is a test path whether or not it carries a `test/` segment |
| AC-2 | `rm internal/x/y_test.go  # cleanup of test/draft/` | Blocked. A draft named outside the deleting verb's arguments clears nothing |
| AC-3 | `git rm internal/x/y_test.go && ls test/draft/` and `git checkout -- internal/x/y_test.go ; ls test/draft/` | Blocked, both |
| AC-4 | `rm test/draft/../plugin/live.ci`, and a `Write` to `/repo/test/draft/../plugin/live.ci` carrying an `RFC requirement:` tag | Blocked, both. The path is normalized before it is matched |
| AC-5 | `_is_draft` and `_draft_only` given the same path | The same verdict. One definition of a draft, matching `.gitignore` |
| AC-6 | A symlink under `test/draft/` pointing at a live test, deleted or edited | Blocked. The link target decides, not the link name |
| AC-7 | `rm test/draft/p/wip.ci`, `rm -r test/draft/p/`, and an edit of an `RFC requirement:` tagged draft | Allowed, unchanged. The six fixtures of `9f6cada32` stay green |
| AC-8 | Each of AC-1 to AC-6 re-run against the code as `9f6cada32` left it | RED. Every new fixture discriminates |

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
| `mixed-rm-go-test-still-needs-approval` | `scripts/dev/hook-fixture-check.py` | an agent deletes a live Go test beside a draft | |
| `rm-live-with-draft-in-comment-still-blocks` | `scripts/dev/hook-fixture-check.py` | an agent names a draft in a trailing comment | |
| `rm-live-with-draft-after-and-still-blocks` | `scripts/dev/hook-fixture-check.py` | an agent chains a draft listing after the delete | |
| `git-checkout-live-with-draft-still-blocks` | `scripts/dev/hook-fixture-check.py` | an agent reverts a live test with a draft named after it | |
| `draft-traversal-rm-still-blocks` | `scripts/dev/hook-fixture-check.py` | an agent deletes through `test/draft/..` | |
| `draft-traversal-edit-still-blocks` | `scripts/dev/hook-fixture-check.py` | an agent edits a tagged live test through `test/draft/..` | |
| `draft-symlink-edit-still-blocks` | `scripts/dev/hook-fixture-check.py` | an agent edits a live test through a link inside the incubator | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| n/a | n/a | n/a | Scope is tooling; no protocol peer is involved | n/a |

## Files to Modify
- `.claude/hooks/pretool-bash.py` - `_draft_only` and its call site in
  `check_test_deletion`: normalize, scope to the verb's arguments, and see a Go
  test path
- `.claude/hooks/pretool-writeedit.py` - `_is_draft` and the skip in
  `c_test_weakening`: normalize, resolve links, and agree with `_draft_only`
- `scripts/dev/hook-fixture-check.py` - `run_draft_incubator`: the seven
  fixtures above, each red against the code as `9f6cada32` left it

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
| 10 | Test infrastructure changed? | Yes | `.claude/hooks/README.md` if the draft workflow's description changes |
| 11 | Affects daemon comparison? | No | no daemon behavior |
| 12 | Internal architecture changed? | No | no component boundary moves |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | no metric |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` and `ai/` for anchors naming the two hook files |
| 17 | Existing docs show config/CLI/API examples for this area? | No | no examples cover the hooks |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the seven fixtures RED
   - Tests: the seven names in the Functional Tests table
   - Files: `scripts/dev/hook-fixture-check.py`
   - Verify: each fixture fails against the code as `9f6cada32` left it. A
     fixture that passes before the fix proves nothing (AC-8)
2. **Phase: one definition of a draft** -- normalize, resolve, and share it
   - Tests: `draft-traversal-rm-still-blocks`, `draft-traversal-edit-still-blocks`,
     `draft-symlink-edit-still-blocks`
   - Files: both hook files
   - Verify: `_is_draft` and `_draft_only` agree on every probe path (AC-4, AC-5, AC-6)
3. **Phase: scope the carve-out to the deleting verb's arguments** -- and see a
   Go test path
   - Tests: `mixed-rm-go-test-still-needs-approval`,
     `rm-live-with-draft-in-comment-still-blocks`,
     `rm-live-with-draft-after-and-still-blocks`,
     `git-checkout-live-with-draft-still-blocks`
   - Files: `.claude/hooks/pretool-bash.py`
   - Verify: AC-1 to AC-3 block, AC-7 still passes

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a fixture, and every fixture is red before its own fix |
| Feature completeness | The three routes are independent; check each is closed on its own |
| Correctness | The carve-out passes a draft and nothing else. Name the miss path and the value it returns |
| Naming | One name for the draft predicate across both hooks |
| Data flow | Normalization happens before matching, at every entry point |
| Rule: `ai/rules/evidence.md` | The guard fails closed: an unparseable command blocks rather than clears |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Seven new fixtures | `python3 scripts/dev/hook-fixture-check.py` names them and reports all green |
| Each fixture discriminates | re-run each against the `9f6cada32` version of the two hook files and observe RED |
| The six existing fixtures still pass | the same run reports `run_draft_incubator` fully green |
| `plan/learned/1365-*.md` can land | `/ze-review` re-run returns clean and `review_gate.py record` succeeds |

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
