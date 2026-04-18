# Verify

Run `make ze-verify-fast` and report results clearly.

See also: `/ze-debug` (investigate failures), `/ze-commit` (prepare commit after passing)

## Steps

1. **Check for running verify:** If `tmp/.ze-verify.lock` exists and the PID inside is alive, another session is already running verify. Do NOT start a second run. Instead:
   - Report "ze-verify-fast already running (pid N), waiting for it to finish"
   - Wait for it to complete (the make target handles this automatically)
   - Read `tmp/ze-verify.log` for the results
2. **Run verification:** Execute `make ze-verify-fast` with 240s timeout. Output is auto-captured to `tmp/ze-verify.log`.
   - Custom log path: `make ze-verify-fast ZE_VERIFY_LOG=tmp/ze-verify-myname.log`
   - Race detector lives in `make ze-verify` (the full, sequential variant), NOT in fast. Run `make ze-verify` before commit to get -race coverage.
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

If `make ze-verify-fast` times out (240s), fall back to running stages separately:
1. `make ze-lint` (60s timeout)
2. `make ze-unit-test` (120s timeout)
3. `make ze-functional-test` (120s timeout)

Report whichever stages completed. Note which stage timed out. This gives partial results instead of no results.

## Rules

- Do NOT fix anything. Report only.
- List EVERY failure. No omissions, no "and N more".
- Never say "pre-existing" or "unrelated" to justify ignoring a failure.
- The user decides what to do about failures.
- If another session is running verify, wait for it instead of starting a duplicate run.
