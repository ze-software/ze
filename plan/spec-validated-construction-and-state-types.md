# Spec: validated-construction-and-state-types

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling, config, protocol |
| Depends | Phase 1: `plan/pre-release/spec-config-yang-loader-structural-checks.md` lands first, and the goyang fork `replace` in `go.mod` (enum numbering fix, another agent, 2026-10-09) lands before step 1. Later phases: reconcile the completed enum migration before changing its producers; coordinate overlapping IKE plans below |
| Phase | Phase 1 (YANG resolved schema, strict `DefaultLoader`, validated command arguments, `ArgDef` construction) authorized by the owner on 2026-10-08; design decisions D-1 to D-7 answered by the owner on 2026-10-09. Later phases not authorized |
| Handoff | `plan/handover/12-validated-construction-phase-1.md` |
| Updated | 2026-10-09 |

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

## Phase 1 (owner-authorized 2026-10-08)

### Owner decision, 2026-10-08

> run the state-type refactor as the spec's first phase

The owner authorized a first phase of three concrete migrations. Each is a type that would have made a defect fixed on 2026-10-08 impossible to write. Everything else in this spec (the full census, the IKE families, the remaining Implementation Steps) stays a later phase, unchanged and not authorized by this decision. This section is the Phase 1 design. A design agent wrote it on 2026-10-08; the owner answered its decisions D-1 to D-7 on 2026-10-09 ("Decisions" below), and the design was reworked to match the same day.

→ Decision (2026-10-08, superseded): Status stayed `design` until the owner answered the open decisions.

→ Decision (2026-10-09): Status is `ready`. The owner answered D-1 to D-7 and confirmed D-6, which were the design approval this phase waited for. Resume state for the batches lives in the committed handover `plan/handover/12-validated-construction-phase-1.md`, never under `tmp/`, because the owner moves machines.

→ Constraint: Phase 1 implementation starts only after `plan/pre-release/spec-config-yang-loader-structural-checks.md` lands. That work is editing `internal/component/config/yang/loader.go`, adding `loader_structure.go`, `loader_structure_test.go` and `loader_rfc7950_structural_test.go`, removing `loader_rfc7950_structural_red_test.go`, and names `validator.go` and `loader_rfc7950_test.go`. Phase 1 changes `loader.go` and `validator.go` throughout.

→ Constraint: Phase 1 implementation starts only after another agent's goyang fork lands: `go.mod` gains a `replace` of `github.com/openconfig/goyang` (today `v1.6.3`) with a ze-software fork carrying the enum numbering fix (journal `plan/journal/zero-value-as-valid-answer.md`, rows of 2026-10-09: goyang `EnumType.SetNext` and `Set` count from -1, so `Modules.Process` refuses a valid `enum p { value -5; } enum q; enum r { value 0; }`, an error the `DefaultLoader` path discards today). Shared files: `go.mod`, `go.sum`, `vendor/modules.txt`, the vendored goyang tree `vendor/github.com/openconfig/goyang/` (`pkg/yang/types_builtin.go` and whatever else the fork changes), and on the Ze side `internal/component/config/yang/enum_assignment.go` and `enumRestrictionErrors` in `loader_structure.go`, which read the enum values. The ordering matters for D-6: once `DefaultLoader` stops discarding `process` errors, a valid enumeration goyang misnumbers would fail startup, so the fork must be in before step 3 (strict loader) can land.

### Phase 1 scope

| Type | Defect that motivates it | Target |
|------|--------------------------|--------|
| T1 YANG resolved schema | `yang.Loader` is one type before and after `Resolve`. Nine callers kept using a loader whose `DefaultLoader` or `Resolve` had failed: nil dereferences and silently empty answers. Fixed by threading errors only (ac6c5ce12d, a5b3180063, 40db22a585). `applyPatterns` in `internal/component/config/yang/command.go` still carries a `BUG` panic for "a pattern Loader.Resolve refuses", guarding a state the type allows | `Resolve` returns a distinct resolved type. `BuildCommandTree` and every consumer that needs a checked module set accept only that type, so building from an unchecked or failed loader does not compile |
| T2 Validated command arguments | Handlers take raw token slices. Three dispatch routes ran handlers with no argument validation; 40db22a585 added `command.ValidateArgs` on each (daemon dispatcher, `command.ServeLocal`, the plain local handler `registry.RegisterLocalData` builds) | `ValidateArgs` is the only producer of a validated-arguments type, and every author-facing handler type accepts only it (D-2 A), so a route that skips validation, or code that calls a handler with raw tokens, does not compile |
| T4 Strict `DefaultLoader` (D-6) | `DefaultLoader` discards the errors of `LoadRegistered` and `process` (`loader.go`, two `_ =` lines). `LoadRegistered` also stops at the first parse error, so every module registered after the failing one is skipped in silence. A module set missing an imported module therefore yields a loader whose tree walk skips the unresolved modules, and every caller sees success | `DefaultLoader` returns every `LoadRegistered` and `process` error, joined with the checks, and returns no value when any is present. `LoadRegistered` loads every module and joins every parse error instead of stopping at the first |
| T3 `command.ArgDef` construction | Every field is exported, including `Lengths` and `Patterns`, so any package can build a definition whose ranges overlap or descend, or whose kind and payload disagree. Overlaps AC-13 | A validating constructor, private invariant-bearing fields, accessors that expose no mutable slice. The YANG lowering and every manual producer go through it |

| Excluded from Phase 1 | Reason |
|-----------------------|--------|
| `command.Node` field encapsulation (`ArgDefs`, `Children`, ...) | The per-`ArgDef` invariant survives replacing a slice element with another constructed definition; `Node` has no cross-field invariant this phase needs. Recorded as an alias path, later phase |
| Plugin SDK handler types in `pkg/plugin/` (`sdk.ExecuteCommandHandler`, `rpc.DispatchCommandArgsHandler`, `rpc.ExecuteCommandHandler`) | Plugin API contract external authors compile against. The engine validates before forwarding (route R9 below); the plugin side keeps raw tokens |
| `registry.RootHandler`, `plugin/registry.Registration.CLIHandler`, the `internal/appliance` handler | Their arguments are process flags the handler parses, not YANG leaves; no `ArgDef` describes them. Step 1 re-checks each: a root handler that forwards into R1 to R9 is covered there |
| goyang entry immutability | `GetEntry` and `GetModule` return goyang pointers shared with the module set. Copying the tree per call breaks the zero-copy constraint; Phase 1 records the read-only contract and censuses writes (A-8) |
| IKE families, the full-population census, AC-1, AC-5, AC-8 to AC-12, AC-14, AC-15 | Later phases |

### Phase 1 census

Counts are from `gopls references` over the host build (scratch files `ref-*.txt` and `argref-*.txt` under `tmp/session/2026-10-08-01b175b6-ee3c-45d9-a884-71d61adff15a/scratch/`) and, where marked, from a grep that includes declaration lines. Step 1 reconciles them across build tags and platforms before any edit (A-2).

#### T1 `yang.Loader` (`internal/component/config/yang/loader.go`)

| Census field | Content |
|--------------|---------|
| Invariant | A module set whose extension statements are all declared (`checkExtensions`), whose patterns all compile through `compilePattern` (`checkPatterns`), and whose structures pass `checkStructure` (`loader_structure.go`, in flight). Import resolution (`process`) is strict in `Resolve` and best-effort in `DefaultLoader` |
| Producers | `NewLoader` (126 references: 12 production, 114 test); `DefaultLoader` (63: 22 production, 41 test); `(*Loader).Resolve` (122: 11 production, 111 test). 36 production loader-producing call sites in 25 files (grep). Three resolution policies exist: `Resolve` (strict `process` plus the checks), `DefaultLoader` (discards the `LoadRegistered` and `process` errors, then the checks), and `internal/le/doc/yangcontract/usage.go`, which refuses only `ErrUndeclaredExtension` and then builds a command tree from a loader whose pattern or structure check may have failed |
| Mutation and alias paths | `AddModuleFromText`, `AddModuleFromFile`, `LoadEmbedded` and `LoadRegistered` stay callable after `Resolve`, on the same goyang module set a resolved consumer reads. `GetModule` and `GetEntry` hand out shared goyang pointers. `Validator` (`validator.go`) and the CLI completer (`internal/component/cli/completer.go`) store a loader; the help-shape input in `internal/le/doc/yangcontract/helpshape.go` carries a `Loader` field |
| Consumers | Type `Loader`: 78 references (50 production, 28 test) in 28 packages. In package yang: `BuildCommandTree` (56 calls, 17 production), `WireMethodToPaths`, `WireMethodToPath`, `PathToDescription`, `PathToHelp`, `PathToTaskSupport`, `PathToArgDefs`, `PathToUIResource`, `PublishedRPCs`, `ExtractRPCs`, `ExtractNotifications`, `NewValidator`, `CheckAllValidatorsRegistered`, the private `moduleNames`, `GetEntry` (76 references, 23 production), `GetModule`, `ModuleNames`, `ConfModuleNames`, `APIModuleNames`. Outside: `internal/component/config/yang_schema.go` (`loadYANGModules`, `PluginOnlySchema`), `internal/component/aihelp/aihelp.go`, `internal/le/doc/yangcontract/` (`usage.go`, `helpshape.go`, `helpshape_schema.go`, `contract.go`), `internal/le/cli/dispatch/resolver.go`, `internal/le/cli/grammar/cligrammar.go`, `internal/le/cli/list/commandlist.go`, `internal/le/config/claims/configclaims.go`, `internal/le/arch/enumeration/corpus.go`, `internal/exabgp/migration/schema.go`, `internal/component/config/schema/cli/main.go`, `internal/component/config/cli/cmd_edit.go`, `internal/component/config/infra/authz.go`, `internal/component/config/yang/cli/tree.go`, `internal/component/cli/` (`client/main.go`, `completer.go`, `validator.go`, `testing/headless.go`), `internal/component/iface/validate.go`, `internal/component/plugin/server/server.go`, `cmd/ze/hub/` (`command_meta.go`, `service_web.go`, `session_factory.go`), and the plugin YANG self-tests that call `GetEntry` |
| Tests | 28 test references to the type, 111 to `Resolve`, 114 to `NewLoader`, 41 to `DefaultLoader`, across about 35 packages, including the extension tests ac6c5ce12d and a5b3180063 added |
| Docs | `docs/architecture/config/yang-config-design.md` (loader, `DefaultLoader`, command tree); `docs/contributing/ze-go-style.md`, "One type per lifecycle state", uses this case as its example |

