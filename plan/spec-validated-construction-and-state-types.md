# Spec: validated-construction-and-state-types

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling, config, protocol |
| Depends | Reconcile the completed enum migration before changing its producers; coordinate overlapping IKE plans below |
| Phase | Research and design draft; implementation not authorized |
| Handoff | - |
| Updated | 2026-10-05 |

## Task

Make first-party invariant-bearing Go types establish validity at admission and preserve it through use. The existing style guide already recommends private-field validated values and distinct lifecycle types. The gap is enforcement across every construction, mutation, decoding and ownership path, together with claims that exceed Go's guarantees.

This is a complete proposal for an ordered census and migration of the full first-party population. The examples below establish the problem and the first dependency order; they do not limit the migration to IKE or a pilot. The owner authorized this draft and the accompanying rule clarification. Approval of the proposed migration design, its compatibility decisions and implementation remains a separate gate. Do not claim this spec, change its status to `ready`, or start its implementation as part of the enum sweep.

### Owner requirement, 2026-10-05

> As I have you here there is two aspect to the optimisation: ensuring you can only have valid objects that Go can ensure with the 'new' object function creation and then using sub-type to make sure the data is always valid, you have well understood both and that is what is being implemented ? If the current sweep is about the Enum side, we should consider a follow up spec for the validation at creation and update our rules. Do you agree and if so please have an agent do it.

The governing policy is `ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md`, rendered into `ai/rules/go-standards.md`. Its new-code scope applies from 2026-10-04. This spec proposes the legacy migration scope for separate owner approval; it does not expand the current enum implementation. The style guide explains the language limits without holding a second normative policy.

| Scope | Decision |
|-------|----------|
| Included | Every first-party Go type whose consumers require a restricted value, compatible field combination, validated aggregate, ownership invariant, or Ze-controlled operation order; production, tooling, tests, examples and platform/tag variants all count |
| Discovery population | Enumerate first-party type declarations and invariant-bearing anonymous aggregates, then inspect existing validators, constructors, state transitions, decoders, getters and setters. The enum census is one discovery input; types with no enum or switch still count |
| Excluded from compulsory encapsulation | Unconstrained records, raw external data-transfer objects, open wire identifiers, external plugin contracts and runtime protocol states that intentionally represent unvalidated or changing input; record each exclusion and its admission consumer |
| Preserved | Wire/config/CLI syntax, supported algorithms, policy decisions, error identities, valid zeros and sentinels, registration, transaction ordering, buffer ownership and existing zero-copy paths |
| Not commissioned | New protocol support, algorithm redesign, a generic validation framework, reflection-based validation infrastructure, a central type registry, a blanket linter, or unsafe/reflection-based circumvention of package privacy |
| Release bucket | `plan/`: this is a proposed architectural migration, with no demonstrated operator defect newly claimed here. A reproduced operator defect discovered during the census follows its own immediate-fix scope; it does not turn the entire migration into release-blocking work |

### Related work

| Plan or record | Relationship and constraint |
|----------------|-----------------------------|
| `docs/contributing/ze-go-style.md`, “Every enum switch has a coverage policy” | Carries the C/O/P switch classification and exhaustive-linter policy established by spec-enum-switch-exhaustiveness. `NewX` does not establish C closure. This spec does not retrofit constructors to justify a panic in that sweep |
| `plan/spec-ike-missing-transforms.md` | Owns new algorithms and DH table design. Keep its supported set and algorithm implementation independent; adapt encapsulation to the table if that plan lands first |
| `plan/spec-ike-post-quantum.md` | Owns future key exchange support. No post-quantum or hybrid design enters this migration |
| `plan/spec-ipsec-opaque-selector-port-mask.md` | Owns acceptance and exact dataplane encoding of OPAQUE. Representation changes here preserve the refusal in the implementation tree unless that separate plan has already changed it |
| `plan/journal/earlier-guard-hides-the-better-error.md` | The DH public-value record shows why a wrong-length input cannot prove the numeric range check. Boundary tests below isolate each guard |
| `plan/journal/committed-tree-does-not-build.md` | The storage-constructor record shows missed callers on another platform. Census reconciliation includes tests and every supported build population |
| `plan/journal/unwired-feature.md` | The schema-length record motivates proofs through actual admission consumers. This draft makes no claim that the historical defect remains present |

## Required Reading

### Architecture Docs and Rules

- [ ] `docs/contributing/ze-go-style.md`, Types that cannot lie, One state one variant, Validated at construction and One type per lifecycle state.
  → Constraint: use private invariant-bearing fields and narrower operations, but account for Go zero values, interface embedding, typed nils and surviving aliases.
- [ ] `ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md` and `ai/rules/go-standards.md`.
  → Decision: one authoritative construction policy; the legacy migration requires this spec's separate approval.
- [ ] `ai/rules/planning.md`, `evidence.md`, `writing.md`, `documentation.md`, `architecture.md`, `simplicity.md`, `spec-no-code.md`, `rule-format.md`, `repo-maintenance.md`, and `docs/contributing/rule-authoring.md`.
  → Constraint: retain the enum session's claim, name behavior producers, keep specs code-free, edit canonical rule points, and leave review evidence empty until an independent review runs.
