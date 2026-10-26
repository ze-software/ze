# Spec: bgp-ls-receiver-accept-and-limit-controls

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Required receiver controls and bounded storage belong to the shipped BGP-LS capability. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the RFC asks for.** RFC 9552 Section 8.2.3 states three receiver-side
configuration obligations, of which two bind a BGP-LS receiver. The first:
"An implementation SHOULD allow the operator to specify neighbors to which
Link-State NLRIs will be advertised and from which Link-State NLRIs will be
accepted." The second: "An implementation SHOULD allow the operator to specify
the maximum number of Link-State NLRIs stored in a router's Routing Information
Base (RIB)." `rfc/short/rfc9552.md` carries them as `RFC9552-8.2.3-1` and
`RFC9552-8.2.3-3`, both SHOULD-level, both unannotated and untested today.

**What ze does instead.** Ze has no per-neighbor BGP-LS accept list and no
Link-State RIB ceiling. An operator who wants to refuse BGP-LS from one peer
writes a `bgp/policy/family-filter` instance naming the `bgp-ls` family with
action `remove` and references it from that peer's import chain
(`parseFamilyFilters`, `internal/component/bgp/plugins/filter_family/config.go`;
`handleFilterUpdate`, `handler.go`). That is a hand-written all-or-nothing drop,
not the accept list Section 8.2.3 describes. The declared BGP-LS role that
`plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` designs is also
all-or-nothing: it is what Section 8.2.6's MUST asks for, and it is less than
Section 8.2.3 suggests. That spec is at Status `design`, so the role does not
exist in the tree yet either.

**Why the row waited, and why the wait is over.** The deferral this spec
replaces (`plan/deferrals/bgp-ls-receiver-fault-management.md`, 2026-08-26) made
the RIB ceiling wait on `RFC9552-8.2.6-1`, whose `{gap}` said the bgp-ls prefix
maximum was compared against a count that `countPrefixEntries` derived from a
CIDR walk that never parses a type-length Link-State NLRI. That defect is
settled. `forEachPrefixEntry`
(`internal/component/bgp/reactor/session_prefix.go`) now asks
`nlrisplit.Get(familyFromKey(fk))` first and walks the section with the
per-family splitter, so a bgp-ls section is counted by its own framing, and the
`RFC9552-8.2.6-1` row of `rfc/short/rfc9552.md` carries no `{gap}` today. The
precondition the row named is met, so the two SHOULDs can be scoped.

**What the work is.** Two operator-facing controls, each its own config surface:

1. **The accept list.** Which neighbors ze accepts Link-State NLRIs from,
   declared per peer or per group, so an operator names the accepted set rather
   than writing a filter for every peer that must be refused. The design has to
   decide how this relates to the declared BGP-LS role the fault-management
   spec adds, because both govern import of one family and two mechanisms that
   answer the same question are one mechanism too many (`ai/rules/simplicity.md`).
2. **The RIB ceiling.** A maximum number of Link-State NLRIs held in the RIB.
   The per-peer, per-family prefix maximum ze already enforces
   (`applyPrefixCheck` and `applyInstalledPrefixSections`,
   `internal/component/bgp/reactor/session_prefix.go`) is counted at message
   time against one session. Section 8.2.3 asks about what the RIB stores,
   across peers, which is a different number with a different owner.

**Not in scope.** Section 8.2.3's producer-side SHOULDs (the advertisement rate
limit, the abstracted topologies, and the 4096-byte BGP-LS UPDATE size limit)
have no producer to limit, because ze originates no BGP-LS. They belong to
`plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md` Phase 2,
where the shard's second row already sends them.

**Research corrections, 2026-10-13.** Four statements above were true when the
skeleton was written and are not true at HEAD:

| Skeleton statement | At HEAD | Evidence |
|--------------------|---------|----------|
| "ze originates no BGP-LS" | ze originates BGP-LS (`bgp-ls-export`, from native IS-IS and OSPF state, and `bgp-epe`) and relays received BGP-LS (`bgp-rs` opaque relay, `bgp-rib` opaque storage). The "advertised to" half of `RFC9552-8.2.3-1` therefore has a producer. The other producer-side SHOULDs (8.2.3-2, -4, -7) stay out of scope, unchanged | `docs/architecture/wire/nlri-bgpls.md` "Consumer Decoding and Origination"; `internal/component/bgp/plugins/ls_export/`; `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` |
| "Ze has no per-neighbor BGP-LS accept list" | A peer whose config carries a `family` block negotiates BGP-LS only when that block names `bgp-ls/bgp-ls` (RFC 4760 capability intersection). An UPDATE of a family the session did not negotiate is refused: NOTIFICATION UPDATE Message Error / Optional Attribute Error, or a silent drop when the family is `mode ignore`. The per-peer family list is therefore already an allow-list over both directions, untested and untagged as such. The exception is a peer with NO `family` block, which offers every plugin decode family, BGP-LS included | `validateUpdateFamilies` (`internal/component/bgp/reactor/session_validation.go`) and its caller in `session_read.go`; the KNOWN DEFECT comment in `session_negotiate.go` above the `pluginFamiliesGetter` fallback; journal row 2026-08-16 in `plan/journal/silent-fall-through.md` |
| The fault-management sibling is unbuilt | Its validator half is in the tree: `validateBGPLSNLRISyntax` and `RetainWellFormedNLRI` (`internal/component/bgp/message/rfc7606_bgpls_nlri.go`), documented in `docs/architecture/wire/nlri-bgpls.md` "Receive-Path NLRI Validation". Its declared Consumer-facing role is not built (no `role.go`, no role YANG under `nlri/ls`), and `RFC9552-8.2.6-2`, the row that role answered, now carries `{not-applicable}` in `rfc/short/rfc9552.md` | `ls internal/component/bgp/plugins/nlri/ls/`; `rfc/short/rfc9552.md` row 8.2.6-2 |
| The work depends on the sibling | It does not. Neither control reads the role, and the role is no longer owed by a row. `Depends` is set to `-`; the overlap is recorded as R-1 | this research |

