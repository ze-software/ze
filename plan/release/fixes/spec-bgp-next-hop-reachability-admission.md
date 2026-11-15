# Spec: bgp-next-hop-reachability-admission

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | An unresolved BGP next hop must not displace a usable route; changing interior reachability must update selection and installed forwarding state |
| RFC gaps | RFC4271-3.1-2, RFC4271-9.1.2.1-2 |
| Depends | - |
| Updated | 2026-11-15 |

## Task

Complete and review the next-hop admission work exposed by the RFC test
campaign. Retain received paths, exclude unresolved or prospectively
unresolvable next hops before Phase 2 ranking, and recover when interior
reachability changes. Internal and forked plugins must produce the same
selection and kernel result.

Review separately how this forwarding admission applies to the existing
transparent route-server relay and its recovery election. Preserve ordinary
router admission while the relay view and its authoritative context are reviewed.

The owner narrowed the current campaign on 15 November: build the test
batches, fix straightforward defects, finish investigations already underway,
and retain complicated fixes for later spec review. A failing test remains
visible and is reported. This skeleton owns the remaining investigation and
fixture work; it does not approve an implementation design or claim release
readiness.

No existing spec was found for this mechanism. The proof-debt spec owns
specified old evidence rows, and the dropped-error BGP engine spec explicitly
excludes BGP plugins; neither owns this admission change.

## Authority and Required Reading

- `rfc/full/rfc4271.txt`, Section 9.1.2:
  "If the NEXT_HOP attribute of a BGP route depicts an address that is
  not resolvable, or if it would become unresolvable if the route was
  installed in the routing table, the BGP route MUST be excluded from
  the Phase 2 decision function."
  → Constraint: a currently resolvable next hop is insufficient if installing
  the candidate would make its own resolution recursive or unavailable.
- `docs/architecture/route-selection.md` and `ai/digests/rib.md`
  → Constraint: preserve ranking, received route retention, ADD-PATH identity,
  and reselection on interior changes.
- `docs/architecture/api/process-protocol.md`
  → Constraint: prospective metric proposals are independent, bounded values;
  the preview never installs them.
- `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/core.md`, and
  `ai/doors/plugin-infra.md`
  → Constraint: forked plugins reach the daemon through the SDK; diagnostic
  RPCs must not hold the route-selection or remote-cache locks.

## Existing Mechanisms

| Mechanism | Producer | Disposition |
|-----------|----------|-------------|
| Normal route admission and election | `internal/core/rib/locrib/prospective.go`, `PreviewSelected` | Reuse the ordinary admission/election to preview one contribution without mutation |
| Recursive next-hop resolution | `internal/core/rib/igpcost/igpcost.go`, `Resolve` and private `resolve` | One bounded resolver loop serves ordinary and concrete prospective views |
| Candidate gathering | `internal/component/bgp/plugins/rib/rib_next_hop.go`, `candidateNextHopResolved` | Check feasibility before ranking; preserve families without address next hops |
| Remote metric lifecycle | `internal/component/bgp/plugins/rib/rib_remote.go`, `runRemoteMetrics` | Reuse the worker for bounded metric RPCs, cache invalidation, reselection and coalesced issue delivery |
| Engine metric RPC | `pkg/plugin/sdk/sdk_engine.go`, `RouteMetricsProspective` | Preserve the same response and validation over both transports |

## Current Evidence

