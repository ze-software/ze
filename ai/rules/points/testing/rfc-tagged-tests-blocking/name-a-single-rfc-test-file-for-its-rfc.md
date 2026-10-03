---
kind: directive
level: MUST
stage:
---
- **A unit test file whose `RFC requirement:` tags, proof and gap alike, all cite one RFC MUST be named for it, `rfcNNNN_<topic>_test.go`, and a file named for an RFC MUST carry a tag for that RFC or the marker `// RFC naming: untagged -- <reason>`.** The reason MUST be on the marker's line and MUST state what the file tests and why no tag fits, such as a red defect probe or an RFC that states no requirement the test proves. A file whose tags cite two or more RFCs MAY carry any name that claims no RFC it does not tag. `./le rfc check` reports a file that breaks either direction.
- **A misnamed test file MUST be moved with `./le rfc rename from <old> to <new>` and MUST NOT be moved by hand,** because the rename rewrites the discrimination records and audit verdicts keyed by the old path, and a byte-pure move then owes no new record. The prefixes and the repair are "Test file names" in `docs/contributing/rfc-conformance-gates.md`.
