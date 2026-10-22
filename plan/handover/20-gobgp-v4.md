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

## Progress (BGP peers)

Agent of 2026-10-22, BGP-peer slice of M14. Landed:

| Peer | Old | New | Proof on this host | Commit |
|------|-----|-----|--------------------|--------|
| FRR | 10.3.1 default, 10.4.1/10.7.1 pins | 10.7.1 by digest, one declaration `interoplab.FRRImage` used by the BGP, IPsec, L2TP labs and `perfrunner.DUTs`; the five `frr-image` pins name the same digest | `bgpd --version` 10.7.1; `vtysh -C` over 176 frr*.conf rejects nothing 10.3.1 accepted | ecc446c73b |
| BIRD | 2.15.1 (Alpine 3.21) | 3.4.0 built from gitlab tag v3.4.0, commit-checked | `bird --version` 3.4.0; `bird -p` over 31 bird*.conf: 0 failures; birdc answers | e4c755f987 |
| ExaBGP | 5.0.13 | 5.0.15 (`exaBGPPins`) | installs under uv, healthcheck keeps `--no-ack`, `--fast-interval` | dcd3cb5a35 |
| pmacct pmbmpd | `:latest` | v1.8.0 (`defaultPMACCTImage`) | `pmbmpd -V` 1.8.0 | dcd3cb5a35 |
| StayRTR live test | `:latest` | v0.6.4 (`stayrtrImage`) | `stayrtr -version` 0.6.4 | dcd3cb5a35 |
| OpenBGPD, perf sender | 8.6 (Alpine 3.21); sender alpine:3.21 | 9.3 from the checksummed portable tarball (perf build context is the interop dir, `OPENBGPD_DIR` retired); sender alpine:3.24.3 | `bgpd -V` 9.3; `bgpd -n` on test/perf/configs/openbgpd.conf: OK; perfrunner tests pass | 6dc7d0124f |
| rustbgpd (perf DUT) | v0.7.0, rust 1.88, bookworm | v0.79.0, rust:1.99.1-trixie, debian:trixie-slim | `rustbgpd --help` banner 0.79.0 | 23beacca0f |
| RustyBGP (perf DUT) | master, rust 1.88, alpine 3.21 | v0.2.0 tag, rust:1.99.1-alpine3.24 (+`make` for jemalloc), alpine:3.24.3 | `rustybgpd --version` v0.2.0-97abd957f4 | 23beacca0f |
| StayRTR image | golang:1.26-alpine, alpine:3.21 | golang:1.27-alpine3.24, alpine:3.24.3, still v0.6.4 | `stayrtr -version` 0.6.4 | 23beacca0f |
| BGP lab Ze image | alpine:3.21 | alpine:3.24.3 | `bgp-ebgp-ipv4-bird` passes (1m2s) | 643e104f5e |

Scenario runs (continuation agent, one at a time under the session flock):
`bgp-ebgp-ipv4-bird` pass. Results of the rest are in the table below as they land.

Owed on a free host (scenario runs; only one at a time, under the session
flock): every `*-frr` and `*-bird` scenario, rtr-stayrtr, vpn-frr, evpn-frr,
the pmacct BMP scenarios, interop-ipsec and interop-l2tp with FRR 10.7.1,
`./le test exabgp` for both pins, `./le perf` for frr, bird, openbgpd,
rustbgpd, rustybgp. interop.md prose that states FRR 10.3.1 behavior
(extended-message gating, OSPFv3 virtual-link, Type 5 ESI display, VPN-IPv6
link-local display) and the BIRD next-hop limitation row are unverified at
the new versions until those runs. The five `frr-image` files now duplicate
`interoplab.FRRImage`; deleting them is a test-file deletion and needs the
owner's yes. `TestPPPoELabPeersRunWithoutPrivilege` was red in
internal/le/interoplab/pppoe (scenario ipv6cp-missing-option has no checker),
another session's work, untouched.

## Progress (access, VPN, DHCP peers)

Agent of 2026-10-22, stopped at its 100-call budget; a continuation owes every
row marked OWED. Docker on this host was badly backlogged (one `keepalived -v`
took over two minutes), which is why several proofs are still open.

