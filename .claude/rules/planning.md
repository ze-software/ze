# Planning Requirements

**BLOCKING:** Before implementing ANY non-trivial feature, complete this planning process.

## Meta-Rule: Re-read Before Start and End

**CRITICAL:** Always re-read this file (planning.md):
1. **Before starting work** - Ensure you follow current rules
2. **Before asking to commit** - Verify rules weren't updated; if they were, ensure work follows updated rules

This prevents drift between rules and practice.

## Spec Selection

**Only ONE spec is worked on at a time.** The selected spec is tracked in `.claude/selected-spec`.

### Starting Work on a Spec

```bash
# Select a spec (filename only, not path)
echo "spec-rfc9234-role.md" > .claude/selected-spec
```

### After Context Compaction or Session Restart

**CRITICAL:** After compaction or new session, you MUST:
1. Read the selected spec: `docs/plan/<selected-spec>`
2. **RE-READ ALL docs** listed in the spec's "Required Reading" section
3. **RE-READ ALL source files** listed in "Current Behavior" and "Files to Modify"
4. Re-read this file (planning.md)

**Checkboxes mean nothing after compaction.** A `[x]` next to a doc means you read it in a PREVIOUS session. You don't remember the content. RE-READ IT.

The session-start hook shows the selected spec prominently. If no spec is selected, it lists all active specs for you to choose from.

### Completing Work on a Spec

After moving spec to `docs/plan/done/`:
```bash
# Clear selection
echo "" > .claude/selected-spec
```

## No Code Without Understanding

**CRITICAL:** You are NOT ALLOWED to write any code until you:

1. **Search the codebase** - Find similar patterns, related code, existing solutions
2. **Read relevant files** - Understand current implementation and architecture
3. **Identify reuse opportunities** - Extend existing code, don't duplicate
4. **Understand data flow** - Know how data moves through the system
5. **Check architecture docs** - Read docs matching your task keywords (see table below)

**Why this matters:**
- Prevents duplicate code and conflicting patterns
- Avoids breaking existing functionality
- Ensures changes fit the architecture
- Saves time by reusing existing solutions

**Verification:** Before writing code, you should be able to explain:
- What existing code does this relate to?
- What patterns does the codebase use for this?
- How will your changes integrate with existing code?

## When Planning is Required

- New features or significant changes
- Any work touching BGP protocol code
- Changes affecting multiple files
- Unclear requirements or multiple approaches

**For RFC implementations:** Also consult `docs/contributing/rfc-implementation-guide.md` for component-specific checklists (capabilities, attributes, NLRI, FSM, etc.).

## Pre-Implementation Checklist

Complete IN ORDER. Do not skip steps.

```
[ ] 1. Check existing spec: `docs/plan/spec-<task>.md`
      → If exists: read it, resume from last progress
      → If not: continue

[ ] 2. Read `.claude/INDEX.md` for doc navigation

[ ] 3. Scan `docs/plan/spec-*.md` for related specs

[ ] 4. Match task keywords to docs (see table below)

[ ] 5. Read ALL identified architecture docs

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

[ ] 14. Track spec with git
      → `git add docs/plan/spec-<task>.md`
      → Ensures spec is not lost if session ends

[ ] 15. Begin TDD cycle (test fails → implement → test passes)

[ ] 16. Post-implementation completion (see "Completion Checklist" below)
```

## Keyword → Documentation Mapping

**ALWAYS START WITH:** `docs/architecture/core-design.md` (canonical architecture reference)

