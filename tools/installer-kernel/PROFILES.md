# Installer kernel profiles

<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile -->
<!-- source: internal/appliance/kernelreq.go -- enforceKernelRequirements -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

A profile is valid when both files exist in `tools/installer-kernel/`:

- `<name>.config` contains Kconfig fragments merged after `kernel.config`.
- `<name>.require` lists required symbols, one `CONFIG_*` per line. `CONFIG_FOO` and `CONFIG_FOO=y` are equivalent.

Profile names are safe tokens: lowercase letters, digits, and dashes, starting with a letter or digit. `qemu`, `hardware`, `hardware-kms` and `n100` are all valid name forms; the shipped set is the one below.

This directory holds exactly the profiles the repository ships: `qemu`,
`hardware` and `hardware-kms`. A `.config` and `.require` pair placed here
registers as a real profile. A test that needs its own profile writes the pair
into a scratch directory the test creates.
`TestRegisteredKernelProfilesShippedSet` in
`internal/appliance/kernelreg_test.go` fails on any other pair. Adding a shipped
profile means adding its two files, its name in the sentence above, and its name
in that test.

A profile may extend one base profile with a header in its `.config` file:

```text
# ze-base: hardware
CONFIG_MY_DRIVER=y
```

The base profile's `.config` and `.require` are resolved before the child profile. Nested bases are rejected.

## Shared fragments (`# ze-include:`)

A profile's (or base's) `.config` may pull in a shared fragment from
`tools/kernel-builder/common/` with an include header:

```text
# ze-include: efi-console
CONFIG_MY_LOCAL_SYMBOL=y
```

`# ze-include: efi-console` adds `tools/kernel-builder/common/efi-console.config`
(and its paired `.require`) to the resolved fragment list, once, after the base
and profile fragments. This single-sources a Kconfig subset that more than one
profile needs: the `efi-console` fragment carries the verified-identical Fintek
serial + EFI framebuffer console symbols shared by the runtime kernel and the
installer `hardware` profile. Each profile keeps its own divergent symbols local.
A shared fragment ships a paired `.require` (which may be empty); the native Go
resolver expands the include into one ordered fragment set, and removing a
shared fragment fails resolution rather than silently dropping its symbols.

`ze appliance kernel --profile <name>` resolves the registry in Go (base +
`# ze-base:` + `# ze-include:`), calls the compiled builder with explicit
fragment order, reads the resolved config, and fails if any manifest symbol or
universal installer floor symbol did not resolve to `=y`.
