---
kind: directive
level: MUST
stage:
---
- **A non-lifecycle helper that always-on code needs MUST be extracted to an always-on `internal/core/*` leaf BEFORE the feature is gated.** Extract-then-gate is the order, and the registry work is the easy half.
