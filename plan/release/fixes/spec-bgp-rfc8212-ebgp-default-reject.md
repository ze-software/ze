# Spec: bgp-rfc8212-ebgp-default-reject

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | The route-server release profile requires RFC 8212. Today an EBGP session with no import or export policy accepts every route into the Decision Process and advertises every route, which is the route-leak default RFC 8212 exists to remove |
| RFC gaps | RFC8212-3-1, RFC8212-3-2 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 8212 Section 3 adds two paragraphs to RFC 4271 (`rfc/full/rfc8212.txt`):

- To Section 9.1: "Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied." (RFC8212-3-1)
- To Section 9.1.3: "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied." (RFC8212-3-2)
- And: "A BGP speaker MAY provide a configuration option to deviate from the following updated behaviors." (RFC8212-3-3)

Ze already makes every CONFIGURED import or export policy state `final accept`
or `final reject` (`ze-bgp-conf.yang` grouping `filter-policy`, `final`
mandatory). It does not apply the RFC's default when no policy is configured:
`filterapi.Policy.AcceptsAtEnd` returns true for the zero Policy, and every
ingress and egress consumer reads it with no session type, so an EBGP peer
with no policy at any of the bgp, group or peer layers behaves as `final
accept` in both directions.

Goal: an EBGP session with no explicit import policy contributes no route to
the Decision Process, and one with no explicit export policy is sent no route,
from every egress path (forwarded, reflected, route-server, config `update`
blocks, API and ExaBGP-bridge injections), while IBGP is unchanged and
ExaBGP-migrated configurations keep their ExaBGP behaviour by stating it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - "BGP policy final disposition", the `// Design:` target of `filterapi/policy.go`
  → Decision: the zero Policy denotes an ABSENT operator policy, never a configured one; internal filters do not imply presence (`InternalPolicy`)
  → Constraint: the final applies only at exhaustion; a filter reject stops the chain whatever the final
- [ ] `docs/architecture/config/syntax.md` - filter chain inheritance section
  → Decision: the most specific PRESENT layer supplies the final; an absent child inherits it (`concatPolicies`); "An absent policy retains prior behavior." is the sentence this spec rewrites
- [ ] `docs/guide/bgp-policy.md` - "Verify policy": `show policy chain` and peer detail omit an absent policy
  → Constraint: the operator must see why an EBGP peer's routes are refused; omission is no longer truthful once absence rejects
