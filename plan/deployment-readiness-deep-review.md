# Deployment Readiness Deep Review

Date: 2026-04-28
Scope: whole repository review for moving Ze out of experimental status.

Method:

- Initial repository map: architecture docs, feature inventory, Makefile gates, known failures, deferrals, schemas, components, plugins, and test surface.
- Forked subsystem agents: BGP, config/engine/plugin framework, dataplane/network, access/AAA/subscriber protocols, UI/observability/API, docs/tests/release.
- Direct spot verification of the highest-severity findings with file reads.
- Initial local `make ze-verify-fast` run failed at `ze-lint` before tests due typecheck import errors in `internal/component/cli/model_commands_show_test.go:11-14`. Later reruns after remediations passed lint, race unit tests, build, encode, plugin, parse, decode, reload, editor, and web functional suites. The monolithic command exceeded this session's tool timeout during ExaBGP compatibility retries; `make ze-exabgp-test` passed when run separately.

Reference caveat: file:line references are snapshots from this review pass. Treat them as anchors to the cited function or block, not immutable coordinates; line numbers may drift as nearby code changes.

Status: not deployment-ready. The project has strong architecture and large coverage, but several release-blocking correctness, security, and evidence gaps remain.

Remediation progress in this branch:

- P0-5 SSH streaming commands now use dispatcher authorization and accounting, with remote-address propagation tests.
- P0-6 API transport now rejects plaintext non-loopback REST and requires gRPC TLS for authenticated non-loopback listeners.
- P0-3 API config commits now call the existing reload path after saving and return an error if runtime apply fails. Full transactional rollback and shared validation semantics remain open.
- P0-7 RADIUS client responses now demux by server, identifier, and authenticator under concurrent exchange; CoA/DM now requires fresh Event-Timestamp, verifies Message-Authenticator when present, and caches retransmission responses so duplicate packets do not replay side effects. Targeted `go test -race -count=1 ./internal/component/radius ./internal/plugins/l2tpauthradius` passed locally.
- P0-8 route producers now use distinct Linux `rtm_protocol` owner IDs for FIB, static, and policy-route routes; FIB monitoring ignores all Ze-owned route producers, static removal uses the exact installed route identity, and nft apply no longer sweeps unknown `ze_*` tables by prefix alone. Native unit tests and Linux compile-only checks passed locally; privileged Linux ownership/recovery tests remain open evidence.
- P0-2 production plugin aggregation no longer imports `internal/test/plugins`; functional tests build their DUT with the `zetest` tag to load fakeredist/fakel2tp only for .ci scenarios. The expected-plugin list in `cmd/ze/main_test.go` now reflects this split (fakel2tp/fakeredist removed) and adds the new `policy-routes` plugin.
- P0-2 egress-filter release evidence now uses destination-peer wire assertions for the eight previously partial tests; the parallel targeted run `go run ./cmd/ze-test bgp plugin 91 128 129 250 251 252 253 254` passed locally.
- P0-1 known release-gate entries for stale `remote { accept ... }` placement and `TestPeerInfoPopulatesStats` uptime have been resolved or verified stale in this branch.
- P0-1 `TestSessionConnectContext` no longer depends on an environment-specific unroutable address; it uses a context-blocking test dialer and the reactor package passes locally.
- P0-3 static config validation now enforces YANG leaf enum and numeric range restrictions; VPP invalid hugepage and poll-interval parse tests pass locally. Plugin `OnConfigVerify` parity remains open. A `port-spec` custom YANG validator is now registered for firewall/policy-route port match fields, validating ports 1-65535, ranges, comma-separated lists, and named set references.
- P1 policyroute now declares its firewall dependency so startup ordering matches its `firewall.ApplyAll()` use.
- P1 MCP GET SSE streams now clear the HTTP write deadline before entering the long-lived stream loop, matching the looking-glass SSE pattern.
- P1 telemetry Prometheus now defaults implicit listeners to `127.0.0.1:9273`; explicit all-interface binds remain possible when configured.
- P1 web mutating authenticated routes now share the same Origin/Referer same-origin defense previously used only by related-tool execution; unauthenticated login and read-only GETs are unchanged.
- P1 TACACS+ response headers now validate type, version, sequence number, and supported flags before response body parsing. Configurable strict fallback mode remains open.
- P1 BGP `SplitWireUpdate` nil source context now falls back to no ADD-PATH and has regression coverage.
- P2 API config sessions now serialize operations per session so concurrent REST/gRPC requests cannot race inside a session editor.
- P1 API docs/OpenAPI now follow REST auth when auth is configured.
- P1 API startup now fails closed when an explicitly configured REST/gRPC listener cannot be constructed or bound, rather than logging a warning and continuing without the requested management API.
- P1 route-refresh direct CLI decoding now accepts an explicit capability code, so enhanced route-refresh code 70 is decoded and tested instead of being reported as code 2.
- The legacy Finder web functional suite now runs against Finder by default, uses an isolated temporary zefs config directory, asserts DOM/text state through the right browser surfaces, and passes locally (`64 passed, 0 failed, 8 skipped`).
- Telemetry `BasicAuthConfig.Password` renamed to `BcryptHash` to clarify the field holds a bcrypt hash, not cleartext. Config session `CleanExpired` uses `maps.Copy` instead of manual loop. Test HTTP requests use `http.NoBody`.
- CI test runner now supports `-p`/`--parallel` concurrency flag with per-subcommand defaults; managed tests default to sequential (1) to prevent shared-state races.
- BGP decode fork test rewritten to use a lightweight shell stub instead of building the full `ze` binary inside the test. External plugin timeout raised to 30s for race-enabled parallel runs. L2TP config test validates via `LoadConfig` instead of bypassing YANG validation. Editor pipe test uses valid hold-time values and tests duplicate-IP validation.
- P2 documentation truth pass started: functional-test docs now match the 11-suite Makefile release gate and the `zetest` test-plugin split; `docs/features.md` has per-feature status labels; README test counts are conservative dated approximations; REST/gRPC API docs now describe REST loopback-only behavior, gRPC non-loopback TLS/auth requirements, authenticated docs, and unsupported production streaming hooks. `ze-doc-drift` now checks README claims, feature status labels, and functional-test gate claims derived from the Makefile; the stale DESIGN plugin table and interop count drift is also cleared.

