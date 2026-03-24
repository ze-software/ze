---
name: test-failures-always-report
description: Always report test failures visibly. Investigate the root cause. Ask user how to proceed -- context and risk level determine the right call.
type: feedback
---

Always report test failures visibly. Investigate every failure regardless of origin.

**Why:** Claude's recurring pattern was dismissing failures as "pre-existing" or "unrelated" and continuing without investigation. The user had to repeatedly force investigation.

**How to apply:**
1. When a test fails, STOP and REPORT every failure to the user immediately
2. Investigate the root cause -- do not skip this step
3. Ask the user how to proceed. The right call depends on context and risk level:
   - Low-risk, clearly unrelated work: user may say continue
   - High-risk, interrelated changes: user may want it fixed first
4. Never assume a failure is safe to ignore. Never dismiss without investigating.
5. Never present a commit proposal when tests are failing without having reported the failures first.
