# Spec: vrrp-0 -- VRRP on Interfaces (Umbrella)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. Child specs: `spec-vrrp-1-packet.md` through `spec-vrrp-7-vpp.md`
4. `rfc/short/rfc9568.md` (VRRPv3), `rfc/short/rfc3768.md` (VRRPv2)
5. Session state: `tmp/session/session-state-98112.md` (research digests)

## Task

Add VRRP (Virtual Router Redundancy Protocol) support to ze interfaces: RFC 9568
(VRRPv3, default) and RFC 3768 (VRRPv2, opt-in for keepalived interop), IPv4 and
IPv6, with RFC-strict virtual-MAC failover from day one (macvlan device carrying
00:00:5e:00:01:{vrid} / 00:00:5e:00:02:{vrid}).

User decisions taken 2026-07-14 (four explicit answers):

| Decision | Choice | Rejected alternatives |
|----------|--------|----------------------|
| Config placement | Under the interface unit: `interface <type> <name> unit <u> ipv4/ipv6 vrrp group <vrid>` (Junos/Nokia style) | Standalone `vrrp` tree (ze OSPF style); VyOS named groups |
| Address family model | Explicit `ipv4` / `ipv6` containers, independent VRID namespaces per family (RFC 9568 separate virtual routers) | Single group list with family inferred from addresses |
| Virtual MAC | macvlan per instance from day one (holo-routing model, RFC-strict) | keepalived default mode (VIPs on parent, real MAC + GARP only) |
| Versions | v3 default for both families; per-group `version 2` opt-in (IPv4 only); v2 auth NOT implemented (auth type 0 only) | v3-only; v2 default |

### Config surface (agreed shape)

CORRECTED 2026-07-15 (user): the group is keyed by an operator-assigned NAME,
not by the VRID; `vrid` is a mandatory leaf inside. Leaf-lists use bracket
syntax (the parser rejects repeated statements). This exact text is accepted by
`bin/ze config validate`:

```
interface {
    backend netlink;
    ethernet eth0 {
        unit 0 {
            ipv4 {
                address [ 192.0.2.251/24 ];
                vrrp {
                    group uplink {
                        vrid 10;
                        virtual-address [ 192.0.2.1 192.0.2.2 ];
                        priority 200;
                        preempt true;
                        advertise-interval-milliseconds 1000;
                    }
                }
            }
            ipv6 {
                vrrp {
                    group uplink-v6 {
                        vrid 10;
                        virtual-address [ fe80::1 2001:db8::1 ];
                    }
                }
            }
        }
    }
}
```

Why a name: it matches ze's own list convention (`list peer { key "name" }`,
`internal/component/iface/yang/ze-iface-conf.yang:960-967`, whose name leaf is
"a free-form label, used as list key. Not sent on the wire") and
`ai/rules/cli-grammar.md` "Identifiers Are Strings". It also lets an operator
renumber a group's vrid without the config tree treating it as a different
object, and gives logs, `show`, metrics, and events a stable human label.

Two consequences the VRID-keyed shape did not have:
- **New rule:** two groups on one unit+family MUST NOT share a vrid (it was
  unrepresentable when the vrid WAS the key). Enforced by the plugin verifier
  (`validateGroups`, `internal/plugins/vrrp/groups.go:439`); the same vrid in
  the two families stays legal (RFC 9568 Section 1.2).
- **`vrid` is mandatory** (`yang/ze-vrrp-conf.yang:44-52`, enforced again in
  `applyGroupLeaves` `internal/plugins/vrrp/groups.go:345`): the name carries no
  protocol meaning, so nothing else can supply the virtual router's identity.

