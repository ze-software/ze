---
kind: directive
level:
stage:
---
**`caps=net-admin` exists because Linux alone is not the requirement.** On an
unprivileged Linux host (CI runners, most dev boxes, any rootless container) a
test that applies interface config does NOT fail cleanly. The interface plugin
fails its stage-2 (configure) handshake with `operation not permitted` and the
DAEMON does exit 1 (verified 2026-07-25 in QEMU as an unprivileged user), so do
not read this as the daemon hanging. The TEST hangs, to the suite timeout,
because its check peer goes on waiting for a BGP session the exited daemon will
never open. Seven `test/reload/` tests spent their life in exactly that state,
mis-recorded as "load-sensitive". The gate reads `CapEff` from
`/proc/self/status` (`internal/test/runner/caps_linux.go`), not uid 0: a setcap'd
binary holds the capability without being root, and a restricted container can be
root without it.
