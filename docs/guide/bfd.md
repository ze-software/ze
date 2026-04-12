# BFD — Bidirectional Forwarding Detection

**Status:** the plugin is live, the production transport is hardened
(GTSM, IP_TTL=255 outbound, SO_BINDTODEVICE for single-hop and multi-VRF,
RFC 5880 §6.8.7 TX jitter), the BGP peer opt-in is wired through the
reactor lifecycle, and the Stage 4 operator surface (`show bfd
sessions`, `show bfd session <peer>`, `show bfd profile`, Prometheus
`ze_bfd_*` metrics) is available. Adding `bfd { ... }` inside a
`bgp peer connection` block opens a per-peer BFD session on
Established and tears the BGP session down with RFC 9384 Cease
subcode 10 ("BFD Down") when BFD reports the forwarding path lost.
<!-- source: internal/plugins/bfd/bfd.go — runtimeState, loopFor, newUDPTransport, pluginService -->
<!-- source: internal/plugins/bfd/api/registry.go — SetService, GetService -->
<!-- source: internal/plugins/bfd/engine/loop.go — passesTTLGate, tick jitter -->
<!-- source: internal/plugins/bfd/transport/udp.go — ListenConfig.Control, ReadMsgUDPAddrPort -->
<!-- source: internal/plugins/bfd/transport/udp_linux.go — IP_RECVTTL, IP_TTL, SO_BINDTODEVICE -->
<!-- source: internal/component/bgp/reactor/peer_bfd.go — startBFDClient, stopBFDClient, runBFDSubscriber -->

BFD (RFC 5880) is a low-overhead liveness protocol that detects forwarding
failures between a pair of systems in tens to hundreds of milliseconds,
one or two orders of magnitude faster than BGP keepalives or OSPF hellos.
In ze, BFD runs as a plugin that exposes a session service to other
components: BGP, OSPF (when added), and static-route monitors call it to
track peer liveness, and tear down their own adjacencies when BFD reports
a session down.

## When to use it

| Scenario | BFD value |
|----------|----------|
| eBGP transit session | Sub-second failure detection without waiting for BGP hold time (default 90 s) |
| iBGP over a loopback | Multi-hop BFD can accelerate route reconvergence if the IGP alone is too slow |
| Static route to a gateway | Withdraw the route when the gateway stops forwarding, even if the physical link is up |
| OSPF adjacency (when ze has OSPF) | Collapse the hello-based timeout from 40 s to 150 ms |

Do not use BFD between two routers where the IGP already runs fast BFD —
the redundant sessions double the packet rate and can amplify instability.

## Basic configuration

### Profiles

Profiles are named bundles of timer and feature parameters. Sessions
reference a profile by name and inherit every leaf from it. Profiles are
optional: sessions can carry their parameters inline.

```
bfd {
    profile fast-link {
        desired-min-tx-us   50000;    # 50 ms
        required-min-rx-us  50000;    # 50 ms
        detect-multiplier   3;        # 150 ms detection time
    }

    profile lan-default {
        desired-min-tx-us   300000;   # 300 ms
        required-min-rx-us  300000;
        detect-multiplier   3;
    }

    profile ibgp-loopback {
        desired-min-tx-us   300000;
        required-min-rx-us  300000;
        detect-multiplier   5;
    }
}
```

All interval fields are in **microseconds** because RFC 5880 expresses
every interval in microseconds. A future release may add `-ms` aliases.

| Leaf | Range | Default | Meaning |
|------|-------|---------|---------|
| `desired-min-tx-us` | 1 – 4 294 967 295 | 300 000 | Local target transmit rate. RFC §6.8.3 enforces a 1 000 000 µs floor while the session is not Up. |
| `required-min-rx-us` | 0 – 4 294 967 295 | 300 000 | Minimum inter-packet gap the local end can handle. Zero means "do not send me periodic control packets." |
| `detect-multiplier` | 1 – 255 | 3 | Number of consecutive missed packets that trigger a Down transition. |
| `passive` | boolean | false | Active sessions transmit from creation. Passive waits for the peer's first packet. See RFC 5883 §4.3 for the unidirectional-link use case. |
| `auth { type key-id secret }` | see below | none | RFC 5880 §6.7 cryptographic authentication (Stage 5). |
| `echo { desired-min-echo-tx-us }` | see below | none | RFC 5880 §6.4 Echo mode (single-hop only). |

