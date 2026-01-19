# Spec: Decoding and Parsing Functional Tests

**Created:** 2025-12-31
**Status:** ✅ Complete (18/18 decoding pass, 10/10 parsing pass)

## Problem

ZeBGP's functional test tool only supports `encoding` and `api` commands. ExaBGP has two additional test types that ZeBGP should also support:

1. **Decoding tests** - Validate BGP message parsing produces correct JSON
2. **Parsing tests** - Validate config files parse without errors

Test data already exists:
- `test/data/decode/*.test` - 18 decoding test files
- `test/data/parse/valid/*.conf` - 10 positive parsing test configs
- `test/data/parse/invalid/*.conf` + `.expect` - negative parsing tests

## Current Status

| Component | Status | Notes |
|-----------|--------|-------|
| `zebgp decode` CLI | ✅ Complete | All NLRI types parsed |
| `functional decoding` | ✅ Complete | 18/18 tests pass |
| `functional parsing` | ✅ Complete | 10/10 tests pass |
| `make functional` | ✅ Exists | All types integrated |

### Decoding Test Results

| Test | Status | Notes |
|------|--------|-------|
| bgp-evpn-1 | ✅ | EVPN Type 2 with lenient label parsing |
| bgp-open-software-version | ✅ | Capabilities parsed |
| ipv4-unicast-1 | ✅ | IPv4 unicast |
| ipv4-unicast-2 | ✅ | as-path object format, aggregator |
| bgp-flow-1..4 | ✅ | FlowSpec structured output |
| bgp-ls-1..4, 10 | ✅ | BGP-LS Link/Node/Prefix NLRI |
| bgp-ls-5 | ✅ | SR-MPLS Adjacency SID (TLV 1099) |
| bgp-ls-6..9 | ✅ | SRv6 End.X SID, lossless array format |

## Requirements

### 1. Decoding Tests (`functional decoding`)

**Purpose:** Verify BGP message/NLRI decoding produces correct JSON output.

**Test file format** (3 lines):
```
<type> [<afi> <safi>]           # e.g., "update l2vpn/evpn" or "open"
<hex_packet>                     # UPDATE message body (no BGP header)
<expected_json>                  # Expected JSON output (ExaBGP format)
```

**IMPORTANT - Hex format clarification:**
- Test hex does NOT include BGP header (no FF*16 marker)
- Test hex is UPDATE message body starting with path attributes
- Decoder must detect: if FF*16 present → full message; if absent → assume UPDATE body

**Execution:**
```bash
zebgp decode --<type> [-f "<afi> <safi>"] <hex_packet>
```

**Success criteria:**
- Exit code 0
- JSON output matches expected (after removing volatile fields)
- **On parse failure:** Return valid JSON with `"parsed": false` (not error exit)

**Required changes:**
1. ✅ Add `zebgp decode` command to CLI
2. ⚠️ Integrate `internal/bgp/nlri/` parsers (EVPN, FlowSpec, BGP-LS)
3. ⚠️ Match ExaBGP JSON structure exactly
4. ✅ Add `decoding` subcommand to functional test runner

### 2. Parsing Tests (`functional parsing`)

**Purpose:** Verify config files parse without errors.

**Status:** ✅ Complete - 10/10 tests pass

**Execution:**
```bash
zebgp validate <config_file>
```

**Success criteria:**
- Exit code 0 (config parses successfully)
- No error output

## Implementation Plan

### Phase 1: Fix `zebgp decode` Command

**Current issues:**
1. Outputs raw hex instead of parsed NLRI
2. Duplicates parsing code instead of using `internal/bgp/nlri/`
3. JSON structure doesn't match ExaBGP format

**Required fixes:**

#### 1.1 Header Detection
```go
// If data starts with FF*16, it's a full BGP message
// Otherwise, assume UPDATE message body
func detectFormat(data []byte) string {
    if len(data) >= 16 && hasValidMarker(data) {
        return detectMessageType(data) // from header byte 18
    }
    return msgTypeUpdate // default to UPDATE body
}
```

