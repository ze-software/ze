# Planning Requirements

**BLOCKING:** Before implementing ANY non-trivial feature, complete this planning process.

## Spec Selection

**Only ONE spec is worked on at a time.** The selected spec is tracked in `.claude/selected-spec`.

### Starting Work on a Spec

```bash
# Select a spec (filename only, not path)
echo "spec-rfc9234-role.md" > .claude/selected-spec
```

### Completing Work on a Spec

After moving spec to `docs/plan/done/`:
```bash
# Clear selection
echo "" > .claude/selected-spec
```

## When Planning is Required

- New features or significant changes
- Any work touching BGP protocol code
- Changes affecting multiple files
- Unclear requirements or multiple approaches

**For RFC implementations:** Also consult `docs/contributing/rfc-implementation-guide.md` for component-specific checklists (capabilities, attributes, NLRI, FSM, etc.).

## Pre-Implementation Checklist

Complete IN ORDER. Do not skip steps. Steps are grouped into four phases.

```
── RESEARCH phase ──────────────────────────────────────────────
   Read, search, understand. No spec writing, no implementation.
   Gate: Can name 3 related files, describe current behavior,
         and have read architecture docs matched by keyword table.

[ ] 1. Check existing spec: `docs/plan/spec-<task>.md`
      → If exists: read it, resume from last progress
      → If not: continue

[ ] 2. Read `.claude/INDEX.md` for doc navigation

[ ] 3. Scan `docs/plan/spec-*.md` for related specs

[ ] 4. Match task keywords to docs (see `.claude/INDEX.md` keyword tables)

[ ] 5. Read identified architecture docs
      → Cannot proceed to DESIGN phase without reading these

[ ] 6. RFC Summary Check (for protocol work)
      → Identify ALL RFCs needed for implementation
      → For each RFC:
        a. Check if `rfc/short/rfcNNNN.md` exists
        b. If missing summary: run agent with `/rfc-summarisation rfcNNNN`
        c. If missing RFC: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
      → Read ALL relevant RFC summaries

[ ] 7. RFC Implementation Guide (for protocol work)
      → Read `docs/contributing/rfc-implementation-guide.md`
      → Identify which phases apply (capability, attribute, NLRI, FSM, etc.)
      → Use phase checklists to ensure completeness

[ ] 8. Read source code for affected area
      → READ the ACTUAL source files that will be modified (not just docs)
      → Document current behavior: What does this code output? What format?
      → **BLOCKING:** Cannot write spec until you can answer:
        "What does the existing code do? What must be preserved?"

[ ] 9. Trace data flow (see `rules/data-flow-tracing.md`)
      → Identify entry points (wire bytes, API, config, plugin)
      → Trace each transformation (parse → validate → store → process → encode)
      → Verify boundary crossings (Engine↔Plugin, FSM↔Reactor, Wire↔RIB)
      → Check for architectural violations (bypassed layers, coupling, duplication)
      → **BLOCKING:** Cannot write spec until data flow is understood

── DESIGN phase ────────────────────────────────────────────────
   Write spec, define acceptance criteria, get user approval.
   Gate: User approves spec.

[ ] 10. Document existing behavior in spec
      → Add "Current Behavior" section to spec
      → List exact output formats, function signatures, test expectations
      → These are preserved unless user explicitly says to change

[ ] 11. TDD Planning - identify tests BEFORE implementation
      → Unit tests needed (write BEFORE implementation - strict TDD)
      → Boundary tests for all numeric inputs (see tdd.md for 3-point rule)
      → Functional tests needed (write AFTER feature works - end of plan)
      → Test file locations

[ ] 12. Present implementation plan to user
      → WAIT for approval before continuing

[ ] 13. Write spec to `docs/plan/spec-<task>.md`
      → FIRST complete "Pre-Spec Verification" checklist below
      → Match template format EXACTLY (not approximately)
      → Include Acceptance Criteria (testable AC-N assertions)
      → Include context checkpoints (→ Decision: / → Constraint:) under Required Reading

[ ] 14. Track spec with git
      → `git add docs/plan/spec-<task>.md`
      → Ensures spec is not lost if session ends

── IMPLEMENT phase ─────────────────────────────────────────────
   TDD cycle. Log mistakes as they happen in Mistake Log.
   Gate: All tests pass.

[ ] 15. Begin TDD cycle (test fails → implement → test passes)
      → Log wrong assumptions and failed approaches in Mistake Log IMMEDIATELY

── VERIFY phase ────────────────────────────────────────────────
   Audit, docs, completion. No new code.
   Gate: Audit complete, `make verify` passes.

[ ] 16. Post-implementation completion (see "Completion Checklist" below)
      → Review Mistake Log escalation candidates
```

