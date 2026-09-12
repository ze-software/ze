---
kind: directive
level: MUST
stage:
rationale: plan/learned/020-quote-the-requirement-do-not-summarize-it.md
---
**An owner requirement MUST be recorded verbatim on a durable page.** Every spec, comment and test that depends on that requirement MUST point at the page rather than restate it. A summary of a requirement is a second declaration of it, and it drifts like any other copy. Code that agrees with the summary is not evidence, because the code was built from the summary. Where the requirement arrived in conversation, the quote MUST carry its date, and the words MUST NOT be edited. `docs/architecture/config/apply-ordering.md` is the shape: the quote under its own heading, then the design derived from it.
