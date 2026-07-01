# Spec: AS112 BGP Integration (Conditional Origination)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-as112-1-iface-address-registry, spec-as112-2-dns-server |
| Phase | - |
| Updated | 2026-07-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-as112-0-umbrella.md` - RFC compliance mapping, cross-cutting decisions
4. `plan/spec-as112-2-dns-server.md` - the DNS server this spec announces routes for
5. `internal/component/bgp/plugins/watchdog/`, `internal/component/bgp/plugins/healthcheck/` - the conditional-announcement mechanism this spec wires, unmodified

## Task

Wire the AS112 DNS service (spec-as112-2) into BGP using existing,
already-tested mechanisms only: a `healthcheck` probe that queries the as112
DNS server, a `watchdog`-gated `update` block per target peer-group carrying
the operator's chosen community and an optional AS112-origin override, and a
worked, validated configuration example. This spec adds **no new BGP plugin
code** — its deliverable is a proven, tested configuration pattern plus the
end-to-end test connecting spec-as112-2's DNS correctness to actual wire-level
BGP behavior (announce only when healthy, correct community, correct
AS_PATH origin).

**Three correctness requirements the design review pinned down — the worked
example and its tests MUST honor all three:**
- **H1/M4 — the healthcheck probe queries an *anycast service address*, not
  loopback**, using child 2's `ze … as112 health` command (`dig` is absent on the
  appliance and `ze resolve dns` cannot target a server). A loopback probe would
  report UP even when the anycast address is unreachable, defeating RFC 7534
  §3.3/§3.5. (If the operator sets child 2's `allow-from` access list, the probe
  is unaffected: on-box/loopback sources are always permitted, so the health
  query is never dropped — see spec-as112-2.)
- **H2 — the `update` block's `watchdog` block MUST include the `withdraw`
  marker** so the route starts withdrawn. Verified: absence of `withdraw`
  defaults the route to *announced* (`watchdog/config.go:145,292`; the YANG has
  no real `default "true"`), which would announce before the DNS is healthy. An
  advisory doctor check flags an AS112 watchdog-gated `update` block missing
  `withdraw`.
- **H3 — the announced NLRI are the four covering /24,/48 prefixes**
  (192.175.48.0/24, 2620:4f:8000::/48, 192.31.196.0/24, 2001:4:112::/48), NOT the
  /32,/128 host addresses bound on `lo`. The worked example spells them out.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md`
  → Constraint: confirms BGP origination correctly stays out of the as112 plugin (spec-as112-2) — it's ordinary BGP config, not as112-specific code. This spec's "implementation" is documentation + tests, not a new plugin.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7534.md` - §3.4 (routing software requirements: announce the service prefixes, restrict to a prefix+AS_PATH filter), §4.2 (withdraw before planned downtime)
  → Constraint: the worked example must demonstrate the prefix-list/AS-PATH-filter recommendation (umbrella RFC Compliance Mapping item #8) via a dedicated peer-group.
- [ ] `rfc/short/rfc1997.md` - NO_EXPORT (0xFFFFFF01) / NO_ADVERTISE (0xFFFFFF02) / NO_EXPORT_SUBCONFED (0xFFFFFF03)
  → Constraint: community names already parse via `internal/core/bgp/attribute/text.go:49-58`; this spec only needs to prove the configured community survives onto the wire for the AS112 routes specifically.
- [ ] `rfc/short/rfc3765.md` - NOPEER (0xFFFFFF04)
  → Constraint: RFC 7534's own recommended community for routes sent to bilateral peers — the worked example should show NOPEER as one of the available choices, not just NO_EXPORT.

**Key insights:**
- Every BGP mechanism this spec needs already exists and is independently
  tested: `update`-block communities, `asn.local`/`local-options replace-as`,
  and `healthcheck`→`watchdog`. This spec's value is proving the *composition*
  works end-to-end for this specific use case, and documenting it so an
  operator doesn't have to rediscover the pattern.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `update` block (lines 200-266): `community` leaf-list (line 237), `watchdog` container (lines 260-264, `name`+`withdraw`); `asn` container (lines 445-468): `local` + `local-options [no-prepend, replace-as]`.
- [ ] `internal/component/bgp/plugins/watchdog/config.go` - `collectWatchdogRoutes()` (lines 127-172) parses `update.watchdog{name;withdraw}` into per-peer route pools keyed by watchdog-group name; the same group name used across multiple peer-groups' `update` blocks shares one announce/withdraw state.
- [ ] `internal/component/bgp/plugins/watchdog/pool.go` - per-peer announced/withdrawn state tracking (lines 15-307).
- [ ] `internal/component/bgp/plugins/watchdog/server.go` - `request bgp watchdog announce|withdraw <group> [peer]` command surface (lines 44-82).
- [ ] `internal/component/bgp/plugins/healthcheck/fsm.go` - INIT→RISING/FALLING→UP/DOWN FSM, `rise`/`fall` thresholds (lines 5-92).
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` - `dispatchStateAction()` (lines 279-301): UP → watchdog announce; DOWN → withdraw (if `withdraw-on-down=true`) or re-announce with `down-metric`.
- [ ] `internal/component/bgp/plugins/healthcheck/probe.go` - shell-command probe execution, `/bin/sh -c`, timeout, process-group isolation (lines 17-51).
- [ ] `internal/component/bgp/plugins/healthcheck/config.go` - probe schema (lines 14-43): `command`, `group`, `interval`, `rise`, `fall`, `withdraw-on-down`.
- [ ] `test/plugin/healthcheck-announce.ci`, `test/plugin/healthcheck-withdraw.ci` - existing functional tests this spec's new test is modeled on.
- [ ] `internal/component/bgp/config/peers.go` - `ExportFilters` concatenation (lines 155-156); confirms group-scoped `update` blocks are announced only within that group's peer subtree, which is the mechanism "send to chosen peer-group(s)" relies on.

