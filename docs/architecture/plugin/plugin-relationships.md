# Plugin Relationships

<!-- source: internal/plugins/*/register.go -- plugin registrations -->
<!-- source: internal/component/bgp/plugin/register.go -- BGP plugin -->
<!-- source: internal/component/iface/register.go -- interface plugin -->
<!-- source: internal/component/bgp/plugins/*/register.go -- BGP sub-plugins -->

This document maps every plugin in ze, what it owns, what it depends on, and how
plugins communicate. Use it to understand cross-plugin impacts when changing config,
bus topics, or the transaction protocol.

> **See also:** [Config Transaction Protocol](../config/transaction-protocol.md) for
> how `ConfigRoots` and `WantsConfig` drive config transaction participation.

---

## 1. System Plugins

These are top-level plugins that own config roots and manage OS or routing resources.

### bgp

| Field | Value |
|-------|-------|
| Location | `internal/component/bgp/plugin/register.go` |
| Description | BGP routing daemon |
| ConfigRoots | `bgp` |
| WantsConfig | `bgp` |
| Dependencies | - |
| FatalOnConfigError | Yes |
| Transaction | Verify (peer diff count), Apply (journal-wrapped reconcilePeers), Rollback (journal replay) |
| VerifyBudget | 5s |
| ApplyBudget | 30s |

**Bus publishes:**

| Topic | When |
|-------|------|
| `bgp/state` | Peer state change (up/down) |
| `bgp/negotiated` | Capability negotiation complete |
| `bgp/update` | UPDATE sent or received |
| `bgp/congestion` | Forward path congestion state change |
| `bgp/listener/ready` | Listener bound to address |

**Bus subscribes:**

| Prefix | Reacts to |
|--------|-----------|
| `interface/` | `addr/added` (start listener), `addr/removed` (stop listener) |

---

### interface

| Field | Value |
|-------|-------|
| Location | `internal/component/iface/register.go` |
| Description | OS network interface monitoring and management |
| ConfigRoots | `interface` |
| WantsConfig | `interface` |
| Dependencies | - |
| Transaction | Verify (parse config), Apply (journal-wrapped applyConfig), Rollback (re-apply previous config) |
| VerifyBudget | 2s |
| ApplyBudget | 10s |

**Bus publishes:**

| Topic | Payload |
|-------|---------|
| `interface/created` | LinkPayload: name, type, index, mtu, managed |
| `interface/deleted` | LinkPayload |
| `interface/up` | StatePayload: name, index |
| `interface/down` | StatePayload |
| `interface/addr/added` | AddrPayload: name, unit, index, address, prefix-length, family |
| `interface/addr/removed` | AddrPayload |
| `interface/dhcp/lease-acquired` | DHCPPayload |
| `interface/dhcp/lease-renewed` | DHCPPayload |
| `interface/dhcp/lease-expired` | DHCPPayload |
| `interface/rollback` | nil (signals transaction rollback, downstream should re-query state) |

**Bus subscribes:** None.

---

### rib

| Field | Value |
|-------|-------|
| Location | `internal/plugins/sysrib/register.go` |
| Description | System RIB: selects best route across protocols by admin distance |
| ConfigRoots | `rib` |
| WantsConfig | `rib` |
| Dependencies | - |
| Commands | `rib show` |
| Transaction | Verify (parse admin distance), Apply (journal-wrapped distance update), Rollback (restore previous distances) |
| VerifyBudget | 1s |
| ApplyBudget | 2s |

**Bus publishes:**

| Topic | When |
|-------|------|
| `system-rib/best-change` | Best route changed for a prefix (per-family) |
| `rib/replay-request` | Requests replay from protocol RIBs |

**Bus subscribes:** `rib/best-change/` prefix (receives Loc-RIB changes from bgp-rib).

---

### fib-kernel

| Field | Value |
|-------|-------|
| Location | `internal/plugins/fib/kernel/register.go` |
| Description | Programs OS routes from system RIB via netlink/route socket |
| ConfigRoots | `fib.kernel` |
| WantsConfig | `fib.kernel` |
| Dependencies | `rib` |
| Commands | `fib-kernel show` |
| Transaction | Verify (accept), Apply (no-op journal, reacts to bus events), Rollback (no-op) |
| VerifyBudget | 1s |
| ApplyBudget | 1s |

**Bus publishes:**

| Topic | When |
|-------|------|
| `fib/external-change` | External kernel route modification detected |
| `system-rib/replay-request` | Requests rib replay on startup |

**Bus subscribes:** `system-rib/best-change` (programs OS routes from system best).

---

### fib-p4

