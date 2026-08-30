---
kind: directive
level: MUST
stage:
---
**A new integration package MUST be added to `integrationPackages` in `internal/le/qemu/alltests.go`.** `./le qemu all-tests` runs that closed list, so a package absent from it never runs and nothing goes red.
