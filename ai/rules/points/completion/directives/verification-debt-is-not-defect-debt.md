---
kind: directive
level: MUST NOT
stage:
---
**VERIFICATION debt MAY be recorded. DEFECT debt MUST NOT be.** Verification debt is a gate that has not yet run over code you believe correct, or one red on another session's uncommitted work: nothing is broken, only a check is owed. A gate that RAN and went red on YOUR code is a defect, and so is behavior an acceptance criterion requires and nothing implements; neither is recordable.
**The override keywords on `./le commit create` are SELF-SERVICE, and you MUST NOT stop to ask Thomas before using one.** `unverified`, `structural-red-ok`, `missing-full-verify-ok`, `stale-index-ok` and `review-override` each take a truthful reason, admit one unrun gate, and write a row in `plan/verification-debt/<session>.md`.
**Enforcement is at the PUSH, where code reaches users: `create push <remote>` refuses while any row is open, and `./le commit debt-clear` re-runs each owed gate once and writes `cleared` only where it exits 0.**