| Peer | Old | New | Proof so far | Commit subject |
|------|-----|-----|--------------|----------------|
| strongSwan (lab) | 5.9.14 apk | 6.1.0 source, `test/interop-ipsec/Dockerfile.strongswan` | built; charon and swanctl 6.1.0; the 40 non-certificate `swanctl.conf` load clean; CCM from aesni, from openssl without aesni | `interop-ipsec: strongSwan 6.1.0 from source` |
| NAT box, Ze IPsec image | alpine 3.21 | alpine 3.24.3 | built; iptables 1.8.13 keeps nf_tables, as before | same |
| FreeRADIUS | 3.2.7 | 3.2.10 by digest, `interoplab.FreeRADIUSImage` (declared once) | `-XC` OK on the 4 admin scenarios and the subscriber config; radius unit tests ok | `interop access labs: FreeRADIUS 3.2.10, ppp 2.5.4` |
| pppd + rp-pppoe client | 2.5.1 apk | ppp 2.5.4 source, `test/interop-pppoe/Dockerfile.client` | first build failed: scanelf prints an empty NEEDED line for a file that links nothing, which became a bare `so:` apk refused; fixed in 8c82bd749b. Now `pppd version 2.5.4`, pppoe.so and pppol2tp.so in `/usr/lib/pppd/2.5.4` | same, plus 8c82bd749b |
| xl2tpd + pppd LACs | 1.3.18 / 2.5.1 | 1.3.20 / 2.5.4 source, both LAC Dockerfiles with the same pins | same build defect, fixed in 8c82bd749b; both LACs: `xl2tpd-1.3.20`, `pppd version 2.5.4`, pppol2tp.so present | same, plus 8c82bd749b |
| accel-ppp | 1.13.0 | 1.14.0, the Alpine 3.24 package | built; `accel-ppp 1.14.0` | same |
| PPP preflight image | `alpine:3.21` three times | `interoplab.PPPProbeImage`, alpine:3.24.3 | l2tp and radiussubscriber unit ok; pppoe red `TestPPPoELabPeersRunWithoutPrivilege` ("scenario ipv6cp-missing-option has no Go checker"), red at HEAD and not this change | same |
| Kea | 2.6.2 | 3.2.1 source (meson), `test/interop/Dockerfile.kea` | built; `-V` 3.2.1; `-t` exit 0 on both confirm configs; the renew config started and logged DHCP6_STARTED | `interop: Kea 3.2.1, keepalived 2.4.3, dnsmasq 2.93` |
| keepalived | 2.3.1 | 2.4.3 source | built; `Keepalived v2.4.3`; `-t` exit 0 on both vrrp configs | same |
| dnsmasq | 2.90 | 2.93 source | built; the first image reported an empty version (busybox `sort` lacks `-V`, which `bld/get-version` needs), coreutils added in 8c82bd749b; now `Dnsmasq version 2.93`, `--test` OK on the 3 configs | same, plus 8c82bd749b |
| QEMU guest Alpine | 3.21.3 | 3.24.3 (`AlpineVersion`, `AlpineMinor`, `internal/le/test/qemu/run.go`) | qemu unit tests ok; the ISO URL answers 200; no guest booted | `qemu: Alpine guest 3.24.3` |
| `docker/Dockerfile.lab` | alpine:3.21 | alpine:3.24.3, docker pages follow | OWED: build | same |
| flowexport collectors base | debian bookworm | debian:trixie-slim by index digest | built (Debian 13.7); nfdump and nfcapd `1.7.10-release options: LZ4 ZSTD BZIP2`; sflowtool starts | 64280b0c3d |

QEMU guest peers still behind upstream after the bump, from Alpine 3.24
packages: strongSwan 6.0.7 (6.1.0 upstream, the mobike guest) and keepalived
2.3.4 (2.4.3). accel-ppp 1.14.0 is current. M1 moves the guests to Ze's own
system, which closes that gap; not done here.

Scenarios, one at a time under the session's shared lock. The continuation
agent's first verdicts:

| Scenario | Peer | Verdict |
|----------|------|---------|
| `radius-admin-pap-freeradius` | FreeRADIUS 3.2.10 | first run FAIL on infrastructure only ("docker run -d did not finish within 1m0s", 13 sibling builds on the daemon); rerun PASS |
| `radius-acct-restart-freeradius` | FreeRADIUS 3.2.10, LAC on pppd 2.5.4 / xl2tpd 1.3.20 | PASS (39s) |
| `psk-site-to-site` | strongSwan 6.1.0 | PASS |
| `dhcpv4-renew-nak-dnsmasq` | dnsmasq 2.93 | PASS |
| `vrrp-mastership-keepalived` | keepalived 2.4.3 | PASS |
| `01-pppoe-chap-ipv4` (`docker-pppoe-accel-test`) | accel-ppp 1.14.0 | PASS |
| `01-ppp-ipv4` (`docker-l2tp-ppp-test`) | xl2tpd 1.3.20 / pppd 2.5.4 | FAIL on infrastructure only ("docker run -d did not finish within 1m0s"); rerun queued |
| `dhcpv6-client-renew-kea` | Kea 3.2.1 | FAIL, lab drift: "Kea extended the lease in 6 of 13 RENEW exchanges, want every one". Kea 3.0 turned lease caching on by default (Kea ChangeLog 2360, `cache-threshold` default 0.25 in `simple_parser6.cc` of 3.2.1), so every other RENEW got DHCP6_LEASE_REUSE instead of DHCP6_LEASE_RENEW. Fix: `"cache-threshold": 0.0` in the scenario's `kea-dhcp6.conf`, the checker unchanged; rerun queued |

