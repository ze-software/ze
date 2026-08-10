---
kind: directive
level: MAY
stage:
---
- An assumption without a validation method is a guess. Name the test, grep, or user confirmation that would settle it.
- A `broken` assumption gets a Mistake Log "Wrong Assumptions" row and a Deviations entry. If it invalidates the approved design, STOP and present to the user.
- No assumption MAY still be `unvalidated` at Pre-Commit Verification (the spec's "Assumptions Resolved" table records final status with evidence).
- Existing specs (created before this rule) are exempt; do not retrofit without user request.
