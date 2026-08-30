---
kind: directive
level: MUST NOT
stage:
---
**The assertion pattern MUST match the row that describes the case, and
`expect=exit:code=0` alone MUST NOT be relied on.**

| Pattern | When to use |
|---------|------------|
| `expect=stderr:pattern=<production log line>` plus `reject=stderr:pattern=<wrong outcome>` | The plugin emits a decision log on every iteration. Preferred. |
| Return an error from the compiled observer | The observer computes something the engine cannot log directly |
| Rely on `expect=exit:code=0` alone | Forbidden: it does not prove the observer's assertion |
