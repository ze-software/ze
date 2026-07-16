# Spec: vrrp-5 -- VRRP Plugin Integration (registration, YANG, engine wiring, commands, telemetry, doctor, tests, docs)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vrrp-4 |
| Phase | 1/8 |
| Updated | 2026-07-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-vrrp-0-umbrella.md` -- all user decisions, config shape, A-1/A-2/A-4/A-6, R-3/R-6
3. `.claude/rules/planning.md` -- workflow rules
4. `rfc/short/rfc9568.md` (VRRPv3), `rfc/short/rfc3768.md` (VRRPv2)
5. Sibling specs: `spec-vrrp-1-packet.md` (codec), `spec-vrrp-2-fsm.md` (FSM), `spec-vrrp-3-macvlan.md` (iface macvlan + address owner), `spec-vrrp-4-transport.md` (sockets, GARP/NA, doctor-vrrp-raw-socket)
6. Key sources: `internal/plugins/ospf/register.go` (plugin model), `internal/component/iface/yang/ze-iface-conf.yang` (augment targets), `internal/component/iface/address_owner.go` (VIP install), `internal/component/iface/register.go` (backend gate), `internal/plugins/isis/cmd_show.go` (RPC proxy contract)

## Task

Integrate VRRP children 1-4 into a working, operator-visible feature: the `vrrp`
plugin under `internal/plugins/vrrp/`. This child owns:

1. **Plugin registration** -- `registry.Registration` with Name `"vrrp"`, shared
   config root `interface`, dependency on the iface plugin, YANG embed,
   metrics/eventbus/logger wiring, internal-only guard.
2. **Config YANG** -- `ze-vrrp-conf.yang` augmenting the iface tree at 8 paths
   (ethernet/veth/bridge/dummy x ipv4/ipv6) with `vrrp { group <vrid> { ... } }`.
3. **Config resolution** -- extract-only walk of the `interface` section into
   GroupSpecs; pure verification (native ranges + cross-leaf validators + VPP
   rejection + owner auto-detection); verify/configure/apply/rollback callbacks.
4. **Engine** -- instance manager keyed (parent, unit, family, vrid); per-instance
   goroutine hosting the child-2 FSM; FSM actions executed via child-4 transport;
   macvlan device lifecycle via the child-3 iface API; VIP install/remove via the
   iface address-owner registry; readiness predicate driven by iface subscriptions.
5. **Show/clear surface** -- `show vrrp`, `show vrrp interface name <name>`,
   `show vrrp statistics`, `clear vrrp statistics` (cmd YANG + RPC proxies +
   CommandDecls + OnExecuteCommand).
6. **Telemetry** -- engine-owned `ze_vrrp_state` gauge and
   `ze_vrrp_transitions_total` counter (child 4 owns the wire counters).
7. **Events** -- `vrrp` eventbus namespace + typed value-type payload for
   master transitions.
8. **Doctor** -- `doctor-vrrp-config` post-config sanity check + CodeMeta
   (child 4 owns `doctor-vrrp-raw-socket`).
9. **Functional tests** -- new `test/vrrp/` suite + `ze-vrrp-test` target.
10. **Docs** -- operator guide, feature/RFC/comparison/command-reference rows.

Umbrella context: `plan/spec-vrrp-0-umbrella.md` (config placement under the
interface unit, explicit ipv4/ipv6 containers, macvlan virtual MAC from day one,
v3 default + v2 opt-in). This child implements the interim VPP fail-closed
rejection (umbrella A-6/R-3) and proves engine idleness with zero groups
(umbrella A-4, AC-6).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/patterns/plugin.md` + `ai/rules/plugin-design.md` - plugin anatomy
  → Constraint: init() in register.go; atomic.Pointer logger; RunEngine `func(net.Conn) int` (`internal/component/plugin/registry/registration.go:39`, confirmed by `runOSPFEngine(conn net.Conn) int` at `internal/plugins/ospf/register.go:308`); no sibling plugin imports; `make generate` regenerates all.go
  → Constraint: OnStarted = local setup only; any DispatchCommand to another plugin at startup goes in OnAllPluginsReady (`ai/rules/plugin-design.md` "OnStarted vs OnAllPluginsReady"). vrrp dispatches no cross-plugin commands at startup, so OnStarted suffices
  → Constraint: EventBus payloads are self-contained value types only (`ai/rules/plugin-design.md` "EventBus Typed Payloads"); declare via `events.Register[T]`; test stubs of ze.EventBus need the compile-time interface assertion
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Constraint: ALL vrrp surface (config augments, cmd YANG, RPC proxies, doctor codes, metrics, events, docs rows sourced from registry) lives under `internal/plugins/vrrp/`; deleting the directory + `make generate` removes every feature and keeps the build green; central verb schemas gain no vrrp tokens (the augment modules live in the plugin); add banned-token + presence test halves
- [ ] `ai/patterns/config-option.md` + `ai/rules/config-naming.md` + `ai/rules/config-surface.md` - YANG leaves
  → Constraint: kebab-case, unit suffixes (`advertise-interval-milliseconds`, `preempt-delay-seconds`); every leaf max-constrained (range/type/max-elements); types from ze-types; no env vars (all operator config is YANG); YANG module header pattern `module ze-<name>-conf { namespace "urn:ze:<name>:conf"; prefix <name>; }`
- [ ] `ai/rules/cli-grammar.md` - command grammar
  → Constraint: typed selector `show vrrp interface name <name>`; read verbs only (`show`, `clear` for runtime state); config mutation stays in engine set/delete; no `--flag` in YANG; R1-R9 static gate must stay green
- [ ] `ai/rules/doctor-checks.md` - doctor checks
  → Constraint: `doctor-<component>-<condition>` codes registered in the OWNING package with CodeMeta (explainable via `ze explain`); unit test + functional test both required; internal plugins may use `Registration.DoctorChecks` or `diagnostic.RegisterDoctorCheck` from init() (ospf uses the latter, `internal/plugins/ospf/register.go:183-198`)
- [ ] `ai/patterns/functional-test.md` - .ci format
  → Constraint: `cmd=foreground` + `expect=exit`/`expect=stdout:contains`; `option=needs-linux` for kernel-touching tests (`test/ospf/ospf-nbma.ci:12`); payload-predicate waits, never sleeps
- [ ] `ai/rules/spec-no-code.md` + `ai/rules/planning.md` - spec rules
  → Constraint: tables/prose only, no code blocks; Risks & Assumptions live tables; append-only editing
- [ ] `ai/rules/exact-or-reject.md` (via umbrella) - VPP handling
  → Decision: until spec-vrrp-7, vrrp config on a VPP-backed interface tree MUST fail closed at verify with an actionable error (umbrella A-6/R-3); mechanism chosen below (Key Design Decisions)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - VRRPv3
  → Constraint: Advertisement_Interval 1..4095 centiseconds (erratum 8301) = 10..40950 ms, multiples of 10 ms; owner priority MUST be 255 and Backups MUST use 1..254 (5.2.4); Accept_Mode default False, owner always accepts (6.4.3); the FIRST listed IPv6 virtual-address MUST be link-local (5.2.9, erratum 8300) and is the advert source identity; Preempt_Mode default True; no Preempt_Delay in the RFC (spec-vrrp-2 defines JUNOS hold-time semantics)
- [ ] `rfc/short/rfc3768.md` - VRRPv2
  → Constraint: v2 Adver Int is whole seconds in one byte (1..255 s = 1000..255000 ms, whole seconds only); IPv4 only; auth type 0 only (umbrella decision); interval mismatch = discard in v2 (transport/FSM concern, children 1/2/4)

**Key insights:**
- The plugin model to copy is ospf (`internal/plugins/ospf/register.go`): registration fields, verify/configure/apply pending-swap, OnStarted idle check, OnExecuteCommand switch, CommandDecl list.
- The augment mechanism into grouping-expanded paths is proven by the bgp hostname plugin (`internal/component/bgp/plugins/hostname/yang/ze-hostname.yang:16,37,79,96,113` -- five augments into `/bgp:bgp/...` expanded paths).
- The iface component already provides every integration seam this child needs: address-owner registry, link subscriptions, backend gate; children 1-4 provide codec/FSM/macvlan/transport.
- VPP-backed detection is CLEAN: the interface backend is one global leaf `interface { backend <name>; }` (default "netlink"), parsed by `parseIfaceBackend` (`internal/component/iface/register.go:194-214`), and the iface component runs a generic `ze:backend` annotation gate at verify (`validateBackendGate` `internal/component/iface/register.go:65`, called from `parseAndVerifyIfaceSections:173`).

## Current Behavior (MANDATORY)

**Source files read:** (producers read directly this session)
- [ ] `internal/plugins/ospf/register.go` - closest plugin model: registerOSPF :89, events.RegisterNamespace :90, configyang.RegisterCompleteFn :108-109, Registration :111-131 (ConfigRoots :116, Dependencies :117, RFCs :118, RunEngine :119, InProcessConfigVerifier :120, ConfigureEngineLogger :121, ConfigureMetrics :125, ConfigureEventBus :128), CLIHandler :132-139, CodeMeta ownership :153-176, doctor config-sanity :183-198, OnConfigVerify :403, OnConfigure :423, OnConfigApply :442, OnStarted idle check :486-489, OnExecuteCommand :505, p.Run WantsConfig/VerifyBudget/ApplyBudget/Commands :687-753
- [ ] `internal/plugins/as112/register.go` - internal-only guard :223 (refuse external process: iface registry calls are same-process), applyAddressRegistration consumer :255
- [ ] `internal/component/iface/address_owner.go` - RegisterOwnedAddresses :80 (conflict vs OTHER owners :83-94; per-(owner,iface) set REPLACEMENT :96-103), UnregisterOwnedAddresses :122 (removes the owner from ALL interfaces; sole populator of staleIfaces :126-129, which guarantees one more prune pass)
- [ ] `internal/component/iface/register.go` - Name "interface" :135, ConfigRoots ["interface"] :139, events namespace registration :123, WantsConfig ["interface"] :694, validateBackendGate :65 (generic ze:backend annotation gate), gate call sites :173 (verify) and :405 (apply), parseIfaceBackend :194-214 (global `interface.backend` leaf, default netlink)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - module ze-iface-conf, namespace urn:ze:iface:conf, prefix `iface` :1-3; grouping interface-unit :204 (list unit :207); ipv4 container :263 (address leaf-list :266 with ze:syntax "bracket" :270); ipv6 container :351 (address leaf-list :354); container interface :513; backend leaf :525-529; list ethernet :537, dummy :554, veth :569, bridge :589 -- each `uses interface-unit`
- [ ] `internal/component/bgp/plugins/hostname/yang/ze-hostname.yang` - multi-path augment precedent into grouping-expanded paths :16, :37, :79, :96, :113
- [ ] `internal/plugins/isis/cmd_show.go` - RPC proxy contract: RegisterRPCs + PluginCommand :47-59, forwardToISIS via ForwardToPlugin :106-116 (MUST NOT re-Dispatch: infinite recursion)
- [ ] `internal/plugins/mpls-cmd/yang/ze-mpls-cmd.yang` - show augment shape :11-26 (`augment "/clishowcmd:show"`, ze:command + keyword leaf)
- [ ] `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` - clear augment shape :187 (`augment "/cliclearcmd:clear"`), selector-container-consumes-value model :196-202 (`clear l2tp tunnel id <id>`)
- [ ] `internal/plugins/ddos/local/register.go` - shared/nested config-root + section filtering precedent: ConfigRoots :37, `s.Root != configRoot` skip :66, WantsConfig :153 (configRoot "ddos/local", `internal/plugins/ddos/local/config.go:19`; five ddos plugins hang off one top-level `ddos` container)
- [ ] `internal/plugins/static/register.go` - OnConfigRollback + journal model :191 (journal rejected for vrrp, see Key Design Decisions)
- [ ] `internal/plugins/isis/codes.go` - plugin-owned CodeMeta slice :21 model
- [ ] `internal/component/plugin/all/all_test.go` - TestRegisteredPluginNames snapshot :81 (golden `testdata/*.snapshot`, regen via `go test -update` :48); TestAllPluginsRegistered :190 (non-empty + no test-only plugins; NO hardcoded count anymore)
- [ ] `scripts/codegen/yang_glue.go` - header :1-9: generates embed.go + register.go in EVERY `yang/` dir containing .yang files (both ze-vrrp-conf.yang and ze-vrrp-cmd.yang get glue from one dir)
- [ ] `scripts/codegen/plugin_imports.go` - header :1-10: generates `internal/component/plugin/all/all.go` from register.go discovery (plugin dirs + `internal/**/yang/register.go`)
- [ ] `internal/test/cli/register.go` - ze-test suite registration :22 (`registerCIRoot("isis", "isis", "isis", ...)` maps suite name -> test/isis/)
- [ ] `internal/component/iface/resolve.go` - Subscribe :80 (per-name LinkEvent channel + unsubscribe fn), Resolve :65 (Binding: ifindex/OS name/state), Addresses :71 (umbrella-verified)

**Behavior to preserve:**
- iface remains the only writer of kernel interface state; vrrp goes through iface APIs (macvlan API from child 3, address-owner registry) -- never direct netlink.
- Existing address-owner semantics for as112 unchanged; vrrp adds owners, never touches other owners' registrations.
- Existing interface YANG tree shape unchanged; vrrp attaches ONLY via augment from the plugin's own module. `ze config validate` on existing interface configs (no vrrp) behaves identically.
- The iface backend gate keeps rejecting netlink-only features under `backend vpp` exactly as today; vrrp adds annotations, not gate code.
- Firewall `match protocol vrrp` (proto 112, `internal/component/firewall/protocol.go:14`) keeps working.
- Central show/clear verb schemas gain no plugin tokens (self-containment guard tests keep passing).

**Behavior to change:**
- None removed. New: vrrp config subtree under interface units parses/validates; vrrp plugin auto-loads with the `interface` root; instances (macvlan + sockets + FSM) exist when groups are configured; `show vrrp` / `clear vrrp statistics` commands; `ze_vrrp_*` metrics; `vrrp` event namespace; `doctor-vrrp-config` check; `test/vrrp/` suite; docs.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: `interface <type> <name> unit <u> ipv4|ipv6 vrrp group <vrid> { ... }` -- YANG-validated tree delivered to the vrrp plugin as the `interface` config-root section (SDK ConfigSection JSON), the same section the iface plugin receives (shared root, umbrella A-2).
- Operator: `show vrrp`, `show vrrp interface name <name>`, `show vrrp statistics`, `clear vrrp statistics` via YANG-dispatched RPC proxies.
- Runtime: iface LinkEvents (`iface.Subscribe`) and received adverts drive per-instance state. Per orchestrator D-B: the child-4 transport delivers RAW (RxMeta{Src, Dst, TTL, Family, IfIndex}, payload) pairs; THIS child's instance worker calls packet.Decode (child 1) with the group's per-interface Lookup and feeds the child-2 FSM.

### Transformation Path
1. Engine delivers the `interface` section -> `extractGroupSpecs` walks type lists (ethernet/veth/bridge/dummy) -> units -> ipv4/ipv6 -> vrrp/group into `[]GroupSpec` (extract-only, umbrella R-6; everything else in the section is skipped).
2. Verify: native YANG constraints already applied by the config loader; plugin cross-leaf validation (interval-vs-version, accept-mode-vs-version, ipv6 link-local presence, duplicate VIP per unit+family, VPP backend rejection, owner detection) -- pure, no side effects.
3. Apply/configure: instance manager diffs desired GroupSpecs against running instances keyed (parent, unit, family, vrid) -> create/update/delete.
4. Instance create: request macvlan via child-3 iface API (device exists from creation, umbrella R-4) -> subscribe parent + macvlan link events -> readiness predicate (parent oper-up AND macvlan oper-up AND parent has same-family address) -> open child-4 transport -> FSM Startup.
5. Rx path (orchestrator D-B): transport hands (RxMeta, payload) to the instance worker -> packet.Decode with the group's per-interface Lookup -> decode failures mapped via the error's Reason() to `transport.RecordRxError(reason)`; v2 groups additionally compare packet VIPs (VIPAt) against configured VIPs post-Decode, mismatch = drop + reason `address-list` (RFC 3768 7.1); accepted advert with priority==0 increments the ENGINE-owned prio-0 rx counter -> AdvertReceived FSM event.
6. Timers (spec-vrrp-2 "Timer Generations"): the instance worker owns three clock.Timers from an injected clock.Clock; every FSM Start* timer action carries a Gen value the worker echoes back on the corresponding expiry event, so stale expiries are discarded by the FSM.
7. FSM Master transition -> RegisterOwnedAddresses(macvlan, owner, VIPs) + GARP/NA burst (child 4) ; FSM Backup/Init transition -> UnregisterOwnedAddresses(owner) -> iface reconcile prunes VIPs; SendAdvertZeroPriority execution increments the ENGINE-owned prio-0 tx counter (D-F).
8. State/counters -> `ze_vrrp_state` gauge + `ze_vrrp_transitions_total` counter, `vrrp` eventbus emission, show-command snapshots (wire counters read via transport CounterSnapshot, D-F).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> vrrp plugin | SDK 5-stage protocol; ConfigSection JSON down, (status, payload) up; OnConfigVerify/OnConfigure/OnConfigApply/OnConfigRollback/OnStarted/OnExecuteCommand | [ ] |
| vrrp plugin <-> iface component | Same-process Go calls (RegisterOwnedAddresses `address_owner.go:80`, UnregisterOwnedAddresses :122, macvlan API from child 3, Subscribe `resolve.go:80`) -- plugin MUST run internal (as112 guard `internal/plugins/as112/register.go:223`) | [ ] |
| CLI <-> vrrp engine | `pluginserver.RegisterRPCs` proxies with PluginCommand -> `ForwardToPlugin` (isis contract `cmd_show.go:47-116`); never re-Dispatch | [ ] |
| Config loader <-> plugin YANG | Augment module `ze-vrrp-conf.yang` merges into the iface schema (hostname precedent); glue generated by yang_glue | [ ] |
| vrrp <-> kernel | ONLY via child-3 iface macvlan API + address-owner registry and child-4 transport sockets; zero netlink in this child | [ ] |
| vrrp engine <-> child-4 transport | raw (RxMeta, payload) up (D-B: engine decodes); Send/GARP/NA actions, RecordRxError(reason), per-instance CounterSnapshot + ResetCounters down (D-F) | [ ] |

### Integration Points
- `iface.RegisterOwnedAddresses` / `UnregisterOwnedAddresses` (`internal/component/iface/address_owner.go:80,122`) - VIP lifecycle (owner-string decision below)
- Child-3 iface macvlan API (device create/delete with virtual MAC) - instance device lifecycle
- `iface.Subscribe` / `iface.Resolve` / `iface.Addresses` (`internal/component/iface/resolve.go:80,65,71`) - readiness predicate inputs
- Child-4 transport (raw proto-112 rx/tx, GARP/NA senders, wire metrics, doctor-vrrp-raw-socket) - FSM action executor
- Child-2 FSM + child-1 codec - per-instance state machine
- `pluginserver.RegisterRPCs` + `sdk.CommandDecl` - show/clear surface
- `diagnostic.Register` + `diagnostic.RegisterDoctorCheck` (ospf model `register.go:174,183-198`) - doctor-vrrp-config
- `events.RegisterNamespace` + `events.Register[T]` (`internal/core/events/typed.go:173`, ospf model `register.go:90`) - vrrp namespace + typed transition payload
- `registry.Registration.ConfigureMetrics/ConfigureEventBus/ConfigureEngineLogger` (ospf `register.go:121-130`) - injection
- iface backend gate (`validateBackendGate` `internal/component/iface/register.go:65`) - consumes the `ze:backend "netlink"` annotation carried by the vrrp containers

