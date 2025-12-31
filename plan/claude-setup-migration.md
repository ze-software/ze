# Migration Plan: .claude Setup Alignment with Official Patterns

**Created:** 2025-12-31
**Goal:** Restructure `.claude/` to follow official Claude Code patterns

---

## Executive Summary

| Metric | Current | Target | Reduction |
|--------|---------|--------|-----------|
| Mandatory session context | ~1,600 lines | ~150 lines | 91% |
| Duplicate rule locations | 4-5 per rule | 1 each | 80% |
| Manual checklist steps | 11 | 0 (automated) | 100% |
| Total .md files | 48 | ~20 | 58% |

---

## Phase 1: Core Structure (HIGH PRIORITY)

### 1.1 Move CLAUDE.md to Project Root

**Current:** `.claude/CLAUDE.md` (non-standard location)
**Target:** `./CLAUDE.md` (official location)

```bash
# Action
git mv .claude/../CLAUDE.md ./CLAUDE.md 2>/dev/null || true
# Note: CLAUDE.md is already in root, but pointing to .claude/ internals
```

**New ./CLAUDE.md content (~100 lines):**

```markdown
# ZeBGP - Claude Instructions

## Commands
- `make test` - Run unit tests
- `make lint` - Run linter
- `make functional` - Run functional tests (37 tests)

## Workflow
1. Check git status (automated via hook)
2. For BGP code: read RFC from `rfc/` folder first
3. Write test, see it fail, implement, see it pass (TDD)
4. Run `make test && make lint` before claiming done
5. Only commit when explicitly requested

## Key Rules
- **TDD MANDATORY** - Test must exist and fail before implementation
- **RFC compliance** - BGP code must follow RFCs, add `// RFC NNNN` comments
- **Verify before claiming** - Paste command output as proof
- **Git safety** - Never commit/push without explicit request

## Reference Paths
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- RFCs: `rfc/` directory
- Current state: `plan/CLAUDE_CONTINUATION.md`

## Architecture Docs
When working on specific areas, read:
- Wire formats: `.claude/zebgp/wire/MESSAGES.md`
- NLRI types: `.claude/zebgp/wire/NLRI.md`
- Attributes: `.claude/zebgp/wire/ATTRIBUTES.md`
- Memory pools: `.claude/zebgp/POOL_ARCHITECTURE.md`
- Zero-copy: `.claude/zebgp/ENCODING_CONTEXT.md`
- ExaBGP compat: `.claude/zebgp/EXABGP_CODE_MAP.md`

## Style
- Terse, emoji-prefixed status lines
- No fluff, no reassurance
- Paste command output as proof
```

### 1.2 Create .claude/rules/ Directory

**Action:** Create path-based rule files

```bash
mkdir -p .claude/rules
```

**Files to create:**

| File | Paths Matcher | Content Source |
|------|---------------|----------------|
| `tdd.md` | `**/*.go` | TDD_ENFORCEMENT.md (condensed) |
| `go-standards.md` | `**/*.go` | CODING_STANDARDS.md (condensed) |
| `rfc-compliance.md` | `pkg/bgp/**/*.go` | ESSENTIAL_PROTOCOLS.md Section 0 |
| `testing.md` | `**/*_test.go` | TESTING_PROTOCOL.md (condensed) |
| `git-safety.md` | `*` | ESSENTIAL_PROTOCOLS.md git section |

---

## Phase 2: Rules Migration (HIGH PRIORITY)

### 2.1 Create .claude/rules/tdd.md

```yaml
---
paths: "**/*.go"
---

# Test-Driven Development

**BLOCKING:** Tests must exist and fail before implementation.

## Cycle
1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run test → MUST FAIL (paste output)
3. Write minimum implementation
4. Run test → MUST PASS (paste output)
5. Refactor while green

## Test Documentation Required
```go
// TestFeatureName verifies [behavior].
//
// VALIDATES: [what correct behavior looks like]
// PREVENTS: [what bug this catches]
func TestFeatureName(t *testing.T) { ... }
```

## Forbidden
- Implementation before test exists
- Test that passes immediately (invalid test)
- Claiming "done" without pasting test output
```

### 2.2 Create .claude/rules/go-standards.md

```yaml
---
paths: "**/*.go"
---

# Go Coding Standards

## Required
- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context for cancellation: `context.Context` first param