**Dependency on route D of `spec-plugin-ownership-imports`.**
Route D moves the BGP-LS wire vocabulary (`types*.go`, the NLRI parsers, the JSON
helpers) from `internal/component/bgp/plugins/nlri/ls` to
`internal/core/bgp/nlri/bgpls`, and splits `rfc9552_test.go`. This spec creates
and edits nothing under `internal/component/bgp/plugins/nlri/ls/`, and names no
symbol route D moves, so the two land in either order without conflict. The
skeleton's planned `nlri/ls/accept_test.go` is dropped for that reason (and
because the accept decision is not made in the ls plugin). If route D lands
first, nothing here changes; if this lands first, route D's inventory is
unaffected.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - how a Link-State NLRI is framed and how it reaches the RIB
  → Constraint: ze both receives and produces BGP-LS: `bgp-ls-export` and `bgp-epe` originate, `bgp-rs` relays opaquely, `bgp-rib` stores bgp-ls in the opaque map of `storage.FamilyRIB`. A control on "neighbors to which Link-State NLRIs will be advertised" therefore has real producers to act on
  → Constraint: the Section 8.2.2 receive validation (`validateBGPLSNLRISyntax`, `RetainWellFormedNLRI`) already runs before callback dispatch; a ceiling check placed later in the receive path counts only well-formed NLRIs
  → Decision: no file this spec creates lives under `internal/component/bgp/plugins/nlri/ls/`, so route D of `spec-plugin-ownership-imports` is unaffected
- [ ] `docs/architecture/core-design.md` - declared by `filter_family/handler.go` ("family-filter per-update decision")
  → Constraint: the family-filter decision is generic over every registered family and both directions (`remove` in import or export, `tear-down` in import only); a per-direction BGP-LS refusal is expressed with it, never by a BGP-LS-specific branch
- [ ] `docs/architecture/testing/interop.md` - BGP interop suites, sidecars, scenario naming
  → Constraint: the third-party BGP peers are FRR, BIRD and GoBGP in Docker; the `le test interop-bgp speaker` sidecar is ze's own code, so a scenario that rests on it alone is not third-party interop evidence
  → Constraint: scenario directories are named, never numbered; the spec names its planned scenarios

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9552.md` - rows `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3`
  → Constraint: both are SHOULD-level, so `./le rfc check` does not gate them; the proof owed is a tagged test, not a gate pass
  → Constraint: `RFC9552-8.2.6-2` now reads `{not-applicable}` (it binds the operator); `RFC9552-8.2.6-1` is tagged by `TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes` and `TestBGPLSPrefixLimitActuallyFires` (`internal/component/bgp/reactor/rfc9552_prefix_limit_test.go`). Neither test is edited by this spec
  → Constraint: every tag this spec adds carries a discrimination record written by `./le rfc record` in the same change (`ai/rules/rfc-compliance.md`)

**RFC text, read at `rfc/full/rfc9552.txt`:**

| Section | Whole sentence quoted |
|---------|-----------------------|
| 8.2.3 | "An implementation SHOULD allow the operator to specify neighbors to which Link-State NLRIs will be advertised and from which Link-State NLRIs will be accepted." |
| 8.2.3 | "An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB)." |
| 8.2.6 | "An implementation MUST have the means to limit inbound updates." |
| 8.2.6 | "An operator MUST define an import policy to limit inbound updates as follows: * Drop all updates from peers that are only serving BGP-LS Consumers." |

**Key insights:** (minimal context to resume after compaction)
- The counting defect that blocked this work is fixed: `forEachPrefixEntry` dispatches through `nlrisplit`.
- The accept/advertise list already exists as the per-peer `family` block (capability negotiation), plus `family-filter` for one direction. What is missing is proof, a tag, documentation, and the peer-with-no-family-block hole.
- The RIB ceiling does not exist at any scope wider than one session. `prefixCounts` is built per `Session` (`NewSession`, `session.go`) and destroyed with it; nothing counts across peers.
- `Globals`/`GlobalsFromTree` (`internal/component/bgp/reactor/config.go`) is the existing bgp-level config surface, read both at load (`config/loader_create.go`) and at reload (`reactor_api.go`).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `forEachPrefixEntry` asks `nlrisplit.Get` for the family's splitter and counts a bgp-ls section by its own framing; `countPrefixEntries` is the nil-callback form. The maximum this feeds is per peer and per family, checked as the message is read, and it is not a ceiling on what the RIB holds.
- [ ] `internal/component/bgp/plugins/filter_family/config.go` - `parseFamilyFilters` reads the `family-filter` instances an operator writes; this is the only surface today by which a peer's BGP-LS can be refused.
- [ ] `rfc/short/rfc9552.md` - `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3` carry no annotation and no test; `RFC9552-8.2.6-1` carries no `{gap}`.
- [ ] `internal/component/bgp/reactor/session_validation.go` - `validateUpdateFamilies` walks MP_REACH/MP_UNREACH and, for a family `Negotiated().SupportsFamily` does not hold, either drops the UPDATE (`IgnoreFamilyMismatch` or `mode ignore`) or returns `ErrFamilyNotNegotiated`, which `session_read.go` turns into NOTIFICATION UPDATE Message Error / Optional Attribute Error (RFC 4760 Section 7) and a closed connection.
  → Constraint: this is today's accept-side refusal for a peer whose negotiated set lacks bgp-ls; AC-1/AC-2 prove it, they do not rebuild it
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - with no `family` block in config the OPEN advertises every plugin decode family (BGP-LS included); the in-code KNOWN DEFECT comment names three candidate fixes and refuses to pick one.
  → Constraint: a peer the operator did not name in any `family` block accepts BGP-LS today; this spec does not fix that fallback (Q-2)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - per-peer `family <afi/safi>` list with `mode` (`enable`, `disable`, `require`, `ignore`) and `prefix { maximum, warning, teardown, count (offered|installed), idle-timeout, reconnect }`; the family key is validated by `registered-address-family`.
  → Constraint: a new router-wide ceiling keys families the same way and reuses `registered-address-family`, so no BGP-LS literal enters the schema
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `prefixCounts` (per `Session`) holds `counts`, `installed` (families in `count installed` mode), `sets` (route identities per installed family) and `limits`; `applyInstalledPrefixSections` applies each section to its set, checks the per-peer maximum, and routes the delta through `applyPrefixDelta`/`applyPrefixCheck`; `rollbackPrefixSets` undoes a refused message; `reportPrefixExceeded` raises the report and metric.
  → Constraint: "installed" mode is the only count that equals what the peer holds in ze (set cardinality, as FRR `pcount` and BIRD `imp_routes`); a router-wide ceiling must be fed from it, never from the offered tally
  → Constraint: `prefixCounts` dies with the `Session`; a router-wide counter must be decremented by the session's whole installed set on teardown, or it leaks and the ceiling refuses forever
- [ ] `internal/component/bgp/reactor/config.go` - `Globals{RouterID, LocalAS}` and `GlobalsFromTree` read bgp-level leaves; `PeersFromTree` reads per-peer settings.
  → Decision: the ceiling's configuration joins `Globals`, the existing bgp-level surface, rather than a new parse path
- [ ] `internal/component/bgp/plugins/filter_family/handler.go` - `handleFilterUpdate` matches the instance's family against the UPDATE's MP family and, for `remove`, strips the MP attribute or rejects the UPDATE when nothing remains; direction-agnostic for `remove`.
  → Constraint: "advertise to X but do not accept from X" is a `remove` filter on X's import chain with bgp-ls negotiated; "accept from X but do not advertise to X" is the same filter on X's export chain
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib_opaque.go` and `familyrib.go` - bgp-rib stores bgp-ls per peer (`PeerRIB` → `FamilyRIB`, opaque map); its metrics sum `PeerRIB.Len()` per peer.
  → Constraint: bgp-rib sees a route only after the reactor accepted and dispatched it, and does not gate propagation; a ceiling enforced in bgp-rib would leave the RIB and the forwarded set disagreeing (rejected alternative, see Key Design Decisions)

