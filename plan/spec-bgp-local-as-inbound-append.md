# Spec: bgp-local-as-inbound-append -- the RFC 7705 inbound append as a configuration recipe

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | `plan/immediate/spec-bgp-local-as-options.md`, `plan/immediate/spec-bgp-as-migration.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 7705 Section 3.3 item 1 asks a router carrying a per-neighbor "Local AS" to
append that ASN to the AS_PATH of routes RECEIVED from that neighbor: "The
router SHOULD append the configured 'Local AS' ASN in the AS_PATH attribute
before installing the route or advertising the UPDATE to an iBGP neighbor." Ze
does not do it. Every AS_PATH write site is gated on an eBGP DESTINATION, so
the requirement (`RFC7705-3.3-9` in `rfc/pending/rfc7705.md`) is an
unimplemented SHOULD, and it is why the `no-prepend` enum of `local-options`
suppresses nothing.

The owner ruled on 2026-09-06 that this is not engine work: "we should write a
spec to make this configuration best practice easy to follow when performing
ASN migration but this is not a normal requirement on normal operation of BGP.
ASN migration are rare and these are SHOULD, I will defer this work for later.
Create a spec, I have the feeling this can be expressed as an import or export
filter."

The hypothesis holds. An import-chain `as-path-prepend 1` on the migrated
session produces exactly Section 3.3 item 1, with no engine change to the
append itself. So the deliverable is a documented CONFIGURATION RECIPE, plus
the three things ze owes before that recipe is safe to publish: one wire defect
the recipe walks into, two config-load refusals that stop the recipe from being
attached where it is wrong, and the tests for each.

This spec is DEFERRED work. It sits at the top level of `plan/` because the
first release ships without it: an operator who is not migrating an ASN never
meets it (`plan/README.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/structural-forwarding.md` - the rail that decides where an inbound append can live
  → Constraint: ze forwards the RECEIVED wire per destination and records each destination's edits as intent; it does not rebuild an UPDATE from the Loc-RIB. An append written onto the received payload therefore reaches every downstream reader at once, and an append written into the RIB reaches none of them.
- [ ] `ai/patterns/config-option.md` - read to size a new YANG leaf
  → Decision: no new leaf is added. The recipe is expressible with the leaves that already exist, and a leaf whose only job is to select a filter the operator can name himself is a second declaration of the same fact.
- [ ] `ai/rules/config.md` - YANG versus env var, and naming
  → Constraint: a tunable defaults to a YANG leaf, and every node declares `description` plus `ze:help`. Applied here to the EXISTING `as-path-prepend` and `local-options` texts, whose prose this spec corrects rather than extends.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/pending/rfc7705.md` - the parked summary; `rfc/short/rfc7705.md` does not exist yet
  → Constraint: enrolment of RFC 7705 belongs to `plan/immediate/spec-bgp-as-migration.md`. This spec adds no `rfc/short/` row and no `RFC requirement:` tag, because `./le rfc check` answers "unknown RFC requirement" until that spec lands. The seam is: that spec owns the LEDGER, this spec owns the RECIPE and the defects under it.
- [ ] `rfc/short/rfc7947.md` - route server transparency
  → Constraint: Section 2.2.2.1 forbids a route server from modifying AS_PATH "in any other way", which is wider than the prepend it names. An inbound append on a route-server path is inside that prohibition, and RFC 7705 never mentions route servers, so the interaction is ze's to settle.

