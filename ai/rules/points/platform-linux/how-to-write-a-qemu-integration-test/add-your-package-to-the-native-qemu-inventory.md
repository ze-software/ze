---
kind: note
level:
stage:
---
Add your package to `integrationPackages` in `internal/le/qemu/alltests.go`. `./le qemu all-tests` runs that closed list with the `integration` tag and refuses a path that does not exist.
