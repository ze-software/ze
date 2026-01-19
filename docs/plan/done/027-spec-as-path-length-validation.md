# Spec: AS Path Length Validation

**Status:** ✅ COMPLETED (2025-12-27)

---

## Task

Add AS_PATH attribute validation per RFC 4271:
1. Validate segment types (1-4 only)
2. Add configurable maximum AS path length limit (DoS protection)
3. Return appropriate error codes (Malformed AS_PATH = subcode 11)

## Implementation Summary

### Files Modified

| File | Change |
|------|--------|
| `internal/bgp/attribute/attribute.go:28-31` | Added `ErrMalformedASPath` error |
| `internal/bgp/attribute/aspath.go:25-26` | Fixed RFC 5065 constants (ASConfedSequence=3, ASConfedSet=4) |
| `internal/bgp/attribute/aspath.go:34-39` | Added `MaxASPathTotalLength = 1000` |
| `internal/bgp/attribute/aspath.go:304-309` | Added segment type validation (1-4) |
| `internal/bgp/attribute/aspath.go:311-314` | Added total path length check |
| `internal/bgp/attribute/aspath_test.go` | Added 7 new tests |
| `internal/bgp/attribute/as4_test.go:565-570` | Fixed wire data for AS_CONFED_SEQUENCE |

### Tests Added

| Test | Purpose |
|------|---------|
| `TestParseASPathInvalidSegmentType` | Validates types 0,5+ rejected, 1-4 accepted |
| `TestParseASPathMaxLength` | Validates 1000 ASN limit |
| `TestParseASPathEmptySegment` | Validates count=0 accepted |
| `TestParseASPathConfederationTypes` | Validates wire format 0x03/0x04 |
| `TestParseASPathConfederationPathLength` | Validates confed excluded from length |
| `TestParseASPath2ByteValidation` | Validates 2-byte mode works |
| `TestASPathSegmentTypes` | Updated with RFC references |

### Bug Fix Discovered

During implementation, discovered pre-existing RFC 5065 violation:
- `ASConfedSet` was 3 (should be 4)
- `ASConfedSequence` was 4 (should be 3)

Fixed to match RFC 5065 Section 3.

## Verification

```
✅ make test: PASS
✅ make lint: 0 issues
```

## RFC References

- **RFC 4271 Section 4.3** - AS_PATH encoding format
- **RFC 4271 Section 6.3** - Error Subcode 11 (Malformed AS_PATH)
- **RFC 5065 Section 3** - AS_CONFED_SEQUENCE=3, AS_CONFED_SET=4
- **RFC 6793** - 4-octet AS numbers

## Verification Checklist

- [x] Tests written and shown to FAIL first
- [x] `ErrMalformedASPath` error added
- [x] Segment type validation (1-4)
- [x] Maximum path length validation (1000)
- [x] Implementation makes tests pass
- [x] `make test` passes
- [x] `make lint` passes
- [x] RFC references in code comments
- [x] RFC 5065 constant bug fixed
- [x] Critical review performed
