# Spec: ci-attribute-reorder

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. test/data/encode/*.ci - Sample test files                   │
│  6. pkg/bgp/attribute/origin.go - Attribute parsing code        │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Create a tool to reorder path attributes in .ci test files to comply with RFC 4271.

RFC 4271 Section 5: "Path attributes associated with a given BGP UPDATE message
MUST be encoded in ascending order of attribute type."

ExaBGP-generated .ci files have non-RFC attribute ordering (e.g., type 16 before type 14).
This tool fixes the raw bytes to use ascending type code order.

## Current State

- Tests: 24 passed, 13 failed
- Git: 3 modified files (mpls-vpn parsing, FlowSpec ordering - to be reverted)
- Issue: ZeBGP follows RFC, ExaBGP tests don't

## Context Loaded

```
📁 Files:
  - test/data/encode/*.ci (37 encode tests)
  - test/data/api/*.ci (14 api tests)
  - Raw format: MARKER:LENGTH:TYPE:PAYLOAD

📋 .ci raw line format:
  N:raw:FFFF...(32 hex):LLLL:TT:PAYLOAD
  - MARKER: 32 hex chars (16 bytes, all FF)
  - LENGTH: 4 hex chars (message length)
  - TYPE: 2 hex chars (1=OPEN, 2=UPDATE, 3=NOTIF, 4=KEEPALIVE)
  - PAYLOAD: hex-encoded message body

📋 UPDATE payload format:
  - 2 bytes: withdrawn routes length
  - N bytes: withdrawn routes
  - 2 bytes: path attributes length
  - N bytes: path attributes (NEED REORDERING)
  - Remaining: NLRI
```

## Problem Analysis

**Raw message example (non-RFC order):**
```
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0051:02:0000003A4001010040020040050400000064C01914000C2A020B800000000100000000000000010000800E1200018500000C0120C0A8000202200A000001
```

Breaking down payload `0000003A4001010040020040050400000064C01914...800E12...`:
- `0000` - withdrawn length (0)
- `003A` - path attributes length (58 bytes)
- Path attributes:
  - `4001 01 00` - ORIGIN (type 1)
  - `4002 00` - AS_PATH (type 2)
  - `4005 04 00000064` - LOCAL_PREF (type 5)
  - `C019 14 000C2A02...` - IPv6_EXT_COMMUNITY (type 25)
  - `800E 12 00018500...` - MP_REACH_NLRI (type 14)

Issue: Type 25 before type 14 (should be 14 before 25)

## Goal Achievement

```
🎯 User's goal: Fix .ci files for RFC compliance
| Check | Status |
|-------|--------|
| Tool parses raw? | ✅ Done |
| Attributes reordered? | ✅ Done |
| Files updated? | ✅ Done (12 files, 51 lines) |
| Tests pass? | ✅ Done (make test, make lint) |
Plan achieves goal: YES - COMPLETED
```

## Embedded Rules

- TDD: Write test for reorder function first
- Verify: make test && make lint before done
- RFC: Follow RFC 4271 Section 5 for attribute ordering

## Documentation Impact

- [ ] `plan/CLAUDE_CONTINUATION.md` - Update after completion
- [ ] `.claude/zebgp/FUNCTIONAL_TESTS.md` - Add note about RFC ordering

## Implementation Steps

### Phase 1: Revert Previous Changes

First, revert the ExaBGP-compatibility changes made to pkg/bgp/message/update_build.go
since we're going RFC-compliant instead.

### Phase 2: Create Tool

Create `test/cmd/ci-fix-order/main.go`:

```go
// ci-fix-order reorders path attributes in .ci files to RFC 4271 order.
//
// RFC 4271 Section 5: Path attributes MUST be in ascending type code order.
//
// Usage:
//   go run ./test/cmd/ci-fix-order [options] [files...]
//
// Options:
//   --dry-run    Show changes without modifying files
//   --all        Process all .ci files
//   --verbose    Show detailed output

package main
```

**Algorithm:**
1. Read .ci file line by line
2. For each `N:raw:` line:
   a. Parse MARKER:LENGTH:TYPE:PAYLOAD
   b. If TYPE != 02 (UPDATE), skip
   c. Decode payload hex → bytes
   d. Parse withdrawn routes length, skip withdrawn
   e. Parse path attributes length
   f. Parse each attribute: flags, code, length, value
   g. Sort attributes by code (ascending)
   h. Re-encode attributes
   i. Update LENGTH field if needed
   j. Rebuild raw line
3. Write updated file

### Phase 3: Test

```bash
# Dry run on single file
go run ./test/cmd/ci-fix-order --dry-run test/data/encode/flow-redirect.ci

# Fix all files
go run ./test/cmd/ci-fix-order --all

# Verify tests pass
make self-check
```

### Phase 4: Verification

```bash
make test && make lint
go run ./test/cmd/self-check --all
```

## Checklist

- [x] Previous FlowSpec ordering changes reverted (fixed in update_build.go)
- [x] Tool parses .ci raw lines correctly
- [x] Attributes reordered to ascending code
- [x] Files updated correctly (12 files, 51 lines)
- [x] make test passes
- [x] make lint passes
- [x] Functional tests pass (flow-redirect; others have unrelated issues)
- [x] Spec moved to plan/done/
- [ ] plan/README.md updated

## Completion Notes

**Completed:** 2024-12-30
**Commit:** 7f6c7e9

**Additional fixes made:**
- Fixed incorrect comment in update_build.go claiming ExaBGP uses wrong order
- ZeBGP now correctly orders MP_REACH_NLRI (14) before EXT_COMMUNITIES (16)

**Remaining issues (unrelated to attribute ordering):**
- vpn/mvpn tests: Missing `split` directive support in config loader
- Extended community sorting: ExaBGP sorts by value, ZeBGP preserves config order