Leaf naming per `ai/rules/config-naming.md` (unit suffix when ambiguous):
`advertise-interval-milliseconds`, `preempt-delay-seconds`. Grammar examples:
`set interface ethernet eth0 unit 0 ipv4 vrrp group uplink priority 200`,
`show vrrp`, `show vrrp interface name eth0`.

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-vrrp-1-packet.md` | VRRPv2/v3 packet codec + receive validation (pure Go, no sockets) | - |
| 2 | `spec-vrrp-2-fsm.md` | Per-group state machine + timers (Initialize/Backup/Master, skew, master-down, preempt, accept-mode, owner, prio-0), injected clock | spec-vrrp-1 |
| 3 | `spec-vrrp-3-macvlan.md` | iface component macvlan device support (netlink create/delete/MAC, owned devices, address-owner integration; VPP = verify-reject) | - |
| 4 | `spec-vrrp-4-transport.md` | Raw proto-112 sockets (rx parent / tx macvlan), multicast joins, TTL 255, gratuitous ARP + unsolicited NA senders, doctor raw-socket check, QEMU integration tests | spec-vrrp-1 |
| 5 | `spec-vrrp-5-plugin.md` | Plugin registration, YANG augments into iface tree, config resolution, engine wiring, VIP install on macvlan, show/clear commands, telemetry, doctor config checks, .ci tests, docs | spec-vrrp-1..4 |
| 6 | `spec-vrrp-6-interop.md` | keepalived interop (container scenarios + QEMU netns evidence script), v2 + v3, failover/preempt/prio-0/owner cases | spec-vrrp-5 |
| 7 | `spec-vrrp-7-vpp.md` | VPP dataplane support via VPP's native VRRP plugin (skeleton -- until it lands, VPP-backed interfaces REJECT vrrp config at verify per `ai/rules/exact-or-reject.md`) | spec-vrrp-5 |

spec-vrrp-7 stays `skeleton` in this pass (needs VPP binapi research); children 1-6
are taken to `ready`. The interim VPP behavior (explicit verify-time rejection,
fail closed) is implemented in spec-vrrp-5 so the dual-dataplane rule
(`ai/rules/design-context.md` "VPP support can be added later" anti-pattern) is
honored by an explicit, tested rejection rather than silent drift.

### Prior decision superseded

`plan/spec-improve-0-umbrella.md` (Declined Findings, row "4: protocol breadth")
declined VRRP in 2026-07-08 as not needed then. The user explicitly requested
VRRP on 2026-07-14; this umbrella supersedes that decline for VRRP only.
`docs/features/interfaces.md:113` lists "Gateway Redundancy | VRRP / keepalived |
missing | medium" -- this umbrella closes that row.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/module-tiers.md` - tier placement for the new packages
  → Decision: VRRP engine is an edge plugin (`internal/plugins/vrrp/`) -- config-driven engine nothing depends on, same tier as isis/ospf/ldp; macvlan support extends the existing iface component/backends
- [ ] `ai/patterns/plugin.md` + `ai/rules/plugin-design.md` - plugin anatomy
  → Constraint: register via init() in register.go; RunEngine `func(net.Conn) int` (`internal/component/plugin/registry/registry.go:43`); atomic.Pointer logger; no sibling plugin imports; make generate + TestAllPluginsRegistered count
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Constraint: ALL vrrp surface (YANG augments, show commands, doctor checks, metrics) lives under the vrrp plugin or vrrp-owned packages; deleting `internal/plugins/vrrp/` + regenerating must remove every vrrp feature and keep the build green
- [ ] `ai/rules/config-naming.md` + `ai/rules/config-surface.md` - leaf naming, YANG vs env var
  → Constraint: all tunables are YANG config (no env vars); unit suffixes on ambiguous leaves (`advertise-interval-milliseconds`)
- [ ] `ai/rules/cli-grammar.md` - command grammar
  → Constraint: `show vrrp`, `show vrrp interface name <name>` (typed selector); config mutation stays in engine set/delete; no new operational add/del verbs
- [ ] `ai/rules/qemu-testing.md` - linux-only testing
  → Constraint: every //go:build linux file ships with QEMU-runnable integration tests; .ci tests that apply kernel config are `option=needs-linux`; keepalived installs from Alpine packages for the netns lab
- [ ] `ai/rules/interop-and-goal-validation.md` - interop requirement
  → Constraint: VRRP is a wire protocol; interop against keepalived is MANDATORY before the umbrella closes
- [ ] `ai/rules/doctor-checks.md` - runtime dependency checks
  → Constraint: raw-socket proto-112 probe + macvlan-capability probe registered by their owning packages. UPDATED 2026-07-14: codes are `doctor-vrrp-raw-socket` (transport, spec-vrrp-4), `doctor-vrrp-config` (plugin, spec-vrrp-5), and `doctor-iface-macvlan` (iface netlink backend, spec-vrrp-3 deliberate deviation from the provisional `doctor-vrrp-*` naming -- the macvlan capability is iface-owned, proximity rule)
- [ ] `ai/rules/exact-or-reject.md` - VPP handling
  → Decision: until spec-vrrp-7, VPP-backed interfaces reject vrrp config at verify with a clear error; no silent netlink-only approximation

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - VRRPv3 (85 compliance items, 6 errata integrated)
  → Constraint: v3 timers in centiseconds (Advertisement_Interval 1..4095 cs, errata 8301); Skew = ((256-prio)*Active_Adver_Interval)/256 cs; backups ADOPT the active router's interval; ~~checksum uses pseudo-header (v4 AND v6)~~ CORRECTED 2026-07-14: v3/IPv4 checksum is message-only, NO pseudo-header (`rfc/full/rfc9568.txt:880-885`, change note :194-196); only v3/IPv6 includes the RFC 8200 pseudo-header. TX strict RFC 9568; RX dual-accepts legacy RFC 5798 pseudo-header sums with a distinct compat counter (spec-vrrp-1 Key Design Decision; older keepalived used the pseudo-header, recent keepalived/FRR/Arista follow 9568); TTL/hop-limit MUST be 255 on rx; accept-mode v3-only
