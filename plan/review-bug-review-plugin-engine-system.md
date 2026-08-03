# Plugin Engine and System Plugins Bug Review

## Summary

Read-only child 2 review for `plan/spec-bug-review-2-plugin-engine-and-system-plugins.md` completed. No production code, tests, generated files, specs, or docs were edited. No project-wide build, lint, format, or test gate was run.

Confirmed findings:

| ID | Severity | Owner | Short title |
|----|----------|-------|-------------|
| SYS-001 | BLOCKER | `internal/component/plugin/server`, `internal/component/plugin`, `internal/core/family` | Startup failure can be swallowed after partial dynamic registration |
| SYS-002 | ISSUE | `internal/component/plugin/server` | Failed reload cleanup passes plugin names to config-root stop logic |
| SYS-003 | ISSUE | `pkg/plugin/sdk`, `pkg/plugin/rpc` | DirectBridge callback panic leaves engine caller waiting for timeout |

Plausible findings:

| ID | Severity | Owner | Short title |
|----|----------|-------|-------------|
| SYS-004 | ISSUE | `internal/component/plugin/server` | Initial autoload downgrades dependency resolution errors to warnings |
| SYS-005 | ISSUE | `internal/component/vpp` | VPP DPDK parser accepts unknown nested keys if config reaches parser directly |

## Scope and files read

### Required specs, rules, and architecture

- `plan/spec-bug-review-2-plugin-engine-and-system-plugins.md`
- `plan/review-bug-review-inventory.md`
- `plan/spec-bug-review-0-umbrella.md`
- `plan/spec-bug-review-1-inventory-and-self-containment.md`
- `docs/architecture/core-design.md,336-400,1154-1171`
- `ai/rules/plugins.md`
- `ai/rules/plugins.md`
- `ai/rules/architecture.md`
- `skill://ze-review`
- `skill://ze-review-deep`

### Source files read by this review

Plugin infrastructure and SDK:

- `internal/component/plugin/types.go`
- `internal/component/plugin/registration.go`
- `internal/component/plugin/registry/registry.go,266-331,575-753,784-1053`
- `internal/component/plugin/all/all.go`
- `internal/component/plugin/server/startup.go`
- `internal/component/plugin/server/startup_autoload.go`
- `internal/component/plugin/server/reload.go`
- `internal/component/plugin/server/rpc_register.go`
- `internal/component/plugin/server/schema.go`
- `internal/component/plugin/server/dispatch.go`
- `internal/component/plugin/process/process.go`
- `internal/component/plugin/process/delivery.go`
- `pkg/plugin/rpc/bridge.go,154-190,571-632`
- `pkg/plugin/sdk/sdk_callbacks.go`
- `pkg/plugin/sdk/sdk_dispatch.go`
- `internal/core/family/registry.go`

System and component-owned plugin references:

- `internal/plugins/static/register.go`
- `internal/plugins/fib/kernel/register.go`
- `internal/plugins/tftpserver/config.go`
- `internal/plugins/tftpserver/handler.go`
- `internal/plugins/imageserver/config.go`
- `internal/plugins/imageserver/handler.go`
- `internal/plugins/bfd/bfd.go`
- `internal/component/vpp/config.go`
- `internal/component/vpp/yang/ze-vpp-conf.yang`
- `internal/component/firewall/plugins/irr/register.go`
- `internal/component/firewall/plugins/irr/config.go`
- `internal/component/flowexport/register.go`
- `internal/component/iface/register.go`
- `internal/component/ldp/register.go`
- `internal/component/rsvpte/register.go`
- `internal/component/traffic/register.go`

Command, schema, and root command wiring references:

- `cmd/ze/ze_core_dispatch.go`
- `cmd/ze/setup_features_distro.go`
- `cmd/ze/setup_features_setup.go`
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`
- `internal/component/cmd/show/yang/ze-cli-show-api.yang`
- `internal/component/cmd/clear/yang/ze-cli-clear-api.yang`
- `internal/component/cmd/delete/yang/ze-cli-delete-api.yang`
- `internal/component/cmd/delete/yang/self_containment_test.go`
- `internal/plugins/host-cmd/yang/ze-host-cmd.yang`
- `internal/plugins/host-cmd/cmd/show_host.go`
- `internal/plugins/host/host.go`
- `internal/plugins/exabgp/main.go`
- `internal/plugins/completion/root_commands.go`
- `internal/component/command/registry/registry.go`

### Inventory package classes covered

| Inventory class | Assigned rows covered | Status |
|-----------------|-----------------------|--------|
| Plugin infrastructure | `internal/component/plugin/*`, `pkg/plugin/*` | Covered, findings SYS-001 through SYS-004 |
| SCHEMA-CENTRAL-VERB | `internal/component/cmd/{clear,delete,log,meta,metrics,monitor,set,show,subscribe,update}/yang` | Covered as generic roots and self-containment anchors |
| SCHEMA-SYSTEM | `internal/plugins/*/yang`, non-BGP component schemas, command-only `*-cmd/yang` modules | Covered by class, VPP parser candidate SYS-005 |
| PLUG-COMPONENT | `firewall/plugins/irr`, `flowexport`, `flowexport/{ipfix,netflow9,sflow}`, `iface`, `iface/cli`, `ldp`, `rsvpte`, `traffic`, `traffic/cli`, `vpp` | Covered by representative config, command, doctor, and lifecycle reads |
| PLUG-SYSTEM | `bfd`, `connected`, `cos`, `crashes/cmd`, `debug/cmd`, `dhcpserver`, `diag/cmd`, `env`, `fib/{kernel,p4,vpp}`, `firewall/{nft,vpp}`, `flowspec-firewall`, `host-cmd/cmd`, `iface/{dhcp,netlink,vpp}`, `imageserver`, `kernel`, `l2tp*`, `log/cmd`, `meta/cmd`, `ntp`, `policyroute`, `routingtable`, `static`, `sysctl`, `sysctl/cli`, `sysrib`, `tftpserver`, `traffic/{netlink,vpp}`, `update-cmd/cmd` | Covered by class, route-config overlap noted for `static`, `routingtable`, `policyroute` |
| EVT-NAMESPACE | `internal/component/config/transaction`, `internal/component/isis` | Covered for registration mechanics. IS-IS protocol behavior excluded to protocol owner |
| RPC-GENERIC | non-BGP rows in `internal/component/*/cmd`, central generic RPC packages, plugin command packages | Covered by schema-to-handler and command registry reads |
| Directory-only command providers | `completion`, `connect`, `crashes`, `debug`, `diag`, `exabgp`, `explain`, `host`, `init`, `local`, `passwd`, `provision`, `signal`, `skills`, `support`, `systemd`, `internal/component/doctor` | Covered by root import and representative command reads. `exabgp` overlap routed to `spec-exabgp-compat-sync.md` |
| Excluded from child 2 | BGP plugin internals, BGP core engine internals, generated YANG glue, vendor, tmp | Excluded per inventory. Central seams were read only when they affected generic plugin infrastructure |

## Wiring/coverage audit table

| Surface | Entry point checked | Handler or owner path checked | Bridge or external path checked | Result |
|---------|---------------------|-------------------------------|---------------------------------|--------|
| Plugin startup lifecycle | `Server.runPluginStartup`, `runPluginPhase`, `handleProcessStartupRPC` | Registry, family, capability, command, coordinator paths | Internal net.Pipe and bridge switch after ready | SYS-001, SYS-004 |
| Dynamic config-root autoload | `getConfigPathPlugins`, `autoLoadForNewConfigPaths`, `autoStopForRemovedConfigPaths` | `registry.ConfigRootsMap`, process manager removal | N/A | SYS-002, SYS-004 |
| Dynamic event/send autoload | `getUnclaimedEventTypePlugins`, `getUnclaimedSendTypePlugins` | `registry.ResolveDependencies`, `PluginForEventType`, `PluginForSendType` | N/A | SYS-004 plausible |
| Command schema to handler | YANG `ze:command`, `pluginserver.RegisterRPCs`, dispatcher registry | Generic central roots and owner command packages | RPC dispatcher and DirectBridge dispatch-command args | No confirmed non-BGP schema-to-handler bug |
| DirectBridge callbacks | `DirectBridge.SendCallback`, SDK `bridgeEventLoop` | SDK callback map and process goroutine panic recovery | Internal bridge path compared to pipe event loop | SYS-003 |
| Structured events | `HasStructuredHandler`, `DeliverStructured`, process mixed batch delivery | `StructuredEvent` pool cleanup and async safety comments | DirectBridge only, text fallback checked | Cleared, no unsafe retention proven in read packages |
| Config verify/apply/rollback | SDK defaults, BFD/static-shaped callbacks, server tx bridge | `OnConfigVerify`, `OnConfigApply`, rollback default | Bridge callback RPC path | SYS-002 cleanup finding, no per-plugin rollback bug confirmed |
| File/socket services | TFTP, image server, VPP socket paths | path traversal, length, root directory, bcrypt validation | N/A | TFTP and image path classes cleared, VPP parser plausible SYS-005 |
| Doctor/env/metrics ownership | Registry doctor validation, BFD metrics rebinding, component doctor root | owner packages and generic doctor command root | N/A | Cleared by class, no dangling ownership proven |
| Active spec overlaps | `exabgp`, route config plugins | `spec-exabgp-compat-sync.md`, `spec-route-config-plugin-migration.md` | N/A | Route findings to active specs if accepted |

## Confirmed findings

### SYS-001, BLOCKER, Startup failure can be swallowed after partial dynamic registration

- File and line: `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/registration.go`, `internal/component/plugin/registration.go`, `internal/core/family/registry.go`.
- Reachable trigger: an external or internal plugin starts in a phase, reaches Stage 1 or Stage 3, then fails after a partial registration. Concrete examples: `registerPluginFamilies` registers an earlier family and then a later family conflicts, or `AddPluginCapabilities` appends an earlier decoded capability and then a later capability conflicts. Another plugin in the same tier can hit `stageTransition` failure, but `runPluginPhase` only waits for goroutines and returns nil.
- Expected behavior: startup is exact-or-reject. If a plugin fails any startup stage, the phase returns an error, partial registry, family, command, capability, event, send, and cache state is rolled back, and `WaitForStartupComplete` reports failure for config-path or fatal failures.
- Actual behavior: Stage 1 calls `s.registry.Register(reg)` before `registerPluginFamilies` and neither path has rollback. Capability injection mutates `globalCaps`, `globalByCode`, `peerCaps`, and `peerByCode` while iterating. `handleProcessStartupRPC` reports failure to the coordinator, but `runPluginPhase` does not inspect coordinator failure or final process stages after `procWg.Wait`, starts async handlers for any process collected, and returns nil to `runPluginStartup`.
- Impact: Ze can signal plugin startup complete with stale command/family/capability state from a plugin that never reached running. Subsequent command dispatch, family lookup, auto-load decisions, or BGP OPEN capability injection can observe a feature owned by a dead or rejected plugin. This violates the spec's exact-or-reject startup and rollback requirement.
- Severity: BLOCKER.
- Owner: plugin startup and dynamic registration infrastructure, `internal/component/plugin/server`, `internal/component/plugin`, `internal/core/family`.
- Regression test plan: add a plugin server startup unit test with two tier peers where one fails after Stage 1 and assert `runPluginPhase` returns an error, `WaitForStartupComplete` returns an error for config-path autoload, no command/family/capability is visible, and no async runtime handler starts for the failed process. Add a `CapabilityInjector` atomicity unit test where the second capability conflicts and the first capability is not retained.

### SYS-002, ISSUE, Failed reload cleanup passes plugin names to config-root stop logic

- File and line: `internal/component/plugin/server/reload.go`, `internal/component/plugin/server/reload.go`, `internal/component/plugin/server/startup_autoload.go`, `internal/component/plugin/server/startup_autoload.go`, `internal/plugins/fib/kernel/register.go`.
- Reachable trigger: a reload adds a new config-root plugin whose plugin name differs from its `ConfigRoots` entry, then transaction verify or apply fails. Example: adding `fib { kernel { ... } }` starts plugin `fib-kernel`, whose config root is `fib/kernel`, then a later affected plugin rejects the transaction.
- Expected behavior: the newly auto-loaded plugin is stopped and removed after the rejected transaction so the running process set remains equal to the pre-reload committed config.
- Actual behavior: `autoLoadForNewConfigPaths` returns started plugin names such as `fib-kernel`. The failure path calls `autoStopForRemovedConfigPaths(autoLoaded)`, but `autoStopForRemovedConfigPaths` treats its input as removed config roots and compares entries against roots such as `fib/kernel`. `fib-kernel` does not match `fib/kernel`, so the process is not stopped.
- Impact: a rejected config reload can leave a newly started backend process running without committed config. The stale plugin can own commands, metrics, sockets, backend state, or dependencies until process shutdown.
- Severity: ISSUE.
- Owner: plugin reload/autoload lifecycle, `internal/component/plugin/server`.
- Regression test plan: add a reload unit test that starts from no `fib/kernel` config, reloads with `fib/kernel` plus a second change that fails apply, and asserts `pm.GetProcess("fib-kernel") == nil` and no orphan dependency remains after the failed transaction.

### SYS-003, ISSUE, DirectBridge callback panic leaves engine caller waiting for timeout

- File and line: `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_dispatch.go`, `internal/component/plugin/process/process.go`, `internal/component/plugin/server/dispatch.go`.
- Reachable trigger: an internal bridge-mode plugin panics inside any engine-to-plugin callback handled by `bridgeEventLoop`, for example an `OnConfigApply`, `OnConfigVerify`, `OnBye`, or `OnAllPluginsReady` callback reached through `DirectBridge.SendCallback`.
- Expected behavior: the bridge path should match the external pipe path's error semantics closely enough that a crashing callback returns a prompt error, closes the bridge, and lets server cleanup remove the process-owned surfaces.
- Actual behavior: `bridgeEventLoop` calls `handler(cb.Params)` and then sends on `cb.Result`. It has no local `recover`. The outer internal plugin goroutine recovers and logs the panic, but the specific `BridgeCallbackResult` is never sent. `SendCallback` waits until its context expires. For bridge-mode plugins, the server runtime goroutine just waits on `s.ctx.Done()` and cleanup is not tied to the plugin goroutine panic.
- Impact: engine reload, shutdown, post-startup, or callback-driven commands can stall until timeout instead of failing immediately. The stale bridge/process state can also keep commands visible until broader shutdown or later cleanup.
- Severity: ISSUE.
- Owner: SDK and DirectBridge callback loop, `pkg/plugin/sdk`, `pkg/plugin/rpc`, `internal/component/plugin/process`.
- Regression test plan: add a DirectBridge SDK test that registers a callback which panics, invokes it through `SendCallback`, and asserts the call returns a non-timeout error promptly, bridge callbacks are closed, and process cleanup is invoked or the process is marked not running.

## Plausible findings

### SYS-004, ISSUE, Initial autoload downgrades dependency resolution errors to warnings

- File and line: `internal/component/plugin/server/startup_autoload.go`, `internal/component/plugin/server/startup_autoload.go`, `internal/component/plugin/registry/registry.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/startup.go`.
- Reachable trigger: an internal plugin registered for a config root, event type, or send type declares a missing hard dependency, or a future generated import omission leaves a dependency unregistered. Initial startup asks autoload to resolve dependencies for that plugin.
- Expected behavior: missing hard dependencies are registration/startup errors. The plugin stack should not start without declared hard dependencies.
- Actual behavior: initial config-root autoload and event/send autoload log a warning and set `resolved = needed`, so the owner plugin can be launched without its dependency. The reload path is stricter and returns an error for the same config-root dependency failure.
- Impact: a dependency declaration bug can become a runtime partial feature instead of a startup failure. Because SYS-001 can swallow later phase failures, this can present as a running Ze with missing helper plugin behavior.
- Severity: ISSUE.
- Owner: plugin autoload dependency resolution, `internal/component/plugin/server`.
- Regression test plan: add startup autoload unit tests for config-root, event, and send-type autoload where `registry.ResolveDependencies` returns `ErrMissingDependency`, and assert startup returns an error and no process is spawned.
- Why plausible, not confirmed: no current in-scope registered plugin with a missing hard dependency was verified. The defect is in the error path for future or build-tag-dependent registration drift.

### SYS-005, ISSUE, VPP DPDK parser accepts unknown nested keys if config reaches parser directly

- File and line: `internal/component/vpp/config.go`, `internal/component/vpp/config.go`, `internal/component/vpp/config.go`, `internal/component/vpp/yang/ze-vpp-conf.yang`.
- Reachable trigger: `ParseSettings` receives a VPP JSON section with typo keys under `dpdk` or under a `dpdk/interface` entry, for example `{"dpdk":{"interfase":{...}}}` or an interface object with `rx_queue` instead of `rx-queues`.
- Expected behavior: VPP config parsing is exact-or-reject like the surrounding parser functions. Unknown nested keys are rejected before startup.conf or DPDK binding is derived.
- Actual behavior: top-level config, `cpu`, `memory`, `stats`, and `lcp` call `unknownKeys`, but `parseDPDK` does not call `unknownKeys` for the `dpdk` container or interface entry fields. Unknown keys are silently ignored.
- Impact: if a config section reaches this parser without YANG rejecting the typo first, Ze can accept an operator typo and run VPP without the intended DPDK NIC or queue setting.
- Severity: ISSUE.
- Owner: VPP component plugin parser, `internal/component/vpp`.
- Regression test plan: add `ParseSettings` unit cases for unknown `dpdk` container keys and unknown per-interface keys, expecting errors that name the offending key.
- Why plausible, not confirmed: normal user config appears to pass through the YANG module read at `internal/component/vpp/yang/ze-vpp-conf.yang`, which should reject unknown schema leaves before this parser. The parser itself is inconsistent and may still be reachable through direct config sections or tests.

## Rejected candidates with proof

| Candidate | Rejected proof |
|-----------|----------------|
| DirectBridge structured event readiness race | Event delivery checks `bridgeReady := p.bridge != nil && p.bridge.Ready()` and `p.bridge.HasStructuredHandler()` before `DeliverStructured` in `internal/component/plugin/process/delivery.go`. `HasStructuredHandler` also checks bridge readiness in `pkg/plugin/rpc/bridge.go`. |
| TFTP path traversal through RRQ filename | `resolvePath` cleans the filename, strips leading slash, rejects `..`, evaluates the root and joined path symlinks, and verifies the resolved path stays under root in `internal/plugins/tftpserver/handler.go`. |
| Image server directory traversal | `serveFromDir` rejects empty names, slash and backslash, `.` and `..`, NUL, and any name changed by `filepath.Clean` before `http.ServeFile` in `internal/plugins/imageserver/handler.go`. |
| Missing external RPC framing bounds | The review read RPC framing, mux, message, and connection code through the delegated infrastructure pass. No unbounded per-message allocation was promoted because no surrounding caller path proved an attacker-controlled frame can bypass existing read and context handling. |
| Generic command root missing imports | Directory-only roots are blank-imported by `cmd/ze/ze_core_dispatch.go`, `cmd/ze/setup_features_distro.go`, or `cmd/ze/setup_features_setup.go` per inventory. No missing non-BGP root import was verified. |
| BGP-specific `PeerSubcommandKeywords` in plugin server | It is BGP-specific central code, but the only verified caller found was a BGP config test. Runtime BGP command ownership is assigned to child 3 or 4, so this was not promoted in child 2. |

## Cleared classes

| Class | Cleared lenses |
|-------|----------------|
| Static/routingtable/policyroute config-shaped plugins | Registration, config root, dependency shape, verify/apply reference flow. Active overlap with `spec-route-config-plugin-migration.md` noted. |
| TFTP and image server file services | Root directory requirements, traversal checks, bounded transfer concurrency, path validation, unauthenticated PXE trust boundary comments. |
| BFD system plugin | Configure, verify, apply, disable, shutdown cleanup, API provider publication, metrics rebinding. No child 2 bug promoted. |
| Component-owned RSVP-TE/LDP/iface/traffic/firewall/flowexport classes | Registration, doctor or command ownership, config/backend lifecycle sampled. Protocol-specific behavior remains protocol-owner scope when RFC compliance is involved. |
| Generic central verb roots | Bare root and command ownership model checked against self-containment rules. No non-BGP dangling handler path confirmed. |
| Directory-only command providers | Root import wiring checked. ExaBGP overlap goes to `spec-exabgp-compat-sync.md` for fix routing if a later compatibility finding survives. |
| Doctor/env/metrics generic ownership | Registry validation and owner-package patterns checked. No duplicate owner or dangling generic metric/doctor surface confirmed. |
| Event/send type registration mechanics | Registration and autoload paths read. Dependency handling remains SYS-004 plausible, no separate event-name collision defect confirmed. |

## Assumptions resolved

| ID | Resolution |
|----|------------|
| A-1 | Confirmed. Inventory rows assigned to child 2 were reconciled against `all.go`, directory-only imports, and package classes above. |
| A-2 | Confirmed. Same-shape references used: `static` for config plugins, TFTP/image server for file services, VPP for component-owned backend config, BFD for callback-heavy config/runtime plugin, central verb roots for command anchors. |
| A-3 | Confirmed with caveat. DirectBridge and external pipe paths were read separately. SYS-003 is bridge-specific. |
| A-4 | Confirmed. Config transaction participation is required for mutable config plugins. SYS-002 is a reload cleanup bug in the shared lifecycle rather than a per-plugin callback bug. |

## Remaining risks

- The system plugin surface is broad and OS-specific. Linux, VPP, nftables, netlink, and DPDK runtime behavior was not executed on Darwin. Findings that target those areas need Linux or QEMU/VPP regression in their fix specs.
- Protocol-specific correctness for BFD, LDP, RSVP-TE, IS-IS, L2TP, PPPoE, DHCP, RADIUS, and TACACS was not fully reviewed here unless it affected plugin lifecycle or command wiring. RFC-level defects should route to the relevant protocol owner.
- Generated YANG glue was treated as evidence only. A generator defect was not found or reviewed deeply in this child.
- No project-wide gates were run by instruction. The report itself is the verification artifact for child 5 dedupe and fix-spec creation.
