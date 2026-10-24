---
kind: directive
level: MUST
stage:
---
- **A test weakening MUST be approved by the owner before the edit, and his words MUST be recorded with `./le test weakened approve unit <package>.<TestName> reason "<the owner's words>"`.** An agent MUST NOT write its own approval, and one approval names one test, never a tree or a file. The edit hook and `./le commit create` read the row from `tmp/commit-weakening-approved-<session>.md`, keyed on the main thread's namespace, so a subagent's edit sees its parent's record; the commit carries each used row as a `Weakening-approved:` line, and a row a landed commit used approves nothing further. An `RFC-approved:` row for the unit also accepts its weakening. The route is in `docs/contributing/testing.md`, and the design in `docs/architecture/testing/test-health.md`.
