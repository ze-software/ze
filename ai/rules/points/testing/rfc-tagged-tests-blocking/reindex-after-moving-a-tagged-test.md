---
kind: directive
level: MUST
stage:
---
- **After adding, moving, deleting, or re-tagging a tagged test, or after an edit shifts its line, `./le rfc index-update` MUST run and BOTH of its outputs MUST land in the SAME commit**: `ai/RFC-REQUIREMENTS.md` and every changed file under `rfc/requirements/`. The per-RFC file records each test's `file:line`, and `./le rfc check` fails on a stale index AND on a stale per-RFC file, so committing the index alone lands on the next session as a red gate.
- **Which carrier a tag MAY live in, and what evidence kind and tier it earns, is `docs/contributing/rfc-implementation-guide.md`.** A tier is derived from the carrier and MUST NOT be declared by the test.
