# Deployment Readiness Deep Review

Date: 2026-05-01
Scope: whole repository review for moving Ze out of experimental status.

Method:

- Original repository map: architecture docs, feature inventory, Makefile gates, known failures, deferrals, schemas, components, plugins, and test surface.
- Original forked subsystem agents: BGP, config/engine/plugin framework, dataplane/network, access/AAA/subscriber protocols, UI/observability/API, docs/tests/release.
- 2026-05-01 refresh: static cross-check of the current checkout against the original P0/P1/P2 findings, with direct file reads for the highest-risk open and resolved items.
- 2026-05-01 remediation pass: code changes for the release gate, config/API validation, plugin reload, security policy, L2TP/PPP, DNS/NTP, and BGP unsupported-family handling.
- 2026-05-02 local verification: `make ze-verify` passed after the remediation pass, including lint, cached unit tests, race unit tests, all 11 functional suites, and ExaBGP compatibility 37/37.
- Current worktree is dirty with remediation edits and pre-existing unrelated changes. Treat this checkout as unsuitable for final release-candidate evidence until cleaned or intentionally incorporated and rerun on the target runner.

Reference caveat: file:line references are snapshots from this review pass. Treat them as anchors to the cited function or block, not immutable coordinates; line numbers may drift as nearby code changes.

Status: not deployment-ready. The original P0 surface has shrunk substantially, and several P1 security/protocol gaps are code-remediated. Release still depends on clean release-candidate gate evidence, global reload atomicity, interface/traffic rollback proof, RADIUS Access-Accept policy, CSP hardening, BGP incomplete-peer behavior, and privileged dataplane evidence.

Current state summary:

- `make ze-verify` now targets lint, cached unit tests, race-on-changed unit tests, the 11-suite functional gate, and ExaBGP compatibility (`Makefile:353-358`). `.woodpecker/verify.yml` is present and runs `make ze-verify`.
- The functional release gate is now 11 suites: encode, plugin, parse, decode, reload, ui, editor, managed, l2tp, firewall, and web (`Makefile:146-175`, `docs/functional-tests.md:17-21`). Makefile help and doc drift checks now derive or verify the same suite set.
- Closed or code-remediated from the original P0 list: production/test plugin split, egress-filter wire assertions, SSH streaming authorization/accounting, API transport security, RADIUS client demux and CoA/DM replay cache plus `Message-Authenticator` enforcement, distinct route producer ownership IDs, scoped nft cleanup, and exact static route deletion.
- Config validation is stronger than the previous review stated: YANG enum, range, and pattern checks are enforced; static/API/CLI validation runs registered in-process config verifiers; duplicate list keys are rejected by parser paths; API commit rolls disk config back on runtime reload failure. Live external plugin `OnConfigVerify` callbacks remain explicitly out of scope for static validation.
- The current checkout has local green gate evidence, but it is not usable as final release-candidate evidence because the worktree is dirty with remediation edits and unrelated changes.

## P0 Release Blockers

