# 910 -- Installer initrd console visibility, foreign-DHCP recovery, and two reusable gotchas

## Context

A bare-metal PXE install fetched kernel+initrd over iPXE but never wrote the disk.
Two defects: (1) installer progress/FATAL output was invisible because userspace
stdio followed a `/dev/console` that the generated `console=` ordering could point
at a dead device; (2) `ensure_network()` trusted a kernel `ip=dhcp` default route,
so when a foreign/corporate DHCP won the race on a non-isolated provisioning
network the install probed an unreachable server then dropped to a shell nobody
could see. Fixed in `tools/installer-initrd/init` plus arch-aware `console=` in the
generated `boot.ipxe` (`internal/plugins/imageserver/handler.go`).

## Decisions

- `init` fans every `log`/`fatal` line to all consoles in
  `/sys/class/tty/console/active` (helper `emit`), and binds the rescue shell to a
  real, serial-preferred console (`debug_console`) instead of inheriting PID 1's
  possibly-dead stdio.
- `ensure_network` re-validates a kernel default route against an HTTP probe of
  `ze.server` (`server_reachable`); an unreachable lease falls through to
  per-interface DHCP that flushes interfaces with no route to the server.
- `boot.ipxe` selects `console=` per client arch via iPXE `${buildarch}` so x86
  drops the ARM-only `ttyAMA0` (which never registers on x86 and can dead-end
  `/dev/console`).

## Reusable gotchas (the point of this entry)

- **A POSIX shell function whose last executed command is a conditional can abort
  PID 1 under `set -e`.** `setup_console`'s final statement was a `for` loop whose
  body `[ -c "/dev/$c" ] && CONSOLES=...` returns non-zero when the last console
  has no `/dev` node. Run bare under the script's global `set -e`, that non-zero
  return propagated and killed the installer before `parse_cmdline`. Fix: end such
  functions with an explicit `return 0`. Verified empirically (`set -e` subshell
  aborted before the fix, survived after).

- **The orchestrated `.ci` runner (`internal/test/runner/runner_exec.go`,
  `runOrchestrated`) does NOT evaluate `expect=stdout:regex=`.** It enforces only
  `expect=exit:code` (last `cmd` via `ExpectExitCode`; earlier `cmd`s via the
  inline `proc.Wait()` error path) and `expect=stdout:contains=`/`!contains=`
  against the *combined* client output. A `:regex=` line is parsed but silently
  inert in this runner (the ParsingRunner in `parsing.go` does honor it, but the
  install/bgp/etc. suites use the encoding runner). When wiring shell unit tests
  into an orchestrated `.ci`, rely on the wrapper script's exit code (`exit 1` on
  any failed assertion) -- that is the gate that actually fires. Confirmed by
  reading the runner and empirically (a deliberately broken `:regex=` passed; a
  non-zero `exit` failed).

## Verification surfaces added

- New `test/test-console.sh` (console fan-out + `set -e` regression) and
  `test/test-applets.sh` (guards that every applet `init` uses is symlinked in the
  Makefile -- `printf` had regressed this), wired into both the Makefile `test`
  target and `test/install/initrd-flow.ci` (which previously ran only 5 of the
  shell suites).

## Mistakes

- The first `setup_console`/`debug_console` design (and the original handoff) shipped
  the `set -e` abort latent; only an adversarial review pass caught it. Default to
  `return 0` on any shell helper called bare under `set -e`.
- `printf` was used by `init` but absent from the Makefile's explicit applet list,
  violating the documented defense-in-depth invariant. `test-applets.sh` now guards
  the whole list.

## Files

None recorded.
