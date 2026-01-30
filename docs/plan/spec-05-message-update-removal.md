# Spec: Remove message.Update, Consolidate to WireUpdate

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/message/update.go` - type to remove
4. `internal/plugin/wire_update.go` - target type
5. `docs/architecture/core-design.md` - doc to update (Section 10)

## Task

Remove `message.Update` type and consolidate all UPDATE handling to `WireUpdate`. Update `core-design.md` Section 10 to reflect completed migration.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - Section 10 claims message.Update should be removed
- [ ] `docs/architecture/wire/messages.md` - wire format details
- [ ] `docs/architecture/update-building.md` - UPDATE construction patterns

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - UPDATE message format

**Key insights:**
- `message.Update` builds complete messages WITH header
- `WireUpdate` stores payload only, has context-id for zero-copy forwarding
- Both do similar parsing/extraction of sections

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/bgp/message/update.go` - UPDATE type with WriteTo, UnpackUpdate
- [ ] `internal/plugin/wire_update.go` - WireUpdate with lazy parsing, context tracking

**message.Update usage (11 files):**

| File | Usage Pattern |
|------|---------------|
| `reactor/peer.go` | 12 functions build `*message.Update` for sending |
| `reactor/reactor.go` | 8 functions build `*message.Update` for API commands |
| `reactor/session.go` | `SendUpdate(*message.Update)` |
| `rib/commit.go` | `buildGroupedUpdateTwoLevel`, `buildSingleUpdate` |
| `rib/update.go` | `BuildGroupedUpdate`, `BuildGroupedUpdates` |

**Key difference:**

| Aspect | message.Update | WireUpdate |
|--------|----------------|------------|
| Header | WriteTo() adds 19-byte header | Payload only (no header) |
| Parsing | UnpackUpdate pre-extracts sections | Lazy parsing on demand |
| Context | None | sourceCtxID, messageID, sourceID |
| Purpose | Build & send | Receive & forward |

**Behavior to preserve:**
- UPDATE messages sent to peers must have correct wire format
- Chunking/splitting for large updates
- All build* functions produce valid UPDATEs

**Behavior to change:**
- Consolidate to single type for both send and receive paths

## Design Decision

**Option A: Extend WireUpdate with WriteTo**
- Add `WriteTo(buf, off) int` that writes WITH header
- Keep lazy parsing for receive path
- Single type for both directions

**Option B: Create UpdateBuilder that produces WireUpdate**
- Builder pattern for constructing
- WireUpdate for transport/storage
- Clearer separation of concerns

**Recommended: Option A** - simpler, less code, WireUpdate already has payload

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWireUpdateWriteTo` | `internal/plugin/wire_update_test.go` | WriteTo produces valid UPDATE with header | |
| `TestWireUpdateRoundTrip` | `internal/plugin/wire_update_test.go` | Build → WriteTo → Parse roundtrip | |
| `TestWireUpdateChunking` | `internal/plugin/wire_update_test.go` | Large NLRI splits correctly | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI size | 0-4077 | 4077 (std) | N/A | 4078 triggers split |
| Attr size | 0-4077 | 4077 | N/A | 4078 error |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing update tests | `test/update/*.ci` | Verify no regression | |

## Files to Modify

- `internal/plugin/wire_update.go` - Add WriteTo, NewWireUpdateFromParts
- `internal/plugin/bgp/reactor/peer.go` - Replace message.Update with WireUpdate
- `internal/plugin/bgp/reactor/reactor.go` - Replace message.Update with WireUpdate
- `internal/plugin/bgp/reactor/session.go` - Update SendUpdate signature
- `internal/plugin/bgp/rib/commit.go` - Replace message.Update with WireUpdate
- `internal/plugin/bgp/rib/update.go` - Replace message.Update with WireUpdate
- `docs/architecture/core-design.md` - Update Section 10 to mark as completed

## Files to Delete

- `internal/plugin/bgp/message/update.go` - Replaced by WireUpdate
- `internal/plugin/bgp/message/update_test.go` - Tests moved to wire_update_test.go
- `internal/plugin/bgp/message/update_split_test.go` - Tests moved

## Implementation Steps

### Phase 1: Extend WireUpdate

1. **Add builder constructor**
   ```
   NewWireUpdateFromParts(withdrawn, attrs, nlri []byte, ctxID ContextID) *WireUpdate
   ```
   → **Review:** Does it build valid payload format?

2. **Add WriteTo method**
   ```
   WriteTo(buf []byte, off int) int  // Writes WITH 19-byte header
   ```
   → **Review:** Produces identical output to message.Update.WriteTo?

3. **Add ChunkNLRI to wire package**
   → **Review:** Same logic as message.ChunkNLRI?

4. **Write unit tests** - roundtrip, chunking
   → **Review:** Tests FAIL before implementation?

5. **Run tests** - verify FAIL (paste output)

6. **Implement** - minimal code to pass

7. **Run tests** - verify PASS (paste output)

### Phase 2: Migrate Callers

8. **Update reactor/session.go**
   - Change `SendUpdate(*message.Update)` to `SendUpdate(*WireUpdate)`
   → **Review:** All callers updated?

9. **Update reactor/peer.go**
   - Change all build* functions to return `*WireUpdate`
   - ~12 functions to update
   → **Review:** Each function produces same wire output?

10. **Update reactor/reactor.go**
    - Change all build* functions to return `*WireUpdate`
    - ~8 functions to update
    → **Review:** Each function produces same wire output?

11. **Update rib/commit.go**
    - Change build functions to return `*WireUpdate`
    → **Review:** CommitService still works?

12. **Update rib/update.go**
    - Change BuildGroupedUpdate(s) to return `*WireUpdate`
    → **Review:** All callers handle new type?

### Phase 3: Cleanup

13. **Delete message/update.go**
    → **Review:** No remaining imports?

14. **Delete message/update_test.go, update_split_test.go**
    → **Review:** Coverage maintained in new location?

15. **Update core-design.md Section 10**
    - Change `message.Update | Remove | WireUpdate` to completed status
    - Or remove the "What Gets Eliminated" section if all done
    → **Review:** Doc accurately reflects code?

### Phase 4: Verify

16. **Run full verification**
    ```
    make lint && make test && make functional
    ```
    → **Review:** Zero failures?

17. **Final self-review**
    - All message.Update references gone?
    - Wire format identical?
    - No performance regression?

## RFC Documentation

### Reference Comments
- Existing RFC 4271 Section 4.3 comments preserved in WireUpdate

### Constraint Comments
- UPDATE format constraints already documented in WireUpdate

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
- [ ] No premature abstraction (single type, not hierarchy)
- [ ] No speculative features (only what's needed)
- [ ] Single responsibility (WireUpdate = UPDATE transport)
- [ ] Explicit behavior (WriteTo clearly writes with header)
- [ ] Minimal coupling (callers don't care about internals)
- [ ] Next-developer test (clear which type to use)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