| ID | Area | Finding | Evidence | Risk | Required action |
|----|------|---------|----------|------|-----------------|
| P0-1 | Release gate evidence | Locally closed, release-candidate evidence still open. `.woodpecker/verify.yml` runs `make ze-verify`, Makefile help/drift checks agree with the 11-suite functional gate, and `make ze-verify` passed locally on 2026-05-02 after remediation. The worktree is still dirty, so this is not final release-candidate evidence. | `.woodpecker/verify.yml`, `Makefile:146-175`, `Makefile:353-358`, `scripts/docvalid/doc_drift.go`, `plan/known-failures.md:11-24`, `plan/known-failures.md:37-126`, `plan/known-failures.md:154-205` | A release can still ship without a clean, reproducible signal from the intended runner. The dirty checkout also prevents treating the local run as final release evidence. | Clean or intentionally incorporate the worktree changes, make `make ze-verify` green on a clean Linux runner, and triage every known failure as blocker, platform-only, or post-release. |
| P0-2 | Test evidence | Closed in code. Test-only plugins are excluded from production aggregation, and the eight egress-filter cases now assert destination-peer wire behavior. | `internal/component/plugin/all/all.go:81-139`, `cmd/ze/plugins_zetest.go:1-7`, `cmd/ze/main_test.go:22`, `test/plugin/community-strip.ci`, `test/plugin/forward-overflow-two-tier.ci`, `test/plugin/forward-two-tier-under-load.ci`, `test/plugin/role-otc-*.ci` | Regression risk if production aggregation imports test plugins again or wire assertions are weakened. | Preserve the production/test plugin split and keep destination `expect=bgp` assertions in the release gate. |
| P0-3 | Config/API validation and commit | Closed in code. API commits validate before save and roll back the saved config if runtime reload fails; YANG enum/range/pattern checks are enforced; static/API/CLI validation runs registered in-process config verifiers; parser paths reject duplicate list keys; live external plugin `OnConfigVerify` callbacks are documented as reload/commit-time only. | `internal/component/api/config_session.go`, `internal/component/config/parser_list.go`, `internal/component/config/parser_freeform.go`, `internal/component/config/plugin_verify.go`, `internal/component/config/yang_schema.go:598-603`, `internal/component/config/schema.go:750-765` | Regression risk if rollback, duplicate-key rejection, or static/live plugin verifier scope drifts. | Keep rollback, duplicate-key, and verifier-scope tests/docs in the gate. |
| P0-4 | Reload atomicity | Open. Plugin reload has transaction machinery (`plugin/server/reload.go` verify-apply with txLock and rollback), but the engine still iterates subsystems sequentially and returns on first error without rolling back already-reloaded subsystems. | `internal/component/hub/reload.go` (orchestrator plugin lifecycle), `internal/component/plugin/server/reload.go:228-247` (config reload transactions), `internal/component/engine/engine.go:121-132` (subsystem iteration, no rollback) | Failed reload can leave plugins, config provider, and subsystems on different versions. | Introduce all-or-nothing reload across plugin transactions, provider roots, and subsystem reload, or preflight all failing work before mutating shared runtime state. |
| P0-5 | Authorization | Closed in code. SSH streaming commands now use dispatcher authorization/accounting with user and remote-address propagation. | `cmd/ze/hub/infra_setup.go:213-236`, `internal/component/ssh/ssh.go:631-640` | Regression risk if future streaming paths bypass dispatcher wrappers. | Keep denied streaming-command and accounting tests in the gate. |
| P0-6 | API transport security | Closed in code for the reviewed exposure. REST rejects non-loopback listeners because it has no TLS transport, and gRPC rejects non-loopback listeners unless auth and TLS are configured. | `internal/component/api/rest/server.go:99-126`, `internal/component/api/grpc/server.go:87-127` | Regression risk if listener policy is weakened or docs drift. | Keep transport policy tests and docs aligned with loopback REST and authenticated TLS gRPC. |
| P0-7 | RADIUS/L2TP security | Closed in code for the original P0. RADIUS responses demux by `(server, identifier)` map key with per-waiter authenticator verification via `VerifyResponseAuth`; CoA/DM requires fresh `Event-Timestamp`; duplicates return cached responses without replaying side effects; CoA/DM now requires valid `Message-Authenticator`. | `internal/component/radius/client.go:46-49` (responseKey), `internal/component/radius/client.go:242-261` (dispatch + auth verify), `internal/component/radius/packet.go`, `internal/plugins/l2tpauthradius/coa.go`, `internal/plugins/l2tpauthradius/coa_test.go` | Regression risk if CoA/DM authentication or replay cache semantics are weakened. | Keep CoA/DM missing/invalid `Message-Authenticator`, replay, and demux tests in the gate. |
| P0-8 | Dataplane ownership | Code-remediated, evidence still open. FIB, static, and policyroute use distinct Linux route protocol IDs; FIB monitor ignores all Ze-owned producers; static route removal uses exact identity; nft cleanup no longer sweeps unknown `ze_*` tables by prefix alone. | `internal/core/rtproto/rtproto.go:5-30`, `internal/plugins/fib/kernel/monitor_linux.go:56-58`, `internal/plugins/static/backend_linux.go:40-54`, `internal/plugins/firewall/nft/backend_linux.go:49-82` | Ownership code looks correct, but recovery and privileged kernel-state behavior still need release evidence. | Add privileged Linux tests for route ownership, nft ownership, restart/recovery, and multi-producer non-interference. |

