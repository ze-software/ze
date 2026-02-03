# Spec: Extract Shared UPDATE Parsing

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/message/update.go` - message.Update with UnpackUpdate
4. `internal/plugin/wire_update.go` - WireUpdate with lazy accessors
5. `internal/plugin/bgp/wire/` - target location for shared parsing

## Task

Extract duplicated UPDATE section parsing into shared `wire.UpdateSections`. Both `message.Update` and `WireUpdate` will use this shared parser. Fix WireUpdate's inefficient re-parsing on every accessor call.

**NOT doing:** Type consolidation. Both types remain — they serve different purposes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - UPDATE handling architecture
- [ ] `docs/architecture/wire/messages.md` - wire format details
- [ ] `docs/architecture/buffer-architecture.md` - zero-copy patterns

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - UPDATE message format (Section 4.3)

**Key insights:**
- `message.Update` = build & send (stores sections, has WriteTo)
- `WireUpdate` = receive & forward (zero-copy view, lazy parsing)
- Both parse the same wire format but implement parsing separately
- WireUpdate re-parses offsets on EVERY accessor call (inefficient)

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/bgp/message/update.go` - UnpackUpdate parses once, stores sections
- [ ] `internal/plugin/wire_update.go` - Withdrawn(), Attrs(), NLRI() each re-parse offsets

**Duplicated parsing logic:**

| Operation | message.UnpackUpdate | WireUpdate accessors |
|-----------|---------------------|----------------------|
| Read withdrawn length | Line 71 (once) | Lines 54, 77, 100 (3 times!) |
| Calculate attr offset | Line 78 (once) | Lines 78, 101 (2 times!) |
| Read attr length | Line 79 (once) | Lines 82, 105 (2 times!) |
| Calculate NLRI offset | Line 87 (once) | Line 106 |
| Bounds validation | Lines 65-82 | Repeated in each accessor |

**Problem:** WireUpdate's lazy accessors re-compute offsets on every call. If you call `Withdrawn()`, `Attrs()`, and `NLRI()`, the withdrawn length is read 3 times.

**Behavior to preserve:**
- Zero-copy: accessors return slices into original payload
- Decode-on-demand: parsing only when accessor called
- message.Update build/send functionality unchanged
- All existing tests pass

**Behavior to change:**
- Extract shared offset parsing to `wire.UpdateSections`
- WireUpdate caches parsed offsets (parse once, access many)

## Design Decision

**Approach: Shared UpdateSections struct**

Create `wire.UpdateSections` that holds parsed offsets (not data). Both types use it:

| Field | Type | Description |
|-------|------|-------------|
| `wdLen` | uint16 | Withdrawn routes length |
| `attrStart` | uint16 | Offset where attrs begin |
| `attrLen` | uint16 | Attributes length |
| `nlriStart` | uint16 | Offset where NLRI begins |

**Usage pattern:**

| Type | How it uses UpdateSections |
|------|---------------------------|
| `message.UnpackUpdate()` | Calls `ParseUpdateSections()`, extracts slices, stores in struct |
| `WireUpdate` | Caches `*UpdateSections` (lazily parsed on first accessor), uses for all accessors |

### Zero-Copy Requirements (BLOCKING)

**Definition:** Zero-copy means returning slices that share the same underlying array as the input buffer.

| Requirement | Verification |
|-------------|--------------|
| Accessors return slices into original payload | `&payload[0]` within bounds of `&Withdrawn()[0]` |
| No allocations in accessor path | No `make()`, no `append()` that grows |
| Single parse pass | Offsets computed once, cached |

