# Planning

**When:** Complete before implementing any non-trivial feature
**Severity:** blocking

## Directives

Complete before implementing any non-trivial feature.
Rationale: `ai/rationale/planning.md`

## Spec Selection

One spec at a time per session.

## Plan File Location

Prefer writing a spec (`plan/spec-<task>.md`) over a plan file.

## Creating a Spec (BLOCKING)

**Always start from `plan/TEMPLATE.md`.** Read the template, copy its full
content, then fill in relevant sections and leave others as `(fill during
design)` placeholders. Never write a spec from memory -- the `validate-spec`
hook rejects files missing required section headers, and writing from scratch
always misses some. One read of the template before the first Write avoids the
rejected-then-rewrite cycle.

## Pre-Implementation

```
── RESEARCH ── (read, search, understand — no code)
   Gate: Name 3 related files + describe current behavior.

[ ] 1. Check existing spec: plan/spec-<task>.md
[ ] 2. Read ai/INDEX.md for doc navigation
[ ] 3. Scan plan/spec-*.md for related specs
[ ] 4. Match keywords → docs (INDEX.md tables)
[ ] 5. Read identified architecture docs
[ ] 6. RFC check: verify rfc/short/rfcNNNN.md exists; create if missing
[ ] 7. Read docs/contributing/rfc-implementation-guide.md (protocol work)
[ ] 7. Read ACTUAL source files — document current behavior
      BLOCKING: cannot write spec without "what does existing code do?"
[ ] 7. Trace data flow (rules/data-flow-tracing.md)

── DESIGN ── (write spec, get approval)
[ ] 7. Document existing behavior (preserve unless user says change)
[ ] 7. TDD planning — identify tests BEFORE implementation
[ ] 7. Present plan — WAIT for approval
[ ] 7. Write spec using plan/TEMPLATE.md — complete Pre-Spec Verification first

── IMPLEMENT ── (TDD cycle)
[ ] 14. Test fails → implement → test passes. Log mistakes immediately.

── SELF-REVIEW ── (adversarial, BEFORE presenting to user)
   Gate: Adversarial Self-Review (rules/quality.md) — all 5 questions answered, fixes applied.
[ ] 14. Run adversarial self-review. Fix what it reveals. Do NOT present work yet.
[ ] 14. Check for unanswered questions from earlier in conversation. Re-state them.

── VERIFY ── (complete checklist, present evidence)
[ ] 14. Complete Completion Checklist — all 12 steps, in order, no skipping.
[ ] 14. Present work with evidence. Do NOT suggest committing.
```

## Implementation Plan Format

Present BEFORE writing code. Must include: docs read + insights, current behavior (source files, behavior to preserve/change), TDD plan, implementation phases, files affected, data flow, design decisions, RFC references (protocol code).

**WAIT FOR USER APPROVAL.** During design discussions (naming, alternatives, approach),
present options and wait. Never edit files until explicitly approved.

## Spec Rules

- **Style:** Tables and prose, never code (`ai/rules/spec-no-code.md`)
- **Editing:** Append-only. Strikethrough + reason for superseded content.
- **Deletion allowed:** Writing summary to learned, user requests, typo fixes only.
- **Research capture (MUST DO):** All findings from RESEARCH phase go in spec exhaustively — file surveys, function lists, split decisions, reasons for NOT splitting. Spec is single source of truth. Implementation sessions execute from spec alone.

## Risks & Assumptions (BLOCKING for new specs)

Every spec carries a `## Risks & Assumptions` section (see `plan/TEMPLATE.md`) that is
written during RESEARCH/DESIGN and kept live through implementation. Concerns raised at
/ze-spec gates (assumption challenge, Failure Mode Analysis) MUST be written into these
tables, not just spoken at the gate — gate conversation does not survive the session.

| Table | Captures | Lifecycle |
|-------|----------|-----------|
| Assumptions (A-N) | Beliefs the design depends on, with Basis and a validation method | `unvalidated` → `confirmed` or `broken`. Validate cheap ones (grep/read) during the /ze-implement audit, before coding. |
| Risks (R-N) | Failure modes that exist even if assumptions hold, with early signal + mitigation | Reviewed at each phase; surviving risks copy forward to the Executive Summary and learned summary. |

Rules:

- An assumption without a validation method is a guess. Name the test, grep, or user
  confirmation that would settle it.
