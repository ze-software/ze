# Deployment later-delta preparation, November 13

## Latest landed/runtime checkpoint (supersedes earlier pending rows)

GPG access restored. Interface commit actually landed as
0ec5666e2e942a26e871ead6789b3a51bf44cd4e. Its immutable path list has eight Go
paths plus three documentation/index paths: docs/features/interfaces.md,
docs/architecture/api/process-protocol.md and ai/INDEX.md (11 paths total).
The eight Go producers were reviewed previously and ROOT matched their blobs
and selected hunks to the prepared script. This reviewer read the three actual
committed doc/index deltas now: signal/topic navigation, startup/replay payload
contract, duplicate-add tolerance, inactive-provider behavior and monitor
ownership remain consistent with the retained source review. ROOT84733 is
running all-five flavor checks pinned to this actual commit; no verdict yet.

An intervening foreign enum-owner commit b9c22dfb7d landed between5797 and0ec.
Actual git metadata says141 paths, 8249 insertions and609 deletions. That is not
an owned deployment source-review verdict. At final immutable reconciliation,
identify any intersection with the retained431 producer population and review
those deltas as needed; do not silently treat the foreign commit as reviewed
or shrink the retained original population. No broad baseline reread now.

Connected landing is blocked by an actual scoped commit-create refusal:
connected.go replay-failure warning lacks the registered doctor code/transient
report-bus treatment. ROOT resumed the connected author for a bounded test-first
repair; earlier CLEAR freezes do not cover that forthcoming change. Keep the
warning and use the correct diagnostic, not an unrelated code or suppressed log.
Await exact new source/test freeze for independent delta review.

ROOT73645 is running the fresh FRR diagnostic VM and preparing images. There is
no live verdict. Interop observability now has six Go paths rather than the
earlier four: new check_ospf_redistribute_default_diagnostics.go and its test,
plus changed existing checker observations. Another independent reviewer
(default_runtime_dataflow) cleared the exact six-Go/three-scenario scope; the
three checker parent tests passed race in7275e264 (named receipt read here).
This retained reviewer has not yet read the new diagnostics bodies; that exact
delta is a remaining source-read task after diagnostic/source freeze. The old
seven-full-file interop prospective manifest is superseded for landing; do not
omit the new diagnostics files.

Netlab profile fixture obligations now passed on ZeKernel: draft baseline
a5d95e14, ROOT-reported stress80 receipt807779fa31 (actual scratch filename
ends7779fa31), and promoted live16ec32cf. The independently approved7080 patch
bytes were promoted exactly. Named baseline/promoted check-exit0,
command-exit0 and lab-exit0 excerpts were read here; do not confuse this profile
fixture proof with the still-owed unchanged six-ping full lab run.

No formal review gate/CLEAN/ze-close is started. Await new diagnostic receipt,
connected diagnostic-source freeze, pinned flavor verdict and final artifacts.

Independent retained reviewer, source only. Immutable baseline remains
5797d548acf77ffdf92c3d5b6504e37523b1722b: 431 historical native paths and all
120 foreign IKE paths retained, all scoped historical changes/reached producers
reviewed through three lenses. No baseline rereview or exclusion. Current
uncommitted source preparation does not enlarge that immutable census until
the exact committed delta is admitted.

No tests/builds/formatters/product execution, commits, subagents, tracked edits,
formal CLEAN or ze-close by this reviewer. Original older census runtime tables
are historical; current ROOT handoff owns remaining actual proof status.

## Current bounded source/read ledger