- [ ] `docs/guide/redistribution.md` - "An absent direction inherits its parent's policy without creating a new policy."
- [ ] `docs/architecture/bgp/structural-forwarding.md` - the `// Design:` target of `forward_rs.go`; peer-up replay and "export-final restoration" replay share the recipient ADD-PATH identity reservations
  → Decision: AC-6 (reload makes an absent EBGP export explicit) reaches the peer through the existing export-final restoration replay, so the effective-verdict change must trigger the same replay `restoreFinal` triggers today; no new replay path
  → Constraint: the route-server fast path hands every destination it will not send to back on the skipped list; an absent-EBGP destination joins that list, it is never silently passed over

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8212.md` - rows RFC8212-3-1, RFC8212-3-2 (`{gap}`), RFC8212-3-3 (MAY)
  → Constraint: both rows are SHALL NOT on EBGP only; IBGP is outside the requirement
- [ ] `rfc/short/rfc4271.md` - Sections 9.1 and 9.1.3, where RFC 8212 inserts its paragraphs

**Key insights:**
- Both FRR and BIRD apply the default; they differ in WHEN (runtime discard vs config refusal), and both exempt confederation-member sessions (see Key Design Decisions)
- Ze has no BGP confederation support, so session type is `PeerSettings.IsEBGP`, the negation of `isIBGPWith` (local AS or migration AS)

## Interop evidence: FRR and BIRD (read 2026-10-26)

FRR (`bgpd/bgp_route.c`, master, fetched 2026-10-26). The deciding functions:

| Function | Deciding text |
|----------|---------------|
| `bgp_inbound_policy_exists` | returns true when `peer->sort == BGP_PEER_CONFED \|\| peer->sort == BGP_PEER_IBGP \|\| peer->sub_sort == BGP_PEER_EBGP_OAD`; for `BGP_PEER_EBGP` returns true only when `ROUTE_MAP_IN_NAME(filter) \|\| PREFIX_LIST_IN_NAME(filter) \|\| FILTER_LIST_IN_NAME(filter) \|\| DISTRIBUTE_IN_NAME(filter)`; otherwise false |
| `bgp_outbound_policy_exists` | the same shape over the OUT names plus `UNSUPPRESS_MAP_NAME(filter)` |
| `bgp_update` | `if (CHECK_FLAG(bgp->flags, BGP_FLAG_EBGP_REQUIRES_POLICY)) if (!bgp_inbound_policy_exists(peer, &peer->filter[afi][orig_safi])) { reason = "inbound policy missing"; ... goto filtered; }`, with a `zlog_warn` "EBGP inbound policy not properly setup, please configure in order for your peering to work correctly" at most every 15 minutes |
| `subgroup_announce_check` | `if (CHECK_FLAG(bgp->flags, BGP_FLAG_EBGP_REQUIRES_POLICY)) if (!bgp_outbound_policy_exists(peer, filter)) { ... zlog_warn(... "EBGP outbound policy not properly setup ..."); return false; }`. This check runs for every path, locally originated `network` routes included |
| `bgp_vty.c` `FRR_CFG_DEFAULT_BOOL(BGP_EBGP_REQUIRES_POLICY, ...)` | default true, false for the `datacenter` profile and for configs older than 7.4; `bgp ebgp-requires-policy` / `no bgp ebgp-requires-policy` is the RFC8212-3-3 switch, per BGP instance |
| `bgp_vty.c` neighbor JSON | `inboundEbgpRequiresPolicy` "Inbound updates discarded due to missing policy" and the outbound twin are reported per neighbor |

BIRD (`proto/bgp/bgp.c`, v3.2.1, local checkout `~/Code/gitlab.nic.cz/labs/bird`).
The deciding function is `bgp_postconfig`:

| Function | Deciding text |
|----------|---------------|
| `bgp_postconfig` | `int interior = internal \|\| cf->confederation_member;` then per channel: `if (cc->c.in_filter == FILTER_UNDEF) if (interior) cc->c.in_filter = FILTER_ACCEPT; else cf_error("EBGP requires explicit import policy");` and `if (cc->c.out_filter == FILTER_UNDEF) if (interior) cc->c.out_filter = FILTER_REJECT; else cf_error("EBGP requires explicit export policy");` |
| `proto/bgp/config.Y` channel creation | `BGP_CC->c.in_filter = FILTER_UNDEF; BGP_CC->c.out_filter = FILTER_UNDEF;` so only an explicit `import`/`export` clause defines one; BIRD has no deviation switch |

Both refuse the routes; neither sends or accepts without explicit policy on
EBGP. A Ze peer that applies the default interoperates with both, because the
behaviour is local: nothing on the wire changes except which routes are sent.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/filterapi/policy.go` - `Policy{filters, final}`; zero `final` is absent; `Configured()` is `final != 0`; `AcceptsAtEnd()` is `final != FinalReject`, so absent accepts
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `runIngressPolicyChain`: no active filter and `AcceptsAtEnd` gives accept, else `ingressPolicyWithdrawals(payload)`; `runEgressPolicyChain` and `runEgressPolicyChainASN4`: no active filter returns `accept: exportPolicy.AcceptsAtEnd()`; `destInternal` is already a parameter
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - `exportFilterForBody`: originated, injected and bgp-rs `update text` routes; `facts == nil` (no forwarding snapshot) returns accept ("retain the existing admission"); `facts.isEBGP` exists
- [ ] `internal/component/bgp/reactor/peer_run.go` - `session.egressFinalAccepts` closure reads `exportPolicy.AcceptsAtEnd()`
- [ ] `internal/component/bgp/reactor/policy_reload.go` - export final reject on reload triggers the withdrawal barrier
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `restoreFinal` and `finalChanged` compare `AcceptsAtEnd` before and after a reload
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain` applies `AcceptsAtEnd` at exhaustion
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` - `tracePolicyFilterChain` mirrors the live final
- [ ] `internal/component/bgp/reactor/forward_rs.go` - route-server fast path: a destination whose `pf.exportPolicy.AcceptsAtEnd()` is false, or that has an active export filter, is handed back on the skipped list so the cached rail decides it; read twice (first pass and the `only` retry); `pf.isEBGP` is in the same snapshot
  → Constraint: an absent-EBGP destination must be skipped here too, or the fast path sends what the cached rail refuses
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `policyInfo` returns nil for an absent policy with no filters
- [ ] `internal/component/bgp/config/peers.go` - bgp, group, peer and dynamic-group layers combined by `concatPolicies`
- [ ] `internal/component/bgp/config/redistribution.go` - `extractFilterPolicy`, `concatPolicies` (most specific configured final wins)
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `IsEBGP` is `!IsIBGP`; `session_as_migration.go::isIBGPWith` compares local AS and migration AS
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `filter-policy` grouping, presence containers `filter import` and `filter export` at bgp, group and peer
- [ ] `internal/exabgp/migration/migrate.go` - `migrateNeighbors` builds each peer tree; no `filter` container is written

**Behavior to preserve:**
- IBGP sessions: absent policy accepts in both directions, as today
- Any configured policy, at any layer, keeps its stated final; filter semantics, ordering and the reload withdrawal machinery are unchanged
- `final reject` on export still withdraws previously advertised routes on reload; withdrawals still pass a rejecting export final
- `show policy chain` and peer detail output for a configured policy

**Behavior to change:**
- EBGP session, import direction, no configured policy at any layer: every received route is treated as policy-rejected (the existing `ingressPolicyWithdrawals` path), so none reaches the Decision Process
- EBGP session, export direction, no configured policy at any layer: no route is added to its Adj-RIB-Out from any egress path, including before the forwarding snapshot exists
- The absent EBGP verdict is visible: peer detail, `show policy chain` and policy dry-run report it, and one warning per direction per session says why and how to fix it
- `ze exabgp migrate` writes an explicit `filter import { final accept; }` and `filter export { final accept; }` on every migrated peer

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Ingress: UPDATE wire bytes from an established EBGP session, reaching `runIngressPolicyChain`
- Egress: a route selected for an EBGP destination, from the forward pool (`runEgressPolicyChain`), or from originated, injected, API, ExaBGP-bridge and bgp-rs paths (`exportFilterForBody`, `egressFinalAccepts`)
- Config: `bgp filter`, `bgp group <g> filter`, `bgp peer <p> filter` presence containers

