---
kind: directive
level: MUST
stage:
---
**Round 1 reviews the whole diff with at least two lenses; round N+1 reviews only the fixes round N made plus the sibling call sites they touched, and each round's scope MUST be written into the spec's Review Gate section before it runs**, or it shrinks to whatever produces a clean round. The loop ends when a round finds no BLOCKER and no ISSUE inside its own scope AND no always-in-scope finding anywhere. A NOTE MUST NOT re-open a round, wherever it was found.
**Reviews MUST treat these findings as at least ISSUE when they affect the agreed capability or acceptance criteria:** an unwired symbol, a vacuous test, missing entry-point or Linux/QEMU proof, a removed guard, a fail-open guard, and RFC or interoperability defects. An absent requirement of an implemented RFC is a finding like any other: the review records it and `ai/rules/rfc-compliance.md` makes it owed work (M2).
**A review MUST apply M30 (`ai/META-DECISIONS.md`) when deciding whether an absent RFC feature blocks the first release.** A coverage gap alone MUST NOT fail release review; a requirement needed for correct implemented behavior remains a blocker.
**A finding outside the round's scope MUST be fixed when the agreed goal depends on it.** Otherwise, record a defect in `plan/journal/<class>.md` or an absent RFC requirement in its RFC summary. Unclear feature boundaries MUST be resolved before implementation.
