---
kind: directive
level: MUST
stage:
---
**No gate reads a config example today, and the gate that lands here MUST gate what ze's own parser recognizes as a config attempt, with an opt-OUT that states its reason on the block.** Gating every fenced block in `docs/` would fire mostly on deliberate excerpts, an estimated four in five, because they start mid-tree or carry a placeholder. An opt-IN marker inverts the failure: every example already refused stays unmarked and uncaught, which is the `rpki.md` case exactly.

**MUST state that cost when proposing it, rather than sell the gate.** Somebody annotates the excerpts once, and each new excerpt pays one line. `ai/rules/repo-maintenance.md` owns registering the gate and its row in the hook mapping. `plan/spec-ze-config-fmt.md` owns how an example is RENDERED, so cross-reference it: a formatter decides the shape, and this decides whether the shape parses.
