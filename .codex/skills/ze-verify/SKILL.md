---
name: ze-verify
description: Use when working in the Ze repo and the user asks for ze-verify or wants the standard verification run. Respect the repo lock file, run make ze-verify-fast in the foreground, fall back to split stages on timeout, and report every failure without proposing fixes automatically.
---

# Ze Verify

This skill runs the Ze verification workflow and reports the result clearly.

## Workflow

1. If `tmp/.ze-verify.lock` is active, wait for the running verification and then read `tmp/ze-verify.log`.
2. Otherwise run `make ze-verify-fast` in the foreground with a timeout and let it write the shared log.
3. If it times out, fall back to `make ze-lint`, `make ze-unit-test`, and `make ze-functional-test` as separate stages.
4. Parse the log and report:
   - full pass summary, or
   - every failure with type, test, and error

## Rules

- Do not fix anything automatically.
- Do not hide or collapse failures.
- Prefer the repo log file over memory when summarizing the result.
