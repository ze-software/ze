---
kind: directive
level: MUST NOT
stage:
---
**VERIFICATION debt MAY be recorded. DEFECT debt MUST NOT be.** They are different
things and this rule bans only one of them. Verification debt is a gate that has not
yet run over code you believe is correct: a full `ze-precommit-verify` you have not
waited for, a review not yet done, an index left stale by a concurrent session. There
is nothing broken to fix, only a check owed. Its home is
`plan/verification-debt/<session>.md`, one row per owed gate, written by
`scripts/dev/commit_helper.py create` when you pass an override.
