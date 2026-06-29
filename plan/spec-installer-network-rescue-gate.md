# Spec: installer-network-rescue-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-06-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md`, `ai/rules/qemu-testing.md`, `ai/rules/no-partial-completion.md`.
3. `tools/installer-initrd/init` (the busybox installer), `internal/plugins/imageserver/handler.go`, `internal/plugins/provision/main.go`.
4. Field evidence: a multi-homed PXE target booted via iPXE on the install NIC (198.19.255.x) but the kernel `ip=dhcp` configured a *second* office NIC (10.12.104.x), leaving no route to the image server. All HTTP fetches timed out (`bytes=0`), the installer falsely reported "Server reachable", then `fatal`'d into a rescue shell that was bound to serial (invisible on the monitor) and ungated.

## Task

Fix three bugs in the PXE installer so an install completes on a multi-homed target and fails safely:

1. **NIC selection**: the installer must use the same NIC iPXE booted from, not a different (office) NIC the kernel's `ip=dhcp` happened to pick.
2. **Honest reachability**: a wget *timeout* must count as unreachable (it currently false-positives as "Server reachable (got HTTP response)"), which also re-enables the per-interface DHCP recovery in `ensure_network`.
3. **Rescue shell**: on `fatal`, present an interactive shell on **every** active console (monitor + serial), **gated by the admin password** (no shell without auth; fail-closed if no hash configured).

## Required Reading

### Source files read (Current Behavior)
- `tools/installer-initrd/init` — busybox installer.
  → Constraint: sourced for unit tests via `ZE_INIT_NO_MAIN=1 . init`; helper funcs are individually testable (`tools/installer-initrd/test/*.sh`).
  → Constraint: `emit()` (init:65-80) fans every line to all `CONSOLES`; `setup_console` (init:48-63) fills `CONSOLES` from `/sys/class/tty/console/active`.
  → Decision: `fatal()` (init:110-124) execs the shell onto `debug_console` (init:87-104) = first serial console → invisible on tty0. Replace with per-console gated shells.
  → Decision: `server_reachable` (init:685-691) & `wait_for_server` (init:778-787) treat any non-"can't connect" stderr as reachable → a timeout false-positives. Fix: reachable iff rc==0 OR stderr matches `server returned error`.
  → Decision: `ensure_network` (init:693-751) already handles the foreign-DHCP case but is defeated by the false-positive above; add MAC-pinning via `ze.mac`.
  → Constraint: `parse_cmdline` (init:128-166) parses `ze.*` kernel args.
- `internal/plugins/imageserver/handler.go` — `serveBootIPXE` (155-209) emits the dynamic boot.ipxe; kernel line at :200-201 currently has `ip=dhcp`.
- `internal/plugins/provision/main.go` — `ze provision`; has plaintext `--ssh-password` (:35), `hashPassword` bcrypt (:304), `generateConfig` writes the imageserver config block (:165-180).
- `internal/plugins/imageserver/config.go` — parses image-server config (`ssh-password-hash` at :81).
- `internal/plugins/imageserver/yang/ze-image-server-conf.yang` — config leaves.

## Current Behavior (MANDATORY)

**Behavior to preserve:**
- boot.ipxe still emits `ze.server`, `ze.image`, arch-selected `${zeconsole}`, `panic=-1` (handler_test.go:426-442).
- `wait_for_server` still returns reachable on HTTP 404 (test-connectivity.sh Test 8) and unreachable on "can't connect" (Tests 9-10).
- Existing install on a correctly-isolated single-NIC target keeps working.

**Behavior to change (user-approved):**
- Add `ze.mac` + `ze.shell-auth` to the kernel cmdline.
- Reachability: timeout ⇒ unreachable.
- Rescue shell: all consoles, password-gated, fail-closed.
- DHCP recovery: `ensure_network`'s per-interface `udhcpc` now applies the lease
  via an installed handler script, so the NIC actually receives an address +
  default route (busybox udhcpc configures nothing without a `-s` script).

## Data Flow (MANDATORY)

### Entry Point
- Kernel cmdline (`/proc/cmdline`), produced by the iPXE script `serveBootIPXE`
  generates and delivered by firmware/iPXE before the kernel boots. New args:
  `ze.mac=<boot-nic-mac>`, `ze.shell-auth=<sha256-hex>`.
