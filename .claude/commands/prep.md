---
description: Create task specification with embedded protocol requirements (project)
argument-hint: <task description>
---

# /prep - Prepare Task Specification

Create a task specification with embedded protocol requirements.

## Instructions

When this skill is invoked, execute these steps IN ORDER:

### Step 1: Session Start Verification (MANDATORY)

**Before anything else, verify session state:**

1. **Check git status:**
   ```bash
   git status && git diff --stat
   ```
   - If modified files exist: **STOP and ASK user** how to handle them
   - Do NOT proceed until user responds

2. **Read ESSENTIAL_PROTOCOLS.md into context:**
   - Use the **Read tool** (not bash) to read `.claude/ESSENTIAL_PROTOCOLS.md`
   - Actually LOAD the file contents into context, don't just list it
   - This loads session rules, TDD requirements, verification rules
   - Summarize key rules after reading

### Step 2: Verify Current State (MANDATORY)

**WHY:** The continuation file gets out of sync. Trusting it without verification
causes incorrect information to propagate session after session.

1. **Run functional tests:**
   ```bash
   make functional 2>&1 | tail -40
   ```
   (Runs both encoding and API tests)

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

### Step 3: Load Protocol Files (MANDATORY)

Load ESSENTIAL_PROTOCOLS.md (done in Step 1) plus task-specific protocols
based on keywords (see table in Step 4).

### Step 4: Load Architecture Docs

**CRITICAL:** "Read" means USE THE READ TOOL to load file contents into context.
Do NOT just list files or acknowledge them. Actually read them so the content
is available for planning.

| Keywords in Task | Files to READ INTO CONTEXT |
|------------------|--------------------------|
| implement, add, create, feature, new | `.claude/TDD_ENFORCEMENT.md`, `.claude/CODING_STANDARDS.md` |
| test, fix test, failing, coverage | `.claude/TESTING_PROTOCOL.md`, `.claude/CI_TESTING.md` |
| RFC, protocol, compliance | `.claude/RFC_DOCUMENTATION_PROTOCOL.md`, read `rfc/rfcNNNN.txt` |
| ExaBGP, exabgp, compatibility | Check `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/` for reference implementation |
| API, endpoint, command, process | `.claude/zebgp/api/ARCHITECTURE.md`, `.claude/zebgp/api/COMMANDS.md`, `.claude/zebgp/api/PROCESS_PROTOCOL.md` |
| encoder, json, text, format, output | `.claude/zebgp/api/JSON_FORMAT.md`, `.claude/zebgp/api/ARCHITECTURE.md` |
| FSM, state, session, peer | `.claude/zebgp/behavior/FSM.md` |
| wire, message, parse, encode, decode | `.claude/zebgp/wire/MESSAGES.md`, `.claude/RFC_DOCUMENTATION_PROTOCOL.md` |
| attribute, path attribute | `.claude/zebgp/wire/ATTRIBUTES.md` |
| NLRI, prefix, route | `.claude/zebgp/wire/NLRI.md` |
| capability, open | `.claude/zebgp/wire/CAPABILITIES.md` |
| config, configuration | `.claude/zebgp/config/SYNTAX.md`, `.claude/zebgp/config/TOKENIZER.md` |
| context, encoding context | `.claude/zebgp/ENCODING_CONTEXT.md` |
| pool, buffer | `.claude/zebgp/POOL_ARCHITECTURE.md` |
| refactor, rename, move | (already in ESSENTIAL after consolidation) |
| git, commit, push | (already in ESSENTIAL after consolidation) |

**ENFORCEMENT:** After reading each file, briefly summarize what was loaded:
```
📖 Loaded: ARCHITECTURE.md - API package structure, route injection flow
📖 Loaded: JSON_FORMAT.md - RouteUpdate structure, encoder interface
```

### Step 5: Read Source Code (MANDATORY)

Before writing the spec, READ THE ACTUAL SOURCE FILES that will be modified:

1. **Identify files** from architecture docs that relate to the task
2. **Use Read tool** to load each file into context
3. **Understand current implementation** before proposing changes

**Example for API encoder task:**
```
📖 Read: pkg/api/server.go - OnUpdateReceived() at line 403
📖 Read: pkg/api/json.go - JSONEncoder, RouteUpdate struct
📖 Read: pkg/api/text.go - FormatReceivedUpdate(), ReceivedRoute struct
📖 Read: pkg/api/types.go - ProcessConfig.Encoder field
```

**WHY:** You cannot write a good implementation plan without understanding the
current code. Reading architecture docs is not enough - read the actual source.

### Step 6: Trace End-to-End User Flow (MANDATORY)

**CRITICAL:** Before writing the spec, trace the COMPLETE user flow:

1. **Configuration**: How does the user configure this feature?
   - What config syntax enables it?
   - Where is config parsed? Does it reach the right struct?
   - Are there defaults that might interfere?

2. **Execution path**: Follow data from entry to exit
   - What functions are involved?
   - Are there related handlers that need the same fix?
   - What events trigger this code?

3. **Related functionality**: What similar features exist?
   - If fixing X, does Y need the same fix?
   - Are there other event types that need updating?
   - Are there withdrawal/cleanup paths?

**Example for API encoder task:**
```
🔍 User flow analysis:
1. Config: "process foo { encoder json; receive update; }"
2. Parsing: pkg/config/bgp.go:543 sets Encoder, but line 548 only sets
   ReceiveUpdate=true for text! JSON processes never receive updates.
3. Execution: OnUpdateReceived() ignores cfg.Encoder (the stated bug)
4. Related: What about withdrawals? No OnWithdrawReceived exists.

⚠️ Found additional bugs:
- Config parsing bug: JSON encoder → ReceiveUpdate=false
- Missing feature: Withdrawals not forwarded to processes
```

**WHY:** Fixing only the stated issue often leaves related bugs unfixed.
Tracing end-to-end reveals the complete picture.

### Step 7: Verify Plan Achieves Goal (MANDATORY)

**BEFORE writing the spec, answer these questions:**

1. **User's actual goal**: What does the user want to achieve? (Not just the stated task)
   - Example: "User wants JSON-formatted updates in their script" (not just "fix encoder switching")

2. **Will the plan achieve it?** Trace through:
   - After implementation, can user configure the feature? → Config works?
   - After implementation, does the feature execute correctly? → Code works?
   - After implementation, does user see expected output? → End result correct?

3. **Blockers identified**: List anything that would prevent the goal:
   - Config parsing bugs that block feature enablement
   - Missing handlers that block feature execution
   - Missing output formatters that block correct output

4. **Plan covers all blockers?** For each blocker:
   - Is there an implementation step to fix it?
   - If not, add one or ask user about scope

**Example check for API encoder task:**
```
🎯 User goal: JSON-formatted route updates in process stdin

Will plan achieve it?
1. Config: ❌ JSON processes get ReceiveUpdate=false (blocker)
2. Execution: ❌ OnUpdateReceived always uses text (stated bug)
3. Output: ✅ JSONEncoder.RouteAnnounce exists

Blockers: [config parsing, encoder switching]
Plan covers: [config parsing ✅, encoder switching ✅]
→ Plan achieves goal: YES
```

**If plan does NOT achieve goal:** Expand scope or ask user.

### Step 8: Extract Key Rules (3-5 per protocol)

From each protocol file read, extract **only 3-5 key rules** that apply to this task.

**DO NOT paste entire protocol files.** Extract the most critical rules.

### Step 9: Generate Specification

Write the specification to `plan/spec-<task-name>.md` using this format:

```markdown
# Spec: <task-name>

## Task
$ARGUMENTS

## Current State (verified)
- `make test`: PASS/FAIL
- `make lint`: PASS/FAIL
- Functional tests: X passed, Y failed
- Failing: [list of failing test codes]
- Last commit: <hash>

## Context Loaded

### Architecture Docs
- `.claude/zebgp/api/ARCHITECTURE.md` - [brief summary of what's relevant]
- [other docs loaded]

### Source Files Read
- `pkg/api/server.go:403-424` - OnUpdateReceived() implementation
- [other source files read with specific line numbers]

## Problem Analysis
[Detailed analysis based on source code read - not guessing]

## End-to-End User Flow
[Trace the complete user flow from config to execution]

### Configuration Path
- Config syntax: `<how user configures this>`
- Parsing: `<where parsed, what structs populated>`
- Issues found: `<any config parsing bugs>`

### Execution Path
- Entry point: `<function that receives the event>`
- Processing: `<what happens to data>`
- Output: `<what user sees>`

### Related Handlers
- `<other functions that need same fix>`
- `<withdrawal/cleanup handlers>`
- `<other event types>`

## Goal Achievement Check

### User's Actual Goal
`<what user wants to achieve, not just the stated task>`

### Will Plan Achieve It?
| Step | Status | Notes |
|------|--------|-------|
| Config works? | ✅/❌ | `<can user enable feature?>` |
| Code works? | ✅/❌ | `<does feature execute correctly?>` |
| Output correct? | ✅/❌ | `<does user see expected result?>` |

### Blockers and Coverage
| Blocker | Plan Step | Covered? |
|---------|-----------|----------|
| `<blocker 1>` | Phase X Step Y | ✅/❌ |
| `<blocker 2>` | Phase X Step Y | ✅/❌ |

**Plan achieves goal:** YES/NO

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

### From TDD_ENFORCEMENT.md
- <3-5 key rules relevant to this task>

### From ESSENTIAL_PROTOCOLS.md
- <3-5 key rules relevant to this task>

### From <other-protocol>.md
- <3-5 key rules if additional protocols were read>

## Design Decision
[If multiple approaches exist, document options and chosen approach with rationale]

## Implementation Steps
1. <specific step with clear deliverable>
2. <specific step with clear deliverable>
...

## Verification Checklist
- [ ] Tests written and shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] **Goal achievement verified** (user can configure, execute, see correct output)
- [ ] <task-specific verification>

## Test Specification
[Detailed test cases with VALIDATES/PREVENTS documentation]
```

### Step 10: Confirm

