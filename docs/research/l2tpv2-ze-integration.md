# L2TPv2 Ze Integration Design

Companion to `l2tpv2-implementation-guide.md` (protocol spec).
This document maps the protocol to ze's architecture: subsystem lifecycle,
plugin ecosystem, event integration, redistribution, buffer-first encoding,
and registration patterns.

Linux-only. No platform abstraction needed.

---

## Table of Contents

1. [Architectural Placement](#1-architectural-placement)
2. [Kernel PPP: Control/Data Split](#2-kernel-ppp-controldata-split)
3. [Subsystem Lifecycle](#3-subsystem-lifecycle)
4. [Registration Points](#4-registration-points)
5. [YANG Schema Design](#5-yang-schema-design)
6. [Plugin Ecosystem](#6-plugin-ecosystem)
7. [Event Namespace and Types](#7-event-namespace-and-types)
8. [Cross-Subsystem Event Flow](#8-cross-subsystem-event-flow)
9. [Redistribution Integration](#9-redistribution-integration)
10. [Buffer-First Wire Encoding](#10-buffer-first-wire-encoding)
11. [Concurrency Model](#11-concurrency-model)
12. [Package Layout](#12-package-layout)
13. [File Descriptor Lifecycle](#13-file-descriptor-lifecycle)
14. [Configuration Pipeline](#14-configuration-pipeline)
15. [CLI Commands](#15-cli-commands)
16. [Metrics](#16-metrics)
17. [Functional Tests](#17-functional-tests)

---

## 1. Architectural Placement

L2TP is a **subsystem**, not a plugin. It owns network transport (UDP sockets),
manages its own state machines (tunnel FSM, session FSM), and creates kernel
resources (L2TP tunnels, PPP interfaces). This is the same tier as BGP.

| Tier | Examples | Why L2TP fits here |
|------|----------|-------------------|
| Subsystem | BGP, FIB | Owns transport, manages sessions, creates interfaces |
| Plugin | bgp-rib, bgp-rs, bgp-gr | Policy decisions, no owned transport |
| Component | config, authz, telemetry | Single-purpose libraries |

L2TP-specific policy (authentication, IP pool selection, session filtering)
lives in plugins, not in the subsystem.

---

## 2. Kernel PPP: Control/Data Split

Verified against Linux kernel source (`ppp_generic.c`, `l2tp_ppp.c`) and
accel-ppp's implementation. The kernel splits PPP into control and data planes:

### What the kernel handles (data plane)

- L2TP data message encapsulation/decapsulation (T=0 packets)
- PPP frame forwarding through pppN interfaces
- IP packet routing via normal kernel networking stack
- Data sequencing and reordering (via socket options)

### What ze handles in userspace (control plane)

- L2TP control messages (SCCRQ/SCCRP/SCCCN, ICRQ/ICRP/ICCN, etc.)
- PPP negotiation: LCP, authentication (PAP/CHAP/MS-CHAPv2), IPCP, IPv6CP
  via `/dev/ppp` channel and unit file descriptors
- IP address assignment (driven by IPCP negotiation, configured via netlink)
- Interface configuration (address, routes, MTU via netlink)
- Session lifecycle decisions (accept/reject, teardown)

### Kernel intercept behavior

After `L2TP_CMD_TUNNEL_CREATE`, the kernel installs `encap_recv` on the
UDP socket. Data messages (T=0) are intercepted by the kernel and delivered
to the PPP subsystem. Control messages (T=1) pass through to userspace
reads on the same UDP socket. This is transparent.

### PPP frame routing inside the kernel

`ppp_generic.c:ppp_receive_nonmp_frame()` inspects the 2-byte PPP protocol
field:
- Known data protocols (0x0021=IPv4, 0x0057=IPv6): delivered to kernel
  networking stack via `netif_rx()`. Never reaches userspace.
- Everything else (0xC021=LCP, 0x8021=IPCP, 0xC023=PAP, 0xC223=CHAP):
  queued to the userspace process via the `/dev/ppp` file descriptor.

**Conclusion:** kernel delegation loses zero control plane visibility. Ze
drives all negotiation, authentication, and IP assignment. The kernel
provides data plane acceleration only.

---

## 3. Subsystem Lifecycle

L2TP implements `ze.Subsystem`:

```go
type Subsystem interface {
    Name() string
    Start(ctx context.Context, eventBus EventBus, config ConfigProvider) error
    Stop(ctx context.Context) error
    Reload(ctx context.Context, config ConfigProvider) error
}
```

### Start sequence

1. Parse L2TP config from `ConfigProvider` (YANG tree).
2. Probe kernel modules: `modprobe l2tp_ppp || modprobe pppol2tp`.
3. Resolve Generic Netlink family ID for `"l2tp"`.
4. Create UDP listener socket, bind to configured address:port (default 0.0.0.0:1701).
5. Register L2TP event namespace and types on `EventBus`.
6. Register L2TP redistribute source via `redistribute.RegisterSource()`.
7. Start reactor goroutine (reads UDP socket, dispatches to tunnel state machines).
8. Start timer goroutine (retransmission, hello keepalive).
9. Emit `(l2tp, listener-ready)` event.

### Stop sequence

1. Stop accepting new tunnels.
2. Send StopCCN to all established tunnels.
3. Wait for acknowledgments (up to full retransmit cycle, ~31s).
4. Close all PPPoL2TP sockets.
5. Delete all kernel sessions (`L2TP_CMD_SESSION_DELETE`).
6. Delete all kernel tunnels (`L2TP_CMD_TUNNEL_DELETE`).
7. Close UDP listener socket.
8. Emit `(l2tp, stopped)` event.

### Reload sequence

1. Parse new config.
2. Diff against current config (listen address, shared secret, tunnels, etc.).
3. For unchanged tunnels/sessions: no action.
4. For removed tunnels: send StopCCN, clean up.
5. For changed parameters (hello interval, retransmit settings): update in place.
6. For new listen address: requires stop + start (warn user).

---

## 4. Registration Points

Following ze's `init()` + `register.go` pattern:

### 4.1 Subsystem registration

```go
// internal/component/l2tp/register.go
func init() {
    engine.RegisterSubsystem(&l2tpSubsystem{})
}
```

### 4.2 YANG module registration

```go
// internal/component/l2tp/schema/register.go

//go:embed ze-l2tp-conf.yang
var ZeL2TPConfYANG string

func init() {
    yang.RegisterModule("ze-l2tp-conf.yang", ZeL2TPConfYANG)
}
```

### 4.3 Environment variable registration

```go
// internal/component/l2tp/environment.go

var _ = env.MustRegister(env.EnvEntry{
    Key:         "ze.l2tp.listen.port",
    Type:        "int",
    Default:     "1701",
    Description: "L2TP UDP listen port",
})

var _ = env.MustRegister(env.EnvEntry{
    Key:         "ze.l2tp.hello.interval",
    Type:        "duration",
    Default:     "60s",
    Description: "L2TP hello keepalive interval",
})
```

### 4.4 Event type registration

```go
// internal/component/l2tp/events.go
func init() {
    plugin.RegisterEventType(NamespaceL2TP, EventTunnelUp)
    plugin.RegisterEventType(NamespaceL2TP, EventTunnelDown)
    plugin.RegisterEventType(NamespaceL2TP, EventSessionUp)
    plugin.RegisterEventType(NamespaceL2TP, EventSessionDown)
    plugin.RegisterEventType(NamespaceL2TP, EventSessionIPAssigned)
    plugin.RegisterEventType(NamespaceL2TP, EventListenerReady)
    plugin.RegisterEventType(NamespaceL2TP, EventStopped)
}
```

This requires modifying `internal/component/plugin/events.go`:

1. Add the namespace constant alongside the existing ones:
   ```go
   NamespaceL2TP = "l2tp"
   ```

2. Add the valid event type map:
   ```go
   var ValidL2TPEvents = map[string]bool{
       "tunnel-up":            true,
       "tunnel-down":          true,
       "session-up":           true,
       "session-down":         true,
       "session-ip-assigned":  true,
       "session-auth-request": true,
       "session-ip-request":   true,
       "listener-ready":       true,
       "stopped":              true,
   }
   ```

3. Add the entry in the `ValidEvents` initializer:
   ```go
   NamespaceL2TP: ValidL2TPEvents,
   ```

### 4.5 Redistribute source registration

```go
// internal/component/l2tp/redistribute.go
func init() {
    redistribute.RegisterSource(redistribute.RouteSource{
        Name:        "l2tp",
        Protocol:    "l2tp",
        Description: "L2TP subscriber routes (PPP-assigned addresses)",
    })
}
```

### 4.6 Logger registration

```go
// internal/component/l2tp/logger.go
var logger = slogutil.Logger("l2tp")
```

With env var `ze.log.l2tp` for level control.

---

## 5. YANG Schema Design

Follow existing ze config patterns. Mirror `bgp peer connection { local { ip } }`
for endpoint shapes.

```yang
module ze-l2tp-conf {
    namespace "urn:ze:l2tp:conf";
    prefix l2tp;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    container l2tp {
        description "L2TP subsystem configuration";

        container listen {
            ze:listener;
            uses zt:listener;
            // refine ip default "0.0.0.0"
            // refine port default 1701
        }

        leaf host-name {
            type string;
            description "Host Name AVP value sent in SCCRQ/SCCRP";
        }

        leaf shared-secret {
            type string;
            ze:sensitive;
            description "Tunnel authentication shared secret";
        }

        container timers {
            leaf hello-interval {
                type uint16;
                default 60;
                units seconds;
                description "Seconds of silence before sending HELLO";
            }
            leaf retransmit-initial {
                type uint16;
                default 1;
                units seconds;
            }
            leaf retransmit-max {
                type uint8;
                default 5;
                description "Maximum retransmission attempts before tunnel teardown";
            }
            leaf retransmit-cap {
                type uint16;
                default 16;
                units seconds;
            }
        }

        leaf receive-window {
            type uint16 { range "1..max"; }
            default 16;
            description "Receive Window Size advertised to peers";
        }

        leaf ppp-max-mtu {
            type uint16;
            default 1420;
            description "Maximum PPP MRU to negotiate in LCP";
        }

        leaf hide-avps {
            type boolean;
            default false;
            description "Encrypt sensitive AVPs using shared secret";
        }

        leaf data-sequencing {
            type enumeration {
                enum allow;
                enum deny;
                enum prefer;
                enum require;
            }
            default allow;
        }

        leaf reorder-timeout {
            type uint32;
            default 0;
            units milliseconds;
            description "Data message reorder timeout. 0 = disabled.";
        }

        container limits {
            leaf max-tunnels {
                type uint32;
                default 0;
                description "Maximum simultaneous tunnels. 0 = unlimited.";
            }
            leaf max-sessions {
                type uint32;
                default 0;
                description "Maximum simultaneous sessions. 0 = unlimited.";
            }
        }
    }
}
```

---

## 6. Plugin Ecosystem

Policy decisions are plugins. The L2TP subsystem provides the protocol engine;
plugins control what happens at decision points.

### 6.1 l2tp-auth (authentication and authorization)

```go
registry.Register(registry.Registration{
    Name:        "l2tp-auth",
    Description: "L2TP session authentication via RADIUS",
    RunEngine:   runL2TPAuth,    // func(conn net.Conn) int
    CLIHandler:  cliL2TPAuth,    // func(args []string) int
    ConfigRoots: []string{"l2tp.auth"},
    YANG:        l2tpauthschema.ZeL2TPAuthConfYANG,
    ConfigureEngineLogger: func(name string) { setLogger(slogutil.PluginLogger(name, slog.LevelWarn)) },
    ConfigureMetrics:      func(reg any) { SetMetricsRegistry(reg.(metrics.Registry)) },
    ConfigureEventBus:     func(eb any) { setEventBus(eb.(ze.EventBus)) },
})
```

Responsibilities:
- Subscribe to `(l2tp, session-auth-request)` events.
- Query RADIUS for authentication (PAP/CHAP credentials from PPP negotiation).
- Return accept/reject + session attributes (IP pool name, rate limits).
- Handle RADIUS CoA (Change of Authorization) for live sessions.
- Handle RADIUS DM (Disconnect Message) to terminate sessions.

### 6.2 l2tp-pool (IP address management)

```go
registry.Register(registry.Registration{
    Name:        "l2tp-pool",
    Description: "IP address pool for L2TP PPP sessions",
    RunEngine:   runL2TPPool,
    CLIHandler:  cliL2TPPool,
    ConfigRoots: []string{"l2tp.pool"},
    YANG:        l2tppoolschema.ZeL2TPPoolConfYANG,
    ConfigureEngineLogger: func(name string) { setLogger(slogutil.PluginLogger(name, slog.LevelWarn)) },
    ConfigureEventBus:     func(eb any) { setEventBus(eb.(ze.EventBus)) },
})
```

Responsibilities:
- Manage IPv4 and IPv6 address pools.
- Allocate addresses on `(l2tp, session-ip-request)` events.
- Release addresses on `(l2tp, session-down)` events.
- Support RADIUS-directed pool selection (Framed-Pool attribute).
- Expose pool statistics via CLI commands.

### 6.3 l2tp-shaper (traffic shaping)

```go
registry.Register(registry.Registration{
    Name:        "l2tp-shaper",
    Description: "Traffic shaping for L2TP subscriber sessions",
    RunEngine:   runL2TPShaper,
    CLIHandler:  cliL2TPShaper,
    ConfigRoots: []string{"l2tp.shaper"},
    YANG:        l2tpshaperschema.ZeL2TPShaperConfYANG,
    ConfigureEngineLogger: func(name string) { setLogger(slogutil.PluginLogger(name, slog.LevelWarn)) },
    ConfigureEventBus:     func(eb any) { setEventBus(eb.(ze.EventBus)) },
})
```

Responsibilities:
- Apply TC (traffic control) rules to pppN interfaces.
- Subscribe to `(l2tp, session-up)` to apply initial shaping.
- Handle rate changes from RADIUS CoA.
- Clean up TC rules on `(l2tp, session-down)`.

### 6.4 l2tp-stats (telemetry)

```go
registry.Register(registry.Registration{
    Name:        "l2tp-stats",
    Description: "L2TP session statistics and monitoring",
    RunEngine:   runL2TPStats,
    CLIHandler:  cliL2TPStats,
    ConfigRoots: []string{"l2tp.stats"},
    YANG:        l2tpstatsschema.ZeL2TPStatsConfYANG,
    ConfigureEngineLogger: func(name string) { setLogger(slogutil.PluginLogger(name, slog.LevelWarn)) },
    ConfigureMetrics:      func(reg any) { SetMetricsRegistry(reg.(metrics.Registry)) },
    ConfigureEventBus:     func(eb any) { setEventBus(eb.(ze.EventBus)) },
})
```

Responsibilities:
- Periodic session statistics collection via `PPPIOCGL2TPSTATS` ioctl.
- Prometheus metrics export.
- Per-session accounting (bytes/packets in/out).
- RADIUS accounting start/interim/stop.

### 6.5 Main binary wiring

The L2TP subsystem must be registered with the engine at startup. This
happens in the main binary's setup code, following the pattern used for
other subsystems. The exact wiring point depends on ze's engine
initialization (see `docs/architecture/subsystem-wiring.md`).

```go
// In the engine setup path (e.g., cmd/ze/ or internal/component/engine/)
engine.RegisterSubsystem(l2tp.NewSubsystem())
```

L2TP plugins (`l2tp-auth`, `l2tp-pool`, etc.) are wired via blank imports
in `internal/component/plugin/all/all.go`, following the same pattern as
BGP plugins:

```go
_ "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpauth"
_ "codeberg.org/thomas-mangin/ze/internal/plugins/l2tppool"
_ "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpshaper"
_ "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpstats"
```

### 6.6 Config transaction integration

L2TP participates in ze's config transaction protocol (verify/apply/commit).
Per-plugin config events are registered dynamically at startup:

```go
plugin.RegisterEventType(plugin.NamespaceConfig, "verify-l2tp")
plugin.RegisterEventType(plugin.NamespaceConfig, "apply-l2tp")
```

The L2TP subsystem subscribes to these events and responds with
`verify-ok`/`verify-failed` and `apply-ok`/`apply-failed` following the
same protocol as other subsystems. On `rollback`, the subsystem reverts
to the previous configuration.

---

## 7. Event Namespace and Types

### 7.1 Namespace definition

Add to `internal/component/plugin/events.go`:

```go
const NamespaceL2TP = "l2tp"
```

### 7.2 Event types

| Event Type | Producer | Payload | Consumers |
|-----------|----------|---------|-----------|
| `tunnel-up` | L2TP subsystem | `{"tunnel-id": N, "peer-address": "...", "peer-host-name": "..."}` | Monitoring, logging |
| `tunnel-down` | L2TP subsystem | `{"tunnel-id": N, "reason": "...", "result-code": N}` | Monitoring, cleanup |
| `session-up` | L2TP subsystem | `{"session-id": N, "tunnel-id": N, "interface": "pppN", "calling": "...", "called": "..."}` | l2tp-shaper, l2tp-stats, redistribute |
| `session-down` | L2TP subsystem | `{"session-id": N, "tunnel-id": N, "interface": "pppN", "reason": "..."}` | l2tp-pool, l2tp-shaper, l2tp-stats, redistribute |
| `session-ip-assigned` | L2TP subsystem | `{"session-id": N, "interface": "pppN", "ipv4": "...", "ipv6": "...", "prefix": "..."}` | redistribute, fibkernel |
| `session-auth-request` | L2TP subsystem | `{"session-id": N, "username": "...", "auth-type": "chap", ...}` | l2tp-auth |
| `session-ip-request` | L2TP subsystem | `{"session-id": N, "pool-name": "...", "username": "..."}` | l2tp-pool |
| `listener-ready` | L2TP subsystem | `{"address": "0.0.0.0", "port": 1701}` | Monitoring |
| `stopped` | L2TP subsystem | `{}` | Engine lifecycle |

### 7.3 Event payload format

All payloads follow ze's JSON conventions:
- kebab-case keys
- IP addresses as strings
- Numbers as JSON numbers
- No camelCase or snake_case

---

## 8. Cross-Subsystem Event Flow

### 8.1 L2TP subscribes to

| Namespace | Event Type | Purpose |
|-----------|-----------|---------|
| `interface` | `addr-added` | Detect pppN interface address assignment (confirmation) |
| `interface` | `addr-removed` | Detect pppN address removal (external intervention) |
| `interface` | `down` | Detect pppN interface going down unexpectedly |
| `config` | `verify-l2tp` | Config transaction: verify proposed L2TP config |
| `config` | `apply-l2tp` | Config transaction: apply L2TP config changes |
| `config` | `rollback` | Config transaction: rollback L2TP config |

### 8.2 Other subsystems subscribe to L2TP

| Subscriber | Namespace | Event Type | Purpose |
|-----------|-----------|-----------|---------|
| BGP reactor | `l2tp` | `session-ip-assigned` | Trigger route advertisement for subscriber |
| BGP reactor | `l2tp` | `session-down` | Withdraw subscriber route |
| sysrib | `l2tp` (via redistribute) | `best-change` | L2TP routes flow through sysrib to fibkernel |
| fibkernel | `system-rib` | `best-change` | Programs subscriber routes into kernel FIB |
| Interface monitor | (automatic) | `created`, `up`, `down` | Netlink detects pppN creation/destruction |

### 8.3 End-to-end flow: subscriber comes up

```
1. LAC sends SCCRQ
   -> L2TP subsystem: tunnel state machine processes, sends SCCRP
2. LAC sends SCCCN
   -> Tunnel established
   -> L2TP emits (l2tp, tunnel-up)
3. LAC sends ICRQ
   -> L2TP sends ICRP
4. LAC sends ICCN
   -> L2TP creates kernel tunnel + session
   -> L2TP creates PPPoL2TP socket, /dev/ppp channel+unit
   -> Kernel creates pppN interface
5. L2TP runs LCP negotiation (via /dev/ppp channel fd)
   -> LCP CONFREQ/CONFACK exchange
6. L2TP runs PPP authentication (via /dev/ppp channel fd)
   -> L2TP emits (l2tp, session-auth-request)
   -> l2tp-auth plugin queries RADIUS
   -> l2tp-auth returns accept + attributes (pool name, rate limit)
7. L2TP runs IPCP negotiation (via /dev/ppp unit fd)
   -> L2TP emits (l2tp, session-ip-request) with pool name
   -> l2tp-pool allocates address, returns it
   -> L2TP uses allocated address in IPCP CONFACK
8. L2TP configures pppN interface via netlink
   -> Set address, set peer address, set MTU, add /32 route
   -> Netlink monitor detects interface + address
   -> Interface monitor emits (interface, created), (interface, addr-added)
9. L2TP emits (l2tp, session-ip-assigned)
   -> Redistribute plugin injects /32 route into protocol RIB
   -> Protocol RIB emits (bgp-rib, best-change)
   -> sysrib selects best route, emits (system-rib, best-change)
   -> fibkernel programs route with RTPROT_ZE
10. L2TP emits (l2tp, session-up)
    -> l2tp-shaper applies TC rules
    -> l2tp-stats starts accounting
11. IP traffic flows through pppN (kernel data plane, no ze involvement)
```

### 8.4 End-to-end flow: subscriber goes down

```
1. CDN received (or local close, or admin terminate)
   -> L2TP emits (l2tp, session-down)
2. l2tp-stats sends RADIUS Accounting-Stop
3. l2tp-shaper removes TC rules
4. l2tp-pool releases IP address
5. L2TP closes PPPoL2TP socket
6. L2TP deletes kernel session
   -> Kernel removes pppN interface
   -> Netlink monitor detects removal
   -> Interface monitor emits (interface, down)
7. Redistribute plugin withdraws /32 route
   -> sysrib removes route, emits (system-rib, best-change)
   -> fibkernel removes route from kernel FIB
8. If last session in tunnel and no keepalive needed:
   -> L2TP sends StopCCN, cleans up tunnel
   -> L2TP emits (l2tp, tunnel-down)
```

---

## 9. Redistribution Integration

### 9.1 Route source registration

L2TP registers as a redistribute source at init time:

```go
func init() {
    redistribute.RegisterSource(redistribute.RouteSource{
        Name:        "l2tp",
        Protocol:    "l2tp",
        Description: "L2TP subscriber routes (PPP-assigned addresses)",
    })
}
```

This makes `l2tp` available in BGP config:

```
bgp {
    redistribute l2tp;
}
```

### 9.2 Route injection flow

When a session gets an IP address (step 9 in the subscriber-up flow):

1. L2TP subsystem calls the redistribute plugin with the subscriber route:
   ```json
   {
       "protocol": "l2tp",
       "family": "ipv4/unicast",
       "changes": [{
           "action": "add",
           "prefix": "100.64.1.1/32",
           "next-hop": "0.0.0.0",
           "priority": 0,
           "metric": 0,
           "protocol-type": "l2tp"
       }]
   }
   ```

2. This follows the same path as any other route source:
   - Protocol RIB receives the route
   - Protocol RIB emits `(bgp-rib, best-change)` if it becomes best
   - sysrib receives, compares admin distance, selects system best
   - sysrib emits `(system-rib, best-change)`
   - fibkernel programs kernel route
   - BGP (if configured with `redistribute l2tp`) advertises to peers

3. On session teardown, the same path in reverse with `"action": "del"`.

### 9.3 Admin distance

L2TP connected routes should have a low admin distance (directly connected
subscriber). Suggested: 0 (same as connected/direct), configurable via:

```yang
rib {
    admin-distance {
        l2tp 0;
    }
}
```

### 9.4 Route attributes

L2TP subscriber routes carry:
- Prefix: /32 for IPv4, /128 for IPv6 (point-to-point PPP)
- Next-hop: 0.0.0.0 (directly connected)
- Protocol: "l2tp"
- Metric: configurable per-interface or per-pool, default 0
- The interface `metric` field (recently added to ze's `AddRoute`) is used
  when programming the kernel route

### 9.5 IPv6 Routes

IPv6 addressing for PPP subscribers involves two separate protocols:

1. **IPv6CP** (RFC 5072): negotiates only the 64-bit interface identifier.
   Does NOT assign addresses or prefixes. After IPv6CP completes, the pppN
   interface has a link-local address derived from the negotiated identifier.

2. **DHCPv6-PD** (RFC 3633): runs over the established PPP link to delegate
   an IPv6 prefix (e.g., /48 or /56) to the subscriber. This is a separate
   protocol from IPv6CP, requiring a DHCPv6 server or relay.

Alternatively, Router Advertisements (SLAAC, RFC 4862) can assign a /64
prefix to the subscriber.

When a delegated prefix is assigned (via DHCPv6-PD or SLAAC), it is
injected as a redistribute route:

```json
{
    "protocol": "l2tp",
    "family": "ipv6/unicast",
    "changes": [{
        "action": "add",
        "prefix": "2001:db8:1::/48",
        "next-hop": "::",
        "priority": 0,
        "protocol-type": "l2tp"
    }]
}
```

The /128 link-local route from IPv6CP is also injected if the subscriber
needs to be reachable by address.

---

## 10. Buffer-First Wire Encoding

All L2TP wire encoding follows `rules/buffer-first.md`. No `append()`,
no `make([]byte)` in encoding helpers, no `buildFoo() []byte`.

### 10.1 Pool design

```go
var controlBufPool = sync.Pool{
    New: func() any {
        b := make([]byte, 1500)  // Max L2TP control message over UDP
        return &b
    },
}
```

Control messages are small (typically < 512 bytes). A single 1500-byte pool
(matching UDP/Ethernet MTU) is sufficient.

### 10.2 Header encoding

```go
// writeControlHeader writes a 12-byte L2TP control message header.
// Returns the number of bytes written (always 12).
func writeControlHeader(buf []byte, off int, length uint16, tunnelID, sessionID, ns, nr uint16) int {
    binary.BigEndian.PutUint16(buf[off:], 0xC802)     // T=1,L=1,S=1,Ver=2
    binary.BigEndian.PutUint16(buf[off+2:], length)
    binary.BigEndian.PutUint16(buf[off+4:], tunnelID)
    binary.BigEndian.PutUint16(buf[off+6:], sessionID)
    binary.BigEndian.PutUint16(buf[off+8:], ns)
    binary.BigEndian.PutUint16(buf[off+10:], nr)
    return 12
}
```

### 10.3 AVP encoding (skip-and-backfill)

```go
// writeAVP writes an AVP at buf[off:] and returns bytes written.
// Uses skip-and-backfill for the length field.
func writeAVP(buf []byte, off int, mandatory, hidden bool, vendorID, attrType uint16, value []byte) int {
    start := off

    // Byte 0-1: flags + length (backfill length later)
    var flags uint16
    if mandatory {
        flags |= 0x8000
    }
    if hidden {
        flags |= 0x4000
    }
    lengthPos := off  // save position for backfill
    off += 2

    // Byte 2-3: Vendor ID
    binary.BigEndian.PutUint16(buf[off:], vendorID)
    off += 2

    // Byte 4-5: Attribute Type
    binary.BigEndian.PutUint16(buf[off:], attrType)
    off += 2

    // Byte 6+: Value
    copy(buf[off:], value)
    off += len(value)

    // Backfill length (total AVP length including 6-byte header)
    totalLen := uint16(off - start)
    flags |= totalLen & 0x03FF  // 10-bit length
    binary.BigEndian.PutUint16(buf[lengthPos:], flags)

    return off - start
}
```

### 10.4 Typed AVP helpers

These helpers write directly into the AVP value region. They call
`writeAVPDirect` which writes the 6-byte AVP header around an
already-written value, avoiding a `copy()` of the value into itself.

```go
// writeAVPDirect writes the AVP header for a value already at buf[off+6:off+6+valueLen].
// Returns total bytes written (6 + valueLen).
func writeAVPDirect(buf []byte, off int, mandatory bool, vendorID, attrType uint16, valueLen int) int {
    totalLen := 6 + valueLen
    var flags uint16
    if mandatory {
        flags |= 0x8000
    }
    flags |= uint16(totalLen) & 0x03FF
    binary.BigEndian.PutUint16(buf[off:], flags)
    binary.BigEndian.PutUint16(buf[off+2:], vendorID)
    binary.BigEndian.PutUint16(buf[off+4:], attrType)
    return totalLen
}

func writeAVPUint16(buf []byte, off int, m bool, attrType, value uint16) int {
    binary.BigEndian.PutUint16(buf[off+6:], value)
    return writeAVPDirect(buf, off, m, 0, attrType, 2)
}

func writeAVPUint32(buf []byte, off int, m bool, attrType uint16, value uint32) int {
    binary.BigEndian.PutUint32(buf[off+6:], value)
    return writeAVPDirect(buf, off, m, 0, attrType, 4)
}

func writeAVPString(buf []byte, off int, m bool, attrType uint16, s string) int {
    n := copy(buf[off+6:], s)
    return writeAVPDirect(buf, off, m, 0, attrType, n)
}
```

**Hidden AVPs**: the `writeAVP` function (which accepts a separate `value`
parameter and supports H=1) is used only for hidden AVP encoding. The
typed helpers above are for non-hidden AVPs only. Hidden AVPs encrypt the
value in a scratch region before writing, so they must not share the
destination buffer with the plaintext. See section 14 of the protocol guide.

### 10.5 Message construction

```go
// writeSCCRP writes a complete SCCRP message into buf.
// Returns total bytes written.
func writeSCCRP(buf []byte, tunnelID, sessionID, ns, nr uint16,
    assignedTID uint16, hostName string, framingCap uint32,
    challenge, challengeResp []byte) int {

    // Skip header (backfill length later)
    off := 12

    // Message Type AVP (MUST be first)
    off += writeAVPUint16(buf, off, true, 0, 2)  // type=0, value=2 (SCCRP)

    // Protocol Version
    buf[off+6] = 1  // ver
    buf[off+7] = 0  // rev
    off += writeAVP(buf, off, true, false, 0, 2, buf[off+6:off+8])

    // Framing Capabilities
    off += writeAVPUint32(buf, off, true, 3, framingCap)

    // Host Name
    off += writeAVPString(buf, off, true, 7, hostName)

    // Assigned Tunnel ID
    off += writeAVPUint16(buf, off, true, 9, assignedTID)

    // Challenge Response (conditional)
    if len(challengeResp) > 0 {
        off += writeAVP(buf, off, true, false, 0, 13, challengeResp)
    }

    // Challenge (optional)
    if len(challenge) > 0 {
        off += writeAVP(buf, off, true, false, 0, 11, challenge)
    }

    // Backfill header with total length
    writeControlHeader(buf, 0, uint16(off), tunnelID, sessionID, ns, nr)
    return off
}
```

---

## 11. Concurrency Model

Following ze's patterns: reactor model, not goroutine-per-connection.
Following `rules/goroutine-lifecycle.md`: long-lived workers only.

### 11.1 Reactor goroutine

Single goroutine reads from the UDP socket and dispatches:

```go
func (r *L2TPReactor) run(ctx context.Context) {
    buf := make([]byte, 1500)
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        n, addr, err := r.conn.ReadFromUDP(buf)
        if err != nil { /* handle */ }
        r.dispatch(buf[:n], addr)
    }
}
```

`dispatch()` determines the tunnel by Tunnel ID, delivers the message
to the tunnel's state machine. All tunnel/session state machines run
synchronously within the reactor goroutine (no per-tunnel goroutines).

### 11.2 Timer goroutine

Single goroutine manages all timers:
- Retransmission timers (per-tunnel)
- Hello keepalive timers (per-tunnel)
- Session establishment timeouts (per-session)

Uses a priority queue (min-heap by deadline). Wakes up on the nearest
deadline, processes all expired timers, calculates next wake.

### 11.3 PPP worker pool

PPP negotiation (LCP/auth/IPCP/IPv6CP) involves reading from `/dev/ppp`
file descriptors, which is blocking I/O. Use a fixed-size worker pool:

```go
type pppWorkerPool struct {
    sessions chan *pppSession
    wg       sync.WaitGroup
}
```

Each worker picks up a session, runs the PPP FSM to completion (or timeout),
then picks up the next. This bounds the number of concurrent PPP negotiations
without creating a goroutine per session.

### 11.4 Event emission

Events are emitted via `eventBus.Emit()` from the reactor goroutine.
The event bus handles fan-out to subscribers asynchronously.

---

## 12. Package Layout

```
internal/
  component/
    l2tp/
      l2tp.go              # Subsystem implementation (Start/Stop/Reload)
      reactor.go           # UDP reader + tunnel dispatch
      reactor_tunnel.go    # Tunnel state machine
      reactor_session.go   # Session state machine
      tunnel.go            # Tunnel data structures
      session.go           # Session data structures
      avp.go               # AVP types, parsing, serialization
      header.go            # L2TP header parsing, serialization
      reliable.go          # Ns/Nr, retransmission, sliding window, congestion
      auth.go              # Challenge/response computation
      hidden.go            # Hidden AVP encryption/decryption
      ppp.go               # PPP FSM (LCP, IPCP, IPv6CP)
      ppp_auth.go          # PPP authentication (PAP, CHAP, MS-CHAPv2)
      ppp_worker.go        # PPP worker pool
      netlink.go           # Kernel L2TP Generic Netlink interface
      pppox.go             # PPPoL2TP socket management, /dev/ppp ioctls
      events.go            # Event type constants, payload structs
      redistribute.go      # Redistribute source registration
      environment.go       # Env var registration
      logger.go            # Package logger
      register.go          # init() + engine.RegisterSubsystem()
      schema/
        register.go        # YANG module registration
        ze-l2tp-conf.yang  # YANG schema
  plugins/
    l2tpauth/
      l2tpauth.go          # RADIUS authentication plugin
      register.go
    l2tppool/
      l2tppool.go          # IP address pool plugin
      register.go
    l2tpshaper/
      l2tpshaper.go        # Traffic shaping plugin
      register.go
    l2tpstats/
      l2tpstats.go         # Statistics and accounting plugin
      register.go
```

---

## 13. File Descriptor Lifecycle

Each L2TP session involves multiple file descriptors with strict ordering:

```
Creation order:
1. UDP socket (tunnel level, shared across sessions)
   -> socket(AF_INET, SOCK_DGRAM, 0)
   -> bind(local_addr)
   -> connect(peer_addr)  [optional, for connected UDP]

2. Kernel tunnel (Generic Netlink)
   -> L2TP_CMD_TUNNEL_CREATE with UDP socket fd

3. Kernel session (Generic Netlink)
   -> L2TP_CMD_SESSION_CREATE with tunnel ID + session IDs

4. PPPoL2TP socket
   -> socket(AF_PPPOX, SOCK_DGRAM, PX_PROTO_OL2TP)
   -> connect(sockaddr_pppol2tp with tunnel/session IDs)

5. PPP channel fd
   -> open("/dev/ppp", O_RDWR)
   -> ioctl(PPPIOCGCHAN)  // on PPPoL2TP socket, get channel index
   -> ioctl(PPPIOCATTCHAN) // on /dev/ppp fd, attach to channel

6. PPP unit fd
   -> open("/dev/ppp", O_RDWR)
   -> ioctl(PPPIOCNEWUNIT)  // allocate pppN interface
   -> ioctl(PPPIOCCONNECT)  // connect channel to unit

Destruction order (MUST reverse):
1. Close PPP unit fd
2. Close PPP channel fd
3. Close PPPoL2TP socket
4. L2TP_CMD_SESSION_DELETE
5. L2TP_CMD_TUNNEL_DELETE (only if last session)
6. Close UDP socket (only if tunnel being destroyed)
```

Each fd must be tracked per-session and closed in order. A leaked fd
or out-of-order close orphans kernel resources.

---

## 14. Configuration Pipeline

### 14.1 Config flow

```
YANG schema (ze-l2tp-conf.yang)
  -> Schema registration (yang.RegisterModule)
  -> Config parser (File -> Tree)
  -> ConfigProvider delivers to L2TP subsystem at Start()
  -> L2TP parses into typed Go structs
  -> Config transaction events for reload
```

### 14.2 Config struct

```go
type l2tpConfig struct {
    ListenAddr       string
    ListenPort       uint16
    HostName         string
    SharedSecret     string
    HelloInterval    time.Duration
    RetransmitInit   time.Duration
    RetransmitMax    int
    RetransmitCap    time.Duration
    ReceiveWindow    uint16
    PPPMaxMTU        uint16
    HideAVPs         bool
    DataSequencing   string // "allow", "deny", "prefer", "require"
    ReorderTimeout   time.Duration
    MaxTunnels       int
    MaxSessions      int
}
```

### 14.3 Env var overrides

Following ze convention, env vars override config file values:
- `ze.l2tp.listen.port` -> ListenPort
- `ze.l2tp.hello.interval` -> HelloInterval
- `ze.l2tp.ppp.max.mtu` -> PPPMaxMTU

---

## 15. CLI Commands

Registered via YANG `ze:command` declarations:

| Command | Description |
|---------|-------------|
| `show l2tp tunnels` | List all L2TP tunnels with state, peer, session count |
| `show l2tp sessions` | List all sessions with state, interface, IP, username |
| `show l2tp tunnel <id>` | Detailed tunnel info (AVPs, timers, counters) |
| `show l2tp session <id>` | Detailed session info (PPP state, counters) |
| `show l2tp statistics` | Global L2TP statistics |
| `clear l2tp tunnel <id>` | Administratively close a tunnel |
| `clear l2tp session <id>` | Administratively close a session |

---

## 16. Metrics

Following ze's Prometheus metrics pattern:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_l2tp_tunnels_active` | Gauge | | Current active tunnel count |
| `ze_l2tp_sessions_active` | Gauge | | Current active session count |
| `ze_l2tp_sessions_total` | Counter | `result` (accepted/rejected) | Total session attempts |
| `ze_l2tp_tunnels_total` | Counter | `result` | Total tunnel attempts |
| `ze_l2tp_control_messages_total` | Counter | `type`, `direction` | Control messages sent/received |
| `ze_l2tp_retransmissions_total` | Counter | | Control message retransmissions |
| `ze_l2tp_auth_failures_total` | Counter | | Tunnel authentication failures |
| `ze_l2tp_ppp_negotiations_total` | Counter | `result` | PPP negotiation outcomes |

---

## 17. Functional Tests

### 17.1 Unit tests

| Area | Test File | Coverage |
|------|-----------|----------|
| Header parsing/serialization | `header_test.go` | Known byte sequences, round-trip |
| AVP parsing/serialization | `avp_test.go` | All 40 AVP types, hidden AVPs |
| Sequence number arithmetic | `reliable_test.go` | Wraparound, duplicate detection |
| State machine transitions | `tunnel_test.go`, `session_test.go` | Every transition in tables |
| Challenge/response | `auth_test.go` | Known test vectors |
| Hidden AVP crypto | `hidden_test.go` | Known plaintext/ciphertext pairs |
| Congestion control | `reliable_test.go` | Slow start, congestion avoidance |
| PPP LCP FSM | `ppp_test.go` | RFC 1661 state table |
| PPP IPCP | `ppp_test.go` | Address negotiation |

### 17.2 Fuzz tests (mandatory for wire format)

```go
func FuzzParseHeader(f *testing.F) { ... }
func FuzzParseAVP(f *testing.F) { ... }
func FuzzParseControlMessage(f *testing.F) { ... }
```

### 17.3 Functional tests (.ci)

| Test | Location | Scope |
|------|----------|-------|
| L2TP config parsing | `test/parse/l2tp-*.ci` | Valid/invalid config |
| L2TP tunnel establishment | `test/l2tp/tunnel-*.ci` | Full SCCRQ/SCCRP/SCCCN exchange |
| L2TP session with PPP | `test/l2tp/session-*.ci` | Full session lifecycle |
| L2TP + BGP redistribute | `test/l2tp/redistribute-*.ci` | Route appears in BGP |
| L2TP authentication | `test/l2tp/auth-*.ci` | Challenge/response, reject |

### 17.4 Integration tests

Require root (for kernel L2TP modules, /dev/ppp, netlink):
- Kernel tunnel/session creation and deletion
- PPPoL2TP socket lifecycle
- pppN interface creation
- End-to-end data path (ping through L2TP tunnel)

These can be tested against accel-ppp as a peer (LAC or LNS role).
