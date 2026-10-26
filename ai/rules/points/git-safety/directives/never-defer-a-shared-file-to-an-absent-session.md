---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/git-safety.md
---
**A file carrying another session's hunks MUST NOT be left out of your commit unless the foreign hunk can change behavior or make a statement false, judged against HEAD.** Landing is the presumption, because withholding is the option that breaks something: your own change does not work without the file, while a foreign hunk that adds a function, adds a stage whose action HEAD already carries, or corrects prose costs the tree nothing. Carry it, and say in the subject or body whose hunks rode along.

**The judgement is against HEAD, never against the working tree.** A hunk is additive and still unsafe when what it names is not committed: a stage whose action is not in HEAD reddens every verify, and a page describing a producer that is not in HEAD publishes a claim the tree does not support. Read the referent out of HEAD before you decide, and hold the file back only when it is missing.

**Where the foreign hunk is unsafe to carry and your own change needs the file, you MUST commit your hunks alone with `hunks <path> <patch-file>`, and MUST NOT withhold the file or take the foreign hunks out of the working tree.** Save `git diff -- <path>`, delete the foreign hunks from the patch, and pass it: the commit carries HEAD's blob plus your hunks, the content gates and the review pin judge that blob, and the working-tree file keeps both sessions' work. The verification state still judges the working-tree file, and the page names each judgement (`docs/contributing/committing.md`, "Committing some hunks of a shared file").

**Where the hunk is unsafe to carry, deferring is a claim about a future you MUST verify: that somebody else will commit it.** Two sessions both reading "leave another session's work alone" defer on the same file, and the change then belongs to nobody. Show the holder is still running, by its `tmp/session/<date>-<id>/` directory still being written. Where the holder is gone, the work lands anyway; where the hunks trace to nobody, ask the owner. "Modified by someone else" is the reason to look, never the answer.
