---
kind: note
level:
stage:
rationale: plan/journal/concurrent-session-corruption.md
---
This is a false positive, not a rule you are violating, and it bites exactly
when auditing commit scripts, which is what the shared-file sharding work spent
its time doing.
Do not rephrase the ban away or work around the hook's intent. Scan with
Python instead, which keeps the verb out of the command line:
