---
kind: directive
level: MUST NOT
stage:
---
**A file carrying another session's hunks MUST NOT be left out of your commit unless the foreign hunk can change behavior or make a statement false, judged against HEAD (owner directive, 2026-09-07).** Landing is the presumption, because withholding is the option that breaks something: your own change does not work without the file, while a foreign hunk that adds a function, adds a stage whose action HEAD already carries, or corrects prose costs the tree nothing. Carry it, and say in the subject or body whose hunks rode along.

**The judgement is against HEAD, never against the working tree.** A hunk is additive and still unsafe when what it NAMES is not committed: a stage whose action is not in HEAD reddens every verify, and a page describing a producer that is not in HEAD publishes a claim the tree does not support. Read the referent out of HEAD before you decide, and hold the file back only when it is missing.

**Where the hunk is unsafe to carry, deferring is a claim about a future you MUST verify: that somebody else will commit it.** Two sessions both reading "leave another session's work alone" defer on the same file, and the change then belongs to nobody. Show the holder is still running, by its `tmp/session/<date>-<id>/` directory still being written. Where the holder is gone, the work LANDS anyway; where the hunks trace to nobody, ask the owner. "Modified by someone else" is the reason to look, never the answer.
