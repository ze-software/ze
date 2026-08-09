# Anomaly Response: Shadow-First Responder

The response tier of the behavioral security chain. It subscribes to the
`anomaly-detect` events described in [anomaly-detect](anomaly-1-detect.md) and
installs a per-source firewall action for an anomalous SOURCE, shadow first.

It mirrors `ddos/local`'s install path and adds the four safety capabilities
`ddos/local` does not have: per-entity arm, timed auto-revert, a blast-radius cap,
and a kill switch. It is a separate security domain: no `ddosevent`, and a
distinct firewall owner key.

## One mutex, whole-set replace

<!-- source: internal/plugins/anomaly/shape/responder.go -- responder state, registerTables -->
<!-- source: internal/plugins/anomaly/shape/match.go -- buildTables -->
<!-- source: internal/plugins/anomaly/shape/match.go -- buildTables -->

All state (the armed map, the armed count, the killed flag, the timers) sits under
one mutex, and `firewall.ApplyAll` runs inside the lock. The event rate is low, so
the lock hold time is not on any hot path.

Every change re-registers the owner `anomaly-shape` from the FULL armed map:
`registerTables(owner, buildTables(...))`. Add and remove are a whole-set replace,
never a partial diff. IPv4 and IPv6 entities go into two tables, because nft
cannot mix families in one table.

## Auto-revert uses a responder-level generation

<!-- source: internal/plugins/anomaly/shape/responder.go -- rearm, autoRevertFire, gen -->

`rearm` bumps a responder-wide `gen` counter and binds the timer to it.
`autoRevertFire(entity, gen)` acts only when `armed[entity].gen == gen`, so a
timer superseded by a re-arm, a clear, the kill switch or `Stop` is a no-op.

The generation MUST be responder-level and not per-record. A per-record counter
resets to 1 when a cleared entity re-arms, so a stale timer from the first arming
matches the new record's generation 1 and withdraws a fresh mitigation.

Auto-revert fires whatever the detector says. It is not conditional on a `Cleared`
event. That is the safety ceiling of the whole responder.

## Injectable clock and firewall

<!-- source: internal/plugins/anomaly/shape/responder.go -- afterFunc, registerTables, applyAll -->

`afterFunc func(time.Duration, func()) stopper` is `time.AfterFunc` in production
and a recording fake in tests, which fires timers on demand. `registerTables` and
`applyAll` are package variables, the same seam `ddos/local` uses. The whole state
machine is therefore unit-testable with no sleep and no kernel.

## The kill switch is the only path

<!-- source: internal/plugins/anomaly/shape/responder.go -- killSwitch -->
<!-- source: internal/plugins/anomaly/shape/register.go -- activate -->

The first version set `killed = cfg.KillSwitch` in `newResponder` and relied on
`activate()` calling `Stop()` to revert. That left `killSwitch()` and its metric
unwired in production: the feature worked through a side path and the named method
was dead.

`newResponder` now starts not-killed, and `activate()` calls `resp.killSwitch()`
when `cfg.KillSwitch` is set, so the method and
`ze_anomaly_shape_killswitch_total` are exercised on the production path.

This is a class of bug worth naming: a feature that works through a side path
leaves the named method and its metric dead. Wire the method as the single path.

## Reconfigure reverts everything

<!-- source: internal/plugins/anomaly/shape/register.go -- activate -->

`activate()` calls the OLD responder's `Stop()` before it builds the new one, so a
config change withdraws every armed action.

## Surface

<!-- source: internal/plugins/anomaly/shape/show.go -- show anomaly shape -->
<!-- source: internal/plugins/anomaly/shape/metrics.go -- shape metric set -->
<!-- source: internal/plugins/anomaly/shape/doctor.go -- anomaly-shape-firewall -->

`anomaly-shape { mode armed }` arms the plugin. Each incident source gets its own
rate limit (`firewall.MatchSourceAddress` plus `Limit`, with a drop fallback) and
a TTL. `show anomaly shape` reports the mode and the armed sources. Metrics are
`ze_anomaly_shape_armed`, `ze_anomaly_shape_reverted_total`,
`ze_anomaly_shape_arm_refused_total` and `ze_anomaly_shape_killswitch_total`. The
doctor check `anomaly-shape-firewall` warns when the plugin is armed and no
firewall backend is loaded.

## Known limit

`reinstall` does not roll back the armed count when `ApplyAll` fails, unlike
`ddos/local`'s drop path. The registry then holds the table while the kernel may
not. The next change re-applies the full set, so the state is eventually
consistent.