**Behavior to preserve:** (unless the user explicitly said to change it)
- A peer with no `family` block keeps today's negotiation (Q-2 decides whether a separate spec changes that).
- A peer whose `family` block names `bgp-ls/bgp-ls` keeps accepting and propagating BGP-LS exactly as today when no ceiling is configured.
- The per-peer, per-family prefix maximum keeps its message-time semantics, its NOTIFICATION, its report codes and its metrics.
- With no router-wide ceiling configured, no session carries an installed set it did not carry before, so the receive path costs nothing new.

**Behavior to change:** (only what the user asked for)
- Prove, tag and document the existing per-peer accept/advertise control for Link-State NLRIs (`RFC9552-8.2.3-1`).
- Add an operator-declared router-wide ceiling on Link-State NLRIs held across all sessions (`RFC9552-8.2.3-3`), generic over registered families.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI with AFI 16388 arrives on an established session, in wire bytes.
- Operator configuration arrives as a YANG-modeled config tree at config-apply.

### Transformation Path
1. OPEN: the per-peer `family` block becomes Multiprotocol capabilities; the negotiated set is the intersection with the peer's (`session_negotiate.go`). This is where a neighbor is admitted to BGP-LS in both directions.
2. Receive: `validateUpdateFamilies` refuses an UPDATE of a non-negotiated family (`session_validation.go`, called from `session_read.go`).
3. Receive: Section 8.2.2 validation (`enforceRFC7606` → `validateBGPLSNLRISyntax` / `RetainWellFormedNLRI`).
4. Receive: prefix accounting (`checkPrefixLimits` → `applyInstalledPrefixSections`, `session_prefix.go`). The router-wide ceiling is checked here, after the per-peer maximum, against a counter shared by every session.
5. Import/export policy: `family-filter` instances in the peer's chains (`filter_family/handler.go`) refuse one direction.
6. Storage: `bgp-rib` and `bgp-rs` see only what the reactor dispatched.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ reactor | per-peer `family` list (existing); the ceiling arrives through `Globals` at load and at reload | Yes for the family list (`parseFamiliesFromTree`); No for the ceiling (A-2) |
| Session ↔ reactor-wide state | each session reserves against, and releases to, one counter per ceiling family owned by the reactor | No (new) |
| Reactor ↔ plugins | unchanged: a refused UPDATE is never dispatched | Yes (`checkPrefixLimits` runs before dispatch) |

