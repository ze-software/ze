# Deployment Readiness Deep Review

Date: 2026-05-02
Scope: whole repository review for moving Ze out of experimental status.

Method:

- Original repository map: architecture docs, feature inventory, Makefile gates, known failures, deferrals, schemas, components, plugins, and test surface.
- Original forked subsystem agents: BGP, config/engine/plugin framework, dataplane/network, access/AAA/subscriber protocols, UI/observability/API, docs/tests/release.
- 2026-05-01 refresh: static cross-check of the current checkout against the original P0/P1/P2 findings, with direct file reads for the highest-risk open and resolved items.
- 2026-05-01 remediation pass: code changes for the release gate, config/API validation, plugin reload, security policy, L2TP/PPP, DNS/NTP, and BGP unsupported-family handling.
- 2026-05-02 prior local verification: `make ze-verify` passed after the remediation pass, including lint, cached unit tests, race unit tests, all 11 functional suites, and ExaBGP compatibility 37/37.
- 2026-05-02 static refresh: the refresh started from tracked-clean source code. `git status --short` showed one unrelated untracked file, `plan/comparison/mikrotik-winbox-vs-ze.md`. This review file is the only intended tracked edit from the refresh. No test, build, or lint command was rerun during this document update.
- 2026-05-02 follow-up hardening pass: code/docs for BGP incomplete-peer handling, Web CSP inline removal, RADIUS Access-Accept exact-or-reject, bounded hub reload rollback, interface ownership-scoped deletion, VPP traffic startup orphan scan, tc original-qdisc snapshot/restore, stale docs/deferral drift, and a Docker-backed `make ze-linux-test` target for Linux-tagged Go unit tests from non-Linux workstations.

Reference caveat: file:line references are snapshots from this review pass. Treat them as anchors to the cited function or block, not immutable coordinates; line numbers may drift as nearby code changes.

Status: not deployment-ready. The original P0 surface has shrunk substantially, and several P1 security/protocol gaps are code-remediated. Release still depends on clean release-candidate gate evidence, target-runner privileged dataplane evidence, and full L2TP PPP/NCP/kernel peer proof.

Current state summary:

- `make ze-verify` now targets lint, cached unit tests, race-on-changed unit tests, the 11-suite functional gate, and ExaBGP compatibility (`Makefile:356-362`). `.woodpecker/verify.yml` is present and runs `make ze-verify`.
- The functional release gate is now 11 suites: encode, plugin, parse, decode, reload, ui, editor, managed, l2tp, firewall, and web (`Makefile:149-178`, `docs/functional-tests.md:17-21`). Makefile help and doc drift checks now derive or verify the same suite set.
- Closed or code-remediated from the original P0/P1 list: production/test plugin split, egress-filter wire assertions, SSH streaming authorization/accounting, API transport security, RADIUS client demux and CoA/DM replay cache plus `Message-Authenticator` enforcement, distinct route producer ownership IDs, scoped nft cleanup, exact static route deletion, BGP incomplete-peer preservation of valid peers, strict Web CSP without inline script/style dependencies, unsupported RADIUS Access-Accept exact-or-reject policy, interface ownership-scoped deletion, VPP traffic startup orphan policer scan, and tc original-qdisc snapshot/restore for restorable roots.
- Config validation is stronger than the previous review stated: YANG enum, range, and pattern checks are enforced; static/API/CLI validation runs registered in-process config verifiers; duplicate list keys are rejected by parser paths; API commit rolls disk config back on hook/reload failure. This does not close global runtime reload atomicity, which remains P0-4. Live external plugin `OnConfigVerify` callbacks remain explicitly out of scope for static validation.
- The refresh started from tracked-clean source code with one unrelated untracked comparison document. This review file is the only intended tracked edit from the refresh. Prior local green gate evidence exists, but this static refresh did not rerun the gate, and no clean target-runner release-candidate result is recorded here.
- 2026-05-02 handoff follow-up: local `make ze-verify` passed in this checkout. This is useful sanity evidence, but it is not clean target-runner release-candidate evidence because `target-runner` is not available here and the worktree still contains the review doc edit plus the untracked comparison document.
- Drift found during the 2026-05-02 static refresh has been corrected in this pass: `docs/functional-tests.md` now acknowledges `make ze-chaos-web-test`; `docs/features.md` reflects plugin autoload, reload-diff, provider/subsystem rollback, and transactional changed-plugin replacement; `docs/guide/tacacs.md` reflects strict fallback; the duplicate-key deferral row is closed; Makefile stress-test wording now says the in-tree ze-test peer injector; L2TP/RADIUS docs now reflect Access-Accept exact-or-reject behavior.
- 2026-05-03 follow-up: PPP LCP Opened-state RXR re-entry is code-remediated, and functional test port allocation now holds runner-level advisory reservations for the suite lifetime instead of only probing and releasing ports before later binds.
- 2026-05-03 follow-up: interface apply no longer continues best-effort after individual mutating failures. Successful create, address, bridge-port, mirror, and selected property operations are journaled with scoped inverse callbacks, and the first apply failure rolls back the recorded steps before returning.
- 2026-05-03 follow-up: plugin gate flake hardening removed fixed sleeps from `nexthop`, `watchdog`, and `watchdog-med-override`, made `bfd-auth-meticulous-persist` wait for persisted sequence progress, and gated `show-errors-received` dispatch on plugin post-startup. The prefix-maximum flake remains tracked until a full release gate proves the shape is gone.
- 2026-05-03 follow-up: the stale static-validation deferral for plugin `OnConfigVerify` parity is closed for in-process verifiers. `ze config validate`, CLI editor validation, and API pre-save validation now run side-effect-free `InProcessConfigVerifier` hooks; live external plugin callbacks remain intentionally reload/commit-only transaction participants.
- 2026-05-03 follow-up: L2TP route redistribution is no longer an open implementation deferral. The real RouteObserver emits add/remove batches, synthetic-producer functional tests cover BGP announce/withdraw and per-peer `NEXT_HOP self`, and Docker-backed `xl2tpd` evidence covers real external LAC control tunnel plus incoming-call session establishment. Local privileged Docker evidence now covers interface first-apply non-adoption, reload deletion scope, and rollback of created links after partial apply failure; FIB restart sweep and flush-on-stop preserving static and policyroute-owned routes; nft same-instance cleanup and restart reapply preservation; traffic netlink qdisc snapshot/restore after backend restart; real VPP FIB add/withdraw plus traffic-control apply/bind, same-config Ze restart preservation, and stale Ze policer startup cleanup via `make ze-deployment-vpp-test`; and external L2TP control/session proof via `make ze-deployment-l2tp-test`. A strict Linux-only full L2TP PPP/NCP peer target now exists as `make ze-deployment-l2tp-ppp-test`, with a Docker wrapper at `make ze-deployment-l2tp-ppp-docker-test`; both still require PPPoL2TP kernel support. Clean release-candidate evidence, a passing full L2TP PPP/NCP/kernel peer run, and target-runner privileged dataplane evidence are still required.
- 2026-05-03 follow-up: `make ze-release-check` now provides a Docker-backed clean-source substitute for release-candidate gate evidence when `target-runner` is unavailable. It refuses dirty worktrees, clones the repository into an ephemeral container, mirrors the Woodpecker dependency setup, and runs `make ze-verify`. It was attempted during this pass and correctly refused the dirty worktree, so no clean release-candidate evidence was produced.
- 2026-05-03 follow-up: REST/gRPC API streaming is code-remediated for registered streaming commands. The production hub now wires `APIEngine.Stream` to the same pluginserver streaming-handler registry used by SSH monitor commands, preserving dispatcher authorization, caller metadata, and accounting. Generic command completion remains future work.

