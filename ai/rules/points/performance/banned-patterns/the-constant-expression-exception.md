---
kind: directive
level: MAY
stage:
---
- **A compile-time constant expression whose sides are both untyped string literals MAY use `+`: `const x = "foo" + "bar"`. The compiler folds it, so nothing is allocated. No other `+` between strings is permitted.**