### Transformation Path
1. Config parse: `extractFilterPolicy` per layer, `concatPolicies` into `PeerSettings.ImportPolicy` / `ExportPolicy` (absent stays the zero final)
2. Session type: `PeerSettings.IsEBGP` at the decision, read through the guarded `Peer.IsEBGP` or the forwarding snapshot's `isEBGP`
3. Exhaustion verdict: one function of (policy, session external) replaces `AcceptsAtEnd()`; absent and external yields reject
4. Ingress reject becomes withdrawals (`ingressPolicyWithdrawals`); egress reject suppresses the announcement and keeps withdrawals
5. Status: `policyInfo`, `show policy chain`, dry-run read the same verdict

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to reactor | `PeerSettings.ImportPolicy` / `ExportPolicy` (zero final = absent) | No |
| Reactor to plugins (rib, adj-rib-in, rs) | ingress reject arrives as withdrawals before dispatch; plugins see no rejected route | No |
| Plugins to reactor (bgp-rs, API, ExaBGP bridge) | announcements reach `exportFilterForBody` per destination | No |
| Reactor to CLI/JSON | `plugin.PolicyInfo` in peer detail and `show policy chain` | No |

### Integration Points
- `filterapi.Policy` - the verdict function lives here, beside the type that encodes absence
- `forwardFacts` snapshot - already carries `isEBGP` and `exportPolicy` together
- `ingressPolicyWithdrawals` / `policyWithdrawals` - reused, no new reject path
- `internal/exabgp/migration` - emits explicit policies on migrated peers

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The verdict stays inside the existing chain functions; no new egress or ingress path |
| No unintended coupling (components stay isolated) | Yes | filterapi gains a session-type argument, not a reactor import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the final-reject withdrawal machinery |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The absent path does no serialization, as the current no-active-filter path |
| Registration over hardcoding, outbound | N-A | No command, family or handler is added |
| Registration over hardcoding, inbound | Yes | No list learns a name; the rule is a property of the session type and policy presence |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every exhaustion verdict goes through `AcceptsAtEnd`, so changing its signature makes the compiler find every consumer | `grep AcceptsAtEnd` (2026-10-28) lists filter_ordered, filter_chain, egress_inject_filter, peer_run, policy_reload, peer_settings_apply, reactor_api, policy_dryrun and forward_rs (route-server fast path, two reads); the only other `Policy.Configured()` reader is `config/redistribution.go` (layer merge) | A path keeps the absent-accepts verdict | Signature change compiles only once every caller passes the session type; reviewer greps for `Configured()` readers too | validated 2026-10-28; forward_rs added to Files to Modify |
| A-2 | The ingress reject happens before rib, adj-rib-in and bgp-rs see the route | `runIngressPolicyChain` returns withdrawals in the ordered ingress pipeline | A plugin stores or forwards a route RFC 8212 makes ineligible | Functional test asserts the RIB and bgp-rs export are empty | unvalidated |
| A-3 | `IsEBGP` is known at every decision: ingress and egress only run on an established session, whose peer AS is fixed | `Peer.IsEBGP`, `forwardFacts.isEBGP`; a dynamic peer's template carries `PeerAS = 0`, and `publishDynamicResolution` writes PeerAS and both resolved policies together under `p.mu` at establishment | An undecided session type would need a third answer | Read `reactor_dynamic.go` 2026-10-28: route decisions hold. Status is the exception: before establishment `isIBGPWith(0)` is false, so a dynamic IBGP template would read as EBGP; status reports such a direction as undetermined (AC-8) | validated 2026-10-28 for route decisions; status constraint added |
| A-4 | An explicit `filter import { final accept; }` at the bgp level is the operator's explicit Import Policy for every peer, so no separate RFC8212-3-3 switch is needed | RFC 8212 Section 1: "routes are neither imported nor exported unless specifically enabled by configuration" | An operator could ask for a knob FRR-style | Owner reads Key Design Decisions | unvalidated |
| A-5 | ExaBGP's own behaviour is accept-all in both directions with no policy | ExaBGP has no import or export policy construct | Migrated configs would over-permit | Searched `~/Code/github.com/exa-networks/exabgp/main` 2026-10-28: no import/export policy, route-map or RFC 8212 handling | validated 2026-10-28 |
| A-6 | Adding an explicit `final accept` to an RFC-tagged fixture's config does not change what its tag proves, so its discrimination record stays valid or is re-recorded mechanically | The edit restores today's behaviour for that fixture; no assertion changes | 46 tagged carriers each owe a fresh `./le rfc record` and a judge verdict, a phase of its own | Read how `./le rfc check` stales a record (`docs/contributing/rfc-conformance-gates.md`) before Phase 4; if a config edit stales it, Phase 4 re-records each carrier | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Hundreds of functional tests configure EBGP peers with no policy and go red. Census 2026-10-28 (heuristic parse of every `.ci` config block and interop `ze*.conf`): 656 fixtures carry an EBGP peer with an absent direction; about 230 exchange routes with it and change behaviour (test/plugin 184, interop at least 114 scenario dirs, test/encode 9, test/reload 7, test/draft 15 not run); test/parse, test/ui, static, firewall and vrrp (about 150) only bring sessions up or parse and are unaffected under the runtime-discard design. 46 affected `.ci` carry `RFC requirement:` tags; 11 assert peer-detail or policy output; 6 `test/exabgp/*/expected.conf` migrate goldens change; about 25 reactor Go test files drive the chain functions with EBGP settings and a zero Policy (estimate 30 to 60 functions). Blind spots: configs built in Go (interoplab templates, chaos, stress, perf), `.et` tests | `./le test` runs of the reached suites go red with routes missing | Phase 4 adds the explicit policy each affected config needs, changing no assertion: merged into an existing bgp-level `filter` where one exists, the missing direction only where one is already configured, and in every reload generation (`next.conf`, `ze-reload.conf`) so a reload does not flip the verdict. Peer-detail goldens change through `ze-test-judge`. Go-built configs are found by running the reached packages after Phase 2 |
| R-2 | `exportFilterForBody` accepts when no forwarding snapshot exists (`facts == nil`), a fail-open before the snapshot | An originated route reaches an EBGP peer with no export policy in the window | Fall back to `peer.ExportPolicy()` and `peer.IsEBGP()` under the lock, as `egressFinalAccepts` already does |
| R-3 | A reload that adds or removes the only policy layer changes the verdict without restarting the session | Routes neither withdrawn nor re-sent after the reload | `restoreFinal` / `finalChanged` compare the effective verdict (policy and session type), not the raw final |
| R-4 | Looking-glass and route-collector deployments silently receive nothing after upgrade | Empty RIB with established sessions | Warning at session up naming the fix; profile docs and examples state the explicit import accept / export reject a collector needs |
| R-5 | A hand-written ze config used with the ExaBGP bridge (not produced by `ze exabgp migrate`) stops sending API routes | Bridge announcements not on the wire | Warning names the fix; the ExaBGP difference is documented, per the owner rule that the RFC outranks ExaBGP behaviour |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes silently not exchanged with EBGP peers (too strict), or the route leak stays (too lax) |
| How is it reverted? | Single commit revert of the verdict function; configs that added explicit policies keep working either way |
| Who else touches this path? | bgp-rs, rib, adj-rib-in plugins; the ExaBGP bridge and migration; policy reload; every BGP functional suite |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| EBGP peer, no import policy, peer sends UPDATE | → | `runIngressPolicyChain` absent-EBGP reject | `test/plugin/rfc8212-ebgp-no-import-policy.ci` |
| EBGP peer, no export policy, config `update` block and a forwarded route | → | `exportFilterForBody`, `runEgressPolicyChain` absent-EBGP reject | `test/plugin/rfc8212-ebgp-no-export-policy.ci` |
| IBGP peer, no policy | → | absent-IBGP accept | `test/plugin/rfc8212-ibgp-no-policy-unchanged.ci` |
| Reload adds `filter import { final accept; }` | → | reload re-evaluation | `test/reload/rfc8212-explicit-policy-reload.ci` |
| `ze exabgp migrate` of an EBGP neighbor | → | `migrateNeighbors` emits explicit policies | `TestMigrateWritesExplicitRFC8212Policies` and the existing `test/exabgp-compat` suite run unchanged |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EBGP session, no import policy at bgp, group or peer layer, peer announces prefixes | No prefix enters the RIB or is offered to bgp-rs; `show` of the RIB is empty for that peer |
| AC-2 | EBGP session, no export policy at any layer; routes exist from config `update`, another peer, the API and the ExaBGP bridge | The peer receives no announcement from any of the four sources, including during session start before the forwarding snapshot |
| AC-3 | Same as AC-1 and AC-2 with `final accept` configured at the bgp level only | Routes flow in both directions, as today |
| AC-4 | IBGP session, no policy | Routes flow in both directions, as today |
| AC-5 | EBGP session with only automatic (internal) filters, no operator policy | Treated as absent: AC-1 and AC-2 apply |
| AC-6 | Established EBGP session, reload adds an import or export policy with `final accept` | Received routes become eligible and are installed (route refresh or stored replay); exportable routes are announced, without a session reset |
| AC-7 | Established EBGP session, reload removes the only export policy | Previously advertised routes are withdrawn |
| AC-8 | Peer detail, `show policy chain` and policy dry-run for an EBGP peer with an absent direction | Each reports the direction's effective final as reject and marks it as the RFC 8212 default, distinct from a configured final; an IBGP absent direction stays omitted; a dynamic peer not yet established (peer AS unknown) reports an absent direction as undetermined, never as reject or accept |
| AC-9 | EBGP session reaches Established with an absent direction | One warning per direction per session names the peer, the direction, RFC 8212, and the exact config command that adds the explicit policy |
| AC-10 | `ze exabgp migrate` of any ExaBGP neighbor | The produced peer carries `filter import { final accept; }` and `filter export { final accept; }`, and the migration output says why |
| AC-11 | RFC8212-3-1 and RFC8212-3-2 | Each carries a positive and a negative tagged test with a discrimination record; the `{gap}` annotations are removed from `rfc/short/rfc8212.md` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an EBGP peer and forgets policy | config, session up, UPDATE, ingress reject, warning | `test/plugin/rfc8212-ebgp-no-import-policy.ci` |
| 2 | Runs an IXP route server with group policies | dynamic group policy, bgp-rs, egress | `test/plugin/rfc8212-ebgp-no-export-policy.ci` plus an existing route-server suite with explicit policies |
| 3 | Migrates an ExaBGP config and runs its API scripts | migrate, bridge, `exportFilterForBody` | `TestMigrateWritesExplicitRFC8212Policies` and `test/exabgp-compat` |
| 4 | Peers with FRR | interop | `bgp-rfc8212-default-reject-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC8212Section3AbsentImportRejectsOnEBGP` | `internal/component/bgp/reactor/rfc8212_default_reject_test.go` | AC-1, AC-5 | |
| `TestRFC8212Section3AbsentExportRejectsOnEBGP` | `internal/component/bgp/reactor/rfc8212_default_reject_test.go` | AC-2, R-2 | |
| `TestRFC8212AbsentPolicyAcceptsOnIBGP` | `internal/component/bgp/reactor/rfc8212_default_reject_test.go` | AC-4 | |
| `TestPolicyVerdictAtExhaustion` | `internal/component/bgp/filterapi/policy_test.go` | absent x external, accept, reject truth table | |
| `TestRFC8212ReloadReevaluatesVerdict` | `internal/component/bgp/reactor/` | AC-6, AC-7, R-3 | |
| `TestRFC8212PolicyInfoReportsDefault` | `internal/component/bgp/reactor/` | AC-8 | |
| `TestMigrateWritesExplicitRFC8212Policies` | `internal/exabgp/migration/` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc8212-ebgp-no-import-policy` | `test/plugin/` | EBGP peer without import policy contributes no route; warning logged | |
| `rfc8212-ebgp-no-export-policy` | `test/plugin/` | EBGP peer without export policy receives nothing from config, forward or API | |
| `rfc8212-ibgp-no-policy-unchanged` | `test/plugin/` | IBGP unaffected | |
| `rfc8212-explicit-policy-reload` | `test/reload/` | adding policy by reload starts the exchange without reset | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc8212-default-reject-frr` | `test/interop/` | FRR (`bgp ebgp-requires-policy` on, explicit route-maps) | Ze with no policy installs none of FRR's routes and FRR receives none of Ze's; after a reload adding explicit accept, both RIBs carry the routes. Forced red: revert the verdict, rebuild, FRR's routes appear in Ze's RIB | |