- [ ] `plan/README.md`, `plan/TEMPLATE.md`, `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, and `docs/contributing/writing-style.md`.
  → Decision: use the optional-release bucket and the existing Go-style discovery entry; derive additional documentation from the census's actual files.
- [ ] `docs/architecture/core-design.md` and `docs/architecture/buffer-architecture.md`; `ai/rules/performance.md`.
  → Constraint: keep component dependency direction, registered command discovery and borrowed buffer lifetimes. Encapsulation cannot introduce per-event copies or replace byte views with allocated object trees.
- [ ] `docs/architecture/ike/ipsec-6-ikev2-crypto.md`.
  → Constraint: keep unsupported-group errors, peer key checks and best-effort clearing. A type change cannot claim secure memory erasure.
- [ ] `docs/architecture/ike/ipsec-3-data-model.md`, `ipsec-7-ikev2-engine.md`, `ipsec-8-ikev2-child-xfrm.md`, and `rfcgate-1b-rfc7296-pilot.md` in `docs/architecture/ike/`.
  → Constraint: parsing, cross-reference validation and backend programmability are different claims. Preserve selector narrowing, protocol-zero rules, proposal ordering and candidate-PKI isolation.
- [ ] `docs/architecture/config/transaction-protocol.md`.
  → Constraint: verification has no live-state side effects; runtime apply and publication can still fail after a candidate is structurally valid.
- [ ] `docs/architecture/api/commands.md` and `docs/contributing/running-commands.md`.
  → Constraint: YANG supplies command metadata and shared consumers validate arguments; keep registration-derived vocabulary and use the native build/test populations.
- [ ] `docs/architecture/config/yang-config-design.md`, CLI Help from YANG.
  → Constraint: command metadata admission preserves the independently authored summary and description; no consumer derives one from the other.

### RFC Summaries

- [ ] `rfc/short/rfc7296.md`, including the current Support and Support remaining rows.
  → Constraint: preserve DH input refusal, selector narrowing and the disclosed OPAQUE limitation. No Support row or RFC verdict is promoted by this representation migration. Read each affected enforcing requirement before an implementation changes its carrier.

**Key insights:** validation is a property of a value at a use site, including its aliases and lifetime. A successful constructor proves only what it checked. Parsed config can still lack valid references. A finite local category can already be safe without a public constructor. Go cannot make a transition consume every copy of its input.

## Current Behavior

Source was read before this draft. The enum census manifest and the completed `EnumAuditSecuritySubscriber01`, `EnumAuditSecuritySubscriber02`, `EnumAuditTools03` and `EnumAuditConfigPlatform02` reports supplied discovery leads only. The following claims come from their named producers and consumers. `gopls references` resolved `DHExchange.GroupID` and `PortSelector.Form`; those host references are not a full build-matrix census.

**Source files read:**
- [ ] `internal/component/ike/crypto/dh.go`: exchange construction, shared-secret validation and clearing.
- [ ] `internal/component/ike/engine/initiator.go`: exchange construction and encoded group identity.
- [ ] `internal/component/ike/ipsec/traffic_selector.go`: port normalization, config parsing and programmability.
- [ ] `internal/component/ike/engine/ts_narrow.go`: wire conversion, intersection and contextual admission.
- [ ] `internal/component/ike/engine/child.go`: selector-to-dataplane conversion.
- [ ] `internal/component/ike/ipsec/config.go`: parsed config construction and empty defaults.
- [ ] `internal/component/ike/ipsec/types.go`: config maps, proposal slices and aggregate fields.
- [ ] `internal/component/ike/engine/config.go`: raw JSON parsing and candidate validation.
- [ ] `internal/component/ike/engine/register.go`: verify/configure/apply callbacks and staging.
- [ ] `internal/component/config/yang/command.go`: argument metadata lowering.
- [ ] `internal/component/command/node.go`: argument kind and payload representation.
- [ ] `internal/component/command/argvalidate.go`: argument validation consumers.
- [ ] `internal/le/chaos/selftest/actions.go`: private finite categories and their captured consumers.
- [ ] `internal/component/ike/engine/config_verify_test.go`: candidate-PKI side-effect proof.

| Source files and symbols read | Observed behavior | Migration consequence |
|--------------------------------|-------------------|-----------------------|
| `internal/component/ike/crypto/dh.go`: `DHExchange`, `NewDHExchange`, `ecpExchangePublic`, `SharedSecret`, `HasPrivate`, `Clear`; `engine/initiator.go`: `newInitiatorSA`, `encodeSAInitRequest` | Constructor accepts supported groups and returns an error otherwise. GroupID and PublicKey are exported and mutable; private key fields are private. SharedSecret checks private-key presence, then dispatches by GroupID and validates peer bytes. Clear changes the object and zeros the public slice. The initiator casts configured group IDs, checks constructor errors and encodes the exchange's own GroupID | Keep group, public bytes and private implementation consistent through creation and use. Privacy alone does not prevent copying a DHExchange with shared private pointers; clear/use behavior needs an ownership contract |
| `internal/component/ike/ipsec/traffic_selector.go`: `PortSelector`, `AnyPort`, `PortSelectorFromWire`, `parsePortSelector`, `checkPortProgrammable`, `Wire`, `String` | Form and Port are independent exported fields. Wire normalization accepts ANY, OPAQUE and equal endpoints, including single port zero. Config parsing accepts numeric ports 1..65535. Programmability rejects OPAQUE and rejects a single port with protocol zero. Unknown forms reach ANY in Wire/String, although validation rejects them | Separate raw encoding, normalized port form and context-dependent programmability. A successful parse is not proof that installation is allowed; an invalid form cannot be admitted as a wildcard |
| `internal/component/ike/engine/ts_narrow.go`: `wireToSelectors`, `intersectPort`, `programmableSelector`; `engine/child.go`: `selectorPort` | Wire ranges can be narrowed to their first nonzero port. Programmability is checked during negotiation. The final sink maps single and opaque forms explicitly and otherwise returns any-port | Preserve legitimate range narrowing and the safe protocol checks. Replacing an exported field needs the config, negotiation and dataplane callers together |
| `internal/component/ike/ipsec/config.go`: `EmptyConfig`, `ParseIPsecConfig`; `ipsec/types.go`: `IPsecConfig`, `IKEGroup`, `ESPGroup` | Parser returns a mutable config with maps, proposal slices and references. EmptyConfig explicitly selects bypass, because the action zero is PROTECT. Parse success does not run the full cross-reference chain | Keep empty-delivery behavior. Give parsed and admitted config different types instead of treating a parsed pointer as a validation token |
| `internal/component/ike/engine/config.go`: `parseIPsecFromJSON`, `parseVPNSections`, `parseIPsecSections`, `candidatePKI`, `validateIPsecSections`; `engine/register.go`: verify/configure/apply callbacks, `ikeConfigStaging.stage`, `commit` | JSON becomes an open map, then a Tree and IPsecConfig. Validation returns only an error and reads candidate PKI without publishing it. Runtime verify then parses again, checks unmatched-policy support and stages a plain pointer. stage assumes prior validation. commit clears pending before apply and rejects absent pending config. Startup parsing can load PKI | Proposed candidate construction returns the exact validated object that staging accepts. Preserve startup/reload differences and separate validation from installation; do not infer a current exploit from this structural gap |
| `internal/component/config/yang/command.go`: `yangTypeToArgDef`; `internal/component/command/node.go`: `ArgDef`; `argvalidate.go`: `ValidateArgString`, `validateUint`, `validateUnion` | YANG lowering chooses a finite kind and populates kind-specific payloads. ArgDef exposes Kind, numeric width/ranges, enum slices and union slices independently. Consumers trust those relationships; numeric width zero means 64. At research time an unknown kind returns nil in ValidateArgString | Inventory command definitions as invariant-bearing combinations and preserve established defaults. The current enum spec already owns unknown-kind dispatch policy; this migration follows its completed result |
| `internal/le/chaos/selftest/actions.go`: `Action`, `Table`, `table`, `actionAnswer`, `Argv`, `envOptions` | Table assigns private kinds, table captures those actions, and actionAnswer derives argv/environment from the same category. Zero/unknown kinds already assert a BUG | This is an existing finite-producer pattern. Do not add constructors merely because the type is a struct; record its zero/misuse contract and whether any wider API needs migration |
| `internal/component/ike/engine/config_verify_test.go`: `TestValidateIPsecSectionsDoesNotMutatePKIStore` | A candidate has valid PKI material and invalid VPN config, then the test checks that the live store did not acquire the candidate CA | Extend this behavioral proof through the registered verify/apply path; preserve its valid-PKI premise |

### Behavior to preserve

| Contract | Preservation requirement |
|----------|--------------------------|
| Accepted data | Every previously valid config, command argument and peer message keeps its meaning, ordering, output and error contract. Invalid construction is closed without silently changing protocol support |
| Zero and sentinel behavior | Keep valid enum zeros, absent selector/config defaults, empty collections, nil-as-absent where supported, width-zero-as-64, and cleared-exchange errors. Record per-operation zero behavior before migration; do not reinterpret a sentinel as an ordinary successful domain value |
| Selector boundaries | ANY remains 0/65535, OPAQUE remains 65535/0, single N remains N/N. Wire single zero and configured numeric zero remain distinct admission cases. Preserve range narrowing and current OPAQUE programmability refusal |
| Config transactions | Candidate verification cannot publish PKI, config or kernel changes. Apply still performs environment-dependent checks and existing rollback/publication handling. Host availability is not an immutable type invariant |
| Crypto | Keep peer length/range/curve checks, entropy failures, supported algorithms, clear behavior and best-effort erasure claims |
| Open extension points | A plugin, file, wire or kernel input remains open until its real admission boundary checks it; extension registries remain authoritative |

The intended change is to make internal APIs carry those established guarantees, close mutation bypasses and remove repeated internal validation only where producer and ownership evidence permits it. No measured speedup is claimed.

## Data Flow

### Entry Point

Config enters as JSON section deliveries or parsed trees; peer key and selector data enters as wire payloads. Command metadata enters through YANG lowering, while lifecycle operations enter through the owning package's methods and SDK callbacks.

### Transformation Path

| Entry | Current path | Proposed path after approval |
|-------|--------------|------------------------------|
| Config JSON/tree | Decode → parse → validators return error → reparse → plain-pointer staging → apply | Decode into raw/candidate data → parse and validate the candidate once → package-owned validated snapshot → staged transaction → apply with current host checks |
| Peer DH bytes and configured group | Raw group → NewDHExchange → mutable exchange → SharedSecret checks bytes | Raw group → validated exchange construction → private consistent algorithm/key state → SharedSecret still checks each peer value and live ownership |
| Config or peer selector | Parse/normalize → mutable form and payload → contextual checks → narrowing → dataplane | Raw pair/token → normalized value → context-validated selector → narrowing result → sink; each boundary preserves the supported set |
| YANG command metadata | Lower into ArgDef → publish tree → validators/completers/renderers | Lower into builder/raw metadata → construct valid argument variant → publish immutable metadata → existing consumers |
| Ze-controlled lifecycle | State plus mutable payload and caller convention | State-specific APIs for operations; shared runtime state where revocation, reuse or aliases can outlive a transition |

### Boundaries Crossed

| Boundary | How | Evidence and residual check |
|----------|-----|-----------------------------|
| External data → domain | Parsers, decoders, constructor errors | The producers in Current Behavior exist; the narrowed return types are proposed. Invalid input returns the current safe error/refusal rather than a BUG panic |
| Parsed → validated config | Candidate validation and staging | `validateIPsecSections` and `ikeConfigStaging.stage` currently share an ordinary pointer type. Validate cross-references against the same candidate snapshot |
| Validated value → mutable storage | Maps, slices, pointers, pool views | Publication includes a declared owner/lifetime. Do not return an unchecked mutable alias from an accessor |
| Validated local state → peer interaction | FSM/message handlers | Peer events and time-dependent facts remain runtime checked, regardless of the local state type |

### Integration Points

| Existing integration | Proposed connection |
|----------------------|---------------------|
| `validateIPsecSections` and registered SDK verify/apply callbacks | Return the admitted candidate to staging without reparsing; keep environment checks at apply |
| `NewDHExchange`, `newInitiatorSA` and `SharedSecret` | Carry consistent private exchange state while preserving peer-key validation |
| `PortSelectorFromWire`, `wireToSelectors` and `selectorPort` | Carry normalized then context-validated selectors to the existing dataplane sink |
| `yangTypeToArgDef` and `ValidateArgString` | Publish admitted argument definitions for existing validators, completers and renderers |

### Architectural Verification

| Check | Holds in the proposed design | Evidence / constraint |
|-------|------------------------------|-----------------------|
| No bypassed layers | Required | Reuse actual parser, SDK verify/apply, negotiation and sink entry points; no alternate validation path |
| No unintended coupling | Required | Validators retain existing callbacks for external references; no new cross-component imports or global validator |
| No duplicated functionality | Required | Replace producer APIs and migrate all consumers; retain raw types only where they still represent distinct external input |
| Zero-copy preserved | Required, not measured | Package-owned views, existing pools and owner-scoped access replace mutable escape paths before considering a copy |
| Registration, outbound | N-A for new registrations | No new command, protocol family, handler or plugin is introduced |
| Registration, inbound | Required | ArgDef conversion preserves YANG and registry discovery. This design introduces no feature-name vocabulary or new central list |

## Proposed Representation and Admission Contract

Apply the canonical construction policy through the following design choices. The census records a choice for every in-scope type before its callers change.

| Situation | Representation and proof |
|-----------|--------------------------|
| Restricted scalar | Use a package-owned value with private payload and a fallible boundary parser/constructor when unrestricted casts would break the invariant. Retain an ordinary enum for open IDs and unconstrained categories where it expresses the actual domain |
| Mutually exclusive payloads | Use distinct concrete variants for distinct payloads. Share behavior through an interface only when consumers need it; package-private marker methods alone do not admit arbitrary embedded implementations safely |
| Aggregate invariant | Keep invariant-bearing fields private. Derive redundant information rather than store independently mutable copies. Construct children before publishing the complete aggregate |
| Raw versus validated | Decoding targets raw/candidate storage. Validation returns the domain value that downstream APIs require. A second type earns its place by removing an invalid combination or an invalid operation from an API |
| Mutation/update | Prefer immutable values and replacement. A mutator validates proposed data before modifying published state; a failed update leaves the previous valid value and its aliases unchanged. In-place decoding of a published validated object is excluded unless it provides the same atomic failure contract |
| Zero | Select and document valid-zero or safe-invalid-zero per type. Exported structs can be zero-created even with private fields, and an unexported returned type can still be copied or zero-created with type inference. Safe-invalid zero is refused at admission/use with no side effect. Existing valid zeros and sentinels keep their behavior |
| Nil and variants | State nil and typed-nil admission separately. If a public interface is retained, embedding an allowed interface/type does not establish valid payloads; validate the admitted concrete representation and nil case. Avoid interfaces when a concrete value satisfies the use |
| Lifecycle and aliases | Distinct types limit which methods compile. They cannot revoke old values, pointer aliases or copied handles. Keep live-resource state in the existing shared owner, and check generation/closed/consumed status where stale aliases remain possible; introduce a generation only if the concrete lifecycle requires one |
| Slice/map/pointer ownership | State who can mutate backing storage before and after construction. Prefer scalar copies, iterators or owner-scoped operations; use existing pool lifetime contracts. A slice returned as read-only by comment remains writable Go memory. A private field is insufficient when a constructor retains caller-owned mutable data |
| Necessary copies | Copy only at a demonstrated ownership boundary where the caller retains mutation rights and no existing ownership API closes the path. Charge cold admission once; no per-use copying or generic defensive clone. Secret material does not acquire avoidable copies |
| Residual runtime facts | Validate peer input on every new message; retain authorization, time, revocation, environment availability, resource lifetime and concurrency checks. A static snapshot does not prove that any of those remains true |

### Selected migration decisions

| Family | Proposed migration | Evidence needed before removing a guard |
|--------|--------------------|----------------------------------------|
| DHExchange | Private group/public/private-key relationship; read access that does not expose mutable stored key bytes; construction bound to the supported implementation; preserve Clear/HasPrivate semantics across aliases | Full constructor/literal/copy/reference graph, clear/use tests and caller-buffer or scoped access proof. Never remove remote-public validation |
| PortSelector and selector aggregates | Private normalized port payload; admission distinguishes raw/normalized from programmable selectors. Remove public Form/Port mutation from validated values and migrate all narrowing/sink callers | Every raw pair, numeric conversion, config parser and error-result consumer accounted for; invalid forms never publish/install ANY |
| Parsed and validated config | Separate raw/parsed values from the immutable validated candidate accepted by staging; return that candidate from the existing validation chain and avoid reparsing the same delivery | Same candidate references and owned collections survive until apply. Initial startup, reload, failed verify and apply-without-verify each pass through the intended checks |
| ArgDef and command tree | Kind-specific validated argument payloads with private invariant-bearing members; construction occurs during lowering/publication, with mutable builders confined before publication | Enumerate manual ArgDef/Node construction as well as YANG lowering, including tests and plugin metadata; preserve kind defaults and consumer output |
| Already-closed finite category | Retain `Action`-style package construction where it satisfies the policy, documenting permitted zero/misuse behavior | Trace the actual producer and captured caller; no ceremonial NewAction wrapper or claim that a private enum proves the whole exported object valid |

The main design tradeoff is preserving a convenient value API while controlling backing-store and resource aliases. An opaque struct is cheaper than interface allocation and usually easier to use, but it cannot express affine ownership. Choose package-owned immutable values for data and existing shared runtime ownership for resources. Reject a design that removes a necessary runtime guard merely to claim all validity is static.

## Census and Complete-Migration Contract

| Census field | Required content |
|--------------|------------------|
| Identity/population | Package, type or anonymous aggregate producer, build constraints, owning source and design page; generated declarations point to their generator |
| Invariant | Exact admitted values/combinations/operations, consumer that relies on them, raw versus validated distinction and zero/nil/sentinel contract |
| All producers | Constructors, zero/new, literals, scalar conversions, factories, reflection/codec population used by production, deserialization, restoration, test fixtures and generated code |
| All changes and escapes | Field writes, setters, transitions, in-place decode, whole-value copies, embedded interfaces, pointer aliases, input/output slices/maps, pool return and shared-owner concurrency |
| Disposition | Already enforced; migrate; intentionally raw/open with named validator; runtime-state with named checks; unconstrained data. Every excluded row has a source-based reason |
| Migration proof | Chosen representation, all callers/tests/docs, compile-negative cases, runtime boundary cases, allocation/ownership evidence and any overlap owner |
| Completion evidence | Final source symbols, direct observed checks, build populations and reconciliation of added/removed declarations since census |

Use temporary type-aware analysis and LSP references, with the repository's build/tag/platform population. Do not install a permanent type inventory or checker. Retain the census and decisions in this spec's implementation record; scratch output alone is not closure evidence. Each declaration receives a disposition, and invariant-bearing anonymous aggregates join the same inventory. Exclusions are reviewed so a renamed DTO or unconstrained label cannot hide a domain invariant. Complete every migration row and every affected caller before closure; finishing the named examples does not satisfy this contract.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | The owner wants the full ordered migration proposed here after reviewing its design | Owner requirement authorizes a follow-up spec; it does not approve this implementation scope | Starting code would exceed authorization | Explicit owner design/implementation gate in the main thread | Awaiting owner decision |
| A-2 | The existing build population and source declarations can bound the census | Existing enum census and feature-tag workflows | Types in unsupported/exempt populations escape the claim | Reconcile repository file inventory, generated inputs, platform files, tests and separate-module examples in Phase 1 | To establish during approved census |
| A-3 | Most mutable data can be confined without a hot-path copy | Existing package ownership and buffer architecture | A chosen API adds allocations or leaves aliases writable | Per-type ownership graph plus before/after allocation and lifetime tests | To establish per migration row |
| A-4 | Preserving current valid inputs and documented sentinel contracts permits a clean internal cutover | Go standards permit internal API replacement; selected sources show existing errors/defaults | A public contract or observable output changes | Trace external consumers, record exact zero/error contracts and return incompatible choices to design | To establish per migration row |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Constructor naming is mistaken for proof | NewX exists beside public fields or direct casts | Inventory all creation and mutation paths; keep input classified open until the full proof holds |
| R-2 | Zero/typed-nil/embedding bypass causes a panic | Compile-negative proof claims these values cannot be made | Positive compilation controls demonstrate what Go permits; runtime admission tests cover those residual cases |
| R-3 | Clearing or consuming one alias leaves another usable | A value copy retains key pointers, maps or a staged handle | Keep a shared owner or immutable snapshot; exercise stale copies and repeat transitions without claiming move semantics |
| R-4 | Validation succeeds, then alias mutation changes the candidate | Constructor input or accessor shares writable backing storage | Close ownership at publication and test mutation of retained inputs/outputs and nested collections |
| R-5 | Failed decode/update partially publishes | Valid early fields replace the old value before a late error | Validate temporary candidate, publish atomically and assert previous object/store state after failure |
| R-6 | A peer reaches a new BUG assertion | Removed guard used to reject wire/config/plugin input | Keep boundary refusal and peer FSM validation; assert malformed input through the actual handler |
| R-7 | Validation certificate outlives a host or authorization fact | Apply relies solely on validation performed earlier | Limit static guarantees to immutable data; recheck resource/environment facts at their current runtime boundary |
| R-8 | Encapsulation adds boxing, cloning or per-event allocation | Benchmark allocs/op rises or borrowed buffers outlive pool return | Prefer concrete values and scoped access; measure the affected path before accepting the representation |
| R-9 | Another active plan changes the same producer | DH table or OPAQUE behavior differs from Current Behavior | Re-read landed source, preserve the separately approved feature and update this spec's recorded baseline |
| R-10 | Census becomes a pilot or silently excludes old tests/platforms | Unmigrated rows disappear or a host-only build is called complete | Reconcile the full declaration population at closure; migrate all callers without compatibility aliases |
| R-11 | Earlier rejection makes a test vacuous | Test passes when the target admission/update guard is removed | Pair valid/invalid controls and verify the target error or state change; use correctly sized DH inputs for range tests |
| R-12 | Over-broad type-state creates new layers and allocation | One-use interface or wrapper forwards every call unchanged | Keep concrete types; separate a lifecycle only where it removes an actual invalid operation |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config can be rejected incorrectly, selector policy can widen, keys can be used after clear, commands can accept invalid arguments, or a peer can panic the daemon |
| How is it reverted? | No wire, config or persisted-data migration is intended. An owner-approved revert/corrective patch must include changed producers and callers together; no partial rollback to mixed APIs |
| Who else touches this path? | The enum sweep and the IKE plans in Related work; the census adds source owners for every later package |

## Wiring Test

New test names below are planned proofs, not claims that tests already exist or passed. Each later census row adds an equivalent concrete entry-point proof before that row's implementation starts.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config section delivery through the registered verify and apply callbacks | → | Validated candidate construction and `ikeConfigStaging` | `TestIKEConfigAdmissionPublishesOnlyValidatedCandidate` in engine config verification tests |
| Rejected config delivery with valid candidate PKI and invalid VPN references | → | Candidate validation before staging/publication | `TestIKERejectedCandidateLeavesLiveStateUnchanged` through the SDK callback path |
| Peer traffic-selector payload through the existing narrowing handler | → | Wire normalization, contextual admission and `selectorPort` | `TestSelectorAdmissionRejectsInvalidFormWithoutWidening` in engine selector tests |
| Supported group selected by initiator setup, followed by a malformed peer public value | → | `newInitiatorSA`, `NewDHExchange`, `SharedSecret` | `TestDHAdmissionPreservesPeerRejection` in engine tests |
| Loaded YANG command definition and actual argument-validation entry | → | `yangTypeToArgDef`, validated argument construction and `ValidateArgString` | `TestYANGArgumentAdmissionPreservesValidation` in config/yang tests |
| Operator starts and reloads an IPsec peer | → | SDK verification, staging, apply, peer reconciliation and visible state | `test/ipsec/ipsec-peer-reload-applies-selectors.ci`, `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci`, plus new `test/ipsec/ipsec-rejected-candidate-preserves-state.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Census over all first-party declarations, anonymous invariant-bearing aggregates and supported build populations | Every item has one justified disposition; every migrate row names its producers, mutation/alias paths, consumers, tests and documentation. Reconciliation accounts for later additions/removals |
| AC-2 | Caller attempts a nonzero invalid domain construction through a field, literal or scalar cast | The intended restricted operation fails to type-check where Go can exclude it. Constructors accept all preserved valid inputs and reject invalid boundary inputs without publishing a value |
| AC-3 | An aggregate with incompatible payloads or wrong lifecycle operation | The public/internal consumer API cannot express the incompatible combination or call the operation on the wrong state type. Same-package construction remains audited and tested |
| AC-4 | Zero, new-created, nil, typed-nil and embedded-interface values | Each follows its recorded contract with no unintended side effect. Existing valid zeros and sentinels remain valid. Tests do not claim that Go prevents constructing an exported opaque zero or retaining an alias |
| AC-5 | Invalid update or late decoder error on a previously valid object | Error/refusal leaves the prior object's observable fields, nested data and published state unchanged; no partially validated replacement is exposed |
| AC-6 | Caller mutates retained constructor inputs or accessor-returned storage | Validated state cannot be changed outside its controlled mutation contract. Tests include nested slices/maps/pointers and pool lifetime where present |
| AC-7 | Old pointer/value alias used after clear, consume or transition; repeated transition | The declared live-resource contract is enforced through the shared owner or the value remains a valid immutable snapshot. No use-after-clear, duplicate apply or stale privilege succeeds |
| AC-8 | New peer input, unknown wire identifier, changed environment or authorization state | Existing runtime validation/refusal remains reachable and non-panicking. No class-C claim follows solely from constructor naming or a narrowed local state type |
| AC-9 | Candidate config passes validation and is staged | Staging accepts the exact validated candidate with owned data; it cannot accept raw/merely parsed config. The same delivery is not reparsed merely to recover the object validation discarded |
| AC-10 | Candidate config fails validation or apply has no successful stage | No candidate PKI/config is published, no peers are reconciled for the rejected candidate, and absent-stage apply returns its existing refusal. Existing startup/reload/rollback semantics remain intact |
| AC-11 | Selector ANY/OPAQUE/single endpoints, range input, protocol zero and invalid form | Preserved valid encodings/narrowing remain exact. Invalid forms never become installed ANY. Config numeric zero stays rejected; wire single zero retains its existing normalization and contextual checks; OPAQUE support follows the separate owning plan |
| AC-12 | DH exchange construction, access, copying and clear | Group and key implementation cannot drift through caller mutation; supported exchanges still agree, unsupported groups and malformed peer values retain errors, and SharedSecret retains its safe refusal for cleared or nil exchanges through HasPrivate. Clear retains its non-nil receiver precondition |
| AC-13 | Command metadata lowering and all manual producers | All published definitions meet kind/payload invariants; range/default/union ordering and rendered metadata remain unchanged for valid definitions. No hand-maintained vocabulary replaces YANG or registries |
| AC-14 | Repeated consumer-side validation removed as an optimization | Its census row proves all producer, decode, update, alias and lifetime paths. If proof is incomplete the check remains; wire and runtime obligations are not counted as redundant |
| AC-15 | A representation change touches a hot path | Before/after evidence names the entry point, toolchain/build and allocs/op, bytes/op and ownership lifetime. Zero-allocation paths remain zero and no avoidable copying/boxing is introduced; cold required copies are justified individually |
| AC-16 | Final migration tree | Every in-scope row and caller is migrated, obsolete APIs/aliases/comments are removed, tests use real constructors except intentional raw/internal-negative tests, and the build/tag/platform population compiles |
| AC-17 | Rules, guide and owning architecture pages | One authoritative policy remains, new-code applicability is distinguished from approved legacy scope, Go limits are accurate, and every changed invariant/data flow has updated source-anchored documentation |
| AC-18 | Tests and completion report | Runtime tests reach actual admission/update/publication consumers, compile-negative probes fail for the expected type error with positive controls, no permanent source-text/forwarding-only test is introduced, and only observed checks are reported |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Commits a valid selector edit and then an invalid candidate | CLI/config transaction → IKE verify → validated stage → apply/reject → visible peer and PKI state | `ipsec-peer-reload-applies-selectors.ci` and `ipsec-rejected-candidate-preserves-state.ci` |
| 2 | Connects a peer with valid keys/selectors, then supplies malformed negotiation input | Receive handler → existing wire parser → domain admission → safe refusal, unchanged installed policy | `TestDHAdmissionPreservesPeerRejection`, `TestSelectorAdmissionRejectsInvalidFormWithoutWidening` and the existing IPsec establishment suite |
| 3 | Executes a command with a valid or out-of-range argument | YANG command tree → admitted argument definition → command argument validation | `TestYANGArgumentAdmissionPreservesValidation` and existing command functional carriers selected by the census |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File / carrier | Validates | Status |
|------|----------------|-----------|--------|
| `TestValidatedConstructionAPI` | Temporary external-package compiler fixtures for each migrated package, retained as implementation evidence | Private-field writes/nonzero literals, scalar forgery, wrong-state calls and passing parsed config to validated staging fail for the intended reason; matching valid use compiles | Planned |
| `TestValidatedZeroAndNilContracts` | Owning package external tests, including typed nil only for retained interface APIs | AC-4; zero construction compiles and runtime behavior follows the recorded contract | Planned per census row |
| `TestValidatedUpdateFailureIsAtomic` | Owning package test beside each mutator/decoder | AC-5; valid first fields plus invalid last field cannot alter old value or published data | Planned per census row |
| `TestValidatedOwnershipPreventsAliasMutation` | Owning package tests | AC-6; mutate retained inputs, returned data, nested members and copied resource handles | Planned per census row |
| `TestValidatedTransitionRejectsStaleAlias` | Owning lifecycle package tests | AC-7; transition and then use old aliases, clear twice, or apply twice as the existing contract requires | Planned per census row |
| `TestIKEConfigAdmissionPublishesOnlyValidatedCandidate`, `TestIKERejectedCandidateLeavesLiveStateUnchanged` | `internal/component/ike/engine/config_verify_test.go` and registered-callback fixture | AC-9/10; inspect live config/PKI and apply effects, not a call counter alone | Planned |
| `TestValidateIPsecSectionsDoesNotMutatePKIStore`, `TestIKEConfigApplyWithoutVerifyIsRefused` | Existing engine config verification and reconciliation tests | Preserve side-effect-free validation and missing-stage refusal | Existing, not run for this draft |
| `TestPortSelectorWireRoundTrip`, `TestSelectorAdmissionRejectsInvalidFormWithoutWidening` | Existing `ipsec/traffic_selector_test.go`; engine selector tests | AC-11, including accepted controls and invalid-form admission | Existing plus planned extension |
| `TestDHUnsupportedGroup`, `TestDHInvalidPublicKey`, `TestDHAdmissionPreservesPeerRejection` | Existing `crypto/dh_test.go`; engine admission test | AC-12; correct-length invalid MODP values reach the range guard | Existing plus planned extension |
| `TestYANGArgumentAdmissionPreservesValidation` | `internal/component/config/yang/command_test.go` | Real lowering and validation of each supported kind, union order and defaults | Planned |
| `BenchmarkValidatedAdmission` and `BenchmarkValidatedUse` | Owning package benchmark tests for changed hot paths | Separate one-time construction cost from repeated operation cost and allocations; compare the same inputs before/after | Planned per changed hot path |

