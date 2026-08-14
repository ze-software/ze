# Spec: fixit-dynamic-group-peer-config

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-implement at stage 10.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Updated | 2026-08-14 |

<!-- Phase count went 4 -> 5 on 2026-08-13. The four keying plugins cannot be
     finished at the altitude of configjson alone (see Implementation Steps),
     so phase 3 was cut into three: blackholecfg's consumers, role's OTC and
     RPC contract, and rpki + filter_community. -->

**Landed so far (phases 1 and 2).** The `configjson` delivery layer and its
tests; the capability path end to end, proven by
`test/plugin/dynamic-peer-gets-group-role-capability.ci`; and `blackholecfg`
accepting and carrying a dynamic group's template. Remaining: phases 3, 4 and 5,
which are AC-5, AC-6, AC-7 and AC-8.

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** A peer created from a dynamic group receives NO per-peer plugin
config. For `bgp-role` this means no Role capability is advertised and no RFC 9234
OTC gate runs for that peer. `bgp-rpki` has the identical limitation.

**Producer.** Such peers establish with real addresses but have no entry in the
`peer` list, so `configjson.ForEachPeer`
(`internal/component/bgp/configjson/traverse.go`) never visits them -- it iterates
the peers map, and a dynamic-group peer is not in it. Neither the group's config nor
any peer's config is therefore delivered.

**Two corrections to this spec's own record, made 2026-08-13 and verified by
grep.** The package is `internal/component/bgp/configjson`, not
`internal/component/config/configjson`, which does not exist. And `ForEachPeer` has
**11** non-test call sites, not the 2 recorded under Blast Radius below: `role`,
`rpki`, `blackholecfg`, `filter_community`, `filter_irr`, `filter_family`,
`softver`, `gr`, `gr_llgr`, `llnh` and `hostname`.

**Why this is NOT the keying bug.** A sibling defect -- role config keyed by peer NAME
when no remote IP resolves, while all readers key by ADDRESS -- was fixed on
2026-07-27 in `internal/component/bgp/plugins/role/config.go`. That fix rejects
unreachable keys loudly. This is different and remains: the config never reaches the
delivery layer at all, so there is no key to get wrong.

**Blast radius.** Every plugin that consumes per-peer config through `ForEachPeer`.
Recorded here as `bgp-role` and `bgp-rpki`; the real count is 11, enumerated above.

**Severity.** Silent under-enforcement of a security-relevant policy. An operator who
configures a role on a dynamic group gets no error and no enforcement.

**Goal.** Per-peer config reaches peers created from a dynamic group, or the config
is rejected at verify with an error naming the group (`ai/rules/protocol.md`).
A half-fix that resolves only one plugin is explicitly out of scope -- the delivery
layer is the right altitude.

**Design note captured at discovery.** A real fix likely means matching a peer's
address against the group's range at lookup time rather than at delivery time, which
changes the shape of the delivered config rather than just its contents. That is why
this was not attempted as a drive-by.

**Provenance.** Found 2026-07-27 while closing the three OTC role-resolution holes
(handover 22). Documented as the one remaining known limit in
`docs/architecture/meta/role.md`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/configjson/traverse.go` - `ForEachPeer` visits
  `bgpTree["peer"]` and `bgpTree["group"][*]["peer"]`. A dynamic group carries NO
  `peer` list, so the `groupMap["peer"]` type assertion fails and the group is
  skipped entirely. The group's own key (its NAME) is discarded by the
  `for _, groupData := range groupsMap` loop, so no caller can key on it.
  → Constraint: the spec's stated path `internal/component/config/configjson/` does
  not exist. The package is `internal/component/bgp/configjson`.
- [ ] `internal/component/bgp/config/resolve.go` - `isDynamicGroup` is the
  authoritative marker: `connection > remote > ip == "dynamic"`. `ResolveBGPTree`
  injects `group-name` into each STATIC grouped peer's resolved map.
  → Constraint: the same marker must decide the new visit, so one definition of
  "dynamic group" governs both the reactor and the plugin traversal.
- [ ] `internal/component/plugin/server/startup.go` - `BuildPluginConfigSections`
  serializes the RAW config tree. The plugin JSON is the operator's document, so a
  peer that does not exist in it cannot be enumerated from it.
  → Decision: enumerate-at-config-time cannot see a dynamic member by construction,
  so making `ForEachPeer` yield dynamic MEMBERS is not reachable. It can only yield
  the TEMPLATE.
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - `buildDynamicPeerSettings`
  sets `Name` to `"dyn-<addr>"`, `Address` to the remote address, and `GroupName` to
  the group's name. Of the three identities a plugin can key on, only GroupName
  exists in the config document.
  → Decision: the group name is the one identity shared by config time and runtime.
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `PeerFilterInfo` already
  carries `GroupName`, and all seven build sites populate it (`peer_forward_facts.go`,
  `reactor_notify.go`, `reactor_api_forward.go`, `reactor_api_batch.go`,
  `reactor_api_forward_batch.go`, `reactor_api_relay.go`, `forward_rs.go`). No plugin
  reads it for config lookup.
  → Constraint: no reactor change is needed to carry the identity; only to READ it.
- [ ] `internal/component/bgp/reactor/peer.go` - `getPluginCapabilities` already
  falls back from `settings.Name` to `settings.Address.String()`. A third fallback is
  the same mechanism, not a new one.
- [ ] `internal/component/plugin/registration.go` - `AddPluginCapabilities` stores
  each `CapabilityDecl.Peers` entry as an exact string key in `peerCaps`, and
  `GetCapabilitiesForPeer` reads it back by exact string. No pattern matching exists,
  so a group name works there as a key with no change to the injector.

**Behavior to preserve:**
- A statically configured peer's config keying is unchanged: `PeerRemoteIP` first,
  the config-map key as fallback when it parses as an address (`role/config.go`).
- `blackholecfg.Parse` still REFUSES a stated blackhole block whose peer has no
  usable remote IP. A dynamic group is not that case: it has a group name.
- Every existing `ForEachPeer` closure body keeps its bgp → group → peer merge order.
- A peer-level statement still wins over its group's, for dynamic and static alike.

**Behavior to change:**
- `ForEachPeer` visits a dynamic group once, as a peer visit with a nil peer map.
- Plugins key that visit under the GROUP NAME and fall back to it at lookup.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config: a `bgp` group whose `connection > remote > ip` is `dynamic` and
  whose `connection > remote > range` names the prefixes it accepts.
- Format at entry: the raw config tree, serialized to JSON by
  `config.BuildPluginConfigSections` and delivered once at Stage 2 `OnConfigure`.

### Transformation Path
1. `config.BuildPluginConfigSections` serializes the `bgp` root to JSON
   (`internal/component/plugin/server/startup.go`).
2. The plugin's `OnConfigure` callback parses it with `configjson.ParseBGPSubtree`.
3. `configjson.ForEachPeer` walks the subtree and yields each peer with its
   enclosing group map. **Today a dynamic group yields nothing.**
4. The plugin merges bgp → group → peer levels into its own config value and stores
   it under a key derived from `configjson.PeerRemoteIP`.
5. At decision time the plugin looks that key up from `PeerFilterInfo.Address`
   (`role.getFilterConfig`, `rpki`, `blackholecfg`) or from `PeerFilterInfo.Name`
   (`filter_community`), or the reactor looks up injected capabilities from
   `PeerSettings.Name` then `.Address` (`reactor.Peer.getPluginCapabilities`).
   **A dynamic peer misses at every one of these.**

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | `bgp` subtree JSON at Stage 2 `OnConfigure`; `CapabilityDecl.Peers` back | Yes |
| Reactor ↔ Filter | `filterapi.PeerFilterInfo` (Address, Name, GroupName) | Yes |

### Integration Points
- `configjson.ForEachPeer` - the one traversal all 11 non-test callers share.
- `filterapi.PeerFilterInfo.GroupName` - already populated, not yet read for config.
- `reactor.Peer.getPluginCapabilities` - already a fallback chain, gains one link.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The fix is inside `ForEachPeer` and the existing lookup chains; no plugin gains a second config source |
| No unintended coupling (components stay isolated) | Yes | Plugins learn the group name from `configjson` and `PeerFilterInfo`, both existing seams |
| No duplicated functionality (extends existing, does not recreate) | Yes | One traversal keeps one visitor; the lookup fallback extends the existing name→address chain rather than adding a second resolver |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The group map is passed by reference exactly as it is today |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugins.md`) | Yes | No plugin is named in a core package; `configjson` gains no plugin spelling |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The plugin config JSON is the RAW operator tree, so a dynamic member cannot appear in it | `BuildPluginConfigSections` in `plugin/server/startup.go`; `ForEachPeer` walks `group.<name>.peer`, the raw shape | Making `ForEachPeer` yield members would be reachable and would be the simpler fix | read the producer | confirmed |
| A-2 | `PeerSettings.GroupName` is set for a dynamic peer | `buildDynamicPeerSettings` writes `ps.GroupName = dg.GroupName` | The group name is not an identity the runtime carries, and the whole design fails | read the producer | confirmed |
| A-3 | `PeerFilterInfo.GroupName` is populated at every filter decision site | all seven build sites in `reactor/` set it | Filters could not fall back to the group, and reactor changes would be in scope | read all seven producers | confirmed |
| A-4 | The capability injector matches `Peers` entries by exact string, so a group name works as a key with no injector change | `AddPluginCapabilities` and `GetCapabilitiesForPeer` in `plugin/registration.go` | `CapabilityDecl` would need a selector type, widening the change to the plugin RPC contract | read both producers | confirmed |
| A-5 | `connection > remote > ip == "dynamic"` is the one marker of a dynamic group | `isDynamicGroup` in `bgp/config/resolve.go` | The traversal and the reactor would disagree about which groups are dynamic | read the producer | confirmed |
| A-6 | `ForEachPeer` has 11 non-test callers, not the 2 the spec recorded | `grep -rn ForEachPeer --include=*.go` over the tree | The blast radius is understated and the package is mis-sized | grep | confirmed |
| A-7 | A group name and a peer name cannot collide in one config | `validatePeerName` / `validateGroupName` and the duplicate checks in `ResolveBGPTree` | A group's template could shadow a peer's config under one key | read `resolve.go`, then a test | **broken** |

