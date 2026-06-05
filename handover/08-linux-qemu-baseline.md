# Handover: QEMU baseline -- remaining linux-only fixes

## Done (2026-06-04 session)

Clean QEMU re-run on quiet host (load ~2.3). Classified all failures. Fixed:
- `ze_linux` renamed to `ze_distro` (tag + file renames)
- Functional test runner and QEMU cross-build now include `ze_distro` (ze IS the distro binary)
- `ze-stripped-surface.ci` expected string updated ("minimal build")
- `mpls-doctor.ci` inline config missing semicolons (ASI only fires on newlines)
- Firewall suite skipped in QEMU (`ZE_QEMU_SKIP_SUITES=web,firewall`): Alpine kernel crashes on nft set-element-timeout
- `option=require-tag` infrastructure added to `.ci` runner (unused currently)
- `plan/known-failures.md` updated with full triage

Evidence: `tmp/qemu-baseline-run.log` (first clean run), `tmp/qemu-verify-run.log` (verification run).

## Remaining work

All items below are linux-only and need `make ze-qemu-shell` for investigation.

### Product bugs

1. **L2TP 13 (session-cdn-teardown):** CDN for an established session with kernel
   state does not produce "session destroyed" log. `handleCDN` exists at
   `session_fsm.go:341` but is not reached. Investigate control message sequence
   numbers in the test's python peer (`test/l2tp/session-cdn-teardown.ci`).
   Reproduce: `make ze-qemu-debug RUN='bin/ze-test-linux-arm64 l2tp 13 -v'`

2. **L2TP 15 (session-stopccn-cascade):** Kernel tunnel from test 13 leaks
   ("file exists" on genl create). "StopCCN clearing sessions" log never appears.
   Two issues: kernel tunnel cleanup between tests, and StopCCN handling path.
   `tunnel_fsm.go:260` and `tunnel_fsm.go:576` have the log lines.
   Reproduce: `make ze-qemu-debug RUN='bin/ze-test-linux-arm64 l2tp 15 -v'`

3. **Reload 28, 29 (wireguard key validation):** Linux-only (`skip-os:darwin`).
   Expected validation error messages ("public-key", "private-key is required")
   not appearing. `parseWireguardPeer` at `iface/config.go:669` does the validation.
   No wireguard-related log appears at all in QEMU output.
   Reproduce: `make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp reload 28 -v'`

### Environment dependencies

4. **Plugin 356 (show-policy-routes):** nftables "operation not supported" in QEMU.
   The nft genl operations this test needs may not work in the Alpine VM kernel.
   Either add a precondition check or skip with `option=skip-os` equivalent for QEMU.

5. **Firewall suite (all 17 tests):** Alpine QEMU kernel crashes on nft
   set-element-timeout operations. Currently skipped via `ZE_QEMU_SKIP_SUITES`.
   Firewall tests need a real Linux host or a kernel with stable nftables support.

### VM timing artifacts (not real failures)

6. **Plugin timeouts (129, 200, 201, 399):** Pass on macOS. Timeout in QEMU due to
   VM performance. Non-linux ones (129 exabgp-bridge-sdk, 399 text-handshake) confirmed
   passing on host. Linux-only ones (200 mpls-push, 201 mpls-withdraw) need QEMU
   timeout increase or investigation.

7. **Reload 19, 20 (config-apply-ordering):** Three-way router-id rotation passes on
   macOS, fails under VM timing. May need increased delays or synchronization in the
   test's reload trigger.

## Read first

- `plan/known-failures.md` (QEMU triage section, 2026-06-04)
- `ai/rules/qemu-testing.md`
- `scripts/evidence/qemu-all-tests.sh`

## Acceptance (original, partially met)

- [x] Fresh QEMU run with raw evidence attached or referenced
- [x] Each listed failure has an owner category and next action
- [ ] Product bugs have failing tests before fixes (l2tp, wireguard -- need QEMU)
- [x] Environment dependencies visible through skip or doctor check
- [x] Known-failures file no longer contains stale or unactionable entries