## P0 Release Blockers

| ID | Area | Finding | Evidence | Risk | Required action |
|----|------|---------|----------|------|-----------------|
| P0-1 | Release gate evidence | Locally closed, release-candidate evidence still open. `.woodpecker/verify.yml` runs `make ze-verify`, Makefile help/drift checks agree with the 11-suite functional gate, and `make ze-verify` passed locally on 2026-05-02 after remediation. This refresh started from tracked-clean source code with one unrelated untracked comparison document; this review file is the only intended tracked edit. The static refresh did not rerun the gate. A later local dirty-worktree `make ze-verify` run passed, but no target-runner result is recorded here. `make ze-release-check` now exists as a Docker-backed clean-source substitute and was attempted during this pass, but it correctly refused the dirty worktree before running `make ze-verify`. | `.woodpecker/verify.yml`, `Makefile:146-178`, `Makefile:356-362`, `Makefile`, `scripts/evidence/effective-verify.sh`, `scripts/docvalid/doc_drift.go`, `plan/known-failures.md:11-24`, `plan/known-failures.md:37-126`, `plan/known-failures.md:154-205`, `plan/known-failures.md:318-323` | A release can still ship without a clean, reproducible signal from the intended runner or clean Docker substitute. Current source state is no longer blocked by tracked remediation edits, but release-candidate evidence still has to come from a clean source snapshot. | Remove, ignore, or intentionally include the untracked comparison doc, make `make ze-verify` green on a clean Linux runner or via `make ze-release-check`, and triage every known failure as blocker, platform-only, or post-release. |
| P0-2 | Test evidence | Closed in code. Test-only plugins are excluded from production aggregation, and the eight egress-filter cases now assert destination-peer wire behavior. | `internal/component/plugin/all/all.go:81-139`, `cmd/ze/plugins_zetest.go:1-7`, `cmd/ze/main_test.go:22`, `test/plugin/community-strip.ci`, `test/plugin/forward-overflow-two-tier.ci`, `test/plugin/forward-two-tier-under-load.ci`, `test/plugin/role-otc-*.ci` | Regression risk if production aggregation imports test plugins again or wire assertions are weakened. | Preserve the production/test plugin split and keep destination `expect=bgp` assertions in the release gate. |
| P0-3 | Config/API validation and commit | Closed in code for validation and saved-config rollback. API commits validate before save and roll back the saved config if the commit hook or reload path fails; YANG enum/range/pattern checks are enforced; static/API/CLI validation runs registered in-process config verifiers; parser paths reject duplicate list keys; live external plugin `OnConfigVerify` callbacks are documented as reload/commit-time only. | `internal/component/api/config_session.go`, `internal/component/config/parser_list.go`, `internal/component/config/parser_freeform.go`, `internal/component/config/plugin_verify.go`, `internal/component/config/yang_schema.go:583-603`, `internal/component/config/schema.go:748-783` | Regression risk if rollback, duplicate-key rejection, or static/live plugin verifier scope drifts. Runtime side effects after a later global reload failure remain covered by P0-4, not this row. | Keep rollback, duplicate-key, and verifier-scope tests/docs in the gate. |
| P0-4 | Reload atomicity | Code-remediated for the reviewed reload paths. Hub `doReload` snapshots config-provider roots before mutation and rolls plugin config, provider roots, and subsystems back if subsystem reload fails. Plugin-server reload has transaction machinery (`plugin/server/reload.go` verify-apply with txLock and rollback). Orchestrator-mode changed-plugin replacement now pre-starts replacements before removing old handlers, and reload-added/replaced subsystems update the frozen dispatch snapshot. | `cmd/ze/hub/main.go` (provider snapshot + rollback), `cmd/ze/hub/main_test.go` (rollback regression), `internal/component/plugin/server/reload.go:232-266` (config reload transactions), `internal/component/hub/reload.go` (transactional changed-plugin replacement), `internal/component/hub/reload_test.go`, `internal/component/plugin/server/subsystem.go`, `internal/component/plugin/server/subsystem_test.go` | Regression risk if future reload paths mutate runtime state before verification or without rollback hooks. Privileged component side effects remain covered under dataplane evidence, not this row. | Keep provider/subsystem rollback, changed-plugin failure, and frozen-snapshot reload tests in the gate. |
| P0-5 | Authorization | Closed in code. SSH streaming commands now use dispatcher authorization/accounting with user and remote-address propagation. | `cmd/ze/hub/infra_setup.go:213-236`, `internal/component/ssh/ssh.go:631-640` | Regression risk if future streaming paths bypass dispatcher wrappers. | Keep denied streaming-command and accounting tests in the gate. |
| P0-6 | API transport security | Closed in code for the reviewed exposure. REST rejects non-loopback listeners because it has no TLS transport, and gRPC rejects non-loopback listeners unless auth and TLS are configured. | `internal/component/api/rest/server.go:99-126`, `internal/component/api/grpc/server.go:87-127` | Regression risk if listener policy is weakened or docs drift. | Keep transport policy tests and docs aligned with loopback REST and authenticated TLS gRPC. |
| P0-7 | RADIUS/L2TP security | Closed in code for the original P0. RADIUS responses demux by `(server, identifier)` map key with per-waiter authenticator verification via `VerifyResponseAuth`; CoA/DM requires fresh `Event-Timestamp`; duplicates return cached responses without replaying side effects; CoA/DM now requires valid `Message-Authenticator`. | `internal/component/radius/client.go:46-49` (responseKey), `internal/component/radius/client.go:242-261` (dispatch + auth verify), `internal/component/radius/packet.go`, `internal/plugins/l2tpauthradius/coa.go`, `internal/plugins/l2tpauthradius/coa_test.go` | Regression risk if CoA/DM authentication or replay cache semantics are weakened. | Keep CoA/DM missing/invalid `Message-Authenticator`, replay, and demux tests in the gate. |
| P0-8 | Dataplane ownership | Code-remediated, local privileged evidence added, target-runner evidence still open. FIB, static, and policyroute use distinct Linux route protocol IDs; FIB monitor ignores all Ze-owned producers; static route removal uses exact identity; nft cleanup no longer sweeps unknown `ze_*` tables by prefix alone. FIB privileged integration now covers restart sweep and flush-on-stop preserving static and policyroute routes. nft privileged integration covers same-instance cleanup plus restart reapply without sweeping unknown `ze_*` tables. | `internal/core/rtproto/rtproto.go:5-30`, `internal/plugins/fib/kernel/monitor_linux.go:56-58`, `internal/plugins/static/backend_linux.go:40-54`, `internal/plugins/firewall/nft/backend_linux.go:49-82`, `internal/plugins/fib/kernel/integration_linux_test.go`, `internal/plugins/firewall/nft/integration_linux_test.go`, `Makefile:516-539` | Local Docker-backed privileged runs passed, but release still needs the intended CAP_NET_ADMIN target runner to prove the same behavior in the release environment. | Run `make ze-integration-test` on the target runner and keep route/nft ownership plus multi-producer non-interference in that gate. |

