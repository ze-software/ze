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
<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -->

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
    hello-retries 2;             // unanswered HELLO intervals before dead-peer teardown (0 disables)
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

## Initiator: dial targets and outgoing calls

Beyond answering tunnels, ze can INITIATE them (send SCCRQ) toward a
configured remote. A `remote` list under `l2tp {}` declares dial targets;
declaring one grants no dial by itself. An operator action (RPC) or a PPPoE
relay binding drives the dial.

```
l2tp {
    enabled true;
    remote lns-retail {
        address 203.0.113.10;   // control-plane IP to dial (SCCRQ destination)
        port 1701;              // remote UDP port (default 1701)
        shared-secret <secret>; // per-remote CHAP-MD5 (empty = no Challenge)
        outgoing-calls true;    // permit `request l2tp outgoing-call` to this remote
    }
    relay <service-name> {      // PPPoE Service-Name -> L2TP incoming call (LAC)
        remote lns-retail;      // must reference an l2tp/remote above
    }
}
```

Place an LNS-side outgoing call (RFC 2661 S10.4) with:

```
request l2tp outgoing-call remote lns-retail called <number>
```

The command dials the remote if no tunnel is up, sends an OCRQ, and blocks
until the call establishes (returning the local/remote session IDs) or fails
(returning the cause and RFC 2661 Result Code: tunnel auth reject, tie-breaker
loss, peer CDN, or timeout). The remote must have `outgoing-calls true`.

The `relay` binding is the LAC path: a PPPoE subscriber whose Service-Name
matches is relayed into an L2TP incoming call (ICRQ) toward the bound remote
instead of terminating PPP locally. The subscriber-PPP↔L2TP kernel channel
bridge that carries frames at the LAC is Linux-only and exercised under QEMU
(`make ze-qemu-l2tp-ppp-test`). Initiator tunnel interop is proven against
xl2tpd in `test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns`.

<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- remote/relay lists -->
<!-- source: internal/component/l2tp/cmd/outgoing_call.go -- handleOutgoingCall -->

## Rejecting a malformed SCCRQ

RFC 2661 Section 4.4.3 makes the Assigned Tunnel ID "a 2 octet non-zero
unsigned integer". An SCCRQ that carries zero is a protocol error, and ze
answers it with a StopCCN that carries Result Code 2 (general error, see the
Error Code) and Error Code 3 (a field value out of range). The reply goes out
with Tunnel ID 0, because the peer supplied no tunnel id ze can address it by,
and no tunnel entry is created for it.

The reply is rate-bounded: one StopCCN per source-address slot per second, over
a fixed 256-slot table. A spoofed SCCRQ flood therefore allocates nothing and
draws at most 256 replies per second from the whole reactor. Every other
malformed TunnelID=0 datagram keeps its silent drop.
<!-- source: internal/component/l2tp/reactor.go -- answerZeroTunnelIDSCCRQ, sendUnassociatedStopCCN -->

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

<!-- source: internal/component/l2tp/cmd/l2tp.go -->

### Destructive commands

| Command | Effect |
|---------|--------|
| `clear l2tp tunnel id <id>` | Sends StopCCN Result Code 6 (administrative) to the named tunnel |
| `clear l2tp tunnel all` | Same, for every live tunnel |
| `clear l2tp session id <id> [reason <text...>] [cause <code>]` | Sends CDN Result Code 3 (administrative) to the named session |
| `clear l2tp session all` | Same, for every live session (tunnels remain) |

The `clear l2tp session id` command accepts optional keyword arguments:

- `reason <text...>` -- free-text audit reason, recorded in the per-session event ring
- `cause <code>` -- RADIUS Disconnect-Cause value (uint16), recorded alongside the reason

RFC 2661 S4.4.2 / S5.4.2 define the Result Codes and teardown
semantics. Teardown of unknown IDs returns an error naming the ID.

