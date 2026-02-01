# Spec: fix-json

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/json-format.md` - JSON format reference
4. `internal/api/json.go` - current JSON encoding implementation

## Task

Fix documentation to match the actual Ze JSON format produced by the code. The code produces the correct format; the docs have examples with an outdated structure.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/json-format.md` - primary format reference
- [ ] `docs/architecture/api/ipc_protocol.md` - protocol documentation
- [ ] `docs/architecture/api/architecture.md` - API architecture

### Source Files
- [ ] `internal/api/json.go` - actual JSON encoder (source of truth)

**Key insights:**
- Code produces correct format with `bgp.message.type`, docs show old `bgp.type` structure
- No version numbers should appear in docs (Ze has no released versions)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/api/json.go` - produces correct JSON structure

**Actual JSON format (from code):**

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "update", "id": 123, "direction": "received"},
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "attr": {"origin": "igp", "as-path": [65001]},
    "nlri": {
      "ipv4/unicast": [
        {"action": "add", "next-hop": "10.0.0.1", "nlri": ["192.168.1.0/24"]}
      ]
    }
  }
}
```

**Documentation shows (incorrect):**

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "update": {
      "message": {"id": 123, "direction": "received"},
      ...
    }
  }
}
```

**Behavior to preserve:**
- All JSON output from code remains unchanged (code is correct)

**Behavior to change:**
- Documentation examples need to match the code output

## Key Structural Differences

| Aspect | Docs (incorrect) | Code (correct) |
|--------|------------------|----------------|
| Event type location | `bgp.type` | `bgp.message.type` |
| Message metadata | `bgp.update.message` (nested) | `bgp.message` (top of bgp) |
| Event data container | `bgp.update` | Flat under `bgp` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | Documentation-only changes - no code modified | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Documentation-only changes - no tests needed | N/A |

## Files to Modify

- `docs/architecture/api/ipc_protocol.md` - fix JSON examples, remove version
- `docs/architecture/api/json-format.md` - fix structure docs, remove version
- `docs/architecture/api/architecture.md` - fix UPDATE format examples
- `docs/architecture/api/capability-contract.md` - fix refresh/borr/eorr examples
- `docs/architecture/api/commands.md` - fix response format examples

## Files to Create

None - documentation fixes only.

## Implementation Steps

1. **Read each doc file** - Understand current content
2. **Remove version references** - No `Version: 2.0` or version history sections
3. **Fix JSON examples** - Update to match code output format
4. **Verify consistency** - All examples follow same pattern

### Specific Changes Per File

**1. docs/architecture/api/ipc_protocol.md**
- Remove: Line 3-4 `Version: 2.0`
- Remove: Lines 724-730 Version History section
- Fix: All BGP event JSON examples

**2. docs/architecture/api/json-format.md**
- Remove: Version line (already done)
- Fix: Message structure section (lines 17-29)

**3. docs/architecture/api/architecture.md**
- Fix: UPDATE format examples (lines 575-631)
- Fix: API Output with Message ID (lines 860-879)

**4. docs/architecture/api/capability-contract.md**
- Fix: refresh/borr/eorr JSON examples (lines 175-179)

**5. docs/architecture/api/commands.md**
- Fix: response format examples (lines 490-499)

### State Events Format

State events use the same structure but without id/direction since they're not wire messages:

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "state"},
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "state": "up"
  }
}
```

## Implementation Summary

### What Was Implemented

1. **Removed version references:**
   - `ipc_protocol.md`: Removed "Version: 2.0" line and Version History section
   - `json-format.md`: Removed version line

2. **Fixed JSON examples in `architecture.md`:**
   - Updated "Wire Bytes in Events" section with IPC Protocol wrapped format
   - Fixed "JSON Format (Command Style)" announcements/withdrawals/mixed examples
   - Fixed "API Output with Message ID and Direction" example
   - Fixed state event JSON format (now wrapped with `{"type":"bgp","bgp":{...}}`)

3. **Fixed `capability-contract.md`:**
   - Updated refresh/borr/eorr JSON examples to IPC Protocol format with proper peer object

4. **Fixed `commands.md`:**
   - Updated response format examples to use proper `{"type":"response","response":{...}}` wrapper

### Deviations from Plan

- None. All planned changes were implemented as specified.

## Checklist

### 🏗️ Design
- [x] No premature abstraction (N/A - docs only)
- [x] No speculative features (N/A - docs only)
- [x] Single responsibility (docs match code)
- [x] Explicit behavior (examples show exact format)

### 🧪 TDD
- [x] Tests written (N/A - documentation-only changes)
- [x] Tests FAIL (N/A - documentation-only changes)
- [x] Tests PASS (N/A - documentation-only changes)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [ ] `make functional` passes (2 pre-existing flaky failures unrelated to docs)

### Documentation
- [x] All doc files updated
- [x] Version references removed
- [x] JSON examples consistent with code

### Completion
- [ ] All files committed together (awaiting user commit request)
