# 1026 -- Installer overwrote the image-baked seed, stranding SSH on localhost

## Context

Follow-on from [[1025-installer-dhcp-broadcast-flag]]. Once DHCP was fixed the
box installed end-to-end and booted, but the operator could no longer SSH in:
the appliance came up with SSH bound to `127.0.0.1:2222` instead of the
provisioned `0.0.0.0:21982` from `prod.json`. "Previously I could ssh to the box
after installation" -- a regression, not a new feature gap.

## Root cause (read the producer, not the caller)

Two `database.zefs` builders exist and one clobbers the other:

- `ze appliance build` bakes the **full** appliance seed into the image `/perm`
  partition: `internal/appliance/cmd_build.go` `injectZeFS` writes the
  assembled `dbPath` to `ze/database.zefs`. The seed carries the SSH listener
  overrides (`internal/appliance/cmd_assemble.go` ->
  `set environment ssh server default ip 0.0.0.0` / `port 21982`), the
  `ze.conf` template, web certs, and instance identity.
- The image server's `/install/database.zefs` endpoint builds a **minimal
  first-boot bootstrap** database: `internal/plugins/imageserver/handler.go`
  `buildZefsDB` writes only SSH/admin creds and hardcodes the listener to
  `sshHost="127.0.0.1"`, `sshPort="2222"` (`handler.go`). It writes **no**
  `ze.conf` template -- by design, per [[811-install-3-image-server]].

The installer's `mountInjectDB` (`internal/install/disk/system.go`) then
**unconditionally** downloaded `/install/database.zefs` and wrote it over
`/perm/ze/database.zefs`, replacing the full baked seed with the localhost-only
bootstrap DB. A raw image is dd'd to disk, so `/perm` arrives with the good seed;
the injection destroyed it on every install.

## Why no test caught it (the untested-path lesson)

The QEMU evidence harness `scripts/evidence/effective-install-qemu.py` runs
`ze appliance assemble --keep`, then serves that **full** assembled seed at
`/install/database.zefs` from its **own** Python HTTP server. It never exercises
the real imageserver plugin's `buildZefsDB`, so the localhost-only bootstrap DB
was never on the tested path. AC-10 (SSH login) passed in QEMU while hardware
failed. A harness that substitutes the correct artifact for the component under
test proves nothing about that component.

## Fix

- `mountInjectDB` is now non-destructive: it keeps a seed the image already
  carries and only downloads the bootstrap DB for a seedless image
  (`bakedSeedPresent` guards on a non-empty, non-dir file). This restores the
  exact prior behaviour (box boots the baked prod seed) with no operator-workflow
  change. Unit test: `internal/install/disk/system_test.go`.
- Install visibility: the installer kernel cmdline now carries
  `loglevel=8 earlycon` (`internal/plugins/imageserver/handler.go`), matching the
  runtime kernel's `KernelExtraArgs`, so kernel/console/driver messages are
  verbose and appear from the earliest boot moment instead of a blank screen.

## Reusable lessons

- When two builders write the same on-disk artifact, the *last writer wins* --
  audit the whole write ordering, not just the producer you are looking at.
- The image-baked seed is authoritative for a provisioned appliance; the image
  server endpoint is a bootstrap fallback for seedless images. Do not let a
  fallback overwrite the real thing.
- A visibility change (`loglevel`/`earlycon`) surfaces kernel-side console
  bring-up but cannot cure a hardware framebuffer/serial mapping fault. Confirm
  a blank screen on the actual box before claiming it fixed.

## Validation done

- `go test ./internal/install/disk/ -run TestBakedSeedPresent`, imageserver
  boot.ipxe cmdline test: pass.
- `GOOS=linux GOARCH=amd64 go vet -tags ze_installer ./internal/install/disk/
  ./cmd/ze-installer/`: clean. `make ze-lint-changed`: 0 issues.
- End-to-end proof (box boots prod seed, external SSH on `0.0.0.0:21982`)
  requires a hardware redeploy on `ze-installer`; not claimed until observed.

## Files

None recorded.
