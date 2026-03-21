# Spec: iface-bus — Interface Management on the Bus

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-arch-0 |
| Phase | - |
| Updated | 2026-03-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-arch-0-system-boundaries.md` — umbrella arch spec (Bus, Subsystem, Plugin)
4. `pkg/ze/bus.go` — Bus interface (Publish, Subscribe, Consumer)
5. `pkg/ze/subsystem.go` — Subsystem interface
6. `internal/component/engine/engine.go` — Engine (starts subsystems in order)

## Task

Add interface lifecycle management to Ze via the Bus. An **interface plugin** (one per OS) monitors and manages OS network interfaces, publishing events to hierarchical Bus topics. BGP and other subsystems subscribe to these events to react to address availability changes.

The primary use case is **make-before-break interface migration**: create a new interface, add an IP, wait for BGP to bind, remove the IP from the old interface, then remove the old interface — ensuring the IP is always reachable.

### Design Decision: Plugin per OS

Interface management is implemented as a **plugin** (not a subsystem), with one plugin per operating system:

| OS | Plugin | Mechanism |
|----|--------|-----------|
| Linux | `iface-linux` | Netlink (`vishvananda/netlink`) — `RTM_NEWLINK`, `RTM_NEWADDR`, multicast monitoring |
| macOS | `iface-darwin` | Route sockets (`syscall.AF_ROUTE`) |
| BSD | `iface-bsd` (future) | Route sockets (similar to macOS) |

Go build tags (`//go:build linux`, `//go:build darwin`) select the correct `register.go` at compile time. Only the platform-appropriate plugin is compiled and registered.

### Scope

The plugin both **monitors all OS interfaces** (reacting to external changes) and **manages Ze-created interfaces** (creating/deleting on command). BGP reacts to any IP appearing or disappearing, regardless of who created it.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-arch-0-system-boundaries.md` — Bus, Subsystem, Plugin boundaries
  → Decision: Bus is content-agnostic, payload always `[]byte`, topics hierarchical with `/`
  → Decision: Plugins extend subsystem behavior by reacting to bus events
  → Constraint: Plugin infrastructure MUST NOT import plugin implementations — use registry
- [ ] `docs/architecture/core-design.md` — current engine + plugin architecture
  → Constraint: Bus never type-asserts payloads
- [ ] `.claude/rules/plugin-design.md` — plugin registration, 5-stage protocol, import rules
  → Constraint: registration via `init()` in `register.go`, auto-discovered through registry

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` — BGP-4: TCP connection binding, session establishment
  → Constraint: BGP binds to specific local addresses per peer (Section 8)
- [ ] `rfc/short/rfc4724.md` — Graceful Restart: session preservation across restarts
  → Constraint: GR allows session survival during interface migration if forwarding state preserved

