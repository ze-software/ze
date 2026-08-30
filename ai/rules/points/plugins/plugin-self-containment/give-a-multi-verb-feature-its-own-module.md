---
kind: directive
level: MUST
stage:
---
- **A feature that spreads across several verbs MUST get its own module `internal/component/<feature>` that owns every one of those commands, rather than being scattered across the verb packages.** Create the module when none exists. When two such modules would share low-level primitives, those primitives MUST be extracted to an `internal/core/<x>` package, so neither feature module depends on the other or on a central verb package.
