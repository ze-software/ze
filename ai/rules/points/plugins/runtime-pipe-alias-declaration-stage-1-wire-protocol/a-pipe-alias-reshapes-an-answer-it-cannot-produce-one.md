---
kind: note
level: MUST
stage:
---
**A pipe alias SELECTS and re-sequences an answer. It renames no key, adds no
numbers and counts no matching rows.** So a command that wants an alias MUST
emit the aggregate fields beside the detail rows, as siblings at one level.
`show bgp rpki` is the worked example: `overviewCommand` writes both halves into
one record, and `| summary` selects the first half. A command whose second view
needs computed data stays a subcommand, and so does one that takes a value.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- overviewCommand, appendSummaryFields, summaryFieldNames -->