| Later bucket | Exact read scope / retained ledger | Disposition and remaining admission |
| --- | --- | --- |
| Interface replay | Eight Go paths and own interfaces guide delta in iface-address-replay-independent-source-review.md; exact eight-Go + doc list in iface-address-replay-commit-files.txt | Source CLEAR; match eventual committed blobs/hunks to frozen receipt, without rerunning the original 431. |
| Connected startup/removal | Four modified Go paths plus full new event consumers/regressions as recorded in connected-bootstrap-lifecycle-independent-source-review.md; full new lifecycle_test.go in connected-retirement-fences-independent-source-review.md; narrow connected redistribution guide paragraphs | Source CLEAR. Actual four guard-specific REDs and 4f5cd634 GREEN now independently read; exact extra Add, Remove, ReplayID73 Adds and repeated Removes all observed. Earlier seven real-runner parent receipt 522bd10c also read. Seven current Go paths total. Match committed versions; do not imply generic crash/external orchestrator repair. |
| BGP late-bound Default | Eight producer Go files, seven existing test adaptations and five full new test files; final writers/queue drains/parser/cursor/negotiation and all new wire helpers reached; exact prospective list below | Source CLEAR after the new caller-reference comments were added and read. Existing/default override behavior and final peer-own guards traced. Runtime/interop/freshness still ROOT-owned. |
| BGP RFC proofs | Actual four record-file diffs read; five exact handwritten reports independently approved in default-next-hop-rfc-handwritten-break-judgment.md | ROOT reports all eight native recordings successful and inspected semantic failures. Three existing draft rows renew whole-producer halt scope; five new mutant rows preserve their exact bounded claims. Native scoped check and immutable fingerprints remain owed; no source reviewer mints/reseals records. |
| New OSPF-to-eBGP scenario | Four full Go files, both configs, scenario README and own interop guide paragraph; actual FRR10.7.1 source independently checked as recorded in ospf-redistribute-default-independent-source-review.md | Source CLEAR. Read corrected guide attribution to Ze's final peer-own withholding; no predicate/assertion/config change. ROOT has old-explicit semantic RED after all prerequisite controls; restored actual scenario and all six netlab pings still running at this checkpoint. |
| RFC new-carrier move repair | rename.go changed hunks/reached producers, whole new rename_new_carrier_test.go, only own Moving-a-tagged-test-file guide paragraphs | Source CLEAR in rfc-new-carrier-move-independent-source-review.md. Actual old RED and ca7ff413 selected race/coverage GREEN read; ROOT reports actual canonical new LL move succeeded. Match committed source; no manual rename. |
| Netlab template/bootstrap | Current ze/ze.j2 and all three golden deltas: empty interface root and state-only receive grant to redistribution orchestrator; whole new internal/le/test/netlab/bootstrap_config_test.go | Source CLEAR for these narrow additions. Tests use actual schema/BGP registrations and parsed peer bindings; exact peer counts and receive/send grants plus unmanaged empty interface root are asserted. Existing native render/compare/validate path remains the template/golden boundary. Actual live traffic remains ROOT-owned. |
| Netlab tool recipe | Current whole Dockerfile.tools, own README/netlab-guide diff and source anchors read in this pass | Source CLEAR for the recipe/receipt text. Dockerfile prints versions and runs pip check; ROOT's actual tool image receipt and lab execution establish runtime scope. No compatibility or latest-version claim follows from source pins alone. |
| Remaining docs/site/artifacts | Later final source-bound preview, site/wiki/llms, news, owner-finding patches and spec reconciliation | Review exact selected final deltas when frozen. These are not accepted by the above source preparation or by old source-unbound container evidence. |

No concrete source blocker remains from this pass. One identified compliance
issue was fixed: new resolveQueuedNextHop now names RFC4271 section5.1.3 before
default selection and RFC4271 section5.1.3/RFC8950 section4 before common
resolution. The edit changes comments only. The exact current full-file hash
below includes it. No blind evidence resealing is authorized.

Netlab empty-root prose was also checked against actual
iface/config_apply.go::applyConfigPass and config_sysctl.go::applyIPv6ForwardingSwitch:
empty desired RPF sets replace Ze's checks, and the interface-owned IPv6 global
forwarding default is withdrawn rather than guessed or forced to zero. The
guide discloses these effects. Current narrow additional read hashes:

```text
483f0723e486fdb7a527cf7613c8586fae46c9949b3972a71583f9193dc287a2  internal/le/test/netlab/bootstrap_config_test.go
aef4be1db9faf0abe32a688061c3717e10ba687e0899cb0ecc79743b46ab6c4f  contrib/netlab/Dockerfile.tools
d769fa7edf3f101aaf8947c5d3f68564f7948b23705847a7681ed333cac5bf6a  contrib/netlab/ze/ze.j2
acbf8748442af193f2b587c37cda85f9c749778f417b3dfae01cf697fa555847  contrib/netlab/golden/r1.conf
3cd6280ba746b29d99a3581a53096b92618d6085603abede842bcba9f07e9921  contrib/netlab/golden/r2.conf
d918114a94251d7b29bb04a71459801471eaa17ecee8222852e98196e52a9279  contrib/netlab/golden/r3.conf
```