Compiler fixtures belong inside an import-legal first-party location so Go's `internal` restriction cannot be the reason for failure. Keep one forbidden operation per case, first compile the positive control under the same module/tags, and record the specific error. A test-only foreign package cannot prove same-package privacy. Interface embedding, opaque zero construction and retaining an old alias are compile-positive controls; their safety belongs in runtime tests. These probes are one-shot evidence, not a new permanent compiler/linter harness or source-text assertion.

### Boundary Tests

| Field / condition | Valid case | Invalid boundary or distinct context | Required result |
|-------------------|------------|--------------------------------------|-----------------|
| Config numeric port | 1 and 65535 | 0, negative token, 65536, malformed token | Preserve config rejection and accepted spellings |
| Wire port pair | ANY 0/65535, OPAQUE 65535/0, equal endpoints including 0/0 | Ordered non-single range and inverted non-OPAQUE range | Preserve supported normalization, legitimate first-port narrowing and invalid-range refusal at the correct boundary |
| Port form plus protocol | Single port with nonzero supported protocol, ANY with protocol zero | Single/opaque with protocol zero; unnamed form | Preserve contextual refusal; no widened installation |
| DH input | Every supported group's valid peer value | Unsupported group; length one below/above; correct-length MODP 0, 1 and p-1; invalid curve point | Existing specific errors; no panic or derived secret |
| Argument metadata/value | Existing widths, width zero default, each range endpoint and each union member | Invalid metadata combination, value below/above range, no accepted union member | Invalid metadata cannot publish; values retain current validation outcome |
| Candidate references | All references resolved against candidate data | One missing group/certificate with otherwise valid candidate PKI | Reject with old live state untouched |
| Mutator/decode | Fully valid replacement | Error after partial parse; nil/zero replacement; retained mutable alias | Preserve previous object and no partial publication |

