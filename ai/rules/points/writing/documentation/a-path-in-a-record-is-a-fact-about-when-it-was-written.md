---
kind: directive
level: MUST NOT
stage:
---
A path inside a `plan/` record MUST NOT be repointed when the file it names
moves. A spec, a journal row, a deferral, a debt row and a known-failure shard
each describe what was true when they were written, so the path is a fact about
that moment rather than a claim about the tree today. Rewriting it makes the
record say something that was never true.

`./le doc check links` does not police those trees, and
`citationExcludePrefixes` (`internal/le/doc/check/links.go`) names them with the
reason. The live instruction files under `plan/` stay in scope and are listed by
name: `plan/README.md`, the two templates, and the learned indexes. Those are
read for what is true NOW, so a dead path in one misleads a reader.

Everywhere else the rule is FIX ON TOUCH: repair a stale path in a file you are
already editing for another reason, and leave the rest alone. A rename owes the
record trees nothing.

The cost of the opposite policy is measured. One package rename left 383
dangling references across `plan/`, which the gate reported as breakage of the
same kind as a live document pointing at a deleted file. Chasing them meant
editing hundreds of historical files, each edit racing another session writing
its own rows, to make records restate the present.
