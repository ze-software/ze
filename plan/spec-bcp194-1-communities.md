# Spec: BCP 194 child 1 -- community handling (RFC 7454 §11 and RFC 8195)

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | spec-bcp194-0-umbrella |
| Phase | - |
| Deferral shard | `plan/deferrals/bcp194-1-communities.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze must handle BGP communities as RFC 7454 §11 and RFC 8195 describe. Child 1 of
the set that `plan/spec-bcp194-0-umbrella.md` coordinates.

Three problems, in order of severity.

**Dead code that a green test suite hides.** `parseLargeCommunityAttr`
(`internal/component/bgp/wireu/community.go`) records a prepend request from
`<rsASN>:101..103:<targetASN>` into `CommunityPolicy.PrependTargets`. The reader
`PrependCount` has no caller outside `internal/component/bgp/plugins/rs/community_test.go`,
which asserts it returns 1, 2 and 3. No AS_PATH is changed. `RFC7999Blackhole`
has the same shape: `parseCommunityAttr` sets it and nothing reads it.

**A leak.** Both route-server rails remove attribute code 8 only. The large
control communities the same rails just consumed stay on the route and reach the
client.

**Two absent features.** RFC 8195 Function 3 (relation to origin) exists nowhere,
although `resolvePeerRole` already answers the question it needs. RFC 7454 §11's
inbound scrub of communities carrying Ze's own ASN in the Global Administrator
field cannot be expressed: `ingressStripCommunities` matches whole 12-byte
literal values from a named set.

**One obligation the same engine satisfies.** RFC 7999 §3.2 asks a speaker that
receives a route tagged with the BLACKHOLE community to add NO_ADVERTISE or
NO_EXPORT, so the prefix does not propagate outside the local AS. That is an
ingress community-add keyed on a received community, which is the engine this
spec builds. It also bounds the new scrub: a well-known community must survive
it. Honouring BLACKHOLE, which means discarding traffic, is child 6
(`plan/spec-bcp194-6-blackhole.md`) and is not in this spec.

A separate defect in the same area is in scope here: `rfc/short/rfc7999.md` was
written with no source text in the repository. It captured 6 requirements and
none of the 4 MUST-level obligations the document carries, anchored every id to
`x`, and stated two rules the RFC does not contain. It was re-authored against
the source on 2026-08-08, and the disposition row that called the source missing
was corrected. That work is done and this spec records it as history, not as
remaining work.

Owner decisions taken at the /ze-spec design gate on 2026-08-08 are recorded in
Key Design Decisions below.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - the zero-copy wire model the community
      paths sit in
  → Constraint: attribute values are written buffer-first with
    `WriteTo(buf, off) int`. No path may return a fresh `[]byte` for an attribute
    value on the forwarding rail.
  → Decision: an UPDATE that gains no new attribute keeps its base byte order and
    stays byte-identical. Adding an attribute forces a merge-insert at the
    ascending type-code slot, so a tag that varies per destination gives every
    destination a distinct edit set and defeats the fan-out dedup.

- [ ] `docs/guide/bgp-policy.md` - the operator-facing policy surface this spec
      extends
  → Constraint: the "Route-server control communities" section documents the
    `0:X`, `RS:X` and `RS:101..103:X` spellings to operators. Any numbering change
    is a documented behavior change, not an internal detail.

### RFC Summaries (Scope: protocol)

- [ ] `rfc/short/rfc8195.md` - the conventions this spec implements
  → Constraint: §3.2 and §4.1.1 both say an AS "could assign" a function number.
    The numbers 3, 4 and 6 are examples. Hard-coding them states a convention on
    the operator's behalf.
  → Decision: Function 3 is informational and its Global Administrator is the AS
    that tags the route (§2.1). Functions 4 and 6 are action communities and
    their Global Administrator is the AS expected to act (§2.2). The two halves
    run in opposite directions: Function 3 on ingress, Functions 4 and 6 on
    egress.

- [ ] `rfc/short/rfc7454.md` - §11 community scrubbing, written 2026-08-08
  → Constraint: §11's first bullet is one obligation with a carve-out, not two.
    Scrub inbound communities carrying your own number in the high-order bits,
    AND allow only those communities that customers and peers use as a signaling
    mechanism. The carve-out makes the design a keep-list, not a deny-list.
  → Constraint: §11's second and third bullets forbid removing communities the
    operator did not ask to remove, and name NO_EXPORT specifically.

- [ ] `rfc/short/rfc7947.md` - route-server transparency
  → Constraint: `RFC7947-x-1` is a SHOULD NOT, corrected from MUST NOT on
    2026-07-23. A route server SHOULD NOT prepend its own AS nor modify AS_PATH in
    any other way. Its stated purpose is backward compatibility with clients that
    cannot accept a non-adjacent leftmost AS.
  → Decision: the prepend is applied only when the route carries the control
    community that requests it. A client that sends it has asked for the
    modification, which is the exception the SHOULD NOT contemplates.

- [ ] `rfc/short/rfc9234.md` - the role Function 3 derives from
  → Constraint: Table 2 pairs the roles. `peerRoleComplement` in
    `internal/component/bgp/plugins/role/otc.go` is the value form of that table.
  → Decision: the relation parameter derives from what the peer IS to Ze, which
    `resolvePeerRole` returns, not from the configured local role. The resolver
    already prefers a received Role capability over the configuration.

- [ ] `rfc/short/rfc7999.md` - the BLACKHOLE community
  → Constraint: §3.2 asks a receiving speaker to add NO_ADVERTISE or NO_EXPORT to
    a BLACKHOLE-tagged route, and says the choice between them follows the
    operator's routing policy. So the leaf offers both and neither is forced.
  → Constraint: BLACKHOLE is 65535:666, a well-known value whose Global
    Administrator is not Ze's ASN. The §11 scrub must not reach it, and neither
    may it reach NO_EXPORT or NO_ADVERTISE, which RFC 7454 §11 protects by name.
  → Decision: §3.1 makes ignoring BLACKHOLE a conformant choice, so adding the
    propagation guard does not commit Ze to honouring the community.

- [ ] `rfc/short/rfc8092.md` - the LARGE_COMMUNITY wire format
  → Constraint: enrolled with seven MUST-level requirements over
    `internal/core/bgp/attribute/community.go`. This spec adds no wire format and
    must not weaken any tagged test of that codec.

**Key insights:** (minimal context to resume after compaction)

- The egress action engine is not new. `wireu.CommunityPolicy` already models
  Function 4 as `ShouldForwardTo` and Function 6 as `PrependCount`. The first is
  live on both route-server rails. The second reaches no wire.
- `filterapi.EgressFilterFunc` returns false to suppress one destination and
  carries a `ModAccumulator` for attribute edits. Both halves of the egress work
  have a seam already, so the general rail needs no reactor redesign.
- `PeerFilterInfo` carries `PeerAS` and `LocalAS`, so an ingress filter derives
  the Global Administrator per peer with no system-wide setting.
- The relation tag is written ONCE on ingress, per received route. It does not
  vary per destination and does not defeat the fan-out dedup.
- `ze-filter-community.yang` augments `bgp:filter` at four levels: bgp, group,
  group/peer and peer. Its leaf-lists carry `ze:cumulative`, so levels combine
  rather than replace.
- Named community set values are `type string` with no pattern, so today nothing
  rejects a malformed or wildcard value at config time.

## Current Behavior (MANDATORY)

**Source files read:** (read before this spec was written)

- [ ] `internal/component/bgp/wireu/community.go` - parses COMMUNITY (code 8) and
      LARGE_COMMUNITY (code 32) from UPDATE wire bytes into `CommunityPolicy`.
      `parseCommunityAttr` reads `65535:666` into `RFC7999Blackhole`, `0:X` into
      `BlacklistASNs`, `RS:0` into `RSBlackhole` and `RS:X` into `WhitelistASNs`.
      `parseLargeCommunityAttr` reads `RS:101..103:X` into `PrependTargets`, keeping
      the highest count when a target repeats. `StripControlCommunities` returns
      the wire bytes of matching code 8 values only, and matches on the low
      sixteen bits of the route-server ASN.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - route-server fast path.
      Calls `wireu.ParseCommunityPolicy`, gates forwarding on `ShouldForwardTo`,
      and records one removal operation for code 8.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the general
      forward path. Its route-server branch does the same three things. Its
      non-route-server path does none of them.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `runIngressPolicyChain`
      accepts on an empty filter chain, and fails closed when filters are
      configured but the API server is absent.
- [ ] `internal/component/bgp/plugins/filter_community/filter.go` -
      `ingressTagCommunities` creates the attribute when absent and always writes
      extended length. `ingressStripCommunities` matches with an exact whole-value
      lookup into a set.
- [ ] `internal/component/bgp/plugins/filter_community/config.go` - `parseLargeWire`
      requires all three fields to be literal integers. No wildcard, no field mask.
- [ ] `internal/component/bgp/plugins/filter_community/egress.go` - records removal
      operations into the accumulator rather than rewriting bytes.
- [ ] `internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang` -
      named sets at `/bgp/community/{standard,large,extended}` as lists keyed by
      name with a `leaf-list value` of `type string`. Filter fields are
      `ingress/community/{tag,strip}` and `egress/community/{tag,strip}`, all
      `ze:cumulative`, augmented into `bgp:filter` at four levels.
- [ ] `internal/component/bgp/plugins/role/otc.go` - `resolvePeerRole` returns what
      the peer is to Ze, preferring the received Role capability and falling back
      to `peerRoleComplement` of the configured role. It warns and returns empty
      when the table and the config validator disagree. `OTCIngressFilter` and
      `OTCEgressFilter` are the shape a new filter copies.
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `PeerFilterInfo`,
      `IngressFilterFunc`, `EgressFilterFunc`, `ModAccumulator`, `AttrGenerator`.
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` adds
      NO_EXPORT and removes nothing.
