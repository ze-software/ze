---
paths:
  - "**/*.go"
---

# Test-Driven Development

**BLOCKING:** Tests must exist and fail before implementation.
Rationale: `.claude/rationale/tdd.md`

## Cycle

1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run → MUST FAIL (paste output)
3. Minimum implementation
4. Run → MUST PASS (paste output)
5. Refactor while green

## Patterns

- **Table-driven:** `tests := []struct{...}` with `t.Run(tt.name, ...)`
- **Round-trip:** `original → packed → unpacked == original`
- **Fuzz (MANDATORY for wire format):** All external input parsing
- **Non-default params:** Always test with non-default/non-zero values

## Boundary Testing (MANDATORY)

All numeric ranges MUST test: last valid, first invalid below, first invalid above.

| Range | Last Valid | Invalid Below | Invalid Above |
|-------|------------|---------------|---------------|
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

## Rules

- If you debug something, add a test so it's never re-investigated
- Implementation before test exists → delete impl, write test
- Test passes immediately → invalid test, add failing assertion
- Claiming "done" without test output → run it, paste it
