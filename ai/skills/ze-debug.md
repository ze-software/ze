---
name: ze-debug
description: Debug Failing Tests
---

# Debug Failing Tests

Investigate and fix test failures using parallel hypotheses.

The user will paste failing test output as context.

See also: `/ze-verify` (re-verify after fix)

## Delegation

`ai/rules/planning.md`: this skill runs in the MAIN THREAD and does its
own fan-out. Do not wrap the whole skill in a single agent. That buries the
parallel lenses one level down and costs exactly the independence they exist to
provide (`ai/rules/planning.md`).

Launch the agents this skill defines, all in ONE message, on `model: opus`,
with `subagent_type: ze-work`. Every lens here implements its own fix at step
4, so none of them can be `ze-read`, which holds no Edit. `ze-work` costs about
6k fewer startup tokens per agent than the default
(`ai/rules/context-economy.md`).
Never trade their model down for cost; cut their NUMBER instead
(`ai/rules/planning.md`). You do not need to ask permission to spawn them
(`ai/INSTRUCTIONS.md`, STANDING REQUEST).

## Steps

1. **Read the failing test output** provided by the user
2. **Identify the failing tests:** Extract test names, packages, error messages, and expected vs actual values
3. **Read the page before you spawn anything (BLOCKING, `ai/rules/documentation.md`):** look up each failing file in `ai/CODE-TO-DOCS.md`. Read the pages it lists. Read the file's `// Design:` header. Put those page paths in every agent prompt below. Say what each page claimed. A page that contradicts the failure is the bug or the defect. Step 5 decides which.
4. **Launch 4 parallel investigation agents** (use `model: opus` -- diagnosis is judgment work, see `ai/rules/planning.md`):

   **Task 1 -- Format/parsing mismatch:**
   Check test expectations against actual output formats. Are the tests expecting a different structure, field name, or encoding than what the code produces?

   **Task 2 -- Data flow issue:**
   Trace the data from source through each transformation. Is data being lost, corrupted, or transformed incorrectly between layers?

   **Task 3 -- Configuration/initialization issue:**
   Check setup, defaults, and initialization order. Are dependencies missing, nil, or initialized in the wrong sequence?

   **Task 4 -- Concurrency/wiring issue:**
   Check for race conditions, nil pointer from uninitialized dependencies, wrong production path (grep for ALL implementations of the handler -- the test may call a different one than production), and plugin wiring gaps (feature implemented but not reachable from its entry point).

5. **Each task must DIAGNOSE before it fixes** (per `ai/rules/completion.md`):
   - Read the relevant source code.
   - Produce a Diagnosis: **symptom**, **root cause traced to the exact function** where behavior diverges from intent (cite it — no guessing), the **owning layer**, **two candidate fixes labeled `[workaround]` vs `[source]`**, and one line on **why the workaround is wrong**.
   - If the failure is a check/validation rejecting the input, answer the three-way question: is the check wrong, is the input wrong, or is the check's data/config incomplete?
   - **Only then** implement the `[source]` fix at the owning layer.
   - Run `go test ./...` to verify the specific fix.
6. **Confirm the fix is at the source, not the symptom:** before accepting any fix, re-read its Diagnosis. If the change makes the test pass by editing the test, renaming a symbol, or special-casing the failing input rather than correcting the traced root cause, reject it and return to step 5. Changing a test to match broken code is never the fix (`ai/rules/testing.md`).
7. **Update the pages the fix made wrong, in this diff (BLOCKING, `ai/rules/documentation.md`):** the pages are the ones step 3 read. Two things get repaired here. A page the fix made wrong, and a page step 3 found wrong about the code. Neither waits for closure.
8. **Run full verification:** `./le verify lint run && ./le test-unit all && ./le functional gating` -- the fix must not break anything else
9. **Report back** on each fixed failure. Give the Diagnosis (symptom, root-cause function, owning layer). Give the correct hypothesis, the `[source]` fix chosen over the `[workaround]`, the page updated, and the full suite green

## Fallback

If all 4 hypotheses fail to explain the failure, report:
- What each hypothesis investigated
- What was ruled out
- Any clues discovered
Then ask the user for guidance (3-fix rule: 3 failed approaches -> stop and ask).

## Rules

- **Diagnosis before fix (BLOCKING):** No edit until the five-part Diagnosis is written (`ai/rules/completion.md`). The success criterion is "root cause fixed at the owning layer", not "test green". Trigger words ("let me just rename / skip / special-case / adjust the test") mean stop and diagnose.
- Do NOT modify test expectations unless the tests are genuinely wrong per the spec
- If multiple hypotheses find real issues, fix all of them
- If none of the 4 hypotheses explain the failure, report what was ruled out and ask the user for guidance
