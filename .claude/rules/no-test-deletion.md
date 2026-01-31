# Test Deletion Requires Approval

**BLOCKING:** Test deletion requires explicit user approval.

## Why This Rule Exists

When tests fail, the temptation is to delete them and move on. This hides bugs instead of fixing them. But sometimes there are legitimate reasons to remove tests.

## What Triggers Approval Request

| Action | Triggers |
|--------|----------|
| Delete test files (`rm *_test.go`, `*.ci`) | ✅ |
| Delete `func Test*` / `func Fuzz*` / `func Benchmark*` | ✅ |
| Remove `t.Run()` subtests | ✅ |
| Remove table-driven test entries (`{name: "..."}`) | ✅ |
| Remove all assertions from a test | ✅ |
| Remove lines from `.ci` functional tests | ✅ |
| `git checkout --` to discard test changes | ✅ |

## Workflow

1. Claude attempts test deletion
2. Hook blocks and asks: "Allow this test deletion?"
3. User approves or denies
4. If approved, deletion proceeds

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
