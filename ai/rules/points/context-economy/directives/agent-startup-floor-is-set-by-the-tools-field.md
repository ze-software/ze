---
kind: directive
level: MUST
stage:
---
**A name in an agent's `tools:` field that the harness does not serve is dropped in SILENCE, so a `tools:` list MUST be probed by spawning the agent and asking it to enumerate its own registry.** A name in the MAIN thread's registry is no evidence the subagent gets it, and a definition carrying no `tools:` field inherits every schema in the registry. A new or edited definition takes effect only in the NEXT session: write it in `ai/agents/`, then run `./le ai skills-sync`.
