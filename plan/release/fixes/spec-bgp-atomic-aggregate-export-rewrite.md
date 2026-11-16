# Spec: bgp-atomic-aggregate-export-rewrite

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Export policy can advertise a received ATOMIC_AGGREGATE route as a more-specific prefix on both forwarding rails |
| RFC gaps | RFC4271-5.1.6-1 |
| Depends | - |
| Phase | D: accepted subset source retirement |
| Handoff | - |
| Updated | 2026-11-16 |

Scope is a correctness repair in the existing BGP forwarding capability. This
spec records the demonstrated violation and the approved restricted v1 design.
Scoped main-thread tests verify the admission and retirement repairs below.
Entry-point and canonical producer-break proofs remain open.

Current checkpoint: the main thread's
`job-atomic-preparation-settlement-final-8a1a060f.log` observes all seven
preparation/settlement roots pass under race (2.395s), including all eight
original route cases, cached/native callback unlock and replacement controls,
both AIGP replay rows, true reflection suppression and five healthy recipients.
The earlier `ec101c7d` run had two fixture failures: extended-send recipients
still had a 4096-byte writer buffer, and a filter reject was incorrectly treated
as suppression although it dispatches a withdrawal. Independently approved
setup corrections preserve the original result and socket assertions; main
formatted them in `a40e8494`. The first thirty-root affected run, `641b7b32`,
has 28 passing roots, including the new prequeue AIGP retry control, and two
failures. The stale queued/fitting row used a channel-only completion marker
which could overtake overflow. After independently approved replacement with
the existing overflow-aware barrier, `job-atomic-stale-barrier-race20-final-6a619e8c.log`
observes all twenty root executions and 180 cases pass under race (2.435s).
After both independently judged setup repairs,
`job-atomic-final-forward-regressions-green-641b7b32.log` observes all thirty
selected roots pass under race (3.380s), with no failed root. This is a separate
log from the earlier 28-PASS/two-FAIL run despite the shared hash suffix.
Native checks and entry-point proofs remain open; these scoped results do not
complete those obligations.

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
missing-delivery timeouts. The regression was untagged at that baseline. Its
reviewed narrow negative tag is now applied. The canonical
`job-atomic-rfc-rewrite-record-f3037485.log` observes all six clean cases pass,
then all four marked cases expose the forbidden /25 under the admission bypass
while both unmarked controls pass. The recorded credit is limited to this
negative legacy-IPv4 claim; the requirement remains partial.

| Retained item | Purpose | Removal condition |
|---------------|---------|-------------------|
| `tmp/session/2026-11-14-210034/scratch/job-rfc4271-atomic-source-condition-b0e9f1c7.log` | Focused final matrix: both unmarked controls pass; all four marker-retaining/stripping cases emit the forbidden /25 on the two rails, with exact bytes | Remove when the focused result and eventual verified fix are retained in this spec's committed closure evidence |
| `tmp/session/2026-11-14-210034/scratch/job-rfc-policy-and-length-investigation-bbb4c83f.log` | Main session's passing RFC 8654 fatal/valid length units after the independently approved shared-fixture repair | Remove when those fixture results are retained in their owning length work's committed evidence or superseded by a focused retained run; atomic evidence is held by the focused log above |

The focused run demonstrates that stripping ATOMIC_AGGREGATE from policy
output does not prevent the leak. The required condition therefore has to be
bound to the received route, irrespective of the output marker.

The main thread refreshed the regression after concurrent next-hop/fixture
changes: its retained `1b84` / `ba9` runs still show the four intended bad-wire
cases. The new restricted-admission matrix then ran before central guard
wiring in `d43e9738`: all sixteen source-refusal cases exposed either leaked
output or a missing old-source withdrawal, while six equal/subset/reordering
controls and two unmarked-input-adds-marker controls passed. These are
observed TDD reds supplied by the main thread, not an author-side test run.

## Required Reading

### Forwarding failure and negotiated-family retirement follow-up

The main thread observed the new two-root matrix fail in
`job-atomic-forward-failure-red-3b08a959.log` (1.690s). Both source-owned A
retirements were missing after accepted B exceeded destination encoding limits;
both foreign-owner controls passed. All four small unsupported-family controls
passed, while all four large BGP-LS sibling cases failed encoding the indivisible
5021-byte key and suppressed useful IPv6 B. The policy acceptance and exact
owed-A withdrawal preconditions passed. No compile or race failure was reported.

The repair stages the accepted group before survivor construction, then replaces
that same pending slot on successful survivor materialization. Cleanup alone
does not report the failed survivor delivered. Native handback releases its
staged slot. Shared materialization projects removal sections through actual
negotiated capability support before encoding; final ownership remains mandatory.
The new `filter_atomic_forward_failure_test.go` contains the two socket roots,
foreign-owner protection, useful later output, small/large native siblings,
mixed legacy projection and no invented EOR. The main thread observed both
roots and all twelve cases pass in
`job-atomic-forward-failure-green-3b08a959.log` (2.505s), then pass again in the
affected callback matrix. Bounded independent source review found no remaining
concrete defect in this repair. The later stale-splitting repair has its own
focused evidence below; entry-point proof obligations remain open.

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