Scoped evidence: `./le test unit` over `internal/component/bgp/filterapi`,
`internal/component/bgp/reactor`, `internal/exabgp/migration`; the named
`.ci` tests; `./le test interop` for the scenario; then `./le verify changed`
over the closing commits.

## Files to Modify
- `internal/component/bgp/filterapi/policy.go` - replace `AcceptsAtEnd()` with a verdict that takes the session type; the zero Policy no longer answers alone
- `internal/component/bgp/reactor/filter_ordered.go` - pass session type on ingress and egress
- `internal/component/bgp/reactor/filter_chain.go` - same at exhaustion
- `internal/component/bgp/reactor/egress_inject_filter.go` - session type, and close the `facts == nil` window (R-2)
- `internal/component/bgp/reactor/forward_rs.go` - route-server fast path skips an absent-EBGP destination in both passes
- `internal/component/bgp/reactor/peer_run.go` - `egressFinalAccepts`
- `internal/component/bgp/reactor/policy_reload.go`, `peer_settings_apply.go` - compare effective verdicts (R-3)
- `internal/component/bgp/reactor/policy_dryrun.go` - same verdict as live
- `internal/component/bgp/reactor/reactor_api.go` - `policyInfo` reports the RFC 8212 default
- `internal/component/plugin/types_bgp.go` - `PolicyInfo` gains the marker for a default final
- session-up warning in the reactor's Established handling
- `internal/exabgp/migration/migrate.go` - explicit policies per migrated peer
- `rfc/short/rfc8212.md` - remove `{gap}`, add tags
- functional test fixtures that configure EBGP peers with no policy (Phase 4)

