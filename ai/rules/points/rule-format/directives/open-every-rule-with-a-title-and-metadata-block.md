---
kind: directive
level: MUST
stage:
---
**Every rendered rule MUST open with a title and a machine-readable metadata block**, so tooling parses triggers and severity without guessing. The manifest frontmatter carries both, and the renderer emits them.
**An ALL-CAPS stem is a generated artifact, never a rule**: `INDEX.md`, `TRIGGERS.md`, `CORE.md`. The generators skip it by that shape, so a new artifact needs no code change.