Destructive commands live under the `clear` verb (not a top-level `l2tp`
noun) to match ze's CLI grammar. The `clear` prefix is denied in the
built-in read-only authz profile.

<!-- source: internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang -->

### Offline dispatcher

`ze l2tp show ...` and the `ze l2tp tunnel|session {id <id>|all}` commands
forward to the running daemon via SSH. Output is the same JSON the daemon
handler returns. Each accepts `--user <name>` (short alias `-u`) to name the
SSH login user; without it the login resolves to the zefs super-admin. The flag
must come before the subcommand arguments (`ze l2tp show --user alice tunnels`),
and one left in the positional tail is rejected rather than forwarded, so a
misplaced `--user` never silently answers for the default user. Shell completion
offers the flag on all three verbs. (Inside the daemon CLI these dispatch as
`clear l2tp tunnel|session {id <id>|all} ...`; `clear` already means tear
down, so no `teardown` token is needed.)
`ze l2tp decode` is an offline wire-decode tool that does not require a
running daemon.

```
echo c8020044... | ze l2tp decode --pretty
```

<!-- source: internal/component/l2tp/cli/show.go -- offline forwarder -->
<!-- source: internal/component/l2tp/cli/decode.go -- cmdDecode -->

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
   parallel with IPCP when both NCPs are enabled. NCPs are independent
   (RFC 1661 S2): if the pool handler declines IPv6 (for example an
   IPv4-only static pool), IPv6CP is dropped and the session stays up
   with IPv4 alone rather than being torn down.

<!-- source: internal/component/l2tp/ppp/ncp.go -- requestIPv6CPInterfaceID declined path -->

Each phase has a configurable timeout. LCP proxy (RFC 2661 S18) is
supported: when the LAC provides proxy LCP AVPs, ze validates them
and optionally renegotiates rather than starting LCP from scratch.

NCP enablement is controlled via the `ncp` container under `l2tp`:

```
l2tp {
    ncp {
        enable-ipcp true
        enable-ipv6cp true
        timeout 30
    }
}
```

<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- ncp container -->

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

Two auth handlers ship with ze. The slot holds one handler, and configuration
decides its owner: `l2tp-auth-radius` claims it when a RADIUS server is
configured, and `l2tp-auth-local` keeps it otherwise. Both transports read that
one slot, so the same rule governs a PPPoE subscriber.
<!-- source: internal/component/l2tp/plugins/authradius/register.go -- activateRadiusConfig claims the slot -->

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
- **Accounting** -- Start, Stop, and Interim-Update records (RFC 2866),
  reporting the subscriber address as Framed-IP-Address
- **CoA/DM** -- Change of Authorization and Disconnect-Message listener
  (RFC 5176) for RADIUS-initiated session changes and disconnects

Every Accounting-Request carries User-Name, Acct-Status-Type, Acct-Session-Id,
Service-Type (Framed), Framed-Protocol (PPP), NAS-Port-Type (Virtual) and
NAS-Port. NAS-IP-Address, NAS-Identifier and NAS-Port-Id are added when they are
configured. Stop and Interim-Update add Acct-Session-Time, the input and output
octet and packet counters, and the RFC 2869 gigaword counters when a counter
passes 2^32.

Framed-IP-Address carries the address the session was actually given, which RFC
2866 Section 4.1 requires of the attribute. The value is the IPCP-negotiated
peer address the reactor put on `pppN`. RFC 2865 Section 5.8 makes the attribute
four octets, so a session with no address yet, or one whose only assignment is
IPv6, sends no attribute rather than a wrong one.
<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- buildAcctPacket -->

Three attributes an operator may look for are deliberately absent, because no
runtime value exists for them: Framed-Interface-Id, Framed-IPv6-Prefix and
Delegated-IPv6-Prefix.

Configured under the `l2tp` config tree:

```
l2tp {
    auth {
        radius {
            nas-identifier ze-lns;
            nas-port-id-format "{nas-id}:{tunnel-id}.{session-id}";
            timeout 3;
            retries 3;
            acct-interval 300;
            coa-port 3799;
            server main {
                address 10.0.0.10;
                port 1812;
                shared-key radiussecret;
            }
        }
    }
}
```

`nas-port-id-format` is the template for the NAS-Port-Id attribute (RFC 2869
Section 5.17). An LNS has no physical port to number, so the operator composes
one from `{nas-id}`, `{tunnel-id}` and `{session-id}`. Every other character is
copied. All three values are known before the session has an interface, so the
Access-Request and every accounting record of one session carry the same text
and a billing system can join them. The text is resolved once per session, so a
config reload does not move it mid-session.

Three templates are refused when the config is committed: one naming a
placeholder that does not exist, one longer than 253 characters (the largest
value a RADIUS attribute can carry), and one using `{nas-id}` with no
`nas-identifier` set. Unset sends no attribute.

`coa-port` enables the UDP Change of Authorization and Disconnect-Message
listener. It has no default: leaving it unset keeps the listener off, so an
upgrade cannot expose a new RADIUS endpoint unexpectedly. Port 3799 is the
standard deployment choice. Ze accepts CoA/DM requests only from addresses
listed under `server`; the authentication and accounting destination port on
each server is configured separately.

<!-- source: internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang -- coa-port -->
<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- source-address validation -->


Authentication timing is controlled via the `authentication` container under `l2tp`:

```
l2tp {
    authentication {
        timeout 30
        reauth-interval 0
    }
}
```

The `timeout` leaf (1-3600 seconds, default 30) bounds the PPP auth phase.
The `reauth-interval` leaf (0 or 5-86400 seconds, default 0) enables periodic
re-authentication when non-zero. Values 1-4 are rejected to prevent re-auth storms.

<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- authentication container -->
<!-- source: internal/component/l2tp/plugins/authlocal/ -->
<!-- source: internal/component/l2tp/plugins/authradius/ -->

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

Address allocation prefers RADIUS metadata when present. `Framed-Pool`
selects a named pool for gateway and DNS values; an unknown named pool rejects
the IPCP request. `Framed-IP-Address` then bypasses bitmap allocation and uses
the selected pool's gateway and DNS with the RADIUS-assigned peer address.
`Framed-IP-Netmask` is parsed into session metadata, but the current IPv4 IPCP
response has no netmask field to apply.

Session-down events release allocated addresses back to the pool.

<!-- source: internal/component/l2tp/plugins/pool/ -->

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

RADIUS `Filter-Id` can override the default shaping rate when it contains a
parseable rate, otherwise Ze keeps the configured default rate. `Session-Timeout`
and `Idle-Timeout` start per-session teardown timers, and
`Acct-Interim-Interval` overrides the accounting update cadence within the
supported clamp range. RADIUS CoA rate updates do not tear down the session.

<!-- source: internal/component/l2tp/plugins/shaper/ -->

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
<!-- source: internal/component/l2tp/plugins/authradius/metrics.go -->

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

