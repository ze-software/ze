# Handover 54: spec-rfc4301-architecture-gaps, Review Gate fixes before closure

Spec: `plan/release/fixes/spec-rfc4301-architecture-gaps.md`, in-progress. Claim it
with `./le spec claim spec-rfc4301-architecture-gaps.md`. Thomas ordered it closed
("close it then", 2026-11-20). Handover 53 is done: its work landed as `25a3115a04`
(RFC 4301 rows re-judged, ICMP and DF records, audits). RFC check: no rfc4301
finding.

## Why the closure stopped

The closure gate (`judgeReviewCoverage`, `internal/le/commit/review.go`) pins every
code file the spec's 77 `Spec-work: rfc4301-architecture-gaps` commits touched: 216
files, about 31k added lines. The first closer reviewed only `25a3115a04`. The
Review Gate over the whole diff ran as seven independent read-only reviewers split by
package (engine x3, dataplane x2, ipsec/transport/cmd, tests and le). Their findings
are being fixed. The review is not recorded yet, so commits A and B have not landed.

Commit A's script `tmp/commit-b9332549-b-10d4e5.sh` was created (session namespace
b9332549) and NOT run. It is stale: recreate it. Its file list: the spec,
`rfc/short/rfc4301.md`, `plan/release/validation/spec-ipsec-fragment-receive-and-kernel-proof.md`,
`website/changes/fragments/rfc4301-architecture-gaps.md`, `rfc/audit/rfc4301/RFC4301-6-1.json`,
plus every fix below. Commit B: `./le commit create append script <A's script>
subject "plan: remove spec-rfc4301-architecture-gaps (closed)" remove
plan/release/fixes/spec-rfc4301-architecture-gaps.md`.

## Uncommitted fixes in the main checkout (this spec's; carry them)