Severity note: web CSRF is still not listed as P0 because the session cookie is `Secure`, `HttpOnly`, and `SameSite=Strict` (`internal/component/web/auth.go:245-252`), and authenticated mutating routes now share the same-origin wrapper (`cmd/ze/hub/main.go:1206-1265`). CSP hardening remains P1.

## P1 High Priority

Resolved P1 findings now tracked as regression coverage: `SplitWireUpdate` nil source context, enhanced route-refresh code 70 decode, explicit REST/gRPC startup failure, authenticated web same-origin checks, authenticated API docs/OpenAPI, telemetry loopback default, MCP SSE write-deadline clearing, TACACS response header validation, policyroute firewall dependency, and exact static route deletion.

| Area | Finding | Evidence | Required action |
|------|---------|----------|-----------------|
| BGP config | Closed in code. Incomplete peers are skipped without dropping valid peers; hard peer errors still fail. | `internal/component/bgp/reactor/config.go`, `internal/component/bgp/reactor/config_test.go`, `internal/component/bgp/config/loader_create.go` | Keep partial-valid-peer and hard-error regression tests in the gate. |
| BGP protocol | Closed in code for strict unsupported-family UPDATE handling. Non-negotiated MP_REACH_NLRI is rejected before plugin delivery, sends UPDATE Message Error / Optional Attribute Error, and closes the session. | `internal/component/bgp/reactor/session_read.go`, `internal/component/bgp/reactor/session_test.go` | Keep the non-negotiated MP family NOTIFICATION regression test in the gate. |
| Plugin lifecycle | Closed in code. Config-path auto-load failures now fail closed during startup and reload when a required config-root plugin cannot load. | `internal/component/plugin/server/startup_autoload.go`, `internal/component/plugin/server/reload.go`, `internal/component/plugin/server/reload_test.go` | Keep reload auto-load fail-closed tests in the gate. |
| Hub reload | Closed in code. Orchestrator-mode reload diffs the full plugin definition and restarts when executable/config source changes. | `internal/component/hub/reload.go`, `internal/component/hub/reload_test.go` | Keep same-name changed-`run` restart tests in the gate. |
| Web CSP | Closed in code. CSP is now `default-src 'self'; script-src 'self'; style-src 'self'`, with inline scripts/styles/handlers moved to static assets or CSS classes. | `internal/component/web/auth.go`, `internal/component/web/assets/cli.js`, `internal/component/web/assets/notification.js`, `internal/component/web/render_test.go`, web templates | Keep security-header and no-inline-template tests in the gate. |
| Interface management | Closed in code with local privileged evidence. Reload deletes only interfaces Ze managed in the previous config and which disappeared from the current config; first apply does not adopt/delete arbitrary existing manageable links. Privileged netns tests now prove both behaviors against real kernel links. | `internal/component/iface/config.go`, `internal/component/iface/config_test.go`, `internal/component/iface/config_integration_linux_test.go` | Keep ownership-scoped deletion tests in the gate and run `make ze-integration-iface-test` on the target runner. |
| Interface rollback | Code-remediated with local privileged evidence for reviewed apply failures. Ownership-scoped deletion prevents rollback/reload from deleting unrelated manageable links. Interface apply now aborts on the first mutating failure and rolls back successful steps that have exact inverses, including created interfaces, address changes, bridge ports, mirrors, and selected property changes; changed tunnel recreation journals the old tunnel recreate before creating the replacement. Privileged netns tests now prove created links are removed after a later bridge-member failure. | `internal/component/iface/config.go`, `internal/component/iface/config_test.go`, `internal/component/iface/config_integration_linux_test.go`, `internal/component/iface/register.go` | Keep scoped rollback regression tests in the gate, and run `make ze-integration-iface-test` on the target runner. |
| Traffic control | Code-remediated for reviewed in-process safety paths; local privileged tc evidence and real VPP traffic evidence added. VPP apply has undo-on-error, same-run removal reconciliation, stale-index tolerant deletes, startup scan for stale `ze/` policers, optional dependency ordering on the VPP component when both roots are configured, cancellation-safe waiting for the VPP connector during cold startup, and same-process rebind on UPDATE so a VPP-side output unbind converges on the next Apply without requiring a Ze restart. Netlink now snapshots the original root qdisc before replacement, persists the snapshot under the config state directory, restores via `RestoreOriginal`/`Close`, rejects generic qdiscs and root class/filter state before changing tc state, and no longer synthesizes `fq_codel` on traffic-control removal. Privileged netns integration snapshots a real `fq` qdisc, applies HTB, reloads the persisted snapshot into a fresh backend, and restores the original qdisc. Real VPP evidence covers FIB add/withdraw, traffic policer apply/bind, same-config Ze restart preservation, and stale Ze traffic policer cleanup. | `internal/plugins/traffic/netlink/backend_linux.go`, `internal/plugins/traffic/netlink/snapshot_linux.go`, `internal/plugins/traffic/netlink/backend_linux_test.go`, `internal/plugins/traffic/netlink/integration_linux_test.go`, `internal/plugins/traffic/netlink/translate_linux.go`, `internal/component/traffic/register.go`, `internal/plugins/traffic/vpp/backend_linux.go`, `internal/plugins/traffic/vpp/apply_test.go`, `scripts/evidence/effective-vpp.py`, `plan/deferrals.md:200-213` | Run privileged tc integration on the target runner and add reactor-level boot/reload kernel-state proof when CI can expose a managed interface. |
| L2TP/PPP | Closed in code for the reviewed safety policy. PPP required auth rejects `AuthMethodNone`; L2TP defaults have finite caps and CHAP-MD5 auth; no-auth requires explicit `allow-no-auth true`; hidden mandatory AVPs fail closed. Docker-backed `xl2tpd` evidence proves a real LAC can establish Ze's control tunnel and incoming-call session. A peer-isolated Docker lab (`make ze-deployment-l2tp-ppp-docker-test`) now exists with Ze LNS, real xl2tpd/pppd LAC, and FRR in separate containers; it proves PPP LCP/IPCP, kernel pppN, dataplane ping, and BGP route redistribution from a live PPP session. The lab requires host kernel PPPoL2TP support; proof is open until it passes on a supported host. | `internal/component/ppp/start_session.go`, `internal/component/ppp/session_run.go`, `internal/component/l2tp/config.go`, `internal/component/l2tp/avp.go`, `internal/component/l2tp/schema/ze-l2tp-conf.yang`, `scripts/evidence/effective-l2tp-peer.py`, `scripts/evidence/effective-l2tp-ppp.py`, `test/l2tp-interop/run.py`, `test/l2tp-interop/lab.py`, `Makefile` | Keep PPP required-auth, L2TP auth-policy, caps, reload, CLI, hidden mandatory AVP tests, and the external control/session evidence in the gate. Run `make ze-deployment-l2tp-ppp-test` or `make ze-deployment-l2tp-ppp-docker-test` when the target runner has suitable L2TP PPP support. |
| RADIUS policy | Closed in code for exact-or-reject. Access-Accept rejects unsupported deployment-affecting attributes (`Framed-IP-Address`, `Framed-IP-Netmask`, `Framed-Pool`, `Filter-Id`, `Session-Timeout`, `Idle-Timeout`, `Acct-Interim-Interval`) instead of accepting and ignoring them. | `internal/plugins/l2tpauthradius/handler.go`, `internal/plugins/l2tpauthradius/handler_test.go`, `docs/guide/l2tp.md`, `docs/guide/plugins.md`, `docs/features.md` | Keep unsupported Access-Accept attribute rejection tests and docs aligned. |
| RADIUS CoA policy | Closed in code. CoA/DM requires valid `Message-Authenticator`; missing or invalid packets are rejected before side effects. | `internal/plugins/l2tpauthradius/coa.go`, `internal/component/radius/packet.go`, `internal/plugins/l2tpauthradius/coa_test.go` | Keep missing/invalid `Message-Authenticator` tests in the gate. |
| TACACS+ | Closed in code. Configurable `strict-fallback` denies local fallback on TACACS infrastructure failure when enabled. | `internal/component/tacacs/authorizer.go`, `internal/component/tacacs/config.go`, `internal/component/tacacs/schema/ze-tacacs-conf.yang` | Keep strict fallback config and authorizer tests in the gate. |
| DNS/NTP | Closed in code for the reviewed production-safety policy. DNS resolver no longer falls back to public recursive DNS when no resolver is configured; NTP has bounded `max-step` with explicit `0` opt-out. | `internal/component/resolve/dns/resolver.go`, `internal/plugins/ntp/ntp.go`, `internal/plugins/ntp/schema/ze-ntp-conf.yang` | Keep no-public-fallback and NTP `max-step` tests in the gate. |

