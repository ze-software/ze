# Spec: fixit-spec-hygiene-tooling

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Task

**[HIGH]** The spec/plan system has no freshness gate, so it rots silently. A
2026-07-16 audit found four self-reinforcing failure modes:

1. **Citations rot with no checker.** Specs reference sibling `plan/spec-*.md`
   files and `file:line` locations that drift or vanish. Example: `plan/spec-fixit-authz-admin-fallthrough.md:424,426`
   cites `plan/spec-fixit-tacacs-empty-profile-mapping.md` and
   `plan/spec-fixit-radius-empty-profile-mapping.md` as existing; both were
   deleted and folded into `plan/deferrals.md`. Thousands of `file:line`
   citations live across `plan/spec-*.md`, and `scripts/dev/` has no
   citation-freshness checker at all.
2. **A done-but-unclosed spec inflates the backlog.** `spec-closure-check.py --list`
   reports `spec-ipsec-13-rekey-wire` at HIGH confidence (its committed
   `plan/learned/1069-ipsec-13-rekey-wire.md` proves commit A ran; the spec is
   still `in-progress`). This detector exists but is not surfaced per session.
3. **65 of the open specs are never-developed `skeleton` stubs** (title plus
   template), some roughly two months old, counted as open work indistinguishably
   from committed backlog.
4. **Two specs contend on `test/.ci-sleep-baseline`** with conflicting
   absolute-target arithmetic (`spec-fixit-sleeps-cli-harness` targets 132 to 129
   vs `spec-fixit-reject-fence-observability` 132 to 130): whichever lands second
   hits a guaranteed gate conflict because the ratchet stores one absolute integer.

Scope: add a cheap `scripts/dev/spec-citation-check.py`; introduce a skeleton
TTL/triage and a backlog-vs-idea split in `ze-spec-status`; convert the sleep
ratchet to a composable count-removed delta; surface the closure advisory per
session. Registration over hardcoding: the citation gate is registered as a make
target on the verify path, not run ad hoc.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - spec metadata, status vocabulary, Spec Closure (two-commit), closure enforcement gates
  → Constraint: `skeleton` means "task defined, design not started"; a TTL/triage must not delete a spec that another gate treats as open work without recording the deletion.
  → Decision: the new citation gate joins the closure-enforcement family (detector + advisory), it does not replace `spec-closure-check.py`.
- [ ] `scripts/dev/spec-closure-check.py` - the closure detector to surface as advisory
  → Constraint: `--list` prints two tiers (high-confidence + needs-verification); reuse it read-only, do not re-implement its heuristic.
- [ ] `scripts/dev/verify_wiring_docs.py` - ratchet home (`check_ci_sleep_ratchet` :196, `_sleep_is_justified` :239, `check_ci_sleep_justification` :258)
  → Constraint: `:196` reads `test/.ci-sleep-baseline` as a single int and fails when the tree count exceeds it (:217); the delta change lives here.

