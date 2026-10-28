# Handover: privilege-skipping tests run unprivileged in a user namespace

Spec: `plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`
(status in-progress). Claim it with
`./le spec claim plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`.
Thomas ordered this work ("convert A and B", plus the `/dev/ppp` setup step).
Ask Thomas before starting any agent. The final request for four diagnostic
agents and one kernel-proof investigator was cancelled; it grants no permission.

Branch: `main`. The spec remains open. The handover commit does not land the
P5/P7 implementation or its generated records.

| Blocker | Next action |
|---|---|
| Native RSVP reload and shutdown stall | Trace removal of the deliberately rejected tunnel; preserve all assertions and timeouts |
| VRRP ND sometimes fails before mutation | Diagnose the missing virtual-MAC advertisement before resuming those records |
| Nine RFC792 kernel-produced claims lack discrimination | Find a supported kernel-producer proof mechanism; do not mutate fixture inputs or remove claims |

At handoff, `./le spec current` reported the unrelated
`spec-homepage-first-interaction.md`. Use the explicit claim command above in the
next session; do not infer this work's ownership from the current selector.

Evidence and per-batch handoffs live in the old session's directory
`tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/`:
`state/session-state-spec-privileged-tests-run-in-a-user-namespace-f3ae7c00-09af-4420-914f-39f2f7169980.md`
(batch handoffs) and `scratch/userns-spec-edits.md` (every spec edit owed,
collected while the spec was frozen for parallel agents). Read both first.