**Phase D baseline diagnosis:** the restricted admission fixed the original
forbidden rewrite, but its accepted subset facility did not retire removed
source routes. At that baseline, `evaluateLibraryChain` and `recordNLRIRewrite`
materialized kept NLRI without their removed complement. `notifyOneMessage`
and both forwarding rails used only the retained body. The main thread's
`job-atomic-final-boundaries-red-af59d2f0.log` shows six exact
old-source withdrawal failures: four cached/native export subset or EOR cases
and two import cases. Useful B must remain, while old A leaves. This is a
source-retirement defect in the supported facility, not an RFC permission to
drop every marked route or weaken its old-state assertions.

**[workaround]** Refuse every strict subset or withdraw the whole original
route set; that loses supported useful output. **[source]** Compute missing
source native identities from the original input and final effective result,
including original withdrawn siblings omitted from the result, then apply
owned withdrawal continuations for admission before surviving output/EOR. An
active RIB recovery owner can then settle the key asynchronously by withdrawal
or a permitted replacement; unrelated output does not complete that decision.
Output identities
already announced or withdrawn cover the same source key, so no continuation
removes a useful survivor. Two MP families require two separate bodies. The
source-key helper and internal result/writer plumbing implement that repair
without a public ancestry envelope. The six baseline failures now pass in the
main thread's affected callback matrix.

**Stale-splitting baseline diagnosis:** an accepted subset lost its source
retirement when the survivor could not split. `sendUpdateOwnedWithSplit` calls
the splitter before its first send callback, and the queued sender likewise
transfers removals only on its first dispatch. A valid indivisible attribute
can exceed the destination size before either callback runs. The main thread's
`job-atomic-stale-encoding-red-606e6df4.log` shows missing A withdrawals for a
local owner and a current exact receipt on the direct path, and a current
exact receipt on the queued path. Five foreign/newer-owner and fitting-output
controls pass. Policy acceptance, exact owed-A identity and the actual
`ErrAttributesTooLarge` failure all precede those assertions.

The owning layer is stale batch materialization in
`reactor_api_batch.go::sendBatchUpdatePreFiltered`. **[workaround]** Reject all
accepted subsets, losing useful B. **[source]** Retire an unconsumed removal
group through the existing withdrawal sender and captured fences after a
route-scoped split failure. Preserve the original failure, join cleanup
failure, and never repeat a transferred or partly written group. The main
thread's `job-atomic-stale-and-fixtures-green-230bf916.log` now verifies all
nine encoding-failure cases, including the added cleanup-write-failure control.
The earlier stale normalization and useful-sender roots also pass. Independent
source review reports no remaining concrete defect in this repair. Native
producer-break and daemon proofs remain separate obligations.

**Original diagnosis:** the symptom was a /25 announcement of a received marked /24.
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
**[source]** Refuse marker removal and new marked native keys at the shared
import/export chain result boundary. This keeps the received condition and
surviving destinations in existing cache/store/replay bytes. The approved v1
below deliberately restricts optional raw rewriting rather than introducing
a public ancestry carrier.

**Source files read:**
- [ ] `reactor/filter_library.go::runLibraryChain`, `libraryCaller.CallFilter` - raw answer acceptance
- [ ] `reactor/filter_chain.go::decodeFilterRawOverride` - structural/canonical decoding does not compare source and output prefix sets
- [ ] `reactor/filter_ordered.go::runEgressPolicyChainASN4` - accepting raw result becomes a wire override
- [ ] `reactor/reactor_api_forward.go::forwardUpdateSection` - per-destination override replaces the effective base
- [ ] `reactor/forward_rs.go::reactorForwardRSSection`, `forward_rs_chain.go::rsBaseOverride` - native equivalent
- [ ] `reactor/forward_build.go::buildModifiedPayloadWithLimit` - direct NLRI override and unchanged-tail branches
- [ ] `reactor/forward_provenance.go::fwdProvenance` - existing transformation ownership carrier; its suitability for this rule is not established

Paths above are relative to `internal/component/bgp/`.

**Preserve:** unchanged marked NLRI, independent unmarked routes, supported
policy results, existing withdrawal sections, exact ownership and ADD-PATH
identity, negotiated encoding and sizing, and the current filter transport.
Do not weaken the new regression or retag the existing byte-preservation proof
as proof of this missing admission.

## Existing Mechanisms

| Mechanism | Where | Current behavior | Disposition to research |
|-----------|-------|------------------|-------------------------|
| Received `WireUpdate` plus encoding context | Received cache and forwarding entry points | Holds the admitted route before export policy | Reuse unchanged; import result admission preserves the received marker/destination invariant before publication |
| Import/export library chain | `filter_library.go`, `filter_ordered.go` | Returns typed edits or a canonical raw replacement | Extend the common result boundary with marked-input subset/marker admission; restrict unsupported optional raw rewriting |
| Forward provenance and final ownership | `forward_provenance.go`, session writer | Maps transformed output back to received owners | Reuse existing source withdrawal and ADD-PATH identity unchanged across direct, queued, split and recovery output |
| Registered family codecs/splitters | `internal/core/bgp/nlri/nlrisplit` | Frames native NLRI under negotiated ADD-PATH | Reuse registered semantics; an MPLS label or path identifier is not an IP prefix length |
| Existing withdrawal synthesis | `forward_build.go`, forward withdrawal ownership | Converts withheld announcements while preserving prior ownership | Reuse `OutcomeWithdraw`; verify a known prior source advertisement is retired while foreign owners survive |

