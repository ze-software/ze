# Doctor and Runtime Health Checks

Ze has three tiers of self-diagnosis, and they differ by WHEN they answer.

The pre-config `store-recovery-pending` check reports retained full-store
recovery staging with `doctor-storage-recovery-pending`. It reads the same
retained directory authority used to refuse a second recovery source, names
the first unresolved directory and verifies its durable ownership record. A
proven source gets a complete quoted restore command with the exact target
`--path`. The command completes recovery only after all store owners stop.
<!-- source: internal/component/doctor/doctor_checks.go -- doctorOwnedChecks -->
<!-- source: internal/component/doctor/checks_storage_recovery.go -- doctorCheckRecoveryPending -->
<!-- source: internal/component/config/storage/daemon_recovery_manual.go -- PendingRecoveryCommand -->

An unproven or partially cleaned directory gets preservation instructions.
Store owners must stop, and the operator must verify the intended live store.
The operator then moves the whole directory outside its configuration directory,
preserves every byte and overwrites no destination. This resolves
admission debt and retains unknown material for owner inspection.
<!-- source: internal/component/doctor/checks_storage_recovery.go -- doctorCheckRecoveryPending -->
<!-- source: internal/component/config/storage/daemon_recovery_manual.go -- PendingRecoveryCommand -->

Diagnosis leaves all source bytes untouched. Failure to
inspect the actual store directory is an error finding rather than a healthy
answer. See [source-owned recovery](storage-backends.md) for the shared
ownership and import protocol.
<!-- source: internal/component/doctor/doctor_checks.go -- doctorOwnedChecks -->
<!-- source: internal/component/doctor/checks_storage_recovery.go -- doctorCheckRecoveryPending -->
<!-- source: internal/component/config/storage/daemon_recovery_manual.go -- PendingRecoveryCommand -->

| Tier | Command | When the fact is produced |
|------|---------|---------------------------|
| Readiness | `ze doctor`, and its daemon form `show doctor` | At read time, by re-probing config and the environment. `show doctor` runs the producer (`doctor.Answer`) inside the daemon, and adds the `runtime`-phase checks and the checks the running plugins declared; `ze doctor` asks the running daemon for that answer and draws it, and runs the config-phase checks in the operator's own process only when no daemon answers |
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

## BGP policy absences in the supplied configuration

The `bgp-policy-absence` post-config check uses the same peer walk and resolved
same-process grant graph as config validation. It reports an ordinary internal
recipient with no export policy under `doctor-bgp-learned-export-no-policy`:
Ze sends it no routes learned from other peers, while local announcements remain
permitted. An eligible explicit role and an unattached internal peer do not get
this finding. External missing directions retain their RFC8212 explanation under
`doctor-bgp-rfc8212-no-policy`.
<!-- source: internal/component/bgp/config/policy_absence_doctor.go -- doctorCheckBGPPolicyAbsences -->
<!-- source: internal/component/bgp/reactor/policy_absence.go -- PolicyAbsences -->

Each warning names the configured peer or group and the editor command that adds
its policy. A subsequent check over a supplied configuration with an explicit
accepting or rejecting policy clears the absence. The check clones the shared
saved tree before peer resolution. It reports that configuration, not live OPEN
state or external declarations; the Established warning reads the live captured
owner separately.
<!-- source: internal/component/bgp/config/policy_absence_doctor.go -- doctorCheckBGPPolicyAbsences -->
<!-- source: internal/component/bgp/config/doctor_checks.go -- bgpDoctorChecks -->
<!-- source: internal/component/bgp/reactor/policy_absence.go -- PolicyAbsences -->

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

The BGP RIB's `bgp-rib-selected-output` callback reads a failed selected-route
capture where that RIB runs. It declares `doctor-bgp-selected-output`, retains
the first cause, and names the configured instance's restart command as its
fix. The same code is raised through the SDK report bus after selection locks
are released. An acknowledged error raise is sent once; the runtime finding
continues to report the cause until the RIB restarts.
<!-- source: internal/component/bgp/plugins/rib/rib_advertise_diagnostic.go -- advertiseDoctorDeclaration, advertiseDoctorCheck, reportAdvertiseIssue -->

Alongside `bgp-rib-selected-output`, `bgp-rib` declares two warning-severity
runtime checks in Stage 1 `sdk.Registration.DoctorChecks` and answers them from
its `OnDoctorCheck` dispatcher (`doctorCheck`) in both internal and forked
placements:

- `bgp-rib-unresolved-next-hop` (`doctor-bgp-next-hop-unresolvable`, warning):
  reads the bounded per-RIB `unresolvedNextHopTracker`
  (`maxTrackedUnresolvedNextHops = 4096`), reports the count and bounded sample
  of routes currently excluded from Phase 2 by an unresolvable next hop with
  fix `ze show bgp rib | match next-hop-unresolvable`, and clears automatically
  on resolution, withdrawal, or peer teardown. Its code metadata is registered
  in `doctor_codes.go`.
- `bgp-rib-next-hop-metrics` (`doctor-bgp-next-hop-metrics`, warning): reads
  live `remoteRIB.metricFailureActive` state without issuing an engine RPC so
  `show doctor` reports an active forked `route-metrics` lookup failure
  directly alongside the SDK report bus (deduplicated with `recent-events` via
  `DoctorCheckDecl.Codes`). Its code metadata and event code remain
  single-registered in `register.go`.
<!-- source: internal/component/bgp/plugins/rib/rib_next_hop_doctor.go -- unresolvedNextHopDoctorDeclaration, nextHopMetricsDoctorDeclaration, doctorCheck, unresolvedNextHopDoctorCheck, nextHopMetricsDoctorCheck, unresolvedNextHopTracker -->
<!-- source: internal/component/bgp/plugins/rib/rib.go -- runRIBPlugin -->
<!-- source: internal/component/bgp/plugins/rib/doctor_codes.go -- codeDoctorBGPNextHopUnresolvable, registerDoctorCodes -->
<!-- source: internal/component/bgp/plugins/rib/register.go -- init -->

A `runtime` check only reads, and applies nothing. It is handed the platform
and the configuration doctor read: the tree the `post-config` phase of the same
answer judged, or no tree when that phase did not run (the configuration is
missing, does not parse, or the loader refused it), which the check reports as
what it cannot tell. That tree is the saved file the daemon route loads from
the store, never the running configuration. A runtime check uses it only to
agree with a config-phase check that read the same file, so the two do not both
report one fact, and it never reads the tree as what the daemon runs. Its
owner's live state is still read where it lives. The dns-source checks
(`doctor-dns-source-empty`, `doctor-dns-source-interface-silent`,
`doctor-dns-source-self-unserved`) are the first such checks, owned by
`internal/component/config/system`.

| In-tree runtime check | Owner | Reads | Reports | Code |
|-----------------------|-------|-------|---------|------|
| `ddos-flowspec-stopped-record` | `internal/plugins/ddos/flowspec` | the daemon's plugin snapshot, the store the daemon publishes, and the configuration doctor read | a recorded FlowSpec rule while the daemon holds no running `ddos-flowspec` process: the plugin configured and stopped, with the restart as fix; or no process of that name while that configuration names `ddos flowspec` or did not load, with `ze show config running` as fix and no withdrawal, because a plugin block of another name may hold the rule live. With the section absent from that configuration it is silent: the `post-config` check `ddos-flowspec-orphan-record` reports the record | `doctor-ddos-flowspec-orphan-announcement`, `doctor-storage-unavailable` |
<!-- source: internal/component/doctor/doctor.go -- runChecks -->
<!-- source: internal/component/doctor/answer.go -- RuntimeChecks, Answer -->
<!-- source: internal/plugins/ddos/flowspec/doctor.go -- stoppedRecordCheck, judgeStoppedRecord, unrunReport, orphanRecordCheck -->
<!-- test: internal/component/doctor/doctor_test.go TestRuntimeChecksReceiveTheConfigurationDoctorRead -->
<!-- test: internal/plugins/ddos/flowspec/doctor_stopped_record_test.go TestStoppedRecordCheckWithTheSectionUnrunOffersNoWithdrawal -->
<!-- test: internal/plugins/ddos/flowspec/doctor_stopped_record_test.go TestRecordChecksRegisterAtTheirPhases -->

Plugins declare these runtime checks in their Stage 1 registration and answer
them through their doctor callbacks, because the state they read lives in each
plugin's process:

| Check | Plugin | Reports | Code |
|-------|--------|---------|------|
| `bgp-rib-selected-output` | `bgp-rib` | a failed selected-route capture, source authorization, or destination output in that RIB process (error) | `doctor-bgp-selected-output` |
| `bgp-rib-unresolved-next-hop` | `bgp-rib` | stored BGP routes currently excluded from Phase 2 best-path selection by an unresolvable next hop, with excluded count, bounded sample, and `ze show bgp rib \| match next-hop-unresolvable` as fix (warning) | `doctor-bgp-next-hop-unresolvable` |
| `bgp-rib-next-hop-metrics` | `bgp-rib` | an active forked engine `route-metrics` lookup failure leaving affected next hops fail-closed until recovery (warning) | `doctor-bgp-next-hop-metrics` |
| `ldp-label-entries` | `ldp` | a push or pop LDP wants that no writer carried out | `doctor-fib-mpls-no-writer` |
| `rsvp-te-label-entries` | `rsvp-te` | an entry a writer refused on apply or a replay did not restore, until a later apply of the same slot succeeds | `doctor-fib-mpls-no-writer` |
| `ospf-sr-label-entries` | `ospf` | a Segment Routing push, swap or pop no writer carried out, or one OSPF cannot program because it runs forked | `doctor-fib-mpls-no-writer` |
| `bgp-epe-label-entries` | `bgp-epe` | a PeerNode SID label no writer installed or removed as EPE wants | `doctor-fib-mpls-no-writer` |
| `fib-vpp-label-entries` | `fib-vpp` | label entries installed without their path MTU; a label route a previous run left in VPP that could not be removed | `doctor-fib-vpp-mpls-path-mtu`, `doctor-fib-vpp-mpls-stale-label` |
| `fib-vpp-stale-routes` | `fib-vpp` | an IP route or labeled push a previous run left in VPP that could not be removed, IP tables that could not be read, or no FIB source to mark routes with | `doctor-fib-vpp-stale-route` |
| `fib-vpp-srv6` | `fib-vpp` | an SRv6 ownership reconciliation failure blocking mutations, or an SRv6 service route refused because durable daemon state is unavailable or VPP steering failed; silent while no SRv6 route is refused | `doctor-fib-vpp-srv6-refused` |
| `ipsec-state-scope` | `ike` | a missing, partial or unreadable complete state-scope receipt with configured IPsec peers, before a Child SA attempt; the finding retains the shared installation validator's cause and native kernel-build repair commands | `doctor-ipsec-state-scope` |
| `ipsec-spd-install` | `ike` | the SPD catch-all (`vpn ipsec unmatched`), or a bound XFRM interface's own final entry, the dataplane refused at start, so that default discard is not in force; an if_id 0 exemption of a bound XFRM interface the dataplane refused on the last install pass, so the traffic routed into that interface is dropped; an operator SPD entry the dataplane refused at start; a selector where a failed undo or removal left the dataplane disagreeing with the running configuration, with its direction, selector, priority and the dataplane's answer | `doctor-ipsec-spd-catch-all-missing`, `doctor-ipsec-spd-exemption-missing`, `doctor-ipsec-spd-install-refused`, `doctor-ipsec-spd-undo-incomplete` |
| `ipsec-source-guard` | `ike` | an IPv6 address of a bound XFRM interface whose tunnel-source guard the kernel refused, its source rule or the publish of the nft table `ze_ipsec_source_guard`, so its if_id 0 source exemption stays uninstalled and IPv6 host traffic from it routed into the interface is dropped at the DISCARD catch-all, with the interface, the address, the refused step, the cause the kernel's answer gives and that cause's fix | `doctor-ipsec-source-guard-missing` |
| `ipsec-spd-forward-bypass` | `ike` | an installed operator SPD bypass with a port, on a dataplane with a forward lookup (XFRM) whose kernel does not reassemble forwarded fragments first (no capability bit 3, or a receipt Ze could not read), whose local prefix is not one address the host holds now, so forwarded traffic on its ports misses it (RFC 4301 Section 7.4); a commit refuses such an entry and a start installs and logs it. Ze's runtime kernel gathers fragments, so the entry is projected and nothing is reported | `doctor-ipsec-spd-forward-bypass` |
| `ipsec-ike-socket-bypass` | `ike` | an IKE socket the engine holds whose per-socket IPsec bypass the kernel refused, with the socket and the kernel's answer | `doctor-ipsec-ike-socket-bypass` |