| Field | Value |
|-------|-------|
| Location | `internal/plugins/fib/p4/register.go` |
| Description | Programs P4 switch forwarding entries from system RIB |
| ConfigRoots | `fib.p4` |
| WantsConfig | `fib.p4` |
| Dependencies | `rib` |
| Commands | `fib-p4 show` |
| Transaction | Verify (accept), Apply (no-op journal, reacts to bus events), Rollback (no-op) |
| VerifyBudget | 1s |
| ApplyBudget | 1s |

**Bus publishes:**

| Topic | When |
|-------|------|
| `system-rib/replay-request` | Requests rib replay on startup |

**Bus subscribes:** `system-rib/best-change` (programs P4 entries from system best).

---

### iface-dhcp

| Field | Value |
|-------|-------|
| Location | `internal/plugins/iface/dhcp/register.go` |
| Description | DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal |
| ConfigRoots | - |
| WantsConfig | `interface` (planned -- not yet wired) |
| Dependencies | `interface` |

DHCP client logic exists (lease negotiation, renewal, address management via
`iface.ReplaceAddressWithLifetime`) but config-driven activation is not yet
wired. `NewDHCPClient()` is defined but never called from config.

When complete, DHCP will be a `WantsConfig: ["interface"]` plugin: it reads
interface config to discover which interfaces have DHCP enabled, and
participates in config transactions to start/stop clients on reload. During
apply, it waits for `interface/created` before binding (dependency waiting
pattern from the transaction protocol).

**Bus publishes:**

| Topic | When |
|-------|------|
| `interface/dhcp/lease-acquired` | DHCPv4/v6 ACK received |
| `interface/dhcp/lease-renewed` | Renewal succeeded |
| `interface/dhcp/lease-expired` | Lease timed out |

---

### iface-netlink

| Field | Value |
|-------|-------|
| Location | `internal/plugins/iface/netlink/register.go` |
| Description | Netlink backend for interface plugin (Linux) |
| ConfigRoots | - |
| Dependencies | - |

Not a full plugin. Registers a backend factory with `iface.RegisterBackend("netlink", ...)`.
The interface plugin delegates OS operations to this backend.

---

## 2. BGP Sub-Plugins

These plugins extend BGP behavior. They run within the BGP subsystem and
typically depend on `bgp` and read BGP config.

### Capability Plugins

These declare BGP capabilities injected into OPEN messages.

| Plugin | Location | RFCs | Capability Codes | Dependencies |
|--------|----------|------|-----------------|-------------|
| bgp-gr | `bgp/plugins/gr/` | 4724, 9494 | 64, 71 | bgp, bgp-rib |
| bgp-route-refresh | `bgp/plugins/route_refresh/` | 2918, 7313 | 2, 70 | bgp |
| bgp-role | `bgp/plugins/role/` | 9234 | 9 | bgp |
| bgp-llnh | `bgp/plugins/llnh/` | - | 77 | bgp |
| bgp-hostname | `bgp/plugins/hostname/` | draft | 73 | bgp |
| bgp-softver | `bgp/plugins/softver/` | draft | 75 | bgp |

Capability plugins declare `ConfigRoots: ["bgp"]` in their registration but do not
need direct config change notifications. The BGP reactor mediates config for them:
it parses capability settings from the BGP tree into `PeerSettings.RawCapabilityConfig`
and passes them to plugins during startup (`OnConfigure`). On config reload, changed
peers are torn down and re-created by `reconcilePeers`, which triggers fresh capability
negotiation with updated settings. The reactor handles the config change lifecycle
on behalf of all capability plugins.

### Filter Plugins

These implement ingress/egress route filters.

| Plugin | Location | Ingress | Egress | Dependencies |
|--------|----------|---------|--------|-------------|
| bgp-role | `bgp/plugins/role/` | OTC ingress | OTC egress | bgp |
| bgp-gr | `bgp/plugins/gr/` | - | LLGR egress | bgp, bgp-rib |
| bgp-filter-community | `bgp/plugins/filter_community/` | Community ingress | Community egress | bgp |

### Storage Plugins

| Plugin | Location | Description | Dependencies | Bus publishes |
|--------|----------|-------------|-------------|---------------|
| bgp-adj-rib-in | `bgp/plugins/adj_rib_in/` | Raw UPDATE storage for replay | - | - |
| bgp-rib | `bgp/plugins/rib/` | Loc-RIB: best path selection | - | `bgp-rib/best-change/bgp` |
| bgp-persist | `bgp/plugins/persist/` | Route persistence across restarts | - | - |

### Protocol Plugins

