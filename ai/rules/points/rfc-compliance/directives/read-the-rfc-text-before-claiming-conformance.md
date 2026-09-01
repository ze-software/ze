---
kind: directive
level: MUST
stage:
---
**Before you claim a protocol behavior is correct, or report that Ze violates an RFC, you MUST read the RFC's own text in `rfc/full/<stem>.txt` or `rfc/drafts/` and quote at least one whole sentence with the section number you read it at.** A `rfc/short/` summary is a derived artifact and never the authority, so a finding that cites only a requirement id, only a summary line, or only its own paraphrase is UNVERIFIED and MUST be labelled so. Fetch a missing text first: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`.
**The RFC MUST outrank ExaBGP API compatibility, which MUST outrank the ExaBGP implementation.** An RFC the current one OBSOLETES MUST NOT be read as evidence about what Ze owes; the lineage that matters runs FORWARD, through the documents that UPDATE the current one and its errata. The enrolment walk, the extraction sign-off, the superseded marker and the nine ratchets are in `docs/contributing/rfc-conformance-gates.md`.
