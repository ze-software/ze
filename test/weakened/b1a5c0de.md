# Site: the 177 legacy URLs 404, owner decision 2026-10-08

The owner withdrew AC-12 of spec-site-renderers-in-go on
2026-10-08: no redirect stubs and no legacy-URL rewriting, and the paused code
is deleted with the legacy table rather than kept unregistered (no-layering).
The behavior these tests proved no longer exists.

| Test | Reason |
|------|--------|
| mplsmtu_integration_linux_test | `TestMPLSIntegration_FragmentProgress` now also skips when `kernelcap.MPLSIPMTU` answers "unknown" (it already skipped on "absent"), via `requireMPLSIPMTU`. Safety precondition, not lost coverage: on a kernel without `0002-mpls-ip-mtu.patch`, `ip_do_fragment` rounds a sub-quantum fragment budget to zero payload and spins forever (it froze the 6.8 host on 2026-10-08), and "unknown" cannot rule such a kernel out. The QEMU guest runtime kernel answers "present" and runs every case. |