**Key insights:**
- Bus topics are hierarchical strings; prefix subscriptions match all subtopics
- Plugins register via `init()` + `register.go`, discovered through registry
- BGP already has per-peer `LocalAddress` binding — the interface plugin provides the "when is this address available?" signal
- Make-before-break requires ordering guarantees: new IP confirmed usable before old IP removed

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/ze/bus.go` — Bus interface: `CreateTopic`, `Publish`, `Subscribe`, `Unsubscribe`. Event has Topic (string), Payload (`[]byte`), Metadata (`map[string]string`)
- [ ] `pkg/ze/subsystem.go` — Subsystem interface: `Name`, `Start(ctx, Bus, ConfigProvider)`, `Stop`, `Reload`
- [ ] `internal/component/engine/engine.go` — Engine starts plugins first, then subsystems in registration order. Stops in reverse
- [ ] `internal/component/bus/bus.go` — Bus implementation with hierarchical topics, prefix matching, per-consumer delivery goroutine
- [ ] `internal/component/bgp/reactor/listener.go` — `Listener` wraps `net.ListenConfig`, bound to `"addr:port"` strings
- [ ] `internal/component/bgp/reactor/reactor_peers.go` — `startListenerForAddressPort(addr, port, peerKey)` creates per-address listeners
- [ ] `internal/core/network/network.go` — `RealDialer` with optional `LocalAddr` for outbound connections
- [ ] `internal/component/plugin/registry/` — plugin registration via `init()`, `Register()` function

**Behavior to preserve:**
- BGP per-peer `LocalAddress` binding via `net.ListenConfig`
- Bus content-agnostic — payload is `[]byte`, bus never type-asserts
- Plugin registration pattern via `init()` + `register.go`
- Engine startup order: plugins first, then subsystems

**Behavior to change:**
- BGP currently assumes configured IPs exist — no verification or reactive binding
- No interface lifecycle events exist on the Bus today
- No OS interface management capability exists

## Data Flow (MANDATORY)

### Entry Points

Three entry points for interface state:

| Source | Entry | Format |
|--------|-------|--------|
| OS kernel | Netlink multicast (Linux) / route socket (macOS) | Kernel netlink messages |
| Config | YANG `ze-iface-conf` | Config tree → `map[string]any` |
| CLI | `ze interface create/delete/migrate` | Command arguments |

### Transformation Path

1. **Kernel event** — netlink multicast delivers `RTM_NEWLINK`, `RTM_DELLINK`, `RTM_NEWADDR`, `RTM_DELADDR` to the interface plugin's monitor goroutine
2. **Event classification** — plugin maps netlink message type to Bus topic (`interface/created`, `interface/addr/added`, etc.)
3. **Payload encoding** — plugin serializes interface state as JSON `[]byte` (kebab-case keys per `rules/json-format.md`)
4. **Bus publish** — `bus.Publish("interface/addr/added", payload, metadata)` with metadata for filtering (e.g., `"address" → "10.0.0.1"`)
5. **BGP subscription** — BGP subsystem's `interface/` consumer receives event, checks if any peer's `LocalAddress` matches
6. **BGP reaction** — on `addr/added`: start listener, initiate connections. On `addr/removed`: gracefully drain sessions on that address

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| OS ↔ Interface Plugin | Netlink socket (Linux) / route socket (macOS) | [ ] |
| Interface Plugin ↔ Bus | `bus.Publish(topic, []byte, metadata)` | [ ] |
| Bus ↔ BGP Subsystem | `consumer.Deliver([]Event)` | [ ] |
| BGP ↔ Peers | `net.ListenConfig` / `net.Dialer` binding to address | [ ] |

### Integration Points
- `internal/component/plugin/registry/` — interface plugin registers here
- `pkg/ze/bus.go` — `Bus.Publish`, `Bus.Subscribe`
- `internal/component/bgp/reactor/listener.go` — BGP starts/stops listeners in response to events
- `internal/component/bgp/reactor/reactor_peers.go` — peer connection management reacts to address availability

### Architectural Verification
- [ ] No bypassed layers (interface plugin → Bus → BGP, never direct)
- [ ] No unintended coupling (BGP never imports interface plugin)
- [ ] No duplicated functionality (extends existing Bus, not new IPC)
- [ ] Zero-copy preserved where applicable (Bus payload is `[]byte`, no intermediate structs in bus layer)

## Bus Topics

### Topic Hierarchy

| Topic | Published When | Payload Fields |
|-------|---------------|----------------|
| `interface/created` | Interface appeared (created or first seen) | `name`, `type`, `index`, `mtu` |
| `interface/deleted` | Interface removed | `name`, `index` |
| `interface/up` | Link state transitioned to up | `name`, `index` |
| `interface/down` | Link state transitioned to down | `name`, `index` |
| `interface/addr/added` | IP address assigned and confirmed (DAD complete for IPv6) | `name`, `index`, `address`, `prefix-length`, `family` |
| `interface/addr/removed` | IP address removed | `name`, `index`, `address`, `prefix-length`, `family` |

### Metadata for Filtering

| Key | Value | Purpose |
|-----|-------|---------|
| `name` | Interface name (e.g., `"eth0"`) | Filter by interface |
| `address` | IP address string (e.g., `"10.0.0.1"`) | BGP matches against peer `LocalAddress` |
| `family` | `"ipv4"` or `"ipv6"` | Address family filter |

### Payload Format (JSON, kebab-case)

All payloads follow `rules/json-format.md`. Example for `interface/addr/added`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Interface name |
| `index` | integer | OS interface index |
| `address` | string | IP address (no prefix) |
| `prefix-length` | integer | CIDR prefix length |
| `family` | string | `"ipv4"` or `"ipv6"` |
| `managed` | boolean | `true` if Ze created this interface |

## Interface Migration Protocol

### Make-Before-Break Sequence

The migration ensures the IP is always reachable. Five phases, strict ordering:

| Phase | Action | Trigger | Bus Event Published | BGP Expected Reaction |
|-------|--------|---------|--------------------|-----------------------|
| 1 | Create new interface | Config reload or CLI `ze interface create` | `interface/created` then `interface/up` | None — no IP yet |
| 2 | Add IP to new interface | Config or CLI `ze interface addr add` | `interface/addr/added` | Start new listener on address, begin peer connections |
| 3 | Confirm BGP ready | BGP publishes readiness | `bgp/listener/ready` (new topic) | N/A — BGP is the publisher |
| 4 | Remove IP from old interface | Orchestrator proceeds after Phase 3 | `interface/addr/removed` | Gracefully drain sessions on old listener, peers reconnect to new |
| 5 | Remove old interface | After drain completes | `interface/deleted` | No impact — already drained |

### Ordering Guarantee

Phase 4 MUST NOT start until Phase 3 confirms BGP has established sessions on the new address. The orchestrator (config reload handler or CLI migration command) waits for the `bgp/listener/ready` event before proceeding.

### Failure Handling

| Failure | Recovery |
|---------|----------|
| New interface creation fails | Abort migration, old interface unchanged |
| IP add fails (address in use) | Abort migration, clean up new interface |
| BGP fails to bind new address | Timeout → abort migration, remove IP from new interface, old interface unchanged |
| Old IP removal fails | Log warning, continue (new IP already working) |
| Old interface removal fails | Log warning (stale interface, no IP) |

## OS-Level Operations

### Linux (Netlink)

| Operation | Netlink Message | Key Attributes |
|-----------|-----------------|----------------|
| Create interface | `RTM_NEWLINK` + `NLM_F_CREATE` | `IFLA_IFNAME`, `IFLA_LINKINFO` (`IFLA_INFO_KIND` = `"dummy"` / `"veth"` / `"bridge"`) |
| Set interface up | `RTM_NEWLINK` | `ifi_change = IFF_UP`, `ifi_flags = IFF_UP` |
| Set MTU | `RTM_NEWLINK` | `IFLA_MTU` |
| Set MAC | `RTM_NEWLINK` | `IFLA_ADDRESS` |
| Add IPv4 address | `RTM_NEWADDR` + `NLM_F_CREATE` | `IFA_LOCAL` + `IFA_ADDRESS` (both required for IPv4) |
| Add IPv6 address | `RTM_NEWADDR` + `NLM_F_CREATE` | `IFA_ADDRESS` only |
| Remove address | `RTM_DELADDR` | `IFA_LOCAL` + `IFA_ADDRESS` (IPv4) or `IFA_ADDRESS` (IPv6) |
| Delete interface | `RTM_DELLINK` | `IFLA_IFNAME` |
| Monitor changes | Multicast groups | `RTMGRP_LINK`, `RTMGRP_IPV4_IFADDR`, `RTMGRP_IPV6_IFADDR` |

Dependency: `github.com/vishvananda/netlink` (high-level Go netlink library)

### macOS (Route Sockets)

| Operation | Mechanism | Notes |
|-----------|-----------|-------|
| Create interface | Not directly supported for most types | Use `ifconfig` / `networksetup` for loopback aliases |
| Add IP address | `SIOCAIFADDR` ioctl (IPv4), `SIOCAIFADDR_IN6` (IPv6) | Or `ifconfig <iface> alias <addr>` |
| Remove IP address | `SIOCDIFADDR` ioctl | Or `ifconfig <iface> -alias <addr>` |
| Monitor changes | `AF_ROUTE` socket | `RTM_IFINFO` (link), `RTM_NEWADDR`/`RTM_DELADDR` (address) |

macOS plugin has reduced creation capability but full monitoring capability.

## Plugin Registration

### Structure

| File | Build Tag | Purpose |
|------|-----------|---------|
| `internal/plugins/iface-linux/iface.go` | `//go:build linux` | Linux implementation using `vishvananda/netlink` |
| `internal/plugins/iface-linux/register.go` | `//go:build linux` | `init()` → `registry.Register(...)` |
| `internal/plugins/iface-linux/monitor.go` | `//go:build linux` | Netlink multicast monitor goroutine |
| `internal/plugins/iface-darwin/iface.go` | `//go:build darwin` | macOS implementation using route sockets |
| `internal/plugins/iface-darwin/register.go` | `//go:build darwin` | `init()` → `registry.Register(...)` |
| `internal/plugins/iface-darwin/monitor.go` | `//go:build darwin` | Route socket monitor goroutine |