After writing the spec, confirm:
```
✅ Spec written to plan/spec-<task-name>.md

📖 Context loaded:
  - ESSENTIAL_PROTOCOLS.md (session rules)
  - <architecture docs loaded>
  - <source files read with line numbers>

📋 Protocols embedded:
  - TDD_ENFORCEMENT.md (test-first rules)
  - <other protocols>

🔍 State verified:
  - make test: PASS/FAIL
  - make lint: PASS/FAIL
  - functional: X passed, Y failed

🎯 Goal achievement:
  - User goal: <actual goal>
  - Blockers: <N identified, N covered by plan>
  - Plan achieves goal: YES/NO

🎯 Ready to implement (only if plan achieves goal)
```

**IMPORTANT:** If "Plan achieves goal" is NO, do NOT proceed. Either:
1. Expand the plan to cover missing blockers
2. Ask user if reduced scope is acceptable

---

## Examples

### Example 1: `/prep fix API encoder switching`

**Step 1 - Session verification:**
```
git status → 1 modified file
⚠️ STOP: Ask user how to handle modified files
```

**Step 2 - State verification:**
```
🔍 functional tests: 24 passed, 13 failed
📋 Continuation file: up-to-date
```

**Step 3 - Read ESSENTIAL_PROTOCOLS.md:**
```
📖 Loaded: ESSENTIAL_PROTOCOLS.md
  - TDD: test must fail before implementation
  - Verification: paste command output as proof
  - Self-review: mandatory after completion
```

**Step 4 - Detect keywords and load docs:**
Keywords found: `API`, `encoder`
```
📖 Loaded: .claude/zebgp/api/ARCHITECTURE.md
  - Route injection flow, ProcessConfig structure
📖 Loaded: .claude/zebgp/api/JSON_FORMAT.md
  - RouteUpdate struct, JSONEncoder interface
```

**Step 5 - Read source files:**
```
📖 Read: pkg/api/server.go:403-424 - OnUpdateReceived() uses FormatReceivedUpdate always
📖 Read: pkg/api/json.go - RouteUpdate struct, JSONEncoder.RouteAnnounce()
📖 Read: pkg/api/text.go - ReceivedRoute struct, FormatReceivedUpdate()
📖 Read: pkg/api/types.go - ProcessConfig.Encoder field
📖 Read: pkg/config/bgp.go:543-550 - Config parsing logic
```

**Step 6 - End-to-end analysis:**
```
🔍 Configuration path:
  - User configures: process foo { encoder json; }
  - Parsing at bgp.go:548: if pc.Encoder == "text" { pc.ReceiveUpdate = true }
  ⚠️ BUG: JSON processes get ReceiveUpdate=false!

🔍 Execution path:
  - OnUpdateReceived always calls FormatReceivedUpdate (text)
  ⚠️ BUG: cfg.Encoder never checked

🔍 Related handlers:
  - FormatReceivedWithdraw exists but never called
  - JSONEncoder.RouteWithdraw exists but never called
  ⚠️ MISSING: No withdrawal forwarding to processes
```

**Step 7 - Goal achievement check:**
```
🎯 User goal: JSON-formatted route updates in process stdin

Blockers: [config parsing, encoder switching]
Plan covers: [config parsing ✅, encoder switching ✅]
→ Plan achieves goal: YES
```

**Output spec includes:**
- Verified test state
- All docs loaded with summaries
- All source files read with line numbers
- **End-to-end user flow analysis**
- **Goal achievement check with blockers**
- **Additional bugs discovered** (config parsing, missing withdrawals)
- Implementation steps in phases
- Questions for user about scope

### Example 2: `/prep implement AS path validation`

**Steps 1-2:** Verify git status, test status
**Step 3:** Load ESSENTIAL_PROTOCOLS.md
**Step 4:** Keywords `implement`, `path` → Load TDD_ENFORCEMENT.md, ATTRIBUTES.md
**Step 5:** Read source files:
```
📖 Read: pkg/bgp/attribute/aspath.go - current validation logic
📖 Read: pkg/bgp/attribute/aspath_test.go - existing tests
```
**Steps 6-7:** Trace user flow, verify goal achievement

---

## Why This Matters

This skill exists because:

1. **Claude skips reading protocol files "on demand"** - By forcing protocol reading
   as part of `/prep`, the rules are embedded directly in the spec.

2. **Claude doesn't read source code before planning** - Without Step 5, specs
   are based on assumptions, not actual code. This causes wrong implementations.

3. **Documentation gets stale** - The continuation file drifts from reality.
   Step 2 forces verification BEFORE planning, catching drift immediately.

4. **Architecture docs aren't enough** - Reading ARCHITECTURE.md tells you the
   structure, but reading the actual source at specific line numbers tells you
   what the code actually does.

5. **Context must be loaded, not referenced** - Saying "read file X" is not the
   same as actually reading it. The file contents must be in context.

6. **Plans often miss the actual goal** - Fixing the stated issue isn't enough
   if blockers prevent the user from achieving their goal. Step 7 forces
   verification that the plan will actually work end-to-end.

**The spec contains VERIFIED state, LOADED context, embedded rules, and GOAL ACHIEVEMENT CHECK.**
