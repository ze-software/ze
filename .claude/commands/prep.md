---
description: Create task specification with embedded protocol requirements (project)
argument-hint: <task description>
---

# /prep - Prepare Task Specification

## PHASES 0-1: MANDATORY READING (BLOCKING)

```
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 0: READ PROTOCOLS (do this FIRST)                        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. plan/CLAUDE_CONTINUATION.md - Current state, priorities     │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  PHASE 1: READ INDEX + FIND DOCS                                │
│                                                                 │
│  3. .claude/INDEX.md - Navigation, find what docs to load       │
│  4. Identify task type → find docs from Quick Navigation table  │
│  5. Check if spec exists:                                       │
│     - If plan/spec-<task>.md exists → READ IT, skip to Phase 6  │
│     - If no spec exists → proceed with Phase 2-5 to CREATE spec │
│                                                                 │
│  DO NOT SKIP THIS. DO NOT PROCEED WITHOUT READING PROTOCOLS.    │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Summary

```
Phase 0: READ PROTOCOLS (BLOCKING) - ESSENTIAL_PROTOCOLS.md, CLAUDE_CONTINUATION.md
Phase 1: READ INDEX.md → find what docs to load for task type
Phase 2: Check git status, verify test status
Phase 3: Load context (see CONTEXT_LOADING.md)
Phase 4: Analyze problem, trace user flow
Phase 5: Write spec to plan/spec-<name>.md
Phase 6: Confirm ready to implement
--- AFTER IMPLEMENTATION ---
Phase 7: Move spec to plan/done/, update README
```

---

## Phase 0 Output

```
📖 Protocols loaded:
  ✅ .claude/ESSENTIAL_PROTOCOLS.md
  ✅ plan/CLAUDE_CONTINUATION.md
```

## Phase 1 Output

```
📖 INDEX.md loaded
🎯 Task type: [wire format | API | config | pool | etc.]
📚 Docs to load: [list from INDEX.md Quick Navigation table]
📋 Spec exists: [yes → READ IT, skip to Phase 6 | no → proceed to Phase 2]
```

---

## Phase 2: Session State

```bash
git status && git diff --stat
```
- If modified files: **STOP and ASK user**

```bash
make functional 2>&1 | tail -40
```
- Compare to `plan/CLAUDE_CONTINUATION.md`, update if different

Output:
```
🔍 Git: [clean | N modified - STOPPED]
🧪 Tests: X passed, Y failed
```

---

## Phase 3: Context Loading (BLOCKING)

**Read `.claude/CONTEXT_LOADING.md` for detailed steps.**

Summary:
1. Load protocol rules (ESSENTIAL_PROTOCOLS, QUICK_REFERENCE)
2. Load task-specific architecture docs
3. Read actual source code (with line numbers)
4. Check ExaBGP reference (for BGP code)
5. Output **Context Loading Verification block**

**Cannot proceed without verification block.**

---

## Phase 4: Problem Analysis

After context loaded:
1. Trace user flow end-to-end (config → execution → output)
2. Identify blockers
3. Goal achievement check:

```
🎯 User's actual goal: [what they want]
| Check | Status |
|-------|--------|
| Config works? | ✅/❌ |
| Code works? | ✅/❌ |
| Output correct? | ✅/❌ |
Plan achieves goal: YES/NO
```

---

## Phase 5: Write Specification

Write to `plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## SOURCE FILES (read before implementation)

\`\`\`
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. [relevant source files for this task]                       │
│  2. [relevant .claude design docs - see INDEX.md]               │
│                                                                 │
│  NOTE: Protocol files (.claude/ESSENTIAL_PROTOCOLS.md,          │
│  .claude/INDEX.md, plan/CLAUDE_CONTINUATION.md) should have     │
│  been read at SESSION START, before /prep was invoked.          │
│                                                                 │
│  ON COMPLETION: Update design docs listed in Documentation      │
│  Impact section to match any design changes made.               │
└─────────────────────────────────────────────────────────────────┘
\`\`\`

## Task
$ARGUMENTS

## Current State
- Tests: X passed, Y failed
- Last commit: <hash>

## Context Loaded
[Verification block from Phase 3]

## Problem Analysis
[User flow, blockers, related code]

## Goal Achievement
[Check table, blockers coverage]

## Embedded Rules
- TDD: test must fail before impl
- Verify: make test && make lint before done
- RFC: read RFC first for protocol code

## Documentation Impact
[List docs that need updating if design changes]
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - if API design changes
- [ ] `.claude/zebgp/config/SYNTAX.md` - if config syntax changes
- [ ] `.claude/zebgp/wire/*.md` - if wire format changes
- [ ] `plan/CLAUDE_CONTINUATION.md` - always update after impl

## Implementation Steps
### Phase 1: Tests
### Phase 2: Implementation
### Phase 3: Verification
### Phase 4: Documentation Updates

## Checklist
- [ ] Tests fail first
- [ ] Tests pass after impl
- [ ] make test passes
- [ ] make lint passes
- [ ] Goal achieved
- [ ] Documentation updated (if design changed)
- [ ] Spec moved to plan/done/
- [ ] plan/README.md updated
```

---

## Phase 6: Confirmation

```
✅ Spec written to plan/spec-<task-name>.md

📖 Context loaded:
  - [docs read]
  - [source files with line numbers]

🎯 Goal achievement: YES/NO

🚀 Ready to implement
```

---

## Enforcement

Before claiming /prep done:
1. ✅ Context Loading Verification block complete
2. ✅ Source code read with file:line references
3. ✅ Patterns identified
4. ✅ Goal achievement checked (YES/NO)
5. ✅ Spec written to plan/spec-*.md
6. ✅ Documentation Impact section identifies docs to update

**If any incomplete, DO NOT proceed.**

Before claiming task complete (after implementation):
1. ✅ All checklist items checked
2. ✅ Spec moved to plan/done/
3. ✅ plan/README.md updated
4. ✅ plan/CLAUDE_CONTINUATION.md updated

**If any incomplete, task is NOT complete.**

---

## Documentation Update Rule

**BLOCKING:** When design changes affect architecture, syntax, or behavior:
1. Identify affected docs in "Documentation Impact" section
2. Include doc updates in implementation phases
3. Update docs BEFORE claiming implementation complete

Design changes include:
- New config syntax or semantics
- API interface changes
- Wire format changes
- New message flows or dispatch patterns

---

## Phase 7: Completion (AFTER IMPLEMENTATION)

**Run after implementation is verified, BEFORE committing.**

1. Move spec to done:
```bash
mv plan/spec-<task-name>.md plan/done/
```

2. Update spec checklist to mark all items complete

3. Update `plan/CLAUDE_CONTINUATION.md`:
   - Update status to ready/complete
   - Add to completed work summary

4. Commit implementation + spec together:
```bash
git add pkg/... plan/done/spec-<task-name>.md plan/CLAUDE_CONTINUATION.md
git commit -m "feat/fix(...): description

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

**PROPER WORKFLOW:** Include the completed spec in the same commit as the
implementation. This keeps the spec and code changes atomic and traceable.

Output:
```
✅ Implementation complete
📁 Moved: plan/spec-<task-name>.md → plan/done/
📝 Updated: plan/CLAUDE_CONTINUATION.md
🏁 Task closed
```

**BLOCKING:** Do not claim task complete until spec is in `done/`.
