# Spec: verification-debt discharge

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The verification-debt ledger cannot reach zero, so `./le commit create ... push` is
refused permanently.

`refusePushWithDebt` (`internal/le/commit/prepare.go`) refuses an authorized push
while ANY row in `plan/verification-debt/` is open. `clearDebtWith`
(`internal/le/commit/actions.go`) diverts two gate names into `unrunnableSet`
before a runner is chosen, and `passed` is only ever filled from
`result.Runnable`, so no branch in the repository can write `cleared` for a row
naming `independent critical review` or `owner approval for an RFC-tagged test
change`. Measured 2026-09-07 over 208 shards: 3526 rows, 3245 open, 281 cleared,
of which 68 name the review gate and 4 the owner-approval gate. Those 72 are
permanent by construction.

The obligation behind most of the 72 is not outstanding. It was discharged (the
owner ordered the commit, or an independent pass ran after it), or it never
applied (no spec closes in that commit, so no review was owed), or it is not yet
due (the spec is still open and reviews at its own Review Gate). The ledger has
no way to record any of those three facts: `Debt` (`internal/le/commit/debt.go`)
carries Shard, Line, Date, Session, Subject, Gate, Reason, Status and nothing
else, and `clearDebtRows` selects rows by gate NAME alone.

The goal is that every row has a route to a non-open state, that the route
RE-DERIVES what a machine can re-derive rather than trusting the operator's
word, and that a discharge nobody can re-derive is distinguishable in the ledger
from one that was.

Two documentation defects in `docs/architecture/testing/verify-freshness-scope.md`
describe this same code and are wrong today. The change makes that page wrong
anyway, so both are corrected here (`ai/rules/documentation.md`).

### Why the 68 review rows cannot clear (measured 2026-09-07, every reason cell read)

| Why the reviewer was bypassed | Rows | Route this spec gives it |
|---|---|---|
| No spec closes in that commit at all (journal row, rule change, docs) | 26 | `kind not-applicable`, re-derived from the commit |
| Owner ordered the commit anyway | 12 | `kind owner`, attested |
| The spec was open at commit time and has CLOSED since, with a recorded Review Gate | 10 | `kind closed`, re-derived from the closure commit |
| Another session's rows swept into a shared journal class file | 9 | `kind not-applicable`, re-derived from the commit |
| An independent review ran, or was scheduled, against that commit | 6 | `kind not-applicable` where that commit closes no spec, `kind reviewed` otherwise |
| The owner was himself the reviewer | 5 | `kind owner`, attested |

The 68 come from 12 commit sessions; 58 are dated 2026-08-18, 2026-08-19 or
2026-08-20. The 4 owner-approval rows take `kind owner` or, where the commit
changed no tagged unit, `kind not-applicable`.

The 10 rows in the third row of that table were traced to their specs on
2026-09-07, and every one of those specs is ALREADY CLOSED. Eight closed with a
Review Gate carrying an artifact reference, a rounds count and a findings loop
(`fixit-firewall-irr-term-fails-validation` at `47bbb259d3` found 1 BLOCKER and
2 ISSUEs and fixed them; `fixit-zefs-diff-structural-ops` at `3d06ae4c04` found
2 BLOCKERs; `finish-ci-coverage` at `d95ef465ff` pinned 17 files clean). Two
closed at `skeleton`, never implemented, with their defects verified gone at the
producer. So the obligation those rows record is not merely "not yet due": it
has been DISCHARGED, and the discharge is recoverable.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the page that declares debt behavior; `debt.go` names it in its `// Design:` header
  → Constraint: the page's "Verification debt" section is the canonical description and it is WRONG in two places today; the corrections are AC-18, and they land in the same work as the code
  → Decision: the page already states the intended answer for the two unrunnable gates ("answered by doing the work the row names ... recorded through `internal/le/spec/session/review.go`"), so the doc is the side that is right and the code is the side that is missing a verb
- [ ] `docs/contributing/committing.md` - the operator-facing commit route
  → Constraint: it states that the command refuses a push while any debt row is open; that sentence stays true and gains the discharge route
- [ ] `ai/rules/cli.md` - closed keywords, structured payload, all pipe operators
  → Constraint: the first token after the verb is a keyword from a closed set; `parseKeywords` (`internal/le/commit/actions.go`) is the existing enforcement and the new verb uses it
  → Constraint: the answer is structured data with kebab-case JSON keys and a `Text()` rendering, so `| json`, `| yaml` and `| table` each render the same payload
- [ ] `ai/patterns/cli-command.md` - structural template
  → Decision: `le` areas are offline commands; a verb is one row in `commandVerbs` plus one case in `Answer`, and nothing else registers
- [ ] `ai/rules/principles.md` - fail closed, declare once, derive
  → Constraint: an unrecognized gate name must not fall through to "runnable"; a discharge record must not be a cached verdict a reader trusts
- [ ] `ai/rules/no-layering.md` - replace, never keep both
  → Constraint: the two hardcoded gate-name literals in `clearDebtWith` are DELETED when the `debtGates` table answers the question; no fallback to the literals

### RFC Summaries (Scope: protocol)
- N-A. Scope is tooling: no wire protocol, no RFC obligation.

**Key insights:**
- `Debt` has no SHA, no spec stem and no file list; every discharge therefore needs the operator to NAME the commit, and the verb re-derives from that commit rather than from the row.
- `ReviewArtifactPath` (`internal/le/spec/session/review.go`) resolves through `lepath.ResolveSession`, so it answers only about the CURRENT harness session. A later session cannot resolve another session's artifact path, which is why the `reviewed` kind takes an explicit artifact path.
- Review artifacts live under `tmp/review/` (`reviewDir`, `internal/le/spec/session/review.go`), which is untracked, so `tmp/review/` is NOT the durable record of a review. 41 artifacts exist today and none belongs to the three sessions that own the six "a review ran" rows.
- **Git is the durable store.** A spec's `## Review Gate` section is committed prose, and closure removes the spec, so `git show <closure-sha>^:<spec-path>` recovers the whole gate table permanently: the artifact reference, the check verdict, the rounds and the findings fixed. Verified 2026-09-07 against `47bbb259d3`, `b7bfd5ab02`, `1ed6b74e34` and `d95ef465ff`.
  → Constraint: the gate's verdict row carries two different LABELS across the specs on disk, `review_gate.py check` and `review check`, and they do not sort by date: `d95ef465ff^:plan/spec-finish-ci-coverage.md` closed on 2026-09-03 with the first spelling. The label is authored prose, so it tracks neither the recorder nor the era, and a verifier that matches one spelling reads the other as absent. The predicate is over the section's rows, never over one spelling.
  → Constraint: a `## Review Gate` section can exist and record `not recorded / not run`. One of the 68 reason cells says exactly that about its own spec, so the presence of the section is not the proof; a filled artifact reference and a rounds count are.
