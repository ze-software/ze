# Release Readiness Review - 2026-04-24

## Scope

This document consolidates a fresh full-project review for moving Ze from experimental to an initial controlled deployment.

- Review mode only. No code was intentionally changed.
- The existing untracked audit at `plan/audits/release-readiness-review-2026-04-23.md` was read as a baseline, then rechecked and extended.
- The worktree was already dirty before this review. Existing untracked files were not overwritten.
- Forked agents reviewed security, BGP core, BGP plugins, config and command routing, platform/dataplane, L2TP/PPP/BFD/TACACS, docs/tests/specs, and persistence/lifecycle.

## Method

- Read project rules, architecture docs, feature inventory, current security policy, Makefile, existing audit, and active specs.
- Forked eight focused review agents after initial investigation.
- Ran registry, status, documentation, consistency, main verification, unit, and functional evidence commands.
- Consolidated only findings with source evidence or command output evidence.

## Repository Facts

Current inventory from `make ze-inventory-json`:

| Metric | Value |
|-------|-------|
| Registered RPCs | 181 in inventory, 182 YANG commands in command validator |
| Registered plugins in inventory | Includes BGP, BFD, static, traffic, firewall, VPP, L2TP plugins, FIB backends, sysrib, sysctl |
| Functional `.ci` tests | 790 |
| Functional test suites in inventory | encode, decode, parse, plugin, reload, ui, editor, managed, l2tp, l2tp-wire, firewall, traffic, static, vpp, chaos-web, exabgp-compat |
| Main Go source areas | `internal/`, `cmd/`, `pkg/` |

## Gate State

The project is not releasable today. The main gate and independent evidence runs are red.

| Command | Result | Evidence |
|---------|--------|----------|
| `make ze-verify-fast` | FAIL | Stops in `ze-lint` with 20 issues before tests run. |
| `make ze-unit-test` | FAIL | Failing packages include `cmd/ze`, `internal/chaos/inprocess`, `internal/component/cmd/subscribe`, `internal/component/plugin/all`, `internal/component/ppp`, `internal/component/radius`, `internal/core/clock`, `internal/plugins/bfd/engine`. |
| `make ze-functional-test` | FAIL | encode: 1 timeout; plugin: 11 failures/timeouts; parse: 72 failures; decode, reload, UI, editor, managed passed. |
| `make ze-doc-test` | FAIL | 40 doc drift issues and 29 YANG commands with no handler. |
| `make ze-consistency` | FAIL | 11794 errors and 4511 warnings, mostly because it scans `gokrazy/modcache`, plus real source size and plugin-structure errors. |
| `make ze-inventory-json` | PASS | Reports 790 `.ci` tests and current registry inventory. |
| `make ze-spec-status-json` | PASS | Shows many active/in-progress specs and done L2TP specs still in `plan/`. |

Important direct command evidence:

| Area | Evidence |
|------|----------|
| Lint baseline | `internal/plugins/policyroute/*`, `internal/plugins/static/*`, `internal/component/bgp/plugins/rs/server_withdrawal.go`, `internal/component/firewall/registry.go`, `internal/component/cmd/show/ping.go`. |
| Hot path race | `make ze-unit-test` reports data races between `session_coalesce.go` writes and plugin reads through Adj-RIB-In during `internal/chaos/inprocess.TestInProcessSpeed`. |
| SSH auth regression | `test/plugin/ssh-pubkey-auth.ci` failed: `FAIL: SSH with wrong key should NOT authenticate`. |
| L2TP command routing | `test/plugin/teardown-session*.ci` and `teardown-tunnel*.ci` failed with `unknown command` for `l2tp session teardown`, `l2tp session teardown-all`, `l2tp tunnel teardown`, and `l2tp tunnel teardown-all`. |
| YANG validator registration | Multiple plugin functional failures logged `missing validator registrations: ipv4-address, ipv4-prefix, ipv6-address, ipv6-prefix`. |
| Parse suite | 72 parse cases failed after the parse runner fixes, so current config/parser behavior and expectations are out of sync. |

## Fixed Since 2026-04-23 Baseline

These prior findings appear fixed or materially improved in the current tree. They still need regression evidence in the final release gate.

