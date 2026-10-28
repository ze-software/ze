# Handover: privilege-skipping tests run unprivileged in a user namespace

Spec: `plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`
(efa14b976e, status in-progress). Claim it with
`./le spec claim plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`.
Thomas ordered this work ("convert A and B", plus the `/dev/ppp` setup step).
Session rule from Thomas for the session that wrote this: ask before starting
any agent. Assume it still holds and ask him at the start.

Evidence and per-batch handoffs live in the old session's directory
`tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/`:
`state/session-state-spec-privileged-tests-run-in-a-user-namespace-f3ae7c00-09af-4420-914f-39f2f7169980.md`
(batch handoffs) and `scratch/userns-spec-edits.md` (every spec edit owed,
collected while the spec was frozen for parallel agents). Read both first.

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

## Uncommitted in the shared tree (do these first)

1. **P3 ike, 17 files in `internal/component/ike/dataplane/`.** Compile and lint
   clean, 47/48 PASS (TestXFRMReadbackWildcardOwners is a known red, journalled,
   owned by spec-ike-eap-one-off-defects). Patch backup:
   `scratch/userns-P3/dataplane-conversion.patch`. The commit gate wants four
   weakening approvals. Recorded already (session 99a74e76's file
   `tmp/commit-weakening-approved-99a74e76.md`): `dataplane.skipWithoutPolicyPermission`,
   `dataplane.skipUnprivileged`. Still owed, now that 2bdc5103d8 carries the guard:
   a ze-test-judge confirms and records `dataplane.encapOwnProcess` and
   `dataplane.encap_dualform_integration_linux_test` (verdict text in
   `scratch/userns-spec-edits.md`, replace `<sha>` with 2bdc5103d8).
2. **P6 fib, 15 files in `internal/plugins/fib/kernel/`.** 108 PASS, 4 named
   SKIP (`skipWithoutInitNamespacePrivilege`), 1 known red (rfc1195_onlink, tagged,
   not converted: name it in the body). Weakening approval
   `kernel.mplsmtu_integration_linux_test` is recorded in session 99a74e76's file.

Approval rows are keyed by the session that recorded them
(`tmp/commit-*-approved-<session>.md`). A new session's `./le commit create`
may not read session 99a74e76's rows: check how the gate reads them
(`docs/contributing/committing.md`, `internal/le/commit/rfcchange.go`) and, if it
does not, have a judge re-record the same verdicts in the new session. Never
self-approve.

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
- P3 remainder: `engine/child_policy_delete_integration_linux_test.go`,
  `engine/rfc3948_encap_port_integration_linux_test.go` (check the RFC file-naming
  rule), `transport/udp_df_integration_linux_test.go`; proposal for the tagged
  `dataplane.TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath` (RFC4301-7.4-2).
- P6 remainder: copp, policyroute, firewall/nft (counter, integration),
  traffic/netlink (cs6, integration), flowexport/conntrack, trafficusage; proposals
  for fib `rfc1195_onlink` and flowspec-firewall `rfc8955_packet_zero`,
  `rfc8955_selected`.
- Now convertible after 2bdc5103d8 (flags forwarded): `flowexport/producer_integration_linux_test.go`
  (`-flowexport-ze-path`), `rsvpte/producer_integration_linux_test.go` (`-rsvp-producer-daemon`).
- P8 services batch: not started (inventory rows in the spec).
- P10: docs (`docs/contributing/testing.md`, `qemu-integration.md` userns section,
  `developer-setup.md`, `ci-workflows.md` line 37), apply every line in
  `scratch/userns-spec-edits.md` to the spec (inventory additions, reclassified
  rows, evidence, AC annotations), review rounds, closure.

## Reclassified (need more than Enter; keep their skips, rows owed in the spec)

fib mplsipmtuprobe, mplstransitmtu, mplsmtu FragmentProgress (kernelcap probe binds
/proc/sys: EINVAL in a user namespace); fib srv6 (no SRv6 packet within 3s, cause
untraced); iface/netlink vlanqoslab EgressPCPOnWire, DSCPFullChain (kernel lacks
`CONFIG_NF_TABLES_NETDEV`, journalled, owned by spec-kernel-capability-gate).

## Rules that bit during this work

- `Enter` must be the first statement of each top-level test; no `t.Parallel`.
- Editing a committed RFC-tagged unit is locked until a ze-test-judge runs
  `./le rfc approve`; batch agents write proposals, judges approve, then apply.
- Never leave the shared tree uncompilable between tool rounds; other sessions build from it.
- The cache disk (`/home/thomas/.cache`) filled twice; `./le scratch trim` and
  `./le scratch clean` are the only safe actions; anything else is Thomas's call.
- `./le verify changed` is owed to Thomas over every commit above.
