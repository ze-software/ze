---
kind: directive
level: MUST NOT
stage:
---
**`caps=net-admin` exists because Linux alone is not the requirement.** On an
unprivileged Linux host (CI runners, most dev boxes, any rootless container) a
test that applies interface config does NOT fail cleanly. The interface plugin
fails its stage-2 (configure) handshake with `operation not permitted` and the
DAEMON exits 1, so you MUST NOT read this as the daemon hanging. The TEST hangs
because its check peer waits for a BGP session the exited daemon will never
open. The gate reads `CapEff` from
`/proc/self/status` (`internal/test/runner/caps_linux.go`), not uid 0: a setcap'd
binary holds the capability without being root, and a restricted container can be
root without it.
