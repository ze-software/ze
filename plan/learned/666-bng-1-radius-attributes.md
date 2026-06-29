# 666 -- bng-1 RADIUS Attribute Consumption

## Context

Ze's RADIUS auth handler rejected L2TP sessions when the RADIUS server returned subscriber profile attributes (Framed-IP-Address, Framed-Pool, Session-Timeout, Idle-Timeout, Filter-Id, Acct-Interim-Interval) in Access-Accept. The `unsupportedAccessAcceptAttrs` map caused a hard rejection, making Ze unsuitable as a production BNG where RADIUS profiles are the standard provisioning mechanism. The goal was to remove the rejection logic and wire each attribute to its consumer.

## Decisions

- Chose sync.Map metadata store over extending AuthResult signature, because multiple independent consumers (pool, reactor, shaper, acct) need the same data without changing the auth respond callback contract.
- Chose goroutine-per-session timers over a central timer wheel for Session-Timeout/Idle-Timeout, because the expected session count (low thousands) does not justify the complexity of a shared wheel.
- Chose Linux sysfs `/sys/class/net/<iface>/statistics/rx_bytes` for idle traffic detection over netlink stats, because it requires no socket and works from any goroutine. Non-Linux returns 0 (timer fires unconditionally).
- Deferred AC-2 (Framed-IP-Netmask applied to pppN interface) to bng-3, because PPP is point-to-point and the netmask only matters for delegated-prefix routing.
- Chose `clampAcctInterval` [60,3600]s over trusting the RADIUS value blindly, to prevent accounting storms from misconfigured servers.

## Consequences

- Named pools (`named-pool` YANG list) are now available for any future feature that needs pool selection, not just Framed-Pool.
- Session-Timeout/Idle-Timeout infrastructure can be reused for any future per-session timer (e.g., Acct-Session-Time monitoring).
- `AuthMetadata` is the canonical carrier for RADIUS profile data. Future attributes (Delegated-IPv6-Prefix, Class) should extend this struct.
- bng-3 (IPv6 pools) depends on the metadata store established here.
- The pool handler's `sessionAddr.fromPool` flag is load-bearing for teardown: RADIUS-assigned addresses must not be released back to the pool.

## Gotchas

- The metadata store key is (tunnelID, sessionID). Session IDs are per-tunnel, so the compound key is necessary. A plain sessionID key would collide across tunnels.
- `cancelSessionTimeouts` must be called while holding `tunnelsMu` (inside teardown), but `startSessionTimeouts` must NOT hold `tunnelsMu` when starting goroutines (deadlock risk if the goroutine immediately calls TeardownSessionByID).
- Filter-Id rate parsing supports both `rate:20mbit/5mbit` (prefixed) and bare `20mbit/5mbit` formats because RADIUS servers vary. The `rate:` prefix is optional.
- `ClearSessionMetadata` must be called in BOTH teardown paths (session teardown and tunnel teardown) to prevent sync.Map leaks.

## Files

- `internal/component/l2tp/plugins/auth_radius/extract.go` -- attribute extraction, IP validation
- `internal/component/l2tp/session_metadata.go` -- per-session AuthMetadata store
- `internal/component/l2tp/session_timeout.go` -- Session-Timeout/Idle-Timeout goroutines
- `internal/component/l2tp/iface_stats_linux.go` / `iface_stats_other.go` -- traffic detection
- `internal/component/l2tp/plugins/pool/register.go` -- named pools, Framed-IP bypass, sessionAddr tracking
- `internal/component/l2tp/plugins/shaper/filter_rate.go` -- Filter-Id rate parsing
- `internal/component/l2tp/plugins/auth_radius/acct.go` -- per-session Acct-Interim-Interval
