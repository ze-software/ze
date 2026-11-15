# Spec: bgp-atomic-aggregate-export-rewrite

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Export policy can advertise a received ATOMIC_AGGREGATE route as a more-specific prefix on both forwarding rails |
| RFC gaps | RFC4271-5.1.6-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-15 |

Scope is a correctness repair in the existing BGP forwarding capability. This
skeleton records the demonstrated violation and the research required before
implementation. No producer fix or complete cross-family design is claimed.

## Task

A received route for `10.42.0.0/24` carries ATOMIC_AGGREGATE. An export member
returns a structurally valid raw UPDATE body with the same attributes and NLRI
`10.42.0.0/25`. Both the ordinary cached rail and the native route-server rail
advertise the /25. Their identical unmarked-route controls pass, proving the
member runs and the rewritten route reaches the live socket.

RFC 4271 Section 5.1.6 states: "A BGP speaker that receives a route with the
ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific
(as defined in 9.1.4) when advertising this route to other BGP speakers."
Section 9.1.4 states: "In particular, a route that carries the
ATOMIC_AGGREGATE attribute MUST NOT be de-aggregated." It defines more-specific
routes by the smaller destination set, represented by a longer prefix.

The policy rewrite facility is optional. The received-route MUST NOT is not:
once Ze receives the marked route, policy cannot authorize deaggregation.
Removing ATOMIC_AGGREGATE in a policy answer does not undo the received
condition. A guard that reads only the effective output's marker misses that
case. Preserve the received source condition and compare it with the effective
advertisement after all relevant policy transformations.

The root requirement remains partial in `rfc/short/rfc4271.md`. The earlier
tagged tests prove byte-preserving rebuilds and independent unmarked inputs;
they do not prove policy-rewrite safety, MP NLRI or stored replay.

## Demonstrated evidence

The main session ran
`TestRFC4271AtomicAggregatePolicyRewriteDoesNotDeaggregate` in
`internal/component/bgp/reactor/rfc4271_atomic_aggregate_policy_rewrite_test.go`.
Both `ordinary/unmarked-control` and `native-rs/unmarked-control` passed.
All four `atomic-aggregate` and `atomic-aggregate-stripped` subtests failed at
the decoded socket assertion: prefix length 25 exceeds the received length 24.

Both marker-retaining cases emitted this UPDATE body:
`0000001b4001010040020a02020000fde80000fde94003040a0000fe400600190a2a0000`.
The `400600` attribute is ATOMIC_AGGREGATE, and trailing `190a2a0000` is
`10.42.0.0/25`. Both marker-stripping cases emitted
`000000184001010040020a02020000fde80000fde94003040a0000fe190a2a0000`,
with no ATOMIC_AGGREGATE and the same forbidden /25. The received route was
marked in all four cases. These are bad-wire reds, not compile failures or
missing-delivery timeouts. The regression is untagged and earns no conformance
credit.

| Retained item | Purpose | Removal condition |
|---------------|---------|-------------------|
| `tmp/session/2026-11-14-210034/scratch/job-rfc4271-atomic-source-condition-b0e9f1c7.log` | Focused final matrix: both unmarked controls pass; all four marker-retaining/stripping cases emit the forbidden /25 on the two rails, with exact bytes | Remove when the focused result and eventual verified fix are retained in this spec's committed closure evidence |
| `tmp/session/2026-11-14-210034/scratch/job-rfc-policy-and-length-investigation-bbb4c83f.log` | Main session's passing RFC 8654 fatal/valid length units after the independently approved shared-fixture repair | Remove when those fixture results are retained in their owning length work's committed evidence or superseded by a focused retained run; atomic evidence is held by the focused log above |