- A `broken` assumption gets a Mistake Log "Wrong Assumptions" row and a Deviations
  entry. If it invalidates the approved design, STOP and present to the user.
- No assumption may still be `unvalidated` at Pre-Commit Verification (the spec's
  "Assumptions Resolved" table records final status with evidence).
- Existing specs (created before this rule) are exempt; do not retrofit without user request.

## Spec Sets

When multiple specs form a related set (umbrella + child specs), use a shared prefix with numbering:

| Pattern | Example |
|---------|---------|
| Naming | `spec-<prefix>-<N>-<name>.md` |
| Umbrella | `spec-utp-0-umbrella.md` |
| Children | `spec-utp-1-event-format.md`, `spec-utp-2-command-format.md` |
| Done path | `plan/learned/NNN-<prefix>-<N>-<name>.md` |

- **Prefix:** short mnemonic for the effort (e.g., `utp` = unified text protocol)
- **Number:** 0 = umbrella, 1+ = children in execution order
- **Cross-references:** all specs in a set reference siblings by filename
- **Selected spec:** point to the umbrella; select children individually when implementing

## Spec Metadata (BLOCKING)

Every spec MUST have a metadata table immediately after the `# Spec:` title. This is the source of truth for spec status, parsed by `make ze-spec-status` and validated by `validate-spec.sh`.

| Field | Purpose | Values |
|-------|---------|--------|
| Status | Current state | `skeleton`, `design`, `ready`, `in-progress`, `blocked`, `deferred` |
| Depends | Blocking prerequisite | Spec filename (e.g., `spec-rib-04`) or `-` |
| Phase | Multi-phase progress | `N/M` (e.g., `3/5`) or `-` for single-phase |
| Updated | Date of last status change | `YYYY-MM-DD` -- NOT last file edit |

### When to Update (BLOCKING)

Status transitions happen at the BEGINNING of the phase, not at the end.
A spec that stays in `design` during implementation is lying about its state.

| Event | Status change | Phase | Updated | When exactly |
|-------|--------------|-------|---------|--------------|
| Start research | `skeleton` to `design` | - | Yes | When research begins |
| Spec approved | `design` to `ready` | - | Yes | After user approves design |
| Start coding | `ready` to `in-progress` | Set `1/N` | Yes | When coding begins |
| Finish a phase | - | Increment | Yes | After phase tests pass |
| Blocked | to `blocked` | - | Yes | When blocker identified |
| Deferred | to `deferred` | - | Yes | When user agrees to defer |

### Status Vocabulary

| Status | Meaning |
|--------|---------|
| `skeleton` | Task defined, design not started |
| `design` | Research/design in progress |
| `ready` | Design complete, ready for implementation |
| `in-progress` | Actively being implemented |
| `blocked` | Waiting on prerequisite (see Depends) |
| `deferred` | Explicitly postponed |

### Viewing Status

`make ze-spec-status` shows the full inventory table. `make ze-spec-status-json` for machine-readable output.

## Pre-Spec Verification

```
[ ] Metadata table present with valid Status, Depends, Phase, Updated
[ ] INDEX.md keyword table checked
[ ] RFC summaries exist for all referenced RFCs
[ ] Template format followed (🧪 emoji, tables not prose)
[ ] Checkboxes use [ ] not [x]
[ ] No code snippets
[ ] Files to Modify includes feature code, not only tests
[ ] Current Behavior section completed
[ ] Data Flow section completed
[ ] AC-N table rows with testable assertions
[ ] Risks & Assumptions filled — every assumption has Basis + validation method; failure modes recorded as risks
[ ] Required Reading has → Decision: / → Constraint: checkpoints
[ ] All research findings captured exhaustively
[ ] CLI grammar: if adding CLI commands, Integration Checklist marks "CLI grammar" as needed
[ ] Doctor checks: if adding runtime dependencies, Integration Checklist marks "Doctor check" as needed
```

## Retroactive Specs

If a spec describes work that is **already implemented**, run the full Completion Checklist immediately — audit, write summary to `plan/learned/`, include in the same commit as the code. Never commit a spec in `plan/` for work that's already done.

## Completion Checklist

After all tests pass, complete IN ORDER:

