---
globs: "**/*.go"
---

# Test-Driven Development

**BLOCKING:** Tests must exist and fail before implementation.

## TDD Cycle
1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run test → MUST FAIL (paste output)
3. Write minimum implementation
4. Run test → MUST PASS (paste output)
5. Refactor while green

## Test Documentation Required

```go
// TestFeatureName verifies [behavior].
//
// VALIDATES: [what correct behavior looks like]
// PREVENTS: [what bug this catches]
func TestFeatureName(t *testing.T) { ... }
```

## Quick Commands

```bash
# Run tests for current package
go test -race ./pkg/bgp/message/... -v

# Run all tests
make test
```

## Forbidden
- Implementation before test exists → Delete impl, write test
- Test that passes immediately → Invalid test, add failing assertion
- Claiming "done" without pasting test output → Run test, paste output