The focused run demonstrates that stripping ATOMIC_AGGREGATE from policy
output does not prevent the leak. The required condition therefore has to be
bound to the received route, irrespective of the output marker.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md` - engine, filter and stored-route boundaries
  → Constraint: preserve the engine/plugin registration boundary and both in-process and external filter transports; the engine must not name a particular policy member.
- [ ] `docs/architecture/bgp/structural-forwarding.md` - transformed wire ownership and recipient dispatch
  → Constraint: output-byte replacement preserves ingress ownership. Source protocol facts must survive replacement too; source identity alone does not prove prefix containment.
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - provenance-sensitive and final-wire checks
  → Constraint: distinguish a rule bound to received attributes from a rule bound only to final output. Do not infer received ATOMIC_AGGREGATE from the final attribute set.
- [ ] `docs/architecture/core-design.md`, section 9 and Policy Filter Chain - receive, policy and final-write order
  → Constraint: preserve ordinary and native route-server pipeline order and existing policy error handling.
- [ ] `docs/architecture/encoding-context.md`, `docs/architecture/memory/lifetime-contracts.md` - research owed before selecting a provenance carrier
  → Constraint: received bytes are borrowed; retain source NLRI semantics across queues, splits and stored replay without keeping an expired receive-buffer slice.

### RFC Text and Summary
- [ ] `rfc/full/rfc4271.txt`, Sections 5.1.6 and 9.1.4; `rfc/short/rfc4271.md`, RFC4271-5.1.6-1
  → Decision: the received marked route binds every later advertisement of that route, including one whose policy output removes the marker.
  → Constraint: the new regression and its control cover legacy IPv4 only. Do not promote the whole requirement from partial until every applicable declared scope is independently proven.

## Current Behavior

**Diagnosis:** the symptom is a /25 announcement of a received marked /24.
`Reactor.runLibraryChain` accepts the canonical raw policy answer as
`modifiedPayload`; `runEgressPolicyChainASN4` returns it as `wireOverride`.
`reactorAPIAdapter.forwardUpdateSection` uses that override as `peerBaseWire`.
`reactorForwardRSSection` uses `rsBaseOverride` as its new base. The later
builder preserves the already-rewritten NLRI or honors an explicit NLRI
override, without comparing it with the received marked route's destinations.
Both final sockets therefore receive the forbidden /25.

The owning layer is forwarding transformation/admission with received-source
provenance. **[workaround]** Inspect only output ATOMIC_AGGREGATE, or prohibit
every policy edit to any marked UPDATE. The first loses the rule when policy
strips the marker; the second also rejects equal-prefix, attribute-only,
prefix-removal and less-specific results the MUST NOT does not forbid.
**[source]** Carry the received condition and source NLRI semantics to the
effective-output decision, at a boundary every applicable forwarding path
reaches. The carrier, cross-family comparison and rejection/withdrawal outcome
still need design and independent review.

**Source files read:**
- [ ] `reactor/filter_library.go::runLibraryChain`, `libraryCaller.CallFilter` - raw answer acceptance
- [ ] `reactor/filter_chain.go::decodeFilterRawOverride` - structural/canonical decoding does not compare source and output prefix sets
- [ ] `reactor/filter_ordered.go::runEgressPolicyChainASN4` - accepting raw result becomes a wire override
- [ ] `reactor/reactor_api_forward.go::forwardUpdateSection` - per-destination override replaces the effective base
- [ ] `reactor/forward_rs.go::reactorForwardRSSection`, `forward_rs_chain.go::rsBaseOverride` - native equivalent
- [ ] `reactor/forward_build.go::buildModifiedPayloadWithLimit` - direct NLRI override and unchanged-tail branches
- [ ] `reactor/forward_provenance.go::fwdProvenance` - existing transformation ownership carrier; its suitability for this rule is not established

Paths above are relative to `internal/component/bgp/`.

**Preserve:** unchanged marked NLRI, independent unmarked routes, permitted
policy results, existing withdrawal sections, exact ownership and ADD-PATH
identity, negotiated encoding and sizing, and the current filter transport.
Do not weaken the new regression or retag the existing byte-preservation proof
as proof of this missing admission.

## Existing Mechanisms

| Mechanism | Where | Current behavior | Disposition to research |
|-----------|-------|------------------|-------------------------|
| Received `WireUpdate` plus encoding context | Received cache and forwarding entry points | Holds the original route before export policy | Extend source provenance rather than trying to recover it from output |
| Export library chain | `filter_library.go`, `filter_ordered.go` | Returns typed edits or a canonical raw replacement | Preserve the optional policy facility; enforce the protocol condition around its effective result |
| Forward provenance and final ownership | `forward_provenance.go`, session writer | Maps transformed output back to received owners | Assess extension across direct, queued, split and recovery output; do not assume existing identity data includes source prefix semantics |
| Registered family codecs/splitters | `internal/core/bgp/nlri/nlrisplit` | Frames native NLRI under negotiated ADD-PATH | Reuse registered semantics; an MPLS label or path identifier is not an IP prefix length |
| Existing withdrawal synthesis | `forward_build.go`, forward withdrawal ownership | Converts withheld announcements while preserving prior ownership | Decide the correct refusal result and verify that a known prior advertisement does not remain stale |

## Data Flow

### Entry Point
Received UPDATE carrying ATOMIC_AGGREGATE; export filter replies with raw or
typed changes. The demonstrated path is received legacy IPv4, cached/general
or native-RS relay, policy override, final Session socket.

### Transformation Path
1. Capture the received ATOMIC_AGGREGATE condition and the source NLRI semantics.
2. Run the existing policy/transformation pipeline.
3. Compare the effective advertised destinations against their received source.
4. Admit permitted output, or apply the designed withholding/withdrawal result.
5. Preserve that decision and source fact through queues, split bodies and replay.

These are required properties, not an approved implementation sequence.

### Changed-Wire Census and Research Owed

The full writer/reader census is open. Before design, cover direct and queued
general forwarding, native route-server dispatch, raw and typed policy edits,
legacy plus MP sections, negotiated ADD-PATH, stored-route relay/replay,
withdrawal conversion, split UPDATEs and the final Session writers. Include
third-party filter transports and registered native family semantics. The
demonstrated output is one IPv4 case; do not declare other families defective
or safe without their own source trace and test.

No plugin envelope change is selected. If the source carrier crosses a plugin
boundary, add its complete changed-wire census and public contract pages
before implementation.

## Risks & Assumptions

| ID | Risk or open assumption | Validation required |
|----|-------------------------|---------------------|
| R-1 | Policy can remove the output marker | Demonstrated on both rails in the focused red matrix; the future guard must inspect the received condition, not just output |
| R-2 | A global builder check has already lost the original route | Trace each caller and preserve source provenance before replacing the effective base |
| R-3 | Prefix comparison treats path IDs, labels or native NLRI as CIDR bits | Test registered family semantics, mixed sections and ADD-PATH boundaries |
| R-4 | A guard prevents the leak but leaves a prior advertisement installed | Exercise an advertised original followed by a forbidden rewrite, and assert the designed recipient state |
| R-5 | A source buffer expires before the queued final check | Review ownership/refcounts and prove queued, split and stored replay paths |
| R-6 | A blanket clamp rejects permitted output or independent more-specific routes | Retain useful unmarked, equal-prefix, attribute-only, less-specific and withdrawal controls |

## Acceptance Criteria to Complete in Design

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Received marked /24, export raw answer makes it /25, marker retained or removed | Every applicable rail prevents the more-specific advertisement using the received-source condition | Safe preservation or suppression is permitted by this MUST NOT; the final refusal outcome and prior-owner withdrawal need explicit design |
| AC-2 | The same configured policy acts on the unmarked control | The legitimate configured rewrite still reaches the recipient | Independent unmarked more-specific routes must not be clamped merely to clear the marked-route red |
| AC-3 | Received marked route is kept equal, made less specific, has attributes changed, loses an NLRI or is withdrawn | Each permitted operation retains its intended useful behavior and correct recipient ownership | The requirement forbids deaggregation, not all changes |
| AC-4 | MP, native-family, ADD-PATH, mixed, queued, split or stored-replay output | The same source-bound condition is honored at each applicable output boundary | Applicability and comparison semantics must be established per registered family, without invented generic CIDR handling |
| AC-5 | RFC proof and audit artifacts | Each promoted claim has independent semantic review and a real named-unit producer-break red | Keep the root row partial while any applicable scope remains unmet or unproven |

## 🧪 TDD Test Plan

| Test / evidence | Current status | Next proof |
|-----------------|----------------|------------|
| `TestRFC4271AtomicAggregatePolicyRewriteDoesNotDeaggregate` | Untagged; both unmarked controls pass and all four marker-retaining/stripping cases emit bad wire on the two rails | Use the retained full matrix while designing the fix; no repeat is owed merely to reconfirm this output |
| Existing exact-NLRI relay proof pair | Bounded tagged proof retained | Preserve its scope and records; do not treat it as the policy-rewrite guard proof |
| MP, ADD-PATH, mixed, queued, split and stored replay cases | Not designed or implemented | Complete the census, add named cases, and retain separate per-rail discrimination where a single first failure could hide the sibling rail |
| Functional and interoperability proof | Not designed | Before asserting peer outcomes, read deciding FRR/BIRD source; add focused real-peer cases rather than a whole-tree run |

## Files and Implementation Work Still to Design

Candidate producer layers are the shared export decision, ordinary and native
forwarding, received provenance, stored-route carriers and final writers.
Do not start by editing `forward_build.go` alone: its effective base may already
be the policy output. Enumerate the actual file set, source lifetimes, all
family semantics, refusal diagnostics, architecture/API documentation,
functional/interop tests and exact owed commands before changing producers.

## Related Work and Boundaries

The existing `spec-bgp-refresh-reapplies-export-policy.md` owns pre-policy
attribute retention and refresh evaluation. Coordinate shared provenance and
replay carriers without importing that spec's larger reconciliation work here.
The RFC proof-debt spec owns historical records, not this newly demonstrated
wire defect. No existing ATOMIC_AGGREGATE fix owner was found before creating
this narrow release-fix skeleton.

The current task intentionally stops at the retained regression and this
owning skeleton, under the owner's permission to keep complex fixes red for
later review. This is open release correctness work, not an optional new
policy feature or completed RFC coverage.
