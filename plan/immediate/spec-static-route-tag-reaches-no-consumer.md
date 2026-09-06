# Spec: static-route-tag-reaches-no-consumer

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 1/3 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.**
`internal/plugins/static/yang/ze-static-conf.yang` declares `leaf tag` under
`static { table { route { ... } } }` with the description "Opaque tag Ze stores
against this route." The word opaque says Ze does not interpret the value, and a
network operator reads a route tag as the thing a route policy later matches on,
because that is what the leaf does in every other daemon that carries it. The
promise a reader takes is that the tag travels with the route.

**What Ze does instead.** The value is parsed and stored, and it reaches two
surfaces, both read-only. `parseRoute`
(`internal/plugins/static/config.go`) assigns it to `staticRoute.Tag`
(`internal/plugins/static/model.go`). `(*routeManager).showRoutes`
(`internal/plugins/static/inject.go`) copies it into the `Tag` field of
`showRoute`, which `show static route` renders as the `tag` JSON key.
`routesEqual` (`internal/plugins/static/diff.go`) compares it, so a change to
the tag alone re-applies the route. Those three are the only readers.

Two consumers that would make the tag mean something do not carry it.
`(*netlinkStaticBackend).buildRoute`
(`internal/plugins/static/backend_linux.go`) builds a `netlink.Route` with
`Dst`, `Priority` and `Table` and no tag, so no kernel route holds the value.
`(*routeManager).emitRouteChangeID` (`internal/plugins/static/inject.go`)
appends a `redistevents.RouteChangeEntry` carrying `Action`, `Prefix`, `Metric`
and `Table`, and `RouteChangeEntry`
(`internal/core/redistevents/events.go`) declares no `Tag` field, so no consumer
of the redistribute bus ever sees it. No route policy can match on the tag,
which is the one thing the leaf is for.

**What closing it means.** The implementer chooses between building the behavior
and refusing the leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. Building it means
deciding which consumer the tag is for, and the two candidates are not
equivalent. Adding a `Tag` field to `RouteChangeEntry` puts the value on the
redistribute bus, where a BGP or a policy consumer can match it, and it changes
a struct in `internal/core/` that every redistribution producer shares. Putting
it in the kernel route is the other reading of the promise, and the design has
to establish first whether the netlink route Ze programs has anywhere to carry
it. Neither is obviously right, so the design owes the answer to "which consumer
matches on this tag" before it writes any code. A refusal is the honest
placeholder if the answer is that no consumer wants it yet, and the spec then
stays open, which is the shape `plan/spec-vrf.md` records for `vrf`.

## Decision Taken (owner, 2026-09-06)

**The FRR behavior is right.** Thomas settled D-1 and D-3 on 2026-09-06, and the
answer covers both halves the survey below separates.

| # | Question | Answer |
|---|----------|--------|
| D-1 | Which consumer is this leaf for? | BOTH. A static route's `tag` is a LOCAL marker a redistribution rule matches on, which is the promise `docs/guide/static-routes.md` still prints ("opaque value for route policy matching in redistribute"). AND, when the route is redistributed into OSPF, the same value becomes the External Route Tag of the Type 5 / Type 7 LSA |
| D-2 | Route tag or `ospf { redistribute { source static { tag N } } }`? | OPEN, for the owner to confirm. The implementer's reading is below, and the code implements it |
| D-3 | Is the leaf refused at commit? | No. The behavior is built |

RFC 2328 defines the field in Appendix A.4.5, page 215, and Section 12.4.4
governs the origination that carries it: "External Route Tag / A 32-bit field
attached to each external route.  This is not used by the OSPF protocol itself.
It may be used to communicate information between AS boundary routers; the
precise nature of such information is outside the scope of this specification."
Ze therefore owes the field a value, and the RFC does not say which value.

### D-2: the reading this spec implements, for the owner to confirm

**The route's own tag WINS when it is nonzero. The per-source `tag N` under
`ospf { redistribute { source <src> } }` is the fallback for routes that carry
none.**

| Why | Evidence |
|-----|----------|
| The specific beats the general | `tag N` under `ospf/redistribute/source static` names a whole SOURCE. A tag on one route names that route |
| Ze already resolved this exact fork once, the same way, in the same dispatch path | `RouteChangeEntry.OriginAS` (`internal/core/redistevents/events.go`) is preferred over the batch `OriginASN` when nonzero, and `dispatchEntryToConsumer` (`internal/component/bgp/plugins/redistribute_egress/redistribute.go`) applies it. A second precedence rule in the opposite direction, two fields apart, is a trap for the next reader |
| It preserves every configuration that exists today | A route with no `tag` leaf takes the configured tag exactly as before this change |
| FRR agrees in the only place it can be compared | FRR's `router ospf` redistribute command carries no `tag` keyword at all: the redistributed route's own tag reaches the AS-External-LSA, and a route-map `set tag` is what overrides it. So FRR has no per-source tag for a per-route tag to lose to. NOT verified against FRR source in this session: no FRR checkout is present in this tree, and the interop lab runs FRR from a container image. Labelled UNVERIFIED |

