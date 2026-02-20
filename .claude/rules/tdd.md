---
paths:
  - "**/*.go"
---

# Test-Driven Development

**BLOCKING:** Tests must exist and fail before implementation.
Rationale: `.claude/rationale/tdd.md`

## TDD Cycle

1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run test → MUST FAIL (paste output)
3. Write minimum implementation
4. Run test → MUST PASS (paste output)
5. Refactor while green

## Test Patterns

- **Table-driven:** `tests := []struct{...}` with `t.Run(tt.name, ...)`
- **Round-trip:** Every pack/unpack must pass `original → packed → unpacked == original`
- **Fuzz (MANDATORY for wire format):** All external input parsing must have fuzz tests
- **Non-default params:** Always test with non-default/non-zero parameter values

## Boundary Testing (MANDATORY)

All numeric ranges MUST test 3 points:
1. Last valid value
2. First invalid below (if applicable)
3. First invalid above

| Example Range | Last Valid | Invalid Below | Invalid Above |
|---------------|------------|---------------|---------------|
| Port 1-65535 | 65535 | 0 | 65536 |
| Hold time 0,3+ | 0, 3 | 1, 2 | N/A |
| Prefix IPv4 0-32 | 32 | N/A | 33 |
| Message len 19-4096 | 4096 | 18 | 4097 |

## Coverage

| Code Type | Target |
|-----------|--------|
| Wire format (pack/unpack) | 90%+ |
| Public functions | 100% |
| Error paths | 100% |

## Investigation → Test Rule

If you debug something, add a test so future devs don't re-investigate.

## Forbidden

- Implementation before test exists → delete impl, write test
- Test passes immediately → invalid test, add failing assertion
- Claiming "done" without pasting test output → run it, paste it
