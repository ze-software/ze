---
kind: note
level:
stage:
---
A shard is a small markdown file with the six-column table header and only the rows
it owns. Add your row to the shard for its source (create the shard if it does not
exist); never touch another source's shard except to correct a row it owns. Because
each path has a single writer, `git add <shard>` stages only your row and git merges
disjoint shard creations without conflict.
