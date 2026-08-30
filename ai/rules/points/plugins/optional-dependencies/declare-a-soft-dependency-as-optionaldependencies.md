---
kind: directive
level: MUST
stage:
---
- **A plugin that USES another plugin when it is present but runs without it MUST declare the relationship as `OptionalDependencies`, never as `Dependencies`.** `Dependencies` is hard: a missing name gives `ErrMissingDependency` and startup fails. The resolver, validation and ordering semantics are `docs/architecture/plugin/plugin-system.md`, "Dependencies".
