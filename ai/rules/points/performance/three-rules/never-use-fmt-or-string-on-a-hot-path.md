---
kind: directive
level: MUST
stage:
---
1. **`fmt` MUST NOT be used on hot paths.** Append-based primitives MUST be used instead.
2. **`.String()` MUST NOT be used on hot paths.** `AppendTo` MUST be used into a stack buffer instead.
3. **Typed values MUST be stored, not strings.** `netip.Addr` MUST be compared directly, not string representations.