Runner for the rest (continuation of the queue below, one scenario per lock
hold): `run3.sh` over `batch3.txt` in the session scratchpad `scripts/`
directory, one log per entry at
`tmp/session/2026-10-22-fb7d3e9b-9f4c-4139-a921-e861e0268991/scratch/scn3-<name>.log`
ending `EXIT=`. Order: `docker/Dockerfile.lab` build, the two reruns, then
eap-tls13, eap-mschapv2 (certificate-authenticated responder), the QEMU MOBIKE
guest (`./le test qemu ipsec-mobike-test kernel /boot/vmlinuz-7.2.9-ze`),
natt-tunnel-inner-checksum, the rest of radius, radius-subscriber, l2tp, pppoe,
the other dhcp and vrrp scenarios, then the rest of interop-ipsec.

Second round (runner `run3.sh`, logs `scn3-<name>.log`):

| Scenario | Verdict |
|----------|---------|
| `docker/Dockerfile.lab` build | built (alpine 3.24.3), image `ze-m14-lab:test` removed |
| `dhcpv6-client-renew-kea` rerun | PASS with the fix, 66448f7699 |
| `01-ppp-ipv4` rerun | FAIL on infrastructure again ("docker info did not finish within 15s"); a third try `01-ppp-ipv4-r3` is queued at the end |
| `eap-mschapv2` (strongSwan certificate-authenticated responder) | PASS |
| `eap-tls13` | FAIL, lab drift: strongSwan 6.0.2+ "Setting `remote.eap_id` now always initiates an EAP-Identity exchange" (NEWS), and `apply_eap_identity` refused the anonymous NAI ze sends under RFC 9190 Section 2.1.8 ("EAP identity 'ze-test-client' required, not matched by 'anonymous'"). Fix: drop `eap_id` in `eap-tls13` and `eap-tls` swanctl.conf, so charon keys the TLS client-certificate check on ze's IKE IDi `ze-test-client` (same certificate identity asserted). Committed unproven; rerun `eap-tls13-r2` queued at the end; `eap-tls` runs in the ipsec rest |
| QEMU MOBIKE guest | not run: `tmp/qemu` is a dangling symlink into `~/.cache/ze-tmp/main/qemu`, whose tree was gone ("mkdir .../tmp/qemu: file exists"). Target recreated with `mkdir -p`; journal row in `plan/journal/error-names-no-fix-command.md`; rerun `qemu-mobike-r2` queued at the end |
| `natt-tunnel-inner-checksum` | FAIL, cause OPEN: the IKE SA comes up over UDP 4500 ("faking NAT situation"), but the strongSwan log ends at "IKE_SA ze[1] established" with no CHILD_SA line and no IKE_AUTH response, and ze's Udp.NoPorts never moves for the good-checksum nping probe (`checkInnerChecksums`). psk-site-to-site (no forced encap) passes, so suspect the 6.1.0 CHILD_SA install with UDP encapsulation. Next: read the full charon log of `natt-tunnel-inner-checksum-r2` (queued) and `natt-transport-inner-checksum` from the ipsec rest |

`docs/guide/ipsec.md` TLS claim re-read in the strongSwan 6.1.0 source: the
default is still TLS 1.2 (`src/libtls/tls.c`) and libtls still implements no
extended master secret. Corrected to name 6.1.0 in 679bbfa69a.

