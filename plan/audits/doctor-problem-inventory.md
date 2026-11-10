# Doctor problem inventory

Every problem an operator can hit in Ze, grouped by problem class and owner, with
the doctor check and code that reports it today or `gap` where none does. It is
the audit that acceptance criterion AC-6 of
`plan/release/validation/spec-doctor-coverage-1-audit-and-gate.md` names, and
the input of the area children `spec-doctor-coverage-<n>-<area>.md`.

- Date: 2026-11-10.
- How it was produced: `./le arch doctor-coverage report` over the working tree
  (baseline: 2412 problem sites, 0 covered, 175 registered doctor codes), then
  15 audit slices (A1 to A15), one or more per door area. Each slice read every
  report site of its packages at the producing function, grouped the sites into
  problem classes, and walked the silent states no log line names (refusals that
  leave state degraded, config accepted but not in effect, missing capability,
  every health row).
- The per-site list stays in the report (R-3): run `./le arch doctor-coverage
  report` for it, with each site's line. The source-sites column of each area
  file gives the site count and at most three files; line numbers are left to
  the report, which regenerates them, because a line cited in prose rots.
- Layout: this index holds the totals, the AC-9 list, the not-problem groups,
  the defects, the shared mechanisms and the canonical codes; each door area's
  class table is its own file under `doctor-problem-inventory/`.
- Route: `local` means readable from config, the store, the host or a network
  probe without the daemon; `runtime` means it needs live daemon state;
  "recent event" means a transient class reported through the report bus and the
  `doctor-recent-events` row (O-2).
- Codes: an existing code is named with its check. A proposed code is named
  `new:` as the slice wrote it, and the cell ends with `Canonical:` and the one
  name the area child registers. The full proposed-name mapping is
  `tmp/session/2026-11-09-223019/scratch/audit/CODES.md`; the canonical list is
  the last section here. A gap row with no name gets its code from its area
  child.
- Rows marked "owned: child 2" belong to
  `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` and are not
  re-specified.

## Areas

Totals: 615 classes, 539 gaps (partial included), 67 covered, 9 owned by child 2.

| Door area | Slices | Class rows | Gap | Covered | Child 2 |
|-----------|--------|-----------:|----:|--------:|--------:|
| [access-security](doctor-problem-inventory/access-security.md) | A1, A2 | 114 | 89 | 19 | 6 |
| [api-surfaces](doctor-problem-inventory/api-surfaces.md) | A3 | 13 | 13 | 0 | 0 |
| [commands-cli](doctor-problem-inventory/commands-cli.md) | A3 | 8 | 8 | 0 | 0 |
| [config-yang](doctor-problem-inventory/config-yang.md) | A3 | 18 | 18 | 0 | 0 |
| [hub-engine](doctor-problem-inventory/hub-engine.md) | A4 | 22 | 21 | 1 | 0 |
| [appliance](doctor-problem-inventory/appliance.md) | A4 | 19 | 16 | 3 | 0 |
| [system](doctor-problem-inventory/system.md) | A4 | 21 | 17 | 4 | 0 |
| [bgp-engine](doctor-problem-inventory/bgp-engine.md) | A5 | 47 | 45 | 2 | 0 |
| [bgp-plugins](doctor-problem-inventory/bgp-plugins.md) | A6, A7 | 66 | 65 | 1 | 0 |
| [forwarding](doctor-problem-inventory/forwarding.md) | A8, A9 | 99 | 86 | 10 | 3 |
| [routing-protocols](doctor-problem-inventory/routing-protocols.md) | A11, A12 | 65 | 54 | 11 | 0 |
| [system-plugins](doctor-problem-inventory/system-plugins.md) | A13, A14, A15 | 94 | 78 | 16 | 0 |
| [plugin-infra](doctor-problem-inventory/plugin-infra.md) | A10 | 29 | 29 | 0 | 0 |

Merged rows. A class that two slices both listed keeps one row, under the area
that owns the state, and the cell says what was merged:

| Class | Kept under | Merged from |
|-------|------------|-------------|
| Plugin exited or crashed | plugin-infra (A10, "A configured plugin is not running") | A1, A2, A4-system, A5 C50, A6, A7 B1, A8, A9 |
| Learned-resolver write | appliance ("resolv.conf not written") | A8 learned resolvers row |
| System-DNS publication and consumer sync | appliance ("System-DNS consumer publication not kept") | A8 RDNSS publish row, A13 consumer row |
| Management listener not serving | api-surfaces, and commands-cli for SSH | A4-hub-engine service row |
| `gnmi server` extra listeners ignored | api-surfaces | A4-hub-engine extra-listeners row (plugin-hub half stays in hub-engine) |
| Confirmed commit pending | config-yang | A4-hub-engine confirmed-commit row |
| Firewall apply behind | forwarding (A9) | A13 anomaly-shape armed rate-limit |
| Firewall stale tables | forwarding (A9) | A13 anomaly-shape tables left, A13 ddos-local drop rule left |
| `ze_vrrp` table gone | forwarding (A9 drift) | the VRRP master-dataplane row keeps its per-group intent and points to drift |
| BGP OPEN refused or unreadable | bgp-engine (A5 C5) | A6 OPEN unreadable row |

## AC-9: checks reading daemon state outside runtime

Each check below is registered at `pre-config`, `parsed-config` or
`post-config` but reads state only the running daemon holds, or probes a port
the running daemon itself binds. On the local route it reads that state as
empty and healthy, or reports a false problem on a healthy box. Each moves to
`runtime` (or splits into a local half and a runtime half) in its area child.
17 entries, 16 checks: `doctor-bfd-port` is a code of the `listeners` check,
listed apart because its owner is BFD.