- [ ] `scripts/dev/rfc_requirements.py` - `check_new_summaries` continues only for
      an enrolled stem or a parse error. `source_keyword_count` counts
      `MUST NOT|MUST|SHALL NOT|SHALL` over the whole source text.
      `source_prose_keyword_count` sits beside it and its comment states that a
      genuinely non-normative document shows zero uppercase hits.

**Behavior to preserve:**

- The `0:X`, `RS:X`, `RS:0` and `RS:101..103:X` spellings keep their current
  meaning on the route-server rail. Operators send them today.
- `ShouldForwardTo` keeps suppressing a client when the route names its ASN.
- An UPDATE that no filter modifies stays byte-identical on the forwarding rail.
- `TestReactorForwardRSTransparent` stays green and becomes the negative case for
  the conditional prepend.
- The seven RFC 8092 tagged tests over the LARGE_COMMUNITY codec.
- `send-community` remains the only path that suppresses a community attribute
  wholesale, and it stays off by default.
- `ze:cumulative` semantics on the existing leaf-lists.
- The empty-chain accept in `runIngressPolicyChain`. This spec adds no implicit
  filter.

**Behavior to change:**

- `PrependCount` gains a caller and the prepend reaches the wire, conditioned on
  the requesting community.
- Both route-server rails remove the large control communities they consumed.
- `filter_community` gains a derived value source and a Global Administrator
  scoped match.
