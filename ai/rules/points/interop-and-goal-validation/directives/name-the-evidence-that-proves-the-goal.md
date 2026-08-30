---
kind: directive
level: MUST
stage:
---
**Before you claim a feature is done, you MUST answer for each spec goal: what concrete evidence proves this goal is achieved, beyond the individual test assertions?** "All tests pass", "AC-1 through AC-5 implemented", "I tested it manually" and "the code looks correct" are not that evidence.
**The spec's Goal Validation table MUST carry one row per stated goal from the Task section, with a concrete reference in the Evidence column and no empty cell.** Interop needs a passing scenario against the named peer daemon, performance a pasted `ze-perf` result, a user workflow a `.ci` or `.et` test over the whole path, data correctness a functional test with explicit hex or JSON assertions, resilience a chaos or fault-injection scenario, and security a negative test whose unauthorized attempt fails with the expected error.
