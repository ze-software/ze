---
description: Create task specification
argument-hint: <task description>
---

# /prep - Create Task Spec

## Steps

1. Check if spec exists: `plan/spec-<task>.md`
   - If exists: read it, skip to step 5
   - If not: continue

2. Identify relevant docs from `.claude/INDEX.md`

3. Read source code for the affected area

4. Write spec to `plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Task
$ARGUMENTS

## Files to Read
- [relevant source files]
- [relevant .claude/zebgp/ docs]

## Current State
- Tests: [from plan/CLAUDE_CONTINUATION.md]
- Last commit: [hash]

## Implementation Steps
1. Write test (TDD)
2. See test fail
3. Implement
4. See test pass
5. Run `make test && make lint`

## Checklist
- [ ] Test fails first
- [ ] Test passes after impl
- [ ] make test passes
- [ ] make lint passes
```

5. Report ready:
```
✅ Spec: plan/spec-<name>.md
📖 Docs: [list]
🚀 Ready
```

## After Implementation

Move completed spec:
```bash
mv plan/spec-<name>.md plan/done/
```
