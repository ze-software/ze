---
kind: directive
level: MUST
stage:
---
**A crash is not the only reproduction.** By default only a CRASH signature
(panic / `DATA RACE` / runtime error) counts, and everything else is discarded
down to the last 500 bytes. An assertion flake (a test whose `expect=` pattern
is merely missed under load) exits non-zero with no crash signature, so
`--any-failure` MUST be passed, or the run reports "not reproduced" while quietly throwing the
evidence away.