| Keywords in task | Required docs | RFC summaries |
|------------------|---------------|---------------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md`, `rules/buffer-first.md` | `rfc4271.md` |
| encode, Pack, WriteTo, make, alloc | `rules/buffer-first.md`, `buffer-architecture.md` | |
| UPDATE, message, build, route, announce | `core-design.md`, `update-building.md`, `encoding-context.md`, `rules/buffer-first.md` | `rfc4271.md`, `rfc4760.md` |
| attribute, AS_PATH, NEXT_HOP, MED, LOCAL_PREF | `core-design.md`, `wire/attributes.md`, `update-building.md`, `rules/buffer-first.md` | `rfc4271.md`, `rfc6793.md` |
| community | `wire/attributes.md` | `rfc1997.md` |
| extended community, RT, RD | `wire/attributes.md` | `rfc4360.md`, `rfc5701.md` |
| large community | `wire/attributes.md` | `rfc8092.md`, `rfc8195.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `core-design.md`, `wire/nlri.md` | `rfc4760.md` |
| multiprotocol, MP-BGP, AFI, SAFI | `wire/nlri.md`, `wire/capabilities.md` | `rfc4760.md` |
| capability, OPEN, negotiate | `wire/capabilities.md` | `rfc4271.md`, `rfc5492.md`, `rfc9072.md` |
| multiple labels capability | `wire/capabilities.md` | `rfc8277.md` |
| pool, memory, dedup, zero-copy | `core-design.md`, `pool-architecture.md`, `encoding-context.md` | |
| forward, reflect, wire cache | `core-design.md`, `encoding-context.md`, `update-building.md` | |
| route, rib, storage, duplication | `core-design.md`, `route-types.md`, `rib-transition.md` | |
| factory, family, builder | `core-design.md` | `rfc4760.md` |
| FSM, state, session, peer | `behavior/fsm.md` | `rfc4271.md`, `rfc4724.md` |
| keepalive, hold timer | `behavior/fsm.md` | `rfc4271.md` |
| notification, error, cease | `behavior/fsm.md` | `rfc4271.md`, `rfc7606.md`, `rfc9003.md` |
| shutdown, reset, admin | `behavior/fsm.md` | `rfc9003.md` |
| API, command, announce, withdraw | `api/architecture.md`, `api/capability-contract.md` | |
| config, YAML, load | `config/syntax.md` | |
| FlowSpec, traffic filter | `wire/nlri.md`, `wire/nlri-flowspec.md` | `rfc8955.md`, `rfc8956.md` |
| VPN, L3VPN, VPNv4, VPNv6, MPLS-VPN, 6PE, 6VPE | `wire/nlri.md` | `rfc4364.md`, `rfc4659.md`, `rfc4798.md` |
| labeled unicast, label, MPLS, label stack | `wire/nlri.md` | `rfc8277.md`, `rfc3032.md` |
| EVPN, MAC-IP, ethernet, VXLAN | `wire/nlri.md`, `wire/nlri-evpn.md` | `rfc7432.md`, `rfc9136.md` |
| VPLS, L2VPN, pseudowire, PW | `wire/nlri.md` | `rfc4761.md`, `rfc4762.md` |
| RT constraint | `wire/nlri.md` | `rfc4684.md` |
| BGP-LS, link-state | `wire/nlri-bgpls.md` | `rfc7752.md`, `rfc9085.md`, `rfc9514.md` |
| segment routing, SR, SID, prefix-SID | `wire/nlri-bgpls.md`, `wire/attributes.md` | `rfc9085.md`, `rfc8669.md` |
| SRv6 | `wire/nlri-bgpls.md` | `rfc9514.md` |
| ExaBGP, compatibility | `exabgp/exabgp-code-map.md`, `exabgp/exabgp-compatibility.md` | |
| design, transition, architecture | `rib-transition.md` | |
| ASN4, AS4, 4-byte AS | `edge-cases/as4.md` | `rfc6793.md`, `rfc4271.md` |
| ADD-PATH, path-id | `edge-cases/addpath.md` | `rfc7911.md` |
| extended message, >4096 | `edge-cases/extended-message.md` | `rfc8654.md` |
| graceful restart, GR | `behavior/fsm.md` | `rfc4724.md`, `rfc4271.md` |
| route-refresh, ORF | | `rfc2918.md`, `rfc7313.md` |
| role, OTC, route leak | | `rfc9234.md` |
| IPv6 next hop, extended NH | | `rfc8950.md`, `rfc4760.md` |
| treat-as-withdraw | | `rfc7606.md` |
| test, functional, .ci, ze-peer, VFS | `functional-tests.md`, `testing/ci-format.md` | |

All architecture docs are in `docs/architecture/` unless otherwise specified.
All RFC summaries are in `rfc/short/`.

