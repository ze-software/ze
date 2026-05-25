# Spec: pol-2-actions -- Remove Private AS Action Filter

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/10 (scoped verification blocked) |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-pol-0-umbrella.md` - policy umbrella and action vocabulary
4. `plan/learned/541-policy-framework.md` - policy framework decisions
5. `plan/learned/572-cmd-8-policy-show.md` - policy introspection decisions
6. `plan/learned/593-cmd-2-session-policy.md` - session policy knobs and AS path action context
7. `rfc/short/rfc6996.md` - Private Use ASN ranges and removal requirement
8. `rfc/short/rfc6793.md` - AS4_PATH and four-octet ASN handling
9. `internal/component/bgp/reactor/filter_chain.go` - policy chain execution
10. `internal/component/bgp/reactor/filter_delta.go` - text delta to wire modification path
11. `internal/component/bgp/reactor/filter_delta_handlers.go` - AS_PATH modification handler
12. `internal/component/bgp/reactor/reactor_api_forward.go` - export policy and EBGP prepend ordering
13. `internal/component/bgp/attribute/aspath.go` - AS_PATH segment model and parsing
14. `internal/component/bgp/attribute/as4.go` - AS4_PATH model and RFC 6793 constraints

## Task

Implement the `remove-private-as` route policy action filter.

This spec covers the remove-private-AS slice of the broader `spec-pol-2-actions` work from `spec-pol-0-umbrella.md`. The umbrella also names inc/dec arithmetic, community add/remove, and `as-path-length`; those are outside this spec and require separate design before implementation. This spec must not be marked complete for those other action macros.

Operators need a simple export-chain policy action that removes Private Use ASNs from AS path attributes without building a regex rewrite pipeline. The feature must preserve the existing policy-chain model: match filters and action filters are composed linearly, plugins communicate by text delta, and the reactor owns wire-safe attribute rewriting.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - filter pipeline, ModAccumulator, progressive build
  -> Decision: policy filters run through `PolicyFilterChain`, return accept/reject/modify, and wire changes are applied through `ModAccumulator` and attribute handlers.
  -> Constraint: reactor remains the owner of wire bytes; filter plugins send text decisions, not cross-package pointers.
- [ ] `docs/architecture/wire/attributes.md` - AS_PATH and AS4_PATH wire attributes
  -> Decision: AS_PATH segment types are part of the wire semantics and cannot be reconstructed from flat text.
  -> Constraint: AS_PATH uses 2-byte or 4-byte ASN width depending on encoding context; AS4_PATH always carries 4-byte ASNs.
- [ ] `docs/architecture/update-building.md` - receive, forward, and copy-on-modify paths
  -> Decision: received UPDATEs are cached as `WireUpdate`; modified outbound payloads are copy-on-modify.
  -> Constraint: forwarding must preserve zero-copy for the no-modification common case.
- [ ] `docs/architecture/config/syntax.md` - policy config syntax and YANG-driven editor behavior
  -> Decision: filter definitions live under `bgp/policy`; chains reference filter names under `bgp/filter`, group filter, or peer filter.
  -> Constraint: schema changes must be reflected through YANG, not ad hoc parser-only config.
- [ ] `docs/contributing/rfc-implementation-guide.md` - RFC implementation workflow
  -> Decision: RFC behavior requires summary, source audit, tests, and RFC comments above enforced MUST rules.
  -> Constraint: protocol behavior must be validated with unit and interop tests.

### Rules and Patterns

- [ ] `ai/patterns/plugin.md` - new plugin registration and generated all-imports
  -> Decision: new filter plugin uses `register.go`, `schema/`, and `make generate` to update `internal/component/plugin/all/all.go`.
  -> Constraint: plugin packages register with `registry.Register` and do not import sibling plugins.
- [ ] `ai/patterns/registration.md` - registry pattern
  -> Decision: `FilterTypes` maps `remove-private-as` to the owning plugin for chain canonicalization.
  -> Constraint: every new runtime feature must be reachable through registration, not direct imports.
- [ ] `ai/rules/plugin-design.md` - plugin isolation and DirectBridge/EventBus boundaries
  -> Decision: the filter plugin returns only text-level route policy output; the reactor owns raw UPDATE mutation.
  -> Constraint: values crossing plugin boundaries are serializable values, not shared pointers.
- [ ] `ai/rules/config-design.md` - YANG augment and config validation
  -> Decision: a BGP filter plugin augments `/bgp:bgp/bgp:policy` with a `ze:filter` list.
  -> Constraint: no silent ignore of unknown config keys; YANG must define the full syntax.
- [ ] `ai/rules/tdd.md` - TDD workflow
  -> Decision: tests must fail before implementation, then pass with minimal code.
  -> Constraint: every numeric range needs boundary tests.
- [ ] `ai/rules/testing.md` - unit and functional test placement
  -> Decision: pure AS_PATH rewrite logic belongs in Go unit tests; end-user config behavior belongs in `.ci` tests.
  -> Constraint: functional tests are mandatory for end-user behavior.

### RFC Summaries

- [ ] `rfc/short/rfc6996.md` - Private Use ASN ranges and removal requirement
  -> Constraint: Private Use ASNs are `64512-65534` and `4200000000-4294967294`, inclusive.
  -> Constraint: Private Use ASNs MUST be removed from AS path attributes, including AS4_PATH when four-octet AS space is used, before global Internet advertisement.
- [ ] `rfc/short/rfc6793.md` - AS4_PATH and ASN4 behavior
  -> Constraint: AS4_PATH is optional transitive and carries four-octet ASNs.
  -> Constraint: AS4_PATH must not include confederation segments when constructed for OLD speakers; existing AS4 code already documents this rule.
- [ ] `rfc/short/rfc4271.md` - AS_PATH base semantics
  -> Constraint: AS_PATH is well-known mandatory and contains AS_SEQUENCE and AS_SET segments.
  -> Constraint: EBGP advertisement prepends local AS to AS_PATH.

**Key insights:**
- `remove-private-as` is an action macro, not a new policy language feature.
- Text filter format intentionally flattens AS_PATH segments, so wire-safe removal must use raw AS_PATH attributes.
- The reactor already has an attribute modification pipeline; extend it instead of bypassing it.
- Export AS_PATH policy modifications must happen before EBGP local-AS prepend and must not be lost by the existing EBGP wire cache.
- RFC 6996 explicitly includes AS4_PATH, so AS_PATH-only stripping is incomplete.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/spec-pol-0-umbrella.md` - defines macros-over-language policy direction, lists `remove-private-as`, and identifies the AS_PATH wire-level gap.
  -> Constraint: `remove-private-as` should be a single-purpose filter plugin composed in the existing chain.
