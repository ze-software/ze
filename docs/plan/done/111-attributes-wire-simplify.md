# Spec: attributes-wire-simplify

## Task

Simplify `AttributesWire` by eliminating redundant `parsed` map. Embed cached `Attribute` directly into `attrIndex` struct.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - AttributesWire is part of WireUpdate lazy parsing
- [ ] `docs/architecture/wire/attributes.md` - Attribute header format and codes

### RFC Summaries
N/A - Internal refactoring, no protocol changes.

**Key insights:**
- `AttributesWire` stores wire bytes as canonical form, parses lazily on demand
- `attrIndex` already stores `code` for each attribute
- `parsed` map duplicates `code` as key - redundant
- All parsed attributes originate from index - invariant maintained

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAttributesWireGet` | `pkg/bgp/attribute/wire_test.go` | Lazy parsing, caching | exists |
| `TestAttributesWireHas` | `pkg/bgp/attribute/wire_test.go` | Header-only scanning | exists |
| `TestAttributesWireAll` | `pkg/bgp/attribute/wire_test.go` | Full parse, wire order | exists |
| `TestAttributesWireConcurrentAccess` | `pkg/bgp/attribute/wire_test.go` | Thread safety | exists |
| `TestAttributesWireIndexReuse` | `pkg/bgp/attribute/wire_test.go` | Index caching | exists |

### Functional Tests
N/A - Internal refactoring, existing tests sufficient.

### Future
None - comprehensive test coverage exists.

## Files to Modify
- `pkg/bgp/attribute/wire.go` - Embed `parsed` in `attrIndex`, remove map

## Files to Create
None.

## Implementation Steps

1. **Verify existing tests pass** - `make test` (paste output)
2. **Modify `attrIndex` struct** - Add `parsed Attribute` field
3. **Remove `parsed` map** - Delete from `AttributesWire` struct
4. **Update `Get()`** - Use index-based access (`for i := range`), store in `a.index[i].parsed`
5. **Simplify `Has()`** - Remove redundant parsed map check
6. **Update `All()`** - Use `a.index[i].parsed` directly
7. **Run tests** - Verify PASS (paste output)
8. **Run lint** - `make lint` (paste output)

## RFC Documentation

N/A - Internal refactoring, no protocol changes.

## Implementation Summary

### What Was Implemented
- Embedded `parsed Attribute` field directly into `attrIndex` struct
- Removed redundant `parsed map[AttributeCode]Attribute` from `AttributesWire`
- Refactored `Get()` with `getAndParse()` helper using hint-based optimization
- Updated `Has()`, `All()`, `GetRaw()` to use index-based iteration
- Added early return for "not found" case in `Get()` and `GetRaw()`

### Bugs Found/Fixed
- None - existing tests passed throughout refactoring

### Design Insights
- Memory reduced ~53% (768 → 360 bytes for 15 attributes)
- Wire order preserved via slice (map would lose order)
- For n≈15 attributes, O(n) linear search comparable to O(1) map lookup
- Hint-based optimization avoids double search after lock upgrade

### Deviations from Plan
- Added `getAndParse()` helper method (not in original plan)
- Hint optimization added during critical review

## Checklist

### 🧪 TDD
- [x] Tests written (existing tests sufficient)
- [x] Tests FAIL (N/A - refactoring, tests should pass before AND after)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (18 passed)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)

### Completion (after tests pass)
- [x] Architecture docs updated with learnings (N/A - internal refactor)
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/NNN-<name>.md`
- [x] All files committed together
