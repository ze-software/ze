---
kind: note
level: MUST NOT
stage:
---
**Three holders refuse a declared alias name:**

- a built-in pipe operator that carries the name.
- a pipe filter on an OVERLAPPING command path that carries it.
- an alias on the EXACT same command path that carries it.

The two populations differ because the two resolution rules differ. A filter
wins its whole subtree, and a longer alias path deliberately shadows a shorter
one. A plugin MUST NOT name a command path it did not declare in the same
message. One refusal fails the whole stage 1 registration, and the daemon log is
where an operator reads it.
