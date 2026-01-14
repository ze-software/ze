# Spec: json-validation

## Task
Add JSON validation to test framework - make `json:` lines in `.ci` files validate against actual decoded output.

## Required Reading

### Architecture Docs
- [ ] `test/functional/record.go` - MessageExpect struct stores JSON, not validated
- [ ] `test/functional/runner.go` - Test execution, ReceivedRaw capture
- [ ] `test/functional/decode.go` - DecodedMessage, IPv4-only, no JSON output
- [ ] `test/functional/decoding.go` - compareJSON() reusable for normalization
- [ ] `cmd/zebgp/decode.go` - Comprehensive decoder, all families, ExaBGP envelope format

**Key insights:**
- `json:` lines parsed to `MessageExpect.JSON` but never validated
- `test/functional/decode.go` only handles IPv4 unicast - insufficient
- `cmd/zebgp/decode.go` handles all families but outputs ExaBGP envelope format
- Plugin JSON format differs from ExaBGP envelope - need transformation
- Peer context (address, ASN) comes from config, not message bytes

## Design Analysis

### Format Comparison

**ExaBGP envelope (from `zebgp decode`):**
```json
{
  "exabgp": "5.0.0",
  "time": 123.456,
  "host": "hostname",
  "pid": 123,
  "type": "update",
  "neighbor": {
    "address": {"local": "127.0.0.1", "peer": "127.0.0.1"},
    "asn": {"local": 65533, "peer": 65533},
    "direction": "in",
    "message": {
      "update": {
        "attribute": {"origin": "igp", "local-preference": 200},
        "announce": {"ipv4 unicast": {"10.0.1.254": [{"nlri": "10.0.1.0/24"}]}}
      }
    }
  }
}
```

**Plugin format (from `.ci` json: lines):**
```json
{
  "meta": {"version": "1.0.0", "format": "zebgp"},
  "message": {"type": "update"},
  "direction": "in",
  "peer": {"address": "127.0.0.1", "asn": 65000},
  "origin": "igp",
  "local-preference": 200,
  "ipv4/unicast": [{"next-hop": "10.0.1.254", "action": "add", "nlri": ["10.0.1.0/24"]}]
}
```

### Transformation Required

| ExaBGP path | Plugin path |
|-------------|-------------|
| (constant) | `meta.version` = "1.0.0" |
| (constant) | `meta.app` = "zebgp" |
| `type` | `message.type` |
| `neighbor.direction` | `direction` |
| `neighbor.address.peer` | `peer.address` |
| `neighbor.asn.peer` | `peer.asn` |
| `neighbor.message.update.attribute.origin` | `origin` |
| `neighbor.message.update.attribute.local-preference` | `local-preference` |
| `neighbor.message.update.announce.<family>.<nexthop>` | `<family>[].next-hop, nlri, action:add` |
| `neighbor.message.update.withdraw.<family>` | `<family>[].nlri, action:del` |

### Scope Limitation

**Phase 1 (this spec):** IPv4/IPv6 unicast only
- `ipv4/unicast`, `ipv6/unicast` families
- ~15 of 33 files with `json:` lines

**Deferred:** Complex families requiring special handling
- EVPN (`l2vpn/evpn`) - complex route types
- FlowSpec (`ipv4/flow`, `ipv6/flow`) - rule components
- VPN (`ipv4/mpls-vpn`, `ipv6/mpls-vpn`) - RD encoding
- BGP-LS - link-state TLVs

### Context Requirements

| Field | Source |
|-------|--------|
| `peer.address` | Test config or ignore in comparison |
| `peer.asn` | Test config `option:asn:` or ignore |
| `direction` | Always "in" for received messages |

**Decision:** Ignore `peer.address`, `peer.asn`, `direction` in comparison - these are context-dependent and don't validate message content.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTransformEnvelopeToPlugin_IPv4Announce` | `test/functional/json_test.go` | IPv4 announce transformation | |
| `TestTransformEnvelopeToPlugin_IPv4Withdraw` | `test/functional/json_test.go` | IPv4 withdraw → action:del | |
| `TestTransformEnvelopeToPlugin_IPv6Announce` | `test/functional/json_test.go` | IPv6 unicast transformation | |
| `TestTransformEnvelopeToPlugin_EOR` | `test/functional/json_test.go` | End-of-RIB (empty update) | |
| `TestComparePluginJSON_Match` | `test/functional/json_test.go` | Matching JSON passes | |
| `TestComparePluginJSON_Mismatch` | `test/functional/json_test.go` | Mismatch fails with diff | |
| `TestComparePluginJSON_IgnoresContextFields` | `test/functional/json_test.go` | peer/direction ignored | |
| `TestIsSupportedFamily` | `test/functional/json_test.go` | Family detection | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - No numeric range validation in this feature.

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `ipv4.ci` | `test/data/plugin/` | IPv4 unicast announce/withdraw | |
| `ipv6.ci` | `test/data/plugin/` | IPv6 unicast announce/withdraw | |
| `new-v4.ci` | `test/data/encode/` | IPv4 basic encoding | |
| `new-v6.ci` | `test/data/encode/` | IPv6 basic encoding | |