**A-7 is broken, and it decides the key.** `ResolveBGPTree` collects every peer name
into one `peerNames` map and refuses a duplicate, but a group name only goes through
`validateGroupName`. Nothing compares the two namespaces, so `bgp { peer ix {...}
group ix {...} }` is accepted. A bare string key would therefore let a dynamic
group's template answer a lookup meant for a peer of the same name, which is the
zero-value trap of `ai/rules/evidence.md` wearing a different hat: the caller cannot
tell whose config it received. The key is a typed pair (`PeerConfigKey`) instead of a
prefixed string, so the two namespaces cannot collide by construction rather than by
convention. On the capability path, where the RPC contract fixes the key as a plain
string, the group entry is consulted only for a peer whose `PeerSettings.IsDynamic`
is set, which closes the same hole from the other side.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The visitor signature change touches 11 callers, and a caller that ignores the new dynamic visit keeps the defect silently | a plugin still misses on a dynamic member after the change | `TestEveryForEachPeerCallerIsAccountedFor` (`configjson/dynamic_group_test.go`) scans `internal/`, `cmd/` and `pkg/` for non-test `configjson.ForEachPeer(` call sites and requires the set to equal `forEachPeerCallers`, which records for each one the keying helper it uses or the reason it keys nothing. A twelfth caller fails the gate until its author records either |
| R-2 | A plugin that today refuses a peer with no usable remote IP starts refusing the dynamic template instead | `blackholecfg.Parse` returns an error on a config that used to load | the template visit carries the group name as its key, and each refusing path is given that key before it reaches the address parse |

**R-3 FIRED too, in `filter_community`, and it is now closed.** The template visit
landed in `configs[peerName]`, a map keyed by peer NAME and read by `src.Name` /
`dest.Name`. `peerName` on a template visit IS the group's bare name, and
`ForEachPeer` visits groups AFTER standalone peers, so a config holding both
`peer ix` and `group ix` gave the peer the GROUP's community policy on every
ingress and egress decision. Measured: with the bare key, the peer's
`ingressTag` reads `["group-tag"]` instead of `["peer-tag"]`. The template is now
keyed by `configjson.CapabilitySelector`, the same prefixed selector the
capability path uses, and `configLabel` reads the prefix back so an error about a
group says `group ix` rather than `peer group:ix`. Covered by
`TestDynamicGroupTemplateDoesNotOverwriteAPeerOfTheSameName` and
`TestConfigErrorNamesAGroupAsAGroup`, both measured red against the bare key.

**One widening is deliberate and is NOT a regression: `filter_family`.**
`validateNoTearDownInExport` now runs over a dynamic group's own export chain,
which it never reached before, because a group with no `peer` list produced no
visit. A listen-range group stating `filter { export <tear-down-instance> }`
therefore fails config validation where it used to load. That refusal is correct:
the reactor DOES build every member's export chain from that group
(`peersAndDynamicGroups`, proven by `dynamic-peer-applies-group-filters.ci`), so
the invalid chain was being applied while the check that exists to refuse it could
not see it. This is the Goal's second branch -- "or the config is rejected at
verify with an error naming the group".

**R-2 FIRED, and it is now closed.** Phase 1 gave `ForEachPeer` the template visit
and left `blackholecfg.Parse` reading `PeerRemoteIP` for every visit, so a dynamic
group stating a `blackhole` block made the whole configuration fail to load:
`blackhole: peer "ix" states a blackhole block and has no usable remote IP
("dynamic")`. No test covered either the old behaviour or the new one, which is why
all 11 packages stayed green and the tree was correctly held back from commit.

