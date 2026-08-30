---
kind: directive
level: MUST
stage:
---
An identifier MUST be string-typed even when the conventional representation is
numeric. Cache IDs, VLAN IDs and session IDs are accepted and stored as strings, and
parsed to numeric only at the point of use when the underlying API requires it. This
avoids:

- Grammar ambiguity between numeric keywords and numeric IDs
- Unnecessary coupling to a representation (an ID can become non-numeric later)
- Parsing errors surfacing at the wrong layer
