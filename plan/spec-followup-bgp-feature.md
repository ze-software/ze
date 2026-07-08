# Spec: followup-bgp-feature

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | implement (per-item, smallest-first) |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

BGP protocol/policy/observability follow-ups across the GR, filter, and Prometheus plugins.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **GR advanced (L81,L86)** - Selection Deferral Timer (L81, F-bit+crash-recovery already done); VPN ATTR_SET (RFC 6368) + hard-reset N-bit (RFC 8538) (L86, RFC 4724/9494 already shipped).
- **Raw-mode default-originate-filter (L119)** - `raw=true` filters bound to `default-originate-filter` get empty hex (no wire bytes for the synthetic route); reject at validate time or pre-encode the synthetic UPDATE.
- **AS-Confederation OTC (L88)** - RFC 9234 Section 5; private-AS removal filter already shipped.
- **Decorators v2 (L89)** - reverse-DNS + community-name decorators (RPKI-status decorator already shipped).
- **Prometheus behavioral spy tests (L25)** - spy-registry increment assertions for RIB churn / config-reload / wire-bytes counters (names registered only).
- **GR plugin per-peer metric-label cleanup (L27)** - staleRoutes/timerExpired gauges never `DeleteLabelValues` on peer removal (`gr.go:47`).
- **Prometheus phase 6 (L84)** - process/runtime metrics, `bgp_as_path_loop_detected_total`, RPKI/ASPA metrics.

## Required Reading

### Source files / docs

- [ ] `internal/component/bgp/plugins/gr/gr.go`, `gr_state.go`, `gr_llgr.go`
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/bgp/reactor/reactor_metrics.go`, `peer_initial_sync.go` (default-originate at :709)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/bgp/plugins/role/otc.go` (OTC); decorator plugin
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/component/bgp/plugins/gr/gr.go`
- [ ] `internal/component/bgp/reactor/reactor_metrics.go`
- [ ] `internal/component/bgp/plugins/role/otc.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- BGP session events (GR), route export (filters/decorators), Prometheus scrape

### Transformation Path
1. A session/route/metric event occurs
2. The GR / filter / metrics plugin processes it
3. Observable state (routes, decorated attrs, metrics) reflects the result

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| session events -> GR plugin | FSM callbacks | [ ] |
| reactor -> Prometheus | metric registry | [ ] |