```
[ ] 1. Documentation updates — check Documentation Update Checklist below.
      Every question must be answered Yes/No. Every Yes requires a file path.
      BLOCKING: code that changes documented behavior without updating docs is not done.
[ ] 2. Env var check — if YANG config leaves were added under `environment/`,
      verify matching `ze.<name>.<leaf>` env vars are registered via `env.MustRegister()`.
      Run `ze env registered` (or grep for `MustRegister`) to confirm.
[ ] 3. Dead code check — search unused functions/types, ASK before removing
[ ] 3. File modularity check — for each modified .go file:
      Line count: >600 → review concerns, >1000 → split (rules/file-modularity.md)
      // Design: topic annotation still matches file's actual concern?
      If split: copy to new files, adjust annotation per new concern
      // Related: still accurate? Add/update for new couplings
      (rules/design-doc-references.md, rules/related-refs.md)
[ ] 4. Implementation Audit (BLOCKING — rules/implementation-audit.md)
[ ] 5. Pre-Commit Verification (BLOCKING — do NOT trust the audit)
      Re-read spec from scratch. For each item, independently verify:
      - Files Exist: `ls` every file from "Files to Create" — paste output
      - AC Verified: for each AC-N, grep/test for fresh evidence — do NOT copy from audit
      - Wiring Verified: read each .ci file, confirm it tests the claimed path
      - Assumptions Resolved: every A-N row is `confirmed` or `broken` with evidence —
        none `unvalidated`; broken ones have Mistake Log + Deviations entries
      Fill the "## Pre-Commit Verification" section in the spec.
      Hook `pre-commit-spec-audit.sh` (exit 2) checks this section is filled.
[ ] 6. Critical Review (BLOCKING — rules/quality.md)
[ ] 7. Review Mistake Log — check MEMORY.md, promote if seen before
[ ] 7. Update spec — Implementation Summary, Documentation Updates, Deviations
[ ] 7. Write learned summary: plan/learned/NNN-<name>.md (allocate NNN with `scripts/dev/commit_helper.py learned-next <slug>`)
[ ] 7. Verify: `make ze-verify` + git status + git diff, no unintended changes
[ ] 7. Executive Summary Report — present to user with what was done and what is left (including deferred).
        BLOCKING: learned summary (step 10) must exist. Name the file in the report.
        Do NOT ask to commit. The user will tell you when to commit.
[ ] 7. Commit (when user says so) -- ONE helper-generated script, TWO commits (per Spec Closure above):
        - **Commit A:** `scripts/dev/commit_helper.py create --replace` with `--file` for all implementation files (code, tests, docs, schema)
          + `--file plan/learned/NNN-<name>.md` + `--file plan/spec-<name>.md` (preserves edits)
          + include `--file ai/LEARNED-INDEX.md` if updated
        - **Commit B:** `scripts/dev/commit_helper.py create --append --remove plan/spec-<name>.md` (spec closure)
        Run the generated script yourself and the work is done. There is no
        second step. If spec closure or learned summary is missing, it never happens.
        Disjoint systems (e.g., CLI and BGP encoding) get separate commits.
```

## Review Gate (BLOCKING)

Before final testing/verify, run a code review against the diff. Fill the
`## Review Gate` section in the spec with the findings list. If ANY finding
is severity BLOCKER or ISSUE (anything above NOTE), fix it and re-run the
review. Loop until the review returns only NOTEs (or nothing). Paste the
final clean review output into the spec. NOTE-only findings do NOT block.

## Spec Closure (BLOCKING)

**A spec that passes its Review Gate is not done until it is deleted from `plan/`.**

The lifecycle is: `in-progress` -> Review Gate clean -> write learned summary -> `git rm` spec.
Leaving a completed spec in `plan/` causes every future session to count it as open work.

**TWO commits, ONE script.** The spec is edited during implementation (design notes,
status updates, corrected assumptions). Those edits are valuable design history.
`git rm` destroys the working copy. If the edited spec is never committed before
deletion, the design work is lost from git history forever.

The helper-generated commit script MUST produce two commits:
1. **Commit A (implementation + spec):** `scripts/dev/commit_helper.py create --replace`
   with `--file` for all code, tests, docs, learned summary, LEARNED-INDEX,
   AND the spec file itself (with all edits from implementation).
2. **Commit B (spec closure):** `scripts/dev/commit_helper.py create --append --remove plan/<spec>` only.

