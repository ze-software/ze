# Handover: privilege-skipping tests run unprivileged in a user namespace

Spec: `plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`
(status in-progress). Claim it with
`./le spec claim plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`.
Thomas allowed agents for this work ("as many as you need"), development on
the Mac with runs on the Linux host over `ssh claude`, and a commit as each
fix or batch lands.

Every conversion batch has landed (P1 to P9, P11). The implementation work is
done. The spec stays open only on the owner decisions below, then `/ze-review`
and `/ze-close`. The previous version of this file (P5/P7 blockers) is in git
history; every blocker it named is fixed or explained.

## State on 2026-10-30

Thomas decided the four questions below on 2026-10-29. They are done except
the reboot:

| Decision | Outcome |
|---|---|
| AC-8 kernel features | Tests run on Ze's kernel (META rule). Ze's build carries all five options; the host ran a 2026-10-14 build. A current runtime kernel is built in `~/.cache/ze/mac-check-kernel` on the Linux host; the install replaces the running release's modules, so it runs immediately before the reboot: `cd ~/.cache/ze/mac-check-kernel && ./le setup docker kernel install confirm 7.2.9-ze && sudo reboot`, at a moment Thomas picks. Then rerun the three tests (323bb6fd26 makes them name the missing option). Ze loading modules on vendor kernels is its own spec, `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` (ready), for a fresh session |
| AC-12, AC-13, AC-15 | Rewritten to the landed P2 design (323bb6fd26) |
| RFC 792 kernel rows | (a): seven rows are kernel-producer escapes (54fb01a2e6, 7f1a79f1cc); route (b) is `plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md` |
| VRRP ND window | Closed in the product (0dc36bd960). The boot-announce defect it exposed is a journal row owned by `spec-vrrp-owner-arp-rfc-defects` (03833b3160) |

Left for closure: the reboot and the AC-8 rerun; AC-10 and AC-11 whole-tree
checks (Thomas runs them); a judge for the RFC-tag comments in
`internal/plugins/vrrp/instance_test.go` that still name the old location of
`doInstallVIPs`; then `/ze-review` and `/ze-close`.

## Waiting on Thomas (2026-10-29, now answered)

| Decision | What it unblocks |
|---|---|
| Build and boot a host kernel with `NF_FLOW_TABLE`, `NET_ACT_POLICE`, `NET_ACT_GACT` (the runtime profile already requires them) | AC-8: three P6 tests fail ENOENT on the running kernel. After the boot, run them once |
| AC-12, AC-13, AC-15 describe the original `/dev/ppp` design; the landed P2 (55cb1821dd) reports `missing` and runs `modprobe ppp_generic`, writes a commented udev rule with `static_node=ppp`, and persists through the static node, not a `modules-load.d` drop-in (D-10). Rewrite the ACs to the landed design, or change the code | AC-12, AC-13, AC-15 |
| Seven RFC 792 rows on `rfc792_gateway_icmp_integration_linux_test.go` are produced by the kernel from the packet alone, so no Go break discriminates them (Format-1 on all three gateway tests, TimeExceeded-1 and ParamProblem-1 both polarities; D-12). Choose: (a) a `kernel-producer` escape reason (publishes as escaped), (b) a patched-kernel proof route, (c) leave them unproven | AC-11 for RFC 792 |
| Close the window where shipped VRRP can miss a multicast ND solicitation after becoming master (macvlan rx filter installs on a work queue; RFC 9568 membership is met; one no-op RTM_NEWLINK after installVIPs closes it) | Optional hardening |

## Gates for closure that are Thomas's to run

AC-10 wants `./le test unit <group>` to reach every inventory row, and AC-11 a
full `./le rfc check`. Both are whole-tree checks.

## Landed in the 2026-10-29 session

| Commit | What |
|---|---|
| b8c4909334 | RFC 792 DF carrier installs its MTU through Ze's SetMTU; Unreachable-1 discriminated by hand-written SetMTU mutants (Format-1 mtu+1 refused by the judge) |
| c3eb71d64e | Plugins forked from a thread that lives as long as ze: Pdeathsig is bound to the forking thread, and kernelcap probes destroy their locked thread, which SIGKILLed the RSVP observer mid-reload |
| 227ecf2200 | VRRP ND tests sync the macvlan rx filter before probing (clean-baseline failure 8/25 was a harness race); 22 records re-taken |
| 18644fc76a | State-store reads never wait for a writer (OSPF startup GR read died behind the boot record) |
| 5b567e3540 | The remaining 54 VRRP records of the P7 continuation |
| c3112d00f6 | RSVP native carrier keeps daemon stores on tmpfs: the "reload stall" was host ext4 fsync latency (up to 15 s), not Ze |
| 68b723ec4f | P8 services batch, 15 tagged units judged, 15 records re-taken |
| e32e1868f3 | P10 pages (testing.md, qemu-integration.md, developer-setup.md, ci-workflows.md) and spec reconciliation (D-10..D-12) |
| e8abdcbcab | Store writes split into configuration and runtime-state key spaces: `request ospf graceful-restart` no longer fails behind the boot record |
| 7d11a51269 | P11: the three files AC-9's grep found plus the RFC 2385 egress file; RFC 4659 fixture gains the RFC 8212 export policy |

## Known reds, not this spec's

- `TestRecoveryQueuedDeliveryTransport` (plugin/process) times out identically at
  1b1a96660f; journal row in `feature-test-missing-build-tag.md`.
- Scoped lint: four findings in `cmd/ze/hub/main_reload.go` and
  `main_reload_test.go` (ff2aac12b0); nine in other reactor files.
- `./le doc check all`: dns resolver command paths, two flow-export draft `.ci`
  anchors.
- The commit gate compares tagged bodies only, so a helper change reached by a
  tagged unit carries no `RFC-approved:` trailer
  (`plan/journal/check-cannot-see-the-change-it-looks-for.md`, open).

## Working on the Linux host

- Go is at `/usr/lib/go-1.27/bin`; a non-interactive `ssh claude` does not set it.
- Never write into `~/Code/.../ze/main` on Linux (another session's checkout).
  Make a mirror: `git clone --shared` of `~/.cache/ze/mac-check`, fetch a git
  bundle of the Mac's commits, check out detached, copy uncommitted files.
- Full feature tags: `. ./feature-tags && tags=ze_core,ze_distro,$(feature_tags feature-gates.txt ",")`.
- The root disk is busy and 86% full; fsync stalls of 10 s or more occur.
