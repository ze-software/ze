# Spec: doc-claims-are-checked-not-just-resolved

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | `plan/spec-fixit-dead-design-pointers-in-tests.md` |
| Phase | - |
| Deferral shard | `plan/deferrals/doc-claims-are-checked-not-just-resolved.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every documentation-freshness gate in this repository verifies REFERENCE
INTEGRITY and none verifies CLAIM TRUTH. A path that exists, a line inside a
file, a pointer that resolves: all checked. Whether the sentence above the
anchor describes what the named function does: never checked.

Measured while closing `plan/spec-problem-journal.md`, which moved 1155
`// Design:` pointers and wrote about 130 new architecture documents:

| Evidence | Number |
|---|---|
| Go-targeted `<!-- source: -->` anchors swept | 1611 |
| Anchors naming a symbol ABSENT from the file they point at | 82 |
| Of those, found by an independent reviewer sampling by hand | 7 |
| False prose claims in one IKE page, each above a resolving anchor | 3 |
| Dead citations hidden behind a `doc-links: ignore` marker no gate reads | 98 |

Every one of the 82 passed a green gate. A resolving anchor lends a false
sentence credibility, which is the mechanism this spec exists to break.

Three changes. The `_test.go` blindness in `check_design_refs` is the fourth and
it has its own spec, named in Depends.

1. **The source-anchor gate verifies the symbols it already carries.** An anchor
   is `<!-- source: <path> -- Sym1, Sym2 -->`. `check_source_anchor_stale_paths`
   in `scripts/dev/validate.py` verifies `<path>` and ignores everything after
   the `--`. Verify the symbols too.
2. **A suppression carries a reason a gate reads.** `doc-links: ignore` is
   honoured by `check_doc_links.py` and read by nothing else, so 98 dead
   citations sat behind it while `digest_check.py` was hard red on the same
   lines. A marker with no audited reason is a silent allowlist.
3. **An independent reader is mandatory where prose makes claims.** Only a
   reader can falsify a sentence. `/ze-review-docs` exists; nothing requires it
   when a spec touches a subsystem document.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - which gate owns which surface
  → Constraint: a new gate is registered in the gate list, or nobody discovers it
- [ ] `ai/rules/evidence.md` - why a claim needs its producer read
  → Constraint: this spec mechanises the cheap half; the reader is still owed for the rest

**Key insights:**
- The symbol list already exists in every anchor. This is a gate that stopped
  short, not a convention that needs inventing.
- `gopls symbols <file>` resolves a file's declarations for about 1.3 KB against
  44 KB for the file itself (`ai/rules/context-economy.md`), so the check is cheap.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/validate.py` - `check_source_anchor_stale_paths()` verifies the path only; `SOURCE_ANCHOR_RE` captures it
- [ ] `scripts/dev/check_doc_links.py` - `check_design_refs()`, `path_resolves()`, and the `doc-links: ignore` handling
- [ ] `scripts/dev/digest_check.py` - `check_digest()` verifies `file:line` exists and is in range, and does NOT read `doc-links: ignore`
- [ ] `ai/skills/ze-review-docs.md` - the reader that can falsify a claim

**Behavior to preserve:**
- Every current check keeps its coverage. This adds, it never relaxes.
- An anchor whose target is outside the repo stays exempt: `check_source_anchor_stale_paths` already documents why a `~/` or `/` path cannot be resolved here.

**Behavior to change:**
- A symbol named in an anchor must exist in the anchored file.
- A `doc-links: ignore` marker must carry a reason, and an unreasoned marker fails.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-doc-test` and `make ze-validate` run the gates over `docs/` and `ai/digests/`.

### Transformation Path
1. `SOURCE_ANCHOR_RE` captures the path; the symbol list after `--` is currently discarded.
2. The new check resolves the file's declarations and compares.
3. A symbol that names a call, a parameter, or an env key rather than a declaration is the known false-positive shape: 18 of the 82 were exactly that.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ Go type information | `gopls symbols`, or a declaration scan | No |

