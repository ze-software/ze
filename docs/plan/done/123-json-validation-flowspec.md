# Spec: json-validation

## Task
Add JSON validation to test framework - make `json:` lines in `.ci` files validate against actual decoded output.

## Required Reading

### Architecture Docs
- [ ] `internal/test/runner/record.go` - MessageExpect struct stores JSON, not validated
- [ ] `internal/test/runner/runner.go` - Test execution, ReceivedRaw capture
- [ ] `internal/test/runner/decode.go` - DecodedMessage, IPv4-only, no JSON output
- [ ] `internal/test/runner/decoding.go` - compareJSON() reusable for normalization
- [ ] `cmd/zebgp/decode.go` - Comprehensive decoder, all families, ExaBGP envelope format

**Key insights:**
- `json:` lines parsed to `MessageExpect.JSON` but never validated
- `internal/test/runner/decode.go` only handles IPv4 unicast - insufficient
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

**Supported families:**
- `ipv4/unicast`, `ipv6/unicast` - standard prefix NLRIs
- `ipv4/flowspec`, `ipv6/flowspec` - FlowSpec rule components

**Deferred:** Complex families requiring special handling
- EVPN (`l2vpn/evpn`) - complex route types
- VPN (`ipv4/mpls-vpn`, `ipv6/mpls-vpn`) - RD encoding
- FlowSpec-VPN (`ipv4/flowspec-vpn`) - RD + FlowSpec
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
| `TestTransformEnvelopeToPlugin_IPv4Announce` | `internal/test/runner/json_test.go` | IPv4 announce transformation | ✅ |
| `TestTransformEnvelopeToPlugin_IPv4Withdraw` | `internal/test/runner/json_test.go` | IPv4 withdraw → action:del | ✅ |
| `TestTransformEnvelopeToPlugin_IPv6Announce` | `internal/test/runner/json_test.go` | IPv6 unicast transformation | ✅ |
| `TestTransformEnvelopeToPlugin_IPv6Withdraw` | `internal/test/runner/json_test.go` | IPv6 withdraw → action:del | ✅ |
| `TestTransformEnvelopeToPlugin_EOR` | `internal/test/runner/json_test.go` | End-of-RIB (empty update) | ✅ |
| `TestTransformEnvelopeToPlugin_FlowSpecAnnounce` | `internal/test/runner/json_test.go` | FlowSpec with no-nexthop | ✅ |
| `TestTransformEnvelopeToPlugin_FlowSpecWithNextHop` | `internal/test/runner/json_test.go` | FlowSpec with redirect next-hop | ✅ |
| `TestTransformEnvelopeToPlugin_IPv6FlowSpec` | `internal/test/runner/json_test.go` | IPv6 FlowSpec components | ✅ |
| `TestTransformEnvelopeToPlugin_FlowSpecWithdraw` | `internal/test/runner/json_test.go` | FlowSpec withdraw → action:del | ✅ |
| `TestComparePluginJSON_Match` | `internal/test/runner/json_test.go` | Matching JSON passes | ✅ |
| `TestComparePluginJSON_Mismatch` | `internal/test/runner/json_test.go` | Mismatch fails with diff | ✅ |
| `TestComparePluginJSON_IgnoresContextFields` | `internal/test/runner/json_test.go` | peer/direction ignored | ✅ |
| `TestComparePluginJSON_OrderIndependent` | `internal/test/runner/json_test.go` | JSON key order independent | ✅ |
| `TestIsSupportedFamily` | `internal/test/runner/json_test.go` | Family detection (unicast+flowspec) | ✅ |
| `TestExtractFamily` | `internal/test/runner/json_test.go` | Family extraction from envelope | ✅ |

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
- EVPN JSON validation - requires route type specific handling
- VPN JSON validation - requires RD handling
- FlowSpec-VPN JSON validation - requires RD + FlowSpec handling
- BGP-LS JSON validation - requires link-state TLV handling

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
{"meta":{"version":"1.0.0","app":"zebgp"},"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"...","action":"add","nlri":["..."]}]}
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
- `internal/test/runner/runner.go` - Add `validateJSON()` call, invoke `zebgp decode`
- `internal/test/runner/record.go` - Add `JSONError` field to Record, `JSONValidated` bool

## Files to Create
- `internal/test/runner/json.go` - `transformEnvelopeToPlugin()`, `comparePluginJSON()`, `isSupportedFamily()`
- `internal/test/runner/json_test.go` - Unit tests

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
- `internal/test/runner/json.go`: JSON validation utilities
  - `isSupportedFamily()`: Checks if family is IPv4/IPv6 unicast or FlowSpec
  - `isFlowSpecFamily()`: Detects FlowSpec families for special handling
  - `extractFamily()`: Extracts address family from zebgp decode envelope
  - `transformEnvelopeToPlugin()`: Converts zebgp decode format to plugin format (routes to appropriate transformer)
  - `transformAnnounce()`: Transforms unicast announce section (NLRI with next-hop)
  - `transformFlowspecAnnounce()`: Transforms FlowSpec announce section (preserves rule components in `nlri` object)
  - `transformFlowspecWithdraw()`: Transforms FlowSpec withdraw section (component objects with action:del)
  - `transformWithdraw()`: Transforms unicast withdraw section (handles both `[{"nlri":"..."}]` and `["..."]` formats)
  - `comparePluginJSON()`: Compares transformed JSON with expected, ignoring peer/direction fields
  - `normalizeForComparison()`, `normalizeValue()`, `normalizeSlice()`, `sortSliceOfMaps()`: Normalize JSON for deep comparison
- `internal/test/runner/json_test.go`: Unit tests for all above functions (15 tests total)
- `internal/test/runner/runner.go`: Integration
  - `validateJSON()`: Called after raw byte validation passes, validates JSON expectations
  - `decodeToEnvelope()`: Executes `zebgp decode --update` and parses JSON output
  - `extractNLRIs()`: Updated to handle FlowSpec families (uses "string" field as identifier)
  - `extractAction()`: Updated to handle FlowSpec families
  - `extractNLRIFromEntry()`: Updated to handle FlowSpec nlri map format
  - Skips validation for tests using ExaBGP envelope format (contains "exabgp" key)

### FlowSpec Support
FlowSpec NLRI contains rule components (destination-ipv4, tcp-flags, protocol, etc.) rather than simple prefixes.

**Plugin format for FlowSpec:**
```json
{
  "ipv4/flowspec": [{
    "action": "add",
    "nlri": {
      "next-hop": "1.2.3.4",  // optional, omitted for "no-nexthop"
      "destination-ipv4": ["192.168.0.1/32"],
      "tcp-flags": ["=syn"],
      "string": "flow destination-ipv4 192.168.0.1/32 tcp-flags [ =syn ]"
    }
  }]
}
```

### Bugs Found/Fixed
- Withdraw format mismatch: zebgp decode outputs `[{"nlri":"..."}]` not `["..."]` for withdraws. Fixed `transformWithdraw()` to handle both formats.

### Design Insights
- Two JSON formats exist in test files: older ExaBGP envelope format (in `test/data/encode/`) and newer plugin format (in `test/data/plugin/`). Detection via "exabgp" key presence allows both to coexist.
- Context fields (peer, direction) are test-environment-dependent and correctly excluded from comparison.
- FlowSpec "no-nexthop" means the rule doesn't redirect to a next-hop (e.g., rate-limit, discard actions). In plugin format, the `next-hop` field is omitted from the `nlri` object.

### Deviations from Plan
- Added ExaBGP envelope format detection to skip validation for older test files with different JSON format.
- Simplified function signatures: `transformEnvelopeToPlugin` returns `(map, string)` instead of `(map, string, error)` since errors never occur.

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
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/NNN-<name>.md`
- [x] All files committed together
