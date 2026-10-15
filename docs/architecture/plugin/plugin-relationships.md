# Plugin Relationships

<!-- source: internal/component/bfd/register.go -- plugin registrations (representative) -->
<!-- source: internal/component/bgp/plugin/register.go -- BGP plugin -->
<!-- source: internal/component/iface/register.go -- interface plugin -->
<!-- source: internal/component/bgp/plugins/rib/register.go -- BGP sub-plugins (representative) -->

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

BFD (`internal/component/bfd/register.go`) starts after `interface` when both are
selected: the interface component publishes logical-to-kernel bindings before
BFD configures pinned sessions. Its `StartAfter` declaration is order-only, not
a dependency that activates an interface backend for BFD-only configurations.
Runtime protocol clients use the same binding boundary before a session reaches
the BFD engine.

<!-- source: internal/component/bfd/register.go -- StartAfter -->
<!-- source: internal/component/bfd/session_identity.go -- canonicalRequest -->

---

### rib

| Field | Value |
|-------|-------|
| Location | `internal/component/sysrib/register.go` |
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
| Commands | `show fib kernel` |
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
| Commands | `show fib p4` |
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
| WantsConfig | - |
| Dependencies | `interface` |

The DHCP package registers a factory with the `interface` component at init time.
The `interface` plugin owns `WantsConfig: ["interface"]`, parses DHCP leaves from
interface units, and calls the factory from `reconcileDHCP()` on startup and reload

<!-- source: internal/plugins/iface/dhcp/register.go -- SetDHCPClientFactory and newDHCPClientFromFactory -->
<!-- source: internal/component/iface/register.go -- reconcileDHCP and sdk.Registration WantsConfig -->

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

Capability plugins declare `ConfigRoots: ["bgp"]` and `WantsConfig: ["bgp"]`, so
each takes part in every transaction that changes `bgp` (section 7). They read
their capability settings at startup (`OnConfigure`) and declare them through
`DeclareCapabilitiesFrom`, which refuses at config-verify a reload that changes,
adds or removes a declared capability, because a capability already sent in an
OPEN cannot change while the session runs. bgp-gr, bgp-hostname, bgp-llnh,
bgp-softver read nothing else, so they declare that they apply nothing;
bgp-role also applies `strict`, `export` and its name map live. bgp-route-refresh
reads no leaf (the engine advertises route-refresh) and also declares that it
applies nothing.

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
| bgp-rib | `bgp/plugins/rib/` | Mandatory Loc-RIB: best path selection and in-memory sent history for reconnect | - | `bgp-rib/best-change/bgp` |

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
  bgp-aigp, all NLRI family plugins

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
| `bgp` | bgp | every BGP sub-plugin with `WantsConfig: ["bgp"]` (section 7), the external exabgp plugin | each reader parses its own subtree; peers are reconciled by the reactor |
| `interface` | interface | static | BGP reads address changes via bus events, not WantsConfig |
| `rib` | rib | - | |
| `fib.kernel` | fib-kernel | - | |
| `fib.p4` | fib-p4 | - | |

Several plugins read a root another plugin owns, either through the SDK
`Registration.WantsConfig` or through the registry's `ConfigReads`, and each
such read makes them a participant in every transaction that changes that root
(section 7). Examples: `interface` reads `class-of-service` to resolve unit QoS
profiles against the candidate; `static` reads `routing-table`, `bfd` and
`interface`; `bgp` and `ospf` read `bfd`; `bgp-rpki` reads `pki`; `bgp-bmp`
reads `environment` and `system`. To list them, grep `ConfigReads:` and
`WantsConfig:` in the non-test Go files.

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

**Who participates.** A plugin is a participant in a transaction when a root it
wants changed. The roots it wants are its SDK `Registration.WantsConfig` joined
with its registry `ConfigReads` (`joinConfigReads`, `server/startup.go`). There
is no subtree granularity: every plugin that wants `bgp` takes part in every
`bgp` edit, and every plugin that wants `service` in every `service` edit, so
each one compares its parsed candidate with what it runs and does nothing when
they are equal. `ConfigRoots` alone autoloads a plugin and does not make it a
participant.

**How a participant answers.** Every participant registers one of three
answers. A participant with none is refused by the SDK at config-verify, naming
the plugin and the changed roots, with the restart steps
(`refuseWithoutApply`, `pkg/plugin/sdk/sdk_callbacks.go`); a reload never
reports success for a plugin that kept its old config.

| Answer | Registered by | Meaning |
|--------|---------------|---------|
| Transaction helper | `sdk.NewConfigTx(...).Register(p)` (`pkg/plugin/sdk/config_tx.go`) | verify parses into a candidate, apply installs it and keeps the previous, rollback reinstalls the previous; an unchanged candidate installs nothing |
| Own handlers | `OnConfigVerify`, `OnConfigApply`, `OnConfigRollback` | the plugin holds its own verify-to-apply state; a verify may refuse a change it cannot take while running |
| Applies nothing | `DeclareAppliesNothing(reason)` | the plugin reads no leaf it could re-apply (its config is only a capability declaration, or its root is an empty container); the reload is accepted and a debug line names the reason |

**The BGP plugins.** The table re-derives the rows from the registrations
(2026-10-15). It is a copy: the source is each plugin's `sdk.Registration` and
its `run*` function, and no check compares the two, so re-derive it when a
plugin's registration changes.

| Plugin | Participates | Answer |
|--------|--------------|--------|
| bgp | Yes (owner of `bgp`; reads `bfd`) | own handlers, applies peer changes |
| bgp-filter-aspath, -aspath-length, -community, -community-match, -family, -irr, -modify, -path-asn, -prefix, -remove-private-as | Yes (`bgp`) | transaction helper: the next UPDATE filtered after the reload uses the new definitions; stored and advertised routes are not re-evaluated |
| bgp-healthcheck | Yes (`bgp`) | transaction helper: reconciles the probe set without waiting for stopped probes |
| bgp-rib | Yes (`bgp`) | transaction helper: `maximum-paths`, `relax-as-path`, blackhole rules (reselects stored routes), AS notation |
| bgp-role | Yes (`bgp`) | transaction helper for `strict`, `export` and the name map; prunes learned roles at commit (`OnConfigCommitted`); a role value change is refused by the capability comparison |
| bgp-rs | Yes (`bgp`) | transaction helper: `route-server { worker-queue-size }` for workers created afterwards; the `ze.bgp.route-server.worker-queue-size` env var still wins |
| bgp-watchdog | Yes (`bgp`) | own handlers: merges pools, keeps announce state, withdraws removed announced entries |
| bgp-rpki | Yes (`bgp`; reads `pki`) | own handlers |
| bgp-bmp | Yes (`bgp`, `environment`, `system`) | own handlers |
| bgp-epe, bgp-ls-export | Yes (own root) | own handlers |
| bgp-gr, bgp-hostname, bgp-llnh, bgp-softver | Yes (`bgp`) | applies nothing: their config is a capability declaration, and a change to it is refused by `DeclareCapabilitiesFrom` |
| bgp-route-refresh | Yes (`bgp`) | applies nothing: reads no leaf; the engine advertises route-refresh |
| bgp-adj-rib-in, bgp-aigp, bgp-capa, bgp-rr, bgp-rpki-decorator, redistribute-orchestrator | No | no `WantsConfig`, no `ConfigReads` |
| iface-dhcp | No | no config root, no `WantsConfig`: the `interface` plugin creates its DHCP clients through the factory iface-dhcp registers (`iface.SetDHCPClientFactory`) |
| All NLRI plugins | No | Pure codec, no config involvement |

Outside BGP, every plugin that wants a root registers own handlers, except
these: `class-of-service` (cos), `routing-table`, and the `service` plugins
dhcp-server, tftp-server, image-server, as112 and geodns use the transaction
helper; connected and kernel apply nothing (empty containers); the exabgp
bridge refuses at verify every non-capability change under `exabgp` while it
runs, and the external exabgp plugin applies nothing. What each plugin's reload
does for the operator is in [Config Reload](../../guide/config-reload.md).
