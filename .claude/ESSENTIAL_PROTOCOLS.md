# Essential Protocols (READ EVERY SESSION)

**Purpose:** Core rules that apply to ALL interactions
**Size:** ~5 KB
**Read time:** <1 minute

---

## CRITICAL RULES

### 0. Work Preservation (NEVER LOSE CODE)

**Core principle:** NEVER discard uncommitted work. ALWAYS ask first.

## FORBIDDEN without EXPLICIT user permission:

- `git reset` (any form)
- `git revert`
- `git checkout -- <file>`
- `git restore` (to discard changes)
- `git stash drop`
- `rm` / deleting tracked files

**NO EXCEPTIONS. ALWAYS ASK FIRST.**

## MANDATORY WORKFLOW when you want to revert/change approach:

**STEP 1: ALWAYS save first**
```bash
git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch
```

**STEP 2: ALWAYS ask the user**
- "Tests are failing. Should I: (a) keep debugging, (b) save and try different approach, (c) revert?"
- WAIT for user response before ANY destructive action

**Backup location:** `.claude/backups/`

---

### 1. Verification Before Claiming

**Core principle:** Never claim success without proof

**Forbidden without verification:**
- "Fixed" / "Complete" / "Working" / "All tests pass"
- Checkmarks without command output
- Explanations instead of proof

**Required:**
1. Run the actual command/test
2. Paste exact output
3. Let output prove success/failure

**Example:**
```bash
make test
# [paste full output]
# PASS
```

---

### 2. Communication Style

**Core principle:** Terse, direct, emoji-prefixed status lines

**Do:**
- Start status lines with emoji
- One-sentence responses for simple actions
- Direct statements, no hedging

**Don't:**
- Politeness, reassurance, explanations
- "I'll help you..." / "Great news!" / "Unfortunately..."
- Multi-paragraph responses for simple tasks

**Examples:**
- "Tests pass (go test: 42 passed)"
- "Build failed: missing import in fsm.go:45"

**See:** `output-styles/zebgp.md` for full guidelines

---

### 3. Test-Driven Development (TDD) - BLOCKING RULE

```
┌─────────────────────────────────────────────────────────────────┐
│  TESTS MUST EXIST AND FAIL BEFORE IMPLEMENTATION CODE EXISTS   │
│                                                                 │
│  Writing implementation without failing tests = VIOLATION      │
└─────────────────────────────────────────────────────────────────┘
```

**See:** `TDD_ENFORCEMENT.md` for complete workflow.

## TDD Cycle (MANDATORY - NO EXCEPTIONS)

```
1. WRITE TEST (with documentation)
   ↓
2. RUN TEST → MUST FAIL (paste failure output)
   ↓
3. WRITE IMPLEMENTATION (minimum to pass)
   ↓
4. RUN TEST → MUST PASS (paste pass output)
   ↓
5. REFACTOR (keep tests green)
```

## Test Documentation Required

Every test MUST document:
```go
// TestFeatureName verifies [behavior].
//
// VALIDATES: [what correct behavior looks like]
//
// PREVENTS: [what bug/failure this catches]
func TestFeatureName(t *testing.T) { ... }
```

## Forbidden Actions

| Action | Violation | Correction |
|--------|-----------|------------|
| Write impl before test | Protocol violation | Delete impl, write test |
| Test passes immediately | Invalid test | Add failing assertion |
| Skip failure verification | No proof test works | Show failure first |

## During Development

```bash
# Run tests for current package (fast feedback)
go test -race ./internal/pool/... -v
```

## Before Claiming Done

```bash
make test  # Full suite - ALL must pass
```

---

### 4. Coding Standards

**Core principle:** Go 1.21+, idiomatic Go, strict linting

**Required:**
- Go 1.21+ (for slog, modern generics)
- `golangci-lint` must pass
- Error handling: NEVER ignore errors
- Context: Use `context.Context` for cancellation
- Concurrency: Prefer channels over mutexes where sensible

**Prohibited:**
- `panic()` for normal error handling
- Global mutable state
- `init()` functions (except for registry patterns)

**Linting:**
```bash
make lint  # golangci-lint run
```

**See:** CODING_STANDARDS.md for complete standards

---

### 5. Solution Quality (Right Solution, Not Easy Solution)

**Core principle:** Always implement the RIGHT solution, not the easiest.

