# The spec workflow

How a spec is written, tracked, closed, and what each artifact around it holds.
`ai/rules/planning.md` states the obligations; this page carries the formats,
the vocabularies, and what each gate reads.

## Spec status vocabulary

<!-- source: internal/le/spec/status -- Answer -->
Every spec carries a metadata table under its `# Spec:` title. `./le spec status`
parses it, and `hookValidateSpec` in `internal/le/hookruntime/lifecycle.go`
validates it on every write.

| Field | Purpose | Values |
|-------|---------|--------|
| Status | Current state | `skeleton`, `design`, `ready`, `in-progress`, `verification`, `blocked`, `deferred` |
| Handoff | Who closes this spec | `verify` for the two-session handoff, `-` for closure in the same session |
| Depends | Blocking prerequisite | A spec filename, or `-` |
| Phase | Multi-phase progress | `N/M`, or `-` for single-phase |
| Updated | Date of the last status change | `YYYY-MM-DD`, not the last file edit |

| Status | Meaning |
|--------|---------|
| `skeleton` | Task defined, design not started |
| `design` | Research and design in progress |
| `ready` | Design complete, ready for implementation |
| `in-progress` | Actively being implemented |
| `verification` | Implementation complete and committed, awaiting an independent review and closure on Opus 5. Reached only under `Handoff: verify` |
| `blocked` | Waiting on the prerequisite named in Depends |
| `deferred` | Explicitly postponed |

| Event | Status change | Phase | Updated |
|-------|--------------|-------|---------|
| Start research | `skeleton` to `design` | - | Yes |
| Spec approved | `design` to `ready` | - | Yes |
| Start coding | `ready` to `in-progress` | Set `1/N` | Yes |
| Finish a phase | - | Increment | Yes |
| Hand off for review (`Handoff: verify` only) | `in-progress` to `verification` | - | Yes |
| Blocked | to `blocked` | - | Yes |
| Deferred | to `deferred` | - | Yes |

`./le spec status` prints the whole inventory, and `./le spec status | json`
gives the machine-readable form.

## Spec sets

A related set of specs shares a prefix and a number.

| Part | Example |
|------|---------|
| Naming | `spec-<prefix>-<N>-<name>.md` |
| Umbrella | `spec-utp-0-umbrella.md` |
| Children | `spec-utp-1-event-format.md`, `spec-utp-2-command-format.md` |
| Done path | A journal row in `plan/journal/<class>.md` naming the spec in its Spec column |

`inspectClosureSpec` in `internal/le/spec/status/closure.go` treats a stem
containing `-umbrella-` as an umbrella, and never raises it to a high-confidence
closure candidate.

## What each closure gate reads

<!-- source: internal/le/spec/status/closure.go -- CheckClosure, closureCompletedNotClosed -->
<!-- source: internal/le/hookruntime/lifecycle.go -- hookStop -->
<!-- source: internal/le/commit/review.go -- closureStem, CheckReview -->

| Gate | Where | What it reads |
|------|-------|---------------|
| Detector | `./le spec status closure list` | Every spec still `in-progress`. High confidence is a committed journal row whose Spec cell equals the stem, or a `plan/learned/NNN-<slug>.md` whose slug equals it, while the spec is not an umbrella. `closure check spec <s>` exits 3 only for the high-confidence set; weaker candidates are listed under NEEDS VERIFICATION |
| Stop-hook block | `hookStop` (the `block-premature-stop` action) | The session's claimed spec. It calls `specstatus.CheckClosure`, and refuses the stop when the report is blocked |
| Review artifact | `CheckReview` at `./le commit create` | The one spec this commit closes, which `closureStem` answers |

`closureStem` reads a REMOVED `plan/spec-*.md` as the closure. A spec removed
from `plan/` and added under `plan/future/` in the same commit is a RELOCATION,
not a closure, and `relocatedSpecs` excludes it. A journal row is read only when
the commit removes no spec at all, and only when every added row agrees on one
stem: five sessions share this checkout, so a class file carries rows nobody in
this commit wrote. One commit closes one spec, and `oneStem` refuses a second.

The review gate then requires a CLEAN `./le spec session review record` artifact
that covers every reviewable file in the commit and whose hashes still match, so
any edit after the review invalidates it. `review-override <reason>` is the only
way past, and it records a verification-debt row.

## Which reference form each link gate reads

Closure deletes the spec, so every citation of it has to be repointed first. A
grep limited to `// Design:` finds only one of the three forms.

