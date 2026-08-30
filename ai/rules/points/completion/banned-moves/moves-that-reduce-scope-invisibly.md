---
kind: directive
level: MUST NOT
stage:
---
**These moves MUST NOT be made. Each reduces scope invisibly:**

| Banned | Why |
|--------|-----|
| Offering the user "drop the interop / functional test" as an option | Reducing coverage to reach green is the failure, not a choice. Do not put it on the table. |
| Weakening or deleting a test so a red goes green | `ai/rules/testing.md`, and "No Workarounds For Missing Behavior" below. The test describes the behaviour; the code is what is wrong. |
| "Pre-existing defect, out of scope for this spec" when it blocks the goal | You are the entry point that reaches it. Fix it, or say plainly that you are stuck and why. Never quietly route around it. |
| Moving unfinished work or a bug report to `tmp/` and reporting the rest as complete | `tmp/` is not a destination. Parked is not delivered. |
| Marking a goal-validation row "N/A" or "blocked" to avoid the work | An empty goal validation for a completed feature is a false completion (`ai/rules/interop-and-goal-validation.md`). |
