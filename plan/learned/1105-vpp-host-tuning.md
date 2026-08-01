# vpp-host-tuning

Host-side VPP tuning: expose `poll-sleep-microseconds`, reserve hugepages at boot
on the gokrazy appliance, and add a `ze doctor` readiness check. Designed to
`ready` by a Fable phase-1 agent, implemented by Opus.

## Context

Ze generated VPP's `startup.conf` (cores, buffers, page-size) but left the HOST
side (hugepage reservation, idle-worker behaviour) to the operator. This spec
added three surfaces on top of `spec-vpp-isolated-cpus`. NUMA/SMT (original
item 3) was split into `spec-vpp-numa-smt` because the host facts it needs
(per-CPU NUMA node, NIC `numa_node`, SMT sibling map) do not exist in the
inventory yet.

## Key decisions

- **`poll-sleep-usec` is a VPP `unix`-section directive, not `cpu`.** (Skeleton
  assumption A-3 was wrong.) Ze groups YANG by operator concern, not by
  startup.conf section, so the leaf lives under `vpp/cpu` but is emitted inside
  the `unix { }` block. Precedent: the `memory` container already feeds the
  buffers/heapsize/statseg sections. An explicit `0` is emitted (operator asked);
  absent leaf = byte-identical output (AC-2).
- **Hugepage reservation is appliance `config.json` (`image.hugepages`), not
  YANG.** It is consumed on the build host at image-assembly time, before any
  target YANG config exists. Optional `image.memory-bytes` bounds the reservation
  (≤50%) and also sizes `ze appliance run`'s QEMU `-m`.
- **Kernel args go through a derived gokrazy instance config in a temp parent
  dir**, never by editing the checked-in `gokrazy/ze/config.json`. gok resolves
  `<parent_dir>/<instance>/config.json`; `materializeDerivedParent` symlinks every
  sibling entry, writes a raw-JSON-patched `config.json` (preserving unknown
  fields via `map[string]json.RawMessage` — R-4), and excludes `builddir` for a
  cold isolated rebuild. `kernelargs.go` is a shared seam for this spec and
  spec-vpp-isolated-cpus.
- **Doctor check owned by the vpp component, linux-tagged.** Reads sysfs/procfs
  behind overridable roots; short-circuits to one error when nothing is reserved.

## Gotchas (save the next agent)

- **`env.Get` aborts on an unregistered key.** The doctor functional test
  redirects procfs/sysfs via `ze.test.doctor.hugepages-root`; that key MUST be
  `env.MustRegister`-ed (package-level `var _ =`) or `ze doctor` crashes the
  moment the check runs. Symptom: the check silently produces no diagnostic
  end-to-end while its unit tests pass.
- **Hook-shaped code structure:** registration must be in `register*.go` not a
  general file's `init()`; library files under `internal/` may not add new
  `fmt.Fprintf(os.Stderr, …)` (fold errors into an existing print or a helper
  returning `error`); string/number building uses `textbuf`, never `+` or
  `strconv.Format*`. TDD hook requires the `_test.go` before the impl file.
- **Cross-session staging contamination (shared working tree).** A concurrent
  session committed `cmd_build.go` while it carried my uncommitted
  `resolveBuildParentDir` call, so HEAD referenced a function defined only in my
  still-untracked `kernelargs.go` — HEAD was uncompilable-from-clean while every
  local working tree compiled. On this repo's shared tree, any file you edit that
  another session also touches can be swept into their commit; keep the defining
  and calling files in one atomic change and commit promptly.
- **`ze-validate` re-checks every exported symbol in a file you touch.** Adding a
  leaf to `config.go` surfaced 5 pre-existing "exported symbol has no
  cross-package caller" findings (`ParseSettings`, `GenerateStartupConf`, …) that
  are same-package-only and predate the change; they are not yours to refactor.
- **Linux-tagged non-integration tests run under normal `go test` on linux.** The
  skeleton's "add the package to `ze-qemu-integration-test`" was a no-op (that
  target is `-tags integration`); `make ze-unit-test` (`go test -race ./...`)
  already exercises `//go:build linux` files on a linux host.
- **Go 1.26 `new(expr)`** returns a pointer to a copy; the codebase uses
  `new(uint8(0))` in tests — match it for `*uint32` fields.

## Files

None recorded.
