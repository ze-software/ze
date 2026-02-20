# No Test Deletion Rationale

Why: `.claude/rules/no-test-deletion.md`

## Why This Rule Exists
When tests fail, the temptation is to delete them and move on. This hides bugs instead of fixing them.

## The Question to Ask Yourself
"Am I deleting this test because it's wrong, or because it's inconvenient?"

## When to Ask for Approval (specific items)
- Deleting test files (`*_test.go`, `*.ci`)
- Removing `func Test*` / `func Fuzz*` / `func Benchmark*`
- Removing `t.Run()` subtests
- Removing table-driven test entries
- Removing assertions from a test
- Removing lines from `.ci` functional tests

## Legitimate vs Illegitimate Reasons

| Legitimate | Illegitimate |
|-----------|-------------|
| Testing removed functionality | Test is failing and hard to fix |
| Duplicating another test | Test is slow |
| Fundamentally wrong (not just failing) | Test is "annoying" |
| Replacing with better coverage | Don't understand what test checks |
