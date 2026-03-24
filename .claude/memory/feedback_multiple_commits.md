---
name: Commit granularity by system
description: Same system = one commit. Disjoint systems = separate commits.
type: feedback
---

Changes to the same system (code, tests, docs for one feature) go in one commit. Disjoint work (e.g., CLI changes and BGP encoding changes) gets separate commits.

**Why:** User wants readable, reversible history. One big bundle makes it hard to review and revert. But splitting related changes (feature + its tests) across commits makes no sense either.

**How to apply:**
- Feature code + its tests + its docs = one commit
- Unrelated bug fix in a different package = separate commit
- Rule file updates + the code they document = one commit if related, separate if not
- Ask if unsure whether changes are "same system" or "disjoint"
