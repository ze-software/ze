---
kind: directive
level: MUST NOT
stage:
---
**A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to
match the code.** It is the proof behind a public compliance claim in
`docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so
such an edit retires the evidence while the claim stays up.
