# Planning

**BLOCKING:** Complete before implementing any non-trivial feature.
Rationale: `ai/rationale/planning.md`

## Spec Selection

One spec at a time per session.

## Plan File Location

Prefer writing a spec (`plan/spec-<task>.md`) over a plan file.

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

- **Style:** Tables and prose, never code (`rules/spec-no-code.md`)
- **Editing:** Append-only. Strikethrough + reason for superseded content.
- **Deletion allowed:** Writing summary to learned, user requests, typo fixes only.
- **Research capture (MUST DO):** All findings from RESEARCH phase go in spec exhaustively — file surveys, function lists, split decisions, reasons for NOT splitting. Spec is single source of truth. Implementation sessions execute from spec alone.

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
[ ] Required Reading has → Decision: / → Constraint: checkpoints
[ ] All research findings captured exhaustively
```

## Retroactive Specs

If a spec describes work that is **already implemented**, run the full Completion Checklist immediately — audit, write summary to `plan/learned/`, include in the same commit as the code. Never commit a spec in `plan/` for work that's already done.

## Completion Checklist

**BLOCKING:** After all tests pass, complete IN ORDER:

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
      Fill the "## Pre-Commit Verification" section in the spec.
      Hook `pre-commit-spec-audit.sh` (exit 2) checks this section is filled.
[ ] 6. Critical Review (BLOCKING — rules/quality.md)
[ ] 7. Review Mistake Log — check MEMORY.md, promote if seen before
[ ] 7. Update spec — Implementation Summary, Documentation Updates, Deviations
[ ] 7. Write learned summary: plan/learned/NNN-<name>.md (see plan/TEMPLATE.md for summary format)
[ ] 7. Verify: `make ze-verify` + git status + git diff, no unintended changes
[ ] 7. Executive Summary Report — present to user with what was done and what is left (including deferred).
        BLOCKING: learned summary (step 10) must exist. Name the file in the report.
        Do NOT ask to commit. The user will tell you when to commit.
[ ] 7. Commit (when user says so) — TWO commits, in order:
        **Commit A:** code + tests + docs + completed spec (with filled audit/verification tables).
        This preserves the completed spec in git history for future review.
        **Commit B:** delete spec (`git rm plan/spec-<name>.md`) + add learned summary (`plan/learned/NNN-<name>.md`).
        The learned summary replaces the spec as the durable artifact.
        Disjoint systems (e.g., CLI and BGP encoding) get separate commit pairs.
```

## Deferred Work (BLOCKING)

See `rules/deferral-tracking.md` for the full deferral process and log format.

**No deferral without a destination.** Work deferred from a spec MUST land in a concrete, existing spec with an explicit task item for this work.

Before marking a spec done, for every deferral: verify the receiving spec exists, has the deferred item listed, and the deferral is recorded in the current spec's Deviations section.

## Executive Summary Report

**BLOCKING:** Present to user when all work is complete. Format below.

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
| Risks & observations | Things that might bite later: new coupling, stale references elsewhere, edge cases not covered, follow-up work needed. |
| Verification | What was run, what passed. Not "make ze-test passes" but actual output or specific test names. |

## Documentation Update Checklist (BLOCKING)

See `rules/documentation.md` for the canonical 12-row checklist.
Every row must be answered Yes/No. Every Yes must name the file and what to add.

## Writing Learned Summaries

When a spec is complete, write a concise summary to `plan/learned/` using the next available number:

```bash
LAST=$(for f in plan/learned/[0-9]*-*.md; do basename "$f" | cut -d- -f1; done 2>/dev/null | sort -n | tail -1)
test -z "$LAST" && LAST=0
NEXT=$(printf "%03d" $((LAST + 1)))
# Write summary to plan/learned/${NEXT}-<name>.md (see TEMPLATE.md for format)
```

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
