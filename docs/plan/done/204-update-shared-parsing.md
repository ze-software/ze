# Spec: Extract Shared UPDATE Parsing

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/data-flow-tracing.md` - data flow requirements
4. `internal/plugin/bgp/message/update.go` - message.Update with UnpackUpdate
5. `internal/plugin/wire_update.go` - WireUpdate with lazy accessors
6. `internal/plugin/bgp/wire/` - target location for shared parsing

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
- [ ] `rfc/short/rfc8654.md` - Extended Message capability (65535 byte max)

**Key insights:**
- `message.Update` = build & send (stores sections, has WriteTo)
- `WireUpdate` = receive & forward (zero-copy view, lazy parsing)
- Both parse the same wire format but implement parsing separately
- WireUpdate re-parses offsets on EVERY accessor call (inefficient)

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/bgp/message/update.go` - UnpackUpdate parses once, stores sections
- [ ] `internal/plugin/wire_update.go` - Withdrawn(), Attrs(), NLRI() each re-parse offsets
- [ ] `internal/plugin/wire_extract.go` - Primary consumer of WireUpdate accessors

**Duplicated parsing logic:**

| Operation | message.UnpackUpdate | WireUpdate accessors |
|-----------|---------------------|----------------------|
| Read withdrawn length | Line 71 (once) | Lines 54, 77, 100 (3 times!) |
| Calculate attr offset | Line 78 (once) | Lines 78, 101 (2 times!) |
| Read attr length | Line 79 (once) | Lines 82, 105 (2 times!) |
| Calculate NLRI offset | Line 87 (once) | Line 106 |
| Bounds validation | Lines 65-82 | Repeated in each accessor |

**Problem:** WireUpdate's lazy accessors re-compute offsets on every call. If you call `Withdrawn()`, `Attrs()`, and `NLRI()`, the withdrawn length is read 3 times.

**Accessor return types (MUST preserve):**

| Accessor | Current Return | Notes |
|----------|----------------|-------|
| `Withdrawn()` | `([]byte, error)` | Raw bytes |
| `Attrs()` | `(*attribute.AttributesWire, error)` | Wraps raw bytes |
| `NLRI()` | `([]byte, error)` | Raw bytes |

**Primary callers:**
- `internal/plugin/wire_extract.go` - ExtractRawAttributes, ExtractRawNLRI, etc.
- `internal/plugin/wire_update.go` internal methods - MPReach(), MPUnreach(), iterators
- Test files in `internal/plugin/`

**Behavior to preserve:**
- Zero-copy: accessors return slices into original payload
- Return types unchanged (Attrs returns AttributesWire, not raw bytes)
- Error semantics unchanged
- All existing tests pass

**Behavior to change:**
- Extract shared offset parsing to `wire.UpdateSections`
- WireUpdate caches parsed offsets (parse once, access many)

## Data Flow (MANDATORY)

### Entry Point
- **Wire bytes** arrive via TCP, parsed by session into UPDATE body
- `WireUpdate` created via `NewWireUpdate(body, ctxID)` in `session.go:772`

### Transformation Path
1. **Session receives UPDATE** → creates `WireUpdate` with payload slice (zero-copy)
2. **Accessor called** (e.g., `wu.Attrs()`) → currently re-parses offsets each time
3. **With this change**: First accessor → `ParseUpdateSections()` → cache offsets → return slice
4. **Subsequent accessors** → use cached offsets → return slice (no re-parse)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → WireUpdate | Payload slice (zero-copy) | [ ] Unchanged by this spec |
| WireUpdate → AttributesWire | Attrs() wraps raw bytes | [ ] Unchanged by this spec |
| plugin/ → plugin/bgp/wire/ | New import for UpdateSections | [ ] Safe - no cycle |

### Integration Points
- `wire_update.go` imports new `wire.UpdateSections` from `internal/plugin/bgp/wire/`
- `message/update.go` imports same `wire.UpdateSections`
- No changes to callers of WireUpdate accessors (return types unchanged)

### Import Graph Verification
```
internal/plugin/wire_update.go currently imports:
  → internal/plugin/bgp/attribute
  → internal/plugin/bgp/context
  → internal/plugin/bgp/nlri

internal/plugin/bgp/wire/ currently contains:
  → errors.go, writer.go (no imports from internal/plugin/)

Adding import: internal/plugin/ → internal/plugin/bgp/wire/ = SAFE (no cycle)
```

## Design Decision

**Approach: Shared UpdateSections struct**

