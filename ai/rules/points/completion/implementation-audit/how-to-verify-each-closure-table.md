---
kind: directive
level: MUST
stage:
---
**Every closure table MUST be re-verified after the audit, by the method its row names:**

| Table | What to verify | How |
|-------|---------------|-----|
| Files Exist | Every file from "Files to Create" | `ls -la <path>`, paste output |
| AC Verified | Every AC-N | grep, test output, or ls, NOT a copy from audit |
| Wiring Verified | Every wiring test row | Read the .ci file, confirm it tests the claimed path |
| Assumptions Resolved | Every A-N | `confirmed` or `broken` with evidence; `unvalidated` is not a final status |
| Documentation Verified | Every Yes/No in the Documentation checklist | The edited claim checked against source, or the grep proving no update was needed |