### Integration Points
- `scripts/dev/validate.py` - extend `check_source_anchor_stale_paths` or add a sibling check beside it

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The false-positive rate is manageable, because a token after `--` may legitimately name a call or a field rather than a declaration | 18 of 82 flagged tokens were accurate on inspection | the gate cries wolf and gets ignored | run the check over the whole tree before arming it, and count | unvalidated |
| A-2 | Symbols behind a build tag resolve | `gopls` uses one build context, which already cost this work an anchor error | linux-only declarations read as absent | run the sweep under `GOOS=linux` too and diff the finding sets | unvalidated |
| A-3 | Every `doc-links: ignore` marker in the tree today can be given a reason or removed | the 98 in `ai/digests/` were removed outright, none needed a reason | arming the check reds the tree | count the surviving markers first | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The symbol check floods on the non-declaration shape and is switched off | more findings than a reviewer will read | classify a token as a declaration, a member, or free text, and gate only on declarations first |
| R-2 | Mandating `/ze-review-docs` makes every doc-touching spec slower and gets waived | the waiver appears in specs | scope the requirement to specs that CREATE a doc or change a claim, not to a typo fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A doc gate reds on correct documentation. Nothing user-visible |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every session that writes a doc |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-validate` | → | `validate.py` symbol-anchor check | `test_anchor_naming_an_absent_symbol_fails` |
| `make ze-doc-test` | → | `check_doc_links.py` marker-reason check | `test_ignore_marker_without_a_reason_fails` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An anchor names a symbol not declared in the anchored file | the gate reports the file, the anchor, and the symbol |
| AC-2 | An anchor names a symbol that IS declared there | no finding |
| AC-3 | An anchor names a call, a field, or an env key rather than a declaration | no finding, or a finding at a severity the tree can carry, per A-1's measurement |
| AC-4 | A `doc-links: ignore` marker carries no reason | the gate fails and names the line |
| AC-5 | A spec creates a `docs/architecture/` file or changes a claim in one | closure requires a recorded `/ze-review-docs` pass |
| AC-6 | The whole tree, after the work | `make ze-validate` and `make ze-doc-test` are green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_anchor_naming_an_absent_symbol_fails` | `scripts/dev/validate_test.py` | AC-1 | |
| `test_anchor_naming_a_declared_symbol_passes` | `scripts/dev/validate_test.py` | AC-2 | |
| `test_member_token_is_not_flagged` | `scripts/dev/validate_test.py` | AC-3 | |
| `test_ignore_marker_without_a_reason_fails` | `scripts/dev/check_doc_links_test.py` | AC-4 | |

### Functional Tests

The subject is a documentation gate, so the end-to-end proof is the gate running
inside its make target over the real tree, plus the existing suites staying green.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-doc-test` | `Makefile` | an agent cannot land a doc claiming a symbol that does not exist | |
| `make ze-plugin-test` | `test/plugin/*.ci` | the gate change breaks no daemon behaviour | |

## Files to Modify
- `scripts/dev/validate.py` - the symbol check
- `scripts/dev/validate_test.py` - its tests
- `scripts/dev/check_doc_links.py` - the marker-reason requirement
- `scripts/dev/check_doc_links_test.py` - its test
- `ai/rules/planning.md` (via its point files) - when `/ze-review-docs` is owed
- `ai/rules/repo-maintenance.md` (via its point files) - register the new gate

## Files to Create
- none expected

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no config surface |
| YANG custom validators | N-A | no config surface |
| CLI commands/flags | N-A | a make target |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no config surface |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no route output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | Yes | `gopls` is a runtime dependency of the check if it shells out; `make ze-setup` installs it |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface | N-A | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | agent tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` and the `ai/rules/repo-maintenance.md` gate list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on the changed scripts |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the check exists and is reachable, reporting nothing
   - Tests: `test_anchor_naming_a_declared_symbol_passes`
   - Files: `scripts/dev/validate.py`, `validate_test.py`
   - Verify: `make ze-validate` runs the new check and the tree is unchanged
2. **Phase: Measure before arming** -- run the check over the whole tree, under both `GOOS` values, and count
   - Verify: A-1 and A-2 answered with numbers, and the severity chosen from them rather than guessed
3. **Phase: Arm the symbol check** -- fix or reclassify what it finds, then let it fail the gate
   - Tests: `test_anchor_naming_an_absent_symbol_fails`, `test_member_token_is_not_flagged`
4. **Phase: The suppression reason** -- `doc-links: ignore` requires a reason
   - Tests: `test_ignore_marker_without_a_reason_fails`
   - Verify: count the surviving markers first, per A-3
5. **Phase: The reader** -- `/ze-review-docs` owed when a spec creates a doc or changes a claim
   - Files: the point files under `ai/rules/points/planning/`, then `make ze-rules-condensed`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A declaration behind a build tag is found, not reported absent |
| Naming | The finding names the anchor and the symbol, not only the file |
| Data flow | One check, extending the existing anchor walk, not a second walk over `docs/` |
| Rule: `ai/rules/evidence.md` | The gate must fail CLOSED: an unreadable file is a finding, never a pass |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The symbol check exists and fails on an absent symbol | `python3 scripts/dev/validate_test.py` |
| The tree is clean under it | `make ze-validate` |
| No unreasoned suppression survives | `git grep -n 'doc-links: ignore'` reviewed against the gate |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Repo-authored input only. An unreadable or unparseable file must be a finding, not a skip |

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

- A resolving anchor makes a false sentence MORE credible, not less. That is why
  the reviewer who found the first false claims had to be told to hunt claims
  rather than links.
- The rule text is documentation too. Two rule files asserted that a closure gate
  fired after it had silently stopped firing, and no gate can check a rule's
  claim about a gate. Only a reader catches that.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Check the symbols already in the anchors | Invent a new annotation | The convention exists and is populated. This is a gate that stopped short |
| Measure before arming | Arm and fix the fallout | 82 findings were real, but 18 flagged tokens were accurate. Arming first would teach everyone to ignore the gate |
| Keep the human reader for claim truth | Try to verify prose mechanically | A sentence is falsifiable only by reading the producer. The gate buys the cheap half |

## Known Limitations
- A symbol that exists and a sentence that is wrong about it still passes. This
  closes the anchor half, not the prose half.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional coverage: the gate runs inside its make target
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal row written for anything this teaches
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-doc-claims-are-checked-not-just-resolved.md` only
