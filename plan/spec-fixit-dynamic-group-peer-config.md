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
| Phase | 2/5 |
| Deferral shard | - |
| Updated | 2026-08-13 |

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
| R-1 | The visitor signature change touches 11 callers, and a caller that ignores the new dynamic visit keeps the defect silently | a plugin still misses on a dynamic member after the change | one guard test enumerates the callers and fails when a new one appears unhandled |
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
| `TestGetFilterConfigFallsBackToGroup` | `internal/component/bgp/plugins/role/role_test.go` | AC-5 at the OTC decision | phase 4 |
| `TestRPKIPeerActionsForDynamicGroup` | `internal/component/bgp/plugins/rpki/rpki_config_test.go` | AC-6 | phase 5 |
| `TestBlackholeRuleForDynamicGroup` | `internal/component/bgp/blackholecfg/blackholecfg_test.go` | AC-7 at the RIB decision, once a group identity reaches it | phase 3 |
| `TestDynamicGroupPluginConfigMatchesAStaticPeer` | `internal/component/bgp/configjson/dynamic_group_test.go` | AC-8, the guard: one config states the same plugin leaves on a static peer and on a dynamic group, and every caller must resolve them alike | phase 6 |

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
  name and visits a dynamic group's template; `IsDynamicGroup`, `PeerConfigKey` and
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
