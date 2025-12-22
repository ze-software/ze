# File Naming Conventions

**Last Updated:** 2025-12-19

## Standard: lowercase-with-hyphens for docs, snake_case for Go

---

## Rules

### 1. Special Top-Level Files (UPPERCASE)

These files use UPPERCASE because they are standard project files:

- `README.md` - Project readme
- `INDEX.md` - Top-level index
- `CHANGELOG.md` - Change history
- `LICENSE` - License file
- `Makefile` - Build file

### 2. AI Assistant Protocol Files in `.claude/` (UPPERCASE)

Files in `.claude/` directory use UPPERCASE for critical protocols:

- `ESSENTIAL_PROTOCOLS.md`
- `MANDATORY_REFACTORING_PROTOCOL.md`
- `GIT_VERIFICATION_PROTOCOL.md`
- `CODING_STANDARDS.md`
- `TESTING_PROTOCOL.md`
- `CI_TESTING.md`

**Rationale:** UPPERCASE signals critical nature, similar to README.md.

### 3. Documentation Files (lowercase-with-hyphens)

```
Good:
async-architecture.md
api-integration.md
pool-completion.md
phase-1-wireformat.md

Bad:
ASYNC_ARCHITECTURE.md
AsyncArchitecture.md
async_architecture.md
```

### 4. Go Source Files (snake_case)

Follow Go conventions:

```
Good:
pool.go
pool_test.go
compaction_scheduler.go
evpn_type2.go

Bad:
Pool.go
pool-test.go
compactionScheduler.go
```

### 5. Go Package Names (lowercase, single word preferred)

```
Good:
package pool
package message
package nlri

Bad:
package Pool
package bgp_message
package nlri_types
```

---

## Directory Structure Examples

### Documentation

```
.claude/
├── README.md                          # Special file
├── ESSENTIAL_PROTOCOLS.md             # Protocol (UPPERCASE)
├── zebgp/                             # Reference docs
│   ├── wire/
│   │   ├── MESSAGES.md               # Reference (UPPERCASE)
│   │   ├── NLRI.md
│   │   └── ATTRIBUTES.md
│   └── edge-cases/
│       ├── AS4.md
│       └── ADDPATH.md
└── docs/
    └── README.md
```

### Go Code

```
pkg/
├── bgp/
│   ├── message/
│   │   ├── message.go
│   │   ├── open.go
│   │   ├── update.go
│   │   └── message_test.go
│   ├── nlri/
│   │   ├── nlri.go
│   │   ├── inet.go
│   │   ├── evpn_type2.go
│   │   └── nlri_test.go
│   └── attribute/
│       ├── attribute.go
│       └── as_path.go
internal/
└── pool/
    ├── pool.go
    ├── pool_test.go
    ├── handle.go
    ├── compaction.go
    └── scheduler.go
```

### Plan Files

```
plan/
├── README.md                          # Index
├── ARCHITECTURE.md                    # Reference (UPPERCASE)
├── pool-integration.md                # Active/planned (lowercase)
├── peer-encoding-extraction.md        # Active/planned (lowercase)
└── done/                              # Completed plans folder
    ├── unified-commit-system.md
    └── rfc7606-extension.md
```

---

## Go-Specific Conventions

### Test Files

```
foo.go          # Implementation
foo_test.go     # Tests for foo.go
```

### Build-Tagged Files

```
debug.go              # Default
debug_release.go      # Build tag: !debug
platform_linux.go     # Build tag: linux
platform_darwin.go    # Build tag: darwin
```

### Generated Files

```
generated.go          # Hand-edited
foo_gen.go           # Generated (suffix _gen)
foo.pb.go            # Protocol buffers
```

---

## Migration Guide

If you find non-conforming files:

```bash
# Rename documentation
git mv ASYNC_ARCHITECTURE.md async-architecture.md

# Update references
grep -rn "ASYNC_ARCHITECTURE.md" . | cut -d: -f1 | xargs sed -i '' 's/ASYNC_ARCHITECTURE\.md/async-architecture.md/g'
```

---

## Rationale Summary

| Type | Convention | Why |
|------|------------|-----|
| Docs | lowercase-hyphens | Web-standard, easy to type |
| Go files | snake_case | Go convention |
| Packages | lowercase | Go requirement |
| Protocols | UPPERCASE | Signal importance |
| Special | UPPERCASE | Universal convention |

---

**Updated:** 2025-12-19