## P0 Release Blockers

| ID | Area | Finding | Evidence | Risk | Required action |
|----|------|---------|----------|------|-----------------|
| P0-1 | Release gate | The documented release gate is broken and the checked-out repository does not contain CI that runs it. `plan/known-failures.md` records BGP config drift breaking `encode`, `plugin`, `parse`, `reload`, `ui`, and `exabgp-test`; the only workflow file present in this checkout is GitHub CodeQL, which may be mirror-only for a Codeberg-first project; `ze-ci` is only lint, unit, build. | `plan/known-failures.md:353-365`, `.github/workflows/codeql.yml:12-51`, `Makefile:288-344` | No trustworthy release signal is visible from the repository. A deployment can ship with known red suites unless an external/authoritative CI gate exists elsewhere. | Make `make ze-verify` green on clean Linux, add or document the authoritative CI that runs it, and quarantine any platform-only gates explicitly. |
| P0-2 | Test evidence | Remediated in this branch: test-only plugins are excluded from production aggregation and the eight egress-filter tests now assert destination-peer wire behavior. | `plan/known-failures.md:126-158`, `test/plugin/community-strip.ci`, `test/plugin/forward-overflow-two-tier.ci`, `test/plugin/forward-two-tier-under-load.ci`, `test/plugin/role-otc-*.ci` | Keep these tests in the release gate and avoid weakening destination `expect=bgp` assertions back to observer-only checks. | Preserve production/test plugin split and wire-level egress assertions. |
| P0-3 | Config/API | Partially remediated in this branch: API config-session commit now calls reload after save, and static validation enforces YANG enum/range restrictions. Static validation still lacks pattern enforcement and plugin `OnConfigVerify` parity. | `internal/component/api/config_session.go:162-173`, `cmd/ze/hub/api.go:49-55`, `internal/component/config/parser.go:200-203`, `internal/component/config/yang_schema.go:573-590` | Remaining invalid config can pass static validation when constraints live in pattern statements or plugin verify callbacks. | Enforce YANG pattern constraints and invoke plugin `OnConfigVerify` from every static/API/CLI validation path. |
| P0-4 | Reload atomicity | Reload commits plugin apply, mutates the shared config provider, then reloads subsystems. If a subsystem fails, no global rollback restores plugin state or provider roots. | `cmd/ze/hub/main.go:823-852`, `internal/component/engine/engine.go:121-132` | Failed reload can leave plugins, config provider, and subsystems on different versions. | Introduce all-or-nothing reload transaction across plugins, provider, and subsystems, or fail before mutating shared runtime state. |
| P0-5 | Authorization | SSH streaming commands bypass dispatcher authorization and TACACS/accounting hooks. | `internal/component/ssh/ssh.go:617-625`, `cmd/ze/hub/infra_setup.go:181-224` | Authenticated users can run streaming commands that policy would deny, without accounting records. | Route streaming execution through the same authorizer/accounting path as normal dispatch, with tests for denied streaming commands. |
| P0-6 | API transport security | REST uses plaintext HTTP for bearer tokens; gRPC TLS is optional. | `internal/component/api/rest/server.go:51-56`, `cmd/ze/hub/api.go:90-127` | Management credentials and config changes can traverse non-loopback listeners without transport confidentiality. | Require TLS or loopback-only for bearer/password management APIs. |
| P0-7 | RADIUS/L2TP security | Static reading indicates the RADIUS client uses one UDP socket for concurrent requests and can discard another goroutine's response; this demux claim needs a targeted runtime concurrency test. CoA/Disconnect replay risk is directly visible from static validation only checking source and authenticator before acting. | `internal/component/radius/client.go:124-160`, `internal/plugins/l2tpauthradius/coa.go:72-104`, `internal/plugins/l2tpauthradius/coa.go:151-187` | False auth failures, accounting loss, retry storms, and replayed disconnect/throttle attacks. | Add request demux or per-exchange socket/reader model; add CoA replay cache and Event-Timestamp/Message-Authenticator policy; prove the demux failure or disprove it with a concurrent RADIUS test. |
| P0-8 | Dataplane ownership | FIB kernel, static routes, and policyroute all use RT protocol 250 and list/delete by that shared marker. Firewall nft backend deletes every `ze_*` table before recreate. | `internal/plugins/fib/kernel/backend_linux.go:21-24`, `internal/plugins/static/backend_linux.go:16`, `internal/plugins/policyroute/rules_linux.go:54-61`, `internal/plugins/firewall/nft/backend_linux.go:46-52` | One producer can delete or misclassify another's routes/tables; multi-process or recovery scenarios can destroy unrelated state. | Add per-producer ownership identity in route/table metadata and cleanup only owned state; make nft apply ownership-aware. |

