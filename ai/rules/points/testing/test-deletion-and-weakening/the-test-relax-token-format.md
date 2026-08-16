---
kind: fence
level:
stage:
---
Two columns, under the exact header the parser anchors on:

```
| Test | Reason |
|------|--------|
| TestName | <what left the suite, and why the commit is correct without it> |
```

The name is the enclosing top-level `func TestXxx` for Go, and the file stem for
a `.ci`, a `.et`, or a Go weakening sitting outside every func.
`scripts/dev/rfc_tagged_scope.py` resolves each one, so the edit-time hook and
the commit gate name the same unit. A bare name is accepted when it resolves to
exactly one weakened test in the commit. Write `package.TestName` when it does
not.
