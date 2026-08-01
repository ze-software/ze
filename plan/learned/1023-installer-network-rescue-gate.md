# installer-network-rescue-gate

Closes `plan/spec-installer-network-rescue-gate.md`.

## Problem

A multi-homed PXE install target booted via iPXE on the install NIC
(198.19.255.x) but the booted kernel's `ip=dhcp` configured a *second* office
NIC (10.12.104.x), leaving no route to the image server. The installer falsely
reported "server reachable", every HTTP fetch timed out, then it `fatal`'d into
a rescue shell bound to serial (invisible on the monitor) and ungated.

## Fix (three bugs)

1. **NIC selection** — pin to the NIC iPXE booted from, carried on the kernel
   cmdline as `ze.mac`, instead of whichever NIC the kernel's `ip=dhcp` picked.
2. **Reachability honesty** — probe the install server for real before declaring
   it reachable; only fall back to bringing up other NICs when it is not.
3. **Rescue gate** — the rescue shell runs on all consoles and is
   password-gated (`ze.shell-auth`, sha256 of the admin password) and
   fail-closed on the network path.

## Where it landed (and how it moved)

- Server side, still live: `serveBootIPXE` emits `ze.mac=${mac}` and
  `ze.shell-auth=<hex>` on the kernel line (`internal/plugins/imageserver/handler.go:212`);
  provision writes `shell-auth-sha256` into the generated config + YANG leaf
  (`internal/plugins/provision/main.go`).
- Client side moved: the spec implemented this in the busybox init
  (`tools/installer-initrd/init`), which was subsequently **deleted**
  (`faabc1cbb`) and re-implemented in the pure-Go installer:
  `ensureNetwork` does honest reachability + MAC-pin to `cfg.Mac`
  (`internal/install/disk/network.go:36-79`), and `cfg.Mac` comes from parsing
  `ze.mac` (`internal/install/disk/cmdline.go:55-56`).
- Follow-up: `make ze-pxe`'s embedded iPXE also now emits `ze.mac=${mac}`
  (`mk/appliance.mk:116`, commit `887a690ab`) so the build/pxe path matches the
  server-generated `boot.ipxe`.

## Status at closure

Implementation complete; automated gates were green per the spec's Review Gate
(busybox init unit suites, `go test ./internal/plugins/imageserver/...` +
`.../provision/...`, `make ze-lint-changed`, `bin/ze-test install`). Those
busybox unit suites no longer exist (the shell init was deleted); the behaviour
is now carried by the pure-Go installer and its own tests.

## CAVEAT — closed WITHOUT the hardware/QEMU validation

Spec AC-7 (manual multi-homed hardware/QEMU PXE re-test: a multi-homed target
completes the image write via `ze.mac` pinning, and a forced `fatal` shows a
password prompt on the monitor) was **NOT performed** — no lab hardware access
in the closing session. This is the one open item and it is **unverified**.
Closed by owner decision accepting that deferral, not because the field test
passed. If the pure-Go install is later validated on the multi-homed lab target
(or in QEMU via `plan/learned/1024-installer-initrd-pure-go.md` AC-2), that is the real proof
this rescue behaviour works end-to-end.

## Files

None recorded.