**RFC Summary Existence:** Before starting work, verify summaries exist for each RFC listed. If missing, run `/rfc-summarisation rfcNNNN` to create it.

For complete RFC keyword mapping, see `.claude/INDEX.md` → "RFC Summaries" section.

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

**Specs are like RFCs - describe logic and behavior, not implementation.**

### No Code in Specs

Specs must NOT contain:
- Go code snippets
- Python code snippets
- Any programming language code

Specs SHOULD contain:
- Protocol message formats (text examples, not code)
- State descriptions (tables, not structs)
- Behavior descriptions (prose, not functions)
- Data structures (tables with field descriptions)
- Message flows (diagrams or step lists)

### Why No Code

- Code belongs in source files, not documentation
- Specs describe WHAT and WHY, code shows HOW
- Code in specs becomes stale and misleading
- Implementation details should emerge from TDD, not be prescribed

### Describing Data Structures

Instead of Go structs, use tables:

```markdown
| Field | Type | Description |
|-------|------|-------------|
| Module | string | YANG module name |
| Handlers | list | Handler paths this schema provides |
```

### Describing Behavior

Instead of function implementations, use prose or steps:

```markdown
**Verify routing:**
1. Find handler by longest prefix match
2. Route to plugin that registered this handler
3. Send command via pipe
4. Return response to caller
```

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
[ ] 1. Re-read this file (planning.md) - don't rely on memory
[ ] 2. Keyword table checked - ALL matching docs identified
[ ] 3. RFC summaries exist for all referenced RFCs (create if missing)
[ ] 4. Template visible - match format exactly, not approximately
[ ] 5. Checkboxes use [ ] not [x] - template shows unchecked
[ ] 6. Each doc has "- [why relevant]" after the path
[ ] 7. Section headers match template exactly (including 🧪 emoji)
[ ] 8. Tables used for Unit Tests and Functional Tests (not prose)
[ ] 9. Implementation steps include "(paste output)" where shown
[ ] 10. No code snippets - use tables and prose (see Spec Writing Style)
[ ] 11. Files to Modify includes feature code (internal/*, cmd/*), not only tests
[ ] 12. Functional Tests section includes .ci files for end-user verification
[ ] 13. Current Behavior section completed (source files read, behavior documented)
[ ] 14. "Behavior to change" is empty OR user explicitly requested the change
[ ] 15. Data Flow section completed (see `rules/data-flow-tracing.md`)
```

**Common mistakes:**
- `[x]` for read docs → use `[ ]` per template
- Missing `🧪` in TDD Test Plan header
- Skipping keyword→doc mapping table
- Prose instead of table for Functional Tests
- "- [description]" instead of "- [why relevant]" in Required Reading
- Missing RFC summaries for protocol work (MUST exist before implementation)
- Code snippets in spec → use tables and prose instead (see Spec Writing Style)
- Files to Modify contains only test files → feature code must be integrated (`internal/*`, `cmd/*`)
- Missing functional tests → every feature needs `.ci` tests for end-user verification
- Missing "Current Behavior" section → MUST document what existing code does BEFORE writing spec
- Inventing new formats → preserve existing behavior unless user explicitly asked to change it
- Missing "Data Flow" section → MUST trace data through system before implementation
- Skipping boundary verification → changes may violate architectural layers

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

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]

**Key insights:**
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
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what user expects to happen] | |

### Future (if deferring any tests)
- [Tests to add later and why deferred]

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/...` - [feature changes]

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

8. **Verify all** - `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests deterministic? No race conditions?

9. **Final self-review** - Before claiming done:
   - Re-read all code changes: any bugs, edge cases, or improvements?
   - Check for unused code, debug statements, TODOs
   - Verify error messages are clear and actionable
   - If issues found: FIX THEM before proceeding

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

### Design Insights
- [Key learnings that should be documented elsewhere]

### Documentation Updates
<!-- Check the Post-Implementation Updates table below. If your task changed any listed area, update the corresponding docs and record what you updated here. -->
- [List docs updated, or "None — no architectural changes"]

### Deviations from Plan
- [Any differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| [Feature 1 from Task section] | | | |
| [Feature 2 from Task section] | | | |

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
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass - see Completion Checklist)
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed (all items have status + location)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
```

## Completion Checklist

**BLOCKING:** After implementation passes all tests, complete these steps IN ORDER:

```
[ ] 1. Review architecture docs
      → Did we learn something not documented?
      → Add design insights, gotchas, or patterns discovered
      → Update docs in "Post-Implementation Updates" table below

[ ] 2. Check for dead code and test coverage
      → Search for unused functions, types, or variables introduced
      → Check if refactoring left orphaned code
      → ASK user before removing: "Found unused X, remove it?"
      → Delete confirmed dead code (no backwards-compat shims needed)
      → Check if old tests overlap with new tests
      → Don't just delete redundant tests - integrate their coverage into new tests
      → Ensure edge cases from old tests are preserved in refactored tests

[ ] 3. Complete Implementation Audit (BLOCKING - see rules/implementation-audit.md)
      → Go through EVERY requirement in spec's Task section
      → Go through EVERY test in TDD Test Plan (unit + functional)
      → Go through EVERY file in Files to Modify/Create
      → Fill Implementation Audit table with status + location for each
      → Status must be: ✅ Done, ⚠️ Partial, ❌ Skipped, or 🔄 Changed
      → Any ⚠️ Partial or ❌ Skipped requires explicit user approval
      → Complete Audit Summary with accurate totals
      → CANNOT proceed until ALL items are accounted for

[ ] 4. Update spec to reflect reality
      → Mark all checklist items with actual status
      → Add "Implementation Summary" section if missing
      → Fill "Documentation Updates" subsection (which docs were updated, or "None")
      → Document any bugs found/fixed
      → Document any deviations from original plan

[ ] 5. Move spec to done folder
      → Use the "Moving Completed Specs" script below
      → Spec number determined at move time

[ ] 6. Verify all changes
      → `git status` to see all modified files
      → `git diff` to review changes
      → Ensure no unintended modifications

[ ] 7. Commit (when user approves)
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

## Why This Matters

Reading architecture docs BEFORE implementation prevents:
- Duplicate work rediscovering existing patterns
- Wrong assumptions about intentional design decisions
- Changes that don't fit the architecture
- Missing zero-copy/pool considerations
- Breaking ExaBGP compatibility unknowingly

## Integration with Other Rules

This rule works with:
- `implementation-audit.md` - Line-by-line verification that spec was fully implemented
- `data-flow-tracing.md` - Verify changes fit architecture via data flow analysis
- `design-principles.md` - Scalability, maintainability, YAGNI
- `tdd.md` - TDD cycle enforcement
- `rfc-compliance.md` - RFC reading and comments
- `go-standards.md` - Code quality
- `git-safety.md` - Safe commits
- `docs/contributing/rfc-implementation-guide.md` - Component checklists for RFC work

## Self-Critical Review

**BLOCKING:** After completing each implementation step, perform a critical review.

### What to Check

| Category | Questions |
|----------|-----------|
| **Correctness** | Does it actually work? Edge cases handled? |
| **Simplicity** | Is this the simplest solution? Over-engineered? |
| **Consistency** | Does it follow existing patterns in the codebase? |
| **Completeness** | Any TODOs, FIXMEs, or unfinished work? |
| **Quality** | Debug statements removed? Error messages clear? |
| **Tests** | Tests cover the change? Any flaky tests introduced? |

### When Issues Are Found

1. **FIX immediately** - Don't defer to later
2. **Document** - Add to "Bugs Found/Fixed" in spec if significant
3. **Add test** - If the bug could have been caught by a test, add one

### Review Triggers

Perform critical review:
- After each implementation step (see Implementation Steps)
- Before claiming "done" or "complete"
- Before asking to commit
- When tests pass but behavior seems suspicious

### Common Issues to Catch

- Unused variables or imports
- Error paths that don't clean up resources
- Race conditions in concurrent code
- Hard-coded values that should be configurable
- Missing validation on user input
- Inconsistent naming with rest of codebase

## Design-First Principle

**SEARCH before implementing:**
1. Search codebase for similar patterns
2. Extend existing code, don't duplicate
3. Think deeply about implications
4. Consider zero-copy/pool architecture
5. Check ExaBGP compatibility requirements
