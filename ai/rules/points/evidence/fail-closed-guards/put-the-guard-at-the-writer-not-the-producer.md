---
kind: directive
level: MUST
stage:
---
**A guard MUST sit where the effect leaves the system, not where its input is
produced.** Producers multiply: several call paths build the same value, each
resolves it differently, and none of them is visible from the one you happen to be
editing. Writers are few and closed, because there is only so much code that
touches the socket, the kernel, the disk or the reply.

**A per-producer guard is N copies to keep in step, and the N+1th producer is born
without one.** The count of producers is not knowable from inside a producer, so
"I guarded them all" is a claim that cannot be checked at the moment it is made.
A guard at the writer covers producers that do not exist yet.

**When placing one, name the writers and name the producers, and place it on the
smaller closed set.** When a guard has to sit at a producer, because the writer
cannot see what it needs, MUST say in the code which producers exist and what
makes that set complete, so the next person can check the claim rather than
inherit it.

**Before recording a requirement as met, MUST enumerate every path that can produce
the effect it forbids.** A guard on some of them is a claim about all of them
(`[[one-instance-is-not-a-population]]`).
