---
kind: directive
level: MUST
stage:
rationale: ai/rationale/no-layering.md
---
**When replacing X with Y, X MUST be deleted first and Y MUST be implemented after. A hybrid, a gradual migration, a fallback to the old path and any other shape that keeps both MUST NOT be written.** Before every change, ask "am I adding or replacing?".
