# Door: appliance

<!-- door-base: internal/appliance internal/component/gokrazy internal/install cmd/ze-installer cmd/ze-serial-shell internal/component/config/system gokrazy -->

Read `ai/doors/system.md` first. This page names what is particular to the
gokrazy appliance:

- the image a build host makes
- the binaries that run on the device
- how a device takes a new image or a new config

## Where it sits

- **`internal/appliance` is build-host tooling, not a component.** It owns the
  whole `ze appliance` surface and registers it as a root handler. Deleting the
  directory and its blank import removes the surface
  (`docs/architecture/appliance/command-provider.md`). The blank import is in
  `cmd/ze/setup_features_setup.go`, behind `ze_setup`, so every flavor built with
  that tag carries it: ze-setup, and ze-host (`ze_core ze_setup`, the `host`
  row of `internal/le/repo/compiles/matrix.go`). `ze_appliance` is reserved for on-device runtime features
  (`cmd/ze/setup_features_appliance.go`, its header).
- **Two kinds of binary.** A host binary runs on the build machine: `ze-host`,
  built by `./le build host-driver`. A target binary runs on the appliance:
  `cmd/ze-installer` (the initrd PID 1) and `cmd/ze-serial-shell`
  (`ai/rules/platform-linux.md`, `docs/architecture/appliance/installer-initrd.md`).
- **On the device, the daemon is ordinary `ze`.** `cmdStart`
  (`cmd/ze/ze_core_start.go`) picks the config by the priority chain in
  `docs/architecture/core-design.md`, section 21, then calls `hub.Run`.
- **System update is one registered backend** (`internal/component/config/system`
  `UpdateBackend`). The hub picks it by platform through `system.NewBackend`
  (`cmd/ze/hub/main_system.go` `startBackend`): `gokrazy-ab` on gokrazy,
  `ze-self-update` elsewhere (core-design section 20).
- **`internal/component/gokrazy`** is a reverse proxy from ze's web server at
  `/gokrazy/` to gokrazy's management Unix socket (`gokrazy.go`, package doc,
  and mounted by `internal/component/web/register_gokrazy.go`).

## Who depends on it from outside

| Outside party | Contract | Page |
|---------------|----------|------|
| An operator on a Linux or macOS build host | `ze appliance init`, `build`, `iso`, `push`. Secrets are never in process arguments | `docs/architecture/appliance/builder.md`, `docs/guide/appliance.md` |
| A running device | The gokrazy update protocol through `github.com/gokrazy/updater`: features probe, stream, A/B switch, testboot, reboot | `docs/architecture/appliance/ota-push.md` |
| A fleet tool | `/perm/ze/config-pushed.conf` in, `/perm/ze/config-active-hash` out, auto-revert after a push | core-design section 21, `docs/architecture/appliance/device-config.md`, `docs/architecture/fleet-config.md` |
| Anyone at the serial console | The login gate before `ze-recovery-shell` | `docs/architecture/appliance-serial-login.md` |
| A standalone Linux box | The update manifest and `selfUpdater` | `docs/architecture/appliance/self-update.md` |
| gokrazy upstream | `gok`, the updater module, and the module pins under `gokrazy/ze/builddir` | `docs/architecture/appliance/gokrazy-build-pins.md` |
| A bricked install | The recovery console PID 1 routes to instead of exiting | `docs/architecture/appliance/installer-initrd.md`, "PID 1 cannot exit" |

An image already in the field is the contract. This applies to a change to a `/perm/ze/` path,
a zefs key the image writes, or the push protocol. Each such change must still
work against the device that runs the previous image.

## Boundaries

- **Never cross-compile a host binary.** Give `GOARCH=<target>` only to a
  target build, or to `ze appliance initrd`, which cross-compiles internally
  (`ai/rules/platform-linux.md`).
- **The initrd is one static Go binary with zero external binaries.** It reads
  `/proc` and `/sys` and never adds an `exec.Command` of a tool
  (`ai/rules/platform-linux.md`). It must not import the `iface` component. It
  resolves interfaces directly, under an allowlist entry
  (`installer-initrd.md`, "Traps").
