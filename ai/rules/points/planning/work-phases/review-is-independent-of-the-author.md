---
kind: directive
level: MUST
stage:
---
**Review is independent of the author.** A different model is not a different
context. A fresh session, a phase agent spawned after the
implementing phase ended, or reviewer subagents MUST be used, and the context
that wrote the code MUST NOT sit in judgment on it. Any one of the three
satisfies the guarantee, so the review MUST NOT be spawned again from a context
that already meets it -- `/ze-close` is the case that bites, and it MUST run the
review itself (owner directive, 2026-08-15).