| # | Finding | Fix (producer) | State |
|---|---------|----------------|-------|
| 1 | BLOCKER: an operator SPD entry or transit gate replaced a peer's Child SA policy with the same kernel identity (Linux matches selector, dir, if_id, mark; priority ignored), so protected traffic left in clear | `policyOwners.claim`/`deleteThenRelease`/`operatorHeldLocked` refuse both ways (dataplane/policy_owner.go); `spdSharedKernelPolicy`, `spdPeerSelectorRefusal` refuse at commit (engine/spd_order.go); `spdInstallRefusal` fix text; tests policy_owner_node_wide_test.go, engine/spd_peer_selector_test.go, engine/spd_peer_selector_integration_linux_test.go | written, unit green on darwin |
| 2 | Residual of 1: the node-wide catch-all has the same identity as a non-VTI 0/0 peer PROTECT; every reload now fails while such a peer is up, and its teardown removes the catch-all | design agent in flight (engine/unmatched.go, apply.go) | IN FLIGHT |
| 3 | VPP backend had no IKE bypass after e700fb7392 (latent: VPP refuses IKE's policies today) | `ensureIKEBypass`, `vppIKEBypassEntries` (dataplane/vpp.go, vpp_policy.go); evidence expectations in internal/le/test/deployment/vppevidencerun.go; vpp_ike_bypass_test.go | written |
| 4 | Ungated dataplane imported gated ike/transport for the IKE socket mark | `dataplane.PriorityIKESocket` (override_guard.go); transport uses it | written, unit green |
| 5 | Dataplane refusals and lease bug | `xfrmForwardDefaultRefusal` names the kernel build; `vppICMPSelectorError` names set commands; `changeForwardDefault` stale lease; `xfrmZECapKnown` hoisted; recording backend ICMPErrorYield; `TestRuntimeKernelBuildFixMatchesHostDriver` | written, unit green |
| 6 | Engine: fragment-gathering answer not pinned; verify callback untested; wrong MOBIKE diagnostic; three severities; gate collision text; double Error log and RFC wording | `spdForwardAnswer` (spd_forward_answer.go, apply.go); `verifyIPsecCommit` (config_verify_commit.go); `CodeDoctorIPsecMOBIKESocketNotShared` (register_mobike_socket.go, codes.go); forward default Error everywhere; `spdCollidingGate`; `unmatchedStartRefusal` quotes RFC 4301 Section 5 | written, unit green |
| 7 | IPsec config | `TestShowDataplanePolicyRendersICMPErrorPayload`; port refusal an opererr naming UDP-Lite 136; stale comments child.go, register.go; docs/guide/ipsec.md | written, unit green |
| 8 | Kernel-cache tooling | `servedKernelCachePath`, `servableKernelCachePaths` (internal/appliance/cmd_kernel.go), variadic `evictKeepN`; `RuntimeKernelBuilder` opererr; tests incl. internal/le/test/qemu/dockerlab_provenance_test.go | appliance and qemu unit green; lint owed |
| 9 | Dataplane test findings D1-D5 and follow-ups (evidence probe retarget, design page VPP paragraph, dataplane_test.go comment) | agent in flight | IN FLIGHT |
| 10 | Engine test findings E1-E14 and two stale comments | agent in flight | IN FLIGHT |
| 11 | goconst: duplicate `spdPrefixLabel` | deleted; `ipNetText` used (spd_order.go) | written |

The findings list for 9 and 10 is `/Users/thomas/.claude/jobs/1f8958df/tmp/wave2-findings.md`.
That path is outside the repo; if it is gone, the reviewers' findings are summarised in
this table and in the spec's Review Gate once recorded. Other sessions' uncommitted
edits share these trees (l2tp, bgp rib, command pipe, several specs): carry none.

## Owed next, in order

1. Wait for items 2, 9 and 10 to finish (their edits land in the tree whether or not
   this session is alive). Check `git status` and the compile.
2. Independent ze-test-judge, one batch, for each changed existing test:
   `dataplane.TestPolicyOwnersOwnAChildSAForwardAllow` (inverted: operator claim on a
   peer's selector now refused; old expectation contradicts RFC 4301 4.4.1 and let
   protected traffic leave in clear); `engine.TestDoctorIPsecForwardDefaultReadsTheLiveChecker`
   (warning to error); in `rfc7296_vpp_message_test.go` the message counts of
   `TestVPPInstallPolicyCreatesAndBindsSPD` (3 to 7), `TestVPPRemovePolicyParams`
   (1 to 5), `TestVPPCloseRemovesWhatItInstalled` and
   `TestVPPCloseReportsARefusedRemovalAndContinues` (4 to 8); and whatever items 9
   and 10 report (tagged E3 rename, E9, E10, E11; D1).
3. `./le rfc record` re-takes for each tagged unit items 9 and 10 changed, in Ze's
   arm64 guest: kernel `/Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-53c57c37-5d7f6651/vmlinuz`,
   image `/Users/thomas/.cache/ze/docker-host/arm64/c88a4387bff259f1/docker-host.img`.
   `./le commit create` prints the exact line for each stale record.
4. Scoped runs, one CPU job at a time: `./le test unit pkg path` for ike/dataplane,
   ike/transport, ike/engine, ike/ipsec, ike/cmd, core/diagnostic, appliance,
   le/test/qemu, le/test/deployment; `./le go lint run scope ./internal/component/ike/...`
   and the appliance, le/test/qemu, le/test/deployment scopes; `./le arch tier check`;
   the Linux integration tests in the guest (`./le test qemu`), at least
   `TestOperatorGateNeverReplacesALiveChildSAPolicy` and every integration test items
   9 and 10 changed; `./le test functional parse` for `ipsec-icmp-type-selector-refused`;
   the VPP deployment evidence if the image is available.
5. A second review round over the fixes only (independent reviewers, as before).
6. Record the Review Gate: `./le spec review record spec rfc4301-architecture-gaps
   verdict CLEAN rounds <n> file <path>...` over all 216 paths (list:
   `judgeReviewCoverage` refusal from `./le commit create append ... remove <spec>`)
   plus every new fix file, by a reviewer of the rounds, then `./le spec review check`.
   Fill the spec's Review Gate with every finding above and its fix.
7. Recreate commit A (replace) with the fix files added, commit B (append remove),
   run the script, `./le spec citation`, `./le spec release`.

## Item 9 result (dataplane test findings, reported after this handover was first written)

Done except D1. D1's rewrite is in
`tmp/session/2026-11-20-215415/scratch/d1-xfrm_bypass_integration_linux_test.go.proposed`;
the weakening hook refused it until a judge rules. Renames: `ikeBypassOut`,
`TestXFRMIKEBypassOutranksChildSAPolicyInKernel` becomes
`TestXFRMOperatorBypassOutranksChildSAPolicyInKernel`, and
`TestXFRMBypassInstallIsIdempotent` becomes
`TestXFRMOperatorEntryReinstallReplacesInPlace`, which proves more. Send them to the
judge with the batch in step 2.

- **D2:** the per-socket IKE bypass moved to `dataplane.BypassIKESocket`
  (dataplane/ike_socket_bypass_linux.go, _other.go, _linux_test.go). Owed deletion
  in commit A: `remove internal/component/ike/transport/bypass_linux.go`,
  `bypass_other.go` and `bypass_linux_test.go` (now dead). The test move needs a
  judge ruling that it is a move.
- **D3:** new leaf `internal/test/vppcleartext/vppcleartext.go`;
  `testBypassPriority` was renamed `testOperatorOrder`.
- **D4, D5, F1 (probe-esp-discard now targets 100.64.45.20), F2 (design page VPP
  paragraph), F3:** done.
- **Owed:** gofmt on `vpp_real_cleartext_integration_test.go`,
  `kernel_scope_wire_linux_test.go`, `vppevidencecleartext.go` and
  `vppevidencecleartext_proof_test.go` (script-edited). Comments changed inside the
  tagged unit `TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`
  (RFC4301-7.4-2), and in `rfc4301_spd_lifecycle_test.go` and
  `rfc4301_espform_policy_integration_linux_test.go`: `./le rfc check` says whether
  a record needs re-taking.
- **Possible defect, not fixed:** on VPP the IKE BYPASS entries pass any ESP
  addressed to the tunnel end whose SPI matches no PROTECT, because VPP's inbound
  ESP lookup matches on addresses only (`ipsec_input_policy_match`, VPP v26.06). The
  design page states it. Fix it in this spec if small, else give it a journal row
  owned by `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`.
- **Stale comment:** `TestVPPInstallPolicyBypass` in `rfc7296_vpp_message_test.go`
  still names the deleted `engine/bypass.go`.
- **Compile state:** once every agent had finished, each symbol the editor
  flagged was declared once in source (`PolicySelectorsEqual`,
  `PolicyInfo.Displaced`, `xfrmPolicyUpdate`, `catchAllYieldsToICMPErrorCheck`,
  `xfrmPolicyDirIn`). The editor's errors were stale. Not compiled since the last
  wave: start with the scoped unit runs.

## Item 2 result (catch-all vs wildcard peer, reported after this handover was first written)

Written, nothing run.

- **Design:** in the XFRM ledger, new `dataplane/policy_final.go`. A catch-all
  asked for under a peer's PROTECT holder is recorded in `policyOwners.finals`
  with no kernel call (`yieldFinal`). The peer's removal becomes one
  `XFRM_MSG_UPDPOLICY` of the kept catch-all over the PROTECT, with no delete
  (`reinstateFinal`), so there is no gap.
- **Same collision for the interface catch-all:** a `vti bind` wildcard peer
  has the same collision with `interfaceSPDCatchAll`, and the fix covers it.
- **Visible:** `PolicyInfo.Displaced` is rendered by `show vpn ipsec dataplane
  policy` as `displaced-final-entry` and `displaced-final-action`.
- **Prior art:** strongSwan's `add_policy`/`del_policy` reach the same kernel state.
- **Tests:** `dataplane/policy_final_test.go` (5),
  `dataplane/xfrm_final_entry_linux_test.go` (2, over a fake kernel map, seam
  `xfrmPolicyUpdate`), the guest-root
  `engine/spd_wildcard_peer_integration_linux_test.go`
  `TestWildcardPeerStandsInForTheCatchAllAndGivesItBack` (untried: a 0/0 Child
  SA may trip the lab), and `cmd/show_dataplane_displaced_test.go`.
- **Docs:** ipsec-8 "Policy ownership", ipsec-dataplane-inspection.md,
  guide/ipsec.md.
- **Owed:** gofmt on policy_owner.go, policy_final.go, xfrm_linux.go,
  dataplane.go, show_dataplane.go, unmatched.go and the two new Linux test files
  (script-edited). The spec needs an acceptance criterion for this fix. No RFC tags
  were added.

## Known reds outside this spec

`./le doc check all` (BGP rib filter order, ze-cos cleanup with no handler),
`./le spec citation` (two citations of the closed spec-l2tp-session-identity-and-publish-stall),
`./le cli grammar`, 227 RFC check violations in other stems. The disk-pressure
launchd job `net.ze-software.go-cache-pressure` runs `go clean -cache` when free
space is under 20 GB and no Go build runs; it broke one record run mid-build here.

## Notes for the spec record

- 6-1 is now `{partial}` (ICMPv6 gap, fragment-receive AC-8), stamped partial.
- Auditor NOTE not fixed: the 6-1, 6.2-3, 6.2-4, 6.2-5 reasons say "the first
  paragraph of Section 6" applies the section to ICMPv6; it is the second paragraph.
  Fixing it stales four verdicts.
- Several new records break a producer with a panic body (reach, not
  discrimination); `./le rfc check` accepts them.
- `docs/architecture/appliance/build-artifacts.md` carries another session's test
  anchors `TestCachedRuntimeKernelTreeServesEitherBackend` and
  `TestRuntimeKernelTreeAdoptsTheOtherBackendsBuild`, which exist nowhere.
