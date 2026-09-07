---
kind: directive
level: MUST NOT
stage:
---
**A file carrying another session's hunks MUST NOT be left out of your commit unless you have SHOWN that session is still running, and the check is whether its `tmp/session/<date>-<id>/` directory is still being written.** Leaving it out is a claim about a future you did not verify: that somebody else will commit it. Two sessions reading "leave another session's work alone" both defer, and the change then belongs to nobody and reaches no commit. Where the holder is gone, the work LANDS, under a subject that says whose hunks it carries; where you cannot tell whose it is, ask the owner rather than deferring by default. Deferring is correct only while a live session is going to commit it, and "modified by someone else" is the reason to look, never the answer.