## Files to Create
- `internal/component/bgp/reactor/rfc8212_default_reject_test.go`
- `test/plugin/rfc8212-ebgp-no-import-policy.ci`, `test/plugin/rfc8212-ebgp-no-export-policy.ci`, `test/plugin/rfc8212-ibgp-no-policy-unchanged.ci`
- `test/reload/rfc8212-explicit-policy-reload.ci`
- `test/interop/bgp-rfc8212-default-reject-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The explicit policy containers already exist; no switch is added (A-4) |
| YANG validation constraints | No | Unchanged |
| YANG custom validators | No | Unchanged |
| CLI commands/flags | No | No new command; existing peer detail and `show policy chain` change output |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `show policy chain` output for an absent EBGP direction, in `rfc8212-ebgp-no-import-policy.ci` |
| Pipe completeness | N-A | Existing commands |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | No | No runtime dependency added |
| Prometheus counters/metrics | No | The existing policy-reject accounting applies; design confirms whether a per-reason label exists |
| BGP family surface (new SAFI / capability / attribute) | N-A | Family independent |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the BGP policy feature file under `features/` (find with `./le feature report`) |
| 2 | Config syntax changed? | Yes (semantics) | `docs/architecture/config/syntax.md` ("An absent policy retains prior behavior."), `docs/guide/configuration.md` examples with EBGP peers |
| 3 | CLI command added/changed? | Yes (output) | `docs/guide/bgp-policy.md` "Verify policy" |
| 4 | API/RPC added/changed? | Yes (output) | `docs/architecture/api/commands.md` `ze-bgp:show-policy-chain` |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md`, `docs/guide/bgp-peering.md`, `docs/guide/redistribution.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `PolicyInfo` JSON in `docs/architecture/api/process-protocol.md` if it documents peer policy |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8212.md`, `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` policy final disposition; `docs/architecture/bgp/structural-forwarding.md` where the route-server fast path's skip conditions are described; `docs/architecture/api/architecture.md` (declared by a changed file's `// Design:` header) where it describes peer policy reporting |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every guide example of an EBGP peer gains its explicit policy; ExaBGP migration guide states the emitted policies |