- [ ] `plan/learned/541-policy-framework.md` - original policy framework decisions.
  -> Constraint: specialized filters are preferred over a generic route policy language.
- [ ] `plan/learned/572-cmd-8-policy-show.md` - policy introspection decisions.
  -> Constraint: registered filter types are visible through policy introspection.
- [ ] `plan/learned/593-cmd-2-session-policy.md` - session policy knobs and attribute suppression context.
  -> Constraint: attribute actions use the existing modification accumulator model.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - runs policy filters and merges text deltas.
  -> Constraint: chain execution is linear, reject short-circuits, and default is accept.
- [ ] `internal/component/bgp/reactor/filter_format.go` - renders UPDATEs to filter text.
  -> Constraint: AS_PATH text is flat and explicitly does not preserve segment type.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - converts text changes to wire ops.
  -> Constraint: AS_PATH and NLRI are skipped by generic text delta conversion.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - registers generic and AS_PATH handlers.
  -> Constraint: AS_PATH handler currently supports Set and Prepend, but Prepend currently dominates Set when both are present.
- [ ] `internal/component/bgp/reactor/forward_build.go` - applies `ModAccumulator` ops to UPDATE payloads.
  -> Constraint: modifications are applied by per-attribute handlers in a single progressive build.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - export policy chain and EBGP prepend path.
  -> Constraint: export policy currently creates `exportWireOverride`, but EBGP prepend still uses the original cached update payload.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - import policy modification path.
  -> Constraint: import modifications replace the cached `WireUpdate` before downstream consumers see it.
- [ ] `internal/component/bgp/attribute/aspath.go` - AS_PATH segment representation, parse, write, and prepend.
  -> Constraint: segment type and segment length are first-class wire semantics.
- [ ] `internal/component/bgp/attribute/as4.go` - AS4_PATH parse/write behavior.
  -> Constraint: AS4_PATH is a separate optional transitive attribute and needs its own handler path.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - EBGP AS_PATH prepend rewrite.
  -> Constraint: EBGP prepend parses raw AS_PATH and reserializes based on destination ASN4 support.
- [ ] `internal/component/bgp/plugins/filter_modify/` - existing unconditional modify filter.
  -> Constraint: `as-path-prepend` already uses a pseudo text directive and a dedicated extractor.
- [ ] `internal/component/bgp/plugins/filter_aspath/` - existing AS-path regex match filter.
  -> Constraint: this filter is accept/reject only and uses flat AS_PATH text for matching.
- [ ] `internal/component/bgp/config/filter_registry.go` - named filter registry.
  -> Constraint: duplicate filter names across filter types are rejected.
- [ ] `internal/component/bgp/config/redistribution.go` - filter reference canonicalization.
  -> Constraint: plain, type-prefixed, and plugin-prefixed refs must resolve through `FilterTypesMap`.
- [ ] `internal/component/plugin/registry/registry_bgp_filter.go` - filter and modification contracts.
  -> Constraint: `ModAccumulator` supports Set, Add, Remove, Prepend, and Suppress actions.
- [ ] `pkg/plugin/rpc/types.go` - filter RPC input and output.
  -> Constraint: `FilterUpdateOutput.Raw` exists but is not consumed by the policy path today.
- [ ] ExaBGP reference tree - searched for remove-private/private-AS implementation.
  -> Constraint: no existing ExaBGP reference implementation was found in the checked tree.

**Behavior to preserve:**
- Policy chain remains a linear composition of named filters.
- Filter plugins continue to communicate by SDK filter-update RPC and text deltas.
- Unknown filters and invalid plugin responses continue to fail closed.
- Existing `as-path-prepend` behavior remains supported.
- AS_PATH segment structure is preserved for existing routes when rewriting.
- No wire copy occurs for routes that do not match a modification.
- EBGP local AS prepend remains RFC 4271 compliant.
- Route-server clients continue to obey RFC 7947 behavior that route servers do not modify AS_PATH for RS-client forwarding.

**Behavior to change:**
- Add a configured `remove-private-as` policy filter that can strip private ASNs from AS_PATH.
- Add `replace-with peer-as` mode that replaces private ASNs with the destination peer ASN.
- Apply remove-private-AS rewrites to AS4_PATH when AS4_PATH is present.
- Make export AS_PATH policy rewrites feed into the EBGP prepend source payload.
- Make AS_PATH Set and Prepend operations compose predictably in the attribute handler.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point

- Config enters through YANG under `bgp/policy/remove-private-as`.
- Policy chain references enter through `bgp/filter import` and `bgp/filter export`, plus group and peer filter containers.
- Runtime UPDATE data enters as `WireUpdate` payload bytes with an encoding context.
- Filter plugin runtime decisions enter as filter-update RPC responses.

### Transformation Path