**Key insights:**
- `test/.ci-sleep-baseline` currently holds `132` (verified). The absolute target is the conflict source; a delta ("this change removed N sleeps") composes across parallel specs where an absolute floor does not.
- `make ze-spec-status` runs `scripts/status/spec_status.go` (Go, not Python); the backlog-vs-idea split is a rendering change there.
- `make ze-verify-wiring-docs` (`mk/inventory.mk:70`) runs `verify_wiring_docs.py`; the citation gate wires in alongside it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/verify_wiring_docs.py` - `check_ci_sleep_ratchet` compares a whole-tree `time.sleep(` count against the absolute int in `test/.ci-sleep-baseline`; over is a fail, under prints "lower the baseline".
  → Constraint: the baseline is one integer file; two specs that both lower it collide on the second merge.
- [ ] `scripts/dev/spec-closure-check.py` - `--list` triage view already exists; nothing invokes it per session.
- [ ] `scripts/status/spec_status.go` - renders the spec inventory; treats `skeleton` rows the same as `design`/`ready` in the open-work tally.
- [ ] `plan/spec-fixit-authz-admin-fallthrough.md` - lines 424,426 cite two deleted specs (dangling references, verified absent on disk).

**Behavior to preserve:**
- `spec-closure-check.py` heuristic and exit codes; the Stop-hook block path is untouched.
- The sleep ratchet's monotonic intent (sleep count only goes down); only its representation changes from absolute to delta.
- `validate-spec.sh` section requirements; the citation gate is additive.

**Behavior to change:**
- Add a citation-freshness gate; split skeleton idea-capture from committed backlog; make the sleep ratchet composable; print the closure advisory once per session.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `make ze-verify-wiring-docs` (`mk/inventory.mk:70`) is the real verify entry that runs `verify_wiring_docs.py`; the new `spec-citation-check.py` registers on this path. `make ze-spec-status` (`scripts/status/spec_status.go`) is the real status entry for the backlog split and closure advisory.

### Transformation Path
1. A run of the verify/status target enumerates `plan/spec-*.md`.
2. The citation gate scans each spec for `plan/spec-*.md` references (FAIL if the target file is absent) and `path:line` citations (WARN if the quoted token is no longer on that line).
3. `spec_status.go` buckets rows into committed backlog (`design`/`ready`/`in-progress`) vs idea capture (`skeleton`), and flags skeletons older than the TTL.
4. The closure advisory shells `spec-closure-check.py --list` and prints its output non-blocking.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Spec text ↔ citation gate | new `spec-citation-check.py` reads `plan/spec-*.md`, resolves referenced paths/lines | [ ] |
| Citation gate ↔ verify path | registered as a make target run under the verify/wiring gate | [ ] |
| Status render ↔ ratchet | `spec_status.go` split + delta baseline in `verify_wiring_docs.py` | [ ] |

### Integration Points
- `mk/inventory.mk` (register the citation-check target and the advisory), `scripts/dev/verify_wiring_docs.py` (delta ratchet), `scripts/status/spec_status.go` (backlog split + TTL), `test/.ci-sleep-baseline` (representation change). Registration over hardcoding: discovery is one make target, not an ad hoc script call.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Referenced `plan/spec-*.md` names can be resolved by existence on disk | folded specs (tacacs/radius empty-profile) are gone from `plan/` | false positives if a spec legitimately names a deleted predecessor | grep the two known-dangling refs; confirm absent | confirmed (both absent) |
| A-2 | `path:line` citations carry a quotable token the check can re-find on that line | specs quote `!ok || len(profiles) == 0` next to `authenticator.go:104` | WARN-only if the quote convention is inconsistent | sample N specs during design | unvalidated |
| A-3 | The sleep ratchet can be expressed as a composable delta without losing its monotonic guarantee | `check_ci_sleep_ratchet` only needs count-does-not-rise | a delta scheme could permit a net rise | design the delta arithmetic; add a rising-count test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Citation FAIL is too strict and blocks the verify path on legacy drift | first run reddens on many specs | ship path-existence as FAIL, line-token as WARN; grandfather existing drift if the count is large |
| R-2 | TTL auto-deletion destroys an idea nobody re-triaged | a two-month skeleton vanishes | TTL promotes-or-flags, never silently deletes; deletion stays a human action |
| R-3 | Delta ratchet still conflicts if both deltas edit one file | second merge conflicts again | design a per-change representation that sums, not one shared integer |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| verify path runs the citation gate | -> | `spec-citation-check.py` fails on a dangling `plan/spec-*.md` ref | `test_citation_dangling_spec_fails` |
| verify path runs the citation gate | -> | WARN when a `path:line` quote is no longer on that line | `test_citation_line_drift_warns` |
| status path renders backlog split | -> | `spec_status.go` buckets skeleton vs committed backlog | `test_spec_status_backlog_split` |
| verify path runs the sleep ratchet | -> | delta baseline composes two independent removals | `test_sleep_ratchet_delta_composes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A spec references a `plan/spec-*.md` file absent on disk | `spec-citation-check.py` exits non-zero naming the spec, the dangling ref, and the citing line |
| AC-2 | A spec cites `path:line` whose quoted token is no longer on that line | the gate prints a WARN (non-fatal) naming spec, citation, and the missing token |
| AC-3 | `make ze-spec-status` on the current tree | committed backlog (design/ready/in-progress) and idea capture (skeleton) are rendered as distinct buckets; skeletons past the TTL are flagged |
| AC-4 | Two changes each remove one sleep from different `.ci` files | the delta ratchet accepts both without a baseline merge conflict; a net rise still fails |
| AC-5 | Start of a session (or a status run) | `spec-closure-check.py --list` output is surfaced as a non-blocking advisory |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_citation_dangling_spec_fails` | `scripts/dev/spec_citation_check_test.py` | AC-1 | |
| `test_citation_line_drift_warns` | `scripts/dev/spec_citation_check_test.py` | AC-2 | |
| `test_spec_status_backlog_split` | `scripts/status/spec_status_test.go` | AC-3 | |
| `test_sleep_ratchet_delta_composes` | `scripts/dev/verify_wiring_docs_test.py` | AC-4 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| skeleton age (weeks) | 0..TTL | == TTL (flag boundary) | N/A | > TTL (promote/flag) |
| sleep delta sum | net <= 0 | net 0 (no change) | N/A | net > 0 (ratchet fail) |

### Functional Tests
Test infrastructure only; no user-facing features. The gate is exercised by the
verify/status make targets; no `.ci` functional test applies.

## Files to Modify
- `scripts/dev/verify_wiring_docs.py` - convert `check_ci_sleep_ratchet` to a composable count-removed delta; optionally host the citation gate call
- `scripts/status/spec_status.go` - split committed backlog vs idea capture; flag skeletons past the TTL
- `mk/inventory.mk` - register the citation-check target and the closure advisory
- `test/.ci-sleep-baseline` - change representation from absolute floor to delta-friendly form

## Files to Create
- `scripts/dev/spec-citation-check.py` - the citation-freshness gate (dangling `plan/spec-*.md` FAIL, `path:line` token WARN)
- `scripts/dev/spec_citation_check_test.py` - unit tests for the gate

## Implementation Steps
1. **Wiring first:** add `spec-citation-check.py` with a minimal FAIL-on-dangling-ref body; register a make target on the verify path; confirm it runs and reddens on the known `authz-admin-fallthrough` dangling refs.
2. Add the `path:line` token WARN pass (non-fatal).
3. Convert `check_ci_sleep_ratchet` to a composable delta; add the rising-count and two-removal tests; update `test/.ci-sleep-baseline` accordingly.
4. Split backlog vs idea capture and add the skeleton TTL flag in `spec_status.go`.
5. Surface `spec-closure-check.py --list` as a non-blocking advisory (session start or status run).
6. Full verify; then two-commit closure with a learned summary.

Note (record, do not implement here): separately close `spec-ipsec-13-rekey-wire`
(HIGH-confidence completed-but-not-closed) and prune the un-indexed learned files;
these are operational follow-ups, out of scope for this tooling spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete, every row a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (citation gate is a registered make target)

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary cases (TTL boundary, net-zero delta) present

## Notes
- Skeleton captured from the 2026-07-16 repository audit (HIGH, with the sleep-ratchet
  and closure-advisory MEDIUMs folded in). Design not started.
- Verified drift: `test/.ci-sleep-baseline` = 132; `spec-fixit-authz-admin-fallthrough.md:424,426`
  cites two deleted specs; `plan/learned/1069-ipsec-13-rekey-wire.md` is committed while
  `spec-ipsec-13-rekey-wire` stays `in-progress`; 65 skeleton specs on disk.
- Grounding re-verification (2026-07-17, readiness pass): all cited sources confirmed
  live against source. `test/.ci-sleep-baseline` still `132`; `check_ci_sleep_ratchet`
  at `verify_wiring_docs.py:196` reads it as a single int and fails at `:217` when the
  tree count exceeds it (`_sleep_is_justified:239`, `check_ci_sleep_justification:258`
  also confirmed). `ze-verify-wiring-docs` is at `mk/inventory.mk:70`.
  `spec-closure-check.py --list` emits the two documented tiers ("Completed but not
  closed — high confidence" + "Possibly closable — NEEDS VERIFICATION"). `spec_status.go`
  only sort-orders `skeleton` (statusOrder:48) as one more status row — there is no
  backlog-vs-idea split yet, confirming the gap AC-3 fills. The two dangling refs
  persist but the citing lines DRIFTED from `:424,426` to `:439,441` (the authz spec was
  edited 2026-07-17) — a live demonstration of exactly the `path:line` drift this
  checker targets. Skeleton stubs on disk are now `75` (was `65` at the 2026-07-16
  audit); the count only grew, so the triage need is unchanged. Many specs also cite
  legitimately-closed (git-rm'd) predecessor specs, so AC-1's dangling-file FAIL must
  grandfather existing drift (R-1) or it reddens broadly on first run.
- Open question for design: should the citation line-token check be WARN-forever, or
  ratchet to FAIL once the existing drift is cleaned up?
  → AUTONOMOUS DEFAULT (2026-07-17): WARN-forever initially. Rationale: per
    `ai/rules/discovery-updates.md` a new ratchet ships as a warning and only hardens
    once the tree is clean and the check is proven low-false-positive. The line-token
    heuristic (A-2, still unvalidated) can false-positive on inconsistent quote
    conventions, whereas path-existence (AC-1) already fails closed on the
    higher-signal dangling-file case — so the fatal signal stays reserved for the
    unambiguous check. This keeps AC-2 and R-1 (both already WARN) unchanged; no
    dependent Wiring/AC/Files row needs to move. Possible future follow-up (OUT OF
    SCOPE for this spec): after the existing line drift is cleaned up and the WARN
    pass runs a full cycle at a low false-positive rate, harden the line-token check
    from WARN to FAIL under a separate ratchet spec. Thomas: override if wrong.
