# Spec: closure-review-covers-spec-commits

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | - |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas's rule (`ai/META-DECISIONS.md`, "Ze is pre-release"): work is committed
as soon as a piece of it is complete, so a spec's implementation lands as
several commits, not one at closure. The closure review gate was built for the
old shape: `CheckReview` (`internal/le/commit/review.go`) judges the review
artifact's coverage against the closure commit's own paths only, so code a
spec committed before closure reaches `main` and is never covered by the
review. `ai/rules/planning.md` now states that gap.

Thomas chose the fix (2026-10-29): the closure review covers everything the
spec committed since it started, not only the closure commit. Intermediate
commits owe no review of their own.

What has to exist, to be designed at RESEARCH and DESIGN:

| Need | Why |
|------|-----|
| Each commit records the spec it belongs to | No commit carries its spec today (no `Spec:` trailer), so nothing can list a spec's commits. `./le commit create` knows the session's claimed spec (`./le spec current`) |
| The closure gate gathers the reviewable files of all the spec's commits plus the closure commit | So the review artifact must cover the whole spec's change set, judged against the files' current hashes |
| A refusal names the uncovered files and the exact `./le spec review` command | `ai/rules/user-facing-errors.md` |

Open for DESIGN: how a commit made by an agent that claimed no spec is
attributed, and what the gate does with a spec whose earlier commits predate
the trailer.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - commit keywords, refusals, trailer table
  → Constraint: every trailer `./le commit create` writes is listed in the trailer table with whether it holds the push; a new trailer gets a row there in the same change
  → Constraint: `review-override "<reason>"` writes `Review-owed: <stem>: <reason>`; the push derivation (`owed_answers.go`, `answerReviewed`) re-judges coverage from the closure commit's own blobs, so any widening of the population must reach that reader too
- [ ] `docs/contributing/spec-workflow.md` - closure commits A and B, "Review artifact" gate row
  → Constraint: closure is ONLY the removal of a spec (commit B); commit A is byte-for-byte an ordinary `in-progress` commit and the page says so ("What this gives up")
  → Constraint: commits A and B come from ONE script, so when B is prepared, A has not landed: its paths exist only in the script's earlier blocks (`scriptBase` already reads those blocks, for `plan/` markdown only)
- [ ] `ai/rules/planning.md` - review gate directive
  → Constraint: the rule states the gap ("Its coverage check sees only this commit, so code a spec committed before closure is not covered by it"); the fix rewrites that sentence