Severity note: web CSRF is still not listed as P0 because the session cookie is `Secure`, `HttpOnly`, and `SameSite=Strict` (`internal/component/web/auth.go:245-252`), and authenticated mutating routes now share the same-origin wrapper (`cmd/ze/hub/main.go:1206-1265`). CSP hardening remains P1.

## P1 High Priority

Resolved P1 findings now tracked as regression coverage: `SplitWireUpdate` nil source context, enhanced route-refresh code 70 decode, explicit REST/gRPC startup failure, authenticated web same-origin checks, authenticated API docs/OpenAPI, telemetry loopback default, MCP SSE write-deadline clearing, TACACS response header validation, policyroute firewall dependency, and exact static route deletion.

| Area | Finding | Evidence | Required action |
|------|---------|----------|-----------------|
| BGP config | One incomplete peer causes all peers to be dropped despite comments saying incomplete peers are skipped. | `internal/component/bgp/config/loader_create.go:89-98` | Return partial valid peer list plus per-peer warnings, or reject the config loudly. |
| BGP protocol | Closed in code for strict unsupported-family UPDATE handling. Non-negotiated MP_REACH_NLRI is rejected before plugin delivery, sends UPDATE Message Error / Optional Attribute Error, and closes the session. | `internal/component/bgp/reactor/session_read.go`, `internal/component/bgp/reactor/session_test.go` | Keep the non-negotiated MP family NOTIFICATION regression test in the gate. |
| Plugin lifecycle | Closed in code. Config-path auto-load failures now fail closed during startup and reload when a required config-root plugin cannot load. | `internal/component/plugin/server/startup_autoload.go`, `internal/component/plugin/server/reload.go`, `internal/component/plugin/server/reload_test.go` | Keep reload auto-load fail-closed tests in the gate. |
| Hub reload | Closed in code. Orchestrator-mode reload diffs the full plugin definition and restarts when executable/config source changes. | `internal/component/hub/reload.go`, `internal/component/hub/reload_test.go` | Keep same-name changed-`run` restart tests in the gate. |
| Web CSP | CSP says `script-src 'self'`, but templates contain inline scripts; inline styles are still allowed. | `internal/component/web/auth.go:281-286`, `internal/component/web/templates/page/layout.html:35-64`, `internal/component/web/templates/page/workbench.html:40-69` | Move scripts to static assets or use nonce/hash-based CSP deliberately. |
| Interface management | Reload deletes manageable link types absent from config based on type, not persistent ownership. | `internal/component/iface/config.go:1269-1282` | Add explicit ze ownership marker or adoption model before destructive reconciliation. |
| Interface rollback | Interface apply/rollback is best-effort, continues after individual failures, and may delete/recreate tunnels. | `internal/component/iface/config.go:990-992`, `internal/component/iface/register.go:441-474`, `internal/component/iface/config.go:1045-1056` | Add transactional preflight, scoped inverse rollback, and privileged failure tests. |
| Traffic control | Netlink cleanup does not restore original qdisc; VPP cleanup/rebind/orphan behavior has open gaps. | `internal/plugins/traffic/netlink/backend_linux.go:23-44`, `internal/component/traffic/register.go:278-287`, `plan/deferrals.md:204-213` | Add explicit reconcile semantics and real privileged/VPP evidence. |
| L2TP/PPP | Closed in code for the reviewed safety policy. PPP required auth rejects `AuthMethodNone`; L2TP defaults have finite caps and CHAP-MD5 auth; no-auth requires explicit `allow-no-auth true`; hidden mandatory AVPs fail closed. | `internal/component/ppp/start_session.go`, `internal/component/ppp/session_run.go`, `internal/component/l2tp/config.go`, `internal/component/l2tp/avp.go`, `internal/component/l2tp/schema/ze-l2tp-conf.yang` | Keep PPP required-auth, L2TP auth-policy, caps, reload, CLI, and hidden mandatory AVP tests in the gate. |
| RADIUS policy | Access-Accept mostly ignores deployment-critical attributes such as Framed-IP-Address, Framed-Pool, Filter-Id, timeouts, and rate policy. | `internal/plugins/l2tpauthradius/handler.go:109-119`, `internal/plugins/l2tppool/register.go:108-143`, `docs/features.md:87` | Implement or explicitly reject unsupported RADIUS attributes, and correct docs. |
| RADIUS CoA policy | Closed in code. CoA/DM requires valid `Message-Authenticator`; missing or invalid packets are rejected before side effects. | `internal/plugins/l2tpauthradius/coa.go`, `internal/component/radius/packet.go`, `internal/plugins/l2tpauthradius/coa_test.go` | Keep missing/invalid `Message-Authenticator` tests in the gate. |
| TACACS+ | Closed in code. Configurable `strict-fallback` denies local fallback on TACACS infrastructure failure when enabled. | `internal/component/tacacs/authorizer.go`, `internal/component/tacacs/config.go`, `internal/component/tacacs/schema/ze-tacacs-conf.yang` | Keep strict fallback config and authorizer tests in the gate. |
| DNS/NTP | Closed in code for the reviewed production-safety policy. DNS resolver no longer falls back to public recursive DNS when no resolver is configured; NTP has bounded `max-step` with explicit `0` opt-out. | `internal/component/resolve/dns/resolver.go`, `internal/plugins/ntp/ntp.go`, `internal/plugins/ntp/schema/ze-ntp-conf.yang` | Keep no-public-fallback and NTP `max-step` tests in the gate. |

