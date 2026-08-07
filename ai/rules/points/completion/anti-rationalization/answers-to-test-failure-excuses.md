---
kind: table
level:
stage:
---
| Excuse | Answer |
|--------|--------|
| "Transient" / "resource contention" | Investigate. A failure happened |
| "Only fails under load" / "passes in isolation" | That is the diagnosis, not an excuse: the test asserts on elapsed time. Make it wait on the condition (see "Load is never an explanation") |
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | Fix it. "Pre-existing" says when it started, not whose it is. You are the entry point that reached it |
