---
kind: directive
level: MUST
stage:
---
**A `file <path>` list is a list of PATHS, and a path carries whatever the file holds
at that moment. Before naming a path, you MUST run `git diff` over it and confirm
every hunk is yours.**

Checking that your own edit is ready is not the same as checking that the file is.
In a checkout several sessions share, a file you edited an hour ago can hold
another session's uncommitted work by the time you commit, and the helper takes
the whole file. The commit then lands correct content under a message that
describes something else, and the session whose work you carried loses the
attribution rather than the work.

This is distinct from the shared-plan-log case, which is structural and expected.
That one cannot be avoided by checking, because a single-file log is written by
several sessions by design. A source file, a doc page or a script has no such
excuse: the hunks are separable, they are visible in one command, and reading
them costs a second.

The two failures are told apart by what the diff shows, not by which directory
the file is in:

- Every hunk is yours: commit it.
- A hunk is another session's, and the file is a shared plan log or journal class
  file: carry it, say so, and do not clean it out.
- A hunk is another session's, and the file is anything else: drop the path from
  this commit and let the session that owns the hunk land it, or say plainly in
  the message that you carried it and why.

The repair when it goes wrong is disclosure, never history. Rewriting to reclaim
attribution is banned, and the content is already safe.
