# Handover 56: each feature reports its own missing capability

Written 2026-11-22 by the session that started the work on the server
(`claude`), in the main checkout. Thomas paused it. Nothing of it is
committed; every change below is in the working tree of the main checkout.
No spec was written: Thomas ordered the fix directly and gave its design in
his order (`ai/META-DECISIONS.md`, "How agents work": a fix he ordered is done
directly).

## Thomas's order (verbatim)

> Implement and verify a fix for Ze's misleading doctor-daemon-capability-missing
> warning. Follow the repository's AGENTS.md.
>
> Start with warnDaemonPrivileges in cmd/ze/hub/main.go, CheckPrivileges in
> internal/core/privilege/check_linux.go, and checkDaemonCapabilities in
> internal/component/doctor/checks_daemon_capabilities.go. Ze currently checks
> its full capability list before reading the config. An interfaces-and-SSH
> config starts successfully with only CAP_NET_ADMIN, yet Ze logs a warning and
> tells the operator to reinstall the systemd unit.
>
> Move the generic capability inventory to Debug and keep it informational in
> show doctor. Use existing subsystem checks to report actual impact. Check
> configured features at startup and reload, and check on-demand operations
> when invoked. A BGP bind failure should name the configured address and port.
> Check eBPF readiness only for enabled features that need it. Preserve ping's
> unprivileged socket fallback and traceroute's raw-socket diagnostic. If a
> check can't determine readiness, say so. Offer a systemd repair command only
> when Ze can establish that the affected process belongs to the Ze service.
>
> Build the updated binary through the repo's ./le workflow. Verify both cases:
> the interfaces-and-SSH config starts without a capability warning and show
> host trend succeeds, while an enabled feature that genuinely lacks a required
> privilege still reports the feature, failed operation, and remedy. Keep
> etc/ze/claude-ssh.conf local and untracked.

Manual check he gave: stop the foreground daemon, start it with
`sudo setpriv --reuid=thomas --regid="$(id -g thomas)" --init-groups --reset-env --bounding-set=-all,+net_admin --inh-caps=-all,+net_admin --ambient-caps=-all,+net_admin -- ./ze start etc/ze/claude-ssh.conf`,
then run `./ze show host trend` in a second terminal. `etc/ze/claude-ssh.conf`
is his local test config: never stage it.

## Design decisions taken (they bind the remaining work)