This preserves the final spec state in git history. `git log -p -- plan/<spec>` shows
the full design record. The deletion in commit B is a clean removal of a file whose
final state is already committed.

**Design references survive closure.** Before commit B, grep
`// Design: plan/<spec>` across the tree and rewrite every hit to the learned
summary (`plan/learned/NNN-<name>.md`) inside commit A. Deleting a spec that
source files still reference breaks design traceability;
`scripts/dev/check_doc_links.py --design-only` reports the breakage.

**Closure resolves the spec's deferral rows.** Before commit B, grep
`plan/deferrals.md` for this spec's filename. Every row naming it as **Destination**
must be resolved inside commit A: set Status `done` and Destination to the learned
summary (`plan/learned/NNN-<name>.md`), which is where the knowledge now lives.

Why: closure DELETES the spec, and `deferral_unassigned_problems`
(`scripts/dev/commit_helper.py`) checks that every live row's destination exists on
disk. A row left pointing at a closed spec can therefore never be satisfied -- it
dangles forever and blocks every future commit, and the next reader cannot tell
whether the work was done or silently lost. The two rules collided precisely because
neither side was written down: "destination must exist" and "closure deletes the
spec" are both right, and closure is the side that must give.

Resolving the row is not a claim that the deferred work was implemented. It records
that the deferral has a permanent home. If the learned summary does NOT record the
item (check, do not assume -- a Review Gate NOTE on a deleted spec evaporates, and a
summary with no Known Limitations section records nothing), then the row has no home:
keep it live and give it a real destination spec per `ai/rules/deferral-tracking.md`
"Choosing the Destination Spec". Never resolve a row to a summary that does not
mention it -- that is the fail-open the gate exists to catch
(`ai/rules/fail-closed-guards.md`).

**Never `git rm -f` a spec without committing it first.** The `-f` flag silently
discards uncommitted edits. If the spec was modified during implementation (it
almost always is), those modifications must be committed before deletion.

| Banned | Why |
|--------|-----|
| "I'll close it later" | Later never comes. Other sessions see it as in-progress. |
| `git rm` a spec while a deferral row still names it as Destination | The row dangles forever and blocks every future commit. Resolve it in commit A. |
| Resolving a row to a learned summary that never mentions the item | Fail-open bookkeeping: the row goes quiet and the knowledge is lost. Verify the summary records it. |
| "The user will handle it" | The user asked us to implement. Closure is part of implementation. |
| `git rm` in the same commit as implementation | Spec edits are lost from history. Two commits required. |
| `git rm -f` without a prior commit of the spec | Destroys uncommitted design work. |
| "Run the commit, then I'll prepare closure" | The user will not ask. One script, one run, done. |

### Closure Enforcement (automated)

Closure once depended on remembering the two-commit step, so it was routinely
dropped and specs piled up in `plan/` as false "open work". Three mechanical
gates now enforce it (registered in `ai/rules/hook-mapping.md`):

| Gate | Where | Fires when |
|------|-------|-----------|
| Detector | `scripts/dev/spec-closure-check.py` | `--list` reports completed-but-not-closed specs in two tiers; `--spec <s>` exits 3 only for a high-confidence one. High confidence = a **committed** `plan/learned/NNN-<slug>.md` whose slug **exactly equals** the spec stem while the spec is still `in-progress` and is **not an umbrella** (commit A ran, commit B did not). Weaker `[umbrella]` / `[weak-match]` candidates (child/sibling/predecessor summaries) are listed under NEEDS VERIFICATION and must be audited before closing — they are usually false positives. Only the high-confidence set triggers the `--spec` block. |
| Stop-hook block | `.claude/hooks/block-premature-stop.sh` | This session's claimed spec is completed-but-not-closed: the session cannot end (exit 2) until commit B runs, or `tmp/session/.closure-ack-<stem>` records why the spec is genuinely still open. |
| Commit reminder | `scripts/dev/commit_helper.py` | A commit adds a learned summary but removes no spec: it prints the closure-commit reminder to stderr. |

Run `scripts/dev/spec-closure-check.py --list` any time to see the backlog.

## Verify Specs Against Code (BLOCKING)

Never report spec progress by reading the spec alone. Grep the codebase to verify
claims. Spec "What Remains" and "Implementation Summary" sections go stale. Before
reporting any item as unimplemented, search for the function/type/test in the code.
If it exists, the spec is stale, not the code. Update the spec to match reality.