### Registration Fields

| Field | Value |
|-------|-------|
| `Name` | `"iface"` |
| `Description` | `"OS interface lifecycle management"` |
| `Features` | `"interface monitor"` |
| `ConfigRoots` | `["interface"]` |

### Plugin Lifecycle

The interface plugin follows the standard 5-stage protocol:

| Stage | Action |
|-------|--------|
| 1. Declaration | Registers as `"iface"`, declares `"interface"` feature |
| 2. Config | Receives interface configuration (managed interfaces to create) |
| 3. Capabilities | Declares monitoring + management capabilities |
| 4. Registry | Receives registry (unused — no NLRI/capability decode needed) |
| 5. Ready | Opens netlink/route socket, starts monitor goroutine, creates configured interfaces, begins publishing to Bus |

## BGP Subsystem Reactions

### Subscription

BGP subscribes to `interface/` prefix on the Bus during its `Start()`:

| Event | BGP Action |
|-------|------------|
| `interface/addr/added` | Check if address matches any peer's `LocalAddress`. If yes: start listener on that address, attempt outbound connections for peers configured with that address |
| `interface/addr/removed` | Check if address matches any active listener. If yes: stop accepting new connections, gracefully close existing sessions (send NOTIFICATION cease with "other configuration change" subcode), remove listener |
| `interface/down` | Check if any active peer sessions use interfaces that went down. Mark those peers for reconnection when interface comes back |
| `interface/up` | Check if any pending peers have addresses on this interface. Resume connection attempts |

