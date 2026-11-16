# Spec: bgp-atomic-aggregate-export-rewrite

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Priority | P1 |
| Release reason | Export policy can advertise a received ATOMIC_AGGREGATE route as a more-specific prefix on both forwarding rails |
| RFC gaps | RFC4271-5.1.6-1 |
| Depends | - |
| Phase | Closure ready: all bounded deliverables and native gates accepted |
| Handoff | `tmp/session/2026-11-14-210034/state/session-state-bgp-atomic-aggregate-export-rewrite-01a1db03-fbc1-7b32-8cbf-72b6284c110b.md` |
| Updated | 2026-11-16 |

Scope is a correctness repair in the existing BGP forwarding capability. This
spec records the demonstrated violation and the approved restricted v1 design.
Scoped main-thread tests verify the admission and retirement repairs below.
The canonical daemon proof and separate live FRR admission/retirement walks
now pass. Both origin/stale failure-transport walks observe clean passes and
intended broken failures. Final scoped lint has no reactor or owned atomic
interop finding; unrelated interop findings remain. External producer publication
and all forty-four refresh observations have landed. The independent native CLEAN artifact binds all 122 committed paths at
`bcc0059b72`. Auto's authorized four-fixture/audit handoff is published with
explicit authorship credit, including fresh native test-meaning approvals.
All bounded prerequisites to the source-only A/B closure are accepted.

Current checkpoint: native script `c-a39592` published the 68-path caller
commit `94079ae78b`, with no push. The final daemon baseline
`job-atomic-final-daemon-baseline-9bf5156b.log` passes both internal and forked
placements. The corrected stress run
`job-atomic-final-daemon-stress80-coherent-8269f0cb.log` reports no reproduction
in all 80 invocations under load. Its exit code 1 is the harness's
NOT_REPRODUCED result. The earlier same-suffix file without `-coherent`
failed its build on an incomplete next-hop API and ran no invocation.

Main promoted the draft to
`test/plugin/atomic-aggregate-policy-admission.ci`; the draft path is gone.
`job-atomic-canonical-daemon-green-05237645.log` then passes that canonical
selection. Separate native daemon walks observe clean PASS and broken FAIL:
`job-atomic-canonical-admission-redwalk-6c70d9ed.log` fails both placements on
a forbidden marker-stripped announcement instead of the expected withdrawal.
`job-atomic-canonical-retirement-redwalk-6ab3c235.log` fails both placements
after useful B and the fence arrive without exact excluded A's withdrawal
within the unchanged observation bound. These results preserve the bounded
daemon claim. They supply neither live FRR acceptance nor whole-RFC support.
Final scoped lint `f394fac4` completes every normal/Linux/capability/compile-out
variant with no reactor or owned atomic interop finding. It retains fifteen
normal/Linux and sixteen compile-out findings in ten foreign interop files.
Earlier attempts stopped during concurrent dependency edits and supply no
passing analysis.

The standalone quiesce correction landed as `79aaa2daf4` through native script
`c-7b2a2f`, with four selected paths and no push. Its nine-root final run
`87fe9053` and twenty stopped-worker executions `099c84e0` remain the scoped
evidence. This separate commit does not supply live interoperability proof.
Main removed its four superseded private quiesce inputs and native script/message
after checking ownership and active references. Commit `79aaa2daf4` is the
authority; do not rerun commands that name the removed private manifest.

The first Linux FRR baseline, `job-atomic-frr-kernel-baseline-6ae119d5.log`,
fails at assertion 17 after 1m13s. The preceding route, absence and source-key
checks completed, but FRR's establishment count changed after a real
Cease/Out of Resources. Recovery rebuilds one original candidate per key;
the interop helper's MED60 branch incorrectly required bundled D+E input.
The new `TestAtomicPolicySDKSubsetRecovery` reproduces both singleton failures
and an incorrectly accepted unknown sibling in `00a9a6de` / `2e8e3494`.
An independently approved helper correction now applies the same subset
decision to bundled D+E and singleton D or E. All existing checker assertions
remain. The judge recorded native content approval for `bgp.atomicPolicyAnswer`
in `tmp/commit-weakening-approved-ebf09f54.md`. Main formatted both files in
`0fd2e18d`. `job-atomic-interop-policy-green-0bd81110.log` then observes all
seven selected roots PASS (0.864s), including the new singleton regression,
existing exact route/source/raw-answer controls and session-generation control.
The corrected native baseline
`job-atomic-frr-kernel-baseline-corrected-6ae119d5.log` passes in 1m3s.
It reports no lab resources left behind and the VM stopped. Any later helper
content change needs a fresh binding.

`job-atomic-frr-admission-redwalk-4ebc0142.log` observes native verdict `red`
and exit 0. Clean passes in 48s; broken fails in checker assertion 11 after
1m17s. FRR's captured table contains forbidden `10.42.1.0/25` and
`10.42.2.0/24`, alongside useful C/25, E/MED88, the independent control and
the fence. This is the intended admission failure, not a setup failure.
The clean image is
`sha256:d3c577f3b2c6ddb26f9c320177201d44651c31334b8991467a2287b118a5cbaf`;
the broken image is
`sha256:cb69fc9f189e9b3183c68183c0e1d0cc36a63e57bd522d64fd3586d0737bd825`.
Both are rebuilt through the inherited overlay seam. Native teardown reports
none left behind and the VM stopped.

The separate authorized FRR retirement walk,
`job-atomic-frr-retirement-redwalk-authorized-179e2b85.log`, returns native
verdict `red` and exit 0. Clean passes in 47s; broken fails checker assertion
11 after 1m44s. FRR retains excluded `10.42.4.0/24` at MED77 while useful
E/24 has MED88 and C/25 plus both controls remain. The clean image is
`sha256:552ff7843494f3be285ae1ae33fd192f9e87c5395565d649dee7d9aa2145065a`;
broken is
`sha256:0b43639de73957b14216b585ceadbb2533837214fcdb071741be355965c5ba15`.
Native teardown leaves none and stops the VM. The earlier same-suffix attempt
without `-authorized` was denied on its setup lock before testing and supplies
no proof. Origin/stale failure-transport walks are recorded separately below.

