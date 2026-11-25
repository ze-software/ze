---
kind: directive
level: MUST
stage:
---
- **Separate observed facts, derived conclusions, and inferences in every report, audit, and spec.** An observed fact comes directly from read source or observed tool output; a derived conclusion follows mechanically from observations; an inference is judgment. You MUST label every inference as inferred rather than presenting it beside observed facts as settled.
- **Carry search boundaries and truncation limits forward.** When a search, walk, or audit hits a row cap, skips build-tagged files, or inspects only a subset, the report MUST name what was excluded and MUST NOT summarize a partial scan as complete.
