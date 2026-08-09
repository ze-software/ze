# RADIUS subscriber-profile attributes

A broadband network gateway provisions subscribers from RADIUS. Ze's auth
handler once rejected any Access-Accept that carried a profile attribute, which
made it unusable in that role. Each attribute now reaches its consumer:
Framed-IP-Address, Framed-Pool, Session-Timeout, Idle-Timeout, Filter-Id and
Acct-Interim-Interval.

<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- extractAuthMetadata, extractFramedRoutes, isValidSubscriberIP -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata, StoreSessionMetadata, LoadSessionMetadata, ClearSessionMetadata -->
<!-- source: internal/component/l2tp/session_timeout.go -- startSessionTimeouts, runSessionTimeout, runIdleTimeout, cancelSessionTimeouts -->
<!-- source: internal/component/l2tp/plugins/shaper/filter_rate.go -- parseFilterRate -->

## Decisions

**A `sync.Map` metadata store, not a wider auth result.** The pool, the reactor,
the shaper and the accounting path all need the same data. Extending the auth
respond callback signature would have changed a contract that four independent
consumers depend on.

**A goroutine per session timer, not a central timer wheel.** The expected
session count is in the low thousands. A shared wheel costs complexity that the
count does not justify.

**Idle detection reads the Linux sysfs interface statistics.** The receive-byte
counter under the interface's statistics directory needs no socket and is
readable from any goroutine. Netlink statistics were rejected for that reason.
The non-Linux build returns 0, so the timer fires unconditionally there.

<!-- source: internal/component/l2tp/iface_stats_linux.go -- interface byte counters from sysfs -->
<!-- source: internal/component/l2tp/iface_stats_other.go -- non-Linux stub -->

**The accounting interval is clamped to 60 to 3600 seconds.** A misconfigured
RADIUS server would otherwise drive an accounting storm.

**Framed-IP-Netmask is not applied to the PPP interface.** PPP is point to
point, so the netmask only matters for delegated-prefix routing. That belongs
with the IPv6 pool work, not here.

## Consequences worth knowing

- `AuthMetadata` is the canonical carrier for RADIUS profile data. A new
  attribute such as Delegated-IPv6-Prefix or Class extends that struct.
- Named pools are available to any feature that needs pool selection, not only
  to Framed-Pool.
- The pool handler's from-pool flag is load-bearing at teardown. An address
  assigned by RADIUS must not be released back into the pool.

## Traps this code exists to avoid

**The metadata key is the tunnel id AND the session id.** Session ids are per
tunnel, so a plain session id collides across tunnels.

**The two timeout entry points have opposite locking rules.** Cancelling must
happen while the tunnel lock is held, inside teardown. Starting must NOT hold
that lock, because the new goroutine can immediately call teardown by session
id, which deadlocks.

**Metadata must be cleared on BOTH teardown paths.** Session teardown and tunnel
teardown each reach the store. Missing either one leaks map entries.

**Filter-Id rate accepts two spellings.** RADIUS servers vary, so both
`rate:20mbit/5mbit` and the bare `20mbit/5mbit` parse. The prefix is optional.
