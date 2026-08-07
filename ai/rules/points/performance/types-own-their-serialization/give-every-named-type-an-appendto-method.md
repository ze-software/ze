---
kind: directive
level: MUST
stage:
---
- **Named types MUST have an `AppendTo([]byte) []byte` method. Callers never format a type from the outside.**
- **Callers chain: `buf = typeA.AppendTo(typeB.AppendTo(buf[:0]))`.**
