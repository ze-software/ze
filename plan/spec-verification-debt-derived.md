# Spec: verification-debt-derived

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le commit create` writes a row into the tracked file
`plan/verification-debt/<session>.md` whenever the commit lacks fresh evidence,
and the commit then carries that file. The session-start hook prints
"verification debt: N gate(s) owed", and `create ... push` is refused while any
row is open. Rows are cleared by `./le commit debt-clear` (runs a full verify in
a worktree at HEAD) or answered by `./le commit debt-discharge` (four kinds,
records under `plan/verification-debt/discharged/`).

The ledger is a second copy of a derivable fact: "is the tree this push
publishes the tree a green full native verification ran over". That copy
breaks `ai/rules/principles.md` ("Every fact MUST be declared once"). It costs
three ways today:

| Cost | Evidence |
|------|----------|
| Every commit drags a tracked file it did not author | `checkVerificationGates` (`internal/le/commit/prepare.go`) appends the shard to the commit's paths |
| Parallel sessions collide on it | 2026-10-18: a commit was refused because another session had just committed the debt file |
| It drifts from reality | `./le commit debt-status` on 2026-10-18 at revision time: 753 open, 696 cleared, 386 discharged (57 not-applicable, 329 owner), while most of those rows sit on commits already on `origin/main` |

Goal (owner decision 2026-10-18, "derive it, write the spec"): delete the
ledger and its write path, and DERIVE the debt from git plus a record of the
last green full verification. The push gate, the `commit create` report and the
session-start hook all read the derivation. A person-act obligation (review,
owner approval) moves into a commit trailer the derivation reads.

### Owner decisions (2026-10-18, verbatim reply "approve")

| # | Decision | What this spec does with it |
|---|----------|-----------------------------|
| D-1 | Adopt the simpler push rule: a push needs HEAD's tree (`HEAD^{tree}`) to equal the tree recorded by the last green full verify; the list of owed commits is only for display | The gate compares two tree ids and nothing else for the verify half. The owed-commit list is computed for the message and never decides admission |
| D-2 (Q1) | Each machine re-verifies before it pushes; commits pulled from the other machine are never owed twice | The green record stays machine-local under `tmp/`. The display range is `HEAD ^@{u}`, so a commit that arrived through `origin` is in `@{u}` and never listed |
| D-3 (Q2) | An owner attestation becomes a commit trailer, keyed to something that survives a rebase (the tree or the patch-id, not the SHA) | `Owner-attested:` trailer keyed by the stable patch-id of the owed commit (see Person-act answers) |
| D-4 (Q3) | Drop the old verify rows; carry the open independent-review and RFC-approval rows as one trailer each on the ledger-deletion commit | 706 open verify rows at review time (753 at revision) are dropped. The person-act rows still open when the deletion commit is prepared each become one `Ledger-carried:` trailer on it (19 at review time, 26 on `./le commit debt-list` at revision time; the count is read when the deletion commit is prepared) |
| D-5 (Q4) | Docs-only commits still owe a verify (today's behavior) | No path filter: any tree change moves `HEAD^{tree}` away from the green tree |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - certificate, verification debt, discharge
  → Constraint: `WriteCertificate` (`internal/le/verify/engine/status.go`) overwrites `tmp/ze-verify.status` on EVERY run, red or green; on 2026-10-18 it held `exit=130` over `605de63e22`. So the last green verification is not recoverable from the certificate once a later run fails. The derivation needs its own record, written only for a green, full, unskipped run.
  → Constraint: the certificate's `tree_hash` is a `job.SnapshotTree` digest of the working tree (or `tree-moved-during-run`), NOT a git tree object id. The green record must store the git tree id of the verified commit (`<commit>^{tree}`) so it compares with `HEAD^{tree}`, and must refuse a run whose snapshot moved.
  → Constraint: `CheckCertificate` stales any non-zero exit and any mode `StagesForMode` does not know (`full-part-2-of-6`), so a cut piece never certifies. The cut aggregation today is `recordDebtPart` / `provenDebtParts` / `forgetDebtParts` (`internal/le/commit/debtpart.go`, record `tmp/ze-verify-debt-parts.json`), called from `clearDebt` (`internal/le/commit/actions.go`) with `root` = the ASKING checkout, after `verify.Run` returns. It must stay at that root when it moves: the engine runs inside the throwaway worktree, so aggregation there would be deleted with the worktree.
  → Constraint: `CopyCertificate` loops over `StatusPath` and `ManifestPath` and returns an error wrapping `os.ErrNotExist` for a missing file; `internal/le/verify/lifecycle.go` reads that error as "no certificate to publish". The green record must not be added to that loop: an absent green record is the normal outcome of a red run and must neither fail the publish nor be read as "no certificate".
  → Decision: the runnable gates (`unverified`, `structural-red-ok`, `missing-full-verify-ok`, `broken-head-fix`) all clear on the same event, one green full verify (`clearDebt` marks EVERY runnable gate passed on exit 0). They collapse to one derived question: is `HEAD^{tree}` the green tree.
- [ ] `docs/contributing/committing.md` - keywords, push, debt sections
  → Constraint: `rfc-change-ok` "records a verification-debt row naming the owner's answer as owed". Owner approvals already travel as `RFC-approved:` trailers written by `Message` (`internal/le/commit/input.go`), which is the precedent for a trailer-carried obligation.
  → Constraint: in a `ZE_REPO_ROOT` tree that is not the Ze checkout (`lepath.IsCheckout` false) no debt is written, and "the push authorisation apply in every tree" (line 309). The site and wiki push through the same route, so the new push route answers NOT-APPLICABLE for the derivation there and still pushes.
- [ ] `ai/rules/precommit-verify.md`, `ai/rules/pre-release.md`
  → Constraint: "The gate is owed before a push"; "A commit MUST NOT be held for a green gate". The derivation keeps both: `create` never refuses for debt, the push does.
- [ ] `ai/rules/completion.md` (generated from `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md`, listed in `ai/rules/points/completion/manifest.md` line 17)
  → Constraint: the directives name `plan/verification-debt/<session>.md`, `debt-clear`, `debt-discharge` and `debtGates`; they are rewritten in the point file and `ai/rules/CORE.md`/`completion.md` regenerated by `./le ai rules condensed-update`, never hand-edited.
- [ ] `ai/rules/planning.md` (generated; its line 44 comes from `ai/rules/points/planning/work-phases/the-commit-gate-that-needs-a-clean-review-artifact.md`)
  → Constraint: "`review-override <reason>` is an explicit owner decision that records a verification-debt row" becomes "writes a `Review-owed:` trailer"; edit the point, regenerate.
- [ ] `ai/rules/git-safety.md` (generated from `ai/rules/points/git-safety/directives/push-only-on-the-owners-order.md`), `AGENTS.md`, `ai/INSTRUCTIONS.md`
  → Constraint: all three name `./le commit create ... push "<owner authorisation>"` as the only push route. B1 below replaces that route, so each surface changes in the same piece of work.
- [ ] `.claude/rules/handover-in-repo.md`
  → Constraint: verification state is machine-local (`tmp/`); work moves between Linux and Mac. D-2 settles it: each machine re-verifies.

**Key insights:**
- Seven gate keys exist (`debtGates`, `internal/le/commit/debt.go`); four are runnable and collapse into one tree comparison; one is retired; two are person acts that need a trailer.
- The certificate cannot be the green record because a red run overwrites it, and its tree hash is not a git tree id.
- `create ... push` always prepares a commit block (`Prepare`, `prepare.go`: `nextTag`, `snapshotIndexEntries`, `composeScript` run for every call, and the push is rendered after the commit blocks). Under D-1 a push gate evaluated at prepare time would always refuse it: the commit the script is about to make changes `HEAD^{tree}`. The push therefore needs a route that makes no commit.
- `renderPush` (`internal/le/commit/script.go`) emits the marker line, a comment and a bare `git push`; nothing re-derives anything between prepare and the push. The commit-block precedent is the `# critical-review gate re-check` line `composeScript` writes from `commitBlock.ReviewCheck`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/prepare.go` - `Prepare` validates the tag and the push authorisation, then (Ze checkout only) runs `checkSourceGates`, `Message`, `checkVerificationGates` and, when a push is authorised, `refusePushWithDebt(root, result.Debt)`; then always allocates a tag, snapshots the index and composes a commit block. `checkVerificationGates` builds `observed` (`gateUnverified` when the certificate is not FRESH, `gateMissingFullVerifyOK` when Go and mode not full), refuses charged structural reds without `structural-red-ok`, calls `owedDebt` then `recordDebt`, adds the shard to the commit paths. `refusePushWithDebt` refuses when `openDebt` plus this commit's rows is non-zero.
- [ ] `internal/le/commit/debt.go` (476L) - `debtDir`, `debtGates`, `debtGateAt`, `ListDebt`, `readDebt`, `recordDebt`, `extendDebtRow`, `openDebt`, `clearDebtRows`, gate key and status constants. `clearDebtRows` marks every open row whose GATE passed as cleared; it never compares the row's commit with the verified commit.
- [ ] `internal/le/commit/debtpart.go` (237L) - `debtPart` (Index, Of, All), `recordDebtPart` (flock, pinned to commit and cut, resets when either moves), `provenDebtParts`, `debtPartsComplete`, `forgetDebtParts`; record `tmp/ze-verify-debt-parts.json`.
- [ ] `internal/le/commit/discharge.go` (1330L), `dischargerecord.go` (272L) - kinds `not-applicable`, `closed`, `reviewed`, `owner`. `verifyKindNotApplicable` re-runs the producer (review: `closedSpecStem` empty; RFC: `changedTaggedUnits` empty). `verifyKindClosed` reads the Review Gate from the removed spec at the closure parent via `reviewGateRecorded` (design/skeleton status owes none). `verifyKindReviewed` judges a named review artifact with `judgeReviewCoverage` (`review.go`) against the commit's own blob sha256s, and falls back to `verifyKindClosed` when the artifact is gone.
- [ ] `internal/le/commit/actions.go` - verbs `debt-list`, `debt-status`, `debt-clear`, `debt-discharge`; `clearDebt` (cut sweep with `part all of <m>`, resumes proven pieces, `hostReady` pre-check, `recordDebtPart` at `root`, `clearDebtRows`, `forgetDebtParts`); keyword table holds `push`, `unverified`, `structural-red-ok`, `missing-full-verify-ok`, `broken-head-fix`, `rfc-change-ok`; the create report line `debt=N row(s) -> <shard>`.
- [ ] `internal/le/commit/script.go` - `pushMarker`, `renderPush`, `splitPush` (one marker, final section), `commitBlock.ReviewCheck` re-check line, `PreparedHead` refusal when HEAD moved.
- [ ] `internal/le/commit/review.go`, `internal/le/spec/review.go` - review artifacts live at `tmp/review/<stem>-<harness sid>.md` (`reviewDir`), header `<!-- ze-review ... verdict=... -->`, one sha256 per reviewed path.
- [ ] `internal/le/commit/input.go` - `Message(subject, body, trailers)`, `refuseHandWrittenTrailer`.
- [ ] `internal/le/commit/verification.go` - `verificationState` over `verifyengine.CheckCertificate`; `carriesGo`. Line 1 `// Design: ... -- verification debt at commit preparation`.
- [ ] `internal/le/verify/engine/status.go` - `Certificate` (exit, timestamp, mode, skipped, git_sha, tree_hash), `WriteCertificate`, `CopyCertificate`, `ReadCertificate`, `SkippedSuites` (reads `ZE_SKIP_SUITES` and its dotted equivalent).
- [ ] `internal/le/verify/lifecycle.go` - `Options{Commit, Keep, Part, Parts}`; publishes with `CopyCertificate(path, root)`, treats `os.ErrNotExist` as "no certificate to publish".
- [ ] `internal/le/verify/status/answer.go` - `write` (`./le verify status write exit-code <n> mode <m>`) calls `WriteCertificate` at the tree root with the caller's exit and mode.
- [ ] `internal/le/verify/engine/stages.go` - `Stage.Structural` comment (line 55) "a commit is refused instead of taking a debt row".
- [ ] `internal/le/hookruntime/lifecycle.go` - `reportDebt` (`commit.ListDebt`) in `sessionStartSteps`, inside the 3.5 s `sessionStartBudget`; errors go to stderr.

