# Spec: container-3-network

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature. The owner ruled (umbrella O-3, O-7) that containers ship after the first release and that their networks and published ports are container config realised through Ze's interface and firewall mechanisms. |
| RFC gaps | - |
| Depends | spec-container-1-runtime |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A container that Ze starts has a network namespace with a loopback device and
nothing else. This child of `plan/post-release/features/spec-container-0-umbrella.md`
gives it a network, in state Ze owns and shows:

- A container network is a bridge device with the gateway address on it. Each
  attached container has one link: the host end is a member of the bridge, the
  other end is inside the container.
- A published port is a destination translation, and a container network's
  outbound traffic is source-translated, in firewall tables Ze owns by the
  `ze_` prefix and lists with its own commands.
- A container may instead share the host network namespace, or have no network.
- Nothing is installed beside Ze's firewall, and no engine or config code names
  the container feature: the plugin reaches interfaces and the firewall through
  declared requests that also work when it runs as a separate process.

The owner rulings are fixed by the umbrella (O-7 for this child). This spec
refines umbrella AC-8 and AC-9.

### Config nodes this child needs

The words and the grammar are set by `spec-container-4-operator-surface.md`.
This table states what must be expressible.

| Node (plain words) | On | Value | Rule this child applies |
|--------------------|----|-------|-------------------------|
| Network name | network | key | The bridge device is named `zcn-` plus the name, so the name is at most 11 characters and is refused when longer, never truncated |
| Prefix | network | one IPv4 and/or one IPv6 prefix | IPv4 length 1 to 30, IPv6 length 1 to 126. Two networks must not overlap, and a prefix must not contain an address the interface config sets elsewhere |
| MTU | network | 68 to 16000, default 1500 | Applied to the bridge and both ends of every link. Refused below 1280 when the network has an IPv6 prefix |
| Outbound translation | network | on or off | See Key Design Decisions for the default |
| Name servers | network, container | list of addresses | See "Name resolution" |
| VRF | network | name | Present so the refusal can say why. Refused by the plugin's verifier until VRF support ships |
| Network mode | container | attached to networks, host, or none | Host and none refuse every node below |
| Attachment | container | network name, optional address per family | An address the operator sets, or one assigned from the prefix |
| Published port | container | protocol (tcp, udp), router port or range, container port or range, optional router addresses | Ranges have equal size. See "Published ports" |

### The seam with the runtime child

`spec-container-1-runtime.md` owns the process. This child needs four things from it.

| # | The runtime child provides | This child does with it |
|---|----------------------------|-------------------------|
| 1 | A call made after the container is created and before its entrypoint runs, carrying the container name and the init process id. The start continues only when the call returns success, and a failure destroys the container and records the reason | Creates each link with its container end inside that process's network namespace, and configures it |
| 2 | The network mode as an input when it builds the container: a namespace of its own, or the host's | For host mode the runtime creates no network namespace and this child is not called |
| 3 | A call when a container has exited or is destroyed | Removes the links of that container |
| 4 | A place in the container root for a resolver file, written before the entrypoint runs | Supplies the resolver addresses |

The loopback device is brought up by the runtime's library, before call 1. This
child never touches it.

### Addressing

| Rule | Detail |
|------|--------|
| Gateway | The network address plus one, in each family. It is on the bridge, and it is each container's default route |
| Operator-set address | Inside the prefix, not the network address, not the IPv4 broadcast address, not the gateway, unique in the network |
| Assigned address | The lowest free address above the gateway that no container holds or is configured with. The assignment is stored in the plugin's runtime state under the network and container names, so it is the same after a restart, a reload and a reboot. It is released when the container or its attachment leaves the config |
| Exhaustion | A config with more attachments than free addresses is refused at verify |
| Hardware address | Each container end gets a locally administered unicast address derived from the network and container names, so neighbours keep a valid entry across a restart. Two attachments that derive the same address are refused at verify |
| IPv6 | The address is installed with duplicate address detection off, because Ze guarantees uniqueness and an entrypoint may bind at once |

### Published ports and outbound translation

The plugin contributes two tables whose names it reserves: `ze_container`
(family ip) and `ze_container6` (family ip6). They follow the config, not the
run state: a configured, enabled container's translation exists while the
container is stopped, and a disabled container has none.

| Chain | Hook | Terms |
|-------|------|-------|
| Destination translation | prerouting, and output for connections the router itself opens | One term per published port: the protocol, the router port or range, the destination is an address of this router (and is one of the listed router addresses when the operator set any), then translate to the container address and port |
| Source translation | postrouting | Per network with outbound translation on: source in the network prefix, jump to that network's chain. The network's chain returns when the output interface is the network's bridge, then masquerades |

The "destination is an address of this router" match does not exist in the
firewall model today. Without it a published port would capture transit traffic
to the same port. This child adds it to the firewall model as a general match.

A published port does not open the operator's filter. A drop in any base chain
is final, so a forward chain with policy drop needs a term that accepts the
traffic. Doctor reports that case.

### Name resolution

| Order | Source of a container's resolver addresses |
|-------|--------------------------------------------|
| 1 | The container's own name servers |
| 2 | The name servers of its first network |
| 3 | The router's own resolver list at container start (`system name-server`, then learned resolvers, as `dns-source` orders them), with loopback addresses removed |

Ze does not answer container names in this child. An operator who wants one
stable resolver address runs the LAN resolver on the network's gateway address
and names it on the network.

### Problem classes

Each row is a doctor check with a diagnostic code registered from the plugin's
own package. A refusal row is also a config-phase check.

| Code (lower-kebab) | Phase | Problem | Fix the message names |
|--------------------|-------|---------|-----------------------|
| `container-network-backend` | config, verify | The interface backend cannot create a container link, or the firewall backend cannot carry the translation tables | The backend leaf |
| `container-kernel-link`, `-bridge`, `-translation`, `-local-match` (absent and unknown each) | kernel capability | The kernel lacks veth, bridge, translation, or the local-address match | The kernel option |
| `container-network-device-conflict` | runtime | A device Ze does not own holds the bridge name | Rename the network or remove the device |
| `container-network-device-state` | runtime | The bridge is absent, down, or lacks its gateway address | The listing command, and the commit that re-applies |
| `container-link-attach-failed` | runtime | A container did not start because its link could not be made | The reason, and the restart command |
| `container-address-conflict` | config, verify | Duplicate address, address outside the prefix, overlapping prefixes, exhausted prefix | The node to change |
| `container-address-overlap-learned` | runtime | A network prefix contains an address the kernel holds on another interface | Change the prefix |
| `container-port-conflict` | config, verify | Two publishers, or a publisher and a Ze listener, on one protocol, address and port | The node to change |
| `container-translation-not-applied` | runtime | The firewall refused or did not apply the plugin's tables | The firewall error, and the commit that re-applies |
| `container-forwarding-disabled` | runtime | IP forwarding is off for a family a network uses | The sysctl node |
| `container-filter-drops-published` | runtime | A forward chain has policy drop and no term names the network's bridge | The term to add |
| `container-resolver-empty` | runtime | A container has no resolver address | The name server node |
| `container-host-network-settings` | config, verify | A host or none container sets a port, an attachment or a network setting | Remove the node or change the mode |
| `container-vrf-unimplemented` | config, verify | A network sets the VRF node | Remove the node |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the process boundary and registration
  → Constraint: the plugin runs internal or forked. A Go call into `iface` or `firewall` state from the plugin mutates a disconnected copy when forked, so every write crosses as a typed request
  → Constraint: a declaration is a pure value. Reservations and grants are made from `init()`, so every process of the binary holds the same set
- [ ] `ai/doors/forwarding.md` - interfaces and firewall
  → Constraint: Ze owns only kernel state it marked. A device is Ze's only when it carries the owned alias and is of a kind the registry creates
  → Constraint: "The firewall table registry is process-local. A producer that can run forked needs an RPC, not a call to `RegisterTables`"
  → Constraint: exact or refuse at verify. The VPP interface backend has no veth
- [ ] `ai/doors/plugin-infra.md` - engine operations
  → Decision: one engine-op entry per RPC. The JSON path and the direct path derive from the same entry, so the new requests are owner-registered engine operations and nothing else
  → Constraint: an engine call during startup waits at the barrier unless the operation is declared safe for the configure stage
- [ ] `ai/doors/system-plugins.md` - placement
  → Decision: the network code is a package of the gated container plugin under `internal/plugins/`, and it owns its doctor checks and codes
- [ ] `ai/doors/config-yang.md` - validation
  → Constraint: `ze config validate` never calls a live `OnConfigVerify`. Every refusal in this spec is in the plugin's in-process verifier too
  → Constraint: a `ze:validate` under a root runs only when the root is in `validatedSections`, a literal list in config code. The VRF refusal is therefore the plugin's own verifier, which needs no edit there
  → Constraint: a removed root arrives as an empty section. An empty section withdraws every bridge, link and table
- [ ] `ai/doors/core.md` - runtime state
  → Constraint: a plugin persists through the SDK state calls, under keys its package grants with `statestore.RegisterPluginKeys`. A shipped key is a contract
- [ ] `ai/doors/appliance.md`, `ai/doors/hub-engine.md` - the device and the daemon's stop
  → Constraint: a state key an image in the field writes must still work after the next image. The assignment value is written whole and read tolerantly
  → Constraint: plugins stop together with no order. State that must be gone after an orderly stop is removed by one ordered actor that holds a live backend
- [ ] `docs/architecture/core-design.md` sections 14, 14a, 14b, 19 - backends and boundaries
  → Decision: `ze:backend` on a schema node plus `config.ValidateBackendFeaturesJSON` is the commit-time gate. The container network node carries the annotation and the plugin runs the walker against the interface backend
  → Constraint: `ApplyAll` holds one lock for the whole kernel round trip and re-applies every owner's tables. The plugin contributes at config changes only, never per container start
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - table ownership
  → Constraint: Rule 2b. A producer reserves its table names with `MustReserveTableNames` from `init()`, with the plugin, what decides the rules, and the command that shows it
  → Constraint: Rule 3. On an orderly stop the firewall engine removes every Ze table. A crash keeps them, so the next start claims the reserved names and withdraws what the config no longer asks for (Rule 4)
- [ ] `docs/architecture/iface/logical-name-resolution.md` - names
  → Constraint: created kinds are made under their own name and have no alias mapping. The backend's own netlink lookups are the allowed direct resolution
- [ ] `docs/architecture/plugin/plugin-system.md`, "Communication patterns", "The process boundary"
  → Decision: the EventBus carries no answer, so it cannot carry "the link is in place, release the container". A typed request does
- [ ] `ai/rules/protocol.md` - deny by default and exactness
  → Constraint: a change that captures traffic must find every Ze service under it. A published port can capture a Ze listener's port, so the conflict check includes Ze's listeners
- [ ] `ai/rules/platform-linux.md`, `ai/rules/testing.md` - proof
  → Constraint: a `.ci` that needs a kernel feature carries `option=needs-linux` and runs in the QEMU guest on Ze's runtime kernel. Kernel-touching Go tests carry `integration && linux`
  → Constraint: every reachable lifecycle scenario is proven in both plugin modes

