---
kind: directive
level: MUST NOT
stage:
---
- **NO gate blocks an implementation edit by model, and none MUST be written.** `c_model_phase` in `.claude/hooks/pretool-writeedit.py` did, and it went with the Opus 4.8 requirement above. What is gated now is review independence, at both ends. Which files you edit on which model is yours to judge.
- **The escape from the spawn gate is a deliberate act, not a flag.** When the operator decides to proceed on this model, record the reason in `tmp/session/.model-ack-<sid>`. MUST NOT write that file except on the operator's instruction. It is the same contract as the spec-closure ack.
- **Review is gated at both ends.** `.claude/hooks/pretool-agent-skill.py` refuses to SPAWN a review agent when the session is not on Opus 5, and `scripts/dev/review_gate.py record` refuses to RECORD the artifact. The second is the one that matters, because recording is the moment a review is claimed.
- **A subagent inherits the PHASE, not the task shape.** Spawning a reviewer from an implementation session still reviews on the wrong model, and it is usually the session that wrote the code.
- **The record gate has an operator escape: `--model-override "<reason>"`.** Their call, not yours.
- **Both gates share one reader, `scripts/dev/running_model.py`.** It resolves the model from the session transcript, skips subagent lines, and answers nothing when it cannot tell. Every caller then stands down and SAYS so, rather than going quiet.