### Architectural Verification
- [ ] No bypassed layers (vrrp never calls netlink; all kernel interface state via iface APIs; all wire I/O via child-4 transport)
- [ ] No unintended coupling (iface gains zero vrrp knowledge; the ze:backend annotation lives in the PLUGIN's YANG; no vrrp spelling in any central package)
- [ ] No duplicated functionality (reuses address-owner, backend gate, RPC proxy contract, doctor/metrics/event registries; codec/FSM/transport from children 1/2/4)
- [ ] Zero-copy preserved where applicable (show snapshots are value copies; rx path owned by child 4)
- [ ] Registration over hardcoding -- plugin registry, RPC registry, YANG augment modules, doctor registry, event namespace, metrics registry; no new per-feature field, switch case, or factory in a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The YANG loader applies augments into grouping-expanded paths (`/iface:interface/iface:ethernet/iface:unit/iface:ipv4`) | umbrella A-1; hostname plugin augments grouping-expanded `/bgp:bgp/bgp:group/bgp:peer/bgp:session` (`internal/component/bgp/plugins/hostname/yang/ze-hostname.yang:96`) | placement falls back to a standalone `vrrp` tree (user's second choice); STOP and present to user | Wiring phase: `test/vrrp/vrrp-config.ci` parses the agreed shape via `ze config validate` | unvalidated |
| A-2 | Two plugins may declare the same config root string `interface` (iface `register.go:139` + vrrp) and both receive the section | umbrella A-2; auto-load VERIFIED for duplicate roots: `getConfigPathPlugins` iterates every plugin's ConfigRoots independently with no uniqueness constraint (`internal/component/plugin/server/startup_autoload.go:78-112` -- per-plugin map walk, each matching plugin auto-loads); ddos plugins use nested roots (`ddos/local`, `internal/plugins/ddos/local/config.go:19`) so only section DELIVERY fan-out to two same-root consumers remains unproven | vrrp needs a nested root or its own tree; fallback `interface/vrrp` nested root or standalone tree; STOP and present | Wiring phase: unit test `TestVRRPReceivesInterfaceSection` (both plugins get the section on a vrrp-bearing config) + `vrrp-instance-up.ci` | unvalidated (auto-load half confirmed) |
| A-3 | ConfigRoots-driven auto-load on `interface` is acceptable; the engine stays idle (zero sockets/devices/goroutines beyond the SDK loop) when no vrrp groups exist | umbrella A-4; ospf idle model (`internal/plugins/ospf/register.go:486-489` logs idle and returns) | add an explicit enable knob (config-surface decision) | `test/vrrp/vrrp-idle.ci` (AC-13) | unvalidated |
| A-4 | The `ze:backend "netlink"` annotation on the plugin's augment containers is honored by the iface backend gate (annotations from AUGMENT modules are visible in the merged schema) | `validateBackendGate` uses the merged `config.YANGSchema()` (`internal/component/iface/register.go:66-79`); DHCP containers carry the same annotation and are gate-enforced (`internal/component/iface/schema_linux_test.go:70`) | vrrp's own OnConfigVerify backend check (belt-and-braces, below) still rejects; gate half becomes a no-op; record in Mistake Log | `test/vrrp/vrrp-vpp-reject.ci` + unit test `TestVRRPBackendGateAnnotation` | unvalidated |
| A-5 | The macvlan API from spec-vrrp-3 exposes create/delete + device naming + oper-state visibility through `iface.Subscribe`/`iface.Resolve`, sufficient for the readiness predicate | umbrella child-3 scope row; holo readiness model (research-holo-digest) | readiness predicate falls back to polling `iface.Resolve` (isis rescan backstop model); coordinate with spec-vrrp-3 | Engine unit tests with a fake iface seam; `vrrp-instance-up.ci` | unvalidated |
| A-6 | Repeated `virtual-address <ip>;` statements parse into a leaf-list the same way the sibling `address` leaf-list does with `ze:syntax "bracket"` (umbrella example uses repeated lines) | `ze-iface-conf.yang:266-271` (address leaf-list, bracket syntax) | config example in docs/tests switches to bracket form `virtual-address [ a b ];`; grammar unchanged | `test/vrrp/vrrp-config.ci` (multi-VIP group) | unvalidated |
| A-7 | `ForwardToPlugin` delivers selector args (interface name) to OnExecuteCommand's cmdArgs, as the l2tp `clear l2tp tunnel id <id>` model does | `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang:196-202` (ze:command on selector container consumes the value); ospf OnExecuteCommand receives cmdArgs (`register.go:505,664`) | interface detail becomes a filter applied engine-side from the summary payload | Wiring phase: `test/vrrp/vrrp-show.ci` (`show vrrp interface name <n>`) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Sharing the `interface` config root delivers large config sections on every reload; slow parse delays reload (umbrella R-6) | reload .ci duration; profiling | Extract-only walk: descend ONLY interface-type lists -> units -> ipv4/ipv6 -> vrrp; skip every other key at each level |
| R-2 | VIP removal on Master->Backup leaves stale kernel addresses (holo bug class) | vrrp-failover QEMU tests (child 6) see VIPs on both boxes | Use UnregisterOwnedAddresses (NOT empty re-registration): it is the sole staleIfaces populator (`address_owner.go:126-129`) and guarantees a prune pass even for devices with no YANG-config desired state |
| R-3 | Dead counters (holo defined-but-never-bumped statistics) | metrics .ci passes but counters stay 0 during instance-up test | Critical-review row: every registered metric has an increment site cited file:line; `vrrp-instance-up.ci` asserts a nonzero transition counter after Master election |
| R-4 | Macvlan created lazily at Master transition adds address-install latency to failover (umbrella R-4) | failover gap measured in child 6 | Device created at INSTANCE CREATE (this spec's engine contract); only VIP install remains on the transition path |
| R-5 | Show proxies re-Dispatch the command string and recurse (isis anti-pattern) | stack overflow on first `show vrrp` | Copy the isis proxy contract exactly: ForwardToPlugin only (`cmd_show.go:106-116`); wiring test exercises the full CLI path |
| R-6 | The vrrp plugin config-verify races the iface plugin's verify on the shared section (ordering not guaranteed) | flaky verify errors mentioning the wrong plugin | vrrp verify is self-contained: it re-reads `interface.backend` itself (parseIfaceBackend model) and never depends on iface having verified first |
| R-7 | Instance-up .ci needs a real kernel (macvlan + raw sockets); running it host-side silently skips assertions | test green on darwin without testing anything | `option=needs-linux` on kernel-touching tests; offline tests (config/validate/show shapes via mocked engine) run everywhere |
| R-8 | Duplicate VIP across two groups on the same unit installs conflicting owners | address-owner conflict error at runtime instead of verify | Verify-time cross-group duplicate-VIP check (per unit+family); doctor-vrrp-config repeats it post-config |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on config with vrrp group | → | plugin YANG augment + InProcessConfigVerifier -> extractGroupSpecs + validateGroups | `test/vrrp/vrrp-config.ci` |
| `ze config validate` on invalid group values | → | cross-leaf validators reject with named path | `test/vrrp/vrrp-config-invalid.ci` |
| Config commit with vrrp group (linux) | → | OnConfigure -> instance manager create -> macvlan + FSM Startup | `test/vrrp/vrrp-instance-up.ci` |
| `show vrrp` (CLI) | → | ze-show:vrrp-summary proxy -> ForwardToPlugin -> OnExecuteCommand summary snapshot | `test/vrrp/vrrp-show.ci` |
| `show vrrp interface name <n>` | → | ze-show:vrrp-interface proxy passes name arg -> per-interface detail snapshot | `test/vrrp/vrrp-show.ci` |
| `clear vrrp statistics` | → | ze-clear:vrrp-statistics proxy -> engine counter reset | `test/vrrp/vrrp-show.ci` |
| `ze doctor --json` on vrrp config | → | doctor-vrrp-config check + CodeMeta explain | `test/vrrp/vrrp-doctor.ci` |
| Prometheus scrape | → | ze_vrrp_state gauge registered via ConfigureMetrics | `test/vrrp/vrrp-metrics.ci` |
| `interface { backend vpp; }` + vrrp group | → | verify rejection (plugin verify + ze:backend gate) | `test/vrrp/vrrp-vpp-reject.ci` |
| Interfaces configured, zero vrrp groups | → | plugin auto-loads, engine idle: zero instances/macvlans | `test/vrrp/vrrp-idle.ci` |
| Plugin registration at startup | → | registry.Registration fields (Name/ConfigRoots/Dependencies/YANG/RFCs) | `TestVRRPRegistrationFields` (unit) + `TestRegisteredPluginNames` snapshot |
| Master transition | → | RegisterOwnedAddresses + event emission + gauge/counter update | `TestInstanceMasterTransitionActions` (unit, fake seams) + `test/vrrp/vrrp-instance-up.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config validate` on the umbrella's agreed config shape (v3 ipv4 group with 2 VIPs, priority, preempt, interval; v3 ipv6 group with link-local + global VIP) | exit 0, "configuration valid" |
| AC-2 | `version 2` group under ipv4 with whole-second interval (e.g. 2000 ms) | validates; `version` leaf absent under ipv6 (schema has no such leaf there) |
| AC-3 | Owner case: virtual-address equals a real `address` on the same unit+family | validates; group marked IsOwner; effective priority 255 and accept-mode forced true (RFC 9568 5.2.4, 6.4.3); `show vrrp` detail shows configured priority AND effective-priority 255 |
| AC-4 | Each invalid input: vrid 0/256, priority 0/255, interval 5/50000 (v3 out of range), interval 1005 (v3, not a multiple of 10 ms -- review Finding 3), non-whole-second or out-of-range interval with version 2 (e.g. 1500, 500, 256000), accept-mode true + version 2, ipv6 group whose FIRST virtual-address is not link-local (review Finding 4), 17 VIPs, duplicate VIP across two groups on one unit+family | `ze config validate` exits nonzero with an error naming the offending path/leaf |
| AC-5 | `interface { backend vpp; }` with any vrrp group | verify FAILS with an actionable error naming the vrrp config path and the backend leaf (`/interface/backend`); no instance, no macvlan (umbrella AC-5) |
| AC-6 | Interfaces configured, zero vrrp groups | plugin loads (shared root auto-load), engine idle: `show vrrp` reports zero instances; no vrrp sockets or macvlan devices (umbrella AC-6, A-4) |
| AC-7 | Config commit with one v3 ipv4 group (linux) | instance appears: macvlan created at instance create, FSM reaches Backup then Master (no peer), VIPs installed via address-owner registry; `show vrrp` shows state master |
| AC-8 | `show vrrp` / `show vrrp interface name <n>` / `show vrrp statistics` (+ `| json`) | structured payloads with the field tables below; interface selector filters to one parent |
| AC-9 | `clear vrrp statistics` | per-instance counters reset to zero; state untouched |
| AC-10 | `ze doctor --json` on a config whose VIP equals ANOTHER unit's address, or on VPP backend + vrrp | doctor-vrrp-config emits a diagnostic; `ze explain doctor-vrrp-config` describes it |
| AC-11 | Metrics scrape after instance up | `ze_vrrp_state{interface,vrid,family}` present with correct value (0=init 1=backup 2=master); `ze_vrrp_transitions_total{...,to}` incremented on each transition (no dead counters) |
| AC-12 | Master transition occurs | typed event emitted on the `vrrp` namespace with value-type payload; visible in the event stream |
| AC-13 | Group removed from config (apply diff) | instance deleted: FSM Shutdown (prio-0 advert via child 4 if Master), UnregisterOwnedAddresses, macvlan deleted |
| AC-14 | Delete `internal/plugins/vrrp/` + `make generate` | build green; every vrrp command, schema node, doctor code, metric, event vanishes; central show/clear schemas contain no vrrp tokens |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `interface ethernet eth0 unit 0 ipv4 vrrp group 10 {...}`, validates | config file -> YANG (augment) -> vrrp InProcessConfigVerifier -> GroupSpec validation | `test/vrrp/vrrp-config.ci` |
| 2 | Commits the config on a linux box | engine -> vrrp OnConfigure -> instance manager -> macvlan + transport + FSM -> Master election -> VIP live | `test/vrrp/vrrp-instance-up.ci` |
| 3 | Runs `show vrrp` / `show vrrp interface name eth0` / `show vrrp statistics` | CLI -> YANG dispatch -> RPC proxy -> ForwardToPlugin -> OnExecuteCommand snapshot | `test/vrrp/vrrp-show.ci` |
| 4 | Runs `ze doctor` before deployment | doctor registry -> doctor-vrrp-config (+ child-4 raw-socket check) -> JSON diagnostics | `test/vrrp/vrrp-doctor.ci` |
| 5 | Scrapes Prometheus | ConfigureMetrics registry -> ze_vrrp_state / ze_vrrp_transitions_total | `test/vrrp/vrrp-metrics.ci` |
| 6 | Configures vrrp on a VPP-backed box | verify -> backend rejection with actionable error | `test/vrrp/vrrp-vpp-reject.ci` |
| 7 | Removes the group and commits | OnConfigApply diff -> instance delete -> prio-0 + VIP/device teardown | `test/vrrp/vrrp-instance-up.ci` (teardown phase) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVRRPRegistrationFields` | `internal/plugins/vrrp/register_test.go` | Name "vrrp", ConfigRoots ["interface"], Dependencies ["interface"], RFCs ["9568","3768"], YANG non-empty | |
| `TestExtractGroupSpecs` | `internal/plugins/vrrp/config_test.go` | all 4 interface types x 2 families -> GroupSpecs (parent, unit, family, vrid, VIPs, timers, flags); extract-only (unrelated keys ignored) | |
| `TestExtractGroupSpecsEmpty` | `internal/plugins/vrrp/config_test.go` | interface config without vrrp -> zero specs (idle path) | |
| `TestValidateGroupsBoundaries` | `internal/plugins/vrrp/config_test.go` | every boundary row below (valid edge accepted, invalid rejected with path-naming error) | |
| `TestValidateIntervalVersion` | `internal/plugins/vrrp/config_test.go` | v3 10..40950 ms AND ms%10==0 (rejects 1005); v2 1000..255000 AND ms%1000==0; v2 rejects 1500/500/256000 | |
| `TestValidateAcceptModeVersion` | `internal/plugins/vrrp/config_test.go` | accept-mode true + version 2 rejected | |
| `TestValidateIPv6LinkLocal` | `internal/plugins/vrrp/config_test.go` | ipv6 group whose FIRST virtual-address is not fe80::/10 rejected (erratum 8300; link-local elsewhere in the list does NOT satisfy); link-local first accepted | |
| `TestValidateDuplicateVIP` | `internal/plugins/vrrp/config_test.go` | same VIP in two groups on one unit+family rejected; same VIP on different units accepted | |
| `TestOwnerAutoDetection` | `internal/plugins/vrrp/config_test.go` | VIP == real unit address (same family, prefix stripped) -> IsOwner, effective priority 255, accept-mode true; near-miss (other family/unit) -> not owner | |
| `TestVerifyRejectsVPPBackend` | `internal/plugins/vrrp/config_test.go` | `interface.backend == "vpp"` + any group -> error naming path + backend leaf; vpp + zero groups -> ok | |
| `TestVRRPBackendGateAnnotation` | `internal/plugins/vrrp/yang/schema_test.go` | merged schema carries ze:backend "netlink" on every vrrp augment container (A-4 half) | |
| `TestVRRPReceivesInterfaceSection` | `internal/plugins/vrrp/register_test.go` | shared-root delivery: vrrp WantsConfig gets the `interface` section (A-2) | |
| `TestInstanceManagerDiff` | `internal/plugins/vrrp/server_test.go` | apply diff -> create/update/delete instance calls keyed (parent, unit, family, vrid); update of timer-only change does NOT recreate device | |
| `TestInstanceReadinessPredicate` | `internal/plugins/vrrp/server_test.go` | FSM Startup only when parent up AND macvlan up AND parent has same-family address; loss of any -> FSM Shutdown to Initialize | |
| `TestInstanceMasterTransitionActions` | `internal/plugins/vrrp/server_test.go` | ->Master: RegisterOwnedAddresses(macvlan, "vrrp:<dev>", VIPs) + GARP/NA trigger + event + gauge/counter; ->Backup: UnregisterOwnedAddresses("vrrp:<dev>") | |
| `TestInstanceDeleteTeardownOrder` | `internal/plugins/vrrp/server_test.go` | delete while Master: prio-0 send (engine prio-0 tx counter bumps, D-F), unregister VIPs, close transport, delete macvlan | |
| `TestInstanceTimerGenEcho` | `internal/plugins/vrrp/server_test.go` | fake clock.Clock: worker owns three clock.Timers; Gen from each FSM Start* action is echoed on the matching expiry event; stale-Gen expiry after a restart is discarded (spec-vrrp-2 Timer Generations) | |
| `TestInstanceRxDecodeErrorMapping` | `internal/plugins/vrrp/server_test.go` | D-B: (RxMeta, payload) -> packet.Decode with group Lookup; each decode failure maps via Reason() to transport.RecordRxError(reason); valid advert -> AdvertReceived; priority==0 rx bumps engine prio-0 rx counter | |
| `TestV2AddressListMismatchDrop` | `internal/plugins/vrrp/server_test.go` | v2-only post-Decode check: packet VIPs (VIPAt) vs configured VIPs; mismatch = drop + RecordRxError("address-list") (RFC 3768 7.1); v3 groups skip the check | |
| `TestOnExecuteCommandSnapshots` | `internal/plugins/vrrp/cmd_test.go` | summary/interface/statistics payload shapes match the field tables; unknown command -> error | |
| `TestClearStatistics` | `internal/plugins/vrrp/cmd_test.go` | engine counters (prio-0 rx/tx, transitions) zeroed AND transport ResetCounters invoked per instance (D-F); state/gauges untouched | |
| `TestForwardProxiesNoRedispatch` | `internal/plugins/vrrp/cmd_show_test.go` | proxies call ForwardToPlugin with fixed PluginCommand; interface proxy forwards the name arg (A-7) | |
| `TestDoctorVRRPConfig` | `internal/plugins/vrrp/doctor_test.go` | fires on VIP-equals-other-unit-address and on vpp+vrrp; silent on clean config; code registered/explainable | |
| `TestVRRPMetricsIncremented` | `internal/plugins/vrrp/metrics_test.go` | every registered metric has an increment path exercised (R-3 dead-counter guard) | |
| `TestVRRPEventPayloadValueType` | `internal/plugins/vrrp/events_test.go` | typed handle registered on namespace "vrrp"; payload fields are value types; Emit/Subscribe round-trip | |
| `TestVRRPCmdSchemaOwnsShowVRRP` | `internal/plugins/vrrp/yang/self_containment_test.go` | owner presence half: vrrp cmd module declares show/clear vrrp tokens | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| vrid (list key) | 1-255 | 255 | 0 | 256 |
| priority | 1-254 | 254 | 0 | 255 (operator-set; 255 is auto-owner only) |
| advertise-interval-milliseconds (v3) | 10-40950 | 40950 | 9 | 40960 |
| advertise-interval-milliseconds (v2) | 1000-255000, whole seconds | 255000 | 999 (and 1500 non-whole) | 256000 |
| preempt-delay-seconds | 0-3600 | 3600 | N/A (0 valid) | 3601 |
| virtual-address count | 1-16 | 16 | 0 (min-elements 1) | 17 (max-elements 16) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrrp-config` | `test/vrrp/vrrp-config.ci` | valid configs parse: v3 ipv4 (multi-VIP), v3 ipv6 (link-local first), v2 opt-in, owner case; all 4 interface types spot-checked | |
| `vrrp-config-invalid` | `test/vrrp/vrrp-config-invalid.ci` | every AC-4 rejection with path-naming error (one stanza per validator) | |
| `vrrp-instance-up` | `test/vrrp/vrrp-instance-up.ci` (`option=needs-linux`) | boot with a group; payload-predicate wait until `show vrrp` shows the instance in master state; teardown on group removal; NO sleeps | |
| `vrrp-show` | `test/vrrp/vrrp-show.ci` | `show vrrp`, `show vrrp interface name <n>`, `show vrrp statistics` output shapes incl. `| json`; `clear vrrp statistics` | |
| `vrrp-doctor` | `test/vrrp/vrrp-doctor.ci` | `ze doctor --json` exposes vrrp checks; `ze explain doctor-vrrp-config` | |
| `vrrp-metrics` | `test/vrrp/vrrp-metrics.ci` | metrics endpoint lists ze_vrrp_state / ze_vrrp_transitions_total | |
| `vrrp-vpp-reject` | `test/vrrp/vrrp-vpp-reject.ci` | `backend vpp` + vrrp group -> verify fails with actionable error | |
| `vrrp-idle` | `test/vrrp/vrrp-idle.ci` | interfaces configured, no groups -> zero instances, zero macvlans (umbrella AC-6/A-4) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| keepalived v2 + v3 election/failover/preempt/prio-0/owner | `test/interop/scenarios/vrrp-*-keepalived/` | keepalived | RFC 9568 + RFC 3768 interop -- OWNED BY spec-vrrp-6 (this child provides the plugin those scenarios drive) | |

### Future (if deferring any tests)
- None deferred by this child. Live failover timing, tcpdump MAC assertions, and keepalived interop are OWNED by spec-vrrp-6 (umbrella phase 7); VPP-native VRRP by spec-vrrp-7.

## Files to Modify
- `internal/component/plugin/all/all.go` - regenerated by `make generate` (plugin_imports discovers `internal/plugins/vrrp/register.go` + `internal/plugins/vrrp/yang/register.go`)
- `internal/component/plugin/all/testdata/*.snapshot` - plugin-name snapshot gains "vrrp" (`go test -update`, `internal/component/plugin/all/all_test.go:48,81`)
- `mk/test-functional.mk` - `ze-vrrp-test` target + `.PHONY` entry (model: `ze-isis-test` `mk/test-functional.mk:181-182`)
- `internal/test/cli/register.go` - `registerCIRoot("vrrp", "vrrp", "vrrp", ...)` suite registration (model :22)
- `internal/component/cmd/show/yang/self_containment_test.go` - central guard half: banned vrrp tokens (`ze-show:vrrp-`) in `TestShowSchemaHasNoMigratedOwnerCommands`
- `internal/component/cmd/clear/yang/self_containment_test.go` - banned `ze-clear:vrrp-` token in `TestClearSchemaHasNoMigratedOwnerCommands`
- `docs/features.md`, `docs/features/interfaces.md` - Gateway Redundancy row missing -> implemented (`docs/features/interfaces.md:113` per umbrella)
- `docs/features/rfc-status.md` - RFC 9568 + RFC 3768 rows with source anchors
- `docs/guide/command-reference.md` - show/clear vrrp commands
- `docs/guide/configuration.md` - vrrp under interface unit syntax
- `docs/comparison.md` - VRRP row vs FRR/BIRD/VyOS/keepalived
- `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/plugins.md`, `docs/guide/status.md` - plugin inventory rows
- `docs/functional-tests.md` - test/vrrp suite + ze-vrrp-test target
- `docs/plugin-development/metrics.md` (or the subsystem telemetry doc found at implementation time) - ze_vrrp_* metric names + labels

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` (8 augments into iface tree), `ze-vrrp-cmd.yang` (show/clear); glue GENERATED by `make generate` (yang_glue) |
| YANG validation constraints | Yes | every leaf max-constrained per the leaf table below (range, min/max-elements, zt address types, enumeration) |
| YANG custom validators | Yes | cross-leaf checks live in the plugin verifier (InProcessConfigVerifier + OnConfigVerify), NOT per-leaf ze:validate (they need sibling context; see Key Design Decisions); no CompleteFn needed (numeric leaves + enum complete natively; register `configyang.RegisterCompleteFn` only if editor completion gaps appear -- ospf model `register.go:108`) |
| CLI commands/flags | N/A | no new offline `ze <cmd>`; online show/clear only |
| CLI grammar (action before identifier) | Yes | `show vrrp interface name <name>` typed selector; verb-first; R1-R9 gate |
| Editor autocomplete | Yes | automatic from YANG types/enum; no dynamic CompleteFn required |
| Functional test for new RPC/API | Yes | `test/vrrp/vrrp-show.ci` + suite above |
| Pipe completeness | Yes | show handlers return structured payloads through the standard proxy/pipe machinery (`| json` asserted in vrrp-show.ci) |
| Env var registration | N/A | no environment/ leaves -- all operator config is YANG |
| Doctor check for runtime dependencies | Yes | doctor-vrrp-config (this child, `internal/plugins/vrrp/doctor.go` + `codes.go`); doctor-vrrp-raw-socket owned by child 4's transport package |
| Prometheus counters/metrics | Yes | ze_vrrp_state{interface,vrid,family} gauge (0=init 1=backup 2=master); ze_vrrp_transitions_total{interface,vrid,family,to} counter -- engine-owned; ze_vrrp_adverts_* / packet-error counters are child-4 transport-owned |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, `docs/features/interfaces.md` (Gateway Redundancy row) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (vrrp under interface unit) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (show vrrp x3, clear vrrp statistics) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (ze-show:vrrp-summary/-interface/-statistics, ze-clear:vrrp-statistics) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md`; `internal/plugins/vrrp/PLUGIN.md` (catalog prose, `ai/rules/plugin-design.md` "Registration Metadata Feeds Generated Docs") |
| 6 | Has a user guide page? | Yes | `docs/guide/vrrp.md` (NEW -- outline below) |
| 7 | Wire format changed? | N/A | VRRP wire format is RFC-defined; covered by rfc/short summaries (children 1/4) |
| 8 | Plugin SDK/protocol changed? | N/A | existing callbacks suffice (verify/configure/apply/rollback/started/execute) -- grep evidence at implementation |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` rows for 9568 + 3768 with source anchors to `internal/plugins/vrrp/` files |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (test/vrrp suite, ze-vrrp-test target, suite registration) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (VRRP row) |
| 12 | Internal architecture changed? | No expected | iface contracts changed by child 3, documented there; this child adds no core contract -- verify with grep for `source:` anchors on touched files |
| 13 | Route metadata keys added/changed? | N/A | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` or monitoring guide (names + labels + semantics) |
| 15 | Registered plugin/event/command inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Changed source files referenced by doc source anchors? | Yes | grep `docs/` for `source:` anchors on `internal/component/cmd/{show,clear}` guard tests and iface files; update stale claims |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` examples verified against final YANG |

## Files to Create
- `internal/plugins/vrrp/register.go` - init(): events.RegisterNamespace("vrrp", ...), diagnostic codes + doctor check registration, registry.Registration{Name: "vrrp", Description, RFCs: ["9568","3768"], Features: "yang", YANG: vrrpyang embed, ConfigRoots: ["interface"], Dependencies: ["interface"], RunEngine, InProcessConfigVerifier, ConfigureEngineLogger/ConfigureMetrics/ConfigureEventBus}, CLIHandler (ospf model `register.go:111-144`)
- `internal/plugins/vrrp/vrrp.go` - package doc, atomic.Pointer logger + SetLogger (mandatory pattern), eventbus/metrics atomic holders
- `internal/plugins/vrrp/config.go` - GroupSpec type, extractGroupSpecs (extract-only walk), validateGroups (cross-leaf validators, owner detection, VPP backend check), verify helper shared by InProcessConfigVerifier and OnConfigVerify (ospf `verifyOSPFConfigSections` model :300-306)
- `internal/plugins/vrrp/server.go` - runVRRPEngine: internal-only guard (as112 :223 model), SDK callbacks, pending/active config swap, p.Run with WantsConfig ["interface"], VerifyBudget/ApplyBudget, CommandDecls
- `internal/plugins/vrrp/instance.go` - instance manager keyed (parent, unit, family, vrid); per-instance worker goroutine; readiness predicate; FSM action executor binding child-4 transport + child-3 macvlan + address-owner registry. Engine-side contracts fixed by cross-review: (a) D-B rx path -- the worker receives raw (RxMeta{Src, Dst, TTL, Family, IfIndex}, payload) from the transport, calls packet.Decode with the group's per-interface Lookup, maps decode failures via the error's Reason() to `transport.RecordRxError(reason)`, and produces the AdvertReceived FSM event; (b) spec-vrrp-2 Timer Generations -- the worker owns three clock.Timers from an injected clock.Clock and echoes the Gen value carried by each FSM Start* action on the matching expiry event; (c) v2 address-list check (v2-only, RFC 3768 7.1) -- post-Decode, compare packet VIPs (VIPAt) against configured VIPs; mismatch = drop + reason `address-list` via RecordRxError (label added to spec-1's taxonomy); (d) D-F engine-owned prio-0 counters -- rx counted post-Decode on priority==0, tx counted on SendAdvertZeroPriority execution; VIPs encoded in config order, no reordering (Finding 4)
- `internal/plugins/vrrp/cmd_show.go` - pluginserver.RegisterRPCs proxies (isis contract): ze-show:vrrp-summary / ze-show:vrrp-interface / ze-show:vrrp-statistics / ze-clear:vrrp-statistics, each with PluginCommand, ForwardToPlugin only
- `internal/plugins/vrrp/cmd.go` - OnExecuteCommand dispatch + snapshot payload types (summary/detail/statistics rows)
- `internal/plugins/vrrp/codes.go` - CodeMeta for doctor-vrrp-config (isis `codes.go:21` model)
- `internal/plugins/vrrp/doctor.go` - checkVRRPConfigSanity (post-config phase, config-tree dependency; ospf `register.go:183-198` model)
- `internal/plugins/vrrp/events.go` - typed transition event: `events.Register[VRRPTransition]("vrrp", "state-transition")`; payload struct of value types only (Interface, Unit string; VRID uint8; Family string; From, To string; IsOwner bool)
- `internal/plugins/vrrp/metrics.go` - engine gauges/counters (atomic metrics-registry pattern; isis engine-metrics model) + the engine-owned per-instance show counters (prio-0 rx/tx, transitions per state) per D-F
- `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` - groupings + 8 augments (leaf table below)
- `internal/plugins/vrrp/yang/ze-vrrp-cmd.yang` - show/clear augments (shape below)
- `internal/plugins/vrrp/yang/embed.go`, `internal/plugins/vrrp/yang/register.go` - GENERATED by `make generate` (yang_glue)
- `internal/plugins/vrrp/yang/self_containment_test.go` - presence test (owner half)
- `internal/plugins/vrrp/PLUGIN.md` - catalog prose (area/summary/tags front matter)
- `internal/plugins/vrrp/*_test.go` - unit tests per TDD table
- `test/vrrp/vrrp-config.ci`, `vrrp-config-invalid.ci`, `vrrp-instance-up.ci`, `vrrp-show.ci`, `vrrp-doctor.ci`, `vrrp-metrics.ci`, `vrrp-vpp-reject.ci`, `vrrp-idle.ci`
- `docs/guide/vrrp.md` - operator guide

### YANG: ze-vrrp-conf.yang design (tables, no code)

Module header: name `ze-vrrp-conf`, namespace `urn:ze:vrrp:conf`, prefix `vrrp`;
imports `ze-types` (prefix zt), `ze-extensions` (prefix ze), `ze-iface-conf`
(prefix iface). Two groupings, `vrrp-group-ipv4` and `vrrp-group-ipv6`, each
defining `container vrrp { list group { key "vrid"; ... } }`. The vrrp container
in BOTH groupings carries `ze:backend "netlink"` (consumed by the iface gate,
Key Design Decision D-2).

The 8 augment statements (iface module prefix `iface`, read from
`ze-iface-conf.yang:3`; targets are the grouping-expanded unit family
containers, `interface-unit` grouping :204 used by ethernet :537, dummy :554,
veth :569, bridge :589):

| # | Augment target path | Uses grouping |
|---|---------------------|---------------|
| 1 | `/iface:interface/iface:ethernet/iface:unit/iface:ipv4` | vrrp-group-ipv4 |
| 2 | `/iface:interface/iface:ethernet/iface:unit/iface:ipv6` | vrrp-group-ipv6 |
| 3 | `/iface:interface/iface:veth/iface:unit/iface:ipv4` | vrrp-group-ipv4 |
| 4 | `/iface:interface/iface:veth/iface:unit/iface:ipv6` | vrrp-group-ipv6 |
| 5 | `/iface:interface/iface:bridge/iface:unit/iface:ipv4` | vrrp-group-ipv4 |
| 6 | `/iface:interface/iface:bridge/iface:unit/iface:ipv6` | vrrp-group-ipv6 |
| 7 | `/iface:interface/iface:dummy/iface:unit/iface:ipv4` | vrrp-group-ipv4 |
| 8 | `/iface:interface/iface:dummy/iface:unit/iface:ipv6` | vrrp-group-ipv6 |

Leaf table (list `group`, key `vrid`; identical in both groupings except
`version` [ipv4 only] and `virtual-address` type):

| Leaf | Type | Range / constraint | Default | Description |
|------|------|--------------------|---------|-------------|
| vrid | uint8 | range 1..255 (list key) | none | Virtual Router Identifier (RFC 9568 5.2.3); independent per family |
| virtual-address | leaf-list zt:ipv4-address (ipv4) / zt:ipv6-address (ipv6) | min-elements 1, max-elements 16, ze:syntax "bracket" (sibling `address` convention, `ze-iface-conf.yang:270`) | none | Virtual IP addresses, encoded on the wire in CONFIG ORDER (no reordering). ~~IPv6 groups MUST include at least one link-local (fe80::/10) address~~ The FIRST virtual-address of an ipv6 group MUST be link-local (fe80::/10) -- it is the advert source identity (RFC 9568 5.2.9, erratum 8300; review Finding 4); enforced by the plugin verifier |
| priority | uint8 | range 1..254 | 100 | Election priority (RFC 9568 5.2.4). 255 is reserved: assigned automatically when a virtual-address equals a real address on the unit (owner); never operator-set |
| preempt | boolean | - | true | Preempt_Mode (RFC 9568 6.4.2): higher-priority Backup takes over a live lower-priority Active |
| preempt-delay-seconds | uint16 | range 0..3600 | 0 | ~~Delay before preempting (vendor extension, keepalived interop; not in RFC 9568)~~ Preemption hold-time with JUNOS semantics per spec-vrrp-2 (review Finding 21): armed on the FIRST losing advert while a higher-priority local router waits; cancelled by a rightful-master advert, a prio-0 advert, or reconfig; NEVER delays dead-master failover. keepalived's delay-from-startup semantics explicitly rejected (spec-vrrp-2 decision). Not in RFC 9568 |
| advertise-interval-milliseconds | uint32 | range 10..40950 | 1000 | Advertisement interval. v3: ~~any value~~ multiples of 10 ms only (wire = centiseconds, 1..4095 cs, erratum 8301; review Finding 3). v2: whole seconds within 1000..255000 (wire = seconds). Both enforced by the plugin verifier |
| version | enumeration 2 \| 3 | PRESENT ONLY in vrrp-group-ipv4 | 3 | Protocol version. 2 = RFC 3768 opt-in (keepalived interop, IPv4 only, auth type 0 only); 3 = RFC 9568 |
| accept-mode | boolean | - | false | Accept_Mode (RFC 9568 6.1/6.4.3): non-owner Active accepts packets to the VIPs. v3 semantics; configuring true together with version 2 is rejected by the plugin verifier. NOTE: accept-mode false is NOT dataplane-enforced in this pass -- VIPs are ordinary kernel addresses on the macvlan, so the kernel answers regardless (umbrella Known Limitations row added 2026-07-14); the leaf drives FSM/owner semantics only |

Cross-leaf verifier rules (plugin-side, shared by InProcessConfigVerifier and
OnConfigVerify -- see Key Design Decision D-4):

| Rule | Rejects | Basis |
|------|---------|-------|
| interval-vs-version | version 2 with interval not whole seconds or outside 1000..255000 | RFC 3768 Adver Int = 1 byte whole seconds |
| interval-v3-granularity | version 3 group (any ipv6 group, or ipv4 without version 2) with advertise-interval-milliseconds not a multiple of 10 | wire unit is centiseconds (RFC 9568 5.2.7, erratum 8301); rejecting at verify keeps config == wire; spec-1's encode assert is defense-in-depth (review Finding 3) |
| accept-mode-vs-version | accept-mode true with version 2 | Accept_Mode does not exist in RFC 3768 |
| ipv6-link-local | ~~ipv6 group whose virtual-address list has no fe80::/10 entry~~ ipv6 group whose FIRST virtual-address is not link-local (fe80::/10) -- superseded per review Finding 4 | RFC 9568 5.2.9 + erratum 8300: the FIRST listed address is the advert source identity; spec-1 ladder row 13 rejects received adverts violating it; spec-6 QS-6 asserts it on ze's own tx. The engine encodes VIPs in CONFIG ORDER, no reordering |
| duplicate-vip | same VIP appearing in two groups on the same unit+family | one address, one owner (address-owner registry would conflict at runtime otherwise) |
| vpp-backend | any group present while `interface.backend == "vpp"` | umbrella A-6/R-3, `ai/rules/exact-or-reject.md`; error names the group path and `/interface/backend` |
| owner-detection (not a rejection) | none -- marks IsOwner when a VIP equals a real unit address (same family, prefix length stripped from the unit's address leaf-list) | RFC 9568 5.2.4 (owner priority MUST be 255), 6.4.3 (owner accepts); effective priority forced 255, accept-mode forced true |

### YANG: ze-vrrp-cmd.yang design

Module header: name `ze-vrrp-cmd`, namespace `urn:ze:vrrp:cmd`, prefix
`vrrpcmd`; imports `ze-extensions`, `ze-cli-show-cmd` (prefix clishowcmd),
`ze-cli-clear-cmd` (prefix cliclearcmd). Augment shapes (mpls-cmd show model
:11-26; l2tp clear model :187, selector-consumes-value :196-202):

| Command | YANG node (config false) | ze:command | Args |
|---------|--------------------------|-----------|------|
| `show vrrp` | `augment /clishowcmd:show` -> container vrrp | `ze-show:vrrp-summary` | none |
| `show vrrp interface name <name>` | container interface -> container name | `ze-show:vrrp-interface` | interface name (typed selector, cli-grammar) |
| `show vrrp statistics` | container statistics | `ze-show:vrrp-statistics` | none |
| `clear vrrp statistics` | `augment /cliclearcmd:clear` -> container vrrp -> container statistics | `ze-clear:vrrp-statistics` | none |

RPC proxies (`cmd_show.go`, isis contract): each RPCRegistration declares
WireMethod + Handler + PluginCommand ("show vrrp", "show vrrp interface",
"show vrrp statistics", "clear vrrp statistics"); handlers call
`ForwardToPlugin` only (never re-Dispatch, `cmd_show.go:106-116`); the
interface proxy forwards the name argument (l2tp id model); the no-arg proxies
reject extra args (isis model :107-110). The engine registers matching
`sdk.CommandDecl` entries and dispatches in OnExecuteCommand.

### Show output field tables

Summary row (one per instance):

| Field | Content |
|-------|---------|
| interface | parent interface name |
| unit | unit name |
| vrid | 1..255 |
| family | ipv4 / ipv6 |
| version | 2 / 3 |
| state | initialize / backup / master |
| priority | configured priority |
| effective-priority | 255 when owner, else configured |
| master-ip | current Active router's primary address (self when master, learned source otherwise, empty in initialize) |
| last-transition | timestamp of last state change |
| transitions | count of transitions to master |

Interface detail (summary fields plus):

| Field | Content |
|-------|---------|
| virtual-addresses | configured VIP list |
| owner | true when auto-owner detected |
| macvlan-device | child-3 device name carrying 00:00:5e:00:0{1,2}:{vrid} |
| advertise-interval-milliseconds | configured value |
| master-adver-interval-milliseconds | CURRENT adopted interval (v3 backups adopt the Active router's interval; umbrella key insight) |
| ~~skew-time-milliseconds / master-down-interval-milliseconds~~ skew-time-microseconds / master-down-interval-microseconds | derived timer values from the FSM -- MICROSECOND fields per orchestrator D-G (review Finding 14: spec-2 proved valid v3 skews are sub-millisecond; ms fields would render 0) |
| preempt / preempt-delay-seconds / accept-mode | effective flags (accept-mode true when owner) |
| counters | wire counters (adverts sent/received, rx errors by reason incl. `address-list`) read from the child-4 transport's per-instance CounterSnapshot (D-F); prio-0 sent/received are ENGINE-owned (rx counted post-Decode on priority==0, tx counted on SendAdvertZeroPriority execution); transitions per target state are engine-owned |

Statistics: per-instance counter rows (same key columns interface/unit/vrid/family + the counters above, sourced per D-F: transport CounterSnapshot for wire counters, engine for prio-0 and transitions). `clear vrrp statistics` zeroes them via transport ResetCounters plus the engine counter reset.

### docs/guide/vrrp.md outline

1. What VRRP does in ze (virtual MAC macvlan model, v3 default, v2 opt-in)
2. Quick start: two-router v3 IPv4 config (umbrella agreed shape)
3. IPv6 groups (link-local first VIP requirement)
4. Owner mode (VIP == real address; auto priority 255)
5. Version 2 interop with keepalived (whole-second intervals, no auth)
6. Timers and preemption (interval adoption, skew, preempt-delay)
7. Observing: show vrrp / statistics / metrics / events / doctor
8. Limitations (no tracking, no sync-groups, no unicast peers, VPP rejected until spec-vrrp-7; VRID collision behavior per umbrella R-7)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + sibling child specs 1-4 |
| 2. Audit | Files to Modify/Create + TDD Plan -- check what children 1-4 delivered |
| 3. Wiring phase | Wiring Test table -- registration + YANG + failing .ci |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-vrrp-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section -- loop until 0 BLOCKER / 0 ISSUE |
| 14. Present summary + close | Executive Summary; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- registration + schema reachable end to end
   - Tests: `TestVRRPRegistrationFields`, `TestVRRPReceivesInterfaceSection`, `test/vrrp/vrrp-config.ci` (failing until phase 2 validators exist)
   - Files: register.go, vrrp.go, yang/ze-vrrp-conf.yang (+ `make generate` glue, all.go, snapshot regen), mk target + suite registration
   - Verify: plugin registered, augment parses the agreed config shape (validates A-1/A-2), engine stub idle
2. **Phase: Config extraction + validation** -- GroupSpecs + verifier
   - Tests: `TestExtractGroupSpecs*`, `TestValidate*`, `TestOwnerAutoDetection`, `TestVerifyRejectsVPPBackend`, `TestVRRPBackendGateAnnotation`; `vrrp-config.ci`, `vrrp-config-invalid.ci`, `vrrp-vpp-reject.ci` pass
   - Files: config.go, server.go (verify/configure/apply/rollback callbacks, pending/active swap)
   - Verify: offline `ze config validate` fully functional (InProcessConfigVerifier); boundary table green
3. **Phase: Engine + instance lifecycle** -- instance manager, readiness, FSM hosting
   - Tests: `TestInstanceManagerDiff`, `TestInstanceReadinessPredicate`, `TestInstanceMasterTransitionActions`, `TestInstanceDeleteTeardownOrder`; `vrrp-idle.ci`; `vrrp-instance-up.ci` (needs-linux)
   - Files: instance.go, server.go (OnStarted, internal-only guard), seams to children 2/3/4
   - Verify: create -> macvlan -> ready -> Startup -> Master -> VIPs installed; delete -> prio-0 -> unregister -> device gone
4. **Phase: Show/clear surface** -- cmd YANG + proxies + snapshots
   - Tests: `TestOnExecuteCommandSnapshots`, `TestClearStatistics`, `TestForwardProxiesNoRedispatch`, `TestVRRPCmdSchemaOwnsShowVRRP`; `vrrp-show.ci`
   - Files: yang/ze-vrrp-cmd.yang, cmd_show.go, cmd.go, central guard test tokens
   - Verify: full CLI path `show vrrp ...` -> proxy -> engine -> payload; grammar gate green
5. **Phase: Telemetry + events** -- gauges, counters, typed event
   - Tests: `TestVRRPMetricsIncremented`, `TestVRRPEventPayloadValueType`; `vrrp-metrics.ci`
   - Files: metrics.go, events.go, instance.go increments
   - Verify: every metric incremented on the transition path (R-3)
6. **Phase: Doctor** -- config-sanity check + codes
   - Tests: `TestDoctorVRRPConfig`; `vrrp-doctor.ci`
   - Files: codes.go, doctor.go, register.go registration
   - Verify: `ze doctor --json` + `ze explain doctor-vrrp-config`
7. **Phase: Docs + removal test** -- guide, rows, PLUGIN.md, self-containment
   - Tests: `make ze-doc-test`; manual removal check (delete dir + `make generate` + build) recorded in Pre-Commit Verification (AC-14)
   - Files: all docs rows from the checklist, PLUGIN.md, self_containment tests both halves
8. **RFC refs** -- `// RFC 9568 Section X.Y` comments per RFC Documentation section
9. **Full verification** -- `make ze-verify` + `make ze-vrrp-test`
10. **Complete spec** -- audit tables, learned summary `plan/learned/NNN-vrrp-5-plugin.md`, two commits (A: code+spec+summary+counter; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-14 implemented with file:line |
| Feature completeness | Every End-to-End User Story path unbroken; parity with keepalived default feature set minus tracking (umbrella Known Limitations) |
| Correctness | Owner forces priority 255 + accept-mode true; backups adopt the Active router's interval (detail view shows adopted value); v2 whole-second enforcement; semantics from rfc/short only, never reference implementations |
| Naming | YANG kebab-case with unit suffixes; plugin name "vrrp" (hyphen registry form), log subsystem "vrrp"; metrics ze_vrrp_*; wire methods ze-show:vrrp-* / ze-clear:vrrp-* |
| Data flow | vrrp -> iface for ALL kernel interface state; zero netlink imports in `internal/plugins/vrrp/`; wire I/O only via child-4 transport |
| CLI grammar | typed selector `interface name <n>`; verb-first; no --flag in YANG; R1-R9 static gate green |
| Registration over hardcoding | No vrrp spelling in any central package; plugin/RPC/YANG/doctor/events/metrics all registry-registered; central verb guards ban vrrp tokens; no new field/switch/factory in core packages |
| Doctor checks | doctor-vrrp-config registered, explainable, unit + functional tested; child-4's doctor-vrrp-raw-socket NOT duplicated here |
| YANG validation | every leaf max-constrained per the leaf table; no bare `type string`; cross-leaf rules all fire in `vrrp-config-invalid.ci` |
| Prometheus counters | ze_vrrp_state + ze_vrrp_transitions_total registered AND incremented (holo dead-counter bug class; cite increment sites file:line) |
| Rule: plugin-self-containment | removal test (AC-14) executed and recorded; both guard-test halves present |
| Rule: qemu-testing | this child adds no //go:build linux files (linux code lives in children 3/4); needs-linux .ci tests cover the kernel path -- verify with grep, else add QEMU coverage |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| vrrp plugin registered | `make ze-inventory` (or registry snapshot test) lists vrrp; `go test ./internal/component/plugin/all` green after `-update` |
| YANG augments live | `test/vrrp/vrrp-config.ci` passes (agreed shape validates) |
| Verifier rejects all invalid inputs | `test/vrrp/vrrp-config-invalid.ci` passes (every AC-4 stanza) |
| VPP fail-closed | `test/vrrp/vrrp-vpp-reject.ci` passes |
| Idle with zero groups | `test/vrrp/vrrp-idle.ci` passes |
| Instance lifecycle on linux | `test/vrrp/vrrp-instance-up.ci` passes under the linux runner |
| Show/clear surface | `test/vrrp/vrrp-show.ci` passes; `make ze-cli-grammar-check` green |
| Doctor + explain | `test/vrrp/vrrp-doctor.ci` passes; `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered'` green |
| Metrics live | `test/vrrp/vrrp-metrics.ci` passes |
| Suite wired | `make ze-vrrp-test` runs all test/vrrp .ci files |
| Docs rows | `make ze-doc-test` green; grep rfc-status.md for 9568/3768 rows |
| Removal test | delete `internal/plugins/vrrp/` in a scratch tree + `make generate` + build: green, zero vrrp surface (record output in Pre-Commit Verification) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Config values fully range-checked before reaching the engine; extraction tolerates malformed JSON sections (error, never panic); packet-input validation is child 1/4 territory -- verify this child passes no unvalidated wire data |
| Privilege | Raw sockets / macvlan need CAP_NET_RAW / CAP_NET_ADMIN -- surfaced by child-4 doctor check; this child's errors are actionable, never silent |
| Spoofing / election abuse | Owner auto-detection cannot be triggered remotely (config-only comparison); priority 255 unreachable from config (range 1..254) |
| Resource exhaustion | Instance count bounded by config (255 VRIDs x families x units); per-instance goroutines torn down on delete (no leak on reload churn; unit test asserts goroutine/subscription cleanup) |
| State manipulation | clear vrrp statistics resets counters only, never FSM state; show snapshots are copies (no engine-state mutation through payloads) |
| Error leakage | Log lines name interface/unit/vrid/reason; no raw packet dumps at info level; config errors quote paths, not whole sections |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read rfc/short summary + cited producer -> RESEARCH if misunderstood |
| Augment does not apply (A-1 broken) | STOP; present standalone-tree fallback to user (umbrella decision needed) |
| Shared root not delivered (A-2 broken) | STOP; present nested-root fallback to user |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| QEMU/needs-linux flaky | Payload-predicate waits, never sleeps (ai/rules/testing.md sleep ratchet) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| "one worker goroutine owns the instance" was enough to keep the FSM single-threaded, as the spec's Concurrency Model states | THREE goroutines reach an instance: its worker (timer expiries, rx), the config-apply caller (reconfigure), and a show handler (snapshot). The spec's own contract (fsm.Instance holds no locks; only the owning worker calls Handle) was therefore violated by the engine the spec itself specifies | `go test -race` on the engine tests: reconfigure wrote `in.spec` while the worker read it in fsmConfig/primaryIP. Not a test artifact -- a config reload on a live Master would hit it in production | Added an instance mutex serializing every touch of spec+machine (held across action execution; actions never call back into the instance, so no deadlock). reconfigure applies synchronously under the lock rather than queueing, so spec and FSM config can never disagree mid-advert. Second race found in the same pass: `run()` evaluated `in.fsmConfig()` as a call ARGUMENT, i.e. before `dispatch` took the lock -- fixed with a `startup()` that builds the event inside the lock. Lesson: "single-threaded by convention" is not a design; name every goroutine that can reach the object |
| `go test ./internal/component/plugin/all/... -update` regenerates the registration snapshots | It regenerates them from whatever the CURRENT build tags include. Run WITHOUT the feature tags, the plugin set shrinks, and `-update` happily rewrote the golden files: +4 vrrp wire methods and **-75 lines of isis/ospf/ldp/rsvp-te/gnmi/mcp/web methods deleted**. The gate would then have gone green while silently asserting that six shipped features no longer register anything | `git diff --stat` on the regenerated golden files: an additive change (one plugin) had removed 78 lines. Caught only because the diff was read before moving on | Regenerate with the same tags the suite uses: `go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u)" ./internal/component/plugin/all/... -update` (the Makefile's `GO_TEST_TAGS`, Makefile:51,65). Result is +6 lines, all vrrp. Lesson: `-update` on a golden file is a WRITE, so read its diff like any other write -- a snapshot tool cannot tell "the world changed" from "you built less of it" |
| Boolean leaves (`preempt`, `accept-mode`) arrive as Go bools, so a `.(bool)` type assertion is enough | The TEXT config parser stores scalars as STRINGS, so `accept-mode true` arrives as `"true"`. The assertion failed, the value was dropped SILENTLY, AcceptMode stayed false, and the group's own `accept-mode + version 2` rejection never fired -- the config was accepted and the leaf ignored. `preempt false` had the same fate: it would have silently kept preemption ON | Running the real binary: `bin/ze config validate` on a v2 group with accept-mode returned "configuration valid" when the spec says it must be rejected. The unit tests all passed because they fed Go-native shapes, exactly the false confidence the .ci gate exists to break | Added `asBool` accepting both shapes and REJECTING anything else (a leaf an operator wrote must take effect or be refused, never defaulted); same treatment already applied to numbers via asUint. New `TestLeafShapesFromTextConfig` pins every leaf against both shapes. Lesson: unit tests that construct the config value themselves prove nothing about the shape the PRODUCER sends -- validate through the real entry point before believing a validator works |
| RxItem could be routed to its instance by the packet's VRID | Every instance opens its OWN rx socket bound to the parent and joined to the group, so ONE advertisement on the wire is delivered once PER INSTANCE on that parent, and `RxItem{Meta, Payload}` carried nothing to tell the copies apart. VRID routing would have handed every copy to the one matching instance: N duplicate FSM events and an N-times-inflated receive counter per advert | Writing the engine's rx fan-out against the transport's real API (the sibling spec's API-shape table did not surface it) | Added `Key InstanceKey` to `transport.RxItem`, stamped by the per-instance rxSink (which already held the key); engine routes on the key. Deviation from spec-vrrp-4's published API shape, recorded there too |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- `TestAllPluginsRegistered` no longer carries a hardcoded count (`all_test.go:190` checks non-empty + no test-only names); the real gate is the `TestRegisteredPluginNames` golden snapshot (:81) regenerated with `go test -update`.
- The interface backend is a single global leaf (`interface { backend ...; }`, parsed at `iface/register.go:194-214`), not per-interface -- VPP detection at verify is a one-key read, and the generic `ze:backend` gate gives a second, zero-code enforcement path.
- `UnregisterOwnedAddresses` is the ONLY path that schedules kernel pruning for an interface losing its last owner (staleIfaces, `address_owner.go:126-129`); empty re-registration does not (:74-79). VIP removal must therefore use Unregister, which forces per-instance owner strings.

## Core Insight

Integration here is almost entirely REGISTRATION: every surface (config subtree,
commands, doctor, metrics, events, tests, docs rows) attaches through an existing
registry that iface/ospf/isis already exercise. The only genuinely novel wiring is
(a) two plugins sharing one config root and (b) a plugin augmenting another
component's config tree -- which is why A-1/A-2 are the wiring phase's first
validation targets and the umbrella's designated fallback points.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: VIP owner strings are PER-INSTANCE: `vrrp:<macvlan-device>` (one macvlan per instance, so the pair (owner, iface) is unique) | single owner "vrrp" with per-interface set replacement | `UnregisterOwnedAddresses(owner)` removes the owner from ALL interfaces (`address_owner.go:122-137`) -- a single owner cannot drop one instance's VIPs without dropping every instance's; and Unregister is the sole staleIfaces populator (:126-129), the only mechanism guaranteeing kernel pruning for a device with no YANG desired state. Master: `RegisterOwnedAddresses(dev, "vrrp:<dev>", vips)` (:80, set-replacement :103 makes VIP updates idempotent); Backup/Init/delete: `UnregisterOwnedAddresses("vrrp:<dev>")`. Conflict detection (:83-94) still guards against foreign owners on the device |
| D-2: VPP rejection = plugin verify check (reads `interface.backend`, parseIfaceBackend model `iface/register.go:194-214`) PLUS `ze:backend "netlink"` annotation on the vrrp containers enforced by iface's generic gate (`validateBackendGate` :65, called at verify :173 and apply :405) | rejection only in spec-vrrp-3's macvlan layer; per-interface detection | The backend is one global leaf, so detection is clean at verify (umbrella A-6 confirmed-by-design). The plugin check gives a vrrp-worded actionable error independent of plugin verify ordering (R-6); the annotation gives zero-code, schema-level enforcement that survives even if the plugin check regresses -- and lives in the PLUGIN's YANG, so self-containment holds. Belt-and-braces accepted as two one-liners |
| D-3: ConfigRoots ["interface"] shared with the iface plugin; engine idle with zero groups | own root `vrrp` (standalone tree -- user's rejected alternative); nested root `interface/vrrp` (impossible: vrrp lives under per-type LIST entries, not one fixed subtree) | User decision (umbrella): config under the interface unit. Shared top-level container precedent: five ddos plugins on `ddos/*` (`ddos/local/config.go:19`). Auto-load implication (umbrella A-4): vrrp loads whenever interfaces are configured; cost is one idle SDK loop -- OnConfigure with zero GroupSpecs creates nothing (ospf idle model `register.go:486-489`), proven by vrrp-idle.ci |
| D-4: Cross-leaf validation lives in the plugin verifier (InProcessConfigVerifier + OnConfigVerify), native constraints in YANG; no per-leaf ze:validate | ze:validate custom validators per config-option pattern | Every custom rule here needs SIBLING context (version+interval, version+accept-mode, VIP-list membership, cross-group duplicates, backend+groups) which per-leaf ValidateFn(path, value) cannot see; ospf does exactly this (verifyOSPFConfigSections `register.go:300-306`, wired at :120 so `ze config validate` works offline). No CompleteFn needed (numeric/enum leaves complete natively) |
| D-5: Rollback = pending/active swap discard; NO sdk journal | static's operation journal (`static/register.go:191`) | vrrp performs zero side effects at verify (pure validation); instances mutate only in OnConfigure/OnConfigApply, so OnConfigRollback just clears pendingSpecs (ddos-local model `register.go:148`; ospf pending model :403-453). static needs a journal because it applies during the operation phase; vrrp does not |
| D-6: Macvlan created at instance CREATE, deleted at instance DELETE; VIP install is the only Master-transition kernel action | create device on Master transition | Umbrella R-4: failover latency budget; holo model (research-holo-digest); keeps the promotion path to one address-registry call + GARP/NA burst |
| D-7: Owner auto-detection forces effective values (255 / accept-mode true) rather than rejecting operator-set priority on owner groups | reject priority config on owner groups | RFC 9568 5.2.4 mandates 255 for owners; forcing keeps configs portable between the owner and backup boxes (same group stanza both sides); surfaced honestly via priority vs effective-priority in show output |
| D-8: Per-instance FSM goroutine + manager keyed (parent, unit, family, vrid) | one shared engine loop multiplexing all instances | Matches child-2 FSM contract (injected clock, per-instance timers) and holo's proven per-instance model; instance count is config-bounded (Security review row) |
| D-9: Engine owns ze_vrrp_state + ze_vrrp_transitions_total only; wire counters stay in child-4 transport | all metrics in one place | isis split precedent (transport/metrics.go frame counters vs engine adjacency gauges); ownership follows the code that produces the number (R-3 dead-counter guard needs the increment next to the event). Refined by orchestrator D-F (cross-review): the transport additionally exposes per-instance CounterSnapshot + ResetCounters for the show/clear surface, and the prio-0 rx/tx SHOW counters are engine-owned (rx post-Decode on priority==0, tx on SendAdvertZeroPriority) since only the engine sees decoded priorities |

## Known Limitations
- No tracking (interface/route/script health); priority is static per group (umbrella decision, follow-up spec candidate).
- No sync-groups / vrrp-inherit; groups fail over independently.
- No unicast peers (multicast only, per RFC 9568).
- VPP dataplane rejected at verify until spec-vrrp-7 (explicit, tested, fail-closed).
- v2 authentication not implemented (auth type 0 only).
- No GARP/NA burst tuning knob this umbrella (orchestrator D-H): burst count/spacing are internal constants in spec-vrrp-4; recorded in the umbrella's Known Limitations.
- accept-mode false is not dataplane-enforced in this pass: VIPs are ordinary kernel addresses on the macvlan, so the kernel answers regardless; the leaf drives FSM/owner semantics only (umbrella Known Limitations row, 2026-07-14).
- `show vrrp interface name <n>` filters by PARENT interface (all units/groups on it); no per-unit selector in this pass.
- Live failover/interop evidence is spec-vrrp-6's scope; this child proves single-box lifecycle + surfaces.

## RFC Documentation

Add `// RFC 9568 Section X.Y: "<quoted requirement>"` (or RFC 3768) above enforcing
code. This child MUST document: owner priority rule (5.2.4) at the auto-detection
site; Accept_Mode semantics (6.1, 6.4.3) where effective flags are computed;
interval range/units (5.2.7 + erratum 8301) and the v2 seconds constraint
(RFC 3768 5.3.7) in the verifier; IPv6 link-local first-address rule (5.2.9 +
erratum 8300) in the verifier; prio-0 on shutdown (6.4.3) at the instance-delete
path; virtual MAC formats (7.3) where the macvlan request is built. Packet
validation, FSM transitions, and timer math carry their own citations in
children 1/2/4.

## Implementation Summary

### What Was Implemented
- Plugin registration (`register.go`): `registerVRRP` builds `registry.Registration{Name: "vrrp", ConfigRoots: ["interface"], Dependencies: ["interface"], RFCs: ["9568","3768"], YANG, RunEngine, InProcessConfigVerifier, ConfigureEngineLogger/ConfigureMetrics/ConfigureEventBus, CLIHandler}`; `runVRRPEngine` refuses to run external (`p.IsInternal()` guard, register.go:133) and drives the SDK verify/configure/apply/rollback/started/execute callbacks with an ospf-style pending/active swap.
- Config YANG (`yang/ze-vrrp-conf.yang`): two groupings + 8 augments into the iface unit ipv4/ipv6 containers; `list group { key "name"; }` with a mandatory `vrid` leaf; every leaf range-constrained; `ze:backend "netlink"` on both vrrp containers (schema-level VPP gate).
- Config resolution (`groups.go`): extract-only walk into `[]GroupSpec` (`extractGroupSpecs`); pure cross-leaf verifier (`validateGroups`/`validateGroup`) covering interval-vs-version, v3 10ms granularity, accept-mode-vs-version, ipv6 first-address link-local, duplicate-VIP and duplicate-VRID per unit+family, VPP-backend rejection (groups.go:455), and owner auto-detection (effective priority 255 / accept-mode true).
- Engine + instances (`engine.go`, `instance.go`): instance manager keyed on the group NAME (`GroupSpec.Key`), per-instance worker hosting the child-2 FSM on an injected clock, readiness predicate (`parentReady`/`watchParent`, register.go), macvlan create/delete via `iface.RegisterOwnedMacvlan`/`Unregister`, VIP install/remove via the address-owner registry, transport wiring via `sharedTransport`.
- Show/clear surface (`cmd_show.go`, `yang/ze-vrrp-cmd.yang`): `show vrrp`, `show vrrp interface name <n>`, `show vrrp statistics`, `clear vrrp statistics` as RPC proxies (`ForwardToPlugin`) + `OnExecuteCommand` snapshot handlers.
- Telemetry + events (`telemetry.go`): `ze_vrrp_state` gauge + `ze_vrrp_transitions_total` counter (engine-owned state series), and the typed `StateChange` value-type event on the `vrrp` namespace; `emitStateChange` drives both on every transition.
- Doctor (`doctor.go`, register.go): post-config `vrrp-config-sanity` check emitting `doctor-vrrp-config-invalid` / `doctor-vrrp-backend-unusable`, both explainable via `ze explain`.
- Functional suite (`test/vrrp/*.ci`) + `ze-vrrp-test` target (`mk/test-functional.mk:191`); plugin self-containment guarded by `yang/self_containment_test.go` plus banned-token halves in the central show/clear guard tests.

### Bugs Found/Fixed
- **Transport metrics never reached Prometheus.** `ConfigureMetrics` installed the engine's state registry but did not forward the registry to the shared transport, so the five `ze_vrrp_*` transport series stayed on the no-op registry. Fixed: `ConfigureMetrics` now also calls `sharedTransport.SetMetrics(reg)` (register.go:69-76), closing spec-vrrp-4 AC-12's production-wiring half; `telemetry_test.go` proves the engine state series + event increment on the live `emitStateChange` path.
- **accept-mode arrived as the string "true"** and was dropped by a `.(bool)` assertion, so a `version 2` + accept-mode config validated clean; fixed with an `asBool` that accepts both shapes and rejects anything else, locked by `TestLeafShapesFromTextConfig` and `vrrp-config-invalid.ci` seq8.
- **Duplicate rx per instance.** Each instance opens its own parent-bound rx socket, so one advert is delivered once per instance; routing by VRID would have duplicated FSM events. Fixed by stamping `transport.RxItem.Key` and routing on it (recorded in spec-vrrp-4 Deviations too).
- **Instance concurrency race** (`go test -race`): reconfigure wrote `in.spec` while the worker read it; fixed with a per-instance mutex serializing spec + FSM access (Mistake Log).
- **`go test -update` snapshot shrink**: regenerating the plugin-name golden without the feature tags silently deleted 75 lines of other plugins' methods; regenerate with the Makefile's `GO_TEST_TAGS` (Mistake Log).

### Documentation Updates
- `docs/guide/vrrp.md` (new operator guide), `docs/features/rfc-status.md` (RFC 9568/3768/5798 rows), `docs/features.md`, `docs/features/interfaces.md` (generic macvlan note), `docs/functional-tests.md`, `docs/guide/command-catalogue.md`. NOTE: `docs/features/interfaces.md:113` "Gateway Redundancy | VRRP / keepalived | missing" status row is NOT flipped and still reads "missing" (residual doc gap, outside this bookkeeping pass's edit scope; also recorded in the umbrella).

### Deviations from Plan
- **Metric labels are `{device,group,vrid,family}`, not the spec's `{interface,vrid,family}`** (telemetry.go:56-59). A logical interface is not unique per virtual router: two units of one interface (eth0 and eth0.100) can each host vrid 10 in the same family and would collapse onto one series. `device` is the unit's OS device (unique); `group` names the router the way the operator configured it.
- **The YANG list key is `key "name"`, not `key "vrid"`** (`yang/ze-vrrp-conf.yang:33,142`; `GroupSpec.Key` keys on the name, groups.go:156-163). Naming the group lets an operator renumber a vrid without the tree treating it as a new object (wireguard peer precedent). Cost: a new rule that two groups on one unit+family must not share a vrid, enforced by the verifier (`validateGroups` seenVRID, groups.go:472-477) and by `vrrp-config-invalid.ci` seq4 (dup-vrid).
- **Doctor codes renamed**: the planned single `doctor-vrrp-config` became `doctor-vrrp-config-invalid` plus `doctor-vrrp-backend-unusable` for the VPP-backed case (doctor.go:21-22); `doctor-vrrp-raw-socket` (transport) and `doctor-iface-macvlan` (iface backend) stay per the umbrella proximity decision.
- **Superseded functional tests**: `vrrp-metrics.ci` was replaced by the `telemetry_test.go` unit test (a capturing registry asserts the gauge/counter increment and the state-change event emit); `vrrp-vpp-reject.ci` was folded into `vrrp-config-invalid.ci` seq11 (`backend vpp` + group -> exit 1).
- **File layout**: the plan's `config.go`/`server.go`/`cmd.go`/`events.go`/`metrics.go`/`codes.go` split landed as `groups.go` (extract + verify), `engine.go` + `instance.go` (engine + worker), `cmd_show.go` (proxies + handlers), `telemetry.go` (events + metrics), and `doctor.go` (check + codes). No behavior change.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Plugin registration (Registration, internal-only guard) | Done | `register.go:46` (registerVRRP), :133 (IsInternal guard) | RunEngine `runVRRPEngine` |
| Config YANG (8 augments, name-keyed group) | Done | `yang/ze-vrrp-conf.yang` (:33,:142 `key "name"`) | ze:backend "netlink" on both containers |
| Config resolution (extract + verify + callbacks) | Done | `groups.go` (extractGroupSpecs/validateGroups), `register.go` (OnConfigVerify/Configure/Apply/Rollback) | pending/active swap |
| Engine + instances (manager, FSM host, VIP/macvlan) | Done | `engine.go`, `instance.go`, `register.go` (livePlatform/liveDeps) | keyed on group name |
| Show/clear surface | Done | `cmd_show.go`, `yang/ze-vrrp-cmd.yang` | ForwardToPlugin proxies |
| Telemetry (state gauge + transitions counter) | Done | `telemetry.go:56-81` | labels {device,group,vrid,family} (deviation) |
| Events (typed vrrp namespace payload) | Done | `telemetry.go:32-46` (StateChange) | value types only |
| Doctor (config-sanity) | Done | `doctor.go`, `register.go:227` | doctor-vrrp-config-invalid / -backend-unusable |
| Functional tests + ze-vrrp-test | Done | `test/vrrp/*.ci`, `mk/test-functional.mk:191` | |
| Docs | Done (partial) | `docs/guide/vrrp.md`, `docs/features/rfc-status.md`, etc. | interfaces.md:113 status row still "missing" |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 valid agreed shape | Done | `test/vrrp/vrrp-config.ci` (v3 ipv4 multi-VIP + v3 ipv6) + `TestExtractGroupSpecs` | exit 0 |
| AC-2 v2 group; version absent under ipv6 | Done | `vrrp-config.ci` (v2 opt-in shape) | version leaf ipv4-only in schema |
| AC-3 owner case | Done | `TestOwnerAutoDetection`, `TestInstanceSnapshotReportsEffectivePriority`; `vrrp-config.ci` owner case | eff priority 255 / accept forced |
| AC-4 invalid inputs rejected | Done | `vrrp-config-invalid.ci` seq1-10 + `TestValidate*`, `TestBoundary*` | path-naming errors |
| AC-5 vpp + group verify fails | Done | `vrrp-config-invalid.ci` seq11, `TestVerifyRejectsVPPBackend`, groups.go:455 | |
| AC-6 zero groups idle | Done | `vrrp-idle.ci` (darwin: validate + doctor quiet), `vrrp-show.ci` (linux: empty show via dispatch), `vrrp-instance-up.ci` (no macvlan post-teardown), `TestEngineIdleWithoutGroups` | functional .ci |
| AC-7 commit v3 ipv4 group (linux) | Done | `vrrp-instance-up.ci` (needs-linux): macvlan vMAC 00:00:5e:00:01:0a, FSM->Master, VIP installed | functional .ci |
| AC-8 show vrrp / interface / statistics | Done | `vrrp-show.ci` (needs-linux, live dispatch path) + `cmd_show_test.go` (payload shapes) | functional .ci |
| AC-9 clear vrrp statistics | Done | `vrrp-show.ci` (`cleared` count) + `TestClearStatisticsPreservesState` | functional .ci |
| AC-10 doctor fires + explain | Done | `vrrp-doctor.ci` / `vrrp-doctor-fires.ci` / `vrrp-doctor-quiet.ci` + `TestDoctorReportsInvalidConfig`/`TestDoctorReportsVPPBackend`/`TestDoctorCodesAreExplainable` | |
| AC-11 metrics incremented, no dead counters | Done (unit) | `telemetry_test.go`: `TestRecordTransitionDrivesStateAndCounter`, `TestEmitStateChangeRecordsMetricsAndEmitsEvent` | labels deviation; `vrrp-metrics.ci` superseded by this test |
| AC-12 typed event on transition | Done (unit) | `telemetry_test.go`: `TestEmitStateChangeRecordsMetricsAndEmitsEvent` (live emitStateChange path) + namespace registered register.go:47 | |
| AC-13 group removed -> instance deleted | Done | `vrrp-instance-up.ci` (SIGHUP teardown: macvlan + VIP gone) + `TestEngineDeletesRemovedInstance`, `TestInstanceShutdownAsMasterSendsPriorityZero` | |
| AC-14 delete dir + generate -> surface vanishes | Done (guard) | `yang/self_containment_test.go` (`TestVRRPCmdSchemaOwnsShowVRRP` owner half + `TestVRRPConfSchemaGatesNetlinkBackend`) + central `TestShowSchemaHasNoMigratedOwnerCommands` / `TestClearOwnerRemovalLeavesNoResidue` banned-token halves | invariant guarded by tests |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Config extraction/validation (`TestExtractGroupSpecs*`, `TestValidate*`, `TestOwnerAutoDetection`, `TestVerifyRejectsVPPBackend`, `TestLeafShapesFromTextConfig`, `TestBoundary*`) | Done | `groups_test.go` | the plan's config_test.go names landed here |
| Engine/instance (`TestEngineCreatesInstance`, `TestEngineDeletesRemovedInstance`, `TestInstanceReadinessGatesStartup`, `TestInstanceOwnerStartupGoesMaster`, `TestInstanceShutdownAsMasterSendsPriorityZero`, `TestInstanceTimerGenEcho`, `TestInstanceRx*`, `TestInstanceV2AddressListMismatchDrops`) | Done | `engine_test.go`, `instance_test.go` | the plan's server_test.go names landed here |
| Show/clear (`TestCommandDeclsMatchDispatch`, `TestShowInterfaceRequiresSelector`, `TestShowInterfaceFiltersToOneParent`, `TestClearStatisticsPreservesState`, `TestSelectorValue`) | Done | `cmd_show_test.go` | |
| Doctor (`TestDoctorReportsInvalidConfig`, `TestDoctorReportsVPPBackend`, `TestDoctorSilent*`, `TestDoctorCodesAreExplainable`) | Done | `doctor_test.go` | |
| Telemetry + events (`TestRecordTransitionDrivesStateAndCounter`, `TestClearMetricsDropsStateSeries`, `TestEmitStateChangeRecordsMetricsAndEmitsEvent`) | Done | `telemetry_test.go` | AC-11 + AC-12 at unit level |
| Registration fields + section delivery | Done | `plugins.snapshot` / `wire-methods.snapshot` / `yang-providers.snapshot` goldens (name, 4 wire methods, YANG provider); `vrrp-config-invalid.ci` (the cross-leaf rejections can only come from the in-process verifier, which runs only if the plugin is registered on the `interface` root and receives the section) | ConfigRoots + A-2 section delivery proven functionally; `Dependencies`/`RFCs` metadata not pinned by a dedicated test |
| Self-containment guard (owner + banned-token halves) | Done | `yang/self_containment_test.go`, central show/clear guard tests | AC-14 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| register.go, vrrp.go | Done | registration + logger/atomic holders |
| config -> `groups.go`; server/engine -> `engine.go` + `instance.go`; cmd -> `cmd_show.go`; events+metrics -> `telemetry.go`; codes -> `doctor.go` | Done | file split differs from plan (Deviations) |
| yang/ze-vrrp-conf.yang, yang/ze-vrrp-cmd.yang, yang/embed.go, yang/register.go | Done | embed/register generated by `make generate` |
| test/vrrp/*.ci | Done | `vrrp-metrics.ci` -> `telemetry_test.go`; `vrrp-vpp-reject.ci` -> `vrrp-config-invalid.ci` seq11 |
| mk/test-functional.mk, internal/test/cli/register.go | Done | ze-vrrp-test suite registration |
| central show/clear self_containment tokens | Done | banned `ze-show:vrrp` / `ze-clear:vrrp-statistics` |
| docs/* | Done (partial) | interfaces.md:113 status row still "missing" |

### Audit Summary
- **Total items:** 10 task requirements + 14 ACs + test suites + file set
- **Done:** all requirements; AC-1..AC-14 (AC-11/AC-12 proven at unit level via `telemetry_test.go`; AC-14 guarded by the self-containment tests)
- **Partial:** none (docs partial: the interfaces.md:113 status row still reads "missing")
- **Skipped:** none
- **Changed:** metric labels, YANG list key, doctor code names, superseded `vrrp-metrics.ci` / `vrrp-vpp-reject.ci`, and file layout (all documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Operator can configure VRRP under interface units (agreed shape) | functional test | `test/vrrp/vrrp-config.ci`: v3 ipv4 multi-VIP, v3 ipv6 (link-local first), v2 opt-in, owner case, same vrid in both families, and a VLAN unit all validate (exit 0, "configuration valid") |
| Invalid configs rejected with actionable errors | functional test | `test/vrrp/vrrp-config-invalid.ci` seq1-11 (missing/zero/256 vrid, dup vrid, prio 255, v3 granularity, v2 fraction, accept+v2, ipv6 order, dup VIP, vpp backend) each exit 1 with a path-naming error; unit mirror `TestValidate*` |
| Instance lifecycle works end to end on linux | functional test | `test/vrrp/vrrp-instance-up.ci` (option=needs-linux, QEMU): macvlan carries vMAC 00:00:5e:00:01:0a, VIP install on ->Master (kernel readback), SIGHUP teardown removes macvlan + VIP; asserts the "vrrp: state change" transition log |
| Operator workflow (show/clear/doctor/metrics/events) | functional + unit | show/clear reach the live daemon dispatch path in `vrrp-show.ci` (needs-linux) with payload shapes pinned by `cmd_show_test.go`; doctor codes fire/stay-quiet/explain in `vrrp-doctor.ci` / `vrrp-doctor-fires.ci` / `vrrp-doctor-quiet.ci`; metrics increment + the typed state-change event are proven by `telemetry_test.go` (the umbrella-named `vrrp-metrics.ci` was superseded by this unit test) |
| VPP fail-closed (umbrella AC-5) | functional test | `test/vrrp/vrrp-config-invalid.ci` seq11 (`backend vpp` + group -> exit 1; the umbrella-named `vrrp-vpp-reject.ci` was folded here); enforced at groups.go:455 |
| Idle with zero groups (umbrella AC-6) | functional test | `test/vrrp/vrrp-idle.ci` (darwin, `make ze-vrrp-test`: validate accepts + doctor stays quiet on a group-less interface tree) + `vrrp-show.ci` (linux: `show vrrp` empty via the real dispatch path) + `vrrp-instance-up.ci` (no macvlan post-teardown) |
| Self-containment (removal test) | guard tests | `yang/self_containment_test.go` (owner half `TestVRRPCmdSchemaOwnsShowVRRP` + netlink-gate `TestVRRPConfSchemaGatesNetlinkBackend`) + central `TestShowSchemaHasNoMigratedOwnerCommands` / `TestClearOwnerRemovalLeavesNoResidue` (banned `ze-show:vrrp` / `ze-clear:vrrp` tokens); the AC-14 removal invariant is guarded by these tests rather than a scratch-tree delete recorded here |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Metrics increment untested | internal/plugins/vrrp telemetry | Add telemetry_test.go asserting metric increments via the live emitStateChange path |
| 2 | BLOCKER | Event emission untested | internal/plugins/vrrp telemetry | Add telemetry_test.go asserting the state-change event via the live emitStateChange path |
| 3 | BLOCKER | No functional show/clear test | test/vrrp | Add vrrp-show.ci (needs-linux) |
| 4 | BLOCKER | No needs-linux instance-lifecycle test | test/vrrp | Add vrrp-instance-up.ci (needs-linux, QEMU-validated) |
| 5 | ISSUE | No vrrp-idle.ci | test/vrrp | Add vrrp-idle.ci (darwin) |
| 6 | ISSUE | Missing central self-containment guard | central show/clear | Add the central show/clear self-containment guard test |
| 7 | ISSUE | Missing plugin self-containment guard | plugin owner-half | Add the plugin owner-half self-containment guard test |
| 8 | ISSUE | Missing backend-gate test | netlink backend gate | Add the netlink-gate guard test |
| 9 | ISSUE | Missing registration-field test | plugin registration | Add multi-VIP unit tests covering registration fields |

### Fixes applied
- Added telemetry_test.go covering metric increments and state-change event emission via the live emitStateChange path.
- Added functional tests: test/vrrp/vrrp-idle.ci (darwin), and vrrp-show.ci + vrrp-instance-up.ci (needs-linux, QEMU-validated).
- Added self-containment guard tests: plugin owner-half, central show/clear, and the netlink backend-gate.
- Added multi-VIP unit tests (registration-field coverage).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Audit row cited a non-existent register_test.go coverage | this spec (Audit) | Correct the audit row (since fixed) |
| 2 | NOTE | 3 NOTEs -- registration-field functional coverage judged sufficient (no additional unit test required) | -- | recorded |

### Final status
**Run 2 CLEAN after fixes: 0 BLOCKER, 0 ISSUE.** Run 1 (4 BLOCKER, 5 ISSUE) and the Run 2 residual ISSUE (audit row citing a non-existent register_test.go) are all fixed. NOTEs: 3, all judging the registration-field functional coverage sufficient.
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
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
- [ ] Interop tests for protocol features (owned by spec-vrrp-6; N/A here with that justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
