---
kind: table
level:
stage:
---
| Where a void answer hides | What to do when you meet one |
|---------------------------|------------------------------|
| A `plan/learned/` deviation record, or a spec `Deviations` row | Do not rely on it. Raise the requirement with Thomas again, then correct the record with the new answer |
| A `{gap}` / `{not-applicable}` / `partial` in `rfc/short/*.md` or `docs/features/rfc-status.md` | Re-derive it from the RFC text. If it still reads as less than full compliance, ask |
| A deferral row marked `user-approved-drop`, or a `cancelled` status, covering an RFC obligation | Void. Re-raise it; the row is not a close |
| A code comment or `rfc/audit/*.json` verdict calling the deviation deliberate | A comment is a belief, not a ruling (`ai/rules/evidence.md`). Void by default; ask |
