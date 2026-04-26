# Release Readiness Review - 2026-04-23

## Scope

This document consolidates a full-project review for release readiness and first deployment.

- Review mode only. No code was changed.
- Forked review passes covered control-plane wiring, BGP reactor paths, BGP plugins and RIB, operator surfaces, platform integrations, session protocols, tooling and test infrastructure, and a final runtime/platform sweep.
- The worktree was already dirty when this review started. Those changes were not modified.

## Method

- Read project rules, architecture docs, status docs, feature inventory, prior audit material, and current known-failure tracking.
- Mapped repository structure and current scope.
- Ran forked bundle reviews with traced entry paths and file:line evidence.
- Ran `make ze-verify-fast`.

## Repository Facts

Current tree facts gathered during this review:

| Metric | Value |
|-------|-------|
| Go files in repo | 5466 |
| Primary source Go files (`internal cmd pkg api`) | 2583 |
| Go test files | 987 |
| `.ci` files under `test/` | 789 |
| `.et` editor tests | 145 |
| `.wb` web tests | 58 |
| Docs files under `docs/` | 190 |

## Gate State

`make ze-verify-fast` is currently red before any tests run.

It stops in lint with 20 issues, including:

- `internal/plugins/policyroute/{register.go,marks.go,model.go,rules_other.go}`
- `internal/plugins/static/{config.go,backend.go,diff_test.go}`
- `internal/component/bgp/plugins/rs/server_withdrawal.go`
- `internal/component/firewall/registry.go`

This is not the main release risk, but it means the default fast gate is already failing before deeper runtime evidence is considered.

## Confirmed Findings

### P0 - Release Blockers

| ID | Area | Type | Location | Finding | Impact | First Fix |
|----|------|------|----------|---------|--------|-----------|
| P0-1 | BGP RPKI / ADD-PATH | correctness | `internal/component/bgp/plugins/rpki/rpki.go:365-370,507-515`, `internal/component/bgp/plugins/adj_rib_in/rib.go:422-426,458-463`, `internal/component/bgp/plugins/adj_rib_in/rib_commands.go:158-166,186-193` | ADD-PATH RPKI validation drops `pathID` and rebuilds keys as `pathID=0`. | Wrong accept/reject decisions or permanently pending routes on ADD-PATH sessions. | Carry `pathID` end-to-end through validation requests and handlers, then add ADD-PATH RPKI tests. |
| P0-2 | BGP RIB command path | wiring / correctness | `internal/component/bgp/plugins/rib/rib_commands.go:282-290,331-364,473-490,629-653,745-772`, `internal/component/bgp/plugins/rib/rib_structured.go:184-209`, `internal/component/bgp/plugins/rib/rib.go:937-957`, `internal/component/bgp/plugins/rib/rib_bestchange.go:694-809,989-1002` | Command-driven RIB mutations bypass best-change, Loc-RIB, and EventBus update paths. | Operator or GR commands can change Adj-RIB-In while Loc-RIB, system RIB, FIB, and consumers stay stale. | Route all command mutations through the same mutation path used by received updates. |
| P0-3 | Parse test runner | tooling trust | `internal/test/runner/parsing.go:170-275,361-415` | Parse runner ignores large parts of `.ci` semantics, including `cmd`, `tmpfs`, stdout assertions, and reject logic, but the files still read like full tests. | A large part of the parse suite is false evidence and should not be used for release confidence. | Replace the bespoke parse runner or make it execute full `.ci` semantics with coverage for all directives. |
| P0-4 | Parse test runner | tooling trust | `internal/test/runner/parsing.go:242-245` | Repeated `expect=stderr:contains=` lines overwrite each other instead of accumulating. | Negative parse tests can pass while only validating the last expected error fragment. | Accumulate all stderr expectations and assert them all. |

### P1 - High Risk Before Any Initial Deployment

