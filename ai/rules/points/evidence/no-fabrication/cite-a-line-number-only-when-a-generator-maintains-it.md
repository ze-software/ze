---
kind: directive
level: MUST NOT
stage:
---
**A line number in a document MUST NOT appear unless a GENERATOR maintains it (owner directive, 2026-08-03). Hand-typing one MUST NOT be done, because nothing refreshes it and nothing can tell it has gone wrong.** `ai/RFC-REQUIREMENTS.md` is the working example: its `file.go:line` entries are derived from `RFC requirement:` tags on every `make ze-rfc-index`, so they move when the tests move. A file earns this by declaring `GENERATED ... do not edit` in its first ten lines, and `c_line_number_ref` reads that declaration rather than a list of filenames.
