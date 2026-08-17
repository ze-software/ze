---
kind: directive
level: MUST
stage:
---
**Explicit commit requests are a fast path.** When the user asks for a
commit, the implementation/review phase is over. You MUST prepare the commit
script and run it immediately. You MUST NOT re-audit the implementation, run late
completeness/remaining-work tables, inspect speculative companion artifacts,
or rerun lint/tests just because commit was requested. You MUST inspect only enough
state to avoid staging unrelated, ignored, generated, or out-of-scope paths.
**One check is exempt, because it cannot run earlier: `make ze-repository-tracked-build-check`
after the script has run** (step 7). It judges the commit you just made, which no
run before that commit could see.