| Area | Status |
|------|--------|
| Remote SSH host-key verification | Remote hosts now require trust unless `ze.ssh.insecure=true`. |
| Managed TLS | Verification is now default; insecure mode is opt-in. |
| Managed first-boot validation | Fetched config is parsed before caching. |
| Web admin and web L2TP identity | Handlers now pass username and remote address. |
| BGP forward shared `ReceivedUpdate` mutation | Per-peer `exportWireOverride` appears to avoid mutating the shared cached object. |
| Structured event double pool return | Ownership appears consolidated in delivery code. |
| Malformed OTC | Now maps to treat-as-withdraw conversion. |
| RPKI ADD-PATH validation | Path ID is now carried through validation accept/reject paths. |
| RIB command paths | Inject, withdraw, release, purge mostly reconcile best path now. |
| Stale outbound marking | Source scoping now exists. |
| Best-path router ID fallback | Peer metadata is now used. |
| Parse runner false semantics | Parser runner now handles more directives and multiple stderr expectations, but parse suite is red. |
| BFD timer arithmetic | Remote desired-TX handling appears fixed. |
| L2TP PPP-down cleanup | PPP-driven cleanup appears fixed. |
| PAP first-request hang | Prior hang appears improved, but a new PAP wrong-protocol test still times out. |
| Static route error propagation | Static route apply/remove and non-Linux unsupported handling appear improved. |
| VPP startup config sockets | API and stats socket paths are now emitted. |
| `fib-vpp` cold boot | Now listens for both `vpp.connected` and `vpp.reconnected`. |
| API config session expiry | Last-activity tracking and cleanup now exist. |
| Resolve runtime wiring | DNS, Cymru, PeeringDB, and IRR resolvers are now constructed. |

## Confirmed Findings

### P0 - Release Blockers

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P0-1 | Release gate | `make ze-verify-fast`, `make ze-unit-test`, `make ze-functional-test`, `make ze-doc-test` | The main verification gate is red at lint, and independent unit, functional, and docs gates are also red. | No release or initial deployment decision can rely on current evidence. | Restore all mandatory gates to green before considering deployment. |
| P0-2 | Hot path concurrency | `internal/component/bgp/reactor/session_coalesce.go:183-186`, `internal/component/bgp/plugins/adj_rib_in/rib.go:235,314`, `internal/component/plugin/process/delivery.go:175-266` | Race detector shows the session read/coalesce buffer being overwritten while plugin delivery reads the same data. | Corruption or nondeterministic route parsing on the receive path under load. | Give delivered raw messages immutable ownership, copy before buffer reuse, or refcount the buffer until all consumers complete. |
| P0-3 | SSH authentication | `test/plugin/ssh-pubkey-auth.ci`, `internal/component/ssh/`, `cmd/ze/hub` | Functional test says a wrong SSH key authenticated. | Remote admin surface may accept unauthorized public keys until disproven. | Investigate immediately, add a focused unit/integration test, and make the functional test deterministic. |
| P0-4 | Command routing | `internal/component/l2tp/schema/ze-l2tp-api.yang`, `internal/component/cmd/l2tp/`, plugin functional tests 309-312 | L2TP teardown commands are in tests/docs but dispatch as `unknown command`. | Operator cannot tear down L2TP sessions/tunnels through the advertised command surface. | Register the command paths or correct tests/docs to the real path. |

### P1 - Security And Trust Boundaries

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P1-1 | MCP authz | `internal/component/mcp/streamable.go:784-804,1138-1144`, `internal/component/mcp/session.go:217-223`, `internal/component/mcp/handler.go:242` | MCP stores authenticated identity but does not pass it to command dispatch. | Authenticated MCP users execute as an empty username. | Set dispatcher username from session identity and test restricted MCP users. |
| P1-2 | Authz defaults | `internal/component/authz/authz.go:326-342,380-382` | Empty username and unassigned users are allowed or become admin. | Every missing identity propagation bug becomes admin-equivalent. | Fail closed for empty/unassigned users unless an explicit default profile is configured. |
| P1-3 | SSH plugin protocol | `internal/component/ssh/ssh.go:589-603`, `cmd/ze/hub/infra_setup.go:244-246`, `internal/component/plugin/server/adhoc.go:19-62` | Any authenticated SSH user can enter `plugin protocol` before normal authorization. | Low-privilege users may access plugin IPC behavior. | Gate this path behind explicit admin authorization or remove it from production. |
| P1-4 | REST/gRPC API auth | `cmd/ze/hub/main.go:591-645`, `internal/component/api/rest/server.go:249-267`, `internal/component/api/grpc/server.go:39-42` | API can start on remote listeners without auth and only warn. | Remote command execution/config edit if API is enabled without users/token. | Refuse non-loopback no-auth listeners unless explicit insecure mode is set. |
| P1-5 | API principal model | `cmd/ze/hub/api.go:133-159`, `cmd/ze/hub/main.go:623-635`, `internal/component/api/rest/server.go:249` | API auth loads only ZeFS users, and single-token mode dispatches as `api`. | Config users may not work; `api` principal becomes admin if unassigned. | Align API auth with SSH/AAA user sources and require explicit profile assignment. |
| P1-6 | L2TP PPP auth | `internal/component/l2tp/drain.go:89-92`, `internal/plugins/l2tpauthlocal/auth.go:42-45` | No auth handler or an empty local user table can accept PPP sessions. | Subscriber sessions may be accepted without valid credentials. | Fail closed unless `allow-unauthenticated` is explicitly configured. |
| P1-7 | RADIUS accounting | `internal/plugins/l2tpauthradius/acct.go:233-240` | Plugin shutdown cancels accounting sessions without sending Accounting-Stop. | Billing/NAS state can leak online users. | Send bounded Stop records for active sessions on shutdown. |
| P1-8 | Plugin IPC token compare | `internal/component/plugin/ipc/tls.go:113-118,194-198,303-308` | Token comparisons use variable-length `ConstantTimeCompare`. | Token length leaks by timing. | Compare fixed-length digests like MCP bearer auth. |

