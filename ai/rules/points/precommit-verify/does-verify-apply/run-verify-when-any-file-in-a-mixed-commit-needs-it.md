---
kind: directive
level: MUST NOT
stage:
---
**A mixed commit with one YES row MUST be verified, and it MUST NOT be split to
escape that.** The question to answer is "could this make a Go test fail or break
the build?". A no skips the gate and says so in the commit summary. Anything
short of certainty MUST be treated as a yes.
