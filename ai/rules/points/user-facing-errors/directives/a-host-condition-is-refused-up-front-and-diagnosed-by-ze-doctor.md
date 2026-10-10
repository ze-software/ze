---
kind: directive
level: MUST
stage:
rationale: ai/rationale/user-facing-errors.md
---
- **When Ze or its tooling cannot act because of the host (a missing program, an absent kernel feature, a security policy that refuses it), the error MUST name that condition and its fix, and MUST refuse before the work starts whenever the condition can be checked first, rather than fail halfway with the symptom.** `ze doctor` is where a host condition the daemon depends on is diagnosed and reported, so the operator can find it before the failure and confirm the fix after it; the check a new runtime dependency owes is in `ai/rules/repo-maintenance.md`. Where tooling can make the operator do the right thing (a setup action, a preflight check, a refusal that prints the command), that tooling SHOULD exist rather than a page the operator has to find.