Severity note: web CSRF is not listed as P0 because the session cookie is already `Secure`, `HttpOnly`, and `SameSite=Strict` (`internal/component/web/auth.go:245-252`). The missing same-origin check on other mutating routes is tracked below as P1 defense in depth.

## P1 High Priority

| Area | Finding | Evidence | Required action |
|------|---------|----------|-----------------|
| BGP config | One incomplete peer causes all peers to be dropped despite comments saying incomplete peers are skipped. | `internal/component/bgp/config/loader_create.go:89-98` | Return partial valid peer list plus per-peer warnings, or reject the config loudly. |
| BGP forwarding | `SplitWireUpdate` documents `srcCtx == nil` as valid, but dereferences it for ADD-PATH state. | `internal/component/bgp/wireu/split.go:24-25`, `internal/component/bgp/wireu/split.go:173-175`, `internal/component/bgp/wireu/split.go:332-335`, `internal/component/bgp/reactor/forward_body.go:42-48` | Treat nil context as no ADD-PATH or return a controlled error; add oversized UPDATE tests. |
| BGP protocol | Strict unsupported-family UPDATE errors appear to close locally without clear NOTIFICATION coverage. | `internal/component/bgp/reactor/session_validation.go:216-227`, `internal/component/bgp/reactor/session_handlers.go:202-205` | Add RFC error handling and interop tests for non-negotiated family updates. |
| BGP diagnostics | Remediated in this branch: route-refresh direct CLI decode accepts capability code 2 or 70 and has enhanced-route-refresh coverage. | `internal/component/bgp/plugins/route_refresh/route_refresh.go:166-207`, `internal/component/bgp/plugins/route_refresh/route_refresh_test.go:85-160` | Keep code 70 covered in direct CLI and engine decode tests. |
| Plugin lifecycle | Config-path plugin autoload failures are logged and swallowed. Reload can accept config whose required plugin never started. | `internal/component/plugin/server/startup_autoload.go:191-223`, `internal/component/plugin/server/reload.go:172-225` | Fail reload/startup when a required config-root plugin cannot load, unless explicitly optional. |
| Hub reload | Orchestrator-mode reload keys plugin definitions only by name, so `run`/`use` changes can SIGHUP the old process instead of restart with the new command. | `internal/component/hub/reload.go:18-47`, `internal/component/hub/reload.go:93-149`, `internal/component/hub/config.go:210-215` | Diff the full plugin definition and restart when executable/config source changes. |
| API startup | Remediated in this branch: explicit REST/gRPC construction and bind failures now fail daemon startup. | `cmd/ze/hub/api.go:90-127`, `internal/component/api/rest/server.go:131-180`, `internal/component/api/grpc/server.go:138-190` | Keep targeted startup-failure coverage in REST/gRPC transport tests. |
| Web CSRF defense-in-depth | Session cookies are already `Secure`, `HttpOnly`, and `SameSite=Strict`, which is the primary CSRF protection. Most mutating POST routes do not have the additional same-origin check used by the related-tool handler. | `internal/component/web/auth.go:245-252`, `cmd/ze/hub/main.go:1222-1252`, `internal/component/web/handler_tools.go:88-100` | Consider extending the same-origin check to all mutating web routes as defense in depth, especially because Basic Auth fallback and future cookie-policy changes are outside SameSite's protection model. |
| Web CSP | CSP says `script-src 'self'`, but templates contain inline scripts; inline styles are still allowed. | `internal/component/web/auth.go:281-286`, `internal/component/web/templates/page/layout.html:35-64`, `internal/component/web/templates/page/workbench.html:40-69` | Move scripts to static assets or use nonce/hash-based CSP deliberately. |
| API docs exposure | REST OpenAPI and docs are unauthenticated even when API auth is enabled. | `internal/component/api/rest/server.go:254-258`, `internal/component/api/rest/server.go:650-676` | Require auth for schema/docs or provide an explicit opt-in public-docs flag. |
| Telemetry exposure | Remediated in this branch: Prometheus defaults to loopback, and `BasicAuthConfig.Password` renamed to `BcryptHash` to eliminate ambiguity about the stored credential format. | `internal/core/metrics/server.go:228-230` | Keep explicit bind requirement for all-interface exposure and document firewalling. |
| MCP | MCP SSE likely hits HTTP `WriteTimeout` after 30s. | `cmd/ze/hub/mcp.go:170-177`, `internal/component/mcp/streamable.go:945-1013` | Clear write deadlines for SSE as looking glass already does. |
| Interface management | Reload deletes manageable link types absent from config based on type, not persistent ownership. | `internal/component/iface/config.go:1269-1282` | Add explicit ze ownership marker or adoption model before destructive reconciliation. |
| Interface rollback | Interface apply/rollback is best-effort, continues after individual failures, and may delete/recreate tunnels. | `internal/component/iface/config.go:990-992`, `internal/component/iface/register.go:441-474`, `internal/component/iface/config.go:1045-1056` | Add transactional preflight, scoped inverse rollback, and privileged failure tests. |
| Policyroute | Policyroute calls `firewall.ApplyAll()` but does not declare a firewall dependency. | `internal/plugins/policyroute/register.go:20-31`, `internal/plugins/policyroute/register.go:189-191` | Declare startup dependency or lazy-load firewall backend before policyroute apply. |
| Static routes | Static route removal omits metric/table/link identity while apply includes metric. | `internal/plugins/static/backend_linux.go:46-52`, `internal/plugins/static/backend_linux.go:88-92` | Delete exact route identity, including table, priority, and nexthop/action. |
| Traffic control | Netlink cleanup does not restore original qdisc; VPP cleanup/rebind/orphan behavior has open gaps. | `internal/plugins/traffic/netlink/backend_linux.go:23-44`, `internal/component/traffic/register.go:278-287`, `plan/deferrals.md:204-213` | Add explicit reconcile semantics and real privileged/VPP evidence. |
| L2TP/PPP | Auth negotiation can downgrade to `AuthMethodNone`; L2TP tunnel/session defaults are unbounded; hidden AVPs are parsed but not decrypted before use. | `internal/component/ppp/auth.go:47-67`, `internal/component/ppp/session_run.go:481-517`, `plan/deferrals.md:168`, `plan/deferrals.md:170` | Make mandatory auth fail closed, set non-zero safety defaults, and implement hidden AVP handling or reject hidden mandatory AVPs. |
| RADIUS policy | Access-Accept mostly ignores deployment-critical attributes such as Framed-IP-Address, Framed-Pool, Filter-Id, timeouts, and rate policy. | `internal/plugins/l2tpauthradius/handler.go:109-119`, `internal/plugins/l2tppool/register.go:108-143`, `docs/features.md:87` | Implement or explicitly reject unsupported RADIUS attributes, and correct docs. |
| TACACS+ | Fail-open local fallback is a deployment policy risk. | `internal/component/aaa/types.go:95-123`, `internal/component/tacacs/authorizer.go:73-97` | Add configurable strict mode that denies on TACACS infrastructure failure. |
| DNS/NTP | Resolver silently falls back to Google DNS; NTP can step system time from unauthenticated servers without max-step policy. | `internal/component/resolve/dns/resolver.go:69-76`, `internal/plugins/ntp/ntp.go:140-165` | Avoid public resolver fallback in appliance mode; add NTP authentication or max-step/slew policy. |

