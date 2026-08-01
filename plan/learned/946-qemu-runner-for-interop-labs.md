# 946 -- qemu-runner-for-interop-labs

Date: 2026-06-20

## What this was

Added a PPPoE interop test: Ze's `pppoe-client` interface kind dialing a real
accel-ppp access concentrator. Shipped as a Docker lab (`test/pppoe-interop/`)
AND a QEMU runner (`make ze-qemu-pppoe-accel-test` -> `scripts/evidence/
effective-pppoe-accel.py`). Proven end-to-end in QEMU on macOS.

## Core lesson: Linux-only interop labs need a QEMU runner, not just Docker

A Docker interop lab that depends on host-kernel features does not run on macOS
or in CI by itself: Docker Desktop's VM lacks the modules and the Alpine QEMU VM
has no Docker. "It's Linux-only / needs the host kernel" is the trigger to build
the QEMU runner, not an excuse to skip it. The four-step pattern is now codified
in `ai/rules/qemu-testing.md` ("Interop Labs ... Need a QEMU Runner Too"):

1. netns `effective-<feature>.py` (two namespaces + veth, no Docker), mirroring
   `effective-l2tp-ppp.py`;
2. install the peer daemon from Alpine via `--packages` (accel-ppp, frr, xl2tpd
   are all in Alpine community -- prefer the package over a from-source image);
3. if a kernel module is missing from the stock Alpine kernel, add the `CONFIG_*`
   to `gokrazy/kernel/runtime.config` + `tools/kernel-builder/build.sh` and run
   with `--kernel tmp/kernel/vmlinuz`;
4. a `ze-qemu-<feature>-test` target wired into `.PHONY` + Makefile help.

## Why PPPoE and not L2TP (the original ask)

L2TP needs one LAC + one LNS. Both Ze (LAC mode deferred) and accel-ppp
(documented LNS, no usable client mode) are LNS-only, so a Ze<->accel-ppp L2TP
tunnel is impossible. PPPoE is where the roles are complementary: accel-ppp's
first-class role is the server (AC), Ze has a full RFC 2516 client. Check role
compatibility before promising an interop test.

## Three bugs found while making the QEMU path actually run

1. **Kernel build broke on arm64**: `python3: not found` in the kernel-builder
   image. arm64 `defconfig` pulls in the MSM GPU driver whose header generator
   needs python3. This broke every arm64 runtime-kernel build, including the
   pre-existing l2tp QEMU test. Fix: add `python3` to
   `tools/kernel-builder/Dockerfile`.
2. **`tmp/kernel/vmlinuz` was referenced but never produced.** Both QEMU targets
   require it; `make ze-kernel` wrote only `gokrazy/kernel/build/vmlinuz`, and
   `ze-kernel-clean` even deleted `tmp/kernel`. Fix (chosen over a wildcard
   fallback): `make ze-kernel` stages `tmp/kernel/vmlinuz`, keeping it the single
   source of truth and fixing the l2tp target for free.
3. **Verification gated on Info-level log markers under the wrong domain.** The
   pppoe-client logs under `subsystem=interface` (component name "interface", not
   "iface"), and Info is filtered at the default level, so `ze.log.iface=debug`
   surfaced nothing and the test failed even though the session was up. Fix: gate
   on kernel ground truth (the `pppN` interface + assigned address + ping), which
   is immune to log config, and set `ze.log.interface=debug` for diagnostics only.

## Reusable takeaways

- Assert kernel/observable ground truth, not log strings, when a log string adds
  a hidden dependency on log level/domain config. Log-marker waits (as in the
  l2tp script) are fragile.
- The component log domain is the registered component `Name` (here "interface"),
  resolved via `CanonicalSubsystemName`; `ze.log.<that-name>=debug`.
- The runtime kernel is shared by the gokrazy appliance and the QEMU evidence
  tests; protocol kernel options (PPP/PPPoL2TP/PPPoE) belong in it as `=y`.
- See [[705-cpe-1-pppoe-client]] for the client implementation this exercises.

## Files

None recorded.
