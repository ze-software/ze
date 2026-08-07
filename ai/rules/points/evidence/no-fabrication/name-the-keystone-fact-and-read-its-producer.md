---
kind: directive
level:
stage:
---
1. Name the single keystone fact the claim depends on (e.g. "a session-down yields `err == nil`").
2. Read the function that *produces* that fact (returns or sets the value), not only the one that consumes it.
3. If I have read only the consumer, label it a hypothesis, not a finding, and verify before recommending any action.
