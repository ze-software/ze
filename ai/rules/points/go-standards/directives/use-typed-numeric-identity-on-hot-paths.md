---
kind: directive
level: MUST
stage:
rationale: ai/rationale/enum-over-string.md
---
**A hot path and a component seam MUST carry typed numeric identity, an enum, a registered ID, a bitset or a packed integer, and MUST NOT carry a string.** A typed enum MUST make its zero value invalid so an unset field cannot pass for a real one, a map on such a path MUST be keyed by the numeric type, the string MUST be parsed ONCE where it enters the component, an accepted string key MUST be bound to a constant, and conversion back to string MUST happen only at the wire sink or the human sink. The boundaries where a string IS correct are in `docs/contributing/go-conventions.md`.
