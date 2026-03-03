# 217 — Fix ExaBGP Tests

## Objective

Fix the ExaBGP compatibility test infrastructure so that test failures reflect real encoding gaps rather than test harness bugs.

## Decisions

- Config parsing success was decoupled from wire encoding correctness — a config that parses without error may still produce wrong wire bytes.
- Test infrastructure fixes were prioritized before attempting to pass more tests, to get accurate signal on what was actually broken.

## Patterns

- Fix the test harness before fixing the code under test — misleading test results waste more time than fixing them costs.

## Gotchas

- Spec predicted 33/37 tests would pass after infrastructure fixes. Actual result: 20/37. The extra 13 failures were wire encoding feature gaps not previously visible because the harness masked them. Config parsing and wire encoding are independent correctness dimensions.

## Files

- `internal/exabgp/` — ExaBGP migration and serialization
- `test/` — ExaBGP compatibility test cases