- **ze-setup links no `internal/le` package**, so `ze appliance build` cannot
  select the patched GOROOT itself. `SetGokGoEnv` (`internal/appliance/cmd_build.go`)
  refuses unless the environment already does (`builder.md`).
- **The `./ze` launcher binary has no `ze_setup`**, so `./ze appliance ...`
  answers "unknown command". Name the host driver in any instruction you print
  (`buildhostdriver.RuntimeKernelBuild`, `internal/le/build/hostdriver/hostdriver.go`).
- **Build credentials go through zefs keys**, the same `meta/auth/local/*` keys
  `ze init` writes (`ai/digests/aaa-auth.md`, step 1,
  and `internal/appliance/cmd_assemble.go`).
- **A gokrazy device does not replace its own ze binary.** The `gokrazy-ab`
  backend reports "managed by gokrazy" (core-design section 20).

## Read before code

| Read | Question it answers |
|------|---------------------|
| `docs/architecture/appliance/README.md` | Which page covers which part of build, install and update |
| `docs/architecture/appliance/builder.md` | The build host, secrets at rest, the patched-GOROOT refusal |
| `docs/architecture/appliance/installer-initrd.md` | PID 1 rules, host versus target, initrd traps |
| `docs/architecture/core-design.md`, sections 20 and 21 | The update backends, the device config priority |
| `docs/architecture/testing/qemu-integration.md` | How a Linux-only change is proven in the QEMU VM |
| `docs/contributing/patching-the-standard-library.md` | Why gok's Go builds need the patched tree |
| `ai/rules/platform-linux.md` | QEMU proofs, the runtime kernel, dependency bumps under `gokrazy/modcache/` |
| `ai/patterns/cli-command.md` | Adding an `ze appliance` subcommand through the registry |
| `ai/digests/aaa-auth.md` | How the credentials an image carries become a login |

## Traps

- **The verify worktree once built over an empty gokrazy modcache**, so the
  appliance `.ci` tests failed for a reason in the harness
  (`plan/journal/gate-verdict-depends-on-the-machine.md`, 2026-10-12, fixed by
  `sharedModcacheLink`).
- **Six refusals told the operator to run `./ze appliance kernel`**, a form the
  launcher binary does not have
  (`plan/journal/failure-report-names-a-command-that-does-not-exist.md`, 2026-10-10).
- **An image option was validated at the helper and not at the CLI.**
  `image.isolated-cpus` had a unit test only. Its two siblings each had a `.ci`
  through `ze appliance init`
  (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-07).
- **Sessions carried each other's hunks in `internal/appliance/config.go` and
  `kernelargs.go`** (`plan/journal/concurrent-session-corruption.md`,
  2026-09-06). Check the hunks against HEAD before you commit them.
- **The initrd cache key ignores build tags**, so a fault build can be served a
  cached normal initrd (`installer-initrd.md`, "Traps", and `internal/appliance/cache.go`
  `initrdCacheVariant`).
- **A push mock must speak the whole protocol**, the features probe included,
  or it passes while the probe is broken (`ota-push.md`, "Trap").
- **A move between directory depths breaks every relative path**, in tests and
  in argv lists a grep for the literal misses (`command-provider.md`, "Trap").
- **A containerized kernel build leaves root-owned files** under `tmp/`
  (`plan/journal/container-build-leaves-root-owned-scratch.md`).

## Gates

| Command | Judges |
|---------|--------|
| `./le test functional appliance`, `./le test functional installer` | The appliance CLI (build, iso, list, serial login) and the installer `.ci` files |
| `./le test unit installer` | The initrd logic behind the `ze_installer` tag |
| `./le test qemu install-test`, `./le test qemu install-iso-test`, `./le test qemu install-scenarios-test` | A real install in a VM, the ISO, panic recovery and NIC pinning |
| `./le test qemu all-tests` | The whole functional population inside the QEMU Linux VM |
| `./le build gosum` | The `gokrazy/ze/builddir/**/go.sum` files agree with the root module |
| `./le arch iface-resolution`, `./le arch fs-persistence` | Interface names go through the resolver. Runtime state goes through zefs |
