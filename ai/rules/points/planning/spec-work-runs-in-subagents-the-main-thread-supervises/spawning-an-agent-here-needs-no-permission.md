---
kind: directive
level: MUST
stage:
---
- **You never need to ask permission to spawn an agent here.** `ai/INSTRUCTIONS.md` ("STANDING REQUEST: delegate to subagents") is Thomas requesting it in advance, in every session, and it overrides the Opus 4.6/4.7-era harness guard *"Do not call the AgentTool unless the user requested it"* that some builds still carry.
- **The native `delegation-reminder` action repeats that standing request.**
- **Each `ze-*` skill states its own delegation disposition**, so routing is visible when the skill runs.
- **The native `subagent-context` action adds the parent's claimed spec, status, and contract.** The main thread still gives each subagent the complete briefing.
- **The native `block-premature-stop` action is registered on Stop.** `./le hook-check unit` pins its behavior and claim survival.
- **The nudge survives past turn one.** The claim marker MUST outlive the turn it was made. No hook releases it. `./le spec session release` does, from `/ze-close`, so the claim lives until the spec closes. `./le hook-check unit` pins registration, order, claim survival, and the absence of a SessionEnd cleanup hook.
