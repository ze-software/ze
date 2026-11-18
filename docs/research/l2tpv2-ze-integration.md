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

The shared PPP authenticator caches the PAP reply Code after a successful
reply write. During NCP negotiation and the established network phase, repeated
Authenticate-Requests receive that decision with their own Identifier, without
another authentication lookup. An LCP restart clears the cached decision.
<!-- source: internal/component/l2tp/ppp/pap.go -- runPAPAuthPhase, reanswerPAP -->

PAP, CHAP and MS-CHAPv2 result writers apply the admitted session transmit
limit before encoding. Optional display text can shrink; the authentication
outcome, packet identifiers and required MS-CHAPv2 authenticator/error fields
remain intact. PAP reanswers use an empty optional message. A required result
preface that cannot fit ends the attempt with an explicit failure.
<!-- source: internal/component/l2tp/ppp/pap.go -- runPAPAuthPhase, reanswerPAP -->
<!-- source: internal/component/l2tp/ppp/chap.go -- runCHAPAuthPhase -->
<!-- source: internal/component/l2tp/ppp/mschapv2.go -- runMSCHAPv2AuthPhase, fitMSCHAPv2FailureMessage -->

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
// internal/component/l2tp/yang/register.go

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

The sketch below is historical, not the current schema. The implemented
`ze-l2tp-conf.yang` uses root `l2tp mru` (68..1500 Information-field octets,
default 1500), with a minimum of 1280 when IPv6CP is enabled, and
`l2tp authentication allow` (user-ordered permitted methods).
An absent allow-list derives the configured method's policy: CHAP-MD5 only,
MS-CHAPv2 only, CHAP-MD5 then PAP for PAP, and empty for none. Original-tree
validation checks the allow-list against `auth-method`; both startup extraction
and strict provider reload parsing resolve it before new-session dispatch.
<!-- source: internal/component/l2tp/config_policy.go -- validateAuthPolicy, providerAuthAllow -->
<!-- source: internal/component/l2tp/config_policy.go -- validateMRUPolicy -->
<!-- source: internal/component/l2tp/ppp/auth_policy.go -- ResolveAuthAllow -->

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

The registration below is an early sketch. The current plugin consumes root
`l2tp` and binds the shared pool handlers at registration time.
The pool runs inside the daemon, either automatically or explicitly internal.
It refuses an external process before configuration, because that process
cannot configure the daemon's registered handlers.
<!-- source: internal/component/l2tp/plugins/pool/register.go -- init, runPlugin -->

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
- Answer address requests through the shared L2TP and PPPoE pool handler registry.
- Release addresses on subscriber `session-down` events.
- Support RADIUS-directed pool selection (Framed-Pool attribute).
- Expose pool statistics via CLI commands.

The pool uses `sdk.ConfigTx` to stage a declarative candidate at verify and
install it at apply. Installation rebuilds the bitmaps from live session
records. Removed ranges leave their live assignments with the sessions.
Rollback repeats that reconciliation against the previous configuration.
Allocation, record insertion, release and reconciliation share the pool
plugin's locks, so an assignment cannot fall between the old and new bitmaps.
<!-- source: internal/component/l2tp/plugins/pool/reload.go -- configTransaction, installPoolConfiguration -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- setEventBus, handle, handlePrefix, onSessionDown -->

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
- Handle rate changes from RADIUS CoA through the synchronous
  `request l2tp shaper rate` transaction command. The owner prepares its complete
  last successfully programmed QoS, applies both directions, and confirms the
  backend result. Rollback reapplies that confirmed state, including after a
  partially failed backend call. One pending transaction and one completed token
  are retained. Pending transactions expire after one minute; the next admitted
  owner command restores the prior state before handling its operation. A failed
  restore retains the original token and snapshot for retry, and a lost prepare
  reply cannot occupy the slot forever. Tokens belong to the requesting Dynamic
  Authorization Client's address. Session removal invalidates tokens so a late rollback cannot restore
  a departed subscriber's shaping. The listener ACKs only after every requested
  owner confirms its apply. Post-commit rate events remain notifications.

<!-- source: internal/component/l2tp/plugins/shaper/rate_transaction.go -- rateOperation, prepareRate, applyRate, restoreRate -->
<!-- source: internal/component/l2tp/plugins/authradius/coa_transaction.go -- confirmCoA -->
- Clean up TC rules on `(l2tp, session-down)`.

The hidden transaction command is registered by `commandDecls` and validates
its keyword, bounded JSON value and authenticated source in `rateOperation`.
The configuration module holds the persistent `l2tp shaper` settings.
<!-- source: internal/component/l2tp/plugins/shaper/register.go -- commandDecls -->
<!-- source: internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang -- container l2tp -->

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

L2TP runtime plugins are wired via blank imports in
`internal/component/plugin/all/all.go`, following the same pattern as BGP
plugins:

