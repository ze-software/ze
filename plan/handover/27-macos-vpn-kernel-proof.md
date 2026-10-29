# Mac handover: native VPN kernel proof

**Record:** `spec-appliance-kernel-vpn-modules`, AC-5 and AC-6; durable contract: `docs/architecture/testing/qemu-integration.md`, “VPN facilities without module loading”.
**Branch:** `main`.
**Result:** Native arm64/HVF traffic and the selected-cache receipt are accepted. The commands below are historical instructions, not a request for another run.

## What is ready

- The native proof implementation is committed: `22f782e1c2`, with the NULL-ESP log-flush repair in `8544ee50de`.
- Native amd64/KVM passed. That result does not satisfy the required arm64/HVF acceptance.
- The Linux session continues its own fixes and native proofs. Do not copy its scratch binaries, private overlays or cached amd64 kernel onto the Mac.
- The website decision is WebP screenshots plus lightweight terminal playback. No video conversion, upload or website publication is part of this handover.

## Mac results: traffic and cache receipt accepted

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
runtime floor also protects `CONFIG_SENSORS_PWM_FAN`.

The third run at `ceb83b7140`, including repair `6e8ff49248`, built the new
arm64 image `4c4d23d6ca4234aa` and passed under HVF. The console shows the fan
module absent, an empty loaded-module list, module loading disabled and both
modprobe absence guards passing. All five ESP scenarios and WireGuard passed;
cleanup reported no leftovers and the VM stopped. Linux received the five logs
and identity files in `tmp/mac-vpn-proof-2026-10-27-pwmfan/` and recorded their
hashes and source-backed verdict in the spec.

Do not repeat the passing commands below. On 2026-10-28 the owner returned
the read-only cache observations. Exporting recorded revision `ceb83b7140`
and querying its resolver, with and without explicit Docker, selected
`7.2.9-runtime-arm64-runtime-docker-43d6bdb1-34cba457`.
Its `config` has HWMON, PWM and SENSORS_PWM_FAN built in; `kernel.version`
identifies 7.2.9/runtime/runtime/arm64/modules=yes/builder=docker.
The installed `lib/modules/7.2.9-ze/modules.builtin` names
`kernel/drivers/hwmon/pwm-fan.ko`; the same tree has no loadable fan file or
fan entry in `modules.dep` or `modules.order`. It retains 1608 other modules.
The older `a2bfd4f0` tree was the control: fan `=m` with a loadable module.
These are owner-returned observations, not files transferred to Linux.
No kernel/image rebuild, boot or traffic run occurred. Abbreviated hashes
are not full-checksum evidence, and no image-kernel byte extraction is claimed.

Linux completed the four deliberate breaks and clean restoration. A new Mac
traffic run is not required to confirm the WireGuard receipt: Linux read
`checkWireGuardProof` and `requireWireGuardReceipt`, unchanged from the Mac
revision. Their PASS requires a successful tunnel ping plus matching peer keys,
nonzero handshake timestamps and positive received bytes on both peers.


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
  ./le --name mac-vpn-proof test qemu vpn-kernel timeout 30m 2>&1 | tee "$run/vpn-kernel.log"

printf 'Native commands completed successfully. Return the evidence from %s\n' "$run"
SH
```

A cold kernel/image build can precede the guest run. The explicit 30-minute value is the existing documented VPN proof setting, not permission to extend a failure's deadline.

## Original return instructions (fulfilled)

The following was the private evidence request; no further transfer is required:

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

- This run proves Ze's docker-host kernel facilities, not the default appliance image's packaged configuration or its L2TP/PPP workflow. Those were separately proved under `spec-appliance-ships-ze-kernel`; its committed closure record retains the amd64 image readback and owner-run L2TP receipt. The surviving build and proof contracts are in `docs/guide/appliance.md` and `docs/architecture/testing/qemu-integration.md`.
- Live forced-break discrimination and final reconciliation completed under `spec-appliance-kernel-vpn-modules`; the Mac run alone was not the closure evidence.
- It does not prove Ze's WireGuard configuration-commit workflow; `plan/release/validation/spec-wireguard-runtime-proof.md` owns that work.
- Do not run `verify current`, `verify worktree`, a full functional suite or a whole-tree lint for this request. No push, upload or deployment is requested.

## Source and command references already inspected

- `internal/le/test/qemu/vpnproof.go`: native entry point, emitted-config checks and disposable guest preparation.
- `internal/le/test/qemu/dockerlab.go`: native guest transport, hardware acceleration, provenance and cleanup.
- `internal/le/test/deployment/vpnproof.go` and `wireguardproof.go`: inner aggregate and traffic proof.
- `docs/architecture/testing/qemu-integration.md`, “VPN facilities without module loading”. Use that route, not the page's generic Alpine examples.
- `docs/guide/appliance.md`, “VPN Kernel Proof”.
- Native `le help setup docker host build`: verified `arch arm64` and image-build action.

**Verification limit:** Native arm64/HVF preparation and all six traffic
scenarios passed after the fan repair. Linux subsequently completed all four
deliberate breaks, an unmutated six-case restoration and the default-image
packaged-config readback. The owner-returned selected-cache/config/module-tree
receipt was reconciled on 2026-10-28. No product WireGuard config-commit or
external both-role rekey coverage is claimed. No passing run is repeated.
