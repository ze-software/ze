---
kind: directive
level: MUST
stage:
---
- **`.claude/hooks/pretool-agent-skill.py` BLOCKS the spawn** when the agent prompt asks for something a skill covers. It matches the ASK, never the subject: "review this diff" is routed, "explain how review works" is not.
- **Naming the skill in the prompt satisfies the gate**, so a subagent that MUST follow `/ze-explore` is spawned by saying so.
- The map it enforces: research is `/ze-explore`, review is `/ze-review`, spec conformance is `/ze-review-spec`, a red test is `/ze-debug`, spec work is `/ze-implement`, bug classes are `/ze-hunt`, spec audit is `/ze-audit`.
- A hand-written prompt reproduces a worse version of the skill and drops every gate it carries. That is what the gate exists to stop.
