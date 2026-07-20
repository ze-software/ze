# Test Deletion and Weakening

**When:** A red test means the CODE is wrong by default
**Severity:** advisory

## Directives

Rationale: `ai/rationale/no-test-deletion.md`

A red test means the CODE is wrong by default. Diagnose the failure and fix the
source. Do NOT weaken the test to make it green. ASK the user before deleting OR
weakening any test code (`*_test.go`, `.ci`, `Test*`, `t.Run`, assertions, table
entries). Exception: the user already explicitly requested it.

**Legitimate:** testing removed functionality, duplicating another test, fundamentally wrong, replacing with better coverage.
**Not legitimate:** failing and hard to fix, slow, "annoying", don't understand what it checks.

## Mechanically blocked (c_test_weakening in pretool-writeedit.py)

Blocked on Edit / Write / MultiEdit to a test file (exit 2):

- adding `t.Skip` / `t.Skipf` / `t.SkipNow` (the test stops running)
- removing assertions (any net drop, not only all-removed)
- downgrading fatal assertions to non-fatal (`require` -> `assert`, `t.Fatal` -> `t.Error`)
- commenting out assertions
- adding an `ignore` build tag (file dropped from the build)
- deleting a `Test`/`Fuzz`/`Benchmark` func, `t.Run` cases, or table rows
- removing `.ci` test lines

**Not detected (by design, to avoid false positives):** changing an expected
value in place while the assertion structure stays (e.g. `Equal(t, 1, x)` ->
`Equal(t, 2, x)`). This is the one weakening the hook cannot see; treat it with
the same discipline manually. Adjusting an expected value to match broken code is
the same violation as removing the assertion.

## Test Rewrite as Replacement (BLOCKING)

When fixing a new issue that happens to touch an area with existing tests, ADD a
new test case or function for the new issue. Do not repurpose an existing test to
cover the new behavior. The old test verified a behavior that still needs coverage.

| Scenario | Correct | Wrong |
|----------|---------|-------|
| New bug in `parsePeer`, existing `TestParsePeer` | Add `TestParsePeerRejectsEmpty` alongside `TestParsePeer` | Rewrite `TestParsePeer` to test the new edge case |
| Table-driven test, new case needed | Add a row to the table | Replace an existing row with the new case |
| Existing test fails because code changed | Fix the code so both old test and new test pass | Rewrite the old test to match the changed (broken) code |

**Why the hook cannot catch this:** the rewrite maintains the same structural
shape (same function count, same assertion count), so the mechanical check sees
no weakening. The coverage loss is semantic, not structural.

**Detection:** `/ze-review` step 0 (`audit-test-relaxation.py`) flags structural
changes. For semantic replacement, `/ze-review` step 7 (removed-behavior audit)
must verify that every assertion the diff replaces still has coverage elsewhere.
When reviewing a test edit that changes WHAT is asserted (not just adding new
assertions), ask: "is the old behavior still tested?"

## Escape hatch (auditable)

When relaxation IS legitimate, document the reason on or above the changed line:

```
// test-relax: <why this test/assertion no longer applies>
```

The token unblocks the edit and leaves an audit trail. Review all relaxations with:
`grep -rn 'test-relax:' --include='*_test.go'`. Using the token without a real
reason is a violation, not a bypass.