| ID | Area | Type | Location | Finding | Impact | First Fix |
|----|------|------|----------|---------|--------|-----------|
| P1-1 | Remote CLI | security | `cmd/ze/internal/ssh/client/client.go:338-349` | Remote SSH client uses `ssh.InsecureIgnoreHostKey()`. | Remote CLI and control traffic are MITM-able. | Require `known_hosts` or explicit fingerprint pinning for remote targets. |
| P1-2 | Config reload | correctness / transactionality | `internal/component/plugin/server/reload.go:160-180`, `internal/component/plugin/server/startup_autoload.go:305-317,390-393` | Reload stops config-path plugins before verify/apply succeeds. | Runtime can diverge from persisted config when later verification fails. | Make config-path plugin stop/start transactional. |
| P1-3 | BGP forward path | correctness / concurrency | `internal/component/bgp/reactor/reactor_api_forward.go:447-465`, `internal/component/bgp/reactor/received_update.go:28-38` | Per-peer export rewrite mutates shared `ReceivedUpdate`. | One peer's policy rewrite can leak into other peers and later forwards. | Keep per-peer local wire data, never mutate cached `ReceivedUpdate`. |
| P1-4 | Structured event pool | memory safety / concurrency | `internal/component/bgp/server/events.go:204-240,545-572`, `internal/component/plugin/process/delivery.go:191-196` | `StructuredEvent` objects are returned to the pool in both caller and callee. | Double put can cause pooled-object aliasing and hard-to-reproduce corruption. | Give ownership to one layer only, preferably delivery code. |
| P1-5 | BGP cache counts | lifecycle / backpressure | `internal/component/bgp/server/events.go:299-305,321-334` | Batch activation counts cache consumers before checking whether delivery actually succeeded. | Failed plugin delivery can pin cache entries waiting for acks that will never come. | Count only successful cache-consumer deliveries in the batch path. |
| P1-6 | BGP stale marking | correctness | `internal/component/bgp/plugins/rib/rib_commands.go:695-702`, `internal/component/bgp/route.go:47-56`, `internal/component/bgp/plugins/rib/rib_structured.go:324-339` | `bgp rib mark-stale` marks all outbound routes stale, not only those sourced from the restarting peer. | GR or LLGR state can contaminate unrelated outbound routes. | Track source peer identity in outbound route state and scope stale propagation. |
| P1-7 | Best path tie-break | correctness | `internal/component/bgp/plugins/rib/bestpath.go:79-93,364-377`, `internal/component/bgp/plugins/rib/rib_commands.go:876-880` | Step 7 tie-break uses incomplete metadata and can fall back to peer address instead of Router ID / ORIGINATOR_ID. | Reflected vs non-reflected paths can be chosen incorrectly. | Carry Router ID into candidates and implement the documented tie-break fully. |
| P1-8 | Role / OTC | correctness | `internal/component/bgp/plugins/role/otc.go:24-29,167-169,301-309`, `internal/component/bgp/reactor/reactor_notify.go:337-341` | Malformed OTC is treated as a drop, not a treat-as-withdraw. | A bad update can leave an older route installed instead of being withdrawn. | Add an explicit treat-as-withdraw outcome in the filter path. |
| P1-9 | Web admin, web L2TP, MCP | authz bypass | `cmd/ze/hub/main.go:851-858`, `internal/component/plugin/server/command.go:90-97`, `internal/component/web/handler_admin.go:125-147`, `internal/component/web/handler_l2tp.go:293-330`, `internal/component/mcp/streamable.go:1105-1143`, `internal/component/mcp/handler.go:239` | Dispatcher calls from web admin, web L2TP, and MCP drop username and remote-address context. | Authenticated but restricted operators can execute commands outside their profile on those surfaces. | Pass caller identity and address through `CommandContext` for all non-SSH surfaces. |
| P1-10 | SSH lifecycle commands | privilege boundary | `internal/component/ssh/ssh.go:540-584` | Any authenticated SSH user can stop, restart, or reboot before authz runs. | Low-privilege accounts can terminate the daemon or reboot the host. | Gate lifecycle operations behind normal authorization or a privileged capability. |
| P1-11 | API streaming | unsupported feature exposed as supported | `cmd/ze/hub/api.go:161-168`, `internal/component/api/engine.go:123-130`, `docs/features.md:62`, `docs/guide/api.md:77,165` | REST SSE and gRPC streaming are documented, but production wiring passes `nil` stream source. | Published streaming workflows do not work in a real daemon. | Either wire a real stream source or remove the claim and endpoints. |
| P1-12 | Managed transport | security | `internal/component/managed/client.go:81-90`, `cmd/ze/main.go:854-859` | Managed TLS uses `InsecureSkipVerify: true`. | Hub impersonation and config injection are possible over untrusted networks. | Make certificate verification mandatory by default. |
| P1-13 | Managed bootstrap | exact-or-reject | `cmd/ze/main.go:778-789,834-841`, `internal/component/managed/handler.go:50-68` | First-boot managed config is written to storage before parse/verify. | Invalid remote config can poison bootstrap state. | Run the normal validation path before first cache write. |
| P1-14 | Backend switch rollback | rollback bug | `internal/component/traffic/register.go:269-299`, `internal/component/traffic/backend.go:136-157`, `internal/component/firewall/engine.go:249-277`, `internal/component/firewall/backend.go:87-109` | Traffic and firewall backend changes close the old backend before the new apply is proven. | Failed backend switches can leave dataplane state split across old and new mechanisms. | Keep the previous backend alive until new apply succeeds. |
| P1-15 | Static routes | correctness / exact-or-reject | `internal/plugins/static/register.go:81-96,101-137`, `internal/plugins/static/inject.go:56-77,122-146`, `internal/plugins/static/backend_other.go:9-13` | Static route programming failures are logged but not returned, and non-Linux backend is a silent noop. | Commits can report success while no static routes are installed. | Propagate backend failures and reject unsupported platforms instead of nooping. |
| P1-16 | Interface backend reload | correctness | `internal/component/iface/register.go:399-426,277-293` | Reload does not switch the active interface backend even if config changes from `netlink` to `vpp`. | Runtime backend can differ from committed config and from backend-gating assumptions. | Detect backend changes and load the new backend transactionally on reload. |
| P1-17 | BFD timers | correctness | `internal/plugins/bfd/session/fsm.go:55-61`, `internal/plugins/bfd/session/timers.go:19-42` | Detection interval uses peer `RequiredMinRx` instead of peer `DesiredMinTx`. | Asymmetric peers can flap early or detect failure too late. | Store remote desired-TX separately and use it in detection arithmetic. |
| P1-18 | L2TP PPP-down cleanup | resource leak | `internal/component/l2tp/reactor.go:978-1020`, `internal/component/l2tp/session.go:198-209`, `internal/component/l2tp/teardown.go:97-112` | PPP-driven session teardown does not drain pending kernel teardowns like operator-driven teardown does. | Kernel L2TP and PPP resources can leak across churn. | Reuse the same kernel teardown enqueue path on PPP-down. |
| P1-19 | PAP auth phase | liveness / correctness | `internal/component/ppp/pap.go:154-177`, `internal/component/ppp/session_run.go:733-760,341-356` | PAP waits forever for the first PAP request and treats stray non-PAP frames as fatal. | Sessions can hang indefinitely or fail spuriously during auth. | Bound the pre-request wait and route non-PAP frames through normal frame handling. |
| P1-20 | VPP startup config | unsupported feature / deployment blocker | `internal/component/vpp/startupconf.go:23-27,94-97`, `internal/component/vpp/config.go:24,60,257,271` | Configured VPP API and stats socket paths are parsed but never written into generated `startup.conf`. | Managed VPP startup or telemetry can break whenever non-default socket paths are configured. | Emit configured socket paths into generated `startup.conf`. |
| P1-21 | FIB VPP cold boot | race / blackhole | `internal/component/vpp/register.go:100-115`, `internal/plugins/fib/vpp/register.go:119-139` | `fib-vpp` falls back to noop backend on cold start and only listens for `vpp.reconnected`, not the first `vpp.connected`. | Route installation into VPP can stay permanently inert after boot. | Wait for initial VPP connectivity or recreate backend on both connected and reconnected. |
| P1-22 | Policy route startup | rollback bug | `internal/plugins/policyroute/register.go:72-85,169-195` | Firewall state is applied before kernel rules, and failed startup does not roll back already-applied firewall state. | Failed startup can leave live marking or MSS changes behind. | Journal startup like reload or explicitly roll back firewall state on failure. |
| P1-23 | Coverage docs | misleading evidence | `docs/ci-test-coverage.md:13-19,27-29`, cited `.ci` files using observer `sys.exit(1)` | Coverage doc marks cases closed while cited tests still use the observer-exit pattern that the runner does not treat as authoritative failure. | Coverage closure claims are overstated. | Re-audit every cited covered file and migrate them to `runtime_fail` or deterministic production-log assertions. |
| P1-24 | Default release gate | evidence gap | `Makefile:132-154`, `cmd/ze-test/{l2tp,firewall,traffic,vpp,web}.go`, `docs/functional-tests.md:1066-1075` | `ze-verify` omits shipped L2TP, firewall, traffic, VPP, and web suites, while docs suggest broader coverage and even mention a non-existent `make ze-l2tp-test`. | The default gate does not exercise several user-facing subsystems. | Either add those suites to the release gate or document them as non-gating and not deployment evidence. |
| P1-25 | Security docs | misleading trust model | `cmd/ze/main.go:854-857`, `cmd/ze/internal/ssh/client/client.go:338-349`, `docs/architecture/fleet-config.md:294-303`, `SECURITY.md:7-29` | Current docs do not clearly disclose that remote managed TLS and remote SSH control lack host trust verification. | Operators can assume secure remote control paths that are currently insecure. | Update security and operations docs immediately, even before code fixes land. |

