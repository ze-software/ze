# Documentation Placement Guide

**When to read:** Before creating ANY new .md file
**Prerequisites:** ESSENTIAL_PROTOCOLS.md
**Size:** ~3 KB

---

## Quick Decision Tree

**Before creating a doc, ask:**

### 1. Is this about HOW WE WORK (protocols, rules)?

**YES** → `.claude/` (root level)

Examples: verification rules, git workflow, coding standards

### 2. Is this a WIRE FORMAT or PROTOCOL REFERENCE?

**YES** → `.claude/zebgp/wire/` or `.claude/zebgp/`

Examples: NLRI formats, attribute encoding, capability negotiation

### 3. Is this an IMPLEMENTATION PLAN?

**YES** → `plan/` (project root)

Examples: feature plans, refactoring plans, phase breakdowns

### 4. Is this about COMPLETED WORK?

**YES** → `.claude/docs/projects/`

Examples: post-mortems, design decisions, historical context

### 5. Is this about EDGE CASES or SPECIAL BEHAVIOR?

**YES** → `.claude/zebgp/edge-cases/`

Examples: AS4 handling, extended messages, add-path

---

## Directory Structure

```
ze/
├── plan/                              # Implementation plans (project root)
│   ├── README.md
│   ├── ARCHITECTURE.md
│   ├── <name>.md                      # Active/planned work
│   └── done/                          # Completed plans
│       └── <name>.md
│
├── .claude/
│   ├── # PROTOCOLS (how we work)
│   ├── ESSENTIAL_PROTOCOLS.md
│   ├── CODING_STANDARDS.md
│   ├── TESTING_PROTOCOL.md
│   ├── GIT_VERIFICATION_PROTOCOL.md
│   ├── RFC_DOCUMENTATION_PROTOCOL.md
│   ├── MANDATORY_REFACTORING_PROTOCOL.md
│   ├── ERROR_RECOVERY_PROTOCOL.md
│   ├── CI_TESTING.md
│   ├── SESSION_END_CHECKLIST.md
│   ├── PRE_FLIGHT_CHECKLIST.md
│   ├── FILE_NAMING_CONVENTIONS.md
│   ├── DOCUMENTATION_PLACEMENT_GUIDE.md  # This file
│   ├── README.md
│   │
│   ├── output-styles/
│   │   └── zebgp.md                   # Communication style
│   │
│   ├── # CODEBASE REFERENCE
│   ├── zebgp/
│   │   ├── POOL_ARCHITECTURE.md       # Design docs
│   │   ├── POOL_ARCHITECTURE_REVIEW.md
│   │   ├── MESSAGE_BUFFER_DESIGN.md
│   │   ├── EXABGP_CODE_MAP.md         # ExaBGP compatibility
│   │   ├── TEST_INVENTORY.md          # Test cases
│   │   ├── wire/                      # Wire format docs
│   │   │   ├── MESSAGES.md
│   │   │   ├── ATTRIBUTES.md
│   │   │   ├── NLRI.md
│   │   │   ├── NLRI_EVPN.md
│   │   │   ├── NLRI_FLOWSPEC.md
│   │   │   ├── NLRI_BGPLS.md
│   │   │   ├── CAPABILITIES.md
│   │   │   └── QUALIFIERS.md
│   │   ├── api/                       # API docs
│   │   │   ├── COMMANDS.md
│   │   │   ├── JSON_FORMAT.md
│   │   │   └── PROCESS_PROTOCOL.md
│   │   ├── config/                    # Config docs
│   │   │   ├── SYNTAX.md
│   │   │   ├── ENVIRONMENT.md
│   │   │   └── TOKENIZER.md
│   │   ├── behavior/                  # Runtime behavior
│   │   │   ├── FSM.md
│   │   │   └── SIGNALS.md
│   │   └── edge-cases/                # Special handling
│   │       ├── AS4.md
│   │       ├── ADDPATH.md
│   │       └── EXTENDED_MESSAGE.md
│   │
│   ├── # DOCUMENTATION
│   ├── docs/
│   │   ├── README.md
│   │   └── projects/                  # Completed projects
│   │
│   ├── hooks/                         # Claude hooks
│   │   └── README.md
│   │
│   └── backups/                       # Work preservation
```

---

## Quick Reference Table

| Doc Type | Location | Example |
|----------|----------|---------|
| Work protocols | `.claude/` | CODING_STANDARDS.md |
| Wire formats | `.claude/zebgp/wire/` | NLRI_EVPN.md |
| Design docs | `.claude/zebgp/` | POOL_ARCHITECTURE.md |
| Edge cases | `.claude/zebgp/edge-cases/` | AS4.md |
| API reference | `.claude/zebgp/api/` | COMMANDS.md |
| Config reference | `.claude/zebgp/config/` | SYNTAX.md |
| Implementation plans | `plan/` | pool-integration.md |
| Completed plans | `plan/done/` | unified-commit-system.md |
| Completed work | `.claude/docs/projects/` | pool-implementation/ |

---

## Examples

### "I want to document the EVPN wire format"

**Decision:**
- Wire format? YES
- **Location:** `.claude/zebgp/wire/NLRI_EVPN.md` (already exists)

### "I want to plan a new feature"

**Decision:**
- Implementation plan? YES
- **Location:** `plan/<feature-name>.md`

### "I completed a plan"

**Decision:**
- Move to done folder
- **Location:** `git mv plan/<name>.md plan/done/<name>.md`

### "I want to document git workflow rules"

**Decision:**
- Protocol/rules? YES
- **Location:** `.claude/GIT_VERIFICATION_PROTOCOL.md` (already exists)

---

## File Size Policy

- Core protocols: < 5 KB
- Reference docs: < 8 KB
- Plans: < 10 KB
- READMEs: < 3 KB

**If exceeding: split or compress**

---

## Golden Rule

**If you can't decide, ask the user.**

---

**Updated:** 2025-12-19
