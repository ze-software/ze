---
kind: directive
level: MUST
stage:
---
**The first token after the noun MUST be a keyword from a closed set known at compile time, and a free-form value MUST NOT sit in an untyped positional slot.** A command path MUST name the subject and stop: selecting members of a set is a pipe operation the command declares, named for the field it selects on (`show sysctl | key <k>`, `show bgp rib | peer <sel>`), and MUST NOT be typed as an argument on the path. Keyword before value still governs every token left on the command path, and `selector` MUST NOT be exposed as an operator keyword. Peer commands are the one exception: they address a peer positionally. The two token orders this produces are on `docs/architecture/cli/root-namespace-grammar.md`.
