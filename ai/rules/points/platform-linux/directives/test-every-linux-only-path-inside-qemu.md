---
kind: directive
level: MUST
stage:
---
- Linux-only code (`//go:build linux`) MUST ship with integration tests that run in the QEMU Alpine VM. "Needs real hardware" is never a valid reason to skip tests. Virtual substitutes exist for every kernel feature ze uses.
- A functional `.ci` test that boots a daemon which exercises a real Linux kernel feature MUST be marked `option=needs-linux`, and MUST be validated inside the QEMU Alpine VM, never natively on darwin.
- Every Linux-only interop lab that runs as Docker containers and depends on host-kernel features MUST also ship a QEMU-runnable path. Treat "it is Linux-only / needs the host kernel" as the trigger to build the QEMU runner, not as an excuse to skip it.
- The installer initrd is a single statically-linked Go binary (`cmd/ze-installer`) running as PID 1 with zero external binaries (busybox removed). Detect system state through `/proc` and `/sys` reads, not external commands, and never reintroduce `exec.Command` of an external tool.
- A Dependabot alert on a `go.mod` under `gokrazy/modcache/` is almost always a stale vendored upstream manifest, not your real dependency graph. Follow the runbook under "Appliance Dependency Bumps".
- Never cross-compile a host binary. A target-arch `ze-host` cannot exec on the build host ("exec format error"). Apply `GOARCH=<target>` only to the build of a target binary, or to the `ze appliance initrd` invocation that cross-compiles one internally, never to the build of the host tool that runs it.