- `closureStem` (`internal/le/commit/review.go`) stopped reading journal rows as a closure signal on 2026-08-31. 58 of the 68 rows predate that fix, so their review obligation was created by a proxy that no longer exists; re-running today's `closureStem` over those commits is the machine proof that no review was ever owed.
- `committedText` (`internal/le/commit/rfcchange.go`) already reads a path's bytes at a named revision, and `rfc.IsTagCarrier` already decides whether a path can carry an RFC tag. The discharge verifiers reuse both.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/debt.go` - declares `debtGates` (7 rows, Key and Name), the `Debt` type, `recordDebt`, `ListDebt`, `parseDebtRow`, `openDebt`, `clearDebtRows`. `parseDebtRow` accepts a line of exactly 8 pipe-split cells whose status cell is `open` or `cleared`, lowercased; anything else is not a row. `clearDebtRows` builds a per-shard, per-line selection from `openDebt`, selects on the gate name alone, and rewrites the status cell to `cleared` under an exclusive `flock`.
- [ ] `internal/le/commit/actions.go` - `commandVerbs` (8 verbs today), `Answer`'s dispatch, `parseKeywords` and `keywordRule`, `clearDebt` and `clearDebtWith`, and the `Text()` renderings. `clearDebtWith` compares each row's gate against two string LITERALS to build `unrunnableSet`, runs ONE `verify.Run` over `HEAD` when any runnable name exists, and on a zero report code marks every runnable name passed.
- [ ] `internal/le/commit/prepare.go` - `Create`, `checkVerificationGates`, `owedDebt`, `refusePushWithDebt`. `owedDebt` walks `debtGates` and writes an open row for any gate whose override reason or observed reason is non-empty. `refusePushWithDebt` refuses when the open count plus the owed count is not zero.
- [ ] `internal/le/commit/review.go` - `closureStem`, `relocatedSpecs`, `oneStem`, `CheckReview`, `isReviewCode`, `reviewHash`. `CheckReview` resolves the artifact through `specsession.ReviewArtifactPath`, requires a clean verdict in the header, and compares each code-bearing path's recorded hash against `reviewHash` of the file ON DISK.
- [ ] `internal/le/commit/rfcchange.go` - `rfcChangeProblems`, `changedRFCUnits`, `committedText`. `committedText` resolves a revision, lists the path at it, and returns the blob text.
- [ ] `internal/le/hookruntime/lifecycle.go` - the session-start hook calls `commit.ListDebt` and counts rows whose Status equals `open`, printing that N gates are owed and that a push is refused until they clear.
- [ ] `internal/test/fixture/misc_fixture_runner.go` - `verifyScopeDebtClearDriver`, the fixture behind `test/runner/verify-scope-debt-clear.ci`. It drives the real `le commit debt-clear` against a scratch checkout and asserts the LEDGER, never its own edit.
- [ ] `plan/journal/gate-owed-a-clearance-no-command-writes.md` - the 2026-09-07 row recording this defect and both doc disagreements.

**Behavior to preserve:**
- The on-disk debt row keeps its 6 data cells and its two written statuses, `open` and `cleared`. Every one of the 3526 rows stays parseable by one parser.
- `recordDebt` keeps its per-session shard, its `flock`, and its duplicate suppression.
- `debt-clear` keeps clearing a row only after the gate it names exits 0, over a COMMIT rather than the working tree.
- `refusePushWithDebt` keeps refusing a push while a real obligation is open.
- The three consumers keep reading one producer: nothing gains its own copy of "which rows count as open".

