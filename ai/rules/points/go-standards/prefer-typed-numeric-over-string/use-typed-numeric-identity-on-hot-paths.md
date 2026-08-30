---
kind: directive
level: MUST
stage:
rationale: ai/rationale/enum-over-string.md
---
**A hot path MUST carry typed numeric identity: an enum, a registered ID, a bitset, or a packed integer. It MUST NOT carry a string identity.** Across a component or engine seam the same rule holds, plus the pointer restrictions in `ai/rules/repo-maintenance.md`.
**The surfaces, the boundaries where a string IS correct, the acceptable `map[string]V` cases, and the anti-patterns with their fixes are in `docs/contributing/go-conventions.md`.**
