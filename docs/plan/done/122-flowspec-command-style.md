# Spec: flowspec-command-style

## Task

Update FlowSpec NLRI String() output to use command-style syntax matching the API input format.

**Current format:**
```
flowspec(dest-prefix=10.0.0.0/24 dest-port[=80 =443] protocol[=6])
```

**Target format:**
```
flowspec destination 10.0.0.0/24 destination-port =80 =443 protocol =6
```

This matches the API command syntax (see `docs/architecture/api/architecture.md` FlowSpec section), enabling round-trip parsing.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command syntax for FlowSpec
- [ ] `docs/architecture/wire/nlri-flowspec.md` - FlowSpec wire format and components

### RFC Summaries
- [ ] `docs/rfc/rfc8955.md` - FlowSpec component types, operator encoding

**Key insights:**
- API uses `destination <prefix>` not `dest-prefix set <prefix>` (FlowSpec is different from EVPN)
- Operators are inline with values: `=80`, `>=1024`, `<=65535`
- Multiple values space-separated: `destination-port =80 =443`
- Bitmask components use flags: `tcp-flags =syn&ack`, `fragment !is-fragment`
- VPN variant includes RD: `flowspec-vpn rd 65000:100 destination 10.0.0.0/24`

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFlowSpecStringCommandStyle` | `internal/bgp/nlri/flowspec_test.go` | FlowSpec.String() command format | |
| `TestFlowSpecVPNStringCommandStyle` | `internal/bgp/nlri/flowspec_test.go` | FlowSpecVPN.String() with RD | |
| `TestPrefixComponentString` | `internal/bgp/nlri/flowspec_test.go` | prefixComponent.String() format | |
| `TestNumericComponentString` | `internal/bgp/nlri/flowspec_test.go` | numericComponent.String() format | |
| `TestNumericOperatorString` | `internal/bgp/nlri/flowspec_test.go` | Operator symbols (=, >, <, etc.) | |
| `TestBitmaskComponentString` | `internal/bgp/nlri/flowspec_test.go` | TCP flags, fragment string format | |
| `TestFlowSpecStringRoundTrip` | `internal/bgp/nlri/flowspec_test.go` | String() matches API parse input | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

*Note: This change is String() output formatting only, no new validation.*

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Changes are in String() methods, tested via unit tests | |

## Files to Modify
- `internal/bgp/nlri/flowspec.go` - Update String() methods for FlowSpec, FlowSpecVPN, prefixComponent, numericComponent

## Files to Create
- None

## Implementation Steps
1. **Write unit tests** - Add tests for command-style String() output
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Update FlowSpec String() methods
4. **Run tests** - Verify PASS (paste output)
5. **Verify all** - `make lint && make test && make functional` (paste output)

## Format Specification

### Component Type Mapping

| Component | Current | Target |
|-----------|---------|--------|
| FlowDestPrefix | `dest-prefix=10.0.0.0/24` | `destination 10.0.0.0/24` |
| FlowSourcePrefix | `source-prefix=192.168.0.0/16` | `source 192.168.0.0/16` |
| FlowIPProtocol | `protocol[=6]` | `protocol =6` |
| FlowPort | `port[=80 =443]` | `port =80 =443` |
| FlowDestPort | `dest-port[=80]` | `destination-port =80` |
| FlowSourcePort | `source-port[=1024]` | `source-port =1024` |
| FlowICMPType | `icmp-type[=8]` | `icmp-type =8` |
| FlowICMPCode | `icmp-code[=0]` | `icmp-code =0` |
| FlowTCPFlags | `tcp-flags[2]` | `tcp-flags =syn` |
| FlowPacketLength | `packet-length[=1500]` | `packet-length =1500` |
| FlowDSCP | `dscp[=46]` | `dscp =46` |
| FlowFragment | `fragment[1]` | `fragment dont-fragment` |
| FlowFlowLabel | `flow-label[=0x12345]` | `flow-label =0x12345` |

### Operator Format

| Wire Op | Output | Example |
|---------|--------|---------|
| EQ (0x01) | `=` | `=80` |
| GT (0x02) | `>` | `>1024` |
| LT (0x04) | `<` | `<65535` |
| GE (0x03) | `>=` | `>=1024` |
| LE (0x05) | `<=` | `<=65535` |
| NE (0x06) | `!=` | `!=0` |

**Note on AND:** For numeric components, the parser infers AND from position (second+ values are ANDed).
No `&` prefix is output. For bitmask components (tcp-flags, fragment), `&` IS output as the parser expects it.

### Bitmask Format (TCP Flags)

| Flag Value | Output |
|------------|--------|
| 0x01 (FIN) | `fin` |
| 0x02 (SYN) | `syn` |
| 0x04 (RST) | `rst` |
| 0x08 (PSH) | `psh` |
| 0x10 (ACK) | `ack` |
| 0x20 (URG) | `urg` |
| 0x40 (ECE) | `ece` |
| 0x80 (CWR) | `cwr` |

Combined flags: `syn&ack` (value 0x12)

### Bitmask Format (Fragment)

| Flag Value | Output |
|------------|--------|
| 0x01 (DF) | `dont-fragment` |
| 0x02 (IsF) | `is-fragment` |
| 0x04 (FF) | `first-fragment` |
| 0x08 (LF) | `last-fragment` |

### FlowSpec NLRI String

**Current:**
```go
func (f *FlowSpec) String() string {
    parts := make([]string, len(f.components))
    for i, c := range f.components {
        parts[i] = c.String()
    }
    return fmt.Sprintf("flowspec(%s)", strings.Join(parts, " "))
}
```

**Target:**
```go
func (f *FlowSpec) String() string {
    parts := make([]string, 0, len(f.components))
    for _, c := range f.components {
        parts = append(parts, c.String())
    }
    return "flowspec " + strings.Join(parts, " ")
}
```

### FlowSpecVPN String

**Current:**
```go
func (f *FlowSpecVPN) String() string {
    return fmt.Sprintf("flowspec-vpn(rd:%s %s)", f.rd, f.flowSpec)
}
```

**Target:**
```go
func (f *FlowSpecVPN) String() string {
    return fmt.Sprintf("flowspec-vpn rd %s %s", f.rd, f.flowSpec.ComponentString())
}
```

## Implementation Summary

### What Was Implemented
- Updated `FlowComponentType.String()` to use command-style names:
  - `dest-prefix` → `destination`
  - `source-prefix` → `source`
  - `dest-port` → `destination-port`
- Updated `FlowSpec.String()`: removed parentheses, format now `flowspec <components>`
- Added `FlowSpec.ComponentString()` method for FlowSpecVPN embedding
- Updated `FlowSpecVPN.String()`: format now `flowspec-vpn rd <rd> <components>`
- Updated `prefixComponent.String()`: format now `<type> <prefix>` with space
- Updated `numericComponent.String()`: removed brackets, format now `<type> <op><value>...`
- Added `bitmaskString()` method for TCP flags and Fragment components
- Added `tcpFlagsToString()` helper for named TCP flag output
- Added `fragmentFlagsToString()` helper for named fragment flag output
- Updated `TestJSONEncoderFlowSpec` test expectations to match new format
- Added comprehensive TDD tests: `TestFlowSpecStringCommandStyle`, `TestFlowSpecVPNStringCommandStyle`, `TestPrefixComponentString`, `TestNumericComponentString`, `TestNumericOperatorString`, `TestBitmaskComponentString`, `TestFlowSpecStringRoundTrip`

### Design Insights
- Bitmask types (TCP flags, Fragment) require special formatting with named flags
- The `ComponentString()` method avoids duplicating "flowspec" prefix in VPN output
- Match operators need careful handling: bitmask operators (NOT, Match) vs numeric operators (LT, GT, EQ)
- **CRITICAL:** Numeric components must NOT output `&` prefix - the parser infers AND from position. Only bitmask components use `&` prefix.

### Bugs Found/Fixed
- **AND prefix bug (CRITICAL):** Initial implementation output `&<=65535` for numeric AND, but parser doesn't handle `&` prefix for numeric operators. This caused silent value dropping when parsing back. Fixed by removing `&` prefix from numeric component output.
- **Protocol operator bug (CRITICAL):** Output `protocol =6` but parser `parseFlowSpecProtocol` uses custom logic that doesn't handle operator prefix - it accepts `6` or `tcp` but NOT `=6`. Fixed by outputting plain numeric for protocol component without `=` prefix.

### Deviations from Plan
- Spec originally showed `&<=65535` format but this doesn't match parser. Corrected to `<=65535` (no `&` prefix).

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified)
- [x] Implementation complete
- [x] Tests PASS (verified)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (18 passed, 0 failed)

### Documentation
- [x] Required docs read
- [x] RFC references added to code

### Completion
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/122-flowspec-command-style.md`