**Behavior to change:**
- A per-row discharge verb exists and writes a discharge record.
- `ListDebt` overlays valid discharge records, so a discharged row reports status `discharged` to every consumer.
- The unrunnable set is derived from `debtGates` rather than from two literals.
- An unrecognized gate name fails closed instead of clearing on a green verify.
- `debt-clear` exits non-zero when the verification it ran was red.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit debt-discharge shard <name> line <n> [line <n> ...] kind <kind> <evidence keyword> <value>`, typed by an operator or an agent.
- Format at entry: argv, parsed by `parseKeywords` against a closed rule map.

### Transformation Path
1. `Answer` dispatches the verb and resolves the checkout root through `lepath.Root`.
2. `parseKeywords` accepts only the closed keywords; an unknown keyword or a missing value is exit 2.
3. The named shard and lines are read through `ListDebt`, so the row text under judgement is the row the ledger holds.
4. The kind's verifier runs. Every kind but `owner` reads git: `not-applicable` reads the named commit's file list, `closed` reads the removed spec at the closure commit's PARENT, and `reviewed` reads the artifact plus the commit's blobs. `owner` reads nothing.
5. On a verified verdict, one discharge row per debt row is appended to `plan/verification-debt/discharged/<session>.md` under an exclusive `flock`.
6. Every later read of the ledger re-runs step 4 from the recorded evidence, so the record holds the INPUT and never a cached verdict.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le command ↔ ledger files | markdown rows under `plan/verification-debt/`, `flock` on write | No |
| le command ↔ git | revision resolution, the commit's name-status file list, and the blob at a path, through the shape `committedText` already uses | No |
| le command ↔ a closed spec | the removed spec's bytes at the closure commit's parent, read with the same revision-and-path shape as the RFC gate | No |
| ledger ↔ session hook | `commit.ListDebt` status field, unchanged call site | No |

### Integration Points
- `commandVerbs` and `Answer` (`internal/le/commit/actions.go`) - one new row, one new case.
- `ListDebt` (`internal/le/commit/debt.go`) - the single producer every consumer already reads.
- `closureStem` (`internal/le/commit/review.go`) - reused unchanged as the `not-applicable` verifier for the review gate, and reused again by `closed` to name the spec the closure commit removed.
- `committedText` (`internal/le/commit/rfcchange.go`) - reused three times: with `rfc.IsTagCarrier` for the RFC gate's `not-applicable`, for the removed spec's bytes under `closed`, and for the commit's blobs under `reviewed`.

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
| A-1 | Re-running today's `closureStem` over a 2026-08 commit answers the empty string for the 26 "no spec closes here" rows | `closureStem` reads only REMOVED spec paths since 2026-08-31, and each of the 26 reason cells states no spec was removed | Those rows need `kind owner` instead, and the owner-attested residue grows from 17 to about 43 | Run the discharge pass; a refusal names the stem it found | unvalidated |
| A-2 | The commit a debt row belongs to can be identified by the operator from the row's Date, Session and Subject | The row carries no SHA; a subject search over the log is the practical route | The pass cannot proceed row by row and the ledger needs a widened row after all | The first discharge pass; a subject that matches no commit, or two, is reported rather than guessed | unvalidated |
| A-3 | The 6 "a review ran" rows are reached by a derived kind without the tmp artifact: `not-applicable` where their commit closes no spec, `reviewed` against a committed Review Gate otherwise | Their reason cells describe baseline and repair commits ("a commit baseline before review", "this repair commit", "the final committed le cutover"), which remove no spec; and `git show <closure-sha>^:<spec-path>` recovers a Review Gate permanently | Whichever of the 6 neither route reaches falls back to `kind owner`, raising the attested count above 17 | The pass, row by row; each refusal names what the derivation found | unvalidated |
| A-6 | A Review Gate recovered from a closure commit's parent can be judged without matching one field label | Measured 2026-09-07: the verdict row is spelled `review_gate.py check` in `d95ef465ff^:plan/spec-finish-ci-coverage.md` and `review check` in `plan/immediate/spec-traffic-vpp-deferred-reply-timeout.md`. Across the specs on disk the label splits between those two spellings, and every gate read carries a filled Artifact row and a Rounds row whichever it uses | The verifier matches one spelling and reads the other as absent, which either strands rows or, worse, discharges on a section it never read | `TestClosedDischargeRequiresARecordedReviewGate` over both spellings | confirmed: the test drives `review_gate.py check` and `review check` through one predicate over the section's rows, and both discharge |
| A-4 | The four legacy gate spellings measured today are the complete set in the ledger | Measured 2026-09-07 across 208 shards. Re-measured 2026-09-08 over the ledger on disk: FIVE legacy spellings, not four. `./le verify current mode full (not FRESH-green)` on 178 rows, `./le verify current mode full structural gates (red)` on 157, `full ./le verify current mode full over this commit's Go` on 118, `./le repository tracked-build check (HEAD does not compile)` on 5 and `repository-tracked-build/check (HEAD does not compile)` on 3 | A fifth spelling stays open and is named by the new unrecognized report, which is the fail-closed behavior AC-15 asks for | `TestEveryLedgerGateNameIsDeclared` | broken: the structural-gate spelling was the fifth, and it is declared as an alias per the Failure Routing row |
| A-5 | The owner will supply one authorization sentence covering the 17 rows he ordered or reviewed himself (12 plus 5) | Those rows quote his order in their own reason cell, and one of them states that an owner-approval row an author wrote for himself is a forgery | Those 17 rows stay open and the push gate stays shut | Owner answer, recorded verbatim in the discharge record | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hand-edited discharge record silently un-refuses a push | A discharge row naming a shard and line whose digest does not match | The digest is re-checked on every read; a mismatch is ignored, the debt row counts OPEN, and the mismatch is reported by name |
| R-2 | Re-deriving on every read makes `debt-status` slow as discharged rows accumulate | `debt-status` takes longer than a second | The derivation is one git read per commit-bearing kind and the discharged population is bounded by the 72; if it grows past a few hundred, narrow the read rather than caching a verdict |
| R-3 | `kind owner` becomes the lazy route for every awkward row | The discharged population is mostly `owner` | `debt-status` prints the discharged split by kind, so the ratio is visible in one command; a derived kind that applies refuses nothing, so choosing `owner` over it is a deliberate act |
| R-8 | The `closed` verifier accepts a Review Gate section that records no review | A spec closed at `in-progress` with a gate reading "not recorded / not run" discharges | The predicate requires a FILLED artifact reference and a rounds count, and the skeleton branch is reached only by the removed spec's Status at the parent, never by a missing gate (AC-7, AC-20) |
| R-4 | A discharge record is written for a row another session is clearing at the same time | Two writers on one shard | The record is a separate per-session file and clearing rewrites only the debt shard, so the two writers never share a file. A row both cleared and discharged is consistent, not a conflict |
| R-5 | The operator names the wrong commit and the derivation passes for the wrong subject | A `not-applicable` verdict for a commit whose subject does not match the row | The record stores the SHA, so the pairing stays auditable; the verb prints the commit's subject beside the row's subject and refuses when neither string contains the other |
| R-6 | Making an unrecognized gate name fail closed strands rows nobody can clear | The unrecognized count is not zero after the pass | The alias declaration is the repair, and `TestEveryLedgerGateNameIsDeclared` turns a stranded name into a red test rather than a silent open row |
| R-7 | The overlay changes what the session hook prints, and an agent reads the silence as "no debt" | The hook stops naming rows that are discharged | The hook's line is derived from the same producer, and `debt-status` still prints the discharged count, so the fact stays visible where it is actionable |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A push is allowed while a real review obligation is outstanding, or the push gate stays shut. Nothing user-visible in the daemon: the change is development tooling |
| How is it reverted? | Single commit revert. The discharge records are additive files; removing them returns every row to open |
| Who else touches this path? | Every session that commits. `internal/le/hookruntime/lifecycle.go` reads the ledger on session start; `test/runner/verify-scope-debt-clear.ci` drives the clearing verb |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit debt-discharge ...` typed at the shell | → | `Answer` case plus `dischargeDebt` (`internal/le/commit/discharge.go`) | `TestDebtDischargeIsReachableFromTheCommitVerbTable` |
| `./le commit create ... push "<auth>"` | → | `refusePushWithDebt` reading the overlaid `openDebt` | `TestPushProceedsWhenEveryRemainingRowIsDischarged` |
| session-start hook | → | `commit.ListDebt` status field | `TestSessionHookCountsNoDischargedRowAsOwed` |
| `ze-test fixture runner/verify-scope-debt-discharge` | → | the real `le commit debt-discharge` against a scratch checkout | `test/runner/verify-scope-debt-discharge.ci` |
| `./le commit debt-clear` over a row naming an unrecognized gate | → | `clearDebtWith` gate resolution | `TestUnrecognizedGateNameIsNeverClearedByAGreenVerify` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le commit debt-discharge` with an unknown keyword, a missing value, or a `kind` outside the closed set | Exit 2, stderr names the offending token and lists the accepted kinds; nothing is written |
| AC-2 | `kind not-applicable commit <sha>` on a row naming `independent critical review`, where that commit removes no spec file | The row is discharged and the record stores the SHA |
| AC-3 | The same, where that commit DOES remove a spec file | Refused, naming the stem the derivation found; nothing is written |
| AC-4 | `kind not-applicable commit <sha>` on a row naming `owner approval for an RFC-tagged test change`, where no path in that commit is a tag carrier holding an RFC requirement tag at that revision | The row is discharged; a commit that does change a tagged unit is refused, naming the unit |
| AC-5 | `kind reviewed commit <sha> artifact <path>` where the artifact's header verdict is clean and every code-bearing path of that commit carries a hash equal to that path's bytes AT that commit | The row is discharged |
| AC-6 | The same, where the verdict is not clean, a code-bearing path is uncovered, or a recorded hash differs from the commit's bytes | Refused, naming the verdict or every offending path |
| AC-6b | `kind reviewed commit <sha>` with no `artifact` keyword, or with one naming a path that no longer exists under `tmp/` | The verifier reads the Review Gate committed in the spec instead: it takes the spec the commit removed, reads it at the commit's PARENT, and judges the gate by AC-7's predicate. A commit that removes no spec is refused, naming that there is nothing to read |
| AC-7 | `kind closed commit <closure-sha>` where the removed spec's `## Review Gate` at the parent carries a filled artifact reference and a rounds count | The row is discharged. Both era spellings of the verdict row are accepted, because the predicate reads the section's rows and never one field label |
| AC-7b | The same, where the gate section is absent, is the unfilled template, or records "not recorded" or "not run", and the removed spec's Status at the parent is `ready`, `in-progress` or `verification` | Refused, quoting what the gate said. An implemented spec with no recorded review does NOT discharge through the skeleton branch |
| AC-8 | `kind owner owner "<authorisation>"` with a non-empty string | The row is discharged and the string is recorded verbatim; an empty string is exit 2 |
| AC-9 | Any accepted discharge | One row per debt row is appended to `plan/verification-debt/discharged/<session>.md`, carrying the date, the shard and line, the SHA-256 of the debt row's raw text, the kind, and the evidence |
| AC-10 | `ListDebt` over a ledger holding a valid discharge record | That row's Status is `discharged`, and its discharge kind and evidence are on the row |
| AC-11 | A discharge record whose digest does not match the debt row at that shard and line, or that names a shard or line the ledger does not hold | The record is not applied, the debt row reports `open`, and the record is reported as invalid by name |
| AC-12 | A ledger whose only remaining rows are discharged, and an authorized push | `refusePushWithDebt` allows the push; `openDebt`, `clearDebtRows` and the session hook each treat the row as not open, with no second copy of the rule |
| AC-13 | Any read of a discharged row | The kind's derivation runs again from the recorded evidence; no verdict is read from the record, and a record whose derivation now fails leaves the row `open` |
| AC-14 | `debt-clear` over rows naming the review gate or the owner-approval gate | Those gates are named unrunnable because `debtGates` declares them unrunnable; the two string literals in `clearDebtWith` no longer exist |
| AC-15 | `debt-clear` over a row naming a gate string that is neither a declared gate Name nor a declared alias | The row is NOT cleared by a green verify, and the report names it as unrecognized. The five legacy spellings measured on 2026-09-08 are declared aliases and do clear |
| AC-16 | `debt-clear` whose verification run was red | Exit is non-zero and the report states that nothing cleared, so the exit code and the ledger agree |
| AC-17 | `debt-status` and `debt-list` | `debt-status` reports open, cleared and discharged, with the discharged count split by kind; `debt-list` rows carry a discharge kind and a discharge evidence field; both answer structured data, so `\| json`, `\| yaml` and `\| table` each render it, and every JSON key is kebab-case |
| AC-18 | `docs/architecture/testing/verify-freshness-scope.md` after the change | Its debt section states that ONE verification runs per pass and marks every runnable gate name passed, states the discharge verb and its kinds, and no longer claims a runner table the code does not have |
| AC-19 | After the discharge pass over the 72 rows | `./le commit debt-status` reports zero open rows naming a gate no command can run; any residue is named with the reason it could not be discharged |
| AC-20 | `kind closed commit <closure-sha>` where the removed spec's Status at the parent is `skeleton` or `design` | The row is discharged with no Review Gate read, because a spec that carried no implementation owed no review. The Status is read from the spec's own metadata table at the parent, never inferred from an absent gate |
| AC-21 | Any kind, `owner` included, over a row whose gate `debtGates` declares `Runnable`, or over a gate `debtGates` declares neither as a Name nor as an alias | Refused before the kind's own evidence is read, naming the gate; the runnable one is routed to `debt-clear`, which clears it by RUNNING it. The check is read from the row in `verifyDischarge`, once, so a fifth kind inherits it |
| AC-22 | A discharge over a row whose subject cell carries `(+N more)` | `commit <sha>` repeats and the row discharges only when N+1 commits are named: each distinct, each running the kind's derivation, and each bound to the row by carrying its subject or by writing its ledger shard, with at least one carrying the subject. Fewer, a repeat, or a commit bound neither way is refused, naming the count the row needs |
| AC-23 | A discharge over a row whose Status is `cleared`, and a hand-written record naming one | Refused; the overlay leaves the row `cleared` and reports the record as invalid, because a cleared row's gate ran green and `debt-status` must not read it as attested (R-3) |
| AC-24 | `kind closed` or `kind reviewed` over a row naming `owner approval for an RFC-tagged test change` | Refused, naming the gate: both kinds assert that a REVIEW ran, and an owner's approval is an act no reviewer performs |
| AC-25 | `kind closed` or `kind reviewed` where the Review Gate's artifact cell holds `n/a`, `-`, or any text naming no file | Refused, saying the artifact row names no file. A cell naming a path with a file name on it is the filled reference R-8 asks for |
| AC-26 | A keyword the kind never reads: `commit` or `artifact` with `kind owner`, `owner` with any derived kind, `artifact` with `kind not-applicable` or `kind closed` | Exit 2, naming the keyword and the kind. An unread keyword would be stored and printed as evidence the derivation never read |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDebtDischargeIsReachableFromTheCommitVerbTable` | `internal/le/commit/discharge_test.go` | AC-1, the verb is listed and dispatched | PASS |
| `TestDischargeRefusesAnUnknownKindAndWritesNothing` | `internal/le/commit/discharge_test.go` | AC-1 | PASS |
| `TestNotApplicableDischargeDerivesTheClosureStem` | `internal/le/commit/discharge_test.go` | AC-2 and AC-3, both polarities over a fixture checkout with and without a removed spec | PASS, red under PROBE-6 |
| `TestNotApplicableDischargeReadsRFCTagCarriersAtTheCommit` | `internal/le/commit/discharge_test.go` | AC-4, both polarities | PASS, red under PROBE-7 |
| `TestReviewedDischargeJudgesTheArtifactAgainstTheCommitBytes` | `internal/le/commit/discharge_test.go` | AC-5 and AC-6, including a hash that matches the working tree but not the commit | PASS, red under PROBE-5 |
| `TestReviewedDischargeFallsBackToTheCommittedReviewGate` | `internal/le/commit/discharge_test.go` | AC-6b: the tmp artifact is deleted and the discharge still derives, and a commit removing no spec is refused | PASS |
| `TestClosedDischargeRequiresARecordedReviewGate` | `internal/le/commit/discharge_test.go` | AC-7 over both spellings of the verdict row, and AC-7b over an absent gate, the unfilled template, and a "not recorded" gate | PASS, red under PROBE-3 |
| `TestSkeletonClosureDischargesAndAnImplementedOneDoesNot` | `internal/le/commit/discharge_test.go` | AC-20 and its fail-open twin: `skeleton` at the parent discharges, `in-progress` with the same missing gate refuses | PASS, red under PROBE-3 |
| `TestOwnerDischargeRecordsTheAuthorisationVerbatim` | `internal/le/commit/discharge_test.go` | AC-8 and AC-9 | PASS |
| `TestDischargedRowsReportDischargedFromOneProducer` | `internal/le/commit/discharge_test.go` | AC-10 and AC-12 over `openDebt` and `clearDebtRows` | PASS, red under PROBE-8 |
| `TestATamperedDischargeRecordLeavesTheRowOpen` | `internal/le/commit/discharge_test.go` | AC-11, the fail-closed guard, driven from `ListDebt` | PASS, red under PROBE-2 |
| `TestDischargeVerdictIsNeverReadFromTheRecord` | `internal/le/commit/discharge_test.go` | AC-13: the evidence is invalidated after the record is written and the row returns to open | PASS |
| `TestPushProceedsWhenEveryRemainingRowIsDischarged` | `internal/le/commit/commit_test.go` | AC-12 at the push gate | PASS, red under PROBE-8 |
| `TestUnrunnableGatesComeFromTheGateTable` | `internal/le/commit/commit_test.go` | AC-14, no literal survives | PASS, red under PROBE-1 |
| `TestUnrecognizedGateNameIsNeverClearedByAGreenVerify` | `internal/le/commit/commit_test.go` | AC-15, fail closed | PASS, red under PROBE-1 |
| `TestEveryLedgerGateNameIsDeclared` | `internal/le/commit/ledger_test.go` | AC-15, every gate string in `plan/verification-debt/` resolves to a declared Name or alias | PASS over the live ledger; it found the fifth spelling |
| `TestDebtClearingReportsRedVerificationAsFailure` | `internal/le/commit/commit_test.go` | AC-16, extends the red half of `TestDebtClearingHonorsTheGateExit` | PASS, red under PROBE-4 |
| `TestSessionHookCountsNoDischargedRowAsOwed` | `internal/le/hookruntime/lifecycle_test.go` | AC-12 at the third consumer | PASS, red under PROBE-8 |
| `TestDebtStatusSplitsDischargedByKind` | `internal/le/commit/discharge_test.go` | AC-17 | PASS, red under PROBE-8 |
| `TestDischargedDebtRowsCarryTheirKindAndEvidence` | `internal/le/commit/discharge_test.go` | AC-17's debt-list half: the row's kebab-case discharge fields | PASS |
| `TestADischargeRefusesACommitTheRowDoesNotName` | `internal/le/commit/discharge_test.go` | R-5: the pairing guard, both polarities | PASS, red under PROBE-9 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `line` | 1 to the line count of the named shard | the shard's last row line | 0, refused with exit 2 | last plus one, refused: no row there |
| `line` repeats | 1 to every open row of the shard | all open rows of the shard | N/A | a repeat naming the same line twice is refused |
| row digest | 64 hex characters | the row's SHA-256 | a shorter string is an invalid record | a longer string is an invalid record |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-debt-discharge` | `test/runner/verify-scope-debt-discharge.ci` | An agent discharges a row whose commit closes no spec, sees the ledger stop counting it as open, then tampers with the record and sees the row counted open again | written and compiling (`go vet ./internal/test/fixture/` clean); NOT RUN. The suite drives `bin/le`, and `bin/le` cannot be built from this working tree: another session's uncommitted edit to `internal/component/config/transaction/orchestrator.go:249` calls `o.operationNodes`, which `TxCoordinator` does not have, and passes three arguments to `runOperationPath`, which takes two. That is a peer's in-flight work, not this change. Run `./le functional runner` once it compiles |
| `verify-scope-debt-clear` | `test/runner/verify-scope-debt-clear.ci` | Existing scenario: a row no gate can re-run is named and left open. Its expectations are re-read against the derived unrunnable set | |

