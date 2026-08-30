---
kind: directive
level: MUST
stage:
---
- Every point whose `kind` is `directive` MUST state its obligation in a capitalised RFC 2119 keyword, and its `level:` MUST name the strongest TIER the body states: MAY, then SHOULD with SHOULD NOT, then MUST with MUST NOT. A directive whose weight a reader infers from tone is a directive two readers weigh differently.
- The lowercase spellings `must`, `shall`, `should` and `may` MUST NOT appear in a directive body, and a block that states no obligation is `kind: note` or `kind: table`. `writePointLanguage` (`internal/le/hookruntime/writeedit.go`) refuses the write and `./le rules lint` refuses the finished tree. The accepted keywords and how each maps to a level are in `docs/contributing/rule-authoring.md`.
