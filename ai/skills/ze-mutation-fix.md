---
name: ze-mutation-fix
description: Fix Surviving Mutations
---

# Fix Surviving Mutations

Run mutation testing on a package, analyze surviving mutants, and either
strengthen tests or fix code bugs exposed by the mutations.

See also: `./le mutation` (report combination and history actions)

## Instructions

1. Accept `$ARGUMENTS` as the target package path (e.g., `./internal/core/textbuf/`).
   If empty, ask the user which package to target.

2. Run mutation testing with the pinned Go module tool:
   ```
   go run github.com/sivchari/gomu/cmd/gomu run \
     --workers 2 --timeout 120 --threshold 0 --output json \
     --incremental=false --fail-on-gate=false "$ARGUMENTS"
   ```

3. Read `mutation-report.json` and inspect the entries whose `status` is `SURVIVED`.

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
   - For a code bug that changes documented behavior: update the pages
     `ai/CODE-TO-DOCS.md` lists for that file in the same batch, never at the end
     (`ai/rules/documentation.md`)
   - Run `go test -race "$ARGUMENTS"` after each batch

6. Re-run the same `go run github.com/sivchari/gomu/cmd/gomu run ... "$ARGUMENTS"` command to measure improvement. Use `./le mutation record-history report mutation-report.json` when the run must enter the committed history.

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
