# Device-side config: priority, last-known-good, auto-revert

`core-design.md` section 21 holds the mechanism: the priority chain `cmdStart`
walks, where the active hash is written, and what the build stores as the
immutable baseline. This page holds the reasoning behind it.

<!-- source: cmd/ze/pushed_config.go -- pushed config loading, validation, active hash -->
<!-- source: cmd/ze/health_revert.go -- HealthRevert, Start, OnPeerClosed, revert -->
<!-- source: cmd/ze/ze_core_start.go -- the wiring that constructs HealthRevert -->

## Decisions

| Decision | Reason |
|----------|--------|
| The pushed config lives at `/perm/ze/config-pushed.conf` | `/perm` is gokrazy's persistent partition, and ZeFS is read-only after the build |
| Validation goes through the normal config load | the same parse and the same YANG validation as any other config. There is no weaker path for a pushed file |
| The pushed-config check runs on every boot, not only on an appliance | the `ENOENT` fast path costs less than a guard that asks whether this is an appliance |
| The last-known-good hash uses the same hash function as the manifest | one hash function, so the build manifest and ZeFS cannot disagree |
| Revert is two-tier: previous pushed config, then the ZeFS seed | one undo before falling back to the immutable seed |
| The health window is 30 seconds | BGP sessions establish in 5 to 10 seconds, so the window still catches a delayed failure |

An invalid pushed config is deleted and logged, so a bad push cannot wedge the
boot path on the next restart.

## Constraint the code does not state

`HealthRevert` reaches the filesystem through package-level function variables,
because the `/perm/ze/` paths do not exist on a development machine. Tests
replace them. The same shape appears in the host diff engine: return a value,
let the caller decide what to do with it, and keep the engine testable with no
bus and no filesystem.

The revert timer is `time.AfterFunc` under a mutex. `OnPeerClosed` stops the
timer and reverts; timer expiry confirms the config.

## Related

- `remote-operations.md` for the bastion side of a config push
- `../core-design.md` section 21 for the loading priority table