### Echo mode

Profiles may carry an `echo { desired-min-echo-tx-us N }` block
that opts sessions into RFC 5880 §6.4 Echo mode. The block is
valid only on single-hop sessions -- RFC 5883 §4 prohibits
multi-hop echo, and the parser rejects the combination with a
descriptive error.
<!-- source: internal/plugins/bfd/config.go — parseEchoConfig + validate -->
<!-- source: internal/plugins/bfd/packet/echo.go — ZEEC 16-byte envelope -->
<!-- source: internal/plugins/bfd/session/timers.go — EchoEnabled, EchoInterval -->

```
bfd {
    profile fast-echo {
        desired-min-tx-us   100000;
        required-min-rx-us  100000;
        echo {
            desired-min-echo-tx-us 50000;   # 50 ms
        }
    }

    single-hop-session 203.0.113.9 {
        profile fast-echo;
    }
}
```

When a session inherits this profile, every outgoing Control
packet sets `RequiredMinEchoRxInterval` to the configured rate.
Peers that see a non-zero advertisement learn that the local end
is willing to reflect echo packets.

**Current coverage:** the YANG surface, wire advertisement, and
session state plumbing ship in Stage 6. The actual echo transport
(UDP 3785 socket, per-session TX scheduler, RX demux, RTT
histogram, detection-time switchover, async slow-down) is tracked
as `spec-bfd-6b-echo-transport`. Configurations written against
the Stage 6 surface remain valid when the transport half lands.
The `ze_bfd_echo_tx_packets_total` and `ze_bfd_echo_rx_packets_total`
metric families are registered now so downstream alerting can
reference them from day one, even though the counters stay at
zero until the transport half lands.

### Authentication

Profiles may carry an `auth { ... }` block that enables RFC 5880 §6.7
authentication for every session inheriting them. Four types are
supported: `keyed-md5`, `meticulous-keyed-md5`, `keyed-sha1`, and
`meticulous-keyed-sha1`. Simple Password is refused at config parse
time because RFC 5880 §6.7.2 warns it provides no cryptographic
protection.
<!-- source: internal/plugins/bfd/auth/signer.go — Signer/Verifier + Settings -->
<!-- source: internal/plugins/bfd/auth/sha1.go — digestSigner/digestVerifier -->
<!-- source: internal/plugins/bfd/config.go — parseAuthConfig rejects simple-password -->

```
bfd {
    persist-dir "/var/lib/ze/bfd";

    profile authenticated-fast {
        desired-min-tx-us   50000;
        required-min-rx-us  50000;
        detect-multiplier   3;
        auth {
            type                meticulous-keyed-sha1;
            key-id              7;
            secret              "BFD-SHARED-SECRET-V1";
        }
    }
}
```

The meticulous variants enforce strict monotonic sequence numbers;
non-meticulous variants allow a receiver to accept equal sequence
numbers across retransmits. The `persist-dir` leaf (top-level on
`bfd { }`) names a directory where ze stores the last TX sequence
per session so a Meticulous session resumes above the peer's replay
window after a process restart. Without `persist-dir`, Meticulous
sessions still work at runtime but briefly re-synchronize after a
restart while the peer's replay window slides forward.

Authentication failures increment `ze_bfd_auth_failures_total{mode}`.

### Enabling BFD on a BGP peer

The common path. Adding a `bfd { ... }` container inside a BGP peer's
`connection { ... }` block opts that peer into BFD. On session
Established, the reactor calls the BFD plugin's `api.Service.EnsureSession`
with the peer's local / remote address and the configured mode; on
session teardown the handle is released. When BFD reports the
forwarding path Down, the reactor tears the BGP session with RFC 9384
Cease NOTIFICATION (subcode 10, "BFD Down") without waiting for the
hold timer.
<!-- source: internal/component/bgp/reactor/peer_bfd.go — startBFDClient, runBFDSubscriber -->
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang — peer connection bfd container -->