### Graceful Drain on Address Removal

When `interface/addr/removed` fires for an address with active BGP sessions:

| Step | Action | Duration |
|------|--------|----------|
| 1 | Stop accepting new connections on that address | Immediate |
| 2 | Send NOTIFICATION (cease, subcode 6 "other configuration change") to all peers on that address | Immediate |
| 3 | Wait for peers to disconnect or hold timer to expire | Up to hold time (default 90s) |
| 4 | Force-close remaining connections | After timeout |
| 5 | Remove listener | After all connections closed |

## YANG Configuration

New YANG module: `ze-iface-conf.yang`

### Design Reference: VyOS

The YANG schema follows VyOS's interface configuration hierarchy: **type-first grouping** with common options inherited across types. VyOS uses `set interfaces <type> <name>` where each type is a separate container with shared fragments for `address`, `mtu`, `description`, `disable`, and `vrf`. Ze adapts this pattern to YANG `grouping`/`uses`.

VyOS source reference: `~/Code/github.com/vyos/vyos-1x/interface-definitions/`

### Hierarchy (VyOS-aligned)

The top-level `interface` container groups interfaces by type (like VyOS's `set interfaces <type>`), with each type as a YANG `list` keyed by name.

**Naming convention:** interface names are prefixed by type. The prefix identifies the interface type unambiguously:

| Type | Prefix | Example Names |
|------|--------|---------------|
| Ethernet | `eth` | `eth0`, `eth1` |
| Dummy | `dum` | `dum0`, `dum1` |
| Veth | `veth` | `veth0`, `veth1` |
| Bridge | `br` | `br0`, `br1` |
| Loopback | `lo` | `lo` (singleton) |
| VLAN | `<parent>.<id>` | `eth0.100`, `eth1.200` |

Linux also uses kernel-assigned names (`ens3`, `enp0s3`, `eno1`) for ethernet — these are accepted as-is under the `ethernet` type.

| YANG Path | Node Type | VyOS Equivalent | Description |
|-----------|-----------|-----------------|-------------|
| `interface` | container | `interfaces` | Top-level interface container |
| `interface/ethernet` | list (key: `name`) | `interfaces ethernet <name>` | Physical ethernet interfaces (configure only, not created) |
| `interface/ethernet/name` | leaf string | tagNode key | Interface name (e.g., `eth0`, `ens3`) |
| `interface/ethernet/address` | leaf-list string | `address` multi leafNode | IP addresses in CIDR notation |
| `interface/ethernet/description` | leaf string | `description` leafNode | Human-readable description (max 255) |
| `interface/ethernet/mtu` | leaf uint16 | `mtu` leafNode | MTU (68-16000, default 1500) |
| `interface/ethernet/disable` | leaf empty | `disable` valueless | Administrative shutdown |
| `interface/ethernet/vrf` | leaf string | `vrf` leafNode | VRF membership |
| `interface/ethernet/vlan` | list (key: `id`) | `interfaces ethernet <name> vif <id>` | 802.1Q VLAN sub-interfaces |
| `interface/ethernet/vlan/id` | leaf uint16 | tagNode key | VLAN ID (1-4094) |
| `interface/ethernet/vlan/address` | leaf-list string | `address` multi leafNode | IP addresses in CIDR notation |
| `interface/ethernet/vlan/description` | leaf string | `description` leafNode | Human-readable description |
| `interface/ethernet/vlan/mtu` | leaf uint16 | `mtu` leafNode | MTU (68-16000, default parent MTU) |
| `interface/ethernet/vlan/vrf` | leaf string | `vrf` leafNode | VRF membership |
| `interface/dummy` | list (key: `name`) | `interfaces dummy <name>` | Dummy/loopback-like interfaces |
| `interface/dummy/name` | leaf string | tagNode key | Interface name (e.g., `dum0`) |
| `interface/dummy/address` | leaf-list string | `address` multi leafNode | IP addresses in CIDR notation |
| `interface/dummy/description` | leaf string | `description` leafNode | Human-readable description (max 255) |
| `interface/dummy/mtu` | leaf uint16 | `mtu` leafNode | MTU (68-16000, default 1500) |
| `interface/dummy/disable` | leaf empty | `disable` valueless | Administrative shutdown |
| `interface/dummy/vrf` | leaf string | `vrf` leafNode | VRF membership |
| `interface/veth` | list (key: `name`) | `interfaces virtual-ethernet <name>` | Virtual ethernet pairs |
| `interface/veth/name` | leaf string | tagNode key | Interface name (e.g., `veth0`) |
| `interface/veth/peer-name` | leaf string | `peer-name` leafNode | Name of the other end |
| `interface/veth/address` | leaf-list string | `address` multi leafNode | IP addresses in CIDR notation |
| `interface/veth/description` | leaf string | `description` leafNode | Human-readable description |
| `interface/veth/mtu` | leaf uint16 | `mtu` leafNode | MTU (68-16000, default 1500) |
| `interface/veth/disable` | leaf empty | `disable` valueless | Administrative shutdown |
| `interface/veth/vrf` | leaf string | `vrf` leafNode | VRF membership |
| `interface/bridge` | list (key: `name`) | `interfaces bridge <name>` | Bridge interfaces |
| `interface/bridge/name` | leaf string | tagNode key | Interface name (e.g., `br0`) |
| `interface/bridge/member` | leaf-list string | `member interface` tagNode | Member interface names |
| `interface/bridge/address` | leaf-list string | `address` multi leafNode | IP addresses in CIDR notation |
| `interface/bridge/description` | leaf string | `description` leafNode | Human-readable description |
| `interface/bridge/mtu` | leaf uint16 | `mtu` leafNode | MTU (68-16000, default 1500) |
| `interface/bridge/disable` | leaf empty | `disable` valueless | Administrative shutdown |
| `interface/bridge/stp` | leaf empty | `stp` valueless | Enable Spanning Tree |
| `interface/bridge/vrf` | leaf string | `vrf` leafNode | VRF membership |
| `interface/loopback` | container | `interfaces loopback lo` | Loopback (singleton, no key) |
| `interface/loopback/address` | leaf-list string | `address` multi leafNode | IP addresses in CIDR notation |
| `interface/loopback/description` | leaf string | `description` leafNode | Human-readable description |

### Shared Grouping: `interface-common`

Following VyOS's fragment pattern (`include/interface/*.xml.i`), common options are defined as a YANG `grouping` reused across all interface types:

| Field | Type | Default | Constraint | VyOS Fragment |
|-------|------|---------|------------|---------------|
| `address` | leaf-list string | — | `ipv4net` or `ipv6net` CIDR format | `address-ipv4-ipv6.xml.i` |
| `description` | leaf string | — | max 255 chars | `generic-description.xml.i` |
| `mtu` | leaf uint16 | 1500 | 68-16000 | `mtu-68-16000.xml.i` |
| `disable` | leaf empty | — | present = disabled | `disable.xml.i` |
| `vrf` | leaf string | — | must reference existing VRF | `vrf.xml.i` |

### Monitor Configuration

| YANG Path | Node Type | Description |
|-----------|-----------|-------------|
| `interface/monitor` | container | OS interface monitoring settings |
| `interface/monitor/enabled` | leaf boolean | Enable monitoring (default: true) |
| `interface/monitor/filter` | leaf-list string | Interface name patterns to monitor (empty = all) |

### BGP Integration: `update-source` Enhancement

Following VyOS, BGP's `local-address` (Ze's equivalent of `update-source`) should accept both IP addresses and interface names:

| Current Ze | VyOS | Proposed Ze |
|------------|------|-------------|
| `local-address` accepts IP string or `"auto"` | `update-source` accepts IP or interface name | `local-address` accepts IP, interface name, or `"auto"` |

When `local-address` is an interface name, BGP resolves it to the interface's primary IP address and re-resolves on `interface/addr/added` and `interface/addr/removed` events. This is the key integration point — BGP peers reference interfaces by name, and the interface plugin tells BGP when those interfaces gain or lose addresses.

## CLI Design (VyOS-aligned)

### Configuration Commands

Ze's config file maps to a VyOS-like CLI syntax. These are the config stanzas (not runtime commands):

| Config Path | Description | VyOS Equivalent |
|-------------|-------------|-----------------|
| `interface ethernet eth0 { address 10.0.0.1/24; }` | Configure physical interface | `set interfaces ethernet eth0 address 10.0.0.1/24` |
| `interface ethernet eth0 { mtu 9000; }` | Set MTU (jumbo frames) | `set interfaces ethernet eth0 mtu 9000` |
| `interface ethernet eth0 { vlan 100 { address 10.0.100.1/24; } }` | VLAN sub-interface | `set interfaces ethernet eth0 vif 100 address 10.0.100.1/24` |
| `interface dummy dum0 { }` | Create dummy interface | `set interfaces dummy dum0` |
| `interface dummy dum0 { address 10.0.0.1/32; }` | Assign address | `set interfaces dummy dum0 address 10.0.0.1/32` |
| `interface dummy dum0 { mtu 9000; }` | Set MTU | `set interfaces dummy dum0 mtu 9000` |
| `interface dummy dum0 { description "loopback"; }` | Description | `set interfaces dummy dum0 description 'loopback'` |
| `interface dummy dum0 { disable; }` | Admin disable | `set interfaces dummy dum0 disable` |
| `interface veth veth0 { peer-name veth1; }` | Create veth pair | `set interfaces virtual-ethernet veth0 peer-name veth1` |
| `interface bridge br0 { member eth0; member eth1; }` | Bridge members | `set interfaces bridge br0 member interface eth0` |

