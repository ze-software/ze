# A probe that could not look is read as "absent"

A capability probe or a file read fails for a reason of its own (a sandbox, a
permission), and the caller takes that failure as the answer that the thing it
asked about does not exist. It then picks the wrong branch with no error.

Captured: 2026-11-02 rule go-standards/directives/a-probe-that-cannot-look-answers-unknown

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | - | MPLS patch gate of `TestMPLSIntegration_PathMTU`, `TestMPLSIntegration_TransitIPv4Options` (`kernelcap.MPLSIPMTU`) | Inside `userns.Enter` the probe's bind of `/proc/sys` fails with EINVAL, the gate answers unknown, and unknown ran the patch-only bounds, so the tests failed on the hosted runner's stock kernel (1289 > 1288) | fixed: 8a01e79199 asks `kernelcap.MPLSIPMTUInThisNamespace` in the test's own namespace |
| 2026-11-01 | - | stdpatch GOROOT tree, `le test fixture` run as uid 1000 in the QEMU guest | The root-built tree's key directory was 0700, so the child read the tree as absent and rebuilt it on every launch; the daemon's 30s connect-back wait expired and seven rpki-aspa tests timed out | fixed: 85ba133cdb makes every tree directory 0755 (`treeDirMode`) |
