---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/no-partial-completion.md
---
**Completion MUST be judged against the explicitly agreed session acceptance criteria. You MUST NOT claim work is done, complete, ready to commit, or ready for review while any of those criteria remains unmet.** A staged baseline verification deliverable MAY close when its agreed criteria are met, with unfinished features and absent RFC requirements disclosed. That closure claims neither completion of those features nor full RFC support; `ai/rules/rfc-compliance.md` governs protocol scope and conformance reporting.
**You MUST have READ the diff, hunk by hunk, before the claim.** A gate covers what somebody thought to check, so a green run over an unread diff is neither done nor green: say what you have, which is that the gates pass and you have not read the change.
**A report MUST distinguish implemented-and-tested behavior, implemented-but-unverified behavior, and unimplemented requirements.** Name the evidence supporting each tested claim and state unresolved existing-capability defects and verification or build limits. Disclosure does not satisfy an unmet acceptance criterion, and changing those criteria requires the user's agreement.
**Unfinished changes MUST be preserved under `ai/rules/never-destroy-work.md`.** Their presence does not authorize extending or finishing them. State their limits without treating them as completed features or inferring a larger implementation scope.
