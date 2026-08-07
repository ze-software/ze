---
kind: table
level: MUST
stage:
---
| Ratchet | Producer | Fires when |
|---------|----------|-----------|
| Enrolment is monotonic | `check_enrolment` | an RFC whose MUSTs were gated stops being gated |
| **Proof is monotonic** | `check_coverage_ratchet` | a requirement loses a polarity it had at HEAD. `{gap}` is NOT an escape: it is the move being blocked |
| **Requirements do not vanish** | `check_retired_requirements` | a requirement id of an enrolled RFC disappears from its summary. Without this, deleting the checklist line is the CHEAPEST route from red to green, cheaper than `{gap}` which costs a public disclosure row, and the ratchet would be pressuring people to hide obligations rather than declare them. Correcting a misquote means editing the TEXT under the same id, which is allowed |
| **Adding an RFC adds checking** | `check_new_summaries` | a summary that is NEW since HEAD declares gated MUSTs and is not in `rfc/enrolled.txt`, fails to parse, or captures zero requirements while `rfc/full/<stem>.txt` has MUST-level keywords |
| **Non-unit evidence is monotonic, per tier** | `check_evidence_ratchet` | a requirement loses an evidence KIND it had at HEAD -- its `.ci` becomes a unit test, or its verify-tier binding is swapped for a nightly-tier interop one. Keyed by `kind/tier`, so a substitution that leaves the tag COUNT unchanged still fires: a unit test proves the algorithm, only a running functional or interop test proves the daemon or a peer. No annotation satisfies it |
| **Extraction is monotonic** | `check_extraction_ratchet` | a stem that carried a sign-off at HEAD carries none now, or a signed stem's exclusion count RISES without a `resign-reason` and a bumped `signed-off` date. The first stops the bound being un-bound by deleting a file; the second stops the exclusion list becoming an escape hatch where every unmapped site is excluded with a shrug |
| **Public disclosure is monotonic** | `check_status_completeness` | an RFC enrolled since HEAD has no row in `docs/features/rfc-status.md`, or a row that existed at HEAD is gone while its RFC stays enrolled. Enrolment gates that RFC's MUSTs, so the public page must say the RFC exists. A deleted row retires the public claim and leaves the obligation. It is also the one edit that makes `check_status_agreement`'s missing-row branch fire later, on somebody else's unrelated commit |
