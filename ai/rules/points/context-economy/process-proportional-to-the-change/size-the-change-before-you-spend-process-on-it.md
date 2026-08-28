---
kind: directive
level: MUST
stage:
---
**Process is proportional to the change. MUST size the diff BEFORE you spend agents and rounds on it, and state that size when you delegate.**
**The saving is in making the CHANGE smaller. It is never in reviewing a given change less, which stays banned above.**
**Line count decides the SPEC and the phase sequence. It never decides the agents, and it never decides the review rounds.**
**Not the agents: "this edit is small, I will just do it inline" is banned reasoning (`ai/rules/planning.md`), and this rule says MUST SIZE an agent, MUST NOT spawn fewer.**
**Not the rounds: `ai/rules/planning.md` "Bounding the loop" owns that number, and the diff's SIZE never bounds it. Every fix is new code and earns a fresh pass, and any always-in-scope class re-opens the loop whatever the diff's size, so a two-line change that removes a guard earns a second round exactly like a large one. What DOES bound it is what the rounds are finding: `./le spec-session review record` refuses more than three without `--rounds-reason` naming the PRODUCT defect a later round found, because a round auditing the spec's own closure prose is not converging on anything (`ai/rules/planning.md`, "A finding in the record is not a finding in the product").**