### Integration Points
- `applyInstalledPrefixSections` and `rollbackPrefixSets` (`session_prefix.go`) - the ceiling reserves after the per-peer check passes and releases on rollback, withdrawal and session teardown.
- `newPrefixCounts` (`session_prefix.go`) - a family covered by a router-wide ceiling is tracked in installed mode on every session, whether or not the peer's own `count` leaf asked.
- `Globals` / `GlobalsFromTree` (`config.go`) and the reload reader in `reactor_api.go` - carry the ceiling.
- `reportPrefixExceeded` and the `ze_bgp_prefix_*` metrics - the pattern the ceiling's report and metrics follow.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | the ceiling sits beside the per-peer maximum, before dispatch |
| No unintended coupling (components stay isolated) | Yes (design) | no plugin is read; the reactor owns the counter |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | accept/advertise reuses negotiation and `family-filter`; the ceiling reuses installed-set accounting |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | installed sets already key route identities; the ceiling adds one atomic per section |
| Registration over hardcoding | Yes (design) | the ceiling is keyed by any registered family through `registered-address-family`; no BGP-LS literal in reactor or YANG |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A RIB-wide Link-State ceiling has an owner that can count across peers | research 2026-10-13: bgp-rib sees routes only after dispatch and does not gate forwarding; the reactor is the one owner that sees every session's accepted set before dispatch | the ceiling has to be approximated per peer, which is not what Section 8.2.3 asks | read `applyInstalledPrefixSections` and `NewSession` (done) | confirmed for the reactor; the bgp-rib reading is refuted |
| A-2 | `Globals` is re-read at reload and a changed value can reach running sessions without a session reset | `GlobalsFromTree` has a caller in `reactor_api.go` (reload) and one in `config/loader_create.go` (load) | the ceiling applies only to sessions built after a restart; a reload test would fail | Phase 1: read both callers and the reload apply path whole | unvalidated |
| A-3 | The forward path sends a family only to destinations that negotiated it, so the per-peer `family` list also governs "advertised to" | RFC 4760 obliges it; `reactor_api_forward.go` checks negotiation for EoR and refresh markers; the UPDATE forward gate was not located during research | the advertise half of 8.2.3-1 needs its own gate and AC-3 fails | Phase 1: read `ForwardUpdate` and the per-destination send path whole; AC-3's test discriminates it | unvalidated |
| A-4 | GoBGP can be configured to offer and to originate BGP-LS (`ls` afi-safi) toward ze | GoBGP decodes BGP-LS and its neighbor config accepts the `ls` family; whether its CLI or API can inject Link-State routes was not verified | the interop scenarios have no third-party BGP-LS originator; the in-house speaker sidecar is not third-party evidence and the owner must decide (Q-6) | Phase 1: build the GoBGP image the lab uses and try to inject one Node NLRI | unvalidated |
| A-5 | A session's installed set can be released whole on teardown at one site | `prefixCounts` is per `Session`; "Peer creates a new Session per connection" (`session_prefix.go` comment) | a teardown path that skips the release leaks the counter | Phase 3: enumerate every Session close path; one test per path class | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fault-management sibling's declared Consumer-facing role, if built, becomes a third mechanism for "who is BGP-LS accepted from" | that spec reaching implementation unchanged | reported to the owner (Q-5); this spec builds no role and names the overlap; the role's row 8.2.6-2 is now `{not-applicable}` |
| R-2 | The router-wide counter leaks on a teardown path (NOTIFICATION, hold-timer, GR, BFD down, collision) and the ceiling refuses every later NLRI | `ze_bgp_family_ceiling_count` stays above zero with every session down | release at the single session-destroy site (A-5); a test per close-path class asserts the count returns to zero |
| R-3 | Two sessions race past the ceiling between check and add | the count exceeds the maximum under concurrent UPDATEs | reserve with an atomic add, compare, and undo on over: the check and the increment are one operation |
| R-4 | Tracking an installed set for every session of a ceiling family costs memory equal to a second key copy of the Adj-RIB-In for that family | RSS growth after enabling the ceiling on a large BGP-LS feed | only families named by a ceiling pay; the cost is stated in the YANG description and the docs; no RSS measurement is claimed |
| R-5 | Graceful Restart keeps stale routes in bgp-rib while the new Session starts with an empty set, so the ceiling under-counts during the restart window | ceiling count drops to zero on a GR restart while bgp-rib still holds the stale paths | recorded as a Known Limitation with its Section 8.2.3 consequence; Q-4 asks whether it must be closed here |
| R-6 | Dropping the UPDATE that crosses the ceiling desynchronises ze's view from the peer's (the peer believes the route is announced) | an operator sees a peer's route missing with the session up | the same trade the per-peer `teardown false` already makes; the report names the peer, family and count, and a metric counts refusals; Q-3 asks whether tear-down is the default instead |
| R-7 | Lowering the ceiling at reload below the current count evicts nothing, so the RIB stays over the new bound | count above maximum after reload | documented: the ceiling refuses new NLRIs and never evicts, as FRR's maximum-prefix does; AC-8 pins it |
| R-8 | A family-less peer (no `family` block) still negotiates BGP-LS, so an operator who listed bgp-ls nowhere is surprised | a new peer without a `family` block receives BGP-LS | out of scope (Q-2); AC-1/AC-2 state the condition "the peer has a `family` block"; the docs say it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A leaked or racy counter refuses BGP-LS from every peer while sessions stay up, and a topology silently stops filling |
| How is it reverted? | single commit revert; the ceiling defaults to unset and the accept side adds no code |
| Who else touches this path? | `session_prefix.go` is the per-peer prefix-limit path for every family (`spec`s touching prefix limits: none open at research time); `spec-plugin-ownership-imports` route D (no file overlap); `plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` (role overlap only, R-1) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a peer whose `family` block omits `bgp-ls/bgp-ls` sends a Link-State NLRI | → | `validateUpdateFamilies` refusal | `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted` |
| ze relays BGP-LS and one destination's `family` block omits `bgp-ls/bgp-ls` | → | the per-destination forward gate (A-3) | `test/plugin/rfc9552-823-advertise-by-family.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |
| config `bgp` level declares a ceiling for `bgp-ls/bgp-ls` | → | `GlobalsFromTree` → reactor ceiling → every session's `newPrefixCounts` | `TestRouterCeilingConfigReachesEverySession` |
| the router holds the ceiling's maximum of Link-State NLRIs across two peers and one more arrives | → | `applyInstalledPrefixSections` ceiling reserve | `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI` |
| an operator runs ze with the ceiling against two BGP-LS peers | → | the whole receive path | `test/plugin/rfc9552-823-rib-ceiling.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |

## Acceptance Criteria

Each AC names the test that proves it. "Peer with a `family` block" is a precondition of AC-1..AC-4 because of the fallback recorded in R-8.

| AC ID | Input / Condition | Expected Behavior | Proof |
|-------|-------------------|-------------------|-------|
| AC-1 | a peer whose `family` block names `bgp-ls/bgp-ls` sends a Link-State NLRI | it is accepted and reaches the RIB, as today | `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted`; `rfc9552-823-accept-by-family.ci` |
| AC-2 | a peer whose `family` block omits `bgp-ls/bgp-ls` | BGP-LS is not negotiated with it; if it sends a Link-State NLRI anyway the UPDATE is refused with NOTIFICATION UPDATE Message Error / Optional Attribute Error (RFC 4760 Section 7), or dropped with the session kept when the family is `mode ignore`; nothing is dispatched | `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted`; `rfc9552-823-accept-by-family.ci` |
| AC-3 | ze holds BGP-LS (relayed or exported) and has two established destinations, one whose `family` block names `bgp-ls/bgp-ls` and one whose does not | only the first receives the Link-State NLRI | `rfc9552-823-advertise-by-family.ci` |
| AC-4 | a peer negotiates BGP-LS and has a `family-filter` `remove` on `bgp-ls/bgp-ls` in its import chain only | ze advertises BGP-LS to it and accepts none from it; the mirror (export chain only) accepts and does not advertise | `rfc9552-823-accept-by-family.ci` (both directions) |
| AC-5 | `bgp` level declares a ceiling `maximum N` for `bgp-ls/bgp-ls` and the sessions together hold N installed Link-State NLRIs | the next announcement of a NEW Link-State NLRI from any peer is refused: the UPDATE carrying it is not dispatched, the sessions stay up, a report names peer, family, count and maximum, and the refusal counter increments | `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI`; `rfc9552-823-rib-ceiling.ci` |
| AC-6 | at the ceiling, a peer re-announces an NLRI it already holds, or withdraws one | accepted; a withdrawal lowers the count by one and the next new NLRI is accepted | `TestRouterCeilingCountsInstalledNotOffered` |
| AC-7 | a session holding K ceiling NLRIs goes down by any close path | the router-wide count drops by exactly K | `TestRouterCeilingReleasesOnSessionTeardown` |
| AC-8 | the ceiling is lowered at reload below the current count | nothing is evicted; new NLRIs are refused until the count is below the new maximum | `TestRouterCeilingLoweredAtReloadEvictsNothing` |
| AC-9 | no ceiling is configured | no session tracks an installed set it did not track before, and every existing prefix-limit test passes unchanged | `TestNoRouterCeilingAddsNoInstalledSet`; the unchanged tests in "RFC-tagged tests" below |
| AC-10 | a ceiling and a per-peer `prefix maximum` both apply | the per-peer maximum is checked first with its own action; the ceiling is checked only for a message the per-peer check accepted | `TestPerPeerMaximumIsCheckedBeforeRouterCeiling` |
| AC-11 | the ceiling names an unregistered family | config load refuses it with the `registered-address-family` error | `TestRouterCeilingRefusesUnregisteredFamily` |
| AC-12 | two sessions announce concurrently at N-1 | exactly one new NLRI is accepted | `TestRouterCeilingConcurrentReserve` (run under `-race`) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | lists `bgp-ls/bgp-ls` only on the collector peers | config → OPEN → negotiation → receive refusal for others | `rfc9552-823-accept-by-family.ci` |
| 2 | caps the router at N Link-State NLRIs | config → `Globals` → reactor ceiling → session reserve → refusal report | `rfc9552-823-rib-ceiling.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted` | `internal/component/bgp/reactor/rfc9552_accept_by_family_test.go` | AC-1, tag `RFC9552-8.2.3-1 positive` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted` | `internal/component/bgp/reactor/rfc9552_accept_by_family_test.go` | AC-2, tag `RFC9552-8.2.3-1 negative` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingConfigReachesEverySession` | `internal/component/bgp/reactor/router_ceiling_test.go` | wiring, AC-9 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI` | `internal/component/bgp/reactor/rfc9552_router_ceiling_test.go` | AC-5, tag `RFC9552-8.2.3-3 negative` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRFC9552RouterCeilingAcceptsUpToTheMaximum` | `internal/component/bgp/reactor/rfc9552_router_ceiling_test.go` | AC-5, tag `RFC9552-8.2.3-3 positive` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingCountsInstalledNotOffered` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-6 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingReleasesOnSessionTeardown` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-7, one subtest per close-path class found under A-5 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingLoweredAtReloadEvictsNothing` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-8 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestNoRouterCeilingAddsNoInstalledSet` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-9 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestPerPeerMaximumIsCheckedBeforeRouterCeiling` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-10 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingRefusesUnregisteredFamily` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-11 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingConcurrentReserve` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-12 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### RFC-tagged tests this spec changes (owner approval)

