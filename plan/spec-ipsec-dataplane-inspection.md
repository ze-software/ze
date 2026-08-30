# Spec: ipsec-dataplane-inspection

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 5/8 (phases 1-5 done; 6 delegated, 7-8 open) |
| Deferral shard | `plan/deferrals/ipsec-dataplane-inspection.md` |
| Updated | 2026-08-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

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
- [ ] `plan/spec-ipsec-dataplane-inspection.md` - this spec: IPsec dataplane inspection
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

## Current Behavior (MANDATORY)

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
6. For the policy view, the handler joins each kernel policy against `ownerOf` so a selector shows which peer claims it.
7. The handler returns `plugin.Map`. It performs no formatting.
8. `command.ApplyPipes` renders a table, or JSON when the operator appends `| json`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ plugin server | wire method `ze-show:vpn-ipsec-dataplane-*`, `plugin.Response` with `plugin.Map` data | No |
| Command handler ↔ dataplane backend | `dataplane.Get()` then the `Dataplane` interface | No |
| Dataplane ↔ Linux kernel | netlink `XFRM_MSG_GETSA` and `XFRM_MSG_GETPOLICY` dumps through the vendored `vishvananda/netlink` | No |
| Engine belief ↔ kernel truth | drift handler reads `engine.PeerInfoMap()` and the SAD dump, and reports the difference | No |
| Interop harness ↔ ze container | new `ze_cli` helper in `test/interop-ipsec/lab.py` (retired; now `internal/le/interoplab/ipsec/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> running `ze cli -c` through `docker_exec` | No |

### Integration Points
- `dataplane.Get()` - already the accessor used by `engine/register.go` and `engine/reconcile.go`.
- `pluginserver.RegisterRPCs` in the existing `init()` of `show_ipsec.go` - one added registration per command.
- `registry.DoctorCheckDef` list in `engine/register.go` - one added entry for the reachability check.
- `health.Register("ipsec", checkIPsecHealth)` in `engine/health.go` - the drift signal joins the existing status computation.
- `ownerOf` in `dataplane/policy_owner.go` - the join key for the policy view.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `netlink.XfrmState.Statistics` is populated by a plain `XfrmStateList` dump, without a per-SA `XFRM_MSG_GETSA` round trip | vendored `xfrm_state_linux.go` declares `Statistics XfrmStateStats` on the dump-parsed struct | the byte counters need a second netlink call per SA, which changes the handler and the interop assertions | `TestXFRMReadbackCountersAdvance` sends traffic through a known SA and asserts the counter rises | **confirmed** 2026-08-03: `XfrmStateList` parses every dump message through `parseXfrmState` and then `xfrmStateFromXfrmUsersaInfo` (vendored `xfrm_state_linux.go`), which calls `curToStats` into `state.Statistics`. The same function fills `Mode`, `Reqid`, `ReplayWindow`, `Limits` and `Selector`. No second round trip |
| A-2 | `policy_owner.go` lands on main before this spec is implemented | the file is untracked in the working tree at the time of writing | the policy view drops its Owner column and `ownerOf` stays unwired | grep for `ownerOf` on main before starting Phase 3 | **confirmed** 2026-08-03: the file is tracked, and `policyOwners.ownerOf` exists in `internal/component/ike/dataplane/policy_owner.go`. Phase 3's stop condition does not fire |
| A-3 | An appliance kernel with `CONFIG_XFRM_USER=y` lists nothing for `xfrm_user` in `/proc/modules`, so `checkKernelModules` reports a false error today | `loadedKernelModules` reads `/proc/modules`, which lists loaded modules only; built-in code never appears there | the module check is correct and only the ESP floor is missing | run `ze doctor` in QEMU against an IPsec config on the appliance kernel and observe the diagnostic | **confirmed at source** 2026-08-03: `readLoadedModules` (`internal/component/doctor/checks_linux.go`) parses `/proc/modules` alone, and `checkKernelModules` in the same file appends `xfrm_user` and `xfrm_algo`. Observing the diagnostic on a built-in kernel still needs QEMU |
| A-4 | A `.ci` under `option=needs-linux:caps=net-admin` can run the real xfrm backend once `fsuite ipsec` is added to the QEMU runner | `ikeDataplaneName()` defaults to `xfrm` when no override is set; other suites already run with that option | the kernel-level `.ci` has no home and the evidence must come from the Go integration tier alone | add the `fsuite` line and run `./le qemu run command "./le qemu all-tests"` | unvalidated. `internal/le/qemu/alltests.go` carries 23 `fsuite` lines and none is `ipsec`; `caps=net-admin` is already used by several `test/reload/*.ci`. Settled only by a QEMU run |
| A-5 | `XfrmPolicyList` returns the policies IKE installs at `if_id 0`, so the SPD dump is not limited to xfrm-interface peers | `GetXFRMInfo` (`internal/plugins/iface/netlink/xfrm_linux.go`) filters the same call by `Ifid`, which implies the unfiltered call returns all of them | the SPD dump misses site-to-site policies, which is most of them | `TestXFRMReadbackPolicyNoIfID` installs a policy with `IfID` zero and asserts the dump returns it | unvalidated. Settled only by a real kernel, and not upgraded on inference |
| A-6 | Returning `ErrNotSupported` from the noop backend does not break an existing functional test | no `.ci` calls `ListSAs` today, because no production caller exists | a `.ci` goes red and the change needs a different shape | `./le verify current mode full` after Phase 1 | **confirmed** 2026-08-03: the retired `ze-functional-ipsec-test` (current: `./le functional ipsec`) is 13/13 with the change in place, including the new `ipsec-dataplane-show.ci`. Earlier runs of the same suite showed 1 to 3 reds that did not reproduce (privileged UDP 500 bind, NAT-T 4500 contention under parallelism); those are load-sensitive and independent of this change |
| A-7 | A kernel policy row from `XfrmPolicyList` inverts back to the `policySelectorKey` that `ownerOf` compares, losslessly, for every policy Ze itself installs | `xfrmSelectorPort` (`internal/component/ike/dataplane/xfrm_linux.go`) REFUSES any port mask other than 0 or `0xffff`, so Ze installs two port shapes and both invert; every other key field (src, dst, dir, upper proto, ifindex, if_id) appears verbatim on `netlink.XfrmPolicy` | the owner join is unsound and the Owner column must be dropped rather than guessed | `TestPolicyOwnerJoinRoundTrips` covers both port shapes and the wildcard prefix | **confirmed** 2026-08-03, by reading `xfrmSelectorPort` and the vendored `parseXfrmPolicy` |

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
| Who else touches this path? | `plan/spec-ipsec-lifetime-volume.md` (Status: design) needs the SA byte counter this spec sources, and its A-1 is unvalidated for exactly that reason. `plan/spec-ipsec-opaque-selector-port-mask.md` wants a policy read-back to assert installed selectors. `plan/deferrals/ipsec-esp-dual-form-receive.md` holds an open decision about an ESP-form field on `show vpn ipsec sa`. `policy_owner.go` is uncommitted work in the same package. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show vpn ipsec dataplane sa` typed at the CLI | → | `handleShowVPNIPsecDataplaneSA` | `TestShowDataplaneSARegistered` |
| `show vpn ipsec dataplane policy` typed at the CLI | → | `handleShowVPNIPsecDataplanePolicy` | `TestShowDataplanePolicyRegistered` |
| `show vpn ipsec dataplane drift` typed at the CLI | → | `handleShowVPNIPsecDataplaneDrift` | `TestShowDataplaneDriftRegistered` |
| CLI dispatch through the plugin server, end to end | → | the three handlers | `ipsec-dataplane-show.ci` |
| `ze doctor <config>` with an ipsec container | → | `checkXFRMReachable` | `TestXFRMReachableDoctorCheckRegistered`, `doctor-ipsec-xfrm.ci` |
| `show vpn ipsec sa \| json` | → | `saToMap` counter fields | `ipsec-show-sa-counters.ci` |
| Appliance kernel build | → | `runtimeKernelRequirements` ESP entries | `TestRuntimeFloorRequiresESP` |
| strongSwan interop run | → | `lab.ze_cli` reading the SAD dump | scenario `dataplane-readback` `check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Child SA is installed on Linux and the operator runs `show vpn ipsec dataplane sa \| json` | Output lists that SA with its SPI, source, destination, if_id, mode, reqid, encryption and integrity algorithm names, replay window, current and hard byte and packet counters, and the add and use timestamps |
| AC-2 | A Child SA policy is installed and the operator runs `show vpn ipsec dataplane policy \| json` | Output lists that policy with its selector prefixes and ports, direction, priority, upper-layer protocol, if_id, template tunnel endpoints, and the owning peer name resolved through `ownerOf` |
| AC-3 | The engine believes a Child SA is installed and the kernel SAD does not hold its SPI | `show vpn ipsec dataplane drift` names the peer, the missing SPI, and the direction, and the command exits non-zero |
| AC-4 | Engine belief and kernel state agree for every peer | `show vpn ipsec dataplane drift` reports no drift and exits zero |
| AC-5 | Drift exists while the daemon is running | `checkIPsecHealth` reports degraded and names the drifting peer |
| AC-6 | The active dataplane backend is VPP or noop, and the operator runs either dump | The command reports that the backend cannot enumerate the dataplane, and never renders an empty table that reads as "nothing installed" |
| AC-7 | `dataplane.Get()` returns nil, or netlink returns EPERM | The command returns an error response naming the cause. It never returns an empty list |
| AC-8 | An established Child SA has passed traffic, and the operator runs `show vpn ipsec sa \| json` | Each child SA object carries `bytes-in`, `bytes-out`, `packets-in`, and `packets-out`, sourced from the kernel SAD, matching the "byte counts" the YANG description has advertised |
| AC-9 | `ze doctor` runs on a host where the XFRM netlink socket cannot be opened, with an ipsec config | A `doctor-ipsec-xfrm-unavailable` diagnostic is emitted at warning severity, naming the netlink failure |
| AC-10 | `ze doctor` runs on a kernel where XFRM is built in rather than modular, with an ipsec config | No `doctor-module-missing` diagnostic is emitted for `xfrm_user` or `xfrm_algo` |
| AC-11 | The appliance runtime kernel config omits `CONFIG_INET_ESP` | `enforceKernelRequirements` fails the build and names the missing symbol |
| AC-12 | A strongSwan interop scenario establishes a tunnel and passes traffic | Ze's own SAD dump reports the same SPI set that `ip xfrm state` reports in the ze container, and the same set strongSwan reports for the reverse direction |
| AC-13 | A Child SA rekeys during that interop scenario | The SPI set reported by Ze's dump changes to match the kernel's, with the old SPI absent and the new SPI present |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks whether a Child SA reached the kernel | CLI → `ze-show:vpn-ipsec-dataplane-sa` → `dataplane.Get().ListSAs` → `netlink.XfrmStateList` → table | `ipsec-show-dataplane-kernel.ci` |
| 2 | Asks why traffic is not encrypted after the tunnel came up | CLI → `ze-show:vpn-ipsec-dataplane-drift` → engine peer map joined against the SAD dump | `TestDriftReportsMissingSPI` |
| 3 | Asks which peer owns a policy selector | CLI → `ze-show:vpn-ipsec-dataplane-policy` → `netlink.XfrmPolicyList` joined with `ownerOf` | `TestPolicyDumpNamesOwner` |
| 4 | Asks how much traffic a tunnel has carried | CLI → `ze-show:vpn-ipsec-sa` → `saToMap` counter fields sourced from the SAD | `ipsec-show-sa-counters.ci` |
| 5 | Checks an appliance is fit to run IPsec before configuring it | `ze doctor` → `checkXFRMReachable` → netlink probe | `doctor-ipsec-xfrm.ci` |

## 🧪 TDD Test Plan

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
| `TestXFRMReachableDoctorCheckRegistered` | `internal/component/ike/engine/doctor_xfrm_test.go` | the check appears in `registry.PluginDoctorChecks()` with its code and phase | |
| `TestXFRMUnavailableDiagnostic` | `internal/component/ike/engine/doctor_xfrm_test.go` | a probe seam returning an error yields `doctor-ipsec-xfrm-unavailable` at warning severity (AC-9) | |
| `TestKernelModulesBuiltInNotMissing` | `internal/component/doctor/checks_linux_test.go` | with XFRM built in, no `doctor-module-missing` is emitted for `xfrm_user` or `xfrm_algo` (AC-10) | |
| `TestRuntimeFloorRequiresESP` | `internal/appliance/kernelreq_test.go` | a runtime config without `CONFIG_INET_ESP` fails enforcement and names the symbol (AC-11) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SPI selector on the dataplane SA lookup | 1 to 4294967295 | 4294967295 | 0 (reserved by RFC 4303 Section 2.1) | 4294967296 |
| if_id filter argument | 0 to 4294967295 | 4294967295 | N/A (0 means every if_id) | 4294967296 |
| Byte counter rendering | 0 to 18446744073709551615 | 18446744073709551615 | N/A | N/A (uint64, must not be narrowed to uint32) |
| Policy priority | 0 to 4294967295 | 4294967295 | N/A | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-dataplane-show` | `test/ipsec/ipsec-dataplane-show.ci` | the operator reaches all three new commands end to end under the noop backend, and each reports that the backend cannot enumerate rather than showing an empty table. Reachability evidence only, not kernel evidence | |
| `ipsec-show-dataplane-kernel` | `test/ipsec/ipsec-show-dataplane-kernel.ci` | two daemons negotiate IKEv2 with the real xfrm backend under `option=needs-linux:caps=net-admin`, and the SAD dump reports the same SPI the SA table reports | |
| `ipsec-show-sa-counters` | `test/ipsec/ipsec-show-sa-counters.ci` | after traffic passes, `show vpn ipsec sa \| json` reports non-zero `bytes-out` for the tunnel | |
| `doctor-ipsec-xfrm` | `test/ui/doctor-ipsec-xfrm.ci` | `ze doctor --json` on an ipsec config emits the reachability diagnostic when the probe seam fails | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dataplane-readback` | `test/interop-ipsec/scenarios/dataplane-readback/` | strongSwan | Ze's SAD dump, `ip xfrm state` in the ze container, and strongSwan's own view report the same non-empty SPI set, and that set tracks a Child SA rekey (AC-12, AC-13) | |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `in-progress` and the work is not implemented) -->

## Files to Modify
- `internal/component/ike/dataplane/dataplane.go` - widen `SAInfo`, add `PolicyInfo`, add `ListPolicies` to the `Dataplane` interface
- `internal/component/ike/dataplane/xfrm_linux.go` - fill the widened `SAInfo` from `netlink.XfrmState`, implement `ListPolicies` through `netlink.XfrmPolicyList`
- `internal/component/ike/dataplane/xfrm_other.go` - `ListPolicies` stub returning `ErrNotSupported`
- `internal/component/ike/dataplane/vpp.go` - both list methods return `ErrNotSupported`
- `internal/component/ike/dataplane/noop.go` - both list methods return `ErrNotSupported`
- `internal/component/ike/dataplane/policy_owner.go` - expose the owner lookup for the policy view.
  → Decision (2026-08-03): the join RECONSTRUCTS `SPParams` from the kernel dump row and calls the
  existing `ownerOf`, rather than adding a second lookup keyed on `policySelectorKey`. Both routes
  face the same normalization, because the key and the reconstruction read the same fields, so the
  second lookup buys nothing and adds an API. A-7 records why the inversion is lossless: Ze cannot
  install a port mask other than 0 or `0xffff`, so the kernel's mask-free `SrcPort`/`DstPort` invert
  to exactly `AnyPortMatch()` or `ExactPortMatch(n)`.
  → Constraint: the two places the inversion is NOT a plain copy are the direction, where Ze's
  `SADir` is one greater than the kernel's `netlink.Dir`, and the wildcard prefix, where Ze installs
  a nil `*net.IPNet` and the kernel dumps a materialized `0.0.0.0/0` or `::/0`. Both are normalized
  in the reconstruction and both carry a test, because a silent error in either mis-attributes an
  owner rather than failing.
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
- `internal/le/qemu/alltests.go` - add the `fsuite ipsec` line
- `test/interop-ipsec/lab.py` (retired; now `internal/le/interoplab/ipsec/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - add the `ze_cli` helper beside `ze_xfrm_state`
- `docs/guide/ipsec.md` - document the new commands and add a troubleshooting section
- `docs/guide/command-reference.md` - the three new commands

## Files to Create
- `internal/component/ike/cmd/show_dataplane.go` - the three handlers
- `internal/component/ike/cmd/show_dataplane_test.go` - handler unit tests
- `internal/component/ike/engine/doctor_xfrm.go` - the reachability check and its probe seam
- `internal/component/ike/engine/doctor_xfrm_linux.go` - the netlink probe
- `internal/component/ike/engine/doctor_xfrm_other.go` - the non-Linux stub
- `internal/component/ike/engine/doctor_xfrm_test.go` - check unit tests
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
| Doctor check for runtime dependencies | Yes | `internal/component/ike/engine/doctor_xfrm.go`, registered in `engine/register.go`, codes in `internal/core/diagnostic/codes.go`, unit test plus `test/ui/doctor-ipsec-xfrm.ci` |
| Prometheus counters/metrics | Yes | `ze_ipsec_dataplane_sa_count` (gauge, label `if-id`) and `ze_ipsec_dataplane_drift` (gauge, label `peer`), registered in `internal/component/ike/engine/metrics.go` beside the existing tunnel gauges |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, the IPsec row |
| 2 | Config syntax changed? | N-A | no config leaf is added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the three new wire methods |
| 5 | Plugin added/changed? | N-A | the ike component is not a plugin in the `internal/plugins/` sense, and no plugin inventory changes |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, the command table plus a new troubleshooting section |
| 7 | Wire format changed? | N-A | nothing on the wire changes. Every new surface is read-only and local |
| 8 | Plugin SDK/protocol changed? | N-A | the `Dataplane` interface is internal to the ike component and is not part of `pkg/plugin/` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC requirement changes polarity. The interop scenario proves an existing claim more strongly, which adds no row |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, the new `fsuite ipsec` QEMU suite and the `ze_cli` interop helper |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, the diagnostic surface row. strongSwan answers this with `swanctl --list-sas` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the dataplane abstraction gains a read side |
| 13 | Route metadata keys added/changed? | N-A | no route metadata is involved |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md`, the two new gauges |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md`, the command inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors naming `show_ipsec.go`, `dataplane.go`, `xfrm_linux.go`, `checks_linux.go`, and `kernelreq.go`, then correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` states that `show vpn ipsec sa` reports byte counts. That claim becomes true in AC-8 and its example output must be regenerated |

## Implementation Steps

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
7. **Phase: Kernel-level and interop evidence** - prove it against a real kernel and a real peer
   - Tests: `ipsec-show-dataplane-kernel.ci`, scenario `dataplane-readback`
   - Files: `internal/le/qemu/alltests.go`, `test/interop-ipsec/lab.py` (retired; now `internal/le/interoplab/ipsec/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, `test/interop-ipsec/scenarios/dataplane-readback/`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `in-progress` and the work is not implemented) -->
   - Verify: revert the read handler and confirm the interop assertion fails. An SPI set that is empty on both sides is the vacuity trap, so assert non-empty before asserting equality
8. **Phase: Documentation** - every row of the Documentation checklist marked Yes
   - Files: as listed in that checklist
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
| Deliverable | Verification method |
|-------------|---------------------|
| Three commands registered and reachable | `bin/ze cli -c 'show vpn ipsec dataplane sa'` returns a response, and `./le docvalid command-contract` passes |
| `ListSAs` has a production caller | grep for `ListSAs` outside `_test.go` returns a handler |
| `ListPolicies` exists on every backend | `go build ./...` with the `ze_vpp` tag set and unset |
| Byte counters present | `test/ipsec/ipsec-show-sa-counters.ci` passes |
| Doctor check registered | `bin/ze doctor --json <conf>` lists the code, and `ze explain doctor-ipsec-xfrm-unavailable` resolves |
| Kernel floor enforced | `ze appliance kernel` fails when the ESP symbol is removed from the fragment |
| QEMU suite wired | `rg 'fsuite ipsec' internal/le/qemu/alltests.go` matches |
| Interop scenario passes | `./le integration interop-ipsec` includes `dataplane-readback` |

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

## Implementation State (2026-08-03)

Phases 1 to 5 are implemented, green, and mutation-verified. Phase 6 is running in
a delegated agent. Phases 7 and 8 are OPEN.

| AC | State | Producing symbol |
|----|-------|------------------|
| AC-1 | done | `saInfoFromState` (`internal/component/ike/dataplane/xfrm_linux.go`), rendered by `saInfoToMap` (`internal/component/ike/cmd/show_dataplane.go`) |
| AC-2 | done | `xfrmBackend.ListPolicies` and `policyInfoFromKernel` (`xfrm_linux.go`), rendered by `policyInfoToMap` (`show_dataplane.go`) |
| AC-3 | done | `handleShowVPNIPsecDataplaneDrift` and `driftMessage` (`show_dataplane.go`) |
| AC-4 | done | same handler, `StatusDone` with an empty `drift` list |
| AC-5 | done | `driftingPeers` and `driftDetail` (`internal/component/ike/engine/health_drift.go`), folded into `checkIPsecHealth` (`health.go`) |
| AC-6 | done | `noopDataplane.ListSAs`/`ListPolicies` (`noop.go`), `vppBackend.ListSAs`/`ListPolicies` (`vpp.go`), reported by `dataplaneReadError` (`show_dataplane.go`) |
| AC-7 | done | `activeDataplane` and `dataplaneReadError` (`show_dataplane.go`) |
| AC-8 | done | `readSADCounters`, `sadCounters.lookup`, `addChildCounters` (`internal/component/ike/cmd/show_ipsec.go`) |
| AC-9, AC-10, AC-11 | delegated (Phase 6) | not verified by this session |
| AC-12, AC-13 | OPEN | the strongSwan interop scenario `dataplane-readback` is not written |

Still to do, none of it started:
- `test/ipsec/ipsec-show-dataplane-kernel.ci` and `test/ipsec/ipsec-show-sa-counters.ci`
- the `fsuite ipsec` line in `internal/le/qemu/alltests.go`, and a QEMU run
- the `ze_cli` helper in `test/interop-ipsec/lab.py` (retired; now `internal/le/interoplab/ipsec/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> and scenario `dataplane-readback`
- the two Prometheus gauges named in the Integration Checklist
- Phase 8 documentation, except `docs/architecture/testing/ci-format.md` which is done

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
| Render algorithm names and key lengths, never key bytes | Render the full `XfrmState` | The dump would otherwise print session keys to a terminal, a log, and a `| json` pipe |

## Known Limitations
- The VPP backend reports that it cannot enumerate rather than dumping the VPP SAD. Implementing the VPP binary-API dump is separable work and does not block any goal here. Row in the deferral shard.
- The SPD dump reports the kernel's policies for the whole node, not per VRF. Ze does not install per-VRF IPsec policies today.
- `ze_ipsec_tunnel_up` keeps its current definition, so it still reports engine belief. The new `ze_ipsec_dataplane_drift` gauge is the kernel-aware signal beside it. Redefining the existing gauge would change what every deployed dashboard means.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The relevant citations here are RFC 4301 Section 4.4 for the SPD and SAD split,
and RFC 4303 Section 2.1 for the reserved SPI values the selector rejects.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