### Runtime CLI Commands

New subcommand: `ze interface`

| Command | Description | VyOS Equivalent |
|---------|-------------|-----------------|
| `ze interface show` | List all interfaces with state + addresses | `show interfaces` |
| `ze interface show <name>` | Detail for one interface | `show interfaces ethernet eth0` |
| `ze interface show --json` | JSON output | — |
| `ze interface create dummy <name>` | Create dummy interface | (config mode) |
| `ze interface create veth <name> <peer>` | Create veth pair | (config mode) |
| `ze interface delete <name>` | Delete Ze-managed interface | `delete interfaces dummy dum0` |
| `ze interface addr add <name> <addr/prefix>` | Add IP address | `set interfaces dummy dum0 address ...` |
| `ze interface addr del <name> <addr/prefix>` | Remove IP address | `delete interfaces dummy dum0 address ...` |
| `ze interface migrate <addr/prefix> <from-iface> <to-iface>` | Make-before-break IP migration | No VyOS equivalent |

### JSON Output Format

`ze interface show --json` follows `rules/json-format.md` (kebab-case):

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Interface name |
| `type` | string | `"dummy"`, `"veth"`, `"bridge"`, `"ethernet"`, etc. |
| `index` | integer | OS interface index |
| `state` | string | `"up"` or `"down"` |
| `mtu` | integer | Current MTU |
| `mac` | string | MAC address |
| `managed` | boolean | `true` if Ze created this interface |
| `addresses` | array of objects | Each: `{"address": "10.0.0.1", "prefix-length": 32, "family": "ipv4"}` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `interface` section | → | Interface plugin creates OS interface | `TestIfacePluginCreatesInterface` |
| Netlink event (address added externally) | → | Monitor detects, publishes to Bus | `TestIfaceMonitorPublishesAddrAdded` |
| Bus event `interface/addr/added` | → | BGP starts listener on that address | `TestBGPStartsListenerOnAddrAdded` |
| Bus event `interface/addr/removed` | → | BGP drains sessions on that address | `TestBGPDrainsOnAddrRemoved` |
| Config reload with interface migration | → | Full make-before-break sequence | `TestMakeBeforeBreakMigration` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Interface plugin starts on Linux | Opens netlink socket, subscribes to multicast groups `RTMGRP_LINK` + `RTMGRP_IPV4_IFADDR` + `RTMGRP_IPV6_IFADDR`, begins monitoring |
| AC-2 | External IP added to OS interface | Plugin publishes `interface/addr/added` to Bus within 1 second |
| AC-3 | Config specifies managed interface | Plugin creates interface via netlink `RTM_NEWLINK`, brings it up, assigns configured addresses |
| AC-4 | `interface/addr/added` event for a peer's `LocalAddress` | BGP starts listener on that address and attempts outbound connections |
| AC-5 | `interface/addr/removed` event for an active listener address | BGP sends NOTIFICATION cease to peers, drains connections, removes listener |
| AC-6 | Make-before-break migration via config reload | New interface created, IP added, BGP binds, old IP removed, old interface deleted — no period where IP is unreachable |
| AC-7 | Interface plugin starts on macOS | Opens `AF_ROUTE` socket, monitors address changes, publishes same Bus events |
| AC-8 | Interface plugin stops | Removes Ze-managed interfaces (if configured to do so), closes netlink/route socket |
| AC-9 | Multiple peers share same `LocalAddress` | All peers react to address add/remove events, shared listener created once |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBusTopicCreation` | `internal/plugins/iface-linux/iface_test.go` | Plugin creates correct Bus topics on start | |
| `TestNetlinkEventToTopic` | `internal/plugins/iface-linux/monitor_test.go` | Maps netlink message types to correct Bus topics | |
| `TestPayloadFormat` | `internal/plugins/iface-linux/iface_test.go` | JSON payload matches spec (kebab-case, correct fields) | |
| `TestBGPAddrAddedReaction` | `internal/component/bgp/reactor/reactor_iface_test.go` | Listener started when matching addr event received | |
| `TestBGPAddrRemovedReaction` | `internal/component/bgp/reactor/reactor_iface_test.go` | Sessions drained when addr removed event received | |
| `TestMigrationOrdering` | `internal/component/bgp/reactor/reactor_iface_test.go` | Old IP not removed until new IP confirmed | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MTU | 68-65535 | 65535 | 67 | 65536 |
| Prefix length IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix length IPv6 | 0-128 | 128 | N/A | 129 |
| VLAN ID | 1-4094 | 4094 | 0 | 4095 |
| Interface name | 1-15 chars (Linux IFNAMSIZ-1) | 15 chars | empty | 16 chars |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-create` | `test/plugin/iface-create.ci` | Config with interface section creates dummy interface | |
| `test-iface-monitor` | `test/plugin/iface-monitor.ci` | External IP change triggers Bus event | |
| `test-iface-bgp-bind` | `test/plugin/iface-bgp-bind.ci` | BGP session starts after interface IP added | |
| `test-iface-migrate` | `test/plugin/iface-migrate.ci` | Full make-before-break migration | |

