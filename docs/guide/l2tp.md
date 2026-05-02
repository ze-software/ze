# L2TPv2 LNS subsystem

Ze includes a native L2TPv2 (RFC 2661) LNS/LAC implementation used as a
BNG (Broadband Network Gateway) component: it terminates L2TP tunnels
over UDP, runs PPP negotiation (LCP, authentication, IPCP/IPv6CP),
assigns peer IPs, and hands the kernel data plane to the `l2tp_ppp`
module via netlink. Subscriber routes are tracked for redistribution
into the protocol RIB.

This page covers operator-facing use: configuration, the CLI surface,
PPP negotiation, authentication, IP pools, traffic shaping, RADIUS,
metrics, the web UI, reload semantics, and how subscriber routes flow
out to BGP.

<!-- source: internal/component/l2tp/subsystem.go -->
<!-- source: internal/component/l2tp/schema/ze-l2tp-conf.yang -->

## Configuration

The `l2tp {}` container carries protocol settings; listener endpoints
live under `environment { l2tp { server ... } }`:

```
l2tp {
    enabled true;
    shared-secret <secret>;     // CHAP-MD5 tunnel auth (RFC 2661 S4.2)
    auth-method chap-md5;        // PPP Auth-Protocol first advertised
    allow-no-auth false;         // explicit opt-in required for no-auth
    hello-interval 60;           // seconds of peer silence before HELLO
    max-tunnels 1024;            // 0 explicitly means unbounded
    max-sessions 1024;           // per-tunnel, 0 explicitly means unbounded
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

<!-- source: internal/component/l2tp/config.go -- ExtractParameters -->

## CLI commands

### Read commands

| Command | Output |
|---------|--------|
| `show l2tp` | Aggregate counters: tunnel count, session count, listener count |
| `show l2tp tunnels` | Array of tunnel summaries |
| `show l2tp tunnel <id>` | One tunnel detail (AVPs, capabilities, session list) |
| `show l2tp sessions` | Array of session summaries (flattened across tunnels) |
| `show l2tp session <id>` | One session detail (PPP state, assigned IP, speeds) |
| `show l2tp statistics` | Protocol counters (tunnel/session counts, per-message stats) |
| `show l2tp listeners` | Bound UDP endpoints |
| `show l2tp config` | Effective runtime config; `shared-secret` redacted to `<set>`/`<unset>` |

<!-- source: internal/component/cmd/l2tp/l2tp.go -->

### Destructive commands

| Command | Effect |
|---------|--------|
| `clear l2tp tunnel teardown <id>` | Sends StopCCN Result Code 6 (administrative) to the named tunnel |
| `clear l2tp tunnel teardown-all` | Same, for every live tunnel |
| `clear l2tp session teardown <id> [reason <text...>] [cause <code>]` | Sends CDN Result Code 3 (administrative) to the named session |
| `clear l2tp session teardown-all` | Same, for every live session (tunnels remain) |

The `clear l2tp session teardown` command accepts optional keyword arguments:

- `reason <text...>` -- free-text audit reason, recorded in the per-session event ring
- `cause <code>` -- RADIUS Disconnect-Cause value (uint16), recorded alongside the reason

RFC 2661 S4.4.2 / S5.4.2 define the Result Codes and teardown
semantics. Teardown of unknown IDs returns an error naming the ID.

Destructive commands live under the `clear` verb (not a top-level `l2tp`
noun) to match ze's CLI grammar. The `clear` prefix is denied in the
built-in read-only authz profile.

<!-- source: internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang -->

### Offline dispatcher

`ze l2tp show ...` and `ze l2tp clear ...` forward to the running daemon
via SSH. Output is the same JSON the daemon handler returns. `ze l2tp
decode` is an offline wire-decode tool that does not require a running
daemon.

```
echo c8020044... | ze l2tp decode --pretty
```

<!-- source: cmd/ze/l2tp/show.go -- offline forwarder -->
<!-- source: cmd/ze/l2tp/decode.go -- cmdDecode -->

## PPP negotiation

When a session is established (ICRQ/ICRP/ICCN exchange), the subsystem
creates kernel L2TP tunnel and session resources via Generic Netlink,
then opens a PPPoL2TP socket and attaches a `/dev/ppp` channel and unit.
The kernel creates a `pppN` interface.

PPP negotiation proceeds through these phases:

1. **LCP** (RFC 1661) -- MRU, authentication method (PAP/CHAP-MD5/MS-CHAPv2),
   magic number, echo keepalive. 10-state FSM with ~30 transitions.
2. **Authentication** -- PAP (RFC 1334), CHAP-MD5 (RFC 1994), or
   MS-CHAPv2 (RFC 2759). Dispatched to the registered auth handler
   (l2tp-auth-local or l2tp-auth-radius plugin).
3. **IPCP** (RFC 1332) -- IPv4 address assignment + DNS options (RFC 1877).
   Dispatched to the registered pool handler (l2tp-pool plugin).
4. **IPv6CP** (RFC 5072) -- interface identifier negotiation. Runs in
   parallel with IPCP when both NCPs are enabled.

Each phase has a configurable timeout. LCP proxy (RFC 2661 S18) is
supported: when the LAC provides proxy LCP AVPs, ze validates them
and optionally renegotiates rather than starting LCP from scratch.

NCP enablement is controlled via environment variables:

- `ze.l2tp.ncp.enable-ipcp` (default: true)
- `ze.l2tp.ncp.enable-ipv6cp` (default: true)
- `ze.l2tp.ncp.ip-timeout` (default: 30s) -- how long the NCP phase
  waits for the IP handler response

<!-- source: internal/component/l2tp/reactor.go -- PPP worker dispatch -->

## Authentication

Ze separates PPP authentication wire format from credential validation.
The subsystem handles the PPP auth protocol (PAP/CHAP wire framing),
then dispatches an `EventAuthRequest` to the registered auth handler.
The handler responds with accept/reject via a channel.

By default, new sessions advertise `auth-method chap-md5` and
`allow-no-auth false`. If a peer rejects every acceptable Auth-Protocol,
the PPP session is disconnected after LCP instead of falling through to
the no-auth accounting path. Set `allow-no-auth true` only for lab peers
or explicit no-auth deployments; `auth-method none` is rejected unless
that opt-in is present.

Two auth handlers ship with ze:

### l2tp-auth-local

Built-in static user list with PAP/CHAP-MD5/MS-CHAPv2 support.
Configured under the `l2tp` config tree:

```
l2tp {
    auth {
        local {
            user alice {
                password hunter2;
            }
        }
    }
}
```

When no users are configured, the local handler rejects sessions. Add at
least one user or configure RADIUS before enabling subscriber access.

### l2tp-auth-radius

RADIUS client plugin providing:

- **Access-Request** -- PAP/CHAP-MD5/MS-CHAPv2 credential forwarding to
  RADIUS servers with failover and retry
- **Accounting** -- Start, Stop, and Interim-Update records (RFC 2866)
- **CoA/DM** -- Change of Authorization and Disconnect-Message listener
  (RFC 5176) for RADIUS-initiated session changes and disconnects

Configured under the `l2tp` config tree:

```
l2tp {
    auth {
        radius {
            nas-identifier ze-lns;
            timeout 3;
            retries 3;
            acct-interval 300;
            server 10.0.0.10 {
                port 1812;
                shared-key radiussecret;
            }
        }
    }
}
```

Re-authentication interval is configured via `ze.l2tp.auth.reauth-interval`
(env var, not YANG). When set, sessions are periodically re-authenticated
against the auth handler. A safety floor prevents setting the interval
too low.

- `ze.l2tp.auth.timeout` (default: 30s) -- PPP auth-phase timeout

<!-- source: internal/component/l2tp/handler_registry.go -->
<!-- source: internal/plugins/l2tpauthlocal/ -->
<!-- source: internal/plugins/l2tpauthradius/ -->

## IP address pool

The `l2tp-pool` plugin provides bitmap-backed IPv4 address pools.
Pools are registered via the handler registry and dispatched when
IPCP negotiation needs to assign an address.

Configured under the `l2tp` config tree:

```
l2tp {
    pool {
        ipv4 {
            gateway 10.100.0.1;
            start 10.100.0.2;
            end 10.100.255.254;
            dns-primary 8.8.8.8;
            dns-secondary 8.8.4.4;
        }
    }
}
```

Address allocation currently uses the configured Ze pool. RADIUS
Access-Accept attributes that would change address selection, such as
`Framed-IP-Address`, `Framed-IP-Netmask`, and `Framed-Pool`, are rejected
explicitly rather than ignored.

Session-down events release allocated addresses back to the pool.

<!-- source: internal/plugins/l2tppool/ -->

## Traffic shaping

The `l2tp-shaper` plugin applies TC (traffic control) rules on `pppN`
interfaces. Session establishment uses the configured default rate.
RADIUS CoA can update the rate dynamically after the session is up.

Configured under the `l2tp` config tree:

```
l2tp {
    shaper {
        qdisc-type tbf;            // tbf or htb
        default-rate 10mbit;       // download rate for new sessions
        upload-rate 2mbit;         // upload rate (defaults to default-rate)
    }
}
```

RADIUS Access-Accept attributes that would change policy, such as
`Filter-Id`, `Session-Timeout`, `Idle-Timeout`, and
`Acct-Interim-Interval`, are rejected until Ze implements them exactly.
RADIUS CoA rate updates do not tear down the session.

<!-- source: internal/plugins/l2tpshaper/ -->

## CQM (Call Quality Metrics)

Ze provides Firebrick-style CQM monitoring for L2TP sessions. The
Observer records per-session events and per-login sample rings:

**Per-session event ring** -- circular buffer of state transitions
(tunnel-up, session-up, session-down, echo-rtt, disconnect-requested).
Disconnect events include the actor, reason text, and optional cause
code. Used for the event timeline in the web UI.

**Per-login CQM sample ring** -- 100-second aggregated buckets with:
- Echo RTT statistics (min, avg, max)
- Echo count and loss ratio
- Session state (established, negotiating, down)
- Retention: 24h by default (864 buckets)

CQM data feeds:
- LCP echo probes measure RTT; lost echoes contribute to loss ratio
- Each 100s bucket is closed and appended to the sample ring
- The web UI streams new buckets via SSE for live chart updates

Echo interval for CQM: `ze.l2tp.cqm.echo-interval` (env var, default
derived from LCP echo configuration).

<!-- source: internal/component/l2tp/observer.go -->
<!-- source: internal/component/l2tp/cqm.go -->

## Prometheus metrics

L2TP exposes metrics under the `ze_l2tp_*` and `ze_radius_*` namespaces.

### Aggregate gauges

| Metric | Type | Description |
|--------|------|-------------|
| `ze_l2tp_sessions_active` | gauge | Sessions in established state |
| `ze_l2tp_sessions_starting` | gauge | Sessions in negotiation |
| `ze_l2tp_sessions_finishing` | gauge | Sessions being torn down |
| `ze_l2tp_tunnels_active` | gauge | Active tunnels |

### Per-session counters (labels: username, session_id, interface)

| Metric | Type | Description |
|--------|------|-------------|
| `ze_l2tp_session_state` | gauge | FSM state as integer |
| `ze_l2tp_session_uptime_seconds` | gauge | Seconds since session creation |
| `ze_l2tp_session_rx_bytes_total` | counter | RX bytes on pppN interface |
| `ze_l2tp_session_tx_bytes_total` | counter | TX bytes on pppN interface |
| `ze_l2tp_session_rx_packets_total` | counter | RX packets on pppN interface |
| `ze_l2tp_session_tx_packets_total` | counter | TX packets on pppN interface |

### CQM metrics (labels: username)

| Metric | Type | Description |
|--------|------|-------------|
| `ze_l2tp_lcp_echo_rtt_seconds` | histogram | LCP echo round-trip time |
| `ze_l2tp_lcp_echo_loss_ratio` | gauge | Current 100s bucket echo loss ratio |
| `ze_l2tp_bucket_state` | gauge | CQM bucket state (established=0, negotiating=1, down=2) |

### RADIUS metrics (labels: server)

| Metric | Type | Description |
|--------|------|-------------|
| `ze_radius_up` | gauge | Server reachability (1=up, 0=down) |
| `ze_radius_auth_sent_total` | counter | Access-Request packets sent |
| `ze_radius_acct_sent_total` | counter | Accounting-Request packets sent |
| `ze_radius_interim_sent_total` | counter | Interim-Update packets sent |

Kernel interface stats are polled at `ze.l2tp.metrics.poll-interval`
(default: 30s).

<!-- source: internal/component/l2tp/metrics.go -->
<!-- source: internal/plugins/l2tpauthradius/metrics.go -->

## Web UI

The web interface at `/l2tp` provides session management and CQM
graphing. All endpoints require authentication (session cookie or
Basic Auth).

| URL | Method | Purpose |
|-----|--------|---------|
| `/l2tp` | GET | Session list with sortable columns |
| `/l2tp/<sid>` | GET | Session detail: state, PPP options, CQM chart, event timeline, disconnect |
| `/l2tp/<login>/samples` | GET | CQM buckets as columnar JSON (uPlot data shape) |
| `/l2tp/<login>/samples.csv` | GET | CQM buckets as CSV download |
| `/l2tp/<login>/samples/stream` | GET | SSE stream pushing new CQM buckets every 100s |
| `/l2tp/<sid>/disconnect` | POST | Disconnect session (requires `reason` form field; optional `cause`) |

### CQM chart

The detail page renders a client-side CQM graph using uPlot. The
chart loads 24h of historical data via JSON, then appends new
100-second buckets in real time via SSE.

Chart colors are CSS custom properties (configurable via theme):
- `--color-l2tp-established` (default: green #22c55e)
- `--color-l2tp-negotiating` (default: amber #f59e0b)
- `--color-l2tp-down` (default: purple #a855f7)

### Disconnect

The disconnect button triggers a confirm dialog requiring a free-text
reason (1-256 characters) and an optional Disconnect-Cause code. The
POST dispatches through the CLI as `clear l2tp session teardown <sid>
reason <text> [cause <code>]`, so authz is enforced at the CLI layer.
Read-only profiles are denied by the existing `clear` prefix rule.

The disconnect reason and actor are recorded in the per-session event
ring for audit trail purposes.

<!-- source: internal/component/web/handler_l2tp.go -->
<!-- source: internal/component/web/assets/l2tp-chart.js -->

## Kernel integration

Ze uses the Linux kernel's L2TP and PPP subsystems for the data plane.
Control plane (L2TP control messages, PPP negotiation) runs entirely
in userspace.

**Startup:** the subsystem probes for `l2tp_ppp` and `pppol2tp` kernel
modules via modprobe. If modules are not available, Start fails with a
clear error. Set `ze.l2tp.skip-kernel-probe=true` for testing without
kernel support.

**Per-session kernel setup:**
1. Create L2TP tunnel via Generic Netlink (`L2TP_CMD_TUNNEL_CREATE`)
2. Create L2TP session via Generic Netlink (`L2TP_CMD_SESSION_CREATE`)
3. Create PPPoL2TP socket (binds session to L2TP kernel state)
4. Open `/dev/ppp`, attach channel (`PPPIOCGCHAN`, `PPPIOCATTCHAN`)
5. Create PPP unit (`PPPIOCNEWUNIT`, `PPPIOCCONNECT`)
6. Kernel creates `pppN` interface
7. PPP negotiation runs over the `/dev/ppp` channel fd

**Teardown:** reverse order. PPPoL2TP socket close triggers kernel
session removal. Tunnel is removed after all sessions are gone.

<!-- source: internal/component/l2tp/kernel_linux.go -->

## Redistribute

Ze registers `l2tp` as a redistribution source at subsystem Start.
When a PPP NCP (IPCP or IPv6CP) completes for a session, the
subsystem's RouteObserver emits a `(l2tp, route-change)` batch on the
EventBus with the assigned peer IP as a /32 (IPv4) or /128 (IPv6)
prefix. Session-down emits matching remove batches, one per address
family that was up.

The `bgp-redistribute` plugin discovers L2TP as a producer at startup,
subscribes to its events, and advertises the prefixes to BGP peers when
a matching import rule is configured:

```
redistribute {
    import l2tp {
        family [ ipv4/unicast ipv6/unicast ];
    }
}
```

Each peer's UPDATE carries `origin=incomplete`, an empty AS-path, and
`NEXT_HOP` resolved to the peer's local session address.

<!-- source: internal/component/l2tp/route_observer.go -->

## Reload semantics

SIGHUP triggers `Subsystem.Reload`. The implementation diff-applies
each knob according to this policy:

| Field | Reload behaviour |
|-------|------------------|
| `shared-secret` | Hot-apply; takes effect on new SCCRQs. Live tunnels keep the previously-negotiated state. |
| `hello-interval` | Hot-apply; new tunnels use the new interval. Live tunnels keep theirs. |
| `max-tunnels` | Hot-apply at next admission decision. |
| `max-sessions` | Hot-apply at next admission decision. |
| `auth-method` | Hot-apply to new PPP sessions. |
| `allow-no-auth` | Hot-apply to new PPP sessions. |
| `enabled` flip | Rejected with WARN. Restart to enable/disable. |
| Listener endpoint change | Rejected with WARN. Restart to rebind. |

Rationale: the tunnel FSM carries per-tunnel state (sequence numbers,
kernel fds, PPP sessions). Pushing a new `hello-interval` or new
secret onto an existing tunnel would invalidate in-flight state.
Listener changes require full driver teardown which is safer as an
explicit restart.

<!-- source: internal/component/l2tp/subsystem_reload.go -->

## Environment variables

Config leaves have matching `ze.l2tp.*` env var overrides registered
via `env.MustRegister()`. See `ze env registered` for the authoritative
list; the relevant entries are:

### Logging

- `ze.log.l2tp` -- log level for the L2TP subsystem

### PPP / NCP

- `ze.l2tp.ncp.enable-ipcp` (default: true)
- `ze.l2tp.ncp.enable-ipv6cp` (default: true)
- `ze.l2tp.ncp.ip-timeout` (default: 30s) -- NCP IP handler response timeout

### Authentication

- `ze.l2tp.auth.timeout` (default: 30s) -- PPP auth-phase timeout
- `ze.l2tp.auth.reauth-interval` -- periodic re-auth interval (disabled by default; has a safety floor)

### Metrics

- `ze.l2tp.metrics.poll-interval` (default: 30s) -- kernel stats polling interval

### CQM

- `ze.l2tp.cqm.echo-interval` -- echo probe interval for CQM RTT measurement (read at start; default derived from LCP echo config)

### Test-only

- `ze.l2tp.skip-kernel-probe` (default: false) -- skip modprobe at Start for CLI tests without CAP_NET_ADMIN

<!-- source: internal/component/l2tp/config.go -->
<!-- source: internal/component/config/environment.go -->

## Architecture

The subsystem uses a reactor pattern: a single reactor goroutine reads
the shared UDP socket and dispatches to tunnel state machines. A
separate timer goroutine handles retransmission and HELLO keepalive.
PPP negotiation runs on a worker pool for blocking `/dev/ppp` I/O. No
goroutine-per-tunnel.

```
UDP socket ---> Reactor goroutine ---> Tunnel FSM ---> Session FSM
                                                         |
                                                   Kernel worker
                                                   (Generic Netlink,
                                                    PPPoL2TP socket,
                                                    /dev/ppp)
                                                         |
                                                   PPP worker pool
                                                   (LCP, auth, NCP)
                                                         |
                                                   Observer (events,
                                                    CQM buckets)
```

Four L2TP plugins register at startup via init():
- `l2tp-auth-local` -- static user/password authentication
- `l2tp-auth-radius` -- RADIUS authentication, accounting, CoA/DM
- `l2tp-pool` -- bitmap-backed IPv4 address pools
- `l2tp-shaper` -- TC traffic shaping on pppN interfaces

<!-- source: internal/component/l2tp/reactor.go -->
<!-- source: internal/component/l2tp/observer.go -->
