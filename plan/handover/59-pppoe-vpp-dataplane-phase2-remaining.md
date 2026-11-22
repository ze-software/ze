# Handover 59: PPPoE VPP data plane, what phase 2 still owes

Written 2026-11-22 on the server (`claude`), main checkout, by the session
that landed handover 58. Nothing from that session is pushed.

The owning spec is `plan/post-release/features/spec-pppoe-vpp-dataplane.md`
(in-progress, phase 2/7, no live claim). Claim it before editing:
`./le spec claim spec-pppoe-vpp-dataplane.md`. Its "Resume checkpoint" section
(steps 1 to 5) is the plan; this file adds today's state and the first run.

## Landed on 2026-11-22

| Commit | What |
|--------|------|
| `0c93a81fd2` | A late PPP settlement keeps its causes through close (resume step 1, done) |
| `1a92721b86` | The PPP unit liveness probe retries EINTR; a renegotiating CPE session no longer drops under load |
| `bc776b7a2f` | Five netlink test-fixture defects; every CPE and native access test now runs and passes |
| `02319e2f91` | The runtime kernel builds in `CONFIG_NF_TABLES_NETDEV` |
| `7f916f32ea` | Spec rows record the above |

Today's baseline: the whole `./internal/plugins/iface/netlink/` integration
package passes on this host (`-ppp-provider-required=true`, race), except the
two VLAN QoS tests that need the new kernel. Those pass in a guest on it. The
host still boots the old kernel until Thomas approves
`./le setup docker kernel install` and a reboot.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Baseline of the PPP kernel package | Not run since the 2026-11-19 checkpoint. The spec's last population (112 pass, 1 fail) named `TestCreatedUnitNativeRetirementJoinsRetainedOriginalDuplicate` (`created_unit_retirement_integration_linux_test.go`), failing with "original native mutation authority is unavailable" | `go test -race -count=1 -v -tags integration ./internal/component/l2tp/ppp/kernel/ -args -ppp-owner-guard-required=true -ppp-control-socket-required=true` through `./le job run`, then read every FAIL and SKIP. A refusal naming a missing kernel ABI means this host's kernel lacks it: rerun in a `./le test qemu docker-lab` guest on the current runtime tree (`docs/architecture/testing/qemu-integration.md`, "One command in Ze's own system") |
| 2 | Retained-duplicate failure and false-release projection (resume step 2) | The spec says "the other PPP implementation session" owns `ppp/kernel/session_release.go` and this failure. No live claim was seen on 2026-11-22 | Check with `/ze-status` whether that owner is live. If not, it is this spec's work: a failed original unit-file Close must keep `NativeReleasePending` true after its writer and duplicate close |
| 3 | Retirement controls still owed ("Native validation still owed" row) | Five logical and three genuine retirement-projection controls, the corrected original-duplicate fixture, causal cuts | After item 1, map each to its test by name from the spec and the session-state record it cites; report which exist and which are unwritten |
| 4 | IPv6 carrier traffic (resume step 3) | `TestNativeIPv6CarrierOriginalTraffic` has never passed; its readiness fixture was corrected but not run | Compile `./internal/component/l2tp/pppoe/dataplane/kernel/` with `-tags integration` and run that test alone, verbosely |
| 5 | Fragment draft (resume step 4) | `internal/component/l2tp/pppoe/dataplane/kernel/testdata/ppp-ipv6-fragments-draft/`, outside compilation | Only after item 4 passes. Read its README and R2 companion first |
| 6 | Accounting, scoped lint, docs, review (resume step 5) | Open | Last |

## Traps met on 2026-11-22

- The spec pins a "matched kernel/image" pair for CONTROL6 tests
  (`cddff82e...`, image `f92054020556fb26`). The runtime kernel was rebuilt
  today (tree `6db1b4aa`, with the netdev option and another session's seg6
  patch 0015). Check whether the CONTROL6 roots need that old pair or run on
  the current tree before mixing them.
- Another session (SRv6) is editing `gokrazy/kernel/` and the FIB kernel
  plugin. Its half-written edits broke the `le` build once today; wait for it
  rather than touching its files.
- A test fixture that waits for a route with a nil destination never matches:
  netlink v1.3.1 decodes a default route as 0.0.0.0/0.
- An `ETH_P_ALL` packet socket on a fresh VLAN device receives IPv6 MLD/ND
  chatter; bind to the test frames' own ethertype.
- The first interface-name validation in a test process loads the YANG
  schema and can spend a short caller budget.
- Every test change that could lower what a test proves goes to an
  independent `ze-test-judge` agent before it is committed; the commit gate
  refuses a stale approval after any later edit to the same unit.
