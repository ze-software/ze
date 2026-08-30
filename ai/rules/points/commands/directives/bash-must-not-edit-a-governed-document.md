---
kind: directive
level: MUST NOT
stage:
rationale: plan/journal/gate-excludes-part-of-its-population.md
---
- **A file under `plan/` or `ai/rules/` MUST NOT be written from Bash: use the Write or Edit tool, which are the only surfaces the native writeedit checks run on.** The guard binds redirects, in-place editors, `tee`, `cp`, `mv` and interpreter writes, and it refuses a command that merely NAMES those trees beside a write primitive. Reading with `grep`, `cat` or `sed -n` stays free.
- **A refusal that is wrong is answered by `ZE_ADMIT_GOVERNED_WRITE="<reason>"`, and MUST NOT be answered by rewording the command.** An empty reason admits nothing. A false positive costs one environment assignment; a false negative costs the guard.
