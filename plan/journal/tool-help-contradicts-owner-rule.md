# Tool help contradicts an owner rule

A command's help or its documentation page grants an agent a permission that
`ai/META-DECISIONS.md` withholds, so an agent following the tool breaks the rule.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-25 | spec-le-cli-usability | `le verify changed` help and `verify-freshness-scope.md` | Both say an agent runs `verify changed` for its own evidence; the Linux-host owner rule allows scoped package tests only, and an agent ran it on that word | fixed: 530d157df0 makes the help and `verify-freshness-scope.md` say no agent runs it; the `verbChanged` comment in `internal/le/verify/actions.go` now says the same |
