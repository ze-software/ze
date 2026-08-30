---
kind: directive
level: MUST
stage:
---
**The main thread MUST supervise and MUST NOT perform the spec work itself.** It launches each phase, reads what comes back, VERIFIES it against source, decides, and gates the next phase. An agent's report is a claim, so relaying one unverified is fabrication with an extra hop. A session MUST work one spec at a time, claimed with `./le spec session claim spec <spec-file>`, and a main thread past 600k context MUST write its per-spec state file and hand off.
**Independent phases MUST be launched in ONE message with parallel `Agent` calls, and the fan-out MUST be announced first:** how many agents, what each does, and the rough cost. Spawning needs no permission here. Anything the user MUST answer stays in the main thread, because a subagent holds no dialogue.
**Each phase MUST run through its own skill: `/ze-explore` and `/ze-audit` for research, `/ze-implement` for implementation, `/ze-review` and `/ze-review-spec` for the review gate, `/ze-close` for closure, `/ze-verify` for verification. Four MUST stay in the main thread: `/ze-spec` and `/ze-design` need `AskUserQuestion` at their gates, and `/ze-review-deep` and `/ze-debug` fan out themselves.**
