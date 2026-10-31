# Spec: bgp-rfc8212-ebgp-default-reject

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | The route-server release profile requires RFC 8212. Today an EBGP session with no import or export policy accepts every route into the Decision Process and advertises every route, which is the route-leak default RFC 8212 exists to remove |
| RFC gaps | RFC8212-3-1, RFC8212-3-2 |
| Depends | - |
| Phase | 4/7 |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Current implementation status and restart instructions are in
[Implementation checkpoint](#implementation-checkpoint). The task and design
audit below describe the pre-implementation baseline, not the current verdict.

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
| A-10 | Named automatic loop filters already reach their in-process executor without an external RPC | `applyLoopDetectionConfig` reads the named entry and `LoopIngress` runs in the protocol stage | The automatic-only positive control fails before reaching the policy final | Phase 1 native import fixture and config-to-dispatch producer trace | broken 2026-10-28; prerequisite repaired and verified below |
| A-11 | Startup subscriptions retain every supplied peer selector | The SDK and RPC accept a peer list | The export fixture's accepted peer sends its receipt but the observer never sees it | Saved native wire capture and `registerSubscriptions` producer | broken 2026-10-28; prerequisite repaired and verified below |

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
| `test-asserts-superseded-behaviour.md` | 2026-10-30 | reactor tests expect accept for EBGP with no export policy |
| `unwired-feature.md` | 2026-09-02 | Named automatic loop-filter references dispatch to a nonexistent external plugin |
| `unwired-feature.md` | 2026-10-28 | Startup subscriptions discard every peer selector after the first |
| `discarded-error-becomes-destructive.md` | 2026-10-30 | bgp-rib apply acknowledges a refused downstream withdrawal |
| `test-asserts-superseded-behaviour.md` | 2026-10-31 | 10 adj-rib-in and redistribute plugin fixtures had no EBGP import policy (fixed) |

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

Owner decision, 2026-10-28: **Refuse live change**. D-1 is resolved:
an established peer without negotiated Route Refresh keeps its current policy,
and the transaction refuses the import relaxation before publication.
An optional receive store does not bypass this check. With Route Refresh,
the successful recovery path keeps the session and requests every negotiated
family. No new retention store or automatic restart is authorized for this
relaxation. AC-6 must carry both the recovery and refusal cases.

The refusal names this recovery sequence using the peer's concrete selector:
`request peer <selector> disable`, wait until that session is down, commit
the policy change, then `request peer <selector> enable`. The commands hold
and release the peer through `handlePeerDisable` and `handlePeerEnable` in
`internal/component/bgp/plugins/cmd/peer/admin.go`. A teardown alone is not
a stable editing window, because it permits reconnection.

### D-2: removal of the last accepting import policy

This transition currently has no acceptance criterion. It must not report a
successful removal while previously accepted routes remain eligible. D-1
settled recovery from rejection; it did not authorize a restart or choose
how tightening invalidates existing source routes.

| Option | Contract | Trade-off |
|--------|----------|-----------|
| Refuse live removal (recommended) | Reject an established peer's accepting-to-absent import transition before publication. The operator uses the disable, commit, enable sequence above. Removal while down applies normally; removal of a child that still inherits explicit policy is not this transition. | Uses the same explicit interruption contract as D-1, with no new source-reconciliation machinery; the operator cannot remove the last accepting policy in one live commit |
| Restart the affected source session | Apply the removal through session-bound configuration-change teardown, wait for ordered source cleanup, then reconnect under default reject. | Makes one commit sufficient, but interrupts the source session, including its otherwise unchanged export direction |
| In-place source invalidation | Keep TCP up and introduce an ordered invalidation of all formerly eligible source paths and downstream advertisements, fenced against queued old-policy deliveries and rollback. | No transport interruption, but needs a complete cross-plugin lifecycle design rather than changing the final alone |

→ Constraint: `Peer.teardownOf` in `internal/component/bgp/reactor/peer.go`
can queue a teardown and return success before cleanup. A restart design
cannot treat that return as proof that source paths have been removed.
`Peer.sessionDown` in `peer_session_end.go` drains the receive delivery worker
before announcing down. A restart also needs explicit no-retention handling
if its NOTIFICATION cannot be delivered: `sessionCloseFor` in `peer_close.go`
does not classify every local close as `ReasonNotification`.

D-2 is pending owner choice. In-place source invalidation is not implied by
the existing export reconciliation, and no implementation starts before this
transition is specified.

Owner decision, 2026-10-28: **Invalidate in place**. D-2 is resolved:
removing the last accepting import policy keeps the source TCP session up
and invalidates its formerly eligible routes and downstream advertisements.
The implementation must order this against queued received UPDATEs,
forwarding and transaction rollback. Neither refusal of the live removal
nor an automatic restart is the normal successful path.

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

## Completed design proposal, 2026-10-28

This section is proposed for the design gate. Once approved it supersedes
conflicting draft rows above; it does not record implementation or test success.
D-1 and D-2 remain the owner's decisions.

### In-place import invalidation and rollback

→ Constraint: `receivePublication.wait` proves publication into process queues,
not application. `Process.DrainEventsApplied` supplies the next receipt, but a
route-server handler can enqueue departure work and return before downstream
writes finish (`rs.handleStateDown`, `enqueueDeparture`). All three boundaries
must finish before reporting that invalidation succeeded.

→ Constraint: `forwardSourceCurrent` checks session generation, whereas
`adjOutOwnerCurrent` additionally checks revocation. An unchanged TCP session
does not invalidate queued forwarding. The source policy epoch must be captured
at admission and checked at the final writer, never stamped onto old work later.

→ Constraint: `inboundEmpty` clears selecting-RIB storage only; `releaseRoutes`
acts on retained peers. Neither is the required operation. Synthetic DOWN/UP
would change export-destination availability, GR state and replay, so it is not
an implementation of live import invalidation.

→ Decision: extend existing receive backpressure and route ownership with
transaction-scoped reversible invalidation, not a permanent denied-route store.
`Session.Pause` currently lacks an acknowledgement that an in-flight message
has stopped. Add a session-bound admission receipt before policy evaluation;
preserve an already-read message and any coalesced messages without evaluating
them under a tentative policy.

| Stage | Required state and completion |
|-------|-------------------------------|
| Prepare | Preflight the complete candidate, static peers and instantiated dynamic peers included. Capture transaction, peer incarnation, session generation, policy epoch, template generation and participating owner lifetimes. Pin existing route/attribute and advertisement ownership references needed for undo. Refuse an unsupported state owner before policy publication, naming that owner and its remedy |
| Pause and drain | Close source admission before policy evaluation; await its receipt, the captured receive-publication frontier, strict process application receipts, and already-admitted forwarding. Leave later bytes in existing bounded transport buffers. Hold neither reactor nor peer locks across these waits |
| Apply | Advance the monotonic source-policy epoch. Mark formerly eligible paths ineligible in their existing stores; stop their validation, replay and GR/LLGR work from restoring eligibility. Re-elect alternatives and settle downstream replacement/withdrawal writes. Keep source TCP, negotiated capabilities, export-to-source routes and unrelated origins unchanged |
| Abort | After restoring the old configuration and filter definitions, restore eligibility and advertisement ownership from the pinned references, reconcile downstream state, advance the epoch again, then resume unread input through the restored policy. Do not require Route Refresh for this inverse |
| Accept | After the whole reload can no longer compensate, finalize the invalidation, release undo references and resume input through default reject. No denied announcements accumulated during an accepted default-reject period are retained for later policy recovery |

The registered source-invalidation contract has prepare, apply, abort and
finalize operations, carrying transaction ID, source identity, captured session
generation, policy epoch and receive frontier. Internal and external state
owners implement the same acknowledged contract through the existing callback
registry and protocol schema, not a switch over plugin names. Delivery follows
each owner's event FIFO and fences its asynchronous workers. An acknowledgement
identifies the exact operation and includes downstream settlement, not enqueue.
Observers receive a distinct policy-invalidation notification, not a false
session-state transition. The bridge implements any required lifecycle handling
on behalf of ExaBGP scripts; this must not require changing those scripts.

→ Constraint: operation commit is acknowledged by `OperationExecutor.Commit`,
but a later owner's failure still calls rollback in `runOperationPath`.
BGP's current operation-commit callback discards its journal, and the outer
`reloadAcceptance.finish` can compensate after that boundary. D2 undo ownership
must survive both boundaries until whole-reload acceptance. Extend that existing
acceptance path with a reliable bounded acknowledgement; a fire-and-forget
accepted event that merely logs delivery failure cannot release a paused peer.
This transaction integration is required by D2, not a separable future feature.

Pause ownership must compose with existing backpressure: releasing the reload's
pause must not release another pause owner. The pause deadline must be bounded
by the transaction deadline and the captured session's remaining HoldTimer
budget, reserving time for abort and resumption. Zero HoldTime still has the
transaction deadline. Never suspend BGP timers. Outgoing keepalives continue,
but incoming keepalives cannot bypass unread TCP data. Deadline exhaustion aborts
the transaction; real TCP loss, owner death or failed compensation is reported
as failure, never as successful rollback or a normal automatic restart.

### Recovery, retention and preservation

| Boundary | Contract |
|----------|----------|
| Absent to explicit accepting import | D-1: without negotiated Route Refresh refuse before changing policies or templates, even if optional storage exists. Otherwise publish against the captured session and send one request for each negotiated family; return partial-send failure to the transaction |
| Refresh completion | Successful request writes mean recovery was initiated, not that all routes arrived. Basic Route Refresh has no universal completion marker; tests assert eventual route recovery separately |
| Failed import relaxation | Restore rejection through a fresh epoch and invalidate candidate-admitted paths before reporting compensation complete. Late refresh responses cannot repopulate eligible state |
| Accepting import to absent | D-2 applies with or without Route Refresh. A removed child whose parent still supplies explicit policy is not an absent-policy transition |
| Optional receive store | Preserve RFC 7947 filtered entries when that feature is implemented. Import denial is independent of validation ineligibility; validation acceptance cannot clear it. No requirement that a pre-policy diagnostic store be empty |
| Mixed UPDATE | Retain explicit withdrawals and suppress denied announcements. Coordinate the RFC 7947 spec using the original rejected announcement population, not only the final boolean or withdrawal-only projection |
| Export restoration | Retained received sources, configured/static/default and plugin sources follow existing export reconciliation. One-shot API and ExaBGP injections require their producer to reannounce; no desired-state store is added |
| Arbitrary filter edits | Existing behavior remains; this spec does not add general re-evaluation of stored routes for arbitrary filter-definition changes |
| Dynamic peers | Resolve policy and negotiated session type coherently. Establishment and peer creation join the reload admission boundary; template generation alone does not fence receive work |

### Operational representation

Use the existing `PolicyInfo` representation, extended with optional
`final-source`. The only new source value is `rfc8212`; it explains a derived
default, not operator configuration. The CLI policy-chain projection must carry
the same field. Configured policies retain their existing representation.

| Policy and session | Reported direction |
|--------------------|--------------------|
| Explicit configured policy | Existing filters and configured `final`; omit `final-source` |
| Absent policy, known EBGP | Existing automatic filters, if any; `final` is `reject`, `final-source` is `rfc8212` |
| Absent policy, known IBGP | Preserve existing omission for an empty direction and existing automatic-only filter representation |
| Absent policy, unresolved dynamic AS | `final` is `undetermined`; omit `final-source`. Do not infer EBGP from remote AS zero |

Dry-run adds an optional `policy` using that same representation so its UPDATE
action stays distinct from the exhaustion verdict: a pure withdrawal can be
accepted under a default-reject policy. For unresolved session type it returns
action `undetermined`, no executed trace and no resulting route, explaining that
the peer must establish before the command can predict live behavior. It must
not call filters or claim an accepting/rejecting result from an unknown AS.
Known-session filter overrides retain the direction's effective final.

The establishment warning is once per absent direction per captured session,
after dynamic resolution. It names the peer, direction, RFC 8212 and blocked
route exchange. Its repair uses the concrete configured peer or group path:
`set bgp peer <name> filter import final accept` (or export), followed by
`commit now` in the configuration editor. Dynamic peers name their actual group,
not an invented static peer. Explicit accept is an example requiring operator
intent, not an automatic repair. D-1 refusal additionally prints the concrete
disable, commit, enable sequence already recorded above.

### Acceptance amendments and additional proof

| AC | Authoritative assertion | Concrete proof |
|----|-------------------------|----------------|
| AC-1 amendment | Denied EBGP announcements cannot become eligible, selected, redistributed or forwarded; diagnostic pre-policy retention is permitted | `test/plugin/rfc8212-ebgp-no-import-policy.ci` checks eligible RIB and downstream output, not absence of all retained data |
| AC-6 amendment | Import recovery uses negotiated refresh on the same session; without refresh the entire candidate remains unchanged. Export restoration observes the retained-source limits above | `test/reload/rfc8212-explicit-policy-reload.ci`; `test/reload/rfc8212-import-no-refresh.ci` |
| AC-8 amendment | Detail, chain and dry-run expose the table above; configured reject differs from default reject, and unresolved dynamic type never produces a fabricated route verdict | `test/plugin/rfc8212-policy-status.ci`, including explicit, absent, automatic-only and dynamic directions |
| AC-12 | Removing the last accepting import policy invalidates source routes and downstream advertisements without changing source session identity or export-to-source routes; alternatives and unrelated/local-origin paths survive | `test/reload/rfc8212-import-invalidate.ci`, with and without Route Refresh and optional storage |
| AC-13 | A later apply failure, later operation-commit failure or outer reload rejection restores policy, eligible state and downstream advertisements without refresh; input arriving during the tentative change is processed exactly once under the restored policy | `test/reload/rfc8212-import-rollback.ci`; `TestRFC8212ImportRollbackAcrossAcceptance` |
| AC-14 | Old ingress, queued/direct forwarding, cached replay, late validation, GR timers and sent projections cannot reinstall invalidated paths, including across dynamic establishment and a replacement session | `TestRFC8212ImportEpochFences`; the real-process invalidation fixture pauses work across the boundary |
| AC-15 | Internal and external lifecycle owners acknowledge actual invalidation; unsupported owners fail preflight; owner death, deadlines and failed finalization never report successful reload or leave a live session indefinitely paused | `test/reload/rfc8212-import-owner-failure.ci`; `TestRFC8212ImportAdmissionDeadline` |

Boundary coverage includes IPv4 legacy and MP IPv6 announcements, explicit
withdrawals, mixed UPDATEs and EOR; ADD-PATH ID zero and nonzero; selected
ordinary, labeled/VPN and FlowSpec paths through their existing owners.
Tests preserve protocol assertions and prove observable state, not copies of
new fields or callback wiring alone. RFC-tagged fixture edits require independent
judgment and renewed discrimination/audit evidence as already recorded.

### Added files and documentation surfaces

| Surface | Files and required work |
|---------|-------------------------|
| Receive pause and source epoch | Under `internal/component/bgp/reactor/`: `session_flow.go`, `session.go`, `reactor_notify.go`, `receive_publication.go`, `delivery.go`, `forward_pool.go`, `forward_rs.go`; session-bound admission receipt and final-writer checks |
| Reload and source ownership | Under `internal/component/bgp/reactor/`: `peer_settings_apply.go`, `reactor_dynamic.go`, `relay_recovery.go`, `rib_recovery.go`, `policy_reload.go`; static/dynamic prepare, reversible invalidation and session-bound refresh errors |
| Existing source-state owners | `internal/component/bgp/plugins/rib/`, `adj_rib_in/`, `rs/`, `rr/`, `gr/`; preserve/revoke the appropriate state rather than imitate DOWN |
| Transaction acceptance | `internal/component/bgp/plugin/register.go`, `internal/component/config/transaction/orchestrator.go`, `executor.go`, `internal/component/plugin/server/reload_tx.go`; preserve undo through outer acceptance and propagate bounded finalization receipts |
| Registered callback surface | `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/rpc/`, `internal/core/ipc/yang/ze-plugin-callback.yang` and `ze-plugin-engine.yang`; typed internal/external source-invalidation operations, `ze:method` declarations and registration-derived support |
| Status projection | `internal/component/plugin/types_bgp.go`, `internal/component/bgp/plugins/cmd/policy/handler.go`, reactor API and dry-run; `final-source` and dry-run policy context |
| Architecture pages | `docs/architecture/core-design.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/config/transaction-protocol.md`; document admission, undo lifetime and acknowledged completion beside the implementation phase |
| Checklist amendments | YANG/RPC, plugin implementation, SDK protocol, registered lifecycle callbacks and route eligibility metadata now apply. No new user config leaf, CLI verb, environment variable, host dependency, metric or BGP capability is proposed |

### Additional risks and assumptions

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-11 | Paused receive consumes HoldTimer budget | Admission/drain deadline approaches the remaining session budget | Bound prepare/apply/abort; retain normal timers; never promise immunity to independent transport failure |
| R-12 | Undo released at operation commit before outer acceptance | Later participant failure cannot restore imported paths | Keep transaction pins until acknowledged whole-reload acceptance; inject both later-commit and outer-rejection failures |
| R-13 | Old workers acquire the current epoch | Withdrawn routes return after a successful policy commit | Capture epoch at admission and preserve it through every forwarding/replay/validation path |
| R-14 | Import denial overwrites validation state or RFC 7947 retention | Validation reply reactivates a route or diagnostic entries disappear | Independent eligibility reasons; preserve filtered population and test mixed UPDATE overlap |
| R-15 | External owner acknowledges dispatch rather than completion | Reload returns before downstream withdrawal, or legacy scripts need new code | Registered completion contract, real external-process tests and bridge-owned compatibility handling |

| ID | Assumption | Basis | If wrong | Validation |
|----|------------|-------|----------|------------|
| A-9 | Existing pause and ownership mechanisms can be extended without a permanent store | `Session.Pause`, source-owned advertisements and current RIB references exist | If wrong, the proposed reversible design is incomplete | Admission/rollback fault tests with no refresh and no optional store; verify references are released after accept/abort |

### Design alternatives and triple challenge

| Question | Decision and trade-off |
|----------|------------------------|
| Runtime default versus config rejection | Runtime enforcement remains recommended. FRR's `bgp_update` uses `reason = "inbound policy missing"` and `subgroup_announce_check` returns false without outbound policy; BIRD's `bgp_postconfig` instead calls `cf_error("EBGP requires explicit import policy")` and the export equivalent. Runtime enforcement admits dynamic templates and establishes a diagnosable session without leaking routes |
| Reversible pause versus shadow processing | Reversible pause reuses bounded TCP backpressure and existing state references. Continuing old-policy shadow evaluation would also require stable old filter-plugin configurations and a second processing lifecycle; irreversible deletion cannot satisfy rollback without refresh |
| Simplicity | The session-aware final is small; D2 necessarily adds source invalidation and transaction lifetime work. Refusal/reset were simpler but the owner selected in-place invalidation. No generic route store or arbitrary filter re-evaluation is added |
| Uniformity | Existing registries, event application receipts, source ownership and transaction journals are reused. The new acknowledgement is needed because neither existing enqueue receipts nor commit notifications prove this operation completed |
| Performance | Final evaluation stays scalar and allocation-free. Epochs travel with existing ownership records. Transaction-only pins, receipts and rollback bookkeeping allocate on the cold reload path; retained wire/attribute data are referenced, not duplicated per route or UPDATE |

FRR source inspected at
https://github.com/FRRouting/frr/blob/master/bgpd/bgp_route.c
(`bgp_inbound_policy_exists`, `bgp_outbound_policy_exists`, `bgp_update`,
`subgroup_announce_check`); BIRD source inspected at
`~/Code/gitlab.nic.cz/labs/bird/proto/bgp/bgp.c`, `bgp_postconfig`.
These justify the runtime-versus-config design comparison, not an assertion
that either daemon already supplies Ze's selected reload transaction contract.

## Approved implementation contract, 2026-10-28

The owner approved the completed design proposal at the design gate. The
authoritative contract is that proposal, its AC amendments and AC-12 through
AC-15, together with this section. Earlier research and rejected alternatives
remain as history, not alternative implementation instructions. Status remains
`design` until the final save gate.

### Reading amendments

- [ ] `docs/guide/redistribution.md`
  → Constraint: preserve inherited policy presence and most-specific configured
  final; do not turn automatic filters into operator intent.
- [ ] `rfc/short/rfc4271.md` and the RFC's own Sections 9.1 and 9.1.3
  → Constraint: RFC 8212 changes EBGP eligibility and Adj-RIB-Out admission,
  not explicit withdrawal processing or the existence of pre-policy storage.
- [ ] `docs/architecture/config/transaction-protocol.md`
  → Constraint: use the existing config event stream and callback bridge.
  Operation commit precedes whole-reload acceptance; irreversible retirement
  belongs after durable candidate publication, not in the earlier journal discard.
- [ ] `docs/architecture/api/process-protocol.md`
  → Constraint: extend registered typed callback declarations and their existing
  transport; application receipts and downstream writer settlement are distinct.
- [ ] `docs/architecture/behavior/peer-lifecycle.md`
  → Constraint: source invalidation does not synthesize DOWN/UP, reset export
  availability, or activate GR retention.
- [ ] `docs/architecture/plugin/rib-storage-design.md`
  → Constraint: preserve existing state ownership and reference lifetimes;
  temporary undo pins do not create a second route store.
- [ ] `ai/patterns/registration.md`
  → Constraint: support and event routing derive from registrations, not a
  hand-maintained enumeration of participating plugins.

### Transaction boundary details

| Concern | Required contract |
|---------|-------------------|
| Trigger | Compare effective final decisions, not raw final enum values. D-1 governs established EBGP rejection-to-acceptance recovery; D-2's invalidation mechanism handles acceptance-to-rejection, including loss of explicit policy. A change that still inherits the same decision does not trigger a false transition |
| Participants | Capture state-owning receive consumers and their process lifetimes through registration. Ordinary observers are not required to implement route storage; state owners must declare support. Do not infer state ownership from a plugin's spelling |
| Ordering | The existing transaction stream orders prepare/apply/abort/finalize through its callback bridge. No second coordinator, bus or unsequenced side RPC is introduced. Keep FIFO event application live while waiting for downstream settlement |
| Request identity | Every operation carries transaction ID, source identity, peer incarnation, session generation, policy epoch, operation phase and the admitted receive frontier. Validate all fields before mutation; reject stale, foreign or replacement-session tokens |
| Duplicate delivery | Repeated delivery of the same phase/token returns its recorded outcome without repeating writes or releasing references twice. A stale phase cannot undo a later accepted transaction |
| Concurrency | Retain the short-lock order reactor then peer. Never wait for plugins, admission receipts or destination writes while holding either lock. The admission guard is released before callbacks that can re-enter the reactor |
| Deadline | Use the existing transaction budgets and live remaining HoldTimer budget. Reserve abort/resume time before pausing; refuse before mutation if no safe budget remains. Apply and rollback cannot extend the hold timer or leave the pause waiting for an unbounded callback |
| Successful apply | Every affected owner has excluded source paths, async work is fenced, alternatives are selected and destination writer barriers have completed. Keep all undo references while any later participant or outer publication can fail |
| Rejection before acceptance | Restore configuration/filter definitions, owner eligibility and downstream advertisements before resuming preserved input. Failed compensation stays an error with the affected peer and owner; never report that the previous state was restored when it was not |
| Whole-reload acceptance | The accepted decision is reached only after all rollback-capable work and durable publication have succeeded. Deliver acknowledged finalization to each captured owner and release only this transaction's pause and pins |
| Failure after the accepted decision | Do not pretend an irreversible accepted decision was rolled back. Report that the candidate is active but finalization failed, naming the owner and operator recovery command. Preserve fail-closed epoch fencing; bound resource cleanup and release the transaction's receive pause without enabling old source paths. A missing finalization receipt is not successful reload completion |
| Session/owner death | Never apply the captured operation to a successor. Ordinary lifecycle cleanup releases the dead owner's transaction references. Report real loss rather than simulate a successful no-reset transition |

The failure-after-acceptance row distinguishes cleanup failure from a
rollback-capable apply failure; it does not authorize partial successful
invalidation. Apply receipts already prove eligibility and downstream changes
before the accepted decision. No route recovery or RFC compliance claim follows
from a request being queued.

### Final integration checklist

This table supersedes the earlier Integration Checklist.

| Integration point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | `internal/core/ipc/yang/ze-plugin-callback.yang`, `internal/core/ipc/yang/ze-plugin-engine.yang`: registered source-invalidation and completion contract; no new operator policy leaf |
| YANG validation constraints | Yes | Typed identities, bounded strings, operation enumeration and generation/frontier integer constraints in those schemas |
| YANG custom validators | N-A | No operator-entered dynamic value or new completion source; runtime ownership/session validation belongs at the callback boundary |
| CLI commands/flags | Yes, output only | Existing policy handler and peer detail expose effective default; no new verb or flag |
| CLI grammar | N-A | Existing commands and config paths remain unchanged |
| Editor autocomplete | N-A | No new editable configuration leaf |
| Functional test for RPC/API | Yes | `test/plugin/rfc8212-policy-status.ci`, `test/reload/rfc8212-import-owner-failure.ci` exercise actual command and external callback paths |
| Pipe completeness | Yes, preservation | Existing policy command response pipeline must retain new fields through JSON output and normal filtering; assert in the status fixture |
| Environment registration | N-A | No new environment variable |
| Doctor check and diagnostic code | N-A | No new host dependency, socket, executable, file prerequisite or kernel feature; unsupported owner is a transaction preflight error |
| Prometheus counters/metrics | No | Preserve current rejection accounting; no new reason label, counter or cardinality is required |
| New BGP family/capability/attribute | N-A | Existing families and negotiated Route Refresh are used; no wire capability is added |

### Final documentation update checklist

Every row supersedes the earlier row with the same number. A phase updates
the page as it changes the producing behavior; the last phase checks those edits.

| # | Surface | Applies? | Files / reason |
|---|---------|----------|----------------|
| 1 | Feature scope and evidence | Yes | `features/bgp-protocol.md`; currently experimental. Record only implemented and exercised RFC 8212 behavior |
| 2 | Config semantics and examples | Yes | `docs/architecture/config/syntax.md`, `docs/guide/configuration.md`; absent EBGP policy rejects, inheritance unchanged |
| 3 | CLI output | Yes | `docs/guide/bgp-policy.md`, `docs/guide/command-reference.md`; effective default, undetermined state and dry-run policy context |
| 4 | API/RPC | Yes | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md`; operational fields and registered lifecycle boundary |
| 5 | Existing plugins | Yes | `docs/guide/plugins.md`, `docs/architecture/plugin/rib-storage-design.md`; source-state owner participation and rollback |
| 6 | User guides | Yes | `docs/guide/bgp-policy.md`, `docs/guide/bgp-peering.md`, `docs/guide/redistribution.md`, `docs/guide/config-reload.md`; D-1, D-2 and retained-source recovery limits |
| 7 | BGP wire format | No | No changed packet layout. Existing withdrawal and refresh wire formats remain authoritative; implementation comments cite their applicable RFC sections |
| 8 | SDK/protocol | Yes | `docs/architecture/api/process-protocol.md`; phase/token/receipt semantics and internal/external parity. Follow existing plugin rules rather than adding a duplicate rule |
| 9 | RFC support | Yes | `rfc/short/rfc8212.md`; remove mandatory gaps only after recorded proof. Generate `docs/features/rfc-status.md` from the ledger, never edit its derived row by hand |
| 10 | Test infrastructure | No | New scenarios use the existing harness; do not add a new test runner |
| 11 | Daemon comparison | Yes | `docs/comparison.md`; correct support scope only when implementation evidence warrants it |
| 12 | Internal architecture | Yes | Core-design, structural-forwarding, peer-lifecycle, rib-storage-design and `docs/architecture/config/transaction-protocol.md`, as named above |
| 13 | Route metadata | Yes, internal eligibility | `docs/architecture/plugin/rib-storage-design.md` documents import-denied versus validation eligibility and undo pins. No new public `meta/*` key is proposed |
| 14 | Metrics | No | No new metric or label; existing metric docs remain unchanged unless implementation changes their actual meaning |
| 15 | Registered lifecycle surface | Yes | `docs/architecture/api/process-protocol.md` and `docs/guide/plugins.md`; source-owner declarations and notifications. Regenerate schema-derived inventories affected by the new registered method |
| 16 | Source anchors | Yes | Run `./le spec citation anchors spec plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md`; every affected page carries the producing source anchor, and unrelated mentions are explicitly distinguished |
| 17 | Examples and migration | Yes | The guides above, `docs/exabgp/exabgp-migration.md`, `docs/exabgp/exabgp-differences.md`; add explicit policy where exchange is intended, not to examples or tests deliberately demonstrating default rejection |

### Discovery and drift prevention

| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md`: RFC 8212/default-reject keywords point to `docs/guide/bgp-policy.md` and `rfc/short/rfc8212.md`; config-transaction and process-protocol entries remain the lifecycle references |
| What prevents regression | Existing RFC-compliance, registration, documentation and testing rules; recorded discrimination for each mandatory requirement, plus AC-12 through AC-15 transition/fault tests |
| What inventory owns the fact | `filterapi.Policy` owns explicit presence; the captured session owns external/internal type; callback/schema registration owns participant support; RFC and feature registries own published support |
| What proves reachability | Named real-entry `.ci` fixtures, external-owner lifecycle coverage and the FRR scenario; a callback signature or unit helper alone proves nothing |

### Final implementation phases

This dependency-ordered table supersedes the earlier six-step implementation
list. Each phase has its own implementation agent and independent review under
the implementation skill; no implementation is authorized by this spec-writing
session alone.

| Phase | Inputs and files | Output and proof |
|-------|------------------|------------------|
| 1. Wiring | Existing BGP startup, reload, policy commands, migration and external callback entry points | Draft the named functional fixtures outside `test/` per the functional-test skill; connect actual paths and show the specific missing behavior goes red. Tests assert exchanged routes, policy state and session identity, not scaffolding |
| 2. Verdict | `filterapi/policy.go` and all exhaustion/fast-forward/pre-snapshot/dry-run consumers | Session-aware scalar verdict; explicit presence and IBGP preserved; absent EBGP import/export blocked from all route origins. Truth-table and no-policy fixtures pass; policy semantics docs updated |
| 3. Reversible lifecycle | Session admission, source-policy epochs, current source owners, callback schemas/SDK and existing transaction/outer-acceptance paths | Prepare/apply/abort/finalize works in process and externally; tokens remain valid only for their captured source/session; rollback works without refresh. AC-12 through AC-15 fault/epoch tests exercise actual owners; lifecycle and transaction docs updated |
| 4. Reload and operations | Peer settings/dynamic templates, captured-session refresh writes, export reconciliation, policy projections and establishment warnings | D-1 refuses unsupported relaxation before publication; D-2 invalidates in place; status/dry-run/warnings follow the approved table. Reload and status fixtures pass; guides/API fields updated |
| 5. Fixture compatibility | Existing EBGP route-exchange fixtures and RFC-tagged carriers affected by the new default | Add only their intended explicit policy, preserve assertions and purpose, obtain independent test judgment where required. Renew every stale discrimination unit and independent RFC audit judgment; do not merely reseal stale proofs |
| 6. ExaBGP migration | `internal/exabgp/migration/migrate.go`, serializer as required, existing migration goldens and bridge compatibility cases | Per-migrated-peer import/export accept and explanatory output; no global policy or bridge exemption. Existing 4.2/5.0/6.0 contracts remain valid; migration/differences pages updated |
| 7. Interop and proof closure | `test/interop/scenarios/bgp-rfc8212-default-reject-frr/`, mandatory RFC carriers and feature/support ledgers | Latest stable FRR with explicit peer policies proves both defaults, explicit-policy exchange and the selected live transition. Record forced-red/green proof through the RFC tool, scoped regression evidence and accurate generated support surfaces |

Additional new Go tests belong in
`internal/component/bgp/reactor/rfc8212_default_reject_test.go` and the existing
owner/transaction test files that exercise each real boundary. Do not create a
parallel mock-only lifecycle suite. Additional functional files are exactly
those named in the acceptance amendments. The interop directory above replaces
the draft's incorrect path directly under `test/interop/`.

Scoped verification includes affected filterapi/reactor, source-owner plugins,
plugin process/server/SDK, config transaction and migration packages, with their
actual feature tags, not just the draft's three-package list. Run the named
functional cases and affected existing cases, not every suite. The scenario
selector is `INTEROP_SCENARIO=bgp-rfc8212-default-reject-frr ./le test integration interop`.
Commit verification uses `./le verify changed`; no full verification or
performance run is authorized on this Linux host.

### Final interpretation of older assumptions

| Earlier row | Final disposition |
|-------------|-------------------|
| A-1 | A signature change forces ordinary callers to update; it does not prove cached forwarding, direct-write admission or replay is fenced. AC-14 supplies that proof |
| A-2 | Ingress rejection prevents eligible publication. It does not require deleting a future RFC 7947 pre-policy diagnostic population |
| A-3 | Runtime decisions use a coherent session/policy snapshot; status before dynamic establishment uses the explicit undetermined state |
| A-4 and the optional-switch design row | Explicit policy satisfies the mandatory rule; it is not implementation of the RFC's optional deviation switch. No switch is added, and the RFC ledger must not claim one |
| A-5 | Migration preserves ExaBGP policy intent per peer; compatibility is proved by running migrated configurations and their scripts, not by absence of a keyword search alone |
| A-6 | Fixture edits inside `.ci` stdin change the recorded behavior unit and can stale discrimination and audit records. Renew the applicable evidence; do not assume semantic equivalence keeps hashes valid |
| A-7 | Refresh recovery requires observed routes, not just successful request writes; AC-6 and its functional fixture are the proof |
| A-8 | Preserve the RFC 7947 overlap contract for mixed UPDATEs; no hard dependency on implementing that separate retention feature |
| A-9 | Reversible admission/state ownership is an implementation hypothesis with the fault tests specified above, not a claim of existing behavior |
| Blast Radius: revert | The old verdict-only reversal description is obsolete. This design reaches transaction and plugin lifecycle contracts; any rollback of implementation must follow git-safety and account for all changed contracts, never issue a destructive git command |

### Specification independence

| Check | Answer |
|-------|--------|
| Another session can find the authoritative design | Yes: this section identifies the approved proposal and explicit superseding AC/checklist/phase rows |
| Tests can be written without guessing intended behavior | Yes: AC-1 through AC-15 define exchange, eligibility, session identity, refusal, rollback, status and failure outcomes; named fixtures cover the real entry points |
| Implementation inputs, outputs and constraints are stated | Yes: phases name ownership boundaries, dependency order, receipts, undo lifetime and proof; existing registries and storage are the required mechanisms |
| User decisions preserved | Yes: D-1 refusal without refresh, D-2 in-place invalidation, and approval of reversible admission/whole-reload acceptance are recorded here |
| Strongest remaining implementation risk | Bounded pause and reliable acceptance across real external owners; A-9 and R-11 through R-15 require fault proof before implementation can close |

## Review Gate

Reserved for independent implementation review, not approval of this prose.
The current design template places this in the closure supplement; the spec
skill explicitly requires its empty record here. Do not duplicate it at closure.

| Field | Value |
|-------|-------|
| Artifact | |
| `./le spec review check` | |
| Rounds | |
| Reviewer lenses used | |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

### Final surface audit

→ Constraint: `cmd/ze/hub/main_reload.go`, `runReloadContext`, currently calls
the void `acceptReload(true)` after candidate promotion and returns nil.
Include this caller and standalone server reload entry points in Phase 3 so
finalization errors reach the operator and cannot be mistaken for successful
reload. The default outer timeout is not a substitute for a session's live
HoldTimer budget.

| Additional affected surface | Documentation obligation |
|-----------------------------|--------------------------|
| `cmd/ze/hub/main_reload.go` and reload acceptance callers | `docs/architecture/config/transaction-protocol.md`: separate accepted-with-finalization-error from pre-acceptance rollback |
| Undo reference lifetime and preserved receive buffers | `docs/architecture/memory/lifetime-contracts.md`: named retain/release boundaries; do not retain borrowed wire bytes after their owner can recycle them |
| Registered source-state ownership | `docs/architecture/plugin/plugin-system.md`: declaration-driven participation and no hardcoded plugin list |
| Bridge lifecycle compatibility | `docs/architecture/exabgp-bridge.md`: bridge-owned handling without new obligations on 4.2/5.0/6.0 scripts |
| Originated export admission | `docs/architecture/bgp/on-demand-origination.md`: absent-EBGP export default at the existing session gate |

The expanded citation check ran without a blocking missing declared page and
reported advisory source mentions. The table above captures additional affected
surfaces identified from those mentions. The implementation checks the final
changed symbols against the remaining advisory pages; a shared-file mention is
not itself evidence that the page's described behavior changes.

### Final save approval

The owner selected **Save as ready** on 2026-10-28. Design and final save gates
are approved. The spec is ready for implementation; no implementation,
runtime verification or RFC conformance proof is claimed by this session.

## Phase 1 current-tree audit, 2026-10-28

This is source inspection, not runtime proof. The metadata remains
`in-progress`, Phase `1/7`. The approved amendments remain valid; no finding
requires a new product decision. `git log --oneline -30` includes the approved
specification commit but no intervening RFC 8212 implementation. The named
RFC 8212 reactor and functional proof files were absent before this phase.
No feature behavior, public support claim, RFC tag or existing assertion changed.

| AC | Current implementation | Producer and remaining requirement |
|----|------------------------|------------------------------------|
| 1 | Partial | `reactor/filter_ordered.go::runIngressPolicyChain` and `reactor/reactor_notify.go::notifyMessageReceiver` reject before eligible publication, but `filterapi/policy.go::AcceptsAtEnd` accepts absence |
| 2 | Partial | `reactor/egress_inject_filter.go::exportFilterForBody` shares export filtering but accepts absent policy and a missing forwarding snapshot; ordered and fast-forward export also accept absence |
| 3 | Implemented, unverified | `config/redistribution.go::concatPolicies` retains inherited explicit finals; existing `config/policy_test.go` covers configured policies. New real-path regression still owed |
| 4 | Implemented, unverified | `Policy.AcceptsAtEnd` accepts absent IBGP today; session-aware cutover must preserve it |
| 5 | Partial | `filterapi/policy.go::InternalPolicy` preserves absence, but exhaustion still accepts it on EBGP |
| 6 | Partial | `reactor/peer_settings_apply.go::applyHotSwappableSettings` only reconciles export finals. Import refresh recovery and pre-publication refusal are missing; `peer_derived_bindings.go::requestRouteRefreshAll` logs rather than returns send failure |
| 7 | Partial | Existing export rejection reconciliation is reached through effective-final comparison, but removing accept currently still accepts |
| 8 | Partial | `reactor_api.go::policyInfo` omits absence; `plugins/cmd/policy/handler.go::toPolicyChain` projects only filters/final; `policy_dryrun.go::PolicyDryRun` has no policy context or unresolved-session result |
| 9 | Missing | No RFC 8212 establishment warning is produced by the current absent-policy decision; warning proof remains owed |
| 10 | Missing | `migration/migrate.go::MigrateFromExaBGP`, `migrateNeighbors`, `migrateSingleNeighbor` emit no explicit per-peer import/export policy |
| 11 | Missing | `rfc/short/rfc8212.md` correctly retains both mandatory gaps; no conformance claim or record added |
| 12 | Missing | `applyHotSwappableSettings` publishes import changes without source-state invalidation |
| 13 | Missing | `bgp/plugin/register.go` operation-commit callback discards journals; `config/transaction/orchestrator.go::runOperationPath` can still roll back after a later commit failure |
| 14 | Missing | `reactor/forward_pool.go::forwardSourceCurrent` fences generation, not import policy; `relay_recovery.go::adjOutOwnerCurrent` adds binding revocation but not the approved admission epoch |
| 15 | Missing | `reactor/session_flow.go::Pause` has no admission receipt; `plugin/server/reload_tx.go::reloadAcceptance.finish` and hub `runReloadContext` cannot return acceptance-finalization failures |

Counts: two implemented-but-unverified preservation requirements, six partial,
seven missing. These classifications do not mark any AC demonstrated.
Paths beginning `reactor/`, `config/` or `plugins/` above are under
`internal/component/bgp/`; migration is `internal/exabgp/migration/`.

### Assumption validation against the current tree

This table supersedes earlier status cells, without rewriting the historical
assumptions into claims that their proposed mechanisms already exist.

| ID | Status | Cheap validation performed and remaining proof |
|----|--------|-----------------------------------------------|
| A-1 | Confirmed, limited | Live `AcceptsAtEnd`/`Configured` census matches the declared consumers, including both `forward_rs` passes. Parent supplied successful session-tagged gopls references. Signature migration is not epoch/bypass proof |
| A-2 | Confirmed as amended; original withdrawal-conversion claim broken | Read `notifyMessageReceiver`, `runIngressPolicyChain`, `ingressPolicyWithdrawals`, `policyWithdrawals`: denial precedes cache/forward/delivery; only explicit withdrawals survive. No pre-policy-store emptiness requirement |
| A-3 | Confirmed for publication; runtime proof owed | `reactor_dynamic.go::publishDynamicResolution` writes AS/import/export under one lock and validates template generation. Dry-run currently takes separate accessors and has no unknown-AS guard; coherent consumer snapshot remains implementation work |
| A-4 | Confirmed as amended; original deviation interpretation broken | `concatPolicies` keeps explicit presence; RFC 8212 Section 3 makes the deviation switch MAY. Explicit accept is policy, not proof of an optional switch |
| A-5 | Source premise supported; compatibility unverified | ExaBGP `rib/incoming.py::update_cache` stores received routes and `rib/outgoing.py::updates` yields generated updates with no import/export-policy condition in those producers. Controlled RIB search found no policy construct. Migrated configurations/scripts still require the planned compatibility run |
| A-6 | Broken original freshness assumption, approved mitigation retained | `internal/le/rfc/tags.go::behaviorBytes`/`dropCarrierBlocks` retain stdin content but omit tmpfs bodies. Configuration edits cannot be assumed to preserve records or judgment |
| A-7 | Unvalidated recovery hypothesis; source prerequisite confirmed | `requestRouteRefreshAll` gates on negotiation and loops over negotiated families. It is neither session-captured nor error-returning. Eventual route recovery cannot be validated by source inspection |
| A-8 | Confirmed overlap boundary; joint runtime proof owed | RFC 7947 spec is still design; `notifyMessageReceiver` currently returns before retained-store delivery. Preserve future rejected-announcement population without making that separate feature a dependency |
| A-9 | Unvalidated implementation hypothesis; existing primitives confirmed | `Session.Pause`/`Resume` and `adjOutOwnerCurrent` exist but provide neither pause-owner composition nor reversible source eligibility. Fault, deadline, reference-lifetime and outer-acceptance tests must establish extensibility |
| A-10 | Broken premise; prerequisite repaired and verified | Registered ownership now binds the named entry to `LoopIngress`; the RPC chain skips that in-process reference. Scoped binding, inactive, dynamic, dry-run and export-refusal tests pass. The native import draft receives the accepted IPv4 and IPv6 frames before reporting the intended forbidden absent-policy announcement |
| A-11 | Broken premise; prerequisite repaired and verified | `registerSubscriptions` now retains every selector. Focused delivery race tests pass; unchanged native export reaches accepted receipts for all five origins, and the observer's undeclared-event warning is gone |

### Wrong assumptions already resolved by the approved design

| Assumption | Current producer evidence | Approved correction |
|------------|---------------------------|---------------------|
| Denial generates withdrawals for old routes | `policyWithdrawals` retains only explicit withdrawal bytes | D2 reversible source invalidation, not a scalar-verdict-only change |
| Explicit accept implements the optional deviation switch | `concatPolicies` preserves configured policy; RFC 8212 Section 3 conditions the mandatory rule on its absence | No switch and no optional-switch conformance claim |
| Equivalent fixture setup preserves RFC evidence | `behaviorBytes` hashes stdin; tmpfs omission does not establish semantic equivalence | Renew each affected cover and obtain independent judgment |

### Phase 1 artifact boundary

Added untagged tests use existing compiled entry points:
`TestMigrateWritesExplicitRFC8212Policies` drives the public parser/migrator,
and `TestRFC8212PolicyDryRunWiring` drives `PolicyDryRunner.PolicyDryRun` and
its serialized response. Native runs now exist for all nine functional drafts
and the three unit probes. Dry-run, migration and the pre-snapshot probe reach
missing product semantics. The status draft reaches the absent-final mismatch.
The import draft now proves its accepted IPv4/IPv6 control before the intended
absent-policy failure, and the IBGP control passes. Reload drafts reach missing
default rejection, relaxation preflight or downstream invalidation. Their
later rollback/owner-fault boundaries remain gated on that invalidation, not
proven. Export now reaches all five accepted-origin receipts and its native
negative check catches the forbidden static announcement. Nothing is promoted.

Future conformance covers remain RFC8212-3-1 positive (explicit-policy eligible
import) and negative (absent-policy EBGP exclusion), and RFC8212-3-2 positive
(explicit-policy export) and negative (absent-policy suppression across origins).
Dry-run and migration alone do not prove those wire/eligibility requirements.

### Phase 1 mistake log: named automatic-filter prerequisite

The assumption that a configured internal loop entry already works is broken,
not the approved absence rule. The existing journal occurrence is owned above.
Removing `AUTO` would lose the nonempty automatic-policy regression boundary.

| Diagnosis | Evidence and decision |
|-----------|-----------------------|
| Symptom | The import draft's explicit-policy positive control receives no routes; the daemon reports `unknown plugin "AUTO"` |
| Root cause | `config/peers.go::prependDefaultFilters` inserts a bare reference after canonicalization. `reactor/filter/register.go` registers `LoopIngress` in process, while `reactor/filter_chain.go::PolicyFilterChain` treats the reference as an external process |
| Owning layer | Configured filter identity and reactor dispatch, not the fixture's route assertions or external-plugin registration |
| `[workaround]` | Remove the named internal reference, add an explicit policy to the absent direction, or provide a fake plugin |
| `[source]` | Preserve the automatic reference and absent final, resolve its registered execution ownership, and keep loop processing at its existing stage without an external RPC |
| Why not the workaround | It hides the defect or removes the distinction AC-5 must prove. The prerequisite repair must preserve active/inactive configuration, loop settings, dynamic-peer parity and fail-closed external filters |

This repair precedes the scalar-verdict phase. It adds no new protocol feature
and changes no approved acceptance criterion. Its tests, documentation and
native positive-control proof are required before the wiring gate can pass.

### Named automatic-filter prerequisite proof

The repair now has scoped race evidence in filterapi, config and reactor,
including real registered-hook execution, inactive/dynamic preservation,
whole-policy export refusal before terminal callbacks, and original-scope
editor remediation. The actual raw-rewrite probe changed from one external
call and acceptance to rejection with zero calls. Native config validation now
prints the written `AUTO` reference and executable deactivation command.

In `rfc8212-ebgp-no-import-policy`, the absent source received both accepted
control frames before sending its subject: 51-byte IPv4 and 78-byte MP IPv6.
The observer then reported forbidden `10.82.1.0/24` at the sink. This is the
intended missing default-reject behavior, not the earlier unknown-plugin
prerequisite failure. The IBGP preservation control also passed.

The canonical per-spec state records commands, logs and independent reviews.
The named automatic-filter journal occurrence is fixed. The subsequent
subscription prerequisite and final wiring classification follow below.

### Phase 1 mistake log: multiple subscription peers

| Diagnosis | Evidence and decision |
|-----------|-----------------------|
| Symptom | The accepted export peer receives the exact static UPDATE and sends its receipt, but the observer's route set stays empty |
| Root cause | `plugin/server/dispatch.go::registerSubscriptions` constructs a filter from `input.Peers[0]` and ignores the remaining selectors |
| Owning layer | Shared startup/runtime subscription registration, not native-peer sequencing or policy verdicts |
| `[workaround]` | Subscribe the fixture to every peer or omit the accepted receipt |
| `[source]` | Register every supplied selector using the existing subscription representation and process deduplication, retaining configured grant intersection |
| Why not the workaround | It hides a public API defect and leaves the exact multi-peer subscription contract broken |

The saved native capture also shows the absent-policy destination receiving the
forbidden static announcement. That is the intended export rejection failure;
it does not explain why the independent accepted receipt was lost. This
prerequisite changes no RFC 8212 acceptance criterion.

### Wiring handoff to Phase 2

Both prerequisites are repaired, independently reviewed and exercised through
the native entry points. The subscription repair also passes focused race tests
in plugin/server and BGP/server. In the unchanged export draft, the accepted
peer receives and acknowledges static, forwarded, API, ExaBGP bridge and
redistribution announcements; the observer reaches the quiescence request after
those receipts. The absent-policy peer receives the forbidden static route and
its native reject fires. Quiescence then fails; the final withdrawal fences and
session assertions have not passed.

The wiring gate has its intended failing baselines: import/export absence,
pre-snapshot admission, status/dry-run, migration, relaxation preflight and
downstream invalidation. The IBGP control passes. Rollback and owner-fault
injections remain behind the missing invalidation, as the approved dependency
order requires; no lifecycle success is claimed. Phase 2 now implements the
session-aware scalar verdict. Later phases still owe every remaining AC,
fixture migration and RFC proof.

## Implementation checkpoint

Update this section at every phase boundary, including an unfinished package's
handoff. Record what was implemented, what actually ran, unresolved failures
and the next action before continuing. The specification, not only transient
session context, must identify where implementation resumes.

Current phase: **4/7, reload and operations**. No whole-spec success, closure or
RFC conformance claim is made. The approved acceptance criteria remain intact.

Checkpoint 2026-10-30. Phase 3 (D-2, the reversible removal of an Established
EBGP session's last accepting import policy) is implemented and landed with
AC-8 in one commit. The detailed per-package record is the session state file
`tmp/session/2026-10-29-3514eb97-23b7-4530-b40e-d986b1aecd02/state/session-state-spec-bgp-rfc8212-ebgp-default-reject-3514eb97-23b7-4530-b40e-d986b1aecd02.md`,
whose handoffs supersede the 2026-10-28 handover for Phase 3.

What Phase 3 delivered:

- The reload coordinator (`reactor/source_policy_reload.go`): a falling import
  transition pauses the session's input, captures the source-policy owners
  (`bgp-rib`, `bgp-adj-rib-in`, `bgp-rs`, `bgp-rr`) in dependency order,
  prepares and applies the invalidation under a fresh policy epoch, settles
  downstream writers, and on a rejected transaction aborts and restores the
  routes before input resumes. The section path refuses the transition.
- Policy-epoch fences on Adj-RIB-Out, forwarding, released receipts and sent
  owners, so stale work under an old epoch is never reported written.
- The ledger (`core/bgp/sourcepolicy`): a Prepared ledger of an abandoned
  transaction is replaced, an Applied one holds.
- Operation-based reload: the bgp decomposer's `reconcile-peers` residual, one
  candidate resolution per transaction, operation commit refusal on a spent
  admission budget, and `config-complete` finalize or settle.
- `ApplyConfigDiff` loads the tree it is handed, so compensation restores the
  prior tree; a created peer keeps its YANG defaults across a restore.
- Reload functional tests `test/reload/rfc8212-import-invalidate`,
  `-invalidate-refresh`, `-inherited`, `-rollback` and `-owner-failure`, each
  shown red under a broken owner apply or abort, and 80 of 80 under stress.

AC-8 is done: peer detail, `show policy chain` and policy dry-run report the
effective final and mark the RFC 8212 default (`final-source rfc8212`), with
`test/plugin/rfc8212-policy-status.ci` red under two producer breaks.

Known reds at this checkpoint: 75 top-level failures across two full reactor
package runs, triaged 2026-10-30 but not fixed. The run stops at the 10-minute
package timeout because `TestRFC8669UnknownTLVPropagatedAcrossANextHopChange`
hangs on the route-server rail.

| Class | Count | Cause and resume instruction |
|-------|-------|------------------------------|
| A, RFC 8212 fixture debt | 72 | All pass with an overlay that makes an absent policy accept. Give each caller's `PeerSettings` explicit accepting import and export policies, never the shared router helpers (`lowLiveRouter`, `rsLiveRouter`, `addPathLiveRouter`, `announcementLiveRouter`, `newOwedRail`, `newSessionEndCase`, `collapseDestPeer`): `policy_final_test.go` and `rfc8212_released_two_source_test.go` rely on their absent policy. A test that asserts absent-means-accept is rewritten to the RFC 8212 Section 3 reject. Every change goes to `ze-test-judge` |
| C, another session | 1 | `TestRefreshPeerLinkScopesSkipsAnUnchangedTable`, from the neighbor-view skip check added in 8b062b45a8 (`reactor_iface.go`). Owed: one journal row owned by the link-local spec; not fixed here |
| D, cause untraced | 2 | `TestPeerReadyRRReplayErrorCompletion` (the live announcement is never written) and `TestBFDNegotiatedPublicationJoinsSessionEnd` (the read loop never ends while publication is paused). Both stay red under the absent-accept overlay, and their files were last changed by this spec (fa6552968f, ff2aac12b0, 30ff1fce91), so treat them as probable defects of this spec: bisect with overlays and fix the producer |

Paused 2026-10-30: the filter-plugin work is moving the policy types
(`policy.Ref`, `policy.Final` replacing the `filterapi` ones in `mustPolicy` and
the fixtures), so class A resumes on its new types once it lands. Uncommitted
triage edits left in the tree, to re-check against the new types before they
land: `filter_ordered_test.go` and `egress_inject_filter_test.go` (expectations
rewritten to the default reject), `rfc4271_degree_of_preference_test.go`
`receiveExternalRoute` (RFC-tagged RFC4271-5.1.5-5, edited outside the edit
hook: owes `./le rfc approve` and a fresh `./le rfc record`), and the shared
fixtures `wkPeer`, `newAIGPReplayFixture`, `prefixSIDPeer` and
`llnhExternalPeer`. After class A: the full reactor run with `-race -timeout
20m`, then the scoped lint.

Checkpoint 2026-10-30, late (continuation agent):

- Landed: f8f7628bdf, the 30 judge-approved RFC-tagged plugin fixtures (30 of
  31 pass; paths-limit-live is red for an unrelated reason, journal row in
  `fixture-encodes-an-impossible-state` owned by
  `spec-config-reference-completion`, landed with it). 5ce4b7b043: a peer
  `create bgp peer` adds is built through the TreeReloadFunc over the running
  configuration, so it inherits the bgp-level policy
  (`TestCreatedPeerInheritsTheBGPLevelPolicy`).
- Owed on those fixtures: `./le rfc check` reports 22 stale discrimination
  records and the matching stale audit verdicts for the changed fixtures (list
  in the session scratch `stale-discrim-30.txt`): re-record each with
  `./le rfc record`, re-audit with `ze-rfc-audit`.
- Red: `api-peer-create-delete-rib` now passes create, RIB, forward, commit and
  reload, and fails at delete: `show bgp rib` still answers 192.0.2.0/24 after
  `delete bgp peer`. Cause not traced.
- Class A and D not started in this pass; the uncommitted triage edits stand
  as described above (the `filter_ordered_test.go` one changes an expectation
  and goes to the judge).

Checkpoint 2026-10-30, night (reactor class A/D agent):

- Landed: d56664ed56 (judge-approved `TestRunEgressPolicyChainNoExportFiltersAccepts`;
  its name no longer fits the EBGP case, and a rename owes a fresh judgement)
  and bebc0d9cf2 (non-RFC reactor fixtures state accepting policies at their
  callers; `newSessionEndCase` and `addPathLiveRouter` keep the absent
  policy). `./le repo compiles check` OK.
- Full reactor package, `-race -timeout 20m`, over the working tree with the
  uncommitted RFC-file fixtures, skipping the hanging
  `TestRFC8669UnknownTLVPropagatedAcrossANextHopChange`: 2 failures, no data
  race (session `5f56b642` scratch `rxrace.log`). Class A is otherwise green.
- Uncommitted, waiting for `ze-test-judge`, then `./le rfc approve` and
  `./le rfc record`: the RFC-tagged files' fixture edits listed in the session
  state handoff of this date.
- Red, `TestBFDNegotiatedPublicationJoinsSessionEnd`: traced, not this spec.
  `Session.handleBFDEvent` holds `openMu` across `s.fsm.Event`, whose
  OpenConfirm callback publishes `announceNegotiated`; the read goroutine's
  `handleConnectionClose` waits on `openMu`, so the read loop cannot end while
  the publication is paused. The locking came with c987777bb8 (strict BFD
  DelayOpen). The fix is owed in the session lock design.
- Red, `TestOwedKeyGRPurgeRecoversSurvivor` (release/no-survivor): green
  alone, red under package load (withdrawal history 0, want 1). Untraced.
- `TestPeerReadyRRReplayErrorCompletion` passed in the race run.
- Scoped lint owed: `./le go lint run scope ./internal/component/bgp/reactor/...`
  refused twice to plan because other sessions' edits changed its inputs.

What remains:

- Phase 4: D-1 (import relaxation when Route Refresh is unavailable), AC-6 and
  AC-7 end to end on a real reload, static and dynamic boundaries, and the
  AC-9 establishment warnings.
- Phase 5: the reactor reds in the table above and the rest of the EBGP fixture debt,
  with independent judgment of each test change, and renewed RFC
  discrimination and audit records.
- Phase 6: the ExaBGP compatibility runs listed in its row.
- Phase 7: the FRR interop scenario, AC-11 tagged tests with discrimination
  records, removal of the `{gap}` annotations, and closure through `/ze-close`.
- Gates owed for the landed work: a scoped lint of reactor, cmd/policy, rs,
  bgp/config and bgp/plugin once the tree builds for them.

| Phase | Status | Observed evidence and remaining work |
|-------|--------|--------------------------------------|
| 1. Wiring | Finished | Real-entry drafts reached the intended missing verdict, reload and status behavior after the documented automatic-filter and subscription prerequisites were repaired. Rollback/owner-fault fixtures still need the lifecycle before their later fault assertions can run |
| 2. Verdict | Implemented and scoped verification passed | Session-aware exhaustion, coherent session snapshots and raw injection admission are implemented. Focused race tests, native import/IBGP drafts and the seven-origin export draft passed. Raw body, full-packet and concatenated-control bypasses were observed red before repair. Fourteen documentation configurations passed native validation. RFC proof records, draft promotion and broader affected-fixture checks remain in Phases5/7 |
| 3. Reversible lifecycle | Implemented 2026-10-30 | Owner invalidation, undo pins, downstream settlement and whole-reload coordination landed with the D-2 reload functional tests (see the checkpoint above). The 75 triaged reactor reds move to Phase 5 |
| 4. Reload and operations | In progress | D-2 integration and the status projections (AC-8) done. D-1 preflight/refresh, AC-6 and AC-7 end to end, complete static/dynamic boundaries and establishment warnings (AC-9) remain owed |
| 5. Fixture compatibility | In progress | Non-RFC-tagged fixtures that CI run 41071626530 failed carry a bgp-level import and export final accept, with no assertion changed: encode `9320450180`, plugin `ca21d0b884`, `e6bcdec054`, `50060db06e`, reload `d55cad2829`, interop `747d82e911` (unrun: needs the Ze-kernel lab), UI `cliWireConfig` `368d56666c`. Remaining: 32 RFC-tagged plugin fixtures edited but uncommitted, waiting for `ze-test-judge` and `./le rfc record` (the 31 of session 5f56b642's `scratch/plugin-rfc.txt`, plus `med-locally-set-reaches-peer`). `api-peer-create-delete-rib`: a created peer got no bgp-level policy; an uncommitted change building it through the reload pipeline (`createdPeerFromRunning`, `reactor_peers.go`) is in progress in another session. The UI send tests and `paths-limit-live` stay red because a Go fixture names `profile admin` and configures no `system authorization`. `bgp-rfc2545-third-party-linklocal-frr/ze.conf` sets export only and carries another session's hunk. The reactor class A reds and the renewed RFC discrimination/audit records remain owed |
| 6. ExaBGP migration | Implemented; compatibility verification incomplete | Migration package race tests, revised serialized-policy unit and native migration/validation draft passed. Actual CLI smoke verified each peer's explanatory warning. Six golden changes received independent judgment. Old-4.2, old-5.0 and current scripts plus the external-peer wire control and affected compatibility regression remain owed |
| 7. Interop and proof closure | Not started | FRR scenario, forced-red records, support ledgers and final scoped regression evidence remain owed. Closure stays a separate `/ze-close` phase |

### Phase 3 current packages

| Package | State | Resume instruction |
|---------|-------|--------------------|
| Admission and timer budget | Implemented; focused race tests passed | Preserve receipt ownership, work versus hard completion deadlines and terminal-NOTIFICATION ordering when connecting the coordinator |
| Acceptance and compensation | Committed `a99524fc56`, reviewed, both arrangements pass | `reload_recovery_schedule.go` places each removed owner between the retained inverses it depends on and those depending on it (`selectedPluginGraph`), with the solver's addressing-before-binder position only for undeclared participants. Refused before execution: a cycle through a removed owner, or bounds the fixed reverse undo order cannot satisfy (message names the plugins and the restart). A retained-only cycle no longer blocks. One rollback deadline spans context-bound recovery startup and `transaction.RollbackDriver` dispatch; a sent inverse is consumed, an unsent one stays owed. Independent review findings fixed with red-before evidence; 14 targeted tests pass on both transports (`job-compensation-fix-7cb4943d.log`). Judged fixture changes applied (removed-infrastructure `StartAfter`, coarse-restoration decomposer scoped to its own root). The completion gate is now held through recovery startups (documented). Not yet tied to source-policy pins |
| Registered lifecycle transport | Normal transport controls passed; effective deadline repaired 2026-10-29 | `PluginConn.SendSourcePolicy` now transmits the derived context's deadline (the earlier of caller and request). `TestSourcePolicyOwnerEarlierCallerDeadline` passed on both transports after the repair (session `3514eb97` scratch, `job-rfc8212-source-deadline-after-d8817e4b.log`, on the Mac). Independent review found no issue; both protocol pages now state whose deadline is sent. Actual owner invalidation remains separate |
| Live policy-epoch provenance | Final gates implemented 2026-10-29; the three stale-work regressions pass | `sourcePolicyCurrent` gates forwarded, initial and recovery announcements and expected-owner reannouncements; every withdrawal and local origin stays admitted. Stale initial work stays owed (`errAdjOutPolicy`), never reported written. Recovery skips a policy-stale survivor like an export-refused one (next eligible candidate, else withdraw the departed route, RFC 4271 Section 9.2); PATHS-LIMIT drops stale survivors before election so departure and demotion withdrawals still go out; a key whose epoch moves after staging holds back only the stale section (`recoveryKeyPolicyStale`) and wakes the worker. Departure and old-DOWN withdrawal guards use the session-only check, so policy staleness is never a departure. Raw text UPDATE grammar restored (no message ID, no epoch); the empty text UPDATE carries none. Review findings fixed; four new recovery tests (three shown red first, `job-c2-fix-red-a2af2b00.log`, `job-c2-fix-red2-9a2df6a1.log`) and the three original units pass (`job-c2-fix-green-39516f69.log`). Three judged fixture sets applied (13-file, 17-file, peer-ready). Committed `fa6552968f`. Full reactor race run after the commit: 118 failures (`job-rfc8212-reactor-package-2-55a5e1d5.log`), every one also failing before today's repairs (74 on HEAD `4179d2b130`+, `job-rfc8212-reactor-head-baseline-be97ffd2.log`; the other 44 on `7df97ab2f2`, `job-rfc8212-new44-baseline-*.log`), so no regression. They are route-server fast-path refusals, causal-marker timeouts, policy dry-run wiring and MRT collision timeouts: Phase 5 fixture debt to classify, with `TestRFC8212PolicyDryRunWiring` (missing effective policy context) owned by this spec's Phase 4 |
| Stored policy-epoch provenance | New replay test passed; two independent source reviews found no issue in their assigned scope | `TestInitialReplayUsesReceivedPolicyEpoch` passed all eight cases; transport-format controls also passed. Storage and replay/dispatch reviews exclude pending final gates and lifecycle work. The new equality test still has no observed pre-fix discrimination; do not claim one. Full affected package checks, eligibility and undo pins remain owed |
| Owner invalidation and coordinator | Not implemented | After the transport packages, implement independent import eligibility, accepted-reference pins, async-worker/downstream settlement and whole-reload bounded abort/finalize. No optional-store prerequisite, synthetic DOWN or permanent denied-route store |

The four earlier acceptance review defects have targeted passing after-runs:
inner infrastructure-before-service undo, retained-root reconstruction,
mandatory completion decision decoding, and process/connection replacement
during completion. Mixed removed/retained-owner dependency ordering remains
open: both opposing arrangements have observed failing regressions. The third
dependency-ordered source repair is not yet written. Two independent stored
provenance reviews found no issue within their assigned scope, not across the
unfinished lifecycle.
Admission tests and these prerequisite tests do **not** prove AC-12 through
AC-15 end to end.

### Evidence and recovery pointers

Canonical detailed handoff:
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/state/session-state-bgp-rfc8212-ebgp-default-reject-644a4e78-0122-4eeb-8642-eb50638a1641.md`.
It records file/symbol maps, approved test changes, exact command selectors and
package contracts. Its older reports are chronological; later verification and
repair entries supersede earlier claims of work not yet run.

The committed continuation handover above is the durable restart summary.
The detailed state ends with explicit paused-agent handoffs and preserves
producer maps and the remaining gate locations. Keep the session's scratch
evidence and approval ledgers until implementation and closure consume them.

The following logs are under that session's `scratch/` directory:

| Evidence | Log |
|----------|-----|
| Verdict focused race run; two fixture corrections documented in the handoff | `job-rfc8212-phase2-verdict-after-prepend-9acb1dbe.log` |
| Raw injection repair and existing raw-path controls passed | `job-rfc8212-phase2-raw-after-77061f90.log` |
| Admission, residual HoldTimer and terminal controls passed | `job-rfc8212-phase3-admission-4f37a0e7.log` |
| Original journal acceptance cases passed on both transports | `job-rfc8212-acceptance-fixture-corrected-ddecf303.log` |
| Actual hub publication failure and accepted-finalization error cases passed | `job-rfc8212-hub-fixture-corrected-b13c6a7e.log` |
| Inner compensation dependency repair passed | `job-rfc8212-inner-rollback-after-0188b3f4.log` |
| Mandatory request/receipt decisions passed | `job-rfc8212-completion-request-after-91120df1.log`; `job-rfc8212-completion-receipt-after-a4e5b87e.log` |
| Mixed retained-owner reconstruction passed | `job-rfc8212-mixed-rollback-after-c5e04bf8.log` |
| Captured completion lifetime red, then passing repair | `job-rfc8212-completion-lifetime-before-413649b0.log`; `job-rfc8212-completion-lifetime-after-413649b0.log` |
| Current removed-infrastructure compensation failure | `job-rfc8212-removed-infrastructure-before-24532dbd.log` |
| Opposing removed-service compensation failure on both transports | `job-rfc8212-removed-service-before-4a4d2804.log` |
| Effective owner deadline failure on both transports | `job-rfc8212-source-deadline-before-d8817e4b.log` |
| Actual stale-policy writer/replay failures; reconnect control passed | `job-rfc8212-source-epoch-before-79abcafe.log` |
| Normal lifecycle transport, required fields/receipts, retained replay and format controls passed | `job-rfc8212-source-transport-controls-a2e07e28.log` |
| Migration package and revised policy unit passed | `job-rfc8212-phase6-migration-20335d73.log`; `job-rfc8212-migration-policy-after-1e0f3c65.log` |
| Actual per-peer explanatory warning smoke passed | `job-rfc8212-migration-warning-smoke-dddf9ca4.log` |

Resume Phase3 from the current packages above; do not repeat already observed
passing checks merely to confirm them. Run each failed regression after its
producer repair, then the scoped targets reached by the coherent changes.
No full verification or performance run is authorized on this Linux host.