## Implementation Steps

1. **Phase: Wiring** -- write the four `.ci` tests and the unit truth table; they fail against HEAD
2. **Phase: Verdict** -- the session-typed exhaustion verdict in `filterapi`, every consumer passes the session type; R-2 window closed
3. **Phase: Reload and status** -- effective-verdict comparison on reload; `policyInfo`, `show policy chain`, dry-run and the session-up warning
4. **Phase: Fixtures** -- add an explicit policy to every functional and interop config whose EBGP peers rely on the absent default; no assertion changes; `ze-test-judge` for RFC-tagged and golden fixtures
5. **Phase: ExaBGP migration** -- explicit policies per migrated peer and the migration note; `test/exabgp-compat` green unchanged
6. **Phase: Interop and RFC records** -- `bgp-rfc8212-default-reject-frr` with forced red; `./le rfc record` for both rows; docs

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No consumer of the exhaustion verdict reads it without the session type; `grep AcceptsAtEnd` is empty or every hit passes it |
| Correctness | An absent IBGP direction is byte-identical to today |
| Data flow | bgp-rs, API and ExaBGP-bridge announcements to an EBGP peer pass the same verdict as forwarded routes |
| Rule: principles (silent zero) | The zero Policy is never read as an answer by itself |
| Rule: user-facing-errors | The warning names the peer, direction, RFC and the exact fix command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Session-typed verdict | `grep -rn AcceptsAtEnd internal/` shows no session-less call |
| Gap rows closed | `grep gap rfc/short/rfc8212.md` empty for 3-1 and 3-2 |
| Interop | scenario log with green and forced-red runs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | Every egress path, including the pre-snapshot window, rejects on absent EBGP export |
| Authorization that could fail open | A filter execution failure still rejects, as today |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer.md` | 2026-10-26 | `Policy.AcceptsAtEnd` accepts an absent policy on EBGP |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Apply the default at the decision, per session, FRR-style (`bgp_update`, `subgroup_announce_check`) | Refuse the config at commit, BIRD-style (`bgp_postconfig` `cf_error`) | The session type is only certain once the session runs (dynamic groups, migration AS); a refusal would also reject a valid running config on upgrade with no peer to blame. The runtime verdict plus a warning and a status marker gives the operator the same information FRR gives in `inboundEbgpRequiresPolicy` |
| No new RFC8212-3-3 switch; an explicit `final accept` at the bgp level is the deviation | An FRR-like `no bgp ebgp-requires-policy` leaf | The MAY is already met by explicit configuration that states its intent; a second knob would be a second declaration of the same choice, and BIRD offers none |
| Automatic (internal) filters do not count as an explicit policy | Treat any filter as presence | `InternalPolicy` already encodes "Internal filters do not imply presence"; RFC 8212 asks for operator intent |
| Absent import reuses the policy-reject withdrawal path | Drop silently before the chain | One reject path; withdrawals keep any prior state consistent |
| Absent export applies to every egress source, config `update` blocks and API injections included | Exempt locally originated routes | RFC8212-3-2 names the Adj-RIB-Out, not the route's origin, and FRR's `subgroup_announce_check` applies to every path |
| `ze exabgp migrate` writes explicit accept on every migrated peer, not a global policy | A global `bgp filter` accept | ExaBGP has no policy, so explicit accept preserves its behaviour; per peer, a peer the operator adds later still gets the RFC 8212 default. The RFC outranks ExaBGP behaviour (owner rule), so the bridge itself enforces the default and the difference is documented |
| Confederation: no exemption code | Copy FRR's and BIRD's confed-member exemption | Ze has no confederation support; both FRR (`BGP_PEER_CONFED`) and BIRD (`confederation_member` is `interior`) exempt confed-member sessions, which a future confederation spec must decide |

## Known Limitations
- BGP confederations are not supported, so the confederation-member exemption both FRR and BIRD apply has no Ze counterpart yet; the spec that adds confederations owns that decision

## RFC Documentation (Scope: protocol)

Add `// RFC 8212 Section 3: "<quoted requirement>"` above the verdict and at
each call site's section reference, per `ai/rules/rfc-compliance.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] FRR and BIRD deciding functions quoted

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: scoped `./le test unit`, the named `.ci` tests, the interop scenario, `./le verify changed` over the closing commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md`

## Design validation, 2026-10-28

The owner selected design validation on resumption. These findings supersede
the conflicting draft claims identified below. Status stays `design`; AC-6
needs the recovery decision below before final approval.

### Verified constraints

| Draft item | Finding and design consequence | Producer read |
|------------|--------------------------------|---------------|
| A-2, Data Flow step 4, Key Design Decisions import rejection | Denied announcements are suppressed, not converted to withdrawals. Pure withdrawals pass; mixed UPDATEs retain their explicit withdrawal sections. Changing the import verdict alone cannot remove routes accepted earlier. | `internal/component/bgp/reactor/filter_ordered.go`, `runIngressPolicyChain`, `ingressPolicyWithdrawals`, `policyWithdrawals` |
| A-2, ingress boundary | The ordered ingress loop precedes cache insertion, route-server fast forwarding and plugin delivery. An announcement-only rejection returns before these consumers. The acceptance criterion is no eligible or forwarded denied route, not a prohibition on pre-policy storage. | `internal/component/bgp/reactor/reactor_notify.go`, `notifyMessageReceiver` |
| AC-6 import | `applyHotSwappableSettings` publishes import policy but reconciles only changes to the export final. The existing import reload does not request the denied routes again. Outbound stored-route replay is not import re-evaluation. | `internal/component/bgp/reactor/peer_settings_apply.go`, `applyHotSwappableSettings`; `internal/component/bgp/plugins/adj_rib_in/rib.go`, `walkReplayRoutes` |
| AC-6 recovery prerequisite | The existing refresh sender requires negotiated Route Refresh and requests each negotiated family. It logs a send failure and returns no error. Transactional import recovery must return failures and bind requests to the captured session, rather than use this helper unchanged. | `internal/component/bgp/reactor/peer_derived_bindings.go`, `requestRouteRefreshAll` |
| AC-6 export | Preserve the documented source ownership: regenerate configured routes and replay authorized retained routes. A one-shot API or ExaBGP injection needs producer reannouncement after acceptance returns; do not introduce desired-state storage for it. | `docs/guide/bgp-policy.md`, Policy model; `docs/architecture/core-design.md`, Policy Filter Chain |
| A-4, RFC8212-3-3 | An inherited explicit policy remains configured: `concatPolicies` keeps the most-specific configured final. Explicit accept satisfies the policy-presence requirement; it is not a deviation from RFC 8212. Section 3 says a speaker MAY provide a deviation option, not that it must. Keep the no-new-switch design, but do not count explicit policy as proof of a separate deviation switch. | `internal/component/bgp/config/redistribution.go`, `extractFilterPolicy`, `concatPolicies`; `rfc/full/rfc8212.txt`, Section 3 |
| A-6 | A `.ci` is a whole-file tagged unit. Adding policy in `stdin=` changes its discrimination behavior hash. `tmpfs=` bodies are omitted by that hash, so an external or temporary fixture change must be judged by its effect, not assumed covered by hash freshness. | `internal/le/rfc/goscope.go`, `UnitAt`; `internal/le/rfc/tags.go`, `dropCarrierBlocks`, `behaviorBytes`; `internal/le/rfc/discriminate.go`, `verifyOneDiscrimination` |
| A-6, Phase 4 | Audit fingerprints use normalized whole-unit text, not the discrimination normalization. Changed tagged fixtures and cited producer units require the affected audit verdicts to be rejudged; mechanical resealing is not the route for changed behavior. Renew affected discrimination covers and obtain independent test judgment in the implementation change. One carrier can have several requirement/polarity covers. | `internal/le/rfc/inventory.go`, `RequirementSHA`; `internal/le/rfc/check_audit.go`, `checkAuditFreshness`; `docs/contributing/rfc-conformance-gates.md`, The discrimination record |
| AC-8 files | `PolicyInfo` is projected into a separate `policyChain`; changing the reactor result alone loses a new marker in `show policy chain`. Include `internal/component/bgp/plugins/cmd/policy/handler.go` and its renderers in the implementation surface. Specify unknown session type separately from a configured or default final. | `internal/component/plugin/types_bgp.go`, `PolicyInfo`; `internal/component/bgp/reactor/reactor_api.go`, `policyInfo`; `internal/component/bgp/plugins/cmd/policy/handler.go`, `toPolicyChain` |

### Related work boundary

`plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md`
owns pre-policy retention for route-server clients in the existing store.
Its design excludes non-route-server retention and does not define import
re-evaluation. RFC 8212 Section 3 says: "Routes contained in an Adj-RIB-In
associated with an EBGP peer SHALL NOT be considered eligible in the Decision
Process if no explicit Import Policy has been applied."

→ Constraint: do not turn AC-1 into an assertion that the pre-policy
Adj-RIB-In is empty. Assert selection and forwarding exclusion.
→ Constraint: coordinate the mixed-UPDATE case with the RFC 7947 spec:
its proposed rejection hook must not miss denied announcements when this
spec's withdrawal projection returns acceptance for the surviving withdrawals.
→ Constraint: retained import recovery, if selected, needs an explicit
dependency and re-evaluation contract; outbound replay cannot substitute.

### Additional risks and assumptions

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-6 | Live import relaxation succeeds but previously denied routes remain missing | Policy reports accept while the peer's stable routes remain absent | Resolve D-1 before approval; cover refresh capability present/absent and optional receive store present/absent |
| R-7 | Removing the last import policy leaves previously accepted paths eligible | Existing selected routes and downstream advertisements survive the reload | Specify ordered source-state invalidation, including queued deliveries, or an explicit refusal/restart contract; a verdict change and refresh alone are insufficient |
| R-8 | A partial refresh send, concurrent session replacement or transaction rollback leaves routes admitted under an abandoned policy | Reload fails after some families received new routes | Fence the session and receive/delivery generation; specify reconciliation and rollback before implementing import reload |
| R-9 | Fixture migration keeps old proof over changed setup | Changed `stdin=` carrier has a stale unit hash or audit verdict | Phase 4 renews every affected cover and rejudges affected verdicts; do not restrict Phase 6 evidence work to RFC 8212 |
| R-10 | A new default marker is dropped or unknown dynamic AS is treated as EBGP | Peer detail and policy-chain output disagree | Carry the state through the command projection and dry-run; test dynamic templates before and after establishment |

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-7 | A negotiated Route Refresh can recover denied input without new storage | `requestRouteRefreshAll` sends one request for each negotiated family | Remote routes cannot be recovered on the same session | Functional peer advertises routes once, answers refresh, and preserves session generation; assert recovered eligible prefixes, not only request transmission | Design proposal; runtime proof owed |
| A-8 | Existing pre-policy retention work can coexist with default rejection | RFC 7947 spec's filtered entry and replay exclusion design | A denied mixed UPDATE disappears from retention or becomes eligible | Joint mixed-UPDATE test: retained filtered announcements, preserved withdrawals, no selection or export | Design coordination owed |

### D-1: import relaxation when Route Refresh is unavailable

AC-6 currently promises recovery without a session reset and states no
prerequisite. The current receive path has discarded the denied announcements.
This is a product contract choice, not a test exception.

| Option | Contract | Trade-off |
|--------|----------|-----------|
| Refuse the live relaxation (recommended) | With negotiated Route Refresh, publish the policy and request every negotiated family on the same session. Without it, reject before publication, keep the old policy, and name the operator's recovery action. | No new route store and no automatic reset; AC-6 must explicitly distinguish supported recovery from refusal |
| Restart only when refresh is unavailable | Use refresh when negotiated; otherwise restart the affected peer under the new policy so its initial advertisements repopulate state. | Accepts the configuration but interrupts that session; changes AC-6's no-reset promise |
| Retain and re-evaluate denied routes | Extend the existing receive store and define import re-evaluation for every affected peer, coordinated with the RFC 7947 spec. | Preserves the broad no-reset promise but adds retained state, storage prerequisites and a larger lifecycle design |

D-1 is pending owner choice. No option authorizes leaving accepted source
paths eligible after import policy removal. R-7 still requires its own
transition design, whichever recovery option is selected.

### Surface corrections for the final spec

| Surface | Required correction |
|---------|---------------------|
| Feature declaration | Update `features/bgp-protocol.md`, currently experimental in `./le feature report`, with RFC 8212 scope and the new real-path evidence; do not invent an unnamed BGP-policy feature file |
| Process/API contract | Update `docs/architecture/api/process-protocol.md`, Policy Filter Chain and operational `PolicyInfo`, unconditionally; the page already documents these fields |
| ExaBGP guides | Name `docs/exabgp/exabgp-migration.md` and `docs/exabgp/exabgp-differences.md`; migration emits explicit policies, while hand-written bridge configurations still receive the default |
| Reload guide | Add `docs/guide/config-reload.md` and document the selected import recovery/refusal contract in `docs/guide/bgp-policy.md` |
| Interop scenario | Create `test/interop/scenarios/bgp-rfc8212-default-reject-frr/`, not a directory directly under `test/interop/`; `internal/le/interoplab/bgp/register_catalog.go` declares that root and the `INTEROP_SCENARIO` selector |
| Discovery | Add RFC 8212/default-reject keywords to `ai/INDEX.md`, pointing to `docs/guide/bgp-policy.md` and `rfc/short/rfc8212.md`; use the existing policy type, RFC ledger and feature registry rather than a parallel inventory |
| Regression proof | Add import removal, unsupported refresh, partial failure/rollback, mixed IPv4/MP withdrawals and dynamic-session status to the concrete test matrix; retain independent RFC judgment and discrimination proof |
| Documentation timing | Each implementation phase updates the pages whose behavior it changes; Phase 6 verifies those edits rather than owning all documentation |

### Validation performed

Source inspection above confirmed the draft's unresolved contracts. The real
tool commands `./le feature report` and `./le spec citation anchors spec
plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md` ran. The citation
command reported advisory document mentions and no blocking missing declared
page. No BGP runtime test, implementation, or conformance proof was performed.
