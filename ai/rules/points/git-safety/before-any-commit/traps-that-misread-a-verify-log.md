---
kind: directive
level:
stage:
---
- **`tail` on the log of a run that is still going.** The stage banner tells you
  where it is (`### Stage 18/22`). Check `scripts/dev/verify-status.sh check` for
  the verdict instead: it says FRESH, or names the failure and its time.
- **Grepping for `--- FAIL` only.** Lint, tier, doc and inventory stages fail with
  their own wording and no `FAIL` token, so a test-shaped grep reads a red lint
  stage as a clean run. Read the summary block, not a pattern you chose.
