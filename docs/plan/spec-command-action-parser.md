# Spec: command-action-parser

## Task
Refactor duplicated `keyword action value` and `keyword action [values]` parsing into shared helper functions.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command structure
- [ ] `docs/architecture/api/update-syntax.md` - Update command grammar

**Key insights:**
- Pattern `keyword action value` appears 5+ times with identical structure
- `parseBracketedListText` already exists for `[values]` - can be reused
- Each handler duplicates: bounds check, switch on action, value parsing, boundary detection

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseScalarAction` | `pkg/plugin/action_parser_test.go` | set/del with single value | |
| `TestParseScalarActionDel` | `pkg/plugin/action_parser_test.go` | del with/without value (conditional vs unconditional) | |
| `TestParseScalarActionBoundary` | `pkg/plugin/action_parser_test.go` | stops at boundary keywords | |
| `TestParseListAction` | `pkg/plugin/action_parser_test.go` | set/add/del with bracketed list | |
| `TestParseListActionBoundary` | `pkg/plugin/action_parser_test.go` | stops at boundary keywords | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no new numeric fields introduced

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Existing functional tests cover behavior | |

### Future (if deferring any tests)
- None deferred

## Files to Modify
- `pkg/plugin/update_text.go` - replace inline parsing with shared helper
- `pkg/plugin/update_wire.go` - replace inline parsing with shared helper

## Files to Create
- `pkg/plugin/action_parser.go` - shared `ParseScalarAction`, `ParseListAction` helpers
- `pkg/plugin/action_parser_test.go` - unit tests

## Implementation Steps
1. **Write unit tests** - Create tests for `ParseScalarAction` and `ParseListAction` BEFORE implementation
2. **Run tests** - Verify FAIL (paste output)
3. **Implement helpers** - Create `action_parser.go` with:
   - `ParseScalarAction(args []string, boundaryFn func(string) bool) (action string, value string, consumed int, err error)`
   - `ParseListAction(args []string, boundaryFn func(string) bool) (action string, values []string, consumed int, err error)`
4. **Run tests** - Verify PASS (paste output)
5. **Refactor callers** - Replace inline parsing in:
   - `parseNhopSection` (update_text.go)
   - `parsePathInfoSection` (update_text.go)
   - `parseRDSection` (update_text.go)
   - `parseLabelSection` (update_text.go)
   - `parseWireNhopSection` (update_wire.go)
6. **Run all tests** - Verify no regression
7. **Verify all** - `make lint && make test && make functional` (paste output)

## Design Decisions

### Helper Function Signatures

```go
// ScalarActionResult holds the result of parsing a scalar action.
type ScalarActionResult struct {
    Action   string // "set" or "del"
    Value    string // value for set, or conditional del value (empty = unconditional del)
    HasValue bool   // true if value was provided (distinguishes del vs del <value>)
    Consumed int    // args consumed
}

// ParseScalarAction parses: <action> [<value>]
// where action is "set" or "del".
// boundaryFn returns true for tokens that start new sections.
func ParseScalarAction(args []string, boundaryFn func(string) bool) (ScalarActionResult, error)

// ListActionResult holds the result of parsing a list action.
type ListActionResult struct {
    Action   string   // "set", "add", or "del"
    Values   []string // parsed values (uses parseBracketedListText)
    Consumed int      // args consumed
}

// ParseListAction parses: <action> <value> | <action> [ <values> ]
// where action is "set", "add", or "del".
// boundaryFn returns true for tokens that start new sections.
func ParseListAction(args []string, boundaryFn func(string) bool) (ListActionResult, error)
```

### Why This Design
- **Result structs** - cleaner than multiple return values
- **boundaryFn parameter** - allows wire vs text mode to use different boundary detection
- **Reuses `parseBracketedListText`** - no duplication of bracket parsing
- **HasValue field** - distinguishes `del` (unconditional) from `del <value>` (conditional)

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [ ] TBD

### Bugs Found/Fixed
- [ ] TBD

### Design Insights
- [ ] TBD

### Deviations from Plan
- [ ] TBD

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
