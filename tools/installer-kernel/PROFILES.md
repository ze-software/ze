# Installer kernel profiles

<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile -->
<!-- source: internal/appliance/kernelreq.go -- enforceKernelRequirements -->
<!-- source: tools/kernel-builder/build.py -- main -->

A profile is valid when both files exist in `tools/installer-kernel/`:

- `<name>.config` contains Kconfig fragments merged after `kernel.config`.
- `<name>.require` lists required symbols, one `CONFIG_*` per line. `CONFIG_FOO` and `CONFIG_FOO=y` are equivalent.

Profile names are safe tokens: lowercase letters, digits, and dashes, starting with a letter or digit. Examples: `qemu`, `hardware`, `hardware-kms`, `n100`.

A profile may extend one base profile with a header in its `.config` file:

```text
# ze-base: hardware
CONFIG_MY_DRIVER=y
```

The base profile's `.config` and `.require` are resolved before the child profile. Nested bases are rejected.

`ze appliance kernel --profile <name>` is the verified path. It resolves the registry in Go, calls the builder with explicit fragment order, reads `build/config`, and fails if any manifest symbol or universal installer floor symbol did not resolve to `=y`.

Raw `make -C tools/installer-kernel PROFILE=<name>` builds without first building `ze`; it uses the same files but does not perform Go-side verification.
