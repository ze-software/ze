---
kind: table
level:
stage:
---
| Mistake | Fix |
|---------|-----|
| "Needs real hardware, skipping test" | Use a virtual substitute (see table above) |
| `//go:build linux` on a test that needs root | Use `//go:build integration && linux` |
| Forgetting to add a Linux package to `integrationPackages` in `internal/le/qemu/alltests.go` | Test compiles but never runs in the QEMU inventory |
| Using `t.Fatal` for missing capabilities | Use `t.Skip` so the test is portable |
| Hardcoding `/dev/ttyS0` in a test | Use `pty.Open()` for a real PTY pair |
| Reading a QEMU evidence timeout as "tcg is slow" | On Linux, check `kvm-access` first (`./le setup check`). A user outside the `kvm` group makes qemu refuse to start, which surfaces as a timeout |
| Selecting the accelerator on `Path("/dev/kvm").exists()` | Existence is not access. Probe `os.access(..., R_OK\|W_OK)`, and branch on `sys.platform == "darwin"` for `hvf` |
