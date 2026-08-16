---
kind: directive
level: MUST
stage:
excepted-by: writing/language-and-spelling/prose-written-in-thomas-s-voice-keeps-uk-british-english
---
- **The project language is US English.** Every artifact that is part of Ze, code, docs, and user-facing text, uses US English spelling, wording, and date/number conventions. The single exception is prose authored in Thomas's own voice, which uses UK (British) English.
- **Ze writes in ASD-STE100 Simplified Technical English, Issue 9 (2025-01-15). This is rule one of the repository.** The six habits apply to all project text.
- **STE itself is a GUIDELINE. It is not a law, and it is not a gate.** The English variant, the documentation obligations, and the source anchors in this file are enforced. The STE checker reports and lets the work through.
- **A change MUST update documentation when it changes user or agent behavior, changes an architecture contract, invariant, or documented data flow, makes existing documentation stale, or adds a surface users or agents MUST discover.** A private implementation change that meets none of these triggers requires no prose.
- **Every factual claim in `docs/` MUST be verified against actual code before you write it.** Read the source first, then add the anchor.
- **A product comparison is advice, not marketing.** Every claim MUST help the reader choose the right tool, and MUST NOT make Ze look better.
