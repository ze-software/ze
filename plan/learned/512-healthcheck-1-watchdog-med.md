# 512 -- healthcheck-1-watchdog-med

## Context

The healthcheck plugin needs to control MED values on watchdog routes per health state (UP=100, DOWN=1000, DISABLED=500). The existing watchdog `announce` command only supported pre-computed commands with no runtime MED override. Without this, healthcheck would need multiple watchdog groups per probe to express different MED values.

## Decisions

- Chose `med <N>` literal keyword in the command syntax (`watchdog announce <name> med <N> [peer]`) over positional argument, because a bare number would be ambiguous with peer names.
- MED override bypasses the per-peer `announced` boolean dedup, because the pool tracks state (announced/withdrawn) not command content -- without bypass, MED changes on an already-announced route would be silently dropped.
- Stored Route struct in PoolEntry alongside pre-computed commands, over computing Route from command strings at override time. Route is small and avoids reverse-parsing.
- Rejected "med" as a group name at the parser level for disambiguation.

## Consequences

- Healthcheck can use a single watchdog group with `watchdog announce <group> med <metric>` for all states.
- MED override is transient -- reconnect resend uses stored AnnounceCmd (config MED), not the last override.
- Non-MED path is unchanged: same dedup, same pre-computed commands, zero cost.

## Gotchas

- `AnnouncePool` returns nil for both "pool not found" and "all entries deduped." After adding `GetPool` existence check, the nil-after-AnnouncePool case changed meaning from error to no-op. Required removing the error return.
- The `cmd=api` lines in .ci test files are labels for reporting, not execution directives. Actual commands must be sent by a Python plugin script via `send()`.
- Hex for MED=1000 is `000003E8` (4 bytes), not `0000003E8` (5 bytes, odd length).

## Files

- `internal/component/bgp/plugins/watchdog/server.go` -- MED parsing, handleMEDOverride
- `internal/component/bgp/plugins/watchdog/pool.go` -- Route field on PoolEntry
- `internal/component/bgp/plugins/watchdog/config.go` -- store Route in PoolEntry
- `test/plugin/watchdog-med-override.ci` -- functional test