Both states were wrong. Loading the block and doing nothing is the defect this spec
exists to fix; refusing it makes the canonical IXP route-server configuration an
error. `Parse` now returns `map[configjson.PeerConfigKey]Rule` and keys the template
under `configjson.GroupKey`, so the agreement is carried. Its four consumers read
address keys only, so a dynamic member does not yet RESOLVE it -- that is AC-7 and
phase 3, and it needs a group identity threaded through the RIB best-path decision.
The refusal is preserved for a NAMED peer with no usable remote IP, which is still
right: such a peer is never built from a template and has no address any consumer
can produce. Covered by `TestParseKeysADynamicGroupsBlockUnderTheGroup` and its
three siblings; measured to go red when the template branch is removed.
| R-3 | A group name used as a capability-injection key collides with a peer name | a peer receives a capability declared for a group | A-7 decides this; if names can collide the key gets a namespace prefix |
| R-4 | Two plugins now declare the same capability code for one group where they previously declared it per peer, tripping the conflict check | `AddPluginCapabilities` returns a conflict error at startup | the declaration is per group per code exactly as it was per peer per code, so the shape is unchanged; covered by a unit test |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config that loads today could be refused at verify (R-2), or a peer could receive another peer's capability (R-3). Both are startup-visible, not silent. The defect being fixed is itself silent under-enforcement of RFC 9234 and RPKI on every IXP route-server member |
| How is it reverted? | A single commit revert. No config migration: the operator's document is unchanged, only how it is read |
| Who else touches this path? | Six commits landed in this path on 2026-08-13 (`39e72ca6f`, `c8f1d589b`, `281cedcbc` among them). `281cedcbc` is the structural precedent this change follows |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `bgp` group whose `connection > remote > ip` is `dynamic` | → | `configjson.ForEachPeer` yields the group's template as one peer visit | `TestForEachPeerVisitsDynamicGroupTemplate` |
| The same group, read by a plugin's `OnConfigure` | → | `role.extractRoleCapabilities` emits a `CapabilityDecl` whose `Peers` names the GROUP | `TestRoleCapabilityDeclaredForDynamicGroup` |
| An inbound connection inside the group's range | → | `reactor.Peer.getPluginCapabilities` resolves injected capabilities by group name, gated on `IsDynamic` | `test/plugin/dynamic-peer-gets-group-role-capability.ci` (the seam for a unit test does not exist; see the TDD Test Plan) |
| The same connection, reaching Established | → | ze's OPEN carries RFC 9234 capability 9 with the group's stated role | `test/plugin/dynamic-peer-gets-group-role-capability.ci` |
| An UPDATE from that peer | → | `role.OTCIngressFilter` finds a role config through the group fallback | `TestGetFilterConfigFallsBackToGroup` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `bgp` subtree holding one dynamic group and no peers | `ForEachPeer` makes exactly one visit, carrying the group's name, the group's map, and a nil peer map |
| AC-2 | A dynamic group that also lists static peers | `ForEachPeer` visits each static peer AND the template, and the static peers keep the name and maps they get today |
| AC-3 | A group that is not dynamic | `ForEachPeer` makes no template visit, so a plain peer-group is untouched |
| AC-4 | A dynamic group stating `role import rs` | The Role capability is declared for the group, and a peer created from that group advertises capability 9 with the value `rs`, not a default and not absence |
| AC-5 | A dynamic group stating a role, and an UPDATE from a member | The RFC 9234 OTC ingress gate runs for that member, using the group's role |
| AC-6 | A dynamic group stating an RPKI action, and a member | The member's origin-validation actions come from the group's statement |
| AC-7 | A dynamic group stating a `blackhole` block, and a member | The member's blackhole rule is the group's stated communities and prefixes |
| AC-8 | A dynamic group and a static peer stating the same plugin leaves | Both resolve to the same effective plugin config, so the template diverges from a static peer nowhere it was not told to |
| AC-9 | A peer-level statement inside a static group | It still wins over the group's, for every plugin touched |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [e.g. "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForEachPeerVisitsDynamicGroupTemplate` | `internal/component/bgp/configjson/traverse_test.go` | AC-1: one visit, group name carried, nil peer map | pass |
| `TestForEachPeerVisitsStaticPeersOfADynamicGroup` | same | AC-2 | pass |
| `TestForEachPeerSkipsATemplateVisitForAPlainGroup` | same | AC-3 | pass |
| `TestPeerConfigKeySeparatesAGroupFromAPeerOfTheSameName` | same | A-7: the typed key cannot collide | pass |
| `TestPeerKeyRefusesAKeyNoReaderCanProduce` | same | the miss is visible, never a zero value | pass |
| `TestLookupPeerConfigPrefersThePeerOverItsGroup` | same | AC-9 | pass |
| `TestCapabilitySelectorSeparatesAGroupFromAPeerOfTheSameName` | same | A-7 on the one path whose contract fixes the key as a string | pass |
| `TestIsDynamicGroupMatchesTheConfigResolver` | same | one definition of "dynamic group" governs the traversal and the reactor | pass |
| `TestRoleCapabilityDeclaredForDynamicGroup` | `internal/component/bgp/plugins/role/config_keying_test.go` | AC-4 at the declaration: selector is the group, payload is the STATED role | pass |
| `TestRoleConfigForDynamicGroupIsNotKeyedByAnAddress` | same | A-7 at `role`: a peer and a group of one name keep separate entries | pass |
| `TestRoleConfigStillRejectsANamedPeerInheritingThePlaceholder` | same | the template branch is scoped to the template visit alone | pass |
| `TestParseKeysADynamicGroupsBlockUnderTheGroup` | `internal/component/bgp/blackholecfg/blackholecfg_test.go` | the regression: a dynamic group's `blackhole` block loads and is keyed by the group | pass |
| `TestParseKeepsBothAStaticPeerAndTheTemplateOfItsDynamicGroup` | same | AC-2 at `blackholecfg`; both configurations are in force | pass |
| `TestParseStillRefusesANamedPeerWithNoUsableRemoteIP` | same | the refusal is preserved where it is still right | pass |
| `TestParseYieldsNoTemplateForAPlainGroup` | same | AC-3 at `blackholecfg` | pass |
| `TestGetFilterConfigFallsBackToGroup` | `internal/component/bgp/plugins/role/dynamic_group_test.go` | AC-5 at the lookup, and AC-9 beside it | pass |
| `TestOTCIngressGateRunsForADynamicGroupMember` | same | AC-5 at the RFC 9234 Section 5 ingress decision | pass |
| `TestValidateOpenRolePairRunsForADynamicGroupMember` | same | the RFC 9234 Section 4.2 OPEN check, which no dynamic member reached | pass |
| `TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole` | same | strict mode stated on a listen-range group binds its members | pass |
| `TestReconfigureKeepsADynamicGroupMembersLearnedRole` | same | a member's learned role survives a reload that keeps its group | pass |
| `TestRPKIPeerActionsForDynamicGroup` | `internal/component/bgp/plugins/rpki/rpki_config_test.go` | AC-6 at the config: the template is keyed under the group, a named member keeps its own entry (AC-9), and the RFC 7999 agreement resolves for the template | pass |
| `TestBuildDecisions_DynamicGroupMemberUsesGroupAction` | `internal/component/bgp/plugins/rpki/rpki_action_test.go` | AC-6 at the RFC 6811 decision, with AC-9 and the peer/group key collision beside it | pass |
| `TestCarriesAgreedBlackholeResolvesThroughTheGroup` | `internal/component/bgp/plugins/rpki/blackhole_agreement_test.go` | AC-6 for the RFC 7999 Section 3.3 exemption | pass |
| `TestJSONRailCarriesTheGroupIntoTheValidationRequest` / `TestStructuredRailCarriesTheGroupIntoTheValidationRequest` | `internal/component/bgp/plugins/rpki/dynamic_group_test.go` | AC-6 delivery: rpki learns the group from the event on both rails, so the decision above is not fed by the test's own hand | pass |
| `TestOriginRevalidationKeepsTheGroupIdentity` / `TestASPATrackerKeepsTheGroupIdentity` | same | RFC 6811 Section 4 re-validation judges a member's route by the same actions that judged it on arrival | pass |
| `TestStatusCommand_PerPeerActions_NamesAGroupAsAGroup` | `internal/component/bgp/plugins/rpki/rpki_status_test.go` | a template is shown as a group, never as a peer whose address is a group name | pass |
| `TestRPKIDynamicGroupActionsMatchAStaticPeer` | `internal/component/bgp/plugins/rpki/rpki_config_test.go` | AC-8 at `rpki` | pass |
| `TestIngressFilterFallsBackToTheGroupsConfig` / `TestEgressFilterFallsBackToTheGroupsConfig` / `TestRelationIngressFilterFallsBackToTheGroupsConfig` | `internal/component/bgp/plugins/filter_community/dynamic_group_test.go` | a member resolves its group's community policy at all three registered readers | pass |
| `TestCommunityConfigStillPrefersThePeerOverItsGroup` | same | AC-9 at `filter_community` | pass |
| `TestDynamicGroupCommunityConfigMatchesAStaticPeer` | same | AC-8 at `filter_community` | pass |
| `TestBlackholeRuleForDynamicGroup` | `internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go` | AC-7 at the RIB decision: a member's best path is stamped from its group's rule, a session outside the group is not, and a peer's own rule beats its group's | pass |
| `TestBlackholeGroupIdentityArrivesOnAStructuredEvent` / `...OnAJSONEvent` / `...IsEmptyForAStandalonePeer` | `internal/component/bgp/plugins/rib/rib_blackhole_dynamic_test.go` | AC-7 delivery: the RIB learns the group from the event on both rails, so the decision above is not fed by the test's own hand | pass |
| `TestAnnounceBlackholeReachesAMemberOfADynamicGroup` / `...IsWithheldFromASessionOutsideTheGroup` | `internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go` | AC-7 on the send side: RFC 7999 Section 3.1's agreement resolves through the group | pass |
| `TestDynamicGroupPluginConfigMatchesAStaticPeer` | `internal/component/bgp/configjson/dynamic_group_test.go` | AC-8 at the delivery layer: one config states the same plugin leaves on a static peer and on a dynamic group, and both resolve to the same map | pass |
| `TestAPeerInsideADynamicGroupKeepsItsOwnConfig` | same | AC-9 at the delivery layer: the template and a named peer of the group are two keys, both in force | pass |
| `TestEveryForEachPeerCallerIsAccountedFor` | same | R-1's mitigation: the caller set is enumerated from source, so a new caller that ignores `PeerOrigin` fails the gate. Measured red by removing one row: it names the caller it did not expect | pass |

**`TestPluginCapabilitiesResolveByGroupName` was NOT written, and the reason is a
missing seam rather than a skipped test.** `Peer.getPluginCapabilities` reads
`r.api`, a concrete `*pluginserver.Server`, and that type exposes no route to load
a capability into its injector: `capInjector.AddPluginCapabilities` is reached only
from `engineStartupSink.onCapabilities`, an unexported method driven by a plugin
process. Building one in a reactor unit test costs more than it proves, and adding
an exported setter for a test is machinery the problem does not need
(`ai/rules/simplicity.md`). The link is proven at its entry point instead, on the
wire, by `dynamic-peer-gets-group-role-capability.ci`, whose discrimination was
measured (below).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - this change adds no numeric input | N/A | N/A | N/A | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dynamic-peer-gets-group-role-capability` | `test/plugin/dynamic-peer-gets-group-role-capability.ci` | An IXP operator configures a role on a listen-range group. A member connects and ze advertises RFC 9234 capability 9 carrying the group's stated role | pass |

**Discrimination, measured 2026-08-13.** With the group link deleted from
`Peer.getPluginCapabilities`, ze's OPEN arrives as
`...002B 01 04 FFFD 001E 01020304 0E 02 0C 010400010001 41040000FFFD` -- three
octets shorter, optional parameter length `0E`, no capability 9 -- and the file
fails on the hex. Restored, it passes; the runner rebuilds the binaries on every
invocation, so no run was cached. The assertion is not vacuous either: the payload
asserted is `01`, the value RFC 9234 Table 1 gives RS and the value the group
STATES. A peer with no role config receives no capability 9 at all rather than a
default, and `00` would be Provider.

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | The wire encoding of capability 9 is unchanged and already interop-tested. What changes is WHICH peers receive an already-conformant capability, which a `.ci` asserting the OPEN octets settles. No peer daemon behaviour is in question | N/A |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/configjson/traverse.go` - `ForEachPeer` yields the group
  name and visits a dynamic group's template; `isDynamicGroup`, `PeerConfigKey` and
  `LookupPeerConfig` join `PeerRemoteIP` as the shared key and lookup mechanism
- `internal/component/bgp/configjson/traverse_test.go` - traversal unit tests
- `internal/component/bgp/plugins/role/config.go` - key the template under the group
- `internal/component/bgp/plugins/role/role.go` - group fallback at lookup
- `internal/component/bgp/plugins/rpki/rpki_config.go` - same
- `internal/component/bgp/plugins/rpki/rpki.go` - group fallback at lookup
- `internal/component/bgp/blackholecfg/blackholecfg.go` - same
- `internal/component/bgp/plugins/filter_community/filter_community.go` - same
- `internal/component/bgp/plugins/filter_irr/config.go` - visitor signature
- `internal/component/bgp/plugins/filter_family/config.go` - visitor signature; the
  template's export chain becomes validated, which it is not today
- `internal/component/bgp/plugins/softver/softver.go` - declare for the group
- `internal/component/bgp/plugins/gr/gr.go`, `gr_llgr.go` - declare for the group
- `internal/component/bgp/plugins/llnh/llnh.go` - declare for the group
- `internal/component/bgp/plugins/hostname/hostname.go` - declare for the group
- `internal/component/bgp/reactor/peer.go` - `getPluginCapabilities` gains the
  group-name link in its existing fallback chain