### P1 - BGP Core And Routing Correctness

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P1-9 | WireUpdate lazy state | `internal/component/bgp/wireu/wire_update.go:36-134` | Lazy parse/cache fields are unsynchronized despite shared use. | Go data races and invalid lazy state under concurrent plugin/forwarding access. | Use immutable parsed sections or `sync.Once` guarded lazy fields. |
| P1-10 | Malformed UPDATE dispatch | `internal/component/bgp/reactor/session_validation.go:30-53`, `session_read.go:150-222`, `session_handlers.go:198-210` | Truncated section lengths can pass callback dispatch before validation rejects them. | Malformed wire can reach plugins/cache as a route event. | Parse UPDATE sections before dispatch and suppress callbacks on parse errors. |
| P1-11 | Cache ack lifecycle | `internal/component/bgp/server/events.go:280-316`, `internal/component/plugin/process/delivery.go:185-190` | Batch delivery counts cache consumers before async delivery is known to succeed. | Cache entries can be pinned forever waiting for acks that never arrive. | Activate counts only after successful delivery or add failure cleanup. |
| P1-12 | LLGR command path | `internal/component/bgp/plugins/rib/rib_commands.go:1006-1077` | LLGR community attach/delete mutates RIB state without best-change publication. | Loc-RIB/FIB/event consumers can retain stale routes. | Reuse affected-prefix reconciliation used by purge paths. |
| P1-13 | LLGR timer semantics | `internal/component/bgp/plugins/gr/gr.go:114-124`, `rib_commands.go:28-54,721-738` | LLGR stale marking can schedule a 5 second safety purge despite long LLST. | Long-lived stale routes can disappear after seconds. | Separate LLGR stale-level updates from GR restart safety timers. |
| P1-14 | ROUTE-REFRESH subtype | `internal/component/bgp/server/events.go:339-352`, `rib_structured.go:381-414`, `rs/server.go:640-651` | BoRR/EoRR markers are treated as normal refresh requests by structured handlers. | Enhanced refresh markers can trigger replay or propagation. | Carry subtype in structured events and branch marker/request behavior. |
| P1-15 | ADD-PATH persistence | `internal/component/bgp/plugins/persist/server.go:347-355,468-491,766-777` | Persist plugin keys routes by prefix and discards path ID. | Multiple ADD-PATH routes collapse and replay incorrectly. | Key stored routes by `(prefix,pathID)` and add replay tests. |
| P1-16 | Route-refresh capability | `internal/component/bgp/reactor/reactor_api_forward.go:64-136`, `reactor_api.go:194-210` | Normal route-refresh can be sent to peers without negotiated route-refresh capability. | Interop failure or NOTIFICATION from peers. | Require `neg.RouteRefresh` for subtype 0. |

### P1 - Config, Command, API, And Feature Routing

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P1-17 | YANG command aliases | `internal/component/config/yang/command.go:21-43`, `ze-cli-show-cmd.yang:362-393`, `ze-cli-clear-cmd.yang:12-30` | Duplicate `ze:command` methods overwrite each other because `WireMethodToPath` stores one path. | Some documented aliases are unrouted. | Store all paths per wire method and register every alias. |
| P1-18 | `bgp summary` path | `ze-peer-cmd.yang:8-11`, `internal/component/api/rest/server.go:209`, `cmd/ze/hub/session_factory.go:112-117` | Live command is `summary`, while REST/dashboard call `bgp summary`. | REST peer list and dashboard summary can fail. | Restore alias or change callers to live path. |
| P1-19 | Command validator | `make ze-doc-test` | 29 YANG command paths have no handler. | Command docs and schema expose dead paths. | Fix registrations or remove command nodes. |
| P1-20 | Parser/config expectations | `make ze-functional-test` parse suite | 72 parse tests currently fail. | Config/parser evidence is unusable until reconciled. | Fix parser regressions or update invalid expectations with audit. |