```
bgp {
    peer peer1 {
        connection {
            local {
                ip 192.0.2.1
            }
            remote {
                ip 192.0.2.2
            }
            bfd {
                enabled true
                mode single-hop
                profile fast-link
            }
        }
        session {
            asn {
                local 65001
                remote 65002
            }
            router-id 192.0.2.1
            family {
                ipv4/unicast
            }
        }
    }
}
```

With no `bfd` container, BGP behaves exactly as today. The container
has YANG `presence`: its mere existence opts in, and `enabled false`
suspends the opt-in without removing the config (useful during
maintenance). The profile name references a profile defined under the
top-level `bfd { profile ... }` block; the BFD plugin resolves it when
it receives `EnsureSession`. If the BFD plugin is not loaded at all,
the BGP peer starts without BFD and logs a warning -- BGP is not
blocked by a missing BFD plugin.
<!-- source: internal/plugins/bfd/api/registry.go — SetService / GetService -->

### Multi-hop

For iBGP between loopbacks or any peering that crosses more than one IP
hop:

```
bgp {
    peer loop4 {
        connection {
            local {
                ip 10.255.255.1
            }
            remote {
                ip 10.255.255.4
            }
            bfd {
                enabled true
                mode multi-hop
                min-ttl 250
                profile ibgp-loopback
            }
        }
        session {
            asn {
                local 65000
                remote 65000
            }
            router-id 10.255.255.1
            family {
                ipv4/unicast
            }
        }
    }
}
```

`mode multi-hop` tells the plugin to use UDP port 4784 and skip GTSM.
`min-ttl` is a weaker replacement for GTSM — packets arriving with a
TTL below the configured value are discarded. Choose `min-ttl` to be
`256 - max-hops`; for example, `min-ttl 250` allows the packet to
cross up to 5 hops. `min-ttl` must be non-zero for multi-hop; the
parser rejects zero at config-validate time.

## Standalone sessions

Some paths have no protocol client. For those, pin the session explicitly
in the top-level `bfd` block. It exists for the lifetime of the config.

```
bfd {
    profile gateway {
        desired-min-tx-us   200000;
        required-min-rx-us  200000;
        detect-multiplier   3;
    }

    single-hop-session 192.0.2.254 {
        local       192.0.2.1;
        interface   eth0;
        profile     gateway;
    }

    multi-hop-session 198.51.100.7 {
        local       10.0.0.1;
        profile     gateway;
        min-ttl     254;
    }
}
```

Standalone sessions are useful for monitoring a default gateway, a tunnel
endpoint, or any path whose liveness a script or external tool wants to
react to via the event stream.

## Administrative shutdown

`shutdown;` on a session puts it into `AdminDown` (RFC 5880 §6.8.16)
without removing the config. The peer sees AdminDown via the State field
and, per RFC 5882, clients with their own liveness signal keep running
while clients without one treat it as Down.

```
bfd {
    single-hop-session 192.0.2.254 {
        local       192.0.2.1;
        interface   eth0;
        profile     gateway;
        shutdown;
    }
}
```

Remove `shutdown;` (or set it to `false` in a tool-driven edit) to
re-enable the session; it transitions through Down and completes the
three-way handshake normally.

## Session sharing

When multiple clients ask for the same path, the BFD plugin creates one
underlying session and refcounts subscribers. Timer parameters are chosen
as the most aggressive (smallest) value across requesters. For example, if
BGP asks for a 50 ms session and OSPF later asks for a 300 ms session to
the same peer, they share one 50 ms session; if the BGP subscriber goes
away first, the session drops to 300 ms via Poll/Final.

## Editing live

BFD configuration is reachable through the standard `ze config edit` CLI:

```
operator@router> configure
operator@router# edit bfd profile fast-link
operator@router# set desired-min-tx-us 100000
operator@router# show
profile fast-link {
    desired-min-tx-us  100000;
    required-min-rx-us 50000;
    detect-multiplier  3;
}
operator@router# commit
```

On `commit`, the plugin receives a diff section via `OnConfigure` and
reconfigures affected sessions in place. Profile changes propagate to
every session that references the profile via an in-band Poll/Final
sequence — no session flap.

## Observing state

Stage 4 adds three operator-facing commands served by a snapshot of
the engine's live session state. The handlers live in
`internal/component/cmd/bfd/bfd.go` and publish JSON payloads so
scripts can parse the output while the interactive CLI renders them.
<!-- source: internal/component/cmd/bfd/bfd.go — handleShowSessions, handleShowSession, handleShowProfile -->
<!-- source: internal/plugins/bfd/engine/snapshot.go — Loop.Snapshot, Loop.SessionDetail -->

