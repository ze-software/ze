---
kind: directive
level: MUST
stage:
---
**The main thread MUST supervise and MUST NOT perform the spec work itself.** It launches each phase, reads what comes back, verifies it against source, decides, and gates the next phase. Running an agent's owed commands and committing its code is the main thread's own part, and is not spec work (`ai/rules/commands.md`). An agent's report is a claim, so relaying one unverified is fabrication with an extra hop. A session MUST work one spec at a time, claimed with `./le spec claim <spec-file>`; its agents MAY run that spec's child specs or the specs its work depends on, each under the agent's own claim and inside the WIP cap, while the main thread supervises them. A main thread past 600k context MUST write its per-spec state file and hand off.
**Independent phases MUST be launched in one message with parallel `Agent` calls, and the fan-out MUST be announced first:** how many agents, what each does, and the rough cost. Spawning needs no permission here. Anything the user MUST answer stays in the main thread, because a subagent holds no dialogue.
**Each phase MUST run through its own skill: `/ze-explore` and `/ze-audit` for research, `/ze-implement` for implementation, `/ze-review` and `/ze-review-spec` for the review gate, `/ze-close` for closure. Five MUST stay in the main thread: `/ze-spec` and `/ze-design` need `AskUserQuestion` at their gates, `/ze-review-deep` and `/ze-debug` fan out themselves, and `/ze-verify` runs the tests, which the main thread schedules on the shared machine.**