### P1 - Platform, Dataplane, Persistence, Lifecycle

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P1-21 | VPP reload | `internal/component/vpp/register.go:86-122` | SIGHUP changes to `vpp {}` are accepted but not applied. | Runtime diverges from committed config. | Add verify/apply/rollback for VPP manager changes. |
| P1-22 | VPP validation | `internal/component/vpp/register.go:91-113`, `vpp.go:128-140` | Semantic validation can fail inside a goroutine and only log. | Invalid config can pass commit while VPP is inert. | Validate before manager startup and return errors. |
| P1-23 | Backend switching | `iface/backend.go:221-228`, `traffic/backend.go:152-159`, `firewall/backend.go:102-110` | Old backend is closed before new backend apply is proven. | Failed backend switch can orphan dataplane state. | Stage new backend, apply, then atomically swap and close old backend. |
| P1-24 | Traffic orphan qdiscs | `internal/plugins/traffic/netlink/backend_linux.go:23-43` | Removing an interface from config does not remove old qdiscs. | Runtime keeps stale traffic policy. | Reconcile full desired state and delete owned orphans. |
| P1-25 | Interface apply errors | `internal/component/iface/config.go:1010-1180` | Create/admin-up failures are debug-only in several paths. | Commit can succeed while interfaces are missing or down. | Treat declared interface create/up failures as apply errors. |
| P1-26 | ZeFS flush failure | `pkg/zefs/store.go:164-170,480-493,535-547` | In-memory state is rebuilt from new data even if disk flush fails. | Daemon can serve unpersisted config that disappears after restart. | Restore previous in-memory state on flush failure. |
| P1-27 | ZeFS crash durability | `pkg/zefs/store.go:504-532` | Atomic write lacks temp-file and parent-directory fsync. | Successful writes may be lost or corrupted on power loss. | Fsync temp file and parent directory around rename. |
| P1-28 | First boot init | `cmd/ze/init/main.go:229-336` | `ze init` writes the final DB path in multiple commits. | Crash can leave partial database that blocks retry. | Build DB at temp path and atomically rename after complete bootstrap. |
| P1-29 | Reload transaction | `cmd/ze/hub/main.go:801-837`, `plugin/server/reload.go:239-255`, `engine.go:121-130` | Plugin apply, config provider update, and subsystem reload are not one transaction. | Failed reload can leave participants on different configs. | Verify all first, apply with rollback across plugin server, config provider, and subsystems. |
| P1-30 | Reload autoload rollback | `plugin/server/reload.go:172-181,239-242`, `startup_autoload.go:141-231` | Newly autoloaded config-path plugins are not rolled back if later reload fails. | Plugins can run for unaccepted config. | Track and stop new plugins on reload failure. |
| P1-31 | Managed apply ack | `cmd/ze/main.go:837-851`, `managed/client.go:169-190`, `managed/handler.go:38-74` | Managed client validates and caches config, sends OK, but does not apply to runtime in the normal path. | Hub believes config is accepted while daemon runs old config. | Wire managed updates to reload transaction and ack only applied state. |

