---
kind: table
level:
stage:
---
| Gate | Reads | Missed by a `// Design:`-only grep |
|------|-------|-------------------------------------|
| `check_doc_links.py --design-only` | `// Design:` lines in `.go` | no |
| `spec-citation-check.py` | ANY `plan/spec-*.md` string inside a `plan/spec-*.md` | YES -- spec-to-spec citations |
| `check_doc_links.py` check 5 (`check_tracked_citations`) | ANY path reference in ANY tracked file, a `plan/spec-*.md` target included | YES -- a citation from `docs/`, a script, or a test. `scripts/dev/doc_citation_baseline.txt` grandfathers only the pairs that predate the check, so commit B reds the gate for every tracked file that cites the spec it removes |