| Observation | Evidence and limit |
|-------------|--------------------|
| Eight new admission/lifecycle tests and the immediate-next-hop reselection test passed | `job-unit-pkg-130c510f.log` and `job-unit-pkg-a904e39b.log` under the campaign session; this proves the unit boundaries, not installed kernel forwarding |
| Unanswered prospective metric request returns within its bound | `job-bgp-next-hop-as0-boundaries-1662fc0c.log`, `TestProspectiveMetricUnansweredRequestIsBounded`, PASS |
| Initial full affected package run was red | `job-rfc4271-next-hop-packages-125044b6.log`: 148 top-level and 124 nested failures; the other two affected core packages passed. Later scoped label repairs pass below; the whole RIB package has not been rerun and no new whole-package count is inferred |
| Shared fixture constructors supply no admission Loc-RIB | `newTestRIBManager` in `internal/component/bgp/plugins/rib/rib_test.go` and `newTestRIBManagerWithBus` in `rfc4271_rib_bestchange_test.go`; addressed candidates are now excluded before the old assertions |
| Other fixtures supply an empty Loc-RIB | Receive, self-next-hop, AIGP, ADD-PATH, MED, configuration reselection, ECMP, blackhole, ASPA, validation and shutdown fixtures need individual topology review; this is not a claim that every failing assertion has the same cause |
| The concrete resolver supersedes the earlier 320-byte prospective allocation | `job-next-hop-core-final-3b90dc48.log` measures ordinary and prospective direct/recursive resolution at 0 B/op and 0 allocs/op in the same run |
| Response bounds and registered issue reporting pass scoped execution | `job-rfc-campaign-new-batch-reds-37b87864.log` passes `TestProspectiveMetricUnansweredRequestIsBounded`, `TestRemoteMetricWorkerRaisesAndClearsBothTransports` over socket/DirectBridge, and `TestRemoteMetricRevisionFeedUnansweredRequestIsBounded` |
| No resolver-hook bypass defect established | `extractCandidate` calls the existing IGP distance hook before admission. Seed real reachability and rerun unchanged shutdown/locking assertions before changing their control seam |
| Internal and forked kernel tests passed and were promoted | Each draft passed once, then all 80 executed stress iterations in `job-rfc4271-next-hop-kernel-stress80-044ba988.log` and `job-rfc4271-next-hop-kernel-forked-stress80-11635610.log`. Both now live in `test/plugin/`. Ten native observations succeeded in `job-next-hop-kernel-native-proofs-343139f0.log`; two additional kernel-only observations succeeded in `job-next-hop-kernel-delete-native-proofs-e67e9153.log` |
| Kernel removal is independently observed after BGP and Loc-RIB clear | Omitting only runtime FIB withdrawal processing leaves `198.19.150.0/24 via 198.18.230.2 dev zesrnh proto 250 onlink`; both placements fail the actual kernel-zero-route assertion after their best-empty and zero-Loc-RIB checks pass. Their clean baselines pass. Shutdown still removes the owned routes |
| Existing cost-change coverage now has its own RFC mapping and native fault | `TestIGPCostChangeReselectsWithoutAIGP` retains every statement: only interior cost changes, without another UPDATE or AIGP. `job-next-hop-label-cost-semantic-proofs-3477f68f.log` observes clean PASS and a named failure at the cost-30 winner transition when the change subscriber's reselection is omitted |
| Two label fixtures now supply real independent reachability | Independently approved private connected `/32` entries for `10.0.0.1` and `.2` feed the same actual ordinary/prospective resolver. Every label, MED, peer, path ID, count and winner assertion remains. Three native semantic observations pass in `job-next-hop-label-cost-semantic-proofs-3477f68f.log`; all five selected label/cost/immediate-hop units pass under race detection in `job-next-hop-label-cost-final-race-4f0b2faa.log` |
| Private recovery admission preserves forwarded-wire recovery without relaxing ordinary forwarding admission | The initial behavioral baseline, `job-recovery-next-hop-admission-baseline-a2a5d3ac.log`, failed all four CIDR/VPN × published/parsed-command recovery cases while passing the twelve retained guards. After the producer fix, `job-recovery-next-hop-admission-green-9c322622.log` passed both new units, all sixteen cases, eight ordinary admission controls, immediate reselection and the bounded timeout under race detection |
| All eight foreign process recovery cases now pass | `tmp/session/2026-11-14-002623/scratch/job-unit-pkg-401fb152.log`: `TestRecoveryQueuedDeliveryTransport` passes direct/IPC, newer-target-owner/unrelated-prefix and control/held-sent; `TestRSFastReceiptRecipientExactlyOnce` also passes both transports. No artificial IGP seed or weakened assertion was added. The earlier `job-unit-pkg-f2e02aa7.log` records the reproduced failure |

Logs above live in
`tmp/session/2026-11-14-210034/scratch/`. Retain the package-failure log until
fixture review and reruns classify its failures. Retain the allocation log
`job-rfc4271-next-hop-bench-7ba4b266.log` only until the source commit retains
the zero-allocation comparison in `job-next-hop-core-final-3b90dc48.log`, then
remove the superseded log. Timing figures from a loaded host are not acceptance
evidence.

The relay admission investigation is complete: its concrete recovery caller
selects forwarded-wire recovery rather than local forwarding feasibility.
Retain the passing direct/IPC evidence through the source commit and the kernel
pass and stress logs through draft promotion and native proof review. The first kernel
attempt's `rfc4271-next-hop-kernel-guest-capture/16-rfc4271-next-hop-kernel/client-stderr.log`
under the campaign scratch records the argument-placement failure; the later
passing logs supersede its outcome, not the retained diagnosis.

