# Planning

**BLOCKING:** Complete before implementing any non-trivial feature.
Rationale: `.claude/rationale/planning.md`

## Spec Selection

One spec at a time. Tracked in `.claude/selected-spec` (filename only). Clear after moving to `docs/plan/done/`.

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

## Completion Checklist

**BLOCKING:** After all tests pass, complete IN ORDER:

```
[ ] 1. Review architecture docs — update if learnings improve project
      Route: subsystem → arch doc, process → rules, knowledge → memory.md
      Check: YANG, CLI, editor, plugin SDK docs if affected
[ ] 2. Dead code check — search unused functions/types, ASK before removing
[ ] 3. Implementation Audit (BLOCKING — rules/implementation-audit.md)
[ ] 4. Critical Review (BLOCKING — rules/quality.md)
[ ] 5. Review Mistake Log — check MEMORY.md, promote if seen before
[ ] 6. Update spec — Implementation Summary, Documentation Updates, Deviations
[ ] 7. Move spec: docs/plan/done/NNN-<name>.md
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
