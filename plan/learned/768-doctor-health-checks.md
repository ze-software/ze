# 768: Extended Doctor and Runtime Health Checks

**Spec:** `plan/spec-doctor-health-checks.md`
**Commits:** 95353888c (phase 1-6), 80e696547 (phase 7-12)

## What Was Built

Two-tier health monitoring: offline `ze doctor` checks (config semantics,
environment probes) and runtime anomaly detection via the report bus and
health registry.

21 acceptance criteria across 6 categories: offline doctor (AC-1 through AC-7),
BGP session health (AC-8 through AC-11), FIB sync (AC-12 through AC-14),
firewall drift (AC-15/16), infrastructure health (AC-17 through AC-19),
health registry extensions (AC-20/21).

## Key Decisions

- **Prefix counting unconditional.** Originally gated on PrefixMaximum
  configuration. Made unconditional to support route-count-anomaly detection
  on all peers. Added early-return guard in `applyPrefixDelta` to skip
  `familyString` allocation when neither metrics nor warnings are configured.

- **Minimum threshold for anomaly.** 100-prefix floor prevents false positives
  on small tables (route-server clients, management peers with 1-5 routes).

- **Per-family EOR tracking.** Single timer per peer, but tracks expected vs
  received family count. Warning only clears when all negotiated families
  have sent EOR, not on first EOR.

- **Health checks as warning producers.** `checkFirewallHealth` calls
  `AuditTables()` and `checkIfaceHealth` calls `CheckAllInterfaceErrors()`
  to produce warnings before reading them. Kernel calls run in a goroutine
  with 1-second timeout.

- **VPP health gated on socket existence.** `os.Stat` check before dialing
  prevents permanent 503 on non-VPP systems.

- **FIB pending map capped.** `maxPendingEntries = 10000` prevents unbounded
  growth on catastrophic backend failure.

## Mistakes and Corrections

| Mistake | How caught | Fix |
|---------|-----------|-----|
| AuditTables false-positive before first Apply | Critical review | Guard on `LastApplied() == nil` |
| plugin-crash error before validation | Critical review | Move after started/disabled/not-found guards |
| EOR timer created with familyCount=0 | Critical review | Guard on `familyCount == 0` |
| VPP health always 503 on non-VPP | /ze-review pass 4 | Gate on socket existence |
| AuditTables/CheckInterfaceErrors unwired | /ze-review pass 3 | Wire via health check functions |

## Files

### New
- `internal/component/firewall/audit.go` -- firewall drift audit
- `internal/component/iface/health.go` -- interface error counter tracking
- `internal/component/cmd/show/health_checks.go` -- health registry check functions
- `docs/guide/health-checks.md` -- operator guide

### Modified (key)
- `internal/component/bgp/reactor/session_health.go` -- EOR timeout
- `internal/component/bgp/reactor/session_prefix.go` -- route count anomaly
- `internal/plugins/fib/kernel/fibkernel.go` -- FIB sync failure/orphan/lag
- `internal/component/plugin/process/manager.go` -- plugin crash/down
- `internal/component/doctor/doctor.go` -- clock skew check
- `internal/component/doctor/checks_linux.go` -- VPP version check