**None planned.** Every tag above is new, in a new file. The existing tagged
tests on the touched path are named so an implementer who finds one must change
stops and asks: `rfc9552_prefix_limit_test.go` (`RFC9552-8.2.6-1`, two tests),
`rfc4271_prefix_limit_cease_peer_test.go`, and every tagged test in
`session_prefix_test.go`. `newPrefixCounts` and `NewSession` keep their
signatures so these files compile untouched; if the design forces an edit to any
of them, the edit needs an OWNER row in `test/rfc-changed.md` before it lands
(`weakened.Proposed`, `internal/le/testweakened/proposed.go`), and the
implementer stops and raises it rather than writing that row.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ceiling `maximum` | 1 - 4294967295 (`uint32`, `range "1..max"`, as per-peer `prefix maximum`) | 4294967295 | 0 (refused by YANG `range`) | 4294967296 (refused by `uint32`) |
| ceiling `warning` | 0 - `maximum` - 1 (same rule as per-peer `warning`: not less than `maximum` is refused) | `maximum` - 1 | N/A | `maximum` (refused at config validation) |
| router-wide count at the edge | N-1, N, N+1 installed | N accepted | N/A | N+1 refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc9552-823-accept-by-family` | `test/plugin/rfc9552-823-accept-by-family.ci` | two `le test peer` speakers; only the one whose `family` block names bgp-ls has its BGP-LS reach the RIB; the other is refused per AC-2; a third with bgp-ls negotiated and a `family-filter` import `remove` receives BGP-LS and contributes none (AC-4) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `rfc9552-823-advertise-by-family` | `test/plugin/rfc9552-823-advertise-by-family.ci` | ze relays a Link-State NLRI; the destination without bgp-ls in its `family` block receives no MP_REACH for AFI 16388, asserted on the exact frames (AC-3) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `rfc9552-823-rib-ceiling` | `test/plugin/rfc9552-823-rib-ceiling.ci` | ceiling 3; peer A sends 2, peer B sends 2; B's second is refused, both sessions stay up, the RIB holds 3, the report and the counter are visible; A withdraws one and B's resend is accepted | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-ls-accept-by-family-gobgp` | `test/interop/scenarios/bgp-ls-accept-by-family-gobgp/` | GoBGP (offers `ls`) | GoBGP, configured with the `ls` afi-safi, negotiates BGP-LS with the ze peer whose `family` block names it and not with the one whose block omits it, as read from GoBGP's own neighbor state; its injected Node NLRI reaches ze's RIB only through the first (A-4) | | <!-- doc-links: ignore (directory this open spec plans and has not created yet) -->
| `bgp-ls-rib-ceiling-gobgp` | `test/interop/scenarios/bgp-ls-rib-ceiling-gobgp/` | GoBGP | GoBGP injects N+1 Link-State NLRIs against ze's ceiling N; the session stays established, ze holds N, and GoBGP's view of the session shows no NOTIFICATION (A-4) | | <!-- doc-links: ignore (directory this open spec plans and has not created yet) -->

