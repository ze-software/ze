---
kind: directive
level: MUST
stage:
---
- **`tail` on the log of a run that is still going.** The stage banner tells you
  where it is (`### Stage 18/22`). You MUST check `./le verify status check` for
  the verdict instead: it says FRESH, or names the failure and its time.
- **Grepping for `--- FAIL` only.** Lint, tier, doc and inventory stages fail with
  their own wording and no `FAIL` token, so a test-shaped grep reads a red lint
  stage as a clean run. You MUST read the summary block, not a pattern you chose.