**Key insights:** (minimal context to resume after compaction)
- The append is expressible today: `bgp { policy { modify NAME { set { as-path-prepend 1; } } } }` named in the migrated peer's `filter { import }` chain.
- The prepended ASN is not typed by the operator. `ExtractASPathPrependOps` is handed `peer.settings.LocalAS` at the import call site, which IS the per-peer `local-as` override.
- The import chain rewrites the received payload BEFORE the cache, the plugin dispatch and both forward rails, so one append satisfies both moments Section 3.3 item 1 names.
- Three defects sit under the recipe: a four-octet-only prepend segment, no refusal on a route-server client, and no refusal against a peer that also declares `no-prepend`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - the ingress pipeline. `orderedIngressSteps` runs over the received UPDATE; when a step returns a modified payload the function rebuilds `wireUpdate` and overwrites `msg.RawBytes`, `msg.WireUpdate` and `msg.AttrsWire`. That block runs BEFORE `r.recentUpdates.Add`, before the RS fast path and before plugin dispatch, so the rewritten payload is what the RIB plugin, the adj-RIB-in plugin and both forward rails see.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - the two call sites of the prepend extractor: the import chain passes `peer.settings.LocalAS`, the export chain passes `destLocalAS`. The import call site passes `srcASN4` to `ExtractRemovePrivateASOps` on the line above and passes no width to the prepend extractor.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `ExtractASPathPrependOps` builds an AS_SEQUENCE segment of N copies of `localAS`, four octets each, unconditionally, and emits it as `AttrModPrepend`. It takes no ASN4 argument.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - `aspathHandler` splices a prepend op in front of the existing AS_PATH value and re-emits the attribute. It performs no width check and no transcode.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `secondaryPrependAS` answers the ASN behind the per-peer local-as on the EGRESS prepend, reading `LocalASReplaceAS` and deliberately not reading `LocalASNoPrepend`. `localASPrependFor` carries the pair to the announce rail.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the general forward rail. The whole AS-path intent, prepend included, sits inside `if facts.isEBGP`. An iBGP destination gets no AS_PATH edit at all.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - the route-server rail. Same `if facts.isEBGP` gate, with the prepend further gated on `!facts.rsClient`. It forwards `update.WireUpdate.Payload()`, the RECEIVED wire, and never reads the Loc-RIB.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - the announce rail for routes ze ORIGINATES. `prependApplies` requires `!isIBGP`, so this rail also never writes an AS number toward an internal peer.
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerSettings` reads `session > asn > local` into `PeerSettings.LocalAS` (the per-peer override), keeps the router's own ASN in `GlobalLocalAS`, and stores the two `local-options` enums into `LocalASNoPrepend` and `LocalASReplaceAS`.
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `LocalASNoPrepend` is declared and stored. A tree-wide grep finds no non-test reader: the flag selects nothing today, which is the honest state while there is no inbound append to suppress.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - stores what arrived and replays it through `relayRoutes`, which re-enters `forwardUpdateCore` (`reactor_api_relay.go`). Replay therefore reproduces the egress transform from the stored bytes, and inherits whatever the import chain wrote into them.
- [ ] `internal/component/bgp/reactor/filter/loop.go` and `internal/component/bgp/filterapi/filterapi.go` - `LoopIngress` scans the received path for `src.LocalAS`, and the stage constants put protocol filters at 0 and the per-peer chain at 300, so an import-chain prepend runs AFTER ze's own AS_PATH loop check on that route.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `session > asn > local-options` declares the two enums; `filter { import }` exists at the global level, the group level and the peer level, and the three chains accumulate in that order.
- [ ] `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` - `as-path-prepend`, `uint8` range 1..32, whose `ze:help` already states "Ze prepends the local-as of the peer, which the session config holds".
- [ ] `internal/component/bgp/plugins/rs/server.go` - the rs plugin's `OnConfigure` receives config sections and reads the ones whose `Root` is `bgp`, and returning an error from it fails the config apply.
- [ ] `internal/component/config/yang/validator_registry.go` - `ValidateFn` is `func(path string, value any) error`, so a leaf validator sees one path and one value and cannot compare two leaves.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The outbound rail landed in `5001022d13` and proven in `b60737ac8`: toward the migrated peer, `local-as` alone and `local-as` + `no-prepend` both emit the override followed by the globally configured ASN, and `replace-as` emits the override alone. `no-prepend` MUST stay out of `secondaryPrependAS`.
- `test/plugin/bgp-local-as-options.ci`, `test/plugin/bgp-local-as-inbound-untouched.ci` and the four-octet assertions of `TestLocalASFourOctetTowardTwoOctetPeer` keep their current expectations for every peer that does NOT carry the recipe.
- The RS rail's transparency: an RS client's AS_PATH is not modified on egress.
- Ingress loop detection stays at stage 0, ahead of the peer chain.

**Behavior to change:** (only what the user asked for)
- `ExtractASPathPrependOps` learns the source encoding width, so a prepend onto a two-octet path is encoded in two octets instead of producing a mixed-width AS_PATH.
- Config load refuses an `as-path-prepend` modifier on the import chain of a route-server client (RFC 7947 Section 2.2.2.1).
- Config load refuses an `as-path-prepend` modifier on the import chain of a peer that also declares `local-options no-prepend`, because the two state opposite things about the same session.
- `docs/guide/configuration.md` gains the AS-migration recipe, and the `no-prepend` and `as-path-prepend` schema texts are corrected to match it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An UPDATE received from the eBGP peer that carries `session > asn > local` (the legacy ASN), on a session whose `filter > import` chain names a `modify` definition carrying `as-path-prepend 1`.
- Format at entry: raw UPDATE body, AS_PATH in the SOURCE peer's negotiated ASN width.

### Transformation Path
1. `notifyMessageReceiver` (`reactor_notify.go`) builds the ingress `PeerFilterInfo` and walks `orderedIngressSteps`.
2. Stage 0 filters run first: `LoopIngress` scans the RECEIVED path for `src.LocalAS`, which on this session is the legacy ASN.
3. Stage 300, the per-peer chain, dispatches to `bgp-filter-modify`, which returns a text delta carrying `as-path-prepend 1`.
4. `ExtractASPathPrependOps` turns the directive into one `AttrModPrepend` op holding the peer's `LocalAS`, and `aspathHandler` splices it in front of the existing AS_PATH.
5. The modified payload replaces `msg.WireUpdate` and `msg.RawBytes`, and is cached under the message id.
6. Every downstream reader takes the appended path: the RIB plugin (best-path, `show bgp`), the adj-RIB-in store (and therefore replay), the general forward rail, and the RS fast path.
7. Each egress rail then applies its own rule on top: an iBGP destination gets no further edit, a native eBGP destination gets the globally configured ASN prepended, and the migrated peer itself is never a destination for its own route.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ bgp-filter-modify | `filter-update` RPC; the plugin returns a text delta and the reactor converts it to wire ops | Yes -- `handleFilterUpdate` (`filter_modify.go`) and `ExtractASPathPrependOps` (`filter_delta.go`) |
| Engine ↔ bgp-rs | the RS fast path forwards the post-filter payload; `bgp-rs` forwards the same cached entry | Yes -- `reactorForwardRS` reads `update.WireUpdate.Payload()` after the rewrite |
| Engine ↔ bgp-adj-rib-in | plugin dispatch delivers the post-filter message, and replay relays the stored bytes back through `forwardUpdateCore` | Yes -- `reactor_api_relay.go` |
| Config ↔ bgp-rs | `OnConfigure` receives the `bgp` section and can refuse the apply | Yes -- `rs/server.go` |

### Integration Points
- `bgp { policy { modify NAME { set { as-path-prepend 1; } } } }` - the existing modifier vocabulary; the recipe adds no new directive.
- `neighbor <addr> { filter { import } }` and its group-level twin - the per-neighbor and per-neighbor-group scoping RFC 7705 Section 3.3 requires.
- `session > asn > local` - the single declaration of the legacy ASN. The recipe reads it through the engine, never through a literal in the policy.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the append rides the registered ingress filter pipeline, at the stage the peer chain already occupies |
| No unintended coupling (components stay isolated) | Yes | the RS refusal lives in the plugin that owns `rs-client`, and the `no-prepend` refusal in the plugin that owns `as-path-prepend` |
| No duplicated functionality (extends existing, does not recreate) | Yes | no new leaf, no new directive, no second inbound rail |
| Zero-copy preserved where applicable (refs, not copies) | No | the modified payload is heap-allocated by the existing filter path, as `reactor_notify.go` states at the rebuild. The recipe adds no copy of its own, and pays this cost only on a session that carries the modifier |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | both refusals are plugin-owned `OnConfigure` checks; delete the plugin and its refusal goes with it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `bgp` config section delivered to a plugin's `OnConfigure` carries the per-peer `filter { import }` chains and the `policy { modify }` definitions in one subtree, so a plugin can compare them. | `filter_modify.go` parses `policy { modify }` out of the `bgp` section via `configjson.ParseBGPSubtree`; `rs/server.go` reads the same root. | Both refusals lose their home and must move to a whole-tree config validator; a leaf `ze:validate` cannot host them, because `ValidateFn` takes one path and one value and sees no sibling. | Reading the delivered section through `configjson` at implementation, before writing either refusal. | unvalidated |
| A-2 | An import-chain prepend onto a path received from a two-octet peer produces a malformed AS_PATH today. | `ExtractASPathPrependOps` writes four octets per ASN with no width argument, while the sibling extractor on the adjacent line takes `srcASN4`; `aspathHandler` splices without transcoding. | The width fix is unnecessary and AC-2 drops. | A unit test that drives the import chain with a source context whose `ASN4` is false, and decodes the result. | unvalidated |
| A-3 | The route server relays what the import chain wrote, so an RS import prepend reaches other clients. | `reactorForwardRS` forwards `update.WireUpdate.Payload()`, and the cache entry is built from the post-filter `wireUpdate`. | The RS refusal is defence in depth rather than a fix, and stays anyway. | The `.ci` scenario in the Functional Tests table, which reads what a second client receives. | unvalidated |
| A-4 | The recipe leaves the outbound rail untouched. | The import chain rewrites the received payload; the egress prepend is computed from the DESTINATION peer's settings in `secondaryPrependAS`, which reads no source state. | The 2026-09-05 outbound work regresses and its tests go red. | Running `test/plugin/bgp-local-as-options.ci` unchanged beside the new scenario. | unvalidated |
| A-5 | Ze's own ingress loop detection does not drop a route the recipe just appended to. | `LoopIngress` is registered at the protocol stage (0), the peer chain at the peer-chain stage (300). | The recipe is unusable and the append must move ahead of stage 0, which is engine work this spec does not plan. | A `.ci` assertion that the route survives ingress with the modifier attached. | unvalidated |
| A-6 | An operator can express the whole recipe without restating the legacy ASN. | The import call site hands `peer.settings.LocalAS` to the extractor, and the schema's own `ze:help` says the same. | The recipe carries a second declaration of the ASN, and the spec owes a way to derive it. | `TestImportPrependUsesPeerConfiguredLocalAS`, which changes the peer's `local` and expects the emitted ASN to follow. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The same modify definition attached to a peer with NO local-as override prepends the router's own globally configured ASN inbound. Every iBGP neighbor then drops the route on its own loop check, and ze's stage-0 check does not catch it because the peer chain runs after it. | Routes accepted by ze and invisible on a neighbor. | The documented recipe names the precondition, and the recipe's own `.ci` covers the peer that carries the override. A refusal for this case is NOT specified: `as-path-prepend` is a general policy action whose ordinary use is prepending the local AS on egress, and refusing it inbound would remove a legitimate action. |
| R-2 | An operator attaches the modifier at the GLOBAL `filter { import }` level, and every session appends. | Every received route grows one AS. | The recipe states the per-peer and per-group placement and says the global chain is wrong for it; RFC 7705 Section 3.3's per-neighbor MUST is quoted beside it. |
| R-3 | The width fix changes the encoding of an EXISTING `as-path-prepend` user on the export chain. | Policy tests that read AS_PATH after a prepend. | The fix takes the width from the call site that already knows it, and the export site already passes `asn4` to its sibling extractor. Both call sites get a test at both widths. |
| R-4 | The two refusals reject a config an operator already runs, on upgrade. | Config load failing after an upgrade. | Pre-release, so no such operator exists; the error names the peer, the definition and the leaf, and says which of the two to remove. |
| R-5 | The recipe is documented and nobody can find it. | An operator asks how to migrate an ASN. | The page is reachable from the local-as section of `docs/guide/configuration.md`, and the `local-options` `ze:help` points at it. |
| R-6 | The recipe and `local-options` end up as two ways to state the same session behavior, and drift. | An operator sets `no-prepend` and the modifier on one peer. | The refusal in AC-4 makes the contradiction impossible to configure, and the `no-prepend` description says which of the two governs the inbound direction. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A route learned from the migrated peer carries the wrong AS_PATH inside the AS: too short and the legacy ASN is invisible to loop detection at every other router, too long or malformed and a neighbor drops the session's routes or the UPDATE itself. On a route server, a modified path reaches clients whose decision process RFC 7947 says must see the original. |
| How is it reverted? | The recipe is config: remove the modifier name from the import chain and the next UPDATE is unchanged. The two refusals and the width fix are a single commit revert. Routes already re-advertised carry the path onward. |
| Who else touches this path? | `plan/immediate/spec-bgp-local-as-options.md` owns the outbound rail and the `local-options` semantics; `plan/immediate/spec-bgp-as-migration.md` owns RFC 7705 enrolment and the `rfc/short/` ledger. Neither is edited here. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator names a `modify` definition carrying `as-path-prepend 1` in a migrated peer's import chain, and that peer sends a route | → | `ExtractASPathPrependOps` on the import call site, with `peer.settings.LocalAS` | `test/plugin/bgp-local-as-inbound-recipe.ci` |
| The same route reaches an iBGP neighbor and a native eBGP neighbor | → | `forwardUpdateCore` reading the rewritten `update.WireUpdate` | `test/plugin/bgp-local-as-inbound-recipe.ci`, conn=2 and conn=3 |
| The migrated peer negotiated two-octet ASNs | → | the width argument added to `ExtractASPathPrependOps` | `TestImportPrependEncodesAtTheSourceASNWidth` |
| An operator names the modifier on a route-server client's import chain | → | the rs plugin's `OnConfigure` refusal | `TestRouteServerRefusesInboundASPathPrepend` |
| An operator names the modifier on a peer that also declares `local-options no-prepend` | → | the filter-modify plugin's `OnConfigure` refusal | `TestNoPrependRefusesInboundASPathPrepend` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer carrying `session > asn > local 64510` (router global 64500) with `as-path-prepend 1` on its import chain sends a route whose AS_PATH is 64496 | The route installed and re-advertised carries 64510 64496, and the operator states the ASN nowhere in the policy |
| AC-2 | The same peer negotiated two-octet ASNs only | The AS_PATH after the append decodes as one well-formed two-octet AS_SEQUENCE, and no four-octet segment is spliced into a two-octet path |
| AC-3 | The same peer is declared `rs-client` | Config load fails, naming the peer, the modify definition, the `as-path-prepend` leaf and RFC 7947 Section 2.2.2.1, and no client receives a modified path |
| AC-4 | The same peer also declares `local-options no-prepend` | Config load fails, naming both settings and stating that one of the two must go |
| AC-5 | A route from the migrated peer reaches an iBGP neighbor and a native eBGP neighbor, with the recipe attached | The iBGP neighbor receives 64510 64496; the native eBGP neighbor receives 64500 64510 64496 |
| AC-6 | Any peer configured with `local-as`, with the recipe NOT attached | Inbound behavior is unchanged from today: the received AS_PATH is stored and re-advertised verbatim, and the native eBGP neighbor receives 64500 64496 |
| AC-7 | Any of the three documented outbound configurations toward the migrated peer | The AS_PATH the peer receives is byte-identical to what `test/plugin/bgp-local-as-options.ci` asserts today |
| AC-8 | An operator reads the local-as section of the configuration guide | The recipe is there, with the per-peer placement, the precondition that the session carries an override, the two refusals, and what the iBGP and native eBGP neighbors each see |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Migrates an ASN: configures `local-as` on the legacy-facing session and attaches the inbound recipe | config → import chain → `ExtractASPathPrependOps` → rewritten payload → RIB, adj-RIB-in, both forward rails | `test/plugin/bgp-local-as-inbound-recipe.ci` |
| 2 | Runs `show bgp` for a route learned from the migrated peer | rewritten payload → rib plugin best-path → CLI | `test/plugin/bgp-local-as-inbound-recipe.ci`, the show assertion |
| 3 | Runs a route server and tries the same recipe on a client | config load → rs plugin `OnConfigure` → refusal | `TestRouteServerRefusesInboundASPathPrepend` and `test/plugin/bgp-local-as-inbound-route-server.ci` |
| 4 | Ends the migration and removes the modifier from the import chain | config reload → import chain empty → received path verbatim | `test/plugin/bgp-local-as-inbound-untouched.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestImportPrependUsesPeerConfiguredLocalAS` | `internal/component/bgp/reactor/rfc7705_local_as_inbound_test.go` | the ASN in the emitted op is the peer's `LocalAS` override, not the router's global ASN and not a literal | |
| `TestImportPrependEncodesAtTheSourceASNWidth` | `internal/component/bgp/reactor/rfc7705_local_as_inbound_test.go` | two-octet source gives a two-octet segment, four-octet source a four-octet segment, and both decode as one AS_PATH | |
| `TestExportPrependEncodesAtTheDestinationASNWidth` | `internal/component/bgp/reactor/filter_delta_test.go` | the sibling call site keeps its behavior at both widths (R-3) | |
| `TestRouteServerRefusesInboundASPathPrepend` | `internal/component/bgp/plugins/rs/config_refusal_test.go` | `OnConfigure` returns an error naming the peer, the definition and the leaf | |
| `TestNoPrependRefusesInboundASPathPrepend` | `internal/component/bgp/plugins/filter_modify/config_refusal_test.go` | `OnConfigure` returns an error naming both settings | |
| `TestSecondaryPrependASIgnoresNoPrepend` | `internal/component/bgp/reactor/rfc7705_local_as_test.go` | the outbound rail is unchanged by anything in this spec (A-4) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `as-path-prepend` count | 1-32 | 32 | 0 | 33 |
| `session > asn > local` (legacy ASN) | 1-4294967295 | 4294967295 | 0 | N/A |
| legacy ASN against a two-octet peer | 1-65535 fits two octets | 65535 | 0 | 65536 encodes as AS_TRANS with the real value in AS4_PATH |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-local-as-inbound-recipe` | `test/plugin/bgp-local-as-inbound-recipe.ci` | the migrated peer sends a route; the iBGP neighbor sees 64510 64496 and the native eBGP neighbor sees 64500 64510 64496 | |
| `bgp-local-as-inbound-route-server` | `test/plugin/bgp-local-as-inbound-route-server.ci` | a second route-server client receives the path the first client sent, byte-identical, and the config that would have modified it is refused | |
| `bgp-local-as-inbound-untouched` | `test/plugin/bgp-local-as-inbound-untouched.ci` | unchanged: with no modifier attached, nothing rewrites the received path | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-local-as-inbound-append-frr` | `test/interop/scenarios/` | FRR | an FRR iBGP neighbor of ze accepts and installs the appended path and its own loop detection does not reject it; removing the recipe from ze's config and rebuilding removes the legacy ASN from what FRR shows | |

## Files to Modify
- `internal/component/bgp/reactor/filter_delta.go` - `ExtractASPathPrependOps` takes the ASN width and encodes the segment at it
- `internal/component/bgp/reactor/filter_ordered.go` - both call sites pass the width they already hold
- `internal/component/bgp/plugins/rs/server.go` - `OnConfigure` refuses an inbound `as-path-prepend` on a route-server client
- `internal/component/bgp/plugins/filter_modify/config.go` - `OnConfigure` refuses an inbound `as-path-prepend` on a peer declaring `no-prepend`
- `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` - the `as-path-prepend` `ze:help` states which ASN each direction prepends, and names the two refusals
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `no-prepend` enum description names the recipe it refuses to sit beside
- `docs/guide/configuration.md` - the AS-migration recipe, in the local-as section
- `docs/architecture/bgp/egress-attribute-rules.md` - the import direction of `as-path-prepend` and its width rule
- `docs/config-reference.md` - regenerated for the changed schema texts
- `docs/architecture/core-design.md` - declared by `filter_delta.go`, `filter_delta_handlers.go`, `rs/server.go` and `filter_modify/config.go`. Named here as UNAFFECTED: the page describes the filter pipeline and the plugin model as structures, and this spec changes neither. It gains no edit unless the width fix or a refusal changes a structure it states
- `docs/architecture/api/architecture.md` - declared by `filter_ordered.go`. Named here as UNAFFECTED: the page describes the unified filter pipeline and its stages, which this spec leaves exactly as they are; the width argument is internal to one extractor and the refusals sit in config load, not in the pipeline

## Files to Create
- `internal/component/bgp/reactor/rfc7705_local_as_inbound_test.go` - the import-chain unit tests
- `internal/component/bgp/plugins/rs/config_refusal_test.go` - the RS refusal
- `internal/component/bgp/plugins/filter_modify/config_refusal_test.go` - the `no-prepend` refusal
- `test/plugin/bgp-local-as-inbound-recipe.ci` - the recipe end to end
- `test/plugin/bgp-local-as-inbound-route-server.ci` - the route-server path
- `test/interop/scenarios/bgp-local-as-inbound-append-frr/` - the FRR scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf is added; the recipe uses `policy { modify }` and `filter { import }` as they stand |
| YANG validation constraints | No | the existing `uint8` range 1..32 on `as-path-prepend` is unchanged |
| YANG custom validators | No | both refusals are cross-leaf, and a `ValidateFn` sees one path and one value, so they live in plugin `OnConfigure` instead (A-1) |
| CLI commands/flags | No | no command is added; the recipe is config |
| CLI grammar (keyword before value) | N-A | no command is added |
| Editor autocomplete | No | the enums and leaves involved already complete |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-local-as-inbound-recipe.ci`, `test/plugin/bgp-local-as-inbound-route-server.ci` |
| Pipe completeness | N-A | no command output is added |
| Env var registration | N-A | nothing under `environment/` |
| Doctor check for runtime dependencies | No | no new file path, socket, port, module or binary |
| Prometheus counters/metrics | No | `ze_bgp_as_path_loop_detected_total` already covers the loop outcome, and the recipe adds no state worth a counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute is added; AS_PATH is existing |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- the AS-migration recipe is named under BGP |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- the recipe and the two refusals |
| 3 | CLI command added/changed? | No | no command changes |
| 4 | API/RPC added/changed? | No | the `filter-update` RPC is unchanged |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- bgp-filter-modify gains a documented import-direction behavior and bgp-rs a refusal |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` is the page; no new page is added |
| 7 | Wire format changed? | No | the AS_PATH encoding rules are unchanged; the width fix makes ze obey the existing ones |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7705 is not enrolled, `rfc/short/rfc7705.md` does not exist, and `plan/immediate/spec-bgp-as-migration.md` owns creating it. `RFC7705-3.3-9` stays an unimplemented SHOULD, because a recipe an operator must write is not the engine performing it |
| 10 | Test infrastructure changed? | No | existing `.ci` and interop harnesses |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- other daemons offer the inbound append as a per-neighbor knob and ze offers a policy recipe; the row states that plainly |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/egress-attribute-rules.md` -- the import direction of the prepend action and its width rule |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | No | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/spec-bgp-local-as-inbound-append.md`. `filter_modify.go` declares `docs/architecture/bgp/egress-attribute-rules.md` in its `// Design:` header, which is why row 12 is Yes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` and `docs/config-reference.md` show `local-as` and `as-path-prepend`; both examples are checked against the schema after the text changes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the recipe reaches the append, and that today it is reachable and wrong
   - Tests: `test/plugin/bgp-local-as-inbound-recipe.ci`, `TestImportPrependUsesPeerConfiguredLocalAS`
   - Files: the two new test files, the `.ci` and its fixture registration
   - Verify: the `.ci` shows the appended path against a four-octet peer, which is the case that already works, and the width test fails