Still OWED, queued by the continuation agent when its budget ended and not read
by it: `psk-site-to-site` (strongSwan 6.1.0), `dhcpv4-renew-nak-dnsmasq`,
`dhcpv6-client-renew-kea`, `vrrp-mastership-keepalived`,
`01-pppoe-chap-ipv4` (`docker-pppoe-accel-test`) and `01-ppp-ipv4`
(`docker-l2tp-ppp-test`). Then the rest of each suite: interop-ipsec (50),
interop-radius (3 more), interop-radius-subscriber (1 more), interop-l2tp (3
more), interop-pppoe (7 more), and the other dhcp and vrrp scenarios. Selector
variables: `IPSEC_INTEROP_SCENARIO`, `RADIUS_INTEROP_SCENARIO`,
`RADIUS_SUBSCRIBER_INTEROP_SCENARIO`, `ZE_PPPOE_INTEROP_SCENARIO`,
`ZE_L2TP_INTEROP_SCENARIO`, `INTEROP_SCENARIO`. No Ze defect and no lab drift
was found in the two scenarios that ran.

Other owed items, closed by the continuation agent:

- The pre-existing pppoe red (`TestPPPoELabPeersRunWithoutPrivilege`,
  "scenario ipv6cp-missing-option has no Go checker") already has its row in
  `plan/journal/feature-test-missing-build-tag.md` (2026-10-14): it is the
  untagged build, green with `-tags ze_l2tp`. No new row.
- Structural red attribution: `le arch tier check` and `le arch
  iface-resolution` both run green over the current tree, so 180dfc8b8a and
  46f61a5937 caused no red there. The commit gate still charges them from an
  older verify run, so 8c82bd749b and 64280b0c3d carry `structural-red-ok`
  with that reason.
- `docs/features.md`: rendered by `internal/le/feature` (`rebuildPage`), a
  derived artifact `derived.EnsureAll` rebuilds when absent. Moved aside and
  re-rendered: identical to the copy in place, so nothing was hand-edited.
- Lab pages now name the new peers: `docs/guide/pppoe.md`,
  `docs/labs/pppoe-interop.md` (pppd 2.5.4), `docs/guide/vrrp.md` (keepalived
  2.4.3), `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` (QEMU guest
  keepalived 2.3.4). Left as dated measurements, not lab descriptions: the
  strongSwan 5.9.14 sentences in the ike pages and `docs/guide/ipsec.md` (the
  TLS 1.3 default claim there is worth re-checking against 6.1.0), keepalived
  2.3.1 in `docs/functional-tests.md`, xl2tpd 1.3.18 in the l2tp pages.
- Every `ze-m14-*:test` image is removed. `ze-m14-bird-check`,
  `ze-m14-openbgpd-check` and the rust `-check` images belong to the BGP agents.

`alpine:3.21` literals left for their owners: `kernelProbeImage`
(`internal/le/interoplab/kernelcheck.go`), the rsvpte preflight, the docker
test fixtures in `internal/le/interoplab`, `test/interop/Dockerfile.ze`.

## Progress (source-built peers, freshness check)

Agent of 2026-10-22, source-built-peer and pin-freshness slice of M14.

| Item | Commit | Proof |
|------|--------|-------|
| One-month rule: `Docker.Build` calls `RequireFreshSource` (`internal/le/interoplab/sourcepin.go`). A `ZE_SOURCE_COMMIT` older than a month refuses the build and names `./le lab pin bump dockerfile <path>`. A `ZE_SOURCE_CHECKOUT` is fast-forwarded, or refused when dirty | 312f9b8b7e | `go test ./internal/le/interoplab/` green |
| freeRtr RSVP-TE pinned to 96368b02da10 through `ZE_SOURCE_*`; the carrier reads the pin (`rsvpFreePinnedRevision`) | c6fbaa54d1 | image `ze-rsvpte-freertr:probe` built from that pin (REVISION 96368b02da10, patch applied, on Alpine 3.21.3); the carrier compiles with `-tags integration` |
| `le lab pin check` and `le lab pin bump dockerfile <path>` (`internal/le/lab/pin`) | 8058cc6803 | `go test ./internal/le/lab/pin/` green |
| Docs: the interop.md sections "Source-built peers" and "Lab pins against upstream" | ed4004b142 | carried another session's FreeRADIUS 3.2.10 and alpine hunks, which are true at HEAD |
| Scan fix: official image literals in Go (`officialLiteral`, `officialContext`). The slash-only pattern had missed every `alpine:3.x` in Go | 9f47523615 | `TestCheckFindsEveryPinBehind` red, then green |
| Fixtures: `kernelProbeImage` and the interoplab docker fixtures to alpine:3.24.3, the deployment L2TP peer from alpine:3.20 to 3.24.3, deployment vpp-base 24.02 to 25.10, labtest stayrtr latest to v0.6.4 | 541761990d | unit tests green; the docker-tagged fixtures were not run |

