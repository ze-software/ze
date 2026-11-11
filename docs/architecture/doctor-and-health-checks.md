# Doctor and Runtime Health Checks

Ze has three tiers of self-diagnosis, and they differ by WHEN they answer.

| Tier | Command | When the fact is produced |
|------|---------|---------------------------|
| Readiness | `ze doctor`, and its daemon form `show doctor` | At read time, by re-probing config and the environment. `ze doctor` runs in the operator's own process; `show doctor` runs the same producer (`doctor.Answer`) inside the daemon, and adds the `runtime`-phase checks and the doctor checks of the plugins the daemon runs |
| Live health | `show health` | At read time, inside the daemon, by running a registered probe now |
| Recorded setup outcome | `show plugin list` | Once, in a plugin's own `init()`, before `main()`. The `outcome` and `reason` columns replay it |

The third tier is the only one that remembers. A plugin that failed to set
itself up is often a plugin that never reached the line where it would have
registered a probe, so the first two tiers answer for it with silence, and
absence reads as "not built into this binary". The setup registry answers
because the record is written by the plugin and keyed by its name, and the row
set is derived from the plugin registry, so a plugin that recorded nothing is
listed as `unknown` rather than dropped.

The daemon consults that registry once, at the first statement of `hub.run`,
and refuses to start when a plugin recorded a HARD failure. A CLI verb never
reaches `run`, so no `ze` invocation is refused by it: the command that tells
the operator what is wrong keeps working on a host where the daemon will not
boot.

## The runtime phase: checks only the daemon runs

Some readiness facts exist only in the running daemon: a registry its plugins
fill at runtime, such as every consumer's dns-source selection. A check over
such state registers in the `runtime` phase, through the same
`diagnostic.RegisterDoctorCheck` every other check uses, or, for a plugin,
through the `DoctorChecks` of the `sdk.Registration` it sends at Stage 1,
answered by its `OnDoctorCheck` handler. Only `show doctor` runs it:
`HandleDoctor` calls `doctor.RunRuntimeChecks`, which runs the `runtime` phase
of the in-tree registry, then asks every plugin the daemon runs for its
declared checks (`collectDoctorChecksFrom`, which calls each declared check
whatever its phase).

Each check runs once per answer, through one route. A plugin's runtime check
reads state that lives in the plugin's own process, so its one route is the
doctor callback, which reaches a forked plugin as well as an internal one. The
plugin registry refuses a `registry.DoctorCheckDef` in the `runtime` phase: its
function would run in the daemon's process, over an empty copy of a forked
plugin's state, and beside the callback. A `registry.DoctorCheckDef` therefore
carries a config phase only, and runs in whichever process answers, through
`doctor.Answer`. Stage 1 refuses a declared check whose name the plugin's
registry row already holds (`refuseDoctorCheckDeclaredTwice`), so no check
runs on both routes. The local `ze doctor` process never
runs the `runtime` phase itself (`runChecks` runs the four config phases
only), because in its process that state is empty and the check would report
a box it never looked at as healthy. It asks the daemon instead (next section).

A `runtime` check only reads, and applies nothing. It is handed the platform
alone, never a config tree: the daemon route loads the config file from the
store, not the running configuration, so the check reads its owner's live
state where it lives. The dns-source checks (`doctor-dns-source-empty`,
`doctor-dns-source-interface-silent`, `doctor-dns-source-self-unserved`) are
the first such checks, owned by `internal/component/config/system`.

The MPLS label checks are plugin runtime checks, each declared in its
plugin's Stage 1 registration and answered by its doctor callback, because the
state they read lives in that plugin's process:

| Check | Plugin | Reports | Code |
|-------|--------|---------|------|
| `ldp-label-entries` | `ldp` | a push or pop LDP wants that no writer carried out | `doctor-fib-mpls-no-writer` |
| `rsvp-te-label-entries` | `rsvp-te` | an entry a writer's replay did not install, under an LSP RSVP-TE holds up | `doctor-fib-mpls-no-writer` |
| `ospf-sr-label-entries` | `ospf` | a Segment Routing push, swap or pop no writer carried out, or one OSPF cannot program because it runs forked | `doctor-fib-mpls-no-writer` |
| `bgp-epe-label-entries` | `bgp-epe` | a PeerNode SID label no writer installed or removed as EPE wants | `doctor-fib-mpls-no-writer` |
| `fib-vpp-label-entries` | `fib-vpp` | label entries installed without their path MTU; a label route a previous run left in VPP that could not be removed | `doctor-fib-vpp-mpls-path-mtu`, `doctor-fib-vpp-mpls-stale-label` |

What each finding means, and the stale-label sweep behind the last code, are
in `docs/architecture/mpls/mpls-kernel.md`, "Doctor checks".
<!-- source: internal/plugins/ldp/doctor_label.go -- labelEntriesCheckDecl -->
<!-- source: internal/plugins/rsvpte/doctor_label.go -- labelEntriesCheckName -->
<!-- source: internal/plugins/fib/vpp/doctor_label.go -- labelEntriesCheckDecl -->

There is one phase set and one registry. A second registry of daemon-only
checks would let a check pick the one the local route happens to run, so the
phase is the only place that says when a check runs.
<!-- source: pkg/plugin/rpc/types.go -- DoctorPhaseRuntime -->
<!-- source: internal/component/doctor/registry.go -- RunRuntimeChecks -->
<!-- source: internal/component/doctor/cmd/show.go -- HandleDoctor, answerDaemonDoctor, collectDoctorChecksFrom -->
<!-- source: internal/component/plugin/registry/doctor.go -- validateDoctorCheckDef -->
<!-- source: internal/component/plugin/server/startup.go -- refuseDoctorCheckDeclaredTwice -->
<!-- source: internal/component/config/system/doctor_dns_source.go -- dnsSourceDoctorCheck, dnsSourceFindings -->
<!-- test: internal/component/doctor/registry_test.go TestDoctorRuntimePhaseRunsOnlyThroughTheRuntimeRunner -->
<!-- test: internal/component/doctor/cmd/show_test.go TestDoctorRuntimePhaseRunsOnDaemonRoute -->
<!-- test: internal/component/doctor/cmd/show_test.go TestDoctorPluginRuntimeCheckRunsOnce -->
<!-- test: internal/component/plugin/server/startup_doctor_test.go TestRefuseDoctorCheckDeclaredTwice -->
<!-- test: internal/component/config/system/doctor_dns_source_test.go TestDNSSourceChecksRegisterRuntimePhase -->

## The local route asks the running daemon first

`ze doctor` answers for the box, and the runtime rows exist only in the
running daemon, so the local route asks the daemon before it answers alone
(`answerLocalWith`). It resolves the daemon's SSH address as every `ze <verb>`
daemon command does (`sshclient.DaemonAddress`), connects to it, logs in with
the credentials `ze init` stored (`sshclient.LoadCredentialsNoPrompt`, which
never prompts), and sends `show doctor | raw`. When the daemon answers, its
answer, runtime and plugin rows included, is drawn by the local renderer with
the local exit code, and `--json` stays a local drawing flag. A daemon
"answers" only when that ask returns a doctor result of this schema: a
process that exists but refuses the login has not answered.

When no daemon answers, the local route runs its own checks and adds one
`doctor-runtime-not-checked` warning. Its message says the runtime checks need
the running daemon and why none answered; its fix line is the command that
gets an answer:

| Why no daemon answered | Fix |
|------------------------|-----|
| Nothing listens at the daemon's SSH address | `ze start` |
| A daemon listens, but this account has no stored credentials, or the login or the ask failed | `ze cli --user <name> -c "show doctor"` |
| The daemon's answer is not a doctor result this `ze` reads | `ze show doctor` |
| The daemon's SSH address could not be read from the store | `ze status` |
| `config <file>` names standard input or a file outside the daemon's config directories | `ze doctor` |

