---
kind: directive
level: MUST NOT
stage:
---
- **No gate blocks an implementation edit by model.** An implementation phase MUST NOT be refused on the model that runs it.
- **Review is gated at both ends.** The native agent-skill hook refuses a review spawn on the wrong model, and `./le spec session review record` refuses the artifact.
- **A subagent inherits the phase, not the task shape.**
- **The record gate takes `model-override <reason>` only on operator instruction.** It MUST NOT be passed on your own judgement.
- **Both gates share `internal/le/spec/session/model.go`.**
