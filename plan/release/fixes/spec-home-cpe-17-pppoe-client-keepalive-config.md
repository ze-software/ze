# Spec: home-cpe-17-pppoe-client-keepalive-config

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P3 |
| Release reason | Feature: an operator migrating a CPE carries its LCP echo and redial settings; Ze fixes them at 10 s by 3 failures and a 1 to 60 s backoff. Nothing misbehaves with the defaults. |
| RFC gaps | - |
| Depends | Implementation: `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md` (it changes `keepaliveLoop` too, and its scripted access concentrator is the harness of this spec's entry-point test). Closure: no scenario; the tests in this spec are its evidence |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

RFC gaps: none. RFC 1661 Section 5.8 makes Echo-Request a tool, not a schedule; no interval is required.

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
- [ ] `internal/component/l2tp/pppoeclient/session.go` - the package `const` block at the top of the file: `echoInterval` 10 s, `echoMaxFailures` 3, also `lcpNegotiationTimeout`, `authTimeout`, `ncpTimeout` 30 s, `papRetryInterval` 3 s, `papRequestsMax` 5. `sessionConfig` carries MTU, username, password, channel fd only
- [ ] `internal/component/l2tp/pppoeclient/network_phase.go` `keepaliveLoop` - ticker at `echoInterval`; each tick sends an Echo-Request and increments the failure count, ends the session when the count reaches `echoMaxFailures`; any Echo-Reply resets it to zero
- [ ] `internal/component/iface/pppoe_client.go` - `reconnectBaseDelay` 1 s, `reconnectMaxDelay` 60 s; `run` waits `ReconnectDelay(attempts)` after each failed session; `ReconnectDelay` doubles from the base, shift capped at 6, clamped at the max (1, 2, 4, 8, 16, 32, 60 s). `attempts` resets to zero only when a session reaches Up. `PPPoEClientConfig` has no timing fields. `pppoeClientConfigChanged` restarts the client when any listed field changes. No caller outside the file uses `ReconnectDelay`
- [ ] `internal/component/iface/config.go` - `pppoeClientEntry`, `parsePPPoEClientEntry` read leaves from the config map; `route-priority` and `mtu` show the parse pattern with defaults applied at parse
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list pppoe-client` under `container interface` with `source-interface`, `authentication`, `service-name`, `ac-name`, `no-default-route`, `route-priority`, plus `interface-common`
- [ ] LNS side for comparison: `internal/component/l2tp/ppp/start_session.go` `EchoFailures`, `session_run.go` `defaultEchoInterval` 10 s; accel-ppp exposes `lcp-echo-interval` and `lcp-echo-failure` in `[ppp]` (seen in `test/interop-pppoe/scenarios/pppoe-chap-rechallenge-ze-client/accel-ppp.conf`)

**Behavior to preserve:**
- With no new leaf set: 10 s interval, session lost on the 3rd unanswered echo, backoff 1, 2, 4, 8, 16, 32, 60, 60 s
- `attempts` resets on a session that reached Up

**Behavior to change:**
- The four values come from config, per client

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| Client echo constants | `internal/component/l2tp/pppoeclient/session.go` `echoInterval`, `echoMaxFailures`, `sessionConfig` | fixed 10 s interval and 3 failures; `sessionConfig` carries no timing | replaces: the constants are removed and the values travel in `sessionConfig` |
| Client keepalive loop | `internal/component/l2tp/pppoeclient/network_phase.go` `keepaliveLoop` | ticks at `echoInterval`, ends the session at `echoMaxFailures` unanswered echoes, resets on any Echo-Reply | extends: reads interval and failure count from the session config; the exchange is unchanged |
| Redial backoff | `internal/component/iface/pppoe_client.go` `reconnectBaseDelay`, `reconnectMaxDelay`, `ReconnectDelay` | doubles from 1 s, clamped at 60 s | extends: base and max come from config; the doubling and clamp stay |
| Reload comparison | `internal/component/iface/pppoe_client.go` `pppoeClientConfigChanged` | restarts the client when any listed field changes | extends: the timer leaves are excluded from the restart set and update the stored values |
| `pppoe-client` config parse | `internal/component/iface/config.go` `pppoeClientEntry`, `parsePPPoEClientEntry`; `ze-iface-conf.yang` `list pppoe-client` | reads leaves with defaults applied at parse, as `route-priority` and `mtu` do | extends: same parse pattern for the `keepalive` and `reconnect` leaves |
| LNS echo settings | `internal/component/l2tp/ppp/start_session.go` `EchoFailures`, `session_run.go` `defaultEchoInterval` | the server side already carries a configurable failure count and a 10 s default | does not apply: server side, kept as the model for names and defaults only |

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
| `keepalive interval 2; failures 2` in config | → | parse, reconcile, `Dial`, `keepaliveLoop` | `TestPPPoEClientKeepaliveFromConfig` (fake dialer asserts the received values; proves the hand-over only) |
| `reconnect delay-min 5; delay-max 20` | → | `run` backoff | `TestPPPoEClientReconnectDelayFromConfig` (proves the hand-over only) |
| config file through `ze config validate` | → | YANG and parse | `test/parse/pppoe-client-keepalive.ci` |
| a config tree applied through the iface plugin's config apply (the entry the daemon's transaction calls), the production dialer, a real kernel PPP unit | → | parse, `reconcilePPPoEClients`, `PPPoEClient.run`, `Dial`, `keepaliveLoop`, backoff, reload | `TestPPPoEClientKeepaliveConfigEndToEnd` (`internal/component/l2tp/pppoeclient/keepalive_config_integration_linux_test.go`, user namespace, scripted access concentrator on a veth pair) |

`TestPPPoEClientKeepaliveConfigEndToEnd` is the entry-point proof; the two
fake-dialer rows prove only that values are handed over. It reuses the
scripted access concentrator `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md`
builds for `TestDialRenegotiatesLCPInNetworkPhase`, moved into a shared test
helper package `internal/test/pppoeac` (beside `internal/test/userns`) so both
tests use one copy: this is why child 12 is an implementation prerequisite.
Its three subtests assert what the access concentrator observes, not timing:

| Subtest | Steps | Assertion |
|---------|-------|-----------|
| detection | apply `keepalive interval 1; failures 2`; session up; the AC stops answering Echo-Requests | the AC receives exactly 2 unanswered Echo-Requests on the session, then a PADT from the client; the client's state leaves `up` |
| backoff | apply `reconnect delay-min 1; delay-max 2`; the AC answers PADI with no PADO | the gaps between successive PADIs are at least 1 s, then at least 2 s, then at least 2 s (lower bounds only: the bound proves the configured delay was waited, an upper bound would measure the machine) |
| reload uptake | session up with `failures 2`; apply a reload changing only `failures 3` | the session id is unchanged (no restart, AC-6); after the AC drops the session and the client reconnects, the AC receives exactly 3 unanswered Echo-Requests before the PADT |

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
| `TestPPPoEClientKeepaliveConfigEndToEnd` | `internal/component/l2tp/pppoeclient/keepalive_config_integration_linux_test.go` | AC-2, AC-3, AC-6 from the config apply entry point through the real dialer (Wiring Test table) | |

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
- `docs/features/interfaces.md` (the design page `internal/component/iface` declares): read, and nothing there becomes wrong; it lists no `pppoe-client` leaves and no echo or redial timing, and its Capability Table `PPPoE` row is about PPPoE as an interface type, which keepalive timers do not change

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
| 1 | New user-facing feature? | Yes | `features/interfaces.md` (the declaration that carries the PPPoE client): a keepalive and reconnect paragraph, `test/parse/pppoe-client-keepalive.ci` in its Real-path tests, and a fresh Doc review (done 2026-11-07) |
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
| 17 | Existing examples for this area? | Yes | VyOS mapping guide (`docs/guide/migrating-from-vyos.md`, child 10) maps VyOS `holdoff` (pppd's fixed redial wait, default 30, range 0 to 86400) to equal `reconnect delay-min` and `delay-max`, and VyOS's fixed pppd `lcp-echo-interval 20` and `lcp-echo-failure 3` to `keepalive interval 20; failures 4`, because pppd declares the peer dead at the request after the third unanswered one (`pppd/lcp.c` `LcpSendEchoRequest`) while `keepaliveLoop` ends on the request that reaches `failures`. Read at vyos-1x `rolling` `d6902e6423ec` (done 2026-11-07) |

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
- The client's Configure-Request restart timer and the 30 s phase timeouts stay fixed. RFC 1661 Section 4.6 (`rfc/full/rfc1661.txt`): "The Restart timer MUST be configurable, but SHOULD default to three (3) seconds." The client's LCP negotiation ticks a fixed `time.NewTicker(3 * time.Second)` (`internal/component/l2tp/pppoeclient/session.go`) and its IPv6CP the `ipv6cpRestartInterval` constant (`ipv6cp_client.go`). That is an absent requirement of an implemented RFC, so it is owed work, not a limitation this spec can keep: the 2026-11-07 row of `plan/journal/rfc-extension-not-implemented.md` records it, and `plan/release/fixes/spec-lcp-restart-counters.md` owns it beside the LNS side it already covered.

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

## Implementation Summary

### What Was Implemented
- `db2ea98439`: YANG `keepalive { interval; failures }` and `reconnect { delay-min; delay-max }` on `list pppoe-client` (`internal/component/iface/yang/ze-iface-conf.yang`, defaults 10, 3, 1, 60); `PPPoEClientTiming`, `Validate`, `reconnectDelay`, `parsePPPoEClientTiming` (`internal/component/iface/pppoe_client_timing.go`); `parsePPPoEClientEntry` reads it (`config.go`); `PPPoEClient.run` waits `c.cfg.Timing.reconnectDelay`; `pppoeClientConfigChanged` compares every field but `Timing`, and `reconcilePPPoEClients` hands a timing-only change to `setTiming`; `Dial` validates again and carries the values through `sessionConfig` to `keepaliveLoop`, which ends on `echoFails >= session.cfg.keepaliveFailures`. The constants `echoInterval`, `echoMaxFailures`, `reconnectBaseDelay`, `reconnectMaxDelay` and `ReconnectDelay` are deleted. `Schema.LookupDeclared` lets the Linux-only list's defaults resolve on any host.
- `76fcc6d41d`: `test/parse/pppoe-client-keepalive.ci` promoted.
- `c65d67f899`: `features/interfaces.md` paragraph and Real-path test; VyOS mapping rows; the RFC 1661 Section 4.6 Restart-timer journal row owned by `spec-lcp-restart-counters`.

### Bugs Found/Fixed
- None in the product. One documentation inaccuracy, fixed at closure (Review Gate finding 1).

### Documentation Updates
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` (keepalive and reconnect paragraph, anchors `pppoe_client.go -- pppoeClientConfigChanged, setTiming, run`, `pppoe_client_timing.go -- PPPoEClientTiming.Validate, reconnectDelay`, `network_phase.go -- keepaliveLoop`), `docs/guide/pppoe.md` ("PPPoE client keepalive and redial"), `docs/guide/configuration.md` (PPPoE client section), `docs/architecture/config/yang-config-design.md` (`LookupDeclared`): `db2ea98439`.
- `features/interfaces.md`, `docs/guide/migrating-from-vyos.md`: `c65d67f899`.
- Closure: `docs/guide/pppoe.md` no longer equates pppd's `lcp-echo-failure` with `keepalive failures`; it states the off-by-one the VyOS guide already gives, with a `network_phase.go -- keepaliveLoop` anchor.
- `./le doc check all`: every failure is under `../gh-pages/` (the published site checkout, 386 rows each in `reference/cli/index.{md,html}`, 437 in `llms.txt`, per-command pages missing); none is under `docs/`, `features/` or this change.

### Deviations from Plan
- Test homes: the iface unit tests live in `internal/component/iface/pppoe_client_timing_test.go`, not `config_test.go`/`pppoe_client_test.go`; the end-to-end test is `internal/component/iface/pppoe_keepalive_integration_linux_test.go` (three top-level tests `TestPPPoEClientKeepaliveConfigEndToEnd{Detection,Backoff,ReloadUptake}`), not under `pppoeclient`, because it drives the iface config apply entry point.
- Backoff end to end uses `delay-min 3; delay-max 4` rather than 1 and 2: the defaults wait 1 s then 2 s, so 1 and 2 could not go red without the leaves; 3 and 4 do.
- Reload uptake keeps the running session's count (2) and lets the AC withhold replies on it, then checks the next session uses 3, rather than having the AC drop the session first; it proves the same AC-6 claim and also that the running session was not restarted (Echo id 200 answered on the same session).
- Validation of the ranges is done at parse and again at `Dial` (`PPPoEClientTiming.Validate`); YANG `range` is the first check.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The pppoe guide said the keepalive leaves "are the settings pppd calls `lcp-echo-interval` and `lcp-echo-failure`" | pppd's `lcp-echo-failure N` matches `failures N+1` (`docs/guide/migrating-from-vyos.md` row, `keepaliveLoop`) | closure review | guide corrected in commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Echo interval configurable per client | Done | `ze-iface-conf.yang` `container keepalive` `leaf interval`; `network_phase.go` `keepaliveLoop` `session.cfg.keepaliveInterval` | |
| Unanswered-echo count configurable | Done | `leaf failures`; `keepaliveLoop` `echoFails >= session.cfg.keepaliveFailures` | |
| Min and max reconnect delay configurable | Done | `container reconnect`; `pppoe_client_timing.go` `reconnectDelay`; `pppoe_client.go` `run` | |
| Defaults unchanged | Done | YANG `default 10`, `3`, `1`, `60` | `TestPPPoEClientKeepaliveDefaults` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPPPoEClientKeepaliveDefaults`; parse `.ci` accepts an entry with no block | |
| AC-2 | Done | `TestKeepaliveEndsAtConfiguredFailures`; `TestPPPoEClientKeepaliveConfigEndToEndDetection` (exactly 2 unanswered, then PADT) | |
| AC-3 | Done | `TestPPPoEClientReconnectDelayFromConfig` (5, 10, 20, 20 s); `TestPPPoEClientKeepaliveConfigEndToEndBackoff` (lower bounds 3 s, 4 s) | |
| AC-4 | Done | `TestPPPoEClientDelayMaxBelowMin`; `pppoe-client-keepalive.ci` | message names both leaves and the fix |
| AC-5 | Done | `TestPPPoEClientTimingBoundaries`; `pppoe-client-keepalive.ci` boundary steps | |
| AC-6 | Done | `TestPPPoEClientReloadKeepsSessionOnTimerChange`; `TestPPPoEClientKeepaliveConfigEndToEndReloadUptake` | |
| AC-7 | Done | `TestPPPoEClientReloadKeepsSessionOnTimerChange` (other-leaf case); `pppoeClientConfigChanged` compares the whole struct but `Timing` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPPPoEClientKeepaliveDefaults` | Done | `internal/component/iface/pppoe_client_timing_test.go` | file moved |
| `TestPPPoEClientKeepaliveFromConfig` | Done | same | |
| `TestPPPoEClientReconnectDelayFromConfig` | Done | same | synctest |
| `TestPPPoEClientReloadKeepsSessionOnTimerChange` | Done | same | |
| `TestKeepaliveEndsAtConfiguredFailures` | Done | `internal/component/l2tp/pppoeclient/keepalive_config_test.go` | |
| `TestPPPoEClientDelayMaxBelowMin` | Done | `pppoe_client_timing_test.go` | |
| `TestPPPoEClientKeepaliveConfigEndToEnd` | Done | `internal/component/iface/pppoe_keepalive_integration_linux_test.go` | three top-level tests |
| `pppoe-client-keepalive` | Done | `test/parse/pppoe-client-keepalive.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `ze-iface-conf.yang` | Done | |
| `config.go` | Done | parse calls `parsePPPoEClientTiming` |
| `pppoe_client.go` | Done | plus new `pppoe_client_timing.go` |
| `session.go`, `network_phase.go`, `dialer.go` | Done | plus `connected_session.go` zero refusal |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/guide/pppoe.md`, `docs/guide/configuration.md` | Done | |
| `docs/features/interfaces.md` | Changed | read: nothing became wrong; the declaration `features/interfaces.md` carries the paragraph |
| `test/parse/pppoe-client-keepalive.ci` | Done | |

### Audit Summary
- **Total items:** 23
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Dead-peer detection follows the configured interval and failure count | functional, real kernel PPP unit | `TestPPPoEClientKeepaliveConfigEndToEndDetection` PASS in QEMU (`tmp/session/2026-11-07-104251/scratch/rwinv/out-qemu/clean-iface.log`); red with the check fixed at 3 (`c17-failures-fixed`, log `out-qemu/c17-failures-fixed.log`, "echo timeout failures=3") |
| Redial hold-off follows delay-min and delay-max | functional, real kernel PPP unit | `TestPPPoEClientKeepaliveConfigEndToEndBackoff` PASS; red with `delay := t.ReconnectDelayMin / 4` (`out-qemu/c17-backoff-short.log`, delay=750ms) |
| A timing-only reload keeps the session and the next session uses the new values | functional | `TestPPPoEClientKeepaliveConfigEndToEndReloadUptake` PASS; red with the `Timing` exclusion removed from `pppoeClientConfigChanged` (`out-qemu/c17-reload-restarts.log`) |
| Config is validated, defaults unchanged | functional `.ci` | `pppoe-client-keepalive` PASS (`out-gb3/c17-clean.log`); red at seq 11 with the delay-max check disabled (`out-gb3/c17-broken.log`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Configurable client LCP Restart timer (RFC 1661 Section 4.6) | separate obligation, not echo | `plan/release/fixes/spec-lcp-restart-counters.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-17-pppoe-client-keepalive-config-7e292f27-ced0-427f-a7b6-3a931db211f9.md` (15 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring, removed-behavior, concurrency on `cfg.Timing`, input validation, docs against `keepaliveLoop`, RFC proof records |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The guide equated pppd's `lcp-echo-failure` with `keepalive failures`, so an operator copying `lcp-echo-failure 3` detects one interval earlier than before | `docs/guide/pppoe.md`, "PPPoE client keepalive and redial" | states the off-by-one, `failures 4` for pppd's 3, as `migrating-from-vyos.md` does |

### NOTEs
- `reconcilePPPoEClients` reads `client.cfg` unlocked, but it is the only writer (`setTiming`), and the run goroutine reads `cfg.Timing` under `c.mu` (`run`, `runSession`), so there is no race; the scoped `-race` run is clean.
- `run` stops the client with an ERROR naming the fix if `reconnectDelay` refuses the timing; unreachable through config because the parse validates first.
- `./le rfc check`: no discrimination line for rfc1661 or rfc2516 is stale. `RFC2516-7-3` carries a STALE audit verdict and `RFC2516-5.5-3` a SHIFTED one, both from `a02374d9ad` (LCP restart counter) changing `rfc2516_host_termination_test.go`, which this spec's commits never touched.
- `./le test weakened check base db2ea98439~1`: none of its 44 findings is in a file this spec changed. `./le repo check`: all passed.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/parse/pppoe-client-keepalive.ci` | yes | committed in `76fcc6d41d`; `pppoe-client-keepalive` PASS in `out-gb3/c17-clean.log` |
| `internal/component/iface/pppoe_keepalive_integration_linux_test.go` | yes | gopls and Read at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-7 | unit tests pass | `./le job run label unit-pkg quiet command go test -race -tags <feature tags> -run 'PPPoEClient\|Keepalive\|Timing\|ReconnectDelay\|RFC2516' ./internal/component/iface/ ./internal/component/l2tp/pppoeclient/`: `ok iface 2.729s`, `ok pppoeclient 1.436s` (`tmp/session/2026-11-07-104251/scratch/job-unit-pkg-afb9bd06.log`) |
| constants gone | no fixed timer remains | `grep -rn 'echoMaxFailures\|reconnectBaseDelay\|reconnectMaxDelay\|ReconnectDelay(' internal/component/l2tp/pppoeclient/ internal/component/iface/` (non-test): no hit |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| config through `ze config validate` | `test/parse/pppoe-client-keepalive.ci` | PASS and red at seq 11 under the break (`out-gb3/`) |
| config apply through `startPPPoEConfig` to the production dialer | `pppoe_keepalive_integration_linux_test.go` | three tests PASS, each red under its break (`out-qemu/`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | a timer-only reload keeps the session: `TestPPPoEClientKeepaliveConfigEndToEndReloadUptake` (Echo id 200 answered on the same session after reload), red when `Timing` restarts the client |
| A-2 | confirmed | accel-ppp `lcp-echo-interval` and pppd's take whole seconds (spec row 17, VyOS template at `d6902e6423ec`); YANG `uint16` `range "1..3600"` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| cpe-1 keepalive and reconnect paragraph | `keepaliveLoop`, `PPPoEClientTiming.Validate`, `reconnectDelay`, `setTiming`, `runSession` read at closure | yes |
| guide pppoe and features/interfaces paragraphs | YANG defaults and ranges at `ze-iface-conf.yang` 1876-1941 | yes, guide counting corrected |
| `./le doc check all` | failures only under `../gh-pages/` | none in this change |