## P2 Readiness Gaps

| Area | Finding | Evidence | Action |
|------|---------|----------|--------|
| Documentation | Closed in code for the functional gate claim. Functional-test docs, Makefile help, and drift checks agree on the 11-suite release gate. | `docs/functional-tests.md:17-36`, `Makefile:146-175`, `scripts/docvalid/doc_drift.go` | Keep gate claims derived or drift-checked when suite membership changes. |
| Feature inventory | Current repo has `docs/features.md` status labels for every row: supported, experimental, partial, rejected, stub-backed, or future. It also includes explicit partial/stub caveats for REST/gRPC streaming, L2TP redistribution/access gaps, VPP, dataplane evidence, config validation, reload atomicity, TACACS strict mode, and DNS resolver policy. | `docs/features.md:5-113` | Keep status labels current as feature claims change. |
| Doc drift tooling | Closed in code for the release-gate help gap. Drift checks cover README test-count claims, feature inventory status labels, functional-test release-gate claims derived from the Makefile, and Makefile help drift. | `scripts/docvalid/doc_drift.go`, `Makefile:921-924` | Add new user-facing release docs and help claims to drift checks when they contain factual counts or status lists. |
| README counts | README test counts are conservative, dated approximate claims instead of brittle exact totals. | `README.md:3`, `README.md:57-60` | Keep approximate/date wording or derive exact counts in generated docs. |
| API streaming | Production docs now say REST/gRPC streaming hooks return `streaming not supported` because the hub passes nil stream backend, and OpenAPI remains generic execute-only. Runtime still exposes the REST handler and gRPC method, so wiring a real backend remains open if streaming is a supported production claim. | `cmd/ze/hub/api.go:187-196`, `internal/component/api/engine.go:123-126`, `docs/guide/api.md:77-83`, `docs/guide/api.md:173-179` | Either wire a production stream backend or keep the unsupported status explicit. |
| Config sessions | Per-session serialization is implemented and covered. The remaining config-session issue is API save/reload rollback and is tracked as P0-3. | `internal/component/api/config_session.go:43-51`, `internal/component/api/config_session.go:207-239`, `internal/component/api/config_session_test.go:249-280` | Keep concurrency tests and close rollback under P0-3. |
| Test infrastructure | Port allocation probes and releases ports before later bind, matching known flakes. | `internal/test/runner/ports.go:26-66`, `plan/known-failures.md:21-24`, `plan/known-failures.md:107-123` | Reserve ports for the lifetime of each test or use per-test network namespace/isolation. |
| Open deferrals | Many open rows remain release-relevant: static/live plugin validation parity, L2TP peer tests and redistribution, RADIUS Access-Accept policy, VPP real-daemon CI, traffic privileged evidence, BMP Loc-RIB, and raw plugin IPC. Duplicate parser keys, L2TP auth/caps/hidden mandatory AVPs, and DNS/NTP safety policy are code-remediated. | `plan/deferrals.md:147-148`, `plan/deferrals.md:162-170`, `plan/deferrals.md:175-181`, `plan/deferrals.md:195`, `plan/deferrals.md:200-213`, `plan/deferrals.md:231` | Triage every open row into release blocker, scoped deployment exclusion, post-deployment backlog, or explicitly unsupported. |

