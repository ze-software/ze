# Planning Requirements

**BLOCKING:** Before implementing ANY non-trivial feature, complete this planning process.

## Meta-Rule: Re-read Before Start and End

**CRITICAL:** Always re-read this file (planning.md):
1. **Before starting work** - Ensure you follow current rules
2. **Before asking to commit** - Verify rules weren't updated; if they were, ensure work follows updated rules

This prevents drift between rules and practice.

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
        a. Check if `docs/rfc/rfcNNNN.md` exists
        b. If missing summary: run agent with `/rfc-summarisation rfcNNNN`
        c. If missing RFC: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
      → Read ALL relevant RFC summaries

[ ] 7. RFC Implementation Guide (for protocol work)
      → Read `docs/contributing/rfc-implementation-guide.md`
      → Identify which phases apply (capability, attribute, NLRI, FSM, etc.)
      → Use phase checklists to ensure completeness

[ ] 8. Read source code for affected area

[ ] 9. TDD Planning - identify tests BEFORE implementation
      → Unit tests needed (write BEFORE implementation - strict TDD)
      → Boundary tests for all numeric inputs (see tdd.md for 3-point rule)
      → Functional tests needed (write AFTER feature works - end of plan)
      → Test file locations

[ ] 10. Present implementation plan to user
       → WAIT for approval before continuing

[ ] 11. Write spec to `docs/plan/spec-<task>.md`
       → FIRST complete "Pre-Spec Verification" checklist below
       → Match template format EXACTLY (not approximately)

[ ] 12. Track spec with git
       → `git add docs/plan/spec-<task>.md`
       → Ensures spec is not lost if session ends

[ ] 13. Begin TDD cycle (test fails → implement → test passes)

[ ] 14. Post-implementation completion (see "Completion Checklist" below)
```

## Keyword → Documentation Mapping

**ALWAYS START WITH:** `docs/architecture/core-design.md` (canonical architecture reference)

| Keywords in task | Required docs | RFC summaries |
|------------------|---------------|---------------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md` | `rfc4271.md` |
| UPDATE, message, build, route, announce | `core-design.md`, `update-building.md`, `encoding-context.md` | `rfc4271.md`, `rfc4760.md` |
| attribute, AS_PATH, NEXT_HOP, MED, LOCAL_PREF | `core-design.md`, `wire/attributes.md`, `update-building.md` | `rfc4271.md`, `rfc6793.md` |
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
| test, functional, .ci, zebgp-peer, VFS | `functional-tests.md`, `testing/ci-format.md` | |

All architecture docs are in `docs/architecture/` unless otherwise specified.
All RFC summaries are in `docs/rfc/`.

**RFC Summary Existence:** Before starting work, verify summaries exist for each RFC listed. If missing, run `/rfc-summarisation rfcNNNN` to create it.

For complete RFC keyword mapping, see `.claude/INDEX.md` → "RFC Summaries" section.

## Implementation Plan Format

Present to user BEFORE writing code:

```
## 📋 Implementation Plan for <task>

### Docs Read
- `docs/architecture/<doc>.md` - [key insight]

### RFC Summaries (MUST for protocol work)
- `docs/rfc/rfcNNNN.md` - [key insight]

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

### Design Decisions
- [decision]: [rationale from docs]

### RFC References (if BGP protocol code)
- RFC NNNN Section X.Y - [what it covers]

❓ [Any clarifying questions]
```

**WAIT FOR USER APPROVAL** before proceeding.

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
```

**Common mistakes:**
- `[x]` for read docs → use `[ ]` per template
- Missing `🧪` in TDD Test Plan header
- Skipping keyword→doc mapping table
- Prose instead of table for Functional Tests
- "- [description]" instead of "- [why relevant]" in Required Reading
- Missing RFC summaries for protocol work (MUST exist before implementation)

## Spec File Template

Write to `docs/plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Task
<description>

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]

### RFC Summaries (MUST for protocol work)
- [ ] `docs/rfc/rfcNNNN.md` - [why relevant]

**Key insights:**
- [insight from docs]

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
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `test-xxx` | `test/data/.../*.ci` | [description] | |

### Future (if deferring any tests)
- [Tests to add later and why deferred]

## Files to Modify
- `internal/...` - [changes]

## Files to Create
- `internal/...` - [new file purpose]
- `test/data/...` - [test files]

## Implementation Steps
1. **Write unit tests** - Create unit tests BEFORE implementation (strict TDD)
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Minimal code to pass
4. **Run tests** - Verify PASS (paste output)
5. **RFC refs** - Add RFC reference comments
6. **RFC constraints** - Add constraint comments with quoted requirements (see RFC Documentation)
7. **Functional tests** - Create functional tests AFTER feature works
8. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

### Reference Comments
- Add `// RFC NNNN Section X.Y` comments for protocol code
- If RFC missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

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

### Deviations from Plan
- [Any differences from original plan and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)

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

[ ] 3. Update spec to reflect reality
      → Mark all checklist items with actual status
      → Add "Implementation Summary" section if missing
      → Document any bugs found/fixed
      → Document any deviations from original plan

[ ] 4. Move spec to done folder
      → Use the "Moving Completed Specs" script below
      → Spec number determined at move time

[ ] 5. Verify all changes
      → `git status` to see all modified files
      → `git diff` to review changes
      → Ensure no unintended modifications

[ ] 6. Commit (when user approves)
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
- `tdd.md` - TDD cycle enforcement
- `rfc-compliance.md` - RFC reading and comments
- `go-standards.md` - Code quality
- `git-safety.md` - Safe commits
- `docs/contributing/rfc-implementation-guide.md` - Component checklists for RFC work

## Design-First Principle

**SEARCH before implementing:**
1. Search codebase for similar patterns
2. Extend existing code, don't duplicate
3. Think deeply about implications
4. Consider zero-copy/pool architecture
5. Check ExaBGP compatibility requirements
