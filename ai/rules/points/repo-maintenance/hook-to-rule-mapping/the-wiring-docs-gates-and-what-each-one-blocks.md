---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_ci_sleep_ratchet` | `testing.md` | changed `test/**/*.ci` | Caps how MANY `time.sleep(` calls exist tree-wide against a committed delta baseline. BLOCKING. |
| `check_ci_sleep_justification` | `testing.md` | changed `test/**/*.ci` | Caps how many sleeps are UNEXPLAINED: each needs a comment above or trailing it. BLOCKING. |
| `check_known_failure_load_excuses` | `completion.md` | changed `plan/known-failures/*.md` | Rejects a shard blaming host load ("under load", "loaded host", "load average", "load-sensitive", "passes in isolation", "resource contention", "contended host"). `README.md` / `RESOLVED.md` exempt. BLOCKING. |
| `is_docker_exec_source` | `evidence.md`, `testing.md` | changed `test/**/*.py`, the checker, or the floor | Selects `make ze-docker-exec-check` (`scripts/dev/docker_exec_checked.py`). Caps how many test-harness call sites read a fail-open return value without testing it for emptiness: `docker_exec_quiet` answers `""` on any non-zero exit, so an untested read turns a FAILED command into a passing assertion over nothing. The fail-open set is derived to a fixpoint, so a new wrapper is covered the day it is written. The floor in `test/health/docker-exec-baseline.json` may only go DOWN. `test/draft/` exempt; opt out per site with `# fail-open-ok: <reason>`. BLOCKING. |
| `check_ci_log_subsystem_keys` | `testing.md`, `config.md` | changed `test/**/*.ci` | Rejects a `ze.log.<subsystem>` key whose subsystem contains a hyphen that is not declared literally in Go. An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (every hyphen becomes a dot) and `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level silently stays at the WARN default. Scan is tree-wide; `#` comment lines exempt. BLOCKING. |
