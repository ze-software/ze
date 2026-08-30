---
kind: directive
level: MUST NOT
stage:
excepted-by: cli/cli-grammar-keywords-before-values/peer-commands-are-the-exception-to-typed-selectors
---
The first token after the noun (component or resource) MUST be a keyword from a
closed set known at compile time. A command that targets one member of a set MUST
also type the selector with a keyword such as `name`, `id`, `index`, `address`, or
`type`. A free-form value MUST NOT appear in an untyped positional slot. The two
token orders this produces are on
`docs/architecture/cli/root-namespace-grammar.md`, "The command shape, and what
each feeder checks".