- [ ] `rfc/short/rfc3768.md` - VRRPv2 (50 compliance items)
  → Constraint: v2 timers whole seconds; Skew = (256-prio)/256 s (float, must not truncate to 0); checksum NO pseudo-header; interval mismatch = discard (unlike v3); auth fields present but only type 0 supported

**Key insights:**
- Both reference implementations get core v3 behavior wrong; ze must implement from the RFC summaries, using holo/uvrrpd only for architecture shape (macvlan, socket layout) -- the child specs carry the verified bug lists.
- ze already has every integration seam needed except macvlan and GARP/NA senders: raw-socket transports (ospf/isis), address ownership (as112), deep YANG augments (bgp plugins), doctor/telemetry/event registries.

## Current Behavior (MANDATORY)

**Source files read:** (producers read directly this session or agent-verified and spot-checked)
- [ ] `internal/component/iface/address_owner.go` - RegisterOwnedAddresses :80 (conflict check :83-94, reconcile trigger :111), UnregisterOwnedAddresses :122 -- read directly
- [ ] `internal/component/plugin/registry/registry.go` - Registration struct: RunEngine :43, ConfigRoots :50, ConfigureMetrics :104, ConfigureEventBus :111 -- grep-verified directly
- [ ] `internal/plugins/ospf/transport/backend_linux.go` - openInterfaceSocket :235 (AF_INET/SOCK_RAW + SO_BINDTODEVICE), setMulticastOptions :263 (TTL 1 -- vrrp needs 255), joinGroup :287 -- read directly
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - grouping interface-unit :204 (list unit :207); used by ethernet :551, dummy :566, veth :586, bridge :616 -- read directly
- [ ] `internal/plugins/isis/transport/backend_linux.go` - AF_PACKET L2 pattern for GARP/NA: OpenCircuit :53, Send :135, joinMulticast :238
- [ ] `internal/plugins/as112/register.go` - address-owner consumer + internal-only guard :223, applyAddressRegistration :255
- [ ] `internal/component/iface/resolve.go` - Subscribe :80 (LinkEvent), Resolve :65, Addresses :71
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - AddAddress :204, RemoveAddress :222 (netlink apply path)
- [ ] `internal/plugins/isis/cmd_show.go` - show-command proxy contract :47 (RegisterRPCs + PluginCommand + ForwardToPlugin :106)
- [ ] `internal/plugins/ospf/register.go` - closest plugin model: registerOSPF :89, ConfigRoots :116, doctor checks :183-263
- [ ] `internal/component/firewall/protocol.go` - `"vrrp": 112` :14 (only existing VRRP trace besides the VPP translate map)

**Behavior to preserve:** (unless user explicitly said to change)
- iface component remains the only writer of kernel interface state; vrrp requests devices/addresses through iface APIs (address-owner registry, new macvlan API), never direct netlink
- Existing address-owner semantics for as112 unchanged (conflict detection per owner)
- Existing interface YANG tree shape unchanged; vrrp attaches only via augment from the vrrp plugin's own module
- ospf/isis transports untouched (vrrp gets its own transport package)
- Firewall `match protocol vrrp` (proto 112) keeps working

**Behavior to change:**
- None removed. New: vrrp config subtree under interface units; macvlan devices appear when vrrp groups are configured; proto-112 traffic emitted/consumed; new show/clear commands, metrics, doctor checks.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: `interface <type> <name> unit <u> ipv4|ipv6 vrrp group <vrid> {...}` -- YANG-validated tree, delivered to the vrrp plugin as the `interface` config root section (SDK ConfigSection JSON)
- Wire rx: proto-112 datagrams on the parent interface raw socket (multicast 224.0.0.18 / ff02::12)
- Operator: `show vrrp ...`, `clear vrrp statistics` via RPC dispatch

### Transformation Path
1. Config root `interface` -> vrrp plugin extracts per-unit vrrp containers -> group specs (vrid, family, priority, VIPs, timers)
2. Group spec -> engine creates instance: macvlan request to iface (device + virtual MAC), raw sockets on parent (rx) and macvlan (tx), FSM start
3. Rx datagram -> packet.Decode + validation (child 1) -> FSM event (child 2) -> state transition
4. FSM Master transition -> VIP install on macvlan (address-owner registry) + GARP/unsolicited NA burst (child 4) + advert timer
5. FSM Backup transition -> VIP removal, master-down timer with skew
6. State/counters -> metrics registry (ze_vrrp_*), show command payloads, eventbus notifications

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> vrrp plugin | SDK 5-stage protocol; ConfigSection JSON down, (status, payload) up | [ ] |
| vrrp plugin <-> iface component | Same-process Go calls (RegisterOwnedAddresses, new macvlan API) -- plugin MUST run internal (as112 guard model) | [ ] |
| vrrp plugin <-> kernel | Raw sockets (AF_INET/AF_INET6 proto 112, AF_PACKET for GARP/NA) in transport backend, //go:build linux + stub | [ ] |
| iface <-> kernel | netlink macvlan/address ops in `internal/plugins/iface/netlink/` | [ ] |
| vrrp <-> CLI | RPCRegistration proxies + ze-vrrp-cmd.yang augment of /clishowcmd:show | [ ] |

