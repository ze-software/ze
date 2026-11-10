---
kind: directive
level: MUST
stage:
rationale: ai/rationale/user-facing-errors.md
---
- **A refusal or failure an operator sees MUST be an operator error (`internal/core/opererr`): the problem, why it matters, and the fix, which is the exact command alone on the last line, with nothing appended after it.** A daemon command that answers an error response MUST build it with `plugin.OperatorErrorResponse`. A layer that adds context MUST add it to the first line only, and MUST NOT repeat a prefix the wrapped error already carries. `./le arch operator-error check` and `./le arch error-wrap check` refuse a changed line that breaks these; how each surface renders an operator error is in `docs/architecture/cli/error-surface.md`, "The operator error", and the wrap rule is in `docs/contributing/ze-go-style.md`, "Every error is handled".
