# Spec: ipsec-dataplane-inspection

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 9/9 (independent closure review) |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The task and original research below record the starting defect. The closure
sections record the implemented contract, proof, and remaining publication work.

Ze cannot see its own IPsec dataplane. Every IPsec surface it exposes reports what
the IKE engine believes at install time, and nothing in the product reads the
kernel back.

`saToMap` (`internal/component/ike/cmd/show_ipsec.go`) builds `show vpn ipsec sa`
from `engine.ActiveTable()` and `engine.PeerInfoMap()`. `buildIPsecSATableData`
(`internal/component/web/page_vpn_ipsec.go`) uses the same table. `espInstalled`
(`internal/component/ike/engine/metrics.go`) feeds `ze_ipsec_tunnel_up` and
`ze_ipsec_tunnel_degraded` from `ChildSA.ESPInstalled`, and `child.go` sets that
field at the point an install call returns nil. The field records that a call
succeeded. It does not record that the kernel still holds the SA. A kernel
expiry, an external flush, or a rekey that strands a policy all leave
`ze_ipsec_tunnel_up` at 1 while the tunnel carries nothing.

The operator has no fallback. `gokrazy/ze/config.json` builds the appliance from
two Ze binaries plus gokrazy `randomd` and `heartbeat`. There is no iproute2 and
no busybox, so `ip xfrm state` does not exist on an appliance. The only tool that
can show kernel XFRM state is `ze`.

The read path is nearly built. `xfrmBackend.ListSAs`
(`internal/component/ike/dataplane/xfrm_linux.go`) already issues
`netlink.XfrmStateList`, and it has no non-test caller anywhere in `internal/`,
`cmd/`, or `pkg/`. It discards everything except SPI, source, destination, and
if_id.

Give Ze a dataplane read surface: a SAD dump, an SPD dump, a view that compares
engine belief against kernel truth, a doctor check that the dataplane is
reachable, an appliance kernel floor for ESP, and the byte counters that the
command grammar has advertised since 2026-06-03 and never emitted.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/kernel-profiles.md` - a kernel profile is a pair of files in an open registry
- [ ] `docs/architecture/ike/ipsec-10-cli-diag.md` - the presentation layer over the IKE engine: show and clear commands, web, observability
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the native IKEv2 state machine above the wire codec and the crypto layer
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - ESP Child SA creation after IKE_AUTH, and the dataplane abstraction
- [ ] `docs/architecture/ike/ipsec-dataplane-inspection.md` - what the kernel holds, against what the IKE engine believes at install time
- [ ] `docs/architecture/testing/qemu-integration.md` - QEMU integration testing for the Linux-only code paths
- [ ] `docs/features/ai-first.md` - register once, expose everywhere: one command and discovery surface
- [ ] `plan/immediate/spec-ipsec-dataplane-inspection.md` - this spec: IPsec dataplane inspection
- [ ] `ai/patterns/cli-command.md` and `ai/rules/cli.md` - the shape of a new command.
  → Constraint: a per-object lookup takes a typed selector container (the `peer name` precedent in `ze-ipsec-cmd.yang`), never a bare positional, and no `--flag` may appear anywhere in a `.yang` file, descriptions included.
  → Decision: reading kernel state is a `show` verb, not `debug`, however low-level the data is.
- [ ] `ai/rules/cli.md` - output routing.
  → Constraint: an RPC handler returns `plugin.Map` and never formats. `command.ApplyPipes` (`internal/component/command/pipe.go`) dispatches to `ApplyTable`/`ApplyJSON`, and `renderList` (`internal/component/command/pipe_table.go`) derives table columns from the JSON keys sorted alphabetically. A handler that calls `ApplyPipes` itself is the violation.
- [ ] `ai/rules/repo-maintenance.md` - the surface a new runtime-dependency check owes.
  → Constraint: the check lives in the owning package, registers through `registry.DoctorCheckDef`, carries a `doctor-`-prefixed code in `internal/core/diagnostic/codes.go`, and needs both a unit test and a functional `.ci`.
- [ ] `ai/rules/evidence.md` - a zero value must never be a valid-looking answer.
  → Constraint: `vppBackend.ListSAs` (`internal/component/ike/dataplane/vpp.go`) and `noopDataplane.ListSAs` (`noop.go`) both return `(nil, nil)`. Rendered as a table that reads "no SAs installed", which is the exact fail-open shape this rule bans.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps.
  → Constraint: a read-only dump passes on an empty kernel with its body deleted. Every kernel assertion must name an SPI or address the test itself installed, and must assert a transition (present, then absent), not a state.
- [ ] `ai/rules/completion.md` - every exported symbol has a non-test caller.
  → Decision: this spec is the first production caller of `ListSAs`, and of `ownerOf` in `internal/component/ike/dataplane/policy_owner.go`.
- [ ] `ai/rules/platform-linux.md` - Linux-only code ships with QEMU tests.
  → Constraint: `//go:build integration && linux` files are auto-enrolled by `ZE_QEMU_INTEGRATION_PKGS` (`internal/le/integration/gates.go`), which greps for that build line, so a new file in an already-enrolled package needs no the native action tables under `internal/le/` edit.
- [ ] `ai/rules/plugins.md` - removing the plugin removes the feature.
  → Constraint: the commands, their YANG, and their handlers stay in `internal/component/ike/`. `xfrmAvailable` (`internal/plugins/ospf/doctor_ipsec_linux.go`) is unexported and belongs to OSPF. Duplicate its two-line probe in the ike package rather than creating a cross-plugin dependency.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - Security Association Database and Security Policy Database.
  → Constraint: RFC 4301 Section 4.4 defines the SPD and the SAD as two distinct databases. The dump commands mirror that split rather than merging both into one view, because a policy without a matching state is the exact failure this surface exists to show.
- [ ] `rfc/short/rfc7296.md` - Child SA rekey.
  → Constraint: a rekey replaces the SPI while the selector stays identical, so an SPI-set comparison across a rekey is the discriminating interop assertion.

**Key insights:** (minimal context to resume after compaction)
- Every existing IPsec surface reads engine belief. Nothing reads the kernel.
- `ListSAs` exists, dumps through `netlink.XfrmStateList`, and has no production caller.
- `netlink.XfrmState` (vendored) carries `Limits`, `Statistics`, `Replay`, `ESN`, `Encap`, `Mode`, `Reqid`. `SAInfo` keeps four of those fields and drops the rest.
- `ze doctor` runs offline against a config file. `ActiveTable()` returns nil in that process, so belief-versus-kernel reconciliation cannot be a doctor check. It belongs on the live surfaces.

## Current Behavior

