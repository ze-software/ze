# Mac handover: native VPN kernel proof

**Spec:** `plan/release/delivery/spec-appliance-kernel-vpn-modules.md`, AC-5 and AC-6.
**Branch:** `main`.
**Goal:** Run the existing VPN kernel proof on an Apple Silicon Mac and return its native arm64/HVF evidence. No source edits or broad verification are requested.

## What is ready

- The native proof implementation is committed: `22f782e1c2`, with the NULL-ESP log-flush repair in `8544ee50de`.
- Native amd64/KVM passed. That result does not satisfy the required arm64/HVF acceptance.
- The Linux session continues its own fixes and native proofs. Do not copy its scratch binaries, private overlays or cached amd64 kernel onto the Mac.
- The website decision is WebP screenshots plus lightweight terminal playback. No video conversion, upload or website publication is part of this handover.

## Mac results and the next run

The first run at `7aa2f4849e` built the arm64 kernel and image, passed the
emitted-config prerequisite and booted under HVF. Preparation exited with
`Lima boot.sh: exit status 1`; Docker never became ready. No VPN traffic
case ran. Evidence returned to Linux is in `tmp/mac-vpn-proof-2026-10-27/`.

The diagnostic rerun at `835d147952` (including `c135dceac6`) reused image
`a6574e77e607b55b`, passed the emitted-config check and booted under HVF.
Its console confirmed `modules_disabled=1`, then
`loaded_modules='pwm_fan 20480 0 - Live 0xffff80007b830000'`.
The strict `test -z` failed. The modprobe guards, Docker readiness and all VPN
traffic checks were not reached. The VM stopped and cleanup completed.
Keep this evidence in `tmp/mac-vpn-proof-2026-10-27-rerun/` as well as the first run.

The pinned gokrazy `loadModules` attempts `pwm-fan.ko` before services start.
The console proves the loaded module, not the identity of the syscall caller.
The source repair makes the fan driver built in, preserving fan functionality:
`runtime.config` requests `CONFIG_HWMON=y`, `CONFIG_PWM=y` and
`CONFIG_SENSORS_PWM_FAN=y`; `runtime.require` requires all three; the independent
runtime floor also protects `CONFIG_SENSORS_PWM_FAN`. Local implementation is
not yet a verified Mac fix.

After Thomas authorizes the next Mac run, pull the commit carrying the built-in
fan repair and execute the block below once. Keep both earlier evidence sets;
the block makes a new evidence directory. Use the normal native arm64 image
build and resolver. The changed kernel configuration must produce a new kernel
key and an image key other than `a6574e77e607b55b`; do not force a cold build,
purge caches or override the selection with an old key.

Return the complete console trace even if preparation fails again. Do not unload
modules, add an exception, disable module support, change timeouts or relax a
guard to get past a refusal.


## Before running

1. Make this handover commit available in the Mac checkout through your normal Git transport, then pull `main`. This handover does not authorize a push. If the commit has not reached your remote, a normal pull cannot retrieve it; do not run an older checkout and report it as this revision.
2. Use a native Apple Silicon terminal: `uname -sm` must report Darwin and arm64, not an x86_64 Rosetta shell.
3. QEMU must be installed. The documented macOS prerequisite is `brew install qemu` if it is missing. The proof must report HVF, not TCG/emulation.
4. Run one QEMU lab guest at a time. Use Ze's own docker-host image, never Docker Desktop or a stock-distribution guest as the test host. Do not substitute a kernel/image override.
5. If a native prerequisite or disk-space check refuses, preserve its exact output and stop. Do not weaken the check, select an old cache key, increase the timeout, prune shared Docker resources or discard local work.

## Run from the Mac checkout root

The following block creates a fresh evidence directory. `pipefail` preserves a failing command's exit status despite `tee`; `set -e` stops before the proof if image preparation fails.