## Keyword → Documentation Mapping

See `.claude/INDEX.md` for the full keyword→doc and keyword→RFC tables. Consult during RESEARCH phase.

**RFC Summary Existence:** Before starting work, verify summaries exist for each RFC listed. If missing, run `/rfc-summarisation rfcNNNN` to create it.

## Implementation Plan Format

Present to user BEFORE writing code:

```
## 📋 Implementation Plan for <task>

### Docs Read
- `docs/architecture/<doc>.md` - [key insight]

### RFC Summaries (MUST for protocol work)
- `rfc/short/rfcNNNN.md` - [key insight]

### Current Behavior (MANDATORY - read source first)
**Source files read:**
- `path/to/file.go` - [what it does now]

**Behavior to preserve:**
- [output format, function signature, test expectations that MUST NOT change]
- [example: JSON output uses keys like "destination-ipv6", nested [[]] arrays]

**Behavior to change (only if user requested):**
- [explicit changes user asked for, or "None - preserve all"]

### 🧪 Tests First (TDD)
**Unit tests:**
- `internal/.../xxx_test.go` - TestXxx: [what it validates]

**Boundary tests:** (MANDATORY for numeric inputs)
| Field | Last Valid | Invalid Below | Invalid Above |
|-------|------------|---------------|---------------|
| [field] | [value] | [value/N/A] | [value/N/A] |

**Functional tests:** (if needed)
- `qa/tests/xxx/` - [scenario description]

### Implementation Phases
1. Write tests, verify they FAIL
2. Implement minimal code to pass tests
3. [additional phases...]

### Files Affected
- `internal/...` - [what changes]

### Data Flow (see `rules/data-flow-tracing.md`)
1. **Entry:** [where data enters - wire, API, config, plugin]
2. **Transformations:** [parse → validate → store → process → encode]
3. **Boundaries crossed:** [Engine↔Plugin, FSM↔Reactor, Wire↔RIB]
4. **Integration points:** [existing functions/types this connects to]

### Design Decisions
- [decision]: [rationale from docs]

### Design Principles Check (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ use cases exist?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling between components

### RFC References (if BGP protocol code)
- RFC NNNN Section X.Y - [what it covers]

❓ [Any clarifying questions]
```

**WAIT FOR USER APPROVAL** before proceeding.

## Spec Writing Style

See `rules/spec-no-code.md` — specs describe WHAT/WHY in tables and prose, never code.

## Spec Editing: Append-Only

**BLOCKING:** When editing specs, NEVER delete existing content.

| Action | Allowed |
|--------|---------|
| Add new sections | ✅ |
| Add clarifications | ✅ |
| Add status updates | ✅ |
| Mark superseded with ~~strikethrough~~ + reason | ✅ |
| Delete content | ❌ |
| Remove decisions | ❌ |
| Overwrite previous text | ❌ |

**Why:** Preserves decision history. After context compaction, deleted content is lost forever - you'll re-investigate solved problems and remake already-made decisions.

**When deletion IS allowed:**
- Moving spec to `docs/plan/done/` (completion)
- User explicitly requests deletion
- Fixing typos (not content changes)

## Pre-Spec Verification

**BLOCKING: Before writing any spec file, complete this checklist:**

```
[ ] 1. Keyword table checked (`.claude/INDEX.md`) — matching docs identified
[ ] 2. RFC summaries exist for all referenced RFCs (create if missing)
[ ] 3. Template format followed exactly (including 🧪 emoji, tables not prose)
[ ] 4. Checkboxes use [ ] not [x] — template shows unchecked
[ ] 5. No code snippets — use tables and prose (see `rules/spec-no-code.md`)
[ ] 6. Files to Modify includes feature code (internal/*, cmd/*), not only tests
[ ] 7. Current Behavior section completed (source files read, behavior documented)
[ ] 8. Data Flow section completed (see `rules/data-flow-tracing.md`)
[ ] 9. Acceptance Criteria have AC-N table rows with testable assertions
[ ] 10. Required Reading entries have → Decision: or → Constraint: checkpoint lines
```