## P2 Readiness Gaps

| Area | Finding | Evidence | Action |
|------|---------|----------|--------|
| Documentation | Closed for drift found in this review pass. Functional-test docs, Makefile help, and drift checks agree on the 11-suite release gate; chaos-web and stress runner wording is current; TACACS strict fallback and RADIUS Access-Accept exact-or-reject docs are aligned. | `docs/functional-tests.md`, `Makefile`, `docs/guide/tacacs.md`, `docs/guide/l2tp.md`, `docs/guide/plugins.md`, `docs/features.md`, `scripts/docvalid/doc_drift.go` | Keep gate claims derived or drift-checked when suite membership changes. |
| Feature inventory | Current repo has `docs/features.md` status labels for every row: supported, experimental, partial, rejected, stub-backed, or future. It includes explicit partial/stub caveats for REST/gRPC completion, API streaming scope, L2TP control/session evidence versus full PPP/NCP/kernel peer proof, VPP, dataplane evidence, config validation, reload atomicity, TACACS strict mode, DNS resolver policy, and RADIUS Access-Accept rejection. | `docs/features.md:5-113` | Keep status labels current as feature claims change. |
| Doc drift tooling | Closed in code for the release-gate help gap. Drift checks cover README test-count claims, feature inventory status labels, functional-test release-gate claims derived from the Makefile, and Makefile help drift. | `scripts/docvalid/doc_drift.go`, `Makefile:921-924` | Add new user-facing release docs and help claims to drift checks when they contain factual counts or status lists. |
| README counts | README test counts are conservative, dated approximate claims instead of brittle exact totals. | `README.md:3`, `README.md:57-60` | Keep approximate/date wording or derive exact counts in generated docs. |
| API streaming | Closed in code for registered streaming commands. The production hub wires REST SSE and gRPC `Stream` through `APIEngine.Stream` to the pluginserver streaming-handler registry, so commands such as `monitor event` use the same handler, authorization, caller metadata, and accounting path as SSH monitor commands. OpenAPI remains generic command-oriented rather than expanding every streaming command, and generic completion remains future work. | `cmd/ze/hub/api.go`, `internal/component/api/engine.go:119-130`, `internal/component/api/rest/server.go:623-667`, `internal/component/api/grpc/server.go:358-391`, `docs/guide/api.md:77-83`, `docs/guide/api.md:173-179` | Keep API streaming and authorization/accounting regression tests in the gate; keep docs explicit that only registered streaming commands are accepted. |
| Config sessions | Per-session serialization is implemented and covered. API saved-config rollback is also code-closed under P0-3; reviewed runtime reload rollback paths are code-closed under P0-4. | `internal/component/api/config_session.go:43-51`, `internal/component/api/config_session.go:207-245`, `internal/component/api/config_session_test.go:249-280`, `cmd/ze/hub/main.go:817-865` | Keep concurrency, rollback, and reload transaction tests in the gate. |
| Test infrastructure | Code-remediated for runner-level self-collision. `ze-test` BGP and VPP runners now use `ReservePorts`, which holds advisory per-port locks for the suite lifetime while leaving the TCP ports bindable by child `ze` and `ze-peer` processes. This coordinates concurrent `ze-test` processes; it does not reserve ports from arbitrary external processes. | `internal/test/runner/ports.go`, `internal/test/runner/ports_test.go`, `cmd/ze-test/bgp.go`, `cmd/ze-test/vpp.go`, `plan/known-failures.md:92-126` | Keep runner reservation tests and continue investigating any remaining flakes caused by external port consumers, loopback alias setup, or process cleanup. |
| Open deferrals | Open release-relevant rows remain for full L2TP PPP/NCP/kernel peer tests, target-runner privileged evidence, BMP Loc-RIB, and raw plugin IPC. L2TP route redistribution producer, synthetic BGP advertise/withdraw proof, real VPP FIB add/withdraw evidence, real VPP traffic apply/restart cleanup evidence, same-process VPP traffic rebind, and external L2TP control/session evidence are now closed. Duplicate parser keys, L2TP auth/caps/hidden mandatory AVPs, DNS/NTP safety policy, RADIUS Access-Accept policy, VPP traffic orphan scan, and in-process static plugin validation parity are code-remediated or re-triaged. | `plan/deferrals.md:162-181`, `plan/deferrals.md:200-213`, `plan/deferrals.md:231` | Triage remaining open rows into release blocker, scoped deployment exclusion, post-deployment backlog, or explicitly unsupported. |