#### T2 handler argument types

| Handler type | Declared in | Implementations (production) | Route that invokes it | Validated today |
|--------------|-------------|------------------------------|-----------------------|-----------------|
| `pluginserver.Handler` | `internal/component/plugin/server/command.go` | 167 named functions with the signature in 38 packages, plus closures; 369 `RPCRegistration` literal sites (grep) | R1 `Dispatcher.Dispatch` | Yes, `ValidateArgs` when the matched command declares definitions; the error is held and reported after authorization and the flag check |
| `pluginserver.Handler` | same | same | R4 `Server.wrapHandler` (`server.go`) | No. Its comment says nothing dispatches through the RPC dispatcher today, so the route is dormant and unvalidated |
| `pluginserver.Handler` as `EnsureStep.Handler` and `RollbackHandler` | `ensure.go` | ensure-chain steps | R5 `wrapWithEnsureChain`, which passes nil tokens | No tokens (A-9) |
| `registry.LocalDataHandler` | `internal/component/command/registry/registry.go` | about 30 `RegisterLocalData` sites (grep, declarations included) | R2 `command.ServeLocal` (`local_data.go`); R3 the plain handler `RegisterLocalData` builds | Yes, both, through `checkLocalArgs` and `checkLocalDataArgs`; both skip when the path declares no definitions |
| `registry.LocalHandler` | `registry.go` | about 50 `RegisterLocal`, `RegisterLocalMeta` and `RegisterOfflineFallback` sites (grep); 119 production functions shaped as a local or local-data handler | R6 `registry.LookupLocal` (`cmd/ze/ze_core_dispatch.go`, `cmd/ze/internal/cmdutil/cmdutil.go`); R7 `LookupOfflineFallback` (`cmdutil.go`, `internal/component/cli/client/main.go`) | No, except the handlers R3 wraps |
| `pluginserver.StreamingHandler` | `plugin/server/handler.go` | 5 (`monitor vpn ipsec`, `monitor event`, `monitor traffic stat`, `monitor interface rate`, `monitor system netlink`) | R8 SSH streaming (`cmd/ze/hub/service_ssh.go`) | No |
| Plugin-process commands | `RegisteredCommand` (`command_registry.go`) | external and internal plugins | R9 `dispatchPlugin`, `routeToProcess`, `ForwardToPlugin`, `dispatchSubsystem` | No engine-side validation; whether the merged model declares definitions for each is A-10 |
| `rib.CommandHandler` | `internal/component/bgp/plugins/rib/rib_commands.go` | rib sub-dispatch | Inside a plugin, after R1 or R9 | Inherits its caller's validation; recorded, not migrated |

| Census field | Content |
|--------------|---------|
| Invariant | Every token a handler receives was judged by `ValidateArgs` against the definitions the merged model declares for the matched path, including the empty set, and the lone positional binding it returned is the one the route acts on |
| Producer | `command.ValidateArgs` (`internal/component/command/argbind.go`), production callers `local_data.go` and `plugin/server/command.go`; 4 test calls. With no definitions it consumes nothing, refuses nothing and returns no binding: read in its positional phase, where the open-definition count is zero |
| Mutation and alias paths | The token slice is shared between route and handler; a handler may modify it; the route may read it after the handler for audit and selector reporting (census in step 1) |
| Tests | About 560 test lines in 68 test files call a handler directly with a literal token slice or nil (grep); `command_flag_test.go`, `argbind_test.go`, `local_data_test.go`, `registry/local_data_args_test.go`, `test/ui/cli-argument-length-refused.ci` |
| Docs | `docs/architecture/api/commands.md` (typed argument validation, the routes) |

##### T2 under D-2 A: every author-facing handler signature

Counts from greps over production `.go` files on 2026-10-09 (the declaration line or the registration call, closures included), under `tmp/session/2026-10-08-01b175b6-ee3c-45d9-a884-71d61adff15a/scratch/d2a-*.txt`. Step 1 reconciles them with `gopls references` across build tags (A-2).

| Handler type | Signature today | Production sites | Packages | Routes | Test call lines |
|--------------|-----------------|------------------|----------|--------|-----------------|
| `pluginserver.StreamingHandler` | `(ctx, *Server, io.Writer, username, args []string) error` | 5 handlers (`monitor vpn ipsec`, `monitor event`, `monitor traffic stat`, `monitor interface rate`, `monitor system netlink`) | `iface/cmd`, `trafficstat/cmd`, `ike/cmd`, `bgp/plugins/cmd/monitor`, `plugin/server` (declaration and registry), `cmd/ze/hub` (R8), `internal/component/ssh`, `internal/component/cli`, `internal/test/runner` | R8 | Step 1 |
| `registry.LocalDataHandler` | `(args []string) (any, int)` | 23 `RegisterLocalData` calls | `config/schema/cli` 5, `config/cli` 5, `plugins/env` 3, `component/plugin` 3, `le/plugin/imports` 2, `config/yang/cli` 2, `config/storage/cli` 2, `le/le/root` 1 | R2, R3 | Step 1 |
| `registry.LocalHandler` | `(args []string) int` | 8 `RegisterLocal`, 26 `RegisterLocalMeta`, 4 `RegisterOfflineFallback` calls (a call may register a table of handlers) | `plugins/debug` 8, `cmd/ze` 4, `config/cli` 3, `le/plugin/imports` 6, `traceroute/cmd` 2, `ping/cmd` 2, `bgp/cli` 2, `iface/cli`, `config/yang/cli`, `config/storage/cli`, `cmd/ze/internal/cmdutil`, `plugins/support`, `plugins/skills`, `plugins/explain`, `plugins/diag`, `component/doctor`, `plugins/host`, `plugins/crashes` | R6, R7 | Step 1 |
| `pluginserver.Handler` (also `EnsureStep.Handler`, `RollbackHandler`) | `(ctx *CommandContext, args []string) (*plugin.Response, error)` | 339 signature lines in 69 directories (named functions and closures) | Largest: `iface/cmd` 28, `cmd/show` 25, `plugins/ospf` 24, `plugin/server` 23, `l2tp/cmd` 20, `resolve/cmd` 18, `bgp/plugins/cmd/peer` 18, `bgp/plugins/cmd/rib` 12, `plugins/isis` 10, `plugins/meta/cmd` 9, `ike/cmd` 8; 58 more directories with 1 to 7 each (full list in `d2a-handler-dirs.txt`) | R1, R4, R5 | About 560 lines in 68 test files (2026-10-08 grep) |

| Census field | Content under D-2 A |
|--------------|---------------------|
| What changes in a handler | The parameter type only: `args []string` becomes the validated-arguments value. The body reads its tokens through one accessor, so the edit is mechanical: rename the parameter and read the tokens from it in the first statement, or replace each use. No handler changes what it does with its tokens, because `ValidateArgs` passes unmatched tokens through (A-7) |
| What changes in a test | A direct call with a literal slice or nil becomes a call with the value `command.ValidateArgs` returns over that slice and an empty definition list, which is exactly what a route produces for a path that declares nothing (A-7). No exported helper that wraps raw tokens without validating is added outside `command`, because that helper would be a second producer and would void AC-2 |
| Cross-calls | A handler that calls another handler with a rebuilt slice (`args[1:]`, `append`, a literal) needs a validated value for the callee's path. Step 1 censuses every such production call; each one either passes its own value through, calls `ValidateArgs` against the callee's definitions, or is an inline helper that should take `[]string` because it is not a handler. The first grep found no production cross-call in a handler package (the 35 hits are in `internal/test/fixture`, `le` test tooling and one `bgp/plugins/cmd/update` line, to classify) |
| Not migrated | `pkg/plugin/` SDK handler types, `registry.RootHandler`, `Registration.CLIHandler`, the `internal/appliance` handler, `rib.CommandHandler` (unchanged from the Phase 1 exclusions above) |

→ Decision: under D-2 A the option-B invoker is never written. The stored type and the author's type are the same func type, so there is nothing to wrap and nothing to delete later.

→ Constraint: no compatibility layer (`ai/rules/no-layering.md`). The raw-args signature of each handler type is deleted in the same commit that introduces the validated one; no adapter, second registration function or `RawHandler` type exists at any commit.

##### Commit order for D-2 A

A Go func type has one signature. Every value stored in a field of that type, and every call through it, changes in the same compile. So a chunk can be one handler type, never one package group of a type: while a package group of `pluginserver.Handler` is migrated and the rest is not, the unmigrated groups do not compile unless the registration accepts both signatures, which is the hybrid the rule bans. The order below keeps the tree building after each commit:

| Chunk | Content | Builds alone | Why |
|-------|---------|--------------|-----|
| C-T2a | The validated-arguments value in `command`; `ValidateArgs` returns it (its old return shape deleted). Every route R1 to R9 calls `ValidateArgs` (D-3) and hands the handler the tokens of the value it got. Handler signatures unchanged | Yes | Only the producer and its route callers change. It is not a hybrid: there is one handler signature and one producer. It already delivers D-3 (every route validates); the compile-time half arrives per type below |
| C-T2b | `StreamingHandler` takes the value: 5 handlers, R8, its registry in `plugin/server/handler.go`, and their tests | Yes | One type, every implementation and caller together |
| C-T2c | `LocalDataHandler`: 23 registrations, R2, R3, tests | Yes | Same |
| C-T2d | `LocalHandler`: 38 registration calls, R6, R7, tests | Yes | Same |
| C-T2e | `pluginserver.Handler`, `EnsureStep.Handler`, `RollbackHandler`: 339 sites in 69 directories, R1, R4, R5, about 560 test lines | Yes, and only as ONE commit | It cannot be split by package group without a second signature. Inside the batch the work proceeds by group (plugin/server and ensure first, then `component/cmd/*`, then `bgp/plugins/*`, then `internal/plugins/*`, then the other components, then the tests), but the tree builds only when every group is done |

→ Decision: C-T2e is applied by a throwaway AST rewriter kept under the session scratch directory and run over an explicit list of the files that hold the 339 sites and the test call lines, never over a directory glob or "every dirty file" (`.claude/rules/foreign-files.md`). The rewriter changes only the parameter type and the token reads; the cases it cannot rewrite are fixed by hand. This keeps the window in which the shared checkout does not build short (R-18). Before C-T2e starts, every file on its list is checked for another session's uncommitted hunks; such a file waits, or the owner is asked.

#### T3 `command.ArgDef` (`internal/component/command/node.go`)

| Census field | Content |
|--------------|---------|
| Invariant | `Name` non-empty. `Kind` selects exactly one payload: `ArgString` may carry `Lengths` and `Patterns`; `ArgUint` carries `UintBits` in {8, 16, 32, 64} and `Ranges` within that width; `ArgEnum` carries `EnumValues`; `ArgUnion` carries `UnionDefs`, each itself valid, and the flattened `EnumValues` of its enum members; `ArgFlag` carries none. `Ranges` and `Lengths` parts each have Min at most Max and are disjoint and ascending. RFC 7950 Section 9.2.4: "If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order." Section 9.4.4 states the same for length, after "Length-restricting values MUST NOT be negative." |
| Producers | Production: `yangTypeToArgDef` (`config/yang/command.go`: the literal, then writes of `Mandatory`, `ShortHelp`, `Description`, `Kind`, `EnumValues`, `UintBits`, `UnionDefs`, and `applyRange`, `applyLength`, `applyPatterns`); `appendAnchored` in the same file, which copies inherited definitions and writes `Anchor`; `argDefs` in `internal/component/mcp/tools.go`, which projects only `Name`, `Anchor` and `Mandatory` into a zero-kind definition for `command.WriteInvocation`. Test: 123 literal lines in 16 test files |
| Mutation and alias paths | Every field is assignable. The slices are shared across value copies (`PathToArgDefs` map values, `Node.ArgDefs`, `inheritArgDefs` in `plugin/server/rpc_register.go`), so a write through one copy's slice element changes all of them |
| Consumers | Type: 208 references (72 production in 16 files, 136 test). Fields, tests included: `Name` 239, `Kind` 204, `Mandatory` 102, `EnumValues` 46, `UintBits` 36, `Anchor` 31, `UnionDefs` 22, `Patterns` 14, `Ranges` 13, `Lengths` 9. Production readers: `command/` (`argvalidate.go`, `argbind.go`, `usage.go`, `arguments.go`, `completer.go`, `local_data.go`), `plugin/server/` (`command.go`, `rpc_register.go`), `mcp/tools.go`, `web/handler_admin.go`, `cli/client/` (`main.go`, `verb_tree.go`), `le/doc/yangcontract/helpshape.go`, `le/cli/dispatch/resolver.go`, `cmd/ze/hub/command_meta.go` |
| Zero | `ArgDef{}` is today an unnamed, unrestricted `ArgString` (the kind zero), which `ValidateArgString` accepts as any string |
| Docs | `docs/architecture/api/commands.md`; `docs/architecture/config/yang-config-design.md` (CLI Help from YANG, command metadata) |

#### T4 strict `DefaultLoader` (D-6)

Best-effort lives in one place. `DefaultLoader` is the only production caller of `process` besides `Resolve`, and the only place a `LoadRegistered` error is discarded. Every other `LoadRegistered` caller already returns its error.

| Caller | Calls | Discards today |
|--------|-------|----------------|
| `config/yang/loader.go` `DefaultLoader` | `LoadRegistered`, `process` | Both, by `_ =` |
| `config/yang/loader.go` `Resolve` | `process` | No |
| `config/yang_schema.go` (2), `cli/completer.go`, `cli/validator.go`, `config/schema/cli/main.go`, `aihelp/aihelp.go:243`, `le/config/claims/configclaims.go` | `LoadRegistered` | No, each returns it |
| 22 production `DefaultLoader` call sites: `le/arch/enumeration/corpus.go`, `le/doc/yangcontract/` (`contract.go`, `usage.go`, `helpshape.go`), `le/cli/list/commandlist.go`, `le/cli/grammar/cligrammar.go`, `aihelp/aihelp.go` (2), `plugin/server/server.go`, `config/infra/authz.go`, `config/yang/command.go`, `config/yang/cli/tree.go` (4), `config/cli/cmd_edit.go`, `cli/client/main.go`, `cli/testing/headless.go`, `iface/validate.go`, `cmd/ze/hub/` (`session_factory.go`, `command_meta.go`, `service_web.go`) | `DefaultLoader` | Each inherits what `DefaultLoader` discards. `service_web.go` additionally drops a `DefaultLoader` error on its own (`if loader, loaderErr := ...; loaderErr == nil`) |

What the shipped tree discards, measured on 2026-10-09 by running the real code read-only: a throwaway test entered `cmd/ze` through `go test -overlay` from the session scratch directory (no tree file touched). It loaded every registered module one by one, so one failure could not hide the rest, then ran the real `LoadRegistered`, `Resolve` and `DefaultLoader`. A second probe listed every module's registrar and its `import` and `include` edges, and `go list -deps` gave each shipped profile's non-test package set (the `./le repo compiles` matrix plus `le`).

| Profile (tags) | Modules linked | `LoadRegistered` | `process` | Discarded today |
|----------------|----------------|------------------|-----------|-----------------|
| distro (`ze_core ze_distro` + every gate) | 212 | nil | nil | Nothing (run) |
| appliance (`ze_core ze_appliance` + every gate) | 212 | nil | nil | Nothing (run) |
| le (`ze_le` + every gate) | 212 | nil | nil | Nothing (run) |
| setup (`ze_setup`), host (`ze_core ze_setup`), core only (`ze_core ze_distro`) | 89 in the shipped binary | nil | Every import edge of the 89 resolves inside the 89 (static check) | Nothing |
| The same three, but the `cmd/ze` TEST binary | 103 | nil | `no such module: ze-bgp-conf` | Yes, see below |
| installer (`ze_installer`) | 0 | - | - | Nothing |

→ Constraint: no shipped binary discards an error today, so strict `DefaultLoader` makes no shipped startup or command fail on the 2026-10-09 tree. The goyang enum misnumbering (constraint above) is the one known input that would, which is why the fork lands first.

The discarded error that does exist is structural. A module's YANG `import` is not mirrored by a Go import: each `*/yang` package registers its module and imports nothing, and only the composition root (`plugin/all`, feature-gated) makes the imported module present. 50 YANG import edges cross from one registering package to another, and all 50 have no Go import path from importer to imported (checked against `go list -deps -test` over `./cmd/ze` and `./internal/...`): 23 `bgp/plugins/*/yang` and `bgp/reactor/filter/yang` modules import `ze-bgp-conf` (`bgp/yang`); `bgp/yang` imports `ze-hub-conf`; 11 `*-cmd` modules import `ze-cli-show-cmd` or `ze-cli-clear-cmd`; `ze-flowspec-cmd` imports `ze-cli-announce-cmd`; `ze-ssh-conf` imports `ze-authz-conf`; the firewall, policyroute, anomaly, ddos, fib, trafficusage and vrrp plugin modules import their parent component's module. So any binary that links one of those packages without the composition root fails `process`. One such binary exists today: the `cmd/ze` test binary under the gate-free profiles links `bgp/plugins/route_refresh/yang` through `internal/le/doc/yangcontract`, `internal/le/cli/grammar` and `bgp/plugins/route_refresh/handler`, which import it directly, without `bgp/yang`.

| Discarded error found | Where | Becomes under strict | Phase 1 fix |
|-----------------------|-------|----------------------|-------------|
| `no such module: ze-bgp-conf`, from `ze-route-refresh` | `cmd/ze` test binary under `ze_setup`, `ze_core ze_setup`, `ze_core ze_distro` | Every `cmd/ze` test that reaches `DefaultLoader` under those tags fails | Make Go imports follow YANG imports (below) |
| Every other test binary that links a `*/yang` package without the package registering a module it imports | Not yet enumerated: each package whose tests reach `DefaultLoader` (the 22 sites above and their importers) | Those tests fail | Same fix; step 3 runs each such package's tests under the gate-free and full profiles before the strict change lands, and lists any further one here |
| `headUsage` resolves the HEAD module set and discarded every `Resolve` error except `ErrUndeclaredExtension` (the third policy) | `internal/le/doc/yangcontract/usage.go` | The baseline usage tree would silently lack a module that failed, and the comparison would report the loss as a usage change | Found in batch 2 (step 3): `Resolve` is strict there, as `DefaultLoader` is |
| No loader error in either profile once the 50 edges are Go imports (batch 2, every package with a DefaultLoader path, `ze_core ze_distro` and the full tag set): no `no such module`, no `resolve YANG modules`, no `registered YANG module` line in either log | - | - | The edges are generated: `./le yang glue write` derives each `yang/` package's blank imports from its modules' `import` and `include` statements (`internal/le/yang/glue/imports.go`), and `./le arch tier check` admits a blank import between schema packages (`schemaDependency`) |

