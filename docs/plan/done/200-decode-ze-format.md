# Spec: decode-ze-format

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/json-format.md` - Ze JSON format specification
4. `cmd/ze/bgp/decode.go` - current implementation (ExaBGP format)
5. `internal/exabgp/bridge.go` - Ze ↔ ExaBGP translation

## Task

Convert `ze bgp decode` output from ExaBGP-compatible JSON format to Ze native JSON format (IPC Protocol 2.0). Update ExaBGP migration tools as needed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/json-format.md` - Ze JSON format specification (canonical reference)
- [ ] `docs/architecture/api/architecture.md` - API design overview

### RFC Summaries
Not applicable - this is format conversion, not protocol work.

**Key insights:**
- Ze format uses `{"type": "bgp", "bgp": {...}}` envelope
- Peer info is flat: `"peer": {"address": "...", "asn": N}`
- NLRI uses operation arrays: `[{"action": "add", "next-hop": "...", "nlri": [...]}]`
- AS_PATH is a simple array: `[65001, 65002]`
- ExaBGP bridge (`internal/exabgp/`) already converts Ze → ExaBGP

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `cmd/ze/bgp/decode.go` - Outputs ExaBGP-compatible JSON with:
  - Envelope: `{"exabgp": "5.0.0", "time": ..., "host": ..., "pid": ..., "type": ...}`
  - Peer: `"neighbor": {"address": {"local": ..., "peer": ...}, "asn": {"local": ..., "peer": ...}}`
  - Routes: `"announce"` and `"withdraw"` keys
  - AS_PATH: `{"0": {"element": "as-sequence", "value": [asn1, asn2]}}`
  - Attributes: `"attribute"` key

- [x] `cmd/ze/bgp/decode_test.go` - Tests validate ExaBGP format (checks for `"exabgp"`, `"neighbor"`, etc.)

- [x] `test/decode/*.ci` - 22 functional tests expecting ExaBGP format

- [x] `internal/exabgp/bridge.go` - Already has `ZebgpToExabgpJSON()` which converts Ze format → ExaBGP format

**Behavior to preserve:**
- Human-readable output format (non-JSON mode) - unchanged
- NLRI-only mode (`--nlri`) - produces flat JSON without envelope (already correct)
- Plugin decode invocation infrastructure - unchanged
- All existing decode capabilities (OPEN, UPDATE, BGP-LS, FlowSpec, EVPN, VPN)