### Functional Tests

| Test | Location | End-user scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-traffic-selector-config` | `test/ipsec/ipsec-traffic-selector-config.ci` | Accepted policy reaches the running engine | Existing; preserve and strengthen only where needed |
| `ipsec-peer-reload-applies-selectors` | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | Selector edit reaches the replacement peer | Existing; not run |
| `ipsec-peer-reload-leaves-tunnel-alone` | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | Unchanged config does not restart a tunnel | Existing; not run |
| `ipsec-rejected-candidate-preserves-state` | New `test/ipsec/ipsec-rejected-candidate-preserves-state.ci` | Reject invalid candidate, then observe prior peer/config and PKI state through normal commands/events | Planned |
| Census-selected affected entry points | Existing subsystem `.ci` files, named per migration row before editing that row | Every changed runtime boundary remains reachable from its public operation | Required for every migrated runtime family |

### Interop Tests

No new protocol behavior is proposed, so no new interop scenario or RFC support claim is required. Reuse the existing IKE establishment, rekey and selector scenarios for changed wire-path consumers. If the census identifies a wire-visible semantic change, return that choice to design and its owning protocol spec before implementation; do not relabel it as encapsulation.

## Files to Modify

- `internal/component/ike/crypto/dh.go`: encapsulate exchange consistency and alias/clear ownership without changing algorithms; migrate its resolved callers and tests.
- `internal/component/ike/ipsec/traffic_selector.go`: separate normalized ports from context-validated selectors.
- `internal/component/ike/ipsec/types.go`: distinguish parsed and validated aggregates and control mutable backing data.
- `internal/component/ike/ipsec/config.go`: construct parsed candidates without asserting that parsing proves cross-reference validity.
- `internal/component/ike/engine/config.go`: return the exact validated candidate from the existing validation chain.
- `internal/component/ike/engine/register.go`: require the validated candidate at staging and preserve startup/reload callback contracts.
- `internal/component/ike/engine/initiator.go`: consume encapsulated exchanges and validated proposal data.
- `internal/component/ike/engine/child.go`: consume context-validated selector data at installation.
- `internal/component/ike/engine/ts_narrow.go`: preserve wire normalization, intersection and contextual rejection through the narrowed types.
- `internal/component/command/node.go`: represent argument kind/payload invariants at metadata publication.
- `internal/component/command/argvalidate.go`: consume admitted definitions while preserving valid argument outcomes.
- `internal/component/config/yang/command.go`: construct admitted definitions from YANG instead of publishing mutable incompatible payloads.
- All validators, construction sites and consumers resolved from these symbols, including tests: migrate the complete producer-to-consumer path in the same batch.
- All additional first-party migrate rows produced by the census: complete the same cutover in dependency order. This population is part of the deliverable, not optional follow-up work.
- Owning package tests and existing functional carriers listed above: prove invalid-input/update/alias behavior and preserve accepted cases.
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md`: document exchange ownership.
- `docs/architecture/ike/ipsec-3-data-model.md`: document parsed/validated config and owned aggregates.
- `docs/architecture/ike/ipsec-7-ikev2-engine.md`: document validated-candidate staging.
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`: document validated selector installation.
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`: document normalized versus programmable selector admission.
- `docs/architecture/api/commands.md`: document validated argument publication and preserved external behavior.
- `docs/architecture/config/yang-config-design.md`: preserve the documented independent help-text contract while updating command metadata admission.
- Other owning architecture pages derived by the census: update each changed contract during its migration phase.

