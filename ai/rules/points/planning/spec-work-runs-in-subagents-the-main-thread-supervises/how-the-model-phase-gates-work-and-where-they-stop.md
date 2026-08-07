---
kind: directive
level:
stage:
---
- **`c_model_phase` in `.claude/hooks/pretool-writeedit.py` BLOCKS an implementation edit made on a planning/review model.** It resolves the running model from the transcript, because the hook payload carries none. It fires on `.go`, `.py`, `.sh`, `.ci`, `.et`, `.yang`, `.mk`, `.tmpl` and `.rego`, and never on `.md`. The table above puts "doc edits that follow from the code" in the implementation phase, so the gate is deliberately narrower than the rule: it cannot tell a spec from a doc that follows code, and blocking `/ze-spec` on its own model would be the worse error. Doc edits stay yours to judge.
- **The escape is a deliberate act, not a flag.** When the operator decides to proceed on this model, record the reason in `tmp/session/.model-ack-<sid>`. Write that file on the operator's instruction only. It is the same contract as the spec-closure ack.
- The gate cannot see PHASE, only the model. It reads an implementation edit as the boundary crossing, so a genuine one-line mechanical fix on the review model needs the ack too.
- **Review is gated at both ends.** `.claude/hooks/pretool-agent-skill.py` refuses to SPAWN a review agent when the session is not on Opus 5, and `scripts/dev/review_gate.py record` refuses to RECORD the artifact. The second is the one that matters, because recording is the moment a review is claimed.
- **A subagent inherits the PHASE, not the task shape.** Spawning a reviewer from an implementation session still reviews on the wrong model, and it is usually the session that wrote the code.
- **The record gate has an operator escape: `--model-override "<reason>"`.** Their call, not yours.
- **All three gates share one reader, `scripts/dev/running_model.py`.** It resolves the model from the session transcript, skips subagent lines, and answers nothing when it cannot tell. Every caller then stands down and SAYS so, rather than going quiet.