`job-atomic-origin-retirement-redwalk-5247e0e3.log` observes native verdict
`red`: all four clean cases pass, then both local-owner cases miss their
exact socket removal under the origin-write bypass. Both foreign controls
pass. `job-atomic-stale-failure-retirement-redwalk-aff65e8d.log` observes
verdict `red`: all five clean cases pass, then the cleanup bypass loses local
and current-exact-receipt removals and the joined closed-pipe cause. Foreign
and newer-receipt controls pass. These are named semantic failures after
successful clean runs, with no build or setup failure.

The fifteen whole-file companion paths and narrow RFC summary hunk landed as
`43d3055e7b`. The native commit carries the independently approved negative
tag and bounded daemon/FRR evidence. No push was made. The root RFC row and
audit stay partial; the companion commit supplies no whole-RFC acceptance.

Style cleanup `a08a7b67b5` changes only the interop checker/helper wording and
shape. Its helper/oracle package result is recorded separately by main.
A fresh independent review of the 93-path committed scope found one operator
wording issue, with no other behaviour, security or coverage finding. Its
diagnostic correction and its targeted result are recorded below. The focused
independent follow-up cleared that exact diagnostic delta.

The diagnostic correction and independently judged exact phrase assertion
landed as `bc904308ab`. The named `TestAtomicAggregatePolicyDiagnostic`
passes in `job-unit-pkg-173339b5.log` (2.061s), reached by outer `f6f464c1`.
The source message now requests removal without promising completed withdrawal.
Its native ordinary approval is carried by the commit. The focused independent
delta review is clear; the covering native artifact remains a separate gate.

The first 44-cover refresh batch `f4fe4e33` writes forty records and stops
four RFC9494-4.3-3 covers at their clean baseline. Both advertised cases pass;
both not-advertised cases wrongly expect an announcement from an addressless
eBGP fixture. Independently judged destination-only LocalAddress repair leaves
the source grants, capability inputs, assertions and bounds unchanged.
Both named roots and all four cases then pass in `9af535d5` (2.620s, outer
`6874d29b`). Native `08ba517a` writes all four records after clean passes and
whole-producer-disable failures. The native route-server witness disables
`reactorForwardRSSectionResult` and panics; retain this existing proof
limitation rather than describing it as a semantic wire mutation. Fixture and
canonical record land as `e3624f7852`. All forty-four observations are now
complete. The targeted post-refresh inventory `e8502440` found forty current
and only four stale external-producer covers. Native `a964a869` renews those
four after clean passes and expected whole-body panic failures. Auto publishes
their core producers in `52e4ae832a`; main verifies the relevant source files
match HEAD. Native commit `34a5530dd8` lands those final four records. Native
commit `7be40717af` carries thirty-six, and `e3624f7852` carries the four LLGR
fixture records: all forty-four refresh observations are committed. Other
unchanged records need no blind rerun.

The earlier scoped verification remains evidence: the main thread's
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
Separate native producer and daemon/interop observations are accepted above;
these scoped unit results remain a different evidence class.

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
focused evidence below. Accepted daemon and native peer proof is recorded in
the current checkpoint.