## Data Flow

### Entry Point
Received UPDATE carrying ATOMIC_AGGREGATE; export filter replies with raw or
typed changes. The demonstrated path is received legacy IPv4, cached/general
or native-RS relay, policy override, final Session socket.

### Transformation Path
1. Retain the original chain input for the duration of policy evaluation.
2. Run the existing policy/transformation pipeline.
3. On a marked original input, require effective announcements to retain the
   valid marker and use only original family/native/path keys.
4. Admit supported output, or return source `OutcomeWithdraw` on refusal.
5. Carry the admitted bytes through existing queues, splitting and replay;
   repeat the same admission on each later policy run.

### Changed-Wire Census and Research Owed

The full writer/reader census is open. Before design, cover direct and queued
general forwarding, native route-server dispatch, raw and typed policy edits,
legacy plus MP sections, negotiated ADD-PATH, stored-route relay/replay,
withdrawal conversion, split UPDATEs and the final Session writers. Include
third-party filter transports and registered native family semantics. The
demonstrated output is one IPv4 case; do not declare other families defective
or safe without their own source trace and test.

No plugin envelope changes are needed for the approved restricted v1 below.
The Phase A trace establishes why an export-only check cannot close this work:
import policy could erase the marker before cache/store publication. V1 refuses
that transformation at import, preserving the condition in existing bytes.

### Phase A: verified provenance and admission findings

| Requirement / mechanism | Audit status | Producing source and finding |
|-------------------------|--------------|------------------------------|
| Raw policy answer reaches both rails | Done, existing facility | `filter_library.go::libraryCaller.CallFilter` canonicalizes `Raw`; `runLibraryChain` returns it before typed Apply. `filter_ordered.go::runEgressPolicyChainASN4` exposes accepting output as `wireOverride`; ordinary `forwardUpdateSection` and native `reactorForwardRSSection` replace their destination base with it. Canonical framing is not source containment. |
| Typed NLRI changes | Done, existing subset facility | `filter_library.go::libraryNLRIOps` strips families or drops selected source NLRIs, using `dropNLRI` and `recordNLRIRewrite`; the traced typed facility does not create arbitrary new prefixes. Attribute edits still must preserve received-source facts. |
| Original receipt survives import changes | Missing | `reactor_notify.go::notifyOneMessage` builds its first `RawMessage` from received wire, then replaces `wireUpdate`, `msg.RawBytes`, `msg.WireUpdate` and `msg.AttrsWire` on `modifiedPayload`. It subsequently publishes/caches that changed body. A valid import answer can therefore remove ATOMIC_AGGREGATE before an export check reads the cache. |
| Original source destinations in existing ownership manifest | Missing | `forward_provenance.go::prepareFwdProvenance` snapshots identity only. Its sections enumerate transformed NLRI; the original UPDATE is consulted for received-withdrawal keys. `writePath` returns source peer, received path identifier and ADD-PATH framing, not original destination containment. It also normally skips unframed, unsynthesized traffic. |
| Queue/split identity lifetime | Done, existing mechanism | Both forwarding rails call `prepareFwdProvenance` after body building and before dispatch. `fwdProvenance.spans` binds body ordinals; `forward_pool.go::fwdBatchHandler` retains the item as `Session.sentForward`; `releaseItem` releases its manifest and buffers. Any new original-source snapshot must share or extend this owned lifetime, never borrow the session's reused input bytes. |
| Stored receive replay original condition | Missing | `rpc.StoredRoute` carries source, family, attributes, NLRI, next hop, message ID, policy epoch and framing, but no pre-import atomic-source fact. `reactor_api_relay.go::buildRelayUpdate` reconstructs a new `ReceivedUpdate` from those stored fields; `relayRunLen` coalesces matching runs. Reconstructed attributes alone cannot recover a marker import removed. |
| Final common write boundary | Partial | `session_write.go::writeUpdateBody` is shared by ordinary/direct forwarding, queued bodies and rebuilt updates. It runs final export, AS-zero, AS_SET and AIGP admission, then source ownership and PATHS-LIMIT handling. It has `sentForward` identity but no original atomic destination basis. |
| Retire a previously held source route | Done, existing mechanism to extend | A semantic export reject in `forwardUpdateSection` clears the override and sets withdrawal; native `reactorForwardRSSection` retains its original base when the chain rejects. Both synthesize source withdrawals through `fwdWithdrawal.of`. `session_ownership.go::adjOutWrite.allow` admits them only for the matching held source/path. A failed chain instead drops the work and may leave the earlier advertisement held. |

All reactor paths in this table are relative to
`internal/component/bgp/reactor/`. This is a source audit, not new execution
evidence; the demonstrated socket reds above remain the observed result.