The last row keeps the answer about the file the operator named: the daemon
reads a config file only from its own config directories
(`command.CallerPath`), so it would refuse the file or answer for another.
The local route sends `show doctor config <file>` only for a file the daemon
route admits, and otherwise answers that file itself, with the row. The row is
a warning, so it never makes a box not ready on its own.

```
WARN  [doctor-runtime-not-checked] runtime checks did not run: they need the running daemon, and none answered: no daemon listens at 127.0.0.1:2222 (...); this answer holds only the checks ze doctor runs in its own process
      fix: ze start
```
<!-- source: internal/component/doctor/local_route.go -- answerLocalWith, askDaemonDoctor, askDaemonOverSSH, runtimeNotChecked -->
<!-- source: internal/component/cli/sshclient/client.go -- DaemonAddress -->
<!-- test: internal/component/doctor/answer_test.go TestDoctorRuntimePhaseNotRunLocally -->
<!-- test: internal/component/doctor/answer_test.go TestDoctorLocalRouteAsksReachableDaemon -->
<!-- test: internal/component/doctor/answer_test.go TestDoctorLocalRouteNoCredentialsAnswersLocally -->

## Every finding names the command that fixes it

A doctor diagnostic carries `fix`: the command or config statement the
operator runs to clear the problem, alone and with no label, as an operator
error holds its fix (`internal/core/opererr`). The text report draws it on a
line of its own under the finding, behind a `fix: ` label; JSON carries it
under the `fix` key, omitted when the finding names none, on `ze doctor --json`
and on `show doctor` through every surface. A plugin's doctor-check answer
carries the same `fix` key, and both plugin bridges keep it through one
mapping (`doctor.PluginDiagnostic`).

```
ERROR [doctor-fixture-down] fixture is down
      fix: ze start
```

`fix` is not `repair` or `fix-safety`. `repair` names a candidate automated
repair an agent may apply (an id and a summary), and `fix-safety` grades how
safe that repair is. A finding may carry `fix` with neither, and `fix` never
holds a repair id.
<!-- source: internal/core/diagnostic/types.go -- Diagnostic -->
<!-- source: internal/component/doctor/doctor.go -- outputText -->
<!-- source: internal/component/doctor/checks_plugin_registry.go -- PluginDiagnostic -->
<!-- test: internal/component/doctor/render_fix_test.go TestRenderDoctorPrintsFixOnItsOwnLine -->
<!-- test: internal/component/doctor/cmd/show_test.go TestCollectPluginDoctorChecksCarriesFix -->

## Usable disk capacity

`ze doctor` warns below 5% available space on the config filesystem; exactly
5% is healthy. A filesystem reporting zero total blocks is an error, not a
healthy result. Nonpositive available block counts, including FreeBSD's signed
deficits, mean zero usable space rather than a large unsigned capacity.

Doctor, support-bundle disk accounting and the free-space helper share
allocation-free block normalization and overflow-safe arithmetic. Percentages
are rounded down and capped at 100. Support reports available space, excluding
reserved blocks, as `free-bytes`; it caps that count at the total before
calculating `used-bytes`. Byte capacities that exceed `uint64`, or a nonpositive
block size, return an error rather than wrapped capacity. Support retains its
zero-total record with zero used percentage; doctor's stricter zero-total
diagnostic remains unchanged.
<!-- source: internal/core/diskspace/accounting.go -- UsableBlocks, Percent, Bytes -->
<!-- source: internal/core/diskspace/diskspace.go -- Free -->
<!-- source: internal/component/doctor/checks_storage.go -- checkDiskSpace, checkDiskSpaceBlocks -->
<!-- source: internal/component/support/disk_unix.go -- collectDiskInfo, diskUsageFromBlocks -->

## Kernel capabilities: one enrolment, three callers

A subsystem needs a kernel feature the host can lack. MPLS forwarding needs an
AF_MPLS table. IPsec needs the XFRM dataplane. Ze does not discover the absence
when the first labeled route or the first Child SA fails: it asks before it
starts, and it refuses.

The enrolment is `internal/component/kernelcap`. A subsystem registers from its
OWN package, and the registration carries five things: the subsystem name, the
kernel feature, the configuration that requires it, the two diagnostic codes,
and two functions. `InUse` reads the parsed config tree. `Probe` reads the host.

<!-- source: internal/component/kernelcap/kernelcap.go -- Capability, MustRegister, Evaluate, Refuse -->
<!-- source: internal/component/ike/engine/kernelcap_linux.go -- the ipsec enrolment -->
<!-- source: internal/plugins/fib/kernel/kernelcap_linux.go -- the mpls and mpls-transit-mtu enrolments -->
<!-- source: internal/component/ike/dataplane/kernelcap_linux.go -- xfrmKernelCapabilities -->

The XFRM backend enrols what it needs beyond the netlink socket, all with
`Degrades`, because an IPsec configuration can still work without any one of
them:

| Subsystem | Kernel symbol | Probe |
|-----------|---------------|-------|
| `ipsec-mobike` | `CONFIG_XFRM_MIGRATE` (Linux 7.2 on) | `XFRM_MSG_MIGRATE_STATE` naming no SA: `ESRCH` is the handler running, `EINVAL` a kernel before 7.2, `ENOPROTOOPT` the handler built out |
| `ipsec-esp-ipv4`, `ipsec-esp-ipv6` | `CONFIG_INET_ESP`, `CONFIG_INET6_ESP` | `XFRM_MSG_UPDSA` of an AES-GCM SA that cannot exist: `EPROTONOSUPPORT` is no ESP type for the family |
| `ipsec-transform-<name>` | the crypto symbol of each transform | the same update carrying the transform: `ENOSYS` or `ENOENT` is the kernel lacking it |

The transform rows are DERIVED from the backend's algorithm tables
(`xfrmEncNames`, `xfrmAEADNames`, `xfrmAuthNames`), one per distinct kernel
transform, so a cipher the backend learns to install is asked about with no
edit to the enrolment. The update probes need `CAP_NET_ADMIN`. The kernel
builds the whole state, algorithms, ESP type and keyed crypto transform, before
it looks for the SA to replace, and the update names a reserved SPI (RFC 4303
Section 2.1) between documentation addresses, so `ESRCH` proves every piece
exists while nothing was installed.

<!-- source: internal/component/l2tp/kernelcap_linux.go -- l2tpKernelCapabilities -->
<!-- source: internal/component/l2tp/pppoe/kernelcap_linux.go -- pppoeKernelCapability -->
<!-- source: internal/plugins/iface/netlink/kernelcap_linux.go -- ifaceKernelCapabilities -->
<!-- source: internal/component/kernelcap/probe_family_linux.go -- GenericNetlinkFamily, PPPoXProtocol -->

The tunnel and access subsystems enroll the pieces they cannot work without, so
none carries `Degrades` and each refuses a start when its configuration is in
use. They replace the `/proc/modules` lookups the doctor once made for L2TP and
PPPoE, which read "not loaded" for a built-in module and for one the kernel
loads on demand:

| Subsystem | Kernel symbol | In use when | Probe |
|-----------|---------------|-------------|-------|
| `l2tp` | `CONFIG_L2TP` | an `l2tp` block whose `enabled` is not false | generic netlink `CTRL_CMD_GETFAMILY` for `l2tp`: `ENOENT` is no family after the controller's module request |
| `l2tp-ppp` | `CONFIG_PPPOL2TP` | the same | open and close an `AF_PPPOX` `PX_PROTO_OL2TP` socket: `EAFNOSUPPORT` or `EPROTONOSUPPORT` is absent |
| `pppoe` | `CONFIG_PPPOE` | `pppoe { enabled true }` | the same socket with `PX_PROTO_OE` |
| `wireguard` | `CONFIG_WIREGUARD` | a `wireguard` interface under the netlink backend | generic netlink family `wireguard` |
| `xfrm-interface` | `CONFIG_XFRM_INTERFACE` | an `xfrm` interface under the netlink backend | `RTM_NEWLINK` with `NLM_F_CREATE` for kind `xfrm` and interface id 0, inside a throwaway network namespace: `EINVAL` is `xfrmi_newlink` refusing id 0 after the kind resolved, `EOPNOTSUPP` is "Unknown device type" |