→ Decision: the fix for the class is that every `*/yang` package blank-imports the Go package that registers each module its YANG imports or includes, so a module can never be linked without what it imports, in any binary. A new test, `TestYANGImportsFollowGoImports`, derives the edges from the registry (module name, registrar, `import` and `include` statements read from the parsed modules, not by regex over text, because a description can hold the word "import") and refuses any edge whose importer does not reach the imported registrar in Go. The alternative, keeping best-effort for test binaries only, is a second policy, which D-6 rules out. Feature gating is unaffected: a plugin's module imports only modules of the component it extends, and the gate that links the plugin links that component already.

→ Decision (owner, 2026-10-09, verbatim): "May one plugin's YANG package import another: yes to ensure we can have part of the yang used as template". The blank Go imports between `*/yang` packages that this fix adds are approved.

Behavior change under strict, stated for the owner's record:

| Surface | Today | After |
|---------|-------|-------|
| `ze` startup and every command, every shipped profile, 2026-10-09 tree | Succeeds | Succeeds (nothing is discarded today) |
| A future module that fails to parse, or imports a module the build does not link | Loads silently without that module (and, for a parse failure, without every module registered after it) | Startup, CLI, web, MCP and every `le` tool that loads the schema stop with the joined error |
| Unit test binaries linking a module without what it imports | Pass with a partial schema | Fail until the Go-import fix above lands; the fix lands in the same step, before the strict change |
| `cmd/ze/hub/service_web.go` | Drops a `DefaultLoader` error and serves without the schema | Returns the error (it is a discarded error, so D-6 covers it) |

### Phase 1 design

| Type | Representation | Compile-time guarantee | Go limits and residual runtime checks |
|------|----------------|------------------------|---------------------------------------|
| T1 | `Loader` keeps only the loading operations and one transition, `Resolve`, which returns the resolved type or an error. `DefaultLoader` returns the resolved type. Every read operation (`GetEntry`, `GetModule`, the name lists) and every consumer in the census moves to the resolved type. The resolved type holds the module set privately, plus the patterns `checkPatterns` compiled, keyed by pattern text, so `applyPatterns` reads a compiled pattern instead of compiling again | `BuildCommandTree`, the `PathTo*` and `WireMethodTo*` maps, `PublishedRPCs`, `ExtractRPCs`, `ExtractNotifications`, `NewValidator` and `CheckAllValidatorsRegistered` cannot be called with a loader, so a caller that ignored a `Resolve` or `DefaultLoader` error has no value to pass. The `applyPatterns` compile-error branch disappears, because the resolved type holds only checked patterns | The zero value and `new()` of the exported resolved type compile in any package. Contract: safe-invalid zero; each accessor on a zero or nil resolved value ends in a `BUG` panic naming the zero, because only Ze code can construct one and no input reaches it. Code inside `config/yang` can still build one by literal: audited in step 1, covered by AC-19. A pattern lowering reads that `checkPatterns` did not compile is a lowering-versus-check coverage defect, kept as a named BUG and proved absent by AC-20. `Resolve` does not revoke the loader: shared state on the module set refuses every load after resolution (AC-7) |
| T2 | A validated-arguments value type in package `command`, with private tokens and the lone positional binding, returned only by `ValidateArgs`. Every route calls `ValidateArgs`, including for a path with no definitions, and invokes a handler only with the value a successful call returned. Every author-facing handler type (`pluginserver.Handler` with `EnsureStep.Handler` and `RollbackHandler`, `registry.LocalHandler`, `registry.LocalDataHandler`, `pluginserver.StreamingHandler`) takes that value instead of a token slice (D-2 A); the raw-args signatures are deleted | A route outside `command` cannot fabricate a non-zero validated value and cannot invoke any handler, through a registry or by name, with raw tokens, so R4, R6, R7, R8 and R9 cannot stay unvalidated and a new route cannot skip the call | The zero value compiles anywhere and means no tokens and no binding. A route could pass a zero value instead of calling `ValidateArgs`; that bypass is visible as an empty literal and is caught by review and by the per-route discrimination tests (AC-22), not by the compiler. Code inside `command` can build one by literal, and only that package: the value stays in `command`, and a handler type a lower package declares cannot name it (C-T2c). So `LocalDataHandler` and its registry moved from `command/registry` into `command` (`command.RegisterLocalData`, `LookupLocalData`), rather than the value moving into a shared `command/internal/...` package, which would have let every package under `internal/component/command/` (`registry`, `grammar`, `commandtest`) build one and needed an alias for the packages outside to name it. Tests build a value through `commandtest.Args`, which calls `ValidateArgs`. The token slice belongs to the handler for the call; the value does not re-expose it to the route afterwards (AC-6) |
| T4 | `DefaultLoader` joins the `LoadRegistered` and `process` errors with the checks and returns the resolved value only when the join is empty. `LoadRegistered` attempts every module and joins every parse error. `DefaultLoader` and `Resolve` then share one policy, so the `usage.go` third policy folds into it too | A caller of `DefaultLoader` has no value when any registered module failed, exactly as with `Resolve` | None at compile time: it is an error-return change. Proved by AC-24 and by `TestYANGImportsFollowGoImports` for the binaries that link a partial set |
| T3 | `ArgDef` stays a value type, with private fields. One constructor per kind (string, unsigned with width, enum, union, flag) validates name, payload and part order and returns an error; options carry `Mandatory`, `ShortHelp`, `Description` and `Anchor`. A copy-returning method sets the anchor for `appendAnchored`. Read accessors return scalars and iterate slices without exposing them | No package outside `command` can name a payload field, so a definition with overlapping or descending parts, a width outside {8, 16, 32, 64}, or a payload of the wrong kind cannot be written, and no accessor result can be written through | `ArgDef{}` still compiles anywhere. Contract (D-4): `ValidateArgs` refuses a zero definition with an error instead of accepting an unrestricted string. Code and tests inside `command` can still write fields: audited, and tests use constructors (AC-16). Lowering must not fail on input that resolution admitted, so a constructor error inside `BuildCommandTree` is a BUG only if resolution already refuses every restriction the constructor refuses (A-11) |

### Decisions (owner answers 2026-10-09)

| # | Owner answer, 2026-10-09 (verbatim) | Decision in force |
|---|-------------------------------------|-------------------|
| D-1 | "1 yang.Resolved" | The resolved type is `yang.Resolved` |
| D-2 | "2 every handler signature" | Option A: every author-facing handler type (`pluginserver.Handler`, `registry.LocalHandler`, `LocalDataHandler`, `StreamingHandler`) takes the validated-arguments value, not only the stored or dispatched type. The larger scope: about 290 production handlers in about 40 packages (re-counted above: 339 signature lines in 69 directories for `pluginserver.Handler` alone, plus 61 local registrations and 5 streaming handlers) and about 560 test call lines. The recommendation of B below is superseded |
| D-3 | "3 Validate on all of them" | Every route R1 to R9 calls `ValidateArgs` |
| D-4 | "4 Refuse it in the validator." | `ValidateArgs` refuses a zero `ArgDef` with an error |
| D-5 | "5 One type with per-kind constructors" | One `ArgDef` struct, private fields, one constructor per kind |
| D-6 | "6 Keep it; make it strict", then confirmed: "change behaviour is fine, make it strict" | Keep `DefaultLoader`, make it strict: its `LoadRegistered` and `process` no longer discard errors, and every error reaches the caller. The main thread first read the answer this way; the owner confirmed that reading on 2026-10-09, so it is the decision, not a reading. The recommendation to preserve best-effort below is superseded. Census and behavior change: T4 |
| D-7 | "7 The narrower input" | `command.WriteInvocation` takes a name-and-anchor input |

The options the owner chose between, as presented on 2026-10-08:


| # | Decision | Options | Recommendation |
|---|----------|---------|----------------|
| D-1 | Name of the resolved type | `yang.Schema` (the style-guide example); `yang.Resolved`; `yang.ModuleSet` | `yang.Resolved`. `internal/component/config` already declares `Schema`, built from the loader, and imports this package as `yang`, so `yang.Schema` beside `Schema` in one file names two different things |
| D-2 | Which handler types take validated arguments | A: every author-facing handler type (`pluginserver.Handler`, `registry.LocalHandler`, `LocalDataHandler`, `StreamingHandler`): about 290 production handler functions in about 40 packages and about 560 test call lines in 68 files. B: the stored and dispatched handler type only. Registration still accepts the author's function; the registry and dispatcher store it behind an invoker that requires the validated value, and routes are its only callers. Cost: registry and dispatcher internals, the nine routes, and the tests that invoke through them | B in Phase 1. It gives the stated guarantee, that a route which skips validation does not compile, at about a tenth of the cost. Handler bodies read the same tokens either way, because `ValidateArgs` passes unmatched tokens through. Residual: calling a handler function by name bypasses the registry; step 1 censuses production cross-calls. If the owner wants A, split it out as Phase 1b after B lands, deleting B's invoker rather than keeping both |
| D-3 | Routes that validate nothing today (R6 local handlers, R7 offline fallback, R8 streaming, R9 plugin forwarding) | Validate on every route; or only on the three routes 40db22a585 fixed | Every route. The type makes skipping impossible to write, and a route left out keeps the defect class open. It adds refusals only for tokens the model already declares invalid; each route gets a discrimination test |
| D-4 | Zero `ArgDef` contract | Keep it a valid unrestricted string (today); refuse it at `ValidateArgs`; change the zero of `ArgKind` to an unspecified kind | Refuse at `ValidateArgs` with an error, through a private constructed marker, without changing the `ArgKind` zero. A zero definition is a Ze defect, and an unrestricted fallback is fail-open |
| D-5 | Kind and payload representation for `ArgDef` | One struct with private fields and per-kind constructors; or a sealed interface with one variant per kind | One struct with per-kind constructors. The consumers already switch on `Kind` (a closed switch in `argvalidate.go`), definitions travel in value slices, and variants would box each definition. The public API still cannot express a mismatched combination, which is what AC-3 asks |
| D-6 | `DefaultLoader`'s best-effort `LoadRegistered` and `process` | Preserve; make both strict; keep best-effort and report the discarded errors | Preserve in Phase 1, documented on the resolved type as "imports resolved where they resolve"; changing it is a behavior change outside this phase. The `usage.go` policy, which builds a tree after a pattern or structure failure, does not survive: it becomes a call to the shared transition |
| D-7 | The MCP `argDefs` projection | Construct full definitions from the lister; or give `command.WriteInvocation` a name-and-anchor input | Give `WriteInvocation` the narrower input it reads. A definition built with no restrictions is a value that lies about what the argument accepts |

### Phase 1 Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-5 | `spec-config-yang-loader-structural-checks` lands before Phase 1 starts and leaves `checkStructure` inside `Resolve` and `DefaultLoader` | Uncommitted `loader.go` diff and untracked `loader_structure.go` in the working tree on 2026-10-08 | T1's invariant row changes, and the edits collide | Re-read `loader.go` at HEAD before step 1 | Open |
| A-6 | Every consumer of a loader reads it only after `Resolve` or `DefaultLoader` | Census above; the nine callers fixed in ac6c5ce12d and a5b3180063 | A consumer that reads mid-load cannot take the resolved type | Step 1 reference walk, each production call site classified | Open |
| A-7 | Calling `ValidateArgs` on a path with no definitions changes nothing | Read in `argbind.go`: keyword and mandatory phases iterate definitions, and the positional phase refuses only when open definitions remain | Routes that skip today would start refusing | `TestValidateArgsEmptyDefinitionsPassTokens` | Verified by reading, test owed |
| A-8 | No consumer writes into a goyang entry or module obtained through the loader | Not yet checked | The resolved type's read-only contract is false | Step 1: uses of `GetEntry` and `GetModule` results checked for writes | Open |
| A-9 | Ensure-chain creation handlers declare no mandatory argument | Not yet checked; R5 passes nil tokens today | Validating R5 with no tokens would refuse a working ensure step | Step 1: definitions of each ensure-step command | Open |
| A-10 | Plugin-process commands that declare definitions in the merged model can be validated in the engine before forwarding | `inheritArgDefs` in `rpc_register.go` merges model definitions | R9 validation needs definitions the engine does not hold | Step 1: `RegisteredCommand` paths against `PathToArgDefs` | Open |
| A-12 | The goyang fork lands before step 1 and changes no Ze-visible behavior other than enum numbering | Another agent's task on 2026-10-09; journal rows in `zero-value-as-valid-answer.md` | Strict `DefaultLoader` could fail on a valid enumeration, or the vendored tree changes under T1 | Step 1: `go.mod` carries the `replace`; re-run the D-6 probe over the new tree | Open |
| A-13 | No shipped profile links a module without the module it imports | D-6 probe 2026-10-09: runtime over distro, appliance, le; static `go list -deps` over setup, host, core, installer | Strict `DefaultLoader` would stop a shipped binary | `TestYANGImportsFollowGoImports` makes it hold by construction; step 3 re-runs the probe | Verified 2026-10-09 |
| A-11 | Resolution refuses every restriction the `ArgDef` constructor refuses, for the types lowering reads | `checkStructure` refuses length parts that overlap or descend (`ErrLengthOrder`); Ze checks no range order, and goyang's range parsing is unread | A module loaded from a file (`AddModuleFromFile`, used by `le` tooling) could reach a constructor error inside lowering, making a BUG input-reachable | Read goyang's range parsing; if it checks nothing, add the range-order check to resolution in coordination with the structural-checks spec, or make lowering return the error | Confirmed 2026-10-09 (batch 1). goyang `YangRange.parseChildRanges` (`vendor/github.com/openconfig/goyang/pkg/yang/types_builtin.go`) refuses a part whose max is below its min, sorts and coalesces the parts, so lowering never sees parts that overlap or descend, and refuses parts outside the base type's range, so no range exceeds the width. `Type.resolve` (`types.go`) keeps the parent range or length when it refuses, so the best-effort `DefaultLoader` path also lowers only admitted parts. A negative length part falls outside `Uint64Range` and is refused the same way. `parseEnumAssignment` refuses an enumeration that lists no enum, and `yangTypeToArgDef` then builds no argument. Every lowered name is a leaf name. The constructor refusal inside `yangTypeToArgDef` is therefore a named BUG, and `TestCommandTreeBuildsFromEveryRegisteredModule` lowers the full registered set with none |

### Phase 1 Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-13 | The concurrent loader work and Phase 1 edit `loader.go` together | Dirty `loader.go` or untracked `loader_structure*.go` at step 1 | Start only after A-5 holds; re-read the landed source |
| R-14 | Route validation changes an operator-visible answer (text or order) | `cli-argument-length-refused.ci` or a dispatcher test changes output | Keep the held-error order in `Dispatch`; assert each route's existing refusal text |
| R-15 | A constructor refuses a definition the lowering produces today | `BuildCommandTree` over the registered module set reports a constructor error | AC-20 builds the tree from every registered module; a refusal is a model defect, fixed in the model, never by weakening the constructor |
| R-16 | A zero resolved value or zero validated-arguments value slips through | A test or route builds an empty literal | Positive-control probes record that it compiles; AC-4 contract tests; review of every empty literal |
| R-18 | C-T2e leaves the shared checkout unbuildable for other sessions while its 69 directories are edited, and collides with their uncommitted hunks in handler files | Another session reports a build failure in a handler package; `git status` shows foreign hunks in a file on the C-T2e list | AST rewriter over an explicit file list; check each listed file for foreign hunks first; land C-T2e in one sitting and commit at once |
| R-19 | Strict `DefaultLoader` turns test binaries that link a partial module set red | A package's unit tests fail with `no such module` after step 3 | The Go-import fix and `TestYANGImportsFollowGoImports` land before the strict change in the same step; each package with a `DefaultLoader` path is run under the gate-free and full profiles first |
| R-20 | A blank import added for a YANG import creates an import cycle | `go build` reports `import cycle not allowed` | `*/yang` packages hold only registration; if a cycle appears, the module boundary is wrong and is reported to the owner, never solved by keeping best-effort |
| R-17 | Census counts miss build-tagged or platform files | A Linux-only or tagged file fails to build after the migration | Step 1 reconciles with the build-tag populations; the final build covers them |

### Phase 1 Acceptance Criteria

Existing criteria reused, scoped to T1, T2 and T3:

| AC ID | Phase 1 reading |
|-------|-----------------|
| AC-2 | Outside its package, no code can give the resolved type, the validated-arguments type or an `ArgDef` a nonzero state except through `Resolve` or `DefaultLoader`, `ValidateArgs`, and the `ArgDef` constructors. Each constructor accepts every definition the registered modules lower today and refuses an invalid one without returning a value |
| AC-3 | A loader cannot be passed where the resolved type is required; no handler of the four author-facing types (D-2 A) can be declared with or invoked with raw tokens; an `ArgDef` whose kind and payload disagree cannot be written outside `command` |
| AC-4 | A zero or nil resolved value ends in a named BUG panic at the first accessor; a zero validated-arguments value carries no tokens; `ValidateArgs` refuses a zero `ArgDef` with an error. Tests record each, and the compile-positive controls show Go admits the zero |
| AC-6 | No accessor of the three types returns a slice or map whose writes change the stored value; a write to a constructor's input slice after construction changes nothing |
| AC-7 | After `Resolve` succeeds or fails, every load operation on the loader returns an error, and the resolved value's answers are unchanged |
| AC-13 | Every `ArgDef` the YANG lowering, `appendAnchored` and the census's manual producers publish comes from a constructor; usage, completion, help and MCP output for valid definitions are unchanged |
| AC-16 | Every Phase 1 census row and caller is migrated, the `applyPatterns` compile-error panic is gone, tests use the real constructors, and the build-tag populations compile |
| AC-17 | `docs/architecture/config/yang-config-design.md` and `docs/architecture/api/commands.md` describe the resolved type, the validated-arguments value and the constructors, with source anchors |
| AC-18 | Compile-negative probes fail with the expected type error beside a positive control that compiles; runtime tests reach the real routes |

