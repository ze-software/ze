---
name: ze-verify
description: Verify
---

# Verify

Run the tests that cover what changed and report results clearly. A full
verification is Thomas's release check, never run here (below).

| Mode | When | What runs |
|------|------|-----------|
| targeted (the default) | during development: after a phase, a fix or a review round | the scoped package tests, component groups and functional suites for what changed |
| scoped | the work is committed (for a spec, after `/ze-close` landed commit A), or a push is owed | `./le verify changed` |
| full | the caller asks for the release check | nothing: give Thomas the exact command, `./le verify current` or `./le verify worktree commit <revision>` |

A full verification refuses an agent: it loads the whole shared machine, and
only Thomas knows when that is safe (`docs/contributing/running-commands.md`,
"A full verification is Thomas's to start").

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
2. **Run verification** in the foreground, giving each call the largest
   timeout the harness allows. Never poll or kill a slow live run. Native job
   admission queues or attaches equivalent work.
   - **Targeted:** run `./le repo changed scope print both` to name the changed
     packages, their importers and the feature tags they reach
     (`docs/contributing/running-commands.md`, "The changed-set selector").
     Run the scoped package tests under `./le job run`, the component group
     for each package (`./le test unit <group>`), and the functional suite for
     each behavior the change touched (`./le test functional <suite>`), picking
     the rungs of `docs/contributing/testing.md`, "The escalation ladder".
     Do not widen to `./le test unit all`, `./le test functional gating` or a
     full verification.
   - **Scoped:** run `./le verify changed`, which judges the change set since
     the upstream in a fresh worktree at HEAD; `commit <revision>` names
     another commit. Its record is what a push owes.
   - **Full:** run nothing. Hand Thomas the exact command to run at his
     terminal: `./le verify current`, or `./le verify worktree commit
     <revision>` for a commit other than HEAD.
3. **Parse results:** On failure, search the log:
   - `grep -E "^--- FAIL|^FAIL|TEST FAILURE|✗|═══ FAIL" tmp/ze-verify.log`
   - Also check exit code
4. **Report** using this format:

   **On success:**
   ```
   ## Verify (targeted|full): PASS
   - Lint: pass
   - Unit tests: pass ([count] tests)
   - Functional tests: pass ([count] tests)
   - ExaBGP compat: pass
   ```

   **On failure:**
   ```
   ## Verify (targeted|full): FAIL

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

A slow run and admission contention are not failures. Full mode runs nothing,
so it has no fallback: never substitute `./le go lint run`, `./le test unit all`
or `./le test functional gating` for the release check, because each loads the
whole shared machine the same way. In scoped mode, when `./le verify changed`
cannot run for an environmental reason, fall back to the targeted runs of step
2 for what changed.

Report whichever stages completed and which one stopped the run. This gives partial results instead of no results.

## Rules

- Do NOT fix anything. Report only.
- List EVERY failure. No omissions, no "and N more".
- Never say "pre-existing" or "unrelated" to justify ignoring a failure.
- The user decides what to do about failures.
- If another session is running verify, wait for it instead of starting a duplicate run.
