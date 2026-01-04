---
description: Create task specification (project)
argument-hint: <task description>
---

# /prep - Create Task Spec

**CRITICAL:** You MUST read required docs and present your plan BEFORE implementing. User approval is required.

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

5. **MANDATORY: Present implementation plan to user**

   Before writing spec or code, present a clear summary:
   ```
   ## 📋 Implementation Plan for <task>

   ### What I'll do
   - Phase 1: [description]
   - Phase 2: [description]
   ...

   ### Files affected
   - `pkg/...` - [what changes]

   ### Design decisions
   - [decision 1]: [rationale from docs]
   - [decision 2]: [rationale from docs]

   ❓ [Any clarifying questions]
   ```

   **WAIT FOR USER APPROVAL** before proceeding.

6. Write spec to `plan/spec-<task-name>.md`:

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

## Files to Modify
- [source files]

## Current State
- Tests: [from plan/CLAUDE_CONTINUATION.md]
- Last commit: [hash]

## Implementation Steps
1. Write test (TDD)
2. See test fail
3. Implement
4. See test pass
5. Run `make test && make lint`
6. Add RFC references to new/modified code

## RFC Documentation
For any BGP protocol code, add RFC comments explaining the logic:
- **MUST read `rfc/` folder** to get correct wording and section references
- Add `// RFC NNNN Section X.Y` comments for protocol behavior
- Document wire format encoding/decoding with RFC references
- Key RFCs: 4271 (BGP), 4760 (MP-BGP), 7911 (ADD-PATH), 8277 (Labeled), etc.
- If RFC missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

## Checklist
- [ ] Required docs read
- [ ] Test fails first
- [ ] Test passes after impl
- [ ] make test passes
- [ ] make lint passes
- [ ] RFC references added to protocol code
- [ ] Update `.claude/zebgp/` docs if schema/syntax changed
```

7. Report ready:
```
✅ Spec: plan/spec-<name>.md
📖 Required reading:
   - .claude/zebgp/<doc1>.md
   - .claude/zebgp/<doc2>.md
🔑 Key insight: [one line summary from docs]
🚀 Ready
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
