---
kind: directive
level: MUST NOT
stage:
---
**A path inside a `plan/` record MUST NOT be repointed when the file it names moves.** A spec, a journal row, a deferral, a debt row and a known-failure shard each describe what was true when they were written, so the path is a fact about that moment rather than a claim about the tree today. `citationExcludePrefixes` (`internal/le/doc/check/links.go`) keeps those trees out of `./le doc check links`, and it keeps the live instruction files IN by name: `plan/README.md`, the two templates, and the learned indexes.
**Everywhere else the rule is FIX ON TOUCH: you MUST repair a stale path in a file you are already editing for another reason, and you MUST leave the rest alone.** A rename owes the record trees nothing.
