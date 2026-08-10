---
kind: directive
level: MUST
stage:
---
- **Tests MUST exist and fail before implementation.**
- **Every user-facing behavior MUST have a functional test that exercises it through a user entry point. Unit tests (`_test.go`) prove internal logic. Functional tests (`.ci`, `.et`) prove the feature works end-to-end through the daemon. Both are required. Neither substitutes for the other.**
- **A red test means the CODE is wrong by default. MUST diagnose the failure and fix the source. MUST NOT weaken the test to make it green. MUST ask the user before deleting OR weakening any test code (`*_test.go`, `.ci`, `Test*`, `t.Run`, assertions, table entries). Exception: the user already explicitly requested it.**
- **A test that cannot run on every OS MUST either carry a build tag (`//go:build linux`) on its file, or skip (`t.Skip`) with a reason on the OSes where it cannot run. MUST NOT weaken the assertion to accept both outcomes.**
- **Every `time.sleep(` call in a `.ci` test MUST have an explanatory comment on the line directly above it, or trailing it on the same line. A bare sleep with no comment is rejected.**
- **A load-dependent failure is DIAGNOSED, and the outcome is always a fix. Load-dependence is the diagnosis (the test asserts on elapsed time instead of on state), and `ai/rules/completion.md` bans recording it as a `plan/known-failures/` shard, bans "passes in isolation" as a conclusion, and bans raising the timeout. MUST reproduce it with the stress reproducer, then fix the timing assumption.**
