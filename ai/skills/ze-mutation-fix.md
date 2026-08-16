---
name: ze-mutation-fix
description: Fix Surviving Mutations
---

# Fix Surviving Mutations

Run mutation testing on a package, analyze surviving mutants, and either
strengthen tests or fix code bugs exposed by the mutations.

See also: `make ze-mutation-pkg-test PKG=./path/` (raw mutation run)

## Instructions

1. Accept `$ARGUMENTS` as the target package path (e.g., `./internal/core/textbuf/`).
   If empty, ask the user which package to target.

2. Run mutation testing:
   ```
   make ze-mutation-pkg-test PKG=$ARGUMENTS
   ```

3. Parse `tmp/mutation-report.json` for surviving mutations:
   ```python
   [r for r in data['results'] if r['status'] == 'SURVIVED']
   ```

4. For each surviving mutation, read the source file at the indicated line.
   Classify the mutation as one of:

   | Classification | Action |
   |---|---|
   | **Test gap** | The code is correct but tests don't assert this behavior. Write a test that kills the mutant. |
   | **Code bug** | The mutation exposes dead code, unreachable branches, or incorrect logic. Fix the code. |
   | **Equivalent mutant** | The mutation produces identical behavior (e.g., `x * 1` -> `x + 1` when x is always 1). Skip it. |
   | **Acceptable risk** | The mutation is in error-handling or defensive code where testing every boundary isn't worth the cost. Skip with a note. |

5. Implement fixes in batches by file:
   - For test gaps: write the minimal test that fails with the mutation applied
   - For code bugs: fix the code, then verify existing tests catch the fix
   - Run `go test -race ./PKG/...` after each batch

6. Re-run gomu on the same package to measure improvement:
   ```
   make ze-mutation-pkg-test PKG=$ARGUMENTS
   ```

7. Report the before/after mutation score and list what was fixed vs skipped.

## Classification Guidance

**Test gap signals:**
- Branch condition mutations survive (`> -> >=`, `< -> <=`) -- boundary not tested
- Return value mutations survive (`true -> false`) -- return value not asserted
- Arithmetic mutations survive on sizing/capacity code -- exact math not tested

**Code bug signals:**
- A mutation that replaces a condition with `true` or `false` survives and the code still passes -- the condition might be dead
- A mutation that removes an operation survives -- the operation might have no effect

**Equivalent mutant signals:**
- `x + 0` -> `x - 0` (both are no-ops)
- Mutations on unreachable code (behind a condition already tested as always-true)
- Mutations on code that only affects logging or debug output

## Rules

- Do NOT chase 100% mutation score. Focus on mutations that reveal real test or code quality gaps.
- Prefer adding tests over changing code. Most surviving mutations are test gaps, not bugs.
- Each new test must fail when the specific mutation is manually applied (verify the test actually targets the mutant).
- Follow `ai/rules/testing.md` for test placement and style.
- Skip equivalent mutants without guilt -- they are a known limitation of mutation testing.
- Report the final score and stop. Do not iterate indefinitely.
