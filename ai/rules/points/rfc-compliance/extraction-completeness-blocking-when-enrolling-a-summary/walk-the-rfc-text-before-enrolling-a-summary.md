---
kind: note
level: MUST NOT
stage:
---
Before enrolling `rfc/short/<stem>.md` in `rfc/enrolled.txt`, walk the RFC's own
text section by section and confirm every MUST / MUST NOT / SHALL / SHALL NOT /
REQUIRED has a checklist row. Fetch the source first if it is absent:
`curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`. A
claim of "verified against the RFC" is not reproducible when `rfc/full/` lacks
the file.
