---
kind: note
level:
stage:
---
When a struct field is a plain `uint8`/`uint32` but represents a domain concept
(Origin, MED, ASN), it should use a named type with formatting methods. If
changing the field type is too large for the current task, use `textbuf.Buffer`
methods as a stepping stone, but track the typed refactor as follow-up.
