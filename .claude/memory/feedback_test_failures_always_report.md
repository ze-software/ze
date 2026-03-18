---
name: test-failures-always-report
description: ZERO TOLERANCE - Never dismiss test failures. Investigate and fix ALL failures before committing. Never say "pre-existing" or "unrelated" as justification for not fixing.
type: feedback
---

NEVER dismiss a test failure. NEVER use the words "pre-existing" or "unrelated" to justify not investigating and fixing a failure.

**Why:** The user has corrected this behavior MANY times. Each time Claude says "pre-existing, unrelated, not our changes" and moves on, the user has to stop work and force Claude to actually fix the problem. This is the single most frustrating recurring failure. The rules in anti-rationalization.md are crystal clear: "Not related to our changes" -- Answer: "Always report visibly. Pre-existing or not, it needs fixing."

**How to apply:**
1. When `make ze-verify` fails, INVESTIGATE every failure immediately
2. Fix every failure you can fix, regardless of whether you caused it
3. If a failure is genuinely unfixable (needs user input), explain the root cause and ask
4. NEVER say "pre-existing failures don't block commits" as an excuse to skip investigation
5. NEVER present a commit proposal when tests are failing
6. The ONLY acceptable state before asking to commit is: all tests pass, or you've investigated every failure and presented the root cause analysis with a fix proposal

**What NOT to do:**
- "This is pre-existing and unrelated to our changes" -- NO. Investigate and fix it.
- "These failures are in packages we didn't modify" -- NO. Fix them anyway.
- "Pre-existing failures don't block committing unrelated work" -- NO. Fix first, commit second.
- Listing failures and then immediately asking "ready to commit?" -- NO.
