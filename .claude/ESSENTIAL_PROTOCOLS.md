# Essential Protocols (READ EVERY SESSION)

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║   STOP. DO NOT PROCEED UNTIL YOU COMPLETE THE SESSION START CHECKLIST BELOW  ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

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
│  ExaBGP does it in ../src/exabgp/                               │
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
2. Find the equivalent code in `../src/exabgp/bgp/`
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

**Key ExaBGP directories:**
- `../src/exabgp/bgp/message/` - Message encoding/decoding
- `../src/exabgp/bgp/message/open/capability/` - Capability negotiation
- `../src/exabgp/bgp/message/update/attribute/` - Path attributes
- `../src/exabgp/bgp/message/update/nlri/` - NLRI types

**Why:** ExaBGP is the reference for API compatibility and tests validate
against ExaBGP output. But RFC compliance ensures interoperability with
all BGP implementations, not just ExaBGP.

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
1. Explore relevant code paths using Explore agent or grep/read
2. Understand where new code should be added
3. Identify what functions/methods need to be modified
4. If ambiguous, ask user for direction

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

## Load Contextual Protocols Based on Task

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

## Functional Test System

**See:** `.claude/zebgp/FUNCTIONAL_TESTS.md` for complete documentation.

**Quick reference:**
- Encode tests: `test/data/encode/*.ci` + `.conf`
- API tests: `test/data/api/*.ci` + `.conf` + `.run`
- Run single test: `./self-check <nick>` (e.g., `./self-check ah`)
- List tests: `./self-check -list`

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
├── plan/                # Plans, TODOs, tasks
└── test/data/            # Test fixtures
```

**Design patterns:**
- Registry/Factory for message types
- Interface-based polymorphism for NLRI/Attributes
- Goroutine per peer

---

## Git Workflow Essentials

**NEVER commit/push without explicit user request.**

**User must say:** "commit" / "make a commit" / "push"

**MANDATORY: Run `make test && make lint` Before Commit**
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

**Before ANY git operation:**
```bash
git status && git log --oneline -5
```

**Workflow:**
1. Complete work
2. Run `make test` - verify ALL pass or document failures
3. STOP and report what was done
4. WAIT for user instruction
5. Only commit/push if explicitly asked

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

**Auto-fix:** Stop. Run tests. Paste output. Then claim.

**TDD auto-fix:** Delete implementation. Write test. Show failure. Implement.

---

## Self-Check Before Responding

Before EVERY response, verify:

- [ ] Did I check git status at session start?
- [ ] Did I ask about modified files before proceeding?
- [ ] Am I putting files in the correct location?
- [ ] Am I following TDD (test first, show failure, then implement)?
- [ ] Am I being terse and emoji-prefixed?
- [ ] Am I running commands and pasting output as proof?

---

**Updated:** 2025-12-21