2. **Phase: Width** -- make the prepend encode at the source's negotiated ASN width
   - Tests: `TestImportPrependEncodesAtTheSourceASNWidth`, `TestExportPrependEncodesAtTheDestinationASNWidth`
   - Files: `filter_delta.go`, `filter_ordered.go`
   - Verify: both widths decode as one AS_PATH, and the four-octet legacy ASN toward a two-octet peer reads AS_PATH and AS4_PATH together
3. **Phase: Refusals** -- stop the recipe reaching a place it is wrong
   - Tests: `TestRouteServerRefusesInboundASPathPrepend`, `TestNoPrependRefusesInboundASPathPrepend`, `test/plugin/bgp-local-as-inbound-route-server.ci`
   - Files: `rs/server.go`, `filter_modify/config.go`
   - Verify: each refusal names the peer and both settings, and a config without the combination still loads
4. **Phase: Documentation** -- write the recipe an operator follows
   - Tests: none; the check is that every configuration in the page is one the parser accepts
   - Files: `docs/guide/configuration.md`, `docs/features.md`, `docs/comparison.md`, `docs/guide/plugins.md`, `docs/architecture/bgp/egress-attribute-rules.md`, the two YANG texts, `docs/config-reference.md`
   - Verify: the page's configuration loads, and the emitted AS_PATHs match the `.ci` expectations