### Architecture Docs
- [x] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md` - engine, filter and stored-route boundaries
  → Constraint: preserve the engine/plugin registration boundary and both in-process and external filter transports; the engine must not name a particular policy member.
- [x] `docs/architecture/bgp/structural-forwarding.md` - transformed wire ownership and recipient dispatch
  → Constraint: output-byte replacement preserves ingress ownership. Source protocol facts must survive replacement too; source identity alone does not prove prefix containment.
- [x] `docs/architecture/bgp/egress-attribute-rules.md` - provenance-sensitive and final-wire checks
  → Constraint: distinguish a rule bound to received attributes from a rule bound only to final output. Do not infer received ATOMIC_AGGREGATE from the final attribute set.
- [x] `docs/architecture/core-design.md`, section 9 and Policy Filter Chain - receive, policy and final-write order
  → Constraint: preserve ordinary and native route-server pipeline order and existing policy error handling.
- [x] `docs/architecture/encoding-context.md`, `docs/architecture/memory/lifetime-contracts.md` - borrowed-byte constraints read before rejecting a new provenance carrier
  → Constraint: received bytes are borrowed; retain source NLRI semantics across queues, splits and stored replay without keeping an expired receive-buffer slice.

### RFC Text and Summary
- [x] `rfc/full/rfc4271.txt`, Sections 5.1.6 and 9.1.4; `rfc/short/rfc4271.md`, RFC4271-5.1.6-1
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

## Deliverables Checklist

| Deliverable | Producing boundary | Verification and disposition |
|-------------|--------------------|------------------------------|
| Original-input admission for raw and typed answers | `runLibraryChain`, `runFinalJudges`, `runStaleChain`, `atomicAggregatePolicyAllowed` | Native-key matrix, final-judge controls and recorded legacy socket break accepted |
| Exact source retirement with useful survivors | `atomicAggregatePolicyRemovals`, `prepareFwdPolicyRemovals`, `writePolicyRemovals`, `notifyOneMessage` | Import, cached/native, native-family, ADD-PATH, split, queue and owner/receipt matrices pass |
| Retirement survives survivor failure | Origin writers, `sendBatchUpdatePreFiltered`, `sendStalePolicyRemovals` | Forward failure controls, all 180 stale repeat cases and both failure-transport walks accepted |
| Failed preparation settles captured sessions after unlock | Forwarding validation, cached/native fan-out and AIGP replay settlement | Seven settlement roots and thirty affected regression roots pass; independent source review retained |
| Quiesce observes recovery and pending writes | `Peer.syncOwed`, `routeDecisionOwed` | Nine final peer-sync roots and twenty stopped-worker executions pass; landed as `79aaa2daf4` |
| Configured daemon and both transports | `atomicAggregatePolicyScenario`, `atomicAggregatePolicyForwarder` | Canonical selection passes; corrected stress has no reproduction in 80 invocations; both daemon breaks expose named failures |
| Native peer outcome and semantic breaks | `checkAtomicPolicyFRR`, `atomicPolicyAnswer` | Corrected live baseline and separate admission/retirement walks accepted, including source retention and session checks |
| Bounded public claim and documentation | Partial annotation, narrow negative carrier, architecture/guide sections and news fragment | Companion `43d3055e7b` retains three distinct canonical covers and the partial row; source anchors and the source-only fragment STE pass |
| Final closure quality | Scoped lint, proof freshness and whole-change-set Review Gate | Owned lint, source anchors and source-only fragment STE accepted; all forty-four refreshed observations committed. Independent native CLEAN binds all 122 committed paths at `bcc0059b72`; the authorized fixture/audit publication and all native approvals are accepted |

## Security Review Checklist

| Concern | Producing contract | Evidence or remaining check |
|---------|--------------------|-----------------------------|
| Malformed framing and native identity | Helpers parse UPDATE sections and registered framing; membership compares full native keys | Malformed, collision, all-family and ADD-PATH-zero controls pass |
| Borrowed buffers and asynchronous ownership | Synchronous admission borrows read-only input; continuations and overflow own outgoing bytes; parent release frees manifests | Import buffer, queue, split, cache-expired replay and immutable-source controls pass |
| Cross-source removal and stale receipts | Final child admission retains exact expected receipts and local decisions spare received owners | Foreign/local, newer-same-source and exact-receipt controls pass, including broken-run controls |
| Partial writes and retries | Direct callbacks claim groups once; only unconsumed groups take route-scoped cleanup; transport causes remain returned | Nine stale encoding cases pass twenty times; stale walk notices lost removals and cleanup causes |
| Lock order and replacement sessions | Fatal captures settle after dispatch/validation unlock and retain captured session identity | Actual callback TryLock and replacement-session controls pass; no new worker or session state |
| Resource exhaustion | Framing bounds helper work; scratch uses pooled size classes; groups have at most two non-nesting children | Small/4097-key normal allocation bounds, unsupported large-family and preparation-failure controls pass |
| Injection, credentials and external dependencies | Repair adds no shell, path input, credential, cryptographic primitive or product runtime dependency | Existing admission diagnostic declares its code and editor action; fixture endpoints use existing runners |

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
these changed sections; this is not an independent review stamp. Separate
independent documentation review is retained. Native `doc check all` validates
all source anchors, with unrelated catalog drift reported separately. The
source-only news fragment passes its targeted STE check. The current targeted
eight-page STE capture `f7c292` exits 3 on three foreign habit growths: FSM
wording in core-design and two semicolons in an IPsec interop row. None is in
atomic text. Main routes them to their active owners; no whole-page STE pass
is claimed and unrelated prose does not hold this bounded closure.

## Documentation Update Checklist

| Page | Required current behavior | Author update | Closure verification |
|------|---------------------------|---------------|----------------------|
| `docs/architecture/core-design.md` | Shared accepting-result admission, original `chainInput`, source withdrawal, marked subset and unmarked survivors | Written with `runLibraryChain` / helper anchors | Author and retained independent source comparison accepted; native source anchors valid |
| `docs/architecture/bgp/egress-attribute-rules.md` | Received versus post-import provenance; stronger v1 restriction; native identity; withdrawal ownership; historical diagnostic | Written; each new factual paragraph has producing source anchors | Author and retained independent source comparison accepted; native source anchors valid |
| `docs/architecture/bgp/structural-forwarding.md` | Source-removal admission before kept output, asynchronous RIB-owned decisions, captured-session fatal preparation settlement, one destination item/fence/count and child-only byte/provenance lifetime | Written with helper, grouped writer, ownership, recovery and post-unlock settlement anchors | Author and independent comparison accepted with fatal preparation bounded to received forwarding; native source anchors valid |
| `docs/architecture/api/process-protocol.md` | Raw and typed results on both transports share admission; restriction does not use `on-error`; no envelope fields added | Written with guard/helper anchors beside the new paragraphs, retaining type/raw-answer anchors on their own original paragraph | Author and retained independent source comparison accepted; native source anchors valid |
| `docs/guide/bgp-policy.md` | Useful marked/unmarked controls; new marked destinations/marker removal unavailable; source withdrawal; diagnostic and exact editor action | Written with helper/chain and diagnostic anchors | Author and retained independent source comparison accepted; native source anchors valid |
| `docs/architecture/testing/interop.md` | FRR scenario's exact final absences and useful routes; an unrelated fence does not complete asynchronous source recovery | Written with checker, helper, ownership and recovery anchors | Live baseline and both semantic walks accepted; independent source comparison accepted and native source anchors valid |
| `docs/architecture/api/commands.md`, `docs/architecture/update-building.md` | Quiesce waits for current-session cold recovery and pending takeover flush; later unrelated output is not a recovery barrier | Written with `syncOwed`, `routeDecisionOwed`, ownership and recovery anchors | Author and independent comparison accepted against current-session owed/pending checks and held, released, reset and stopped-worker controls; native source anchors valid |

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
| `TestAtomicRetirementOriginFailureCommitsRemoval`, `TestAtomicRetirementStaleFailureKeepsRemovals` | Both roots pass under race in `708accf1`. Separate native walks `5247e0e3` and `aff65e8d` observe clean pass and the named broken socket/cleanup-cause failures with foreign/newer controls passing | Native transport observations accepted. The separate encoding test covers pre-callback stale splitting |
| `TestAtomicForwardFailureRetiresSource`, `TestAtomicForwardRemovalNegotiatedFamilies` | Both roots and all twelve cases pass in `job-atomic-forward-failure-green-3b08a959.log`, then in `708accf1`, after the real `3b08a959` red | Preserve source-owned cleanup, foreign owners, useful negotiated siblings, honest failed-survivor counts and no EOR |
| `TestAtomicRetirementStaleEncodingFailure` | Actual `606e6df4`: three missing-A failures and five passing controls; all nine repaired cases pass in `230bf916`. Later `641b7b32` exposes an early channel-marker assertion in queued/fitting only. After independently approved use of the existing overflow-aware barrier, `job-atomic-stale-barrier-race20-final-6a619e8c.log` observes twenty root executions and all 180 cases pass under race (2.435s); the root also passes in the separate thirty-root `job-atomic-final-forward-regressions-green-641b7b32.log` | All split-error, owner/fence, exact retirement, useful fitting output and no-EOR assertions remain; the earlier same-hash non-final repeat log is a compile failure, not test evidence. Native and entry-point proofs remain separate |
| Affected callback regression matrix | `708accf1` ran 79 roots under race: 77 pass and two fail; the passing scope contains 43 `TestAtomic` roots plus the original rewrite root. Both failed RFC8092/8669 roots now pass in `230bf916`, including all twelve RFC8669 cases | Keep the independently judged ASN4 fixture corrections distinct from the atomic product repair; no old input, assertion or tag was weakened |
| `TestAtomicRetirementRecoveryCanFollowUnrelatedFence` | Both active-RIB rows pass in `job-atomic-recovery-and-quiesce-red-dadbc079.log`: B and an unrelated fence pass while A's lookup is held; release withdraws A or installs a permitted alternate protected from the old source | Preserve exact source-removal admission and eventual decision; never treat the first unrelated wire fence as recovery completion |
| Four `TestDrainPeerSync` recovery roots and existing peer-sync controls | `dadbc079` exposes two false-success barrier failures; all four repaired roots pass in `46d68d65`. After the independently approved forward-disposal setup correction, `job-atomic-stopped-recovery20-final-099c84e0.log` observes twenty stopped-worker passes with no failure (13.530s), and `job-atomic-peer-sync-final-regressions-87fe9053.log` observes all nine selected roots pass (9.626s). Native script `c-7b2a2f` lands the four-path piece as `79aaa2daf4`, with no push | Preserve the actual cold-recovery, pending-flush, reset, stopped-worker, replay, policy-replay, channel/overflow and named-error assertions; these results do not supply daemon or interoperability proof |
| `TestAtomicPreparationFailureRetiresDestination` | Historical `job-atomic-preparation-failure-red-f6a26b94.log` observes four large-key live-obsolete-session failures and four passing small controls, with actual encoding-error preconditions. All eight repaired cases now pass under race in `job-atomic-preparation-settlement-final-8a1a060f.log` (seven selected roots, 2.395s) | Preserve all eight route assertions and legal extended-receive capability through the affected regression run; this result is not daemon or interoperability proof |
| Preparation settlement controls | All six supplemental roots pass in `8a1a060f`, covering actual callback TryLock, accepting validation, replacement session, inherited ordinary error, true reflection suppression, late-fatal receipts, five useful recipients and both AIGP replay rows. Historical `ec101c7d` had two fixture failures, corrected only after independent native approvals for writer-buffer negotiation and true suppression setup; all original outcome/socket assertions remain | Affected forwarding/AIGP/validation/native-RS/stale regressions now pass in the separate thirty-root GREEN below; native checks remain. The injected replay bound proves settlement plumbing only, not an RFC-valid negotiated limit or natural giant retained route |
| `TestAtomicAIGPReplayPrequeueFailureCanRetry` and affected regression selection | The new direct prequeue-error rollback control passes, requiring no false queued/decided result and actual AIGP111 output after facts recover at the same revision. The first `job-atomic-final-forward-regressions-641b7b32.log` has 28 PASS/two FAIL roots. After both independently judged setup repairs, separate `job-atomic-final-forward-regressions-green-641b7b32.log` observes all thirty roots PASS under race (3.380s), no FAIL | Keep this unit scope distinct from the separately accepted native producer-break and daemon/interoperability observations |
| Caller integration | Native script `c-a39592` published commit `94079ae78b`, containing 68 paths; no push | Keep remaining daemon/interop/proof deliverables separate. This caller commit does not close the spec |
| Canonical daemon fixture and stress | Final draft baseline `9bf5156b` passes both placements. Coherent stress file `job-atomic-final-daemon-stress80-coherent-8269f0cb.log` reports NOT_REPRODUCED in 80 invocations under load (exit 1). Earlier same-suffix non-coherent attempt failed to build before any invocation. Main promoted the draft, and canonical `05237645` passes | Preserve internal/forked transport, exact socket/RIB observations and unchanged four-frame/fifteen-second bounds. Historical invocation 77 and its independently judged ordering correction remain distinct from this accepted stress result |
| Canonical daemon admission and retirement discrimination | `6c70d9ed` and `6ab3c235` each observe clean PASS then named broken FAIL in both internal and forked modes. Admission emits a forbidden marker-stripped route instead of withdrawal. Retirement delivers useful B and fence but never exact excluded A's withdrawal within the retained bound | Accepted separate semantic walks; no whole-RFC support follows |
| Native interoperability and discrimination | Original baseline fails on a real reset; the judged per-key helper correction has seven roots PASS and a corrected live baseline PASS1m3s. Admission `4ebc0142` observes clean PASS48s/broken checker11 FAIL1m17s with forbidden A/25 and B/24. Separate authorized retirement `179e2b85` observes clean PASS47s/broken checker11 FAIL1m44s with excluded D/24 MED77 and useful E/MED88 retained. Both walks use distinct clean/broken images and leave no resources or VM running. The denied initial retirement setup ran no proof | Accepted bounded native interoperability and both semantic breaks; owned refresh records are committed and the RFC row stays partial |

The result logs above live in `tmp/session/2026-11-14-210034/scratch/`.
They are retained only as the smallest TDD/result evidence while final
verification and closure are unresolved; remove superseded run output once
the owning committed closure records its bounded results. Keep observed
baseline failures distinct from later named passes. A written repair or an
approved tag does not supply the missing runtime or producer-break result.

| Retained current item | Purpose | Removal condition |
|-----------------------|---------|-------------------|
| Final daemon baseline `job-atomic-final-daemon-baseline-9bf5156b.log` and canonical `job-atomic-canonical-daemon-green-05237645.log` | Retain the observed two-placement baseline and successful promoted selection | Remove when committed closure evidence retains both bounded results |
| `job-atomic-final-daemon-stress80-coherent-8269f0cb.log` and `tmp/stress-repro/plugin-draft-pattern-atomic-aggregate-policy-admission-7a84a48ad411f5d55d7552f2b13a6e9b7f0acfdb587fecacc03b38d24611cf24-20261116-041311-2021576952.log` | Retain the actual 80-invocation NOT_REPRODUCED result and its detailed under-load trace | Remove when committed closure evidence retains the stress result and no investigation needs individual invocation output. The non-coherent build-failure file supplies no invocation evidence |
| `job-atomic-canonical-admission-redwalk-6c70d9ed.log` and `job-atomic-canonical-retirement-redwalk-6ab3c235.log` | Retain clean-before-break daemon discrimination in both transports | Remove when committed closure evidence retains the exact failures and current proof acceptance |
| Private admission/retirement bypass patches under `scratch/atomic-proof/` | Retain the exact separate cuts observed in the accepted daemon and FRR walks | Remove after committed evidence supersedes the consumed inputs and no active canonical refresh needs them |
| Private origin/stale failure-cleanup bypass patches under `scratch/atomic-proof/` | Preserve consumed inputs for accepted origin `5247e0e3` and stale `aff65e8d` native walks | Remove when committed closure evidence retains both outcomes and no active run references the inputs |
| `job-atomic-frr-kernel-baseline-6ae119d5.log`, `job-atomic-interop-singleton-red-00a9a6de.log` and `job-atomic-interop-policy-green-0bd81110.log` | Retain the real reconnect diagnosis and focused helper RED/GREEN evidence | Remove after committed evidence preserves the independently judged fixture repair, actual focused GREEN and live interoperability disposition |
| `job-atomic-frr-kernel-baseline-corrected-6ae119d5.log` and `job-atomic-frr-admission-redwalk-4ebc0142.log` | Retain the accepted live baseline and separate admission discrimination, including captured wrong routes and distinct images | Remove after committed closure evidence preserves these bounded results and the remaining live proof disposition |
| `job-atomic-frr-retirement-redwalk-authorized-179e2b85.log` | Retain the accepted separate retirement discrimination, including exact excluded D/24, useful E/MED88 and distinct images | Remove after committed closure evidence preserves this bounded result. The denied setup attempt supplies no test evidence |
| Closure light check captures under `scratch/atomic-proof/atomic-closure-{repo,weakened,doc}-check.txt` | Retain actual scoped/foreign finding classification | Remove after committed closure evidence preserves the classification and no gate needs individual rows |
| `scratch/atomic-proof/atomic-closure-body.txt` | Exact source-only closure-A commit message input | Remove after the accepted native script captures the message and no active Create needs the file |

The cleanup owner removed twenty obsolete private commit/record inputs and
one failed stress trace after tracking, ownership, reference and process checks
found no live consumer (183,794 bytes). This includes the consumed caller,
companion and final-four lists/hunks; committed receipts replace them. The
accepted native review supersedes the removed 19,948-byte immutable argument
manifest. The owning state records every exact removed path. No temporary
draft remains. The remaining atomic RFC summary patch belongs to its owner
until its own active-reference check clears it.
The four private quiesce inputs and their native script/message are removed
after main checked ownership and active references. Commit `79aaa2daf4`
replaces those inputs. The actual caller script `c-a39592` remains retained.

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
  source/recipient integration matrix; preserve original rewrite inputs and
  assertions while adding only the independently approved narrow tag.
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
- `internal/test/fixture/atomic_aggregate_policy.go` and promoted canonical
  `test/plugin/atomic-aggregate-policy-admission.ci` - real daemon proof on both
  transports. The draft path is gone after accepted baseline/stress results;
  canonical selection and separate daemon admission/retirement walks pass.
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
release. Implementation and its scoped evidence have landed. Scoped main-thread results confirm the
original rewrite and the listed retirement repairs. Entry-point and canonical
producer-break observations are accepted for the bounded capability. Final
the source Review Gate and separate committed publication deltas are CLEAN.
The final native artifact binds 122 committed paths at `bcc0059b72`; every
bounded prerequisite to the source-only closure script is accepted. This work earns
no claim of completed RFC coverage.

---

## Implementation Summary

### What Was Implemented

The restricted policy boundary judges every modified accepting result against
the original chain input. A marked result that still announces retains its
marker and original native identities. Useful attribute edits and subsets
remain available, and originally unmarked input retains its rewrite facility.
The equality restriction deliberately excludes more cases than the RFC's
deaggregation prohibition.

Import publishes owned source removals before the effective replacement.
Forwarding carries one bounded group through existing ownership, queue,
completion and replay fences. Originated and stale writes preserve admitted
removals when a survivor fails. RIB-owned removals can settle through cold
selection after unrelated output; quiesce now waits for that decision and its
writes. On cached/native forwarding, preparation failure closes only the
captured session after locks release.

The implementation landed in `1d8f79b7c1`, `94079ae78b` and `79aaa2daf4`.
The configured daemon, interoperability scenario and bounded proof companions
landed in `43d3055e7b`. Style cleanup, diagnostic wording and the LLGR endpoint
fixture landed in `a08a7b67b5`, `bc904308ab` and `e3624f7852`. Thirty-six
current proof updates landed in `7be40717af`; the narrow mechanical LLGR audit
reseal landed in `58f4906f4f`. The final four external-producer covers landed
in `34a5530dd8`, after their producer publication `52e4ae832a`. Auto's authorized
four-fixture and accepted RFC2545 audit handoff lands in `bcc0059b72`, preserving
all oracles and consuming fresh native approvals. No push or
whole-RFC acceptance is recorded here.

### Bugs Found/Fixed

| Defect | Repair evidence |
|--------|-----------------|
| Marked raw-policy deaggregation and marker erasure | Original socket regression, original-input admission tests and recorded `f3037485` break |
| Accepted subset/EOR leaves excluded source routes | Source-removal helper, import publication, grouped rail and canonical daemon retirement walk |
| Failed survivor construction loses accepted removals | Forward failure pair, origin/stale failure roots and separate native transport walks |
| Large unsupported-family removal suppresses useful sibling | Negotiated-family projection and small/large cached/native controls |
| Quiesce ignores cold decisions or pending takeover writes | Held/released/reset/stopped-worker roots; final nine-root and twenty-repeat results |
| Preparation failure leaves an old session or counts fatal delivery | Captured-session post-unlock settlement; replacement, healthy-recipient, validation and AIGP controls |

### Documentation Updates

The eight pages in the Documentation Update Checklist carry the admission,
ownership, cold-recovery and completion contracts with producing source anchors.
The RFC summary and audit retain the bounded partial claim. The source-only
news fragment records operator-visible behaviour. Native documentation
checking validates source anchors; unrelated catalog drift remains. The source-only
news fragment passes its targeted STE check. The targeted eight-page STE has
three foreign prose findings, assigned to the active FSM/IPsec owners, and
no atomic text finding. Owned refresh metadata has landed in the three recorded proof commits. The
authorized RFC2545 audit and fixture handoff is published in `bcc0059b72`,
with native approvals consumed and no new product behaviour.

### Deviations from Plan

The initial research considered a public ancestry carrier. The approved
restricted rewrite contract instead preserves the source condition through
existing admitted bytes and requires no public envelope change. Source
retirement, quiesce and conservative captured-session settlement were necessary
repairs exposed while proving that supported contract. Fixture maintenance
changed neither the product requirements nor the useful survivor assertions.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | An unrelated output fence was treated as completed withdrawal recovery | Cold RIB selection is asynchronous and can complete later | Retained daemon stress invocation 77 and held-recovery test | Independently judged observer waits for exact removal, survivor and fence within unchanged bounds; architecture states the completion boundary |
| assumption | A channel completion marker was treated as an overflow barrier | The marker can overtake an overflow group | Queued fitting stale case in the first affected regression run | Independently judged replacement uses the existing overflow-aware barrier; all 180 repeated cases pass |
| assumption | Interop policy subset callbacks were expected to retain bundled D+E | RIB recovery lawfully reconstructs one original key | Actual FRR reset and singleton TDD regression | Independently judged helper applies the same subset to bundled and singleton inputs; native baseline and both breaks pass |
| approach | Race instrumentation was used as a zero-allocation oracle | Race pool behaviour changes allocation observations | Actual normal-build comparison | Keep four allocation roots in normal builds and semantic/concurrency roots under race |
| record | News fragment Exposure said none despite naming raw-policy triggers | The affected configuration belongs in the operator fragment | Final closure prose check | Name raw import/export transformation of received marked routes and retained route subsets; scope fatal preparation to cached/native forwarding; no behaviour or review-round change |
| approach | Final source completion was described before all proof and quality gates | Tests and recorded breaks have separate obligations | Closure audit of retained results | Keep final gates explicitly pending; only accepted native outputs update their evidence rows |
| approach | Refusal diagnostic promised that source routes were already withdrawn | Refusal requests owner-checked removal; absent/foreign keys and asynchronous RIB selection prevent a completion promise | Fresh independent scope review traced reporting before the withdrawal outcome | Corrected shared promise and independently judged phrase assertion in `bc904308ab`; named diagnostic test `173339b5` passes; focused follow-up clears the ISSUE and final native artifact is CLEAN |
| assumption | Four LLGR proof baselines expected a useful announcement from an eBGP fixture without a local address | Default Auto becomes next-hop self and correctly withholds without a local endpoint | Native batch `f4fe4e33` clean-baseline failures | Independently judged destination LocalAddress only; both roots/four cases pass in `9af535d5`; four native records written in `08ba517a`, landed `e3624f7852` |

### Diagnostic wording diagnosis

| Part | Finding |
|------|---------|
| Symptom | The old operator message said source routes were withdrawn before completion had been observed |
| Root cause | The old `atomicAggregatePolicyMessage` supplied completed-result prose to `reportAtomicAggregatePolicyRefusal`; `runLibraryChain` emits the report before returning `OutcomeWithdraw` |
| Owning layer | The shared diagnostic constant used by metadata, the historical report and the warning; routing and ownership behaviour are unchanged |
| [workaround] | Force immediate withdrawal or wait for all recovery before logging, which changes ownership and asynchronous completion semantics to satisfy an inaccurate sentence |
| [source] | Corrected to source-route removal requested, retaining the marker/native-identity rule and exact editor action; commit `bc904308ab` carries independent approval and named test `173339b5` passes. Focused delta review clears the ISSUE; final independent native artifact is CLEAN |

### LLGR clean-baseline fixture diagnosis

| Part | Finding |
|------|---------|
| Symptom | Four RFC9494-4.3-3 record attempts stopped before their break because both not-advertised cases missed the exact useful `192.0.2.0/24` announcement |
| Root cause | The failing `receiveAndForward` fixture built an ordinary eBGP destination with Auto next hop but no connected endpoint or LocalAddress; `precomputeNextHop` selected self, and `egressNextHopWithheld` correctly withheld it |
| Owning layer | Fixture construction. The product check is correct and the test's local-endpoint premise is incomplete |
| [workaround] | Force unchanged next hop or weaken the useful announcement assertions, bypassing the ordinary self path and losing the original LLGR negative control |
| [source] | Add only destination LocalAddress `198.51.100.1`, independently judged under alternative (b). Preserve all grants, tags, capabilities, stale metadata, cardinality, exact routes and bounds; no source Send change |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refuse unsafe marked raw and typed replacement | Done | `runLibraryChain`, `atomicAggregatePolicyAllowed` | Same accepting-result boundary on import/export, original input retained for later passes |
| Preserve useful marked and unmarked output | Done | Admission/native-key controls and both configured recipient sockets | Equality, attributes, subsets, withdrawal and independent unmarked rewrite remain useful |
| Retire only authorised excluded source keys | Done | Grouped forwarding, import continuations and final ownership | Cold RIB decisions can install a permitted replacement |
| Preserve removals after later failure | Done | Origin/stale sender and received forwarding failure repairs | Actual RED/GREEN and native failure-transport observations retained |
| Prove real daemon and peer behaviour | Done | Canonical plugin test and FRR scenario | Both daemon transports, 80 stress invocations and separate semantic breaks accepted |
| Land honest bounded claim and finish closure quality | Done: final native review CLEAN | Scoped lint has zero owned findings; thirty-six proof updates and one unchanged-scope audit reseal committed. All forty-four refreshed observations are committed through `34a5530dd8`, after producer publication `52e4ae832a`. The authorized RFC2545 audit/fixture handoff is committed in `bcc0059b72` with fresh native approvals | Requirement remains partial; no scope reduction or release claim |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Original six socket cases, both daemon modes and live FRR admission walk | Refusal and useful unmarked controls are distinct |
| AC-2 | Done | Unmarked-adds-marker and independent /25 controls | A blanket clamp fails these controls |
| AC-3 | Done | Useful attribute/subset/reorder/withdrawal/EOR roots | Stronger product restriction documented |
| AC-4 | Done | Native families, ADD-PATH zero, mixed, queue/split and stored replay roots | Bounded implementation evidence does not promote the whole RFC row |
| AC-5 | Done within bounded claim | Three canonical covers and partial audit in `43d3055e7b`; forty-four affected observations refreshed and committed | RFC4271-5.1.6-1 stays partial. Auto retains its separate RFC2545 audit judgment |
| AC-6 | Done | Import refusal/publication roots and configured Adj-RIB-In checks | No public ancestry field added |
| AC-7 | Done | Nine stale encoding rows repeated twenty times and both transport walks | Foreign/newer owners, fitting output and both errors preserved |
| AC-8 | Done | Final nine peer-sync roots and twenty stopped-worker runs | Stopping a worker does not erase live debt |
| AC-9 | Done | Seven settlement roots and thirty affected regressions | Healthy and replacement sessions survive; closes captured session after unlock |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Admission/native key/allocation population | Done | Helper and policy admission test files | Semantic race and normal allocation evidence remain separate |
| Retirement/publication/final/stale population | Done | Atomic retirement, trace, forwarding and stale failure tests | Includes actual missing-removal REDs before repair |
| Recovery/quiesce/preparation population | Done | Atomic recovery/preparation tests and peer-sync recovery tests | Includes useful alternate and captured-session controls |
| Canonical daemon | Done | `test/plugin/atomic-aggregate-policy-admission.ci` | Promoted only after corrected baseline and stress acceptance |
| Native FRR baseline and both semantic breaks | Done | `bgp-atomic-policy-admission-frr` | Distinct built images; native cleanup reports no resources or running VM |
| Final lint/freshness and exact-blob review | Owned lint clean; final native review CLEAN | Actual `f394fac4` lint has no reactor or atomic interop finding. `7be40717af` preserves thirty-six updates, `e3624f7852` four LLGR covers and `34a5530dd8` the last four renewed producer covers | Fifteen/sixteen foreign interop findings and unrelated global RFC violations do not become an overall green result |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Native admission/subset helpers and semantic/allocation tests | Done | `1d8f79b7c1` |
| Caller, writer, grouped ownership, diagnostics and affected tests | Done | `94079ae78b` |
| Current-session owed work and peer-sync recovery tests | Done | `79aaa2daf4` |
| Canonical daemon fixture, CI, interop helpers/scenario and proof companions | Done | `43d3055e7b` |
| Architecture, API, guide and interop documentation | Done | Selected hunks landed with their producing changes |
| Closure records and source-only news fragment | Prepared | Remain in the current change until closure commits A and B land |

### Audit Summary

Implementation and entry-point proof deliverables have landed. Final scoped
lint is clean for the reactor and owned atomic interop code, with foreign
interop findings retained separately. Producer publication and all forty-four
refreshed observations have landed. The independent source review and separately attributed publication deltas
are CLEAN. The final native binding covers all 122 committed paths at
`bcc0059b72`. All bounded closure prerequisites are accepted.
No acceptance criterion was dropped and no item was transferred out of this spec.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Received marking continues to bind policy output | Socket and producer break | Six-case clean pass, four marked decoded /25 failures under admission bypass, both unmarked controls pass |
| Useful output and authorised retirement coexist | Functional and native peer | Canonical internal/forked daemon plus live FRR useful C/E/control routes and exact exclusions; separate retirement bypass retains forbidden D |
| Ownership survives queue/replay and later failure | Scoped race and failure-transport breaks | Native identities, source publication, expected receipts, 180 stale cases and both origin/stale semantic walks |
| Operator completion waits for actual owed work | Controlled production seam | Held recovery/pending flush, reset/stopped-worker and alternate controls in final peer-sync results |

## Work Not Done

None. Every in-scope deliverable and acceptance criterion is complete.
No item was deferred, dropped or transferred. The broader RFC4271 requirement
remains partial outside this bounded repair.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-atomic-aggregate-export-rewrite-01a1db03-fbc1-7b32-8cbf-72b6284c110b.md`: native CLEAN, 122 committed paths, reviewer `atomic_independent_final`, refreshed 2026-11-16T08:34:38Z at committed `bcc0059b72` |
| Native review check | Final expanded native CLEAN accepted exit 0 at committed `bcc0059b72`, binding all 122 immutable path/mode selections. Separate judgments cover the committed FSM and four-fixture publication deltas. Main's A/B Create/script enforce prospective-tree coverage; no empty-selection check is claimed |
| Rounds | Two: one full committed 93-path generic/spec pass found the diagnostic ISSUE; one focused follow-up cleared that exact delta. Separate LLGR meaning judgments and AIGP one-line review do not become extra full passes. Independent recorder verified all 122 current immutable source bindings before final native recording; fixture/publication judgments remain separately attributed |
| Reviewer lenses used | Existing independent logic/ownership, failure-path, security/bounds, fixture and documentation reviews retained in owning state. Final independent reviewer cleared the diagnostic ISSUE and exact LLGR fixture delta; this author supplies no approval of its earlier stale repair |