### P2 - Important Before Leaving Experimental

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P2-1 | ContextID overflow | `internal/component/bgp/context/registry.go:10-67` | 16-bit context ID silently wraps. | Old context IDs can map to new capabilities. | Use wider IDs or fail on exhaustion. |
| P2-2 | OPEN capability packing | `session_negotiate.go:119-169`, `capability.go:642-727` | Capability block over 255 bytes can be encoded with wrapped inner length. | Peers parse incomplete capabilities. | Split into multiple optional params or reject oversize. |
| P2-3 | ROUTE-REFRESH dispatch order | `session_read.go:203-222`, `session_handlers.go:278-317` | Ignored route-refresh messages can be delivered before handler ignores them. | Plugins can act on protocol-invalid refreshes. | Validate before callback dispatch. |
| P2-4 | RIB safety expiry | `rib_commands.go:39-54,761-819` | Auto-expire purges stale Adj-RIB-In without best-change cleanup. | Loc-RIB/FIB can retain deleted routes. | Reuse purge-stale reconciliation. |
| P2-5 | RIB text metadata | `rib_structured.go:314-346`, `rib.go:721-736`, `rib_commands.go:711-716` | Text sent path does not store `SourcePeer`. | GR stale propagation can miss text-delivered sent routes. | Fill `SourcePeer` from route metadata in text path. |
| P2-6 | MCP tool docs | `internal/component/mcp/handler.go:359-374`, `cmd/ze/help_ai.go:117,416-444`, `docs/guide/mcp/overview.md:158-163` | Docs/help advertise old handcrafted tools not present in `tools/list`. | MCP clients call unknown tools. | Generate docs from live tool list or re-add wrappers. |
| P2-7 | MCP mandatory params | `internal/component/mcp/tools.go:21-318`, `internal/component/l2tp/schema/ze-l2tp-api.yang:42-170` | Generated MCP schemas lose mandatory YANG inputs. | AI clients see required fields as optional. | Preserve mandatory params or split action tools. |
| P2-8 | REST convenience routes | `internal/component/api/rest/server.go:208-216` | Hardcoded commands drift from live registry. | REST routes break silently. | Build convenience routes from command metadata or add registry parity tests. |
| P2-9 | gRPC stream params | `internal/component/api/grpc/server.go:293-315` | Streaming path does not use unary parameter construction. | Future streaming will differ from REST/unary behavior. | Use shared `buildCommand` path. |
| P2-10 | Command inventory tool | `scripts/inventory/commands.go`, `internal/component/plugin/all/all.go` | `ze-command-list` can miss runtime imports. | Inventory/doc checks miss shipped command paths. | Share runtime import set or import `plugin/all`. |
| P2-11 | FIB kernel non-Linux | `internal/plugins/fib/kernel/backend_other.go:14-39` | Non-Linux backend is a silent noop. | Route installs appear to succeed without OS effect. | Reject unsupported platforms. |
| P2-12 | FIB kernel config | `ze-fib-conf.yang:20-36`, `fibkernel.go:278-307` | `flush-on-stop` and `sweep-delay` are accepted but ignored. | Operators configure knobs that do nothing. | Parse and wire the settings. |
| P2-13 | Firewall nft replace | `internal/plugins/firewall/nft/backend_linux.go:36-74` | Existing desired tables are not deleted before add. | Reloading changed rules can fail or leave old state. | Replace existing owned tables in one nft transaction. |
| P2-14 | VPP DPDK modules | `internal/component/vpp/dpdk.go:40-45`, `vpp.go:139-142` | VFIO modules load even with no DPDK interfaces. | VPP startup can fail unnecessarily. | Return early when no DPDK interfaces are configured. |
| P2-15 | Sysctl transaction | `internal/plugins/sysctl/sysctl.go:286-409` | Config apply can partially write kernel tunables, rollback warnings are best effort. | Failed commit can leave changed sysctls. | Preflight, stage, and make rollback failures visible. |
| P2-16 | PeeringDB URL | `internal/component/config/system/system.go:28-117`, `cmd/ze/hub/main.go:1416-1438` | Runtime resolver ignores configured PeeringDB URL. | Custom PeeringDB-compatible endpoints do not work. | Pass `sc.PeeringDBURL` to resolver construction. |
| P2-17 | Telemetry startup | `cmd/ze/hub/main.go:504-508` | Standalone `telemetry {}` still does not start telemetry without BGP. | Accepted config can be a no-op. | Start telemetry independently from BGP. |
| P2-18 | L2TP pool reload | `internal/plugins/l2tppool/register.go:88-103,182-193` | Pool reload can forget live allocations. | New sessions can receive duplicate subscriber IPs. | Migrate/reserve live allocations or reject reload with live sessions. |
| P2-19 | BFD first packet demux | `internal/plugins/bfd/engine/engine.go:175-197`, `loop.go:83-91`, `transport/socket.go:28-47` | Multi-hop first-packet lookup omits local destination. | Passive multi-hop sessions can stay down or collide. | Carry local destination into inbound packet metadata or reject unsupported shapes. |
| P2-20 | BFD Stop hang | `internal/plugins/bfd/engine/engine.go:262-307` | `Stop` can wait forever after failed `Start`. | Cleanup paths can deadlock. | Track successful run start and make Stop idempotent after failed start. |
| P2-21 | RADIUS reload | `internal/plugins/l2tpauthradius/acct.go:53-63,118-120`, `register.go:137-140` | Active interim loops can keep a closed old client after reload. | Interim accounting can stop until teardown. | Indirect sends through current client or restart loops on reload. |
| P2-22 | RADIUS CoA secrets | `internal/plugins/l2tpauthradius/register.go:155-157`, `coa.go:24-83` | CoA/DM validates only with first server secret. | Backup servers with distinct secrets cannot send valid CoA/DM. | Map source address to shared secret. |
| P2-23 | TACACS command format | `internal/component/tacacs/authorizer.go:52-55`, `accounting.go:164-196` | Whole command is sent as `cmd=...` instead of `cmd` plus `cmd-arg`. | TACACS policies may permit or deny incorrectly. | Encode shell arguments per TACACS convention. |
| P2-24 | External plugin goroutine leak | `internal/component/plugin/process/process.go:443-450,581-594`, `delivery.go:103-219` | Delivery loop starts before external connect-back and can leak on connect failure. | Failed plugin starts leak goroutines. | Stop and wait the process on start failure after delivery loop start. |
| P2-25 | Rollback backup | `internal/component/cli/editor.go:1019-1036` | Rollback backup uses cached editor content, not freshly read committed config. | Concurrent editor rollback can save wrong pre-rollback state. | Read current committed config under storage lock before backup. |
| P2-26 | Filesystem storage durability | `internal/component/config/storage/storage.go:186-227` | Rename lacks parent directory fsync. | Successful config writes can be lost after crash. | Fsync parent directory after rename. |

