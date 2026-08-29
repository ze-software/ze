---
kind: directive
level: MUST NOT
stage:
---
- `spec-fixit-redistribute-establishment-stall` -- the redistribute establishment sleeps MUST NOT be converted until this P0 spec lands. It landed on 2026-08-23 in commit `8f3a80bf9`, which closed the spec and deleted it from `plan/`.
- The external-plugin refuse/warn sleeps wait on a reject-fence signal the daemon does not emit, so no deterministic wait exists for them yet.
