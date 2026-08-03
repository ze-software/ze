# Spec: fixit-wire-edit-orphaned-design-refs -- two `// Design:` refs point at a spec this session deleted

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**THE TWO-LINE EDIT LANDED on 2026-08-02**, by the session closing
`spec-rfcgate-1b-rfc7296-pilot.md`, which met this red on its own commit path and
fixed it rather than routing around it (`ai/rules/no-parking.md`). The gate below
is GREEN. What remains of this spec is its own closure: the audit, the
verification tables, and commit B.

`make ze-verify-wiring-docs` was RED, and this spec was the reason. The gate it
runs, `python3 scripts/dev/check_doc_links.py --design-only`, exited 1 with
exactly two findings (re-run and confirmed on 2026-08-02):

| File | Line | Broken reference | Now |
|------|------|------------------|-----|
| `internal/component/bgp/filterapi/fingerprint.go` | 1 | `spec-wire-edit-5-fanout-dedup` (deleted) | `plan/learned/1321-wire-edit-5-fanout-dedup.md` |
| `internal/component/bgp/reactor/forward_dedup.go` | 1 | `spec-wire-edit-5-fanout-dedup` (deleted) | `plan/learned/1321-wire-edit-5-fanout-dedup.md` |

The two dead targets are named above WITHOUT their `plan/` prefix on purpose.
`scripts/dev/spec-citation-check.py` reads any `plan/spec-*.md` string in a spec
as a live citation, so writing the path in full made this spec's own bug report
count as three more dangling references and kept a second gate red.