### Integration Points
- `internal/component/bgp/plugins/gr/`
- `internal/component/bgp/plugins/role/`
- `internal/component/bgp/reactor/reactor_metrics.go`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GR restart with selection-deferral configured | → | deferral timer governs best-path selection | (fill during design) |
| Peer removed (reactor `RemovePeer`) | → | reactor emits `OnPeerStateChange(Down, ReasonPeerRemoved)`; GR deletes per-peer `ze_gr_*` labels + skips retention | `reactor.TestDoRemovePeerReturnsRemovedIdentity`, `reactor.TestRemovePeerNilDispatcherNoPanic`, `gr.TestHandleEventStateRemoved_SkipsActivationAndDeletesLabels`, `gr.TestStateRemovedTombstonePreventsLaterActivation` |
| Config reload / UPDATE received / wire read (real event paths) | → | reactor increments `ze_config_reloads_total`, `ze_config_reload_errors_total{parse}`, `ze_peer_messages_received_total{update}`, session/flap counters, `ze_wire_bytes_received_total` | `reactor.TestReloadIncrementsConfigReloadCounter`, `reactor.TestReloadParseErrorIncrementsErrorCounter`, `reactor.TestPeerEventsIncrementChurnCounters`, `reactor.TestSessionReadIncrementsWireBytesCounter` |
| default-originate bound to a `raw=true` filter | → | `defaultOriginateFilterAccepts` fails closed (no default route) + logs an actionable warning; text filters unaffected | `reactor.TestDefaultOriginateRejectsRawFilter`, `reactor.TestDefaultOriginateAllowsNonRawFilter`, `reactor.TestDefaultOriginateRawGuardIgnoresMalformedRef` |
| Web renders a decorated leaf (peer IP via `ze:decorate "reverse-dns"`; well-known community) | → | `reverse-dns` decorator resolves IP→PTR host; `community-name` maps well-known community→RFC name; both registered in `service_web.go` | `web.TestReverseDNSDecorator`, `web.TestReverseDNSDecoratorGraceful`, `web.TestCommunityNameDecorator`, `config.TestYANGSchemaDecorateExtension` (asserts `connection.remote.ip` carries `reverse-dns`) |
| /metrics scrape / AS_PATH-loop reject / ASPA verify | → | telemetry exporter registers `go_*`+`process_*` collectors; loop filter increments `ze_bgp_as_path_loop_detected_total{peer}`; `buildDecisions` increments `ze_rpki_aspa_outcomes_total{result}` | `metrics.TestRegisterRuntimeCollectors`, `filter.TestASPathLoopMetricIncrements`, `rpki.TestBuildDecisionsRecordsASPAOutcomes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |
| AC-6 (item 6, DONE) | A peer with a live `ze_gr_timer_expired_total{peer}` series is removed from config | Reactor emits `SessionStateDown`/`ReasonPeerRemoved`; GR stops timers, drops caps, deletes `ze_gr_stale_routes{peer}` + `ze_gr_timer_expired_total{peer}`, and does not activate route retention; a racing teardown `down` cannot re-activate GR |
| AC-5 (item 5, DONE) | A successful/failed `Reload()`, a received UPDATE, a session flap, and a wire read run through the real reactor/peer/session producers | Spy registry observes exact increments: `ze_config_reloads_total`+1 (and +1 again on re-reload), `ze_config_reload_errors_total{error_type="parse"}`+1 with success counter unmoved, `ze_peer_messages_received_total{type="update"}`+3, `ze_peer_sessions_established_total`/`ze_peer_state_transitions_total`/`ze_peer_session_flaps_total`+1, `ze_wire_bytes_received_total` = OPEN+KEEPALIVE lengths |
| AC-2 (item 2, DONE) | A `default-originate-filter` references a filter the plugin declared `raw=true` | `defaultOriginateFilterAccepts` fails closed (returns false, default route not originated) and logs an actionable warning ("bind a text filter instead"); a `raw=false` filter is unaffected and proceeds to the normal dry-run; a malformed ref is left to the existing colon check |
| AC-4 (item 4, DONE) | Web UI renders a peer IP leaf (`ze:decorate "reverse-dns"`) or a well-known community value | `reverse-dns` annotates the IP with its PTR hostname (trailing dot stripped; empty on DNS failure/non-IP, no error); `community-name` annotates `65535:65281`→`no-export` (and other RFC well-knowns), empty for ordinary/unparseable values; both registered in `service_web.go` (reverse-dns gated on `resolvers.DNS`, community-name unconditional since it needs no resolver) |
| AC-7 (item 7, DONE) | /metrics is scraped; an AS_PATH loop is rejected; an ASPA-verified route is decided | The endpoint exposes `go_*`/`process_*` runtime metrics (telemetry builds); `ze_bgp_as_path_loop_detected_total{peer}` increments on the AS-loop reject only (not ORIGINATOR_ID/CLUSTER_LIST loops); `ze_rpki_aspa_outcomes_total{result=valid\|invalid\|unknown}` increments per ASPA-verified route and is skipped when ASPA is inactive. RPKI origin-validation (`ze_rpki_validation_outcomes_total`) was already metered; item 7 added runtime/process, AS-path-loop, and ASPA. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| gr-selection-deferral, decorator-v2, prom-phase6 (new) (`.ci`) | test/plugin | GR/policy/metrics behaviour end-to-end | |

## Files to Modify

- `internal/component/bgp/plugins/gr/gr.go` - see Task work items
- `internal/component/bgp/reactor/reactor_metrics.go` - see Task work items
- `internal/component/bgp/plugins/role/otc.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Per-Item Progress