Continuation evidence is in
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/`,
`state/session-state-privileged-tests-run-in-a-user-namespace-644a4e78-0122-4eeb-8642-eb50638a1641.md`.
It supersedes the old P3/P5/P6/P7 proposals. Keep the old files for the P10
reconciliation list, but do not reapply their already-completed edits.

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
- P4's inherited link-move mutation is independently confirmed: current
  `linkMoved:144` still changes `!w.linkDown` to `false`; old `rec4.log` records
  the unwanted REBIND failure. All four records are fresh. This proves Ze's
  repeated-up behavior, not an RFC-prescribed attachment-detection algorithm.
- Remaining P6: 25 converted tests passed, including the real flowexport
  daemon. Three failed ENOENT: `TestApplyAcceptsAFlowOffloadToATableFlowtable`,
  `TestNetlinkIntegration_IngressPolicerReachesTheKernel`, and
  `TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone`.
- Seven new/retaken records (one IKE, six FIB/zero-rate) are fresh. The
  per-requirement check also reports four unproven ISIS adjacency/SPF claims
  outside this spec's namespace inventory. FIB's proof does not discharge them;
  do not fold that separate conformance work into P7's transport conversion.
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

## Applied but uncommitted: P5 and P7

Both batches received fresh independent judgment. Their immutable current
inputs are under continuation scratch `userns-next-P5/` and `userns-next-P7/`.
Each `sources.json` lists every source path, before/after hash and patch.
`units.json`, `claims.json`, `dependencies.json` and `run-plan.json` carry the
unit inventory and exact commands. Do not redo that exploration.

| Batch | Applied source | Observed runtime | Native record writes |
|---|---|---|---|
| P5 | Five files, 16 tagged units: DHCP 10, tunnels 4, RA 2 | All 16 passed under race, no skips | All 26 written: 25 producer-dependency halts and one semantic JoinGroup omission |
| P7 | 23 files, 27 tagged units and complete helper/caller closure | All 47 ordinary selected tests passed under race, no skips; separate native RSVP carrier failed | 49 written; 76 executable routes remain after baseline repair; nine further kernel rows have no admissible route |

P7's current total is 134 claim/polarity rows, not the old 133. The donor-address
fixture is approved and applied: only the link-scope peer `/32` route was added;
donor/borrower addresses and all source/interface assertions remain. Both OSPF
and VRRP raw-probe helpers now build under `linux`. The native RSVP producer
has Enter but retains `integration` and its explicit daemon flag. Its observer
must remain in the daemon namespace and must NOT call Enter.

The 28 source files, these four pages, and the spec were still uncommitted at
handoff:
- `docs/architecture/ospf/ospf-3-ip-transport.md`
- `docs/guide/rsvp-te.md`
- `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`
- `docs/architecture/diagnostics/active-probes.md`

The native writers also changed `rfc/discrimination/`. Select only the owned
records when committing; the checkout and approval ledgers are shared.
Judgments are in `tmp/commit-rfc-approved-0cf1e0e8.md` and
`tmp/commit-weakening-approved-0cf1e0e8.md`. They include unrelated approvals:
preserve those rows. P5's 16 approvals and P7's 27 approvals authorize the exact
reviewed conversions, not a future assertion change.

### Runtime evidence

All following logs are in continuation scratch:
- P5: `job-p5-dhcp-remaining-5a6420fb.log`,
  `job-p5-netlink-remaining-233f93db.log`, `job-p5-ra-remaining-aaed6e6d.log`.
- P7 ordinary: `job-p7-smoke-1-a86e04dc.log` (ISIS),
  `job-p7-smoke-2-8229bd74.log` (OSPF),
  `job-p7-smoke-3-6b4d8097.log` (OSPF transport),
  `job-p7-smoke-4-d680e6fd.log` (RSVP),
  `job-p7-smoke-5-e4d50a8d.log` (VRRP),
  `job-p7-smoke-6-fafa75a3.log` (VRRP transport).

P5's scoped lint attempt produced no plan or verdict because inputs changed
during planning. No P5/P7 lint success is claimed. Formatters ran only on the
five P5 and 23 P7 files. No new verification run was made for this handover.

### Native RSVP failure

`job-p7-smoke-7-213d481d.log` fails at
`internal/plugins/rsvpte/producer_integration_linux_test.go:189`,
`h.reload(t, h.configuration(t, ""))`. The three daemons and OSPF adjacencies
started. The intentionally occupied egress label caused the expected EEXIST
rejection; that is a control, not the defect. The second SIGHUP was received,
but reload never completed. Head-end shutdown also stalled after cancellation.
No root cause or source fix is established. Do not rerun merely to confirm it.

Read `docs/guide/rsvp-te.md` "Native Linux carrier" and
`docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` lifecycle/concurrency
sections before tracing `OnConfigApply`, tunnel reconciliation and teardown.
The page's ordered shutdown and lock-order contract are the investigation
boundary, not evidence that this failing path follows it.

The supplied executable is `scratch/userns-rsvp/ze` in the continuation
directory. It was built with `ze_core ze_rsvpte ze_ospf ze_ssh`; build log:
`job-userns-rsvp-daemon-build-3e09e13b.log`. It is 59,480,330 bytes and had no
`fuser` users at handoff. Retain it as the exact failing artifact until diagnosis
and the rebuilt comparison complete, then remove it. Do not replace it with an
unrelated cached daemon. No service started for this work remains active; other
sessions' browser/preview services were left untouched.

### VRRP clean-baseline failure

Native recording later failed twice before applying a break:
`job-p7-record-rfc5798-002-9d642d58.log` and
`job-p7-record-rfc9568-004-fa8fa0e7.log`. Both fail
`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly` at
`internal/plugins/vrrp/nd_nonowner_integration_linux_test.go:93`:
`macvlan ND: no answer carried the virtual MAC 00:00:5e:00:02:15 (got [])`.
The earlier race smoke and several clean recording runs passed. This is not
mutation evidence and is not yet attributed to timing, kernel setup or capture.

Read `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`, especially the IPv6
macvlan recipe, then trace `ownerAwaitUsable`, `ownerResolveND` and the real
producer. Preserve the physical-MAC control, virtual-MAC-only check and
withdrawal/no-answer check. Do not add a sleep, retry or weaker assertion to
hide the failure.

### Exact record continuation

Current `userns-next-P7/record-resume-after-baseline-fix.json` contains the 76
remaining executable argv arrays. Run them only after repairing the clean
baseline. Do not replay all 125 commands from the original plan.

| RFC | Successful commands already recorded |
|---|---|
| 1195 | 1 |
| 4302 | 4 |
| 2328 | 4 |
| 2205 | 7, including the revised transit mutant |
| 9568 | First three of its serial chain |
| 5798 | First one of its serial chain |
| 3768 | All 21 |
| 792 | Eight existing echo rows; not the nine missing kernel rows |

The 49 writes were reconciled against native writer confirmations in
`job-p7-record-*.log`. If a fix changes a producer or tagged unit, re-record the
affected successful rows too; otherwise leave them alone. Reconcile freshness
per ID before landing, without a whole-tree gate.

`mutations/rsvp-no-transit-interception.json` remains refused: setting
IP_ROUTER_ALERT to zero on a fresh socket fails setup with ENOBUFS.
The independently approved replacement is
`mutations/rsvp-no-transit-interception-v2.json`, selector
`internal/plugins/rsvpte/transport_linux.go:83:12#1`, omitting only registration.
`job-p7-record-rsvp-transit-v2-0b55eaeb.log` proves clean PASS then the intended
receiver failure at test line541, and wrote RFC2205-3-39 positive.