The reading below is the research baseline: what the tree held before this
spec's implementation started.

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/dataplane/dataplane.go` - `Dataplane` has four write methods (`InstallSA`, `RemoveSA`, `InstallPolicy`, `RemovePolicy`, `RemovePolicyParams`) and one read, `ListSAs(ifID uint32) ([]SAInfo, error)`. `SAInfo` carries `SPI`, `Src`, `Dst`, `IfID`. There is no `ListPolicies`. `Get()` returns the active backend and returns nil when none is loaded.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmBackend.ListSAs` calls `netlink.XfrmStateList(netlink.FAMILY_ALL)`, filters on `ifID` when non-zero, and copies four fields per state.
- [ ] `internal/component/ike/dataplane/xfrm_other.go` - the non-Linux stub returns `ErrNotSupported` wrapped with the GOOS name.
- [ ] `internal/component/ike/dataplane/vpp.go` - `vppBackend.ListSAs` returns `(nil, nil)`.
- [ ] `internal/component/ike/dataplane/noop.go` - `noopDataplane.ListSAs` returns `(nil, nil)`.
- [ ] `internal/component/ike/dataplane/policy_owner.go` - `policyOwners`, `ownerOf`, `policySelectorKey`. Ze's own shadow SPD, keyed on the selector fields the kernel compares. `ownerOf` has no production caller. This file is uncommitted in the working tree.
- [ ] `internal/component/ike/cmd/show_ipsec.go` - `init()` calls `pluginserver.RegisterRPCs` with three `RPCRegistration` values whose `WireMethod` values are `ze-show:vpn-ipsec-sa`, `ze-show:vpn-ipsec-status`, `ze-show:vpn-ipsec-peer`. Handler signature is `func(*pluginserver.CommandContext, []string) (*plugin.Response, error)`. `saToMap` emits peer name, state, SPIs, algorithms, timestamps, uptime, NAT detection, rekey count, peer window size, and a child-SA sub-object. It emits no byte or packet counter.
- [ ] `internal/component/ike/yang/ze-ipsec-cmd.yang` - the command tree. Two extensions only, `ze:command` and `ze:task-support`. The `sa` container description states "byte counts", and the `peer name` container description states "byte counts". Neither is produced.
- [ ] `internal/component/ike/yang/cmd_schema_test.go` - pins the three `ze:command` strings.
- [ ] `internal/component/ike/engine/metrics.go` - `RegisterMetrics` registers `ze_ipsec_tunnel_up` and `ze_ipsec_tunnel_degraded`. `espInstalled` reads `ChildSA.ESPInstalled` from the peer session map.
- [ ] `internal/component/ike/engine/child.go` - sets `child.ESPInstalled` true after a successful install and false in `warnDegraded`. The value is the return of an install call, not a kernel read.
- [ ] `internal/component/ike/engine/health.go` - `RegisterHealthCheck` registers `checkIPsecHealth`, which reads `ActiveTable()` and reports down, degraded, or healthy from IKE SA state alone.
- [ ] `internal/component/ike/engine/doctor.go` - `checkIPsecInterface` parses the config tree. Registration is a `registry.DoctorCheckDef` in `register.go`.
- [ ] `internal/component/doctor/checks_linux.go` - `checkKernelModules` appends `xfrm_user` and `xfrm_algo` to its required list when the config tree holds a `vpn ipsec` container, then emits `doctor-module-missing` at `SeverityError` for each name absent from `loadedKernelModules()`, which reads `/proc/modules`.
- [ ] `internal/component/ike/engine/testport.go` - `ikeDataplaneName()` returns the `ze.test.ike.dataplane` override or `xfrm`. Functional tests set it to `noop`.
- [ ] `internal/appliance/kernelreq.go` - `runtimeKernelRequirements` lists `CONFIG_MODULES`, `CONFIG_PPP`, `CONFIG_PPPOE`, `CONFIG_L2TP`, `CONFIG_PPPOL2TP`, `CONFIG_L2TP_V3`. `enforceKernelRequirements` unions the profile manifests with this floor and fails the build when a symbol is not `=y`.
- [ ] `gokrazy/kernel/kernel.config` - holds `CONFIG_XFRM_USER=y`. No fragment holds `CONFIG_INET_ESP` or `CONFIG_XFRM_STATISTICS`.
- [ ] `test/interop-ipsec/lab.py` (retired; now `internal/le/interoplab/ipsec/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - `ze_xfrm_state`, `ze_xfrm_policy`, `wait_xfrm_sa`, `check_xfrm_sa_count`, `xfrm_sa_bytes_by_spi`, and `assert_esp_accepted` all run iproute2 through `docker_exec_quiet`. There is no helper that runs a `ze` CLI command.
- [ ] `internal/le/qemu/alltests.go` - 23 `fsuite` lines. None of them is `ipsec`.
- [ ] `gokrazy/ze/config.json` - image packages are `cmd/ze-serial-shell` and `cmd/ze`, plus gokrazy `randomd` and `heartbeat`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The JSON shape of `show vpn ipsec sa`, `status`, and `peer name`. `test/ipsec/*.ci` assert on those keys. New keys are added; no existing key is renamed or removed.
- `ChildSA.ESPInstalled` keeps meaning "the install call succeeded". The drift view compares it against the kernel; it does not redefine it.
- `ze_ipsec_tunnel_up` and `ze_ipsec_tunnel_degraded` keep their current definitions and labels.
- `ListSAs(0)` keeps meaning "every if_id", and a non-zero argument keeps filtering.
- The noop backend stays the unprivileged functional-test dataplane. The merge gate stays unprivileged.

**Behavior to change:** (only what the user asked for)
- `SAInfo` gains fields. `Dataplane` gains `ListPolicies`.
- `vppBackend.ListSAs` and `noopDataplane.ListSAs` stop returning a silent empty list and return `ErrNotSupported`.
- `show vpn ipsec sa` and `show vpn ipsec peer name` gain the byte and packet counters their grammar already advertises.
- `checkKernelModules` stops reporting a missing module for functionality built into the kernel.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator types `show vpn ipsec dataplane sa` at the Ze CLI, over SSH or the web terminal.
- The CLI resolves the command through the YANG tree merged by `cmd.MergeYANGNodes`, and dispatches the wire method `ze-show:vpn-ipsec-dataplane-sa`.

### Transformation Path
1. CLI front end resolves the command node and sends the wire method to the plugin server.
2. `pluginserver` dispatch routes the wire method to the registered handler in `internal/component/ike/cmd/`.
3. The handler calls `dataplane.Get()`. A nil backend is an error response, not an empty list.
4. The handler calls `ListSAs` or `ListPolicies` on the backend.
5. On Linux, `xfrmBackend` issues `netlink.XfrmStateList` or `netlink.XfrmPolicyList` and maps each kernel record into `SAInfo` or `PolicyInfo`.
6. `policyInfoFromKernel` joins the complete kernel selector against `ownerOf` before the handler maps the record.
7. The handler returns `plugin.Map`. It performs no formatting.
8. `command.ApplyPipes` renders a table, or JSON when the operator appends `| json`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ plugin server | Registered `ze-show:vpn-ipsec-dataplane-*` methods and structured responses | Yes: real CLI functional and strongSwan evidence in Goal Validation |
| Command handler ↔ dataplane backend | `dataplane.Get()` and the shared interface | Yes: command source and live kernel reads |
| Dataplane ↔ Linux kernel | Vendored netlink SAD/SPD dumps | Yes: real-kernel readback, mask, family, and removal regressions |
| Engine belief ↔ kernel truth | `ObserveDataplane` guards a peer-map and SAD observation against concurrent publication and writes | Yes: observation units and live health/metric transitions |
| Interop harness ↔ ze container | `scenarioLab.zeCLI` in `internal/le/interoplab/ipsec/helpers.go` | Yes: populated readback red/green walk |

### Integration Points
- `dataplane.Get()` - already the accessor used by `engine/register.go` and `engine/reconcile.go`.
- `pluginserver.RegisterRPCs` in `show_dataplane.go` registers all three readback methods.
- `kernelcap.Register` in `engine/kernelcap_linux.go` enrolls the IPsec capability for doctor, startup, and validation.
- `health.Register("ipsec", checkIPsecHealth)` in `engine/health.go` - the drift signal joins the existing status computation.
- `ownerOf` in `dataplane/policy_owner.go` - the join key for the policy view.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Real CLI dispatch reaches handlers, the shared backend, netlink, and public renderers |
| No unintended coupling | Yes | IKE owns identity and observation. Generic command formatting knows no IPsec fields |
| No duplicated functionality | Yes | Readback reuses `ownerOf`, the netlink dump, the operational response bus, and the existing interop CLI helper |
| Zero-copy preserved where applicable | Yes | `SAIdentity` uses comparable `netip.Addr` values. Kernel address copies own their bytes. Each metrics pass uses one SAD dump |
| Registration over hardcoding | Yes | IKE registers its RPCs and capability. No IKE dispatch case enters a shared package |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A plain SAD dump carries current counters | `xfrmStateFromXfrmUsersaInfo` calls `curToStats` | Per-SA reads would otherwise be required | Live directed counter agreement in `dataplane-readback` | **confirmed**: four counters match iproute2 before and after traffic |
| A-2 | `policy_owner.go` lands on main before this spec is implemented | the file is untracked in the working tree at the time of writing | the policy view drops its Owner column and `ownerOf` stays unwired | grep for `ownerOf` on main before starting Phase 3 | **confirmed** 2026-08-03: the file is tracked, and `policyOwners.ownerOf` exists in `internal/component/ike/dataplane/policy_owner.go`. Phase 3's stop condition does not fire |
| A-3 | `/proc/modules` cannot establish availability of built-in XFRM | `readLoadedModules` reads only loaded modules | A module-name check would misdiagnose the appliance | `checkKernelModules` and `kernelcap.ProbeXFRMSocket` | **confirmed at source**: XFRM now uses the enrolled capability. No fresh appliance doctor run is claimed |
| A-4 | The two privileged inspection fixtures can run under QEMU | QEMU IPsec suite and XFRM backend | Only integration-tier proof would remain | Functional discrimination walk in Goal Validation | **confirmed**: both real-kernel `.ci` files fail on empty readback and pass after restoration |
| A-5 | Policy enumeration includes `if_id 0` | Unfiltered `XfrmPolicyList` | Ordinary site-to-site policies would disappear | `TestXFRMReadbackPolicyNoIfID` on Linux | **confirmed**: supplied QEMU log records PASS, without a skip |
| A-6 | Explicit noop refusal preserves existing functional behavior | Historical unprivileged fixture run | Callers might treat unsupported as empty | Recorded 2026-08-03 IPsec result | **confirmed**: 13/13 in that historical run. No fresh full-suite claim |
| A-7 | Kernel policies reconstruct the ownership key without losing selector identity | Original reasoning considered only Ze's installable port subset | Readback could lose an owner or borrow one from another policy | Wildcard-owner, family, and foreign-mask regressions | **broken and repaired**: normalize implicit prefixes using the writer-selected family, preserve explicit families and raw masks, and compare the complete key |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The kernel dump is unbounded. A device with thousands of SAs renders a table that floods the terminal and allocates per row | the SAD dump handler allocates one map per SA with no ceiling | add a typed selector (`peer name`, `spi`) so the operator can narrow, and state the full-dump cost in the command description |
| R-2 | The dump needs CAP_NET_ADMIN. A CLI user without it gets a permission error that reads like a bug | netlink returns EPERM | the handler reports the permission failure as an error response naming the capability, never as an empty list |
| R-3 | The drift view produces false positives during a rekey window, when two SPIs legitimately coexist | drift reported on every rekey in the interop suite | define drift against the engine's expected SPI set, which includes both SPIs during a rekey, rather than against a single current SPI |
| R-4 | The kernel-level `.ci` becomes the only evidence, and it runs nowhere because QEMU tests are not in CI | the suite passes locally and never runs again | the Go integration tier and the interop scenario carry the goal evidence. The `.ci` proves reachability, and the spec says so in the Goal Validation rows |
| R-5 | Widening `SAInfo` breaks the VPP backend's build or its semantics | compile error, or a VPP dump that fills the new fields with misleading zeros | VPP returns `ErrNotSupported` from both list methods, so it never fabricates a field it cannot source |
| R-6 | Adding `CONFIG_INET_ESP` to the runtime floor fails the appliance kernel build because the fragment does not set it | `enforceKernelRequirements` fails the build | add the symbol to `gokrazy/kernel/runtime.config` in the same commit as the floor entry, and let the existing enforcement prove it |
| R-7 | The interop scenario asserts agreement between two readers that are both empty, which passes vacuously | the SPI set is empty on both sides and the assertion holds | assert a non-empty set first, then assert equality, then assert the set changes across a rekey |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the forwarding path. Every new surface is read-only. The two behavior changes that reach existing code are the `ErrNotSupported` returns from the noop and VPP backends, and the `checkKernelModules` fix. A wrong module fix would silence a real missing-module error, which is the one fail-open outcome here. |
| How is it reverted? | Single commit revert. No config migration, no wire-visible change, no peer state. |
| Who else touches this path? | `plan/spec-ipsec-lifetime-volume.md` (Status: design) needs the SA byte counter this spec sources, and its A-1 is unvalidated for exactly that reason. `plan/spec-ipsec-opaque-selector-port-mask.md` wants a policy read-back to assert installed selectors. the retired deferral shard "ipsec-esp-dual-form-receive" holds an open decision about an ESP-form field on `show vpn ipsec sa`. `policy_owner.go` is uncommitted work in the same package. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show vpn ipsec dataplane sa` typed at the CLI | → | `handleShowVPNIPsecDataplaneSA` | `ipsec-show-dataplane-kernel.ci` and `ipsec-dataplane-show.ci` |
| `show vpn ipsec dataplane policy` typed at the CLI | → | `handleShowVPNIPsecDataplanePolicy` | `ipsec-dataplane-show.ci`, `TestShowDataplanePolicyRendersFields`, `TestShowDataplanePolicyPreservesPortMasks` |
| `show vpn ipsec dataplane drift` typed at the CLI | → | `handleShowVPNIPsecDataplaneDrift` | `ipsec-dataplane-show.ci` and live `checkDataplaneMonitoring` |
| CLI dispatch through the plugin server, end to end | → | The three handlers | `ipsec-dataplane-show.ci` and `dataplane-readback` |
| `ze doctor <config>` with IPsec in use | → | Enrollment in `engine/kernelcap_linux.go`, evaluated by `kernelcap` | `TestXFRMAbsentIsAStartupError`, `TestXFRMUnknownWarnsRatherThanRefusing`, `doctor-ipsec-xfrm.ci` |
| `show vpn ipsec sa \| json` | → | `saToMap` counter fields | `ipsec-show-sa-counters.ci` |
| Appliance kernel build | → | `runtimeKernelRequirements` ESP entries | `TestRuntimeFloorRequiresESP` |
| strongSwan interop run | → | `scenarioLab.zeCLI` reading the SAD dump | scenario `dataplane-readback`, checker `checkDataplaneReadback` |
| Prometheus scrape of a running daemon | → | `IPsecMetrics.Update` and `publishDataplaneGauges` | `TestDataplaneGaugesPublishFromOneDump` and live `checkDataplaneMonitoring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Child SA is installed on Linux and the operator runs `show vpn ipsec dataplane sa \| json` | Output lists that SA with its SPI, source, destination, if_id, mode, reqid, encryption and integrity algorithm names, replay window, current and hard byte and packet counters, and the add and use timestamps |
| AC-2 | A Child SA policy is installed and the operator runs `show vpn ipsec dataplane policy \| json` | Output lists that policy with its selector prefixes and ports, direction, priority, upper-layer protocol, if_id, template tunnel endpoints, and the owning peer name resolved through `ownerOf` |
| AC-3 | The engine believes a Child SA is installed and the kernel SAD lacks its identity | `show vpn ipsec dataplane drift` names the peer, missing SPI, and direction, and exits non-zero. Identity includes SPI, destination, protocol, and XFRM interface ID |
| AC-4 | Engine belief and kernel state agree for every peer | `show vpn ipsec dataplane drift` reports no drift and exits zero |
| AC-5 | Drift exists while the daemon is running | `checkIPsecHealth` reports degraded and names the drifting peer |
| AC-6 | The active dataplane backend is VPP or noop, and the operator runs either dump | The command reports that the backend cannot enumerate the dataplane, and never renders an empty table that reads as "nothing installed" |
| AC-7 | `dataplane.Get()` returns nil, or netlink returns EPERM | The command returns an error response naming the cause. It never returns an empty list |
| AC-8 | An established Child SA has passed traffic, and the operator runs `show vpn ipsec sa \| json` | Each child SA object carries `bytes-in`, `bytes-out`, `packets-in`, and `packets-out`, sourced from the kernel SAD, matching the "byte counts" the YANG description has advertised |
| AC-9 | `ze doctor` probes XFRM with IPsec in use | Kernel absence emits `doctor-ipsec-xfrm-unavailable` at error severity. An inconclusive probe emits `doctor-ipsec-xfrm-unknown` at warning severity. This replaces the original warning-only plan, as recorded under Deviations from Plan |
| AC-10 | `ze doctor` runs on a kernel where XFRM is built in rather than modular, with an ipsec config | No `doctor-module-missing` diagnostic is emitted for `xfrm_user` or `xfrm_algo` |
| AC-11 | The appliance runtime kernel config omits `CONFIG_INET_ESP` | `enforceKernelRequirements` fails the build and names the missing symbol |
| AC-12 | A strongSwan interop scenario establishes a tunnel and passes traffic | Ze's own SAD dump reports the same SPI set that `ip xfrm state` reports in the ze container, and the same set strongSwan reports for the reverse direction |
| AC-13 | A Child SA rekeys during that interop scenario | The SPI set reported by Ze's dump changes to match the kernel's, with the old SPI absent and the new SPI present |
| AC-14 | The IKE engine is running, the kernel SAD is readable, and Prometheus scrapes | `ze_ipsec_dataplane_sa_count{if_id}` (corrected 2026-09-08 from `if-id`, which Prometheus refuses) reports the number of SAs the kernel holds under each if_id, and `ze_ipsec_dataplane_drift{peer}` reports 1 for each peer whose Child SA SPI the kernel does not hold and 0 for each peer it does |
| AC-15 | The dataplane cannot be read: no backend is loaded, the backend is noop or VPP, or netlink returns EPERM | Neither gauge publishes a series, and a series published by an earlier scrape is deleted. A scrape then carries no zero that reads as "no SAs" or "no drift" |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks whether a Child SA reached the kernel | CLI → `ze-show:vpn-ipsec-dataplane-sa` → `dataplane.Get().ListSAs` → `netlink.XfrmStateList` → table | `ipsec-show-dataplane-kernel.ci` |
| 2 | Asks why traffic is not encrypted after the tunnel came up | CLI → drift handler → coherent peer/SAD observation | `TestDriftReportsMissingSPI` and live `checkDataplaneMonitoring` |
| 3 | Asks which peer owns a policy selector | CLI → policy handler → kernel dump joined with `ownerOf` | `TestShowDataplanePolicyRendersFields` and `TestPolicyOwnerJoinRoundTrips` |
| 4 | Asks how much traffic a tunnel has carried | CLI → `ze-show:vpn-ipsec-sa` → `saToMap` counter fields sourced from the SAD | `ipsec-show-sa-counters.ci` |
| 5 | Checks an appliance is fit to run IPsec before configuring it | `ze doctor` → enrolled IPsec capability → `ProbeXFRMSocket` | `doctor-ipsec-xfrm.ci` and `TestXFRMAbsentIsAStartupError` |

## 🧪 TDD Test Plan

This is the original test plan. The closure audit names the implemented tests
and distinguishes supplied executions from source-only evidence.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowDataplaneSARegistered` | `internal/component/ike/cmd/show_dataplane_test.go` | the wire method is present in `pluginserver.AllBuiltinRPCs()` | |
| `TestShowDataplanePolicyRegistered` | `internal/component/ike/cmd/show_dataplane_test.go` | same, for the policy command | |
| `TestShowDataplaneDriftRegistered` | `internal/component/ike/cmd/show_dataplane_test.go` | same, for the drift command | |
| `TestShowDataplaneSARendersFields` | `internal/component/ike/cmd/show_dataplane_test.go` | a fake backend returning two fixed SAs produces the exact JSON keys and values of AC-1 | |
| `TestShowDataplaneBackendErrorSurfaces` | `internal/component/ike/cmd/show_dataplane_test.go` | a backend returning EPERM produces `StatusError` naming the cause, never an empty list (AC-7) | |
| `TestShowDataplaneNilBackendIsError` | `internal/component/ike/cmd/show_dataplane_test.go` | `dataplane.Get()` nil produces `StatusError` (AC-7) | |
| `TestNoopListSAsNotSupported` | `internal/component/ike/dataplane/noop_test.go` | noop returns `ErrNotSupported` from both list methods (AC-6) | |
| `TestVPPListSAsNotSupported` | `internal/component/ike/dataplane/vpp_test.go` | VPP returns `ErrNotSupported` from both list methods (AC-6) | |
| `TestDriftReportsMissingSPI` | `internal/component/ike/cmd/show_dataplane_test.go` | engine believes an SPI the fake SAD lacks, drift names peer, SPI, direction (AC-3) | |
| `TestDriftSilentDuringRekeyWindow` | `internal/component/ike/cmd/show_dataplane_test.go` | two SPIs present during rekey report no drift (R-3) | |
| `TestDriftCleanReportsNothing` | `internal/component/ike/cmd/show_dataplane_test.go` | agreement produces an empty drift list and exit zero (AC-4) | |
| `TestPolicyDumpNamesOwner` | `internal/component/ike/cmd/show_dataplane_test.go` | the policy row carries the owner from `ownerOf` (AC-2) | |
| `TestIPsecHealthDegradedOnDrift` | `internal/component/ike/engine/health_test.go` | `checkIPsecHealth` reports degraded and names the peer (AC-5) | |
| `TestSAToMapCarriesCounters` | `internal/component/ike/cmd/show_ipsec_test.go` | `saToMap` emits the four counter keys (AC-8) | |
| `TestIPsecCapabilityEnrolled` | `internal/component/ike/engine/doctor_xfrm_test.go` | the IPsec kernel capability is enrolled, so one probe answers for `ze doctor`, for the startup refusal and for `ze config validate` | |
| `TestXFRMAbsentIsAStartupError` | `internal/component/ike/engine/doctor_xfrm_test.go` | an absent XFRM dataplane yields `doctor-ipsec-xfrm-unavailable` at error severity, and a kernel that cannot be asked yields `doctor-ipsec-xfrm-unknown` instead (AC-9, with `TestXFRMUnknownWarnsRatherThanRefusing`) | |
| `TestKernelModulesBuiltInNotMissing` | `internal/component/doctor/checks_linux_test.go` | with XFRM built in, no `doctor-module-missing` is emitted for `xfrm_user` or `xfrm_algo` (AC-10) | |
| `TestRuntimeFloorRequiresESP` | `internal/appliance/kernelreq_test.go` | a runtime config without `CONFIG_INET_ESP` fails enforcement and names the symbol (AC-11) | |
| `TestDriftingPeersFromSAsComparesOneDirection` | `internal/component/ike/engine/health_drift_test.go` | the pure comparison over a supplied SA list keeps the one-direction rule, so a rekey window reports nothing | |
| `TestDataplaneGaugesPublishFromOneDump` | `internal/component/ike/engine/metrics_test.go` | one SAD read feeds both gauges: a count for each if_id present, and 1 or 0 for each peer that holds a Child SA (AC-14) | |
| `TestDataplaneGaugesAbsentWhenKernelUnreadable` | `internal/component/ike/engine/metrics_test.go` | a failing SAD read deletes every series both gauges published and sets none (AC-15) | |
| `TestDataplaneGaugesDeleteAPeerThatWentAway` | `internal/component/ike/engine/metrics_test.go` | a peer that leaves `PeerInfoMap` loses its drift series, so a scrape never reports a value for a peer that no longer exists | |
| `TestDataplaneReadbackRefusesAnEmptySPISet` | `internal/le/interoplab/ipsec/ipsec_test.go` | the interop checker fails closed when either reader returns no SPI, before it compares them (R-7, AC-12) | |
| `TestDataplaneReadbackRequiresTheSPISetToChange` | `internal/le/interoplab/ipsec/ipsec_test.go` | the checker refuses a rekey after which the set is unchanged, so AC-13 asserts a transition rather than a state | |
| `TestDataplaneSPIsDecodeAsIntegers` | `internal/le/interoplab/ipsec/ipsec_test.go` | the `spi` member of the Ze dump and the `0x` form iproute2 prints normalize to the same `uint32`, and an answer that does not decode is an error naming it, never an empty set | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SPI selector on the dataplane SA lookup | 1 to 4294967295 | 4294967295 | 0 (reserved by RFC 4303 Section 2.1) | 4294967296 |
| if_id filter argument | 0 to 4294967295 | 4294967295 | N/A (0 means every if_id) | 4294967296 |
| Byte counter rendering | 0 to 18446744073709551615 | 18446744073709551615 | N/A | N/A (uint64, must not be narrowed to uint32) |
| Policy priority | 0 to 4294967295 | 4294967295 | N/A | 4294967296 |
| `ze_ipsec_dataplane_sa_count` value | 0 to the SAD size | the count the dump returned | N/A. An unreadable SAD deletes the series and never publishes 0 (AC-15) | N/A |
| `bytes-out` on `show vpn ipsec sa` over a real kernel | 0 to 18446744073709551615 | 0, which is a valid answer for an SA that carried nothing | null, which says the SAD was never read and pairs with `counters-known` false | N/A |
| SPI set size in the interop readback | 1 upward | the set the tunnel installed | 0. The checker refuses an empty set before it compares, because two empty sets are equal (R-7) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-dataplane-show` | `test/ipsec/ipsec-dataplane-show.ci` | the operator reaches all three new commands end to end under the noop backend, and each reports that the backend cannot enumerate rather than showing an empty table. Reachability evidence only, not kernel evidence | |
| `ipsec-show-dataplane-kernel` | `test/ipsec/ipsec-show-dataplane-kernel.ci` | the responder daemon runs the real xfrm backend and the initiator runs noop, as `ipsec-teardown-leaves-nothing.ci` does. After the Child SA establishes, `show vpn ipsec dataplane sa` names the addresses and the ESP algorithm this test configured. After `clear vpn ipsec sa` the same command no longer names them, which is the transition, not a state | Passed in the QEMU guest, 2026-09-11. RED and GREEN below |
| `ipsec-show-sa-counters` | `test/ipsec/ipsec-show-sa-counters.ci` | on the same real-kernel fixture, `show vpn ipsec sa \| json` reports `counters-known` true and a numeric `bytes-out`. Zero is the expected value with no traffic through the tunnel, and null is the failure it discriminates: null says the SAD was never read | Passed in the QEMU guest, 2026-09-11. RED and GREEN below |
| `doctor-ipsec-xfrm` | `test/ui/doctor-ipsec-xfrm.ci` | `ze doctor --json` on an ipsec config emits the reachability diagnostic when the probe seam fails | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dataplane-readback` | `test/interop-ipsec/scenarios/dataplane-readback/` | strongSwan | Non-empty SAD agreement, reverse peer SPIs, all four directed counters, complete old-SPI removal across rekey, and clean/drift/unknown/restored/removed monitoring states | Passed: Goal Validation records the red/green evidence |

## Files to Modify
- `internal/component/ike/dataplane/dataplane.go` - widen `SAInfo`, add `PolicyInfo`, add `ListPolicies` to the `Dataplane` interface
- `internal/component/ike/dataplane/xfrm_linux.go` - fill the widened `SAInfo` from `netlink.XfrmState`, implement `ListPolicies` through `netlink.XfrmPolicyList`
- `internal/component/ike/dataplane/xfrm_other.go` - `ListPolicies` stub returning `ErrNotSupported`
- `internal/component/ike/dataplane/vpp.go` - both list methods return `ErrNotSupported`
- `internal/component/ike/dataplane/noop.go` - both list methods return `ErrNotSupported`
- `internal/component/ike/dataplane/policy_owner.go` - expose the owner lookup for the policy view.
  → Decision: reconstruct `SPParams` from the kernel policy and use the existing
  `ownerOf` lookup. The key preserves address families and both raw port masks.
  The original mask-free inversion was wrong for foreign policies and could
  misattribute ownership. A-7 records the corrected normalization.
  → Constraint (`ai/rules/evidence.md`, fail closed): a kernel policy Ze did not install has no
  owner, which is a legitimate and common answer (strongSwan's own policies, the OSPF proto-89
  policies, an operator's). That row renders the owner as `unknown`. It is never blank, and it is
  never guessed from a partial match.
- `internal/component/ike/cmd/show_ipsec.go` - register the three new wire methods, add the counter fields to `saToMap`
- `internal/component/ike/yang/ze-ipsec-cmd.yang` - the `dataplane` container and its three command nodes
- `internal/component/ike/yang/cmd_schema_test.go` - extend the pinned `ze:command` list
- `internal/component/ike/engine/register.go` - add the `registry.DoctorCheckDef` entry
- `internal/component/ike/engine/health.go` - fold drift into `checkIPsecHealth`
- `internal/component/doctor/checks_linux.go` - stop reporting a missing module for built-in XFRM
- `internal/core/diagnostic/codes.go` - add the new diagnostic codes
- `internal/appliance/kernelreq.go` - add the ESP symbols to `runtimeKernelRequirements`
- `gokrazy/kernel/runtime.config` - set the ESP and XFRM statistics symbols
- `gokrazy/kernel/runtime.require` - list the bare symbols
- `internal/le/qemu/alltests.go` - the `ipsec` suite row and the `./internal/component/ike/dataplane` integration package. Both are in HEAD (`vmSuites`, `integrationPackages`), so this phase runs the suite rather than wiring it
- `internal/component/ike/engine/health_drift.go` - split the comparison into `driftingPeersFrom(sas []dataplane.SAInfo)`, a pure function over a supplied SA list, and keep `driftingPeers` as the wrapper that reads the SAD. The gauge publisher and the health check then share ONE dump for each pass
- `internal/component/ike/engine/metrics.go` - the two dataplane gauges, published by `Update`, which already runs on a 5 second ticker (`register.go`). One `ListSAs(0)` for each pass, taken only when `ActiveTable()` is non-nil, so a daemon that runs no IKE issues no netlink dump
- `internal/le/interoplab/ipsec/checkers.go` - `checkDataplaneReadback` and its row in `scenarioCheckers`
- `internal/le/interoplab/ipsec/helpers.go` - the SPI readers the checker joins: the Ze dump decoded into `uint32`, and the `0x` form `espSPIPattern` already captures, normalized to the same type
- `docs/guide/ipsec.md` - the two gauges in the Prometheus paragraph
- `docs/guide/command-reference.md` - the three commands, which the page does not carry today
- `docs/features.md` - the IPsec CLI and Diagnostics row, which lists the IPsec gauges
- `docs/architecture/ike/ipsec-10-cli-diag.md` - the metrics section, which is where the IPsec gauges are declared
- `docs/architecture/ike/ipsec-dataplane-inspection.md` - the decision that a gauge with no readable kernel publishes no series
- `docs/functional-tests.md` - the two new `.ci` files and the interop scenario
- `docs/architecture/testing/interop.md` - the `dataplane-readback` row

## Files to Create
- `internal/component/ike/cmd/show_dataplane.go` - the three handlers
- `internal/component/ike/cmd/show_dataplane_test.go` - handler unit tests
- `internal/component/ike/engine/doctor_xfrm_test.go` - check unit tests.
  → Decision (landed, verified 2026-09-08): the three `doctor_xfrm*.go` files were NOT written. The
  probe is the enrolled kernel capability (`internal/component/kernelcap`), and `doctor.go` reports
  `doctor-ipsec-xfrm-unavailable` and `doctor-ipsec-xfrm-unknown` from it, so `ze doctor`, the
  daemon's startup refusal and `ze config validate` cannot disagree. One probe, three readers
- `internal/component/ike/dataplane/xfrm_readback_integration_linux_test.go` - install, read back, remove, read back again
- `test/ipsec/ipsec-dataplane-show.ci` - reachability under the noop backend
- `test/ipsec/ipsec-show-dataplane-kernel.ci` - real kernel readback
- `test/ipsec/ipsec-show-sa-counters.ci` - the counters AC-8 restores
- `test/ui/doctor-ipsec-xfrm.ci` - the doctor diagnostic
- `test/interop-ipsec/scenarios/dataplane-readback/` - `ze.conf`, `strongswan.conf`, `check.py`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `in-progress` and the work is not implemented) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/yang/ze-ipsec-cmd.yang`, a `dataplane` container with three `ze:command` nodes. Operational commands, not config, so `config false` throughout |
| YANG validation constraints | Yes | the optional `spi` selector leaf takes a `range` constraint, and the peer selector reuses the string type of the existing `peer name` leaf |
| YANG custom validators | N-A | no config leaf is added, so there is nothing to validate at commit time |
| CLI commands/flags | Yes | handlers in `internal/component/ike/cmd/show_dataplane.go`, registered through `pluginserver.RegisterRPCs` in `show_ipsec.go` |
| CLI grammar (keyword before value) | Yes | `show vpn ipsec dataplane sa`, `... policy`, `... drift`. Any per-object lookup is a typed selector container, never a positional |
| Editor autocomplete | Yes | automatic. `internal/plugins/completion/words.go` builds the verb tree from the YANG nodes, so a fixed keyword needs nothing else |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-dataplane-show.ci` and `test/ipsec/ipsec-show-dataplane-kernel.ci` |
| Pipe completeness | Yes | handlers return `plugin.Map` and never format. `command.ApplyPipes` renders table and JSON. Column order follows the JSON key names |
| Env var registration | N-A | no new env var. The existing `ze.test.ike.dataplane` override is reused unchanged |
| Doctor check for runtime dependencies | Yes, done | `internal/component/ike/engine/doctor.go` reports `doctor-ipsec-xfrm-unavailable` and `doctor-ipsec-xfrm-unknown` from the enrolled capability in `internal/component/kernelcap`. Codes in `internal/core/diagnostic/codes.go`, unit tests in `doctor_xfrm_test.go`, functional evidence in `test/ui/doctor-ipsec-xfrm.ci` |
| Prometheus counters/metrics | Yes, DONE 2026-09-08 | `ze_ipsec_dataplane_sa_count` (GaugeVec, label `if_id`, underscored because Prometheus refuses a hyphen in a label name) and `ze_ipsec_dataplane_drift` (GaugeVec, label `peer`), registered in `internal/component/ike/engine/metrics.go` beside the existing tunnel gauges. Both are a GaugeVec because a `Gauge` cannot be deleted and AC-15 needs the series to go away. `IPsecMetrics` holds the label sets it published, so a peer or an if_id that stops appearing loses its series on the next pass |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, the IPsec row |
| 2 | Config syntax changed? | N-A | no config leaf is added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` documents all three commands, qualified drift identity, and SPI filtering |
| 4 | API/RPC added/changed? | N-A | `docs/architecture/api/commands.md` documents the operational report bus, not an inventory of wire methods, and it names no `ze-show:` method outside that bus. The three methods are described where the commands are, in `docs/guide/ipsec.md` |
| 5 | Plugin added/changed? | N-A | the ike component is not a plugin in the `internal/plugins/` sense, and no plugin inventory changes |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, the command table plus a new troubleshooting section |
| 7 | Wire format changed? | N-A | nothing on the wire changes. Every new surface is read-only and local |
| 8 | Plugin SDK/protocol changed? | N-A | the `Dataplane` interface is internal to the ike component and is not part of `pkg/plugin/` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC requirement changes polarity. The interop scenario proves an existing claim more strongly, which adds no row |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` names the fixtures and exclusive group. `docs/architecture/testing/interop.md` documents directed counters and monitoring proof. `ci-format.md` documents foreground lifetime |
| 11 | Affects daemon comparison? | N-A | `docs/comparison.md` has no IPsec or strongSwan comparison. The original checklist premise was wrong |
| 12 | Internal architecture changed? | Yes | The subsystem pages `docs/architecture/ike/ipsec-dataplane-inspection.md` and `ipsec-8-ikev2-child-xfrm.md` own the readback contract and canonical netlink patch |
| 13 | Route metadata keys added/changed? | N-A | no route metadata is involved |
| 14 | Prometheus counters added/changed? | Yes | The two dataplane gauges and unknown-state behavior appear in `ipsec-10-cli-diag.md`, the inspection architecture page, `docs/guide/ipsec.md`, and `docs/features.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | `docs/guide/status.md` carries no `vpn ipsec` command inventory to extend. `docs/guide/command-reference.md` is the inventory, and row 3 owns it |
| 16 | Changed source referenced by existing documentation? | Yes | Closure repaired stale family, mask, owner, VPP type, sort, counter, and drift claims and their source anchors |
| 17 | Existing CLI/API examples? | Yes | Operator pages describe the live kernel counters and distinguish absent identities from failed observations. No fabricated sample counter output was added |

## Implementation Steps

These phases preserve the original implementation plan. The closure audit is
the current state and identifies the tests and files that replaced planned names.

1. **Phase: Wiring (MANDATORY FIRST)** - register the three commands as stubs and prove they are reachable
   - Tests: `TestShowDataplaneSARegistered`, `TestShowDataplanePolicyRegistered`, `TestShowDataplaneDriftRegistered`, `ipsec-dataplane-show.ci`
   - Files: `internal/component/ike/yang/ze-ipsec-cmd.yang`, `internal/component/ike/yang/cmd_schema_test.go`, `internal/component/ike/cmd/show_dataplane.go`, `internal/component/ike/cmd/show_ipsec.go`
   - Verify: `./le docvalid command-contract` passes (it fails a `ze:command` with no handler and a handler with no `ze:command`), and the wiring tests fail on the stub bodies
2. **Phase: Dataplane read contract** - widen the interface and close the fail-open backends
   - Tests: `TestNoopListSAsNotSupported`, `TestVPPListSAsNotSupported`, `TestXFRMReadbackShowsInstalledSA`, `TestXFRMReadbackIfIDFilter`, `TestXFRMReadbackPolicyNoIfID`
   - Files: `dataplane.go`, `xfrm_linux.go`, `xfrm_other.go`, `vpp.go`, `noop.go`, `xfrm_readback_integration_linux_test.go`
   - Verify: the integration test installs a known SPI, reads it back, removes it, and reads back its absence
3. **Phase: SAD and SPD views** - fill the two dump handlers
   - Tests: `TestShowDataplaneSARendersFields`, `TestPolicyDumpNamesOwner`, `TestShowDataplaneBackendErrorSurfaces`, `TestShowDataplaneNilBackendIsError`
   - Files: `show_dataplane.go`, `policy_owner.go`
   - Verify: A-2 first. If `ownerOf` is not on main, stop and raise it rather than duplicating the registry
4. **Phase: Drift and health** - compare belief against the kernel
   - Tests: `TestDriftReportsMissingSPI`, `TestDriftCleanReportsNothing`, `TestDriftSilentDuringRekeyWindow`, `TestIPsecHealthDegradedOnDrift`
   - Files: `show_dataplane.go`, `internal/component/ike/engine/health.go`, `internal/component/ike/engine/metrics.go`
   - Verify: the rekey-window test is written before the drift logic, so the two-SPI case cannot be discovered late
5. **Phase: Byte counters** - make the advertised claim true
   - Tests: `TestSAToMapCarriesCounters`, `ipsec-show-sa-counters.ci`
   - Files: `internal/component/ike/cmd/show_ipsec.go`, `docs/guide/ipsec.md`
   - Verify: A-1 first. Confirm the dump populates `Statistics` without a per-SA round trip
6. **Phase: Doctor and kernel floor** - readiness before configuration
   - Tests: `TestXFRMReachableDoctorCheckRegistered`, `TestXFRMUnavailableDiagnostic`, `TestKernelModulesBuiltInNotMissing`, `TestRuntimeFloorRequiresESP`, `doctor-ipsec-xfrm.ci`
   - Files: `doctor_xfrm.go`, `doctor_xfrm_linux.go`, `doctor_xfrm_other.go`, `engine/register.go`, `internal/component/doctor/checks_linux.go`, `internal/core/diagnostic/codes.go`, `internal/appliance/kernelreq.go`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`
   - Verify: the kernel floor change and the config fragment change land together, or the appliance build fails
   - Landed differently: the probe is enrolled by `engine/kernelcap_linux.go`, not by the planned doctor-local files. `kernelcap` supplies the shared doctor, startup, and validation behavior.
7. **Phase: the two dataplane gauges** - the kernel-aware signal beside the belief gauges
   - Tests: `TestDriftingPeersFromSAsComparesOneDirection`, `TestDataplaneGaugesPublishFromOneDump`, `TestDataplaneGaugesAbsentWhenKernelUnreadable`, `TestDataplaneGaugesDeleteAPeerThatWentAway`
   - Files: `internal/component/ike/engine/health_drift.go`, `internal/component/ike/engine/metrics.go`
   - Shape: `driftingPeers` splits into `driftingPeersFrom(sas []dataplane.SAInfo) []string`, which compares, and the reader that supplies the list. `Update` takes ONE `ListSAs(0)`, publishes the per-if_id count from it, and passes the same slice to the comparison. Two dumps for one pass would be two answers to one question
   - Constraint (`ai/rules/evidence.md`): an unreadable SAD is not a zero. `Update` deletes every series it published and sets none, because a `ze_ipsec_dataplane_drift` of 0 on an unreadable kernel is the false green this spec exists to remove
   - Constraint: the gauges are published only when `ActiveTable()` is non-nil. A daemon with no IKE engine has no belief to contradict, and it MUST NOT issue a netlink dump every 5 seconds to learn that
   - Verify: `TestDataplaneGaugesAbsentWhenKernelUnreadable` fails if the publisher sets 0 instead of deleting
8. **Phase: kernel and interop evidence** - prove it against a real kernel and a real peer
   - Tests: `ipsec-show-dataplane-kernel.ci`, `ipsec-show-sa-counters.ci`, scenario `dataplane-readback`
   - Files: the two `.ci` files, `internal/le/interoplab/ipsec/{checkers,helpers,ipsec_test}.go`, and `test/interop-ipsec/scenarios/dataplane-readback/`
   - Shape of the two `.ci` files: copy the fixture of `test/ipsec/ipsec-teardown-leaves-nothing.ci`. ONE daemon runs the real xfrm backend and the other runs noop through `env ze_test_ike_dataplane=noop` on its own `cmd=` line, because two real backends in one kernel install the same policy twice and the second install fails. Both files carry `option=needs-linux:caps=net-admin` and `option=exclusive:group=ipsec-xfrm`: `ip xfrm state` is node-wide, so a sibling real-XFRM test running beside them puts its own SPIs in the dump
   - Shape of the assertions: engine steps, which are the only per-command scope a `.ci` has. `expect=output:contains=` and `expect=output:json=` re-dispatch the command until the predicate holds, and `expect=output:absent=` is how the transition after `clear vpn ipsec sa` is asserted. A file-level `expect=stdout:contains=` would be satisfied by any command's output and could not express before-and-after
   - Shape of the scenario: `ze.conf` and `swanctl.conf` beside the other 30 scenario directories, plus `checkDataplaneReadback` in `scenarioCheckers`. It reuses `zeCLI`, `espSPIs` and `verifyTunnelTraffic`, which the lab already carries, and adds only the Ze dump reader and the SPI normalization
   - Constraint: iproute2 prints an SPI as `0x...` and the Ze dump answers a JSON number. The join normalizes both to `uint32`, and the Ze answer decodes into a typed struct rather than `map[string]any`, so no SPI passes through `float64`
   - Verify: run the discrimination walk `ai/rules/interop-and-goal-validation.md` requires. Make `xfrmBackend.ListSAs` return an empty slice, rebuild the ze image, and confirm `dataplane-readback` goes RED and `ipsec-show-dataplane-kernel.ci` goes RED. Restore, confirm GREEN, and record the RED output in the spec. Assert the set is non-empty before asserting it is equal
9. **Phase: Documentation** - every row of the Documentation checklist marked Yes
   - Files: the Documentation Update Checklist records the completed updates and justified N-A rows.
   - Verify: `./le doc check verify` and `./le repository check`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Feature completeness | All five user stories have a working path from the CLI to the kernel and back |
| Correctness | The drift comparison uses the engine's expected SPI set, so a rekey window reports nothing |
| Correctness | Every list method that cannot enumerate returns `ErrNotSupported`, and no path renders an empty table for an unsupported backend |
| Naming | JSON keys are kebab-case and match the existing `show vpn ipsec sa` style (`if-id`, not `ifid`) |
| Data flow | Handlers format nothing. Table and JSON both come from `command.ApplyPipes` |
| Data flow | The doctor check derives its expectation from the config tree, never from `ActiveTable()`, which is nil in the doctor process |
| Rule: `ai/rules/evidence.md` | A nil backend, an EPERM, and an unsupported backend are three distinct errors, and none of them is an empty list |
| Rule: `ai/rules/plugins.md` | Nothing is added to `internal/component/doctor` except the built-in module fix, which is that package's own defect |
| Rule: `ai/rules/completion.md` | `ListSAs`, `ListPolicies`, and `ownerOf` each have a non-test caller when the spec closes |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each kernel assertion names an object the test installed, and asserts a transition |

### Deliverables Checklist
| Deliverable | Verification result |
|-------------|---------------------|
| Three commands registered and reachable | Functional and interop command paths ran. The old command-contract gate was not rerun for closure |
| Production list callers | `show_dataplane.go` calls both list methods. `ObserveDataplane` supplies counter, drift, health, and metric consumers |
| Every backend implements the read interface | Source confirms XFRM enumeration and explicit noop/VPP/other-platform refusal. No fresh all-tags build claimed |
| Kernel counters | QEMU counter-source fixture and strongSwan directed-counter agreement passed |
| Doctor capability | Source confirms absent versus unknown handling. Existing Linux boundary tests were inspected. No fresh appliance doctor invocation or Linux doctor test run is claimed |
| Kernel floor | `enforceKernelRequirements`, `runtimeKernelRequirements`, and the fragments agree. Existing boundary test inspected. No fresh kernel build |
| QEMU suite | Both selected inspection `.ci` files passed inside QEMU. This is not a claim that `qemu all-tests` ran |
| Interop scenario | Populated readback failed with empty SAD output and passed after restoration |
| Gauge refusal and cleanup | Live exporter proof covered clean, drift, unknown, restored, and peer-removed states |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The optional SPI selector is a bounded integer. Zero is reserved by RFC 4303 Section 2.1 and is rejected rather than treated as "every SPI" |
| Information disclosure | The SAD dump must never render key material. `netlink.XfrmState` carries `Auth`, `Crypt`, and `Aead`, each holding a key. Render the algorithm name and the key length only, never the key bytes |
| Authorization | The dump requires CAP_NET_ADMIN. A failure is reported as a permission error, never degraded into an empty list that reads as "nothing installed" |
| Resource exhaustion | An unbounded SAD on a large device produces one map per SA. The selector narrows it, and the handler must not hold the whole dump twice |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `ownerOf` absent from main at Phase 3 | Stop. A-2 is broken. Raise it rather than duplicating the ownership registry |
| `Statistics` empty on a dump at Phase 5 | A-1 is broken. Add the per-SA `XfrmStateGet` round trip and revise the interop assertions |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Implementation State

The Implementation Audit below supersedes the 2026-09-08 state table.
The two functional fixtures and the strongSwan scenario have now run.

## Phase 7 and 8 evidence (2026-09-08)

### Phase 7, the two gauges: RED then GREEN

`driftingPeersFrom` red, before the split existed:

```
internal/component/ike/engine/health_drift_test.go:162:11: undefined: driftingPeersFrom
FAIL github.com/ze-software/ze/internal/component/ike/engine [build failed]
```

The three gauge tests red, with the split in place and no publisher:

```
--- FAIL: TestDataplaneGaugesPublishFromOneDump (0.00s)
    metrics_test.go:210: driftSAD called 0 times in one pass, want 1
    metrics_test.go:221: ze_ipsec_dataplane_sa_count{0} absent, want 2
    metrics_test.go:221: ze_ipsec_dataplane_sa_count{7} absent, want 1
    metrics_test.go:221: ze_ipsec_dataplane_drift{peer-a} absent, want 0
    metrics_test.go:221: ze_ipsec_dataplane_drift{peer-b} absent, want 1
--- FAIL: TestDataplaneGaugesAbsentWhenKernelUnreadable (0.00s)
    metrics_test.go:243: the readable pass published no drift series, so the deletion proves nothing
--- FAIL: TestDataplaneGaugesDeleteAPeerThatWentAway (0.00s)
    metrics_test.go:267: the first pass published no drift series for peer-a
```

DISCRIMINATION. With `setGaugeSeries` publishing 0 in place of deleting, which is
the one design this phase exists to refuse:

```
--- FAIL: TestDataplaneGaugesAbsentWhenKernelUnreadable (0.00s)
    metrics_test.go:263: ze_ipsec_dataplane_sa_count{0} still published after the SAD read failed
    metrics_test.go:263: ze_ipsec_dataplane_drift{peer-a} still published after the SAD read failed
--- FAIL: TestDataplaneGaugesDeleteAPeerThatWentAway (0.00s)
    metrics_test.go:286: ze_ipsec_dataplane_drift{peer-a} survived the peer leaving PeerInfoMap
```

Green, whole package:

```
ok  github.com/ze-software/ze/internal/component/ike/engine 30.558s
```

→ Correction (2026-09-08): the label is `if_id`, NOT the `if-id` this spec's
AC-14 and Integration Checklist wrote. A Prometheus label name matches
`[a-zA-Z_][a-zA-Z0-9_]*`, so `MustRegister` refuses a hyphen and the daemon dies
at startup rather than at a scrape. Every other label in the repository is
underscored. `TestIPsecMetricsRegisterOnPrometheus` registers the whole IPsec set
on a real `PrometheusRegistry`, so the refusal cannot come back unnoticed.

### Phase 8, the interop join: RED then GREEN

```
internal/le/interoplab/ipsec/ipsec_test.go:1033:11: undefined: requireSameSPISet
internal/le/interoplab/ipsec/ipsec_test.go:1056:12: undefined: requireSPISetChanged
internal/le/interoplab/ipsec/ipsec_test.go:1086:16: undefined: decodeZeDataplaneSPIs
internal/le/interoplab/ipsec/ipsec_test.go:1091:20: undefined: spiValues
FAIL github.com/ze-software/ze/internal/le/interoplab/ipsec [build failed]
```

The comparison then caught its own fixture, which is the discrimination that
matters here: the first version of `TestDataplaneSPIsDecodeAsIntegers` carried
decimal constants that were not the hexadecimal ones beside them, and the join
refused them rather than agreeing:

```
--- FAIL: TestDataplaneSPIsDecodeAsIntegers (0.00s)
    ipsec_test.go:1097: the two readers disagree after normalization: ze dump and ip xfrm
    state disagree on the kernel SAD: ze dump reports [218893584 3248693700],
    ip xfrm state reports [219025168 3248665540]
```

Green:

```
ok  github.com/ze-software/ze/internal/le/interoplab/ipsec 4.021s
ok  github.com/ze-software/ze/test/interop-ipsec 0.486s
```

The `.ci` half on a darwin host: `./le functional ipsec` is 22/22 with 7 SKIP,
and both new files are among the skips, because a darwin host has no
CAP_NET_ADMIN and no XFRM. That proves they PARSE and are selected. The kernel
evidence is the section below.

## Kernel and strongSwan discrimination walk (2026-09-11)

One break serves all three artifacts. `xfrmBackend.ListSAs`
(`internal/component/ike/dataplane/xfrm_linux.go`) returns an empty SAD in place
of the netlink dump, which is the vacuity a read-only dump invites. The two
`.ci` files run against a cross-built `ze` carrying it, and the interop scenario
rebuilds its container image from the same source, so the break reaches the
daemon each artifact actually drives. The source was restored between the RED
and the GREEN of each.

Evidence paths below are relative to
`tmp/session/2026-09-08-0a21e591-d035-4f7c-8d1b-231ab023a478/scratch/`.

### The two `.ci` files, in the QEMU Linux guest

Driver: `./le qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap"`,
guest command `ze-test ipsec -v -p 1 ipsec-show-dataplane-kernel ipsec-show-sa-counters`.
Log: `job-ipsec-dpi-qemu-walk-5d0a1d41.log`.

RED, with the empty SAD:

```
=================== RED: ListSAs returns an empty SAD ===================
47.3s    1/2  FAIL  24  ipsec-show-dataplane-kernel
19.7s    2/2  FAIL  25  ipsec-show-sa-counters
fail  0/2  0.0%  67.0s  failed 2 [24, 25]
EXPECTED_RED:1
```

The two failures name the surface each file exists to prove:

```
TEST FAILURE: 24 ipsec-show-dataplane-kernel
engine step 4: output missing "cbc(aes)" within 45s
  (last "show vpn ipsec dataplane sa" output "done {\"sas\":[]}")

TEST FAILURE: 25 ipsec-show-sa-counters
engine step 5: json path "peers.0.child-sa.bytes-out" never equaled "0" within 15s
  (last "show vpn ipsec sa" data "{"peers":[{"behind-nat":false,"child-sa":
   {"bytes-in":null,"bytes-out":null,"counters-known":true,...")
```

The counters failure is the one the file's own comment predicted: `counters-known`
stays true because the SAD READ succeeded, and the four counters read null
because the dump named nothing. Null, not zero, is what the fixture
discriminates.

GREEN, source restored:

```
=================== GREEN: the SAD read restored ===================
4.6s     1/2  PASS  24  ipsec-show-dataplane-kernel
2.8s     2/2  PASS  25  ipsec-show-sa-counters
pass  2/2  100.0%  7.4s
GREEN_EXIT:0
QEMU VM: PASS
```

### The `dataplane-readback` scenario, against strongSwan

Driver: `IPSEC_INTEROP_SCENARIO=dataplane-readback ./le integration interop-ipsec`.

GREEN before the break (`job-ipsec-dpi-interop-green-32b5021c.log`):

```
integration: 1 action(s) passed.
```

RED with the break (`job-ipsec-dpi-interop-red-32b5021c.log`):

```
── dataplane-readback ──
  ✗ FAIL: show vpn ipsec dataplane sa reports no ESP SPI, so an agreement with ip xfrm state in the ze container would be vacuous

FAIL  0 passed, 1 failed: dataplane-readback
Failed: interop-ipsec
```

That message is `requireSameSPISet`'s emptiness guard firing BEFORE the
comparison, which is the ordering R-7 asks for: two empty sets are equal, so an
agreement checked first would have held over a kernel holding nothing.

GREEN after the source was restored
(`job-ipsec-dpi-interop-restored-32b5021c.log`):

```
integration: 1 action(s) passed.
```

### The SPI base join was exercised

The design phase warned that the two readers print an SPI in different bases.
The GREEN scenario run exercised the join: `requireSameSPISet` compared a
non-empty Ze set against a non-empty iproute2 set and found them EQUAL, which is
reachable only if both sides normalized. `decodeZeDataplaneSAs` unmarshals into
`zeDataplaneSA{SPI uint32}`, so the Ze side is a typed decode with no
`map[string]any` float64 in the path, and `spiValues` parses iproute2's
`0xc1a2b3c4` with `strconv.ParseUint(..., 16, 32)`. The RED run proves the same
join refuses rather than agreeing when one side is empty.

## Historical Phase 8 Blocker (2026-09-08)

Unrelated BGP and plugin compile failures prevented the initial QEMU and interop
runs. Those are not current blockers. Goal Validation records the later executions.

### Documentation Checklist Resolution

| # | Answer |
|---|--------|
| 3 | DONE. `docs/guide/command-reference.md` gained a `show vpn ipsec dataplane` section carrying the three commands, the `spi` selector's RFC 4303 refusal, and the "cannot enumerate" answer |
| 10 | DONE. `docs/functional-tests.md` names both new `.ci` files and adds them to the `ipsec-xfrm` exclusive group paragraph. `docs/architecture/testing/interop.md` gained the typed-decode and non-empty-before-equal rules the readback scenario turns on |
| 11 | N-A, and the checklist's premise was wrong. `docs/comparison.md` compares BGP and routing daemons (BIRD, FRR, GoBGP, OpenBGPd, ExaBGP). It names strongSwan nowhere and IPsec nowhere, so there is no diagnostic-surface row for an IPsec read-back to join. Verified 2026-09-08 by grep: zero occurrences of `ipsec` or `strongswan` in the page |
| 14 | DONE in the working tree. The gauge documentation is present in the architecture, guide, and feature pages. Shared-page publication must preserve unrelated owner edits |

One piece of test INFRASTRUCTURE was added because the deliverable needed it:
`expect=command-error:contains=` (`parseEngineExpectCommandError` and
`RunEngineSteps`, `internal/test/runner/engine_steps.go`). The plugin SDK turns a
`StatusError` response into a Go error, so before this no `.ci` could assert an
operational error at all: the command step aborted the run before any `expect=`
ran. That made every refusal path untestable end to end, which is exactly the
class AC-6 and AC-7 live in.

## Design Insights

- Belief and truth are two different databases, and Ze only ever had one of them. Every symptom in this spec is a consequence of that single fact: the false green metric, the vacuous test, the operator with no answer, the interop suite that had to reach for iproute2.
- The read path was already written and already unused. `ListSAs` shipped with the XFRM backend and never acquired a caller. An exported method with no caller is not dormant capability, it is an untested claim, and the vacuous `TestXFRMListSAs` (it passes on an empty list) is what an untested claim looks like from the inside.
- A dump command that renders an empty table when the backend cannot enumerate is worse than one that errors, because the empty table answers the operator's question with a confident lie.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Name the surface `dataplane`, not `xfrm` | `show vpn ipsec xfrm state` mirroring `ip xfrm` | `Dataplane` is the abstraction, and VPP is a second backend. Naming the command after the Linux backend would make the VPP case read as a category error |
| Split SAD and SPD into two commands | One merged view | RFC 4301 Section 4.4 keeps them separate, and a policy with no matching state is the exact failure the surface exists to show. Merging hides it |
| Drift is a live CLI view and a health signal, not a doctor check | A `ze doctor` reconciliation check | `ze doctor` runs offline against a config file, in a process where the engine never ran. `ActiveTable()` is nil there, which is why `checkIPsecUDPEncap` already says nothing in that context |
| Unsupported backends return `ErrNotSupported` | Keep returning an empty list | An empty list is indistinguishable from "no SAs installed". `ai/rules/evidence.md` bans a zero value that looks like a valid answer |
| The doctor check probes netlink instead of adding another module row | Add `esp4`/`esp6` to `checkKernelModules` | On an appliance kernel XFRM is built in, so `/proc/modules` lists nothing and the module check produces a false error. A netlink probe tests the capability rather than the packaging |
| Byte counters come from the kernel SAD | Count in the engine as packets are processed | The engine never sees ESP payload; the kernel does. Counting in userspace would report a number that is always zero |
| Drift found returns `StatusError`, and the message names every drifting peer, SPI and direction (2026-08-03) | Return `StatusDone` with drift rows and a count, so the view stays pipeable | AC-3 requires a non-zero exit, and `StatusError` is how this codebase spells one (`internal/component/bfd/cmd/bfd.go` is the precedent). It is not free: the dispatcher's external-plugin path rebuilds an error response as `Error: string(rpcOut.Data)` and DISCARDS `Data`, so a drift table cannot ride along with the non-zero status. The message therefore carries the facts AC-3 names. The clean case (AC-4) returns `StatusDone` with an empty `drift` list and pipes normally, and the two data-bearing commands (`sa`, `policy`) are unaffected |
| Render algorithm names and key lengths, never key bytes | Render the full `XfrmState` | The dump would otherwise print session keys to a terminal, a log, and a `\| json` pipe |
| The XFRM probe is the enrolled kernel capability, not a doctor-local netlink call (landed, verified 2026-09-08) | The `doctor_xfrm.go` probe seam this spec first planned | `internal/component/kernelcap` answers one probe for `ze doctor`, for the daemon's startup refusal and for `ze config validate`, so the three cannot disagree. A doctor-local probe would be a second declaration of the same fact |
| A gauge with no readable kernel publishes NO series (2026-09-08) | Publish 0, or hold the last value | 0 reads as "no drift" and "no SAs installed", which is the false green the whole spec exists to remove. Prometheus says "unknown" by the absence of a series, so both gauges are a `GaugeVec` even where one label value would do, because `Gauge` cannot be deleted |
| One SAD dump for each metrics pass, taken only when the engine is running (2026-09-08) | One dump for the count and one for the drift comparison | `Update` runs every 5 seconds (`internal/component/ike/engine/register.go`). Two dumps for one pass are two answers to one question, and they can disagree across a rekey. The dump is skipped when `ActiveTable()` is nil, so a daemon with no IKE engine issues no netlink traffic |
| Real ESP bytes are proven in the interop scenario, and the counter `.ci` proves the SOURCE (2026-09-08) | A `.ci` that passes traffic through the tunnel and asserts `bytes-out` above zero | A `.ci` fixture has one kernel, one real backend and no route through the tunnel, so it cannot pass ESP traffic. What it CAN discriminate is where the number comes from: over a real kernel the counter is 0 with `counters-known` true, and over the noop backend it is null. The interop lab already pings through the tunnel, so the non-zero assertion goes there, beside `verifyTunnelTraffic`. Both claims are kept; only the carrier of the traffic-bearing one moves |
| The interop join normalizes both SPIs to `uint32` (2026-09-08) | Compare the printed forms | iproute2 prints an SPI as `0xc1a2b3c4` and the Ze dump answers a JSON number. A string comparison would be false for every SPI, and decoding the Ze answer into `map[string]any` would route a `uint32` through `float64`. The checker decodes into a typed struct and normalizes both sides to `uint32` |

## Known Limitations
- VPP enumeration belongs to `plan/spec-ipsec-vpp-dataplane-readback.md`, an owner-approved, unimplemented backlog spec. AC-6 and AC-15 retain explicit unsupported behavior here.
- The SPD dump reports the kernel's policies for the whole node, not per VRF. Ze does not install per-VRF IPsec policies today.
- `ze_ipsec_tunnel_up` keeps its current definition, so it still reports engine belief. The new `ze_ipsec_dataplane_drift` gauge is the kernel-aware signal beside it. Redefining the existing gauge would change what every deployed dashboard means.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The relevant citations here are RFC 4301 Section 4.4 for the SPD and SAD split,
and RFC 4303 Section 2.1 for the reserved SPI values the selector rejects.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`), and under `ai/rules/pre-release.md` a commit owes no green gate, so closure records product proof and review rather than a green worktree
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Recurring mistakes recorded in the existing `plan/journal/` topics named under Mistake Log
- [ ] Main prepares the separate number-precision, runner-lifetime, and inspection closure commits through the native route
- [ ] Main deletes this spec in the separate closure deletion commit after its complete record is preserved

## Work Inherited From a Deferral Row

The 2026-08-02 row requested VPP binary-API SAD and SPD enumeration behind the
shared list methods. On 2026-09-11 the owner approved XFRM-only inspection for
this release and separated that feature into
`plan/spec-ipsec-vpp-dataplane-readback.md`.

That backlog preserves full SAD/SPD enumeration, public command and consumer
reachability, no key disclosure, and explicit unknown/unsupported fields.
It is a skeleton with research and design open. No VPP readback implementation
or proof is claimed by this closure.

---

## Implementation Summary

### What Was Built

The three public dataplane commands read XFRM SAD/SPD state. Counter, drift,
health, and metric consumers use destination-qualified SA identities and one
generation-checked observation. Unsupported and failed observations remain
errors or unknown values.

The shared command renderers preserve integer JSON tokens through pipes and
record rendering. The functional runner keeps background peers alive until a
self-stopping foreground daemon exits.

### Bugs Fixed

| Defect | Source fix |
|--------|------------|
| SPI-only joins could borrow another SA's counters or presence | `dataplane.SAIdentity` and the qualified joins in `show_ipsec.go` and `health_drift.go` |
| Rekey or removal could mix old kernel state with new engine belief | `ObserveDataplane`, publication generations, and paired backend-write guards |
| Unknown readback could report healthy or retain clean metrics | Degraded unknown health and deletion of both dataplane gauge families |
| Wildcard family and raw-mask loss could corrupt policy identity | Complete policy ownership keys and family-aware netlink decoding |
| Unlimited limits appeared finite, and wide counters lost precision | `hardLimitFromKernel` and number-preserving shared JSON decoding |
| Fixtures stopped peers before foreground observations completed | Removed early stops and corrected runner teardown order |
| Interop evidence accepted incomplete rekey cleanup or ambiguous counters | Require every old SPI absent and compare all four directed kernel counters |
| Policy template endpoints ignored their address family | Decode both endpoints with `tmpl.Family` in the canonical vendor patch |
| The CLI rendered a masked port as an exact port | `portString` retains partial masks and distinguishes exact zero from any |

### Documentation Updates

The inspection and child-XFRM architecture pages describe the identity,
observation, sentinel, mask, family, and VPP contracts. The operator guide,
command reference, and feature row describe the public results. The interop
page explains directed counters and live monitoring transitions.

Go Design headers now name the durable inspection page. The DPD backlog reuses
that page and names the new VPP backlog. Three functional fixture headers retain
the historical spec stem without a dangling path. Historical citation testdata
was not rewritten.

### Deviations from Plan

| Original plan | Implemented result and reason |
|---------------|-------------------------------|
| A doctor-local XFRM probe that always warns | Existing kernel-capability enrollment supplies doctor, startup, and validation. Confirmed absence is an error. An inconclusive probe is a warning |
| A mask-free policy-key inversion | The kernel can hold policies Ze cannot install. Readback preserves their raw masks and address families |
| SPI-only comparison | Full identity is necessary because distinct destinations, protocols, or interfaces can share an SPI |
| Counter `.ci` proves traffic volume | The `.ci` proves numeric zero from a real SAD. The strongSwan lab proves traffic, direction, and agreement with iproute2 |
| Full QEMU and worktree gates as closure conditions | Targeted product proofs ran. Broad red instruments remain red under the pre-release rule and are listed below |
| VPP inherited work stays in this spec | Owner explicitly separated it into `plan/spec-ipsec-vpp-dataplane-readback.md`. AC-6 and AC-15 remain unchanged |

## Mistake Log

Main owns the existing `plan/journal/` rows. No new learned-summary file replaces
those records.

| Journal topic | Recorded lesson |
|---------------|-----------------|
| `key-omits-a-fact-the-builder-uses.md` | SA identity and address decoding must retain the builder's family and destination facts |
| `absent-value-is-a-distinct-identity.md` | An implicit wildcard must use the writer-selected family |
| `zero-value-as-valid-answer.md` | Unknown dataplane state cannot report healthy |
| `reconciler-repairs-against-a-stale-snapshot.md` | Belief and kernel reads need one consistency boundary |
| `field-carries-two-meanings.md` | Preserve mask semantics and translate only the unlimited sentinel |
| `declared-format-contradicts-payload.md` | Wide integer counters must not pass through float64 |
| `false-synchronization-claim.md` | Foreground readiness does not mean the foreground observation has finished |
| `green-that-could-not-have-been-red.md` | Prove complete rekey cleanup, directed counters, and an executable rendering fixture |
| `lint-contract-not-applied.md` | Keep bounded scaffolding findings visible instead of extending product work to erase them |
| `declared-summary-without-its-explanation.md` | Preserve the unrelated startup warning rather than hide it |

## Implementation Audit

### Requirements

The read interface, public commands, counters, drift, health, metrics, diagnostic
capability, and appliance floor have producing code and consumers. The
Acceptance Criteria table below distinguishes runtime proof from source review.
No VPP implementation is included.

### Acceptance Criteria

| AC | Result | Producer and evidence |
|----|--------|-----------------------|
| AC-1 | Implemented and exercised | `saInfoFromState` → `saInfoToMap`. E1, E2, E3 |
| AC-2 | Implemented and exercised | `policyInfoFromKernel` → `policyInfoToMap`. E3 proves ownership, raw masks, and wildcard families. E8 breaks and restores each round 5 repair: `portString` for the public mask, `parseXfrmPolicy` for the template family |
| AC-3 | Implemented and exercised | `handleShowVPNIPsecDataplaneDrift` returns an error naming missing identities. E2 and E4 |
| AC-4 | Implemented and exercised | The drift handler accepts extra kernel SAs and a stable matching observation. E2 and E4 |
| AC-5 | Implemented and exercised | `checkIPsecHealth` distinguishes drift from unknown observations. E2 and E4 |
| AC-6 | Preserved | VPP/noop list methods return `ErrNotSupported`. Handler refusal is covered by E4 and historical reachability evidence |
| AC-7 | Implemented and exercised | `activeDataplane` and `dataplaneReadError` retain nil, unsupported, and permission causes. E4 |
| AC-8 | Implemented and exercised | `readSADCounters` and `addChildCounters` preserve four directed uint64 counters. E1, E2, E4, E5 |
| AC-9 | Landed capability behavior verified at source | `kernelcap_linux.go` enrolls the two diagnostic codes. `kernelcap.evaluateOne` distinguishes absence/error from unknown/warning |
| AC-10 | Verified at source | `checkKernelModules` no longer checks XFRM module names. The capability probe works independently of module packaging |
| AC-11 | Verified at source | `enforceKernelRequirements` rejects missing built-in floor entries. `TestRuntimeFloorRequiresESP` covers each ESP/statistics symbol. No fresh appliance build |
| AC-12 | Implemented and exercised | `checkDataplaneReadback` requires non-empty Ze/kernel SPI agreement and the reverse strongSwan pair. E2 |
| AC-13 | Implemented and exercised | Rekey replaces the SPI set and removes every old SPI. E2 |
| AC-14 | Implemented and exercised | `publishDataplaneGauges` uses one observation and qualified identities. E2 and E4 |
| AC-15 | Preserved and exercised | `clearDataplaneGauges` removes earlier series on failed or changing observations. E2 and E4 |

### Tests

The retained regressions defend observable failures: missing objects, wrong
identities, stale observations, imprecise numbers, lost selector semantics, and
premature teardown. The tests and live scenarios named in Goal Validation were
read with their assertions. This closure agent ran no tests or validators.

### Files Created or Modified

The source inventory is
`tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/ipsec-proof/closure-handoff.json`.
The repair round also updates the existing command YANG help. Closure adds the
operator-page and historical-citation repairs described above. The VPP backlog
is the only new spec.

### Audit Summary

The full closure review found two AC-2 defects. Separate repair owners fixed
both. The bounded fifth source pass found no remaining product defect. E8 then
ran each repair red and green, so every AC is implemented and exercised.

## Goal Validation

Evidence paths below are relative to
`tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/`.
Main ran these proofs before the closure review. The closure agent read their
outputs rather than repeat them.

| ID | Evidence | Observed result |
|----|----------|-----------------|
| E1 | `job-ipsec-functional-discrimination-45ec986d.log` | Empty SAD overlay: both inspection fixtures failed, `EXPECTED_INSPECTION_RED:1`. Restored: `pass 2/2 100.0% 2.6s`, QEMU PASS |
| E2 | `job-ipsec-interop-dump-red-f5b3f152.log`, `job-ipsec-interop-restored-green-6b154269.log`, `ipsec-proof/interop-repaired-green.json` | Empty dump: `no ESP SPI` and `0 passed, 1 failed`. Restored live scenario: `integration: 1 action(s) passed.` |
| E3 | `job-ipsec-family-red-67b6cc47.log`, `job-ipsec-linux-readback-green-6fba7f2e.log` | Old address inference collapsed IPv6 into `32.1.13.184`, leaving one identity instead of two. Fixed kernel readback tests passed without skips |
| E4 | `job-ipsec-observation-unit-784b33c3.log` | Command, engine, interop checker, and vendor-patch packages passed |
| E5 | `job-ipsec-integer-red-6eb423e9.log`, `job-ipsec-integer-green-83913299.log`, `job-ipsec-record-precision-green-8aa5208d.log`, `ipsec-proof/integer-green/cli-proof.json` | Old decoding rounded `18446744073709551615` and `9007199254740993`. Corrected record fixture passed. CLI emitted both exact numeric tokens and exit 0 |
| E6 | `job-ipsec-teardown-discrimination-5622dd03.log` | Old runner lost the peer before the foreground observation, `EXPECTED_TEARDOWN_RED:1`. Fixed runner: `pass 1/1 100.0% 1.2s`, QEMU PASS |
| E7 | `job-ipsec-dpi-qemu-walk-5d0a1d41.log`, `job-ipsec-dpi-interop-red-32b5021c.log`, `job-ipsec-dpi-interop-restored-32b5021c.log` (session `2026-09-08-0a21e591-d035-4f7c-8d1b-231ab023a478`) | Empty `ListSAs`: both `.ci` files FAIL in the QEMU guest (`EXPECTED_RED:1`) and `dataplane-readback` fails on `reports no ESP SPI`. Restored: `pass 2/2 100.0% 7.4s`, `QEMU VM: PASS`, and `integration: 1 action(s) passed.` Detail in "Kernel and strongSwan discrimination walk (2026-09-11)" |
| E8 | `ac2-proof/portmask-red.log`, `ac2-proof/portmask-green.log`, `ac2-proof/qemu-family-walk.log`, `ac2-proof/qemu-family-green.log` (session `2026-09-12-3c95c23d-22f9-4872-9b71-423a77f312e9`) | The two round 5 AC-2 repairs, each broken then restored. Port mask: `portString` without its mask arm reports `4608` where the policy holds `4608/0xff00`, and `TestShowDataplanePolicyPreservesPortMasks` FAILS, then PASSES on the restored source. Template family: `parseXfrmPolicy` back on `ToIP()` reports `32.1.13.185 -> 32.1.13.184` for a `2001:db9:: -> 2001:db8::` tunnel, and `TestXFRMReadbackPolicyTunnelAddressFamily` FAILS against a real kernel (`QEMU VM: FAIL`), then the restored build passes every `TestXFRMReadback` and `TestPolicyReadback` case (`QEMU VM: PASS`). The four host packages pass beside it |

The live interop checker covers clean state, deleted-SA drift, unknown state
under a temporary daemon-only descriptor limit, restoration, and peer removal.
Its helper warms the exporter connection before the limit and restores the
limit through both a watchdog and a `finally` path. All four traffic counters
must advance in their measured direction and agree with `ip -s xfrm state`.

### Instruments Left Red or Not Rerun

| Instrument | Recorded result |
|------------|-----------------|
| Documentation check | `job-ipsec-prerequisite-docs-a984b406.log` reports 3,759 drift issues. No baseline refresh or closure rerun |
| Scoped lint | Command: 1 `appendAssign`. IKE: 6 host / 9 Linux findings. Interop: 6 constant/error-text findings. Earlier runner lint: 2 unrelated test findings |
| Integer rendering fixture | The first fixed-source run still failed resolve variants because the synthetic command lacked a registered shape. The corrected fixture passed in E5 |
| CLI startup | E5 retains `WARN YANG command node missing description path=request` on stderr. Exact JSON and exit 0 were unaffected |
| Broad gates | No fresh worktree, repository, command-contract, citation, full-QEMU, all-tags build, or appliance-build run is claimed |

## Work Not Done

| Work | Owner and destination | Closure treatment |
|------|-----------------------|-------------------|
| VPP SAD/SPD enumeration and consumer integration | `plan/spec-ipsec-vpp-dataplane-readback.md`, owner-separated VPP readback work | Explicitly approved backlog. The current backend continues to refuse enumeration |

## Review Gate

| Item | Result |
|------|--------|
| Independence | IpsecClosure did not author the product changes and ran all review lenses inline |
| Model | Available independent reviewer used with the owner's approval because Opus 5 was unavailable |
| Rounds | 5 total: initial inspection, repair review, SAD-family/NOFILE rereview, full closure review, bounded policy-repair rereview |
| Final source pass | 0 remaining product BLOCKER, 0 remaining product ISSUE |
| Native artifact | `tmp/review/ipsec-dataplane-inspection-3c95c23d-22f9-4872-9b71-423a77f312e9.md`, verdict clean over the 59 files of the closure commit. Recorded on 2026-09-12 after E8 ran the final policy repair proof |

### Findings Fixed

| Severity | Finding | Location | Fixed by |
|----------|---------|----------|----------|
| ISSUE | IPv6 policy tunnel endpoints decoded as IPv4 when their trailing bytes were zero | Canonical netlink patch, `parseXfrmPolicy` | PolicyEndpointRepair: use template family for both endpoints and add a real-kernel regression |
| ISSUE | A partial port mask disappeared from public policy output | `show_dataplane.go`, `portString` | PolicyMaskRepair: masked spelling with handler, JSON, and table assertions |

The Go style and security pass checked changed error paths, ownership, bounds,
locks, goroutine joins, shared-layer registration, and public output. No new
peer-reachable panic or key-material publication was found. SPI input is
bounded and zero is rejected. Netlink errors remain failures. Raw dumps are
read-only and allocate one report row per returned object. They do not promise
pagination or a single globally atomic kernel snapshot.

## Pre-Commit Verification

### Files Exist

| Scope | Result |
|-------|--------|
| Handoff source and evidence inventory | Read at the paths listed in the handoff manifest |
| New VPP backlog | Present at `plan/spec-ipsec-vpp-dataplane-readback.md`, status skeleton |
| Planned doctor-local files | Not created. Existing enrolled capability is the implemented producer |
| Durable citations | Full-path scan leaves only this spec's self-reference and historical citation testdata |

### Acceptance Criteria Verified

| Scope | Result |
|-------|--------|
| AC-1 through AC-15 | Producer and proof level recorded in Implementation Audit. AC-2's two round 5 repairs carry their own red-then-green walk in E8 |

### Wiring Verified

| Path | Evidence |
|------|----------|
| CLI → handler → backend → public renderer | E1 and E2, with source registration and handler review |
| Counters, drift, health, and gauges → coherent observation | E2 and E4. Direct gopls definition resolves the CLI call to `engine.ObserveDataplane` |
| Doctor and appliance build → existing capability/floor | Source review and existing boundary tests. No fresh full build claimed |

### Assumptions Resolved

| Scope | Result |
|-------|--------|
| A-1 through A-6 | Confirmed, with source-only limits stated where applicable |
| A-7 | Original inversion was broken. Complete family and mask semantics repair it |

### Documentation Verified

| Scope | Result |
|-------|--------|
| Category checklist | Updated with the actual subsystem pages and justified N-A rows |
| Changed behavioral prose | Checked against producing symbols. No documentation validator was rerun |
| Shared publication | Main owns commit preparation. `docs/features.md` rides along with one PATHS-LIMIT row another session rewrote: its producer `internal/component/bgp/reactor/session_paths_limit.go` and the guide it links are both in HEAD, so the row states nothing the committed tree cannot support, and holding the file back would leave this spec's own IPsec row unpublished (`ai/rules/git-safety.md`, `ai/rules/documentation.md`) |
| Runner documentation | Main owns the authorized runner-only `ci-format.md` snapshot and restoration. This closure agent did not edit that file |
