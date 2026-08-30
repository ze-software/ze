---
kind: directive
level: MUST
stage:
---
- **An egress filter records attribute modifications with `filterapi.ModAccumulator.Op(code, action, buf)`. For an attribute whose value is a LIST of fixed-width wire values, `buf` MUST hold a whole number of those values, concatenated.** Several values in ONE operation is allowed and means "every one of them"; splitting them across operations is allowed too. A buffer that is not a whole number of values is what is forbidden.
- **A handler MUST NOT assume one value.** `wireu.StripControlCommunities` returns every matching route-server control community as one buffer. The per-attribute widths, who checks the arity, and what a violation costs are in `docs/architecture/core-design.md`, "Buffer arity, list-valued attributes".