The closure bookkeeping author wrote the stale encoding test and repair.
It does not approve those changes. Their independent review and independently
judged overflow-barrier correction remain the authority. The independent
final artifact consumes those recorded reviews and binds the
complete current union of all eleven exact Spec-work commits. It records zero
BLOCKER and zero ISSUE in this bounded repair. Markdown is outside the code
pin; the A/B closure adds only the existing spec and source-only fragment.
Main's native Create/script checks the prospective tree against the artifact.

### Final scoped quality checkpoint

Main's actual `job-atomic-final-two-package-lint-f394fac4.log` completed normal,
Linux, capability and compile-out analysis. The reactor and owned atomic interop
files have no finding. Fifteen normal/Linux and sixteen compile-out findings
belong to ten foreign interop files; capability analysis reports zero. This is
not a clean package-wide result and does not trigger repeat testing of unchanged
atomic behaviour.

The closure light checks against `origin/main` report 53 repository ISSUE rows
and, in the retained current weakening check, 33 unexplained weakened findings.
At that checkpoint neither result overlapped the original 117-path union.
The later authorized handoff adopts three fixture carriers named by the
weakening checkpoint. Fresh independent native approvals for all five
changed tagged roots and the default helper are consumed by `bcc0059b72`;
no unexplained weakening is waived. The initial weakening
observation counted 32 before concurrent foreign work advanced. No exemption or
approval was invented for another task's changes. The retained light outputs
are closure inputs, to be removed after accepted closure evidence supersedes
them. Native `doc check all` exits 1 on 389 unrelated catalog/surface drift issues
and a stale discovery-index warning. None names a path in this spec's union.
Its source-anchor phase validates all 5,086 paths across 856 packages, and
rules/digest checks pass. The exact current eight-page STE capture `f7c292`
exits 3 on three foreign prose findings: core-design negotiation wording and
two semicolons in the concurrent IPsec interop row. Main routes these to their
active owners. No atomic text finding or global documentation-green claim
follows.

