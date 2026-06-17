# Install Functional Tests

These tests require Linux (real or QEMU) and cannot run on macOS.

| Test | Validates | Requires |
|------|-----------|----------|
| `installer-go-http.ci` | AC-10: PXE/HTTP install via Go installer | QEMU + ze-install HTTP server |
| `installer-go-iso.ci` | AC-11: ISO install via Go installer | QEMU + ISO image |
| `start-gokrazy-autoinit.ci` | AC-6: boot with empty /perm self-heals | QEMU + gokrazy image |
| `build-verify.ci` | AC-1: build fails when inject produces empty /perm | e2fsprogs on build host |
