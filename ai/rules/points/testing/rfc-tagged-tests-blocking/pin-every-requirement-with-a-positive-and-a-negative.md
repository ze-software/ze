---
kind: directive
level: MUST
stage:
---
- **Every gated requirement MUST have BOTH a positive and a negative test, and the assertion MUST name the EXACT outcome rather than a floor.** A negative-only test passes when the code rejects everything and a positive-only test passes when it accepts everything, so only the pair pins behavior to the requirement. `GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it cannot fail when the implementation over-reacts.
