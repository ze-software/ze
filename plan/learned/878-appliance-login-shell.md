# 878 -- appliance-login-shell

## Context

The gokrazy appliance serial console was unauthenticated: anyone with physical serial access got a busybox shell immediately. The serial console is emergency-only (115200 baud on ttyS0), but on deployed hardware it represents an unprotected root-equivalent access path. The goal was to gate serial access behind the same local admin credentials used for SSH and web login, while keeping the serial console as a last-resort recovery path when the credentials database is missing.

## Decisions

- Chose ze as login gate with busybox staying as the shell over embedding u-root utilities in the ze binary, because u-root commands are `package main` and cannot be imported as Go libraries without gobusybox AST rewriting which introduces `os.Exit` in dispatch and global state contamination
- Chose fail-open when ZeFS is missing over fail-closed, because the serial console is the last-resort recovery path and a corrupted database must not lock out the operator
- Chose renaming the busybox binary to `ze-recovery-shell` at build time over keeping the original name, because the read-only SquashFS root prevents runtime rename and the original name would allow direct access bypassing auth
- Chose a ze-controlled gokrazy package (`ze-serial-shell`) over keeping `serial-busybox` and overwriting its symlink at ze startup, because the own package eliminates the race window and gives full control over both the binary name and symlink target
- Chose argv[0] detection in `ze_core_dispatch.go` `binarySetup` over detection in `main.go`, because main.go has no build tag and would fire on all binary personalities (ze-test, ze-chaos, etc.)
- Chose inlining `DontStartOnBoot` (exit 125 on `GOKRAZY_FIRST_START=1`) over importing `github.com/gokrazy/gokrazy`, because the gokrazy package is not in the main module vendor directory and adding it would pollute the dependency tree for a three-line function

## Consequences

- Serial console access now requires local admin credentials (username + bcrypt password from ZeFS)
- The `serial-busybox` gokrazy package is replaced by `ze-serial-shell` in `gokrazy/ze/config.json`
- The busybox binary lives at `/usr/local/bin/ze-recovery-shell` (not the standard `busybox` name); only ze's login handler knows this path
- Per-package gokrazy env vars (like `ze.config.dir`) are NOT available when ze is exec'd as ash via `runWithCtty`, so the login handler falls back to the hardcoded `/perm/ze` path
- Gokrazy's `tryStartShell` re-invokes ze on every serial session (including after busybox exit), providing re-authentication per session
- The renamed busybox path is duplicated in `login.go` and `ze-serial-shell/main.go` because they are in separate modules; this is a maintenance risk requiring grep-based coordination on changes

## Gotchas

- `gokrazy.DontStartOnBoot()` is trivially simple (exit 125 if `GOKRAZY_FIRST_START=1`), but importing `github.com/gokrazy/gokrazy` into a package built by the main module lint causes vendor resolution failures; inlining the logic is cleaner
- The gokrazy builddir for ze already covers `cmd/ze-serial-shell` via the replace directive pointing to the repo root; no separate builddir entry is needed because both binaries are in the same Go module
- `env.Get("ze.config.dir")` uses ze's env registry (not `os.Getenv`), so tests must use `env.Set` not `t.Setenv`; the env package also requires the key to be registered (via `internal/core/paths` import)
- Gokrazy's `runWithCtty` uses `exec.Command().Run()` (fork), not `syscall.Exec` (replace); the child gets fresh stdin/stdout/stderr on the TTY device, not inherited file descriptors from gokrazy init
- `syscall.Exec` is available on all Unix platforms (not linux-only), so the login handler compiles on darwin without a linux build tag; only the `ze_core` tag is needed

## Files

- `cmd/ze/login.go` -- login handler: argv[0] detection, ZeFS credential loading, bcrypt validation, fail-open, exec into shell
- `cmd/ze/login_test.go` -- 8 unit tests covering all acceptance criteria
- `cmd/ze/ze_core_dispatch.go` -- wired `isShellInvocation(binaryName())` into `zeSetup`
- `cmd/ze-serial-shell/main.go` -- gokrazy wrapper: symlink creation, DontStartOnBoot
- `cmd/ze-serial-shell/_gokrazy/extrafiles_{amd64,arm64}.tar` -- renamed busybox binary
- `gokrazy/ze/config.json` -- replaced serial-busybox with ze-serial-shell
- `docs/guide/appliance.md` -- updated serial console and "What's in the image" descriptions
- `test/appliance/serial-login.ci` -- functional test placeholder (QEMU required)
