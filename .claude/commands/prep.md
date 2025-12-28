---
description: Create task specification with embedded protocol requirements (project)
argument-hint: <task description>
---

# /prep - Prepare Task Specification

Create a task specification with embedded protocol requirements.

## Instructions

When this skill is invoked:

### Step 0: Verify Current State (MANDATORY - PREVENTS STALE INFO)

**WHY:** The continuation file gets out of sync. Trusting it without verification
causes incorrect information to propagate session after session. This step FORCES
verification before any planning work.

1. **Run functional tests:**
   ```bash
   go run ./test/cmd/functional encoding --all 2>&1 | tail -30
   ```

2. **Extract actual status** from test output (passed/failed counts, which tests failed)

3. **Read continuation file:**
   ```bash
   cat plan/CLAUDE_CONTINUATION.md
   ```

4. **Compare and update if different:**
   - If test results differ from what's documented, UPDATE the continuation file
   - Update the "Last verified" date
   - Update the pass/fail counts
   - Update the list of failing tests with actual error messages

5. **Report to user:**
   ```
   🔍 Verified test status: X passed, Y failed
   📋 Continuation file: [up-to-date | UPDATED]
   ```

**DO NOT SKIP THIS STEP.** The whole point is to prevent trusting stale docs.

### Step 1: Read Protocols (MANDATORY)

Read these files in order:

1. `.claude/ESSENTIAL_PROTOCOLS.md` - ALWAYS read this first
2. Based on task keywords, read additional files (see table below)

### Step 2: Detect Task Type

| Keywords in Task | Additional Files to Read |
|------------------|--------------------------|
| implement, add, create, feature, new | `.claude/TDD_ENFORCEMENT.md`, `.claude/CODING_STANDARDS.md` |
| test, fix test, failing, coverage | `.claude/TESTING_PROTOCOL.md`, `.claude/CI_TESTING.md` |
| RFC, protocol, compliance | `.claude/RFC_DOCUMENTATION_PROTOCOL.md`, read `rfc/rfcNNNN.txt` |
| ExaBGP, exabgp, compatibility | Check `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/` for reference implementation |
| API, endpoint, command | `.claude/zebgp/api/COMMANDS.md` |
| FSM, state, session, peer | `.claude/zebgp/behavior/FSM.md` |
| wire, message, parse, encode, decode | `.claude/zebgp/wire/MESSAGES.md`, `.claude/RFC_DOCUMENTATION_PROTOCOL.md` |
| attribute, path attribute | `.claude/zebgp/wire/ATTRIBUTES.md` |
| NLRI, prefix, route | `.claude/zebgp/wire/NLRI.md` |
| capability, open | `.claude/zebgp/wire/CAPABILITIES.md` |
| config, configuration | `.claude/zebgp/config/SYNTAX.md` |
| refactor, rename, move | (already in ESSENTIAL after consolidation) |
| git, commit, push | (already in ESSENTIAL after consolidation) |

### Step 3: Extract Key Rules (3-5 per protocol)

From each file read, extract **only 3-5 key rules** that apply to this task.

**DO NOT paste entire files.** Extract the most critical rules.

### Step 4: Generate Specification

Write the specification to `plan/spec-<task-name>.md` using this format:

```markdown
# Spec: <task-name>

## Task
$ARGUMENTS

## Current State (verified)
- Functional tests: X passed, Y failed
- Failing: [list of failing test codes]
- Last commit: <hash>

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference in `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- <3-5 key rules relevant to this task>

### From <other-protocol>.md
- <3-5 key rules if additional protocols were read>

## Codebase Context
- <relevant existing files to read/understand first>
- <patterns to follow from existing code>

## Implementation Steps
1. <specific step with clear deliverable>
2. <specific step with clear deliverable>
...

## Verification Checklist
- [ ] Tests written and shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] <task-specific verification>
```

### Step 5: Confirm

After writing the spec, confirm:
```
✅ Spec written to plan/spec-<task-name>.md
📋 Protocols read: <list>
🔍 Test status verified: X passed, Y failed
🎯 Ready to implement
```

---

## Examples

### Example 1: `/prep implement AS path validation`

Step 0 output:
```
🔍 Verified test status: 28 passed, 9 failed
📋 Continuation file: up-to-date
```

Protocols to read:
- ESSENTIAL_PROTOCOLS.md (always)
- TDD_ENFORCEMENT.md (keyword: implement)
- CODING_STANDARDS.md (keyword: implement)
- zebgp/wire/ATTRIBUTES.md (keyword: path)

Output spec includes:
- Current state (verified test counts)
- Default rules (TDD, verification, work preservation)
- Key TDD rules (test first, VALIDATES/PREVENTS docs)
- Key coding rules (error handling, no panic)
- Key attribute parsing rules

### Example 2: `/prep fix failing test N`

Step 0 output:
```
🔍 Verified test status: 28 passed, 9 failed
📋 Continuation file: UPDATED (was showing 25 passed)
```

Protocols to read:
- ESSENTIAL_PROTOCOLS.md (always)
- TESTING_PROTOCOL.md (keyword: test, failing)

Output spec includes:
- Current state showing test N actually fails
- Actual error message from test run
- Key testing rules

---

## Why This Matters

This skill exists because:

1. **Claude skips reading protocol files "on demand"** - By forcing protocol reading
   as part of `/prep`, the rules are embedded directly in the spec.

2. **Documentation gets stale** - The continuation file drifts from reality.
   Step 0 forces verification BEFORE planning, catching drift immediately.

3. **Stale info propagates** - Without verification, incorrect test status gets
   copied session after session. Step 0 breaks this cycle.

**The spec contains VERIFIED current state and embedded rules, not stale references.**