### Future (deferred)
- EVPN JSON validation - requires `evpnToJSON` transformation
- FlowSpec JSON validation - requires `flowSpecToJSON` transformation
- VPN JSON validation - requires RD handling

## Follow-up: Convert ExaBGP JSON to ZeBGP Format

**REQUIRED:** All `json:` lines in `.ci` test files using ExaBGP envelope format must be converted to ZeBGP plugin format.

### Current State
- Tests in `test/data/plugin/ipv4.ci` and `ipv6.ci` use correct ZeBGP plugin format
- Tests in `test/data/encode/*.ci` use legacy ExaBGP envelope format (skipped by validation)

### Action Required
Convert all ExaBGP format JSON:
```json
{"exabgp":"6.0.0","type":"update","neighbor":{"message":{"update":{...}}}}
```

To ZeBGP plugin format:
```json
{"meta":{"version":"1.0.0","format":"zebgp"},"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"...","action":"add","nlri":["..."]}]}
```

### Meta Section
All ZeBGP plugin JSON must include:
```json
{
  "meta": {
    "version": "1.0.0",
    "format": "zebgp"
  },
  ...
}
```

### Files to Convert
| File | Family | Status |
|------|--------|--------|
| `test/data/encode/addpath.ci` | ipv4/unicast, ipv4/mpls-vpn | pending |
| `test/data/encode/attributes.ci` | ipv4/unicast | pending |
| `test/data/encode/ebgp.ci` | ipv4/unicast | pending |
| `test/data/encode/extended-*.ci` | various | pending |
| `test/data/encode/l2vpn.ci` | l2vpn/evpn | pending (Phase 2) |
| `test/data/encode/template.ci` | ipv4/unicast | pending |
| `test/data/encode/vpn.ci` | ipv4/mpls-vpn | pending (Phase 2) |
| `test/data/encode/watchdog.ci` | ipv4/unicast | pending |
| `test/data/plugin/nexthop.ci` | ipv6/unicast | pending |

### Why This Matters
- ExaBGP format detection (`"exabgp"` key) is a temporary workaround
- All JSON should use ZeBGP's canonical plugin format
- Enables full validation coverage across all test files

## Files to Modify
- `test/functional/runner.go` - Add `validateJSON()` call, invoke `zebgp decode`
- `test/functional/record.go` - Add `JSONError` field to Record, `JSONValidated` bool

## Files to Create
- `test/functional/json.go` - `transformEnvelopeToPlugin()`, `comparePluginJSON()`, `isSupportedFamily()`
- `test/functional/json_test.go` - Unit tests

## Implementation Steps

1. **Write unit tests** - Create `json_test.go` with transformation and comparison tests
2. **Run tests** - Verify FAIL (paste output)
3. **Implement `isSupportedFamily()`** - Returns true for ipv4/unicast, ipv6/unicast
4. **Implement `transformEnvelopeToPlugin()`** - ExaBGP envelope → plugin format
5. **Implement `comparePluginJSON()`** - Normalize and compare, ignore context fields
6. **Integrate into runner** - Call `zebgp decode`, transform, compare
7. **Run tests** - Verify PASS (paste output)
8. **Verify all** - `make lint && make test && make functional` (paste output)

## Detailed Design

### validateJSON() in runner.go

```go
func (r *Runner) validateJSON(rec *Record) error {
    for i, msg := range rec.Messages {
        if msg.JSON == "" {
            continue // No JSON expectation
        }
        if i >= len(rec.ReceivedRaw) {
            continue // No received message to compare
        }

        // Decode via zebgp decode command
        envelope, err := r.decodeToEnvelope(rec.ReceivedRaw[i])
        if err != nil {
            return fmt.Errorf("message %d: decode failed: %w", msg.Index, err)
        }

        // Check if family is supported
        family := extractFamily(envelope)
        if !isSupportedFamily(family) {
            continue // Skip unsupported families (Phase 1 limitation)
        }

        // Transform to plugin format
        actual, err := transformEnvelopeToPlugin(envelope)
        if err != nil {
            return fmt.Errorf("message %d: transform failed: %w", msg.Index, err)
        }

        // Compare
        if err := comparePluginJSON(actual, msg.JSON); err != nil {
            return fmt.Errorf("message %d: %w", msg.Index, err)
        }
    }
    return nil
}
```

### transformEnvelopeToPlugin()