- `docs/architecture/meta/role.md` - the recorded limit is removed
- `docs/architecture/config/syntax.md` - the traversal's `// Design:` anchor

## Files to Create
- `internal/component/bgp/configjson/dynamic_group_test.go` - the guard that every
  `ForEachPeer` caller keys a dynamic group's template reachably
- `test/plugin/dynamic-peer-gets-group-role-capability.ci` - a route server with no
  static peer; the member's OPEN carries capability 9 with the group's role

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves; dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- the delivery layer yields the template
   - Tests: `TestForEachPeerVisitsDynamicGroupTemplate`,
     `TestForEachPeerVisitsStaticPeersOfADynamicGroup`,
     `TestForEachPeerSkipsATemplateVisitForAPlainGroup`,
     `TestPeerConfigKeySeparatesAGroupFromAPeerOfTheSameName`,
     `TestLookupPeerConfigPrefersThePeerOverItsGroup`
   - Files: `internal/component/bgp/configjson/traverse.go` and its test; every one of
     the 11 caller files updated for the visitor's new group-name argument, behaviour
     unchanged
   - Verify: the traversal tests pass; every caller compiles and its existing tests
     still pass; no plugin behaviour has changed yet
2. **Phase: Capability declaration** -- a dynamic member's OPEN carries what its group states
   - Tests: `TestRoleCapabilityDeclaredForDynamicGroup`,
     `TestPluginCapabilitiesResolveByGroupName`
   - Files: `role/config.go`, `softver/softver.go`, `gr/gr.go`, `gr/gr_llgr.go`,
     `llnh/llnh.go`, `hostname/hostname.go`, `reactor/peer.go`
   - Verify: the declaration names the group; `getPluginCapabilities` resolves it
     only for a peer whose `IsDynamic` is set
3. **Phase: `blackholecfg`'s consumers and the RIB identity** -- AC-7
   - Tests: `TestBlackholeRuleForDynamicGroup`
   - Files: `rib/rib_blackhole.go`, `rib/rib_bestchange.go`, `cmd/announce/blackhole_agreement.go`,
     `rpki/rpki_config.go`
   - What blocks it: `blackholecfg.Parse` already RETURNS the template under
     `configjson.GroupKey`, and no consumer reads it. `RIBManager.blackholeRouteTypeForBest`
     takes only `peerAddr netip.Addr`, so a group identity has to be threaded through
     the best-path decision before a dynamic member can resolve its group's rule.
     `agreedSelector` needs the same from `pluginserver.PeersMatching`
   - Verify: a dynamic member's best path is stamped from its group's stated rule
4. **Phase: `role`'s decision path and the RPC contract** -- AC-5
   - Tests: `TestGetFilterConfigFallsBackToGroup`
   - Files: `role/role.go` (`filterPeerConfigs`, `filterRemoteRoles`, `filterNameToIP`,
     `getFilterConfig`, `setFilterState`'s retention loop), `role/otc.go` (three call
     sites), `role/validate.go`, `bgp/server/validate.go`, and the `rpc` package
   - What blocks it: **`role` cannot do RFC 9234 strict-mode OPEN validation for a
     dynamic peer at all.** `rpc.ValidateOpenInput` carries `Peer`, `Local` and
     `Remote` and no group, so `validateOpenRolePair` resolves `cfg == nil` for
     every dynamic member and returns `Accept: true` unconditionally. Reaching it
     needs a `Group` field added to that contract and filled by
     `internal/component/bgp/server/validate.go`. The OTC gates are separable from
     that: `getFilterConfig` is passed `PeerFilterInfo.Address`, and
     `PeerFilterInfo.GroupName` is already populated at all seven build sites
   - Verify: the OTC ingress gate runs for a member using the group's role, and
     strict mode refuses a member whose OPEN carries no role
5. **Phase: `rpki` actions and `filter_community`** -- AC-6, AC-8
   - Tests: `TestRPKIPeerActionsForDynamicGroup`, `TestDynamicGroupPluginConfigMatchesAStaticPeer`
   - Files: `rpki/rpki_config.go`, `rpki/rpki.go`, `rpki/rpki_status.go`,
     `filter_community/filter_community.go`
   - Verify: each plugin's lookup falls back to the group; AC-9 still holds
6. **Phase: Guard and proof**
   - Tests: `TestDynamicGroupPluginConfigMatchesAStaticPeer`,
     `test/plugin/dynamic-peer-gets-group-role-capability.ci`
   - Files: `internal/component/bgp/configjson/dynamic_group_test.go`, the `.ci`,
     `docs/architecture/meta/role.md`
   - Verify: break the inheritance, watch the `.ci` go red, restore, confirm no
     `(cached)` on the re-run

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, named by file and symbol |
| Every caller handled | All 11 non-test `ForEachPeer` callers are accounted for: each one either keys the template reachably or has a written reason it does not need to |
| Correctness | The merge order stays bgp → group → peer, and a peer-level statement still beats its group's (AC-9) |
| One mechanism | No plugin grows a second config source, and no range matching is introduced beside the group-name key (`ai/rules/no-layering.md`) |
| Key collision | The typed key makes a peer named `x` and a group named `x` distinguishable, and the capability path consults the group only for a dynamic peer (A-7) |
| No new silent miss | A lookup that misses returns a miss the caller can see, never a zero value that reads as "nothing configured" (`ai/rules/evidence.md`) |
| Discrimination | Reverting the template inheritance turns the `.ci` red, and the re-run is not `(cached)` |
| Vacuity | The `.ci` asserts the group's STATED role value, not merely that some capability arrived |

### End-to-End User Stories (filled)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an IXP route server as a listen-range group with a role | config → `ForEachPeer` template visit → `role` declares for the group → `getPluginCapabilities` → OPEN | `dynamic-peer-gets-group-role-capability.ci` |
| 2 | Receives an UPDATE from a member of that group | wire → `reactor_notify` → `OTCIngressFilter` → `getFilterConfig` group fallback | `TestGetFilterConfigFallsBackToGroup` |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- **The blast radius was understated twice.** The spec recorded 2 callers and named
  the wrong package. `grep -rn ForEachPeer --include=*.go` over the tree finds 11
  non-test call sites in `internal/component/bgp/`.
- **"Enumerate peers at config time" cannot see a dynamic member, and no traversal
  fix changes that.** `BuildPluginConfigSections` serializes the operator's own
  document, and a dynamic member is created by `tryCreateDynamicPeer` when a
  connection arrives. The traversal can only yield the TEMPLATE. This is the
  question the task asked to settle before choosing, and it rules out the obvious
  fix.
- **The identity already exists at both ends, and nothing consumes it.**
  `PeerSettings.GroupName` is set by `buildDynamicPeerSettings`, and all seven
  `filterapi.PeerFilterInfo` build sites in the reactor populate `GroupName`. No
  plugin reads it for config lookup. The fix is to READ what is already carried,
  not to carry something new, which is why it needs no reactor change.
- **`reactor.Peer.getPluginCapabilities` is already a fallback chain** (name, then
  address). The group is a third link in an existing mechanism, not a second
  mechanism beside it.
- **A group name and a peer name share no uniqueness check** (A-7, broken). That is
  what forces a typed key rather than a prefixed string, and a `:`-separated
  selector on the one path whose contract fixes the key as a plain string.
- **RFC 9234 strict mode cannot run for a dynamic peer at all, and the RPC contract
  is why.** `rpc.ValidateOpenInput` carries `Peer`, `Local` and `Remote` and no
  group. `validateOpenRolePair` (`internal/component/bgp/plugins/role/validate.go`)
  therefore resolves `cfg == nil` for every dynamic member and returns
  `Accept: true` on its first line, so `strict true` on a listen-range group is
  inert whatever the member's OPEN carries. This is a WIDER hole than the OTC gates
  beside it: those read `PeerFilterInfo.Address` and can reach a group through
  `PeerFilterInfo.GroupName`, which is already populated. Strict mode needs a
  `Group` field ADDED to the plugin RPC contract and filled by
  `internal/component/bgp/server/validate.go`. Phase 4 owns it.
- **The capability path has no unit-test seam, and that is a property of
  `pluginserver.Server`.** Its `capInjector` is unexported and reachable only from
  `engineStartupSink.onCapabilities`, driven by a real plugin process. The
  reactor's `r.api` is that concrete type rather than an interface, so nothing can
  be substituted. Anything that wants to assert capability resolution from the
  reactor's side pays a functional test or an exported setter; this spec paid the
  functional test.
- **`role/config.go`'s name-as-address fallback looks unreachable.** Its comment
  says operators commonly name a peer by its own address, but `validatePeerName`
  refuses a name that parses as one. The fallback was preserved here rather than
  removed, because removing it is a behaviour change outside this spec's goal. It is
  worth settling in the re-cut: `configjson.PeerKey` now carries the same branch.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `ForEachPeer` yields a dynamic group's TEMPLATE as one peer visit, keyed by the group name | Yield dynamic MEMBERS from the traversal; match a peer's address against the group's range at lookup | Members are not in the document the traversal reads, so the first is unreachable. Range matching re-derives a membership the reactor already decided, and would be a second mechanism beside `PeerSettings.GroupName`, which both ends already carry |
