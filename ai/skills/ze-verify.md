---
name: ze-verify
description: Run the scoped tests that cover what changed, directly in the main checkout, and report. Runs in the main thread, never in an agent. Never runs any ./le verify command; GitHub CI verifies a pushed commit.
---

# Verify

Run the tests that cover what changed and report results clearly. No agent
runs any `./le verify` command, `./le verify changed` included: GitHub CI/CD
verifies a pushed commit, and the scoped test runs below, which the main
thread runs, are the evidence (`ai/rules/precommit-verify.md`).

| Mode | When | What runs |
|------|------|-----------|
| targeted (the only mode) | after a phase, a fix, a review round, or once the work is committed | the scoped package tests, component groups and functional suites for what changed, run directly in the main checkout |
| full | the caller asks for the release check | nothing: GitHub CI runs it on the pushed commit, and Thomas starts `./le verify current` himself when he chooses |

Every run happens in the main checkout, never in a worktree, an export or a
copy of the tree. When another session's in-progress edit breaks the build,
report what broke and whose it is, and wait for it to clear.

See also: `/ze-debug` (investigate failures), `/ze-commit` (prepare commit after passing)

## Delegation

This skill runs in the main thread, inline. The main thread schedules heavy
work on the shared machine and is the only caller allowed to start it, so no
agent runs these steps (`ai/rules/commands.md`).

- **If you are the main thread:** run the steps below yourself, one
  CPU-intensive job at a time. The commands come from the `Owed commands` lists
  your agents handed back and from step 2's selector.
- **If you are a subagent:** do not run this skill. The Bash hook refuses its
  commands from a subagent. List the exact commands as `Owed commands` in your
  report, each with what it proves and the result you expect.

## Steps

1. **Read `./le job report` and decide when to start.** It shows the job
   slots, who holds and who waits for them, the load, the available memory and
   the free disk. Start when a slot is free and memory is available. When the
   slots are full or a waiter is listed, do not queue: send your agents work
   that needs no run, and read the report again later. Never read a low load
   average alone as permission, because a run started at a low load has been
   killed for memory: read available memory beside it. Read the report again
   before each further job.
2. **Run the scoped tests** in the foreground, giving each call the largest
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
   - **Cost (targeted only):** after the scoped runs, run
     `./le test cost report suite <suite>` for each functional suite it ran,
     and `./le test cost report log <path>` for each package run whose log
     kept per-test elapsed (`go test -v`). The report ends with each listed
     row as a `NOTE:` line, never as a failure: a row is a candidate to read
     against `ai/rules/testing.md`, "one question per test". A full-mode run
     ran nothing, so it prints none.
3. **Parse results:** On failure, search each run's output:
   - `grep -E "^--- FAIL|^FAIL|TEST FAILURE|✗|═══ FAIL"` over the output
   - Also check exit code
4. **Report** using this format:

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

5. **On failure:** Do NOT propose fixes automatically. Report all failures and ask the user how to proceed.

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
