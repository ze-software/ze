# Spec: rpki-invalid-accepted-and-state-policy

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Implemented origin validation must obey explicit policy and provide the RFC-required state policy behavior. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| ASPA `action` container (`A-6`) | Convert ASPA (`rpki { aspa { action { invalid ...; unknown ...; } } }`) to the same named policy-object style in this spec alongside RPKI origin validation, avoiding a hybrid half-`action` / half-policy config under `rpki`. |

## Task

Ze excludes an RPKI-Invalid route from the Adj-RIB-In under a default nobody wrote.
`parseRPKIConfig` (`internal/component/bgp/plugins/rpki/rpki_config.go`) sets
`OriginInvalidAction: ASPAPolicyReject`, and `action/invalid` in
`internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` carries `default "reject"`.
An operator who configures a cache server and states nothing else loses routes.

RFC 6811 Section 2 states: "An implementation MUST NOT exclude a route from the
Adj-RIB-In or from consideration in the decision process as a side effect of its
validation state, unless explicitly configured to do so." A default is not that explicit
configuration. Ze is the outlier: GoBGP, FRR and BIRD each keep the route until the
operator writes the drop.

RFC 6811 Section 3 states: "An implementation MUST provide the ability to match and set
the validation state of routes as part of its route policy filtering function." Ze
provides the match half only, as the per-state per-peer action set `buildDecisions`
(`internal/component/bgp/plugins/rpki/rpki.go`) resolves. Nothing anywhere assigns a
validation state.

**Goal 1.** An RPKI-Invalid route is kept unless the operator states otherwise
(RFC6811-2-2).
**Goal 2.** Ze carries a route policy object that matches on the origin-validation
result and sets it, so the operator writes the drop (RFC6811-3-1).
**Goal 3.** An operator who wants today's behavior has one stated spelling, and ze tells
him what it is rather than losing routes in silence.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/rpki.md` - the operator-facing page for every surface this spec moves
  → Decision: the page documents `rpki / action / invalid` with `default reject` in its
    Config Reference table, the three-state Validation States table, the per-peer and
    per-group resolution order, and the RFC 6811 Section 4 re-validation. Each of those
    four blocks is edited by this work, in the same phase as the code.
  → Constraint: the page is SILENT on any policy object that matches a validation state.
    That silence is what authorized the source investigation behind this spec.
- [ ] `docs/guide/bgp-policy.md` - the route policy model this spec must join
  → Constraint: policy has exactly two parts, named filter instances under
    `bgp { policy { ... } }` and ordered references under `filter { import [...] export [...] }`.
    A new policy object that invents a third part is refused by this rule.
  → Decision: the page's own example writes `import [ no-self-as rpki:validate ]`, and no
    plugin registers a filter type named `validate`. The example is aspirational, so it
    must not be read as evidence that an rpki filter already exists.
- [ ] `ai/patterns/config-option.md` - YANG leaf structure for the new policy reference
  → Constraint: a leaf takes maximum native validation. The three states are an
    `enumeration`, never `type string`. A name that must resolve against another list
    takes `ze:validate` plus `ValidateFn` and `CompleteFn`, because ze's YANG uses
    `leafref` nowhere (0 occurrences under `internal/`).
- [ ] `ai/patterns/registration.md`, `ai/patterns/plugin.md` - the filter family pattern
  → Constraint: a filter type is declared by `FilterTypes` in the owning plugin's
    `register.go`, its instances augment `/bgp:bgp/bgp:policy` with a `list` marked
    `ze:filter` and keyed by `name`, and `registry.FilterTypesMap()` is the only
    enumeration. No central switch, factory or field is edited.
- [ ] `ai/rules/no-layering.md` - the surface this spec replaces
  → Constraint: the origin `action` container and the new policy object are two ways to
    write one decision. The container is DELETED, not kept beside the replacement.
- [ ] `ai/rules/interop-and-goal-validation.md` - the interop obligation
  → Constraint: route acceptance is wire-visible, so a scenario against a named peer
    daemon is owed, its directory is NAMED with no numeric prefix, and the red phase is
    forced by reverting the change and rebuilding before the green is claimed.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6811.md` - the requirement rows this spec closes
  → Constraint: RFC6811-2-2 and RFC6811-3-1 are both bound to one test today,
    `TestBuildDecisionsOriginInvalidAction`, in `rfc/requirements/rfc6811.md`. Each needs
    a positive and a negative tag after this work, and RFC6811-3-1 needs the set half it
    has never had.
  → Decision: the summary records that the validation state does not travel in BGP
    UPDATEs and that RFC 8097 standardizes the extended community which would carry it.
    Ze implements no part of RFC 8097 (no source hit under `internal/`), so a state this
    spec sets stays a local property, which is what Section 2 calls it.
- [ ] `rfc/full/rfc6811.txt` - the requirement text itself, read at Sections 2, 3 and 5
  → Constraint: Section 2 also says "The implementation should consider the validation
    state as described in the document as a local property or attribute of the Route",
    so a set must write the route's stored state and must not attempt a wire encoding.
  → Decision: Section 5 names the drop as an operator policy: "Policies that could be
    implemented include filtering routes based on validation state (for example,
    rejecting all 'invalid' routes)". The drop is not removed by this spec, it is moved
    from a default to a stated policy.
- [ ] `rfc/extraction/rfc6811.json` - the landed, signed extraction
  → Constraint: the file is landed and signed. No derived field is edited, no id is
    renumbered or removed. Site `3:1` records the set half as an open ask, and this spec
    answers it by implementing it.

**Key insights:** (minimal context to resume after compaction)
- The origin-validation disposition is applied by `buildDecisions` in the rpki plugin,
  asynchronously, after the reactor's ingress filter chain has already accepted the route.
- The reactor ingress chain (`runIngressPolicyChain`, `internal/component/bgp/reactor/filter_ordered.go`,
  called from `notifyMessageReceiver` in `internal/component/bgp/reactor/reactor_notify.go`)
  runs BEFORE plugin dispatch, so no chain filter can see a validation state, and no chain
  filter can be re-run when the VRP set changes.
- RFC 6811 Section 4 re-validation is therefore the constraint that fixes where the policy
  is evaluated: at the rpki decision point, which `applyToInstalled` already re-reaches.
