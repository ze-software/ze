# Planning Requirements

**BLOCKING:** Before implementing ANY non-trivial feature, complete this planning process.

## When Planning is Required

- New features or significant changes
- Any work touching BGP protocol code
- Changes affecting multiple files
- Unclear requirements or multiple approaches

## Pre-Implementation Checklist

Complete IN ORDER. Do not skip steps.

```
[ ] 1. Check existing spec: `plan/spec-<task>.md`
      → If exists: read it, resume from last progress
      → If not: continue

[ ] 2. Read `.claude/INDEX.md` for doc navigation

[ ] 3. Scan `plan/spec-*.md` for related specs

[ ] 4. Match task keywords to docs (see table below)

[ ] 5. Read ALL identified architecture docs

[ ] 6. Read source code for affected area

[ ] 7. TDD Planning - identify tests BEFORE implementation
      → Unit tests needed
      → Functional tests needed
      → Test file locations

[ ] 8. Present implementation plan to user
      → WAIT for approval before continuing

[ ] 9. Write spec to `plan/spec-<task>.md`

[ ] 10. Begin TDD cycle (test fails → implement → test passes)
```

## Keyword → Documentation Mapping

| Keywords in task | Required docs |
|------------------|---------------|
| UPDATE, message, build, route, announce | `UPDATE_BUILDING.md`, `ENCODING_CONTEXT.md` |
| attribute, community, AS_PATH, NEXT_HOP | `wire/ATTRIBUTES.md`, `UPDATE_BUILDING.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `wire/NLRI.md` |
| capability, OPEN, negotiate | `wire/CAPABILITIES.md` |
| pool, memory, dedup, zero-copy | `POOL_ARCHITECTURE.md`, `ENCODING_CONTEXT.md` |
| forward, reflect, wire cache | `ENCODING_CONTEXT.md`, `UPDATE_BUILDING.md` |
| FSM, state, session, peer | `behavior/FSM.md` |
| API, command, announce, withdraw | `api/ARCHITECTURE.md`, `api/CAPABILITY_CONTRACT.md` |
| config, YAML, load | `config/SYNTAX.md` |
| FlowSpec, VPN, EVPN, MPLS | `wire/NLRI.md`, `UPDATE_BUILDING.md` |
| ExaBGP, compatibility | `EXABGP_CODE_MAP.md`, `EXABGP_COMPATIBILITY.md` |
| design, transition, architecture | `plan/DESIGN_TRANSITION.md` |
| ASN4, AS4, 4-byte AS | `edge-cases/AS4.md` |
| ADD-PATH, path-id | `edge-cases/ADDPATH.md` |
| extended message | `edge-cases/EXTENDED_MESSAGE.md` |

All docs are in `.claude/zebgp/` unless otherwise specified.

## Implementation Plan Format

Present to user BEFORE writing code:

```
## 📋 Implementation Plan for <task>

### Docs Read
- `.claude/zebgp/<doc>.md` - [key insight]

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

## Spec File Template

Write to `plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Task
<description>

## Required Reading
- [ ] `.claude/zebgp/<doc>.md` - [why relevant]

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
5. **Verify all** - `make lint && make test && make functional`
6. **RFC refs** - Add RFC comments to protocol code

## RFC Documentation
- Add `// RFC NNNN Section X.Y` comments
- If RFC missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC references added
- [ ] `.claude/zebgp/` updated if schema changed

### Completion
- [ ] Spec moved to `plan/done/NNN-<name>.md`
```

## Post-Implementation Updates

If task changed any of these, update corresponding docs:

| Changed | Update |
|---------|--------|
| Config schema | `config/SYNTAX.md` |
| Wire format | `wire/MESSAGES.md`, `wire/ATTRIBUTES.md` |
| NLRI types | `wire/NLRI.md` |
| Capabilities | `wire/CAPABILITIES.md` |
| UPDATE building | `UPDATE_BUILDING.md` |
| Pool/memory | `POOL_ARCHITECTURE.md` |
| API commands | `api/ARCHITECTURE.md` |

## Moving Completed Specs

Determine number at move time, not during creation:

```bash
LAST=$(ls plan/done/ 2>/dev/null | grep -E "^[0-9]{3}-" | sort | tail -1 | cut -c1-3)
NEXT=$(printf "%03d" $((10#${LAST:-0} + 1)))
mv plan/spec-<name>.md plan/done/${NEXT}-<name>.md
```

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
