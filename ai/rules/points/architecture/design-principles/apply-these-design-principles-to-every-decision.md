---
kind: directive
level: MUST
stage:
rationale: ai/rationale/design-principles.md
---
**One responsibility per function, struct and package: a name joining two with `And` MUST be split, and a wrapper that transforms nothing MUST be deleted so callers pass the data and use the existing type's methods.** Four Ze-specific principles govern the rest and each names the rule that owns its detail: the simplest FULLY correct answer and nothing beyond it (`ai/rules/simplicity.md`), a backend that cannot apply config EXACTLY fails verify or commit rather than approximating (`ai/rules/protocol.md`), buffer-first encoding through the encapsulation onion with no `make` where a pool exists (`ai/rules/performance.md`), and lazy over eager on the read side, raw bytes plus offset iterators rather than parsed structs.
