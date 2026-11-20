# Patched VPP v26.06 for the real-VPP deployment run

This directory owns the retained diff and the reproducible build recipe of the
VPP image `./le test deployment vpp` boots. No copy of the VPP source tree is
kept in Ze.

The base is FD.io VPP **v26.06**, commit
`c3200b88dc46bd380f00a49ca3392a102cc1980b`, the release the deployment run
measured before this patch (as the upstream image `ligato/vpp-base:26.06-release`).
The Dockerfile verifies the SHA256 of that commit's codeload archive before
extraction. `LICENSE` is copied verbatim from that VPP source commit and
installed in the image at `/usr/share/doc/vpp/LICENSE`. The patch is released
under that licence, Apache-2.0, so upstream can take it.

It is a directory of its own, not a layer of `../vpp-patches/`: that series is
pinned to VPP v25.10 for the PPPoE lab, and every file in its directory is part
of the PPPoE image's cache key.

| Patch | Purpose |
|-------|---------|
| 0106 (Ze) | Judge unprotected inbound traffic against the interface's SPD, and pass what a policy-mode SA decrypted on a buffer mark |

## What 0106 changes

VPP's `ipsec4-input-feature` and `ipsec6-input-feature` look a packet up in the
SPD only when it is ESP, AH or UDP to port 500 or 4500; every other packet is
forwarded whatever entry its selectors match. The run of 2026-11-20 measured
it: cleartext matching an inbound PROTECT, DISCARD and BYPASS entry was
forwarded each time, counted `IPSec policy not matched` 1
(`docs/architecture/testing/interop.md`, "Real VPP cleartext proof").

RFC 4301 Section 5.2 step 3b: "If there is a match and the packet is to be
discarded or bypassed, do so."

The input node cannot look every packet up, because in policy mode the packet
`esp4-decrypt` yields re-enters `ip4-input-no-checksum` on the interface the
ESP arrived on, with its RX interface unchanged, and its selectors match the
PROTECT entry that decrypted it. So:

| File | Change |
|------|--------|
| `src/vnet/buffer.h` | New vnet buffer flag `VNET_BUFFER_F_IPSEC_DECRYPTED`, the first free bit (19); the `AVAIL` bits renumbered as the header asks |
| `src/vnet/ipsec/esp_decrypt.c` | `esp_decrypt_post_crypto`, the one post-decrypt step of the sync, async and chained-buffer paths, sets the flag when a policy-mode (`!is_tun`) packet goes to `ip4-input-no-checksum` or `ip6-input`, tunnel and transport mode |
| `src/vnet/ipsec/ah_decrypt.c` | The same mark on the AH packet handed back to IP input |
| `src/vnet/ipsec/ipsec_input.c` | Both input nodes read and clear the flag on every packet. A marked packet passes and counts `IPSec decrypted packet`. An unmarked packet that is not ESP, AH or IKE is matched over the BYPASS and DISCARD entries and the PROTECT entries' selectors (addresses, protocol, ports), highest priority first: BYPASS passes it (`IPSec policy bypass`), DISCARD drops it (`IPSec policy discard`), PROTECT drops it (`IPSec cleartext matched protect`), no match passes it as before |

On equal priority DISCARD wins over PROTECT and PROTECT over BYPASS, so a tie
never passes traffic. A packet whose ports cannot be read (a non-initial IPv4
fragment, a transport header outside the first buffer) matches only an entry
whose port ranges are whole.

Not covered by the patch:

| Path | Why |
|------|-----|
| A fast-path SPD (`ipv4-inbound-spd-fast-path`, `ipv6-inbound-spd-fast-path`) | Its entries are kept out of `spd->policies`, so the lookup finds none and the SPD passes cleartext as before. Ze's VPP configuration does not enable it |
| IPv6 extension headers | Not walked: a packet with one is matched with the extension header as its protocol and no ports, as VPP's ESP lookup also reads `ip6->protocol` alone |
| Tunnel-protect (route-mode) SAs | Not marked: their packets arrive on the tunnel interface, and Ze uses policy mode |
| A decrypted packet whose inner header ends on another arc (multicast) | The mark is cleared only by the unicast input features; it is read by nothing else |
| RFC 4301 Section 5.2 step 4, the inner-selector check after decryption | Not added: a marked packet passes on the mark |

## Building and using it

Run `./le build vpp deployment`. It builds the image as `ze-vpp-deployment`
and keeps it in `le`'s persistent image cache, keyed by the Dockerfile, this
directory and the build host's CPU, because `VPP_BUILD_NATIVE_ONLY=ON` compiles
for that CPU. `./le test deployment vpp` loads that cache entry and never builds
the image: on a cold cache it refuses and names `./le build vpp deployment`. A
run started with `ZE_VPP_DOCKER_IMAGE` set to another image pulls that one
instead. The image runs on the architecture it was built on, so on an arm64
host the run also needs `ZE_VPP_DOCKER_PLATFORM=linux/arm64` and
`ZE_VPP_DOCKER_GOARCH=arm64`.
<!-- source: internal/le/build/vpp/deployment.go -- deploymentImageRecipe, buildDeploymentImage -->
<!-- source: internal/le/test/deployment/vppiface.go -- readyImage -->
