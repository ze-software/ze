---
kind: directive
level: MUST NOT
stage:
---
A positional argument before the action or selector-kind MUST NOT be a
user-supplied value. Ask these questions to decide whether a handler conforms:

1. Is `args[0]` always a keyword from a known set? -> Correct.
2. If the command selects one member of a set, does the handler consume a
   selector-kind keyword before the free-form value? -> Correct.
3. Can any positional argument before the action or selector-kind be a
   user-supplied value? -> Violation.
