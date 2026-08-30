---
kind: directive
level: MUST
stage:
---
**Before enrolling `rfc/short/<stem>.md` in `rfc/enrolled.txt`, you MUST walk the RFC's own text section by section and confirm every MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED has a checklist row.** A green gate is bounded by what was extracted, so an obligation nobody wrote down is invisible to it and to any audit that only re-checks classifications.
**When `rfc/full/` lacks the source, you MUST fetch it first: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`.** A claim of "verified against the RFC" is not reproducible without it.
