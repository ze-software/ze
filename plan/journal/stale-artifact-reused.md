| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-10 | kernel-compose-make-q-assertion-is-vacuous | build | `gokrazy/kernel/Makefile` has no rebuild trigger for `ARCH=` or `BUILDER=`, so `make ARCH=arm64` after an amd64 build reuses the amd64 `vmlinuz` | - |
| 2026-08-10 | kernel-compose-make-q-assertion-is-vacuous | appliance | `kernelCacheVariantFor` (`internal/appliance/cache.go`) omits `qemu-build.py` from the cache key, so an edit to the QEMU backend does not invalidate a cached kernel | - |
| 2026-08-10 | kernel-compose-make-q-assertion-is-vacuous | appliance | `resolveInstallerKernel` (`internal/appliance/cmd_kernel.go`) replaces `build/kernel/Image` and leaves `build/kernel/.variant` describing the previous build | - |
