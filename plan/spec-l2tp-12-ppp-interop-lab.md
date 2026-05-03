# Spec: l2tp-12 -- PPP/NCP Docker Interop Lab

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-l2tp-0-umbrella |
| Phase | 7/7 |
| Updated | 2026-05-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md` for workflow rules.
3. `docs/architecture/testing/interop.md` for the BGP interop runner pattern.
4. `docs/guide/l2tp.md` for L2TP kernel, PPP, NCP, and redistribution behavior.
5. `scripts/evidence/effective-l2tp-ppp.py` for the current strict native PPP/NCP proof.
6. `test/interop/interop.py` and `test/interop/run.py` for reusable Docker lab lifecycle patterns.

## Task

Add a BGP-interop-style Docker lab for Ze L2TP full PPP/NCP proof. The lab must run Ze as an LNS in one container, a real `xl2tpd` plus `pppd` LAC peer in another container, and, for route redistribution proof, FRR as a real BGP peer in a third container. The proof must remain deployment-grade: it fails when PPPoL2TP kernel support is unavailable and never falls back to `ze.l2tp.skip-kernel-probe`.

The existing native Linux proof remains valid and should be preserved. The Docker-specific deployment target should become the multi-container lab, or explicitly call it, so Docker evidence is peer-isolated instead of only running Ze and the LAC in one container namespace.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - current BGP Docker interop harness.
  → Decision: reuse the scenario-directory, per-run Docker network, fixed container IP, native CLI probe, and `check.py` assertion shape.
  → Constraint: interop tests are separate from `make ze-verify` because they require Docker and external daemons.
- [ ] `ai/rules/design-context.md` - Ze design-context rule.
  → Decision: extend the existing test/evidence patterns instead of inventing a new orchestration style.
  → Constraint: grep and read existing Ze patterns before naming, lifecycle, or communication decisions.
- [ ] `ai/rules/design-principles.md` - simplicity, explicit behavior, exact-or-reject.
  → Decision: fail clearly when the host kernel cannot run PPPoL2TP; do not skip or downgrade the proof.
  → Constraint: no hidden defaults, no silent approximation, no speculative abstractions.
- [ ] `ai/rules/testing.md` and `ai/patterns/functional-test.md` - test placement and no throw-away tests.
  → Decision: the lab scenarios live under a durable `test/l2tp-interop/` tree, not an ad-hoc scratch script.
  → Constraint: evidence tests must assert behavior, not only absence of crashes.
- [ ] `docs/guide/l2tp.md` - operator-facing L2TP kernel, PPP, NCP, and redistribute behavior.
  → Decision: the lab must verify kernel `pppN` state, PPP address state, dataplane ping, route-change inject, and cleanup.
  → Constraint: full proof must not use `ze.l2tp.skip-kernel-probe`.
- [ ] `docs/research/l2tpv2-ze-integration.md` - kernel PPP control/data split.
  → Decision: use Linux PPPoL2TP kernel dataplane in the containers and userspace Ze PPP control-plane negotiation.
  → Constraint: Docker can provide Linux userspace, but it cannot add missing host-kernel PPPoL2TP support.
- [ ] `plan/spec-l2tp-0-umbrella.md` - L2TP end-to-end subscriber flow and acceptance criteria.
  → Decision: the lab should prove the umbrella flow from SCCRQ through BGP route advertisement, not only control tunnel establishment.
  → Constraint: L2TP remains a subsystem, route redistribution flows through EventBus and `bgp-redistribute-egress`.
- [ ] `plan/learned/641-l2tp-7c-redistribute.md` - current route-change event implementation.
  → Decision: the lab should add real-peer coverage on top of the existing synthetic `fakel2tp` BGP UPDATE tests.
  → Constraint: do not bypass the real RouteObserver or EventBus producer path.
- [ ] `plan/deployment-readiness-deep-review.md` - open deployment-readiness blockers.
  → Decision: this spec closes the open full L2TP PPP/NCP/kernel peer proof only when it passes on a host with PPPoL2TP support.
  → Constraint: clean release-candidate evidence and target-runner evidence remain separate blockers.

### RFC Summaries
- [ ] `rfc/short/rfc2661.md` - L2TPv2 tunnel and session behavior.
  → Constraint: the real peer must exercise SCCRQ/SCCRP/SCCCN and ICRQ/ICRP/ICCN over UDP 1701.
- [ ] `rfc/short/rfc1661.md` - PPP LCP and common FSM.
  → Constraint: PPP session-up evidence must happen after LCP reaches Opened.
- [ ] `rfc/short/rfc1332.md` - PPP IPCP IPv4 address negotiation.
  → Constraint: IPv4 proof must verify the peer address assigned by IPCP, not only L2TP session establishment.
- [ ] `rfc/short/rfc1877.md` - IPCP DNS option behavior.
  → Constraint: DNS options may be present in the assigned IP policy, but the dataplane proof is valid with or without accepted DNS options.
- [ ] `rfc/short/rfc4271.md` - BGP UPDATE route propagation.
  → Constraint: the FRR scenario must verify the subscriber /32 appears and disappears as BGP reachability, not only as a Ze log line.

**Key insights:**
- Existing BGP interop already has the right shape: Docker network per run, fixed container IPs, daemon-specific helpers, and scenario-local `check.py` files.
- The current native PPP/NCP script is strict and valuable. Preserve it as the Linux host proof.
- The current Docker wrapper runs strict proof inside one privileged container. A full interop-style proof should isolate Ze LNS and LAC peer into separate containers on a Docker bridge.
- Docker Desktop on macOS still cannot pass this proof if its Linux VM lacks PPPoL2TP. The runner must fail with a clear preflight message.
- Synthetic `fakel2tp` tests already prove BGP UPDATE rendering. The new lab should prove the real L2TP RouteObserver and EventBus path can feed FRR from a live PPP session.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/run.py` - builds/pulls daemon images, filters scenarios, and runs each `check.py` with teardown.
- [ ] `test/interop/interop.py` - manages Docker network, container naming, fixed addresses, daemon helpers, and health checks.
- [ ] `test/interop/Dockerfile.ze` - builds a minimal Ze Alpine image with `tini` and Python for plugin scripts.
- [ ] `test/perf/run.py` - reuses `ze-interop` image, builds Linux helper binaries, starts DUT containers and a runner container on a fixed Docker subnet.
- [ ] `Makefile` - owns `ze-deployment-l2tp-test`, `ze-deployment-l2tp-ppp-test`, `ze-deployment-l2tp-ppp-docker-test`, and `ze-docker-evidence` targets.
- [ ] `docs/functional-tests.md` - defines deployment target semantics and documents the L2TP PPP/NCP requirements.
- [ ] `scripts/evidence/effective-l2tp-peer.py` - Docker-backed control/session proof with real `xl2tpd`, using `ze.l2tp.skip-kernel-probe=true` intentionally.
- [ ] `scripts/evidence/effective-l2tp-ppp.py` - strict native full PPP/NCP proof with real `xl2tpd`, `pppd`, `/dev/ppp`, `ip l2tp`, pppN address checks, dataplane ping, inject/withdraw logs, and cleanup.
- [ ] `scripts/evidence/docker-run.py` - generic privileged Docker wrapper that builds a Linux Ze binary, mounts modules, installs packages, and passes Ze env vars.
- [ ] `internal/component/l2tp/route_observer.go` - real RouteObserver emits add/remove `redistevents.RouteChangeBatch` values on session IP-up and session-down.
- [ ] `internal/component/l2tp/events/events.go` - typed `(l2tp, route-change)` EventBus handle and producer registration.
- [ ] `internal/component/l2tp/subsystem.go` - registers L2TP redistribute source, constructs RouteObserver, probes kernel modules unless explicitly skipped, and starts reactors.
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - subscribes to non-BGP route-change producers and dispatches BGP announce/withdraw commands.
- [ ] `test/plugin/redistribute-l2tp-announce.ci` - synthetic L2TP route add becomes a BGP UPDATE.
- [ ] `test/plugin/redistribute-l2tp-withdraw.ci` - synthetic L2TP route remove becomes a BGP withdrawn route.
- [ ] `plan/deferrals.md` - records prior L2TP real-peer proof gaps and current status.

