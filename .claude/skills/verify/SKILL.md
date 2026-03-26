# Verify

Run `make ze-verify` and report results clearly.

## Steps

1. **Run verification:** Execute `make ze-verify` with 180s timeout, capturing output to `tmp/ze-test.log`.
2. **Parse results:** Search the log for failures:
   - `grep -E "^--- FAIL|^FAIL|TEST FAILURE|✗|═══ FAIL" tmp/ze-test.log`
   - Also check exit code
3. **Report** using this format:

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

4. **On failure:** Do NOT propose fixes automatically. Report all failures and ask the user how to proceed.

## Rules

- Do NOT fix anything. Report only.
- List EVERY failure. No omissions, no "and N more".
- Never say "pre-existing" or "unrelated" to justify ignoring a failure.
- The user decides what to do about failures.
