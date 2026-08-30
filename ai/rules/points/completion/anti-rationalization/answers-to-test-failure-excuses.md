---
kind: directive
level: MUST NOT
stage:
---
**These test-failure excuses MUST NOT be acted on:**

| Excuse | Answer |
|--------|--------|
| "Transient" / "resource contention" | Investigate. A failure happened |
| "Only fails under load" / "passes in isolation" | That is the diagnosis, not an excuse: the test asserts on elapsed time. Make it wait on the condition (see "Load is never an explanation") |
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | It is yours: "pre-existing" says when it started, not whose it is, and you are the entry point that reached it. Blocks your goal, fix it now; does not, spec it, close, ask |