```bash
bash <<'SH'
set -euo pipefail
export CGO_ENABLED=0 GOMAXPROCS=2

if [ "$(uname -s)" != Darwin ] || [ "$(uname -m)" != arm64 ]; then
  printf '%s\n' 'This handover requires a native Apple Silicon macOS shell.' >&2
  exit 1
fi
if ! git diff --quiet || ! git diff --cached --quiet; then
  printf '%s\n' 'Tracked local changes would make the result differ from the recorded commit. Preserve them and report this refusal; do not discard or stash them.' >&2
  exit 1
fi

scratch="$(./le session scratch ensure)"
run="$(mktemp -d "$scratch/mac-vpn-proof.XXXXXX")"
printf 'Evidence directory: %s\n' "$run"
uname -sm | tee "$run/host.txt"
git rev-parse HEAD | tee "$run/commit.txt"

./le --name mac-vpn-proof job run label mac-vpn-image command \
  ./le --name mac-vpn-proof setup docker host build arch arm64 2>&1 | tee "$run/image-build.log"
./le --name mac-vpn-proof job run label mac-vpn-proof command \
  ./le --name mac-vpn-proof test qemu vpn-kernel-test timeout 30m 2>&1 | tee "$run/vpn-kernel.log"

printf 'Native commands completed successfully. Return the evidence from %s\n' "$run"
SH
```

A cold kernel/image build can precede the guest run. The explicit 30-minute value is the existing documented VPN proof setting, not permission to extend a failure's deadline.

## Return to the Linux session

Send the evidence-directory path and these files, privately:

- `host.txt` and `commit.txt`;
- `image-build.log` and `vpn-kernel.log`, or the exact first refusal if the proof did not start;
- the guest console file named by the native report, especially for a failed run;
- the shell's exit status if it failed or was interrupted.

Keep the emitted kernel `config`, `kernel.version`, installed `modules.builtin`
manifest and generated image/cache entry until the evidence has been reconciled.
Return their actual paths from this run, the three fan-related `=y` config lines,
the fan entry in `modules.builtin`, and evidence that this new tree contains no
loadable `pwm-fan.ko`. Record the selected kernel key and the image key, which
must differ from `a6574e77e607b55b`. Do not choose a different cached kernel by
date. Do not publish raw credentials or key material if a diagnostic contains them.

## What constitutes acceptance

The native outer report must identify the new arm64 image and HVF, a passing
kernel check, successful lab execution and completed cleanup with no leftover
resources. The console must show `modules_disabled=1`, an empty `/proc/modules`
and successful modprobe absence guards before Docker and traffic run. The inner
report must pass all five ESP traffic cases (AES-GCM, AES-CBC+SHA256,
ChaCha20-Poly1305, 3DES+SHA1 and NULL+SHA256) and the WireGuard
handshake/received-ping-traffic proof. A boot or SA installation without traffic
is insufficient; an interrupted or setup-only run is not a protocol verdict.

Before boot, the action checks the emitted native kernel configuration against `gokrazy/kernel/kernel.require` and `runtime.require`. In its disposable guest snapshot it disables module loading, refuses already-loaded modules and removes `/sbin/modprobe`; it does not modify the cached parent image. These assertions are performed by the native action, not supplied manually by this handover.

## Boundaries and remaining owners

- This run proves Ze's docker-host kernel facilities, not the default appliance image's packaged configuration or its L2TP/PPP workflow. Those remain with `plan/release/delivery/spec-appliance-ships-ze-kernel.md`.
- Live forced-break discrimination and final reconciliation remain with `spec-appliance-kernel-vpn-modules.md`; a green Mac run alone does not close that spec.
- It does not prove Ze's WireGuard configuration-commit workflow; `plan/release/validation/spec-wireguard-runtime-proof.md` owns that work.
- Do not run `verify current`, `verify worktree`, a full functional suite or a whole-tree lint for this request. No push, upload or deployment is requested.

## Source and command references already inspected

- `internal/le/test/qemu/vpnproof.go`: native entry point, emitted-config checks and disposable guest preparation.
- `internal/le/test/qemu/dockerlab.go`: native guest transport, hardware acceleration, provenance and cleanup.
- `internal/le/test/deployment/vpnproof.go` and `wireguardproof.go`: inner aggregate and traffic proof.
- `docs/architecture/testing/qemu-integration.md`, “VPN facilities without module loading”. Use that route, not the page's generic Alpine examples.
- `docs/guide/appliance.md`, “VPN Kernel Proof”.
- Native `le help setup docker host build`: verified `arch arm64` and image-build action.

**Verification limit:** The first run established build, configuration and boot
evidence only. The second confirmed the loaded `pwm_fan` blocker before the
modprobe guards or traffic. The repaired appliance package and focused
enforcement/cache checks passed on Linux. A native amd64 Kconfig smoke resolved
all 156 required symbols to `=y` and compiled the fan object with a built-in
initcall, without OF or COMPILE_TEST. It did not build a full kernel or image.
The new arm64 emitted config, installed module tree and full HVF traffic pass
remain owed; Linux evidence does not replace them.