### Future (if deferring any tests)
- Performance benchmark for netlink event throughput — not needed for correctness
- Chaos test: rapid interface flapping — defer to chaos framework

## Files to Modify

- `internal/component/bgp/reactor/reactor.go` — add Bus subscription for `interface/` events
- `internal/component/bgp/reactor/listener.go` — add `startListenerForAddress` / `stopListenerForAddress` methods
- `internal/component/bgp/reactor/reactor_peers.go` — react to address availability events
- `internal/component/bgp/schema/ze-bgp-conf.yang` — extend `local-address` to accept interface names (VyOS `update-source` pattern)
- `internal/component/plugin/registry/registry.go` — ensure interface plugin can register (may already be sufficient)
- `internal/component/plugin/all/all.go` — blank import for new plugin packages (auto-generated by `make generate`)
- `go.mod` — add `github.com/vishvananda/netlink` dependency

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new module) | [x] | `internal/plugins/iface/schema/ze-iface-conf.yang` (shared across platforms) |
| YANG schema (BGP update) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` (`local-address` accepts interface names) |
| RPC count in architecture docs | [ ] | N/A (no new RPCs — Bus events, not RPCs) |
| CLI commands/flags | [x] | `cmd/ze/interface/main.go` |
| CLI usage/help text | [x] | Same |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A (interface plugin is internal) |
| Editor autocomplete | [x] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/plugin/iface-*.ci` |

## Files to Create

