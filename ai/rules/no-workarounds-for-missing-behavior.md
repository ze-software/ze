# No Workarounds For Missing Behavior

**When:** If a user could experience a problem while trying to achieve a goal, implement the missing behavior at the source
**Severity:** blocking

## Directives

If a user could experience a problem while trying to achieve a goal, implement the missing behavior at the source. Do not bypass, mask, special-case, weaken a check, adjust a fixture, or route around the problem just to pass a test, demo, gate, or narrow scenario.

## Rule

A workaround is evidence that the feature, integration, validator, or test coverage is incomplete. The fix must make the user-visible goal work through the real entry point.

When tempted to work around a problem:

1. Name the user goal that should work.
2. Trace the code path that should provide it.
3. Implement the missing behavior at the owning layer.
4. Update affected callers and tests.
5. Verify the user-visible goal directly.

## Banned Fixes

| Banned | Why |
|--------|-----|
| Weakening or simplifying a test expectation | The test describes the required behavior. Broken code must change. |
| Special-casing only the failing fixture | Users can hit the same class of problem outside the fixture. |
| Skipping validation, errors, or unsupported inputs | Silent acceptance hides missing behavior and ships an operator trap. |
| Adding compatibility shims, aliases, or fallbacks instead of clean cutover | Ze has no released compatibility contract. Keep one real path. |
| Bypassing the owning layer from a caller | The next caller will fail the same way. Fix the owner. |
| Hiding a failure behind retries, sleeps, or broad catches | This masks the defect instead of proving the goal works. |

## Allowed Exception

A workaround is allowed only when the user explicitly asks for the workaround itself as the deliverable. In that case, name the limitation in the implementation notes and never present it as the real feature.

## Verification

Verification must exercise the user-visible goal, not just the workaround boundary. A unit test can prove internal logic, but the behavior is not complete until a functional, integration, or command-level check proves the user can reach the feature through the intended path.
