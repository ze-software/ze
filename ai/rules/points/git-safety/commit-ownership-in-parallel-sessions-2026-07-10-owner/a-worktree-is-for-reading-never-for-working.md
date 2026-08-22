---
kind: note
level: MUST NOT
stage:
---
This checkout is shared by several sessions ON PURPOSE, so that concurrent
work needs no merge. A session working here MUST NOT move its own work into
a worktree it creates: that reintroduces the integration step the shared tree
exists to avoid. (A worktree AGENT, launched into `.claude/worktrees/` by the <!-- doc-links: ignore (a local worktree root, excluded via .git/info/exclude; it never exists in a clean checkout) -->
owner, is a different thing and keeps its own rules above.)

`git worktree add --detach <scratch-path> HEAD` MAY be used to READ a clean
tree -- to establish whether a build break or a red test predates your own
change, which a dirty shared tree cannot answer on its own. It MUST be
created outside the repository, it MUST NOT be written to, and it MUST be
removed with `git worktree remove` as soon as the read is done: a registered
worktree shows up in every other session's `git worktree list`, so one left
behind is a change to shared state nobody asked for. When a single file
answers the question, `git show HEAD:<path>` costs nothing and registers
nothing.
