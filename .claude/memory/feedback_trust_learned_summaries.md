---
name: Don't trust learned summaries about what's impossible
description: Learned summaries may incorrectly claim something requires a design change when the mechanism already exists -- verify claims about missing infrastructure against actual code before reporting them
type: feedback
---

Learned summaries can contain wrong claims about what's "deferred" or "requires X change."
Example: 401-role-otc.md said "Egress OTC stamping requires EgressFilterFunc signature change"
when ModAccumulator was already in the signature for exactly this purpose.

**Why:** The summary was written by a session that didn't realize the existing mechanism
(ModAccumulator) was the intended path for egress modifications. Future sessions trusted
the summary and repeated the wrong claim without checking the actual function signature.

**How to apply:** When a learned summary says something is "deferred because X is missing" or
"requires Y change," verify the claim against actual code before reporting it to the user.
Read the function signature, check the types, look at the docs. Don't parrot deferred-item
descriptions from summaries -- they may describe a problem that doesn't exist.
