# Claude AI Assistant Resources

Documentation and protocols for Claude Code interactions with ZeBGP.

---

## START OF EVERY SESSION

**You have NO memory between sessions**

### MANDATORY FIRST ACTIONS

1. **Read `plan/CLAUDE_CONTINUATION.md`**
   - Current development state and priorities
   - Test status (what passes/fails and why)
   - What was accomplished in previous sessions
   - Key file locations

2. **Read `ESSENTIAL_PROTOCOLS.md`** (~5 KB)
   - Core rules for ALL interactions
   - Verification before claiming success
   - Communication style (terse, emoji-prefixed)
   - Testing requirements (make test)
   - Coding standards essentials

### Then Check Git State

```bash
git status && git diff && git diff --staged
```

If ANY files modified/staged: ASK user how to handle before starting work.

### Load Contextual Protocols Based on Task

**Most tasks are covered by the above.** Only load additional protocols when explicitly needed.

---

## Protocol Files (Tiered System)

### Tier 1: Essential Core (Read Every Session)

- **ESSENTIAL_PROTOCOLS.md** - Core rules for ALL interactions (~5 KB)
  - TDD requirement (tests BEFORE implementation)
  - Verification before claiming
  - Communication style
  - Testing requirements
  - Coding standards
  - Git workflow essentials

- **TDD_ENFORCEMENT.md** - Test-Driven Development workflow (~8 KB)
  - Tests MUST exist and FAIL before implementation
  - Test documentation (VALIDATES/PREVENTS)
  - Fuzzing requirements for all parsers
  - TDD execution template

### Tier 2: Contextual Protocols (Load When Relevant)

**Git & Version Control:**
- GIT_VERIFICATION_PROTOCOL.md - Complete git safety workflow

**Code Quality:**
- CODING_STANDARDS.md - Go coding standards (detailed)
- MANDATORY_REFACTORING_PROTOCOL.md - Safe refactoring (one function at a time)
- ERROR_RECOVERY_PROTOCOL.md - Mistake recovery workflow

**Testing:**
- TESTING_PROTOCOL.md - Go testing discipline
- CI_TESTING.md - Test commands and coverage
- RFC_DOCUMENTATION_PROTOCOL.md - Document wire formats before coding

**Session Management:**
- PRE_FLIGHT_CHECKLIST.md - Session start checklist
- SESSION_END_CHECKLIST.md - Session end checklist

**Documentation:**
- DOCUMENTATION_PLACEMENT_GUIDE.md - Where to put docs
- FILE_NAMING_CONVENTIONS.md - Naming patterns

### Tier 3: Reference Materials (Consult When Needed)