- `internal/core/privilege` holds the one membership rule. `ServiceUnit =
  "ze.service"` (the systemd plugin's unit name now reads it);
  `ServiceMembership` (Unspecified, ZeService, Other, Unknown);
  `DaemonServiceMembership()` reads the cgroup v2 line of `/proc/self/cgroup`
  and answers ZeService only when the last path element is `ze.service`;
  `SetServiceMembershipForTest(m, err) (restore func())` is the test seam;
  `CapabilityRemedy(missing ...string)` gives the systemd repair only under
  ZeService, otherwise a remedy naming the capabilities for the ze process, and
  under Unknown says Ze could not tell. `DaemonCapabilitiesFix` stays (the unit
  needs it); `DaemonCapabilitiesAdvice` is deleted.
- `CheckPrivileges` is now `([]Capability, error)`, the generic inventory; root
  is inventoried too (root can run with a reduced set).
- Doctor gains `diagnostic.SeverityInfo = "info"`: never a readiness failure,
  never counted as a warning. The `daemon-capabilities` row is info, keeps the
  code `doctor-daemon-capability-missing` (its CodeMeta is reworded), and says
  each feature reports its own impact.
- One producer words every BGP listener refusal: `network.ListenError`
  (`internal/core/network/listen_error.go`), naming address, port, the claiming
  peer or group, the capability when the port is below
  `net.ipv4.ip_unprivileged_port_start` (read, never assumed), and the fix.

## State

| # | Part | State | Next action |
|---|------|-------|-------------|
| 1 | Core: inventory to Debug (`cmd/ze/hub` `logDaemonCapabilities`), doctor info severity, membership and `CapabilityRemedy`, docs (`system-architecture.md`, `doctor-and-health-checks.md`, `process-protocol.md`, `features/core-diagnostics.md`) | Written. `./le test unit pkg path` passed for `internal/core/privilege/`, `internal/core/diagnostic/`, `internal/plugins/systemd/` | Run the rest of its owed commands (below) |
| 2 | BGP listener refusal (`internal/core/network/listen_error.go`, `bgp/reactor/listen_refusal.go`, `reactor.go`, `reactor_dynamic.go`, `reactor_iface.go`, `doctor/checks_listener.go`, docs `peer-lifecycle.md`, `interface-event-reactions.md`) | Written, not run. No existing expectation changed. Draft `.ci` at `test/draft/plugin/bgp-listen-privileged-port-refused.ci` (gitignored) | Run its owed commands; settle open item 5 before promoting the `.ci` |
| 3 | IPsec (`internal/component/ike/**`) | Partial. First agent's work done (below); a continuation agent was stopped mid-run, so read the diff before trusting anything | Finish from the list below |
| 4 | `ServiceRemedy` and `RestartRemedy` in `internal/core/privilege`, and every unconditional systemd command outside IPsec routed through them | Not started: no file changed. `dataplane.ServiceRemedy` (ike) is a private copy that must move here | Implement, then delete the ike copy |
| 5 | traffic-usage readiness (`internal/plugins/trafficusage/**`, docs `traffic/traffic-usage.md`, `guide/traffic-usage.md`, `guide/configuration.md`) | Partial, agent stopped mid-run (it was on the guide's doctor section). Read the diff | Finish, then test |
| 6 | sysctl-procfs false finding (`internal/component/sysctl/doctor*.go`) | Not started | Confirm, fix |

### IPsec: done by the first agent

Every IPsec capability site words its fix through `CapabilityRemedy`; the file
limit and esp4_offload fixes go through `dataplane.ServiceRemedy` (to replace,
item 4). Fast-path readiness: `dataplane.ESPFormFastPathReady()`
(`espform_fastpath_probe_linux.go`: loads the collection, TCX probe at ifindex
MaxInt32, closes; EPERM is a capability fault, `ebpf.ErrNotSupported` a kernel
fault, anything else unknown) recorded per apply by `engine/apply.go`
`recordESPFormFastPath`, one Warn per apply with `doctor-ipsec-esp-form-fast-path`,
and the Idle state reports the record. `ipsec-application-override` moved to a
runtime check (`engine/doctor_application_override.go`, order 736; the
post-config row is gone from `register.go`); every finding has a Fix.

### IPsec: remaining

1. Compile blockers in engine tests: an unused `plugin/registry` import in
   `engine/rfc4301_ike_bypass_forward_integration_linux_test.go`; in
   `engine/doctor_application_override_test.go` check `registry` use and add Fix
   assertions (EPERM gives the capability remedy, `ErrNoCgroup2` the mount, other
   the allow statement; guard and socket-policy findings too);
   `engine/doctor_esp_form_offload_linux_test.go` pins the systemd line in its
   offload-fix assertion: pin `MembershipZeService` and add a `MembershipOther`
   case.
2. Tests to write: `newESPFormFastPathRecord`,
   `espFormFastPathReadinessFindings` and the Idle branch (nil, loads, not
   supported, capability fault under both memberships, unknown, empty record);
   `recordESPFormFastPath` modelled on `TestRecordESPFormTUN`;
   `TestEngineDoctorCheckDispatch` Idle with a "will not load" record;
   `TestIPsecEngineStateChecksAreRuntimePhase` (planned by AC-36 of
   `plan/release/validation/spec-doctor-coverage-3-access-security.md`); a
   kernel-fault case expecting "Linux 6.6" in `doctor_esp_form_fast_path_test.go`.
3. `.ci`: `test/ipsec/ipsec-esp-form-fast-path-doctor.ci` and
   `test/ipsec/ipsec-esp-form-fast-path-doctor-external.ci` each expect
   `"fix": "sudo systemctl stop ze && sudo ze install systemd --force --start"`;
   a harness daemon is not under `ze.service`, so they expect the
   other-process remedy. `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`
   pins the removed "older ze does not grant CAP_BPF" text.
4. Every `sudo systemctl restart ze` in ike (`dataplane.ESPFormFixRestart`, and
   three sites in `engine/redistribute.go`) goes through `RestartRemedy`
   (item 4); `dataplane.ServiceRemedy` is deleted for `privilege.ServiceRemedy`.
5. Docs: `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (the fast path and
   the override guard), `docs/guide/ipsec.md` (the guard refusal text, the
   post-config source note, the offload fix),
   `docs/architecture/doctor-and-health-checks.md` (the fast-path record beside
   `recordESPFormTUN`, `ipsec-application-override` among the runtime checks).

### Item 4 brief

`privilege.ServiceRemedy(service, other string) string` (service text under
ZeService, other under Other, an "Ze could not tell whether it runs under
ze.service (<err>); " prefix under Unknown) and `privilege.RestartRemedy()` (the
systemctl restart only under ZeService; otherwise the restart that truly works
for an unsupervised ze: read `ze signal restart`'s handler before choosing, and
say so if no single command restarts one). `CapabilityRemedy` uses
`ServiceRemedy`. Then grep `internal/`, `cmd/`, `pkg/` for `systemctl restart
ze`, `systemctl stop ze`, `ze install systemd`, `systemctl edit ze` and direct
`DaemonCapabilitiesFix` uses (known: l2tp, the plugin server); route each,
except where the operator is managing the unit itself (`internal/plugins/systemd`).

### Item 5 brief (traffic-usage)

Record each configured interface's attach outcome (attached, refused with
`CapabilityRemedy("CAP_BPF","CAP_NET_ADMIN")`, cannot determine); log a refusal
once per change of outcome, not every snapshot; a doctor code on every Warn and
Error line; the doctor check reports the recorded outcome from the daemon; a
disabled feature probes and reports nothing; the inert behaviour on a kernel
without TCX stays.

### Item 6 brief (sysctl)

`checkSysctlProcfs` uses `access("/proc/sys", W_OK)`, which judges the real uid
with no capabilities, while the kernel's `net_ctl_permissions`
(`net/sysctl_net.c`) lets CAP_NET_ADMIN write `net.*` keys: a non-root
CAP_NET_ADMIN daemon is likely reported unable to write what it can. Confirm in
the kernel source, then judge what the daemon can actually write, and say
"cannot determine" when it cannot tell.

## Test expectations changed (route to `ze-test-judge` if the commit gate asks)

- `internal/component/doctor/checks_daemon_capabilities_test.go`: severity
  warning to info; Fix `DaemonCapabilitiesFix` only under ZeService, never a
  systemd command otherwise; message substrings follow the new wording; the
  stub returns `([]privilege.Capability, error)`.
- IPsec tests that pin `DaemonCapabilitiesFix`: membership pinned to ZeService,
  value unchanged (`espform_tun_open_linux_test.go`,
  `xfrm_migrate_linux_test.go`, `xfrm_forward_default_linux_test.go`,
  `espform_reopen_linux_test.go`, `espform_refusal_linux_test.go`,
  `doctor_esp_form_fast_path_test.go`); in the fast-path test the systemd text
  moved from the message to the Fix, and "Linux 6.6" now belongs to the kernel
  fault only.
- None was seen to carry an `RFC requirement:` tag; confirm.

## Owed commands (main thread, one CPU job at a time)

`./le test unit pkg path <dir>/` for: `internal/core/network/`,
`internal/component/bgp/reactor/`, `internal/component/doctor/`,
`internal/component/doctor/cmd/`, `cmd/ze/hub/`,
`internal/component/ike/dataplane/`, `internal/component/ike/engine/`,
`internal/plugins/trafficusage/`, `internal/component/sysctl/`, and every
package item 4 touches; `./le go lint run scope "<dir>"` for each;
`./le arch doctor-coverage check`, `./le arch operator-error check`,
`./le arch error-wrap check`, `./le arch tier check`; `./le doc index update`
then `./le doc check all`; `./le test ui --pattern doctor-bgp-listen`,
`./le test ui --pattern doctor-listeners`; the ipsec functional lines for the
changed `.ci`; `./le test bgp plugin --draft --pattern bgp-listen-privileged-port-refused`.
Capability integration tests (`dataplane/espform_fastpath_unit_caps_integration_linux_test.go`,
the traffic-usage one) need a netns or QEMU run.

## Verification Thomas asked for

1. Interfaces and SSH: `./ze` rebuilds `bin/ze` through `./le job run` when the
   source changed. As the unprivileged user (no capabilities at all, stricter
   than his run), `./ze start` on a config with SSH and no interfaces logs no
   capability warning, and `./ze show host trend` answers. Thomas then runs his
   setpriv command above for the CAP_NET_ADMIN-only case.
2. A feature that truly lacks its privilege: a BGP peer whose listener needs
   port 179 (or the draft `.ci`'s port 178), started as the unprivileged user,
   refuses start naming the address, port, peer, CAP_NET_BIND_SERVICE and the
   remedy (no systemd command, since the process is not under `ze.service`).

## Open items for Thomas

1. Whether `internal/component/iface/register.go` `permissionRemediation`
   (its own "run as root / setcap" text) moves to `CapabilityRemedy`; it changes
   `apply_errors_test.go`, which pins "setcap".
2. `internal/test/fixture/daemon.log` holds the old warning text and looks like
   a stray committed run artefact with no reader.
3. An unreadable capability set is logged at Debug at start (his order), against
   the style rule that a caught error is logged at WARN; the doctor info row
   still reports it.
4. On the daemon route the doctor bind probe meets EADDRINUSE on a port the
   daemon holds: `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`
   AC-1 owns that fix.
5. The BGP `.ci` passes only on a runner without CAP_NET_BIND_SERVICE; the
   runner cannot skip a test when it holds a capability. Promote it with that
   host requirement in its header, or add a runner option first.
6. `ipsec-spd-writers` and `ipsec-interface-address` have the same silent
   daemon-route defect the override check had; AC-36 of
   `spec-doctor-coverage-3-access-security.md` owns them.

## Commits

Commit in coherent slices once each is green: core (item 1), BGP listener
(item 2), item 4 with the ike remedy move, IPsec (item 3), traffic-usage,
sysctl. Other sessions share this checkout: name only these files.

## Related defects found this session, outside this work

- `ze init` (default profile) saves the SSH address for the CLI but writes no
  SSH listener into the config it creates, so `ze cli` cannot reach a daemon
  started from it (`internal/plugins/init/main.go`, the populate step).
- `show host trend` says "no daemon answered" for every connection failure,
  including a refused login (`internal/plugins/host/trend_offline.go`); owed in
  `spec-host-resource-trend-alerts`.
- Ze rewrites bridge settings the kernel already holds (STP via
  `BridgeSetSTP`), which needs CAP_NET_ADMIN even when nothing changes.
