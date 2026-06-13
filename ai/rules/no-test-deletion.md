# Test Deletion and Weakening

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

## Escape hatch (auditable)

When relaxation IS legitimate, document the reason on or above the changed line:

```
// test-relax: <why this test/assertion no longer applies>
```

The token unblocks the edit and leaves an audit trail. Review all relaxations with:
`grep -rn 'test-relax:' --include='*_test.go'`. Using the token without a real
reason is a violation, not a bypass.
