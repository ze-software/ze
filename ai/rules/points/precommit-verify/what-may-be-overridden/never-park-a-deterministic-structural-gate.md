---
kind: directive
level: MUST NOT
stage:
---
**The checklist's "write its spec, finish this commit, ask" branch and its
`plan/known-failures/` shard are for NON-DETERMINISTIC failures only**: flaky or
environmental TEST reds such as load-sensitive races, GC-pressure pool flakes,
and host-specific listener probes. **A deterministic structural gate is NEVER
eligible, MUST be fixed at the source before any commit, and MUST NOT be parked
or passed with `--unverified`.** Those gates (`./le verify lint run`, `./le
changed scope`, `./le tier check`, `./le verify deps evidence-vet`, `./le plugin
boundary check`, `./le iface-resolution`, `./le repository generated-check`,
`./le doc wiring`, `./le repository tracked-build check`) fail only when the tree
is structurally broken. Which stages carry that status is DERIVED rather than
listed: `docs/architecture/testing/verify-freshness-scope.md`.
