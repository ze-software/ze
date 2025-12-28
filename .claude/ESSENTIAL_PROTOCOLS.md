# Essential Protocols (READ EVERY SESSION)

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║   STOP. DO NOT PROCEED UNTIL YOU COMPLETE THE SESSION START CHECKLIST BELOW  ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║   QUALITY MANDATE: THE BEST OF THE BEST - NO EXCEPTIONS                       ║
║                                                                               ║
║   • Resources are NOT a constraint - spend as much as necessary               ║
║   • Always use "ultrathink" (extended thinking) when it would help            ║
║   • 100% correctness is REQUIRED - no approximations, no shortcuts            ║
║   • If unsure, investigate deeper - never guess                               ║
║   • Quality over speed - take the time to do it RIGHT                         ║
║                                                                               ║
║   This project demands excellence. Every implementation must be:              ║
║   • RFC-compliant without exception                                           ║
║   • Thoroughly tested                                                         ║
║   • Properly reviewed before claiming done                                    ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

## Table of Contents

1. [Session Start Checklist](#session-start-checklist-mandatory---do-this-first)
2. [File Locations](#file-locations-mandatory)
3. [Critical Rules](#critical-rules)
   - [RFC 4271 Compliance](#0-rfc-4271-compliance-non-negotiable)
   - [Context Management](#02-use-agents-for-multi-file-work-context-management)
   - [ExaBGP Reference](#04-exabgp-reference-implementation-must-check-before-coding)
   - [Work Preservation](#1-work-preservation-never-lose-code)
   - [Verification](#2-verification-before-claiming)
   - [Self-Review](#25-post-completion-self-review-mandatory)
   - [TDD](#5-test-driven-development-tdd---blocking-rule)
   - [Coding Standards](#6-coding-standards)
4. [Git Safety](#git-safety-protocol)
5. [Error Recovery](#error-recovery-protocol)
6. [Refactoring Protocol](#refactoring-protocol)
7. [Session End Checklist](#session-end-checklist)
8. [/prep Requirement](#prep-requirement-blocking)
9. [Quick Reference](#quick-reference-forbidden-phrases)

---

## SESSION START CHECKLIST (MANDATORY - DO THIS FIRST)

### Step 1: Check Git State IMMEDIATELY

```bash
git status && git diff --stat
```

**IF ANY MODIFIED/STAGED FILES EXIST:**
- STOP all other work
- ASK user: "There are N modified files. How should I handle them before proceeding?"
- WAIT for user response
- Do NOT proceed with any other task until resolved

### Step 2: Read plan/CLAUDE_CONTINUATION.md

```bash
cat plan/CLAUDE_CONTINUATION.md 2>/dev/null || echo "No continuation file"
```

Contains: Current state, priorities, what previous sessions accomplished.

### Step 3: Check Plan State

```bash
ls -la plan/ 2>/dev/null || echo "No plan directory"
```

### Step 4: Only THEN Proceed With User's Request

---

## FILE LOCATIONS (MANDATORY)

| File Type | Location | NOT Here |
|-----------|----------|----------|
| Session continuation | `plan/CLAUDE_CONTINUATION.md` | project root |
| Plans, TODOs, tasks | `plan/` | `.claude/` |
| Claude protocols | `.claude/` | `plan/` |
| Reference docs about codebase | `.claude/zebgp/` | `plan/` |
| Backups/patches | `.claude/backups/` | anywhere else |

**VIOLATION:** Putting plans in `.claude/` or protocols in `plan/`

---

## CRITICAL RULES

### 0. RFC 4271 Compliance (NON-NEGOTIABLE)

```
┌─────────────────────────────────────────────────────────────────┐
│  ZeBGP MUST be a fully RFC 4271 compliant BGP speaker.         │
│                                                                 │
│  ALL implementation decisions MUST follow RFC 4271 and its     │
│  updates. When in doubt, the RFC is authoritative.             │
│                                                                 │
│  Reference: https://datatracker.ietf.org/doc/html/rfc4271      │
└─────────────────────────────────────────────────────────────────┘
```

**Key RFC compliance requirements:**
- Message format validation per Section 4
- FSM behavior per Section 8
- UPDATE processing per Section 9
- Error handling per Section 6
- Path attribute handling per Appendix F

**RFC Reference Folder:** `rfc/`

All BGP RFCs that are fully or partially implemented MUST have their text
version in the `rfc/` folder. See `rfc/README.md` for the index.

**MANDATORY: Read RFC Before Implementation**

```
┌─────────────────────────────────────────────────────────────────┐
│  BEFORE implementing ANY RFC-related code, MUST read the        │
│  relevant sections from `rfc/rfcNNNN.txt` to ensure full        │
│  understanding of requirements, edge cases, and MUST/SHOULD.    │
│                                                                 │
│  Do NOT rely on memory or summaries. READ THE ACTUAL RFC TEXT.  │
└─────────────────────────────────────────────────────────────────┘
```

**MANDATORY: Download Missing RFCs**

If an RFC is needed but not present in `rfc/`:
1. Check if RFC exists: `ls rfc/rfcNNNN.txt`
2. If missing, download it: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
3. Update `rfc/README.md` with the new RFC entry
4. Then proceed with reading and implementation

**MANDATORY: Check RFC Supersession**

Some RFCs supersede or update others. ALWAYS check for the latest RFC:
1. Check `rfc/README.md` for "Obsoletes" and "Updates" information
2. When implementing, start from the LATEST RFC in a chain
3. Example: RFC 4271 is updated by RFC 6286, 6608, 6793, 7606, 7607, 8212, 8654, etc.
4. The `rfc/README.md` MUST track which RFCs update/obsolete which

**Workflow:**
1. Identify which RFC sections apply to the code being written
2. Check if RFC is superseded by a newer one - use the LATEST
3. Verify RFC exists in `rfc/` folder, download if missing
4. Read those sections from `rfc/rfcNNNN.txt` (use Read tool)
5. Note all MUST/SHOULD/MAY requirements
6. Implement according to RFC text
7. Add RFC reference comments in code

**MANDATORY: Handle RFC MAY Clauses**

When encountering RFC "MAY" clauses (optional behavior), ASK user:
1. Should we implement this behavior?
2. Should we skip it?
3. Should we add a configuration option?

Example: "RFC 4760 says speaker MAY treat non-negotiated AFI/SAFI as error.
Options: (1) Always error, (2) Always ignore, (3) Config option?"

**MANDATORY: Confirm Before Any RFC Deviation**

```
┌─────────────────────────────────────────────────────────────────┐
│  If the user requests something that DEVIATES from an RFC:      │
│                                                                 │
│  1. STOP and explain the RFC requirement                        │
│  2. ASK for explicit confirmation before proceeding             │
│  3. If confirmed, document the deviation in code comments       │
│                                                                 │
│  NEVER silently implement non-RFC-compliant behavior.           │
└─────────────────────────────────────────────────────────────────┘
```

When user requests something non-RFC-compliant, respond:
```
⚠️ RFC DEVIATION REQUEST

RFC NNNN Section X.Y requires: [quote requirement]
Your request: [what user asked]

This would make ZeBGP non-compliant with [RFC].
Confirm you want to proceed? (yes/no)
```

If confirmed, code MUST include deviation comment:
```go
// NOTE: RFC DEVIATION - User requested [behavior] on [date].
// RFC NNNN Section X.Y requires [X], but we do [Y] because:
// [User's reason or "explicit user request"]
func nonCompliantBehavior() { ... }
```

**MANDATORY: RFC References in Code**

Code implementing RFC functionality MUST include a comment referencing
the RFC number and section:

```go
// parseOpenMessage parses a BGP OPEN message.
// RFC 4271 Section 4.2 - OPEN Message Format
func parseOpenMessage(data []byte) (*OpenMessage, error) {
    // ...
}

// validateHoldTime checks hold time per RFC 4271.
// RFC 4271 Section 4.2: "Hold Time MUST be either zero or at least three seconds"
func validateHoldTime(holdTime uint16) error {
    // ...
}
```

**Why:** Direct RFC references enable verification of implementation correctness
and make it easy to cross-check behavior against specifications.

---

### 0.2 Use Agents for Multi-File Work (CONTEXT MANAGEMENT)

```
┌─────────────────────────────────────────────────────────────────┐
│  MUST use Task tool agents for multi-file operations to keep   │
│  main conversation context low.                                 │
│                                                                 │
│  Launch parallel agents when tasks are independent.            │
└─────────────────────────────────────────────────────────────────┘
```

**When to use agents:**
- Annotating multiple files with RFC references
- Searching across codebase for patterns
- Implementing features spanning multiple files
- Any task touching 3+ files

**Agent types:**
- `Explore` - codebase search and understanding
- `Plan` - implementation design
- `general-purpose` - complex multi-step tasks

---

### 0.3 Use Programs for Large Refactoring (EFFICIENCY)

```
┌─────────────────────────────────────────────────────────────────┐
│  MUST use sed, perl, or python for large search/replace ops.   │
│  Do NOT manually edit many files with repetitive changes.      │
└─────────────────────────────────────────────────────────────────┘
```

**Examples:**
```bash
# Rename function across codebase
sed -i '' 's/OldFunc/NewFunc/g' pkg/**/*.go

# Add import to multiple files
perl -i -pe 's/(package \w+)/\1\n\nimport "new\/pkg"/' file1.go file2.go

# Complex refactoring
python3 scripts/refactor.py --pattern 'old' --replace 'new'
```

**When to use:**
- Renaming symbols across 3+ files
- Adding/removing imports in bulk
- Consistent formatting changes
- Any repetitive edit pattern

---

### 0.4 ExaBGP Reference Implementation (MUST CHECK BEFORE CODING)

```
┌─────────────────────────────────────────────────────────────────┐
│  BEFORE implementing ANY BGP feature, ALWAYS check how         │
│  ExaBGP does it in:                                            │
│  /Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/
│                                                                 │
│  ZeBGP MUST match ExaBGP's API and behavior for compatibility. │
│                                                                 │
│  HOWEVER: If ExaBGP is NOT RFC-compliant, the RFC TAKES        │
│  PRECEDENCE. RFC compliance is NON-NEGOTIABLE.                  │
└─────────────────────────────────────────────────────────────────┘
```

**Priority Order (STRICT):**
1. **RFC compliance** - Always follow the RFC specification
2. **ExaBGP API compatibility** - Match ExaBGP's interface/behavior
3. **ExaBGP implementation** - Follow ExaBGP's approach when RFC-compliant

**MANDATORY before writing/fixing BGP code:**
1. Read the relevant RFC sections from `rfc/rfcNNNN.txt`
2. Find the equivalent code in `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/`
3. Check if ExaBGP's implementation matches the RFC
4. If ExaBGP matches RFC: follow ExaBGP's approach
5. If ExaBGP deviates from RFC: implement per RFC, document deviation

**When ExaBGP Differs from RFC:**
```go
// parseFeature implements RFC NNNN Section X.Y.
// NOTE: ExaBGP does [X] differently, but RFC requires [Y].
// We follow RFC here for compliance.
func parseFeature(...) { ... }
```

**Key ExaBGP directories:** (base: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`)
- `bgp/message/` - Message encoding/decoding
- `bgp/message/open/capability/` - Capability negotiation
- `bgp/message/update/attribute/` - Path attributes
- `bgp/message/update/nlri/` - NLRI types

**Why:** ExaBGP is a reference implementation for API compatibility, NOT
an authoritative specification. RFCs are the authoritative source.
If tests fail because ZeBGP follows RFC but ExaBGP doesn't, the tests
are wrong - fix the tests, not the RFC-compliant code.

---

### 1. Work Preservation (NEVER LOSE CODE)

**Core principle:** NEVER discard uncommitted work. ALWAYS ask first.

**FORBIDDEN without EXPLICIT user permission:**

- `git reset` (any form)
- `git revert`
- `git checkout -- <file>`
- `git restore` (to discard changes)
- `git stash drop`
- `rm` / deleting tracked files

**NO EXCEPTIONS. ALWAYS ASK FIRST.**

**MANDATORY WORKFLOW when you want to revert/change approach:**

**STEP 1: ALWAYS save first**
```bash
git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch
```

**STEP 2: ALWAYS ask the user**
- "Tests are failing. Should I: (a) keep debugging, (b) save and try different approach, (c) revert?"
- WAIT for user response before ANY destructive action

**Backup location:** `.claude/backups/`

---

### 2. Verification Before Claiming

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

### 2.5. Post-Completion Self-Review (MANDATORY)

```
┌─────────────────────────────────────────────────────────────────┐
│  AFTER COMPLETING ANY TASK, perform critical self-review.       │
│                                                                 │
│  This is NOT optional. Review catches issues before user does.  │
└─────────────────────────────────────────────────────────────────┘
```

**Trigger:** After any task completion (code written, bug fixed, feature added)

**Self-Review Process (ITERATE UNTIL CLEAN):**

1. **Perform Deep Critical Analysis (USE EXTENDED THINKING)**

   Begin review with: "ultrathink critically review the work performed"

   This FORCES extended thinking mode for thorough analysis.
   - Review ALL code changes made during this task
   - Check: Does it actually solve the problem correctly?
   - Check: Are there logic errors, edge cases missed, or subtle bugs?
   - Check: Does it follow existing patterns in the codebase?
   - Check: Are there RFC compliance issues?
   - Check: Are there security issues?

2. **Classify Issues Found:**
   | Severity | Action |
   |----------|--------|
   | 🔴 Critical | FIX IMMEDIATELY - blocks completion |
   | 🟡 Medium | FIX NOW - could cause problems |
   | 🟢 Minor | NOTE for user review |

3. **Fix Critical/Medium Issues:**
   - Fix each 🔴/🟡 issue found
   - Run tests after fixes
   - **LOOP:** Return to step 1 and re-review until NO critical/medium issues remain

4. **Report to User:**
   - Confirm task completion
   - List any 🟢 minor items for user review
   - Example: "✅ Done. Minor items for review: consider adding timeout config (line 45)"

**Example Review Output:**
```
🔍 Self-review:
  🔴 Missing nil check in parse() - FIXING
  🟡 Error message unclear - FIXING
  🟢 Could add metric counter (optional)

[After fixes]
🔍 Re-review: No critical/medium issues
✅ Done. Minor: could add metric counter at line 89
```

**Forbidden:**
- Claiming "done" without self-review
- Skipping re-review after fixes
- Ignoring critical/medium issues

---

### 3. Communication Style

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
- "✅ Tests pass (go test: 42 passed)"
- "❌ Build failed: missing import in fsm.go:45"

**See:** `output-styles/zebgp.md` for full guidelines

---

### 4. Understand Before Implementing (BLOCKING RULE)

```
┌─────────────────────────────────────────────────────────────────┐
│  FULLY UNDERSTAND what needs to be implemented BEFORE writing  │
│  any code or tests. Explore the codebase to understand:        │
│                                                                 │
│  - Where the implementation should go                          │
│  - How existing code handles similar cases                     │
│  - What the call chain looks like                              │
│  - What data structures are involved                           │
│                                                                 │
│  If unsure, ASK the user for clarification before proceeding.  │
└─────────────────────────────────────────────────────────────────┘
```

**Mandatory Before Implementation:**
1. **Run functional tests FIRST** to verify feature isn't already implemented
   - For API features: `go run ./test/cmd/self-check <test-code>`
   - For encode features: `go run ./test/cmd/self-check <test-code>`
   - If test passes, the feature EXISTS - don't reimplement it
2. Explore relevant code paths using Explore agent or grep/read
3. Understand where new code should be added
4. Identify what functions/methods need to be modified
5. If ambiguous, ask user for direction

**Anti-Pattern:** Writing tests/specs for features that already work. This wastes
effort and creates confusion. ALWAYS verify feature status with functional tests
before starting implementation work.

---

### 5. Test-Driven Development (TDD) - BLOCKING RULE

```
┌─────────────────────────────────────────────────────────────────┐
│  TESTS MUST EXIST AND FAIL BEFORE IMPLEMENTATION CODE EXISTS   │
│                                                                 │
│  Writing implementation without failing tests = VIOLATION      │
└─────────────────────────────────────────────────────────────────┘
```

**See:** `TDD_ENFORCEMENT.md` for complete workflow.

**TDD Cycle (MANDATORY - NO EXCEPTIONS):**

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

**Test Documentation Required:**

Every test MUST document:
```go
// TestFeatureName verifies [behavior].
//
// VALIDATES: [what correct behavior looks like]
//
// PREVENTS: [what bug/failure this catches]
func TestFeatureName(t *testing.T) { ... }
```

**Forbidden Actions:**

| Action | Violation | Correction |
|--------|-----------|------------|
| Write impl before test | Protocol violation | Delete impl, write test |
| Test passes immediately | Invalid test | Add failing assertion |
| Skip failure verification | No proof test works | Show failure first |

**During Development:**
```bash
# Run tests for current package (fast feedback)
go test -race ./pkg/bgp/message/... -v
```

**Before Claiming Done:**
```bash
make test && make lint  # Full suite + lint - ALL must pass
```

**When `make test` Fails (MANDATORY):**
1. Identify if failures are from YOUR changes or pre-existing
2. If YOUR changes caused failures: FIX THEM before commit
3. If pre-existing failures:
   - Document them in `plan/CLAUDE_CONTINUATION.md` under TEST STATUS
   - Include: test name, file:line, brief description
   - These block commit until fixed or user explicitly approves

---

### 6. Coding Standards

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

### 7. Solution Quality (Right Solution, Not Easy Solution)

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

### 8. Fix All Issues You Notice (No Broken Windows)

**Core principle:** If you see something wrong, fix it or document it for later.

**MANDATORY:**
- If you notice ANY issue (lint, bug, missing test), even if unrelated to current task:
  - **If quick to fix:** Fix it immediately with a test to prevent regression
  - **If not quick:** Add to `plan/CLAUDE_CONTINUATION.md` for later

**Rationale:** Issues left unfixed accumulate. Tests prevent regressions.

**Never:**
- Ignore issues because "not my change"
- Leave lint warnings unfixed without documenting
- Skip writing tests for edge cases you discover

---

## Load Additional Reference Based on Task

| Activity | Reference File |
|----------|----------------|
| **ANY implementation** | **TDD_ENFORCEMENT.md** (ALWAYS) |
| Writing Go code | CODING_STANDARDS.md |
| Writing protocol code | RFC_DOCUMENTATION_PROTOCOL.md |
| Test failures | TESTING_PROTOCOL.md, CI_TESTING.md |
| Creating docs | DOCUMENTATION_PLACEMENT_GUIDE.md |

**Note:** Git safety, error recovery, refactoring, and session end protocols are now consolidated in this file (see sections above).

---

## Functional Test System

**See:** `.claude/zebgp/FUNCTIONAL_TESTS.md` for complete documentation.

**Quick reference:**
- Encode tests: `test/data/encode/*.ci` + `.conf`
- API tests: `test/data/api/*.ci` + `.conf` + `.run`
- Run all functional tests: `make self-check`
- Run single test: `go run ./test/cmd/self-check <nick>`
- List tests: `go run ./test/cmd/self-check --list`

---

## Codebase Architecture Quick Reference

**Directory structure:**
```
zebgp/
├── cmd/
│   └── zebgp/           # Main daemon
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
├── plan/                # Plans, TODOs, tasks
└── test/data/            # Test fixtures
```

**Design patterns:**
- Registry/Factory for message types
- Interface-based polymorphism for NLRI/Attributes
- Goroutine per peer

---

## Quick Reference: Forbidden Phrases

**Without verification (command + output pasted):**
- "Fixed" / "Complete" / "Working" / "Ready"
- "All tests pass" / "Tests pass"
- Checkmarks without proof

**Without running `make test && make lint`:**
- "Done" / "Finished" / "Complete"
- Any claim code works

**Without explicit user request:**
- `git commit` / `git push`

**TDD Violations:**
- Writing ANY implementation before test exists
- Showing test pass without showing it failed first
- Tests without VALIDATES/PREVENTS documentation

**Self-Review Violations:**
- Claiming "done" without performing self-review
- Skipping re-review after fixing issues
- Not reporting 🟢 minor items to user

**Auto-fix:** Stop. Run tests. Paste output. Then claim.

**TDD auto-fix:** Delete implementation. Write test. Show failure. Implement.

**Review auto-fix:** Perform self-review. Fix 🔴/🟡. Re-review. Report 🟢.

---

## Self-Check Before Responding

Before EVERY response, verify:

- [ ] Did I check git status at session start?
- [ ] Did I ask about modified files before proceeding?
- [ ] **Before implementation:** Did I run `/prep` to create a spec?
- [ ] **For BGP code:** Did I read the RFC and check ExaBGP reference?
- [ ] Am I putting files in the correct location?
- [ ] Am I following TDD (test first, show failure, then implement)?
- [ ] Am I being terse and emoji-prefixed?
- [ ] Am I running commands and pasting output as proof?
- [ ] **After task completion:** Did I perform self-review?
- [ ] **After self-review:** Did I fix all 🔴/🟡 issues and re-review?
- [ ] **After clean review:** Did I report 🟢 minor items to user?

---

## Git Safety Protocol

### Core Rule

**NEVER commit or push without EXPLICIT user request.**

User must say one of: "commit", "make a commit", "git commit", "push", "git push"

**Completing work is NOT permission to commit.**

### Before ANY Git Operation

```bash
git status && git log --oneline -5
```

Check for:
- Unexpected modified files
- Staged changes you didn't make
- Current branch is correct

### MANDATORY: Run Tests Before Commit

```
┌─────────────────────────────────────────────────────────────────┐
│  ALWAYS run `make test && make lint` BEFORE committing.         │
│                                                                 │
│  WHY: To find issues and FIX THEM, not to document and bypass.  │
│                                                                 │
│  If `make test` or `make lint` fails:                           │
│  1. FIX the failures - this is the whole point of testing       │
│  2. Re-run until ALL pass                                       │
│  3. Only then proceed with commit                               │
│                                                                 │
│  DO NOT commit with failing tests or lint. FIX THEM FIRST.      │
└─────────────────────────────────────────────────────────────────┘
```

### Commit Workflow

1. **User explicitly requests commit** - Wait for "commit this", "make a commit"
2. **Check state:** `git status && git diff --staged`
3. **Add files:** `git add <specific-files>`
4. **Create commit** with conventional message:
   ```bash
   git commit -m "$(cat <<'EOF'
   <type>: <description>

   Generated with [Claude Code](https://claude.com/claude-code)

   Co-Authored-By: Claude <noreply@anthropic.com>
   EOF
   )"
   ```
   Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`
5. **Verify:** `git log --oneline -3 && git status`
6. **Update continuation file:** `plan/CLAUDE_CONTINUATION.md`
   - Move completed items to "COMPLETED" section
   - Update test status if changed
   - Add commit hash to relevant entries
   - This is MANDATORY after EVERY commit

### Forbidden Without Permission

**MUST ask before:**
- `git reset` (any form)
- `git revert`
- `git checkout -- <file>`
- `git restore` (to discard)
- `git stash drop`
- `git push --force`
- Deleting branches

**How to ask:**
```
I need to run `git reset --hard HEAD~1`. This will discard the last commit.
May I proceed? (yes/no)
```

### Pre-Commit Workflow

1. Complete work
2. Run `make test && make lint` - ALL must pass
3. **STOP** - do NOT commit yet
4. Report what was done to user
5. **WAIT** for user instruction
6. **Tests passing is NOT permission to commit**
7. Only commit if user EXPLICITLY says "commit"

### Enforcement Checklist

Before committing:
- [ ] User explicitly requested commit
- [ ] `git status` shows expected files
- [ ] `git diff --staged` reviewed
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Commit message follows convention

After committing:
- [ ] `plan/CLAUDE_CONTINUATION.md` updated with completed work
- [ ] Commit hash added to relevant entries
- [ ] Test status updated if changed

Before pushing:
- [ ] User explicitly requested push
- [ ] `git log origin/main..HEAD` shows expected commits
- [ ] Branch is correct

---

## Error Recovery Protocol

### Step 1: STOP

Do not try to fix immediately. Assess first.

### Step 2: Preserve Current State

```bash
git diff > .claude/backups/error-$(date +%Y%m%d-%H%M%S).patch
git diff --staged >> .claude/backups/error-$(date +%Y%m%d-%H%M%S).patch
```

### Step 3: Identify the Problem

| Problem | Command |
|---------|---------|
| Code not compiling | `go build ./...` |
| Tests failing | `go test ./... -v 2>&1 \| head -50` |
| Wrong approach | Document what was tried and why it failed |

### Step 4: Ask User

Present options:
```
Tests failing after change to X.

Options:
(a) Continue debugging - I see the issue is Y
(b) Revert to last working state
(c) Save current work and try different approach

Which?
```

**Wait for response before destructive actions.**

### Recovery Tools

```bash
# Git reflog - find lost commits
git reflog
git cherry-pick <sha>

# Saved patches
ls -la .claude/backups/
git apply .claude/backups/<filename>.patch

# Git stash
git stash list
git stash pop
```

### Anti-Patterns

**Don't:**
- Make more changes to "fix" without understanding the problem
- Delete or revert without saving first
- Claim success after recovery without re-running all tests

**Do:**
- Admit the mistake clearly
- Save state before attempting recovery
- Test thoroughly after recovery

---

## Refactoring Protocol

### Core Rule

**ONE function/type at a time. No batching.**

### Phase 0: Planning

Write numbered steps. Each MUST have:
```
Step N: [Action] [What] in [Where]
  Files: [exact paths]
  Verification: [exact command]
  Expected: "PASS"
```

**Get user approval before proceeding.**

### Phase 1-N: Execution

```
=== STEP N ===
[Make changes]
Verification: [run command]
OUTPUT:
[PASTE EXACT OUTPUT - NO SUMMARY]
Result: PASS
=== STEP N COMPLETE ===
```

**Rules:**
- Announce step
- Complete ONLY that step
- Run verification
- PASTE EXACT OUTPUT
- Stop if failures

**NEVER:** Skip verification, batch steps, summarize output, proceed with failures

### Git Strategy

- **Commit message:** `refactor: rename Pool.Get to Pool.Lookup`
- **When to commit:** Function renamed + all call sites + ALL tests pass
- **One function = one commit**

### Violation Detection

If I do these, I'm violating:
- Batch multiple steps together
- Summarize test output instead of pasting
- Proceed with ANY test failures
- Skip verification command
- Commit without full test suite passing

**Auto-fix:** Stop. Run verification. Paste output. Wait for pass before next step.

---

## Session End Checklist

### Before Ending ANY Session

#### 1. Plan File Updates

If you worked on ANY plan file:
- [ ] Updated "Last Updated" timestamp
- [ ] Documented progress made
- [ ] Documented any test failures
- [ ] Documented any blockers
- [ ] Updated "Resume Point" section

**Resume Point template:**
```markdown
## Resume Point

**Last worked:** YYYY-MM-DD
**Last commit:** [hash or "uncommitted"]
**Session ended:** Mid-task / Clean break / Blocked

**To resume:**
1. [Exact next step]
2. [Context needed]
3. [Watch out for: potential issues]
```

#### 2. Failure Documentation

If ANY tests failed:
- [ ] Each failure has entry in plan file
- [ ] Root cause documented (or "Unknown - needs investigation")
- [ ] Resolution documented (or "Pending")

#### 3. Git Status Check

```bash
git status
```

- [ ] Check if plan files are modified
- [ ] If modified: Include in next commit OR ask user

#### 4. Session Summary

Report to user:
```
Session summary:
- Plans updated: [list or "none"]
- Failures documented: [count or "none"]
- Blockers: [list or "none"]
- Next steps: [what to do next session]
```

---

## /prep Requirement (BLOCKING)

```
┌─────────────────────────────────────────────────────────────────┐
│  USE /prep BEFORE STARTING ANY IMPLEMENTATION TASK              │
│                                                                 │
│  /prep forces reading protocols and embeds rules in the spec.  │
│  Skipping /prep = skipping protocols = likely mistakes.        │
└─────────────────────────────────────────────────────────────────┘
```

**MANDATORY for:**
- Any task involving code changes
- Multi-file modifications
- Feature implementation
- Bug fixes
- Refactoring

**Workflow:**
1. User describes task
2. **IMMEDIATELY** run `/prep <task description>`
3. Spec is written to `plan/spec-<name>.md` with embedded rules
4. Read the spec before implementing
5. Follow the embedded protocol requirements

**Skipping /prep is a protocol violation.** The spec embeds rules I would otherwise skip.

---

**Updated:** 2025-12-27