### Integration Points
- `iface.RegisterOwnedAddresses` / `UnregisterOwnedAddresses` (`internal/component/iface/address_owner.go:80,122`) - VIP lifecycle
- New iface macvlan API (child 3) - device lifecycle, modeled on the wireguard spec-struct pattern (`internal/component/iface/wireguard.go:21`)
- `iface.Subscribe` (`internal/component/iface/resolve.go:80`) - parent/macvlan link tracking
- `pluginserver.RegisterRPCs` + `sdk.CommandDecl` - show/clear commands
- `diagnostic.RegisterDoctorCheck` + plugin-owned CodeMeta - readiness
- `events.RegisterNamespace("vrrp", ...)` - state-change events on the bus

### Architectural Verification
- [ ] No bypassed layers (vrrp never calls netlink directly; all kernel interface state via iface)
- [ ] No unintended coupling (iface gains a generic macvlan API, no vrrp knowledge)
- [ ] No duplicated functionality (reuses address-owner, transport patterns, registries)
- [ ] Zero-copy preserved where applicable (rx buffers reused per socket; encode into caller buffer)
- [ ] Registration over hardcoding -- plugin registry, RPC registry, YANG augments, doctor registry, event namespace; no vrrp spelling in any central package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ze's YANG loader applies augments into grouping-expanded paths (`/interface/ethernet/unit/ipv4`) | bgp plugins augment grouping-expanded `/bgp:bgp/bgp:group/bgp:peer/bgp:session` (`internal/component/bgp/plugins/hostname/yang/ze-hostname.yang:96`); firewall-irr augments 4-deep list path (`internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang:81`) | Placement decision falls back to standalone `vrrp` tree (user's second choice) | spec-vrrp-5 wiring phase: augment module + `ze config validate` .ci test parses a vrrp block | unvalidated |
| A-2 | Multiple plugins may share one config root (vrrp consumes `interface` alongside the iface component) | five ddos plugins declare the same root (`internal/plugins/ddos/*/register.go` ConfigRoots); iface ConfigRoots ["interface"] `internal/component/iface/register.go:139` | vrrp needs its own root -> placement falls back to standalone tree | spec-vrrp-5 wiring test: both plugins receive the section on a config with vrrp groups | unvalidated |
| A-3 | macvlan in bridge mode on the parent delivers proto-112 multicast to the parent's rx socket while tx from the macvlan egresses with the virtual MAC | holo-vrrp uses exactly this split (rx bound parent, tx bound macvlan) in production | rx moves to the macvlan socket, or tx needs AF_PACKET frames like uvrrpd | spec-vrrp-4 QEMU integration test with veth pair + tcpdump assertions | unvalidated |
| A-4 | ConfigRoots-driven auto-load on `interface` (loads vrrp whenever interfaces are configured) is acceptable; engine stays idle (no sockets/devices) when no vrrp groups exist | plugin startup is config-path gated; sockets open per configured group in OnStarted (ospf model `internal/plugins/ospf/register.go:455`) | add explicit enable knob or nested-root support | spec-vrrp-5 .ci: config without vrrp groups boots with zero vrrp sockets/devices (assert via show vrrp + doctor) | unvalidated |
| A-5 | keepalived from Alpine packages can run in a netns lab for both v2 and v3 interop | qemu-testing.md interop-lab pattern (xl2tpd/accel-ppp precedent); keepalived is in Alpine community | fall back to FRR vrrpd container for interop | spec-vrrp-6 evidence script boots keepalived in netns | unvalidated |
| A-6 | VPP-backed interfaces can be detected at verify time so vrrp config on them fails closed | exact-or-reject rule; iface backend selection exists (netlink vs vpp backends under `internal/plugins/iface/`) | rejection moves to commit/apply time with clear error | spec-vrrp-5 verify test with vpp-backed config | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reference implementations normalized wrong v3 behavior (both holo and uvrrpd fail interval-adoption, skew, checksum details) -- copying architecture may smuggle in semantics | Unit tests written from RFC summaries disagree with reference-derived code | Child specs list every known reference bug as an explicit negative test; codec/FSM implemented from `rfc/short/` only |
| R-2 | Timer-unit confusion (s vs cs vs ms) -- the single most common VRRP defect (holo 100x bug, Nokia ds/cs split) | FSM unit tests with injected clock assert exact durations | One internal unit (milliseconds) everywhere; conversion ONLY at wire encode/decode; boundary tests in both child 1 and child 2 |
| R-3 | macvlan + VPP dataplane are disjoint worlds; silent netlink-only behavior would violate dual-dataplane policy | vrrp config accepted on a VPP interface without VPP support | Verify-time rejection implemented and tested in child 5; child 7 tracks native VPP VRRP |
| R-4 | Failover timing depends on netlink address-install latency (VIPs installed on Master transition) | QEMU failover test measures gap between prio-0 advert and new master's GARP | Pre-create macvlan at group create (not at Master transition); only address install remains on the transition path |
| R-5 | GARP/NA senders are net-new; wrong frames (uvrrpd's 0x52 typo, holo's wrong NA dst) are silently ineffective | Interop test: peer's ARP/ND cache updates after failover | Frame golden-byte unit tests + tcpdump-based assertions in the QEMU lab |
| R-6 | Sharing the `interface` config root delivers large config sections to vrrp on every reload; slow parse could delay reload | Reload .ci test duration | Extract-only parsing (walk to vrrp containers, ignore the rest) |
| R-7 | VRID collision between two ze boxes or ze+keepalived on one LAN disrupts both | Interop scenario with duplicate VRID asserts master election converges | Election is the RFC-mandated resolution; doctor check cannot see the LAN -- document in guide |