5. **Phase: Interop** -- prove a second implementation accepts the result
   - Tests: `bgp-local-as-inbound-append-frr`
   - Files: the scenario directory
   - Verify: the scenario goes RED with the modifier removed from ze's config and the artifact rebuilt, then GREEN with it restored

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation or a documented configuration at file:line |
| Feature completeness | All four user stories run end to end, including the one that removes the recipe |
| Correctness | The appended ASN is the peer's override read from settings, never a literal and never the router's global ASN |
| Correctness | The width of the emitted segment matches the width of the path it is spliced into, at both call sites |
| Naming | The refusal messages name the peer, the modify definition and the leaf, and quote the RFC section that forbids the combination |
| Data flow | Nothing in this spec reads or writes `secondaryPrependAS`; the outbound rail is untouched |
| Rule: `ai/rules/evidence.md` | Both refusals fail closed at config load, and each is driven from the config entry point in its test, not from the helper |
| Rule: `ai/rules/rfc-compliance.md` | The spec claims no conformance improvement: `RFC7705-3.3-9` stays an unimplemented SHOULD and no `rfc/short/` row moves |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The recipe is documented | `grep -n "as-path-prepend" docs/guide/configuration.md` shows it inside the local-as section |
| The width fix exists at both call sites | `grep -n "ExtractASPathPrependOps" internal/component/bgp/reactor/filter_ordered.go` shows a width argument on both lines |
| The two refusals exist | the two unit tests pass, and each fails when its check is removed |
| The recipe is reachable by an operator | `./le test plugin bgp-local-as-inbound-recipe` |
| The outbound rail is unchanged | `./le test plugin bgp-local-as-options` passes with no expectation edited |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The prepend count comes from config, bounded 1..32 by the schema and re-checked in `ExtractASPathPrependOps`; the ASN comes from settings, never from the wire |
| Resource exhaustion | A prepend grows the AS_PATH by up to 32 ASNs, inside the existing wire-length checks, and costs one heap buffer per UPDATE on the sessions that carry the modifier |
| Fail open | Both refusals reject the config rather than warning and continuing, so a route server cannot silently modify a client's path |

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
- Ze forwards the received wire, so the two moments RFC 7705 Section 3.3 item 1 names collapse into ONE place: the ingress rewrite. An append written into the RIB instead would be visible to `show bgp` and to best-path and to nothing on the wire, because no rail builds an UPDATE from the Loc-RIB.
- The peer chain running at stage 300, after loop detection at stage 0, is what makes an ingress append safe for ze itself. It also means ze cannot catch an operator who appends the router's own global ASN inbound.
- The `as-path-prepend` action already reads the ASN from the session rather than from the policy, which is why no new leaf is needed. The action's own `ze:help` says so, and the import call site proves it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Document a policy recipe; add no YANG leaf | A `session > asn > local-as-inbound-append` enum leaf with `at-rib-install` and `at-ibgp-advertisement` positions | The filter expresses the behavior with the leaves that exist, on a per-neighbor and per-neighbor-group chain, reading the ASN from the session. A leaf would be a second way to say what the policy already says, for a mechanism used during a rare migration |
| The append lands at ingress, on the received payload | At RIB install (rib plugin only) or at iBGP advertisement (egress rails only) | Under structural forwarding these are not equivalent: the RIB position reaches no wire, and the egress position leaves the local RIB disagreeing with every neighbor. The ingress position gives both at once, which is what "consistency in the AS_PATH throughout the AS" asks for |
| Refuse the recipe on a route-server client at config load | Warn in the documentation only; strip the modification on the RS rail at runtime | RFC 7947 Section 2.2.2.1 forbids the modification. A runtime strip would silently disagree with the operator's config, and documentation alone leaves ze doing the forbidden thing on request |
| Refuse the recipe beside `local-options no-prepend` | Let the modifier win; let the enum win | The two state opposite things about one session. Picking a winner makes one of the operator's two statements a lie that no output reports |
| No refusal for a peer with no local-as override | Refuse the modifier on any import chain | `as-path-prepend` is a general policy action, and refusing it inbound would remove a legitimate use. The hazard is documented instead, as R-1 |

