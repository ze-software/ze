# Spec: home-cpe-17-pppoe-client-keepalive-config

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P3 |
| Release reason | Feature: an operator migrating a CPE carries its LCP echo and redial settings; Ze fixes them at 10 s by 3 failures and a 1 to 60 s backoff. Nothing misbehaves with the defaults. |
| RFC gaps | - (RFC 1661 Section 5.8 makes Echo-Request a tool, not a schedule; no interval is required) |
| Depends | - (touches `keepaliveLoop`, which `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md` also changes; land after 12 to avoid a merge on the same loop) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make the PPPoE client's dead-peer detection and redial hold-off configurable
on each `pppoe-client` entry: the LCP Echo-Request interval, the number of
unanswered echoes that ends the session, and the minimum and maximum reconnect
delay. Defaults stay what Ze does today, so an existing configuration behaves
the same.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - keepalive and session-loss sections
  → Constraint: review finding "Echo failure fired at four losses, not three" (`>` against `>=`); the configured count keeps that meaning: the session ends on the Nth unanswered echo
- [ ] `ai/patterns/config-option.md` and `ai/rules/config.md` - YANG leaf with native range, units and default
  → Constraint: every leaf carries maximum native validation; the default is declared once in YANG and applied by the parser, never by each consumer (journal class `declared-default-applied-by-each-consumer`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - restart timer row says the Restart timer "MUST be configurable"; that concerns Configure-Request retransmission, not echo (Known Limitations)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoeclient/session.go` l.18-26 - constants `echoInterval` 10 s, `echoMaxFailures` 3, also `lcpNegotiationTimeout`, `authTimeout`, `ncpTimeout` 30 s, `papRetryInterval` 3 s, `papRequestsMax` 5. `sessionConfig` carries MTU, username, password, channel fd only
- [ ] `internal/component/l2tp/pppoeclient/network_phase.go` `keepaliveLoop` (l.94) - ticker at `echoInterval`; each tick sends an Echo-Request and increments the failure count, ends the session when the count reaches `echoMaxFailures`; any Echo-Reply resets it to zero
- [ ] `internal/component/iface/pppoe_client.go` - `reconnectBaseDelay` 1 s, `reconnectMaxDelay` 60 s; `run` waits `ReconnectDelay(attempts)` after each failed session; `ReconnectDelay` doubles from the base, shift capped at 6, clamped at the max (1, 2, 4, 8, 16, 32, 60 s). `attempts` resets to zero only when a session reaches Up. `PPPoEClientConfig` has no timing fields. `pppoeClientConfigChanged` restarts the client when any listed field changes. No caller outside the file uses `ReconnectDelay`
- [ ] `internal/component/iface/config.go` - `pppoeClientEntry`, `parsePPPoEClientEntry` (l.977) read leaves from the config map; `route-priority` and `mtu` show the parse pattern with defaults applied at parse
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` l.1708 - `list pppoe-client` with `source-interface`, `authentication`, `service-name`, `ac-name`, `no-default-route`, `route-priority`, plus `interface-common`
- [ ] LNS side for comparison: `internal/component/l2tp/ppp/start_session.go` `EchoFailures`, `session_run.go` `defaultEchoInterval` 10 s; accel-ppp exposes `lcp-echo-interval` and `lcp-echo-failure` in `[ppp]` (seen in `test/interop-pppoe/scenarios/pppoe-chap-rechallenge-ze-client/accel-ppp.conf`)

**Behavior to preserve:**
- With no new leaf set: 10 s interval, session lost on the 3rd unanswered echo, backoff 1, 2, 4, 8, 16, 32, 60, 60 s
- `attempts` resets on a session that reached Up

**Behavior to change:**
- The four values come from config, per client

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config leaves under `interface pppoe-client <name>`: container `keepalive` with `interval` and `failures`; container `reconnect` with `delay-min` and `delay-max`

### Transformation Path
1. YANG validates range and units; defaults declared in YANG
2. `parsePPPoEClientEntry` reads them into `pppoeClientEntry`; a cross-leaf check refuses `delay-max` below `delay-min`
3. `reconcilePPPoEClients` copies them to `PPPoEClientConfig`
4. `Dial` passes interval and failures through `sessionConfig` to `keepaliveLoop`
5. `run` computes the backoff from the client's own min and max

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to iface | YANG tree map parse | No |
| iface to pppoeclient | `PPPoEClientConfig` fields read by `Dial` | No |

### Integration Points
- `PPPoEClientConfig` (shared struct between iface and the dialer)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | values flow config to parse to client |
| No unintended coupling | Yes | existing struct gains fields |
| No duplicated functionality | Yes | constants removed, not kept beside the leaves |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | N-A | |
| Registration over hardcoding, inbound | Yes | `pppoeClientConfigChanged` is the reload comparison; its handling of the new fields is decided below |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Changing a timer does not need to drop the running session | an operator tuning detection does not expect a WAN outage | a restart is acceptable after all | owner review of this spec | unvalidated |
| A-2 | An interval of 1 s is the useful floor | the ticker period; accel-ppp accepts seconds | a sub-second need appears | none needed before a request | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A default applied in two places drifts | `TestPPPoEClientKeepaliveDefaults` fails | default declared in YANG, parse applies it once, constants deleted |
| R-2 | A very small interval with failures 1 flaps on a lossy line | operator reports | documented in the leaf description |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a WAN that is declared dead too early or too late, or redials too fast |
| How is it reverted? | single commit revert; configs that set the leaves then fail validation |
| Who else touches this path? | child 12 (`keepaliveLoop`), child 4 (reconnect drives IKE), `spec-lcp-restart-counters.md` (LNS timers) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `keepalive interval 2; failures 2` in config | → | parse, reconcile, `Dial`, `keepaliveLoop` | `TestPPPoEClientKeepaliveFromConfig` (fake dialer asserts the received values) |
| `reconnect delay-min 5; delay-max 20` | → | `run` backoff | `TestPPPoEClientReconnectDelayFromConfig` |
| config file through `ze config validate` | → | YANG and parse | `test/parse/pppoe-client-keepalive.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No new leaves | interval 10 s, failures 3, delays 1 to 60 s, identical to today |
| AC-2 | `keepalive interval 2; failures 2`, peer stops answering echoes | session ends after the 2nd unanswered echo, about 4 s |
| AC-3 | `reconnect delay-min 5; delay-max 20`, sessions keep failing | waits 5, 10, 20, 20 s |
| AC-4 | `delay-max` below `delay-min` | `ze config validate` refuses with a message naming both leaves and the fix |
| AC-5 | interval 0 or above 3600; failures 0 or above 255; delay out of range | refused by YANG range with the range in the message |
| AC-6 | Reload changing only keepalive or reconnect leaves | running session stays up; new values used from the next session and the next backoff |
| AC-7 | Reload changing any other leaf | client restarts as today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets faster dead-peer detection from a migrated configuration | config to keepalive loop | `TestPPPoEClientKeepaliveFromConfig`, `test/parse/pppoe-client-keepalive.ci` |
| 2 | Sets a longer redial hold-off so a down access network is not hammered | config to `run` | `TestPPPoEClientReconnectDelayFromConfig` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPoEClientKeepaliveDefaults` | `internal/component/iface/config_test.go` | AC-1 | |
| `TestPPPoEClientKeepaliveFromConfig` | `internal/component/iface/pppoe_client_test.go` | wiring | |
| `TestPPPoEClientReconnectDelayFromConfig` | same | AC-3 | |
| `TestPPPoEClientReloadKeepsSessionOnTimerChange` | same | AC-6, AC-7 | |
| `TestKeepaliveEndsAtConfiguredFailures` | `internal/component/l2tp/pppoeclient/session_test.go` | AC-2 with an injected short interval | |
| `TestPPPoEClientDelayMaxBelowMin` | `internal/component/iface/config_test.go` | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| keepalive interval (s) | 1-3600 | 1, 3600 | 0 | 3601 |
| keepalive failures | 1-255 | 1, 255 | 0 | 256 |
| reconnect delay-min (s) | 1-3600 | 1, 3600 | 0 | 3601 |
| reconnect delay-max (s) | 1-3600 | 1, 3600 | 0 | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-client-keepalive` | `test/parse/pppoe-client-keepalive.ci` | valid leaves accepted, each boundary refused | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | config only; the echo exchange itself is unchanged and already runs against accel-ppp | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `keepalive` and `reconnect` containers on `pppoe-client`
- `internal/component/iface/config.go` - entry fields, parse, cross-leaf check
- `internal/component/iface/pppoe_client.go` - config fields, backoff from config, reload comparison excludes timer leaves and updates the stored values; constants removed
- `internal/component/l2tp/pppoeclient/session.go`, `network_phase.go`, `dialer.go` - echo values through `sessionConfig`; constants removed
- `docs/architecture/l2tp/cpe-1-pppoe-client.md`, the interfaces guide PPPoE section, `docs/guide/configuration.md` if it lists pppoe-client leaves

## Files to Create
- `test/parse/pppoe-client-keepalive.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-iface-conf.yang` |
| YANG validation constraints | Yes | `range`, `units seconds`, `default` |
| YANG custom validators | Yes | `delay-max` not below `delay-min`; a `must` if the YANG engine evaluates it, else the parse check with a message |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | Yes | automatic for typed leaves |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | per-client config, not environment |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` PPPoE client entry |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, interfaces guide |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | interfaces guide |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors | Yes | derived at implementation |
| 17 | Existing examples for this area? | Yes | VyOS mapping guide (child 10) maps the VyOS keepalive and holdoff settings to these leaves |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaves, parse into the entry, copy to `PPPoEClientConfig`; consumers still use constants
   - Tests: `TestPPPoEClientKeepaliveFromConfig`
   - Files: YANG, `config.go`, `pppoe_client.go`
   - Verify: test fails because the dialer and `run` ignore the values
2. **Phase: keepalive** -- `sessionConfig` carries interval and failures; constants deleted; AC-2
3. **Phase: reconnect** -- backoff from config; AC-3; reload behaviour AC-6, AC-7
4. **Phase: validation** -- cross-leaf check, `.ci`, boundaries; AC-4, AC-5

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | session ends on the Nth unanswered echo, not N+1 |
| Naming | leaf names kebab-case, units in YANG |
| Rule: no-layering | the four constants are deleted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Leaves present | `grep -n keepalive internal/component/iface/yang/ze-iface-conf.yang` |
| Constants gone | `grep -n echoMaxFailures internal/component/l2tp/pppoeclient/` empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | ranges bound every value; no zero interval reaches a ticker |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

The reconnect counter already resets only on a session that reached Up, so a
configured maximum bounds the redial rate of a dead access network without any
new state.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| none | - | grep of `plan/journal/` for pppoe-client echo or reconnect timing finds no row |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Timer leaves do not restart a running session on reload | restart like every other leaf | a timing change is not worth a WAN outage; the next session and next backoff pick it up |
| No way to switch echo off | interval 0 disables, as accel-ppp's `lcp-echo-interval=0` does | a zero that means "off" is a value doing a second job (`ai/rules/principles.md`); a home CPE needs detection; an explicit `disable` leaf can come if asked |
| Keep the doubling backoff, configure its bounds | a fixed hold-off | bounds cover the migration need; the doubling shape is already tested |
| Container names `keepalive` and `reconnect` | flat `echo-interval` leaves | groups related leaves; names checked against `ai/rules/config.md` at implementation |

## Known Limitations
- The client's Configure-Request restart timer (3 s for IPCP, the LCP negotiator's own) and the 30 s phase timeouts stay fixed. RFC 1661 says the Restart timer MUST be configurable; `plan/release/fixes/spec-lcp-restart-counters.md` owns that obligation for the LNS side and does not name the client. The main thread checks whether it should cover the client and adds a journal row if not.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] Targeted tests pass; `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
