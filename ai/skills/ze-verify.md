---
name: ze-verify
description: Verify
---

# Verify

Run `./le verify worktree` and report results clearly.

See also: `/ze-debug` (investigate failures), `/ze-commit` (prepare commit after passing)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Use `subagent_type: ze-read`, which costs
  about 6k fewer startup tokens per agent than the default
  (`ai/rules/context-economy.md`). Do not run the steps below inline. You do
  not need to ask permission first (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
  Independent work goes out in ONE message with parallel `Agent` calls.
- **If you are that agent:** run the steps below. Resolve symbols with the LSP
  tool if your registry carries it and with `gopls` from Bash if it does not
  (`ai/rules/context-economy.md`). You cannot ask the user, so when you hit a
  STOP-and-ask condition, halt and put the question in your report for the main
  thread to carry.
- **Either way:** every claim in the report names the function that PRODUCES the
  behavior, as the file plus the symbol (`ai/rules/evidence.md`). The main
  thread verifies each one against source before acting; relaying a report
  unverified is fabrication with an extra hop. Report the conclusion and the
  evidence that would overturn it, never the search. Under 40 lines
  (`ai/rules/writing.md`).

## Steps

1. **Check freshness:** Run `./le verify status check`. A FRESH answer forbids
   rerunning the gate. A STALE answer names why verification is owed.
2. **Run verification:** Execute `./le verify worktree` in the foreground,
   giving the call the largest timeout the harness allows. Never poll or kill a
   slow live run. Native job admission queues or attaches equivalent work.
   `commit <revision>` selects a commit other than HEAD; `keep` retains a red
   worktree for inspection.
3. **Parse results:** On failure, search the log:
   - `grep -E "^--- FAIL|^FAIL|TEST FAILURE|✗|═══ FAIL" tmp/ze-verify.log`
   - Also check exit code
4. **Report** using this format:

   **On success:**
   ```
   ## Verify: PASS
   - Lint: pass
   - Unit tests: pass ([count] tests)
   - Functional tests: pass ([count] tests)
   - ExaBGP compat: pass
   ```

   **On failure:**
   ```
   ## Verify: FAIL

   ### Failures
   | # | Type | Test | Error |
   |---|------|------|-------|
   | 1 | unit | TestFoo | expected X, got Y |
   | 2 | lint | govet | file.go:42: shadowed variable |

   ### Passing
   - Lint: pass (if it passed)
   - Unit tests: N passed, M failed
   - Functional tests: pass/fail
   ```

5. **On failure:** Do NOT propose fixes automatically. Report all failures and ask the user how to proceed.

## Fallback

A slow run and admission contention are not failures. Fall back to individual
native stages only when the worktree action cannot run for an environmental
reason:
1. `./le verify lint run`
2. `./le test-unit all`
3. `./le functional gating`

Report whichever stages completed and which one stopped the run. This gives partial results instead of no results.

## Rules

- Do NOT fix anything. Report only.
- List EVERY failure. No omissions, no "and N more".
- Never say "pre-existing" or "unrelated" to justify ignoring a failure.
- The user decides what to do about failures.
- If another session is running verify, wait for it instead of starting a duplicate run.