**What zero means, and why it is a named guard.** A `tag 0` and an absent `tag`
leaf are already the same value everywhere in the static plugin: `mapUint32`
(`internal/plugins/static/config.go`) returns 0 for an absent key, and
`showRoute.Tag` carries `json:"tag,omitempty"`. Zero is also the OSPF default
external route tag. So zero is read as "this route carries no tag", the
precedence guard is named `externalRouteTag`
(`internal/plugins/ospf/redist_wiring.go`) rather than written as a bare `!= 0`,
and it carries its own test both ways (`ai/rules/principles.md`).

**Where zero is NOT read as absence.** A redistribution import rule that filters
on the tag records its match separately from the value (`ImportRule.MatchTag`),
so `import static { tag 0 }` means "only routes with no tag" and an import with
no `tag` leaf means "any tag". A single uint32 there would make the two
indistinguishable, which is the defect this spec exists to remove.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-10-as-external-asbr.md` - the engine side of OSPF
  redistribution, declared by the `// Design:` header of `redist_wiring.go` and of
  `ospf/redistribute/consumer.go`.
  → Constraint: `InjectExternal` is the one seam a redistributed route crosses to
  become an LSA, so a per-route attribute reaches OSPF through that signature or not
  at all.
- [ ] `docs/architecture/static-routes.md` - declared by `inject.go` and `config.go`.
  → Constraint: the plugin computes a diff and applies only the changes, so a tag
  change alone already re-applies the route and re-emits it.
- [ ] `docs/guide/redistribution.md` - the operator page for the `redistribute` root.
  → Decision: the page describes `import <source>` filtering by family only, so a tag
  filter is a new match and the page changes with the code.
- [ ] `docs/guide/static-routes.md` - the operator page for the `tag` leaf.
  → Decision: the page already promised "opaque value for route policy matching in
  redistribute". It was right about the intent and wrong about the build.
- [ ] `docs/architecture/testing/interop.md` - the interop lab.
  → Constraint: a scenario directory is NAMED, carries declarative inputs only, and
  MUST have a Go checker in `internal/le/interoplab/bgp/checkers.go`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2328.md` - OSPFv2, the AS-External-LSA this change fills a field of.
  → Constraint: read at the full text. Section 12.4.4 governs origination; the field
  itself is Appendix A.4.5, page 215: "External Route Tag / A 32-bit field attached to
  each external route.  This is not used by the OSPF protocol itself.  It may be used
  to communicate information between AS boundary routers; the precise nature of such
  information is outside the scope of this specification." The RFC fixes the encoding
  and says nothing about which value Ze puts there, so D-2 is a Ze decision.
- [ ] `rfc/short/rfc3101.md` - the NSSA Type 7, which carries the same field.
  → Constraint: the Type 7 an NSSA-internal ASBR originates takes the tag too, so the
  change is proven on both LSA types.

**Key insights:** (minimal context to resume after compaction)
- The tag has TWO consumers, not one: a redistribution import rule matches on it, and
  OSPF writes it into the External Route Tag.
- The Linux kernel has no route tag attribute. `RTA_FLOW` is the routing realm, which
  `ip rule` and the tc `route` classifier read to classify PACKETS, so writing a tag
  there would change forwarding behavior. The kernel is out of scope by fact, not by
  choice.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/static/config.go` - `parseRoute` reads `tag` through
  `mapUint32` and assigns `staticRoute.Tag`. No validation beyond the uint32 range.
  → Constraint: the value is accepted and stored whatever a consumer does with it.
- [ ] `internal/plugins/static/model.go` - `staticRoute.Tag uint32`.
- [ ] `internal/plugins/static/diff.go` - `routesEqual` compares `Tag`, so a tag
  change alone re-applies the route.
- [ ] `internal/plugins/static/inject.go` - `showRoutes` copies `Tag` into
  `showRoute.Tag` (`json:"tag,omitempty"`); `emitRouteChangeID` builds a
  `redistevents.RouteChangeEntry` with Action, Prefix, Metric and Table only.
- [ ] `internal/plugins/static/backend_linux.go` - `buildRoute` sets `Dst`,
  `Protocol`, `Priority`, `Table`, `Gw`/`LinkIndex`/`MultiPath`. Nothing else.