Native `a964a869` renews both polarities for direct ValidateMP and replay AIGP:
all four clean runs pass, their whole-body panic breaks fail, and all writes
succeed. Auto publishes the relevant core producers in `52e4ae832a`; main
verifies their working-tree files match HEAD. The four-entry metadata commit
lands as native commit `34a5530dd8`. Native `6aa8b952` reseals one shifted LLGR audit with zero refusals,
without changing its judgment or scope; commit `58f4906f4f` preserves it.
The final global RFC output currently lists 343 violations in the concurrent
checkout. RFC4271-5.1.6-1 remains a bounded partial row, not whole-RFC support.
All forty-four owned refreshed observations now have committed receipts. The
authorized RFC2545 audit handoff is committed in `bcc0059b72`; the final
independent 122-path native artifact is separately accepted with zero BLOCKER and zero
ISSUE, not inferred from these proof records.

Main accepts three temporary Auto-exclusive JSON leases and runs the exact
RFC2545-3-2/3-3/3-4 freshness inventories once. All ten assigned exact tuples
are already fresh: one record each, zero stale, unproven or candidate counts.
No proof runs again and no canonical JSON is written. Leases return to Auto.
The four independently cleared coupled fixtures and accepted 25-unit audit
are published in `bcc0059b72`, with explicit Auto authorship credit. Their
exact changes are two family imports and six negotiation-helper calls, with
all assertions, tags and oracles preserved. Five fresh RFC root approvals
and the ordinary helper approval are consumed by that commit. The expanded
122-path native artifact covers this publication; no ten unchanged
observations are rerun. Auto's RFC2545-3-1 seven-tuple and final RFC8950
static-positive native inventories are also current, with no proof rerun.