- The general eBGP forwarding rail gains Functions 4 and 6.
- `rfc/not-enrolled.txt` records rfc7454 and rfc8195 as `non-normative`.
- `source_keyword_count` stops counting the RFC 2119 key-words sentence.

## Data Flow (MANDATORY)

### Entry Point

Three entry points, one per direction.

- Received UPDATE wire bytes, at the ingress filter chain, for the relation tag
  and the §11 scrub.
- Forwarded UPDATE wire bytes, per destination peer, for Functions 4 and 6.
- Configuration, delivered as the unflattened BGP subtree, for every new leaf.

### Transformation Path

1. Config resolution binds the new leaves to the peer at the four augmentation
   sites the module already uses, combining levels by `ze:cumulative`.
2. `runIngressPolicyChain` runs the ordered ingress steps. The §11 scrub runs
   first and rewrites the payload. The relation tag runs second, on the scrubbed
   payload.
3. The route is cached and dispatched with the modified payload.
4. On forward, per destination, the egress filter reads the parsed community
   policy, decides suppression, and records a prepend operation when the route
   requests one.
5. `buildModifiedPayload` plans, sizes and writes the rebuilt UPDATE.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ filter | payload bytes in, modified payload bytes out | No |
| Filter ↔ role plugin | `resolvePeerRole` read under `filterMu` | No |
| Reactor ↔ accumulator | `mods.Op` and `OpGen` operations, applied after all filters pass | No |
| Config ↔ runtime | unflattened BGP subtree walked by the plugin | No |

### Integration Points

- `filterapi.Register` with an explicit `Stage`, which is where ordering is
  declared. Code position does not order filters.
- `filterapi.RegisterAttrModHandler` for the large-community attribute, already
  registered by `filter_community`.
