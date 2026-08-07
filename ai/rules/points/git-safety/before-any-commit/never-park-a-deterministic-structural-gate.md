---
kind: note
level:
stage:
---
The item-2 "log to `plan/known-failures/`" path is for **non-deterministic**
failures only -- flaky or environmental TEST reds (load-sensitive races,
GC-pressure pool flakes, host-specific listener probes). A **deterministic
structural gate** is NEVER eligible: `ze-lint`, `ze-lint-changed`, `ze-tier-check`,
`ze-vet-evidence`, `ze-plugin-boundary-check`, `ze-iface-resolution-check`,
`ze-regen-check-readonly`, `ze-verify-wiring-docs`, and `ze-tracked-build-check`
fail only when the tree is structurally broken (a misplaced module tier, a
lint/vet violation, a broken plugin boundary, an unresolved iface, a stale
generated file, a stale wiring index, a HEAD that does not compile). Such a red
must be fixed at the source before any commit -- do not park it, do not
`--unverified` past it.