## Deployment Plan

### Phase 0: Restore Trust In The Gate

Exit criteria:

- Worktree is clean, or every dirty change is intentionally part of the release candidate.
- `make ze-verify` green on a clean Linux runner.
- GitHub or Codeberg CI runs at least `make ze-verify` on every PR.
- `plan/known-failures.md` contains no untriaged release blockers.
- Release-gate documentation, Makefile help, and drift checks agree on the same suite set.

Work items:

- Keep the authoritative CI gate for `make ze-verify` documented and enabled.
- Resolve or classify the known flakes and platform-only failures that remain in `plan/known-failures.md`.
- Keep Makefile help, docs, and drift checks aligned for `ze-functional-test`.
- Keep the eight egress-filter tests on destination-peer wire assertions.
- Keep production imports of `internal/test/plugins/*` excluded outside `zetest` builds.

### Phase 1: Security Hardening

Exit criteria:

- Existing command authorization/accounting, web same-origin, REST loopback, and gRPC TLS/auth protections stay covered by regression tests.
- TACACS strict mode exists for production deployments that want deny-on-infrastructure-failure semantics.
- RADIUS CoA/DM `Message-Authenticator` policy is explicit and tested.
- L2TP defaults include safe resource caps, and mandatory auth cannot downgrade to no-auth.
- Hidden mandatory L2TP AVPs are decrypted correctly or rejected fail-closed.
- DNS and NTP production policies avoid silent public resolver fallback and unsafe time steps.
- Web CSP is either nonce/hash-based or inline scripts/styles are moved out deliberately.

Work items:

- Keep TACACS strict fallback mode covered.
- Keep CoA/DM `Message-Authenticator` requirements covered.
- Keep L2TP mandatory auth, non-zero caps, and hidden mandatory AVP rejection covered.
- Keep DNS no-public-fallback and NTP max-step policy covered.
- Harden CSP without breaking HTMX/Finder behavior.

### Phase 2: Configuration And Transaction Correctness

Exit criteria:

- API config commits apply runtime state transactionally.
- `ze config validate`, startup, reload, API, and CLI share documented validation semantics, including explicit external-plugin verify behavior.
- Duplicate list keys are rejected or intentionally modeled without silent surprise.
- Reload is all-or-nothing across plugin transactions, config provider state, and subsystem reload.
- Config-path plugin autoload failures fail closed during startup and reload.
- Hub reload restarts plugins when executable/config source changes.

Work items:

- Keep API commit rollback covered.
- Keep duplicate list-key rejection covered.
- Keep static validation semantics for live external plugin callbacks documented.
- Add subsystem rollback or full reload preflight before provider mutation.
- Keep reload config-path autoload fail-closed behavior covered.
- Keep full plugin-definition diffing on hub reload covered.

### Phase 3: Dataplane Ownership And Rollback

Exit criteria:

- Every kernel/VPP/nft object installed by Ze has a clear owner identity and privileged proof.
- Reconciliation deletes only owned objects.
- Static, FIB, and policy routes cannot delete each other under restart/recovery tests.
- Firewall apply is scoped and rollback-safe.
- Interface reconciliation has an explicit ownership/adoption model before destructive deletion.
- Interface apply/rollback has preflight and scoped inverse rollback for privileged failure cases.
- Privileged firewall/traffic/interface tests run in a controlled CI environment.

Work items:

- Add privileged route/nft ownership and recovery tests for the code-remediated route owner split.
- Add interface ownership/adoption model.
- Add interface transactional preflight and failure tests.
- Define traffic-control original-qdisc restore/reconcile semantics.
- Add VPP real-daemon CI for traffic/FIB idempotency and restart cases.