1. YANG registers the `remove-private-as` list under `bgp/policy`.
2. Config resolution builds named filter entries and canonicalizes chain refs to `bgp-filter-remove-private-as:NAME`.
3. Plugin Stage 2 receives the BGP config section and stores named remove-private-AS definitions.
4. Reactor formats an UPDATE to filter text and invokes `PolicyFilterChain` with direction, peer name, peer AS, and update text.
5. The plugin extracts the flat AS_PATH text only to decide whether any visible Private Use ASN exists and to provide modified text for downstream text filters.
6. The plugin returns modify with a dedicated AS_PATH remove-private directive and, when useful, an updated flat `as-path` field for later filters in the same chain.
7. The reactor extracts the dedicated directive from final modified text and reads the raw AS_PATH and AS4_PATH attributes from the current `WireUpdate` payload.
8. The reactor rewrites raw AS_PATH and AS4_PATH segment values while preserving segment types, removing empty segments, and preserving unrelated ASNs.
9. The reactor emits `ModAccumulator` operations for AS_PATH and AS4_PATH.
10. `buildModifiedPayload` applies the operations through registered attribute handlers and produces a modified payload only when a rewrite is needed.
11. On import, the modified payload replaces the cached `WireUpdate` before cache and dispatch.
12. On export, the modified payload becomes the per-peer base wire before EBGP local-AS prepend, AS override, next-hop, and send-community modifications are applied.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | BGP config section delivered by plugin SDK `OnConfigure` | [ ] |
| Config -> Reactor | Filter refs stored in peer settings after canonicalization | [ ] |
| Reactor -> Plugin | `FilterUpdateInput` over filter-update RPC | [ ] |
| Plugin -> Reactor | `FilterUpdateOutput` action and text update | [ ] |
| Text -> Wire | Dedicated remove-private-AS extractor emits `ModAccumulator` ops | [ ] |
| WireUpdate -> Attribute handler | Raw AS_PATH and AS4_PATH parsed and rewritten | [ ] |
| Modified wire -> Forward path | Export override becomes peer base wire before EBGP prepend | [ ] |

### Integration Points

- `registry.Registration.FilterTypes` - maps `remove-private-as` to `bgp-filter-remove-private-as`.
- `BuildFilterRegistry` - includes `remove-private-as` list entries discovered via YANG.
- `canonicalizeFilterRefs` - resolves plain, type-prefixed, and plugin-prefixed filter refs.
- `PolicyFilterChain` - runs the filter in import/export chains and carries the text delta forward.
- `parseFilterAttrs` and `formatFilterAttrs` - preserve the dedicated directive during chain delta merging.
- New AS_PATH remove-private extractor - converts directive plus raw attributes into modification ops.
- `aspathHandler` - composes Set and Prepend operations.
- `genericAttrCodes` or a specialized handler - handles AS4_PATH Set and Suppress operations.
- `reactor_api_forward.go` - uses export policy modified wire as the source for EBGP prepend.
- `reactor_notify.go` - applies import modified payload before cache and event dispatch.

### Architectural Verification

- [ ] No bypassed layers: plugin returns policy intent, reactor rewrites wire bytes.
- [ ] No unintended coupling: filter plugin does not import reactor or attribute wire internals.
- [ ] No duplicated functionality: AS_PATH parsing uses existing `attribute` package helpers.
- [ ] Zero-copy preserved: no modified payload is allocated when no private ASN is present.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| D-1 | Dedicated filter plugin | Matches umbrella decision: macros, not a policy language. |
| D-2 | Reactor-owned wire rewrite | Text AS_PATH is flat and cannot preserve segment structure. |
| D-3 | Dedicated text directive | Avoids treating arbitrary `as-path` text changes as safe wire rewrites. |
| D-4 | Plugin also updates flat `as-path` text | Later text filters in the same chain should see the intended post-action path. |
| D-5 | Preserve segment types | AS_SET and confed segment semantics are wire-visible and must not be flattened. |
| D-6 | Apply before EBGP prepend | RFC 4271 local-AS prepend must happen to the final export base path. |
| D-7 | Strip and peer-AS replacement only | No vendor-specific modes beyond the umbrella's `replace-with peer-as`. |
| D-8 | AS4_PATH included | RFC 6996 explicitly includes AS4_PATH when four-octet AS number space is used. |

## Open Questions