**Behavior to preserve:**
- watchdog and healthcheck plugins are completely unmodified — this spec is a
  new configuration pattern and test, not a code change to either.

**Behavior to change:**
- None to existing BGP plugin code. New: `docs/guide/as112.md`'s worked
  example (owned jointly with the umbrella) and new `.ci`/interop tests.

## Data Flow (MANDATORY)

### Entry Point
- Operator config: a dedicated BGP peer-group with an `update` block whose NLRI
  are the four covering /24,/48 prefixes (H3) + chosen `community` +
  `watchdog{name; withdraw}` (the `withdraw` marker is mandatory — H2),
  optionally `asn.local 112 local-options [replace-as]`; a `healthcheck` probe
  (`group` matching the watchdog name) querying the as112 service via child 2's
  `ze … as112 health` command against an anycast address (H1/M4).

### Transformation Path
1. Healthcheck probe runs `ze … as112 health` (child 2, finding M4), which issues
   a real authoritative query against an **anycast service address** (not
   loopback — H1) for a known in-zone name, on the configured `interval`. The
   route stays withdrawn until this succeeds because the `update` block carries
   the `withdraw` marker (H2).
2. FSM reaches UP after `rise` consecutive successes → `dispatchStateAction`
   sends `request bgp watchdog announce <group>`.
3. Watchdog announces the group-scoped `update`-block routes (carrying the
   configured community and any AS_PATH origin override) to every peer in
   every peer-group that references that watchdog group name.
4. If the probe later fails `fall` consecutive times → FSM DOWN →
   `withdraw-on-down=true` → watchdog withdraws the routes from all those
   peers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| healthcheck probe ↔ as112 DNS server (spec-as112-2) | `ze … as112 health` → authoritative query to an **anycast service address** (H1/M4), existing healthcheck probe mechanism | [ ] |
| healthcheck ↔ watchdog | existing `DispatchCommandArgs`, unmodified | [ ] |
| watchdog ↔ BGP peers | existing wire UPDATE/WITHDRAW, unmodified | [ ] |
| operator config ↔ this spec's worked example | `docs/guide/as112.md` source-anchored example | [ ] |

