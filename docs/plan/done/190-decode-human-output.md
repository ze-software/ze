# Spec: Human-Readable Decode Output

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/bgp/decode.go` - current decode implementation
4. `internal/plugin/text.go` - existing text formatters (FormatOpen, etc.)
5. `internal/plugin/decode.go` - DecodedOpen, DecodedCapability structs

## Task

Add human-readable output format to `ze bgp decode` command as the default, with `--json` flag to get current ExaBGP-compatible JSON output.

**Current behavior:**
- `ze bgp decode --open <hex>` → Always outputs ExaBGP JSON

**Target behavior:**
- `ze bgp decode --open <hex>` → Human-readable output (default)
- `ze bgp decode --open --json <hex>` → ExaBGP JSON (current behavior)
- Same pattern for `--update` and `--nlri`

~~Plugin protocol extended with format argument~~ **Deferred:** See `spec-decode-plugin-text.md`

## Required Reading

### Architecture Docs
- [x] `cmd/ze/bgp/decode.go` - current decode implementation
- [x] `internal/plugin/text.go` - existing text formatters
- [x] `internal/plugin/decode.go` - DecodedOpen, DecodedCapability structs

### RFC Summaries
- [x] `rfc/short/rfc4271.md` - BGP-4 message formats (not needed - presentation only)

**Key insights:**
- Existing `FormatOpen()` in text.go outputs: `peer <ip> <direction> open <msg-id> asn <asn> router-id <id> hold-time <t> [cap <code> <name> <value>]...`
- For decode command, there is no peer context or message ID - format should be simpler
- Human-readable output should be easy to read at a glance

## Current Behavior

**Source files read:**
- [x] `cmd/ze/bgp/decode.go` - decodeHexPacket() returns JSON string
- [x] `internal/plugin/text.go` - FormatOpen(), FormatNotification() patterns

**Behavior to preserve:**
- All JSON output format when `--json` flag is used
- Error handling returns JSON errors

**Behavior to change:**
- Default output changes from JSON to human-readable
- Add `--json` flag to get previous behavior

## Human-Readable Output Format

### OPEN Message

```
BGP OPEN Message
  Version:     4
  ASN:         65533
  Hold Time:   180 seconds
  Router ID:   10.0.0.2
  Capabilities:
    multiprotocol        ipv4/unicast
    asn4                 65533
    fqdn                 my-host-name.my-domain-name.com
```

### UPDATE Message

```
BGP UPDATE Message
  Attributes:
    origin               igp
    as-path              65001 65002 65003
    next-hop             10.0.0.1
    local-preference     100
    med                  50
  Announced (ipv4/unicast):
    10.0.0.0/24
    10.0.1.0/24
  Withdrawn (ipv4/unicast):
    10.0.2.0/24
```

### NLRI Only

```
FlowSpec NLRI (ipv4/flow):
  destination          10.0.0.0/24
  protocol             6 (TCP)
  destination-port     =80
