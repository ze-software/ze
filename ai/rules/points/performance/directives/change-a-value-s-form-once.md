---
kind: directive
level: MUST NOT
stage:
---
**On a hot path a value MUST change form once, from its source form to its destination form, and MUST NOT pass through a third form that it then leaves.** A typed value converted to bytes and back to a typed value (`netip.AddrFrom16(a.As16())`), and wire bytes parsed into a struct only to be encoded back to wire bytes, are the shapes. Inlining does not remove the cost: Vincent Bernat measured that round trip at eight times a direct method, and the compiler passes that could cancel it change order between Go releases. When the type has no direct operation, the operation MUST be added to the type that owns the representation; for a standard library type the round trip MAY stay off the hot path. A claim that the compiler removes the cost MUST cite `-gcflags=-m` or `-gcflags=-S` output. `/ze-find-alloc` hunts the pattern.
