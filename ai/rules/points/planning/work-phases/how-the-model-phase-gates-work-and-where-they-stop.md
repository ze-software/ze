---
kind: directive
level: MUST NOT
stage:
---
- **No gate blocks an implementation edit by model.**
- **Review is gated at both ends.** The native agent-skill hook refuses a review spawn on the wrong model, and `./le spec-session review record` refuses the artifact.
- **A subagent inherits the phase, not the task shape.**
- **The record gate takes `model-override <reason>` only on operator instruction.**
- **Both gates share `internal/le/speclifecycle/model.go`.**
