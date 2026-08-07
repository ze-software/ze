---
kind: table
level: MUST
stage:
---
| Field | Semantics |
|-------|-----------|
| `Dependencies` | Hard. Resolver returns `ErrMissingDependency` when the named plugin is not registered. Startup fails. |
| `OptionalDependencies` | Soft. Resolver pulls the plugin in if registered, silently skips if not. No error. Owner MUST detect absence at run time and fall back cleanly. |