### P2 - Evidence, Tests, Docs

| ID | Area | Location | Finding | Impact | First Fix |
|----|------|----------|---------|--------|-----------|
| P2-27 | Default release gate | `Makefile:132-154`, `cmd/ze-test/*`, `test/static`, `test/traffic`, `test/l2tp-wire` | Default functional gate omits shipped suites and some tests are orphaned. | Release evidence excludes user-facing features. | Define and implement a release evidence matrix. |
| P2-28 | Fuzz inventory | `README.md:60`, `Makefile:190-235`, `internal/**/fuzz_test.go` | 57 fuzz funcs exist, but Makefile enumerates 43 and one name appears wrong. | `make ze-fuzz-test` is not evidence for documented fuzz scope. | Generate fuzz target list or make drift fail. |
| P2-29 | Known failures | `plan/known-failures.md:11-124` | Known flakes remain in full-suite evidence. | Rerun-until-green is not release evidence. | Fix or quarantine with explicit release status. |
| P2-30 | Observer assertions | `internal/test/runner/runner_validate.go:207-218`, `test/plugin/subsystem-list.ci`, `docs/ci-test-coverage.md` | Some observer `sys.exit(1)` failures are still not authoritative. | Coverage docs can overstate behavior tested. | Convert to `runtime_fail` or deterministic foreground assertions. |
| P2-31 | L2TP redistribution evidence | `docs/features.md:83`, `test/plugin/redistribute-l2tp-announce.ci`, `plan/deferrals.md:195` | Docs claim subscriber redistribution, but evidence uses fake producer and deferral says real producer path remains open. | Operators may expect real L2TP route export that is not proven. | Wire/test real L2TP producer or narrow docs. |
| P2-32 | Static route tests | `test/static/*.ci`, `cmd/ze-test/` | Static tests exist but no runner/gate is registered. | Shipped static routes lack functional gate evidence. | Add `ze-test static` and Make target. |
| P2-33 | L2TP wire tests | `test/l2tp-wire/*.ci`, `plan/deferrals.md:158` | `.ci` files exist but no runner is present. | Files look like evidence but do not run. | Add runner or move to spec artifacts. |
| P2-34 | Stale counts | `README.md:3,59`, `docs/guide/status.md:81`, `docs/features.md:61` | Counts are stale: inventory has 790 `.ci`; interop has 33 scenarios. | Public release claims drift from reality. | Generate counts from inventory. |
| P2-35 | `ze-ci` target | `Makefile:319-321` | `ze-ci` runs lint, unit, build only. | A release-named CI target skips functional/compat/fuzz evidence. | Rename to smoke or make it call the release gate. |
| P2-36 | Docs drift | `make ze-doc-test`, `docs/DESIGN.md` | 40 docs drift issues, mostly shipped plugin table and interop count. | Architecture docs are not trustworthy enough for release. | Update docs from live registry or remove static enumerations. |
| P2-37 | Security docs stale | `SECURITY.md:36-58`, `docs/architecture/fleet-config.md:300` | Docs still describe some fixed insecure defaults and miss current issues. | Operators get wrong trust-model guidance. | Rewrite security docs around current source and remaining risks. |
| P2-38 | Consistency tool scope | `scripts/lint/consistency.go`, `gokrazy/modcache/` | Consistency scan includes vendored module caches. | Tool output is noisy enough to hide real source issues. | Exclude vendor/gokrazy modcache/tmp and keep real source checks. |