The spec they cited no longer exists. Commit `c181e0121` ("docs(plan): remove the
closed wire-edit-5-fanout-dedup spec") removed it as closure commit B while both
files still cited it. **This was a self-inflicted gate failure, not an inherited
one.** The fix finished the closure that this repository's own rule requires:
`ai/rules/planning.md` "Design references survive closure" says to grep for
citations BEFORE commit B. That grep was not run.

Each `// Design:` line now points at the learned summary that replaced the spec,
`plan/learned/1321-wire-edit-5-fanout-dedup.md`, and keeps its trailing topic
annotation: "fingerprint the edit set, confirm by equality" for `fingerprint.go`,
"one materialization per policy group" for `forward_dedup.go`.

One more mention of the deleted spec exists and the design-only gate does NOT
flag it: `internal/component/bgp/reactor/forward_dedup_bench_test.go:20` names it
in prose ("A-1 in ... is settled"). Left alone: it is a `_test.go` file, exempt
from the `// Design:` contract, and its sentence is a true statement about a spec
that once existed. Decide at closure whether to correct it for accuracy.

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 1. Filed rather
than fixed because the closing session was told to stop expanding scope.

## Required Reading

- [ ] `ai/rules/planning.md` - "Spec Closure", the design-reference clause
  → Constraint: before a closure commit B, grep for citations of the spec and re-point them.
- [ ] `ai/rules/design-doc-references.md` - the `// Design:` contract
  → Constraint: the `// Design:` line is the FIRST comment in the file, and it must resolve.

**Key insights:**
- The replacement target is a learned summary, not another spec. That is the normal end state after closure.
- The gate scans the whole tree on every verify, so this cannot be waited out.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/filterapi/fingerprint.go` - line 1 cites the deleted spec; the file encodes `ModAccumulator` state as a digest
- [ ] `internal/component/bgp/reactor/forward_dedup.go` - line 1 cites the deleted spec; the file counts one materialization per policy group
- [ ] `scripts/dev/check_doc_links.py` - the checker; `--design-only` restricts it to `// Design:` lines
- [ ] `scripts/dev/verify_wiring_docs.py` - `check_design_refs` runs the checker unconditionally on every verify

**Behavior to preserve:** every `// Design:` line keeps its trailing topic annotation, and stays the first comment in its file. No code changes, no `// Related:` or `// Overview:` line moves.

**Behavior to change:** the two reference targets only.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
`make ze-verify-wiring-docs` calls `check_design_refs` in `scripts/dev/verify_wiring_docs.py`, which shells out to `scripts/dev/check_doc_links.py --design-only`.

### Transformation Path
1. The checker walks every source file in the tree and reads the `// Design:` line.
2. It resolves each cited path against the working tree.
3. An unresolvable path is reported as a broken reference and the process exits 1.
4. `verify_wiring_docs.py` turns that non-zero exit into a gate failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go source ↔ plan tree | a `// Design:` comment naming a repository-relative path | Yes, by `check_doc_links.py --design-only` |
| Verify gate ↔ checker | subprocess exit code | Yes, re-run on 2026-08-02, exit 1 with 2 findings |

### Integration Points
- `scripts/dev/check_doc_links.py` - the only consumer of these two lines.
- `plan/learned/1321-wire-edit-5-fanout-dedup.md` - the replacement target, already on disk.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | N-A | comment-only change |
| No unintended coupling | N-A | comment-only change |
| No duplicated functionality | N-A | comment-only change |
| Zero-copy preserved where applicable | N-A | no code path touched |
| Registration over hardcoding (`ai/rules/plugin-self-containment.md`) | N-A | no command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `plan/learned/1321-wire-edit-5-fanout-dedup.md` carries enough design content to be the reference target. | The file exists and was written as the spec's replacement at closure. | Point at a subsystem architecture doc instead, or write one. | read the summary before editing | unvalidated |
| A-2 | Exactly two files are broken. | `check_doc_links.py --design-only` run on 2026-08-02 reported 2. | Re-point every file the re-run names. | re-run the checker before and after | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Another session closes a different spec in parallel and adds new broken refs, so the gate stays red after this fix. | The re-run names a file this spec does not list. | Fix what the re-run names. The gate is tree-wide, so a partial fix is still red. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A wrong target leaves a reader pointed at the wrong document. |
| How is it reverted? | Single commit revert. |
| Who else touches this path? | Any session closing a spec that these two files might come to cite. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify-wiring-docs` | → | `check_design_refs` in `scripts/dev/verify_wiring_docs.py` | `python3 scripts/dev/check_doc_links.py --design-only` exits 0 (existing gate, no new feature) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `python3 scripts/dev/check_doc_links.py --design-only` | Exits 0 with no broken reference |
| AC-2 | `make ze-verify-wiring-docs` | Passes |
| AC-3 | Each edited file | The `// Design:` line is still the first comment and still carries its topic annotation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none new) | `scripts/dev/check_doc_links.py` is the existing checker | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| not applicable, no user-facing behavior changes: this edits two comment lines. The driving surface is `scripts/dev/check_doc_links.py` | `scripts/dev/` | a developer runs the verify gate and it passes | |

## Files to Modify
- `internal/component/bgp/filterapi/fingerprint.go` - re-point the `// Design:` target
- `internal/component/bgp/reactor/forward_dedup.go` - re-point the `// Design:` target

## Implementation Steps

1. Read `plan/learned/1321-wire-edit-5-fanout-dedup.md` and confirm it is the right target (A-1).
2. Re-run `python3 scripts/dev/check_doc_links.py --design-only` to get the current list (A-2).
3. Edit each named `// Design:` line. Keep the topic annotation.
4. Re-run the checker. It must exit 0.
5. Decide on `forward_dedup_bench_test.go:20`, which the gate does not flag.
6. Run `make ze-verify-wiring-docs`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every file the checker named is edited, not only the two listed here |
| Correctness | The new target resolves, and the topic annotation still describes the file |
| Rule: `ai/rules/design-doc-references.md` | The `// Design:` line is still the first comment in each file |
| Registration over hardcoding | N-A, comment-only change |

## Known Limitations
- This fixes the citations. It does not add a gate that would have caught the omission at commit time. That prevention belongs to `ai/rules/planning.md` and its closure checklist.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste the checker's exit 1 output)
- [ ] Tests PASS (paste the checker's exit 0 output)