## Design

### Alternatives

**Accept/advertise list (`RFC9552-8.2.3-1`)**

| # | Approach | Gains | Costs |
|---|----------|-------|-------|
| A1 (recommended) | The per-peer (and per-group) `family` block is the list: naming `bgp-ls/bgp-ls` admits a neighbor to BGP-LS in both directions through RFC 4760 negotiation; a `family-filter` `remove` in one chain narrows it to one direction. No new config; the work is proof, tag and documentation | no second mechanism; already an allow-list; already per peer and per group; both directions | the family-less fallback (R-8) stays a hole the claim must name |
| A2 | A dedicated `bgp-ls { accept-from [...]; advertise-to [...] }` neighbor list | reads like the RFC sentence | a second answer to the question negotiation answers; two configs can disagree; BGP-LS-specific config in a generic path (`ai/rules/simplicity.md`, `ai/rules/principles.md` "declared once") |
| A3 | Build the fault-management sibling's declared Consumer-facing role and call it the accept list | one config word | all-or-nothing per peer, inbound only; its row (8.2.6-2) is now `{not-applicable}` |

**RIB ceiling (`RFC9552-8.2.3-3`)**

| # | Approach | Gains | Costs |
|---|----------|-------|-------|
| C1 | Tag the existing per-peer `prefix maximum` with `count installed` on `bgp-ls/bgp-ls` as the answer | zero code | bounds one neighbor's contribution, not "a router's RIB"; the claim would have to be scoped evidence with the router-wide clause as the gap |
| C2 | Enforce a ceiling inside bgp-rib's `PeerRIB.Insert` | counts what bgp-rib stores | bgp-rib does not gate dispatch or forwarding, so the RIB and the relayed set disagree; bgp-rs relays without bgp-rib; a refused insert is invisible to the peer path |
| C3 (recommended) | A bgp-level ceiling per registered family, owned by the reactor: one counter per ceiling family shared by every session, fed from the installed sets the per-peer path already keeps; checked after the per-peer maximum, before dispatch | counts what the router holds from all peers, before anything is dispatched; generic per family; reuses installed-set accounting and the report/metric pattern | one atomic per section on ceiling families; a per-session installed set for those families (R-4); a leak class (R-2) |

### Proposed behaviour (C3)

| Aspect | Now | After |
|--------|-----|-------|
| Config | per-peer `family <f> prefix { ... }` only | adds a bgp-level list keyed by registered family with `maximum` and `warning` (name proposed in Q-7); unset by default |
| Session tracking | installed set only for families whose `count` is `installed` | also for every family named by a router ceiling |
| Receive | per-peer check, then dispatch | per-peer check, then router-ceiling reserve for the net new installed NLRIs of the message; over → undo reserve, `rollbackPrefixSets`, drop the UPDATE (no NOTIFICATION, Q-3), report, counter |
| Withdraw / teardown | per-session set shrinks / dies | the router counter is released by the same amounts |
| Reload | n/a | new maximum applies to the next UPDATE; nothing is evicted |
| Observability | `ze_bgp_prefix_*` per peer | adds router-wide gauges for count and maximum and a refusal counter labelled by family and peer (names fixed against `docs/plugin-development/metrics.md` in Phase 3) |

### Triple challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | The accept side adds no code. The ceiling adds one counter type and one check beside an existing one; C1 is simpler but does not answer the sentence, C2 is not simpler and is wrong |
| Uniformity | The ceiling copies the per-peer `prefix` leaf shapes (`maximum`, `warning`, `range "1..max"`), the family key validator, the report and metric pattern, and the installed-count semantics |
| Performance | No cost when unset (AC-9). When set: one atomic add/compare per section of a ceiling family, on the existing installed-set path that already allocates route-identity keys; no new per-UPDATE allocation beyond the set entry the installed mode already makes |

### Strongest concern

The leak class (R-2): every session close path must release, and the reactor
has several (NOTIFICATION sent or received, hold timer, BFD down, collision,
GR, admin stop, reload-driven restart). A single missed path makes the ceiling
refuse forever with every session up. Phase 3 enumerates the paths from source
before writing the release, and AC-7 carries one subtest per path class.

