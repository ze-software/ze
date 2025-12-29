# ZeBGP Documentation Index

**Purpose:** Find what to read. Each doc has TL;DR at top.

---

## Quick Navigation

| Task Type | Start Here | Then Read |
|-----------|------------|-----------|
| **BGP wire format** | `zebgp/wire/MESSAGES.md` | ATTRIBUTES, NLRI, CAPABILITIES |
| **New NLRI type** | `zebgp/wire/NLRI.md` | `EXABGP_CODE_MAP.md` |
| **New attribute** | `zebgp/wire/ATTRIBUTES.md` | `EXABGP_CODE_MAP.md` |
| **Capability work** | `zebgp/wire/CAPABILITIES.md` | `ENCODING_CONTEXT.md` |
| **FSM/peer session** | `zebgp/behavior/FSM.md` | `zebgp/behavior/SIGNALS.md` |
| **API work** | `zebgp/api/ARCHITECTURE.md` | `api/COMMANDS.md`, `JSON_FORMAT.md` |
| **Config parsing** | `zebgp/config/SYNTAX.md` | `zebgp/config/TOKENIZER.md` |
| **Memory/pools** | `zebgp/POOL_ARCHITECTURE.md` | `MESSAGE_BUFFER_DESIGN.md` |
| **Zero-copy routing** | `zebgp/ENCODING_CONTEXT.md` | `POOL_ARCHITECTURE.md` |
| **Writing tests** | `TDD_ENFORCEMENT.md` | `TESTING_PROTOCOL.md` |
| **Writing Go code** | `CODING_STANDARDS.md` | `QUICK_REFERENCE.md` |
| **ExaBGP compat** | `zebgp/EXABGP_CODE_MAP.md` | `EXABGP_DIFFERENCES.md` |

---

## Documentation Tree

```
.claude/
├── ESSENTIAL_PROTOCOLS.md   # Session rules (READ EVERY SESSION)
├── INDEX.md                 # This file
├── QUICK_REFERENCE.md       # Essential patterns (READ BEFORE CODE)
├── CONTEXT_LOADING.md       # How to load context
├── TDD_ENFORCEMENT.md       # Test-first workflow
├── CODING_STANDARDS.md      # Go patterns
│
├── zebgp/                   # PROJECT-SPECIFIC
│   ├── ENCODING_CONTEXT.md  # Zero-copy design
│   ├── POOL_ARCHITECTURE.md # Memory pools
│   ├── EXABGP_CODE_MAP.md   # ExaBGP → ZeBGP
│   │
│   ├── wire/                # WIRE FORMATS
│   │   ├── MESSAGES.md
│   │   ├── ATTRIBUTES.md
│   │   ├── NLRI.md
│   │   └── CAPABILITIES.md
│   │
│   ├── behavior/            # RUNTIME
│   │   └── FSM.md
│   │
│   ├── config/              # CONFIGURATION
│   │   └── SYNTAX.md
│   │
│   └── api/                 # EXTERNAL API
│       └── ARCHITECTURE.md
│
└── commands/
    └── prep.md              # /prep skill
```

---

## Reading Order by Task

### Before ANY implementation:
1. `INDEX.md` (this file)
2. `QUICK_REFERENCE.md`
3. Task-specific docs (see Quick Navigation)
4. Actual source code

### For wire format work:
1. `rfc/rfcNNNN.txt` (the RFC)
2. `wire/MESSAGES.md`
3. Specific format doc
4. `EXABGP_CODE_MAP.md`

### For API work:
1. `api/ARCHITECTURE.md`
2. Source: `pkg/api/server.go`, `pkg/api/command.go`

### For pool/memory work:
1. `POOL_ARCHITECTURE.md`
2. `ENCODING_CONTEXT.md`
3. Source: `internal/store/`, `pkg/bgp/context/`

---

## Every Doc Has TL;DR

Each architecture doc starts with a TL;DR table:
- Key concepts
- Key types/functions
- When to read full doc

Read the TL;DR first. Read full doc only if needed.
