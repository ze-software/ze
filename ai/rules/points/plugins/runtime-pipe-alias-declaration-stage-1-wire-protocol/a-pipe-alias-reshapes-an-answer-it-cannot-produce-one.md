---
kind: directive
level: MUST
stage:
---
- **A pipe alias SELECTS and re-sequences an answer: it renames no key, adds no number and counts no row. A command that wants an alias MUST therefore emit the aggregate fields beside the detail rows, as siblings at one level.** `show bgp rpki` is the worked example: `overviewCommand` writes both halves into one record and `| summary` selects the first half. A view whose data has to be computed stays a subcommand, and so does one that takes a value.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- overviewCommand, appendSummaryFields -->