## Files to Create

- `plan/spec-validated-construction-and-state-types.md`: this design draft and later retained census/decision record.
- `test/ipsec/ipsec-rejected-candidate-preserves-state.ci`: future functional rejection/publication proof.
- Owning-package test files only when no existing file fits the census row: named behavioral and allocation tests from the Test Plan; no generic framework.

No runtime source, test or functional fixture is created by the current drafting assignment. Prefer existing production files during the approved migration; new production files require a concrete owner and responsibility recorded in the census before editing.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No syntax or schema extension; `internal/component/config/yang/command.go` lowering changes behind existing declarations |
| YANG validation constraints | No | Preserve existing native constraints; constructors carry their established result inward |
| YANG custom validators | Yes | Preserve existing callback chains, including `engine/config.go` `validateIPsecSections`; adapt return/consumer types without adding a second validator registry |
| CLI commands/flags | No | No command/flag added; existing validation and rendering callers migrate |
| CLI grammar | No | Existing keyword/value grammar remains unchanged |
| Editor autocomplete | Yes | ArgDef representation affects shared metadata consumers; preserve completion values/order and read-only publication through census-resolved callers |
| Functional test for new RPC/API | N-A | No new RPC; existing entry-point regressions plus the new rejected-candidate `.ci` cover internal API changes |
| Pipe completeness | No | Command output and existing pipe routing remain unchanged |
| Env var registration | N-A | No environment variable |
| Doctor check for runtime dependencies | N-A | No new dependency; retain current host/interface checks at their runtime boundary |
| Prometheus counters/metrics | No | No metric or label added; state-type changes preserve existing consumers |
| BGP family surface | N-A | No SAFI, capability or attribute support is introduced; census migration preserves existing families |

