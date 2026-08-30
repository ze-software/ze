---
kind: directive
level: MUST NOT
stage:
---
**A generic agent such as `Explore` receives NONE of this repository's rules, so repository work MUST NOT be routed to one.** It buys tokens by making the agent ignorant of `ai/rules/evidence.md` and `ai/rules/rfc-compliance.md`. Every phase agent MUST carry a `subagent_type` from `ai/agents/`: `ze-read` for a phase that only reads, `ze-work` for one that edits.
