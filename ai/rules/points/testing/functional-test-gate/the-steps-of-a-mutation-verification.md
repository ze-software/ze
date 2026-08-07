---
kind: directive
level: MUST
stage:
---
1. Disable the producing function (the code the test exists to prove): an early
   `return`, a no-op, or `if true { return }` at the top of the function.
2. Re-run the test. It MUST flip to RED. If it still passes, the test does not gate
   on the feature: find the alternate delivery path and design it out (inject with no
   peers, remove the fallback store, use a genuinely-new peer instead of a reconnect),
   or the test is worthless: delete it, do not ship it.
3. Revert the mutation immediately and confirm the test is green again.