### P2 - Important Before Leaving Experimental

| ID | Area | Type | Location | Finding | Impact | First Fix |
|----|------|------|----------|---------|--------|-----------|
| P2-1 | Environment config | exact-or-reject | `internal/component/config/apply_env.go:33-40`, `internal/component/bgp/config/loader_create.go:254-266`, `internal/component/bgp/reactor/update_group.go:62-66`, `internal/component/bgp/reactor/session_coalesce.go:43-46`, `docs/architecture/config/environment-block.md:57-60` | `environment { chaos {} reactor {} }` leaves are documented but ignored at runtime. | Accepted operator config silently does nothing. | Wire every documented leaf or reject unsupported ones. |
| P2-2 | Plugin hub config | unsupported feature exposed as supported | `cmd/ze/hub/main.go:346-364`, `internal/component/plugin/manager/manager.go:33,74-78,221-267`, `docs/guide/configuration.md:966-994`, `docs/features.md:66` | Hub config is parsed but not used for local external-plugin startup, and only one server listener is honored. | Plugin-hub address and multi-listener behavior differ from config and docs. | Pass extracted hub config into the process manager and either support or reject multi-listener use. |
| P2-3 | `bgp rib show` | operator correctness | `internal/component/bgp/plugins/rib/rib_nlri.go:156-163`, `internal/component/bgp/plugins/rib/rib_pipeline.go:107-116`, `internal/component/bgp/plugins/rib/rib_pipeline_best.go:58-63` | Show paths use an NLRI formatter that only supports IPv4 and IPv6 unicast without ADD-PATH. | Operator output is incomplete or wrong for complex families and ADD-PATH. | Use family-aware decoders for show surfaces. |
| P2-4 | API config sessions | session lifetime | `internal/component/api/config_session.go:41-99`, `cmd/ze/hub/api.go:34`, `docs/guide/api.md:125-127` | Sessions never expire, and the documented idle timeout is not implemented. | Stale candidate sessions can accumulate indefinitely. | Add last-activity tracking and a cleanup loop. |
| P2-5 | gRPC validation parity | input validation | `internal/component/api/grpc/server.go:298-305,478-495`, `internal/component/api/rest/server.go:315-334,700-709`, `docs/guide/api.md:257-266` | gRPC parameter construction is weaker than REST and accepts inputs REST rejects. | Transport-dependent command injection behavior. | Share one validator between REST and gRPC. |
| P2-6 | Web audit trail | operator attribution | `internal/component/web/handler_l2tp.go:291-330`, `docs/features.md:83` | Web L2TP disconnect always records `actor web`, not the authenticated user. | Audit trail is not user-attributed. | Include username in dispatch or audit metadata. |
| P2-7 | TACACS command auth | auth context | `internal/component/plugin/server/command.go:340-348`, `internal/component/aaa/aaa.go:29-30`, `internal/component/tacacs/authorizer.go:43-55`, `internal/component/tacacs/author.go:28-36` | TACACS authorization does not receive `RemoteAddr` even though authn and accounting do. | Per-command policy cannot depend on source address. | Extend authorizer context to carry `RemoteAddr`. |
| P2-8 | BFD multi-hop keying | unsupported feature exposed as supported | `internal/plugins/bfd/api/events.go:127-143`, `internal/plugins/bfd/engine/engine.go:175-180,191-197,381-383`, `internal/plugins/bfd/schema/ze-bfd-conf.yang:213-243` | Multi-hop session uniqueness and first-packet demux ignore local identity. | Multiple sessions to the same peer in one VRF can collide or mis-deliver early packets. | Reject colliding sessions or extend the demux key. |
| P2-9 | Resolve runtime wiring | dead runtime path | `cmd/ze/hub/main.go:1401-1421`, `internal/component/resolve/cmd/resolve.go:43-46,199-285`, `internal/component/config/system/system.go:105-121`, `docs/architecture/resolve.md:27-29,54-65` | Runtime resolve surfaces expose PeeringDB and IRR handlers, but hub startup only builds DNS and Cymru resolvers. | Hub, MCP, and web resolve commands can advertise support but always fail. | Construct and inject PeeringDB and IRR resolvers in hub startup. |
| P2-10 | Telemetry startup | feature wiring gap | `internal/component/bgp/config/loader_create.go:200-223`, `README.md:22`, `docs/guide/monitoring.md:135`, `docs/features.md:76` | Prometheus server and OS collectors only start through the BGP loader path. | `telemetry {}` is effectively a no-op unless `bgp {}` is also present. | Start telemetry independently of BGP. |
| P2-11 | VPP FIB tuning | unsupported config | `internal/plugins/fib/vpp/fibvpp.go:59-63,101-117`, `docs/guide/vpp.md:209-210` | `batch-size` and `batch-interval-ms` are accepted but explicitly ignored. | Operators can tune settings that do nothing. | Implement batching or reject and document them as unsupported. |
| P2-12 | VPP DPDK cleanup | cleanup bug | `internal/component/vpp/dpdk.go:57-69,97-105,157-160`, `docs/guide/vpp.md:95,101` | Ze adds `vfio-pci/new_id` entries but does not remove them on teardown. | Future rescans or hotplug can bind more devices to `vfio-pci` than intended. | Use `driver_override` or remove `new_id` on teardown. |
| P2-13 | API completion | parity gap | `internal/component/api/rest/server.go:200-234`, `internal/component/api/grpc/server.go:359-362`, `docs/guide/api.md:78,168` | Completion is missing or stubbed on API surfaces. | CLI and web have capability discovery parity that the API does not. | Wire completion through the dispatcher or stop advertising it. |