The source-owned refusal result should reuse the semantic-reject/withdrawal
contract, with an explicit diagnostic for the offending transformation. It
must withdraw the **source route**, not simply convert the forbidden final
output to a withdrawal: converting a policy-produced /25 would leave a held
source /24 installed. `Session.writeASZeroAsWithdrawal` is a useful projection
and ownership pattern, but its final-output NLRI basis is insufficient here.

### Approved v1: restrict marked-route policy transformations

The owner's completion request explicitly permits disabling optional features
to help the release. Under that authorization, v1 disables raw NLRI
readdressing and marker removal for a message-policy chain whose original
input carries valid ATOMIC_AGGREGATE. This is a stronger product restriction
than RFC 4271's prohibition: the RFC does not forbid less-specific output or
every disjoint rewrite. Those optional marked-route rewrites are unavailable
in v1; a public ancestry API and a parallel original-byte store are unnecessary.

Every accepting modified chain result is admitted only when its announcements
are an exact native-key subset of the original chain input, including family
and received ADD-PATH identity, and its valid marker remains present. Attribute
changes, reordering and subset removal remain supported. Pure withdrawals and
EOR need no marker because they advertise no route. Unmarked original input
retains its existing rewrite facility, including when a result adds the marker.

`runLibraryChain` applies the same result guard to raw and typed answers. The
guard uses `libraryIntentContext.chainInput` when present, otherwise `payload`;
final judges and the stale rail therefore compare against their original
chain input rather than a member's intermediate rewritten body. It returns
`OutcomeWithdraw` for a refusal: this deliberately uses `ingressChainWithdrawal`
on import and the existing source-owned withdrawal on export, rather than a
failed-member drop or an ordinary reject under a final-accept import policy.

The shared helper contract is
`atomicAggregatePolicyAllowed(input, output []byte, ctxID bgpctx.ContextID) (bool, error)`.
It traverses registered native framing and destination keys; it does not
interpret a native family's length byte as CIDR bits. Membership uses bounded
pooled scratch for one UPDATE, avoiding quadratic pairwise comparisons.

The in-process slot already has a read-only input contract:
`pkg/plugin/rpc/filter_slot.go::FilterSlotHandler` states, "The evaluator never
writes into route.Raw()." The existing HEAD contract was verified in source,
with no working-tree change to that file. `CallFilterSlot` copies returned raw
answers for ownership after evaluation. The guard therefore borrows its
original input synchronously under that existing contract; it does not claim
isolation from an internal evaluator that violates it. No defensive whole-input
copy or public API change is selected.

The invariant is inductive: import cannot erase a received marker or introduce
new marked destinations before publication; each later export/re-advertisement
chain preserves that property. Existing cache/store/replay bytes carry the
marker and surviving destinations. Encoding, splitting and final normalization
preserve those announcements or withdraw them; no new plugin envelope or
pre-import snapshot is selected. The broader source-carrier proposal researched
above is therefore unnecessary for this restricted facility.

### Implementation Phases

1. Add the pure native-key/marker admission helper and unit matrix, preserving
   all registered families and negotiated ADD-PATH identity. Exact retained
   siblings pass; every new marked key is refused, including less-specific,
   disjoint and ambiguous raw rewrites. Keep the current socket red unchanged.
2. Apply the shared result guard to all accepting modified library results,
   covering import, export, final judges, stale runs and both filter transports.
   Supply a specific diagnostic and preserve source-owned withdrawal semantics.
3. Add useful marked controls, import erasure prevention, prior-owner withdrawal,
   MP/native/ADD-PATH, mixed, split, queued and stored replay integration cases,
   then daemon and focused interoperability proof. Do not promote the root RFC
   row beyond the actual independently discriminated scope.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Original input | Final judges and stale second passes use `intent.chainInput`; an originally unmarked input remains unrestricted even if a member adds the marker |
| Slot contract | In-process evaluators obey the existing `FilterSlotHandler` prohibition on writes to `route.Raw()`; do not claim hostile in-process isolation or add an unapproved input copy |
| All result forms | Raw early returns and typed modifications both pass the same accepting-result guard; unchanged bodies remain allocation-free |
| Useful survivors | Marked equal/subset/attribute-only/reordered output and explicit withdrawals survive; controls reach the socket |
| Marker lifetime | Import cannot strip a valid marker before cache/store publication; replay retains it without a new envelope |
| Withdrawal | Refusal is `OutcomeWithdraw`, retires the held source route, preserves foreign owners and received ADD-PATH identity |
| Native families | Every registered family uses its framing/key callback; malformed or unsupported data fails closed without panicking |
| Cost and ownership | Scratch is bounded and pooled; no whole received UPDATE is copied per stored prefix, and no new async borrowed slice is retained |
| Disclosure | Docs identify the stronger v1 restriction as a product choice, not an RFC prohibition on less-specific output |

### Documentation correction implemented

`docs/architecture/bgp/egress-attribute-rules.md`, "How provenance is
established", now distinguishes the original received bytes from the
post-import admitted forwarding baseline, with source anchors on
`notifyOneMessage` and `applyFactsMED`. The eight documentation pages listed
below describe the restricted v1, source withdrawal and its completion boundary.
The final author-side comparison against the producing source is clean within
these changed sections; this is not an independent review stamp. Mechanical
STE/documentation checks remain pending in the checklist below.

## Documentation Update Checklist

| Page | Required current behavior | Author update | Closure verification |
|------|---------------------------|---------------|----------------------|
| `docs/architecture/core-design.md` | Shared accepting-result admission, original `chainInput`, source withdrawal, marked subset and unmarked survivors | Written with `runLibraryChain` / helper anchors | Final author source comparison clean; mechanical STE check pending |
| `docs/architecture/bgp/egress-attribute-rules.md` | Received versus post-import provenance; stronger v1 restriction; native identity; withdrawal ownership; historical diagnostic | Written; each new factual paragraph has producing source anchors | Final author source comparison clean; mechanical STE check pending |
| `docs/architecture/bgp/structural-forwarding.md` | Source-removal admission before kept output, asynchronous RIB-owned decisions, captured-session fatal preparation settlement, one destination item/fence/count and child-only byte/provenance lifetime | Written with helper, grouped writer, ownership, recovery and post-unlock settlement anchors | Final author source comparison clean after bounding the fatal-preparation paragraph to received forwarding; mechanical STE check pending |
| `docs/architecture/api/process-protocol.md` | Raw and typed results on both transports share admission; restriction does not use `on-error`; no envelope fields added | Written with guard/helper anchors beside the new paragraphs, retaining type/raw-answer anchors on their own original paragraph | Final author source comparison clean; mechanical STE check pending |
| `docs/guide/bgp-policy.md` | Useful marked/unmarked controls; new marked destinations/marker removal unavailable; source withdrawal; diagnostic and exact editor action | Written with helper/chain and diagnostic anchors | Final author source comparison clean; mechanical STE check pending |
| `docs/architecture/testing/interop.md` | FRR scenario's exact final absences and useful routes; an unrelated fence does not complete asynchronous source recovery | Written with checker, helper, ownership and recovery anchors; describes the authored assertion contract | Final author source comparison clean; mechanical STE check, live FRR baseline and separate admission/removal producer-break results remain owed |
| `docs/architecture/api/commands.md`, `docs/architecture/update-building.md` | Quiesce waits for current-session cold recovery and pending takeover flush; later unrelated output is not a recovery barrier | Written with `syncOwed`, `routeDecisionOwed`, ownership and recovery anchors | Final author source comparison clean against current-session owed/pending checks and observed held, released, reset and stopped-worker controls; mechanical STE checks pending |

This table is the owning closure checklist, not a claim that a documentation
gate or the final independent read has passed. Source and unrelated
documentation changes from other sessions remain outside these edits.

## Risks & Assumptions

