---
kind: directive
level: MUST NOT
stage:
---
**This checkout is shared by several sessions ON PURPOSE, so that concurrent work needs no merge. A session working here MUST NOT move its own work into a worktree it creates:** that reintroduces the integration step the shared tree exists to avoid. A worktree AGENT launched by the owner is a different thing and keeps its own rules.
**`git worktree add --detach <scratch-path> HEAD` MAY be used to READ a clean tree**, to establish whether a build break or a red test predates your own change. It MUST be created outside the repository, MUST NOT be written to, and MUST be removed with `git worktree remove` as soon as the read is done: a registered worktree shows up in every other session's `git worktree list`. When a single file answers the question, `git show HEAD:<path>` costs nothing and registers nothing.