## P2 Readiness Gaps

| Area | Finding | Evidence | Action |
|------|---------|----------|--------|
| Documentation | Remediated in this branch: functional-test docs now say the release gate runs 11 suites including L2TP, firewall, and web, and `ze-doc-drift` derives that suite list from the Makefile. | `docs/functional-tests.md:17-36`, `Makefile:131-160`, `scripts/docvalid/doc_drift.go:94-147` | Keep gate composition derived from the Makefile. |
| Feature inventory | Remediated in this branch: `docs/features.md` now labels every row as supported, experimental, partial, rejected, stub-backed, or future, with explicit partial/stub caveats for REST/gRPC streaming, L2TP redistribution/access gaps, VPP, dataplane evidence, config validation, reload atomicity, TACACS strict mode, and DNS resolver policy. | `docs/features.md:5-113` | Keep status labels current as feature claims change. |
| Doc drift tooling | Remediated in this branch: drift checks now cover README test-count claims, feature inventory status labels, and functional-test release-gate claims derived from the Makefile; `docs/DESIGN.md` currently has no drift under `ze-doc-drift`. | `scripts/docvalid/doc_drift.go:73-89`, `scripts/docvalid/doc_drift.go:427-585`, `Makefile:921-924` | Add new user-facing release docs to drift checks when they gain factual counts or status tables. |
| README counts | Remediated in this branch: README test counts are conservative, dated approximate claims instead of brittle exact totals. | `README.md:3`, `README.md:57-60` | Keep approximate/date wording or derive exact counts in generated docs. |
| API streaming | Partially remediated in this branch: production docs now say REST/gRPC streaming hooks return `streaming not supported` because the hub passes nil stream backend, and OpenAPI remains generic execute-only. Runtime still exposes the REST handler and gRPC method, so wiring a real backend remains open if streaming is a supported production claim. | `cmd/ze/hub/api.go:187-196`, `internal/component/api/engine.go:123-126`, `docs/guide/api.md:77-83`, `docs/guide/api.md:173-179` | Either wire a production stream backend or keep the unsupported status explicit. |
| Config sessions | Partially remediated: `CleanExpired` now uses `maps.Copy`; per-session serialization was addressed in prior commits. | `internal/component/api/config_session.go:192-207` | Verify full session-level serialization coverage. |
| Test infrastructure | Port allocation probes and releases ports before later bind, matching known flakes. | `internal/test/runner/ports.go:26-66`, `plan/known-failures.md:21-24`, `plan/known-failures.md:107-123` | Reserve ports for the lifetime of each test or use per-test network namespace/isolation. |
| Open deferrals | Many open rows are release-relevant: config validate plugin callbacks, VPP real daemon CI, L2TP redistribution, gRPC `.ci`, secure DNS, BGP dashboard, RIB inject docs/tests, LLGR readvertisement. | `plan/deferrals.md:40`, `plan/deferrals.md:56-68`, `plan/deferrals.md:87`, `plan/deferrals.md:147-148`, `plan/deferrals.md:195`, `plan/deferrals.md:204-213` | Triage every open row into release blocker, post-deployment backlog, or explicitly unsupported. |

