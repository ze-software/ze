---
name: NEVER claim done without wiring proof
description: CRITICAL - user furious about repeated false claims of completion when features are not wired into user-facing paths. This has happened on multiple specs including SSH transport. Zero tolerance.
type: feedback
---

NEVER claim a feature is "done", "complete", or "implemented" unless you can answer YES to this question:
**"Can a user reach and use this feature right now through config/CLI/API?"**

If the answer is no, the feature is NOT done. Say "blocked" or "not wired yet". Never say "done".

**Why:** This has happened repeatedly across multiple specs. The user has explicitly stated this is their #1 frustration with Claude. Unit tests passing is not evidence. Library code existing is not evidence. The ONLY evidence is a .ci functional test or a demonstrated user-facing path.

**How to apply:** Before ANY claim of completion:
1. Name the user entry point (CLI command, config option, API call)
2. Show the .ci test that exercises it, or run the feature live
3. If neither exists, say "logic implemented but not yet wired to user entry point"

Severity: relationship-breaking. The user will not tolerate this happening again.
