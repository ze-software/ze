---
description: Create task specification with embedded protocol requirements (project)
argument-hint: <task description>
---

# /prep - Prepare Task Specification

## Quick Summary

```
Phase 0: Read INDEX.md → find what docs to load
Phase 1: Check git status, verify test status
Phase 2: Load context (see CONTEXT_LOADING.md)
Phase 3: Analyze problem, trace user flow
Phase 4: Write spec to plan/spec-<name>.md
Phase 5: Confirm ready to implement
```

---

## Phase 0: Navigation

Read `.claude/INDEX.md` with Read tool. Output:
```
📖 INDEX.md loaded
🎯 Task type: [wire format | API | config | pool | etc.]
📚 Docs to load: [list from Quick Navigation table]
```

---

## Phase 1: Session State

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

## Phase 2: Context Loading (BLOCKING)

**Read `.claude/CONTEXT_LOADING.md` for detailed steps.**

Summary:
1. Load protocol rules (ESSENTIAL_PROTOCOLS, QUICK_REFERENCE)
2. Load task-specific architecture docs
3. Read actual source code (with line numbers)
4. Check ExaBGP reference (for BGP code)
5. Output **Context Loading Verification block**

**Cannot proceed without verification block.**

---

## Phase 3: Problem Analysis

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

## Phase 4: Write Specification

Write to `plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Task
$ARGUMENTS

## Current State
- Tests: X passed, Y failed
- Last commit: <hash>

## Context Loaded
[Verification block from Phase 2]

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
```

---

## Phase 5: Confirmation

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