**Forbidden:**
- `make([]byte, ...)` in accessor path
- `copy(dst, src)` in accessor path
- Re-parsing offsets on each accessor call

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateSectionsParse` | `internal/plugin/bgp/wire/update_sections_test.go` | ParseUpdateSections extracts correct offsets | |
| `TestUpdateSectionsEmpty` | `internal/plugin/bgp/wire/update_sections_test.go` | Handles empty sections (len=0) correctly | |
| `TestUpdateSectionsTruncated` | `internal/plugin/bgp/wire/update_sections_test.go` | Returns error for truncated payloads | |
| `TestUpdateSectionsZeroCopy` | `internal/plugin/bgp/wire/update_sections_test.go` | Slice accessors return views into original | |
| `TestWireUpdateCachedParsing` | `internal/plugin/wire_update_test.go` | Offsets parsed once, reused across accessors | |
| `TestMessageUpdateUsesShared` | `internal/plugin/bgp/message/update_test.go` | UnpackUpdate uses wire.ParseUpdateSections | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Withdrawn length | 0-4073 | 4073 | N/A | 4074 (exceeds body) |
| Attr length | 0-4073 | 4073 | N/A | 4074 (exceeds body) |
| Total body | 4-4096 | 4096 | 3 (min is 4) | 4097 (exceeds max) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing update tests | `test/update/*.ci` | Verify no regression | |
| existing forward tests | `test/forward/*.ci` | Zero-copy forwarding unchanged | |

## Files to Create

- `internal/plugin/bgp/wire/update_sections.go` - UpdateSections struct and ParseUpdateSections()
- `internal/plugin/bgp/wire/update_sections_test.go` - Unit tests

## Files to Modify

- `internal/plugin/wire_update.go` - Cache UpdateSections, use for accessors
- `internal/plugin/bgp/message/update.go` - UnpackUpdate uses ParseUpdateSections
- `internal/plugin/bgp/message/update_test.go` - Update tests if needed

## Implementation Steps

### Phase 1: Create Shared Parser

1. **Create wire.UpdateSections struct**
   - Struct with offset fields (wdLen, attrStart, attrLen, nlriStart)
   - No data storage — only positions into external buffer
   → **Review:** Struct is small (8-16 bytes)? No pointers to avoid GC pressure?

2. **Create ParseUpdateSections function**
   - Takes `data []byte`, returns `(UpdateSections, error)`
   - Validates minimum length (4 bytes)
   - Parses withdrawn length, calculates attr offset
   - Parses attr length, calculates NLRI offset
   - Validates all offsets within bounds
   → **Review:** Single pass through length fields? All bounds checked?

3. **Add slice accessor methods**
   - `Withdrawn(data []byte) []byte` - returns `data[2:2+wdLen]`
   - `Attrs(data []byte) []byte` - returns `data[attrStart:attrStart+attrLen]`
   - `NLRI(data []byte) []byte` - returns `data[nlriStart:]`
   → **Review:** Methods take data as param (UpdateSections has no data reference)?

4. **Write unit tests for UpdateSections**
   → **Review:** Tests cover empty sections, truncated data, zero-copy verification?

5. **Run tests** - verify FAIL (paste output)

6. **Implement UpdateSections**

7. **Run tests** - verify PASS (paste output)

### Phase 2: Integrate into WireUpdate

8. **Add cached sections field to WireUpdate**
   - Add `sections *wire.UpdateSections` field
   - Initially nil, populated on first accessor call
   → **Review:** Field is pointer to allow nil check?

9. **Add internal parse helper**
   - `ensureParsed() error` - parses if sections is nil, caches result
   - Called at start of Withdrawn(), Attrs(), NLRI()
   → **Review:** Parse happens once? Error cached or re-attempted?

10. **Update Withdrawn() accessor**
    - Call ensureParsed()
    - Return sections.Withdrawn(u.payload)
    - Remove inline offset calculation
    → **Review:** Same return values as before? Zero-copy preserved?

11. **Update Attrs() accessor**
    - Call ensureParsed()
    - Use cached attrStart/attrLen
    - Remove inline offset calculation
    → **Review:** Same return values as before?

12. **Update NLRI() accessor**
    - Call ensureParsed()
    - Return sections.NLRI(u.payload)
    - Remove inline offset calculation
    → **Review:** Same return values as before?

13. **Run WireUpdate tests** - verify all pass

### Phase 3: Integrate into message.Update

14. **Update UnpackUpdate to use shared parser**
    - Call wire.ParseUpdateSections(data)
    - Use returned offsets to extract section slices
    - Remove duplicated offset calculation code
    → **Review:** Same behavior as before? Error handling unchanged?

15. **Run message.Update tests** - verify all pass

### Phase 4: Verify

16. **Run full verification** - make lint, make test, make functional
    → **Review:** Zero failures? No performance regression?

17. **Final review**
    - Duplicated parsing code removed?
    - Zero-copy preserved?
    - WireUpdate parses once per instance?

## RFC Documentation

### Reference Comments
- `wire.UpdateSections` documents RFC 4271 Section 4.3 UPDATE format
- Existing RFC comments in message.Update and WireUpdate preserved

### Constraint Comments
- Bounds validation comments reference RFC 4271 minimum/maximum sizes

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- TBD

### Bugs Found/Fixed
- TBD

### Design Insights
- TBD

### Deviations from Plan
- TBD

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (shared parser serves two concrete users)
- [ ] No speculative features (only what's needed for deduplication)
- [ ] Single responsibility (UpdateSections = offset parsing only)
- [ ] Explicit behavior (accessors take data param, return slices)
- [ ] Minimal coupling (UpdateSections has no dependencies)
- [ ] Next-developer test (clear why shared parser exists)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover truncated/oversized payloads
- [ ] Zero-copy tests verify slice backing arrays

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read
- [ ] RFC references added to new code

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