### Documentation Update Checklist

| # | Question | Applies? | File to update or preservation decision |
|---|----------|----------|-----------------------------------------|
| 1 | New user-facing feature? | No | `docs/features.md` gains no feature or support claim |
| 2 | Config syntax changed? | No | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` syntax stays unchanged; no example rewrite for a type-only change |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` grammar/output remains unchanged |
| 4 | API/RPC added/changed? | Yes, internal metadata contract only | `docs/architecture/api/commands.md`: describe validated argument publication and the unchanged external RPC contract |
| 5 | Plugin added/changed? | No new plugin | `docs/guide/plugins.md` lifecycle contract stays unchanged; IKE callback internals are documented on its architecture page |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`: verify supported config examples against preserved parser behavior; change only a stale claim or source anchor, not advertised support |
| 7 | Wire format changed? | No | Existing IKE wire format and borrowed-buffer rules remain; no encoding support claim |
| 8 | Plugin SDK/protocol changed? | No | `ai/rules/plugins.md` and `docs/architecture/api/process-protocol.md` external protocol remains unchanged; raw SDK DTOs remain valid boundary types |
| 9 | RFC behavior implemented, changed, or newly proven? | No support expansion | `rfc/short/rfc7296.md` and generated `docs/features/rfc-status.md` Support rows stay unchanged. Update source/test anchors only if a migrated enforcing carrier moves |
| 10 | Test infrastructure changed? | No | Use existing test/compiler/functional routes; no new framework or gate, so `docs/functional-tests.md` needs no new command |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` has no capability change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-6-ikev2-crypto.md`: exchange ownership; `ipsec-3-data-model.md`: parsed/validated config; `ipsec-7-ikev2-engine.md`: candidate staging; `ipsec-8-ikev2-child-xfrm.md` and `rfcgate-1b-rfc7296-pilot.md`: validated selector flow; command metadata page as row 4; `docs/architecture/config/yang-config-design.md`: command metadata admission with the independent help-text contract preserved |
| 13 | Route metadata keys added/changed? | No | No keys or metadata registry changes |
| 14 | Prometheus counters added/changed? | No | No telemetry contract changes |
| 15 | Registered plugin/event/send/command/capability/inventory changed? | No new registered item | Keep YANG/registry derivation; no catalog entry for an internal representation |
| 16 | Changed source referenced by doc anchors? | Yes | `ai/CODE-TO-DOCS.md` identifies the IKE and command pages above plus `docs/guide/ipsec.md`. Every census row derives its full anchor/Design set before edits using `./le spec citation anchors spec plan/spec-validated-construction-and-state-types.md`; record unchanged pages with reasons. This command has not run for this draft |
| 17 | Existing config/CLI/API examples? | Yes | Check `docs/guide/ipsec.md`, `docs/guide/configuration.md` and `docs/architecture/api/commands.md` against preserved parser/handler behavior; do not invent new syntax |

The accompanying rule/style correction is owned by the rule-update assignment: canonical point and Go-rule manifest, plus `docs/contributing/ze-go-style.md`. Future implementation references that policy rather than copying it. Regeneration and documentation checks belong to the parent integration step.

### Discovery and Maintenance

| Mechanical question | Decision |
|---------------------|----------|
| Where an agent looks first | Existing `ai/INDEX.md` Every line of Go row → Go standards and style guide; `plan/README.md` locates this future migration |
| What prevents regression | Canonical construction point plus package APIs, compiler proofs and behavioral tests; no blanket constructor-name check |
| What prevents inventory drift | The temporary type-aware census reconciled against final first-party source and existing feature/build manifests; no second permanent registry |
| What verification proves it | AC-1/16 inventory reconciliation, compile-negative/positive evidence, entry-point tests, allocation evidence and the native final verification matrix |
| Generated discovery | Parent renders rules and their condensed/index outputs from canonical points; implementation updates source-to-doc indexes only when source/page mappings change |

## Implementation Steps

These steps are contingent on owner approval. Each package migration is complete across producers, consumers, tests and docs before the next dependent package changes.

1. **Phase: Wiring and census.** Enumerate the full source population, record every disposition and all mutation/ownership paths, and bind each migration to its actual entry point. Add the named failing behavioral/API proofs for the first dependency batch, with valid controls. No production stub or alternate handler is needed: existing callbacks and consumers are the wiring. Append the complete source/doc/test list to this spec before source edits. Reconcile the enum cutover and overlapping plans.
2. **Phase: Leaf values and variants.** Close scalar and payload-combination construction paths first, including normalized selectors and command argument payloads. Migrate every constructor, literal, decoder, test fixture and caller for that type in the same batch. Preserve zero/sentinel and wire behavior; publish allocation evidence for affected hot paths.
3. **Phase: Validated aggregates and publication.** Construct owned validated snapshots, including the IKE candidate. Make staging and downstream consumers accept only those values. Preserve raw external DTOs at decode boundaries, remove redundant reparsing, and prove invalid updates never partially publish. Update data-model and callback docs during this phase.
4. **Phase: Controlled lifecycles and resources.** Apply state-specific APIs where operation ordering is Ze-controlled. Establish shared ownership or immutable snapshot semantics for remaining aliases, including DH clear/use and staged apply. Retain peer, environment and revocation checks. Complete stale-alias and failed-transition tests.
5. **Phase: Remaining census batches.** Complete every remaining first-party migrate row in dependency order, using the same leaf → aggregate → lifecycle sequence. Record already-correct/open/runtime/unconstrained exclusions with sources. This phase cannot end at the representative examples.
6. **Phase: Reconciliation and integration.** Reconcile declarations and all build populations again, remove obsolete APIs and temporary analysis/probe files, and verify documentation mappings. Run the native tests, allocation checks, lint/staticcheck and final worktree verification once after all edits have landed. An independent review checks complete producer closure and residual runtime enforcement before closure.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Population | No type disappeared from the census because it had no enum, lived under a platform tag, or was used only by tests/examples |
| Construction proof | Zero/literal/cast/decode/copy and all field/setter paths are included; private fields are not treated as immutable backing data |
| Validity boundary | Raw and parsed inputs cannot reach validated consumers without the intended checks; admission errors do not publish success values |
| Lifetime | Typed nil, embedding and stale aliases have concrete contracts, and peer FSM checks remain runtime checks |
| Preservation | Empty config bypass, valid enum zeros, command width default, selector range narrowing and current algorithm/OPAQUE support remain intact |
| Optimization | A removed internal check has a producer/ownership proof; no asserted allocation improvement lacks measurement |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Complete source-based migration record | Reconciled type-aware census and LSP references across the declared build population, retained in this spec at implementation |
| Restricted APIs with valid behavior | Positive and negative compiler probes through the Go toolchain under the same module/tags; expected diagnostic recorded |
| Boundary/update/lifetime enforcement | Named unit tests through native `./le test unit all` and affected entry-point functional suites |
| Preserved IPsec operation | `./le test functional ipsec`; kernel-dependent coverage through `./le test qemu netns-test suites ipsec` where required |
| No avoidable allocation regression | Named before/after benchmarks with `-benchmem` under the same native toolchain/build and ownership/lifetime tests |
| Build populations and docs | `./le go lint run`, `./le go staticcheck check`, `./le doc check verify`, `./le verify worktree`; parent records exercised scope, failures and skips |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Untrusted input | Malformed peer/config/plugin input cannot become a BUG panic or an installed permissive default |
| Publication | Rejected candidates leave live PKI, policy, peers and shared metadata unchanged |
| Secret ownership | Private/public exchange consistency, clearing across aliases and no avoidable key copies; no secure-erasure claim |
| Privilege lifetime | Validation never freezes a mutable authorization, revocation or host-availability verdict |
| Resource exhaustion | No per-event validation framework, reflection walk, allocation or unbounded copy introduced |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Missing caller, build-tag compile error or stale doc | Complete that migration row before continuing |
| Compiler fixture fails for import/setup reasons | Repair its positive control; it is not API proof |
| Runtime test rejects before the intended guard | Repair the test's valid prefix and assert the intended outcome |
| Representation requires changed valid-zero/sentinel/public semantics | Return to owner design approval; do not silently reinterpret the contract |
| Overlap changes algorithms or selector support | Reconcile with its owning plan before editing the producer |
| Allocation/ownership requirement cannot be met | Reconsider the concrete API; do not hide the cost behind a wrapper or waive the invariant |
| Reproduced unrelated operator defect | Route under the repository's defect workflow; do not quietly expand this migration |

## Design Insights

The source already contains useful boundary separation: candidate PKI validation is side-effect free, raw wire IDs can be rejected safely, and private local categories can come from finite tables. The migration should preserve those mechanisms and make their guarantees survive storage and mutation. Type-state narrows an API; resource validity still depends on the owner of shared state.

## Key Design Decisions

| Decision | Alternatives considered | Rationale |
|----------|-------------------------|-----------|
| Full census, then complete dependency-ordered migration | A small IKE pilot; immediate repository-wide constructor rewrite | This draft recommends the full census and migration so the design addresses invariant-bearing types consistently; the owner has not approved that scope. A blind rewrite cannot distinguish raw DTOs from validated values |
| Package-owned concrete values and specific variants | NewX naming alone; blanket interface wrappers; generic validation engine | Concrete representations close actual invalid combinations without adding a second validation system |
| Distinct parsed/validated candidate | Validation returns only error and the caller reparses | Carrying the validated object binds proof to the exact data staged and avoids duplicate parsing |
| Shared runtime ownership where aliases survive | Claims that returning a new type consumes the old value | Go has no affine or move semantics; all copies cannot be revoked by a signature |
| Explicit valid-zero or safe-invalid-zero contracts | Making every old enum/type zero invalid | Existing valid zeros and sentinels are observable contracts, and opaque exported structs still have zeros |
| Existing checks until closure is proven | Removing checks as soon as a constructor exists | Mutations, decoders and mutable references can invalidate a constructor's result |

### Design Challenges

| Challenge | Answer |
|-----------|--------|
| Simplicity | The minimum mechanism is a package-owned representation and its real admission boundary. The complete census changes scope, not machinery |
| Uniformity | Extend the existing private-field, finite-producer and side-effect-free validation patterns; use existing registries and test routes |
| Performance | Construction runs once per admitted value; unchanged runtime facts remain checked. Zero-copy access uses existing owners/pools. Any required cold copy and every changed hot path get measured evidence |

## Known Limitations

- This design does not prevent `unsafe` or deliberate reflective violation of package privacy. Ordinary first-party decoder/reflection paths remain in the census and need safe admission.
- Go cannot prevent creation of every invalid zero, typed nil or stale alias. The API states which cases compile and which require runtime refusal.
- External raw data and peer-controlled FSM states remain open representations. This is a necessary boundary model, not an incomplete migration row.
- No new RFC support, algorithm, selector capability or public command is delivered by this spec. The related feature plans retain those scopes.
- The full concrete migration inventory is an output of the ordered census, not a claim made by these representative source reads. Completion requires every in-scope row; unresolved semantic choices return to the owner gate.

## RFC Documentation

Preserve existing requirement comments and tests on enforcing behavior. When moving a guard or type changes a requirement's source anchor, update the existing summary/test reference without changing the quoted requirement or claiming new support. This draft changes no RFC metadata.

## Review Gate

No review run is claimed by this design draft. The independent implementation review records its scope and covering artifact before closure; the run and finding tables remain empty.

| Run | Scope | Reviewer lenses | Verdict | Artifact |
|-----|-------|-----------------|---------|----------|

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

### Pre-Spec Verification

- [ ] Metadata identifies a design draft and distinguishes approval from completeness.
- [ ] Required reading and source producers establish the selected examples.
- [ ] Full census scope, exclusions and completion rule are explicit.
- [ ] Every acceptance criterion has a compiler, behavioral, ownership, allocation or source-reconciliation proof.
- [ ] Integration and documentation rows name concrete changes or justified preservation.
- [ ] No code snippets or completed-review claims appear in this spec.

### Goal Gates

- [ ] Owner approves migration design and implementation scope separately from drafting.
- [ ] AC-1 through AC-18 are demonstrated over the complete reconciled population.
- [ ] All migration callers, tests and owning docs are updated; obsolete APIs are removed.
- [ ] Native build/test/lint/doc checks and allocation evidence name observed outcomes and skips.
- [ ] `./le verify worktree` and the independent review cover the implementation.
- [ ] All assumptions are resolved or explicitly returned to design before closure.

### TDD

- [ ] Tests written.
- [ ] Tests FAIL (paste output).
- [ ] Tests PASS (paste output).
- [ ] Compiler probes have valid controls and expected type errors.
- [ ] Runtime invalid-input, failed-update, stale-alias and no-partial-publication tests discriminate the changed behavior.
- [ ] Functional tests exercise actual public entry points.
- [ ] Preserved accepted cases and allocation-sensitive paths have direct evidence.

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` at closure and complete its implementation/audit sections without inventing review runs.
- [ ] Record the independent covering review through the existing review command.
- [ ] Route lessons to governing rules/docs and repoint live citations before spec removal.
- [ ] Commit A contains the complete implementation, tests, docs and edited spec; commit B removes only this spec in the same approved commit workflow.
