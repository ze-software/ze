---
kind: directive
level: MUST
stage:
---
1. You MUST name the single keystone fact the claim depends on (e.g. "a session-down yields `err == nil`").
2. You MUST read the function that *produces* that fact (returns or sets the value), not only the one that consumes it.
3. If I have read only the consumer, I MUST label it a hypothesis, not a finding, and MUST verify before recommending any action.
