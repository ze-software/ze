---
kind: directive
level: MUST
stage:
---
**All types MUST implement `BufWriter`: `WriteTo(buf, off) int` or `CheckedWriteTo(buf, off) (int, error)`.**
**Context-dependent types MUST take `*PackContext` for ADD-PATH/ASN4.**