### Findings fixed

| Severity | Finding | Location | Fixed by |
|----------|---------|----------|----------|
| ISSUE | Accepted subset loses removals after survivor encoding failure | Cached/native and stale materialization | Source retirement staged or retained independently; scoped RED/GREEN and independent repair review |
| ISSUE | Unsupported native removal suppresses useful sibling | Removal preparation | Negotiated-family projection before encoding; large/small controls |
| ISSUE | Quiesce can report success with decision or pending write debt | Peer-sync owed check | Current-session table debt included; held/reset/stopped controls |
| ISSUE | Hard removal preparation leaves old routes or miscounts fatal delivery | Forwarding preparation/settlement | Exact-session post-unlock terminal settlement and delivery receipts |
| ISSUE | Refusal diagnostic claims completed withdrawal before the removal outcome | `atomicAggregatePolicyMessage`, `reportAtomicAggregatePolicyRefusal` | Corrected promise and judged phrase assertion committed `bc904308ab`; named diagnostic test `173339b5` passes; focused follow-up clears the ISSUE and final covering artifact is native CLEAN |

## Pre-Commit Verification

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| Caller and helper populations | Yes | Native commits `1d8f79b7c1`, `94079ae78b`, `79aaa2daf4` retain the selected source and tests |
| Canonical daemon, fixture and interop population | Yes | `git show --stat 43d3055e7b` lists all fifteen whole files and narrow summary hunk |
| News fragment | Yes | Current source-only `website/changes/fragments/bgp-atomic-aggregate-export-rewrite.md` |
| Proof and audit companions | Yes | Native commits `7be40717af`, `58f4906f4f` and `34a5530dd8`; exact eleven-commit union contains 122 paths, including authorized handoff `bcc0059b72` |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Refusal of unsafe marked output | Six socket cases and recorded `f3037485`; canonical daemon and live FRR admission walk accepted |
| AC-2 | Useful independent unmarked rewrite | Exact /25 socket and native FRR C/25 controls pass |
| AC-3 | Supported marked edits and subsets | Attribute/reorder/subset/withdrawal/EOR roots and canonical daemon useful output pass |
| AC-4 | Identity, mixed, queue/split and replay boundaries | Complete atomic/helper and affected callback results, with corrected fixture results kept separate |
| AC-5 | Only bounded native covers earn credit | Actual discrimination file contains three bounded covers and the partial verdict; all forty-four affected refresh observations committed |
| AC-6 | Import cannot erase received condition | Import publication roots and canonical daemon exact Adj-RIB-In checks pass |
| AC-7 | Failed survivor cannot erase authorised retirement | Final 180 stale cases; read origin `5247e0e3` and stale `aff65e8d` clean/broken named assertions |
| AC-8 | Completion waits for actual owed work | Final nine peer-sync roots and twenty stopped-worker runs pass |
| AC-9 | Captured-session terminal settlement | Seven settlement roots and thirty affected regressions pass |

