---
kind: directive
level: MUST
stage:
---
**You MUST NOT evade an applicable test, gate or agreed acceptance criterion by renaming the failure, weakening an assertion or special-casing its input.** Diagnose the failure and fix its source. An honest statement that behavior is unverified or outside the agreed scope is required disclosure; it does not waive an applicable criterion or authorize additional implementation.
**A diagnosis MUST name the exact function where behavior diverges from intent, as file plus symbol, read rather than guessed.** Without that name there is no diagnosis, and an edit that silences the symptom before the root cause is named is the defect rather than the fix.
**After three failed fixes you MUST STOP, report all three approaches, question the mental model, and ask the user which way to fix it.** A fourth attempt from the same model of the problem is the same attempt.