Phase-specific criteria:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-19 | Every production producer of the resolved type, of the validated-arguments value and of `ArgDef`, after migration | Each is the transition or a constructor; same-package literal construction appears only in that package's zero-contract tests |
| AC-20 | `BuildCommandTree` over the full registered module set, and over a module whose pattern or length restriction resolution refuses | The full set builds with no constructor error and no BUG; the refused module yields a `Resolve` error and no resolved value |
| AC-21 | A module set where resolution fails, through `ze` startup, the CLI tree, API and MCP metadata, and the `le` tooling paths | Each reports the resolution error and none reaches a command tree. The `usage.go` HEAD baseline refuses on a pattern or structure failure as well as on an undeclared extension |
| AC-22 | A token that breaks a declared length, pattern or range, on each route R1 to R9 that carries a declared argument | Each route refuses before the handler runs, with the existing refusal text, and a token at the bound reaches the handler. Removing one route's validation call turns that route's test red |
| AC-23 | A command path with no declared definitions, on each route | Arbitrary tokens reach the handler unchanged |
| AC-24 | A registered module that fails to parse, and a registered module that imports a module no package registers, through `DefaultLoader` | `DefaultLoader` returns an error naming each failure and no value; a module registered after the failing one is still parsed and its own error, if any, is also reported; `ze` startup over such a set stops with that error |
| AC-25 | Every YANG `import` or `include` edge between modules of different registering packages | The importer's Go package reaches the imported module's registering package; removing one such blank import turns `TestYANGImportsFollowGoImports` red |

### Phase 1 Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze` daemon startup | → | `DefaultLoader` returns the resolved type; `cmd/ze/hub/session_factory.go` and `plugin/server/server.go` build the tree from it | Existing `test/ui/cli-argument-length-refused.ci`, which boots the daemon, and the tests in `cmd/ze/hub/session_factory_extension_test.go`, migrated to the resolved type |
| `ze cli -c` with `show env get` and a 129-character key | → | R2 `ServeLocal`, invoking through the validated value | `test/ui/cli-argument-length-refused.ci`, in-process sequence |
| `ze show env get` with a 129-character key | → | R3 and R6 local routes | `test/ui/cli-argument-length-refused.ci`, verb sequence |
| Daemon command `show metrics name` with a 129-character name, over SSH | → | R1 `Dispatch` | `test/ui/cli-argument-length-refused.ci`, daemon sequence |
| A plugin command with a declared argument, chosen in step 1 from R9's census | → | R9 engine-side validation before forwarding | New `test/ui/cli-argument-refused-plugin-route.ci` |
| A `monitor` command with a declared argument, over SSH | → | R8 streaming route | The same new `.ci`, second sequence, if a streaming command declares an argument (step 1); otherwise AC-23's empty-definition test covers R8 |
| `ze` daemon startup over a module set with a registered module that fails to parse | → | strict `DefaultLoader` (T4) | `TestDefaultLoaderReportsEveryRegisteredFailure` plus a `.ci` boot that registers no extra module and asserts a clean start, `test/ui/cli-argument-length-refused.ci` serving as that boot |
| YANG lowering of every registered module | → | `ArgDef` constructors through `BuildCommandTree` | `TestYANGArgumentAdmissionPreservesValidation` (planned above), extended to every kind, plus `TestCommandTreeBuildsFromEveryRegisteredModule` |

### Phase 1 TDD plan

| Test | File / carrier | Validates |
|------|----------------|-----------|
| `TestResolveRefusesLoadAfterResolution` | `internal/component/config/yang/loader_test.go` | AC-7 |
| `TestResolvedZeroValueIsABug` | same package, external test file | AC-4 for T1 |
| `TestCommandTreeBuildsFromEveryRegisteredModule` | `config/yang/command_test.go` | AC-20, AC-13 |
| `TestYANGContractBaselineRefusesFailedResolution` | `internal/le/doc/yangcontract/usage_test.go` | AC-21, the `usage.go` policy |
| `TestValidateArgsEmptyDefinitionsPassTokens` | `internal/component/command/argbind_test.go` | AC-23, A-7 |
| `TestEveryRouteInvokesWithValidatedArguments` | One test per route: `plugin/server/command_flag_test.go` (R1), `server_test.go` (R4), `ensure_test.go` (R5), `command/local_data_test.go` (R2), `command/registry/local_data_args_test.go` (R3, R6, R7), a `cmd/ze/hub` SSH streaming test (R8), a `plugin/server` forwarding test (R9) | AC-22, with a red observed by removing the route's validation call |
| `TestArgDefConstructorRefuses` | `internal/component/command/node_test.go` | AC-2, AC-3: empty name; width outside {8, 16, 32, 64}; a range above the width; overlapping parts; descending parts; empty enum; union with an invalid member. Each case changes one fact of a valid definition |
| `TestArgDefAccessorsDoNotAlias` | same | AC-6: mutate the constructor's input slices; the iterated payload is unchanged |
| `TestZeroArgDefRefused` | `command/argvalidate_test.go` | AC-4, D-4 |
| `TestWriteInvocationPlacesAnchoredValues` | `mcp/tools_anchor_test.go` | D-7, unchanged output |
| `TestDefaultLoaderReportsEveryRegisteredFailure` | `config/yang/loader_test.go` | AC-24: one unparsable module registered before a valid one and an unresolved import; both errors reported, no value |
| `TestYANGImportsFollowGoImports` | `internal/component/plugin/all/` test, over the full registry | AC-25, A-13 |
| `TestHandlerTypesTakeValidatedArguments` | one per handler type: `plugin/server/handler_test.go` (`StreamingHandler`), `command/local_data_args_test.go` (`LocalDataHandler`), `command/registry/local_data_args_test.go` (`LocalHandler`), `plugin/server/command_flag_test.go` (`Handler`) | AC-3: a registered handler of each type receives exactly the tokens `ValidateArgs` returned |

Compile-negative probes follow the procedure in the TDD plan above: one forbidden operation per case, the positive control compiled first under the same module and tags, the error text recorded in this spec, the probe files deleted at step 5.

| Probe | Forbidden operation (expected type error) | Positive control (compiles) |
|-------|--------------------------------------------|-----------------------------|
| P-1 | Passing a loader to `BuildCommandTree` | Passing the value `Resolve` returned |
| P-2 | A literal of the resolved type naming a field, from another package | The empty literal and `new()` of the resolved type |
| P-3 | For each of the four handler types: declaring a handler with a `[]string` parameter and registering it, and calling a handler with a token slice, from a package outside `command` | The same with the value `ValidateArgs` returned |
| P-4 | A literal of the validated-arguments type naming a field, from another package | Its empty literal |
| P-5 | An `ArgDef` literal naming `Lengths`, `Patterns` or `Kind`, from `config/yang` | The constructor call; the empty literal |
| P-6 | Assigning into an element of what an `ArgDef` accessor returns | Ranging over it |

→ Evidence (batch 1, 2026-10-09): P-5 and P-6 ran from throwaway packages under `internal/component/config/yang/`, one forbidden operation each, removed after the run. Positive control (constructor call, `command.ArgDef{}`, ranging over `EnumValues()`): compiles. P-5 `command.ArgDef{Lengths: nil}`: "unknown field Lengths in struct literal of type command.ArgDef, but does have unexported lengths"; `Patterns` and `Kind` give the same error for `patterns` and `kind`. P-6 `def.EnumValues()[0] = "x"`: "cannot index def.EnumValues() (value of func type iter.Seq[string])"; `def.Ranges()[0] = command.UintRange{}`: "cannot index def.Ranges() (value of func type iter.Seq[command.UintRange])". Before batch 1 each of these compiled: the fields were exported and 123 test literal lines named them.

→ Evidence (batch 3, 2026-10-09): P-1 and P-2 ran the same way, from throwaway packages under `internal/component/config/yang/zprobe/`, removed after the run. Positive control (`loader.Resolve()` then `yang.BuildCommandTree(schema)`, `yang.Resolved{}`, `new(yang.Resolved)`): compiles. P-1 `yang.BuildCommandTree(yang.NewLoader())`: "cannot use yang.NewLoader() (value of type *yang.Loader) as *yang.Resolved value in argument to yang.BuildCommandTree". P-2 `yang.Resolved{modules: nil}`: "cannot refer to unexported field modules in struct literal of type yang.Resolved". Red runs: with `refuseLoad` answering nil, `TestResolveRefusesLoadAfterResolution` failed 10 times ("want ErrLoaderResolved"); with the `Resolved.set` guards removed, `TestResolvedZeroValueIsABug` failed 24 times ("want a BUG panic"). Both green restored.

→ Evidence (C-T2a, 2026-10-09, code uncommitted at the handover): `command.ValidatedArgs` (private `tokens`, `bound`, `boundValue`; `Tokens()` answers a copy; `Positional(leaf)`) is what `ValidateArgs` returns, its old `map[string]string` shape deleted. A missing mandatory leaf is a `*command.MissingArgumentError` carrying the lone binding, so `Dispatch` still adopts a positional selector before it reports the missing leaf (`adoptablePositional`). `command.ValidateModelArgs(path, args, preMatched)` judges against the model for the routes that hold no definitions; `registry.ValidateLocalArgs` (installed by `RegisterArgDefSource` through `registry.RegisterLocalArgCheck`) does it for the registry routes. P-4: positive control `command.ValidatedArgs{}` and `new(command.ValidatedArgs)` compile; `command.ValidatedArgs{tokens: []string{"x"}}` from a throwaway `internal/component/plugin/zprobect2a/neg`: "cannot refer to unexported field tokens in struct literal of type command.ValidatedArgs" (probe removed). Per-route red, each route's validation call removed and its handler fed the raw tokens (`ct2a-mutate.py` under the session scratch), the file restored byte-identically after each run: R1 `TestDispatchInvokesWithValidatedArguments` "an over-long label was accepted"; R2 `TestServeLocalJudgesArgumentsAgainstTheirDefinitions` "over-long argument exit code = 0, want 1"; R3 `TestLocalDataPlainHandlerJudgesArguments` "no check installed: exit 0, want 1"; R4 `TestWrapHandlerInvokesWithValidatedArguments` "an over-long name was accepted"; R5 `TestEnsureChainInvokesWithValidatedArguments` "an over-long ancestor name was accepted"; R6 `TestInvokeLocalHandlerJudgesArguments` "over-long key: exit 0, want 1"; R7 `TestOfflineFallbackJudgesArguments` "over-long name: code 0 served true, want 1 true"; R8 `TestStreamingLookupInvokesWithValidatedArguments` "an over-long name was accepted"; R9 `TestRouteToProcessInvokesWithValidatedArguments` "an over-long name was forwarded"; R9 subsystem `TestDispatchSubsystemInvokesWithValidatedArguments` "an over-long name reached the subsystem". Green restored: each passes under `-race` and the feature tags. AC-23 and A-7: `TestValidateArgsEmptyDefinitionsPassTokens`. Not yet discriminated: the second R6 site, the root fallback in `cmd/ze/ze_core_dispatch.go`, has no unit test.