| Item | Deferral | Status | Notes |
|------|----------|--------|-------|
| 6 GR per-peer metric-label cleanup | L27 | DONE (committed separately) | Reactor `RemovePeer` emits `SessionStateDown`/`rpc.ReasonPeerRemoved`; GR deletes per-peer series + tombstones. Tests: `gr_removal_test.go`, `peer_removed_test.go`. |
| 5 Prometheus behavioral spy tests | L25 | DONE (committed separately) | Added `counter`/`counterVec` spy accessors + `reactor_metrics_behavioral_test.go` (4 tests) driving real producers (`Reload`, `IncrUpdatesReceived`, `updatePeerStateMetric`, `readAndProcessMessage`) with exact-delta assertions. RED verified by temporarily neutering `session_read.go:129` (reverted). Test-only; no production change. |
| 2 Raw-mode default-originate-filter | L119 | DONE (committed separately) | Runtime fail-closed guard (matches existing malformed-ref style): `default_originate_raw.go` `defaultOriginateRejectsRawFilter` + `filterRawInfo` seam, wired into `defaultOriginateFilterAccepts`. A raw filter can't gate the synthetic default route (no wire bytes -> empty hex); reject + warn. Tests: 3 in `peer_initial_sync_test.go`. Chose reject-at-runtime over pre-encode: default-originate is a pure accept/reject gate, the text form already describes the fixed default route, and the codebase validates these refs at runtime not config-load. |
| 4 Decorators v2 | L89 | DONE (committed separately) | Two web display decorators (RPKI-status already shipped via bgp/plugins/rpki_decorator): `reverse-dns` (IP→PTR via `resolvers.DNS.ResolvePTR`) wired to `connection.remote.ip` YANG leaf; `community-name` (well-known community→RFC name, reuses `attribute.Community.String()`, no resolver). Both registered in `service_web.go`. Tests: `decorator_reverse_dns_test.go`, `decorator_community_test.go`, + schema wiring assertion in `yang_schema_test.go`. Community leaf wiring deferred (no community leaf exists in BGP YANG; decorator registered + available). |
| 3 AS-Confederation OTC | L88 | RE-DEFERRED (blocked on confederation-member support) | See "Item 3 re-deferral" below. Verified against code: ze is a single-AS speaker with no confed-id/member-AS config, so RFC 9234 §5 confederation rules are vacuously satisfied and the existing single-AS OTC egress is already §5-correct. Implementing true confederation OTC requires first building confederation-member support (large, separate feature). |
| 7 Prometheus phase 6 | L84 | DONE (3 sub-commits) | 7a: `RegisterRuntimeCollectors` on `PrometheusRegistry` (go_*/process_* via vendored collectors subpkg), called from telemetry exporter. 7b: `ze_bgp_as_path_loop_detected_total{peer}` in the loop filter (`SetMetricsRegistry` wired from reactor metrics-enable block, since the loop filter isn't a run plugin). 7c: `ze_rpki_aspa_outcomes_total{result}` in `buildDecisions` (RPKI origin validation already metered). Tests: `runtime_collectors_test.go`, `loop_metrics_test.go`, `aspa_metrics_test.go`. |
| 1 GR advanced (selection-deferral, VPN ATTR_SET, hard-reset) | L81,L86 | DEFERRED to `plan/spec-gr-advanced.md` | Split by RFC/subsystem (2026-07-08). Two genuine GR features (hard-reset RFC 8538, selection-deferral timer RFC 4724 §4.1) captured in the destination spec with verified Current Behavior. VPN ATTR_SET (RFC 6368) is an L3VPN feature mis-bundled here; deferred further to a future L3VPN spec (recorded in `spec-gr-advanced.md` "Known Limitations / Deferred"). ze is GR-Helper-only today; both in-scope features add negotiation/restarting-speaker behaviour that does not exist yet. |

## Item 3 re-deferral (AS-Confederation OTC, L88)

**Decision (2026-07-08, user):** re-defer unchanged; no code change.

**Verified evidence:**
- ze is a single-AS speaker: `role.getLocalASN()` (role.go:66) returns one `filterLocalASN`, set from config (role.go:158). There is **no** confederation-member configuration in ze (no confed-id, no member-AS list). The only confederation code is AS_PATH segment parsing (AS_CONFED_SEQUENCE/SET, RFC 5065/6793) at `attribute/aspath.go:33-34` and the skip in `reactor_wire.go:125` — wire-level pass-through, not local confederation identity.
- OTC egress stamps the single local AS (`otc.go:429`), and `checkOTCEgress` (`otc.go:201`) suppresses OTC-tagged routes to Provider/Peer/RS. Per RFC 9234 §5 ("On egress from the Internet-facing AS, the OTC Attribute MUST NOT contain a value other than the Internet-facing ASN"), this is already correct for a non-confederation speaker.
- RFC 9234 §5's confederation-specific rules (OTC value MUST equal the Confederation Identifier, not a Member-AS) bind only a speaker that **is** a confederation. With exactly one AS, they are vacuously satisfied. §5 also states OTC/Roles between confederation members is NOT RECOMMENDED.

**Why re-deferred, not implemented:** true confederation OTC requires first building AS-confederation-member support (confed-id + member-AS config, confederation-eBGP session semantics, AS_CONFED segment origination), a large feature far beyond "advanced OTC." Tracked for a future dedicated confederation spec.

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
- Being implemented item-by-item (smallest-first), each committed separately. Umbrella stays open until all 7 land or remaining items are re-deferred.
- Item 3 (L88) re-deferred (see above); items 6, 5, 2, 4, 7 done+committed.
- Item 1 (L81,L86) deferred 2026-07-08 to a dedicated destination spec `plan/spec-gr-advanced.md`
  (hard-reset RFC 8538 + selection-deferral timer RFC 4724 §4.1 in scope; VPN ATTR_SET RFC 6368
  split further to a future L3VPN spec). With this, all 7 umbrella items are done or deferred
  with a destination — the umbrella can be closed (two-commit closure) when the user chooses.