| ID | Risk or open assumption | Validation required |
|----|-------------------------|---------------------|
| R-1 | Policy output can erase the condition before a later chain/store reads it | The shared result guard reads original `chainInput` and refuses marker removal on marked announcements at import and export; verify import/publication and final-judge controls |
| R-2 | A builder sees a policy-replaced effective base rather than the chain's original input | Admission runs at the common chain-result boundary before the forwarding base is replaced; verify raw/typed accepting exits plus final/stale invocations, without adding an original-byte carrier |
| R-3 | Prefix comparison treats path IDs, labels or native NLRI as CIDR bits | Test registered family semantics, mixed sections and ADD-PATH boundaries |
| R-4 | A guard prevents the leak but leaves a prior advertisement installed | Exercise an advertised original followed by a forbidden rewrite, and assert the designed recipient state |
| R-5 | An async carrier borrows the guard's source bytes past their lifetime | The guard consumes original input synchronously under the existing read-only slot contract; cache/queue/split/store retain admitted bytes through their existing ownership. Verify queued, split and cache-expired stored replay cases |
| R-6 | A blanket clamp rejects useful supported output or independent unmarked routes | Retain unmarked, equal/subset, attribute-only, reordering and withdrawal controls; explicitly disclose that v1 disables new marked native keys, including less-specific output |
| R-7 | Accepted subset/EOR output leaves excluded old source routes installed, or a later policy pass retains an intermediate removal set | Compute retirement from original input versus final effective keys; preserve omitted original withdrawn siblings and useful survivors; use one grouped source decision/fence and monotonic already-judged import continuations |
| R-8 | Stale survivor splitting fails before the first writer or queue callback transfers its accepted removals | Verify direct and queued cleanup after the observed three missing-A failures; preserve exact-receipt, foreign/newer-owner and replay fences, the original split error, and useful fitting output |
| R-9 | Quiesce reports success while the current destination still owes cold recovery or a takeover flush | Read the current session's `adjOut.owed` and `adjOut.pending` without nested peer/table locks; verify held work, completed flush, reset and stopped-worker controls before claiming the barrier complete |
| R-10 | Hard removal preparation failure leaves an old advertisement, closes a replacement or deadlocks through the FSM callback | Collect exact captured sessions separately from ordinary results; close with notified Cease/Out of Resources after source validation and AIGP dispatch unlock; preserve healthy destinations and exclude fatal captures from final delivery counts |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Received marked /24, export raw answer makes it /25, marker retained or removed | Every applicable rail refuses the output and admits removal of any held source route through existing ownership; an active RIB owner settles its withdrawal or permitted replacement asynchronously | Unknown or foreign-owned prior advertisements survive; the refused source cannot withdraw another owner's route |
| AC-2 | The same configured policy acts on the unmarked control | The legitimate configured rewrite still reaches the recipient | Independent unmarked more-specific routes must not be clamped merely to clear the marked-route red |
| AC-3 | Received marked route keeps exact native destinations, changes attributes, reorders or removes NLRI, or becomes a withdrawal | Each supported operation retains its useful behavior and correct recipient ownership | V1 intentionally disables all new marked native keys, including less-specific/disjoint raw rewrites, and marker removal; these are product restrictions stronger than the RFC |
| AC-4 | MP, native-family, ADD-PATH, mixed, queued, split or stored-replay output | The same source-bound condition is honored at each applicable output boundary | Applicability and comparison semantics must be established per registered family, without invented generic CIDR handling |
| AC-5 | RFC proof and audit artifacts | Each promoted claim has independent semantic review and a real named-unit producer-break red | Keep the root row partial while any applicable scope remains unmet or unproven |
| AC-6 | Import policy removes the marker or introduces a new NLRI before publication | Refuse that result with source withdrawal, so cache/store/replay never acquire an erased condition or changed marked destination | Equal, attribute-only, reordered and subset output remain useful; pure withdrawal/EOR may omit the marker because no route is advertised |
| AC-7 | An accepted marked A+B-to-B subset reaches a stale sender, then B fails normalization or splitting, directly or behind queued forwards | Admit A's removal under captured owner and replay fences, preserving any RIB-owned decision; report B's original failure, join cleanup failure, and emit neither a synthetic EOR nor duplicate retirement | Foreign-owned A and a newer same-source receipt survive; a permitted replacement can win recovery, while fitting B and independent routes remain useful |
| AC-8 | Quiesce sees held cold recovery or an unflushed local takeover for the current destination session | Keep the barrier pending until the decision and writes settle; cancellation reports outstanding work, and reset clears old-table decision debt while queued forwards still drain | An idle worker does not hold the barrier; stopping a worker alone cannot hide live debt, and a valid takeover or alternate remains advertisable |
| AC-9 | An accepted source-removal group cannot be prepared for its captured destination | Close that exact session with notified Cease/Out of Resources after forwarding/validation locks release, with no forged selective withdrawal or success count for the fatal capture | Small and unnegotiated-family cases keep their existing useful output and ownership; other destinations and a replacement session survive. Closing the failed session clears all its routes through normal BGP procedures. This is the selected conservative product contract, not a claim that an unheld key was owned or that RFCs require this particular fallback |

## 🧪 TDD Test Plan

