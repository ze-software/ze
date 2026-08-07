---
kind: note
level:
stage:
---
The installer initrd is a single statically-linked Go binary (`cmd/ze-installer`)
running as PID 1 with **zero external binaries** (busybox removed). Detect system
state through `/proc` and `/sys` reads, not external commands, and never
reintroduce `exec.Command` of an external tool.