## Wiring Test (MANDATORY -- NOT deferrable)

Umbrella-level wiring rows; each is owned and detailed by the named child spec.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config commit with vrrp group | → | vrrp plugin engine creates instance (macvlan + sockets + FSM) | `test/vrrp/vrrp-instance-up.ci` (child 5) |
| Proto-112 advert received | → | packet.Decode -> FSM Backup hold | `TestFSMBackupReceivesAdvert` (child 2) + `test/vrrp/vrrp-backup-hold.ci` (child 6 QEMU) |
| Master-down expiry | → | FSM -> Master: VIP install + GARP | `TestFSMMasterDownPromotion` (child 2) + `effective-vrrp-keepalived.py` failover step (child 6) |
| `show vrrp` | → | RPC proxy -> engine OnExecuteCommand | `test/vrrp/vrrp-show.ci` (child 5) |
| `ze doctor` on vrrp config | → | doctor-vrrp-raw-socket + config sanity checks | `test/vrrp/vrrp-doctor.ci` (child 5) |

## Acceptance Criteria

Umbrella-level (children carry the detailed AC tables):

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two ze boxes, same VRID, priorities 200/100, v3 IPv4 | Higher priority becomes Master within 3x advert + skew; VIP pingable; macvlan carries 00:00:5e:00:01:{vrid} |
| AC-2 | Master stopped (prio-0 advert sent) | Backup promotes within skew time, sends GARP, VIP reachable |
| AC-3 | ze vs keepalived, v2 and v3 | Interop scenarios pass both directions (ze master / keepalived master, preempt on/off) |
| AC-4 | IPv6 group with link-local VIP | v3 advert from macvlan link-local; unsolicited NA on promotion; VIP reachable |
| AC-5 | vrrp group on VPP-backed interface | Verify fails with actionable error naming the interface and the missing VPP support |
| AC-6 | No vrrp groups configured | Zero vrrp sockets, zero macvlan devices, `show vrrp` reports no instances |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a v3 IPv4 group on eth0, commits | config -> vrrp plugin -> macvlan + sockets + FSM -> Master election -> VIP live | `effective-vrrp-keepalived.py` scenario 1 (child 6) |
| 2 | Watches failover after master loss | prio-0/timeout -> Backup FSM -> VIP install + GARP -> traffic flows | `effective-vrrp-keepalived.py` failover scenario (child 6) |
| 3 | Runs `show vrrp` / `show vrrp interface name eth0` | CLI -> RPC proxy -> engine state dump | `test/vrrp/vrrp-show.ci` (child 5) |
| 4 | Runs `ze doctor` before deployment | doctor registry -> raw-socket + config checks | `test/vrrp/vrrp-doctor.ci` (child 5) |
| 5 | Scrapes Prometheus | metrics registry -> ze_vrrp_state / transitions / adverts | `test/vrrp/vrrp-metrics.ci` (child 5) |

## 🧪 TDD Test Plan

