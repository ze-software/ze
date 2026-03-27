# API Commands

**Source:** ExaBGP `reactor/api/command/`, `reactor/api/dispatch/`
**Purpose:** Document all API commands for compatibility

---

## Overview

Ze uses target-first syntax with JSON or text encoding.

### ExaBGP Differences

| Aspect | ExaBGP | Ze |
|--------|--------|-------|
| Syntax styles | v4 (action-first) and v6 (target-first) | Target-first only |
| Encoder | json or text (v4), json only (v6) | json or text |
| Peer selectors | `*`, IP, filters (`[local-as ...]`) | `*`, IP, negated (`!IP`) |
| Multi-session filters | Supported (draft) | Not supported |
| Forward command | Not available | `forward update-id` for route reflection |

See [JSON_FORMAT.md](JSON_FORMAT.md#exabgp-differences) for output format differences.
<!-- source: internal/component/plugin/server/command.go -- Dispatcher -->

---

## Command Verb Taxonomy

Commands follow a **verb-first** convention: `<action> <module> [args...]`.
The action verb determines the command's behavior; the module implements it.

| Verb | Purpose | Examples |
|------|---------|---------|
| `show` | Read-only display (returns data, exits) | `show bgp peer X`, `show bgp warnings` |
| `set` | Create or modify | `set bgp peer X ...` |
| `del` | Remove | `del bgp peer X` |
| `update` | Route operations (announce, withdraw, refresh) | `update bgp peer * prefix ...` |
| `monitor` | Long-running auto-refreshing display | `monitor bgp` (TUI dashboard) |

<!-- source: internal/component/cmd/show/doc.go -- show verb -->
<!-- source: internal/component/cmd/set/doc.go -- set verb -->
<!-- source: internal/component/cmd/del/doc.go -- del verb -->
<!-- source: internal/component/cmd/update/doc.go -- update verb -->

**Internal dispatch:** `<action> <module>` is dispatched as the module implementing the action.
For example, `monitor bgp` is handled by the BGP module's monitor implementation.
Legacy noun-first RPCs (`peer list`, `bgp summary`) remain for internal dispatch but
user-facing commands use verb-first syntax.

**Streaming vs polling:** `monitor` commands keep the display active and auto-refresh.
`monitor event` streams live events line-by-line. `monitor bgp` polls summary data
every 2 seconds and renders a dashboard. Both use the `monitor` verb because they
produce continuously-updating output.

## Command Categories

| Category | Commands |
|----------|----------|
| Daemon | shutdown, reload, restart, status |
| Session | ack, sync, reset, ping, bye |
| System | help, version, api version |
| Peer | list, detail, capabilities, statistics, set (add/save), del, teardown, flush |
| Announce | route, flow, vpls, eor, operational |
| Withdraw | route, flow, vpls, watchdog |
| RIB | routes, best, status, clear |
| Log | levels, set (runtime log levels) |
| Metrics | values, list (Prometheus metrics) |
| Group | start, end (batching) |
| Monitor | monitor bgp (TUI dashboard), monitor event (live event streaming) |
| Subscribe | subscribe, unsubscribe (event filtering) |
<!-- source: internal/component/plugin/server/command.go -- AllBuiltinRPCs -->

---

## Target-First Syntax

### Daemon Commands

```
daemon shutdown          # Graceful shutdown
daemon reload            # Reload configuration
daemon restart           # Restart all peers
daemon status            # Get daemon status
```

### Session Commands

```
plugin session ready     # Signal plugin init complete
plugin session ping      # Health check
plugin session bye       # Disconnect
```
<!-- source: internal/core/ipc/schema/ze-plugin-api.yang -- session RPCs -->

### BGP Plugin Configuration

```
bgp plugin encoding json     # Set event encoding to JSON (default)
bgp plugin encoding text     # Set event encoding to human-readable text
bgp plugin format hex        # Wire bytes as hex string
bgp plugin format base64     # Wire bytes as base64
bgp plugin format parsed     # Decoded fields only (default)
bgp plugin format full       # Both parsed AND wire bytes
bgp plugin ack sync          # Wait for wire transmission
bgp plugin ack async         # Return immediately (default)
```
<!-- source: internal/component/bgp/schema/ze-bgp-api.yang -- plugin-encoding, plugin-format, plugin-ack RPCs -->

### Event Subscription Commands

Plugins subscribe to events via API instead of config. Replaces config-driven `receive {}` blocks.

```
subscribe <namespace> event <type> [direction received|sent|both]
subscribe peer <selector> <namespace> event <type> [direction ...]
subscribe plugin <name> <namespace> event <type> [direction ...]
unsubscribe <namespace> event <type> [direction received|sent|both]
```

**Namespaces:**
- `bgp` - BGP protocol events
- `rib` - RIB events (cache, route changes)

**BGP event types:**

| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | ✅ | UPDATE message |
| `open` | ✅ | OPEN message |
| `notification` | ✅ | NOTIFICATION message |
| `keepalive` | ✅ | KEEPALIVE message |
| `refresh` | ✅ | ROUTE-REFRESH message |
| `state` | ❌ | Peer state change (up/down) |
| `negotiated` | ❌ | Capability negotiation complete |
| `rpki` | ❌ | RPKI validation result (from bgp-rpki plugin) |
| `update-rpki` | ✅ | UPDATE merged with RPKI validation (from bgp-rpki-decorator) |

Plugins may register additional event types via `Registration.EventTypes`. These are validated at runtime against the dynamic registry.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.EventTypes -->

**RIB event types:**

| Event | Description |
|-------|-------------|
| `cache` | Cache entry events |
| `route` | Route change events |

**Examples:**
```
subscribe bgp event update                              # All peers, both directions
subscribe bgp event update direction received           # Received only
subscribe peer upstream1 bgp event update               # Specific peer
subscribe peer * bgp event state                        # All peers, state changes
subscribe peer !upstream1 bgp event update direction sent  # Exclude one peer
subscribe rib event route                               # RIB route events
```

### Monitor Commands

Stream live events or display live dashboards. All monitor commands follow verb-first syntax: `monitor <module> [args...]`.

```
monitor event                                                # All events, all peers
monitor event peer <addr>                                    # Filter by peer address
monitor event peer *                                         # Explicit all peers
monitor event include <type>,<type>                          # Only listed event types
monitor event exclude <type>,<type>                          # All types except listed
monitor event direction received                             # Received events only
monitor event direction sent                                 # Sent events only
monitor event include update peer <addr> direction received  # Combined filters
monitor bgp                                                  # Live peer dashboard (TUI)
```

| Keyword | Values | Default |
|---------|--------|---------|
| `peer` | IP address, peer name, `!exclusion`, or `*` | `*` (all peers) |
| `include` | Comma-separated event types | All types (mutually exclusive with `exclude`) |
| `exclude` | Comma-separated event types | None (mutually exclusive with `include`) |
| `direction` | `received`, `sent` | Both directions |

Keywords may appear in any order. `include` and `exclude` are mutually exclusive.

Event types span all namespaces: BGP (update, open, notification, keepalive, refresh, state, negotiated, eor, congested, resumed, rpki) and RIB (cache, route). Types are validated at parse time.
<!-- source: internal/component/plugin/server/event_monitor.go -- ParseEventMonitorArgs -->

Wire method: `ze-event:monitor`. Supports pipe operators: `| json`, `| table`, `| match`.
<!-- source: internal/component/plugin/server/monitor.go -- MonitorManager -->

**Note:** `monitor bgp` is the live peer dashboard (TUI only). `monitor event` streams live events (SSH exec or TUI).
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

### System Commands

```
system help              # Show help (uses dispatcher, includes plugin commands)
system version software  # Show Ze version
system version api       # Show IPC protocol version
system subsystem list    # List available subsystems
system command list      # List all commands (builtin + plugin)
system command list verbose  # List with source (builtin/process name)
system command help "<name>" # Show command details
system command complete "<partial>"  # Complete command names
system command complete "<cmd>" args [<completed>...] "<partial>"  # Arg completion
```
<!-- source: internal/core/ipc/schema/ze-system-api.yang -- system RPCs -->

### Daemon Commands

```
daemon shutdown          # Gracefully shutdown the daemon
daemon status            # Show daemon status
daemon reload            # Reload the configuration
```

### Peer Commands

```
peer list                # List all peers
peer detail              # Show all peers (detailed)
peer <ip> detail         # Show specific peer detail
peer capabilities        # Show peer capabilities
peer <ip> capabilities   # Show specific peer capabilities
peer statistics          # Show peer statistics (counters)
peer <ip> statistics     # Show specific peer statistics
peer <ip> teardown <code> [<reason>]  # Disconnect peer
set bgp peer <name> with <config>  # Create peer with configuration
del bgp peer <name>                # Remove dynamic peer
peer <sel> flush         # Wait for forward pool to drain (barrier)
```
<!-- source: internal/component/bgp/schema/ze-bgp-api.yang -- peer RPCs -->

### Cache Commands (Ze)

> **Implementation spec:** `plan/learned/148-api-command-restructure-step-8.md`

```
bgp cache <id> forward <sel>    # Forward cached UPDATE to peers
bgp cache <id> retain           # Prevent eviction
bgp cache <id> release          # Allow eviction (reset TTL)
bgp cache <id> expire           # Remove immediately
bgp cache list                  # List cached message IDs

# Batch variants (comma-separated IDs, max 1000):
bgp cache <id1>,<id2>,...,<idN> forward <sel>  # Batch forward
bgp cache <id1>,<id2>,...,<idN> release        # Batch release
```

The cache commands enable route reflection via API:
1. Received UPDATEs are assigned a unique msg-id (per-UPDATE, not per-NLRI)
2. API outputs UPDATE info with msg-id
3. External process decides routing
4. Cache forward command references msg-id (zero-copy when contexts match)
5. Cache entries expire after configurable TTL (default 60s) unless retained
<!-- source: internal/component/bgp/reactor/reactor.go -- cache forward -->

### Log Commands (Ze)

```
bgp log levels                    # Show all subsystem log levels (JSON map)
bgp log set <subsystem> <level>   # Change subsystem log level at runtime
```

Levels: `debug`, `info`, `warn`, `err`. Changes take effect immediately via `slog.LevelVar` atomic swap. Only loggers created via `slogutil.Logger()` or `slogutil.LazyLogger()` (non-disabled) are shown and modifiable.
<!-- source: internal/component/bgp/plugins/cmd/log/schema/ -- ze-bgp-cmd-log-api.yang -->

### Metrics Commands (Ze)

```
bgp metrics values        # Dump Prometheus text format output
bgp metrics list          # List metric names only (no values)
```

Requires telemetry to be enabled in config (`telemetry { prometheus { ... } }`). Returns error if metrics registry is not available.
<!-- source: internal/component/bgp/plugins/cmd/metrics/schema/ -- ze-bgp-cmd-metrics-api.yang -->

### Peer Selectors

```
peer *                   # All peers
peer upstream1            # Specific peer by name
peer !upstream1           # All peers EXCEPT this one (for route reflection)
```
<!-- source: internal/core/selector/selector.go -- Selector -->

The `!<ip>` negated selector is useful for route reflection:
```
# Forward update to all peers except the source
bgp cache 12345 forward !upstream1
```

> **Note:** Filter selectors (`[local-as ...]`, `[peer-as ...]`) from ExaBGP multi-session
> draft are not supported — the draft never became an RFC.

### Route Commands (update text)

All route operations use unified `update text` syntax with flat attribute declarations
(no `set` keyword) and keyword aliases (short forms accepted, see Keyword Aliases below):
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseUpdateText -->

```bash
# Announce routes (flat attributes, no 'set')
peer <selector> update text next <ip> [attributes...] nlri <family> add prefix <prefix>...

# Withdraw routes
peer <selector> update text nlri <family> del prefix <prefix>...

# End-of-RIB marker (RFC 4724)
peer <selector> update text nlri <family> eor

# VPLS (L2VPN/VPLS)
peer <selector> update text nlri l2vpn/vpls add rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>

# EVPN (L2VPN/EVPN)
peer <selector> update text nlri l2vpn/evpn add mac-ip rd <rd> mac <mac> [ip <ip>] label <n>
peer <selector> update text nlri l2vpn/evpn add ip-prefix rd <rd> prefix <prefix> label <n>
peer <selector> update text nlri l2vpn/evpn add multicast rd <rd> ip <ip>
```

### Removed Commands (use update text instead)

The following legacy commands have been removed:

| Old Command | Replacement |
|-------------|-------------|
| `announce ipv4/unicast <p> next-hop <nh>` | `update text next <nh> nlri ipv4/unicast add prefix <p>` |
| `announce ipv6/unicast <p> next-hop <nh>` | `update text next <nh> nlri ipv6/unicast add prefix <p>` |
| `announce eor <afi> <safi>` | `update text nlri <family> eor` |
| `announce vpls ...` | `update text nlri l2vpn/vpls add ...` |
| `announce l2vpn ...` | `update text nlri l2vpn/evpn add ...` |
| `withdraw ipv4/unicast <p>` | `update text nlri ipv4/unicast del prefix <p>` |
| `withdraw ipv6/unicast <p>` | `update text nlri ipv6/unicast del prefix <p>` |
| `withdraw vpls ...` | `update text nlri l2vpn/vpls del ...` |
| `withdraw l2vpn ...` | `update text nlri l2vpn/evpn del ...` |

### Watchdog Commands

```
watchdog announce <name>   # Send all routes in pool to peers
watchdog withdraw <name>   # Withdraw all routes in pool from peers
```

Routes are tagged with a pool when announced:
```bash
update text next 10.0.0.1 nlri ipv4/unicast add prefix 1.0.0.0/24 watchdog set mypool
```

### RIB Commands

```
rib routes [scope] [filters...] [terminal]  # Unified route display with pipeline
    scope: sent | received | sent-received (default)
    filters: path <pattern>, cidr <prefix>, community <value>,
             family <afi/safi>, match <text>
    terminals: count, json
rib best [filters...] [terminal]            # Best-path per prefix (RFC 4271 §9.1.2)
rib status                                  # RIB status (peer/route counts)
rib clear in <selector>                      # Clear Adj-RIB-In (* for all peers)
rib clear out <selector> [family]           # Resend Adj-RIB-Out (* for all, optional family)
```
<!-- source: internal/component/bgp/plugins/rib/schema/ze-rib-api.yang -- RIB RPCs -->

#### Inter-Plugin RIB Commands (GR/LLGR)

These commands are dispatched between plugins (bgp-gr to bgp-rib) and are not intended for direct user invocation:

```
rib retain-routes <peer>                    # Retain routes for peer (GR activation)
rib release-routes <peer>                   # Release retained routes
rib mark-stale <peer> <restart-time> [level]  # Mark routes stale (level: 1=GR, 2=LLGR)
rib purge-stale <peer> [family]             # Purge stale routes (optionally per-family)
rib attach-community <peer> <family> <hex>  # Attach community to stale routes in family
rib delete-with-community <peer> <family> <hex>  # Delete routes carrying community in family
```

### Group Commands (Batching)

```
group start [attributes ...]     # Start batch with shared attributes
peer <selector> update text ...
peer <selector> update text ...
group end                         # End batch, send all
```
<!-- source: internal/component/bgp/transaction/commit_manager.go -- CommitManager -->

---

## Action-First Syntax (Legacy)

### Show Commands

```
show neighbor [summary|extensive|configuration]
show adj-rib in [<afi> <safi>]
show adj-rib out [<afi> <safi>]
```

### Announce/Withdraw

All route operations now use `update text` syntax:

```bash
update text next <ip> [attributes...] nlri <family> add prefix <prefix>
update text nlri <family> del prefix <prefix>
update text nlri <family> eor
```

> **Note:** Legacy `announce`/`withdraw` commands have been removed.
> See "Removed Commands" section above for migration table.

### Control

```
teardown <peer-ip> <code> [<reason>]
shutdown
reload
restart
reset
enable-ack
disable-ack
silence-ack
help
version
```

---

## API Content Configuration (Ze)

### Attribute Filtering

Limit which attributes are parsed for API output:

```
api route-server {
    content {
        encoding json;
        attribute as-path community next-hop;  # Only parse these
    }
    receive { update; }
}
```

Available attribute names:
| Name | Code | Description |
|------|------|-------------|
| `origin` | 1 | ORIGIN |
| `as-path` | 2 | AS_PATH |
| `next-hop` | 3 | NEXT_HOP |
| `med` | 4 | MULTI_EXIT_DISC |
| `local-pref` | 5 | LOCAL_PREF |
| `atomic-aggregate` | 6 | ATOMIC_AGGREGATE |
| `aggregator` | 7 | AGGREGATOR |
| `community` | 8 | COMMUNITIES |
| `originator-id` | 9 | ORIGINATOR_ID |
| `cluster-list` | 10 | CLUSTER_LIST |
| `extended-community` | 16 | EXTENDED_COMMUNITIES |
| `large-community` | 32 | LARGE_COMMUNITIES |
| `all` | - | All attributes (default) |
<!-- source: internal/component/bgp/types/contentconfig.go -- ContentConfig -->

Benefits of partial parsing:
- Reduced CPU (only parse what's needed for routing decision)
- Reduced memory (don't store full parsed attributes)
- Wire bytes preserved for zero-copy forwarding

### NLRI Family Filtering

Limit which address families are included in API output:

```
api route-server {
    content {
        encoding json;
        attribute as-path community next-hop;
        nlri ipv4/unicast;
        nlri ipv6/unicast;
    }
    receive { update; }
}
```

Available families:
| Config Syntax | Canonical Name |
|---------------|----------------|
| `ipv4/unicast` | ipv4/unicast |
| `ipv6/unicast` | ipv6/unicast |
| `ipv4/multicast` | ipv4/multicast |
| `ipv6/multicast` | ipv6/multicast |
| `ipv4 mpls` | ipv4 mpls |
| `ipv6 mpls` | ipv6 mpls |
| `ipv4/mpls-vpn` | ipv4/mpls-vpn |
| `ipv6/mpls-vpn` | ipv6/mpls-vpn |
| `ipv4/flowspec` | ipv4/flowspec |
| `ipv6/flowspec` | ipv6/flowspec |
| `l2vpn/evpn` | l2vpn/evpn |
| `l2vpn/vpls` | l2vpn/vpls |

Special values: `all` (default), `none`

---

## Route Attributes

Attributes are flat keyword-value pairs (no `set` keyword). Both short and long forms accepted.
API text output uses short forms; config output uses long forms.

```
next <ip>                        # Next-hop IP (required) — long: next-hop
origin igp|egp|incomplete        # Origin attribute
path <asn>,<asn>,...             # AS path — long: as-path
pref <int>                       # Local preference — long: local-preference
med <int>                        # Multi-exit discriminator
s-com <comm>,<comm>,...          # Standard communities — long: community
x-com <ext>,<ext>,...            # Extended communities — long: extended-community
l-com <lc>,<lc>,...              # Large communities — long: large-community
originator-id <ip>               # Originator ID
cluster-list <ip>,<ip>,...       # Cluster list
label <label>                    # MPLS label (per-NLRI-section modifier)
rd <rd>                          # Route distinguisher (per-NLRI-section modifier)
info <id>                        # ADD-PATH path ID (per-NLRI-section modifier) — long: path-information
atomic-aggregate                 # Atomic aggregate flag
aggregator <asn> <ip>            # Aggregator
aigp <value>                     # AIGP
split /<len>                     # Ze: prefix expansion (see below)
```
<!-- source: internal/component/bgp/types/types.go -- RouteSpec, PathAttributes -->

### Keyword Aliases

| Long (config) | Short (API) | Also accepts |
|----------------|-------------|--------------|
| `next-hop` | `next` | `nhop` (legacy) |
| `local-preference` | `pref` | — |
| `as-path` | `path` | — |
| `community` | `s-com` | `short-community` |
| `large-community` | `l-com` | — |
| `extended-community` | `x-com` | `e-com` |
| `path-information` | `info` | — |
| `route-distinguisher` | `rd` | — |

Lists use commas (no spaces): `path 65001,65002`. Brackets accepted for transition: `as-path [65001 65002]`.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- keyword alias table -->

---

## Split Keyword (Ze Extension)

The `split` keyword expands a prefix into smaller prefixes. All attributes apply to each generated prefix.

### Syntax

```
split /<target-length>
```

### Example

```
# Announce 2 prefixes with one command
update text next 1.2.3.4 nlri ipv4/unicast add prefix 10.0.0.0/23 split /24
# → 10.0.0.0/24 next-hop 1.2.3.4
# → 10.0.1.0/24 next-hop 1.2.3.4

# With MPLS label - label applies to each prefix
update text next 1.2.3.4 nlri ipv4/nlri-mpls label 100 add prefix 10.0.0.0/22 split /24
# → 10.0.0.0/24 label 100
# → 10.0.1.0/24 label 100
# → 10.0.2.0/24 label 100
# → 10.0.3.0/24 label 100

# With L3VPN - RD and label apply to each prefix
update text next 1.2.3.4 nlri ipv4/mpls-vpn rd 100:1 label 200 add prefix 10.0.0.0/23 split /24
# → 10.0.0.0/24 rd 100:1 label 200
# → 10.0.1.0/24 rd 100:1 label 200
```

### Supported Families

| Family | Split Support | Notes |
|--------|---------------|-------|
| IPv4/IPv6 unicast | ✅ | Standard prefix expansion |
| IPv4/IPv6 nlri-mpls | ✅ | Label copied to each prefix |
| IPv4/IPv6 mpls-vpn | ✅ | RD + label copied to each prefix |
| FlowSpec | ❌ | N/A - uses match rules, not prefixes |
| VPLS/EVPN | ❌ | Different NLRI structure |

### Constraints

- Target length must be longer than source prefix (e.g., /23 → /24, not /24 → /23)
- Maximum expansion: implementation-dependent (avoid /8 → /32)

---

## FlowSpec Commands

```
announce ipv4/flow \
  destination 10.0.0.0/8 \
  destination-port =80 \
  protocol =tcp \
  then discard

withdraw ipv4/flow \
  destination 10.0.0.0/8 \
  destination-port =80
```

### Match Components

| Keyword | Description |
|---------|-------------|
| destination | Destination prefix |
| source | Source prefix |
| destination-port | Destination port |
| source-port | Source port |
| port | Any port |
| protocol | IP protocol |
| next-header | IPv6 next header |
| tcp-flags | TCP flags |
| icmp-type | ICMP type |
| icmp-code | ICMP code |
| fragment | Fragment flags |
| dscp | DSCP value |
| packet-length | Packet length |
| flow-label | IPv6 flow label |

### Actions (then)

| Keyword | Description |
|---------|-------------|
| accept | Accept traffic |
| discard | Drop traffic |
| rate-limit <bps> | Rate limit |
| redirect <rt> | Redirect to VRF |
| redirect-next-hop | Redirect to next-hop |
| mark <dscp> | Set DSCP |
| community [...] | Add community |
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- FlowSpec parsing -->

---

## Filter Callbacks (planned)

The engine sends `filter-update` callbacks to external plugin filters during
UPDATE processing. This is a callback RPC (engine to plugin), not a user command.

| Field | Type | Description |
|-------|------|-------------|
| `filter` | string | Filter name (declared at stage 1, dispatches to the right handler) |
| `direction` | string | `import` or `export` |
| `peer` | string | Peer IP address |
| `peer-as` | uint32 | Peer ASN |
| `update` | string | Text-format attributes and NLRI (only declared attributes) |

Response: `{"action":"accept"}`, `{"action":"reject"}`, or
`{"action":"modify","update":"<delta>"}` with only changed fields.

<!-- source: plan/spec-redistribution-filter.md -- filter-update RPC design -->

---

## Response Format

### Success (with serial prefix)

```json
{"type":"response","response":{"serial":"1","status":"done"}}
```

### Error

```json
{"type":"response","response":{"serial":"1","status":"error","data":"description"}}
```
<!-- source: internal/component/plugin/types.go -- Response struct -->

### Show Neighbor

```json
{
  "neighbor": {
    "address": "192.168.1.2",
    "local-address": "192.168.1.1",
    "local-as": 65001,
    "peer-as": 65002,
    "router-id": "1.1.1.1",
    "state": "established"
  }
}
```

### Show Adj-RIB

```json
{
  "routes": [
    {
      "nlri": "10.0.0.0/8",
      "next-hop": "192.168.1.2",
      "origin": "igp",
      "as-path": [65002]
    }
  ]
}
```

---

## Command Dispatch

### Command Tree Structure

```
daemon
├── shutdown
├── reload
├── restart
└── status

plugin
└── session
    ├── ready
    ├── ping
    └── bye

bgp
├── help
├── command
│   ├── list
│   ├── help
│   └── complete
├── event
│   └── list
├── log
│   ├── levels            # Show subsystem log levels
│   └── set               # Set subsystem log level at runtime
├── metrics
│   ├── values            # Show Prometheus metrics (text format)
│   └── list              # List metric names
└── plugin
    ├── encoding
    ├── format
    └── ack

peer
├── list
├── detail
├── capabilities
├── statistics
└── <selector>
    ├── detail
    ├── capabilities
    ├── statistics
    ├── teardown
    ├── announce
    ├── withdraw
    └── group

rib
├── routes [sent|received|sent-received] [filters...] [count|json]
├── best [filters...] [count|json]
├── status
└── clear [in|out]

group
├── start
└── end

monitor
├── bgp                   # Live peer dashboard (TUI)
└── event                 # Stream live events (keeps session open)
```

---

## Ze Implementation Notes

### Command Dispatcher

```go
type Handler func(ctx *Context, peers []string, remaining string) error

type DispatchTree map[string]interface{}  // Handler or nested DispatchTree

func Dispatch(tree DispatchTree, tokens *Tokenizer, reactor *Reactor) (Handler, []string) {
    // Walk tree consuming tokens
    // Return handler and matched peers
}
```
<!-- source: internal/component/plugin/server/command.go -- Dispatcher, Handler -->

### Peer Selector Parsing

```go
type Selector struct {
    All       bool
    IP        netip.Addr
    Filters   map[string]string  // local-as, peer-as, local-ip, id, family
}

func ParseSelector(s string) (*Selector, error) {
    if s == "*" {
        return &Selector{All: true}, nil
    }
    if strings.HasPrefix(s, "[") {
        return parseFilteredSelector(s)
    }
    ip, err := netip.ParseAddr(s)
    return &Selector{IP: ip}, err
}
```

### Command Registry

```go
var Commands = []CommandInfo{
    {"daemon shutdown", false, nil},
    {"peer * update text", true, []string{"next", "origin", ...}},
    // ...
}
```
<!-- source: internal/component/plugin/server/rpc_register.go -- registeredRPCs -->

---

## Managed Config RPCs

RPCs for hub-client managed configuration. These operate over MuxConn after auth,
separate from the plugin 5-stage protocol.

| Verb | Direction | Payload | Response |
|------|-----------|---------|----------|
| `config-fetch` | Client to hub | `{"version":"<hash-or-empty>"}` | `{"version":"<hash>","config":"<base64>"}` or `{"status":"current"}` |
| `config-changed` | Hub to client | `{"version":"<hash>"}` | `{}` |
| `config-ack` | Client to hub | `{"version":"<hash>","ok":true}` or `{"version":"<hash>","ok":false,"error":"..."}` | `{}` |
| `ping` | Either direction | `{}` | `{}` |

Version hash is truncated SHA-256 (16 hex characters) of config bytes.

<!-- source: pkg/fleet/envelope.go -- RPC payload types -->
<!-- source: internal/component/plugin/server/managed.go -- hub-side handlers -->
<!-- source: internal/component/managed/client.go -- client-side handlers -->

---

**Last Updated:** 2026-03-27