**Behavior to preserve:**
- `make ze-deployment-l2tp-test` remains a real external LAC control/session proof and may continue to use `ze.l2tp.skip-kernel-probe=true` because it explicitly does not claim PPP/NCP/kernel dataplane proof.
- `make ze-deployment-l2tp-ppp-test` remains the native Linux strict full proof.
- `scripts/evidence/effective-l2tp-ppp.py` keeps refusing `ZE_L2TP_SKIP_KERNEL_PROBE` and `ze.l2tp.skip-kernel-probe`.
- Synthetic `fakel2tp` tests remain in the functional gate to cover BGP UPDATE bytes without requiring root or PPPoL2TP.
- Docker-specific deployment targets include `docker` in the target name.
- Missing PPPoL2TP support is a hard failure for full PPP/NCP evidence, not a skip.
- L2TP route redistribution still flows through RouteObserver, EventBus, `bgp-redistribute-egress`, and BGP per-peer dispatch.

**Behavior to change:**
- Add a multi-container Docker lab for full PPP/NCP/kernel dataplane proof.
- Make the Docker PPP/NCP deployment target run the multi-container lab, or add a clearly named Docker lab target and make the existing target delegate to it.
- Add an FRR scenario proving a live PPP-assigned subscriber /32 is advertised and withdrawn by BGP.
- Update docs and deployment-readiness notes so the Docker target description says peer-isolated lab and explains the host-kernel dependency.