| Question | Default for this spec | Needs User Decision Before Coding? |
|----------|-----------------------|------------------------------------|
| Should remove-private-as run on import as well as export? | Yes, if configured in either chain; typical use is export. | No |
| Should private ASNs in AS_SET be removed? | Yes, preserve the AS_SET segment and remove only matching members. | No |
| Should empty AS4_PATH be suppressed? | Yes, because it is optional transitive. | No |
| Should empty AS_PATH be emitted when every ASN is removed? | Yes, AS_PATH is well-known mandatory and may have an empty value. | No |
| Should RFC 6996 normal AS path filtering MAY add new behavior? | No, existing as-path-list handles filtering. | Yes, if a separate built-in filter is desired. |
| Should mixed public/private AS_PATH cause reject instead of strip? | No, RFC 6996 warns about implementations with that limitation; this feature strips private members. | No |

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `bgp/policy/remove-private-as NAME` config | -> | plugin config parser stores definition | `TestParseRemovePrivateASDefs` |
| `filter export remove-private-as:NAME` | -> | chain canonicalizes to plugin ref | `TestCanonicalizeRemovePrivateASRef` |
| export policy chain invokes plugin | -> | plugin returns remove-private-AS modify directive | `TestRemovePrivateASFilterUpdateWiring` |
| final modified text reaches reactor | -> | reactor emits AS_PATH modification op | `TestExtractRemovePrivateASOps` |
| modified export payload reaches EBGP prepend | -> | outgoing AS_PATH has private ASNs removed and local AS prepended | `TestExportRemovePrivateASBeforeEBGPPrepend` |
| functional config file | -> | end-user export chain strips private ASNs | `test/plugin/remove-private-as-export.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config defines `bgp/policy/remove-private-as NAME` | Filter definition parses and appears in the filter registry. |
| AC-2 | Chain references `remove-private-as:NAME` | Reference canonicalizes to `bgp-filter-remove-private-as:NAME`. |
| AC-3 | Chain references plain `NAME` and no duplicate filter name exists | Reference canonicalizes to the remove-private-AS plugin. |
| AC-4 | Unknown remove-private-AS filter name invoked at runtime | Plugin rejects fail-closed. |
| AC-5 | AS_PATH contains 64512 | Default mode removes 64512 from AS_PATH. |
| AC-6 | AS_PATH contains 65534 | Default mode removes 65534 from AS_PATH. |
| AC-7 | AS_PATH contains 64511 or 65535 | Default mode preserves the ASN. |
| AC-8 | AS_PATH contains 4200000000 | Default mode removes 4200000000 when visible in four-octet encoding. |
| AC-9 | AS_PATH contains 4294967294 | Default mode removes 4294967294 when visible in four-octet encoding. |
| AC-10 | AS_PATH contains 4199999999 or 4294967295 | Default mode preserves the ASN. |
| AC-11 | `replace-with peer-as` and peer AS is 65001 | Private ASNs are replaced with 65001. |
| AC-12 | AS_PATH has AS_SEQUENCE, AS_SET, and confed segments | Segment types are preserved and only private ASNs are removed or replaced. |
| AC-13 | A segment becomes empty after stripping | Empty segment is omitted from the rewritten attribute. |
| AC-14 | All ASNs are stripped from AS_PATH | AS_PATH remains present with an empty value. |
| AC-15 | AS4_PATH is present and contains private ASNs | AS4_PATH is rewritten as well as AS_PATH. |
| AC-16 | AS4_PATH becomes empty after stripping | AS4_PATH is suppressed from the UPDATE. |
| AC-17 | Export chain strips private ASNs for an EBGP non-RS peer | EBGP local AS prepend applies to the stripped path, not to the original path. |
| AC-18 | Route-server client export path is used | Existing RFC 7947 AS_PATH non-modification for RS-client forwarding remains preserved unless the explicit policy path is documented to run there. |
| AC-19 | No private ASNs are present | Plugin accepts without modification and no modified payload is allocated. |
| AC-20 | AS_PATH Set and Prepend ops coexist | Handler applies Set as the base path and Prepend in front of that base path. |
| AC-21 | Malformed AS_PATH or AS4_PATH is encountered during rewrite | Rewrite is skipped for that attribute and the route is not silently corrupted. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIsPrivateASN` | `internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go` | RFC 6996 ranges and boundaries | |
| `TestParseRemovePrivateASDefs` | `internal/component/bgp/plugins/filter_remove_private_as/config_test.go` | YANG config JSON parse, default mode, `replace-with peer-as` | |
| `TestRemovePrivateASFilterUpdate` | `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as_test.go` | Plugin accept/modify/reject decisions | |
| `TestRemovePrivateASFlatText` | `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as_test.go` | Updated flat AS_PATH text for downstream filters | |
| `TestExtractRemovePrivateASOps` | `internal/component/bgp/reactor/filter_delta_test.go` | Dedicated directive creates AS_PATH ops | |
| `TestRewriteASPathRemovePrivate` | `internal/component/bgp/reactor/filter_delta_test.go` | Segment-preserving strip and empty segment removal | |
| `TestRewriteASPathReplacePrivateWithPeerAS` | `internal/component/bgp/reactor/filter_delta_test.go` | `replace-with peer-as` wire behavior | |
| `TestRewriteAS4PathRemovePrivate` | `internal/component/bgp/reactor/filter_delta_test.go` | AS4_PATH strip and suppress behavior | |
| `TestAspathHandlerSetAndPrepend` | `internal/component/bgp/reactor/filter_delta_test.go` | Set plus Prepend composition | |
| `TestExportRemovePrivateASBeforeEBGPPrepend` | `internal/component/bgp/reactor/forward_export_filter_test.go` | Export rewrite feeds EBGP prepend source | |
| `TestCanonicalizeRemovePrivateASRef` | `internal/component/bgp/config/redistribution_test.go` | plain, type-prefixed, plugin-prefixed refs | |
| `TestFilterRegistryRemovePrivateAS` | `internal/component/bgp/config/filter_registry_test.go` | `ze:filter` list discovery | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| 16-bit private ASN range | 64512-65534 | 64512 and 65534 | 64511 | 65535 |
| 32-bit private ASN range | 4200000000-4294967294 | 4200000000 and 4294967294 | 4199999999 | 4294967295 |
| AS_PATH segment count | 0-255 | 255 | N/A | malformed segment with value overflow |
| `replace-with peer-as` | uint32 peer ASN | 4294967295 | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `remove-private-as-export` | `test/plugin/remove-private-as-export.ci` | Configured export chain removes 64512 before advertisement | |
| `remove-private-as-replace-peer` | `test/plugin/remove-private-as-replace-peer.ci` | Configured export chain replaces private ASN with peer ASN | |
| `remove-private-as-config-parse` | `test/parse/remove-private-as.ci` | Config with `bgp/policy/remove-private-as` parses successfully | |
| `remove-private-as-invalid-replace-with` | `test/parse/remove-private-as-invalid.ci` | Invalid `replace-with` value is rejected at parse time | |

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `36-remove-private-as-frr` | `test/interop/scenarios/36-remove-private-as-frr/` | FRR | FRR accepts UPDATE after private ASN stripping and EBGP prepend | Passing: `make ze-interop-test INTEROP_SCENARIO=36-remove-private-as-frr` on 2026-05-24 |
| `37-remove-private-as-as4path-frr` | `test/interop/scenarios/37-remove-private-as-as4path-frr/` | FRR | AS4_PATH private ASN stripping produces interoperable UPDATEs | Passing: `make ze-interop-test INTEROP_SCENARIO=37-remove-private-as-as4path-frr` on 2026-05-24 |

### Future (if deferring any tests)

