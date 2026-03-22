# Planning

**BLOCKING:** Complete before implementing any non-trivial feature.
Rationale: `.claude/rationale/planning.md`

## Spec Selection

One spec at a time. Tracked in `.claude/selected-spec` (filename only). Clear after writing summary to `plan/learned/`.

## Plan File Location

Write plan files to project `.claude/plan/ze-plan-<name>`, NOT `~/.claude/plan`. Hook `block-claude-plans.sh` enforces this. Prefer writing a spec (`plan/spec-<task>.md`) over a plan file.

## Pre-Implementation

```
── RESEARCH ── (read, search, understand — no code)
   Gate: Name 3 related files + describe current behavior.

[ ] 1. Check existing spec: plan/spec-<task>.md
[ ] 2. Read .claude/INDEX.md for doc navigation
[ ] 3. Scan plan/spec-*.md for related specs
[ ] 4. Match keywords → docs (INDEX.md tables)
[ ] 5. Read identified architecture docs
[ ] 6. RFC check: verify rfc/short/rfcNNNN.md exists; create if missing
[ ] 7. Read docs/contributing/rfc-implementation-guide.md (protocol work)
[ ] 8. Read ACTUAL source files — document current behavior
      BLOCKING: cannot write spec without "what does existing code do?"
[ ] 9. Trace data flow (rules/data-flow-tracing.md)

── DESIGN ── (write spec, get approval)
[ ] 10. Document existing behavior (preserve unless user says change)
[ ] 11. TDD planning — identify tests BEFORE implementation
[ ] 12. Present plan — WAIT for approval
[ ] 13. Write spec using plan/TEMPLATE.md — complete Pre-Spec Verification first
[ ] 14. git add plan/spec-<task>.md

── IMPLEMENT ── (TDD cycle)
[ ] 15. Test fails → implement → test passes. Log mistakes immediately.

── SELF-REVIEW ── (adversarial, BEFORE presenting to user)
   Gate: Adversarial Self-Review (rules/quality.md) — all 5 questions answered, fixes applied.
[ ] 16. Run adversarial self-review. Fix what it reveals. Do NOT present work yet.
[ ] 17. Check for unanswered questions from earlier in conversation. Re-state them.

── VERIFY ── (complete checklist, present evidence)
[ ] 18. Complete Completion Checklist — all 12 steps, in order, no skipping.
[ ] 19. Present work with evidence. Do NOT suggest committing.
```

## Implementation Plan Format

Present BEFORE writing code. Must include: docs read + insights, current behavior (source files, behavior to preserve/change), TDD plan, implementation phases, files affected, data flow, design decisions, RFC references (protocol code).

**WAIT FOR USER APPROVAL.**

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

### When to Update

| Event | Status change | Phase | Updated |
|-------|--------------|-------|---------|
| Start design | `skeleton` to `design` | - | Yes |
| Design complete | `design` to `ready` | - | Yes |
| Start coding | `ready` to `in-progress` | Set `1/N` | Yes |
| Finish a phase | - | Increment | Yes |
| Blocked | to `blocked` | - | Yes |
| Deferred | to `deferred` | - | Yes |

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
[ ] 2. Dead code check — search unused functions/types, ASK before removing
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
[ ] 8. Update spec — Implementation Summary, Documentation Updates, Deviations
[ ] 9. Write learned summary: plan/learned/NNN-<name>.md (see plan/TEMPLATE.md for summary format)
[ ] 10. Verify: `make ze-verify` + git status + git diff, no unintended changes
[ ] 11. Executive Summary Report — present to user BEFORE asking to commit
        BLOCKING: learned summary (step 9) must exist. Name the file in the report.
