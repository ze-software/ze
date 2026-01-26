# Spec: boundary-test-coverage

## Task
Add missing boundary tests identified during TDD rule update audit.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/wire/messages.md` - message length constraints
- [x] `docs/architecture/wire/nlri.md` - prefix length limits

### RFC Summaries
- [x] `rfc/short/rfc4271.md` - hold time, message length constraints

**Key insights:**
- Hold time must be 0 or >= 3 (RFC 4271)
- Standard message length 19-4096, extended 19-65535

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateSemanticHoldTime` | `internal/config/editor/validator_test.go` | hold time 0,1,2,3,65535 | ✅ Exists |
| `TestSchemaValidateValue` | `internal/config/schema_test.go` | TypeUint16 65536 invalid | ✅ Exists |
| `TestParseHeaderLengthBounds` | `internal/plugin/bgp/message/header_test.go` | 18,19,4096 | ✅ Exists |
| `TestValidateLengthWithMax` | `internal/plugin/bgp/message/header_test.go` | 4097,65535 | ✅ Exists |
| `TestINETErrors` | `internal/plugin/bgp/nlri/inet_test.go` | IPv4 33 invalid | ✅ Exists |
| `TestINETPrefixLengthBoundary` | `internal/plugin/bgp/nlri/inet_test.go` | IPv4 32/33, IPv6 128/129 | ✅ Added |
| FlowSpec boundary tests | `internal/plugin/update_text_test.go` | DSCP 0,63,64; ICMP 0,255,256 | ✅ Exists |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above | Status |
|-------|-------|------------|---------------|---------------|--------|
| Hold time | 0, 3-65535 | 0, 65535 | 1, 2 | 65536 | ✅ |
| Message length (std) | 19-4096 | 4096 | 18 | 4097 | ✅ |
| Message length (ext) | 19-65535 | 65535 | 18 | N/A (uint16) | ✅ |
| IPv4 prefix len | 0-32 | 32 | N/A | 33 | ✅ |
| IPv6 prefix len | 0-128 | 128 | N/A | 129 | ✅ |
| FlowSpec DSCP | 0-63 | 63 | N/A | 64 | ✅ |
| FlowSpec ICMP | 0-255 | 255 | N/A | 256 | ✅ |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | `test/data/` not applicable | Boundary validation is unit-level; invalid values are rejected at parse time before reaching functional test scope | ✅ |

## Files to Modify
- `internal/plugin/bgp/nlri/inet_test.go` - add IPv6 prefix length 129 boundary test
- `internal/plugin/bgp/nlri/inet.go` - existing validation (no changes needed, already correct)

## Files to Create
- None

**Note:** This is a test-coverage-only spec. The validation code already exists in `inet.go:106-108`. This spec adds tests to verify the existing validation works correctly.

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Audit existing tests** - Found most boundary tests already exist ✅
2. **Add missing IPv6 129 test** - Added `TestINETPrefixLengthBoundary` ✅
3. **Run tests** - All pass ✅
4. **Verify all** - `make lint && make test && make functional` passes ✅

## Implementation Summary

### What Was Implemented
- Added `TestINETPrefixLengthBoundary` to `internal/plugin/bgp/nlri/inet_test.go`
- Tests IPv4 32 (valid), 33 (invalid), IPv6 128 (valid), 129 (invalid)

### Audit Results
Most boundary tests already existed:
- Hold time: `TestValidateSemanticHoldTime` + `TestSchemaValidateValue` (TypeUint16 65536)
- Message length: `TestParseHeaderLengthBounds` + `TestValidateLengthWithMax`
- IPv4 prefix 33: `TestINETErrors`
- FlowSpec: Multiple tests for DSCP 0/63/64, ICMP 0/255/256

Only missing: IPv6 prefix length 129 invalid test.

### Note on 65536 boundaries
Message length and hold time use uint16 wire format - 65536 cannot be represented, so overflow is prevented at type level. The `TypeUint16` schema validation test covers parser-level rejection of 65536.

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (N/A - validation already exists, tests verify existing behavior)
- [x] Implementation complete
- [x] Tests PASS
- [x] Boundary tests cover all numeric inputs

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (42 packages)
- [x] `make functional` passes (97 tests)

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
