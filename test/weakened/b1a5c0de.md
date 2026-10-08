# Site: the 177 legacy URLs 404, owner decision 2026-10-08

The owner withdrew AC-12 of spec-site-renderers-in-go on
2026-10-08: no redirect stubs and no legacy-URL rewriting, and the paused code
is deleted with the legacy table rather than kept unregistered (no-layering).
The behavior these tests proved no longer exists.

| Test | Reason |
|------|--------|
| mplsmtu_integration_linux_test | `skipWithoutMPLSIPMTU` skips the cases whose bound only Ze's MPLS IP MTU kernel patch enforces (PathMTU transit and push-over-link, every FragmentProgress case, TransitIPv4Options), and only when `kernelcap.MPLSIPMTU` answers "absent", naming that answer. Any other answer runs them, so the QEMU guest's patched runtime kernel still runs every case. On a stock kernel they cannot pass, and FragmentProgress/rounded-zero-payload livelocked the 6.8 host (`plan/journal/kernel-refuses-what-the-installer-sends.md`). The stock-kernel behavior is now proved by `TestMPLSIntegration_TransitPathMTUFollowsTheProbe`. |