| # | Check | Codes | Owner | Phase today | Daemon state it reads | Effect on the local route | Move | Slice |
|---|-------|-------|-------|-------------|-----------------------|---------------------------|------|-------|
| 1 | `ipsec-udp-encap` | `doctor-ipsec-udp-encap` | internal/component/ike/engine | post-config | `udpEncapState` set by the engine's `openNATT` | empty, healthy | runtime (child 2) | A1 |
| 2 | `ipsec-virtual-ip-pool` | `doctor-ipsec-virtual-ip-pool` | internal/component/ike/engine | post-config | `virtualIPPoolUsage()` over the running SA table | empty, healthy | runtime (child 2) | A1 |
| 3 | `ipsec-interface-address` | `doctor-ipsec-interface-address` | internal/component/ike/engine | post-config | `interfaceWaitPtr`, written by the engine's apply path | empty, healthy | runtime; same owner and `register.go` as 1 and 2, so child 2 is the natural home | A1 |
| 4 | `ipsec-application-override` | `doctor-ipsec-override-guard`, `doctor-ipsec-socket-policy` | internal/component/ike/engine | post-config | `overrideRefusal`, `appliedOverride`, `dataplane.Get()` guard state | empty, healthy | runtime (child 2 home) | A1 |
| 5 | `ipsec-spd-writers` | `doctor-ipsec-spd-writer`, `doctor-ipsec-spd-writer-unknown` | internal/component/ike/engine | post-config | gated on `appliedOverride` | answers nothing even when other SPD writers exist | runtime (child 2 home) | A1 |
| 6 | `ospf-config-sanity` (BFD half) | `doctor-ospf-bfd-plugin-absent` | internal/plugins/ospf | post-config | `bfd/api.GetService()`, filled by the running bfd plugin | always nil: warns "BFD absent" whenever BFD is configured | split the BFD half into a runtime check | A12 |
| 7 | `ospf-state` (SDK-declared) | `doctor-ospf-graceful-restart-nvs` | internal/plugins/ospf | post-config | the running plugin's `p.StateKeys` restart facts | only answerable in the daemon | declare `runtime` | A12 |
| 8 | `ddos-detect-rate-feed` | `doctor-ddos-detect-blind` | internal/plugins/ddos/detect | post-config (`DoctorCheckDecl`) | the running detector's rate feed through `OnDoctorCheck` | never runs locally | declare `runtime` | A13 |
| 9 | `copp-input-chain` | `doctor-copp-missing` | internal/plugins/copp | post-config | `firewall.GetBackend()`, loaded only by `ApplyAll` | no backend locally: "kernel firewall tables could not be read" on a healthy box (inferred from source) | runtime, or load a read-only backend locally | A13 |
| 10 | `ntp-clock-privilege` | `doctor-ntp-clock-privilege` | internal/plugins/ntp | post-config | `/proc/self/status` CapEff and uid of the process running the check | judges the CLI caller, not the daemon | runtime, or read the daemon's pid | A14 |
| 11 | `geodns-listen-capability` | `doctor-geodns-port-unavailable` | internal/plugins/geodns | post-config | bind probe of each configured privileged port | warns on a healthy serving box | runtime (`listenerUp`); the pre-start half only when no daemon answers | A14 |
| 12 | `static-route-skipped` | `doctor-static-route-skipped` | internal/plugins/static | post-config | `activeRouteManager.Load().skippedRoutes()` | nil: silent and healthy | runtime | A15 |
| 13 | `policy-igp-import-hides-nexthop` | `doctor-policy-igp-import-hides-nexthop` | internal/component/policy | post-config | `locrib.Default()` | nil: answers nothing | runtime | A4-system |
| 14 | `gtsm-kernel-state` | `doctor-gtsm-kernel-state` | internal/component/gtsm | post-config | in the daemon `publishedPeers()` and the installed state; locally peers from config | the installed-state half is never read locally | split: installed-state half to runtime, config-route half stays | A4-system |
| 15 | `listeners` | `doctor-listen-unavailable`, `doctor-image-listen`, `doctor-ntp-listen` and the other per-listener codes | internal/component/doctor (`checks_listener.go` `checkListeners`, `bindProbe`) | post-config | bind-probes every configured listener, ports the running daemon holds; `doctor.Answer` runs it on the daemon route too (`doctor/cmd/show.go`) | a healthy serving listener reads as "cannot bind" (EADDRINUSE; not reproduced) | split: local bind probe only when no daemon answers; runtime served-address read per surface (`doctor-mgmt-listener-not-serving` and the per-owner listener rows) | A3-api-surfaces, A14 |
| 16 | `ldp-port` | `doctor-ldp-port-unavailable` | internal/plugins/ldp | post-config | plain UDP bind of :646 (the engine's multicast socket) and TCP :646, which LDP never binds | "port unavailable" while LDP runs (not verified by run) | runtime, and drop the TCP half | A11 |
| 17 | `doctor-bfd-port` (code of `listeners`) | `doctor-bfd-port` | internal/component/doctor, state owned by internal/component/bfd | post-config | bind probe of UDP 3784, which BFD holds | "port unavailable" while BFD runs | moves with the `listeners` split; 4784, 3785 and VRF devices are a gap (routing-protocols) | A11 |

## Not problem classes

Sites a slice judged not to be an operator problem: a programmer defect, a
per-packet line a counter covers, teardown noise, an outcome answered to the
caller. Under O-2 every Warn and Error line still carries a doctor code; the
code each group carries is named, in canonical form. Where a slice named no
code, the merge assigned one and says so.

| Area | Sites | Groups and the code each carries |
|------|------:|----------------------------------|
| access-security (A1) | 48 | 23 IKE crypto/build failures, 9 event-bus emit failures, 2 unreachable group names, 2 authz unreachable paths, 4 transport close at teardown, 2 per-message build paths: `doctor-internal-defect` (assigned by merge). 5 single-response send failures: the IKE listener / send-path class's code (area child names it). 1 accepted longer key: demote to Info. |
| access-security (A2) | 54 | 6 BUG/invariant, 3 host entropy, 14 replies to a gone session, 8 close at teardown: `doctor-internal-defect` (assigned by merge). 12 send-queue enqueue failures and 7 per-packet lines: the peer classes' codes (child 2 `doctor-l2tp-peer-unresponsive`, the malformed-peer class). 3 self-recovering negotiations and 1 documented default (`plugins/pool/register.go`): demote to Info. |
| api-surfaces (A3) | 43 | 39 listener close, client went away, render failure, engine or YANG contract, web add-entry field, MCP edge: `doctor-internal-defect`. 4 request refusals shown to the caller: `doctor-mgmt-request-refused` (or Info). |
| commands-cli (A3) | 3 | `doctor-internal-defect`. |
| config-yang (A3) | 21 | listener registry at init 3, YANG command-tree defects 5, event marshal 4, ack channel full 8, rollback tier 1: `doctor-internal-defect`. |
| hub-engine (A4) | 13 | AAA bundle close 3, signal emit 1, test-only address file 2, `zetest` hold 2: `doctor-internal-defect`. Stop-time drain notices 2: raised on the report bus under the daemon-stop code the area child names, rendered by `doctor-recent-events`. Listener migration notice 1: `doctor-config-reload-refused`. Store recovery notice 1: `doctor-store-integrity`. `--web-only` resolvers 1: `doctor-dns-resolver`. |
| appliance (A4) | 6 (+10 outside scan roots) | backend registration 2, dnssec fallback 2, restart time 1, serial close 1: `doctor-internal-defect`. 10 `internal/appliance` build-host sites are outside the gate's roots: `doctor-appliance-e2fsprogs` and `doctor-internal-defect` if the roots grow. |
| system (A4) | 1 | GTSM on a non-Linux build: `doctor-gtsm-kernel-state`. |
| bgp-engine (A5) | 59 | 35 invariants, 6 recovered panics, 3 plugin protocol misuse, 5 operator or shutdown lines: `doctor-internal-defect` (assigned by merge; the signal status dump may drop to Info). 5 races with a session ending: `doctor-bgp-session-dropped`. 5 policy dry-run rendering lines: demote to Info (the command answer shows them). 3 silent states judged operator intent (admin down, update-delay, manual EOR): no code. |
| bgp-plugins (A6) | 74 | 50 engine-to-plugin events refused, 11 self-reported defects, 2 shutdown races: `doctor-internal-defect`. 5 registration failures at init: `doctor-plugin-setup-failed` (3 `os.Exit` sites reach stderr only). 4 config refusals answered to the committer: `config-plugin-verify` (a diagnostic code no doctor check declares; it needs one). 2 benign notes: `doctor-bgp-rib-note` or Info. |
| bgp-plugins (A7) | 65 | 23 plugin registrations: `doctor-plugin-missing` (existing); 4 auxiliary registrations: `doctor-plugin-setup-failed`. 23 engine-contract invariants, 4 YANG-bounded clamps, 4 RTR panic-guard follow-ups: `doctor-internal-defect`. 3 RPKI log-only lines: `doctor-rpki-log-only-retained` or Info. Withdraw withheld 1: `doctor-bgp-withdraw-withheld` or Info; watchdog config parse 1: `config-plugin-verify`; EPE stop 1: `doctor-internal-defect`. Watchdog session-ready 1: `doctor-bgp-session-ready-failed`. |
| forwarding (A8) | 45 | 30 wiring BUG paths, 6 recovered panics, 4 teardown, 3 races with no lasting effect: `doctor-internal-defect` (assigned by merge). 2 sysrib commit refusals: `config-plugin-verify`. |
| forwarding (A9) | 32 | each group carries its owner's class code: 8 producer BUG paths (`doctor-fib-sync-failure`, `doctor-fib-p4-no-backend`, `doctor-fib-vpp-sync-failure`, `doctor-fib-vpp-srv6-steer`); 3 no event bus (`doctor-fib-kernel-forwarding-off`, `doctor-fib-p4-no-backend`, `doctor-fib-vpp-no-writer`); 2 "no pending config": `doctor-plugin-config-unapplied`; 6 init registrations: `doctor-plugin-setup-failed`; 6 teardown closes: the owner's left-behind code; 5 fib-vpp flush: `doctor-fib-vpp-sync-failure`; 2 external-change emit: `doctor-fib-kernel-reassert-failed`. |
| routing-protocols (A11) | 29 | 3 plugin exits: `doctor-plugin-stopped`. 11 emit failures, 3 probe close, 5 invariants, 3 shutdown: `doctor-internal-defect`. 3 notifications on a closing session: `doctor-ldp-session-not-operational`. 1 non-Linux source address: `doctor-bfd-send-failed`. |
| routing-protocols (A12) | 50 | 23 socket close and rescan lines (slice proposed per-protocol close codes) and 24 Ze defects: `doctor-internal-defect`. 2 plugin exits: `doctor-plugin-stopped`. 1 OSPF started with no config: `doctor-ospf-idle` or Info. |
| system-plugins (A13) | 29 | 14 plugin exits: `doctor-plugin-stopped`. 9 emit failures, 4 marshal, 1 service nil, 1 close: `doctor-internal-defect`. |
| system-plugins (A14) | 26 | 8 plugin exits: `doctor-plugin-stopped`. 17 engine or programmer faults: `doctor-internal-defect`. 1 script killed at shutdown: `doctor-plugin-stop-cleanup-late`. |
| system-plugins (A15) | 26 | 6 plugin exits: `doctor-plugin-stopped` (`doctor-plugin-setup-failed` before ready). 10 invariants, 1 emit, 2 wedged-worker queues, 1 re-encode, 5 teardown close, 1 completion: `doctor-internal-defect`. |
| plugin-infra (A10) | 61 | 24 config-tx payload faults, 13 engine faults, 3 init registration faults: `doctor-internal-defect`. 4 plugin requests refused: `doctor-plugin-request-refused`. 6 answers undelivered: `doctor-plugin-answer-undelivered`. 5 drain incomplete: `doctor-plugin-drain-incomplete`. 5 reloads refused and rolled back: `config-plugin-verify` where a verifier refused, else `doctor-config-reload-refused`. 1 operator goroutine dump: Info, else `doctor-daemon-quit-requested`. Core (not judged by the gate): typed event drops carry `doctor-internal-defect`, report cap clamp `doctor-env-invalid-value`. |

## Defects found

Found by the slices while auditing; each owes a journal row and an owning spec
(the area child where it fits). "Unverified" marks an inference from source not
proven by a run.

1. `internal/component/l2tp/register.go` `health.Register("l2tp", checkHealth)` is unconditional while `PublishService` (`subsystem.go`) runs only after the disabled / no-listener early returns: a `ze_l2tp` build with L2TP unconfigured likely reports `l2tp` degraded "subsystem not running" (unverified).
2. `internal/plugins/pool/register.go`: the `ipv6-pd delegation-length` default (/56) is logged at Warn; a documented default is Info.
3. `internal/component/doctor/checks_listener.go` `listeners` runs on the daemon route (`doctor/cmd/show.go`) and bind-probes ports the daemon holds: healthy listeners read as EADDRINUSE (unverified; AC-9 row 15).
4. `internal/component/ssh/pubkey.go`: a `user ... public-key` that `parseConfiguredKey` cannot parse is accepted at commit (no verifier calls it) and never matches at login.
5. `internal/component/web/handler_config_entry.go`: web add-entry creates the entry without a field that failed, and the operator is not told.
6. `internal/component/config/archive/archive.go` `ChangeTracker.HasChanged` stores the new hash before the archive write (called from `scheduler.go`): a failed on-change archive is not retried until the config changes again.
7. `cmd/ze/hub/main_system.go`: an invalid self-update config (restart immediate and time together, bad HH:MM) is accepted at commit and no update backend starts.
8. `internal/component/config/system/system.go`: `system dns-source` refused at extraction may be accepted at commit (verify defect if so).
9. `cmd/ze/hub/main_system.go`: an invalid `storage smart` value is accepted at commit and replaced by a default.
10. `internal/component/storage/manager.go` `checkSelfTest` drops the self-test start error.
11. `internal/component/host/metrics.go`: `CollectOnce` drops a detect error; and host metrics (ECC, throttle, carrier events) start only from `internal/component/bgp/config/loader_create.go`, so a box without BGP or telemetry never raises them.
12. `cmd/ze/hub/main_reload.go`: a SIGHUP reload refusal is printed on stderr only; `cmd/ze/pushed_config.go` and `health_revert.go` report on stderr or a plain line, which gokrazy never shows (`main.go` comment).
13. `cmd/ze/hub/config_source.go` (`fix`), `listener_migrate.go` (`remedy`), `main_system.go` (`check`): ad hoc fix attributes where the doctor fix field belongs.
14. `internal/component/bgp/plugins/rs/server.go` logs `"rr plugin failed"` from the route server: the message names the wrong plugin.
15. `internal/component/iface/health.go` `CheckAllInterfaceErrors` returns 0 when the backend is nil or `ListInterfaces` fails, so the `iface` health row reads healthy for a kernel it never read (silent zero).
16. `internal/component/iface/config_sysctl.go`: a unit `sysctl-profile` naming no profile is accepted at commit and nothing is applied.
17. `internal/component/iface/pppoe_client.go`: an invalid PPPoE redial wait is accepted at commit and stops the client.
18. `internal/component/firewall/plugins/irr/register.go`: `firewall-irr-data-freshness` emits `doctor-firewall-irr-oversized-data` (`irr/doctor.go`) but its `Codes` omits it, so the gate cannot count it.
19. `firewall-nftables` judges nf_tables by `/proc/modules`: a kernel with nf_tables built in (Ze's own) lists no module, likely a false warning; it should be a kernelcap capability (unverified).
20. The firewall domain and IRR store doctor checks return nil when the store or cache cannot be opened, so the no-data class is silent when the cache is unreadable.
21. `internal/component/plugin/process/manager.go`: `plugin-down` is raised only on the two respawn-limit paths; an `ignore`-policy exit or a failed `restartPlugin` raises nothing, so the `plugins` health row reads healthy while the plugin is down.
22. `internal/component/plugin/server/startup.go`: a failed boot phase 2 to 5 returns early, skips every later phase and leaves `startupErr` unset (explicit-plugin failure drops family, event and send-type auto-load; confirm against `process-protocol.md`).
23. `internal/component/plugin/ipc/tls.go`: `validPluginName` is enforced by no config layer; an external plugin with a bad name is refused at connect-back and its start hangs 30 s.
24. `internal/plugins/rsvpte/register.go` `setupTunnel` increments `pathMsgSent` when `eng == nil`: the metric counts PATH messages never sent.
25. `internal/plugins/ldp` `ldp-port` probes TCP 646, which LDP never binds (it only dials).
26. `internal/plugins/isis/server.go` `openCircuits` returns the first `openCircuit` error and `OnStarted` returns it (`register.go`): one circuit on a missing device stops the whole IS-IS plugin (OSPF fixed the same shape in `instance.go`).
27. `internal/plugins/copp` `copp-input-chain` reports "kernel firewall tables could not be read" on the local route (AC-9 row 9; unverified by run).
28. `internal/plugins/flowexport/health.go`: the `flow-export` health row reads `Sender.Stats` errors, a counter that never resets: one failed send leaves a collector failing for the life of the exporter.
29. `internal/plugins/ntp/ntp.go`: an unreachable server Warns once per try per second until the first sync; `internal/plugins/mrt/async_writer.go` Warns once per dropped record (log floods).
30. `internal/plugins/mrt` reaches the engine through `registry.SetMRTMessageCallback` / `SetMRTPeerCallback`, a registry entry point naming one plugin: an external mrt is silently inert (engine names a plugin, against the modular-engine rule).
31. `internal/plugins/static`: `rm.skipped` is written only in `inject.go`'s apply path, so a reprogram failure after a BFD or link event is invisible to `static-route-skipped`.
32. `internal/plugins/tftpserver/handler.go` refuses write requests while TFTP uploads are in scope (RFC 1350 receiver obligations owed): an RFC gap for the RFC owner, not a doctor class.
33. `internal/plugins/bmp/yang/ze-bmp-conf.yang`: `statistics-timeout` is accepted while no Statistics Report is ever sent, and `route-action redistribute` acts as `monitor` (`bmp/receiver.go`): config accepted, not in effect.

## Shared mechanisms the area children need

1. **Per-peer last-failure record.** IKE keeps no per-peer "last failure / last teardown reason" (A1, about ten rows; child 2's A-3 asks the same); L2TP tunnel authentication (A2); BGP OPEN errors and validate-open refusals (A5 C5, C6); BMP per-session last error (A6); RSVP-TE last PathErr (A11); managed client last ack (A4-hub-engine, A10). One record shape per protocol, written where the failure is decided, read by one runtime check per area, with the event also raised on the report bus for `doctor-recent-events`.
2. **Refusal, suppression and drop counters.** Most runtime gap rows read "not kept today": L2TP tunnel and session capacity (A2), BGP C23, C26, C31, C33, C40, C43, C49 (A5), the RIB and route-server rows (A6), A7's B2, B3, B5, B10 and the plugin-delivery rows, A8's "no state kept" rows, fib-vpp installs (A9), BFD drops and RSVP-TE sends (A11), DHCP server send errors (A13), MRT drops and flow-export read errors (A14), VRRP transmit errors (A15). A check needs a recent-error window, not a lifetime counter (the `flow-export` health row shows why, defect 28).
3. **Report-bus codes that are not doctor codes.** Each is registered as a doctor code under its canonical name (see "Canonical codes"), or renamed, and the health-row filter that matches it (`report.HealthProbeDegraded` / `HealthProbeDown` slice match) changes with it in the same commit:
   - `bgp` row: `session-stuck`, `session-flap`, `eor-timeout`; other reactor codes `op-queue-full`, `prefix-threshold`, `prefix-hold`, `prefix-stale`, `route-count-anomaly`, `session-dropped`, `notification-sent`, `notification-received`.
   - `plugins` row: `plugin-crash`, `plugin-down`.
   - `firewall` row: `firewall-stale-table`, `firewall-drift`. `fib` row: `fib-sync-failure`, `fib-programming-lag`, `fib-orphan`. `iface` row: `iface-errors`.
   - storage `temp-high`, `temp-critical`, `temp-rising`, `smart-failing`; host `ecc-error`, `throttle`, `carrier-change` (passed as a run-time `e.Code`, which the gate refuses as written); update `self-update`, `update-available`; pki `cert-expiry`, `cert-expired`; config `commit-save-failed`, `commit-aborted`, `commit-rollback`; looking glass `lg-unreadable-as-number`.
   - Health rows with no doctor code at all: `ipsec`, `l2tp`, `pki`, `bgp`, `plugins`, `firewall`, `fib`, `iface`, `vpp`, `flow-export`.
4. **`code` log-attribute collisions to rename.** The gate reads a `code` attribute as the doctor code: `internal/component/plugin/process/process.go` (runner exit status, rename `exit-code`), `internal/plugins/rsvpte/reservation.go` (ERROR_SPEC code, rename `error-code`), `internal/component/bgp/plugins/rpki/rtr_session.go` (RTR Error Report code), `internal/component/firewall/plugins/domain/register.go` and `irr/register.go` (`code=meta.Code`), `internal/component/host/metrics.go` (run-time `e.Code`).
5. **Gate scan misses.** A Warn through a function value is not seen: dhcpserver `warnLimited` (`internal/plugins/dhcpserver/register.go`, reached from `segment.go` and `register.go`). A non-literal message is not seen: `internal/plugins/flowexport/register.go` (`log.Warn("flow-export: " + warning)`). Outside the scan roots: `internal/appliance` (10 build-host sites) and `internal/install` / `cmd/ze-installer` (27 sites, initrd PID 1, needs an owner decision on whether a post-install record on /perm feeds doctor); `internal/core/report` is not judged.
6. **"Runs in-process only" registration field.** One declaration per plugin and one local check (`doctor-plugin-external-unsupported`) replace the per-plugin traps in A2 (authradius), A7 (epe, BGP-LS export), A8 (iface-dhcp), A13 (13 plugins), A14 (flow-export, flowspec-firewall, kernel, mrt) and A15 (vrrp, trafficusage).
7. **Served-address read per listener owner.** The runtime half of every listener class (management surfaces, SSH, DNS listeners of as112, dnsresolver and geodns, DHCP server, TFTP, image server, BMP receiver, BGP, BFD loops) needs the set of addresses each owner actually serves, so the `listeners` bind probe can stay local-only (AC-9 row 15).
8. **Last-reload outcome record.** Four hub-engine rows (reload refused, unwind failed, partial apply, audit lost) and BGP C45 need one producer-side record of the last reload, read by one runtime check.
9. **Missing status readers.** DHCP client status (A8: `show interface` has none), managed client connection state (A4-hub-engine), `routewatch` subscription state (A14), IS-IS/OSPF interface-running sets exposed beyond the transport (A12).
10. **Report-bus capacity.** Warning eviction and error-ring wrap can clear any `HealthProbe*` row (an evicted `plugin-down` turns `plugins` healthy): `doctor-report-bus-full` (A10) concerns every area that probes the bus.
11. **`doctor-recent-events`** (P1c, `checks_recent_events.go`) is the reader every "recent event" row assumes; the transient classes depend on it landing.

## Canonical codes

Every new code the area children register, with its owning area. The
cross-cutting codes (`doctor-internal-defect`, `doctor-plugin-stopped`,
`doctor-plugin-crashed`, `doctor-plugin-config-unapplied`,
`doctor-plugin-setup-failed`, `doctor-plugin-external-unsupported`,
`doctor-audit-record-failed`) are owned by plugin-infra. Codes marked child 2 are
that spec's proposals and stay there.

| Canonical code | Owning area |
|----------------|-------------|
| `doctor-anomaly-shape-cap-reached` | system-plugins |
| `doctor-anomaly-shape-kill-switch` | system-plugins |
| `doctor-archive-prune-failed` | config-yang |
| `doctor-archive-write-failed` | config-yang |
| `doctor-audit-record-failed` | plugin-infra |
| `doctor-authz-match-unknown-command` | config-yang |
| `doctor-bfd-auth-failure` | routing-protocols |
| `doctor-bfd-ipv6-not-bound` | routing-protocols |
| `doctor-bfd-loop-restart-needed` | routing-protocols |
| `doctor-bfd-loop-unbound` | routing-protocols |
| `doctor-bfd-packets-unmatched` | routing-protocols |
| `doctor-bfd-send-failed` | routing-protocols |
| `doctor-bfd-session-down` | routing-protocols |
| `doctor-bfd-state-change-dropped` | routing-protocols |
| `doctor-bgp-eor-timeout` | bgp-engine |
| `doctor-bgp-notification-received` | bgp-engine |
| `doctor-bgp-notification-sent` | bgp-engine |
| `doctor-bgp-op-queue-full` | bgp-engine |
| `doctor-bgp-prefix-hold` | bgp-engine |
| `doctor-bgp-prefix-stale` | bgp-engine |
| `doctor-bgp-prefix-threshold` | bgp-engine |
| `doctor-bgp-rib-note` | bgp-plugins |
| `doctor-bgp-route-count-anomaly` | bgp-engine |
| `doctor-bgp-session-dropped` | bgp-engine |
| `doctor-bgp-session-flap` | bgp-engine |
| `doctor-bgp-session-ready-failed` | bgp-plugins |
| `doctor-bgp-session-stuck` | bgp-engine |
| `doctor-bgp-timed-withdraw-refused` | bgp-plugins |
| `doctor-bgp-withdraw-withheld` | bgp-plugins |
| `doctor-bgpls-snapshot-undelivered` | routing-protocols |
| `doctor-cli-editor-unavailable` | commands-cli |
| `doctor-command-deprecated-used` | plugin-infra |
| `doctor-command-ensure-rollback-failed` | plugin-infra |
| `doctor-commit-aborted` | config-yang |
| `doctor-commit-rollback` | config-yang |
| `doctor-commit-save-failed` | config-yang |
| `doctor-config-confirm-pending` | config-yang |
| `doctor-config-confirm-revert-failed` | config-yang |
| `doctor-config-file-out-of-sync` | config-yang |
| `doctor-config-listen-host-overridden` | config-yang |
| `doctor-config-newer-release` | config-yang |
| `doctor-config-password-plaintext` | config-yang |
| `doctor-config-password-weak` | config-yang |
| `doctor-config-recovered-from-rollback` | config-yang |
| `doctor-config-reload-refused` | hub-engine |
| `doctor-cos-profile-unknown` | system-plugins |
| `doctor-cos-qos-apply-failed` | system-plugins |
| `doctor-crash-kernel-recovered` | system-plugins |
| `doctor-crash-records-unharvested` | system-plugins |
| `doctor-daemon-quit-requested` | plugin-infra |
| `doctor-ddos-detect-flow-source-failing` | system-plugins |
| `doctor-ddos-flowspec-announce-failed` | system-plugins |
| `doctor-ddos-flowspec-rate-limited` | system-plugins |
| `doctor-ddos-flowspec-withdraw-failed` | system-plugins |
| `doctor-ddos-flowtriq-unreachable` | system-plugins |
| `doctor-ddos-local-drop-failed` | system-plugins |
| `doctor-ddos-no-victim` | system-plugins |
| `doctor-dhcp-listener-down` | system-plugins |
| `doctor-dhcp-pool-exhausted` | system-plugins |
| `doctor-dhcp-relay-unserved` | system-plugins |
| `doctor-dhcp-reply-send-failing` | system-plugins |
| `doctor-dhcp-server-identifier-not-held` | system-plugins |
| `doctor-dns-listener-down` | system-plugins |
| `doctor-dns-source-consumer-stale` | appliance |
| `doctor-dns-source-host-only-dropped` | appliance |
| `doctor-dns-source-over-limit` | appliance |
| `doctor-editor-storage-write-failed` | commands-cli |
| `doctor-env-invalid-value` | plugin-infra |
| `doctor-exabgp-script-lines-dropped` | system-plugins |
| `doctor-exabgp-script-start-failed` | system-plugins |
| `doctor-exabgp-script-stopped` | system-plugins |
| `doctor-fib-kernel-forwarding-off` | forwarding |
| `doctor-fib-kernel-monitor-stopped` | forwarding |
| `doctor-fib-kernel-mpls-guard` | forwarding |
| `doctor-fib-kernel-mpls-install` | forwarding |
| `doctor-fib-kernel-mpls-remove` | forwarding |
| `doctor-fib-kernel-netlink` | forwarding |
| `doctor-fib-kernel-reassert-failed` | forwarding |
| `doctor-fib-kernel-routes-left` | forwarding |
| `doctor-fib-orphan` | forwarding |
| `doctor-fib-p4-no-backend` | forwarding |
| `doctor-fib-programming-lag` | forwarding |
| `doctor-fib-sync-failure` | forwarding |
| `doctor-fib-vpp-no-writer` | forwarding |
| `doctor-fib-vpp-srv6-ownership` | forwarding |
| `doctor-fib-vpp-srv6-steer` | forwarding |
| `doctor-fib-vpp-sync-failure` | forwarding |
| `doctor-firewall-apply-behind` | forwarding |
| `doctor-firewall-audit-failed` | forwarding |
| `doctor-firewall-domain-cache-write` | forwarding |
| `doctor-firewall-domain-group-truncated` | forwarding |
| `doctor-firewall-drift` | forwarding |
| `doctor-firewall-global-options-unapplied` | forwarding |
| `doctor-firewall-irr-refresh-failing` | forwarding |
| `doctor-firewall-legacy-tables-pending` | forwarding |
| `doctor-firewall-stale-table` | forwarding |
| `doctor-firewall-table-held-back` | forwarding |
| `doctor-firewall-tables-left-after-stop` | forwarding |
| `doctor-firewall-vpp-orphan-nat` | forwarding |
| `doctor-flowexport-collector-advice` | system-plugins |
| `doctor-flowexport-collector-send-failed` | system-plugins |
| `doctor-flowexport-conntrack-read-errors` | system-plugins |
| `doctor-flowexport-conntrack-reader` | system-plugins |
| `doctor-flowexport-psample-unavailable` | system-plugins |
| `doctor-flowexport-sampling-setup` | system-plugins |
| `doctor-flowspec-rule-limit` | system-plugins |
| `doctor-flowspec-rule-refused` | system-plugins |
| `doctor-flowspec-rules-not-applied` | system-plugins |
| `doctor-gnmi-listener-ignored` | api-surfaces |
| `doctor-gnmi-subscriber-dropped` | api-surfaces |
| `doctor-gnmi-tls-material` | api-surfaces |
| `doctor-host-carrier-change` | system |
| `doctor-host-ecc-errors` | system |
| `doctor-host-thermal-throttle` | system |
| `doctor-iface-errors` | forwarding |
| `doctor-iface-no-backend` | forwarding (child 2) |
| `doctor-igp-route-sink-missing` | routing-protocols |
| `doctor-image-build-mismatch` | system-plugins |
| `doctor-image-none` | system-plugins |
| `doctor-image-zefs-failed` | system-plugins |
| `doctor-internal-defect` | plugin-infra |
| `doctor-ipsec-xfrm-exemption-missing` | access-security (child 2) |
| `doctor-ipsec-xfrm-peer-no-child-sa` | access-security (child 2) |
| `doctor-isis-auth-failed` | routing-protocols |
| `doctor-isis-circuit-not-running` | routing-protocols |
| `doctor-isis-hello-rejected` | routing-protocols |
| `doctor-isis-lsdb-full` | routing-protocols |
| `doctor-isis-lsp-fragments-exhausted` | routing-protocols |
| `doctor-isis-redistribute-not-originated` | routing-protocols |
| `doctor-isis-system-id-duplicate` | routing-protocols |
| `doctor-kernel-routewatch-down` | system-plugins |
| `doctor-l2tp-peer-unresponsive` | access-security (child 2) |
| `doctor-l2tp-session-not-established` | access-security (child 2) |
| `doctor-l2tp-tunnel-not-established` | access-security (child 2) |
| `doctor-ldp-discovery-failed` | routing-protocols |
| `doctor-ldp-interface-unavailable` | routing-protocols |
| `doctor-ldp-label-space-exhausted` | routing-protocols |
| `doctor-ldp-loop-detected` | routing-protocols |
| `doctor-ldp-no-lsr-id` | routing-protocols |
| `doctor-ldp-peer-message-ignored` | routing-protocols |
| `doctor-ldp-reload-not-applied` | routing-protocols |
| `doctor-ldp-session-not-operational` | routing-protocols |
| `doctor-lg-backend-unavailable` | api-surfaces |
| `doctor-lg-bmp-peer-omitted` | api-surfaces |
| `doctor-lg-unreadable-as-number` | api-surfaces |
| `doctor-managed-client-notify-dropped` | plugin-infra |
| `doctor-managed-client-rejected` | plugin-infra |
| `doctor-mgmt-auth-failed` | api-surfaces |
| `doctor-mgmt-listener-not-serving` | api-surfaces |
| `doctor-mgmt-request-refused` | api-surfaces |
| `doctor-mgmt-user-list-unreadable` | api-surfaces |
| `doctor-mpls-label-not-programmed` | routing-protocols |
| `doctor-mpls-label-space-empty` | forwarding |
| `doctor-mrt-records-dropped` | system-plugins |
| `doctor-mrt-rib-dump-unavailable` | system-plugins |
| `doctor-mrt-write-failed` | system-plugins |
| `doctor-ntp-no-servers` | system-plugins |
| `doctor-ntp-offset-rejected` | system-plugins |
| `doctor-ntp-persist-failed` | system-plugins |
| `doctor-ospf-auth-failed` | routing-protocols |
| `doctor-ospf-auth-sequence-unreserved` | routing-protocols |
| `doctor-ospf-bfd-neighbor-down` | routing-protocols |
| `doctor-ospf-bfd-session-missing` | routing-protocols |
| `doctor-ospf-bgpls-inter-as-withheld` | routing-protocols |
| `doctor-ospf-graceful-restart-stale` | routing-protocols |
| `doctor-ospf-idle` | routing-protocols |
| `doctor-ospf-interface-not-running` | routing-protocols |
| `doctor-ospf-key-chain-expired` | routing-protocols |
| `doctor-ospf-ldp-sync-stuck` | routing-protocols |
| `doctor-ospf-lsdb-full` | routing-protocols |
| `doctor-ospf-neighbor-parameter-mismatch` | routing-protocols |
| `doctor-ospf-opaque-malformed` | routing-protocols |
| `doctor-ospf-redistribute-not-originated` | routing-protocols |
| `doctor-ospf-sr-fib-undelivered` | routing-protocols |
| `doctor-ospfv3-ipsec-not-installed` | routing-protocols |
| `doctor-ospfv3-ipsec-state-leaked` | routing-protocols |
| `doctor-pki-cert` (existing; takes the report-bus `cert-expiry`, `cert-expired`) | access-security |
| `doctor-plugin-answer-undelivered` | plugin-infra |
| `doctor-plugin-autoload-unresolved` | plugin-infra |
| `doctor-plugin-capabilities-not-restored` | plugin-infra |
| `doctor-plugin-claim-unbacked` | plugin-infra |
| `doctor-plugin-command-not-registered` | plugin-infra |
| `doctor-plugin-config-unapplied` | plugin-infra |
| `doctor-plugin-connect-back-refused` | plugin-infra |
| `doctor-plugin-crashed` | plugin-infra |
| `doctor-plugin-delivery-disagreement` | plugin-infra |
| `doctor-plugin-delivery-failed` | plugin-infra |
| `doctor-plugin-drain-incomplete` | plugin-infra |
| `doctor-plugin-enricher-unavailable` | plugin-infra |
| `doctor-plugin-external-unsupported` | plugin-infra |
| `doctor-plugin-fatal-stop` | plugin-infra |
| `doctor-plugin-panic` | plugin-infra |
| `doctor-plugin-poststartup-undelivered` | plugin-infra |
| `doctor-plugin-removal-unrecovered` | plugin-infra |
| `doctor-plugin-request-refused` | plugin-infra |
| `doctor-plugin-setup-failed` | plugin-infra |
| `doctor-plugin-startup-phase-failed` | plugin-infra |
| `doctor-plugin-stderr-relay-stopped` | plugin-infra |
| `doctor-plugin-stop-cleanup-late` | plugin-infra |
| `doctor-plugin-stopped` | plugin-infra |
| `doctor-plugin-subscription-dropped` | plugin-infra |
| `doctor-policyroute-rollback-partial` | forwarding |
| `doctor-policyroute-rules-left` | forwarding |
| `doctor-pppoe-client-ipcp-not-open` | access-security (child 2) |
| `doctor-pppoe-client-ipv6cp-not-open` | access-security (child 2) |
| `doctor-pppoe-client-no-session` | access-security (child 2) |
| `doctor-provision-address-left` | system-plugins |
| `doctor-redistribute-family-unsupported` | routing-protocols |
| `doctor-redistribute-not-in-effect` | system-plugins |
| `doctor-redistribute-replay-lost` | system-plugins |
| `doctor-redistribute-source-undelivered` | routing-protocols |
| `doctor-report-bus-full` | plugin-infra |
| `doctor-rpki-log-only-retained` | bgp-plugins |
| `doctor-rsvpte-admission-denied` | routing-protocols |
| `doctor-rsvpte-admission-unmapped` | routing-protocols |
| `doctor-rsvpte-frr-unprotected` | routing-protocols |
| `doctor-rsvpte-link-event-dropped` | routing-protocols |
| `doctor-rsvpte-local-addresses` | routing-protocols |
| `doctor-rsvpte-lsp-refused` | routing-protocols |
| `doctor-rsvpte-no-router-id` | routing-protocols |
| `doctor-rsvpte-peer-message-invalid` | routing-protocols |
| `doctor-rsvpte-receive-stopped` | routing-protocols |
| `doctor-rsvpte-restart-needed` | routing-protocols |
| `doctor-rsvpte-send-failed` | routing-protocols |
| `doctor-rsvpte-tunnel-down` | routing-protocols |
| `doctor-self-update-waiting` | appliance |
| `doctor-ssh-public-key-invalid` | commands-cli |
| `doctor-static-bfd-not-tracking` | system-plugins |
| `doctor-static-external-interface-nexthop` | system-plugins |
| `doctor-static-route-no-active-nexthop` | system-plugins |
| `doctor-static-route-not-withdrawn` | system-plugins |
| `doctor-static-route-stale` | system-plugins |
| `doctor-static-vpp-fallback` | system-plugins |
| `doctor-storage-smart-failing` | system |
| `doctor-storage-temp-critical` | system |
| `doctor-storage-temp-high` | system |
| `doctor-storage-temp-rising` | system |
| `doctor-store-staged-leftover` | config-yang |
| `doctor-telemetry-collector-failed` | api-surfaces |
| `doctor-telemetry-config-deprecated` | api-surfaces |
| `doctor-tftp-not-listening` | system-plugins |
| `doctor-tftp-root-unreadable` | system-plugins |
| `doctor-traffic-qdisc-not-restored` | forwarding |
| `doctor-traffic-tc-snapshot-mismatch` | forwarding |
| `doctor-traffic-usage-left-attached` | system-plugins |
| `doctor-traffic-usage-not-attached` | system-plugins |
| `doctor-traffic-usage-read-failed` | system-plugins |
| `doctor-traffic-vpp-orphan` | forwarding |
| `doctor-update-available` | appliance |
| `doctor-update-server-ended` | system-plugins |
| `doctor-vpp-dpdk-left-bound` | forwarding |
| `doctor-vpp-process-restarting` | forwarding |
| `doctor-vpp-restart-required` | forwarding |
| `doctor-vpp-stats-down` | forwarding |
| `doctor-vrrp-advert-discarded` | system-plugins |
| `doctor-vrrp-advert-send-failed` | system-plugins |
| `doctor-vrrp-backup-dataplane-not-withdrawn` | system-plugins |
| `doctor-vrrp-group-not-running` | system-plugins |
| `doctor-vrrp-group-stale-parent` | system-plugins |
| `doctor-vrrp-master-dataplane-incomplete` | system-plugins |
| `doctor-vrrp-no-source-address` | system-plugins |
| `doctor-vrrp-owner-conflict` | system-plugins |
| `doctor-vrrp-parent-down` | system-plugins |
| `doctor-vrrp-sysctl-unapplied` | system-plugins |
| `doctor-vrrp-track-unresolved` | system-plugins |