## Files to Modify
- `internal/component/bgp/reactor/session_prefix.go` - `newPrefixCounts` tracks ceiling families; `applyInstalledPrefixSections` reserves after the per-peer check; rollback and withdraw release
- `internal/component/bgp/reactor/session.go` - the session-destroy site releases the installed sets (A-5)
- `internal/component/bgp/reactor/config.go` - `Globals` and `GlobalsFromTree` carry the ceiling
- `internal/component/bgp/reactor/reactor_api.go` - reload applies a changed ceiling (A-2)
- `internal/component/bgp/config/loader_create.go` - load passes the ceiling to the reactor
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the bgp-level ceiling list
- `rfc/short/rfc9552.md` - `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3` gain tagged tests and their scoped evidence (Q-2 for 8.2.3-1)
- `rfc/discrimination/rfc9552.json` - written by `./le rfc record`, never by hand
- `docs/architecture/wire/nlri-bgpls.md` - a "Receiver Controls (RFC 9552 Section 8.2.3)" section: the family list, the filter, the ceiling
- `docs/architecture/core-design.md` - declared by `filter_family/handler.go`; read at implementation and edited only if the family-filter description must name the per-direction use
- `docs/guide/configuration.md` - the bgp-level ceiling
- `docs/plugin-development/metrics.md` - the new metrics
- `docs/features/rfc-status.md` - generated from `rfc/short/` rows; regenerated, not hand-edited

## Files to Create
- `internal/component/bgp/reactor/router_ceiling.go` - the reactor-owned per-family counter (reserve, release, report) <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/router_ceiling_test.go` - untagged behaviour tests <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/rfc9552_router_ceiling_test.go` - the two `RFC9552-8.2.3-3` tags <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/rfc9552_accept_by_family_test.go` - the two `RFC9552-8.2.3-1` tags <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `test/plugin/rfc9552-823-accept-by-family.ci`, `test/plugin/rfc9552-823-advertise-by-family.ci`, `test/plugin/rfc9552-823-rib-ceiling.ci` <!-- doc-links: ignore (files this open spec plans and has not created yet) -->
- `test/interop/scenarios/bgp-ls-accept-by-family-gobgp/`, `test/interop/scenarios/bgp-ls-rib-ceiling-gobgp/` (A-4) <!-- doc-links: ignore (directories this open spec plans and has not created yet) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`, bgp-level ceiling list |
| YANG validation constraints | Yes | `uint32` `range "1..max"` on `maximum`; key validated by `ze:validate "registered-address-family"` |
| YANG custom validators | Yes | the existing `registered-address-family`; the `warning < maximum` rule reuses the per-peer check's validator |
| CLI commands/flags | No | configuration only |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | automatic from YANG; the family key completes from the registered-family `CompleteFn` |
| Functional test for new RPC/API | Yes | the three `.ci` files above |
| Pipe completeness | N-A | no new command output |
| Env var registration | No | config, not an environment default |
| Doctor check for runtime dependencies | No | no file, socket, port, module, binary or certificate |
| Prometheus counters/metrics | Yes | router-wide count and maximum gauges, refusal counter (family, peer) |
| BGP family surface (new SAFI / capability / attribute) | No | no new family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | the matching `features/<id>.md` (BGP prefix limits, or BGP-LS), found with `ai/INDEX.md` at Phase 3 |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | reactor only |
| 6 | Has a user guide page? | Yes | the BGP-LS section of the guide that `ai/INDEX.md` names for `bgp-ls`; if none exists, `docs/architecture/wire/nlri-bgpls.md` carries it |
| 7 | Wire format changed? | No | no byte ze writes changes |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9552.md`, regenerated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` for the two scenarios |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it lists prefix-limit scopes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/nlri-bgpls.md`; `docs/architecture/core-design.md` reviewed (declared by `filter_family/handler.go`) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors spec plan/release/fixes/spec-bgp-ls-receiver-accept-and-limit-controls.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `prefix { maximum }` example is checked against the YANG after the change |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` rows for `bgp-ls` and for prefix limits point at `docs/architecture/wire/nlri-bgpls.md` and the configuration guide |
| What rule prevents regression | the four tags (`RFC9552-8.2.3-1` ±, `RFC9552-8.2.3-3` ±) with discrimination records, re-verified by `./le rfc check` |
| What registry prevents drift | the family key is validated by the family registry (`registered-address-family`), so no list of families is written |
| What verification proves it | the reactor unit tests, the three `.ci` files, and the two interop scenarios |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- config reaches every session
   - Tests: `TestRouterCeilingConfigReachesEverySession`, `TestNoRouterCeilingAddsNoInstalledSet`
   - Files: `ze-bgp-conf.yang`, `config.go`, `loader_create.go`, `reactor_api.go`, `router_ceiling.go` (stub that never refuses)
   - Verify: the config reaches each new session's `prefixCounts`; the refusal test fails because the stub never refuses
   - Settle A-2 (reload path) and A-3 (forward gate) here; both change what Phases 2 and 3 build
2. **Phase: Accept/advertise proof** -- no product code expected
   - Tests: `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted`, `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted`, `rfc9552-823-accept-by-family.ci`, `rfc9552-823-advertise-by-family.ci`
   - Files: the two new test files; `rfc/short/rfc9552.md`
   - Verify: each tag's red is recorded with `./le rfc record` (break `validateUpdateFamilies`' refusal; break the forward gate found under A-3). If A-3 is refuted, the advertise gate is a defect fixed here
3. **Phase: The ceiling** -- reserve, release, report
   - Tests: the remaining `router_ceiling_test.go` and `rfc9552_router_ceiling_test.go` tests, `rfc9552-823-rib-ceiling.ci`
   - Files: `router_ceiling.go`, `session_prefix.go`, `session.go`
   - Verify: enumerate every Session close path from source before writing the release (A-5); AC-12 under `-race`
4. **Phase: Interop and the ledger**
   - Tests: `bgp-ls-accept-by-family-gobgp`, `bgp-ls-rib-ceiling-gobgp`
   - Files: `test/interop/scenarios/`, `docs/architecture/testing/interop.md`, `rfc/short/rfc9552.md`, docs listed above
   - Verify: settle A-4 first; each scenario is driven red by reverting the producer and rebuilding the ze image (`ai/rules/interop-and-goal-validation.md`)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation or an existing producer at file:line |
