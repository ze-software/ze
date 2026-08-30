---
kind: directive
level: MUST
stage:
---
**Every gated requirement MUST have BOTH a positive and a negative test.** A
negative-only test passes when the code rejects everything, and a positive-only
test passes when it accepts everything; only the pair pins behavior to the
requirement.
**The assertion MUST name the EXACT outcome and MUST NOT assert a floor.**
`GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it
cannot fail when the implementation over-reacts.
