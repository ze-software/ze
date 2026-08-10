---
kind: directive
level: MUST NOT
stage:
---
**Size alone MUST NOT be a reason to split a file that is:**
- Large but single coherent concern (capability registry, pool internals)
- CLI file with one-function-per-subcommand
- Dependency chain where dispatcher references all implementations
