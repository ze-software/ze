---
kind: directive
level: MUST
stage:
---
- **A row's state MUST be a field or a column; it MUST NOT be a character glued to a value.** No `*`, `>`, `+` or leading dot on an identifier. A sigil corrupts the value for `| grep` and has nowhere to live in `| json`, so the text and JSON forms stop agreeing on what the value is. `*` is also already an input token here, the selector wildcard. See "A value carries no marker" below.
