---
name: ze-verify
description: Run the scoped tests that cover what changed, directly in the main checkout, and report. Never runs any ./le verify command; GitHub CI verifies a pushed commit.
---

# Verify

Run the tests that cover what changed and report results clearly. No agent
runs any `./le verify` command, `./le verify changed` included: GitHub CI/CD
verifies a pushed commit, and the scoped test runs below are the agent's
evidence (`ai/rules/precommit-verify.md`).

| Mode | When | What runs |
|------|------|-----------|
| targeted (the only mode) | after a phase, a fix, a review round, or once the work is committed | the scoped package tests, component groups and functional suites for what changed, run directly in the main checkout |
| full | the caller asks for the release check | nothing: GitHub CI runs it on the pushed commit, and Thomas starts `./le verify current` himself when he chooses |

Every run happens in the main checkout, never in a worktree, an export or a
copy of the tree. When another session's in-progress edit breaks the build,
report what broke and whose it is, and wait for it to clear.

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

1. **Run the scoped tests** in the foreground, giving each call the largest
   timeout the harness allows. Never poll or kill a slow live run. Native job
   admission queues or attaches equivalent work.
   - **Targeted:** run `./le repo changed scope print both` to name the changed
     packages, their importers and the feature tags they reach
     (`docs/contributing/running-commands.md`, "The changed-set selector").
     Run the scoped package tests under `./le job run`, the component group
     for each package (`./le test unit <group>`), and the functional suite for
     each behavior the change touched (`./le test functional <suite>`), picking
     the rungs of `docs/contributing/testing.md`, "The escalation ladder".
     Do not widen to `./le test unit all`, `./le test functional gating` or
     any `./le verify` command.
   - **Full:** run nothing. GitHub CI verifies the pushed commit.
2. **Parse results:** On failure, search each run's output:
   - `grep -E "^--- FAIL|^FAIL|TEST FAILURE|✗|═══ FAIL"` over the output
   - Also check exit code
3. **Report** using this format:

   **On success:**
   ```
   ## Verify (targeted): PASS
   - Unit tests: pass ([count] tests, packages named)
   - Functional tests: pass ([count] tests, suites named)
   ```

   **On failure:**
   ```
   ## Verify (targeted): FAIL

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

4. **On failure:** Do NOT propose fixes automatically. Report all failures and ask the user how to proceed.

## Fallback

A slow run and admission contention are not failures. Full mode runs nothing,
so it has no fallback: never substitute `./le go lint run`, `./le test unit all`
or `./le test functional gating` for the release check, because each loads the
whole shared machine the same way. When a targeted run cannot build because
another session's edit broke the tree, report it and wait; never move the run
to a copy of the tree.

Report whichever stages completed and which one stopped the run. This gives partial results instead of no results.

## Rules

- Do NOT fix anything. Report only.
- List EVERY failure. No omissions, no "and N more".
- Never say "pre-existing" or "unrelated" to justify ignoring a failure.
- The user decides what to do about failures.
- If another session is running the same tests, wait for it instead of starting a duplicate run.

## Done when

Done when the `## Verify` report shows PASS, or FAIL with its failures table, and on a failure the user is asked how to proceed.
