---
name: test-failures-always-report
description: Always report test failures visibly regardless of cause. Pre-existing issues do not block commits but must be tracked for fixing.
type: feedback
---

Always report test failures visibly so the user does not miss them. Never dismiss a failure as "pre-existing" and move on silently.

**Why:** The user wants zero tolerance for known bugs. Pre-existing vs introduced is irrelevant to whether the bug should be fixed. The old rule wasted effort on proving blame instead of fixing the issue.

**How to apply:** When a test fails during verification: (1) report it prominently, (2) note whether it appears related to current changes, (3) pre-existing failures do not block committing current work, (4) flag the issue for resolution regardless.
