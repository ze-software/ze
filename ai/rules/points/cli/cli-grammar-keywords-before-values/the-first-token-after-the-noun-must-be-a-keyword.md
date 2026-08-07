---
kind: note
level: MUST NOT
stage:
excepted-by: cli/cli-grammar-keywords-before-values/peer-commands-are-the-exception-to-typed-selectors
---
The first token after the noun (component/resource) MUST be a keyword from a
closed set known at compile time. If a command targets one member of a set,
the selector itself MUST also be typed by a keyword such as `name`, `id`,
`index`, `address`, or `type`. Free-form values MUST NOT appear in an untyped
positional slot.