→ Evidence (C-T2b, 2026-10-09): `pluginserver.StreamingHandler` is `func(ctx, *Server, io.Writer, username string, args command.ValidatedArgs) error`; the `[]string` signature is deleted. The five handlers (`streamIPsecMonitor`, `StreamEventMonitor`, `streamTraffic`, `streamInterfaceRate`, `streamNetlinkMonitor` on both platforms) take the value, and R8 (`cmd/ze/hub/service_ssh.go`, plus the API stream source in `cmd/ze/hub/api.go`) passes the value `GetStreamingHandlerForCommand` answered. Red before the change: `TestHandlerTypesTakeValidatedArguments/StreamingHandler` (`plugin/server/handler_test.go`) did not compile ("cannot use func(...args command.ValidatedArgs) error as StreamingHandler value in argument to RegisterStreamingHandler"; "cannot use validated (variable of struct type command.ValidatedArgs) as []string value in argument to handler"). P-3 for `StreamingHandler`, from throwaway packages under `internal/component/plugin/zprobect2b/` (removed): before the change the positive control failed and both negatives compiled. After it, the positive control (register a `command.ValidatedArgs` handler, call it with the looked-up value) compiles; registering a `[]string` handler fails with "cannot use (func(context.Context, *pluginserver.Server, io.Writer, string, []string) error literal) ... as server.StreamingHandler value in argument to pluginserver.RegisterStreamingHandler"; calling a handler with a token slice fails with "cannot use []string{…} (value of type []string) as command.ValidatedArgs value in argument to h". Green under `-race` and the feature tags: the new test and the package tests of plugin/server, iface/cmd, trafficstat, ike/cmd, bgp/plugins/cmd/monitor and ssh, and the stream tests of cmd/ze/hub and cli. `TestPluginStateTransportParity` is red at HEAD too (run on a `git archive HEAD` export: "context deadline exceeded", load average 12 to 14). The `TestHandlerTypesTakeValidatedArguments` row for `pluginserver.Handler` (C-T2e) is a second subtest of the same test in package `server`, since one package cannot declare the name twice.

→ Decision (C-T2c, 2026-10-09): `command` imports `command/registry`, so the registry could not name `command.ValidatedArgs`. The data-handler registry moved into `command` (`LocalDataHandler`, `RegisterLocalData`, `MustRegisterLocalData`, `LookupLocalData`, `ResetLocalDataForTest` in `command/local_data.go`); the registry's copies are deleted, with no alias. Rejected: moving the value into `command/internal/args`, because Go internal visibility admits every package under `internal/component/command/` (`registry`, `grammar`, `commandtest`, any later one), which widens the producer set, and packages outside the subtree could name the type only through an alias. The `Meta` argument stays `registry.Meta`. The AST readers that find a data registration (`le/cli/grammar` `registryCall`, `le/doc/yangcontract` `localRegistrar`) now match it under the `command` qualifier.

→ Evidence (C-T2c, 2026-10-09): `command.LocalDataHandler` is `func(args command.ValidatedArgs) (any, int)`; the `[]string` signature is deleted with the registry's data-handler code. The 21 data handlers (`plugins/env` 3, `component/plugin` 3, `config/yang/cli` 2, `config/schema/cli` 5, `config/storage/cli` 2, `config/cli` 5, and `le/le/root` through `toolHandler`, which adapts every `leroot.Answer`) take the value. R2 (`ServeLocal`) hands it the value it judged; R3 is `command.plainLocalData`, which judges with `ValidateModelArgs` and hands the handler the value. `le`'s own dispatcher `leroot.Run` now takes a `command.LocalDataHandler` and judges the tool's words against no definitions (`ValidateArgs(args, nil, nil)`), since an `le` tool declares none. Direct test calls build the value with the new `commandtest.Args`. Red before the change: `TestHandlerTypesTakeValidatedArguments/LocalDataHandler` (`command/local_data_args_test.go`) did not compile ("undefined: LocalDataHandler", "undefined: ResetLocalDataForTest"). P-3 for `LocalDataHandler`, from throwaway packages under `internal/component/plugin/zprobect2c/` (removed): before the change, registering a `command.ValidatedArgs` handler failed ("cannot use (func(args command.ValidatedArgs) (any, int) literal) ... as registry.LocalDataHandler value in argument to registry.MustRegisterLocalData") and both negatives compiled. After it, the positive control (register a `command.ValidatedArgs` handler, call the looked-up handler with the value `ValidateModelArgs` returned) compiles; registering a `[]string` handler fails with "cannot use (func(args []string) (any, int) literal) ... as command.LocalDataHandler value in argument to command.MustRegisterLocalData"; calling a handler with a token slice fails with "cannot use tail (variable of type []string) as command.ValidatedArgs value in argument to h". R3's refusal test moved with the code: `TestLocalDataPlainHandlerJudgesArguments` (`command/local_data_args_test.go`). Green under `-race` and the feature tags: command (and grammar, registry), plugins/env, config/cli, config/yang/cli, config/schema/cli, config/storage/cli, le/le/root, le/verify/dispatch, le/doc/yangcontract, le/go/extract, le/workflowcheck, le. Load-flaky and pre-existing: `TestWriteAnswerReleasesBufferOnError` (`component/plugin`, 28 allocations against 27 in the loaded run, green alone `-count=3`); `TestTheRealCheckoutPassesAndWasRead` (`le/cli/grammar`) red on the same five R1 roots as at HEAD (batch 2 note).

→ Decision (C-T2d, 2026-10-10): the local-handler and offline-fallback registries moved from `command/registry` into `command` (`command/local.go`: `LocalHandler`, `RegisterLocal`, `RegisterLocalMeta`, their `Must` forms, `LookupLocal`, `RegisterOfflineFallback`, `MustRegisterOfflineFallback`, `LookupOfflineFallback`, `ListLocal`, `HasLocal`, `LocalCommandEntry`, `ResetLocalForTest`), for the reason C-T2c gave; the registry keeps the root registries and `Meta`. The routes judge through one new function, `command.InvokeLocal(path, handler, args)`, which calls `ValidateModelArgs` and runs the handler on the value it returned, so `registry.ValidateLocalArgs`, `registry.RegisterLocalArgCheck` and `command.validatedLocalTokens` are deleted rather than moved: the indirection existed only because the registry could not import `command`. R3's plain handler (`plainLocalData`) no longer judges itself; it takes the value its R6/R7 route judged. `dispatchHelp` (`cmd/ze`), which called the `help command` and `help ai` handlers directly with raw words, now runs them through `InvokeLocal` under their registered paths like R6. `cmdutil.LocalHandler` (an alias) and `cmdutil.registerLocalCommand` (a passthrough) are deleted (no-layering). The AST reader `le/doc/yangcontract` `localRegistrar` matches every local registration under the `command` qualifier only; `le/cli/grammar` reads no plain local registration and is unchanged.

→ Evidence (C-T2d, 2026-10-10): `command.LocalHandler` is `func(args command.ValidatedArgs) int`; the `[]string` signature is deleted with the registry's local-handler code. The 26 registered handlers take the value (`plugins/debug` 8, `ping/cmd` 2, `traceroute/cmd` 2, `config/cli` 3, `bgp/cli` 2, `iface/cli`, `config/storage/cli`, `config/yang/cli`, `doctor`, `support`, `plugins/diag`, `plugins/explain`, `plugins/skills`, the fallbacks `plugins/host` and `plugins/crashes`, and `cmd/ze`'s `show version`, `help command`, `help ai`, `update serve`). R6 (`cmdutil.invokeLocalHandler`, `invokeRootLocalHandler`, `dispatchHelp`) and R7 (`client.runOfflineFallback`) call `InvokeLocal`. Red before the change: `TestHandlerTypesTakeValidatedArguments/LocalHandler` (`command/local_data_args_test.go`) did not compile ("undefined: ResetLocalForTest"). P-3 for `LocalHandler`, from throwaway packages under `internal/component/plugin/zprobect2d/` in `git archive` exports (never in the tree): at HEAD, registering a `command.ValidatedArgs` handler failed ("cannot use (func(args command.ValidatedArgs) int literal) ... as registry.LocalHandler value in argument to registry.MustRegisterLocal") and both negatives compiled. After the change, the positive control (register a `command.ValidatedArgs` handler, `LookupLocal`, `InvokeLocal`) compiles; registering a `[]string` handler fails with "cannot use (func(args []string) int literal) ... as command.LocalHandler value in argument to command.MustRegisterLocal"; calling a looked-up fallback with a token slice fails with "cannot use tail (variable of type []string) as command.ValidatedArgs value in argument to h". `TestLookupLocalRefusesToSwallowADeclaredChild` moved with `LookupLocal` to `command/local_test.go`; `TestValidateLocalArgsAnswersTheJudgedTokens` went with the function it tested (test/weakened rows).

