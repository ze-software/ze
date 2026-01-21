# Spec: json-direction-in-message

## Task

Move `direction` field from JSON root level into the `message` wrapper.

**Current:**
```json
{"ze-bgp":"6.0.0", "direction":"received", "message":{"type":"notification", "id":42}, ...}
```

**Desired:**
```json
{"ze-bgp":"6.0.0", "message":{"type":"notification", "id":42, "direction":"received"}, ...}
```

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - JSON format examples need updating

### RFC Summaries
N/A - Internal API format, not protocol-related.

**Key insights:**
- `direction` indicates message flow direction ("sent"/"received")
- Used by all message types: UPDATE, OPEN, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH
- RIB plugin parses this field from JSON events

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestJSONEncoderDirectionInMessage` | `internal/plugin/json_test.go` | direction inside message wrapper | |
| `TestJSONEncoderNotification` | `internal/plugin/json_test.go` | Update existing to check message.direction | |
| `TestJSONEncoderNotificationSent` | `internal/plugin/json_test.go` | Update existing | |

### Boundary Tests
N/A - String field, no numeric boundaries.

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Existing plugin tests | `test/data/plugin/*.ci` | Should work unchanged | |

### Future (if deferring any tests)
None.

## Files to Modify

### internal/plugin/json.go
- `Notification()` - move `msg["direction"]` inside message wrapper
- `Open()` - move `msg["direction"]` inside message wrapper
- `Keepalive()` - move `msg["direction"]` inside message wrapper
- `RouteRefresh()` - move `msg["direction"]` inside message wrapper
- Add helper `setMessageDirection()` similar to `setMessageID()`

### internal/plugin/text.go
- `formatFilterResultJSON()` - move direction inside message wrapper
- `formatRawFromResult()` - move direction inside message wrapper

### internal/plugin/rib/event.go
- Move `Direction` field inside message wrapper struct

### internal/plugin/json_test.go
- Update `TestJSONEncoderNotification` assertion
- Update `TestJSONEncoderNotificationSent` assertion
- Update other tests checking `result["direction"]`

### internal/plugin/text_test.go
- Update `TestFormatUpdateParsedJSON` assertion

### internal/plugin/server_test.go
- Update `TestServerFormatMessageNotificationJSON` assertion

### internal/plugin/rib/rib_test.go
- Update test input JSON strings

### docs/architecture/api/architecture.md
- Update all JSON examples showing direction

## Files to Create
None.

## Implementation Steps

1. **Write unit tests** - Add test verifying direction in message wrapper
2. **Run tests** - Verify FAIL (paste output)
3. **Implement json.go** - Move direction into message wrapper
4. **Implement text.go** - Move direction in formatFilterResultJSON
5. **Implement rib/event.go** - Update Event struct
6. **Update existing tests** - Fix assertions for new location
7. **Run tests** - Verify PASS (paste output)
8. **Update docs** - Fix JSON examples in architecture.md
9. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

### Reference Comments
N/A - Internal API format.

### Constraint Comments
N/A - No RFC constraints.

## Implementation Summary

### What Was Implemented
- Added `setMessageDirection()` helper in json.go (similar to `setMessageID()`)
- Updated `Notification()`, `Open()`, `Keepalive()`, `RouteRefresh()` in json.go
- Updated `formatFilterResultJSON()` and `formatRawFromResult()` in text.go
- Added `Direction` field to `MessageInfo` struct in rib/event.go
- Removed root-level `Direction` field from `Event` struct in rib/event.go
- Added `GetDirection()` method to `Event` for accessing direction
- Updated test assertions in json_test.go, server_test.go, text_test.go, rib_test.go
- Updated architecture docs: json-format.md, architecture.md, update-syntax.md, core-design.md, overview.md

### Bugs Found/Fixed
- None

### Investigation → Test Rule
- N/A (straightforward refactoring)

### Design Insights
- `message` wrapper now contains all common message metadata: type, id, direction, time
- This is cleaner than having some fields at root level and some in the wrapper

### Deviations from Plan
- Also updated architecture docs not listed in the spec (core-design.md, overview.md)
- Added `GetDirection()` method for easier access to direction field

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified)
- [x] Implementation complete
- [x] Tests PASS (verified)
- [x] Boundary tests cover all numeric inputs (N/A - string field)

### Verification
- [x] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A - internal API format)
- [x] RFC references added to code (N/A - internal API format)
- [x] RFC constraint comments added (N/A - no RFC constraints)

### Completion (after tests pass - see Completion Checklist)
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/120-json-direction-in-message.md`
- [ ] All files committed together