## Deployment Plan

### Phase 0: Restore Trust In The Gate

Exit criteria:

- `make ze-verify` green on a clean Linux runner.
- `make ze-verify-fast` no longer aliases an already-known broken gate without warning.
- GitHub or Codeberg CI runs at least `make ze-verify` on every PR.
- `plan/known-failures.md` contains no untriaged release blockers.
- Partial `.ci` tests are either fixed or excluded from release evidence with a visible marker.

Work items:

- Keep BGP `remote accept` fixture drift resolved; no stale `remote { accept ... }` placement remains under `test/`.
- Keep current lint/typecheck gate green.
- Keep the eight egress-filter tests on destination-peer wire assertions.
- Keep production imports of `internal/test/plugins/*` excluded outside `zetest` builds.

### Phase 1: Security Hardening

Exit criteria:

- All command paths, including streaming commands, pass through authorization and accounting.
- All mutating web routes either rely on the documented `SameSite=Strict` session-cookie posture or also have same-origin checking where defense in depth is required.
- Remote API listeners require TLS for bearer/password auth or refuse to start.
- RADIUS concurrency and CoA replay have regression tests.
- L2TP defaults include safe resource caps, and mandatory auth cannot downgrade to no-auth.
- TACACS strict mode exists for production deployments.

Work items:

- Fix SSH streaming authorization/accounting bypass.
- Extend the existing related-tool same-origin check to other mutating web routes if the project wants belt-and-braces CSRF protection beyond `SameSite=Strict`.
- Add REST TLS support or enforce loopback-only plaintext.
- Add RADIUS demux/replay protection.
- Add TACACS strict fallback mode.
- Correct misleading L2TP auth startup log.

### Phase 2: Configuration And Transaction Correctness

Exit criteria:

- API config commits apply runtime state transactionally.
- `ze config validate`, startup, reload, API, and CLI share the same validation semantics.
- Reload is all-or-nothing across plugin transactions, config provider state, and subsystem reload.
- Config-path plugin autoload failures fail closed.

Work items:

- Enforce remaining YANG pattern and duplicate-key constraints.
- Invoke plugin `OnConfigVerify` from static validation.
- Add subsystem rollback or reload preflight before provider mutation.
- Diff full plugin definitions on hub reload.

### Phase 3: Dataplane Ownership And Rollback

Exit criteria:

- Every kernel/VPP/nft object installed by Ze has a clear owner identity.
- Reconciliation deletes only owned objects.
- Static, FIB, and policy routes cannot delete each other.
- Firewall apply is scoped and rollback-safe.
- Privileged firewall/traffic/interface tests run in a controlled CI environment.

Work items:

- Split or tag RT protocol ownership, including table/metric/nexthop identity.
- Make nft table cleanup owner-aware.
- Add interface ownership/adoption model.
- Fix policyroute/firewall dependency.
- Add VPP real-daemon CI for traffic/FIB idempotency and restart cases.

### Phase 4: BGP And Routing Correctness

Exit criteria:

- BGP config partial-edit behavior is explicit and tested.
- Oversized UPDATE split has nil-context and ADD-PATH coverage.
- Unsupported-family and malformed UPDATE paths send correct NOTIFICATION or documented RFC 7606 behavior.
- BGP route server/RR/filter tests prove wire-level egress behavior.
- BMP Loc-RIB unsupported status is explicit, or implementation lands.