Test detail lives in the child specs; this table is the umbrella index.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Codec suite (child 1) | `internal/plugins/vrrp/packet/*_test.go` | Encode/decode/validate v2+v3, golden bytes, negative cases from reference-impl bugs | |
| FSM suite (child 2) | `internal/plugins/vrrp/fsm/*_test.go` | All transitions, timer math (skew/master-down), injected clock | |
| macvlan suite (child 3) | `internal/plugins/iface/netlink/macvlan_linux_test.go` + iface unit tests | Create/delete/MAC, reconcile, conflict | |
| Transport suite (child 4) | `internal/plugins/vrrp/transport/*_test.go` | Frame construction (GARP/NA golden bytes), socket option matrix | |
| Plugin config suite (child 5) | `internal/plugins/vrrp/*_test.go` | Config extraction, validation, owner detection | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| vrid | 1-255 | 255 | 0 | 256 |
| priority | 1-254 (255 auto for owner) | 254 | 0 | 255 (operator-set) |
| advertise-interval-milliseconds (v3) | 10-40950 | 40950 | 9 | 40960 |
| advertise-interval-milliseconds (v2) | 1000-255000 (whole s) | 255000 | 999 | 256000 |
| virtual-address count | 1-16 | 16 | 0 | 17 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Config/validate/show/doctor/metrics | `test/vrrp/*.ci` (child 5) | Operator configures and inspects VRRP | |
| Live failover (needs-linux) | `test/vrrp/*.ci` + QEMU (child 6) | Failover behavior on real kernel | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| v3 election + failover + preempt | `test/interop/scenarios/vrrp-*-keepalived/` (child 6) | keepalived | RFC 9568 interop | |
| v2 election (opt-in version 2) | `test/interop/scenarios/vrrp-v2-keepalived/` (child 6) | keepalived (default v2) | RFC 3768 interop | |

### Future (if deferring any tests)
- None deferred at umbrella level. spec-vrrp-7 (VPP) is a skeleton child, explicitly reported to the user as out of this pass.

## Files to Modify
- `internal/component/iface/` - macvlan API (child 3)
- `internal/plugins/iface/netlink/` - macvlan backend (child 3)
- `internal/plugins/iface/vpp/` - macvlan/vrrp verify-reject (child 3/5)
- `docs/features/interfaces.md` - Gateway Redundancy row missing -> implemented
- `docs/features/rfc-status.md` - RFC 9568 + RFC 3768 rows
- `mk/test-functional.mk`, `mk/test-integration.mk` - ze-vrrp-test, ze-qemu-vrrp-keepalived-test targets

## Files to Create
- `internal/plugins/vrrp/` - plugin root: register.go, config.go, server.go, cmd_show.go, codes.go (children 2/5)
- `internal/plugins/vrrp/packet/` - codec (child 1)
- `internal/plugins/vrrp/fsm/` - state machine (child 2)
- `internal/plugins/vrrp/transport/` - sockets + GARP/NA, backend_linux.go/backend_other.go/register.go/metrics.go (child 4)
- `internal/plugins/vrrp/yang/` - ze-vrrp-conf.yang (augments) + ze-vrrp-cmd.yang + generated glue (child 5)
- `test/vrrp/*.ci` - functional suite (child 5/6)
- `test/interop/scenarios/vrrp-*-keepalived/` - interop scenarios (child 6)
- `scripts/evidence/effective-vrrp-keepalived.py` - QEMU netns lab (child 6)
- `docs/guide/vrrp.md` - operator guide (child 5)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` (augments into iface tree), `ze-vrrp-cmd.yang` (show/clear) |
| YANG validation constraints | Yes | range/type on every leaf per child 5 (vrid 1..255, priority 1..254, interval ranges, max-elements 16) |
| YANG custom validators | Yes | version-dependent interval validator; ipv6 FIRST-address link-local validator (ordering, not mere presence); v3 interval multiple-of-10ms rule. UPDATED 2026-07-14: these are cross-leaf checks needing sibling context, so they live in the plugin's InProcessConfigVerifier (spec-vrrp-5 Key Design Decision D-4), not per-leaf `ze:validate` registrations |
| CLI commands/flags | N/A | no new offline `ze <cmd>`; online show/clear only |
| CLI grammar (action before identifier) | Yes | `show vrrp interface name <name>` typed selector (child 5) |
| Editor autocomplete | Yes | automatic from YANG enums/types; interface-name CompleteFn if needed (child 5) |
| Functional test for new RPC/API | Yes | `test/vrrp/vrrp-show.ci` etc. (child 5) |
| Pipe completeness | Yes | show handlers return structured payloads through standard pipe machinery (child 5) |
| Env var registration | N/A | no environment/ leaves -- all operator config is YANG |
| Doctor check for runtime dependencies | Yes | doctor-vrrp-raw-socket (transport, child 4), doctor-vrrp config sanity + macvlan capability (children 3/5) |
| Prometheus counters/metrics | Yes | ze_vrrp_state{interface,vrid,family}, ze_vrrp_transitions_total, ze_vrrp_adverts_sent/received_total, ze_vrrp_packet_errors_total{reason} (children 4/5) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, `docs/features/interfaces.md` (Gateway Redundancy row) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (vrrp under interface unit) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (show vrrp, clear vrrp statistics) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (ze-show:vrrp-* wire methods) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/vrrp.md` (new) |
| 7 | Wire format changed? | N/A | VRRP is not a ze-internal wire format; RFC summaries cover the protocol |
| 8 | Plugin SDK/protocol changed? | N/A | no SDK changes -- existing callbacks suffice |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` rows for 9568 + 3768 with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (test/vrrp suite, QEMU vrrp lab targets) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (VRRP row vs FRR/BIRD/VyOS) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` only if macvlan API changes iface contracts (child 3 decides); else N/A with grep evidence |
| 13 | Route metadata keys added/changed? | N/A | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` or monitoring guide (metric names + labels) |
| 15 | Registered plugin/event/command inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Changed source files referenced by doc source anchors? | Yes | grep `docs/` for `source:` anchors on iface files touched by child 3 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` examples verified against final YANG |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the child spec being implemented |
| 2. Audit | Child's Files to Modify/Create + TDD plan |
| 3. Wiring phase | Child's Wiring Test table |
| 4. Implement (TDD) | Child's Implementation Phases |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` (+ QEMU targets for linux children) |
| 6. Critical review | Child's Critical Review Checklist |
| 7-9. Fix/re-verify loop | Until clean |
| 10. Deliverables review | Child's Deliverables Checklist |
| 11. Security review | Child's Security Review Checklist |
| 12. Documentation review | Umbrella Documentation Update Checklist rows owned by that child |
| 13. /ze-review gate | Child's Review Gate |
| 14. Present summary + close | Two-commit closure per child; umbrella closes after children 1-6 |

