---
kind: directive
level: MUST
stage:
rationale: plan/journal/documentation-shows-config-the-parser-refuses.md
---
**Every config example in `docs/` MUST parse, and an excerpt MUST parse inside the smallest complete config that carries it.** Build the binary and run `ze config validate` over that config before you publish the example.