#### 1.2 Use Existing NLRI Parsers

Existing parsers in `internal/bgp/nlri/`:
- `ParseEVPN()` - All 5 EVPN route types
- `ParseFlowSpec()` - FlowSpec rules
- `ParseBGPLS()` - BGP-LS NLRI

```go
import "codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"

// Parse MP_UNREACH_NLRI for l2vpn/evpn
routes, err := nlri.ParseEVPN(nlriData, false)
```

#### 1.3 ExaBGP JSON Format

Expected structure for EVPN routes:
```json
{
  "withdraw": {
    "l2vpn/evpn": [
      {
        "code": 2,
        "parsed": true,
        "raw": "02210001...",
        "name": "MAC/IP advertisement",
        "rd": "37.44.55.55:1",
        "esi": "-",
        "ethernet-tag": 1901,
        "mac": "FC:15:B4:78:7B:8F",
        "label": [[0]]
      }
    ]
  }
}
```

**Key differences from current output:**
- Routes in arrays, not single objects
- Each route has `code`, `parsed`, `raw`, `name` fields
- Parsed fields: `rd`, `esi`, `ethernet-tag`, `mac`, `ip`, `label`

#### 1.4 Failure JSON Format

When parsing fails, return valid JSON:
```json
{
  "code": 8,
  "parsed": false,
  "raw": "08260001252C37768EAD..."
}
```

### Phase 2: Decoding Test Runner

**Status:** ✅ Infrastructure complete

Files:
- `test/cmd/functional/main.go` - `decoding` command added
- `internal/test/runner/decoding.go` - Test discovery and execution

### Phase 3: Parsing Test Runner

**Status:** ✅ Complete

Files:
- `test/cmd/functional/main.go` - `parsing` command added
- `internal/test/runner/parsing.go` - Test discovery and execution

### Phase 4: Make Integration

Add to `Makefile`:
```makefile
.PHONY: functional
functional: build
	go run ./test/cmd/functional encoding --all
	go run ./test/cmd/functional api --all
	go run ./test/cmd/functional decoding --all
	go run ./test/cmd/functional parsing --all
```

## Test Cases

### Decoding Tests (18 existing)

| Test | Type | Family | NLRI Parser |
|------|------|--------|-------------|
| bgp-evpn-1 | update | l2vpn/evpn | `ParseEVPN` |
| bgp-flow-1..4 | update | ipv4/flowspec | `ParseFlowSpec` |
| bgp-ls-1..10 | update/nlri | bgp-ls | `ParseBGPLS` |
| bgp-open-software-version | open | - | Capabilities |
| ipv4-unicast-1..2 | update | ipv4/unicast | IPv4 prefix |

### Parsing Tests (10 existing)

| Test | Status |
|------|--------|
| community.conf | ✅ Pass |
| dual-neighbor.conf | ✅ Pass |
| healthcheck.conf | ✅ Pass |
| md5.conf | ✅ Pass |
| multiple-process.conf | ✅ Pass |
| multisession.conf | ✅ Pass |
| process.conf | ✅ Pass |
| simple-v4.conf | ✅ Pass |
| simple-v6.conf | ✅ Pass |
| ttl.conf | ✅ Pass |

## CLI Examples

```bash
# Run all decoding tests
go run ./test/cmd/functional decoding --all

# Run specific decoding test
go run ./test/cmd/functional decoding bgp-evpn-1

# Run all parsing tests
go run ./test/cmd/functional parsing --all

# List parsing tests
go run ./test/cmd/functional parsing --list

# Run all functional tests
make functional
```

## JSON Comparison

For decoding tests, compare JSON after removing volatile fields:
- `exabgp` (version)
- `time` (timestamp)
- `host` (hostname)
- `pid`, `ppid` (process IDs)
- `counter` (message counter)

## Success Metrics

