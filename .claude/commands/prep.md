---
description: Create task specification (project)
argument-hint: <task description>
---

# /prep - Create Task Spec

**CRITICAL:** You MUST read required docs and present your plan BEFORE implementing. User approval is required.

## Pre-Flight Checklist

Complete each item IN ORDER. Do not skip ahead.

```
[ ] 1. Check for existing spec: `plan/spec-<task>.md`
      → If exists: read it, skip to "Report ready"
      → If not: continue

[ ] 2. Read `.claude/INDEX.md`
      → List docs identified: ___

[ ] 3. Scan `plan/spec-*.md` for related specs
      → Related specs found: ___ (or "none")

[ ] 4. Match task keywords to doc table below
      → Docs to read: ___

[ ] 5. Read ALL identified docs (step 2 + step 4)
      → Key insights: ___

[ ] 6. Read source code for affected area
      → Files reviewed: ___

[ ] 7. **TDD PLANNING** - Identify tests BEFORE implementation
      → Unit tests needed: ___
      → Functional tests needed: ___
      → Test file locations: ___

[ ] 8. Present implementation plan to user (including test plan)
      → WAIT for approval before continuing

[ ] 9. Write spec to `plan/spec-<task>.md`

[ ] 10. Report ready
```

**STOP at each checkbox. Do not proceed until complete.**
**UPDATE the spec file after completing each step to track progress.**

## Steps

1. Check if spec exists: `plan/spec-<task>.md`
   - If exists: read it, skip to step 7
   - If not: continue

2. **MANDATORY: Identify relevant docs**

   Check these sources for relevant documentation:
   - `plan/` - specs and design docs (e.g., `DESIGN_TRANSITION.md`, `spec-*.md`)
   - `.claude/zebgp/` - architecture docs
   - `.claude/INDEX.md` - doc index

   Use this keyword mapping:

   | Keywords in task | Required docs |
   |------------------|---------------|
   | UPDATE, message, build, route, announce | `UPDATE_BUILDING.md`, `ENCODING_CONTEXT.md` |
   | attribute, community, AS_PATH, NEXT_HOP | `wire/ATTRIBUTES.md`, `UPDATE_BUILDING.md` |
   | NLRI, prefix, MP_REACH, MP_UNREACH | `wire/NLRI.md` |
   | capability, OPEN, negotiate | `wire/CAPABILITIES.md` |
   | pool, memory, dedup, zero-copy | `POOL_ARCHITECTURE.md`, `ENCODING_CONTEXT.md` |
   | forward, reflect, wire cache | `ENCODING_CONTEXT.md`, `UPDATE_BUILDING.md` |
   | FSM, state, session, peer | `behavior/FSM.md` |
   | API, command, announce, withdraw | `api/ARCHITECTURE.md` |
   | config, YAML, load | `config/SYNTAX.md` |
   | FlowSpec, VPN, EVPN, MPLS | `wire/NLRI.md`, `UPDATE_BUILDING.md` |
   | ExaBGP, compatibility | `EXABGP_CODE_MAP.md` |
   | design, transition, architecture | `plan/DESIGN_TRANSITION.md` |

3. **MANDATORY: Read the identified docs NOW**

   Use the Read tool to read each relevant doc.
   Do NOT proceed until you have read them.

4. Read source code for the affected area

5. **MANDATORY: TDD Planning**

   Identify ALL tests that must be written BEFORE implementation:
   - Unit tests: What functions/methods need tests?
   - Functional tests: What end-to-end scenarios need testing?
   - Where do test files go? (existing `*_test.go` or new file?)

6. **MANDATORY: Present implementation plan to user**

   Before writing spec or code, present a clear summary:
   ```
   ## 📋 Implementation Plan for <task>

   ### 🧪 Tests First (TDD)
   **Unit tests:**
   - `pkg/.../xxx_test.go` - TestXxx: [what it validates]

   **Functional tests:** (if needed)
   - `qa/tests/xxx/` - [scenario description]

   ### What I'll do
   - Phase 1: Write tests, verify they FAIL
   - Phase 2: Implement minimal code to pass tests
   - Phase 3: [additional phases...]

   ### Files affected
   - `pkg/...` - [what changes]

   ### Design decisions
   - [decision 1]: [rationale from docs]
   - [decision 2]: [rationale from docs]

   ❓ [Any clarifying questions]
   ```

   **WAIT FOR USER APPROVAL** before proceeding.