Work items:

- Fix incomplete-peer handling.
- Harden `SplitWireUpdate` nil context.
- Add route-refresh decode coverage for enhanced route refresh.
- Finish RS fastpath and two-peer forwarding evidence.
- Triage LLGR/BMP/RIB-inject open deferrals before support claims.

### Phase 5: Access Protocol Deployment Proof

Scope note: Phase 5 is required for deployments that use subscriber access protocols such as L2TP/PPP/RADIUS. A BGP-only deployment can treat this phase as out of its launch critical path after the feature inventory clearly labels subscriber access as not in scope for that deployment.

Exit criteria:

- L2TP/PPP/RADIUS path has at least one full peer integration scenario, ideally accel-ppp or bngblaster.
- RADIUS-assigned address/filter/rate/session attributes are implemented or explicitly rejected.
- Hidden AVP behavior is implemented or rejected fail-closed.
- L2TP route redistribution has end-to-end advertise and withdraw proof.

Work items:

- Add full L2TP + PPP + NCP functional peer test.
- Implement or reject key RADIUS Access-Accept attributes.
- Implement hidden AVP decrypt or mandatory-hidden rejection.
- Complete `spec-bgp-redistribute` plus `spec-l2tp-7c-redistribute`.

### Phase 6: Documentation Truth Pass

Exit criteria:

- `README.md`, `docs/features.md`, `docs/functional-tests.md`, command reference, and API docs agree with code and release gate.
- Each user-facing feature is labeled supported, experimental, partial, stub-backed, rejected, or future.
- Security deployment docs require explicit authz profiles and safe listener binding.

Work items:

- Update functional test docs to match Makefile.
- Extend doc drift checks to feature inventory and release gate docs.
- Correct REST/gRPC API docs around auth, TLS, streaming, and docs exposure.
- Correct L2TP, VPP, BMP, RIB inject, and telemetry claims.

## Verification Matrix

Target status: the `make` targets below were confirmed in the main `Makefile` during this review pass. Some require Docker, root, CAP_NET_ADMIN, network namespaces, or external tools; those are real targets, not aspirational checks, but they need suitable runners. The `bin/ze-test firewall`, `bin/ze-test traffic`, and `bin/ze-test vpp` subcommands are registered in `cmd/ze-test`.

Minimum local gates before initial deployment:

```bash
make ze-verify
make ze-doc-test
make ze-fuzz-test
go test -race ./internal/component/bgp/reactor/... ./internal/component/radius/... ./internal/component/ssh/... ./internal/component/api/...
```

Environment-specific gates:

```bash
make ze-exabgp-test        # requires uv-managed Python deps
make ze-chaos-test         # chaos unit/functional/web suites
make ze-interop-test       # requires Docker
make ze-integration-test   # requires CAP_NET_ADMIN/root-capable runner
make ze-stress-test        # requires Linux, root, netns, iproute2/ethtool; traffic uses in-tree ze-test peer injector, not BNG Blaster
make ze-race-reactor       # reactor race stress, required for reactor concurrency changes
bin/ze-test firewall --all # requires nft/iptables privileges for kernel-state tests
bin/ze-test traffic --all  # traffic-control runner, some cases need CAP_NET_ADMIN
bin/ze-test vpp --all      # GoVPP-stub-backed VPP runner
python3 test/interop/run.py 33-bfd-frr # single BFD FRR interop scenario, requires Docker
```

Current local verification result:

```text
make ze-lint
PASS: 0 issues.

go test ./cmd/ze-test ./internal/component/web ./internal/component/web/testing -count=1
PASS.

make ze-web-test
PASS: 64 passed, 0 failed, 8 skipped.

make ze-plugin-test
PASS: 320/320, skip 1.

make ze-exabgp-test
PASS: 37/37 recovered on retry.

make ze-verify-fast
Passed lint, race unit tests, build, all 11 functional suites, and web.
The command was terminated by this session's tool timeout during the final ExaBGP compatibility retry phase; the ExaBGP target passed when rerun separately.
```
