# Planning Requirements

**BLOCKING:** Before implementing ANY non-trivial feature, complete this planning process.

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

[ ] 7. Read source code for affected area

[ ] 8. TDD Planning - identify tests BEFORE implementation
      → Unit tests needed
      → Functional tests needed
      → Test file locations

[ ] 9. Present implementation plan to user
      → WAIT for approval before continuing

[ ] 10. Write spec to `docs/plan/spec-<task>.md`
       → FIRST complete "Pre-Spec Verification" checklist below
       → Match template format EXACTLY (not approximately)

[ ] 11. Begin TDD cycle (test fails → implement → test passes)
```

## Keyword → Documentation Mapping

| Keywords in task | Required docs | RFC summaries |
|------------------|---------------|---------------|
| buffer, iterator, parse, wire, zero-copy | `docs/architecture/buffer-architecture.md` **(TARGET)** | |
| UPDATE, message, build, route, announce | `docs/architecture/update-building.md`, `encoding-context.md`, `buffer-architecture.md` | `rfc4271.md`, `rfc4760.md` |
| attribute, community, AS_PATH, NEXT_HOP | `docs/architecture/wire/attributes.md`, `update-building.md` | `rfc4271.md`, `rfc1997.md`, `rfc4360.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `docs/architecture/wire/nlri.md` | `rfc4760.md` |
| capability, OPEN, negotiate | `docs/architecture/wire/capabilities.md` | `rfc5492.md`, `rfc9072.md` |
| pool, memory, dedup, zero-copy | `docs/architecture/pool-architecture.md`, `encoding-context.md`, `buffer-architecture.md` | |
| forward, reflect, wire cache | `docs/architecture/encoding-context.md`, `update-building.md`, `buffer-architecture.md` | |
| route, rib, storage, duplication | `docs/architecture/route-types.md`, `rib-transition.md`, `buffer-architecture.md` | |
| FSM, state, session, peer | `docs/architecture/behavior/fsm.md` | `rfc4271.md`, `rfc4724.md` |
| API, command, announce, withdraw | `docs/architecture/api/architecture.md`, `api/capability-contract.md` | |
| config, YAML, load | `docs/architecture/config/syntax.md` | |
| FlowSpec | `docs/architecture/wire/nlri.md`, `wire/nlri-flowspec.md` | `rfc8955.md`, `rfc8956.md` |
| VPN, L3VPN, MPLS-VPN | `docs/architecture/wire/nlri.md` | `rfc4364.md`, `rfc4659.md`, `rfc8277.md` |
| EVPN | `docs/architecture/wire/nlri.md`, `wire/nlri-evpn.md` | `rfc7432.md`, `rfc9136.md` |
| BGP-LS, link-state | `docs/architecture/wire/nlri-bgpls.md` | `rfc7752.md`, `rfc9085.md`, `rfc9514.md` |
| ExaBGP, compatibility | `docs/exabgp/exabgp-code-map.md`, `exabgp-compatibility.md` | |
| design, transition, architecture | `docs/architecture/rib-transition.md` | |
| ASN4, AS4, 4-byte AS | `docs/architecture/edge-cases/as4.md` | `rfc6793.md` |
| ADD-PATH, path-id | `docs/architecture/edge-cases/addpath.md` | `rfc7911.md` |
| extended message | `docs/architecture/edge-cases/extended-message.md` | `rfc8654.md` |
| graceful restart, GR | `docs/architecture/behavior/fsm.md` | `rfc4724.md` |
| route-refresh | | `rfc2918.md`, `rfc7313.md` |
| error handling, notification | | `rfc7606.md`, `rfc9003.md` |
| large community | | `rfc8092.md` |
| extended community, RT | | `rfc4360.md`, `rfc5701.md` |
| role, OTC, route leak | | `rfc9234.md` |
| IPv6 next hop | | `rfc8950.md` |
| labeled unicast, label | | `rfc8277.md`, `rfc3032.md` |

All architecture docs are in `docs/architecture/` unless otherwise specified.
All RFC summaries are in `docs/rfc/`.
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
- `pkg/.../xxx_test.go` - TestXxx: [what it validates]

**Functional tests:** (if needed)
- `qa/tests/xxx/` - [scenario description]

### Implementation Phases
1. Write tests, verify they FAIL
2. Implement minimal code to pass tests
3. [additional phases...]

### Files Affected
- `pkg/...` - [what changes]

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
| Test | File | Validates |
|------|------|-----------|
| `TestXxx` | `pkg/.../xxx_test.go` | [description] |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `test-xxx` | `qa/tests/xxx/` | [description] |

## Files to Modify
- `pkg/...` - [changes]

## Implementation Steps
1. **Write tests** - Create tests
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Minimal code to pass
4. **Run tests** - Verify PASS (paste output)
5. **Verify all** - `make lint && make test && make functional` (lint includes govet + 25 other linters)
6. **RFC refs** - Add RFC reference comments
7. **RFC constraints** - Add constraint comments with quoted requirements (see RFC Documentation)

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

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added (quoted requirement + explanation)
- [ ] `docs/` updated if schema changed

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
```

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

## Design-First Principle

**SEARCH before implementing:**
1. Search codebase for similar patterns
2. Extend existing code, don't duplicate
3. Think deeply about implications
4. Consider zero-copy/pool architecture
5. Check ExaBGP compatibility requirements