- zebgp/POOL_ARCHITECTURE.md - Pool design
- zebgp/wire/*.md - Wire format documentation
- zebgp/edge-cases/*.md - Special case handling
- zebgp/EXABGP_CODE_MAP.md - ExaBGP compatibility reference

---

## Directory Structure

```
.claude/
├── README.md                          # This file
├── ESSENTIAL_PROTOCOLS.md             # Core rules (READ EVERY SESSION)
├── TDD_ENFORCEMENT.md                 # TDD workflow (READ EVERY SESSION)
├── CODING_STANDARDS.md                # Go coding standards
├── TESTING_PROTOCOL.md                # Testing discipline
├── CI_TESTING.md                      # Test commands reference
├── GIT_VERIFICATION_PROTOCOL.md       # Git safety
├── MANDATORY_REFACTORING_PROTOCOL.md  # Safe refactoring
├── RFC_DOCUMENTATION_PROTOCOL.md      # Wire format documentation
├── ERROR_RECOVERY_PROTOCOL.md         # Recovery workflow
├── PRE_FLIGHT_CHECKLIST.md            # Session start
├── SESSION_END_CHECKLIST.md           # Session end
├── FILE_NAMING_CONVENTIONS.md         # Naming patterns
├── DOCUMENTATION_PLACEMENT_GUIDE.md   # Where to put docs
├── settings.local.json                # Claude settings
├── output-styles/
│   └── zebgp.md                       # Communication style
├── hooks/
│   └── README.md                      # Hooks documentation
├── zebgp/                             # Codebase reference
│   ├── POOL_ARCHITECTURE.md           # Pool design
│   ├── POOL_ARCHITECTURE_REVIEW.md    # Pool issues
│   ├── MESSAGE_BUFFER_DESIGN.md       # Buffer design
│   ├── EXABGP_CODE_MAP.md             # ExaBGP compatibility
│   ├── TEST_INVENTORY.md              # Test cases
│   ├── wire/                          # Wire format docs
│   ├── api/                           # API docs
│   ├── config/                        # Config docs
│   ├── behavior/                      # Runtime behavior
│   └── edge-cases/                    # Special handling
├── backups/                           # Work preservation
└── docs/
    └── README.md                      # Documentation index
```

---

## Project Reference

**Implementation Plan:** `ZE_IMPLEMENTATION_PLAN.md` (project root)

**ExaBGP Reference:** `../main/` (Python implementation for compatibility testing)

**Key Design Documents:**
- Phase 1-2: Foundation, Wire Format
- Phase 3-6: Messages, Capabilities, Attributes, NLRI
- Phase 7-9: RIB, FSM, Reactor
- Phase 10-14: Config, CLI, API, Testing, Integration

---

## What Do You Want to Do?

| Task | Read These Docs |
|------|-----------------|
| Understand the project | ZE_IMPLEMENTATION_PLAN.md, plan/ARCHITECTURE.md |
| **Implement ANY feature** | **TDD_ENFORCEMENT.md** (tests first!) |
| Write Go code | CODING_STANDARDS.md, TDD_ENFORCEMENT.md |
| Write protocol code | RFC_DOCUMENTATION_PROTOCOL.md, zebgp/wire/*.md |
| Run tests | TESTING_PROTOCOL.md, CI_TESTING.md |
| Commit changes | GIT_VERIFICATION_PROTOCOL.md |
| Refactor code | MANDATORY_REFACTORING_PROTOCOL.md |
| Recover from error | ERROR_RECOVERY_PROTOCOL.md |
| Create documentation | DOCUMENTATION_PLACEMENT_GUIDE.md |
| Understand pools | zebgp/POOL_ARCHITECTURE.md |
| Check ExaBGP compat | zebgp/EXABGP_CODE_MAP.md |
| Handle edge cases | zebgp/edge-cases/*.md |

---

## Quick Start

**At session start:**
1. Read ESSENTIAL_PROTOCOLS.md
2. Check `git status`, `git diff`, `git diff --staged`
3. If files modified: ASK user before proceeding

**For any code changes:**
1. Write tests FIRST (TDD)
2. Make changes following CODING_STANDARDS.md
3. Run `make test`
4. Only THEN claim success

---

## Testing Quick Reference

```bash
# Before claiming "fixed"/"ready"/"complete":
make lint        # golangci-lint
make test        # go test -race ./...
make build       # go build ./...
```

**All must pass. No exceptions.**

---

## Functional Testing Tools

| Tool | Purpose | Usage |
|------|---------|-------|
| `zebgp-peer` | BGP test peer (ExaBGP bgp port) | `zebgp-peer --sink --port 1790` |
| `pkg/testpeer` | Testpeer as library | `peer := testpeer.New(&Config{...})` |
| `self-check` | Functional test runner | `self-check --all` |

**See CI_TESTING.md for full documentation.**

### Quick Examples

```bash
# Run BGP test peer in sink mode
zebgp-peer --sink --port 1790

# Run test peer with expected messages
zebgp-peer --port 1790 qa/encoding/test.msg

# Run all functional tests
self-check --all

# List available functional tests
self-check --list
```

### Using testpeer in Go tests

```go
import "github.com/exa-networks/zebgp/pkg/testpeer"

peer := testpeer.New(&testpeer.Config{
    Port: 1790,
    Sink: true,
    Output: &bytes.Buffer{},
})
result := peer.Run(ctx)
```

---

**Last Updated:** 2025-12-19