| Plugin | Location | RFCs | Dependencies |
|--------|----------|------|-------------|
| bgp-rpki | `bgp/plugins/rpki/` | 6811, 8210 | bgp, bgp-adj-rib-in |
| bgp-rpki-decorator | `bgp/plugins/rpki_decorator/` | - | bgp, bgp-rpki |
| bgp-rs | `bgp/plugins/rs/` | 7947 | bgp-adj-rib-in |
| bgp-aigp | `bgp/plugins/aigp/` | 7311 | - |
| bgp-watchdog | `bgp/plugins/watchdog/` | - | bgp |
| bgp-healthcheck | `bgp/plugins/healthcheck/` | - | bgp, bgp-watchdog |

---

## 3. NLRI Family Plugins

These register address family encoding/decoding. Auto-loaded when BGP config
references their families. No config roots, no bus interaction.

| Plugin | Location | Families | RFCs |
|--------|----------|----------|------|
| bgp-nlri-evpn | `bgp/plugins/nlri/evpn/` | l2vpn/evpn | 7432, 9136 |
| bgp-nlri-vpn | `bgp/plugins/nlri/vpn/` | ipv4/mpls-vpn, ipv6/mpls-vpn | 4364, 4659 |
| bgp-nlri-labeled | `bgp/plugins/nlri/labeled/` | ipv4/mpls-label, ipv6/mpls-label | 8277 |
| bgp-nlri-flowspec | `bgp/plugins/nlri/flowspec/` | ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn | 8955, 8956 |
| bgp-nlri-vpls | `bgp/plugins/nlri/vpls/` | l2vpn/vpls | 4761, 4762 |
| bgp-nlri-mvpn | `bgp/plugins/nlri/mvpn/` | ipv4/mvpn, ipv6/mvpn | 6514 |
| bgp-nlri-rtc | `bgp/plugins/nlri/rtc/` | ipv4/rtc | 4684 |
| bgp-nlri-ls | `bgp/plugins/nlri/ls/` | bgp-ls/bgp-ls, bgp-ls/bgp-ls-vpn | 7752, 9085, 9514 |
| bgp-nlri-mup | `bgp/plugins/nlri/mup/` | ipv4/mup, ipv6/mup | draft |

---

## 4. Dependency Graph

### Startup Tiers

Plugins load in dependency order. Each tier completes before the next starts.

```
Tier 0 (no dependencies):
  bgp, interface, rib, bgp-adj-rib-in, bgp-rib,
  bgp-aigp, bgp-persist, all NLRI family plugins

Tier 1 (depends on tier 0):
  iface-dhcp (-> interface)
  fib-kernel (-> rib)
  fib-p4 (-> rib)
  bgp-gr (-> bgp, bgp-rib)
  bgp-route-refresh (-> bgp)
  bgp-role (-> bgp)
  bgp-llnh (-> bgp)
  bgp-hostname (-> bgp)
  bgp-softver (-> bgp)
  bgp-filter-community (-> bgp)
  bgp-watchdog (-> bgp)
  bgp-rpki (-> bgp, bgp-adj-rib-in)
  bgp-rs (-> bgp-adj-rib-in)

Tier 2 (depends on tier 1):
  bgp-healthcheck (-> bgp, bgp-watchdog)
  bgp-rpki-decorator (-> bgp, bgp-rpki)
```

### Data Flow Chains

```
Wire -> BGP reactor -> bgp-rib (Loc-RIB)
  |                      |
  |                      +-> bgp-rib/best-change/bgp -> rib
  |                                                    |
  |                                                    +-> system-rib/best-change -> fib-kernel (OS routes)
  |                                                    +-> system-rib/best-change -> fib-p4 (P4 entries)
  |
  +-> bgp-adj-rib-in (raw storage)
  |     |
  |     +-> bgp-rpki (validation)
  |     +-> bgp-rs (route server replay)
  |
  +-> bgp-gr (graceful restart state)
```

```
OS kernel -> iface-netlink (monitor)
  |
  +-> interface/created -------> (consumers: DHCP, monitoring)
  +-> interface/addr/added ----> BGP reactor (listener binding)
  +-> interface/addr/removed --> BGP reactor (listener cleanup)
  +-> interface/dhcp/* --------> (consumers: BGP, DNS)
```

---

## 5. Config Ownership Map

| Config Root | Owner | Read By (WantsConfig) | Notes |
|-------------|-------|-----------------------|-------|
| `bgp` | bgp | - | BGP reactor mediates config for all sub-plugins via reconcilePeers |
| `interface` | interface | - | BGP reads address changes via bus events, not WantsConfig |
| `rib` | rib | - | |
| `fib.kernel` | fib-kernel | - | |
| `fib.p4` | fib-p4 | - | |