## Spec File Template

Write to `docs/plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. [List key architecture docs from Required Reading]
4. [List key source files from Files to Modify]

## Task
<description>

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless user explicitly said to change)
- [output format, e.g., "JSON uses nested [[]] arrays for OR/AND grouping"]
- [function signatures that callers depend on]
- [test expectations from existing .ci files]

**Behavior to change:** (only if user explicitly requested)
- [list changes user asked for, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: e.g., "Wire parsing in internal/bgp/message/"]
2. [Stage 2: e.g., "Attribute extraction via iterators"]
3. [Stage 3: e.g., "Pool storage with dedup"]
4. [Stage N: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | [ ] |
| Wire ↔ Storage | [via iterators/pools] | [ ] |
| [other boundaries] | [mechanism] | [ ] |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |
| AC-2 | [what triggers the behavior] | [observable outcome] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what user expects to happen] | |

**RPC/API functional test rule:** Every new RPC or API endpoint MUST have a functional test that exercises it through the real transport (sockets, CLI, or plugin process). Unit tests with mock sockets test marshaling; functional tests prove the feature works end-to-end. If a functional test requires a test plugin, create one in `test/plugin/`.

### Future (if deferring any tests)
- [Tests to add later and why deferred — requires explicit user approval]

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- CHECK integration points: YANG schemas, CLI dispatch, editor, docs (see table below) -->
- `internal/...` - [feature changes]

### Integration Checklist
<!-- Answer each: does this task require updating these? If yes, add to Files to Modify/Create above. -->
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/yang/modules/*.yang` |
| RPC count in architecture docs | [ ] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [ ] | `cmd/ze/*/main.go` or subcommand files |
| CLI usage/help text | [ ] | Same as above |
| API commands doc | [ ] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | `.claude/rules/plugin-design.md` |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` or `test/decode/*.ci` (unit tests alone are NOT sufficient) |

## Files to Create
<!-- Feature code for codebase integration + functional tests for end-user verification -->
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** - Create unit tests BEFORE implementation (strict TDD)
   → **Review:** Are edge cases covered? Boundary tests for numeric inputs?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason? Not syntax errors?

3. **Implement** - Minimal code to pass
   → **Review:** Is this the simplest solution? Any code duplication? Does it follow existing patterns?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL tests pass? Any flaky behavior? Any warnings?

5. **RFC refs** - Add RFC reference comments
   → **Review:** Are all protocol decisions documented? Any MUST/MUST NOT missing?

6. **RFC constraints** - Add constraint comments with quoted requirements (see RFC Documentation)
   → **Review:** Would a future developer understand WHY each constraint exists?

7. **Functional tests** - Create functional tests AFTER feature works
   → **Review:** Do tests cover the user-visible behavior? Error cases included?

8. **Verify all** - `make lint && make unit-test && make functional-test` (paste output)
   → **Review:** Zero lint issues? All tests deterministic? No race conditions?

9. **Final self-review** - Before claiming done:
   - Re-read all code changes: any bugs, edge cases, or improvements?
   - Check for unused code, debug statements, TODOs
   - Verify error messages are clear and actionable
   - If issues found: FIX THEM before proceeding

### Failure Routing

When a step fails, use this table to determine where to route back:

| Failure | Symptom | Route To |
|---------|---------|----------|
| Compilation error | `go build` fails | Step 3 (Implement) — fix syntax or type errors |
| Test fails, wrong reason | Test errors on setup, not behavior | Step 1 (Write tests) — test itself is wrong |
| Test fails, behavior mismatch | Code does X, test expects Y | Re-read source files from Current Behavior. Was behavior misunderstood? If yes, back to RESEARCH phase |
| Lint failure | `make lint` reports issues | Fix inline. If architectural (e.g., import cycle), back to DESIGN phase |
| Functional test fails | `.ci` test expects wrong output | Check Acceptance Criteria. If AC wrong, update spec (DESIGN). If AC correct, fix implementation (IMPLEMENT) |
| Audit finds missing AC | Acceptance criterion not demonstrated | Back to IMPLEMENT for that specific criterion |

## Mistake Log

<!-- LIVE section — write to IMMEDIATELY when something goes wrong, not at the end. -->
<!-- This captures PROCESS mistakes. "Bugs Found/Fixed" in Implementation Summary captures CODE bugs. -->

### Wrong Assumptions
<!-- Log immediately when an assumption proves false (during any phase) -->
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
<!-- Log when an approach is tried and abandoned (during IMPLEMENT) -->
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
<!-- Fill at VERIFY phase. Check MEMORY.md for recurrence. Promote if seen before. -->
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

<!-- LIVE section — write IMMEDIATELY when you learn something, not at the end. -->
<!-- Only record insights that improve the project going forward. -->
<!-- Include enough context for the insight to be useful after compaction. -->
<!-- At completion, route each insight to its destination: -->
<!--   - Subsystem behavior → architecture doc (Post-Implementation Updates table) -->
<!--   - Process lesson → `.claude/rules/` relevant file -->
<!--   - Project knowledge → `.claude/rules/memory.md` -->

## RFC Documentation

### Reference Comments
- Add `// RFC NNNN Section X.Y` comments for protocol code
- If RFC missing: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

### Constraint Comments (CRITICAL)
When code enforces an RFC rule/constraint, document it ABOVE the code:

\`\`\`go
// RFC 4271 Section 6.3: "If the UPDATE message is received from an external peer"
// MUST check that AS_PATH first segment is neighbor's AS
if peer.IsExternal() && path.FirstAS() != peer.RemoteAS {
    return ErrInvalidASPath
}
\`\`\`

**Why:** Prevents accidental regression during refactoring. Future editors must understand WHY the constraint exists before modifying.

**Format:**
\`\`\`
// RFC NNNN Section X.Y: "<quoted requirement>"
// <brief explanation if not obvious>
<code that enforces it>
\`\`\`

**MUST document:**
- Validation rules (field ranges, required values)
- Error conditions and responses
- State machine transitions
- Timer constraints
- Message ordering requirements
- Any MUST/MUST NOT from RFC

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]
- **For each bug:** Add test that would have caught it (prevents re-investigation)

### Investigation → Test Rule
If you had to investigate/debug something, ask:
- Why wasn't this obvious from tests/docs?
- Add a test that makes the expected behavior explicit
- Future devs should never have to re-investigate the same issue

### Documentation Updates
<!-- Check the Post-Implementation Updates table below. Record ALL docs updated (architecture + rules + memory). -->
- [List docs updated, or "None — no changes"]

### Deviations from Plan
- [Any differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| [Feature 1 from Task section] | | | |
| [Feature 2 from Task section] | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | (test name or manual verification) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| [Test from Unit Tests table] | | | |
| [Test from Functional Tests table] | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| [File from Files to Modify] | | |
| [File from Files to Create] | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-N all demonstrated
- [ ] Tests pass (`make unit-test`)
- [ ] No regressions (`make functional-test`)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Integration completeness: every new feature proven to work from its intended usage point, not just in isolation (see `rules/integration-completeness.md`)
- [ ] Architecture docs updated with learnings and changes (see Post-Implementation Updates table)

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] RFC constraint comments added (quoted requirement + explanation)
- [ ] Implementation Audit fully completed (all items have status + location)
- [ ] Mistake Log escalation candidates reviewed

### 🏗️ Design (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)
- [ ] Functional tests verify end-to-end behavior (`.ci` files — MANDATORY for new RPCs/APIs, unit tests alone are insufficient)

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code

### Completion (after tests pass - see Completion Checklist)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
```

## Completion Checklist

**BLOCKING:** After implementation passes all tests, complete these steps IN ORDER:

```
[ ] 1. Review architecture docs and integration points
      → Did we learn something that improves the project going forward?
      → If yes: document with enough context to be useful standalone
        - Subsystem behavior → architecture doc (Post-Implementation Updates table)
        - Process lesson → `.claude/rules/` relevant file
        - Project knowledge → `.claude/rules/memory.md`
      → **YANG schemas:** If new RPCs were added, update the YANG module + RPC count in architecture.md
      → **CLI:** If new user-facing features, update cmd/ze/ dispatch + usage text + commands.md
      → **Editor:** YANG-driven autocomplete auto-updates, but verify if non-YANG completions needed
      → **Plugin SDK docs:** If new SDK methods, update plugin-design.md SDK tables

[ ] 2. Check for dead code and test coverage
      → Search for unused functions, types, or variables introduced
      → Check if refactoring left orphaned code
      → ASK user before removing: "Found unused X, remove it?"
      → Delete confirmed dead code (no backwards-compat shims needed)
      → Check if old tests overlap with new tests
      → Don't just delete redundant tests - integrate their coverage into new tests
      → Ensure edge cases from old tests are preserved in refactored tests

[ ] 3. Complete Implementation Audit (BLOCKING - see rules/implementation-audit.md)
      → Go through EVERY acceptance criterion (AC-1..AC-N)
      → Go through EVERY requirement in spec's Task section
      → Go through EVERY test in TDD Test Plan (unit + functional)
      → Go through EVERY file in Files to Modify/Create
      → Fill Implementation Audit table with status + location for each
      → Status must be: ✅ Done, ⚠️ Partial, ❌ Skipped, or 🔄 Changed
      → Any ⚠️ Partial or ❌ Skipped requires explicit user approval
      → Complete Audit Summary with accurate totals
      → CANNOT proceed until ALL items are accounted for

[ ] 4. Review Mistake Log escalation candidates
      → Check each entry against MEMORY.md — seen before?
      → Promote recurring mistakes to MEMORY.md or .claude/rules/
      → Mark Action column in Escalation Candidates table

[ ] 5. Update spec to reflect reality
      → Mark all checklist items with actual status
      → Add "Implementation Summary" section if missing
      → Fill "Documentation Updates" subsection (which docs were updated, or "None")
      → Document any bugs found/fixed
      → Document any deviations from original plan

[ ] 6. Move spec to done folder
      → Use the "Moving Completed Specs" script below
      → Spec number determined at move time

[ ] 7. Verify all changes
      → `git status` to see all modified files
      → `git diff` to review changes
      → Ensure no unintended modifications

[ ] 8. Commit (when user approves)
      → Include ALL modified files in ONE commit:
        - Code changes
        - Test files
        - Documentation updates
        - Moved spec file
      → Use descriptive commit message
```

**Why single commit:** All changes for a feature belong together. The spec documents what was done; it should be committed with the code it describes.

## Post-Implementation Updates

If task changed any of these, update corresponding docs:

| Changed | Update |
|---------|--------|
| Config schema | `docs/architecture/config/syntax.md` |
| Wire format | `docs/architecture/wire/messages.md`, `attributes.md` |
| NLRI types | `docs/architecture/wire/nlri.md` |
| Capabilities | `docs/architecture/wire/capabilities.md` |
| UPDATE building | `docs/architecture/update-building.md` |
| Pool/memory | `docs/architecture/pool-architecture.md` |
| API commands | `docs/architecture/api/architecture.md` |
| RPCs (plugin↔engine) | YANG schema (`internal/yang/modules/ze-plugin-engine.yang` or `ze-plugin-callback.yang`) + RPC count in `docs/architecture/api/architecture.md` |
| RPCs (user-facing) | YANG schema for domain (`ze-bgp-api.yang`, `ze-system-api.yang`, etc.) + handler registration in `command.go` |
| CLI commands/flags | `cmd/ze/` dispatch + usage text + `docs/architecture/api/commands.md` |
| Editor/completer | YANG-driven (auto-updates when YANG schema is updated) — verify via `internal/config/editor/` |
| Plugin SDK methods | `.claude/rules/plugin-design.md` SDK Engine Calls table |
| Test format (.ci) | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |

## Moving Completed Specs

Determine number at move time, not during creation:

```bash
# Find highest existing number (use 'command ls' to bypass aliases)
LAST=`command ls -1 docs/plan/done/ 2>/dev/null | sort -n | tail -1 | cut -c1-3`
test -z "$LAST" && LAST=0
NEXT=`printf "%03d" \`expr $LAST + 1\``
mv docs/plan/spec-<name>.md docs/plan/done/${NEXT}-<name>.md
```

**IMPORTANT:** Include the moved spec file in the same commit as the code changes. Do NOT commit the spec separately.

## Related Rules

- `implementation-audit.md` - Line-by-line verification that spec was fully implemented
- `integration-completeness.md` - Features must be proven integrated end-to-end
- `data-flow-tracing.md` - Verify changes fit architecture via data flow analysis
- `quality.md` - Self-critical review after each implementation step
