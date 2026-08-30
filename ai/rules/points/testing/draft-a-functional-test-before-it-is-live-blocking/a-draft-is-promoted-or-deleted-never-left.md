---
kind: directive
level: MUST
stage:
---
**A draft is not a test yet, so the draft workflow MUST end in exactly two moves:
promote it into `test/<suite>/`, or delete it.** Leaving it in the incubator is
the third move, and it is the one that is refused. A draft proves no obligation,
claims no evidence, and appears in no coverage ledger, so a session that finds
one cannot tell abandoned scaffolding from work in progress.

**Deleting a draft needs no approval, and a command that mixes a draft with a
LIVE test is still refused.** The guards that protect tests do not protect
drafts: `bashTestDeletion` and `writeWeakening`
(`internal/le/hookruntime`) both exempt a path under the incubator, and only a
path under it.
**An `RFC requirement:` tag inside a draft is worth nothing until the file is
live**, which is why the tag guard skips it. Promoting the file is what turns the
tag into proof.