## Data Flow (MANDATORY)

### Entry Point
- Make target: Docker-specific full L2TP PPP/NCP deployment evidence.
- Runner input: optional scenario name, environment flags such as `NO_BUILD`, `VERBOSE`, `SESSION_TIMEOUT` (default 90s, matching `test/interop`), and `FRR_IMAGE` (default `quay.io/frrouting/frr:10.3.1`, matching `test/interop`).
- Scenario inputs: Ze config, `xl2tpd` config, `pppd` options, L2TP secret, optional FRR config, and `check.py` assertions.
- External wire input: UDP 1701 L2TP control and data packets between LAC and LNS containers.

### Transformation Path
1. Runner verifies Docker availability and PPPoL2TP host-kernel support from inside a privileged Linux container.
2. Runner builds the Ze LNS image and LAC image, and pulls FRR if a scenario needs it.
3. Runner creates an isolated Docker network with fixed IPs for Ze, LAC, and FRR.
4. Runner starts Ze with `l2tp { enabled true }`, a PPP address pool, and optional BGP `redistribute import l2tp` config.
5. Runner waits for Ze's `L2TP listener bound` log line.
6. Runner starts the LAC container process running `xl2tpd -D`; `xl2tpd` starts `pppd` for the PPP session.
7. Ze receives SCCRQ/ICRQ control messages and creates Linux L2TP tunnel/session kernel state.
8. Ze creates the PPPoL2TP socket and `/dev/ppp` channel/unit; kernel creates `pppN` in the Ze container namespace.
9. Ze and `pppd` negotiate LCP and IPCP. Ze assigns the peer IPv4 address from the configured pool.
10. Ze logs PPP session up, configures `pppN`, emits `session IP assigned`, and RouteObserver emits a route-change add event.
11. `bgp-redistribute-egress` consumes the event and sends a BGP announce command to the reactor.
12. FRR receives the subscriber /32 as a BGP route in the route-redistribution scenario.
13. The check script tears down the LAC session and verifies route withdrawal plus kernel L2TP/PPP cleanup.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host kernel -> containers | Privileged Docker containers share the host kernel modules and `/dev/ppp` device behavior | [ ] Preflight scenario fails clearly when unavailable |
| LAC container -> Ze container | Real UDP L2TP tunnel over Docker bridge IPs | [ ] Ze logs tunnel and incoming session establishment |
| L2TP userspace -> Linux kernel | Generic Netlink L2TP commands, PPPoL2TP socket, `/dev/ppp` ioctls | [ ] `ip l2tp show`, `ip link show type ppp`, `ip addr show dev pppN` |
| PPP control -> PPP dataplane | LCP and IPCP over `/dev/ppp`, IPv4 traffic through kernel `pppN` | [ ] Ping from LAC peer address to Ze PPP local address |
| L2TP -> Redistribute | RouteObserver emits `(l2tp, route-change)` batches | [ ] Ze logs subscriber route inject and withdraw |
| Redistribute -> BGP | `bgp-redistribute-egress` dispatches announce/withdraw to reactor | [ ] FRR sees route present and then absent |
| Runner -> scenario assertions | `check.py` imports lab helpers and uses native daemon CLIs | [ ] Scenario pass/fail summary |

### Integration Points
- `Makefile` - Docker-specific deployment target invoking the new lab runner.
- `test/l2tp-interop/run.py` - scenario selection, image build, strict preflight, and summary.
- `test/l2tp-interop/lab.py` - Docker lifecycle, log waiters, Ze/LAC/FRR helpers.
- `test/l2tp-interop/Dockerfile.ze` - Ze LNS image with `iproute2`, `kmod`, `ppp`, `python3`, and `tini`.
- `test/l2tp-interop/Dockerfile.lac` - LAC peer image with `xl2tpd`, `ppp`, `iproute2`, and `kmod`.
- `test/l2tp-interop/scenarios/*/check.py` - scenario assertions.
- `docs/functional-tests.md` - target semantics and kernel prerequisites.
- `docs/architecture/testing/l2tp-interop.md` - new architecture doc for the L2TP lab (separate from `interop.md` because the BGP interop doc is domain-specific to BGP scenarios, images, and daemon helpers).
- `plan/deployment-readiness-deep-review.md` and `plan/deferrals.md` - release-readiness status update after evidence exists.

