# Handover: privilege-skipping tests run unprivileged in a user namespace

Spec: `plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`
(status in-progress). Claim it with
`./le spec claim plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`.
Thomas ordered this work ("convert A and B", plus the `/dev/ppp` setup step).
Session rule from Thomas for the session that wrote this: ask before starting
any agent. Assume it still holds and ask him at the start.

Evidence and per-batch handoffs live in the old session's directory
`tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/`:
`state/session-state-spec-privileged-tests-run-in-a-user-namespace-f3ae7c00-09af-4420-914f-39f2f7169980.md`
(batch handoffs) and `scratch/userns-spec-edits.md` (every spec edit owed,
collected while the spec was frozen for parallel agents). Read both first.

Continuation evidence is in
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/`,
`state/session-state-privileged-tests-run-in-a-user-namespace-644a4e78-0122-4eeb-8642-eb50638a1641.md`.
It supersedes the old session for P3/P6 and records the current kernel
prerequisites. Keep the old files for pending P5/P7 proposals and P10 edits.

## Landed

| Commit | What |
|---|---|
| 88e01f1f6c | First conversion (dhcpserver neighbour tests), the pattern |
| d42fd1d126 | P1: `userns.Enter` gains a mount namespace and a private `/run/netns`; non-Linux `Enter` runs in place |
| 2bdc5103d8 | `Enter` fails when the child ran no test; forwards the parent's flags (except the withheld list) |
| 55cb1821dd | P2: `ppp-access` setup step (udev rule, group `dip`); Thomas ran `./le setup install`, `/dev/ppp` is `root dip 0660` |
| 8cf23c5998 | P9: the two `/dev/ppp` tests |
| 813ee68c52 | P4 iface-core, 65 tests |
| cf1fe4693e | P4 RFC 9915 tagged units (judge-approved, records re-taken) |
| 82a6aa53b6, 353feb201b | P5 iface plugins, untagged files |
| dbeedd8ffe, 25fbb6e890 | P7 routing, untagged files incl. ospfv3 (test fix for a netns-bound netlink socket) |
| 6f24b5afaf | Journal rows for two reds the conversion exposed (owned elsewhere) |
| 4ee1012c93 | Enter fails every child/startup error, including namespace denial; owner chose this policy |
| ad1c2c3884 | Inherited P3 dataplane conversion, independently rejudged |
| c76d6a29ed | Inherited P6 FIB conversion, retaining four reviewed privileged exceptions |
| 409b985e1e | Error-queue producer repair: retain offender alongside packet-info ancillary data; reject truncated control messages |
| 2a54edc280 | Require built-in GACT in runtime fragment, manifest and compiled floor; old-floor red/fixed green regression |
| a2e7df256c | Remaining P3 engine, oversized-DF and RFC4301 conversions; capture paths stay privileged |
| b00e3c6710 | Remaining P6: 28 conversions, six fresh FIB/zero-rate records and reviewed privileged exceptions |


Approval rows are keyed by the session that recorded them
(`tmp/commit-*-approved-<session>.md`). A new session's `./le commit create`
may not read session 99a74e76's rows: check how the gate reads them
(`docs/contributing/committing.md`, `internal/le/commit/rfcchange.go`) and, if it
does not, have a judge re-record the same verdicts in the new session. Never
self-approve.

## Continuation verification

- Remaining P3: both engine paths passed under race; oversized DF passed with
  its offender assertion unchanged after `409b985e1e`. RFC4301's clean run
  passed and the native direction mutation failed its forbidden-FWD assertion.
  Capture prerequisites skipped; no privileged capture proof is claimed.
- Remaining P6: 25 converted tests passed, including the real flowexport
  daemon. Three failed ENOENT: `TestApplyAcceptsAFlowOffloadToATableFlowtable`,
  `TestNetlinkIntegration_IngressPolicerReachesTheKernel`, and
  `TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone`.
- Seven new/retaken records (one IKE, six FIB/zero-rate) are fresh. The
  per-requirement check also reports four unproven ISIS claims in pending P7;
  those are not discharged by FIB's proof.
- Logs: current scratch `job-userns-remaining-plain-cdf2a222.log`,
  `job-userns-errqueue-fixed-f42fc6eb.log`,
  `job-userns-flowexport-producer-1f800cfb.log`, and
  `job-userns-flowspec-diagnostic-03ec1f4f.log`.
- The disposable flowexport executable was unused and removed, recovering
  52 MiB. Keep logs, immutable review inputs and mutation reports until spec
  closure; no local service was started by this continuation.

The scoped 12-package lint run failed with 17 distinct findings, all outside
this continuation's edited files. Linux ordinary and integration reported the
same 17; FreeBSD repeated the misspelling. Darwin, Linux arm64/riscv64,
capability, appliance and compile-out rows reported zero. No rerun or unrelated
source edit was made.

| File | Findings |
|---|---|
| `internal/plugins/traffic/netlink/tbf_linux_test.go` | unchecked type assertions at 24, 49, 72, 79, 80, 143 |
| `internal/component/ike/engine/esp_fixed_test.go` | defer-in-loop at 89, 94, 103, 137, 142 |
| `internal/appliance/cache_download_test.go` | misspelling at 16 |
| `internal/component/ike/engine/child.go` | modernization at 214, 680 |
| `internal/component/ike/engine/rfc3948_inner_selector_ordered_transport_test.go` | unused parameter/result at 25, 190 |
| `internal/component/ike/engine/rfc3948_inner_selector_transport_test.go` | constant parameter at 45 |

## Judged, waiting to be applied

| Batch | Units | Proposal | Records after applying |
|---|---|---|---|
| P5 | 16 approved (dhcp 10, netlink tunnel_rfc 4, ra rfc4861 2) | `scratch/userns-P5/proposed-judge-edits.md`; overlay in the old session's scratchpad `overlay/overlay.json` | re-take 26 (list in `scratch/userns-spec-edits.md`) |
| P7 | 26 approved, 1 refused | `scratch/userns-P7/proposed-judge-edits.md`, `judge-overlay/`, `judge-overlay-isis-ospf/` | re-take 116, create 17 (`judge-overlay/records.md`) |

P7 specifics: `ospf/transport/raw_probe_integration_linux_test.go` must drop its
`integration` tag in the same change (`rawSocketAvailable`), or the untagged
build breaks. `transport.TestOSPFUnnumberedSourcesFromDonorAddress` was refused:
in the fresh netns the unnumbered end has no default route and loose RPF drops
the reply. Fix the lab (route the peer /32, probe in
`judge-overlay-isis-ospf/variant/`), update its stale comment, send to a fresh judge.

Until P5 and P7 land, unprivileged `-tags integration` runs fail (not skip) for
`iface/netlink/tunnel_rfc`, `iface/ra/rfc4861`, the rsvpte PathCarriage and
AvoidsDataFEC units, and the vrrp tagged units. Root (QEMU) runs are unaffected.

## Also owed

- A judge confirms P4's hand-written RFC9915-18.2.12-2 negative break
  (`scratch/userns-P4/linkmoved-mutant.json`, same change as the old record, line moved 137 -> 144).
- Flowexport's explicit-daemon path has now passed through Enter. RSVP
  `rsvpte/producer_integration_linux_test.go` (`-rsvp-producer-daemon`) remains
  to be converted.
- P8 services batch: not started (inventory rows in the spec).
- P10: docs (`docs/contributing/testing.md`, `qemu-integration.md` userns section,
  `developer-setup.md`, `ci-workflows.md` line 37), apply every line in
  `scratch/userns-spec-edits.md` to the spec (inventory additions, reclassified
  rows, evidence, AC annotations), review rounds, closure.

## Runtime prerequisites and privileged exceptions

The spec's Reclassified table is current for P3/P6. It distinguishes the
untraced SRv6 timeout from source-backed restrictions; do not generalize either
to all kernel tests.

- IKE capture: `SO_RCVBUFFORCE` needs initial-userns `CAP_NET_ADMIN`. Both
  original privileged capture paths, 8 MiB buffers and packet counts remain.
- Selected FlowSpec DSCP: Linux 7.2.9 `nft_payload_set_init` rejects a netns
  owned outside the initial user namespace. The original test and all four
  claims/assertions remain. Zero-rate discard is converted and proven.
- Trafficusage: Enter-only experiment stopped at memlock EPERM, before TCX.
  Original tests remain; no accounting proof is claimed.
- Running host disables `NF_FLOW_TABLE`, `NET_ACT_POLICE`, `NET_ACT_GACT`.
  The unchanged flowtable and two ingress tests fail ENOENT. All three are now
  required by the source runtime profile, but build/boot and runtime proof
  remain owed. Do not turn these failures into skips.
- Earlier VLAN QoS `NF_TABLES_NETDEV` debt remains owned by
  `spec-kernel-capability-gate`; XFRM wildcard-owner fixture overlap remains
  owned by `spec-ike-eap-one-off-defects`.

The GACT regression passed all four emitted-config states under race, and the
old-floor overlay failed the three rejecting states. This does not prove a
built or booted kernel. Only the owner installs/reboots the shared host.

## Rules that bit during this work

- `Enter` must be the first statement of each top-level test; no `t.Parallel`.
- Editing a committed RFC-tagged unit is locked until a ze-test-judge runs
  `./le rfc approve`; batch agents write proposals, judges approve, then apply.
- Never leave the shared tree uncompilable between tool rounds; other sessions build from it.
- The cache disk (`/home/thomas/.cache`) filled twice; `./le scratch trim` and
  `./le scratch clean` are the only safe actions; anything else is Thomas's call.
- `./le verify changed` is owed to Thomas over every commit above.