```

### Errors

```
Error: invalid hex: encoding/hex: odd length hex string
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeOpenHuman` | `cmd/ze/bgp/decode_test.go` | Human-readable OPEN output | ✅ |
| `TestDecodeOpenJSON` | `cmd/ze/bgp/decode_test.go` | JSON OPEN output with --json | ✅ |
| `TestDecodeUpdateHuman` | `cmd/ze/bgp/decode_test.go` | Human-readable UPDATE output | ✅ |
| `TestDecodeUpdateJSON` | `cmd/ze/bgp/decode_test.go` | JSON UPDATE output with --json | ✅ |
| `TestDecodeNLRIHuman` | `cmd/ze/bgp/decode_test.go` | Human-readable NLRI output | ✅ |
| `TestDecodeNLRIJSON` | `cmd/ze/bgp/decode_test.go` | JSON NLRI output with --json | ✅ |
| `TestDecodeErrorHuman` | `cmd/ze/bgp/decode_test.go` | Human error messages | ✅ |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All decode tests | `test/decode/*.ci` | JSON decode with --json | ✅ 20/20 |

## Files to Modify

- `cmd/ze/bgp/decode.go` - `--json` flag, human-readable formatters (already done)
- `internal/test/runner/decoding.go` - Parse `--json` flag from .ci files
- `test/decode/*.ci` - Add `--json` flag to commands expecting JSON output

## Implementation Steps

### Phase 1: CLI and Built-in Formatters ✅ COMPLETE

1. **Add --json flag** to cmdDecode() ✅
   - Line 39: `outputJSON := fs.Bool("json", false, "output JSON instead of human-readable format")`

2. **Create formatOpenHuman()** function ✅
   - Line 2079: Structured multi-line OPEN output

3. **Create formatUpdateHuman()** function ✅
   - Line 2186: Structured multi-line UPDATE output

4. **Create formatNLRIHuman()** function ✅
   - Line 2332: Structured NLRI output

5. **Update decodeHexPacket()** to accept outputJSON bool ✅
   - Line 115: Routes to human or JSON formatters

6. **Fix test runner** ✅
   - Added `OutputJSON` field to `DecodingTest` struct
   - Updated `parseDecodeCmdLine()` to extract `--json` flag from .ci files
   - Updated `runTest()` to conditionally add `--json` based on test config

7. **Update .ci test files** ✅
   - Added `--json` flag to all decode test commands that expect JSON output

### ~~Phase 2: Plugin Protocol Extension~~ DEFERRED

**Moved to separate spec:** `spec-decode-plugin-text.md`

The plugin protocol extension (adding `decode text` format) is deferred because:
- Current implementation works via fallback: request JSON from plugin, format with built-in formatters
- This is the "Backward Compatibility" path described in original spec
- Native plugin text support is an optimization, not required for correct behavior

## Design Decisions

### Why Human-Readable as Default?

| Option | Pros | Cons |
|--------|------|------|
| JSON default | Consistent with current behavior | Not human-friendly for debugging |
| Human default | Easy to read at a glance | Breaking change (mitigated by --json) |

**Decision:** Human default. The decode command is primarily for debugging/inspection. Scripts should use `--json`.

### Output Style

| Option | Example | Pros | Cons |
|--------|---------|------|------|
| Single line | `OPEN asn=65533 rid=10.0.0.2` | Compact | Hard to scan |
| Structured | See format above | Easy to read | More lines |

**Decision:** Structured multi-line. Optimized for human scanning.

## Checklist

### 🏗️ Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling
- [x] Next-developer test

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified before impl)
- [x] Implementation complete
- [x] Tests PASS (all 7 unit tests)
- [x] Boundary tests cover all numeric inputs (N/A)
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior (20/20)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` decode tests pass (20/20)

### Documentation
- [x] Required docs read
- [x] RFC summaries read (N/A - presentation only)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)

### Completion
- [x] Architecture docs updated with learnings (N/A)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together

## Implementation Summary

### What Was Implemented

Phase 1 was already complete when this task started:

1. **CLI with `--json` flag** (`cmd/ze/bgp/decode.go:39`)
   - Default output: human-readable format
   - `--json` or `-json`: ExaBGP-compatible JSON (previous default)

2. **Human-readable formatters** (`cmd/ze/bgp/decode.go:2074-2386`)
   - `formatOpenHuman()` - structured OPEN message output
   - `formatUpdateHuman()` - structured UPDATE message output
   - `formatNLRIHuman()` - structured NLRI output
   - Helper functions for capabilities, attributes, AS-PATH

3. **Unit tests** (`cmd/ze/bgp/decode_test.go:979-1197`)
   - 7 tests covering human and JSON output paths

### Bug Found/Fixed

**Functional test runner not parsing `--json` flag:**
- The test runner in `internal/test/runner/decoding.go` was not parsing `--json` from .ci files
- Tests expected JSON output (`expect:json:`) but command now defaults to human-readable
- Fix:
  1. Added `OutputJSON` field to `DecodingTest` struct
  2. Updated `parseDecodeCmdLine()` to extract `--json` flag
  3. Updated `runTest()` to conditionally add `--json`
  4. Updated all `.ci` files to include `--json` in their commands

### Deferred Work

**Plugin protocol extension** moved to `spec-decode-plugin-text.md`:
- Extend protocol: `decode json/text capability <code> <hex>`
- Hostname plugin native text support
- FlowSpec plugin native text support
- Update process-protocol.md documentation

Current behavior uses fallback: plugins return JSON, CLI formats with built-in formatters.