### Integration Points
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` `dispatchStateAction()` - probe definition routes through here, unmodified
- `internal/component/bgp/plugins/watchdog/config.go` `collectWatchdogRoutes()` - `update` block routes through here, unmodified

### Architectural Verification
- [ ] No bypassed layers (uses the documented, existing config surface end-to-end)
- [ ] No unintended coupling (this spec adds zero new Go packages)
- [ ] No duplicated functionality (zero new conditional-announcement code — confirmed by Current Behavior)
- [ ] Registration over hardcoding — N/A, no new registrable component in this child

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A single watchdog group name can be referenced by `update` blocks under multiple distinct peer-groups simultaneously, and all of them share the one announce/withdraw state | `watchdog/config.go:127-172`'s per-peer (not per-group) pool keying, read from the route's watchdog-group name across all peers it's configured on | "Send to chosen peer-group(s)" would require N independent watchdog groups instead of one shared group | functional test configuring 2 peer-groups referencing the same watchdog group name and confirming both announce/withdraw together |
| A-2 | **REVISED (finding H1) — the probe deliberately targets an anycast service address, NOT loopback.** A loopback probe would report UP while the anycast address is unreachable (freebind lets the server bind before the address lands, spec-as112-2 B2), which is exactly the false-positive RFC 7534 §3.3/§3.5 forbids. Child 2 supplies `ze … as112 health` (M4) so the worked example does not need `dig` or a hand-written anycast query | RFC 7534 §3.3/§3.5; spec-as112-2 B2 (freebind) means loopback-up ≠ anycast-reachable | (was: "loopback is fine") — a loopback probe silently defeats items #6/#7 | interop test asserts the route is withheld while the anycast address is unreachable even though the process is up | design resolves it |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Worked example reuses a general-purpose/transit peer-group instead of a dedicated one, defeating RFC 7534 §3.4's prefix/AS-PATH-filter SHOULD (umbrella item #8) | doc review catches a non-dedicated example | `docs/guide/as112.md` example explicitly uses a peer-group named for AS112 only, with a callout explaining why |
| R-2 | A shallow healthcheck probe (TCP connect / port-open only, or a query to **loopback** instead of the anycast address) gives false UP while the anycast path is down or the DNS content is wrong (umbrella R-2, finding H1) | functional test asserts the probe command queries an **anycast service address** with real content (via `ze … as112 health`), and the interop test withholds the route while the anycast address is unreachable | worked-example probe is `ze … as112 health` targeting an anycast address, not a port-open check and not loopback |
| R-3 | `asn.local 112 replace-as` is set on a peer-group that also carries non-AS112 routes, unintentionally re-originating unrelated prefixes as AS112 — **or** set on a *publicly-peered* group, silently making this an uncoordinated global AS112 node (finding M5) | interop test scoped only to the AS112 watchdog-gated routes | Primary: `docs/guide/as112.md` callout — dedicated-group recommendation (as R-1) plus an explicit hard warning that `replace-as 112` on a public group requires RFC 7534 §3.2/§5 coordination. Optional advisory doctor check `doctor-as112-global-origin-uncoordinated` (placement per Key Design Decisions) |
| R-4 | The worked `update` block omits the `watchdog` `withdraw` marker, so the AS112 route is announced at startup before the DNS is healthy (finding H2 — absence defaults to *announced*, `watchdog/config.go:145,292`) | `as112-healthcheck-announce.ci` asserts the route is absent before the probe reaches UP | Primary: worked example includes `withdraw`; the functional test fails if it is omitted. Optional advisory doctor check `doctor-as112-watchdog-missing-withdraw` (placement per Key Design Decisions) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Healthcheck probe configured against a running as112 service, watchdog-gated `update` block on a peer-group | → | `healthcheck` FSM → `watchdog` announce | `test/plugin/as112-healthcheck-announce.ci` |
| as112 service stopped/unhealthy | → | `healthcheck` FSM → `watchdog` withdraw | `test/plugin/as112-healthcheck-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|--------------------|
| AC-1 | as112 DNS server unhealthy/not yet started; `update` block includes the `watchdog{ withdraw }` marker (H2) | AS112 routes are not announced to any peer in the targeted group(s) — at startup, before the first probe success. (Note H2: without the `withdraw` marker the route would default to *announced*; the worked example must include it) |
| AC-2 | Healthcheck probe reaches UP (`rise` consecutive successes) | Watchdog announces the AS112 routes to every peer in every targeted group |
| AC-3 | Healthcheck probe reaches DOWN (`fall` consecutive failures) after having been UP | Watchdog withdraws the AS112 routes from every peer in every targeted group, within one probe interval |
| AC-4 | `community [ no-export ]` configured in the `update` block | The announced route carries NO_EXPORT, observable via `show bgp` / the receiving peer's RIB |
| AC-5 | `community [ nopeer ]` configured instead | The announced route carries NOPEER instead of NO_EXPORT |
| AC-6 | `asn.local 112; local-options [ replace-as ];` set on the target peer-group | AS_PATH origin for routes to that group's peers is 112, not ze's real local AS |
| AC-7 | A second peer-group has no `asn.local` override but references the same watchdog group | That group's peers see ze's real local AS in AS_PATH, while the AS112-origin group (AC-6) sees 112 — both controlled independently |
| AC-8 | Two distinct peer-groups both reference the same watchdog group name in their `update` blocks | Both groups announce/withdraw together, confirming "send to chosen peer-group(s)" via a single shared watchdog group (A-1) |
| AC-9 | as112 process is running and answers on **loopback**, but the anycast address is NOT reachable/answering | The probe (querying the anycast address via `ze … as112 health`) stays DOWN, so the route is NOT announced — proving the probe validates the advertised path, not just process liveness (finding H1). Proof: functional test, no new code |
| AC-10 | The worked-example `update` block is authored WITHOUT the `watchdog{ withdraw }` marker | `as112-healthcheck-announce.ci` catches the route being announced before the first probe success (finding H2). **Advisory (optional):** a doctor check `doctor-as112-watchdog-missing-withdraw` — see Key Design Decisions for the self-containment placement question that must be resolved before it is built |
| AC-11 | `asn.local 112 replace-as` is set on a group with eBGP sessions to non-private ASNs | Primary proof: `docs/guide/as112.md` hard warning + the interop test scoping origin-112 to the intended group only (finding M5). **Advisory (optional):** a doctor check `doctor-as112-global-origin-uncoordinated`, subject to the same placement question |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|--------------------------|
| 1 | configures the full worked example from `docs/guide/as112.md` (dedicated peer-group, healthcheck probe, watchdog-gated update block) | healthcheck → watchdog → BGP wire | `test/plugin/as112-healthcheck-announce.ci` |
| 2 | the as112 DNS server goes down mid-session | healthcheck DOWN → watchdog withdraw | `test/plugin/as112-healthcheck-withdraw.ci` |
| 3 | wants AS112-origin routes to IX peers but local-ASN routes to internal peers | two peer-groups, one with `asn.local 112 replace-as`, one without, both referencing the same watchdog group | `test/interop/scenarios/NN-as112-origin-as.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none — this child adds no new Go code, only config/test artifacts) | n/a | covered by functional + interop tests below | |

### Boundary Tests
N/A — no new numeric config surface introduced by this child (reuses existing
`healthcheck`/`watchdog`/`update`-block leaves, already boundary-tested where
those plugins were originally specified).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|---------------------|--------|
| `as112-healthcheck-announce` | `test/plugin/as112-healthcheck-announce.ci` | AS112 routes announce only once the DNS server is confirmed healthy; route is withheld at startup with the `withdraw` marker present (AC-1, AC-2, H2) | |
| `as112-healthcheck-withdraw` | `test/plugin/as112-healthcheck-withdraw.ci` | AS112 routes withdraw when the DNS server becomes unhealthy (AC-3) | |
| `as112-probe-anycast-not-loopback` | `test/plugin/as112-probe-anycast-not-loopback.ci` | process up on loopback but anycast address unreachable → probe stays DOWN → route not announced (AC-9, H1) | |
| `as112-community-choice` | `test/plugin/as112-community-choice.ci` | configured community (no-export vs nopeer) appears correctly on the announced route (AC-4, AC-5) | |
| `as112-shared-watchdog-group` | `test/plugin/as112-shared-watchdog-group.ci` | two peer-groups referencing one watchdog group announce/withdraw together (AC-8) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|-----------------|--------|
| `NN-as112-origin-as` | `test/interop/scenarios/` | FRR | A real external BGP speaker observes AS_PATH origin 112 on the AS112-origin-overridden peer-group and ze's real local AS on the non-overridden group, confirming AC-6/AC-7 are wire-correct, not just internally consistent | |
| `NN-as112-community-wire` | `test/interop/scenarios/` | FRR | A real external BGP speaker observes the configured well-known community (NO_EXPORT/NOPEER) on the wire, confirming AC-4/AC-5 | |

## Files to Modify
- `docs/guide/as112.md` - worked configuration example (dedicated peer-group, healthcheck probe using `ze … as112 health` against an anycast address [H1], watchdog-gated update block **with the `withdraw` marker** [H2], the four covering **/24,/48** prefixes as NLRI [H3], community/origin-AS choices), cross-referencing the umbrella's RFC Compliance Mapping; MUST include the hard M5 warning about `replace-as 112` on public groups
- `docs/features.md` - cross-link to the BGP-side example

## Files to Create
- `test/plugin/as112-healthcheck-announce.ci`
- `test/plugin/as112-healthcheck-withdraw.ci`
- `test/plugin/as112-probe-anycast-not-loopback.ci` (finding H1)
- `test/plugin/as112-community-choice.ci`
- `test/plugin/as112-shared-watchdog-group.ci`
- `test/interop/scenarios/NN-as112-origin-as/` (config + expectations, numbered per existing interop scenario convention)
- `test/interop/scenarios/NN-as112-community-wire/`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No — reuses existing `update`/`watchdog`/`healthcheck`/`asn` YANG entirely | n/a |
| YANG validation constraints | [ ] No — no new leaves | n/a |
| YANG custom validators | [ ] No | n/a |
| CLI commands/flags | [ ] No — reuses existing `request bgp watchdog`/`show bgp healthcheck` | n/a |
| CLI grammar | [ ] No | n/a |
| Editor autocomplete | [ ] No | n/a |
| Functional test for new RPC/API | [x] Yes — new `.ci` tests listed above exercise existing RPCs in this new composition | see Files to Create |
| Pipe completeness | [ ] No — no new command output | n/a |
| Env var registration | [ ] No | n/a |
| Doctor check for runtime dependencies | [ ] Optional (advisory only) — up to three advisory checks: shared-group (R-1), missing-`withdraw` (H2/R-4), global-origin-uncoordinated (M5/R-3). Placement is an open question (see Key Design Decisions): they must not hardcode AS112 knowledge into BGP nor require the as112 plugin to read BGP config. If a clean home is found, register codes in `internal/core/diagnostic/codes.go`; otherwise ship tests + docs and record the deferral | TBD during implementation; advisory, not blocking — primary enforcement is tests + docs |
| Prometheus counters/metrics | [ ] No — reuses existing `ze_watchdog_*` and healthcheck metrics, no new metric names | n/a |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|------------------|
| 1 | New user-facing feature? | [x] Yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] No — no new YANG, only a new documented composition of existing config | n/a |
| 3 | CLI command added/changed? | [ ] No | n/a |
| 4 | API/RPC added/changed? | [ ] No | n/a |
| 5 | Plugin added/changed? | [ ] No | n/a |
| 6 | Has a user guide page? | [x] Yes | `docs/guide/as112.md` |
| 7 | Wire format changed? | [ ] No | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] No | n/a |
| 9 | RFC behavior implemented? | [x] Yes | `rfc/short/rfc7534.md` §3.4/§4.2 already exist, no edit needed |
| 10 | Test infrastructure changed? | [ ] No — uses existing `.ci`/interop frameworks | n/a |
| 11 | Affects daemon comparison? | [ ] No | n/a |
| 12 | Internal architecture changed? | [ ] No | n/a |
| 13 | Route metadata keys added/changed? | [ ] No | n/a |
| 14 | Prometheus counters added/changed? | [ ] No | n/a |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] No | n/a |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] To verify — grep `docs/guide/healthcheck.md` for anchors before editing it indirectly via this example | n/a unless grep hits |
| 17 | Existing docs show config/CLI/API examples for this area? | [x] Yes | `docs/guide/healthcheck.md` (ExaBGP migration guide) — the AS112 example should cross-reference it, not duplicate its FSM explanation |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review / fix / re-verify | Critical Review Checklist below |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — draft the worked config example and a minimal failing functional test asserting the route is NOT announced before the DNS server is healthy.
   - Tests: `test/plugin/as112-healthcheck-announce.ci` (failing stage)
   - Files: `docs/guide/as112.md` (draft), `test/plugin/as112-healthcheck-announce.ci`
   - Verify: test fails because as112 (spec-as112-2) + the example aren't wired together yet
2. **Phase: Announce/withdraw correctness** — complete the announce and withdraw functional tests.
   - Tests: `as112-healthcheck-announce.ci`, `as112-healthcheck-withdraw.ci`
   - Files: same as above
   - Verify: tests fail → fix example/config → pass
3. **Phase: Community + origin-AS** — functional tests for community choice and the shared-watchdog-group story.
   - Tests: `as112-community-choice.ci`, `as112-shared-watchdog-group.ci`
   - Verify: tests fail → fix → pass
4. **Phase: Interop** — FRR-backed interop scenarios proving wire-correctness of AC-4/5/6/7.
   - Tests: `NN-as112-origin-as`, `NN-as112-community-wire`
   - Verify: scenario tests fail → fix → pass
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit tables, learned summary, two-commit close

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-11 each demonstrated by a named test (AC-9 anycast-probe; AC-10/AC-11 advisory checks are optional, proven by test+doc) |
| Correctness | Withdraw happens within one probe interval of failure, not delayed by an unrelated cycle |
| Naming | `.ci`/interop scenario names follow existing conventions in `test/plugin/`, `test/interop/scenarios/` |
| Data flow | No new Go code introduced EXCEPT the optional advisory doctor checks (if a clean home was found); confirm via `git diff --stat` that only `docs/`, `test/`, and (at most) the advisory doctor-check file changed |
| Rule: no-layering | Worked example never has the as112 plugin itself touch BGP config |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `docs/guide/as112.md` worked example present | `ls -la docs/guide/as112.md` + grep for the example config block |
| All new `.ci`/interop tests pass | `make ze-functional-test` filtered to `as112-*`; interop scenario runner for `NN-as112-*` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|--------------------|
| Input validation | N/A — no new operator-input surface in this child |
| Route leak | The dedicated-peer-group recommendation (R-1) is the primary mitigation against the AS112 routes leaking onto unrelated sessions; confirmed by the interop test only seeing AS112 prefixes on the targeted session |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | N/A — no Go code in this child |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read `healthcheck`/`watchdog` source from Current Behavior |
| Functional/interop test fails | Check AC; if AC wrong → escalate to umbrella; if AC correct → fix the worked example/config |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- **Probe target is the crux (H1).** The value of the watchdog→healthcheck gate
  depends entirely on WHAT the probe queries. A loopback probe reports UP while
  the anycast address is down (freebind, spec-as112-2 B2, lets the server bind
  before the address lands), which is precisely the false-positive RFC 7534
  §3.3/§3.5 forbids. The probe therefore queries an anycast service address via
  child 2's `ze … as112 health`.
- **`withdraw` marker is load-bearing (H2).** `watchdog/config.go:145,292` makes
  a route start *announced* when the `withdraw` field is absent (the YANG has no
  real `default "true"`, only a misleading description). The worked example must
  include `withdraw`, and `as112-healthcheck-announce.ci` fails if it is missing.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|---------------------------|-----------|
| Zero new BGP plugin code — pure composition of `update`/`community`/`asn.local`/`watchdog`/`healthcheck` (advisory doctor checks are the sole exception; see next row) | A new as112-specific BGP config subtree that the as112 plugin compiles into update blocks | Rejected at the umbrella DESIGN gate: requires new cross-component config synthesis with no precedent; the existing mechanism already does everything needed once documented |
| Advisory doctor checks (missing-`withdraw` H2, global-origin M5, shared-group R-1) are OPTIONAL, and their placement is an open question | (a) put them in the BGP/watchdog plugin; (b) put them in the as112 plugin; (c) don't build them, rely on tests+docs | (a) hardcodes AS112 knowledge into BGP, violating `ai/rules/plugin-self-containment.md`; (b) the as112 plugin has no visibility into BGP `update`/`asn` config by design. So they are advisory-only, NOT the primary enforcement (tests + docs are). Build them only if a clean, self-containment-respecting home is found at implementation; otherwise ship tests + docs and record the deferral. The primary correctness guarantees (H1/H2) do NOT depend on these checks |
| One shared watchdog group referenced by multiple peer-groups, rather than per-group watchdog groups | Per-peer-group distinct watchdog groups | Simpler operator config (one health signal, many destinations); A-1 confirms the pool keying already supports this |

## Known Limitations
- This spec ships no executable code — operators must hand-author the
  `update`/`healthcheck` config themselves from the worked example; there is
  no single "as112 { bgp { ... } }" turnkey config block (an explicit,
  user-discussed trade-off — see umbrella Known Limitations).

## RFC Documentation

`docs/guide/as112.md`'s worked example cites `// RFC 7534 Section 3.4` next to
the prefix-list/AS-PATH-filter recommendation and `// RFC 7534 Section 4.2`
next to the planned-downtime withdrawal guidance.

## Implementation Summary
### What Was Implemented
- [Filled at closure]

### Bugs Found/Fixed
- [Filled at closure]

### Documentation Updates
- [Filled at closure]

### Deviations from Plan
- [Filled at closure]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-------------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|---------------------------|----------------|----------------------|
| Route announced only when DNS healthy, correct community/origin-AS, withdrawn on failure | interop test | `NN-as112-origin-as`, `NN-as112-community-wire` |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [Filled during implementation]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|------------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|---------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|----------------------------------|------------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`) — N/A for this child (docs/tests only), noted in Deviations
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-as112-3-bgp-integration.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-as112-3-bgp-integration.md` only
