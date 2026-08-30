---
kind: directive
level: MUST
stage:
---
**The first token after the noun MUST be a keyword from a closed set known at compile time, and a free-form value MUST NOT sit in an untyped positional slot.** A command addressing one member of a set MUST type the selector with `name`, `id`, `index`, `address`, `type`, `key`, or another schema-defined closed keyword, and `selector` MUST NOT be exposed as an operator keyword. Peer commands are the one exception: they address a peer positionally. The two token orders this produces are on `docs/architecture/cli/root-namespace-grammar.md`.