## Forbidden
- `panic()` for error handling
- `f, _ := func()` (ignoring errors)
- Global mutable state
- `init()` functions (except registry patterns)

## Fail-Early Rule
Configuration/parsing errors MUST propagate immediately.
Never silently ignore parse failures.
```

### 2.3 Create .claude/rules/rfc-compliance.md

```yaml
---
paths: "pkg/bgp/**/*.go"
---

# RFC Compliance

## Before Implementing BGP Features
1. Find RFC in `rfc/` folder (download if missing)
2. Read relevant sections
3. Note MUST/SHOULD/MAY requirements
4. Check ExaBGP reference implementation

## Code Requirements
```go
// parseOpenMessage parses a BGP OPEN message.
// RFC 4271 Section 4.2 - OPEN Message Format
func parseOpenMessage(data []byte) (*OpenMessage, error) {
```

## RFC MAY Clauses
When encountering MAY clauses, ASK user:
1. Implement this behavior?
2. Skip it?
3. Add configuration option?

## ExaBGP Reference
Check `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/`
- If ExaBGP matches RFC: follow ExaBGP approach
- If ExaBGP deviates: implement per RFC, document deviation
```

### 2.4 Create .claude/rules/git-safety.md

```yaml
---
paths: "*"
---

# Git Safety

## Commit Rules
- ONLY commit when user explicitly says "commit"
- Run `make test && make lint` before commit
- Update `plan/CLAUDE_CONTINUATION.md` after commit

## Forbidden Without Permission
- `git reset` (any form)
- `git revert`
- `git checkout -- <file>`
- `git push --force`

## Before Destructive Actions
Save first:
```bash
git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch
```
Then ASK user for permission.
```

---

## Phase 3: Hook Automation (MEDIUM PRIORITY)

### 3.1 Create SessionStart Hook

**File:** `.claude/hooks/session-start.sh`

```bash
#!/bin/bash
set -e

# Check git status
MODIFIED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
if [ "$MODIFIED" -gt 0 ]; then
    echo "modified_files: $MODIFIED"
    git status -s | head -10
    exit 2  # Ask user how to proceed
fi

# Report test status from continuation file
if [ -f "plan/CLAUDE_CONTINUATION.md" ]; then
    STATUS=$(grep -A3 "^## CURRENT STATUS" plan/CLAUDE_CONTINUATION.md 2>/dev/null | tail -3)
    echo "test_status:"
    echo "$STATUS"
fi

echo "session: ready"
```

### 3.2 Update settings.json

**File:** `.claude/settings.json` (add hooks section)

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/session-start.sh",
        "timeout": 5000
      }]
    }]
  }
}
```

### 3.3 Create PreToolUse Hook for Git Safety

**File:** `.claude/hooks/git-safety.sh`

```bash
#!/bin/bash
# Block dangerous git commands without explicit permission

TOOL_NAME="$1"
COMMAND="$2"

if [ "$TOOL_NAME" = "Bash" ]; then
    if echo "$COMMAND" | grep -qE "git (reset|revert|checkout.*--|push.*force)"; then
        echo "blocked: Destructive git command requires explicit user permission"
        exit 2
    fi
fi

exit 0
```

**Add to settings.json:**

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/git-safety.sh \"$TOOL_NAME\" \"$INPUT\""
      }]
    }]
  }
}
```

---

## Phase 4: Cleanup Redundancy (MEDIUM PRIORITY)

### 4.1 Files to DELETE (content moved to rules/)

| File | Reason |
|------|--------|
| `.claude/TDD_ENFORCEMENT.md` | → `.claude/rules/tdd.md` |
| `.claude/CODING_STANDARDS.md` | → `.claude/rules/go-standards.md` |
| `.claude/TESTING_PROTOCOL.md` | → `.claude/rules/testing.md` |
| `.claude/CI_TESTING.md` | Merge into rules/testing.md |

### 4.2 Files to CONSOLIDATE

| Current Files | Target |
|---------------|--------|
| `ESSENTIAL_PROTOCOLS.md` (1060 lines) | Split into rules/*.md + slim reference |
| `QUICK_REFERENCE.md` | Merge into ./CLAUDE.md |
| `CONTEXT_LOADING.md` | Delete (Claude auto-loads) |
| `INDEX.md` | Simplify to just architecture doc links |

### 4.3 Files to KEEP (architecture reference)

```
.claude/zebgp/           # Keep all - domain knowledge
.claude/output-styles/   # Keep - custom style
.claude/hooks/           # Keep - automation scripts
.claude/backups/         # Keep - work preservation
```

### 4.4 Simplified .claude/INDEX.md

```markdown
# ZeBGP Documentation

