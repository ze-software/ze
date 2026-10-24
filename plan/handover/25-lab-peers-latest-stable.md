# 25 Lab peers on latest stable: the runs and fixes still owed

Owner decision M14 (`ai/META-DECISIONS.md`, 2026-10-22): every third-party
peer the interop and integration labs run is on its latest stable upstream
release. Handover 20 (`plan/handover/20-gobgp-v4.md`) did the version bumps
and the GoBGP v4 work. This file carries what is left: proving the bumped
peers in the lab and fixing what the runs show. The owner split it out on
2026-10-24 so that it can run elsewhere.

## Already done (do not redo)

The bumps are committed. Handover 20's "Progress" sections hold the commits
and their proofs.

| Peer | Version |
|---|---|
| FRR | 10.7.1 everywhere, declared once as `interoplab.FRRImage` |
| BIRD | 3.4.0, built on debian:trixie-slim (the Alpine build dies with SIGILL on route import) |
| GoBGP | v4.10.0 |
| OpenBGPD | 9.3 |
| strongSwan | 6.1.0 |
| Kea | 3.2.1 |
| keepalived | 2.4.3 |
| dnsmasq | 2.93 |
| FreeRADIUS | 3.2.10 |
| ppp / xl2tpd | 2.5.4 / 1.3.20 |
| accel-ppp | 1.14.0 |
| ExaBGP | 5.0.15 |
| pmacct | v1.8.0 |
| StayRTR | v0.6.4 |
| rustbgpd / RustyBGP | v0.79.0 / v0.2.0 |
| Base images | alpine 3.24.3, go 1.27, debian trixie |

The one-month source-freshness rule for tac_plus-ng and freeRtr is in
`internal/le/interoplab/sourcepin.go`. `./le lab pin check` reports pins
that are behind upstream.

Proven green, so these need no rerun:

- **GoBGP:** all 32 scenarios that start a GoBGP peer. That set includes
  bgp-ecmp-frr, bgp-remove-private-as-frr, bgp-nexthop-self-local-auto-frr,
  ospfv3-redist-frr, ospfv3-nssa-redist-frr and the pmacct/speaker add-path
  pair.
- **BIRD:** every `*-bird` scenario, plus bgp-vpn-frr,
  bgp-policy-import-export-frr, bgp-route-server-frr,
  local-as-replace-as-partition and rtr-stayrtr. Each ran with the full BIRD 3
  conversion; see item 1 for what is not yet at HEAD.
- **Access, VPN and DHCP:** the passes listed in handover 20.

## How to run on the Mac

- **The lab daemon** is the `ze` Colima profile (Ze's docker-host image,
  kernel `7.2.9-ze`); the labs select it themselves. Never restart, stop or
  recreate it while another session's lab runs on it. Never run a host-wide
  Docker prune (`.claude/rules/docker.md`).
- **One scenario at a time**, under `flock tmp/interop-lab.lock`. Run
  `./le --update` first, because a stale `bin/le` carries old checkers.
- **The shared tree often does not compile** because of other sessions'
  uncommitted Go. Run from a `git clone --shared` of HEAD, with your own files
  copied in, and refresh the clone's `bin/le`.
- **Delete each clone the moment its run ends.** The disk was full twice on
  2026-10-23.
- **Never make a temporary break** (for a revert-red proof) in the shared
  tree. Use the clone or `go test -overlay`.
- **The per-suite scenario selector variables** are listed in
  `docs/architecture/testing/interop.md`, for example `INTEROP_SCENARIO`,
  `IPSEC_INTEROP_SCENARIO` and `ZE_PPPOE_INTEROP_SCENARIO`.

## Owed