### Interop Tests (Scope: protocol)
- N-A. Scope is tooling: no protocol peer exists for a ledger command.

## Files to Modify
- `internal/le/commit/debt.go` - `debtGates` gains a runnable flag and the declared aliases; `ListDebt` overlays valid discharge records; `openDebt` follows without a second rule
- `internal/le/commit/actions.go` - the `debt-discharge` row in `commandVerbs`, its `Answer` case and keyword rules; `clearDebtWith` derives the unrunnable set from `debtGates`, names unrecognized gate strings, and answers non-zero on a red verification; the debt status answer and its `Text()` gain the discharged split
- `internal/le/commit/review.go` - the artifact coverage judgement takes its bytes from a named source, so one producer answers for the working tree and for a commit
- `internal/le/commit/prepare.go` - no behavioral edit expected: `refusePushWithDebt` reads `openDebt`, which the overlay already narrows. Listed so the review re-reads it
- `internal/test/fixture/misc_fixture_runner.go` - the driver for the new scenario, beside `verifyScopeDebtClearDriver`
- `docs/architecture/testing/verify-freshness-scope.md` - AC-18, plus the discharge verb and its kinds
- `docs/contributing/committing.md` - the push refusal sentence gains the discharge route
- `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md` - the enforcement sentence names the discharge verb; regenerate with `./le rules condensed-update`
- `ai/INDEX.md` - the commit row's keyword list gains the discharge keywords

