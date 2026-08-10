# Deferrals: followup-rfc-enrollment

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-rfc-requirement-coverage | **Enroll the remaining 172 RFC summaries into `rfc/enrolled.txt`.** The spec pilots RFC 7606 only (52/52 gated). Every other summary is listed in `ai/RFC-REQUIREMENTS.md` but NOT gated, so nothing enforces its MUSTs. The Coverage-by-RFC rollup sizes it exactly: **2136 MUST-level requirements owe work across 146 summaries**, ranked nearest-to-enrollable, and flags any RFC that is already enrollable at zero cost. A separate derived list names 9 summaries (rfc3630, rfc5187, rfc5303, rfc5304, rfc5310, rfc5392, rfc6549, rfc7684, rfc7770) that capture ZERO of their source RFC's MUSTs and must be re-authored via `/ze-rfc` before they can be enrolled at all | Enrollment is a program, not one spec's worth of work: RFC 7606 alone took a re-author, ~130 tags, 3 compliance fixes and 10 annotated divergences. `ai/rules/testing.md` (Back-Fill New Test Types) requires the remainder be explicit rather than implicit; the ratchet + rollup are that record, and both are DERIVED from the tags so they cannot rot | `plan/spec-followup-rfc-enrollment.md` (owns `rfc/enrolled.txt` and the rollup going forward; created at closure of the pilot spec) | open |