### Implementation Phases

Execution order = child numbering. Children 1+2 (pure Go) and 3 (iface) are
parallelizable; 4 needs 1; 5 needs 1-4; 6 needs 5; 7 is out of this pass.

1. **Phase: Wiring (MANDATORY FIRST, per child)** -- each child spec's phase 1 creates its entry points + failing wiring tests
2. **Phase: spec-vrrp-1-packet** -- codec + validation
3. **Phase: spec-vrrp-2-fsm** -- state machine + timers
4. **Phase: spec-vrrp-3-macvlan** -- iface macvlan support
5. **Phase: spec-vrrp-4-transport** -- sockets + GARP/NA + doctor
6. **Phase: spec-vrrp-5-plugin** -- plugin, YANG, commands, telemetry, docs
7. **Phase: spec-vrrp-6-interop** -- keepalived interop + QEMU lab
8. **Full verification** -- `make ze-verify` + QEMU targets
9. **Complete spec** -- per-child audits; umbrella Implementation Audit aggregates; two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child AC implemented with file:line; umbrella AC-1..6 demonstrated |
| Feature completeness | Every End-to-End User Story has a working path; parity with keepalived default feature set minus tracking (Known Limitations) |
| Correctness | v3 semantics from rfc/short/rfc9568.md, NOT from reference implementations (they are wrong on interval adoption, skew, checksum) |
| Naming | YANG kebab-case with unit suffixes; metrics ze_vrrp_*; plugin name "vrrp"; log subsystem "vrrp" |
| Data flow | vrrp -> iface for ALL kernel interface state; no direct netlink from the plugin |
| CLI grammar | typed selectors; verb-first; R1-R9 gate green |
| Registration over hardcoding | No vrrp spelling in central packages; all surfaces registered (plugin, RPC, YANG augment, doctor, events, metrics) |
| Doctor checks | doctor-vrrp-* codes registered + explainable via `ze explain` |
| YANG validation | every leaf max-constrained; custom validators registered |
| Prometheus counters | all listed metrics registered and incremented (no holo-style dead counters) |
| Rule: qemu-testing | every //go:build linux file covered by a QEMU target |
| Rule: interop-and-goal-validation | keepalived scenarios pass before umbrella close |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| vrrp plugin registered | `make ze-inventory` lists vrrp; TestAllPluginsRegistered count bumped |
| YANG augments live | `ze config validate` accepts the agreed config shape (.ci) |
| macvlan device lifecycle | QEMU integration test creates/deletes with correct MAC |
| GARP/NA on promotion | tcpdump assertion in effective-vrrp-keepalived.py |
| keepalived interop | interop scenarios green (container + QEMU paths) |
| VPP reject | verify .ci with vpp-backed interface fails with actionable error |
| Docs + rfc-status rows | `make ze-doc-test` green |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Decode bounds-checked (IHL-aware IPv4 strip; count-vs-length exact; no panics on malformed -- fuzz target on packet.Decode) |
| Spoofing resistance | TTL/hop-limit 255 check enforced (GTSM); packets failing any check dropped + counted, never processed |
| Resource exhaustion | Advert floods bounded: per-instance rx path is O(1), counters saturate, no per-packet allocations or goroutines |
| Privilege | Raw sockets need CAP_NET_RAW -- doctor check reports actionable error; no privilege escalation paths |
| State manipulation | prio-0 and election rules followed exactly; equal-priority tie-break by IP comparison prevents flapping |
| Error leakage | Log lines name interface/vrid/reason, never raw packet dumps at info level |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read rfc/short summary section -> child spec RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check child AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| QEMU lab flaky | Payload-predicate waits (never sleeps); see ai/rules/testing.md sleep ratchet |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| v3/IPv4 checksum includes a pseudo-header (umbrella RFC annotation, following uvrrpd) | RFC 9568 §5.2.8 is message-only for IPv4; pseudo-header is IPv6-only (`rfc/full/rfc9568.txt:880-885`, change note :194-196). Ecosystem split: legacy keepalived/uvrrpd used pseudo-header, recent keepalived/FRR/Arista/Cisco are message-only | spec-vrrp-1 drafting agent cross-checked the full RFC text against the umbrella annotation | Umbrella annotation corrected; spec-vrrp-1 revised to TX-strict-9568 + RX dual-accept with compat counter; holo digest bug-list entry 5 was itself wrong (holo is 9568-correct here) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- Both studied implementations (holo-vrrp, uvrrpd) independently got v3 interval adoption and skew arithmetic wrong; RFC summaries are the only safe source for FSM semantics.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Under-interface placement | standalone vrrp tree; VyOS named groups | User choice 2026-07-14; Junos/Nokia locality; augment mechanism proven by bgp plugins |
| macvlan virtual MAC day one | keepalived real-MAC mode first | User choice 2026-07-14; RFC-strict failover (no peer MAC re-learning) |
| v3 default, v2 opt-in | v3-only; v2 default | User choice 2026-07-14; RFC-current by default, keepalived-v2 interop preserved |
| Explicit ipv4/ipv6 containers | family inferred from addresses | User choice 2026-07-14; RFC 9568 separate virtual routers; structural validation |
| Group keyed by operator NAME, vrid a mandatory leaf | vrid as the list key (the original shape) | User correction 2026-07-15. Matches ze's `list peer { key "name" }` convention and cli-grammar "Identifiers Are Strings"; lets a vrid be renumbered without the tree seeing a different object; gives logs/show/metrics/events a stable human label. Costs one new rule (no duplicate vrid per unit+family) that the keyed shape got for free |
| Kernel-facing state hangs off the unit's DEVICE (eth0 or eth0.100), not the logical interface name | use the logical interface name everywhere | A unit with a vlan-id lives on a sub-interface (`internal/component/iface/config_apply.go:35-39`); binding the bare parent would advertise into the wrong broadcast domain, and two units of one interface would collide on a single transport InstanceKey and a single metric series |
| Internal milliseconds everywhere, convert at wire | native units per version | Kills the s/cs/ms confusion class (holo's 100x bug, Nokia's ds/cs split). REFINED by spec-vrrp-2: configured/learned INTERVALS are integer ms; COMPUTED durations (skew, master-down) are time.Duration, because valid v3 skews are sub-millisecond and integer-ms would reintroduce the truncate-to-zero bug |
| VIPs + sockets on macvlan via iface APIs | plugin-direct netlink (uvrrpd hook-script style) | iface owns kernel interface state; both dataplanes see one owner |
| No v2 authentication (type 0 only) | simple-text auth | Deprecated by RFC 9568; keepalived interop works with auth off; avoids false security |
| No tracking (interface/route/health) in this umbrella | Junos/Nokia/VyOS track features | Scope control; recorded in Known Limitations; future spec can add priority-decrement tracking |

