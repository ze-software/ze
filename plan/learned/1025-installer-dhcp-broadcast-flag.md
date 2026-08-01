# 1025 -- Installer initrd DHCP: nclient4 must set the BOOTP broadcast flag

## Context

Field regression found by PXE-booting a real Intel I226 (`igc`) appliance off
`pxe.sh`. After the kernel + initrd downloaded, the box waited ~2 minutes and
rebooted instead of installing. The old busybox initrd (deleted in `faabc1cbb`)
installed fine on the same hardware; the pure-Go PID-1 initrd from
[[1024-installer-initrd-pure-go]] (`ed5636f65`) did not. This is exactly the
"first-run risk point #1" that 1024 flagged as unvalidated: *nclient4 DHCP
behaviour outside QEMU slirp.*

## Root cause (confirmed on the wire, not inferred)

`internal/install/disk/dhcp_linux.go` did `client.Request(ctx)` with no
modifiers. `nclient4` builds the DISCOVER/REQUEST with the **BOOTP broadcast
flag clear** (`Flags 0x0000`). During DORA the client owns no IP, so a server
that honours the clear flag delivers the OFFER **unicast to the offered address
(yiaddr)** and ARPs for it -- an address the client cannot answer for yet -- so
the lease is never delivered.

Server-side `tcpdump` on the DHCP segment was decisive:

- installer DISCOVERs: `length 548 ... Flags [none] (0x0000)` -> **no reply**;
  the server immediately `ARP, Request who-has <yiaddr>` and never gets an answer.
- iPXE DISCOVERs on the same link: `Flags [Broadcast] (0x8000)` -> broadcast
  `Reply, Your-IP <addr>`. Works.

So the installer never got an address, never issued `GET /install/image/...`, and
`ensureNetwork` -> `fatalInitrd` rebooted after its timeout (~90s probe + 30s
`branchReboot`). It was never a kernel-driver or `CONFIG_PACKET` problem: the
built kernel has `igc` and `e1000` compiled in, and the packets left the box fine.

## Fix

`dhcp_linux.go`: apply `dhcpv4.WithBroadcast(true)` to every DISCOVER/REQUEST
(`var dhcpRequestModifiers`). `nclient4.Client.Request` forwards modifiers to
both `DiscoverOffer` and `RequestFromOffer`, so one modifier covers the whole
DORA. Guarded by `TestDHCPRequestsBroadcast` (asserts `NewDiscovery(...,
dhcpRequestModifiers...).IsBroadcast()`).

## Reusable lessons

- **A DHCP client doing DORA from 0.0.0.0 must set the BOOTP broadcast flag**, or
  the server must L2-unicast the reply to `chaddr`. iPXE and busybox `udhcpc` set
  it; `nclient4` defaults to clear. Any pure-Go DHCP client added anywhere in Ze
  needs `WithBroadcast(true)` unless it already owns the address.
- **QEMU slirp cannot validate the installer's userspace DHCP path.** In slirp
  the NIC has instant stable carrier, so kernel `ip=dhcp` succeeds and
  `ensureNetwork`'s first `probeServer` returns reachable -- the `nclient4` +
  netlink fallback never runs. The `effective-install-*-qemu.py` harnesses all
  boot with `ip=dhcp`, so this whole path shipped untested. To exercise it you
  must force the fallback: boot WITHOUT `ip=dhcp` (or with delayed/racing carrier)
  so `probeServer` fails first. Until such a gate exists, installer DHCP changes
  need a real-hardware or non-slirp boot.
- **Diagnostic tell for the flag bug**: on the server, a DISCOVER with `Flags
  [none]` that draws no reply while the server ARPs for the *offered* IP. Compare
  against a working client (iPXE) on the same wire to see `Flags [Broadcast]`.
- **The ze DHCP server also mis-serves flag-clear clients** (it unicasts to
  yiaddr + ARPs instead of L2-unicasting to `chaddr` per RFC 2131 4.1). Fixing the
  client restored correct behaviour and matched iPXE/udhcpc; hardening the server
  is an optional separate follow-up, not required for the installer.

## Follow-up: the fix did not deploy (initrd cache-key under-hashing)

The first on-hardware retest still showed `Flags [none]` DISCOVERs. Cause was
**not** the fix: `initrdCacheVariant` (`internal/appliance/cache.go`) hashed only
4 hand-picked source files (`main.go`, `initrd_linux.go`, `bootstrap_linux.go`,
`rescue_linux.go`). `dhcp_linux.go` was not among them, so editing it did not
change the cache variant (`v2-amd64-52c77bd4`), `resolveInitrd` hit the stale
cache, and the old binary was served -- even though ze-installer's source had the
fix. Confirmed on the wire (installer DISCOVER still `Flags [none]`) and on disk
(served `initrd.img.gz` older than the source edit).

Fixed by hashing **every non-test `.go` file** under `cmd/ze-installer` and
`internal/install/disk` (`initrdSourceFiles`), so any installer source edit moves
the variant and forces a rebuild. Guard: `TestInitrdCacheVariantHashesAllDiskSources`.

**Reusable lesson:** a build-artifact cache key must hash the artifact's *whole*
first-party input set, not a hand-picked file list. Under-hashing fails silent --
it serves a stale artifact with no error, and the symptom looks like "my fix
doesn't work" rather than "the cache is stale." 1024 already flagged the initrd
cache key as narrow; this is that trap firing. When the built thing is
`go build ./cmd/X`, hash the package dir(s), not a subset.

## Validation done

Host `ze-verify` does not cover this: the changed files are `//go:build linux`
and the box is darwin. Verified by cross-compiling the package test for
linux/amd64 and running it on the linux appliance builder (`TestDHCPRequestsBroadcast`
+ `TestDHCPAcquireSignature` PASS), plus `make ze-lint-changed` (0 issues).
End-to-end on-hardware confirmation (rebuild initrd via `pxe.sh`, re-boot the
I226 target, watch the DISCOVER draw a broadcast reply and the image download
start) is the deploy step that follows this commit.

## Files

None recorded.
