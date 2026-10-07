# Memory Lock

## Meta

| Field | Value |
|-------|-------|
| Name | Memory Lock |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/memlock, internal/plugins/systemd/unit.go |
| Real-path tests | test/parse/show-plugin-list-memlock.ci |
| Docs | docs/guide/plugins.md, docs/guide/status.md |
| Doc review | 2026-10-07: MLOCK_ONFAULT use, doctor-memlock-rlimit-low code and the CAP_IPC_LOCK exemption read in internal/plugins/memlock |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/plugins/memlock |
| Extra criteria | supported: show plugin list reports memlock succeeded under a lifted RLIMIT_MEMLOCK = test/plugin/memlock-succeeded.ci |

## Description

The `memlock` plugin locks the running ze executable's file-backed pages with `mlock2(2)` and `MLOCK_ONFAULT`, so the kernel cannot evict the daemon's own text under memory pressure and fault it back from disk. Linux only. The lock is taken in the package `init()`, before `main()`, so every ze process holds it from its first instruction. The whole mapped size is charged against `RLIMIT_MEMLOCK`, which the generated `ze.service` unit lifts with `LimitMEMLOCK=infinity`; a limit too small for the binary makes the lock fail, and `show plugin list` reports the `memlock` row as `soft-failure` carrying the cause and the remedy. The failure is soft: the daemon serves every session correctly with an unlocked executable. A `ze doctor` check answers the question the record cannot, before ze runs: it compares `RLIMIT_MEMLOCK` against the size of `/proc/self/exe` and warns under `doctor-memlock-rlimit-low` when the limit cannot hold the executable, staying silent for a process that holds `CAP_IPC_LOCK`. <!-- source: internal/plugins/memlock/memlock_linux.go -- init, the mlockexe.OnFault call and the RecordSetup that reports it --> <!-- source: internal/plugins/memlock/doctor_linux.go -- checkMemlockLimit, the pre-flight rlimit probe --> <!-- source: internal/plugins/systemd/unit.go -- buildUnitFile, LimitMEMLOCK=infinity -->