### P3 - Cleanup And Hardening

| ID | Area | Finding | First Fix |
|----|------|---------|-----------|
| P3-1 | Large files | Many real source files exceed 1000 lines, including BGP reactor/RIB, CLI, iface, L2TP, MCP, web, firewall. | Split only when touching the area, after behavior is covered by tests. |
| P3-2 | Plugin structure | Consistency reports missing doc/schema or dispatch tests for several command plugin packages. | Decide whether command packages are plugins under the same rule, then fix or adjust the rule. |
| P3-3 | Docs style | Several static tables duplicate registries. | Derive docs from inventory where possible. |
| P3-4 | Gokrazy docs/tests | Appliance paths are broad and security-sensitive but not part of default release evidence. | Add appliance-specific smoke and init durability tests before appliance deployment. |

## Cross-Cutting Themes

1. The codebase has moved fast enough that registry-derived truth and handwritten docs/tests diverge constantly.
2. Identity propagation exists in more places than before, but authorization defaults are still too permissive.
3. Some hot paths still pass mutable buffers across goroutines without a clear ownership boundary.
4. Config is accepted before every participant has proven it can apply or roll back.
5. Several platform features still accept config that does not produce the promised dataplane state.
6. Test infrastructure itself is part of the release work. The current test matrix does not represent shipped surface area.

## Proposed Plan

### Phase 0 - Freeze And Restore Evidence

Objective: make the evidence honest and repeatable before fixing deeper behavior.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 0.1 | Fix current lint failures | `make ze-verify-fast` reaches unit tests. |
| 0.2 | Fix unit red packages | `make ze-unit-test` passes with race detector. |
| 0.3 | Fix functional red suites | `make ze-functional-test` passes encode, plugin, parse, decode, reload, ui, editor, managed. |
| 0.4 | Fix docs and command validation | `make ze-doc-test` passes with zero missing handlers. |
| 0.5 | Repair consistency scope | `make ze-consistency` excludes dependency caches and reports actionable source-only findings. |
| 0.6 | Define release evidence matrix | Every shipped suite is either gated, privileged-gated, optional, or explicitly non-evidence. |

### Phase 1 - Close Security Boundaries

Objective: make remote and authenticated control safe enough for a pilot.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 1.1 | Authz fail-closed | Empty usernames, unassigned users, and missing profiles deny unless explicitly configured. |
| 1.2 | MCP identity and scopes | MCP dispatch carries identity and restricted users cannot run privileged commands. |
| 1.3 | SSH controls | Wrong-key auth regression fixed; `plugin protocol` and lifecycle actions are admin-gated. |
| 1.4 | API auth | Remote no-auth API startup is rejected; token principal has explicit profile. |
| 1.5 | L2TP/RADIUS/TACACS auth | PPP fails closed, Accounting-Stop is reliable, CoA secrets map by server, TACACS args are correct. |
| 1.6 | Security docs | `SECURITY.md` and operations docs match current trust model and known opt-in insecure modes. |

### Phase 2 - Fix BGP State And Buffer Correctness

Objective: make the core routing path safe under load and protocol edge cases.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 2.1 | Buffer ownership | Race in `session_coalesce`/plugin delivery eliminated, with race test evidence. |
| 2.2 | WireUpdate concurrency | Lazy parsing is immutable or synchronized. |
| 2.3 | Malformed UPDATE gating | Invalid section lengths never reach plugin/cache callbacks. |
| 2.4 | Cache ack lifecycle | Failed deliveries cannot pin cache entries. |
| 2.5 | LLGR and stale reconciliation | LLGR timers, community commands, safety expiry, and text metadata update Loc-RIB/FIB correctly. |
| 2.6 | Route refresh semantics | Capability gating, subtype handling, and enhanced markers match RFC behavior. |
| 2.7 | ADD-PATH persistence | Persist stores and replays distinct path IDs. |
| 2.8 | Capability/context edge cases | Context exhaustion and oversize OPEN capabilities reject or encode safely. |

### Phase 3 - Make Config And Lifecycle Transactional