### List all sessions

```
operator@router> show bfd sessions
[
  {"peer":"192.0.2.2","vrf":"default","mode":"single-hop","state":"up","diag":"no-diagnostic","local-discriminator":1,"remote-discriminator":2147518038,"tx-interval":50000000,"rx-interval":50000000,"detection-interval":150000000,"detect-multiplier":3,"profile":"fast-link","tx-packets":2312,"rx-packets":2310,"refcount":1,...},
  {"peer":"198.51.100.7","vrf":"default","mode":"multi-hop","state":"down","diag":"control-detection-time-expired","tx-interval":200000000,"rx-interval":200000000,"detection-interval":600000000,"detect-multiplier":3,"profile":"gateway",...}
]
```

Fields are sorted stably by `(mode, vrf, peer)` so successive scrapes
produce diff-able output.

### Session detail

`show bfd session <peer>` returns the same struct for one session
plus the most recent state transitions kept in a bounded in-memory
ring (eight entries by default -- see `api.TransitionHistoryDepth`):

```
operator@router> show bfd session 192.0.2.2
{
  "peer": "192.0.2.2",
  "local": "192.0.2.1",
  "interface": "eth0",
  "vrf": "default",
  "mode": "single-hop",
  "state": "up",
  "diag": "no-diagnostic",
  "profile": "fast-link",
  "local-discriminator": 1,
  "remote-discriminator": 2147518038,
  "tx-interval": 50000000,
  "rx-interval": 50000000,
  "detection-interval": 150000000,
  "detect-multiplier": 3,
  "tx-packets": 2312,
  "rx-packets": 2310,
  "refcount": 1,
  "transitions": [
    {"when":"2026-04-11T09:14:22Z","from":"down","to":"init","diag":"no-diagnostic"},
    {"when":"2026-04-11T09:14:23Z","from":"init","to":"up","diag":"no-diagnostic"}
  ]
}
```

### List profiles

`show bfd profile` returns every resolved (post-default) profile
stored by the plugin. Passing a name filters to one entry; an unknown
name returns an error:

```
operator@router> show bfd profile fast-link
{"name":"fast-link","detect-multiplier":3,"desired-min-tx-us":50000,"required-min-rx-us":50000,"passive":false}
```

### Prometheus metrics

The plugin registers five metric families via
`internal/plugins/bfd/metrics.go`. The families appear on the
telemetry endpoint when `telemetry { prometheus { enabled true } }`
is set.
<!-- source: internal/plugins/bfd/metrics.go — bindMetricsRegistry, metricsHook, refreshSessionsGauge -->

| Metric | Type | Labels | Meaning |
|--------|------|--------|---------|
| `ze_bfd_sessions` | gauge | state, mode, vrf | Live session count, updated on every `show bfd sessions` scrape |
| `ze_bfd_transitions_total` | counter | from, to, diag, mode | Every session state change |
| `ze_bfd_detection_expired_total` | counter | mode | Detection-timer expirations (RFC 5880 §6.8.4) |
| `ze_bfd_tx_packets_total` | counter | mode | Control packets transmitted |
| `ze_bfd_rx_packets_total` | counter | mode | Control packets received (after the TTL gate) |


## Operational guidance

### Picking timers

| Situation | TX / RX | Mult | Detection |
|-----------|---------|------|-----------|
| LAN between routers | 50 ms / 50 ms | 3 | 150 ms |
| Metro eBGP | 100 ms / 100 ms | 3 | 300 ms |
| iBGP over IGP | 300 ms / 300 ms | 5 | 1.5 s |
| WAN over uncertain media | 500 ms / 500 ms | 5 | 2.5 s |

Err on the slow side. A session that runs too fast on a link with jitter
produces false-positive failures that tear down routing state. BFD is
worse than useless in that mode: it amplifies instability instead of
damping it.

### TTL security on single-hop