- imageserver config (the `service { image-server { ... } }` block) produced by
  `ze provision generateConfig`; new leaf `shell-auth-sha256`.

### Transformation Path
1. `ze provision --ssh-password <pw>`: plaintext → `sha256(pw)` hex (new) and
   bcrypt (existing) in `internal/plugins/provision/main.go`.
2. `generateConfig` writes `shell-auth-sha256 "<hex>";` into the image-server block.
3. imageserver `parseConfig` (config.go) → `imageConfig.ShellAuthSHA256`.
4. `serveBootIPXE` (handler.go) emits `ze.mac=${mac} ze.shell-auth=<hex>` on the
   kernel line (alongside existing `ze.server`/`ze.image`/`ip=dhcp`).
5. installer `parse_cmdline` → `ZE_MAC`, `ZE_SHELL_AUTH`.
6. `ensure_network` pins the NIC matching `ZE_MAC`; `fatal()` verifies a typed
   password's `sha256sum` against `ZE_SHELL_AUTH` before forking a shell.

### Boundaries Crossed
- Go (provision) → text config → Go (imageserver) → text iPXE script → kernel
  cmdline → busybox shell (installer init). The credential crosses as a sha256
  hex string (never plaintext) on the cmdline and config.

### Integration Points
- `serveBootIPXE` kernel-line assembly (handler.go:197-204) — add two args.
- `parse_cmdline` (init:128-166) — add two `ze.*` cases.
- `ensure_network` (init:832-925) — MAC pinning; `dhcp_acquire` (init:809) wraps
  `udhcpc -s` with the new `udhcpc.script` lease handler installed by the Makefile.
- `fatal` (init:110-124) — gated multi-console shell.

## Wiring Test

Proves the new behavior is reachable end-to-end, not just defined:
- `make -C tools/installer-initrd test` exercises `parse_cmdline`, `ensure_network`,
  `server_reachable`/`wait_for_server`, `fatal`/`verify_shell_auth` via the sourced
  harness — these are the real init functions, not copies.
- `go test ./internal/plugins/imageserver/...` asserts the generated boot.ipxe
  contains `ze.mac=` and `ze.shell-auth=` (so the cmdline the installer parses is
  actually produced).
- `go test ./internal/plugins/provision/...` asserts `generateConfig` emits
  `shell-auth-sha256` (so the imageserver actually receives it).
- Manual QEMU/hardware: a multi-homed PXE boot completes the image write; a forced
  `fatal` shows a password prompt on the monitor.

## 🧪 TDD Test Plan

### Unit Tests
- `test-connectivity.sh`: wget `download timed out` ⇒ `wait_for_server` returns 1
  (unreachable); 404 still returns 0 (regression for AC-2).
- `test-ensure-network.sh`: `iface_for_mac` returns the iface whose
  `address` matches; `ensure_network` with `ZE_MAC` picks it and ignores a
  foreign-net iface (AC-1).
- `test-udhcpc-script.sh`: the udhcpc lease handler converts the dotted netmask
  to a CIDR prefix and applies address + default route via `ip`; deconfig flushes
  only; missing router/IP handled (AC-1 lease application).
- `test-console.sh`: `fatal` spawns one shell per entry in `CONSOLES` (AC-3);
  `verify_shell_auth` accepts the correct password, rejects a wrong one, and
  returns "no shell" when `ZE_SHELL_AUTH` is empty (AC-4).
- `handler_test.go`: boot.ipxe contains `ze.mac=` and `ze.shell-auth=` (AC-5).
- `provision` Go test: `generateConfig` contains `shell-auth-sha256` (AC-6).

### Functional Tests
- The installer initrd has no `.ci` runner; its functional surface is the
  `make -C tools/installer-initrd test` sourced-harness suite (the established
  pattern for this component) plus manual QEMU/hardware PXE validation. No new
  `.ci` file applies.

## Acceptance Criteria

