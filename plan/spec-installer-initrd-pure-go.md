# Spec: installer-initrd-pure-go

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-installer-network-rescue-gate (still `in-progress` as of 2026-06-29; the shell features landed in commit `7dc7761d9`, so the committed shell code is the ground truth for this port, but watch for further shell changes) |
| Phase | 5/6 |
| Updated | 2026-06-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/initrd-no-external-tools.md` - the endgame this spec fulfils
4. `ai/rules/qemu-testing.md` - QEMU integration tests are mandatory, never skipped
5. `internal/install/disk/run.go`, `network.go`, `cmdline.go` - the engine being extended
6. `tools/installer-initrd/init` - the shell installer being deleted

## Task

Rewrite the busybox shell PXE installer initrd (`tools/installer-initrd/init`,
1176 lines of `/bin/sh` running as PID 1) as a **pure-Go, busybox-free** initrd.

A single statically-linked Go binary (`cmd/ze-installer`, build-tagged,
`CGO_ENABLED=0`, cross-compiled per arch) becomes the initrd `/init` (PID 1). It
performs the PID-1 bootstrap, then installs via the existing
`internal/install/disk` engine, extended to:

1. Replace **every** busybox shell-out (mount, umount, losetup, mknod, ip,
   udhcpc, blockdev, sync, reboot, poweroff) with a `golang.org/x/sys/unix`
   syscall/ioctl or an already-vendored library call.
2. Port the shell's `ze.mac` boot-NIC pinning and DHCP lease application into Go.
3. Replace the shell's password-gated rescue `sh -i` with a **Go recovery
   console** (no `/bin/sh` exists in a busybox-free initrd) gated by
   `ze.shell-auth` via termios ECHO-off + sha256.
4. Fan console output to every console in Go.

Then delete the shell init, its tests, and `udhcpc.script`; rewire
`internal/appliance/cmd_initrd.go`, the initrd build, and provision staging to
ship the Go binary. **Zero external binaries in the initrd.**

This is the explicit endgame of `ai/rules/initrd-no-external-tools.md:22-29`
("swapped for a purpose-built binary when busybox is removed").

## Required Reading

### Architecture Docs
- [ ] `ai/rules/initrd-no-external-tools.md` - the rule this fulfils
  → Constraint: detect state via `/proc` and `/sys` reads, not external commands; isolate the unavoidable syscalls in named helpers.
- [ ] `ai/rules/qemu-testing.md` - linux-only code requires QEMU integration tests
  → Constraint: QEMU tests are mandatory; "needs hardware" is never an excuse to skip.
- [ ] `plan/learned/907-appliance-install-robust.md` - why `ze install disk` (Go) exists
  → Decision: a Go-native installer already replaces the shell logic; this spec wires it into the initrd and finishes the migration learned 907 began but never completed.
- [ ] `plan/learned/910-installer-initrd-console-and-ci-gotchas.md` - console fan-out + arch-aware cmdline
  → Constraint: headless boxes can have a dead preferred console; output must fan to every console.
- [ ] `docs/guide/appliance.md`, `docs/guide/ze-install.md` - install surface

### RFC Summaries (MUST for protocol work)
- N/A - not a wire protocol. DHCP client behavior is library-provided (`insomniacslk/dhcp`).

**Key insights:**
- Every busybox dependency has a pure-Go replacement already in `go.mod` and already used in production ze code; no new dependency is required.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `tools/installer-initrd/init` (1176L) - the live PXE installer: PID-1 bootstrap (`busybox --install`, mount proc/sys/dev), `setup_console`/`emit` fan-out, `parse_cmdline` + `validate_*`, `ensure_network` (ze.mac pin + per-iface DHCP recovery + `dhcp_acquire`), `find_target_disk`, `download_to_disk`, ISO/Ventoy, `fatal`/`rescue_on_all_consoles`/`verify_shell_auth` rescue gate.
  → Constraint: this is the CURRENTLY DEPLOYED installer (`cmd_initrd.go:21` builds from `tools/installer-initrd`); do not delete it until the Go path reaches QEMU-proven parity.
- [ ] `internal/install/disk/run.go` (195L) - `Run` entry; `runHTTP`/`runISO` orchestration.
- [ ] `internal/install/disk/cmdline.go` (66L) - parses 7 params; **does NOT parse `ze.mac` or `ze.shell-auth`** (`cmdline.go:38-53`).
- [ ] `internal/install/disk/network.go` (159L) - `ensureNetwork` has honest reachability (`:36-55`); `fallbackDHCP` brings up ALL NICs (`:57-117`), **no `ze.mac` pinning**, `udhcpc -i ... -t 5 -n -q` with **no `-s` lease script** (`:107`).
- [ ] `internal/install/disk/iso.go` + `system.go` - the shell-out sites: `mount`/`umount`/`mknod`/`losetup` (iso.go), `mount`/`umount`/`sync`/`reboot`/`poweroff` (system.go); `blockdev`/`sync` also in `run.go`. `detect.go`, `download.go`, `validate.go` are ALREADY pure Go (verified: no `runCmd`/`exec.Command`), so they are not in Files to Modify.
- [ ] `internal/appliance/cmd_initrd.go` (`:21` initrdToolsDir, `:112` busybox/cpio/gzip) - builds the shell initrd.
- [ ] `internal/plugins/provision/{staging.go,main.go}` - stages `initrd.img.gz` (`staging.go:21`).
- [ ] Reuse (DHCP request side): `internal/plugins/iface/dhcp/dhcp_v4_linux.go:37` (`nclient4.New` → `client.Request` does DORA).
- [ ] Reuse (DHCP **apply** side, the genuinely-new orchestration): `dhcp_v4_linux.go:158` `handleV4Lease` shows the production applier calling `iface.ReplaceAddressWithLifetime` (`:180`, address+mask) and `iface.AddRoute(... "0.0.0.0/0", router ...)` (`:190`, default route from the RFC 2132 Router option). Those two `iface.*` appliers ARE reusable as-is; `handleV4Lease` itself is coupled to `*DHCPClient.config` (RouteMetric, ResolvConfPath), so the installer must re-implement the orchestration, not call `handleV4Lease`. AC-5's "address + default route" maps to these two calls.
  → Constraint: the `nclient4.New` citation is the request side only; do not assume lease application is free reuse.
- [ ] Reuse: `internal/plugins/iface/netlink/manage_linux.go:58` (`netlink.LinkSetUp`).
- [ ] Reuse: `internal/component/config/system/console_linux.go:96-104` (termios ECHO-off **pattern**: `unix.IoctlGetTermios` + clear `unix.ECHO`). Note this producer configures a serial port for raw mode, not an interactive password read, so the technique transfers but the surrounding code does not.

**Behavior to preserve:**
- Kernel cmdline contract: `ze.source` (http/iso), `ze.server`, `ze.image`, `ze.port`, `ze.target`, `ze.wait`, `ze.media-id`, plus `ze.mac` and `ze.shell-auth`.
- imageserver endpoints consumed unchanged: `/install/image/<name>`, `/install/image/<name>.sha256`, `/install/database.zefs`.
- HTTP and ISO (incl Ventoy) install modes; image write to block device; database injection onto partition 4 (`/perm`); reboot (HTTP) / poweroff (ISO).
- Honest reachability (a probe timeout counts as unreachable).
- **Image download has a stall timeout.** The shell downloads with `wget -T "$WGET_TIMEOUT"` (`init:540`); the Go engine's `streamClient` has NO timeout (`download.go:27`, by design for multi-GB transfers) and the transfer is a bare `io.Copy` (`download.go:134`), so a mid-transfer network stall hangs PID 1 forever (`panic=-1` does NOT catch a userspace hang). Add a **stall/no-progress timeout** (read deadline reset per read, or a watchdog on bytes-written), NOT a total `http.Client.Timeout` (that would kill a legitimate large transfer).
- **Wait for the partition node before mounting.** After re-reading the partition table the shell does `blockdev --rereadpt` → `sleep 1` → `[ ! -b "$PART4" ] && fatal` → mount (`init:1122-1131`). The Go engine re-reads then mounts immediately with no wait (`run.go:127→130→132`), racing devtmpfs's async node creation. Port the bounded poll-for-`/dev/sdX4`-then-FATAL.
- `ze.mac` boot-NIC pinning; per-NIC DHCP recovery that ignores foreign NICs. This is NOT merely "apply a lease". The shell (`tools/installer-initrd/init:782,847-895`) does three things the Go port MUST replicate:
  (a) trusts the kernel `ip=dhcp` lease ONLY if `ze.server` is reachable over it (`has_default_route` + `server_reachable` gate, `:853-857`);
  (b) when the pinned NIC cannot reach the server, **flushes its address and deletes its default route** (`ip addr flush dev`, `ip route del default`, `:887-888`) before scanning the other NICs;
  (c) applies each lease's address AND default route (udhcpc `default.script` passed via `-s`, `:798-810`).
  The Go port must do the flush/delete via the existing `netlink.AddrDel` / `netlink.RouteDel` (`internal/plugins/iface/netlink/manage_linux.go:234,338`, also surfaced as `iface.RemoveAddress`, `internal/component/iface/dispatch.go:94`), not only `AddrAdd`/`RouteAdd`. Otherwise a stale foreign-NIC default route survives and defeats the pin (the exact failure `ze.mac` exists to prevent).
- FATAL rescue policy has **three** branches (`tools/installer-initrd/init:217-227`), not two:
  (a) credential present (`ze.shell-auth` set) → password-gated recovery console on every console;
  (b) no credential + ISO source → ungated recovery console (operator is physically present);
  (c) no credential + network source → print message, `sleep 30`, reboot. Branch (c) is the unattended-box safety valve: a network install with no credential must NOT hang waiting for a password nobody can supply.
- Console output visible on every console.
- **Step-level diagnostic logging (MANDATORY).** Every significant step in the install flow MUST log before it starts and on failure, with enough context to diagnose the root cause from console output alone. The installer runs as PID 1 on bare metal with no ssh, no journal, no debug shell (unless rescue mode). If something fails and the operator sees only "installation failed", the install is a black box. Each step must log: what it is about to do, what inputs it is using, and on failure what went wrong. Pre-slog steps (mount proc/sys/dev) use raw `os.Stdout.WriteString`. Post-slog steps use `slog.Info`/`slog.Warn`/`slog.Error`. The minimum logged steps are: each bootstrap mount, console setup result, parsed cmdline values, validation start, network probe/pin/DHCP/flush with interface names, disk detection result, image download start with URL, BLKRRPART result, partition node wait, mount/umount, DB injection, fatal branch selection with branch name, and reboot/poweroff.
- `panic=-1` boot behavior; `/init` must never exit except to reboot/poweroff.

**Behavior to change:** (user-approved)
- The installer runs as a Go PID-1 binary; the initrd contains **zero busybox / external binaries**.
- DHCP is performed in-process (`nclient4`) with the lease applied via netlink; `udhcpc` and `udhcpc.script` are removed entirely.
- The rescue shell becomes a **Go recovery console** (auth + a small menu: retry network / diagnostics / reboot / poweroff). There is no `/bin/sh`.

## Data Flow (MANDATORY)

### Entry Point
- Kernel unpacks the initrd and execs `/init` (= `cmd/ze-installer`, PID 1).
- `/init` reads `/proc/cmdline` after mounting `/proc`.

### Transformation Path
1. **PID-1 bootstrap**: `unix.Mount` proc, sysfs, devtmpfs; set up console fan-out; install signal/reap handling; never exit.
2. **Parse + validate**: `parseCmdline` (extended with `ze.mac`, `ze.shell-auth`) → `validateConfig`.
3. **Network**: `ensureNetwork` trusts a pre-existing kernel `ip=dhcp` lease ONLY if the server is reachable over it (mirroring `init:853-857`). If not, pin to the `ze.mac` NIC: `netlink.LinkSetUp` it, wait for carrier, `nclient4` single-shot DORA, apply address + default route via netlink, re-probe. If the pinned NIC STILL cannot reach the server, **flush its address + delete its default route** (`netlink.AddrDel`/`netlink.RouteDel`, mirroring `init:887-888`) before bringing up + DHCPing the remaining NICs. Never bring up a foreign NIC while the pin succeeds.
4. **Disk**: `findTargetDisk` (sysfs, already pure Go) → `downloadToDisk` (http + sha256 + `io.Copy` to `/dev/sdX`, already pure Go, **plus a new stall timeout** so a wedged transfer fails instead of hanging) → `BLKRRPART` ioctl → **poll for the partition-4 node to appear** (bounded; FATAL if it never does, mirroring `init:1122-1127`).
5. **DB inject**: `unix.Mount` partition 4 ext4 (only after the node exists) → download `database.zefs` → `unix.Unmount` → `unix.Sync`.
6. **Finish**: `unix.Reboot(LINUX_REBOOT_CMD_RESTART)` (HTTP) or `POWER_OFF` (ISO).
7. **On fatal**: `ze.shell-auth` already **is** the lowercase sha256 hex digest of the admin password (`init:110-116,243`); the console computes `sha256(typed password)` (termios ECHO-off) and compares it to `ze.shell-auth`. Do NOT hash `ze.shell-auth` again. Three-branch policy (`init:217-227`): credential present → gated console; no credential + ISO → ungated console; no credential + network → message, 30s wait, reboot (never hang).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel cmdline → Go | `/proc/cmdline` read + parse | [ ] |
| imageserver (HTTP) → Go | `net/http` GET of image/sha/zefs | [ ] |
| Go → block device | `os.OpenFile` + `io.Copy`; ioctls | [ ] |
| Go → kernel net | netlink (link/addr/route) + `nclient4` DHCP | [ ] |
| Go → mount/loop | `unix.Mount`/`Unmount`, loop ioctls | [ ] |

### Integration Points
- `internal/install/disk` engine - the shared install logic (also backs `ze install disk`).
- `internal/plugins/iface/dhcp` + `iface/netlink` - reuse patterns for DHCP and link/addr/route.
- `internal/component/config/system/console_linux.go` - termios ECHO-off pattern for the password prompt.
- `internal/appliance/cmd_initrd.go` + initrd Makefile - build/embed the Go binary.
- `internal/plugins/provision/staging.go` - stage the new initrd.

### Architectural Verification
- [ ] No bypassed layers (install logic stays in `internal/install/disk`, not duplicated in `cmd/ze-installer`)
- [ ] PID-1 bootstrap (`bootstrap_linux.go`: mount proc/sys/dev, take over child reaping, never-exit wrap) runs ONLY under the new `RunInitrd` entry. `Run` (`run.go:21`, the booted `ze install disk` path) must NOT bootstrap -- proc/sys/dev are already mounted and it is not PID 1. `unix.Reboot`/`unix.Mount` from the booted path preserve today's `reboot -f` / `mount` semantics.
- [ ] No unintended coupling (installer binary does not pull the full BGP/network-OS surface)
- [ ] Decide build-tag boundary: `internal/install/disk` is in `ze_core`, so `RunInitrd` + `bootstrap_linux.go` would compile into the booted `cmd/ze` binary even though only `cmd/ze-installer` calls them. Either accept the dead weight or guard the initrd-only files with a build tag; do not let the "single engine" goal silently bloat `ze`. (Resolve in the Audit stage.)
- [ ] No duplicated functionality (extends the existing engine; deletes the shell, does not add a third path)
- [ ] Zero-copy preserved where applicable (stream to disk via `io.Copy`, no full-image buffering)
- [ ] Registration over hardcoding (syscalls isolated in named `_linux.go` helpers per the no-external-tools rule)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A `CGO_ENABLED=0` static Go binary running as PID 1 builds and runs in an initrd | feasibility precedent: gokrazy's init is itself a Go PID-1 program (the appliance targets gokrazy, CLAUDE.md: "no systemd; must own full process lifecycle"), so Go-as-PID-1 works in this ecosystem. NOTE: ze-installer is its OWN binary, NOT gokrazy init (gokrazy init can't run from RAM -- see Key Design Decisions). The libs (netlink + insomniacslk/dhcp + x/sys) are already in `ze_core` builds (`go.mod:15,22,27`) | fall back to a thinner DHCP/netlink path | `make` cross-compile + QEMU boot | unvalidated |
| A-2 | devtmpfs provides `/dev` nodes (disks, consoles) once mounted at PID-1 time | standard kernel behavior; shell init relies on it | pre-create nodes via `unix.Mknod` | QEMU boot | unvalidated |
| A-3 | `nclient4` works as a single-shot DORA per NIC inside an initrd (raw AF_PACKET, link up first) | production daemon uses it (`dhcp_v4_linux.go:37`) | hand-roll a minimal DHCP over the existing AF_PACKET pattern | QEMU HTTP install | unvalidated |
| A-4 | loop ioctls (`LOOP_CONFIGURE`/`LOOP_SET_FD`) are available on target kernels for Ventoy ISO | gokrazy kernel includes loop | keep mknod+ioctl fallback chain | QEMU ISO/Ventoy install | unvalidated |
| A-5 | `ze.mac` (`${mac}` from iPXE) matches `/sys/class/net/*/address` form | shell init assumes the same (spec-installer-network-rescue-gate A-1) | normalize case/format before compare | QEMU multi-homed test | unvalidated |
| A-6 | The installer kernel has iso9660 + vfat/exfat/ntfs filesystem drivers built in (Ventoy data partitions) | the shell already `mount -t ntfs/exfat/vfat` (`iso.go:94`); moving to `unix.Mount` does NOT change the kernel-driver requirement, it is not a regression | a Ventoy partition on an unsupported fs is unreadable by either implementation; document the supported fs set in the install guide | QEMU Ventoy install | unvalidated |
| A-7 | The installer kernel has the **ext4** driver built in | EVERY HTTP install mounts partition 4 as ext4 to inject `database.zefs` (`run.go:130` → `system.go:24`; shell does the same at `init:1131`), so `unix.Mount(..., "ext4", ...)` after the swap needs it. The gokrazy kernel uses ext4 for `/perm`, so it is almost certainly present, and this is the PRIMARY path, not a Ventoy edge case | DB injection fails on every HTTP install | QEMU HTTP install (AC-2 already exercises it) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `/init` exits → kernel panic / bricked install | QEMU panic on boot | wrap `main` so every path ends in reboot/poweroff or the recovery console; never `return` from PID 1. CAVEAT: a `recover` in `main` does NOT catch panics in other goroutines -- see R-6 |
| R-6 | A panic in ANY goroutine (netlink/dhcp/`net/http` library, a nil deref) crashes the whole Go process → PID 1 dies → kernel panic, regardless of `recover` in `main` (recover is goroutine-local) | QEMU panic mid-install with a goroutine stack trace | single PID-1 binary with internal goroutines is the accepted design (user decision, no supervisor process); the mitigation is therefore in-process: every goroutine the installer spawns starts with `defer recover()` routing to FATAL (recovery console / reboot), never an unhandled crash. QEMU fault-injection test that a forced panic ends in the recovery path, not a kernel panic. Backstop: the PXE cmdline carries `panic=-1` (`imageserver/handler.go:212`), so even an UNrecovered panic → PID-1 death → kernel panic → immediate reboot (a reboot loop, not a silent hang) |
| R-2 | Deleting the shell before Go parity loses the working installer | QEMU regression after the retire phase | phased delivery; the shell stays until phase 5 flips `cmd_initrd.go`, gated by green QEMU. KNOWN parity gaps to close first (see Mistake Log): ISSUE-1 download stall timeout (`download.go`) and ISSUE-2 partition-node wait (`run.go`); do not retire the shell until both are closed and QEMU-proven |
| R-3 | Arch coverage (amd64 + arm64) | one arch fails to boot | cross-compile and QEMU-test both arches |
| R-4 | Binary size bloats the PXE/TFTP-transferred initrd | gzipped initrd exceeds the ceiling | a `CGO_ENABLED=0` static binary with `net/http`+`crypto/sha256`+netlink+dhcp is multi-MB; **771KB parity is impossible**, so set a real ceiling: **≤ 8 MB gzipped** (well within PXE/TFTP limits; production initramfs routinely run tens of MB). The dedicated `ze-installer` build excludes the BGP/network-OS surface; measure the gzipped cpio and assert against the ceiling (no DNS/hostname path needed: `ze.server` is an IPv4 literal, so the pure-Go resolver is never invoked) |
| R-5 | DHCP raw-socket binding fails in the minimal initrd env | DHCP returns no lease in QEMU | reuse the exact `iface/dhcp` setup; fall back to probing the kernel `ip=dhcp` lease first (already the flow) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Kernel execs `/init` (cmd/ze-installer) | → | `cmd/ze-installer` `main` → `disk.RunInitrd()` | `make ze-install-qemu-test` boots the Go initrd and completes an HTTP install end-to-end |
| Kernel cmdline `ze.source=iso` | → | `runISO` (incl Ventoy loop mount) | `make ze-install-iso-qemu-test` boots the Go initrd and completes an ISO install |
| FATAL with `ze.shell-auth` set | → | Go recovery console gate | QEMU forced-fatal evidence: password prompt on the console, correct password opens the menu |
| FATAL, no `ze.shell-auth`, network source | → | `fatal` branch (c) → 30s reboot | QEMU forced-fatal evidence: no console, message, reboot (no hang) |
| Forced panic in a goroutine | → | top-of-goroutine `recover` → FATAL path | QEMU fault-injection: ends in recovery/reboot, NOT kernel panic (R-6) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Build the initrd | The cpio contains a single static Go binary as `/init` and NO busybox or other binary (`grep`/`ls` proof) |
| AC-2 | HTTP PXE boot in QEMU | Image downloaded + SHA256-verified + written + DB injected on part4 + reboot; zero external-binary exec |
| AC-3 | ISO boot in QEMU (direct + Ventoy) | Image written + poweroff |
| AC-4 | Multi-homed target, `ze.mac` set | The matching NIC is pinned and DHCP'd; a foreign NIC is never brought up; server reachable |
| AC-5 | DHCP recovery needed | Done in Go via `nclient4` + netlink; no `udhcpc`, no lease script. The pinned NIC ends with an address + default route; when the pinned NIC cannot reach the server its stale address/default route is FLUSHED (`netlink.AddrDel`/`RouteDel`) before scanning others, so exactly one working default route remains; a kernel `ip=dhcp` lease is trusted only if the server is reachable over it |
| AC-6 | Probe times out | Counts as unreachable (honest reachability preserved) |
| AC-7 | FATAL, `ze.shell-auth` set | Recovery console gated: `sha256(typed)` compared to `ze.shell-auth` (termios ECHO-off); correct password opens the menu; wrong/empty does not |
| AC-7b | FATAL, no `ze.shell-auth`, ISO source | Recovery console opens **ungated** (operator physically present) |
| AC-7c | FATAL, no `ze.shell-auth`, network source | **No** console; message printed, ~30s wait, then reboot. Never hangs (the unattended-box safety valve) |
| AC-8 | Any console configuration | Installer output appears on every active console |
| AC-9 | Repo state after retire phase | `tools/installer-initrd/init`, its tests, and `udhcpc.script` deleted; `cmd_initrd.go` + Makefile + provision build/stage the Go initrd; `defaultInitrdVersion` bumped (`cmd_initrd.go:19`, "v1"→"v2") so a cached/remote v1 (shell) initrd is not reused after the rewrite |
| AC-10 | Static check | No `exec.Command`/`runCmd` of an external binary remains in `internal/install/disk` or `cmd/ze-installer` (`grep` proof) |
| AC-11 | Initrd packaging is pure Go | `defaultInitrdMakeBuild` produces the cpio + gzip in-process: `compress/gzip` (stdlib) + a hand-rolled newc cpio writer (single `init` entry, mode 0755). No `cpio` or `gzip` exec in the build path (`grep` proof). Only `go build` (the toolchain) remains an exec, and it is not part of the initrd. The QEMU harness builds via this same production path, not a re-implementation |

## End-to-End User Stories (MANDATORY)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | PXE-boots a blank multi-homed box | iPXE → kernel+Go initrd → bootstrap → ze.mac pin → DHCP-in-Go → download → write → inject → reboot | `make ze-install-qemu-test` (multi-NIC variant) |
| 2 | Boots an appliance ISO / Ventoy USB | Go initrd → find ISO (loop ioctl) → write → poweroff | `make ze-install-iso-qemu-test` |
| 3 | Hits a fatal install error onsite | fatal → branch by trust context (credential→gated / ISO→ungated / network-no-cred→30s reboot) → recovery console + termios password → menu | QEMU forced-fatal evidence script (all three branches) |
| 4 | Has a single isolated NIC (fast path) | kernel `ip=dhcp` reachable → no DHCP recovery → install | `make ze-install-qemu-test` (single-NIC) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseCmdlineMacAuth` | `internal/install/disk/cmdline_test.go` | `ze.mac`/`ze.shell-auth` parsed into config | |
| `TestIfaceForMac` | `internal/install/disk/network_test.go` | NIC selected by MAC, case-insensitive, lo skipped | |
| `TestApplyLease` | `internal/install/disk/netlink_test.go` | address + default route applied AND a stale prior address/default route flushed (`AddrDel`/`RouteDel`) before re-apply (netlink faked) | |
| `TestRecoveryAuth` | `internal/install/disk/rescue_test.go` | `sha256(typed)==ze.shell-auth` opens console; wrong/empty does not (termios + start stubbed); does NOT double-hash `ze.shell-auth` | |
| `TestFatalPolicy` | `internal/install/disk/rescue_test.go` | three-branch selection: credential→gated, no-cred+ISO→ungated, no-cred+network→30s-reboot (reboot fn stubbed); covers AC-7/7b/7c | |
| `TestMountWrappers` | `internal/install/disk/mount_test.go` | mount/umount call `unix.Mount`/`Unmount` with right args (syscall faked behind iface) | |
| `TestLoopAttach` | `internal/install/disk/loop_test.go` | loop attach/detach via ioctl (faked) | |
| `TestDownloadStallTimeout` | `internal/install/disk/download_test.go` | a body that connects then stalls returns an error within the stall timeout instead of hanging (`httptest` server that stops writing); a steady slow transfer is NOT killed (ISSUE-1) | |
| `TestWaitForPartitionNode` | `internal/install/disk/run_test.go` | poll returns once the node appears; FATALs after the bound if it never does (stat faked) (ISSUE-2) | |
| `TestWriteInitrdPack` | `internal/appliance/initrd_pack_test.go` | the produced file gunzips with `compress/gzip` and the cpio extracts to exactly one entry `init` with mode `0100755` and the original bytes; newc header well-formed; ends with `TRAILER!!!` (AC-11) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ze.port | 1-65535 | 65535 | 0 | 65536 |
| ze.media-id | 32 hex | 32 chars | 31 | 33 |
| sha256 | 64 hex | 64 chars | 63 | 65 |
| netmask prefix | 0-32 | 32 | N/A | 33 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ze-install-qemu-test` | `scripts/evidence/effective-install-qemu.py` | HTTP PXE install completes in a QEMU VM on the Go initrd | |
| `ze-install-iso-qemu-test` | `scripts/evidence/effective-install-iso-qemu.py` | ISO install completes in a QEMU VM on the Go initrd | |

The installer initrd has no `.ci` runner; its functional surface is the QEMU
evidence harness (the established pattern, `mk/test-integration.mk`), which is
MANDATORY here per `ai/rules/qemu-testing.md`.

### Test Infrastructure to Build (NOT a trivial "point at the Go initrd")

Verified against the existing harness, several ACs need NEW QEMU capability that
does not exist today. These are in scope (the ACs are not negotiable), but each is
real work, not a one-line variant. Budget for them:

| Capability | Why (AC) | Current harness reality | Verified at |
|------------|----------|-------------------------|-------------|
| Multi-NIC QEMU topology with explicit MACs + a second (foreign) network; `ze.mac` on the cmdline | AC-4 (pin, foreign NIC never up), AC-5 (flush/route), Story 1 | **single NIC** (`net0` only), cmdline has no `ze.mac` | `scripts/evidence/effective-install-qemu.py:386,397-419` |
| Ventoy disk-image fixture (exFAT/NTFS data partition + nested ISO/loopback layout) | AC-3 (Ventoy) | **no Ventoy**; direct-ISO only | `effective-install-iso-qemu.py` (no ventoy/exfat/ntfs refs) |
| Interactive serial-console driver (pexpect-style): send a password, assert the menu, assert NO menu on wrong/empty | AC-7/7b/7c, Story 3 | **none** (no fatal/rescue/auth path tested) | `effective-install-qemu.py` (only `panic=-1` on cmdline) |
| Forced-panic fault injection (build tag / cmdline flag that panics a goroutine mid-install) | R-6 | **none** | as above |
| arm64 ISO QEMU proof | the both-arches goal gate | HTTP supports arm64 (`:73-79`); **ISO proof is amd64 UEFI only** | `effective-install-iso-qemu.py:33-36` |
| Stall-injection: a server that accepts the connection then stops sending mid-image | ISSUE-1 (download stall must FATAL/retry, not hang) | **none**; `streamClient` has no timeout (`download.go:27`) | n/a (new) |
| Partition-readiness assertion: mount of part-4 succeeds even on a cold devtmpfs | ISSUE-2 (BLKRRPART→mount race) | **none**; Go mounts immediately (`run.go:127→132`) | n/a (new) |

If any of these cannot be built, the corresponding AC cannot be demonstrated and the
spec must NOT be closed (see Goal Gates). Do not silently downgrade an AC to a unit
test; `ai/rules/qemu-testing.md` forbids "needs hardware" excuses.

### Interop Tests
- N/A with justification: this is not a wire protocol. DHCP client behavior is provided by `insomniacslk/dhcp` (already interop-proven by the production daemon).

## Files to Modify
- `internal/install/disk/network.go` - netlink link-up + `nclient4` DHCP + `ze.mac` pinning + apply lease; delete `ip`/`udhcpc` shell-outs.
- `internal/install/disk/cmdline.go` - parse `ze.mac`, `ze.shell-auth`.
- `internal/install/disk/iso.go` - `unix.Mount`/loop ioctls/`unix.Mknod` replace mount/losetup/mknod.
- `internal/install/disk/system.go` - `unix.Mount`/`Unmount`/`Sync`/`Reboot` replace mount/umount/sync/reboot/poweroff.
- `internal/install/disk/run.go` - `BLKRRPART` ioctl + `unix.Sync`; **bounded poll for the part-4 node after `BLKRRPART` before `mountInjectDB`** (ISSUE-2, mirroring `init:1122-1127`); add the PID-1-capable entry (`RunInitrd`).
- `internal/install/disk/download.go` - add a **stall/no-progress timeout** to the streaming image download (ISSUE-1); `streamClient` currently has no timeout (`:27`) and `io.Copy` (`:134`) hangs on a wedged transfer. Use a read deadline reset per read or a bytes-written watchdog; keep total time unbounded for legitimate multi-GB transfers.
- `internal/install/disk/validate.go` - validators for `ze.shell-auth` (64 hex) if not reused.
- `internal/appliance/cmd_initrd.go` - `defaultInitrdMakeBuild` packs the initrd in **pure Go**: write a newc cpio (single `init` entry, mode `0100755`) through a `compress/gzip` writer to `destPath`; replace the `exec.Command("cpio", ...)` (`:160`) and `exec.Command("gzip")` (`:194`). Only the `go build` exec (`:146`, the toolchain) stays. `checkInitrdBuildTools` then has nothing external to check for packing (drop `cpio`/`gzip`; `busybox` already gone). Keep the v2 bump + arch-keyed cache. Set the gzip header `ModTime`/`Name` to zero for reproducible output.
- `internal/plugins/provision/staging.go`, `main.go` - stage the Go initrd.
- Build wiring for the new `cmd/ze-installer` binary (its own `ze_installer` build tag + cross-compile target in the Makefile). NOTE: this is a separate binary, NOT a `cmd/ze` feature; do not add it to `cmd/ze` feature registration. Verify the actual build-tag/target mechanism during the Audit stage before editing.
- `mk/test-integration.mk` - point the install QEMU targets at the Go initrd.
- `scripts/evidence/effective-install-qemu.py`, `effective-install-iso-qemu.py` - the QEMU harness now builds the Go initrd and asserts the slog markers (done). FOLLOW-UP (AC-11): once `cmd_initrd.go` packs in pure Go, change `build_initrd` to invoke the production builder (`bin/ze-setup appliance initrd`) rather than re-implementing cpio/gzip in Python, so the test boots the exact artifact that ships and `cpio`/`gzip` are not required on the test host either.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | installer reads kernel cmdline, not config |
| CLI commands/flags | Yes (build target) | `cmd/ze-installer`, Makefile |
| Doctor check for runtime dependencies | Yes | a new dependency surface (loop ioctls, netlink in initrd) - add a `ze doctor` note per `ai/rules/doctor-checks.md` if a host-facing check applies; initrd-only paths are validated by QEMU |
| Env var registration | No | cmdline-driven |
| Prometheus counters | No | one-shot installer |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/ze-install.md`, `docs/guide/appliance.md` (busybox-free initrd) |
| 3 | CLI command added/changed? | Yes | document `ze-installer` build / `ze install disk` relationship |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (QEMU install gates on the Go initrd) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/*` installer/appliance docs; update `ai/rules/initrd-no-external-tools.md` to reflect busybox removal |
| 16 | Changed source referenced by doc anchors? | Yes | grep `docs/` for anchors on the changed files |

## Files to Create
- `cmd/ze-installer/main.go` (+ build-tag file) - PID-1 entry; calls `disk.RunInitrd()`.
- `internal/install/disk/bootstrap_linux.go` - mount proc/sys/dev, console fan-out, reaping.
- `internal/install/disk/mount_linux.go` - `unix.Mount`/`Unmount` helpers.
- `internal/install/disk/loop_linux.go` - loop device ioctls.
- `internal/install/disk/blockdev_linux.go` - `BLKRRPART`/`BLKGETSIZE64` ioctls.
- `internal/install/disk/netlink_linux.go` - link up + address/route apply.
- `internal/install/disk/dhcp_linux.go` - `nclient4` single-shot DORA per NIC.
- `internal/install/disk/console_linux.go` - multi-console writer (reads `/proc/consoles`, opens each `/dev/ttyX`). The `slog` handler MUST be installed to write through this writer: the engine reports all progress via `slog.Info/Warn/Error` (`run.go`, `network.go`), so AC-8 ("output on every console") holds only if slog output is fanned out, not just ad-hoc prints.
- `internal/appliance/initrd_pack.go` - pure-Go initrd packer used by `cmd_initrd.go` (AC-11): a function that takes the built `init` binary path + dest and writes a newc cpio (single `init` entry, `0100755`, then the `TRAILER!!!` record) through a `compress/gzip` writer. newc format reference: 6-byte magic `070701`, 13 × 8-hex-digit header fields (ino, mode, uid, gid, nlink, mtime, filesize, devmajor/minor, rdevmajor/minor, namesize, check=0), NUL-terminated name, 4-byte alignment padding after the header+name and after the data. No external `cpio`/`gzip`, no new dependency.
- `internal/install/disk/rescue_linux.go` - Go recovery console: three-branch `fatal` policy + termios `sha256(typed)==ze.shell-auth` auth + menu; the never-exit FATAL sink (also the goroutine-panic recovery target, R-6). NON-TRIVIAL concurrency: the shell offers the session on EVERY active console at once (`init:173-189` forks a reader per `$CONSOLES`, then `wait`). The Go port needs a goroutine per console doing a blocking termios read, first-to-authenticate wins, the rest cancelled; after all sessions end it reboots/poweroffs. This is the place R-6's per-goroutine `recover` matters most.
- `*_test.go` alongside each.

### Files to Delete (retire phase)
- `tools/installer-initrd/init`, `tools/installer-initrd/udhcpc.script`, `tools/installer-initrd/test/*.sh`, the busybox cpio path in the Makefile.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-install-qemu-test && make ze-install-iso-qemu-test` |
| 7-13 | Critical/Security review, re-verify |
| 14. Present summary | Executive Summary |

### Implementation Phases

Each phase ends with a Self-Critical Review and a QEMU gate. The shell init stays
in place until Phase 5 so there is always a working installer.

1. **Phase: Wiring + PID-1 bootstrap (MANDATORY FIRST)** - `cmd/ze-installer` build tag; `bootstrap_linux.go` mounts proc/sys/dev + console fan-out; calls the existing `disk.Run` logic (still allowed to shell out this phase). Cross-compile static; cpio it (busybox kept as fallback this phase only).
   - Tests: QEMU boot test proves `/init` runs and reaches install logic.
   - Verify: `/init` does not exit; reaches `findTargetDisk`.
2. **Phase: Syscall swaps + parity fixes** - replace mount/umount/sync/reboot/poweroff/blockdev/mknod/losetup with `unix.*`/ioctls; remove those `runCmd` calls. Close the two parity gaps on this path: ISSUE-2 bounded poll-for-part-4-node after `BLKRRPART` (`run.go`), ISSUE-1 stall timeout on the streaming download (`download.go`).
   - Tests: unit (faked syscalls, `TestDownloadStallTimeout`, `TestWaitForPartitionNode`) + QEMU HTTP install still green + the stall-injection and partition-readiness QEMU cases.
   - Verify: `grep` shows no mount/umount/sync/reboot/blockdev/losetup/mknod exec; a wedged download FATALs/retries (no hang); part-4 mount succeeds on a cold devtmpfs.
3. **Phase: Network in Go** - `cmdline.go` gains `ze.mac`/`ze.shell-auth`; `netlink.LinkSetUp` + `nclient4` single-shot DHCP + apply addr/route; `ze.mac` pinning; remove `ip`/`udhcpc`.
   - Tests: unit (iface-for-mac, lease apply faked) + QEMU multi-homed install.
   - Verify: no `ip`/`udhcpc` exec; lease applied (addr+route present in QEMU).
4. **Phase: Rescue console + auth** - termios ECHO-off prompt; compare `sha256(typed)` to `ze.shell-auth` (which IS the digest, so do not double-hash); Go recovery menu; multi-console; three-branch `fatal` policy (credential→gated, no-cred+ISO→ungated, no-cred+network→30s reboot); top-of-goroutine `recover` so a panic routes to FATAL not a kernel panic (R-6).
   - Tests: unit (`TestRecoveryAuth`, `TestFatalPolicy`, stubbed termios/reboot) + QEMU forced-fatal evidence (all three branches) + QEMU panic fault-injection.
   - Verify: wrong/empty password opens no console for HTTP installs; no-credential network install reboots after 30s (never hangs); forced panic ends in recovery, not kernel panic.
5. **Phase: Retire shell + rewire** - delete shell init + tests + `udhcpc.script`; update `cmd_initrd.go` + Makefile to build the Go-binary initrd (no busybox); update provision staging; remove busybox from the cpio.
   - Tests: full QEMU HTTP + ISO gates on the busybox-free initrd.
   - Verify: AC-1 (no busybox in cpio), AC-9, AC-10.
6. **Full verification + docs + learned** - `make ze-verify` + QEMU gates; docs; `plan/learned/NNN-installer-initrd-pure-go.md`.

### Failure Routing
| Failure | Route To |
|---------|----------|
| QEMU boot panic | PID-1 bootstrap phase; ensure no exit path |
| DHCP no lease in QEMU | Network phase; compare against `iface/dhcp` setup |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (initial framing) "the initrd is the only piece still shell; no Go installer exists" | `internal/install/disk` is a near-complete Go installer (`ze install disk`), just unwired into the initrd | survey of `cmd/ze/install` + `cmd_initrd.go` | reframed scope from "write a Go installer" to "wire the existing engine into a PID-1 initrd, reach parity, delete the shell" |
| "the Go engine is at shell parity; reuse it as-is" | The Go engine is LESS robust than the shell on two PID-1-critical points: (1) the image download has no stall timeout (`download.go:27`) where the shell used `wget -T` (`init:540`) → can hang forever; (2) it mounts part-4 immediately after `BLKRRPART` (`run.go:127→132`) where the shell waits + checks `-b` (`init:1122-1127`) → devtmpfs race | ISSUE-1/ISSUE-2, critical review round 4 | concrete parity gaps to close before retiring the shell (sharpens R-2 from a worry into closeable items); `download.go` + `run.go` added to Files to Modify |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Dedicated `cmd/ze-installer` static binary as `/init` -- a **single PID-1 binary** that does the install inline; internal goroutines are acceptable (user decision, PXE installer) -- sharing `internal/install/disk` | Embed full `ze`; extend `ze install disk` to self-bootstrap as PID 1; **reuse gokrazy init (the appliance's PID 1) with ze-installer as a supervised child** | keeps the PXE/TFTP initrd small (excludes the BGP/network-OS surface) while reusing one engine; `ze install disk` stays the booted-system entry point. The gokrazy-init route is **NOT viable on the pinned gokrazy** (verified in the module cache): `gok` emits only a partitioned disk image -- MBR + SquashFS root written to a partition/path (`tools .../internal/gok/overwrite.go:48-49`, `internal/packer/write.go:22-23`) -- and gokrazy init mounts its root from a disk partition by PARTUUID (`gokrazy/gokrazy.go:148`) and kexecs with `KEXEC_FILE_NO_INITRAMFS` (`reboot_amd64.go:34`). There is no from-RAM/netboot artifact (grep for `netboot|initramfs|pxe|tftp` in `gok`: zero hits), and the installer runs before any disk exists. So the appliance's supervise-a-Go-program model cannot be reused; panic containment is in-process via top-of-goroutine `recover` (R-6), not a separate supervisor. |
| Go recovery console for rescue (zero busybox) | Keep one busybox `sh` solely for rescue | user-approved; the only way to truly reach "no busybox"; auth + diagnostics menu covers the onsite-recovery need |
| DHCP via `nclient4` + netlink apply | Keep `udhcpc` + a lease script | eliminates the external client and the `-s` lease-script class of bug structurally; library already used in-repo |
| Pure-Go initrd packaging: `compress/gzip` + hand-rolled newc cpio | exec `cpio` + `gzip`; or add a cpio dependency (`github.com/cavaliergopher/cpio`) | `compress/gzip` is stdlib and already used (`iso.go`, `cmd_iso.go`, `support.go`); the initrd is ONE file, so a newc writer is ~40 lines and needs NO dependency. Removes the `cpio --quiet` GNU-vs-BSD portability trap (BSD `cpio` on macOS rejects `--quiet`, breaking local runs) and lets the QEMU harness build via the production path instead of duplicating the packing in Python. `go build` stays an exec: the toolchain cannot be replaced and is not in the initrd |

## Known Limitations
- The recovery console is a fixed menu, not a general shell; intentional (no arbitrary command execution onsite, smaller attack surface).
- IPv6-only install networks are out of scope (HTTP install assumes IPv4 `ze.server`, matching the current shell).

## Review Gate

Status: spec drafted and scoped; NOT yet implemented. Run `/ze-review` after Phase 5.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (pending implementation) | | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 (including AC-7b and AC-7c) all demonstrated
- [ ] End-to-End User Stories: every story has a working path + passing QEMU test
- [ ] Wiring Test table complete
- [ ] **Both amd64 and arm64 QEMU-boot + install** (hardware-boot code; an arch that does not boot is a goal failure). HTTP arm64 already works in the harness (`effective-install-qemu.py:73-79`); the **arm64 ISO proof must be built** (`effective-install-iso-qemu.py:33-36` is amd64 UEFI only today) -- see Test Infrastructure to Build
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify` + QEMU install gates pass
- [ ] Shell init + udhcpc.script deleted; initrd build rewired
- [ ] Documentation Update Checklist answered with source evidence

### Quality Gates (SHOULD pass)
- [ ] No external-binary exec remains (AC-10 grep)
- [ ] Gzipped initrd size measured and within the R-4 ceiling (≤ 8 MB gzipped)

### Design
- [ ] No premature abstraction
- [ ] Single engine, two entry points (initrd binary + `ze install disk`)
- [ ] Syscalls isolated in named `_linux.go` helpers

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for numeric inputs
- [ ] QEMU functional gates green
- [ ] Goal Validation table filled

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary + Audit filled
- [ ] Learned summary `plan/learned/NNN-installer-initrd-pure-go.md`
- [ ] Commit A: code + tests + docs + spec + learned; Commit B: `git rm` spec

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| Busybox-free initrd installs over PXE | QEMU functional | `make ze-install-qemu-test` on the Go initrd (pending) |
| Busybox-free initrd installs from ISO | QEMU functional | `make ze-install-iso-qemu-test` (pending) |
| Zero external binaries | static grep | no `exec`/`runCmd` of external tools (pending) |
| Onsite rescue gated | QEMU forced-fatal | password prompt + menu (pending) |
