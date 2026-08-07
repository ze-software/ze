---
kind: directive
level:
stage:
---
1. **No `fmt` on hot paths.** Use append-based primitives instead.
2. **No `.String()` on hot paths.** Use `AppendTo` into a stack buffer instead.
3. **Store typed values, not strings.** Compare `netip.Addr` directly, not string representations.
