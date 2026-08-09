---
kind: table
level:
stage:
---
| Gate | Reads | Missed by a `// Design:`-only grep |
|------|-------|-------------------------------------|
| `check_doc_links.py --design-only` | `// Design:` lines in `.go` | no |
| `spec-citation-check.py` | ANY `plan/spec-*.md` string inside a `plan/spec-*.md` | YES -- spec-to-spec citations |