## Prospective BGP commit scope

`deployment-prospective-bgp-commit-files.txt` now lists exactly 25 full-file
candidates: 21 Go files (8 BGP producers, 12 tests, one OSPF comment-only update)
and 4 RFC records. It omits all
foreign working-tree paths, .pending files, private mutation reports and
generated RFC ledgers. This list is a proposal for ROOT's normal scoped commit
workflow, not a script, stage or commit action.

Prepared bodies, own contextual documentation patches and the additional
whole update-syntax doc input are indexed in deployment-prepared-commit-inputs.md.
The OSPF addEntry comment-only source hash is
3d7235b0e91fe9ea64ea1b918a29253eadd83cfbd9dd7e08707a030296e1c9ef.
Latest ROOT runtime steering supersedes the earlier running-live status above:
restored FRR ec27e3cc failed assertion5 after its four positive prerequisites,
without Ze peer-own withholding warnings; the chained netlab action did not run.
The missing path is now under independent source diagnosis, not a live PASS.

Shared documentation hunks accompany the BGP change separately:

- `docs/architecture/api/update-syntax.md`: table next-hop forms plus the new
  paragraph beginning `next-hop default <gateway>` and its three source anchors.
  Current diff is solely this BGP delta, but use a selected hunk if it changes.
- `docs/guide/redistribution.md`: only the paragraph beginning `When BGP
  receives a redistributed route with a gateway` and its two source anchors.
  The earlier connected table/startup/stop paragraphs belong with connected;
  unrelated family-refusal table deletions must not hitchhike.
- New scenario's own paragraph and three source anchors in
  `docs/architecture/testing/interop.md` belong with the four scenario Go files
  and three config/README files, not with the BGP producer commit. Other
  interop additions in the same working-tree file are foreign to this bucket.
- `docs/contributing/rfc-conformance-gates.md` Moving-a-tagged-test paragraphs
  belong with the separate move-tool repair. Other file hunks are not admitted.

Do not include unchanged session_write.go, capability/negotiated.go or the
private handwritten mutation reports as production changes merely because
records name those producers. No production cut remains; consumer.go content
hash exactly equals ROOT's pristine 80e43ceed... pin.

## Existing native test approvals

Read actual root namespace `tmp/commit-weakening-approved-79a29bf3.md`. Relevant
already-recorded approvals (reuse; do not fabricate another namespace):

- Exact patch d4383de5: redistribute.TestBGPConsumerInjectRouteExplicitNextHop.
- Exact patch 23d779e4: reactor.TestQueueRefusedAfterGateOpens,
  TestQueuedLocalAnnounceSurvivesRemoteWithdrawal,
  TestUpdateDelayQueueOverrunIsReportedNotSwallowed, TestPeerOpQueueOrdering,
  TestPeerShouldQueue, TestPeerOpQueueMixedOperations, TestPeerOpQueueOverflow,
  TestInitialSyncPanicReleasesPeerLocks, TestInitialSyncQueuedWritePanicRunsUnlocked,
  TestClosingDrainRunsQueuedTeardown. The helper Write call adaptations preserve
  all assertions/inputs; these units carry no old RFC tags.
- Exact patch 402ad71f: reactor.TestDefaultNextHopIPv6RequiresExactExtendedPair
  fixture publication repair, approved by this independent reviewer before edit.
- Exact patch 9192617b: update.TestUpdateTextDefaultNextHopRejectsUnusableGateway,
  malformed-gateway negative correction, approved before edit.

The new RFC tags add claims rather than changing previously tagged units. Their
five handwritten proof origins must be disclosed in the commit body. Keep the
20-Go/four-record change coherent; do not commit only a changed producer and
strand its stale/new proof records. ROOT still owns actual native RFC checking,
focused affected repeat verification, signed landing and appropriate committed
flavor builds. Failed full-reactor repeat20 is not a PASS; its separately owned
unchanged failure classes remain disclosed.

## Current prospective content hashes

These pin the read source at this checkpoint, not a future commit:

