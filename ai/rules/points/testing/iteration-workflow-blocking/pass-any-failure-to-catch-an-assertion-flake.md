---
kind: directive
level: MUST
stage:
---
- **A crash is not the only reproduction, so `./le stress-repro run` MUST carry its `any-failure` keyword for a load-dependent failure that is not a crash.** By default only a crash signature (panic, `DATA RACE`, runtime error) counts and everything else is discarded down to the last 500 bytes, so an assertion flake exits non-zero, matches nothing, and the run reports "not reproduced" while throwing the evidence away.