[ ] 12. Commit (when user approves) — ALL files in ONE commit
```

## Deferred Work (BLOCKING)

**No deferral without a destination.** Work deferred from a spec MUST land in a concrete, existing spec — not a vague future phase.

| Deferral | Status |
|----------|--------|
| "Deferred to spec-X" and spec-X exists with explicit task item for this work | Allowed |
| "Deferred to Phase N" but no spec for Phase N exists | **Blocked — spec is not done** |
| "Deferred to next spec" with no filename | **Blocked — spec is not done** |
| "Will be handled later" | **Blocked — spec is not done** |

Before marking a spec done, for every deferral:

```
[ ] 1. Receiving spec exists (filename, not "Phase N")
[ ] 2. Receiving spec has explicit task item listing the deferred work
[ ] 3. Deferred item is recorded in current spec's Deviations section with receiving spec filename
```

If the receiving spec does not exist: either do the work now, or create the receiving spec with the deferred items before marking the current spec done.

**The test:** grep the receiving spec for the deferred item. If it's not there, the deferral is a deletion disguised as a postponement.

## Executive Summary Report

**BLOCKING:** Present to user before every commit request. Format below.

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

**Every row must be answered Yes or No.** Every Yes must name the file to update.
"Update the docs" is not an answer. Name the specific file and what to add.

| # | Question | If Yes, update | Examples |
|---|----------|---------------|----------|
| 1 | Does this add a user-facing feature? | `docs/features.md` | New plugin, new config option, new CLI command |
| 2 | Does this change how users configure ze? | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | New config key, changed syntax, new enum value |
| 3 | Does this add or change a CLI command? | `docs/guide/command-reference.md`, `docs/guide/cli.md` | New subcommand, new flag, changed output |
| 4 | Does this add or change an API/RPC? | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` | New RPC, changed params, new event type |
| 5 | Does this add or change a plugin? | `docs/guide/plugins.md`, `docs/plugin-development/` | New plugin, changed SDK, new registration field |
| 6 | Does this have a user guide page? | `docs/guide/<topic>.md` | rpki.md, graceful-restart.md, monitoring.md |
| 7 | Does this change wire format or attributes? | `docs/architecture/wire/messages.md`, `attributes.md`, `nlri.md` | New attribute, changed encoding, new NLRI type |
| 8 | Does this change the plugin SDK or protocol? | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` | New Registration field, new RPC, changed startup |
| 9 | Does this implement or change RFC behavior? | `rfc/short/rfcNNNN.md`, `docs/architecture/rfc-may-decisions.md` | New RFC support, changed compliance |
| 10 | Does this change test infrastructure? | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` | New test tool, new .ci syntax, new test pattern |
| 11 | Does this affect ze vs other daemons comparison? | `docs/comparison.md` | Feature parity change (RPKI, GR, etc.) |
| 12 | Does this change internal architecture? | `docs/architecture/core-design.md` or relevant subsystem doc | Event delivery, plugin lifecycle, config pipeline |

Route for non-doc updates: process rules go to `.claude/rules/`, session knowledge to `memory.md`.

## Writing Learned Summaries

When a spec is complete, write a concise summary to `plan/learned/` using the next available number:

```bash
LAST=`command ls -1 plan/learned/ 2>/dev/null | sort -n | tail -1 | cut -c1-3`
test -z "$LAST" && LAST=0
NEXT=`printf "%03d" \`expr $LAST + 1\``
# Write summary to plan/learned/${NEXT}-<name>.md (see TEMPLATE.md for format)
```

The summary (~20-30 lines) uses this fixed 5-section format:

| Section | Content |
|---------|---------|
| `# NNN — Name` | Title from spec filename |
| `## Objective` | 1-2 sentences: what was the goal |
| `## Decisions` | Bullet points: what was decided and why |
| `## Patterns` | Bullet points: patterns discovered or confirmed |
| `## Gotchas` | Bullet points: what surprised, failed, or trapped (never skip) |
| `## Files` | Key files modified/created |

Quality check: "If I deleted this entry, would a future session miss something that code alone cannot tell them?"
Source: extract from Implementation Summary, Design Insights, Mistake Log, and Deviations sections of the spec.
The original spec file in `plan/` is deleted after the summary is written.
Include the summary in the same commit as code changes.
