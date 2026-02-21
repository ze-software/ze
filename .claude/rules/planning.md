# Planning Requirements

**BLOCKING:** Complete this process before implementing any non-trivial feature.
Rationale: `.claude/rationale/planning.md`

## Spec Selection

One spec at a time. Tracked in `.claude/selected-spec` (filename only).
Clear after moving to `docs/plan/done/`.

## Pre-Implementation Checklist

```
── RESEARCH ── (read, search, understand — no code)
   Gate: Can name 3 related files + describe current behavior.

[ ] 1. Check existing spec: `docs/plan/spec-<task>.md`
[ ] 2. Read `.claude/INDEX.md` for doc navigation
[ ] 3. Scan `docs/plan/spec-*.md` for related specs
[ ] 4. Match keywords → docs (INDEX.md tables)
[ ] 5. Read identified architecture docs
[ ] 6. RFC check: verify `rfc/short/rfcNNNN.md` exists for each RFC; create if missing
[ ] 7. Read `docs/contributing/rfc-implementation-guide.md` (protocol work)
[ ] 8. Read ACTUAL source files to modify — document current behavior
      BLOCKING: cannot write spec without answering "what does existing code do?"
[ ] 9. Trace data flow (see `rules/data-flow-tracing.md`)

── DESIGN ── (write spec, get approval)
[ ] 10. Document existing behavior in spec (preserve unless user says change)
[ ] 11. TDD planning — identify tests BEFORE implementation
[ ] 12. Present plan to user — WAIT for approval
[ ] 13. Write spec using template: `docs/plan/TEMPLATE.md`
      Complete Pre-Spec Verification first (below)
[ ] 14. `git add docs/plan/spec-<task>.md`

── IMPLEMENT ── (TDD cycle)
[ ] 15. Test fails → implement → test passes. Log mistakes immediately.

── VERIFY ── (audit, docs, completion)
[ ] 16. Complete Completion Checklist (below)
```

## Implementation Plan Format

Present to user BEFORE writing code. Must include:
- Docs read + key insights
- Current behavior (source files read, behavior to preserve/change)
- TDD plan (unit tests, boundary tests, functional tests)
- Implementation phases
- Files affected
- Data flow (entry → transformations → boundaries → integration points)
- Design decisions + principles check
- RFC references (protocol code)

**WAIT FOR USER APPROVAL** before proceeding.

## Spec Rules

- **Style:** Tables and prose, never code (`rules/spec-no-code.md`)
- **Editing:** Append-only. Never delete content. Strikethrough + reason for superseded.
- **Deletion allowed:** Moving to done, user requests, typo fixes only.

## Pre-Spec Verification

```
[ ] 1. INDEX.md keyword table checked
[ ] 2. RFC summaries exist for all referenced RFCs
[ ] 3. Template format followed exactly (🧪 emoji, tables not prose)
[ ] 4. Checkboxes use [ ] not [x]
[ ] 5. No code snippets
[ ] 6. Files to Modify includes feature code, not only tests
[ ] 7. Current Behavior section completed
[ ] 8. Data Flow section completed
[ ] 9. AC-N table rows with testable assertions
[ ] 10. Required Reading has → Decision: / → Constraint: checkpoints
```

## Completion Checklist

**BLOCKING:** After all tests pass, complete IN ORDER:

```
[ ] 1. Review architecture docs — update if learnings improve project
      Route: subsystem → arch doc, process → rules, knowledge → memory.md
      Check: YANG, CLI, editor, plugin SDK docs if affected
[ ] 2. Dead code check — search unused functions/types, ASK before removing
[ ] 3. Implementation Audit (BLOCKING — see rules/implementation-audit.md)
      Every AC, requirement, test, file must have status + location
[ ] 4. Critical Review (BLOCKING — see rules/quality.md Self-Critical Review)
      All 6 checks must pass. Document pass/fail in spec. Failures = fix before continuing.
[ ] 5. Review Mistake Log — check MEMORY.md for recurrence, promote if seen before
[ ] 6. Update spec — Implementation Summary, Documentation Updates, Deviations
[ ] 7. Move spec: docs/plan/done/NNN-<name>.md (number at move time)
[ ] 8. Verify: git status + git diff, no unintended changes
[ ] 9. Commit (when user approves) — ALL files in ONE commit
```

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
