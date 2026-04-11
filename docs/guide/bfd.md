# BFD — Bidirectional Forwarding Detection

**Status:** planned UX. The plugin code is merged but not yet wired into
the engine startup path. This guide describes the configuration shape that
will be exposed once wiring lands so operators and downstream config
generators can plan against it.

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

### Enabling BFD on a BGP peer

The common path. BGP calls the BFD plugin's session service with the peer's
address when a `bfd { ... }` child exists on the peer block.

```
bgp {
    peer 192.0.2.2 {
        router-id       192.0.2.1;
        local-address   192.0.2.1;
        local-as        65001;
        peer-as         65002;
        family {
            ipv4/unicast;
        }
        bfd {
            profile fast-link;
        }
    }
}
```

With no `bfd` block, BGP behaves exactly as today. With `bfd { enabled; }`
(and no profile), built-in defaults (300 ms / 300 ms / mult 3) are used.
With `bfd { profile <name>; }`, the named profile drives timer selection.
Inline overrides on the peer's `bfd` block win over profile values.

### Multi-hop

For iBGP between loopbacks or any peering that crosses more than one IP
hop:

```
bgp {
    peer 10.255.255.4 {
        router-id       10.255.255.1;
        local-address   10.255.255.1;
        local-as        65000;
        peer-as         65000;
        family {
            ipv4/unicast;
        }
        bfd {
            profile      ibgp-loopback;
            multi-hop;
            min-ttl      250;
        }
    }
}
```

`multi-hop` tells the plugin to use UDP port 4784 and skip GTSM. `min-ttl`
is a weaker replacement for GTSM — packets arriving with a TTL below the
configured value are discarded. Choose `min-ttl` to be `256 - max-hops`;
for example, `min-ttl 250` allows the packet to cross up to 5 hops.

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

### List all sessions

```
operator@router> show bfd sessions
PEER             LOCAL          IFACE   MODE   STATE   DIAG     TX/RX/MULT          UP-FOR
192.0.2.2        192.0.2.1      eth0    1-hop  Up      none     50ms/50ms/3         3h12m
192.0.2.10       192.0.2.1      eth0    1-hop  Up      none     300ms/300ms/3       3h12m
10.255.255.4     10.255.255.1   -       multi  Up      none     300ms/300ms/5       17m
192.0.2.254      192.0.2.1      eth0    1-hop  Up      none     200ms/200ms/3       3h12m
198.51.100.7     10.0.0.1       -       multi  Down    detect   200ms/200ms/3       -
```

### List profiles

```
operator@router> show bfd profile
NAME              DESIRED-TX  REQUIRED-RX  MULT   PASSIVE
fast-link         50ms        50ms         3      no
lan-default       300ms       300ms        3      no
ibgp-loopback     300ms       300ms        5      no
gateway           200ms       200ms        3      no
```

### Session detail

```
operator@router> show bfd session 192.0.2.2
peer:                  192.0.2.2
local:                 192.0.2.1
interface:             eth0
mode:                  single-hop
state:                 Up (since 2026-04-11 09:14:23)
local diagnostic:      no-diagnostic
local discriminator:   0xCAFE0001
remote discriminator:  0x80123456
configured tx:         50ms
configured rx:         50ms
detect multiplier:     3
negotiated detect:     150ms
poll outstanding:      no
clients:               bgp[192.0.2.2]
```

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
and received packets with any other TTL are discarded. This makes
single-hop BFD non-trivially spoofable from anywhere off-link. There is
nothing for the operator to configure — ze sets the TTL automatically.

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