Currently no plugin uses `WantsConfig` to read another plugin's config. All
cross-plugin config dependencies are mediated by the owning plugin (BGP reactor
for capability plugins) or by bus events (interface for BGP). The `WantsConfig`
mechanism exists for future plugins that need direct access to another root's
config diffs during transactions (e.g., a DHCP plugin reading interface config).

---

## 6. Bus Topic Map

### Topic Hierarchy

```
bgp/
  state                    bgp -> (monitoring, web UI)
  negotiated               bgp -> (monitoring)
  update                   bgp -> (monitoring, event subscribers)
  congestion               bgp -> (monitoring)
  listener/ready           bgp -> (informational)

interface/
  created                  interface -> (DHCP, monitoring)
  deleted                  interface -> (DHCP, monitoring)
  up                       interface -> (monitoring)
  down                     interface -> (monitoring)
  addr/added               interface -> bgp (listener binding)
  addr/removed             interface -> bgp (listener cleanup)
  dhcp/lease-acquired      interface -> (BGP, DNS)
  dhcp/lease-renewed       interface -> (informational)
  dhcp/lease-expired       interface -> (informational)

rib/
  best-change/bgp          bgp-rib -> rib
  replay-request           rib -> (protocol RIBs)

system-rib/
  best-change              rib -> fib-kernel, fib-p4
  replay-request           fib-kernel, fib-p4 -> rib

fib/
  external-change          fib-kernel -> (monitoring)

config/                    (transaction protocol -- see transaction-protocol.md)
  verify                   engine -> participating plugins
  verify/ok                plugin -> engine
  verify/failed            plugin -> engine
  verify/abort             engine -> plugins
  apply                    engine -> participating plugins
  apply/ok                 plugin -> engine
  apply/failed             plugin -> engine
  rollback                 engine -> plugins
  rollback/ok              plugin -> engine
  committed                engine -> plugins (discard journals)
  applied                  engine -> observers
  rolled-back              engine -> observers
```

### Cross-Plugin Bus Dependencies

These are runtime dependencies expressed through bus subscriptions, distinct from
startup dependencies in the `Dependencies` field.

| Subscriber | Topic prefix | Publisher | Effect |
|------------|-------------|-----------|--------|
| BGP reactor | `interface/` | interface | Binds/unbinds listeners on address changes |
| rib | `rib/best-change/` | bgp-rib | Selects system best routes from protocol RIBs |
| fib-kernel | `system-rib/best-change` | rib | Programs OS routing table |
| fib-p4 | `system-rib/best-change` | rib | Programs P4 switch entries |

---

## 7. Config Transaction Participation

Based on the [Config Transaction Protocol](../config/transaction-protocol.md),
this is which plugins participate in config transactions and why.

| Plugin | Participates | Reason |
|--------|-------------|--------|
| bgp | Yes (owner) | Owns `bgp` root, applies peer changes |
| interface | Yes (owner) | Owns `interface` root, creates/removes interfaces |
| rib | Yes (owner) | Owns `rib` root, applies admin distance config |
| fib-kernel | Yes (owner) | Owns `fib.kernel` root |
| fib-p4 | Yes (owner) | Owns `fib.p4` root |
| bgp-gr | No | Config mediated by BGP reactor via reconcilePeers |
| bgp-route-refresh | No | Config mediated by BGP reactor |
| bgp-role | No | Config mediated by BGP reactor |
| bgp-llnh | No | Config mediated by BGP reactor |
| bgp-hostname | No | Config mediated by BGP reactor |
| bgp-softver | No | Config mediated by BGP reactor |
| bgp-filter-community | No | Config mediated by BGP reactor |
| bgp-watchdog | No | Config mediated by BGP reactor |
| bgp-healthcheck | No | Config mediated by BGP reactor |
| bgp-rpki | No | Config mediated by BGP reactor |
| iface-dhcp | Planned (reader) | Will use `WantsConfig: ["interface"]` to start/stop DHCP clients on interface changes. Not yet wired. |
| bgp-adj-rib-in | No | No config roots, no WantsConfig |
| bgp-rib | No | No config roots, no WantsConfig |
| bgp-persist | No | No config roots, no WantsConfig |
| bgp-aigp | No | No config roots, no WantsConfig |
| bgp-rs | No | No config roots, no WantsConfig |
| bgp-rpki-decorator | No | No config roots, no WantsConfig |
| All NLRI plugins | No | Pure codec, no config involvement |
