---
kind: note
level:
stage:
---
These rules are enforced automatically, not just by review. The reverse-engineered
ruleset is R1-R9 (verb-first, token form, no `--flag`, namespace discipline,
keyword-before-value, action-before-identifier, config-tree-mutation stays in
`set`/`delete`, string identifiers, compound-vs-namespace split), implemented once in
`internal/component/command/grammar` and read from the canonical verb registry
`internal/component/command` (`Verbs`). Seven feeders enforce it:
