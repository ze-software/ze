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

**The image URL never reaches the console as the operator typed it.** A private
mirror is reached with `https://user:password@host/`, and the installer prints
its URL on every retry, in the failure it returns, and in the line that names
the server it is probing. Each of those writes `redact.URL`, and each wrapped
transport failure goes through `redact.URLError`, so the userinfo is replaced
with `<redacted>`. An installer console is read over a serial line and
photographed, which is the worst place for a credential to appear.
<!-- source: internal/core/redact/redact.go -- URL, URLError -->

## What this work exists to prevent

An Intel N150 appliance bricked silently. The build wrote a broken image, and
the device restart-looped on it. Two separate defects lined up.

**The build side trusted a tool that lies.** `debugfs -R` exits 0 even when its
subcommand fails, and the runner only checked the exit code. A failed database
injection therefore shipped as a good image. The fix reads the perm image back
and fails hard when the source bytes are absent. Any future `debugfs`
invocation must verify its result independently, because the exit code does not.

<!-- source: internal/appliance/diskverify.go -- verifyInject, verifyInjectedDB, verifyE2fsck, tryLoopbackVerify -->

**The runtime side had no floor.** With `/perm/ze/` absent, the device
restart-looped. The appliance now imports `/perm/ze/database.zefs` explicitly
into `/perm/ze/database/` on first boot and logs the import. `storage.ImportBlob`
verifies every seed key before retiring the seed as `database.zefs.replaced-*`;
it also resumes an interrupted import without replacing an unrelated tree.
With neither store nor seed present, the appliance auto-initializes to a
reachable but unprovisioned state, gated on `ze.gokrazy.enabled`.

<!-- source: cmd/ze/ze_core_autoinit.go -- gokrazyAutoInit -->

## Decisions

- Build-side verification compares bytes with `bytes.Contains`, not a `debugfs`
  dump read-back. It is pure Go, it shells out to nothing, and it catches the
  exact failure mode.
- `dd` is gone from the build path. `extractPartition` and `writePartition` use
  `ReadAt` and `WriteAt`.
- Injection sets the configuration folder to root-owned 0700 and its seed to
  root-owned 0600. The build host's UID must never become the appliance store
  owner, and debugfs's default directory mode is too loose for the live store.
- The auto-init gate is an environment variable, not a config leaf, because it
  is needed before config loads.
- Fallback auth posture is connectivity-only. No SSH or web credential keys are
  written, matching the serial fail-open recovery posture.
- The installer registers as `ze install disk` under the existing subdispatch,
  not as a new binary.
- PXE injection checks `/perm/ze/database/` before the sibling
  `database.zefs`. It preserves an existing tree and never downloads a bootstrap
  blob beside it. A non-empty baked seed is also preserved. An invalid live
  store path stops installation rather than being overwritten.

<!-- source: internal/install/disk/system.go -- mountInjectDB, bakedSeedPresent -->

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
