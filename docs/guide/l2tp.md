# L2TPv2 LNS subsystem

Ze includes a native L2TPv2 (RFC 2661) LNS/LAC implementation used as a
BNG (Broadband Network Gateway) component: it terminates L2TP tunnels
over UDP, runs PPP negotiation (LCP, authentication, IPCP/IPv6CP),
assigns peer IPs, and hands the kernel data plane to the `l2tp_ppp`
module via netlink. Subscriber routes are tracked for redistribution
into the protocol RIB.

This page covers operator-facing use: configuration, the CLI surface,
reload semantics, and how subscriber routes flow out to BGP.

<!-- source: internal/component/l2tp/subsystem.go — Subsystem -->
<!-- source: internal/component/l2tp/schema/ze-l2tp-conf.yang — config shape -->

## Configuration

The `l2tp {}` container carries protocol settings; listener endpoints
live under `environment { l2tp { server ... } }`:

```
l2tp {
    enabled true;
    shared-secret <secret>;     // CHAP-MD5 tunnel auth (RFC 2661 S4.2)
    hello-interval 60;           // seconds of peer silence before HELLO
    max-tunnels 0;               // 0 = unbounded (16-bit ceiling)
    max-sessions 0;              // per-tunnel, 0 = unbounded
}

environment {
    l2tp {
        server main {
            ip 0.0.0.0;
            port 1701;
        }
    }
}
```

Presence of `l2tp {}` with any content implies the subsystem is
enabled. Set `enabled false` to disable explicitly. Listener endpoints
use the standard `zt:listener` grouping (`ip`, `port`) with port
conflict detection via `ze:listener`.

<!-- source: internal/component/l2tp/config.go — ExtractParameters -->

## CLI commands

The CLI surface is two trees: `show l2tp ...` for read-only operations
and a top-level `l2tp ...` for destructive operations.

### Read commands

| Command | Output |
|---------|--------|
| `show l2tp` | Aggregate counters: tunnel count, session count, listener count |
| `show l2tp tunnels` | Array of tunnel summaries |
| `show l2tp tunnel <id>` | One tunnel's detail (AVPs, capabilities, session list) |
| `show l2tp sessions` | Array of session summaries (flattened across tunnels) |
| `show l2tp session <id>` | One session's detail (PPP state, assigned IP, speeds) |
| `show l2tp statistics` | Protocol counters (tunnel/session counts for now; per-message counters land in spec-l2tp-10-metrics) |
| `show l2tp listeners` | Bound UDP endpoints |
| `show l2tp config` | Effective runtime config; `shared-secret` redacted to `<set>`/`<unset>` |

<!-- source: internal/component/cmd/l2tp/l2tp.go — handler registration -->

### Destructive commands

| Command | Effect |
|---------|--------|
| `l2tp tunnel teardown <id>` | Sends StopCCN Result Code 6 (administrative) to the named tunnel |
| `l2tp tunnel teardown-all` | Same, for every live tunnel |
| `l2tp session teardown <id>` | Sends CDN Result Code 3 (administrative) to the named session |
| `l2tp session teardown-all` | Same, for every live session (tunnels remain) |

RFC 2661 S4.4.2 / S5.4.2 define the Result Codes and teardown
semantics. Teardown of unknown IDs returns an error naming the ID.

### Offline dispatcher

`ze l2tp show ...`, `ze l2tp tunnel teardown ...`, and
`ze l2tp session teardown ...` forward to the running daemon via SSH.
Output is the same JSON the daemon handler returns. `ze l2tp decode`
keeps its existing offline wire-decode use.

<!-- source: cmd/ze/l2tp/show.go — offline forwarder -->

## Reload semantics

SIGHUP triggers `Subsystem.Reload`. The implementation diff-applies
each knob according to this policy:

| Field | Reload behaviour |
|-------|------------------|
| `shared-secret` | Hot-apply; takes effect on new SCCRQs. Live tunnels keep the previously-negotiated state. |
| `hello-interval` | Hot-apply; new tunnels use the new interval. Live tunnels keep theirs. |
| `max-tunnels` | Hot-apply at next admission decision. |
| `max-sessions` | Hot-apply at next admission decision. |
| `enabled` flip | Rejected with WARN. Restart to enable/disable. |
| Listener endpoint change | Rejected with WARN. Restart to rebind. |

Rationale: the tunnel FSM carries per-tunnel state (sequence numbers,
kernel fds, PPP sessions). Pushing a new `hello-interval` or new
secret onto an existing tunnel would invalidate in-flight state.
Listener changes require full driver teardown which is safer as an
explicit restart.

<!-- source: internal/component/l2tp/subsystem_reload.go — Reload -->

## Redistribute

Ze registers `l2tp` as a redistribution source at subsystem Start.
When a PPP NCP (IPCP or IPv6CP) completes for a session, the
subsystem's RouteObserver records the assigned peer IP and logs a
`subscriber route inject`. Session-down logs the matching
`subscriber routes withdrawn`.

The actual RIB write path (producing a real BGP UPDATE for the /32 or
/128) requires a programmatic equivalent to the `bgp rib inject` CLI;
that path lands in `spec-l2tp-7c-rib-inject`. spec-l2tp-7 delivers the
source registration, event tracking, and counters.

<!-- source: internal/component/l2tp/redistribute.go — RegisterL2TPSources -->
<!-- source: internal/component/l2tp/route_observer.go — subscriberRouteObserver -->

## Environment variables

Config leaves have matching `ze.l2tp.*` env var overrides registered
via `env.MustRegister()`. See `ze env registered` for the authoritative
list; the relevant entries are:

- `ze.l2tp.enabled`
- `ze.l2tp.shared-secret` (Secret: cleared from process env after first read)
- `ze.l2tp.hello-interval`
- `ze.l2tp.max-tunnels`
- `ze.l2tp.max-sessions`
- `ze.l2tp.ncp.enable-ipcp`, `ze.l2tp.ncp.enable-ipv6cp`, `ze.l2tp.ncp.ip-timeout`
- `ze.log.l2tp`

<!-- source: internal/component/l2tp/config.go — env.MustRegister block -->
