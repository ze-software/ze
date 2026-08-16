---
kind: directive
level: MUST
stage:
---
- **MUST spawn every phase agent with a `subagent_type` from `ai/agents/`.** MUST use `ze-read` for a phase that only reads. MUST use `ze-work` for one that edits. `ze-close` is the one exception: step 5 spawns reviewers and neither agent holds the Agent tool. An agent's first call is re-fed on every later call, so the saving is the first-call difference times the agent's call count.
- **MUST compare the floor WITHIN one session; MUST NOT compare across.** `make ze-token-economy-report ZE_SESSION=<id-prefix>` prints the per-agent-type table for one session. The floor is the first call with the spawn prompt subtracted. That subtraction makes it a property of the agent TYPE, rather than of how much its parent wrote. Across sessions the always-on preamble changes size and swamps it, so re-run the command.
- **What buys the difference is the `tools:` FIELD, not a short list.** An agent definition without one inherits every schema in the registry. `ze-work` keeps Edit, Write and NotebookEdit and still lands with `ze-read`. MUST NOT drop a tool a phase needs to shorten the list. The length is not the term this lowers.
- **A name the harness does not serve is dropped in SILENCE, so you MUST probe a `tools:` list before you trust it.** MUST spawn the agent and ask it to enumerate its own registry. A name in the MAIN thread's registry is not evidence the subagent gets it.
- **`Explore` receives NONE of this repository's rules.** Repository work MUST NOT be routed to it. Choosing it for a review buys tokens by making the agent ignorant of `ai/rules/evidence.md` and `ai/rules/rfc-compliance.md`.
- **A new or edited agent definition takes effect in the NEXT session.** Claude Code reads `.claude/agents/` at session start. MUST write the canonical file in `ai/agents/`, run `make ze-ai-skills-sync`, and expect the saving from the following session.
