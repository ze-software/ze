# Tool help contradicts an owner rule

A command's help or its documentation page grants an agent a permission that
`ai/META-DECISIONS.md` withholds, so an agent following the tool breaks the rule.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-25 | spec-le-cli-usability | `le verify changed` help and `verify-freshness-scope.md` | Both say an agent runs `verify changed` for its own evidence; the Linux-host owner rule allows scoped package tests only, and an agent ran it on that word | open: help and page state the owner rule, derived from it rather than restated |
