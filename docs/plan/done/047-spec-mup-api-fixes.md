# Spec: MUP API Fixes

## SOURCE FILES (read before implementation)

```
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. pkg/reactor/reactor.go:2146-2230 - buildAPIMUPNLRI()        │
│  2. pkg/config/routeattr.go:1009-1135 - ParsePrefixSIDSRv6()    │
│  3. pkg/plugin/route.go:2845-2990 - parseMUPArgs()                 │
│  4. test/data/api/mup4.ci - expected wire format                │
│                                                                 │
│  ON COMPLETION: Update docs/plan/CLAUDE_CONTINUATION.md              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Fix issues identified in MUP API implementation critical review.

## Issues to Fix

### Issue 1: Missing Prefix/Address Family Validation (BUG - HIGH PRIORITY)

**Problem:** No validation that prefix/address family matches AFI.

**Location:** `pkg/reactor/reactor.go` - `buildAPIMUPNLRI()`

**Example of bug:**
```
announce ipv4/mup mup-isd 2001:db8::/32 rd 100:100 next-hop 2001::1
```
Produces malformed wire format (IPv6 bytes with AFI=1).

**Fix:** Add validation after parsing prefix/address:
```go
case nlri.MUPISD:
    prefix, err := netip.ParsePrefix(spec.Prefix)
    if err != nil {
        return nil, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
    }
    // NEW: Validate family match
    if spec.IsIPv6 != prefix.Addr().Is6() {
        expected := "IPv4"
        if spec.IsIPv6 {
            expected = "IPv6"
        }
        return nil, fmt.Errorf("prefix %q is not %s", spec.Prefix, expected)
    }
    data = buildMUPPrefix(prefix)
```

Same fix needed for:
- `MUPDSD` - validate address family
- `MUPT1ST` - validate prefix family
- `MUPT2ST` - validate address family

### Issue 2: Incomplete T1ST/T2ST Support (MEDIUM PRIORITY)

**Problem:** TEID, QFI, endpoint fields not implemented.

**Location:** `pkg/reactor/reactor.go:2205, 2218`

**Current state:**
```go
case nlri.MUPT1ST:
    // ...
    // TODO: Add TEID, QFI, endpoint if needed

case nlri.MUPT2ST:
    // ...
    // TODO: Add TEID encoding
```

**Fix:** Implement TEID encoding for T1ST/T2ST:
```go
case nlri.MUPT1ST:
    // ... existing prefix handling ...

    // TEID (4 bytes) - required for T1ST
    if spec.TEID != "" {
        teid, err := strconv.ParseUint(spec.TEID, 0, 32)
        if err != nil {
            return nil, fmt.Errorf("invalid TEID %q: %w", spec.TEID, err)
        }
        data = append(data, byte(teid>>24), byte(teid>>16), byte(teid>>8), byte(teid))
    }

    // QFI (1 byte) - optional
    if spec.QFI != 0 {
        data = append(data, spec.QFI)
    }

    // Endpoint address - optional
    if spec.Endpoint != "" {
        ep, err := netip.ParseAddr(spec.Endpoint)
        if err != nil {
            return nil, fmt.Errorf("invalid endpoint %q: %w", spec.Endpoint, err)
        }
        epBytes := ep.AsSlice()
        data = append(data, byte(len(epBytes)*8))
        data = append(data, epBytes...)
    }
```

**Note:** This requires understanding the exact MUP T1ST/T2ST NLRI format from the draft spec. May need to defer if format is unclear.

### Issue 3: Missing Unit Tests (MEDIUM PRIORITY)

**Problem:** Helper functions lack dedicated unit tests.

**Functions to test:**
1. `convertAPIMUPRoute()` - `pkg/reactor/reactor.go:2088`
2. `parseAPIExtCommunity()` - `pkg/reactor/reactor.go:2286`
3. `parseAPIPrefixSIDSRv6()` - `pkg/reactor/reactor.go:2342`
4. `buildAPIMUPNLRI()` - `pkg/reactor/reactor.go:2146`
5. `parseRD()` - `pkg/reactor/reactor.go:2243`
6. `buildMUPPrefix()` - `pkg/reactor/reactor.go:2231`

**Test file:** Create `pkg/reactor/mup_test.go`

**Test cases for each:**

```go
// TestParseRD
- "100:100" -> Type 0, ASN 100, Value 100
- "1.2.3.4:100" -> Type 1, IP 1.2.3.4, Value 100
- "invalid" -> error
- "" -> zero RD