## Deployment Plan

### Phase 0: Restore Trust In The Gate

Exit criteria:

- Worktree is clean, or every dirty change is intentionally part of the release candidate.
- `make ze-verify` green on a clean Linux runner.
- Woodpecker/Codeberg CI runs at least `make ze-verify` on every PR.
- `plan/known-failures.md` contains no untriaged release blockers.
- Release-gate documentation, Makefile help, and drift checks agree on the same suite set.

Work items:

- Keep the authoritative CI gate for `make ze-verify` documented and enabled.
- Resolve or classify the known flakes and platform-only failures that remain in `plan/known-failures.md`.
- Keep Makefile help, docs, and drift checks aligned for `ze-functional-test`.
- Fix stale non-gate target wording for `ze-chaos-web-test` and the `ze-stress-test` runner message.
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
- Keep strict CSP/no-inline coverage without breaking HTMX/Finder behavior.

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
- Keep hub subsystem/provider rollback and orchestrator changed-plugin replacement covered.
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
- Keep interface ownership/adoption model covered.
- Keep interface transactional apply rollback covered, and add privileged kernel-state failure tests.
- Keep traffic-control original-qdisc restore/reconcile semantics covered, and add privileged kernel-state evidence.
- Keep VPP traffic startup orphan scan covered with real-daemon proof, and keep the same-process VPP rebind unit test in the Linux-tagged package gate.
- Add VPP real-daemon CI for traffic/FIB idempotency and restart cases.

Traffic netlink original-qdisc restore semantics:

- Ownership starts when the `tc` backend first successfully applies `traffic-control` to an interface. Before replacing the root qdisc, Ze must snapshot the existing root qdisc and enough class/filter state to restore it exactly.
- The snapshot must survive daemon restart for every still-managed interface. If Ze cannot prove a persisted snapshot belongs to the interface and current link identity, commit or reload must fail before changing qdisc state.
- Removing an interface from `traffic-control`, removing the whole `traffic-control` section, rollback, and clean shutdown must restore the saved qdisc state exactly and then delete the snapshot. Ze must not synthesize `fq_codel` as a cleanup default.
- If the current qdisc cannot be snapshotted or restored exactly, verify or apply must reject with an operator-facing error before replacing it. Restoring only the root qdisc type is an approximation and is not acceptable.
- Privileged tests must cover default classless qdisc restore, pre-existing non-default qdisc restore, unsupported or unrestorable qdisc rejection, daemon restart with persisted snapshot, rollback after partial failure, and the current `fq_codel` reset regression.