→ Decision (C-T2e, 2026-10-10): the throwaway AST rewriter (session scratch `rw4e`, modes scan, decl, calls, fix) ran over an explicit list of 153 files (470 signatures: 342 production in 129 files, 128 test in 24), never a directory glob. By hand: `EnsureStep`'s wrapper takes the value and passes it to the leaf handler; the UPDATE text encoders (`handleUpdateText`, `Hex`, `B64`, `Cursor` and the `updateEncodings` map) keep `[]string`, because they parse the tail after the encoding keyword and are fed `args.Tokens()[1:]`, not a dispatch argument list; `handleMonitorTraceroute` passes the value on to `HandleProbeRound`, and `show bgp` to `handleBgpSummary`; parameters and constants named `command` were renamed (`pluginCommand` in vrrp `forwardNoArgs`/`forwardWithArgs` and the ospf `dbSubviewForwarder`, `commandName` in the hub tests `infra_setup_auth`, `rfc8907_aaa_lifecycle`, `service_grpc`), because they shadowed the package. The owner exception of 2026-10-09 waived the foreign-files rule for this batch; on 2026-10-10 none of the 233 files carried another session's hunk, so none rode along.

→ Evidence (C-T2e, 2026-10-10): `pluginserver.Handler` is `func(ctx *CommandContext, args command.ValidatedArgs) (*plugin.Response, error)` (`plugin/server/command.go`); the `[]string` signature is deleted, and `EnsureStep.Handler` and `RollbackHandler` carry the same type. Red before the change: `TestHandlerTypesTakeValidatedArguments/Handler` (`plugin/server/handler_test.go`) did not compile ("cannot use handler (variable of type func(_ *CommandContext, args command.ValidatedArgs) (...)) as Handler value in argument to d.RegisterWithOptions"). P-3 for `Handler`, from throwaway packages under `internal/component/plugin/zprobect2e/` (removed): before the change the positive control failed and both negatives compiled; after it, the positive control compiles, registering a `[]string` handler fails ("cannot use (func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) literal) ... as server.Handler value in argument to pluginserver.NewDispatcher().Register"), and calling a handler with a token slice fails ("cannot use []string{…} (value of type []string) as command.ValidatedArgs value in argument to h"). `go vet ./...` compiles every package under the full feature tags (darwin and linux) and the appliance, setup, host, installer and distro tag sets. The batch was rebased onto HEAD `2b8d8101e4` (no commit since its base `2d98a75602` touched any of its 233 files) and run under `-race` and the feature tags over its 76 package directories in a `git archive` export: 69 ok, 5 with no test files, 2 red that are not the batch's. `TestWebTemplPortFidelity` (web) fails only because an export under the checkout's `tmp/` breaks its `git archive` pathspec; it is green in the export with `GIT_DIR`/`GIT_WORK_TREE` pointed at the checkout, and green at HEAD in the tree (journal `gate-verdict-depends-on-the-machine.md`, 2026-10-10). `TestRFC5187InterfaceIDPreservedAcrossRestart` (ospf) fails the same way at HEAD: macOS has no `lo` interface (same journal, 2026-10-03). Seven RFC-tagged units changed only their handler parameter type, one local name and one empty-arguments value; the owner approved them on 2026-10-10 (handover, "Owner approval for batch 4e").

### Phase 1 files

| File | Change |
|------|--------|
| `internal/component/config/yang/loader.go` | Resolved type, transition, refusal of loads after resolution, accessors moved. Shared with the structural-checks spec |
| `internal/component/config/yang/loader_structure.go` | Range-order check if A-11 requires it. Shared, in flight |
| `internal/component/config/yang/command.go` | Consumers take the resolved type; lowering through constructors; `applyPatterns` reads compiled patterns |
| `internal/component/config/yang/` `validator.go`, `validator_registry.go`, `rpc.go`, `rpc_publish.go`, `enum.go` | Take the resolved type. `validator.go` is shared with the structural-checks spec; `rpc_publish.go` and `rpc_test.go` carry another session's uncommitted hunks on 2026-10-08 |
| Every T1 consumer file in the census | Take the resolved type |
| `internal/component/command/` `node.go`, `argbind.go`, `argvalidate.go`, `usage.go`, `arguments.go`, `completer.go`, `local_data.go` | `ArgDef` constructors and accessors; validated-arguments value |
| `internal/component/command/registry/registry.go` | `LocalHandler` and `LocalDataHandler` take the validated value (D-2 A); R3, R6, R7 |
| `internal/component/plugin/server/` `command.go`, `server.go`, `ensure.go`, `handler.go`, `rpc_register.go`, `command_registry.go` | R1, R4, R5, R8 lookup, R9; `ArgDef` inheritance through the copy method. `handler.go`, `rpc_register.go` and `owner_prefix_test.go` carry another session's uncommitted hunks on 2026-10-08 |
| `cmd/ze/hub/` `service_ssh.go`, `command_meta.go`, `session_factory.go`, `service_web.go`; `cmd/ze/ze_core_dispatch.go`; `cmd/ze/internal/cmdutil/cmdutil.go`; `internal/component/cli/client/main.go` | Routes and resolved-type consumers |
| `internal/component/mcp/tools.go`, `internal/component/web/handler_admin.go`, `internal/le/doc/yangcontract/`, `internal/le/cli/dispatch/`, `internal/le/cli/grammar/`, `internal/le/cli/list/` | `ArgDef` accessors, D-7, resolved type |
| The 16 test files with `ArgDef` literals, and every test file the T1 and T2 counts name | Constructors, resolved type, validated invocation |
| Every package in the D-2 A census (69 directories for `pluginserver.Handler`, the local and streaming registrants), with their tests | Handler parameter type, chunks C-T2b to C-T2e |
| Every `*/yang` package with a cross-registrar YANG import (the 50 edges in T4) | Blank import of the registering package of each imported module |
| `internal/component/plugin/all/` test file | `TestYANGImportsFollowGoImports` |
| `cmd/ze/hub/service_web.go` | Stop discarding the `DefaultLoader` error |
| `go.mod`, `go.sum`, `vendor/` | Not edited by Phase 1; shared with the goyang fork, which lands first |
| `test/ui/cli-argument-refused-plugin-route.ci` | New |
| `docs/architecture/config/yang-config-design.md`, `docs/architecture/api/commands.md` | Updated in the step that changes the behavior each describes |
| `docs/contributing/ze-go-style.md` | Only if D-1 picks a name other than the guide's illustrative `Schema` and the owner wants the example to match |

### Phase 1 implementation steps

Each step is one commit, or the chunks it names; the tree builds after each. Resume state is the committed handover `plan/handover/12-validated-construction-phase-1.md`.

1. **Precondition and census.** Confirm `spec-config-yang-loader-structural-checks` has landed (A-5) and the goyang fork `replace` is in `go.mod` (A-12). Re-read at HEAD `loader.go`, `loader_structure.go`, `enum_assignment.go`, `validator.go`, `plugin/server/handler.go`, `rpc_register.go` and `config/yang/rpc_publish.go`. Reconcile every count above, including the D-2 A table, with `gopls references` across build tags; classify each production call site and each handler cross-call; resolve A-6 and A-8 to A-11. Re-run the D-6 probe over the new tree. Append the reconciled census to this section before any source edit.
2. **T3 first.** It is the leaf, and the other two read definitions. Write `TestArgDefConstructorRefuses`, `TestArgDefAccessorsDoNotAlias`, `TestZeroArgDefRefused` and probes P-5 and P-6, and watch them fail. Add the constructors and accessors, make the fields private, and migrate the lowering, `appendAnchored`, the MCP projection (D-7), every reader and every test literal in one commit. Run `TestCommandTreeBuildsFromEveryRegisteredModule`.
3. **T4, strict `DefaultLoader` (D-6), two commits.** First, write `TestYANGImportsFollowGoImports`, watch it fail on the 50 edges, add the blank imports, and run the tests of every package with a `DefaultLoader` path under the gate-free and full profiles; add any further discarded error found to the T4 table and fix it. Second, write `TestDefaultLoaderReportsEveryRegisteredFailure`, watch it fail, make `LoadRegistered` join every parse error and `DefaultLoader` return every error, and stop `service_web.go` discarding it. Update `yang-config-design.md` (best-effort no longer exists) in that commit.
4. **T1.** Write `TestResolveRefusesLoadAfterResolution`, `TestResolvedZeroValueIsABug`, `TestYANGContractBaselineRefusesFailedResolution` and probes P-1 and P-2. Introduce `yang.Resolved`, move the read operations, migrate every census consumer and its tests, fold the `usage.go` policy into the shared transition, and remove the `applyPatterns` compile-error branch. Update `yang-config-design.md` in this step.
5. **T2, chunks C-T2a to C-T2e, one commit each, in that order.** C-T2a: write `TestValidateArgsEmptyDefinitionsPassTokens`, the per-route tests and probe P-4; introduce the value, make every route R1 to R9 call `ValidateArgs` (D-3), add `cli-argument-refused-plugin-route.ci`; for each route remove its validation call, record the red, restore it; update `commands.md`. C-T2b to C-T2e: for each handler type, write its row of `TestHandlerTypesTakeValidatedArguments` and its P-3 case, watch them fail, change the type and every implementation, route and test call together, and delete the raw-args signature. C-T2e follows the rewriter decision and R-18. Update `commands.md` with each type.
6. **Reconcile.** Re-run the census for added or removed producers and handlers, delete the probe files after recording their errors here, and hand the long gates to the main thread: `./le go lint run`, `./le test unit all`, the `ui` functional suite and `./le verify worktree`. An independent review follows under `/ze-review`.

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
