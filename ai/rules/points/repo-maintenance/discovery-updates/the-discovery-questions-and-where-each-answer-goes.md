---
kind: directive
level: MUST
stage:
---
1. **Where would an agent look first?** The `ai/INDEX.md` keyword row, the `ai/INDEX.md` task row, or both MUST be added or updated.
2. **What rule prevents regression?** The narrowest existing rule MUST be updated. A new `ai/rules/*.md` MAY be created only when no existing rule owns the behavior.
3. **What source of truth prevents drift?** A registry, generated inventory, YANG schema, or live binary output MUST be used. A static list MUST NOT be copied.
4. **What verification proves it?** The make target, unit test, functional test, hook, or doc validator that catches drift MUST be named.
5. **What docs explain usage?** The exact file and section MUST be named. Source anchors MUST be added for factual `docs/` claims.
6. **What journal record preserves the decision?** A row MUST be appended to the matching `plan/journal/<class>.md` when a recurring trap was hit.
