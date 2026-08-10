---
kind: directive
level: SHOULD
stage:
---
**SHOULD prefer a knob that skips the work over a target that supplies the privilege.**
Five L2TP `test/plugin` tests used to sit in the second target; they now set
`ze.l2tp.disable-kernel-dataplane=true`, build no kernel worker, and pass
unprivileged. That was right because each asserts on the CLI surface and never on
the kernel's view, so nothing was lost. It is the WRONG move whenever the
privileged behaviour is the behaviour under test -- `show system kernel-log`
cannot be freed this way, and neither can
`test/l2tp/session-stopccn-cascade.ci`, which sets `skip-kernel-probe` and still
needs the data plane. Note those are two DIFFERENT knobs:
`skip-kernel-probe` bypasses the modprobe only.
<!-- source: mk/test-integration.mk -- ze-netns-test, ze-netns-plugin-test -->
