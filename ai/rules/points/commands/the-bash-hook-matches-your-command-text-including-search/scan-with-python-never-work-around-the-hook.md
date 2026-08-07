---
kind: note
level:
stage:
rationale: plan/learned/1244-fixit-shared-plan-file-contention.md
---
This is a false positive, not a rule you are violating, and it bites exactly
when auditing commit scripts (see
`plan/learned/1244-fixit-shared-plan-file-contention.md`, the sharding work
that hits this).
Do not rephrase the ban away or work around the hook's intent. Scan with
Python instead, which keeps the verb out of the command line:
