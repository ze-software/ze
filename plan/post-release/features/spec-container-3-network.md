# Spec: container-3-network

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature. The owner ruled (umbrella O-3, O-7) that containers ship after the first release and that their networks and published ports are container config realised through Ze's interface and firewall mechanisms. |
| RFC gaps | - |
| Depends | spec-container-1-runtime |
| Phase | 3/7 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-17 |

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

The owner rulings are fixed by the umbrella: O-7, and the rulings of 2026-11-16
its "Owner decisions" table records (a bridge per network; outbound translation
on by default; a published port must also be allowed by the operator's firewall
policy; no container names over DNS at first). This spec refines umbrella AC-8
and AC-9, and umbrella AC-14 rests on it (see "The owner's closing test").

### Config words this child implements

`spec-container-4-operator-surface.md` holds the one table of config words and
command words (tables S-1, S-2, C-1, V-1). Its design is settled before any
sibling's wiring phase, and this child uses its words and designs none. The
rows below are that child's nodes that land with this child.

| Node (that child's word) | Under | Rule this child applies |
|--------------------------|-------|-------------------------|
| `network` list key `name` | `container` | The bridge device is named `zcn-` plus the name. The name is 1 to 11 characters, lowercase letters, digits and interior hyphens, and a longer name is refused, never truncated. That child's schema must carry this bound |
| `prefix` | `container network` | At most one per family. IPv4 length 1 to 30, IPv6 length 1 to 126. Two networks must not overlap, and a prefix must not contain an address the `interface` config sets |
| `gateway` | `container network` | Default: the network address plus one, per family. A set value is inside the prefix and is not the network or broadcast address |
| `mtu` | `container network` | 68 to 16000, default 1500. Applied to the bridge and both ends of every link. Refused below 1280 when the network has an IPv6 prefix |
| `masquerade` | `container network` | Outbound source translation. Default true, by owner ruling |
| `host-namespace` holding `network` | `container instance` | The container shares the host network namespace. Rule V-1 of that child refuses a `network` entry, a `port` entry and a network `sysctl` beside it |
| `network` list (key: a configured network), `address`, `mac-address` | `container instance` | An attachment. An absent `address` is assigned, an absent `mac-address` is derived, and neither is written to the config |
| `port` list: `protocol`, `listen-address`, `listen-port`, `container-port` | `container instance` | A published port. See "Published ports" |
| `name-server` | `container instance` | See "Name resolution" |

A container network has no VRF node. The keyword is refused as unknown until
VRF support ships (Known Limitations). An instance with no `network` entry and
no shared host network has a loopback device only.

An instance name has no length bound from this child: no device name is built
from it. Inside the container the links are named `eth0`, `eth1`, in the order
of the instance's `network` entries.

This child lands two commands of that child's table C-1, with its words:
`show container network list` and `show container interface list`. It also
gives the operator-surface child one read function for the attachments and
published ports of an instance, which the container detail shows.

Verifier rules: V-1, V-4, V-5, V-6, V-9 and V-13 to V-19 of that child's table
V-1 land with this child, inside the plugin's one judging function. That table
lists each rule and its table E-1 the message. The code and the tests of each
are this child's, in its `network` package. V-13 to V-19 are the rules this
child's design added: the prefix length bounds (V-13), the MTU floor with an
IPv6 prefix (V-14), a prefix with fewer free addresses than attachments (V-15),
a bridge name the `interface` config already declares (V-16), a hardware
address used twice in one network (V-17), a `port` on an instance with no
`network` entry (V-18), and the two backend gates (V-19). V-9 is one rule with
two owners: this child owns the mechanism (the listener conflict contribution),
that child owns the message. The same split holds for the doctor code
`doctor-container-port-blocked`: this child owns the judgment, and the code and
its message are row 6 of that child's table D-1.

### The seam with the runtime child

`spec-container-1-runtime.md` owns the process. Its "Seams with the other
children" table gives this child's participant the rows below, in the same
words here, and this child's package registers one participant for all of them.

| # | What the runtime child provides | This child does with it |
|---|---------------------------------|-------------------------|
| 1 | The created call: each registered participant is called once, after create and before the entrypoint runs, under a deadline, with the container name, the init process id and the network namespace path. An error destroys the container and puts the participant's name and reason in the container's state | Creates each link with its container end inside that namespace and configures it. The whole request finishes inside the deadline. For an instance with no `network` entry the participant returns at once |
| 2 | The network mode: `host-namespace` holding `network` builds no network namespace, and the created call is not made for that container. Its description contribution and its ended call are made as for any container | Makes no link and no translation for it, and still contributes its resolver file |
| 3 | The ended call: made once for every container the runtime child created, after its processes are gone, and for each leftover the sweep removes | Removes the instance's resolver file. The link needs nothing: the kernel removes a link pair with its namespace, and the interface component forgets a link record whose device is gone |
| 4 | The description contribution: before create, each participant may contribute read-only file mounts, and nothing else | Contributes one read-only file mount, the resolver file. This child keeps the file in a directory of its own and hands the runtime child the host path and the path inside the container. The runtime child mounts it read-only and removes nothing it did not create |
| - | The loopback device is up before the created call: the description names it, and the library's init brings it up at the start of its sequence, before it blocks | Never touches the loopback device |

A daemon or plugin that died leaves containers running, and the runtime child
removes every one at the next start, before it starts anything. The links of
those containers go with their namespaces, so no running container outlives its
links. Before each start this child also asks for the removal of any link still
recorded for that instance.

### Addressing

| Rule | Detail |
|------|--------|
| Gateway | The `gateway` value, or the network address plus one, in each family. It is on the bridge, and it is each container's default route |
| Operator-set address | Inside the prefix, one per family, not the network address, not the IPv4 broadcast address, not the gateway, unique in the network |
| Assigned address | The lowest free address above the gateway that no container holds or is configured with. The assignment is stored in the plugin's runtime state under the network and container names, so it is the same after a restart, a reload and a reboot. It is released when the container or its attachment leaves the config |
| Exhaustion | A config with more attachments than free addresses is refused at verify |
| Assignment record | One persistent state key per network, `meta/container/address/{network}`, holding the map from instance name to its addresses and slot, written whole |
| Hardware address | The `mac-address` value, or a locally administered unicast address derived from the network and instance names, so neighbours keep a valid entry across a restart. Two attachments of one network with the same address are refused at verify |
| IPv6 | The address is installed with duplicate address detection off, because Ze guarantees uniqueness and an entrypoint may bind at once |

### Published ports and outbound translation

The plugin contributes two tables whose names it reserves: `ze_container`
(family ip) and `ze_container6` (family ip6). They follow the config, not the
run state: a configured, enabled container's translation exists while the
container is stopped, and a disabled container has none.

| Chain | Hook | Terms |
|-------|------|-------|
| Destination translation | prerouting, and output for connections the router itself opens | One term per `port` entry: the `protocol`, the `listen-port` value or range, the destination is an address of this router (and is one of the `listen-address` values when any is set), then translate to the instance's address on its first network and the `container-port` |
| Source translation | postrouting | Per network whose `masquerade` is true: source in the network prefix, jump to that network's chain. The network's chain returns when the output interface is the network's bridge, then masquerades |

The "destination is an address of this router" match does not exist in the
firewall model today. Without it a published port would capture transit traffic
to the same port. By owner ruling this child adds it to the firewall model as a
general match, which an operator's own firewall config may use too.

By owner ruling a published port must also be allowed by the operator's
firewall policy, and this feature never writes into a table the operator owns.
A drop in any base chain is final, so the translation alone passes nothing
through a chain that drops. Doctor reports a published port that policy blocks.

| Where the connection comes from | Chains of the operator's policy it crosses after translation | What the policy must accept |
|---------------------------------|--------------------------------------------------------------|------------------------------|
| A host beyond the router. This is the closing test's row | The client's packets are translated at the prerouting hook, in the plugin's table, then cross each base chain at the forward hook, entering by the interface the client is behind and leaving by the network's bridge. The replies cross each base chain at the forward hook the other way, entering by the bridge. No chain at the input or output hook is crossed | At the forward hook, towards the container: the `protocol`, destination the instance's address, destination port the `container-port`, output interface the network's bridge. At the forward hook, the replies: by connection state, or by input interface the bridge |
| The router itself, in the host's network namespace | Each base chain at the output hook for the client's packets, and each base chain at the input hook for the replies, which arrive on the network's bridge. The forward hook is not crossed, so a term in a forward chain neither passes nor blocks this connection | At the output hook: the `protocol`, destination the instance's address, destination port the `container-port`. The term names the destination and not the output interface, because the output interface a term sees at that hook was chosen before the translation (A-15). At the input hook: the replies, by connection state or by input interface the bridge |
| No firewall chain at those hooks, or policy accept | None | Nothing: the port is reachable once it is published |

A connection from a host beyond the router to a port no container publishes
matches no translation term, so it is not forwarded: it is addressed to the
router and crosses the input hook, and the router's own stack answers it with a
reset when no listener holds the port. The reset leaves through the output
hook. So the router's policy at the input hook decides what the outside client
observes: a refusal when no base chain there drops the connection (and none at
the output hook drops the reset), a timeout when one does. The forward chain
has no part in it.

A connection from the router itself to a port no container publishes matches
no translation term either. It leaves through the output hook and comes back through
the input hook on the loopback device, and the router's own stack answers it
with a reset when no listener holds the port. For that connection to read as
refused and not as a timeout, no base chain at the output hook or at the input
hook may drop it, by a term or by its policy, and the reset crosses the same
two hooks the other way.

The doctor check `doctor-container-port-blocked` reads the applied
ruleset listing. It reports a `port` entry when a base chain at the forward
hook has policy drop and holds no accepting term whose matches name the
network's bridge as output interface or the instance's address as destination.
It judges those two matches only and does not evaluate the rest of a term, so
it can miss a term that accepts by another match and report it. It reads the
forward hook only: a chain at the output or input hook that blocks a connection
the router itself opens is not reported (Known Limitations).

### The owner's closing test (umbrella AC-14)

The operator-surface child owns the test. It rests on these rows of this spec.

| Part of the test | Rests on |
|------------------|----------|
| The container has a network at its entrypoint | AC-1, AC-2 |
| A client beyond the router gets the server's reply on the published address and port. The client runs in a network namespace of its own, joined to the router by a veth pair with an address on each end, and connects to the router's address on that link | AC-6, through the prerouting term of the destination translation chain, and the policy row "A host beyond the router" above |
| The same test without the policy term fails to connect, and doctor reports `doctor-container-port-blocked` for that `port` entry | The owner's ruling that a published port must also be allowed by firewall policy; the doctor check under "Published ports"; `container-port-blocked` proves it in this child |
| A connection from the same outside namespace to a port the container does not publish is refused | AC-6, its survivor: no translation term matches, so the connection reaches the router's own stack through the input hook and is refused there when no Ze listener holds that port and no input chain drops it. A port a Ze listener holds cannot be published (AC-18) |
| The plugin forked gives the same result | AC-16 |

What the test's config must hold: one `network` with a prefix, the instance's
`network` entry, and one `port` entry with `protocol tcp`, a `listen-port` and
the `container-port` 6379.

The test's client runs beyond the router, as an outside host would: in a
separate network namespace joined to the router by a veth pair with an address
on each end, the way the repository's Linux integration tests build a peer. It
connects to the router's address on that link and the published port. The
connection is forwarded to the container, so it crosses the forward hook in
both directions, where the operator's policy term and the check
`doctor-container-port-blocked` apply. What the test's firewall policy must
hold follows from the policy row "A host beyond the router":

| The test's policy has | It must hold |
|-----------------------|--------------|
| A base chain at the forward hook with policy drop. The test has one, so that the policy term is what lets the client through | A term that accepts `protocol tcp`, destination port 6379, output interface the bridge of the network (or destination the instance's address), and a term that accepts the replies, by connection state `established,related` or by input interface the bridge. Without the first term the client's connection must fail and doctor must report `doctor-container-port-blocked` for the `port` entry: the test asserts both, then adds the term and asserts the reply and no finding |
| A base chain at the input hook | Nothing for the published port, which does not cross it. For the unpublished port to be refused and not timed out, the chain must not drop a connection that arrives on the client's link for an address of the router. With no input chain, the router's own stack refuses it |
| A base chain at the output hook | Nothing for the published port. For the unpublished port, it must not drop the reset the router's stack sends back |

The unpublished port is checked from the same outside namespace. A client on
the router itself is not the closing test's: this child covers that path with
`container-port-from-router` (A-15).

### Name resolution

| Order | Source of a container's resolver addresses |
|-------|--------------------------------------------|
| 1 | The instance's own `name-server` values |
| 2 | The router's own resolver list at container start (`system name-server`, then learned resolvers, as `dns-source` orders them), with loopback addresses removed |

By owner ruling Ze does not answer container names over DNS at first. An
operator who wants one stable resolver address runs the LAN resolver on the
network's gateway address and sets it as the instance's `name-server`.

The resolver file is written per instance before create, in a directory of
this child's own under the run-time tree, and contributed as a read-only mount
at the container's resolver path. This child creates that directory, removes
the instance's file at the ended call, and removes what a previous run left
when the plugin starts. An instance that shares the host network gets one too.

### Problem classes

Each row is a doctor check with a diagnostic code registered from the plugin's
own package. A refusal row is also a config-phase check.

Where the codes register: in this child's package, from its `init()`, with
`diagnostic.Register`, before the checks and the kernel enrolments that name
them. The gated VRRP plugin is the precedent: `registerVRRPDiagnosticCodes` in
`internal/plugins/vrrp/register.go` registers each entry of the plugin's own
code table that way. No container code is added to the always-on table
`internal/core/diagnostic/codes.go`, because a gated feature's name does not
belong in always-on code.

Each enrolled kernel capability has its own pair of codes, the absent one
ending in `-unavailable` and the undetermined one in `-unknown`, as the
interface backend's `doctor-wireguard-unavailable` and
`doctor-wireguard-unknown` are. `kernelcap.MustRegister` takes the pair as two
strings and does not look them up, so the pair may be registered from this
package. The comment on its `Capability` type says both codes must be in
`internal/core/diagnostic/codes.go`: that is true of always-on enrolments only.

| Code (lower-kebab) | Phase | Problem | Fix the message names |
|--------------------|-------|---------|-----------------------|
| `doctor-container-network-backend` | config, verify | The interface backend cannot create a container link, or the firewall backend cannot carry the translation tables | The backend leaf |
| `doctor-container-kernel-link-unavailable`, `doctor-container-kernel-link-unknown` | kernel capability | The kernel lacks veth, or the probe could not tell | The kernel option |
| `doctor-container-kernel-bridge-unavailable`, `doctor-container-kernel-bridge-unknown` | kernel capability | The kernel lacks the bridge device, or the probe could not tell | The kernel option |
| `doctor-container-kernel-translation-unavailable`, `doctor-container-kernel-translation-unknown` | kernel capability | The kernel lacks address translation, or the probe could not tell | The kernel option |
| `doctor-container-kernel-local-match-unavailable`, `doctor-container-kernel-local-match-unknown` | kernel capability | The kernel lacks the local-address match, or the probe could not tell | The kernel option |
| `doctor-container-network-device-conflict` | runtime | A device Ze does not own holds the bridge name | Rename the network or remove the device |
| `doctor-container-network-device-state` | runtime | The bridge is absent, down, or lacks its gateway address | `show container network list`, and the commit that re-applies |
| `doctor-container-link-attach-failed` | runtime | A container did not start because its link could not be made | The reason, and `request container restart name` |
| `doctor-container-address-conflict` | config, verify (V-4, V-5) | Duplicate address, address outside the prefix, overlapping prefixes, exhausted prefix | The node to change |
| `doctor-container-address-overlap-learned` | runtime | A network prefix contains an address the kernel holds on another interface | Change the prefix |
| `doctor-container-port-conflict` | config, verify (V-6, V-9) | Two publishers, or a publisher and a Ze listener, on one protocol, address and port | The node to change |
| `doctor-container-translation-not-applied` | runtime | The firewall refused or did not apply the plugin's tables | The firewall error, and the commit that re-applies |
| `doctor-container-forwarding-disabled` | runtime | IP forwarding is off for a family a network uses | The sysctl node |
| `doctor-container-port-blocked` | runtime | A published port is blocked by the operator's firewall policy, as defined under "Published ports" | The accepting term to add, as a `set firewall` line |
| `doctor-container-resolver-empty` | runtime | A container has no resolver address | `set container instance NAME name-server` |
| `doctor-container-host-network-settings` | config, verify (V-1) | An instance that shares the host network has a `network` entry, a `port` entry or a network `sysctl` | The `delete` line for the statement that conflicts |

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
  → Constraint: a `ze:validate` under a root runs only when the root is in `validatedSections`, a literal list in config code. Every cross-node rule of this child is therefore in the plugin's own judging function, and the container schema declares no `ze:validate`
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
- [ ] `internal/component/sysctl/events/events.go`, `internal/plugins/vrrp/register.go`, `internal/core/statestore/permissions.go`, `pkg/zefs/keys.go`, `internal/le/plugin/boundary/pluginboundary.go` - sysctl defaults over the bus; the in-process-only guard; state key grants and the registered key templates; the watch list of in-process-only calls
- [ ] `plan/post-release/features/spec-container-1-runtime.md` (seams, data flow, files), `spec-container-4-operator-surface.md` (tables S-1, S-2, S-4, V-1, C-1, D-1, E-1) - the sibling designs this spec is reconciled with. `spec-container-2-image-store.md` and `spec-container-5-shell.md` were read for what touches the network: the state key family and the namespace join, and neither changes this spec

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
| Owned-address registry | `internal/component/iface/address_owner.go` `RegisterOwnedAddresses` | A plugin's addresses on a device are part of the desired state and are pruned when released | Does not apply, changed in phase 2: the owned bridge pass keeps the bridge's addresses itself, because this registry is applied only by the config-driven pass, which runs later, while the bridge request answers with the kernel's result |
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
| Kernel requirement enrolment | `internal/component/kernelcap/kernelcap.go` `MustRegister`; `gokrazy/kernel/runtime.config`, `runtime.require` | One enrolment refuses at validate, start and reload and reports in doctor | Extends: the plugin enrols veth, bridge, translation and the local-address match, each in use only when the config needs it, and each with its own `-unavailable` and `-unknown` code pair |
| Listener conflict check | `internal/component/config/listener.go` `ValidateListenerConflicts`, `DiscoverListenerServices` | Finds two services on one protocol, address and port, from schema lists marked `ze:listener` | Extends: a registration lets a plugin contribute endpoints computed from its own section, and an endpoint gains an upper port. A published port list does not have the `ze:listener` list shape |
| Plugin runtime state | `internal/core/statestore/permissions.go` `RegisterPluginKeys`; SDK `StatePut`, `StateGet` | A plugin persists under keys its package grants | Extends: one key pattern for address assignments |
| Interface VRF leaf | `internal/component/iface/yang/ze-iface-conf.yang` `leaf vrf` | Refused by the validator `unimplemented-vrf` | Does not apply: a container network has no VRF node, so the keyword is refused as unknown. The VRF work adds the node when it ships |
| Plugin command as a proxy, and per-command answer declarations | `internal/plugins/vrrp/yang/ze-vrrp-cmd.yang`; `docs/architecture/api/commands.md` "Per-command declarations" (as `spec-container-4-operator-surface.md` reads them) | A YANG command node is bound to a wire method that forwards to the plugin, and declares its answer shape | Extends by use: the two listings of this child are declared that way, with the operator-surface child's words |
| LAN resolver | `internal/plugins/dnsresolver`; `ze-dns-resolver-conf.yang` `list listener`, `allow-from` | A forwarding resolver on operator-named addresses, bound with free-bind | Extends by configuration only: an operator may listen on a gateway address. No code names it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: the container root, delivered to the plugin as its own root, plus the `interface` and `firewall` roots it reads.
- The runtime child's participant registration: the description contribution before create, the created-window call, and the ended call.
- Commands: `show container network list` and `show container interface list`.

### Transformation Path
1. Verify (offline, start, reload): the plugin computes networks, addresses, links and tables from the candidate config and its stored assignments. It refuses address, name, port and mode faults, runs the backend gate against the interface backend, and runs the firewall parser, validator and backend verifier over the tables it would contribute.
2. Apply: the plugin stores new assignments, then sends its complete set of bridges (name, MTU, addresses) to the interface component and its complete set of tables to the firewall component. Each is recorded in the apply journal with the previous set as its undo. It emits the forwarding sysctl defaults.
3. The interface handler checks the caller's grant, records the bridges with their addresses as the caller's complete set, runs the owned-device pass, which also keeps the bridges' addresses (Design Insights, phase 2 difference 1), and answers its outcome. The firewall handler checks the reservation, parses the body, registers the tables under the caller's name, reconciles, and answers.
4. Container start: before create, the participant writes the resolver file and contributes its mount. In the created window it reads the inode number of the network namespace path it was given, and asks the interface component for one link per attachment: the bridge, a slot number, the init process id with that inode number, the container-end name, hardware address, MTU, addresses and gateways. It first asks for the removal of any link still recorded for the instance.
5. The interface handler opens the namespace of that process, refuses when the inode differs, and has the backend create the pair: the host end named by `ComposeOwnedDeviceName`, marked with the owned alias, enslaved to the bridge, MTU set, up; the peer end created inside the namespace under its final name, then addressed, set up and routed through a socket bound to that namespace (phase 2 difference 3). It records the link and answers the host-end name.
6. The participant returns success and the runtime releases the container. On any failure it asks for the links it made to be removed and returns the error, which aborts the start.
7. Container exit: the kernel removes the pair with the namespace, and the interface component forgets a link record whose device is gone, so the link needs no call. The runtime child's ended call still arrives, and the participant removes the instance's resolver file in it.
8. Reload: a removed container, attachment, port or network is absent from the next complete set, so the reconcile removes it.
9. Stop: an orderly daemon stop removes the firewall tables (existing) and the owned devices (this spec). A plugin process that ends without a stop is handled by the two cleanup registrations.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Container plugin to interface component | Engine operations in the interface namespace: replace the caller's owned bridges, create a link, remove a link | No |
| Container plugin to firewall component | One engine operation in the firewall namespace: replace the caller's tables, as a firewall config section | No |
| Container plugin to sysctl component | The existing default event on the EventBus | No |
| Container plugin to the listings | The text command dispatcher, read only | No |
| Runtime child to this child | The runtime's participant registration inside the plugin tree: the description contribution, the created-window call and the ended call, in one process | No |
| Any surface to this child's listings | The dispatcher, the proxy wire method, then the plugin's command, internal and forked | No |
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
- The runtime child's three calls: the description contribution, the created call and the ended call.

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
| A-3 | The loopback device is up before the created call | `spec-container-1-runtime.md`, "Seams with the other children": the description names the loopback device, and the library's init brings it up at the start of its sequence, before it blocks. Its AC-1 asserts it | This child's participant raises the loopback device itself, through the same namespace-bound socket | `container-none-network.ci`, and the runtime child's `container-start` | confirmed by the sibling's design |
| A-13 | This child keeps the resolver file in a directory of its own under the run-time tree, with its own entry in the raw-write allowlist. The runtime child mounts it read-only and removes nothing it did not create | `spec-container-1-runtime.md`, "Seams with the other children", the description contribution | The file needs another home, decided with that child | `container-resolver-list.ci`, and `./le arch fs-persistence check` | confirmed by the sibling's design |
| A-4 | A configure-safe engine operation can be called from the plugin's configure stage, and a plugin with `Dependencies` on `interface` and `firewall` configures after both | `register_engine_op.go` `EngineOperationConfigureSafe`; `ai/doors/plugin-infra.md` traps | The first declaration moves to the all-plugins-ready callback, and containers start after it | `container-network-wiring-external.ci` | unvalidated |
| A-5 | The kernel deletes the host end when the namespace holding the peer end is destroyed | Kernel behaviour, not read | The plugin's "forget the links" request must delete the host end, which it does anyway | `TestHostEndVanishesWithNamespace` | unvalidated |
| A-6 | The generation cleanup runs for an internal plugin and a forked one, after its requests drain | `register_engine_op.go` `ProcessGenerationCleanup` comment | A backstop is missing in one mode, and the next reconcile's orphan scan is the only cleanup | `container-network-plugin-ends.ci` in both modes | unvalidated |
| A-7 | The nft `fib` expression is available in the ip and ip6 families on the runtime kernel | `gokrazy/kernel/kernel.config` requests `NFT_FIB_IPV4` and `NFT_FIB_IPV6`; neither is in a require file; no arm64 config was read | The match needs another lowering, or the options need a kernel change | The enrolment's probe, and `TestLocalDestinationDNATLeavesTransitAlone` in the QEMU guest | unvalidated |
| A-8 | Bridged traffic between two containers of one network crosses the forward hook, because bridge netfilter is built in | The kernel research report: `BRIDGE_NETFILTER=y`. The sysctl default was not read | The guide's sentence about same-network traffic changes. No design change | `container-network-same-network-filter.ci` records which it is | unvalidated |
| A-9 | Two enrolments may name different subsystems for kernel options no one enrolled before, and the enrolled-symbol test accepts a line added to `runtime.require` | `kernelcap.go` `Capability`; `TestEnrolledKernelSymbolsAreInRuntimeRequire` (named in the kernel research, not read) | The options are enrolled by the interface and nft backends instead, with a registered in-use predicate | Reading `MustRegister` and running that test | unvalidated |
| A-10 | The runtime child has a test route that starts a container with no registry | `spec-container-1-runtime.md`: a build with the `zetest` tag adds a store that resolves an image name to a folder, and `internal/test/containerprobe` is the fixture program | The functional tests that need a container wait for the image store child | That spec's text, read 2026-11-16 | confirmed by the sibling's design |
| A-11 | The operator-surface child's design, which holds the one table of config and command words, is settled before this child's wiring phase | The umbrella: "its design is settled before any sibling's wiring phase". This spec keeps `Depends: spec-container-1-runtime`, because that child's implementation comes after this one's | Phase 1 waits | That spec reaching `ready` | unvalidated |
| A-14 | The operator-surface child's `network` name bound is 1 to 11 characters and its `masquerade` default is true | That spec's table S-2 states both, by the owner's rulings of 2026-11-16 | A name of 12 characters or more passes the schema and is refused by this child's verifier instead, with the same message | That spec's `TestNetworkNameBound` | confirmed by the sibling's design |
| A-15 | For a connection the router itself opens to a published port: the plugin's output term translates it before the operator's filter chains at the output hook run, so those chains see the instance's address and the `container-port`; the output interface they see is the one chosen before the translation; and the replies arrive on the bridge and cross the input hook | Kernel netfilter behaviour, not read in source. The firewall model names the hooks (`internal/component/firewall/model.go` `HookOutput`, `HookInput`, `HookForward`) and bounds a base chain's priority, so an operator's chain placed before the translation sees the router's address and the `listen-port` instead | The output row of the policy table names other matches. No design change | `container-port-from-router.ci` in the QEMU guest | unvalidated |
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
| R-9 | The operator's forward chain drops the published traffic and the port looks broken | A translation term whose counter rises while no connection completes | The doctor check `doctor-container-port-blocked`, the policy table under "Published ports", and the guide. The reserved tables name `show container network list` as the command that shows what decides their rules |
| R-10 | After a daemon crash, the first reconcile deletes the links of containers that are still running | Running containers lose their network at restart | By owner ruling containers are not re-adopted: the runtime child removes every leftover container at the next start, before it starts anything, so no running container keeps a link this child deleted |
| R-15 | The created-window deadline ends while the interface request is in flight | A start fails with a deadline error under load | The request is bounded below the deadline, and a link made after the abort is removed by the pre-start removal of the next attempt and by the orphan scan |
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
| A forked plugin declares an owned bridge | → | The interface engine operation, the owned registry, the netlink backend | `test/container/container-network-wiring-external.ci` |
| An internal plugin declares an owned bridge | → | The same handler through the direct path | `test/container/container-network-wiring-internal.ci` |
| A forked plugin asks for a link into a namespace it holds | → | The link handler and the netlink link creator | `test/container/container-link-wiring-external.ci` |
| A forked plugin contributes a reserved table | → | The firewall engine operation, `RegisterTables`, `ApplyAll` | `test/container/container-table-wiring-external.ci` |
| A config with a network and an attached container is committed | → | The plugin's verify and apply, the runtime seam, the link | `test/container/container-network-reach-gateway.ci` |
| A config publishes a port | → | The plugin's table builder and the firewall contribution | `test/container/container-port-published.ci` |
| `ze config validate` on a config with a host container and a port | → | The plugin's in-process verifier | `test/container/container-host-network-refused.ci` |
| `show container network list` and `show container interface list` | → | This child's two listing handlers, in the plugin's process | `test/container/container-network-listings.ci` |
| The runtime's participant registration | → | This child's participant: the resolver mount and the link | `TestNetworkParticipantIsRegistered` |

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
| AC-11 | An instance has `host-namespace network` | It runs in the host network namespace: it sees the host's interfaces and binds the host's addresses. Ze creates no link and no translation for it | Such an instance may set its own `name-server`, because a resolver file is not a setting of the namespace |
| AC-12 | An instance with `host-namespace network` also has a `network` entry, a `port` entry or a network `sysctl` (rule V-1) | Validate, start and reload refuse the config with the operator-surface child's V-1 message, which names the instance and the `delete` line for the statement that conflicts. On a reload the running containers keep running | The same config without that statement is accepted, because sharing the host network alone is valid |
| AC-13 | An instance has no `network` entry and does not share the host network | It holds a loopback device that is up, and that is its only link | No translation exists for it, because it has no address. A `port` entry on it is refused at verify, because nothing can receive the traffic |
| AC-14 | Two networks and three attached instances are running | `show container network list` holds one row per network with its `name`, `prefix`, `gateway`, `interface` (the bridge device), `mtu`, `masquerade` and `attached` count. `show container interface list` holds one row per attachment with its `instance`, `network`, `interface` (the host end), `address`, `mac-address` and `state`. Both answer the same with the plugin forked | A stopped instance's attachment row shows its assigned address and no host end, because the link exists only while the container runs |
| AC-15 | The interface backend is `vpp` and the config has a container network, or the firewall backend is `vpp` and the config needs a translation | Validate, start and reload refuse, naming the container node and the backend leaf | A config whose instances all share the host network or have no `network` entry is accepted on those backends, because nothing needs a link or a table |
| AC-16 | The container plugin runs as a separate process | AC-1, AC-2, AC-6 and AC-8 hold exactly as when it runs internal | None differ between the two modes, because both use the same requests |
| AC-17 | An instance starts with no `name-server` | Its resolver file lists the router's resolver addresses of that moment, without loopback addresses. With `name-server` set on the instance, the file lists those values | A container whose list is empty still starts, and doctor reports it, because a missing resolver is not a start failure |
| AC-18 | Two containers publish the same protocol, address and port, or one publishes a port a Ze listener uses | Validate, start and reload refuse, naming both parties and the node to change | Two publishers on different router addresses are accepted, because they do not overlap |
| AC-19 | Two attachments set the same address, an address is outside the prefix, two networks overlap, or a network has fewer free addresses than attachments | Validate, start and reload refuse, naming the node and the value | A config where each address is unique and inside its prefix is accepted, including one that fills the prefix exactly |
| AC-20 | A config uses a container network on a kernel without veth, bridge, translation or the local-address match | Validate, start and reload refuse, naming the option, with that capability's `-unavailable` code. Doctor reports the same code. A probe that cannot tell reports the capability's `-unknown` code as a warning | A config with no container network is accepted on that kernel, because nothing needs the option |
| AC-21 | Each problem class in the "Problem classes" table is provoked | `ze doctor` reports it with its code, its cause and the command or node that fixes it | A healthy network reports no finding, because a check that always reports is noise |
| AC-22 | VRRP is configured and the daemon stops in an orderly way | Its macvlans are removed by the interface engine's stop path, and the stop completes with success | Unmarked macvlans remain, because the stop path requires the mark |
| AC-23 | The `interface` config names a device, or a veth peer, whose name lies inside a device name prefix a plugin granted itself (scope widened in phase 2, owner question Q-1 open) | The config is refused when it is parsed, and the refusal names the plugin that owns the prefix and the prefix. Proven by `TestConfigRefusesNameInsideOwnedDevicePrefix` (a bridge name and a veth peer name inside the prefix are refused) | A configured name outside every granted prefix is accepted, because no plugin creates devices there |

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
| `TestVerifyRefusesHostNetworkWithNetworkSettings` | same | Rule V-1: a `network` entry, a `port` entry or a network `sysctl` beside the shared host network | |
| `TestVerifyRefusesPortWithNoNetwork` | same | A `port` entry on an instance with no `network` entry | |
| `TestVerifyRefusesGatewayOutsidePrefix` | same | Rule V-5: a set `gateway` outside its prefix | |
| `TestNetworkParticipantIsRegistered` | `internal/plugins/container/network/attach_test.go` | The participant is in the runtime's registration and contributes the resolver mount | |
| `TestParticipantSkipsHostAndUnattachedInstances` | same | No interface request for an instance that shares the host network or has no `network` entry | |
| `TestListingsCarryTheDeclaredFields` | `internal/plugins/container/network/command_test.go` | The two listings answer the fields of the operator-surface child's table C-1 | |
| `TestVerifyRefusesIPv6BelowMinimumMTU` | same | MTU under 1280 with an IPv6 prefix | |
| `TestVerifyRefusesBridgeNameInInterfaceConfig` | same | The derived name is declared in `interface` | |
| `TestBridgeNameIsRefusedNotTruncated` | `internal/plugins/container/network/name_test.go` | A 12-character network name | |
| `TestHardwareAddressIsStableAndUnicast` | same | The derived address | |
| `TestTablesForPublishedPort` | `internal/plugins/container/network/tables_test.go` | The prerouting and output terms of one port | |
| `TestTablesForOutboundTranslation` | same | The jump, return and masquerade terms | |
| `TestTablesFollowConfigNotRunState` | same | A stopped, enabled container has its term; a disabled one has none | |
| `TestPublishedPortEndpoints` | `internal/plugins/container/network/listeners_test.go` | The endpoints the conflict check receives | |
| `TestResolverPrecedence` | `internal/plugins/container/network/resolver_test.go` | The instance's `name-server`, else the router's list without loopback | |
| `TestOwnedBridgeRequestRefusesCallerWithoutGrant` | `internal/component/iface/device_owner_test.go` | The grant check | |
| `TestReconcileCreatesOwnedBridgeWithAlias` | `internal/component/iface/config_apply_owned_test.go` | Create and mark, with a scripted backend | written in phase 2, passed in the scoped package run |
| `TestOrphanScanKeepsForeignBridge` | same | An unmarked bridge and a marked non-owned kind survive | written in phase 2, passed in the scoped package run |
| `TestOrphanScanDeletesLinkWithNoRecord` | same | A marked link no record names | written in phase 2, passed in the scoped package run |
| `TestConfigRefusesNameInsideOwnedDevicePrefix` | same | AC-23: a bridge name and a veth peer name inside a granted prefix are refused at parse, naming the plugin and the prefix; a name outside is accepted | written in phase 2, passed in the scoped package run |
| `TestLinkRequestRefusesNamespaceMismatch` | `internal/component/iface/cmd/owned_link_test.go` | The inode check | written in phase 2, passed in the scoped package run |
| `TestStopPathDeletesOwnedDevices` | `internal/component/iface/register_stop_test.go` | The ordered stop deletion | written in phase 2, passed in the scoped package run |
| `TestVPPBackendRefusesOwnedLink` | `internal/plugins/iface/vpp/owned_test.go` | Exact or refuse | written in phase 2, passed in the scoped package run |
| `TestParseLocalDestinationMatch` | `internal/component/firewall/config_local_test.go` | The new match parses, only from its leaf, and is refused in a bridge table | written in phase 3, not run |
| `TestContributionRefusesUnreservedTable` | `internal/component/firewall/contribution_test.go` | Only the caller's reserved names | written in phase 1 |
| `TestContributionReplacesOwnersCompleteSet` | `internal/component/firewall/contribution_replace_test.go` | The contribution reaches the backend beside an operator table, a config apply keeps it, a refused set leaves the previous set in the registry and the backend, an empty set withdraws it | written in phase 3, not run |
| `TestGenerationCleanupWithdrawsOwner` | `internal/component/firewall/contribution_replace_test.go` | A gone process's tables are withdrawn, an older generation withdraws nothing, a refused withdrawal raises `doctor-firewall-contributed-tables` until a reconcile succeeds | written in phase 3, not run |
| `TestLowerLocalDestination` | `internal/plugins/firewall/nft/lower_local_test.go` | The nft expressions of the match | written in phase 3, not run |
| `TestVPPVerifierRefusesLocalDestination` | `internal/plugins/firewall/vpp/verify_local_test.go` | The VPP verdict on the match, in a filter term and in a destination translation | written in phase 3, not run |
| `TestListenerContributorJoinsConflictCheck` | `internal/component/config/listener_contributor_test.go` | A contributed endpoint conflicts with a service | |
| `TestPortRangeOverlapIsAConflict` | same | Ranges that overlap | |
| `TestOwnedLinkIsCreatedInsideNamespace` | `internal/plugins/iface/netlink/owned_link_integration_linux_test.go` | Kernel: the peer end, its name, address, route and state inside a namespace | written in phase 2, not run |
| `TestHostEndVanishesWithNamespace` | same | Kernel: the pair is gone when the namespace is | written in phase 2, not run |
| `TestOwnedBridgeIsAdoptedAfterRestart` | `internal/component/iface/owned_bridge_integration_linux_test.go` (it drives the unexported pass) | Kernel: a marked bridge is adopted, and its drifted MTU and addresses are put back in place, never re-made | written in phase 2, not run |
| `TestLocalDestinationDNATLeavesTransitAlone` | `internal/plugins/firewall/nft/local_dnat_integration_linux_test.go` | Kernel: a router namespace and a client namespace (phase 3 difference 2); only router-addressed traffic is translated, with a positive control and a control without the match | written in phase 3, not run |

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
drafted under `test/draft/container/` and promoted. The suite is the `container`
suite the runtime child registers.

The rows that start a container run in Ze's own-system guest. They rest on
acceptance criterion AC-14 of
`plan/post-release/features/spec-appliance-perm-grows-at-install.md`: the
own-system guest's persistent partition has the appliance's format and enforces
a size cap. Without it the guest cannot hold a container's storage the way the
appliance does, and these tests would prove the network of a container the
device cannot run.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `container-network-wiring-external` | `test/container/container-network-wiring-external.ci` | A forked test plugin's bridge appears in the kernel with the owner mark | drafted in `test/draft/container/`, not run since phase 2; expected green |
| `container-network-wiring-internal` | `test/container/container-network-wiring-internal.ci` | The same, internal | drafted in `test/draft/container/`, not run since phase 2; expected green |
| `container-link-wiring-external` | `test/container/container-link-wiring-external.ci` | A forked test plugin's link has its peer end configured in a namespace | drafted in `test/draft/container/`; phase 2 added the `link-netnsid` check and the peer end's facts through the `iface/peer-end` fixture; not run; expected green |
| `container-table-wiring-external` | `test/container/container-table-wiring-external.ci` | A forked test plugin's table is in the ruleset listing | drafted in `test/draft/container/`; phase 3 added a `firewall { backend nft }` section; not run since phase 3; expected green |
| `container-network-reach-gateway` | `test/container/container-network-reach-gateway.ci` | AC-1, AC-2: an entrypoint that connects to the gateway at once succeeds | |
| `container-network-forked` | `test/container/container-network-forked.ci` | AC-16: the same with the plugin forked | |
| `container-network-address-stable` | `test/container/container-network-address-stable.ci` | AC-3 | |
| `container-network-ipv6` | `test/container/container-network-ipv6.ci` | AC-4 | |
| `container-network-firewall-term` | `test/container/container-network-firewall-term.ci` | AC-5, with the term's counter as the positive control | |
| `container-network-same-network-filter` | `test/container/container-network-same-network-filter.ci` | A-8: whether a forward term sees traffic between two containers of one network | |
| `container-port-published` | `test/container/container-port-published.ci` | AC-6: a connection from a network namespace beyond the router reaches the container and the listing shows the term | |
| `container-port-transit-untouched` | `test/container/container-port-transit-untouched.ci` | AC-6 survivor: transit to the same port is not translated | |
| `container-port-from-router` | `test/container/container-port-from-router.ci` | AC-6 and A-15, this child's own coverage of the router-originated path, which the closing test does not take: a client on the router itself, with output and input chains whose policy is drop and the accepting terms of the policy row "The router itself", reaches the published port. Without the output term the connection fails. A port no container publishes is refused, not timed out | |
| `container-network-outbound` | `test/container/container-network-outbound.ci` | AC-7 | |
| `container-network-reload-removes` | `test/container/container-network-reload-removes.ci` | AC-8, with a foreign bridge and table that must survive | |
| `container-network-daemon-stop` | `test/container/container-network-daemon-stop.ci` | AC-9, AC-22 | |
| `container-network-crash-leftovers` | `test/container/container-network-crash-leftovers.ci` | AC-10 | |
| `container-host-network` | `test/container/container-host-network.ci` | AC-11 | |
| `container-host-network-refused` | `test/container/container-host-network-refused.ci` | AC-12, parse only | |
| `container-none-network` | `test/container/container-none-network.ci` | AC-13 | |
| `container-network-listings` | `test/container/container-network-listings.ci` | AC-14: the two listings, internal and forked | |
| `container-port-blocked` | `test/container/container-port-blocked.ci` | The closing test's policy rule, proven here: a client in a network namespace beyond the router, joined by a veth pair, connects to the router's address on that link and the published port. With a forward chain whose policy is drop and no accepting term, the connection fails and doctor reports `doctor-container-port-blocked` naming the instance and the `port` entry; adding the accepting term lets the same client through and clears the finding. A port no container publishes is refused from the same namespace | |
| `container-network-vpp-refused` | `test/container/container-network-vpp-refused.ci` | AC-15, parse only | |
| `container-resolver-list` | `test/container/container-resolver-list.ci` | AC-17 | |
| `container-resolver-on-gateway` | `test/container/container-resolver-on-gateway.ci` | The LAN resolver listening on a gateway address answers a container | |
| `container-port-listener-conflict` | `test/container/container-port-listener-conflict.ci` | AC-18, parse only | |
| `container-address-refused` | `test/container/container-address-refused.ci` | AC-19, parse only | |
| `doctor-container-network` | `test/container/doctor-container-network.ci` | AC-21: one provoked class per step of the sequence, each with its code | |
| `container-network-plugin-ends` | `test/container/container-network-plugin-ends.ci` | The plugin process ends: its devices and tables are withdrawn; both modes | |

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
- `internal/component/iface/device_owner.go` - the `init()` grant of a name prefix to an owner; the gauge counts bridges and links; `ownedDevicePrefixes`, `refuseGrantedDeviceName`. The bridge and link records live in the new `owned_bridge_link.go` (phase 2)
- `internal/component/iface/config_apply.go` - `reconcileOwnedDevices` runs the owned bridge pass and forgets link records whose host end is gone; the orphan scan covers macvlan, bridge and veth and needs both the mark and an owned kind
- `internal/component/iface/config.go` - a configured name or veth peer inside a granted prefix is refused at parse (AC-23, phase 2 scope widening)
- `internal/component/iface/register.go` - the stop path deletes marked devices before `CloseBackend`
- `internal/component/iface/config_test.go`, `migrate_linux_test.go` - the test backends implement the two new `Backend` methods
- `internal/core/diagnostic/codes.go` - the code `doctor-iface-owned-device-cleanup`, an interface code, not a container one
- `internal/plugins/iface/netlink/backend_other.go` - the non-Linux backend refuses both new methods
- `internal/test/plugins/fakeowner/fakeowner.go`, `register.go` - the link request carries a hardware address, an address and a gateway; the outcome serves `link-pid`; the plugin depends on the interface component
- `test/container/container-link-wiring-external.ci` - asserts `link-netnsid` on the host end and the peer end's facts through `iface/peer-end`
- `internal/component/iface/yang/embed.go`, `register.go` - the glue lines of `ze-iface-owned-api.yang` (the three requests are not in `ze-iface-api.yang`, see Design Insights)
- `internal/component/firewall/yang/embed.go`, `register.go` - the glue lines of `ze-firewall-api.yang`
- `internal/component/iface/device_owner_test.go` - `TestOwnedBridgeRequestRefusesCallerWithoutGrant`, `TestOwnedDeviceGrantRefusesOverlappingPrefix`
- `internal/test/plugins/all/all.go` - the blank import of `fakeowner`
- `docs/plugin-development/protocol.md`, `docs/functional-tests.md` - the SDK methods, the `fakeowner` test plugin and the `iface/peer-end` fixture
- `internal/plugins/iface/vpp/ifacevpp.go`, and the non-Linux backend - refuse both new methods
- `internal/component/firewall/model.go`, `config.go`, `validate.go`, `yang/ze-firewall-conf.yang` - the local-address match (the leaf `destination-local`, the type `MatchDestinationLocal`)
- `internal/component/firewall/cmd/show.go` - both match switches name the new match (phase 3)
- `internal/component/firewall/contribution.go`, `register_contribution.go` - the replacement, the restore, the generation cleanup; `tablesContributionNotBuilt` deleted (phase 3)
- `internal/core/diagnostic/codes.go` - the code `doctor-firewall-contributed-tables`, a firewall code, not a container one (phase 3)
- `test/container/container-table-wiring-external.ci` - a `firewall { backend nft }` section, so the firewall engine runs and its orderly stop flushes the contributed table (phase 3)
- `docs/plugin-development/protocol.md` - the table request answers with the kernel's outcome (phase 3)
- `internal/plugins/firewall/nft/lower_linux.go` - its lowering
- `internal/plugins/firewall/vpp/verify.go` - its refusal
- `internal/component/config/listener.go` - contributed endpoints and the upper port
- `internal/le/plugin/boundary/pluginboundary.go` - the in-process owned-device register functions join `dangerousCalls`
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - the options this child enrols that are not pinned
- `feature-gates.txt` - the `network` package under the runtime child's `ze_container` tag
- `pkg/zefs/keys.go` - the key template `meta/container/address/{network}`
- `internal/le/arch/fspersistence/fspersistence.go` - the `fileAllowlist` entry for the file that writes the resolver file, with its reason
- `docs/architecture/zefs-format.md` - the new key in the key registry section
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
- `internal/component/iface/owned_bridge_link.go` - `OwnedBridgeSpec`, `OwnedLinkSpec`, the bridge and link records, `ReplaceOwnedBridges`, `CreateOwnedLink`, `RemoveOwnedLink`, `WithdrawOwnedDevices`, the owned bridge pass, the stop-path deletion, the request bounds and refusals
- `internal/component/iface/owned_namespace_linux.go`, `owned_namespace_other.go` - open the target namespace and compare its inode; non-Linux refuses
- `internal/component/iface/config_apply_owned_test.go` - `TestReconcileCreatesOwnedBridgeWithAlias`, `TestOrphanScanKeepsForeignBridge`, `TestOrphanScanDeletesLinkWithNoRecord`, `TestConfigRefusesNameInsideOwnedDevicePrefix`
- `internal/component/iface/register_stop_test.go` - `TestStopPathDeletesOwnedDevices`
- `internal/component/iface/cmd/owned_link_test.go` - `TestLinkRequestRefusesNamespaceMismatch`
- `internal/plugins/iface/vpp/owned_test.go` - `TestVPPBackendRefusesOwnedLink`
- `internal/plugins/iface/netlink/owned_link_integration_linux_test.go` - `TestOwnedLinkIsCreatedInsideNamespace`, `TestHostEndVanishesWithNamespace`
- `internal/component/iface/owned_bridge_integration_linux_test.go` - `TestOwnedBridgeIsAdoptedAfterRestart`
- `internal/test/fixture/iface_peer_end_linux.go`, `register_iface_peer_end_linux.go` - the fixture `iface/peer-end`, which prints a link's facts from inside the namespace of a process a daemon answer names
- The doctor check for `doctor-iface-owned-device-cleanup` (likely `internal/component/iface/doctor_owned.go`) - in progress by the phase 2 fix agent
- `internal/component/firewall/contribution.go`, `register_contribution.go` - the firewall request and cleanup
- `internal/component/firewall/yang/ze-firewall-api.yang` - the request
- `internal/component/firewall/contribution_test.go` - `TestContributionRefusesUnreservedTable`
- `internal/component/firewall/contribution_replace_test.go` - `TestContributionReplacesOwnersCompleteSet`, `TestGenerationCleanupWithdrawsOwner` (phase 3)
- `internal/component/firewall/doctor_contribution.go` - the doctor check `firewall-contributed-tables` and the commit observer that clears its warnings (phase 3)
- `internal/component/firewall/config_local_test.go`, `internal/plugins/firewall/nft/lower_local_test.go`, `internal/plugins/firewall/nft/local_dnat_integration_linux_test.go`, `internal/plugins/firewall/vpp/verify_local_test.go` - the local-address match tests (phase 3)
- `internal/component/iface/yang/ze-iface-owned-api.yang` - the three interface requests
- `internal/component/plugin/server/engine_op_decode.go` - `DecodeEngineOperationInput`, the strict request decoder both components use
- `test/container/container-network-wiring-external.ci`, `container-network-wiring-internal.ci`, `container-link-wiring-external.ci` - the bridge and link wiring tests, green in the guest since phase 2 and promoted. Each config holds an `interface` section: the owned-device requests are refused while no interface backend is loaded, and the backend loads only when the interface component is configured
- `test/container/container-table-wiring-external.ci` - the table wiring test, green in the guest since phase 3 and promoted
- `pkg/plugin/rpc/owned_devices.go`, `pkg/plugin/rpc/firewall_tables.go` - the value types and method names
- `pkg/plugin/sdk/sdk_owned_devices.go`, `pkg/plugin/sdk/sdk_firewall_tables.go` - the SDK methods
- `internal/plugins/container/network/` - `verify.go`, `address.go`, `name.go`, `tables.go`, `resolver.go`, `listeners.go`, `attach.go` (the participant), `command.go` (the two listings and the read function), `register_tables.go`, `register_kernelcap_linux.go`, `kernelcap_linux.go`, `doctor.go`, and the tests named above
- `internal/plugins/container/yang/ze-container-conf.yang`, `internal/plugins/container/yang/ze-container-cmd.yang`, `internal/plugins/container/register.go` - created by the first child to land (the runtime child). This child adds its nodes and its two command nodes with the operator-surface child's words, `ze:backend "netlink"` on the `network` list, the declared reads of the `interface` and `firewall` roots, the dependencies on both plugins, the two command declarations and the state key grant
- `internal/test/plugins/fakeowner/` - a test plugin that declares a bridge, a link into a namespace it holds, and a table
- The `.ci` files in the Functional Tests table
- `docs/architecture/container/network.md` - the design page: ownership marks, the two requests, the seam, the tables

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/iface/yang/ze-iface-api.yang`, `internal/component/firewall/yang/ze-firewall-api.yang`, `ze-firewall-conf.yang`; the container nodes go in the plugin's schema as `spec-container-4-operator-surface.md` names them |
| YANG validation constraints | Yes | The ranges in the Boundary Tests table, as native `range` and `length`; addresses and prefixes from `ze-types.yang` |
| YANG custom validators | No | No `ze:validate` is added: a container root is not in `validatedSections`, so every cross-node rule is the plugin's verifier |
| CLI commands/flags | Yes | `show container network list` and `show container interface list`: command nodes in the plugin's command module, handlers in `internal/plugins/container/network/command.go`. The words are table C-1 of `spec-container-4-operator-surface.md` |
| CLI grammar (keyword before value) | Yes | Fixed by the operator-surface child; `./le cli grammar` judges it |
| Editor autocomplete | Yes | Automatic for the enumerations; the attachment's network name is a reference to a configured network |
| Functional test for new RPC/API | Yes | The four wiring `.ci` files |
| Pipe completeness | Yes | Both listings answer rows with a declared shape and column order, and `show container interface list` declares `address` as an address field |
| Env var registration | No | No tunable is added |
| Doctor check for runtime dependencies | Yes | `internal/plugins/container/network/doctor.go`, `kernelcap_linux.go`; netlink and nftables use through the two components |
| Doctor code per problem class | Yes | The "Problem classes" table: each code, its phase and its fix, registered from the plugin's package with `diagnostic.Register`, as `registerVRRPDiagnosticCodes` does (`internal/plugins/vrrp/register.go`) |
| Prometheus counters/metrics | Yes | `ze_iface_owned_devices{owner}` already counts owned devices and now counts the new kinds. No new series |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/containers.md`, created by the runtime child: add networks and published ports |
| 2 | Config syntax changed? | Yes | `docs/guide/firewall.md` for the local-address match; the container nodes are documented by `spec-container-4-operator-surface.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: the two listings, in the section the operator-surface child writes |
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
   - Verify: a forked and an internal test plugin reach both handlers. The wiring tests fail because the handlers refuse every request. This phase uses no container word, and it still starts only after the operator-surface child's design is settled (A-11)
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
   - Verify: needs the runtime child's participant registration and its test image store. Tests fail → implement → pass in both plugin modes
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
| A forked plugin gets the same result as an internal one | `./le test container` rows `container-network-forked`, `container-network-wiring-external` |
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

- Permission: the owner allowed this spec to be claimed above the work-in-progress cap on 2026-11-16.
- Phase 1 deviation, to be ruled by the main session: the three interface requests cannot live in `ze-iface-api.yang`. `TestAPIRPCInputMatchesCommandTree` (`internal/component/iface/yang/api_rpc_input_test.go`) requires every rpc of that module to be bound to a command path, and an engine operation is bound to none. They go in a module of their own, `ze-iface-owned-api.yang`, beside it in the same package, as the system RIB's operations live in `ze-sysrib-api.yang`.
- Decision of the main session, 2026-11-16: phase 1 does not stub the Wiring Test rows 5 to 9 (reach-gateway, port-published, host-network-refused, listings, `TestNetworkParticipantIsRegistered`). They need the network package and the participant, and they land with phase 4, where the step order puts that package.
- Phase 1 as applied: the strict request decoder is one exported helper, `pluginserver.DecodeEngineOperationInput` (`internal/component/plugin/server/engine_op_decode.go`), used by both components, instead of a copy per component beside the system RIB's own. The firewall handler refuses a section with no `firewall` table map, because `ParseFirewallConfig` answers no table both for that and for an empty map, and a contribution with no table withdraws everything: only an explicit empty map withdraws. A caller that reserved no table at all is refused before its section is read. The SDK reply bound reuses `rpc.RouteViewResultBudget`, the frame budget, so the direct path refuses what RPC cannot frame; its name is the route view's.
- Phase 2 as applied, seven differences from the design above (the Data Flow steps 3 and 5 and the owned-address row of Existing Mechanisms are read through them):
  1. The bridge's addresses are kept by the owned bridge pass itself, not by the owned-address registry: that registry is applied only by the config-driven pass, which runs later in the background, while the bridge request must answer with the kernel's result.
  2. A marked bridge is adopted and its MTU and addresses are put back in place, never deleted and re-made: deleting it would drop the links that joined it. `TestOwnedBridgeIsAdoptedAfterRestart` proves the adoption instead of a re-make.
  3. The peer end is created under its final name inside the namespace, so there is no rename step.
  4. `TestOwnedBridgeIsAdoptedAfterRestart` lives in `internal/component/iface/owned_bridge_integration_linux_test.go`, not in the netlink package, because it drives the unexported pass.
  5. The bridge and link records live in the new `internal/component/iface/owned_bridge_link.go`, not in `device_owner.go`.
  6. Scope widened: a configured interface name, or a veth peer name, inside a prefix a plugin granted itself is refused when the config is parsed (AC-23). Without it the config apply could adopt or delete a device a plugin owns. Owner question Q-1 below is open on it.
  7. An unmarked device holding an owned bridge name is refused and left alone, never adopted. The owned macvlan pass still adopts and re-marks an unmarked macvlan under a registered name; the bridge pass does not, because an unmarked bridge may be the operator's. A crash between the netlink backend's create and its mark leaves such a bridge, and the operator deletes it (`CreateOwnedBridge` comment).

- Phase 3 as applied, seven differences from the design above (the Data Flow steps 3 and 9 are read through them):
  1. The config word of the local-address match is `destination-local`, an empty leaf of the `from` block, lowered by nft to `fib daddr type local`. It is valid in families ip and ip6, as A-7 assumes; verify refuses it elsewhere, family inet included: the runtime kernel leaves `NFT_FIB_INET` unset, so a ruleset with the match in an inet table is refused whole by the kernel (seen in the guest: `flush: ... no such file or directory`). Open for the owner: set `NFT_FIB_INET` in the kernel so a dual-stack table may carry the match. Verify refuses it and the VPP verifier refuses it in a filter term and in a destination translation, each naming the leaf.
  2. `TestLocalDestinationDNATLeavesTransitAlone` builds a router namespace and one client namespace, not two namespaces beside a router: the transit target is an address on no link of the router, so an untranslated connection to it is never answered. A second control applies the same term without the match and the transit connection reaches the router's service, so the transit assertion could have gone the other way. It covers IPv4; the ip6 lowering is the same expressions. It carries `//go:build linux` and `userns.Enter`, as every test of the nft package does.
  3. A contribution failure with no caller to answer (the restore after a refused set, the withdrawal after a process ended) is logged and raised with a new firewall code, `doctor-firewall-contributed-tables`, which the doctor check `firewall-contributed-tables` reports. A commit observer clears it at the next successful reconcile of any owner, when the kernel holds the registry's merged set. The Problem classes table holds container codes only, so this code is not in it.
  4. A config apply needed no change to keep contributed tables: `registerConfigTables` registers only the firewall engine's two owners. `TestContributionReplacesOwnersCompleteSet` proves it.
  5. The stop path needed no change: with a `firewall` section the engine's orderly stop calls `FlushAllTables`, which clears every owner. Without one the firewall engine does not run, and contributed tables leave only through the generation cleanup when the plugin process ends. Whether that cleanup runs at an orderly daemon stop was not read, so A-6 stays open, and the table wiring draft carries a `firewall { backend nft }` section.
  6. `TestGenerationCleanupWithdrawsOwner` lives in `contribution_replace_test.go`, not `contribution_test.go`.
  7. Scope widened by one line: the web firewall page (`internal/component/web/page_firewall.go`) names the new match as `fib daddr type local` rather than its Go type through the default arm. The kernel enrolment of the fib expression (A-7) stays with phase 6.

### Owner questions

| ID | Question | Status |
|----|----------|--------|
| Q-1 | Configured interface names inside a prefix a plugin claimed for its devices are refused when the config is parsed. Is that reservation acceptable, or should the clash be refused only when a plugin asks for that name? | open |

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
| A published port must also be allowed by the operator's firewall policy (owner ruling) | The plugin adds an accept term | The feature never writes into a table the operator owns, and a drop in another base chain is final. Doctor reports a published port that policy blocks |
| A container network has no VRF node (owner ruling, with the operator-surface child) | A node the plugin's verifier refuses; `ze:validate "unimplemented-vrf"` on a node | An unknown keyword is the refusal the umbrella states, and it needs no code. The validator runs only under roots in a literal list in config code |
| A bridge per network (owner ruling) | One routed link per container | Containers of one network share a segment, as operators expect. The cost is the limitation on matching one container by interface |
| A link needs no call at container exit | Removing the link in the ended call | The kernel removes the pair with the namespace, and the interface component forgets a record whose device is gone. This holds for links only: the runtime child makes its ended call for every container it created, and this child uses that call to remove the instance's resolver file (seam row 3) |
| Marked devices are deleted by the interface engine's stop path | Each owner releases at its own stop | Plugins stop together with no order, so an owner's release races the backend close. The firewall solved the same race the same way |
| Assignments are stored | Assign the lowest free address at each start | A restart in another order would move a container's address |
| Ze does not answer container names, at first (owner ruling) | The plugin contributes names to the LAN resolver through a new registration | The resolver forwards only and has no local data today. Addresses are stable, so a name service is a separate feature |
| Resolver default is the router's list at start, loopback removed (owner ruling) | (a) No default. (b) Always the gateway address | (a) leaves a container without names on a box that learned its resolvers. (b) needs the LAN resolver, which the engine must not configure for the operator |
| Outbound translation defaults to on, with `masquerade false` to turn it off (owner ruling) | Off unless set | Operators of the reference model expect it, and a private prefix is unreachable from outside without it |
| One persistent state key holds the address assignments of a network (owner ruling) | Assign at each start with no key | A stable address needs a record, and one value written whole can grow |
| Device names: `zcn-` plus the network name, and `zc-`, the bridge's interface index and a slot for a host end (owner ruling) | Names built from the instance name | Fifteen characters cannot hold two names. The network name is bounded at 11 characters, and the listing maps an instance to its host end |

## Known Limitations
- A container network cannot be placed in a VRF: the network has no VRF node, so the keyword is refused as unknown. `plan/post-release/features/spec-vrf-0-umbrella.md` owes the container network as one of its consumers, and adds the node when VRF support ships.
- A published port is reachable only when the operator's firewall policy also allows it. Ze adds no accepting term to the operator's tables.
- The doctor check for a published port that policy blocks judges two matches of a forward chain's terms, the output interface and the destination address, and no other. It does not read the output and input hooks, so it does not report a policy that blocks a connection the router itself opens to a published port.
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
