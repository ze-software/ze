---
kind: note
level:
stage:
---
Every gated requirement needs BOTH a positive and a negative test. A negative-only test
passes if the code rejects everything; a positive-only test passes if it accepts
everything. Only the pair pins behavior to the requirement. Assert the EXACT outcome, never
a floor: `GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it cannot
fail when the implementation over-reacts. See `ai/skills/ze-rfc.md`.