## Known Limitations
- `RFC7705-3.3-9` stays an unimplemented SHOULD. The recipe is operator configuration, so ze does not perform the append by itself and no conformance row moves. Implementing it as an engine mechanism is the deferred work this spec's Task section records the owner's ruling on.
- `local-options no-prepend` still selects nothing, and after this spec it still cannot: with no engine append there is nothing for it to suppress. Its only new effect is the refusal it triggers beside the recipe.
- The RFC's "flexible model" (Section 3.3, the BGP Alias MAY) is out of scope and unchanged.
- Enrolment of RFC 7705 into `rfc/short/`, and every `RFC requirement:` tag over Section 3.3, belong to `plan/immediate/spec-bgp-as-migration.md`.

## RFC Documentation (Scope: protocol)

RFC 7705 asks for an append in four places, all in Section 3.3 and nowhere
else; everything before Section 3.3 is the worked migration example. Each line
below is quoted from `rfc/full/rfc7705.txt`.

| # | Item | Quoted requirement | Ze today |
|---|------|--------------------|----------|
| 1 | Internal (SHOULD, inbound) | "The router SHOULD append the configured 'Local AS' ASN in the AS_PATH attribute before installing the route or advertising the UPDATE to an iBGP neighbor. The decision of when to append the ASN is an implementation detail outside the scope of this document." | Not performed. Every AS_PATH write is gated on an eBGP destination |
| 2 | External (SHOULD, outbound) | "The BGP router SHOULD first append the globally configured ASN to the AS_PATH immediately followed by the 'Local AS' value before advertising the UPDATE to an eBGP neighbor." | Performed by `secondaryPrependAS` and the three egress rails |
| 3 | No Prepend Inbound (MUST NOT, MUST) | "it MUST NOT append the 'Local AS' ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors, but it MUST still append the globally configured ASN as normal when advertising the UPDATE to other local eBGP neighbors" | The MUST NOT holds vacuously, because item 1 was never built; the MUST is what the egress rails already do |
| 4 | Replace Old AS (MUST NOT, MUST) | "the BGP speaker MUST NOT append the globally configured ASN from the AS_PATH attribute. The BGP router MUST append only the configured 'Local AS' ASN value to the AS_PATH attribute before sending the BGP UPDATEs outbound to the eBGP neighbor." | Performed: `secondaryPrependAS` returns zero under `replace-as` |

