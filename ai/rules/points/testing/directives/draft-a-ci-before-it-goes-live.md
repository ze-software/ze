---
kind: directive
level: MUST
stage:
---
- **A `.ci` MUST be written and iterated in `test/draft/<suite>/`, and a live one MUST NOT be edited in place.** `test/<suite>/` runs on every verify in this checkout, including runs by other sessions, who then have to work out whether your half-written test is their regression.
- **The draft workflow MUST end in a promotion or a deletion.** The incubator is gitignored and skipped by every repo-wide gate, so nothing in it proves anything, and a session that finds one cannot tell abandoned scaffolding from work in progress. The commands are `docs/functional-tests.md`, "Writing a Test: Draft First".