Not committed. In the working tree, waiting for image proof:

- The RSVP-TE lab to alpine:3.24.3: `test/interop-rsvpte/Dockerfile.freertr` (both stages), `test/interop-rsvpte/Dockerfile.ze`, `mplsPreflightContainer` in `internal/le/interoplab/rsvpte/rsvpte.go`, the interop.md lab-base sentence and the kernel probe example.
- The perf freeRtr DUT `test/interop/Dockerfile.freertr`: eclipse-temurin 21 to 27 (jdk and jre), golang 1.26 to 1.27, native-builder alpine:3.24.3. It cannot be built on this host, which has no `../rtr` checkout.

Owed:

- The RSVP-TE scenario run (`RSVPTE_INTEROP_SCENARIO=transit-strict-hop-forwarded ./le test integration interop-rsvpte`). The first attempt failed in setup: the MPLS preflight `docker run` timed out at 2m. A bare `docker run --rm alpine:3.24.3` took 2m31s by hand, at load average 46 with no container running. The host was too loaded; the image was not the cause.
  A second attempt at 12:40 (load average 15) failed the same way: the preflight hit its 2m bound, then `docker rm -f` hit its 30s bound. The Docker daemon itself was answering slowly while other lab runs held it, so the scenario is still owed on a quiet daemon.
- Done since: the Alpine 3.24.3 image built (`ze-rsvpte-freertr:m14-alpine324`, patch applied, REVISION 96368b02da10), and the RSVP-TE alpine set landed in 843f3bafa9.
- `le lab pin check` after these commits: 132 pins, 19 behind. Owned by sibling agents: `Dockerfile.gobgp` (golang 1.25, alpine 3.21), `frr_image_test.go` (FRR 10.4.1), and the `Dockerfile.rustbgpd` trixie-slim tag. Left alone because they are parser fixtures, not lab pins: `internal/le/go/versionpin/goversion_test.go`. freeRtr RSVP-TE is one upstream commit behind, which the one-month rule allows. `pinnedImage` in `labtest_test.go` (stayrtr 0.6 by digest) is unresolved, because upstream has no tag in that shape.
- `le doc check verify` is red on the published wiki and gh-pages command catalogs (`clear bgp rib advertised`, `request peer *`). That is ze's command surface, not these commits.
- Done since: the perf freeRtr DUT bases landed unbuilt in 4f48c7f190; build it where a `../rtr` checkout exists (the Mac).

## Progress (GoBGP v4.10.0 scenarios)

Runs go one at a time under the session flock, from a `git clone --shared` of
committed main, because other sessions' uncommitted work broke the lab
cross-compile in the shared tree. Run `./le --update` first: a stale launcher
still read v3's `neighbor-ip` field and failed bgp-flowspec-gobgp spuriously.

| Scenario | Result on v4.10.0 | Fix |
|----------|-------------------|-----|
| bgp-flowspec-gobgp | pass | none |
| bgp-flowspec-sctp-gobgp | pass | 62bd6f2a09: nft prints SCTP as protocol 132 (`meta l4proto 132`), so the old check on the word `sctp` could never fail; the same on nftables 1.1.1 and 1.1.6 |
| bgp-vpn-gobgp | pass on unmodified main | a stronger proof sits in the working tree, uncommitted: an ipv4/unicast route beside the two mpls-vpn routes (`vpnSameAFIUnicastPrefix`, the `scenarioVPNGoBGP` case of `announcementPlan`, an ipv4 RIB wait in `check_extras.go`, the plan digest in `TestAnnouncementPlanOrderAndTimingFixture`). Owed: package unit test, rerun, then remove the "GoBGP 3.31 (not seen in 4.10.0 source)" limitation row and its source comment from interop.md, and commit them together |

Owed next, in order:

1. Revert-red for bgp-sr-policy-malformed-receive-gobgp. Red a: an empty slice to the `nlrisyntax.Register` loop of the srpolicy registration. Red b: disable the `srPolicyAnnounced` branch of `applyTunnelEncap`. On v4 a session reset can restore the route without the malformed one, so if red a stays green, make the scenario discriminate again (for example by asserting that GoBGP's `Flops` stays 0 in `gobgp neighbor`).
2. Revert-red for bgp-large-community-no-repeat-gobgp: disable `largeCommunityRedundant` in the egress normalizer.
3. bgp-exabgp-bridge-rtc-gobgp, then the rest of the scenario list above.