```go
_ "github.com/ze-software/ze/internal/component/l2tp/plugins/authlocal"
_ "github.com/ze-software/ze/internal/component/l2tp/plugins/authradius"
_ "github.com/ze-software/ze/internal/component/l2tp/plugins/pool"
_ "github.com/ze-software/ze/internal/component/l2tp/plugins/shaper"
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
7. L2TP runs IPCP negotiation (via /dev/ppp channel fd; after the
   post-NCP PPPIOCCONNECT, later IPCP and IPv6CP frames arrive on the unit
   fd, which the session's unit reader reads)
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

Session monitoring follows a separate original observation scope. Start mints
one nonzero native generation before publishing listeners, binds its metrics
before constructing CQM, and assigns that original observer to every reactor.
Lifecycle notices retain the reactor's generation even when terminal delivery
is delayed. Retired native roots keep the matching observer until their exact
cleanup finishes; Stop seals copied callbacks before unsubscribe. Event history
is keyed by complete Birth, while bounded login slots retain their quality
phase contributor and original metric-series leases.
<!-- source: internal/component/l2tp/subsystem.go -- Start, Stop, unwindLocked -->
<!-- source: internal/component/l2tp/observer_events.go -- observeEvent -->
<!-- source: internal/component/l2tp/session_retirement.go -- retireObserverLocked -->

Each original logical session carries an immutable packed Birth and a cached
canonical subscriber ID. The actor captures framed-route metadata with that ID
before releasing its row lock. Its ordered IP-assignment callback supplies the
Birth and captured route slice to the route observer; labels remain diagnostic.
The observer retains one record per complete Birth and copies the route input.
<!-- source: internal/component/l2tp/session.go -- Birth, SubscriberID -->
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleSessionIPAssigned -->
<!-- source: internal/component/l2tp/route_observer.go -- OnSessionIPUp -->

A separate nonzero native contribution token belongs to each route lifetime.
The record preserves it on address and framed-route adds, replay and removal.
Token allocation is a cold operation that refuses exhaustion instead of
recycling an existing contribution. Equal wire labels or prefixes do not merge
two original sessions.
<!-- source: internal/component/l2tp/route_source.go -- routeSourceSequence, next -->
<!-- source: internal/component/l2tp/route_observer.go -- routeRecord, reemitAll, OnSessionDown -->

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
    distance {
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

The PPP admission cap must be at least 1280 when IPv6CP is enabled. Before
starting IPv6CP, the driver also checks the negotiated directional MRUs. A
smaller peer limit withholds IPv6 address allocation, negotiation and service;
independent IPCP can still establish IPv4. If IPCP is disabled, the unusable
IPv6-only session fails explicitly. An LCP restart reevaluates these sizes
without changing the admitted NCP policy.
<!-- source: internal/component/l2tp/ppp/mru_policy.go -- ValidateNetworkMRU, ipv6MRUSupported -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- runNCPPhase, onNCPOpened -->

1. **IPv6CP** (RFC 5072): negotiates only the 64-bit interface identifier.
   Does NOT assign addresses or prefixes. After IPv6CP completes, ze installs
   the link-local pair on pppN itself, `fe80::<local identifier>/64` peer
   `fe80::<peer identifier>/64` (RFC 5072 Section 5), before the RA sender and
   the DHCPv6 server start, and removes it when the IPv6 service stops. The
   kernel would not: `addrconf_dev_config` (`net/ipv6/addrconf.c`)
   autoconfigures only the device types it lists, and ARPHRD_PPP is not one,
   so without it every Router Advertisement on pppN would leave with no
   link-local source (RFC 4861 Section 4.2)
   (`installLinkLocal`, `internal/component/l2tp/ppp/ipv6_service.go`).

   IPv6CP can reach Opened without ever learning the peer's identifier (the
   peer's Configure-Request carried no Interface-Identifier option). ze
   tracks that fact separately from the identifier value itself
   (`pppSession.peerInterfaceIDNegotiated`, `internal/component/l2tp/ppp/session.go`):
   an all-zero identifier is both the Go zero value and the one value RFC
   5072 Section 4.1 never lets stand as the negotiated result, so the value
   alone cannot tell "never negotiated" from "negotiated to a value ze would
   refuse". When the fact
   is false, ze starts no IPv6 service, installs no route, and publishes no
   session-up address for that family; the session stays up IPv4-only
   (`afterLCPOpen`, `internal/component/l2tp/ppp/session_run.go`).

   RFC 5072 Section 4.1 also states what ze answers for each comparison
   outcome, evaluated in `evalIPv6CPRequest`
   (`internal/component/l2tp/ppp/ncp.go`). A request naming no
   Interface-Identifier option draws a Configure-Nak with a suggestion the
   FIRST time; every later request that still omits it is Acked instead, so a
   peer whose implementation does not support the option converges rather than
   being asked forever. A request naming the option twice is refused the same
   as any other malformed option list (`parseIPv6CPOptions`, `ipv6cp.go`). A
   request whose identifier equals ze's own AND is zero draws a
   Configure-Reject carrying a zero identifier, which ends that identifier's
   negotiation. ze's own local identifier is never legitimately zero
   (`generateIPv6CPInterfaceID` retries until it draws a valid one), so the
   last case is a defensive branch rather than one a conformant peer reaches
   in practice.

   Every Configure-Nak's suggestion is drawn by `suggestIPv6CPInterfaceID`
   (`internal/component/l2tp/ppp/ipv6cp.go`), never read raw from
   `peerInterfaceID`: it retries `generateIPv6CPInterfaceID`'s draw until the
   value differs from ze's own `localInterfaceID` (the identifier of ze's
   last-sent Configure-Request, which RFC 5072 Section 4.1's comparison is
   against) and clears the "u" (universal/local) bit -- canonical bit 6,
   `octet[0] & 0x02` -- which the RFC requires of a suggested identifier ze
   never derives from a globally unique EUI-48/EUI-64 source. The result
   passes `isValidIPv6CPInterfaceID`, the same validator ze applies to a
   received identifier, so ze never proposes a value it would itself refuse.

2. **DHCPv6-PD** (RFC 3633): runs over the established PPP link to delegate
   an IPv6 prefix (e.g., /48 or /56) to the subscriber. This is a separate
   protocol from IPv6CP, requiring a DHCPv6 server or relay.

Alternatively, Router Advertisements (SLAAC, RFC 4862) can assign a /64
prefix to the subscriber.

The session IPv6 carrier contract separates these protocols from the socket
that carries them. `IPv6Carrier` provides cancellable solicitation and DHCPv6
reads, RA and DHCPv6 writes, and one idempotent close. Read cancellation keeps
the write path available: the owner joins the DHCPv6 and solicitation workers,
sends the final RA, then closes the carrier. A carrier borrowing a PPP frame
endpoint must never close that endpoint or start a second reader of it.
The kernel carrier owns dedicated ICMPv6 and UDPv6 sockets on the created
interface; its cancellation changes only the read deadline.

The kernel carrier's ordinary `Close` keeps its first result, including failures.
`RetryIPv6CarrierCleanup(ctx)` reconciles only its original failed socket owners;
it never closes a successful socket again or retries a numeric file descriptor.
Concrete Linux socket owners can confirm that their original Go socket has
closed. An opaque provider without an explicit recovery capability remains
unconfirmed. If acquisition fails and socket cleanup also fails, the returned
carrier owns cleanup only: its reads and writes refuse service. The caller must
retain that owner alongside the acquisition error until cleanup is confirmed.
<!-- source: internal/component/l2tp/ppp/kernel/ipv6_cleanup.go -- ipv6SocketCleanup, failedIPv6Acquisition -->
<!-- source: internal/component/l2tp/ppp/kernel/ipv6_socket_cleanup.go -- nativeIPv6SocketCleanup -->

Startup installs the negotiated link-local pair, reconciles forwarding through
the existing session handle, then opens its carrier. The assignment has already
been acknowledged; neither RA nor DHCPv6 workers start before this reconciliation.
If reconciliation or carrier acquisition fails, the service withdraws that pair before returning.
A failed withdrawal returns both causes and retains the partial service in the
session. NCP reset and final teardown retry its existing cleanup lifecycle;
they do not forget an address whose removal was never confirmed. The original
carrier refusal remains nonfatal to an independent network protocol. A failed
rollback also raises the retained PPP cleanup warning for the created interface.
<!-- source: internal/component/l2tp/ppp/ipv6_service_start.go -- startIPv6Service, rollbackIPv6ServiceStart -->
<!-- source: internal/component/l2tp/ppp/session_run.go -- afterLCPOpenIPv6Service, stopIPv6Service -->
<!-- source: internal/component/l2tp/ppp/ipv6_carrier.go -- IPv6Carrier -->
<!-- source: internal/component/l2tp/ppp/kernel/ipv6.go -- carrierRead, Close -->

The VPP carrier borrows the existing PPP endpoint instead. Its one control
reader validates direct IPv6 RS and DHCPv6 datagrams, including their
pseudoheader checksums, and hands admitted controls to two fixed-depth queues.
It constructs outbound IPv6 RA and UDPv6 packets itself, using the actual
IPv6CP local link-local source already installed by the service. Received
RS and DHCPv6 source addresses need not match the IPv6CP peer identifier.
The shared RS validator permits unspecified or unicast sources. RFC 9915
Sections 5 and 7.2 allow DHCPv6 clients to use other unicast addresses and
source ports; Section 18.3.10 requires a reply to the received source address,
through the same link, and Section 7.2 fixes its destination port at 546.
This provider accepts client requests to `ff02::1:2` port 547 and rejects
the obsolete server-unicast request delivery, following Section 16.
Closing the carrier
unregisters the queues and wakes reads; only the session closes the endpoint.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/ipv6_carrier.go -- ipv6Carrier.Close, ipv6Carrier.WriteRA, ipv6Carrier.WriteDHCPv6 -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/socket.go -- controlSocket.Read -->

Solicitation admission checks the received interface and Hop Limit before it
signals the RA scheduler. The shared body validator checks the type, code,
minimum length and bounded option lengths, and rejects a source link-layer
option with an unspecified source. Reserved fields, unknown options and
options defined for other ND messages remain accepted. The kernel ICMPv6
receive path owns checksum validation; a carrier reading full PPP IPv6 frames
must validate that checksum before calling the body validator.
The source check is Linux v6.18 `net/ipv6/raw.c`: `rawv6_init_sk` enables
checksum verification for ICMPv6, and `rawv6_rcv_skb` refuses a bad checksum
before queueing the packet. This source check does not replace the shipped
kernel's receiver proof.
<!-- source: internal/component/l2tp/ppp/router_solicitation.go -- ValidRouterSolicitation -->
<!-- source: internal/component/l2tp/ppp/kernel/ipv6.go -- ReadRS -->

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

The secondary tunnel index uses the peer endpoint and peer TID captured by
`newTunnel`, separately from the latest control reply address. Registration
and removal use that immutable key. Refreshing the reply address during
dispatch leaves index membership unchanged. Discard removes an entry only
while it still points to that original tunnel, preserving a fresh admission
or another peer's entry.
<!-- source: internal/component/l2tp/tunnel.go -- newTunnel -->
<!-- source: internal/component/l2tp/reactor.go -- locateTunnelLocked, handle, discardTunnelLocked -->

### 11.2 Timer goroutine

Single goroutine manages all timers:
- Retransmission timers (per-tunnel)
- Hello keepalive timers (per-tunnel)
- Session establishment timeouts (per-session)

The worker keeps deadlines in a min-heap and waits for the nearest one. A due
tick remains pending until the reactor receives it. While delivery is blocked,
the worker continues accepting deadline updates and stop signals. Updates for
the same tunnel ID keep their own heap entry, preserving the new deadline.
<!-- source: internal/component/l2tp/timer.go -- run -->

### 11.3 PPP session ownership

Each admitted PPP session receives a borrowed `FrameChannel` and its owning
`SessionHandle` through `StartSession`. The protocol driver holds no descriptors
or global interface backend. Its main goroutine drives LCP, authentication and
the NCPs, and starts one reader per supplied lane. The kernel handle supplies
separate Control and Opened streams; a single-stream backend omits Opened.
The handle supplies the created interface's identity and performs typed address,
route, MTU, activation and counter operations for that session.

Address cleanup is idempotent within that owner: no owned receipt means no
native delete is issued. An attempted add records its receipt before mutation,
and an uncertain add or failed withdrawal retains it until cleanup succeeds.
This distinguishes a known refusal before submission from an unknown native
outcome without deleting another owner's address or discarding an error.

L2TP owns a separate OS interface backend for its kernel PPP sessions. It opens
that backend through the iface registry without changing the configured active
backend before publishing a native worker. A local admission captures that
original backend's unit creator before allocating a tunnel, session or PPPoX
transport. The resulting unit retains its creation-bound counter reader;
reactors never reread the global backend when a session arrives. A LAC bridge
attaches only its original channel and requires no unused local unit creator. The
subsystem closes this owner after workers, PPP drivers and their drains stop;
changing the active backend to VPP cannot redirect a Linux `pppN` operation.
<!-- source: internal/component/l2tp/subsystem.go -- Start, Stop, unwindLocked -->
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleKernelSuccess -->

PPPoE binds session admission to the original prepared access through
`AccessOwner.Open`. Kernel preparation opens one independent netlink backend
and captures its original unit creator before publication. Child interface
operations, ordered creation and creation-bound counter readers retain that
original backend. Typed channel and unit owners transfer through the original
unit's non-consuming `SessionConfig` and the consuming real PPP constructor;
an active-backend replacement cannot redirect them. VPP opens through its
captured preparation and API connection generation. The caller MUST stop
admission and join sessions and reads. It MUST then close the access.
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- AccessOwner, preparedAccess.Open -->
<!-- source: internal/component/l2tp/pppoe/dataplane/kernel/prepare.go -- Prepare, prepareWithCreator, kernelAccessOwner.Open, kernelAccessOwner.Close -->
<!-- source: internal/component/l2tp/pppoe/dataplane/kernel/factory.go -- openWithOps, preparedFactoryOps -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/prepare.go -- accessPreparationLease.Open, accessPreparation.borrow -->

`SessionCounters` keeps numeric receive/transmit bytes and packets. Its receive/
transmit error and drop fields each carry a `CounterReading`. Consumers call
`Value` on each diagnostic.

A measurement includes a valid zero. Unknown
returns `ErrCounterUnavailable` and must be shown with that reason. Diagnostics
default to Unknown until a provider supplies a native measurement. These fields
alone do not establish a backend counter reader.
<!-- source: internal/component/l2tp/ppp/session_handle.go -- SessionCounters -->
<!-- source: internal/component/l2tp/ppp/counter_reading.go -- CounterReading.Value -->

The shared kernel library also provides an original-unit creator. It creates
named and unnamed PPP units through the supplied backend's ordered creation
socket, retaining the actual ECHO identity, original unit file and that socket's
generation-bound counter reader. A renamed original remains readable from
another caller namespace; native deletion, movement, lost history and backend
closure refuse the reader. Original-file POLLHUP checks and a final closed-state
check discard a late snapshot without holding a descriptor lease or owner lock
across a counter query. On creation failure, any returned unit still owns its
original file, including a partial echo, uncertain reply or provider panic; it
cannot supply counters until creation was confirmed. Transport setup returns
that original unit and its original attached channel as typed owners. The
non-consuming SessionConfig snapshot preserves both originals until NewSession
claims each fresh owner independently on every outcome. A copied, replayed,
closed, substituted, wrong-backend or wrong-namespace owner is refused; checked
numeric fields cannot replace either original file.

The same original unit supplies a passive `WaitNativePPPUnitRetired` join. It
never starts cleanup. The caller first arranges final accounting, disconnects
PPP, interrupts readers and closes the exact original owner. A successful join
requires the actual original unit-file Close, all short original-file borrows
and the library's one retained original duplicate to finish successfully. The
native-release seal can keep that duplicate alive after borrowed streams stop;
the join remains pending until the same seal permits its exact close. Failed
unit or duplicate closes remain errors and are never retried by numeric file
descriptor. Namespace-file cleanup errors stay in Close's result independently
of confirmed native-unit retirement. No name, index, snapshot or inaccessible
query can prove retirement. The join holds no owner lock or descriptor lease
while waiting, and a delayed counter reply does not delay original unit Close.
<!-- source: internal/component/l2tp/ppp/kernel/created_unit_retirement.go -- CreatedUnit.WaitNativePPPUnitRetired, publishNativeUnitRetirementLocked -->
<!-- source: internal/component/l2tp/ppp/kernel/session_unit_retention.go -- CreatedUnit.retainNativeUnit, nativePPPUnitHold.close -->

The provider never closes the unit on the caller's behalf. Native withdrawal
that needs retirement to settle must not block final exact-owner Close, and a
native-release seal must not wait for that same retirement before releasing
its duplicate. The caller retains uncertain writers and cleanup causes through
this ordering; a passive join cannot resolve a cycle in its caller's ownership.
<!-- source: internal/component/l2tp/ppp/kernel/created_unit_retirement.go -- CreatedUnit.WaitNativePPPUnitRetired -->

The kernel session constructor consumes the original typed channel and unit
owners. The channel producer proves the original PPPoX socket namespace and
attaches its original file; the unit producer retains its actual ECHO identity,
counter generation and creator backend. Both producers retain native namespace
files, whose identity must agree before admission. The constructor claims both
resources independently, so a fresh sibling is closed even if the other owner
was copied or already consumed. Descriptor labels supply no resource custody.
The interruptible streams keep those same original files. Original-file death
refuses admission, and every partial wrapper or panic retains cleanup custody.
Counter reads release the operation gate and discard a reply after Close or an
identity, native birth or original lease change. Rename preservation concerns
raw counter reads. Existing name-addressed interface operations retain their
current contracts.
<!-- source: internal/component/l2tp/ppp/kernel/created_unit.go -- UnitCreator.Create, CreatedUnit.Counters, CreatedUnit.Close -->
<!-- source: internal/component/l2tp/ppp/kernel/created_unit_linux.go -- NewUnitCreator, createOriginalPPPUnit, pollOriginalPPPUnit -->
<!-- source: internal/component/l2tp/ppp/kernel/created_channel_linux.go -- CreatePPPChannel, originalPPPChannelNamespace, captureOriginalPPPChannel -->
<!-- source: internal/component/l2tp/ppp/kernel/session_unit_owned.go -- CreatedUnit.SessionConfig, claimedCreatedUnit.describe -->
<!-- source: internal/component/l2tp/ppp/kernel/session_namespace.go -- capturedSessionNamespace.sameSessionNamespace -->
<!-- source: internal/component/l2tp/ppp/kernel/relay_owned_unit_linux.go -- newInterruptibleSessionFile -->

The required compiled original-resource PPP ABI is inspected with
`PPPIOCGSUPPORT`, query42, on one freshly opened, unbound `/dev/ppp` file.
The 16-byte ABI1 request has zero features and reserved fields. A positive
reply must preserve the ABI and zero reserved fields, contain only the declared
bits 0 through 10, and include capture8 and connect6. An actual bridge role
also requires bridge7. Bit9 declares original-unit IPv6 control sockets, and
bit10 declares exact address-pair deletion; these roles do not require either
optional bit. Unknown bits, including bit11, are refused. This compiled-ABI
declaration does not grant original-resource ownership.
The file stays retained through the actual ioctl and is then closed exactly
once. The query allocates no PPP unit/channel and changes no forwarding mode,
attachment or bridge. `ENOTTY` or a valid reply missing a required bit is
Unavailable; permission, other refusal, malformed reply and failed inspection
or cleanup are Unknown. Every original inspection and close cause is retained.
No errno, including `EBADF`, is a positive declaration.
<!-- source: internal/component/l2tp/ppp/kernel/original_attachment_linux.go -- inspectOriginalAttachmentWith, inspectOriginalAttachmentFile, validateOriginalSupportReply -->

The existing doctor registry carries three disjoint configured scopes:
`l2tp-original-ppp-support` for enabled L2TP, with bridge7 when its relay or
relay-realm role needs bridging; `pppoe-original-ppp-support` for an enabled
access concentrator with access interfaces and the actual schema-backed
`netlink` session backend; and `pppoe-client-original-ppp-support` for active
CPE entries. A VPP-only access concentrator, disabled client, or unrelated
configuration does not acquire a native PPP dependency. L2TP registers in its
own package, so its check remains reachable without importing PPPoE or the
CPE. Findings use `doctor-ppp-original-unit-attachment`, with their own role
path; the common kernel package owns that code's explanation and the single
probe producer. No check publishes mutable global health state.
<!-- source: internal/component/l2tp/register_original_ppp_support_linux.go -- l2tpOriginalPPPSupportScope -->
<!-- source: internal/component/l2tp/pppoe/register_original_ppp_support_linux.go -- pppoeOriginalPPPSupportScope -->
<!-- source: internal/component/l2tp/pppoeclient/register_original_ppp_support_linux.go -- clientOriginalPPPSupportScope -->

The CPE prerequisite calls the same real probe before opening its independent
backend or allocating a transport. Typed pair construction queries again in
the retained original namespace before taking pair/owner locks or file leases.
Bridge preparation performs the bridge-specific declaration in its retained
original namespace. None of these declarations establishes a native permission
grant or original resource authority: positive unit51, channel46 and
transport44 receipts on the exact original files remain mandatory. Every
actual operation still validates its original objects, namespace, lifetime
and permission. A late `ENOTTY` remains a refusal with its original cause;
there is no numeric attachment fallback and unit input remains disabled.
<!-- source: internal/component/l2tp/pppoeclient/client_owner.go -- prepareClientDial -->
<!-- source: internal/component/l2tp/ppp/kernel/session_pair_linux.go -- prepareOriginalSessionPair -->
<!-- source: internal/component/l2tp/ppp/kernel/original_bridge_support_linux.go -- checkOriginalBridgeSupport -->

An unavailable dependency names the native runtime kernel build commands:
`./le build host-driver`, then
`./ze-host appliance kernel --target runtime --arch <Ze binary architecture>`.
Those commands do not install or boot the image. This platform's kernel image
owner must install and boot the matching image; a firmware binary self-update
does not replace the kernel. Unknown instead directs the operator to
`ze show doctor` to inspect the daemon's own configured scope, device access
and native capability grant. The answer includes actual uname release,
version and machine. Those fields identify the answering kernel but certify
neither the selected profile, patch series nor exact image bytes. A failure
to obtain provenance also remains Unknown with its cause.
<!-- source: internal/component/l2tp/ppp/kernel/original_attachment_linux.go -- originalAttachmentDoctorCheck, originalAttachmentKernel, originalAttachmentFix, originalAttachmentRefusal -->


Kernel sessions obtain all eight native traffic, error and drop totals through
the original unit's creation-bound reader. Error and drop totals use measured
readings, including zero. A failed read or missing snapshot returns an error
without a counter sample. No caller-supplied name callback establishes counter
ownership. The session releases its mutex before the original-unit query and
checks its closure and exact retained lease again before returning the sample;
a sample arriving after Close is rejected.
<!-- source: internal/component/l2tp/ppp/kernel/created_unit.go -- CreatedUnit.Counters -->
<!-- source: internal/component/l2tp/ppp/kernel/session.go -- session.Counters -->

The worker keeps the original `CreatedChannel` and `CreatedUnit` pointers until
the reactor claims the exact worker receipt. Preparing their `SessionConfig`
is non-consuming on every outcome. The reactor checks that receipt before
fallible capability or snapshot checks, then rechecks both the original owner
and resource pair before transferring custody. A stale event cannot publish a
failure for a replacement subscriber. Bridged calls and calls without a local
PPP driver retain their resources in the worker.

The reactor constructs a kernel session through its own factory binding, fixed
before the reactor starts. Production binds `kernel.NewSession`, which claims
each fresh original channel and unit independently on every outcome, including
a malformed or replayed sibling. After the call, neither the worker nor reactor
uses or closes those original files. Numeric descriptor fields are checked
diagnostic labels and never establish ownership. Protocol fixtures can supply
their own socket-owned streams through private fault seams; those seams do not
grant a valid public kernel owner.
<!-- source: internal/component/l2tp/kernel_ppp_owners.go -- kernelPPPOwners -->
<!-- source: internal/component/l2tp/kernel_linux.go -- preparePPPTransfer, takePPPConfig -->
<!-- source: internal/component/l2tp/ppp/kernel/session_unit_owned.go -- CreatedUnit.SessionConfig -->

Factory refusals and ownership conflicts raise `doctor-ppp-session-admission`
through the existing runtime `recent-events` check. The retained event names
the affected call and original cause, including any joined rollback error;
`ze show errors` opens its details. IPv6 carrier read, parse, handler and write
failures, and a refused service replacement, similarly raise
`doctor-ppp-ipv6-service` with the interface and failed operation. Plain or
singly wrapped cancellation or closure after the reader's context ends is
normal. Composite errors remain visible with all their causes, as does an
unexpected closure or unrelated backend error. These
are historical events, distinct from unconfirmed removal. A refused session
whose owning handle fails to close uses the existing retained
`doctor-ppp-cleanup` warning.
<!-- source: internal/component/l2tp/reactor.go -- newL2TPReactor -->

The transport also supplies its canonical subscriber ID in `StartSession`.
The driver copies that opaque value at admission and stamps every authentication
and address request in `awaitAuthDecision` and `awaitIPDecision`. PPP neither
constructs an access namespace nor derives subscriber identity from its uint16
routing pair. A protocol-only caller may omit the ID; production subscriber
drains require it. Transport producers use the shared `subscriber.SessionID`
formatter with their full transport identifier before narrowing an interface
index for PPP routing.
<!-- source: internal/component/l2tp/ppp/manager.go -- spawnSession -->
<!-- source: internal/component/l2tp/ppp/auth.go -- awaitAuthDecision -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- awaitIPDecision -->
<!-- source: internal/component/l2tp/subscriber/id.go -- SessionID -->

Local shutdown stops IPv6 service before closing the frame owner. The run's
finalizer cancels both readers' queued frame sends, joins both readers and
returns queued frames to the pool. This per-run cancellation also covers a
natural session failure, when neither the per-session nor whole-driver stop
signal has fired. The first session-down publication
also stops IPv6 first: receiving that event permits the transport to send PADT.
A peer PADT instead gates its transport immediately, so service cleanup cannot
emit frames after the peer ended the session. Rejected or canceled queue
transfers release their distinct supplied handle as well. An admitted owner
cannot transfer again, even under another key or through a malformed payload.
The driver reserves each owner until its worker exits and removes only that
worker's incarnation, preserving a replacement admitted under its old key.
`EventSessionRejected.ExistingSession` is true only when the incoming key was
already active at refusal; the transport must preserve that admitted session.
Reusing its owner under a different key leaves the flag false, so the refused
new transport can still be released.
<!-- source: internal/component/l2tp/ppp/session_run.go -- run, closeFiles, drainFramesToPool -->
<!-- source: internal/component/l2tp/ppp/start_session.go -- StartSession, closeFiles -->
<!-- source: internal/component/l2tp/ppp/session_handle.go -- SessionHandle -->

The LCP Restart timer is a stopped, per-session one-shot timer. Only a
Configure-Request, Terminate-Request or Zero-Restart-Count action arms it.
Each request transmission, including the first, spends one Restart count.
Expiry selects TO+ or TO- from the remaining count. The session's immutable
LCP configuration supplies the interval and Max-Configure or Max-Terminate.
<!-- source: internal/component/l2tp/ppp/session_run.go -- applyTransition, armRestartTimer, spendRestartCount, handleRestartTimeout -->

Valid Nak/Reject replies can renew the RFC counter, so a separate convergence
deadline bounds each negotiating lifetime at twice its configured silent-peer
budget. A silent peer still exhausts the default ten requests in 30 seconds.
A responsive nonconverging peer has 60 seconds at the defaults. Configured
longer intervals are not cut short by an independent fixed 30-second timer.
<!-- source: internal/component/l2tp/ppp/session_run.go -- updateLCPConvergence -->

The actual LCP transition stops the convergence timer on entering Opened and
renews it on entering a new negotiation. This also applies when authentication
or NCP processes a Configure-Request before it returns to the outer loop.
A nested restart receives a full bound rather than the initial lifetime's remainder.
<!-- source: internal/component/l2tp/ppp/session_run.go -- applyTransition, armRestartTimer, spendRestartCount, handleRestartTimeout, updateLCPConvergence -->

Max-Failure counts successfully sent Configure-Naks since the last successfully
sent Configure-Ack. After the configured number of Naks, further unacceptable
peer options draw a Configure-Reject carrying their original bytes. Permitted
and absent locally desired options are excluded from that Reject. The shared
converter preserves the first instance that earned each Nak. If a malformed
duplicate caused the Nak after a permitted instance, only the malformed
instance is rejected. The permitted instance is retained.
<!-- source: internal/component/l2tp/ppp/session_run.go -- sendConfigureNakOrReject, sendConfigureAck, LCPRejectNakOptions -->

Leaving LCP Opened first disconnects the PPP channel and removes its NCP
resources. EventLCPDown then waits for the transport's network-lifetime cleanup
acknowledgment before the accompanying LCP responses are sent. Authentication
and NCP negotiate anew on the same transport. An Echo keeps Opened and invokes
none of this cleanup. Failed route or address withdrawal retains its ownership
state and refuses reuse of the handle. A kernel reconnect also waits for the
Opened reader's EOF acknowledgment and discards frames from the prior lifetime.
<!-- source: internal/component/l2tp/ppp/session_run.go -- run, applyTransition, performAction, sendConfigureNakOrReject -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- resetNCP -->

Each NCP also owns an independent network lifetime. Leaving its Opened state
withdraws only that family's resources before any replacement Configure output.
IPCP retains the prefix actually installed separately from its mutable proposal,
so a Nak cannot redirect cleanup to the proposed replacement address. IPv6CP
stops its existing service and keeps that owner if withdrawal fails. Normal
Closed or Stopped NCP state leaves the other family and PPP link available;
mandatory-option rejection retains its separate refusal policy. A failed
withdrawal ends negotiation and prohibits reusing the unresolved lifetime.
Each family owns its own bounded Restart counter and timer, driven by the same
RFC 1661 transition table as LCP. A peer's Terminate-Request receives one
Terminate-Ack and enters its termination grace; the timer settles Stopping to
Stopped without an acknowledgment of that Ack. Both the initial convergence
loop and the active session dispatch these timers. Once only termination grace
remains, the initial convergence deadline cannot shorten it or end an
independently opened family. LCP down and final session exit stop both timers.
The convergence deadline is disabled while only that grace remains. If another
family resumes Configure negotiation before the grace finishes, it gets a new
bounded convergence interval; valid replies inside that interval may reset the
RFC Restart budget but cannot extend the convergence deadline.

A connected reopen installs IPv4 before publishing its assignment, waits for
the existing acknowledgment, then reconciles forwarding. IPv6 waits for the
same acknowledgment before restarting its service and installing its pair.
The session's single frame-dispatch loop is blocked at that acknowledgment,
so another NCP cannot stage an unacknowledged address during reconciliation.
Concrete handles stage address ownership without granting traffic; repeated
Connect calls reconcile eligible families without repeating physical attachment.
The kernel handle drops both protocol modes before its first attachment, and
drops the retiring family before address removal. Failed or uncertain address
operations retain cleanup-only ownership which later reconciliation cannot
enable; the handle bounds these address witnesses at 1024.
Initial activation still preserves MTU, channel connection, receive MRU and
admin-up ordering, with the later IPv6 pair reconciled before its carrier opens.
<!-- source: internal/component/l2tp/ppp/ncp.go -- performNCPAction, onNCPOpened, reconcileOpenedNCP, withdrawNCPFamily -->
<!-- source: internal/component/l2tp/ppp/ipv6_service_start.go -- startIPv6Service -->
<!-- source: internal/component/l2tp/ppp/kernel/session.go -- Connect, AddAddressP2P, RemoveAddress -->

### 11.4 Event emission

PPP admission refusal distinguishes an already admitted session key from a
new call. A duplicate refusal preserves the existing L2TP session and queues
no CDN. A refused new call is removed and receives a CDN, including when its
PPP owner already belongs to another key.
<!-- source: internal/component/l2tp/reactor_kernel.go -- handlePPPEvent -->

A session event is emitted via `eventBus.Emit()` on the goroutine that changed
the session: the reactor goroutine, or the operator, timeout or CoA goroutine
that tore the session down. An in-process subscriber runs on that goroutine,
during the `Emit`. One tunnel's session events are delivered in the order its
sessions changed state, through the tunnel's order ticket
(`docs/architecture/l2tp/subscriber-session-model.md`, "The order of one L2TP
session's notifications").

<!-- source: internal/component/l2tp/events/events.go -- the reader contract in the package doc -->
<!-- source: internal/component/l2tp/session.go -- sessionNoticeOrder, issueNoticeTicket -->

The tunnel lifecycle events (`tunnel-up`, `tunnel-down`) are emitted from the
reactor goroutine.

### 11.5 Receiver goroutines and a failing socket

Four receiver workers consume network input and dispatch what arrives: the
UDP listener's `readLoop` (`listener.go`), the PPPoE discovery reader
(`pppoe/subsystem.go`), and the RA and DHCPv6-PD readers
(`ppp/ra_carrier.go`, `ppp/dhcpv6_carrier.go`). Each worker tests its own
exit signal on every read error: `readLoop` checks a `closed` flag under a
mutex, the discovery reader checks for `errSocketClosed` from its own
`readDiscoveryFrame`, and the RA and DHCPv6-PD readers check a canceled
context. It returns when that signal fires.

IPv6 service stop joins DHCPv6 and solicitation readers before the final RA
and carrier close. Concurrent stop calls share that one worker shutdown, then
serialize address and delegated-route withdrawals. A failed withdrawal keeps
its installed marker and lease; its error prevents replacement negotiation.
The service claims the exact attempted link-local pair or delegated route
before asking its handle to add it, because an error can follow a partial
native installation. Startup runs the same rollback after a failed address
add as after a failed carrier acquisition. A failed rollback returns its owner
for the session to retain and retry; a delegated prefix is released only after
its possibly installed route is withdrawn. A handle's withdrawal of an absent
owned receipt performs no foreign native delete and completes idempotently.
Repeated Solicit messages reuse the one prefix that this service already owns;
they cannot allocate a replacement and overwrite a retained route's cleanup key.
The final transport close still releases the handle and joins PPP readers.
<!-- source: internal/component/l2tp/ppp/ipv6_service.go -- Stop, cleanupPrefix, removeLinkLocal -->

Every other read error cannot be classified. It can clear on the next
read, or the socket can stay broken for as long as the daemon runs. A
goroutine that retries an unclassified error at once, forever, spins a
core, and two of the four did so silently, with no log line to find it by.
Each of the four goroutines now logs the error, counts it, and paces the
retry:

- The log line names the socket the error came from (the bound address for
  `readLoop`, the interface for the RA and DHCPv6-PD readers; the discovery
  reader's socket is shared across every configured access interface, so it
  names none).
- A counter rises once per swallowed error: `ze_l2tp_listener_read_errors_total`
  for `readLoop`, `ze_pppoe_discovery_read_errors_total` for the discovery
  reader, and `ze_ppp_reader_errors_total{loop="ra"|"dhcpv6"}` for the RA
  and DHCPv6-PD readers, which share one metric because they live in the
  same package and count the same kind of event.
- The retry is paced through `internal/core/pacer` (`Pacer.Wait`): the
  delay stays at zero for the first failure after a success, doubles on
  each further consecutive failure, and stops growing at a fixed 250ms
  ceiling. A successful read resets the pacer, so the first retry after
  recovery is immediate again.

`Wait` takes the caller's own exit signal: `readLoop` and the discovery
reader each pass a plain `chan struct{}` closed by their own `Stop`, and
the RA and DHCPv6-PD readers pass a context's `Done()` directly, since a
context satisfies `<-chan struct{}` without an adapter. A goroutine pacing
a retry still returns at once when it is told to stop rather than sitting
out the delay.

The discovery reader also observes the stop channel through its socket's
100ms receive timeout. `Stop` closes that channel and joins the reader before
closing the descriptor; closing a raw Linux descriptor alone does not interrupt
an in-flight `recvfrom`. The descriptor remains open until PPP session events
have sent their final PADTs.

The ceiling is a fixed constant in the pacer package, not a YANG leaf. An
operator has no information with which to pick a value, and a wrong choice
reintroduces the spin this design removes.

---

## 12. Package Layout

Both server transports parse `lcp` settings through the shared PPP YANG
grouping. Its defaults are 3000 milliseconds, Max-Terminate 2, Max-Configure 10
and Max-Failure 5. Each parse reads the grouping from the module set
`yang.DefaultLoader` already resolved and builds no loader of its own, so the
defaults cost no second resolve and follow the set the process published. A
start whose config holds no `l2tp` and no `pppoe` block never reads them.
`ppp.NewLCPConfig` validates the immutable snapshot before a
transport places it in `StartSession.LCP`. `Driver.spawnSession` rejects a
missing snapshot and copies an admitted one into the session. L2TP carries it
through `reactorParams`, while PPPoE carries it through each interface-server
generation.
<!-- source: internal/component/l2tp/ppp/lcp_config.go -- NewLCPConfig -->
<!-- source: internal/component/l2tp/ppp/manager.go -- spawnSession -->
<!-- source: internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang -- shared LCP defaults -->
<!-- source: internal/component/l2tp/pppconfig/lcp.go -- loadLCPDefaults -->
<!-- source: internal/component/l2tp/config.go -- ExtractParameters -->
<!-- source: internal/component/l2tp/pppoe/config.go -- ExtractParameters -->

Reload updates the next admission and preserves every running
session's values, including across LCP renegotiation. No Go bootstrap defaults
duplicate the grouping. CHAP retransmission and the proxied PAP wait retain
their separate authentication timing.
<!-- source: internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang -- shared LCP defaults -->
<!-- source: internal/component/l2tp/ppp/lcp_config.go -- NewLCPConfig -->
<!-- source: internal/component/l2tp/ppp/manager.go -- spawnSession -->
<!-- source: internal/component/l2tp/pppoe/subsystem_reload.go -- applyInterfaceConfig -->

The transport parser refuses unknown provider settings instead of silently
using a default, and suggests the closest declared leaf with a complete
transport-specific repair command. It renders invalid provider values as
structured operator errors. Their final line restores the schema default with
a complete `set l2tp lcp ...` or `set pppoe lcp ...` command. An unavailable or invalid
shared schema instead identifies the required build repair. A leaf change
cannot install a missing module.

<!-- source: internal/component/l2tp/ppp/lcp_config.go -- NewLCPConfig -->
<!-- source: internal/component/l2tp/pppconfig/lcp.go -- ParseLCP -->
<!-- source: internal/component/l2tp/ppp/start_session.go -- StartSession.LCP -->

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

6. Original PPP unit owner
   -> captured UnitCreator opens the original /dev/ppp file
   -> ordered RTM_NEWLINK kind ppp, IFLA_PPP_DEV_FD, ECHO and ACK
   -> query44 and capture45 on SAME original PPPoX *os.File, no numeric attach
   -> retain actual SIOCGSKNS socket namespace; create unit there in joined worker
   -> retain original channel/unit files and actual creation identity
   -> unit.SessionConfig(channel), then ppp/kernel.NewSession consumes both

7. After authentication and NCP complete
   -> SessionHandle.SetMTU
   -> SessionHandle.Connect  // PPPIOCCONNECT, then wake the Opened reader
   -> SessionHandle.SetReceiveMRU
   -> SessionHandle.SetAdminUp

Local PPP lease shutdown:
1. Stop IPv6 services while their transport is still available
2. Close the owning handle's PPP channel and unit
3. Join borrowed frame readers and return queued buffers

Transport teardown:
1. Close PPPoL2TP socket
2. L2TP_CMD_SESSION_DELETE
3. L2TP_CMD_TUNNEL_DELETE (only if last session)
4. Close UDP socket (only if tunnel being destroyed)
```