## Files to Create
- `internal/le/commit/discharge.go` - the verb, the four kinds, their verifiers, and the discharge-record reader and writer
- `internal/le/commit/discharge_test.go` - the unit tests above
- `test/runner/verify-scope-debt-discharge.ci` - the functional scenario
- `plan/verification-debt/discharged/<session>.md` - written BY the pass, never by hand

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` is offline development tooling and reaches no daemon; no YANG node exists for any `le` verb |
| YANG validation constraints | N-A | Same reason |
| YANG custom validators | N-A | Same reason |
| CLI commands/flags | Yes | `internal/le/commit/actions.go`: one `commandVerbs` row and one `Answer` case; no flag is added |
| CLI grammar (keyword before value) | Yes | `parseKeywords` with a closed rule map: `shard`, `line`, `kind`, `commit`, `artifact`, `owner` |
| Editor autocomplete | N-A | `le` verbs are listed by `Subs`, which reads `commandVerbs`, so the new verb is listed with no second edit |
| Functional test for new RPC/API | Yes | `test/runner/verify-scope-debt-discharge.ci` |
| Pipe completeness | Yes | The answer is a struct with kebab-case tags and a `Text()` method, rendered by the `leaction` layer (`internal/le/leaction/leaction.go`), so `\| json`, `\| yaml` and `\| table` all apply |
| Env var registration | N-A | No new environment input |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module or binary at daemon runtime. The verb runs git, which every `le` commit path already requires |
| Prometheus counters/metrics | N-A | Development tooling, not daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | `docs/features.md` describes the daemon; a development ledger verb is not a product feature |
| 2 | Config syntax changed? | N-A | No config surface |
| 3 | CLI command added/changed? | N-A | `docs/guide/command-reference.md` documents `ze`, not `le` development verbs |
| 4 | API/RPC added/changed? | N-A | No RPC |
| 5 | Plugin added/changed? | N-A | No plugin |
| 6 | Has a user guide page? | Yes | `docs/contributing/committing.md`, the page an author reads before committing |
| 7 | Wire format changed? | N-A | No wire format |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | The RFC-approval gate is a commit gate, not RFC behavior; no `rfc/short/` row moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` gains the new scenario beside `verify-scope-debt-clear` |
| 11 | Affects daemon comparison? | N-A | No daemon behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, the page `debt.go` declares |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | `le` verbs are not in the plugin or command inventories those pages publish |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Named from the changed files' own `// Design:` headers. `debt.go` declares `docs/architecture/testing/verify-freshness-scope.md`, updated at row 12. `actions.go` declares `docs/architecture/core-design.md`, unaffected: that page's claim is one structured commit workflow command, which a ninth verb does not change. `review.go` declares `docs/features/ai-first.md`, unaffected: that page describes no review artifact and the commit-time gate keeps its behavior. `rfcchange.go` declares `docs/contributing/rfc-implementation-guide.md`, unaffected: `committedText` is read, not changed. Re-derive with `./le spec citation anchors spec plan/immediate/spec-verification-debt-clearing.md` before the closure commit |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/committing.md` and `docs/architecture/testing/verify-freshness-scope.md` both show debt commands; both are re-read against the implemented verb |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the verb exists, is reachable, and refuses everything
   - Tests: `TestDebtDischargeIsReachableFromTheCommitVerbTable`, `TestDischargeRefusesAnUnknownKindAndWritesNothing`
   - Files: `internal/le/commit/actions.go` (verb row, `Answer` case, keyword rules), `internal/le/commit/discharge.go` (kind table, entry function that refuses every kind)
   - Verify: `./le commit` lists the verb; the wiring tests fail while the kinds are stubs and pass once the refusals are real
2. **Phase: the record and the overlay** -- a discharge is stored and read back, fail closed
   - Tests: `TestOwnerDischargeRecordsTheAuthorisationVerbatim`, `TestDischargedRowsReportDischargedFromOneProducer`, `TestATamperedDischargeRecordLeavesTheRowOpen`, `TestPushProceedsWhenEveryRemainingRowIsDischarged`, `TestSessionHookCountsNoDischargedRowAsOwed`
   - Files: `internal/le/commit/discharge.go`, `internal/le/commit/debt.go`
   - Verify: the `owner` kind alone proves the storage and the overlay end to end, with no derivation in the way
3. **Phase: the derived kinds** -- `not-applicable`, `closed`, `reviewed`
   - Tests: `TestNotApplicableDischargeDerivesTheClosureStem`, `TestNotApplicableDischargeReadsRFCTagCarriersAtTheCommit`, `TestClosedDischargeRequiresARecordedReviewGate`, `TestSkeletonClosureDischargesAndAnImplementedOneDoesNot`, `TestReviewedDischargeJudgesTheArtifactAgainstTheCommitBytes`, `TestReviewedDischargeFallsBackToTheCommittedReviewGate`, `TestDischargeVerdictIsNeverReadFromTheRecord`
   - Files: `internal/le/commit/discharge.go`, `internal/le/commit/review.go` (the byte source)
   - Verify: each verifier is tested in both polarities; a verifier that cannot refuse has an untested guard
4. **Phase: the gate table** -- unrunnable and unrecognized derive from `debtGates`
   - Tests: `TestUnrunnableGatesComeFromTheGateTable`, `TestUnrecognizedGateNameIsNeverClearedByAGreenVerify`, `TestEveryLedgerGateNameIsDeclared`, `TestDebtClearingReportsRedVerificationAsFailure`
   - Files: `internal/le/commit/debt.go`, `internal/le/commit/actions.go`
   - Verify: the two string literals are deleted, not shadowed
5. **Phase: the answers and the page** -- `debt-status`, `debt-list`, the functional scenario, the docs
   - Tests: `TestDebtStatusSplitsDischargedByKind`, `test/runner/verify-scope-debt-discharge.ci`
   - Files: `internal/le/commit/actions.go`, `internal/test/fixture/misc_fixture_runner.go`, `test/runner/verify-scope-debt-discharge.ci`, `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/committing.md`, `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md`, `ai/INDEX.md`
   - Verify: the page's two wrong sentences are replaced by what the code does
6. **Phase: the pass over the 72** -- discharge the rows and report the residue
   - Tests: none new; the evidence is `./le commit debt-status`
   - Files: `plan/verification-debt/discharged/<session>.md`
   - Verify: every row that could not be discharged is named with the reason, and the owner-attested rows quote the owner's own sentence
7. **Phase: the row's own gate and cover decide the discharge** -- the repair an independent pass over `27b41390a` asked for
   - Tests: `TestADischargeAnswersOnlyAGateNoVerificationRuns`, `TestADischargeAnswersOnlyAnOpenRow`, `TestARowCoveringSeveralCommitsAnswersForEachOfThem`, `TestClosedAndReviewedAnswerTheReviewGateOnly`, `TestAReviewGateArtifactMustNameAFile`, `TestDischargeRefusesAnUnknownKindAndWritesNothing`
   - Files: `internal/le/commit/discharge.go`, `internal/le/commit/dischargerecord.go`, `internal/le/commit/debt.go`, `internal/le/commit/actions.go`, `docs/architecture/testing/verify-freshness-scope.md`
   - Verify: each new guard is broken on purpose and its test observed RED before the green is trusted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol, and each of the four kinds has both polarities tested |
| Feature completeness | The push gate, the clearing verb, the status answer and the session hook all read the overlaid ledger |
| Correctness | A discharge verdict is never read from disk; the derivation re-runs on every read (AC-13) |
| Guard shape | Each verifier fails closed: an unreadable commit, a missing artifact, an unmatched digest and an unknown shard each leave the row OPEN (`ai/rules/evidence.md`). The skeleton branch is entered from the removed spec's Status, never from an absent Review Gate |
| Data flow | One producer answers "is this row open": `ListDebt`. `openDebt`, `clearDebtRows`, `refusePushWithDebt` and the session hook read it and hold no second rule |
| Naming | JSON keys kebab-case; the kind vocabulary is closed and spelled once |
| Rule: `ai/rules/no-layering.md` | The two gate-name literals in `clearDebtWith` are DELETED, and the on-disk debt row keeps ONE format with 6 data cells |
| Rule: `ai/rules/cli.md` | Keyword before value; the payload is structured data and every pipe operator applies |
| Rule: `ai/rules/principles.md` | An unrecognized gate name is not silently runnable; an empty authorization is not a valid attestation |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The verb is listed | `./le commit` prints `debt-discharge` |
| The kinds are closed | `./le commit debt-discharge kind bogus` exits 2 and lists the four kinds |
| The overlay reaches every consumer | grep for the open-status test across `internal/le/commit` and `internal/le/hookruntime` shows one rule, not four |
| The literals are gone | grep for the two gate-name literals in `internal/le/commit/actions.go` returns nothing |
| The page agrees with the code | `docs/architecture/testing/verify-freshness-scope.md` no longer claims a per-gate run or a runner table |
| The ledger can reach zero | `./le commit debt-status` reports no open row naming an unrunnable gate |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `shard` is a base name under `plan/verification-debt/`, never a path holding a separator or a parent reference; `line` is a positive integer; `artifact` is read through the checkout root like every other ledger path |
| Argument injection | The commit revision and the paths reach git as argv operands, never as a shell string, matching the shape `committedText` already uses |
| Authorization that could fail open | The `owner` kind is an attestation and is marked as one; it must never be the default kind, and a missing `kind` keyword is a refusal rather than an owner discharge |
| Resource exhaustion | A discharge reads at most one commit per row, and a `line` list is bounded by the shard's row count |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| A verifier passes in both polarities | The test is vacuous: force the red before trusting the green |
| `TestEveryLedgerGateNameIsDeclared` red | A fifth legacy spelling exists: declare it as an alias, never widen the fallback |
| The discharge pass refuses a row A-1 predicted would pass | Record what the derivation found; the row moves to `kind owner` only with the owner's sentence |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The debt row's missing SHA is not repaired by widening the row, because the fact is already missing from 3526 written rows. It is supplied at DISCHARGE time by the operator and then re-derived, which turns an unrecoverable gap into a checkable claim.
- The 68 review rows are mostly the residue of a gate that has since been fixed: `closureStem` stopped reading journal rows as closures on 2026-08-31. Re-running the current producer over the old commits is not an amnesty, it is the correct verdict computed by today's code.
- Keeping the verdict OUT of the record is what makes the record safe to commit. A reader who trusts a stored verified cell trusts whoever last edited the file.
- The review evidence this repository keeps is in GIT, not in `tmp/review/`. The artifact is a working file that a machine reset deletes; the Review Gate is committed prose that closure preserves in history. Any future check that wants to know whether a spec was reviewed should read the closure commit's parent, and none should depend on `tmp/`.
- A missing Review Gate has two meanings and they must not share a branch. On a `skeleton` spec it means there was nothing to review; on an implemented one it means the review is missing. Reading the removed spec's Status is what separates them, and reading the absence itself is the fail-open shape.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A per-row discharge verb that records HOW the obligation was met | Widening the `Debt` row so a review check can judge it | Refuted before this spec: `CheckReview` needs a spec stem to resolve `specsession.ReviewArtifactPath`, and 26 of the 68 rows were written for commits where no spec exists, so a widened row has no stem to carry. Widening also puts 3526 existing 6-cell rows behind a dual-format parser, which `ai/rules/no-layering.md` forbids |
| The discharge record is a separate file and the debt row is not rewritten | A third status written into the debt row's status cell | The row's on-disk vocabulary stays open and cleared, so one parser reads all 3526 rows. The discharged status exists in memory, produced by the overlay, which is why the session hook needs no edit |
| The record stores the kind and the evidence, never the verdict | Storing a verified column the reader trusts | A stored verdict is a claim by whoever last wrote the file. Re-deriving on every read costs one git read per commit-bearing row and cannot go stale (`ai/rules/principles.md`) |
| Four kinds, one of them attested | One free-text reason per discharge | An attested discharge and a derived one must be distinguishable in the ledger, and a free-text reason cell is exactly what failed for the 68 rows |
| `closed` asks whether the review HAPPENED, at the closure commit | `spec-open`, which asked whether the spec file still exists and read "the review is not yet due" as a discharge | A promise is not a discharge. All 10 rows this kind was written for have since closed, 8 with a Review Gate carrying an artifact, a rounds count and a findings loop, so the completed review is available and the weaker predicate is unnecessary. `spec-open` would also have gone WRONG on exactly these rows: the spec file no longer exists, so it would have refused every one of them |
| The durable evidence is the committed Review Gate, not `tmp/review/` | Reading only the artifact `tmp/review/` holds | `tmp/` is untracked and is emptied; the spec's gate section is committed, and closure removes the spec, so the gate is recoverable for ever from the closure commit's parent. `reviewed` therefore falls back to it when the artifact is gone |
| `kind owner` requires a FRESH authorization string | Treating the row's own reason cell as the owner's word | The reason cells were written by the author, and one of them says it outright: an owner-approval row an author wrote for himself is a forgery. The trust model matches the push authorization |
| An unrecognized gate name fails closed, with declared aliases for the legacy spellings | Keeping today's rule, where anything but the two literals is runnable | Today's default clears about 930 rows on a green verify under names `debtGates` never emits. That reading is right for those four spellings and wrong as a default. Declaring the aliases keeps the rows clearable while an unknown name stops clearing silently |
| The 9 foreign-session rows are discharged by derivation, not by vouching | Asking each writing session, or leaving them open | Their reason cells state the stem came from another session's rows in a shared journal class file, and today's `closureStem` reads no journal row as a closure. The derivation voids the obligation without anyone vouching for it |

## Known Limitations
- `kind owner` cannot be verified by any code. It is an attestation with the same trust model as the push authorization, and the ledger marks it as one.
- A review whose spec never closed is unreachable by any derived kind: the artifact is in `tmp/`, the Review Gate is uncommitted, and nothing durable records it. Such a row falls back to `kind owner`. Every review this repository has actually recorded and closed IS reachable, through the closure commit.
- A row whose commit cannot be identified stays open. The verb has no way to guess, and guessing is what the discharge record exists to prevent.
- Normalizing the legacy gate spellings in the rows that carry them is NOT done here: they are declared as aliases and clear on a green verify. Rewriting historical rows would edit evidence for cosmetic gain.

## Goal Validation

| Goal | Evidence that proves it |
|------|------------------------|
| The ledger can reach zero, so a push is reachable | `./le commit debt-status` after the phase-6 pass reports no open row naming a gate no command can run, and `TestPushProceedsWhenEveryRemainingRowIsDischarged` proves the push gate follows the overlay rather than the raw row count |
| A discharge records HOW the obligation was met, with evidence | AC-9 and `TestOwnerDischargeRecordsTheAuthorisationVerbatim`: the record carries the shard, the line, the row digest, the kind and the evidence, and a row cleared by `debt-clear` is a different state from a row discharged by attestation in `debt-status` |
| The verb re-verifies what a machine can re-verify | `TestNotApplicableDischargeDerivesTheClosureStem`, `TestNotApplicableDischargeReadsRFCTagCarriersAtTheCommit`, `TestClosedDischargeRequiresARecordedReviewGate`, `TestSkeletonClosureDischargesAndAnImplementedOneDoesNot`, `TestReviewedDischargeJudgesTheArtifactAgainstTheCommitBytes` and `TestReviewedDischargeFallsBackToTheCommittedReviewGate`, each red in its refusing polarity, plus `TestDischargeVerdictIsNeverReadFromTheRecord` for the no-cached-verdict rule |
| An unverifiable discharge is distinguishable from a verified one | `TestDebtStatusSplitsDischargedByKind`: the discharged count is split by kind, and the kind alone says whether a machine derived it |
| A tampered or stale record cannot un-refuse a push | `TestATamperedDischargeRecordLeavesTheRowOpen`, driven from `ListDebt` so every consumer inherits the fail-closed reading |
| The documented behavior matches the code | AC-18 and its Deliverables row: the page's two wrong sentences are replaced, and `TestEveryLedgerGateNameIsDeclared` keeps the alias claim honest as the ledger grows |
| The clearing path stops reporting success over a red run | `TestDebtClearingReportsRedVerificationAsFailure`: a red verification exits non-zero instead of exiting 0 over zero cleared rows |

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
- [ ] AC-1 through AC-20, including AC-6b, AC-7 and AC-7b, all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
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
