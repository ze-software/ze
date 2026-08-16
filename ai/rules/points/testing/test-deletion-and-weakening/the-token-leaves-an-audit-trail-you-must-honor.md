---
kind: note
level:
stage:
---
The row unblocks the edit and leaves an audit trail. `test/weakened.md` is
replaced per commit and never accumulates, so the trail lives in git history:
`git log -p -- test/weakened.md` shows the rows of any commit beside the change
they accepted. `scripts/dev/commit_helper.py` refuses a commit that weakens a
test and does not carry the file, so no row is left behind in the working tree.
Writing a row without a real reason is a violation.