## Deferred Work (BLOCKING)

See `ai/rules/deferral-tracking.md` for the full deferral process and log format.

**No deferral without a destination spec.** Work deferred from a spec MUST land in a concrete, existing spec with an explicit task item for this work. Search for one that already covers the topic first (`grep -l "<topic>" plan/spec-*.md`, `make ze-spec-status`) and prefer it. Only if none exists, create a skeleton (`Status | skeleton`, from `plan/TEMPLATE.md`) named `plan/spec-<source>-deferred-<subtask>.md`, where `<source>` is the stem of the spec the work came from -- see `ai/rules/deferral-tracking.md` "Choosing the Destination Spec".

Before marking a spec done, for every deferral: verify the receiving spec exists, has the deferred item listed, and the deferral is recorded in the current spec's Deviations section.

## Executive Summary Report

Present to user when all work is complete. Format below.

```
## Executive Summary

**Objective:** [1-2 sentences — what the work aimed to achieve, as understood]

**Changes:**
| File | What changed | Why |
|------|-------------|-----|
| path/file.go | Added X, modified Y | To achieve Z |

**Design decisions:**
- [Decision and reasoning, or "None — all choices were explicit"]

**Deviations:** [From spec/plan/instructions, or "None"]

**Not done:** [Scope boundaries, deferred items, or "N/A"]

**Risks & observations:**
- [Anything noteworthy for future sessions]

**Verification:** [Command run + result summary]
```

| Section | Purpose |
|---------|---------|
| Objective | Confirms alignment. If the goal was misunderstood, this is the last chance before it becomes a commit. |
| Changes | Per-file summary with *why*, not just *what*. `git diff --stat` says "planning.md +8 -5" — useless. "Added modularity check as step 3, renumbered 4-10" — actionable. |
| Design decisions | Choices made during implementation that weren't explicitly dictated. The user should know what was decided on their behalf. "None — all choices were explicit" is valid. |
| Deviations | What differed from spec/plan/instructions and why. "None" is valid. |
| Not done | Explicit scope boundary. Prevents the assumption that everything related was handled. Surfaces deferred items. |
| Risks & observations | Things that might bite later: new coupling, stale references elsewhere, edge cases not covered, follow-up work needed. Start from the spec's Risks table (R-N rows that survived implementation) — this section is a copy-forward, not an invention at the end. |
| Verification | What was run, what passed. Not "make ze-test passes" but actual output or specific test names. |

## Documentation Update Checklist (BLOCKING)

See `ai/rules/documentation.md` for the canonical 12-row checklist.
Every row must be answered Yes/No. Every Yes must name the file and what to add.

## Writing Learned Summaries

When a spec is complete, write a concise summary to `plan/learned/` using the next available number.
Allocate the number with `scripts/dev/commit_helper.py learned-next <slug>`: it takes max(existing `plan/learned/NNNN-*.md` prefixes) + 1 and creates the file immediately, so concurrent sessions in one tree cannot collide. Include the created file in the Commit A helper command.

The summary (~25-35 lines) uses this fixed 5-section format:

| Section | Content |
|---------|---------|
| `# NNN -- Name` | Title from spec filename |
| `## Context` | Short paragraph (3-5 sentences): what problem existed, what was the symptom, what was the goal |
| `## Decisions` | Bullet points: what was decided, what was rejected, and why |
| `## Consequences` | Bullet points: what this enables, constrains, or changes going forward |
| `## Gotchas` | Bullet points: what surprised, failed, or trapped (never skip) |
| `## Files` | Key files modified/created |

**Context** replaces Objective. It preserves the spec's Task section: the problem, the symptom, the goal. Quality check: "Could a future reader reconstruct *why this work was worth doing* from this section alone?"

**Decisions** must include "over" clauses when alternatives were considered: "chose X over Y because Z."

**Consequences** captures forward-looking impact: capabilities unlocked, constraints accepted, future work this interacts with. Quality check: "If someone touches this area next, what do they need to know that the code alone won't tell them?"

General quality check: "If I deleted this entry, would a future session miss something that code alone cannot tell them?"
Source: extract from Task, Implementation Summary, Design Insights, Mistake Log, and Deviations sections of the spec.
The original spec file in `plan/` is deleted after the summary is written.
Include the summary in the same commit as code changes.