**Row kinds the producer writes, and their fate:**

| Gate key (keyword) | Row gate name | Written when | Cleared today by | Derived replacement |
|--------------------|---------------|--------------|------------------|---------------------|
| `unverified` | full native verification (not FRESH-green) | certificate not FRESH for the commit's paths, or the keyword | green `debt-clear` | the tree rule (D-1). The keyword is DELETED: it admitted nothing `create` refuses, it only wrote the row |
| `missing-full-verify-ok` | full native verification over this commit's Go | commit carries Go and certificate mode is not full, or the keyword | green `debt-clear` | the tree rule; the green record only accepts mode full. The keyword is DELETED for the same reason |
| `structural-red-ok` | native structural checks (red) | the keyword admitted a charged structural red | green `debt-clear` | the tree rule. The commit-time refusal of an un-admitted structural red STAYS; the keyword stays as that admission and its reason becomes a `Structural-red-ok:` trailer, audit only |
| `broken-head-fix` | repository tracked-build/check (HEAD does not compile) | the keyword | green `debt-clear` | the tree rule. The keyword stays as the admission; reason into a `Broken-head-fix:` trailer, audit only |
| `stale-index-ok` | discovery-index freshness | never (retired 2026-09-11) | discharge only | dropped: no commit can owe it, its rows leave with the ledger |
| `review-override` | independent critical review | the keyword admitted a closure without a clean review | discharge only (not-applicable, closed, reviewed, owner) | `Review-owed:` trailer; open until one of the four answers below holds |
| `rfc-change-ok` | owner approval for an RFC-tagged test change | the keyword admitted an RFC-tagged unit change without approval | discharge only (not-applicable, owner) | `RFC-approval-owed:` trailer per unit; open until one of the three answers below holds |