Objective: failed reloads, managed updates, plugin starts, and storage writes must leave one coherent state.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 3.1 | Reload transaction | Plugin server, config provider, and subsystems verify/apply/rollback as one transaction. |
| 3.2 | Autoload rollback | Plugins started for failed reloads are stopped and removed. |
| 3.3 | Managed apply semantics | Managed ACK means runtime applied, or response says cached-pending. |
| 3.4 | ZeFS durability | Flush failure restores old in-memory state; successful write is fsync-durable. |
| 3.5 | Init atomicity | First-boot DB is built at temp path and renamed only once complete. |
| 3.6 | Process leaks | External plugin connect-back failure cleans delivery goroutines. |

### Phase 4 - Fix Platform And Dataplane Exactness

Objective: accepted platform config must take effect or fail visibly.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 4.1 | VPP lifecycle | Reload, validation, DPDK no-interface behavior, and socket settings are applied transactionally. |
| 4.2 | Backend switching | iface, traffic, firewall keep old backend alive until new apply succeeds. |
| 4.3 | Reconcile orphans | traffic qdiscs, nft tables, sysctls, and FIB routes reconcile full desired state. |
| 4.4 | Unsupported platforms | FIB/static/platform backends reject unsupported platforms rather than no-op. |
| 4.5 | Runtime wiring | PeeringDB URL and standalone telemetry work as configured. |

### Phase 5 - Finish Session Protocol Safety

Objective: L2TP, PPP, BFD, and RADIUS must survive reload and churn.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 5.1 | L2TP pools | Reload cannot duplicate live subscriber IPs. |
| 5.2 | L2TP teardown commands | Advertised teardown commands are routed and tested end-to-end. |
| 5.3 | BFD demux/stop | Multi-hop first-packet demux is exact, failed Start followed by Stop cannot hang. |
| 5.4 | PPP/PAP regression | Wrong protocol on PAP handler returns deterministically. |
| 5.5 | RADIUS reload | Interim loops migrate or use the current client after reload. |

### Phase 6 - Publish Truthful Release Docs

Objective: user-facing docs state what is shipped, what is gated, and what is deferred.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 6.1 | Registry-derived docs | Plugin/command/RPC/count tables are generated or checked against inventory. |
| 6.2 | Feature status | L2TP, static, traffic, VPP, API streaming, MCP, BFD, and telemetry docs reflect current gates. |
| 6.3 | Known failures | `plan/known-failures.md` contains no unqualified release-gate failures. |
| 6.4 | Release manifest | A generated manifest lists test suites, fuzz targets, interop scenarios, skipped tests, and known caveats. |

## Suggested Spec Set

| Suggested Spec | Scope |
|----------------|-------|
| `spec-release-0-gate-trust.md` | Lint, unit, functional, doc-test, consistency scope, suite matrix, fuzz inventory. |
| `spec-release-1-security-boundaries.md` | Authz fail-closed, MCP identity, API auth, SSH plugin protocol, SSH key regression, L2TP/RADIUS/TACACS auth. |
| `spec-release-2-bgp-buffer-state.md` | Receive buffer ownership, WireUpdate concurrency, malformed UPDATE gating, cache ack lifecycle. |
| `spec-release-3-bgp-routing-semantics.md` | LLGR/stale reconciliation, route refresh, ADD-PATH persist, ContextID/OPEN edge cases. |
| `spec-release-4-transactional-lifecycle.md` | Reload, plugin autoload rollback, managed apply, ZeFS, init, external process leaks. |
| `spec-release-5-platform-dataplane.md` | VPP, iface, traffic, firewall, FIB, sysctl, telemetry, resolve. |
| `spec-release-6-session-protocols.md` | L2TP pool/teardown, PPP/PAP, BFD demux/Stop, RADIUS reload/accounting. |
| `spec-release-7-docs-manifest.md` | Registry-derived docs, security docs, feature status, release evidence manifest. |

## Recommendation

Do not leave experimental status yet.

Minimum bar for an initial controlled deployment:

- All P0 findings closed.
- `make ze-verify-fast`, `make ze-unit-test`, `make ze-functional-test`, and `make ze-doc-test` green.
- Security phase closed for SSH, API, MCP, authz defaults, L2TP PPP auth, and RADIUS accounting.
- BGP receive-buffer race and malformed UPDATE delivery fixed with race and malformed-input tests.
- Reload and managed config semantics made transactional enough that failed changes do not split runtime state.
- Release docs rewritten to match the live registry and actual gate matrix.

After that, a narrow pilot can be considered with documented exclusions for privileged integration, VPP, live RPKI, interop, and appliance tests if those are not in the pilot scope. Broader deployment should wait for platform/dataplane and session-protocol phases to close.
