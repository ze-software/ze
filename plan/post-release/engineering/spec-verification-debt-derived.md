# Spec: verification-debt-derived

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P2 |
| Release reason | The per-commit ledger refuses every push while any row is open, and over 2,000 are open, so nothing reaches origin until this lands. |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Owner permission (2026-10-23): this spec starts above the work-in-progress cap,
claimed at `ZE_SPEC_WIP_CAP=15` (14 in flight against a cap of 12), because the
ledger it deletes blocks every push. The exception covers this spec only.

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
| D-1 | Adopt the simpler push rule: a push needs HEAD's tree (`HEAD^{tree}`) to equal the tree recorded by the last green full verify; the list of owed commits is only for display | The gate compares two tree ids and nothing else for the verify half. The owed-commit list is computed for the message and never decides admission. Refined by D-6: the tree compared is the push target's, `P^{tree}` |
| D-2 (Q1) | Each machine re-verifies before it pushes; commits pulled from the other machine are never owed twice | The green record stays machine-local under `tmp/`. The display range is `HEAD ^@{u}`, so a commit that arrived through `origin` is in `@{u}` and never listed |
| D-3 (Q2) | An owner attestation becomes a commit trailer, keyed to something that survives a rebase (the tree or the patch-id, not the SHA) | `Owner-attested:` trailer keyed by the stable patch-id of the owed commit (see Person-act answers) |
| D-4 (Q3) | Drop the old verify rows; carry the open independent-review and RFC-approval rows as one trailer each on the ledger-deletion commit | 706 open verify rows at review time (753 at revision) are dropped. The person-act rows still open when the deletion commit is prepared each become one `Ledger-carried:` trailer on it (19 at review time, 26 on `./le commit debt-list` at revision time; the count is read when the deletion commit is prepared) |
| D-5 (Q4) | Docs-only commits still owe a verify (today's behavior) | No path filter: any tree change moves `HEAD^{tree}` away from the green tree |

### Owner decisions (2026-10-19, verbatim reply "I agree with all the suggestions")

The reply accepted every suggestion of the second review, so the review fixes
I2, I4, I5, N1 and N2 below are applied as decided, not as readings.

| # | Decision | What this spec does with it |
|---|----------|-----------------------------|
| D-6 | `./le commit push` pushes the exact verified commit, even after HEAD has moved on | Refines D-1. The push target P is chosen by the derivation (Push target, below): HEAD when `HEAD^{tree}` is the green tree, else the green record's commit when it is an ancestor of HEAD. The gate compares `P^{tree}` with the green tree. The push-only script runs `./le commit debt-status commit <P>`, which re-derives against that SHA over the range `<P> ^@{u}`, and pushes P. A commit landing after the script was prepared is not published and does not stop the push |
| D-7 | An owner attestation may be an EMPTY commit carrying only its trailer, so it does not change the tree and force a new full verify | `create ... attest` with no `file`/`remove` prepares an empty commit block; `Prepare` and the script accept it (Empty attestation commit, below). An attestation commit leaves `HEAD^{tree}` equal to the green tree, so P stays HEAD and the attestation is published with what it answers |
| D-8 | The 12 open RFC-approval rows are discharged by the owner | Review blocker B1 (unpushed commits whose RFC-approval rows would be silently dropped) no longer applies to them. The rule stays stated and enforced: a carried person-act row whose commit is in `HEAD ^@{u}` when the deletion commit is prepared gates the push until answered (`Ledger-carried:`, below) |
| I2 | A concurrent verify run can overwrite the published certificate between this run's publish and the green-record write | The green record is built from the run's own in-memory verification (`Report.Verify`) and `Report.Commit`, never from the certificate's verdict; the certificate is read only for its tree-moved marker, and the write is refused when its `git_sha` is not `Report.Commit` |
| I4 | The cut sweep that moves out of `debt-clear` needs a grammar, docs and tests | `./le verify worktree part <i> of <m>` and `./le verify worktree all of <m>` (Cut grammar, below); the piece rule is written over `Options.Part` / `Options.Parts` |
| I5 | AC-2 and the test plan disagreed on `links_test.go` line 290 | The `plan/verification-debt/` prefix leaves `citation.excludePrefixes` (it names a tree that no longer exists, and `retired.go` refuses any citation of it); the line 290 case is deleted with the behavior it tests and the test's ledger paragraph rewritten. AC-2 names `retired.go` as the only remaining match. `create ... append` onto a push-only script is refused (Push-only script, below) |
| N1 | `AGENTS.md` is generated | It is a copy of `ai/INSTRUCTIONS.md` written by `./le ai sync write` (`internal/le/ai/sync/ai.go`); edit `ai/INSTRUCTIONS.md` and regenerate, never edit `AGENTS.md` |
| N2 | Template sections that do not apply | RFC Summaries, End-to-End User Stories and RFC Documentation are N/A (Scope tooling; no RFC, no user-facing product path) |

### Owner decision (2026-10-23, committed as an owner rule in `ai/META-DECISIONS.md` by 764d6ca131)

| # | Decision | What this spec does with it |
|---|----------|-----------------------------|
| D-9 | "A commit that changes only documentation or prose owes no full gate, before a commit or before a push: the targeted checks for the files it touched are its evidence. A push gate compares the pushed tree with the last green tree ignoring such paths." | Replaces D-5 and refines D-1 and D-6. The tree rule becomes COVERAGE, not equality: the green tree covers a tree T when every path `git diff --name-only <green tree> <T>` names is a prose path. A prose path is Markdown under `ai/`, `plan/` or `docs/`, declared ONCE by marking those three entries of `nonGoPathRules` (`internal/le/repo/changed/selector.go`) and read through one exported predicate there; the push gate keeps no list of its own. Every other path, root-level Markdown (`AGENTS.md`, `CLAUDE.md`, `README.md`), the RFC corpus, `.ci`/`.et`/`.wb` bodies and every non-Markdown file under the three prefixes included, stays owed. An empty diff is covered. A diff that cannot be computed is NOT covered (never read as empty). The diff is computed per candidate, so its cost is one `git diff --name-only` per commit the push target walk visits |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - certificate, verification debt, discharge
  → Constraint: `WriteCertificate` (`internal/le/verify/engine/status.go`) overwrites `tmp/ze-verify.status` on EVERY run, red or green; on 2026-10-18 it held `exit=130` over `605de63e22`. So the last green verification is not recoverable from the certificate once a later run fails. The derivation needs its own record, written only for a green, full, unskipped run.
  → Constraint: the certificate's `tree_hash` is a `job.SnapshotTree` digest of the working tree (or `tree-moved-during-run`), NOT a git tree object id. The green record must store the git tree id of the verified commit (`<commit>^{tree}`) so it compares with `HEAD^{tree}`, and must refuse a run whose snapshot moved.
  → Constraint: `CheckCertificate` stales any non-zero exit and any mode `StagesForMode` does not know (`full-part-2-of-6`), so a cut piece never certifies. The cut aggregation today is `recordDebtPart` / `provenDebtParts` / `forgetDebtParts` (`internal/le/commit/debtpart.go`, record `tmp/ze-verify-debt-parts.json`), called from `clearDebt` (`internal/le/commit/actions.go`) with `root` = the ASKING checkout, after `verify.Run` returns. It must stay at that root when it moves: the engine runs inside the throwaway worktree, so aggregation there would be deleted with the worktree.
  → Constraint: `CopyCertificate` loops over `StatusPath` and `ManifestPath` and returns an error wrapping `os.ErrNotExist` for a missing file; `internal/le/verify/lifecycle.go` reads that error as "no certificate to publish". The green record must not be added to that loop: an absent green record is the normal outcome of a red run and must neither fail the publish nor be read as "no certificate".
  → Decision: the runnable gates (`unverified`, `structural-red-ok`, `missing-full-verify-ok`, `broken-head-fix`) all clear on the same event, one green full verify (`clearDebt` marks EVERY runnable gate passed on exit 0). They collapse to one derived question: does the green tree cover `HEAD^{tree}` (equal, or differing only in prose paths, D-9).
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
  → Constraint: all three name `./le commit create ... push "<owner authorisation>"` as the only push route. The `./le commit push` verb replaces that route, so each surface changes in the same piece of work.
  → Constraint: `AGENTS.md` is generated, a copy of `ai/INSTRUCTIONS.md` written by `./le ai sync write` (`internal/le/ai/sync/ai.go` lines 11-12, action `write` in `internal/le/ai/sync/actions.go`). Edit `ai/INSTRUCTIONS.md`, then run `./le ai sync write`.
- [ ] `internal/le/verify/actions.go`, `internal/le/verify/deps/actions.go`
  → Constraint: `./le verify worktree` takes only `commit <revision>` and `keep` today (`optionsFrom` builds `Options{Commit, Keep}`); `Options.Part`/`Parts` are reachable only through `debt-clear` (`debtVerifyOptions`). The cut grammar is therefore new on `verify worktree`. The precedent grammar is `partFrom` in `verify/deps/actions.go` (both keywords or neither, counting numbers, `Part.Valid`).
- [ ] `ai/META-DECISIONS.md` (handovers live in the repository)
  → Constraint: verification state is machine-local (`tmp/`); work moves between Linux and Mac. D-2 settles it: each machine re-verifies.

### RFC Summaries (Scope: protocol)
N/A: Scope tooling; no RFC is referenced (N2).

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
- [ ] `internal/le/commit/actions.go` - verbs `debt-list`, `debt-status`, `debt-clear`, `debt-discharge`; `clearDebt` / `clearDebtWith` (grammar from `debtPartFrom` in `debtpart.go`: `debt-clear part <n> of <m>` runs one piece, `debt-clear all of <m>` sweeps every piece, `all` with `part` refused; the sweep reads `provenDebtParts` and skips proven pieces, runs `hostReady` (`verify.Options.HostPreflight`) for every piece it will run before the first starts, carries on past a red piece; then `recordDebtPart` at `root`, `clearDebtRows`, `forgetDebtParts`); keyword table holds `push`, `unverified`, `structural-red-ok`, `missing-full-verify-ok`, `broken-head-fix`, `rfc-change-ok`; the create report line `debt=N row(s) -> <shard>`.
- [ ] `internal/le/commit/prepare.go` line 85 - `Prepare` refuses a call naming no `file` and no `remove` ("at least one file or remove path is required"), so an empty commit cannot be prepared today.
- [ ] `internal/le/commit/script.go` - `pushMarker`, `renderPush`, `splitPush` (one marker, final section; `composeScript` with `append` strips the push section and re-adds it after the new block), `commitBlock.ReviewCheck` re-check line, `PreparedHead` refusal when HEAD moved. Each block builds its tree in a private index, makes the commit with `git commit-tree`, and refuses when `git diff-tree` of parent and commit is empty ("HEAD already holds the prepared content of every path it names") or names a path the block does not declare (stray guard).
- [ ] `internal/le/verify/lifecycle.go` `Run` - after `verifyengine.RunPart` it sets `report.Verify = &verification` and `report.Code`; `report.Commit` is the SHA `resolveCommit` returned. Then `CopyCertificate(path, root)` publishes the certificate. `engine.Report` (`run.go`) carries `Mode` (`full`, or `full-part-<i>-of-<m>` from `part.go`), `Commit`, `Code`, `Completed`, `Failure` and `Attached` (non-nil when the run took another run's verdict and judged nothing).
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
- A push is refused while any obligation on the commits it publishes is open, and the refusal names the command that clears each kind.
- The session-start hook prints one warning line and at most five items, inside its budget, and says on stderr when it cannot answer (never reads "cannot answer" as zero).
- The cut verification (pieces dealt over one commit, all must pass over ONE commit and ONE cut, a sweep resumes proven pieces) stays available, because an uncut full verify is OOM-killed on a shared box.
- NOT-APPLICABLE outside the Ze checkout; the site and wiki can still push.

**Behavior to change:**
- No tracked debt file is written; no commit carries one.
- `plan/verification-debt/` (every shard and `discharged/`) is deleted.
- Verbs `debt-list`, `debt-clear`, `debt-discharge` are deleted; `debt-status` answers from the derivation.
- Keywords `unverified`, `missing-full-verify-ok` and `push` (on `create`) are deleted. A push goes through the new `./le commit push` verb, which prepares a script with no commit block and pushes the verified commit P (D-6), not the branch tip.
- The cut grammar moves from `debt-clear part <n> of <m>` / `debt-clear all of <m>` to `./le verify worktree part <i> of <m>` / `./le verify worktree all of <m>` (I4).
- `create ... attest` may prepare an empty commit (D-7).
- A rebase re-owes every local commit (see Design Insights).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le verify worktree` (uncut, or the piece that completes one cut, or a sweep `all of <m>`) exiting 0 in mode full with no skipped suite: the lifecycle writes the green record at the asking checkout's root.
- `./le commit push "<owner authorisation>"`: chooses the push target P, prepares a push-only script after the derivation admits P; the script re-derives against P immediately before `git push`.
- `./le commit create` (report), `./le commit debt-status [commit <rev>]`, session-start hook: read the derivation.

### Transformation Path
1. Verify lifecycle, at the ASKING root, after `CopyCertificate` returned nil. The verdict comes from the run's OWN in-memory result, never from the published certificate's verdict, because a concurrent run may have overwritten `tmp/ze-verify.status` between the copy and this read (I2). The run is green when `report.Verify` is non-nil, `report.Verify.Attached` is nil (an attached run judged nothing), `report.Code` is 0, `report.Verify.Completed` is true, `report.Verify.Mode` equals `Part.Name("full")` for the run's own `Options.part()` (`full` uncut or `1 of 1`, `full-part-<i>-of-<m>` for a cut piece; `part.go`), and `verifyengine.SkippedSuites()` is empty. The published certificate is then read for one fact only, whether the tree moved during the run (`tree_hash` = `tree-moved-during-run`); when its `git_sha` is not `report.Commit` another run wrote it, and the green record is NOT written (the run prints `verify-worktree: green record not written: certificate names <sha>, this run verified <sha>`). A green run that passes both:
   - uncut (`Options.Part == 0 && Options.Parts == 0`, or `part 1 of 1`, where `Part.Cut()` is false): write `tmp/ze-verify-green.status` (fields below) with `commit` = `report.Commit`, `tree` = `<report.Commit>^{tree}`, `cut` = `uncut`.
   - a piece (`Options.Part = i`, `Options.Parts = m`, `m > 1`, `Part.Valid`): add `i` to the parts record keyed by (`report.Commit`, `m`) (moved from `debtpart.go`, same flock; the record resets when the commit or `m` differs from the record's). When the record then holds every index 1..m, write the green record with `cut` = `m` and forget the parts record.
   - Anything else leaves both records untouched. Neither record is written inside the worktree, so nothing is copied back and `CopyCertificate` is unchanged.
2. Derivation (one function, new file `internal/le/commit/owed.go`), for a candidate push target (HEAD's choice by default, or the commit `debt-status commit <rev>` names):
   1. Not the Ze checkout: NOT-APPLICABLE.
   2. Resolve `@{u}`; none: answer "cannot derive: no upstream" (error, never zero).
   3. Read the green record; none, or `skipped` non-empty: verify-owed with "no green record".
   4. Push target P (Push target, below). No P: verify owed.
   5. Fast-forward: `@{u}` must be an ancestor of P, else owed with "the upstream moved: `git pull --rebase --no-autostash`, then `./le verify worktree`".
   6. Display list: the commits the push would leave behind or that block it, ONE `git rev-list` call: `HEAD ^P` when P exists (listed as "not in this push, owe a full verify"), else `HEAD ^@{u} ^G` (or `HEAD ^@{u}` with no green record or when G is not in the repository), formatted with short SHA and subject. When no P exists and the list is empty, the message says HEAD's tree is not the verified tree, never "0 owed".
   7. Person acts: one `git log` over `P ^@{u}` (over `HEAD ^@{u}` when no P exists, for display) reading the declared trailers; each owed item is open until an answer holds (table below). Answers are read over the same range, so an attestation committed after P is not counted for a push of P.
3. Consumers: `./le commit push` refuses when no P exists, when `P ^@{u}` is empty (nothing verified to push), or on any open item in `P ^@{u}`; the push-only script runs `./le commit debt-status commit <P>` and pushes P; `create` prints the counts; the hook prints the warning.

**Push target P (D-6):**

| Condition, in order | P | Note |
|---------------------|---|------|
| the green tree covers `HEAD^{tree}` (D-9) | HEAD | covers G itself, G followed by empty attestation commits (D-7), and G followed by prose-only commits |
| the green record's commit G is an ancestor of HEAD | the last commit X of the walk below | the commits `HEAD ^X` are named as left behind; they are not published |
| otherwise (no record, or G left the branch through a rebase) | none | verify owed, `./le verify worktree` named |

The walk: start at X = G, then follow `git rev-list --reverse --first-parent G..HEAD`, advancing X to each commit while the green tree covers that commit's tree, and stopping at the first commit it does not cover. So prose-only commits landed after G are published with it, and the first commit carrying anything else ends the walk.

With `debt-status commit <rev>` the candidate is `<rev>` itself: it is admissible only when the green tree covers `<rev>^{tree}` (D-9), `<rev>` is an ancestor of HEAD (a commit that left the branch is never published), and `@{u}` is an ancestor of `<rev>`.

The display list of owed commits (`create` report, session-start hook) names only commits in `HEAD ^X` that change a non-prose path; a prose-only commit is never listed as owing a full verify.

**Cut grammar (I4), on `./le verify worktree`, beside `commit <revision>` and `keep`:**

| Form | Runs | Refused when |
|------|------|--------------|
| neither keyword | the uncut population (`Options{Part: 0, Parts: 0}`) | - |
| `part <i> of <m>` | one piece, `Options{Part: i, Parts: m}` | one keyword without the other; `i` or `m` not a counting number; `i > m` (`Part.Valid`) |
| `all of <m>` | every piece of cut `m` over the one resolved commit, skipping indices the parts record already holds for that commit and `m`; `HostPreflight` runs for every piece it will run before the first starts, and a host that cannot run one starts none; a red piece does not stop the sweep, and the exit is non-zero when any piece was red | `all` with `part`; `all` without `of` |

The sweep resolves `commit <revision>` (default HEAD) once, so every piece runs over the same SHA even when HEAD moves during the sweep.

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
| `Owner-attested:` | `create ... attest <commit> "<owner words>"`, an empty commit (D-7) | stable patch-id of the named commit, the owner's words | yes (as an answer) |
| `RFC-approved:` | `create` from `./le rfc approve` (exists today) | unit | yes (as an answer) |
| `Structural-red-ok:` | `create ... structural-red-ok "<reason>"` | reason | no, audit only |
| `Broken-head-fix:` | `create ... broken-head-fix "<reason>"` | reason | no, audit only |
| `Ledger-carried:` | the ledger-deletion commit only (D-4) | original commit full SHA, its stable patch-id when the commit is in `HEAD ^@{u}` at preparation, gate key, reason | only when it carries a patch-id (D-8 rule, below); audit only otherwise |

**Person-act answers (derived at read time; no record is written for the first three):**

| Answer | `Review-owed:` | `RFC-approval-owed:` | Source producer kept |
|--------|----------------|----------------------|----------------------|
| not-applicable | the commit closes no spec | the commit changes no RFC-tagged unit | `closedSpecStem`, `changedTaggedUnits` |
| closed | the removed spec, read at the commit's parent, records Review Gate rows (or its Status is design or skeleton) | N-A | `verifyKindClosed`, `reviewGateRecorded`, `specStatus`, `removedSpecPath` |
| reviewed | a clean `tmp/review/<stem>-*.md` artifact whose hashes match the commit's own blobs for every code path (the `kindReviewed` derivation the first draft omitted) | N-A | `verifyKindReviewed`, `judgeReviewCoverage`, `committedText` |
| approved | N-A | an `RFC-approved:` trailer for the same unit on a commit in `HEAD ^@{u}` | `Message` trailer parse |
| owner | an `Owner-attested:` trailer in `HEAD ^@{u}` whose patch-id equals the owed commit's stable patch-id | same | new |

The reviewed answer reads a machine-local artifact keyed by blob content, so it survives a rebase that keeps the bytes and is simply absent on the other machine (owed there, the safe direction). The owner answer is keyed by `git patch-id --stable`, which a rebase keeps unless conflict resolution changes the diff, in which case the commit is owed again.

**Carried person-act rows (D-4, D-8 rule).** When the ledger-deletion commit is prepared, every open `review-override` or `rfc-change-ok` row becomes one `Ledger-carried:` trailer. A row whose commit is already in `@{u}` is published and no push can change it: its trailer carries no patch-id and is audit only. A row whose commit is in `HEAD ^@{u}` is not yet published, so dropping it would let its commit reach the upstream with the obligation unanswered (review blocker B1): its trailer carries the commit's stable patch-id, and the derivation reads it as an owed item of the row's gate kind whenever the deletion commit is in `P ^@{u}`. The item is matched to the commit in `P ^@{u}` with that patch-id and answered by the same table, evaluated over that commit (owner attestation by that patch-id). When no commit in the range has the patch-id (a rebase conflict changed its diff), the item stays open and names the carried short SHA; `Owner-attested:` naming the deletion commit answers every such unmatched item on it. On 2026-10-19 the owner discharged the 12 open RFC-approval rows (D-8), so they are not open when the deletion commit is prepared and carry nothing that gates; the rule binds whatever else is open then.

**Empty attestation commit (D-7).** `create ... attest <commit> "<owner words>"` with no `file` and no `remove` is the one call `Prepare` admits without a path: the line 85 refusal becomes "at least one file or remove path is required, unless the commit is an attestation". `attest` combined with `file` or `remove` is refused, naming why: an attestation that changes the tree moves `HEAD^{tree}` off the green tree and re-owes the verify it exists to spare. An empty block runs no source or verification gate (it changes no byte), stages nothing and snapshots no index. The script builds it with `git commit-tree -p <parent> <parent>^{tree}` and inverts the empty-diff refusal for that block only: an empty block whose `git diff-tree` names any path is refused as a stray, and an empty diff is its success. The block keeps `PreparedHead`, and may be appended after or before commit blocks like any other.

**Push-only script.** `./le commit push "<owner authorisation>"` writes the session's script (the same `script=` path `create` prints) holding no commit block: the generated-script marker (`scriptMarker`, so `append` and `replace` recognize it), the authorisation marker, then, in the Ze checkout, `./le commit debt-status commit <P>` whose non-zero exit refuses and pushes nothing, then `git push <remote of @{u}> <P>:<upstream branch ref>`. It does not check that HEAD is still P: under D-6 a commit landing after preparation is simply not published. It is refused when `debt-status commit <P>` fails, so a rebase that took P off the branch, a moved upstream or a lost green record each stop it. Outside the Ze checkout P is HEAD at preparation and the script carries no re-check line (AC-23). Interaction with the other script forms:

| Existing script at the path | `./le commit push` | `create` (no keyword) | `create ... append` | `create ... replace` |
|-----------------------------|--------------------|-----------------------|---------------------|----------------------|
| none | writes the push-only script | as today | as today | as today |
| commit blocks only | refused: run that script first, then verify and push (an unrun commit is neither verified nor published) | as today | as today | as today |
| push-only | overwrites it (P is chosen again) | refused as today for an existing script | refused: an appended commit would not be published by a script that pushes P, and the session would read it as pushed; run the push script first, or use `replace` | replaces it (it names no path, so `refuseForeignReplace` passes); the push is prepared again later with `./le commit push` |

`renderPush`, `splitPush` and the push section of `composeScript` are deleted: no script carries a commit block and a push together.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| verify worktree to checkout | certificate published by `CopyCertificate` as today; green record and parts record written by the lifecycle at the asking root | No |
| derivation to git | `git rev-parse` (`@{u}`, `HEAD^{tree}`, `P^{tree}`), `git merge-base --is-ancestor` (G to HEAD, `@{u}` to P), one `git rev-list` for display, one `git log` over `P ^@{u}` for trailers, `git patch-id --stable` per owed commit only when an attestation or a patch-id-keyed carried row is in range | No |
| push-only script to git | `./le commit debt-status commit <P>`, then `git push <remote> <P>:<upstream branch ref>` | No |

### Integration Points
- `verify/lifecycle.go` after `CopyCertificate` - writes the green record and the parts record at root, from `report.Verify` and `report.Commit` (I2).
- `verify/actions.go` `optionsFrom` - the `part`/`of`/`all` keywords on `verify worktree` (I4).
- `Message` (`internal/le/commit/input.go`) - writes the new trailers, the way it writes `RFC-approved:`; `refuseHandWrittenTrailer` refuses every declared key in subject and body.
- `closedSpecStem`, `changedTaggedUnits`, `verifyKindClosed`, `verifyKindReviewed`, `reviewGateRecorded`, `removedSpecPath`, `specStatus`, `committedText` move into `owed.go` (or a sibling) before `discharge.go` is deleted.
- `script.go` - push-only script rendering with the re-check; empty attestation block (D-7).

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
| R-7 | A commit lands between the push script's preparation and its run | HEAD differs from P | D-6: the script pushes P, never the moving branch tip, so the new commit is not published; `debt-status commit <P>` still requires P to be an ancestor of HEAD, so a rebase that took P off the branch refuses and pushes nothing |
| R-9 | A concurrent verify run overwrites `tmp/ze-verify.status` between this run's publish and its green-record write, and a green record names a verdict this run did not earn (I2) | certificate `git_sha` differs from `report.Commit` | the verdict is read from `report.Verify`, never from the certificate; the certificate is read only for the tree-moved marker, and the write is refused when its `git_sha` is not `report.Commit`; `TestGreenRecordIgnoresConcurrentCertificate` |
| R-10 | An appended commit rides behind a push-only script and is believed published | `create ... append` onto a push-only script | refused (Push-only script table); `TestAppendOntoPushOnlyScriptRefused` |
| R-11 | A carried person-act row on an unpushed commit is dropped by the ledger deletion (review B1) | open row whose commit is in `HEAD ^@{u}` | its `Ledger-carried:` trailer carries the patch-id and gates (D-8 rule); `TestCarriedRowOnUnpushedCommitGatesPush` |
| R-8 | `ZE_SKIP_SUITES` set in the environment admits a partial verification | green record with skipped suites | the record is not written when skipped is non-empty; a reader treats a non-empty skipped field as not green; `./le verify status write` never writes it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the push gate admits an unverified push (under-count) or refuses forever (over-count); nothing in the product |
| How is it reverted? | revert the code commit; the deleted shards return with the revert of the deletion commit |
| Who else touches this path? | every session committing (`commit create`), every push (site and wiki included), the session-start hook; `plan/post-release/engineering/spec-verify-scope-0-umbrella.md` (verify scope) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit push "<auth>"` | → | derivation refuses or prepares a push-only script | `test/runner/commit-push-refused-by-derived-debt.ci` |
| push-only script | → | `debt-status commit <P>` re-check before `git push` of P | `test/runner/commit-push-script-rechecks.ci` |
| `./le verify worktree` green over a commit | → | green record written at the asking root | `test/runner/verify-green-record-clears-derived-debt.ci` |
| `./le verify worktree part <i> of <m>` / `all of <m>` | → | cut grammar, parts record, green record on the completing piece | `test/runner/verify-worktree-cut-sweep.ci` |
| `./le commit create attest <c> "<words>"` with no file | → | empty attestation commit answers the owed item, push admitted | `test/runner/commit-attest-empty-commit.ci` |
| session-start hook | → | `reportDebt` over the derivation | `TestSessionStartReportsDerivedDebt` (`internal/le/hookruntime/lifecycle_test.go`) |
| `./le commit create review-override "<r>"` | → | `Review-owed:` trailer, push refused | `test/runner/commit-review-owed-trailer.ci` |

## End-to-End User Stories
N/A: Scope tooling (N2). The developer paths are covered by the Wiring Test table and the `.ci` tests.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le commit create` in the Ze checkout with a stale certificate | the generated script stages no file under `plan/verification-debt/`; the output says `debt=<n> commit(s) owe a full verify since <G or "no green record">` |
| AC-2 | `plan/verification-debt/` after the change lands | the directory does not exist in HEAD; `grep -rn 'plan/verification-debt' internal/le` (Go source and tests) matches only `internal/le/doc/check/retired.go`; the `citation.excludePrefixes` entry and the `links_test.go` line 290 case are gone with the behavior they tested |
| AC-3 | green full uncut `./le verify worktree` at HEAD, no open person-act trailer, then `./le commit push "<auth>"` and the printed script | the script is prepared with P = HEAD; running it passes `debt-status commit <P>` and pushes P to the upstream branch |
| AC-4 | one commit C made after the green record over G, then `./le commit push "<auth>"` | the script is prepared with P = G; the output names C by short SHA and subject as not in this push and owing `./le verify worktree`; running the script leaves `@{u}` = G |
| AC-5 | push script prepared at AC-3, then a commit lands before the script runs | the script still pushes P (the prepared SHA), and the new commit is not on the upstream afterwards |
| AC-6 | push script prepared at AC-3, then the green record is removed, or the branch rebased so P is no longer an ancestor of HEAD, before the script runs | `debt-status commit <P>` exits non-zero and the script pushes nothing |
| AC-7 | a red full verify after a green one | the green record still names the earlier tree; the gate answer is unchanged by the red run |
| AC-8 | `./le verify worktree part <i> of <m>` for every i over one commit, each green | the piece completing 1..m writes the green record (`cut` = m) at the asking root (not in the worktree); pieces over two commits or two values of m never write it |
| AC-9 | a cut piece green alone, a non-full mode green, any green run with `ZE_SKIP_SUITES` non-empty, a run whose tree moved | no green record written or changed |
| AC-10 | `./le verify status write exit-code 0 mode full` | writes the certificate as today; the green record is not written or changed |
| AC-11 | branch with no upstream in the main checkout | `./le commit push` refused and the hook warns on stderr, each naming the missing upstream and `git branch --set-upstream-to`; never reads as zero debt |
| AC-12 | linked worktree session, branch with no upstream | the hook prints nothing about debt; `./le commit push` still refuses |
| AC-13 | commit made with `review-override "<reason>"` | message carries `Review-owed: <stem>: <reason>`; push refused naming it until one answer holds: not-applicable, closed, reviewed (clean artifact over the commit's blobs), or `Owner-attested:` |
| AC-14 | commit made with `rfc-change-ok "<reason>"` | message carries one `RFC-approval-owed: <package>.<Test>: <reason>` per unit; push refused until not-applicable, a later `RFC-approved:` for the unit in range, or `Owner-attested:` |
| AC-15 | `create ... attest <commit> "<owner words>"` with no `file`/`remove` | an empty commit (tree = parent's tree) whose message carries `Owner-attested: <stable patch-id>: <owner words>`; the named commit must be in `HEAD ^@{u}` and carry an owed item, else refused naming why; after it lands `HEAD^{tree}` still equals the green tree, so `./le commit push` takes P = HEAD without a new verify |
| AC-16 | an attested commit rebased with no conflict | still answered; when conflict resolution changes its diff, owed again |
| AC-17 | `git pull --rebase` that moved the upstream, after a green verify | push refused; the list names every local commit (G is no longer an ancestor) |
| AC-18 | commits pulled from `origin` that the other machine made | never in the owed list (they are in `@{u}`) |
| AC-19 | a commit changing only Markdown under `docs/`, `ai/` or `plan/` after the green record (D-9) | not owed: `./le commit push` takes P = HEAD with no new verify, and the hook lists no owed commit. The same commit also touching `AGENTS.md`, a `.ci` body, an `rfc/` file or any Go file is owed, and P stops before it |
| AC-34 | the prose-path predicate (D-9) | answers true only for paths a `nonGoPathRules` entry marks as prose; a unit test asserts `docs/x.md`, `ai/rules/y.md`, `plan/z.md` are prose and `README.md`, `AGENTS.md`, `rfc/short/rfc4271.md`, `docs/x.png`, `test/runner/a.ci`, `internal/x/y.go` are not; the push gate holds no prose classification of its own: `internal/le/commit` decides prose only through `IsProse` (a prefix used for another purpose, such as the closure-stem journal rule in `review.go`, is not a prose list) |
| AC-35 | `git diff --name-only` fails during the walk | the commit is treated as NOT covered; the refusal names the git error; never read as an empty diff |
| AC-20 | `structural-red-ok` / `broken-head-fix` given | admission unchanged; reason recorded as a trailer; no obligation beyond the tree rule |
| AC-21 | `unverified`, `missing-full-verify-ok`, or `push` on `create` | unknown keyword error naming the replacement (`./le verify worktree` for the first two, `./le commit push "<owner authorisation>"` for the third) |
| AC-22 | session start with HEAD's tree not the green tree | one line `verification debt: N commit(s) owe a full verify since <G>, push refused until cleared` plus up to five items; nothing when the trees match and no person act is open |
| AC-23 | `ZE_REPO_ROOT` at a non-Ze tree | `verify=NOT-APPLICABLE`, no derivation; `./le commit push` prepares the push-only script with no re-check line |
| AC-24 | `./le commit debt-list`, `debt-clear`, `debt-discharge` | unknown verb error naming the replacement (`./le commit debt-status`, `./le verify worktree`, `create ... attest`) |
| AC-25 | `./le commit debt-status` | exit 0 when P is HEAD and nothing is owed, 1 when anything is owed (listing it, P included), 2 when the derivation cannot answer. `debt-status commit <rev>`: exit 0 when `<rev>` is an admissible push target with no open item in `<rev> ^@{u}`, whatever follows it on the branch; 1 otherwise, naming why; 2 when it cannot answer |
| AC-26 | the ledger-deletion commit | carries one `Ledger-carried:` trailer per person-act row still open when it is prepared; a row on a commit in `@{u}` carries no patch-id and is not read by the gate; a row on a commit in `HEAD ^@{u}` carries its patch-id and holds the push until answered (D-8 rule) |
| AC-27 | `create ... attest <c> "<w>" file <path>` | refused, naming that an attestation must not change the tree |
| AC-28 | the empty attestation block in the script | built on the parent's tree; refused when `git diff-tree` names any path; lands when the diff is empty |
| AC-29 | `create ... append` onto a push-only script; `./le commit push` onto a script holding commit blocks | each refused with the reason and command in the Push-only script table; `create ... replace` onto a push-only script replaces it |
| AC-30 | a concurrent run rewrote `tmp/ze-verify.status` with another commit's certificate before this green run's record write | no green record written; the run prints the two SHAs |
| AC-31 | a run that attached to another run's verdict (`Attached` non-nil) with exit 0 | no green record written |
| AC-32 | `./le verify worktree all of <m>` with pieces already proven over the commit | runs only the unproven pieces, preflights each before the first, carries on past a red piece, writes the green record when 1..m are proven; `all part 2 of 3`, `all` without `of`, `part 2` without `of`, `part 0 of 3`, `part 4 of 3` refused naming the form |
| AC-33 | `./le commit push` when `@{u}` is not an ancestor of P, or `P ^@{u}` is empty | refused: the first names `git pull --rebase --no-autostash` then `./le verify worktree`; the second says nothing verified is ahead of the upstream |

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
| `TestDerivedDebtProseOnlyCommitIsCovered` (replaces `TestDerivedDebtDocsOnlyCommitOwes`, D-9) | same | AC-19 | |
| `TestDerivedDebtReadsPersonActTrailers` | same | AC-13, AC-14 (each answer kind, the reviewed kind included) | |
| `TestOwnerAttestationSurvivesRebase` | same | AC-15, AC-16, A-6 | |
| `TestDebtStatusExitCodes` | `internal/le/commit/actions_test.go` | AC-25 | |
| `TestPushScriptPushesPreparedSHAToUpstream` | `internal/le/commit/script_test.go` | AC-3, A-2 | |
| `TestPushTargetIsGreenCommitWhenHeadMovedOn` | `internal/le/commit/owed_test.go` | AC-4, D-6 | |
| `TestPushScriptPushesPreparedCommitAfterHeadMoved` | `internal/le/commit/script_test.go` | AC-5, R-7 | |
| `TestDebtStatusCommitRefusesCommitOffBranch` | `internal/le/commit/owed_test.go` | AC-6, AC-25 | |
| `TestPushRefusedWhenUpstreamMovedOrNothingAhead` | same | AC-33 | |
| `TestAttestPreparesEmptyCommit` / `TestAttestWithFileRefused` | `internal/le/commit/commit_test.go` | AC-15, AC-27, D-7 | |
| `TestEmptyAttestationBlockRefusesAnyPath` | `internal/le/commit/script_test.go` | AC-28 | |
| `TestAppendOntoPushOnlyScriptRefused` / `TestPushOntoCommitScriptRefused` | `internal/le/commit/commit_test.go` | AC-29, R-10 | |
| `TestCarriedRowOnUnpushedCommitGatesPush` | `internal/le/commit/owed_test.go` | AC-26, R-11, D-8 | |
| `TestGreenRecordSurvivesRedRun` | `internal/le/verify/green_test.go` | AC-7 | |
| `TestGreenRecordOnlyForFullUnskippedUncutOrCompletedCut` | same | AC-8, AC-9, piece rule over `Options.Part`/`Parts` (moved from `TestOnePieceOfTheVerificationClearsNoRow`, `TestAHeadMoveDiscardsThePiecesProvenBeforeIt`, `TestADifferentCutStartsTheRecordAgain` in `debtpart_test.go`) | |
| `TestGreenRecordIgnoresConcurrentCertificate` | same | AC-30, R-9, I2 | |
| `TestAttachedRunWritesNoGreenRecord` | same | AC-31 | |
| `TestGreenRecordWrittenAtAskingRoot` | `internal/le/verify/lifecycle_test.go` | AC-8, R-4 | |
| `TestWorktreeCutKeywordsTakeBothOrNeither` | `internal/le/verify/actions_test.go` | AC-32 grammar (moved from `TestThePartKeywordsTakeBothOrNeither`) | |
| `TestWorktreeSweepProvesEveryPiece`, `TestWorktreeSweepSkipsProvenPiece`, `TestWorktreeSweepCarriesOnPastRedPiece`, `TestWorktreeSweepOnUnreadyHostStartsNoPiece` | `internal/le/verify/sweep_test.go` | AC-32 (moved from the four `TestASweep*` in `debtpart_test.go`, assertions kept) | |
| `TestStatusWriteNeverWritesGreenRecord` | `internal/le/verify/status/answer_test.go` | AC-10 | |
| `TestMessageWritesOwedTrailers` / `TestMessageRefusesDeclaredTrailerInBody` | `internal/le/commit/input_test.go` | AC-13, AC-14, AC-15, R-6 | |
| `TestCreateStagesNoDebtFile` | `internal/le/commit/commit_test.go` | AC-1 | |
| `TestRemovedKeywordsNameReplacement` | same | AC-21 | |
| `TestSessionStartReportsDerivedDebt` | `internal/le/hookruntime/lifecycle_test.go` | AC-22, AC-11, AC-12, A-5 | |

Deleted with the code they test (no assertion weakened, the behavior is removed): `debtpart_test.go` (moved, see above), `discharge_test.go`, `discharge_rfc_test.go`, `ledger_test.go`, debt tests in `commit_test.go` (`TestDebtRowsDeduplicateOnlyWhileOpen`, `TestCreateRefusesBadTagBeforeRecordingDebt` adapted to its remaining refusal, `TestDebtClearingHonorsTheGateExit`, `TestDebtClearingReportsRedVerificationAsFailure`, `TestPushProceedsWhenEveryRemainingRowIsDischarged` replaced by the AC-3 test), `TestSessionHookCountsNoDischargedRowAsOwed`. Comments that describe the ledger in surviving tests are rewritten: `snapshot_test.go` line 60, `publish_test.go` line 360. The `links_test.go` line 290 case (`plan/verification-debt/20476e05.md`, expects `citation.Excluded` true) tests the `plan/verification-debt/` entry of `excludePrefixes` in `internal/le/doc/citation/policed.go` line 73. That entry is deleted, because it excludes a tree that no longer exists and `retired.go` already refuses any citation of the path, so the case is deleted with the behavior it tests and the test's comment paragraph on ledger shards ("The ledgers close the same way...") is rewritten to name only the trees still excluded. The other six cases stay unchanged (AC-2).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `./le verify worktree part <i> of <m>` | 1..m | i=m | 0 | m+1 (refused, as `debt-clear` refuses today) |
| `./le verify worktree all of <m>` / `of <m>` | m >= 1 | m=1 (one piece, the uncut population, `Part.Cut()` false) | 0 (`Part.Valid` refuses) | none |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `commit-push-refused-by-derived-debt` | `test/runner/*.ci` | agent commits after a green, push refused, verify, push prepared | |
| `commit-push-script-rechecks` | `test/runner/*.ci` | push script prepared, a commit lands, script pushes P only; green record removed, script refuses | |
| `verify-worktree-cut-sweep` | `test/runner/*.ci` | `./le verify worktree all of <m>` over a fixture runner proves every piece and writes the green record; a second sweep skips proven pieces | |
| `commit-attest-empty-commit` | `test/runner/*.ci` | `review-override` commit, push refused, `create ... attest` with no file, push admitted with P = HEAD and no new verify | |
| `verify-green-record-clears-derived-debt` | `test/runner/*.ci` | green verify over HEAD empties the derived set | |
| `commit-review-owed-trailer` | `test/runner/*.ci` | review-override closure carries the trailer and holds the push until attested | |
| replaces `verify-scope-debt-clear.ci`, `verify-scope-debt-discharge.ci` | `test/runner/` | deleted with their fixture drivers in `internal/test/fixture/misc_fixture_runner.go` | |
| `le-rfc-ledger-is-derived.ci` | `test/runner/` | comment line 6 updated; assertion unchanged | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## RFC Documentation (Scope: protocol)
N/A: Scope tooling; no protocol code (N2).

## Files to Modify
- `internal/le/commit/prepare.go` - drop `recordDebt` call and shard staging; drop the push path from `Prepare`; trailers instead of rows; the line 85 refusal admits an `attest` call with no path, and `attest` with a path is refused (D-7); `append` onto a push-only script refused (I5).
- `internal/le/verify/actions.go` - `part`, `of`, `all` keywords on `worktree`, parsed into `Options.Part`/`Parts` and the sweep (I4); the action's `Why` names them.
- `internal/le/commit/actions.go` - delete `debt-list`, `debt-clear`, `debt-discharge` and the `push`, `unverified`, `missing-full-verify-ok` keywords; add the `push` verb and the `attest` keyword; `debt-status` from the derivation with exit codes 0/1/2; report line; trailer table.
- `internal/le/commit/script.go` - push-only script: prepared SHA check, `./le commit debt-status` re-check (Ze checkout only), push of the prepared SHA to the upstream; `renderPush`/`splitPush` replaced, not kept beside it.
- `internal/le/commit/input.go` - trailers in `Message`; `refuseHandWrittenTrailer` over the table.
- `internal/le/commit/verification.go` - line 1 Design comment; green record reader if not in verify.
- `internal/le/commit/rfcchange.go` - `rfc-change-ok` yields a trailer per unit.
- `internal/le/verify/lifecycle.go` - write green and parts records at root after the publish, from `report.Verify` and `report.Commit`, refused when the certificate's `git_sha` is not `report.Commit` (I2); cut aggregation moved here (from `debtpart.go`); the sweep (`clearDebtWith` today) moves to a new `internal/le/verify/sweep.go` driven by `all of <m>`.
- `internal/le/verify/engine/stages.go` - line 55 comment ("instead of taking a debt row").
- `internal/le/hookruntime/lifecycle.go` - `reportDebt` over the derivation; worktree silence.
- `internal/le/ai/hooks/parity.go`, `internal/le/completeness_record_test.go` - verb lists.
- `internal/le/doc/check/retired.go`, `internal/le/doc/citation/policed.go` (the `excludePrefixes` entry at line 73 deleted, I5), `internal/le/site/producer.go`, `internal/le/test/weakened/shard.go`, `internal/le/le/path/commitsession.go`, `internal/le/derived/derived.go`, `internal/le/setup/system.go` - remove the path and stale comments (`retired.go` keeps it as retired).
- `internal/le/commit/snapshot_test.go`, `publish_test.go`, `internal/le/doc/check/links_test.go` - as named in the test plan.
- `internal/test/fixture/misc_fixture_runner.go` - delete the two debt drivers, add the new ones.
- Docs and rules: see Documentation checklist.

## Files to Create
- `internal/le/commit/owed.go`, `owed_test.go` - the derivation and the moved answer producers.
- `internal/le/verify/green.go`, `green_test.go` - green record and cut aggregation.
- `internal/le/verify/sweep.go`, `sweep_test.go` - the `all of <m>` sweep moved from `clearDebtWith`.
- the six `.ci` files above.

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
| CLI commands/flags | Yes | `internal/le/commit/actions.go`: verbs `debt-list`, `debt-clear`, `debt-discharge` removed, verb `push` added; keywords `push`, `unverified`, `missing-full-verify-ok` removed from `create`, `attest` added; `debt-status` gains `commit <rev>`; `structural-red-ok`, `broken-head-fix`, `review-override`, `rfc-change-ok` kept. `internal/le/verify/actions.go`: `worktree` gains `part <i> of <m>` and `all of <m>` |
| CLI grammar | Yes | every removed verb and keyword errors naming its replacement (AC-21, AC-24; `ai/rules/cli.md`); the cut forms and their refusals (AC-32); `attest` with a path (AC-27); `append`/`push` onto the wrong script (AC-29) |
| Editor autocomplete | N-A | `./le` completion derives from the verb table |
| Functional test for new RPC/API | Yes | the six new `test/runner/*.ci` |
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
| 6 | Guide page | Yes | `docs/contributing/running-commands.md` lines 195, 209, 356 (the `debt-clear part`/`all` forms become `./le verify worktree part <i> of <m>` / `all of <m>`, with the resume and preflight behavior); `docs/contributing/spec-workflow.md` line 144; `docs/guide/developer-setup.md` line 393 (`./le commit debt-clear`) |
| 7 | Wire format | N-A | none |
| 8 | Plugin SDK | N-A | none |
| 9 | RFC behavior (RFC Documentation) | N-A | Scope tooling: no RFC is implemented or changed (N2) |
| 10 | Test infrastructure | Yes | `docs/functional-tests.md` line 480; `docs/architecture/testing/test-health.md` line 148 |
| 11 | Daemon comparison | N-A | none |
| 12 | Internal architecture | Yes | `docs/architecture/testing/verify-freshness-scope.md` sections "Verification debt" and "Discharging a row no gate can run" rewritten (push target P, empty attestation commit, carried rows); green record under "Certificate", with the concurrent-run refusal; the cut grammar of `verify worktree` and the sweep |
| 13 | Route metadata | N-A | none |
| 14 | Prometheus | N-A | none |
| 15 | Inventory | Yes | `ai/INDEX.md` lines 330, 626; `.claude/hooks/README.md` lines 184-187; `ai/DOCS-TO-CODE.md` regenerated |
| 16 | Source anchors | Yes | `./le spec citation anchors spec plan/post-release/engineering/spec-verification-debt-derived.md` at implementation |
| 17 | Examples and rules | Yes | points: `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md` (and its entry in `ai/rules/points/completion/manifest.md` line 17 if renamed), `ai/rules/points/pre-release/directives/the-gate-is-owed-before-a-push-not-before-a-commit.md`, `ai/rules/points/planning/work-phases/the-commit-gate-that-needs-a-clean-review-artifact.md` (generates `ai/rules/planning.md` line 44), `ai/rules/points/git-safety/directives/push-only-on-the-owners-order.md`; then `./le ai rules condensed-update`. `ai/INSTRUCTIONS.md` push route, then `./le ai sync write` regenerates `AGENTS.md` from it (N1; never edit `AGENTS.md` by hand). Hand-written: `continue.md`; `ai/skills/ze-rfc.md` and its generated `.claude/skills/ze-rfc/SKILL.md` |
| 18 | Design docs declared by changed files | Yes | `docs/contributing/rfc-implementation-guide.md` (declared by `input.go`): the `rfc-change-ok` paragraph and the approval-trailer section gain `RFC-approval-owed:` and `Owner-attested:`. `docs/architecture/core-design.md` and `docs/features/ai-first.md` (declared by `actions.go`, `prepare.go`, `script.go`, `review.go`): `ai-first.md` re-read for the review-override and push wording |

### Discovery (repo-maintenance)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` row "verification debt, gate owed, push refused" pointing at `internal/le/commit/owed.go` |
| Rule preventing regression | completion point rewritten: debt is derived, never written |
| Registry preventing drift | trailer table beside the keyword table; `retired.go` keeps `plan/verification-debt/` as a retired path so a doc citing it is refused |
| Verification proving it | AC-2 grep plus the six new `.ci` |

## Implementation Steps

1. **Phase: Wiring** -- failing `.ci` and `owed_test.go` against a stub derivation.
2. **Phase: Green record and cut grammar** -- lifecycle writes it at root from the run's own verification (I2); move cut aggregation from `debtpart.go` and the sweep from `clearDebtWith` into `internal/le/verify`, with the `part`/`all` grammar on `verify worktree` and its docs (I4). Tests AC-7..AC-10, AC-30..AC-32.
3. **Phase: Delete ledger code** -- remove `debt.go`, `debtpart.go`, `discharge*.go` (answer producers moved first), the verbs, the removed keywords, `renderPush`, the fixture drivers and their `.ci` (no-layering: before the derivation is wired in).
4. **Phase: Derivation and consumers** -- `owed.go` with the push target P, `push` verb and push-only script, `debt-status commit <rev>`, empty attestation commit, create report, hook, trailers in `Message`. Tests AC-1, AC-3..AC-6, AC-11..AC-25, AC-27..AC-29, AC-33.
5. **Phase: Docs and rules** -- every row above, rules regenerated.
6. **Phase: Ledger deletion commit** -- `remove` of every file under `plan/verification-debt/`, separate commit, one `Ledger-carried:` trailer per open person-act row; rows on commits in `HEAD ^@{u}` carry the patch-id and gate (AC-26, D-8 rule). The 12 RFC-approval rows the owner discharged on 2026-10-19 are not open by then.

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
| push re-checks | the generated push script holds the `debt-status commit <P>` line before `git push <remote> <P>:<branch ref>` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Trailer forgery | a subject or body line spelling a declared trailer key is refused by `Message` |
| Attestation scope | `attest` refuses a commit outside `HEAD ^@{u}` or carrying no owed trailer |
| Push target | the script pushes P, a commit on HEAD's branch that the green record covers, to the upstream branch only |
| Empty attestation | an empty block whose diff names any path is refused; `attest` with a path is refused |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | re-read Current Behavior |
| 3 fix attempts failed | STOP, report |

## Design Insights

- A rebase re-owes every local commit: `git pull --rebase` over a moved upstream gives HEAD a tree no verification saw, and G stops being an ancestor, so the display lists every local commit. This is stricter than the ledger, whose cleared rows stayed cleared across the rebase, and it is correct: the rebased tree is what the push publishes.
- The derivation fixes an under-count in `clearDebtRows` (`internal/le/commit/debt.go`): it clears every open row whose gate passed without comparing the row's commit with the commit the verification ran over, so a commit another session landed while `debt-clear` ran is cleared unverified. The tree rule compares the bytes pushed with the bytes verified and has no such window.
- `create ... push` cannot coexist with a tree gate: the commit it prepares moves the tree. Splitting the push into its own verb is what makes the gate satisfiable at all, and the re-check in the script is what makes the prepare-time answer still true at push time.
- Pushing P rather than HEAD (D-6) separates "what is verified" from "where the branch is". Sessions keep committing while a verify runs; under a HEAD-only rule every such commit would void the verify. With P, the verified commit publishes and the later commits wait for the next verify, so no session's commit stalls another's push.
- An attestation is evidence about a commit, not a change to the product, so an empty commit is its natural carrier (D-7): it lands in history, travels with the push, and leaves the tree the verify judged untouched.
- The green record does not need copying: the lifecycle already holds the verdict at the asking root after the publish, so writing the record there removes the copy, the worktree-deletion hazard and the "missing file read as no certificate" ambiguity together.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Push rule is tree equality (D-1) | ancestry of G over every range commit | owner decision; one comparison, and the owed list is display only |
| Separate green record, written only on green full unskipped | read the certificate | the certificate is overwritten by red runs and its tree hash is a snapshot digest, not a git tree id |
| Green record written by the lifecycle at the asking root | engine writes it in the worktree and an optional copy publishes it | the review asked for its own optional copy; writing at root removes the copy entirely and keeps cut aggregation where `recordDebtPart` already runs |
| Push is its own verb with a script that makes no commit | keep `create ... push` | under D-1 a push behind a new commit is always refused (B1) |
| Push script re-derives against P and pushes P (D-6) | bare `git push`; refuse when HEAD moved | a commit landing between prepare and run must not ride along and must not void the push; a lost green record or a P taken off the branch must stop it |
| Push target P: HEAD when its tree is green, else the green commit G when it is HEAD's ancestor (D-6) | HEAD only | owner decision 2026-10-19 |
| Attestation commits are always empty (D-7) | allow an attestation with files | the owner allowed empty attestations; requiring it keeps one statement ("an attestation never moves the tree") and stops a file riding on an attestation from re-owing the verify. Stated for the owner to overturn |
| Green record verdict from `report.Verify` and `report.Commit`; certificate read only for tree-moved, refused on a foreign `git_sha` (I2) | read the published certificate | a concurrent run can rewrite the certificate between the copy and the read |
| Cut grammar on `verify worktree` (`part <i> of <m>`, `all of <m>`), the sweep resolving its commit once (I4) | keep `debt-clear` as the cut entry; re-resolve HEAD per piece as `clearDebtWith` does | `debt-clear` is deleted; resolving once keeps every piece over one commit, which is what the parts record requires anyway |
| `create ... append` onto a push-only script refused (I5) | insert the commit before the push | the push publishes P, so an appended commit would read as pushed and not be |
| Delete `unverified` and `missing-full-verify-ok` | keep them as no-op admissions | they admitted nothing `create` refuses; a keyword that does nothing is a layer (no-layering) |
| Owner attestation keyed by stable patch-id (D-3) | tree id, SHA | a commit's own tree changes when the upstream under it moves; the patch-id survives a clean rebase; the SHA survives nothing |
| Person-act answers derived (not-applicable, closed, reviewed, approved) plus `Owner-attested:` | attestation only | keeps every discharge kind that exists today, the reviewed kind included, with no record file |
| `Ledger-carried:` trailers are audit only for rows on commits in `@{u}`, and gate (keyed by patch-id, answered over the owing commit) for rows on commits in `HEAD ^@{u}` (D-4, D-8) | all audit only; all gating | a row on a published commit can be changed by no push; a row on an unpublished commit dropped silently is review blocker B1. Owner accepted 2026-10-19 |
| Runnable gates collapse to one question | keep per-gate rows | one green full verify clears all four today |
| Range for display is `HEAD ^@{u}` | whole history since G | D-2: pulled commits are never owed twice |

## Known Limitations
- Intermediate commits are not each verified: only the pushed tree is. A bisect can land on a red intermediate, as it can today after `debt-clear` at HEAD.
- The reviewed answer reads `tmp/review/`, which is machine-local: a commit reviewed on Linux reads owed on the Mac until it is attested or reviewed again there (safe direction).
- An owner attestation stops answering when a rebase conflict changes the commit's diff; the owner attests again.
- Only `./le verify worktree` writes the green record; a verify run in the live checkout (`./le verify current`) never does, because its tree is not a commit's tree.
- A push of P = G leaves the commits after G unpublished until the next green verify covers them (D-6); the push output names them.
- No open item from this spec is outstanding work; nothing is deferred.

## Open Questions for the Owner

None open. Q1 to Q4 and the push rule were answered on 2026-10-18; the push target, empty attestations, the carried-row rule and every second-review fix were accepted on 2026-10-19 ("I agree with all the suggestions"). Both Owner decisions tables hold them. The one reading still stated for the owner to overturn is that an attestation commit is always empty (Key Design Decisions).

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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-verification-debt-derived.md` only