What each finding means, and the stale-label sweep behind the
`doctor-fib-vpp-mpls-stale-label` code, are in
`docs/architecture/mpls/mpls-kernel.md`, "Doctor checks". The stale IP route
sweep behind `doctor-fib-vpp-stale-route` is in
`docs/architecture/fib/fib-vpp-route-ownership.md`. The SRv6 refusal and
ownership-restore behavior behind `doctor-fib-vpp-srv6-refused` is in
`docs/architecture/fib/fib-depth-4-srv6.md`, "Ownership and restart".
<!-- source: internal/plugins/ldp/doctor_label.go -- labelEntriesCheckDecl -->
<!-- source: internal/plugins/rsvpte/doctor_label.go -- labelEntriesCheckName -->
<!-- source: internal/plugins/fib/vpp/doctor_label.go -- labelEntriesCheckDecl -->
<!-- source: internal/plugins/fib/vpp/doctor_route.go -- staleRoutesCheckDecl -->
<!-- source: internal/plugins/fib/vpp/doctor_srv6.go -- srv6CheckDecl, srv6DoctorFindings -->
<!-- source: internal/component/ike/engine/doctor_state_scope.go -- stateScopeCheckDecl, answerStateScope -->
<!-- source: internal/component/ike/engine/doctor_spd_install.go -- spdInstallCheckDecl -->
<!-- source: internal/component/ike/engine/doctor_source_guard.go -- sourceGuardCheckDecl -->
<!-- source: internal/component/ike/engine/doctor_ike_socket_bypass.go -- ikeSocketBypassCheckDecl -->
<!-- source: internal/component/ike/engine/doctor_runtime.go -- engineDoctorChecks -->

The `container` plugin, in a build with `ze_container`, declares five runtime
checks the same way. Each reads the problems its supervisor recorded for a
container, and its finding names the container and the command that fixes it:

| Check | Reports | Code |
|-------|---------|------|
| `container-out-of-memory` | a container the kernel killed for memory | `doctor-container-out-of-memory` |
| `container-process-limit` | a container that reached its process limit | `doctor-container-process-limit` |
| `container-stop-killed` | a container that outlived its stop timeout and was killed | `doctor-container-stop-killed` |
| `container-cleanup-incomplete` | a container that left processes, a control group or a mount behind | `doctor-container-cleanup-incomplete` |
| `container-failed` | an enabled container whose last start failed, as an error, with the step, its cause and the fix that refusal names | `doctor-container-failed` |

No code writes such a record yet, so the four answer no finding today
(`docs/architecture/container/runtime.md`, "Runtime checks").
<!-- source: internal/plugins/container/doctor.go -- runtimeDoctorChecks, supervisor.answerDoctorCheck -->

A sixth runtime check reads the image store rather than the supervisor's
records, through the store's side of the seam (`StoreChecker`):

| Check | Reports | Code |
|-------|---------|------|
| `container-store` | a store root under the data directory that cannot be used: no data directory, absent, a symbolic link, not a folder, owned by another user, or open to group or others, as an error, with the store's own fix | `doctor-container-store-unwritable` |

The check opens and checks the root and changes nothing. A store with no
folder of its own, such as the runtime's fixture store in a functional-test
build, answers no finding. The free-space reserve, `doctor-container-store-full`,
is not checked yet (`docs/architecture/container/image-store.md`).
<!-- source: internal/plugins/container/doctor_store.go -- storeDoctorChecks, answerStoreUsable -->
<!-- source: internal/plugins/container/imagestore/store.go -- CheckStore -->

The plugin's network package declares one more runtime check through the
plugin's sibling seam (`RegisterSiblingDoctorCheck`), answered in the plugin's
process from what its last apply recorded:

| Check | Reports | Code |
|-------|---------|------|
| `container-port-blocked` | a published port the operator's firewall policy blocks: a base chain at the forward hook has policy drop and no accepting term names the network's bridge as output interface or the container's address as destination, as a warning, with the accepting term as `set firewall` lines | `doctor-container-port-blocked` |

