---
kind: directive
level: MUST
stage:
---
1. **Where would an agent look first?** The `ai/INDEX.md` keyword row, the `ai/INDEX.md` task row, or both MUST be added or updated.
2. **What rule or gate prevents regression?** Name the current rule or gate when one covers the behavior. Update it when this change makes it wrong. A NEW `ai/rules/*.md` MUST wait for a recurrence that exposes a missing instruction no current rule or gate gives.
3. **What source of truth prevents drift?** A registry, generated inventory, YANG schema, or live binary output MUST be used. A static list MUST NOT be copied.
4. **What verification proves it?** The native action, unit test, functional test, hook, or doc validator that catches drift MUST be named.
5. **What docs explain usage?** The exact file and section MUST be named. Source anchors MUST be added for factual `docs/` claims.
6. **What journal record preserves the decision?** A row MUST first be appended to the matching `plan/journal/<class>.md` when a recurring trap is hit. The row is the record, never the fix: a blocking or related defect MUST still be fixed (`ai/rules/completion.md`).
