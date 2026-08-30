---
kind: directive
level: MUST
stage:
---
| You are about to ... | Do instead |
|----------------------|------------|
| Classify a requirement `{gap}`, `{not-applicable}`, `partial`, or "does not apply to ze" | Ask. A classification that lowers what Ze owes is a decision about compliance, not bookkeeping |
| Leave a MUST implemented but unproven (no `RFC requirement:` tagged test) | Ask. "Implemented" is a claim; the tagged test is the evidence (`ai/rules/testing.md`, `ai/rules/testing.md`) |
| Leave a MUST unextracted, or scope a spec so an RFC obligation falls outside it | Ask. See Extraction Completeness -- the gate cannot see an obligation nobody wrote down |
| Defer an RFC requirement to a follow-up spec, a deferral row, or a known-failure shard | Ask. Recording is not fixing (`ai/rules/completion.md`), and the deferral machinery is not a compliance decision procedure. The spec-close-ask route in the conformance table above IS that ask, made the same session. The deferral row is never a substitute for it |
| Lower a requirement's level in `rfc/short/`, so the row stops being MUST-level | Read the RFC sentence first. It is a CORRECTION only when the document states the lower strength, and then it MUST be recorded as a `Correction <YYYY-MM-DD>:` paragraph quoting that sentence, which `check_level_ratchet` refuses to do without. Anything else lowers what Ze owes: ask |
| Close a spec, review, or audit whose RFC rows are anything other than implemented-and-proven | Ask before closing, not after |
| Answer "is this conformant enough" with anything but yes | Ask. "Enough" is Thomas's word to say, never yours |
