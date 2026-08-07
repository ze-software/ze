---
kind: directive
level:
stage:
---
**Sleep ratchet (BLOCKING):** the total `time.sleep(` count across
`test/**/*.ci` may only go down. The committed baseline lives in
`test/.ci-sleep-baseline`; `make ze-verify-wiring-docs` fails when the count
exceeds it. Use `ze_api` `wait_for_event` / `wait_for_shutdown` / `wait_until` /
`dispatch_until` (the payload-predicate waits, below) instead of sleeps (sleeps
hide real races). When your change removes sleeps, lower the baseline in the same
change. Known violations are tracked in `plan/known-failures/`
and must be migrated.
