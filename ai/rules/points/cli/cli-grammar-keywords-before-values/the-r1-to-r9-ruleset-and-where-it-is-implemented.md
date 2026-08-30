---
kind: directive
level: MUST
stage:
---
The R1-R9 ruleset (verb-first, token form, no `--flag`, namespace discipline,
keyword-before-value, action-before-identifier, config-tree mutation stays in
`set`/`delete`, string identifiers, compound-versus-namespace split) is implemented
once in `internal/component/command/grammar`, and reads the canonical verb registry
`internal/component/command` (`Verbs`). Seven feeders enforce it, and
`docs/architecture/cli/root-namespace-grammar.md` names what each one checks. A
grammar change MUST be run through `./le cli-grammar`, which drives them.