- None. Any deferral requires explicit user approval and a destination spec.

## Files to Modify

- `internal/component/bgp/reactor/filter_chain.go` - include the remove-private-AS directive as a policy attr name if delta merging needs it.
- `internal/component/bgp/reactor/filter_delta.go` - add dedicated extraction from text directive to AS_PATH and AS4_PATH wire ops.
- `internal/component/bgp/reactor/filter_delta_handlers.go` - compose AS_PATH Set plus Prepend and add AS4_PATH handler support.
- `internal/component/bgp/reactor/reactor_api_forward.go` - ensure export modified wire is the base for EBGP prepend and per-peer policy modifications.
- `internal/component/bgp/reactor/reactor_notify.go` - apply remove-private-AS ops on import chain modifications.
- `internal/component/bgp/config/filter_registry.go` - no logic change expected, but add tests proving discovery of the new YANG list.
- `internal/component/bgp/config/redistribution.go` - no logic change expected, but add tests proving reference canonicalization.
- `internal/component/plugin/all/all.go` - generated blank imports for new plugin and schema.
- `docs/architecture/core-design.md` - update policy actions and export AS_PATH rewrite ordering if behavior changes materially.
- `docs/guide/plugins.md` - document the new built-in filter plugin.
- `docs/features.md` - mention private ASN removal as a policy feature.
- `rfc/short/rfc6996.md` - keep the new protocol summary with implementation commit.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Yes | `internal/component/bgp/plugins/filter_remove_private_as/schema/ze-filter-remove-private-as.yang` |
| CLI commands/flags | [ ] No | N/A |
| CLI grammar | [ ] No | N/A |
| Editor autocomplete | [ ] Yes | YANG-driven from new schema |
| Functional test for new behavior | [ ] Yes | `test/plugin/remove-private-as-export.ci` and parse tests |
| Doctor check for runtime dependencies | [ ] No | No new files, sockets, kernel modules, ports, external binaries, or certs |
| Generated plugin imports | [ ] Yes | `internal/component/plugin/all/all.go` via `make generate` |
| RFC comments | [ ] Yes | Code enforcing RFC 6996 MUST removal behavior |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` if policy syntax is documented there |
| 3 | CLI command added/changed? | [ ] No | N/A |
| 4 | API/RPC added/changed? | [ ] No | Existing filter-update RPC only |
| 5 | Plugin added/changed? | [ ] Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] No | N/A unless policy guide exists during implementation |
| 7 | Wire format changed? | [ ] No | Existing AS_PATH and AS4_PATH formats only; behavior uses existing format |
| 8 | Plugin SDK/protocol changed? | [ ] No | Existing filter-update RPC only |
| 9 | RFC behavior implemented? | [ ] Yes | `rfc/short/rfc6996.md` exists and must remain protocol-only |
| 10 | Test infrastructure changed? | [ ] No | N/A |
| 11 | Affects daemon comparison? | [ ] Yes | `docs/comparison.md` if comparison matrix lists policy features |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/core-design.md` if AS_PATH export ordering changes are non-trivial |

## Files to Create

