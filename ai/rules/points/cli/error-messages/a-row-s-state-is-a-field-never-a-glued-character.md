---
kind: directive
level: MUST
stage:
---
**A row's state MUST be a FIELD or a COLUMN. It MUST NOT be a character glued to another
field's value.** No `*`, no `>`, no `+`, no leading dot. If a row is different,
say so in a place a reader and a pipe can both find.