**Key insights:** (minimal context to resume after compaction)
- `pluginserver.RegisterEngineOp` lets an owner component declare a plugin-to-engine request from its own `init()`. The handler receives the authenticated process. `RegisterProcessGenerationCleanup` tells the owner when a plugin process is gone. Only the system RIB uses both today.
- The interface component already has an owned-device registry: a plugin registers a macvlan, the reconcile pass creates it marked `ze:owned:<owner>` in the link alias, and an orphan scan deletes an aliased macvlan nobody registers. It is process-local, so VRRP refuses to run forked.
- `plugin/server` imports `iface`, so `iface` cannot import `plugin/server`. The interface requests register from `internal/component/iface/cmd`, which already registers the interface RPCs.
- The vendored netlink library creates a veth with its peer end directly in another namespace, and a netlink socket bound to that namespace configures it. No Ze thread stays in the container's namespace.
- The firewall model has DNAT, SNAT, masquerade, jump and return, and interface and prefix matches. It has no negated match and no "destination is local" match.
- The firewall config section is a JSON form with a parser (`ParseFirewallConfig`), a validator and a per-backend verifier, all callable without daemon state. It is the wire form of a contributed table.
- The VPP firewall verifier refuses a non-base chain, so the source-translation chains are refused there at verify.
- The LAN resolver binds with free-bind, so it can listen on a gateway address that appears later.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/iface/backend.go` - `Backend` has `CreateVeth(name, peerName)`, `CreateBridge`, `BridgeAddPort`, `CreateMacvlanDevice` (alias set at create). No method names a namespace
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - `CreateVeth` adds the pair in the host namespace and sets both ends up
- [ ] `internal/plugins/iface/netlink/bridge_linux.go` - `BridgeAddPort` enslaves by name
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `CreateVeth` answers "not supported on VPP backend"
- [ ] `internal/component/iface/device_owner.go`, `macvlan.go`, `config_apply.go` `reconcileOwnedDevices` - the owned-device registry, the alias prefix `ze:owned:`, name composition that refuses instead of truncating, create, drift re-assert, and an orphan scan that requires kind macvlan and the alias
- [ ] `internal/component/iface/address_owner.go` - a plugin registers addresses on a device and the reconcile adds and prunes them
- [ ] `internal/component/iface/register.go` - after `p.Run` returns, the engine stops its workers and clients and ends in `CloseBackend`; it deletes no device
- [ ] `internal/component/iface/cmd/cmd.go`, `internal/component/iface/yang/ze-iface-api.yang` - the package and the module that hold the interface RPCs, apart from `iface` to avoid the import cycle
- [ ] `internal/component/kernelcap/namespace_linux.go`, `kernelcap.go` - a probe runs in a throwaway namespace on a thread that is never unlocked; a `Capability` is enrolled from the owner's `init()`
- [ ] `internal/plugins/iface/netlink/kernelcap_linux.go` - the enrolment pattern; veth and bridge are not enrolled
- [ ] `internal/component/firewall/registry.go`, `reservation.go`, `backend.go`, `config.go`, `model.go`, `yang/ze-firewall-conf.yang` - `RegisterTables` refuses a name without `ze_` or one another owner reserved; `ApplyAll` verifies the merged set and records it for the listings; the model's matches and actions; the `from` and `then` leaves
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerNAT`, `lowerMasquerade` - the translation actions take the family from the target address
- [ ] `internal/plugins/firewall/vpp/verify.go` - accepts DNAT, SNAT and masquerade in a base NAT chain and refuses every non-base chain
- [ ] `internal/component/plugin/server/register_engine_op.go`, `internal/component/sysrib/register_route_view.go`, `pkg/plugin/sdk/sdk_route_view.go` - owner-registered engine operations, their startup admission, and the SDK side
- [ ] `internal/component/plugin/registry/registry.go`, `config_need.go` - `ConfigRoots`, `ConfigReads`, `Dependencies`, `InProcessConfigVerifier`; `ConfigNeed` refuses a config value whose provider plugin is compiled out
- [ ] `internal/component/config/backend_gate.go`, `listener.go`, `validate_sections.go` - the backend gate walker, the listener conflict check over `ze:listener` lists, the literal list of validated sections
- [ ] `internal/component/config/system/yang/ze-system-conf.yang`, `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang`, `internal/plugins/dnsresolver/server.go` - `system name-server`, `dns-source`, the LAN resolver's listeners, `allow-from` and free-bind
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list veth`, `list bridge` (both `ze:backend "netlink"`), `leaf vrf` with `ze:validate "unimplemented-vrf"`
- [ ] `internal/component/sysctl/events/events.go`, `internal/plugins/vrrp/register.go`, `internal/core/statestore/permissions.go`, `internal/le/plugin/boundary/pluginboundary.go` - sysctl defaults over the bus; the in-process-only guard; state key grants; the watch list of in-process-only calls

**Behavior to preserve:**
- A config-declared veth, bridge and bridge member behave as today. The config apply and prune never touch a device outside the config.
- VRRP's owned macvlans: create, drift re-assert, orphan deletion, the `ze_iface_owned_devices` gauge.
- Every existing firewall owner's tables, reservations and merge rules. `show firewall ruleset` output for them.
- The listener conflict verdict for every existing service.
- A device or nft table that does not carry Ze's mark is never changed or deleted.

**Behavior to change:**
- The interface engine's orderly stop deletes every device that carries the owned alias before it closes its backend. Today the owner's own release races that close.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Owner-registered engine operations | `internal/component/plugin/server/register_engine_op.go` `RegisterEngineOp`, `RegisterProcessGenerationCleanup`; `pkg/plugin/sdk/sdk_route_view.go` | A component declares a plugin-to-engine request from `init()`; the handler gets the authenticated process; the SDK method works over the bridge and over RPC. Used by the system RIB only | Extends: this is the carrier. The interface and firewall components each declare their requests in their own namespace, and each registers a cleanup for a plugin process that is gone |
| Owned-device registry | `internal/component/iface/device_owner.go` `RegisterOwnedMacvlan`; `config_apply.go` `reconcileOwnedDevices`; `macvlan.go` `ComposeOwnedDeviceName` | A plugin registers a macvlan; the reconcile creates it with the alias `ze:owned:<owner>`, re-asserts drift, and deletes an aliased macvlan with no registration. Process-local | Extends: two more kinds (bridge, container link), reached through the engine operations so a forked plugin works. The orphan scan keeps its two-fact rule: an owned kind and the alias |
| Owned-address registry | `internal/component/iface/address_owner.go` `RegisterOwnedAddresses` | A plugin's addresses on a device are part of the desired state and are pruned when released | Extends: the gateway address of an owned bridge is registered through it by the handler |
| Interface backend registry | `internal/component/iface/backend.go` `RegisterBackend`, `Backend` | One backend per data plane behind one interface | Extends: two methods, create an owned bridge and create a link whose peer end is in a namespace. The VPP and non-Linux backends refuse both |
| Commit-time backend gate | `internal/component/config/backend_gate.go` `ValidateBackendFeaturesJSON`; `core-design.md` section 14a | Refuses a config node whose `ze:backend` annotation excludes the active backend, naming the path and the backend leaf | Extends: the container network node carries `ze:backend "netlink"`, and the plugin runs the walker with the interface backend it reads from the `interface` root |
| Firewall table registry and reservations | `internal/component/firewall/registry.go` `RegisterTables`, `ApplyAll`; `reservation.go` `MustReserveTableNames` | Owners register tables; one reconcile applies the union and records it for the listings; reserved names are refused to others. Process-local | Extends: an engine operation registers the calling plugin's tables and reconciles. Only a name the caller reserved at `init()` is accepted |
| Firewall config form | `internal/component/firewall/config.go` `ParseFirewallConfig`; `validate.go` `ValidateTables`; `backend.go` `RunVerifier` | Turns the firewall config section into tables, checks them, and asks the backend's verifier | Extends: the contributed tables travel in this form, so the schema is the wire contract and the plugin's verifier runs the same three checks offline |
| Policy routing as a table producer | `internal/plugins/policyroute/register.go` (traced in `ai/digests/firewall.md`, step 13) | Registers `ze_pr` and reconciles, in process | Model. It does not cross a process, so its call is not reused |
| `registry.ConfigNeed` | `internal/component/plugin/registry/config_need.go` | Refuses a config value whose provider plugin is compiled out, and pulls the provider when the value holds | Does not apply: `interface` and `firewall` are always linked. The plugin declares both as `Dependencies` so they configure first |
| EventBus | `pkg/ze/eventbus.go`; `internal/component/sysctl/events/events.go` `EventDefault` | One-to-many notification with no answer. A plugin contributes a sysctl default this way | Extends for sysctl only: the plugin contributes the forwarding defaults. Does not carry links or tables, because the caller needs the answer |
| Text command dispatcher | `plugin-system.md`, "Communication patterns", `DispatchCommand`; `ze-iface-api.yang` `interface-create-veth` | A plugin sends a command to the registry and reads the structured answer. Interface create and delete exist as operator RPCs | Extends, read only: the doctor checks read the interface and ruleset listings through it. Not used for writes: a write command would be an operator command that puts a link into any process's namespace |
| DirectBridge | `pkg/plugin/rpc/bridge.go` | The in-process form of the same protocol | Extends by derivation: an engine operation gets its direct path from its one entry |
| Locked-thread namespace helper | `internal/component/kernelcap/namespace_linux.go` `InThrowawayNetworkNamespace` | Runs a probe in a fresh namespace on a thread that never returns to the scheduler | Extends for the kernel probes only (veth, bridge, translation). The link itself needs no thread in the container's namespace |
| Kernel requirement enrolment | `internal/component/kernelcap/kernelcap.go` `MustRegister`; `gokrazy/kernel/runtime.config`, `runtime.require` | One enrolment refuses at validate, start and reload and reports in doctor | Extends: the plugin enrols veth, bridge, translation and the local-address match, each in use only when the config needs it |
| Listener conflict check | `internal/component/config/listener.go` `ValidateListenerConflicts`, `DiscoverListenerServices` | Finds two services on one protocol, address and port, from schema lists marked `ze:listener` | Extends: a registration lets a plugin contribute endpoints computed from its own section, and an endpoint gains an upper port. A published port list does not have the `ze:listener` list shape |
| Plugin runtime state | `internal/core/statestore/permissions.go` `RegisterPluginKeys`; SDK `StatePut`, `StateGet` | A plugin persists under keys its package grants | Extends: one key pattern for address assignments |
| Interface VRF leaf | `internal/component/iface/yang/ze-iface-conf.yang` `leaf vrf` | Refused by the validator `unimplemented-vrf` | Does not apply as a mechanism: that validator runs only under listed roots. The same refusal is the plugin's verifier |
| LAN resolver | `internal/plugins/dnsresolver`; `ze-dns-resolver-conf.yang` `list listener`, `allow-from` | A forwarding resolver on operator-named addresses, bound with free-bind | Extends by configuration only: an operator may listen on a gateway address. No code names it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: the container root, delivered to the plugin as its own root, plus the `interface` and `firewall` roots it reads.
- The runtime child's two calls: "created, not yet running" and "exited or destroyed".

### Transformation Path
1. Verify (offline, start, reload): the plugin computes networks, addresses, links and tables from the candidate config and its stored assignments. It refuses address, name, port and mode faults, runs the backend gate against the interface backend, and runs the firewall parser, validator and backend verifier over the tables it would contribute.
2. Apply: the plugin stores new assignments, then sends its complete set of bridges (name, MTU, addresses) to the interface component and its complete set of tables to the firewall component. Each is recorded in the apply journal with the previous set as its undo. It emits the forwarding sysctl defaults.
3. The interface handler checks the caller's grant, registers the bridges and their addresses in the owned registries, runs the reconcile, and answers its outcome. The firewall handler checks the reservation, parses the body, registers the tables under the caller's name, reconciles, and answers.
4. Container start: at the runtime child's "created" call, the plugin asks the interface component for one link per attachment: the bridge, a slot number, the init process id with the inode number of its network namespace, the container-end name, hardware address, MTU, addresses and gateways.
5. The interface handler opens the namespace of that process, refuses when the inode differs, and has the backend create the pair: the host end named by `ComposeOwnedDeviceName`, marked with the owned alias, enslaved to the bridge, MTU set, up; the peer end created inside the namespace, then renamed, addressed, routed and set up through a socket bound to that namespace. It records the link and answers the host-end name.
6. The plugin returns success and the runtime releases the container. On any failure the plugin asks for the links it made to be removed and returns the failure.
7. Container exit: the kernel removes the pair with the namespace. The plugin asks the interface component to forget the links.
8. Reload: a removed container, attachment, port or network is absent from the next complete set, so the reconcile removes it.
9. Stop: an orderly daemon stop removes the firewall tables (existing) and the owned devices (this spec). A plugin process that ends without a stop is handled by the two cleanup registrations.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Container plugin to interface component | Engine operations in the interface namespace: replace the caller's owned bridges, create a link, remove a link | No |
| Container plugin to firewall component | One engine operation in the firewall namespace: replace the caller's tables, as a firewall config section | No |
| Container plugin to sysctl component | The existing default event on the EventBus | No |
| Container plugin to the listings | The text command dispatcher, read only | No |
| Runtime child to this child | Two calls inside the plugin, in one process | No |
| Interface backend to kernel | rtnetlink in the host namespace and through a socket bound to the container's namespace | No |

### Changed-Wire Census

**Search evidence:** searched `internal`, `cmd`, `pkg`, `test`, `docs` and `ai` for `ze:owned` and `ownedDeviceAliasPrefix` (writers and readers of the link alias); `internal`, `cmd` and `pkg` for `RegisterEngineOp(` (three callers, all in `internal/component/sysrib`); the same trees for `LinkSetNsFd`, `PeerNamespace`, `NewHandleAt` and `GetFromPid` (shipped code: `internal/core/routewatch/routewatch_linux.go` only, every other hit is a test or `internal/le`); `internal/component/firewall/model.go` for `Local`, `AddrType` and `Fib` (no hit, so no local-address match exists). Positive controls: the alias search finds `internal/plugins/iface/netlink/macvlan_linux.go` (writer) and `show_linux.go` (reader); the engine-op search finds `register_route_view.go`. Non-Go readers of the alias: four `test/vrrp/*.ci` files, which read macvlans only.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Interface owned-device requests (new) | writer | New SDK methods in `pkg/plugin/sdk`, called by the container plugin and the test plugin | No existing method: `RegisterEngineOp(` has no caller under `internal/component/iface` | New |
| Interface owned-device requests (new) | reader | New handlers registered from `internal/component/iface/cmd`, declared in `ze-iface-api.yang` | Same search | New |
| Firewall table contribution (new) | writer | New SDK method, called by the container plugin and the test plugin | No caller of `RegisterEngineOp(` under `internal/component/firewall` | New |
| Firewall table contribution (new) | reader | New handler in the firewall component, declared in a new `ze-firewall-api.yang` | Same search | New |
| Firewall config section | writer | The operator's config, delivered to the firewall engine; and now the contribution body | `config.go` `ParseFirewallConfig` reads the wrapped section | Preserved, plus one new match leaf. Every existing config parses as before |
| Firewall config section | reader | `internal/component/firewall/config.go` `ParseFirewallConfig`; `internal/plugins/firewall/nft/lower_linux.go`; `internal/plugins/firewall/vpp/verify.go` | Read at source | The new match is lowered by nft and refused by the VPP verifier in the same change |
| Link alias `ze:owned:<owner>` | writer | `internal/plugins/iface/netlink/macvlan_linux.go`; new: the owned bridge and link creators | Alias search | Preserved for macvlan. New kinds write the same form |
| Link alias `ze:owned:<owner>` | reader | `internal/plugins/iface/netlink/show_linux.go` (`InterfaceInfo.Alias`); `config_apply.go` `reconcileOwnedDevices`; `test/vrrp/*.ci` | Alias search | The orphan scan reads the kind too, so a macvlan verdict is unchanged. The `.ci` readers match macvlans and are re-run |
| Address assignment state key (new) | writer and reader | The container plugin through the SDK state calls | No container key in `pkg/zefs/keys.go` | New. Permanent once shipped |
| Sysctl default event | writer | New: the container plugin. Existing: the firewall engine's global options | `internal/component/sysctl/events/events.go` `EventDefault` | Preserved. A new source name |
| Sysctl default event | reader | The sysctl component | Same file | Unchanged |
| Container network and port nodes (new) | writer | The operator's config | No container root exists (umbrella census) | New, named by `spec-container-4-operator-surface.md` |
| Container network and port nodes (new) | reader | The container plugin's verifier and apply | Same | New |

### Integration Points
- `pluginserver.RegisterEngineOp` - the interface and firewall requests register here.
- `iface.Backend` - gains two methods; three backends answer them.
- `reconcileOwnedDevices` - handles the new kinds.
- `firewall.RegisterTables`, `ApplyAll` - called by the firewall handler under the caller's name.
- `config.ValidateBackendFeaturesJSON` - called by the plugin's verifier.
- The listener conflict check - gains contributed endpoints.
- `kernelcap.MustRegister` - four enrolments.
- The runtime child's two calls.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To show: the plugin holds no netlink or nftables call. `./le plugin boundary check` and the forked functional test |
| No unintended coupling (components stay isolated) | No | To show: `iface`, `firewall` and `config` contain no word of the container feature. A search for the plugin name in those trees |
| No duplicated functionality (extends existing, does not recreate) | No | To show: one owned-device registry, one table registry, one conflict check |
| Zero-copy preserved where applicable (refs, not copies) | No | Not a packet path. Requests are small values at config and start time |
| Registration over hardcoding, outbound | No | To show: the requests, grants, reservations, capabilities, codes and conflict contributor all register from `init()` |
| Registration over hardcoding, inbound | No | To show: `validatedSections`, the listener service discovery, the backend gate callers and `dangerousCalls` were searched. Only `dangerousCalls` gains entries, and they name interface functions, not the feature |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The vendored netlink library creates a veth whose peer end is in another namespace, in one request | `vendor/github.com/vishvananda/netlink/link.go` `Veth.PeerNamespace`, and its use in `link_linux.go` | The peer is created in the host namespace and moved, which needs a temporary name | `TestOwnedLinkIsCreatedInsideNamespace` | unvalidated |
| A-2 | A netlink socket bound to the container's namespace renames, addresses, routes and raises the peer end from the daemon | `vendor/github.com/vishvananda/netlink/handle_linux.go` `NewHandleAt`; `internal/core/routewatch/routewatch_linux.go` uses the same library | A worker thread enters the namespace under the `kernelcap` thread rule | `TestOwnedLinkIsCreatedInsideNamespace` | unvalidated |
| A-3 | The runtime child gives the four seam items, and the loopback device is up before the "created" call | Umbrella key insights; the runtime research report, section 5 (a report, not read at source by this spec) | The seam is renegotiated in both specs | The runtime child's spec text, and `container-network-reach-gateway.ci` | unvalidated |
| A-4 | A configure-safe engine operation can be called from the plugin's configure stage, and a plugin with `Dependencies` on `interface` and `firewall` configures after both | `register_engine_op.go` `EngineOperationConfigureSafe`; `ai/doors/plugin-infra.md` traps | The first declaration moves to the all-plugins-ready callback, and containers start after it | `container-network-wiring-external.ci` | unvalidated |
| A-5 | The kernel deletes the host end when the namespace holding the peer end is destroyed | Kernel behaviour, not read | The plugin's "forget the links" request must delete the host end, which it does anyway | `TestHostEndVanishesWithNamespace` | unvalidated |
| A-6 | The generation cleanup runs for an internal plugin and a forked one, after its requests drain | `register_engine_op.go` `ProcessGenerationCleanup` comment | A backstop is missing in one mode, and the next reconcile's orphan scan is the only cleanup | `container-network-plugin-ends.ci` in both modes | unvalidated |
| A-7 | The nft `fib` expression is available in the ip and ip6 families on the runtime kernel | `gokrazy/kernel/kernel.config` requests `NFT_FIB_IPV4` and `NFT_FIB_IPV6`; neither is in a require file; no arm64 config was read | The match needs another lowering, or the options need a kernel change | The enrolment's probe, and `TestLocalDestinationDNATLeavesTransitAlone` in the QEMU guest | unvalidated |
| A-8 | Bridged traffic between two containers of one network crosses the forward hook, because bridge netfilter is built in | The kernel research report: `BRIDGE_NETFILTER=y`. The sysctl default was not read | The guide's sentence about same-network traffic changes. No design change | `container-network-same-network-filter.ci` records which it is | unvalidated |
| A-9 | Two enrolments may name different subsystems for kernel options no one enrolled before, and the enrolled-symbol test accepts a line added to `runtime.require` | `kernelcap.go` `Capability`; `TestEnrolledKernelSymbolsAreInRuntimeRequire` (named in the kernel research, not read) | The options are enrolled by the interface and nft backends instead, with a registered in-use predicate | Reading `MustRegister` and running that test | unvalidated |
| A-10 | The runtime child has a test route that starts a container from a root folder in the test tree, with no registry | Not stated anywhere yet | The functional tests that need a container wait for the image store child | The runtime child's spec text | unvalidated |
| A-11 | The node names are fixed by the operator-surface design before this child's Phase 4 | The umbrella child table | Phase 4 blocks | That spec reaching `ready` | unvalidated |
| A-12 | The interface engine's stop path can delete owned devices before it closes its backend | `internal/component/iface/register.go`: the stop path runs after `p.Run` returns and ends in `CloseBackend` | The deletion needs another ordered actor | `container-network-daemon-stop.ci`, and a VRRP stop test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A published port captures transit traffic, or a Ze listener such as SSH | A connection through the router to another host's same port lands in the container | The local-address match on every translation term; the conflict check against Ze's listeners |
| R-2 | The orphan scan deletes a bridge or veth another owner made | A foreign bridge disappears after a commit | Two facts are required, an owned kind and the alias. A name held by an unmarked device fails closed. `TestOrphanScanKeepsForeignBridge` |
| R-3 | The init process id is reused between the plugin's request and the handler, and the link lands in another process's namespace | None without a check | The request carries the namespace inode number. The handler opens the namespace once, compares, and uses that descriptor |
| R-4 | A direct Go call into the registries works internal and does nothing forked | The forked test shows no bridge | The plugin has no such call. The in-process register functions of the owned-device registry join `dangerousCalls`. Every functional test of the wiring runs in both modes |
| R-5 | A failed attach leaves a host end with no container | A marked veth with a dead peer after a failed start | The plugin removes what it made before it returns the failure, and the orphan scan removes a marked link no live record names |
| R-6 | A third-party plugin uses the requests to make devices or tables | A device with an unknown owner | A request is accepted only from a caller whose package made a grant or a reservation at `init()`. A plugin outside the binary has none |
| R-7 | Every reconcile re-applies all owners' tables, and a container start would pay it | Slow container start under a large ruleset | Tables follow the config, so the firewall is touched at config changes only |
| R-8 | An entrypoint binds its IPv6 address while duplicate address detection holds it tentative | A bind error in the first second | The address is installed with detection off |
| R-9 | The operator's forward chain drops the published traffic and the port looks broken | A translation term whose counter rises while no connection completes | The doctor check `container-filter-drops-published`, and the guide |
| R-10 | After a daemon crash, the first reconcile deletes the links of containers that are still running | Running containers lose their network at restart | Stated in the seam: the runtime child stops or re-adopts such a container, and a re-adopted one is attached again |
| R-11 | A reload rollback leaves the new bridge or tables in place | State of a refused commit in the kernel | Each declaration is a journal step whose undo is the previous complete set |
| R-12 | An address is released and at once assigned to another container while neighbours hold the old hardware address | Brief loss after a container swap | The derived hardware address changes with the container name, and the kernel refreshes on the first reply. Recorded, no further action |
| R-13 | A connection made before a port is unpublished keeps flowing | An old session survives the commit | Recorded as a permitted survivor: the kernel keeps a tracked flow |
| R-14 | The assignment key becomes a field contract in a shape that cannot grow | A later change needs a migration | One value per network, a map from container name to its addresses and slot, written whole |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A container without connectivity; a published port that captures traffic meant for the router or for a host behind it; a foreign bridge or table deleted; VRRP macvlans left or removed wrongly at stop |
| How is it reverted? | Revert the commits. Kernel state is removed by an orderly stop, and marked leftovers are removed by the next start. The assignment key stays in the store and is ignored |
| Who else touches this path? | VRRP (owned devices), every firewall table owner, `spec-container-1-runtime.md`, `spec-container-4-operator-surface.md`, `plan/post-release/features/spec-vrf-0-umbrella.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A forked plugin declares an owned bridge | → | The interface engine operation, the owned registry, the netlink backend | `test/plugin/container-network-wiring-external.ci` |
| An internal plugin declares an owned bridge | → | The same handler through the direct path | `test/plugin/container-network-wiring-internal.ci` |
| A forked plugin asks for a link into a namespace it holds | → | The link handler and the netlink link creator | `test/plugin/container-link-wiring-external.ci` |
| A forked plugin contributes a reserved table | → | The firewall engine operation, `RegisterTables`, `ApplyAll` | `test/plugin/container-table-wiring-external.ci` |
| A config with a network and an attached container is committed | → | The plugin's verify and apply, the runtime seam, the link | `test/plugin/container-network-reach-gateway.ci` |
| A config publishes a port | → | The plugin's table builder and the firewall contribution | `test/plugin/container-port-published.ci` |
| `ze config validate` on a config with a host container and a port | → | The plugin's in-process verifier | `test/plugin/container-host-network-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config declares a container network and is committed | A bridge named `zcn-` plus the network name exists, is up, has the configured MTU and holds the gateway address of each prefix. The interface listing shows it with the owner mark | Every interface the config or another owner made is unchanged, because the reconcile acts only on marked devices of owned kinds |
| AC-2 | A container attached to that network starts (umbrella AC-8) | Before its entrypoint runs, the container holds one link with the configured name, address, MTU and a default route through the gateway, and it reaches the gateway address. The host end is a member of the bridge and is listed as an interface with the owner mark | The loopback device stays as the runtime made it, because this child never touches it |
| AC-3 | A container with no configured address is started, restarted, the config is reloaded, and the device reboots | It holds the same assigned address each time, the lowest one that was free when it was first assigned | An address is released when its container or attachment leaves the config, because nothing uses it then |
| AC-4 | A network has an IPv4 and an IPv6 prefix | The container holds one address and one default route per family, and reaches both gateway addresses at entrypoint start | A network with one prefix gives one family, because the operator configured one |
| AC-5 | The firewall config has a term that matches the network's bridge as input interface and drops (umbrella AC-8) | Routed traffic from the container is dropped by that term and its counter rises. Removing the term lets the traffic pass | Traffic the firewall config permits passes. Every nft table present is one Ze owns, because nothing else creates packet-filter state |
| AC-6 | A container publishes a TCP port (umbrella AC-9) | A connection to an address of the router on that port reaches the container. The ruleset listing shows the translation term in the plugin's reserved table, with its counter | A connection through the router to another host's same port is forwarded untranslated, because only traffic addressed to the router is published. A port no container publishes stays closed |
| AC-7 | A network has outbound translation on, and a container connects to a host beyond the router | The host sees the router's address as the source. Traffic between two containers of the network keeps its container source address | A network with outbound translation off is forwarded untranslated, because the operator routes its prefix |
| AC-8 | A reload removes a container that had a link and a published port | The other containers keep their links, addresses and translations and stay running, and the listings show only them: the removed container's link, bridge membership and translation term are gone | A bridge, a veth and an nft table made outside Ze, present before the reload, are unchanged, because they carry no Ze mark. A connection tracked before the removal may finish, because the kernel keeps a tracked flow |
| AC-9 | The daemon stops in an orderly way | The daemon exits with success, and a kernel readback after it shows every device and table that carries no Ze mark exactly as it was before the start. Every device with the owned alias and both `ze_container` tables have been removed | A device or table without Ze's mark remains, because Ze does not own it. With `flush-on-shutdown false` the tables remain, because the operator asked for that |
| AC-10 | The daemon is killed, a container is removed from the config, and the daemon starts | The remaining networks are re-asserted and their containers attach again. The removed container's leftover link and translation are removed at start | Unmarked devices and tables remain, because the scan requires the mark |
| AC-11 | A container is configured with the host network mode | It runs in the host network namespace: it sees the host's interfaces and binds the host's addresses. Ze creates no link and no translation for it | A host container may set its own name servers, because a resolver file is not a setting of the namespace |
| AC-12 | A host or none container sets a published port, an attachment or a network setting | Validate, start and reload refuse the config with a message that names the node and the two fixes: remove it or change the mode. On a reload the running containers keep running | The same config without that node is accepted, because the mode alone is valid |
| AC-13 | A container is configured with no network | It holds a loopback device that is up, and that is its only link | No translation exists for it, because it has no address |
| AC-14 | A network sets the VRF node | Validate, start and reload refuse with a message that says VRF support is not implemented and names the node to remove | A network without the node is accepted |
| AC-15 | The interface backend is `vpp` and the config has a container network, or the firewall backend is `vpp` and the config needs a translation | Validate, start and reload refuse, naming the container node and the backend leaf | A config with only host or none containers is accepted on those backends, because nothing needs a link or a table |
| AC-16 | The container plugin runs as a separate process | AC-1, AC-2, AC-6 and AC-8 hold exactly as when it runs internal | None differ between the two modes, because both use the same requests |
| AC-17 | A container starts with no name server configured anywhere on it | Its resolver file lists the router's resolver addresses of that moment, without loopback addresses. With name servers on the container, the file lists those; with none on the container and some on its first network, it lists the network's | A container whose list is empty still starts, and doctor reports it, because a missing resolver is not a start failure |
| AC-18 | Two containers publish the same protocol, address and port, or one publishes a port a Ze listener uses | Validate, start and reload refuse, naming both parties and the node to change | Two publishers on different router addresses are accepted, because they do not overlap |
| AC-19 | Two attachments set the same address, an address is outside the prefix, two networks overlap, or a network has fewer free addresses than attachments | Validate, start and reload refuse, naming the node and the value | A config where each address is unique and inside its prefix is accepted, including one that fills the prefix exactly |
| AC-20 | A config uses a container network on a kernel without veth, bridge, translation or the local-address match | Validate, start and reload refuse, naming the option. Doctor reports the same code | A config with no container network is accepted on that kernel, because nothing needs the option |
| AC-21 | Each problem class in the "Problem classes" table is provoked | `ze doctor` reports it with its code, its cause and the command or node that fixes it | A healthy network reports no finding, because a check that always reports is noise |
| AC-22 | VRRP is configured and the daemon stops in an orderly way | Its macvlans are removed by the interface engine's stop path, and the stop completes with success | Unmarked macvlans remain, because the stop path requires the mark |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Declares a network and attaches a container | config → plugin verify → interface request → bridge; container start → link request → link in the namespace | `container-network-reach-gateway.ci` |
| 2 | Publishes a container's port to the outside | config → table builder → firewall request → nft table → connection reaches the container | `container-port-published.ci` |
| 3 | Writes a firewall term for a container network | firewall config → nft → match on the bridge device | `container-network-firewall-term.ci` |
| 4 | Removes a container | config → next complete set → reconcile removes link and term | `container-network-reload-removes.ci` |
| 5 | Runs a container on the host network | config → mode → no link, no term; ports refused | `container-host-network.ci`, `container-host-network-refused.ci` |
| 6 | Asks why a container has no network | `ze doctor` → the plugin's runtime checks → code and fix | `doctor-container-network.ci` |

## 🧪 TDD Test Plan

The container plugin's path is `internal/plugins/container` as the runtime child
creates it; this child's package is `network` under it.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGatewayIsNetworkAddressPlusOne` | `internal/plugins/container/network/address_test.go` | The gateway in each family | |
| `TestAssignLowestFreeAddress` | same | Assignment skips the gateway and held addresses | |
| `TestAssignmentIsKeptAcrossRestart` | same | A stored assignment is reused | |
| `TestAssignmentReleasedWhenAttachmentLeaves` | same | Release on removal | |
| `TestVerifyRefusesDuplicateAddress` | `internal/plugins/container/network/verify_test.go` | Two attachments, one address | |
| `TestVerifyRefusesAddressOutsidePrefix` | same | Outside, network, broadcast and gateway addresses | |
| `TestVerifyRefusesOverlappingNetworks` | same | Two prefixes that overlap | |
| `TestVerifyRefusesExhaustedPrefix` | same | More attachments than addresses | |
| `TestVerifyRefusesHostModeNetworkSettings` | same | Port, attachment or setting on host and none | |
| `TestVerifyRefusesVRF` | same | The VRF node | |
| `TestVerifyRefusesIPv6BelowMinimumMTU` | same | MTU under 1280 with an IPv6 prefix | |
| `TestVerifyRefusesBridgeNameInInterfaceConfig` | same | The derived name is declared in `interface` | |
| `TestBridgeNameIsRefusedNotTruncated` | `internal/plugins/container/network/name_test.go` | A 12-character network name | |
| `TestHardwareAddressIsStableAndUnicast` | same | The derived address | |
| `TestTablesForPublishedPort` | `internal/plugins/container/network/tables_test.go` | The prerouting and output terms of one port | |
| `TestTablesForOutboundTranslation` | same | The jump, return and masquerade terms | |
| `TestTablesFollowConfigNotRunState` | same | A stopped, enabled container has its term; a disabled one has none | |
| `TestPublishedPortEndpoints` | `internal/plugins/container/network/listeners_test.go` | The endpoints the conflict check receives | |
| `TestResolverPrecedence` | `internal/plugins/container/network/resolver_test.go` | Container, then network, then router list without loopback | |
| `TestOwnedBridgeRequestRefusesCallerWithoutGrant` | `internal/component/iface/device_owner_test.go` | The grant check | |
| `TestReconcileCreatesOwnedBridgeWithAlias` | `internal/component/iface/config_apply_owned_test.go` | Create and mark, with a scripted backend | |
| `TestOrphanScanKeepsForeignBridge` | same | An unmarked bridge and a marked non-owned kind survive | |
| `TestOrphanScanDeletesLinkWithNoRecord` | same | A marked link no record names | |
| `TestLinkRequestRefusesNamespaceMismatch` | `internal/component/iface/cmd/owned_link_test.go` | The inode check | |
| `TestStopPathDeletesOwnedDevices` | `internal/component/iface/register_stop_test.go` | The ordered stop deletion | |
| `TestVPPBackendRefusesOwnedLink` | `internal/plugins/iface/vpp/owned_test.go` | Exact or refuse | |
| `TestParseLocalDestinationMatch` | `internal/component/firewall/config_local_test.go` | The new match parses | |
| `TestContributionRefusesUnreservedTable` | `internal/component/firewall/contribution_test.go` | Only the caller's reserved names | |
| `TestGenerationCleanupWithdrawsOwner` | same | A gone process's tables are withdrawn | |
| `TestLowerLocalDestination` | `internal/plugins/firewall/nft/lower_local_test.go` | The nft expressions of the match | |
| `TestVPPVerifierRefusesLocalDestination` | `internal/plugins/firewall/vpp/verify_local_test.go` | The VPP verdict on the match | |
| `TestListenerContributorJoinsConflictCheck` | `internal/component/config/listener_contributor_test.go` | A contributed endpoint conflicts with a service | |
| `TestPortRangeOverlapIsAConflict` | same | Ranges that overlap | |
| `TestOwnedLinkIsCreatedInsideNamespace` | `internal/plugins/iface/netlink/owned_link_integration_linux_test.go` | Kernel: the peer end, its name, address, route and state inside a namespace | |
| `TestHostEndVanishesWithNamespace` | same | Kernel: the pair is gone when the namespace is | |
| `TestOwnedBridgeIsAdoptedAfterRestart` | same | Kernel: a marked bridge that matches is kept, a drifted one is re-made | |
| `TestLocalDestinationDNATLeavesTransitAlone` | `internal/plugins/firewall/nft/local_dnat_integration_linux_test.go` | Kernel: two namespaces and a router namespace; only router-addressed traffic is translated, with a positive control | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Network MTU | 68-16000 | 16000 | 67 | 16001 |
| Network MTU with an IPv6 prefix | 1280-16000 | 16000 | 1279 | 16001 |
| IPv4 prefix length | 1-30 | 30 | 0 | 31 |
| IPv6 prefix length | 1-126 | 126 | 0 | 127 |
| Published port | 1-65535 | 65535 | 0 | 65536 |
| Port range size (router and container sides equal) | 1-65535 | 65535 | 0 | N/A |
| Network name length | 1-11 | 11 | 0 | 12 |

### Functional Tests
Every row carries `option=needs-linux` unless marked "parse only". Each is
drafted under `test/draft/plugin/` and promoted.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `container-network-wiring-external` | `test/plugin/container-network-wiring-external.ci` | A forked test plugin's bridge appears in the kernel with the owner mark | |
| `container-network-wiring-internal` | `test/plugin/container-network-wiring-internal.ci` | The same, internal | |
| `container-link-wiring-external` | `test/plugin/container-link-wiring-external.ci` | A forked test plugin's link has its peer end configured in a namespace | |
| `container-table-wiring-external` | `test/plugin/container-table-wiring-external.ci` | A forked test plugin's table is in the ruleset listing | |
| `container-network-reach-gateway` | `test/plugin/container-network-reach-gateway.ci` | AC-1, AC-2: an entrypoint that connects to the gateway at once succeeds | |
| `container-network-forked` | `test/plugin/container-network-forked.ci` | AC-16: the same with the plugin forked | |
| `container-network-address-stable` | `test/plugin/container-network-address-stable.ci` | AC-3 | |
| `container-network-ipv6` | `test/plugin/container-network-ipv6.ci` | AC-4 | |
| `container-network-firewall-term` | `test/plugin/container-network-firewall-term.ci` | AC-5, with the term's counter as the positive control | |
| `container-network-same-network-filter` | `test/plugin/container-network-same-network-filter.ci` | A-8: whether a forward term sees traffic between two containers of one network | |
| `container-port-published` | `test/plugin/container-port-published.ci` | AC-6: a connection reaches the container and the listing shows the term | |
| `container-port-transit-untouched` | `test/plugin/container-port-transit-untouched.ci` | AC-6 survivor: transit to the same port is not translated | |
| `container-network-outbound` | `test/plugin/container-network-outbound.ci` | AC-7 | |
| `container-network-reload-removes` | `test/plugin/container-network-reload-removes.ci` | AC-8, with a foreign bridge and table that must survive | |
| `container-network-daemon-stop` | `test/plugin/container-network-daemon-stop.ci` | AC-9, AC-22 | |
| `container-network-crash-leftovers` | `test/plugin/container-network-crash-leftovers.ci` | AC-10 | |
| `container-host-network` | `test/plugin/container-host-network.ci` | AC-11 | |
| `container-host-network-refused` | `test/plugin/container-host-network-refused.ci` | AC-12, parse only | |
| `container-none-network` | `test/plugin/container-none-network.ci` | AC-13 | |
| `container-network-vrf-refused` | `test/plugin/container-network-vrf-refused.ci` | AC-14, parse only | |
| `container-network-vpp-refused` | `test/plugin/container-network-vpp-refused.ci` | AC-15, parse only | |
| `container-resolver-list` | `test/plugin/container-resolver-list.ci` | AC-17 | |
| `container-resolver-on-gateway` | `test/plugin/container-resolver-on-gateway.ci` | The LAN resolver listening on a gateway address answers a container | |
| `container-port-listener-conflict` | `test/plugin/container-port-listener-conflict.ci` | AC-18, parse only | |
| `container-address-refused` | `test/plugin/container-address-refused.ci` | AC-19, parse only | |
| `doctor-container-network` | `test/plugin/doctor-container-network.ci` | AC-21: one provoked class per step of the sequence, each with its code | |
| `container-network-plugin-ends` | `test/plugin/container-network-plugin-ends.ci` | The plugin process ends: its devices and tables are withdrawn; both modes | |

### Lifecycle Scenarios (`ai/rules/testing.md`)
| Scenario | Reached? | Proof or reason |
|----------|----------|-----------------|
| A plugin's process ends after it declared | Yes | `container-network-plugin-ends.ci`, internal and forked: the cleanup registrations withdraw its state |
| A plugin exits during the stop drain or during teardown | Yes | `container-network-daemon-stop.ci`: the two ordered stop paths remove the state whatever the plugin did |
| A plugin crashes during verify, crashes during apply, misses its deadline, or reports broken | Yes | `container-network-reload-removes.ci` has a step where the apply fails: the journal restores the previous complete sets |
| A plugin fails a startup stage after it declared | Yes | Covered by the first row's test: a stage failure ends the process generation |
| A restarting plugin passes its restart bound | No | This spec adds no restart rule; the runtime child owns the plugin's failure policy |
| The `respawn` leaf disagrees with the declaration | No | Same reason |
| A signal arrives during the stop | No | This spec adds no signal handling. The stop paths it extends run inside the existing ordered teardown |
| A reload or commit arrives after shutdown began | No | The requests are made from config callbacks, which the existing gate refuses then |
| A reload's context ends during a startup handshake | No | The configure-safe requests finish within the stage's context or fail the stage, which is the first row |
| A plugin dispatches another plugin's command at startup | No | The read-only listings are dispatched from doctor checks at run time only |

## Files to Modify
- `internal/component/iface/backend.go` - two `Backend` methods: create an owned bridge, create a link with its peer end in a namespace
- `internal/component/iface/device_owner.go` - owned bridge and link records, the `init()` grant of a name prefix to an owner, the gauge
- `internal/component/iface/config_apply.go` - `reconcileOwnedDevices` handles the new kinds
- `internal/component/iface/register.go` - the stop path deletes marked devices before `CloseBackend`
- `internal/component/iface/yang/ze-iface-api.yang` - the three requests
- `internal/plugins/iface/vpp/ifacevpp.go`, and the non-Linux backend - refuse both new methods
- `internal/component/firewall/model.go`, `config.go`, `validate.go`, `yang/ze-firewall-conf.yang` - the local-address match
- `internal/plugins/firewall/nft/lower_linux.go` - its lowering
- `internal/plugins/firewall/vpp/verify.go` - its refusal
- `internal/component/config/listener.go` - contributed endpoints and the upper port
- `internal/le/plugin/boundary/pluginboundary.go` - the in-process owned-device register functions join `dangerousCalls`
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - the options this child enrols that are not pinned
- `feature-gates.txt` - the `network` package under the runtime child's tag
- `docs/features/interfaces.md` - owned bridges and links, the stop path
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - the producer row, and the forked producer route
- `docs/architecture/core-design.md` sections 14 and 14b - the owned kinds and the contribution request
- `docs/architecture/api/process-protocol.md` - the new engine operations
- `docs/guide/firewall.md` - the local-address match
- `docs/architecture/config/syntax.md` - the listener conflict check takes contributed endpoints
- `docs/architecture/firewall/fw-6-firewall-vpp.md` - the VPP verifier refuses the local-address match
- `ai/doors/forwarding.md` - the registry is reachable from a forked producer through the request

## Files to Create
- `internal/component/iface/cmd/owned_devices.go` - registers the interface requests and the generation cleanup
- `internal/plugins/iface/netlink/owned_bridge_linux.go`, `owned_link_linux.go` - the kernel side
- `internal/component/firewall/contribution.go`, `register_contribution.go` - the firewall request and cleanup
- `internal/component/firewall/yang/ze-firewall-api.yang` - the request
- `pkg/plugin/rpc/owned_devices.go`, `pkg/plugin/rpc/firewall_tables.go` - the value types and method names
- `pkg/plugin/sdk/sdk_owned_devices.go`, `pkg/plugin/sdk/sdk_firewall_tables.go` - the SDK methods
- `internal/plugins/container/network/` - `verify.go`, `address.go`, `name.go`, `tables.go`, `resolver.go`, `listeners.go`, `attach.go`, `register_tables.go`, `register_kernelcap_linux.go`, `kernelcap_linux.go`, `doctor.go`, and the tests named above
- `internal/test/plugins/fakeowner/` - a test plugin that declares a bridge, a link into a namespace it holds, and a table
- The `.ci` files in the Functional Tests table
- `docs/architecture/container/network.md` - the design page: ownership marks, the two requests, the seam, the tables

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/iface/yang/ze-iface-api.yang`, `internal/component/firewall/yang/ze-firewall-api.yang`, `ze-firewall-conf.yang`; the container nodes go in the plugin's schema as `spec-container-4-operator-surface.md` names them |
| YANG validation constraints | Yes | The ranges in the Boundary Tests table, as native `range` and `length`; addresses and prefixes from `ze-types.yang` |
| YANG custom validators | No | No `ze:validate` is added: a container root is not in `validatedSections`, so every cross-node rule is the plugin's verifier |
| CLI commands/flags | N-A | No command is added here. The listings are `spec-container-4-operator-surface.md` |
| CLI grammar (keyword before value) | N-A | No command is added here |
| Editor autocomplete | Yes | Automatic for the enumerations; the attachment's network name is a reference to a configured network |
| Functional test for new RPC/API | Yes | The four wiring `.ci` files |
| Pipe completeness | N-A | No command is added here |
| Env var registration | No | No tunable is added |
| Doctor check for runtime dependencies | Yes | `internal/plugins/container/network/doctor.go`, `kernelcap_linux.go`; netlink and nftables use through the two components |
| Doctor code per problem class | Yes | The "Problem classes" table: each code, its phase and its fix, registered from the plugin's package |
| Prometheus counters/metrics | Yes | `ze_iface_owned_devices{owner}` already counts owned devices and now counts the new kinds. No new series |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The container feature file under `features/`, created by the runtime child: add networks and published ports |
| 2 | Config syntax changed? | Yes | `docs/guide/firewall.md` for the local-address match; the container nodes are documented by `spec-container-4-operator-surface.md` |
| 3 | CLI command added/changed? | No | No command is added here |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`: the interface and firewall engine operations |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: the container plugin's network section, with the runtime child's entry |
| 6 | Has a user guide page? | Yes | The container guide page, created by `spec-container-4-operator-surface.md`: networks, ports, the filter rule an operator writes, name resolution |
| 7 | Wire format changed? | No | No protocol wire |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/handlers.md`: the new SDK methods and who may call them |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC behavior |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the `fakeowner` test plugin |
| 11 | Affects daemon comparison? | No | Not a routing daemon feature |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` sections 14 and 14b; new `docs/architecture/container/network.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | An existing gauge counts more kinds; its page text is checked under row 16 |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/appliance.md` "Runtime Kernel Requirements" and `docs/architecture/appliance/kernel-profiles.md`: the enrolled options |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md`, `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/architecture/core-design.md`, `docs/architecture/iface/logical-name-resolution.md` (named as unaffected: no resolver change). Declared by the changed files' `// Design:` headers: `docs/architecture/config/syntax.md` (from `listener.go`: add the contributed endpoints of the listener conflict check), `docs/architecture/firewall/fw-6-firewall-vpp.md` (from the VPP verifier: add the refusal of the local-address match), `docs/research/vpp-deployment-reference.md` (from the VPP interface backend: named as unaffected, a research page, and the two refused methods are recorded in `docs/features/interfaces.md`). Re-derive with `./le spec citation anchors spec plan/post-release/features/spec-container-3-network.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/firewall.md` examples are checked against the new match |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the two components declare their requests, the SDK gains the methods, and a test plugin calls them
   - Tests: the four `container-*-wiring-*.ci`, `TestOwnedBridgeRequestRefusesCallerWithoutGrant`, `TestContributionRefusesUnreservedTable`
   - Files: `internal/component/iface/cmd/owned_devices.go`, `internal/component/firewall/register_contribution.go`, the two API modules, `pkg/plugin/rpc`, `pkg/plugin/sdk`, `internal/test/plugins/fakeowner/`
   - Verify: a forked and an internal test plugin reach both handlers. The wiring tests fail because the handlers refuse every request
2. **Phase: Owned bridge and link** -- the interface side
   - Tests: the `iface` unit tests, the three netlink integration tests, `TestVPPBackendRefusesOwnedLink`, `TestStopPathDeletesOwnedDevices`
   - Files: `backend.go`, `device_owner.go`, `config_apply.go`, `register.go`, the netlink and VPP backends
   - Verify: tests fail → implement → the bridge and link wiring tests pass
3. **Phase: Firewall contribution and the local-address match**
   - Tests: the firewall unit tests, `TestLowerLocalDestination`, `TestLocalDestinationDNATLeavesTransitAlone`
   - Files: the firewall component, the nft and VPP backends, `ze-firewall-conf.yang`
   - Verify: tests fail → implement → the table wiring test passes. Run `TestRepositoryConfigsValidate`, because the schema changed
4. **Phase: The plugin's network package** -- verify, addresses, tables, the seam
   - Tests: the `network` unit tests, then `container-network-reach-gateway`, `container-port-published` and the other container `.ci` files
   - Files: `internal/plugins/container/network/`
   - Verify: needs A-10 and A-11. Tests fail → implement → pass in both plugin modes
5. **Phase: Port conflicts**
   - Tests: the two listener tests, `container-port-listener-conflict`
   - Files: `internal/component/config/listener.go`, `listeners.go`
   - Verify: the existing listener conflict tests stay green
6. **Phase: Kernel enrolment and doctor**
   - Tests: `doctor-container-network`, the enrolled-symbol test
   - Files: `kernelcap_linux.go`, `doctor.go`, the two kernel files
   - Verify: each class in the table is provoked and reported
7. **Phase: Pages** -- each page is edited in the phase that makes it wrong; this phase checks the list and runs `./le doc check all`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Every translation term carries the local-address match. The orphan scan requires both the kind and the alias. The namespace inode is compared before any link is made |
| Naming | JSON keys of the requests are kebab-case. Device names come from `ComposeOwnedDeviceName` or the `zcn-` rule and are refused when too long |
| Data flow | The plugin holds no netlink or nftables call and no call into `iface` or `firewall` state. `iface`, `firewall` and `config` hold no word of the container feature |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line names a code of the "Problem classes" table, and each check carries `Fix` at the phase that holds its state |
| Rule: `ai/rules/plugins.md` | The doctor checks over live state are runtime checks answered by the plugin, declared in its Stage 1 registration |
| Rule: `ai/rules/testing.md` | Each reachable lifecycle scenario runs internal and forked |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The interface and firewall requests are registered | `./le doc yang-contract command-contract`, and the four wiring `.ci` files |
| A forked plugin gets the same result as an internal one | `./le test functional plugin` rows `container-network-forked`, `container-network-wiring-external` |
| No engine or config code names the feature | A search for the plugin name under `internal/component/iface`, `internal/component/firewall`, `internal/component/config` finds nothing |
| Foreign state survives | `container-network-reload-removes.ci` |
| The kernel options are pinned and enrolled | `TestEnrolledKernelSymbolsAreInRuntimeRequire` |
| Linux-only code is proven on Ze's kernel | `./le test qemu all`, and the `needs-linux` rows through the QEMU functional route |
| The pages match the code | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Who may call the requests | A caller with no `init()` grant or reservation is refused before any state changes. The owner is the authenticated process, never a name in the request body |
| Namespace target | The process id is paired with the namespace inode number, and the handler uses one opened descriptor for the check and the create |
| Request bounds | The number of bridges, links, addresses and tables per request is bounded, and each name, address and prefix is validated before use |
| Capture of router services | A published port that equals a Ze listener is refused. Only router-addressed traffic is translated |
| Foreign state | No delete without both ownership facts. A held name fails closed |
| Container reach | A container cannot change the host end or the bridge: they are in the host namespace. The container end is the container's to change only when the operator grants it the capability |
| Error text | A refusal names the node and the fix, and does not echo an unbounded value |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The requests are owner-registered engine operations | (a) EventBus events. (b) Operator commands sent through the text dispatcher. (c) The plugin writes netlink and nftables itself | (a) has no answer, and the runtime must know the link exists before it releases the container. (b) makes "put a link into any process" an operator command. (c) is state installed beside Ze's interface and firewall owners, which O-7 forbids |
| A network is an owned bridge, extending the owned-device registry | (a) The plugin inserts entries into the `interface` config. (b) One routed point-to-point link per container, with no bridge | (a) has no mechanism, and the config would show nodes the operator did not write. (b) gives a per-container interface a firewall term can match and sends all traffic through the forward hook, but containers of one network lose their shared segment, which the reference operator model has |
| A link is requested per container start, not declared | Declaring links in the complete set with the bridges | A link dies with its namespace, so a reconcile would re-make a link whose container is gone |
| The peer end is created inside the namespace | Create in the host namespace, then move | One request, no temporary name in the host namespace, no moment where the peer is visible on the host |
| Contributed tables travel as a firewall config section | A typed copy of the 45 expression types in the public RPC package | The section form already has a schema, a parser, a validator and a verifier, and a plugin in any language can produce it |
| Translation follows the config | Add and remove the term at each container start and stop | Every reconcile re-applies all owners' tables under one lock. A stopped container's port then answers "unreachable", never a service of the router |
| The local-address match is added to the firewall model | Require a router address on every published port | A dynamic router address could not be published, and a forgotten address would capture transit traffic |
| A published port does not open the filter | The plugin adds an accept term | A drop in another base chain is final, and the plugin cannot write the operator's tables |
| The VRF node is refused by the plugin's verifier | `ze:validate "unimplemented-vrf"` on the node | That validator runs only under roots in a literal list in config code, and adding the container root there has config code name the feature |
| Marked devices are deleted by the interface engine's stop path | Each owner releases at its own stop | Plugins stop together with no order, so an owner's release races the backend close. The firewall solved the same race the same way |
| Assignments are stored | Assign the lowest free address at each start | A restart in another order would move a container's address |
| Ze does not answer container names | The plugin contributes names to the LAN resolver through a new registration | The resolver forwards only and has no local data today. Addresses are stable, so a name service is a separate feature |
| Resolver default is the router's list at start | (a) No default. (b) Always the gateway address | (a) leaves a container without names on a box that learned its resolvers. (b) needs the LAN resolver, which the engine must not configure for the operator |
| Outbound translation defaults to on | Off unless set | The reference operator model translates by default, and a private prefix is unreachable from outside without it. Put to the owner |

## Known Limitations
- A container network cannot be placed in a VRF. The node is refused until `plan/post-release/features/spec-vrf-0-umbrella.md` ships, and that spec owes the container network as one of its consumers.
- A firewall term cannot match one container by its host end for routed traffic: the input interface of that traffic is the network's bridge. A term matches the network by its bridge, and one container by its address, which is stable.
- A container cannot reach its own published port through the router's address from inside its network. Containers of one network reach each other by address.
- Ze answers no container names.
- The VPP interface backend cannot carry a container network, and the VPP firewall backend cannot carry the translation tables.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