**DO:**
- Fix the root cause, not just the symptom
- Refactor properly even if it touches multiple files
- Follow established patterns in the codebase

**DON'T:**
- Use type assertions without checks to silence errors
- Apply workarounds that leave technical debt
- Choose minimal changes over correct changes

---

### 6. Fix All Issues You Notice (No Broken Windows)

**Core principle:** If you see something wrong, fix it or document it for later.

**MANDATORY:**
- If you notice ANY issue (lint, bug, missing test), even if unrelated to current task:
  - **If quick to fix:** Fix it immediately with a test to prevent regression
  - **If not quick:** Add to `CLAUDE_CONTINUATION.md` or `plan/` for later

**Rationale:** Issues left unfixed accumulate. Tests prevent regressions.

**Examples:**
```
# You notice a lint warning while working on something else
→ Fix it now if <5 min, or add to plan/TODO.md

# You see code that could break under edge cases
→ Write a test for it, even if not your current task

# You find a pre-existing bug
→ Document in CLAUDE_CONTINUATION.md: "BUG: [description] in [file:line]"
```

**Never:**
- Ignore issues because "not my change"
- Leave lint warnings unfixed without documenting
- Skip writing tests for edge cases you discover

---

## Session Start Workflow

### 1. Read this file (you just did)

### 2. Check git state

```bash
git status && git diff && git diff --staged
```

If ANY modified/staged files: ASK user how to handle before starting work.

### 3. Check plan state

```bash
ls -la plan/ 2>/dev/null || echo "No plan directory"
```

### 4. Load contextual protocols based on task

| Activity | Load Protocol |
|----------|---------------|
| **ANY implementation** | **TDD_ENFORCEMENT.md** (ALWAYS) |
| Git operations | GIT_VERIFICATION_PROTOCOL.md |
| Writing Go code | CODING_STANDARDS.md |
| Writing protocol code | RFC_DOCUMENTATION_PROTOCOL.md |
| Refactoring | MANDATORY_REFACTORING_PROTOCOL.md |
| Test failures | TESTING_PROTOCOL.md, CI_TESTING.md |
| Error recovery | ERROR_RECOVERY_PROTOCOL.md |
| Creating docs | DOCUMENTATION_PLACEMENT_GUIDE.md |
| Session ending | SESSION_END_CHECKLIST.md |

---

## Codebase Architecture Quick Reference

**Directory structure:**
```
zebgp/
├── cmd/
│   ├── zebgp/           # Main daemon
│   ├── zebgp-cli/       # CLI client
│   └── zebgp-decode/    # Message decoder
├── pkg/
│   ├── bgp/             # BGP protocol
│   │   ├── message/     # Message types
│   │   ├── attribute/   # Path attributes
│   │   ├── nlri/        # NLRI types
│   │   ├── capability/  # Capabilities
│   │   └── fsm/         # State machine
│   ├── reactor/         # Event loop
│   ├── rib/             # RIB
│   ├── config/          # Configuration
│   └── api/             # External API
├── internal/
│   ├── store/           # Deduplication stores
│   └── pool/            # Buffer pools
└── testdata/            # Test fixtures
```

**Design patterns:**
- Registry/Factory for message types
- Interface-based polymorphism for NLRI/Attributes
- Goroutine per peer

---

## Git Workflow Essentials

**NEVER commit/push without explicit user request.**

**User must say:** "commit" / "make a commit" / "push"

**Before ANY git operation:**
```bash
git status && git log --oneline -5
```

**Workflow:**
1. Complete work
2. STOP and report what was done
3. WAIT for user instruction
4. Only commit/push if explicitly asked

---

## Quick Reference: Forbidden Phrases

**Without verification (command + output pasted):**
- "Fixed" / "Complete" / "Working" / "Ready"
- "All tests pass" / "Tests pass"
- Checkmarks without proof

**Without running `make test`:**
- "Done" / "Finished" / "Complete"
- Any claim code works

**Without explicit user request:**
- `git commit` / `git push`

**TDD Violations:**
- Writing ANY implementation before test exists
- Showing test pass without showing it failed first
- Tests without VALIDATES/PREVENTS documentation

**Auto-fix:** Stop. Run tests. Paste output. Then claim.

**TDD auto-fix:** Delete implementation. Write test. Show failure. Implement.

---

**Updated:** 2025-12-19