// TestParseAPIExtCommunity
- "[target:10:10]" -> RT bytes
- "target:10:10" -> RT bytes (without brackets)
- "[origin:100:200]" -> origin bytes
- "invalid" -> error

// TestParseAPIPrefixSIDSRv6
- "l3-service 2001::1 0x48 [64,24,16,0,0,0]" -> full encoding
- "l3-service 2001::1" -> minimal encoding
- "l2-service 2001::1 0x48" -> L2 service type
- "invalid" -> error

// TestBuildAPIMUPNLRI
- IPv4 ISD route
- IPv6 ISD route
- IPv4 DSD route
- IPv6 DSD route
- Family mismatch -> error (after Issue 1 fix)

// TestBuildMUPPrefix
- "10.0.1.0/24" -> [24, 10, 0, 1]
- "2001:db8::/32" -> [32, 0x20, 0x01, 0x0d, 0xb8]
```

### Issue 4: Code Duplication (LOW PRIORITY)

**Problem:** `parseAPIPrefixSIDSRv6()` duplicates `config/routeattr.go:ParsePrefixSIDSRv6()`.

**Options:**
1. **Export and reuse** - Move shared logic to a common package
2. **Accept duplication** - Keep separate for API vs config contexts
3. **Create shared parser** - `pkg/parse/srv6.go`

**Recommendation:** Option 2 (accept duplication) for now. The API parser handles string args differently than config parser. Refactoring adds complexity without clear benefit.

**If refactoring later:**
```go
// pkg/parse/srv6.go
func ParsePrefixSIDSRv6(s string) ([]byte, error)

// Used by both:
// - pkg/config/routeattr.go
// - pkg/reactor/reactor.go
```

## Implementation Steps

### Phase 1: Fix Family Validation Bug (HIGH PRIORITY)

1. **Add validation to buildAPIMUPNLRI():**
   - After parsing prefix in MUPISD case
   - After parsing address in MUPDSD case
   - After parsing prefix in MUPT1ST case
   - After parsing address in MUPT2ST case

2. **Add test case for family mismatch:**
   - `TestParseMUPArgs` - add case for IPv6 prefix with IPv4 AFI

3. **Verify existing tests still pass**

### Phase 2: Add Unit Tests (MEDIUM PRIORITY)

1. **Create `pkg/reactor/mup_test.go`:**
   - TestParseRD
   - TestParseAPIExtCommunity
   - TestParseAPIPrefixSIDSRv6
   - TestBuildMUPPrefix
   - TestBuildAPIMUPNLRI
   - TestConvertAPIMUPRoute

2. **Run tests, verify coverage**

### Phase 3: T1ST/T2ST Support (DEFER)

**Defer until:**
- Clear understanding of MUP T1ST/T2ST NLRI format
- Test cases available for T1ST/T2ST routes

**Document limitation:**
- Add comment noting T1ST/T2ST TEID/QFI/endpoint not yet supported
- Update API documentation

### Phase 4: Verification

1. `make test` - all unit tests pass
2. `make lint` - no new issues
3. `go run ./test/cmd/functional api --all` - 14/14 pass
4. Test family mismatch rejected with clear error

## Checklist

- [x] Family validation added to buildAPIMUPNLRI()
- [x] Family mismatch test case added (10 test cases)
- [x] Unit tests for helper functions created (24 total in mup_test.go)
- [x] make test passes
- [x] make lint passes (pre-existing issues only)
- [x] All 14 API functional tests pass
- [x] docs/plan/CLAUDE_CONTINUATION.md updated
- [x] Spec moved to docs/plan/done/

## Estimated Scope

| Task | Complexity | Files |
|------|------------|-------|
| Family validation | Low | 1 (reactor.go) |
| Unit tests | Medium | 1 new (mup_test.go) |
| T1ST/T2ST | Deferred | - |
| Code dedup | Deferred | - |

**Priority order:** Issue 1 → Issue 3 → (defer others)