- `resolvePeerRole` for the relation parameter.
- `wireu.CommunityPolicy` for the egress decisions.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No operator relies on `RS:101..103:X` being ignored | `PrependCount` has no non-test caller, so the request has never reached the wire | An upgrade starts prepending where it did not before, changing path selection at clients | Owner confirmation plus a release note | unvalidated |
| A-2 | `resolvePeerRole` answers for every peer with a role configured or advertised | `internal/component/bgp/plugins/role/otc.go`, read 2026-08-08 | The relation tag is absent or wrong for some peers | Unit test over the five role values plus the empty case | unvalidated |
| A-3 | A dynamic-group peer reaches the ingress filter with no role config | `plan/deferrals/ad-hoc-2026-07-27-423eaa77.md` records exactly this | Dynamic peers silently get no relation tag | Read `extractPeerRoleConfigs`, then a functional test with a dynamic peer | unvalidated |
| A-4 | Function 4 suppression composes with existing egress filters without reordering them | `buildOrderedEgressSteps` resolves order from the declared stage | A suppression decision runs before a filter that would have modified the route | Read the ordered egress builder, then an ordering test | unvalidated |
| A-5 | Ingress scrubbing does not break the route-server rail, which reads communities after ingress | The scrub is peer-scoped and eBGP-only | A route server scrubs the control communities its clients send it | Functional test: a client sends a control community and the route server still acts on it | unvalidated |
| A-6 | `source_keyword_count` is the only reader that counts the RFC 2119 boilerplate as a gate input | Read of `check_new_summaries` and both counters on 2026-08-08 | Narrowing it leaves the red in place, or moves it elsewhere | Run `make ze-rfc-check` before and after | unvalidated |
| A-7 | New leaves under the existing `ingress/community` container inherit the four augment sites without a new augment statement | `ze-filter-community.yang` uses one grouping at four sites | The new leaves are settable at some levels and not others | Read the generated schema, then a config test setting the leaf at group level | unvalidated |
| A-8 | The keep-list can be expressed without changing `strip` semantics | `strip` is an exact whole-value match today and a wildcard there would change every existing config | Operators find `strip` silently matching more than before | Separate container, plus a test that an existing `strip` set still matches exactly | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A reviewer reads the ingress tag as a per-destination write and objects that the fan-out dedup is defeated | Review comment citing the 416ns rebuild | The spec states the tag is written once on ingress. Add the same sentence to the code comment |
| R-2 | Narrowing `source_keyword_count` weakens a compliance gate for every RFC | `make ze-rfc-check` passes for a summary that should fail | Exclude only the key-words sentence, and add a fixture proving a genuine MUST still fires the gate |
| R-3 | The keep-list defaults let a forged Function 4 through, so a neighbour suppresses a route toward a third party | A customer suppresses a route it does not originate | Bound the parameter to ASNs the peer may name, and default the keep-list conservatively |
| R-4 | Conditional prepend breaks a client that cannot accept a non-adjacent leftmost AS | A client session drops or rejects routes after upgrade | The prepend is opt-in per route by construction. Absent the community nothing changes |
| R-5 | Two rails carry different function numbers and an operator assumes one config governs both | A support question, or a tag that does nothing | Separate leaves per rail, and one documentation table showing both |
| R-6 | Removing the large control communities changes what clients see, and a client may key on them | Client-side policy stops matching after upgrade | The removal makes the two attribute codes consistent. Announce it as a behavior change |
| R-7 | The scrub runs on a route the route server must forward transparently | An RFC 7947 tagged test goes red | The scrub is bound per peer and is not enabled on route-server sessions by default |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes suppressed to the wrong peer, or an AS_PATH prepended where it should not be. Both are wire-visible at every client of a route server and change path selection downstream. A wrong scrub silently deletes operator signalling |
| How is it reverted? | Single commit revert for the code. Not revertible for routes peers already accepted, so the prepend and the suppression need their functional tests before release |
| Who else touches this path? | spec-fixit-send-community-suppress-ignored, closed 2026-08-12 (egress community suppression: `filterapi.LastSetOrSuppress` is the one fold, and `genericCommunityHandler` drops a community attribute whose last Set-or-Suppress op is a Suppress), spec-fixit-rs-community-strip-arity, closed 2026-08-10 (strip arity on this exact rail: a Remove buffer may carry a whole number of wire values, `filter_community.wholeValues` judges it, and a violation is refused and counted as `ze_bgp_attr_mod_remove_buffer_refused_total`), spec-fixit-otc-src-role-meta-fallback, closed 2026-08-11 (role resolution), `plan/spec-fixit-dynamic-group-peer-config.md` (dynamic peers get no plugin config) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer config enables relation tagging | → | ingress relation filter | `TestRelationTagWiring`, then `test/plugin/community-relation-tag.ci` |
| peer config enables own-GA scrub | → | ingress scrub filter | `TestScrubOwnGAWiring`, then `test/plugin/community-scrub-own-ga.ci` |
| route carries own-GA Function 4 naming the destination ASN | → | egress suppression | `TestFunction4SuppressWiring`, then `test/plugin/community-function4-noexport.ci` |
| route carries own-GA Function 6 naming the destination ASN | → | egress prepend generator | `TestFunction6PrependWiring`, then `test/plugin/community-function6-prepend.ci` |
| route-server client sends the prepend control community | → | `PrependCount` reaches the AS_PATH writer | `TestRSPrependReachesWire`, then `test/plugin/rs-prepend-control-community.ci` |
| route-server forward of a route carrying large control communities | → | code 32 removal operation | `TestRSStripsLargeControlCommunities`, then `test/plugin/rs-control-community-strip.ci` |
| eBGP peer sends a route carrying BLACKHOLE | → | ingress propagation guard | `TestBlackholePropagationGuardWiring`, then `test/plugin/community-blackhole-noexport.ci` |
| new summary with zero gated MUSTs over a 2119-boilerplate source | → | `check_new_summaries` | `test_new_summary_boilerplate_only_passes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | eBGP peer whose resolved role is provider, relation tagging enabled | The received route carries large community `<localAS>:3:4` |
| AC-2 | eBGP peer whose resolved role is customer | The received route carries `<localAS>:3:2` |
| AC-3 | eBGP peer whose resolved role is peer | The received route carries `<localAS>:3:3` |
| AC-4 | Peer whose resolved role is rs or rs-client | No relation tag is written and the forwarded bytes stay identical |
| AC-5 | eBGP peer sends a route already carrying `<localAS>:3:2` | The forged value is removed before the relation tag is written, and the stored route carries exactly one Function 3 value |
| AC-6 | eBGP peer sends `<localAS>:<keptFunction>:<asn>` where the function is in the keep-list | The value survives ingress unchanged |
| AC-7 | eBGP peer sends `<localAS>:<function>:<x>` where the function is not in the keep-list | The value is removed on ingress |
| AC-8 | iBGP peer sends any own-GA community | Nothing is scrubbed and no relation tag is written |
| AC-9 | Route carries `<localAS>:4:<X>`, forwarded to a general eBGP peer whose ASN is X | That peer does not receive the route, and a peer whose ASN is not X does receive it |
| AC-10 | Route carries `<localAS>:6:<X>`, forwarded to a general eBGP peer whose ASN is X | That peer receives the route with `<localAS>` prepended once, and a peer whose ASN is not X receives an unchanged AS_PATH |
| AC-11 | Route-server client A sends `<rsAS>:101:<clientB-ASN>` | Client B receives the route with the route-server ASN prepended once. A client not named receives a byte-identical AS_PATH |
| AC-12 | Route-server forward of a route carrying any large control community whose Global Administrator is the route server | The value is removed before the route reaches any client |
| AC-13 | Relation function number leaf set to a value other than 3 | The tag is written with that function number, and the shipped default remains 3 |
| AC-14 | Route-server rail with no explicit numbering config | `0`, `1` and `101` through `103` keep their current meaning |
| AC-15 | `rfc/not-enrolled.txt` after this spec lands | rfc7454 and rfc8195 both carry kind `non-normative` with a reason that states a property of the source text and does not judge what Ze owes |
| AC-16 | A new summary declaring zero gated requirements whose source contains MUST-level keywords only inside the RFC 2119 key-words sentence | `check_new_summaries` does not fail it, and a summary whose source contains a genuine MUST still fails |
| AC-17 | Route-server client sends a control community on a session where the §11 scrub is enabled | The route server still acts on the control community |
| AC-18 | An existing config using `strip` with a named large-community set | The set matches exactly the values it matched before this change |
| AC-19 | eBGP peer sends a route carrying BLACKHOLE (65535:666), propagation guard enabled | The stored route also carries NO_EXPORT, or NO_ADVERTISE when the leaf selects it, and the route is not advertised outside the local AS |
| AC-20 | The own-GA scrub runs over a route carrying BLACKHOLE, NO_EXPORT and NO_ADVERTISE | All three survive. The scrub reaches no community whose Global Administrator is 65535 |
| AC-21 | eBGP peer sends BLACKHOLE with the propagation guard disabled | No community is added, and the route keeps the bytes it arrived with |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Declares a peer a transit provider with `role import customer`, turns on relation tagging, then reads the RIB | config → role resolution → ingress filter → stored route | `test-community-relation-tag` |
| 2 | Sends a route to their upstream tagged so one competitor does not receive it | customer UPDATE → ingress keep-list → egress Function 4 → destination suppressed | `test-community-function4-noexport` |
| 3 | Asks their upstream to prepend once toward one peer | customer UPDATE → egress Function 6 → AS_PATH generator → wire | `test-community-function6-prepend` |
| 4 | Runs a route server and sends the Euro-IX prepend community | client UPDATE → RS rail → `PrependCount` → AS_PATH → other client | `test-rs-prepend-control-community` |
| 5 | Runs a route server and expects its own control tags not to leak | client UPDATE → RS rail → code 8 and code 32 removal → other client | `test-rs-control-community-strip` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRelationParameterFromRole` | `internal/component/bgp/plugins/filter_community/relation_test.go` | The five role values and the empty case map to the RFC 8195 parameters, with rs and rs-client producing no tag | |
| `TestScrubKeepList` | `internal/component/bgp/plugins/filter_community/scrub_test.go` | Own-GA values outside the keep-list are removed and values inside it survive | |
| `TestScrubIgnoresForeignGA` | `internal/component/bgp/plugins/filter_community/scrub_test.go` | A community whose Global Administrator is another ASN is never removed, per §11 bullet two | |
| `TestScrubThenTagOrder` | `internal/component/bgp/plugins/filter_community/filter_test.go` | A forged Function 3 does not survive the pass | |
| `TestStripSetStillExact` | `internal/component/bgp/plugins/filter_community/filter_test.go` | Adding the scrub does not make `strip` match more than before (AC-18) | |
| `TestPrependCountReachesGenerator` | `internal/component/bgp/wireu/community_test.go` | The recorded count produces the right number of AS_PATH entries | |
| `TestStripControlCommunitiesLarge` | `internal/component/bgp/wireu/community_test.go` | Code 32 control values are returned for removal, and foreign-GA large values are not | |
| `TestRFC7999BlackholeFieldHasReader` | `internal/component/bgp/wireu/community_test.go` | `RFC7999Blackhole` drives the propagation guard, so the field is no longer write-only | |
| `TestWellKnownCommunitiesSurviveScrub` | `internal/component/bgp/plugins/filter_community/scrub_test.go` | BLACKHOLE, NO_EXPORT and NO_ADVERTISE all survive the own-GA scrub (AC-20) | |
| `test_new_summary_boilerplate_only_passes` | `scripts/dev/rfc_requirements_test.py` | A 2119-boilerplate-only source with zero gated requirements passes, and a genuine MUST still fails | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Function number leaf | 0-4294967295 | 4294967295 | N/A | a value above 32 bits is rejected at config |
| Relation parameter | 1-4 | 4 | 0 is reserved and never written | 5 is never written |
| Prepend count from the control community | 1-3 | 3 | function 100 is not a prepend request | function 104 is not a prepend request |
| Global Administrator for the scrub | 0-4294967295 | 4294967295 | N/A | N/A |
| Route-server ASN for standard-community match | 0-65535 after truncation | 65535 | N/A | a 4-byte ASN cannot express `RS:X`, which is a property of RFC 1997 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-community-relation-tag` | `test/plugin/*.ci` | Operator sees the relation community on a route from a declared provider | |
| `test-community-scrub-own-ga` | `test/plugin/*.ci` | A forged relation tag from a neighbour does not survive | |
| `test-community-function4-noexport` | `test/plugin/*.ci` | A route is withheld from one peer and delivered to another | |
| `test-community-function6-prepend` | `test/plugin/*.ci` | One peer sees a prepended AS_PATH and another does not | |
| `test-rs-prepend-control-community` | `test/plugin/*.ci` | The Euro-IX prepend community changes the AS_PATH one client sees | |
| `test-rs-control-community-strip` | `test/plugin/*.ci` | Large control communities do not reach clients | |
| `test-community-blackhole-noexport` | `test/plugin/*.ci` | A blackhole request from a customer does not leave the local AS | |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-community-relation-frr` | `test/interop/scenarios/` | FRR | A conforming peer accepts and displays the relation community Ze writes | |
| `NN-community-function6-bird` | `test/interop/scenarios/` | BIRD | The prepended AS_PATH Ze produces is accepted and changes the peer's best-path choice | |

Vacuity note, per `ai/rules/interop-and-goal-validation.md`: a conforming peer
accepts any large community, so a scenario that only checks the session stays up
proves nothing. Each scenario asserts the value in the peer's own route output,
and each must be shown to FAIL when its producing change is reverted. The
Function 6 scenario asserts a path-length change at the peer, which a tag alone
cannot produce.

## Files to Modify

- `internal/component/bgp/wireu/community.go` - large control community removal;
  the disposition of `RFC7999Blackhole`
- `internal/component/bgp/reactor/forward_rs.go` - code 32 removal; apply the
  prepend
- `internal/component/bgp/reactor/reactor_api_forward.go` - the same two, plus
  Functions 4 and 6 off the route-server gate
- `internal/component/bgp/plugins/filter_community/config.go` - derived source and
  Global Administrator scoped match
- `internal/component/bgp/plugins/filter_community/filter.go` - relation tag and
  keep-list scrub
- `internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang` -
  new leaves inside the existing grouping, so all four augment sites inherit them
- `internal/component/bgp/plugins/filter_community/register.go` - stage placement
- `scripts/dev/rfc_requirements.py` - `source_keyword_count`
- `rfc/not-enrolled.txt` - both disposition rows
- `docs/guide/bgp-policy.md` - the numbering table for both rails
- `docs/features/rfc-status.md` - the RFC 7947 row gains the conditional prepend
  note

## Files to Create

- `internal/component/bgp/plugins/filter_community/relation.go` - the role to
  parameter mapping
- `internal/component/bgp/plugins/filter_community/scrub.go` - the keep-list match
- `test/plugin/community-relation-tag.ci` and the five siblings named above
- `test/interop/scenarios/NN-community-relation-frr/` and the BIRD sibling

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang`, inside `community-filter-fields` so all four sites inherit |
| YANG validation constraints | Yes | Function-number leaves take a `range`; the relation enable is a boolean; the keep-list is a leaf-list of function numbers. The existing `value` leaf-lists are unconstrained `type string` and gain a `pattern` |
| YANG custom validators | Yes | A `ze:validate` that refuses a keep-list containing the relation function, which would permit a forged tag to survive |
| CLI commands/flags | No | The feature is configuration, not a new verb |
| CLI grammar (keyword before value) | N-A | No new verb |
| Editor autocomplete | Yes | Automatic for the enum and boolean leaves |
| Functional test for new RPC/API | Yes | The six `.ci` files listed above |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | No | No new file path, socket, service, module, port, procfs entry, binary or certificate |
| Prometheus counters/metrics | Yes | Counters for values scrubbed, tags written, destinations suppressed and prepends applied. Names and labels fixed at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability or attribute code. RFC 8092's LARGE_COMMUNITY already exists |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | No | No new verb |
| 4 | API/RPC added/changed? | No | No new RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, the `filter_community` entry |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md` |
| 7 | Wire format changed? | No | RFC 8092 encoding is unchanged |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7947.md` conditional-deviation note, `docs/features/rfc-status.md`, `rfc/not-enrolled.txt` |
| 10 | Test infrastructure changed? | No | Existing runners cover the new tests |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, which names community capabilities |
| 12 | Internal architecture changed? | No | No new layer or seam |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md`, if the egress decision travels in meta |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md`, since the filter gains registered behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `wireu/community.go` and the two forward paths |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bgp-policy.md` documents the control-community spellings and must show both rails |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the entry points and write the
   failing wiring tests
   - Tests: the seven names in the Wiring Test table
   - Files: `register.go`, the two new filter files as stubs, the YANG leaves
   - Verify: every entry point exists and is reachable, and every wiring test
     fails because the feature is a stub
2. **Phase: Ingress relation tag** -- AC-1 to AC-4, AC-13
   - Tests: `TestRelationParameterFromRole`, `test-community-relation-tag`
   - Files: `relation.go`, `config.go`, the YANG
   - Verify: the five role values map correctly and rs and rs-client write nothing
3. **Phase: Ingress §11 scrub** -- AC-5 to AC-8, AC-17, AC-18, AC-20
   - Tests: `TestScrubKeepList`, `TestScrubIgnoresForeignGA`, `TestScrubThenTagOrder`,
     `TestStripSetStillExact`, `TestWellKnownCommunitiesSurviveScrub`
   - Files: `scrub.go`, `config.go`, the YANG validator
   - Verify: order is scrub then tag, foreign Global Administrators are untouched,
     well-known communities survive, and the route-server rail still sees its
     control communities
3b. **Phase: RFC 7999 §3.2 propagation guard** -- AC-19, AC-21
   - Tests: `TestRFC7999BlackholeFieldHasReader`, `test-community-blackhole-noexport`
   - Files: `filter.go`, `config.go`, the YANG
   - Verify: `RFC7999Blackhole` has a production reader, and the guard adds nothing
     when it is disabled
4. **Phase: Route-server rail repair** -- AC-11, AC-12, AC-14
   - Tests: `TestPrependCountReachesGenerator`, `TestStripControlCommunitiesLarge`,
     `test-rs-prepend-control-community`, `test-rs-control-community-strip`
   - Files: `wireu/community.go`, `forward_rs.go`, `reactor_api_forward.go`
   - Verify: `TestReactorForwardRSTransparent` stays green as the negative case
5. **Phase: General rail Functions 4 and 6** -- AC-9, AC-10
   - Tests: `TestFunction4SuppressWiring`, `TestFunction6PrependWiring`, the two
     functional tests, the two interop scenarios
   - Files: `reactor_api_forward.go`, the egress filter
   - Verify: each interop scenario fails when its producing change is reverted
6. **Phase: Ledger and gate** -- AC-15, AC-16
   - Tests: `test_new_summary_boilerplate_only_passes`
   - Files: `scripts/dev/rfc_requirements.py`, `rfc/not-enrolled.txt`
   - Verify: `make ze-rfc-check` passes, and a genuine MUST still fires the gate

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path with no broken link |
| Correctness | Scrub runs before tag. The relation parameter derives from the peer's role, not the local role. rs and rs-client write no tag |
| Correctness | The prepend is applied only when the requesting community is present, and never otherwise |
| Naming | The YANG leaf names use the RFC 8195 vocabulary and do not invent a third word for a function |
| Data flow | The tag is written once on ingress, not per destination, so the fan-out dedup is not defeated |
| Vacuous tests | `PrependCount` no longer has test-only callers. Every assertion over it drives a wire outcome |
| Rule: `ai/rules/evidence.md` | The keep-list fails closed. An unrecognised function is scrubbed, never kept |
| Rule: `ai/rules/rfc-compliance.md` | The RFC 7947 conditional deviation is recorded against `RFC7947-x-1` with its gate stated |
| Rule: `ai/rules/testing.md` | No existing tagged test is weakened. `TestReactorForwardRSTransparent` keeps its assertions |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `PrependCount` has a non-test caller | `grep -rn "PrependCount" internal/ --include "*.go"` shows a production call site |
| Large control communities are removed | `grep -rn "AttrModRemove" internal/component/bgp/reactor` shows an operation for the large-community code |
| Both disposition rows say `non-normative` | `grep -E "^rfc(7454|8195)" rfc/not-enrolled.txt` |
| The RFC gate is green | `make ze-rfc-check` |
| Every functional test exists | `ls test/plugin/community-*.ci test/plugin/rs-*.ci` |
| The interop scenarios discriminate | Revert each producing change and confirm the scenario fails |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The keep-list is a denylist inversion. Confirm an unknown function is scrubbed rather than kept |
| Forgery | A neighbour cannot cause a relation tag of its own choosing to be stored. AC-5 is the test |
| Third-party denial | A neighbour cannot use Function 4 to suppress a route it does not originate toward an unrelated AS. Bound the parameter |
| Resource exhaustion | A route carrying many own-GA values must not cause an unbounded rebuild. Confirm the edit-set digest bound is respected |
| Guard failing open | When the role cannot be resolved, no tag is written and nothing is scrubbed. Confirm that is the coded behavior and that it is logged |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Interop test passes when its change is reverted | The test is vacuous. Return to the test plan and assert a peer-visible outcome |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The egress half of RFC 8195 was already modelled in `wireu.CommunityPolicy` and
  half-wired. Reading the producer rather than the config surface is what found
  it. A search of the documentation for "does Ze support Function 6" answers no,
  and a search of the type answers yes.
- A green test suite over a symbol with no production caller is the failure this
  spec corrects twice: once for the prepend, once for the blackhole field.
- `source_prose_keyword_count` already documents the invariant its neighbour
  breaks. The comment says a genuinely non-normative document shows zero
  uppercase hits. RFC 7454 shows four, all inside the sentence that defines the
  words. The fix restores an assumption the file already states.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extend `filter_community` rather than add a plugin | A dedicated RFC 8195 plugin; expansion at config-resolution time | One plugin keeps ownership of attribute 32, so no inter-plugin ordering becomes operator-visible. Config expansion cannot express the §11 scrub, whose match set is unbounded, and cannot do per-destination egress actions. Owner decision, 2026-08-08 |
| Function numbers are configurable, defaulting to 3, 4 and 6 on the general rail and to the existing numbers on the route-server rail | Hard-code RFC 8195 numbers everywhere; accept both spellings with no config | RFC 8195 §3.2 and §4.1.1 say an AS "could assign" the number, so it is a convention. Ze already ships 0, 1 and 101 to 103 and operators send them. Hard-coding would silently change the meaning of live configuration. Owner decision, 2026-08-08 |
| The prepend is applied, gated on the requesting community | Delete the dead code; implement on the general rail only | RFC 7947 §2.2.2.1 is a SHOULD NOT whose stated purpose is compatibility with clients that cannot accept a non-adjacent leftmost AS. A client that sends the community has asked for the modification. Owner decision, 2026-08-08 |
| The relation parameter derives from `resolvePeerRole` | Read the configured `role/import` leaf directly | The resolver already prefers a received Role capability over configuration and already holds the RFC 9234 Table 2 complement. Reading the leaf would duplicate both and would disagree with the resolver when a peer advertises a role |
| The Global Administrator is the per-peer local AS | A system-wide ASN leaf, as the VyOS request proposes | `PeerFilterInfo.LocalAS` already carries the value, and a system-wide setting is wrong for any peer using a local-AS override |
| No relation tag on rs or rs-client sessions | Map both to parameter 3, as the VyOS request suggests | RFC 7947 requires route-server transparency, and writing an attribute at a route server is the behavior it forbids. Parameter 3 would in any case be a guess |
| §11 is implemented as a keep-list | A denylist of known-bad functions | §11's own text is a scrub with a carve-out for what customers may signal. A denylist fails open for any value a neighbour invents, which is what §11 exists to prevent |
| The scrub gets its own container rather than a wildcard in `strip` | Allow wildcards in the existing `strip` leaf-list | `strip` is an exact whole-value match today. Admitting a wildcard there changes the meaning of every existing config that uses it |

## Known Limitations

- RFC 8195 Function 3 parameter 1, route originated internally, is not written. It
  applies to locally originated routes rather than to any peer, so it belongs with
  route origination rather than with a peer filter.
- RFC 8195 §4.4's large-community form of route-server distribution control
  (`RS:0:X` and `RS:1:X`) is not added, so a 4-byte-ASN route server still cannot
  express `RS:X` unambiguously, because `StripControlCommunities` matches the low
  sixteen bits. Recorded as a deferral row with a destination.
- RFC 8195 Functions 5, 7 and 8 to 12 (location-based actions and LOCAL_PREF
  manipulation) are out of scope. §4.3.3 and RFC 4264 make the LOCAL_PREF family a
  separate design problem.
- Honouring the BLACKHOLE community is not done here. This spec adds the RFC 7999
  §3.2 propagation guard only. RFC 7999 §3.3's two conditions for accepting and
  honouring a blackhole announcement, and the traffic discard itself, are child 6
  (`plan/spec-bcp194-6-blackhole.md`). §3.1 makes ignoring the community a
  conformant choice, so the guard commits Ze to nothing further.
- TCP-AO, prefix-list defaults, role-derived policy and route flap dampening are
  the other children of the umbrella and are not touched here.

## Open Questions

| # | Question | Status |
|---|----------|--------|
| Q-1 | Is `CommunityPolicy.RFC7999Blackhole` wired, or removed as dead code? | SETTLED 2026-08-08, owner brought RFC 7999 into scope. The field is WIRED, and it gains its reader here: it drives the §3.2 propagation guard (AC-19). Honouring the community, which means discarding traffic, is child 6. The field stops being write-only without this spec taking on the blackhole feature |

## RFC Documentation (Scope: protocol)

Add the quoted requirement above each enforcing site. The sites are the relation
mapping (RFC 8195 §3.2), the scrub (RFC 7454 §11), the suppression (RFC 8195
§4.1.1), the prepend (RFC 8195 §4.2.1), and the conditional deviation from RFC
7947 §2.2.2.1 together with the condition that gates it.

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-21 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features, each shown to fail when its producing change is reverted

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
