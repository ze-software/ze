---
name: Prefer multiple focused commits
description: Break work into logical commits rather than bundling everything into one
type: feedback
---

Break work into multiple focused commits when changes are logically separate. Don't bundle unrelated fixes, rules, docs, and learned summaries into a single commit.

**Why:** User explicitly corrected the one-big-commit pattern. Separate commits make history readable and reversible.

**How to apply:** After completing work, group changes by logical unit (bug fix, rule addition, doc update) and commit each separately. A bug fix + its test fix is one commit. A new rule file is another. A learned summary is another.