- [ ] `internal/core/redistevents/events.go` - `RouteChangeEntry` declares
  Action, Prefix, NextHop, Metric, Table, OriginAS. No Tag.
  → Decision: `OriginAS` and `Community` are the precedent for an additive
  per-entry field, so the shape of a `Tag` field is settled; its READER is not.
- [ ] `internal/component/config/redistribute/route.go` - `ImportRule` matches on
  source, destination and family. Nothing in Ze's redistribution rules or BGP
  filter types matches on a route tag.
- [ ] `internal/plugins/ospf/redist_wiring.go` - `externalParams` resolves the
  external route tag from the OSPF container's per-source `redistribute` entry
  (the `tag` leaf under `ospf/redistribute/source` in `ze-ospf-conf.yang`), NOT
  from the redistributed route.
  → Constraint: OSPF already has an operator-set tag for this exact route set, so
  a per-route tag needs a precedence rule against it.
- [ ] `internal/plugins/ospf/redistribute/consumer.go` - `InjectRoute` calls
  `ExternalInjector.InjectExternal(prefix, source)`. The injector seam carries no
  tag, so a per-route tag needs that signature changed.
- [ ] `internal/plugins/isis/redistribute/` - no tag of any kind. IS-IS carries no
  RFC 5130 administrative tag anywhere in this build.

**Consumer survey (the question the Task section says the design owes):**

| Candidate consumer | Can it carry a route tag? | Verdict |
|--------------------|---------------------------|---------|
| Linux kernel FIB | No. `enum rtattr_type_t` in `linux/rtnetlink.h` has no tag attribute. `RTA_FLOW` is the routing REALM pair, read by `ip rule` and the tc route classifier, and `netlink.Route.Realm` encodes it | Writing an opaque tag there changes packet classification. A realm is a different concept and would be a different leaf |
| Redistribution rules / BGP filters | No. `ImportRule` matches source, destination, family. No filter type matches a tag | Built here as a new match on the existing `ImportRule`, which is the leaf's original promise |
| BGP wire | No attribute means "route tag" | Mapping it to a community is an invention |
| OSPF external route tag (RFC 2328 Type 5, and the OSPFv3 external LSA) | Yes: `packet.ASExternalLSA.ExternalRouteTag`, already originated from `externalParams` | The only legible consumer, and it is already fed from OSPF-side config |

**Behavior to preserve:**
- `show static route` JSON key `tag` (`test/static/static-show.ci` asserts it).
- `routesEqual` re-applying a route whose tag alone changed.
- `ospf { redistribute { source <src> { tag N } } }` continuing to set the
  external route tag for sources that carry no per-route tag.

**Behavior to change:**
- `RouteChangeEntry` carries the tag, so every redistribution consumer sees it.
- An import rule can name a tag, and then imports only the routes that carry it.
- The External Route Tag of an AS-External-LSA (and of an NSSA Type 7) comes from the
  route when the route carries one, and from `ospf { redistribute { <src> { tag N } } }`
  otherwise.
- The `tag` leaf's `description` and `ze:help`, corrected in `b1e62392fe` to state that
  no consumer sees the value, say what the two consumers do with it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `static { table default { route <prefix> { tag N } } }`, parsed by `parseRoute`
  (`internal/plugins/static/config.go`) into `staticRoute.Tag`.
- `redistribute { destination <proto> { import <source> { tag N } } }`, parsed by
  `ExtractRedistributeRules` (`internal/component/config/loader_redistribute.go`).
- `ospf { redistribute { <source> { tag N } } }`, unchanged, now the FALLBACK.

### Transformation Path
1. `(*routeManager).emitRouteChangeID` (`internal/plugins/static/inject.go`) copies
   `staticRoute.Tag` into the `RouteChangeEntry` it appends to the batch.
2. `handleBatch` (`internal/component/bgp/plugins/redistribute_egress/redistribute.go`)
   sets `RedistRoute.Tag` from the entry and evaluates acceptance PER ENTRY, because
   the tag belongs to the route and not to the batch. `handleReplayBatch`
   (`replay.go`) does the same on the replay path.
3. `dispatchEntryToConsumer` (same file) copies the tag into
   `configredist.RouteEntry.Tag`.
4. `(*Consumer).InjectRoute` (`internal/plugins/ospf/redistribute/consumer.go`) passes
   it to `ExternalInjector.InjectExternal(prefix, source, routeTag)`.
5. `(*engine).InjectExternal` (`internal/plugins/ospf/redist_wiring.go`) and
   `(*engine).v6InjectExternal` (`origination_v6_external.go`) hand it to
   `externalParams`, which resolves the final value through `externalRouteTag`.
