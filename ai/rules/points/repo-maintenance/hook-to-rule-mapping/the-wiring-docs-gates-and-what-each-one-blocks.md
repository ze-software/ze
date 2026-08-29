---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `checkSleepRatchet` | `testing.md` | changed `test/**/*.ci` | Caps how MANY `time.sleep(` calls exist tree-wide against a committed delta baseline. BLOCKING. |
| `checkSleepJustification` | `testing.md` | changed `test/**/*.ci` | Caps how many sleeps are UNEXPLAINED: each needs a comment above or trailing it. BLOCKING. |
| `checkLoadExcuses` | `completion.md` | changed `plan/known-failures/*.md` | Rejects a shard blaming host load ("under load", "loaded host", "load average", "load-sensitive", "passes in isolation", "resource contention", "contended host"). `README.md` / `RESOLVED.md` exempt. BLOCKING. |
| `checkLogSubsystemKeys` | `testing.md`, `config.md` | changed `test/**/*.ci` | Rejects a `ze.log.<subsystem>` key whose subsystem contains a hyphen that is not declared literally in Go. An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (every hyphen becomes a dot) and `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level silently stays at the WARN default. Scan is tree-wide; `#` comment lines exempt. BLOCKING. |
