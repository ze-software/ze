# Spec: vpp-interface-in-use

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Validate VPP interface role conflicts and referenced-interface deletions before apply. |
| Depends | - |
| Phase | - |
| Updated | 2026-11-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md` and `ai/rules/spec-no-code.md`
3. `internal/component/vpp/register.go`, `internal/component/vpp/startup_feature.go`, and `internal/component/vpp/register_doctor_codes.go`
4. `internal/component/config/plugin_verify.go` and `internal/component/plugin/server/reload.go`
5. `internal/component/iface/interface_reference.go` and `internal/plugins/iface/vpp/mirror.go`, `lcp.go`, `tunnel.go`, `vxlan.go`

## Task

Ze's VPP-touching features validate their own configuration subtrees in isolation. While the data-plane compatibility guard and YANG `configured-interface` reference checks already cover several single-leaf and kernel-only exclusions, two cross-feature referential-integrity gaps remain under `interface backend vpp`:

1. **Conflicting VPP interface roles are not detected across features.** An interface (or VLAN sub-interface) can be claimed in incompatible VPP dataplane roles at the same time — for example, used as a SPAN mirror destination (`interface/*/unit/*/mirror/{ingress,egress}`) or an L2 bridge member (`interface/bridge/*/member`) while simultaneously targeted by a VPP forwarding feature (such as `traffic control`, `pppoe`, or any future registered VPP feature) or paired as a VPP Linux Control Plane (LCP) host TAP. No cross-feature check catches the conflict before apply.
2. **Deleting an interface or VLAN sub-interface still referenced by a VPP feature without `ze:reference "configured-interface"` is not blocked on commit or reload.** In particular, `traffic control interface <name>` under `traffic control backend vpp` does not carry `ze:reference "configured-interface"` (preserving static configs that name an unmanaged OS device when no `interface` entry existed), so removing a previously declared physical, dummy, or VPP tunnel interface (`gre`/`gretap`/`ipip`/`vxlan`), removing a referenced VLAN sub-interface (`<parent>.<vlan-id>`), or deleting a parent interface whose VLAN sub-interface is still referenced succeeds at verify and leaves dangling VPP dataplane state.

Add registration-based VPP interface-usage and role validation:

- Provide a shared, registration-based VPP interface-role registry in `internal/component/vpp` so each VPP feature package contributes its interface usages without `vpp` hardcoding any feature paths.
- At config verify (`ze config validate`, CLI/web/API commit, and daemon reload), aggregate all registered VPP interface roles over the candidate config tree, detect incompatible roles symmetrically in both directions, and reject with an operator error (`opererr`) naming the interface (or VLAN sub-interface), both conflicting references, why they conflict, and the fix. Report the same conflict verdict in `ze doctor` (`doctor-vpp-interface-in-use`, parsed-config phase) while never refusing `ze start` (honoring the 2026-11-26 "Ze always starts" ruling).
- On config transitions (editor/API/web commit and daemon reload), compare declared interface targets (physical, dummy, tunnel, wireguard, and other schema-declared interface kinds plus all `<parent>.<vlan-id>` VLAN sub-interfaces) between the previous and candidate trees. Reject deletion of any target still referenced by any registered VPP feature, mirror source, bridge member, or LCP-aware reference — including parent deletion that removes a referenced VLAN sub-interface — naming the exact removed target (and deleted parent when applicable), every referencing feature, and the fix.

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - VPP component lifecycle, registered startup features (`RegisterStartupFeature`), and LCP architecture.
  → Decision: VPP feature packages register pure candidate-tree readers into `internal/component/vpp` rather than `vpp` enumerating consumers.
  → Constraint: Any new VPP registry stays under the `ze_vpp` build tag and accepts pure, concurrent-safe candidate readers.
- [ ] `docs/architecture/iface/logical-name-resolution.md` - `ze:reference "configured-interface"` and how declared interface names and `<parent>.<vlan-id>` units are derived from the compiled YANG schema.
  → Decision: Interface kinds are discovered from schema list nodes under `/interface`, never from a hardcoded list of kinds, and `<parent>.<vlan-id>` is the canonical sub-interface name.
  → Constraint: Leaves outside `configured-interface` (such as `traffic control interface`) may legitimately name undeclared devices in static single-file configs, so deletion blocking must compare previous vs candidate declared targets rather than rejecting all undeclared names statically.
- [ ] `docs/architecture/core-design.md` - data-plane compatibility guard and the 2026-11-26 "Ze always starts" decision.
  → Decision: `ze config validate`, commit, and reload refuse configuration conflicts; `ze start` is not refused by an unscoped check, and `ze doctor` reports pre-existing config conflicts with their fix.
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - VPP firewall commit-time rejection matrix and global ACL/NAT44 binding behavior.
  → Constraint: Today VPP firewall rejects `input-interface`/`output-interface` matches and binds ACLs/NAT44 globally; its registered provider must emit no false-positive per-interface conflict for global chains.
- [ ] `docs/architecture/traffic/fw-7-traffic-vpp.md` - VPP traffic-control policer bindings per interface.
  → Constraint: `traffic control backend vpp` binds policers per named interface; those entries participate as VPP feature references.
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - VPP PPPoE access interface preparation and LCP requirement.
  → Constraint: VPP PPPoE explicitly requires `vpp lcp enabled true`, so LCP pairing and feature usage (`RoleFeature`) must coexist without false positives.
- [ ] `ai/rules/plugins.md` and `ai/doors/system.md` - plugin self-containment and `ze_<feature>` compile-tag boundaries.
  → Constraint: Always-on and non-`ze_vpp` packages (including `ze_flowexport`) must never import `internal/component/vpp`; every VPP role provider lives in a `ze_vpp`-gated package and registers from `init()`.

**Key insights:**
- Since the original spec draft, HEAD landed both `ze:reference "configured-interface"` (covering mirror destinations, bridge members, tunnel local interfaces, and PPPoE access interfaces) and `spec-dataplane-compatibility-guard` (gating `/interface/bridge` and `flow-export sampling`/`conntrack` as kernel-only under `interface backend vpp`).
- The remaining real gap is a shared registration-based VPP interface-role registry plus transition-aware deletion checking for VPP features not covered by `configured-interface` (notably `traffic control`) and cross-feature role-conflict checking (mirror destination, bridge member, LCP pairing, and VPP features) across physical, dummy, tunnel, and VLAN sub-interface targets.
- Existing VPP traffic `.ci` tests (`test/traffic/traffic-vpp-accept-multiclass.ci`, `traffic-vpp-accept-dscp-filter.ci`, `traffic-vpp-not-connected.ci`) declare `interface { backend vpp }` with no `ethernet eth0` entry alongside `traffic { control { backend vpp; interface eth0 { ... } } }`; transition-based deletion checks preserve those static configs while blocking removal of any previously configured target.
- Wiring the verifier through `config.RegisterCandidateVerifier` (invoked by `VerifyPluginConfigMapTransition` and `reloadConfig`) plus a `DoctorPhaseParsedConfig` check (`doctor-vpp-interface-in-use`) refuses validate, commit, and reload while respecting the 2026-11-26 "Ze always starts" ruling.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD 2026-11-26):**
- [ ] `internal/component/vpp/register.go` - `verifyVPPConfig` (wired as `InProcessConfigVerifier`) parses the `vpp` root and registered startup features via `ParseConfigSections`, then calls `parsed.Validate()`. No interface-role or deletion cross-reference runs here.
- [ ] `internal/component/vpp/startup_feature.go` - `RegisterStartupFeature` provides the existing registration pattern by which `ze_vpp` feature packages supply pure `*config.Tree` readers to `vpp`.
- [ ] `internal/component/vpp/register_doctor_codes.go` - registers VPP doctor diagnostic codes (`doctor-vpp-process`, `doctor-vpp-stats`, `doctor-vpp-dpdk`, `doctor-vpp-hugepages`, `doctor-vpp-cpu-isolation`); no code exists yet for VPP interface-role conflicts.
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` receives both `previousTree` and `configTree` during `ze config validate` (`previousTree == nil`) and editor/web/API commit transitions (`previousTree != nil`), calling each plugin's `InProcessConfigVerifier` (which sees only candidate sections, with deleted whole roots sent as `{}`). It has no whole-tree `(previous, candidate)` candidate verifier registry yet.
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` holds `running` (`s.reactor.ReloadRunning(newTree)`) and `newTree` and computes `config.DiffMaps(running, newTree)`, but runs no registered whole-tree candidate/transition verifiers before `runTxCoordinator`.
- [ ] `internal/component/iface/interface_reference.go` and `internal/component/iface/register_interface_reference.go` - `configuredInterfaceReference` (`ze:reference "configured-interface"`) already refuses undeclared or deleted targets (including dummy/tunnel VLAN units and parent deletions) for `mirror/{ingress,egress}`, `bridge/member`, `tunnel/.../local/interface`, `pppoe-client/source-interface`, and `pppoe/interface/name`.
- [ ] `internal/plugins/iface/vpp/mirror.go`, `lcp.go`, `tunnel.go`, `vxlan.go`, `ifacevpp.go`, `register.go` - `SetupMirror` (`sw_interface_span_enable_disable`), `SetupLCPPair` (`lcp_itf_pair_add_del`, called for VPP `dummy` interfaces in `internal/component/iface/config_apply.go`), `createGRETunnel`/`createIPIPTunnel`/`createVxlanTunnel`, and `CreateBridge`/`BridgeAddPort`. None register interface roles for cross-feature conflict validation.
- [ ] `internal/plugins/traffic/vpp/verify.go`, `register.go`, `internal/component/traffic/yang/ze-traffic-control-conf.yang` - `trafficvpp.Verify` validates per-interface qdisc/class/filter semantics only; `traffic/control/interface/name` does not carry `ze:reference "configured-interface"` and is never checked against interface deletions or role conflicts.
- [ ] `internal/plugins/firewall/vpp/verify.go`, `backend_linux.go`, `nat_linux.go`, `register.go` - `firewallvpp.Verify` rejects per-interface `input-interface`/`output-interface` matches and `flowtable` carries `ze:backend "kernel"`; filter ACLs and NAT44 SNAT/masquerade bind globally to all live VPP interfaces.
- [ ] `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - registers `pppoe-vpp-control-plane` via `config.RegisterTreeValidator` and `pppoe` via `vpp.RegisterStartupFeature`, requiring `vpp enabled true` and `vpp lcp enabled true`, but does not report `pppoe/interface/<name>` to any VPP interface-role conflict check.
- [ ] `internal/plugins/flowexport/config.go`, `register.go` - `flow-export sampling` and `flow-export conntrack` are declared `Planes: ["kernel"]` in `DataPlaneFeatures` and rejected under `interface backend vpp` by `refuseDataPlaneMismatches`; `flowexport` sits under `ze_flowexport` (not `ze_vpp`).
- [ ] `internal/component/iface/operation.go` - `decomposeIfaceOperations` emits transaction ordering ops for creates/destroys, not referential-integrity rejection.

**Behavior to preserve:**
- Every feature's existing single-subtree validation, `ze:backend` feature gates, `DataPlaneFeatures` checks, and `ze:reference "configured-interface"` checks stay unchanged.
- `ze start` (`LoadConfigForStart`) is never refused by VPP interface-role checks; pre-existing role conflicts in a config file on disk are reported by `ze doctor` under `doctor-vpp-interface-in-use`.
- Valid single-role VPP interface assignments (feature-only, mirror-destination-only, LCP-paired-only, or bridge-member-only in unit tests) continue to pass.
- Valid coexisting roles — specifically an LCP-paired VPP interface carrying a VPP forwarding feature (such as `traffic control` or `pppoe`) — continue to pass with no false positives.
- Static initial configs with `interface { backend vpp }` and `traffic { control { backend vpp; interface eth0 { ... } } }` where `eth0` was never declared in `interface {}` (as pinned by `test/traffic/traffic-vpp-accept-multiclass.ci`, `traffic-vpp-accept-dscp-filter.ci`, and `traffic-vpp-not-connected.ci`) continue to pass.
- Deleting an unreferenced VPP `dummy` interface (whose only role is its own implicit LCP pair) continues to pass; `DeleteInterface` tears down the LCP pair cleanly.
- Builds without `ze_vpp` continue to compile cleanly with zero always-on or cross-tag imports of `internal/component/vpp`.

**Behavior to change:**
- Incompatible VPP interface roles on any interface or VLAN sub-interface are rejected at `ze config validate`, commit, and reload with an `opererr` naming the interface (or sub-interface) and both conflicting references, and reported by `ze doctor` under `doctor-vpp-interface-in-use`.
- Deleting a previously declared VPP interface (physical, dummy, tunnel, or other schema-declared kind) or VLAN sub-interface — or deleting a parent interface whose VLAN sub-interface is referenced — while still referenced by any registered VPP feature or mirror source is rejected on commit and reload with an `opererr` naming the exact removed target and referencing feature(s).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| YANG `ze:reference "configured-interface"` gate | `internal/component/iface/interface_reference.go` (`configuredInterfaceReference`, `configuredInterfaceNames`), `docs/architecture/iface/logical-name-resolution.md` | Enforces at `ze config validate`, boot, and commit that `mirror/{ingress,egress}`, `bridge/member`, `tunnel/.../local/interface`, `pppoe-client/source-interface`, and `pppoe/interface/name` name an interface or `<parent>.<vlan-id>` unit declared in the candidate `interface` section | Does not replace; complements it: `configured-interface` already blocks dangling deletions for annotated leaves, while this spec adds cross-feature role-conflict detection and transition-based deletion guards for features without `configured-interface` (notably `traffic control`) and for LCP-aware diagnostics |
| Engine-owned data-plane compatibility guard and `ze:backend` walk | `internal/component/config/dataplane_verdict.go` (`ResolvePrimaryDataPlane`, `refuseDataPlaneMismatches`), `internal/component/config/backend_gate.go` (`walkBackendFeaturesTree`) | Resolves the primary data plane (`interface backend`), enforces `ze:backend` annotations (including `/interface/bridge` as `kernel` today), and enforces `DataPlaneFeatures` (including `flow-export sampling` and `conntrack` as `kernel`) across `LoadConfig`, `verifyTree`, and `ze doctor` | Reuses `config.ResolvePrimaryDataPlane` to gate VPP role validation only when VPP is active; does not relax any existing kernel-only backend gate |
| VPP startup-feature registration pattern | `internal/component/vpp/startup_feature.go` (`RegisterStartupFeature`) | Allows `ze_vpp`-gated feature packages to register pure `*config.Tree` readers into `internal/component/vpp` from `init()` without `vpp` enumerating consumers | Extends the same pattern with `vpp.RegisterInterfaceRoleProvider` for cross-feature VPP interface-usage declarations |
| Transition-aware commit verification and daemon reload | `internal/component/config/plugin_verify.go` (`VerifyPluginConfigMapTransition`), `internal/component/plugin/server/reload.go` (`reloadConfig`) | Both hold the previous/running config tree alongside the candidate config tree on validate (`previous == nil`), commit, and reload | Extends both doors via `config.RegisterCandidateVerifier` / `config.RunCandidateVerifiers` so candidate conflict checks and transition deletion checks run identically across validate, CLI/web/API commits, and SIGHUP/daemon reloads without blocking `ze start` |
| Parsed-config doctor check framework | `internal/component/vpp/register_doctor_codes.go`, `internal/component/config/dataplane_verdict.go` (`DoctorPhaseParsedConfig`) | Runs config-tree readiness checks in `ze doctor` under registered diagnostic codes with explicit `Fix` strings | Extends with `vpp-interface-in-use` (`doctor-vpp-interface-in-use`) reusing the same pure conflict checker |
| Interface apply-ordering decomposer | `internal/component/iface/operation.go` (`decomposeIfaceOperations`) | Orders interface and address create/remove operations during commit transactions | Does not apply for verify-time rejection: ordering runs only after verify succeeds; this spec rejects invalid configs during verify |
| Per-backend firewall and traffic verifiers | `internal/plugins/firewall/vpp/verify.go` (`Verify`), `internal/plugins/traffic/vpp/verify.go` (`Verify`) | Validate single-component VPP expression and qdisc compatibility in `OnConfigVerify` and `InProcessConfigVerifier` | Does not replace: each stays focused on its own component; their `ze_vpp` registration files additionally register VPP interface-role providers into `vpp` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "A compile tag is a boundary, and code MUST NOT reach across it." | agrees | `vpp.RegisterInterfaceRoleProvider` and all VPP role providers live exclusively in `ze_vpp`-gated packages; `config.RegisterCandidateVerifier` lives in always-on `config` with zero import of `vpp`, and `ze_flowexport` does not import `vpp`. |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/component/vpp/*`, `internal/plugins/iface/vpp/*`, `internal/plugins/traffic/vpp/*`, `internal/plugins/firewall/vpp/*` | "Exact or refuse, at verify." | agrees | Role conflicts and deletions of referenced VPP interfaces/VLAN units are refused at verify time with an operator error rather than failing partway through VPP binary API apply. |
| `ai/doors/config-yang.md` | Boundaries | the door covering `internal/component/config/plugin_verify.go` | "A backend that cannot apply exactly refuses at verify (`ai/rules/protocol.md`)." | agrees | Candidate role conflicts and transition deletions run via `config.RegisterCandidateVerifier` on `ze config validate`, commit, and reload before any apply runs. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/server/reload.go` | "The engine never enumerates plugins." | agrees | `reloadConfig` calls the generic `config.RunCandidateVerifiers` registry with zero knowledge of VPP or any plugin name. |
| `ai/doors/access-security.md` | Where it sits | the door covering `internal/component/l2tp/plugins/pppoe-vpp/startup.go` | "The allowed imports of each are the table in `docs/architecture/core-design.md`, section 19." | agrees | `pppoe-vpp/startup.go` already lives under `ze_l2tp && ze_vpp` and imports `internal/component/vpp`; adding its `RegisterInterfaceRoleProvider` call introduces no new import edge. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/vpp-interface-in-use.ci` | "**A bare `go test` omits the feature tags**, so plugins do not register and unrelated tests fail." | agrees | Every test command in this spec runs through `./le test unit pkg path ...` and `./le test functional plugin ...` so `ze_vpp` and all feature tags are linked. |
| `docs/architecture/config/yang-config-design.md` | Layer 3: Plugin config verifiers | the `// Design:` page of `internal/component/config/plugin_verify.go` | "A plugin declares `InProcessConfigVerifier` on its registration and receives its own config subtree, before and after, at `VerifyPluginConfig` and `VerifyPluginConfigContentTransition`." | agrees | Adding `config.RegisterCandidateVerifier` in `plugin_verify.go` preserves existing `InProcessConfigVerifier` behavior while adding a cross-root `(previous, candidate)` verification hook invoked at validate, commit, and reload. |
| `docs/architecture/config/syntax.md` | An Absent Config Root Delivers No Section At All | anchored to `internal/component/config/plugin_verify.go` | "A component that declares a config root is called with that root's section, and it is NOT called when the operator wrote no block for it." | agrees | `BuildPluginConfigSections` and `ExtractConfigSubtree` stay unchanged; the new whole-tree candidate verifier runs alongside them without altering per-root section delivery. |
| `docs/architecture/core-design.md` | 14a. Commit-Time Backend Capability Gate | anchored to `internal/component/config/plugin_verify.go` | "The config component runs one walk over the whole config tree as written (before any schema default is materialized) at its two config doors (`LoadConfig` and `verifyTree`) and in `ze doctor` (`doctor-dataplane-mismatch`), driven by every declared `DataPlaneSelector`." | agrees | This spec preserves the `ze:backend` walk unchanged and reuses `config.ResolvePrimaryDataPlane` to gate VPP role validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Existing kernel-only data-plane exclusions (including `flow-export sampling`/`conntrack` and `/interface/bridge`) and host-side exceptions (`fib kernel`, `sysctl`) stay unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Data-plane mismatch handling in `dataplane_verdict.go` is untouched, and VPP interface-role checks obey the superseding 2026-11-26 start ruling below while sharing one verdict across validate, commit, reload, and doctor. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | The VPP firewall role provider does not touch YANG `ze:backend` annotations or alter `firewallvpp.Verify` rules. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | All backend checks in this spec compare exclusively against `vpp` / `kernel` through `config.ResolvePrimaryDataPlane` and `config.ResolveDataPlaneSelector`, using no retired backend names. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | VPP interface-role validation does not touch BGP configuration or export next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | VPP interface-role validation does not touch BGP policy chains or next-hop authoring forms. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | VPP interface-role validation does not touch BGP engine filter steps or `FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | VPP interface-role validation does not touch BGP UPDATE forwarding or export-chain folding. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | VPP interface-role validation does not touch BGP import or export next-hop validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | VPP interface-role validation does not touch BGP route origination or next-hop forwarding. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | VPP interface-role validation adds no filter-chain members and leaves implied-filter display untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | VPP interface-role validation registers via `config.RegisterCandidateVerifier` (refusing `ze config validate`, commit, and reload as a whole) and reports pre-existing conflicts via `ze doctor` (`doctor-vpp-interface-in-use`), never making `ze start` exit or refuse to start. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | VPP interface-role validation follows this exact pattern: `ze config validate`, commit, and reload refuse conflicts and referenced deletions, `ze start` stays unblocked, and `ze doctor` reports any pre-existing conflict with its fix under `doctor-vpp-interface-in-use`. |
| `docs/architecture/plugin/plugin-system.md` | Dependencies | anchored to `internal/component/config/plugin_verify.go` and `internal/component/plugin/server/reload.go` | "The set those doors resolve starts from `ExtractPluginsFromTree`." | agrees | `refuseUnresolvableDependencies` and plugin dependency resolution in `verifyTree` and `reloadConfig` stay unchanged. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/plugin/plugin-system.md` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | `config.RegisterCandidateVerifier` runs purely over parsed trees after schema publication and does not alter external-plugin query-mode declaration loading. |
| `docs/architecture/api/process-protocol.md` | Config Reload Protocol | the `// Design:` page of `internal/component/plugin/server/reload.go` | "If any plugin rejects, the reload is aborted." | agrees | Running `config.RunCandidateVerifiers` in `reloadConfig` before `runTxCoordinator` aborts invalid role conflicts or referenced deletions before any plugin apply runs, preserving atomic verify-before-apply reload semantics. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation and changes no plugin identity or authorization profile. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `docs/architecture/api/process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec changes no event queue or engine-to-plugin event delivery path. |
| `docs/architecture/behavior/signals.md` | Startup Paths | anchored to `internal/component/plugin/server/reload.go` | - | silent | Anchored only to `ConfigTransactionDone` in `reload.go`; this spec does not change signal handling, SIGHUP queueing, or transaction lock signaling. |
| `docs/architecture/config/apply-ordering.md` | Phases 2 and 5, for BGP | anchored to `internal/component/plugin/server/reload.go` | - | silent | Anchored only to `appendDecomposingPlugins` in `reload.go`; candidate verification runs before `runTxCoordinator` and does not alter operation decomposition or apply ordering. |
| `docs/architecture/config/transaction-protocol.md` | Shutdown while a transaction runs | anchored to `internal/component/plugin/server/reload.go` | - | silent | Anchored only to `stopTransaction` and shutdown fencing in `reload.go`; candidate verification runs synchronously before `runTxCoordinator` and does not alter shutdown fencing or rollback context. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | the `// Design:` page of `internal/component/vpp/register_doctor_codes.go` and anchored to `internal/component/plugin/server/reload.go` | "A check over such state registers in the `runtime` phase, through the same `diagnostic.RegisterDoctorCheck` every other check uses" | agrees | Because VPP interface-role conflicts are pure config-tree facts (not daemon-only runtime state), the new `vpp-interface-in-use` check registers in `DoctorPhaseParsedConfig` with its `doctor-vpp-interface-in-use` metadata in `register_doctor_codes.go`, so offline `ze doctor` and `show doctor` both report it. |
| `docs/research/vpp-deployment-reference.md` | startup.conf Syntax | the `// Design:` page of `internal/component/vpp/register.go`, `internal/plugins/iface/vpp/register.go`, and modified by this spec | "The VPP component does not enumerate the forwarding features that consume it." | agrees | `vpp.RegisterInterfaceRoleProvider` mirrors `RegisterStartupFeature` so each VPP consumer registers its own pure candidate reader and `internal/component/vpp` never enumerates consumers; the page will document the new registry with source anchors. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | Config-verify interface-role validation does not add any unmanaged VPP runtime mode or alter VPP process supervision. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | Pure config-tree validation runs before VPP runtime lifecycle and preserves external VPP management semantics unchanged. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | Interface-usage validation is pure config verification and never tears down or alters live VPP reconnect adoption. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | This spec adds verify-time config guards only and does not interfere with crash detection or IGP/BGP model updates. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | Pure config validation touches no VPP health-probe or loss-declaration timers. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | Pure config validation does not modify VPP termination, pidfd exit proof, or restart ownership. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | Pure config validation runs before apply and leaves both VPP recovery forms unchanged. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | Config validation rejects broken role combinations before apply without affecting restart withdrawal timing. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | Pure config validation imposes no systemd unit policy and does not inspect systemd units. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | Pure config validation introduces no runtime timers and leaves the two-limit failure lifecycle unchanged. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | Pure config validation does not alter live VPP adoption or background reconciliation. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | Pure config validation does not modify recovery deadlines, crash-loop limits, or withdrawal behavior. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | Pure config validation does not modify post-withdrawal recovery backoff or route restoration. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | Pure config validation does not touch protocol timers or runtime loss handling. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | Pure config validation never probes VPP's API socket or refuses based on live process identity. |
| `docs/architecture/firewall/fw-6-firewall-vpp.md` | What VPP programs, everything else rejected at commit | the `// Design:` page of `internal/plugins/firewall/vpp/register.go` and anchored to `internal/component/vpp/register.go` | "Everything else rejects at commit with a message naming the unsupported expression" | agrees | The VPP firewall provider plugs into the VPP interface-role registry without changing the VPP firewall rejection matrix or falsely flagging global filter/NAT44 chains. |
| `docs/architecture/traffic/fw-7-traffic-vpp.md` | Exact or reject | the `// Design:` page of `internal/plugins/traffic/vpp/register.go` | "If a backend cannot apply EXACTLY what the operator's config asks for, the verifier fails at commit with a clear error." | agrees | Registering `traffic/control/interface` references with VPP ensures conflicting roles and referenced deletions fail at commit rather than failing partway through VPP policer apply. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Decisions | the `// Design:` page of `internal/component/l2tp/plugins/pppoe-vpp/startup.go` | "No feature-specific branch in VPP selects PPPoE." | agrees | `pppoe-vpp/startup.go` registers its interface-role provider into `vpp` alongside `RegisterStartupFeature`; `internal/component/vpp` never imports or names PPPoE. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | agrees | Pure config-tree interface-role registration in `pppoe-vpp` does not alter VPP API loss recovery or owner retirement. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | Pure config validation does not modify adopted VPP session cleanup or identity checks. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | agrees | Pure config validation does not touch PPPoE session adoption, process leases, or teardown behavior. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | Pure config validation does not affect PPPoE VPP recovery forms or background reconciliation. |

## Data Flow (MANDATORY)

### Entry Point
- **Validate and commit doors (`VerifyPluginConfigMapTransition`):** `internal/component/config/plugin_verify.go` invokes `RunCandidateVerifiers(previousTree, candidateTree)` for every call to `VerifyPluginConfig` / `VerifyPluginConfigTransition` / `VerifyPluginConfigContentTransition` (`ze config validate` with `previous == nil`, and CLI editor, web, and API commits with `previous != nil`).
- **Daemon reload door (`Server.reloadConfig`):** `internal/component/plugin/server/reload.go` invokes `config.RunCandidateVerifiers` over `running` (`s.reactor.ReloadRunning(newTree)`) and `newTree` before starting `runTxCoordinator` (covering SIGHUP reload and daemon reload RPC).
- **Readiness diagnosis (`ze doctor` / `show doctor`):** `vpp-interface-in-use` (`DoctorPhaseParsedConfig`, code `doctor-vpp-interface-in-use`) runs the same pure candidate role-conflict checker over the parsed config tree so hand-edited configs on disk surface conflicts while `ze start` stays unblocked.

### Transformation Path
1. **Data-plane gate:** Resolve the primary data plane via `config.ResolvePrimaryDataPlane(tree, schema)`. When the candidate (for single-tree checks) or neither previous nor candidate (for transition checks) selects `vpp`, return `nil` immediately.
2. **Registration-based usage collection:** Run every registered `vpp.InterfaceRoleProvider` (in deterministic sorted registration-name order) over the candidate `*config.Tree`. Each provider returns value-type `InterfaceUsage` entries `(Interface, Role, Source)`:
   - `iface/vpp` provider: `RoleMember` for active `interface/bridge/<br>/member`; `RoleMirrorDestination` for active unit `mirror/ingress` and `mirror/egress` (naming the mirroring source interface and unit); `RoleLCPPaired` for active `interface/dummy/<name>` when `vpp/lcp/enabled` resolves to `true`.
   - `traffic/vpp` provider: `RoleFeature` for active `traffic/control/interface/<name>` when `traffic/control/backend` resolves to `vpp`.
   - `firewall/vpp` provider: `RoleFeature` for any per-interface VPP firewall binding when `firewall/backend` resolves to `vpp` (none for global filter/NAT44 chains).
   - `pppoe-vpp` provider: `RoleFeature` for active `pppoe/interface/<name>` when `pppoe.ConfiguredSessionBackendName(tree)` resolves to `vpp`.
3. **Symmetric role-conflict detection:** Group collected usages by target interface name (sorted for deterministic diagnostics). For every pair of roles on the same target, check `incompatibleRoles(a, b)` symmetrically:
   - `RoleMember` conflicts with `RoleFeature`, `RoleMirrorDestination`, and `RoleLCPPaired`.
   - `RoleMirrorDestination` conflicts with `RoleFeature`, `RoleMember`, and `RoleLCPPaired`.
   - `RoleLCPPaired` coexists with `RoleFeature` (PPPoE requires LCP; routed LCP interfaces may carry traffic-control QoS).
   - On conflict, collect an `opererr` naming the target interface (or VLAN sub-interface), both conflicting `Source` descriptions, why VPP cannot apply both roles to one interface, and the exact fix (remove one of the two conflicting config entries).
4. **Declared-target extraction (transitions, when `previous != nil`):** Walk the compiled `interface` schema lists (`config.YANGSchema()` -> `schema.Lookup("interface")`, matching `configuredInterfaceNames`) over both `previous` and `candidate`, collecting every active declared interface entry (physical `ethernet`, `dummy`, `tunnel`, `wireguard`, `bridge`, `veth` and its `peer`, `xfrm`, `pppoe-client`) and every active VLAN sub-interface `<parent>.<vlan-id>` along with its parent interface name.
5. **Referenced-deletion check (transitions, when `previous != nil`):** For each target `T` present in `previous` declared targets and absent from `candidate` declared targets (sorted deterministically):
   - Inspect all candidate usages referencing `T`, excluding only a self-owned `RoleLCPPaired` entry on `T` itself (deleting an unreferenced `dummy` interface remains permitted).
   - If one or more references remain in `candidate` (a VPP feature, mirror source, bridge member, or cross-interface reference — plus note if `T` also held an active LCP pairing on `previous` per AC-7), reject with an `opererr`:
     - If `T` is a VLAN sub-interface `<parent>.<vlan-id>` whose parent still exists in `candidate`, name `T` explicitly (AC-9).
     - If `T` is a VLAN sub-interface whose parent was deleted from `candidate`, name `T` explicitly AND note that deleting its parent removes `T` (AC-10).
     - Name every referencing source in `candidate` and give the exact fix (remove the referencing configuration before deleting the interface or unit).
6. **Return:** Join all findings deterministically via `errors.Join`; return `nil` when no conflict or dangling deletion exists.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze_vpp` feature plugins ↔ `internal/component/vpp` | Value-type `InterfaceUsage` returned by `InterfaceRoleProvider` registered via `vpp.RegisterInterfaceRoleProvider` in `init()` | [ ] |
| `internal/component/vpp` ↔ `internal/component/config` | Registers pure candidate/transition verifier via `config.RegisterCandidateVerifier`; `config` never imports `vpp` | [ ] |
| `internal/component/plugin/server` ↔ `internal/component/config` | `reloadConfig` calls `config.RunCandidateVerifiers` over `running` and `newTree`; server never imports `vpp` | [ ] |

### Changed-Wire Census

No wire format, RPC message, IPC framing, YANG leaf, or protocol encoding changes in this spec.

**Search evidence:** Scoped search across `internal/component/vpp/`, `internal/component/config/plugin_verify.go`, `internal/component/plugin/server/reload.go`, `internal/plugins/{iface,traffic,firewall}/vpp/`, and `internal/component/l2tp/plugins/pppoe-vpp/` for `pkg/plugin/rpc` message structs, `yang/*.yang` modules, and GoVPP binapi requests (positive control: `pkg/plugin/rpc/types.go` `ConfigVerifyInput` and `internal/component/vpp/yang/ze-vpp-conf.yang`). All edits are strictly in-process Go verification callbacks (`RegisterCandidateVerifier`, `RegisterInterfaceRoleProvider`, `DoctorPhaseParsedConfig`) returning standard Go `opererr` errors and `diagnostic.Diagnostic` entries with no new or changed wire fields.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none (no wire changed) | writer / reader unchanged | `pkg/plugin/rpc/types.go` (`ConfigVerifyInput`) — positive control verified untouched | Validation runs in-process inside `config` and `plugin/server` before plugin RPCs; zero wire schemas or encodings change | Preserved unchanged; proven by existing config/reload tests |

### Integration Points
- `internal/component/config/plugin_verify.go` (`RegisterCandidateVerifier`, `RunCandidateVerifiers`, `snapshotCandidateVerifiers`) - new candidate/transition verifier registry invoked from `VerifyPluginConfigMapTransition`.
- `internal/component/plugin/server/reload.go` (`reloadConfig`) - calls `config.RunCandidateVerifiers` on `running` and `newTree` before `runTxCoordinator`.
- `internal/component/vpp/register.go` (`init`) - registers `vpp-interface-refs` via `config.RegisterCandidateVerifier` and registers the `vpp-interface-in-use` parsed-config doctor check.
- `internal/component/vpp/register_doctor_codes.go` - registers `doctor-vpp-interface-in-use` diagnostic metadata.
- `internal/plugins/iface/vpp/register.go`, `internal/plugins/traffic/vpp/register.go`, `internal/plugins/firewall/vpp/register.go`, `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - each calls `vpp.RegisterInterfaceRoleProvider` from `init()`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Validate, commit, and reload all run through `RunCandidateVerifiers`; `ze doctor` runs the same pure conflict check in `DoctorPhaseParsedConfig`; `ze start` stays unblocked per 2026-11-26 ruling |
| No unintended coupling (components stay isolated) | Yes | `config` and `plugin/server` never import `vpp`; `vpp` never imports `iface`, `traffic`, `firewall`, `pppoe`, or `flowexport`; all providers live in `ze_vpp`-gated packages |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `config.ResolvePrimaryDataPlane`, `config.YANGSchema`, `diagnostic.Register`, and mirrors `vpp.RegisterStartupFeature` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Pure config validation path; no packet/wire hot path touched |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Features register `InterfaceRoleProvider` callbacks via `vpp.RegisterInterfaceRoleProvider`; `vpp/verify_refs.go` discovers all providers from the registry and interface kinds from `config.YANGSchema()` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `plugin_verify.go`, `reload.go`, `vpp/register.go`, `vpp/register_doctor_codes.go`, and `feature-gates.txt`: no hardcoded verifier name list or feature list exists; `candidateVerifiers`, `interfaceRoleProviders`, and `diagnostic.Register` are all self-registering registries |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A shared, registration-based VPP interface-role registry in `internal/component/vpp` keeps all feature packages decoupled and respects the `ze_vpp` build tag | `internal/component/vpp/startup_feature.go` (`RegisterStartupFeature`) and `feature-gates.txt` (`ze_vpp` lists `component/vpp`, `plugins/{iface,traffic,firewall}/vpp`, and `pppoe-vpp`) | Cross-tag import failure caught by `./le arch tier check` | `./le arch tier check` and unit tests | confirmed |
| A-2 | VPP interface roles in HEAD span bridge membership (`RoleMember`), SPAN mirror destinations (`RoleMirrorDestination`), LCP pairing (`RoleLCPPaired`), and VPP forwarding features (`RoleFeature`: `traffic control`, `pppoe`, per-interface `firewall`), over physical, dummy, tunnel (`gre`/`gretap`/`ipip`/`vxlan`), and VLAN sub-interface (`<parent>.<vlan-id>`) targets | Direct read of `internal/plugins/iface/vpp/{mirror,lcp,tunnel,vxlan,ifacevpp}.go`, `internal/plugins/traffic/vpp/verify.go`, `internal/plugins/firewall/vpp/verify.go`, `internal/component/l2tp/plugins/pppoe-vpp/startup.go`, `internal/component/iface/interface_reference.go`, and owner decision 2026-07-10 | Additional VPP features added later simply call `RegisterInterfaceRoleProvider` with zero `vpp` edit | Unit tests and `test/plugin/vpp-interface-in-use.ci` | confirmed |
| A-3 | Adding `config.RegisterCandidateVerifier` invoked from `VerifyPluginConfigMapTransition` and `reloadConfig` gives full `(previous, candidate)` trees on validate, commit, and reload while leaving `ze start` (`LoadConfigForStart`) unblocked per the 2026-11-26 decision | Direct read of `internal/component/config/plugin_verify.go`, `internal/component/plugin/server/reload.go`, `docs/architecture/core-design.md` (Design decisions 2026-11-26), and `test/traffic/traffic-vpp-accept-multiclass.ci` | A validate, commit, or reload door could miss checks | Unit tests on both doors + `test/plugin/vpp-interface-in-use.ci` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Hardcoding feature paths or interface kinds in `internal/component/vpp` violates plugin self-containment | Reviewer or `./le arch enumeration check` flags a central list | All features register via `vpp.RegisterInterfaceRoleProvider`; interface kinds are read from `config.YANGSchema()` |
| R-2 | False positives reject legitimate VPP configs (such as LCP-paired dummy/ethernet interfaces used by `pppoe` or `traffic control`, global VPP firewall chains, deleting an unreferenced dummy interface, or static `traffic-vpp-accept-*.ci` configs with no `ethernet` stanza) | Existing `test/traffic/*`, `test/vpp/*`, or `test/plugin/*` suites fail | Scope conflicts strictly to incompatible role pairs (`RoleMember` or `RoleMirrorDestination` vs `RoleFeature`/`RoleLCPPaired`/each other); exclude self-only LCP pairing on unreferenced deletion; run deletion check only on transitions where the target existed in `previous` |
| R-3 | `VerifyPluginConfigMapTransition` and `reloadConfig` hold lowered `map[string]any` rather than `*config.Tree` | Reconstruction via `TreeFromPluginMap` fails or drops active nodes | Both doors already hold active-only lowered maps (`ActiveConfigMap`); convert via `config.TreeFromPluginMap` (the exact pattern already used by `refuseRedistributeRulesMap` in `plugin_verify.go` and `parseStartupSections` in `startup_feature.go`) |
| R-4 | Package-level provider/verifier registries contaminate parallel or repeated unit tests (`go test -count=2`) | Second test run fails with duplicate registration or leaked test provider | Provide `snapshotInterfaceRoleProviders` and `snapshotCandidateVerifiers` test helpers restored via `t.Cleanup` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false positive rejects valid VPP configs at validate/commit/reload; a false negative allows conflicting VPP roles or dangling interface deletions to reach apply |
| How is it reverted? | Single commit revert; no persisted state, schema, or wire format changes |
| Who else touches this path? | `internal/component/config/plugin_verify.go` and `internal/component/plugin/server/reload.go` are shared validate/commit/reload doors; changes there are additive hook calls that no-op when no candidate verifier is registered or when `interface backend` is not `vpp` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Candidate config with incompatible VPP interface roles (feature + mirror destination, or LCP-paired + mirror destination) submitted to `ze config validate`, commit, or `ze doctor` | → | `VerifyPluginConfigMapTransition` / `reloadConfig` -> `RunCandidateVerifiers` -> `verifyVPPInterfaceRefs` -> registered providers -> conflict `opererr` (and `doctor-vpp-interface-in-use` in `ze doctor`) | `test/plugin/vpp-interface-in-use.ci` |
| Commit/reload deleting a VPP physical or dummy interface still referenced by `traffic control` | → | `VerifyPluginConfigMapTransition` / `reloadConfig` -> `RunCandidateVerifiers` -> `verifyVPPInterfaceRefs` -> dangling-reference `opererr` | `test/plugin/vpp-interface-in-use.ci` |
| Commit/reload deleting an interface still used as a SPAN mirror destination | → | YANG `configured-interface` / `verifyVPPInterfaceRefs` -> rejects deletion naming the mirroring source | `test/plugin/vpp-interface-in-use.ci` |
| Commit/reload removing a VLAN sub-interface (`<parent>.<vlan-id>`) or deleting its parent while the sub-interface is still referenced by a VPP feature or mirror | → | `verifyVPPInterfaceRefs` / `configured-interface` -> rejects removal naming the exact sub-interface | `test/plugin/vpp-interface-in-use.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Under `interface backend vpp`, an interface is claimed both by a VPP feature (`RoleFeature`: e.g. `traffic control`, `pppoe`, or a registered VPP firewall/flow-export provider) AND by an exclusive member/sink role (`RoleMember` bridge member or `RoleMirrorDestination` SPAN sink) | Config verify (`ze config validate`, commit, reload) rejects with an `opererr` naming the interface, both conflicting references, why they conflict, and the fix command; `ze doctor` reports the same finding under `doctor-vpp-interface-in-use`; detection holds identically regardless of provider registration order | Non-VPP configs (`interface backend kernel`) are unaffected because this registry validates VPP dataplane role constraints only |
| AC-2 | Under `interface backend vpp`, a previously declared interface is still referenced by a VPP feature (such as `traffic control`) and is deleted from `interface` in a commit or reload transition | Transition verify rejects the deletion with an `opererr` naming the deleted interface, the referencing feature, and the fix | Static single-file configs with no previous tree where `traffic control` names an interface absent from `interface {}` remain accepted, preserving `test/traffic/traffic-vpp-accept-*.ci` and unmanaged-device mode |
| AC-3 | Under `interface backend vpp`, an interface is used by a single VPP feature (or coexisting compatible roles such as LCP pairing plus `traffic control` or `pppoe`) with no conflicting member or sink role | Config verify accepts the configuration | Valid single-role and compatible multi-role VPP configs are the intended accepted path |
| AC-4 | Under `interface backend vpp`, an interface has only a single exclusive role (bridge-member-only in unit test, mirror-destination-only, or unreferenced LCP-paired dummy) | Config verify accepts the configuration, and deleting an unreferenced LCP-paired `dummy` interface succeeds | Single-role exclusivity permits one owner and permits deleting an unreferenced interface |
| AC-5 | Any conflict or referenced-deletion refusal produced by the VPP interface-usage verifier | The `opererr` message names the exact affected interface (or VLAN sub-interface), both conflicting references (or the referencing feature(s) on deletion), why VPP cannot apply the config, and the exact operator fix on its last line | No bare internal error strings or Go symbols permitted in user-facing output |
| AC-6 | Under `interface backend vpp`, an interface is configured as a SPAN mirror destination (`mirror ingress` or `mirror egress` of another interface's unit) and is deleted from `interface` | Verify rejects the deletion naming the destination interface and the mirroring source interface | Deletion succeeds once the mirroring source's `mirror` reference is removed in the same or an earlier commit |
| AC-7 | Under `interface backend vpp` with `vpp lcp enabled true`, an LCP-paired interface is assigned an incompatible role (bridge member or SPAN mirror destination) or is deleted while still referenced by a VPP feature | Usage aggregation records the `RoleLCPPaired` role; conflicting-role commits reject naming the LCP pairing and conflicting role, and referenced deletions account for and name the LCP-paired status alongside the referencing feature | Deleting an unreferenced LCP-paired `dummy` interface with no other feature/mirror reference remains permitted because `DeleteInterface` removes the LCP pair cleanly |
| AC-8 | Under `interface backend vpp`, a VPP-created tunnel interface (`gre`, `gretap`, `ipip`, or `vxlan`) is referenced by a VPP feature (such as `traffic control`) or as a SPAN mirror destination, then deleted from `interface` | Verify rejects the deletion naming the tunnel interface and referencing feature/source, identical to physical and dummy interfaces | Tunnel deletion succeeds once no VPP feature or mirror references it |
| AC-9 | Under `interface backend vpp`, a VLAN sub-interface (`<parent>.<vlan-id>`) of a dummy, ethernet, or bridgeable tunnel (`gretap`) interface is referenced by a VPP feature or as a mirror destination, then that `unit` is removed while its parent stays | Verify rejects the removal and explicitly names the `<parent>.<vlan-id>` sub-interface (not only the parent) and the referencing feature/source | Removing an unreferenced VLAN unit while its parent stays remains permitted |
| AC-10 | Under `interface backend vpp`, a parent interface with VLAN sub-interfaces is deleted while one of its VLAN sub-interfaces (`<parent>.<vlan-id>`) is still referenced by a VPP feature or mirror | Verify rejects the parent deletion and explicitly names the referenced `<parent>.<vlan-id>` sub-interface (stating that deleting the parent removes it) and the referencing feature/source | Deleting a parent whose VLAN sub-interfaces have no remaining references remains permitted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Assigns a VPP interface both as a SPAN mirror destination and under `traffic control` (or as an LCP-paired dummy and SPAN mirror destination) and validates or commits | CLI/validate/commit/reload -> `VerifyPluginConfigMapTransition` / `reloadConfig` -> `RunCandidateVerifiers` -> `verifyVPPInterfaceRefs` -> registered providers -> `opererr` refusal (and `ze doctor` reports `doctor-vpp-interface-in-use`) | `test/plugin/vpp-interface-in-use.ci` |
| 2 | Deletes a VPP dummy, tunnel (`gre`/`gretap`/`ipip`/`vxlan`), VLAN unit (`dummy0.100`/`tun0.100`), or parent interface while `traffic control` or a SPAN mirror still references it | Commit/reload -> `VerifyPluginConfigMapTransition` / `reloadConfig` -> `RunCandidateVerifiers` -> `verifyVPPInterfaceRefs` -> `opererr` refusal naming exact target and referencing feature | `test/plugin/vpp-interface-in-use.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPInterfaceConflictFeatureAndMember` | `internal/component/vpp/verify_refs_test.go` | AC-1, AC-5: feature+bridge-member and feature+mirror-destination conflicts rejected symmetrically in both directions, naming interface, both refs, and fix | |
| `TestVPPInterfaceDeleteWhileReferenced` | `internal/component/vpp/verify_refs_test.go` | AC-2, AC-5: transition deleting an interface still referenced by a VPP feature is rejected; static validate without previous tree passes | |
| `TestVPPInterfaceSingleRoleOK` | `internal/component/vpp/verify_refs_test.go` | AC-3, AC-4: single-role feature, bridge-member-only, mirror-destination-only, LCP+feature coexistence, and unreferenced dummy deletion all pass | |
| `TestVPPMirrorDestinationDelete` | `internal/component/vpp/verify_refs_test.go` | AC-6: deleting a SPAN mirror target rejects and names the mirroring source interface | |
| `TestVPPLCPRoleConflictAndDelete` | `internal/component/vpp/verify_refs_test.go` | AC-7: LCP pairing participates in conflict checks (bridge member / mirror destination) and referenced-deletion diagnostics | |
| `TestVPPTunnelTargetDelete` | `internal/component/vpp/verify_refs_test.go` | AC-8: referenced `gre`/`gretap`/`ipip`/`vxlan` tunnel targets cannot be deleted | |
| `TestVPPVLANUnitDelete` | `internal/component/vpp/verify_refs_test.go` | AC-9: removing a referenced `<parent>.<vlan-id>` unit while its parent stays rejects and names the sub-interface | |
| `TestVPPParentDeleteWithReferencedUnit` | `internal/component/vpp/verify_refs_test.go` | AC-10: deleting a parent whose `<parent>.<vlan-id>` unit is referenced rejects and names the sub-interface | |
| `TestRegisterInterfaceRoleProviderValidation` | `internal/component/vpp/verify_refs_test.go` | Registration guards: empty/invalid name, nil provider, duplicate registration refused; snapshot/restore prevents test contamination | |
| `TestVPPInterfaceInUseDoctorCheck` | `internal/component/vpp/verify_refs_test.go` | `vpp-interface-in-use` parsed-config doctor check reports `doctor-vpp-interface-in-use` with fix on conflicts and passes on valid VPP configs | |
| `TestCandidateVerifierHooks` | `internal/component/config/plugin_verify_test.go` | `RegisterCandidateVerifier` runs from `VerifyPluginConfigMapTransition` for both `previous == nil` and `previous != nil`, and rejects invalid registrations | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (referential integrity over interface name strings, no new numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-interface-in-use` | `test/plugin/vpp-interface-in-use.ci` | Exercises AC-1 through AC-10 through real `ze` config validate/commit/reload and `ze doctor`: role conflicts fail naming both references and surface in `ze doctor`, referenced deletions (physical/dummy, mirror destination, LCP-paired, tunnel, VLAN unit, and parent with referenced VLAN unit) fail naming the exact target and referencing feature, and valid single-role/compatible configs pass | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A (Scope: config — validation only, no wire protocol) | - | - | validated by unit and functional `.ci` tests | - |

## Files to Modify
- `internal/component/config/plugin_verify.go` - add `RegisterCandidateVerifier`, `RunCandidateVerifiers`, and `snapshotCandidateVerifiers`, and invoke `RunCandidateVerifiers` from `VerifyPluginConfigMapTransition`
- `internal/component/plugin/server/reload.go` - call `config.RunCandidateVerifiers` in `reloadConfig` against `running` and `newTree` before `runTxCoordinator`
- `internal/component/vpp/register.go` - register `vpp-interface-refs` via `config.RegisterCandidateVerifier` and register the `vpp-interface-in-use` parsed-config doctor check in `init()`
- `internal/component/vpp/register_doctor_codes.go` - register `doctor-vpp-interface-in-use` diagnostic metadata
- `internal/plugins/iface/vpp/register.go` - register the `iface-vpp` interface role provider (bridge members, mirror destinations, LCP-paired interfaces)
- `internal/plugins/traffic/vpp/register.go` - register the `traffic-vpp` interface role provider (`traffic/control/interface` when backend is `vpp`)
- `internal/plugins/firewall/vpp/register.go` - register the `firewall-vpp` interface role provider (per-interface bindings when backend is `vpp`)
- `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - register the `pppoe-vpp` interface role provider (`pppoe/interface` when PPPoE session backend is `vpp`)
- `docs/research/vpp-deployment-reference.md` - document `RegisterInterfaceRoleProvider` and VPP interface-usage verification with source anchors

## Files to Create
- `internal/component/vpp/verify_refs.go` - `InterfaceRoleKind`, `InterfaceUsage`, `InterfaceRoleProvider`, `RegisterInterfaceRoleProvider`, schema-driven target discovery, symmetric role-conflict checks, transition deletion checks, and parsed-config doctor check
- `internal/component/vpp/verify_refs_test.go` - unit tests covering AC-1 through AC-10, provider registration guards, and doctor check
- `internal/plugins/iface/vpp/interface_roles.go` - VPP interface role provider for bridge members, SPAN mirror destinations, and LCP-paired interfaces
- `internal/plugins/traffic/vpp/interface_roles.go` - VPP traffic-control interface role provider
- `internal/plugins/firewall/vpp/interface_roles.go` - VPP firewall interface role provider
- `test/plugin/vpp-interface-in-use.ci` - functional test proving AC-1 through AC-10 end-to-end

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Validation only; no new YANG leaves, containers, or RPCs |
| YANG validation constraints | No | No YANG schema additions |
| YANG custom validators | No | Uses `config.RegisterCandidateVerifier` over the whole tree |
| CLI commands/flags | No | No new CLI commands or flags |
| CLI grammar (keyword before value) | N-A | No CLI command added or changed |
| Editor autocomplete | No | No completion surface changed |
| Functional test for new RPC/API | Yes | `test/plugin/vpp-interface-in-use.ci` tests config validation, commit/reload, and `ze doctor` behavior end-to-end |
| Pipe completeness | N-A | No command output added |
| Env var registration | No | No new env vars |
| Doctor check for runtime dependencies | No | Pure config-tree validation; adds no new runtime dependency, socket, path, or binary |
| Doctor code per problem class | Yes | `doctor-vpp-interface-in-use` registered in `internal/component/vpp/register_doctor_codes.go`, checked in `DoctorPhaseParsedConfig` by `vpp-interface-in-use` in `internal/component/vpp/verify_refs.go` |
| Prometheus counters/metrics | No | Validation-only; no runtime state to meter |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Validation fix on existing VPP dataplane surface |
| 2 | Config syntax changed? | No | Validation only |
| 3 | CLI command added/changed? | No | No CLI command changes |
| 4 | API/RPC added/changed? | No | No API/RPC changes |
| 5 | Plugin added/changed? | No | Existing `vpp` component and VPP backend plugins only |
| 6 | Has a user guide page? | No | `docs/guide/vpp.md` documents VPP configuration and remains accurate |
| 7 | Wire format changed? | No | No wire format changes |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/` SDK or protocol changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement changes |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner |
| 11 | Affects daemon comparison? | No | Internal validation hardening only |
| 12 | Internal architecture changed? | Yes | `docs/research/vpp-deployment-reference.md` (add VPP interface-role registry section with source anchors) |
| 13 | Route metadata keys added/changed? | No | Not route metadata |
| 14 | Prometheus counters added/changed? | No | No metrics added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No plugin registration inventory or command catalog changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/research/vpp-deployment-reference.md` (updated). Unaffected anchored pages (existing anchored symbols and contracts stay unchanged): `docs/architecture/firewall/fw-6-firewall-vpp.md`, `docs/architecture/traffic/fw-7-traffic-vpp.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/core-design.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/apply-ordering.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/behavior/signals.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/DESIGN.md`, `docs/features/interfaces.md`, `docs/guide/bfd.md`, `docs/guide/config-reload.md`, `docs/guide/ddos-mitigation.md`, `docs/guide/firewall.md`, `docs/guide/migrating-from-vyos.md`, `docs/guide/plugins.md`, `docs/guide/pppoe.md`, `docs/plugin-development/schema.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Existing examples remain valid single-role VPP configs |

## Design presentation

### What changes

Under `interface backend vpp`, Ze gains a registration-based VPP interface-usage verifier (`vpp.RegisterInterfaceRoleProvider`) wired into config validation, commit/reload transitions (`config.RegisterCandidateVerifier`), and `ze doctor` (`doctor-vpp-interface-in-use`, parsed-config phase). Each `ze_vpp`-gated feature package (`iface/vpp`, `traffic/vpp`, `firewall/vpp`, `pppoe-vpp`) registers a pure candidate-tree reader declaring which interfaces or `<parent>.<vlan-id>` VLAN sub-interfaces it uses and in what role (`RoleMember`, `RoleMirrorDestination`, `RoleLCPPaired`, `RoleFeature`). Config validate, commit, and reload refuse incompatible role combinations on the same target before apply (while `ze start` stays unblocked per the 2026-11-26 ruling and `ze doctor` reports pre-existing conflicts on disk), and commit/reload transitions refuse removing any previously declared physical, dummy, tunnel, or VLAN sub-interface (including via parent deletion) while any registered VPP feature or mirror source still references it.

### Before and after

1. **Conflicting VPP interface roles (SPAN mirror destination + `traffic control` QoS):**
   - Before: A VPP config declaring `ethernet eth0` mirroring ingress traffic to `dummy0` while also configuring `traffic control backend vpp` on `dummy0` passes `ze config validate`, commit, and `ze doctor`, leaving VPP with both a SPAN sink and a QoS policer attached to the same interface.
   - After: `ze config validate`, commit, and reload fail with an operator error: `VPP interface "dummy0" has conflicting roles: mirror ingress from eth0 unit default (mirror-destination) and traffic control interface dummy0 (feature)` explaining why VPP cannot apply both roles to one interface, with the fix on the last line to remove one of the conflicting entries. `ze doctor` reports the same conflict under `doctor-vpp-interface-in-use`.
2. **Deleting a VPP interface or tunnel still referenced by `traffic control`:**
   - Before: Committing a change that removes `tunnel gre0` (or `dummy d0`) from `interface {}` while leaving `traffic control backend vpp` referencing `gre0` (or `d0`) passes commit and fails later at runtime when VPP cannot resolve the interface.
   - After: Commit and reload fail before apply with an operator error naming the removed interface (`gre0`) and the referencing feature (`traffic control interface gre0`), with the fix to remove the `traffic control` entry first or keep the interface.
3. **Removing a VLAN sub-interface or deleting its parent while referenced:**
   - Before: Committing a change that deletes `unit 100` from `dummy d0` (or deletes `dummy d0` altogether) while `traffic control` still references `d0.100` succeeds at verify and leaves a dangling VPP policer reference.
   - After: Commit and reload fail before apply naming `d0.100` explicitly (and stating that deleting parent `d0` removes `d0.100` in the parent-delete case) alongside the referencing feature and exact fix command.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Central hardcoded walk of `traffic`, `firewall`, `pppoe`, and `mirror` config paths inside `internal/component/vpp` | Violates `ai/rules/plugins.md` and `docs/research/vpp-deployment-reference.md` ("The VPP component does not enumerate the forwarding features that consume it"); every new VPP feature would require editing `vpp`. |
| Unscoped `config.RegisterTreeValidator` that refuses `LoadConfigForStart` | Conflicts with the 2026-11-26 "Ze always starts" design decision in `docs/architecture/core-design.md`: a daemon start must not exit over a single feature's misconfiguration; validate, commit, and reload refuse while `ze doctor` reports pre-existing conflicts on disk. |
| Annotating `traffic/control/interface/name` unconditionally with `ze:reference "configured-interface"` | Breaks existing supported mode and pinned tests (`test/traffic/traffic-vpp-accept-multiclass.ci`, `traffic-vpp-accept-dscp-filter.ci`, `traffic-vpp-not-connected.ci`) where a static config runs `traffic control` on an interface with no `ethernet` stanza under `interface {}`. Transition-based deletion checks block removing previously declared interfaces without breaking unmanaged-device configs. |
| Importing `internal/component/vpp` from `internal/plugins/flowexport` | Violates `ai/doors/system.md` compile-tag boundary: `flowexport` is gated by `ze_flowexport`, not `ze_vpp`, so a `ze_flowexport && !ze_vpp` build would fail to compile; furthermore `flow-export sampling` and `conntrack` are already kernel-only under `DataPlaneFeatures`. |
| Putting transition deletion state inside `vppLifecycle` (`OnConfigVerify`) alone | `OnConfigVerify` does not run in offline commit validation (`VerifyPluginConfigTransition` / CLI editor pre-commit checks), causing editor commit and daemon reload to disagree. A pure `config.RegisterCandidateVerifier` covers every validate, commit, and reload door uniformly with no mutable state. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — add `config.RegisterCandidateVerifier` / `RunCandidateVerifiers` / `snapshotCandidateVerifiers` in `internal/component/config/plugin_verify.go`, wire `config.RunCandidateVerifiers` into `VerifyPluginConfigMapTransition` (`plugin_verify.go`) and `reloadConfig` (`plugin/server/reload.go`), create the `vpp.RegisterInterfaceRoleProvider` registry and no-op verifier + doctor check in `internal/component/vpp/verify_refs.go`, register them in `internal/component/vpp/register.go` and `register_doctor_codes.go`, and write failing `test/plugin/vpp-interface-in-use.ci` plus `TestCandidateVerifierHooks`.
   - Tests: `TestCandidateVerifierHooks`, `TestRegisterInterfaceRoleProviderValidation`, failing `test/plugin/vpp-interface-in-use.ci`
   - Files: `internal/component/config/plugin_verify.go`, `internal/component/plugin/server/reload.go`, `internal/component/vpp/verify_refs.go`, `internal/component/vpp/register.go`, `internal/component/vpp/register_doctor_codes.go`, `test/plugin/vpp-interface-in-use.ci`
   - Verify: entry points are reachable on validate, commit, reload, and `ze doctor`; wiring tests fail because conflict/deletion checks are stubs.
2. **Phase: Role providers and declared-target extraction** — implement `interface_roles.go` providers in `internal/plugins/iface/vpp`, `internal/plugins/traffic/vpp`, `internal/plugins/firewall/vpp`, and `internal/component/l2tp/plugins/pppoe-vpp/startup.go`, plus schema-driven declared-target extraction (physical, dummy, tunnel, wireguard, bridge, veth+peer, and `<parent>.<vlan-id>` units with parent attribution) in `internal/component/vpp/verify_refs.go`.
   - Tests: `TestVPPInterfaceSingleRoleOK`, `TestVPPLCPRoleConflictAndDelete`
   - Files: `internal/plugins/iface/vpp/{interface_roles.go,register.go}`, `internal/plugins/traffic/vpp/{interface_roles.go,register.go}`, `internal/plugins/firewall/vpp/{interface_roles.go,register.go}`, `internal/component/l2tp/plugins/pppoe-vpp/startup.go`, `internal/component/vpp/verify_refs.go`
3. **Phase: Conflict matrix, transition deletion checks, and doctor check** — implement symmetric `incompatibleRoles` checking, transition deletion validation (including tunnel targets, VLAN unit removal, parent deletion naming referenced VLAN units, and `opererr` diagnostics per AC-5), and the `vpp-interface-in-use` parsed-config doctor check in `internal/component/vpp/verify_refs.go`.
   - Tests: all unit tests in `internal/component/vpp/verify_refs_test.go` covering AC-1 through AC-10 and `TestVPPInterfaceInUseDoctorCheck`
   - Files: `internal/component/vpp/verify_refs.go`, `internal/component/vpp/verify_refs_test.go`
4. **Phase: Functional test and documentation** — complete `test/plugin/vpp-interface-in-use.ci` covering AC-1 through AC-10 (conflicts, deletions, mirror destinations, LCP pairing, tunnels, VLAN units, parent deletion, `ze doctor`, and valid controls) and document `RegisterInterfaceRoleProvider` with source anchors in `docs/research/vpp-deployment-reference.md`.
   - Scoped main-thread verification commands:
     - `./le test unit pkg path ./internal/component/config/`
     - `./le test unit pkg path ./internal/component/plugin/server/`
     - `./le test unit pkg path ./internal/component/vpp/`
     - `./le test unit pkg path ./internal/plugins/iface/vpp/`
     - `./le test unit pkg path ./internal/plugins/traffic/vpp/`
     - `./le test unit pkg path ./internal/plugins/firewall/vpp/`
     - `./le test unit pkg path ./internal/component/l2tp/plugins/pppoe-vpp/`
     - `./le test functional plugin vpp-interface-in-use`
     - `./le test functional traffic`
     - `./le arch tier check`
     - `./le plugin boundary check`
     - `./le arch doctor-coverage check`
     - `./le doc check all`
5. **Phase: Review Gate and closure** — run `/ze-review` + `/ze-review-spec`, fix findings, and complete two-commit closure via `/ze-close`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-10 has an implementation at file:line and a passing unit + functional assertion |
| Correctness | Symmetric bidirectional role-conflict detection; exact sub-interface naming on AC-9 and AC-10; zero false positives on LCP+feature, global firewall, unreferenced dummy delete, and static `traffic-vpp-accept-*.ci` |
| Data flow | Candidate conflicts and transition deletions run in `RunCandidateVerifiers` on `VerifyPluginConfigMapTransition` and `reloadConfig`; `ze doctor` runs the same conflict checker in `DoctorPhaseParsedConfig`; `ze start` stays unblocked |
| Operator errors and doctor codes | Every refusal is an `opererr` naming the affected interface/sub-interface, both sides of the conflict/reference, why it matters, and the fix command on the last line; `doctor-vpp-interface-in-use` carries registered metadata and `Fix` (`ai/rules/user-facing-errors.md`) |
| Registration over hardcoding | `vpp/verify_refs.go` imports no feature package and hardcodes no feature paths or interface kind lists; compile-tag isolation passes `./le arch tier check` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Candidate/transition verifier hooks in `config` and `plugin/server` | `./le test unit pkg path ./internal/component/config/` and `./le test unit pkg path ./internal/component/plugin/server/` |
| VPP interface-role registry, conflict matrix, deletion verifier, and doctor check | `./le test unit pkg path ./internal/component/vpp/` (passes with `-count=2`) and `./le arch doctor-coverage check` |
| Registered VPP feature providers (`iface/vpp`, `traffic/vpp`, `firewall/vpp`, `pppoe-vpp`) | `./le test unit pkg path` for each package + `./le arch tier check` + `./le plugin boundary check` |
| End-to-end functional proof (AC-1 through AC-10) | `./le test functional plugin vpp-interface-in-use` and `./le test functional traffic` |
| Updated architecture reference with verified source anchors | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error messages | Name interfaces and config paths only; never echo secrets or credentials from unrelated config subtrees |
| Denial of service | Target extraction and usage aggregation are linear in declared interface + feature reference counts with bounded deterministic sorting and no I/O |
| Fail-closed validation | Schema lookup errors return explicit `opererr` refusals rather than silently skipping checks |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error or cross-tag import violation | Fix in the phase that introduced it (`./le arch tier check`) |
| Existing `test/traffic/*` or `test/vpp/*` regression | Check R-2 false-positive boundaries in `verify_refs.go` |
| Functional test fails on behavior mismatch | Re-read producer code in Current Behavior; if design assumption broke, route to DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches and ask the user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-2 (original): only VPP bridge membership is a member role | Follow-up VPP waves added SPAN mirror destinations, LCP pairing, and VPP tunnel targets (`gre`/`gretap`/`ipip`/`vxlan`) and VLAN sub-interfaces | Code audit (2026-07-10, 2026-08-18) | Widened roles and target discovery to mirror, LCP, tunnels, and VLAN units (AC-6..AC-10) |
| Original draft: `flow-export` and `interface bridge` were active on VPP and `mirror` lacked deletion guards | HEAD (2026-11-26) already landed `ze:reference "configured-interface"` for `mirror`/`bridge`/`tunnel`/`pppoe` and `spec-dataplane-compatibility-guard` gating `/interface/bridge` and `flow-export sampling`/`conntrack` as kernel-only; `flowexport` is under `ze_flowexport`, not `ze_vpp` | Source audit against HEAD (2026-11-26) | Retired landed guards into `## Existing Mechanisms`; kept `flowexport` decoupled from `ze_vpp`; scoped new work to the registration-based role registry, conflict checks, and transition deletion checks |

## Design Insights
<!-- LIVE -->

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Registration-based `vpp.RegisterInterfaceRoleProvider` inside `internal/component/vpp` (matching `RegisterStartupFeature`) | Central hardcoded walk in `vpp`; putting registry in always-on `iface` | Keeps `vpp` free of feature imports, keeps all VPP role providers inside `ze_vpp` packages, and requires zero edits to `vpp` when new VPP features arrive |
| Pure `config.RegisterCandidateVerifier` wired into `VerifyPluginConfigMapTransition` and `reloadConfig`, paired with `DoctorPhaseParsedConfig` (`doctor-vpp-interface-in-use`) | Unscoped `RegisterTreeValidator` blocking `ze start`; stateless `ze:reference "configured-interface"` on `traffic/control/interface/name`; mutable state in `vppLifecycle` | Refuses `ze config validate`, commit, and reload uniformly while respecting the 2026-11-26 "Ze always starts" ruling, surfaces pre-existing disk conflicts in `ze doctor`, and preserves static unmanaged-device configs (`test/traffic/traffic-vpp-accept-*.ci`) |
| LCP pairing (`RoleLCPPaired`) coexists with `RoleFeature` and conflicts with exclusive L2/sink roles (`RoleMember`, `RoleMirrorDestination`) | Treating LCP pairing as conflicting with all features | VPP PPPoE explicitly requires `vpp lcp enabled true` (`pppoe-vpp/startup.go`) and VPP dummies auto-pair when LCP is enabled (`iface/config_apply.go`); allowing coexistence avoids false positives while still catching sink/bridge conflicts and referenced deletions |
| Keep `internal/plugins/flowexport` out of `ze_vpp` imports | Calling `vpp.RegisterInterfaceRoleProvider` directly from `flowexport` | `flowexport` is gated by `ze_flowexport`, not `ze_vpp`; importing `vpp` would break `ze_flowexport && !ze_vpp` builds (`ai/doors/system.md`), and `flow-export sampling`/`conntrack` are already kernel-only via `DataPlaneFeatures` |

## Known Limitations
- VPP xconnect and VPP bond member roles are out of scope because Ze has no VPP xconnect or bond features today; when added, their `ze_vpp` packages will register `RoleMember` via `vpp.RegisterInterfaceRoleProvider` with no change to `vpp`.
- `/interface/bridge` carries `ze:backend "kernel"` in HEAD (`test/parse/iface-vpp-rejects-bridge.ci`), so bridge-member conflicts on VPP are proven directly in Go unit tests and will apply automatically at commit if VPP bridge domains are later enabled in YANG.

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Scope, Priority, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets (`ai/rules/spec-no-code.md`)
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow (with Changed-Wire Census), and Design presentation completed
- [ ] AC-1..AC-10 carry positive observable expectations and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration and Documentation checklists answered Yes/No/N-A

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] Scoped main-thread verification commands pass (`./le test unit pkg path ...`, `./le test functional plugin vpp-interface-in-use`, `./le test functional traffic`, `./le arch tier check`, `./le plugin boundary check`, `./le arch doctor-coverage check`, `./le doc check all`)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
