---
kind: directive
level: SHOULD
stage:
---
**SHOULD prefer a knob that skips the work over an action that supplies the privilege.**
Use `ze.l2tp.disable-kernel-dataplane=true` when a test asserts only on the CLI
surface and never on the kernel's view. It is the WRONG move whenever the
privileged behaviour is the behaviour under test -- `show system kernel-log`
cannot be freed this way, and neither can
`test/l2tp/session-stopccn-cascade.ci`, which sets `skip-kernel-probe` and still
needs the data plane. `skip-kernel-probe` is a different knob and bypasses only
the modprobe.