Single-hop BFD enforces GTSM (RFC 5082): packets are sent with `TTL=255`
(`IP_TTL` socket option applied at `Start`) and received packets with any
other TTL are silently discarded by the engine. There is nothing for the
operator to configure -- ze sets the TTL automatically and the engine's
TTL gate fails closed when the kernel cannot extract the received TTL
(for example, when running on a platform without `IP_RECVTTL`).

Multi-hop sessions have no GTSM equivalent; instead each `multi-hop-session`
carries a `min-ttl` leaf (default 254). Packets arriving with a TTL below
that floor are discarded before reaching the FSM. Pick `min-ttl` to be
`256 - max-hops`.
<!-- source: internal/plugins/bfd/engine/loop.go — passesTTLGate -->

### Multi-VRF and interface binding

Single-hop pinned sessions may specify an `interface` leaf. When every
pinned session in the same `(vrf, single-hop)` pair names the same
interface, the plugin binds the socket to that interface via
`SO_BINDTODEVICE` so egress and ingress are guaranteed to traverse the
named device. If a pinned set mixes interfaces in the same VRF, the
bind-to-device fallback is disabled (a warning is logged) and the
engine-side TTL gate remains the sole protection.

Non-default VRFs bind the socket to the VRF device name (Linux's
`SO_BINDTODEVICE` accepts VRF masters as device names). Both paths
require `CAP_NET_RAW` at daemon startup on Linux.

`SO_BINDTODEVICE` can only name one device per socket, so under a
non-default VRF the session `interface` leaf is ignored -- the socket
binds to the VRF master and receives packets from every slave device
in that VRF. Ze logs an `Info` line naming the dropped interface leaves
whenever this override fires so the operator can correlate a reload
with the behaviour change. If you need per-interface pinning in a
non-default VRF, stand up separate sessions per slave device outside
the shared VRF, or wait for the Stage 3 BGP peer opt-in which can
drive one session per peer.
<!-- source: internal/plugins/bfd/bfd.go — resolveLoopDevices, newUDPTransport -->
<!-- source: internal/plugins/bfd/transport/udp_linux.go — applySocketOptions -->

### Transmit jitter

RFC 5880 §6.8.7 requires the TX interval to be reduced by 0-25% on each
packet, and the reduction must be at least 10% when `detect-multiplier`
is 1 so the receiver cannot detect before the next packet arrives. Ze
implements both bands via `engine.Loop.applyJitter`; operators do not
configure it.
<!-- source: internal/plugins/bfd/engine/engine.go — applyJitter -->
<!-- source: internal/plugins/bfd/engine/loop.go — tick -->


### GC pause sensitivity

At 50 ms intervals with `mult=3`, a 150 ms GC pause looks indistinguishable
from a real failure. Ze's BFD plugin uses pool-backed buffers and runs
every session on a dedicated goroutine (the "express loop") to minimise GC
pressure on the session-driving thread. On a heavily loaded ze instance,
watch for detection-timer expirations that coincide with high allocation
rates elsewhere in the daemon. The metric surface (coming with the wiring
commit) will expose `bfd_control_detection_time_expired_total` per session
so the correlation is visible in Prometheus.

### Interop

ze's wire format matches FRR `bfdd`, BIRD 3.x, and Junos. Interop tests
against FRR are the primary validation; `test/plugin/bfd/` will carry a
namespace-based scenario once the functional tests land.

## What is not yet supported

| Feature | RFC | Status |
|---------|-----|--------|
| Authentication | 5880 §6.7 | Parser only; no digest verification. Add keyed SHA1 first when a deployment asks. |
| Echo mode | 5880 §6.4, 5881 §5 | Not implemented. Single-hop only when added. |
| Demand mode | 5880 §6.6 | Not implemented; rarely deployed. |
| Seamless BFD (S-BFD) | 7880, 7881 | Not implemented. |
| Micro-BFD on LAG | 7130 | Not implemented. |
| Multipoint BFD | 8562 | Not implemented. |
| Data-plane offload | FRR `bfddp_packet.h` | Future work. |

The skeleton is intentionally minimum-viable. See the architecture
document for the full gap list and the intended follow-up order.

## Reference

- Protocol details: `rfc/short/rfc5880.md`, `rfc5881.md`, `rfc5882.md`, `rfc5883.md`
- Internal architecture: `docs/architecture/bfd.md`
- Implementation research: `docs/research/bfd-implementation-guide.md`
