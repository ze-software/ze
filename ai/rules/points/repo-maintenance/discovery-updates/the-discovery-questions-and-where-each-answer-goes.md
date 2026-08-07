---
kind: directive
level:
stage:
---
1. **Where would an agent look first?** Add or update the `ai/INDEX.md` keyword row, `ai/INDEX.md` task row, or both.
2. **What rule prevents regression?** Update the narrowest existing rule. Create a new `ai/rules/*.md` only when no existing rule owns the behavior.
3. **What source of truth prevents drift?** Use a registry, generated inventory, YANG schema, or live binary output. Do not copy static lists.
4. **What verification proves it?** Name the make target, unit test, functional test, hook, or doc validator that catches drift.
5. **What docs explain usage?** Name the exact file and section. Add source anchors for factual `docs/` claims.
6. **What learned record preserves the decision?** Update `ai/LEARNED-INDEX.md` if the learned summary changes future design choices.
