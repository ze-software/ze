---
kind: directive
level: MUST
stage:
rationale: plan/journal/gate-excludes-part-of-its-population.md
---
**When a change alters what reaches a component at runtime, the tests you MUST re-check are the ones its new semantics can REACH, not only the ones it edited.** Delivery, wiring, subscription and permission are the four shapes of that change: each one moves a fixture onto a different code path while every line of that fixture stays as it was.

**Every gate in this repository scopes itself to the files the commit touched, so the reachable set is yours to find.** `changed_test_files` (`./le commit audit`) builds its population from `git diff --name-status`, and the lint, the relaxation audit and the changed-file targets all read that same list. A fixture the change never opened is outside every one of them.