| The group is a third link in the EXISTING lookup chains | A shared range-matching resolver type used by every plugin | `getPluginCapabilities` already falls back name → address. Adding a link costs one lookup; a resolver type is machinery the problem does not need (`ai/rules/simplicity.md`, `ai/rules/no-layering.md`) |
| `PeerConfigKey{ID, Template}` rather than a `"group:"`-prefixed string | One string space with a reserved prefix everywhere | A-7 is broken, so the two namespaces really can collide. A typed pair separates them by construction. The prefix is used only where the RPC contract fixes the key as a string, and there `:` is unusable in either name |
| The template visit is emitted for a group whose `remote > ip` is `dynamic`, reusing `config.isDynamicGroup`'s exact test | Treat the presence of a `range` as the marker | Two definitions of "dynamic group" would let the reactor build peers from a template the traversal delivered no config for, which is the defect restored by another route |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

`configjson.ForEachPeer` now visits a dynamic group once, as a peer visit with a
nil peer map and `PeerOrigin{Group: <name>, Template: true}`. `PeerOrigin` is the
only thing that separates that visit from a configured peer's, because a peer
that states no fields also has a nil map. Around it, `configjson` gained the key
and lookup mechanism the 11 callers share: `PeerConfigKey{ID, Template}`,
`PeerKey`, `GroupKey`, `KeyFor`, `LookupPeerConfig` and `isDynamicGroup`, which
reuses `config.isDynamicGroup`'s exact marker so the traversal and the reactor
cannot disagree about which groups are dynamic.

Nine callers key the template reachably and two record why they key nothing.
The six capability plugins (`role`, `softver`, `gr`, `gr_llgr`, `llnh`,
`hostname`) and `filter_community` use `configjson.CapabilitySelector`, the
`group:`-prefixed string the RPC contract's plain-string key forces;
`blackholecfg` uses `GroupKey` and `rpki` uses `KeyFor`, both typed.

Four decision paths then resolve that entry from an identity the runtime already
carried and no plugin read:

| Path | Producer | Identity it resolves from |
|------|----------|---------------------------|
| The OPEN's capability 9 | `reactor.Peer.getPluginCapabilities` | `PeerSettings.GroupName`, a third link on an existing name → address chain, gated on `PeerSettings.IsDynamic` |
| RFC 9234 Section 5 OTC gates | `role.getFilterConfig` | `filterapi.PeerFilterInfo.GroupName` |
| RFC 9234 Section 4.2 OPEN pair, strict included | `role.applyValidateOpen` | `rpc.ValidateOpenInput.Group`, a new field on the plugin RPC contract filled by `bgp/server/validate.go` |
| RFC 6811 origin actions, RFC 7999 blackhole | `rpki.buildDecisions`, `rpki.carriesAgreedBlackhole`, `rib.blackholeRouteTypeForBest`, `announce.agreedSelector` | the group name carried on both event rails (`bgp/event.go:GetPeerGroup`) and on route metadata |

### Bugs Found/Fixed

- **R-2 fired during phase 1.** `blackholecfg.Parse` read `PeerRemoteIP` on every
  visit, so a dynamic group stating a `blackhole` block failed the whole
  configuration with `no usable remote IP ("dynamic")`. No test covered either
  state. `Parse` now returns `map[configjson.PeerConfigKey]Rule` and branches on
  `origin.Template` before the address parse. The refusal is preserved for a
  NAMED peer with no usable remote IP, where it is still right.
  Covered by `TestParseKeysADynamicGroupsBlockUnderTheGroup` and three siblings.
- **R-3 fired in `filter_community`.** The template landed in `configs[peerName]`,
  a map keyed by peer NAME, and `ForEachPeer` visits groups after standalone
  peers, so a config holding both `peer ix` and `group ix` gave the PEER the
  group's community policy on every ingress and egress decision. Measured: the
  peer's `ingressTag` read `["group-tag"]` instead of `["peer-tag"]`. Fixed by
  the prefixed selector. Covered by
  `TestDynamicGroupTemplateDoesNotOverwriteAPeerOfTheSameName`.
- **A-7 was broken**, and it decided the key. Nothing compares the peer and group
  namespaces, so `bgp { peer ix {...} group ix {...} }` is accepted. A bare
  string key would let a group's template answer a lookup meant for a peer.
- **Stale comments in the `.ci`, found at closure.** `test/plugin/dynamic-peer-\
  gets-group-role-capability.ci` carried two blocks stating that the Section 5
  OTC gates and strict mode "cannot name a group, so neither runs for a dynamic
  member". Phase 4 made both run, so the file documented a hole it had closed.
  Rewritten to name the unit tests that now prove both, and to say why the file
  still keeps to what the member ADVERTISES: that is the one link with no
  unit-test seam.
- **R-1's mitigation was missing, found at closure.** The spec promised a guard
  that "enumerates the callers and fails when a new one appears unhandled". The
  delivered `dynamic_group_test.go` tested the delivery layer only and named no
  caller, so a twelfth caller ignoring `PeerOrigin` would have left it green.
  `TestEveryForEachPeerCallerIsAccountedFor` now scans `internal/`, `cmd/` and
  `pkg/` for non-test `configjson.ForEachPeer(` sites and requires the set to
  equal `forEachPeerCallers`. Measured red by removing one row: it names the
  caller it did not expect.

### Documentation Updates

- `docs/architecture/meta/role.md`: the "Known limits" section said role config
  "cannot reach peers created from a dynamic group". Replaced by a "Dynamic
  groups" section giving the three readers and the identity each resolves from,
  with anchors on `traverse.go -- ForEachPeer, CapabilitySelector`,
  `role/config.go -- extractPeerRoleConfigs`, `role/role.go -- getFilterConfig,
  applyValidateOpen` and `reactor/peer.go -- getPluginCapabilities`. The "Config
  keying" section above it was corrected in the same pass: both parse-time
  refusals are about a NAMED peer, and a dynamic group's placeholder never
  reaches them.
- `docs/guide/rpki.md`: "dynamically-addressed peers use the global actions" was
  false for a listen-range group and true for a named peer with no static remote
  IP. The two cases are now separated, with the startup message the second one
  produces, and a listen-range example beside the existing group example.
- `ai/CODE-TO-DOCS.md`: regenerated; `peer.go` gains `docs/architecture/meta/role.md`.
- `rfc/requirements/{rfc6810,rfc7606,rfc7999,rfc8210,rfc9234}.md` regenerated by
  `make ze-rfc-index`. RFC 7999 Section 3.1 and 3.3 gain positive and negative
  tests from `rib_blackhole_dynamic_test.go` and
  `announce/blackhole_agreement_test.go`; the rest are line shifts inside
  `role/dynamic_group_test.go`.
- `rfc/audit/rfc7606.json` re-stamped by `make ze-rfc-reseal`. `RFC7606-2-1` was
  SHIFTED because phase 3 added `feedReceivedFromGroup` to
  `rib_structured_test.go` and had `feedReceived` delegate to it. The tagged
  assertions are byte-identical, which the reseal tool's own fingerprint check
  confirms independently.
- `make ze-doc-test` fails on one unrelated item: `rules-points: writing.md is
  stale`, produced by another session's uncommitted `ai/rules/writing.md`. No
  other check in that target fails.

### Deviations from Plan

- **Phase count went 4 → 5 → 6.** The four keying plugins could not be finished
  at the altitude of `configjson` alone: `blackholecfg`'s consumers needed a
  group identity threaded through the RIB best-path decision, and `role`'s
  strict mode needed a field added to the plugin RPC contract.
- **`TestPluginCapabilitiesResolveByGroupName` was not written.** The seam does
  not exist: `pluginserver.Server.capInjector` is unexported and reached only
  from `engineStartupSink.onCapabilities`, driven by a real plugin process, and
  the reactor's `r.api` is that concrete type rather than an interface. Adding an
  exported setter for a test is machinery the problem does not need
  (`ai/rules/simplicity.md`). The link is proven at its entry point instead, on
  the wire, by the `.ci`, whose discrimination was measured.
- **One widening is deliberate and is not a regression: `filter_family`.**
  `validateNoTearDownInExport` now runs over a dynamic group's own export chain,
  which it never reached, because a group with no `peer` list produced no visit.
  A listen-range group stating `filter { export <tear-down-instance> }` now fails
  config validation. That refusal is correct: the reactor DOES build every
  member's export chain from that group, so the invalid chain was being applied
  while the check that exists to refuse it could not see it. This is the Goal's
  second branch.
- **AC-8 at `rpki` compares effective actions with provenance masked.** The
  test's `effective()` helper rewrites the four `Source` fields on both sides
  before comparing. That is deliberate and matches AC-8's wording ("the same
  effective plugin config"): the sources SHOULD differ, one resolving
  `sourceGroup` where the other resolves `sourcePeer`. The provenance itself is
  asserted separately by `TestStatusCommand_PerPeerActions_NamesAGroupAsAGroup`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-7: a group name and a peer name cannot collide in one config | `ResolveBGPTree` collects peer names into one map and refuses duplicates, but a group name only goes through `validateGroupName`. Nothing compares the two namespaces, so `peer ix` and `group ix` coexist | read `resolve.go`, then wrote the test | the key became a typed pair `PeerConfigKey{ID, Template}`, and a `group:`-prefixed selector on the one path whose RPC contract fixes the key as a string |
