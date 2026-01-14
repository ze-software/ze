# Spec: json-direction-in-message

## Task

Move `direction` field from JSON root level into the `message` wrapper.

**Current:**
```json
{"zebgp":"6.0.0", "direction":"received", "message":{"type":"notification", "id":42}, ...}
```

**Desired:**
```json
{"zebgp":"6.0.0", "message":{"type":"notification", "id":42, "direction":"received"}, ...}
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
| `TestJSONEncoderDirectionInMessage` | `pkg/plugin/json_test.go` | direction inside message wrapper | |
| `TestJSONEncoderNotification` | `pkg/plugin/json_test.go` | Update existing to check message.direction | |
| `TestJSONEncoderNotificationSent` | `pkg/plugin/json_test.go` | Update existing | |

### Boundary Tests
N/A - String field, no numeric boundaries.

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Existing plugin tests | `test/data/plugin/*.ci` | Should work unchanged | |

### Future (if deferring any tests)
None.

## Files to Modify

### pkg/plugin/json.go
- `Notification()` - move `msg["direction"]` inside message wrapper
- `Open()` - move `msg["direction"]` inside message wrapper
- `Keepalive()` - move `msg["direction"]` inside message wrapper
- `RouteRefresh()` - move `msg["direction"]` inside message wrapper
- Add helper `setMessageDirection()` similar to `setMessageID()`

### pkg/plugin/text.go
- `formatFilterResultJSON()` - move direction inside message wrapper
- `formatRawFromResult()` - move direction inside message wrapper

### pkg/plugin/rib/event.go
- Move `Direction` field inside message wrapper struct

### pkg/plugin/json_test.go
- Update `TestJSONEncoderNotification` assertion
- Update `TestJSONEncoderNotificationSent` assertion
- Update other tests checking `result["direction"]`

### pkg/plugin/text_test.go
- Update `TestFormatUpdateParsedJSON` assertion

### pkg/plugin/server_test.go
- Update `TestServerFormatMessageNotificationJSON` assertion

### pkg/plugin/rib/rib_test.go
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

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
-

### Bugs Found/Fixed
-

### Investigation → Test Rule
-

### Design Insights
-

### Deviations from Plan
-

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass - see Completion Checklist)
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
