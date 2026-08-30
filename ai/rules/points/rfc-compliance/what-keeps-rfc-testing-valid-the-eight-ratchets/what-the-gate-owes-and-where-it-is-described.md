---
kind: directive
level: MUST
stage:
---
**`./le rfc check` reads the WORKING TREE, and eight comparisons against HEAD supply what a tree cannot tell: "never proven" from "stopped being proven". Each fires only on a real downgrade, so you MUST treat a green run as evidence the proof held rather than as evidence nobody looked.**
**A red ratchet names a DOWNGRADE you made. You MUST restore the evidence it names; you MUST NOT reach for the annotation, the level change, or the deleted row that would make it green.** Each ratchet, what fires it, and the one documented escape are in `docs/contributing/rfc-conformance-gates.md`.
