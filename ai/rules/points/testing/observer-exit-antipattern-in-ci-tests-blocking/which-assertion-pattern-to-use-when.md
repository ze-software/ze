---
kind: table
level:
stage:
---
| Pattern | When to use |
|---------|------------|
| `expect=stderr:pattern=<production log line>` + `reject=stderr:pattern=<wrong outcome>` | Plugin emits a decision log on every iteration. **Preferred.** |
| `runtime_fail(...)` from observer when assertion fails | Observer must compute something the engine cannot log directly |
| Rely on `expect=exit:code=0` alone with a Python observer | Forbidden -- silent false positive |