```text
313f2db48f4712d166698d1e931a4a2b29eeaf76be6ca69d1af0b2489e214f80  internal/component/bgp/plugins/cmd/update/cursor.go
560063fe05346302c62640a4dcda19a38dfcf9eb8f9e9256a88a62561014dc8f  internal/component/bgp/plugins/cmd/update/update_text.go
57167cc3e783989eab28b31e8109397719e98db0cf5321ae4298bb31b0a1fb1a  internal/component/bgp/plugins/cmd/update/default_next_hop_test.go
ece330e5bd537f1233994a4f892ee373fb8e4618a952cc27286c39fd6ce3364e  internal/component/bgp/plugins/cmd/update/default_next_hop_link_local_test.go
d9b5ea5bb65b288994425d239345886f119967820eb633d8c5b31dba089a8c95  internal/component/bgp/reactor/peer.go
6c19fdde6fdfa34de482711b10c1c44e2a8504e48854a6427dea78c6733ffc29  internal/component/bgp/reactor/peer_initial_sync.go
0621e10fa12485d625b2e2c5d91edd2e716863a16a4e3e8ef7f23bc3fbc15f5e  internal/component/bgp/reactor/reactor_api_batch.go
68db25f3d79ee19919289589faffde1b6132eff4cfc4b3f825b1672eb363aec3  internal/component/bgp/reactor/forward_initial_sync_order_test.go
3bca3750ef9d529383cdec354b8f6adffb24920abfcf29347bd3179d51499a76  internal/component/bgp/reactor/peer_initial_sync_unwind_test.go
b313c93b6dfee2794cc6cb8083504460a6463d3c6c6b420f248eb22e1f463b1d  internal/component/bgp/reactor/peer_queue_gate_test.go
1a965ce669d1f57bcdd18de06ee3e4de8e04f54aaea6f9556a02822bb0c60293  internal/component/bgp/reactor/peer_queued_teardown_test.go
582d219affcb3b1d07440088679f03a299ca349d09e485f2ae27612262ef8b99  internal/component/bgp/reactor/peer_test.go
a90a69463c7724001ad7f95ddc3091d5bc8ced4dd801aab7b0ba0fee58935ac6  internal/component/bgp/reactor/update_delay_test.go
d32eef91eded9d89fa7b6da9511a18b64dff196cc42808afb177636679c9627e  internal/component/bgp/reactor/default_next_hop_controls_test.go
2b0d9950f16d9e9db68934591a2ff882934eaa8a4008247186fd08b1aafb5269  internal/component/bgp/reactor/redistributed_next_hop_test.go
9fa962c61f194ea3a64bde1640500c7525bee0577a44efe217ab1187a9164f71  internal/component/bgp/reactor/rfc8950_default_link_local_test.go
80e43ceedf6727f343765a74222c5af83a814f1f0682bb7b8fb4234b7108d4b0  internal/component/bgp/redistribute/consumer.go
441e1e80060a65c449284325333ee1b0168afd9c236a0c3b4e0681bc7e434729  internal/component/bgp/redistribute/consumer_test.go
8d23e0aa6762895b737e7834cfb83d2f6e6a7fe8ebe9d1916ec2ffcafc1a42e9  internal/component/bgp/types/nexthop.go
7adf1d1dea894a1d0b440f3208095ce53f4030fd25ae5598541c9ed4342bb56c  internal/component/bgp/types/types.go
865fe01f39fb15c8f9e7d82222bb531f14e317559d3d16996bce46e6c532e540  rfc/discrimination/rfc4271/RFC4271-5.1.3-1.json
7c0600b03694b9cdc17c276e2665ed176e44dbfae2025dadbf3681afc192a27b  rfc/discrimination/rfc8950/RFC8950-4-1.json
67531a98aca8c53412f59bf9bbbfe761a26f57a3487d7110345ce8575c1d4e85  rfc/discrimination/draft-ietf-idr-linklocal-capability/DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1.json
2be00f7a0a51174754a53b6733e93b4889f5216c9763f46ff1955342b9bf1ffc  rfc/discrimination/draft-ietf-idr-linklocal-capability/DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1.json
```

## Formal closure entry still owed

ROOT must finish pending actual restored FRR/full six-ping netlab and draft/stress
promotion obligations, final preview/site/wiki/docs artifacts and exact owner
finding disposition. Then freeze the owned prospective immutable tree, reconcile
all later commits/hunks against retained 431 plus the above read ledgers, check
all acceptance evidence with its actual source/artifact limits, and explicitly
start the formal review round. Only after those prerequisites may CLEAN/ze-close
be recorded. No new kernel implementation or paused IPsec work is inferred.