- `internal/component/bgp/plugins/filter_remove_private_as/register.go` - plugin registration.
- `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as.go` - SDK entry point and filter-update handler.
- `internal/component/bgp/plugins/filter_remove_private_as/config.go` - config parsing.
- `internal/component/bgp/plugins/filter_remove_private_as/private_as.go` - RFC 6996 range helper and text rewrite helper.
- `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as_test.go` - plugin runtime tests.
- `internal/component/bgp/plugins/filter_remove_private_as/config_test.go` - config tests.
- `internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go` - RFC 6996 boundary tests.
- `internal/component/bgp/plugins/filter_remove_private_as/schema/embed.go` - embedded YANG.
- `internal/component/bgp/plugins/filter_remove_private_as/schema/register.go` - YANG registration.
- `internal/component/bgp/plugins/filter_remove_private_as/schema/ze-filter-remove-private-as.yang` - schema augment.
- `internal/component/bgp/reactor/forward_export_filter_test.go` - export ordering regression tests if no existing test file is a better fit.
- `test/plugin/remove-private-as-export.ci` - end-user export stripping test.
- `test/plugin/remove-private-as-replace-peer.ci` - end-user replace-with peer-AS test.
- `test/parse/remove-private-as.ci` - valid config parse test.
- `test/parse/remove-private-as-invalid.ci` - invalid config parse test.
- `test/interop/scenarios/remove-private-as-frr/` - FRR interop scenario.
- `test/interop/scenarios/remove-private-as-as4path-frr/` - FRR AS4_PATH interop scenario.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement TDD | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint`, `make ze-unit-test`, `make ze-functional-test`, interop tests |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Failure Routing |
| 9. Re-verify | Repeat full verification |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Repeat full verification |
| 14. Present summary | Executive Summary Report per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register plugin and schema, add failing wiring tests.
   - Tests: `TestParseRemovePrivateASDefs`, `TestCanonicalizeRemovePrivateASRef`, `TestRemovePrivateASFilterUpdateWiring`.
   - Files: new plugin package, schema files, generated all-imports.
   - Verify: config and chain reach the plugin, tests fail because runtime rewrite is not implemented yet.
2. **Phase: RFC 6996 helpers** - implement Private Use ASN detection and flat text rewrite.
   - Tests: `TestIsPrivateASN`, `TestRemovePrivateASFlatText`.
   - Files: plugin helper files.
   - Verify: boundary tests fail first, then pass.
3. **Phase: Reactor wire rewrite** - convert dedicated directive to AS_PATH and AS4_PATH modification ops.
   - Tests: `TestExtractRemovePrivateASOps`, `TestRewriteASPathRemovePrivate`, `TestRewriteASPathReplacePrivateWithPeerAS`, `TestRewriteAS4PathRemovePrivate`.
   - Files: `filter_delta.go`, possibly a new reactor helper file if `filter_delta.go` grows too large.
   - Verify: segment-preserving wire tests fail first, then pass.
4. **Phase: Attribute handler composition** - make AS_PATH Set and Prepend compose and register AS4_PATH handler support.
   - Tests: `TestAspathHandlerSetAndPrepend`, AS4_PATH handler tests.
   - Files: `filter_delta_handlers.go`.
   - Verify: Set plus Prepend test fails under current behavior, then passes.
5. **Phase: Export ordering** - make policy-modified export wire feed EBGP prepend source.
   - Tests: `TestExportRemovePrivateASBeforeEBGPPrepend`.
   - Files: `reactor_api_forward.go`, possible helper tests.
   - Verify: regression test proves private ASN is absent and local AS is prepended.
6. **Phase: Import path parity** - apply the same directive extraction on import modifications.
   - Tests: import unit test in `reactor_notify` or `filter_delta` test package.
   - Files: `reactor_notify.go`.
   - Verify: modified import payload becomes the cached representation.
7. **Phase: Functional and interop tests** - add end-user config and peer daemon validation.
   - Tests: functional `.ci` files and FRR interop scenarios.
   - Files: `test/plugin/`, `test/parse/`, `test/interop/scenarios/`.
   - Verify: targeted functional tests pass, then relevant interop tests pass.
8. **Phase: Documentation and generated files** - update docs and generated imports.
   - Tests: `make generate`, docs checks through normal verification.
   - Files: docs and `internal/component/plugin/all/all.go`.
   - Verify: generated diff contains only expected imports.
9. **Full verification** - run `make ze-verify` as the final gate.
10. **Complete spec** - fill audit tables, write learned summary, and close the spec only after all ACs are demonstrated.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation and direct test evidence. |
| Correctness | Private ASN ranges are inclusive and exact; public boundary ASNs are preserved. |
| Segment semantics | AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET are not flattened in wire output. |
| AS4_PATH | AS4_PATH is rewritten or suppressed per RFC 6996 coverage. |
| Export ordering | EBGP prepend uses the policy-modified base wire. |
| Handler composition | AS_PATH Set and Prepend operations produce the expected final path. |
| No bypass | Plugin does not mutate raw wire bytes or import reactor internals. |
| No silent corruption | Malformed AS_PATH or AS4_PATH is not rewritten into a guessed value. |
| Performance | No modified payload allocation when no private ASN is present. |
| Config UX | YANG names are kebab-case and chain references work in all supported forms. |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| New plugin package exists | `ls` on `internal/component/bgp/plugins/filter_remove_private_as/` |
| YANG schema registered | `grep` for `ze-filter-remove-private-as.yang` registration and generated import |
| Filter type registered | `grep` for `FilterTypes` containing `remove-private-as` |
| RFC helper boundaries tested | `go test -run TestIsPrivateASN` in plugin package |
| AS_PATH wire rewrite tested | `go test -run TestRewriteASPathRemovePrivate` in reactor package |
| AS4_PATH wire rewrite tested | `go test -run TestRewriteAS4PathRemovePrivate` in reactor package |
| Export ordering regression tested | `go test -run TestExportRemovePrivateASBeforeEBGPPrepend` |
| Functional export behavior tested | targeted `ze-test` plugin test for remove-private-as export |
| Interop behavior tested | FRR scenario output recorded |
| Docs updated | Diff shows expected updates in docs files |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Config mode only accepts absent/default strip or `peer-as`. |
| Resource bounds | AS_PATH parsing respects existing segment length and total ASN limits. |
| Malformed wire | Malformed AS_PATH and AS4_PATH do not panic and do not produce guessed rewrites. |
| Fail-closed behavior | Unknown filter and plugin errors reject by default. |
| Policy leakage | Private ASNs are absent from export functional and interop assertions. |
| Logging | Warnings do not leak excessive raw route data. |
| Concurrency | Plugin config storage uses the existing atomic pointer pattern. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it. |
| Test fails wrong reason | Fix the test setup or assertion. |
| Test fails behavior mismatch | Re-read Current Behavior and update design only with user approval. |
| Lint failure | Fix inline if local; if architectural, return to design. |
| Functional test fails | Verify config syntax and runtime chain wiring. |
| Interop fails | Capture UPDATE bytes, compare AS_PATH and AS4_PATH contents, fix wire rewrite. |
| Three fix attempts fail | Stop, report all three approaches, ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Text AS_PATH could be safely rewritten | Filter text flattens all AS_PATH segment types | `attribute/text_append.go` comments and source audit | Requires raw segment-preserving reactor rewrite |
| Export policy override automatically fed EBGP prepend | EBGP prepend helper currently uses original cached update payload | `reactor_api_forward.go` source audit | Requires export ordering regression fix |
| Raw filter output could solve this directly | `FilterUpdateOutput.Raw` is not consumed in policy path | `pkg/plugin/rpc/types.go` and reactor search | Use existing text plus dedicated reactor extractor instead |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Generic text-level AS_PATH Set | Loses AS_SET and confed segment structure and can conflict with EBGP prepend | Dedicated reactor wire rewrite |
| Plugin raw=true full payload rewrite | Policy path does not consume raw output and would move wire ownership into plugin | Reactor-owned rewrite using existing attribute handlers |
| New general policy language | Contradicts umbrella macros-over-language decision | Single-purpose `remove-private-as` filter |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| None yet | N/A | N/A | N/A |

## Design Insights

- AS_PATH policy actions need a wire-aware path even when the operator-facing config is a simple action macro.
- The existing `as-path-prepend` pseudo directive is the closest local precedent, but remove-private-AS needs raw attribute parsing because it changes existing members.
- Export policy, AS override, and EBGP prepend all modify AS_PATH, so tests must cover their ordering together rather than in isolation.
- RFC 6996 support is incomplete if AS4_PATH is ignored.

## RFC Documentation

Add RFC comments above enforcement code for:

| RFC | Requirement to Quote in Code | Expected Location |
|-----|------------------------------|-------------------|
| RFC 6996 Section 4 | Private Use ASNs MUST be removed from AS path attributes, including AS4_PATH, before global Internet advertisement | remove-private-AS rewrite entry point |
| RFC 6996 Section 5 | Private Use ASN ranges are 64512-65534 and 4200000000-4294967294 | private ASN range helper |
| RFC 4271 Section 5.1.2 or 9.1.2 | EBGP local AS prepend behavior | export ordering and AS_PATH handler tests if touched |
| RFC 6793 Section 3 | AS4_PATH carries four-octet ASNs and is optional transitive | AS4_PATH rewrite or handler code |

## Implementation Summary

### What Was Implemented

- Added the `bgp-filter-remove-private-as` plugin with `remove-private-as` filter-type registration and YANG schema under `bgp/policy/remove-private-as`.
- Implemented default strip mode and `replace-with peer-as` mode.
- Added wildcard filter declaration support in the plugin server so config-defined filter names can share one plugin declaration.
- Added reactor extraction for the `remove-private` directive and segment-preserving AS_PATH / AS4_PATH rewrite operations.
- Registered AS4_PATH attribute modification handling and fixed AS_PATH Set plus Prepend composition.
- Updated import and export policy paths to apply remove-private-AS directives; export policy-modified wire is now the base for EBGP local-AS prepend.
- Added unit, config, parse, import, export, replacement functional, and FRR interop tests. Targeted functional and interop commands pass; scoped verification is currently blocked by unrelated MCP task functional tests.

### Bugs Found/Fixed

- Export policy rewrites could be lost because EBGP prepend used the original cached wire. The export path now prepends from the per-peer policy-modified base wire when present.
- AS_PATH Set and Prepend previously did not compose: Prepend could ignore a Set base. The AS_PATH handler now applies Set first, then inserts Prepend in front.
- AS4_PATH had no generic modification handler, so AS4_PATH policy rewrite or suppression could not be emitted through the existing mod pipeline.

### Documentation Updates

- `docs/features.md` lists remove-private-AS as a policy plugin feature.
- `docs/guide/plugins.md` documents `bgp-filter-remove-private-as`, config syntax, strip mode, `replace-with peer-as`, AS4_PATH handling, and export ordering.
- `docs/architecture/core-design.md` documents AS_PATH Set plus Prepend ordering for policy actions.
- `rfc/short/rfc6996.md` records the protocol-only Private Use ASN ranges and removal requirement.

### Deviations from Plan

- `make ze-verify-changed` cannot complete in the current worktree. The earlier unrelated lint and temp rollback-pointer blockers were cleared; the current blocker is unrelated MCP task functional coverage (`ze-test bgp plugin 362` / `task-cancel` times out). This spec remains open until the gate passes.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Implement remove-private-as policy action | Implemented | `internal/component/bgp/plugins/filter_remove_private_as/` | Runtime loaded as `bgp-filter-remove-private-as`. |
| Preserve segment structure | Implemented | `internal/component/bgp/reactor/filter_delta.go` | Uses existing AS_PATH parser/writer; text AS_PATH is not authoritative. |
| Include AS4_PATH | Implemented | `internal/component/bgp/reactor/filter_delta.go`, `filter_delta_handlers.go` | Rewrites or suppresses optional AS4_PATH. |
| Add tests | Implemented, gate blocked | Unit, `.ci`, and interop files listed below | Targeted unit, functional, and interop tests pass; scoped verification is blocked outside this feature. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-4 | Implemented | `TestParseRemovePrivateASDefs`, `TestCanonicalizeRemovePrivateASRef`, `TestRemovePrivateASFilterUpdate` | Config parse, ref canonicalization, runtime reject/modify behavior. |
| AC-5..AC-10 | Implemented | `TestIsPrivateASN`, `TestRewriteASPathText`, `TestExtractRemovePrivateASOps` | Boundary ASNs covered for RFC 6996 ranges. |
| AC-11 | Implemented | `TestExtractRemovePrivateASOpsReplacePeerAS`, `test/plugin/remove-private-as-replace-peer.ci` | Functional test passed on 2026-05-24. |
| AC-12..AC-14 | Implemented | `TestExtractRemovePrivateASOps`, `TestExtractRemovePrivateASOpsEmptyASPath` | Preserves segments, removes empty segments, keeps mandatory AS_PATH present. |
| AC-15..AC-16 | Implemented | `TestRemovePrivateASFilterUpdateAS4Path`, `TestExtractRemovePrivateASOpsAS4Path` | AS4_PATH rewrite and empty suppression covered by unit tests. |
| AC-17 | Implemented | `TestExportRemovePrivateASBeforeEBGPPrepend`, `test/plugin/remove-private-as-export.ci`, `36-remove-private-as-frr` | Functional and interop tests passed on 2026-05-24. |
| AC-18 | Implemented | Existing RS fast-path tests plus docs update | Explicit policy chains are documented as applying on import/export; existing RS-client behavior remains covered by the RS fast-path suite. |
| AC-19 | Implemented | `TestRemovePrivateASFilterUpdate`, `TestExtractRemovePrivateASOpsNoPrivateASN` | No private ASN returns accept/no mod. |
| AC-20 | Implemented | `TestAspathHandler/set_and_prepend` | Set base plus Prepend composition covered. |
| AC-21 | Implemented | `TestExtractRemovePrivateASOpsMalformedASPath` | Malformed AS_PATH produces no rewrite op. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit tests | Passing targeted | Plugin, config, reactor packages | See Pre-Commit Verification for commands. |
| Parse tests | Passing targeted | `test/parse/remove-private-as*.ci` | `ze-test bgp parse remove-private-as` and `ze-test bgp parse remove-private-as-invalid` passed on 2026-05-24. |
| Import functional test | Passing targeted | `test/plugin/remove-private-as-import.ci` | `ze-test bgp plugin remove-private-as-import` passed on 2026-05-24. |
| Export functional tests | Passing targeted | `test/plugin/remove-private-as-export.ci`, `test/plugin/remove-private-as-replace-peer.ci` | Both targeted `ze-test bgp plugin` commands passed on 2026-05-24. |
| Interop tests | Passing targeted | `test/interop/scenarios/36-remove-private-as-frr/`, `test/interop/scenarios/37-remove-private-as-as4path-frr/` | Both `make ze-interop-test INTEROP_SCENARIO=...` commands passed with image builds on 2026-05-24. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Plugin and schema files | Added | `internal/component/bgp/plugins/filter_remove_private_as/` |
| Reactor policy/wire files | Updated | `filter_chain.go`, `filter_delta.go`, `filter_delta_handlers.go`, `reactor_notify.go`, `reactor_api_forward.go` |
| Config and plugin server tests | Added/updated | Config ref canonicalization and wildcard filter declaration support. |
| Functional tests | Added | Parse, import, export strip, and export replace-peer files. |
| Docs and RFC summary | Added/updated | Feature list, plugin guide, architecture note, RFC 6996 summary. |
| Interop scenarios | Passing targeted | `36-remove-private-as-frr` and `37-remove-private-as-as4path-frr` both pass against FRR. |

### Audit Summary

- **Total items:** 6 groups tracked here.
- **Done:** 6 groups: implementation, unit tests, docs, RFC summary, functional tests, interop scenarios.
- **Partial:** 0 groups.
- **Skipped:** 0.
- **Missing:** 0 groups.
- **Blocked gate:** `make ze-verify-changed` still cannot complete because unrelated MCP task functional coverage times out after lint and targeted unit checks pass.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Operators can configure remove-private-as in a policy chain | Unit and functional tests | `TestParseRemovePrivateASDefs`, `TestCanonicalizeRemovePrivateASRef`, `ze-test bgp parse remove-private-as`, `ze-test bgp plugin remove-private-as-import` passed on 2026-05-24. |
| Private ASNs are absent from AS_PATH after export | Unit, functional, and interop tests | `TestExportRemovePrivateASBeforeEBGPPrepend`, `ze-test bgp plugin remove-private-as-export`, and `make ze-interop-test INTEROP_SCENARIO=36-remove-private-as-frr` passed on 2026-05-24. |
| AS4_PATH is handled | Unit and interop tests | `TestRemovePrivateASFilterUpdateAS4Path`, `TestExtractRemovePrivateASOpsAS4Path`, and `make ze-interop-test INTEROP_SCENARIO=37-remove-private-as-as4path-frr` passed on 2026-05-24. |
| EBGP prepend still occurs after stripping | Unit, functional, and interop tests | `TestExportRemovePrivateASBeforeEBGPPrepend`, `ze-test bgp plugin remove-private-as-export`, and `36-remove-private-as-frr` all assert Ze local AS remains after private-AS stripping. |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | | | |

### Fixes applied

- Not started.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | | | |

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Pending implementation files | Pending | Pending |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-4..AC-16, AC-19, AC-21 | Unit behavior covered | `go test ./internal/component/bgp/... ./internal/component/plugin/server ./internal/component/config ./internal/component/config/yang ./internal/test/runner ./internal/component/web ./internal/component/cli -count=1` passed on 2026-05-25. |
| AC-2, AC-3 | Filter refs canonicalize | `go test ./internal/component/bgp/... ./internal/component/plugin/server ./internal/component/config ./internal/component/config/yang ./internal/test/runner ./internal/component/web ./internal/component/cli -count=1` passed on 2026-05-25. |
| AC-17, AC-20 | Export ordering and AS_PATH op composition | `go test ./internal/component/bgp/... ./internal/component/plugin/server ./internal/component/config ./internal/component/config/yang ./internal/test/runner ./internal/component/web ./internal/component/cli -count=1` passed on 2026-05-25. |
| Functional tests | Passing targeted | `ze-test bgp parse remove-private-as`, `ze-test bgp parse remove-private-as-invalid`, `ze-test bgp plugin remove-private-as-import`, `ze-test bgp plugin remove-private-as-export`, and `ze-test bgp plugin remove-private-as-replace-peer` passed on 2026-05-25. |
| Changed-package lint/unit | Passing | `make ze-lint-changed` and `make ze-unit-test-changed` passed on 2026-05-25. |
| Scoped verification | Blocked | `make ze-verify-changed` progressed past lint/unit after unrelated fixes, but full plugin functional testing is still blocked; targeted `ze-test bgp plugin 362` (`task-cancel`) times out on 2026-05-25. |
| Interop tests | Passing targeted | `python3 -m py_compile` passed for both interop checks; `make ze-interop-test INTEROP_SCENARIO=36-remove-private-as-frr` and `make ze-interop-test INTEROP_SCENARIO=37-remove-private-as-as4path-frr` passed on 2026-05-24. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| remove-private-as import chain | `test/plugin/remove-private-as-import.ci` | Passing targeted on 2026-05-24. |
| remove-private-as export strip chain | `test/plugin/remove-private-as-export.ci` | Passing targeted on 2026-05-24. |
| remove-private-as export replace-peer chain | `test/plugin/remove-private-as-replace-peer.ci` | Passing targeted on 2026-05-24. |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete and every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated under `internal/`
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass, defer only with user approval)

- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features beyond remove-private-as and `replace-with peer-as`
- [ ] Single responsibility per component
- [ ] Explicit behavior over implicit behavior
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL with expected reason
- [ ] Tests PASS after implementation
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING before ANY commit)

- [ ] Critical Review passes and is documented
- [ ] Partial or skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-pol-2-actions.md`
- [ ] Summary included in commit with code, tests, docs, schema, and generated files
