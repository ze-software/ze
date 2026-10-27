# Mac handover: native VPN kernel proof

**Spec:** `plan/release/delivery/spec-appliance-kernel-vpn-modules.md`, AC-5 and AC-6.
**Branch:** `main`.
**Goal:** Run the existing VPN kernel proof on an Apple Silicon Mac and return its native arm64/HVF evidence. No source edits or broad verification are requested.

## What is ready

- The native proof implementation is committed: `22f782e1c2`, with the NULL-ESP log-flush repair in `8544ee50de`.
- Native amd64/KVM passed. That result does not satisfy the required arm64/HVF acceptance.
- The Linux session continues its own fixes and native proofs. Do not copy its scratch binaries, private overlays or cached amd64 kernel onto the Mac.
- The website decision is WebP screenshots plus lightweight terminal playback. No video conversion, upload or website publication is part of this handover.

## First Mac result and the next run

The first run at `7aa2f4849e` built the arm64 kernel and image, passed the
emitted-config prerequisite and booted under HVF. Preparation exited with
`Lima boot.sh: exit status 1`; Docker never became ready. No VPN traffic
case ran. Evidence returned to Linux is in `tmp/mac-vpn-proof-2026-10-27/`.

The preparation now traces only its fixed module checks and refuses failed
procfs reads before testing their values. It still rejects every loaded module
and requires `/sbin/modprobe` to be absent. This is a diagnostic and guard
repair, not a claim that the Mac failure is fixed.

After Thomas authorizes the next Mac run, pull the commit carrying this repair
and execute the block below once. Keep the original evidence; the block makes
a new evidence directory. Let the normal resolver choose valid cached artifacts,
without forcing a rebuild or selecting an old image by hand.

Return the complete console trace even if preparation fails again. In
particular, retain the module list and the last traced assertion. The pinned
gokrazy `loadModules` attempts `pwm-fan.ko` before this preparation, and the
Mac image contains it; successful loading is only a hypothesis until the
guest's module list establishes it. Do not unload modules, add an exception,
change the kernel or relax a guard to get past the refusal.


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

Keep the emitted kernel `config`, `kernel.version` and the generated image/cache entry until the evidence has been reconciled. Use the paths from this run; do not choose a different cached kernel by date. Do not publish raw credentials or key material if a diagnostic contains them.

## What constitutes acceptance

The native outer report must identify the arm64 image and HVF, a passing kernel check, successful lab execution and completed cleanup with no leftover resources. The inner report must pass every ESP traffic case and the WireGuard handshake/received-traffic proof. A boot or SA installation without traffic is insufficient; an interrupted or setup-only run is not a protocol verdict.

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

**Verification limit:** The first Mac run established build, configuration and boot evidence only. The Linux sandbox exercises the repaired shell guards, not arm64 BusyBox or VPN traffic. The diagnostic Mac rerun and full native arm64 traffic proof remain owed.
