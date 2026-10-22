# 20 GoBGP interop lab on v4.10.0, and the move of every lab peer to latest stable

Owner decision M14 (`ai/META-DECISIONS.md`, 2026-10-22): every third-party lab
peer runs its latest stable upstream release. The GoBGP lab ran v3.31.0 because
its first install named the v3 module path with `@latest` (2026-03-20), which
resolves to the newest v3 although v4.0.0 shipped 2025-11-03; the review pin
froze that.

## Landed with this handover

The GoBGP port: `test/interop/Dockerfile.gobgp` builds gobgpd, gobgp and the
`gobgp-paths` gRPC reader from `github.com/osrg/gobgp/v4` at v4.10.0
(`golang:1.25-alpine`, v4's go.mod floor). The FlowSpec checker reads the
sending peer from v4's `peer-address` JSON field (v3 named it `neighbor-ip`;
`pkg/apiutil/util.go`, `Path.PeerAddress`). Comments and docs that described
v3.31.0 behaviour now say what v4.10.0 does. Image built; `go vet` and the
`internal/le/interoplab/bgp` unit tests pass.

## Owed: scenario runs on v4.10.0 (not run, host overloaded, owner order)

Passed on v4.10.0: bgp-attr-flags-low-nibble-gobgp, bgp-as-migration-local-as,
bgp-addpath-readvertise-collision-frr.

Not yet run, each with `INTEROP_SCENARIO=<name> ./le test integration interop`
under `./le job run`, on a host that is not busy:

| Scenario | Why it matters on v4 |
|----------|----------------------|
| bgp-flowspec-gobgp, bgp-flowspec-sctp-gobgp | confirm the `peer-address` checker change |
| bgp-vpn-gobgp | v4 source no longer deduplicates MP capabilities by AFI; confirm and drop the limitation row in `docs/architecture/testing/interop.md` if so |
| bgp-sr-policy-malformed-receive-gobgp | v4 turns AFI/SAFI-disable into a session reset, so the step can pass even when Ze relays the malformed NLRI: re-run the revert-red proof |
| bgp-large-community-no-repeat-gobgp | v4 keeps repeated Large Communities on receipt; re-run the revert-red proof |
| bgp-exabgp-bridge-rtc-gobgp | v4 no longer panics on the 0-bit default membership (`processRTCMembership`, `HasDefaultRouteTarget`); the scenario could now also send it |
| bgp-addpath-best-path-pmacct, bgp-addpath-rail-agreement-speaker | setup timed out under load; cross-compile failed on another session's uncommitted bridge work |
| bgp-attribute-default-localpref-gobgp, bgp-cluster-list-length-bird, bgp-ebgp-gobgp, bgp-ecmp-frr, bgp-evpn-gobgp, bgp-exabgp-bridge-sr-policy-gobgp, bgp-ipv6-ebgp-gobgp, bgp-local-pref-strip-gobgp, bgp-med-across-as-gobgp, bgp-med-ibgp-post-selection-removal-gobgp, bgp-med-increment-gobgp, bgp-med-remove-configured-gobgp, bgp-multihop-ebgp-gobgp, bgp-mup-malformed-t1st-withdraw-gobgp, bgp-nexthop-self-local-auto-frr, bgp-remove-private-as-frr, bgp-role-gobgp, bgp-routes-gobgp, bgp-sr-policy-gobgp, ospfv3-nssa-redist-frr, ospfv3-redist-frr | plain regression on v4 |

## Owed: every other lab peer to latest stable (M14)

Inventory taken 2026-10-22. Alpine 3.21, the base of nearly every lab image,
reaches end of life 2026-11-01.

| Peer | Used | Target |
|------|------|--------|
| FRR | 10.3.1 default, 10.4.1 and 10.7.1 per scenario | 10.7.1 everywhere (default image string has five copies, plus comments and the interop.md daemon row) |
| BIRD | 2.15.1 (Alpine 3.21 `bird`) | BIRD 3 line only, 3.4.0 (owner: "BIRD 3") |
| OpenBGPD | 8.6 | 9.3 |
| Kea | 2.6.2 | 3.2.1 (3.x restricts config and socket paths) |
| keepalived | 2.3.1 | 2.4.3 |
| dnsmasq | 2.90 | 2.93 |
| strongSwan | 5.9.14 | 6.1.0 (lab and the QEMU mobike guest) |
| accel-ppp | 1.13.0 | 1.14.0 |
| ppp / xl2tpd | 2.5.1 / 1.3.18 | 2.5.4 / 1.3.20 |
| FreeRADIUS | 3.2.7 | 3.2.10 |
| rustbgpd | v0.7.0 | v0.79.0 (needs rust 1.95+) |
| rustybgp | unpinned master | pin v0.2.0 or newer |
| pmacct pmbmpd | `:latest` | pin v1.8.0 |
| VPP | `ligato/vpp-base:latest` | pin 26.06-release |
| StayRTR live test | `rpki/stayrtr:latest` | pin v0.6.4 |
| gnmic | 0.42.1 | v0.49.1 |
| ExaBGP | 5.0.13 | 5.0.15 |
| tac_plus-ng | third-party image, built 2025-11-11 | build ourselves from upstream master |
| freeRtr RSVP-TE | commit 6c295d8 (2026-09-23) | latest upstream; Ze patch must still apply |
| freeRtr perf DUT | local `../rtr` checkout | keep the checkout, refresh it when over a month old |
| Bases | alpine 3.21, golang 1.25/1.26, rust 1.88, debian bookworm | alpine 3.24.3, go 1.27, rust current, debian trixie |

Where Alpine 3.24 still lags upstream, build the peer from source. tac_plus-ng
and freeRtr (no stable releases): before a build, when the source is more than
a month old, update it to upstream latest first (M14), enforced in the image
build. Add a check that reports any pin behind upstream latest stable:
`le go version-pin check` only compares Go builder tags with go.mod and never
looks upstream.

Testing on the Linux host: image builds, static checks and unit tests only
until the owner says the host is free (2026-10-22: "do not run full testing on
this host, it is too busy").