## Known Limitations
- accept-mode false (the RFC 9568 default) is not dataplane-enforced in this pass: VIPs live as kernel addresses on the macvlan, so the Master's kernel answers traffic to VIPs regardless of the leaf (holo has the same consequence). RFC-strict non-accept filtering (dropping non-ND traffic to unowned VIPs) would need nftables integration -- deferred; the leaf exists and drives FSM/owner semantics, and interop scenarios needing pingable VIPs set accept-mode true explicitly (spec-vrrp-6 A-7).
- No tracking (interface/route/script health) in this pass -- priority is static per group. Follow-up spec candidate.
- GARP/NA burst parameters are internal constants (3 repeats x 100 ms, spec-vrrp-4); a config knob is deliberately out of scope for this umbrella.
- No sync-groups / vrrp-inherit; each group fails over independently.
- No VRRP unicast peers (RFC 9568 is multicast; keepalived unicast_peer is out of scope).
- VPP dataplane rejected at verify until spec-vrrp-7 (explicit, tested, fail-closed).
- v2 authentication not implemented (auth type 0 only).

## RFC Documentation

Add `// RFC 9568 Section X.Y: "<quoted requirement>"` (or RFC 3768) above enforcing
code. MUST document: receive validation rules, state transitions, timer constraints,
prio-0 handling, tie-breaks, checksum rules. Children carry the per-file lists.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ze speaks RFC 9568 VRRPv3 with another implementation | interop test | (fill: keepalived v3 scenario names + pass output) |
| ze speaks RFC 3768 VRRPv2 opt-in | interop test | (fill: keepalived v2 scenario) |
| RFC-strict virtual MAC failover | functional/QEMU test | (fill: tcpdump MAC assertions) |
| Operator workflow (config/show/doctor/metrics) | functional .ci tests | (fill: test names) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during /ze-review)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
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
- [ ] AC-1..AC-6 all demonstrated
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
