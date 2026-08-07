---
kind: directive
level:
stage:
---
**One verification command.** The spec's Goal Gates name `make ze-verify`, the
pre-commit gate (`ai/rules/git-safety.md`). Fast targets are for the inner
iteration loop and never appear as the gate. The template previously shipped
three different spellings, one of which was the fuzz-inclusive `ze-test` target
that the commit rule does not use.