### P3 - Cleanup, Docs, and Evidence Debt

| ID | Area | Type | Location | Finding | Impact | First Fix |
|----|------|------|----------|---------|--------|-----------|
| P3-1 | Architecture docs | docs drift | `docs/architecture/subsystem-wiring.md:8-10`, `cmd/ze/hub/main.go:404-430`, `README.md:7`, `docs/guide/status.md:98` | Startup ownership, subsystem wiring, and auto plugin discovery docs no longer match the live code. | Reviewers and operators are working from stale architecture descriptions. | Rewrite startup and plugin-autoload docs from current code. |
| P3-2 | Test race claims | docs drift | `docs/guide/status.md:76-86`, `Makefile:109-117,131-154,190-259` | Docs say all tests run with `-race`, but many functional, browser, and compat suites do not. | Concurrency coverage is overstated. | Narrow the claim or expand the gate. |
| P3-3 | BGP text API docs | docs drift | `docs/architecture/api/text-format.md`, `docs/architecture/api/text-parser.md`, `docs/architecture/api/commands.md:494-497`, `docs/architecture/api/update-syntax.md:879-881` | API text format and parser docs describe older behavior and disagree on `watchdog set`. | User and developer docs are internally contradictory. | Refresh API text docs from current parser and handlers. |
| P3-4 | TACACS docs | docs drift | `docs/guide/tacacs.md:14,118-126,151-153`, `test/plugin/tacacs-author.ci`, `cmd/ze/tacacs/main.go:58-93` | Guide says TACACS authorization test and observability are not there, but both exist. | Release docs understate current surface and evidence. | Update the guide to match shipped commands and tests. |
| P3-5 | BFD docs | docs drift | `docs/architecture/bfd.md:3-8,168-188`, `docs/guide/bfd.md:113-122`, `docs/features.md:22` | Docs still describe BFD as skeletal and defer work that is now shipped. | Status pages are unreliable and can hide the real remaining gaps. | Rewrite BFD docs around the current implementation and actual open issues. |
| P3-6 | Functional test docs | docs drift | `docs/functional-tests.md:95-102,294-331,1070`, `docs/guide/status.md:78-85`, `README.md:3,53-61` | Runner names, counts, limits, and Make targets are stale. | Documentation no longer reflects the real test harness. | Regenerate counts and rewrite functional-test docs from the current toolchain. |
| P3-7 | Web security headers | hardening gap | `internal/component/web/auth.go:176-177,191-192,200-202,227-230,267-275` | Security headers are added for authenticated responses, but login failures and other unauthenticated responses skip `addSecurityHeaders`. | The web surface has inconsistent clickjacking, CSP, HSTS, and cache-control behavior on exactly the pages most likely to be exposed before login. | Apply the same security-header set to login and 401 responses too. |
| P3-8 | Resolve config path | config ignored | `cmd/ze/hub/main.go:1401-1411`, `internal/component/resolve/dns/resolver.go:49-52,65-73`, `docs/architecture/resolve.md:67-71` | `system.dns.resolv-conf-path` is extracted and documented but ignored by runtime resolver construction. | Runtime resolve commands can query the wrong upstream resolvers. | Honor configured `resolv-conf-path` in runtime resolver setup. |

