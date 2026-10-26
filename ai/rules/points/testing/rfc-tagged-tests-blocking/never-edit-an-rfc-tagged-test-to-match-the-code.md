---
kind: directive
level: MUST
stage:
---
- **A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to match the code.** It is the proof behind a public claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so the edit retires the evidence while the claim stays up. Fix your code instead.
- **A change to an RFC-tagged test MUST be approved by an independent agent running the `ze-test-judge` skill, which approves only when the old expectation is wrong against the RFC text it cites, and it MUST NOT be put to the owner.** The agent that made the change MUST NOT judge it. The judge's verdict MUST be recorded before the edit with `./le rfc approve unit <package>.<TestName> reason "<the judge's verdict>"`, which writes one row into the main thread's `tmp/commit-rfc-approved-<session>.md`; `writeWeakening` and the commit gate both read that file from disk, the commit carries each row it used as an `RFC-approved:` trailer line, and the generated script drops the used rows once the commit lands. That row also accepts a weakening of the same unit.