| Test / evidence | Current status | Next proof |
|-----------------|----------------|------------|
| `TestRFC4271AtomicAggregatePolicyRewriteDoesNotDeaggregate` | Native `job-atomic-rfc-rewrite-record-f3037485.log` observes all six clean socket cases pass, then a marked-input admission bypass exposes forbidden /25 output in all four marked cases while both unmarked controls pass; the canonical negative record now accompanies the two older relay covers | Commit the unchanged reviewed tag with its observed canonical record and bounded annotation; preserve all assertions and keep the requirement partial |
| Existing exact-NLRI relay proof pair | Bounded tagged proof retained | Preserve its scope and records; do not treat it as the policy-rewrite guard proof |
| `TestAtomicPolicyAdmissionRefusalWithdrawsSource`, useful/unmarked/import controls | All refusal and useful-output cases pass under race in `708accf1` | Preserve both sides of the condition; daemon and native interoperability proof remain separate |
| Native helper families, identity, mixed, malformed, pool reuse, forced hash collision and long keys | All named helper roots pass under race in `708accf1`, after the size-class repair | Native-key helper tests do not replace socket or daemon proof |
| `TestAtomicPolicyAdmissionStoredReplayOutlivesCache`, `TestAtomicPolicyAdmissionAddPathZeroUsesReceivedIdentity`, `TestAtomicPolicyAdmissionQueuedSplitSurvives` | Cache-expired stored replay, received ADD-PATH zero, queued and split output pass under race in `708accf1` | Preserve per-rail source identity and lifetime observations |
| `TestAtomicPolicyAdmissionExplicitWithdrawalRemainsUseful`, `TestAtomicPolicyAdmissionEORRemainsUseful` | Both corrected fixture-context roots pass under race in `708accf1` | Preserve the useful withdrawal/EOR assertions; the earlier context correction was test maintenance |
| Normal allocation controls | All four roots pass in `job-atomic-normal-allocation-current-6c6d42fa.log`: small and 4097-route admission use zero warmed allocations; no-removal comparison uses zero; owned removal stays at two allocations | Preserve normal-build oracles; race instrumentation is not allocation evidence |
| `TestAtomicAggregatePolicyDiagnostic` | Event and bounded historical-report root passes under race in `708accf1` | Preserve the operator diagnostic and editor action through final source review |
| `TestAtomicPolicyAdmissionRawSubsetRetiresRemovedSource`, `TestAtomicPolicyAdmissionImportRawSubsetRetiresRemovedSource` | All six export/import prior-held subset or empty-subset cases pass under race in `708accf1`, following the actual `af59d2f0` missing-withdrawal baseline | Keep useful B, original withdrawal siblings, native identity, ownership, ordering and single-destination accounting |
| `TestAtomicRetirementOriginFailureCommitsRemoval`, `TestAtomicRetirementStaleFailureKeepsRemovals` | Both normalization/final-admission failure roots pass under race in `708accf1`, including foreign/newer ownership and joined cleanup-error controls | Their separate native producer-break walks remain owed; these roots do not cover pre-callback stale split failure |
| `TestAtomicForwardFailureRetiresSource`, `TestAtomicForwardRemovalNegotiatedFamilies` | Both roots and all twelve cases pass in `job-atomic-forward-failure-green-3b08a959.log`, then in `708accf1`, after the real `3b08a959` red | Preserve source-owned cleanup, foreign owners, useful negotiated siblings, honest failed-survivor counts and no EOR |
| `TestAtomicRetirementStaleEncodingFailure` | Actual `606e6df4`: three missing-A failures and five passing controls; all nine repaired cases pass in `230bf916`. Later `641b7b32` exposes an early channel-marker assertion in queued/fitting only. After independently approved use of the existing overflow-aware barrier, `job-atomic-stale-barrier-race20-final-6a619e8c.log` observes twenty root executions and all 180 cases pass under race (2.435s); the root also passes in the separate thirty-root `job-atomic-final-forward-regressions-green-641b7b32.log` | All split-error, owner/fence, exact retirement, useful fitting output and no-EOR assertions remain; the earlier same-hash non-final repeat log is a compile failure, not test evidence. Native and entry-point proofs remain separate |
| Affected callback regression matrix | `708accf1` ran 79 roots under race: 77 pass and two fail; the passing scope contains 43 `TestAtomic` roots plus the original rewrite root. Both failed RFC8092/8669 roots now pass in `230bf916`, including all twelve RFC8669 cases | Keep the independently judged ASN4 fixture corrections distinct from the atomic product repair; no old input, assertion or tag was weakened |
| `TestAtomicRetirementRecoveryCanFollowUnrelatedFence` | Both active-RIB rows pass in `job-atomic-recovery-and-quiesce-red-dadbc079.log`: B and an unrelated fence pass while A's lookup is held; release withdraws A or installs a permitted alternate protected from the old source | Preserve exact source-removal admission and eventual decision; never treat the first unrelated wire fence as recovery completion |
| Four `TestDrainPeerSync` recovery roots and existing peer-sync controls | `dadbc079` exposes two false-success barrier failures; all four repaired roots pass in `46d68d65`. After the independently approved forward-disposal setup correction, `job-atomic-stopped-recovery20-final-099c84e0.log` observes twenty stopped-worker passes with no failure (13.530s), and `job-atomic-peer-sync-final-regressions-87fe9053.log` observes all nine selected roots pass (9.626s) | Preserve the actual cold-recovery, pending-flush, reset, stopped-worker, replay, policy-replay, channel/overflow and named-error assertions; these results do not supply daemon or interoperability proof |
| `TestAtomicPreparationFailureRetiresDestination` | Historical `job-atomic-preparation-failure-red-f6a26b94.log` observes four large-key live-obsolete-session failures and four passing small controls, with actual encoding-error preconditions. All eight repaired cases now pass under race in `job-atomic-preparation-settlement-final-8a1a060f.log` (seven selected roots, 2.395s) | Preserve all eight route assertions and legal extended-receive capability through the affected regression run; this result is not daemon or interoperability proof |
| Preparation settlement controls | All six supplemental roots pass in `8a1a060f`, covering actual callback TryLock, accepting validation, replacement session, inherited ordinary error, true reflection suppression, late-fatal receipts, five useful recipients and both AIGP replay rows. Historical `ec101c7d` had two fixture failures, corrected only after independent native approvals for writer-buffer negotiation and true suppression setup; all original outcome/socket assertions remain | Affected forwarding/AIGP/validation/native-RS/stale regressions now pass in the separate thirty-root GREEN below; native checks remain. The injected replay bound proves settlement plumbing only, not an RFC-valid negotiated limit or natural giant retained route |
| `TestAtomicAIGPReplayPrequeueFailureCanRetry` and affected regression selection | The new direct prequeue-error rollback control passes, requiring no false queued/decided result and actual AIGP111 output after facts recover at the same revision. The first `job-atomic-final-forward-regressions-641b7b32.log` has 28 PASS/two FAIL roots. After both independently judged setup repairs, separate `job-atomic-final-forward-regressions-green-641b7b32.log` observes all thirty roots PASS under race (3.380s), no FAIL | Keep the exact selected scope distinct from broader checks, native producer-break and daemon/interoperability evidence; those obligations remain open |
| Draft daemon fixture | Earlier prerequisite repairs reached a standalone pass; stress invocation 77 failed an assertion that demanded A's asynchronous recovery before the first unrelated fence. The held-RIB test establishes that this order is not required, and independent judgment approved retaining every exact observation within the original four-frame, fifteen-second bounds | Run the corrected focused draft, fresh 80-iteration stress and retirement-bypass proof before promotion; no completed stress acceptance is claimed |
| Native interoperability and discrimination | FRR scenario, checker and separate admission/removal cuts are written; no live FRR baseline or producer-break result is recorded here | Run the native scenario and separate semantic break proofs; keep the RFC requirement partial |