### Phase 4: BGP And Routing Correctness

Exit criteria:

- BGP config partial-edit behavior is explicit and tested.
- Unsupported-family and malformed UPDATE paths send correct NOTIFICATION or documented RFC 7606 behavior.
- BGP route server/RR/filter tests prove wire-level egress behavior.
- BMP Loc-RIB unsupported status is explicit, or implementation lands.
- LLGR/RIB-inject/redistribution support claims are aligned with implemented behavior.

Work items:

- Keep incomplete-peer handling covered.
- Keep unsupported-family NOTIFICATION behavior tests covered.
- Keep `SplitWireUpdate`, ADD-PATH, and enhanced route-refresh regression coverage in the gate.
- Triage LLGR/BMP/RIB-inject open deferrals before support claims.

### Phase 5: Access Protocol Deployment Proof

Scope note: Phase 5 is required for deployments that use subscriber access protocols such as L2TP/PPP/RADIUS. A BGP-only deployment can treat this phase as out of its launch critical path after the feature inventory clearly labels subscriber access as not in scope for that deployment.

Exit criteria:

- L2TP/PPP/RADIUS path has at least one full PPP/NCP/kernel peer integration scenario. For Ze's current LNS path this requires a LAC peer such as `xl2tpd`, `pppd`, and PPPoL2TP kernel support. Docker-backed `xl2tpd` evidence already covers external control tunnel and incoming-call session setup.
- RADIUS-assigned address/filter/rate/session attributes are implemented or explicitly rejected.
- Hidden AVP behavior is implemented or rejected fail-closed.
- Mandatory authentication cannot negotiate down to `AuthMethodNone` unless the operator explicitly allows no-auth.
- L2TP route redistribution has end-to-end advertise and withdraw proof.

Work items:

- Add full L2TP + PPP + NCP functional peer test.
- Keep key RADIUS Access-Accept attributes implemented or explicitly rejected.
- Keep mandatory-hidden AVP rejection covered.
- Keep non-zero tunnel/session safety defaults covered.
- Keep `spec-bgp-redistribute`, `spec-l2tp-7c-redistribute`, and `make ze-deployment-l2tp-test` coverage, and add the full L2TP + PPP + NCP peer integration proof.

### Phase 6: Documentation Truth Pass

Exit criteria:

- `README.md`, `docs/features.md`, `docs/functional-tests.md`, command reference, and API docs agree with code and release gate.
- Each user-facing feature is labeled supported, experimental, partial, stub-backed, rejected, or future.
- Security deployment docs require explicit authz profiles and safe listener binding.
- API streaming scope, VPP real-daemon status, L2TP access gaps, and validation limitations stay explicit until closed.

Work items:

- Keep Makefile help text aligned for the functional gate.
- Extend drift checks to any new release-gate/help/status claims.
- Keep REST/gRPC API docs aligned with auth, TLS, streaming, and docs exposure behavior.
- Correct L2TP, VPP, BMP, RIB inject, DNS/NTP, TACACS strict-mode, and Modular Deployment claims as those gaps close.
- Re-triage stale deferral rows, especially static validation parity.

## Verification Matrix

Target status: the `make` targets below exist in the main `Makefile`. Some require Docker, root, CAP_NET_ADMIN, network namespaces, or external tools; those are real targets, not aspirational checks, but they need suitable runners. The `bin/ze-test firewall`, `bin/ze-test traffic`, and `bin/ze-test vpp` subcommands are registered in `cmd/ze-test`. The 2026-05-02 static refresh did not rerun these gates.

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
make ze-integration-firewall-test # nft ownership integration tests
make ze-stress-test        # requires Linux, root, netns, iproute2/ethtool; traffic uses in-tree ze-test peer injector, not BNG Blaster
make ze-race-reactor       # reactor race stress, required for reactor concurrency changes
make ze-linux-test         # Docker-backed Linux Go unit tests; defaults to traffic/vpp
make ze-integration-traffic-test # tc qdisc snapshot/restore integration tests
make ze-release-check # clean Docker release-candidate ze-verify evidence
make ze-deployment-preflight # checks target-runner, real VPP, and external L2TP peer tooling
make ze-deployment-vpp-test # Docker-backed real VPP FIB and traffic-control evidence
make ze-deployment-l2tp-test # Docker-backed real xl2tpd LAC control/session evidence
make ze-deployment-l2tp-ppp-test # Linux-only real xl2tpd/pppd PPP/NCP evidence
make ze-deployment-l2tp-ppp-docker-test # Docker-wrapped full xl2tpd/pppd PPP/NCP evidence
bin/ze-test firewall --all # requires nft/iptables privileges for kernel-state tests
bin/ze-test traffic --all  # traffic-control runner, some cases need CAP_NET_ADMIN
bin/ze-test vpp --all      # GoVPP-stub-backed VPP runner
python3 test/interop/run.py 33-bfd-frr # single BFD FRR interop scenario, requires Docker
```

Current verification status for this refresh:

```text
2026-05-02 static refresh:
No test, build, or lint command was rerun for this document update.
git status --short shows one unrelated untracked file:
?? plan/comparison/mikrotik-winbox-vs-ze.md

Tracked files are clean before this document edit.

Prior 2026-05-02 local run recorded by the remediation pass:
make ze-verify
PASS: lint, cached unit tests, race unit tests, build, all 11 functional suites, and ExaBGP compatibility 37/37.

Additional focused checks from the same pass:
go test -run 'TestParserInlineListDuplicatePathInformation|TestMigrateStaticDuplicatePathInformation' ./internal/component/config ./internal/exabgp/migration
PASS.
go test ./internal/component/config ./internal/exabgp/migration
PASS.
make ze-exabgp-test
PASS: 37/37.

