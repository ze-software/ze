# Appliance Serial Console Login

The gokrazy serial console (115200 baud on ttyS0) is the emergency access path.
Without a gate it hands out a root-equivalent shell to anyone with physical
access. ze authenticates the serial session against the same local admin
credentials the SSH and web logins use.

<!-- source: cmd/ze/login.go -- argv[0] detection, credential load, bcrypt check, exec into the shell -->
<!-- source: cmd/ze-serial-shell/main.go -- wellKnownSerialShell, the symlink, the GOKRAZY_FIRST_START exit 125 -->

## ze is the gate, busybox stays the shell

Embedding u-root utilities in the ze binary was refused. u-root commands are
`package main` and cannot be imported as libraries without gobusybox AST
rewriting, which introduces `os.Exit` inside dispatch and shared global state.

The busybox binary is renamed to `ze-recovery-shell` at build time. The root
filesystem is read-only SquashFS, so a rename at runtime is impossible, and
keeping the original name leaves a path that bypasses the gate. Only the login
handler knows the renamed path.

The rename is delivered by a ze-owned gokrazy package, `ze-serial-shell`,
replacing `serial-busybox`. Overwriting the stock package's symlink at ze
startup would leave a race window between boot and startup, and the ze-owned
package controls both the binary name and the symlink target.

## Fail open when the credential store is missing

A missing or unreadable ZeFS store lets the operator in. The serial console is
the last-resort recovery path, and a corrupt credential database must not lock
the operator out of the box it is meant to rescue.

## Placement constraints

- argv[0] detection sits in `ze_core_dispatch.go` `binarySetup`, not in
  `main.go`. `main.go` carries no build tag, so the check would fire for every
  binary personality, including ze-test and ze-chaos.
- `DontStartOnBoot` (exit 125 when `GOKRAZY_FIRST_START=1`) is inlined rather
  than imported. `github.com/gokrazy/gokrazy` is not vendored in the main
  module, and vendoring it for a three-line function pollutes the dependency
  tree and breaks the module lint.
- Per-package gokrazy environment variables are not present when ze is executed
  as the login shell through `runWithCtty`, so the handler falls back to the
  `/perm/ze` path.
- gokrazy re-invokes ze on every serial session, including after busybox exits,
  which gives re-authentication per session with no extra code.
- The renamed busybox path appears in both `cmd/ze/login.go` and
  `cmd/ze-serial-shell/main.go`. They are separate modules, so the constant
  cannot be shared: a change to that path needs both files.

## Two environment facts

`env.Get("ze.config.dir")` reads the ze env registry, not `os.Getenv`, so a
test sets it with `env.Set` rather than `t.Setenv`, and the key must be
registered.

`syscall.Exec` exists on every Unix platform, so the login handler compiles on
darwin and needs only the `ze_core` build tag, not a Linux one.
