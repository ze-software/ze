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

## AC-Linked Tests (BLOCKING)

Every AC-N MUST have a test whose assertion directly verifies the AC's **expected behavior**, not just the mechanism used to achieve it.

| AC text says | Test MUST assert | Test MUST NOT assert |
|-------------|-----------------|---------------------|
| "rejected" / "not installed" | Route is absent from delivery / RIB | No error returned (mechanism) |
| "session torn down" | Connection closed + NOTIFICATION sent | NOTIFICATION struct returned (mechanism) |
| "warning logged" | Log entry exists (or counter incremented) | No teardown (absence of something) |
| "rejected at parse time" | Error returned with specific message | Generic error returned |

**The test:** Quote the AC expected behavior in the `VALIDATES:` comment. Read the test assertion. Does it verify that exact behavior? If the assertion would still pass with a stub implementation that does nothing, the test is invalid.

**Red flag:** Test that asserts the ABSENCE of an action ("no NOTIFICATION", "no error") as proof that a DIFFERENT action happened ("routes rejected"). Absence of X does not prove Y.

## Rules

- If you debug something, add a test so it's never re-investigated
- Implementation before test exists → delete impl, write test
- Test passes immediately → invalid test, add failing assertion
- Claiming "done" without test output → run it, paste it
