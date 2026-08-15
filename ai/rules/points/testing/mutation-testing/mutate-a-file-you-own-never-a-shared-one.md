---
kind: directive
level: MUST
stage:
---
**Proving a test discriminates means breaking the mechanism on purpose, and in a
shared checkout that mutation MUST land in a file this session owns.** Every
other session reads the same working tree. A file under mutation is a file that
is deliberately wrong, and for as long as the window lasts anyone who builds,
lints or runs a suite gets that wrongness as their answer.

**Restoring from a snapshot does not make the window safe.** The snapshot holds
what the file looked like when it was taken, so an edit another session makes
inside the window is overwritten by the restore. The mutation is visible; the
loss is silent, and it lands in somebody else's work.

**When the only mutation point is a shared file, say so rather than taking the
window.** A discrimination claim that cannot be made safely is reported as not
made, with the file named and the reason given. That is a smaller cost than a
lost edit nobody can attribute, and `ai/rules/never-destroy-work.md` already
settles which of the two is acceptable.

**A build file, a manifest and a generated artifact are shared by default**, and
so is any source file another session's uncommitted work touches. Check before
mutating, not after restoring.
