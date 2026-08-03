# 988 -- Kernel build consolidation

## Context
After 870 unified the kernel build ENGINE (`build.py`) and 982 gave the installer
kernel a Go-verified profile registry, the build DRIVER and the config FRAGMENTS
were still duplicated: the docker/qemu invocation was copy-pasted across two
Makefiles and `cmd_kernel.go`, the Fintek-serial + EFI-framebuffer Kconfig subset
was hand-synced between the runtime and installer-hardware configs, the kernel
version lived under three different variable names, and `make ze-kernel` mutated
the pinned `rtr7/kernel` modcache in place with a fragile `.ze-pinned-kernel`
backup/restore. This spec finished the consolidation: one driver, one shared
fragment, one version reader, and out-of-tree runtime-kernel consumption -- all
behaviour-preserving for the produced kernels.

## Decisions
- One `tools/kernel-builder/run.py` driver called by both Makefiles AND `cmd_kernel.go`, over a shared `mk/` include that left Go's argv duplicated: run.py owns docker/qemu selection, the single arch->platform map, the single build-time `kernel.version` reader, and provenance; Go's `defaultDockerBuild`/`defaultQEMUBuild`/`dockerPlatform`/`selectBuilder` were deleted.
- `# ze-include: <name>` shared fragments in `tools/kernel-builder/common/`, mirroring the existing `# ze-base:` one-level layering, over a driver-prepended config list: explicit per-profile opt-in keeps the shared symbols out of the qemu profile. The shared fragment is config-only with an EMPTY paired `.require`, so removing it fails at include-RESOLUTION (file-not-found), never silently at enforcement.
- Resolver duplicated in `run.py` (single python copy, make path) and `kernelreg.go` (Go verified path) over a single resolver, because the make path must stay Ze-binary-free; a cross-language fixture (`kernel-shared-fragment.ci` + `TestResolveSharedInclude`) guards drift.
  - **CORRECTION (2026-07-21): the "make path must stay Ze-binary-free" rule was REVERSED (Option C, Thomas 2026-07-16).** The make kernel-cache path now asks the HOST `ze-host` binary for the arch+config-keyed cache dir (`mk/gokrazy.mk`: `ze-host appliance kernel --print-cache-dir` / `--evict-cache`), so `kernelCacheVariantFor` (`internal/appliance/cache.go`) is the single source of truth for the cache key and cross-language drift on the KEY is impossible rather than merely detectable. `make ze-kernel` now requires a compiled host `ze-host` first. Do NOT "restore" the Ze-binary-free rule for the cache path. Reversal originated in `plan/spec-fixit-qemu-artifact-cache.md` (superseded by `plan/learned/1173-relocate-scratch-and-cache.md`, `plan/learned/1173-relocate-scratch-and-cache.md`).
- Out-of-tree runtime-kernel consumption via a `go.mod replace` of `github.com/rtr7/kernel` to an assembled `tmp/kernel/pkg` (pinned module copy + our vmlinuz/modules), over in-place modcache overwrite + backup/restore: gok resolves the kernel dir via `go list -mod=mod`, which honours the replace, so the pinned modcache is never mutated.
- One `KERNEL_VERSION` name across Makefiles + the builder env, with `run.py` self-locating `internal/appliance/kernel.version` (build-time reader) and Go `//go:embed` (compile-time reader), over each Makefile `cat`-ing the file under its own variable name.

## Consequences
- Adding a board's console fix once (edit `common/efi-console.config`) updates both kernels; bumping the kernel version is one file; the build invocation lives in one driver.
- The runtime kernel now has the SAME verified guarantee the installer got in 982: `ze appliance kernel --target runtime` resolves the `gokrazy/kernel` registry, enforces a runtime floor (`CONFIG_MODULES` + the L2TP/PPP/PPPoE set), and caches the artifact as a directory TREE keyed by `target` (distinct from the installer single-file `Image` cache).
- `make ze-kernel` leaves the working tree's builddir `go.mod` carrying a transient `replace` (reverted by `make ze-kernel-clean`); this is git-visible, not a hidden backup.
- `ze appliance kernel --version` now rejects pre-7 / malformed versions in the command path (the version is single-sourced; the flag only overrides).

## Gotchas
- Behaviour preservation is provable cheaply WITHOUT a full kernel compile: moving N `=y` lines from a profile config into an included fragment keeps the multiset of directives fed to `merge_config` byte-identical, so the resolved `.config` is identical. The real-build baseline (config-only `defconfig`+`merge`+`olddefconfig` in docker, no `-j` compile) is the gold-standard backstop; both agreed byte-for-byte for all four profiles.
- `CONFIG_X86_SYSFB` is DROPPED by `olddefconfig` in linux-7.1.1 (absent in the resolved config though requested `=y`). The baseline captures the ACTUAL resolved config, so this existing quirk is preserved rather than "fixed".
- gok finds the kernel package via `go list -mod=mod -tags gokrazy -f '{{.Dir}}'` (vendor/.../gotool.go) -- reading that one line confirmed the path-replace mechanism (A-1) at the code level, no full gok build needed to validate resolution.
- The out-of-tree pkg must be a VALID Go module: copy the pinned module's `go.mod` + `empty.go` (the packer placeholder) alongside our vmlinuz/modules, or the replace target is not a module and gok fails. The raw `tmp/kernel/build` output is not a module.
- Once builder selection moved into run.py, the Go-fronted qemu path can no longer be hermetically faked end-to-end: use a smart fake `python3` that passes `run.py` through to the real interpreter and stubs only the `qemu-build.py` VM step.
- `build.py` imports its sibling `ksource` module, so any out-of-tree `importlib` exec of `build.py` (e.g. a `.ci` that imports it) must put `tools/kernel-builder` on `sys.path` first.

## Files
- `tools/kernel-builder/run.py` (new), `tools/kernel-builder/ksource.py` (new), `tools/kernel-builder/common/efi-console.{config,require}` (new)
- `tools/kernel-builder/build.py`, `tools/kernel-builder/qemu-build.py`
- `gokrazy/kernel/Makefile`, `tools/installer-kernel/Makefile`, `mk/gokrazy.mk`
- `gokrazy/kernel/kernel.config`, `tools/installer-kernel/hardware.config`
- `internal/appliance/cmd_kernel.go`, `kernelreg.go`, `kernelreq.go`, `cache.go` (+ their `_test.go`)
- `internal/plugins/provision/staging.go` (+ `_test.go`)
- `test/install/`: new (`kernel-builder-single-driver`, `kernel-arch-mapping-single`, `kernel-shared-fragment`, `appliance-kernel-runtime`, `ze-kernel-no-modcache-mutation`, `kernel-tarball-dedup`, `kernel-version-single-reader`, `kernel-version-provenance`); rewritten (`kernel-wiring`, `kernel-builder-no-shell`, `kernel-runtime-deps`, `kernel-compose`, `appliance-kernel-{docker,qemu,auto-docker,auto-qemu}`, `ze-kernel-overlay`)
- `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `tools/installer-kernel/README.md`, `tools/installer-kernel/PROFILES.md`, `docs/functional-tests.md`
- `plan/learned/870-kernel-build-convergence.md` (additive correction pointer)
