# Spec: bgp-best-route-advertised-to-peers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Routing correctness of a shipped capability: under the default configuration a Ze node between two BGP peers passes on only the routes it originates, where RFC 4271 Section 9.2 has a speaker advertise the routes its Decision Process chose |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` (a best route advertised toward an eBGP peer under the default `next-hop auto` needs the rewrite that spec owns); `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` (the commit-time refusal of a missing or narrower `attach process` grant, which transit uses for `bgp-rib`) |
| Phase | 2/5: contracts, source authority and dissemination implementation |
| Handoff | `tmp/session/2026-11-14-210034/state/session-state-bgp-best-route-advertised-to-peers-01a1db03-fbc1-7b32-8cbf-72b6284c110b.md` |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-18 |

Bucket: `plan/release/fixes/`. `plan/README.md` puts incorrect shipped
behaviour there, routing defects named, and `post-release/features/` holds
additional capabilities and absent RFC features. Ze ships a BGP speaker that
implements RFC 4271, and the lab below shows the default configuration failing
to do what Section 9.2 makes the Update-Send process do. That is a defect of
the shipped routing behaviour, not an extra capability, so it is P1 routing
correctness.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner's ruling, 2026-11-13. The deployment-user-journeys session asked:

> "Should Ze pass best routes between BGP peers by default, as FRR and BIRD
> do? Today plain transit needs `bgp-rs` attached to every peer, and it
> forwards every received UPDATE with nothing arbitrating against
> redistribution. If yes, that is a new spec; if no, the guides now say what
> to configure and I leave it."

Thomas answered: "absoletely" (absolutely, yes).

Goal, in his terms: Ze passes the best route it elected between its BGP peers
by default, as an ordinary BGP speaker such as FRR or BIRD does, without the
operator loading or attaching a route-server plugin. The ruling is recorded in
the design page that owns the subject,
`docs/architecture/rib-transition.md`, "Owner ruling: best routes reach the
other peers by default".

Implementation is authorized. On 2026-11-16, Thomas instructed:

> implement plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md no other spec

This instruction authorizes the current advertiser design and its scoped
implementation. Other specs remain outside this session's work. The dated
investigations below retain the approval state at the time they were written;
they do not withhold this authorization. The framing
in the question above ("nothing arbitrating against redistribution") was the
asking session's suspicion; the source trace below distinguishes writer
ownership from route preference arbitration. Dated investigations retain their
original evidence and phase descriptions.

### Owner decision, 2026-11-16: a learned route needs the recipient's export policy

Thomas asked whether Ze should carry BGP routes between its peers through an
explicit BGP-to-BGP redistribution mechanism of its own. The answer he took is
no such mechanism: the explicit decision is the recipient's export policy, and
it is required on every peer, internal ones included. It is recorded in
`docs/architecture/rib-transition.md`, "Owner ruling: a learned route needs the
recipient's export policy".

This narrows the first ruling without reversing it. Best routes still pass
with no route-server plugin loaded or attached. "By default" now means
"wherever the recipient's configured export policy accepts them": a peer with
no export policy is sent no route Ze learned from another peer. RFC 4271
Section 9.1.3 allows it ("This policy MAY exclude a route in the Loc-RIB from
being installed in a particular Adj-RIB-Out."), and BIRD 3.4.0 does the same:
its guide gives the channel option `export` "Default: `none` (except for EBGP
and L3VPN)." FRR advertises to an internal peer with no policy, so the two
disagree and Ze chooses.

Thomas also ruled how far the requirement reaches (Q-5 below): it binds
learned routes carried by ordinary selected output, and leaves local origins
and explicit route-server and reflector roles on today's default.

### Implementation progress, 2026-11-18

This spec remains **in progress**. All 27 acceptance criteria remain in scope;
the entries below distinguish landed work from source integration and proof.
On 2026-11-18, Thomas asked for coherent implementation chunks to be committed
as they become available, including work whose testing still needs updating.
Those commits do not close the spec or claim release readiness.

| Piece | Current state | Evidence and remaining work |
|-------|---------------|-----------------------------|
| Learned-only export policy, reload and diagnostics | Committed in `067a0c5b302cca916135dc13dd0b55292af1cd54` | Scoped policy and diagnostics runs and four functional scenarios passed; final integrated regression remains owed |
| Selected-only replacement-recipient replay, AC-27 | Committed in `96dc0b610e` | Actual socket regression, useful FlowSpec controls and producing-code discrimination passed; final integrated regression remains owed |
| Applied-event completion across plugin transports | Committed in `a2247ff49e190f4b61030a4bee932d472f858c50` and `824dc2284b21e5ab54b883b616c8ba48d9beb242` | Scoped unit, race and producing-code discrimination runs passed; the complete advertiser lifecycle is still being tested |
| Source authority, ordinary selection, retained validation and RS/RR lifetimes | Committed in `a27a9512390843d4d21fd6864f52c4dd649eb460` | The implementation checkpoint passed its native commit gates. Remaining fixture repairs, functional execution and source-fault proof remain in scope. Independent source reviews do not substitute for execution |
| Recovery integration session and owner checks | Committed in `17cb71234606f678ebed537720601fc96084a6e1` by the recovery session | Six companion test files cover original-session EOR, publication and recovery delivery; their owning session retains its scoped execution evidence. Final integrated regression remains owed |
| Mutable mock RTR snapshots and query framing | Committed in `f8ebbaf49b6bd0a1f5a65e7154f3c7dbf13490de` | All seven mock-package roots passed. Removing the producing length guard fails the real TCP query test; the same healthy root then passed with race detection. Repeated race runs and the remaining snapshot discrimination are owed |
| Shared source-preference comparison | Committed in `559acf8707db9bdfca411f4a03d0c7f6983f8926` | Four selected ranking roots and then all 74 Loc-RIB package roots passed with race detection; this does not establish the full advertiser lifecycle |
| Reusable accepted-source snapshot validation and comparison | Committed in `20a28ca82f5656f547e683fc47966358214e306c` | All five original lookup roots passed with race detection after the extraction. The outer reply still checks exact identity, state and frame bounds; these helpers grant no source authority |
| Cached-write settlement primitives | Committed in `bc88d02701e58c7d6c0d51d9f0c870d205eb977a` | Both standalone roots passed with race detection. Removing the nil-cause guard fails the same regression; the healthy source passed again afterward. Engine and SDK adoption remain part of the integration |
| Internal source/owner metadata fields | Committed in `a3db9f5f1447cd830583ddb49af63a005049287b` | All 254 roots across BGP types and WireUpdate passed with race detection. These fields grant no authority and do not by themselves prove actual capture or output |
| Registered route/marker variants and recovery grammar | Implemented in the combined working tree | Three route/marker roots and four recovery roots passed after fixture repairs; final source-fault and integration checks remain owed |
| Real GR/RTR/ordinary-recipient scenario | Driver implemented; four scenarios remain drafts | The missing explicit destination `update-sent` grants and canonical ASN4 attribute expectation are corrected. All six GR-to-RIB source commands, owner-excluded refresh and auxiliary withdrawal debt are implemented in the working tree after independent source review. Scoped execution and the real route-lifecycle proof remain in progress |
| FRR and BIRD user-outcome scenarios | Committed in `865c3e6543f117a3f560f69f653a51a125616e33` | Both checker branches and all three new oracle/preparation roots passed with race detection. Native source/backup/recipient configurations and ordered checkers are present. Both actual pinned-daemon scenarios, producing-code discrimination and final Review Gate remain owed |

The current execution evidence and exact source-review pins stay in the session
state file named above. No acceptance criterion is marked complete solely
because its source has been applied or committed.

After the narrow rerun passed nine selected RS roots and fifteen selected RIB
roots, the broader race run completed with the following results:

| Package | Result at this checkpoint |
|---------|---------------------------|
| RIB | 836 roots passed; 24 failed; no race reported |
| Route server | 193 roots passed; eight failed; two races reported in a structured-retention fixture; a retirement test timed out and prevented five pending roots from finishing |
| Reflector | 62 roots passed; two failed |
| Shared Loc-RIB | All 74 roots passed |

These failures remain part of this spec's work. Fixes must preserve their
behavioral questions; passing counts are not RFC coverage percentages or
acceptance-criterion completion. The native log and individual failures are
retained in the owning session record.

The next targeted RIB rerun passed fifteen of sixteen roots with race
detection, including the original-source recovery selection repair. The
remaining burst root retained all 180,000 pending paths but observed none of
its pending-admission notifications. Both receive paths now propose the
missing notification, with a focused regression awaiting execution; the
original route population and assertions remain intact. The new GR
source-command tests now reach their expected pre-implementation failure at
the missing command registration, rather than failing to compile.

After the pending-admission fix, the unchanged 180,000-route burst and its
focused typed/JSON regression passed. The next scoped race run passed 79
roots and failed two new auxiliary-cleanup roots on their framed transport;
their bridge cases passed and no race was reported. The original 24 RIB
failures passed their targeted reruns. The framed failures were traced to
missing original destination-session provenance in SENT JSON. After the
formatter repair, all eleven selected regression roots passed with race
detection, including every framed and bridge auxiliary-cleanup case.

The subsequent full RS/RR/RIB/GR/formatter race run passed 1,499 roots and
failed five, with no race reported. The complete RS and GR packages passed.
The remaining failures concern one reflector deadline diagnostic, one RIB
large-event request deadline, one structured-DOWN fixture and two native TEXT
expectations. They remain active work; this implementation checkpoint does
not claim that the whole affected test set passes. Four GR/RTR functional
drafts, actual FRR/BIRD interoperability, current RFC proof records and final
independent review also remain owed.

The reflector deadline diagnostic and both TEXT expectations subsequently
passed their targeted race checks after independent review. A new RIB
regression reproduced DOWN returning before a concurrent withdrawal mirror
completed; its reviewed source fix retains the peer reference through that
completion. Both withdrawal roots then passed twenty race runs. Seven RS worker lifecycle roots also
passed twenty race runs after unused discard plumbing was removed.

The native implementation-commit gate then refused thirty RS warnings and
one redistribution warning without declared doctor coverage, plus a refresh
error without an operator fix. Their reviewed fixes are now implemented.
The new diagnostic tests failed against the original warning paths, then all
seven selected diagnostic/withdrawal roots passed with race detection.
Twenty repeats passed 136 of 140 root executions, with no race reported;
four failures came from a new doctor snapshot assertion comparing unordered
status rows. After an independently reviewed multiset assertion replaced the
order-dependent comparison, that root passed twenty race runs. The source
checkpoint then passed its native gates and landed in `a27a9512390843d4d21fd6864f52c4dd649eb460`.

The next full run passed the redistribution package, exposed a route-server
fixture reading an asynchronously changed map without its lock, and stopped
the RIB package at a fixture callback closing its channel twice. The latter
fixture had exposed its manager before the SDK runtime was ready. Reviewed
repairs use the actual readiness handshake and locked observation. All six
reached roots then passed with race detection; repeated and full-package
reruns remain owed. That run did not reach the large-event root. Its isolated
170,000-route race run passed, so the earlier full-run timeout remains under
investigation rather than being declared resolved.

The real GR/RTR draft now reaches retained-route advertisement, withdrawal
and restoration on recipient TCP. Its purge stimulus used a selecting-RIB
command that does not own passive validation cleanup. The independently
reviewed correction reconnects with GR state preserved and sends real
End-of-RIB, requiring passive removal while the successor is still open.
Execution of that correction and all four complete variants remains owed.

### Observed evidence

The 2026-11-18 replacement-recipient probe exposed a competing automatic
Adj-RIB-In replay after ordinary selection had excluded an unresolved route.
This spec owns that correctness repair. A non-authorizing
`ordinary-replay-owned` state advisory stands both auxiliary received-route and
FlowSpec peer-up replay down for the captured ordinary destination. Bookkeeping,
session readiness, no-owner/RS/RR fallback and explicit replay commands remain.
The existing failing `TestRFC4271OrdinaryReplacementRecipientSkipsUnresolvedRoute`
stays unchanged as the source regression; useful selected FlowSpec replay to a
late and replacement recipient must survive the repair.

| Evidence | What it shows | Where |
|----------|---------------|-------|
| netlab reference topology, before commit `ef1a3cf33d` | the render attached only `bgp-rib` and `redistribute-orchestrator` to each peer; r1 never told r2 about r3's loopback, and three of six sourced pings passed | commit `ef1a3cf33d` message; `git show a64267bb2f:plan/handover/deployment-user-journeys-2026-11-13/README.md`, "Progress after the migration" |
| the workaround in commit `ef1a3cf33d` | the render now loads and attaches `bgp-rs` and `bgp-adj-rib-in` on every peer, and sets `next-hop self` toward an external peer; all six pings then passed | `contrib/netlab/ze/ze.j2` |
| the user guide | "When `bgp-rs` is not loaded, received routes are not forwarded to other peers. The `bgp-rib` plugin stores routes and performs best-path selection, but does not re-advertise them." | `docs/guide/route-reflection.md`, "Without Route Reflection" |
| the architecture page | "`bgp-rib` forwards nothing between peers" | `docs/architecture/core-design.md`, the paragraph on a changed grant on a route-consuming process |

### Selected-route investigation, 2026-11-15

Superseded in part. This dated record keeps its evidence. Where it speaks of
deriving default loading, bindings or feeds, the owner decisions of 2026-11-16
replace it: nothing is derived, `bgp-rib` is attached explicitly, and a
learned route needs the recipient's export policy.

This investigation adds learned-BGP defect probes only. The spec remains a
skeleton; neither the advertiser design nor API/redistribution arbitration is
approved by these tests. The main-thread run below reproduces the missing
initial advertisement; no discrimination result is claimed.

| Finding | Producer or contract | Consequence for the design |
|---------|----------------------|----------------------------|
| Selection reaches shared routing state and best-change publication, but supplies no ordinary send | `rib_bestchange.go` `checkRouteBestChange`, `publishBestChanges`; the latter emits `ribevents.BestChange` and the redistribution bridge. A source search of `rs` and `rr` finds no best-change consumer; `bmp_locrib.go` `startLocRIB` subscribes for monitoring | The missing connection is an ordinary advertiser, not another best-path selector |
| Retained received routes already have an owned wire representation with ingress provenance | `rib_recovery.go` `recoveryCandidate` owns attributes, NLRI and next hop and carries source, message ID, source-policy epoch and ADD-PATH framing; `sdk_engine.go` `RelayStoredRoute` reaches the typed bridge or external RPC | Reuse this representation and the existing egress rail. Ordinary export denial of the global winner must withdraw the previous advertisement, without electing a lower per-recipient candidate through `recoveryRoutes` |
| A Loc-RIB forwarding handle cannot be the advertisement carrier | `docs/architecture/rib/forward-handle.md`, "The decisions": a control-plane re-election can carry no handle, and the handle is state-tracker infrastructure. A received handle can contain several NLRIs, including losing candidates | Read the selected retained path; never forward a whole received UPDATE merely because one NLRI became best |
| Writer coexistence and startup remain design work | `reactor_api_relay.go` `RelayStoredRoute` already enforces sender permission and source-dependent egress | Derive default loading and bindings, one opaque forwarding-role claim per destination beside RS/RR, replay and withdrawal ownership. Do not hardcode plugin names in the engine |

The four new probes are in
`internal/component/bgp/reactor/rfc4271_selected_advertisement_test.go`.
They run the current RIB and Adj-RIB-In SDK consumers with established
`net.Pipe` sessions and no RS/RR process or client role. Real terminal host
routes in the admission Loc-RIB resolve both source next hops. Candidate
storage, reachability and the selected Loc-RIB path must succeed before a
missing recipient advertisement can fail the test. The export-denial case
uses a compiled library policy that accepts LOCAL_PREF 100 and rejects 200.
Later source markers bound the unchanged-route observations.

| Probe | Assertion after the positive selection control | Evidence state |
|-------|-----------------------------------------------|----------------|
| `TestRFC4271OrdinarySelectedWinnerReachesPeer` | Two received candidates yield only the selected path at the ordinary recipient | Compiled; initial recipient advertisement absent after successful selection. Later marker and history assertions not reached |
| `TestRFC4271OrdinarySelectedReplacementLifecycle` | A better winner replaces the earlier route, withdrawal restores the retained fallback, and the last withdrawal reaches the socket once | Compiled; initial recipient advertisement absent. Replacement, fallback and withdrawal assertions not reached |
| `TestRFC4271OrdinaryUnchangedWinnerIsNotReadvertised` | An identical receipt and a lower candidate's receipt/removal send no duplicate or withdrawal | Compiled; initial recipient advertisement absent. Repeated receipt and duplicate-suppression assertions not reached |
| `TestRFC4271OrdinarySelectedExportDenialWithdraws` | Excluding the selected winner withdraws the earlier route and never re-advertises the lower retained candidate | Compiled; initial exportable recipient advertisement absent. Selected export denial and withdrawal assertions not reached |

The main thread compiled and ran all four probes on 2026-11-15. Each reached
its first recipient-wire wait after actual next-hop resolution, source storage
and Loc-RIB selection succeeded, then failed with `timed out waiting for
ordinary selected route on recipient socket`. The missing prefixes were
`198.19.241.0/24` through `198.19.244.0/24`. This reproduces the initial
advertisement gap; it does not yet show that replacement, duplicate suppression
or export-denial handling is defective, because those assertions were not
reached. The minimal producer cause is `publishBestChanges`: it emits the
best-change batch and redistribution event, without an ordinary peer sender.

Retain
`tmp/session/2026-11-14-210034/scratch/job-core-bgp-lifecycle-advertisement-bdcd2163.log`
as the observed baseline, specifically its four `TestRFC4271Ordinary` failure
blocks. Remove this spec's retention claim after an implemented advertiser
has been rerun against all four probes and the owning evidence records the
outcomes of their later assertions; another finding may still require the
shared log. No new temporary evidence copy is needed.

Three additional untagged probes in
`internal/component/bgp/reactor/rfc4271_selected_boundaries_test.go` complete
the original test campaign's selected-route boundary cases. They use the same
real SDK consumers, connected next-hop routes and established sockets without
RS/RR. They add no advertiser design or conformance credit. Execution is owed
to the main thread for the later assertions; the initial baseline results are
recorded below.

| Probe | Required useful output before the boundary assertion | Later boundary assertion |
|-------|------------------------------------------------------|--------------------------|
| `TestRFC4271OrdinarySelectedMEDChangeReachesPeer` | The initial winner reaches the recipient; both equal-LOCAL_PREF candidates have AS_PATH beginning with the same AS 65010 | Lowering only the other path's MED changes the selected next hop and sends exactly its replacement, without a withdrawal |
| `TestRFC4271OrdinarySelectedIBGPSplitHorizon` | An eBGP-source control reaches the internal recipient, and the iBGP-source subject reaches a permitted external recipient | The internal recipient receives neither announcement nor withdrawal for the iBGP-source subject, while its external-source control remains |
| `TestRFC8212OrdinarySelectedAbsentExportPolicyWithholdsRoute` | The same selected subject and a later source marker reach an explicitly accepting eBGP recipient | The eBGP recipient with no export policy receives neither route; source ingress, SDK consumer work and the shared forward writer are joined before inspection |

The main-thread scoped run in
`tmp/session/2026-11-14-210034/scratch/job-unit-pkg-334ed25d.log` compiled and
ran all three probes. Each passed source storage, real next-hop reachability
and Loc-RIB selection, then failed at its first required advertisement:
MED's initial winner `198.19.250.0/24`, split horizon's permitted external-source
control `198.19.251.0/24`, and default rejection's explicitly exportable subject
`198.19.254.0/24`. The absence at the control recipient is the observed defect;
the later MED replacement, split-horizon and absent-policy assertions were not
reached. The source cause remains `rib_bestchange.go` `publishBestChanges`,
which emits best-change and redistribution events without an ordinary send.

