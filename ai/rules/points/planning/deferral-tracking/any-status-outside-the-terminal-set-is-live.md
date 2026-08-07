---
kind: note
level:
stage:
---
`open` and `deferred` are synonyms, and the redundancy is a wart: it is what let the
gate and this rule teach different words in the first place. Do not add a third.
**Any word that is not in the terminal set is treated as live and checked**,
deliberately: the gate is a denylist of terminal states, not an allowlist of live
ones, so a status nobody has invented yet fails closed rather than slipping through
silently (`ai/rules/evidence.md`).