- `internal/plugins/iface/schema/ze-iface-conf.yang` — YANG config schema (platform-independent)
- `internal/plugins/iface-linux/iface.go` — Linux interface plugin (netlink management)
- `internal/plugins/iface-linux/register.go` — Linux registration with `//go:build linux`
- `internal/plugins/iface-linux/monitor.go` — Netlink multicast monitor goroutine
- `internal/plugins/iface-darwin/iface.go` — macOS interface plugin (route sockets)
- `internal/plugins/iface-darwin/register.go` — macOS registration with `//go:build darwin`
- `internal/plugins/iface-darwin/monitor.go` — Route socket monitor goroutine
- `cmd/ze/interface/main.go` — CLI subcommand for interface management
- `cmd/ze/interface/show.go` — `ze interface show` handler
- `cmd/ze/interface/create.go` — `ze interface create` handler
- `cmd/ze/interface/addr.go` — `ze interface addr add/del` handler
- `cmd/ze/interface/migrate.go` — `ze interface migrate` handler
- `test/plugin/iface-create.ci` — Functional test: interface creation
- `test/plugin/iface-monitor.ci` — Functional test: monitoring
- `test/plugin/iface-bgp-bind.ci` — Functional test: BGP binding
- `test/plugin/iface-migrate.ci` — Functional test: migration

## Implementation Steps

### Phase 1: Interface Plugin Core (Linux)

1. **Write unit tests** for netlink event → Bus topic mapping → Review: edge cases?
2. **Run tests** → Verify FAIL (paste output)
3. **Implement** `iface-linux` plugin: registration, Bus topic creation, netlink monitor goroutine
4. **Run tests** → Verify PASS
5. **Add dependency** `vishvananda/netlink` to `go.mod`
6. **Functional test** `test-iface-create` — config creates dummy interface

### Phase 2: BGP Bus Subscription

7. **Write unit tests** for BGP reaction to `interface/addr/added` and `interface/addr/removed`
8. **Run tests** → Verify FAIL
9. **Implement** Bus subscription in reactor, listener start/stop on events
10. **Run tests** → Verify PASS
11. **Functional test** `test-iface-bgp-bind` — BGP session after interface IP added

### Phase 3: Migration Orchestration

12. **Write unit tests** for make-before-break ordering
13. **Run tests** → Verify FAIL
14. **Implement** migration sequence in config reload path
15. **Run tests** → Verify PASS
16. **Functional test** `test-iface-migrate` — full migration

### Phase 4: macOS Plugin

17. **Write unit tests** for route socket event → Bus topic mapping
18. **Run tests** → Verify FAIL
19. **Implement** `iface-darwin` plugin with route socket monitoring
20. **Run tests** → Verify PASS

### Phase 5: Verify & Complete

21. `make ze-test` (lint + all ze tests including fuzz + exabgp)
22. **Critical Review** → All 6 checks from `rules/quality.md`
23. Complete spec audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix syntax/types) |
| Test fails wrong reason | Step 1 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

### freeRouter Reference

freeRouter uses separate C helper processes per interface, communicating via UDP on loopback. Each helper opens a raw socket (privileged), then drops root. Key patterns observed:

| Pattern | freeRouter Approach | Ze Approach |
|---------|--------------------|----|
| Interface creation | `tapInt.c` uses `/dev/net/tun` + `TUNSETIFF`; `veth.c` uses netlink `RTM_NEWLINK` | Go netlink via `vishvananda/netlink` — no C helpers needed |
| IP assignment | `tapInt.c` shells out `ip addr add`; `p4mnl_msg.h` uses netlink `RTM_NEWADDR` | Netlink `RTM_NEWADDR` (no shell) |
| State monitoring | `rawInt.c` polls `SIOCGIFFLAGS` every 1 second | Netlink multicast (async, no polling) |
| Interface removal | Process exit (kernel reclaims fd) | Explicit `RTM_DELLINK` for clean teardown |
| Offload management | `seth.c` disables 7 ethtool features via `SIOCETHTOOL` | Not needed — Ze doesn't do raw packet I/O |
| IP migration | Not supported | Make-before-break via Bus events |

### Key netlink details for IPv4 vs IPv6 address assignment

IPv4 `RTM_NEWADDR` requires both `IFA_LOCAL` and `IFA_ADDRESS` attributes.
IPv6 `RTM_NEWADDR` requires only `IFA_ADDRESS` (no `IFA_LOCAL`).
The `vishvananda/netlink` library abstracts this via `netlink.AddrAdd()`.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