Retain that shared log for these three named failure blocks until an
implemented advertiser supplies superseding results for all later assertions.
Other findings may still require the same log. This evidence makes no claim
that the shared run as a whole passed or was free of races. The later
implementation must retain the useful-output controls and join any advertiser
work it adds before the negative observations.

#### Q-3: local and learned routes for the same prefix

The owner's then-pending question, "What do BIRD and FRR do?", was investigated
against the lab versions on 2026-11-15: FRR 10.7.1, pinned by digest in
`internal/le/interoplab/discover.go`, and BIRD 3.4.0, selected by
`test/interop/Dockerfile.bird`. The local reference checkouts are FRR 10.5.3
and BIRD 2.19 development, so they were not used as evidence for these version
specific decisions. The primary source links below name the release tags.

RFC 4271 Section 9.1.1 says: "The exact nature of this policy information,
and the computation involved, is a local matter." Section 9.1.3 separately
requires: "All routes in the Loc-RIB are processed into Adj-RIBs-Out according
to configured policy." These clauses do not establish a universal preference
for every route described as local. Origination, cross-protocol selection and
recipient export policy must be distinguished.

| Route form or boundary | FRR 10.7.1 | BIRD 3.4.0 |
|------------------------|------------|------------|
| Explicit BGP origination | `bgp_static_update` creates a self-owned `BGP_ROUTE_STATIC` candidate for a network statement; import-check validity and the network route-map still apply. This is distinct from a system static route. [Source](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c) | `static_announce_rte` inserts an `RTS_STATIC` route into the configured shared table. An inactive unicast next hop suppresses it. Exporting it to BGP does not first turn it into another imported BGP candidate. [Source](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/static/static.c) |
| Redistributed route versus learned BGP | `bgp_path_info_cmp` first calls `bgp_path_info_cmp_distance` when a self-owned `BGP_ROUTE_REDISTRIBUTE` path competes with a zebra-installable path: lower administrative distance wins; equality continues through BGP comparison. Its comment says, "Prefer the path with a lower admin distance." [Source](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c) | `rte_better` compares shared-table preference first, higher winning. Only tied routes of the same owner class reach its protocol-specific comparator; a cross-class tie uses class-address ordering, not BGP LOCAL_PREF. [Source](https://github.com/CZ-NIC/bird/blob/v3.4.0/nest/rt-table.c) |
| Explicit origination versus learned BGP | Outside that redistribution comparison, weight and LOCAL_PREF precede the local-route tie-break. Local defaults include weight 32768, so ordinary network origination normally wins but is policy-sensitive. [Defaults](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c), [constant](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.h) | Static preference 200 and Direct preference 240 exceed BGP preference 100. Import filters or channel configuration can change preference. [Constants](https://github.com/CZ-NIC/bird/blob/v3.4.0/nest/route.h), [Direct producer](https://github.com/CZ-NIC/bird/blob/v3.4.0/nest/rt-dev.c) |
| Merely present in the kernel | Presence alone creates no BGP origination; network or redistribution admission is a separate step. | `krt_got_route` / `krt_learn_scan` admit kernel or alien routes only under the configured learning mode. The Kernel protocol uses inherited preference 10; it is not Static 200 or Direct 240. OS-created connected routes need `learn all` on that import path. [Source](https://github.com/CZ-NIC/bird/blob/v3.4.0/sysdep/unix/krt.c), [guide](https://bird.nic.cz/doc/bird-3.4.0.html#kernel) |
| Selection versus ordinary export | `bgp_process_main_one` passes the selected path to `group_announce_route`; `subgroup_announce_check` applies recipient admission, with FIB-pending suppression an additional configured constraint. | `channel_notify_optimal_req` supplies the selected route to `rt_notify_basic`; `export_filter` applies protocol preexport and the configured filter. `bgp_preexport` accepts non-BGP routes for this next filter stage; `bgp_update_attrs` then supplies missing BGP attributes. It is not an unconditional export grant. [Table source](https://github.com/CZ-NIC/bird/blob/v3.4.0/nest/rt-table.c), [BGP source](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/bgp/attrs.c) |

The following outcomes assume both candidates are valid and admitted into the
same relevant table, ordinary single-path advertisement, accepting export
policy, default distances/preferences, and no policy overrides. FRR's source
distance is carried by `bgp_redistribute_add`; `bgp_distance_apply` resolves
the learned BGP distance. Defaults are static 1, OSPF 110, eBGP 20 and iBGP 200
([constants](https://github.com/FRRouting/frr/blob/frr-10.7.1/lib/frrdistance.h)).
BIRD's corresponding preferences are Static 200, OSPF 150 and BGP 100.
These are source-derived outcomes, not new interop measurements.

| Competing inputs for the same prefix | FRR default winner | BIRD default winner |
|--------------------------------------|--------------------|---------------------|
| Admitted explicit BGP network origination / BIRD Static, versus learned BGP | Network candidate | Static candidate |
| System static redistributed into BGP / BIRD Static, versus learned eBGP or iBGP | Static candidate | Static candidate |
| OSPF redistributed into BGP / BIRD OSPF, versus learned eBGP | Learned eBGP: 20 is lower than 110 | OSPF: 150 is higher than 100 |
| The same OSPF input, versus learned iBGP | OSPF: 110 is lower than 200 | OSPF: 150 is higher than 100 |
| A route imported by BIRD Kernel, versus learned BGP | Depends on its source and how it was admitted into BGP; not the explicit-network case | Learned BGP: 100 is higher than inherited 10 |

An export filter rejecting the winner does not by default promote the loser
for that recipient. BIRD's `secondary` option explicitly changes this and
defaults off; ADD-PATH also changes the output population. Those configured
roles are separate from the ordinary selected-route contract
([BIRD guide](https://bird.nic.cz/doc/bird-3.4.0.html#bgp)).

Ze already has two relevant mechanisms, but their existence does not settle
Q-3. `internal/core/rib/locrib/entry.go` `selectBest` ranks system candidates
by administrative distance then metric. In contrast,
`internal/component/bgp/redistribute/consumer.go` `BGPConsumer.InjectRoute`
dispatches an originated `UpdateRoute` command, and
`reactor_api_batch.go` `announceTarget.initBatchOwner` preserves that local
authority. `session_ownership.go` `adjOutContests` detects competing writer
identities; it does not compare route preferences. The source-bound
`oweAnnouncement` path can defer a contest to recovery, whose
`rib_recovery.go` `recoveryCandidate` supplies retained received-BGP bytes.
This is not yet a demonstrated election spanning explicit API origination,
redistribution and learned BGP.

The implementations therefore answer the comparison question differently for
some inputs, notably OSPF versus eBGP. This dated comparison did not itself decide Ze's origin forms or selection
boundary. The 2026-11-16 owner decision and concrete proposal below supersede
its pending-choice state; no universal FRR/BIRD numeric preference is inferred. The four learned-only regression
probes above remain valid and do not depend on resolving mixed-origin policy.

The next fix work, after design review, is:

1. Read FRR's and BIRD's deciding functions and settle the advertiser's owning
   component, default startup and derived bindings within the plugin boundary.
   Apply the subsequently recorded Q-3 owner decision and scoped admission proposal below.
2. Feed the global selected path to the existing stored-route relay using
   owned route bytes and original source, message ID and source-policy epoch.
   Add an opaque destination forwarding-role claim so ordinary, RS and RR
   writers cannot duplicate ownership. Do not use `ForwardHandle` or a
   plugin-name check in the engine.
3. Carry replacement, final withdrawal and replay through that ownership
   model. Apply recipient export policy to the global winner; when it is
   excluded, withdraw the prior advertisement without selecting a lower
   candidate for that destination.
4. Rerun package `./internal/component/bgp/reactor` with selector
   `^TestRFC4271Ordinary`, the native runner's complete feature tags,
   `CGO_ENABLED=0` and `-count=1` through `./le job run`, then assess every later assertion. Add RFC tags
   only after their claims pass and have recorded discrimination proof.

The probes carry untagged `VALIDATES` comments, and no RFC gap row or
conformance count changes. The fixture starts the two existing RIB consumers
explicitly: a future separate default advertiser will also need its normal
startup discovery represented. Default autoload, forked transport, late-peer
replay and kernel forwarding remain unproven.

### Current research, 2026-11-16

Superseded in part, by the owner decisions made later the same day. The source
facts in its table stand. Its consequences that call for a plugin-owned
default delivery declaration, default grants or derived feeds do not: see
"Explicit grants and role changes".

The main-thread baseline in
`tmp/session/2026-11-14-210034/scratch/job-unit-pkg-3734c8a8.log` ran all seven
existing probes in 37.813 seconds. All seven failed at their first required
recipient advertisement after the fixture's storage, next-hop resolution and
selection checks. The four `rfc4271_selected_advertisement_test.go` probes and
three `rfc4271_selected_boundaries_test.go` probes therefore still reproduce
missing initial useful output. Their later replacement, silence, withdrawal,
MED, split-horizon and export-default assertions were not reached. The log also
contains interface/neighbor watcher warnings; this run does not prove Linux
kernel forwarding or absence of unrelated platform limitations.

RFC qualification from the narrow audit and main-thread source verification:
these seven initial failures establish the missing ordinary Phase 3 integration
under RFC 4271 Section 9.1.3, including "All routes in the Loc-RIB are processed
into Adj-RIBs-Out according to configured policy." They do not prove that the
Section 9.2 Update-Send process failed after an ordinary Adj-RIB-Out had been
populated. Existing RFC4271-9.2-7 tags and records honestly cover explicit
non-ADD-PATH route-server withdrawal/replacement; this baseline supplies no
basis to change their verdict or add a gap/partial annotation to that row.
No existing requirement row directly quotes that Phase 3 processing sentence.
The spec therefore keeps `RFC gaps` as `-`, cites Sections 9.1.3 and 9.2 as
the governing context, and treats RFC4271-9.2-7 as related coverage. The existing
RFC4271-9.2-3 partial coverage already names the later ordinary export-denial
assertions; those assertions remain unreached in these initial-failure runs.
This is a bounded interpretation of the seven probes, not a full RFC audit or
new conformance credit.

Retain that existing log until a superseding run records every later assertion
of these seven probes. It is shared with the preceding atomic-export repair;
check its remaining references before removal. No duplicate evidence export is
needed. No test execution is attributed to the research agent.

| Source fact | Consequence for the proposal |
|-------------|------------------------------|
| `rib/rib_bestchange.go` `checkRouteBestChange` selects globally, compares path identity and attribute fingerprint, and retains the winning candidate until `releaseCandidates` | Capture owned advertisement bytes from that retained revision, before release; a second lookup after publication could substitute another revision |
| `rib/rib_recovery.go` `recoveryCandidate` constructs owned attributes, NLRI and next hop with original message ID, policy epoch and ADD-PATH framing | Reuse the retained-route representation; never relay a whole received UPDATE containing losing sibling NLRIs |
| `reactor/session_ownership.go` `oweAnnouncement` sends a different-owner contest to recovery; `rib/rib_recovery.go` `recoverySurvivorsLocked` returns candidates in preference order after recipient/source exclusions | Ordinary selected output needs a distinct selection contract at the existing writer. Feeding it into recipient recovery can promote a losing route and is not a global-winner implementation |
| `reactor/reactor_api_forward.go` `forwardUpdateCore` synthesizes source-bound withdrawals for export denial | Denial of new winner B does not by itself authorize removing the previously advertised winner A. The selected stream must retire its previous output without giving B arbitrary authority over unrelated writers |
| `reactor/reactor_api_relay.go` `RelayStoredRoute` checks permissions and source liveness, reconstructs pooled received-shape bytes and calls `forwardUpdateCore` | Reuse the egress rail. It is not by itself a final source-down retirement rail: source resolution refuses a departed source before processing its withdrawal |
| `rib/rib_replay.go` `collectPeerUpReplay` skips first-session history and otherwise restores recorded sent routes | First-peer selected-table replay needs a selected snapshot and ordered deltas; existing Adj-RIB-Out history cannot seed a new recipient |
| `rib/register.go` declares config-root autoload, `SignalsSessionReady` and `FencesLiveForwards`; `rib/rib.go` registers startup subscriptions and the external SDK event bus | Extending the existing RIB plugin can reuse startup, forked transport and replay fencing; autoload alone does not grant peer delivery or send permission |
| `bgp/config/redistribute_binding.go` `wireRedistributeDelivery` derives RIB feeds for redistribution, FlowSpec or AIGP | Ordinary default delivery needs a plugin-owned declaration consumed generically, including process aliases and explicit narrower grants; do not add engine/config checks for plugin names |
| `rib/source_policy.go` `applySourcePolicy` denies captured epochs, settles sent removals and re-elects before its receipt; `session_ownership.go` `allow` fences stale source-policy announcements | New selected dispatch must preserve original provenance and participate in apply/abort completion and drain. Never restamp old work with the current policy epoch |
| `docs/architecture/plugin/plugin-system.md`, "Exclusive role claims", describes `Claims`, `ConfigureInput.Claims`, `ClaimActive` and per-event `UnheldRoles` | Extend the existing opaque claim mechanism for ordinary/RS/RR exclusivity; prove destination scope and reload transfer instead of creating a parallel claim registry |

RFC 4271 Section 9.1.3 was read in `rfc/full/rfc4271.txt`: "All routes in
the Loc-RIB are processed into Adj-RIBs-Out according to configured policy."
It also says: "If a route in Loc-RIB is excluded from a particular
Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be
withdrawn from service by means of an UPDATE message (see 9.2)." These decide
winner-then-export and old-output retirement; they do not decide Q-3's
admission of API origins into a common selection domain.

#### Alternatives for the design gate (unapproved)

| Alternative | Existing mechanisms used | Trade-off |
|-------------|--------------------------|-----------|
| Extend `bgp-rib` with ordinary selected advertisement (research recommendation) | Retained election, SDK relay, existing sent inventory, source-policy lifecycle and session-ready fence | Keeps selection and its owned snapshot together; requires careful dispatch ordering and adds advertisement work to the existing plugin |
| Separate ordinary advertiser plugin | RIB selection publication plus a self-contained selected-route snapshot/query contract; SDK relay and existing claims | Separates scheduling, but adds another process dependency, snapshot/delta protocol and startup/reload lifecycle. It must not select independently or import the RIB plugin |

Both alternatives need selected-output authority at the existing final writer,
distinct from original source provenance. A newer accepted selection can replace
the earlier ordinary output; an excluded winner removes that output; stale
selection work cannot remove a newer replacement. The receipt shape and its
relationship to exact-owner retirement remain design work, not a new API
contract approved by this research. Neither alternative moves selection into
the reactor or uses UPDATE-driven RS forwarding as the ordinary selector.

Q-3 policy is resolved by the owner ruling recorded in
`docs/architecture/rib-transition.md`, "Owner ruling: local and learned route
competition": use configured preferences, following FRR/BIRD where they agree.
A valid local announcement is not unconditionally pinned until withdrawal.
The concrete scoped admission and API-origin preference mapping below remain
proposed implementation semantics for whole-design review. Main owns that
approval; the first-choice answer does not authorize implementation.

The parallel claims/bindings trace adds constraints for main-thread source
verification. `rs/register.go` also declares the BGP config root, so absence of
an explicit RS block does not prove the RS process is absent; actual per-peer
grants must decide the role. Existing Claims is a daemon-wide promise with
per-event retraction, not a unique destination owner: multiple claimants can
hold the token. `server/events.go` also checks send-update authority for the
existing peer-up replay claim. A new forwarding claim needs an explicit
collision rule and generic requirements, not an assumption of uniqueness.

That trace finds receive-only binding changes handled live, while changed
send/OPEN grants restart the session. The proposal may reuse that restart as
the ownership-transfer cut; keeping the session live instead would require
quiesce/revoke, replacement-owner replay and live-release fencing. Existing
claims alone perform none of that transfer. Third-party Stage-1 declarations
must be resolved before first peer events too. These are design constraints,
not approval of either transfer approach.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/rib-transition.md` - the design this ruling changes: the engine caches bytes, a plugin decides forwarding
  → Decision: the owner ruled that best routes reach the other peers by default ("Owner ruling" section)
  → Constraint: the engine holds no policy and no Loc-RIB (`ai/doors/bgp-engine.md`, "Where it sits"); a design that moves best path into the reactor departs from that page and needs a discussion with Thomas first
- [ ] `docs/architecture/core-design.md` - "Forwarding Path", "BGP RIB Best-Path Tracking", "The Two Deliveries a Redistribution Rule Derives"
  → Constraint: three forwarding paths exist today (the reactor RS fast path, the text `cached` path, the typed `ForwardCached` path), and the last two end in the shared `ForwardUpdate` core
  → Constraint: autoload and peer grants are separate. By owner decision no grant is derived for transit: the operator attaches `bgp-rib` explicitly, and a missing or partial grant is refused at commit ("Explicit grants and role changes"). No route-server attachment is needed
- [ ] `docs/architecture/rib/forward-handle.md` - optional state-tracker bytes; re-election can carry no handle, and RS/RR never use it
  → Constraint: selected advertisement uses one retained route revision, never `ForwardHandle` bytes that may include losing sibling routes
- [ ] `docs/guide/route-reflection.md` - "Forwarding and Destination Election", "Plugin Bindings", "Without Route Reflection"
- [ ] `docs/architecture/plugin/plugin-system.md` - the process boundary: any design must work with the deciding plugin forked
  → Constraint: reuse opaque `Claims` and per-event `UnheldRoles`; prove their destination and reload semantics. DirectBridge is an optimization of the same external protocol, not permission to pass pointers across plugins

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - rows RFC4271-9.2-6, RFC4271-9.2-7, RFC4271-9.2-9, RFC4271-9.2-2, RFC4271-9.2-3
  → Constraint: RFC4271-9.2-7 carries no gap annotation, though under the default configuration no newly installed route reaches another peer; the design phase checks whether that row's claim holds and corrects the summary if it does not
- [ ] `rfc/short/rfc8212.md` - RFC8212-3-1, RFC8212-3-2
- [ ] `rfc/short/rfc4456.md`, `rfc/short/rfc7947.md` - the reflector and route server roles that must keep working

### RFC text (read from `rfc/full/rfc4271.txt` and `rfc/full/rfc8212.txt`)

| RFC section | Sentence, byte-exact | Why it decides |
|-------------|----------------------|----------------|
| RFC 4271 Section 9.1 | "The output of the Decision Process is the set of routes that will be advertised to peers; the selected routes will be stored in the local speaker's Adj-RIBs-Out, according to policy." | the elected route is what a speaker advertises |
| RFC 4271 Section 9.1.3 | "All routes in the Loc-RIB are processed into Adj-RIBs-Out according to configured policy." | dissemination is of Loc-RIB routes, the elected ones |
| RFC 4271 Section 9.1.3 | "This policy MAY exclude a route in the Loc-RIB from being installed in a particular Adj-RIB-Out." | exclusion is policy, per destination |
| RFC 4271 Section 9.1.3 | "When the updating of the Adj-RIBs-Out and the Routing Table is complete, the local BGP speaker runs the Update-Send process of 9.2." | Phase 3 drives Update-Send |
| RFC 4271 Section 9.2 | "The Update-Send process is responsible for advertising UPDATE messages to all peers." | the default this ruling restores |
| RFC 4271 Section 9.2 | "When a BGP speaker receives an UPDATE message from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing information contained in that UPDATE message to other internal peers (unless the speaker acts as a BGP Route Reflector [RFC2796])." | iBGP split horizon, RFC4271-9.2-6 |
| RFC 4271 Section 9.2 | "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message." | RFC4271-9.2-7 |
| RFC 4271 Section 5.1.3, item 1 | "When sending a message to an internal peer, if the route is not locally originated, the BGP speaker SHOULD NOT modify the NEXT_HOP attribute unless it has been explicitly configured to announce its own IP address as the NEXT_HOP." | next hop toward iBGP; the eBGP default is owned by the Depends spec |
| RFC 8212 Section 3 | "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied." | a default that advertises toward eBGP meets this MUST |

**Key insights:**
- The Decision Process exists (`bgp-rib`), and forwarding exists (`bgp-rs`, `bgp-rr`, the reactor fast path). What is missing is that the elected route reaches the other peers without the operator adding a route-server plugin.
- `bgp-rs` forwards received UPDATEs, not the Loc-RIB; a destination without ADD-PATH defers to the `bgp-rib` election only where sources contest a prefix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `publishBestChanges` emits the elected-route changes as one batch on the event bus, then calls the BGP redistribution bridge directly
- [ ] `internal/component/bgp/plugins/rib/forward_tracker.go` - `forwardStateTracker` records the prefixes whose Loc-RIB best is a BGP path and reads optional `ForwardHandle` bytes; it sends nothing
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` - `batchForwardUpdateSkipped` batches cached message ids toward the peers the reactor fast path did not serve
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the forward loop suppresses an iBGP source toward an iBGP destination unless one of them is a reflector client (`isReflected`)
- [ ] `internal/component/bgp/reactor/session_ownership.go` - `oweAnnouncement` and `adjOutContests`: the destination writer detects a second owner for a key and, while a recovery owner receives the source's UPDATEs, owes the key to the RIB election
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop` maps `NextHopAuto` and `NextHopUnchanged` to the same no-op mode
- [ ] `internal/core/redistevents/registry.go` - `WouldLoop` returns true when source and destination protocol names are equal, so BGP is never redistributed into BGP
- [ ] `internal/component/bgp/redistribute/producer.go` - `EmitBestChange` turns `bgp-rib` best changes into redistribution route changes for other protocols

**Behavior to preserve:**
- A configuration that attaches `bgp-rs` or `bgp-rr` keeps its route server or reflector behaviour (RFC 7947, RFC 4456), the reactor fast path included.
- iBGP split horizon (RFC 4271 Section 9.2) and the reflector rules in the forward loop.
- The destination writer's per-key ownership rules (a withdrawal from another source never erases a replacement or a locally originated route).
- Redistribution of BGP into other protocols, and of other protocols into BGP.

**Behavior to change:**
- Under the default configuration, the best route Ze elected reaches its other BGP peers, subject to policy and the RFC rules above, with no route-server plugin loaded or attached by the operator. How is the design question.

## Existing Mechanisms (MANDATORY)

The original mechanisms were read at the producing code or its page on
2026-11-13; their dispositions below now reflect the 2026-11-16 unapproved
whole-design recommendation. Q-3 policy is resolved; whole-design approval
remains pending.

The following 2026-11-16 rows extend that historical inventory. Their proposed
dispositions remain subject to the design gate.

| Mechanism | Where | Current behavior | Proposed disposition |
|-----------|-------|------------------|----------------------|
| Selected route snapshot | `rib_bestchange.go` `checkRouteBestChange`; `rib_recovery.go` `recoveryCandidate` | Election holds one retained winner; the recovery converter owns its wire bytes and provenance | Extend extraction at election, without a second lookup or recipient-specific election |
| Stored-route SDK relay | `reactor_api_relay.go` `RelayStoredRoute`; `pkg/plugin/rpc/types.go` `StoredRoute` | Permissioned received-shape reconstruction enters shared egress; old source-policy epoch remains explicit | Extend with the agreed selected-output receipt; preserve existing callers and both transports |
| Recipient recovery | `session_ownership.go` `oweAnnouncement`; `rib_recovery.go` `recoverySurvivorsLocked` | Contests can try multiple candidates after destination exclusions | Preserve for existing roles; do not use its candidate ladder for ordinary selected output |
| Opaque role claims | `plugin-system.md` "Exclusive role claims"; registry `Claims`, SDK `ClaimActive`, event `UnheldRoles` | Startup claims precede peer events and can be retracted per event when the peer feeds no claimant | Extend existing declarations and prove destination scope/reload transfer; no parallel role registry |
| Sent history and initial fence | `rib_replay.go` `collectPeerUpReplay`; `rib/register.go` | Reconnect history and initial-ready/live-forward fencing already exist | Extend with selected snapshot/deltas for first and later recipients; retain unrelated local history |
| Atomic source-policy removal | `source_policy.go` `applySourcePolicy`; writer `allow` | Captured paths become ineligible, sent owners are retired, re-election runs, stale epochs are refused | Preserve and include selected dispatch completion, rollback and drain in the same contract |
| Absent-policy end disposition | `internal/component/bgp/filterapi/policy.go` `UnconfiguredAccepts`, read by `Policy.AcceptsAtEnd` and by `policychain.CompileUnconfigured` | The one statement of what an unconfigured direction does at the end of its list: accept on an internal session, reject on an external one (RFC 8212 Section 3). It answers for both directions and for every writer: ordinary, route server, reflector, local origin and replay | Extend that one statement with the export case the owner decision adds (Q-5). No second evaluator beside it, and the import answer does not change |
| Absent-policy report | `internal/component/bgp/reactor/rfc8212_warning.go` `RFC8212Absences`, `warnRFC8212AbsentPolicy` | One value per closed direction of an external session, with its message and the editor command that fixes it; `ze config validate` reports it and the reactor logs it at Established. It answers nothing for an internal session | Extend with the internal session's absent export policy, worded for what it withholds and not as an RFC 8212 requirement; the same value feeds validate, the log and the doctor check owed below |

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `bgp-rib` election and best-change batch | `rib/rib_bestchange.go` `publishBestChanges`; `core-design.md` "BGP RIB Best-Path Tracking" | elects the best path per prefix and publishes the changes; subscribers include BMP Loc-RIB monitoring (`bmp/bmp_locrib.go`) and flow-export enrichment (`flowexport/enrichbgp.go`), and the BGP redistribution bridge is called directly; neither `bgp-rs` nor `bgp-rr` subscribes | Extend retained selection capture as the ordinary advertisement source; preserve monitoring/redistribution publication |
| Loc-RIB mirror and `ForwardHandle` | `rib/forward_tracker.go` `forwardStateTracker`; `docs/architecture/rib/forward-handle.md` | carries optional received UPDATE bytes to state trackers; control-plane re-election can carry nil, and a received UPDATE can contain several NLRIs | does not apply as the advertisement carrier: the documented contract is state tracking, and received bytes do not identify only the selected path |
| `bgp-rs` forwarding | `rs/server_forward.go` `batchForwardUpdateSkipped`; `docs/guide/route-reflection.md` "Forwarding and Destination Election" | forwards each received UPDATE by cache id to the other peers it is attached to; a destination without ADD-PATH defers to the `bgp-rib` election only where sources contest a prefix | Preserve configured RS behavior under its eligible destination claim; replace plain-transit use with ordinary selected output |
| Reactor RS fast path | `reactor/forward_rs.go` `reactorForwardRS`; `core-design.md` "Forwarding Path" | forwards inline on the read goroutine for peers with `rs-fast-path`, handing back to `bgp-rs` what it cannot serve | Preserve for eligible RS-owned destinations; ordinary output uses selected reconstruction/shared egress, never UPDATE-driven fast-path selection |
| Forward loop rules | `reactor/reactor_api_forward.go` (`isReflected`, the eBGP AS_PATH prepend, the egress chain) | applies iBGP split horizon, reflector rules, AS_PATH prepend and egress policy per destination | extends: any advertiser reaches the destination through these rules, never a second copy of them |
| Destination writer ownership | `reactor/session_ownership.go` `oweAnnouncement`, `adjOutContests` | tracks the owner of each advertised key and owes a contested key to the RIB election while the recovery owner receives the source's UPDATEs | Extend final-writer selected authority separately from original source provenance; existing contest detection is not mixed-origin preference selection. The resolved Q-3 policy and scoped-origin proposal below define that input boundary |
| `bgp-rr` | `internal/component/bgp/plugins/rr` (registered as `bgp-rr`, "Route Reflector") | reflects iBGP routes under RFC 4456 when attached | does not apply as the default (reflection is a configured role), but must keep working beside it |
| Redistribution | `internal/core/redistevents/registry.go` `WouldLoop`; `internal/plugins/redistribute`; `bgp/redistribute/producer.go` `EmitBestChange` | carries routes between protocols and refuses BGP into BGP | Preserve configured cross-protocol admission, never use BGP-to-BGP redistribution as carrier; admitted origins use the Q-3 configured-preference proposal below |
| Derived process binding | `core-design.md` "The Two Deliveries a Redistribution Rule Derives" | a redistribution rule derives the `attach process` binding a peer needs, so the operator does not type it | Does not apply as a carrier: by owner decision no binding is derived for transit. Reuse only its alias resolution, to name the process in the missing-grant report |
| Config-root auto-load | `core-design.md`, protocol-agnostic plugin loading; `bgp-rib` auto-loads under its registry name | starts a plugin the config implies without a `plugin` block | Reuse existing RIB autoload; loaded RS presence alone does not claim a destination without eligible peer grants |
| Filter insertions | `rpc.FilterInsertion`; `ai/doors/bgp-plugins.md` "Boundaries" | a plugin inserts the filter-list members its config implies | Preserve existing inserted filters and shared egress rules; the advertiser adds no duplicate protocol-policy evaluator |
| RFC 8212 import default | `rfc/short/rfc8212.md` RFC8212-3-1; `bgp-rib` source policy | `bgp-rib` enforces the default import rejection | Preserve source-policy admission; selected operation reaches existing egress export default and requires explicit eBGP export policy |
| Next-hop default | `plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` | owns the `next-hop auto` rewrite toward eBGP that `precomputeNextHop` lacks | dependency: an elected route advertised to eBGP under the default is wrong until it lands |

## Approved implementation design

Q-3 records the resolved owner policy and its proposed concrete mapping. The
other rows are technical recommendations grounded in the existing mechanisms and RFC constraints, to be
reviewed together as one design, not put to Thomas as separate implementation
questions. Their recommendation is not implementation authorization.

| # | Subject | Recommendation | Alternative and why not recommended |
|---|---------|----------------|-------------------------------------|
| Q-1 | Advertiser owner | Extend `bgp-rib`, where `checkRouteBestChange` already owns the retained selected revision and source-policy lifecycle | A separate advertiser uses the same selection but adds snapshot/query and startup ordering; RS mode conflates ordinary election with UPDATE-driven role behavior; reactor selection violates the plugin boundary |
| Q-2 | RS/RR coexistence | Resolve one effective dissemination owner per destination from existing Claims declarations plus actual receive/state/send grants. An explicit eligible RS/RR role displaces ordinary output; ambiguous eligible explicit claimants are rejected during config verification | Parallel ordinary and role writers leave duplicate/recovery contests; daemon-wide ClaimActive alone wrongly suppresses unrelated destinations |
| Q-3 | Local/redistributed and learned origins | Owner resolved configured-preference arbitration. Proposed scoped admission uses existing Ze distance override → protocol declaration, then metric/stable tie; explicit BGP local origin has a distinct identity mapped to the configured static preference class as a new proposal | Unconditional local authority is rejected by the owner policy. FRR/BIRD disagree on OSPF/eBGP defaults, so preserve Ze defaults rather than invent a common numeric ranking. Exact API mapping is proposed, not an existing source fact |
| Q-4 | Protocol egress | Use existing `forwardUpdateCore`, including `isReflected`, source/destination context, next-hop transformation and policy chain | Duplicating split horizon/reflection in the advertiser creates a second implementation and risks different initial/replay behavior |
| Q-5 | Export default | Owner decided: a route learned from one peer reaches another only where the recipient has a configured export policy, on internal sessions as on external ones, and no BGP-to-BGP redistribution mechanism is added. Reach, also owner decided: the requirement binds learned routes carried by ordinary selected output. A local origin (API, configured or redistributed) and an explicit RS/RR destination keep today's `UnconfiguredAccepts` answer, because an announcement, a redistribution rule and a role attachment are each already an explicit operator decision. External sessions keep RFC 8212 unchanged for every route | Requiring export policy for every route on every writer is one simpler rule, but it stops every reflector and every API announcement toward an internal peer that has no export policy, and a migrated ExaBGP config then needs an export policy that accepts its own announcements, which opens transit unless that policy can match local origin. A separate BGP-to-BGP redistribution rule was set aside: redistribution carries a prefix for the destination protocol to originate afresh, where a BGP route must keep its path attributes, split horizon and reflection rules. FRR's default (advertise to an internal peer with no policy) was set aside by the owner decision |
| Q-6 | Existing configs | Convert plain-transit examples and fixtures to ordinary output with an export policy on each recipient, the netlab render included; preserve explicit RS/RR tests/configs, and revalidate replay-only fixtures against unintended default output. A config with internal peers and no export policy gains no new output, so its conversion is needed only where a test expects transit | Leaving the transit workaround teaches obsolete configuration; mechanically removing every RS attachment would destroy role-specific coverage |
| Q-7 | Process boundary | Use one registered selected operation with typed and external SDK transports; owned value snapshots and authenticated process identity | Internal-only callbacks or references to RIB storage break forked plugins |
| Q-8 | Trigger and selection | Publish only changes of the global selected route; duplicate/lower-candidate changes do not enqueue an advertisement | Forwarding every UPDATE and relying on recipient recovery can advertise losers; RFC 4271 Section 9.1.3 requires selected-route dissemination |
| Q-9 | Late-peer/reconnect replay | Selected snapshot plus ordered deltas before the existing initial-ready fence opens; reconcile admitted local origins in the same scope before replay; preserve unrelated role-owned sent history | Sent-history-only replay cannot seed a first recipient; replaying every received candidate violates ordinary single-best output |

### Export default by route kind (reach decided, mechanism proposed)

The owner decided what the default is. How the code tells the two kinds apart
on one peer is proposed here. Source facts first.

`internal/component/bgp/filterapi/policy.go` `UnconfiguredAccepts` takes one
input, whether the session is external, and `Policy.AcceptsAtEnd` passes it
through. Every reader asks per session, never per route:

| Reader | What it decides today |
|--------|-----------------------|
| `internal/component/bgp/reactor/filter_ordered.go` `runEgressPolicyChain`, `runEgressPolicyChainASN4` | The forward rail's end disposition for a destination with no active export filter |
| `internal/component/bgp/reactor/egress_inject_filter.go` | The same for an originated, injected or redistributed body, which has no source peer |
| `internal/component/bgp/reactor/forward_rs.go` | Whether the route-server fast path serves a destination |
| `internal/component/bgp/policychain/chain.go` `endFinal` | The end of a chain compiled for an unconfigured policy, read from the view's session |
| `internal/component/bgp/reactor/peer_session_factory.go` `egressFinalAccepts`, read by `session_write.go` `writeUpdateBody` | The final writer's recheck: one answer for the whole body, so a forward that outlived its policy generation becomes withdrawals |
| `internal/component/bgp/reactor/peer_settings_apply.go`, `policy_reload.go` `reconcileExportFinal` | Whether a reload changed the session's final, and what to do about it |
| `internal/component/bgp/reactor/policy_dryrun.go`, `reactor_api.go` `policyInfo` | What the dry run and the status surface report as the direction's final |
| `internal/component/bgp/reactor/peer_initial_sync.go` `defaultOriginateChain` | The end of the default-originate condition chain |

`reconcileExportFinal` shows the cost of one answer per session. When the
final rejects it calls `reactor_api_advertised.go` `withdrawPeerAdvertised`,
which withdraws the destination's whole Adj-RIB-Out. When the final accepts
again it restores static, default-originate and plugin routes and asks for a
replay. An internal session whose export policy is removed must instead lose
its learned routes and keep its local ones.

| Boundary | Proposed behavior | Existing mechanism extended |
|----------|-------------------|-----------------------------|
| Disposition input | The absent-export disposition takes a second input beside the session type: the route's export kind, learned-by-ordinary-output or not. External rejects both kinds (RFC 8212 Section 3). Internal rejects the first and accepts the second. Import keeps its one input. A configured policy's final applies to every kind, as today | `UnconfiguredAccepts`, kept as the one statement; no second evaluator |
| Where the kind comes from | The engine derives it and no caller asserts it. The selected envelope already separates a learned-source receipt from a local-origin receipt, and the destination's effective dissemination owner (Q-2) says whether an explicit role serves it. A route is the first kind only when it carries a learned-source receipt and ordinary selected output owns the destination. The existing originated, RS and RR rails are the second kind by construction | Authenticated selected operation, `fwdItem` ownership (`originated`, received peer), Q-2 role resolution |
| Per-route readers | The forward rail, the injected rail, the chain end and the final writer ask with the kind of the route in hand. The chain end reads it from the view, which the engine fills. The final writer's recheck judges each owned item by its kind, so a body mixing kinds is never refused or passed as one | `runEgressPolicyChainASN4`, `policychain` `endFinal`, `writeUpdateBody` |
| Status surface | `plugin.PolicyInfo` (`internal/component/plugin/types_bgp.go`) answers one final per direction and is empty for an absent policy on a known internal session. For an absent export policy on an internal destination that ordinary output owns, it reports the final for learned routes as reject, with a final source of its own that names the owner's export rule and is never `rfc8212`, and states that local routes are sent. A role-served destination, the import direction and an external session report as today | `reactor_api.go` `policyInfo`, `absentPolicyInfo`; the JSON keys stay kebab-case, and every writer and reader of the value is a row of the changed-wire census, the handler's `toPolicyChain` copy included |
| Dry run | `plugin.PolicyDryRunner.PolicyDryRun` takes a peer, a direction, a filter override and an UPDATE, and no route kind. It gains a simulated kind for the export direction, `learned` or `local`. The kind is display only: it selects which answer the dry run reports and authorizes nothing, since a real route's kind is still derived by the engine. Omitted, it is `learned`, the stricter answer, and the result states which kind it simulated. An unknown value is refused with the two valid ones. Given with the import direction it is refused, since import has one answer. For a role-served destination the result says the role's default applies whatever the kind | `policy_dryrun.go`; the `show policy test` handler in `internal/component/bgp/plugins/cmd/policy/handler.go`, which gains the argument as a keyword its YANG declares; `plugin.PolicyDryRunResult` gains the simulated kind |
| Reload: policy removed on an internal session | Retire the entries whose recorded owner is a learned route under ordinary output, and only those, after the same writer barrier. Local origins and role-owned entries are not withdrawn and not re-sent | `reconcileExportFinal`; the Adj-RIB-Out entry already records its owner, so retirement selects by that record and never re-derives a kind |
| Reload: policy configured on an internal session | Restore learned routes from the current selected snapshot through the selected stream, under the existing policy-replay hold. No Adj-RIB-In candidate replay, which could install a loser | Policy-replay hold in `reconcileExportFinal`; the selected snapshot of Q-9 |
| Rollback and late work | An aborted apply restores the previous disposition and its output through the same two paths. Work queued under the accepting disposition and released after the removal is refused at the final writer, which reads the current disposition for that item's kind | Existing final-writer recheck and apply/abort barriers |
| Absent-policy report | An internal session raises the export absence only where ordinary selected output owns the destination. A reflector client or route-server client served by its explicit role raises nothing for it. External sessions keep the RFC 8212 absences unchanged | `RFC8212Absences`, read by `internal/component/bgp/config/register.go` `policyAbsencesFromTree` for `ze config validate` and by the Established log line |

### Route delivery to a process stays explicit

The owner's remark of 2026-11-16: delivering routes to an API process should
be explicit too. It already is, and this spec keeps it.
`docs/architecture/api/architecture.md`, "Attaching a Process to a Peer",
states the model: "A peer that attaches no block for a program is fed to it
never, and refuses every message the program aims at that peer. Silence is the
default, and the config is what breaks it."

| Subject | What this spec does |
|---------|---------------------|
| An operator's process, in any language, and the ExaBGP bridge | Nothing changes. It receives a peer's routes only where that peer's `attach process` block grants `receive`, and sends only where it grants `send`. Ordinary selected output never widens a grant, and no default feed is derived for such a process |
| The deciding plugin, `bgp-rib` | Explicit too, by owner decision: that page says the silence "holds for ze's own plugins as it holds for an operator's program", and this spec keeps it. Transit needs `bgp-rib` attached to the peers involved. No route-server plugin is needed, which is what the first ruling asked. The bindings a redistribution rule, a FlowSpec family or AIGP derive today are outside this spec and unchanged by it |
| A peer's own grant | Is the only grant. One too narrow for transit is refused at commit with the line that completes it, never widened |

### Concrete selected-output mechanism (proposed)

The resolved Q-3 policy applies only to admitted origins: eligible learned BGP,
explicit local BGP announcements in their authorized peer scope, and routes
admitted by configured redistribution. System/FIB presence alone is never
permission to export. The concrete scope and preference mapping immediately
below is proposed for whole-design approval, not implemented behavior.

### Admitted origin scopes and preference arbitration (proposed after Q-3)

Source verification distinguishes three inputs. `sdk_engine.go` `UpdateRoute`
and `UpdateRouteSel` carry a selector and command; `rpc.UpdateRouteInput` has
no protocol or administrative distance. `announceTarget.initBatchOwner` marks
output `originated`, and the RIB's SENT handler records `LocalOrigin` in
`ribOutEntry`; neither is a selection candidate. `rib_inject.go` monitored BMP
injection explicitly does not select, and pretending a local origin is a
received peer is not an admission mechanism. The current system Loc-RIB is
therefore not already a unified advertisement domain.

`config/redistribute/consumer.go` `RouteEntry` carries `Source`, targeted `Peer`
and the accepted `Route` view. `redistribute.go` `dispatchEntryToConsumer`
constructs these after configured admission, but BGP `InjectRoute` ignores
source/view and formats an ordinary UpdateRoute command. Its withdrawal API
carries only family/prefix. Preserve origin provenance before that lossy step;
do not reconstruct protocol or distance from BGP ORIGIN or the sender name.

| Input or boundary | Proposed admission and ranking | Preservation and authority |
|-------------------|--------------------------------|----------------------------|
| Learned BGP | Keep the existing global BGP best-path election, including LOCAL_PREF, conditional MED and stale handling, over admitted learned candidates. Its one selected route enters cross-origin arbitration with its actual eBGP/iBGP distance class | Never replace BGP best-path with system AD ordering among learned peers. No destination filter can nominate a second learned candidate |
| Explicit API/configured BGP origin | Retain a distinct local-BGP origin kind and producer identity. Proposed new mapping: use the existing configured `static` preference class (default 10), without pretending the route is a system static route. Explicit accepted distance override, where supplied through the existing policy contract, precedes that class | This mapping is new proposal semantics, not today's UpdateRoute behavior or an owner-approved numeric mapping. It matches the common default local-origin-winning outcome of FRR/BIRD, not their identical algorithms. No implicit FIB install; BGP attributes and late-bound next-hop behavior remain those of local origination |
| Configured redistribution | Admit only the configured consumer's accepted source contribution, carrying source protocol/instance, effective policy override, metric, native identity and admission generation. Use original protocol class, not the redistribution sender's name or local-BGP class | Mere system winner visibility creates no candidate. Preserve redistribution loop checks, accepted transforms and targeted replay. Withdrawal removes the exact source contribution, not another origin's route with the same prefix |
| Cross-origin order | Resolve override first, then current Ze declared protocol distance; lower wins, then lower metric, then stable admission order as in `locrib/entry.go selectBest`. Reuse/extract that comparison rather than duplicate a private numerical policy. An explicit local origin without a metric uses zero; retain accepted metrics for other origins | `locrib/distance.go resolvedDistance`, `distance/distance.go` and sysrib YANG define static 10, eBGP 20, OSPF 110, IS-IS 115 and iBGP 200. Preserve them and configured overrides where FRR/BIRD differ. No new OSPF/eBGP default or config leaf |
| Recipient scope before election | Capture the concrete peer identities admitted by the existing selector and sender grants; a targeted origin competes only in that scope. Each scope combines its admitted local/redistributed origins with the same common learned winner. Keep shared candidate storage plus scope membership, not copied per-peer candidate RIBs | A targeted A-only announcement cannot suppress or reach B. Wildcards retain existing selector resolution semantics; new/restarted sessions replay only their admitted origins through existing lifecycle, not an expanded guessed selector |
| Recipient export after election | Elect once for the admitted scope, then apply existing egress to that exact winner. Denial or split-horizon exclusion retires the previous selected output; it does not retry local, redistributed or learned losers | Scope admission is not an export-policy verdict. A peer outside a local origin's scope may use the common learned winner, but a peer inside the scope cannot fall back merely because its selected origin is denied. A recipient with no configured export policy excludes a learned winner the same way a rejecting policy does: its previous output is retired and no local or learned loser is promoted (Q-5) |
| Origin replacement and removal | Retain source-owned candidate identity and generation across output changes. Replacement, source withdraw, configured-route restoration, producer exit, permission removal and redistribution-rule removal update only that origin and re-elect affected scopes | A losing but still admitted origin remains available for a later genuine winner withdrawal. A loser withdrawal cannot erase another origin's current output. Existing keep/withdraw-on-exit behavior governs candidate lifetime |
| Preference reload and replay | Re-resolve affected admitted candidates on existing distance/policy apply and rollback; issue new scoped selected revisions and join the same final-output barriers. Snapshot membership, candidate provenance and preference generation together | Config changes can replace a still-valid local origin with learned output without waiting for withdrawal. Reconnect/first-peer replay never independently resurrects old local SENT history over the current selected result |

The engine validates syntax, original sender and scope using its existing API
path, then offers the owned local-origin value to the registered deciding
plugin before direct output at ordinary destinations. Extend the existing
registered plugin request/response pattern with a typed fast path and the same
external RPC; no reactor import of `bgp-rib`, shared-memory callback-only
admission or engine candidate election. The plugin owns candidates and returns
admission/completion receipts. The current request's cancellation and error
semantics remain explicit; an accepted losing candidate is retained but does not increment Announced.
Preserve existing counts, including completed withheld withdrawals; the
explicit admission receipt below distinguishes acceptance from output. Received-origin maintenance,
SENT replay and raw/control messages are not new local candidates. Explicit
RS/RR destinations keep their role contract, not a second ordinary writer.

#### Origin admission result transport (proposed)

Current `plugin/server/dispatch.go` `extractUpdateRouteOutput` copies only
`plugin.RouteResult.Announced` and `Withdrawn`; warning-map responses lose their
reason and become zero-success. Both `opUpdateRoute` and
`handleUpdateRouteSelDirect` use it, and SDK UpdateRoute returns just two counts
and error. `config/redistribute/consumer_factory.go` `RouteUpdater` has that same
signature. Consequently BGPConsumer's `added == 0` warning cannot distinguish
an accepted losing candidate from a failure. Merely adding an internal RIB
receipt would leave this defect intact.

Extend the existing typed command result and `rpc.UpdateRouteOutput` with an
explicit admission result, carried as a value on both transports. It records
per native key and authorized scope whether a candidate was accepted/retained,
whether it is currently selected or suppressed by selection/export, and its
admission generation; refusal/failure carries an explicit reason. Command
aggregation preserves partial outcomes and does not convert a warning map or
missing receipt into accepted admission. The fields are receipt facts, not
client-supplied authority or free-form route metadata.

`extractUpdateRouteOutput` must preserve this value; the registered JSON
`opUpdateRoute` serializes it. Add a result-bearing typed bridge entry/SDK
method returning the same full value. Keep existing count-returning SDK and
bridge methods as compatibility wrappers over it: old clients receive their
existing count/error contract, new consumers can observe admission. Extend
`RouteUpdater`/its registered factory connection with the full-result method;
migrate the SDK implementation, BGP consumer construction and test mocks.
BGPConsumer consumes the full receipt, retaining accepted losers without a
false no-peer warning; admission refusal, missing required receipt and real
transport failures remain explicit errors/owned diagnostics. Do not treat an
arbitrary zero-success from an old server as an accepted candidate.

Counts keep their existing command meaning. Announced is not incremented for
an accepted loser. Withdrawn is not a physical wire count: `cmd/update/update_text.go`
counts `ErrWithdrawWithheld` as completed when that session advertised nothing,
with the explanatory warning. Preserve this compatibility, including partial
commands, instead of redefining both fields as socket writes. Final selected
writer completion remains its separate barrier contract.

For provenance-bearing removal, extend the consumer contribution contract
with an exact source/instance removal value. Migrate `RedistConsumer`, its BGP
implementation, `internal/plugins/ospf/redistribute/consumer.go` and
`internal/plugins/isis/redistribute/consumer.go`, plus factory fixtures/mocks.
OSPF/IS-IS adapters retain their current prefix-withdraw semantics while BGP
uses the exact origin key. Their advertisement policy and returned counts do
not change merely to support the BGP receipt.

The selected envelope distinguishes learned-source receipts from authenticated
local-origin receipts; local output must enter the existing originated build
path, not fabricate a received source to satisfy `resolveRelaySource`. Reuse
per-destination next-hop/AS-path construction and egress. One selected stream
owns replacement across these admitted origin kinds; capture revision under
the same native-key serialization whenever either the learned winner, admitted
scope membership, local candidate or preference generation changes. Receipts
bind the selection scope to the authenticated destination, so scope A's newer
revision cannot retire scope B's output. The existing current-state resource
bound includes current admitted origin records and current scope memberships;
withdrawn never-admitted origins cannot become history-only tombstones.

This admission bridge and redistribution provenance migration are required
parts of the proposal, not claims that a system Loc-RIB subscription already
implements them. No additional owner semantic question is identified; exact
contract names and implementation remain subject to whole-design approval.

The recommendation is a **distinct selected-operation envelope**, containing a
batch of selected changes and reusing the existing retained-route value for an
announcement. It is registered through the existing engine-op/SDK machinery
and converges on shared reconstruction, egress and the final writer. It does
not change the authority of legacy `StoredRoute` callers. The alternative is
an optional authority field on the existing value; although it could preserve
absent-field behavior, it spreads a second authority mode through replay-page,
chunk and direct-reconstruction carriers. A distinct envelope makes granting
selected authority explicit at one authenticated boundary. No method or field
spelling is approved by this proposal.

| Boundary | Concrete proposed behavior | Existing mechanism extended |
|----------|----------------------------|-----------------------------|
| Selection capture | While the winner's retained candidate and selection shard are held, capture its native family/key, owned wire representation and original source/message/policy epoch. Assign a monotonically increasing revision for that key within the deciding process incarnation. Capture absence as a selected tombstone with the same key/revision domain | `checkRouteBestChange`, retained `recoveryCandidate` representation and per-key selection serialization |
| Dispatch ordering | One RIB scheduler records latest selected state and per-destination dirty keys after selection locks release. Each destination has at most one in-flight bounded batch and its own cancellable completion worker; a blocked destination never occupies another destination's sender. Repeated pending changes coalesce to latest revision; in-flight bytes stay immutable through completion | Existing per-session writers and long-lived lifecycle workers; no goroutine per route and no blocking wire/RPC send under election locks |
| Admission identity | Engine derives sender process incarnation and effective role from authenticated transport and destination session. The producer supplies only its selected key/revision and original source provenance. A caller cannot assert another process or override an explicit RS/RR owner | Existing `procSender`, per-peer send grant and session-bound final writer |
| Final output ownership | Existing Adj-RIB-Out keeps the selected authority/revision beside original source ownership. Newer same-stream selection can replace the earlier ordinary winner. Equal/older completed revisions are idempotent/stale. A different or retired process incarnation cannot overwrite the current owner | Extend final-writer authority instead of a second sent table; retain source identity for source-policy and departure retirement |
| Recipient exclusion | Run the selected announcement through existing egress. If excluded, retire this stream's previous selected output even when it came from a different source. Do not retire local/role output merely because it shares the prefix; an admitted local origin participates in this same selected stream, while unrelated role/scope output remains protected | Existing exact-owner removal and selected output receipt; source-bound B withdrawal alone is not sufficient to retire A |
| Final source departure | Non-retained departure produces a selected tombstone/replacement; valid GR/LLGR retention may instead leave the retained source as winner. Its announcement uses a captured source-facts receipt plus the selection owner's live retention authority, not a current Established lookup. Expiry/revocation invalidates that retention before removal/re-election settles | Existing plugin-owned retention and selected retirement; do not route eligible retained winners through `resolveRelaySource`'s Established guard |
| Retry/recovery | Dirty work remains owed until the writer confirms its outcome. Re-read only the current selected record for that native key and admitted scope; never enumerate recipient-filtered candidates. A failure or stale source receipt requests another current-selection reconciliation, not promotion of the next candidate | Existing recovery scheduling and completion; selected-only lookup is an explicit mode/operation, separate from `recoverySurvivorsLocked` |
| Source-policy transaction | Apply/abort uses the same dispatcher barrier: after source eligibility changes and re-election, settle affected selected output before acknowledgment. Old queued work retains its old policy epoch and cannot become current by restamping. An unsuccessful settlement fails the transaction instead of returning applied | Existing `applySourcePolicy`, ledger, exact sent retirements and writer epoch fence |
| Peer snapshot | Capture selected records and a replay cut while registering the destination for subsequent dirty changes. Send snapshot with the destination's current session receipt, drain changes beyond the cut, then report initial ready. Session end cancels that replay; a new session takes a new cut | Existing `SignalsSessionReady`, `FencesLiveForwards`, captured initial-session identity |
| Stop/process replacement | Stop accepting new selection work, cancel/drain dispatcher and outstanding sends, and invalidate the process incarnation's authority. A replacement deciding process starts with a fresh incarnation and must replay before live release | Existing plugin/session lifecycle and writer fences; old queued work cannot inherit new process authority |

Coalescing is bounded by current selected state, actual destination output and
one in-flight batch per destination, not by the history of unique keys seen.
An affected destination for absence is one that has acknowledged an output for
the key or has accepted an in-flight announcement that may still install it.
An announcement not yet admitted to that destination, followed by withdrawal,
coalesces away completely: no tombstone or pending-key record remains there.
Random withdrawn keys that were never sent create no destination state.
Owned route snapshots are reused across destination fanout, and pooled wire
reconstruction stays in the engine. Implementation must measure allocations
and prove release on completion, error and cancellation. A send/RPC acceptance
is not a wire-completion receipt: the selected operation must join the final
writer's completion before clearing dirty work or acknowledging a barrier.
This is why the existing relay's success return alone is insufficient.
Completion must come from the final write operation, not from waiting for a
sent-event callback into the same plugin delivery loop: a source-policy
callback that blocks that loop while waiting for its own sent event deadlocks.
The engine's completion receipt and the plugin's later sent-history projection
therefore have different roles, and both transports must preserve that split.

### Explicit grants and role changes (proposed)

#### Retained sources, progress isolation and public fencing

Source evidence requires a retained-source branch: `rib.go`
`handleState`/`handleStructuredState` keeps routes when `retainedPeers` is set;
`rib_commands.go` `retainRoutes`, `markStaleCommand` and
`purgeStaleCommand` own retention/stale transitions; `bestpath.go`
`compareBeforeMED` de-prefers stale candidates rather than rejecting every one.
Conversely `reactor_api_relay.go` `resolveRelaySource` requires Established and
extracts source AS/class, router ID, group/name, reflector/RS facts, LLGR fact,
receive encoding context and source identity. The selected operation must not
reuse that liveness precondition for a route the plugin validly retains.

The engine captures those immutable source facts in a source-session receipt
while the session exists and supplies that receipt through the existing event
contract. The RIB holds it with the retained candidate, including source path
identity and original policy admission. The authorized selected owner can
extend the receipt's use through its plugin-owned retention decision. Engine
admission checks the issuing source incarnation, selected-owner incarnation,
active retained authorization, source-policy revocation and current destination
receipt; it never computes GR/LLGR duration, chooses a stale path or stores a
second RIB. The owner supplies current stale level through the selected value
so existing egress filters apply the correct LLGR treatment.

Expiry, EOR purge, source configuration removal, policy revocation and owner
shutdown revoke the affected retained authorization before their completion
barrier, then publish the resulting selection/retraction. A reconnect has a new
source receipt; it cannot silently refresh the old route's epoch or source
facts. Until the retained revision is explicitly replaced/purged, a late
destination may receive it with its captured original facts. Final writer
admission rechecks revocation, so an in-flight expired snapshot cannot return
after the barrier. Reuse source receipt/revocation storage and add only missing
facts/lifetime references; no engine GR policy or parallel candidate table.

Per-destination workers are bounded to one in-flight batch each, with the
unsent state coalesced by native key. The scheduler never waits for one
destination's socket/RPC completion before submitting another destination's
batch. Cancellation closes/retire-writes through the existing session path and
joins that destination worker. A transaction barrier waits for the affected
destinations only; a timeout fails the transaction with unresolved destinations
identified, while unrelated destinations continue sending. A plugin stop joins
all its workers before releasing shared snapshots. A bounded batch is also
bounded by encoded IPC bytes, not merely route count.

The pending representation is a desired-versus-acknowledged difference, not an
append-only change log. For each destination its key population is a subset
of the union of the current selected keys, that session's actual known
Adj-RIB-Out keys, and keys in its one admitted in-flight batch. Existing
Adj-RIB-Out remains the output authority; the plugin's completion projection
contains only the identity/revision needed to reconcile it, not a second route
history. A pending absence is retained only while the known output or possible
in-flight install exists. A final removal/suppression receipt deletes that
projection and absence obligation together. If a withdrawal follows an
in-flight announcement, preserve the retirement until that announcement's
receipt resolves; a refused/canceled announcement that installed nothing can
discharge it, while a successful announcement still owes removal.

This gives a structural storage bound of current selected keys plus current
known output keys plus the single batch's keys, with one shared owned snapshot
per needed revision. Global selected tombstones exist only while such actual
destination obligations reference them; never-admitted unique-key churn is
removed from both global and destination pending maps. A blocked recipient
cannot retain every historical withdrawn key. No arbitrary new route-count
limit or configuration option is needed for this bound. The existing IPC
`MaxMessageSize` remains the hard byte ceiling per batch; the selected envelope
uses its actual serialized overhead when chunking. One route that cannot fit
fails explicitly for that destination, leaves its genuine current obligation
owed and reports the error; it does not block event ingestion or silently drop
an acknowledged route's retirement. Destination cancellation retires its
captured session, joins its in-flight write and releases that session's
projection, while healthy destinations continue.

True external fence participation is part of the proposed public declaration:
add the live-forward fence opt-in alongside `SignalsSessionReady` in the
Stage-1 registration value, carry it through internal declarations and genuine
external handshake state, and resolve it by authenticated runtime process
identity. Validate that fencing requires readiness signaling. The current
`registry.FencesLiveForwards` reads compiled registration/aliases only; that
lookup must consume the runtime declaration too, with process removal cleanup.
Both typed and external declarations reach the same session ready/live-release
machinery. Existing external plugins that do not opt in remain non-fencing;
never infer fencing merely because they send routes. Prove a plugin absent
from the compiled registry, not only a forked built-in alias.

Owner decision, 2026-11-16: no binding is derived for ordinary transit. The
operator attaches `bgp-rib` to each peer with an `attach process` block, as
for any process: `receive` on the peers whose routes it elects over, `send` on
the peers it advertises to. Explicit configuration is what prevents surprises,
its friction is accepted, and tooling carries the help. So this spec adds no
default peer-feed declaration and derives no grant.

Extend the existing plugin registration and Stage-1 declaration with opaque
role eligibility only. The plugin also declares, as a pure value, the receive
and send grants its transit job needs, and nothing grants them from it:
the generic check of
`plan/release/fixes/spec-explicit-plugin-and-attach-config.md` reads the
declaration and refuses a commit that grants less.

What asks for transit toward a peer is that peer's `attach process` block
granting the deciding plugin `send`. An export policy alone does not ask for
it: a peer that only receives API announcements has one too, and attaches no
RIB. A capability that is on by default does not ask for it either: AIGP is
enabled on every internal session by RFC 7311 Section 3.3
(`internal/component/bgp/reactor/config_aigp.go` `AIGPEnabled`), and that
default is no operator request for a RIB. So the refusal judges the blocks
the operator wrote for the deciding plugin, and infers nothing from a policy
or a default.

The plugin declares two grant sets, taken from what it subscribes to
(`internal/component/bgp/plugins/rib/rib.go`: sent updates, received updates,
`state` and `refresh`):

| Role of the peer | Grants its block needs | Why |
|------------------|------------------------|-----|
| Source: its routes are elected over | `receive` of received updates and `state` | candidates, and the session's end, which retires them |
| Destination: it is sent the elected routes | `send [ update ]`, and `receive` of `state`, `refresh` and sent updates | `send` alone delivers no lifecycle event: session up starts the selected snapshot and the readiness report, session down retires the destination's output, a route refresh is answered from the selected set, and sent updates feed the sent-history projection |

A block that grants only part of a set (the owner's example, `receive [ state ]`
alone, or `send [ update ]` alone), or a destination block while no peer is a
complete source, is refused at validate, commit and start, with the line that
completes it (AC-25). The sets are judged per configured process: grants given
to two processes that run the same plugin under different names do not
complete each other, because each is its own instance with its own table. A
peer that is a complete destination and has no export policy is a legal
config that withholds learned routes, and it gets the absent-policy report of
AC-21, not a refusal.

The effective destination role is derived from the complete declared set and
the existing delivery graph, not whether an RS process happens to autoload.
Multiple eligible explicit claimants are a configuration conflict, not an
ordering tie-break. Preserve third-party declaration transport; no engine or
config switch names RIB, RS or RR. Claims/UnheldRoles remain discovery and
delivery facts, while the final writer enforces ownership.

For a reload that changes the effective dissemination owner, use the existing
session-restart boundary already used when send/OPEN grants change. Mark such
a role change as requiring the same boundary even if the raw grant lists happen
to remain equal: retire the old session, apply the new effective owner, then
replay before releasing live output. This deliberately avoids inventing an
in-place transfer protocol. Receive-only changes with unchanged owner keep
the existing live-update behavior. The alternative, a live role transfer,
requires revoke/quiesce, exact removal, new-owner replay and a release fence;
it adds complexity without an owner requirement to retain the TCP session.

### Configuration migration disposition

Research search: `bgp-rs|bgp-rr` in `contrib/netlab`, `docs/guide`,
`test/plugin` and `test/interop/scenarios`, over `.ci`, `.conf`, `.j2`, `.md`,
JSON and YAML. The population includes plain-transit, role-specific and
replay-isolation users; the token alone cannot decide their purpose.

| Population | Disposition |
|------------|-------------|
| `contrib/netlab/ze/ze.j2` and `contrib/netlab/golden/r1.conf`, `r2.conf`, `r3.conf` | Remove the plain-transit RS workaround and regenerate the golden configs; keep explicitly configured next-hop/export behavior only where the new correct defaults/policy still require it |
| `docs/guide/netlab.md`, `bgp-peering.md`, `configuration.md`, `ubuntu-build-install.md`, `route-reflection.md`, `plugins.md` | Replace statements requiring RS for ordinary transit; preserve separate RS/reflection examples |
| Role-specific scenarios such as `bgp-route-server-control-community-frr`, `bgp-route-server-replacement-frr`, `bgp-route-reflection-frr`, `bgp-addpath-departure-reflector-frr` | Preserve explicit role and its assertions; ordinary advertisement must yield at the relevant destinations |
| Generic protocol/attribute scenarios and `.ci` users found by that search | Classify by their actual asserted producer during implementation: convert transit-only attachment, retain an RS/relay role when that rail is the subject; record disposition with the changed fixture rather than bulk removal |
| Explicit replay-only fixtures listed in the changed-wire census | Preserve producer isolation. New ordinary output must not satisfy the replay assertion before the explicit replay; adapt setup rather than weaken the useful-output control |

No owner decision is needed to execute this migration rule after whole-design
approval. Its per-fixture proof belongs to implementation because the new
default must exist to observe whether it introduces a second producer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: an UPDATE received from one BGP peer.
- Config: the default configuration, with no route-server plugin loaded or attached.

### Transformation Path
1. Current: receive, ingress admission, cache, permissioned plugin dispatch,
   retained `bgp-rib` selection, best-change publication and system-RIB mirror.
   No ordinary advertiser consumes the selected result.
2. Proposed, unapproved: capture the globally elected retained revision, then
   dispatch its owned bytes and selection receipt outside election locks to
   permitted ordinary destinations through the SDK stored-route rail.
3. Shared egress applies source/destination protocol rules and export policy.
   A recipient with no configured export policy is sent no learned route
   (Q-5); the existing absent-policy statement answers it, not the advertiser.
   The existing final writer admits the selected receipt and original source
   provenance separately; replacement/denial/final absence retire only the
   appropriate earlier output. Ordinary recovery never promotes a lower
   candidate because the global winner is denied to this recipient.
4. Peer establishment obtains a selected snapshot plus ordered changes before
   completing its existing ready fence. Source-policy apply/abort and shutdown
   join or invalidate captured dispatch before acknowledging completion.
5. The scoped-origin proposal supplies admitted origins and configured-preference
   arbitration before export; a denied winner never triggers another election.
   Registered explicit forwarding claims determine where
   RS/RR owns dissemination instead of ordinary output.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → RIB plugin | Existing structured events or external event envelope, over the feeds each peer's `attach process` block grants | Existing transport; the missing-grant report is unimplemented |
| RIB plugin → Engine | SDK stored-route relay, typed bridge or external RPC, extended only after selected-receipt design approval | Current relay exists; proposed selected authority unimplemented |
| Selection → recipient writer | Owned revision plus original source provenance, egress policy, exact selected-output replacement/removal | Research constraint; contract and implementation unapproved |

### Changed-Wire Census

**Search evidence:** On 2026-11-16, searched `pkg` and `internal` Go/YANG for
`StoredRoute`, `RelayStoredRoute` and `relay-stored-route`, including tests;
separately searched repository Python, shell, `.ci` and YANG with `rg -uuu`
for those spellings plus `attr-hex`, `next-hop-hex` and `nlri-framing`, excluding
`vendor`, `.git`, `tmp` and `reference`. A second source search followed JSON
marshal/unmarshal, relay page/chunk and reconstruction calls. Known writer
control: `adj_rib_in/rib.go` `replayStoredRoute`. Known reader control:
`plugin/server/dispatch_cached.go` `opRelayStoredRoute`. Both were found and
read, as were the SDK choice of typed versus JSON rail and the engine-op
registration joining them. SDK writer control:
`pkg/plugin/sdk/sdk_engine.go` `Plugin.RelayStoredRoute`; engine-op reader
control: `internal/component/plugin/server/dispatch_registry.go` relay entry
dispatching `opRelayStoredRoute` and installing its typed handler. This is the
existing-carrier census, not approval
of a new field name or receipt contract. The final design must map its exact
new representation onto these rows before admission.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| StoredRoute Go value | writer | `adj_rib_in/rib.go` `replayStoredRoute`, `buildReplayRoutes`, `relayRoutes`, `relayChunk`; `rib_replay_path.go` `replayPathCommand`, `replayFlowSpecs` | Constructs retained initial updates, validation announcements and withdrawals using `rpc.StoredRoute` | Preserve source/message/policy/framing identity; no implicit selected authority |
| StoredRoute Go value | writer | `rs/server.go` `relayReplayRoutes`; `rs/server_validation.go` stored withdrawal and FlowSpec producers | Hands role-owned values to SDK relay | Preserve configured RS candidate population and validation |
| StoredRoute Go value | writer | `rr/rr.go` `replayForPeer`; `rr/validation.go` `replayValidation`, `replayFlowSpecs` | Hands reflector values to SDK relay/shared replay helper | Preserve reflection and validation/replay semantics |
| StoredRoute Go value | writer | `reactor/policy_reload.go` relay callback and withdrawal producer | Re-enters relay under the existing owner | Ordinary reconciliation must use global winner; preserve legacy role callers |
| StoredRoute Go value | writer | `reactor/aigp_readvertise.go` `replayAIGP` | Constructs value and calls reconstruction directly | Preserve source receipt; account for selected output beyond public-method checks |
| StoredRoute Go value | writer | `reactor/relay_recovery.go` `queueRecoveryOutcome`, `queueRecovery`, `queueRecoveryWithdrawal`; `session_paths_limit_elect.go` `queuePathsLimitElection` | Constructs survivor/withdrawal values for direct reconstruction/queued writer | Preserve role-specific recovery/ADD-PATH; selected output never inherits loser promotion |
| StoredRoute Go value | reader | `pkg/plugin/sdk/sdk_engine.go` `Plugin.RelayStoredRoute`; `pkg/plugin/rpc/bridge.go` `RelayStoredRouteHandler`, setter/call/presence methods | Consumes slice on typed rail or encodes external relay input | Preserve one value contract and transport parity |
| StoredRoute Go value | reader | `plugin/server/dispatch_registry.go` typed relay handler; `dispatch_cached.go` `relayStoredRoute`; `plugin/coordinator.go` `RelayStoredRoute` through `ReactorRelayCoordinator` | Reads destination/routes and forwards to registered reactor; no-reactor failure remains explicit | Preserve authenticated `procSender` and permission checks |
| StoredRoute Go value | reader | `reactor/reactor_api_relay.go` `RelayStoredRoute`, `relayRunLen`, `buildRelayUpdate`, `buildRelayWithdrawal` | Reads identity/framing/attributes and reconstructs pooled received-shape bytes | Preserve legacy reconstruction; selected envelope converges here with explicit authority |
| StoredRoute Go value | writer | Reactor relay/default-policy/permission/recovery tests, coordinator mocks and RS/RR validation fixtures | Constructs typed fixture values | Preserve proof; never infer ordinary authority from legacy fixtures |
| StoredRoute Go value | reader | RS/RR replay/validation bridge-capture tests and coordinator mock relay handlers | Inspects captured values and provenance | Keep expectations and independent test judgment for changed proof |
| External relay JSON | writer | `pkg/plugin/sdk/sdk_engine.go` `Plugin.RelayStoredRoute` → `callEngineWithResult` | Encodes `RelayStoredRouteInput` for `MethodRelayStoredRoute` when typed slot absent | Preserve existing source-bound API |
| External relay JSON | writer | Outside Go/non-Go plugin calling documented `ze-plugin-engine:relay-stored-route` (`docs/plugin-development/protocol.md`) | Public protocol permits third-party request producers; implementations outside checkout were not inspected | Preserve legacy authority; publish deliberate contract changes |
| External relay JSON | reader | `plugin/server/dispatch_cached.go` `opRelayStoredRoute`, registered in `dispatch_registry.go` | Unmarshals `rpc.RelayStoredRouteInput` then invokes shared handler | Preserve exact typed/external behavior and authenticated sender |
| External relay JSON | writer | `plugin/server/dispatch_relay_test.go` request fixtures | Constructs JSON handler inputs | Preserve malformed-input and transport parity controls |
| Replay-page JSON | writer | `adj_rib_in/replay_page.go` `replayRoutesCommand`, `buildReplayPage`; `rib_replay_path.go` `replayPathCommand` | Returns `ribevents.ReplayPage` with embedded StoredRoute values | Preserve bounded pages, cursor and source-path withdrawal semantics |
| Replay-page JSON | reader | `internal/core/bgp/ribevents/replay.go` `readReplayPage`, `RelayReplay`, `RelayReplayPath` | Unmarshals command answer then relays pages under caller identity | Preserve strict cursor progress and onward frame bounds |
| Replay-page JSON | writer | `internal/core/bgp/ribevents/replay_test.go`; RS/RR replay/validation command-answer fixtures | Supplies encoded page responses | Keep page/transport failure controls |
| Replay-page JSON | reader | Outside plugin consuming the documented Adj-RIB-In replay command response | Public command answer embeds the page value; outside implementations were not inspected | Preserve existing response semantics and document any deliberate expansion |
| Source event provenance | writer | `reactor/reactor_notify.go` metadata → `bgp/server/events.go` structured population/external event delivery | Existing source-policy/session identity produced here; captured source facts/retention receipt is proposed extension | Carry explicit receipt on typed/external events and reset pooled `StructuredEvent` fields |
| Source event provenance | reader | `rib/rib_structured.go` and `rib/rib.go` received handlers | Current handlers retain provenance with routes | Proposed capture/retain/revoke receipt follows same lifecycle; test retained route after DOWN |
| Source event provenance | reader | Outside plugin receiving documented BGP events through the plugin process protocol | Event contract is public; outside implementations were not inspected | Preserve existing fields; document the explicit new receipt and its authority limits |
| Stage-1 declarations | writer | SDK registration/handshake using `pkg/plugin/rpc/types.go` declaration value; built-in registrations | Existing readiness/claims declarations; live fence and transit-grant-need fields proposed | Same declaration for internal/forked paths, explicit opt-in |
| Stage-1 declarations | writer | Genuine external third-party plugin handshake, without compiled registry alias | Public Stage-1 writer; existing public shape has readiness only | Proposed fence/feed opt-in supported without compiled registration |
| Stage-1 declarations | reader | Plugin server process registration and runtime declaration lookup; `registry.FencesLiveForwards` | Current fence lookup reads compiled registration/aliases | Extend runtime process lookup/cleanup and ready dependency validation; absence stays non-fencing |
| Selected envelope | writer | Proposed RIB selected scheduler and SDK selected-operation method | New producer specified by capture/revision/retention and per-destination completion design; not implemented | Authenticated owner, original source/local-origin receipt and selected revision remain separate |
| Selected envelope | reader | Proposed registered engineOps handler/typed slot, shared reconstruction and final writer | New consumer specified by role/session/source checks and exact selected retirement; not implemented | Explicit selected authority only; preserve all legacy relay callers |
| BGP UPDATE wire | writer | Existing `reactor/reactor_api_forward.go` `forwardUpdateCore` and `session_write.go` final session writer | Existing egress/wire path; proposal changes selected route population and retirement, not BGP encoding format | Preserve protocol transforms, ownership and final completion |
| BGP UPDATE wire | reader | BGP peers, including pinned FRR/BIRD scenarios and existing reactor socket fixtures | External protocol consumers inspect actual announcements/withdrawals | Prove selected winner/retirement and preserved RS/RR behavior; no new interop run claimed |

Q-3 supplemental search evidence (2026-11-16): `UpdateRoute`,
`UpdateRouteInput`, `UpdateRouteSel`, `UpdateRouteRequest`, `update-route` and
`send bgp.*update` across `pkg`/`internal` Go and YANG, including tests; and
`update-route|send bgp.*update|announce route|neighbor .*announce` across
`test`/`contrib` Python, shell and `.ci` using `rg -uuu`. Known writer control
is SDK `UpdateRoute`; known reader controls are server `opUpdateRoute` and
`announceTarget.initBatchOwner`. The search found actual Python producers
for this origin contract, unlike the narrower legacy StoredRoute search.
The following rows extend the census; external clients remain uninspected.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Origin command request | writer | SDK `sdk_engine.go` UpdateRoute family; `rpc/bridge.go` UpdateRouteSel; outside SDK/non-Go callers | Selector/command/meta/source-policy epoch, no origin preference today | Preserve request grammar, authenticated identity and selected peers; new internal admission receipt must not grant clients another source's authority |
| Origin command request | reader | Server `dispatch_registry.go` opUpdateRoute, `dispatch.go` handleUpdateRouteSelDirect, command dispatch; BGP cmd update_text/update_wire → reactor batch path | Typed and JSON converge on command dispatch and originated output | Intercept actual new local origins for registered admission at ordinary destinations; preserve received maintenance/replay classification, errors and cancellation |
| Origin command request | writer | `bgp/config/bgp_routes.go`, `bgp/redistribute/consumer.go`; watchdog command producers, `ls_export/export_state.go`, `ddos/flowspec/responder.go` | Existing configured, redistributed and plugin-owned origin producers reach send/update command paths | Preserve configured restoration, watchdog and producer lifetime; redistribution migrates provenance before text conversion. Non-CIDR keys keep native identity, no IP-only system-RIB conversion |
| Origin command request | writer | `internal/exabgp/bridge/bridge_command.go`, `bridge_route_forms.go`, `bridge_flow.go`; native Go fixture/interop producers and command tests | Bridge writes selected `send bgp ... update` forms and distinguishes EOR/refresh from routes | Preserve selector/withdraw grammar and non-route controls; test actual useful output and admitted-loser result semantics |
| Origin command request | reader | `rpc.UpdateRouteOutput` consumers in SDK, BGP redistribution consumer, native fixtures and outside clients | SDK returns Announced/Withdrawn; consumer currently treats zero as no peer announcement | Preserve legacy command-count meaning, including completed withheld withdrawal; explicit admission result distinguishes retained loser, never inflate Announced |
| ExaBGP command text | writer | `test/exabgp-compat/etc/run/exabgp_api.py`, `old-5.0/etc/run/exabgp_api.py`, operator scripts outside checkout; `.ci` API cases under test/exabgp-compat and native send cases under test/encode and test/plugin | Python/embedded command producers exercise announce/withdraw and targeted neighbors | Preserve syntax, process matching, acknowledgments and keep/withdraw-on-exit behavior; extend mixed-origin positive and scoped-survivor cases |
| ExaBGP command text | reader | `internal/exabgp/bridge/bridge_command.go` conversion and bridgerun dispatcher | Converts peer-scoped origin commands into native command requests | No scope broadening or local-origin inference from received replay; control messages remain controls |
| Redistribution origin contribution | writer | `internal/plugins/redistribute/redistribute.go` dispatchEntryToConsumer, through config `RouteEntry`/`RouteUpdater` adapter | Accepted Source/Route/Peer exist before text formatting; current WithdrawRoute loses source identity | Extend the typed consumer/updater contract deliberately to retain source/instance, override and exact removal; migrate all implementers/mocks and preserve non-BGP consumer semantics |
| Redistribution origin contribution | reader | `internal/component/config/redistribute/consumer.go` RedistConsumer and consumer factory; BGP `redistribute/consumer.go` InjectRoute/WithdrawRoute; other registered consumers | BGP currently ignores Source/Route and calls selector-plus-text UpdateRoute | BGP consumes provenance through new origin-intake; `internal/plugins/ospf/redistribute/consumer.go` and `internal/plugins/isis/redistribute/consumer.go` preserve current behavior via explicit adapters, not a silent signature change |
| Admitted origin request | writer | Proposed authenticated reactor origin intake and BGP redistribution adapter, through generic registered typed/external plugin operation | New self-contained native key, origin kind/provenance, concrete target scope, preference inputs and generation | Bind original authority before plugin call; redistribution source/override and exact removal must survive; never serialize process-local pointers |
| Admitted origin request | reader | Proposed `rib/rib_origins.go` registered handler, internal/forked with same value contract | Extends deciding plugin candidate storage and scoped selection | Compare only admitted origins and common learned winner; source-owned withdrawal, snapshot/reload generations and no recipient loser search |
| Origin command result | writer | `plugin/types_bgp.go` RouteResult, BGP cmd/update result aggregation; `plugin/server/dispatch.go` extractUpdateRouteOutput | Current extraction copies only two counts and drops warning-map distinction | Extend typed result and extraction with explicit admission outcome; preserve legacy counts, partial outcomes and real errors |
| Origin command result | writer | `plugin/server/dispatch_registry.go` opUpdateRoute; `dispatch.go` handleUpdateRouteSelDirect | Both currently call count-only extraction; direct handler returns two counts | JSON result and new typed result bridge carry identical admission receipt; old typed method remains compatibility wrapper |
| Origin command result | reader | `pkg/plugin/sdk/sdk_engine.go` UpdateRouteRequest/UpdateRouteSel and new full-result method; `pkg/plugin/rpc/bridge.go` typed entry | Existing SDK discards everything except Announced/Withdrawn | Add result-bearing API/bridge entry and retain count wrappers for outside callers; malformed/missing receipt never accepted implicitly |
| Origin command result | reader | `config/redistribute/consumer_factory.go` RouteUpdater/ConsumerFactory, `bgp/redistribute/consumer.go` InjectRoute, associated factories/mocks | Two-count interface feeds zero-count warning today | Extend connection method and migrate implementation/mocks; BGP consumes explicit admitted-loser result without false warning; actual failure remains reported |
| Admitted origin receipt | writer | Proposed deciding plugin registered handler plus selected completion | Separates candidate acceptance from resulting final output | Bound to original request/scope and current generation; explicit failure and cancellation, no fabricated wire count |
| Admitted origin receipt | reader | Proposed reactor origin intake/SDK response adapter and redistribution adapter | Joins candidate admission with actual output result | Preserve UpdateRoute response meaning; distinguish admitted loser from transport/admission failure |

Status and dry-run search evidence (2026-11-16): `PolicyInfo`,
`toPolicyChain`, `policyStatus`, `PolicyDryRun`, `PolicyDryRunResult` and
`PolicyTraceEntry` across `internal`, `pkg` and `cmd` Go, tests included; the
JSON keys `import-policy`, `export-policy`, `final` and `final-source`, and
the command words `show policy chain` and `show policy test`, across Go,
YANG, `.templ`, JavaScript, Python, shell, `.ci` and `.et` under `internal`,
`cmd`, `pkg`, `test` and `contrib`. Known writer control: `reactor_api.go`
`policyInfo`. Known reader control: the policy handler's `toPolicyChain`. Both
were found and read. No web template, looking-glass page, MCP adapter,
Python or shell file reads these values by name; the web, REST, gRPC and MCP
surfaces reach them only through the two registered commands and the peer
detail command, so their rows are the commands' rows.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Policy status value | writer | `internal/component/bgp/reactor/reactor_api.go` `policyInfo`, `absentPolicyInfo`, called by `Peer.policyStatus` and placed in `plugin.PeerInfo.ImportPolicy` and `ExportPolicy` where the peer list is built | An absent policy on a known internal session answers nil, or filters with no final; an external one answers reject with source `rfc8212` | Change: an absent export policy on an internal destination ordinary output owns answers reject for learned routes with a source of its own, and says local routes are sent. Import, external sessions and role-served destinations preserved |
| Policy status value | reader | `internal/component/bgp/plugins/cmd/peer/peer.go`, the peer detail row keys `import-policy` and `export-policy` | Emits the value as it is, and omits the key when the value is nil | Change: the export key now appears for such an internal peer, carrying the new final and source. No field is dropped from the value on the way |
| Policy status value | reader | `internal/component/bgp/plugins/cmd/policy/handler.go` `toPolicyChain`, called by `handleShowPolicyChain` for each direction | Copies `Filters`, `Final` and `FinalSource` into `policyChain`, field by field: "so the chain view and peer detail never disagree" | Change: carry every field the value gains, the statement about local routes included, so `show policy chain` and peer detail keep agreeing. A field added to the value and not to this copy would be dropped in silence |
| Policy status value | reader | `plugin.PolicyDryRunResult.Policy`, filled in `internal/component/bgp/reactor/policy_dryrun.go` | The dry run reports the direction's effective policy in the same form | Change: same new final and source for the same destination |
| Policy status value | reader | `internal/test/fixture/rfc8212_policy_status.go` `rfc8212PolicyProjection`, driven by `test/plugin/rfc8212-policy-status.ci` | Decodes `filters`, `final` and `final-source` from peer detail, `show policy chain` and `show policy test` for external and unresolved peers | Preserve its external expectations; extend with the internal absent-export case and the role-served case. A change to what it asserts is judged independently |
| Policy status value | reader | Outside clients of peer detail and `show policy chain` over REST, gRPC, MCP, SSH and the web | Public command answers; outside implementations were not inspected | A new source value and a new statement are additions to a documented answer shape; the pages that document it are updated in the same work |
| Dry-run request | writer | An operator or client issuing `show policy test peer <selector> <direction> ...`; the functional tests `test/plugin/policy-test-*.ci` | Tokens selector, direction, filter, update and source-asn4, declared in `internal/component/bgp/plugins/cmd/policy/yang/ze-policy-cmd.yang` under `container test` | Change: a declared keyword for the simulated kind, `learned` or `local`. Existing requests carry none and keep working, answered as `learned` on export |
| Dry-run request | reader | The policy handler's `parsePolicyTestArgs` and the handler that calls `plugin.PolicyDryRunner.PolicyDryRun` | Parses the tokens and passes peer, direction, filter override, payload and asn4 | Change: parse the kind, refuse an unknown value with the two valid ones, refuse it with the import direction, and pass it through |
| Dry-run request | reader | `internal/component/bgp/reactor/policy_dryrun.go` `PolicyDryRun` on the reactor adapter | Refuses an unknown direction, since the interface is exported, then runs the chain and applies `AcceptsAtEnd` | Change: take the kind, refuse an invalid one here as well, and read the absent-policy statement with it. The kind selects the answer shown and authorizes nothing |
| Dry-run result | writer | `policy_dryrun.go`, the two sites that build `plugin.PolicyDryRunResult` | Direction, peer, action, reason, policy, trace, text before and after, changed attributes and wire changes | Change: add the simulated kind, and state for a role-served destination that the role's default applies |
| Dry-run result | reader | The command's renderer for every surface, the functional tests above, and outside clients | The result is the command's data, rendered through the shared pipes | Additive field, documented in `docs/guide/bgp-policy.md`, "Verify policy" |

Supporting evidence that is not a wire speaker stays outside the table:

- `pkg/plugin/rpc/types.go` declares `StoredRoute`, `RelayStoredRouteInput`
  and `NLRIFraming`: original source epoch/message, withdrawal, initial update
  and framing stay explicit. The recommended selected envelope reuses values
  without silently reclassifying legacy callers.
- `adj_rib_in/relay_chunk.go` `relayRouteJSONMax`/`relayChunkEnd` and
  `replay_page.go` budget encoded bytes. Preserve
  `TestRelayRouteJSONMaxBoundsMarshal` and page/chunk boundary tests; derive
  selected-envelope overhead for its own batches rather than assume a route
  count bounds IPC size.
- All repository `.yang` search found no literal legacy relay RPC node.
  Completed engineOps trace below proves runtime registration. Legacy schema
  completeness remains unverified, not an unwired-runtime finding; the proposed
  selected operation gets explicit engine YANG and engineOps registration.
- Python/shell search found no matching direct encoder/decoder in the searched
  repository population, including ignored files outside the stated exclusions.
  This is a negative search result, not an invented speaker or proof about
  third-party source outside the checkout.
- Six `.ci` references concern replay behavior: `adj-rib-in-replay-on-peerup`,
  `adj-rib-in-replay-addpath-source`, `bgp-rs-relay-aspath-transparency`, its
  `-forked` sibling, `med-not-propagated-across-as`, and
  `local-pref-strip-ebgp`. Inspected launchers use `le test fixture`/`ze plugin`,
  not embedded StoredRoute JSON codecs. Preserve their producer isolation so
  new ordinary output cannot satisfy an explicit replay assertion prematurely.

Related maintenance `40e82b0f8d` corrected the JSON-overhead test's constant
and passed its canonical scoped check in 2.0 seconds, as reported by the main
thread. It was committed with this spec's work attribution. It is CI
maintenance, not an advertiser implementation or evidence that the seven
selected-route probes now pass.

The recommended wire contract is the distinct selected envelope/registered
operation using shared egress and final writer. Extending the existing value
with optional authority remains the documented, unrecommended alternative. An optional field must preserve legacy classification when
absent; a separate operation must not duplicate forwarding logic. Neither
requires rewriting every legacy producer automatically. Rows marked preserve
remain unchanged when transport/ownership tests prove their semantics; migrate
only the carriers the agreed contract actually changes. The complete census
exists to make that decision observable, not to prescribe a wider diff.

The runtime declaration trace is concrete: `dispatch_registry.go` builds
`engineOpTable` from `engineOps`; `lookupEngineOp` selects by the RPC method,
and `serveEngineOpJSONContext`/`serveEngineOpDirect` invoke that entry's handler.
The relay entry supplies both `opRelayStoredRoute` and the typed bridge setter.
Thus the relay runtime is registered and does not need a command-YANG lookup
to reach its handler. `internal/core/ipc/yang/register.go` separately embeds
and registers the engine YANG module. The absent literal schema node is not an
unwired-runtime finding; schema-contract completeness remains an explicit
design check before extending this public operation.

### Integration Points
- RIB retained selection and retention commands feed the selected scheduler.
- Registered selected operation authenticates role/source receipts and enters
  shared egress/final-writer completion, with isolated destination progress.
- Explicit `attach process` grants, the declared transit needs behind the
  missing-grant report, and runtime claim/fence declarations use existing registration,
  delivery graph, alias resolution and session-restart lifecycle.
- Source-policy, GR/LLGR expiry/purge and shutdown join affected output barriers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Proposed; unimplemented | RIB selects; registered operation reaches existing reconstruction, egress and final writer; final completion is explicit |
| No unintended coupling (components stay isolated) | Proposed; unimplemented | Self-contained snapshot/source receipt and typed/external parity; no sibling-plugin import, no engine GR policy |
| No duplicated functionality (extends existing, does not recreate) | Proposed; unimplemented | Existing selection, claims, sent ownership, revocation, session fencing and per-session writers are extended; no parallel RIB/claim registry |
| Zero-copy preserved where applicable (refs, not copies) | Proposed; measurement owed | Own one retained selected snapshot across fanout; pool reconstruction remains shared; bounded batches and release-on-error/cancel require allocation/lifetime tests |
| Registration over hardcoding, outbound | Proposed; unimplemented | Selected engine operation and the transit grant needs declared by owner, read only to report a missing grant; authenticated runtime role lookup |
| Registration over hardcoding, inbound | Proposed; unimplemented | Existing delivery graph/Claims plus public fence declaration; genuine external registration tested without compiled alias |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No forwarder consumes the `bgp-rib` best-change batch | Current producers: `rib_bestchange.go::publishBestChanges` emits the typed event and redistribution. Direct subscribers are `flowexport/enrichbgp.go::bgpEnrichBuilder.Start` (enrichment table) and `bmp/bmp_locrib.go::BMPPlugin.startLocRIB` (BMP monitoring); neither sends ordinary peer UPDATEs | an existing consumer already advertises and the defect is elsewhere | Source re-read on 2026-11-16: both subscriber handlers and the publication function; RS/RR have no best-change subscription | confirmed |
| A-2 | The netlab failure is the default behaviour, not a render mistake | commit `ef1a3cf33d`: pings failed with `bgp-rib` alone and passed with `bgp-rs` attached | the spec fixes the wrong layer | Main-thread corrected functional run, terminal 4037 exit 1: no-RS forked RIB reaches real API output, next-hop resolution and selected-owner controls, then times out at `elected iBGP route never reached external destination`; matches `publishBestChanges` lacking ordinary send | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two advertisers send the same prefix to the same peer (the default and an attached `bgp-rs`, or the default and redistribution) | flapping or duplicate announcements in a `.ci` with both | Resolved Q-3 policy plus proposed scope arbitration and Q-2 role resolution yield one ordinary selected stream per destination key; prove cross-origin replacement before approval |
| R-2 | The default advertises routes an operator never meant to export, toward an external or an internal peer | learned routes seen on a peer with no export policy | Q-5: no learned route reaches a peer without a configured export policy; RFC 8212 Section 3 still governs every route on an external session |
| R-14 | An operator expects transit and gets none, because the internal recipient has no export policy | a peer is Established and holds no learned route while the Loc-RIB has winners | The absent-policy report names the peer, what is withheld and the editor command that fixes it, at `ze config validate`, in the Established log line and in `ze doctor` (AC-21) |
| R-15 | The export default changes for a writer the decision did not mean: a reflector client or an API announcement toward an internal peer stops being sent | reflector and API `.ci` reds on internal sessions with no export policy | The Q-5 reach the owner decided leaves local origins and explicit roles on today's answer; AC-20 names them as survivors |
| R-3 | Route server and reflector regressions | `bgp-rs-*` and reflector `.ci` and interop reds | preserve both roles and run their suites |
| R-4 | Recipient recovery promotes a lower candidate after the global winner is denied | Existing export-denial probe advertises the old loser again | Selected-only recovery contract; retain the current role-specific candidate ladder only for its existing callers |
| R-5 | New winner B is denied but old advertised winner A remains | Recipient still holds A after a positive initial advertisement and later denied election | Selected stream retires its previous output; source-bound B withdrawal alone is insufficient |
| R-6 | Old selection dispatch overtakes a newer selection or policy apply/abort | Stale source attributes return after a joined replacement or policy boundary | Ordered receipts, original source-policy provenance and completion/drain under the existing writer/session fences |
| R-7 | Source-down removal tries to relay through a source that is no longer established | Final withdrawal is refused at `RelayStoredRoute` source resolution | Use exact-owner retirement; prove replacement survivor and final absence through recipient wire |
| R-8 | A new recipient receives no selected routes, or a reconnect duplicates history | First-session snapshot absent or duplicate advertisements around ready | Selected snapshot plus ordered deltas; preserve session receipt and existing local-origin history |
| R-9 | Daemon-wide claim suppresses ordinary output to unrelated destinations | Ordinary peer loses useful advertisement merely because another peer attaches RS/RR | Extend existing claims with tested destination scope and reload ownership transfer |
| R-10 | Allocation/dispatch failure silently loses a selected change | Sender reports completion but recipient keeps stale output | Define bounded batching, ownership and retry/failure reporting before approval; no silent queue drop or acknowledgment on incomplete drain |
| R-11 | Eligible retained GR/LLGR winner is suppressed because its source is down | Late recipient receives no valid retained winner | Plugin-owned retention authority plus captured source facts; expiry/revocation rechecked at writer and completion barrier |
| R-12 | Slow recipient stalls unrelated output | Held socket A prevents useful output at B | One bounded outstanding batch per destination, independent completion workers and cancellation; scheduler never waits for A to submit B |
| R-13 | Genuine external advertiser declares ready but cannot raise a live fence | External non-registry process sends live delta before replay | Public Stage-1 fence declaration and runtime process lookup parity; absence stays non-fencing |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes leaked to or withheld from peers on every Ze node with more than one BGP peer |
| How is it reverted? | If implementation must be rolled back, stop the affected sessions, deploy the last known-good executable/config pair and restart with its documented explicit-transit setup. Retire the selected-owner incarnation so no queued new-format work crosses that boundary. No runtime toggle or destructive git operation is proposed; repository undo remains owner-controlled under git safety rules |
| Who else touches this path? | `spec-bgp-next-hop-auto-rewrites-for-ebgp.md`; route server and reflector work under `internal/component/bgp/plugins/rs` and `rr`; the reactor forward path |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from iBGP peer A, `bgp-rib` attached to A and B, eBGP peer B with an export policy | → | Explicit RIB grants → retained global selection → proposed selected operation → shared egress/final writer | `test/plugin/bgp-best-route-transit-default.ci` (planned name) |
| UPDATE from eBGP peer A, iBGP peer C with no export policy, then the policy configured | → | Absent-policy end disposition → selected output withheld, then delivered; absent-policy report → validate and doctor | `test/plugin/bgp-best-route-needs-export-policy.ci` (planned name) |

### Phase 1 implementation audit, 2026-11-16

This audit judges the current ordinary-output capability, not the completeness
of the existing role-specific implementations. No acceptance criterion is yet
established as complete. Existing mechanisms make each group partial; the
ordinary selected-output integration and its required proofs remain missing.

| Acceptance criteria | Current producer and reusable behavior | Missing work |
|---------------------|-----------------------------------------|--------------|
| AC-1 to AC-4 | `rib_bestchange.go::publishBestChanges` publishes the selected winner; `rib_recovery.go::recoveryCandidate` captures retained bytes and provenance | Ordinary output, replacement/final retirement and the ordinary split-horizon control; seven existing probes are red before useful output |
| AC-5, AC-24 to AC-26 | `registry.go::ClaimsFor`, `registry.go::FencesLiveForwards` and `reactor_api_relay.go::RelayStoredRoute` provide existing role metadata, replay fencing and send permission checks | Destination-eligible role resolution, same-instance complete grant validation, ordinary scope isolation and the API-only/no-RIB configuration proof |
| AC-6, AC-10, AC-11, AC-13, AC-18 | `reactor_api_relay.go::RelayStoredRoute` is source-bound; `rpc/types.go::StoredRoute` preserves source epoch, message ID and NLRI framing | Distinct selected authority, exact retirement, destination identity, selected-only recovery and lifecycle completion barriers; retain existing relay semantics |
| AC-7, AC-8 | `rib_recovery.go::recoveryCandidate` owns the selected route's bytes; `rib_replay.go::collectPeerUpReplay` replays sent history on returning sessions only | Selected initial snapshot, eligible retained-source authority and queued-output revocation |
| AC-9, AC-12, AC-14 | `rib_bestchange.go::publishBestChanges` emits after election locks, but has no ordinary destination dispatch | Bounded independent destination workers, churn cleanup and actionable reconciliation failure/clear proofs |
| AC-15 to AC-17, AC-19 | Existing `rpc/types.go::StoredRoute` and source-bound relay are carriers, not cross-origin preference or admission receipts | Scoped origin intake, configured preference arbitration, admitted redistribution and accepted-loser result contract |
| AC-20 to AC-23 | `filterapi/policy.go::Policy.AcceptsAtEnd` and `UnconfiguredAccepts` implement the existing external-only absent-policy default | Engine-derived per-route kind, learned-only internal refusal/reload, role exemptions, validate/log/status/dry-run/doctor projections and their survivor proofs |

Planned-file audit: the existing relay, RIB election/recovery/replay, policy and
registration partitions remain the extension points. The selected-operation,
origin-admission and ordinary-dispatch additions are missing. The two named
functional tests now have ignored drafts under `test/draft/plugin/`, driven by
`internal/test/fixture/bgp_best_route_transit.go`; no live `.ci` is promoted.
Both configurations have explicit source received-update/state grants and
destination send/state/refresh/sent-update grants on the same `rib` instance,
with no RS/RR attachment. AIGP and Graceful Restart are explicitly disabled to
isolate ordinary transit. Each draft covers internal and forked RIB placements.

Test-plan audit: the seven named reactor probes already exist and remain
untouched. The additional proposed unit regressions and both FRR/BIRD scenarios
remain missing. The first draft proves the wiring for useful elected output
and final withdrawal beside local API output. The second proves an internal
API survivor, learned withholding, and policy reload releasing retained output.
Its validate/doctor/role-exemption assertions remain owed in subsequent phases;
the draft does not claim AC-21 or AC-22. The main thread's corrected default
transit run reached feature RED at missing selected output (terminal 4037,
forked placement), confirming A-2. Its internal-placement lines were elided,
so this evidence alone does not prove both placements reached that assertion.
The policy draft separately reached its intended internal-placement feature
RED after API, election, withholding and applied-reload controls; the forked
placement's active-file correction still owes a rerun.
The policy draft's absent-output fence requires the new selected dispatcher to
join the existing `request quiesce` barrier, including scheduled/in-flight
selection work. An elected-owner query alone proves selection, not completion
of downstream dispatch. Without that join, an old forbidden announcement could
arrive after the API fence and masquerade as output released by the reload.
The authority/dispatch phases must provide this join before the draft's
negative assertion is credited; a timing delay cannot replace it.

Documentation read for this phase: `docs/architecture/rib-transition.md`,
`docs/guide/route-reflection.md`, `docs/functional-tests.md`,
`docs/architecture/testing/ci-format.md` and
`docs/contributing/running-commands.md`. This phase adds test drivers only;
it changes no product behavior or operator documentation claim.

First-run diagnosis (main thread, 2026-11-16): the fresh named runner rejected
configuration before socket controls ran: `AIGP selection requires process
"rib" to receive update-received and state from every BGP peer`. The producer
is `reactor/config_aigp.go::PeerSettings.AIGPEnabled`, which enables AIGP on
internal sessions by default, followed by
`config/redistribute_binding.go::wireRedistributeDelivery` and
`config/flowspec_binding.go::requireRIBDelivery`, which require source grants
on every peer. This is a fixture-input concern for the wiring question.
`[workaround]` granting received updates to the destination would conceal the
exact source/destination boundary the approved spec requires. `[source]` the
drafts explicitly disable unrelated AIGP selection on all peers, preserving
the intended grants. The separate AC-26 default-AIGP/no-RIB proof remains owed;
this correction claims no fix to that behavior. The corrected runs must reach
election and useful-output controls before counting as wiring RED evidence.

Second setup diagnosis (main thread, 2026-11-16): the policy test's internal
placement reached feature RED at `policy reload did not release retained
elected route`, but its forked placement failed at `open ze-bgp.conf: no such
file or directory`. `runner/runner_config.go::zeConfigFileName` gives the first
daemon `ze-bgp.conf` and the next distinct stdin block its own cwd and named
file, here `daemon-2/ze-config-forked.conf`. The fixture incorrectly read the
single-daemon constant; its reused `liveStagesReload` would also write that
wrong name. The owning layer is this new test driver. `[workaround]` creating
a `ze-bgp.conf` copy or guessing by placement could make the read succeed while
reloading unchanged active configuration. `[source]` the policy `.ci` now
passes each daemon's explicit active filename; `bgpBestRouteAddPolicy` reads
it and `bgpBestRouteReload` writes that same path and requests the canonical
reload command, then requires the actual daemon's reload generation to advance
and report `applied`. No runner or product code changes, aliases or source-route
reannouncements are involved.

Complete reload-path diagnosis after that correction: the internal placement
again reached feature RED, while the forked observer reported `daemon.pid not
found` (main run 82727, `job-advertiser-wiring-policy-red-authorized-19d94541.log`).
`runner_exec.go` creates each daemon's config directory but publishes
`daemon.pid` at the test's `Record.WorkDir`. The hub's
`declared_plugins.go::pluginServerConfigs` supplies `loadResult.ConfigDir` as
the external plugin WorkDir, and `process/process.go` applies it to the child.
Consequently the second observer runs beside its active config and not beside
the runner-owned PID file. The daemon's `hub/pidfile.go::writePIDFile` creates
no separate file when `ze.pid.file` is unset. The owning defect is the new
fixture's assumption that active config and runner PID share a directory.
`[workaround]` guessing a parent PID path or manufacturing a local alias binds
the fixture to runner bookkeeping. `[source]` the fixture now uses the existing
`request reload` command on its authenticated daemon connection, removing its
PID dependency entirely. `server/system.go::handleDaemonReload` calls the hub's
`main_reload.go::newRequestReloadFunc`, which runs `signalReload`, the same
source-file reload implementation SIGHUP uses. The fixture still checks reload
generation advancement and `applied` before waiting for learned output. This
correction awaits the main thread's fresh policy rerun; it establishes no new
product behavior or successful advertisement claim.

## Acceptance Criteria

The initial ruling/RFC criteria and the proposed lifecycle, transport and
resource-bound criteria below form one unapproved design for whole review.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | No route-server plugin configured, and `bgp-rib` attached to A and B with the grants transit needs (this holds for every criterion below that expects transit); peer A (iBGP) announces a prefix, peer B is eBGP with an export policy that accepts it | B receives the prefix with the path Ze elected | none: the ruling makes this the default |
| AC-2 | The same, with A eBGP under an import policy that accepts the prefix, and B iBGP with an export policy that accepts it | B receives the elected route | none: both policies are configured and accept |
| AC-3 | A and B both iBGP, no reflection configured | the route stays in Ze's Loc-RIB, and B is sent only what RFC 4271 Section 9.2 allows | A's route is not sent to B: Section 9.2 forbids re-distribution to other internal peers |
| AC-4 | The elected route is withdrawn and no replacement exists | every peer that was sent it receives a withdrawal | none: RFC 4271 Section 9.2, RFC4271-9.2-7 |
| AC-5 | A destination grants an explicit eligible RS or RR process its required receive/state/send permissions, beside an ordinary destination | Explicit role owns dissemination there; ordinary output yields at that destination and continues useful output at the ordinary destination. Two eligible explicit claimants are refused during configuration verification with both processes and peer named | Configured role output and ordinary output at unrelated destinations survive; no duplicate ordinary owner at the role destination |
| AC-6 | Advertised winner A is replaced by globally selected B which recipient policy excludes | Recipient receives withdrawal of A and no lower candidate is promoted | Unrelated prefixes and other recipients accepting B retain useful output |
| AC-7 | Source is down but its globally selected route remains eligible under plugin-owned GR/LLGR retention; a new recipient establishes | New recipient receives the retained winner using captured source facts and correct stale treatment | Retained path survives until plugin expiry/purge/revocation; fresher eligible winners may replace it |
| AC-8 | A retained selected output is queued when its retention expires or policy is revoked | Removal/replacement completes before acknowledgment and delayed old output cannot restore it | Unaffected source/route output remains; applicable fresh replacement survives |
| AC-9 | Destination A blocks final flush while B accepts output | B receives selected updates without waiting for A; canceling A joins its bounded worker | B and other destinations retain their independent useful routes |
| AC-10 | Genuine external advertiser absent from compiled registry declares ready and live-forward fence | Initial selected replay precedes its live delta using runtime declaration on both transports | External plugins without the opt-in keep their existing non-fencing behavior |
| AC-11 | Source path ID zero, a replacement source path, and independent opaque native keys | Correct replacement and old destination path retirement with exact source/native identities | Different opaque key and unrelated selected routes survive; zero is never treated as missing |
| AC-12 | A blocks one accepted batch while many unique routes are announced then withdrawn before A admits them; B remains writable | Pending output keys stay within current selected keys, known Adj-RIB-Out keys and one in-flight batch; candidate storage additionally contains only current admitted origins and current scope memberships. Never-admitted churn leaves no tombstones. B receives useful current routes, and release/cancellation of A joins and releases its session state | A's genuine applied/in-flight obligations remain until resolved; B's current routes survive; no history-only obligations remain |
| AC-13 | Source-policy apply/abort, retention expiry or shutdown reaches its completion barrier while selected work is queued | Acknowledgment follows the final writer outcome for every affected destination, or fails naming unresolved destinations; stale work cannot reappear afterward | Unaffected routes/destinations continue; valid restored abort state follows the acknowledged policy, not an old receipt |
| AC-14 | Selected dispatch fails from permission, source-receipt or transport/resource failure after useful output | Genuine obligation stays owed and an actionable owned diagnostic identifies destination/cause/fix; successful reconciliation clears it | Healthy destinations and previously valid unrelated output survive; intentional export-policy exclusion is a completed removal, not an error diagnostic |
| AC-15 | Explicit A-only local origin and common learned winner exist for the same key, with accepting A/B recipients | A uses its admitted scoped winner; B receives the common learned winner, never the A-only local route. Denying A's elected route withdraws A's old output without loser promotion | B's useful learned route survives because A's origin was never admitted to B; unrelated scope outputs survive |
| AC-16 | Local origin is initially preferred, then configured distance makes the learned winner preferable; withdraw the losing local origin | Learned replacement reaches the wire while local is still valid; losing-origin withdrawal does not remove it | Other admitted origins remain candidates for genuine later reselection; unrelated prefixes survive |
| AC-17 | Configured redistribution admits a source route beside learned BGP, while another system route lacks admission | Existing Ze distance/override determines useful selected output; rule removal retires only that source and exposes the admitted replacement; no unadmitted system route is exported | Learned replacement and unrelated admitted routes survive; a system route without redistribution remains forwarding-only |
| AC-18 | Preference apply/rollback or recipient reconnect overlaps scoped origin replacement | Replay and final completion reflect one admitted scope/preference generation; delayed old local history cannot overwrite the selected winner | Other scoped candidates and healthy recipients retain their authorized useful output |
| AC-19 | An API/redistribution origin is admitted but loses to a useful selected route on both typed and external transports; later remove the winner, and separately inject a true failure | Initial receipt explicitly says retained/suppressed with Announced zero and no false failure/no-peer warning; winner removal advertises the retained origin. True admission/transport failure still reports its cause; completed withheld withdrawal retains its legacy count | Current winner survives loser admission; the retained loser survives until genuine re-election; unrelated origins remain unchanged |
| AC-20 | Default config, no route-server plugin; A is eBGP under an accepting import policy, B is iBGP with an accepting export policy, C is iBGP with no export policy. A announces a prefix; an API announcement is also sent toward C. Then B's export policy is removed | B receives A's route and C never does. C still receives the API announcement. Removing B's export policy withdraws A's route from B | The API announcement toward C survives: the requirement binds learned routes carried by ordinary selected output (Q-5). A's route stays selected in the Loc-RIB. Role-served destinations are AC-22 |
| AC-21 | The same config, with C Established | `ze config validate`, the Established log line and `ze doctor` each name C, say that it has no export policy so Ze passes it no route learned from other peers, and give the editor command that configures one. The report clears once C has an export policy | An external peer keeps its RFC 8212 wording; a peer with a configured export policy raises nothing, whatever that policy rejects |
| AC-22 | The AC-20 config plus D, an internal client served by an attached `bgp-rr`, E, an external client served by an attached `bgp-rs`, both with no export policy, and an internal source S announcing a distinct prefix | D receives A's external-learned route without reflection attributes and receives S's internal-learned route with its RFC 4456 attributes. E receives nothing, because RFC 8212 Section 3 binds every route on an external session. `ze config validate`, the log and `ze doctor` raise the internal absence for C and not for D, and the RFC 8212 absence for E | D's useful role output survives for both sources. C stays without either route, so the exemption is tied to the attached role; an external-learned route is not falsely treated as iBGP reflection |
| AC-23 | B is internal with an accepting export policy and holds a learned route and an API announcement. Its export policy is removed, restored, removed in an apply that is then rolled back, and a learned-route write queued before a removal is released after it | Removal withdraws the learned route only. Restoration sends the current selected winner once. The rolled-back removal leaves B holding both routes. The late write installs nothing | The API announcement is never withdrawn and never re-sent across the four steps; other recipients are untouched |
| AC-24 | A process attached to no peer, and one attached to peer A with `receive [ update ]` only, beside the default transit config | The first receives no route event and every send it attempts is refused. The second receives A's routes and nothing from other peers. Neither changes when ordinary transit starts passing routes | `bgp-rib` is fed by the same rule: only the peers that attach it |
| AC-25 | Three refused configs, each then completed: A's block for `bgp-rib` grants `receive [ state ]` only while B is a complete destination; B's block grants `send [ update ]` only while A is a complete source; A's source grants name process `rib-one` and B's destination grants name process `rib-two`, both running `bgp-rib` | Each is refused at validate, commit and start, naming the peer, the process under its configured name, the missing grants and the completed `attach process` line. Completed on one process, each loads: B receives the elected route, receives the selected snapshot when its session comes up, and has its route refreshed on request | No grant is derived or widened at any step |
| AC-26 | Internal peers A and B with AIGP left at its RFC 7311 default (enabled), accepting export policies, an API process attached with `send [ update ]`, and no `bgp-rib` attached to any peer | The config loads at validate, commit and start. A and B receive the API announcements, and neither receives a route learned from the other | The config survives because neither an export policy nor a default-on capability asks for a RIB. The same config with `bgp-rib` attached as a partial set is refused under AC-25 |
| AC-27 | An ordinary recipient is replaced while received P remains unresolved; an auxiliary Adj-RIB-In store can send UPDATEs | Only current selected routes replay. The replacement receives useful K and no P, then receives P after reachability recovery without another source UPDATE. Typed/JSON observers receive advisory ownership without selected eligibility; auxiliary readiness still names the captured session | Useful selected FlowSpec reaches late and replacement recipients. No-owner auxiliary replay, RS/RR unheld-role fallback and explicit replay commands remain permitted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator configures a node between an iBGP and an eBGP peer and adds no plugin | wire → `bgp-rib` election → advertiser → forward rules → wire | `test/plugin/bgp-best-route-transit-default.ci` (planned name) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4271OrdinarySelectedWinnerReachesPeer` | `reactor/rfc4271_selected_advertisement_test.go` | Initial selected output; losing candidate remains stored but is not advertised | Initial output red, current 3734c8a8 log |
| `TestRFC4271OrdinarySelectedReplacementLifecycle` | same | Better replacement, retained fallback after withdrawal, final withdrawal exactly once | Initial output red; later assertions not reached |
| `TestRFC4271OrdinaryUnchangedWinnerIsNotReadvertised` | same | Useful initial winner survives identical receipt and losing-candidate churn without duplicate output | Initial output red; later assertions not reached |
| `TestRFC4271OrdinarySelectedExportDenialWithdraws` | same | Initial exportable winner, then denied global winner removes old advertisement without loser promotion | Initial output red; later assertions not reached |
| `TestRFC4271OrdinarySelectedMEDChangeReachesPeer` | `reactor/rfc4271_selected_boundaries_test.go` | Initial useful output followed by MED-only replacement | Initial output red; later assertions not reached |
| `TestRFC4271OrdinarySelectedIBGPSplitHorizon` | same | Permitted eBGP control and external recipient output survive iBGP-to-iBGP suppression | Initial output red; later assertions not reached |
| `TestRFC8212OrdinarySelectedAbsentExportPolicyWithholdsRoute` | same | Explicitly accepting recipient gets subject/marker while policy-absent eBGP recipient gets neither | Initial output red; later assertions not reached |

These seven fixtures currently start RIB and Adj-RIB-In SDK consumers
explicitly. When production grant checking and role discovery are implemented, route
their startup and permission setup through that production mechanism; do not
grant an artificial sender privilege just to turn the socket assertions green.
Any test/setup edit that changes what the fixture proves requires an independent
`ze-test-judge` verdict. Keep the actual storage, reachability, selection and
useful-output controls, and join the new dispatcher before negative observations.

Q-5 changes what these fixtures owe. A probe whose recipient is internal
(`TestRFC4271OrdinarySelectedIBGPSplitHorizon`'s internal recipient, and any
of the other six that use one) now needs a configured accepting export policy
on that recipient before its useful-output control can pass. Adding it is a
setup change the independent judge rules on, with the others. The RFC 8212
probe keeps its external recipients and its tag scope: the internal case is an
owner decision, not an RFC 8212 requirement, and its test carries no RFC 8212
claim.

Additional regression names below are proposed, not existing tests. Every
negative observation follows useful output and joins the work it observes.

| Proposed test | Boundary and useful result | Permitted survivor |
|---------------|----------------------------|--------------------|
| `TestOrdinarySelectedAcceptedLoserReceipt` | Through SDK and BGPConsumer on typed/external rails, retain losing origin with Announced zero and no false no-peer warning; remove real winner and observe useful loser output; separately force admission/transport error and assert reported failure; preserve withheld-withdraw count | Useful winner survives initial admission; retained loser becomes useful only after true winner removal, unrelated origins survive errors |
| `TestOrdinarySelectedScopedOriginAdmission` | A-only API/ExaBGP announcement competes with common learned winner; accepting B gets learned output, then A denies its selected origin and gets withdrawal without fallback | B and unrelated scoped origins survive because admission scope precedes export |
| `TestOrdinarySelectedLocalPreferenceReload` | Local initially wins; change existing static distance so learned wins while local remains valid; withdraw losing local | Learned replacement survives; no unconditional pin |
| `TestOrdinarySelectedRedistributionAdmission` | Admit static/OSPF through configured redistribution, compare using default and overridden distances, remove rule; include unadmitted system-route control | Admitted learned replacement survives; no system/FIB-only export |
| `TestOrdinarySelectedOriginReplayRollback` | Hold old local replay during preference apply/rollback and reconnect; observe current scoped winner on both internal and forked rails | Healthy scope output survives and stale history cannot replace current winner |
| `TestOrdinarySelectedInternalAbsentExportPolicyWithholdsLearnedRoute` | An internal recipient with an accepting export policy gets the learned winner and a later marker; an internal recipient with none gets neither, after the dispatcher is joined. Remove the first recipient's policy and observe the withdrawal. A local origin toward the policy-less recipient still arrives | The local origin and the selected Loc-RIB path survive; no loser is promoted at either recipient |
| `TestAbsentExportDispositionByRouteKind` | The one absent-policy statement over session type and route kind: external rejects both kinds, internal rejects a learned route under ordinary output and accepts a local origin and a role-served route; import is unchanged; a configured final ignores the kind | The existing RFC 8212 answers for external sessions |
| `TestExportPolicyReloadRetiresLearnedOnly` | An internal recipient holding a learned route and a local origin: remove its export policy, restore it, roll a removal back, and release a learned write queued before a removal. Observe the wire at each step, after joining the writer | The local origin is never withdrawn or re-sent; the restored learned route is the current selected winner, sent once |
| `TestOrdinarySelectedRoleDestinationsKeepDefault` | A reflector client with no export policy receives the reflected route beside an ordinary internal recipient that receives none; an external route-server client with no export policy receives none | The ordinary recipient's refusal proves the exemption is the role's, and the external refusal proves RFC 8212 still binds |
| `TestAbsentExportPolicyReportInternalSession` | The absent-policy value for an internal session with no export policy names the direction, what is withheld and the editor fix; it is empty once the policy is configured, empty for a destination an explicit role serves, and an external session keeps its RFC 8212 wording | A configured policy that rejects everything raises nothing |
| `TestOrdinarySelectedSourceDownRetiresOutput` | Advertise A, end its source, observe B replacement or final withdrawal | B survives only when it is the new globally selected eligible candidate |
| `TestOrdinarySelectedStaleDispatchCannotReplaceWinner` | Advertise A then B; release delayed A dispatch after B is committed | B remains, because older selection cannot replace newer output |
| `TestOrdinarySelectedPolicyApplyAbortDrains` | Advertise useful paths, apply captured source-policy denial and abort under held dispatch | Unaffected sources/local origins survive; restored paths follow the acknowledged abort policy and exact captured ownership |
| `TestOrdinarySelectedFirstPeerReplay` | Establish a recipient after selection; it receives selected snapshot and intervening replacements once | Current exportable winners survive; losing and withdrawn routes do not |
| `TestOrdinarySelectedForwardingClaimScope` | Ordinary destination gets useful output beside a configured RS/RR destination; transfer role on reload | Configured RS/RR behavior survives under its role; no duplicate ordinary writer at that destination |
| `TestOrdinarySelectedForkedTransport` | Same selected replacement/withdrawal results from forked and internal deciding plugin | Identical permitted route population and provenance on both transports |
| `TestOrdinarySelectedDispatchFailureRemainsOwed` | A successful control sends; fail one subsequent dispatch and recover it | Unaffected route remains; failed update is retried or reported incomplete, never silently acknowledged |
| `TestOrdinarySelectedRetainedWinnerLatePeer` | Source advertises, GR/LLGR owner retains it across DOWN, then new recipient establishes and receives that eligible retained winner with correct source/stale treatment | Retained winner survives only while retention is valid; a fresh better candidate may replace it |
| `TestOrdinarySelectedRetentionExpiryRevokesQueuedOutput` | Advertise retained winner, hold a queued replay, expire/purge or revoke its admission and join removal, then release old replay | New eligible winner/unrelated routes survive; expired receipt cannot resurrect old output |
| `TestOrdinarySelectedSlowDestinationIsolation` | Hold destination A's final flush, update selected route, and require B's useful output before releasing A; cancel A and join | B progresses and survives A cancellation; A's pending work remains bounded/owed |
| `TestOrdinarySelectedSlowDestinationUniqueKeyChurn` | Hold A's one in-flight batch; announce then withdraw many distinct keys never admitted to A, while B receives useful current routes. Assert pending state tracks only current selected/known output/in-flight keys, then release or cancel A and join | B's useful output survives; A's genuinely installed or possibly in-flight keys retain retirement until resolved, while never-admitted churn leaves no tombstones or owned snapshots |
| `TestOrdinarySelectedExternalFenceDeclaration` | A true external process absent from compiled registry declares fence+ready, emits initial snapshot, then live delta; observe correct ordering on socket | External non-fencing control remains non-fencing; alias/forked built-in behavior also preserved |
| `TestOrdinarySelectedAddPathZeroReplacement` | Advertise source path ID zero, replace selected path with another source/path identity, inspect destination ADD-PATH announcement/old-ID retirement | Only new selected path survives; zero is a valid identifier, and non-selected paths remain stored but not advertised by ordinary single-best output |
| `TestOrdinarySelectedOpaqueIdentityReplacement` | Advertise distinct non-CIDR native keys sharing an IP portion, replace/withdraw one, inspect exact wire identities and sent path IDs | Other opaque key survives; no prefix-only conflation, labels/path framing use existing native-key operations |
| `TestOrdinarySelectedDiagnosticLifecycle` | Send useful control, inject one destination failure and observe its actionable owned doctor issue on internal/external rails; reconcile and require clear | Healthy recipient output survives; export-policy exclusion completes normally and raises no failure issue |
| `TestOrdinaryForwardingRoleConflictRejected` | Valid ordinary/explicit-role configuration produces useful output; adding a second eligible explicit claimant fails verification naming peer and both owners | Existing valid running configuration remains authoritative after refusal; unrelated peer roles remain usable |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Source ADD-PATH identifier | Unsigned 32-bit including zero | 4294967295; zero explicitly valid | Negative/missing framing refused, never conflate zero with absent | Overflow rejected at protocol parsing |
| Selected revision within owner incarnation | Positive monotonic unsigned 64-bit revision | Maximum value handled without wrap; exhaustion requires a new owner incarnation/replay boundary | Zero/unrecorded authority refused; equal revision idempotent, older revision stale | Overflow must not wrap into a newer accepted write |
| Encoded selected batch | At most existing `rpc.MaxMessageSize`, including envelope overhead | Exact byte ceiling accepted | Empty batch is an explicit no-op, not a readiness/completion claim | Ceiling plus one split before send; a single unsplittable item fails its destination explicitly |
| Outstanding batches per destination | Zero or one | One may be blocked while another destination progresses | N-A | Second batch waits/coalesces in current-state difference, never creates another in-flight send |
| Pending key population | Output subset of current selected keys union known Adj-RIB-Out keys union one admitted batch's keys; candidate/membership storage bounded by current admitted origins/scopes | Every actual output/in-flight retirement retained | Never-admitted announce then withdraw leaves no key | Unique withdrawn-history-only key is a failed invariant; churn regression asserts no accumulation |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-best-route-transit-default` (planned) | `test/plugin/` | a node between two peers passes the elected route on with no route-server plugin | |
| `bgp-best-route-needs-export-policy` (planned) | `test/plugin/` | an internal peer with no export policy receives no learned route and still receives an API announcement; `ze config validate` and `ze doctor` name the peer and the fix; configuring the policy delivers the route; a reflector client with no export policy still receives its reflected route and raises no report (AC-20 to AC-22) | |

### Interop Tests (Scope: protocol)
Owed against both FRR and BIRD. Before a check expects either to accept or
print a route, read that behaviour in its source at the lab's version and name
the function.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-best-route-transit-frr` (planned name) | `test/interop/scenarios/` | FRR | FRR on one side learns, through Ze, the route FRR on the other side announced, with no route-server plugin on Ze; red when the change is reverted | |
| `bgp-best-route-transit-bird` (planned name) | `test/interop/scenarios/` | BIRD 3 | the same with BIRD on both sides | |

## Files to Modify
- Recommended RIB-extension source partitions are enumerated below. Docs:
  `docs/architecture/rib-transition.md`, `docs/architecture/core-design.md`,
  `docs/guide/route-reflection.md` ("Without Route Reflection"),
  `docs/guide/plugins.md`, and `contrib/netlab/ze/ze.j2` (Q-6).

The following bounded file groups are the authorized implementation scope.
Their dependencies determine when groups can run independently. The
origin-admission partition implements the resolved Q-3 policy and must land
with its selected receipt and claim contracts.

| Proposed partition | Source files implicated | Required integration |
|--------------------|-------------------------|----------------------|
| Selected-output authority | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_engine.go`, `internal/component/bgp/reactor/reactor_api_relay.go`, `internal/component/bgp/reactor/session_ownership.go` and the existing recovery writer | Same typed/external operation, original source provenance, selected-only recovery, replacement and denial retirement; finish the RPC producer/consumer census before approval |
| Origin admission and arbitration | Proposed `rib/rib_origins.go`; `reactor/reactor_api_batch.go`, existing cmd update text/wire handlers, SDK/bridge and plugin dispatch; `bgp/config/bgp_routes.go`; `bgp/redistribute/consumer.go`, `config/redistribute/consumer.go`, `plugins/redistribute/redistribute.go` | Registered typed/external origin intake, source-owned withdrawal, scoped membership, preference generation and actual local/received output kind; preserve API/ExaBGP syntax and role-specific paths |
| Selection dispatch and replay | `internal/component/bgp/plugins/rib/rib_bestchange.go`, `rib_recovery.go`, `rib.go`, `rib_replay.go`, `source_policy.go` in that package; proposed focused `rib_advertise.go` | Snapshot the retained winner, ordered dispatch outside election locks, first-peer replay, policy apply/abort and shutdown drain |
| Missing-grant report and role declarations (no binding is derived; `redistribute_binding.go` is read as the precedent for alias resolution and is not extended) | `internal/component/plugin/registry/registry.go`, declaration transport in `pkg/plugin/rpc/types.go`, `internal/component/bgp/config/redistribute_binding.go`, RIB/RS/RR `register.go` and their SDK declarations | Extend registration and existing claims, not a plugin-name switch; aliases, explicit grants, destination scope and reload transfer |
| Export default by route kind | `internal/component/bgp/filterapi/policy.go`; `internal/component/bgp/policychain/chain.go`; in `internal/component/bgp/reactor`: `filter_ordered.go`, `egress_inject_filter.go`, `forward_rs.go`, `peer_session_factory.go`, `session_write.go`, `peer_initial_sync.go` | One absent-policy statement over session type and route kind; the kind derived by the engine and read per route by both egress rails, the chain end and the final writer |
| Selective export reload | In `internal/component/bgp/reactor`: `policy_reload.go`, `peer_settings_apply.go`, `reactor_api_advertised.go` | Retire learned entries only on removal, restore from the selected snapshot, rollback and late-write refusal |
| Absent-policy diagnostics | `internal/component/bgp/reactor/rfc8212_warning.go`, `policy_dryrun.go` and `reactor_api.go` in the same package, `internal/component/plugin/types_bgp.go`, `internal/component/bgp/plugins/cmd/policy/handler.go` and its YANG, `internal/component/bgp/config/register.go`, and the doctor check declared beside the absence value | Internal export absence that respects the role exemption, on validate, the Established log line, the status surface, the dry run and `ze doctor` |
| Proof and user surfaces | Existing seven reactor probes, `test/plugin/bgp-best-route-needs-export-policy.ci`, `test/plugin/bgp-best-route-transit-default.ci`, proposed forked/replay/ownership regressions, FRR/BIRD scenarios and the docs listed above | Useful wire output, surviving unrelated routes, Linux entry-point proof and preserved RS/RR role behavior |

`internal/component/bgp/plugins/rib/rib_next_hop.go` belongs to concurrent
next-hop work and is not assigned to this proposal. Coordinate with that owner
instead of editing its live source. New public receipt/declaration fields also
owe `docs/architecture/api/process-protocol.md` and the owning SDK/API page;
the final census must name every actual typed and JSON producer/reader before
the design gate, including unchanged callers and non-Go clients.

## Files to Create
- `test/plugin/bgp-best-route-transit-default.ci` - functional test
- `test/plugin/bgp-best-route-needs-export-policy.ci` - functional test of AC-20 to AC-22
- the two interop scenarios above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes, proposed | Declare the distinct selected operation in `internal/core/ipc/yang/ze-plugin-engine.yang` with its authenticated-envelope semantics and extend Stage-1 fence/feed declaration. Register the same operation in `plugin/server/dispatch_registry.go`. No new operator config option; registered origin-intake request/receipt also needs declared plugin RPC and typed/external parity |
| YANG validation constraints | Yes, proposed | Selected key/revision/announcement-or-absence consistency, origin-kind-appropriate receipt presence for announcements, size/framing limits; Stage-1 fence requires ready signaling. Validate at the registered operation and declaration boundary |
| YANG custom validators | N-A | No new config leaf/path predicate. Use generic registration/declaration validation for feed requirements, eligible claimant conflict and fence/ready dependency |
| CLI commands/flags | No in current proposal | Ordinary operation is default, with no new operator command; separate selected RPC, if chosen, is an SDK operation |
| CLI grammar (keyword before value) | N-A in current proposal | No new CLI syntax; existing replay commands retain grammar |
| Editor autocomplete | N-A in current proposal | No new editable leaf/command; reuse configured distance declarations |
| Functional test for new RPC/API | Yes | Default transit test plus forked selected lifecycle and preserved legacy replay; typed/JSON dispatch tests in plugin server and reactor |
| Pipe completeness | N-A in current proposal | No new presentation or CLI answer shape |
| Env var registration | No in current proposal | No tunable justified; batching/failure policy is design work, not a new env knob |
| Doctor check for runtime dependencies | Yes, proposed | `rib/rib_advertise_diagnostic.go` owns selected-output reconciliation health; SDK issue reporting reaches daemon doctor on both transports. Check reports an unsettled destination with native key/revision, cause and exact corrective action; clear only after acknowledged reconciliation or session retirement. Contracts/authority phase wires it before RIB dissemination |
| Doctor code per problem class | Yes, proposed identifiers | `doctor-bgp-selected-output` in RIB diagnostic registration covers unresolved selected dispatch (permission, invalid receipt, resource/transport failure distinguished in cause); `doctor-plugin-peer-role` in generic plugin registration/delivery validation covers conflicting eligible role owners or insufficient mandatory grants. Declare code metadata at each owning registration, expose through existing doctor/report path, with tests for raise/clear and actionable fixes. These names are proposed, not existing codes |
| Doctor check for a peer that withholds learned routes | Yes, proposed | The absent-policy value of `reactor/rfc8212_warning.go` gains the internal session's missing export policy, and one doctor check reads that value, so validate, the Established log line and `ze doctor` cannot word it differently. A config-phase check over the peer's settings, declared by the package that owns the value; it names the peer, what is withheld and the editor command, and covers the external RFC 8212 absences the same way (AC-21) |
| Prometheus counters/metrics | No new metric proposed | Existing route/session metrics remain; owned actionable diagnostics cover reconciliation failures. Boundedness/progress are proven by state-and-wire tests, not a new counter. No metric is needed to implement the behavior |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/guide/route-reflection.md`, `docs/guide/plugins.md`; feature inventory level must follow demonstrated output, not this proposal |
| 2 | Config syntax changed? | No new syntax proposed | Update default/role examples and document explicit-origin use of the existing static preference class |
| 3 | CLI command added/changed? | No in current proposal | Existing replay command grammar preserved; SDK operation documentation handled separately |
| 4 | API/RPC added/changed? | Yes, proposed selected operation | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `docs/architecture/api/architecture.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/exabgp-bridge.md`; admitted-origin scope/results plus explicit source/selected receipts and completion semantics are specified above |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md`; `docs/guide/bgp-policy.md`, whose section on the default for an absent direction gains the internal session's export case and a before-and-after transit example |
| 7 | Wire format changed? | Yes, proposed plugin contract | Distinct selected operation, source-event receipt and runtime Stage-1 fence/feed declaration; BGP wire encoding stays existing, route population changes |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md`, process protocol and `docs/architecture/api/ipc_protocol.md`; same typed/external meaning |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` (RFC4271-9.2-7), `rfc/short/rfc8212.md` |
| 10 | Test infrastructure changed? | No advertiser harness change proposed | Existing native runners/fixtures suffice; related constant maintenance is separately recorded and proves no advertiser behavior |
| 11 | Affects daemon comparison? | Yes | Check and qualify ordinary-transit claims in `docs/comparison.md` against actual FRR/BIRD results |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rib-transition.md`, `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No arbitrary metadata key proposed | Source receipt and selected authority are explicit typed/JSON event and operation fields, documented in process/API protocol pages; do not smuggle authority through the free-form route metadata map |
| 14 | Prometheus counters added/changed? | No | Existing metrics unchanged; selected health is an owned doctor issue with reconciliation clear semantics |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Transit-grant-need, role and fence declarations and selected engine operation; `docs/architecture/plugin/plugin-system.md` and generated inventory. Recommended design extends existing RIB, adding no plugin |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Existing relay, best-change, replay and SDK anchors occur in the listed architecture/API pages; final exact anchor check belongs to main after file set is agreed |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the guides that attach `bgp-rs` for transit |

### Discovery and maintenance mapping

| Discovery question | Existing surface and proposed action | Proof owed after implementation |
|--------------------|--------------------------------------|---------------------------------|
| Where does an agent look first? | `ai/INDEX.md` already maps RIB storage, route selection, sent-history replay, plugin SDK and DirectBridge. Extend the owning architecture pages those rows name; add a row only if a genuinely new subsystem is chosen | Source anchors and current index point to the implemented owner |
| What prevents regression? | Existing permission, source-policy, replay-fence and selected-route tests plus proposed retirement/claim/drain regressions | Main runs useful-output controls and later negative assertions, forked parity and configured RS/RR preservation |
| What registry prevents drift? | Existing plugin registration, `engineOps`, opaque Claims and generated composition/inventory; a plugin-owned declaration of the grants transit needs is proposed, read only by the missing-grant report | Declaration parity, boundary/import checks and destination-scope tests; no central plugin-name list |
| What proves the user outcome? | Default functional transit and both pinned-daemon interop scenarios; seven existing real socket probes | Initial route, selected replacement, final withdrawal and export denial all reach the actual recipient wire with preserved survivors |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the functional test above, red at HEAD
2. **Proposed phase: contracts and authority** -- selected envelope, captured source facts/retention receipt, public runtime fence declaration, existing writer admission and completion, selected-only recovery. Preserve legacy relay callers unchanged.
3. **Proposed phase: RIB dissemination** -- retained capture/revision, per-destination bounded dispatch, snapshot/deltas, policy/retention/stop barriers; include scoped origin admission and configured-preference arbitration described above.
4. **Proposed phase: explicit grants and role wiring** -- the plugin's declared source and destination grant sets, their refusal at commit through the dependency spec's check, destination claim resolution, conflict verification and session-restart ownership transfer. No binding is derived.
5. **Proposed phase: integration and migration** -- production-wired seven probes plus retained-source, external, isolation and native identity regressions; judge fixture proof changes independently; migrate transit users and run configured-role/interop coverage. Main runs all tests and gates. The owner's implementation instruction authorizes these phases; they run in dependency order after the wiring RED.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | one owner per destination key whatever plugins are attached (Q-2, Q-3) |
| Data flow | the advertiser reaches peers through the existing forward rules, not a copy of them |
| Rule: `ai/rules/no-layering.md` | Ordinary transit uses the RIB selected path. Explicit eligible RS/RR owns its destination instead; no duplicate ordinary role writer or second candidate selector |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Default transit | `bgp-best-route-transit-default.ci` passes, red when reverted |
| Interop | both scenarios pass, red recorded |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Route leak | Ordinary selected output sends no learned route to a peer without a configured export policy, internal or external. No route of any origin or role reaches an eBGP peer without one (RFC 8212 Section 3). The two exemptions on an internal session are exactly a local origin and a destination served by an explicit RS/RR role, and no caller can claim either: the engine derives the route's kind; no system route enters advertisement without existing origination/redistribution admission |

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

- The unapproved concrete proposal above replaces the earlier technical open
  alternatives as the recommendation. Historical open rows record discovery,
  not additional owner questions. Q-3 policy is resolved; whole-design approval
  remains pending.

Research constraint: changing the initial send alone cannot satisfy AC-4 or
export-denial correctness. The writer presently knows received-source owners,
while ordinary dissemination needs authority over the previous selected
output. Preserve both identities instead of weakening source ownership.

Research constraint: no performance result is claimed. Snapshot ownership,
batching and dispatch lifetime must be designed before implementation, with
buffer-pool reuse and bounded work. The existing retained conversion is a
representation precedent, not permission for unmeasured per-event copies.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Transit needs `bgp-rib` attached explicitly; no binding is derived for it (owner decision, 2026-11-16) | derive the binding for every config; derive it where a peer has an export policy | Explicit configuration prevents surprises, its friction is the accepted tradeoff, and tooling carries the help (`ai/META-DECISIONS.md`). `attach process` is already the one control over what a process is fed and may send |
| A learned route reaches a peer only where that peer has a configured export policy, internal sessions included; no BGP-to-BGP redistribution mechanism (owner decision, 2026-11-16) | a Ze-only redistribution rule from BGP to BGP split by iBGP and eBGP; FRR's default of advertising to an internal peer with no policy | Owner decision in `docs/architecture/rib-transition.md`, "Owner ruling: a learned route needs the recipient's export policy". Keeps "nothing flows unless configured" on the knob every operator knows; BIRD 3.4.0 has the same default. By the same decision it binds learned routes only: local origins and explicit RS/RR roles keep today's absent-policy answer, since requiring a policy for every writer would stop reflectors and API announcements toward internal peers |
| Configured-preference competition (owner ruling, 2026-11-16) | unconditional local authority until withdrawal | Owner ruling in `docs/architecture/rib-transition.md`, "Owner ruling: local and learned route competition"; FRR/BIRD agreement guides defaults, Ze declared preferences resolve their disagreements. Exact local-origin class mapping remains a whole-design proposal |
| Best routes reach the other peers by default (owner ruling, 2026-11-13) | keep transit behind `bgp-rs` and document it | Thomas answered "absoletely" to the question quoted under Task |

## Known Limitations

- None yet.

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, `./le test functional plugin`, and the two interop scenarios through `./le test integration interop`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