The commit-preparation files `rfc4271-nh-production-files.txt` and
`process-protocol-prospective-only.patch` in the campaign scratch retain the
27 whole-file paths and the separate prospective-query documentation hunk.
Keep them through review and scoped commit preparation; remove them once the
matching source commit has landed and its native evidence is retained.
`rfc4271-nh-ready-files.txt` expands that base to the exact source, approved
label fixtures, promoted kernel tests and current RFC evidence population;
`rfc4271-nh-ready-body.txt` holds the matching commit explanation. Remove those
two commit inputs after the generated source commit script succeeds.

The same scratch holds `rfc4271-next-hop-kernel-reviewed-summary-proposal.txt`
and `rfc4271-next-hop-kernel-reviewed-audit-pending.json`, the independently
reviewed summary scope and native rejudge input. Remove both after their
canonical summary/audit source commit. Keep the two current native kernel
logs, the semantic label/cost log and final scoped race log through that commit
and the remaining evidence review; do not retain duplicate console exports.

## Route-server Role Investigation

RFC 7947 was read in full. Section 2.2.1 states:

> As the route server does not participate in the actual routing of traffic,
> the NEXT_HOP attribute MUST be passed unmodified to the route server clients,
> similar to the "third-party" next-hop feature described in Section 5.1.3. of
> [RFC4271].

Section 2.1 states:

> A route server MUST accept all UPDATE messages received from each of its
> clients for inclusion in its Adj-RIB-In.

The following sentences permit local-policy filtering from the Loc-RIB and
recommend one or more decision processes. The full RFC contains no explicit
instruction to disable next-hop resolution. Attribute transparency alone does
not establish that exemption. RFC 4271 Section 9.1.2's complete exclusion
sentence is quoted above; its surrounding text concerns Loc-RIB selection,
routing-table installation and actual forwarding. Ordinary forwarding admission
therefore remains required. Neither an RFC violation nor a universal RS-client
resolution exemption is established by these eight failures.

The deciding primary sources are pinned releases, not the older local FRR and
BIRD checkouts:

| Source | Deciding condition and limit |
|--------|-----------------------------|
| [FRR 10.7.1 `bgpd/bgp_route.c`](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c), `bgp_update_check_valid_flags` | Ordinary unicast and labeled-unicast use next-hop tracking without a `PEER_FLAG_RSERVER_CLIENT` exception |
| [FRR 10.7.1 `bgpd/bgp_nht.c`](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_nht.c), `bgp_find_or_add_nexthop` and `bgp_isvalid_nexthop` | `BGP_INSTANCE_TYPE_VIEW` returns valid without resolution; the ordinary validity predicate also accepts when zebra has never connected. These are view/standalone conditions, not a neighbor-role exemption |
| [FRR 10.7.1 `doc/user/bgp.rst`](https://github.com/FRRouting/frr/blob/frr-10.7.1/doc/user/bgp.rst), Views | A view distributes routing information through BGP independently of Zebra/kernel routes and does not install selected routes into the kernel |
| [BIRD 3.4.0 `proto/bgp/bgp.c`](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/bgp/bgp.c), `bgp_postconfig` | RS-client mode defaults next-hop preservation to `NH_ALL`; gateway mode defaults independently to direct for direct sessions and recursive for multihop |
| [BIRD 3.4.0 `proto/bgp/packets.c`](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/bgp/packets.c), `bgp_apply_next_hop` | No `rs_client` exception: direct mode requires a directly reachable, nonlocal neighbor; recursive mode uses the configured IGP table. Direct mode does not require an interior route-table cover |
| [BIRD 3.4.0 `proto/bgp/attrs.c`](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/bgp/attrs.c), `bgp_rte_better` | Route resolvability is compared before ordinary BGP ranking |

The implementations differ. FRR view behavior cannot be inferred from an RS
neighbor flag, and BIRD's direct-neighbor check cannot be replaced with a claim
that every RS needs an interior Loc-RIB cover. This research selects no universal
precedence or new Ze mode.

The foreign fixture in
`internal/component/plugin/process/recovery_delivery_integration_test.go`,
`recoveryDeliveryRouter`, sets `RSClient` and `RSFastPath` on four loopback TCP
peers. `recoveryAnnouncement` supplies `192.0.2.{2,3,4}` next hops. It establishes
neither direct-neighbor reachability for those next hops nor a non-forwarding
view contract. Its red result must not be cleared by an assumed synthetic IGP
seed or by a blanket RS-client bypass; neither is approved.

The concrete sharing point is `rib_recovery.go`, `recoverySurvivorsLocked`:
it now selects `candidateAdmissionRecovery` in the shared
`gatherPrefixCandidatesForLocked` and `gatherKeyCandidatesForLocked` producers
in `rib_commands.go`. The ordinary gather wrappers select
`candidateAdmissionForwarding` and retain `candidateNextHopResolved`; for CIDR
routes, that predicate previews installation into Ze's forwarding Loc-RIB.
The live `bgp-rs` relay (`server_forward.go`, `batchForwardUpdate`) and native
relay (`reactor_notify.go`, `notifyMessageReceiver`) do not depend on this
election. Before the repair, recovery shared ordinary forwarding admission and
could exclude a live-relayed path solely because its next hop could not resolve
locally. The private recovery purpose removes that mismatch without relaxing
the ordinary forwarding election.

`Candidate`, `peerMetadata`, `PeerInfoJSON`, `StructuredEvent` and
`ribevents.RecoveryRequest` do not carry an RS role. A subsequent caller trace
establishes the needed context without adding a public role field or mode:

- The only reactor production election caller is `recovery_owed.go`,
  `recoveryWorker.decide`. It snapshots destination Adj-RIB-Out owed records,
  drains source publication receipts and the owner's applied event deliveries,
  then calls `lookupRecovery`. Its retry changes only the PATHS-LIMIT bound.
- `relay_recovery.go`, `lookupRecovery`, reaches the same `recoveryRoutes`
  producer through `ribevents.RecoveryProvider` or the hidden
  `request bgp rib recovery` command. The IPC parser does not select a different
  election. Neither path installs a forwarding-table contribution.
- Recovery is triggered by forwarded owner withdrawals, sent lifecycle
  removals, RS/RR source-DOWN inventory and destination PATHS-LIMIT contests.
  `WithdrawNLRIBatch` first checks the sender's update-send grant.
  `oweDeparture` requires matching source ownership and an ended session within
  the stated generation. Owner withdrawals use `recoveryOwedFor` to require
  that the registered recovery process receives the source's UPDATEs.
- `queueRecoveryOutcome` checks the original import policy epoch and tries
  candidates through `queueRecovery` and `forwardUpdateCore`. Reconstruction
  requires a current source publication receipt. Ordinary export/reflection
  policy and final destination ownership, session and epoch checks remain on
  the wire writer. An originated announcement removes the owed record.

This is an existing forwarded-wire recovery contract, distinct from ordinary
Loc-RIB installation. The implemented private gather purpose selected at
`recoverySurvivorsLocked` omits only local forwarding-resolution admission for
that view, without a new public API or operator choice. Ordinary gather entry
points remain subject to `candidateNextHopResolved`. The shared import-policy,
self-next-hop, SRv6 and validation gates, retained snapshot ownership, ranking,
destination/down/excluded-source filters and writer checks remain in place.
`RSFastPath` remains an execution choice, not an admission exemption.
`rib_recovery_admission_test.go` checks both recovery entry points and ordinary
forwarding exclusion against the same retained input. The scoped and direct/IPC
passes are recorded above; broader fixture repair, proof evidence and closure
remain open. No synthetic reachability or weakened assertion was added.

## Remaining Work for Review

The later scoped run
`job-rfc-campaign-new-batch-reds-37b87864.log` passed all eight admission tests,
the immediate-next-hop reselection test, the prospective response bound, issue
raise/clear over both transports, and the unanswered revision-poll bound.
`job-next-hop-core-final-3b90dc48.log` passed both affected core packages and
measured ordinary and prospective direct/recursive resolution at zero bytes
and zero allocations per operation in the same run. This supersedes the
earlier allocation observation; the original failure log remains useful until
the reviewed fixture work is complete.

The concurrent CI-fix session's static rerun also reported read-error-to-value
findings at `remoteRIB.prospectiveDistance` and `RIBManager.igpDistance`.
Both currently report the failure and return unresolved distance, rather than
propagate an error to the selection caller. Review that fail-closed boundary,
including repeated failures and warning coalescence, before clearing the
findings. No static-check exemption or resolved verdict is claimed here.

Native diagnostics found stale RFC8277-3.1-1 records for the changed gather
wrappers. Both affected unicast label fixtures lacked actual reachability;
their approved local topology repairs and current semantic proofs are recorded
above. Label-biased preference now fails ADD-PATH MED selection and the
two-session post-swap invariance assertion; bypassing MED elimination fails
the initial two-session winner. These faults return normally and preserve
candidate ownership. The old producer-panic route held `peerMu.RLock` while
test cleanup tried `SetLocRIB(nil)`; its captured SIGQUIT diagnostic in
`job-next-hop-label-cost-native-proofs-f7a59a75.log` was refused by the recorder
and wrote no evidence. Retain it only until this diagnosis and the replacement
semantic results are committed, then remove it.

`TestRFC8277VPNAddPathLabelsAreOneElection` retains its renewed positive record
in `job-next-hop-vpn-label-current-proof-5ab81f55.log`; that panic observation
proves wrapper reachability, not an independently discriminated VPN MED order.
The separate existing opaque mixed-session unit supplies the per-peer
aggregation boundary; its mapping and native proof are reviewed separately.
`job-label-opaque-aggregation-native-proof-5c1adaba.log` now observes the
unchanged VPN mixed-mode unit pass, then fail its two-candidate assertions
when the opaque gather stops before a second peer contributes. Its EVPN
sibling is not claimed for RFC8277. The refreshed audit combines that boundary
with the separate label-bias and MED assertions; it makes no full RFC8277,
live-session or interoperability claim. Retain this one current log through
the source commit and remaining evidence review.
The changed `igpcost.Resolve`
producer's positive and negative RFC7311-3.4.3-2 records are renewed in
`job-next-hop-aigp-current-proof-51c2e268.log` and
`job-next-hop-aigp-negative-proof-83a0338e.log`; each records a clean named-unit
pass and failure under its producer break. These whole-producer reverts prove
dependence on the named boundary, not every semantic fault or whole-RFC support.

1. Audit each failing fixture and its role. For ordinary forwarding fixtures,
   establish explicit independent terminal host reachability in the actual
   admission Loc-RIB, preserving table-size assertions and
   avoiding an always-resolved stub. Obtain independent test-judge approval
   before changing shared helpers or RFC-tagged tests. The shared gather's
   private recovery purpose and the eight foreign process cases are now repaired
   and passing as recorded above; ordinary forwarding fixture review remains.
2. Run unchanged ranking, ownership, allocation and concurrency assertions.
   Distinguish fixture failures from any further product defect at its producer.
3. The concrete prospective resolver's allocation comparison passed at zero
   bytes and allocations for all four cases in one toolchain/architecture run.
   Retain that comparison and remeasure if the resolver changes.
4. Issue lifecycle tests passed over socket RPC and DirectBridge, including
   the unanswered revision-poll deadline and report delivery outside both locks.
   Retain their evidence while reviewing the fail-closed findings above.
5. The internal and forked kernel drafts in `test/draft/plugin/`, named
   `rfc4271-next-hop-kernel.ci` and `rfc4271-next-hop-kernel-forked.ci`, have each
   passed once, completed 80/80 stress iterations each, and were promoted.
   Their native admission, cover-change, removal and direct-control proofs are
   recorded above. Preserve usable fallback, withdrawal when none resolves, and
   restoration without a new UPDATE. Add the separate live prospective
   self-dependency case before completing RFC4271-9.1.2-1.
6. Record actual passing-baseline/failing-break proofs before adding or upgrading
   RFC claims. Keep the mandatory gap rows honest until whole-path evidence exists.

## Risks and Assumptions

| ID | Concern | Validation |
|----|---------|------------|
| A-1 | Common empty-topology setup explains many reds, but has not classified every failure | Review each failing unit and rerun after its approved topology correction |
| R-1 | A broad reachable default hides prospective recursion defects | Seed independent host reachability and retain explicit self/mutual-recursion negatives |
| R-2 | Reporting while selection holds a lock deadlocks the plugin | Worker tests must acquire both locks from report callbacks on both transports |
| R-3 | A unit preview passes while the daemon still installs an unresolved route | Assert actual kernel state in both daemon modes |
| R-4 | Forwarded-wire recovery inherits forwarding-table feasibility | Select its existing recovery context at the producer, prove equal direct/IPC behavior, and preserve ordinary admission |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `conflicting-feature-contracts.md` | 2026-11-15 | RS live relay and recovery next-hop admission |

## Review Gate

Research and independent review of the bounded recovery producer repair are
complete; scoped recovery and ordinary admission tests pass, including real
direct/IPC delivery. Kernel stress and native fault observations passed both
placements; promotion is complete. Independent current audits enforce
RFC4271-9.1.2-4 and RFC4271-9.1.2.1-2. RFC4271-9.1.2-1 remains partial because
the live prospective-recursion condition is unproven. Broader fixture repair,
the existing fail-closed findings and spec closure remain outstanding. No full
RFC support or whole-package pass is claimed.