None of these needs a privilege except the xfrm interface probe. rtnetlink
answers `ENODEV` to a request without `NLM_F_CREATE` before it looks at the
kind, so only a create request reaches the answer, and a kernel older than the
id-0 check (Linux 6.0) would create the link. The throwaway namespace keeps
that link where nothing sees it, and creating the namespace needs
`CAP_SYS_ADMIN`; without it the row reads unknown and says why. Both probes
that need one go through `kernelcap.InThrowawayNetworkNamespace`.

`mpls-transit-mtu` asks from a throwaway network namespace too, for another
reason: the question needs a label space, and every new namespace starts with
`net.mpls.platform_labels` at 0. The probe sizes the label space inside its own
namespace and asks there, so it answers in a Docker container and on a host
where ze has not yet programmed a label, and the reader's label space is left
as it was. It also needs `CAP_SYS_ADMIN`, and a mount namespace in which
`/proc/sys` is writable; the method is
`docs/architecture/mpls/mpls-kernel.md`, "On a kernel without the patch".

A security policy confining ze can refuse those steps. Docker's
`docker-default` AppArmor profile does both: `deny mount,` refuses the private
mount namespace, and `deny @{PROC}/sys/[^k]** w,` refuses the label-space
write. A step that fails with `EACCES` while `/proc/self/attr/apparmor/current`
(or `/proc/self/attr/current`) names a label other than `unconfined` reads
`denied`, not `unknown`: its reason names the module and the label, the step
and the grant. The module is AppArmor for the AppArmor file and for a
`name (mode)` label, SELinux for a `user:role:type` context, and unnamed
otherwise. An SELinux context whose type is one of the targeted policy's
unconfined domains (`unconfined_t`, `unconfined_service_t`, `kernel_t`), or
`spc_t`, the domain of a privileged container, is no confining label: Red
Hat's SELinux guide says the policy allows those domains "almost all access",
and container-selinux's `container.te` declares
`unconfined_domain_noaudit(spc_t)`, so an `EACCES` under one is not blamed on
SELinux.
`EPERM`, a missing capability, stays `unknown`, and so does `EACCES`
with no confining label. Ze ships the grant for AppArmor only, so under any
other module the reason names the steps the host's policy must grant and no
Ze profile. The AppArmor grant is the profile `ze-kernel-probe`
(`kernelcap.ProbeAppArmorProfile`): docker-default with `deny mount,` replaced
by exactly the three mounts the probe makes and the `/proc/sys` write deny
replaced by a chain that leaves only `/proc/sys/net/mpls/platform_labels`
writable. On a Docker host `./le setup docker kernel apparmor` loads it
(`docs/architecture/testing/interop.md`, "The Docker host kernel check").

<!-- source: internal/component/kernelcap/policy_linux.go -- stepFailed, readConfiningPolicy, classifyLabel -->
<!-- source: internal/component/kernelcap/apparmor.go -- ProbeAppArmorProfile -->

This is the `ze doctor` tier of the table above, not a fourth one. The verdict is
produced at read time, in the reader's own process, and it keeps no memory of a
start. It cannot move into the setup registry, whose records are written before
`main()` and therefore cannot see the configuration this verdict depends on.

`ProbeAll` asks the other question: does this kernel hold every enrolled
feature, whatever the configuration uses. It probes each enrolment with `InUse`
ignored and returns one `Row` per subsystem (subsystem, kernel symbol, state,
reason), and a probe that gave no verdict reads `unknown`. A host that runs Ze's
Docker labs must answer it with every row present
(`plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md`). The operator
reaches it as `ze doctor [--json] kernel-capabilities`, which takes no config
file, prints the rows (as `{"ready":...,"capabilities":[...]}` under `--json`)
and exits 0 only when at least one capability is enrolled and every row is
`present`. An `unknown` or `denied` row is not a pass, and neither is an empty enrolment:
the owners enroll on Linux only, so off Linux the mode answers not ready.

<!-- source: internal/component/kernelcap/kernelcap.go -- ProbeAll, Row -->
<!-- source: internal/component/doctor/kernel_capabilities.go -- writeKernelCapabilities -->

`MustRegister` registers the doctor check too, so one call from the owner puts
the capability on all four surfaces. A second registration would be a second
declaration of the same fact.

### Four states, and the third one is why this is safe

| Probe answer | Diagnostic | What the daemon does |
|--------------|-----------|----------------------|
| Present | none | starts |
| Absent | `SeverityError`, the subsystem's absent code | refuses, exit 1 |
| Absent, capability declares `Degrades` | `SeverityWarning`, the subsystem's absent code, naming what is lost | starts |
| Cannot determine | `SeverityWarning`, the subsystem's unknown code | starts |
| Denied by the host's security policy | `SeverityWarning`, `doctor-kernel-capability-denied`, naming the policy and the grant | starts |