- AC-1 (NIC): with `ze.mac` set, `ensure_network` selects the interface whose `/sys/class/net/*/address` matches and gets a server-reachable lease on it; a second NIC on a foreign network is ignored. The lease is applied by `udhcpc.script` (busybox udhcpc configures nothing on its own). Unit tests in `test-ensure-network.sh` + `test-udhcpc-script.sh`.
- AC-2 (reachability): a wget timeout (`download timed out`) ⇒ `wait_for_server`/`server_reachable` return unreachable; 404 still reachable. New test in `test-connectivity.sh`.
- AC-3 (shell visible): `fatal()` spawns a shell on every console in `CONSOLES`. Unit test in `test-console.sh`.
- AC-4 (shell gated): with `ZE_SHELL_AUTH` set, a correct password forks the shell; a wrong one does not; with it empty, no shell is forked (fail-closed). Unit test.
- AC-5 (boot.ipxe): emits `ze.mac=` and `ze.shell-auth=`; `ip=dhcp` retained as the fast path. `handler_test.go`.
- AC-6 (provision→config): `generateConfig` writes `shell-auth-sha256`; config.go parses it; YANG leaf added. Go tests.
- AC-7: `make test` (installer harness) + `go test ./internal/plugins/imageserver/... ./internal/plugins/provision/...` green; `make ze-lint-changed` clean.

## Files to Modify
- `tools/installer-initrd/init` — parse_cmdline (ze.mac, ze.shell-auth), server_reachable, wait_for_server, ensure_network (+iface_for_mac), fatal (per-console gated shell + verify_shell_auth).
- `tools/installer-initrd/Makefile` — add `stty`, `udhcpc` to applet symlinks; install `udhcpc.script`.
- `tools/installer-initrd/udhcpc.script` (new) — udhcpc lease handler: applies address + default route via `ip`.
- `tools/installer-initrd/test/test-applets.sh` — add stty, udhcpc.
- `tools/installer-initrd/test/test-udhcpc-script.sh` (new) — lease-handler cases (netmask→prefix, address, route, deconfig).
- `tools/installer-initrd/test/test-connectivity.sh`, `test-console.sh`, `test-ensure-network.sh` — new cases.
- `internal/plugins/imageserver/handler.go` — boot.ipxe ze.mac + ze.shell-auth.
- `internal/plugins/imageserver/config.go` + `yang/ze-image-server-conf.yang` — `shell-auth-sha256` leaf.
- `internal/plugins/provision/main.go` — sha256(password) → config.
- Go test files alongside.

## Implementation Steps

1. **AC-2 reachability (init + test)**: add a timeout regression case to
   `test-connectivity.sh`; change `server_reachable`/`wait_for_server` to treat
   only an HTTP reply (`server returned error`/rc 0) as reachable. Run `make test`.
2. **AC-1 NIC pinning (init + test)**: `parse_cmdline` `ze.mac`; add `iface_for_mac`;
   `ensure_network` brings up/DHCPs the matching NIC first. Add `udhcpc.script`
   (lease handler) + `dhcp_acquire` wrapper so the recovery DHCP applies the
   address/route; cover it in `test-udhcpc-script.sh`. Add `test-ensure-network.sh`
   cases. Run `make test`.
3. **AC-3/4 rescue shell (init + Makefile + tests)**: add `verify_shell_auth`
   (stty -echo prompt, sha256sum compare); `fatal()` spawns a gated shell per
   console via `setsid`; fail-closed when `ZE_SHELL_AUTH` empty. Add `stty`,`setsid`
   to Makefile + `test-applets.sh`; add `test-console.sh` cases. Run `make test`.
4. **AC-5 boot.ipxe (handler + test)**: emit `ze.mac=${mac}` and `ze.shell-auth=<hex>`;
   update `handler_test.go`. `go test ./internal/plugins/imageserver/...`.
5. **AC-6 config plumbing**: YANG leaf `shell-auth-sha256`; config.go parse;
   provision sha256 + `generateConfig` emit. Go tests for provision + config.
6. **AC-7 gates**: `make test`, `go test` for both Go packages, `make ze-lint-changed`.

## Risks & Assumptions

- A-1: iPXE `${mac}` expands to the boot NIC's MAC in `aa:bb:..` form matching
  `/sys/class/net/*/address`. Verify on hardware/QEMU.
- A-2: the build busybox includes the `stty` applet (busybox-static does);
  `busybox --install -s` + explicit Makefile symlink cover it. `stty -echo` is
  best-effort (`|| true`) so the gate still holds if it is absent.
