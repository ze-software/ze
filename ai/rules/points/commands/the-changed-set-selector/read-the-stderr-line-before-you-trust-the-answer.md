---
kind: directive
level: MUST
stage:
---
**You MUST read the selector's stderr before you trust a scoped run.** It widens
to `./...` and names the reason whenever it cannot narrow, and one reason is
routine: `tmp/ze-verify.status` holding no green commit. With nothing proven,
every scoped target judges the whole tree until a full run passes. The contract
is `docs/architecture/testing/verify-freshness-scope.md`.