**Key insights:**
- The gap is wider than the Task states. Commit B carries only the spec removal (checked on the last two closures: `3211ac9d09` removes one file, its commit A carries 56; `e6e6089851` removes one, its commit A carries 125). Markdown is not reviewable, so B's coverage set is empty and the coverage check passes vacuously: the gate today proves a clean artifact exists, not that it covers anything. Commit A's code is as uncovered as every intermediate commit's.
- `Spec:` is already taken as free prose: 91 commits carry a body line starting `Spec: `, some naming a spec path, some prose ("Spec: closure sections appended"). The attribution key must be one nothing writes by hand.
- The claim reader exists: `specOwner.currentSpec` (`internal/le/spec/session.go`) answers the session's claim, and an agent with no claim answers its parent's. It is unexported; `internal/le/commit` needs an exported reader.
- Attribution by stem survives a session change, so a `Handoff: verify` spec (implemented by one session, closed by another) is covered.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/review.go` - `closureStem`, `closedSpecStem`, `CheckReview`, `checkReviewSelected`, `judgeReviewCoverage`, `scriptBase`
  → Constraint: coverage is judged over the `paths` of the commit under construction only; `judgeReviewCoverage` takes a `hashOf` function so the same judgement serves the prepare gate (working tree, or the hunks blob) and the push discharge (commit blobs). The widened population MUST go through this one function, not a copy
  → Constraint: `closedSpecStem` is the single statement of "which spec does this commit close" for both callers; preserve it
  → Constraint: `scriptBase` refuses an unreadable script rather than falling back to HEAD; the widened population over earlier script blocks keeps that refusal
- [ ] `internal/le/commit/prepare.go` - `checkVerificationGates`
  → Constraint: the gate runs only when `closureStemSelected` answers a stem; a non-closure commit pays nothing and MUST keep paying nothing beyond writing its attribution
  → Constraint: trailers accumulate in `result.Trailers` and render through `Message`; an attribution trailer joins that list
- [ ] `internal/le/commit/owed_answers.go` - `answerReviewed`, `answerClosed`
  → Constraint: the discharge judges an artifact against `facts.Paths` of the closure commit, and falls back to the committed Review Gate when the artifact is gone; it must judge the same widened population at the closure commit's tree
- [ ] `internal/le/commit/owed_trailers.go` - `readLine`, `declaredTrailers`
  → Constraint: every declared trailer key is parsed strictly and a malformed one holds the push; an attribution trailer that owes nothing MUST NOT become a person act
- [ ] `internal/le/spec/session.go` - `specOwner.currentSpec`, `Claim`
  → Constraint: per-session, per-agent markers with parent fallback; no shared file. The commit tooling reads the claim, never writes it

**Behavior to preserve:**
- One commit closes one spec (`oneStem`); relocations and renames are not closures
- Hunks paths are pinned by their selected blob, not the working-tree file
- `review-override` stays the only way past, writing `Review-owed:`
- A non-closure commit owes no review

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create ...` (every commit) writes the attribution; `./le commit create ... remove <spec>` (closure, usually appended to commit A's script) runs the gate; `./le commit push` re-derives a `Review-owed:` discharge

### Transformation Path
1. Ordinary commit: claim (session marker) → attribution trailer in the message
2. Closure prepare: removed spec → stem → commits on HEAD carrying that stem's attribution, plus the earlier blocks of the pending script, plus this commit → union of reviewable paths → `judgeReviewCoverage` against current content
3. Push: closure commit → same stem → same union read at the closure commit's tree → discharge verdict

### Boundaries Crossed
- `internal/le/spec` (claim) → `internal/le/commit` (trailer, gate); git history read via `gitOutput`

### Integration Points
- `judgeReviewCoverage`, `closedSpecStem`, `scriptBase`, `Message`, the trailer table in `docs/contributing/committing.md`

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation |
|----|------------|-------|----------|------------|
| A-1 | A session's claim names the spec its commits belong to | `./le spec claim` is the per-session marker and session-start reads it | An unrelated fix made while a spec is claimed is attributed to it and its closure review must cover it | DESIGN decides whether that is acceptable or needs an explicit opt-out |
| A-2 | Searching all history for the attribution trailer is cheap enough at closure | 14k commits; `git log --grep` over them is one process | Closure prepare slows noticeably | Time `git log -F --grep` on this checkout at DESIGN |
| A-3 | Earlier commits' paths still exist or were deleted by the spec, so current-content pins are meaningful | Reviewer pins the tree at review time; a later foreign edit makes the pin stale, as today | A foreign session's edit to a spec file between review and closure refuses the closure | Same refusal as today; named in the refusal |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Substring match: `git log --grep "Spec-work: spec-a"` also matches `Spec-work: spec-ab` | A closure of `spec-a` demands review of `spec-ab`'s files | `--grep` only prefilters; each candidate's message is split into lines and the trailer line must equal the key plus the stem exactly (AC-4) |
| R-2 | `declaredTrailers` is read by the push derivation, which parses every declared key as `<subject>: <reason>`; `Spec-work: <stem>` has no reason and would read as malformed, holding every push | Push refused with "malformed Spec-work: trailer" | The declaration says whether a trailer owes a person act; the derivation skips the shape check and the owed list for one that owes nothing (AC-6) |
| R-3 | The commit session (`options.Session`, eight-hex namespace) is not the harness session the claim marker is keyed by; reading the claim under the wrong one finds nothing and silently writes no trailer | Commits made under a claim carry no `Spec-work:` | The action layer resolves the claim through `internal/le/spec` (the package that writes the marker, harness session plus agent fallback) and passes the stem in `Options`; `Prepare` never resolves it (AC-1, AC-2) |
| R-4 | Commit A is unlanded when commit B is prepared in the same script, so history alone misses A's files (the exact case the gap is about) | Closure gate passes with A's code unreviewed | The gate also reads every path the script's earlier blocks commit (snapshot entries and removals), refusing an unreadable script as `scriptBase` does (AC-3) |
| R-5 | Spec renamed mid-work: earlier commits carry the old stem | Closure under the new stem covers fewer commits than the spec made | Accepted and named: the closure result lists the commits it covered, so a short list is visible. A rename is rare and a recorded decision (Known Limitations) |
| R-6 | The script's review re-check (`reviewCheckCommand`) re-runs with only this commit's files and passes while the prepare gate refused, or the reverse | Script re-check disagrees with prepare | The re-check line names the same union the prepare gate judged (AC-5) |
| R-7 | A foreign session edits a file the spec committed earlier, after the review | Closure refused as stale on a file this session did not touch last | Same refusal as for this commit's own files today; the refusal names the file and the review command, and a fresh review over current content is the answer (A-3) |
| R-8 | The push discharge and the prepare gate judge different populations, so a `Review-owed:` is answered by an artifact the gate would refuse | A discharge clears with fewer files than the gate demanded | One function produces the population for both; the discharge reads tagged commits reachable from the closure commit, hashed at the closure commit's tree (AC-7) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create` under a claimed spec (action layer resolves the claim) | → | `answerCreate` → `Options.SpecWork` → `Prepare` → `Message` trailers | `TestCreateWritesSpecWorkFromTheClaim` (`internal/le/commit/spec_work_test.go`): a temp checkout with a claim marker; the prepared message carries `Spec-work: <stem>` |
| `./le commit create ... remove <spec>` after tagged commits | → | `checkVerificationGates` → spec population → `checkReviewSelected` | `TestClosureRefusesCodeAnEarlierSpecCommitCarried` (same file): a real git history in a temp repo; the closure is refused naming the earlier commit's code file |
| `./le commit push` discharging `Review-owed:` | → | `answerReviewed` → same spec population at the closure commit | `TestDischargeJudgesTheSpecsEarlierCommits` (same file) |

## Acceptance Criteria

| AC ID | Input / Trigger | Expected Behavior |
|-------|-----------------|-------------------|
| AC-1 | `./le commit create` that changes files, made while the session (or, for an agent with no claim of its own, its parent) has `spec-x.md` claimed | The commit message ends with the trailer line `Spec-work: spec-x`, after any other trailers' block rules (`Message`) |
| AC-2 | `./le commit create` with no claim; and an `attest` commit with a claim | No `Spec-work:` trailer; the commit is not refused |
| AC-3 | Closure commit B removing `spec-x.md`, appended to a script whose earlier block (commit A) carries `a.go`; history holds a commit tagged `Spec-work: spec-x` carrying `b.go`; the review artifact pins only the spec-free closure | Refused. The problem lists `a.go` and `b.go` as not covered, and names `./le spec review record spec spec-x ...` as the route (usage shape, `...` for the reviewer's words: no command that pins) |
| AC-4 | History holds a commit tagged `Spec-work: spec-xy` carrying `c.go`; closure of `spec-x` | `c.go` is not in the population |
| AC-5 | Closure of `spec-x` whose artifact pins `a.go`, `b.go` at current content, clean | Accepted. The script's review re-check line names `a.go` and `b.go` (and this commit's own reviewable files); `ReviewResult` lists the tagged commits it covered |
| AC-6 | `./le commit push` derivation over commits carrying `Spec-work:` | No malformed-trailer problem, no owed item: the trailer owes nothing |
| AC-7 | A closure committed with `review-override` (so `Review-owed: spec-x: ...`), then an artifact that covers only the closure commit's files | The discharge refuses, naming the earlier commit's uncovered file; with an artifact covering the whole population at the closure commit's tree, it answers |
| AC-8 | A subject or body line hand-written as `Spec-work: spec-x` | Refused by `refuseHandWrittenTrailer`, naming `./le commit create` under a claim as the only writer |
| AC-9 | A legacy spec: earlier commits carry no trailer, closure carries code | Only the closure's own files and tagged commits' files are judged; untagged commits are not searched for (Thomas, 2026-10-29: cover tagged only) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateWritesSpecWorkFromTheClaim` | `internal/le/commit/spec_work_test.go` | AC-1 | |
| `TestCreateWithoutAClaimWritesNoSpecWork` | same | AC-2 (no claim, attest) | |
| `TestClosureRefusesCodeAnEarlierSpecCommitCarried` | same | AC-3 (history and earlier script block both) | |
| `TestSpecWorkMatchIsExactNotSubstring` | same | AC-4 | |
| `TestClosureAcceptsAReviewCoveringTheSpecsCommits` | same | AC-5, re-check line contents | |
| `TestSpecWorkTrailerOwesNothingAtPush` | `internal/le/commit/owed_test.go` | AC-6 | |
| `TestDischargeJudgesTheSpecsEarlierCommits` | `internal/le/commit/spec_work_test.go` | AC-7 | |
| `TestHandWrittenSpecWorkIsRefused` | `internal/le/commit/commit_test.go` (beside `TestMessageAndKeywordGrammarAreClosed`) | AC-8 | |
| `TestClaimedSpecFallsBackToTheParent` | `internal/le/spec/session_test.go` | the exported claim reader answers agent and parent as `currentSpec` does | |
| AC-9 | covered by `TestClosureRefusesCodeAnEarlierSpecCommitCarried` holding an untagged commit with `d.go` that must not appear | | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
N-A as `.ci`: `./le commit` is developer tooling with no `.ci` harness; the unit tests above drive `Prepare` and the derivation over a real temporary git repository, which is the entry point's own code path below argument parsing. The action-layer claim resolution is covered by `TestCreateWritesSpecWorkFromTheClaim` calling the action, not `Prepare`.

## Files to Modify
- `internal/le/commit/actions.go`: `trailerSpecWork` constant; `declaredTrailers` entry with an "owes nothing" marker; `answerCreate` resolves the claim and sets `Options.SpecWork`
- `internal/le/commit/input.go` (`Options`): `SpecWork` field
- `internal/le/commit/prepare.go`: append `Spec-work:` to the trailers of a native commit that changes files; `checkVerificationGates` judges the spec population; re-check line names it
- `internal/le/commit/review.go`: the spec population (tagged history with exact match, earlier script blocks' paths, this commit); `ReviewResult` gains the covered commit list; refusal names the review command
- `internal/le/commit/owed_trailers.go`: skip shape check and owed list for a trailer that owes nothing
- `internal/le/commit/owed_answers.go`: `answerReviewed` judges the population at the closure commit
- `internal/le/spec/session.go`: exported claim reader over `newSpecOwner(root).currentSpec()`
- `docs/contributing/committing.md`: trailer table row for `Spec-work:`; review gate paragraph
- `docs/contributing/spec-workflow.md`: "Review artifact" gate row and the "What this gives up" paragraph (commit A is now covered)
- `ai/rules/points/planning/work-phases/the-commit-gate-that-needs-a-clean-review-artifact.md`: replace the gap sentence; regenerate with `./le ai rules condensed-update`

## Files to Create
- `internal/le/commit/spec_work_test.go`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | developer tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | no new keyword: the trailer comes from the claim (Thomas declined an opt-out) |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no keyword |
| Functional test for new RPC/API | N-A | no RPC; see Functional Tests |
| Pipe completeness | N-A | no CLI output change |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency; reads git, which `./le commit` already requires |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer tooling, not a product feature |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md` (new trailer, wider closure gate) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/contributing/spec-workflow.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | the planning rule point named in Files to Modify |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Changed files referenced by doc anchors? | Yes | `review.go` declares `docs/features/ai-first.md` (Design header); `owed_answers.go` → `spec-workflow.md`; `owed_trailers.go` → `committing.md`, `docs/architecture/testing/verify-freshness-scope.md` (its trailer table and the `Review-owed:` answer paragraph gain the owes-nothing `Spec-work:` and the widened discharge population); `session.go` → `docs/architecture/core-design.md` (unaffected: it describes the spec library's shape, and an exported reader adds no verb). Each is read at implementation and edited where it states the old coverage |
| 17 | Existing docs show examples for this area? | Yes | committing.md trailer table examples verified against the code |

### Discovery (repo-maintenance)
| Question | Answer |
|----------|--------|
| Where does an agent look first | `ai/INDEX.md` → `docs/contributing/committing.md` (trailers) and `spec-workflow.md` (closure gate) |
| What rule prevents regression | the planning rule point on the review artifact |
| What registry prevents drift | `declaredTrailers` is the one list of trailers create writes; the hand-written refusal and the push parser both read it |
| What verification proves it | the tests above, over a real git history |

## Implementation Steps
1. Tests first (`spec_work_test.go`, the owed and session tests); run and record the red.
2. Exported claim reader in `internal/le/spec`; `Options.SpecWork`; action layer resolves it.
3. Trailer constant and declaration with the owes-nothing marker; derivation skips it; hand-written refusal comes from the declaration.
4. Native, file-changing commits get the trailer appended last.
5. Spec population in `review.go`: exact-match tagged history from HEAD (prepare) or from the closure commit (discharge), plus earlier script blocks (prepare only), plus this commit; deduplicated; one function both callers use.
6. Gate, re-check line and discharge judge that population through `judgeReviewCoverage`; refusal names the review command shape.
7. Docs and the rule point in the same pass; regenerate the rules.
8. Run the scoped tests (`internal/le/commit`, `internal/le/spec`), commit through `./le commit create`.

## Known Limitations
- Commits made before `Spec-work:` existed are not covered (Thomas, 2026-10-29: cover tagged only).
- A spec renamed mid-work loses the commits tagged with its old stem (R-5).
- A commit made under a claim for unrelated work joins that spec's closure review (Thomas, 2026-10-29: attributed anyway).

## Owned journal rows

| Row | Journal file |
|-----|--------------|
| - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le verify changed` evidence for the change set
