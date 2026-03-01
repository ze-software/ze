# Planning

**BLOCKING:** Complete before implementing any non-trivial feature.
Rationale: `.claude/rationale/planning.md`

## Spec Selection

One spec at a time. Tracked in `.claude/selected-spec` (filename only). Clear after moving to `docs/plan/done/`.

## Plan File Location

Write plan files to project `.claude/plan/ze-plan-<name>`, NOT `~/.claude/plan`. Hook `block-claude-plans.sh` enforces this. Prefer writing a spec (`docs/plan/spec-<task>.md`) over a plan file.

## Pre-Implementation

```
── RESEARCH ── (read, search, understand — no code)
   Gate: Name 3 related files + describe current behavior.

[ ] 1. Check existing spec: docs/plan/spec-<task>.md
[ ] 2. Read .claude/INDEX.md for doc navigation
[ ] 3. Scan docs/plan/spec-*.md for related specs
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
[ ] 13. Write spec using docs/plan/TEMPLATE.md — complete Pre-Spec Verification first
[ ] 14. git add docs/plan/spec-<task>.md

── IMPLEMENT ── (TDD cycle)
[ ] 15. Test fails → implement → test passes. Log mistakes immediately.

── VERIFY ── (audit, docs, completion)
[ ] 16. Complete Completion Checklist
```

## Implementation Plan Format

Present BEFORE writing code. Must include: docs read + insights, current behavior (source files, behavior to preserve/change), TDD plan, implementation phases, files affected, data flow, design decisions, RFC references (protocol code).

**WAIT FOR USER APPROVAL.**

## Spec Rules

- **Style:** Tables and prose, never code (`rules/spec-no-code.md`)
- **Editing:** Append-only. Strikethrough + reason for superseded content.
- **Deletion allowed:** Moving to done, user requests, typo fixes only.
- **Research capture (MUST DO):** All findings from RESEARCH phase go in spec exhaustively — file surveys, function lists, split decisions, reasons for NOT splitting. Spec is single source of truth. Implementation sessions execute from spec alone.

## Spec Sets

When multiple specs form a related set (umbrella + child specs), use a shared prefix with numbering:

| Pattern | Example |
|---------|---------|
| Naming | `spec-<prefix>-<N>-<name>.md` |
| Umbrella | `spec-utp-0-umbrella.md` |
| Children | `spec-utp-1-event-format.md`, `spec-utp-2-command-format.md` |
| Done path | `docs/plan/done/NNN-<prefix>-<N>-<name>.md` |

- **Prefix:** short mnemonic for the effort (e.g., `utp` = unified text protocol)
- **Number:** 0 = umbrella, 1+ = children in execution order
- **Cross-references:** all specs in a set reference siblings by filename
- **Selected spec:** point to the umbrella; select children individually when implementing

## Pre-Spec Verification

```
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

If a spec describes work that is **already implemented**, run the full Completion Checklist immediately — audit, move to `done/`, include in the same commit as the code. Never commit a spec in `docs/plan/` for work that's already done.

## Completion Checklist

**BLOCKING:** After all tests pass, complete IN ORDER:

```
[ ] 1. Architecture docs — check Post-Implementation Updates table below.
      If code changed documented behavior, docs MUST match. Not optional.
      Route: subsystem → arch doc, process → rules, knowledge → memory.md
[ ] 2. Dead code check — search unused functions/types, ASK before removing
[ ] 3. File modularity check — for each modified .go file:
      Line count: >600 → review concerns, >1000 → split (rules/file-modularity.md)
      // Design: topic annotation still matches file's actual concern?
      If split: copy to new files, adjust annotation per new concern
      // Related: still accurate? Add/update for new couplings
      (rules/design-doc-references.md, rules/related-refs.md)
[ ] 4. Implementation Audit (BLOCKING — rules/implementation-audit.md)
[ ] 5. Critical Review (BLOCKING — rules/quality.md)
[ ] 6. Review Mistake Log — check MEMORY.md, promote if seen before
[ ] 7. Update spec — Implementation Summary, Documentation Updates, Deviations
[ ] 8. Move spec: docs/plan/done/NNN-<name>.md
[ ] 9. Verify: `make test-all` + git status + git diff, no unintended changes
[ ] 10. Executive Summary Report — present to user BEFORE asking to commit
[ ] 11. Commit (when user approves) — ALL files in ONE commit
```

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
| Verification | What was run, what passed. Not "make test-all passes" but actual output or specific test names. |

## Post-Implementation Updates

| Changed | Update |
|---------|--------|
| Config schema | `docs/architecture/config/syntax.md` |
| Wire format | `docs/architecture/wire/messages.md`, `attributes.md` |
| NLRI types | `docs/architecture/wire/nlri.md` |
| Capabilities | `docs/architecture/wire/capabilities.md` |
| UPDATE building | `docs/architecture/update-building.md` |
| Pool/memory | `docs/architecture/pool-architecture.md` |
| API commands | `docs/architecture/api/architecture.md` |
| RPCs (plugin↔engine) | YANG schema + RPC count in arch docs |
| RPCs (user-facing) | YANG domain schema + handler registration |
| CLI commands/flags | `cmd/ze/` dispatch + usage + commands.md |
| Plugin SDK methods | `.claude/rules/plugin-design.md` SDK tables |
| Test format (.ci) | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |

## Moving Completed Specs

```bash
LAST=`command ls -1 docs/plan/done/ 2>/dev/null | sort -n | tail -1 | cut -c1-3`
test -z "$LAST" && LAST=0
NEXT=`printf "%03d" \`expr $LAST + 1\``
mv docs/plan/spec-<name>.md docs/plan/done/${NEXT}-<name>.md
```

Include moved spec in same commit as code changes.