| Correctness | the ceiling reserves only the NET new installed NLRIs of a message (re-announcements and withdrawals never consume) |
| Correctness | reserve is one atomic operation with its compare; an over reserve is undone before the per-session rollback |
| Correctness | every Session close path releases; the count is zero with every session down |
| Correctness | the per-peer maximum's action (NOTIFICATION or drop) is unchanged and runs first |
| Naming | no `bgp-ls` literal in the reactor or the ceiling YANG; the ceiling is per registered family |
| Data flow | the ceiling decision is made before dispatch; no plugin is consulted |
| Rule: `ai/rules/rfc-compliance.md` | each of the four tags has a discrimination record; the 8.2.3-1 claim states the family-block precondition and names the family-less fallback as the gap |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| bgp-level ceiling in YANG | grep `ze-bgp-conf.yang`; `ze config validate` on a config that sets it |
| four new tags with records | `./le rfc check`; `rfc/discrimination/rfc9552.json` |
| three `.ci` tests | `./le test functional` on each, by name |
| two interop scenarios | `./le test integration interop` on each, by name |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | the ceiling is the defence; its own set adds memory per ceiling family (R-4), stated in the YANG description |
| Fail open | a leaked counter fails closed (refuses); an under-count (R-5) fails open during GR only, documented |
| Input validation | the family key and `maximum` are validated by YANG; `warning` by the existing rule |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A tagged existing test must change | STOP; owner approval through `test/rfc-changed.md` |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owner Questions

| ID | Question | Recommendation | Blocks |
|----|----------|----------------|--------|
| Q-1 | Is the per-peer `family` block plus `family-filter` accepted as ze's answer to `RFC9552-8.2.3-1` (no new BGP-LS config), or do you want a dedicated BGP-LS neighbor list? | the family block (A1) | Phase 2 |
| Q-2 | A peer with no `family` block offers every plugin family, BGP-LS included (`session_negotiate.go` KNOWN DEFECT; journal `silent-fall-through.md` 2026-08-16). Does 8.2.3-1 land now as scoped evidence ("peers with a family block") with that fallback as the named gap, or wait for a separate spec that fixes the fallback? | land now, scoped; the fallback is a design decision across all families | the 8.2.3-1 claim text |
| Q-3 | When the router-wide ceiling is reached: drop the crossing UPDATE and keep every session up, or tear down the peer whose UPDATE crossed (Cease / Maximum Number of Prefixes Reached)? | drop and keep up: the crossing peer is not necessarily the one that filled the RIB | Phase 3 |
| Q-4 | What does the ceiling count: paths summed across peers (what the Adj-RIB-In stores), or distinct Link-State NLRIs across peers? And must Graceful Restart stale paths count (R-5)? | paths summed; GR window as a Known Limitation | Phase 3 |
| Q-5 | The fault-management sibling still plans a declared Consumer-facing role for `RFC9552-8.2.6-2`, which the summary now marks `{not-applicable}`, and its validator half is already in the tree. Should that spec be re-scoped (validator closed, role dropped or re-justified)? This spec builds no role either way | re-scope the sibling | nothing here |
| Q-6 | If GoBGP cannot inject BGP-LS (A-4), no third-party BGP-LS originator exists in the lab (FRR does not originate BGP-LS). Is the in-house `interop-bgp speaker` sidecar acceptable as the BGP-LS peer, or must another daemon be added? | try GoBGP first; ask again only if A-4 is refuted | Phase 4 |
| Q-7 | Config name and place for the ceiling: a bgp-level `family <afi/safi> { rib-maximum N; rib-warning M }`-style list, or a `prefix-limit` container at bgp level mirroring the per-peer `prefix` container? | mirror the per-peer `prefix` shape at bgp level so the two read alike | Phase 1 |

-> Decision (owner, 2026-10-13, "LS follow recommendations"): Q-1 to Q-7 are answered as recommended: the family block; land 8.2.3-1 now scoped, with the no-family-block fallback named as a gap against RFC 9552 Section 8.1.2's off-by-default; drop the crossing UPDATE and keep sessions up; count paths summed, GR stale paths a Known Limitation; re-scope the fault-management sibling; try GoBGP first (FRR originates BGP-LS only from 10.7); mirror the per-peer `prefix` shape, citing the IETF YANG draft's `max-number` as the external name. Evidence: research 2026-10-13 (`research-drain-ospf-startup-bgpls.md`).

## Known Limitations
- Section 8.2.3's producer-side SHOULDs other than the advertise half of 8.2.3-1 (the rate limit 8.2.3-2, abstracted topologies 8.2.3-4, the 4096-byte UPDATE size 8.2.3-7) stay with `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md`.
- A peer with no `family` block negotiates BGP-LS (R-8, Q-2); the fix belongs to whoever designs the family fallback.
- During a Graceful Restart window the router-wide count under-counts stale paths still held by bgp-rib (R-5), unless Q-4 brings it into scope.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The per-peer `family` block is the accept/advertise list | dedicated BGP-LS neighbor list; the sibling's role | one declaration per fact; already per peer and per group; covers both directions |
| The ceiling lives in the reactor, fed by installed sets | per-peer maximum only; bgp-rib insert check | only the reactor sees every session before dispatch; installed count is what is held |
| The ceiling is per registered family, not BGP-LS-only | a `bgp-ls`-specific leaf | registration over enumeration; the next family that wants a router cap costs no code |

## RFC Documentation (Scope: protocol)

The ceiling's check carries `// RFC 9552 Section 8.2.3: "An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB)."` above the statement that refuses, and the test files carry the four `RFC requirement:` tags.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every owner question Q-1..Q-7 answered by the owner, with the answer recorded here
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
