# Spec: l2tp-remaining-proof-backlog

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Remaining L2TP and PPPoE interop/CLI proof obligations homed from closed spec-finish-l2tp so zero validation work is dropped. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Own the four remaining validation items carried over when `spec-finish-l2tp` closed (2026-11-23) after completing AC-2 (`l2tp-many-sessions-kl2tpd`) and AC-3 (`ppp-code-reject-xl2tpd` FF 03 proof for `RFC1661-6.6-1` and `RFC1661-6.6-2`).

Items homed here (zero items dropped; all other legacy `spec-finish-l2tp` items were verified either already completed on `main` or owned by `plan/release/fixes/spec-lifecycle-invariants.md`):

1. **Native netns L2TP-PPP release-proof run (L44):** Execute and record a release-proof run of `./le test deployment l2tp-ppp` (native Linux netns xl2tpd/pppd LAC + FRR lab) on a host/guest with `/dev/ppp` and `CONFIG_PPPOL2TP`. (Note: the Docker lab `docker-l2tp-ppp` has already run and passed on Linux 7.3; this item tracks the native netns target specifically.)
2. **accel-ppp LCP-Opened + MTU `.ci` (L162):** Add a functional/interop test exercising Ze LNS LCP-Opened and MTU negotiation against an `accel-ppp` L2TP LAC peer (requires `/dev/ppp` + root + `accel-ppp`). Note from the 2026-11-15 peer-source read in `spec-finish-l2tp`: `accel-ppp` LAC mode authenticates and assigns in PPP without completing full IPCP against Ze, so re-read the pinned `accel-ppp` source before writing assertions.
3. **offline-show-tunnels `.ci` (L194):** Add a `.ci` functional test exercising the `ze l2tp show tunnels` SSH client credentials round-trip (including `--user`/`-u` and `forwardToDaemon` in `internal/component/l2tp/cli/show.go`, complementing `test/plugin/l2tp-tunnels-show.ci` which queries the daemon through an external TLS plugin).
4. **PPPoE-at-scale interop (from `spec-finish-l2tp` scale item):** Add a many-sessions PPPoE interop scenario driving `pppd` 2.5.5's `pppoe` plugin (one `pppd` per session on its own macvlan, reusing the maintained LAC image), verifying distinct addresses, routes, traffic, and targeted teardown. Read `pppd/plugins/pppoe/discovery.c` at pppd 2.5.5 before designing assertions.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md`
- [ ] `docs/labs/l2tp-interop.md` - existing L2TP Docker lab scenarios and peers
- [ ] `docs/architecture/testing/interop.md` - interop suite registration and kernel capabilities

### Source files
- [ ] `internal/le/test/deployment/actions.go` - `l2tp-ppp` native deployment target
- [ ] `internal/component/l2tp/cli/show.go` and `internal/component/l2tp/cli/register.go` - `ze l2tp show` SSH credential path
- [ ] `test/plugin/l2tp-tunnels-show.ci` - existing plugin-path tunnel display test

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time)
- [ ] `internal/component/l2tp/cli/show.go` - forwards `show l2tp` over SSH using `sshclient.LoadCredentialsWithFlags`
- [ ] `internal/le/test/deployment/actions.go` - defines native `l2tp-ppp` deployment proof

**Behavior to preserve:**
- All existing L2TP/PPPoE interop scenarios, deployment proofs, and CLI tests.

**Behavior to change:**
- Add only the missing proof runs and test scenarios enumerated in Task.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Native L2TP deployment proof | `internal/le/test/deployment/actions.go` (`l2tp-ppp`) | Runs xl2tpd/pppd LAC + FRR against Ze LNS in Linux netns | Extends: records release-proof evidence on Ze's kernel (L44) |
| L2TP and PPPoE Docker interop labs | `internal/le/interoplab/l2tp/`, `internal/le/interoplab/pppoe/`, `test/interop-l2tp/` | Registers Docker scenarios and maintained `pppd` 2.5.5 LAC image | Extends: adds `accel-ppp` LCP/MTU (L162) and `pppd` macvlan PPPoE-at-scale scenarios |
| Functional `.ci` test runner | `internal/test/runner/`, `test/l2tp/`, `test/plugin/` | Runs daemon + CLI `.ci` fixtures | Extends: adds `ze l2tp show tunnels` SSH-credentials round-trip `.ci` (L194) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Native Linux netns lab (`./le test deployment l2tp-ppp`), Docker interop selectors, and `ze l2tp show tunnels` CLI over SSH.