- [x] All 18 decoding tests pass ✅
- [x] All 10 parsing tests pass ✅
- [x] `make functional` runs all test types ✅

## Completed Work (2025-12-31)

1. **SR-MPLS Adjacency SID** (bgp-ls-5) ✅
   - Implemented TLV 1099 parsing (RFC 9085)
   - Array format for multiple instances (lossless JSON)

2. **SRv6 End.X SID key name** (bgp-ls-6..9) ✅
   - Changed `srv6-endx-sid` → `srv6-endx`
   - Fixed `remote-router-ids` to use array (lossless)
   - Updated test files with correct expected output

3. **Lossless JSON Format** ✅
   - `sr-adj`: Array of entries (was: duplicate keys, data loss)
   - `local-router-ids`: Array of IPv4+IPv6 (already correct)
   - `remote-router-ids`: Array of IPv4+IPv6 (was: single value, data loss)

## Completed Work

1. **FlowSpec output format** (4 tests) ✅
   - Structured component output with operators and values

2. **BGP-LS structured output** (10 tests, 5 pass) ✅
   - Node, Link, Prefix NLRI types
   - Node descriptors, link descriptors
   - BGP-LS attribute (TLV 29) parsing
   - SRv6 TLVs (RFC 9514): 1106, 1107, 1108, sub-TLV 1252

3. **as-path object format** (ipv4-unicast-2) ✅
   - ExaBGP format: `{"0": {"element": "as-sequence", "value": [...]}}`
   - AGGREGATOR attribute parsing

## Files Modified

| File | Status | Purpose |
|------|--------|---------|
| `cmd/zebgp/main.go` | ✅ | Added `decode` command |
| `cmd/zebgp/decode.go` | ✅ | Decode logic, TLV 1099, lossless arrays |
| `cmd/zebgp/decode_test.go` | ✅ | Unit tests including TLV 1099 |
| `test/cmd/functional/main.go` | ✅ | Added decoding/parsing commands |
| `internal/test/runner/decoding.go` | ✅ | Decoding test infrastructure |
| `internal/test/runner/parsing.go` | ✅ | Parsing test infrastructure |
| `test/data/decode/bgp-ls-5.test` | ✅ | Updated for sr-adj array format |
| `test/data/decode/bgp-ls-6..9.test` | ✅ | Updated for lossless router-ids |
| `rfc/rfc9085.txt` | ✅ | Downloaded for TLV 1099 reference |
| `rfc/README.md` | ✅ | Added BGP-LS section |
| `Makefile` | ✅ | Has `functional` target |

## API Breaking Changes

**ZeBGP now outputs lossless JSON format for BGP-LS attributes:**

| Before | After | Reason |
|--------|-------|--------|
| `"remote-router-id": "x"` | `"remote-router-ids": ["x", "y"]` | Preserve IPv4+IPv6 |
| `"sr-adj": {...}` | `"sr-adj": [{...}, {...}]` | Preserve multiple TLVs |
| `"srv6-endx-sid": [...]` | `"srv6-endx": [...]` | Match ExaBGP key name |

**Impact:** Code parsing ZeBGP's BGP-LS JSON output needs updating.

## ExaBGP Sync Required

ExaBGP has the same duplicate-key bug causing data loss. Needs fix:

**Files to modify in ExaBGP** (`src/exabgp/bgp/message/update/attribute/bgpls/`):

| File | Change |
|------|--------|
| `link/remotetepv4.py` | TLV 1030: `remote-router-id` → `remote-router-ids`, accumulate |
| `link/remotetepv6.py` | TLV 1031: `remote-router-id` → `remote-router-ids`, accumulate |
| `link/adjacencysid.py` | TLV 1099: `sr-adj` output as array element |
| `link/__init__.py` | Add accumulation logic for multi-instance TLVs |

**Key change:** Add `ACCUMULATING_KEYS = {'sr-adj', 'local-router-ids', 'remote-router-ids'}` and merge into arrays instead of overwriting.