Chart colours are CSS custom properties (configurable via theme):
- `--color-l2tp-established` (default: green #22c55e)
- `--color-l2tp-negotiating` (default: amber #f59e0b)
- `--color-l2tp-down` (default: purple #a855f7)

### Disconnect

The disconnect button triggers a confirm dialog requiring a free-text
reason (1-256 characters) and an optional Disconnect-Cause code. The
POST dispatches through the CLI as `clear l2tp session id <sid>
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

The redistribute orchestrator discovers L2TP as a producer at startup,
subscribes to its events, and dispatches the prefixes to registered consumers
(BGP) when a matching import rule is configured:

```
redistribute {
    destination bgp {
        import l2tp {
            family [ ipv4/unicast ipv6/unicast ];
        }
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
| `hello-retries` | Hot-apply to all reactors; affects the dead-peer deadline on the next tick. |
| `max-tunnels` | Hot-apply at next admission decision. |
| `max-sessions` | Hot-apply at next admission decision. |
| `auth-method` | Hot-apply to new PPP sessions. |
| `allow-no-auth` | Hot-apply to new PPP sessions. |
| `authentication/timeout` | Hot-apply to new PPP sessions. |
| `authentication/reauth-interval` | Hot-apply to new PPP sessions. |
| `ncp/enable-ipcp` | Hot-apply to new PPP sessions. |
| `ncp/enable-ipv6cp` | Hot-apply to new PPP sessions. |
| `ncp/timeout` | Hot-apply to new PPP sessions. |
| `enabled` flip | Rejected with WARN. Restart to enable/disable. |
| Listener endpoint change | Rejected with WARN. Restart to rebind. |

Rationale: the tunnel FSM carries per-tunnel state (sequence numbers,
kernel fds, PPP sessions). Pushing a new `hello-interval` or new
secret onto an existing tunnel would invalidate in-flight state.
Listener changes require full driver teardown which is safer as an
explicit restart.

<!-- source: internal/component/l2tp/subsystem_reload.go -->

## Dead-peer detection

A HELLO is sent after `hello-interval` seconds of peer silence and is
delivered reliably, so the peer's ZLB ACK proves the control channel is
alive. Two independent clocks govern an established tunnel:

- **lastActivity** (delivered control messages only) decides *when to send
  the next HELLO*. A ZLB ACK does not refresh it, so HELLOs keep probing a
  quiet peer.
- **lastLiveness** (any delivered message *or* an acknowledgement of one of
  our messages, including a ZLB ACK of a HELLO) decides *when the peer is
  dead*. An idle-but-alive peer that only ZLB-ACKs HELLOs refreshes
  lastLiveness and is never torn down.

`hello-retries` is the dead-peer threshold: when no liveness signal arrives
for `hello-retries x hello-interval`, the tunnel is torn down (sessions
cleared, subscriber routes withdrawn, StopCCN sent, `(l2tp, tunnel-down)`
emitted with reason `keepalive-timeout`). With the default `hello-retries 2`,
a tunnel using `hello-interval 5` is declared dead ~10s after the peer goes
silent -- far faster than the reliable engine's ~31s retransmit exhaustion,
which is the only signal when a peer (e.g. xl2tpd) dies without sending
StopCCN.

Dead-peer detection is deliberately separate from the reliable-transport
retransmit backoff and runs only for established tunnels, so setup
(pre-established) and teardown (closed) retain the full ~31s retransmit
budget for link-loss tolerance. Set `hello-retries 0` to disable dead-peer
detection and fall back to retransmit exhaustion alone. When
`hello-retries x hello-interval` exceeds ~31s (e.g. the defaults
`2 x 60s`), retransmit exhaustion fires first and the threshold has no
effect; lower `hello-interval` to get faster detection.

<!-- source: internal/component/l2tp/reactor.go -- handleTick dead-peer check -->
<!-- source: internal/component/l2tp/tunnel.go -- lastLiveness vs lastActivity -->

## Environment variables

Remaining `ze.l2tp.*` env vars (not promoted to YANG config):

- `ze.log.l2tp` -- log level for the L2TP subsystem
- `ze.l2tp.metrics.poll-interval` (default: 30s) -- kernel stats polling interval
- `ze.l2tp.cqm.echo-interval` -- echo probe interval for CQM RTT measurement
- `ze.l2tp.skip-kernel-probe` (default: false) -- skip modprobe at Start (test-only)
- `ze.l2tp.disable-kernel-dataplane` (default: false) -- build no kernel worker,
  so a session establishes on the control plane and nothing is programmed into
  the kernel (test-only). It is separate from `skip-kernel-probe`, which
  bypasses only the modprobe and still needs the data plane.

PPP authentication and NCP settings are now YANG config leaves under
`l2tp { authentication { ... } }` and `l2tp { ncp { ... } }`.

<!-- source: internal/component/l2tp/config.go -->
<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -->

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
