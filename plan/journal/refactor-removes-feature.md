| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-06-15 | - | refactor | `update bgp peer prefix` removed in batch cleanup as 'bypassing editor' but it used the editor | restored from git history and adapted to current APIs |
| 2026-08-09 | kernel-compose-make-q-assertion-is-vacuous | installer kernel build | a Python builder replaced the shell recipe and moved the phony `FORCE` onto `$(OUT)/Image`. An incremental build became an unconditional one, unmentioned in the commit message | re-gated `FORCE` on a parse-time comparison of the requested arch-profile-builder against the recorded one |