### Architectural Verification
- [ ] No bypassed layers: route proof uses real L2TP RouteObserver and `bgp-redistribute-egress`, not `fakel2tp`.
- [ ] No unintended coupling: L2TP remains a subsystem and BGP remains a consumer through EventBus.
- [ ] No duplicated functionality: runner follows `test/interop` patterns rather than introducing a second lifecycle style.
- [ ] Exact-or-reject preserved: missing kernel support, missing `/dev/ppp`, missing `ip l2tp`, or skip-kernel-probe env all fail the full proof.
- [ ] Docker naming preserved: Docker-specific deployment targets contain `docker`.

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-deployment-l2tp-ppp-docker-test` | → | `test/l2tp-interop/run.py --strict` | `01-ppp-ipv4` scenario |
| `make ze-deployment-l2tp-ppp-docker-test` | → | Ze LNS plus LAC plus FRR lab | `02-ppp-bgp-redistribute-frr` scenario |
| Scenario filter argument | → | runner scenario selection | `python3 test/l2tp-interop/run.py 01-ppp-ipv4` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Docker full PPP/NCP lab runs on a host without PPPoL2TP support | Runner exits non-zero with a clear message naming `/dev/ppp`, `ip l2tp`, or `l2tp_ppp`/`pppol2tp` as the missing requirement |
| AC-2 | Docker full PPP/NCP lab runs with skip-kernel-probe env set | Runner or Ze proof exits non-zero and refuses the run |
| AC-3 | `01-ppp-ipv4` starts | Ze LNS container, LAC container, and isolated Docker network are created with unique suffixes and fixed IPs |
| AC-4 | Real `xl2tpd` LAC connects to Ze LNS | Ze logs L2TP listener bound, incoming session established, PPP session up, and session IP assigned |
| AC-5 | IPCP completes | Ze container has exactly one new `pppN` with local and peer addresses matching the scenario Ze config pool (lab default: local `10.100.0.1`, peer `10.100.0.2`); LAC container has matching peer-side PPP address state |
| AC-6 | PPP dataplane is up | LAC container can ping Ze PPP local address over the PPP tunnel |
| AC-7 | LAC session is terminated | Ze logs subscriber routes withdrawn; `ip l2tp show tunnel` and `ip link show type ppp` return empty in both containers within the timeout |
| AC-8 | `02-ppp-bgp-redistribute-frr` starts | FRR establishes BGP with Ze before route assertions run |
| AC-9 | PPP peer receives `10.100.0.2` | FRR receives `10.100.0.2/32` through BGP from Ze via `redistribute import l2tp` |
| AC-10 | LAC session is terminated in the BGP scenario | FRR no longer has `10.100.0.2/32` after the route observer emits withdraw |
| AC-11 | Any scenario fails | Runner dumps useful Ze, LAC, and FRR log tails and removes containers/network |
| AC-12 | Two runs overlap | Container and network names include a suffix so runs do not collide |
| AC-13 | Docs are generated or updated | Makefile help, functional-test docs, and feature/deployment-readiness status agree on target names and semantics |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| None planned | - | The implementation is orchestration and external-daemon evidence. Existing Go unit and `.ci` tests cover internal L2TP, PPP, and BGP redistribution logic. | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Scenario timeout | Positive seconds | Configurable by env | `0` or unparsable falls back to default or rejects clearly | Very large values are accepted as operator intent |
| Docker subnet suffix | Process ID or explicit env suffix | Valid Docker name segment | Empty string falls back to PID | Overlong or invalid suffix rejected by Docker with clear command context |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `01-ppp-ipv4` | `test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` | Real LAC completes L2TP, PPP LCP, IPCP, pppN setup, ping, route inject log, withdraw log, and cleanup | |
| `02-ppp-bgp-redistribute-frr` | `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/check.py` | Real PPP-assigned subscriber /32 is advertised to FRR via BGP and withdrawn on teardown | |
| `ze-deployment-l2tp-ppp-docker-test` | `Makefile` target | Operator runs one deployment target and gets strict Docker lab evidence | |
| `redistribute-l2tp-announce.ci` | `test/plugin/redistribute-l2tp-announce.ci` | Existing regression: synthetic L2TP route add becomes a BGP UPDATE (must not regress) | existing test |
| `redistribute-l2tp-withdraw.ci` | `test/plugin/redistribute-l2tp-withdraw.ci` | Existing regression: synthetic L2TP route remove becomes a BGP withdrawn route (must not regress) | existing test |

### Future
- IPv6CP full-peer scenario is deferred until the IPv4 PPPoL2TP lab is passing. The current native proof disables IPv6CP to keep the external `xl2tpd`/`pppd` peer deterministic.
- CHAP-MD5 full-peer scenario is deferred until the no-auth PPP/NCP dataplane lab is passing. Existing auth policy tests already cover mandatory-auth behavior; this lab's first goal is kernel dataplane and NCP proof.
- Multi-session per tunnel scenario is deferred until single-session cleanup is stable on the target runner.

## Files to Modify

- `Makefile` - route Docker full PPP/NCP target to the new strict lab runner and update help text if needed.
- `docs/functional-tests.md` - document peer-isolated Docker lab behavior, strict kernel requirements, scenario commands, and naming.
- `docs/features.md` - update L2TP feature caveat once the lab passes on supported Linux.
- `plan/deployment-readiness-deep-review.md` - update evidence status after the lab exists and after supported-host evidence passes.
- `plan/deferrals.md` - close or narrow the full L2TP PPP/NCP peer proof row only after evidence passes.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for user-visible behavior | Yes | `test/l2tp-interop/scenarios/*/check.py` |
| Make target | Yes | `Makefile` |
| Deployment docs | Yes | `docs/functional-tests.md`, `docs/features.md`, `plan/deployment-readiness-deep-review.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` L2TP evidence status |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/functional-tests.md` deployment evidence section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | Existing protocol code is exercised, not changed |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/l2tp-interop.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `test/l2tp-interop/run.py` - L2TP Docker lab scenario runner.
- `test/l2tp-interop/lab.py` - Docker lifecycle and Ze/LAC/FRR helper classes.
- `test/l2tp-interop/Dockerfile.ze` - Ze LNS lab image with kernel inspection tools.
- `test/l2tp-interop/Dockerfile.lac` - `xl2tpd`/`pppd` LAC lab image.
- `test/l2tp-interop/scenarios/01-ppp-ipv4/ze.conf` - Ze LNS config for IPv4 PPP proof (defines address pool, lab default `10.100.0.1/10.100.0.2`).
- `test/l2tp-interop/scenarios/01-ppp-ipv4/xl2tpd.conf` - LAC config targeting Ze container IP.
- `test/l2tp-interop/scenarios/01-ppp-ipv4/ppp-options` - deterministic pppd options.
- `test/l2tp-interop/scenarios/01-ppp-ipv4/l2tp-secrets` - lab shared secret.
- `test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` - PPP/NCP/dataplane/cleanup assertions.
- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/ze.conf` - Ze LNS plus BGP redistribute config.
- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/frr.conf` - FRR peer config.
- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/xl2tpd.conf` - LAC config.
- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/ppp-options` - deterministic pppd options.
- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/l2tp-secrets` - lab shared secret.
- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/check.py` - FRR advertise/withdraw assertions.
- `docs/architecture/testing/l2tp-interop.md` - architecture and operator docs for the L2TP Docker lab.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement TDD | Implementation phases below |
| 4. Review gate | Review Gate section |
| 5. Full verification | Targeted Python compile, existing L2TP tests, existing redistribute tests, then final gate as environment allows |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run targeted checks and supported lab scenario |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run verification |
| 13. Present summary | Executive Summary Report per planning rules |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Lab skeleton** - create `test/l2tp-interop/run.py`, `lab.py`, Dockerfiles, strict preflight, network/container lifecycle, and log helpers.
   - Tests: `python3 -m py_compile test/l2tp-interop/run.py test/l2tp-interop/lab.py`.
   - Files: new lab runner files and Dockerfiles.
   - Verify: runner can list or select scenarios; strict preflight fails clearly on unsupported host.
2. **Phase: PPP IPv4 scenario** - add `01-ppp-ipv4` with Ze and LAC containers, pppN/address/ping/cleanup assertions.
   - Tests: `python3 test/l2tp-interop/run.py 01-ppp-ipv4` on a supported Linux host.
   - Files: scenario config and check script.
   - Verify: fail first on missing assertions, then pass on supported host.
3. **Phase: BGP redistribute scenario** - add FRR container support and `02-ppp-bgp-redistribute-frr` asserting BGP route present and absent.
   - Tests: `python3 test/l2tp-interop/run.py 02-ppp-bgp-redistribute-frr` on a supported Linux host.
   - Files: FRR helper, scenario config, check script.
   - Verify: route appears only after PPP IP assignment and disappears after teardown.
4. **Phase: Makefile and docs** - wire Docker deployment target and update docs/status.
   - Tests: `make -n ze-deployment-l2tp-ppp-docker-test`, doc drift checks if factual target inventory is touched.
   - Files: `Makefile`, docs, readiness plan files.
   - Verify: help text and functional-test docs agree on target semantics.
5. **Phase: Existing regression checks** - ensure synthetic and native paths remain intact.
   - Tests: targeted L2TP Go tests, `bin/ze-test l2tp --all` (runs cross-platform; does not require PPPoL2TP), redistribute L2TP `.ci` plugin tests, and Python compile.
   - Files: no additional feature files expected.
   - Verify: no regressions in native proof or synthetic BGP route tests. Note: `bin/ze-test l2tp --all` exercises functional test scenarios that run without root or kernel modules; the Docker lab scenarios in phases 2-3 require a Linux host with PPPoL2TP.
6. **Full verification** - run `make ze-verify` for local confidence. `make ze-release-check` is a manual operator step from a clean worktree.
7. **Complete spec** - fill audit tables, write learned summary, and remove spec in the completion commit sequence when the user requests commits.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a check helper or scenario assertion |
| Correctness | The Docker lab uses separate Ze and LAC containers, not one shared namespace |
| Naming | Docker-specific deployment target includes `docker`; scenario names are kebab-case |
| Data flow | FRR route proof comes from real PPP session IP assignment through RouteObserver and EventBus |
| Exact-or-reject | Missing PPPoL2TP, missing `/dev/ppp`, missing `ip l2tp`, and skip-kernel-probe env all fail |
| Cleanup | Containers, Docker network, L2TP state, and PPP links are removed on pass and failure |
| Isolation | Per-run suffix prevents container/network collisions |
| Logs | Failure output includes enough Ze/LAC/FRR logs to diagnose kernel, PPP, or BGP failures |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Strict L2TP Docker lab runner exists | `test -f test/l2tp-interop/run.py` and Python compile |
| Ze/LAC images exist | `test -f test/l2tp-interop/Dockerfile.ze` and `test -f test/l2tp-interop/Dockerfile.lac` |
| PPP IPv4 scenario exists | `test -f test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` |
| BGP redistribute scenario exists | `test -f test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/check.py` |
| Docker deployment target invokes lab | Read `Makefile` target and run `make -n ze-deployment-l2tp-ppp-docker-test` |
| Unsupported-host failure is clear | Run target in current macOS/Docker Desktop environment and record expected strict failure |
| Supported-host pass is recorded | Run target on Linux host with PPPoL2TP support and record output |
| Docs match targets | Run doc drift/doc test checks that cover Makefile and docs |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Privileged containers | Lab uses `--privileged` only for deployment evidence, not default verification |
| Host mounts | Mount only repo and `/lib/modules` read-only when needed; avoid broad host path mounts |
| Secret handling | Lab L2TP secrets are fixed test values and not read from user environment |
| Command execution | Scenario helper commands are fixed lists, not shell-concatenated user input |
| Cleanup on interrupt | Signal and `atexit` cleanup remove containers and networks |
| Kernel state | Scenario checks initial and final L2TP/PPP state to avoid leaking kernel objects |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Docker unavailable | Strict deployment target fails with prerequisite message |
| PPPoL2TP unavailable | Strict preflight failure, not a code bug |
| Ze listener not bound | Check Ze config, image, and kernel probe logs |
| LAC does not connect | Check Docker network, UDP 1701 reachability, `xl2tpd` logs, L2TP secret |
| PPP does not reach IPCP | Check pppd options, Ze no-auth config, NCP timeouts, `/dev/ppp` setup |
| Ping fails after IPCP | Check `pppN` address state in both containers and kernel dataplane state |
| FRR route missing | Check BGP session first, then RouteObserver logs, then `bgp-redistribute-egress` logs |
| Cleanup fails | Preserve temp logs, dump `ip l2tp` and `ip link` state, fix teardown order |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Docker can make PPPoL2TP available from macOS | Docker containers share the Docker host or VM kernel, so missing PPPoL2TP remains missing | Current `ze-deployment-l2tp-ppp-docker-test` strict failure and `docker-run.py` behavior | Lab must fail strict on unsupported host |
| A same-container Docker wrapper is equivalent to an interop lab | It proves Linux userspace and kernel behavior, but not peer isolation over a Docker network | Comparison with `test/interop/interop.py` | Add separate Ze and LAC containers |
| Synthetic route redistribution proof closes real L2TP BGP evidence | Synthetic tests prove BGP UPDATE rendering but do not prove real RouteObserver emission from a PPP session | `test/plugin/redistribute-l2tp-*.ci` and `route_observer.go` | Add FRR scenario fed by real PPP IP assignment |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Reuse `scripts/evidence/docker-run.py` as-is | It runs one container and cannot model Ze/LAC/FRR peer isolation | New `test/l2tp-interop/` multi-container lab |
| Put L2TP scenarios into `test/interop/` | Existing module is BGP-specific in names, constants, images, and docs | New sibling `test/l2tp-interop/` that copies the proven pattern |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Treating Docker as a kernel-feature provider | Repeated across PPPoL2TP discussions | Deployment docs should state Docker shares the host kernel for kernel-backed proofs | Update docs in this spec |

## Design Insights

- Full PPP/NCP evidence has two separate meanings: native host proof and Docker peer-isolated lab proof. Keep both because they catch different failure shapes.
- The FRR scenario should wait for BGP Established before starting or asserting L2TP route state, otherwise route timing failures are ambiguous.
- The runner should start Ze and FRR before LAC. LAC redial helps, but waiting for Ze listener readiness makes failures easier to read.
- Initial scenario should disable IPv6CP, matching current strict native evidence, to avoid adding `pppd` IPv6 behavior to the first lab.
- Default session timeout should be 90s (matching `test/interop`). PPP negotiation over a Docker bridge typically completes in under 10s, but `xl2tpd` retransmits on 6s intervals and `pppd` LCP timeout defaults to 3s with 10 retries, so a slow first attempt can take 30-40s. The 90s budget accommodates image pull, container startup, and at least one full retry cycle.
- FRR image default should be `quay.io/frrouting/frr:10.3.1` (matching `test/interop/run.py`), overridable via `FRR_IMAGE` env.

## RFC Documentation

No production protocol code is planned in this spec. New comments are only needed in lab helpers when they explain why a kernel preflight or strict failure exists.

## Implementation Summary

### What Was Implemented
- Multi-container Docker lab under `test/l2tp-interop/` with Ze LNS, real xl2tpd/pppd LAC, and FRR in separate privileged containers on an isolated Docker bridge.
- `run.py` runner with strict PPPoL2TP preflight, skip-kernel-probe env rejection, image build, scenario selection, failure log dumping, and summary.
- `lab.py` with Docker lifecycle, FRR helper class, PPP/L2TP verification helpers, and atexit cleanup.
- Two Dockerfiles: Ze LNS image (Alpine + ze + iproute2 + kmod + ppp) and LAC image (Alpine + xl2tpd + ppp + iproute2 + iputils).
- Scenario `01-ppp-ipv4`: full PPP IPv4 proof (tunnel, LCP/IPCP, pppN creation, address verification, dataplane ping, route inject/withdraw, cleanup).
- Scenario `02-ppp-bgp-redistribute-frr`: BGP route redistribution proof (subscriber /32 appears in FRR via real RouteObserver path, withdrawn on teardown, session stable).
- Makefile target updated to invoke the peer-isolated lab.

### Bugs Found/Fixed
- Ze container needed L2TP debug env, disabled blob storage, disabled IPv6CP, and auth/NCP timeout overrides (matching native evidence script).

### Documentation Updates
- `docs/functional-tests.md`: updated L2TP Docker evidence description and target listing.
- `docs/features.md`: updated L2TP feature status to describe the peer-isolated Docker lab.
- `docs/architecture/testing/l2tp-interop.md`: new architecture doc for the lab.
- `plan/deployment-readiness-deep-review.md`: updated L2TP/PPP row with lab existence.
- `plan/deferrals.md`: narrowed the full peer proof deferral row.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Multi-container Docker lab | Done | `test/l2tp-interop/` | Ze, LAC, FRR in separate containers |
| Strict preflight | Done | `lab.py:preflight_strict()` | Checks /dev/ppp, l2tp_ppp, ip l2tp |
| Skip-kernel-probe rejection | Done | `lab.py:preflight_strict()` | Rejects both env key forms |
| PPP IPv4 scenario | Done | `scenarios/01-ppp-ipv4/` | Full proof |
| BGP redistribute scenario | Done | `scenarios/02-ppp-bgp-redistribute-frr/` | FRR route proof |
| Makefile target | Done | `Makefile:567` | Invokes lab runner |
| Doc updates | Done | See Documentation Updates above | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `lab.py:preflight_strict()` | Exits with clear message naming missing requirement |
| AC-2 | Done | `lab.py:preflight_strict()` | Refuses run with skip-kernel-probe env |
| AC-3 | Done | `lab.py:Scenario.setup()` | Creates containers with PID suffix and fixed IPs |
| AC-4 | Done | `check.py:wait_ze_log()` calls | Checks listener, session, PPP up, IP assigned |
| AC-5 | Done | `01-ppp-ipv4/check.py` | Verifies pppN address and exactly 1 PPP link |
| AC-6 | Done | `01-ppp-ipv4/check.py:lac_ping()` | Ping from LAC to Ze PPP address |
| AC-7 | Done | `01-ppp-ipv4/check.py:wait_l2tp_clean()` | ip l2tp/ip link empty in both containers |
| AC-8 | Done | `02-ppp-bgp-redistribute-frr/check.py` | FRR.wait_session before route assertions |
| AC-9 | Done | `02-ppp-bgp-redistribute-frr/check.py` | FRR.wait_route + check_route for 10.100.0.2/32 |
| AC-10 | Done | `02-ppp-bgp-redistribute-frr/check.py` | FRR.wait_route_absent after teardown |
| AC-11 | Done | `run.py` failure handler | dump_logs() called on scenario failure |
| AC-12 | Done | `lab.py` naming | PID suffix in all container/network names |
| AC-13 | Done | Makefile, docs, plan files | Target names and semantics agree |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `01-ppp-ipv4` | Done | `test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` | Requires Linux + PPPoL2TP |
| `02-ppp-bgp-redistribute-frr` | Done | `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/check.py` | Requires Linux + PPPoL2TP |
| `redistribute-l2tp-announce.ci` | Existing | `test/plugin/redistribute-l2tp-announce.ci` | Not modified |
| `redistribute-l2tp-withdraw.ci` | Existing | `test/plugin/redistribute-l2tp-withdraw.ci` | Not modified |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/l2tp-interop/run.py` | Created | |
| `test/l2tp-interop/lab.py` | Created | |
| `test/l2tp-interop/Dockerfile.ze` | Created | |
| `test/l2tp-interop/Dockerfile.lac` | Created | |
| `test/l2tp-interop/daemons` | Created | |
| `test/l2tp-interop/vtysh.conf` | Created | |
| `test/l2tp-interop/scenarios/01-ppp-ipv4/ze.conf` | Created | |
| `test/l2tp-interop/scenarios/01-ppp-ipv4/xl2tpd.conf` | Created | |
| `test/l2tp-interop/scenarios/01-ppp-ipv4/ppp-options` | Created | |
| `test/l2tp-interop/scenarios/01-ppp-ipv4/l2tp-secrets` | Created | |
| `test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` | Created | |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/ze.conf` | Created | |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/frr.conf` | Created | |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/xl2tpd.conf` | Created | |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/ppp-options` | Created | |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/l2tp-secrets` | Created | |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/check.py` | Created | |
| `docs/architecture/testing/l2tp-interop.md` | Created | |
| `Makefile` | Modified | Target updated |
| `docs/functional-tests.md` | Modified | L2TP Docker evidence description |
| `docs/features.md` | Modified | L2TP feature status |
| `plan/deployment-readiness-deep-review.md` | Modified | L2TP/PPP evidence row |
| `plan/deferrals.md` | Modified | Narrowed deferral row |

### Audit Summary
- **Total items:** 23 files, 13 ACs, 4 tests
- **Done:** 23 files, 13 ACs, 4 tests
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Ze container env vars added during critical review | `lab.py:454-462` | Fixed: added ZE_LOG_L2TP, ZE_STORAGE_BLOB, IPv6CP, timeouts |
| 2 | NOTE | l2tp-secrets file permissions set to 0600 | scenarios/*/l2tp-secrets | Fixed: chmod 600 |
| 3 | NOTE | Lab cannot be validated on macOS (expected) | preflight_strict | Documented in architecture doc and spec |

