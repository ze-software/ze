---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/no-partial-completion.md
---
**You MUST NOT claim work is done, complete, ready to commit, or ready for review while any in-scope acceptance criterion remains unimplemented, and the claim has no synonyms.** "Deferred", "tracked in a shard", "all core functionality implemented", "the remaining items are minor" and "will be handled in a follow-up" each name work that is not done.
**You MUST have READ the diff, hunk by hunk, before the claim.** A gate covers what somebody thought to check, so a green run over an unread diff is neither done nor green: say what you have, which is that the gates pass and you have not read the change.