7. Write spec to `plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Task
$ARGUMENTS

## Required Reading (MUST complete before implementation)

The following docs MUST be read before starting implementation:

- [ ] `.claude/zebgp/<doc1>.md` - [reason this is relevant]
- [ ] `.claude/zebgp/<doc2>.md` - [reason this is relevant]

**Key insights from docs:**
- [insight 1 relevant to this task]
- [insight 2 relevant to this task]

## 🧪 TDD Test Plan (MANDATORY - Write tests FIRST)

### Unit Tests
| Test | File | What it validates |
|------|------|-------------------|
| `TestXxx` | `pkg/.../xxx_test.go` | [description] |

### Functional Tests (if needed)
| Test | Location | Scenario |
|------|----------|----------|
| `test-xxx` | `qa/tests/xxx/` | [description] |

## Files to Modify
- [source files]

## Current State
- Tests: [from plan/CLAUDE_CONTINUATION.md]
- Last commit: [hash]

## Implementation Steps
1. **Write tests** - Create unit tests (and functional tests if needed)
2. **Run tests** - Verify they FAIL (paste output as proof)
3. **Implement** - Write minimal code to pass tests
4. **Run tests** - Verify they PASS (paste output as proof)
5. **Verify all** - Run `make test && make lint && make functional`
6. **Add RFC refs** - Add RFC references to new/modified code

**IMPORTANT:** Update this spec's progress after completing each step (check boxes, add notes).

## RFC Documentation
For any BGP protocol code, add RFC comments explaining the logic:
- **MUST read `rfc/` folder** to get correct wording and section references
- Add `// RFC NNNN Section X.Y` comments for protocol behavior
- Document wire format encoding/decoding with RFC references
- Key RFCs: 4271 (BGP), 4760 (MP-BGP), 7911 (ADD-PATH), 8277 (Labeled), etc.
- If RFC missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

## Checklist

### 🧪 TDD (MUST complete in order)
- [ ] Unit tests written
- [ ] Functional tests written (if needed)
- [ ] Tests run and FAIL (paste output below)
- [ ] Implementation complete
- [ ] Tests run and PASS (paste output below)

### Verification
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC references added to protocol code
- [ ] Update `.claude/zebgp/` docs if schema/syntax changed

### Completion
- [ ] Move spec to `plan/done/NNN-<name>.md` (pick NNN at move time, not before)

**UPDATE PROGRESS:** Check boxes and add notes as you complete each step above.
```

8. Report ready:
```
✅ Spec: plan/spec-<name>.md
📖 Required reading:
   - .claude/zebgp/<doc1>.md
   - .claude/zebgp/<doc2>.md
🧪 Tests planned:
   - [unit test files]
   - [functional test dirs if any]
🔑 Key insight: [one line summary from docs]
🚀 Ready - TDD: write tests first, see them fail, then implement
```

## After Implementation

### Update Docs (if applicable)

If the task changed any of these, update the corresponding `.claude/zebgp/` doc:

| Changed | Update |
|---------|--------|
| Config schema (new fields/blocks) | `config/SYNTAX.md` |
| Wire format (messages, attributes) | `wire/MESSAGES.md`, `wire/ATTRIBUTES.md` |
| NLRI types | `wire/NLRI.md` |
| Capabilities | `wire/CAPABILITIES.md` |
| UPDATE building | `UPDATE_BUILDING.md` |
| Pool/memory | `POOL_ARCHITECTURE.md` |
| API commands | `api/ARCHITECTURE.md` |

### Update Design Docs

If the implementation changed how ZeBGP works, update `.claude/zebgp/` docs:
- Document new behavior, APIs, or architectural patterns
- Keep docs in sync with actual implementation

### Move Completed Spec

**IMPORTANT:** Only determine the spec number NOW, at move time. Do NOT pre-assign numbers during spec creation.

Find next free 3-digit number (starting from 001) and move:
```bash
# Find next number (check existing in plan/done/, start at 001 if empty)
LAST=$(ls plan/done/ 2>/dev/null | grep -E "^[0-9]{3}-" | sort | tail -1 | cut -c1-3)
NEXT=$(printf "%03d" $((10#${LAST:-0} + 1)))
# Move with number prefix
mv plan/spec-<name>.md plan/done/${NEXT}-<name>.md
# Commit the completion
git add plan/done/${NEXT}-<name>.md && git commit -m "docs: complete spec ${NEXT}-<name>"
```

## Why This Matters

Reading architecture docs BEFORE implementation prevents:
- Uninformed "critical reviews" that miss intentional design
- Duplicate work rediscovering existing patterns
- Wrong assumptions about "bugs" that are actually by-design
- Wasted effort on changes that don't fit the architecture
