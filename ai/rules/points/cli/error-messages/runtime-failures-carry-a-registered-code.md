---
kind: directive
level: MUST
stage:
---
**A user-facing or runtime failure (doctor, startup, config apply, readiness, plugin load) MUST carry a registered code in `internal/core/diagnostic/codes.go`, and the handler MUST return that code with structured fields rather than a pre-formatted sentence** (`ai/rules/evidence.md`). What each code holds, and how it reaches an operator, is `docs/architecture/cli/error-surface.md`.