Nine first RFC792 rows remain in `userns-next-P7/kernel-proof-rows.json`.
Their three gateway tests passed, but the producing code is Linux. The existing
Go mutation routes do not supply the missing kernel-producer proof.
The INTEROP-only foreign-producer escape does not apply. Keep every claim and
assertion. Do not call these rows namespace-infeasible or substitute AddRoute
dependency halts for their missing semantic proof.

## Next five actions

1. Claim this spec explicitly and ask before any agent. Diagnose the RSVP
   removal/shutdown failure above; no justified source patch exists yet.
2. Diagnose the VRRP ND baseline failure above, then obtain independent judgment
   for any test change. Fix the producing defect, not the observed symptom.
3. Resolve the nine kernel-proof rows through an admissible mechanism. No new
   recorder implementation, kernel build or host reboot was authorized by the
   cancelled agent question.
4. Resume only the outstanding records and any records made stale by fixes.
   Finish bounded lint/freshness checks, then land P5 and P7 separately through
   `./le commit create`. P5 need not wait for unrelated P7 repairs.
5. Phase 2: request agents for P8 services (not started), then P10 docs and
   closure. Reconcile old `scratch/userns-spec-edits.md` with the current spec,
   preserving D-9's owner-approved fail-all-startup-errors policy. Shared pages
   still owed: `docs/contributing/testing.md`, `qemu-integration.md`,
   `developer-setup.md`, and `ci-workflows.md`. Keep the spec open until its
   acceptance criteria and runtime-kernel prerequisites have evidence.

### Commands after a source fix

Use the documented default `./le`: it runs committed tooling while selected
tests use current sources. `--name session` rebuilds the whole working-tree
launcher and previously hit other sessions' SDK/BGP compile edits. Do not bypass
the launcher with a binary path. Set the new session's identity, not this one's.

For RSVP, build the corrected daemon to a NEW absolute scratch path first:

```sh
./le job run label userns-rsvp-fixed-build quiet command go build \
  -tags 'ze_core ze_rsvpte ze_ospf ze_ssh' -o "$daemon" ./cmd/ze
CGO_ENABLED=1 ./le job run label userns-rsvp-fixed quiet command \
  go test -race -count=1 -v -timeout=4m -tags integration \
  ./internal/plugins/rsvpte -run '^TestRSVPNativeProducer$' \
  -args "-rsvp-producer-daemon=$daemon"
```

For the ND clean baseline, retain the native recorder's feature-tag selection
from its failed log when checking the repaired path. The focused race command is:

```sh
CGO_ENABLED=1 ./le job run label userns-vrrp-nd-fixed quiet command \
  go test -race -count=1 -v -timeout=4m ./internal/plugins/vrrp \
  -run '^TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly$'
```

Then use the exact remaining native argv arrays to prove clean and broken runs.
Allow a 600-second outer command deadline for each focused smoke, including
admission wait. No full verification, full suite or performance run on this host.

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
- Remaining post-commit verification is scoped to changed components through
  `./le verify changed`. Documentation-only commits owe no test run.
