---
kind: directive
level: MUST NOT
stage:
---
**A variable or constant MUST name what the value IS. It MUST NOT encode its Go type.** `famStr`, `levelStr` and `addrStr` name the type; `family`, `level` and `addr` name the value. The banned patterns and their fixes are in `docs/contributing/go-conventions.md`.