## Tests And Docs That Are Not Safe As Full Evidence Yet

These should be treated as partial or non-authoritative until reworked:

| Path | Problem |
|------|---------|
| `test/plugin/nexthop-self-ipv6-forward.ci` | Self-declared partial, forwarding proof blocked by fixture limitation. |
| `test/plugin/llgr-readvertise.ci` | Self-declared partial, non-LLGR suppression not proven. |
| `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Self-declared partial, does not compare forwarded bytes. |
| `test/plugin/policy-show-list.ci` | Self-declared blocked by framework issue. |
| `test/plugin/rr-ipv6-config.ci` | Self-declared blocked by single-peer fixture limitation. |
| `test/plugin/gr-cli-restart.ci` | Self-declared partially demonstrated only. |
| `test/plugin/rib-best-selection.ci` | Uses `bgp rib inject`, does not prove real receive path. |
| `test/plugin/bestpath-reason.ci` | Uses `bgp rib inject`, not wire-driven best-path evidence. |
| `docs/ci-test-coverage.md` | Overstates closure while parse suite and observer-exit problems remain open. |
| `plan/known-failures.md` | Still tracks active flakes and partial or blocked cases that must stay in release decisions. |

## Cross-Cutting Themes

1. Security trust boundaries are not yet deployment-safe.
   Remote SSH control, managed TLS, and several non-SSH command paths still trust too much or drop caller identity.

2. The release evidence is weaker than the docs suggest.
   The parse runner is structurally unsound, the default `ze-verify` gate omits several shipped suites, and counts or coverage claims are stale.

3. There are still silent no-op and accepted-but-ignored configuration paths.
   This shows up in `environment {}`, VPP FIB tuning, plugin-hub listener config, telemetry-without-BGP, and resolver config.

4. Several state mutation paths bypass the authoritative data flow.
   The worst examples are BGP command-driven RIB updates, per-peer forward rewrites mutating shared state, and reload-time runtime mutation before transaction success.

5. Platform and dataplane rollback semantics need hardening.
   Traffic, firewall, policy-route, static-route, and VPP-managed paths still have rollback or cleanup gaps.

## Proposed Plan

### Phase 0 - Restore Trust In The Evidence

Objective: make the release gate and documentation truthful before relying on them.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 0.1 | Fix current `make ze-verify-fast` lint failures | Fast gate runs cleanly to the test phase. |
| 0.2 | Repair or replace parse runner | Parse tests execute the behavior their files claim to test. |
| 0.3 | Re-audit `docs/ci-test-coverage.md` and observer-exit tests | Coverage doc only cites valid evidence. |
| 0.4 | Update README, status, functional-test docs, security docs | Counts, gating claims, and trust model match the tree. |

### Phase 1 - Close Security And Operator Trust Gaps

Objective: remove the trust-model blockers for any initial deployment.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 1.1 | Remote SSH host-key verification | Remote CLI refuses unknown or untrusted hosts by default. |
| 1.2 | Managed TLS verification and bootstrap validation | Managed config channel verifies peers and validates first-boot config before write. |
| 1.3 | Web, MCP, and web-L2TP authz context propagation | All dispatcher paths carry identity and remote address. |
| 1.4 | SSH stop/restart/reboot authorization | Only privileged identities can invoke lifecycle actions. |
| 1.5 | REST/gRPC parity and API session expiry | Transport validation is shared and sessions expire as documented. |

### Phase 2 - Fix Core Control-Plane And BGP Correctness

Objective: make the main routing state machine trustworthy.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 2.1 | RPKI ADD-PATH pathID handling | ADD-PATH validation decisions are correct and tested. |
| 2.2 | RIB command-path unification | All RIB mutations update best path, Loc-RIB, FIB, and event consumers consistently. |
| 2.3 | Forward path state ownership | No shared `ReceivedUpdate` mutation, no structured-event double pool return, no cache overcounting. |
| 2.4 | Best-path, GR/LLGR, OTC semantics | Tie-break, stale marking, and treat-as-withdraw behavior match docs and RFC intent. |
| 2.5 | Reload transactionality and env parity | Runtime changes happen only after verify passes, and documented env leaves are either wired or rejected. |

### Phase 3 - Fix Platform Runtime And Dataplane Wiring

Objective: make accepted config produce the intended dataplane effect.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 3.1 | Interface, traffic, firewall, policy-route backend switching and rollback | Backend switches are transactional and reversible. |
| 3.2 | Static-route error propagation | Failed route programming fails startup or commit visibly. |
| 3.3 | VPP startup and FIB wiring | Socket-path config works, cold boot connects cleanly, ignored knobs are gone or implemented. |
| 3.4 | Resolve and telemetry wiring | PeeringDB and IRR resolve paths are live, resolver config is honored, telemetry starts independently of BGP. |
| 3.5 | VPP DPDK cleanup | Managed VPP startup and shutdown do not leave the host in a widened vfio state. |

### Phase 4 - Fix Session Protocol Semantics

Objective: make L2TP, PPP, and BFD safe for real churn and failure conditions.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 4.1 | BFD timer semantics and session-key collision handling | Timers match RFC intent and multi-hop uniqueness is explicit. |
| 4.2 | L2TP PPP-driven teardown cleanup | PPP-down fully tears down kernel session state. |
| 4.3 | PAP pre-request timeout and stray-frame handling | Sessions cannot hang forever waiting for the first PAP frame. |
| 4.4 | Add stateful regression coverage | Timer, teardown, and auth-window bugs are covered end-to-end. |

### Phase 5 - Final Release Gate For First Deployment

Objective: define the real go-live bar.

| Step | Scope | Exit Criteria |
|------|-------|---------------|
| 5.1 | Bring shipped suites into the release decision | Either `ze-verify` runs them or they are explicitly non-gating. |
| 5.2 | Clear P0 and P1 findings | No P0 and no accepted P1 left open for the intended deployment scope. |
| 5.3 | Re-run full evidence set | `make ze-verify-fast`, selected privileged suites, relevant interop and chaos runs are green. |
| 5.4 | Publish truthful status docs | README, status, security, and feature docs describe what is real today. |

## Suggested Spec Set

To execute this cleanly, split remediation into a related spec set instead of one giant patch stream.

| Suggested Spec | Scope |
|----------------|-------|
| `spec-release-0-gate-trust.md` | Parse runner, coverage docs, gate definition, lint baseline, release docs truthfulness |
| `spec-release-1-remote-trust.md` | Remote SSH trust, managed TLS trust, bootstrap validation, non-SSH authz context |
| `spec-release-2-bgp-state-correctness.md` | RPKI ADD-PATH, RIB mutation path, forward ownership, best-path, GR and OTC fixes |
| `spec-release-3-platform-runtime.md` | Interface/backend switching, static-route failures, traffic/firewall rollback, VPP runtime fixes, resolve, telemetry |
| `spec-release-4-session-protocols.md` | BFD timer and keying, L2TP teardown, PAP auth-window fixes |
| `spec-release-5-evidence-and-docs.md` | Remaining partial tests, suite gating, status and feature docs, known-failures closeout |

## Recommendation

Do not treat the project as ready to leave experimental status yet.

Minimum bar for an initial controlled deployment:

- All P0 findings fixed.
- P1 security and authz findings fixed.
- BGP command-path and ADD-PATH correctness fixed.
- Release evidence made trustworthy, especially parse tests and omitted suites.
- Status and security docs rewritten so operators know what is real and what is still partial.

After that, a narrow pilot deployment becomes reasonable. Broader deployment should wait for the platform-runtime and session-protocol phases to close.