The transport worker transfers both original PPP file owners to the kernel
session constructor on every outcome; it never closes them again after that call.
The kernel handle closes each once. The PPPoL2TP socket has a separate owner,
which permits a realm relay to release local PPP while retaining the transport.
An abort initiated by the peer gates that transport before waiting for PPP
cleanup, rather than permitting a late protocol frame onto an ended session.

Every nonnegative kernel descriptor is owned, including descriptor zero; a
negative value denotes no descriptor. Before publication, a returned error or
a panic releases each distinct supplied descriptor once. Rollback attempts both
resources even when one close fails or panics, and preserves the construction
failure together with cleanup failures. Only successful construction publishes
a frame channel and its owning handle.
Final handle cleanup uses independent panic boundaries for each borrowed stream.
Both are attempted once, and close failures and cleanup panics are returned
together. Concurrent or repeated closes retain that result after native release
is confirmed; an unconfirmed native release seal remains retryable.
Failed final network-resource removal is retained for `show warnings` and the
runtime `doctor-ppp-cleanup` check; closing the handle does not prove removal.

A kernel transport with remote rate operations requires a native release seal.
The transport binds its admitted policy through `NativeReleaseHandle` before
the first remote operation. The policy supplies one full opaque birth tag through
`NativeBirthTag`; the owner writes it to the native interface alias and confirms
it by reading the alias back before binding. It does not reconstruct a prefix
or infer birth from a reused name or index. The successful binding retains that
receipt through `NativeReleaseBirthTag`, including after release, so cleanup
can identify only its own warning without calling the policy again.

