# A probe that could not look is read as "absent"

A capability probe or a file read fails for a reason of its own (a sandbox, a
permission), and the caller takes that failure as the answer that the thing it
asked about does not exist. It then picks the wrong branch with no error.

Captured: 2026-11-02 rule go-standards/directives/a-probe-that-cannot-look-answers-unknown

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-11 | spec-single-owner-lifecycle | `newStaticBackend` (`internal/plugins/static/backend_linux.go`), called once by `runStaticPlugin` before the handshake | A nil VPP connector or a channel not yet open is read as "no VPP data plane": static keeps the kernel backend for the daemon's life, with no event to reselect and no edge to `vpp`. Named-table routes go to the kernel (`applyProgrammed`); main-table routes are unaffected. Forwarding effect on a VPP box not confirmed | open: the design is the open question of `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` |
| 2026-11-10 | spec-doctor-coverage-1-audit-and-gate (A10 audit, found) | report bus `evictOldestWarning` and the error ring (`internal/core/report/report.go`), read by every `HealthProbe*` row | Read, not run: at the warning cap the oldest active warning is evicted (logged) and the ring wraps silently, and a probe then reads the evicted code's absence as healthy: an evicted `plugin-down` turns the `plugins` row healthy | open: spec-doctor-coverage-12-plugin-infra AC-7, a probe reads degraded after an eviction it cannot rule out, and doctor reports `doctor-report-bus-full` |
| 2026-11-10 | spec-doctor-coverage-1-audit-and-gate (A9 audit, found) | `checkFirewallNftables` (`internal/plugins/firewall/nft/doctor_linux.go`), reading `kernelcap.LoadedModules` | Judges nf_tables by `/proc/modules`, which lists no built-in module; Ze's kernel builds `CONFIG_NF_TABLES=y` (`gokrazy/kernel/kernel.config`), so the check warns "not loaded" on a kernel that has it, and an unreadable file also reads as absent | open: enrol nf_tables as a `kernelcap.Capability` of the nft backend with a netlink probe, replacing the module-list check |
| 2026-11-01 | - | MPLS patch gate of `TestMPLSIntegration_PathMTU`, `TestMPLSIntegration_TransitIPv4Options` (`kernelcap.MPLSIPMTU`) | Inside `userns.Enter` the probe's bind of `/proc/sys` fails with EINVAL, the gate answers unknown, and unknown ran the patch-only bounds, so the tests failed on the hosted runner's stock kernel (1289 > 1288) | fixed: 8a01e79199 asks `kernelcap.MPLSIPMTUInThisNamespace` in the test's own namespace |
| 2026-11-01 | - | stdpatch GOROOT tree, `le test fixture` run as uid 1000 in the QEMU guest | The root-built tree's key directory was 0700, so the child read the tree as absent and rebuilt it on every launch; the daemon's 30s connect-back wait expired and seven rpki-aspa tests timed out | fixed: 85ba133cdb makes every tree directory 0755 (`treeDirMode`) |