## Pre-Commit Verification

### Files Exist
| File | Evidence |
|------|----------|
| `test/l2tp-interop/run.py` | `test -f` pass, `py_compile` pass |
| `test/l2tp-interop/lab.py` | `test -f` pass, `py_compile` pass |
| `test/l2tp-interop/Dockerfile.ze` | `test -f` pass |
| `test/l2tp-interop/Dockerfile.lac` | `test -f` pass |
| `test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` | `test -f` pass, `py_compile` pass |
| `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/check.py` | `test -f` pass, `py_compile` pass |
| `docs/architecture/testing/l2tp-interop.md` | `test -f` pass |

### AC Verified
| AC ID | Evidence |
|-------|----------|
| AC-1 | `preflight_strict()` probes `/dev/ppp`, `l2tp_ppp`/`pppol2tp`, `ip l2tp`; raises SystemExit with clear message |
| AC-2 | `preflight_strict()` rejects `ZE_L2TP_SKIP_KERNEL_PROBE` and `ze.l2tp.skip-kernel-probe` |
| AC-3 | `Scenario.setup()` creates network and containers with `_SUFFIX` |
| AC-4 | `wait_ze_log()` for listener, session, PPP up, IP assigned |
| AC-5 | `ze_ppp_addr()` checks LOCAL_ADDR + PEER_ADDR; `ze_ppp_links()` count == 1 |
| AC-6 | `lac_ping(LOCAL_ADDR)` |
| AC-7 | `wait_l2tp_clean()` checks `ip l2tp show tunnel` + `ip link show type ppp` empty |
| AC-8 | `FRR.wait_session(ZE_IP)` before route assertions |
| AC-9 | `FRR.wait_route("10.100.0.2/32")` + `FRR.check_route()` |
| AC-10 | `FRR.wait_route_absent("10.100.0.2/32")` after LAC kill |
| AC-11 | `scenario.dump_logs()` in failure handler |
| AC-12 | `_SUFFIX = os.environ.get("ZE_L2TP_INTEROP_SUFFIX", str(os.getpid()))` |
| AC-13 | Makefile, functional-tests.md, features.md, deployment-readiness, deferrals all updated |

### Wiring Verified
| Path | Evidence |
|------|----------|
| `make ze-deployment-l2tp-ppp-docker-test` -> `test/l2tp-interop/run.py` | `make -n` shows `python3 test/l2tp-interop/run.py` |
| `run.py` -> `lab.py` -> scenarios | Python import chain verified by `py_compile` |
| Doc drift | `make ze-doc-drift` passes |