That policy must reserve a bounded, strong cleanup owner before submission.
`Close` must wake the borrowed readers while retaining
the native PPP unit if remote cleanup is unconfirmed. Its cleanup owner retries
release after the same policy seals future operations and confirms earlier
cleanup. A returned error or an unreachable Go object does not retain ownership.
<!-- source: internal/component/l2tp/ppp/session_release.go -- NativeReleaseSeal, NativeReleaseHandle -->

Kernel route programming shares that session's exclusive PPP unit lease and
mutex. Before invoking a backend add, the handle retains the destination,
gateway, metric and protocol as a cleanup receipt, including an uncertain add
that returns an error. Withdrawal with no matching receipt performs no native
operation. A failed withdrawal preserves both its error and receipt for retry;
successful withdrawal consumes the receipt. The handle retains at most 1024
route receipts and refuses another distinct route before native mutation.
The wildcard protocol is refused as a producer before a receipt is acquired.
Prefix and gateway aliases share canonical receipts; both backend requests use
the masked network and unmapped IPv4 gateway. Priority must fit uint32 and
protocol must fit uint8, with negative values refused before receipt acquisition.
Peer gateways must be specified, including after IPv4 unmapping.
Because Linux treats priority zero as a deletion wildcard, the owner refuses
concurrent zero and nonzero priorities sharing the same canonical destination,
gateway and protocol, including uncertain cleanup intents. Distinct nonzero
priorities and different match tuples remain permitted. This relies on the
session's exclusive unit and producer namespace; it does not recognize another
writer's state injected into that same namespace.
<!-- source: internal/component/l2tp/ppp/kernel/session.go -- NewSession, Close, AddRoute, RemoveRoute -->
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleKernelSuccess -->

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
