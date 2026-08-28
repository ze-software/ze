---
kind: table
level:
stage:
---
| Pattern | When to use |
|---------|------------|
| `expect=stderr:pattern=<production log line>` plus `reject=stderr:pattern=<wrong outcome>` | The plugin emits a decision log on every iteration. Preferred. |
| Return an error from the compiled observer | The observer must compute something the engine cannot log directly |
| Rely on `expect=exit:code=0` alone | Forbidden: it does not prove the observer's assertion |