A capability whose absence only weakens its subsystem says so in `Degrades`,
the text the warning carries. `mpls-transit-mtu` is one: a stock kernel refuses
a path MTU on an `AF_MPLS` route, so RSVP-TE transit routes install without it
and still forward (`docs/architecture/mpls/mpls-kernel.md`, "On a kernel
without the patch"). Refusing the start there would stop a working router for
the loss of a bound.

A capability that could not be DETERMINED never refuses. The gate runs on every
`ze` start on Linux, so refusing on a probe ze could not read would turn a
working deployment into a dead one. A probe that returns no verdict at all is
reported the same way: the zero `State` is `Unspecified`, and it is never read as
a pass.

The probes read netlink or procfs. Neither executes an external binary: a second
dependency that can be absent for its own reasons is the fault this enrolment
removes, not a way to detect it. `TestCapabilityProbeExecsNoBinary` holds the
rule over the package's source.

### Four callers, one verdict

| Caller | Where | What it does |
|--------|-------|--------------|
| `ze doctor` | the registered check, run by `runChecks` | renders the diagnostic; `Run` already exits 1 on any `SeverityError` |
| daemon start | `runYANGConfig`, after the config load and any downgrade recovery, before `EnsureActiveVersion` | one stderr line, one `logStartupFailure`, `return 1` |
| config reload | `runReloadContext`, before `ReloadConfig` | refuses with the running configuration still serving |
| `ze config validate` | `runValidation` | `config-kernel-capability` at error severity |

<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the kernel capability refusal -->
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext, the same gate before ReloadConfig -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation -->

The start refusal keeps the idiom the plugin setup refusal uses, and for the same
reason it names EVERY failing subsystem rather than the first: an operator who
repairs one fault and restarts to meet the next pays a whole boot for each fault
after the first.

`ze config validate` answers about the host running the command. A config written
for another machine and validated on a workstation is judged against the
workstation. That is why the gate is not inside `LoadConfig`, which also runs
under `ze doctor` and under offline validation.

There is no operator override, by owner decision (2026-08-14). A NOS that
half-works on a kernel missing a required feature is the hazard this removes, and
an override is what an operator reaches for under pressure.

Tests have one, and a shipped ze cannot read it. The private variable
`ze.test.kernelcap.force` is registered and read only in a build carrying the
`zetest` tag, which is the functional-test daemon and never a shipped binary;
any other build ignores it whatever the environment holds and runs every probe.
It takes `<subsystem>=<state>` entries, comma-separated, where a state is spelled as
the doctor row prints it and any state but unspecified can be forced
(`forcedStates`; the variable's description lists them), and
every reader of the enrolment (doctor, the start and reload gates, validate,
`ProbeAll`) takes the forced answer for a subsystem it names instead of running
that probe. Every forced answer says so: the row's reason and the diagnostic
carry `forced answer (ze.test.kernelcap.force)`, and a forced `present`, which a
probed present never does, reports `doctor-kernel-capability-forced` at warning
severity. A misspelt state leaves the real probe in charge. It exists so a
functional test reaches the absent and cannot-determine rows on a healthy host:
`test/ui/doctor-l2tp-kernelcap.ci`, `test/ui/doctor-pppoe-kernelcap.ci`,
`test/ui/doctor-ipsec-xfrm.ci` and `test/plugin/kernel-capability-*.ci`. A unit
test in another package, whose test binary is not a `zetest` build, forces
through `kernelcap.ForceAnswersForTest`, a Go call no operator input reaches.
<!-- source: internal/component/kernelcap/probe.go -- probe, forcedFor, forcedStates, ForceAnswersForTest -->
<!-- source: internal/component/kernelcap/probe_force_zetest.go -- forcedAnswersFromEnv -->
<!-- source: internal/component/kernelcap/probe_force_shipped.go -- forcedAnswersFromEnv -->
<!-- source: internal/component/kernelcap/kernelcap.go -- forcedPresent -->

### The predicate decides more than the refusal

`IPsecInUse` has three readers: the gate, the kernel module check and the
listener check. An empty `vpn { ipsec { } }` block installs no Security
Association, so it refuses no start, warns about no module and binds no IKE port.
Over-reporting is the same defect in all three.

`MPLSInUse` checks `fib { kernel { } }` BEFORE it looks at any labeled family. A
VPP or P4 backend does its own MPLS, and the plugin that programs kernel labels
is the one that carries the requirement.

### A dead label space is repaired, not reported

`net.mpls.platform_labels` is the size of the kernel's label table and it
defaults to 0, which disables MPLS. A kernel that builds MPLS in therefore boots
with the table present and the label space empty, which is the state every ze
appliance starts in.

So the MPLS probe reads EXISTENCE only, and the fib kernel plugin writes the
label space once, immediately before it programs its first label. An operator's
own value is never overwritten: only a table reading exactly 0 is repaired.

<!-- source: internal/plugins/fib/kernel/labelspace_linux.go -- ensureLabelSpace, repairLabelSpace -->

The appliance kernel builds MPLS routing in (`gokrazy/kernel/runtime.config`,
enforced through `runtime.require`), so an appliance whose configuration puts
MPLS in use passes the gate. `./le test qemu mpls-boot`
(`test/appliance/appliance-mpls-qemu.ci`) proves it on a booted image: the seed
adds `set fib kernel` and `set ldp`, and `show ldp neighbor | json` must answer
over the Ze CLI, which a daemon the gate refused cannot do.

<!-- source: internal/le/test/qemu/mplsboot.go -- MPLSBoot, mplsSeedLines, askJSON -->

## An external plugin depends on two binaries, so doctor reads both

Ze starts an external plugin by giving the operator's `run` string to a shell,
`/bin/sh -c <run>`. The run string is written for a shell: it carries quoting,
and the daemon appends no argv to it. So the shell is a runtime dependency of
every external plugin, beside the binary the run string names.

`plugin-binaries` reads the first dependency and `plugin-shell` reads the second.
Both are registered by the plugin component, because it owns the two call sites
that fork: `(*Process).startExternal` for a live start, and `runQuery` for the
declaration query behind `show plugin declarations`.

<!-- source: internal/component/plugin/shell.go -- Shell, ShellAvailable -->
<!-- source: internal/component/plugin/doctor/check_shell.go -- diagnosePluginShell -->
<!-- source: internal/component/plugin/doctor/register.go -- the plugin-shell registration -->

`plugin-shell` reports `doctor-plugin-shell-missing` at error severity when the
config names at least one external plugin and the host carries no shell ze can
start. The probe asks the kernel whether THIS process may execute that path,
rather than reading the mode bits, because a mode carries three answers and only
the one for ze decides whether the fork starts: a shell with mode 0700 owned by
root has an execute bit and starts nothing under an unprivileged ze. A config
that names no external plugin forks nothing, so it depends on no shell and the
check stays silent. A gokrazy appliance image carries no shell utilities, which is the
host where this fires, and its repair is an `internal` block rather than a shell.

The two fork sites name the shell when they meet the same absence: the live
start reports it in place of the plugin's start error, and the declaration query
carries it as the `unstartable` row's reason. Without that, one missing file
reads as every external plugin failing for its own reason.

## Two tiers on one topic: locking the executable

`memlock` shows why tier one and tier three are not the same fact, and why
neither derives from the other.

| Question | Tier | What answers it |
|----------|------|-----------------|
| Can THIS HOST lock the ze executable at all? | `ze doctor`, before ze runs | The `memlock-rlimit` check compares `RLIMIT_MEMLOCK` against the size of `/proc/self/exe` |
| Did THIS PROCESS lock it? | `show plugin list`, after ze ran | The `memlock` row's `outcome`, recorded by the plugin's own `init()` |

The doctor check warns under `doctor-memlock-rlimit-low` when the limit is
below the executable's size. The file size is a FLOOR for the mapped size, so a
limit below it cannot possibly hold the executable, a limit above it still
might not, and the check claims only the first. It stays silent for a process
holding `CAP_IPC_LOCK`, which locks what it likes whatever the limit says, so
it does not warn on an appliance where ze runs as root. When it cannot read the
host it says so under `doctor-memlock-rlimit-unknown` rather than passing.

The registry cannot answer the first question, because it replays what already
happened on a host where ze already started. The doctor check cannot answer the
second, because it runs in the operator's own process rather than the daemon's.
<!-- source: internal/plugins/memlock/doctor_linux.go -- checkMemlockLimit, memlockLimitDiagnostics -->
<!-- source: internal/plugins/memlock/memlock_linux.go -- init, the outcome it records -->

<!-- source: internal/component/doctor/doctor.go -- offline checks and listener collection -->
<!-- source: internal/core/health/registry.go -- runtime health registry -->
<!-- source: internal/component/plugin/registry/setup.go -- RecordSetup, SetupResults, HardSetupFailures -->
<!-- source: cmd/ze/hub/startup_gate.go -- hardSetupFailure, the refusal run applies -->

## The phase decides what a check may read

Every check declares the phase it runs in, and `runChecks` runs the phases in
order: `pre-config`, `missing-config` (only when no config is found),
`parsed-config`, `post-config`. The fifth phase, `runtime`, never runs there:
only the daemon route runs it (see "The runtime phase: checks only the daemon
runs"). The phase set is declared once, in
`pkg/plugin/rpc` (`DoctorCheckPhase`), and `internal/core/diagnostic` aliases
it, so a check registered in the tree and one a plugin declares name the same
phases, and a plugin can declare every phase an in-tree check can use. Each
phase runs the `diagnostic` registry and then the plugin checks
(`runPluginRegistryChecks`), which filter on the same phase value.

Doctor runs `parsed-config` checks after syntax parsing and before validated
loading. These checks must accept invalid semantics and must not apply config.
The BGP filter-reference check uses this phase so a loader refusal cannot hide
`doctor-config-reference`. `ReadConfig` still runs all load validation; a refusal
remains an error and prevents every `post-config` check from running.

A config phase runs in whichever process answers: the operator's own process
on the local route, and the daemon on the daemon route, where `doctor.Answer`
runs `runChecks` too. So a check registers at the phase whose process holds
the state it reads. Config and host state read the same from both, so a check
over them takes a config phase. Live daemon state exists only in the daemon,
so a check over it takes `runtime`: read from a config phase, the local route
finds it empty and reports the box healthy. A bind probe MUST NOT judge a port
its own running daemon holds, because inside the daemon that bind fails on
every healthy box. The listener probe (`checkListeners`) does not tell the
routes apart yet: on the daemon route it reports each port the daemon serves
as a bind failure. `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`
(AC-1) owns that repair.
<!-- source: internal/component/doctor/doctor.go -- runChecks -->
<!-- source: internal/component/doctor/registry.go -- runDoctorChecks -->
<!-- source: internal/component/doctor/checks_plugin_registry.go -- runPluginRegistryChecks -->
<!-- source: pkg/plugin/rpc/types.go -- DoctorCheckPhase, DoctorPhaseParsedConfig -->
<!-- source: internal/core/diagnostic/doctor_registry.go -- DoctorCheckPhase -->
<!-- source: internal/component/bgp/config/filter_reference_doctor.go -- filterReferenceDoctorCheck -->
<!-- test: internal/component/doctor/checks_plugin_registry_test.go TestRunPluginRegistryChecks_ParsedConfig -->

## Discovery beats enumeration

Listener checks are derived from the YANG schema, through
`RegisterListenerDefault` and `CollectListenersWithDefaults`. A new
`ze:listener` service is then covered by doctor with no doctor code change.
There is no second list: when the schema fails to load, doctor reports
`doctor-listener-set-unknown` (an error naming the cause) and probes no schema
listener. The schema is compiled into ze, so a load failure means a broken
binary, and a guessed list would report the services it misses as checked. A
schema that loads and declares no `ze:listener` service is an answer: the
linked modules carry no listener, so there is no schema listener to probe. The protocol listeners that come
from Go extractors (BGP, BFD, IPsec, TFTP, image server, NTP) do not depend on
the schema and are still probed. The alternative, extending the YANG compiler to
propagate `refine` defaults, would have touched every consumer of the compiler.

<!-- source: internal/component/config/listener_defaults.go -- builtin listener defaults -->
<!-- source: internal/component/doctor/checks_listener.go -- collectSchemaListeners, listenerSetUnknown -->
<!-- source: internal/le/config/ports/actions.go -- Answer -->

Two constraints hold this together:

- The ze YANG compiler does not process `refine`, so `LeafNode.Default` is
  always empty for a `uses zt:listener` child. Every schema-driven default
  lookup has to allow for that, not only the listener one.
- `config.YANGSchema()` succeeds with an empty schema when no YANG module is
  registered, which is the context of a test run without the feature tags.
  Doctor then probes no schema listener and reports nothing about it, so a
  doctor test that asserts a schema listener's probe runs with the full tag
  set.

`ze doctor` and the daemon's `show doctor` read one request (`answerDoctor`)
and run one set of checks. The local route asks the running daemon for its
answer and runs the checks in the operator's own process only when no daemon
answers, with no plugin check, because no plugin runs there (see "The local
route asks the running daemon first"). `doctor` is an offline root, not a verb, so it
has no daemon path of its own: its registration names `show doctor` as its
daemon form (`registry.Meta.DaemonPath`), and every daemon surface offers
`show doctor` in its place. The daemon registers one RPC (`HandleDoctor`),
which runs the same producer and adds the doctor checks of every plugin the
daemon runs, so an operator or an AI on SSH, MCP, REST, gRPC or the web reads
the readiness of the daemon's host. `show doctor` declares the `config` and
`kernel-capabilities` leaves, and `ze doctor` is judged by the same leaves; `--json` stays a local
rendering flag, and on the daemon route the caller's `| json` renders. On the
daemon route `config <file>` must lie in a config directory of the daemon's
host, and `-` (stdin) is refused. The text report and its exit code (1 when
not ready) belong to the local drawing; the daemon answers a not-ready result
as data.
<!-- source: internal/component/doctor/answer.go -- Answer, renderDoctor -->
<!-- source: internal/component/doctor/cmd/show.go -- HandleDoctor -->

## Probe the role the service actually plays

NTP is a client in ze, not a server. The reachability check sends an SNTP
probe over UDP to each configured server, matching the daemon's own clock-skew
probe. A TCP connect would hit a closed port and report a false failure. Doctor
does bind UDP/123 separately, for the NTP listener check.

Injectable function variables (`httpHead`, `probeWritable`,
`ntpServerReachable`) isolate the probes in tests, which matches the existing
doctor test style. `probeWritableDir` removes its temporary file before
checking the `Close()` error, otherwise a failing close leaks the file.

## A run with no store warns and exits 0; a store it cannot read errors

`ze doctor` opens the store read-only before any check runs. When the folder
holds no store at all (`storage.ErrNoStore`) the run reports
`doctor-storage-unavailable` as a WARNING naming the folder, and each check that
needs the store then refuses its own feature by name under the same code (`local
certificate authority root: refused, this doctor run resolved no storage`, `web
TLS pair: refused, ...`), so the operator reads one warning per feature and the
run exits 0: a host that has not run `ze init` yet is not broken. A folder with
a store the process cannot read stays an ERROR: `doctor-store-permissions` for a
permission refusal, `doctor-storage-unavailable` for any other open failure, and
a corrupt key inside an open store is `doctor-store-integrity`.

<!-- source: internal/component/doctor/doctor.go -- resolveStorageWithDiag -->
<!-- source: internal/component/pki/doctor.go -- checkCARoot, the no-store refusal -->
<!-- source: internal/component/web/doctor_material.go -- webTLSMaterialDiagnostics, the no-store refusal -->

## Health checks produce warnings before reading them

`checkFirewallHealth` calls `AuditTables()` and `checkIfaceHealth` calls
`CheckAllInterfaceErrors()`. Each kernel call runs on a goroutine with a
one-second timeout, so a stuck kernel call cannot stall the health endpoint.

`AuditTables` has three outcomes, not two: it finds drift, it finds none, or it
could not read the kernel at all. The third returns an error beside its zero
finding count, and `checkFirewallHealth` reports it as degraded with the reason
"firewall audit could not run". A check that answers healthy for a kernel it
never read publishes an answer it does not have.

A desired ze_* table the kernel no longer has is DRIFT, and it raises
`firewall-drift` with the other structural differences. The apply that
installed the table returned long ago, so its absence means something outside
ze removed it, and nothing else looks for it: the `firewall-stale-table` branch
reports only tables ze does NOT want. `ze_vrrp` carries the RFC 9568 Section
6.4.3 accept filter, so the table vanishing means a non-owner router accepts
packets for the virtual address.

The `l2tp` check reads the data-plane outcome the subsystem recorded at start,
not only whether the subsystem is registered. A listener whose kernel worker
could not be built (no l2tp generic netlink family, the
`ze.l2tp.disable-kernel-dataplane` knob, a non-Linux build) still establishes
sessions on the control plane, and none of them carries traffic, so the check
reports degraded with that reason.

The `vpp` check dials the socket the running vpp component dials
(`Connector.Socket`, the configured `vpp { api-socket }`), not a fixed path.
While VPP is the interface backend, a vpp component that is not running, a
socket file that does not exist and a socket that refuses connections each
report down with the path and `ze doctor` as the next step: a VPP backend
whose daemon never started has no data plane, so it never reads healthy. A check with nothing to measure registers through
`health.RegisterWhen`: while its gate answers false the report omits the row,
and a gate that panics reports the row down.

<!-- source: internal/plugins/iface/vpp/health.go -- checkVPPHealth, vppHealthApplies, activeVPPSocket -->
<!-- source: internal/component/vpp/conn.go -- Connector.Socket -->
<!-- source: internal/core/health/registry.go -- RegisterWhen, invokeCheck -->
<!-- source: internal/component/firewall/audit.go -- firewall drift audit -->
<!-- source: internal/component/iface/health.go -- interface error counter tracking -->
<!-- source: internal/component/l2tp/health.go -- checkHealth -->
<!-- source: internal/component/l2tp/subsystem.go -- kernelDataPlane -->
<!-- source: internal/component/bgp/reactor/session_health.go -- EOR timeout and session anomaly -->

Four bounds keep the checks honest:

| Check | Bound | Reason |
|-------|-------|--------|
| Route-count anomaly | 100-prefix floor | a route-server client or a management peer holds 1 to 5 routes, and any ratio on those is noise |
| Per-family EOR | one timer per peer, expected against received family count | the warning clears only when every negotiated family has sent EOR, not on the first one |
| Firewall drift | skipped while `LastApplied() == nil` | before the first apply there is nothing to compare, so every table reads as drift |
| VPP health | registered with `health.RegisterWhen`, gated on `interface backend vpp` being the loaded backend | a host with no VPP has no row, so it neither answers 503 forever nor carries a healthy row nobody measured |

Prefix counting is unconditional rather than gated on a configured maximum,
because the anomaly detector needs a count on every peer. An early return in
`applyPrefixDelta` skips the family-string allocation when neither metrics nor
warnings are configured.

The FIB pending map is capped at 10000 entries, so a catastrophic backend
failure cannot grow it without bound.

## The dependency inventory is a test

`TestDoctorDependencyInventory` holds an expected total. Adding a runtime
dependency without a doctor check fails it, which forces the decision to be
made rather than forgotten.

## Every problem site names a doctor code

<!-- source: internal/le/arch/doctorcoverage/sites.go -- sitesOf, siteOf, codeAttr, resolve, findingsOf, findingSite -->
<!-- source: internal/le/arch/doctorcoverage/codes.go -- coveredCodes, checkDeclarations, addCheck -->
<!-- source: internal/le/arch/doctorcoverage/report.go -- checkLines, treeReport, judgeChange, addedSites -->
<!-- source: internal/le/arch/doctorcoverage/register.go -- changecheck.Register -->

The inventory test counts dependencies. It says nothing about the problems an
operator meets while Ze runs. `./le arch doctor-coverage` is the gate for
those. Three kinds of call are a problem site, and so is a finding the doctor
or the config validator builds:

| Site | Where the operator meets it | How it names its code |
|------|-----------------------------|-----------------------|
| A `Warn` or `Error` log line, any form (`WarnContext`, `Log` and `LogAttrs` at `slog.LevelWarn` or `slog.LevelError`), its message a literal, a constant, a concatenation, a string variable or an error's text; or a call of a wrapper that forwards its message and attributes to one | the log | a `code` attribute: `"code", diagnostic.CodeDoctorX` among the key-value arguments, or `slog.String("code", ...)`, nested in an operator error's `LogAttrs` included; a wrapper's call names it among the attributes it passes |
| `report.RaiseWarning` or `report.RaiseError` | `show warnings`, `show errors` | its code argument |
| `health.Register` or `health.RegisterWhen` | the health endpoint | none: a health row carries a name, so the audit maps it to its check by hand |
| a `diagnostic.Diagnostic` composite literal (a doctor finding), or an `rpc.DoctorCheckDiagnostic` literal (the finding a plugin's doctor callback answers) or one of its SDK alias `sdk.DoctorCheckDiagnostic`, written plainly, with `&`, as an elided element of a slice literal, through an import alias, or unqualified inside the declaring package | `ze doctor`, `show doctor`, config validation, `ze config fix --plan` | none: a finding is judged by its `Fix` field, not by a code |

The scan reads syntax, not types, so a method call on a value is a log call
only when its arguments read as one: the message is a string the source
shows (a literal, a constant, a concatenation with a string operand, a
variable an enclosing function declares as a string or with a string value,
an error's `Error()` text, a `String()` or `fmt.Sprint` call), or the
message is followed by attributes. A value's `Error(err)` with nothing after
it is no site, because `err` is no string. A wrapper is a function or method
whose last two parameters are a string message and a variadic attribute list,
and whose body passes both on, the message alone or with a prefix
concatenated (`"iface dhcp v6: "+msg`), to a `Warn` or `Error` call or to
another wrapper (dhcpserver's `warnUnserved` forwards to `warnLimited`, which
forwards to `Warn`). It is found from its body, never from its name. Each
call of a wrapper is a site of the wrapper's level, judged for the `code`
attribute that call passes, and the forwarding call inside the wrapper is
none. With no types, a name is matched as Go scopes it: a function by its
package and name, a method by its name in its package, and an exported
method by its name anywhere. A method named `Warn`, `Error` or `Log` is
judged by its own shape instead. A `Log` whose level the source does not
spell (`x.Log(ctx, level, msg)`) is no site, and neither is a wrapper that
forwards one. A function stored in a field and called through it (`seg.warn
= r.warnWithheld`, then `seg.warn(...)`) is not followed.
<!-- source: internal/le/arch/doctorcoverage/wrappers.go -- findWrappers, candidateOf, forwardsOf, passesOn, stringValued, stringName -->
<!-- test: internal/le/arch/doctorcoverage/judge_test.go TestGateSeesWrapperAndNonLiteralWarn -->
<!-- test: internal/le/arch/doctorcoverage/report_test.go TestDoctorCoverageReportListsPluginFindings -->
<!-- test: internal/le/arch/doctorcoverage/judge_test.go TestDoctorCoverageCheckRefusesPluginFindingWithoutFix -->

A site is covered when its code is one that a doctor check declares. The
gate reads the covered codes from the same parsed source the sites come from,
never from the registries linked into `le`: every composite literal of a
doctor check declaration type (`diagnostic.DoctorCheck`,
`registry.DoctorCheckDef`, `rpc.DoctorCheckDecl` and its SDK alias, and
`kernelcap.Capability`, whose check `kernel-capability-<Subsystem>` declares
its `CodeAbsent` and `CodeUnknown`), whatever registers it, and every
`diagnostic.EventCode` literal, whose code is declared by each
`diagnostic.DoctorCheck` literal that sets `DeclaresEventCodes: true` (see
"Event codes" below). Each code in its
`Codes` list resolves the way a site's code does (a literal, a constant of its
package, an exported constant of another tree package), and the list may be a
package-level variable. So a check in a package no composition root imports
covers its codes, and the commit form reads each side's covered set from that
side's tree: a commit that adds a check and its first use is admitted. The
gate holds no list of codes. Its list of declaration types is a second copy
of what the registries hold, so
`TestDoctorCoverageSourceHoldsEveryRegisteredCode` links the product and
fails when a registered check declares a code the source read misses. A code
must be a literal or a constant the scan can read: a code built at run time
cannot be matched to a check, and is refused like a missing one.

A doctor finding is the record the operator reads as it is, so it is covered
when it names what clears it: its `Fix` field is set to anything other than
the empty string literal. A literal with no `Fix`, or with `Fix: ""`, is
uncovered; a constant, a variable or a call counts as set, because only a
literal can be read as empty. Only the literal is read: a `Fix` a caller
assigns afterwards (`diags[i].Fix = ...` over the findings a shared helper
built) is not seen, so the helper's literal reports `no-fix` even on the path
whose caller sets one. A literal with no element at all is the zero
value a function answers beside "no finding" (`return
diagnostic.Diagnostic{}, false`), and is no site. The call sites are judged
under the three site roots, because a library's caller decides what the
operator sees; a finding is judged under every shipped tree,
`internal/core` and `internal/appliance` included, because a doctor check's
own result reaches the operator unchanged. A finding's code needs no check
declaring it. Each site's kind and status are on `Scan`'s `Site` record
(`Kind` `doctor-finding`, `Status` `fix-set` or `no-fix`), and
`Status.Covered` answers covered for a call site or a finding alike.
`ScanSide` is `Scan` over one side of a commit; `./le arch error-inventory`
reads it there, so its commit judge leaves a writer call on a line this scan
names to this gate (`docs/contributing/error-inventory.md`).
<!-- source: internal/le/arch/doctorcoverage/report.go -- ScanSide -->

| Form | What it judges | Exit |
|------|----------------|------|
| `./le arch doctor-coverage check` | the sites on the lines changed since the upstream: call sites under `internal/component`, `internal/plugins` and `cmd/ze`, findings in any shipped Go. A site with no code, a run-time code, or a code no check declares is refused, with its file and line and the cover; so is a finding with no `Fix`, whose cover is to set `Fix` to the command or config statement that clears the finding. A health row is listed but not refused | 0 clean, 1 refused, 2 could not judge |
| `./le arch doctor-coverage report` | every site in the tree that is not covered, grouped by door area and owner package, with counts, the doctor findings and those with a `Fix` among them. A site the change did not add is reported here and never refused | 0, or 2 |
| `./le commit create` | the sites the commit adds: it scans the tree as the commit leaves the carried Go files and as the parent holds them, and refuses each refusable site in a carried file that the parent held fewer of, keyed by file, call (a finding's type), message and code expression. Moving an old uncovered line is no addition; a new or reworded one is, and so is a covered line whose code, or a finding whose `Fix`, the commit removes. A commit carrying no shipped Go is admitted without a scan | refused, or exit 3 when the check could not judge |

No marker comment exempts a site, because the scan never reads comments. A
transient class (a peer reset that recovers, one retry) still carries its
code: it raises its event on the report bus under that code, so the event
is reported under it rather than only logged. The `code` key is the doctor
code's: the scan reads whatever value a log call gives it as a doctor code,
so a protocol code or an exit code goes under another key, and under `code`
it is refused like an undeclared code. A line that reports nothing wrong is
not a problem site and logs at `Info` or `Debug`: a `Warn` that reports
success owes a code no check could declare. The commit form registers into
the `changecheck` seam from the package's own `register.go`, so the commit
code never names it (`docs/contributing/committing.md`). A scan that finds fewer than
1000 sites, or a covered set below 20 codes, exits 2: a call shape or a
declaration type the gate no longer matches would otherwise read as a clean
tree, or refuse every new line.

## Recent events: the report bus in show doctor

A transient problem (a peer reset that recovered, one failed retry) is no
state a check can probe after the fact. Its producer raises it on the report
bus (`internal/core/report`): an error event into the bounded error ring, or an
active warning until the producer clears it. `show errors` and `show warnings`
print that store. The `recent-events` check, owned by `internal/component/doctor`
because the bus is core and has no component, reads the same store
(`report.Errors` and `report.Warnings`) and keeps no copy of its own.

It runs in the `runtime` phase, so only `show doctor` runs it: in the local
`ze doctor` process the bus is always empty. It answers one row per producer
code and subject, error events and active warnings apart, newest first:

| Part of the row | What it holds |
|-----------------|---------------|
| Code | the event's own code, when `ze show diagnostic` describes it; otherwise `doctor-recent-events`, with the event's code written in the message |
| Severity | `warning`, whatever the event's: the event already happened and its owner may have recovered, so the row never makes the box read not ready |
| Message | the source, error event or warning, the subject, how many times it happened and when it last did (RFC 3339, UTC), and the newest event's message |
| `fix` | the fix the newest event's raise carried (`report.RaiseErrorFix`, `report.RaiseWarningFix`); when it carried none, `show errors` for error events and `show warnings` for warnings, which each list every event behind the row |

The producer knows the command that fixes its subject, so the fix travels on
the raise (the `Issue` field `Fix`, JSON key `fix`, at most 256 bytes) rather
than in a table of codes here. `report.RaiseWarning` and `report.RaiseError`
raise with no fix.
<!-- source: internal/core/report/report.go -- Issue, RaiseWarningFix, RaiseErrorFix, validFields -->

An active warning whose code another `runtime` check declares in its own
`Codes` is that check's row and is not repeated here: the check reads the state
the warning mirrors, so one fault gives one row (`doctor-plugin-stopped` is
reported by `plugin-stopped`, not by `recent-events`). Error events always
render, because they are history no state check reports.
<!-- source: internal/component/doctor/checks_recent_events.go -- runtimeCheckCodes, warningsNoCheckOwns -->
<!-- test: internal/component/doctor/checks_recent_events_test.go TestRecentEventsSkipsWarningAStateCheckOwns -->

### A full bus

The bus is bounded: `ze.report.warnings.max` active warnings and
`ze.report.errors.max` error events, each clamped to 10000. Past the warning cap
`evictOldestWarning` drops the oldest active warning, and past the error cap the
ring overwrites its oldest event. Both are counted, and `report.Losses` answers
the counts with the caps. The `report-bus-capacity` check, registered by
`internal/component/plugin/doctor` at the `runtime` phase, answers one
`doctor-report-bus-full` warning per kind of loss, naming the cap, how many
entries were lost and the fix: an export of the env key at twice the cap, or
`ze show warnings` / `ze show errors` when the cap is already at its largest.

A dropped warning is also absent from `report.Warnings`, so a `HealthProbe*` row
that filters on its code would read the absence as healthy while the condition
still holds. The bus keeps each evicted key until its owner clears it
(`ClearWarning`, `ClearSource`) or raises it again, and a probe with none of its
codes active reads degraded "report bus full" while one of them is in that set.
The set is bounded by the warning cap; past that bound every probe reads
degraded until the store is replaced. A bus that never reached its caps reads
as it always did.
<!-- source: internal/core/report/report.go -- evictOldestWarning, mayHaveEvicted, Losses, evictionDoubt -->
<!-- source: internal/core/report/health_probe.go -- probe -->
<!-- source: internal/component/plugin/doctor/check_report_bus.go -- checkReportBusFull, busFullRow -->

### Event codes

The check declares `doctor-recent-events` and every event code. An owner that
raises a transient code on the bus registers it from its `init()` with
`diagnostic.RegisterEventCode(diagnostic.EventCode{Code: ...})`, the code a
literal or a constant. The registry refuses a code `ze show diagnostic` cannot
describe: its entry must be in the builtin table (`builtinCodes`, read
directly, because it registers only after `init()`) or registered already.
Two owners may register one code. The check sets `DeclaresEventCodes`, so
every listing of the doctor registry (`DoctorChecks`, `DoctorChecksForPhase`)
answers its `Codes` followed by `diagnostic.EventCodes()`: the codes are added
when the registry is read, because the owners' `init()` order is not fixed.
The doctor-coverage gate reads the same declaration from the source: an
`EventCode` literal's code is covered under every `DoctorCheck` literal that
sets `DeclaresEventCodes: true`.

A bus code nobody registered is still rendered under its own code when
`ze show diagnostic` describes it, and under `doctor-recent-events` when it
does not: so a transient class is covered either by its registration here or
by a code its own owner's check declares, and raising the event on the bus is
what makes this check report it. Registering a code whose sites only log, and
never raise, would declare a row this check never shows: every site carrying
a registered event code raises it.

A plugin raises through `sdk.Plugin.RaiseIssue`, which sends `report-issue`
to the daemon, never through `internal/core/report` directly: a forked
plugin's own bus is a copy in its process that this check never reads. The
daemon raises it under the plugin's configured name with the fix the plugin
gave, and refuses a code that is not a registered event code, so a plugin can
raise only a code this check declares (`docs/architecture/api/process-protocol.md`,
"Plugin problems").
<!-- source: internal/component/plugin/server/dispatch_report.go -- opReportIssue -->
<!-- source: pkg/plugin/sdk/report.go -- RaiseIssue, ClearIssue -->
<!-- source: internal/core/diagnostic/event_codes.go -- EventCode, RegisterEventCode, EventCodes, describedCode -->
<!-- source: internal/core/diagnostic/doctor_registry.go -- DoctorCheck, listedDoctorCheckReg -->
<!-- source: internal/component/doctor/checks_recent_events.go -- recentEventsCheck, checkRecentEvents, groupRecentEvents, recentEventsDiagnostic -->
<!-- source: internal/component/doctor/register.go -- init -->
<!-- source: internal/core/report/report.go -- Errors, Warnings -->
<!-- test: internal/component/doctor/checks_recent_events_test.go TestDoctorRecentEventsRowsFromReportRing -->
<!-- test: internal/component/doctor/checks_recent_events_test.go TestDoctorRecentEventsCodeWithoutMetadata -->
<!-- test: internal/component/doctor/checks_recent_events_test.go TestEventCodeRegistrationDeclared -->
<!-- test: internal/component/doctor/checks_recent_events_test.go TestRecentEventsRendersRaiseFix -->

## Plugin state a runtime check reads

A runtime check is handed the platform alone, and `DoctorCheckContext` is
shared core, so it carries no handle on the plugin server. The server publishes
itself instead, in the core-leaf registry shape of `internal/core/ipsecinventory`:
`StartWithContext` publishes and `Stop` withdraws, and a check calls
`server.PluginDoctorSnapshot` for value types. Each configured plugin appears
whether or not a process runs for it, with whether it is running, the stage it
reached, whether the respawn limit disabled it, and the cause recorded for a
failed startup. With no server in the process the answer is
`ErrDoctorSnapshotNotPublished`, and before plugin startup begins
`ErrDoctorSnapshotNotStarted`: never an empty list that reads as every plugin
being fine.
<!-- source: internal/component/plugin/server/doctor_snapshot.go -- PluginDoctorSnapshot, DoctorSnapshot, PluginDoctorState, publishDoctorSnapshot -->

Why ze last stopped itself survives the stop in the `meta/plugin/last-stop`
key (`docs/architecture/zefs-format.md`). `stopDaemonForPlugin` writes a
`fatal-policy` entry before it stops ze, a startup that ends with an error
writes `boot-refused`, and `RecordStopCleanupLate` writes `stop-cleanup-late`
for an owner whose stop-time cleanup ran late. Each entry carries the reason,
the plugin, a detail and the time; the record keeps the first 16 and counts the
rest. A start that completes cleanly deletes it, unless that same run recorded
a stop. The daemon reads it with `ReadLastStop`; the local checks, whose
process registers no statestore, read it from the store they are handed with
`ReadLastStopFrom`.
<!-- source: internal/component/plugin/server/last_stop.go -- RecordLastStop, RecordStopCleanupLate, ReadLastStop, ReadLastStopFrom, settleLastStop -->

### Plugin lifecycle checks

Every path that leaves a configured plugin not running records it with the
process manager (`ProcessManager.RecordStopped`): its failure policy
(`ignore`, or none declared) after an exit, the respawn limit, a failed
restart, and a startup handshake that did not reach running. The record holds
the cause, the policy applied, the exit status (an internal plugin's runner
code, a fork's reaped state) and the detail, and it raises
`doctor-plugin-stopped` on the bus, so the `plugins` health row reads down on
every one of those paths. A generation that reaches running clears it
(`superviseProcess`), and so does a removal (`RemoveProcess`): a plugin the
configuration no longer names is not down. `RemoveProcess` and `ReportCrash`
also clear the warnings the plugin raised itself through `report-issue`
(`report.ClearSource` of its name), because only the plugin could clear them.
<!-- source: internal/component/plugin/process/stopped.go -- RecordStopped, ClearStopped, StoppedMessage, RestartCommand, ExitStatus -->
<!-- source: internal/component/plugin/server/failure_policy.go -- superviseProcess, applyFailurePolicy -->

| Check | Phase | Code | Reports | Fix |
|-------|-------|------|---------|-----|
| `plugin-stopped` | runtime | `doctor-plugin-stopped` (error) | each configured plugin recorded down, with the cause, policy, exit status and time | `ze request plugin restart <name>` |
| `recent-events` | runtime | `doctor-plugin-crashed` (event) | each unasked exit, with its exit status and the policy ze applied | `ze show errors` |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-fatal-stop` (error) | each `fatal-policy` or `boot-refused` entry of the last-stop record, with plugin, detail and time | the plugin's log lines before that time, then its config change |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-stop-cleanup-late` (warning) | each `stop-cleanup-late` entry of the last-stop record: the stop went on before that plugin finished its cleanup, so state it installs may remain | `journalctl -u ze --until <time> \| grep <plugin>` |
| `plugin-autoload` | post-config | `doctor-plugin-autoload-unresolved` (error) | the config's auto-load set that `registry.ResolveDependencies` refuses, with the boot refusal's text | the fix the refusal names |
| `plugin-setup` | pre-config | `doctor-plugin-setup-failed` (error for hard, warning for soft) | each `registry.SetupResults` failure, with the recorded reason | the remedy after the reason's last `; `, else `ze show plugin list` |

`request plugin restart <name>` starts a configured plugin that is not running
again, lifting a respawn-limit disable because the operator asked for this
restart; a running plugin and an unknown name are refused. Tab after the
command offers the configured plugins that are not running, the names it
accepts (`completePluginRestart`, served through `system command complete
args`; `docs/architecture/cli/command-completion.md`).
<!-- source: internal/component/plugin/server/register_plugin_restart.go -- handlePluginRestart, pluginRestartRefusal, completePluginRestart -->
<!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- checkPluginFatalStop, cleanupLateRow -->
<!-- source: internal/component/plugin/doctor/register_lifecycle.go -- init -->
<!-- test: internal/component/plugin/doctor/checks_runtime_test.go TestPluginStoppedEveryDownPath -->
<!-- test: internal/component/plugin/doctor/check_setup_test.go TestPluginSetupFailedReportsRecordedOutcome -->

## Environment values ze could not use

An env getter that cannot parse a value answers its caller's default, and the
report bus replaces a cap set outside its range: either way the daemon runs on a
value the operator did not export. `internal/core/env` records each case and
judges every registered entry against its type (`env.InvalidValues`,
`docs/architecture/config/environment.md`, "Values ze could not use"). The
`env-values` check, owned by `internal/component/doctor` because env is core
and has no component, turns each record into one `doctor-env-invalid-value`
warning naming the key, the value (withheld for a `Secret` key), the type and
what ze uses instead. Its fix is the export that corrects it, the name in its
shell spelling. It runs in the `runtime` phase: the daemon's environment is the
one the daemon runs on.
<!-- source: internal/component/doctor/checks_env_values.go -- envValuesCheck, checkEnvValues, envValueDiagnostic -->
<!-- source: internal/core/env/invalid.go -- InvalidValues -->
<!-- test: internal/component/doctor/checks_env_values_test.go TestEnvValuesCheckReportsUnusableValue -->
<!-- test: internal/component/doctor/checks_env_values_test.go TestEnvValueDiagnosticWithholdsSecret -->

## Which check a new dependency needs

| New dependency | Doctor check |
|----------------|--------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel feature a configured subsystem cannot work without | A `kernelcap.Capability` registered from the owning package. `/proc/modules` cannot answer it: a built-in feature lists no module |
| Kernel module requirement with no configured subsystem behind it | `/proc/modules` check (Linux) |
| New listen address or port | Port bind probe |
| New UDP listener | UDP `ListenPacket` bind probe |
| New service with TLS | Certificate validity and expiry check |
| Embedded certificate material | Parse the certificate and check its validity window |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |
| Procfs or sysctl dependency | Read and write probe for the exact `/proc` path |
| Netlink dependency | Open the specific netlink family or handle |

## Where each owner registers its check

<!-- source: internal/core/diagnostic -- RegisterDoctorCheck -->

| Dependency owner | Registration mechanism |
|------------------|------------------------|
| Internal plugin (registered by `registry.Register`) | The `Registration.DoctorChecks` field. The doctor runner bridges these at execution time through `checks_plugin_registry.go`. The check function takes `registry.DoctorCheckContext` and returns `[]rpc.DoctorCheckDiagnostic`, and Component is set from the plugin name. `l2tpauthradius/register.go` is the reference example |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from the owning component's `init()` |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from the SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from the backend owner |
| Kernel module, procfs, sysctl, netlink, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from the owning backend or component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or a dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the absent owner explicit |

## The two tests a new check carries

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when its config block is present, and emits the registered code | The owning package beside the registration, or `internal/component/doctor` when there is no narrower owner |
| Functional test | `ze doctor --json config <file>` exposes the behavior through the user entry point | `internal/component/doctor`, or the existing functional suite for that entry point |