### Phase 4: BGP And Routing Correctness

Exit criteria:

- BGP config partial-edit behavior is explicit and tested.
- Unsupported-family and malformed UPDATE paths send correct NOTIFICATION or documented RFC 7606 behavior.
- BGP route server/RR/filter tests prove wire-level egress behavior.
- BMP Loc-RIB unsupported status is explicit, or implementation lands.
- LLGR/RIB-inject/redistribution support claims are aligned with implemented behavior.

Work items:

- Fix incomplete-peer handling.
- Keep unsupported-family NOTIFICATION behavior tests covered.
- Keep `SplitWireUpdate`, ADD-PATH, and enhanced route-refresh regression coverage in the gate.
- Triage LLGR/BMP/RIB-inject open deferrals before support claims.

### Phase 5: Access Protocol Deployment Proof

Scope note: Phase 5 is required for deployments that use subscriber access protocols such as L2TP/PPP/RADIUS. A BGP-only deployment can treat this phase as out of its launch critical path after the feature inventory clearly labels subscriber access as not in scope for that deployment.

Exit criteria:

- L2TP/PPP/RADIUS path has at least one full peer integration scenario, ideally accel-ppp or bngblaster.
- RADIUS-assigned address/filter/rate/session attributes are implemented or explicitly rejected.
- Hidden AVP behavior is implemented or rejected fail-closed.
- Mandatory authentication cannot negotiate down to `AuthMethodNone` unless the operator explicitly allows no-auth.
- L2TP route redistribution has end-to-end advertise and withdraw proof.

Work items:

- Add full L2TP + PPP + NCP functional peer test.
- Implement or reject key RADIUS Access-Accept attributes.
- Keep mandatory-hidden AVP rejection covered.
- Keep non-zero tunnel/session safety defaults covered.
- Complete `spec-bgp-redistribute` plus `spec-l2tp-7c-redistribute`.

### Phase 6: Documentation Truth Pass

Exit criteria:

- `README.md`, `docs/features.md`, `docs/functional-tests.md`, command reference, and API docs agree with code and release gate.
- Each user-facing feature is labeled supported, experimental, partial, stub-backed, rejected, or future.
- Security deployment docs require explicit authz profiles and safe listener binding.
- Unsupported production API streaming, VPP real-daemon status, L2TP access gaps, reload atomicity, and validation limitations stay explicit until closed.

Work items:

- Keep Makefile help text aligned for the functional gate.
- Extend drift checks to any new release-gate/help/status claims.
- Keep REST/gRPC API docs aligned with auth, TLS, streaming, and docs exposure behavior.
- Correct L2TP, VPP, BMP, RIB inject, DNS/NTP, TACACS strict-mode, and reload-atomicity claims as those gaps close.

## Verification Matrix

Target status: the `make` targets below exist in the main `Makefile`. Some require Docker, root, CAP_NET_ADMIN, network namespaces, or external tools; those are real targets, not aspirational checks, but they need suitable runners. The `bin/ze-test firewall`, `bin/ze-test traffic`, and `bin/ze-test vpp` subcommands are registered in `cmd/ze-test`. The 2026-05-01 refresh did not rerun these gates.

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

Current verification status for this refresh:

```text
2026-05-02 local run:
make ze-verify
PASS: lint, cached unit tests, race unit tests, build, all 11 functional suites, and ExaBGP compatibility 37/37.

Additional focused checks from the same pass:
go test -run 'TestParserInlineListDuplicatePathInformation|TestMigrateStaticDuplicatePathInformation' ./internal/component/config ./internal/exabgp/migration
PASS.
go test ./internal/component/config ./internal/exabgp/migration
PASS.
make ze-exabgp-test
PASS: 37/37.

The checkout remains dirty with remediation edits and pre-existing unrelated changes.
Do not treat this checkout as final release-candidate evidence until the worktree is cleaned or those changes are intentionally included and verified on the target runner.
```

Last recorded local verification result from the original remediation pass:

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

make ze-verify
Passed lint, race unit tests, build, all 11 functional suites, and web.
The command was terminated by this session's tool timeout during the final ExaBGP compatibility retry phase; the ExaBGP target passed when rerun separately.
```
