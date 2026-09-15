---
kind: directive
level: MUST
stage:
---
- **A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to match the code.** It is the proof behind a public claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so the edit retires the evidence while the claim stays up. Fix your code instead.
- **A weakening row is your own justification and MUST NOT be read as approval here.** Once the user approves, what they approved MUST be recorded before the edit with `./le rfc approve unit <package>.<TestName> reason "<the owner's words>"`, which writes one row into this commit session's `tmp/commit-rfc-approved-<session>.md`; `writeWeakening` and the commit gate both read that file from disk, the commit carries each row it used as an `RFC-approved:` trailer line, and the generated script drops the used rows once the commit lands.
