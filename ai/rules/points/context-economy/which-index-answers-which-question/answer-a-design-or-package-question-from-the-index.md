---
kind: directive
level: MUST
stage:
---
- **A question about what a file is FOR, what a package does, or which doc governs a subsystem MUST go to the index that answers it, before Read and before Grep over `docs/`.** The symbol route above answers what code IS; these answer what it is FOR, and neither substitutes for the other. `docs/contributing/navigating-the-code.md` names which index answers which question.
- **Every non-test `.go` file carries that answer in its own first 25 lines, as a `// Design: <doc> -- topic` header** (`HeaderLines` and `designLine`, `internal/le/docstocode/docstocode.go`).
- **MUST grep an index; MUST NOT read one.** `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` are each several hundred kilobytes, so reading either whole costs more than the source it was meant to save.
- **A digest orients; it never proves.** `ai/digests/*.md` are hand-maintained (`ai/digests/README.md`), so MUST open the files a digest names before stating what code does (`ai/rules/evidence.md`).