2026-05-02 follow-up hardening focused checks:
go test ./internal/component/bgp/reactor ./internal/component/bgp/config -run 'TestPeersFromTree|TestCheckRequiredFields' -count=1
PASS.
go test ./internal/component/web -run 'TestSecurityHeaders|TestIntegration_SecurityHeaders|TestTemplatesAvoidInlineScriptAndStyle|TestHandleCLIPageAvoidsInlineStyle' -count=1
PASS.
go test ./internal/plugins/l2tpauthradius -run 'TestAccessAccept|TestRADIUSAuthAccessAcceptRejectsUnsupportedAttribute' -count=1
PASS.
go test ./cmd/ze/hub -run 'TestRollbackReloadRestoresProviderOnSubsystemFailure' -count=1
PASS.
go test ./internal/component/iface -run 'TestReconcileOnReady_(PreservesUnownedManageableInterface|PrunesPreviouslyManagedInterface)' -count=1
PASS.
go test ./internal/component/hub -run 'TestOrchestratorReloadChangedPlugin(StartFailurePreservesOld|RunKeepsSubsystem)' -count=1
PASS.
go test ./internal/component/plugin/server -run 'TestSubsystemManager(Register|Replace|Unregister)AfterFreeze' -count=1
PASS.
make ze-linux-test
PASS: Docker-backed Linux Go unit tests for ./internal/plugins/traffic/vpp.
make ze-doc-drift
PASS: No documentation drift detected.
git diff --check
PASS.

2026-05-02 follow-up tc original-qdisc focused checks:
go test ./internal/component/traffic ./internal/plugins/l2tpshaper -count=1
PASS.
make ze-linux-test ZE_LINUX_TEST_PACKAGES="./internal/plugins/traffic/netlink ./internal/plugins/traffic/vpp"
PASS: Docker-backed Linux Go unit tests for tc netlink and VPP traffic backends.

2026-05-03 follow-up PPP LCP and runner port checks:
go test ./internal/component/ppp -run 'TestHandleLCPPacketOpenedRXRDoesNotReenterOpened|TestLCPEcho|TestLCPFSMRXRInOpened' -count=1
PASS.
go test -race ./internal/component/ppp -count=1
PASS.
go test ./internal/test/runner -run 'TestFindFreePortRange|TestReservePorts|TestAllocatePorts|TestCheckPortAvailable|TestPortRangeString' -count=1
PASS.
go test ./cmd/ze-test ./internal/test/runner -count=1
PASS.
go test -race ./internal/test/runner -count=1
PASS.

2026-05-03 follow-up interface rollback checks:
go test ./internal/component/iface -run 'TestApplyConfigRollsBackCreatedInterfaceOnAddressFailure|TestApplyConfigStopsAfterFirstCreateFailure|TestReconcileOnReady|TestApplyTunnels|TestApplyWireguards|TestIfaceApplyJournal' -count=1
PASS.
go test ./internal/component/iface -count=1
PASS.
go test -race ./internal/component/iface -count=1
PASS.
make ze-lint
PASS: 0 issues.
git diff --check
PASS.

2026-05-03 follow-up plugin flake checks:
bin/ze-test bgp plugin nexthop -v
PASS.
bin/ze-test bgp plugin watchdog -v
PASS.
bin/ze-test bgp plugin watchdog-med-override -v
PASS.
bin/ze-test bgp plugin bfd-auth-meticulous-persist -v
PASS.
bin/ze-test bgp plugin prefix-maximum-enforce -v
PASS.
bin/ze-test bgp plugin show-errors-received -v
PASS.
bin/ze-test bgp plugin -c 3 nexthop
PASS: 3/3.
bin/ze-test bgp plugin -c 3 watchdog
PASS: 3/3.
bin/ze-test bgp plugin -c 3 watchdog-med-override
PASS: 3/3.
bin/ze-test bgp plugin -c 3 bfd-auth-meticulous-persist
PASS: 3/3.
bin/ze-test bgp plugin -c 3 prefix-maximum-enforce
PASS: 3/3.
bin/ze-test bgp plugin -c 3 show-errors-received
PASS: 3/3.
bin/ze-test bgp encode addpath -v
PASS.
bin/ze-test bgp plugin fib-vpp-coexist-with-fib-kernel -v
PASS.
bin/ze-test bgp plugin fib-vpp-plugin-load -v
PASS.
make ze-exabgp-test
PASS: 37/37.
go test -race ./internal/component/bgp/reactor -run TestFwdPool_StopUnblocksDispatch -count=500
PASS.
make ze-linux-test ZE_LINUX_TEST_PACKAGES="./internal/plugins/firewall/nft"
PASS.
git diff --check
PASS.

2026-05-03 follow-up static plugin validation deferral triage:
go test ./internal/component/config -run 'TestVerifyPluginConfig|TestValidateTree' -count=1
PASS.
go test ./internal/component/cli -run TestValidateRunsPluginConfigVerifier -count=1
PASS.
go test ./cmd/ze/config -run Test -count=1
PASS.
make ze-doc-drift
PASS: No documentation drift detected.
make ze-exabgp-test
PASS: 37/37.
git diff --check
PASS.

2026-05-03 follow-up API streaming checks:
go test ./cmd/ze/hub -run 'TestAPI(StreamSource|Executor|ConfigValidation)' -count=1
PASS.
go test -race ./cmd/ze/hub -run TestAPIStreamSource -count=1
PASS.
go test ./internal/component/api/... -count=1
PASS.
go test ./cmd/ze/hub -count=1
PASS.
make ze-doc-drift
PASS: No documentation drift detected.
git diff --check
PASS.

2026-05-03 follow-up L2TP redistribute and FIB ownership triage:
go test ./internal/plugins/fib/kernel -count=1
PASS.
go test ./internal/component/config/redistribute ./internal/component/bgp/plugins/redistribute_egress ./internal/component/l2tp -run 'TestEvaluator|TestHandleBatch|TestSubscribe|TestCommandText|TestWithdrawText|TestObserver|TestRegisterL2TPSources' -count=1
PASS.
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test -tags=integration ./internal/plugins/fib/kernel -count=1
PASS.
make ze-doc-drift
PASS: No documentation drift detected.
git diff --check
PASS.