The check judges the committed firewall config and those two matches only
(`docs/architecture/container/network.md`, "The firewall policy and the
port-blocked check").
<!-- source: internal/plugins/container/network/policy.go -- portPolicy, answerPortBlocked -->
<!-- source: internal/plugins/container/sibling.go -- RegisterSiblingDoctorCheck -->

There is one phase set and one registry. A second registry of daemon-only
checks would let a check pick the one the local route happens to run, so the
phase is the only place that says when a check runs.

The system RIB declares `rib-routing-view` through its SDK registration. It
reports initialization, reconstruction and retained failure with
`doctor-rib-routing-view-unavailable`. A complete selected snapshot or local
authoritative scan becomes healthy after its forwarding output is acknowledged.
A reset clears readiness until reconstruction succeeds. This callback reads
the consumer's own monitor in both internal and external placement. Its fix
uses the configured instance name acknowledged by the daemon:
`ze request plugin restart <configured-name>`.
When a declared callback can no longer answer because its plugin stopped or
lost its connection, the daemon retains the `doctor-plugin-unreachable`
finding and its cause, with the same configured instance's restart command.
<!-- source: internal/component/sysrib/doctor_route_view.go -- routeViewDoctorDeclarations, findings -->
<!-- source: internal/component/sysrib/register.go -- runRouteViewPlugin -->
<!-- source: internal/component/doctor/cmd/show.go -- collectDoctorChecksFrom -->
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
daemon command does (`sshclient.DaemonAddress`), probes its listener with a
connect that reads no credential (`sshclient.ProbeDaemon`), logs in with
the credentials `ze init` stored (`sshclient.LoadCredentialsNoPrompt`, which
never prompts), and sends `show doctor | raw`. When the daemon answers, its
answer, runtime and plugin rows included, is drawn by the local renderer with
the local exit code, and `--json` stays a local drawing flag. A daemon
"answers" only when that ask returns a doctor result of this schema: a
process that exists but refuses the login has not answered. The first three
steps are one function, `sshclient.ReachDaemon`, which answers the step it
stopped at; `ze config validate` takes the same walk before it sends a config
to the daemon, and each command words its own fix per step.

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
<!-- source: internal/component/cli/sshclient/client.go -- DaemonAddress, ProbeDaemon -->
<!-- source: internal/component/cli/sshclient/reach.go -- ReachDaemon -->
<!-- test: internal/component/doctor/answer_test.go TestDoctorRuntimePhaseNotRunLocally -->
<!-- test: internal/component/doctor/answer_test.go TestDoctorLocalRouteAsksReachableDaemon -->
<!-- test: internal/component/doctor/answer_test.go TestDoctorLocalRouteNoCredentialsAnswersLocally -->

## Three severities

A finding carries one of three severities (`diagnostic.Severity`):

| Severity | Text tag | Readiness | Counted as |
|----------|----------|-----------|------------|
| `error` | `ERROR` | makes the box not ready, and `ze doctor` exits 1 | an error |
| `warning` | `WARN` | the box stays ready | a warning |
| `info` | `INFO` | the box stays ready | nothing: the summary line counts errors and warnings only |

An `info` row is a fact that is not a problem by itself: the checks that can
judge its impact report that impact under their own codes. A plugin's
doctor-check answer may carry any of the three; `show doctor` maps any other
severity to `warning`, and the text report draws and counts one that reaches it
anyway as a warning, so an unrecognized finding is never shown as less than a
problem. JSON carries the severity as written, under `severity`.

The daemon's capability inventory is the `info` row of the `daemon-capabilities`
runtime check, under `doctor-daemon-capability-missing`: the capabilities of
the daemon set the running daemon lacks, each with what it serves, or that Ze
cannot determine them. A config that uses nothing a missing capability serves
runs without it, so the row never warns. Its fix is
`privilege.CapabilityRemedy` for the capabilities it names, which offers the
systemd unit rewrite only when the daemon runs under `ze.service`
(`docs/architecture/system-architecture.md`, "Capabilities without root").

```
INFO  [doctor-daemon-capability-missing] The ze daemon runs without CAP_BPF (eBPF: IPsec ESP form fast path, IPsec override guard, traffic-usage). This is not a problem by itself: each configured feature that needs one checks for it and reports its own impact.
      fix: restart ze as root, or holding CAP_BPF: grant it as an ambient capability from whatever starts ze, or run sudo setcap cap_bpf+ep on the ze binary
```
<!-- source: internal/core/diagnostic/types.go -- SeverityError, SeverityWarning, SeverityInfo -->
<!-- source: internal/component/doctor/doctor.go -- outputText -->
<!-- source: internal/component/doctor/answer.go -- readyOf -->
<!-- source: internal/component/doctor/cmd/show.go -- normalizeSeverity -->
<!-- source: internal/component/doctor/checks_daemon_capabilities.go -- checkDaemonCapabilities, daemonCapabilitiesMissing, daemonCapabilitiesUnknown -->
<!-- test: internal/component/doctor/render_info_test.go TestRenderDoctorInfoIsNotAProblem -->
<!-- test: internal/component/doctor/checks_daemon_capabilities_test.go TestDaemonCapabilitiesCheck -->

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

## Persistent storage smaller than its disk

On an appliance the installer grows the persistent partition and its
filesystem to the disk. A system written to its disk another way, or whose
growth failed at install, keeps the size of the image. The `storage-grown`
check reports that as a warning under `doctor-storage-not-grown`, with the
device, the size in use and the size of the disk, and `request system storage
grow` as its fix. The installer logs a failed growth under the same code. It
links no component, so it spells the code and the fix itself, and
`TestInstallerSpellsTheStorageDoctorCode` compares both with the check's.

The check has three other answers. Storage that reaches its disk reports
nothing. A system with nothing mounted on the persistent mount point reports
nothing, because it has no storage to grow. A state that cannot be read is a
warning that carries the cause, with `show host storage` as its fix, because
storage that was not measured may be smaller than its disk.

The host component owns the state and reads it. It cannot register the check:
`internal/core/diagnostic` imports the host component for the platform type,
so the import back would be a cycle. The check is therefore registered by
`internal/component/doctor`, for the appliance platform only, in the
`pre-config` phase: it reads the disk and no config.

The size read is not written yet. `DetectPersistentStorage` reads the mount
table and answers that no persistent storage is mounted, or an error that
names the mounted device. On every appliance the check therefore reports the
warning for a state that cannot be read.
<!-- source: internal/component/doctor/checks_storage.go -- checkStorageGrown, readPersistentStorage, binarySize -->
<!-- source: internal/component/doctor/doctor_checks.go -- doctorOwnedChecks, doctorCheckStorageGrown -->
<!-- source: internal/component/host/storage_linux.go -- DetectPersistentStorage, persistentDevice -->
<!-- source: internal/component/host/inventory.go -- PersistentStorage, ErrNoPersistentStorage -->
<!-- source: internal/install/disk/run.go -- reportStorageNotGrown -->
<!-- test: internal/component/doctor/checks_storage_grown_test.go TestStorageDoctorReportsStorageSmallerThanDisk -->
<!-- test: internal/component/doctor/checks_storage_grown_test.go TestInstallerSpellsTheStorageDoctorCode -->
<!-- test: internal/component/host/storage_persistent_linux_test.go TestDetectPersistentStorageReadsTheMountTable -->

## Host resources that grow toward exhaustion

The host trend monitor (`docs/architecture/host/observability.md`, "The trend
monitor") lives in the daemon, so its checks take the `runtime` phase. The
package that registers them is `internal/component/host/trend`, not the host
component, for the import cycle the section above names.

| Runtime check | Registered by | Reports | Code |
|---------------|---------------|---------|------|
| `host-trend-monitor` | the monitor's own file | the monitor not running, with the restart for this process as fix (`trend.NotRunningFix`, worded by `privilege.RestartRemedy`: `sudo systemctl restart ze` only when ze runs under ze.service, otherwise `ze signal stop`, then a start the way ze was started), because the daemon's system start starts it; and each rule whose latest read failed, naming the path and the error, with `show host trend` | `doctor-host-trend-not-running`, `doctor-host-trend-unreadable` |
| `host-trend-kernel-memory` | the kernel-memory rule's file | the rule's standing warning, with the message and the fix the report bus holds; no row while the rule is quiet, warming up or not present | `doctor-host-kernel-memory-growth` |

Every row is a warning: the box still reads ready. Because each runtime check
declares the code it reports, `recent-events` skips those warnings
(`ownedCodes`) and adds no second row. A rule check answers nothing while the
monitor is not running, so the monitor check reports that once. A new rule
registers its own check from its own file, with `ruleCheck` as the function.
<!-- source: internal/component/host/trend/register.go -- checkMonitor, ruleCheck -->
<!-- source: internal/component/host/trend/register_kernel_memory.go -- the host-trend-kernel-memory registration -->
<!-- test: internal/component/host/trend/trend_doctor_linux_test.go TestTrendDoctorCheckRowFollowsRuleState -->
<!-- test: internal/component/host/trend/trend_doctor_test.go TestTrendMonitorNotRunningIsReported -->

## Kernel capabilities: one enrolment, three callers

Configured L2TP, kernel PPPoE and PPPoE clients inspect the same positive
[compiled original-resource PPP support](../research/l2tpv2-ze-integration.md).
The registered checks retain separate role paths. A capability declaration
does not grant authority over a channel, unit or transport.
<!-- source: internal/component/l2tp/ppp/kernel/original_attachment_linux.go -- originalAttachmentDoctorCheck -->

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
<!-- source: internal/plugins/fib/kernel/kernelcap_linux.go -- mpls, mpls-transit-mtu, srv6-encap, srv6-forward-ttl, srv6-forward-frag -->
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
| `wireguard` | `CONFIG_WIREGUARD` | a `wireguard` interface under the kernel backend | generic netlink family `wireguard` |
| `xfrm-interface` | `CONFIG_XFRM_INTERFACE` | an `xfrm` interface under the kernel backend | `RTM_NEWLINK` with `NLM_F_CREATE` for kind `xfrm` and interface id 0, inside a throwaway network namespace: `EINVAL` is `xfrmi_newlink` refusing id 0 after the kind resolved, `EOPNOTSUPP` is "Unknown device type" |

The `container` plugin enrols `container-cpu-limit`
(`CONFIG_CFS_BANDWIDTH`), in use when the configuration holds a container
instance whose `enabled` is not false. Its probe reads whether
`/proc/sys/kernel/sched_cfs_bandwidth_slice_us` exists. Its two codes register
from the plugin's package, never from the always-on code table, because a
build can leave the plugin out
(`docs/architecture/container/runtime.md`, "The kernel requirement").
<!-- source: internal/plugins/container/kernelcap_linux.go -- containerKernelCapabilities, containerInUse, cpuLimitProbe -->

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

`fib-kernel` also enrolls `srv6-encap` (`CONFIG_IPV6_SEG6_LWTUNNEL`),
`srv6-forward-ttl` (`CONFIG_IPV6_SEG6_FORWARD_TTL`), and `srv6-forward-frag`
(`CONFIG_IPV6_SEG6_FORWARD_FRAG`), all with `Degrades`.
`SRv6InUse` activates them when `fib { kernel { } }` is configured with at least
one BGP peer or peer group, because IBGP always accepts Prefix-SID attributes.
One shared behavioral probe (`kernelcap.SRv6Encap`, `kernelcap.SRv6ForwardTTL`,
`kernelcap.SRv6ForwardFrag`) answers them inside `InThrowawayNetworkNamespace`.
It remounts `/proc/sys` writable and sets `net.ipv4.ip_forward`,
`net.ipv6.conf.all.forwarding`, and `net.ipv4.conf.all.send_redirects`. It then
installs a seg6 encap route on a TUN device and inspects the inner TTL and
DF-clear IPv4 fragmentation behavior of forwarded packets. Missing
`/dev/net/tun` or `CAP_SYS_ADMIN` answers `unknown`, and a confining policy
refusal answers `denied`.

<!-- source: internal/plugins/fib/kernel/kernelcap_linux.go -- srv6EncapCapability, srv6ForwardTTLCapability, srv6ForwardFragCapability -->
<!-- source: internal/component/kernelcap/predicate.go -- SRv6InUse -->
<!-- source: internal/component/kernelcap/probe_srv6_linux.go -- SRv6Encap, SRv6ForwardTTL, SRv6ForwardFrag, askSRv6InThrowawayNamespace -->

A security policy confining ze can refuse those steps. Docker's
`docker-default` AppArmor profile does both: `deny mount,` refuses the private
mount namespace, and `deny @{PROC}/sys/[^k]** w,` refuses those sysctl writes.
A step that fails with `EACCES` while `/proc/self/attr/apparmor/current`
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
(`kernelcap.ProbeAppArmorProfile`). It takes `docker-default`, replaces
`deny mount,` with the three private `/proc/sys` mounts, and replaces the
`/proc/sys` write deny through `procSysWriteDeniesMulti`. That trie leaves only
`/proc/sys/net/mpls/platform_labels` and the three SRv6 forwarding sysctls
(`net.ipv4.ip_forward`, `net.ipv6.conf.all.forwarding`, and
`net.ipv4.conf.all.send_redirects`) writable. `stepFailedFor` names the failing
probe's sysctl paths in a `denied` reason. On a Docker host
`./le setup docker kernel apparmor` loads it
(`docs/architecture/testing/interop.md`, "The Docker host kernel check").

<!-- source: internal/component/kernelcap/policy_linux.go -- stepFailed, stepFailedFor, readConfiningPolicy, classifyLabel -->
<!-- source: internal/component/kernelcap/apparmor.go -- ProbeAppArmorProfile, procSysWriteDeniesMulti, probeSysctlKeepPaths -->

This is the `ze doctor` tier of the table above, not a fourth one. The verdict
is produced in the process that answers doctor (the daemon when it answers, the
operator's own process otherwise). It cannot move into the setup registry, whose
records are written before `main()` and therefore cannot see the configuration
this verdict depends on. Most probes evaluate at read time with no memory of a
start, because modules can load or sysctls can change. Probes for compiled-in
boolean kernel features (`srv6-encap`, `srv6-forward-ttl`, `srv6-forward-frag`)
memoize a definitive `present` or `absent` verdict for the process life
(`runSRv6ProbeMemoized`), because only a reboot can change it. `unknown` and
`denied` verdicts are never memoized.

<!-- source: internal/component/kernelcap/probe_srv6_linux.go -- runSRv6ProbeMemoized, isDefinitiveSRv6Verdict -->

`ProbeAll` asks the other question: does this kernel hold every enrolled
feature, whatever the configuration uses. It probes each enrolment with `InUse`
ignored and returns one `Row` per subsystem (subsystem, kernel symbol, state,
reason), and a probe that gave no verdict reads `unknown`. `ProbeSubsystems`
answers the same question narrowed to the exact subsystem names or
trailing-wildcard prefixes (`ipsec-esp-*`, `ipsec-transform-*`) that a caller
needs, refusing any selector that matches no enrolled capability so a typo
cannot silently skip a check. Full-host checks probe every enrolment; each
Docker interop suite and scenario probes only the capabilities it requires
(`docs/architecture/testing/interop.md`, "The Docker host kernel check"). The operator
reaches it as `ze doctor [--json] kernel-capabilities [subsystem <selector> ...]`,
which takes no config
file, prints the rows (as `{"ready":...,"capabilities":[...]}` under `--json`)
and exits 0 only when at least one capability is probed and every row is
`present`. An `unknown` or `denied` row is not a pass, and neither is an empty enrolment:
the owners enroll on Linux only, so off Linux the mode answers not ready.

<!-- source: internal/component/kernelcap/kernelcap.go -- ProbeAll, ProbeSubsystems, Row -->
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

The probes read netlink sockets or procfs. Neither executes an external binary: a second
dependency that can be absent for its own reasons is the fault this enrolment
removes, not a way to detect it. `TestCapabilityProbeExecsNoBinary` holds the
rule over the package's source.

### Four callers, one verdict

| Caller | Where | What it does |
|--------|-------|--------------|
| `ze doctor` | the registered check, run by `runChecks` | renders the diagnostic; `Run` already exits 1 on any `SeverityError` |
| daemon start | `runYANGConfig`, after `applyEvolutions` and before `EnsureActiveVersion` | one stderr line, one `logStartupFailure`, `return 1` |
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
second: it is a config-phase check, so it reads the limit of whichever process
answers doctor (the daemon when it answers, the operator's own process
otherwise), and the limit says nothing about whether the lock was taken.
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
only the daemon route runs it, handed the tree `post-config` judged, which
`runChecks` returns (see "The runtime phase: checks only the daemon runs"). The phase set is declared once, in
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
the state it reads. Config and kernel state read the same from both. A probe
whose answer depends on the asking process's privileges (a capability, an
rlimit, an execute permission, an eBPF load) does not: on the daemon route it
measures the daemon, the process that needs them, and on the local fallback
the operator's process; either way it takes a config phase, and its message
says which process it measured. Live daemon state exists only in the daemon,
so a check over it takes `runtime`: read from a config phase, the local route
finds it empty and reports the box healthy. State a plugin holds (its loaded
dataplane, its SA table, an apply outcome) is neither: the plugin may run
forked, so its check is a `runtime` check declared in its Stage 1 registration
and answered by its doctor callback, never a `registry.DoctorCheckDef`
(`ai/rules/plugins.md`). The IKE engine's `ipsec-esp-form-fast-path` is one: it
reads the dataplane `runEngine` loaded, which is nil in every other process.
`ipsec-child-install` is another: it reads the refusal each peer session
records (`recordChildRefused`), which exists only in the engine's process.
`ipsec-spd-install` is a third: it reports each operator `vpn ipsec policy`
entry the dataplane refused when the daemon started, which the start records
(`installSPDPoliciesAtStart`) instead of refusing the IPsec block. A reload
never leaves one behind, because a refused entry refuses the commit. It reports
first, under `doctor-ipsec-spd-catch-all-missing`, an SPD catch-all the start
could not install whole (`assertUnmatched`): the finding says the catch-all, under
`discard` the default discard, is not in force, with the dataplane's answer and
the restart for this process (`spdInstallFixRestart`, which is
`privilege.RestartRemedy`: `sudo systemctl restart ze` only under ze.service,
otherwise `ze signal stop` and a start the way ze was started, because
`ze signal restart` exits without starting a new process); the reload whose catch-all goes in clears it. Under `doctor-ipsec-spd-exemption-missing` it reports each bound XFRM
interface with an if_id 0 exemption the last install pass could not place
(`interfaceExemptionRefused`, rebuilt by every pass at start, reload or address
event), with the direction, the dataplane's answer and the same restart; the
pass that places the exemption clears it. No commit is refused for it, because
the entry fails closed. A source exemption whose tunnel-source guard the kernel
refused never reaches the dataplane, so it is not in that record: the
`ipsec-source-guard` check reports it under `doctor-ipsec-source-guard-missing`,
one finding per interface and address, from the guard's own record
(`sourceGuards.ruleRefused`, the last refused source rule, and
`publishRefused`, the last refused publish of the nft table, which leaves
unguarded each held address the table does not cover). The cause and fix come
from the classifier the Error line uses (`sourceRuleRequest`, `nftRequest`),
so the log and the doctor say the same thing; the pass that installs the step
clears it, and so does the prune that drops the address. The same
check reports, under `doctor-ipsec-spd-undo-incomplete`, each selector at which
a failed reconcile step left the dataplane disagreeing with the running
configuration: the undo of a refused install that could not restore or remove a
policy, or an accepted reload that could not remove an entry's previous form.
Each reconcile answers what it left selector by selector (`spdReconcile`), and
`recordSPDReconcile` folds it into the engine's record: a selector a later
reconcile settles is cleared, and one no later reconcile touches stays reported
until the same restart rebuilds the SPD. `ipsec-ike-socket-bypass` is a
fourth: it reports each IKE socket the engine holds whose per-socket IPsec
bypass the kernel refused, which the transport records when it opens the socket
(`exemptFromIPsec`, read through `BypassError`), with
`doctor-ipsec-ike-socket-bypass`, the cause and the fix for the kernel's
answer (`transport.BypassCause`, `transport.BypassFix`, which the socket's Warn
line names too): `EPERM` is a missing `CAP_NET_ADMIN`, fixed by
`privilege.CapabilityRemedy`; `ENOPROTOOPT` a kernel without `CONFIG_XFRM`;
`EACCES` a security module; any other answer is named as unclassified with both
fixes. Every restart in them is worded by `privilege.RestartRemedy`.
None of these checks probes the host: the TUN device outcome is recorded by the apply
(`recordESPFormTUN`), because opening `/dev/net/tun` may create the node, and
the fast path's readiness is likewise recorded at apply (`recordESPFormFastPath`)
so an idle receiver still reports a missing capability or absent TCX support.
`ipsec-application-override` is also a runtime check in `engineDoctorChecks`,
so `show doctor` reaches the engine's recorded override state and refusal in
internal and forked modes alike, while an offline local run never reads empty
engine state as healthy.
A bind probe MUST NOT judge a port
its own running daemon holds, because inside the daemon that bind fails on
every healthy box. The listener probe (`checkListeners`) does not tell the
routes apart yet: on the daemon route it reports each port the daemon serves
as a bind failure. `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`
(AC-1) owns that repair.
<!-- source: internal/component/doctor/doctor.go -- runChecks -->
<!-- source: internal/component/doctor/answer.go -- Answer -->
<!-- source: internal/component/ike/engine/doctor_runtime.go -- engineDoctorChecks, answerDoctorCheck, answerESPFormFastPath -->
<!-- source: internal/component/ike/engine/doctor_child_install.go -- childInstallCheckDecl, recordChildRefused -->
<!-- source: internal/component/ike/engine/doctor_spd_install.go -- spdInstallCheckDecl, answerSPDInstall, recordSPDReconcile -->
<!-- source: internal/component/ike/engine/doctor_source_guard.go -- sourceGuardCheckDecl, answerSourceGuard, sourceGuards.findings -->
<!-- source: internal/component/ike/engine/spd_policy.go -- installSPDPoliciesAtStart, reconcileSPDPolicies, spdReconcile, undoSPDInstall, removeReplacedSPDPolicies -->
<!-- source: internal/component/ike/engine/doctor_ike_socket_bypass.go -- ikeSocketBypassCheckDecl, answerIKESocketBypass, checkIKESocketBypass -->
<!-- source: internal/component/ike/transport/udp.go -- exemptFromIPsec, BypassError -->
<!-- test: internal/component/ike/engine/doctor_spd_diverged_test.go TestSPDPreviousFormNotRemovedIsReportedByDoctor -->
<!-- test: internal/component/ike/engine/doctor_spd_diverged_test.go TestSPDUndoNotTakenBackIsReportedByDoctor -->
<!-- test: internal/component/ike/engine/doctor_ike_socket_bypass_test.go TestIKESocketBypassRefusalIsReportedByDoctor -->
<!-- source: internal/component/ike/engine/apply.go -- recordESPFormTUN, recordESPFormFastPath -->
<!-- source: internal/component/ike/engine/doctor_application_override.go -- applicationOverrideCheckDecl, answerApplicationOverride -->
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

A probe that fails reports the refusal the daemon reports for the same bind,
built by the same producer (`network.ListenError`, the one the BGP listener
answers at start, at reload and when an address appears;
`docs/architecture/behavior/peer-lifecycle.md`, "What a refused listener tells
the operator"). The finding keeps its listener's code and severity, names the
address, port and service, and carries the kernel's answer as evidence and the
fix in its `Fix` field: for a permission refusal on a port below
`net.ipv4.ip_unprivileged_port_start`, `CAP_NET_BIND_SERVICE` and
`privilege.CapabilityRemedy`; for a port in use, the `ss` line that finds the
holder; for an address no interface holds, the address list. When the start
cannot be read, the finding says the port may need the capability rather than
asserting it. A permission refusal depends on the process that bound, so the
finding names the pid and uid the probe ran as, which is the daemon only when
the daemon answered the request. The probe still reads a port the running
daemon holds as a failure on the daemon route (above, and
`plan/release/validation/spec-doctor-coverage-11-hub-engine.md` AC-1).

<!-- source: internal/component/config/listener_defaults.go -- builtin listener defaults -->
<!-- source: internal/component/doctor/checks_listener.go -- collectSchemaListeners, listenerSetUnknown, listenerProbeFinding -->
<!-- source: internal/core/network/listen_error.go -- ListenError, IsPortPermissionRefusal -->
<!-- test: internal/component/doctor/checks_listener_test.go TestListenerProbeFindingNamesTheCapability -->
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
answer and runs the config-phase checks in the operator's own process only
when no daemon answers. That fallback runs the `registry.DoctorCheckDef`
functions of the plugins linked into `ze`, because `runPluginRegistryChecks`
reads the registry, and no check a plugin declared at Stage 1, because no
plugin runs there (see "The local
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
forwards one. A call through a function value (`seg.warn = r.warnWithheld`,
then `seg.warn(...)`) names no wrapper, so the line would be judged nowhere.
A wrapper under the site roots whose name is used other than as the callee
of a judged call (stored in a field, passed as an argument, returned, or used
at all from a file outside the site roots) escapes, and it is judged in two
places. Each direct call of it stays a site, judged for the code that call
passes, as for any wrapper. Its forwarding call is a site too, standing for
the calls made through the value. A forward passes its caller's attributes
on and spells no code of its own, so that site is always uncovered: it is
listed once in the report as an uncovered site, and a commit is refused for
it only when the commit carries the file that holds the forward and that file
held no such site before it. The gate judges only the files a commit carries,
so a commit that adds the escaping use in another file, without the
wrapper's, is admitted, and so is a commit that adds a direct call with a
declared code. dhcpserver's
`warnWithheld` escapes, and its call of `warnLimited` is a site. Names are
matched without types, so a field or a local of the same name also makes a
wrapper escape, which adds its forward as a site and never loses a line. A
wrapper outside the site roots is judged only at its callers, since its
forward is in no judged file. The forwards are
marked once every wrapper is found, so a wrapper's call of another found in a
later pass is a forward too, whatever the file and declaration order.
<!-- source: internal/le/arch/doctorcoverage/wrappers.go -- findWrappers, candidateOf, forwardsOf, passesOn, escapedRefs, holds, stringValued, stringName -->
<!-- test: internal/le/arch/doctorcoverage/judge_test.go TestGateSeesWrapperAndNonLiteralWarn -->
<!-- test: internal/le/arch/doctorcoverage/judge_test.go TestGateJudgesEscapingWrapperAtItsForward -->
<!-- test: internal/le/arch/doctorcoverage/judge_test.go TestGateMarksEveryForwardWhateverThePassOrder -->
<!-- test: internal/le/arch/doctorcoverage/report_test.go TestDoctorCoverageReportListsPluginFindings -->
<!-- test: internal/le/arch/doctorcoverage/judge_test.go TestDoctorCoverageCheckRefusesPluginFindingWithoutFix -->

A site is covered when its code is one that a doctor check declares. The
gate reads the covered codes from the same parsed source the sites come from,
never from the registries linked into `le`: every composite literal of a
doctor check declaration type (`diagnostic.DoctorCheck`,
`registry.DoctorCheckDef`, `rpc.DoctorCheckDecl` and its SDK alias,
`kernelcap.Capability`, whose check `kernel-capability-<Subsystem>` declares
its `CodeAbsent` and `CodeUnknown`, and `vpp.StartupRuntime`, whose check
`vpp-startup-<name>` declares its `Code`), whatever registers it, and every
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

Captured-session L2TP teardown raises `doctor-l2tp-control-send` when its control
message cannot be sent. The retained event names the actual peer and socket
failure, with `ze show errors` as the next command. Local teardown still ends
only the captured session; a stale capture performs no send and raises no event.
Peer receipt and native PPP cleanup remain separate outcomes.
<!-- source: internal/component/l2tp/diagnostic_control.go -- reportCapturedControlSendFailure -->
<!-- source: internal/component/l2tp/teardown.go -- teardownCapturedSession -->

An L2TP message with an AVP Ze could not reveal raises
`doctor-l2tp-hidden-avp-unrevealed` from `l2tp`, the subject the peer that sent
it, on the TunnelID=0 SCCRQ path and on an open tunnel alike. The fix follows
the reason: with no secret, configure `l2tp shared-secret` to the one the peer
hides AVPs under; on a length mismatch, check that the configured secret
matches the peer's; for a hidden Random Vector, or a hidden AVP with no Random
Vector before it, correct the peer, which violates RFC 2661 Section 4.3. The
warning line carries the code and the peer. A message on an open tunnel is not
rate-limited, so the event is raised at most once a minute per peer through
`report.RaiseErrorFixOnce`.
<!-- source: internal/component/l2tp/hidden.go -- hiddenLeft.log, hiddenRevealFix -->
<!-- source: internal/component/l2tp/register_hidden_diagnostic.go -- reportHiddenLeft -->
<!-- test: internal/component/l2tp/register_hidden_diagnostic_test.go TestHiddenLeftCarriesCodeAndRaisesEventForPeer -->

An L2TP call refused at `max-sessions`, or a tunnel refused at `max-tunnels`,
raises `doctor-l2tp-admission-limit` from `l2tp`, at most once a minute per
tunnel for sessions and once a minute for tunnels, through
`report.RaiseErrorFixOnce`, with `set l2tp max-sessions <count>` or
`set l2tp max-tunnels <count>` as the fix. The warning line carries the code.
A refusal whose slots are held by removed sessions with unconfirmed cleanup is
not this event: it raises a `doctor-ppp-cleanup` warning, which `show warnings`
lists beside the cleanup failures that hold the slots.
<!-- source: internal/component/l2tp/register_admission_diagnostic.go -- reportSessionLimit, reportTunnelLimit -->
<!-- source: internal/component/l2tp/session_retirement.go -- reportCleanupHeldSessionSlots, reportCleanupHeldTunnelSlots -->

A Framed-Route or Framed-IPv6-Route text in an Access-Accept that Ze cannot
parse is skipped, and the rest of the authorization applies. The skip raises
`doctor-radius-framed-route-skipped` from `l2tp-auth-radius`, the subject the
subscriber's user name, the message the attribute name and the parse error
only, and the fix `correct the subscriber's RADIUS profile`. A profile repeats
the bad text on every Access-Accept, so the event is raised at most once a
minute per subscriber.
<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- appendFramedRoutes, reportFramedRouteSkipped -->
<!-- source: internal/component/l2tp/plugins/authradius/register.go -- init -->
<!-- test: internal/component/l2tp/plugins/authradius/extract_route_diagnostic_test.go TestFramedRouteSkippedCarriesCodeAndRaisesEventForSubscriber -->

Shared CLI history publishes failed reads and writes as
`doctor-cli-history-read` and `doctor-cli-history-write`, from `cli-history` on
the fixed subject `command-recall`. Missing history raises nothing. Each class
raises at most once per minute across users and modes, using
`report.RaiseErrorFixOnce` and the existing error ring as its deduplication
ledger. A write that later succeeds still leaves the earlier event. The
messages classify the operation and mode without copying commands, usernames,
uploaded content or storage error values. The next diagnostic command is
`ze show doctor`, which includes the existing storage readiness checks.

An overdue web cleanup raises `doctor-web-cleanup-overdue` from `web-server`
on `web-shutdown`, with the same minute bound across concurrent shutdown calls.
Its command is `ze show log recent level err component web.server`. The
service keeps joining admitted requests and accepted history publications;
the diagnostic allowance never releases their borrowed store.
<!-- source: internal/component/cli/history_diagnostic.go -- reportHistoryReadFailure, reportHistoryWriteFailure -->
<!-- source: cmd/ze/hub/service_web.go -- webService.Shutdown -->

The browser request boundary reports a failed recall submission, including an
invalid receipt, as `doctor-web-history-unavailable`. A failed JSON delivery
uses `doctor-web-history-response`. Both come from `web-history`, on the fixed
subjects `command-recall` and `command-recall-response`. Each class raises at
most once per minute without commands, usernames or raw errors. Both name
`Open /cli and reconnect to restore command recall` as their next action.
These classes describe browser operations, separate from shared storage reads
and writes.
<!-- source: internal/component/web/register_cli_history_diagnostic.go -- reportBrowserHistoryUnavailable, reportBrowserHistoryResponseFailure -->

A failed network shutdown raises `doctor-web-network-cleanup` from `web-server`
on `web-network-cleanup`, before the service drains history. It names
`ze show log recent level err component web.server` and raises at most once
per minute. The service still joins history and returns both failures. Startup
cleanup and standalone shutdown rely on these constituent producers instead of
logging the joined error again.
The network grace's own expiration does not raise this event when forced
closure and request joins complete successfully. Unexpected cancellation,
transport failures and failed forced closure still raise it; route and history
cleanup failures retain their separate classifications.
<!-- source: cmd/ze/hub/register_web_history_diagnostic.go -- reportWebNetworkCleanupFailure -->
<!-- source: cmd/ze/hub/service_web.go -- webService.Shutdown, runWebOnly, startWebServer -->

An active warning whose code another `runtime` check declares in its own
`Codes` is that check's row and is not repeated here: the check reads the state
the warning mirrors, so one fault gives one row (`doctor-plugin-stopped` is
reported by `plugin-stopped`, not by `recent-events`). The same holds for a
running plugin's doctor check: `show doctor` asks the plugins first, and hands
the runtime phase the codes declared by each plugin check that answered, so a
plugin that raises a warning under a code its own check reports gives one row.
A plugin check whose callback failed reports no state, so its codes are not
handed on and the warning still renders here beside the
`doctor-plugin-unreachable` row. Error events always render, because they are
history no state check reports.
<!-- source: internal/component/doctor/checks_recent_events.go -- ownedCodes, warningsNoCheckOwns -->
<!-- source: internal/component/doctor/cmd/show.go -- answerDaemonDoctor, collectDoctorChecksFrom, pluginCheckAnswer -->
<!-- test: internal/component/doctor/checks_recent_events_test.go TestRecentEventsSkipsWarningAStateCheckOwns -->
<!-- test: internal/component/doctor/cmd/show_recent_events_test.go TestShowDoctorListsAPluginOwnedWarningOnce -->

### A problem owned by a retained resource

A daemon-owned resource can outlive the plugin process that reported its
failure. Its owner reserves one `report.ResourceWarning` slot before native
work starts. A full reservation set refuses that admission; an unrelated
warning cannot consume the reserved capacity later. The reservation and exact
failure episode remain in the daemon through process loss. Ordinary source
cleanup, warning clearance, same-key raises and warning-cap eviction cannot
delete or replace the original cause, subject, fix or first-raised time.

The owner retains one `ResourceWarningEpisode` before publishing. The episode
keeps bounded first and latest actual failure causes, its original fix and
first-raised time. `Update` changes only that same episode's latest cause.
Optional structured details preserve the original resource identity. Ze copies
the first map. Callers MUST keep mutable values inside that map unchanged.
Domain cleanup counts remain independent. Retrying its publication changes no
native state. Only its original resource's recovery proof or acknowledgement
permits `Clear`; the bus does not establish that domain fact. An old episode's
repeated clear never clears a later failure. An owner with no failure closes
its unused reservation without a synthetic raise or clear. This is
daemon-local custody; no resource pointer crosses the plugin protocol.
<!-- source: internal/core/report/resource_warning.go -- NewResourceWarning, ResourceWarning.NewEpisode, ResourceWarningEpisode.Publish, ResourceWarningEpisode.Update, ResourceWarningEpisode.Clear, ResourceWarning.Close -->


A plugin that declares `exit-confirms-cleanup` reserves its original process's
warning slot before native startup. Its terminal failure produces
`doctor-plugin-cleanup`, including the retained cause and actual exit outcome.
The host also captures a forked runner's structured final cause when log relay
is disabled. Incomplete stderr leaves cleanup unknown. Only that original
engine's confirmed zero return releases an unused reservation. A successful
replacement does not clear a failed original process's warning.
<!-- source: internal/component/plugin/process/terminal_cleanup.go -- AdmitTerminalCleanup, captureTerminalStderr, finishTerminalCleanup -->

### A full bus

The bus is bounded: `ze.report.warnings.max` active warnings, including
reserved resource slots, and
`ze.report.errors.max` error events, each clamped to 10000. Past the warning cap
`evictOldestWarning` drops the oldest ordinary warning in the remaining
capacity; reserved resource warnings stay owned. Past the error cap the ring
overwrites its oldest event. Both are counted, and `report.Losses` answers
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
degraded until the store is replaced. A plugin cannot drive the bus there on
its own: the `report-issue` route refuses a plugin's new warning past its
share of the cap (`rpc.ReportActiveWarningsShare`), so one plugin never evicts
another owner's warnings. A bus that never reached its caps reads as it
always did.
<!-- source: internal/core/report/report.go -- evictOldestWarning, mayHaveEvicted, Losses, evictionDoubt -->
<!-- source: internal/core/report/health_probe.go -- probe -->
<!-- source: internal/component/plugin/doctor/check_report_bus.go -- checkReportBusFull, busFullRow -->

### Event codes

The BGP RIB registers `doctor-bgp-selected-output` from its `init()` for
selected capture, source authorization and destination output failures raised
through `report-issue`. This event registration admits those reports on the
daemon's bus when the RIB runs internally or in another process. Its separate
Stage 1 `bgp-rib-selected-output` doctor declaration lets the daemon query the
RIB's live state; that declaration alone does not register a report-bus code.
<!-- source: internal/component/bgp/plugins/rib/register.go -- init -->
<!-- test: internal/component/bgp/plugins/rib/rib_selected_report_registration_test.go TestSelectedOutputReportRegistrationBothTransports -->

The BGP source authority service declares `doctor-bgp-source-lease-cleanup`
for a stopped selecting process or a permanently removed source whose revoked
final writes miss their join deadline. The event names the process or exact
source peer and gives `ze request restart` as the repair. Authority stays
revoked. A context-free source stop keeps its completion barrier closed until
the already-canceled guards finish; a report of overdue work is not a successful
join. The code is reported by the existing recent-events check.
The same process-generation barrier owns an unfinished enhanced-refresh Begin,
including an empty replay: it retires and joins that original Session. Successful
retirement adds no warning class. The existing code also reports a failure to
remove an ended selecting process's current output after retiring the captured
destination; that report gives `ze show bgp peers` as the next check.
<!-- source: internal/component/bgp/reactor/register_source_lease.go -- sourceLeaseCleanup, completeSourceLeaseRemoval -->
<!-- source: internal/component/bgp/reactor/register_source_lease.go -- sourceGenerationCleanup -->
<!-- source: internal/component/bgp/reactor/selected_cleanup.go -- retireSelectedOwner -->
<!-- source: internal/component/bgp/reactor/selected_refresh_owner.go -- joinSelectedRefreshClose -->

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

The plugin infrastructure's faults that had only a log line follow that rule.
Each line carries its code and its site raises an error event under it, with
`ze show errors` as the fix:

| Code | Raised by (bus source) | What happened |
|------|------------------------|---------------|
| `doctor-internal-defect` | the plugin server (`plugin-server`): a config transaction payload it could not decode or an ack it could not emit, an engine subscription or streaming registration with no handler or prefix, a typed event payload engine code emitted that did not decode (once per event), a config diff it could not marshal, a tier process missing from the process manager | a state only a defect produces |
| `doctor-internal-defect` | the plugin package (`plugin-registry`): a plugin event or send type refused at startup, a reactor the coordinator could not attach; the event bus (`event-bus`): a typed event dropped because its payload is not the registered type (once per subscription and payload type, as `AsString` does) | the same, raised under `report.CodeInternalDefect`, the bus's copy of the code for packages `internal/core/diagnostic` depends on |
| `doctor-internal-defect` | the managed client (`managed`, subject `hub client <name>`): a hub client built without access to the device's baseline, refused before it connects (`RunManagedClient`) | the device does not follow its hub until the defect is fixed |
| `doctor-plugin-request-refused` | `plugin-server`, subject the plugin: a command it dispatched failed, its `forward-cached` request named a malformed destination, a typed event it emitted carries a payload that does not decode as the registered type (the emit is refused and reaches nobody, whether its subscribers run in the daemon or forked), or its `report-issue` raises passed the rate or its share of active warnings. The config transaction coordinator also writes this code on the log line (it raises no event) for an acknowledgement it ignores: one a plugin sent twice, or sent for a transaction it takes no part in | the plugin did not get what it asked for; a plugin's own requests drive these events, so `refusePluginRequest` raises at most one per plugin in `rpc.ReportRaiseWindow`, reading the error ring as its ledger (`report.RaiseErrorFixOnce`); the log keeps a line for each refused emit, command and destination, and one per plugin in each report-issue rate window for each kind of `report-issue` refusal, whether or not that refusal raised the event |
| `doctor-plugin-answer-undelivered` | `plugin-server`: an answer, refusal or completion its caller never took, or a plugin answer that ended before its terminator | the caller waits or goes on without it |
| `doctor-plugin-drain-incomplete` | `plugin-server`: a drain round with a failed or late plugin, or one a signal aborted | queued work was dropped |
| `doctor-config-reload-refused` | `plugin-server`, subject `config-reload`: a configuration transaction that did not commit, the plugins a new configuration needs that could not be resolved or did not start, a configuration refused once they started, or a configuration that declares other external plugins than the ones ze started with (`RefuseChangedDeclarations`) | the reload is refused and ze keeps the configuration it was running; raised while ze restored that configuration, the event says the restore did not complete |
| `doctor-plugin-schema-changed` | `plugin-server`, subject the plugin, raised as a warning: an external plugin started again while ze ran (a respawn, `request plugin restart`) declared other YANG modules (config, command or rpc modules) than the ones boot published for it (`refuseChangedSchema`) | the plugin is refused at Stage 1 and is not run; the published modules, the running configuration and every other plugin are unchanged. The fix is `restart ze`, and a later start with the published modules clears the warning |
| `doctor-plugin-schema-undeclared` | `plugin-server`, subject the plugin, raised as a warning by boot between the early Stage 1 and the full parse (`ReportUndeclaredSchema`): a declared external plugin gave ze no declaration, or, beside a config load refused for a name no schema defines, a declaration with no YANG module (`SchemalessRefusal`) | a config section only that plugin's schema defines is refused as an unknown keyword and the start ends, with the code on the log line beside the refusal; boot writes each such plugin to the last-stop record (`RecordUndeclaredSchemaStop`), and the local `plugin-fatal-stop` check reports it under this code until a clean start deletes the record; with no such section the daemon starts and `ze show doctor` reports the warning, which is never cleared, because the schema ze holds changes only at a restart. The fix is the conformance check of the plugin's run line |
| `doctor-plugin-schema-refused` | `plugin-server`, subject `external-plugin-schema`, raised as an error by boot when the join of the declared modules refuses one (`ReportRefusedSchema`) | the start ends with exit 1, so the event never reaches a later `ze show doctor`: the report bus is in the process that exits. The log line and the refusal on the terminal carry the join's operator error and its fix. Boot also writes the refusal to the last-stop record (`RecordRefusedSchemaStop`), and the local `plugin-fatal-stop` check reports it under this code until a clean start deletes the record |
| `doctor-build-registration-unheld` | `plugin-server`, subject `ze-build`, raised as an error by boot before the first pass when a module compiled into ze names a `ze:validate` function or a `ze:reference` source this ze does not hold (`ReportUnheldRegistration`, called by `refuseWronglyBuilt`) | the start ends with exit 1. The log line and the refusal on the terminal name the module, the extension, the name and the schema path, and say the build is wrong and the config is not at fault. Boot writes the refusal to the last-stop record (`RecordUnheldRegistrationStop`), and the local `plugin-fatal-stop` check reports it under this code until a clean start deletes the record. A command with no daemon (`validatorFromResolved`) logs the same names under this code and goes on |

A refused reload is the one row whose fix is not always `ze show errors`. When
the refusal's error chain holds an operator error (`internal/core/opererr`),
the event carries that error's own fix: a plugin dependency this build does not
carry names the build that does. A plugin's verify or apply refusal reaches the
server as text over the plugin connection, so it carries no fix of its own, and
the event then names `ze show errors`. `ze config validate` is no fallback: it
takes a file path, reads only the file system, and never runs the live verify
callback of a running plugin, which is what refuses a reload. For the same
reason the transaction's line cannot tell a plugin's verify refusal from a
plugin that did not answer or an apply that rolled back, so it carries
`doctor-config-reload-refused`, never `config-plugin-verify`. A fatal plugin
whose startup the reload's tier or caller ended is left not running: the
startup phase records it through `RecordStopped`, so its line carries
`doctor-plugin-stopped` and raises nothing more. Every event message passes
through `report.FitMessage`, which cuts it to the bus's message bound, because
the bus refuses a longer message and the event would be lost.
<!-- source: internal/component/plugin/server/server_events.go -- raiseReloadRefused, reloadRefusalFix, reloadRefusalMessage, configReloadSubject -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- selectAutoLoad, startAutoLoaded, autoLoadRefusalProblem -->
<!-- source: internal/component/plugin/server/failure_policy.go -- stopDaemonOnStartupFailure -->
<!-- source: internal/core/report/report.go -- FitMessage -->
<!-- test: internal/component/plugin/server/reload_refused_events_test.go TestRefusedReloadRaisesEvent -->

The process manager raises one more error event of this kind, with a fix of
its own. `ProcessManager.Respawn` stops the process it replaces and waits for
it to end for `Process.StopJoinBound`, that process's exit grace plus one
second: 2s for a forked plugin, whose process group `superviseExit` kills 1s
after the stop, and 1s for a plugin that runs in the daemon. The wait is
`Process.Wait`, which joins every goroutine of the process: the engine and the
event delivery loop of a plugin in the daemon, and the reaper, the stderr
relay, the exit supervisor and the delivery loop of a forked one. When it
expires, the replacement starts anyway, and `recordRestartOverlap` logs it at
WARN and raises `doctor-plugin-restart-overlap` (`plugin` source), with the
plugin as its subject.

The message says what was left, from the process's `Running` value. The engine
clears `Running` when it returns, and the reaper clears it once the forked
process is reaped. An unfinished engine or a forked process that had not exited
can hold the same resources as the replacement.
<!-- source: internal/component/plugin/process/doctor_records.go -- recordRestartOverlap, restartOverlapText -->

Otherwise only the delivery loop was left, or for a forked plugin the delivery
loop or stderr relay. The relay reads until every holder of the plugin's stderr
closes it. Nothing distinguishes those two forked-plugin cases, so the message
claims no shared resources for them.
<!-- source: internal/component/plugin/process/doctor_records.go -- restartOverlapText -->
<!-- source: internal/component/plugin/process/process.go -- Wait -->

During this restart, ze takes no further step to end what is left. A second
restart is no fix, because it replaces the new process. Terminal shutdown
retains the outgoing physical owner for its bounded join before providers stop.
<!-- source: internal/component/plugin/process/manager.go -- Respawn -->
<!-- source: internal/component/plugin/process/manager_shutdown.go -- waitShutdownProcess -->

The fix is the ze command that shows what is left. For a plugin in the daemon,
`ze show system goroutines full` shows the unfinished engine or delivery-loop
stacks. For a forked plugin, `ze show log recent component plugin.relay` shows
what its processes still write to relayed stderr. Neither command names a PID.
A forked plugin's group ID is its reaped leader's PID. The kernel can already
have assigned that PID to another program.
<!-- source: internal/component/plugin/process/manager.go -- Respawn, respawnJoinMargin -->
<!-- source: internal/component/plugin/process/process.go -- StopJoinBound, Wait -->
<!-- source: internal/component/plugin/process/doctor_records.go -- recordRestartOverlap, restartOverlapText -->
<!-- test: internal/component/plugin/process/restart_overlap_test.go TestRespawnRecordsAnExpiredJoin -->
<!-- test: internal/component/plugin/process/restart_overlap_test.go TestRestartOverlapTextNamesWhatWasUnfinished -->

A stop with no replacement raises its own event. At boot the plugin manager
starts each external plugin the config declares before the full parse, and
stops one that sent no declaration (`FailDeclared`), one the parsed config no
longer names (`DropDeclaredExcept`) and the held plugins of a phase that was
refused (`takeDeclared`). Each stop waits `Process.StopJoinBound` in
`Manager.stopHeldLocked`. When the wait expires, `process.RecordStopUnfinished`
logs it at WARN and raises `doctor-plugin-stop-unfinished` (`plugin` source),
with the plugin as its subject, and the manager then removes the process. The
message is built from the same fact (`Running`) and carries the same two
fixes. A forked process that had not exited was already sent the kill for its
process group, 1s after the stop, and its reaper goes on waiting for it; the
message says it may still be running. No replacement exists, so the message
claims no shared resources.
<!-- source: internal/component/plugin/manager/manager.go -- stopHeldLocked, FailDeclared, DropDeclaredExcept, takeDeclared -->
<!-- source: internal/component/plugin/process/doctor_records.go -- RecordStopUnfinished, stopUnfinishedText -->
<!-- source: internal/component/plugin/process/register.go -- processEventCodes -->
<!-- test: internal/component/plugin/process/stop_unfinished_test.go TestRecordStopUnfinishedRaisesTheEvent -->
<!-- test: internal/component/plugin/process/stop_unfinished_test.go TestStopUnfinishedTextNamesWhatWasUnfinished -->
<!-- test: internal/component/plugin/manager/declared_test.go TestFailDeclaredRaisesAStopThatOutlivedItsBound -->

The operator's goroutine dump (`request quit`) reports nothing wrong, so it
logs at `Info`.
<!-- source: internal/component/plugin/server/server_events.go -- raiseServerEvent, serverEventSource -->
<!-- source: internal/component/plugin/resolve.go -- pluginEventSource, RegisterPluginEventTypes -->
<!-- source: internal/core/events/typed.go -- raiseDrop, eventBusSource -->
<!-- source: internal/component/plugin/server/dispatch_report.go -- refusePluginRequest -->
<!-- source: internal/core/report/report.go -- RaiseErrorFixOnce, raiseErrorOnce -->
<!-- source: internal/component/plugin/server/dispatch.go -- refuseUndecodedPayload, decodeTypedPayload -->
<!-- source: internal/core/report/report.go -- CodeInternalDefect -->
<!-- test: internal/component/plugin/server/server_events_test.go TestNotProblemSitesRaiseEvents -->

A plugin raises through `sdk.Plugin.RaiseIssue`, which sends `report-issue`
to the daemon, never through `internal/core/report` directly: a forked
plugin's own bus is a copy in its process that this check never reads. The
daemon raises it under the plugin's configured name with the fix the plugin
gave, and refuses a code that is not a registered event code, so a plugin can
raise only a code this check declares (`docs/architecture/api/process-protocol.md`,
"Plugin problems").
<!-- source: internal/component/plugin/server/dispatch_report.go -- opReportIssue -->
<!-- source: pkg/plugin/sdk/report.go -- RaiseIssue, ClearIssue -->

`RaiseIssue` waits for the daemon's answer, and a plugin meets its problems on
goroutines that must not wait: the BFD express loop, a Hello sender, code that
holds a lock every received packet takes. So a plugin raises and clears through
one report queue, `internal/component/plugin/issuequeue`, which its SDK entry
starts over the SDK plugin and stops on exit, after its engines have stopped so
their final clears are sent. A raise or a clear only enqueues: a full queue
(256 reports) or a stopped one refuses it at once, and the problem's WARN line
carries `report=queued` or the `report-error`. One worker per plugin sends each
report, bounded to one second, and hands a report the daemon refused to the
caller's callback, which logs it at WARN under the problem's own code. The stop
drains what is queued under one deadline of three seconds in all, so a plugin
stopping against a daemon that does not answer stops in seconds, and each
report the drain had no time for still reaches its callback. A condition that
raises once per run (an LDP or IS-IS Hello that cannot be sent) keeps a
`RunRaises` record of the run, so a raise the daemon refused is raised again at
a later failure, at most once per the plugin's warn window. BFD, OSPF, IS-IS
and LDP report this way.
<!-- source: internal/component/plugin/issuequeue/queue.go -- Queue, Start, Stop, Outcome -->
<!-- source: internal/component/plugin/issuequeue/raises.go -- RunRaises -->
<!-- test: internal/component/plugin/issuequeue/queue_test.go TestQueueStopBoundsTheDrainAgainstAHungDaemon -->
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

A runtime check is handed the platform and the configuration doctor read, and
`DoctorCheckContext` is shared core, so it carries no handle on the plugin
server. The server publishes
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
`fatal-policy` entry before it stops ze. `RecordStopCleanupLate` writes
`stop-cleanup-late` for an owner whose stop-time cleanup ran late.
<!-- source: internal/component/plugin/server/last_stop.go -- RecordLastStop, RecordStopCleanupLate -->

At final startup publication, a startup error writes `boot-refused` only if the
server lifetime is still live and the server has recorded no stop. A stop
already recorded during startup remains the cause, so one fault gives one
entry. If Stop was admitted or parent cancellation was observed before final
publication, completion skips last-stop settlement. Completion preserves an
earlier live refusal, or records the terminal error if no earlier refusal exists.
Stop or parent cancellation after final publication does not change the
historical startup result.
<!-- source: internal/component/plugin/server/last_stop.go -- settleLastStop -->
<!-- source: internal/component/plugin/server/startup.go -- finishStartupCompletion, recordStartupFailure, startupCompletionStoppedLocked -->

Each entry carries the reason, the plugin, a detail and the time. The record
names the run that wrote it (`Run`, the start time of the ze process). It keeps
that run's first 16 entries and counts the rest. A later run's first entry
replaces the previous run's record. A start that completes cleanly deletes the
record, unless that same run recorded a stop.
<!-- source: internal/component/plugin/server/last_stop.go -- LastStop, lastStopRun, recordLastStopIn, RecordLastStop, RecordStopCleanupLate, ReadLastStop, ReadLastStopFrom, settleLastStop -->

The daemon reads the record with `ReadLastStop`. The local checks' process
registers no statestore, so those checks use `ReadLastStopFrom` with the store
they are handed.
<!-- source: internal/component/plugin/server/last_stop.go -- ReadLastStop, ReadLastStopFrom -->

A terminal stop that cannot resolve plugin dependency order retains its first
failure on the process manager and raises `doctor-plugin-shutdown-order` as an
error event. This covers a missing resolver and a selected dependency cycle.
The server persists that same failure as `shutdown-order-failed` before its
store closes. Local doctor reports the reason after the stop, with the command
that reads its cause. A valid order, including an empty resolved graph, creates
no such record. This finding does not claim cleanup ran late or left installed
state behind.
<!-- source: internal/component/plugin/process/manager_shutdown.go -- RecordShutdownOrderFailure, ShutdownOrderFailure -->
<!-- source: internal/component/plugin/server/server.go -- cleanup -->
<!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- shutdownOrderRow -->

### Plugin lifecycle checks

Every path that leaves a configured plugin not running records it with the
process manager (`ProcessManager.RecordStopped`): its failure policy
(`ignore`, or none declared) after an exit, the respawn limit, a failed
restart, and a startup handshake that did not reach running. The record holds
the cause, the policy applied, the exit status (an internal plugin's runner
code, a fork's reaped state, or, when no exit was recorded within
`exitStatusWaitMax` of the connection closing, that the process had not
ended, a value of its own that no message calls an exit) and the detail, and
it raises `doctor-plugin-stopped` on the bus, so the `plugins` health row reads
down on every one of those paths. The detail is an error's text of any
length, so the bus message is fitted to the bus bound (`report.FitMessage`)
rather than refused; the doctor row prints it whole. A generation that reaches running clears it
(`superviseProcess`), and so does taking the process out of its slot
(`RemoveProcess`), after which a failed handshake records the plugin down
again. A reload's removal (`stopCollectedProcesses`) takes the process out of
its slot the same way, so a plugin a reload removed while it ran is not
reported down. Once a reload is accepted, `finishRemovalScope` calls
`ProcessManager.UnconfigureProcess` for each final removal (and
`rollbackStartupProcess` drops any process stopped below `StageRunning` during
a rolled-back reload start), so the removed name leaves `ConfiguredNames`:
neither `plugin-stopped`, `request plugin restart`, nor its completion offers
or accepts a plugin the configuration no longer runs. While a reload is still
in flight (or after a rejected reload recovers its removals), the name remains
configured until the scope settles. A plugin that was already down when a
reload removes its config roots or displaces its FIB selection is recorded on
the reload acceptance scope and dropped by `ProcessManager.UnconfigureProcess`
when the reload is accepted, clearing its configured entry, stop record, and
`doctor-plugin-stopped` row; a rejected reload preserves them. A candidate
plugin that a refused reload auto-loaded and whose startup failed at any stage
(below `StageRunning` or during generation readiness) is unconfigured
immediately without recording a stop, so neither `ConfiguredNames` nor
`doctor-plugin-stopped` retains it. `StartMore` replaces
the config of a name added back, so the configured set holds each plugin once.
External `bus > endpoint` declarations (and their schemas) are read at boot and
changes to them still require a restart (`RefuseChangedDeclarations`).
Compiled-in `bus > endpoint` declarations reload: `judgeReloadWriters` starts
and stops compiled-in `ProgramsFIB` endpoints as the candidate selection
changes, while leaving non-`ProgramsFIB` compiled-in endpoints untouched. While a process a restart stopped still
owes its runtime cleanup (`lateCleanupRecord`), every restart and every startup
phase naming the plugin is refused, and both the `doctor-plugin-stopped`
report-bus warning (`ze show warnings`, daemon log) and `ze show doctor`'s
`plugin-stopped` row say so from one shared declaration
(`process.StoppedMessage` / `process.StoppedFix`, reading
`PluginStop.CleanupOwed`): that cleanup had not
finished within the restart's bound, either because it waits for a call the
stopped process made into ze or because it is still running. Both keep
`ze request plugin restart <name>` alone as `Fix`, to be run again once that
cleanup has finished. Both name the last
resort as what it is: a restart of the ze daemon, which ends every call a
plugin made into it, worded for this process by `privilege.RestartRemedy`:
`sudo systemctl restart ze` only when ze runs under ze.service; otherwise
`ze signal stop`, then a start the way ze was started (`ze start`, or
`ze start <config-file>`), prefixed, when membership cannot be read, by a
sentence saying Ze could not tell. `ze signal restart` is never offered: it
writes the graceful-restart marker and exits without starting a new process,
so ze comes back only under a supervisor. Once every owed process finishes its
cleanup (`cleanupProcess` / `lateCleanupOwed`), `ClearStoppedCleanupOwed`
refreshes the bus warning to drop the cleanup-owed notice while keeping the
warning active until the plugin runs again.
`RemoveProcess` and `ReportCrash` also clear the ordinary warnings the plugin
raised through `report-issue` (`report.ClearSource` of its name), because only
the plugin could clear them. A warning reserved by a retained daemon resource
remains owned by that original resource until its recovery is confirmed.
<!-- source: internal/component/plugin/process/stopped.go -- RecordStopped, ClearStopped, SetStoppedCleanupOwed, ClearStoppedCleanupOwed, StoppedMessage, StoppedFix, DaemonRestartLastResort, RestartCommand, ExitStatus -->
<!-- source: internal/component/plugin/process/manager.go -- RemoveProcess, StartMore, ConfiguredNames, UnconfigureProcess -->
<!-- source: internal/component/plugin/server/failure_policy.go -- superviseProcess, applyFailurePolicy -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- collectProcessesForRemovedConfigPaths, stopCollectedProcesses -->
<!-- source: internal/component/plugin/server/startup_removal.go -- finishRemovalScope, rollbackStartupProcess -->
<!-- source: internal/component/plugin/server/reload_dataplane_writer.go -- judgeReloadWriters -->
<!-- source: internal/component/plugin/server/restart.go -- lateCleanupRecord, lateCleanupOwed -->
<!-- source: internal/component/plugin/doctor/checks_runtime.go -- pluginStoppedRow -->

| Check | Phase | Code | Reports | Fix |
|-------|-------|------|---------|-----|
| `plugin-stopped` | runtime | `doctor-plugin-stopped` (error) | each configured plugin recorded down, with the cause, policy, exit status and time | `ze request plugin restart <name>`; while a stopped process of it owes its cleanup, the same command once that cleanup has finished, with a restart of the ze daemon named in the row as the last resort |
| `recent-events` | runtime | `doctor-plugin-crashed` (event) | each unasked exit, with its exit status (or, for a plugin that closed its connection and whose process had not ended within the wait, that it had not ended) and the policy ze applied | `ze show errors` |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-fatal-stop` (error) | each `fatal-policy` or `boot-refused` entry of the last-stop record, with plugin, detail and time | the plugin's log lines before that time, then its config change; for an entry that names no plugin (a boot refusal other than a missing dependency), ze's log before that time, with no grep |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-schema-refused` (error) | each `schema-refused` entry of the last-stop record: the start ended because ze refused the YANG module an external plugin declared, with the time and the refusal's own text | the command the refusal named (the conformance check of the plugin's run line); ze's log before that time when it named none |
| `plugin-fatal-stop` | pre-config | `doctor-build-registration-unheld` (error) | each `build-registration-unheld` entry of the last-stop record: the start ended because a module compiled into ze names a registration this ze does not hold, with the time and the refusal's own text | the fix the refusal named (rebuild ze with the registering package); ze's log before that time when it named none |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-schema-undeclared` (error) | each `schema-undeclared` entry of the last-stop record: the start ended on a configuration name no schema defines while ze held no config schema of that declared external plugin | the command the refusal named (the conformance check of the plugin's run line) |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-shutdown-order` (warning) | each `shutdown-order-failed` entry: dependency ordering failed, so providers can have stopped before consumers completed | `journalctl -u ze` |
| `plugin-fatal-stop` | pre-config | `doctor-plugin-stop-cleanup-late` (warning) | each `stop-cleanup-late` entry of the last-stop record: the stop went on before that plugin finished its cleanup, so state it installs may remain | `journalctl -u ze --until <time> \| grep <plugin>` |
| `plugin-autoload` | post-config | `doctor-plugin-autoload-unresolved` (error) | the config's auto-load set that `registry.ResolveDependencies` refuses, with the boot refusal's text | the fix the refusal names |
| `plugin-setup` | pre-config | `doctor-plugin-setup-failed` (error for hard, warning for soft) | each `registry.SetupResults` failure, with the recorded reason | the remedy after the reason's last `; `, else `ze show plugin list` |
| `plugin-dataplane-writer` | post-config | `doctor-plugin-no-dataplane-writer` (error) | the config's auto-load set holds a `NeedsDataPlane` producer, no `ProgramsFIB` plugin is selected or configured, and `registry.JudgeDataPlaneWriter` refuses the data plane the config selects: the boot and reload refusal of `appendDataPlaneWriterWithReason` / `judgeReloadWriters`, in its words | the fix the refusal names |
| `fib-dummy-holds-routes` | post-config | `doctor-fib-dummy-holds-routes` (warning when any `NeedsDataPlane` producer is selected, naming each; info otherwise; silent without `fib dummy`) | `fib dummy` holds selected system RIB routes in memory and writes them to no forwarding table (named-table static routes, policy routing, IPsec tunnel-source guards, and interface addresses are outside the system RIB and untouched) | `delete fib dummy` |
| `recent-events` | runtime | `doctor-fib-dummy-invalid-change` (warning event, raised once via `RaiseIssue`) | `fib-dummy` received a `(system-rib, best-change)` entry carrying an invalid prefix, counted it in `invalid-changes`, and held it nowhere | `ze show fib dummy counters` |
| `plugin-removal-recovery` | runtime | `doctor-plugin-removal-unrecovered` (error) | each `pluginRemoval` whose restore after a refused reload failed (`recoveryErr`), with plugin and cause | `ze request reload`, which retries the restore first |
| `plugin-capability-restore` | runtime | `doctor-plugin-capabilities-not-restored` (error) | the latest failed `restoreCapabilities` after a refused reload, kept until a reload commits | `ze request reload` |
| `audit-record` | runtime | `doctor-audit-record-failed` (error) | the published audit log's last failed write (`audit.PublishedWriteFailure`): file, error, time, count, last surface; cleared by a write that lands | `ls -ld <dir> && df -h <dir>`, then make it writable |

Two command events go through `recent-events`: a parent an ensure-exists chain
created and could not delete again (`doctor-command-ensure-rollback-failed`,
error, the subject the parent's command path, the fix the delete command with
the same selectors), and the first use per session of a deprecated command
name (`doctor-command-deprecated-used`, warning, the fix the new name). The
`stop-cleanup-late` entries are written by `Server.stopTransaction` when the
config transaction does not unwind within its grace, and by `Server.cleanup`
for every engine `ProcessManager.CleanupLate` names.
<!-- source: internal/component/plugin/doctor/check_dataplane_writer.go -- checkDataPlaneWriter, dataPlaneWriterRows -->
<!-- source: internal/plugins/fib/dummy/doctor.go -- checkFIBDummyHoldsRoutes, holdsRoutesDoctorCheck -->
<!-- source: internal/plugins/fib/dummy/register_doctor_codes.go -- holdsRoutesCodeMeta, invalidChangeCodeMeta -->
<!-- source: internal/component/plugin/doctor/check_reload_records.go -- removalUnrecoveredRows, capabilitiesNotRestoredRows -->
<!-- source: internal/component/plugin/server/doctor_reload_records.go -- PluginReloadRecords -->
<!-- source: internal/component/plugin/server/command_events.go -- ensureRollbackFailed, deprecatedUsed -->
<!-- source: internal/component/plugin/server/stop_cleanup_late.go -- stopTransactionLate, recordLateEngines -->
<!-- source: internal/component/doctor/checks_audit.go -- checkAuditRecordFailed -->

`request plugin restart <name>` restarts a configured plugin, lifting a
respawn-limit disable because the operator asked for this restart. A running
plugin is stopped first through `Respawn`, which joins the process it replaces,
and its failure policy is not applied because the name already holds the
replacement; the restart holds the config transaction lock, so one in flight
refuses it, and an unknown name is refused. Every wait the restart makes under
that lock has a bound, the replacement's handshake included, and a wait that
runs out answers as a failed restart (`process-protocol.md`, the restart
bounds).

The command joins any deferred peer reconstruction and runtime
restoration when it releases that lock. It checks the exact replacement's
completed restoration, current slot and admission. A doctor record from an
older process cannot make a healthy replacement fail the command.

Tab after the command offers every
configured plugin, saying which run (`completePluginRestart`, served through
`system command complete args`; `docs/architecture/cli/command-completion.md`).
<!-- source: internal/component/plugin/server/register_plugin_restart.go -- handlePluginRestart, pluginRestartRefusal, completePluginRestart -->
<!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- checkPluginFatalStop, cleanupLateRow -->
<!-- source: internal/component/plugin/doctor/register_lifecycle.go -- init -->
<!-- test: internal/component/plugin/doctor/checks_runtime_test.go TestPluginStoppedEveryDownPath -->
<!-- test: internal/component/plugin/doctor/check_setup_test.go TestPluginSetupFailedReportsRecordedOutcome -->

### External plugin and managed fleet checks

Two checks judge the `plugin external` blocks the daemon cannot start, and six
runtime checks read what the managed config server recorded. The name check runs on the
parsed tree, before `ReadConfig`, because the commit now refuses a name of the
wrong form and the load would hide the block; a run that hands it no tree gets
a row saying the names were not checked, never the empty answer. The
in-process check reads `Registration.RunsInProcessOnly` of the registration
each external block resolves to. The managed server publishes its record when
`Start` applies its initial listeners (`ApplyManagedListeners`), and `Stop`
withdraws it; all six managed runtime checks read `ManagedRuntimeHealth`. With
no managed server running, `checkManagedListenerUnbound` still reports any
recorded `StartupFailure` (`doctor-managed-server-unavailable`), and all other
managed checks stay silent because no listener was asked for and no client was
served.
<!-- source: internal/component/plugin/doctor/register_external.go -- init -->
<!-- source: internal/component/plugin/doctor/check_external.go -- checkExternalPluginNames, checkExternalInProcess -->
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- checkManagedListenerUnbound, checkManagedClientRejected, checkManagedDeviceRecordUnwritable, checkManagedDeviceRecordUnreadable, checkManagedCertificateRefused, checkManagedHandshakeSaturated -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- ManagedRuntimeHealth, managedDoctorState, authRefused -->

| Check | Phase | Code | Reports | Fix |
|-------|-------|------|---------|-----|
| `plugin-external-name` | parsed-config | `doctor-plugin-connect-back-refused` (error) | each external block whose name is not of the plugin name form | rename the block, then `ze config validate` |
| `plugin-external-in-process` | post-config | `doctor-plugin-external-unsupported` (error) | each external block that runs a plugin declared in-process only | the `internal` block that runs it |
| `managed-listener-unbound` | runtime | `doctor-managed-listener-unbound` (error) | each managed address whose socket or certificate could not be prepared | release the occupied port, repair or remove the named certificate reference, or restore the issuing authority, as the row specifies |
| `managed-listener-unbound` | runtime | `doctor-managed-server-unavailable` (error) | the reason the hub could not construct the managed server or read its device registry at startup | follow the reported startup repair and restart the hub |
| `managed-client-rejected` | runtime | `doctor-managed-client-rejected` (warning) | each client whose last exchange was a refusal: a second session under a connected name, a config it refused to apply, a token that is not its secret, a name the hub has no client entry for, or a remote address whose connection ended before naming a client without a certificate-rejection TLS alert (including non-certificate fatal TLS alerts, TLS version/cipher mismatches, closed connections, and non-TLS bytes; up to 16 addresses kept for one hour, plus a forgotten-count row) | the cause's own fix |
| `managed-device-record-unwritable` | runtime | `doctor-managed-device-record-unwritable` (error) | each device whose latest history write failed, or a key for an unknown device that the hub could not remove, with its underlying store error | run `ze show data check`, correct the store error and reconnect the device to retry. For an unknown device, use `ze data rm <key>` |
| `managed-device-record-unreadable` | runtime | `doctor-managed-device-record-unreadable` (error) | each stored device record the hub could not read or decode, with the key and the cause | reconnect the device to replace its record, or use `ze data rm meta/fleet/device/<name>/record` to delete its history |
| `managed-certificate-refused` | runtime | `doctor-managed-certificate-refused` (warning) | each remote address whose TLS handshake ended with one of the six RFC 8446 / IANA certificate-rejection alerts (`bad_certificate`, `unsupported_certificate`, `certificate_revoked`, `certificate_expired`, `certificate_unknown`, `unknown_ca`), naming the address, count, first/last times, alert, presented certificate, and devices last seen from that address (up to 16 addresses kept for one hour, cleared on authentication, plus a forgotten-count row) | run `ze doctor` on the device at that address, or have the hub present a certificate its devices verify (`bus hub server <block> certificate`); `show fleet devices` lists every device not connected |
| `managed-handshake-saturated` | runtime | `doctor-managed-handshake-saturated` (warning) | that all 32 unauthenticated handshake slots on the managed listener were occupied at once, naming the last time, count, and remote addresses holding slots | check firewall rules and the listed addresses for stalled or abusive connections, and run `ze show fleet devices` |

The managed client has four checks of its own, and its hub one more, all
registered by `internal/component/managed`. The first four are post-config checks,
so they report with the daemon running or stopped; the fifth (`hub-session`) runs
in the runtime phase and reads the running daemon's hub client state. The first two read the
configuration tree. The third reads the store only: the managed flag, the hash
of the configuration the device runs, and the baseline. The fourth runs on a
hub and reads the copies the hub keeps of what holding clients run, and the
serving blocks of the tree. A client that holds is seen from its hub in that
fourth row once the hub keeps its copy; a hold with no copy (a configuration
over the bound, a copy that failed, a client built before the copy existed)
stays a config it refused to apply, in the `managed-client-rejected` row
above.
<!-- source: internal/component/managed/register.go -- init -->
<!-- source: internal/component/managed/doctor.go -- hubDoctorCheck, hubClientDoctorCheck, managedDivergedDoctorCheck, hubDivergedDoctorCheck, hubSessionDoctorCheck -->

| Check | Phase | Code | Reports | Fix |
|-------|-------|------|---------|-----|
| `hub-unreachable` | post-config | `doctor-hub-unreachable` (warning) | a configuration whose first hub `client` block names a hub that does not answer a TCP probe | start the hub at that address, or correct the `host` and `port` of the block on the hub |
| `hub-session` | runtime | `doctor-hub-certificate-unverified` (warning) | the running hub client refused the hub's certificate (unknown issuer, wrong host name, outside its validity dates, or another verification failure) or its configuration names a `pki ca` that `pki` does not define | chosen by cause and trust anchor (`ze init --managed --force --hub-ca <file>`, `pki ca <name>`, `bus hub server <block> certificate`, or renewing `pki certificate <name>`) |
| `hub-session` | runtime | `doctor-hub-handshake-failed` (warning) | the running hub client reached the hub's TCP port and the TLS handshake failed without a certificate verification error | `ze show doctor` on the hub at that address, or correct `host` and `port` |
| `hub-session` | runtime | `doctor-hub-auth-rejected` (warning) | the running hub client completed TLS and the hub refused its authentication | `ze show doctor` on the hub (`doctor-managed-client-rejected`), then correct the `client` entry on the hub or the device's secret |
| `hub-client-missing` | post-config | `doctor-hub-client-missing` (warning) | a managed client (`meta/instance/managed` is `true`) whose configuration has no hub `client` block, so it follows no hub | add the block on the hub, then `ze init --managed --force` |
| `managed-diverged` | post-config | `doctor-managed-diverged` (warning) | a managed client whose active configuration differs from its baseline (`meta/managed/base-version`), so it holds and applies nothing its hub sends; also a managed client whose store cannot answer one of the two values. Silent on a device that is not managed, on one with no baseline, and on one whose two hashes are equal | one line that names the hub commands over the device's name and the version it runs: `show fleet divergence name <client>`, then `update fleet divergence name <client> adopt version <hash>` to keep it or `request fleet divergence name <client> revert version <hash> hub-version <hub-version>` to discard it, the hub hash being the one the hub's show answers. For a configuration larger than the 1 MiB copy a hub takes, the two manual routes instead: give the hub the device's configuration, or discard it on the device (`docs/guide/fleet-config.md`, "When a Client Holds"). `ze show data check` when the store cannot answer |
| `managed-client-diverged` | post-config | `doctor-managed-client-diverged` (warning) | on a hub, each copy it keeps of the configuration a holding client runs (`meta/config/client-<name>.conf/held`), with the client, the copy's version, the version the hub serves it and when the copy was edited and received; a copy whose client no `plugin hub server` block declares is reported orphaned, because that client cannot connect. Also a copy, or the list of copies, the store cannot answer. Read from the store and the serving blocks, so it reports with the hub running or stopped, and after a hub restart | the three hub commands over the client and the copy's version, the revert also over the version the hub serves it. For an orphaned copy: declare the client again in a server block, or remove the copy with the hub stopped: `ze data rm meta/config/client-<name>.conf/held`. `ze show data check` when the store cannot answer |
<!-- source: internal/component/managed/doctor.go -- checkHubReachable, checkHubClientDeclared, checkManagedDiverged, diagnoseManagedUncompared, managedDivergedFix, managedDivergedManualFix, checkHubClientsDiverged, diagnoseHeldClient, diagnoseHeldOrphan -->
<!-- source: internal/component/plugin/server/managed_held.go -- HeldClients, ReadHeldRecord, HeldRecordKey -->
<!-- test: internal/component/managed/doctor_test.go TestDivergedFixNamesHubCommands -->
<!-- test: internal/component/managed/doctor_test.go TestHubDivergedCheckDeclared -->
<!-- test: internal/component/managed/doctor_test.go TestHubDivergedCheckOrphan -->
<!-- source: internal/component/managed/baseline.go -- ManagedFlag, ActiveVersion, BaselineVersion, Diverged -->
<!-- test: internal/component/managed/doctor_test.go TestDivergedDoctorCheck -->
<!-- test: internal/component/managed/doctor_test.go TestDivergedDoctorSilentWhenUnmanaged -->

The managed server authenticates with per-client secrets and no shared
secret. `AuthenticateWithLookup` refuses an empty expected token, so a name
with no client entry is refused whatever token it sends; before that guard, an
unknown name that sent an empty token authenticated. The refusal is recorded
under the claimed name and never with the token. A name of the plugin name
form is shown unchanged. Any other name is escaped the way `strconv.Quote`
escapes it (control characters, non-printable and bidirectional runes,
invalid bytes, the quote mark and the backslash), without the outer quote
marks, and cut to 64 octets of escaped text with `...` after the cut. An empty
claim is shown as `""`. A
configured name is recorded beside the other refusals of that client; an
unknown name is chosen by whoever connects, so only the 16 most recent are
kept.
<!-- source: internal/component/plugin/ipc/tls.go -- AuthenticateWithLookup -->
<!-- test: internal/component/plugin/server/managed_auth_refused_test.go TestManagedUnknownNameEmptyTokenRefused -->

A refused plugin connect-back is an event, not a state: the acceptor logs it
at WARN and raises `doctor-plugin-connect-back-refused` on the report bus,
where `recent-events` renders it, naming the claimed name (or the remote
address when the request named none) and never the token.
<!-- source: internal/component/plugin/ipc/connect_back_refusal.go -- reportConnectBackRefused -->

### Plugin registration and delivery checks

Nine runtime checks read what a plugin was refused or never received after it
started. Each per-plugin record lives on the plugin's `process.Process`
(`DoctorRecord`, bounded at 16 entries per list, the rest counted), so a
respawn or restart starts clean; `PluginDoctorState.Records` carries the copy.
Each string a record keeps that the plugin chose is bounded at the recorder
(`boundDoctorText`): a declared command or alias and a subscribed namespace at
128 octets, the bound of a declared doctor check name, and a refusal reason,
a parse detail and every kept error at 1024, the bound the managed config
server keeps of a client's error. A longer one is cut at a UTF-8 rune
boundary and ends `...(cut)`, so a row never presents a cut name as whole.
What has no process to live on is on the server: the recovered engine handler
panics, the enricher failures per plugin name (cleared with the plugin's
process cleanup and again when its next process registers its enrichers, and
recorded only from a call through the connection the plugin's enrichers are
registered on, so a late failure of a replaced or ended process is never
kept, whether or not its replacement declares enrichers) and the
daemon's last committed transaction. The server
lists are bounded at 16 too. Each bounded list carries its count of the rest
into the snapshot (`EnrichersDropped`, and the `...Dropped` fields of
`DoctorRecord`), and its check adds one row naming that count. Each entry
past the bound was only logged by the plugin server, with its code, and never
raised on the report bus, so the row's fix is the recent log of that
subsystem at the entry's level, `ze show log recent level warn component
plugin.server` (`level err` for the handler panics), which lists them while
the log buffer holds them; `ze show warnings` and `ze show errors` read only
the bus and would list none. A failing
enricher past the bound is counted once however often it fails, and leaves the
count when a call succeeds. An engine handler has no process a restart
replaces, so its panics are kept the way delivery problems are: the 16 newest,
reported while inside `HandlerPanicWindow` (five minutes), with
`HandlerPanicsTruncated` and one more row when the ring overwrote a panic still
inside it. The snapshot
also carries every finding of `deliveryDisagreements(nil)` and every role
`unbackedClaims` returns, with its claimants. With no server published each
check answers one warning saying it could not read the plugin state.
<!-- source: internal/component/plugin/doctor/register_delivery.go -- init -->
<!-- source: internal/component/plugin/doctor/checks_delivery.go -- snapshotCheck -->
<!-- source: internal/component/plugin/process/doctor_records.go -- DoctorRecord, noteBatch, boundDoctorText -->
<!-- source: internal/component/plugin/server/doctor_delivery.go -- registerProcessCommands, recordHandlerPanic, recordEnricherFailure -->
<!-- source: internal/component/plugin/server/doctor_snapshot.go -- DeliveryDisagreement, UnbackedClaim -->
<!-- test: internal/component/plugin/doctor/checks_delivery_test.go TestDoctorClassChecks -->

| Check | Code | Reports | Recorded by | Fix |
|-------|------|---------|-------------|-----|
| `plugin-commands` | `doctor-plugin-command-not-registered` (warning) | each command or deprecated alias the registry refused, with the plugin that owns the name, or `builtin` | `registerProcessCommands` | remove or rename one of the two plugins, or the plugin that shadows a builtin |
| `plugin-subscriptions` | `doctor-plugin-subscription-dropped` (error) | each dropped subscription block: unknown namespace, no default namespace, or ready parameters that did not parse | `resolveSubscriptionNamespace`, the sinks' `readyParamsRefused` | a plugin version that subscribes in a registered namespace, then the restart |
| `plugin-delivery-agreement` | `doctor-plugin-delivery-disagreement` (warning) | each finding of `deliveryDisagreements(nil)` | computed at the snapshot | the `bus` filter change (`DeliveryDisagreement.Fix`) |
| `plugin-claims` | `doctor-plugin-claim-unbacked` (error) | each exclusive role with no claimant running, naming the claimants | computed at the snapshot | `ze request plugin restart <claimant>` |
| `plugin-poststartup` | `doctor-plugin-poststartup-undelivered` (error) | each running plugin whose post-startup callback failed; one still starting, or whose callback is in flight, is not reported | `recordPostStartupOutcome` | `ze request plugin restart <name>` |
| `plugin-panic` | `doctor-plugin-panic` (error) | a plugin's first failed event application (`projectionErr`), and each recovered engine handler panic with its namespace and event type | `deliverBatch`, `safeDeliverBatch`, `invokeHandler` | the restart, or `ze show log recent level err component plugin.server` for an engine handler, whose panics are logged and not raised on the report bus |
| `plugin-delivery` | `doctor-plugin-delivery-failed` (warning) | the batches that failed with no caller waiting, and the batches over 500 ms, in the last five minutes (`DeliveryWindow`), with the last error of a failure it counts (a slow batch whose error went to a waiting caller keeps none) | `noteBatch`, timed in `deliverBatch` for every transport | `ze request plugin restart <name>` |
| `plugin-enrichers` | `doctor-plugin-enricher-unavailable` (warning) | each `show` enricher that failed to register, or whose last call failed | `registerProxyEnrichers`, `makeProxyCall` | remove the plugin holding the same field, or the restart |
| `plugin-config-commit` | `doctor-plugin-config-unapplied` (warning) | each running plugin whose last config-committed callback failed or had no connection to go on, with the daemon's last committed transaction | `dispatchCommitted`, `subscribeCommitted` | `ze request plugin restart <name>` |

A batch of exactly 500 ms is not slow; one of 501 ms is (`DeliverySlow`). The
window keeps the 64 newest problems and counts only those inside it, so a
failure at boot stops counting five minutes later. While a problem the ring
overwrote would still be inside the window, the summary is `Truncated` and the
row says "at least" before each count. A stopped stderr relay is an
event, not a state: the relay logs it at ERROR and raises
`doctor-plugin-stderr-relay-stopped` naming the plugin and the cause, with the
restart as its fix; the end where a descendant holds stderr after the plugin
exited stays at Debug.
<!-- source: internal/component/plugin/process/doctor_records.go -- DeliverySlow, raiseRelayStopped, noteBatch -->
<!-- test: internal/component/plugin/process/doctor_records_test.go TestDeliverySlowBoundary -->
<!-- test: internal/component/plugin/process/doctor_records_bounds_test.go TestDeliveryWindowSaysWhenTheRingOverwrote -->

A `plugin-config-commit` record clears when the plugin is told of a later
transaction (`RecordConfigCommit` replaces it) or when a restart replaces its
process. A reload is no fix: `reloadConfig` answers "no changes" for an
unchanged config before any transaction starts, and a changed one tells only
the plugins whose roots it changes. The restart runs the startup handshake on
the replacement, whose Stage 2 delivers the daemon's config.
<!-- source: internal/component/plugin/server/reload.go -- reloadConfig -->
<!-- source: internal/component/plugin/server/config_tx_bridge.go -- dispatchCommitted, committedTarget -->
<!-- test: internal/component/plugin/server/doctor_delivery_bounds_test.go TestConfigUnappliedClearsOnLaterCommitNotOnUnchangedReload -->
<!-- test: internal/component/plugin/server/doctor_delivery_bounds_test.go TestRestartClearsConfigUnappliedRecord -->

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

The BGP source-session issuer registers `bgp-source-receipt-issuer` in the
daemon's runtime phase. It reads the daemon-wide identity clock and reports
`doctor-bgp-source-receipt-admission` if no new identity can be allocated.
Restarting the BGP subsystem cannot repair that clock; the fix is
`ze request restart`. A failure to capture a session's negotiated facts raises
the same owned code with its precise cause on the bounded recent-events bus and
closes that connection before UP is published. A healthy issuer returns no
runtime finding.
<!-- source: internal/component/bgp/reactor/source_session_diagnostic.go -- sourceReceiptDiagnostics, reportSourceReceiptAdmissionFailure -->

BGP's `bgp-transport-scopes` check reads the host interface names and indices
when the configuration has a BGP block. Its
`doctor-bgp-transport-scope-unknown` warning means differently spelled TCP
interface zones cannot safely be equated. Runtime comparison refuses that alias
and raises the same code on the report bus until an interface read succeeds.
Scoped peer startup and live admission also refuse when this read fails, because
their physical duplicate/self checks cannot establish a safe binding identity.
The configured addresses and peer keys remain intact. The warning names the
host read failure and offers `ze show interface brief` as the next inspection.
<!-- source: internal/component/bgp/reactor/register_transport_scope.go -- doctorTransportScopes, transportScopeDiagnostic -->
<!-- source: internal/component/bgp/reactor/transport_scope.go -- sameTransportAddress, readTransportInterfaces, validatePeerTransportsLocked -->

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
| Netlink socket dependency | Open the specific netlink family or handle |

## Where each owner registers its check

<!-- source: internal/core/diagnostic -- RegisterDoctorCheck -->

| Dependency owner | Registration mechanism |
|------------------|------------------------|
| Internal plugin (registered by `registry.Register`), check over config or the host | The `Registration.DoctorChecks` field, at a config phase only. The doctor runner bridges these at execution time through `checks_plugin_registry.go`, in whichever process answers doctor. The check function takes `registry.DoctorCheckContext` and returns `[]rpc.DoctorCheckDiagnostic`, and Component is set from the plugin name. `l2tpauthradius/register.go` is the reference example |
| Any plugin, internal or forked, check over state its own process holds | A `runtime` `rpc.DoctorCheckDecl` in the `DoctorChecks` of the `sdk.Registration` it sends at Stage 1, answered by its `OnDoctorCheck` handler. `ldp/register.go` and `ike/engine/doctor_runtime.go` are reference examples |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from the owning component's `init()` |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from the SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from the backend owner |
| Kernel module, procfs, sysctl, netlink sockets, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from the owning backend or component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or a dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the absent owner explicit |

## The two tests a new check carries

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when its config block is present, and emits the registered code | The owning package beside the registration, or `internal/component/doctor` when there is no narrower owner |
| Functional test | `ze doctor --json config <file>` exposes the behavior through the user entry point | `internal/component/doctor`, or the existing functional suite for that entry point |


BGP identifier forwarding records `doctor-bgp-forward-path-origin` in the bounded
recent-event ring when an original source receipt, stable owner, physical
connection or SourceID does not resolve. The refused operation guesses no source
facts. The event retains its exact cause and `ze request restart` recovery
command, is rate-limited per peer, and leaves no active warning after recovery.
<!-- source: internal/component/bgp/reactor/forward_path_origin_diagnostic.go -- reportForwardPathOriginFailure -->

The reflector declares the runtime `bgp-rr-replay` check under
`doctor-bgp-rr-replay`. It reads current original-source/destination output,
peer replay and validation obligations where the reflector runs. An optional
page-read refusal is a warning and is also raised through the SDK report bus;
required unresolved work is an error. A successful read clears the current
optional finding, and exact ended-source lease settlement clears only the
matching source output finding. No doctor read applies a repair or confers
forwarding authority.
<!-- source: internal/component/bgp/plugins/rr/replay_diagnostic.go -- replayDoctorCheck, reportOptionalReplay -->


The route server declares `bgp-rs-lifecycle` under `doctor-bgp-rs-lifecycle`.
Its runtime callback reads the existing original-source replay failures,
unfinished source-lease releases, unknown captured outputs and failed settlement
where that plugin runs. It never retries or clears work. The corresponding
status command is `ze show bgp rs status`.

Refused RS input and lifecycle operations also enter the daemon's report bus
through one bounded SDK issue queue, on internal and forked transports. One
transient event per report window bounds repeated refusal traffic; each log
keeps its original cause and says whether reporting was queued, rate-limited
or refused. The worker starts before the RS engines and drains only after they
stop, before the SDK connection closes. No SDK request runs under an RS lock.
The reflector's replay code is likewise registered as a transient event code,
so the daemon accepts its existing SDK reports rather than refusing a code
that has only descriptive metadata.
<!-- source: internal/component/bgp/plugins/rs/lifecycle_diagnostic.go -- lifecycleDoctorDeclaration, lifecycleDoctorCheck, warnLifecycle -->
<!-- source: internal/component/bgp/plugins/rs/server.go -- RunRouteServer, routeServerRegistration -->
<!-- source: internal/component/bgp/plugins/rr/replay_diagnostic.go -- init -->