**Behavior to change:**
- JSON output format: ExaBGP → Ze format
- Test expectations: All decode tests must expect Ze format
- Functional tests: All `.ci` files must expect Ze format

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeOpenZeFormat` | `cmd/ze/bgp/decode_test.go` | OPEN produces Ze format | |
| `TestDecodeUpdateZeFormat` | `cmd/ze/bgp/decode_test.go` | UPDATE produces Ze format | |
| `TestDecodeASPathFormat` | `cmd/ze/bgp/decode_test.go` | AS_PATH is simple array | |

### Boundary Tests
Not applicable - no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All decode tests | `test/decode/*.ci` | Decode produces Ze format JSON | |

### Future
- None deferred

## Files to Modify
- `cmd/ze/bgp/decode.go` - Change JSON output format from ExaBGP to Ze
- `cmd/ze/bgp/decode_test.go` - Update test expectations

## Files to Create
- None

## Implementation Steps

### Phase 1: Update decode.go

1. **Replace envelope structure**
   - Remove `makeEnvelope()` ExaBGP fields (`exabgp`, `time`, `host`, `pid`, `ppid`, `counter`)
   - Add Ze envelope: `{"type": "bgp", "bgp": {"type": "<msg-type>", ...}}`

2. **Change peer structure**
   - From: `"neighbor": {"address": {"local": ..., "peer": ...}, "asn": {"local": ..., "peer": ...}}`
   - To: `"peer": {"address": "<peer-ip>", "asn": <peer-asn>}`
   - Note: decode doesn't have real peer info, use placeholder

3. **Change NLRI structure**
   - From: `"announce": {"family": {"nexthop": [{"nlri": "..."}]}}` and `"withdraw"`
   - To: `"nlri": {"family": [{"action": "add", "next-hop": "...", "nlri": [...]}, {"action": "del", "nlri": [...]}]}`

4. **Change AS_PATH format**
   - From: `{"0": {"element": "as-sequence", "value": [asn1, asn2]}}`
   - To: `[asn1, asn2]` (simple array)

5. **Change attribute key**
   - From: `"attribute"`
   - To: `"attr"`

6. **Add message info**
   - Add `"message": {"id": 0, "direction": "received"}` for decoded messages

### Phase 2: Update decode_test.go

1. Update `TestDecodeOpen` to expect Ze format
2. Update `TestDecodeUpdate` to expect Ze format
3. Update `TestDecodeOpenFQDNWithoutPlugin` to expect Ze format
4. Update `TestDecodeOpenFQDNWithPlugin` to expect Ze format
5. Update all BGP-LS tests to expect Ze format
6. Update FlowSpec tests to expect Ze format

### Phase 3: Update functional tests

1. Update all `test/decode/*.ci` files to expect Ze format JSON

### Phase 4: Verify ExaBGP bridge

1. Verify `internal/exabgp/bridge.go` still works correctly
2. The bridge converts Ze → ExaBGP, so it should continue to work
3. Add test if needed to verify round-trip: decode → Ze format → bridge → ExaBGP format

## Ze Format Reference

### OPEN Message
```json
{
  "type": "bgp",
  "bgp": {
    "type": "open",
    "peer": {"address": "127.0.0.1", "asn": 65533},
    "open": {
      "message": {"id": 0, "direction": "received"},
      "asn": 65533,
      "router-id": "10.0.0.2",
      "hold-time": 180,
      "capabilities": [
        {"code": 1, "name": "multiprotocol", "value": "ipv4/unicast"}
      ]
    }
  }
}
```

### UPDATE Message
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "127.0.0.1", "asn": 65533},
    "update": {
      "message": {"id": 0, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001, 65002],
        "med": 100,
        "local-preference": 100
      },
      "nlri": {
        "ipv4/unicast": [
          {"next-hop": "192.0.2.1", "action": "add", "nlri": ["10.0.0.0/24"]},
          {"action": "del", "nlri": ["172.16.0.0/16"]}
        ]
      }
    }
  }
}
```

## Design Decisions

1. **Placeholder peer info**: decode command doesn't have real peer context, use `127.0.0.1` and `65533` as placeholders (matches current behavior)

2. **Message ID**: Use `0` for decoded messages (no real message ID available)

3. **Direction**: Use `"received"` as default (decode is typically for analyzing received messages)

4. **Capabilities array vs map**: Ze format uses array of capability objects (not map keyed by code)

5. **NLRI-only mode unchanged**: `--nlri` mode produces flat NLRI JSON without any envelope (already correct for Ze)

## Design Principles Check (see `rules/design-principles.md`)
- [x] No premature abstraction - direct format conversion
- [x] No speculative features - implementing documented format
- [x] Single responsibility - decode produces Ze format
- [x] Explicit > implicit - format is documented
- [x] Minimal coupling - decode is standalone

## Implementation Progress (Session State)

**Last updated:** 2026-01-31

### Completed
1. ✅ Unit tests updated in `cmd/ze/bgp/decode_test.go` - expect Ze format
2. ✅ `decode.go` implementation complete:
   - `makeZeEnvelope()` - creates Ze IPC 2.0 envelope
   - `decodeOpenMessage()` - returns Ze format with capabilities array
   - `decodeUpdateMessage()` - returns Ze format with `attr` key, simple AS_PATH array
   - `capabilityToZeJSON()` - converts caps to `{code, name, value}` format
   - `parsePathAttributesZe()` - uses simple AS_PATH array
   - `buildMPReachZe()` / `buildMPUnreachZe()` - Ze NLRI operations format
   - Human formatters updated for Ze structure
3. ✅ All unit tests pass: `go test ./cmd/ze/bgp/... -run "TestDecode|TestBGPLS|TestFlowSpec"`

### In Progress
~~4. 🔄 Functional tests (`test/decode/*.ci`) - need updating to Ze format~~
   - ~~Updated: `bgp-open-sofware-version.ci`, `ipv4-unicast-1.ci`, `ipv4-unicast-2.ci`~~
   - ~~Remaining: ~19 more tests need expected JSON updated~~

**SUPERSEDED:** All 22 functional tests updated and passing (verified 2026-02-01)

### Key Decisions
- `message` field IS included in decode output (id:0, direction:received) - per user feedback
- Capabilities are array format: `[{code, name, value}, ...]`
- AS_PATH is simple array: `[asn1, asn2, ...]`
- Attributes key is `attr` (not `attribute`)
- NLRI uses operations: `[{action, next-hop, nlri}, ...]`

### Files Modified
- `cmd/ze/bgp/decode.go` - main implementation
- `cmd/ze/bgp/decode_test.go` - unit tests
- `test/decode/*.ci` - functional tests (partial)

## Implementation Summary

### What Was Implemented
- Converted `ze bgp decode` JSON output from ExaBGP format to Ze IPC Protocol 2.0 format
- Key format changes:
  - Envelope: `{"type": "bgp", "bgp": {...}}` instead of `{"exabgp": "5.0.0", ...}`
  - Peer info: `"peer": {"address": "...", "asn": N}` (flat structure)
  - AS_PATH: Simple array `[asn1, asn2]` instead of segment objects
  - Attributes: `"attr"` key instead of `"attribute"`
  - NLRI: Operation arrays `[{"action": "add", "next-hop": "...", "nlri": [...]}]`
  - Capabilities: Array of `{code, name, value}` objects
- Updated all 22 functional tests in `test/decode/*.ci` to expect Ze format
- Human-readable output formatters updated to work with new structure

### Bugs Found/Fixed
- None

### Design Insights
- NLRI-only mode (`--nlri`) correctly produces flat JSON without envelope - no changes needed
- Plugin decode infrastructure (FlowSpec, EVPN, BGP-LS, VPN) works unchanged
- ExaBGP bridge (`internal/exabgp/`) converts Ze → ExaBGP and remains functional

### Deviations from Plan
- None - implementation followed plan exactly

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
- [x] Tests PASS (unit tests)
- [x] Feature code integrated
- [x] Functional tests updated (22/22 done - verified 2026-02-01)

### Verification
- [x] `make lint` passes (0 issues - verified 2026-02-01)
- [x] `make test` passes (unit tests)
- [x] `make functional` passes (22 decode tests pass - verified 2026-02-01)

### Documentation
- [x] Required docs read
- [x] Ze format documented in json-format.md
- [x] Code comments updated

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [x] All files committed together (already committed in prior session)