### Wiring Verified

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Configured raw import/export and internal/forked typed filter | `test/plugin/atomic-aggregate-policy-admission.ci` | Both config bodies register separate forwarder/filter connections, real source/recipient sockets, Adj-RIB-In and typed transport oracles |
| Native FRR export policy | Interop `bgp-atomic-policy-admission-frr` | Registered SDK helper and checker; injector phases released by real UPDATEs; useful routes, exact absences, source keys and session checked |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| R-1 | Confirmed with admission boundary | Import publication and marker-erasure refusal preserve the source condition |
| R-2 | Confirmed with original input | Final/stale controls retain original `chainInput` despite intermediate output |
| R-3 | Confirmed with native identity | Family callback, label, ADD-PATH-zero, malformed and collision matrix |
| R-4 | Confirmed with owned retirement | Previous-advertisement refusal and exact-owner controls |
| R-5 | Confirmed with ownership controls | Queue/split/cache-expiry and original receipt-buffer transfer tests |
| R-6 | Confirmed with useful controls | Unmarked rewrite, marked attribute/reorder/subset and withdrawal output |
| R-7 | Confirmed with final retirement | Subset/EOR, final-pass and cold-recovery controls |
| R-8 | Confirmed with repaired stale sender | Three observed encoding REDs, final 180 cases and original/joined error assertions |
| R-9 | Confirmed with repaired completion boundary | Real held decision, pending writes, reset and stopped worker outcomes |
| R-10 | Confirmed with captured-session settlement | Callback unlock, replacement, validation, AIGP and healthy-recipient controls |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Marked policy restriction and useful output | Admission helper and common accepting-result boundary | Independent bounded doc review retained; native source anchors all valid, with unrelated catalog drift kept separate |
| Grouping, source ownership and asynchronous recovery | Removal writers, final ownership and recovery producers | Existing source review and held-recovery tests retained |
| Quiesce completion | Current-session `syncOwed`, owed/pending table checks | Final peer-sync evidence accepted |
| Native peer and bounded RFC claim | Actual checker/helper, three canonical covers and partial annotation/audit | Source checked; all forty-four affected observations refreshed and committed; requirement remains partial |
| No config, public SDK envelope or runtime dependency added | Caller changes reuse existing policy/result and owner contracts | No schema migration, SDK compatibility path or product dependency declaration selected |
