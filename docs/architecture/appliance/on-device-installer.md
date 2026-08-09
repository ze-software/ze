# On-device installer and the un-brick path

`ze install disk` is the Go installer that writes a Ze image to a local disk. It
replaced an 884-line busybox shell script. The initrd calls it rather than
reimplementing the logic in shell.

<!-- source: internal/install/disk/run.go -- ze install disk entry point -->
<!-- source: internal/install/disk/register.go -- command registration -->
<!-- source: internal/install/disk/validate.go -- input validation, parity with the old shell validators -->
<!-- source: internal/install/disk/detect.go -- target disk detection -->
<!-- source: internal/install/disk/download.go -- HTTP download with SHA-256 and retry -->
<!-- source: internal/install/disk/iso.go -- ISO media detection, Ventoy included -->
<!-- source: internal/install/disk/cmdline.go -- kernel cmdline parsing -->
<!-- source: internal/install/disk/network.go -- network fallback -->

## What this work exists to prevent

An Intel N150 appliance bricked silently. The build wrote a broken image, and
the device restart-looped on it. Two separate defects lined up.

**The build side trusted a tool that lies.** `debugfs -R` exits 0 even when its
subcommand fails, and the runner only checked the exit code. A failed database
injection therefore shipped as a good image. The fix reads the perm image back
and fails hard when the source bytes are absent. Any future `debugfs`
invocation must verify its result independently, because the exit code does not.

<!-- source: internal/appliance/diskverify.go -- verifyInject, verifyInjectedDB, verifyE2fsck, tryLoopbackVerify -->

**The runtime side had no floor.** With `/perm/ze/` absent, blob creation failed
and the blob gate tripped, so gokrazy restart-looped. The device now
auto-initializes to a reachable but unprovisioned state, gated on the
`ze.gokrazy.enabled` environment variable. The operator must re-provision, by
design.

<!-- source: cmd/ze/ze_core_autoinit.go -- gokrazyAutoInit -->

## Decisions

- Build-side verification compares bytes with `bytes.Contains`, not a `debugfs`
  dump read-back. It is pure Go, it shells out to nothing, and it catches the
  exact failure mode.
- `dd` is gone from the build path. `extractPartition` and `writePartition` use
  `ReadAt` and `WriteAt`.
- The auto-init gate is an environment variable, not a config leaf, because it
  is needed before config loads.
- Fallback auth posture is connectivity-only. No SSH or web credential keys are
  written, matching the serial fail-open recovery posture.
- The installer registers as `ze install disk` under the existing subdispatch,
  not as a new binary.

## Constraints the code does not state

- **gokrazy does not reformat `/perm`.** It mounts or skips, and leaves `/perm`
  read-only on failure. The first assumption here was that gokrazy wipes
  `/perm`, and it was false. The correct frame is "ensure runtime
  mountability", not "prevent a reformat".
- **Build-time inject and on-device inject are different mechanisms.** One
  writes at a file offset, the other writes to a block device. They share the
  verifiers, the validators, and the codebase, and they do not share one inject
  function. A change to one is not a change to the other.
- `e2fsck` structural check and the loopback mount verify are extra layers.
  The first is best-effort, the second is root-only.

## Trap

`hcsshim/ext4/tar2ext4` holds a usable pure-Go ext4 writer (`compactext4`), and
it is tempting. It is an `internal/` package, so importing it pulls the full
Windows container runtime. Forking it costs about 1500 lines. It was rejected.

## Related

- `installer-initrd.md` for the PID 1 binary that calls this installer
- `iso-installer.md` for the ISO transport
