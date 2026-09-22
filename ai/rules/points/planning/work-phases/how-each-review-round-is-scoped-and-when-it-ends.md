---
kind: directive
level: MUST
stage:
---
**Round 1 reviews the WHOLE diff with at least two lenses; round N+1 reviews ONLY the fixes round N made plus the sibling call sites they touched, and each round's scope MUST be written into the spec's Review Gate section BEFORE it runs**, or it shrinks to whatever produces a clean round. The loop ends when a round finds no BLOCKER and no ISSUE inside its OWN scope AND no always-in-scope finding anywhere. A NOTE MUST NOT re-open a round, wherever it was found.
**Reviews MUST treat these findings as at least ISSUE when they affect the agreed capability or acceptance criteria:** an unwired symbol, a vacuous test, missing entry-point or Linux/QEMU proof, a removed guard, a fail-open guard, and RFC or interoperability defects. An absent RFC feature follows the gap policy in `ai/rules/rfc-compliance.md` and MUST NOT expand the review into implementation.
**A finding outside the round's scope MUST be fixed when the agreed goal depends on it.** Otherwise, record a defect in `plan/journal/<class>.md` or an absent RFC requirement in its RFC summary. Unclear feature boundaries MUST be resolved before implementation.