### Transformation Path
1. Fill at design time per selected item.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LAC / PPPoE client -> Ze | L2TP/PPPoE + PPP over kernel sockets | No |
| `ze l2tp show` -> daemon | SSH client credentials + RPC dispatch | No |

### Integration Points
- `internal/le/test/deployment/`
- `internal/le/interoplab/l2tp/` and `internal/le/interoplab/pppoe/`
- `test/l2tp/` or `test/plugin/`

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each homed item remains needed when picked up for design | Carried over from closed `spec-finish-l2tp` (2026-11-23) | Re-verify against `main` at design time and retire items already superseded | source check at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `accel-ppp` LAC mode behaviour differs from expectations | Peer stops before IPCP or lacks expected MTU option | Read pinned `accel-ppp` source before writing L162 assertions |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Native `l2tp-ppp` deployment run | → | Ze LNS + kernel PPPoL2TP + FRR | (fill at design time) |
| `ze l2tp show tunnels` SSH client | → | `cmdShow` + `forwardToDaemon` | (fill at design time) |
| Multi-session PPPoE clients | → | PPPoE server + PPP driver + pool + observer | (fill at design time) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Native Linux netns `./le test deployment l2tp-ppp` on Ze kernel (L44) | Full PPP/NCP/FRR proof completes green with recorded output | (define at design time) |
| AC-2 | `accel-ppp` L2TP LAC scenario (L162) | LCP reaches Opened with negotiated MTU verified against peer | (define at design time) |
| AC-3 | `ze l2tp show tunnels` SSH-credentials `.ci` (L194) | Authenticated CLI round-trip returns live tunnel state and supports `--user`/`-u` | (define at design time) |
| AC-4 | Multi-session PPPoE scale scenario | Multiple `pppd` PPPoE clients on distinct macvlans establish, pass traffic, and tear down independently | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per selected item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (define at design time) | `test/l2tp/`, `test/interop-l2tp/`, `test/interop-pppoe/` | L44, L162, L194, PPPoE-at-scale | |

## Files to Modify
- (define at design time)

## Files to Create
- (define at design time)

## Notes
- Created 2026-11-23 when closing `spec-finish-l2tp` so its unselected open validation items keep a visible, countable spec home (`plan/README.md`, "Deferred work remains a visible spec").
- Legacy `spec-finish-l2tp` items NOT moved here because they are already done or owned elsewhere:
  - L41 (`addAddrP2PErr`): done in `TestIPCPBackendAddressFailureFailsSession` (`internal/component/l2tp/ppp/ncp_test.go`).
  - L42 (renegotiation after Opened): owned by `plan/release/fixes/spec-lifecycle-invariants.md`.
  - L43 (IPCP DNS Configure-Reject absorb): done in `TestIPCPDNSIsOptional` (`internal/component/l2tp/ppp/ncp_test.go`) and `TestIPCPDNSRequestWithoutServer` (`internal/component/l2tp/ppp/ncp_dns_reject_test.go`).
  - L163 (LCP restart counter / IRC-ZRC + AckRcvd): done by closed `spec-lcp-restart-counters` (`f4bb002b35`, `internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go`).
  - L164 (`docs/architecture/core-design.md` `ppp` row): done (`docs/architecture/core-design.md`, section 19 component-imports table).
  - `RFC2661-4.1-1` AVP reserved bits 2-5 on receive: done (`internal/component/l2tp/avp.go`, `rfc2661_avp_test.go`, `rfc/discrimination/rfc2661/RFC2661-4.1-1.json`).
  - Fencing `test/l2tp/rfc2661-emitted-control-shape.ci`: done in commit `89af0cc45e`.