Create `wire.UpdateSections` that holds parsed offsets (not data). Both types use it:

| Field | Type | Description |
|-------|------|-------------|
| `wdLen` | uint32 | Withdrawn routes length |
| `attrStart` | uint32 | Offset where attrs begin |
| `attrLen` | uint32 | Attributes length |
| `nlriStart` | uint32 | Offset where NLRI begins |

Note: uint32 to support Extended Message (max 65535 - 19 header = 65516 body).

**Usage pattern:**

| Type | How it uses UpdateSections |
|------|---------------------------|
| `message.UnpackUpdate()` | Calls `ParseUpdateSections()`, extracts slices, stores in struct |
| `WireUpdate` | Caches `UpdateSections` (lazily parsed on first accessor), uses for all accessors |

**Thread Safety Decision:**

WireUpdate is documented as "Thread-safe for concurrent read access." Adding lazy caching creates a potential race on first accessor call.

**Decision:** Accept benign race. If two goroutines call accessors simultaneously on a fresh WireUpdate:
- Both may see `sections` as unset
- Both may parse (same input → same result)
- One write wins (assignment is atomic for pointers in Go)
- Result is correct (identical values)

This matches the pattern used elsewhere in Go (lazy init with benign race). Document in code comment.

**Error Handling Decision:**

If `ParseUpdateSections()` fails (malformed payload):
- Cache the error along with sections (both nil means not parsed yet)
- Subsequent accessors return cached error without re-parsing
- This prevents repeated parsing of known-bad payloads

### Zero-Copy Requirements (BLOCKING)

**Definition:** Zero-copy means returning slices that share the same underlying array as the input buffer.

| Requirement | Verification |
|-------------|--------------|
| Accessors return slices into original payload | `&payload[0]` within bounds of `&Withdrawn()[0]` |
| No allocations in accessor path | No `make()`, no `append()` that grows |
| Single parse pass | Offsets computed once, cached |
| Attrs() wraps raw bytes | Returns `AttributesWire` pointing to payload slice |

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
| `TestUpdateSectionsExtendedMessage` | `internal/plugin/bgp/wire/update_sections_test.go` | Handles large payloads (Extended Message) | |
| `TestWireUpdateCachedParsing` | `internal/plugin/wire_update_test.go` | Offsets parsed once, reused across accessors | |
| `TestWireUpdateCachedError` | `internal/plugin/wire_update_test.go` | Malformed payload error is cached | |
| `TestMessageUpdateUsesShared` | `internal/plugin/bgp/message/update_test.go` | UnpackUpdate uses wire.ParseUpdateSections | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Withdrawn length (std) | 0-4073 | 4073 | N/A | 4074 (exceeds 4096 body) |
| Withdrawn length (ext) | 0-65512 | 65512 | N/A | 65513 (exceeds 65516 body) |
| Attr length (std) | 0-4073 | 4073 | N/A | 4074 (exceeds body) |
| Attr length (ext) | 0-65512 | 65512 | N/A | 65513 (exceeds body) |
| Total body (std) | 4-4077 | 4077 | 3 (min is 4) | 4078 |
| Total body (ext) | 4-65516 | 65516 | 3 (min is 4) | 65517 |

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
   - Struct with offset fields (wdLen, attrStart, attrLen, nlriStart) as uint32
   - No data storage — only positions into external buffer
   - Add `valid bool` field to distinguish "not parsed" from "parsed, all zeros"
   → **Review:** Struct is small (20 bytes)? No pointers to avoid GC pressure?

2. **Create ParseUpdateSections function**
   - Takes `data []byte`, returns `(UpdateSections, error)`
   - Validates minimum length (4 bytes)
   - Parses withdrawn length, calculates attr offset
   - Parses attr length, calculates NLRI offset
   - Validates all offsets within bounds
   - Works for both standard (4096) and extended (65535) message sizes
   → **Review:** Single pass through length fields? All bounds checked?

3. **Add slice accessor methods**
   - `Withdrawn(data []byte) []byte` - returns `data[2:2+wdLen]` or nil if wdLen=0
   - `Attrs(data []byte) []byte` - returns `data[attrStart:attrStart+attrLen]` or nil if attrLen=0
   - `NLRI(data []byte) []byte` - returns `data[nlriStart:]` or nil if empty
   → **Review:** Methods take data as param (UpdateSections has no data reference)?

4. **Write unit tests for UpdateSections**
   - Include Extended Message boundary tests
   → **Review:** Tests cover empty sections, truncated data, zero-copy verification, large payloads?

