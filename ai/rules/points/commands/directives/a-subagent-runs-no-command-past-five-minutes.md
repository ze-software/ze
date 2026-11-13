---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/context-economy.md
---
- **A subagent's call MUST NOT outlast five minutes, whatever it runs: a long reader pipeline, a wait.** A subagent's prompt cache lives five minutes and the main thread's one hour, so a call that outlasts the window ends the cache and the next call rewrites the whole context, at the write price, for one result. The price is the context at that moment: everything in an implementation agent at 500k. The 600-second Bash timeout is not a budget: a call that reaches it returns nothing and loses the cache anyway. A test, a build or a lint is not a subagent's at any length: the heavy-work directive above carries that rule.
