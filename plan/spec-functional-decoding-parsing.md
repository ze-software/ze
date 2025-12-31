# Spec: Decoding and Parsing Functional Tests

**Created:** 2025-12-31
**Status:** In Progress (Phase 1 partial, Phase 2-3 infrastructure done)

## Problem

ZeBGP's functional test tool only supports `encoding` and `api` commands. ExaBGP has two additional test types that ZeBGP should also support:

1. **Decoding tests** - Validate BGP message parsing produces correct JSON
2. **Parsing tests** - Validate config files parse without errors

Test data already exists:
- `test/data/decode/*.test` - 18 decoding test files
- `test/data/parse/*.conf` - 10 parsing test configs

## Current Status

| Component | Status | Notes |
|-----------|--------|-------|
| `zebgp decode` CLI | ✅ Complete | EVPN Type 2 parsed |
| `functional decoding` | ⚠️ Partial | 3/18 tests pass |
| `functional parsing` | ✅ Complete | 10/10 tests pass |
| `make functional` | ✅ Exists | All types integrated |

### Decoding Test Results

| Test | Status | Notes |
|------|--------|-------|
| bgp-evpn-1 | ✅ | EVPN Type 2 with lenient label parsing |
| bgp-open-software-version | ✅ | Capabilities parsed |
| ipv4-unicast-1 | ✅ | IPv4 unicast |
| ipv4-unicast-2 | ❌ | Needs as-path object format, aggregator |
| bgp-flow-1..4 | ❌ | FlowSpec needs structured output |
| bgp-ls-1..10 | ❌ | BGP-LS needs raw NLRI format (no envelope) |

## Requirements

### 1. Decoding Tests (`functional decoding`)

**Purpose:** Verify BGP message/NLRI decoding produces correct JSON output.

**Test file format** (3 lines):
```
<type> [<afi> <safi>]           # e.g., "update l2vpn evpn" or "open"
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
2. ⚠️ Integrate `pkg/bgp/nlri/` parsers (EVPN, FlowSpec, BGP-LS)
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
2. Duplicates parsing code instead of using `pkg/bgp/nlri/`
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

Existing parsers in `pkg/bgp/nlri/`:
- `ParseEVPN()` - All 5 EVPN route types
- `ParseFlowSpec()` - FlowSpec rules
- `ParseBGPLS()` - BGP-LS NLRI

```go
import "github.com/exa-networks/zebgp/pkg/bgp/nlri"

// Parse MP_UNREACH_NLRI for l2vpn evpn
routes, err := nlri.ParseEVPN(nlriData, false)
```

#### 1.3 ExaBGP JSON Format

Expected structure for EVPN routes:
```json
{
  "withdraw": {
    "l2vpn evpn": [
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
- `test/functional/decoding.go` - Test discovery and execution

### Phase 3: Parsing Test Runner

**Status:** ✅ Complete

Files:
- `test/cmd/functional/main.go` - `parsing` command added
- `test/functional/parsing.go` - Test discovery and execution

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
| bgp-evpn-1 | update | l2vpn evpn | `ParseEVPN` |
| bgp-flow-1..4 | update | ipv4 flowspec | `ParseFlowSpec` |
| bgp-ls-1..10 | update/nlri | bgp-ls | `ParseBGPLS` |
| bgp-open-software-version | open | - | Capabilities |
| ipv4-unicast-1..2 | update | ipv4 unicast | IPv4 prefix |

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

- [ ] All 18 decoding tests pass (currently 3/18)
- [x] All 10 parsing tests pass
- [x] `make functional` runs all test types

## Remaining Work

1. **FlowSpec output format** (4 tests)
   - Needs structured component output: `destination-ipv6`, `next-header`, etc.
   - Current output is string-based, expected is structured arrays

2. **BGP-LS raw NLRI format** (10 tests)
   - Tests use `nlri bgp-ls bgp-ls` type - raw NLRI without envelope
   - Expected output is flat JSON without `exabgp`, `neighbor` wrapper
   - Needs test runner to handle different comparison for "nlri" type

3. **as-path object format** (1 test: ipv4-unicast-2)
   - Expected: `{"0": {"element": "as-sequence", "value": [...]}}`
   - Current: `[asn1, asn2, ...]`
   - Also needs AGGREGATOR attribute parsing

## Files Modified

| File | Status | Purpose |
|------|--------|---------|
| `cmd/zebgp/main.go` | ✅ | Added `decode` command |
| `cmd/zebgp/decode.go` | ✅ | Decode logic with EVPN lenient parsing |
| `cmd/zebgp/decode_test.go` | ✅ | Unit tests |
| `test/cmd/functional/main.go` | ✅ | Added decoding/parsing commands |
| `test/functional/decoding.go` | ✅ | Decoding test infrastructure |
| `test/functional/parsing.go` | ✅ | Parsing test infrastructure |
| `Makefile` | ✅ | Has `functional` target |