- A-4 (DECISION): `fatal` backgrounds one `( verify_shell_auth ) <con >con &`
  per console rather than using `setsid`. initramfs PID 1 has no controlling
  terminal, so these background console readers are not stopped by SIGTTIN; this
  also keeps `wait` semantics (setsid would self-background and break the
  reboot-after-all-gave-up path). No setsid dependency added.
- R-1: sha256-on-cmdline is unsalted and visible in boot logs; acceptable per
  user decision (deters casual onsite access; strong password assumed). Documented.
- R-2: per-console shell wiring is hard to unit-test fully; covered by
  table-testing `password_matches` + `verify_shell_auth` (start_shell stubbed),
  a `fatal` fail-closed test, a source-structure guard, and manual QEMU.
- A-3: keep `ip=dhcp` as the fast path for isolated single-NIC installs; `ze.mac`
  drives recovery only when the kernel picked the wrong NIC.
- A-5: the build busybox includes the `udhcpc` applet (busybox-static does);
  `busybox --install -s` + the explicit Makefile symlink cover it at runtime.
  busybox udhcpc applies nothing itself, so `udhcpc.script` (installed at the
  default path and passed via `-s`) is what configures the NIC. Without udhcpc the
  kernel `ip=dhcp` fast path still serves isolated single-NIC installs; only the
  multi-homed recovery needs it. Verify on hardware/QEMU.

## Checklist

### Goal Gates
- [ ] AC-1 NIC pinning implemented + unit test green
- [ ] AC-2 reachability honesty implemented + unit test green
- [ ] AC-3 shell on all consoles implemented + unit test green
- [ ] AC-4 shell password-gated + fail-closed + unit test green
- [ ] AC-5 boot.ipxe emits ze.mac + ze.shell-auth + handler_test green
- [ ] AC-6 provision→config→YANG plumbing + Go tests green

### Quality Gates
- [ ] `make -C tools/installer-initrd test` green
- [ ] `go test ./internal/plugins/imageserver/... ./internal/plugins/provision/...` green
- [ ] `make ze-lint-changed` clean
- [ ] Manual QEMU/hardware PXE validation (multi-homed completes; forced fatal prompts on monitor)

## Review Gate

Status: implementation complete; all automated gates green. Hardware/QEMU PXE
re-test pending (requires the multi-homed lab target).

Verification run:
- `tools/installer-initrd` unit suite (10 scripts): all pass
  (connectivity 32, ensure_network 42, console 27, cmdline-parse 68, applets 29,
  udhcpc-script 12, disk-detect 47, iso-media 11, download 22, ventoy-detect 15).
- `go test ./internal/plugins/imageserver/... ./internal/plugins/provision/...`: pass.
- `make ze-lint-changed`: 0 issues. `go build ./...`: pass.
- `bin/ze-test install -a`: 37/37 pass (3 unrelated skips). id 13
  `install-generated-config-valid` and id 11 `image-server-config` pass, proving
  `ze config validate` accepts the new `shell-auth-sha256` leaf end-to-end.

/ze-review pass 1 findings — RESOLVED:
- BLOCKER (ISO path fail-closed): `fatal` is now source-aware
  (`rescue_console`/`rescue_on_all_consoles`): credential present → gated shell;
  no credential + ISO → ungated shell then `poweroff` (operator controls media);
  no credential + network → fail closed (reboot). `fatal` stays terminal. Tests:
  test-console.sh ISO-open/poweroff, http-fail-closed, structure guards.
- ISSUE (missing functional test for shell-auth validation): added
  `test/parse/image-server-invalid-shell-auth.ci` (id 129, passes — `ze config
  validate` rejects a malformed hash).
- NOTE (sha256-on-cmdline exposure, failure-mode UX): accepted/intended.
Re-verified: init suite 9/9 (console 27); `ze-test bgp parse` image-server 2/2;
`ze-test install` generated-config pass; test-relaxation audit clean.

Open items (not BLOCKER/ISSUE — environment-bound):
- AC-7 manual QEMU/hardware PXE validation of: (a) multi-homed target completes
  the image write via `ze.mac` pinning, (b) a forced `fatal` shows a password
  prompt on the monitor and gates the shell. To run: rebuild the initrd
  (`make -C tools/installer-initrd`) and ze-setup, redeploy, restart the install
  server, re-PXE.