```go
func transformEnvelopeToPlugin(envelope map[string]any) (map[string]any, error) {
    result := map[string]any{
        "message": map[string]any{"type": envelope["type"]},
    }

    neighbor := envelope["neighbor"].(map[string]any)
    msg := neighbor["message"].(map[string]any)
    update := msg["update"].(map[string]any)

    // Copy attributes to top level
    if attrs, ok := update["attribute"].(map[string]any); ok {
        for k, v := range attrs {
            result[k] = v
        }
    }

    // Transform announce → family arrays with action:add
    if announce, ok := update["announce"].(map[string]any); ok {
        for family, nhMap := range announce {
            // ... transform to plugin format
        }
    }

    // Transform withdraw → family arrays with action:del
    // ...

    return result, nil
}
```

### comparePluginJSON()

```go
func comparePluginJSON(actual, expected string) error {
    var actualMap, expectedMap map[string]any
    json.Unmarshal([]byte(actual), &actualMap)
    json.Unmarshal([]byte(expected), &expectedMap)

    // Remove context-dependent fields
    contextFields := []string{"direction", "peer"}
    for _, f := range contextFields {
        delete(actualMap, f)
        delete(expectedMap, f)
    }

    // Compare
    actualBytes, _ := json.Marshal(actualMap)
    expectedBytes, _ := json.Marshal(expectedMap)

    if string(actualBytes) != string(expectedBytes) {
        return fmt.Errorf("JSON mismatch:\n%s", diff(expectedBytes, actualBytes))
    }
    return nil
}
```

### Message Matching Strategy

Match by position after filtering keepalives:
1. `rec.ReceivedRaw` contains all received messages (excluding keepalives, already filtered by `extractReceivedMessages`)
2. `rec.Messages[i]` with `JSON` set corresponds to `ReceivedRaw[i]`
3. If indices don't align (multi-connection tests), match by `RawHex` equality

## Implementation Summary

### What Was Implemented
- `test/functional/json.go`: JSON validation utilities
  - `isSupportedFamily()`: Checks if family is IPv4/IPv6 unicast (Phase 1)
  - `extractFamily()`: Extracts address family from zebgp decode envelope
  - `transformEnvelopeToPlugin()`: Converts zebgp decode format to plugin format
  - `transformAnnounce()`: Transforms announce section (NLRI with next-hop)
  - `transformWithdraw()`: Transforms withdraw section (handles both `[{"nlri":"..."}]` and `["..."]` formats)
  - `comparePluginJSON()`: Compares transformed JSON with expected, ignoring peer/direction fields
  - `normalizeForComparison()`, `normalizeValue()`, `normalizeSlice()`, `sortSliceOfMaps()`: Normalize JSON for deep comparison
- `test/functional/json_test.go`: Unit tests for all above functions
- `test/functional/runner.go`: Integration
  - `validateJSON()`: Content-based matching (NLRI + action), not position-based
  - `extractNLRIs()`, `extractAction()`: Extract matching keys from plugin JSON
  - `decodeToEnvelope()`: Executes `zebgp decode --update` and parses JSON output
  - Skips validation for unsupported families (FlowSpec, VPN, EVPN, etc.)
- All `test/data/**/*.ci` files: Converted JSON from ExaBGP envelope to ZeBGP plugin format

### Unsupported Families (Phase 2)
JSON validation is **skipped** for these families - zebgp decode works but transform not implemented:

| Family | Reason |
|--------|--------|
| `ipv4/flowspec`, `ipv6/flowspec` | Different NLRI structure (rule components), no next-hop |
| `ipv4/mpls-vpn`, `ipv6/mpls-vpn` | RD encoding, label handling |
| `l2vpn/vpls`, `l2vpn/evpn` | Complex route types |
| `ipv4/mup`, `ipv6/mup` | SRv6 MUP specific fields |

Test files for these families have JSON converted to ZeBGP format but validation does not run.

### Bugs Found/Fixed
- Withdraw format mismatch: zebgp decode outputs `[{"nlri":"..."}]` not `["..."]` for withdraws. Fixed `transformWithdraw()` to handle both formats.
- Message ordering: ZeBGP sends routes in lexicographic order, not config order. Fixed by content-based matching instead of position-based.

### Design Insights
- Content-based matching (NLRI + action) handles ZeBGP's lexicographic route ordering
- Context fields (peer, direction) are test-environment-dependent and correctly excluded from comparison
- `used` flag prevents matching same received message to multiple expected messages

### Deviations from Plan
- Changed from position-based to content-based matching due to ZeBGP route ordering
- Simplified function signatures: `transformEnvelopeToPlugin` returns `(map, string)` instead of `(map, string, error)` since errors never occur

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)
- [x] Boundary tests cover all numeric inputs (N/A - no numeric inputs in this feature)

### Verification
- [x] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A - no RFCs referenced)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)

### Completion (after tests pass - see Completion Checklist)
- [x] Architecture docs updated with learnings (N/A - test infrastructure, not architecture)
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
