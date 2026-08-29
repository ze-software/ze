---
kind: note
level:
stage:
---
The pre-commit checklist's "write its spec, finish this commit, ask" branch, and
its `plan/known-failures/` shard, are for **non-deterministic** failures only.
Those are flaky or environmental TEST reds: load-sensitive races, GC-pressure pool
flakes, host-specific listener probes ("Reading A Red", above). A **deterministic
structural gate** is NEVER eligible: `./le verify lint run`, `./le changed scope`,
`./le tier check`, `./le verify deps evidence-vet`, `./le plugin boundary check`,
`./le iface-resolution`, `./le repository generated-check`, `./le doc wiring`,
and `./le repository tracked-build check`
fail only when the tree is structurally broken (a misplaced module tier, a
lint/vet violation, a broken plugin boundary, an unresolved iface, a stale
generated file, a stale wiring index, a HEAD that does not compile). Such a red
must be fixed at the source before any commit -- do not park it, do not
`--unverified` past it.