**Behavior to preserve:**
- `create` never refuses a commit for missing verification; it refuses a charged structural red without `structural-red-ok`, and a closure without a clean review or `review-override`, exactly as now.
- A push is refused while any obligation is open, and the refusal names the command that clears each kind.
- The session-start hook prints one warning line and at most five items, inside its budget, and says on stderr when it cannot answer (never reads "cannot answer" as zero).
- The cut verification (pieces dealt over one commit, all must pass over ONE commit and ONE cut, a sweep resumes proven pieces) stays available, because an uncut full verify is OOM-killed on a shared box.
- NOT-APPLICABLE outside the Ze checkout; the site and wiki can still push.

**Behavior to change:**
- No tracked debt file is written; no commit carries one.
- `plan/verification-debt/` (every shard and `discharged/`) is deleted.
- Verbs `debt-list`, `debt-clear`, `debt-discharge` are deleted; `debt-status` answers from the derivation.
- Keywords `unverified`, `missing-full-verify-ok` and `push` (on `create`) are deleted. A push goes through the new `./le commit push` verb, which prepares a script with no commit block.
- A rebase re-owes every local commit (see Design Insights).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le verify worktree` (uncut, or the piece that completes one cut) exiting 0 in mode full with no skipped suite: the lifecycle writes the green record at the asking checkout's root.
- `./le commit push "<owner authorisation>"`: prepares a push-only script after the derivation admits it; the script re-derives immediately before `git push`.
- `./le commit create` (report), `./le commit debt-status`, session-start hook: read the derivation.

### Transformation Path
1. Verify lifecycle, at the ASKING root, after `CopyCertificate` succeeded: read the published certificate. Uncut run with exit 0, mode full, `skipped` empty and a tree hash that did not move: write `tmp/ze-verify-green.status` (fields below). Cut piece with the same conditions: add it to the parts record (moved from `debtpart.go`, same flock, same reset on commit or cut change, now also reset when a piece's `skipped` is non-empty); the piece that completes the set writes the green record and forgets the parts record. Anything else leaves both untouched. The green record is never written inside the worktree, so there is nothing to copy back and `CopyCertificate` is unchanged.
2. Derivation (one function, new file `internal/le/commit/owed.go`):
   1. Not the Ze checkout: NOT-APPLICABLE.
   2. Resolve `@{u}`; none: answer "cannot derive: no upstream" (error, never zero).
   3. Read the green record; none: verify-owed with "no green record".
   4. Verify gate: owed unless `HEAD^{tree}` equals the record's tree and the record's `skipped` is empty.
   5. Display list, only when the verify gate is owed: ONE call `git rev-list HEAD ^@{u} ^G` (or `git rev-list HEAD ^@{u}` with no green record or when G is not in the repository), formatted with short SHA and subject. When the list is empty but the trees differ, the message says so (HEAD's tree is not the verified tree), never "0 owed".
   6. Person acts: one `git log` over `HEAD ^@{u}` reading the declared trailers; each owed trailer is open until an answer holds (table below).
3. Consumers: `./le commit push` refuses on any open item; the push-only script runs `./le commit debt-status` (exit 0 only when nothing is owed and the derivation answered) after checking HEAD did not move, then pushes the prepared SHA; `create` prints the counts; the hook prints the warning.

**Green record fields:**

| Field | Type | Description |
|-------|------|-------------|
| commit | git SHA | the commit the worktree held |
| tree | git tree id | `<commit>^{tree}`, the value compared with `HEAD^{tree}` |
| mode | string | always `full`; any other value is not written |
| skipped | string | `ZE_SKIP_SUITES` as the certificate recorded it; written for audit, MUST be empty for the record to exist, and a reader treats a non-empty value as not green |
| cut | `uncut` or `<m>` | how the population was dealt |
| timestamp | RFC 3339 | when the record was written |

**Person-act trailers (declared once, in one table beside the keyword table in `actions.go`):**

| Trailer | Written by | Value | Read by the gate |
|---------|-----------|-------|------------------|
| `Review-owed:` | `create ... review-override "<reason>"` | spec stem the closure removes, reason | yes |
| `RFC-approval-owed:` | `create ... rfc-change-ok "<reason>"` | one per unit: `<package>.<Test>`, reason | yes |
| `Owner-attested:` | `create ... attest <commit> "<owner words>"` | stable patch-id of the named commit, the owner's words | yes (as an answer) |
| `RFC-approved:` | `create` from `./le rfc approve` (exists today) | unit | yes (as an answer) |
| `Structural-red-ok:` | `create ... structural-red-ok "<reason>"` | reason | no, audit only |
| `Broken-head-fix:` | `create ... broken-head-fix "<reason>"` | reason | no, audit only |
| `Ledger-carried:` | the ledger-deletion commit only (D-4) | original commit short SHA, gate key, reason | no, audit only (see Key Design Decisions) |

**Person-act answers (derived at read time; no record is written for the first three):**

| Answer | `Review-owed:` | `RFC-approval-owed:` | Source producer kept |
|--------|----------------|----------------------|----------------------|
| not-applicable | the commit closes no spec | the commit changes no RFC-tagged unit | `closedSpecStem`, `changedTaggedUnits` |
| closed | the removed spec, read at the commit's parent, records Review Gate rows (or its Status is design or skeleton) | N-A | `verifyKindClosed`, `reviewGateRecorded`, `specStatus`, `removedSpecPath` |
| reviewed | a clean `tmp/review/<stem>-*.md` artifact whose hashes match the commit's own blobs for every code path (the `kindReviewed` derivation the first draft omitted) | N-A | `verifyKindReviewed`, `judgeReviewCoverage`, `committedText` |
| approved | N-A | an `RFC-approved:` trailer for the same unit on a commit in `HEAD ^@{u}` | `Message` trailer parse |
| owner | an `Owner-attested:` trailer in `HEAD ^@{u}` whose patch-id equals the owed commit's stable patch-id | same | new |

The reviewed answer reads a machine-local artifact keyed by blob content, so it survives a rebase that keeps the bytes and is simply absent on the other machine (owed there, the safe direction). The owner answer is keyed by `git patch-id --stable`, which a rebase keeps unless conflict resolution changes the diff, in which case the commit is owed again.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| verify worktree to checkout | certificate published by `CopyCertificate` as today; green record and parts record written by the lifecycle at the asking root | No |
| derivation to git | `git rev-parse` (`@{u}`, `HEAD^{tree}`), one `git rev-list HEAD ^@{u} ^G`, one `git log` for trailers, `git patch-id --stable` per owed commit with an attestation in range | No |
| push-only script to git | HEAD check against the prepared SHA, `./le commit debt-status`, `git push <remote> <prepared SHA>:<upstream branch>` | No |

### Integration Points
- `verify/lifecycle.go` after `CopyCertificate` - writes the green record and the parts record at root.
- `Message` (`internal/le/commit/input.go`) - writes the new trailers, the way it writes `RFC-approved:`; `refuseHandWrittenTrailer` refuses every declared key in subject and body.
- `closedSpecStem`, `changedTaggedUnits`, `verifyKindClosed`, `verifyKindReviewed`, `reviewGateRecorded`, `removedSpecPath`, `specStatus`, `committedText` move into `owed.go` (or a sibling) before `discharge.go` is deleted.
- `script.go` - push-only script rendering with the re-check.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | consumers call one derivation function |
| No unintended coupling | Yes | commit reads a verify artifact beside the certificate it already reads |
| No duplicated functionality | Yes | deletes the ledger; the derivation is the one statement |
| Zero-copy preserved | N-A | tooling |
| Registration over hardcoding, outbound | Yes | trailers are declared in one table; `Message`, the derivation and `refuseHandWrittenTrailer` iterate it |
| Registration over hardcoding, inbound | No | search owed for `debtGates`, `debt-` verb lists in `internal/le/ai/hooks/parity.go`, `completeness_record_test.go`, and every path list naming `plan/verification-debt/` (`doc/check/retired.go`, `doc/citation/policed.go`, `site/producer.go`, `test/weakened/shard.go`, `le/path/commitsession.go`, `doc/check/links_test.go` line 290) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Equal git tree ids mean the verified bytes are the bytes pushed | a tree id is the hash of the content | none for content; intermediate commits are not each verified (Known Limitations), as today | read git object model | unvalidated |
| A-2 | The push publishes exactly the prepared SHA to `@{u}` | the push-only script pushes `<prepared SHA>:<upstream branch>` to the upstream's remote, not a bare `git push` | another ref escapes the gate | `TestPushScriptPushesPreparedSHAToUpstream` | unvalidated |
| A-3 | Dropping the open verify rows loses no obligation anyone acts on | D-4; push is the enforcement point and those commits are published | none: owner decision | owner answer D-4 | confirmed |
| A-4 | Unpushed commits only exist on the machine that made them | D-2; commits move between Linux and Mac through `origin` | a commit moved by bundle/rsync arrives with no green record and reads owed (safe direction) | owner answer D-2 | confirmed |
| A-5 | One `rev-list`, one `log`, and a patch-id only when an attestation is in range fit the session-start budget | range is tens of commits; the old hook read 369 shards in 0.6 to 0.8 s | hook cut at deadline, says so on stderr | timing in `TestSessionStartReportsDerivedDebt` | unvalidated |
| A-6 | `git patch-id --stable` of a commit is unchanged by a conflict-free rebase | patch-id hashes the diff, not the parent | an attestation stops answering after a clean rebase (safe direction: owed again) | `TestOwnerAttestationSurvivesRebase` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Red run overwrites the green record | green record changes on a red run | written only by the lifecycle on exit 0, mode full, empty skipped, unmoved tree; `TestGreenRecordSurvivesRedRun` |
| R-2 | No upstream makes the range unbounded or silently empty | `@{u}` fails | derivation errors; `./le commit push` refuses naming `git branch --set-upstream-to`; hook says "not derived" on stderr. In a linked worktree session (git-dir differs from git-common-dir) with no upstream the hook prints nothing, because a worktree agent never pushes; the push still refuses |
| R-3 | Green record from another branch or worktree sharing `tmp/` admits a push | record names a tree HEAD does not have | only tree equality admits; another branch's tree never equals HEAD's unless the bytes are the same, which is the property the gate wants |
| R-4 | Cut-piece aggregation lost in the move | no green record after every green piece | move `debtpart.go` logic and its tests into `internal/le/verify` before deleting `debt-clear`; call it from the lifecycle at root |
| R-5 | Deleting the tracked shards in one commit collides with sessions mid-commit writing a shard | `create` refusal on a removed path | land the code change first (no writer), then the deletion commit |
| R-6 | A trailer written by hand or an old `./le` is misspelled and never read | trailer present but not counted | keys declared once; `Message` refuses a subject or body line starting with a declared key |
| R-7 | A commit lands between the push script's preparation and its run | HEAD differs from the prepared SHA | the script refuses before the re-check and pushes nothing; it also pushes the prepared SHA, never the moving branch tip |
| R-8 | `ZE_SKIP_SUITES` set in the environment admits a partial verification | green record with skipped suites | the record is not written when skipped is non-empty; a reader treats a non-empty skipped field as not green; `./le verify status write` never writes it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the push gate admits an unverified push (under-count) or refuses forever (over-count); nothing in the product |
| How is it reverted? | revert the code commit; the deleted shards return with the revert of the deletion commit |
| Who else touches this path? | every session committing (`commit create`), every push (site and wiki included), the session-start hook; `plan/spec-verify-scope-0-umbrella.md` (verify scope) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit push "<auth>"` | → | derivation refuses or prepares a push-only script | `test/runner/commit-push-refused-by-derived-debt.ci` |
| push-only script | → | HEAD check and `debt-status` re-check before `git push` | `test/runner/commit-push-script-rechecks.ci` |
| `./le verify worktree` green over a commit | → | green record written at the asking root | `test/runner/verify-green-record-clears-derived-debt.ci` |
| session-start hook | → | `reportDebt` over the derivation | `TestSessionStartReportsDerivedDebt` (`internal/le/hookruntime/lifecycle_test.go`) |
| `./le commit create review-override "<r>"` | → | `Review-owed:` trailer, push refused | `test/runner/commit-review-owed-trailer.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le commit create` in the Ze checkout with a stale certificate | the generated script stages no file under `plan/verification-debt/`; the output says `debt=<n> commit(s) owe a full verify since <G or "no green record">` |
| AC-2 | `plan/verification-debt/` after the change lands | the directory does not exist in HEAD; grep of `internal/le` for the path matches only `retired.go` |
| AC-3 | green full uncut `./le verify worktree` at HEAD, no open person-act trailer, then `./le commit push "<auth>"` and the printed script | the script is prepared; running it passes the re-check and pushes the prepared SHA to the upstream branch |
| AC-4 | one commit made after the green record, then `./le commit push "<auth>"` | refused naming 1 commit by short SHA and subject, and the command `./le verify worktree` |
| AC-5 | push script prepared at AC-3, then a commit lands before the script runs | the script refuses (HEAD moved) and pushes nothing |
| AC-6 | push script prepared at AC-3, then the green record is removed before the script runs | the re-check exits non-zero and the script pushes nothing |
| AC-7 | a red full verify after a green one | the green record still names the earlier tree; the gate answer is unchanged by the red run |
| AC-8 | cut verify, every piece of one cut green over one commit | the last piece writes the green record at the asking root (not in the worktree); pieces over two commits or two cuts never write it |
| AC-9 | a cut piece green alone, a non-full mode green, any green run with `ZE_SKIP_SUITES` non-empty, a run whose tree moved | no green record written or changed |
| AC-10 | `./le verify status write exit-code 0 mode full` | writes the certificate as today; the green record is not written or changed |
| AC-11 | branch with no upstream in the main checkout | `./le commit push` refused and the hook warns on stderr, each naming the missing upstream and `git branch --set-upstream-to`; never reads as zero debt |
| AC-12 | linked worktree session, branch with no upstream | the hook prints nothing about debt; `./le commit push` still refuses |
| AC-13 | commit made with `review-override "<reason>"` | message carries `Review-owed: <stem>: <reason>`; push refused naming it until one answer holds: not-applicable, closed, reviewed (clean artifact over the commit's blobs), or `Owner-attested:` |
| AC-14 | commit made with `rfc-change-ok "<reason>"` | message carries one `RFC-approval-owed: <package>.<Test>: <reason>` per unit; push refused until not-applicable, a later `RFC-approved:` for the unit in range, or `Owner-attested:` |
| AC-15 | `create ... attest <commit> "<owner words>"` | message carries `Owner-attested: <stable patch-id>: <owner words>`; the named commit must be in `HEAD ^@{u}` and carry an owed trailer, else refused naming why |
| AC-16 | an attested commit rebased with no conflict | still answered; when conflict resolution changes its diff, owed again |
| AC-17 | `git pull --rebase` that moved the upstream, after a green verify | push refused; the list names every local commit (G is no longer an ancestor) |
| AC-18 | commits pulled from `origin` that the other machine made | never in the owed list (they are in `@{u}`) |
| AC-19 | a docs-only commit after the green record | owed, like any commit (D-5) |
| AC-20 | `structural-red-ok` / `broken-head-fix` given | admission unchanged; reason recorded as a trailer; no obligation beyond the tree rule |
| AC-21 | `unverified`, `missing-full-verify-ok`, or `push` on `create` | unknown keyword error naming the replacement (`./le verify worktree` for the first two, `./le commit push "<owner authorisation>"` for the third) |
| AC-22 | session start with HEAD's tree not the green tree | one line `verification debt: N commit(s) owe a full verify since <G>, push refused until cleared` plus up to five items; nothing when the trees match and no person act is open |
| AC-23 | `ZE_REPO_ROOT` at a non-Ze tree | `verify=NOT-APPLICABLE`, no derivation; `./le commit push` prepares the push-only script with no re-check line |
| AC-24 | `./le commit debt-list`, `debt-clear`, `debt-discharge` | unknown verb error naming the replacement (`./le commit debt-status`, `./le verify worktree`, `create ... attest`) |
| AC-25 | `./le commit debt-status` | exit 0 when nothing is owed, 1 when anything is owed (listing it), 2 when the derivation cannot answer |
| AC-26 | the ledger-deletion commit | carries one `Ledger-carried:` trailer per person-act row still open when it is prepared; the gate does not read them |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDerivedDebtEmptyWhenTreesMatch` | `internal/le/commit/owed_test.go` | AC-3 over a temp repo with upstream | |
| `TestDerivedDebtListsCommitsAfterGreen` | same | AC-4, one `rev-list` call | |
| `TestDerivedDebtReportsTreeMismatchWithEmptyList` | same | no silent zero | |
| `TestDerivedDebtRefusesWithoutUpstream` | same | AC-11 | |
| `TestDerivedDebtRebaseReowesEveryLocalCommit` | same | AC-17 | |
| `TestDerivedDebtSkipsPulledCommits` | same | AC-18 | |
| `TestDerivedDebtDocsOnlyCommitOwes` | same | AC-19 | |
| `TestDerivedDebtReadsPersonActTrailers` | same | AC-13, AC-14 (each answer kind, the reviewed kind included) | |
| `TestOwnerAttestationSurvivesRebase` | same | AC-15, AC-16, A-6 | |
| `TestDebtStatusExitCodes` | `internal/le/commit/actions_test.go` | AC-25 | |
| `TestPushScriptPushesPreparedSHAToUpstream` | `internal/le/commit/script_test.go` | AC-3, A-2 | |
| `TestPushScriptRefusesMovedHead` | same | AC-5, R-7 | |
| `TestGreenRecordSurvivesRedRun` | `internal/le/verify/green_test.go` | AC-7 | |
| `TestGreenRecordOnlyForFullUnskippedUncutOrCompletedCut` | same | AC-8, AC-9 (moved from `debtpart_test.go`) | |
| `TestGreenRecordWrittenAtAskingRoot` | `internal/le/verify/lifecycle_test.go` | AC-8, R-4 | |
| `TestStatusWriteNeverWritesGreenRecord` | `internal/le/verify/status/answer_test.go` | AC-10 | |
| `TestMessageWritesOwedTrailers` / `TestMessageRefusesDeclaredTrailerInBody` | `internal/le/commit/input_test.go` | AC-13, AC-14, AC-15, R-6 | |
| `TestCreateStagesNoDebtFile` | `internal/le/commit/commit_test.go` | AC-1 | |
| `TestRemovedKeywordsNameReplacement` | same | AC-21 | |
| `TestSessionStartReportsDerivedDebt` | `internal/le/hookruntime/lifecycle_test.go` | AC-22, AC-11, AC-12, A-5 | |

Deleted with the code they test (no assertion weakened, the behavior is removed): `debtpart_test.go` (moved, see above), `discharge_test.go`, `discharge_rfc_test.go`, `ledger_test.go`, debt tests in `commit_test.go` (`TestDebtRowsDeduplicateOnlyWhileOpen`, `TestCreateRefusesBadTagBeforeRecordingDebt` adapted to its remaining refusal, `TestDebtClearingHonorsTheGateExit`, `TestDebtClearingReportsRedVerificationAsFailure`, `TestPushProceedsWhenEveryRemainingRowIsDischarged` replaced by the AC-3 test), `TestSessionHookCountsNoDischargedRowAsOwed`. Comments that describe the ledger in surviving tests are rewritten: `snapshot_test.go` line 60, `publish_test.go` line 360. The `links_test.go` line 290 case names `plan/verification-debt/20476e05.md`; it is re-read at implementation and kept if it classifies a path string under the retired prefix, otherwise pointed at a live record path with the same expectation.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| cut `part <i> of <m>` | 1..m | i=m | 0 | m+1 (refused as today) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `commit-push-refused-by-derived-debt` | `test/runner/*.ci` | agent commits after a green, push refused, verify, push prepared | |
| `commit-push-script-rechecks` | `test/runner/*.ci` | push script prepared, a commit lands, script refuses | |
| `verify-green-record-clears-derived-debt` | `test/runner/*.ci` | green verify over HEAD empties the derived set | |
| `commit-review-owed-trailer` | `test/runner/*.ci` | review-override closure carries the trailer and holds the push until attested | |
| replaces `verify-scope-debt-clear.ci`, `verify-scope-debt-discharge.ci` | `test/runner/` | deleted with their fixture drivers in `internal/test/fixture/misc_fixture_runner.go` | |
| `le-rfc-ledger-is-derived.ci` | `test/runner/` | comment line 6 updated; assertion unchanged | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/commit/prepare.go` - drop `recordDebt` call and shard staging; drop the push path from `Prepare`; trailers instead of rows.
- `internal/le/commit/actions.go` - delete `debt-list`, `debt-clear`, `debt-discharge` and the `push`, `unverified`, `missing-full-verify-ok` keywords; add the `push` verb and the `attest` keyword; `debt-status` from the derivation with exit codes 0/1/2; report line; trailer table.
- `internal/le/commit/script.go` - push-only script: prepared SHA check, `./le commit debt-status` re-check (Ze checkout only), push of the prepared SHA to the upstream; `renderPush`/`splitPush` replaced, not kept beside it.
- `internal/le/commit/input.go` - trailers in `Message`; `refuseHandWrittenTrailer` over the table.
- `internal/le/commit/verification.go` - line 1 Design comment; green record reader if not in verify.
- `internal/le/commit/rfcchange.go` - `rfc-change-ok` yields a trailer per unit.
- `internal/le/verify/lifecycle.go` - write green and parts records at root after the publish; cut aggregation moved here (from `debtpart.go`), sweep `part all of <m>` moved with it.
- `internal/le/verify/engine/stages.go` - line 55 comment ("instead of taking a debt row").
- `internal/le/hookruntime/lifecycle.go` - `reportDebt` over the derivation; worktree silence.
- `internal/le/ai/hooks/parity.go`, `internal/le/completeness_record_test.go` - verb lists.
- `internal/le/doc/check/retired.go`, `internal/le/doc/citation/policed.go`, `internal/le/site/producer.go`, `internal/le/test/weakened/shard.go`, `internal/le/le/path/commitsession.go`, `internal/le/derived/derived.go`, `internal/le/setup/system.go` - remove the path and stale comments (`retired.go` keeps it as retired).
- `internal/le/commit/snapshot_test.go`, `publish_test.go`, `internal/le/doc/check/links_test.go` - as named in the test plan.
- `internal/test/fixture/misc_fixture_runner.go` - delete the two debt drivers, add the new ones.
- Docs and rules: see Documentation checklist.

## Files to Create
- `internal/le/commit/owed.go`, `owed_test.go` - the derivation and the moved answer producers.
- `internal/le/verify/green.go`, `green_test.go` - green record and cut aggregation.
- the four `.ci` files above.

## Files to Delete (no-layering: deleted before the replacement is written)
- `internal/le/commit/debt.go`, `debtpart.go`, `discharge.go` (after the producers named in Integration Points moved), `dischargerecord.go` and their tests.
- `plan/verification-debt/` (all shards and `discharged/`), via `remove`, in its own commit after the code lands (R-5), carrying the `Ledger-carried:` trailers (D-4).
- `test/runner/verify-scope-debt-clear.ci`, `test/runner/verify-scope-debt-discharge.ci`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | dev tooling |
| YANG validation constraints | N-A | dev tooling |
| YANG custom validators | N-A | dev tooling |
| CLI commands/flags | Yes | `internal/le/commit/actions.go`: verbs `debt-list`, `debt-clear`, `debt-discharge` removed, verb `push` added; keywords `push`, `unverified`, `missing-full-verify-ok` removed from `create`, `attest` added; `structural-red-ok`, `broken-head-fix`, `review-override`, `rfc-change-ok` kept |
| CLI grammar | Yes | every removed verb and keyword errors naming its replacement (AC-21, AC-24; `ai/rules/cli.md`) |
| Editor autocomplete | N-A | `./le` completion derives from the verb table |
| Functional test for new RPC/API | Yes | the four `test/runner/*.ci` |
| Pipe completeness | N-A | not ze CLI |
| Env var registration | N-A | `ZE_SKIP_SUITES` already exists; read, not added |
| Doctor check for runtime dependencies | N-A | new file under `tmp/` only, beside existing verify artifacts |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope changed? | N-A | dev tooling, no `features/` entry |
| 2 | Config syntax | N-A | none |
| 3 | CLI command changed | Yes | `docs/contributing/committing.md` (keyword table line 47 `push` row, debt paragraphs, push paragraph line 264, NOT-APPLICABLE paragraph line 309) |
| 4 | API/RPC | N-A | none |
| 5 | Plugin | N-A | none |
| 6 | Guide page | Yes | `docs/contributing/running-commands.md` lines 195, 209, 356; `docs/contributing/spec-workflow.md` line 144; `docs/guide/developer-setup.md` line 393 (`./le commit debt-clear`) |
| 7 | Wire format | N-A | none |
| 8 | Plugin SDK | N-A | none |
| 9 | RFC behavior | N-A | none |
| 10 | Test infrastructure | Yes | `docs/functional-tests.md` line 480; `docs/architecture/testing/test-health.md` line 148 |
| 11 | Daemon comparison | N-A | none |
| 12 | Internal architecture | Yes | `docs/architecture/testing/verify-freshness-scope.md` sections "Verification debt" and "Discharging a row no gate can run" rewritten; green record under "Certificate" |
| 13 | Route metadata | N-A | none |
| 14 | Prometheus | N-A | none |
| 15 | Inventory | Yes | `ai/INDEX.md` lines 330, 626; `.claude/hooks/README.md` lines 184-187; `ai/DOCS-TO-CODE.md` regenerated |
| 16 | Source anchors | Yes | `./le spec citation anchors spec plan/spec-verification-debt-derived.md` at implementation |
| 17 | Examples and rules | Yes | points: `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md` (and its entry in `ai/rules/points/completion/manifest.md` line 17 if renamed), `ai/rules/points/pre-release/directives/the-gate-is-owed-before-a-push-not-before-a-commit.md`, `ai/rules/points/planning/work-phases/the-commit-gate-that-needs-a-clean-review-artifact.md` (generates `ai/rules/planning.md` line 44), `ai/rules/points/git-safety/directives/push-only-on-the-owners-order.md`; then `./le ai rules condensed-update`. Hand-written: `AGENTS.md` and `ai/INSTRUCTIONS.md` push route (or their generator if generated); `continue.md`; `ai/skills/ze-rfc.md` and its generated `.claude/skills/ze-rfc/SKILL.md` |
| 18 | Design docs declared by changed files | Yes | `docs/contributing/rfc-implementation-guide.md` (declared by `input.go`): the `rfc-change-ok` paragraph and the approval-trailer section gain `RFC-approval-owed:` and `Owner-attested:`. `docs/architecture/core-design.md` and `docs/features/ai-first.md` (declared by `actions.go`, `prepare.go`, `script.go`, `review.go`): `ai-first.md` re-read for the review-override and push wording |

### Discovery (repo-maintenance)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` row "verification debt, gate owed, push refused" pointing at `internal/le/commit/owed.go` |
| Rule preventing regression | completion point rewritten: debt is derived, never written |
| Registry preventing drift | trailer table beside the keyword table; `retired.go` keeps `plan/verification-debt/` as a retired path so a doc citing it is refused |
| Verification proving it | AC-2 grep plus the four `.ci` |

## Implementation Steps

1. **Phase: Wiring** -- failing `.ci` and `owed_test.go` against a stub derivation.
2. **Phase: Green record** -- lifecycle writes it at root on green full unskipped uncut; move cut aggregation from `debtpart.go` into `internal/le/verify`. Tests AC-7..AC-10.
3. **Phase: Delete ledger code** -- remove `debt.go`, `debtpart.go`, `discharge*.go` (answer producers moved first), the verbs, the removed keywords, `renderPush`, the fixture drivers and their `.ci` (no-layering: before the derivation is wired in).
4. **Phase: Derivation and consumers** -- `owed.go`, `push` verb and push-only script, create report, hook, trailers in `Message`. Tests AC-1, AC-3..AC-6, AC-11..AC-25.
5. **Phase: Docs and rules** -- every row above, rules regenerated.
6. **Phase: Ledger deletion commit** -- `remove` of every file under `plan/verification-debt/`, separate commit, one `Ledger-carried:` trailer per open person-act row (AC-26).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC at file:line |
| Correctness | no path where "cannot derive" reads as zero owed; no skipped suite reaches a green record |
| Data flow | one derivation function, four consumers (push verb, push script, create, hook) |
| Rule: no-layering | no reader of `plan/verification-debt/` survives; no `create ... push` path survives beside the verb |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| ledger gone | `git ls-files plan/verification-debt` empty |
| no writer | grep `verification-debt` in `internal/le` matches only `retired.go` |
| push re-checks | the generated push script holds the HEAD check and the `debt-status` line before `git push` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Trailer forgery | a subject or body line spelling a declared trailer key is refused by `Message` |
| Attestation scope | `attest` refuses a commit outside `HEAD ^@{u}` or carrying no owed trailer |
| Push target | the script pushes the prepared SHA to the upstream branch only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | re-read Current Behavior |
| 3 fix attempts failed | STOP, report |

## Design Insights

- A rebase re-owes every local commit: `git pull --rebase` over a moved upstream gives HEAD a tree no verification saw, and G stops being an ancestor, so the display lists every local commit. This is stricter than the ledger, whose cleared rows stayed cleared across the rebase, and it is correct: the rebased tree is what the push publishes.
- The derivation fixes an under-count in `clearDebtRows` (`internal/le/commit/debt.go`): it clears every open row whose gate passed without comparing the row's commit with the commit the verification ran over, so a commit another session landed while `debt-clear` ran is cleared unverified. The tree rule compares the bytes pushed with the bytes verified and has no such window.
- `create ... push` cannot coexist with a tree gate: the commit it prepares moves the tree. Splitting the push into its own verb is what makes the gate satisfiable at all, and the re-check in the script is what makes the prepare-time answer still true at push time.
- The green record does not need copying: the lifecycle already holds the verdict at the asking root after the publish, so writing the record there removes the copy, the worktree-deletion hazard and the "missing file read as no certificate" ambiguity together.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Push rule is tree equality (D-1) | ancestry of G over every range commit | owner decision; one comparison, and the owed list is display only |
| Separate green record, written only on green full unskipped | read the certificate | the certificate is overwritten by red runs and its tree hash is a snapshot digest, not a git tree id |
| Green record written by the lifecycle at the asking root | engine writes it in the worktree and an optional copy publishes it | the review asked for its own optional copy; writing at root removes the copy entirely and keeps cut aggregation where `recordDebtPart` already runs |
| Push is its own verb with a script that makes no commit | keep `create ... push` | under D-1 a push behind a new commit is always refused (B1) |
| Push script re-derives and pushes the prepared SHA | bare `git push` | a commit or a lost green record between prepare and run must not ride along |
| Delete `unverified` and `missing-full-verify-ok` | keep them as no-op admissions | they admitted nothing `create` refuses; a keyword that does nothing is a layer (no-layering) |
| Owner attestation keyed by stable patch-id (D-3) | tree id, SHA | a commit's own tree changes when the upstream under it moves; the patch-id survives a clean rebase; the SHA survives nothing |
| Person-act answers derived (not-applicable, closed, reviewed, approved) plus `Owner-attested:` | attestation only | keeps every discharge kind that exists today, the reviewed kind included, with no record file |
| `Ledger-carried:` trailers are audit only (D-4) | carry them as gating `Review-owed:` / `RFC-approval-owed:` | the rows sit on commits already on `origin/main`; a gating trailer on the deletion commit would hold every push behind items no push can change, and the derived answers would run over the deletion commit instead of the commit that owed them. This reading of "carry" is stated for the owner to overturn |
| Runnable gates collapse to one question | keep per-gate rows | one green full verify clears all four today |
| Range for display is `HEAD ^@{u}` | whole history since G | D-2: pulled commits are never owed twice |

## Known Limitations
- Intermediate commits are not each verified: only the pushed tree is. A bisect can land on a red intermediate, as it can today after `debt-clear` at HEAD.
- The reviewed answer reads `tmp/review/`, which is machine-local: a commit reviewed on Linux reads owed on the Mac until it is attested or reviewed again there (safe direction).
- An owner attestation stops answering when a rebase conflict changes the commit's diff; the owner attests again.
- Only `./le verify worktree` writes the green record; a verify run in the live checkout (`./le verify current`) never does, because its tree is not a commit's tree.
- No open item from this spec is outstanding work; nothing is deferred.

## Open Questions for the Owner

None open. Q1 to Q4 and the push rule were answered on 2026-10-18 (Owner decisions table). The one reading taken in their absence, `Ledger-carried:` as audit only, is in Key Design Decisions.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/spec-verification-debt-derived.md` only
