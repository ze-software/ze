---
kind: note
level:
stage:
---
User-facing and runtime failures (doctor, startup, config apply, readiness, plugin
load) must carry a registered code in `internal/core/diagnostic/codes.go` with
title, description, examples, and remediation, explainable via
`ze explain <code>`. Return the code plus structured fields, not a pre-formatted
sentence -- see `ai/rules/evidence.md`. The diagnostic code is what
makes the corrective action machine-readable for an agent.