- The state already survives an accept: `buildDecisions` writes `valState = req.state` on
  every accepted route, and `handleBatchValidateTyped`
  (`internal/component/bgp/plugins/adj_rib_in/rib_commands.go`) admits `ValidationInvalid`
  as a stored state. Nothing new is needed to keep an Invalid route marked Invalid.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` - `parseRPKIConfig` builds `rpkiConfig` with `OriginInvalidAction: ASPAPolicyReject` and `OriginNotFoundAction: ASPAPolicyAccept`, then overrides each from `rpki/action`. `resolveLeaf` merges peer over group over global per leaf into `PeerActions`, keyed by `configjson.PeerConfigKey`.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - `buildDecisions` resolves the effective per-peer action set, switches on `req.state`, sets `reject` only when the action is `ASPAPolicyReject`, applies the RFC 7999 blackhole exemption, then lets ASPA override an accept. It writes `ValState` on accepts only.
- [ ] `internal/component/bgp/plugins/rpki/validate.go` - `ROACache.Validate` produces `ValidationValid` (1), `ValidationNotFound` (2) or `ValidationInvalid` (3); `ValidationNotValidated` is 0. An unparseable prefix answers Invalid and warns.
- [ ] `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - `typedef validation-action` with `reject`, `log-only`, `accept`; the global `action` container defaults `invalid` to `reject` and `not-found` to `accept`; `grouping rpki-peer-policy` carries the same two leaves with no defaults at four augment points.
- [ ] `internal/component/bgp/plugins/rpki/register.go` - the `bgp-rpki` registration declares no `FilterTypes` and no `DoctorChecks`.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `RouteEntry.ValidationState` stores the RFC 6811 state per route.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - `promoteToInstalled` and `applyToInstalled` write that field; `applyToInstalled` is how a re-validation decision reaches a route already installed.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `handleBatchValidateTyped` refuses a decision whose state is not 1, 2 or 3, so an accepted Invalid route is already a legal decision.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `runIngressPolicyChain` is one ordered ingress step.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` walks the ingress steps and returns `false` on a reject, which is documented in place as "Route rejected by filter; don't cache or dispatch". Plugin dispatch is downstream of the chain.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `policyFilterFunc` builds `rpc.FilterUpdateInput`.
- [ ] `pkg/plugin/rpc/types.go` - `FilterUpdateInput` carries `Filter`, `Direction`, `Peer`, `PeerAS`, `Update` and `Raw`, and no validation state.
- [ ] `pkg/plugin/rpc/bridge.go` - `ValidationDecision` carries `Accept` and `ValState`.
- [ ] `internal/component/bgp/plugins/filter_aspath_length/register.go` - the family's registration shape: `FilterTypes`, `ConfigRoots`, `RunEngine`, `CLIHandler`.
- [ ] `internal/component/bgp/plugins/filter_aspath_length/yang/ze-filter-aspath-length.yang` - the family's YANG shape: `augment "/bgp:bgp/bgp:policy"`, `list <type>`, `ze:filter`, `key "name"`.
- [ ] `internal/component/bgp/plugins/filter_aspath_length/filter_aspath_length.go` - the family's runtime shape: `OnConfigure` parses the named definitions, `OnFilterUpdate` answers `FilterAccept` or `FilterReject`.
- [ ] `internal/component/config/retired.go` - `retiredKeywords` maps a keyword ze no longer accepts to its replacement, and `RetiredKeywordHint` is the sentence the parse error carries.
- [ ] `internal/component/cmd/show/show_policy.go` - `handleShowPolicyList` derives its answer from `registry.FilterTypesMap()`, so a registered filter type appears with no edit here.
- [ ] `internal/component/bgp/plugins/rpki/rpki_batch_test.go` - `TestBuildDecisionsOriginInvalidAction` is the sole test bound to RFC6811-2-2 and RFC6811-3-1, in both polarities.
- [ ] `test/plugin/rpki-validate-reject.ci` - states a cache server and no `action` block, so its expected reject comes from the default this spec removes.
- [ ] `test/interop/scenarios/rpki-frr/ze.conf` - states `action { invalid reject; not-found accept; }` explicitly.
- [ ] `rfc/requirements/rfc6811.md` - the requirement-to-test binding table.
- [ ] `docs/features/rfc-status.md` - the RFC 6811 row names "invalid-action reject/log-only/accept, default reject" as the current answer.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The three validation states, their numeric values, and `ROACache.Validate`'s answers,
  including Invalid for an unparseable prefix.
- Per-peer over per-group over global resolution, resolved per leaf, keyed by
  `configjson.PeerConfigKey`, including the listen-range group template case and the
  startup line `rpki: per-peer action override ignored: no static remote ip`.
- RFC 6811 Section 4 re-validation, including `applyToInstalled` reaching installed routes.
- The RFC 7999 `blackhole-exempt` leaf and its narrow exemption.
- The ASPA `aspa { action { invalid|unknown } }` container, its defaults, and
  `aspaOverridesAccept`. ASPA is a different state space under a different document.
- The fail-open `validation-timeout` guard.
- `ze_rpki_validation_outcomes_total` and `ze_rpki_aspa_outcomes_total`.

**Behavior to change:** (only what the user asked for)
- The default disposition for the Invalid state becomes accept.
- The origin `action` container (`action/invalid`, `action/not-found`) at all four augment
  points is deleted and replaced by a reference to a named policy object.
- A new `rpki-state` policy object type matches a validation state and sets one.
- `show bgp rpki status` reports the resolved policy in place of the `actions` and
  `peer-actions` records.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Origin disposition configuration | `rpki/rpki_config.go`, `parseRPKIConfig` | Resolves global, group and peer actions but invents invalid=reject when absent | Replaces implicit rejection and the origin action container with explicit policy; preserves resolution precedence |
| Origin validation and decision construction | `rpki/validate.go`, `ROACache.Validate`; `rpki/rpki.go`, `buildDecisions` | Calculates the origin state and converts the effective policy into acceptance and stored state | Extends the existing decision point for state policy rather than introducing an ingress evaluator without validation data |
| Revalidation of retained routes | `adj_rib_in/rib_validation.go`, `applyToInstalled` | Applies changed validation decisions to installed routes | Extends the same path so cache and policy changes preserve consistent stored state and eligibility |
| Filter registration | `filter_aspath_length/register.go`, `FilterTypes`; its YANG policy list | Registers named definitions through the existing plugin registry | Reuses registration and named policy syntax; does not add a central feature switch |
| RTR response processing | `rpki/rtr_session.go`, `RTRSession.handlePDU`, `applyEndOfData` | Parses response PDUs and publishes the completed cache state | Extends session identity checks before publication; preserves reset and serial query distinctions and existing expiry/report machinery |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Two entry points, and both must reach the same policy evaluation.
1. Operator config text: `bgp { policy { rpki-state <name> { ... } } }` and
   `bgp { rpki { policy <name> } }`, delivered to the plugin as the `bgp` config JSON
   subtree through `OnConfigure`.
2. A received BGP UPDATE: wire bytes on a peer session, dispatched to the rpki plugin as
   an `update-received` event after the reactor's ingress steps accept it.

### Transformation Path
1. Config text parses to the YANG-validated tree; `ResolveBGPTree` produces the `bgp` map.
2. `parseRPKIConfig` reads `bgp/policy/rpki-state/*` into named policy definitions and
   `bgp/rpki/policy` plus the per-peer and per-group `rpki/policy` leaves into a resolved
   policy NAME per peer key, replacing the two action enums it resolves today.
3. An UPDATE reaches `handleStructuredUpdate` or `handleEvent`, which derive the origin AS
   and call `ROACache.Validate` per NLRI, producing a `validationRequest` per prefix.
4. `buildDecisions` resolves the effective policy for the route's peer, evaluates the
   named policy against `req.state`, and produces `rpc.ValidationDecision` with `Accept`
   and the possibly-overridden `ValState`.
5. `batch-validate` carries the decisions to the adj-rib-in, which promotes, discards, or
   rewrites an installed route through `applyToInstalled`.
6. On a VRP change the origin tracker re-validates and re-enters step 4, so the same
   policy governs the re-decision.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ rpki plugin | `bgp` subtree JSON via `OnConfigure`, parsed by `parseRPKIConfig` | No |
| rpki plugin ↔ adj-rib-in | `rpc.ValidationDecision` over `batch-validate` (DirectBridge in-process, JSON out-of-process) | No |
| Plugin registry ↔ policy validation | `registry.FilterTypesMap()` names the type; `ValidateFilterNames` decides a chain reference | No |
| rpki plugin ↔ CLI | `show bgp rpki status` and `show bgp rpki policy` responses | No |

### Integration Points
- `parseRPKIConfig` and `resolveLeaf` (`rpki_config.go`) - the resolved per-peer value
  changes type from an action enum pair to a policy name.
- `buildDecisions` (`rpki.go`) - the switch on `req.state` becomes a policy evaluation.
- `statusCommand` (`rpki.go`) - the `actions` and `peer-actions` fields change.
- `registry.Registration` for `bgp-rpki` - gains `FilterTypes` and `DoctorChecks`.
- `retiredKeywords` (`internal/component/config/retired.go`) - gains the retired spelling.

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
| A-1 | The reactor ingress filter chain cannot see a validation state, so the policy cannot be a chain filter | `rpc.FilterUpdateInput` (`pkg/plugin/rpc/types.go`) carries no state field, and `notifyMessageReceiver` runs the chain before plugin dispatch | The design's central choice is wrong and the object belongs in `filter { import [...] }` | Read `policyFilterFunc` and the ingress step loop; a functional test that a chain reference is refused | unvalidated |
| A-2 | RFC 6811 Section 4 re-validation cannot re-run the ingress chain over a stored route | `applyToInstalled` (`adj_rib_in/rib_validation.go`) is reached from the batch-validate handler only, never from the reactor's ingress path | A chain filter could satisfy Section 4 after all, and A-1's consequence weakens | Trace every caller of `applyToInstalled` with `gopls references` | unvalidated |
| A-3 | A separate `filter_rpki_state` plugin cannot evaluate the policy, because the ROA cache and the decision point both live in `bgp-rpki` | `ROACache` is a field of `rPKIPlugin`; a plugin may run out-of-process (`RunEngine` over a socket) | The filter can be its own plugin directory and the family pattern is followed more literally | Confirm no cross-plugin accessor for the cache exists | unvalidated |
| A-4 | Accepting an Invalid route already works end to end, so goal 1 needs no new storage | `buildDecisions` writes `valState = req.state` on accepts, and `handleBatchValidateTyped` admits state 3 | The default flip would install routes with a lost state | `TestBuildDecisionsOriginInvalidAction` already asserts the Invalid marker survives an accept | unvalidated |
| A-5 | Deleting a YANG leaf makes an existing config fail to load with an unknown-field error, which `RetiredKeywordHint` can improve | `internal/component/config/retired.go` exists for exactly this and maps `process` to `attach process` | The migration message never reaches the operator and the flip is silent | A `.ci` test that loads the retired spelling and asserts the replacement sentence | unvalidated |
| A-6 | ASPA's action container is a different fact and stays | `aspa/action` is bound to draft-ietf-sidrops-aspa-verification, not RFC 6811, and `aspaOverridesAccept` runs after the origin decision | Leaving it is the hybrid `no-layering` bans, and it must be converted too | Owner ruling at the design gate | unvalidated |
| A-7 | Setting a validation state has no wire-visible effect, so no interop scenario can prove the set half | RFC 8097 has no implementation in ze (0 source hits under `internal/`), and RFC 6811 Section 2 calls the state a local property | The set half owes interop evidence this spec does not plan | Grep for an RFC 8097 encoder before claiming done | unvalidated |
| A-8 | Thomas's ruling that conformance outranks the current operational default (RFC 7454, MANRS) is settled and not to be revisited | Owner statement, 2026-08-30, in the task that commissioned this spec | The whole spec is wrong at its first line | Owner ruling recorded in Key Design Decisions | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator upgrading keeps his config unchanged and silently starts accepting Invalid routes | None by construction, which is the risk | The retired-keyword refusal makes the old spelling a LOAD error, not a silent no-op, so no config that stated the drop can start accepting without the operator editing it. The startup warning and the doctor check cover the config that never stated it |
| R-2 | The `.ci` tests that rely on the implicit default reject go red | `test/plugin/rpki-validate-reject.ci` states a cache server and no action block | Each such test states the policy explicitly. The test's subject is that the reject works, so stating the policy is a correction, never a weakening |
| R-3 | Changing the `show bgp rpki status` answer collides with the answer-shape work in flight | `plan/spec-plugin-declares-answer-shape.md` and `spec-cli-show-bgp-answer-shapes` both touch this command | Name the collision at the design gate and let the owner order the two. The field rename is one record, so a rebase is mechanical |
| R-4 | The per-peer resolution rewrite loses the listen-range group template case | `rpki-group-action.ci` and `rpki-per-peer-action.ci` cover it | Both tests are converted to the new spelling FIRST, before the resolution code changes, so the conversion is proven against the old behavior |
| R-5 | A policy that sets a state creates a loop with Section 4 re-validation: the set state is re-derived on the next VRP change and set again | A route whose reported state oscillates in the logs | The set writes the DECISION's state, never the cache's, so re-validation recomputes from the cache and re-applies the policy deterministically. A test drives two consecutive re-validations and requires the same answer |
| R-6 | A registered filter type invites `rpki-state:<name>` in an import chain, where it would silently never see a state | An operator config that parses and does nothing | `ValidateFilterNames` refuses the reference with a message naming `rpki { policy <name> }`. The guard fails closed, and a `.ci` test drives the refusal |
| R-7 | The package is too big for one session: the delete, the new type, the resolution rewrite, 36 rpki `.ci` files, two interop scenarios and the doc rewrite | The implementation session passing its budget before phase 4 | The phases are ordered so each is independently committable, and phase 1 plus phase 2 alone deliver goal 1. Report the size to the main thread rather than trimming an AC |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes an operator intended to drop are installed and used for forwarding. That is the failure RPKI exists to prevent, so a wrong landing is an outage-class security regression, not a cosmetic one |
| How is it reverted? | The code is a single-commit revert. The config is NOT: an operator who edited his config to the new spelling has to edit it back, so a revert after release is a config migration in both directions |
| Who else touches this path? | `plan/spec-plugin-declares-answer-shape.md` and `spec-cli-show-bgp-answer-shapes` declare `show bgp rpki` answer shapes. `plan/post-release/features/spec-bgp-filtered-route-storage.md` touches route storage. The `guard-added-to-one-half-of-a-pair.md` journal row records an open RFC6811-2-1 defect on the JSON event rail |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp { rpki { cache-server ... } }` with no policy stated, Invalid route received | → | `buildDecisions` (`rpki.go`) | `TestBuildDecisionsKeepsInvalidWithoutPolicy` |
| `bgp { policy { rpki-state drop-invalid { ... } } }` config text | → | `parseRPKIStatePolicies` (`state_policy.go`) | `TestParseRPKIStatePolicyDefinitions` |
| `bgp { rpki { policy drop-invalid } }` config text | → | `parseRPKIConfig` policy resolution | `TestParseRPKIConfigResolvesPolicyName` |
| `peer { rpki { policy <name> } }` config text | → | `resolveLeaf` over policy names | `TestPeerPolicyOverridesGroupThenGlobal` |
| Invalid route received under a stated drop policy | → | `evaluateStatePolicy` in `buildDecisions` | `test/plugin/rpki-policy-drop-invalid.ci` |
| Invalid route received under no policy | → | `buildDecisions` | `test/plugin/rpki-invalid-kept-by-default.ci` |
| `not-found` route under a policy stating `set-state valid` | → | `evaluateStatePolicy` state override | `test/plugin/rpki-policy-set-state.ci` |
| `filter { import [ rpki-state:x ] }` config text | → | `ValidateFilterNames` refusal | `test/parse/rpki-state-not-a-chain-filter.ci` |
| Retired `rpki { action { invalid reject } }` config text | → | `RetiredKeywordHint` | `test/parse/rpki-action-retired.ci` |
| `ze cli -c "show bgp rpki status"` | → | `statusCommand` | `test/ui/show-bgp-rpki-policy.ci` |
| `ze cli -c "show bgp rpki policy name drop-invalid"` | → | `policyCommand` | `test/plugin/rpki-policy-command.ci` |
| `ze doctor` with validation on and no policy | → | `bgp-rpki` `DoctorChecks` entry | `test/ui/doctor-rpki-no-origin-policy.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A cache server is configured, no policy is stated, and a route validates Invalid | The route is installed in the Adj-RIB-In and is marked with validation state `invalid` |
| AC-2 | The same condition, and a route validates NotFound or Valid | The route is installed and marked with its own state, unchanged from today |
| AC-3 | `bgp { policy { rpki-state <name> { invalid { disposition reject } } } }` is stated and referenced from `bgp { rpki { policy <name> } }`, and a route validates Invalid | The route is excluded from the Adj-RIB-In |
| AC-4 | The same policy, and a route validates Valid | The route is installed. A disposition is keyed on the state it names and reaches no other state |
| AC-5 | A policy states `disposition log-only` for a state | The route is installed, still marked with that state, and one warning names the prefix and the peer |
| AC-6 | A policy states `set-state <state>` for a state | The installed route carries the SET state, not the state the ROA lookup produced |
| AC-7 | A policy states both `disposition reject` and `set-state` for one state | The config is refused at validation time, because a rejected route has no state to carry |
| AC-8 | A policy is stated at the peer, the group and the global level | The peer's policy wins, then the group's, then the global one, matching the resolution the action leaves used |
| AC-9 | A policy name is referenced that no `rpki-state` object defines | The config is refused, and the message names the missing policy |
| AC-10 | `rpki-state:<name>` appears in `filter { import [...] }` or `filter { export [...] }` | The config is refused, and the message names `rpki { policy <name> }` as the correct place |
| AC-11 | A config states the retired `rpki { action { invalid reject } }` | The config is refused, and the message names the replacement spelling |
| AC-12 | Validation is active and no policy excludes Invalid routes | One startup line and one `ze doctor` finding say so, each naming the policy spelling that would drop them |
| AC-13 | The VRP set changes so an installed route becomes Invalid under a stated drop policy | The route is removed from the Adj-RIB-In by re-validation, and the same policy governs the re-decision |
| AC-14 | The VRP set changes twice with no config change | The disposition and the set state are identical after each re-validation |
| AC-15 | `show bgp rpki status` is run | The answer reports the resolved global policy name and the per-peer resolved policy names, with the source of each |
| AC-16 | `show bgp rpki policy name <name>` is run | The answer reports the named policy's per-state disposition and set-state clauses |
| AC-17 | `show policy` is run | `rpki-state` appears in `filter-types` with `bgp-rpki` as its plugin, discovered from the registry with no central list edited |
| AC-18 | A peer states `blackhole-exempt true` and a stated policy rejects Invalid, and a BLACKHOLE route is Invalid on prefix length alone | The route is kept, exactly as it is today |
| AC-19 | ASPA validation is enabled and a route is ROA-accepted but ASPA Invalid under `aspa { action { invalid reject } }` | The route is excluded, exactly as it is today |
| AC-20 | The editor completes the value of `rpki { policy <TAB> }` | The defined `rpki-state` object names are offered |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a cache server, states nothing else, and receives an Invalid route from a peer | wire -> reactor ingress steps -> rpki `update-received` -> `Validate` -> `buildDecisions` -> `batch-validate` -> adj-rib-in installed, marked invalid | `test/plugin/rpki-invalid-kept-by-default.ci` |
| 2 | Writes an `rpki-state` policy that rejects Invalid and points the rpki container at it | config text -> YANG -> `ResolveBGPTree` -> `parseRPKIConfig` -> `buildDecisions` -> adj-rib-in discard | `test/plugin/rpki-policy-drop-invalid.ci` |
| 3 | Writes a policy that sets NotFound routes to Valid for one trusted peer | config text -> per-peer policy resolution -> `buildDecisions` state override -> adj-rib-in `ValidationState` | `test/plugin/rpki-policy-set-state.ci` |
| 4 | Upgrades a daemon whose config states the retired action spelling | config load -> unknown field -> `RetiredKeywordHint` | `test/parse/rpki-action-retired.ci` |
| 5 | Runs `ze doctor` after upgrading and learns his Invalid routes are now kept | doctor run -> `bgp-rpki` doctor check -> diagnostic code | `test/ui/doctor-rpki-no-origin-policy.ci` |
| 6 | Asks which policy applies to a peer | `show bgp rpki status` -> `statusCommand` -> resolved policy record | `test/ui/show-bgp-rpki-policy.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildDecisionsKeepsInvalidWithoutPolicy` | `internal/component/bgp/plugins/rpki/rpki_batch_test.go` | AC-1, AC-2: with no policy, an Invalid route is accepted and keeps state 3, and Valid and NotFound are unchanged. RFC6811-2-2 negative | |
| `TestBuildDecisionsStatedPolicyRejectsInvalid` | `internal/component/bgp/plugins/rpki/rpki_batch_test.go` | AC-3, AC-4: a stated reject excludes Invalid and leaves Valid installed. RFC6811-2-2 positive, RFC6811-3-1 match positive | |
| `TestBuildDecisionsPolicySetsState` | `internal/component/bgp/plugins/rpki/rpki_batch_test.go` | AC-6: the decision carries the SET state. RFC6811-3-1 set positive | |
| `TestBuildDecisionsPolicySetIsStateSpecific` | `internal/component/bgp/plugins/rpki/rpki_batch_test.go` | AC-6: a set on one state leaves the other two unchanged. RFC6811-3-1 set negative | |
| `TestBuildDecisionsPolicyLogOnlyKeepsAndWarns` | `internal/component/bgp/plugins/rpki/rpki_batch_test.go` | AC-5 | |
| `TestBuildDecisionsRevalidationIsIdempotent` | `internal/component/bgp/plugins/rpki/rpki_batch_test.go` | AC-14, R-5: two consecutive evaluations of one request answer identically | |
| `TestParseRPKIStatePolicyDefinitions` | `internal/component/bgp/plugins/rpki/state_policy_test.go` | The named objects parse, including all three states and both clause kinds | |
| `TestParseRPKIStatePolicyRejectsSetOnReject` | `internal/component/bgp/plugins/rpki/state_policy_test.go` | AC-7 | |
| `TestParseRPKIConfigResolvesPolicyName` | `internal/component/bgp/plugins/rpki/rfc8210_rpki_config_test.go` | The global reference resolves to a definition | |
| `TestParseRPKIConfigRefusesUnknownPolicyName` | `internal/component/bgp/plugins/rpki/rfc8210_rpki_config_test.go` | AC-9 | |
| `TestPeerPolicyOverridesGroupThenGlobal` | `internal/component/bgp/plugins/rpki/rfc8210_rpki_config_test.go` | AC-8, including the listen-range group template | |
| `TestBlackholeExemptSurvivesStatedPolicy` | `internal/component/bgp/plugins/rpki/blackhole_decision_test.go` | AC-18 | |
| `TestASPAOverrideSurvivesStatedPolicy` | `internal/component/bgp/plugins/rpki/rpki_action_test.go` | AC-19 | |
| `TestStatusCommandReportsResolvedPolicy` | `internal/component/bgp/plugins/rpki/rpki_status_test.go` | AC-15 | |
| `TestPolicyCommandReportsClauses` | `internal/component/bgp/plugins/rpki/rpki_commands_test.go` | AC-16 | |
| `TestRPKIStateFilterTypeRegistered` | `internal/component/bgp/plugins/rpki/rpki_test.go` | AC-17: the registry names `rpki-state` against `bgp-rpki` | |
| `TestRetiredRPKIActionHint` | `internal/component/config/retired_test.go` | AC-11: the sentence names the replacement | |
| `TestValidateFilterNamesRefusesRPKIState` | `internal/component/bgp/config/filter_registry_test.go` | AC-10, R-6 | |
| `TestDoctorRPKINoOriginPolicy` | `internal/component/bgp/plugins/rpki/rpki_test.go` | AC-12: the check reports when validation is on and nothing rejects Invalid, and stays quiet otherwise | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `rpki-state` policy name length | 1..64 characters, `naming.ValidateNodeName` | 64 characters | empty name | 65 characters |
| Validation state in a `set-state` clause | enumeration of `valid`, `invalid`, `not-found` | `not-found` | N/A, the type is an enumeration and admits no number | N/A |
| `rpki-state` objects per config | 0..no stated cap | any count the tree holds | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-invalid-kept-by-default` | `test/plugin/rpki-invalid-kept-by-default.ci` | Cache server, no policy, Invalid route stays and reads `invalid` | |
| `rpki-policy-drop-invalid` | `test/plugin/rpki-policy-drop-invalid.ci` | A stated policy drops the Invalid route | |
| `rpki-policy-set-state` | `test/plugin/rpki-policy-set-state.ci` | A stated `set-state` changes the stored state | |
| `rpki-policy-log-only` | `test/plugin/rpki-policy-log-only.ci` | A stated `log-only` keeps the route and logs it | |
| `rpki-policy-per-peer` | `test/plugin/rpki-policy-per-peer.ci` | Rewrite of `rpki-per-peer-action.ci` for the policy spelling | |
| `rpki-policy-group` | `test/plugin/rpki-policy-group.ci` | Rewrite of `rpki-group-action.ci` for the policy spelling | |
| `rpki-validate-reject` | `test/plugin/rpki-validate-reject.ci` | Existing test, now stating its policy rather than relying on the default | |
| `rpki-revalidate-late-sync` | `test/plugin/rpki-revalidate-late-sync.ci` | Existing test, converted, proving AC-13 | |
| `rpki-policy-command` | `test/plugin/rpki-policy-command.ci` | `show bgp rpki policy name <name>` answers the clauses | |
| `rpki-action-retired` | `test/parse/rpki-action-retired.ci` | The retired spelling is refused with its replacement | |
| `rpki-state-not-a-chain-filter` | `test/parse/rpki-state-not-a-chain-filter.ci` | A chain reference is refused with the right place named | |
| `coverage-rpki` | `test/parse/coverage-rpki.ci` | Existing schema-coverage test, extended to the new nodes | |
| `show-bgp-rpki-policy` | `test/ui/show-bgp-rpki-policy.ci` | The status answer reports the resolved policy | |
| `doctor-rpki-no-origin-policy` | `test/ui/doctor-rpki-no-origin-policy.ci` | The doctor finding fires and explains itself | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rpki-invalid-kept-gobgp` | `test/interop/scenarios/bgp-rpki-invalid-kept-gobgp/` | GoBGP | RFC6811-2-2: ze with a cache server and no stated policy keeps the Invalid route GoBGP announces, and answers `invalid` for it, matching GoBGP's own default | |
| `rpki-frr` | `test/interop/scenarios/rpki-frr/` | FRR | RFC6811-3-1 match: ze under a stated `rpki-state` policy drops the Invalid route, and FRR under `match rpki invalid` plus a deny route-map drops the same one. Existing scenario, config rewritten to the policy spelling | |

The set half (AC-6) carries no interop row, and the reason is stated rather than assumed:
RFC 6811 Section 2 calls the validation state a local property of the route, and ze
implements no part of RFC 8097, so a set state reaches no wire byte and no peer daemon
can observe it. A-7 records the grep that must confirm this before the claim is made.

## Files to Modify
- `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - delete the origin `action` container at all four augment points, add `leaf policy` beside the per-peer leaves, add the `rpki-state` list augmenting `/bgp:bgp/bgp:policy`
- `internal/component/bgp/plugins/rpki/rpki_config.go` - parse the named objects, resolve a policy name per peer key, delete the two origin action fields
- `internal/component/bgp/plugins/rpki/rpki.go` - evaluate the policy in `buildDecisions`, report it in `statusCommand`, register the `show bgp rpki policy` command
- `internal/component/bgp/plugins/rpki/rpki_status.go` - the action-to-string helper becomes a policy record writer
- `internal/component/bgp/plugins/rpki/register.go` - add `FilterTypes` and `DoctorChecks`
- `internal/component/config/retired.go` - the retired origin action spelling
- `internal/component/bgp/config/filter_registry.go` - refuse an `rpki-state` reference in an import or export chain
- `internal/core/diagnostic/codes.go` - the diagnostic code for the new doctor check
- `test/interop/scenarios/rpki-frr/ze.conf` - the policy spelling
- `test/plugin/rpki-validate-reject.ci`, `test/plugin/rpki-revalidate-late-sync.ci`, `test/plugin/rpki-per-peer-action.ci`, `test/plugin/rpki-group-action.ci`, `test/parse/coverage-rpki.ci` - convert to the policy spelling
- `internal/component/bgp/plugins/rpki/rpki_batch_test.go` - replace `TestBuildDecisionsOriginInvalidAction` with the tagged tests above
- `rfc/requirements/rfc6811.md` - rebind RFC6811-2-2 and RFC6811-3-1 to the new tests, both polarities
- `rfc/short/rfc6811.md` - the two requirement rows and the implementation notes
- `docs/features/rfc-status.md` - the RFC 6811 row, which names the reject default today
- `docs/guide/rpki.md` - the Config Reference table, the Validation States default column, the per-peer section, the status example, and a migration section
- `docs/guide/bgp-policy.md` - the RPKI paragraph, and the aspirational `rpki:validate` example
- `docs/guide/configuration.md`, `docs/config-reference.md` - the rpki config surface
- `docs/features.md`, `docs/guide/plugins.md`, `docs/plugin-overview.md` - the new filter type
- `docs/comparison.md` - the row comparing ze's RPKI default with GoBGP, FRR and BIRD

## Files to Create
- `internal/component/bgp/plugins/rpki/state_policy.go` - the named policy definition, its parse, and its evaluation
- `internal/component/bgp/plugins/rpki/state_policy_test.go` - the unit tests above
- `internal/component/bgp/plugins/rpki/doctor.go` - the no-origin-policy check
- `test/plugin/rpki-invalid-kept-by-default.ci`, `test/plugin/rpki-policy-drop-invalid.ci`, `test/plugin/rpki-policy-set-state.ci`, `test/plugin/rpki-policy-log-only.ci`, `test/plugin/rpki-policy-per-peer.ci`, `test/plugin/rpki-policy-group.ci`, `test/plugin/rpki-policy-command.ci`
- `test/parse/rpki-action-retired.ci`, `test/parse/rpki-state-not-a-chain-filter.ci`
- `test/ui/show-bgp-rpki-policy.ci`, `test/ui/doctor-rpki-no-origin-policy.ci`
- `test/interop/scenarios/bgp-rpki-invalid-kept-gobgp/` - `ze.conf`, `gobgp.conf`, `rpki-server`, and the observer script

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang`: the `rpki-state` list augmenting `/bgp:bgp/bgp:policy`, and `leaf policy` at the four rpki augment points |
| YANG validation constraints | Yes | The disposition and the set-state values are `enumeration`, reusing the existing `validation-action` typedef for the disposition and a new `validation-state` typedef for the set. The policy name reuses the naming pattern the other filter lists use |
| YANG custom validators | Yes | `ze:validate` plus `ValidateFn` on `rpki/policy` to refuse an undefined name (AC-9) and on the `rpki-state` list to refuse a disposition-reject beside a set-state (AC-7). Ze uses `leafref` nowhere, so the reference cannot be native |
| CLI commands/flags | Yes | `show bgp rpki policy` and `show bgp rpki policy name <name>` registered in `internal/component/bgp/plugins/rpki/rpki.go` beside the existing `show bgp rpki` commands |
| CLI grammar (keyword before value) | Yes | The name is taken behind the `name` keyword. `plan/journal/command-takes-an-untyped-positional-value.md` records `show bgp rpki roa <prefix>` as the defect this avoids |
| Editor autocomplete | Yes | `CompleteFn` on `rpki/policy` offers the defined `rpki-state` names (AC-20). The enumerations complete natively |
| Functional test for new RPC/API | Yes | `test/plugin/rpki-policy-command.ci` and the twelve `.ci` files listed above |
| Pipe completeness | Yes | Both new commands answer structured data through the plugin response path the existing rpki commands use, so `\| json`, `\| yaml` and `\| table` each render it |
| Env var registration | N-A | No leaf under `environment/`. Every setting here is BGP config, which `ai/rules/config.md` puts in YANG |
| Doctor check for runtime dependencies | Yes | `bgp-rpki` gains a `DoctorChecks` entry reporting that validation is active and no policy excludes Invalid routes, with its diagnostic code in `internal/core/diagnostic/codes.go` beside `doctor-rpki-unreachable`, plus `TestDoctorRPKINoOriginPolicy` and `test/ui/doctor-rpki-no-origin-policy.ci` |
| Prometheus counters/metrics | Yes | `ze_rpki_policy_dispositions_total`, labels `policy`, `state`, `disposition`. `ze_rpki_validation_outcomes_total` keeps its meaning, which is the lookup result rather than the decision, so the two answer different questions |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new address family, NLRI type, capability or path attribute. The validation state stays a local property (RFC 6811 Section 2) and reaches no wire byte |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: the `rpki-state` policy object |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`: `show bgp rpki policy` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: the new command and the changed `show bgp rpki status` answer |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md`, `docs/features/plugins.md`: `bgp-rpki` now owns a filter type and a doctor check |
| 6 | Has a user guide page? | Yes | `docs/guide/rpki.md` and `docs/guide/bgp-policy.md` |
| 7 | Wire format changed? | N-A | No wire byte changes. The state is a local property and RFC 8097 is unimplemented |
| 8 | Plugin SDK/protocol changed? | No | `rpc.ValidationDecision` keeps its fields. Its `ValState` comment names only states 1 and 2 while the code already writes 3, so the comment is corrected in place |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc6811.md`, `rfc/requirements/rfc6811.md`, `docs/features/rfc-status.md`, each with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md` name the new scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: ze's default now matches GoBGP, FRR and BIRD |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md`, which `validate.go` names in its `// Design:` header |
| 13 | Route metadata keys added/changed? | No | The stored `ValidationState` field keeps its name and its values. Only the value a policy can write is new |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md`: `ze_rpki_policy_dispositions_total` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, not from memory: run `./le spec citation anchors spec plan/release/fixes/spec-rpki-invalid-accepted-and-state-policy.md` in phase 1 and name every blocking page it lists. `validate.go` declares `docs/architecture/plugin/rib-storage-design.md` in its `// Design:` header, and `docs/guide/rpki.md` carries seven `<!-- source: -->` anchors into the files this spec edits. `docs/features/ai-first.md` is declared by `internal/core/diagnostic/codes.go`, which this spec edits: it is UNAFFECTED, because the page describes the agent-facing diagnostic system rather than any individual code, and this spec adds one code to that system without changing how the system works |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every `action { ... }` example in `docs/guide/rpki.md` becomes a policy example. `docs/guide/bgp-policy.md` writes `import [ no-self-as rpki:validate ]`, which names a filter no plugin registers, and it is corrected in the same pass |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the filter type and the command, write the failing wiring tests
   - Tests: `TestRPKIStateFilterTypeRegistered`, `TestPolicyCommandReportsClauses`, and the four config-parse wiring tests
   - Files: `internal/component/bgp/plugins/rpki/register.go`, `internal/component/bgp/plugins/rpki/rpki.go`, `internal/component/bgp/plugins/rpki/state_policy.go` as a stub
   - Verify: `show policy` names `rpki-state` against `bgp-rpki`, `show bgp rpki policy` resolves, and every other wiring test fails because the feature is a stub. Run `./le spec citation anchors` here and record its blocking pages
2. **Phase: Default flip** -- goal 1 alone, independently committable
   - Tests: `TestBuildDecisionsKeepsInvalidWithoutPolicy`, `test/plugin/rpki-invalid-kept-by-default.ci`, and the conversion of `test/plugin/rpki-validate-reject.ci` to state its reject
   - Files: `rpki_config.go` default, `ze-rpki.yang` default, `docs/guide/rpki.md`, `docs/features/rfc-status.md`, `rfc/short/rfc6811.md`
   - Verify: the new test fails before the flip and passes after; the converted test passes both before and after, which is what proves the conversion is not a weakening
3. **Phase: The policy object** -- YANG, parse, validators, evaluation
   - Tests: `TestParseRPKIStatePolicyDefinitions`, `TestParseRPKIStatePolicyRejectsSetOnReject`, `TestParseRPKIConfigResolvesPolicyName`, `TestParseRPKIConfigRefusesUnknownPolicyName`, `TestBuildDecisionsStatedPolicyRejectsInvalid`, `TestBuildDecisionsPolicySetsState`, `TestBuildDecisionsPolicySetIsStateSpecific`, `TestBuildDecisionsPolicyLogOnlyKeepsAndWarns`
   - Files: `ze-rpki.yang`, `state_policy.go`, `rpki_config.go`, `rpki.go`
   - Verify: each test fails, then passes. AC-3 through AC-7 hold
4. **Phase: Resolution and the delete** -- peer over group over global on policy names, and the removal of the action container
   - Tests: `TestPeerPolicyOverridesGroupThenGlobal`, `TestBlackholeExemptSurvivesStatedPolicy`, `TestASPAOverrideSurvivesStatedPolicy`, `TestRetiredRPKIActionHint`, `TestValidateFilterNamesRefusesRPKIState`, `test/parse/rpki-action-retired.ci`, `test/parse/rpki-state-not-a-chain-filter.ci`
   - Files: `rpki_config.go`, `ze-rpki.yang`, `internal/component/config/retired.go`, `internal/component/bgp/config/filter_registry.go`, the four `.ci` conversions, `test/interop/scenarios/rpki-frr/ze.conf`
   - Verify: the old spelling is refused with its replacement, and every preserved behavior in Current Behavior still holds
5. **Phase: Re-validation** -- prove the policy governs the Section 4 path
   - Tests: `TestBuildDecisionsRevalidationIsIdempotent`, `test/plugin/rpki-revalidate-late-sync.ci` converted
   - Files: none expected. A file changed here means the design's evaluation point was wrong, which routes back to DESIGN
   - Verify: AC-13 and AC-14 hold, and R-5 is closed by the idempotence test
6. **Phase: Operator surfaces** -- status, the policy command, the startup line, the doctor check, the metric
   - Tests: `TestStatusCommandReportsResolvedPolicy`, `TestDoctorRPKINoOriginPolicy`, `test/ui/show-bgp-rpki-policy.ci`, `test/ui/doctor-rpki-no-origin-policy.ci`, `test/plugin/rpki-policy-command.ci`
   - Files: `rpki.go`, `rpki_status.go`, `doctor.go`, `register.go`, `internal/core/diagnostic/codes.go`
   - Verify: AC-12, AC-15, AC-16, AC-20 hold
7. **Phase: Interop** -- the GoBGP scenario and the FRR rewrite
   - Tests: `bgp-rpki-invalid-kept-gobgp`, `rpki-frr`
   - Files: the new scenario directory, `test/interop/scenarios/rpki-frr/ze.conf`
   - Verify: force the red phase for each. Revert the default flip, rebuild the ze image the scenario drives, confirm `bgp-rpki-invalid-kept-gobgp` goes RED, restore, confirm GREEN, and record the RED output. Do the same for `rpki-frr` against the policy evaluation
8. **Phase: Documentation** -- every row of the Documentation checklist not already written in its own phase
   - Tests: `./le verify docs`, and the schema-coverage test `test/parse/coverage-rpki.ci`
   - Files: the `docs/` list under Files to Modify
   - Verify: no page still describes the action container, and no example writes a spelling the parser refuses

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-6 in particular has a producer that WRITES the state rather than a caller that reads one |
| Feature completeness | Story 3 is the one most likely to be half-built: the set must survive the batch-validate boundary and reach `RouteEntry.ValidationState`, not stop at the decision struct |
| Correctness | The policy is evaluated on the re-validation path as well as the first path. A grep that finds the evaluation called from exactly one place is a defect, not a simplification |
| Correctness | The disposition is keyed on the state it names. A policy stating only `invalid` must leave Valid and NotFound untouched, including their stored states |
| Naming | JSON keys kebab-case: `policy`, `set-state`, `peer-policies`. The YANG enum values are `valid`, `invalid`, `not-found`, matching RFC 6811 Section 2 and the strings `validationStateString` already emits |
| Data flow | The reactor stays unaware of validation state. No field is added to `rpc.FilterUpdateInput`, and nothing under `internal/component/bgp/reactor/` learns the word rpki |
| Rule: `ai/rules/no-layering.md` | The origin action container is GONE from the YANG, from `rpkiConfig`, from `peerActionSet` and from every `.ci` and doc. A grep for `action {` under an rpki container returns only ASPA hits |
| Rule: `ai/rules/rfc-compliance.md` | Every enforcing branch carries its `// RFC 6811 Section N: "quote"` comment, and both requirement ids are bound to a positive and a negative test in `rfc/requirements/rfc6811.md` |
| Rule: `ai/rules/principles.md` | The drop is declared in exactly one place. An operator must have no second spelling that produces it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Invalid is kept by default | `grep -n 'default' internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` shows no origin action default, and `test/plugin/rpki-invalid-kept-by-default.ci` passes |
| The action container is gone, not kept | `grep -rn 'OriginInvalidAction\|OriginNotFoundAction' internal/` returns nothing |
| The filter type is discovered, not listed | `ze cli -c "show policy"` names `rpki-state` against `bgp-rpki`, and `grep -rn 'rpki-state' internal/component/plugin/ internal/component/command/` returns nothing |
| The set half exists | `TestBuildDecisionsPolicySetsState` passes, and `rfc/requirements/rfc6811.md` binds RFC6811-3-1 to it in both polarities |
| The migration is loud | `test/parse/rpki-action-retired.ci` and `test/ui/doctor-rpki-no-origin-policy.ci` pass |
| Interop proves the default | `./le test integration bgp-rpki-invalid-kept-gobgp` passes, with the recorded RED from the reverted build |
| No page contradicts the code | `./le verify docs` passes and `grep -rn 'action {' docs/guide/rpki.md` returns only ASPA examples |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed on an unresolvable policy | A policy name that does not resolve at RUNTIME must not silently degrade to accept. Config validation refuses an unknown name (AC-9), and the runtime path must have no branch that treats a missing definition as no policy |
| The default is a security relaxation | This spec makes ze accept routes it used to drop. Every operator-facing surface that can say so must say so: the startup line, the doctor check, the status answer and the migration section. Silence here is the security defect |
| Set-state cannot launder a hijack | A `set-state valid` clause is operator-stated per peer and per state. It must never be reachable from peer-controlled data: no community, no attribute and no AS_PATH content may select a policy |
| Resource exhaustion | The policy evaluation runs per NLRI on the validation worker. It must be a map or array lookup on an already-resolved definition, with no allocation and no parse per route |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Phase 5 needs a code change | The evaluation point is wrong → DESIGN, and A-1 and A-2 are re-tested |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The ordering constraint is what decides this design, and it is not obvious from either
  the RFC or the config surface. Ze's reactor runs its ingress filter chain and then
  dispatches to plugins, so a chain filter is upstream of every validation state that
  exists. A policy that matches a validation state cannot live where ze's other filters
  live, and no amount of plumbing in `rpc.FilterUpdateInput` fixes it, because RFC 6811
  Section 4 re-validation would still have no chain to re-run.
- FRR reaches the same conclusion from the other side. Its documentation states that soft
  reconfiguration must be enabled for re-validation to work, which is the same requirement
  to hold a pre-policy copy of the route. Ze's Adj-RIB-In already is that copy, so ze
  needs no equivalent switch.
- The default flip narrows an existing defect rather than widening it. The open
  RFC6811-2-1 finding makes a confederation route and a locally originated route read
  Invalid on the JSON event rail. Under today's default those routes are dropped. Under
  the new default they are kept, so the flip strictly reduces that defect's cost.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Invalid is accepted by default | Keep the reject default, which is RFC 7454 and MANRS operational practice | Owner ruling, 2026-08-30. The RFC's MUST NOT is unconditional and a default is not the explicit configuration it excepts. An operator arriving from GoBGP, FRR or BIRD must not meet silent route loss he did not configure. This is settled and is not reopened at review |
| The policy is evaluated at the rpki decision point | A filter in the reactor's `filter { import [...] }` chain, like every other ze filter | The chain runs before plugin dispatch, so no state exists there, and the chain cannot be re-run for RFC 6811 Section 4 re-validation. The decision point is re-entered by `applyToInstalled` already |
| `bgp-rpki` owns the `rpki-state` filter type | A separate `internal/component/bgp/plugins/filter_rpki_state/` directory, matching the `filter_*` family layout literally | The evaluation needs the ROA cache and the decision point, and both are fields of `rPKIPlugin`. A separate plugin would need the state over IPC per route, which the hot path forbids. The family PATTERN is registration plus a `bgp/policy` augment, and both are followed; only the directory differs |
| The origin `action` container is deleted | Keep it as the Section 2 default and add the policy as the Section 3 surface | Two spellings for one decision is the duplication `ai/rules/principles.md` bans and the hybrid `ai/rules/no-layering.md` bans. GoBGP and FRR each carry one surface, not two |
| ASPA's action container survives | Convert it in the same spec | ASPA is a different state space under a different document, and `aspaOverridesAccept` runs after the origin decision rather than inside it. Converting it is a separable change with its own RFC obligations, and folding it in doubles this package's size |
| A chain reference to `rpki-state` is refused | Accept it and evaluate it synchronously in the chain | An operator who writes it would get a filter that can never see a state after a VRP change. A guard that fails closed with the right spelling in its message costs one validation branch |
| RFC6811-2-1 is NOT fixed here | Fix the JSON event rail's origin derivation in this spec | The dependency test decides it: this spec's goals hold whether or not that defect exists, and the flip reduces its cost. Its fix changes the plugin event payload to carry AS_PATH segment types and the local AS, which is a protocol contract with its own compatibility surface. It has a journal row at `plan/journal/guard-added-to-one-half-of-a-pair.md` and needs its own spec |

## Current test campaign investigation

The November 2026 RFC test campaign adds behavior tests before a production
change. These tests have no `RFC requirement` tags: a compiled or observed red
is not conformance evidence, and no recorded producer break is claimed here.
The main thread owns formatting, compilation, scoped runs and native records.

| Test | Entry point and controls | Investigated producer and expected baseline failure |
|------|--------------------------|-----------------------------------------------------|
| `rpki/rfc6811_default_invalid_policy_test.go::TestRPKIInvalidExclusionRequiresExplicitConfiguration` | Parse cache-only and explicit invalid=reject configurations, validate the same real VRP against authorized and unauthorized origin ASNs, then build the typed decisions | `parseRPKIConfig` sets implicit invalid=reject. Cache-only is expected to fail the accept/eligibility assertions; explicit reject and Valid controls should pass. This is Goal 1's existing defect, not a new policy choice. |
| `rpki/rfc8210_rtr_identity_lifecycle_test.go::TestRTRTLSSerialResponseSessionIDMismatch` | Two authenticated TLS connections to one cache: a completed v1 Reset Query base, then a Serial Query; change only the Cache Response Session ID and matching End of Data ID in the second reply | `RTRSession.handlePDU` adopts every Cache Response ID without comparison. The changed-ID serial reply is expected to succeed and merge the delta incorrectly, omit Corrupt Data, and retain the old base instead of flushing it. The identical-ID control must succeed. |
| `rpki/rfc8210_rtr_identity_lifecycle_test.go::TestRTRTLSEndOfDataSessionIDMismatch` | Keep the Cache Response ID fixed and change only End of Data's ID; the identical-ID control completes | `handlePDU`/`applyEndOfData` do not compare the End of Data Session ID with the response. The mismatch is expected to commit the staged payload, omit the mandatory report and omit the cache-data flush. |
| `rpki/rfc8210_rtr_identity_lifecycle_test.go::TestRTRTLSSupportedVersionCannotChangeWithinResponse` | After a v1 Cache Response, change just a Prefix PDU or End of Data PDU to another supported version, keeping Session ID and serial base fixed; unchanged-version control completes | `handlePDU` checks the supported range but not equality with the negotiated version. Both mixed-version replies are expected to publish and advance the v1 serial incorrectly. This test asserts the MUST to drop; it does not turn Error Report 8's SHOULD into a MUST. |

RFC 8210 Section 5.1 states: "If, at any time after the protocol version has
been negotiated (Section 7), either the router or the cache finds that the value
of the Session ID is not the same as the other's, the party which detects the
mismatch MUST immediately terminate the session with an Error Report PDU with
code 0 ("Corrupt Data"), and the router MUST flush all data learned from that
cache." Section 7 states: "If either party receives a PDU for a different
Protocol Version once the above negotiation completes, that party MUST drop
the session; unless the PDU containing the unexpected Protocol Version was
itself an Error Report PDU, the party dropping the session SHOULD send an Error
Report with an error code of 8 ("Unexpected Protocol Version")."

The bounded repair path is a separate RTR session-identity pass: retain the
query kind and negotiated identity, distinguish legitimate replacement after a
Reset Query from an ID change while answering a Serial Query, check End of
Data before publishing, and route detected mismatches through the existing
Error Report encoder and cache-data expiry/revalidation machinery. It requires
independent design/review before production edits. This test campaign supplies
the failing behavior without implementing that larger change.

The existing `TestBuildDecisionsOriginInvalidAction` proves explicit action
selection, but never tests the absence of that configuration (RFC6811-2-2) or
sets a route's validation state through policy (RFC6811-3-1). Its tags must not
be used to declare either whole requirement proven. `TestValidateFourOctetAS`
does discriminate matching 32-bit ASNs from ASNs with the same low 16 bits.
`TestHandleROAChangeReValidates` exercises cache-change dispatch; the paired
Adj-RIB-In test separately checks application to installed routes. Neither
test alone is an end-to-end cache-to-selection proof.

The main thread ran the two existing controls and four hand-written semantic
faults for four-octet origin lookup and origin-cache-change dispatch. All four
native clean-green/named-red observations succeeded; commit `ce329adfcd` retains
them in `rfc/discrimination/rfc6811/RFC6811-3-2.json` and
`rfc/discrimination/rfc6811/RFC6811-4-1.json`. The superseded scratch proposals,
command script and job output can now be removed. These records do not resolve
the default-policy or RTR identity failures above, and do not prove policy
state-setting or end-to-end cache-to-selection behavior. The new known-red
tests receive no conformance credit while their clean baseline fails.

### Selected-route revalidation probe

The existing late-sync observer checks applied Adj-RIB-In eligibility, while
the tagged unit checks the decision queue. Neither reads actual route selection.
The new `test/plugin/rpki-revalidate-selected-route.ci`
uses `internal/test/fixture/rpki_selected_revalidation.go` to close that evidence
gap for a mapping addition. Its independently reviewed positive tag claims
only deselection after addition, retained bytes and a selected Valid control.
It does not claim that the subject's candidate disappears. No product repair
or whole-RFC credit is claimed.

The test runs on Ze's Linux kernel through the existing per-test namespace
runner. Its declared dummy interface `zerpkisel` holds `10.0.0.254/24`.
The daemon's real kernel interface backend and connected plugins must publish that
connected path before a BGP next hop of `10.0.0.1` can be selected. The fixture
never injects a constructor-created RIB path. It explicitly loads bgp-rpki,
bgp-adj-rib-in, bgp-rib and the internal system RIB, with Invalid rejection and
NotFound acceptance stated in the configuration.

| Phase | Required observations |
|-------|-----------------------|
| Before release | Both real wire routes are retained as NotFound and eligible. The subject and control are selected in `show bgp rib best`, the authoritative `show rib candidates`, and the selected system view `show rib`; the connected candidate names the actual namespace device. The VRP cache is still empty. |
| Mapping addition | Only the observer's existing mock-release file barrier changes. The fixed covering VRP authorizes the control's origin and makes the subject Invalid, without another source UPDATE. |
| After release | The subject remains retained as Invalid and ineligible, with exact original nonempty attributes, next-hop and NLRI fields. It leaves BGP best and selected Loc-RIB state. The independently Valid control stays retained, eligible and selected through every queried view, with its original bytes. Failed, null or malformed inspection answers cannot satisfy absence. |

The state polls and observer context bound each phase. Success is reported only
after the final selection and peer End-of-RIB checks. Main observed one
counted execution in
`tmp/session/2026-11-14-210034/scratch/job-rpki-selected-own-system-0b89ec1c.log`
and all 80 load iterations in
`tmp/session/2026-11-14-210034/scratch/job-rpki-selected-counted80-c4318ee9.log`.
Both runs report the exact requested, completed and executed counts,
per-test namespaces, all phases passing and no skipped phase. The private
guest runner was built after this fixture was frozen; its build receipt is
`tmp/session/2026-11-14-210034/scratch/job-rpki-guest-fixture-le-ready-d7151f12.log`.
Main then promoted the draft, changing only its reviewed claim comments.
The native normal-return fault observation
omits only `r.removeLocRIB(fam, pfx)` in the no-candidates branch of
`rib/rib_bestchange.go::RIBManager.checkRouteBestChange`. That branch removes
the BGP best record before the Loc-RIB call, so this cut must leave retained
Invalid state and BGP deselection intact while the authoritative Loc-RIB still
selects the subject. Suppressing the later `publishBestChanges` event is not
this proof: the Loc-RIB mutation already happened before that event. A queue
or received-table assertion is not a substitute for the selected-state red.

The promoted test passed by name in the native clean run, with exactly one
executed test and no skipped phase. Under that private compiler overlay it
failed at the intended post-sync assertion: the authoritative Loc-RIB still
selected `10.0.1.0/24`, while the Valid `10.0.2.0/24` control and the actual
connected namespace path remained selected. The retained Invalid/ineligible
verdict, immutable received bytes and BGP-best deselection checks passed
before that failure. The broken run does not reach the selected-system-view
check; the clean case does. This is an assertion failure, not a startup
failure, panic or outer test timeout.

`rfc/discrimination/rfc6811/RFC6811-4-1.json` retains the native positive
observation. The clean and broken named results are in
`tmp/session/2026-11-14-210034/scratch/job-rpki-selected-native-proof-cli-fixed-f115cdb9.log`.
Both own-system VMs stopped after their runs, using Ze's 7.3.0 kernel with
SHA-256 `db34b481243156f83adda0d83f6e878ac18e22f01bc18e2b05bc6711af1ca1d2`.
Independent review accepted the observation and retained a weak whole-row
verdict because mapping deletion remains unproven.

Mapping deletion remains outside this addition-only probe. The existing
`le mock rtr` serves a fixed VRP set on every query and has no state-driven
VRP deletion input. This task adds no generic mock option. The Section 4 audit
therefore remains bounded until deletion and the other missing proof are
separately covered. The ignored draft was removed by promotion. Retain the
two execution receipts and the private guest bundle under the existing
campaign scratch ownership note through the native fault proof and commit;
remove the compiled bundle after no owned VM uses it.

The older retained-route carrier also passed exactly 80 requested, completed
and executed iterations, with no skipped phase, in
`tmp/session/2026-11-14-210034/scratch/job-rpki-late-sync-counted80-a1c7df8e.log`.
Main promoted its three header comments to
`test/plugin/rpki-revalidate-late-sync.ci`; its executable test is unchanged.
Its native clean run then passed by name, and the normal-return
`handleROAChange` fault left that synchronized route NotFound and eligible.
The intended post-sync Invalid/ineligible assertion failed with
`ZE-OBSERVER-FAIL` in
`tmp/session/2026-11-14-210034/scratch/job-rpki-late-sync-native-proof-help-fixed-e5eaf04b.log`.
The recorder wrote the functional positive observation to
`rfc/discrimination/rfc6811/RFC6811-4-1.json`. This proves applied retained
validation, not selected-route publication. Neither carrier exercises
mapping deletion.

Final reconciliation also completed both existing tags on
`adj_rib_in.TestReValidationAppliesToInstalledRoutes`. Independent review
first narrowed its positive claim: typed and text decisions rewrite stored
state, the typed rewrite preserves replay sequence, and its explicit
`Accept=false, Ineligible=false` discard removes the route. Normal
Invalid-policy rejection instead carries `Ineligible=true` and retains the
received bytes; this unit's discard subcase is not evidence of that policy.
Its assertions and negative claim are unchanged.

Both native clean runs passed all five subcases. Forcing stored state to
remain NotFound failed exactly the three state-rewrite subcases, with discard
and unknown-prefix controls passing. Replacing only the absent-route early
buffer with promotion failed exactly the unknown-prefix subcase: it exposed
an extra route the peer never sent and a missing early buffer; the four
installed-route controls passed. These normal-return observations are in
`tmp/session/2026-11-14-210034/scratch/job-rpki-installed-state-positive-proof-a04a0993.log`
and
`tmp/session/2026-11-14-210034/scratch/job-rpki-installed-unknown-negative-proof-979cc635.log`.
Together with the two tracker and two functional entries, the canonical
Section 4 record now holds six bounded observations. Mapping deletion still
prevents a whole-requirement enforcement claim.

Five further normal-return producer faults completed the existing Section 2
origin-state claims. Every named clean run passed. Forcing promotion to Valid
failed both transports' absolute Invalid-state checks; reversing each lookup
result failed the corresponding Valid or Invalid assertion. Selecting the
first AS_PATH segment failed only the two new multisegment cases, and using
an AS_SET member as origin failed both final-set cases while the other
controls passed. The observations are retained in
`rfc/discrimination/rfc6811/RFC6811-2-1.json`; their clean/red receipt is
`tmp/session/2026-11-14-210034/scratch/job-rpki-origin-existing-native-proofs-e302411f.log`.
Retain this receipt through review and commit, then remove it once the
canonical records and independently rejudged audit preserve the decision.
That rejudge retains `enforced` for the exact state-from-lookup MUST. The
scoped RFC 6811 inventory reports 19 recorded claims, zero stale records and
zero unproven claims. This is claim evidence, not whole-RFC support.

## Known Limitations

- RFC 8097 stays unimplemented, so a validation state, set or derived, does not propagate
  to an iBGP peer. Ze holds it as the local property RFC 6811 Section 2 describes. Carrying
  it in the extended community is a separate feature with its own wire surface.
- The `rpki-state` object is referenced from the rpki container only. It is not composable
  with the ordered import and export chains, and the refusal in AC-10 makes that explicit
  rather than leaving it to be discovered.
- ASPA keeps its own `action` container, so the daemon carries two shapes of RPKI-adjacent
  policy config until ASPA is converted. This is stated, not hidden, and the two govern
  disjoint state spaces.
- RFC6811-2-1's exact state-from-lookup MUST is audited as enforced. Structured
  receive, reload replay and JSON receive with raw attributes use segment-aware
  origin extraction; normal subscriptions request full events. The flattened
  compatibility fallback without raw attributes lacks segment types. These
  proofs do not establish every legacy input shape or family-delivery path,
  and do not close the separate policy and cache-revalidation obligations.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Site | Section | Requirement to quote |
|------|---------|---------------------|
| The absence of a default disposition in `parseRPKIConfig` | RFC 6811 Section 2 | "An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so." |
| The policy evaluation in `buildDecisions` | RFC 6811 Section 3 | "An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function." |
| The set-state clause writing `ValState` | RFC 6811 Section 2 | "The implementation should consider the validation state as described in the document as a local property or attribute of the Route." |
| The `rpki-state` list description in YANG | RFC 6811 Section 5 | "Policies that could be implemented include filtering routes based on validation state (for example, rejecting all 'invalid' routes)" |
| The re-validation path's policy call | RFC 6811 Section 4 | The re-validation obligation already documented at `applyToInstalled` |

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package and relevant functional/interop tests pass; GitHub CI supplies whole-tree verification after an authorized push. Agents do not run any `./le verify` command or test a copy of the tree
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson is recorded in its governing surface; no separate lesson artifact is required merely for closure
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** remove the spec through the generated `./le commit create` script (commit A preserves it in history)

## Observed test-campaign results: 2026-11-15

The four new probes compiled and ran in
`tmp/session/2026-11-14-210034/scratch/job-rfc-campaign-new-batch-reds-37b87864.log`.
These are observed behavior failures, not build failures or discrimination
records. Their conforming controls passed.

| Probe | Observed failure | Passing control |
|-------|------------------|-----------------|
| `TestRPKIInvalidExclusionRequiresExplicitConfiguration` | Configuring a cache alone rejects an Invalid route | Explicit invalid=reject still rejects it |
| `TestRTRTLSSerialResponseSessionIDMismatch` | Changed Cache Response ID is accepted without the required error/flush | Matching ID completes normally |
| `TestRTRTLSEndOfDataSessionIDMismatch` | Changed End of Data ID completes and publishes the response | Matching ID completes normally |
| `TestRTRTLSSupportedVersionCannotChangeWithinResponse` | A supported version switch in a prefix or End of Data advances/publishes a mixed response | Unchanged version completes normally |

The TLS cases use real mutual-TLS connections and a completed initial response
before the serial query. They do not establish that every transport or every
RPKI policy surface is correct. Complex repair remains in this spec for later
review under the owner's test-first campaign instruction. No gap, audit
verdict, support figure or approval gate was upgraded.

Retain the shared log until all owning specs that cite it have superseding
evidence. Do not remove it solely because this spec's cases later pass.

## Review Gate

<!-- Filled at implementation time by /ze-review, per ai/rules/planning.md.
     Round 1 reviews the whole diff with at least two lenses; round N+1 reviews
     only the fixes round N made plus the sibling call sites they touched.
     Each round's scope is written here BEFORE it runs. -->

### Round 1
| Scope | Lens | BLOCKER | ISSUE | NOTE |
|-------|------|---------|-------|------|

### Round 2
| Scope | Lens | BLOCKER | ISSUE | NOTE |
|-------|------|---------|-------|------|