Section 3.3 opens with the scoping MUST the recipe satisfies through the
per-peer and per-group import chains: "The mechanisms introduced in this
section MUST be configurable on a per-neighbor or per-neighbor-group basis to
allow for maximum flexibility."

The route-server constraint is RFC 7947 Section 2.2.2.1: a route server "SHOULD
NOT prepend its own AS number to the AS_PATH segment nor modify the AS_PATH
segment in any other way". The second clause is wider than the prepend it
names, which is why an inbound append on a route-server path is refused rather
than documented away.

What each documented configuration emits, for a router whose global ASN is
64500, a migrated peer carrying `local-as 64510` and remote AS 64496 that
originates a route with AS_PATH 64496, an iBGP neighbor, and a native eBGP
neighbor. The recipe column is the import-chain `as-path-prepend 1`.

| Configuration | Recipe | iBGP neighbor sees | Native eBGP neighbor sees | Migrated peer receives, for a route ze relays to it |
|---------------|--------|--------------------|---------------------------|------------------------------------------------------|
| `local-as` alone | absent | 64496 | 64500 64496 | 64510 64500 and the rest |
| `local-as` alone | attached | 64510 64496 | 64500 64510 64496 | 64510 64500 and the rest |
| `local-as` + `no-prepend` | absent | 64496 | 64500 64496 | 64510 64500 and the rest |
| `local-as` + `no-prepend` | attached | refused at config load (AC-4) | refused | refused |
| `local-as` + `replace-as` | absent | 64496 | 64500 64496 | 64510 and the rest |
| `local-as` + `replace-as` | attached | 64510 64496 | 64500 64510 64496 | 64510 and the rest |

The last column is the outbound rail landed in `5001022d13` and proven in
`b60737ac8`. No row of it changes in this spec, and AC-7 is what holds it.

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
- [ ] AC-1..AC-8 all demonstrated
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
