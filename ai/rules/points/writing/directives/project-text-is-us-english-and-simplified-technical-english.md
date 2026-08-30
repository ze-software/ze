---
kind: directive
level: MUST
stage:
rationale: ai/rationale/writing.md
---
**Project text MUST be US English AND Simplified Technical English (ASD-STE100 Issue 9), and prose written in Thomas's voice MUST be UK English.** Text ABOUT Ze is US English: Go identifiers, comments and godoc, error messages, CLI output, YANG descriptions, `docs/`, `ai/`, `plan/`, and commit messages. Text that IS Thomas speaking is UK English (`colour`, `behaviour`, `licence` as a noun): blog posts, articles, essays, emails, and the weekly update, all produced under the `/write` skill. A new identifier, comment, doc or error string MUST be US English with no exceptions, and UK spelling that has drifted into `docs/` MUST be fixed opportunistically in a file you already touch, never by a global replace, because some occurrences are quoted external text.
**STE is a GUIDELINE. It is not a law, and it is not a gate.** The checker reports and lets the work through, so apply a finding when it makes the text clearer and never rewrite a sentence only to satisfy a count. The six habits, the word choices, the sentence limits, the detail budget, the comparison rules and the config-example rules are in `docs/contributing/writing-style.md`. Read that page before documentation work, a deep prose review, or resolving an STE finding.
