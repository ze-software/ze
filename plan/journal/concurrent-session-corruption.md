| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-04-13 | - | workflow | another session's commit picked up our uncommitted files via shared staging | coordinated manually after identifying file ownership |
| 2026-04-16 | - | workflow | concurrent `make ze-verify` corrupted the shared log file | waited for other session's verify to finish |
