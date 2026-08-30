---
kind: directive
level: MUST
stage:
---
- **A struct field that is a plain `uint8` or `uint32` but represents a domain concept (Origin, MED, ASN) SHOULD be a named type carrying formatting methods. When changing the field type is too large for the task in hand, `textbuf.Buffer` methods MAY be used as a stepping stone, and the typed refactor MUST be tracked as follow-up.**
