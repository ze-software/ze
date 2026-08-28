---
kind: table
level:
stage:
---
| Phase | Skill | Runs in | The main thread does |
|-------|-------|---------|----------------------|
| Research a topic or subsystem | `/ze-explore`, `/ze-audit` | subagent | states the question, reads the findings, decides what they change |
| Write or revise a spec | `/ze-spec` | **main thread**, its gates need `AskUserQuestion` | relays the user's answers, approves the design, owns the status transition |
| Stress-test a design | `/ze-design` | **main thread**, its gates need `AskUserQuestion` | carries the one-decision-per-question dialogue with the user |
| Implement | `/ze-implement` | subagent | selects the spec, relays user decisions, checks the report against the spec's ACs |
| Review gate | `/ze-review`, `/ze-review-spec` | subagent | verifies each finding, decides which are real, loops until zero |
| Review gate, deep | `/ze-review-deep` | **main thread**, and it fans out itself | verifies each finding, decides which are real, loops until zero |
| Close | `/ze-close` | subagent | confirms the Review Gate artifact is clean, then that the two closure commits actually ran |
| Debug a red test or gate | `/ze-debug` | **main thread**, and it fans out itself | confirms the diagnosis names a root-cause function, not a symptom |
| Verify | `/ze-verify` | subagent | reads the failure index, decides what to fix next |