## Architecture Docs (read when working on specific areas)

| Area | Doc |
|------|-----|
| Wire formats | `zebgp/wire/MESSAGES.md` |
| NLRI types | `zebgp/wire/NLRI.md` |
| Attributes | `zebgp/wire/ATTRIBUTES.md` |
| Capabilities | `zebgp/wire/CAPABILITIES.md` |
| Memory pools | `zebgp/POOL_ARCHITECTURE.md` |
| Zero-copy | `zebgp/ENCODING_CONTEXT.md` |
| ExaBGP mapping | `zebgp/EXABGP_CODE_MAP.md` |
| FSM | `zebgp/behavior/FSM.md` |
| API | `zebgp/api/ARCHITECTURE.md` |
| Config syntax | `zebgp/config/SYNTAX.md` |
```

---

## Phase 5: /prep Skill Simplification (LOW PRIORITY)

### Current Problem
`/prep` is 276 lines that mostly:
1. Reminds to read protocols (now automated via hooks)
2. Creates spec files (useful, keep)
3. Enforces context loading (Claude does this automatically)

### Simplified /prep

```markdown
---
description: Create task specification
argument-hint: <task description>
---

# /prep - Create Task Spec

Write spec to `plan/spec-<task-name>.md`:

```markdown
# Spec: <task-name>

## Task
$ARGUMENTS

## Files to Read
- [relevant source files]
- [relevant .claude/zebgp/ docs]

## Implementation Steps
1. Write test (TDD)
2. Implement
3. Verify: make test && make lint

## Checklist
- [ ] Test fails first
- [ ] Test passes after impl
- [ ] make test passes
- [ ] make lint passes
```

Report: "Spec written to plan/spec-<name>.md"
```

---

## Migration Order

### Week 1: Foundation
1. [ ] Create `.claude/rules/` directory
2. [ ] Create `rules/tdd.md`
3. [ ] Create `rules/go-standards.md`
4. [ ] Create `rules/rfc-compliance.md`
5. [ ] Create `rules/git-safety.md`
6. [ ] Test that rules are loaded correctly

### Week 2: Automation
1. [ ] Create `hooks/session-start.sh`
2. [ ] Add SessionStart hook to settings.json
3. [ ] Create `hooks/git-safety.sh`
4. [ ] Add PreToolUse hook to settings.json
5. [ ] Test hooks work correctly

### Week 3: Cleanup
1. [ ] Update ./CLAUDE.md to slim version
2. [ ] Delete redundant files (TDD_ENFORCEMENT.md, etc.)
3. [ ] Simplify ESSENTIAL_PROTOCOLS.md (keep as reference only)
4. [ ] Simplify INDEX.md
5. [ ] Simplify /prep skill

### Week 4: Validation
1. [ ] Run full session with new setup
2. [ ] Verify context usage reduced
3. [ ] Verify rules apply correctly by path
4. [ ] Verify hooks fire correctly
5. [ ] Document any issues

---

## Rollback Plan

If migration causes issues:

```bash
# All changes are tracked in git
git checkout HEAD~N -- .claude/
git checkout HEAD~N -- CLAUDE.md
```

Keep `.claude/backups/` of original files before deletion.

---

## Success Metrics

| Metric | Before | After | How to Measure |
|--------|--------|-------|----------------|
| Session start context | ~1,600 lines | ~150 lines | Count lines loaded |
| Manual checklist items | 11 steps | 0 steps | SessionStart hook |
| Duplicate rules | 4-5x each | 1x each | Grep for duplicates |
| Time to first useful action | ~5 min | ~30 sec | Measure |

---

## Risks

| Risk | Mitigation |
|------|------------|
| Rules not loading by path | Test with `claude --debug` |
| Hooks failing silently | Add error logging to hooks |
| Lost domain knowledge | Keep zebgp/ docs unchanged |
| Team disruption | Migrate incrementally, document changes |

---

**Next Step:** Approve this plan, then start with Phase 1.1 (slim CLAUDE.md) + Phase 2.1 (rules/tdd.md)
