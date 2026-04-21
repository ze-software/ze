# No Layering

**BLOCKING:** When replacing X with Y: DELETE X first, then implement Y. Never keep both.
Rationale: `ai/rationale/no-layering.md`

Forbidden: "keep old + add new", "hybrid approach", "gradual migration", "fallback to old".
Ask "am I adding or replacing?" before every change.
