# 1049 - Anomaly Response (anomaly/shape): shadow-first autonomous responder

**Spec:** spec-anomaly-2-shape (closed) | **Date:** 2026-07-02 | **Depends:** 1048 (anomaly/detect)

## Context

The response tier of the Darktrace-style spine: subscribes to Spec 2a's `anomaly-detect`
events and, for an anomalous SOURCE, installs a surgical per-source firewall action --
SHADOW-FIRST. Mirrors `ddos/local`'s alert/enforce install path but adds the four safety
capabilities ddos/local lacks: per-entity arm, timed auto-revert, blast-radius cap,
kill-switch. Separate security domain (no `ddosevent`, distinct firewall owner key).

## Key Decisions

- **Pinned Responder State Machine, one mutex.** All state (armed map, armedCount, killed,
  timers) is under one mutex; the firewall `ApplyAll` happens inside the lock (~event rate is
  low). The owner `"anomaly-shape"` is re-registered from the FULL armed map on every change
  (`registerTables(owner, buildTables(...))`) -- add/remove is a whole-set replace, no partial
  diffs. v4/v6 entities go into two tables (nft cannot mix families in one).
- **Timed auto-revert with a RESPONDER-LEVEL monotonic generation guard.** `rearm` bumps a
  responder-wide `gen` counter and binds the timer to it; `autoRevertFire(entity, gen)` acts
  only if `armed[entity].gen == gen`, so a superseded timer (re-arm / clear / kill-switch /
  Stop) is a no-op. The gen MUST be responder-level, not per-record: a per-record counter
  (review pass 2 finding) resets to 1 when a cleared entity re-arms, so a stale timer from the
  first arming could match the new record's gen 1 and withdraw the fresh mitigation. Auto-revert
  fires REGARDLESS of a Cleared event -- the safety ceiling. Regression: `TestStaleTimerAcrossReArmNoop`.
- **Injectable clock + mockable firewall for tests.** `afterFunc func(d, f) stopper` (prod:
  `time.AfterFunc`; test: records timers, fired on demand) and package vars
  `registerTables`/`applyAll` (mirrors ddos/local) make the whole state machine unit-testable
  with no real sleep and no kernel.
- **Kill-switch is the canonical path (review fix).** First cut set `killed = cfg.KillSwitch`
  in `newResponder` and relied on `activate()`'s `Stop()` to revert -- which left `killSwitch()`
  (and its metric/log) unwired in production. Fixed: `newResponder` starts not-killed and
  `activate()` calls `resp.killSwitch()` when `cfg.KillSwitch`, so the method + the
  `ze_anomaly_shape_killswitch_total` metric are actually exercised.
- **Reconfigure reverts all (AC-9).** `activate()` calls the OLD responder's `Stop()` before
  building the new one, so a config change withdraws every armed action.

## Consequences

- `anomaly-shape` is a report-consuming ACTOR plugin: `anomaly-shape { mode armed }` arms it;
  each incident source gets its own rate-limit (`firewall.MatchSourceAddress`+`Limit`, drop
  fallback) with a TTL. `show anomaly-shape` shows mode + armed sources. Metrics
  `ze_anomaly_shape_{armed,reverted_total,arm_refused_total,killswitch_total}`; doctor
  `anomaly-shape-firewall` (+ diagnostic code) warns on armed-without-firewall.
- The Darktrace-style fact -> judgment -> response spine is now complete across three specs
  (1046 facts, 1048 judgment, 1049 response), each an independent domain-clean layer.

## Gotchas

- **killSwitch-unwired class of bug**: a feature that "works" via a side path (Stop + a flag)
  can leave the named method + its metric dead. Wire the method as the single path.
- **reinstall does not roll back armedCount on an ApplyAll failure** (unlike ddos/local's
  drop path). The registry holds the table but the kernel may not; the next change re-applies,
  so it is eventually consistent. Documented limitation, not a correctness bug for slice-one.
- Emit/arm path is not functionally .ci-testable without the detector + traffic; the state
  machine is fully unit-tested (fake clock + mocked firewall); `anomaly-shape-shadow.ci` proves
  the config -> RunEngine -> setGlobalResponder -> show wiring.
- Concurrent-OSPF known-reds (same as 1046/1048): plugin/all snapshot + ze-doc-test drift are
  the OSPF sessions'; verify MY additions accepted and leave theirs. Allocate learned numbers
  via `commit_helper.py learned-next` (the .counter races across sessions).
- `strings +` concatenation for a term name trips no-sprintf-alloc -> use `textbuf.Buffer`.

## Files

None recorded.