| approach | Phase 1 delivered the template visit to every caller and left each caller's key untouched, on the reading that a delivery-layer change is behaviour-neutral | Two callers keyed the template into a namespace they share with peers: `blackholecfg` failed the whole config, `filter_community` silently gave a peer its group's policy | `blackholecfg` failed loudly; `filter_community` was found by writing the collision test R-3 named | the visit and the KEY are one change, not two. Every caller was re-examined for the namespace its map uses before phase 2 closed |
| approach | The closure's own guard test was written against the delivery layer and read as satisfying R-1 | R-1 is about CALLERS, and a delivery-layer test names none. A twelfth caller would have left it green | the caller-verification pass at closure read the test body rather than its name | `TestEveryForEachPeerCallerIsAccountedFor` enumerates the call sites from source, and was measured red |
| assumption | The capability fallback chain was read as three probes in precedence order, and the `.ci` passing was taken as proof the third link works | `GetCapabilitiesForPeer` merges the GLOBAL capabilities into every answer, so probe one never returns empty once any plugin declares one, and probes two and three are unreachable. The `.ci` passes only because it loads one plugin that declares none | a reviewer read the injector rather than the chain that calls it | the chain became one call over an ordered selector list. The lesson is that a fallback written as "if the last answer was empty" is only correct when the source can actually return empty |
| escalation | The fix for that BLOCKER added the new method and left the old one beside it | `ai/rules/no-layering.md` is always-on: replacing X with Y means DELETE X first. Leaving both is how a second mechanism is born, and `ze-validate` would have named it at the next commit anyway | the confirmation review round | both deleted in the same session that added the replacement, before any commit |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-peer config reaches peers created from a dynamic group | Done | `configjson/traverse.go:ForEachPeer` (template visit), then the four decision paths in the Implementation Summary table | The delivery layer was the right altitude: one traversal change, then one lookup link per plugin |
| ... or the config is rejected at verify with an error naming the group | Done | `filter_family/config.go` export-chain validation now reaches a dynamic group's own chain | The Goal's second branch, and the one place a dynamic group's config is now REFUSED rather than delivered |
| A half-fix resolving only one plugin is out of scope | Done | 9 of 11 callers key the template; the other 2 record why they key nothing | `TestEveryForEachPeerCallerIsAccountedFor` holds the accounting |
| The recorded limit in `docs/architecture/meta/role.md` is removed | Done | `docs/architecture/meta/role.md`, "Dynamic groups" replaces "Known limits" | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForEachPeerVisitsDynamicGroupTemplate` over `traverse.go:ForEachPeer` | exactly one visit, name `ix`, nil peer map, non-nil group map, `PeerOrigin{Group:"ix",Template:true}` |
| AC-2 | Done | `TestForEachPeerVisitsStaticPeersOfADynamicGroup` | both visits; the static peer keeps the name and maps it gets today |
| AC-3 | Done | `TestForEachPeerSkipsATemplateVisitForAPlainGroup`, `TestIsDynamicGroupMatchesTheConfigResolver` | 0 templates, 1 peer; the marker is pinned to `config.isDynamicGroup` |
| AC-4 | Done | `TestRoleCapabilityDeclaredForDynamicGroup` (selector `group:ix`, code 9, payload `01`) and `test/plugin/dynamic-peer-gets-group-role-capability.ci` (the OPEN, whole) | the payload asserted is the STATED value, not the `00` a default would give |
| AC-5 | Done | `TestOTCIngressGateRunsForADynamicGroupMember`, `TestValidateOpenRolePairRunsForADynamicGroupMember`, `TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`, `TestGetFilterConfigFallsBackToGroup` | each carries a no-group negative control |
| AC-6 | Done | `TestBuildDecisions_DynamicGroupMemberUsesGroupAction`, `TestRPKIPeerActionsForDynamicGroup`, `TestCarriesAgreedBlackholeResolvesThroughTheGroup`, plus both rail delivery tests | the rail tests stop the decision test feeding itself by hand |
| AC-7 | Done | `TestBlackholeRuleForDynamicGroup` (3 subtests), `TestBlackholeGroupIdentityArrivesOn{AStructuredEvent,AJSONEvent}`, `...IsEmptyForAStandalonePeer`, and the announce reach/withheld pair | delivery proven from real UPDATE bytes through `updatePeerMetadata` |
| AC-8 | Done | `TestDynamicGroupPluginConfigMatchesAStaticPeer`, `TestRPKIDynamicGroupActionsMatchAStaticPeer`, `TestDynamicGroupCommunityConfigMatchesAStaticPeer` | the first asserts equality AND the exact stated map, so two misses cannot pass |
| AC-9 | Done | `TestLookupPeerConfigPrefersThePeerOverItsGroup`, `TestAPeerInsideADynamicGroupKeepsItsOwnConfig`, and a peer-beats-group case inside each plugin's suite | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 8 traversal tests | pass | `configjson/traverse_test.go` | AC-1, AC-2, AC-3, AC-9, A-7 both key shapes, and the marker |
| All 3 delivery-layer guard tests | pass | `configjson/dynamic_group_test.go` | AC-8, AC-9, and R-1's caller enumeration |
| 3 `role` config-keying tests | pass | `role/config_keying_test.go` | AC-4 at the declaration; A-7 at `role` |
| 5 `role` decision-path tests | pass | `role/dynamic_group_test.go` | AC-5, both RFC 9234 sections, and reload retention |
| 4 `blackholecfg` tests | pass | `blackholecfg/blackholecfg_test.go` | the R-2 regression and its three siblings |
| 4 `rib` blackhole tests | pass | `rib/rib_blackhole_wiring_test.go`, `rib/rib_blackhole_dynamic_test.go` | AC-7 at the decision and on both delivery rails |
| 2 `announce` tests | pass | `announce/blackhole_agreement_test.go` | AC-7 on the send side, RFC 7999 Section 3.1 |
| 8 `rpki` tests | pass | `rpki/{rpki_config,rpki_action,blackhole_agreement,dynamic_group,rpki_status}_test.go` | AC-6, AC-8, both rails, re-validation, and the status surface |
| 5 `filter_community` tests | pass | `filter_community/dynamic_group_test.go` | three readers, AC-9, AC-8 |
| `dynamic-peer-gets-group-role-capability` | pass | `test/plugin/` | test 186 of `make ze-plugin-test` |
| `TestPluginCapabilitiesResolveByGroupName` | Changed | not written | no seam exists; recorded in Deviations, proven by the `.ci` instead |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `configjson/traverse.go` + test | Done | the template visit and the shared key/lookup mechanism |
| `configjson/dynamic_group_test.go` | Done | created; carries the AC-8 guard and R-1's caller enumeration |
| `role/{config.go,role.go}`, `role/otc.go`, `role/validate.go` | Done | key, three lookup paths, and the strict-mode branch |
| `rpki/{rpki_config.go,rpki.go,rpki_status.go}`, `rpki/{blackhole,origin_tracker,aspa_tracker}.go` | Done | key, lookup, status label, and identity retention through re-validation |
| `blackholecfg/blackholecfg.go` | Done | typed key map; template branch before the address parse |
| `filter_community/filter_community.go` | Done | prefixed selector, and `lookupPeerConfigLocked` at all three readers |
| `filter_irr/config.go`, `filter_family/config.go` | Done | signature plus a written reason each |
| `softver`, `gr`, `gr_llgr`, `llnh`, `hostname` | Done | declare for the group |
| `reactor/peer.go` | Done | the three selectors, resolved in one call, the group gated on `IsDynamic` |
| `internal/component/plugin/registration.go`, `internal/component/plugin/server/server.go` | Added | not in the plan: `GetCapabilitiesForSelectors` replaces `GetCapabilitiesForPeer` at both layers. The old getters are DELETED, not left beside it (`ai/rules/no-layering.md`). This is what makes the third link reachable at all |
| `bgp/server/validate.go`, `pkg/plugin/rpc/types.go` | Added | not in the plan: `ValidateOpenInput.Group` is what makes strict mode reachable at all |
| `rib/{rib.go,rib_blackhole.go,rib_blackhole_config.go,rib_structured.go}`, `bgp/event.go`, `announce/blackhole_agreement.go` | Added | not in the plan: AC-7 needs the group identity threaded through the best-path decision and both event rails |
| `docs/architecture/meta/role.md` | Done | |
| `docs/architecture/config/syntax.md` | Not needed | grepped: it carries no `// Design:` anchor naming `traverse.go`, and the traversal's own anchor is in `meta/role.md`, which was updated |

