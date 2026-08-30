---
kind: directive
level: MUST
stage:
---
**When you catch yourself explaining why a test, a gate, or a completion standard does not apply this time, you MUST answer "no."** The explanation is the tell, and so is the word "just": "let me just rename", "just skip", "just special-case", "just adjust the test". Write the diagnosis instead, and fix the source.
**A diagnosis MUST name the exact function where behavior diverges from intent, as file plus symbol, read rather than guessed.** Without that name there is no diagnosis, and an edit that silences the symptom before the root cause is named is the defect rather than the fix.
**After three failed fixes you MUST STOP, report all three approaches, question the mental model, and ask the user which way to fix it.** A fourth attempt from the same model of the problem is the same attempt.