2026-05-03 follow-up dataplane privileged evidence:
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test -tags=integration ./internal/plugins/firewall/nft -count=1
PASS.
go test ./internal/plugins/fib/kernel -count=1
PASS.
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test -tags=integration ./internal/plugins/fib/kernel -count=1
PASS.
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test -tags=integration ./internal/plugins/traffic/netlink -count=1
PASS.
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test -tags=integration ./internal/component/iface -run 'TestIntegrationApplyConfig' -count=1
PASS.
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9 go test -tags=integration -count=1 -race -timeout 120s ./internal/component/iface/...
PASS.
go test ./internal/component/iface -count=1
PASS.
go test ./internal/plugins/iface/netlink -count=1
PASS.
docker run --rm -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test ./internal/plugins/traffic/netlink -count=1
PASS.
make ze-linux-test ZE_LINUX_TEST_PACKAGES="./internal/plugins/firewall/nft ./internal/plugins/fib/kernel ./internal/plugins/traffic/netlink ./internal/plugins/traffic/vpp"
PASS.
make -n ze-integration-test
PASS: dry-run expansion includes iface, FIB, firewall nft, and traffic-control netlink integration targets.
docker run --rm --privileged -v "$PWD:/src" -w /src -e HOME=/tmp -e GOCACHE=/src/tmp/linux-go-cache -e GOMODCACHE=/src/tmp/linux-gomodcache golang:1.25.9-alpine go test -tags=integration ./internal/plugins/firewall/nft ./internal/plugins/fib/kernel ./internal/plugins/traffic/netlink -count=1
PASS.
go test ./internal/plugins/sysrib -count=1
PASS.
make ze-deployment-vpp-test
PASS: real VPP daemon FIB contains then withdraws 10.20.0.0/24; real VPP traffic policer `ze/loop0/default` is created and bound; same-config Ze restart preserves it; startup cleanup removes the stale Ze traffic policer when the next config removes the interface.
make ze-deployment-l2tp-test
PASS: real xl2tpd LAC establishes Ze L2TP control tunnel and incoming-call session.
python3 -m py_compile scripts/evidence/effective-l2tp-ppp.py scripts/evidence/docker-run.py
PASS.
go test ./internal/component/l2tp -count=1
PASS.
bash -n scripts/evidence/effective-verify.sh
PASS.
make -n ze-release-check
PASS: target expands to clean Docker evidence script. Full run not attempted because this worktree is intentionally dirty and user asked not to rerun full `make ze-verify`.
make ze-doc-drift
PASS: No documentation drift detected.
git diff --check
PASS.

2026-05-03 follow-up VPP traffic evidence and deployment evidence attempts:
go test ./internal/component/traffic -count=1
PASS.
make ze-linux-test ZE_LINUX_TEST_PACKAGES="./internal/plugins/traffic/vpp"
PASS.
python3 -m py_compile scripts/evidence/effective-vpp.py
PASS.
make ze-deployment-vpp-test
PASS: real VPP FIB add/withdraw, real VPP traffic policer apply/bind, same-config Ze restart preservation, and stale Ze traffic policer startup cleanup.
bin/ze-test bgp plugin -c 10 prefix-maximum-enforce
PASS: 10/10.
make ze-release-check
FAIL (strict, expected in this dirty worktree): clean release-candidate evidence requires a clean git worktree. The current tree has tracked edits from this remediation plus the unrelated untracked `plan/comparison/mikrotik-winbox-vs-ze.md`.
make ze-deployment-preflight
FAIL (strict, expected in this environment): `target-runner` is missing, and full L2TP PPP/NCP peer evidence lacks `xl2tpd`, `pppd`, `/dev/ppp`, `iproute2`, and PPPoL2TP kernel support.
make ze-deployment-l2tp-ppp-test
FAIL (expected on macOS): full L2TP PPP/NCP evidence requires Linux.
make ze-deployment-l2tp-ppp-docker-test
FAIL (strict, expected on this Docker Desktop kernel): the container installs `xl2tpd`, `ppp`, `iproute2`, and Python, then rejects the run because PPPoL2TP kernel support is missing.
target-runner
UNAVAILABLE: `target-runner` is not installed in this environment. Docker-backed clean-source substitute exists via `make ze-release-check` but requires a clean worktree and runs full `make ze-verify`.
vpp / vppctl
AVAILABLE through Docker-backed `make ze-deployment-vpp-test`; host-native `vpp` and `vppctl` remain unavailable.
full L2TP PPP/NCP peer proof
UNAVAILABLE: `make ze-deployment-l2tp-ppp-test` now exists, but this environment lacks `xl2tpd`, `pppd`, `/dev/ppp`, and PPPoL2TP kernel support for full PPP/NCP proof. Docker-backed `xl2tpd` control/session evidence is available through `make ze-deployment-l2tp-test`.
make ze-deployment-l2tp-ppp-test
FAIL (expected on macOS): full L2TP PPP/NCP evidence requires Linux.
make ze-deployment-l2tp-ppp-docker-test
FAIL (strict, expected on this Docker Desktop kernel): the container starts, installs `xl2tpd`/`ppp`/`iproute2`, and then rejects the run because PPPoL2TP kernel support is missing.
make ze-deployment-preflight
FAIL (strict, expected in this environment): Docker can run the clean `ze-verify` substitute, real VPP evidence, and L2TP control/session evidence. `target-runner`, `xl2tpd`, `pppd`, `/dev/ppp`, `iproute2`, and PPPoL2TP kernel support remain missing, so deployment evidence is not 100% complete.
make ze-verify
NOT RERUN: prior commit-time run already passed; an attempted local rerun was stopped to avoid wasting time per user instruction.

Do not treat this static refresh as final release-candidate evidence until the target runner verifies the release candidate with a clean or intentionally scoped worktree.
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
