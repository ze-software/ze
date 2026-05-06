# Learned: spec-l2tp-8c-shaper -- TC Shaper Plugin

## What Was Built

Traffic shaper plugin (`l2tpshaper`) applying TBF or HTB qdiscs to pppN
interfaces on session establishment, removing them on teardown, and
updating rates via CoA-triggered events.

## Key Decisions

- **TBF and HTB only.** Config validation rejects other qdisc types. TBF
  is the default when `qdisc-type` is omitted.

- **Event-driven, not handler-based.** Shaper subscribes to `SessionUp`,
  `SessionDown`, and `SessionRateChange` typed events via EventBus. No
  handler function registration (unlike auth/pool).

- **Rate change via event, not direct CoA.** The RADIUS plugin translates
  CoA bandwidth attributes into `SessionRateChange` events. Shaper only
  sees the event, never touches RADIUS directly. Clean separation.

- **Atomic config pointer.** Hot-reloadable via `atomic.Pointer`. New
  sessions get new config; existing sessions keep their current shaping
  until explicitly changed via CoA.

- **OriginalStateRestorer for cleanup.** If the traffic backend implements
  `OriginalStateRestorer`, shaper calls `RestoreOriginal` on session down.
  Otherwise relies on kernel cleaning up the qdisc when the pppN interface
  is removed.

## Patterns Worth Reusing

- Event subscription for side-effect plugins (shaper, logging, billing)
  keeps them decoupled from the core session lifecycle.
- `atomic.Pointer` for hot-reloadable config is the standard ze pattern
  for plugins that need config without blocking the data path.
