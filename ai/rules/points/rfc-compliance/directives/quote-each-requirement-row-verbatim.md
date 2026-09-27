---
kind: directive
level: MUST
stage:
rationale: ai/rationale/rfc-compliance.md
---
**The text of a requirement row in `rfc/short/<stem>.md` MUST be a verbatim span of the RFC section it cites, and MUST NOT be a paraphrase.** `./le rfc check` refuses a row that a commit adds or edits when its quote is not in that section, and refuses a stem whose count of unquoted rows rises. The unquoted count of a stem MUST NOT rise. The matching, the refusals and the backfill are in `docs/contributing/rfc-conformance-gates.md`, "The row quote".
**A verbatim row proves only that the RFC says the sentence.** It catches an obligation the summary invented. It does not catch one the summary missed, so the forward arithmetic of a walk is still owed, and a row not yet quoted stays a claim.
