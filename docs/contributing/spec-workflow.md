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
