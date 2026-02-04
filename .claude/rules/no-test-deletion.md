# Test Deletion Guidelines

Before deleting tests, ask for user approval unless the user has already explicitly requested the deletion.

## Why This Rule Exists

When tests fail, the temptation is to delete them and move on. This hides bugs instead of fixing them. But sometimes there are legitimate reasons to remove tests.

## When to Ask for Approval

Ask before:
- Deleting test files (`*_test.go`, `*.ci`)
- Removing `func Test*` / `func Fuzz*` / `func Benchmark*`
- Removing `t.Run()` subtests
- Removing table-driven test entries
- Removing assertions from a test
- Removing lines from `.ci` functional tests

**Exception:** If the user explicitly requested the deletion, proceed without asking again.

## Legitimate Reasons to Delete Tests

- Test was testing removed functionality
- Test was duplicating another test
- Test was fundamentally wrong (not just failing)
- Refactoring tests (replacing with better coverage)

## Not Legitimate Reasons

- Test is failing and hard to fix
- Test is slow
- Test is "annoying"
- Don't understand what test is checking

## The Question to Ask Yourself

"Am I deleting this test because it's wrong, or because it's inconvenient?"