### Audit Summary
- **Total items:** 9 ACs, 4 Task requirements, 16 file groups
- **Done:** 9 ACs, 4 requirements, 14 file groups
- **Partial:** none
- **Skipped:** none
- **Changed:** `TestPluginCapabilitiesResolveByGroupName` (no seam; recorded in Deviations); `docs/architecture/config/syntax.md` (not needed); 2 file groups ADDED beyond the plan (the RPC contract and the RIB identity)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Per-peer config reaches peers created from a dynamic group | functional (`.ci`), on the wire | `test/plugin/dynamic-peer-gets-group-role-capability.ci`, test 186 of `make ze-plugin-test`, `1.1s PASS`. It asserts ze's OPEN whole, including `09 01 01`. Discrimination measured 2026-08-13: deleting the group link from `getPluginCapabilities` makes the OPEN 3 octets shorter with optional-parameter length `0E` and no capability 9, and the file goes red on the hex |
| ... proven for a member, not merely for the config | functional | the same file asserts `dynamic peer created` and `session established` on stderr. Without them the wire assertions would hold for any peer ze accepted, which is how a first attempt at a dynamic test passes against unfixed code |
| The fix is at the delivery layer, not one plugin | unit + source-enumerating guard | `TestEveryForEachPeerCallerIsAccountedFor` requires the set of non-test `ForEachPeer` call sites to equal a table recording, per caller, the keying helper it uses or the reason it keys none. Measured red by removing one row |
| A dynamic group's config that cannot be honored is REFUSED, naming the group | unit | `filter_family`'s `validateNoTearDownInExport` now reaches a dynamic group's export chain. A listen-range group stating a tear-down export instance fails config validation where it used to load silently |
| RFC 9234 conformance for a member (Sections 4.2 and 5) | unit, RFC-tagged | `RFC9234-4.2-1`, `-4.2-2`, `-4.2-5` and `-5-1` in `rfc/requirements/rfc9234.md` now carry `role/dynamic_group_test.go` sites in both polarities. `make ze-rfc-check` exit 0 |
| RFC 7999 conformance for a member (Sections 3.1 and 3.3) | unit, RFC-tagged | `RFC7999-3.1-2` gains `announce/blackhole_agreement_test.go:272` positive and `:300` negative; `RFC7999-3.3-1` and `-3.3-2` gain `rib/rib_blackhole_dynamic_test.go`. `make ze-rfc-check` exit 0 |
| Interop | N-A, with a reason | The wire encoding of capability 9 is unchanged and already interop-tested. What changes is WHICH peers receive an already-conformant capability, which a `.ci` asserting the OPEN octets settles. No peer daemon behaviour is in question |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `plan/deferrals/ad-hoc-2026-07-27-423eaa77.md`, row 2 (2026-07-27, handover 22): "a peer created from a dynamic group receives no per-peer plugin config at all" | done | This spec. Row updated in place with the producing functions and the `.ci` that proves it end to end. The spec's own metadata names no shard (`Deferral shard | -`), so this is a FOREIGN shard carrying the row that created this spec |
| That shard's remaining rows | deferred (unchanged) | `spec-fixit-zefs-diff-structural-ops`, `spec-fixit-netns-test-dut-tags` and `spec-fixit-peers-from-tree-stale-shape` are still live, so the shard is NOT removed by this closure |
| `plan/journal/gate-excludes-part-of-its-population.md`, row added 2026-08-14 | recorded, not fixed | `configjson.PeerKey` stores the operator's RAW address spelling while every runtime reader produces the canonical `netip.Addr.String()` form, so `2001:0DB8::1` keys an entry nothing reaches. It is the STATIC path and a listen-range member has no configured address at all, so it does not block this goal. Phase 5 normalized inside `rpki` only. Whether `PeerKey` itself should normalize is one decision covering every consumer |
| `plan/journal/gate-verdict-depends-on-the-machine.md`, row added 2026-08-14 | recorded, not fixed | `internal/plugins/iface/ra/ifacera.go`'s `maxFinalAdvertisements` reads unused on darwin because its only reader carries `//go:build linux`. Present in HEAD, untouched here |
| `plan/journal/same-bytes-decoded-once-per-accessor.md`, new class file | recorded, not fixed | `Event.GetPeerGroup` is a third full `json.Unmarshal` of the same peer object per received UPDATE on the JSON rail, beside the two already there. The pattern predates this change, which extended it by one. The structured rail, which the default in-process deployment uses, carries `PeerGroup` as a struct field and decodes nothing, so no goal here depends on it. The fix is one `Event.PeerInfo()` that decodes once, which changes every existing accessor's call sites |
| `plan/journal/unwired-feature.md`, row added 2026-08-14 | recorded, not fixed, **and NOT committed by this closure** | `make ze-validate` is red at HEAD on three exported accessors in `internal/component/bgp/event.go` with no cross-package non-test caller: `ParseFamilyOps`, `Event.GetDirection` and `Event.GetPeerSelector`. The row is written into the working tree, but that journal file carries another session's in-flight rows, so this commit leaves it for whoever commits that file next (`ai/rules/git-safety.md`, shared plan files) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-dynamic-group-peer-config-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md` |
| `review_gate.py check` | clean |
| Rounds | 4. Round 2 found the capability BLOCKER (a global declaration cost every peer its own); round 3 found the replaced getter left with no caller (`ai/rules/no-layering.md`) and the structured rail's group READ having no test at all; round 4 confirmed those fixes |
| Reviewer lenses used | caller accounting + AC-to-producer verification; logic + wiring + silent-miss; security + RFC conformance + test discrimination; post-fix confirmation |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The `.ci` carried two comment blocks stating the Section 5 OTC gates and Section 4.2 strict mode "cannot name a group, so neither runs for a dynamic member". Phase 4 made both run, so the file documented a hole it had closed (`ai/rules/stale-comments.md`) | `test/plugin/dynamic-peer-gets-group-role-capability.ci` | rewritten to name the unit tests that prove both, and to say why the file still keeps to the advertised capability: it is the one link with no unit-test seam |
| 2 | ISSUE | R-1's stated mitigation was not delivered. `dynamic_group_test.go` tested the delivery layer and named no caller, so a twelfth `ForEachPeer` caller that ignored `PeerOrigin` would have left it green, restoring the silent miss for its own plugin | `internal/component/bgp/configjson/dynamic_group_test.go` | `TestEveryForEachPeerCallerIsAccountedFor` scans `internal/`, `cmd/` and `pkg/` for non-test call sites and requires the set to equal `forEachPeerCallers`, which records per caller the keying helper used or the reason none is. Measured red by removing one row |
| 3 | NOTE | The spec's TDD row for `TestDynamicGroupPluginConfigMatchesAStaticPeer` read `phase 6` after the test was passing, and its sibling `TestAPeerInsideADynamicGroupKeepsItsOwnConfig` had no row | `plan/spec-fixit-dynamic-group-peer-config.md` | both rows corrected; the new guard test added beside them |
| 4 | NOTE | AC-8 at `rpki` compares effective actions with the four `Source` fields masked on both sides, so it is narrower than the row's wording | `rpki/rpki_config_test.go` | recorded in Deviations as deliberate: the sources SHOULD differ, and provenance is asserted separately by `TestStatusCommand_PerPeerActions_NamesAGroupAsAGroup` |
| 5 | **BLOCKER** | **AC-4 failed under any configuration that also ran a plugin declaring a GLOBAL capability.** `CapabilityInjector.GetCapabilitiesForPeer` merges `globalCaps` into every answer, so `getPluginCapabilities`'s first probe (`settings.Name`) came back non-empty and its `len(injected) == 0` guards skipped BOTH the address probe and the new group probe. A dynamic member then advertised no Role capability, and a statically configured peer lost its address-keyed capabilities too, which is a defect that predates this spec. `ze --plugin ze.exabgp` is enough to arm it: `internal/plugins/exabgp/main_sdk.go` declares code 2 with no `Peers`. The `.ci` loads only `bgp-role`, so it stayed green | `internal/component/plugin/registration.go:GetCapabilitiesForPeer`, `internal/component/bgp/reactor/peer.go:getPluginCapabilities` | `GetCapabilitiesForSelectors(selectors ...string)` resolves the whole ordered list in ONE call: first selector wins per capability code, globals fill only the codes no selector claimed. `getPluginCapabilities` builds the three selectors and calls once. Covered by `TestSelectorsResolveBesideAGlobalCapability` and `TestSelectorPrecedenceIsFirstWins`, both measured red when the resolution is cut back to the first selector |
| 6 | ISSUE | RFC 6811 Section 4 re-validation carried the group in production but no test drove the DISPATCH site: deleting `peerGroup: c.peerGroup` (`handleROAChange`) or `peerGroup: rt.peerGroup` (`handleASPAChange`) left every rpki test green while a member's re-validated route silently fell back to the global actions | `internal/component/bgp/plugins/rpki/rpki.go:handleROAChange`, `:handleASPAChange` | `TestOriginRevalidationDispatchesWithTheGroup` and `TestASPARevalidationDispatchesWithTheGroup` drive both handlers and assert the enqueued request's group. Both measured red against the deletion |
| 7 | ISSUE | The structured rail's own READ of the group had no test. `TestStructuredRailCarriesTheGroupIntoTheValidationRequest` passes `validateNLRIs` a literal `"ix"`, so it pins the walker and not the read: setting `peerGroup := ""` in `handleStructuredUpdate` left every rpki test green. That rail is what the default in-process deployment uses | `internal/component/bgp/plugins/rpki/rpki.go:handleStructuredUpdate` | `TestStructuredRailReadsTheGroupOffTheEvent` builds a real `rpc.StructuredEvent` carrying `PeerGroup` and a wire UPDATE, and asserts the group reaches the request. Measured: it is the only test that reddens on that deletion |
| 8 | ISSUE | The BLOCKER's own fix left `Server.GetPluginCapabilitiesForPeer` and `CapabilityInjector.GetCapabilitiesForPeer` in the tree with no production caller, which is the layering `ai/rules/no-layering.md` forbids: replace X with Y means DELETE X | `internal/component/plugin/server/server.go`, `internal/component/plugin/registration.go` | both deleted, their nine test callers migrated to the selector API, and the three comments naming the old getter corrected (`reactor/session.go` twice, `reactor/peer_settings_negotiation.go`) |
| 9 | ISSUE | `configjson.IsDynamicGroup` was exported with no cross-package non-test caller, which `make ze-validate` flags and `ai/rules/completion.md` forbids | `internal/component/bgp/configjson/traverse.go` | unexported to `isDynamicGroup`; `ze-validate` no longer names any symbol of ours |
| 10 | ISSUE | `configjson.PeerKey`'s doc claimed "operators very commonly name a peer by its own address", which `config.validatePeerName` refuses, and the same diff's `rib_blackhole.go` comment said the opposite. A `rpki_config_test.go` subtest built that unloadable config without saying so | `internal/component/bgp/configjson/traverse.go:PeerKey`, `rpki/rpki_config_test.go` | both comments corrected to state what the validator actually allows, and the subtest renamed to say it pins a `PeerKey` unit contract rather than a reachable operator scenario. The dead arm is KEPT: deleting it is a behavior change this spec's goal does not need |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/configjson/dynamic_group_test.go` | yes | `ls -la` 2026-08-14; created this spec, carries 3 tests |
| `internal/component/bgp/plugins/rib/rib_blackhole_dynamic_test.go` | yes | `ls -la` 2026-08-14, 4.6K |
| `internal/component/bgp/plugins/rpki/dynamic_group_test.go` | yes | `ls -la` 2026-08-14, 5.9K |
| `internal/component/bgp/plugins/role/dynamic_group_test.go` | yes | `ls -la`, 11K; tracked, landed in `424527bc7` |
| `internal/component/bgp/plugins/filter_community/dynamic_group_test.go` | yes | `ls -la`, 11K; tracked and modified this phase |
| `test/plugin/dynamic-peer-gets-group-role-capability.ci` | yes | `ls -la`, 6.6K; tracked, landed in `6eda867e2`, comments corrected at closure |
| `plan/journal/same-bytes-decoded-once-per-accessor.md` | yes | created at closure; new problem class, one row |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3, AC-8, AC-9 | the traversal yields the template, skips a plain group, and a peer beats its group | `make ze-test-pkg PKG=./internal/component/bgp/configjson` exit 0 |
| AC-4 declaration, AC-5 | selector is the group; both RFC 9234 gates run for a member | `make ze-test-pkg PKG=./internal/component/bgp/plugins/role` exit 0 |
| AC-4 wire | the member's OPEN carries `09 01 01` | `make ze-plugin-test`: `1.1s 186/631 PASS 186 dynamic-peer-gets-group-role-capability` |
| AC-6 | a member's origin-validation actions come from the group | `make ze-test-pkg PKG=./internal/component/bgp/plugins/rpki` exit 0 |
| AC-7 | a member's blackhole rule is the group's stated rule | `make ze-test-pkg PKG=./internal/component/bgp/plugins/rib` and `.../plugins/cmd/announce` and `.../blackholecfg`, all exit 0 |
| AC-8, AC-9 at `filter_community` | a member resolves its group's policy at all three readers | `make ze-test-pkg PKG=./internal/component/bgp/plugins/filter_community` exit 0 |
| R-1 | a new unhandled caller fails a gate | mutation: removing the `llnh` row from `forEachPeerCallers` gives exit 2 naming `internal/component/bgp/plugins/llnh/llnh.go`; restored, exit 0 |
| AC-4 beside a global capability | a plugin declaring a global capability no longer costs a peer its own | `make ze-test-pkg PKG=./internal/component/plugin` exit 0. Mutation: cutting `GetCapabilitiesForSelectors` back to the first selector reddens `TestSelectorsResolveBesideAGlobalCapability` and `TestSelectorPrecedenceIsFirstWins`; restored, exit 0 |
| AC-6 re-validation and the structured rail | the group survives RFC 6811 Section 4 re-validation and is READ off the event | mutations, each restored to green: dropping `peerGroup` from `handleROAChange` and `handleASPAChange` reddens the two re-validation dispatch tests; `peerGroup := ""` in `handleStructuredUpdate` reddens `TestStructuredRailReadsTheGroupOffTheEvent` and nothing else |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A `bgp` group whose `remote > ip` is `dynamic`, read by a plugin's `OnConfigure` | `dynamic-peer-gets-group-role-capability.ci` | yes: the config block is one dynamic group and NO static peer, so the only socket in the run is the one the group asks for |
| An inbound connection inside the group's range reaching Established | same | yes: `expect=stderr:contains=dynamic peer created` and `session established`. Read the file; both assertions are present and are what stop the test passing when the inbound is attributed to a static peer |
| ze's OPEN carries RFC 9234 capability 9 with the group's role | same | yes: `expect=bgp:conn=1:seq=1:hex=...090101`, the OPEN whole. `01` is RS per RFC 9234 Table 1, the value the group STATES |
| `role.OTCIngressFilter` finds a role config through the group fallback | unit, not `.ci` | yes: `TestOTCIngressGateRunsForADynamicGroupMember`, with a no-group negative control |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `BuildPluginConfigSections` (`plugin/server/startup.go`) serializes the RAW operator tree; a dynamic member is created by `tryCreateDynamicPeer` at connection time, so it cannot appear in it |
| A-2 | confirmed | `buildDynamicPeerSettings` (`reactor/reactor_dynamic.go`) writes `ps.GroupName` |
| A-3 | confirmed | all seven `filterapi.PeerFilterInfo` build sites populate `GroupName` |
| A-4 | confirmed | `AddPluginCapabilities` and `GetCapabilitiesForSelectors` (`plugin/registration.go`) match by exact string, so `group:<name>` works with no injector change. The assumption held; what did NOT hold was the reading of the CALLER, which finding 5 corrects |
| A-5 | confirmed | `isDynamicGroup` (`configjson/traverse.go`) reuses `config.isDynamicGroup`'s marker, pinned by `TestIsDynamicGroupMatchesTheConfigResolver` |
| A-6 | confirmed | 11 non-test callers, now enforced by `TestEveryForEachPeerCallerIsAccountedFor` rather than by a grep at a point in time |
| A-7 | **broken** | `ResolveBGPTree` never compares the peer and group namespaces. Mistake Log row 1; the key became typed, and R-3 fired in `filter_community` exactly as this predicted |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/meta/role.md` "Dynamic groups": three readers and the identity each resolves from | read `role.go:getFilterConfig` (:190), `role.go:applyValidateOpen` (:384-398), `reactor/peer.go:getPluginCapabilities` (:912-923) | yes |
| `docs/architecture/meta/role.md`: the capability chain is gated on `IsDynamic` | `reactor/peer.go:923`, the third link fires only after name and address miss and only when `settings.IsDynamic` | yes |
| `docs/guide/rpki.md`: a listen-range group's actions govern every session it accepts | `rpki/rpki_config.go:parsePeerActions` keys the template via `KeyFor`; `rpki/rpki.go:739` resolves it from `req.peerGroup` | yes |
| `docs/guide/rpki.md`: a NAMED peer with no static remote IP still uses the global actions, reported at startup | the message text was read back from the producing branch in `rpki_config.go`, not from memory | yes |
| Category 2 (config syntax): no YANG leaf added or changed | the feature reads leaves that already exist (`remote > ip dynamic`, `range`); `grep` of `internal/component/bgp/yang/ze-bgp-conf.yang` shows both predate this work | yes |
| Categories 3, 4, 7, 8, 10, 11, 14 | no CLI command, no wire format, no SDK transport change, no test-infrastructure change, no counter added. `rpc.ValidateOpenInput.Group` is an additive `omitempty` field on an existing callback, documented at its declaration | yes |
| `make ze-doc-test` | exit 2 on `rules-points: writing.md is stale` ALONE, from another session's uncommitted `ai/rules/writing.md`. No other check in that target fails | yes |

## Core Insight

The identity needed to fix this already existed at both ends and nothing read it.
`PeerSettings.GroupName` was written by `buildDynamicPeerSettings`, and all seven
`filterapi.PeerFilterInfo` build sites populated `GroupName`, before any of this
work started. The defect was not a missing mechanism but an unread one, which is
why the fix needed no reactor change and why it fits as one more link on lookup
chains that already fell back name → address.

The second half is less comfortable. Delivering a value and KEYING it are one
change, not two. Phase 1 delivered the template visit to all 11 callers and left
their keys alone, on the reading that the delivery layer is behaviour-neutral.
Two callers then keyed the template into a namespace they share with peers:
`blackholecfg` failed the whole configuration, and `filter_community` silently
handed a peer its group's community policy. A new value in an existing map is a
namespace decision at every caller that owns a map.

The third half is the one that nearly shipped. A fallback chain written as "try
the next selector if the last answer was empty" is correct only when the source
can actually return empty, and `GetCapabilitiesForPeer` never could: it merged
the global capabilities into every answer. So the chain read as three probes in
precedence order and behaved as one, and it had behaved that way for the address
link long before this spec added the group link. The functional test could not
see it, because a `.ci` loads the plugins it names and the defect needs a second
plugin that declares a global capability. Extending a fallback chain means
reading what its source returns on a miss, not what the chain looks like.