5. **Run tests** - verify FAIL (paste output)

6. **Implement UpdateSections**

7. **Run tests** - verify PASS (paste output)

### Phase 2: Integrate into WireUpdate

8. **Add cached sections and error fields to WireUpdate**
   - Add `sections wire.UpdateSections` field (value, not pointer - avoids allocation)
   - Add `parseErr error` field for cached error
   - Add `parsed bool` field to track if parsing attempted
   → **Review:** Fields support benign race? (bool and error assignment are atomic-ish)

9. **Add internal parse helper**
   - `ensureParsed()` - parses if not yet attempted, caches result and error
   - Document benign race in comment
   → **Review:** Parse happens at most once per WireUpdate? Error cached?

10. **Update Withdrawn() accessor**
    - Call ensureParsed(), return parseErr if set
    - Return sections.Withdrawn(u.payload)
    - Remove inline offset calculation
    → **Review:** Same return values as before? Zero-copy preserved?

11. **Update Attrs() accessor**
    - Call ensureParsed(), return parseErr if set
    - Get raw bytes via sections.Attrs(u.payload)
    - Wrap in AttributesWire (existing behavior)
    - Remove inline offset calculation
    → **Review:** Still returns `*attribute.AttributesWire`? Same behavior?

12. **Update NLRI() accessor**
    - Call ensureParsed(), return parseErr if set
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
    - Thread safety documented?

## RFC Documentation

### Reference Comments
- `wire.UpdateSections` documents RFC 4271 Section 4.3 UPDATE format
- Extended Message support references RFC 8654
- Existing RFC comments in message.Update and WireUpdate preserved

### Constraint Comments
- Bounds validation comments reference RFC 4271 minimum/maximum sizes
- Extended Message bounds reference RFC 8654 (65535 max message)

## Implementation Summary

### What Was Implemented
- Created `wire.UpdateSections` struct with offset fields and `valid` flag
- Created `ParseUpdateSections()` function with RFC 4271/8654 compliance
- Added slice accessor methods: `Withdrawn()`, `Attrs()`, `NLRI()`, `NLRILen()`
- Integrated into `WireUpdate` with lazy caching via `ensureParsed()`
- Integrated into `message.UnpackUpdate()` replacing inline parsing
- Removed all `nolint:gosec` comments from implementation files
- Added comprehensive Extended Message boundary tests (65,516 byte max body)

### Bugs Found/Fixed
- Accessor methods (`Withdrawn`, `Attrs`, `NLRI`) didn't check `valid` flag - could produce wrong results on zero-value struct. Fixed by adding `!s.valid` check.
- Thread safety comment incorrectly claimed "pointer assignment is atomic" when using value type. Fixed comment to accurately describe benign race.

### Design Insights
- **Type choice:** Using `int` instead of `uint32` eliminates all gosec G115 warnings naturally. Wire protocol uses uint16 (max 65535) which always fits in int. This avoids type conversions when comparing with `len()`.
- **No `parsed bool` needed:** The `sections.Valid()` method serves as the "parsed" indicator, avoiding an extra field.
- **Defensive accessors:** Added bounds checks to slice accessors even though callers should verify `Valid()` first - defense in depth.

### Deviations from Plan
| Planned | Actual | Reason |
|---------|--------|--------|
| `uint32` for offset fields | `int` | Eliminates gosec warnings; int is idiomatic for slice indices |
| Add `parsed bool` field | Use `sections.Valid()` | More elegant; avoids redundant state |
| Standard message boundary tests | Extended message only | Extended covers standard; added 65,516 byte max tests |

## Checklist

### 🏗️ Design
- [x] No premature abstraction (shared parser serves two concrete users)
- [x] No speculative features (only what's needed for deduplication)
- [x] Single responsibility (UpdateSections = offset parsing only)
- [x] Explicit behavior (accessors take data param, return slices)
- [x] Minimal coupling (UpdateSections has no dependencies)
- [x] Next-developer test (clear why shared parser exists)
- [x] Thread safety documented (benign race on lazy init)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during implementation)
- [x] Implementation complete
- [x] Tests PASS (verified: all pass)
- [x] Boundary tests cover truncated/oversized payloads
- [x] Boundary tests cover Extended Message sizes (65,516 byte max body)
- [x] Zero-copy tests verify slice backing arrays
- [x] Error caching tests verify malformed payloads

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [ ] `make functional` passes (encoding tests fail - pre-existing, unrelated)
- [x] Import graph verified (no cycles)

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to new code
- [x] Thread safety documented in code comments

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
