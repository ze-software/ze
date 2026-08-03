# 907 -- Appliance Install Robust

## Context

A real Intel N150 appliance bricked silently: `injectZeFS` trusts `debugfs -R` exit codes, but debugfs exits 0 on internal failure, so a failed DB injection ships a broken image. On boot, `/perm/ze/` is absent, `zefs.Create` fails (no MkdirAll of parent), `NewBlob` errors, the blob gate trips, gokrazy restart-loops. Two fixes: build-side verification catches silent injection failures before shipping, and a runtime auto-init fallback un-bricks boxes whose `/perm/ze` is missing. A Go-native installer (`ze install disk`) replaces the 884-line busybox shell script for the on-device install path.

## Decisions

- Chose `bytes.Contains` for build-side verification over debugfs dump read-back, because it is pure Go with no shell-out and catches the exact failure mode (source bytes absent from perm image). Rejected debugfs dump (shells out, requires mock command-string parsing in tests).
- Replaced `dd` with Go `ReadAt`/`WriteAt` (`extractPartition`/`writePartition`) over keeping dd via `runExternalFn`, because dd is trivially implementable in Go and eliminates a shell dependency.
- Chose `ze.gokrazy.enabled` env var as the auto-init gate over a new config surface, because it is a bootstrap-time setting (needed before config loads) already registered and set on the appliance.
- Fallback auth posture: connectivity-only (A1) over random printed password (A2), matching existing serial fail-open recovery philosophy (`appliance.md`). No SSH/web credential keys written.
- Go installer registers as `ze install disk` under the existing `cmd/ze/install` subdispatch over a new binary, reusing the established subcommand pattern.
- Supersedes learned 813 ("no Go in initrd"): ze already ships a Go binary; the initrd calls `ze install disk` instead of reimplementing in shell.

## Consequences

- Build-side: `injectZeFS` now hard-fails if the database bytes are not found in the perm image after debugfs writes. Broken ISO images can no longer ship silently.
- Runtime: a gokrazy appliance with missing `/perm/ze` auto-initializes to a reachable-but-unprovisioned state instead of brick-looping. Operator must re-provision (by design).
- `dd` is no longer used in the build path; `extractPartition`/`writePartition` are pure Go.
- `e2fsck` structural check and loopback mount verify are additional defense layers (best-effort and root-only respectively).
- The Go installer (`internal/install/disk/`) has full validator parity with the shell script, HTTP download with SHA256 + retry, and ISO media detection including Ventoy.

## Gotchas

- `debugfs -R` exits 0 even when its subcommand fails internally. `runExternal` uses `cmd.Output()` which only checks exit code. This is the root cause of the silent build failure; any future debugfs invocation must verify results independently.
- gokrazy does NOT reformat `/perm`. It mounts-or-skips, leaving `/perm` read-only on failure. The initial assumption ("gokrazy wipes /perm") was false and reframed the fix from "prevent reformat" to "ensure runtime-mountability".
- Build and on-device inject are DIFFERENT mechanisms (file offset vs block device). Only verify/validators/codebase are shared, not one literal inject function.
- The pre-tool hooks enforce `textbuf` over `fmt.Sprintf`/string-concat and `slog` over `fmt.Fprintf(os.Stderr)` in new `internal/` files. Existing code in `cmd_build.go` is grandfathered but new edits that include existing lines in the diff get checked too.
- `hcsshim/ext4/tar2ext4` contains a usable pure-Go ext4 writer (`compactext4`), but it is an `internal/` package and importing it pulls the full Windows container runtime. Not viable without forking the ~1500 lines.

## Files

- `internal/appliance/diskverify.go` (new): V3 verify, extractPartition, writePartition
- `internal/appliance/diskverify_test.go` (new): 7 tests
- `internal/appliance/cmd_build.go` (modified): Go I/O replacing dd, verifyInject call
- `internal/appliance/cmd_build_test.go` (modified): 3 new tests
- `cmd/ze/ze_core_autoinit.go` (new): gokrazyAutoInit
- `cmd/ze/ze_core_autoinit_test.go` (new): 2 tests
- `cmd/ze/ze_core_start.go` (modified): blob gate fallback
- `internal/component/config/environment.go` (modified): description update
- `internal/install/disk/` (new package): validate, cmdline, detect, download, iso, run, system, register
- `internal/install/disk/*_test.go` (new): 24 tests
- `cmd/ze/setup_features_setup.go` (modified): import disk installer
