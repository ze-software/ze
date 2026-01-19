# Spec: flowspec-wire-format

**Status:** BACKFILL - Implementation completed Dec 2025

## Task
Implement FlowSpec NLRI wire format encoding/decoding per RFC 8955 (IPv4) and RFC 8956 (IPv6), including VPN variants (SAFI 133/134).

## Required Reading
- [x] `.claude/zebgp/wire/NLRI.md` - NLRI wire formats
- [x] `rfc/rfc8955.txt` - FlowSpec v2 specification
- [x] `rfc/rfc8956.txt` - IPv6 FlowSpec extensions

**Key insights:**
- FlowSpec NLRI = ordered list of match components (not prefix-based)
- Components MUST follow strict type ordering (RFC 8955 Section 4.2)
- Two operator formats: numeric (RFC 8955 4.2.1.1) and bitmask (4.2.1.2)
- Length encoding: 1-byte (<240) or 2-byte (0xF0 prefix for >=240)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestFlowComponentTypeConstants` | `internal/bgp/nlri/flowspec_test.go` | Type values match RFC 8955 Section 4.2.2 |
| `TestFlowDestPrefixComponent` | `internal/bgp/nlri/flowspec_test.go` | Type 1 dest prefix encoding |
| `TestFlowSourcePrefixComponent` | `internal/bgp/nlri/flowspec_test.go` | Type 2 source prefix encoding |
| `TestFlowNumericComponent` | `internal/bgp/nlri/flowspec_test.go` | Types 3-8,10-11 numeric operators |
| `TestFlowBitmaskComponent` | `internal/bgp/nlri/flowspec_test.go` | Types 9,12 bitmask operators |
| `TestFlowFlowLabelComponent` | `internal/bgp/nlri/flowspec_test.go` | Type 13 IPv6 flow label (RFC 8956) |
| `TestFlowSpecParse` | `internal/bgp/nlri/flowspec_test.go` | Full NLRI parsing |
| `TestFlowSpecWriteTo` | `internal/bgp/nlri/flowspec_test.go` | Zero-alloc encoding |
| `TestFlowSpecVPN` | `internal/bgp/nlri/flowspec_test.go` | SAFI 134 with RD |
| `TestFlowSpecRoundTrip` | `internal/bgp/nlri/flowspec_test.go` | Parse → WriteTo consistency |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `bgp-flow-1` | `test/data/decode/` | Basic FlowSpec decoding |
| `bgp-flow-2` | `test/data/decode/` | Multi-component FlowSpec |
| `bgp-flow-3` | `test/data/decode/` | FlowSpec with complex operators |
| `bgp-flow-4` | `test/data/decode/` | FlowSpec VPN variant |

## Files Created/Modified

| File | Lines | Purpose |
|------|-------|---------|
| `internal/bgp/nlri/flowspec.go` | ~1,200 | Core wire format types and encoding |
| `internal/bgp/nlri/flowspec_test.go` | ~670 | Comprehensive unit tests |

## Implementation Summary

### Types Implemented

```go
// Core types
type FlowSpec struct           // IPv4/IPv6 FlowSpec NLRI
type FlowSpecVPN struct         // VPN variant with RD (SAFI 134)
type FlowComponent interface    // Match component interface

// Component implementations (RFC 8955 Section 4.2.2)
type FlowDestPrefixComponent    // Type 1: Destination prefix
type FlowSourcePrefixComponent  // Type 2: Source prefix
type FlowNumericComponent       // Types 3-8, 10-11: Numeric values
type FlowBitmaskComponent       // Types 9, 12: Bitmask values
type FlowFlowLabelComponent     // Type 13: IPv6 flow label (RFC 8956)

// Operator types (RFC 8955 Section 4.2.1)
type FlowOperator byte          // Numeric operator (lt/gt/eq/and/end)
type FlowBitmaskOp byte         // Bitmask operator (match/not/and/end)
```

### Key Functions

```go
// Parsing
func ParseFlowSpec(family Family, data []byte) (*FlowSpec, int, error)
func ParseFlowSpecVPN(family Family, data []byte) (*FlowSpecVPN, int, error)

// Zero-allocation encoding
func (f *FlowSpec) WriteTo(buf []byte, off int, ctx *EncodingCtx) (int, error)
func (f *FlowSpecVPN) WriteTo(buf []byte, off int, ctx *EncodingCtx) (int, error)

// Component construction
func NewFlowDestPrefixComponent(prefix netip.Prefix) *FlowDestPrefixComponent
func NewFlowNumericComponent(typ FlowComponentType, ops []FlowOperator, values []uint64) *FlowNumericComponent
func NewFlowBitmaskComponent(typ FlowComponentType, ops []FlowBitmaskOp, values []uint8) *FlowBitmaskComponent
```

### Wire Format Details

**Length encoding (RFC 8955 Section 4):**
- Length < 240: 1 byte
- Length >= 240: 2 bytes (0xF0 prefix, remaining bits + next byte)

**Numeric operator (RFC 8955 Section 4.2.1.1):**
```
  0   1   2   3   4   5   6   7
+---+---+---+---+---+---+---+---+
| e | a |  len  | 0 |lt |gt |eq |
+---+---+---+---+---+---+---+---+
```

**Bitmask operator (RFC 8955 Section 4.2.1.2):**
```
  0   1   2   3   4   5   6   7
+---+---+---+---+---+---+---+---+
| e | a |  len  | 0 | 0 |not|mat|
+---+---+---+---+---+---+---+---+
```

### Component Ordering
Per RFC 8955 Section 4.2: Components MUST be sorted by type number in increasing order. The implementation enforces this via `slices.SortFunc()` before encoding.

## Key Commits

| Commit | Description |
|--------|-------------|
| `9f87a41` | Initial FlowSpec implementation (Phase 6 NLRI types) |
| `3b812bb` | Add FlowSpec VPN (SAFI 134), type 13, IPv6 offset |
| `8a3bdc4` | Improve numeric component parsing with match operators |
| `453f789` | Add RFC annotations (Phase 4) |
| `502905f` | Implement zero-alloc WriteTo |

## RFC References
- RFC 8955 Section 4 - NLRI Encoding
- RFC 8955 Section 4.2 - Component ordering
- RFC 8955 Section 4.2.1.1 - Numeric operator format
- RFC 8955 Section 4.2.1.2 - Bitmask operator format
- RFC 8955 Section 4.2.2 - Component types 1-12
- RFC 8956 Section 3 - IPv6 extensions (type 13)
- RFC 8955 Section 8 - VPN FlowSpec (SAFI 134)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (initial implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references added (comments throughout flowspec.go)
- [x] `.claude/zebgp/wire/NLRI.md` updated

### Completion
- [x] Spec moved to `docs/plan/done/091-flowspec-wire-format.md`