6. `OriginateExternal` / `OriginateNSSA` write it into the LSA's External Route Tag.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| static plugin → EventBus | `redistevents.RouteChangeEntry.Tag`, a value type in a pooled batch | Yes: `TestStaticEmitCarriesRouteTag` |
| EventBus → orchestrator → consumer | `configredist.RouteEntry.Tag` | Yes: `TestHandleBatchCarriesEntryTag` |
| OSPF consumer → OSPF engine | `ExternalInjector.InjectExternal` third parameter | Yes: `TestOSPFRedistConsumerPassesRouteTag` |
| Ze → FRR, on the wire | External Route Tag of the Type 5 AS-External-LSA | Yes: interop scenario `ospf-redist-static-tag-frr` |

### Integration Points
- `redistevents.RouteChangeEntry` - additive value field, the shape `OriginAS` set.
- `configredist.ImportRule` - a new match, evaluated by the existing `Accept`.
- `ospfredistribute.ExternalInjector` - the one seam OSPF origination crosses.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The tag rides the existing producer → bus → orchestrator → consumer chain. No new channel, no new event type |
| No unintended coupling (components stay isolated) | Yes | `redistevents` stays a leaf of value types. OSPF reads `RouteEntry.Tag`; BGP and IS-IS ignore it, as they ignore `OriginASN` and `Community` |
| No duplicated functionality (extends existing, does not recreate) | Yes | The filter is a field on `ImportRule`, judged by `ImportRule.Accept`. No second evaluator |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `Tag uint32` is a value field in the pooled entry; the pool's `clear()` resets it and nothing allocates |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes, with one qualification | Nothing registers, because nothing new is dispatched. `RouteChangeEntry.Tag` IS a field on a shared struct, and it is the same shape the package already carries for `OriginAS` and `Community`: a generic per-route attribute any producer may set and any consumer may read, named after the concept rather than after a plugin. No package spells `static` or `ospf` on the other's behalf |

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
| A-1 | The Linux kernel has no route attribute that carries an opaque tag, so the kernel is not a candidate consumer | `enum rtattr_type_t` in `linux/rtnetlink.h`; `RTA_FLOW = 0xb` is the realm, and `netlink.Route.Realm` (`vendor/github.com/vishvananda/netlink/route_linux.go`) encodes it | The design would owe a kernel path as well | read the header and the vendored encoder | confirmed |
| A-2 | A `tag 0` and an absent `tag` leaf are already the same value on a static route | `mapUint32` (`internal/plugins/static/config.go`) returns 0 for an absent key; `showRoute.Tag` carries `json:"tag,omitempty"` | Zero could not be read as "no tag" and the precedence would need a second field on the route as well | read both producers | confirmed |
| A-3 | Every input to `Evaluator.Accept` other than the tag is constant across one batch, so moving the call inside the entry loop changes no other verdict | `handleBatch` builds one `RedistRoute` from the batch's protocol and family | Per-entry evaluation would change acceptance for reasons unrelated to the tag | read `handleBatch`; the existing package tests still pass | confirmed |
| A-4 | The interop lab's `ze` container refuses to start without a `bgp { }` block once a `redistribute` root is present | measured: the first run of `ospf-redist-static-tag-frr` failed with `bgp: create reactor: build peers: missing required bgp { } block`, and `isis-redist-frr` and `ospfv3-redist-frr` both carry a bgp block | the scenario would need a different shape | the scenario ran and failed, then passed once the block was added | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Per-entry evaluation costs one rule-set walk for each redistributed route rather than one per batch | a redistribution burst showing more CPU in `evaluate` | The rule set is a handful of entries and the dispatch loop is already per entry, so the order of the work is unchanged. The batch-level call was DELETED rather than kept beside the new one: two evaluations of one rule set is layering (`ai/rules/no-layering.md`) |
| R-2 | An operator who set `ospf { redistribute { static { tag N } } }` and also tags individual routes sees the per-route value win where the per-source one used to apply | an external route arriving at a peer with a tag the operator did not expect | This is D-2 and it is the point of the change. The behavior is documented in `docs/guide/ospf.md`, `docs/guide/configuration.md` and the `tag` leaf's `ze:help`, and the interop scenario asserts both halves. Marked for the owner to confirm |
| R-3 | A route tag that is legitimately 0 cannot override a nonzero per-source tag | an operator asking for "tag this route 0, whatever the source tag says" | Accepted. Zero already means "no tag" everywhere in the static plugin and in OSPF, so the request cannot be expressed today in any daemon Ze is compared against. Naming it here so a later reader can find the decision |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An AS-External-LSA carries a tag the operator did not intend, which another AS boundary router's policy may match. No route is lost and no session drops: the field is not read by the OSPF protocol itself (RFC 2328 Appendix A.4.5). A wrong tag FILTER is heavier: a rule that names a tag nothing carries redistributes nothing, so a prefix stops being advertised |
| How is it reverted? | Single commit revert. No config migration: every leaf this change reads is optional, and a configuration written before it behaves identically |
| Who else touches this path? | `internal/component/config/redistribute/consumer.go` carried another session's `sort` to `slices` modernization while this work was in flight. `internal/component/config/loader.go` and `internal/core/family/family.go` are held by other sessions and are not in this change |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `static { route <p> { tag N } }` | → | `(*routeManager).emitRouteChangeID` | `TestStaticEmitCarriesRouteTag` |
| a batch entry carrying a tag | → | `dispatchEntryToConsumer` | `TestHandleBatchCarriesEntryTag` |
| `redistribute { destination ospf { import static { tag N } } }` | → | `ExtractRedistributeRules`, `ImportRule.Accept` | `TestExtractRedistributeRulesTagFilter`, `TestHandleBatchFiltersEntriesByTag` |
| a `RouteEntry` carrying a tag | → | `(*Consumer).InjectRoute` then `InjectExternal` | `TestOSPFRedistConsumerPassesRouteTag` |
| a redistributed route reaching OSPF | → | `externalParams`, `externalRouteTag`, `OriginateExternal` | `TestEngineInjectExternalRouteTagWins` |
| an operator's config file | → | the whole daemon | `test/parse/redistribute-tag-filter.ci`, and the interop scenario `ospf-redist-static-tag-frr` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A static route carries `tag 4242` and is redistributed | The route-change event for that prefix carries 4242; a route with no `tag` leaf carries 0 |
| AC-2 | A route-change entry carries a tag | Every consumer receives it on the `RouteEntry`, on the incremental path and on the replay path |
| AC-3 | The OSPF consumer injects a route carrying a tag | The tag reaches the external injector, for IPv4 and for IPv6 |
| AC-4 | A redistributed route carries `tag 4242` and `ospf { redistribute { static { tag 7 } } }` is configured | The AS-External-LSA carries External Route Tag 4242 |
| AC-5 | A redistributed route carries no tag, same configuration | The AS-External-LSA carries External Route Tag 7 |
| AC-6 | An import rule names `tag 42` | Only routes carrying 42 are imported; a route with 43, and a route with none, are rejected |
| AC-7 | An import rule names no tag | Every route from that source is imported, whatever tag it carries |
| AC-8 | One batch holds routes with different tags | Each route is judged on its own tag: the batch is not accepted or rejected whole |
| AC-9 | `import static { tag 0 }` | Only the untagged routes are imported |
| AC-10 | An operator writes the tag filter in a config file | `ze config validate` accepts it |
| AC-11 | An operator writes `tag 4294967296` | The daemon refuses the configuration and names the offending value |
| AC-12 | FRR peers with Ze over OSPFv2 and Ze redistributes a tagged and an untagged static route | FRR reports External Route Tag 4242 for the tagged prefix and 7 for the untagged one |
| AC-13 | An NSSA-internal ASBR redistributes a tagged route | The Type 7 carries the route's tag as well as the Type 5 would |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | tags a static route and redistributes it into OSPF, so a neighboring AS boundary router can match the tag in policy | config, `parseRoute`, `emitRouteChangeID`, orchestrator, OSPF consumer, `externalRouteTag`, AS-External-LSA, FRR | interop `ospf-redist-static-tag-frr` |
| 2 | tags a subset of static routes and redistributes only that subset | config, `ExtractRedistributeRules`, `ImportRule.Accept` per entry, consumer | `TestHandleBatchFiltersEntriesByTag`, `test/parse/redistribute-tag-filter.ci` |
| 3 | keeps a single tag for every route from one source, as before | `ospf { redistribute { static { tag N } } }`, `externalParams`, LSA | `TestEngineInjectExternalUntaggedRouteTakesConfiguredTag`, and the second assertion of the interop scenario |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStaticEmitCarriesRouteTag` | `internal/plugins/static/redist_tag_test.go` | AC-1 | red then green |
| `TestHandleBatchCarriesEntryTag` | `internal/component/bgp/plugins/redistribute_egress/redistribute_tag_test.go` | AC-2 | red then green |
| `TestHandleBatchUntaggedEntryCarriesZero` | same | AC-2 negative | green with AC-2 |
| `TestHandleBatchFiltersEntriesByTag` | same | AC-6, AC-8 | red then green |
| `TestImportRuleMatchesTag` | `internal/component/config/redistribute/route_tag_test.go` | AC-6, AC-7, AC-9 | red then green |
| `TestEvaluatorRulesCopiesTag` | same | the diagnostic copy keeps the filter | red then green |
| `TestExtractRedistributeRulesTagFilter` | `internal/component/config/loader_redistribute_tag_test.go` | AC-10 at the loader | red then green |
| `TestExtractRedistributeRulesTagOutOfRange` | same | AC-11 at the loader | red then green |
| `TestOSPFRedistConsumerPassesRouteTag` | `internal/plugins/ospf/redistribute/tag_test.go` | AC-3 IPv4 | red then green |
| `TestOSPFRedistConsumerPassesRouteTagIPv6` | same | AC-3 IPv6 | red then green |
| `TestExternalRouteTagPrecedence` | `internal/plugins/ospf/redist_route_tag_test.go` | the guard, both polarities | red then green |
| `TestEngineInjectExternalRouteTagWins` | same | AC-4 | red then green |
| `TestEngineInjectExternalUntaggedRouteTakesConfiguredTag` | same | AC-5 | red then green |
| `TestEngineInjectExternalNSSARouteTag` | same | AC-13 | red then green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| static `tag` | 0-4294967295 | 4294967295 | N/A (uint32) | 4294967296, refused by the YANG type |
| redistribute import `tag` | 0-4294967295 | 4294967295, asserted in `test/parse/redistribute-tag-filter.ci` seq 2 | N/A (uint32) | 4294967296, asserted in seq 3 |
| `externalRouteTag` route tag | 0-4294967295 | 0xFFFFFFFF, asserted in `TestExternalRouteTagPrecedence` | N/A | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-tag-filter` | `test/parse/redistribute-tag-filter.ci` | An operator writes a tagged static route, an OSPF per-source tag and a tag filter in one file, and `ze config validate` accepts it; an out-of-range tag is refused by name | red then green (drafted under `test/draft/parse/`, then promoted) |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-redist-static-tag-frr` | `test/interop/scenarios/` | FRR 10.3.1 | FRR reads External Route Tag 4242 on the tagged prefix and 7 on the untagged one, so Ze's Type 5 carries the route's tag and the per-source fallback | red then green, with the image rebuilt between the two |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/core/redistevents/events.go` - `RouteChangeEntry.Tag`
- `internal/plugins/static/inject.go` - `emitRouteChangeID` sets it
- `internal/plugins/static/yang/ze-static-conf.yang` - the `tag` leaf's `description` and `ze:help` now state what consumes the value
- `internal/component/config/redistribute/consumer.go` - `RouteEntry.Tag`
- `internal/component/config/redistribute/route.go` - `RedistRoute.Tag`, `ImportRule.Tag`, `ImportRule.MatchTag`, the comparison in `Accept`
- `internal/component/config/redistribute/evaluator.go` - `Rules()` copies the filter
- `internal/component/config/redistribute/yang/ze-redistribute-conf.yang` - the `tag` leaf under `import`
- `internal/component/config/loader_redistribute.go` - `importTag`
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - per-entry evaluation, the tag on `RouteEntry`
- `internal/component/bgp/plugins/redistribute_egress/replay.go` - the same on the replay path
- `internal/plugins/ospf/redistribute/redistribute.go` - the `ExternalInjector` signature
- `internal/plugins/ospf/redistribute/consumer.go` - passes `entry.Tag`
- `internal/plugins/ospf/redist_wiring.go` - `externalParams`, `externalRouteTag`
- `internal/plugins/ospf/origination_v6_external.go` - `v6InjectExternal` carries the tag
- `internal/plugins/ospf/register_multiaf.go` - `v6InjectorAF` forwards it
- `internal/le/interoplab/bgp/checkers.go` - the scenario's assertions
- `docs/guide/static-routes.md`, `docs/guide/redistribution.md`, `docs/guide/ospf.md`, `docs/guide/configuration.md` - the operator pages