The result logs above live in `tmp/session/2026-11-14-210034/scratch/`.
They are retained only as the smallest TDD/result evidence while final
verification and closure are unresolved; remove superseded run output once
the owning committed closure records its bounded results. Keep observed
baseline failures distinct from later named passes. A written repair or an
approved tag does not supply the missing runtime or producer-break result.

## Files to Modify / Create

- `internal/component/bgp/reactor/filter_library.go` - guard every accepting
  modified result while preserving original `intent.chainInput` and withdrawal semantics.
- `internal/component/bgp/reactor/filter_ordered.go`, `stale_readvertise_chain.go`,
  `egress_inject_filter.go`, `reactor_api_batch.go`, `session.go`,
  `session_write.go`, `policy_dryrun_library.go` - carry the final original-source
  removal result through accepted import, origination, stale and display paths.
- `internal/component/bgp/reactor/filter_atomic_subset.go` and its semantic and
  normal-allocation tests - bounded native original-minus-final withdrawals.
- `internal/component/bgp/reactor/forward_policy_removal.go`, `forward_pool.go`,
  `forward_body.go`, `forward_bucket.go`, `forward_rs.go`, `forward_rs_direct.go`,
  `reactor_api_forward.go`, `reactor_notify.go` - one destination-owned group,
  per-child existing provenance, lifetime and import continuation publication.
- `internal/component/bgp/reactor/filter_atomic_aggregate.go` and
  `filter_atomic_aggregate_test.go` - pure registered native-key/marker helper
  and its bounded cost/family/input matrix.
- `internal/component/bgp/reactor/filter_atomic_diagnostic.go` and
  `filter_atomic_diagnostic_test.go` - declared transient diagnostic, bounded
  report history and exact configuration-editor action.
- `internal/component/bgp/reactor/rfc4271_atomic_policy_admission_test.go` -
  source/recipient integration matrix; leave the original regression unchanged.
- `internal/component/bgp/reactor/filter_atomic_retirement_pipeline_test.go`,
  `filter_atomic_trace_test.go`, `filter_atomic_stale_failure_test.go`,
  `filter_atomic_forward_failure_test.go`,
  `filter_atomic_stale_encoding_failure_test.go` - entry-point source retirement,
  owner/fence/accounting, display and failed-survivor controls.
- `internal/component/bgp/reactor/filter_atomic_recovery_order_test.go` - active
  RIB selection after unrelated output, including a permitted alternate.
- `internal/component/bgp/reactor/peer_sync_owed.go`,
  `peer_sync_recovery_test.go` - current-session cold-recovery and pending-write
  debt in the existing peer-sync barrier; no new worker, registry or session state.
- `internal/component/bgp/reactor/forward_result.go`, `forward_validation.go`,
  `forward_aigp.go`, `aigp_readvertise.go`,
  `filter_atomic_prepare_failure_test.go`,
  `filter_atomic_prepare_settlement_test.go` - per-call terminal captures,
  post-unlock settlement, surviving delivery counts and their bounded controls.
- New functional fixture and draft under `test/draft/plugin/`, owned by the
  entry-point test phase; promote only after focused successful proof.
- `docs/architecture/core-design.md`,
  `docs/architecture/bgp/egress-attribute-rules.md`,
  `docs/architecture/bgp/structural-forwarding.md`,
  `docs/architecture/api/process-protocol.md`, `docs/guide/bgp-policy.md`,
  `docs/architecture/testing/interop.md`, `docs/architecture/api/commands.md`,
  `docs/architecture/update-building.md` - disclose admission order, asynchronous
  source selection and the quiesce completion barrier.

No store, replay RPC field, public ownership envelope, generated composition root or
native family registration changes are selected.

## Related Work and Boundaries

The existing `spec-bgp-refresh-reapplies-export-policy.md` owns pre-policy
attribute retention and refresh evaluation. Coordinate shared provenance and
replay carriers without importing that spec's larger reconciliation work here.
The RFC proof-debt spec owns historical records, not this newly demonstrated
wire defect. No existing ATOMIC_AGGREGATE fix owner was found before creating
this narrow release-fix skeleton.

The owner has ordered this correctness repair completed and authorized the
restricted v1 under the original permission to disable optional features for
release. Implementation is active. Scoped main-thread results confirm the
original rewrite and the listed retirement repairs. Entry-point and canonical
producer-break proofs remain open. This is release correctness work, not an
optional new policy feature or completed RFC coverage.