| Gate | Reads |
|------|-------|
| `./le doc check links` | `// Design:` lines and tracked path citations |
| `./le spec citation` (`internal/le/spec/citation`) | Every `plan/spec-*.md` citation inside a spec |
| `internal/le/doc/check.CheckLinks` tracked-citation pass | Every tracked path citation, a `plan/spec-*.md` target included |

## Deferral records

`plan/deferrals/` holds one file per source. There is no single
`plan/deferrals.md` and no committed aggregate: <!-- doc-links: ignore (the single aggregate file is deliberately retired and must not exist) -->
the live backlog is a fold over
the directory, computed on read. A stored aggregate would be a shared file every
session appends to, which is the cross-commit hazard the layout removes
(`ai/rules/git-safety.md`).

| Source of the row | Shard file |
|-------------------|------------|
| A spec (the row's Source names `spec-<stem>`) | `plan/deferrals/<stem>.md` |
| Ad-hoc, with no source spec | `plan/deferrals/ad-hoc-<YYYY-MM-DD>-<sid>.md` |

A shard is the six-column table header plus the rows it owns:

```
| Date | Source | What | Reason | Destination | Status |
```

| Column | Content |
|--------|---------|
| Date | `YYYY-MM-DD` |
| Source | The spec filename, a task description, or `ad-hoc`. It also selects the shard |
| What | The specific work deferred, never a vague category |
| Reason | Why it is deferred |
| Destination | The receiving `plan/spec-*.md`, or `cancelled`, or `user-approved-drop` |
| Status | See below |

| Status | Meaning |
|--------|---------|
| `open` | Live: the work is outstanding, and it names its home spec |
| `deferred` | Live: a synonym of `open` |
| `done` | Terminal. The work landed, or the row was superseded |
| `cancelled` | Terminal. The owner decided not to do it |
| `resolved` | Terminal. Closed with evidence: a journal row, or the commit that landed the work |

| To close as | Set Status to | Set Destination to |
|-------------|---------------|--------------------|
| Implemented | `done` | The spec or commit where it landed |
| The owner decided not to do it | `cancelled` | `user-approved-drop` |
| Superseded | `done` | The row or spec that took it over |

<!-- source: internal/le/hookruntime/lifecycle.go -- hookDeferrals -->
**One observer reads these rows, and it reads only the literal word `open`.**
`hookDeferrals` runs on Stop, globs `plan/deferrals/*.md`, and counts rows whose
Status cell equals `open`, case-insensitively. It prints the count and up to five
What cells to stderr. It never blocks, and it never checks a Destination. A row
written `deferred` is invisible to it. No commit-time gate reads these files at
all, so homing a deferral is an obligation on the author, not something a gate
enforces.

### Naming a deferral holder spec

A spec created only to hold work deferred out of another spec:

```
plan/spec-<source>-deferred-<subtask>.md
```

| Part | Content | Example |
|------|---------|---------|
| `<source>` | The stem of the spec the work came from, without the `spec-` prefix | `bgp-rib-flush` |
| `<subtask>` | Short kebab-case name of the deferred work | `ipv6-coverage` |

Create it from `plan/TEMPLATE.md` with `Status | skeleton`, fill only the
`## Task` section with the points to complete and any constraint already known,
name the source spec there so the provenance survives, then record the row in
`plan/deferrals/<source>.md` with the new spec as Destination. Keep it small: a
skeleton is captured intent, not a designed spec. It moves to `design` when
somebody picks it up.

## The executive summary report

Presented when all the work is complete. The sections are a checklist of what to
cover, never a quota to fill: a section with nothing to report says `None` on one
line, and the whole report stays under about 15 lines.

```
## Executive Summary

**Objective:** [1-2 sentences: what the work aimed to achieve, as understood]

**Changes:**
| File | What changed | Why |
|------|-------------|-----|
| path/file.go | Added X, modified Y | To achieve Z |

**Design decisions:**
- [Decision and reasoning, or "None, all choices were explicit"]

**Deviations:** [From the spec, or "None"]

**Not done:** [Scope boundaries, deferred items, or "N/A"]

**Risks & observations:**
- [Anything noteworthy for future sessions]

**Verification:** [Command run and its result]
```

| Section | Purpose |
|---------|---------|
| Objective | Confirms alignment. If the goal was misunderstood, this is the last chance before it becomes a commit |
| Changes | Per file, with the WHY. `git diff --stat` says "planning.md +8 -5", which is useless; "added the modularity check as step 3" is actionable |
| Design decisions | Choices made during implementation that nobody dictated. The reader should know what was decided for them |
| Deviations | What differed from the spec, and why |
| Not done | The explicit scope boundary, so nothing related is assumed handled |
| Risks & observations | What might bite later. Copy the spec's surviving R-N rows forward rather than inventing this at the end |
| Verification | What ran and what passed: actual output or named tests, not "verify passes" |

## The session handoff

A handoff leads with its rationale so the owner can catch a misaligned handoff
BEFORE the next session applies the edits. Each rationale bullet names a
DECISION, not a fact, and ties to one or more edits below it.

```
RATIONALE (verify this matches what we agreed):
- Decision 1: [what and why] -> EDIT N
- Decision 2: [what and why] -> EDIT N
- Anything still open or assumed: [list]

If any bullet is wrong, STOP and fix the handoff before applying edits.

FILES ALREADY HANDLED (do not re-read): [list]

EDIT 1: [file, with its line range]
- Delete or replace: [exact old text -> new text]

THEN: [test command with a timeout]
```

| Include | Exclude |
|---------|---------|
| The rationale: what was agreed and why, in 3 to 6 bullets | Re-derivation of background research |
| The design decisions the edits encode | File summaries unrelated to the edits |
| A file path and line range per edit | Speculative future work |
| Old text to new text, copy-pasteable | Restatement of the codebase |
| The "do not re-read these" list | |
| The final verification command | |

A handoff that has to survive the chat, for multi-session work or work picked up
days later, is written to `plan/handover/NN-<slug>.md` with the same template.

## Journal rows

`plan/journal/README.md` owns the row format. One row per occurrence, and the
row count is the recurrence.

## Creating a spec

Start from `plan/TEMPLATE.md`: read it, copy its full content, fill the relevant
sections and leave the rest as `(fill during design)`. Writing a spec from memory
loses required section headers, which `hookValidateSpec` then rejects.

The closure half of the lifecycle lives in a second template,
`plan/TEMPLATE-CLOSURE.md`, appended by `/ze-close` at step 1. Do not copy the
closure sections into a new spec. Measured across 161 specs, sections copied at
creation but used only at closure arrived at closure untouched in 65-75% of
in-progress specs, while sections authors added when they needed them were
untouched in 0%. Distance from use is what empties a section.

Placeholders belong to `skeleton` alone. From `design` onward the placeholder
guards in `internal/le/hookruntime/lifecycle.go` block the write, because the
status is a claim that those sections are written.

The Goal Gates name `./le verify worktree` and nothing else. Fast targets are for
the inner iteration loop, never for the gate.

### Risks and assumptions

Every spec carries a `## Risks & Assumptions` section, written during research
and design and kept live through implementation. A concern raised in a `/ze-spec`
gate is written into these tables; gate conversation does not survive the session.

| Table | Captures | Lifecycle |
|-------|----------|-----------|
| Assumptions (A-N) | Beliefs the design depends on, with Basis and a validation method | `unvalidated` to `confirmed` or `broken`. Validate the cheap ones during the `/ze-implement` audit, before coding |
| Risks (R-N) | Failure modes that exist even if the assumptions hold, with an early signal and a mitigation | Reviewed at each phase. A surviving risk copies forward to the Executive Summary, and to the journal row when one is owed |

An assumption with no validation method is a guess: name the test, the grep, or
the user confirmation that would settle it. A `broken` assumption earns a
Deviations entry, and if it invalidates the approved design the work stops and
goes back to the user. No assumption is still `unvalidated` at Pre-Commit
Verification.

## Spec triage

Before working a backlog spec, decide what it is: a defect in the shipped
product, a gap in the evidence that the product is correct, or an improvement.
Only the first two can hold a release. Derive the verdict from the TREE, never
from the spec's own Task or Status: a spec states what its author believed on one
day, and a backlog accumulates specs whose defect another change already fixed. A
spec judged an improvement leaves the release backlog, and a spec whose subject no
longer exists is raised for deletion rather than filed.

## The completion checklist

Once the tests pass, these finish in order, and none is skipped: documentation
updates, the env-var registration check for any new YANG leaf under
`environment/`, the dead-code and file-modularity review of every changed `.go`,
the implementation audit, the Pre-Commit Verification section re-derived from the
spec, the critical review, the spec's own Implementation Summary and Deviations,
the `plan/journal/<class>.md` row naming the spec, `./le verify worktree`, and the
executive summary.

Pre-Commit Verification does not trust the audit. Re-read the spec from scratch,
list every file in "Files to Create", give every acceptance criterion fresh
evidence, read every `.ci` file to confirm it tests the claimed path, and drive
every assumption to `confirmed` or `broken` with evidence.

A spec describing work that is ALREADY implemented runs this checklist
immediately and closes in the same commit as its code. A spec for work that is
already done is never left in `plan/`.

## The implementation audit

Extract every requirement from the spec (task items, AC-N assertions, TDD tests,
files listed), give each one a status, and fill the audit table from
`plan/TEMPLATE.md`.

| Status | What it owes |
|--------|--------------|
| Done | The file and the symbol |
| Partial | What is missing, and the user's answer |
| Skipped | Why, and the user's answer |
| Changed | The deviation. No approval is needed when it is an improvement |

For each acceptance criterion, quote its expected behavior from the AC table,
then name the test and its assertion. The assertion verifies the BEHAVIOR, never
only the mechanism.

Then re-verify every item independently, in the spec's own Pre-Commit
Verification section, with fresh evidence:

| Table | What to verify | How |
|-------|---------------|-----|
| Files Exist | Every file from "Files to Create" | `ls -la <path>`, paste the output |
| AC Verified | Every AC-N | grep, test output, or `ls`, never a copy from the audit |
| Wiring Verified | Every wiring test row | Read the `.ci` file, confirm it tests the claimed path |
| Assumptions Resolved | Every A-N | `confirmed` or `broken` with evidence |
| Documentation Verified | Every Yes/No in the documentation checklist | The edited claim checked against source, or the grep proving no update was needed |

| Claim | Acceptable evidence |
|-------|--------------------|
| Feature works | Test name and output |
| Feature is wired in | A wiring test that exercises the entry-to-feature path |
| AC-N done (wiring) | A functional test name exercising the full path |
| AC-N done (logic) | A unit test name and file whose assertion matches the AC text |
| AC-N done (behavior) | A test that asserts the AC's expected behavior directly |

Every table needs at least one evidence row. `pre_commit_verification_gaps`
(`internal/le/commit`) checks them one at a time and names the empty ones on the
closure commit. Each table is a separate obligation.

## Proving a feature is reachable

A wiring test proves the feature is reachable from its intended entry point:
config, CLI, event dispatch, or plugin launch. For a user-facing feature it is a
`.ci` functional test in `test/`, never a Go unit test. Wiring is the FIRST
implementation step: the spec's `## Wiring Test` table names every entry point
before implementation starts, `/ze-implement` step 4 creates the entry-point
skeleton and a failing wiring test, and `/ze-review` step 1 checks wiring before
any other analysis.

| New code in | Must be called from |
|-------------|---------------------|
| `internal/component/host/` | `cmd/ze/hub/main.go`, `loader_create.go`, `internal/component/cmd/show/system.go`, or `web/page_system.go` |
| `internal/component/config/system/` | `cmd/ze/hub/main.go`, at startup and at reload |
| A new metrics registration | The `loader_create.go` telemetry block |
| A new report bus emission | Verified through `show warnings` and `show errors` |

| Feature type | Required test | Where its `.ci` test lives |
|--------------|---------------|----------------------------|
| Config option | The option affects runtime behavior | `test/parse/` |
| CLI flag or subcommand | The flag changes program behavior | `test/parse/` or `test/ui/` |
| API or RPC | The caller reaches the handler through the real transport | `test/plugin/` |
| Plugin capability | The engine dispatches to the plugin | `test/plugin/` |
| Event or hook | The event fires and the subscriber receives it | `test/plugin/` |
| Wire encoding | Config with a route, verified hex output | `test/encode/` |
| Wire decoding | Hex input, verified JSON output | `test/decode/` |
| YANG config leaf | The env var is registered and appears in `ze env registered` | `test/parse/` |
| Injectable interface | A fake is injected and the component uses it | Per the consuming surface |

Before modifying a handler, a dispatcher, or a protocol step, grep for ALL of its
implementations: Ze has several code paths for one protocol step, and finding *a*
handler is not finding *the* handler the consumer calls. List every
implementation, trace which one each consumer reaches, and change that one.

## Closing a spec

Closure runs in order: `in-progress`, then a clean Review Gate, then the journal
row, then the deletion of the spec. A completed spec left in `plan/` is counted
as open work by every later session.

It takes TWO commits from ONE `./le commit create` script. The spec is edited
throughout implementation, and those edits are design history that a deletion
destroys.

| Commit | Carries |
|--------|---------|
| A | All code, tests, docs, the journal row, and the spec file itself with every implementation edit |
| B | `remove plan/<spec>.md` only, plus `remove plan/deferrals/<spec-stem>.md` when every row in that shard is terminal |

Forcing the deletion of a spec that was never committed discards the uncommitted
edits silently. Commit the spec first, always.

Before commit B, grep the WHOLE PATH `plan/spec-<stem>.md` across the tree, not
the `// Design:` prefix, and repoint every hit inside commit A: an `ai/rules/`
file, a `docs/architecture/` page, or one of `plan/learned/DESIGN-HISTORY.md`,
`plan/learned/HOOK-FRICTION.md`, `plan/learned/RECURRING-PATTERNS.md`. Then
re-read what the substitution produced. A bulk repoint turns a sentence ABOUT the
spec into a sentence about its destination, and some of those become false: a
rule page does not "own rows", is not "active", has no "phase 2b", and is not
somewhere work can be "implemented inside". Grep the new path next to `that spec`,
`this spec`, `owns`, `active` and `Depends`, and rewrite what reads wrong. Naming
the spec WITHOUT its `plan/` path keeps a true sentence about a file that is gone,
because the citation gate matches the path rather than the name.

A spec-to-spec citation has three repairs and the baseline is the last of them:
repoint it at the durable document that replaced the spec, restate the fact
inline, or add the stem to `plan/.citation-baseline` when the citation is a
historical record of the closed spec. All three ride on commit A. A citation with
a live source is repointed or restated; the baseline never absorbs it.
`./le spec citation` passes after the repair.

### The deferral rows closure has to settle

Closure deletes the spec, so a row that names it can never be satisfied, and no
gate reads deferral destinations.

| Direction | What closure does |
|-----------|-------------------|
| A row naming this spec as **Destination**, in any source's shard | Resolve it inside commit A: Status `done`, Destination the journal class file where the knowledge now lives. Resolve to a record only after checking that the record mentions the item |
| A row this spec **sourced**, homed at another spec | It stays live. Its shard survives this closure and keeps its source-keyed name |
| This spec's own shard, every row terminal | `remove` it in commit B: it is residue |
| Another spec's shard whose last live row this closure set to `done` | `remove` it in commit B too. Nobody else will: every other closure scopes its removal to its own stem |

A shard whose source spec is gone while live rows remain is the correct end
state, not mess to sweep. Read the rows before removing one; every signal over
these rows folds across the `plan/deferrals/` directory, so removing a
live-bearing shard lowers the counts instead of raising them.

### Homing a deferral

A destination is chosen at the moment the deferral is made, in this order, and a
new deferral spec is never created before step 1 has been searched.

| Order | Action |
|-------|--------|
| 1 | Search `plan/spec-*.md` and `./le spec status` for a spec that already covers the topic. Prefer a `spec-finish-<subsystem>` or `spec-followup-<subsystem>` umbrella that owns the area |
| 2 | If one exists, add the work to its `## Task` section, and record the row with that spec as Destination, Status `deferred` |
| 3 | Only if no spec covers the topic, create a deferral spec (see "Naming a deferral holder spec"), and record the row the same way |

Both routes record a LIVE row. Filing work in a spec is not finishing it, so the
row stays `deferred` and keeps naming its home until the work lands. A `done` row
is never destination-checked again, so closing one on filing is exactly how the
work stops being watched. A live row is the normal, correct state of a homed
deferral, and closing one early to quiet the Stop-hook count hides the work from
the only thing watching it.

When homing writes a NEW spec for the item, that spec carries the date, the
source, the item and the reason, so the row is duplicate bookkeeping and goes in
the same commit. When homing points at an EXISTING spec, either add the item to
that spec and drop the row, or keep the row, because it is then the only link
between the work and its home.

| Trigger | Row |
|---------|-----|
| Deciding work is "out of scope" | Record with the reason |
| Moving work to another spec | Record with the destination spec |
| Skipping a task item from a spec | Record with the reason |
| Postponing for any reason | Record with the reason |
| The user asks to skip something | Record it: user-requested, still tracked |
| Finding a defect the work in hand does not depend on | NOT a deferral. One row in `plan/journal/<class>.md`, then close the work in hand |

Every row names a `plan/spec-*.md` that exists on disk, whatever its Status; only
`cancelled` and `user-approved-drop` may name none. "later", "future work", "a
follow-up" and "TBD" are not destinations. "Edge cases" is not a What. Record when
the decision is made, never in a batch at commit time. Completing work that was
never in scope, choosing between two valid approaches, and the Go `defer` keyword
are not deferrals.

Before writing "deferred, requires X", grep for X. If it exists, implement it. If
it is genuinely absent, name the specific missing thing and where it would be
added.
