---
kind: note
level:
stage:
---
This lints all packages with uncommitted Go changes, once for each BUILD, not
once. golangci-lint analyzes one GOOS, one GOARCH and one tag set for each run,
so a file outside that build is not merely unchecked: the pass exits 0 and reads
as clean over it.

Two passes are written in the recipe: the host build, then `GOOS=linux` with the
`integration` build tag. The second is the only thing that reads a
`//go:build integration` file, and on a non-Linux host it is the only thing that
reads a `//go:build linux` file. The rest come from
`scripts/dev/lint_flavors.py`, one for each personality tag (`ze_installer`,
`ze_distro`, `ze_appliance`, `ze_setup`), the capability tags, `tinygo`, and each
GOOS and GOARCH a tracked file names. Each flavor lints only the packages holding
a file the first two passes do not load, and that package set is derived from the
tree on every run rather than written down.

One directory answers with the whole tree, on purpose. Every file in
`cmd/ze-installer` carries `//go:build linux && ze_installer`, so `go list` under
the unit tag set reports no package there and the change-set selector has nothing
narrower to name (`scripts/checks/verify_scope_selector.go`,
`uncompiledTreeReaders`). It widens to `./...`, and the wide answer is what makes
the `ze_installer` flavor run at all: a narrow one would hand the driver a scope
the initrd's PID 1 is not in, and the gate would exit 0 over it.

Takes 3-10 seconds once the caches are warm, plus about 2 seconds for each flavor
whose packages the change reaches. The first run after a checkout pays a cold
analysis for each build, which is minutes.
