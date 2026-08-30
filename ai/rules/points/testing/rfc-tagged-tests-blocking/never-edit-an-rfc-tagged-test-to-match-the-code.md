---
kind: directive
level: MUST
stage:
---
- **A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to match the code.** It is the proof behind a public claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so the edit retires the evidence while the claim stays up. Fix your code instead.
- **A row in `test/weakened.md` is your own justification and MUST NOT be read as approval here.** Once the user approves, what they approved MUST be written as one row in `test/rfc-changed.md` before the edit; `writeWeakening` and the commit gate both read that file from disk.
