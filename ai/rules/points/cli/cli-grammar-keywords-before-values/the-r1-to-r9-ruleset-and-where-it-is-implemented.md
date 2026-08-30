---
kind: directive
level: MUST
stage:
---
**The R1-R9 ruleset (verb-first, token form, no `--flag`, namespace discipline, keyword-before-value, action-before-identifier, config-tree mutation stays in `set`/`delete`, string identifiers even when numeric, compound-versus-namespace split) is implemented once in `internal/component/command/grammar` over the canonical verb registry, seven feeders enforce it, and a grammar change MUST be run through `./le cli-grammar`.** Ze is unreleased, so an unreleased grammar MUST be replaced outright rather than deprecated; command ownership MUST NOT be reshuffled while grammar is being fixed, and a rename of a programmatic command path breaks the wire, so every programmatic sender MUST be found first. What each feeder checks is `docs/architecture/cli/root-namespace-grammar.md`.