## Files to Create
- `internal/plugins/static/redist_tag_test.go`
- `internal/component/bgp/plugins/redistribute_egress/redistribute_tag_test.go`
- `internal/component/config/redistribute/route_tag_test.go`
- `internal/component/config/loader_redistribute_tag_test.go`
- `internal/plugins/ospf/redistribute/tag_test.go`
- `internal/plugins/ospf/redist_route_tag_test.go`
- `test/parse/redistribute-tag-filter.ci`
- `test/interop/scenarios/ospf-redist-static-tag-frr/ze.conf`, `frr.conf`

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/redistribute/yang/ze-redistribute-conf.yang`, the `tag` leaf under `import` |
| YANG validation constraints | Yes | `type uint32` is the whole constraint the value admits; the range is the type's own and the parser names the offending value |
| YANG custom validators | N-A | The native type is sufficient. A tag names nothing in a registry, so there is no set to complete against |
| CLI commands/flags | N-A | No command changes. `show static route` already prints the tag |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes, automatic | A typed leaf completes from the schema |
| Functional test for new RPC/API | Yes | `test/parse/redistribute-tag-filter.ci` |
| Pipe completeness | N-A | No new output |
| Env var registration | N-A | No environment leaf |
| Doctor check for runtime dependencies | N-A | The change adds no file path, socket, service, module, port, procfs entry, netlink call, binary or certificate |
| Prometheus counters/metrics | No | `ze_bgp_redistribute_filtered_rule_total` already counts an entry the evaluator rejects, and a tag rejection increments it through the same call. No new series |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/redistribution.md` gains the tag filter; `docs/guide/static-routes.md` states what the tag now reaches |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, the OSPF redistribution paragraph |
| 3 | CLI command added/changed? | No | No command surface changed |
| 4 | API/RPC added/changed? | No | No RPC changed |
| 5 | Plugin added/changed? | No | No plugin registration changed |
| 6 | Has a user guide page? | Yes | `docs/guide/static-routes.md`, `docs/guide/redistribution.md`, `docs/guide/ospf.md` |
| 7 | Wire format changed? | No | The External Route Tag field already existed and was already originated. Its VALUE changes; the encoding does not |
| 8 | Plugin SDK/protocol changed? | No | `RouteChangeEntry` is internal to `internal/core/redistevents`; no `pkg/` type changed |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 2328 Appendix A.4.5 states the field is not used by OSPF itself and leaves its content outside the specification, so no requirement's implementation status moved. No `rfc/short/` row changes and no tagged test is added |
| 10 | Test infrastructure changed? | No | One scenario directory and one checker entry, both of which the existing discovery reads. No runner, format or budget changed |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not enumerate route tags |
| 12 | Internal architecture changed? | No | `docs/architecture/ospf/ospf-10-as-external-asbr.md` describes the injector seam and the Type 5 origination, neither of which changed shape. The parameter it gained is documented at the interface |
| 13 | Route metadata keys added/changed? | No | No `meta` key changed |
| 14 | Prometheus counters added/changed? | No | No new series |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registered or unregistered |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, and each is named | `ai/CODE-TO-DOCS.md` maps `internal/core/redistevents/events.go` and `redistribute_egress/redistribute.go` to `docs/architecture/core-design.md`, `docs/comparison.md`, `docs/features.md`, `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/guide/redistribution.md` and `docs/plugin-development/metrics.md`; and `ospf/redistribute/consumer.go` to `docs/architecture/ospf/ospf-10-as-external-asbr.md`, `ospf-ext-15-multi-af.md`, `docs/guide/ospf.md` and `docs/guide/configuration.md`. `docs/guide/redistribution.md`, `docs/guide/configuration.md` and `docs/guide/ospf.md` are updated. The rest are unaffected: none states what a `RouteChangeEntry` carries, what an import rule matches on, or what the external route tag is set from |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` and `docs/guide/static-routes.md` show the `tag` leaf; both were read against the parser and both remain valid |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- carry the tag from the producer to the OSPF LSA
   - Tests: `TestStaticEmitCarriesRouteTag`, `TestHandleBatchCarriesEntryTag`, `TestOSPFRedistConsumerPassesRouteTag`, `TestEngineInjectExternalRouteTagWins`
   - Files: `redistevents/events.go`, `static/inject.go`, `config/redistribute/consumer.go`, `redistribute_egress/redistribute.go`, `ospf/redistribute/{redistribute,consumer}.go`, `ospf/{redist_wiring,origination_v6_external,register_multiaf}.go`
   - Verify: the entry point exists and is reachable end to end
2. **Phase: the tag filter** -- an import rule matches on the tag
   - Tests: `TestImportRuleMatchesTag`, `TestExtractRedistributeRulesTagFilter`, `TestHandleBatchFiltersEntriesByTag`
   - Files: `config/redistribute/{route,evaluator}.go`, `loader_redistribute.go`, `ze-redistribute-conf.yang`, `redistribute_egress/{redistribute,replay}.go`
   - Verify: a batch with three tags dispatches one entry
3. **Phase: the operator's proof** -- the config surface and the wire
   - Tests: `test/parse/redistribute-tag-filter.ci`, interop `ospf-redist-static-tag-frr`
   - Files: the scenario directory, `internal/le/interoplab/bgp/checkers.go`, the four guide pages, the `tag` leaf's two help texts
   - Verify: FRR reports both tags; the discrimination cut turns the scenario red

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol, and a test named in the TDD table |
| Feature completeness | Both halves are built: the filter AND the LSA. Neither alone satisfies the owner decision |
| Correctness | `externalRouteTag` prefers the route's tag ONLY when it is nonzero, and the per-source tag still applies to every route that carries none |
| Correctness | The tag filter is judged per ENTRY on both the incremental and the replay path, not per batch |
| Naming | `MatchTag` names the guard the zero would otherwise be. The YANG leaf, the Go field and the doc all spell the concept `tag` |
| Data flow | `redistevents` stays free of any plugin's name; the OSPF-specific reading of the tag lives in the OSPF engine alone |
| Rule: `ai/rules/principles.md` | Every zero that means "absent" is named, commented and tested on both sides |
| Rule: `ai/rules/no-layering.md` | The batch-level `ev.Accept` call was deleted, not kept beside the per-entry one |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The tag reaches every consumer | `./le test-unit all`, or the four packages through `./le job run label unit-pkg command go test ...` |
| The tag filter works | the same, over `internal/component/config` and `internal/component/config/redistribute` |
| An operator can write it | `./le functional parse` |
| FRR reads what Ze writes | `INTEROP_SCENARIO=ospf-redist-static-tag-frr ./le integration interop` |
| No discrimination cut survives | `grep -rn "MUTATION-APPLIED" internal/ test/ cmd/` returns nothing |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The tag arrives from the operator's config file, never from a peer. The YANG type bounds it to a uint32 and `importTag` refuses anything the type would not hold, by name rather than by clamping |
| Fail closed | A tag filter that cannot be parsed FAILS THE CONFIG LOAD. It does not fall back to "import everything", which would be a silent widening of what leaves the router |
| Resource exhaustion | The filter adds no allocation and no unbounded loop: one uint32 comparison inside a loop that already ran |
| Error leakage | The refusal names the source and the value the operator wrote, both of which the operator already holds |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- **The consumer survey asked the wrong question.** It looked for THE consumer, found
  one, and concluded the leaf was for OSPF. The leaf is for two things at once: a local
  marker a rule matches on, and a value a protocol carries. A survey that asks "which
  existing code can read this" cannot find the consumer that does not exist yet.
- **A per-route attribute forces the acceptance decision down to the route.** The
  orchestrator judged a whole batch because every input to the judgement was a batch
  property. The first per-route input moves the call, and it moves it on the replay
  path too. The tell that this is right rather than expensive: the dispatch loop under
  the decision was already per entry.
- **`OriginAS` had already answered the precedence question.** `RouteChangeEntry.OriginAS`
  is preferred over the batch `OriginASN` when nonzero, two fields above the one this
  spec adds. Answering the same fork the other way would have put two opposite rules in
  one struct.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The route's own tag wins; the per-source `tag` is the fallback | The per-source tag stays authoritative and the route's tag is the fallback | The specific beats the general, `RouteChangeEntry.OriginAS` already resolves the same fork that way in the same dispatch path, and every existing configuration keeps its behavior. Marked D-2, for the owner to confirm |
| `ImportRule.MatchTag bool` beside `ImportRule.Tag uint32` | One `uint32` where 0 means "no filter", matching the `Families` idiom | Zero is a tag a route really carries, so one field makes "import the untagged routes" indistinguishable from "import everything". The `Families` idiom is safe because an empty family list is not a family |
| A third parameter on `InjectExternal` | A value struct carrying prefix, source and tag; a second method beside the first | The struct is machinery for one field, and a second method is layering. Three parameters still reads at the call site |
| The tag filter lives on the existing `ImportRule` | A new filter type, or a BGP filter plugin | The operator already writes `import <source> { family [...] }`. A tag is another narrowing of the same rule, judged by the same `Accept` |
| The kernel route carries nothing | Write the tag into `netlink.Route.Realm` (`RTA_FLOW`) | A realm is read by `ip rule` and the tc `route` classifier to classify PACKETS. Writing an opaque tag there changes forwarding. Linux has no route tag attribute at all |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- A route tag of 0 cannot override a nonzero per-source OSPF tag, because zero means
  "no tag" throughout. R-3 records the reasoning.
- IS-IS carries no route tag. RFC 5130 defines an administrative tag sub-TLV and
  nothing in this build implements it, so a redistributed route's tag reaches the
  IS-IS consumer on the `RouteEntry` and that consumer ignores it. This is an absent
  FEATURE rather than a gap this spec leaves: no IS-IS behavior in the tree claims to
  carry a tag.
- BGP carries no route tag either. No path attribute means "route tag", and mapping it
  to a community would be an invention rather than an implementation.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