| # | Item | Detail |
|---|------|--------|
| 1 | BIRD 3 hunks held out of HEAD | f47e651ab8 landed 13 of 16 files. The BIRD 3 hunks in `internal/le/interoplab/bgp/names.go` (`rfc7705BIRDASPath`), `check_special.go` (community, large community) and `check_extras.go` (local_pref, med) are in the working tree, as is the BIRD 3 paragraph of `docs/architecture/testing/interop.md`. Each of those files also holds session 6b51c217's uncommitted addpath-zero-id-frr / ldp-init-unknown-tlv work, which names files not in HEAD, and `interop.md` holds foreign RADIUS and LDP rows. `check_extras.go` also holds the ospfv3-nssa-redist-frr checker fix (`show ipv6 ospf6 database type-7 adv-router 172.30.0.2`, proven green and red in handover 20). Land them when those files are committed, or when their holder is gone (`ai/rules/git-safety.md`). Until then bgp-as-migration-local-as, bgp-policy-import-export-frr, ospfv3-nssa-redist-frr and the package's BIRD unit tests are red at HEAD. Rerun the three scenarios at HEAD once the hunks land |
| 2 | FRR 10.7.1 reds, not diagnosed | Seen on 2026-10-22 on the Linux host: as112-origin-as, as112-redistribute-origin-custom, 4byte-asn, exabgp-bridge-vpnv4, holdtime-deadpeer, ipv6-ebgp, labeled-withdraw-compatibility, linklocal-only-multihop-withdraw, max-prefix-cease. The run was from the shared tree while other sessions' work was in flight, so first rerun each one from a clean clone, then classify it (FRR drift, a stale scenario, a Ze defect, infrastructure) and fix it at its cause |
| 3 | FRR 10.7.1 scenarios not run | Derive the set from `scenarioPeers` (`internal/le/interoplab/bgp/prepare.go`): every scenario with an FRR peer, minus those already green above. About 40 |
| 4 | IPsec on strongSwan 6.1.0 | Undiagnosed reds: esp-integrity-only-null, ike-padded-probe-strongswan, mobike-initiator, mtu-negotiated-transform, responder-ike-rekey, eap-tls13 (its `eap_id` fix was committed but never proven). The rest of the 50-scenario suite after eap-tls was never read. child-rekey-narrowing is fixed (34ad745067) |
| 5 | QEMU MOBIKE guest boot | `./le test qemu ipsec-mobike-test kernel <vmlinuz>`. The `console=ttyAMA0`-on-x86 cause was fixed in 2d3ffe8782, but no boot has proven it. On the Mac, use the arm64 kernel (M13) |
| 6 | Reruns that failed on infrastructure | 01-ppp-ipv4 (`docker-l2tp-ppp-test`), natt-tunnel-inner-checksum (after the rawprobe change), ipv6cp-zero-identifier. The last should now reach `errIPv6CPNeverEngaged`, the known IPv6CP gap that `spec-l2tp-ipv6-subscriber` owns |
| 7 | pmacct, ExaBGP, RSVP-TE | bmp-locrib-pmacct and bmp-statistics-pmacct; `./le test exabgp` for both ExaBGP pins; `RSVPTE_INTEROP_SCENARIO=transit-strict-hop-forwarded ./le test integration interop-rsvpte`, which never ran because the Linux daemon was too slow |
| 8 | Perf DUT smoke | `./le perf` for frr, bird, openbgpd, rustbgpd and rustybgp. Also build the freeRtr perf DUT (`test/interop/Dockerfile.freertr`), which needs a `../rtr` checkout beside the repository; neither the Mac nor the Linux host had one on 2026-10-23 |
| 9 | `interop.md` prose | Text that states FRR 10.3.1 behaviour (extended-message gating, OSPFv3 virtual-link, Type 5 ESI display, VPN-IPv6 link-local display) and the BIRD next-hop limitation row. Re-check each against 10.7.1 / 3.4.0 runs from items 2 and 3 |
| 10 | Pins still behind | `./le lab pin check` on 2026-10-24 reports 5 behind: `Dockerfile.gobgp` golang 1.25 and alpine 3.21; the freeRtr RSVP-TE source commit; the tac_plus-ng source commit; `frr_image_test.go` naming FRR 10.4.1. The five scenario `frr-image` files are kept (owner, 2026-10-24): they pin the same digest as `interoplab.FRRImage` so that an older `FRR_IMAGE` override cannot start them, and `./le lab pin check` reports them if they fall behind. It also reports `debian trixie-slim` as floating (pin it to 13.7 by digest) and two unresolved pins. One of those, `router1 10.0.0.1`, was the scanner reading its own `officialContext` doc comment, which holds both the word "Image" and the address it means to exclude. That false positive is fixed in 7bf3ea5149: Go image pins are read from code only, never from comments, and the check now reads 87 pins with `rpki/stayrtr 0.6` the one unresolved pin left |
| 11 | Two code checks | Whether the best-path replay (`collectBestPaths`, rib) drops the labels kept beside the record, as 2d08a20a55 found for AS_PATH (unverified, no row yet). And a `/ze-rfc-audit` rejudge of RFC3948-3.1.2-1: its note still names `nping --badsum`, which `test/interop-ipsec/rawprobe.c` replaced. Audit files are per requirement since dcc219072a (`rfc/audit/<stem>/<rid>.json`) |
| 12 | Linux host leftovers | Remove the `ze-m14-*` and `ze-m14b-*` check images by name. They belong to these agents and exist only on the Linux host |

Out of scope: the QEMU guests' strongSwan 6.0.7 and keepalived 2.3.4 trail
upstream because they are Alpine packages. M1 moves those guests to Ze's own
system, which closes that gap.

## Gates owed over handover 20's code

Handover 20 closes with these, not this file: the full verify (started
2026-10-24), the weakened-test check, and an independent review of the AIGP
fixes (690c96d6bf, d4373233df) and the IKE fix (34ad745067).
