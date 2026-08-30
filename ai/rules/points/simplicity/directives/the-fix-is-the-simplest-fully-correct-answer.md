---
kind: directive
level: MUST
stage:
rationale: ai/rationale/simplicity.md
excepted-by: simplicity/directives/complexity-a-rule-requires-is-not-over-engineering
---
**A fix MUST be the simplest solution that is fully correct, it MUST sit at the ROOT of the defect, and nothing in the change MUST exist for a problem you were not asked to solve.** A special case bolted onto shared infrastructure is not the simpler option: it adds a branch AND leaves the defect live for every caller the special case does not name.
**"Simplest" is measured in what the next developer holds in their head, never in line count: they meet the code cold and say what it does and where to change it, in about 30 seconds, with no second file open.** A dense expression they have to simulate fails that measure exactly as a five-file framework does, so write the version that is boring to read.
