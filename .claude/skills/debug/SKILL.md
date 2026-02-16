# Debug Failing Tests

Investigate and fix test failures using parallel hypotheses.

The user will paste failing test output as context.

## Steps

1. **Read the failing test output** provided by the user
2. **Identify the failing tests:** Extract test names, packages, error messages, and expected vs actual values
3. **Launch 3 parallel investigation tasks:**

   **Task 1 — Format/parsing mismatch:**
   Check test expectations against actual output formats. Are the tests expecting a different structure, field name, or encoding than what the code produces?

   **Task 2 — Data flow issue:**
   Trace the data from source through each transformation. Is data being lost, corrupted, or transformed incorrectly between layers?

   **Task 3 — Configuration/initialization issue:**
   Check setup, defaults, and initialization order. Are dependencies missing, nil, or initialized in the wrong sequence?

4. **Each task must:**
   - Read relevant source code
   - Form a hypothesis
   - Implement a fix
   - Run `go test ./...` to verify the specific fix
5. **Run full verification:** `make lint && make unit-test && make functional-test` — the fix must not break anything else
6. **Report back** with: root cause, which hypothesis was correct, and full test suite passing

## Rules

- Do NOT modify test expectations unless the tests are genuinely wrong per the spec
- If multiple hypotheses find real issues, fix all of them
- If none of the 3 hypotheses explain the failure, report what was ruled out and ask the user for guidance
